package system

import (
	"boops/client"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
)

// ANSI color codes for styled output
const (
	Reset       = "\033[0m"
	Bold        = "\033[1m"
	Cyan        = "\033[36m"
	Yellow      = "\033[33m"
	Green       = "\033[32m"
	Red         = "\033[31m"
	BlackOnCyan = "\033[46;30m" // Black text on cyan background
)

// PrintStyledMessage prints a styled message with type and border
func PrintStyledMessage(msgType string, msg string) {
	var colorCode string

	switch strings.ToLower(msgType) {
	case "info":
		colorCode = Cyan + Bold
	case "success":
		colorCode = Green + Bold
	case "warning":
		colorCode = Yellow + Bold
	case "error":
		colorCode = Red + Bold
	default:
		colorCode = Cyan + Bold // Default to info style
	}

	// Border and padding for the message box
	fmt.Println()
	fmt.Printf("%s%s%s\n", BlackOnCyan, strings.ToUpper(msgType)+":", Reset)
	fmt.Printf("%s %s %s\n", colorCode, msg, Reset)
	fmt.Println()
}

// ApplyNetworkSettings accepts the legacy map and the API's NIC slice.
func ApplyNetworkSettings(ifaceArg interface{}) error {
	var ifaces []client.InterfaceInfo
	switch v := ifaceArg.(type) {
	case []client.InterfaceInfo:
		ifaces = v
	case map[string]client.InterfaceInfo:
		names := make([]string, 0, len(v))
		for name := range v {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			info := v[name]
			if info.Name == "" {
				info.Name = name
			}
			if info.Name != name {
				return fmt.Errorf("NIC map key %s differs from name %s", name, info.Name)
			}
			ifaces = append(ifaces, info)
		}
	default:
		return fmt.Errorf("unsupported interface argument type: %T", ifaceArg)
	}
	return ApplyNetworkSettingsWithOps(ifaces, RealOps())
}

func ApplyNetworkSettingsWithOps(ifaces []client.InterfaceInfo, ops Ops) error {
	normalized, err := client.NormalizeInterfaces(ifaces)
	if err != nil {
		return err
	}
	if len(normalized) == 0 {
		return fmt.Errorf("no network interfaces provided")
	}
	switch ops.OS() {
	case "linux":
		identities := make([]client.InterfaceInfo, len(normalized))
		for i, info := range normalized {
			out, err := ops.Run("ip", "-j", "link", "show", "dev", info.Name)
			var links []localLinkIdentity
			if err != nil {
				// Older iproute2 lacks JSON output; sysfs still supplies local identity.
				mac, readErr := ops.ReadFile("/sys/class/net/" + info.Name + "/address")
				if readErr != nil {
					return fmt.Errorf("NIC %s preflight: %w (%s); sysfs: %v", info.Name, err, strings.TrimSpace(string(out)), readErr)
				}
				links = append(links, localLinkIdentity{Name: info.Name, MAC: string(mac)})
			} else if err := json.Unmarshal(out, &links); err != nil {
				return fmt.Errorf("NIC %s identity: %w", info.Name, err)
			}
			if len(links) != 1 || links[0].Name != info.Name {
				return fmt.Errorf("NIC %s identity did not identify the requested interface", info.Name)
			}
			localMAC := links[0].matchingMAC()
			if localMAC != "" {
				mac, err := net.ParseMAC(localMAC)
				if err != nil || len(mac) != 6 {
					return fmt.Errorf("NIC %s has invalid local MAC %q", info.Name, localMAC)
				}
				localMAC = mac.String()
			}
			// Local identity is only for backend matching, never desired API/state data.
			identities[i] = info
			identities[i].MacAddress = localMAC
		}
		exists, err := ops.Exists("/etc/network/interfaces")
		if err != nil {
			return err
		}
		if exists {
			owned, err := interfacesOwnRequestedNICs(normalized, ops)
			if err != nil {
				return err
			}
			if owned {
				return applyInterfaces(normalized, ops)
			}
		}
		if _, err := ops.LookPath("netplan"); err == nil {
			files, err := loadNetplanFiles(ops)
			if err != nil {
				return err
			}
			if len(files) > 0 {
				return applyNetplan(identities, ops)
			}
		}
		if _, err := ops.LookPath("nmcli"); err == nil {
			return applyNetworkManager(normalized, ops)
		}
		if exists {
			return applyInterfaces(normalized, ops)
		}
		if _, err := ops.LookPath("netplan"); err == nil {
			return applyNetplan(identities, ops)
		}
		return fmt.Errorf("no supported Linux network configuration backend found")
	case "windows":
		for _, info := range normalized {
			if len(info.IPs) == 0 {
				return fmt.Errorf("NIC %s requires an IPv4 address on Windows", info.Name)
			}
		}
		return applyWindows(normalized, ops)
	default:
		return fmt.Errorf("unsupported OS: %s", ops.OS())
	}
}

func subnetMaskToCIDR(mask string) (int, error) {
	addr, err := netip.ParseAddr(mask)
	if err != nil || !addr.Is4() {
		return 0, fmt.Errorf("invalid IPv4 subnet mask %q", mask)
	}
	b := addr.As4()
	ones, bits := net.IPMask(b[:]).Size()
	if bits != 32 {
		return 0, fmt.Errorf("non-contiguous subnet mask %q", mask)
	}
	return ones, nil
}
func MaskToCIDR(mask string) string {
	bits, err := subnetMaskToCIDR(mask)
	if err != nil {
		return ""
	}
	return fmt.Sprint(bits)
}
func addressCIDRs(info client.InterfaceInfo) []string {
	out := make([]string, len(info.IPs))
	for i, ip := range info.IPs {
		bits, _ := subnetMaskToCIDR(ip.Subnet)
		out[i] = fmt.Sprintf("%s/%d", ip.IP, bits)
	}
	return out
}
func dnsAddresses(info client.InterfaceInfo) []string {
	if info.DnsServers == "" {
		return nil
	}
	return strings.Split(info.DnsServers, ",")
}

func GatherNetworkInterfaces() (map[string]client.InterfaceInfo, error) {
	return gatherNetworkInterfacesWithOps(RealOps())
}
func gatherNetworkInterfacesWithOps(ops Ops) (map[string]client.InterfaceInfo, error) {
	out, err := ops.Run("ip", "-j", "addr")
	if err != nil {
		return nil, fmt.Errorf("read network interfaces: %w (%s)", err, out)
	}
	var data []struct {
		Name      string `json:"ifname"`
		MAC       string `json:"address"`
		Addresses []struct {
			Family string `json:"family"`
			Local  string `json:"local"`
			Prefix int    `json:"prefixlen"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, fmt.Errorf("parse interface data: %w", err)
	}
	result := make(map[string]client.InterfaceInfo, len(data))
	for _, entry := range data {
		if entry.Name == "" {
			continue
		}
		info := client.InterfaceInfo{Name: entry.Name, MacAddress: entry.MAC}
		for _, address := range entry.Addresses {
			addr, err := netip.ParseAddr(address.Local)
			if err != nil || !addr.Is4() || address.Prefix < 0 || address.Prefix > 32 {
				continue
			}
			info.IPs = append(info.IPs, client.IPInfo{IP: addr.String(), Subnet: cidrToMask(address.Prefix)})
		}
		result[entry.Name] = info
	}
	return result, nil
}

func GetMacAddress(iface string) (string, error) {
	nic, err := net.InterfaceByName(iface)
	if err != nil {
		return "", err
	}
	if len(nic.HardwareAddr) == 0 {
		return "", fmt.Errorf("MAC address not found for NIC %s", iface)
	}
	return nic.HardwareAddr.String(), nil
}
