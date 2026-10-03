package system

import (
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
