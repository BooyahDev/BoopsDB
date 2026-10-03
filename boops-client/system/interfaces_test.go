package system

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestInterfacesKeepsUnmanagedOptionsAndClearsGateway(t *testing.T) {
	o := newFixtureOps(t, "interfaces")
	o.put("/etc/network/interfaces", "# keep\nauto lo eth0 eth1\niface lo inet loopback\n\niface eth0 inet static\n  address 192.0.2.9\n  netmask 255.255.255.0\n  mtu 9000\n\niface eth1 inet static\n  address 198.51.100.9\n  gateway 198.51.100.1\n  post-up ip route add 203.0.113.0/24 via 198.51.100.5\n\niface eth1 inet6 auto\n  accept_ra 1\n", 0644)
	if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err != nil {
		t.Fatal(err)
	}
	b, _ := o.ReadFile("/etc/network/interfaces")
	s := string(b)
	for _, p := range []string{"iface lo inet loopback", "mtu 9000", "post-up ip route add 203.0.113.0/24", "iface eth1 inet6 auto", "address 192.0.2.10/24", "address 192.0.2.11/24", "address 198.51.100.10/24"} {
		if !strings.Contains(s, p) {
			t.Fatalf("lost %q:\n%s", p, s)
		}
	}
	if strings.Contains(s, "gateway 198.51.100.1") {
		t.Fatal("old gateway retained")
	}
	if o.writes != 1 {
		t.Fatal("not one batch")
	}
}

func TestInterfacesApplyFailureRestoresOriginalBytesAndMode(t *testing.T) {
	o := newFixtureOps(t, "interfaces")
	original := "auto eth0 eth1\niface eth0 inet dhcp\niface eth1 inet dhcp\n"
	o.put(interfacesPath, original, 0640)
	failed := false
	o.fail = func(n string, _ []string) bool {
		if n == "ifup" && !failed {
			failed = true
			return true
		}
		return false
	}
	if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err == nil {
		t.Fatal("ifup failure ignored")
	}
	got, _ := o.ReadFile(interfacesPath)
	if string(got) != original {
		t.Fatal("original not restored")
	}
	info, _ := os.Stat(o.local(interfacesPath))
	if info.Mode().Perm() != 0640 {
		t.Fatal("mode not restored")
	}
}

func TestInterfacesSourceDirectoryUsesRelativeNestedIncludes(t *testing.T) {
	o := newFixtureOps(t, "interfaces")
	o.put(interfacesPath, "source-directory interfaces.d\n", 0644)
	o.put("/etc/network/interfaces.d/extra", "source nested\n", 0644)
	o.put("/etc/network/interfaces.d/nested", "iface eth1 inet static\n  address 198.51.100.5\n", 0644)
	if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err == nil {
		t.Fatal("nested conflict accepted")
	}
	if o.writes != 0 {
		t.Fatal("nested conflict wrote config")
	}
}

func TestInterfacesMappingAndAliasPreventWrites(t *testing.T) {
	for _, content := range []string{"mapping eth*\n  script /usr/local/bin/map-nic\n", "source interfaces.d/*\n"} {
		t.Run(content, func(t *testing.T) {
			o := newFixtureOps(t, "interfaces")
			o.put(interfacesPath, content, 0644)
			if strings.HasPrefix(content, "source") {
				o.put("/etc/network/interfaces.d/alias", "iface eth1:1 inet static\n  address 198.51.100.20\n", 0644)
			}
			if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err == nil {
				t.Fatal("mapping/alias conflict accepted")
			}
			if o.writes != 0 {
				t.Fatal("mapping/alias changed host")
			}
		})
	}
}

func TestInterfacesIfdownReadsOriginalConfiguration(t *testing.T) {
	for _, original := range []string{
		"auto eth0\niface eth0 inet static\n  address 192.0.2.9/24\n  gateway 192.0.2.254\n",
		"auto eth0\niface eth0 inet dhcp\n",
	} {
		t.Run(original, func(t *testing.T) {
			o := newFixtureOps(t, "interfaces")
			o.put(interfacesPath, original, 0644)
			o.response = func(n string, _ []string) ([]byte, error) {
				if n == "ifdown" {
					got, _ := o.ReadFile(interfacesPath)
					if string(got) != original {
						t.Fatalf("ifdown read replacement instead of original:\n%s", got)
					}
				}
				return nil, nil
			}
			if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err != nil {
				t.Fatal(err)
			}
			if o.count("ifdown", "") != 1 || o.count("ifup", "") != 1 || o.writes != 1 {
				t.Fatal("successful apply was not one batch")
			}
		})
	}
}

func TestInterfacesContinuedDefaultHookAndInheritancePreventWrites(t *testing.T) {
	for _, content := range []string{
		"auto eth0\niface eth0 inet static\n  post-up ip route add \\\n    default via 192.0.2.254\n",
		"iface defaults inet static\n  mtu 9000\n  post-up ip route add 203.0.113.0/24 via 192.0.2.5\nauto eth0\niface eth0 inet static inherits defaults\n  address 192.0.2.9/24\n",
	} {
		t.Run(content, func(t *testing.T) {
			o := newFixtureOps(t, "interfaces")
			o.put(interfacesPath, content, 0644)
			if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err == nil {
				t.Fatal("continued default route/inheritance accepted")
			}
			if o.writes != 0 || o.count("ifdown", "") != 0 || o.count("ifup", "") != 0 {
				t.Fatal("preflight conflict changed host")
			}
		})
	}
}

func TestInterfacesRollbackStopsNewBeforeRestoringOriginal(t *testing.T) {
	o := newFixtureOps(t, "interfaces")
	original := "auto eth0\niface eth0 inet dhcp\n"
	o.put(interfacesPath, original, 0640)
	var downs, ups []string
	failed := false
	o.response = func(n string, _ []string) ([]byte, error) {
		if n == "ifdown" || n == "ifup" {
			data, _ := o.ReadFile(interfacesPath)
			if n == "ifdown" {
				downs = append(downs, string(data))
			} else {
				ups = append(ups, string(data))
				if !failed {
					failed = true
					return nil, errors.New("activation failed")
				}
			}
		}
		return nil, nil
	}
	if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err == nil {
		t.Fatal("activation failure ignored")
	}
	if len(downs) != 2 || downs[0] != original || !strings.Contains(downs[1], "address 192.0.2.10/24") {
		t.Fatalf("wrong teardown configurations: %#v", downs)
	}
	if len(ups) != 2 || !strings.Contains(ups[0], "address 192.0.2.10/24") || ups[1] != original {
		t.Fatalf("wrong activation configurations: %#v", ups)
	}
}

type interfaceWriteFailureOps struct {
	*fixtureOps
	failed bool
}

func (o *interfaceWriteFailureOps) WriteFile(p string, b []byte, m fs.FileMode) error {
	if !o.failed {
		o.failed = true
		return errors.New("fixture write failure")
	}
	return o.fixtureOps.WriteFile(p, b, m)
}

func TestInterfacesTeardownAndWriteFailuresReactivateOriginal(t *testing.T) {
	for _, failure := range []string{"down", "write"} {
		t.Run(failure, func(t *testing.T) {
			o := newFixtureOps(t, "interfaces")
			original := "auto eth0\niface eth0 inet dhcp\n"
			o.put(interfacesPath, original, 0640)
			var ops Ops = o
			if failure == "down" {
				o.fail = func(name string, _ []string) bool { return name == "ifdown" }
			} else {
				ops = &interfaceWriteFailureOps{fixtureOps: o}
			}
			upCount := 0
			o.response = func(name string, _ []string) ([]byte, error) {
				if name == "ifup" {
					upCount++
					data, _ := o.ReadFile(interfacesPath)
					if string(data) != original {
						t.Fatal("recovery used replacement instead of original")
					}
				}
				return nil, nil
			}
			if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], ops); err == nil {
				t.Fatal("failure ignored")
			}
			if upCount != 1 {
				t.Fatal("original configuration not reactivated")
			}
			data, _ := o.ReadFile(interfacesPath)
			if string(data) != original {
				t.Fatal("original file changed")
			}
		})
	}
}

func TestInterfacesContinuedOrdinaryHookAndManagedValues(t *testing.T) {
	o := newFixtureOps(t, "interfaces")
	o.put(interfacesPath, "auto eth0\niface eth0 inet static\n  address \\\n    192.0.2.9/24\n  dns-nameservers \\\n    8.8.8.8\n  post-up ip route add \\\n    203.0.113.0/24 via 192.0.2.5\n", 0644)
	if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err != nil {
		t.Fatal(err)
	}
	data, _ := o.ReadFile(interfacesPath)
	if strings.Contains(string(data), "192.0.2.9") || strings.Contains(string(data), "8.8.8.8") {
		t.Fatalf("continued old values survived:\n%s", data)
	}
	if !strings.Contains(string(data), "post-up ip route add \\\n    203.0.113.0/24 via 192.0.2.5") {
		t.Fatalf("ordinary continued hook lost:\n%s", data)
	}
}
