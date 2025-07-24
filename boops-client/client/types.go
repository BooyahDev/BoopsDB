package client

type Machine struct {
	ID            string          `json:"id"`
	Hostname      string          `json:"hostname"`
	ModelInfo     interface{}     `json:"model_info"`
	Purpose       string          `json:"purpose"`
	UsageDesc     interface{}     `json:"usage_desc"`
	Memo          string          `json:"memo"`
	LastAlive     interface{}     `json:"last_alive"`
	CpuInfo       string          `json:"cpu_info"`
	CpuArch       string          `json:"cpu_arch"`
	MemorySize    string          `json:"memory_size"`
	DiskInfo      string          `json:"disk_info"`
	OsName        string          `json:"os_name"`
	IsVirtual     int             `json:"is_virtual"`
	ParentMachine *string         `json:"parent_machine_id"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
	Interfaces    []InterfaceInfo `json:"interfaces"`
}

type InterfaceInfo struct {
	IPs        []IPInfo `json:"ips"`
	Gateway    string   `json:"gateway"`
	DnsServers string   `json:"dns_servers,omitempty"` // Receive as comma-separated string from API
	MacAddress string   `json:"mac_address,omitempty"`
}

type IPInfo struct {
	IP     string `json:"ip_address"`
	Subnet string `json:"subnet_mask"`
}
