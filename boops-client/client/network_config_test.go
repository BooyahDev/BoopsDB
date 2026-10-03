package client

import (
	"reflect"
	"testing"
)

func networkInterfaces() []InterfaceInfo {
	return []InterfaceInfo{
		{Name: "eth0", IPs: []IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0", DNSRegister: 1}, {IP: "192.0.2.11", Subnet: "255.255.255.0"}}, Gateway: "192.0.2.1", DnsServers: "1.1.1.1, 8.8.8.8", MacAddress: "02:00:00:00:00:01"},
		{Name: "eth1", IPs: []IPInfo{{IP: "198.51.100.10", Subnet: "255.255.255.0"}}, Gateway: " 0.0.0.0 "},
	}
}

func TestNormalizeInterfacesSingleGateway(t *testing.T) {
	in := networkInterfaces()
	got, err := NormalizeInterfaces(in)
	if err != nil || got[1].Gateway != "" || got[0].DnsServers != "1.1.1.1,8.8.8.8" {
		t.Fatalf("unexpected normalization: %#v %v", got, err)
	}
	got[0].IPs[0].IP = "192.0.2.20"
	if in[0].IPs[0].IP != "192.0.2.10" || in[1].Gateway != " 0.0.0.0 " {
		t.Fatal("normalization mutated input")
	}
	for _, gateway := range []string{"", " ", "0.0.0.0"} {
		in[0].Gateway = gateway
		if got, err := NormalizeInterfaces(in); err != nil || got[0].Gateway != "" {
			t.Fatalf("gateway %q: %v %v", gateway, got, err)
		}
	}
}

func TestNormalizeInterfacesRejectsBeforeApply(t *testing.T) {
	tests := []struct {
		name string
		edit func([]InterfaceInfo)
	}{
		{"two gateways", func(i []InterfaceInfo) { i[1].Gateway = "198.51.100.1" }},
		{"duplicate name", func(i []InterfaceInfo) { i[1].Name = "eth0" }},
		{"empty name", func(i []InterfaceInfo) { i[1].Name = " " }},
		{"path name", func(i []InterfaceInfo) { i[1].Name = "../eth1" }},
		{"no IP", func(i []InterfaceInfo) { i[1].IPs = nil }},
		{"IPv6", func(i []InterfaceInfo) { i[1].IPs[0].IP = "2001:db8::1" }},
		{"noncontiguous mask", func(i []InterfaceInfo) { i[1].IPs[0].Subnet = "255.0.255.0" }},
		{"invalid DNS", func(i []InterfaceInfo) { i[1].DnsServers = "1.1.1.1,broken" }},
		{"invalid MAC", func(i []InterfaceInfo) { i[1].MacAddress = "eth1" }},
		{"invalid gateway", func(i []InterfaceInfo) { i[1].Gateway = "invalid" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := networkInterfaces()
			tt.edit(in)
			if _, err := NormalizeInterfaces(in); err == nil {
				t.Fatal("invalid interfaces accepted")
			}
		})
	}
}

func TestInterfacesEqualUsesNameAndAllFields(t *testing.T) {
	a := networkInterfaces()
	b := networkInterfaces()
	b[0].IPs[0], b[0].IPs[1] = b[0].IPs[1], b[0].IPs[0]
	b[0], b[1] = b[1], b[0]
	if !InterfacesEqual(a, b) {
		t.Fatal("display order caused reapply")
	}
	for _, tt := range []struct {
		name string
		edit func([]InterfaceInfo)
	}{
		{"name", func(i []InterfaceInfo) { i[0].Name = "new-name" }},
		{"MAC", func(i []InterfaceInfo) { i[0].MacAddress = "02:00:00:00:00:02" }},
		{"DNS", func(i []InterfaceInfo) { i[0].DnsServers = "9.9.9.9" }},
		{"dns_register", func(i []InterfaceInfo) { i[0].IPs[0].DNSRegister = 0 }},
		{"gateway", func(i []InterfaceInfo) { i[0].Gateway = "192.0.2.2" }},
		{"subnet", func(i []InterfaceInfo) { i[0].IPs[0].Subnet = "255.255.0.0" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := networkInterfaces()
			tt.edit(b)
			if InterfacesEqual(a, b) {
				t.Fatal("changed field ignored")
			}
		})
	}
	if !reflect.DeepEqual(a, networkInterfaces()) {
		t.Fatal("comparator mutated input")
	}
}
