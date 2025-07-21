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

var apiBase = "http://localhost:3001/api/machines"

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

	if runtime.GOOS == "linux" && len(m.Interfaces) > 0 {
		for name, info := range m.Interfaces {
			if info.IP != "" {
				cmds := []string{
					fmt.Sprintf("ip addr flush dev %s", name),
					fmt.Sprintf("ip addr add %s/%s dev %s", info.IP, system.MaskToCIDR(info.Subnet), name),
				}
				if info.Gateway != "" {
					cmds = append(cmds, fmt.Sprintf("ip route add default via %s dev %s", info.Gateway, name))
				}

				for _, cmd := range cmds {
					fmt.Printf("Running command: %s\n", cmd)
					cmdResult := exec.Command("sh", "-c", cmd)
					output, err := cmdResult.CombinedOutput()
					if err != nil {
						log.Printf("Command failed with error: %v, output: %s", err, string(output))
					} else {
						fmt.Printf("Command succeeded with output: %s\n", string(output))
					}
				}
			}
		}
	} else if runtime.GOOS == "windows" && len(m.Interfaces) > 0 {
		for name, info := range m.Interfaces {
			if info.IP != "" {
				args := []string{
					"interface ip set address", fmt.Sprintf("name=\"%s\"", name), fmt.Sprintf("static %s %s", info.IP, info.Subnet),
				}
				if info.Gateway != "" {
					args = append(args, info.Gateway)
				}

				cmd := exec.Command("netsh", args...)
				output, err := cmd.CombinedOutput()
				if err != nil {
					log.Printf("Command failed with error: %v, output: %s", err, string(output))
				} else {
					fmt.Printf("Network settings applied successfully for interface %s\n", name)
				}
			}
		}
	}

	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s/update-last-alive", apiBase, machineID), nil)
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
