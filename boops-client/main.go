package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

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

	if err := system.ApplyNetworkSettings(m.Interfaces); err != nil {
		log.Fatalf("Failed to apply network settings: %v", err)
	}

	http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s/update-last-alive", apiBase, machineID), nil)
	http.DefaultClient.Do(&http.Request{
		Method: http.MethodPut,
		URL:    mustParse(fmt.Sprintf("%s/%s/update-last-alive", apiBase, machineID)),
	})
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

func mustParse(url string) *http.URL {
	u, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		panic(err)
	}
	return u.URL
}