package system

import (
	"boops/client"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

type localLinkIdentity struct {
	Name         string `json:"ifname"`
	MAC          string `json:"address"`
	PermanentMAC string `json:"permaddr"`
	LinkInfo     struct {
		Kind string `json:"info_kind"`
	} `json:"linkinfo"`
}

func (link localLinkIdentity) matchingMAC() string {
	if link.PermanentMAC != "" {
		return strings.TrimSpace(link.PermanentMAC)
	}
	return strings.TrimSpace(link.MAC)
}

func localNetplanLinks(ops Ops) ([]localLinkIdentity, error) {
	data, err := ops.Run("ip", "-j", "-d", "link", "show")
	var links []localLinkIdentity
	if err == nil {
		if err := json.Unmarshal(data, &links); err != nil {
			return nil, fmt.Errorf("read local NIC inventory: %w", err)
		}
		return links, nil
	}
	paths, err := ops.Glob("/sys/class/net/*/address")
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		mac, err := ops.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read local NIC identity %s: %w", path, err)
		}
		link := localLinkIdentity{Name: filepath.Base(filepath.Dir(path)), MAC: strings.TrimSpace(string(mac))}
		physical, err := ops.Exists("/sys/class/net/" + link.Name + "/device")
		if err != nil {
			return nil, err
		}
		if !physical {
			link.LinkInfo.Kind = "unverified"
		}
		links = append(links, link)
	}
	return links, nil
}

func validateNetplanFragmentMACs(def *netplanDefinition, files []netplanFile) error {
	var binding string
	for _, f := range def.files {
		device := nodeValue(nodeValue(nodeValue(files[f].doc.Content[0], "network"), def.kind), def.id)
		copy, err := cloneNetplanForInspection(device, 0)
		if err != nil {
			return err
		}
		mac := nodeValue(nodeValue(copy, "match"), "macaddress")
		if mac == nil {
			continue
		}
		parsed, err := net.ParseMAC(mac.Value)
		if err != nil {
			return fmt.Errorf("invalid MAC for Netplan ID %s in %s", def.id, files[f].path)
		}
		if binding != "" && binding != parsed.String() {
			return fmt.Errorf("Netplan ID %s has conflicting MAC bindings across file fragments", def.id)
		}
		binding = parsed.String()
	}
	return nil
}

// Rebind only an explicitly named physical NIC whose old MAC has no local owner.
// MAC-only groups and definitions for another name retain their original binding.
func matchExistingNetplan(def *netplanDefinition, info client.InterfaceInfo, ops Ops) (bool, bool, error) {
	matches, err := netplanMatches(def.id, def.effective, info)
	if err != nil || matches {
		return matches, false, err
	}
	if def.kind != "ethernets" {
		return false, false, nil
	}
	device := def.effective
	setName := nodeValue(device, "set-name")
	if setName != nil && setName.Value != info.Name {
		return false, false, nil
	}
	match := nodeValue(device, "match")
	mac := nodeValue(match, "macaddress")
	if mac == nil {
		return false, false, nil
	}
	name := nodeValue(match, "name")
	if def.id != info.Name && setName == nil && (name == nil || name.Value != info.Name) {
		return false, false, nil
	}
	if name != nil && name.Value != info.Name {
		return false, false, fmt.Errorf("cannot rebind Netplan ID %s with unmatched name %s", def.id, name.Value)
	}
	for i := 0; i < len(match.Content); i += 2 {
		if key := match.Content[i].Value; key != "name" && key != "macaddress" {
			return false, false, fmt.Errorf("cannot rebind Netplan ID %s with match property %s", def.id, key)
		}
	}
	if nodeValue(device, "macaddress") != nil {
		return false, false, fmt.Errorf("cannot rebind Netplan ID %s with an explicit runtime MAC", def.id)
	}
	oldMAC, err := net.ParseMAC(mac.Value)
	if err != nil || len(oldMAC) != 6 || mac.Kind != yaml.ScalarNode {
		return false, false, fmt.Errorf("invalid Netplan MAC binding for %s", def.id)
	}
	links, err := localNetplanLinks(ops)
	if err != nil {
		return false, false, err
	}
	targets := 0
	for _, link := range links {
		if link.Name == info.Name {
			targets++
			if !strings.EqualFold(link.matchingMAC(), info.MacAddress) || link.LinkInfo.Kind != "" {
				return false, false, fmt.Errorf("cannot verify replacement physical NIC %s", info.Name)
			}
		}
		for _, value := range []string{link.MAC, link.PermanentMAC} {
			actual, err := net.ParseMAC(strings.TrimSpace(value))
			if err == nil && actual.String() == oldMAC.String() {
				return false, false, fmt.Errorf("Netplan ID %s MAC %s still belongs to local NIC %s", def.id, oldMAC, link.Name)
			}
		}
	}
	if targets != 1 {
		return false, false, fmt.Errorf("cannot verify replacement NIC %s in local inventory", info.Name)
	}
	return true, true, nil
}
