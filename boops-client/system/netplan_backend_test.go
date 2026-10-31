package system

import (
	"boops/client"
	"strings"
	"testing"
)

const raspiosNetworkManagerNetplan = `network:
  version: 2
  ethernets:
    eth0:
      renderer: NetworkManager
      match: {}
      dhcp4: true
      dhcp6: true
      networkmanager:
        uuid: "uuid-eth0"
        name: "netplan-eth0"
`

func assertNoNetworkManagerMutation(t *testing.T, o *fixtureOps) {
	t.Helper()
	for _, call := range o.calls {
		if call.name != "nmcli" || len(call.args) < 2 || call.args[0] != "connection" {
			continue
		}
		if call.args[1] == "modify" || call.args[1] == "up" || call.args[1] == "down" {
			t.Fatalf("NetworkManager mutation occurred: %s", strings.Join(call.args, " "))
		}
	}
}

func TestNetworkManagerNetplanEmptyMatchUsesVerifiedProfile(t *testing.T) {
	o := newFixtureOps(t, "netplan-nmcli")
	o.put("/etc/netplan/90-NM-uuid-eth0.yaml", raspiosNetworkManagerNetplan, 0600)
	o.response = func(name string, args []string) ([]byte, error) {
		if name == "nmcli" && strings.Join(args, " ") == "-t --escape no -f UUID,DEVICE connection show --active" {
			return []byte("uuid-eth0:eth0\n"), nil
		}
		return nil, nil
	}
	before, _ := o.ReadFile("/etc/netplan/90-NM-uuid-eth0.yaml")
	err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{
		Name:    "eth0",
		IPs:     []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}},
		Gateway: "192.0.2.1", DnsServers: "192.0.2.53,192.0.2.54",
	}}, o)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := o.ReadFile("/etc/netplan/90-NM-uuid-eth0.yaml")
	if string(after) != string(before) {
		t.Fatal("NetworkManager Netplan YAML was rewritten")
	}
	if o.count("netplan", "apply") != 0 {
		t.Fatal("Netplan apply was used for a NetworkManager profile")
	}
	if o.count("nmcli", "connection") == 0 {
		t.Fatal("NetworkManager profile was not updated")
	}
}

func TestNetworkManagerNetplanUUIDMismatchFailsClosed(t *testing.T) {
	o := newFixtureOps(t, "netplan-nmcli")
	yaml := strings.Replace(raspiosNetworkManagerNetplan, "uuid-eth0", "uuid-yaml", 1)
	o.put("/etc/netplan/90-NM-uuid-eth0.yaml", yaml, 0600)
	o.response = func(name string, args []string) ([]byte, error) {
		if name == "nmcli" && strings.Join(args, " ") == "-t --escape no -f UUID,DEVICE connection show --active" {
			return []byte("uuid-live:eth0\n"), nil
		}
		return nil, nil
	}
	err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{Name: "eth0", IPs: []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}}}}, o)
	if err == nil || !strings.Contains(err.Error(), "does not match active UUID") {
		t.Fatalf("unexpected result: %v", err)
	}
	if o.writes != 0 || o.count("netplan", "apply") != 0 {
		t.Fatal("UUID mismatch changed the host")
	}
	assertNoNetworkManagerMutation(t, o)
}

func TestNetworkManagerNetplanMissingUUIDFailsClosed(t *testing.T) {
	o := newFixtureOps(t, "netplan-nmcli")
	yaml := strings.Replace(raspiosNetworkManagerNetplan, "        uuid: \"uuid-eth0\"\n", "", 1)
	o.put("/etc/netplan/90-NM-uuid-eth0.yaml", yaml, 0600)
	o.response = func(name string, args []string) ([]byte, error) {
		if name == "nmcli" && strings.Join(args, " ") == "-t --escape no -f UUID,DEVICE connection show --active" {
			return []byte("uuid-live:eth0\n"), nil
		}
		return nil, nil
	}
	err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{Name: "eth0", IPs: []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}}}}, o)
	if err == nil || !strings.Contains(err.Error(), "has no UUID") {
		t.Fatalf("unexpected result: %v", err)
	}
	if o.writes != 0 || o.count("netplan", "apply") != 0 {
		t.Fatal("missing UUID changed the host")
	}
	assertNoNetworkManagerMutation(t, o)
}

func TestMixedNetworkManagerAndNetworkdRequestsFailClosed(t *testing.T) {
	o := newFixtureOps(t, "netplan-nmcli")
	yaml := raspiosNetworkManagerNetplan + `    eth1:
      renderer: networkd
      dhcp4: true
`
	o.put("/etc/netplan/90-NM-uuid-eth0.yaml", yaml, 0600)
	o.response = func(name string, args []string) ([]byte, error) {
		if name == "nmcli" && strings.Join(args, " ") == "-t --escape no -f UUID,DEVICE connection show --active" {
			return []byte("uuid-eth0:eth0\nuuid-eth1:eth1\n"), nil
		}
		return nil, nil
	}
	err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{
		{Name: "eth0", IPs: []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}}},
		{Name: "eth1", IPs: []client.IPInfo{{IP: "192.0.2.2", Subnet: "255.255.255.0"}}},
	}, o)
	if err == nil || !strings.Contains(err.Error(), "mixed Netplan and NetworkManager") {
		t.Fatalf("unexpected result: %v", err)
	}
	if o.writes != 0 || o.count("netplan", "apply") != 0 {
		t.Fatal("mixed backend request changed the host")
	}
	assertNoNetworkManagerMutation(t, o)
}

func TestSameNICNetworkManagerAndNetworkdDefinitionsFailClosed(t *testing.T) {
	o := newFixtureOps(t, "netplan-nmcli")
	config := raspiosNetworkManagerNetplan + `    wired:
      renderer: networkd
      match: {name: eth0}
      dhcp4: true
`
	o.put("/etc/netplan/90-NM-uuid-eth0.yaml", config, 0600)
	o.response = func(name string, args []string) ([]byte, error) {
		if name == "nmcli" && strings.Join(args, " ") == "-t --escape no -f UUID,DEVICE connection show --active" {
			return []byte("uuid-eth0:eth0\n"), nil
		}
		return nil, nil
	}
	err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{Name: "eth0", IPs: []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}}}}, o)
	if err == nil || !strings.Contains(err.Error(), "matches both NetworkManager and non-NetworkManager") {
		t.Fatalf("unexpected result: %v", err)
	}
	if o.writes != 0 || o.count("netplan", "apply") != 0 {
		t.Fatal("ambiguous ownership changed the host")
	}
	assertNoNetworkManagerMutation(t, o)
}

func TestNetworkManagerNetplanMissingActiveProfileFailsClosed(t *testing.T) {
	o := newFixtureOps(t, "netplan-nmcli")
	o.put("/etc/netplan/90-NM-uuid-eth0.yaml", raspiosNetworkManagerNetplan, 0600)
	o.response = func(name string, args []string) ([]byte, error) {
		if name == "nmcli" && strings.Join(args, " ") == "-t --escape no -f UUID,DEVICE connection show --active" {
			return []byte("uuid-other:eth1\n"), nil
		}
		return nil, nil
	}
	err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{Name: "eth0", IPs: []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}}}}, o)
	if err == nil || !strings.Contains(err.Error(), "has no active UUID") {
		t.Fatalf("unexpected result: %v", err)
	}
	if o.writes != 0 || o.count("netplan", "apply") != 0 {
		t.Fatal("missing active profile changed the host")
	}
	assertNoNetworkManagerMutation(t, o)
}

func TestNetworkManagerNetplanLeavesUnrequestedWiFiUntouched(t *testing.T) {
	o := newFixtureOps(t, "netplan-nmcli")
	yaml := raspiosNetworkManagerNetplan + `  wifis:
    wlan0:
      renderer: NetworkManager
      match: {}
      dhcp4: true
      networkmanager:
        uuid: "uuid-wlan0"
        name: "netplan-wlan0"
`
	o.put("/etc/netplan/90-NM-uuid-eth0.yaml", yaml, 0600)
	o.response = func(name string, args []string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "nmcli" && joined == "-t --escape no -f UUID,DEVICE connection show --active" {
			return []byte("uuid-eth0:eth0\nuuid-wlan0:wlan0\n"), nil
		}
		if name == "nmcli" && strings.HasPrefix(joined, "-g ") && strings.HasSuffix(joined, "connection show uuid uuid-eth0") {
			switch args[1] {
			case "ipv4.method":
				return []byte("auto\n"), nil
			case "ipv4.addresses":
				return []byte("192.0.2.9/24\n"), nil
			case "ipv4.gateway":
				return []byte("192.0.2.1\n"), nil
			case "ipv4.dns":
				return []byte("192.0.2.53\n"), nil
			case "ipv4.never-default":
				return []byte("no\n"), nil
			case "ipv4.routes":
				return []byte("192.0.2.0/24 0.0.0.0 100\n"), nil
			}
		}
		return nil, nil
	}
	err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{
		Name:    "eth0",
		IPs:     []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}},
		Gateway: "192.0.2.1", DnsServers: "192.0.2.53,192.0.2.54",
	}}, o)
	if err != nil {
		t.Fatal(err)
	}
	var ethModify []string
	for _, call := range o.calls {
		if call.name != "nmcli" {
			continue
		}
		joined := strings.Join(call.args, " ")
		if strings.HasPrefix(joined, "connection modify uuid ") {
			if strings.Contains(joined, "uuid-wlan0") {
				t.Fatalf("unrequested Wi-Fi profile was modified: %s", joined)
			}
			if strings.Contains(joined, "uuid-eth0") {
				ethModify = call.args
			}
		}
		if strings.HasPrefix(joined, "connection up uuid ") && strings.Contains(joined, "uuid-wlan0") {
			t.Fatalf("unrequested Wi-Fi profile was activated: %s", joined)
		}
	}
	if len(ethModify) == 0 {
		t.Fatal("eth0 NetworkManager profile was not modified")
	}
	joined := strings.Join(ethModify, " ")
	for _, want := range []string{
		"ipv4.method manual",
		"ipv4.addresses 192.0.2.10/24",
		"ipv4.gateway 192.0.2.1",
		"ipv4.dns 192.0.2.53,192.0.2.54",
		"ipv4.never-default no",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("eth0 desired setting %q missing: %s", want, joined)
		}
	}
}

func TestNetworkdRequestIgnoresUnrelatedNetworkManagerProfile(t *testing.T) {
	o := newFixtureOps(t, "netplan")
	yaml := `network:
  version: 2
  ethernets:
    eth0:
      renderer: networkd
      dhcp4: true
  wifis:
    wlan0:
      renderer: NetworkManager
      match: {}
      dhcp4: true
      networkmanager:
        uuid: "uuid-wlan0"
        name: "netplan-wlan0"
`
	o.put("/etc/netplan/90-network.yaml", yaml, 0600)
	err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{
		Name:    "eth0",
		IPs:     []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}},
		Gateway: "192.0.2.1", DnsServers: "192.0.2.53",
	}}, o)
	if err != nil {
		t.Fatal(err)
	}
	if o.count("netplan", "apply") != 1 {
		t.Fatalf("networkd request did not use Netplan apply: %#v", o.calls)
	}
	for _, call := range o.calls {
		if call.name == "nmcli" {
			t.Fatalf("unrelated NetworkManager profile caused nmcli use: %#v", call)
		}
	}
	updated, _ := o.ReadFile("/etc/netplan/90-network.yaml")
	if !strings.Contains(string(updated), "192.0.2.10/24") || !strings.Contains(string(updated), "uuid-wlan0") {
		t.Fatalf("networkd update lost expected content: %s", updated)
	}
}

func TestNetworkdRequestRejectsUnrelatedGenericEmptyMatch(t *testing.T) {
	o := newFixtureOps(t, "netplan")
	yaml := `network:
  version: 2
  ethernets:
    eth0:
      renderer: networkd
      dhcp4: true
    eth1:
      renderer: networkd
      match: {}
      dhcp4: true
`
	o.put("/etc/netplan/90-network.yaml", yaml, 0600)
	err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{
		Name: "eth0",
		IPs:  []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}},
	}}, o)
	if err == nil || !strings.Contains(err.Error(), "empty Netplan match for eth1") {
		t.Fatalf("unexpected result: %v", err)
	}
	if o.writes != 0 || o.count("netplan", "apply") != 0 {
		t.Fatal("generic empty match changed the host")
	}
}

func TestNetworkManagerNetplanDuplicateUUIDFailsBeforeMutation(t *testing.T) {
	o := newFixtureOps(t, "netplan-nmcli")
	yaml := `network:
  version: 2
  ethernets:
    eth0:
      renderer: NetworkManager
      match: {}
      networkmanager: {uuid: "uuid-shared"}
    eth1:
      renderer: NetworkManager
      match: {}
      networkmanager: {uuid: "uuid-shared"}
`
	o.put("/etc/netplan/90-NM-duplicate.yaml", yaml, 0600)
	o.response = func(name string, args []string) ([]byte, error) {
		if name == "nmcli" && strings.Join(args, " ") == "-t --escape no -f UUID,DEVICE connection show --active" {
			return []byte("uuid-shared:eth0\nuuid-other:eth1\n"), nil
		}
		return nil, nil
	}
	err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{Name: "eth0", IPs: []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}}}}, o)
	if err == nil || !strings.Contains(err.Error(), "ambiguous NetworkManager Netplan UUID") {
		t.Fatalf("unexpected result: %v", err)
	}
	if o.writes != 0 || o.count("netplan", "apply") != 0 {
		t.Fatal("duplicate UUID changed the host")
	}
	assertNoNetworkManagerMutation(t, o)
}

func TestNetworkManagerNetplanMalformedRendererFailsBeforeMutation(t *testing.T) {
	o := newFixtureOps(t, "netplan-nmcli")
	yaml := strings.Replace(raspiosNetworkManagerNetplan, "renderer: NetworkManager", "renderer: {bad: true}", 1)
	o.put("/etc/netplan/90-NM-malformed.yaml", yaml, 0600)
	err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{Name: "eth0", IPs: []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}}}}, o)
	if err == nil || !strings.Contains(err.Error(), "renderer for eth0 must be scalar") {
		t.Fatalf("unexpected result: %v", err)
	}
	if o.writes != 0 || o.count("netplan", "apply") != 0 {
		t.Fatal("malformed renderer changed the host")
	}
	assertNoNetworkManagerMutation(t, o)
}

func TestNetworkManagerNetplanGlobalRendererUsesVerifiedProfile(t *testing.T) {
	o := newFixtureOps(t, "netplan-nmcli")
	yaml := `network:
  version: 2
  renderer: NetworkManager
  ethernets:
    eth0:
      match: {}
      networkmanager: {uuid: "uuid-eth0"}
`
	o.put("/etc/netplan/90-NM-global.yaml", yaml, 0600)
	o.response = func(name string, args []string) ([]byte, error) {
		if name == "nmcli" && strings.Join(args, " ") == "-t --escape no -f UUID,DEVICE connection show --active" {
			return []byte("uuid-eth0:eth0\n"), nil
		}
		return nil, nil
	}
	err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{Name: "eth0", IPs: []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}}}}, o)
	if err != nil {
		t.Fatal(err)
	}
	if o.count("netplan", "apply") != 0 {
		t.Fatal("global NetworkManager renderer used Netplan apply")
	}
}

func TestNetworkManagerNetplanOpaqueIDUsesActiveUUID(t *testing.T) {
	o := newFixtureOps(t, "netplan-nmcli")
	yaml := `network:
  version: 2
  ethernets:
    NM-opaque-eth0:
      renderer: NetworkManager
      match: {}
      networkmanager: {uuid: "uuid-eth0"}
`
	o.put("/etc/netplan/90-NM-opaque.yaml", yaml, 0600)
	o.response = func(name string, args []string) ([]byte, error) {
		if name == "nmcli" && strings.Join(args, " ") == "-t --escape no -f UUID,DEVICE connection show --active" {
			return []byte("uuid-eth0:eth0\n"), nil
		}
		return nil, nil
	}
	err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{Name: "eth0", IPs: []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}}}}, o)
	if err != nil {
		t.Fatal(err)
	}
	if o.count("netplan", "apply") != 0 || o.count("nmcli", "connection") == 0 {
		t.Fatalf("opaque NetworkManager profile did not use verified nmcli path: %#v", o.calls)
	}
}

func TestNetworkManagerNetplanNMCLIFailureDoesNotMutate(t *testing.T) {
	o := newFixtureOps(t, "netplan-nmcli")
	o.put("/etc/netplan/90-NM-uuid-eth0.yaml", raspiosNetworkManagerNetplan, 0600)
	o.fail = func(name string, args []string) bool {
		return name == "nmcli" && strings.Join(args, " ") == "-t --escape no -f UUID,DEVICE connection show --active"
	}
	err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{Name: "eth0", IPs: []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}}}}, o)
	if err == nil {
		t.Fatal("nmcli failure was ignored")
	}
	if o.writes != 0 || o.count("netplan", "apply") != 0 {
		t.Fatal("nmcli preflight failure changed the host")
	}
	assertNoNetworkManagerMutation(t, o)
}

func TestNetworkManagerNetplanSelectionChangeFailsBeforeMutation(t *testing.T) {
	o := newFixtureOps(t, "netplan-nmcli")
	o.put("/etc/netplan/90-NM-uuid-eth0.yaml", raspiosNetworkManagerNetplan, 0600)
	activeReads := 0
	o.response = func(name string, args []string) ([]byte, error) {
		if name == "nmcli" && strings.Join(args, " ") == "-t --escape no -f UUID,DEVICE connection show --active" {
			activeReads++
			if activeReads == 1 {
				return []byte("uuid-eth0:eth0\n"), nil
			}
			return []byte("uuid-other:eth0\n"), nil
		}
		return nil, nil
	}
	err := ApplyNetworkSettingsWithOps([]client.InterfaceInfo{{Name: "eth0", IPs: []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}}}}, o)
	if err == nil || !strings.Contains(err.Error(), "changed from uuid-eth0") {
		t.Fatalf("unexpected result: %v", err)
	}
	if o.writes != 0 || o.count("netplan", "apply") != 0 {
		t.Fatal("changed active selection mutated the host")
	}
	assertNoNetworkManagerMutation(t, o)
}
