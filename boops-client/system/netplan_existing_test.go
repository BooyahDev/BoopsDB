package system

import (
	"boops/client"
	"fmt"
	"os"
	"strings"
	"testing"
)

const ubuntuInstaller = `# This is the network config written by 'subiquity'
network:
  ethernets:
    ens18:
      dhcp4: true
      dhcp6: true
      match:
        macaddress: bc:24:11:d4:ae:87
      set-name: ens18
  version: 2
`

func TestUbuntuInstallerConfigurationIsUpdatedInPlace(t *testing.T) {
	o := newFixtureOps(t, "netplan")
	path := "/etc/netplan/00-installer-config.yaml"
	o.put(path, ubuntuInstaller, 0600)
	o.response = func(n string, a []string) ([]byte, error) {
		if n == "ip" {
			return []byte(`[{"ifname":"ens18","address":"bc:24:11:d4:ae:87"}]`), nil
		}
		return nil, nil
	}
	in := []client.InterfaceInfo{{Name: "ens18", IPs: []client.IPInfo{{IP: "10.0.2.42", Subnet: "255.255.0.0"}}, Gateway: "10.0.0.1", DnsServers: "1.1.1.1"}}
	if err := ApplyNetworkSettingsWithOps(in, o); err != nil {
		t.Fatal(err)
	}
	data, _ := o.ReadFile(path)
	for _, s := range []string{"dhcp4: false", "dhcp6: true", "bc:24:11:d4:ae:87", "set-name: ens18", "10.0.2.42/16", "10.0.0.1", "1.1.1.1", "# This is the network config"} {
		if !strings.Contains(string(data), s) {
			t.Fatalf("lost %s: %s", s, data)
		}
	}
	if exists, _ := o.Exists(netplanPath); exists {
		t.Fatal("created competing definition")
	}
	if o.count("netplan", "apply") != 1 {
		t.Fatal(o.calls)
	}
}

func TestExistingNetplanSplitDefinitionAndRollback(t *testing.T) {
	for _, failure := range []string{"", "generate", "apply", "write"} {
		t.Run(failure, func(t *testing.T) {
			o := newFixtureOps(t, "netplan")
			first := "/etc/netplan/00-installer.yaml"
			second := "/etc/netplan/50-cloud-init.yaml"
			a := "network:\n  version: 2\n  ethernets:\n    lan:\n      match: {name: eth0}\n      dhcp6: true\n      addresses: [192.0.2.9/24, '2001:db8::9/64']\n      nameservers: {addresses: [8.8.8.8]}\n"
			b := "network:\n  ethernets:\n    lan:\n      dhcp4: true\n      gateway4: 192.0.2.254\n      addresses: [192.0.2.8/24]\n    spare:\n      dhcp4: true\n"
			o.put(first, a, 0640)
			o.put(second, b, 0600)
			failed := false
			o.fail = func(n string, args []string) bool {
				if !failed && n == "netplan" && args[0] == failure {
					failed = true
					return true
				}
				return false
			}
			if failure == "write" {
				o.failWritePath = second
			}
			err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o)
			if failure != "" {
				if err == nil {
					t.Fatal("failure ignored")
				}
				for p, want := range map[string]string{first: a, second: b} {
					got, _ := o.ReadFile(p)
					if string(got) != want {
						t.Fatalf("not restored %s", p)
					}
					st, _ := os.Stat(o.local(p))
					mode := os.FileMode(0600)
					if p == first {
						mode = 0640
					}
					if st.Mode().Perm() != mode {
						t.Fatal("mode changed")
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			x, _ := o.ReadFile(first)
			y, _ := o.ReadFile(second)
			all := string(x) + string(y)
			for _, s := range []string{"192.0.2.9/24", "192.0.2.8/24", "192.0.2.254", "8.8.8.8", "dhcp4: true\n      gateway"} {
				if strings.Contains(all, s) {
					t.Fatalf("old IPv4 remains %s: %s", s, all)
				}
			}
			for _, s := range []string{"2001:db8::9/64", "dhcp6: true", "spare:"} {
				if !strings.Contains(all, s) {
					t.Fatal(fmt.Sprint("lost ", s))
				}
			}
			if strings.Count(all, "192.0.2.10/24") != 1 {
				t.Fatal("duplicate/missing new address")
			}
		})
	}
}

func TestLibraryNetplanIsShadowedWithoutChangingVendorBytes(t *testing.T) {
	o := newFixtureOps(t, "netplan")
	path := "/lib/netplan/40-default.yaml"
	original := "network:\n  version: 2\n  ethernets:\n    eth0:\n      dhcp4: true\n    spare:\n      dhcp6: true\n"
	o.put(path, original, 0644)
	if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err != nil {
		t.Fatal(err)
	}
	vendor, _ := o.ReadFile(path)
	if string(vendor) != original {
		t.Fatal("vendor configuration overwritten")
	}
	local, _ := o.ReadFile("/etc/netplan/40-default.yaml")
	if !strings.Contains(string(local), "192.0.2.10/24") || !strings.Contains(string(local), "spare:") {
		t.Fatal(string(local))
	}
}

func TestNetplanBridgeKeepsPortsAndManualMember(t *testing.T) {
	o := newFixtureOps(t, "netplan")
	path := "/etc/netplan/50-bridge.yaml"
	o.put(path, "network:\n  version: 2\n  ethernets:\n    eth0: {}\n  bridges:\n    br0:\n      interfaces: [eth0]\n      parameters: {stp: false}\n      addresses: [192.0.2.8/24, '2001:db8::8/64']\n", 0600)
	in := []client.InterfaceInfo{{Name: "eth0"}, {Name: "br0", IPs: twoNICs()[0].IPs, Gateway: "192.0.2.1"}}
	if err := ApplyNetworkSettingsWithOps(in, o); err != nil {
		t.Fatal(err)
	}
	data, _ := o.ReadFile(path)
	for _, value := range []string{"interfaces: [eth0]", "stp: false", "2001:db8::8/64", "192.0.2.10/24"} {
		if !strings.Contains(string(data), value) {
			t.Fatalf("lost %s: %s", value, data)
		}
	}
	if strings.Contains(string(data), "192.0.2.8/24") {
		t.Fatal("old IPv4 retained")
	}
}

func TestNewNetplanNICPreservesSameNameVendorFile(t *testing.T) {
	for _, includeExisting := range []bool{false, true} {
		t.Run(fmt.Sprint(includeExisting), func(t *testing.T) {
			o := newFixtureOps(t, "netplan")
			vendor := "network:\n  version: 2\n  renderer: networkd\n  ethernets:\n    eth0:\n      addresses: [192.0.2.9/24]\n      dhcp6: true\n    spare:\n      dhcp4: true\n"
			o.put("/lib/netplan/01-netcfg.yaml", vendor, 0644)
			in := twoNICs()[1:]
			if includeExisting {
				in = twoNICs()
			}
			if err := ApplyNetworkSettingsWithOps(in, o); err != nil {
				t.Fatal(err)
			}
			original, _ := o.ReadFile("/lib/netplan/01-netcfg.yaml")
			if string(original) != vendor {
				t.Fatal("vendor file changed")
			}
			data, err := o.ReadFile(netplanPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"renderer: networkd", "eth0:", "eth1:", "spare:", "dhcp6: true", "198.51.100.10/24"} {
				if !strings.Contains(string(data), want) {
					t.Fatalf("lost %s: %s", want, data)
				}
			}
			if includeExisting && (!strings.Contains(string(data), "192.0.2.10/24") || strings.Contains(string(data), "192.0.2.9/24")) {
				t.Fatalf("existing NIC update lost: %s", data)
			}
		})
	}
}

func TestRenamedNetplanNICRequiresVerifiedMAC(t *testing.T) {
	for _, verified := range []bool{false, true} {
		t.Run(fmt.Sprint(verified), func(t *testing.T) {
			o := newFixtureOps(t, "netplan")
			match := "name: old0"
			if verified {
				match += ", macaddress: '02:00:00:00:00:01'"
			}
			o.put("/etc/netplan/00-installer.yaml", "network:\n  version: 2\n  ethernets:\n    installer:\n      match: {"+match+"}\n      set-name: eth0\n      dhcp4: true\n", 0600)
			err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o)
			if verified {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || o.writes != 0 || o.count("netplan", "apply") != 0 {
				t.Fatalf("unverified rename modified config: %v writes=%d", err, o.writes)
			}
		})
	}
}

func TestApplyFilesRejectsDuplicateDestinationsBeforeWriting(t *testing.T) {
	o := newFixtureOps(t, "netplan")
	o.put(netplanPath, "original", 0600)
	snapshot, err := readSnapshot(o, netplanPath, 0600)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	err = applyFiles(o, []fileChange{{snapshot, []byte("first")}, {snapshot, []byte("second")}}, func() error { called = true; return nil })
	if err == nil || o.writes != 0 || called {
		t.Fatalf("duplicate plan was applied: %v writes=%d called=%v", err, o.writes, called)
	}
}
