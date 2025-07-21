package system

import (
	"boops-client/api"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func CollectSystemInfo(id string) (*api.MachineInfo, error) {
	hostname, _ := exec.Command("hostname").Output()
	osName := getOSName()
	cpuInfo, cpuArch, cores, threads := getCPUInfo()
	memory := getMemoryInfo()
	disk := getDiskInfo()
	interfaces := getInterfaces()

	return &api.MachineInfo{
		ID:         id,
		Hostname:   strings.TrimSpace(string(hostname)),
		OSName:     osName,
		CPUInfo:    fmt.Sprintf("%s (%d cores, %d threads)", cpuInfo, cores, threads),
		CPUArch:    cpuArch,
		MemorySize: memory,
		DiskInfo:   disk,
		Interfaces: interfaces,
	}, nil
}

func getOSName() string {
	if runtime.GOOS == "windows" {
		out, _ := exec.Command("cmd", "/C", "ver").Output()
		return strings.TrimSpace(string(out))
	}
	out, _ := exec.Command("lsb_release", "-ds").Output()
	return strings.Trim(string(out), "\" \n")
}

func getCPUInfo() (string, string, int, int) {
	out, _ := exec.Command("lscpu").Output()
	lines := strings.Split(string(out), "\n")
	var model, arch string
	var cores, threads int
	for _, line := range lines {
		if strings.HasPrefix(line, "Architecture") {
			arch = strings.TrimSpace(strings.Split(line, ":")[1])
		} else if strings.HasPrefix(line, "Model name") {
			model = strings.TrimSpace(strings.Split(line, ":")[1])
		} else if strings.HasPrefix(line, "CPU(s):") && threads == 0 {
			threads, _ = strconv.Atoi(strings.TrimSpace(strings.Split(line, ":")[1]))
		} else if strings.HasPrefix(line, "Core(s) per socket") {
			cores, _ = strconv.Atoi(strings.TrimSpace(strings.Split(line, ":")[1]))
		}
	}
	return model, arch, cores, threads
}

func getMemoryInfo() string {
	out, _ := exec.Command("free", "-g").Output()
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Mem:") {
			fields := strings.Fields(line)
			return fields[1] + "GB"
		}
	}
	return ""
}

func getDiskInfo() string {
	out, _ := exec.Command("lsblk", "-b", "-o", "NAME,SIZE,TYPE", "-dn").Output()
	lines := strings.Split(string(out), "\n")
	var buf bytes.Buffer
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[2] == "disk" {
			size, _ := strconv.ParseInt(fields[1], 10, 64)
			gb := size / (1024 * 1024 * 1024)
			buf.WriteString(fmt.Sprintf("/dev/%s : %dGB\n", fields[0], gb))
		}
	}
	return strings.TrimSpace(buf.String())
}

func getInterfaces() map[string]api.InterfaceInfo {
	interfaces, _ := net.Interfaces()
	result := make(map[string]api.InterfaceInfo)
	for _, iface := range interfaces {
		if (iface.Flags & net.FlagLoopback) != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		ip := ""
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
				break
			}
		}
		mac := iface.HardwareAddr.String()
		result[iface.Name] = api.InterfaceInfo{
			IP:         ip,
			Subnet:     "",
			Gateway:    "",
			DNSServers: []string{},
			MAC:        mac,
		}
	}
	return result
}

func ApplyNetworkSettings(ifaces map[string]api.InterfaceInfo) error {
	osName := runtime.GOOS
	if osName == "linux" {
		return applyLinuxNetwork(ifaces)
	} else if osName == "windows" {
		return applyWindowsNetwork(ifaces)
	}
	return fmt.Errorf("unsupported OS: %s", osName)
}

func applyLinuxNetwork(ifaces map[string]api.InterfaceInfo) error {
	out, _ := exec.Command("grep", "^ID=", "/etc/os-release").Output()
	osID := strings.Trim(strings.Split(string(out), "=")[1], "\"\n")
	if strings.Contains(osID, "ubuntu") || strings.Contains(osID, "debian") {
		return applyNetplan(ifaces)
	}
	return applyIfcfg(ifaces) // RHEL系
}

func applyNetplan(ifaces map[string]api.InterfaceInfo) error {
	for name, info := range ifaces {
		// gateway4 セクション（指定されていない場合は除外）
		gateway := ""
		if info.Gateway != "" {
			gateway = fmt.Sprintf("      gateway4: %s\n", info.Gateway)
		}

		// nameservers セクション（DNSがあれば指定）
		nameservers := ""
		if len(info.DNSServers) > 0 {
			nameservers = fmt.Sprintf(`      nameservers:
        addresses: [%s]
`, strings.Join(info.DNSServers, ", "))
		}

		yaml := fmt.Sprintf(`network:
  version: 2
  ethernets:
    %s:
      dhcp4: false
      addresses: [%s/%s]
%s%s`, name, info.IP, maskToCIDR(info.Subnet), gateway, nameservers)

		// 設定ファイルを書き換えて反映
		cmd := exec.Command("sh", "-c", fmt.Sprintf("rm /etc/netplan/* && echo '%s' > /etc/netplan/01-boops.yaml && netplan apply", yaml))
		cmd.Run()
	}
	return nil
}

func applyIfcfg(ifaces map[string]api.InterfaceInfo) error {
	for name, info := range ifaces {
		// 基本設定
		conf := fmt.Sprintf(`DEVICE=%s
BOOTPROTO=static
ONBOOT=yes
IPADDR=%s
NETMASK=%s
`, name, info.IP, info.Subnet)

		// Gateway（任意）
		if info.Gateway != "" {
			conf += fmt.Sprintf("GATEWAY=%s\n", info.Gateway)
		}

		// DNS（任意）
		if len(info.DNSServers) > 0 {
			for i, dns := range info.DNSServers {
				conf += fmt.Sprintf("DNS%d=%s\n", i+1, dns)
			}
		}

		// ファイル書き出しとネットワーク再起動
		path := fmt.Sprintf("/etc/sysconfig/network-scripts/ifcfg-%s", name)
		err := os.WriteFile(path, []byte(conf), 0644)
		if err != nil {
			return err
		}

		// 再起動コマンド
		exec.Command("systemctl", "restart", "network").Run()
	}
	return nil
}

func applyWindowsNetwork(ifaces map[string]api.InterfaceInfo) error {
	for name, info := range ifaces {
		mask := info.Subnet
		if mask == "" {
			mask = "255.255.255.0"
		}

		args := []string{
			"interface", "ip", "set", "address", fmt.Sprintf("name=\"%s\"", name),
			"static", info.IP, mask,
		}
		if info.Gateway != "" {
			args = append(args, info.Gateway)
		} else {
			args = append(args, "none")
		}

		exec.Command("netsh", args...).Run()

		if len(info.DNSServers) > 0 {
			// 既存DNSクリア
			exec.Command("netsh", "interface", "ip", "set", "dns", fmt.Sprintf("name=\"%s\"", name), "source=static", fmt.Sprintf("addr=%s", info.DNSServers[0])).Run()
			for i := 1; i < len(info.DNSServers); i++ {
				exec.Command("netsh", "interface", "ip", "add", "dns", fmt.Sprintf("name=\"%s\"", name), fmt.Sprintf("addr=%s", info.DNSServers[i]), "index=2").Run()
			}
		}
	}
	return nil
}

func maskToCIDR(mask string) string {
	ip := net.ParseIP(mask).To4()
	if ip == nil {
		return "24"
	}
	cidr := 0
	for _, octet := range ip {
		for i := 7; i >= 0; i-- {
			if octet&(1<<i) != 0 {
				cidr++
			}
		}
	}
	return fmt.Sprintf("%d", cidr)
}

