package system

import (
	"bufio"
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

func GetMacAddresses() (map[string]string, error) {
	macAddrs := make(map[string]string)

	cmd := exec.Command("ip", "link")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %v", err)
	}

	PrintStyledMessage("info", fmt.Sprintf("IP link output:\n%s", string(output)))

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	var currentInterface string
	var isLoopback bool

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Look for lines that define interfaces (start with a number followed by ":")
		if fields := strings.Fields(line); len(fields) > 0 && strings.Contains(fields[0], ":") {
			// The interface name is the part after ":" and before "<"
			parts := strings.SplitN(fields[0], ":", 2)
			if len(parts) != 2 {
				continue
			}

			index := strings.Index(parts[1], "<")
			var ifName string
			if index == -1 {
				// No "<" found, use the whole part after ":"
				ifName = parts[1]
			} else {
				// Extract only the interface name before "<"
				ifName = parts[1][:index]
			}

			// Check if this is a new interface section
			currentInterface = ifName

			// Skip loopback interface by checking name and flags
			isLoopback = (strings.ToLower(currentInterface) == "lo") ||
				strings.Contains(strings.ToLower(line), "loopback")

			PrintStyledMessage("info", fmt.Sprintf("Found interface: %s, isLoopback: %v", currentInterface, isLoopback))
			continue
		}

		// If this line contains a MAC address and we're not in a loopback section
		if !isLoopback && strings.Contains(line, "link/ether") {
			PrintStyledMessage("info", fmt.Sprintf("Found link/ether in line: %s", line))

			fields := strings.Fields(line)
			for i, field := range fields {
				if field == "link/ether" && i+1 < len(fields) {
					macAddr := fields[i+1]
					macAddrs[currentInterface] = macAddr
					PrintStyledMessage("info", fmt.Sprintf("Found MAC address %s for interface %s", macAddr, currentInterface))
					break
				}
			}
		}
	}

	return macAddrs, nil
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
