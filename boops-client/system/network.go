package system

import (
	"bufio"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"boops/client"
)

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

	fmt.Printf("IP link output:\n%s\n", string(output))

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	var currentInterface string
	var isLoopback bool

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Check for interface name lines (lines that start with a number followed by ":")
		if fields := strings.Fields(line); len(fields) > 0 {
			firstField := fields[0]
			if index := strings.Index(firstField, ":"); index != -1 {
				// Extract the interface name part
				ifNamePart := firstField[index+1:]
				ifName := strings.TrimSpace(ifNamePart)

				// Check if this is a new interface section
				currentInterface = ifName

				// Skip loopback interface by checking name and flags
				isLoopback = (strings.ToLower(currentInterface) == "lo") ||
					strings.Contains(strings.ToLower(line), "loopback")

				fmt.Printf("Found interface: %s, isLoopback: %v\n", currentInterface, isLoopback)
				continue
			}
		}

		// If this line contains a MAC address and we're not in a loopback section
		if !isLoopback && strings.Contains(line, "link/ether") {
			fmt.Printf("Found link/ether in line: %s\n", line)

			fields := strings.Fields(line)
			for i, field := range fields {
				if field == "link/ether" && i+1 < len(fields) {
					macAddr := fields[i+1]
					macAddrs[currentInterface] = macAddr
					fmt.Printf("Found MAC address %s for interface %s\n", macAddr, currentInterface)
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
