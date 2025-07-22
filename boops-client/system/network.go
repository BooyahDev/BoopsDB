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
