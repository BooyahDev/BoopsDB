package client

type InterfaceInfo struct {
	IP         string   `json:"ip"`
	Subnet     string   `json:"subnet"`
	Gateway    string   `json:"gateway"`
	DNSServers []string `json:"dns_servers"`
	MACAddress string   `json:"mac_address"`
}

type Machine struct {
	ID             string                  `json:"id"`
	Hostname       string                  `json:"hostname"`
	CPUInfo        string                  `json:"cpu_info"`
	CPUArch        string                  `json:"cpu_arch"`
	MemorySize     string                  `json:"memory_size"`
	DiskInfo       string                  `json:"disk_info"`
	OSName         string                  `json:"os_name"`
	IsVirtual      int                     `json:"is_virtual"`
	ParentMachine  string                  `json:"parent_machine_id"`
	Interfaces     map[string]InterfaceInfo `json:"interfaces"`
}