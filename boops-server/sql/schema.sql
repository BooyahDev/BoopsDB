CREATE DATABASE IF NOT EXISTS boopsdb;
USE boopsdb;

CREATE TABLE devices (
  id CHAR(36) PRIMARY KEY,
  username VARCHAR(255), groupname VARCHAR(255), ssh_key TEXT,
  is_virtual BOOLEAN, parent_name VARCHAR(255), os_name VARCHAR(255),
  hostname VARCHAR(255),
  interfaces JSON, ipv4_addresses JSON, default_gateways JSON, macaddrs JSON,
  ports TEXT, dns TEXT, smtp TEXT,
  arch VARCHAR(255), cpu TEXT, mem TEXT, disk TEXT,
  machine_group VARCHAR(255), usage_desc TEXT, model_info TEXT,
  memo TEXT, last_alive DATETIME,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
