package system

import (
	"os"
	"strings"
	"testing"
)

const netplanOriginal = `# preserve header
network:
  version: 2
  ethernets:
    eth0:
      # retain device comment
      mtu: 9000
      addresses: [192.0.2.9/24]
      gateway4: 192.0.2.254
      routes:
        - to: 203.0.113.0/24
          via: 192.0.2.5
        - to: default
          via: 192.0.2.254
        - to: default
          via: "2001:db8::1"
    spare:
      dhcp4: true # unrelated
  bridges:
    br0:
      interfaces: [spare]
  vlans:
    vlan10:
      id: 10
      link: br0
`

func TestNetplanKeepsTwoNICsAndUnmanagedNodes(t *testing.T) {
	o := newFixtureOps(t, "netplan")
	o.put("/etc/netplan/01-netcfg.yaml", netplanOriginal, 0640)
	if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err != nil {
		t.Fatal(err)
	}
	b, err := o.ReadFile("/etc/netplan/01-netcfg.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, part := range []string{"eth0:", "eth1:", "192.0.2.10/24", "198.51.100.10/24", "spare:", "br0:", "vlan10:", "mtu: 9000", "203.0.113.0/24", "2001:db8::1", "# preserve header", "# retain device comment"} {
		if !strings.Contains(s, part) {
			t.Fatalf("lost %q:\n%s", part, s)
		}
	}
	if strings.Contains(s, "gateway4:") || strings.Contains(s, "192.0.2.254") {
		t.Fatalf("old default route retained:\n%s", s)
	}
	if o.writes != 1 || o.count("netplan", "apply") != 1 {
		t.Fatalf("not one batch: writes=%d calls=%v", o.writes, o.calls)
	}
	st, _ := os.Stat(o.local("/etc/netplan/01-netcfg.yaml"))
	if st.Mode().Perm() != 0640 {
		t.Fatalf("mode changed: %o", st.Mode().Perm())
	}
}

func TestApplyFailureRestoresConfig(t *testing.T) {
	for _, failure := range []string{"generate", "apply"} {
		t.Run(failure, func(t *testing.T) {
			o := newFixtureOps(t, "netplan")
			o.put("/etc/netplan/01-netcfg.yaml", netplanOriginal, 0640)
			failed := false
			o.fail = func(n string, a []string) bool {
				if !failed && n == "netplan" && a[0] == failure {
					failed = true
					return true
				}
				return false
			}
			if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err == nil {
				t.Fatal("apply failure ignored")
			}
			got, _ := o.ReadFile("/etc/netplan/01-netcfg.yaml")
			if string(got) != netplanOriginal {
				t.Fatal("original bytes not restored")
			}
			st, _ := os.Stat(o.local("/etc/netplan/01-netcfg.yaml"))
			if st.Mode().Perm() != 0640 {
				t.Fatal("original mode not restored")
			}
		})
	}
}

func TestForeignNetplanAndInterfacesHooksPreventWrites(t *testing.T) {
	for _, path := range []string{"/lib/netplan/10-foreign.yaml", "/etc/netplan/10-foreign.yaml", "/run/netplan/10-foreign.yaml"} {
		t.Run(path, func(t *testing.T) {
			o := newFixtureOps(t, "netplan")
			o.put("/etc/netplan/01-netcfg.yaml", netplanOriginal, 0600)
			o.put(path, "network:\n  ethernets:\n    eth1:\n      dhcp4: true\n", 0600)
			if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err == nil {
				t.Fatal("foreign config accepted")
			}
			if o.writes != 0 || o.count("netplan", "apply") != 0 {
				t.Fatal("conflict changed host")
			}
		})
	}
	for _, s := range []string{"iface eth1 inet static\n  up ip route add default via 198.51.100.1\n", "source interfaces.d/*\n"} {
		t.Run(s, func(t *testing.T) {
			o := newFixtureOps(t, "interfaces")
			o.put("/etc/network/interfaces", s, 0644)
			if strings.HasPrefix(s, "source") {
				o.put("/etc/network/interfaces.d/extra", "iface eth1 inet dhcp\n", 0644)
			}
			if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err == nil {
				t.Fatal("interfaces conflict accepted")
			}
			if o.writes != 0 || o.count("ifup", "") != 0 {
				t.Fatal("conflict changed host")
			}
		})
	}
}

func TestNetplanForeignAliasesAndInvalidVersionPreventWrites(t *testing.T) {
	for _, kind := range []string{"alias", "merge", "version"} {
		t.Run(kind, func(t *testing.T) {
			o := newFixtureOps(t, "netplan")
			o.put(netplanPath, netplanOriginal, 0600)
			switch kind {
			case "alias":
				o.put("/etc/netplan/20-foreign.yaml", "network:\n  ethernets:\n    template: &config\n      dhcp4: true\n    eth1: *config\n", 0600)
			case "merge":
				o.put("/etc/netplan/20-foreign.yaml", "network:\n  ethernets:\n    template: &config\n      dhcp4: true\n    eth1:\n      <<: *config\n", 0600)
			case "version":
				o.put(netplanPath, "network:\n  version: 1\n", 0600)
			}
			if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err == nil {
				t.Fatal("unsafe Netplan accepted")
			}
			if o.writes != 0 || o.count("netplan", "apply") != 0 {
				t.Fatal("unsafe Netplan changed host")
			}
		})
	}
}

func TestNetplanShadowedDefinitionsAndIPv6ArePreserved(t *testing.T) {
	o := newFixtureOps(t, "netplan")
	o.put(netplanPath, netplanOriginal, 0600)
	o.put("/lib/netplan/20-other.yaml", "network:\n  ethernets:\n    eth1:\n      dhcp4: true\n", 0600)
	o.put("/run/netplan/20-other.yaml", "network:\n  ethernets:\n    spare2:\n      dhcp4: true\n", 0600)
	if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err != nil {
		t.Fatal(err)
	}
	foreign, _ := o.ReadFile("/lib/netplan/20-other.yaml")
	if !strings.Contains(string(foreign), "eth1:") {
		t.Fatal("foreign file changed")
	}
	o = newFixtureOps(t, "netplan")
	o.put(netplanPath, "network:\n  version: 2\n  ethernets:\n    eth0:\n      addresses: [192.0.2.9/24, '2001:db8::10/64']\n      nameservers:\n        search: [example.test]\n        addresses: [8.8.8.8, '2001:4860:4860::8888']\n", 0600)
	if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err != nil {
		t.Fatal(err)
	}
	got, _ := o.ReadFile(netplanPath)
	for _, part := range []string{"2001:db8::10/64", "2001:4860:4860::8888", "example.test"} {
		if !strings.Contains(string(got), part) {
			t.Fatalf("lost IPv6/search %q", part)
		}
	}
}

func TestNetplanPermissionReadFailurePreventsWrites(t *testing.T) {
	o := newFixtureOps(t, "netplan")
	o.put(netplanPath, netplanOriginal, 0640)
	o.fail = func(n string, _ []string) bool { return n == "stat" }
	if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err == nil {
		t.Fatal("permission read failure ignored")
	}
	if o.writes != 0 {
		t.Fatal("wrote without backup permissions")
	}
}

func TestNetplanAmbiguousDeviceAndInheritedNICPreventWrites(t *testing.T) {
	for _, kind := range []string{"shared-match", "foreign-merge", "id-collision"} {
		t.Run(kind, func(t *testing.T) {
			o := newFixtureOps(t, "netplan")
			switch kind {
			case "shared-match":
				o.put(netplanPath, "network:\n  version: 2\n  ethernets:\n    lan:\n      match:\n        name: eth*\n      dhcp4: true\n", 0600)
			case "foreign-merge":
				o.put(netplanPath, netplanOriginal, 0600)
				o.put("/etc/netplan/20-foreign.yaml", "defaults: &config\n  eth1:\n    dhcp4: true\nnetwork:\n  ethernets:\n    <<: *config\n", 0600)
			case "id-collision":
				o.put(netplanPath, "network:\n  version: 2\n  ethernets:\n    eth0:\n      match:\n        name: unrelated\n      dhcp4: true\n", 0600)
			}
			if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err == nil {
				t.Fatal("ambiguous Netplan accepted")
			}
			if o.writes != 0 {
				t.Fatal("ambiguous definition changed host")
			}
		})
	}
}
