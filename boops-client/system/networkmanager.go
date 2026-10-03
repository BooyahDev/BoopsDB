package system

import (
	"boops/client"
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

var nmProperties = []string{"ipv4.method", "ipv4.addresses", "ipv4.gateway", "ipv4.dns", "ipv4.never-default", "ipv4.routes"}

type nmPlan struct {
	uuid, device      string
	original, desired []string
}

// nmRoutes accepts nmcli's human route representation and setter representation.
// Unknown attributes stay attached to their original non-default route.
func nmRoutes(raw string, dropDefault bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "--" {
		return "", nil
	}
	var routes []string
	if strings.HasPrefix(raw, "{") {
		for raw != "" {
			raw = strings.TrimLeft(raw, " ;,")
			if raw == "" {
				break
			}
			if raw[0] != '{' {
				return "", fmt.Errorf("unsupported nmcli route output %q", raw)
			}
			end := strings.IndexByte(raw, '}')
			if end < 0 {
				return "", fmt.Errorf("unterminated nmcli route")
			}
			fields := strings.Split(raw[1:end], ",")
			var dest, hop, metric string
			var attrs []string
			for _, field := range fields {
				parts := strings.SplitN(strings.TrimSpace(field), "=", 2)
				if len(parts) != 2 {
					return "", fmt.Errorf("unsupported nmcli route field %q", field)
				}
				key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
				switch key {
				case "ip":
					dest = value
				case "nh":
					hop = value
				case "mt":
					metric = value
				default:
					attrs = append(attrs, key+"="+value)
				}
			}
			prefix, err := netip.ParsePrefix(dest)
			if err != nil || !prefix.Addr().Is4() {
				return "", fmt.Errorf("invalid nmcli IPv4 route %q", dest)
			}
			if !(dropDefault && prefix.Bits() == 0) {
				tokens := []string{dest}
				if hop != "" {
					tokens = append(tokens, hop)
				}
				if metric != "" {
					tokens = append(tokens, metric)
				}
				tokens = append(tokens, attrs...)
				routes = append(routes, strings.Join(tokens, " "))
			}
			raw = raw[end+1:]
		}
	} else {
		for _, route := range strings.Split(raw, ",") {
			route = strings.TrimSpace(route)
			tokens := strings.Fields(route)
			if len(tokens) == 0 {
				continue
			}
			prefix, err := netip.ParsePrefix(tokens[0])
			if err != nil || !prefix.Addr().Is4() {
				return "", fmt.Errorf("invalid nmcli IPv4 route %q", route)
			}
			if !(dropDefault && prefix.Bits() == 0) {
				routes = append(routes, route)
			}
		}
	}
	return strings.Join(routes, ", "), nil
}

func planNetworkManager(ifaces []client.InterfaceInfo, ops Ops) ([]nmPlan, error) {
	out, err := ops.Run("nmcli", "-t", "--escape", "no", "-f", "UUID,DEVICE", "connection", "show", "--active")
	if err != nil {
		return nil, fmt.Errorf("read active NetworkManager connections: %w", err)
	}
	byDevice := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid active connection output")
		}
		if parts[1] == "" || parts[1] == "--" {
			continue
		}
		if _, ok := byDevice[parts[1]]; ok {
			return nil, fmt.Errorf("multiple active connections for NIC %s", parts[1])
		}
		byDevice[parts[1]] = parts[0]
	}
	plans := make([]nmPlan, 0, len(ifaces))
	used := map[string]bool{}
	for _, info := range ifaces {
		uuid, ok := byDevice[info.Name]
		if !ok || uuid == "" {
			return nil, fmt.Errorf("NIC %s has no active NetworkManager connection", info.Name)
		}
		if used[uuid] {
			return nil, fmt.Errorf("connection %s is shared by requested NICs", uuid)
		}
		used[uuid] = true
		p := nmPlan{uuid: uuid, device: info.Name}
		original := map[string]string{}
		for _, property := range nmProperties {
			value, err := ops.Run("nmcli", "-g", property, "connection", "show", "uuid", uuid)
			if err != nil {
				return nil, fmt.Errorf("read %s on NIC %s: %w", property, info.Name, err)
			}
			s := strings.TrimSpace(string(value))
			if s == "--" {
				s = ""
			}
			if property == "ipv4.routes" {
				s, err = nmRoutes(s, false)
				if err != nil {
					return nil, err
				}
			}
			original[property] = s
			p.original = append(p.original, property, s)
		}
		routes, err := nmRoutes(original["ipv4.routes"], true)
		if err != nil {
			return nil, err
		}
		neverDefault := "yes"
		if info.Gateway != "" {
			neverDefault = "no"
		}
		p.desired = []string{"ipv4.method", "manual", "ipv4.addresses", strings.Join(addressCIDRs(info), ","), "ipv4.gateway", info.Gateway, "ipv4.dns", info.DnsServers, "ipv4.never-default", neverDefault, "ipv4.routes", routes}
		plans = append(plans, p)
	}
	return plans, nil
}

func applyNetworkManager(ifaces []client.InterfaceInfo, ops Ops) error {
	plans, err := planNetworkManager(ifaces, ops)
	if err != nil {
		return err
	}
	changed := 0
	rollback := func(cause error) error {
		errs := []error{cause}
		for i := changed - 1; i >= 0; i-- {
			p := plans[i]
			args := append([]string{"connection", "modify", "uuid", p.uuid}, p.original...)
			if err := runChecked(ops, "nmcli", args...); err != nil {
				errs = append(errs, fmt.Errorf("restore NIC %s: %w", p.device, err))
			}
		}
		for i := 0; i < changed; i++ {
			p := plans[i]
			if err := runChecked(ops, "nmcli", "connection", "up", "uuid", p.uuid, "ifname", p.device); err != nil {
				errs = append(errs, fmt.Errorf("reactivate NIC %s: %w", p.device, err))
			}
		}
		return errors.Join(errs...)
	}
	for i, p := range plans {
		changed = i + 1
		args := append([]string{"connection", "modify", "uuid", p.uuid}, p.desired...)
		if err := runChecked(ops, "nmcli", args...); err != nil {
			return rollback(err)
		}
	}
	for _, p := range plans {
		if err := runChecked(ops, "nmcli", "connection", "up", "uuid", p.uuid, "ifname", p.device); err != nil {
			return rollback(err)
		}
	}
	return nil
}
