package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"boops/client"
	"boops/system"
)

// var apiBase = "https://boopsdb-api.booyah.dev/api/machines"
var apiBase = "http://10.0.1.1:3001/api/machines"

// Store current network settings
var currentSettings map[string]client.InterfaceInfo

// ANSI color codes
const (
	Reset       = "\033[0m"
	Bold        = "\033[1m"
	Dim         = "\033[2m"
	Italic      = "\033[3m"
	Underline   = "\033[4m"
	Red         = "\033[31m"
	Green       = "\033[32m"
	Yellow      = "\033[33m"
	Cyan        = "\033[36m"
	White       = "\033[37m"
	BlackOnCyan = "\033[46;30m" // Black text on cyan background
)

// PrintStyledMessage prints a styled message with optional type and border
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
		colorCode = White + Bold
	}

	// Border and padding for the message box
	fmt.Println()
	fmt.Printf("%s%s%s\n", BlackOnCyan, strings.ToUpper(msgType)+":", Reset)
	fmt.Printf("%s %s %s\n", colorCode, msg, Reset)
	fmt.Println()
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: boops <regist|sync> [machine-id]")
	}

	switch os.Args[1] {
	case "regist":
		if len(os.Args) != 3 {
			log.Fatal("Usage: boops regist <machine-id>")
		}
		handleRegist(os.Args[2])
	case "sync":
		cfg, err := client.LoadConfig()
		if err != nil {
			log.Fatal("Not registered. Run: boops regist <machine-id>")
		}
		handleSync(cfg.ID)
	default:
		log.Fatal("Unknown command")
	}
}

func handleRegist(machineID string) {
	sysInfo := system.GatherSystemInfo()
	sysInfo.ID = machineID

	if err := client.SaveConfig(machineID); err != nil {
		log.Fatalf("Failed to save config: %v", err)
	}

	postJSON(sysInfo)
}

func handleSync(machineID string) {

	PrintStyledMessage("info", fmt.Sprintf("Operating system: %s", runtime.GOOS))

	resp, err := http.Get(fmt.Sprintf("%s/%s", apiBase, machineID))
	if err != nil {
		log.Fatalf("Failed to fetch machine info: %v", err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	var m client.Machine
	if err := json.Unmarshal(body, &m); err != nil {
		log.Fatalf("Invalid JSON from API: %v", err)
	}

	// Load previous machine state
	prevState, _ := client.LoadMachineState()
	stateChanged := prevState == nil || !client.InterfacesEqual(prevState.Interfaces, m.Interfaces)

	// Set hostname if changed
	if m.Hostname != "" && (prevState == nil || prevState.Hostname != m.Hostname) {
		cmd := fmt.Sprintf("hostnamectl set-hostname %s", m.Hostname)
		PrintStyledMessage("info", fmt.Sprintf("Setting hostname to: %s", m.Hostname))
		cmdResult := exec.Command("sh", "-c", cmd)
		output, err := cmdResult.CombinedOutput()
		if err != nil {
			PrintStyledMessage("error", fmt.Sprintf("Failed to set hostname with error: %v, output: %s", err, string(output)))
		} else {
			PrintStyledMessage("success", "Hostname set successfully")
		}
	}

	// Get current OS name and update it in the server
	sysInfo := system.GatherSystemInfo()
	updateOsName := fmt.Sprintf(`{"os_name": "%s"}`, sysInfo.OsName)
	PrintStyledMessage("info", fmt.Sprintf("Current OS name: %s", updateOsName))
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s/update-os_name", apiBase, machineID), strings.NewReader(updateOsName))
	if err != nil {
		log.Fatalf("Failed to create OS name update request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	respUpdate, err := http.DefaultClient.Do(req)
	if err != nil {
		PrintStyledMessage("error", fmt.Sprintf("Failed to send OS name update request: %v", err))
	} else if respUpdate.StatusCode >= 300 {
		PrintStyledMessage("warning", fmt.Sprintf("OS name update failed with status code: %d", respUpdate.StatusCode))
	} else {
		PrintStyledMessage("success", fmt.Sprintf("Successfully updated OS name to: %s", sysInfo.OsName))
	}

	// Update memory size
	sysInfo = system.GatherSystemInfo()
	updateMemoryPayload := fmt.Sprintf(`{"memory_size": "%s"}`, sysInfo.MemorySize)
	PrintStyledMessage("info", fmt.Sprintf("Updating memory size to: %s", sysInfo.MemorySize))
	req, err = http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s/update-memory_size", apiBase, machineID), strings.NewReader(updateMemoryPayload))
	if err != nil {
		log.Fatalf("Failed to create memory size update request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	respMemUpdate, err := http.DefaultClient.Do(req)
	if err != nil {
		PrintStyledMessage("error", fmt.Sprintf("Failed to send memory size update request: %v", err))
	} else if respMemUpdate.StatusCode >= 300 {
		PrintStyledMessage("warning", fmt.Sprintf("Memory size update failed with status code: %d", respMemUpdate.StatusCode))
	} else {
		PrintStyledMessage("success", fmt.Sprintf("Successfully updated memory size to: %s", sysInfo.MemorySize))
	}

	// Update CPU architecture
	sysInfo = system.GatherSystemInfo()
	cpuArchPayload := fmt.Sprintf(`{"cpu_arch": "%s"}`, sysInfo.CpuArch)
	PrintStyledMessage("info", fmt.Sprintf("Updating CPU architecture to: %s", sysInfo.CpuArch))
	req, err = http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s/update-cpu_arch", apiBase, machineID), strings.NewReader(cpuArchPayload))
	if err != nil {
		log.Fatalf("Failed to create CPU architecture update request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	respCpuUpdate, err := http.DefaultClient.Do(req)
	if err != nil {
		PrintStyledMessage("error", fmt.Sprintf("Failed to send CPU architecture update request: %v", err))
	} else if respCpuUpdate.StatusCode >= 300 {
		PrintStyledMessage("warning", fmt.Sprintf("CPU architecture update failed with status code: %d", respCpuUpdate.StatusCode))
	} else {
		PrintStyledMessage("success", fmt.Sprintf("Successfully updated CPU architecture to: %s", sysInfo.CpuArch))
	}

	// Update CPU model info
	sysInfo = system.GatherSystemInfo()
	// Replace newlines with spaces to avoid JSON parsing issues
	cpuInfo := strings.ReplaceAll(sysInfo.CpuInfo, "\n", " ")
	cpuInfoPayload := fmt.Sprintf(`{"cpu_info": "%s"}`, cpuInfo)
	PrintStyledMessage("info", fmt.Sprintf("Updating CPU model info to: %s", cpuInfo))
	req, err = http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s/update-cpu_info", apiBase, machineID), strings.NewReader(cpuInfoPayload))
	if err != nil {
		log.Fatalf("Failed to create CPU info update request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	respCpuModelUpdate, err := http.DefaultClient.Do(req)
	if err != nil {
		PrintStyledMessage("error", fmt.Sprintf("Failed to send CPU info update request: %v", err))
	} else if respCpuModelUpdate.StatusCode >= 300 {
		PrintStyledMessage("warning", fmt.Sprintf("CPU info update failed with status code: %d", respCpuModelUpdate.StatusCode))
	} else {
		PrintStyledMessage("success", fmt.Sprintf("Successfully updated CPU model info to: %s", cpuInfo))
	}

	// Update disk info
	sysInfo = system.GatherSystemInfo()
	diskInfo := strings.ReplaceAll(sysInfo.DiskInfo, "\n", " ")
	diskInfoPayload := fmt.Sprintf(`{"disk_info": "%s"}`, diskInfo)
	PrintStyledMessage("info", fmt.Sprintf("Updating disk info to: %s", diskInfoPayload))
	req, err = http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s/update-disk_info", apiBase, machineID), strings.NewReader(diskInfoPayload))
	if err != nil {
		log.Fatalf("Failed to create disk info update request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	respDiskUpdate, err := http.DefaultClient.Do(req)
	if err != nil {
		PrintStyledMessage("error", fmt.Sprintf("Failed to send disk info update request: %v", err))
	} else if respDiskUpdate.StatusCode >= 300 {
		PrintStyledMessage("warning", fmt.Sprintf("Disk info update failed with status code: %d", respDiskUpdate.StatusCode))
	} else {
		PrintStyledMessage("success", "Successfully updated disk info")
	}

	req, err = http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s/update-last-alive", apiBase, machineID), nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	_, err = http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to send update-last-alive request: %v", err)
	}

	// Update MAC addresses for all interfaces
	for _, ifaceInfo := range m.Interfaces {
		if len(ifaceInfo.IPs) == 0 {
			continue // Skip interfaces without IPs
		}

		ipAddr := ifaceInfo.IPs[0].IP
		macAddr, err := system.GetMacAddress(ipAddr)
		if err != nil {
			PrintStyledMessage("warning", fmt.Sprintf("Failed to get MAC address for interface with IP %s: %v", ipAddr, err))
			continue
		}

		// Only update if the MAC address is different or empty in the server data
		currentMac := ifaceInfo.MacAddress
		if currentMac == "" || currentMac != macAddr {
			updatePayload := fmt.Sprintf(`{"mac_address": "%s"}`, macAddr)
			req, err = http.NewRequest(
				http.MethodPut,
				fmt.Sprintf("%s/%s/interfaces/ip-%s/update-mac_address", apiBase, machineID, ipAddr),
				strings.NewReader(updatePayload),
			)
			if err != nil {
				PrintStyledMessage("error", fmt.Sprintf("Failed to create MAC address update request for IP %s: %v", ipAddr, err))
				continue
			}
			req.Header.Set("Content-Type", "application/json")

			respMacUpdate, err := http.DefaultClient.Do(req)
			if err != nil {
				PrintStyledMessage("error", fmt.Sprintf("Failed to send MAC address update request for IP %s: %v", ipAddr, err))
			} else if respMacUpdate.StatusCode >= 300 {
				PrintStyledMessage("warning", fmt.Sprintf("MAC address update failed for IP %s with status code: %d", ipAddr, respMacUpdate.StatusCode))
			} else {
				PrintStyledMessage("success", fmt.Sprintf("Successfully updated MAC address for interface with IP %s to %s", ipAddr, macAddr))
			}
		}
	}

	PrintStyledMessage("info", fmt.Sprintf("Applying network settings for interfaces: %v", m.Interfaces))

	if len(m.Interfaces) > 0 && stateChanged {
		// Use the actual interface names from the API response
		ifaceMap := make(map[string]client.InterfaceInfo)
		for _, ifaceInfo := range m.Interfaces {
			if len(ifaceInfo.IPs) > 0 {
				ifaceMap[ifaceInfo.Name] = ifaceInfo // Use the actual interface name from struct
			}
		}

		if err := system.ApplyNetworkSettings(ifaceMap); err != nil {
			PrintStyledMessage("error", fmt.Sprintf("Failed to apply network settings: %v", err))
		}

		// Save new state
		state := &client.MachineState{
			Interfaces: m.Interfaces,
			Hostname:   m.Hostname,
		}
		if err := client.SaveMachineState(state); err != nil {
			PrintStyledMessage("error", fmt.Sprintf("Failed to save machine state: %v", err))
		} else {
			PrintStyledMessage("success", "Successfully saved new machine state")
		}
	}

	PrintStyledMessage("success", "Sync completed successfully.")
}

func postJSON(data any) {
	b, _ := json.Marshal(data)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s", apiBase, data.(client.Machine).ID), strings.NewReader(string(b)))
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	log.Println("Registered successfully.")
}
