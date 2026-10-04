package system

import (
	"strings"
	"testing"
)

func TestWindowsExplicitGatewayNoneAndAllCommandsChecked(t *testing.T) {
	o := newFixtureOps(t, "netsh")
	o.osName = "windows"
	if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range o.calls {
		if c.name == "netsh" && strings.Contains(strings.Join(c.args, " "), "set address name=eth1") {
			if strings.Contains(strings.Join(c.args, " "), "gateway=none") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("gateway none missing: %#v", o.calls)
	}
	o = newFixtureOps(t, "netsh")
	o.osName = "windows"
	failed := false
	o.fail = func(n string, a []string) bool {
		if !failed && strings.Contains(strings.Join(a, " "), "add address") {
			failed = true
			return true
		}
		return false
	}
	if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err == nil {
		t.Fatal("netsh failure ignored")
	}
}

func TestWindowsMissingSecondNICPreventsBackupAndMutation(t *testing.T) {
	o := newFixtureOps(t, "netsh")
	o.osName = "windows"
	o.fail = func(n string, a []string) bool {
		return n == "netsh" && strings.Contains(strings.Join(a, " "), "show addresses name=eth1")
	}
	if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err == nil {
		t.Fatal("missing NIC accepted")
	}
	if o.writes != 0 {
		t.Fatal("wrote before all NIC validation")
	}
	for _, call := range o.calls {
		if strings.Contains(strings.Join(call.args, " "), "set address") {
			t.Fatal("changed NIC before preflight")
		}
	}
}

func TestWindowsFailedCommandReplaysBackup(t *testing.T) {
	o := newFixtureOps(t, "netsh")
	o.osName = "windows"
	failed := false
	restored := false
	o.fail = func(n string, a []string) bool {
		if n == "netsh" && !failed && strings.Contains(strings.Join(a, " "), "set dnsservers") {
			failed = true
			return true
		}
		return false
	}
	o.response = func(n string, a []string) ([]byte, error) {
		if n == "netsh" && len(a) == 2 && a[0] == "-f" {
			data, err := o.ReadFile(a[1])
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != "# original netsh configuration\n" {
				t.Fatal("backup script changed")
			}
			restored = true
		}
		return nil, nil
	}
	if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err == nil {
		t.Fatal("failure ignored")
	}
	if !restored {
		t.Fatal("backup not replayed")
	}
}
