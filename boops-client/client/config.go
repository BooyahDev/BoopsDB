package client

import (
	"encoding/json"
	"os"
)

type Config struct {
	ID string `json:"id"`
}

var configPath = "/etc/boops/config.json"

func SaveConfig(id string) error {
	cfg := Config{ID: id}
	data, _ := json.Marshal(cfg)
	os.MkdirAll("/etc/boops", 0755)
	return os.WriteFile(configPath, data, 0644)
}

func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = json.Unmarshal(data, &cfg)
	return &cfg, err
}

// MachineState represents the machine state for comparison
type MachineState struct {
	Interfaces map[string]InterfaceInfo `json:"interfaces"`
	Hostname   string                   `json:"hostname,omitempty"`
}

var machineStatePath = "/etc/boops/machine_state.json"

func SaveMachineState(state *MachineState) error {
	data, _ := json.Marshal(state)
	return os.WriteFile(machineStatePath, data, 0644)
}

func LoadMachineState() (*MachineState, error) {
	data, err := os.ReadFile(machineStatePath)
	if err != nil {
		return nil, err
	}
	var state MachineState
	err = json.Unmarshal(data, &state)
	return &state, err
}

// InterfacesEqual compares two interface maps for equality
func InterfacesEqual(a, b map[string]InterfaceInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for name, infoA := range a {
		infoB, exists := b[name]
		if !exists || infoA.Gateway != infoB.Gateway || len(infoA.IPs) != len(infoB.IPs) {
			return false
		}

		// Check if IPs match (ignoring order)
		aIpMap := make(map[string]string)
		for _, ipInfo := range infoA.IPs {
			aIpMap[ipInfo.IP] = ipInfo.Subnet
		}
		bIpMap := make(map[string]string)
		for _, ipInfo := range infoB.IPs {
			bIpMap[ipInfo.IP] = ipInfo.Subnet
		}

		if len(aIpMap) != len(bIpMap) {
			return false
		}

		for ip, subnet := range aIpMap {
			if bSubnet, exists := bIpMap[ip]; !exists || subnet != bSubnet {
				return false
			}
		}
	}
	return true
}
