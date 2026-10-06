package system

import (
	"boops/client"
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
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

type interfaceFiles struct {
	files        map[string]fileSnapshot
	targetIPv4   map[string]string
	targetAny    map[string]string
	definitions  map[string]bool
	auto         map[string]bool
	bridge       map[string]bool
	dependencies map[string][]string
}

func collectInterfacesFile(ops Ops, snapshot fileSnapshot, targets map[string]client.InterfaceInfo, visited map[string]bool, result *interfaceFiles) error {
	path := filepath.Clean(snapshot.path)
	if visited[path] {
		return nil
	}
	visited[path] = true
	snapshot.path = path
	result.files[path] = snapshot
	data := snapshot.data
	lines, _, err := interfaceLogicalLines(strings.Split(string(data), "\n"))
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 1 && (fields[0] == "auto" || fields[0] == "allow-auto") {
			for _, name := range fields[1:] {
				if strings.HasPrefix(name, "#") {
					break
				}
				result.auto[strings.SplitN(name, ":", 2)[0]] = true
			}
		}
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
		result.definitions[stanza.name] = true
		if err := validateInterfaceStanza(lines, stanza, targets); err != nil {
			return err
		}
		physical := strings.SplitN(stanza.name, ":", 2)[0]
		if _, ok := targets[physical]; ok && physical != stanza.name && stanza.family == "inet" {
			return fmt.Errorf("NIC %s has a separate ifupdown alias %s in %s", physical, stanza.name, path)
		}
		var stanzaDependencies []string
		bridgeStanza := false
		for _, line := range lines[stanza.start+1 : stanza.end] {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			var count int
			switch fields[0] {
			case "bridge-ports", "bridge_ports", "bond-slaves", "bond_slaves":
				if fields[0] == "bridge-ports" || fields[0] == "bridge_ports" {
					bridgeStanza = true
				}
				count = len(fields) - 1
			case "vlan-raw-device", "vlan_raw_device":
				count = 1
			}
			for _, dependency := range fields[1 : 1+count] {
				if strings.HasPrefix(dependency, "#") {
					break
				}
				if dependency != "none" {
					stanzaDependencies = append(stanzaDependencies, dependency)
				}
			}
		}
		if len(stanzaDependencies) > 0 {
			result.dependencies[stanza.name] = append(result.dependencies[stanza.name], stanzaDependencies...)
		}
		if bridgeStanza {
			result.bridge[stanza.name] = true
		}
		info, target := targets[stanza.name]
		if !target {
			info, target = targets[physical]
		}
		if target {
			result.targetAny[info.Name] = path
			if stanza.family == "inet" {
				if previous, duplicate := result.targetIPv4[info.Name]; duplicate {
					return fmt.Errorf("NIC %s has multiple IPv4 ifupdown definitions in %s and %s", info.Name, previous, path)
				}
				result.targetIPv4[info.Name] = path
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
				included, err := readSnapshot(ops, include, 0644)
				if err != nil {
					return fmt.Errorf("read include %s: %w", include, err)
				}
				if !included.exists {
					continue
				}
				if err := collectInterfacesFile(ops, included, targets, visited, result); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func collectInterfaces(ops Ops, root fileSnapshot, targets map[string]client.InterfaceInfo) (interfaceFiles, error) {
	result := interfaceFiles{
		files:        map[string]fileSnapshot{},
		targetIPv4:   map[string]string{},
		targetAny:    map[string]string{},
		definitions:  map[string]bool{},
		auto:         map[string]bool{},
		bridge:       map[string]bool{},
		dependencies: map[string][]string{},
	}
	if err := collectInterfacesFile(ops, root, targets, map[string]bool{}, &result); err != nil {
		return interfaceFiles{}, err
	}
	return result, nil
}

// interfacesOwnRequestedNICs reports whether ifupdown already defines any of
// the requested devices in the main file or its source/source-directory graph.
// Network backend selection uses this to ignore an empty placeholder file on
// Netplan or NetworkManager hosts while still selecting ifupdown for existing
// Debian/Proxmox definitions.
func interfacesOwnRequestedNICs(ifaces []client.InterfaceInfo, ops Ops) (bool, error) {
	snapshot, err := readSnapshot(ops, interfacesPath, 0644)
	if err != nil || !snapshot.exists {
		return false, err
	}
	targets := make(map[string]client.InterfaceInfo, len(ifaces))
	for _, info := range ifaces {
		targets[info.Name] = info
	}
	configs, err := collectInterfaces(ops, snapshot, targets)
	if err != nil {
		return false, err
	}
	for _, info := range ifaces {
		if _, ok := configs.targetAny[info.Name]; ok || interfaceReferencedAsDependency(info.Name, configs) {
			return true, nil
		}
	}
	return false, nil
}

// checkInterfacesIncludes validates the complete ifupdown include graph.
// Callers that need to update included files should use collectInterfaces.
func checkInterfacesIncludes(ops Ops, path string, data []byte, targets map[string]client.InterfaceInfo, visited map[string]bool, definitions map[string]bool) error {
	result := interfaceFiles{
		files:        map[string]fileSnapshot{},
		targetIPv4:   map[string]string{},
		targetAny:    map[string]string{},
		definitions:  definitions,
		auto:         map[string]bool{},
		bridge:       map[string]bool{},
		dependencies: map[string][]string{},
	}
	return collectInterfacesFile(ops, fileSnapshot{path: path, data: data, mode: 0644, exists: true}, targets, visited, &result)
}

func validateIfupdownNames(ifaces []client.InterfaceInfo) error {
	for _, info := range ifaces {
		if strings.IndexFunc(info.Name, unicode.IsSpace) >= 0 {
			return fmt.Errorf("ifupdown cannot represent NIC name %q containing whitespace", info.Name)
		}
	}
	return nil
}

func interfaceCoveredByAuto(name string, configs interfaceFiles, visiting map[string]bool) bool {
	if configs.auto[name] {
		return true
	}
	if visiting[name] {
		return false
	}
	visiting[name] = true
	defer delete(visiting, name)
	for parent, members := range configs.dependencies {
		for _, member := range members {
			if member == name && interfaceCoveredByAuto(parent, configs, visiting) {
				return true
			}
		}
	}
	return false
}

func interfaceHasBridgeAncestor(name string, configs interfaceFiles, visiting map[string]bool) bool {
	if configs.bridge[name] {
		return true
	}
	if visiting[name] {
		return false
	}
	visiting[name] = true
	defer delete(visiting, name)
	for parent, members := range configs.dependencies {
		for _, member := range members {
			if member == name && interfaceHasBridgeAncestor(parent, configs, visiting) {
				return true
			}
		}
	}
	return false
}

func interfaceReferencedAsDependency(name string, configs interfaceFiles) bool {
	for _, members := range configs.dependencies {
		for _, member := range members {
			if member == name {
				return true
			}
		}
	}
	return false
}

func generateInterfacesWithMissing(data []byte, ifaces []client.InterfaceInfo, appendMissing bool) ([]byte, error) {
	if err := validateIfupdownNames(ifaces); err != nil {
		return nil, err
	}
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
	const managedAddressMarker = "# boops-managed-address "
	settings := func(info client.InterfaceInfo) []string {
		var result []string
		addresses := addressCIDRs(info)
		if len(addresses) > 0 {
			// The classic Debian ifupdown static method documents one address
			// option per stanza. Keep the primary address portable and install
			// additional addresses with idempotent iproute2 hooks.
			result = append(result, "    address "+addresses[0])
			for _, addr := range addresses[1:] {
				result = append(result,
					"    "+managedAddressMarker+addr,
					"    up ip -4 addr replace "+addr+" dev \"$IFACE\"",
					"    down ip -4 addr del "+addr+" dev \"$IFACE\" || true",
				)
			}
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
		method := "static"
		if len(info.IPs) == 0 {
			method = "manual"
		}
		out = append(out, "iface "+info.Name+" inet "+method)
		out = append(out, settings(info)...)
		for j := ends[i]; j < stanza.end; j++ {
			if marker := strings.TrimSpace(logical[j]); strings.HasPrefix(marker, managedAddressMarker) {
				addr := strings.TrimSpace(strings.TrimPrefix(marker, managedAddressMarker))
				last := j
				for _, hook := range []string{
					"up ip -4 addr replace " + addr + " dev \"$IFACE\"",
					"down ip -4 addr del " + addr + " dev \"$IFACE\" || true",
				} {
					next := last + 1
					if next >= stanza.end || strings.TrimSpace(logical[next]) != hook {
						break
					}
					last = ends[next] - 1
				}
				j = last
				continue
			}
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
	if appendMissing {
		for _, info := range ifaces {
			if seen[info.Name] {
				continue
			}
			method := "static"
			if len(info.IPs) == 0 {
				method = "manual"
			}
			out = append(out, "", "auto "+info.Name, "iface "+info.Name+" inet "+method)
			out = append(out, settings(info)...)
		}
	}
	return []byte(strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"), nil
}

func generateInterfaces(data []byte, ifaces []client.InterfaceInfo) ([]byte, error) {
	return generateInterfacesWithMissing(data, ifaces, true)
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
		// A physical name alone cannot reactivate a different logical definition
		// after ifdown clears its state, so leave that configuration untouched.
		if logical != physical {
			return nil, fmt.Errorf("NIC %s uses unsupported ifupdown logical name %s", physical, logical)
		}
		state[physical] = logical
	}
	return state, nil
}

func applyInterfaces(ifaces []client.InterfaceInfo, ops Ops) error {
	if err := validateIfupdownNames(ifaces); err != nil {
		return err
	}
	_, reloadErr := ops.LookPath("ifreload")
	if reloadErr != nil {
		for _, tool := range []string{"ifdown", "ifup"} {
			if _, err := ops.LookPath(tool); err != nil {
				return fmt.Errorf("ifupdown requires %s: %w", tool, err)
			}
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
	configs, err := collectInterfaces(ops, s, targets)
	if err != nil {
		return err
	}
	for _, info := range ifaces {
		if _, direct := configs.targetAny[info.Name]; !direct && interfaceReferencedAsDependency(info.Name, configs) {
			return fmt.Errorf("NIC %s is referenced by an existing ifupdown topology without its own stanza; refusing to create a standalone configuration", info.Name)
		}
	}
	reloadTargetAuto := func(info client.InterfaceInfo) bool {
		if _, existingIPv4 := configs.targetIPv4[info.Name]; !existingIPv4 {
			return true
		}
		return interfaceCoveredByAuto(info.Name, configs, map[string]bool{})
	}
	var reloadAutoCount, reloadCurrentlyUpCount int
	var reloadInactiveNames []string
	if reloadErr != nil {
		for _, info := range ifaces {
			if interfaceHasBridgeAncestor(info.Name, configs, map[string]bool{}) {
				return fmt.Errorf("NIC %s has a bridge configuration; ifreload/ifupdown2 is required to avoid an ifdown bridge outage", info.Name)
			}
		}
	}
	if reloadErr == nil {
		for _, info := range ifaces {
			// A target without an existing IPv4 stanza receives a new auto
			// stanza below. Existing non-auto stanzas are reloaded with
			// --currently-up when active, or activated explicitly below.
			auto := reloadTargetAuto(info)
			if auto {
				reloadAutoCount++
				continue
			}
			if _, active := state[info.Name]; active {
				reloadCurrentlyUpCount++
			} else {
				reloadInactiveNames = append(reloadInactiveNames, info.Name)
			}
		}
		if len(reloadInactiveNames) > 0 {
			if _, err := ops.LookPath("ifup"); err != nil {
				return fmt.Errorf("ifupdown requires ifup to activate non-auto NICs: %w", err)
			}
			if _, err := ops.LookPath("ifdown"); err != nil {
				return fmt.Errorf("ifupdown requires ifdown to roll back non-auto NIC activation: %w", err)
			}
		}
	}
	var oldNames, originallyActiveNames []string
	for _, info := range ifaces {
		if logical, active := state[info.Name]; active {
			if !configs.definitions[logical] {
				return fmt.Errorf("NIC %s has ifupdown state %s without an existing definition", info.Name, logical)
			}
			oldNames = append(oldNames, info.Name)
			originallyActiveNames = append(originallyActiveNames, info.Name)
		} else if configs.definitions[info.Name] {
			oldNames = append(oldNames, info.Name)
		}
	}
	paths := make([]string, 0, len(configs.files))
	for path := range configs.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	changes := make([]fileChange, 0, len(paths))
	for _, path := range paths {
		snapshot := configs.files[path]
		var local []client.InterfaceInfo
		for _, info := range ifaces {
			updatePath, ok := configs.targetIPv4[info.Name]
			if !ok {
				updatePath = configs.targetAny[info.Name]
			}
			if !ok && updatePath == "" {
				updatePath = interfacesPath
			}
			if updatePath == path {
				local = append(local, info)
			}
		}
		if len(local) == 0 {
			continue
		}
		data, err := generateInterfacesWithMissing(snapshot.data, local, true)
		if err != nil {
			return err
		}
		if bytes.Equal(data, snapshot.data) {
			continue
		}
		changes = append(changes, fileChange{before: snapshot, data: data})
	}
	if len(changes) == 0 {
		return nil
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
		if err := runInterfaces("ifup", originallyActiveNames); err != nil {
			return errors.Join(cause, fmt.Errorf("reactivate original ifupdown configuration: %w", err))
		}
		return cause
	}
	if reloadErr == nil {
		applyAttempts := 0
		return applyFiles(ops, changes, func() error {
			applyAttempts++
			if reloadAutoCount > 0 {
				if err := runChecked(ops, "ifreload", "-a"); err != nil {
					return err
				}
			}
			if reloadCurrentlyUpCount > 0 {
				if err := runChecked(ops, "ifreload", "-c"); err != nil {
					return err
				}
			}
			if applyAttempts > 1 || len(reloadInactiveNames) == 0 {
				return nil
			}
			if err := runInterfaces("ifup", reloadInactiveNames); err != nil {
				if cleanupErr := runInterfaces("ifdown", reloadInactiveNames); cleanupErr != nil {
					return errors.Join(err, fmt.Errorf("stop partially activated non-auto NICs: %w", cleanupErr))
				}
				return err
			}
			return nil
		})
	}
	// ifdown needs the original method/options to release the old configuration.
	if err := runInterfaces("ifdown", oldNames); err != nil {
		return reactivateOriginal(err)
	}
	applyAttempts := 0
	err = applyFiles(ops, changes, func() error {
		applyAttempts++
		if applyAttempts > 1 {
			return runInterfaces("ifup", originallyActiveNames)
		}
		if err := up(); err != nil {
			// Remove any partly activated new settings while their files are present.
			if stopErr := down(); stopErr != nil {
				return errors.Join(err, fmt.Errorf("stop failed replacement configuration: %w", stopErr))
			}
			return err
		}
		return nil
	})
	if err != nil && applyAttempts == 0 {
		return reactivateOriginal(err)
	}
	return err
}
