package client

import (
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
	"unicode"
)

// NormalizeInterfaces validates the whole request without changing the input.
func NormalizeInterfaces(ifaces []InterfaceInfo) ([]InterfaceInfo, error) {
	result := make([]InterfaceInfo, len(ifaces))
	names := make(map[string]bool, len(ifaces))
	var gatewayNames []string
	for i, info := range ifaces {
		info.Name = strings.TrimSpace(info.Name)
		if info.Name == "" || strings.ContainsAny(info.Name, "/\\\"'") || strings.IndexFunc(info.Name, unicode.IsControl) >= 0 {
			return nil, fmt.Errorf("invalid NIC name %q", info.Name)
		}
		if names[info.Name] {
			return nil, fmt.Errorf("duplicate NIC name %q", info.Name)
		}
		names[info.Name] = true
		info.IPs = append([]IPInfo(nil), info.IPs...)
		for j := range info.IPs {
			ip, err := parseIPv4(info.IPs[j].IP)
			if err != nil {
				return nil, fmt.Errorf("NIC %s address: %w", info.Name, err)
			}
			mask, err := parseIPv4(info.IPs[j].Subnet)
			if err != nil {
				return nil, fmt.Errorf("NIC %s mask: %w", info.Name, err)
			}
			bytes := mask.As4()
			if _, bits := net.IPMask(bytes[:]).Size(); bits != 32 {
				return nil, fmt.Errorf("NIC %s has non-contiguous subnet mask %s", info.Name, mask)
			}
			if info.IPs[j].DNSRegister != 0 && info.IPs[j].DNSRegister != 1 {
				return nil, fmt.Errorf("NIC %s has invalid dns_register", info.Name)
			}
			info.IPs[j].IP, info.IPs[j].Subnet = ip.String(), mask.String()
		}
		info.Gateway = strings.TrimSpace(info.Gateway)
		if info.Gateway == "0.0.0.0" {
			info.Gateway = ""
		}
		if info.Gateway != "" {
			gateway, err := parseIPv4(info.Gateway)
			if err != nil {
				return nil, fmt.Errorf("NIC %s gateway: %w", info.Name, err)
			}
			info.Gateway = gateway.String()
			gatewayNames = append(gatewayNames, info.Name)
		}
		var dns []string
		for _, raw := range strings.Split(info.DnsServers, ",") {
			if strings.TrimSpace(raw) == "" {
				continue
			}
			addr, err := parseIPv4(raw)
			if err != nil {
				return nil, fmt.Errorf("NIC %s DNS: %w", info.Name, err)
			}
			dns = append(dns, addr.String())
		}
		info.DnsServers = strings.Join(dns, ",")
		if len(info.IPs) == 0 && (info.Gateway != "" || info.DnsServers != "") {
			return nil, fmt.Errorf("NIC %s has a gateway or DNS without IPv4 addresses", info.Name)
		}
		info.MacAddress = strings.TrimSpace(info.MacAddress)
		if info.MacAddress != "" {
			mac, err := net.ParseMAC(info.MacAddress)
			if err != nil || len(mac) != 6 {
				return nil, fmt.Errorf("NIC %s has invalid MAC %q", info.Name, info.MacAddress)
			}
			info.MacAddress = mac.String()
		}
		result[i] = info
	}
	if len(gatewayNames) > 1 {
		return nil, fmt.Errorf("multiple default gateways on NICs: %s", strings.Join(gatewayNames, ", "))
	}
	return result, nil
}

func parseIPv4(raw string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil || !addr.Is4() {
		return netip.Addr{}, fmt.Errorf("invalid IPv4 address %q", raw)
	}
	return addr, nil
}

// InterfacesEqual compares NIC names and settings, ignoring NIC/IP display order.
func InterfacesEqual(a, b []InterfaceInfo) bool {
	if len(a) != len(b) {
		return false
	}
	a, err := NormalizeInterfaces(a)
	if err != nil {
		return false
	}
	b, err = NormalizeInterfaces(b)
	if err != nil {
		return false
	}
	bNames := make(map[string]InterfaceInfo, len(b))
	for _, info := range b {
		bNames[info.Name] = info
	}
	for _, info := range a {
		other, ok := bNames[info.Name]
		if !ok || info.Gateway != other.Gateway || info.DnsServers != other.DnsServers || info.MacAddress != other.MacAddress || len(info.IPs) != len(other.IPs) {
			return false
		}
		keys := func(ips []IPInfo) []string {
			out := make([]string, len(ips))
			for i, ip := range ips {
				out[i] = fmt.Sprintf("%s/%s/%d", ip.IP, ip.Subnet, ip.DNSRegister)
			}
			sort.Strings(out)
			return out
		}
		x, y := keys(info.IPs), keys(other.IPs)
		for i := range x {
			if x[i] != y[i] {
				return false
			}
		}
	}
	return true
}
