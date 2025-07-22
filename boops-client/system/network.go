package system

import (
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

func ApplyNetworkSettings(ifaces map[string]client.InterfaceInfo) error {
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
	for name, info := range ifaces {
		cmds := []string{
			fmt.Sprintf("ip addr flush dev %s", name),
			fmt.Sprintf("ip addr add %s/%s dev %s", info.IP, MaskToCIDR(info.Subnet), name),
		}
		if info.Gateway != "" {
			cmds = append(cmds, fmt.Sprintf("ip route add default via %s dev %s", info.Gateway, name))
		}
		for _, cmd := range cmds {
			exec.Command("sh", "-c", cmd).Run()
		}
	}
	return nil
}

func applyWindows(ifaces map[string]client.InterfaceInfo) error {
	for name, info := range ifaces {
		args := []string{
			"interface ip set address", fmt.Sprintf("name=\"%s\"", name), fmt.Sprintf("static %s %s", info.IP, info.Subnet),
		}
		if info.Gateway != "" {
			args = append(args, info.Gateway)
		}
		exec.Command("netsh", args...).Run()
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
// func GatherNetworkInterfaces() map[string]client.InterfaceInfo {
// 	out, _ := exec.Command("ip", "-j", "addr").Output()
// 	var data []map[string]interface{}
// 	json.Unmarshal(out, &data)
// 	result := make(map[string]client.InterfaceInfo)
// 	for _, iface := range data {
// 		name := iface["ifname"].(string)
// 		mac := iface["address"].(string)
// 		var ip, mask string
// 		if addrs, ok := iface["addr_info"].([]interface{}); ok && len(addrs) > 0 {
// 			addr := addrs[0].(map[string]interface{})
// 			ip = addr["local"].(string)
// 			mask = cidrToMask(int(addr["prefixlen"].(float64)))
// 		}
// 		result[name] = client.InterfaceInfo{
// 			IP:         ip,
// 			Subnet:     mask,
// 			Gateway:    "",
// 			DnsServers: []string{},
// 			MacAddress: mac,
// 		}
// 	}
// 	return result
// }

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
