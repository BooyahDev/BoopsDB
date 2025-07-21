package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"boops/client"
	"boops/system"
)

var apiBase = "http://localhost:3001/api/machines"

// Store current network settings
var currentSettings map[string]client.InterfaceInfo

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

	fmt.Printf("Applying network settings for interfaces: %v\n", m.Interfaces)

	// Load previous machine state
	prevState, _ := client.LoadMachineState()
	stateChanged := prevState == nil || !client.InterfacesEqual(prevState.Interfaces, m.Interfaces)

	if len(m.Interfaces) > 0 && stateChanged {
		if err := system.ApplyNetworkSettings(m.Interfaces); err != nil {
			log.Printf("Failed to apply network settings: %v", err)
		}

		// Save new state
		state := &client.MachineState{
			Interfaces: m.Interfaces,
			Hostname:   m.Hostname,
		}
		if err := client.SaveMachineState(state); err != nil {
			log.Printf("Failed to save machine state: %v", err)
		}
	}

	// Set hostname if changed
	if m.Hostname != "" && (prevState == nil || prevState.Hostname != m.Hostname) {
		cmd := fmt.Sprintf("hostnamectl set-hostname %s", m.Hostname)
		fmt.Printf("Setting hostname to: %s\n", m.Hostname)
		cmdResult := exec.Command("sh", "-c", cmd)
		output, err := cmdResult.CombinedOutput()
		if err != nil {
			log.Printf("Failed to set hostname with error: %v, output: %s", err, string(output))
		} else {
			fmt.Printf("Hostname set successfully\n")
		}
	}

	// Get current OS name and update it in the server
	sysInfo := system.GatherSystemInfo()
	updateOsName := fmt.Sprintf(`{"os_name": "%s"}`, sysInfo.OsName)
	fmt.Printf("Current OS name: %s\n", updateOsName)
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s/update-os_name", apiBase, machineID), strings.NewReader(updateOsName))
	if err != nil {
		log.Fatalf("Failed to create OS name update request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	respUpdate, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Failed to send OS name update request: %v", err)
	} else if respUpdate.StatusCode >= 300 {
		log.Printf("OS name update failed with status code: %d", respUpdate.StatusCode)
	} else {
		fmt.Printf("Successfully updated OS name to: %s\n", sysInfo.OsName)
	}

	// Update memory size
	sysInfo = system.GatherSystemInfo()
	updateMemoryPayload := fmt.Sprintf(`{"memory_size": "%s"}`, sysInfo.MemorySize)
	fmt.Printf("Updating memory size to: %s\n", sysInfo.MemorySize)
	req, err = http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s/update-memory_size", apiBase, machineID), strings.NewReader(updateMemoryPayload))
	if err != nil {
		log.Fatalf("Failed to create memory size update request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	respMemUpdate, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Failed to send memory size update request: %v", err)
	} else if respMemUpdate.StatusCode >= 300 {
		log.Printf("Memory size update failed with status code: %d", respMemUpdate.StatusCode)
	} else {
		fmt.Printf("Successfully updated memory size to: %s\n", sysInfo.MemorySize)
	}

	req, err = http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s/update-last-alive", apiBase, machineID), nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	_, err = http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to send update-last-alive request: %v", err)
	}

	fmt.Println("Sync completed successfully.")
}

func postJSON(data any) {
	b, _ := json.Marshal(data)
	res, err := http.Post(fmt.Sprintf("%s/%s", apiBase, data.(client.Machine).ID), "application/json", strings.NewReader(string(b)))
	if err != nil {
		log.Fatalf("POST failed: %v", err)
	}
	defer res.Body.Close()
	log.Println("Registered successfully.")
}
