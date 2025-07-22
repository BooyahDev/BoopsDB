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
	out, _ := exec.Command("sh", "-c", "free -g | grep Mem | awk '{print $2 \"GB\"}'").Output()
	return strings.TrimSpace(string(out))
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

func getInterfaces() map[string]client.InterfaceInfo {
	out, _ := exec.Command("ip", "-j", "addr").Output()
	var data []map[string]interface{}
	json.Unmarshal(out, &data)
	result := make(map[string]client.InterfaceInfo)
	for _, iface := range data {
		name := iface["ifname"].(string)
		mac := iface["address"].(string)
		var ip, mask string
		if addrs, ok := iface["addr_info"].([]interface{}); ok && len(addrs) > 0 {
			addr := addrs[0].(map[string]interface{})
			ip = addr["local"].(string)
			mask = cidrToMask(int(addr["prefixlen"].(float64)))
		}
		result[name] = client.InterfaceInfo{
			IP:         ip,
			Subnet:     mask,
			Gateway:    "",
			DnsServers: []string{},
			MacAddress: mac,
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
