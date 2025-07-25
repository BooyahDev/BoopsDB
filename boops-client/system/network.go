package system

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"boops/client"
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

func ApplyNetworkSettings(ifaceArg interface{}) error {
	var ifaces map[string]client.InterfaceInfo

	switch v := ifaceArg.(type) {
	case map[string]client.InterfaceInfo:
		ifaces = v
	case []client.InterfaceInfo:
		ifaces = make(map[string]client.InterfaceInfo)
		for i, info := range v {
			if len(info.IPs) > 0 && len(info.IPs[0].IP) > 0 {
				ifaceName := info.Name // Use actual interface name if available
				if ifaceName == "" {
					ifaceName = fmt.Sprintf("interface-%d", i)
				}
				ifaces[ifaceName] = info
			}
		}
	default:
		return fmt.Errorf("unsupported interface argument type: %T", ifaceArg)
	}

	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	if len(ifaces) == 0 {
		return fmt.Errorf("no network interfaces provided")
	}

	if runtime.GOOS == "linux" {
		return applyLinux(ifaces)
	} else if runtime.GOOS == "windows" {
		return applyWindows(ifaces)
	}
	return nil
}

func applyLinux(ifaces map[string]client.InterfaceInfo) error {
	isDebian, err := isDebianSystem()
	if err != nil {
		return fmt.Errorf("unable to determine system type: %v", err)
	}

	for name, info := range ifaces {
		// Check if interface exists
		cmd := exec.Command("ip", "link", "show", name)
		output, err := cmd.CombinedOutput()
		if err != nil || strings.Contains(string(output), "Device does not exist") {
			return fmt.Errorf("interface %s does not exist on this system", name)
		}

		if isDebian {
			err = applyNetplan(name, info)
		} else {
			err = applyNmcli(name, info)
		}
		if err != nil {
			return fmt.Errorf("failed to apply settings for interface %s: %v", name, err)
		}
	}

	return nil
}

func isDebianSystem() (bool, error) {
	// Check for Debian-based system first
	if _, err := exec.Command("test", "-f", "/etc/debian_version").CombinedOutput(); err == nil {
		return true, nil
	}

	// Then check for RedHat-based system
	if _, err := exec.Command("test", "-f", "/etc/redhat-release").CombinedOutput(); err == nil {
		return false, nil
	}

	// If neither file exists, we can't determine if it's Debian or RedHat
	return false, fmt.Errorf("unable to determine OS type")
}

func applyNetplan(iface string, info client.InterfaceInfo) error {
	configPath := "/etc/netplan/01-netcfg.yaml"
	// IP アドレスとサブネットを CIDR 形式で連結
	var addresses []string
	for _, ip := range info.IPs {
		cidr, err := subnetMaskToCIDR(ip.Subnet)
		if err != nil {
			return fmt.Errorf("invalid subnet mask %s: %w", ip.Subnet, err)
		}
		addresses = append(addresses, fmt.Sprintf("%s/%d", ip.IP, cidr))
	}

	// DNS サーバをスライスに変換
	var dnsList []string
	for _, dns := range strings.Split(info.DnsServers, ",") {
		trimmed := strings.TrimSpace(dns)
		if trimmed != "" {
			dnsList = append(dnsList, trimmed)
		}
	}

	// YAML 生成
	content := fmt.Sprintf(`network:
  version: 2
  ethernets:
    %s:
      dhcp4: no
      addresses: [%s]
      gateway4: %s
      nameservers:
        addresses: [%s]
`, iface, strings.Join(addresses, ", "), info.Gateway, strings.Join(dnsList, ", "))

	// Remove all existing netplan configurations to avoid conflicts
	var cmd *exec.Cmd
	var output []byte
	var err error

	cmd = exec.Command("sh", "-c", "sudo rm -f /etc/netplan/*.yaml")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove existing netplan configs: %v, output: %s", err, string(output))
	}

	err = writeNetplanConfig(configPath, content)
	if err != nil {
		return fmt.Errorf("failed to write netplan config: %v", err)
	}

	cmd = exec.Command("sh", "-c", "sudo chmod 600 "+configPath)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set file permissions: %v, output: %s", err, string(output))
	}

	cmd = exec.Command("sh", "-c", "sudo netplan apply")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("netplan apply failed with error: %v, output: %s", err, string(output))
	}
	return nil
}

func writeNetplanConfig(path, content string) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | sudo tee %s > /dev/null", content, path))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write config: %v, output: %s", err, string(output))
	}
	return nil
}

func applyNmcli(iface string, info client.InterfaceInfo) error {
	// Check if interface exists before applying settings
	cmd := exec.Command("ip", "link", "show", iface)
	output, err := cmd.CombinedOutput()
	if err != nil || strings.Contains(string(output), "Device does not exist") {
		return fmt.Errorf("interface %s does not exist on this system", iface)
	}

	// IP アドレスとサブネットを CIDR 形式で連結
	var addresses []string
	for _, ip := range info.IPs {
		cidr, err := subnetMaskToCIDR(ip.Subnet)
		if err != nil {
			return fmt.Errorf("invalid subnet mask %s: %w", ip.Subnet, err)
		}
		addresses = append(addresses, fmt.Sprintf("%s/%d", ip.IP, cidr))
	}

	// DNS サーバをスライスに変換
	var dnsList []string
	for _, dns := range strings.Split(info.DnsServers, ",") {
		trimmed := strings.TrimSpace(dns)
		if trimmed != "" {
			dnsList = append(dnsList, trimmed)
		}
	}

	cmds := []string{
		fmt.Sprintf("nmcli dev disconnect iface '%s'", iface),
		fmt.Sprintf("nmcli dev set '%s' ipv4.addresses \"%s\" ipv4.method manual connection.autoconnect yes", iface, strings.Join(addresses, " ")),
	}

	if info.Gateway != "" {
		cmds = append(cmds, fmt.Sprintf("nmcli dev set '%s' ipv4.gateway %s", iface, info.Gateway))
	}

	for _, cmd := range cmds {
		output, err := exec.Command("sh", "-c", "sudo "+cmd).CombinedOutput()
		if err != nil {
			return fmt.Errorf("nmcli command failed with error: %v, output: %s", err, string(output))
		}
	}

	if len(dnsList) > 0 {
		cmd := fmt.Sprintf("nmcli dev set %s ipv4.dns \"%s\"", iface, strings.Join(dnsList, " "))
		output, err := exec.Command("sh", "-c", "sudo "+cmd).CombinedOutput()
		if err != nil {
			return fmt.Errorf("Setting DNS failed with error: %v, output: %s", err, string(output))
		}
	}

	cmd = exec.Command("sh", "-c", "sudo nmcli con up "+iface)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nmcli connection up failed with error: %v, output: %s", err, string(output))
	}
	return nil
}

func applyWindows(ifaces map[string]client.InterfaceInfo) error {
	for name, info := range ifaces {
		for _, ipInfo := range info.IPs {
			args := []string{
				"interface ip set address", fmt.Sprintf("name=\"%s\"", name), fmt.Sprintf("static %s %s", ipInfo.IP, ipInfo.Subnet),
			}
			if info.Gateway != "" {
				args = append(args, info.Gateway)
			}
			exec.Command("netsh", args...).Run()
		}
	}
	return nil
}

func MaskToCIDR(mask string) string {
	parts := strings.Split(mask, ".")
	bits := 0
	for _, part := range parts {
		n := 0
		fmt.Sscanf(part, "%d", &n)
		for n > 0 {
			bits += int(n % 2)
			n >>= 1
		}
	}
	return fmt.Sprintf("%d", bits)
}

// GatherNetworkInterfaces returns a map of network interfaces and their current information
func GatherNetworkInterfaces() (map[string]client.InterfaceInfo, error) {
	out, err := exec.Command("ip", "-j", "addr").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("command failed with error: %v, output: %s", err, string(out))
	}
	var data []map[string]interface{}
	err = json.Unmarshal(out, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	result := make(map[string]client.InterfaceInfo)
	for _, ifaceData := range data {
		name := ifaceData["ifname"].(string)

		var macAddr string
		macCmd := exec.Command("ip", "-o", "link")
		macOutput, err := macCmd.CombinedOutput()
		if err == nil {
			lines := strings.Split(string(macOutput), "\n")
			for _, line := range lines {
				if strings.Contains(line, name) && strings.Contains(line, "link/ether") {
					fields := strings.Fields(line)
					for i, field := range fields {
						if field == "link/ether" && i+1 < len(fields) {
							macAddr = fields[i+1]
							break
						}
					}
				}
			}

			if macAddr == "" {
				cmd := exec.Command("cat", "/sys/class/net/"+name+"/address")
				output, err := cmd.CombinedOutput()
				if err == nil && len(output) > 0 {
					macAddr = strings.TrimSpace(string(output))
				}
			}
		}

		var ipInfos []client.IPInfo
		if addrs, ok := ifaceData["addr_info"].([]interface{}); ok && len(addrs) > 0 {
			for _, addrData := range addrs {
				addrMap := addrData.(map[string]interface{})
				local := addrMap["local"].(string)
				prefixlen := int(addrMap["prefixlen"].(float64))
				subnet := cidrToMask(prefixlen)

				ipInfos = append(ipInfos, client.IPInfo{
					IP:     local,
					Subnet: subnet,
				})
			}
		}

		result[name] = client.InterfaceInfo{
			IPs:        ipInfos,
			Gateway:    "",
			DnsServers: "", // Empty string for DNS servers, will be set to comma-separated values elsewhere if needed
			MacAddress: macAddr,
			Name:       name, // Add the actual interface name
		}
	}

	return result, nil
}

// GetMacAddress retrieves the MAC address for a given interface name using platform-specific commands
func GetMacAddress(iface string) (string, error) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("ip", "-o", "link")
	case "windows":
		cmd = exec.Command("wmic", "nic where \"NetConnectionID like '%"+iface+"%'", "get MACAddress /value")
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed with error: %v, output: %s", err, string(output))
	}

	var macAddr string
	switch runtime.GOOS {
	case "linux":
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, iface) && strings.Contains(line, "link/ether") {
				fields := strings.Fields(line)
				for i, field := range fields {
					if field == "link/ether" && i+1 < len(fields) {
						macAddr = fields[i+1]
						break
					}
				}
			}
		}

		if macAddr == "" {
			cmd = exec.Command("cat", "/sys/class/net/"+iface+"/address")
			output, err = cmd.CombinedOutput()
			if err == nil && len(output) > 0 {
				macAddr = strings.TrimSpace(string(output))
			}
		}

	case "windows":
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "MACAddress=") {
				macAddr = strings.ReplaceAll(strings.TrimSpace(strings.Split(line, "=")[1]), "\"", "")
				break
			}
		}
	}

	if macAddr == "" {
		return "", fmt.Errorf("MAC address not found for interface: %s", iface)
	}

	return macAddr, nil
}

// サブネットマスク（例: "255.255.255.0"）を CIDR 表記（例: 24）に変換
func subnetMaskToCIDR(mask string) (int, error) {
	octets := strings.Split(mask, ".")
	if len(octets) != 4 {
		return 0, fmt.Errorf("invalid subnet format")
	}
	cidr := 0
	for _, octet := range octets {
		switch octet {
		case "255":
			cidr += 8
		case "254":
			cidr += 7
		case "252":
			cidr += 6
		case "248":
			cidr += 5
		case "240":
			cidr += 4
		case "224":
			cidr += 3
		case "192":
			cidr += 2
		case "128":
			cidr += 1
		case "0":
			// nothing
		default:
			return 0, fmt.Errorf("unsupported octet: %s", octet)
		}
	}
	return cidr, nil
}
