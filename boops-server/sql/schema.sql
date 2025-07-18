CREATE DATABASE IF NOT EXISTS boopsdb;
USE boopsdb;

CREATE TABLE machines (
  id CHAR(36) PRIMARY KEY,
  hostname VARCHAR(255) NOT NULL,
  model_info TEXT,
  usage_desc TEXT,
  memo TEXT,
  last_alive DATETIME,
  cpu_info TEXT, -- New field for CPU information
  memory_size TEXT, -- New field for memory size
  disk_info TEXT, -- New field for disk information
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
  FOREIGN KEY (machine_id) REFERENCES machines(id) ON DELETE CASCADE
);
