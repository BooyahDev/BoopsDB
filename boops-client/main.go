package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"boops/client"
	"boops/system"
	"boops/update"
)

var version = "dev"
var apiBase = "https://boopsdb-api.booyah.dev/api/machines"
var apiClient = &http.Client{Timeout: 30 * time.Second}

func PrintStyledMessage(kind, message string) {
	fmt.Printf("%s: %s\n", strings.ToUpper(kind), message)
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: boops <regist|sync|version|update> [machine-id]")
	}
	var err error
	switch os.Args[1] {
	case "version":
		if len(os.Args) != 2 {
			log.Fatal("Usage: boops version")
		}
		fmt.Println(version)
	case "regist":
		if len(os.Args) != 3 {
			log.Fatal("Usage: boops regist <machine-id>")
		}
		err = registerMachine(os.Args[2], fetchMachine, client.SaveConfig)
		if err == nil {
			PrintStyledMessage("success", "Registered existing machine successfully.")
		}
	case "sync":
		if len(os.Args) != 2 {
			log.Fatal("Usage: boops sync")
		}
		var cfg *client.Config
		cfg, err = client.LoadConfig()
		if err == nil {
			err = syncWithUpdate(cfg, checkUpdate, handleSync)
		}
	case "update":
		if len(os.Args) != 2 {
			log.Fatal("Usage: boops update")
		}
		_, err = checkUpdate(true)
	default:
		err = fmt.Errorf("unknown command: %s", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}
}

func checkUpdate(force bool) (bool, error) {
	executable, err := os.Executable()
	if err != nil {
		return false, err
	}
	result, err := update.Check(context.Background(), update.Options{
		CurrentVersion: version, ExecutablePath: executable, StateDir: "/etc/boops", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Force: force,
	})
	if result.Updated {
		PrintStyledMessage("success", "Installed client version "+result.Version+". The next sync uses the new executable.")
	} else if force && err == nil {
		PrintStyledMessage("info", "Update check completed. "+result.SkippedReason)
	}
	return result.Updated, err
}

// The updater runs before API access so an unavailable API cannot prevent updates.
func syncWithUpdate(cfg *client.Config, check func(bool) (bool, error), synchronize func(string) error) error {
	if cfg.AutoUpdateEnabled() {
		replaced, err := check(false)
		if err != nil {
			if replaced {
				PrintStyledMessage("warning", "Client was replaced; update bookkeeping reported an error: "+err.Error())
			} else {
				PrintStyledMessage("warning", "Update check failed: "+err.Error())
			}
		}
		if replaced {
			return nil
		}
	}
	return synchronize(cfg.ID)
}

func registerMachine(machineID string, fetch func(string) (client.Machine, error), saveID func(string) error) error {
	if machineID == "" {
		return fmt.Errorf("machine ID is required")
	}
	machine, err := fetch(machineID)
	if err != nil {
		return fmt.Errorf("verify existing machine: %w", err)
	}
	if machine.ID != machineID {
		return fmt.Errorf("API returned a different machine ID")
	}
	if err := saveID(machineID); err != nil {
		return fmt.Errorf("save registration: %w", err)
	}
	return nil
}

func machineURL(machineID string) string {
	return strings.TrimRight(apiBase, "/") + "/" + url.PathEscape(machineID)
}

func fetchMachine(machineID string) (client.Machine, error) {
	var machine client.Machine
	resp, err := apiClient.Get(machineURL(machineID))
	if err != nil {
		return machine, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return machine, fmt.Errorf("GET machine returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if err != nil {
		return machine, fmt.Errorf("read machine response: %w", err)
	}
	if len(data) > 4*1024*1024 {
		return machine, fmt.Errorf("machine response exceeds size limit")
	}
	if err = json.Unmarshal(data, &machine); err != nil {
		return machine, fmt.Errorf("invalid machine JSON: %w", err)
	}
	return machine, nil
}

// Applying all NICs and saving their comparison state form one success boundary.
func syncNetworkState(machine client.Machine, previous *client.MachineState, apply func([]client.InterfaceInfo) error, save func(*client.MachineState) error) error {
	interfaces, err := client.NormalizeInterfaces(machine.Interfaces)
	if err != nil {
		return err
	}
	changed := previous == nil || !client.InterfacesEqual(previous.Interfaces, interfaces)
	if changed && len(interfaces) > 0 {
		if err := apply(interfaces); err != nil {
			return fmt.Errorf("apply network settings: %w", err)
		}
	}
	if changed || previous.Hostname != machine.Hostname {
		return save(&client.MachineState{Interfaces: interfaces, Hostname: machine.Hostname})
	}
	return nil
}

func handleSync(machineID string) error {
	machine, err := fetchMachine(machineID)
	if err != nil {
		return err
	}
	if machine.ID != machineID {
		return fmt.Errorf("API returned a different machine ID")
	}
	previous, err := client.LoadMachineState()
	if err != nil {
		previous = nil
		if !os.IsNotExist(err) {
			PrintStyledMessage("warning", "Previous network state is unreadable; retrying settings: "+err.Error())
		}
	}
	if err := syncMachineSettings(machine, previous, runtime.GOOS, func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).CombinedOutput()
	}, func(interfaces []client.InterfaceInfo) error {
		return system.ApplyNetworkSettingsWithOps(interfaces, system.RealOps())
	}, client.SaveMachineState); err != nil {
		return err
	}
	updateInventory(machineID, machine)
	PrintStyledMessage("success", "Sync completed successfully.")
	return nil
}

func syncMachineSettings(machine client.Machine, previous *client.MachineState, platform string, run func(string, ...string) ([]byte, error), apply func([]client.InterfaceInfo) error, save func(*client.MachineState) error) error {
	// Validate the entire request before changing the hostname or network.
	if _, err := client.NormalizeInterfaces(machine.Interfaces); err != nil {
		return err
	}
	if machine.Hostname != "" && (previous == nil || previous.Hostname != machine.Hostname) {
		applied := false
		if platform == "linux" {
			output, err := run("hostnamectl", "set-hostname", "--", machine.Hostname)
			if err != nil {
				PrintStyledMessage("warning", fmt.Sprintf("Set hostname failed; continuing network sync: %v: %s", err, strings.TrimSpace(string(output))))
			} else {
				applied = true
			}
		}
		if !applied {
			machine.Hostname = ""
			if previous != nil {
				machine.Hostname = previous.Hostname
			}
		}
	}
	return syncNetworkState(machine, previous, apply, save)
}

func putMachineField(path string, payload any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(http.MethodPut, path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := apiClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if _, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024)); err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func updateInventory(machineID string, machine client.Machine) {
	info := system.GatherSystemInfo()
	for _, field := range []struct{ name, value string }{
		{"os_name", info.OsName}, {"memory_size", info.MemorySize}, {"cpu_arch", info.CpuArch}, {"cpu_info", info.CpuInfo}, {"disk_info", info.DiskInfo},
	} {
		if err := putMachineField(machineURL(machineID)+"/update-"+field.name, map[string]string{field.name: field.value}); err != nil {
			PrintStyledMessage("warning", "Update "+field.name+": "+err.Error())
		}
	}
	if err := putMachineField(machineURL(machineID)+"/update-last-alive", nil); err != nil {
		PrintStyledMessage("warning", "Update last_alive: "+err.Error())
	}
	for _, iface := range machine.Interfaces {
		mac, err := system.GetMacAddress(iface.Name)
		if err != nil {
			PrintStyledMessage("warning", "Read MAC "+iface.Name+": "+err.Error())
			continue
		}
		if mac != iface.MacAddress {
			if err := putMachineField(machineURL(machineID)+"/interfaces/"+url.PathEscape(iface.Name)+"/update-mac_address", map[string]string{"mac_address": mac}); err != nil {
				PrintStyledMessage("warning", "Update MAC "+iface.Name+": "+err.Error())
			}
		}
	}
}
