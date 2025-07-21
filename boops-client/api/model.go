package api

type InterfaceInfo struct {
    IPAddress    string `json:"ipAddress"`
    SubnetMask   string `json:"subnetMask"`
    Gateway      string `json:"gateway"`
    MACAddress   string `json:"macAddress"`
    DNS          string `json:"dns"`
}

type MachineInfo struct {
    ID          string                    `json:"id"`
    Hostname    string                    `json:"hostname"`
    OS          string                    `json:"os"`
    CPU         string                    `json:"cpu"`
    Memory      string                    `json:"memory"`
    Disk        string                    `json:"disk"`
    Interfaces  map[string]InterfaceInfo `json:"interfaces"`
}

