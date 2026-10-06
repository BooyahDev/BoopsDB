package system

import (
	"boops/client"
	"bytes"
	"fmt"
	"io"
	"net/netip"
	"path/filepath"
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
		if !ok {
			if !renamed {
				return false, nil
			}
			if nodeValue(match, "macaddress") == nil {
				return false, fmt.Errorf("cannot verify renamed NIC %s in definition %s without MAC match", info.Name, id)
			}
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

func cloneNetplanForInspection(n *yaml.Node, depth int) (*yaml.Node, error) {
	if n == nil || depth > 128 {
		return nil, fmt.Errorf("unsupported recursive Netplan YAML")
	}
	if n.Kind == yaml.AliasNode {
		return cloneNetplanForInspection(n.Alias, depth+1)
	}
	if n.Kind == yaml.MappingNode && nodeValue(n, "<<") != nil {
		return nil, fmt.Errorf("cannot safely inspect foreign Netplan YAML merge")
	}
	copy := *n
	copy.Anchor = ""
	copy.Alias = nil
	copy.Content = nil
	for _, child := range n.Content {
		cloned, err := cloneNetplanForInspection(child, depth+1)
		if err != nil {
			return nil, err
		}
		copy.Content = append(copy.Content, cloned)
	}
	return &copy, nil
}

// Netplan combines maps recursively, appends sequences, and replaces scalars.
// Both inputs are private inspection copies, never nodes from a saved file.
func mergeInspectedNetplan(previous, next *yaml.Node) *yaml.Node {
	if previous == nil {
		return next
	}
	if previous.Kind == yaml.MappingNode && next.Kind == yaml.MappingNode {
		for i := 0; i < len(next.Content); i += 2 {
			key := next.Content[i].Value
			setNode(previous, key, mergeInspectedNetplan(nodeValue(previous, key), next.Content[i+1]))
		}
		return previous
	}
	if previous.Kind == yaml.SequenceNode && next.Kind == yaml.SequenceNode {
		previous.Content = append(previous.Content, next.Content...)
		return previous
	}
	return next
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

func updateNetplanDevice(device *yaml.Node, info client.InterfaceInfo) error {
	addresses, err := keepIPv6Nodes(nodeValue(device, "addresses"), addressCIDRs(info), true)
	if err != nil {
		return fmt.Errorf("NIC %s: %w", info.Name, err)
	}
	setNode(device, "addresses", addresses)
	setNode(device, "dhcp4", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"})
	removeNode(device, "gateway4")
	routes := nodeValue(device, "routes")
	if routes == nil {
		routes = sequence(nil)
	} else if routes.Kind != yaml.SequenceNode {
		return fmt.Errorf("NIC %s routes must be a sequence", info.Name)
	}
	kept := make([]*yaml.Node, 0, len(routes.Content))
	for _, route := range routes.Content {
		if route.Kind != yaml.MappingNode {
			return fmt.Errorf("NIC %s has unsupported route", info.Name)
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
		return err
	}
	dns, err := keepIPv6Nodes(nodeValue(nameservers, "addresses"), dnsAddresses(info), false)
	if err != nil {
		return err
	}
	setNode(nameservers, "addresses", dns)
	return nil
}
