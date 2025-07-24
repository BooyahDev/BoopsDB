package system

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"boops/client"
)

func GatherSystemInfo() client.Machine {
	return client.Machine{
		Hostname:   getHostname(),
		OsName:     getOSInfo(),
		CpuInfo:    getCPUModel(),
		CpuArch:    runtime.GOARCH,
		MemorySize: getMemorySize(),
		DiskInfo:   getDiskInfo(),
		Interfaces: getInterfaces(),
		IsVirtual:  isVirtual(),
	}
}

func getHostname() string {
	host, _ := exec.Command("hostname").Output()
	return strings.TrimSpace(string(host))
}

func getOSInfo() string {
	if runtime.GOOS == "windows" {
		out, _ := exec.Command("cmd", "/C", "ver").Output()
		return strings.TrimSpace(string(out))
	}
	out, _ := exec.Command("sh", "-c", "lsb_release -d | sed 's/\\t/ /g'").Output()
	if len(out) == 0 {
		out, _ = exec.Command("uname", "-a").Output()
	}
	return strings.TrimSpace(string(out))
}

func getCPUModel() string {
	if runtime.GOOS == "windows" {
		out, _ := exec.Command("wmic", "cpu", "get", "Name").Output()
		lines := strings.Split(string(out), "\n")
		if len(lines) > 1 {
			return strings.TrimSpace(lines[1])
		}
		return "Unknown"
	}
	out, _ := exec.Command("sh", "-c", "lscpu | grep 'Model name' | cut -d: -f2 | sed 's/^[[:space:]]\\+//'").Output()
	return strings.TrimSpace(string(out))
}

func getMemorySize() string {
	if runtime.GOOS == "windows" {
		out, _ := exec.Command("wmic", "ComputerSystem", "get", "TotalPhysicalMemory").Output()
		lines := strings.Split(string(out), "\n")
		if len(lines) > 1 {
			bytes := strings.TrimSpace(lines[1])
			return fmt.Sprintf("%dGB", toGB(bytes))
		}
		return "0GB"
	}

	// Use free -b for bytes, then convert to GB with proper handling of decimal values
	out, _ := exec.Command("sh", "-c", "free -b | grep Mem").Output()
	lines := strings.Split(string(out), "\n")
	if len(lines) > 0 {
		fields := strings.Fields(strings.TrimSpace(lines[0]))
		if len(fields) >= 2 {
			var bytes int64
			fmt.Sscanf(fields[1], "%d", &bytes)
			gb := float64(bytes) / (1024 * 1024 * 1024)
			return fmt.Sprintf("%.1fGB", gb)
		}
	}
	return "0GB"
}

func getDiskInfo() string {
	if runtime.GOOS == "windows" {
		out, _ := exec.Command("wmic", "logicaldisk", "get", "Caption,Size").Output()
		lines := strings.Split(string(out), "\n")
		var results []string
		for _, line := range lines[1:] {
			tokens := strings.Fields(line)
			if len(tokens) == 2 {
				size := toGB(tokens[1])
				results = append(results, fmt.Sprintf("%s : %dGB", tokens[0], size))
			}
		}
		return strings.Join(results, "\n")
	}
	out, _ := exec.Command("sh", "-c", "lsblk -b -o NAME,SIZE -dn | awk '{printf(\"/dev/%s : %.0fGB\\n\", $1, $2/1024/1024/1024)}'").Output()
	return strings.TrimSpace(string(out))
}

func getInterfaces() []client.InterfaceInfo {
	out, _ := exec.Command("ip", "-j", "addr").Output()
	var data []map[string]interface{}
	json.Unmarshal(out, &data)
	result := make([]client.InterfaceInfo, 0)

	for _, ifaceData := range data {
		name := ifaceData["ifname"].(string)
		if name == "" { // Skip empty interface names
			continue
		}

		var ipInfos []client.IPInfo

		if addrs, ok := ifaceData["addr_info"].([]interface{}); ok && len(addrs) > 0 {
			for _, addrData := range addrs {
				addrMap := addrData.(map[string]interface{})
				local := addrMap["local"].(string)
				prefixlen := int(addrMap["prefixlen"].(float64))

				// Ensure prefix length is valid
				if prefixlen < 0 || prefixlen > 32 {
					continue // Skip invalid addresses
				}

				subnet := cidrToMask(prefixlen)

				ipInfos = append(ipInfos, client.IPInfo{
					IP:     local,
					Subnet: subnet,
				})
			}
		}

		if len(ipInfos) > 0 { // Only include interfaces with valid IP addresses
			result = append(result, client.InterfaceInfo{
				IPs:        ipInfos,
				Gateway:    "",
				DnsServers: "",   // Empty string instead of slice
				MacAddress: name, // Use interface name as ID for now
			})
		}
	}

	return result
}

func isVirtual() int {
	out, _ := exec.Command("systemd-detect-virt").Output()
	virtType := strings.TrimSpace(string(out))
	if virtType == "" || virtType == "none" {
		return 0
	}
	return 1
}

func toGB(raw string) int64 {
	var b int64
	fmt.Sscanf(raw, "%d", &b)
	return b / 1024 / 1024 / 1024
}
