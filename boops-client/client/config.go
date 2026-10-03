package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ID         string `json:"id"`
	AutoUpdate *bool  `json:"auto_update,omitempty"`
}

func (cfg *Config) AutoUpdateEnabled() bool {
	return cfg.AutoUpdate == nil || *cfg.AutoUpdate
}

var configPath = "/etc/boops/config.json"

func SaveConfig(id string) error {
	values := make(map[string]json.RawMessage)
	mode := os.FileMode(0644)
	previous, err := os.ReadFile(configPath)
	if err == nil {
		if err := json.Unmarshal(previous, &values); err != nil {
			return fmt.Errorf("read existing config: %w", err)
		}
		if values == nil {
			return fmt.Errorf("existing config must be a JSON object")
		}
		info, err := os.Stat(configPath)
		if err != nil {
			return err
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	values["id"], err = json.Marshal(id)
	if err != nil {
		return err
	}
	data, err := json.Marshal(values)
	if err != nil {
		return err
	}
	directory := filepath.Dir(configPath)
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), configPath)
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
	Interfaces []InterfaceInfo `json:"interfaces"`
	Hostname   string          `json:"hostname,omitempty"`
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
