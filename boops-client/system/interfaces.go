package system

import (
	"boops/client"
	"errors"
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

// Keep physical indexes so generation can preserve unrelated original bytes.
func interfaceLogicalLines(lines []string) ([]string, []int, error) {
	logical := make([]string, len(lines))
	ends := make([]int, len(lines))
	for i := 0; i < len(lines); i++ {
		start := i
		text := lines[i]
		if !strings.HasPrefix(strings.TrimSpace(text), "#") {
			for strings.HasSuffix(strings.TrimRight(text, " \t\r"), "\\") {
				trimmed := strings.TrimRight(text, " \t\r")
				text = trimmed[:len(trimmed)-1]
				i++
				if i >= len(lines) {
					return nil, nil, fmt.Errorf("unterminated ifupdown continuation at line %d", start+1)
				}
				text += " " + strings.TrimSpace(lines[i])
			}
		}
		logical[start] = text
		ends[start] = i + 1
	}
	return logical, ends, nil
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

func hasIPv4DefaultRouteHook(line string) bool {
	if !hasDefaultRouteHook(line) {
		return false
	}
	// A compound shell hook cannot be classified as IPv6 from one flag.
	if strings.ContainsAny(line, ";&|") {
		return true
	}
	fields := strings.Fields(strings.ToLower(line))
	ipv6 := false
	for _, field := range fields[1:] {
		if field == "-4" || field == "inet" {
			return true
		}
		if field == "-6" || field == "inet6" {
			ipv6 = true
		}
	}
	return !ipv6
}

func validateInterfaceStanza(lines []string, stanza interfaceStanza, targets map[string]client.InterfaceInfo) error {
	physical := strings.SplitN(stanza.name, ":", 2)[0]
	info, target := targets[physical]
	if !target {
		return nil
	}
	for _, field := range strings.Fields(lines[stanza.start])[4:] {
		if strings.HasPrefix(field, "#") {
			break
		}
		if field == "inherits" {
			return fmt.Errorf("NIC %s has inherited ifupdown configuration", info.Name)
		}
	}
	for _, line := range lines[stanza.start+1 : stanza.end] {
		if fields := strings.Fields(line); len(fields) > 0 && fields[0] == "inherits" {
			return fmt.Errorf("NIC %s has inherited ifupdown configuration", info.Name)
		}
		if hasIPv4DefaultRouteHook(line) {
			return fmt.Errorf("NIC %s has a custom IPv4 default-route hook in %s stanza", info.Name, stanza.family)
		}
	}
	return nil
}

var sourceDirectoryName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func checkInterfacesIncludes(ops Ops, path string, data []byte, targets map[string]client.InterfaceInfo, visited map[string]bool, definitions map[string]bool) error {
	if visited[path] {
		return nil
	}
	visited[path] = true
	lines, _, err := interfaceLogicalLines(strings.Split(string(data), "\n"))
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
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
		definitions[stanza.name] = true
		if err := validateInterfaceStanza(lines, stanza, targets); err != nil {
			return err
		}
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
				if err := checkInterfacesIncludes(ops, include, bytes, targets, visited, definitions); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func generateInterfaces(data []byte, ifaces []client.InterfaceInfo) ([]byte, error) {
	lines := strings.Split(string(data), "\n")
	logical, ends, err := interfaceLogicalLines(lines)
	if err != nil {
		return nil, err
	}
	targets := map[string]client.InterfaceInfo{}
	for _, info := range ifaces {
		targets[info.Name] = info
	}
	stanzas := interfaceStanzas(logical)
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
		for _, field := range strings.Fields(logical[stanza.start])[4:] {
			if strings.HasPrefix(field, "#") {
				break
			}
			if field == "inherits" {
				return nil, fmt.Errorf("NIC %s has inherited ifupdown configuration", stanza.name)
			}
		}
		for _, line := range logical[stanza.start+1 : stanza.end] {
			if fields := strings.Fields(line); len(fields) > 0 && fields[0] == "inherits" {
				return nil, fmt.Errorf("NIC %s has inherited ifupdown configuration", stanza.name)
			}
			if hasIPv4DefaultRouteHook(line) {
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
		for j := ends[i]; j < stanza.end; j++ {
			fields := strings.Fields(logical[j])
			if len(fields) > 0 {
				switch fields[0] {
				case "address", "netmask", "gateway", "dns-nameservers":
					j = ends[j] - 1
					continue
				}
			}
			out = append(out, lines[j])
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

func readIfupdownState(ops Ops, ifaces []client.InterfaceInfo) (map[string]string, error) {
	path := "/run/network/ifstate"
	exists, err := ops.Exists(path)
	if err != nil {
		return nil, err
	}
	state := map[string]string{}
	if !exists {
		return state, nil
	}
	data, err := ops.ReadFile(path)
	if err != nil {
		return nil, err
	}
	requested := map[string]bool{}
	for _, info := range ifaces {
		requested[info.Name] = true
	}
	for _, line := range strings.Split(string(data), "\n") {
		physical, logical, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !requested[physical] {
			continue
		}
		if !ok || logical == "" || strings.ContainsAny(logical, "= \t\r\n") {
			return nil, fmt.Errorf("invalid ifupdown state for NIC %s", physical)
		}
		if _, duplicate := state[physical]; duplicate {
			return nil, fmt.Errorf("duplicate ifupdown state for NIC %s", physical)
		}
		state[physical] = logical
	}
	return state, nil
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
	state, err := readIfupdownState(ops, ifaces)
	if err != nil {
		return err
	}
	for _, info := range ifaces {
		if logical, ok := state[info.Name]; ok {
			targets[logical] = info
		}
	}
	definitions := map[string]bool{}
	if err := checkInterfacesIncludes(ops, interfacesPath, s.data, targets, map[string]bool{}, definitions); err != nil {
		return err
	}
	var oldNames []string
	for _, info := range ifaces {
		if logical, active := state[info.Name]; active {
			if !definitions[logical] {
				return fmt.Errorf("NIC %s has ifupdown state %s without an existing definition", info.Name, logical)
			}
			oldNames = append(oldNames, info.Name)
		} else if definitions[info.Name] {
			oldNames = append(oldNames, info.Name)
		}
	}
	data, err := generateInterfaces(s.data, ifaces)
	if err != nil {
		return err
	}
	runInterfaces := func(tool string, nics []string) error {
		if len(nics) == 0 {
			return nil
		}
		return runChecked(ops, tool, append([]string{"--force", "--"}, nics...)...)
	}
	down := func() error { return runInterfaces("ifdown", names) }
	up := func() error { return runInterfaces("ifup", names) }
	reactivateOriginal := func(cause error) error {
		if err := runInterfaces("ifup", oldNames); err != nil {
			return errors.Join(cause, fmt.Errorf("reactivate original ifupdown configuration: %w", err))
		}
		return cause
	}
	// ifdown needs the original method/options to release the old configuration.
	if err := runInterfaces("ifdown", oldNames); err != nil {
		return reactivateOriginal(err)
	}
	if err := replaceFile(ops, s.path, data, s.mode); err != nil {
		return reactivateOriginal(err)
	}
	if err := up(); err != nil {
		errs := []error{err}
		// Remove any partly activated new settings while their file is present.
		if stopErr := down(); stopErr != nil {
			errs = append(errs, fmt.Errorf("stop failed replacement configuration: %w", stopErr))
		}
		if restoreErr := replaceFile(ops, s.path, s.data, s.mode); restoreErr != nil {
			return errors.Join(append(errs, fmt.Errorf("restore %s: %w", s.path, restoreErr))...)
		}
		if restoreErr := runInterfaces("ifup", oldNames); restoreErr != nil {
			errs = append(errs, fmt.Errorf("reactivate restored ifupdown configuration: %w", restoreErr))
		}
		return errors.Join(errs...)
	}
	return nil
}
