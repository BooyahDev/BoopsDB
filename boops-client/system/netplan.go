package system

import (
	"boops/client"
	"bytes"
	"fmt"
	"io"
	"net/netip"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

const netplanPath = "/etc/netplan/01-netcfg.yaml"

func parseNetplan(data []byte) (*yaml.Node, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("network:\n  version: 2\n")
	}
	var doc yaml.Node
	d := yaml.NewDecoder(bytes.NewReader(data))
	if err := d.Decode(&doc); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("Netplan must contain one YAML document")
	}
	var validate func(*yaml.Node) error
	validate = func(n *yaml.Node) error {
		if n.Kind == yaml.MappingNode {
			seen := map[string]bool{}
			for i := 0; i < len(n.Content); i += 2 {
				key := n.Content[i]
				if key.Kind != yaml.ScalarNode || seen[key.Value] {
					return fmt.Errorf("invalid or duplicate YAML key %q", key.Value)
				}
				seen[key.Value] = true
			}
		}
		for _, child := range n.Content {
			if err := validate(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := validate(&doc); err != nil {
		return nil, err
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("Netplan root must be a mapping")
	}
	if network := nodeValue(doc.Content[0], "network"); network != nil {
		if version := nodeValue(network, "version"); version != nil && (version.Kind != yaml.ScalarNode || version.Value != "2") {
			return nil, fmt.Errorf("Netplan version must be 2")
		}
	}
	return &doc, nil
}

func nodeValue(n *yaml.Node, key string) *yaml.Node {
	for depth := 0; n != nil && n.Kind == yaml.AliasNode && depth < 32; depth++ {
		n = n.Alias
	}
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}
func scalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}
func mapping() *yaml.Node { return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"} }
func sequence(values []string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, v := range values {
		n.Content = append(n.Content, scalar(v))
	}
	return n
}
func setNode(n *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			old := n.Content[i+1]
			value.HeadComment = old.HeadComment
			value.LineComment = old.LineComment
			value.FootComment = old.FootComment
			n.Content[i+1] = value
			return
		}
	}
	n.Content = append(n.Content, scalar(key), value)
}
func removeNode(n *yaml.Node, key string) {
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			n.Content = append(n.Content[:i], n.Content[i+2:]...)
			return
		}
	}
}
func ensureMapping(n *yaml.Node, key string) (*yaml.Node, error) {
	value := nodeValue(n, key)
	if value == nil {
		value = mapping()
		setNode(n, key, value)
	}
	if value.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("Netplan %s must be a mapping (aliases/merges cannot be managed)", key)
	}
	if nodeValue(value, "<<") != nil {
		return nil, fmt.Errorf("Netplan merge in %s cannot be managed", key)
	}
	return value, nil
}

func netplanMatches(id string, n *yaml.Node, info client.InterfaceInfo) (bool, error) {
	setName := nodeValue(n, "set-name")
	renamed := setName != nil && setName.Value == info.Name
	match := nodeValue(n, "match")
	if match == nil {
		return id == info.Name || renamed, nil
	}
	for depth := 0; match != nil && match.Kind == yaml.AliasNode && depth < 32; depth++ {
		match = match.Alias
	}
	if match == nil || match.Kind != yaml.MappingNode {
		return false, fmt.Errorf("Netplan match for %s must be a mapping", id)
	}
	if name := nodeValue(match, "name"); name != nil {
		if name.Kind != yaml.ScalarNode {
			return false, fmt.Errorf("Netplan match name for %s must be scalar", id)
		}
		ok, err := filepath.Match(name.Value, info.Name)
		if err != nil {
			return false, err
		}
		if !ok && !renamed {
			return false, nil
		}
	}
	if mac := nodeValue(match, "macaddress"); mac != nil {
		if info.MacAddress == "" {
			return false, fmt.Errorf("cannot verify MAC match for NIC %s in definition %s", info.Name, id)
		}
		if !strings.EqualFold(mac.Value, info.MacAddress) {
			return false, nil
		}
	}
	for i := 0; i < len(match.Content); i += 2 {
		key := match.Content[i].Value
		if key != "name" && key != "macaddress" {
			return false, fmt.Errorf("cannot verify Netplan match property %s for NIC %s", key, info.Name)
		}
	}
	if len(match.Content) == 0 {
		return false, fmt.Errorf("empty Netplan match for %s", id)
	}
	return true, nil
}

func managedNetplanIDs(data []byte, ifaces []client.InterfaceInfo) (map[string]string, error) {
	doc, err := parseNetplan(data)
	if err != nil {
		return nil, err
	}
	ids := map[string]string{}
	for _, info := range ifaces {
		ids[info.Name] = info.Name
	}
	network := nodeValue(doc.Content[0], "network")
	for _, kind := range []string{"ethernets", "wifis", "bridges", "bonds", "vlans"} {
		nodes := nodeValue(network, kind)
		if nodes == nil {
			continue
		}
		if nodes.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("Netplan %s must be a mapping", kind)
		}
		for i := 0; i < len(nodes.Content); i += 2 {
			for _, info := range ifaces {
				matches, err := netplanMatches(nodes.Content[i].Value, nodes.Content[i+1], info)
				if err != nil {
					return nil, err
				}
				if matches {
					ids[nodes.Content[i].Value] = info.Name
				}
			}
		}
	}
	return ids, nil
}

func ipv4DefaultRoute(n *yaml.Node) bool {
	to := nodeValue(n, "to")
	if to == nil {
		return false
	}
	if to.Value == "0.0.0.0/0" {
		return true
	}
	if to.Value != "default" {
		return false
	}
	via := nodeValue(n, "via")
	if via == nil {
		return true
	}
	addr, err := netip.ParseAddr(via.Value)
	return err == nil && addr.Is4()
}

func netplanHasIPv4Settings(n *yaml.Node) bool {
	if nodeValue(n, "<<") != nil {
		return true
	}
	for _, key := range []string{"addresses", "dhcp4", "gateway4"} {
		if nodeValue(n, key) != nil {
			return true
		}
	}
	if routes := nodeValue(n, "routes"); routes != nil {
		for _, r := range routes.Content {
			if ipv4DefaultRoute(r) {
				return true
			}
		}
	}
	return false
}

func checkForeignNetplan(ifaces []client.InterfaceInfo, managedIDs map[string]string, ops Ops) error {
	// Equal basenames are shadowed by /run, then /etc, then /lib.
	effective := map[string]string{}
	for _, dir := range []string{"/lib/netplan", "/etc/netplan", "/run/netplan"} {
		paths, err := ops.Glob(dir + "/*.yaml")
		if err != nil {
			return err
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
	for _, name := range names {
		path := effective[name]
		if path == netplanPath {
			continue
		}
		data, err := ops.ReadFile(path)
		if err != nil {
			return err
		}
		doc, err := parseNetplan(data)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		network := nodeValue(doc.Content[0], "network")
		if nodeValue(doc.Content[0], "<<") != nil || nodeValue(network, "<<") != nil {
			return fmt.Errorf("cannot safely resolve foreign Netplan merge in %s", path)
		}
		for _, kind := range []string{"ethernets", "wifis", "bridges", "bonds", "vlans"} {
			nodes := nodeValue(network, kind)
			if nodes == nil {
				continue
			}
			if nodes.Kind != yaml.MappingNode {
				return fmt.Errorf("%s: %s must be a mapping", path, kind)
			}
			if nodeValue(nodes, "<<") != nil {
				return fmt.Errorf("cannot safely resolve foreign Netplan %s merge in %s", kind, path)
			}
			for i := 0; i < len(nodes.Content); i += 2 {
				id, device := nodes.Content[i].Value, nodes.Content[i+1]
				if nic, managed := managedIDs[id]; managed && (netplanHasIPv4Settings(device) || nodeValue(device, "match") != nil || nodeValue(device, "set-name") != nil) {
					return fmt.Errorf("NIC %s shares Netplan ID %s with foreign definition in %s", nic, id, path)
				}
				for _, info := range ifaces {
					matches, err := netplanMatches(id, device, info)
					if err != nil {
						return err
					}
					if matches && netplanHasIPv4Settings(device) {
						return fmt.Errorf("NIC %s conflicts with foreign Netplan definition in %s", info.Name, path)
					}
				}
			}
		}
	}
	if path, ok := effective[filepath.Base(netplanPath)]; ok && path != netplanPath {
		return fmt.Errorf("%s shadows managed Netplan file %s", path, netplanPath)
	}
	return nil
}

func hasYAMLSharing(n *yaml.Node) bool {
	if n.Anchor != "" || n.Kind == yaml.AliasNode {
		return true
	}
	for _, child := range n.Content {
		if hasYAMLSharing(child) {
			return true
		}
	}
	return false
}

// keepIPv6Nodes replaces IPv4 lists while preserving existing IPv6 entries.
func keepIPv6Nodes(old *yaml.Node, newValues []string, prefix bool) (*yaml.Node, error) {
	n := sequence(newValues)
	if old == nil {
		return n, nil
	}
	if old.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("address list must be a sequence")
	}
	for _, entry := range old.Content {
		value := entry.Value
		if entry.Kind == yaml.MappingNode && len(entry.Content) > 0 {
			value = entry.Content[0].Value
		} else if entry.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("unsupported address node")
		}
		var addr netip.Addr
		var err error
		if prefix {
			var p netip.Prefix
			p, err = netip.ParsePrefix(value)
			addr = p.Addr()
		} else {
			addr, err = netip.ParseAddr(value)
		}
		if err != nil {
			return nil, fmt.Errorf("invalid existing address %q", value)
		}
		if addr.Is6() {
			n.Content = append(n.Content, entry)
		}
	}
	return n, nil
}

func generateNetplan(data []byte, ifaces []client.InterfaceInfo) ([]byte, error) {
	doc, err := parseNetplan(data)
	if err != nil {
		return nil, err
	}
	if nodeValue(doc.Content[0], "<<") != nil {
		return nil, fmt.Errorf("Netplan root merge cannot be managed")
	}
	network, err := ensureMapping(doc.Content[0], "network")
	if err != nil {
		return nil, err
	}
	if network.Anchor != "" {
		return nil, fmt.Errorf("managed Netplan network is a shared YAML anchor")
	}
	if version := nodeValue(network, "version"); version == nil {
		setNode(network, "version", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "2"})
	}
	ethernets, err := ensureMapping(network, "ethernets")
	if err != nil {
		return nil, err
	}
	if ethernets.Anchor != "" {
		return nil, fmt.Errorf("managed Netplan ethernets is a shared YAML anchor")
	}
	usedIDs := map[string]string{}
	for _, info := range ifaces {
		id := info.Name
		var device *yaml.Node
		for _, kind := range []string{"ethernets", "wifis", "bridges", "bonds", "vlans"} {
			nodes := nodeValue(network, kind)
			if nodes == nil {
				continue
			}
			if nodes.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("Netplan %s must be a mapping", kind)
			}
			for i := 0; i < len(nodes.Content); i += 2 {
				matches, err := netplanMatches(nodes.Content[i].Value, nodes.Content[i+1], info)
				if err != nil {
					return nil, err
				}
				if matches {
					if kind != "ethernets" || device != nil {
						return nil, fmt.Errorf("NIC %s has ambiguous or unsupported Netplan definitions", info.Name)
					}
					id = nodes.Content[i].Value
					device = nodes.Content[i+1]
				}
			}
		}
		if device == nil {
			if nodeValue(ethernets, id) != nil {
				return nil, fmt.Errorf("Netplan ID %s belongs to another NIC", id)
			}
			device = mapping()
			setNode(ethernets, id, device)
		}
		if previous, ok := usedIDs[id]; ok {
			return nil, fmt.Errorf("NICs %s and %s match the same Netplan definition %s", previous, info.Name, id)
		}
		usedIDs[id] = info.Name
		if device.Kind != yaml.MappingNode || nodeValue(device, "<<") != nil || hasYAMLSharing(device) {
			return nil, fmt.Errorf("NIC %s uses unsupported YAML alias/merge", info.Name)
		}
		addresses, err := keepIPv6Nodes(nodeValue(device, "addresses"), addressCIDRs(info), true)
		if err != nil {
			return nil, fmt.Errorf("NIC %s: %w", info.Name, err)
		}
		setNode(device, "addresses", addresses)
		setNode(device, "dhcp4", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"})
		removeNode(device, "gateway4")
		routes := nodeValue(device, "routes")
		if routes == nil {
			routes = sequence(nil)
		} else if routes.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("NIC %s routes must be a sequence", info.Name)
		}
		kept := make([]*yaml.Node, 0, len(routes.Content))
		for _, route := range routes.Content {
			if route.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("NIC %s has unsupported route", info.Name)
			}
			if !ipv4DefaultRoute(route) {
				kept = append(kept, route)
			}
		}
		routes.Content = kept
		if info.Gateway != "" {
			route := mapping()
			setNode(route, "to", scalar("0.0.0.0/0"))
			setNode(route, "via", scalar(info.Gateway))
			routes.Content = append(routes.Content, route)
		}
		if len(routes.Content) > 0 {
			setNode(device, "routes", routes)
		} else {
			removeNode(device, "routes")
		}
		nameservers, err := ensureMapping(device, "nameservers")
		if err != nil {
			return nil, err
		}
		dns, err := keepIPv6Nodes(nodeValue(nameservers, "addresses"), dnsAddresses(info), false)
		if err != nil {
			return nil, err
		}
		setNode(nameservers, "addresses", dns)
	}
	var out bytes.Buffer
	e := yaml.NewEncoder(&out)
	e.SetIndent(2)
	if err := e.Encode(doc); err != nil {
		return nil, err
	}
	if err := e.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func applyNetplan(ifaces []client.InterfaceInfo, ops Ops) error {
	s, err := readSnapshot(ops, netplanPath, 0600)
	if err != nil {
		return err
	}
	ids, err := managedNetplanIDs(s.data, ifaces)
	if err != nil {
		return err
	}
	if err := checkForeignNetplan(ifaces, ids, ops); err != nil {
		return err
	}
	data, err := generateNetplan(s.data, ifaces)
	if err != nil {
		return fmt.Errorf("generate Netplan: %w", err)
	}
	return applyFile(ops, s, data, func() error {
		if err := runChecked(ops, "netplan", "generate"); err != nil {
			return err
		}
		return runChecked(ops, "netplan", "apply")
	})
}
