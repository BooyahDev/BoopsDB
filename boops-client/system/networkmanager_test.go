package system

import (
	"errors"
	"strings"
	"testing"
)

func nmFixture(t *testing.T) *fixtureOps {
	o := newFixtureOps(t, "nmcli")
	o.response = func(n string, a []string) ([]byte, error) {
		if n != "nmcli" {
			return nil, nil
		}
		s := strings.Join(a, " ")
		if strings.Contains(s, "connection show --active") {
			return []byte("uuid-0:eth0\nuuid-1:eth1\n"), nil
		}
		if len(a) > 2 && a[0] == "--escape" {
			return nil, errors.New("unexpected fixture call")
		}
		if strings.Contains(s, "connection show uuid") {
			switch a[1] {
			case "ipv4.method":
				return []byte("manual\n"), nil
			case "ipv4.addresses":
				return []byte("192.0.2.9/24\n"), nil
			case "ipv4.gateway":
				return []byte("192.0.2.254\n"), nil
			case "ipv4.dns":
				return []byte("8.8.8.8\n"), nil
			case "ipv4.never-default":
				return []byte("no\n"), nil
			case "ipv4.routes":
				return []byte("{ ip = 0.0.0.0/0, nh = 192.0.2.254, mt = 10 }; { ip = 203.0.113.0/24, nh = 192.0.2.5, mt = 50, table=100 }\n"), nil
			}
		}
		return nil, nil
	}
	return o
}

func TestNmcliClearsGatewayAndKeepsStaticRoutes(t *testing.T) {
	o := nmFixture(t)
	if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err != nil {
		t.Fatal(err)
	}
	var mods [][]string
	for _, c := range o.calls {
		if c.name == "nmcli" && len(c.args) > 1 && c.args[0] == "connection" && c.args[1] == "modify" {
			mods = append(mods, c.args)
		}
	}
	if len(mods) != 2 {
		t.Fatalf("expected two modifications: %#v", o.calls)
	}
	for _, m := range mods {
		route := ""
		for i := range m {
			if m[i] == "ipv4.routes" {
				route = m[i+1]
			}
		}
		if strings.Contains(route, "0.0.0.0/0") || !strings.Contains(route, "203.0.113.0/24") || !strings.Contains(route, "table=100") {
			t.Fatalf("routes not preserved: %#v", m)
		}
	}
	found := false
	for _, m := range mods {
		if m[3] == "uuid-1" {
			for i := range m {
				if m[i] == "ipv4.gateway" && m[i+1] == "" {
					found = true
				}
				if m[i] == "ipv4.never-default" && m[i+1] != "yes" {
					t.Fatal("default route not disabled")
				}
			}
		}
	}
	if !found {
		t.Fatalf("gateway not explicitly cleared: %#v", mods)
	}
}

func TestNmcliFailureRestoresAllChangedConnections(t *testing.T) {
	o := nmFixture(t)
	failed := false
	o.fail = func(n string, a []string) bool {
		if !failed && n == "nmcli" && strings.HasPrefix(strings.Join(a, " "), "connection up uuid uuid-1") {
			failed = true
			return true
		}
		return false
	}
	if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err == nil {
		t.Fatal("failure ignored")
	}
	mods := 0
	for _, c := range o.calls {
		if c.name == "nmcli" && len(c.args) > 1 && c.args[0] == "connection" && c.args[1] == "modify" {
			mods++
		}
	}
	if mods != 4 {
		t.Fatalf("connections not restored: %d", mods)
	}
}

func TestNmcliPreflightReadsEveryNICBeforeModification(t *testing.T) {
	o := nmFixture(t)
	o.fail = func(n string, a []string) bool {
		return n == "nmcli" && strings.Contains(strings.Join(a, " "), "-g ipv4.routes connection show uuid uuid-1")
	}
	if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err == nil {
		t.Fatal("failed snapshot accepted")
	}
	for _, call := range o.calls {
		if call.name == "nmcli" && len(call.args) > 1 && call.args[0] == "connection" && (call.args[1] == "modify" || call.args[1] == "up") {
			t.Fatal("changed connections before complete preflight")
		}
	}
}
