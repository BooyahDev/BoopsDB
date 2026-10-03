package system

import (
	"boops/client"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const interfacesPath = "/etc/network/interfaces"

type interfaceStanza struct {
	start, end   int
	name, family string
}

func interfaceStanzas(lines []string) []interfaceStanza {
	var result []interfaceStanza
	active := -1
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		boundary := fields[0] == "iface" || fields[0] == "auto" || strings.HasPrefix(fields[0], "allow-") || fields[0] == "mapping" || fields[0] == "source" || fields[0] == "source-directory"
		if boundary && active >= 0 {
			result[active].end = i
			active = -1
		}
		if fields[0] == "iface" && len(fields) >= 4 {
			result = append(result, interfaceStanza{start: i, end: len(lines), name: fields[1], family: fields[2]})
			active = len(result) - 1
		}
	}
	return result
}

func hasDefaultRouteHook(line string) bool {
	fields := strings.Fields(strings.ToLower(line))
	if len(fields) < 2 {
		return false
	}
	switch fields[0] {
	case "up", "pre-up", "post-up", "down", "pre-down", "post-down":
	default:
		return false
	}
	s := strings.Join(fields[1:], " ")
	return (strings.Contains(s, "route") || strings.Contains(s, "ip r ")) && (strings.Contains(s, "default") || strings.Contains(s, "0.0.0.0"))
}

var sourceDirectoryName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func checkInterfacesIncludes(ops Ops, path string, data []byte, targets map[string]client.InterfaceInfo, visited map[string]bool) error {
	if visited[path] {
		return nil
	}
	visited[path] = true
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 1 && fields[0] == "mapping" {
			for _, pattern := range fields[1:] {
				if strings.HasPrefix(pattern, "#") {
					break
				}
				for name := range targets {
					matched, err := filepath.Match(pattern, name)
					if err != nil {
						return err
					}
					if matched {
						return fmt.Errorf("NIC %s has an ifupdown mapping in %s", name, path)
					}
				}
			}
		}
	}
	for _, stanza := range interfaceStanzas(lines) {
		physical := strings.SplitN(stanza.name, ":", 2)[0]
		if _, ok := targets[physical]; ok && physical != stanza.name && stanza.family == "inet" {
			return fmt.Errorf("NIC %s has a separate ifupdown alias %s in %s", physical, stanza.name, path)
		}
	}
	if path != interfacesPath {
		for _, stanza := range interfaceStanzas(lines) {
			if _, ok := targets[stanza.name]; ok && stanza.family == "inet" {
				return fmt.Errorf("NIC %s has included ifupdown definition in %s", stanza.name, path)
			}
		}
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 || (fields[0] != "source" && fields[0] != "source-directory") {
			continue
		}
		for _, pattern := range fields[1:] {
			if strings.HasPrefix(pattern, "#") {
				break
			}
			if strings.ContainsAny(pattern, "$`\\\"'") {
				return fmt.Errorf("cannot safely resolve ifupdown include %q", pattern)
			}
			if !filepath.IsAbs(pattern) {
				pattern = filepath.Join(filepath.Dir(path), pattern)
			}
			paths, err := ops.Glob(pattern)
			if err != nil {
				return err
			}
			if fields[0] == "source-directory" {
				var files []string
				for _, dir := range paths {
					names, err := ops.Glob(filepath.Join(dir, "*"))
					if err != nil {
						return err
					}
					for _, name := range names {
						if sourceDirectoryName.MatchString(filepath.Base(name)) {
							files = append(files, name)
						}
					}
				}
				paths = files
			}
			for _, include := range paths {
				bytes, err := ops.ReadFile(include)
				if err != nil {
					return fmt.Errorf("read include %s: %w", include, err)
				}
				if err := checkInterfacesIncludes(ops, include, bytes, targets, visited); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func generateInterfaces(data []byte, ifaces []client.InterfaceInfo) ([]byte, error) {
	lines := strings.Split(string(data), "\n")
	targets := map[string]client.InterfaceInfo{}
	for _, info := range ifaces {
		targets[info.Name] = info
	}
	stanzas := interfaceStanzas(lines)
	byStart := map[int]interfaceStanza{}
	seen := map[string]bool{}
	for _, stanza := range stanzas {
		if _, ok := targets[stanza.name]; !ok || stanza.family != "inet" {
			continue
		}
		if seen[stanza.name] {
			return nil, fmt.Errorf("NIC %s has multiple IPv4 ifupdown definitions", stanza.name)
		}
		seen[stanza.name] = true
		byStart[stanza.start] = stanza
		for _, line := range lines[stanza.start+1 : stanza.end] {
			if hasDefaultRouteHook(line) {
				return nil, fmt.Errorf("NIC %s has a custom default-route hook", stanza.name)
			}
		}
	}
	settings := func(info client.InterfaceInfo) []string {
		var result []string
		for _, addr := range addressCIDRs(info) {
			result = append(result, "    address "+addr)
		}
		if info.Gateway != "" {
			result = append(result, "    gateway "+info.Gateway)
		}
		if dns := dnsAddresses(info); len(dns) > 0 {
			result = append(result, "    dns-nameservers "+strings.Join(dns, " "))
		}
		return result
	}
	var out []string
	for i := 0; i < len(lines); i++ {
		stanza, ok := byStart[i]
		if !ok {
			out = append(out, lines[i])
			continue
		}
		info := targets[stanza.name]
		out = append(out, "iface "+info.Name+" inet static")
		out = append(out, settings(info)...)
		for _, line := range lines[i+1 : stanza.end] {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				switch fields[0] {
				case "address", "netmask", "gateway", "dns-nameservers":
					continue
				}
			}
			out = append(out, line)
		}
		i = stanza.end - 1
	}
	for _, info := range ifaces {
		if seen[info.Name] {
			continue
		}
		out = append(out, "", "auto "+info.Name, "iface "+info.Name+" inet static")
		out = append(out, settings(info)...)
	}
	return []byte(strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"), nil
}

func applyInterfaces(ifaces []client.InterfaceInfo, ops Ops) error {
	for _, tool := range []string{"ifdown", "ifup"} {
		if _, err := ops.LookPath(tool); err != nil {
			return fmt.Errorf("ifupdown requires %s: %w", tool, err)
		}
	}
	s, err := readSnapshot(ops, interfacesPath, 0644)
	if err != nil {
		return err
	}
	targets := map[string]client.InterfaceInfo{}
	names := make([]string, len(ifaces))
	for i, info := range ifaces {
		targets[info.Name] = info
		names[i] = info.Name
	}
	if err := checkInterfacesIncludes(ops, interfacesPath, s.data, targets, map[string]bool{}); err != nil {
		return err
	}
	data, err := generateInterfaces(s.data, ifaces)
	if err != nil {
		return err
	}
	return applyFile(ops, s, data, func() error {
		if err := runChecked(ops, "ifdown", append([]string{"--force", "--"}, names...)...); err != nil {
			return err
		}
		return runChecked(ops, "ifup", append([]string{"--force", "--"}, names...)...)
	})
}
