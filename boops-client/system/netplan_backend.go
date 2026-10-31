package system

import (
	"boops/client"
	"fmt"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// netplanNMDefinition is the part of an effective Netplan definition that is
// needed to prove that NetworkManager owns a requested interface.  The UUID
// is deliberately read from YAML rather than inferred from the interface
// name; NetworkManager-generated Netplan IDs are not required to be names.
type netplanNMDefinition struct {
	def      *netplanDefinition
	renderer string
	uuid     string
}

type netplanNetworkManagerSelection struct {
	use    bool
	active map[string]string
}

func collectNetplanDefinitions(files []netplanFile) (map[string]*netplanDefinition, error) {
	definitions := map[string]*netplanDefinition{}
	for f, file := range files {
		network := nodeValue(file.doc.Content[0], "network")
		for _, kind := range []string{"ethernets", "wifis", "bridges", "bonds", "vlans"} {
			nodes := nodeValue(network, kind)
			if nodes == nil {
				continue
			}
			if nodes.Kind != yaml.MappingNode || nodeValue(nodes, "<<") != nil {
				return nil, fmt.Errorf("unsupported Netplan %s in %s", kind, file.path)
			}
			for i := 0; i < len(nodes.Content); i += 2 {
				id := nodes.Content[i].Value
				copy, err := cloneNetplanForInspection(nodes.Content[i+1], 0)
				if err != nil {
					return nil, fmt.Errorf("read %s/%s: %w", file.path, id, err)
				}
				def := definitions[id]
				if def == nil {
					def = &netplanDefinition{kind: kind, id: id}
					definitions[id] = def
				} else if def.kind != kind {
					return nil, fmt.Errorf("Netplan ID %s appears in multiple device types", id)
				}
				def.effective = mergeInspectedNetplan(def.effective, copy)
				def.files = append(def.files, f)
			}
		}
	}
	return definitions, nil
}

func netplanGlobalRenderer(files []netplanFile) string {
	renderer := ""
	for _, file := range files {
		network := nodeValue(file.doc.Content[0], "network")
		if value := nodeValue(network, "renderer"); value != nil && value.Kind == yaml.ScalarNode {
			renderer = strings.TrimSpace(value.Value)
		}
	}
	return renderer
}

func netplanDefinitionRenderer(def *netplanDefinition, global string) (string, error) {
	renderer := global
	if value := nodeValue(def.effective, "renderer"); value != nil {
		if value.Kind != yaml.ScalarNode {
			return "", fmt.Errorf("Netplan renderer for %s must be scalar", def.id)
		}
		renderer = strings.TrimSpace(value.Value)
	}
	return renderer, nil
}

func netplanDefinitionNMUUID(def *netplanDefinition) (string, error) {
	networkManager := nodeValue(def.effective, "networkmanager")
	if networkManager == nil {
		return "", nil
	}
	if networkManager.Kind != yaml.MappingNode {
		return "", fmt.Errorf("NetworkManager settings for %s must be a mapping", def.id)
	}
	uuid := nodeValue(networkManager, "uuid")
	if uuid == nil {
		return "", nil
	}
	if uuid.Kind != yaml.ScalarNode || strings.TrimSpace(uuid.Value) == "" {
		return "", fmt.Errorf("NetworkManager UUID for %s must be a non-empty scalar", def.id)
	}
	return strings.TrimSpace(uuid.Value), nil
}

func netplanDefinitionMatchesInfo(def *netplanDefinition, info client.InterfaceInfo) (bool, error) {
	setName := nodeValue(def.effective, "set-name")
	renamed := setName != nil && setName.Kind == yaml.ScalarNode && setName.Value == info.Name
	match := nodeValue(def.effective, "match")
	if match == nil {
		return def.id == info.Name || renamed, nil
	}
	if match.Kind != yaml.MappingNode {
		return false, fmt.Errorf("Netplan match for %s must be a mapping", def.id)
	}
	if name := nodeValue(match, "name"); name != nil {
		if name.Kind != yaml.ScalarNode {
			return false, fmt.Errorf("Netplan match name for %s must be scalar", def.id)
		}
		matched, err := filepath.Match(name.Value, info.Name)
		if err != nil {
			return false, err
		}
		if !matched && !renamed {
			return false, nil
		}
	}
	if mac := nodeValue(match, "macaddress"); mac != nil {
		if mac.Kind != yaml.ScalarNode {
			return false, fmt.Errorf("Netplan match MAC for %s must be scalar", def.id)
		}
		if info.MacAddress == "" || !strings.EqualFold(strings.TrimSpace(mac.Value), strings.TrimSpace(info.MacAddress)) {
			return false, nil
		}
	}
	for i := 0; i < len(match.Content); i += 2 {
		key := match.Content[i].Value
		if key != "name" && key != "macaddress" {
			return false, fmt.Errorf("cannot verify Netplan match property %s for NIC %s", key, info.Name)
		}
	}
	// An empty match is valid for a NetworkManager-generated profile, but only
	// when the profile UUID below is also verified against the active device.
	return len(match.Content) > 0 || def.id == info.Name || renamed, nil
}

func readActiveNetworkManagerBindings(ops Ops) (map[string]string, map[string]string, error) {
	out, err := ops.Run("nmcli", "-t", "--escape", "no", "-f", "UUID,DEVICE", "connection", "show", "--active")
	if err != nil {
		return nil, nil, fmt.Errorf("read active NetworkManager connections: %w", err)
	}
	byDevice := map[string]string{}
	byUUID := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, nil, fmt.Errorf("invalid active NetworkManager connection output")
		}
		uuid, device := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if device == "" || device == "--" {
			continue
		}
		if previous := byDevice[device]; previous != "" && previous != uuid {
			return nil, nil, fmt.Errorf("multiple active connections for NIC %s", device)
		}
		if previous := byUUID[uuid]; previous != "" && previous != device {
			return nil, nil, fmt.Errorf("NetworkManager UUID %s is active on multiple NICs", uuid)
		}
		byDevice[device] = uuid
		byUUID[uuid] = device
	}
	return byDevice, byUUID, nil
}

// netplanNetworkManagerBackend returns true only when every requested NIC
// that is being changed is an explicit NetworkManager-backed Netplan profile.
// It fails closed on mixed backends, missing UUIDs, duplicate definitions, or
// a YAML UUID that does not identify the active NIC. Returning false preserves
// the existing Netplan path for ordinary networkd configurations.
func netplanNetworkManagerBackend(ifaces []client.InterfaceInfo, files []netplanFile, ops Ops) (netplanNetworkManagerSelection, error) {
	definitions, err := collectNetplanDefinitions(files)
	if err != nil {
		return netplanNetworkManagerSelection{}, err
	}
	globalRenderer := netplanGlobalRenderer(files)
	var nmDefs []netplanNMDefinition
	for _, def := range definitions {
		renderer, err := netplanDefinitionRenderer(def, globalRenderer)
		if err != nil {
			return netplanNetworkManagerSelection{}, err
		}
		if renderer != "NetworkManager" {
			continue
		}
		uuid, err := netplanDefinitionNMUUID(def)
		if err != nil {
			return netplanNetworkManagerSelection{}, err
		}
		// Keep definitions without a UUID in the candidate set so a request
		// naming that profile fails closed instead of falling back to a YAML
		// rewrite that cannot safely identify the NetworkManager connection.
		nmDefs = append(nmDefs, netplanNMDefinition{def: def, renderer: renderer, uuid: uuid})
	}
	if len(nmDefs) == 0 {
		return netplanNetworkManagerSelection{}, nil
	}
	// Only ask NetworkManager for its active table when a requested local
	// definition can actually be an NM profile. This keeps a normal networkd
	// request independent from unrelated NM Wi-Fi profiles (and from nmcli
	// being installed or available).
	needsNetworkManager := false
	explicitNonNetworkManager := make(map[string]bool, len(ifaces))
	for _, info := range ifaces {
		for _, def := range definitions {
			renderer, err := netplanDefinitionRenderer(def, globalRenderer)
			if err != nil {
				return netplanNetworkManagerSelection{}, err
			}
			matched, err := netplanDefinitionMatchesInfo(def, info)
			if err != nil {
				return netplanNetworkManagerSelection{}, err
			}
			if !matched {
				continue
			}
			if renderer != "NetworkManager" {
				explicitNonNetworkManager[info.Name] = true
				continue
			}
			needsNetworkManager = true
		}
		// An opaque NM-generated ID with match: {} cannot be matched by name.
		// If no explicit non-NM definition names this request, inspect active
		// UUIDs so that the opaque profile can still be selected safely.
		if !explicitNonNetworkManager[info.Name] {
			needsNetworkManager = true
		}
	}
	if !needsNetworkManager && globalRenderer != "NetworkManager" {
		return netplanNetworkManagerSelection{}, nil
	}
	byDevice, byUUID, err := readActiveNetworkManagerBindings(ops)
	if err != nil {
		return netplanNetworkManagerSelection{}, err
	}
	nmCount, otherCount := 0, 0
	for _, info := range ifaces {
		activeUUID := byDevice[info.Name]
		matches := make([]netplanNMDefinition, 0, 1)
		for _, candidate := range nmDefs {
			if candidate.uuid != "" && candidate.uuid == activeUUID {
				matches = append(matches, candidate)
			}
		}
		if len(matches) > 1 {
			return netplanNetworkManagerSelection{}, fmt.Errorf("NIC %s has ambiguous NetworkManager Netplan UUID %s", info.Name, activeUUID)
		}
		if len(matches) == 1 {
			if explicitNonNetworkManager[info.Name] {
				return netplanNetworkManagerSelection{}, fmt.Errorf("NIC %s matches both NetworkManager and non-NetworkManager Netplan definitions", info.Name)
			}
			if byUUID[matches[0].uuid] != info.Name {
				return netplanNetworkManagerSelection{}, fmt.Errorf("NetworkManager UUID %s is not active on NIC %s", matches[0].uuid, info.Name)
			}
			nmCount++
			continue
		}

		// If this request names an NM definition, silently falling back to the
		// YAML editor could overwrite a generated profile. Require a verified
		// active UUID instead.
		for _, candidate := range nmDefs {
			matched, err := netplanDefinitionMatchesInfo(candidate.def, info)
			if err != nil {
				return netplanNetworkManagerSelection{}, err
			}
			if matched {
				if candidate.uuid == "" {
					return netplanNetworkManagerSelection{}, fmt.Errorf("NetworkManager Netplan definition %s for NIC %s has no UUID", candidate.def.id, info.Name)
				}
				if activeUUID == "" {
					return netplanNetworkManagerSelection{}, fmt.Errorf("NetworkManager Netplan definition %s for NIC %s has no active UUID", candidate.def.id, info.Name)
				}
				return netplanNetworkManagerSelection{}, fmt.Errorf("NetworkManager Netplan UUID %s for NIC %s does not match active UUID %s", candidate.uuid, info.Name, activeUUID)
			}
		}
		if !explicitNonNetworkManager[info.Name] {
			if activeUUID == "" {
				return netplanNetworkManagerSelection{}, fmt.Errorf("NIC %s has no active UUID for NetworkManager Netplan selection", info.Name)
			}
			return netplanNetworkManagerSelection{}, fmt.Errorf("active NetworkManager UUID %s for NIC %s has no matching Netplan definition", activeUUID, info.Name)
		}
		otherCount++
	}
	if nmCount > 0 && otherCount > 0 {
		return netplanNetworkManagerSelection{}, fmt.Errorf("requested NICs use mixed Netplan and NetworkManager backends")
	}
	return netplanNetworkManagerSelection{use: nmCount > 0, active: byDevice}, nil
}
