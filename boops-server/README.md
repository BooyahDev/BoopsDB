# Boops Server

Boops server manages machine information, including network interfaces with multiple IP addresses.

## Database Schema

### Tables:

1. `machines`: Stores machine information
   - id: UUID (Primary Key)
   - hostname: Machine hostname
   - model_info: Machine model information
   - usage_desc: Usage description
   - memo: Notes about the machine
   - purpose: Purpose of the machine
   - last_alive: Last known alive timestamp
   - cpu_info: CPU information
   - cpu_arch: CPU architecture
   - memory_size: Memory size
   - disk_info: Disk information
   - os_name: Operating system name
   - is_virtual: Flag indicating if this is a virtual machine
   - parent_machine_id: UUID of the parent machine (for virtual machines)
   - created_at: Creation timestamp
   - updated_at: Last update timestamp

2. `interfaces`: Stores network interfaces for each machine
   - id: Auto-incrementing ID (Primary Key)
   - machine_id: Machine UUID (Foreign Key to machines.id)
   - name: Interface name
   - gateway: Gateway IP address
   - dns_servers: Comma-separated list of DNS servers
   - mac_address: MAC address

3. `interface_ips`: Stores IP addresses and subnet masks for each interface
   - id: Auto-incrementing ID (Primary Key)
   - interface_id: Interface ID (Foreign Key to interfaces.id)
   - ip_address: IP address
   - subnet_mask: Subnet mask

## API Endpoints

### Machines:

- GET `/api/machines`: Get all machines with their interfaces and IP addresses
- POST `/api/machines`: Create a new machine with interfaces and IP addresses
- PUT `/api/machines/:id`: Update an existing machine and its interfaces/IPs
- DELETE `/api/machines/:id`: Delete a machine and all its interfaces/IPs

### Interfaces:

- POST `/api/machines/:id/interfaces`: Add a new interface to a machine with multiple IPs
- DELETE `/api/machines/:machineId/interfaces/:interfaceName`: Remove an interface from a machine
- PUT `/api/interfaces/:machineId/:interfaceName/ips`: Update IP addresses for an interface

## Data Format Examples

### Create Machine:

```json
{
  "hostname": "test-machine",
  "model_info": "Model XYZ",
  "usage_desc": "Testing",
  "memo": "Test machine for development",
  "purpose": "Development",
  "cpu_info": "Intel i7",
  "memory_size": "16GB",
  "os_name": "Ubuntu 20.04",
  "is_virtual": true,
  "parent_machine_id": "550e8400-e29b-41d4-a716-446655440000",
  "interfaces": {
    "eth0": {
      "ips": [
        { "ip_address": "192.168.1.1", "subnet_mask": "255.255.255.0" },
        { "ip_address": "10.0.0.1", "subnet_mask": "255.0.0.0" }
      ],
      "gateway": "192.168.1.254",
      "dns_servers": ["8.8.8.8", "8.8.4.4"],
      "mac_address": "00:11:22:33:44:55"
    }
  }
}
```

### Add Interface to Machine:

```json
{
  "name": "eth1",
  "ips": [
    { "ip_address": "192.168.2.1", "subnet_mask": "255.255.255.0" },
    { "ip_address": "172.16.0.1", "subnet_mask": "255.240.0.0" }
  ],
  "gateway": "192.168.2.254",
  "dns_servers": ["1.1.1.1"],
  "mac_address": "aa:bb:cc:dd:ee:ff"
}
