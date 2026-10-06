package system

import (
	"boops/client"
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

type netplanFile struct {
	path string
	doc  *yaml.Node
}
type netplanDefinition struct {
	kind, id  string
	effective *yaml.Node
	files     []int
}

func loadNetplanFiles(ops Ops) ([]netplanFile, error) {
	effective := map[string]string{}
	for _, dir := range []string{"/lib/netplan", "/etc/netplan", "/run/netplan"} {
		paths, err := ops.Glob(dir + "/*.yaml")
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			effective[filepath.Base(path)] = path
		}
	}
	names := make([]string, 0, len(effective))
	for name := range effective {
		names = append(names, name)
	}
	sort.Strings(names)
	files := make([]netplanFile, 0, len(names))
	for _, name := range names {
		path := effective[name]
		data, err := ops.ReadFile(path)
		if err != nil {
			return nil, err
		}
		doc, err := parseNetplan(data)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		if nodeValue(doc.Content[0], "<<") != nil || nodeValue(nodeValue(doc.Content[0], "network"), "<<") != nil {
			return nil, fmt.Errorf("unsupported Netplan merge in %s", path)
		}
		files = append(files, netplanFile{path, doc})
	}
	return files, nil
}

func planExistingNetplan(ifaces []client.InterfaceInfo, ops Ops) ([]fileChange, error) {
	files, err := loadNetplanFiles(ops)
	if err != nil {
		return nil, err
	}
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
	ids := make([]string, 0, len(definitions))
	for id := range definitions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	dirty := map[int]bool{}
	used := map[string]string{}
	for _, info := range ifaces {
		var selected *netplanDefinition
		rebindMAC := false
		for _, id := range ids {
			def := definitions[id]
			matches, rebind, err := matchExistingNetplan(def, info, ops)
			if err != nil {
				return nil, err
			}
			if matches {
				if selected != nil {
					return nil, fmt.Errorf("NIC %s matches multiple Netplan IDs %s and %s", info.Name, selected.id, id)
				}
				selected = def
				rebindMAC = rebind
			}
		}
		if selected == nil {
			if definitions[info.Name] != nil {
				return nil, fmt.Errorf("Netplan ID %s belongs to another NIC", info.Name)
			}
			f := -1
			for i, file := range files {
				if filepath.Base(file.path) == filepath.Base(netplanPath) {
					if strings.HasPrefix(file.path, "/run/") {
						return nil, fmt.Errorf("runtime Netplan shadows %s", netplanPath)
					}
					f = i
				}
			}
			if f < 0 {
				for _, file := range files {
					if filepath.Base(file.path) == filepath.Base(netplanPath) && strings.HasPrefix(file.path, "/run/") {
						return nil, fmt.Errorf("runtime Netplan shadows %s", netplanPath)
					}
				}
				doc, _ := parseNetplan(nil)
				files = append(files, netplanFile{netplanPath, doc})
				f = len(files) - 1
			}
			network, err := ensureMapping(files[f].doc.Content[0], "network")
			if err != nil {
				return nil, err
			}
			nodes, err := ensureMapping(network, "ethernets")
			if err != nil {
				return nil, err
			}
			setNode(nodes, info.Name, mapping())
			selected = &netplanDefinition{kind: "ethernets", id: info.Name, files: []int{f}}
		}
		if err := validateNetplanFragmentMACs(selected, files); err != nil {
			return nil, err
		}
		if previous := used[selected.id]; previous != "" {
			return nil, fmt.Errorf("NICs %s and %s share Netplan ID %s", previous, info.Name, selected.id)
		}
		used[selected.id] = info.Name
		if name := nodeValue(nodeValue(selected.effective, "match"), "name"); name != nil && strings.ContainsAny(name.Value, "*?[") {
			return nil, fmt.Errorf("NIC %s has a broad Netplan name pattern", info.Name)
		}
		for j, f := range selected.files {
			file := files[f]
			if strings.HasPrefix(file.path, "/run/") {
				return nil, fmt.Errorf("NIC %s is defined in temporary runtime Netplan %s", info.Name, file.path)
			}
			network := nodeValue(file.doc.Content[0], "network")
			nodes := nodeValue(network, selected.kind)
			device := nodeValue(nodes, selected.id)
			if network.Anchor != "" || nodes.Anchor != "" || device.Kind != yaml.MappingNode || hasYAMLSharing(device) || nodeValue(device, "<<") != nil {
				return nil, fmt.Errorf("NIC %s uses unsupported YAML sharing in %s", info.Name, file.path)
			}
			if rebindMAC {
				match := nodeValue(device, "match")
				if nodeValue(match, "macaddress") != nil {
					setNode(match, "macaddress", scalar(info.MacAddress))
				}
			}
			settings := info
			if j != 0 {
				settings.IPs = nil
				settings.Gateway = ""
				settings.DnsServers = ""
			}
			if err := updateNetplanDevice(device, settings); err != nil {
				return nil, err
			}
			dirty[f] = true
		}
	}
	var changes []fileChange
	for f, file := range files {
		if !dirty[f] {
			continue
		}
		path := file.path
		if strings.HasPrefix(path, "/lib/") {
			path = "/etc/netplan/" + filepath.Base(path)
		}
		snapshot, err := readSnapshot(ops, path, 0600)
		if err != nil {
			return nil, err
		}
		var out bytes.Buffer
		e := yaml.NewEncoder(&out)
		e.SetIndent(2)
		if err := e.Encode(file.doc); err != nil {
			return nil, err
		}
		if err := e.Close(); err != nil {
			return nil, err
		}
		changes = append(changes, fileChange{snapshot, out.Bytes()})
	}
	return changes, nil
}

func applyNetplan(ifaces []client.InterfaceInfo, ops Ops) error {
	changes, err := planExistingNetplan(ifaces, ops)
	if err != nil {
		return err
	}
	exists, err := ops.Exists("/etc/netplan")
	if err != nil {
		return err
	}
	if !exists {
		if err := runChecked(ops, "mkdir", "-p", "--", "/etc/netplan"); err != nil {
			return err
		}
	}
	return applyFiles(ops, changes, func() error {
		if err := runChecked(ops, "netplan", "generate"); err != nil {
			return err
		}
		return runChecked(ops, "netplan", "apply")
	})
}
