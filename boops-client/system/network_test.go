package system

import (
	"boops/client"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type opCall struct {
	name string
	args []string
}
type fixtureOps struct {
	t                     *testing.T
	root, osName, backend string
	calls                 []opCall
	writes                int
	fail                  func(string, []string) bool
	response              func(string, []string) ([]byte, error)
}

func newFixtureOps(t *testing.T, backend string) *fixtureOps {
	o := &fixtureOps{t: t, root: t.TempDir(), osName: "linux", backend: backend}
	if err := os.MkdirAll(o.local(os.TempDir()), 0755); err != nil {
		t.Fatal(err)
	}
	return o
}
func (o *fixtureOps) local(path string) string {
	return filepath.Join(o.root, strings.TrimPrefix(path, "/"))
}
func (o *fixtureOps) put(path, data string, mode fs.FileMode) {
	o.t.Helper()
	p := o.local(path)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		o.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), mode); err != nil {
		o.t.Fatal(err)
	}
	if err := os.Chmod(p, mode); err != nil {
		o.t.Fatal(err)
	}
}
func (o *fixtureOps) OS() string { return o.osName }
func (o *fixtureOps) Run(name string, args ...string) ([]byte, error) {
	o.calls = append(o.calls, opCall{name, append([]string(nil), args...)})
	if o.fail != nil && o.fail(name, args) {
		return []byte("fixture failure"), errors.New("fixture failure")
	}
	if o.response != nil {
		if b, e := o.response(name, args); b != nil || e != nil {
			return b, e
		}
	}
	if name == "ip" && len(args) > 2 && args[0] == "-j" && args[1] == "link" {
		nic := args[len(args)-1]
		mac := "02:00:00:00:00:01"
		if nic == "eth1" {
			mac = "02:00:00:00:00:02"
		}
		return json.Marshal([]map[string]string{{"ifname": nic, "address": mac}})
	}
	if name == "stat" {
		s, e := os.Stat(o.local(args[len(args)-1]))
		if e != nil {
			return nil, e
		}
		return []byte(fmt.Sprintf("%o", s.Mode().Perm())), nil
	}
	if name == "netsh" && strings.Join(args, " ") == "interface ipv4 dump" {
		return []byte("# original netsh configuration\n"), nil
	}
	return []byte{}, nil
}
func (o *fixtureOps) ReadFile(p string) ([]byte, error) { return os.ReadFile(o.local(p)) }
func (o *fixtureOps) WriteFile(p string, b []byte, m fs.FileMode) error {
	o.writes++
	return RealOps().WriteFile(o.local(p), b, m)
}
func (o *fixtureOps) Rename(a, b string) error { return os.Rename(o.local(a), o.local(b)) }
func (o *fixtureOps) Remove(p string) error    { return os.Remove(o.local(p)) }
func (o *fixtureOps) Glob(p string) ([]string, error) {
	r, e := filepath.Glob(o.local(p))
	for i := range r {
		r[i] = "/" + strings.TrimPrefix(r[i], o.root+"/")
	}
	return r, e
}
func (o *fixtureOps) Exists(p string) (bool, error) {
	_, e := os.Stat(o.local(p))
	if errors.Is(e, fs.ErrNotExist) {
		return false, nil
	}
	return e == nil, e
}
func (o *fixtureOps) LookPath(n string) (string, error) {
	if n == o.backend || (o.backend == "interfaces" && (n == "ifup" || n == "ifdown")) {
		return "/usr/bin/" + n, nil
	}
	return "", errors.New("not found")
}
func (o *fixtureOps) count(name string, firstArg string) int {
	n := 0
	for _, c := range o.calls {
		if c.name == name && (firstArg == "" || (len(c.args) > 0 && c.args[0] == firstArg)) {
			n++
		}
	}
	return n
}

func twoNICs() []client.InterfaceInfo {
	return []client.InterfaceInfo{
		{Name: "eth0", IPs: []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}, {IP: "192.0.2.11", Subnet: "255.255.255.0"}}, Gateway: "192.0.2.1", DnsServers: "1.1.1.1"},
		{Name: "eth1", IPs: []client.IPInfo{{IP: "198.51.100.10", Subnet: "255.255.255.0"}}},
	}
}

func TestInvalidAndMissingNICPreventWrites(t *testing.T) {
	for _, tt := range []string{"invalid", "missing"} {
		t.Run(tt, func(t *testing.T) {
			o := newFixtureOps(t, "netplan")
			o.put("/etc/netplan/01-netcfg.yaml", "network:\n  version: 2\n", 0600)
			in := twoNICs()
			if tt == "invalid" {
				in[1].IPs[0].Subnet = "255.0.255.0"
			} else {
				o.fail = func(n string, a []string) bool { return n == "ip" && a[len(a)-1] == "eth1" }
			}
			if err := ApplyNetworkSettingsWithOps(in, o); err == nil {
				t.Fatal("invalid/missing NIC accepted")
			}
			if o.writes != 0 || o.count("netplan", "apply") != 0 {
				t.Fatal("preflight failure changed host")
			}
		})
	}
}

func TestGatherInterfacesSeparatesNameAndMAC(t *testing.T) {
	o := newFixtureOps(t, "")
	o.response = func(n string, a []string) ([]byte, error) {
		if n == "ip" {
			return []byte(`[{"ifname":"eth0","address":"02:00:00:00:00:01","addr_info":[{"family":"inet","local":"192.0.2.10","prefixlen":24},{"family":"inet6","local":"2001:db8::1","prefixlen":64}]}]`), nil
		}
		return nil, nil
	}
	got, err := gatherNetworkInterfacesWithOps(o)
	if err != nil {
		t.Fatal(err)
	}
	if got["eth0"].Name != "eth0" || got["eth0"].MacAddress != "02:00:00:00:00:01" || len(got["eth0"].IPs) != 1 {
		t.Fatalf("wrong gathered info: %#v", got)
	}
}

func TestInvalidLocalIdentityPreventsWrites(t *testing.T) {
	for _, data := range []string{"broken", `[]`, `[{"ifname":"eth0","address":"not a MAC"}]`, `[{"ifname":"other","address":"02:00:00:00:00:01"}]`} {
		t.Run(data, func(t *testing.T) {
			o := newFixtureOps(t, "netplan")
			o.put(netplanPath, netplanOriginal, 0600)
			o.response = func(name string, _ []string) ([]byte, error) {
				if name == "ip" {
					return []byte(data), nil
				}
				return nil, nil
			}
			if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err == nil {
				t.Fatal("invalid local identity accepted")
			}
			if o.writes != 0 || o.count("netplan", "apply") != 0 {
				t.Fatal("identity failure changed host")
			}
		})
	}
}
