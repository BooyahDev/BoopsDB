package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

const defaultAPIBase = "https://boopsdb-api.booyah.dev/api"

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type Machine struct {
	ID              string          `json:"id"`
	Hostname        string          `json:"hostname"`
	ModelInfo       string          `json:"model_info"`
	UsageDesc       string          `json:"usage_desc"`
	Memo            string          `json:"memo"`
	Purpose         string          `json:"purpose"`
	LastAlive       string          `json:"last_alive"`
	CPUInfo         string          `json:"cpu_info"`
	CPUArch         string          `json:"cpu_arch"`
	MemorySize      string          `json:"memory_size"`
	DiskInfo        string          `json:"disk_info"`
	OSName          string          `json:"os_name"`
	IsVirtual       Boolish         `json:"is_virtual"`
	ParentMachineID *string         `json:"parent_machine_id"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
	Interfaces      []InterfaceInfo `json:"interfaces"`
}

type InterfaceInfo struct {
	ID         int      `json:"id,omitempty"`
	Name       string   `json:"name"`
	Gateway    string   `json:"gateway"`
	DNSServers string   `json:"dns_servers"`
	MACAddress string   `json:"mac_address"`
	IPs        []IPInfo `json:"ips"`
}

type IPInfo struct {
	IPAddress   string  `json:"ip_address"`
	SubnetMask  string  `json:"subnet_mask"`
	DNSRegister Boolish `json:"dns_register"`
}

type Boolish bool

func (b *Boolish) UnmarshalJSON(data []byte) error {
	var asBool bool
	if err := json.Unmarshal(data, &asBool); err == nil {
		*b = Boolish(asBool)
		return nil
	}

	var asInt int
	if err := json.Unmarshal(data, &asInt); err == nil {
		*b = Boolish(asInt != 0)
		return nil
	}

	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		parsed, err := strconv.ParseBool(asString)
		if err != nil {
			return nil
		}
		*b = Boolish(parsed)
		return nil
	}

	return nil
}

func (b Boolish) Bool() bool {
	return bool(b)
}

type SearchResponse struct {
	Results    []Machine  `json:"results"`
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"hasMore"`
}

type registerPayload struct {
	Hostname        string                      `json:"hostname"`
	ModelInfo       string                      `json:"model_info"`
	UsageDesc       string                      `json:"usage_desc"`
	Memo            string                      `json:"memo"`
	Purpose         string                      `json:"purpose"`
	LastAlive       string                      `json:"last_alive,omitempty"`
	CPUInfo         string                      `json:"cpu_info"`
	CPUArch         string                      `json:"cpu_arch"`
	MemorySize      string                      `json:"memory_size"`
	DiskInfo        string                      `json:"disk_info"`
	OSName          string                      `json:"os_name"`
	IsVirtual       bool                        `json:"is_virtual"`
	ParentMachineID *string                     `json:"parent_machine_id"`
	Interfaces      map[string]interfacePayload `json:"interfaces"`
}

type interfacePayload struct {
	IPs        []ipPayload `json:"ips"`
	Gateway    string      `json:"gateway"`
	DNSServers []string    `json:"dns_servers"`
	MACAddress string      `json:"mac_address"`
}

type ipPayload struct {
	IPAddress   string `json:"ip_address"`
	SubnetMask  string `json:"subnet_mask"`
	DNSRegister bool   `json:"dns_register"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return nil
	}

	switch args[0] {
	case "register":
		return runRegister(args[1:])
	case "search":
		return runSearch(args[1:])
	case "get", "show":
		return runGet(args[1:])
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return nil
	default:
		return runDefault(args)
	}
}

func runDefault(args []string) error {
	opts, err := parseDefaultOptions(args)
	if err != nil {
		return err
	}
	if opts.showHelp {
		return nil
	}

	c := newClient(opts.apiBase)
	if strings.TrimSpace(opts.machineID) != "" {
		return printMachineDetail(c, strings.TrimSpace(opts.machineID))
	}

	query := strings.TrimSpace(strings.Join(opts.queryParts, " "))
	if query == "" {
		reader := newPrompt(os.Stdin)
		query = reader.ask("検索クエリ", "", true)
	}

	return printSearchResults(c, query, opts.limit, opts.offset, opts.sortBy, opts.sortOrder, !opts.noColor)
}

type defaultOptions struct {
	apiBase    string
	machineID  string
	limit      int
	offset     int
	sortBy     string
	sortOrder  string
	noColor    bool
	showHelp   bool
	queryParts []string
}

func parseDefaultOptions(args []string) (defaultOptions, error) {
	opts := defaultOptions{
		apiBase:   defaultAPIBase,
		limit:     50,
		sortBy:    "updated_at",
		sortOrder: "desc",
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			opts.queryParts = append(opts.queryParts, args[i+1:]...)
			break
		}
		if arg == "-h" || arg == "--help" {
			opts.showHelp = true
			return opts, nil
		}
		if arg == "-no-color" || arg == "--no-color" {
			opts.noColor = true
			continue
		}

		name, value, hasInlineValue := strings.Cut(arg, "=")
		switch name {
		case "-api", "--api":
			v, next, err := optionValue("api", value, hasInlineValue, args, i)
			if err != nil {
				return opts, err
			}
			opts.apiBase = v
			i = next
		case "-id", "--id":
			v, next, err := optionValue("id", value, hasInlineValue, args, i)
			if err != nil {
				return opts, err
			}
			opts.machineID = v
			i = next
		case "-limit", "--limit":
			v, next, err := optionValue("limit", value, hasInlineValue, args, i)
			if err != nil {
				return opts, err
			}
			limit, err := strconv.Atoi(v)
			if err != nil {
				return opts, fmt.Errorf("limit must be a number: %q", v)
			}
			opts.limit = limit
			i = next
		case "-offset", "--offset":
			v, next, err := optionValue("offset", value, hasInlineValue, args, i)
			if err != nil {
				return opts, err
			}
			offset, err := strconv.Atoi(v)
			if err != nil {
				return opts, fmt.Errorf("offset must be a number: %q", v)
			}
			opts.offset = offset
			i = next
		case "-sort", "--sort":
			v, next, err := optionValue("sort", value, hasInlineValue, args, i)
			if err != nil {
				return opts, err
			}
			opts.sortBy = v
			i = next
		case "-order", "--order":
			v, next, err := optionValue("order", value, hasInlineValue, args, i)
			if err != nil {
				return opts, err
			}
			opts.sortOrder = v
			i = next
		default:
			opts.queryParts = append(opts.queryParts, arg)
		}
	}

	return opts, nil
}

func optionValue(name string, inlineValue string, hasInlineValue bool, args []string, index int) (string, int, error) {
	if hasInlineValue {
		if inlineValue == "" {
			return "", index, fmt.Errorf("-%s requires a value", name)
		}
		return inlineValue, index, nil
	}
	if index+1 >= len(args) {
		return "", index, fmt.Errorf("-%s requires a value", name)
	}
	return args[index+1], index + 1, nil
}

func runRegister(args []string) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	apiBase := fs.String("api", defaultAPIBase, "API base URL")
	if err := fs.Parse(args); err != nil {
		return err
	}

	reader := newPrompt(os.Stdin)
	payload, err := collectMachine(reader)
	if err != nil {
		return err
	}

	c := newClient(*apiBase)
	id, err := c.createMachine(payload)
	if err != nil {
		return err
	}
	fmt.Printf("registered: %s\n", id)
	return nil
}

func runSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	apiBase := fs.String("api", defaultAPIBase, "API base URL")
	query := fs.String("q", "", "search query")
	limit := fs.Int("limit", 50, "maximum result count")
	offset := fs.Int("offset", 0, "result offset")
	sortBy := fs.String("sort", "updated_at", "search sort field")
	sortOrder := fs.String("order", "desc", "search sort order")
	noColor := fs.Bool("no-color", false, "disable colored relative update time")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *query == "" && fs.NArg() > 0 {
		*query = strings.Join(fs.Args(), " ")
	}
	if strings.TrimSpace(*query) == "" {
		reader := newPrompt(os.Stdin)
		*query = reader.ask("検索クエリ", "", true)
	}

	c := newClient(*apiBase)
	return printSearchResults(c, *query, *limit, *offset, *sortBy, *sortOrder, !*noColor)
}

func printSearchResults(c *Client, query string, limit int, offset int, sortBy string, sortOrder string, color bool) error {
	machines, pagination, err := c.searchMachines(query, limit, offset, sortBy, sortOrder)
	if err != nil {
		return err
	}
	printMachineRows(os.Stdout, machines, color)
	if pagination.Total > 0 {
		fmt.Fprintf(os.Stdout, "\n%d/%d件を表示しています\n", len(machines), pagination.Total)
	}
	return nil
}

func runGet(args []string) error {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	apiBase := fs.String("api", defaultAPIBase, "API base URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: boops-cli get <machine-id>")
	}
	return printMachineDetail(newClient(*apiBase), fs.Arg(0))
}

func newClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *Client) createMachine(payload registerPayload) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/machines", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	var response struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}
	if err := c.doJSON(req, &response); err != nil {
		return "", err
	}
	return response.ID, nil
}

func (c *Client) searchMachines(query string, limit int, offset int, sortBy string, sortOrder string) ([]Machine, Pagination, error) {
	values := url.Values{}
	values.Set("q", query)
	values.Set("limit", strconv.Itoa(limit))
	values.Set("offset", strconv.Itoa(offset))
	values.Set("sort", normalizeSortField(sortBy))
	values.Set("order", normalizeSortOrder(sortOrder))

	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/machines/search?"+values.Encode(), nil)
	if err != nil {
		return nil, Pagination{}, err
	}

	var response SearchResponse
	if err := c.doJSON(req, &response); err == nil && response.Results != nil {
		sortMachines(response.Results, sortBy, sortOrder)
		return response.Results, response.Pagination, nil
	}

	var fallback []Machine
	if err := c.doJSON(req, &fallback); err != nil {
		return nil, Pagination{}, err
	}
	sortMachines(fallback, sortBy, sortOrder)
	total := len(fallback)
	fallback = sliceMachines(fallback, limit, offset)
	return fallback, Pagination{Total: total, Limit: limit, Offset: offset, HasMore: offset+limit < total}, nil
}

func sortMachines(machines []Machine, sortBy string, sortOrder string) {
	field := normalizeSortField(sortBy)
	desc := normalizeSortOrder(sortOrder) == "desc"
	sort.SliceStable(machines, func(i int, j int) bool {
		cmp := compareMachineField(machines[i], machines[j], field)
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareMachineField(a Machine, b Machine, field string) int {
	switch field {
	case "updated_at":
		return compareTimeString(a.UpdatedAt, b.UpdatedAt)
	case "created_at":
		return compareTimeString(a.CreatedAt, b.CreatedAt)
	case "last_alive":
		return compareTimeString(a.LastAlive, b.LastAlive)
	case "hostname":
		return strings.Compare(strings.ToLower(a.Hostname), strings.ToLower(b.Hostname))
	case "os_name":
		return strings.Compare(strings.ToLower(a.OSName), strings.ToLower(b.OSName))
	case "purpose":
		return strings.Compare(strings.ToLower(a.Purpose), strings.ToLower(b.Purpose))
	default:
		return compareTimeString(a.UpdatedAt, b.UpdatedAt)
	}
}

func compareTimeString(a string, b string) int {
	aTime, aOK := parseAPITime(a)
	bTime, bOK := parseAPITime(b)
	switch {
	case aOK && bOK:
		if aTime.Before(bTime) {
			return -1
		}
		if aTime.After(bTime) {
			return 1
		}
		return 0
	case aOK:
		return 1
	case bOK:
		return -1
	default:
		return 0
	}
}

func sliceMachines(machines []Machine, limit int, offset int) []Machine {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = len(machines)
	}
	if offset >= len(machines) {
		return []Machine{}
	}
	end := offset + limit
	if end > len(machines) {
		end = len(machines)
	}
	return machines[offset:end]
}

func normalizeSortField(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "hostname", "last_alive", "created_at", "updated_at", "os_name", "purpose":
		return strings.ToLower(strings.TrimSpace(value))
	case "updated", "update", "updated-at":
		return "updated_at"
	case "created", "created-at":
		return "created_at"
	case "last", "alive", "last-alive":
		return "last_alive"
	case "os":
		return "os_name"
	default:
		return "updated_at"
	}
}

func normalizeSortOrder(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "asc", "ascending":
		return "asc"
	default:
		return "desc"
	}
}

func (c *Client) getMachine(id string) (Machine, error) {
	if !uuidPattern.MatchString(id) {
		return Machine{}, errors.New("machine ID must be a UUID")
	}
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/machines/"+url.PathEscape(id), nil)
	if err != nil {
		return Machine{}, err
	}
	var machine Machine
	if err := c.doJSON(req, &machine); err != nil {
		return Machine{}, err
	}
	return machine, nil
}

func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error != "" {
			return fmt.Errorf("%s: %s", resp.Status, apiErr.Error)
		}
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	return json.Unmarshal(body, out)
}

type prompt struct {
	scanner *bufio.Scanner
}

func newPrompt(r io.Reader) *prompt {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	return &prompt{scanner: scanner}
}

func (p *prompt) ask(label string, def string, required bool) string {
	for {
		if def == "" {
			fmt.Printf("%s: ", label)
		} else {
			fmt.Printf("%s [%s]: ", label, def)
		}
		if !p.scanner.Scan() {
			return def
		}
		value := strings.TrimSpace(p.scanner.Text())
		if value == "" {
			value = def
		}
		if value != "" || !required {
			return value
		}
		fmt.Println("必須項目です。")
	}
}

func (p *prompt) askBool(label string, def bool) bool {
	defaultText := "n"
	if def {
		defaultText = "y"
	}
	for {
		value := strings.ToLower(p.ask(label+" (y/n)", defaultText, false))
		switch value {
		case "y", "yes", "true", "1":
			return true
		case "n", "no", "false", "0":
			return false
		default:
			fmt.Println("y または n を入力してください。")
		}
	}
}

func collectMachine(p *prompt) (registerPayload, error) {
	fmt.Println("マシン情報を入力してください。")
	payload := registerPayload{
		Hostname:   p.ask("Hostname", "", true),
		OSName:     p.ask("OS", "", false),
		CPUInfo:    p.ask("CPU Info", "", false),
		CPUArch:    p.ask("CPU Architecture", "x86_64", false),
		MemorySize: p.ask("Memory Size", "", false),
		DiskInfo:   p.ask("Disk Info", "", false),
		ModelInfo:  p.ask("Model Info", "", false),
		UsageDesc:  p.ask("Usage Description", "", false),
		Purpose:    p.ask("Purpose", "", false),
		Memo:       p.ask("Memo", "", false),
		IsVirtual:  p.askBool("Is Virtual Machine", false),
		Interfaces: map[string]interfacePayload{},
	}

	if payload.IsVirtual {
		parent := p.ask("Parent Machine ID", "", false)
		if parent != "" {
			payload.ParentMachineID = &parent
		}
	}

	for {
		iface, err := collectInterface(p, len(payload.Interfaces)+1)
		if err != nil {
			return registerPayload{}, err
		}
		if _, exists := payload.Interfaces[iface.name]; exists {
			fmt.Printf("interface %q は既に入力済みです。\n", iface.name)
			continue
		}
		payload.Interfaces[iface.name] = iface.payload
		if !p.askBool("インターフェースを追加しますか", false) {
			break
		}
	}

	return payload, nil
}

type namedInterfacePayload struct {
	name    string
	payload interfacePayload
}

func collectInterface(p *prompt, index int) (namedInterfacePayload, error) {
	fmt.Printf("\nInterface %d\n", index)
	name := p.ask("Name", "eth0", true)
	payload := interfacePayload{
		Gateway:    p.ask("Gateway", "", false),
		MACAddress: p.ask("MAC Address", "", false),
		DNSServers: splitCSV(p.ask("DNS Servers (comma separated)", "", false)),
	}

	for {
		ip := ipPayload{
			IPAddress:   p.ask("IP Address", "", true),
			SubnetMask:  p.ask("Subnet Mask", "255.255.255.0", false),
			DNSRegister: p.askBool("DNS Register", false),
		}
		payload.IPs = append(payload.IPs, ip)
		if !p.askBool("IPアドレスを追加しますか", false) {
			break
		}
	}

	return namedInterfacePayload{name: name, payload: payload}, nil
}

func splitCSV(value string) []string {
	var results []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			results = append(results, part)
		}
	}
	return results
}

func printMachineRows(w io.Writer, machines []Machine, color bool) {
	if len(machines) == 0 {
		fmt.Fprintln(w, "No machines found.")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tHOSTNAME\tIP\tTYPE\tUPDATED\tPURPOSE")
	for _, machine := range machines {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			machine.ID,
			valueOrDash(machine.Hostname),
			strings.Join(machine.IPAddresses(), ", "),
			machineTypeIcon(machine),
			formatRelativeUpdate(machine.UpdatedAt, color),
			valueOrDash(machine.Purpose),
		)
	}
	tw.Flush()
}

func printMachineDetail(c *Client, id string) error {
	machine, err := c.getMachine(id)
	if err != nil {
		return err
	}

	fmt.Printf("ID: %s\n", machine.ID)
	fmt.Printf("Hostname: %s\n", valueOrDash(machine.Hostname))
	fmt.Printf("IP: %s\n", strings.Join(machine.IPAddresses(), ", "))
	fmt.Printf("Purpose: %s\n", valueOrDash(machine.Purpose))
	fmt.Printf("Memo: %s\n", valueOrDash(machine.Memo))
	fmt.Printf("OS: %s\n", valueOrDash(machine.OSName))
	fmt.Printf("CPU: %s\n", valueOrDash(machine.CPUInfo))
	fmt.Printf("CPU Arch: %s\n", valueOrDash(machine.CPUArch))
	fmt.Printf("Memory: %s\n", valueOrDash(machine.MemorySize))
	fmt.Printf("Disk: %s\n", valueOrDash(machine.DiskInfo))
	fmt.Printf("Model: %s\n", valueOrDash(machine.ModelInfo))
	fmt.Printf("Usage: %s\n", valueOrDash(machine.UsageDesc))
	fmt.Printf("Virtual: %t\n", machine.IsVirtual.Bool())
	if machine.ParentMachineID != nil && *machine.ParentMachineID != "" {
		fmt.Printf("Parent Machine ID: %s\n", *machine.ParentMachineID)
	}
	fmt.Printf("Last Alive: %s\n", valueOrDash(machine.LastAlive))
	fmt.Printf("Created At: %s\n", valueOrDash(machine.CreatedAt))
	fmt.Printf("Updated At: %s\n", valueOrDash(machine.UpdatedAt))

	if len(machine.Interfaces) == 0 {
		return nil
	}

	fmt.Println("\nInterfaces:")
	for _, iface := range machine.Interfaces {
		fmt.Printf("- %s\n", valueOrDash(iface.Name))
		fmt.Printf("  MAC: %s\n", valueOrDash(iface.MACAddress))
		fmt.Printf("  Gateway: %s\n", valueOrDash(iface.Gateway))
		fmt.Printf("  DNS Servers: %s\n", valueOrDash(iface.DNSServers))
		for _, ip := range iface.IPs {
			fmt.Printf("  IP: %s/%s", valueOrDash(ip.IPAddress), valueOrDash(ip.SubnetMask))
			if ip.DNSRegister.Bool() {
				fmt.Print(" dns_register=true")
			}
			fmt.Println()
		}
	}
	return nil
}

func (m Machine) IPAddresses() []string {
	var ips []string
	for _, iface := range m.Interfaces {
		for _, ip := range iface.IPs {
			if ip.IPAddress == "" {
				continue
			}
			if ip.SubnetMask != "" {
				ips = append(ips, ip.IPAddress+"/"+ip.SubnetMask)
			} else {
				ips = append(ips, ip.IPAddress)
			}
		}
	}
	if len(ips) == 0 {
		return []string{"-"}
	}
	return ips
}

func valueOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func machineTypeIcon(machine Machine) string {
	if machine.IsVirtual.Bool() {
		return "◆ VM"
	}
	return "■ HW"
}

func formatRelativeUpdate(value string, color bool) string {
	parsed, ok := parseAPITime(value)
	if !ok {
		return "-"
	}

	diff := time.Since(parsed)
	if diff < 0 {
		diff = 0
	}

	var text string
	switch {
	case diff < time.Minute:
		text = "たった今"
	case diff < time.Hour:
		text = fmt.Sprintf("%d分前", int(diff.Minutes()))
	case diff < 24*time.Hour:
		text = fmt.Sprintf("%d時間前", int(diff.Hours()))
	default:
		text = fmt.Sprintf("%d日前", int(diff.Hours()/24))
	}

	if !color {
		return text
	}
	if diff <= 5*time.Minute {
		return "\033[32m" + text + "\033[0m"
	}
	return "\033[31m" + text + "\033[0m"
}

func parseAPITime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}

	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `boops - BoopsDB API client

Usage:
  boops register                     対話式でマシンを登録
  boops "query"                      クエリ指定でマシンを検索
  boops -id <machine-id>             マシン詳細を表示
  boops search                       対話式でマシンを検索
  boops search -q "query"            クエリ指定でマシンを検索
  boops search "query"               引数指定でマシンを検索
  boops get <machine-id>             マシン詳細を表示

Options:
  -api <url>     API base URL (default: https://boopsdb-api.booyah.dev/api)
  -id <uuid>     machine ID to show
  -limit <n>     search result limit (default: 50)
  -offset <n>    search result offset (default: 0)
  -sort <field>  search sort field (default: updated_at)
  -order <order> search sort order: asc or desc (default: desc)
  -no-color      disable colored relative update time`)
}
