package system

import (
	"boops/client"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
)

type ifreloadFixtureOps struct {
	*fixtureOps
}

func (o *ifreloadFixtureOps) LookPath(name string) (string, error) {
	if name == "ifreload" {
		return "/usr/sbin/" + name, nil
	}
	return o.fixtureOps.LookPath(name)
}

func TestInterfacesKeepsUnmanagedOptionsAndClearsGateway(t *testing.T) {
	o := newFixtureOps(t, "interfaces")
	o.put("/etc/network/interfaces", "# keep\nauto lo eth0 eth1\niface lo inet loopback\n\niface eth0 inet static\n  address 192.0.2.9\n  netmask 255.255.255.0\n  mtu 9000\n\niface eth1 inet static\n  address 198.51.100.9\n  gateway 198.51.100.1\n  post-up ip route add 203.0.113.0/24 via 198.51.100.5\n\niface eth1 inet6 auto\n  accept_ra 1\n", 0644)
	if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err != nil {
		t.Fatal(err)
	}
	b, _ := o.ReadFile("/etc/network/interfaces")
	s := string(b)
	for _, p := range []string{"iface lo inet loopback", "mtu 9000", "post-up ip route add 203.0.113.0/24", "iface eth1 inet6 auto", "address 192.0.2.10/24", "# boops-managed-address 192.0.2.11/24", "address 198.51.100.10/24"} {
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

func TestInterfacesUsesPortablePrimaryAddressAndManagedAdditionalHook(t *testing.T) {
	o := newFixtureOps(t, "interfaces")
	o.put(interfacesPath, "auto eth0\niface eth0 inet static\n  address 192.0.2.9/24\n", 0644)
	if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err != nil {
		t.Fatal(err)
	}
	data, err := o.ReadFile(interfacesPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "    address 192.0.2.10/24") {
		t.Fatalf("primary address was not emitted as standard ifupdown setting:\n%s", got)
	}
	if strings.Contains(got, "    address 192.0.2.11/24") {
		t.Fatalf("additional address relies on repeated classic-ifupdown address keys:\n%s", got)
	}
	for _, want := range []string{
		"# boops-managed-address 192.0.2.11/24",
		`up ip -4 addr replace 192.0.2.11/24 dev "$IFACE"`,
		`down ip -4 addr del 192.0.2.11/24 dev "$IFACE" || true`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing portable additional-address hook %q:\n%s", want, got)
		}
	}
}

func TestInterfacesReplacesManagedAdditionalAddressHooks(t *testing.T) {
	o := newFixtureOps(t, "interfaces")
	o.put(interfacesPath, "auto eth0\niface eth0 inet static\n  address 192.0.2.9/24\n", 0644)
	if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err != nil {
		t.Fatal(err)
	}
	updated := twoNICs()[:1]
	updated[0].IPs[1].IP = "192.0.2.12"
	if err := ApplyNetworkSettingsWithOps(updated, o); err != nil {
		t.Fatal(err)
	}
	data, err := o.ReadFile(interfacesPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Contains(got, "192.0.2.11") || !strings.Contains(got, "# boops-managed-address 192.0.2.12/24") || strings.Count(got, "# boops-managed-address ") != 1 {
		t.Fatalf("stale managed additional address survived replacement:\n%s", got)
	}
}

func TestInterfacesAddresslessBridgeUsesManualMethod(t *testing.T) {
	o := &ifreloadFixtureOps{fixtureOps: newFixtureOps(t, "ifreload")}
	o.put(interfacesPath, "auto vmbr0\niface vmbr0 inet static\n  address 192.0.2.9/24\n  gateway 192.0.2.1\n  bridge-ports eno1\n  bridge-stp off\n  bridge-fd 0\n", 0640)
	if err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{Name: "vmbr0"}}, o); err != nil {
		t.Fatal(err)
	}
	data, err := o.ReadFile(interfacesPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "iface vmbr0 inet manual") || strings.Contains(got, "address 192.0.2.9") || strings.Contains(got, "gateway 192.0.2.1") {
		t.Fatalf("addressless bridge was not converted to manual while preserving bridge config:\n%s", got)
	}
	for _, want := range []string{"bridge-ports eno1", "bridge-stp off", "bridge-fd 0"} {
		if !strings.Contains(got, want) {
			t.Fatalf("lost bridge setting %q:\n%s", want, got)
		}
	}
}

func TestInterfacesQuotesManagedAddressHookInterfaceAndRejectsWhitespace(t *testing.T) {
	o := &ifreloadFixtureOps{fixtureOps: newFixtureOps(t, "ifreload")}
	o.put(interfacesPath, "auto eth0\niface eth0 inet static\n  address 192.0.2.9/24\n", 0644)
	if err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{
		Name: "eth0",
		IPs: []client.IPInfo{
			{IP: "192.0.2.10", Subnet: "255.255.255.0"},
			{IP: "192.0.2.11", Subnet: "255.255.255.0"},
		},
	}}, o); err != nil {
		t.Fatal(err)
	}
	data, err := o.ReadFile(interfacesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `dev "$IFACE"`) {
		t.Fatalf("managed address hook did not quote IFACE:\n%s", data)
	}

	o = &ifreloadFixtureOps{fixtureOps: newFixtureOps(t, "ifreload")}
	o.put(interfacesPath, "auto lo\niface lo inet loopback\n", 0644)
	if err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{Name: "Ethernet 2"}}, o); err == nil {
		t.Fatal("ifupdown accepted a NIC name containing whitespace")
	}
	if o.writes != 0 {
		t.Fatalf("invalid ifupdown NIC name changed files: %d writes", o.writes)
	}
}

func TestInterfacesIfreloadCurrentlyUpHandlesNonAutoStanza(t *testing.T) {
	o := &ifreloadFixtureOps{fixtureOps: newFixtureOps(t, "ifreload")}
	o.put(interfacesPath, "allow-hotplug eth0\niface eth0 inet static\n  address 192.0.2.9/24\n", 0644)
	o.put("/run/network/ifstate", "eth0=eth0\n", 0644)
	if err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{
		Name: "eth0",
		IPs:  []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}},
	}}, o); err != nil {
		t.Fatal(err)
	}
	if o.count("ifreload", "-c") != 1 || o.count("ifreload", "-a") != 0 {
		t.Fatalf("non-auto active stanza did not use ifreload -c: %#v", o.calls)
	}
}

func TestInterfacesIfreloadActivatesInactiveNonAutoStanza(t *testing.T) {
	o := &ifreloadFixtureOps{fixtureOps: newFixtureOps(t, "interfaces")}
	o.put(interfacesPath, "allow-hotplug eth0\niface eth0 inet static\n  address 192.0.2.9/24\n", 0644)
	if err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{
		Name: "eth0",
		IPs:  []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}},
	}}, o); err != nil {
		t.Fatal(err)
	}
	if o.count("ifreload", "-a") != 0 || o.count("ifreload", "-c") != 0 || o.count("ifup", "") != 1 {
		t.Fatalf("inactive non-auto stanza did not use checked ifup: %#v", o.calls)
	}
}

func TestInterfacesIfreloadInactiveActivationFailureRestoresFile(t *testing.T) {
	o := &ifreloadFixtureOps{fixtureOps: newFixtureOps(t, "interfaces")}
	original := "allow-hotplug eth0\niface eth0 inet static\n  address 192.0.2.9/24\n"
	o.put(interfacesPath, original, 0644)
	failed := false
	o.fail = func(name string, _ []string) bool {
		if name == "ifup" && !failed {
			failed = true
			return true
		}
		return false
	}
	if err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{
		Name: "eth0",
		IPs:  []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}},
	}}, o); err == nil {
		t.Fatal("inactive non-auto activation failure was ignored")
	}
	data, err := o.ReadFile(interfacesPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("original inactive configuration was not restored:\n%s", data)
	}
	if o.count("ifup", "") != 1 || o.count("ifdown", "") != 1 {
		t.Fatalf("partial non-auto activation was not cleaned up: %#v", o.calls)
	}
}

func TestInterfacesRefusesLegacyIfdownForBridgeWithoutIfreload(t *testing.T) {
	o := newFixtureOps(t, "interfaces")
	original := "auto vmbr0\niface vmbr0 inet static\n  address 192.0.2.9/24\n  bridge-ports eno1\n  bridge-stp off\n  bridge-fd 0\n"
	o.put(interfacesPath, original, 0640)
	if err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{
		Name: "eno1",
	}}, o); err == nil {
		t.Fatal("legacy ifdown was allowed to replace a bridge member without ifreload")
	}
	if o.writes != 0 || o.count("ifdown", "") != 0 || o.count("ifup", "") != 0 {
		t.Fatalf("bridge prerequisite failure changed host: %#v", o.calls)
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
	o := &ifreloadFixtureOps{fixtureOps: newFixtureOps(t, "ifreload")}
	o.put(interfacesPath, "source-directory interfaces.d\n", 0644)
	o.put("/etc/network/interfaces.d/extra", "source nested\n", 0644)
	o.put("/etc/network/interfaces.d/nested", "auto eth1\niface eth1 inet static\n  address 198.51.100.5\n", 0644)
	if err := ApplyNetworkSettingsWithOps(twoNICs()[1:], o); err != nil {
		t.Fatal(err)
	}
	data, err := o.ReadFile("/etc/network/interfaces.d/nested")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "address 198.51.100.10/24") {
		t.Fatalf("nested included NIC was not updated:\n%s", data)
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
	o.put("/run/network/ifstate", "eth0=eth0\n", 0644)
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
			o.put("/run/network/ifstate", "eth0=eth0\n", 0644)
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

func TestInterfacesNewNICSkipsUnknownOldTeardown(t *testing.T) {
	o := newFixtureOps(t, "interfaces")
	o.put(interfacesPath, "auto lo\niface lo inet loopback\n", 0644)
	o.response = func(name string, args []string) ([]byte, error) {
		if name == "ifdown" || name == "ifup" {
			data, _ := o.ReadFile(interfacesPath)
			for _, nic := range args[2:] {
				if !strings.Contains(string(data), "iface "+nic+" ") {
					return nil, fmt.Errorf("unknown interface %s", nic)
				}
			}
		}
		return nil, nil
	}
	if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err != nil {
		t.Fatal(err)
	}
	if o.count("ifdown", "") != 0 || o.count("ifup", "") != 1 || o.writes != 1 {
		t.Fatal("first configuration performed old teardown or failed one batch")
	}
}

func TestInterfacesIPv4HookInIPv6StanzaPreventsWrites(t *testing.T) {
	for _, included := range []bool{false, true} {
		t.Run(fmt.Sprint(included), func(t *testing.T) {
			o := newFixtureOps(t, "interfaces")
			stanza := "iface eth0 inet6 auto\n  post-up ip -4 route add \\\n    default via 192.0.2.254\n"
			root := "auto eth0\niface eth0 inet static\n  address 192.0.2.9/24\n"
			if included {
				o.put(interfacesPath, root+"source interfaces.d/*\n", 0644)
				o.put("/etc/network/interfaces.d/ipv6", stanza, 0644)
			} else {
				o.put(interfacesPath, root+stanza, 0644)
			}
			if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err == nil {
				t.Fatal("IPv4 hook in preserved IPv6 family accepted")
			}
			if o.writes != 0 || o.count("ifdown", "") != 0 || o.count("ifup", "") != 0 {
				t.Fatal("IPv4 hook conflict changed host")
			}
		})
	}
}

func TestInterfacesKeepsIPv6DefaultHook(t *testing.T) {
	o := newFixtureOps(t, "interfaces")
	o.put(interfacesPath, "auto eth0\niface eth0 inet static\n  address 192.0.2.9/24\niface eth0 inet6 auto\n  post-up ip -6 route add default via 2001:db8::1\n", 0644)
	if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err != nil {
		t.Fatal(err)
	}
	data, _ := o.ReadFile(interfacesPath)
	if !strings.Contains(string(data), "post-up ip -6 route add default via 2001:db8::1") {
		t.Fatal("IPv6 hook changed")
	}
}

func TestInterfacesMixedNewNICAndFailureRestoreOnlyOriginalNIC(t *testing.T) {
	for _, activationFails := range []bool{false, true} {
		t.Run(fmt.Sprint(activationFails), func(t *testing.T) {
			o := newFixtureOps(t, "interfaces")
			original := "auto eth0\niface eth0 inet dhcp\n"
			o.put(interfacesPath, original, 0640)
			o.put("/run/network/ifstate", "eth0=eth0\n", 0644)
			failed := false
			o.response = func(name string, args []string) ([]byte, error) {
				if name == "ifdown" || name == "ifup" {
					data, _ := o.ReadFile(interfacesPath)
					for _, nic := range args[2:] {
						if !strings.Contains(string(data), "iface "+nic+" ") {
							return nil, fmt.Errorf("unknown interface %s", nic)
						}
					}
					if activationFails && !failed && name == "ifup" {
						failed = true
						return nil, errors.New("activation failed")
					}
				}
				return nil, nil
			}
			err := ApplyNetworkSettingsWithOps(twoNICs(), o)
			if activationFails && err == nil {
				t.Fatal("activation failure ignored")
			}
			if !activationFails && err != nil {
				t.Fatal(err)
			}
			var calls []string
			for _, call := range o.calls {
				if call.name == "ifdown" || call.name == "ifup" {
					calls = append(calls, call.name+" "+strings.Join(call.args, " "))
				}
			}
			want := "ifdown --force -- eth0|ifup --force -- eth0 eth1"
			if activationFails {
				want += "|ifdown --force -- eth0 eth1|ifup --force -- eth0"
			}
			if strings.Join(calls, "|") != want {
				t.Fatalf("wrong old/new NIC operations: %v", calls)
			}
			if activationFails {
				data, _ := o.ReadFile(interfacesPath)
				if string(data) != original {
					t.Fatal("original not restored")
				}
			}
		})
	}
}

func TestInterfacesLogicalStateRestoredAfterFailedActivation(t *testing.T) {
	o := newFixtureOps(t, "interfaces")
	original := "iface home inet dhcp\n"
	o.put(interfacesPath, original, 0644)
	o.put("/run/network/ifstate", "eth0=home\n", 0644)
	activeLogical := "home"
	failedNew := false
	restoredOriginal := false
	o.response = func(name string, args []string) ([]byte, error) {
		if name != "ifdown" && name != "ifup" {
			return nil, nil
		}
		data, _ := o.ReadFile(interfacesPath)
		for _, requested := range args[2:] {
			nic, logical, explicit := strings.Cut(requested, "=")
			if !explicit {
				logical = nic
			}
			if name == "ifdown" && activeLogical != "" {
				logical = activeLogical
			}
			if !strings.Contains(string(data), "iface "+logical+" ") {
				return nil, fmt.Errorf("unknown interface %s", logical)
			}
			if name == "ifdown" {
				activeLogical = ""
				continue
			}
			if string(data) != original && !failedNew {
				failedNew = true
				return nil, errors.New("new activation failed")
			}
			activeLogical = logical
			if string(data) == original && logical == "home" {
				restoredOriginal = true
			}
		}
		return nil, nil
	}
	err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o)
	if err == nil {
		t.Fatal("activation failure unexpectedly succeeded")
	}
	if o.count("ifdown", "") == 0 && o.writes == 0 {
		return
	}
	if !restoredOriginal {
		t.Fatalf("original eth0=home did not reactivate after state was cleared: %v", err)
	}
}

func TestInterfacesActiveLogicalStateValidation(t *testing.T) {
	o := newFixtureOps(t, "interfaces")
	o.put(interfacesPath, "iface home inet dhcp\n", 0644)
	o.put("/run/network/ifstate", "eth0=home\n", 0644)
	if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err == nil {
		t.Fatal("unsupported logical state accepted")
	}
	if o.writes != 0 || o.count("ifdown", "") != 0 || o.count("ifup", "") != 0 {
		t.Fatal("unsupported logical state changed host")
	}
	o = newFixtureOps(t, "interfaces")
	o.put(interfacesPath, "iface eth0 inet dhcp\n", 0644)
	o.put("/run/network/ifstate", "eth0=eth0\n", 0644)
	if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err != nil {
		t.Fatal(err)
	}
	if o.count("ifdown", "") != 1 {
		t.Fatal("active original logical configuration was not stopped")
	}
	o = newFixtureOps(t, "interfaces")
	o.put(interfacesPath, "iface lo inet loopback\n", 0644)
	o.put("/run/network/ifstate", "eth0=missing\n", 0644)
	if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err == nil {
		t.Fatal("state with missing old definition accepted")
	}
	if o.writes != 0 || o.count("ifdown", "") != 0 {
		t.Fatal("unknown active old state changed host")
	}
}

func TestInterfacesIfreloadPreservesProxmoxBridgeAndIPv6(t *testing.T) {
	base := newFixtureOps(t, "ifreload")
	o := &ifreloadFixtureOps{fixtureOps: base}
	original := "auto lo vmbr0\n" +
		"iface lo inet loopback\n\n" +
		"iface eno1 inet manual\n\n" +
		"auto vmbr0\n" +
		"iface vmbr0 inet static\n" +
		"  address 192.0.2.9/24\n" +
		"  gateway 192.0.2.1\n" +
		"  bridge-ports eno1\n" +
		"  bridge-stp off\n" +
		"  bridge-fd 0\n" +
		"iface vmbr0 inet6 auto\n" +
		"  accept_ra 1\n" +
		"source /etc/network/interfaces.d/*\n"
	o.put(interfacesPath, original, 0640)
	vmbr0 := []client.InterfaceInfo{{
		Name:    "vmbr0",
		IPs:     []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}},
		Gateway: "192.0.2.254",
	}, {Name: "eno1"}}
	if err := ApplyNetworkSettingsWithOps(vmbr0, o); err != nil {
		t.Fatal(err)
	}
	data, err := o.ReadFile(interfacesPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"address 192.0.2.10/24",
		"gateway 192.0.2.254",
		"bridge-ports eno1",
		"bridge-stp off",
		"bridge-fd 0",
		"iface vmbr0 inet6 auto",
		"accept_ra 1",
		"source /etc/network/interfaces.d/*",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("lost %q from generated Proxmox config:\n%s", want, got)
		}
	}
	for _, call := range o.calls {
		if call.name == "ifdown" || call.name == "ifup" {
			t.Fatalf("ifreload path disrupted bridge with %s: %#v", call.name, o.calls)
		}
	}
	if o.count("ifreload", "-a") != 1 {
		t.Fatalf("expected one ifreload -a, calls: %#v", o.calls)
	}
}

func TestInterfacesIncludedNICStanzaUsesMultiFileTransaction(t *testing.T) {
	o := &ifreloadFixtureOps{fixtureOps: newFixtureOps(t, "ifreload")}
	root := "auto lo eth0\niface lo inet loopback\nsource /etc/network/interfaces.d/*\n"
	included := "auto eth0\niface eth0 inet dhcp\n  mtu 9000\n"
	o.put(interfacesPath, root, 0640)
	o.put("/etc/network/interfaces.d/10-eth0", included, 0600)
	if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err != nil {
		t.Fatal(err)
	}
	rootGot, err := o.ReadFile(interfacesPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(rootGot) != root {
		t.Fatalf("root include file changed:\n%s", rootGot)
	}
	data, err := o.ReadFile("/etc/network/interfaces.d/10-eth0")
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{"iface eth0 inet static", "address 192.0.2.10/24", "# boops-managed-address 192.0.2.11/24", "gateway 192.0.2.1", "dns-nameservers 1.1.1.1", "mtu 9000"} {
		if !strings.Contains(got, want) {
			t.Fatalf("included stanza lost %q:\n%s", want, got)
		}
	}
	info, err := os.Stat(o.local("/etc/network/interfaces.d/10-eth0"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("included mode changed: err=%v mode=%o", err, info.Mode().Perm())
	}
	if o.count("ifreload", "-a") != 1 || o.writes != 1 {
		t.Fatalf("expected one multi-file write and reload, writes=%d calls=%#v", o.writes, o.calls)
	}
}

func TestInterfacesIncludedNestedNICStanzaUsesMultiFileTransaction(t *testing.T) {
	o := &ifreloadFixtureOps{fixtureOps: newFixtureOps(t, "ifreload")}
	o.put(interfacesPath, "source-directory interfaces.d\n", 0644)
	o.put("/etc/network/interfaces.d/10-extra", "source nested\n", 0644)
	o.put("/etc/network/interfaces.d/nested", "auto eth1\niface eth1 inet dhcp\n", 0600)
	if err := ApplyNetworkSettingsWithOps(twoNICs()[1:], o); err != nil {
		t.Fatal(err)
	}
	data, err := o.ReadFile("/etc/network/interfaces.d/nested")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "address 198.51.100.10/24") {
		t.Fatalf("nested included NIC was not updated:\n%s", data)
	}
}

func TestInterfacesIfreloadFailureRestoresEveryIncludedFile(t *testing.T) {
	o := &ifreloadFixtureOps{fixtureOps: newFixtureOps(t, "ifreload")}
	root := "source /etc/network/interfaces.d/*\n"
	included := "auto eth0\niface eth0 inet dhcp\n  mtu 9000\n"
	o.put(interfacesPath, root, 0640)
	o.put("/etc/network/interfaces.d/10-eth0", included, 0600)
	failed := false
	o.fail = func(name string, _ []string) bool {
		if name == "ifreload" && !failed {
			failed = true
			return true
		}
		return false
	}
	if err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o); err == nil {
		t.Fatal("ifreload failure ignored")
	}
	for path, want := range map[string]string{interfacesPath: root, "/etc/network/interfaces.d/10-eth0": included} {
		data, err := o.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("%s was not restored:\n%s", path, data)
		}
	}
	if o.count("ifreload", "-a") != 2 {
		t.Fatalf("expected replacement and restore reload, calls=%#v", o.calls)
	}
}

func TestInterfacesOwnRequestedNICsScansIncludes(t *testing.T) {
	o := newFixtureOps(t, "interfaces")
	o.put(interfacesPath, "iface lo inet loopback\nsource /etc/network/interfaces.d/*\n", 0644)
	o.put("/etc/network/interfaces.d/10-eth0", "iface eth0 inet dhcp\n", 0644)
	owned, err := interfacesOwnRequestedNICs(twoNICs()[:1], o)
	if err != nil {
		t.Fatal(err)
	}
	if !owned {
		t.Fatal("included eth0 definition was not detected")
	}
	o = newFixtureOps(t, "interfaces")
	o.put(interfacesPath, "iface lo inet loopback\n", 0644)
	owned, err = interfacesOwnRequestedNICs(twoNICs()[:1], o)
	if err != nil {
		t.Fatal(err)
	}
	if owned {
		t.Fatal("unrelated loopback definition claimed eth0")
	}
	o = newFixtureOps(t, "interfaces")
	o.put(interfacesPath, "auto vmbr0\niface vmbr0 inet static\n  bridge-ports ens18\n", 0644)
	owned, err = interfacesOwnRequestedNICs([]client.InterfaceInfo{{Name: "ens18"}}, o)
	if err != nil {
		t.Fatal(err)
	}
	if !owned {
		t.Fatal("bridge member referenced by an existing ifupdown topology was not detected")
	}
}

func TestInterfacesDoesNotCreateStandaloneDependencyStanza(t *testing.T) {
	o := &ifreloadFixtureOps{fixtureOps: newFixtureOps(t, "ifreload")}
	original := "auto vmbr0\niface vmbr0 inet static\n  bridge-ports ens18\n"
	o.put(interfacesPath, original, 0644)
	if err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{
		Name: "ens18",
		IPs:  []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}},
	}}, o); err == nil {
		t.Fatal("bridge member without an own stanza received a standalone configuration")
	}
	data, err := o.ReadFile(interfacesPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original || o.writes != 0 || o.count("ifreload", "") != 0 {
		t.Fatalf("dependency topology changed: data=%q writes=%d calls=%#v", data, o.writes, o.calls)
	}
}

func TestClassicInterfacesRollbackPreservesInactiveNIC(t *testing.T) {
	for _, failure := range []string{"down", "write", "up"} {
		t.Run(failure, func(t *testing.T) {
			o := newFixtureOps(t, "interfaces")
			original := "iface eth0 inet static\n  address 192.0.2.9/24\n"
			o.put(interfacesPath, original, 0640)
			o.put("/run/network/ifstate", "", 0644)
			if failure == "write" {
				o.failWritePath = interfacesPath
			}
			failed := false
			o.fail = func(name string, _ []string) bool {
				if !failed && ((failure == "down" && name == "ifdown") || (failure == "up" && name == "ifup")) {
					failed = true
					return true
				}
				return false
			}
			err := ApplyNetworkSettingsWithOps(twoNICs()[:1], o)
			if err == nil {
				t.Fatal("failure ignored")
			}
			wantUps := 0
			if failure == "up" {
				wantUps = 1
			}
			if o.count("ifup", "") != wantUps {
				t.Fatalf("rollback activated an originally inactive NIC: %#v", o.calls)
			}
			got, _ := o.ReadFile(interfacesPath)
			if string(got) != original {
				t.Fatal("original bytes not restored")
			}
		})
	}
}
