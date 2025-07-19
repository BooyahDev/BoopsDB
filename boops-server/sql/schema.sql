CREATE DATABASE IF NOT EXISTS boopsdb;
USE boopsdb;

CREATE TABLE machines (
  id CHAR(36) PRIMARY KEY,
  hostname VARCHAR(255) NOT NULL,
  model_info TEXT,
  purpose TEXT, -- Field for machine purpose
  usage_desc TEXT,
  memo TEXT,
  last_alive DATETIME,
  cpu_info TEXT, -- Field for general CPU information
  cpu_arch TEXT, -- New field for CPU architecture information
  memory_size TEXT, -- Field for memory size
  disk_info TEXT, -- Field for disk information
  os_name VARCHAR(255), -- New field for OS name
  is_virtual BOOLEAN DEFAULT FALSE, -- Flag to indicate if this is a virtual machine
  parent_machine_id CHAR(36), -- UUID of the parent machine (if any)
  FOREIGN KEY (parent_machine_id) REFERENCES machines(id) ON DELETE SET NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE interfaces (
  id INT AUTO_INCREMENT PRIMARY KEY,
  machine_id CHAR(36) NOT NULL,
  name VARCHAR(50) NOT NULL,
  ip_address VARCHAR(45) NOT NULL,
  subnet_mask VARCHAR(45) NOT NULL,
  gateway VARCHAR(45),
  dns_servers TEXT, -- Comma-separated list of DNS servers
  mac_address VARCHAR(17), -- MAC address field (e.g., '00:1A:2B:3C:4D:5E')
  FOREIGN KEY (machine_id) REFERENCES machines(id) ON DELETE CASCADE
);
