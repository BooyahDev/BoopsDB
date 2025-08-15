import express from 'express';
import bodyParser from 'body-parser';
import cors from 'cors';
import { v4 as uuidv4 } from 'uuid';
import db from './models/db.js';

const app = express();
app.use(cors());
app.use(bodyParser.json());

// Global error handler for uncaught exceptions
process.on('uncaughtException', (err) => {
  console.error(`Uncaught Exception: ${err.message}`);
  console.error(err.stack);
  process.exit(1); // Exit with failure code
});

// Global error handler for unhandled promise rejections
process.on('unhandledRejection', (reason, promise) => {
  console.error(`Unhandled Rejection at: ${promise}`);
  console.error(`Reason: ${reason}`);
  process.exit(1); // Exit with failure code
});

const port = 3001;

// GET all machines with interfaces
app.get('/api/machines', async (req, res) => {
  try {
    const [machines] = await db.query('SELECT * FROM machines');
    const results = [];

    for (const machine of machines) {
      const [interfaces] = await db.query(
        'SELECT id, name, gateway, dns_servers, mac_address FROM interfaces WHERE machine_id = ?',
        [machine.id]
      );

      for (const iface of interfaces) {
        const [ips] = await db.query(
          'SELECT ip_address, subnet_mask, dns_register FROM interface_ips WHERE interface_id = ?',
          [iface.id]
        );
        iface.ips = ips;
      }

      results.push({ ...machine, interfaces });
    }

    res.json(results);
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// POST new machine with interfaces
app.post('/api/machines', async (req, res) => {
  const { hostname, model_info, usage_desc, memo, purpose, last_alive, cpu_info, cpu_arch, memory_size, disk_info, os_name, is_virtual, parent_machine_id, interfaces } = req.body;

  const conn = await db.getConnection();
  try {
    await conn.beginTransaction();

    const machineId = uuidv4();

    await conn.query(
      'INSERT INTO machines (id, hostname, model_info, usage_desc, memo, purpose, last_alive, cpu_info, cpu_arch, memory_size, disk_info, os_name, is_virtual, parent_machine_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)',
      [machineId, hostname, model_info, usage_desc, memo, purpose || '', last_alive, cpu_info || '', cpu_arch || '', memory_size || '', disk_info || '', os_name || '', is_virtual === true, parent_machine_id || null]
    );

    for (const [name, { ips, gateway, dns_servers, mac_address }] of Object.entries(interfaces)) {
      await conn.query(
        'INSERT INTO interfaces (machine_id, name, gateway, dns_servers, mac_address) VALUES (?, ?, ?, ?, ?)',
        [machineId, name, gateway || '', Array.isArray(dns_servers) ? dns_servers.join(',') : '', mac_address || '']
      );

      const [interfaceResult] = await conn.query(
        'SELECT id FROM interfaces WHERE machine_id = ? AND name = ? ORDER BY id DESC LIMIT 1',
        [machineId, name]
      );
      const interfaceId = interfaceResult[0].id;

      for (const { ip_address: ip, subnet_mask: subnet, dns_register } of ips) {
        if (!ip) {
          return res.status(400).json({ error: `IP address cannot be null for interface ${name}` });
        }
        await conn.query(
          'INSERT INTO interface_ips (interface_id, ip_address, subnet_mask, dns_register) VALUES (?, ?, ?, ?)',
          [interfaceId, ip, subnet || '', !!dns_register]
        );
      }
    }

    await conn.commit();
    res.json({ message: 'Inserted', id: machineId });
  } catch (err) {
    await conn.rollback();
    res.status(500).json({ error: err.message });
  } finally {
    conn.release();
  }
});

// POST add new interface to a machine
app.post('/api/machines/:id/interfaces', async (req, res) => {
  const machineId = req.params.id;
  const { name, ips, gateway, dns_servers, mac_address } = req.body;

  // Validate UUID format for machine ID
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid machine UUID format' });
  }

  // Validate required fields
  if (!name || !ips || ips.length === 0) {
    return res.status(400).json({ error: 'Interface name and at least one IP address are required' });
  }

  try {
    // Check if machine exists
    const [machine] = await db.query('SELECT id FROM machines WHERE id = ?', [machineId]);
    if (machine.length === 0) {
      return res.status(404).json({ error: 'Machine not found' });
    }

    // Check if interface with this name already exists
    const [existingInterface] = await db.query(
      'SELECT id FROM interfaces WHERE machine_id = ? AND name = ?',
      [machineId, name]
    );
    if (existingInterface.length > 0) {
      return res.status(400).json({ error: 'Interface with this name already exists' });
    }

    // Insert new interface
    await db.query(
      'INSERT INTO interfaces (machine_id, name, gateway, dns_servers, mac_address) VALUES (?, ?, ?, ?, ?)',
      [
        machineId,
        name,
        gateway || '',
        Array.isArray(dns_servers) ? dns_servers.join(',') : '',
        mac_address || ''
      ]
    );

    const [interfaceResult] = await db.query(
      'SELECT id FROM interfaces WHERE machine_id = ? AND name = ? ORDER BY id DESC LIMIT 1',
      [machineId, name]
    );
    const interfaceId = interfaceResult[0].id;

    // Insert IP addresses
    for (const { ip_address: ip, subnet_mask: subnet, dns_register } of ips) {
      if (!ip) {
        return res.status(400).json({ error: `IP address cannot be null for interface ${name}` });
      }
      await db.query(
        'INSERT INTO interface_ips (interface_id, ip_address, subnet_mask, dns_register) VALUES (?, ?, ?, ?)',
        [interfaceId, ip, subnet || '', !!dns_register]
      );
    }

    res.json({ message: 'Interface added successfully' });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// DELETE remove an interface from a machine
app.delete('/api/machines/:machineId/interfaces/:interfaceName', async (req, res) => {
  const machineId = req.params.machineId;
  const interfaceName = req.params.interfaceName;

  // Validate UUID format for machine ID
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid machine UUID format' });
  }

  try {
    // Check if interface exists
    const [existingInterface] = await db.query(
      'SELECT id FROM interfaces WHERE machine_id = ? AND name = ?',
      [machineId, interfaceName]
    );

    if (existingInterface.length === 0) {
      return res.status(404).json({ error: 'Interface not found' });
    }

    // Delete the interface and its IP addresses
    const conn = await db.getConnection();
    await conn.beginTransaction();

    await conn.query(
      'DELETE FROM interface_ips WHERE interface_id = ?',
      [existingInterface[0].id]
    );
    await conn.query(
      'DELETE FROM interfaces WHERE machine_id = ? AND name = ?',
      [machineId, interfaceName]
    );

    await conn.commit();
    res.json({ message: 'Interface deleted successfully' });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// PUT update parent machine ID for a virtual machine
app.put('/api/machines/:id/update-parent-id', async (req, res) => {
  const machineId = req.params.id;
  const { parent_machine_id } = req.body;

  // Validate UUID format for machine ID
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid machine UUID format' });
  }

  // Validate parent machine ID format if provided
  if (parent_machine_id && !/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(parent_machine_id)) {
    return res.status(400).json({ error: 'Invalid parent machine UUID format' });
  }

  try {
    // First check if the machine exists and is virtual
    const [machines] = await db.query('SELECT is_virtual FROM machines WHERE id = ?', [machineId]);
    if (machines.length === 0) {
      return res.status(404).json({ error: 'Machine not found' });
    }

    if (!machines[0].is_virtual) {
      return res.status(400).json({ error: 'Only virtual machines can have a parent machine ID' });
    }

    // If parent ID is provided, verify it exists
    if (parent_machine_id) {
      const [parentMachines] = await db.query('SELECT id FROM machines WHERE id = ?', [parent_machine_id]);
      if (parentMachines.length === 0) {
        return res.status(400).json({ error: 'Parent machine not found' });
      }
    }

    // Update the parent machine ID
    await db.query(
      'UPDATE machines SET parent_machine_id = ? WHERE id = ?',
      [parent_machine_id || null, machineId]
    );

    res.json({ message: 'Parent machine ID updated successfully' });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// PUT update virtual machine flag and parent ID
app.put('/api/machines/:id/update-vm-status', async (req, res) => {
  const machineId = req.params.id;
  let { is_virtual, parent_machine_id } = req.body;

  // Validate UUID format for machine ID
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid machine UUID format' });
  }

  // If parent ID is empty string, set is_virtual to false
  if (parent_machine_id === '') {
    is_virtual = false;
    parent_machine_id = null;
  }

  // Validate parent machine ID format if provided
  if (parent_machine_id && !/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(parent_machine_id)) {
    return res.status(400).json({ error: 'Invalid parent machine UUID format' });
  }

  try {
    // If parent ID is provided, verify it exists
    if (is_virtual && parent_machine_id) {
      const [parentMachines] = await db.query('SELECT id FROM machines WHERE id = ?', [parent_machine_id]);
      if (parentMachines.length === 0) {
        return res.status(400).json({ error: 'Parent machine not found' });
      }
    }

    // Update the virtual machine status and parent ID
    await db.query(
      'UPDATE machines SET is_virtual = ?, parent_machine_id = ? WHERE id = ?',
      [is_virtual, is_virtual ? parent_machine_id || null : null, machineId]
    );

    res.json({
      message: 'Virtual machine status updated successfully',
      is_virtual,
      parent_machine_id: is_virtual ? parent_machine_id || null : null
    });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// PUT update machine with interfaces
app.put('/api/machines/:id', async (req, res) => {
  const machineId = req.params.id;
  const { hostname, model_info, usage_desc, memo, purpose, last_alive, cpu_info, cpu_arch, memory_size, disk_info, os_name, is_virtual, parent_machine_id, interfaces } = req.body;

  const conn = await db.getConnection();
  try {
    await conn.beginTransaction();

    await conn.query(
      'UPDATE machines SET hostname=?, model_info=?, usage_desc=?, memo=?, purpose=?, last_alive=?, cpu_info=?, cpu_arch=?, memory_size=?, disk_info=?, os_name=?, is_virtual=?, parent_machine_id=? WHERE id=?',
      [hostname, model_info, usage_desc, memo, purpose || '', last_alive, cpu_info || '', cpu_arch || '', memory_size || '', disk_info || '', os_name || '', is_virtual === true, parent_machine_id || null, machineId]
    );

    await conn.query('DELETE FROM interfaces WHERE machine_id = ?', [machineId]);
    await conn.query('DELETE FROM interface_ips WHERE interface_id IN (SELECT id FROM interfaces WHERE machine_id = ?)', [machineId]);

    for (const [name, { ips, gateway, dns_servers, mac_address }] of Object.entries(interfaces)) {
      if (!ips || ips.length === 0) {
        return res.status(400).json({ error: `At least one IP address is required for interface ${name}` });
      }

      await conn.query(
        'INSERT INTO interfaces (machine_id, name, gateway, dns_servers, mac_address) VALUES (?, ?, ?, ?, ?)',
        [machineId, name, gateway || '', Array.isArray(dns_servers) ? dns_servers.join(',') : '', mac_address || '']
      );

      const [interfaceResult] = await conn.query(
        'SELECT id FROM interfaces WHERE machine_id = ? AND name = ? ORDER BY id DESC LIMIT 1',
        [machineId, name]
      );
      const interfaceId = interfaceResult[0].id;

      for (const { ip_address: ip, subnet_mask: subnet, dns_register } of ips) {
        if (!ip) {
          return res.status(400).json({ error: `IP address cannot be null for interface ${name}` });
        }
        await conn.query(
          'INSERT INTO interface_ips (interface_id, ip_address, subnet_mask, dns_register) VALUES (?, ?, ?, ?)',
          [interfaceId, ip, subnet || '', !!dns_register]
        );
      }
    }

    await conn.commit();
    res.json({ message: 'Updated' });
  } catch (err) {
    await conn.rollback();
    res.status(500).json({ error: err.message });
  } finally {
    conn.release();
  }
});

// PUT update purpose for a specific machine
app.put('/api/machines/:id/update-purpose', async (req, res) => {
  const machineId = req.params.id;
  const { purpose } = req.body;

  // Validate UUID format for machine ID
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid machine UUID format' });
  }

  // Validate purpose
  if (typeof purpose !== 'string') {
    return res.status(400).json({ error: 'Purpose must be a string' });
  }

  try {
    await db.query(
      'UPDATE machines SET purpose = ? WHERE id = ?',
      [purpose, machineId]
    );

    // Check if any rows were affected
    const [result] = await db.query('SELECT ROW_COUNT() AS count');
    if (result[0].count > 0) {
      res.json({ message: 'Purpose updated' });
    } else {
      res.status(404).json({ error: 'Machine not found' });
    }
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// PUT update IP addresses for a specific interface
app.put('/api/interfaces/:machineId/:interfaceName/ips', async (req, res) => {
  const machineId = req.params.machineId;
  const interfaceName = req.params.interfaceName;
  const { ips } = req.body;

  // Validate UUID format for machine ID
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid machine UUID format' });
  }

  // Validate IP addresses
  if (!Array.isArray(ips) || ips.length === 0) {
    return res.status(400).json({ error: 'IP addresses must be an array with at least one valid IP address' });
  }

  try {
    const [interfaceResult] = await db.query(
      'SELECT id FROM interfaces WHERE machine_id = ? AND name = ?',
      [machineId, interfaceName]
    );

    if (interfaceResult.length === 0) {
      return res.status(404).json({ error: 'Interface not found for this machine' });
    }

    const conn = await db.getConnection();
    await conn.beginTransaction();

    // Delete existing IP addresses
    await conn.query(
      'DELETE FROM interface_ips WHERE interface_id = ?',
      [interfaceResult[0].id]
    );

    // Insert new IP addresses
    for (const { ip_address: ip, subnet_mask: subnet, dns_register } of ips) {
      if (!ip) {
        return res.status(400).json({ error: `IP address cannot be null` });
      }
      await conn.query(
        'INSERT INTO interface_ips (interface_id, ip_address, subnet_mask, dns_register) VALUES (?, ?, ?, ?)',
        [interfaceResult[0].id, ip, subnet || '', !!dns_register]
      );
    }

    await conn.commit();
    res.json({ message: 'IP addresses updated' });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// PUT update gateway for a specific interface
app.put('/api/interfaces/:machineId/:interfaceName/update-gateway', async (req, res) => {
  const machineId = req.params.machineId;
  const interfaceName = req.params.interfaceName;
  const { gateway } = req.body;

  // Validate UUID format for machine ID
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid machine UUID format' });
  }

  // Remove validation for empty gateways
  try {
    await db.query(
      'UPDATE interfaces SET gateway = ? WHERE machine_id = ? AND name = ?',
      [gateway || '', machineId, interfaceName]
    );

    // Check if any rows were affected
    const [result] = await db.query('SELECT ROW_COUNT() AS count');
    if (result[0].count > 0) {
      res.json({ message: 'Gateway updated' });
    } else {
      res.status(404).json({ error: 'Interface not found for this machine' });
    }
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// PUT update DNS servers for a specific interface
app.put('/api/interfaces/:machineId/:interfaceName/update-dns', async (req, res) => {
  const machineId = req.params.machineId;
  const interfaceName = req.params.interfaceName;
  const { dns_servers } = req.body;

  // Validate UUID format for machine ID
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid machine UUID format' });
  }

  // Remove validation for empty DNS servers
  try {
    const dnsString = Array.isArray(dns_servers) && dns_servers.length > 0 ?
                      dns_servers.join(', ') :
                      '';
    await db.query(
      'UPDATE interfaces SET dns_servers = ? WHERE machine_id = ? AND name = ?',
      [dnsString, machineId, interfaceName]
    );

    // Check if any rows were affected
    const [result] = await db.query('SELECT ROW_COUNT() AS count');
    if (result[0].count > 0) {
      res.json({ message: 'DNS servers updated' });
    } else {
      res.status(404).json({ error: 'Interface not found for this machine' });
    }
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// PUT update interface name
app.put('/api/interfaces/:machineId/:interfaceName/update-name', async (req, res) => {
  const machineId = req.params.machineId;
  const oldInterfaceName = req.params.interfaceName;
  const { name } = req.body;

  // Validate UUID format for machine ID
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid machine UUID format' });
  }

  // Validate that name is provided
  if (typeof name !== 'string' || !name.trim()) {
    return res.status(400).json({ error: 'Interface name must be a non-empty string' });
  }

  try {
    await db.query(
      'UPDATE interfaces SET name = ? WHERE machine_id = ? AND name = ?',
      [name, machineId, oldInterfaceName]
    );

    // Check if any rows were affected
    const [result] = await db.query('SELECT ROW_COUNT() AS count');
    if (result[0].count > 0) {
      res.json({ message: 'Interface name updated' });
    } else {
      res.status(404).json({ error: 'Interface not found for this machine' });
    }
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// PUT update CPU info for a specific machine
app.put('/api/machines/:id/update-cpu_info', async (req, res) => {
  const machineId = req.params.id;
  const { cpu_info } = req.body;

  // Validate UUID format for machine ID
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid machine UUID format' });
  }

  // Validate that cpu_info is provided
  if (typeof cpu_info !== 'string' || !cpu_info.trim()) {
    return res.status(400).json({ error: 'CPU info must be a non-empty string' });
  }

  try {
    await db.query('UPDATE machines SET cpu_info = ? WHERE id = ?', [cpu_info, machineId]);

    // Check if any rows were affected
    const [result] = await db.query('SELECT ROW_COUNT() AS count');
    if (result[0].count > 0) {
      res.json({ message: 'CPU info updated' });
    } else {
      res.status(404).json({ error: 'Machine not found' });
    }
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// DELETE machine and interfaces
app.delete('/api/machines/:id', async (req, res) => {
  try {
    await db.query('DELETE FROM machines WHERE id = ?', [req.params.id]);
    res.json({ message: 'Deleted' });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// SEARCH machines by any field with advanced features
app.get('/api/machines/search', async (req, res) => {
  const query = req.query.q || '';
  const sortBy = req.query.sort || 'hostname'; // hostname, last_alive, created_at, updated_at
  const sortOrder = req.query.order || 'asc'; // asc, desc
  const limit = parseInt(req.query.limit) || 100;
  const offset = parseInt(req.query.offset) || 0;

  if (!query.trim()) {
    return res.json([]);
  }

  try {
    const keywords = query.trim().split(/\s+/);
    
    // 検索に含めるキーワードと除外するキーワードに分類
    const includeKeywords = keywords.filter(k => !k.startsWith('-'));
    const excludeKeywords = keywords.filter(k => k.startsWith('-')).map(k => k.substring(1));

    let conditions = [];
    let params = [];
    let hasIpKeyword = false;
    let hasInterfaceKeyword = false;

    // 高度な検索パターンの定義
    const ipAddressRegex = /^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$/;
    const partialIpRegex = /^(?:[0-9]{1,3}\.){1,3}[0-9]*\.?$/;
    const cidrRegex = /^(?:[0-9]{1,3}\.){3}[0-9]{1,3}\/[0-9]{1,2}$/;
    const macAddressRegex = /^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$/;
    const fieldSearchRegex = /^([a-zA-Z_]+):(.+)$/;

    // 検索に含めるキーワードの処理
    if (includeKeywords.length > 0) {
      for (const keyword of includeKeywords) {
        const fieldMatch = keyword.match(fieldSearchRegex);
        
        if (fieldMatch) {
          // フィールド指定検索 (例: hostname:server01, os:ubuntu)
          const [, field, value] = fieldMatch;
          const validFields = ['hostname', 'model_info', 'usage_desc', 'memo', 'purpose', 
                             'cpu_info', 'cpu_arch', 'memory_size', 'disk_info', 'os_name'];
          
          if (validFields.includes(field)) {
            conditions.push(`m.${field} LIKE ?`);
            params.push(`%${value}%`);
          }
        } else if (cidrRegex.test(keyword)) {
          // CIDR記法での検索 (例: 192.168.1.0/24)
          const [network, prefixLength] = keyword.split('/');
          const networkParts = network.split('.').map(Number);
          const mask = (0xFFFFFFFF << (32 - parseInt(prefixLength))) >>> 0;
          
          conditions.push(`
            (INET_ATON(ip.ip_address) & ?) = (INET_ATON(?) & ?)
          `);
          params.push(mask, network, mask);
          hasIpKeyword = true;
        } else if (ipAddressRegex.test(keyword)) {
          // 完全なIPアドレス検索
          conditions.push('ip.ip_address = ?');
          params.push(keyword);
          hasIpKeyword = true;
        } else if (partialIpRegex.test(keyword)) {
          // 部分的IPアドレス検索 (例: 192.168.1)
          conditions.push('ip.ip_address LIKE ?');
          params.push(`${keyword.replace(/\.$/, '')}%`);
          hasIpKeyword = true;
        } else if (macAddressRegex.test(keyword)) {
          // MACアドレス検索
          conditions.push('i.mac_address LIKE ?');
          params.push(`%${keyword}%`);
          hasInterfaceKeyword = true;
        } else if (keyword.toLowerCase() === 'virtual' || keyword.toLowerCase() === 'vm') {
          // 仮想マシン検索
          conditions.push('m.is_virtual = TRUE');
        } else if (keyword.toLowerCase() === 'physical') {
          // 物理マシン検索
          conditions.push('m.is_virtual = FALSE');
        } else if (keyword.toLowerCase().startsWith('alive:')) {
          // 最終生存時間での検索 (例: alive:1d, alive:1h, alive:30m)
          const timeValue = keyword.substring(6);
          const timeMatch = timeValue.match(/^(\d+)([hdm])$/);
          if (timeMatch) {
            const [, amount, unit] = timeMatch;
            let sqlUnit;
            switch (unit) {
              case 'h': sqlUnit = 'HOUR'; break;
              case 'd': sqlUnit = 'DAY'; break;
              case 'm': sqlUnit = 'MINUTE'; break;
            }
            conditions.push(`m.last_alive >= DATE_SUB(NOW(), INTERVAL ${amount} ${sqlUnit})`);
          }
        } else {
          // 一般的なキーワード検索（ファジー検索機能付き）
          const fuzzyKeyword = keyword.replace(/[aeiou]/gi, '_'); // 母音をワイルドカードに
          const likeClause = `(
            m.hostname LIKE ? OR m.hostname LIKE ? OR
            m.model_info LIKE ? OR m.model_info LIKE ? OR
            m.usage_desc LIKE ? OR m.usage_desc LIKE ? OR
            m.memo LIKE ? OR m.memo LIKE ? OR
            m.purpose LIKE ? OR m.purpose LIKE ? OR
            m.cpu_info LIKE ? OR m.cpu_info LIKE ? OR
            m.cpu_arch LIKE ? OR m.cpu_arch LIKE ? OR
            m.memory_size LIKE ? OR m.memory_size LIKE ? OR
            m.disk_info LIKE ? OR m.disk_info LIKE ? OR
            m.os_name LIKE ? OR m.os_name LIKE ? OR
            i.name LIKE ? OR i.name LIKE ?
          )`;
          conditions.push(likeClause);
          const exactPattern = `%${keyword}%`;
          const fuzzyPattern = `%${fuzzyKeyword}%`;
          // 各フィールドに対して通常検索とファジー検索の両方を追加
          for (let i = 0; i < 11; i++) {
            params.push(exactPattern, fuzzyPattern);
          }
          hasInterfaceKeyword = true;
        }
      }
    }

    // 検索から除外するキーワードの処理
    if (excludeKeywords.length > 0) {
      for (const keyword of excludeKeywords) {
        const notLikeClause = `(
          m.hostname NOT LIKE ? AND m.model_info NOT LIKE ? AND 
          m.usage_desc NOT LIKE ? AND m.memo NOT LIKE ? AND 
          m.purpose NOT LIKE ? AND m.cpu_info NOT LIKE ? AND 
          m.cpu_arch NOT LIKE ? AND m.memory_size NOT LIKE ? AND 
          m.disk_info NOT LIKE ? AND m.os_name NOT LIKE ?
        )`;
        conditions.push(notLikeClause);
        const notLikeKeyword = `%${keyword}%`;
        for (let i = 0; i < 10; i++) {
          params.push(notLikeKeyword);
        }
      }
    }

    // 基本となるSQLクエリ
    let baseQuery = `
      SELECT DISTINCT m.*
      FROM machines m
    `;
    
    if (hasIpKeyword) {
      baseQuery += `
        JOIN interfaces i ON m.id = i.machine_id
        JOIN interface_ips ip ON i.id = ip.interface_id
      `;
    } else if (hasInterfaceKeyword) {
      baseQuery += `
        JOIN interfaces i ON m.id = i.machine_id
      `;
    }

    if (conditions.length > 0) {
      baseQuery += ' WHERE ' + conditions.join(' AND ');
    }

    // ソート機能の追加
    const validSortFields = ['hostname', 'last_alive', 'created_at', 'updated_at', 'os_name', 'purpose'];
    const validSortOrders = ['asc', 'desc'];
    
    if (validSortFields.includes(sortBy) && validSortOrders.includes(sortOrder.toLowerCase())) {
      baseQuery += ` ORDER BY m.${sortBy} ${sortOrder.toUpperCase()}`;
    } else {
      baseQuery += ' ORDER BY m.hostname ASC';
    }

    // ページネーション
    baseQuery += ' LIMIT ? OFFSET ?';
    params.push(limit, offset);
    
    const [machines] = await db.query(baseQuery, params);
    
    let results = [];
    
    for (const machine of machines) {
      const [interfaces] = await db.query(
        'SELECT id, name, gateway, dns_servers, mac_address FROM interfaces WHERE machine_id = ?',
        [machine.id]
      );
    
      for (const iface of interfaces) {
        const [ips] = await db.query(
          'SELECT ip_address, subnet_mask, dns_register FROM interface_ips WHERE interface_id = ?',
          [iface.id]
        );
        iface.ips = ips;
      }
    
      results.push({ ...machine, interfaces });
    }

    // 検索結果の総数を取得（ページネーション用）
    let countQuery = `
      SELECT COUNT(DISTINCT m.id) as total
      FROM machines m
    `;
    
    if (hasIpKeyword) {
      countQuery += `
        JOIN interfaces i ON m.id = i.machine_id
        JOIN interface_ips ip ON i.id = ip.interface_id
      `;
    } else if (hasInterfaceKeyword) {
      countQuery += `
        JOIN interfaces i ON m.id = i.machine_id
      `;
    }

    if (conditions.length > 0) {
      countQuery += ' WHERE ' + conditions.slice(0, -2).join(' AND '); // LIMIT, OFFSETパラメータを除く
    }

    const countParams = params.slice(0, -2); // LIMIT, OFFSETパラメータを除く
    const [countResult] = await db.query(countQuery, countParams);
    const total = countResult[0].total;

    res.json({
      results,
      pagination: {
        total,
        limit,
        offset,
        hasMore: offset + limit < total
      }
    });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// GET search suggestions and statistics
app.get('/api/search/suggestions', async (req, res) => {
  try {
    // ホスト名の候補を取得
    const [hostnames] = await db.query(
      'SELECT DISTINCT hostname FROM machines WHERE hostname IS NOT NULL AND hostname != "" ORDER BY hostname LIMIT 20'
    );

    // OS名の候補を取得
    const [osNames] = await db.query(
      'SELECT DISTINCT os_name FROM machines WHERE os_name IS NOT NULL AND os_name != "" ORDER BY os_name LIMIT 10'
    );

    // CPUアーキテクチャの候補を取得
    const [cpuArchs] = await db.query(
      'SELECT DISTINCT cpu_arch FROM machines WHERE cpu_arch IS NOT NULL AND cpu_arch != "" ORDER BY cpu_arch LIMIT 10'
    );

    // 用途の候補を取得
    const [purposes] = await db.query(
      'SELECT DISTINCT purpose FROM machines WHERE purpose IS NOT NULL AND purpose != "" ORDER BY purpose LIMIT 10'
    );

    // 統計情報を取得
    const [stats] = await db.query(`
      SELECT 
        COUNT(*) as total_machines,
        COUNT(CASE WHEN is_virtual = TRUE THEN 1 END) as virtual_machines,
        COUNT(CASE WHEN is_virtual = FALSE THEN 1 END) as physical_machines,
        COUNT(CASE WHEN last_alive >= DATE_SUB(NOW(), INTERVAL 1 DAY) THEN 1 END) as alive_last_day,
        COUNT(CASE WHEN last_alive >= DATE_SUB(NOW(), INTERVAL 1 HOUR) THEN 1 END) as alive_last_hour
      FROM machines
    `);

    res.json({
      suggestions: {
        hostnames: hostnames.map(h => h.hostname),
        osNames: osNames.map(o => o.os_name),
        cpuArchs: cpuArchs.map(c => c.cpu_arch),
        purposes: purposes.map(p => p.purpose)
      },
      statistics: stats[0],
      searchHelp: {
        fieldSearch: 'hostname:server01, os:ubuntu, purpose:web',
        ipSearch: '192.168.1.100, 192.168.1, 192.168.1.0/24',
        macSearch: '00:1A:2B:3C:4D:5E',
        typeSearch: 'virtual, physical',
        timeSearch: 'alive:1h, alive:1d, alive:30m',
        exclusion: '-test, -old'
      }
    });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// GET recent search queries (for search history)
app.get('/api/search/history', async (req, res) => {
  try {
    // In a production environment, you might want to store this in a separate table
    // For now, we'll return some common useful searches
    const commonSearches = [
      'virtual',
      'physical', 
      'ubuntu',
      'windows',
      'alive:1d',
      'alive:1h',
      'purpose:web',
      'purpose:database'
    ];

    res.json({
      commonSearches,
      recentSearches: [] // Would be populated from a search history table
    });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// GET advanced search filters
app.get('/api/search/filters', async (req, res) => {
  try {
    // Get unique values for various fields to help with filtering
    const [osNames] = await db.query(
      'SELECT DISTINCT os_name, COUNT(*) as count FROM machines WHERE os_name IS NOT NULL AND os_name != "" GROUP BY os_name ORDER BY count DESC, os_name'
    );

    const [purposes] = await db.query(
      'SELECT DISTINCT purpose, COUNT(*) as count FROM machines WHERE purpose IS NOT NULL AND purpose != "" GROUP BY purpose ORDER BY count DESC, purpose'
    );

    const [cpuArchs] = await db.query(
      'SELECT DISTINCT cpu_arch, COUNT(*) as count FROM machines WHERE cpu_arch IS NOT NULL AND cpu_arch != "" GROUP BY cpu_arch ORDER BY count DESC, cpu_arch'
    );

    const [memSizes] = await db.query(
      'SELECT DISTINCT memory_size, COUNT(*) as count FROM machines WHERE memory_size IS NOT NULL AND memory_size != "" GROUP BY memory_size ORDER BY count DESC, memory_size LIMIT 20'
    );

    // Get IP network ranges
    const [ipRanges] = await db.query(`
      SELECT 
        SUBSTRING_INDEX(ip_address, '.', 3) as network_prefix,
        COUNT(*) as count
      FROM interface_ips 
      WHERE ip_address LIKE '%.%.%.%'
      GROUP BY network_prefix 
      HAVING count >= 2
      ORDER BY count DESC, network_prefix
      LIMIT 20
    `);

    res.json({
      filters: {
        osNames: osNames.map(row => ({ value: row.os_name, count: row.count })),
        purposes: purposes.map(row => ({ value: row.purpose, count: row.count })),
        cpuArchs: cpuArchs.map(row => ({ value: row.cpu_arch, count: row.count })),
        memSizes: memSizes.map(row => ({ value: row.memory_size, count: row.count })),
        networks: ipRanges.map(row => ({ value: row.network_prefix + '.0/24', count: row.count }))
      },
      machineTypes: [
        { value: 'virtual', label: '仮想マシン' },
        { value: 'physical', label: '物理マシン' }
      ],
      timeRanges: [
        { value: 'alive:1h', label: '1時間以内' },
        { value: 'alive:6h', label: '6時間以内' },
        { value: 'alive:1d', label: '1日以内' },
        { value: 'alive:7d', label: '1週間以内' },
        { value: 'alive:30d', label: '1ヶ月以内' }
      ]
    });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// GET machine by UUID
app.get('/api/machines/:uuid', async (req, res) => {
  try {
    const uuid = req.params.uuid;

    // Validate UUID format
    if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(uuid)) {
      return res.status(400).json({ error: 'Invalid UUID format' });
    }

    // Get machine by ID
    const [machines] = await db.query('SELECT * FROM machines WHERE id = ?', [uuid]);

    if (machines.length === 0) {
      return res.status(404).json({ error: 'Machine not found' });
    }

    const machine = machines[0];

    // Get interfaces for the machine
    const [interfaces] = await db.query(
      'SELECT id, name, gateway, dns_servers, mac_address FROM interfaces WHERE machine_id = ?',
      [machine.id]
    );

    for (const iface of interfaces) {
      const [ips] = await db.query(
        'SELECT ip_address, subnet_mask, dns_register FROM interface_ips WHERE interface_id = ?',
        [iface.id]
      );
      iface.ips = ips;
    }

    const result = { ...machine, interfaces };

    res.json(result);
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// PUT update machine's last_alive timestamp
app.put('/api/machines/:id/update-hostname', async (req, res) => {
  const machineId = req.params.id;
  const { hostname } = req.body;

  // Validate UUID format
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid UUID format' });
  }

  // Validate that hostname is provided
  if (typeof hostname !== 'string' || !hostname.trim()) {
    return res.status(400).json({ error: 'Hostname must be a non-empty string' });
  }

  try {
    await db.query('UPDATE machines SET hostname = ? WHERE id = ?', [hostname, machineId]);

    // Check if any rows were affected
    const [result] = await db.query('SELECT ROW_COUNT() AS count');
    if (result[0].count > 0) {
      res.json({ message: 'Hostname updated' });
    } else {
      res.status(404).json({ error: 'Machine not found' });
    }
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

app.put('/api/machines/:id/update-memo', async (req, res) => {
  const machineId = req.params.id;
  const { memo } = req.body;

  // Validate UUID format
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid UUID format' });
  }

  // Validate that memo is provided
  if (typeof memo !== 'string' || !memo.trim()) {
    return res.status(400).json({ error: 'Memo must be a non-empty string' });
  }

  try {
    await db.query('UPDATE machines SET memo = ? WHERE id = ?', [memo, machineId]);

    // Check if any rows were affected
    const [result] = await db.query('SELECT ROW_COUNT() AS count');
    if (result[0].count > 0) {
      res.json({ message: 'Memo updated' });
    } else {
      res.status(404).json({ error: 'Machine not found' });
    }
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

app.put('/api/machines/:id/update-last-alive', async (req, res) => {
  const machineId = req.params.id;

  // Validate UUID format
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid UUID format' });
  }

  // Format the current time in MySQL DATETIME format (YYYY-MM-DD HH:MM:SS)
  const currentTime = new Date().toISOString().slice(0, 19).replace('T', ' ');
  try {
    await db.query('UPDATE machines SET last_alive = ? WHERE id = ?', [currentTime, machineId]);

    // Check if any rows were affected
    const [result] = await db.query('SELECT ROW_COUNT() AS count');
    if (result[0].count > 0) {
      res.json({ message: 'Last alive timestamp updated' });
    } else {
      res.status(404).json({ error: 'Machine not found' });
    }
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// PUT update memory size for a specific machine
app.put('/api/machines/:id/update-memory_size', async (req, res) => {
  const machineId = req.params.id;
  const { memory_size } = req.body;

  // Validate UUID format for machine ID
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid machine UUID format' });
  }

  // Validate that memory_size is provided
  if (typeof memory_size !== 'string' || !memory_size.trim()) {
    return res.status(400).json({ error: 'Memory size must be a non-empty string' });
  }

  try {
    await db.query('UPDATE machines SET memory_size = ? WHERE id = ?', [memory_size, machineId]);

    // Check if any rows were affected
    const [result] = await db.query('SELECT ROW_COUNT() AS count');
    if (result[0].count > 0) {
      res.json({ message: 'Memory size updated' });
    } else {
      res.status(404).json({ error: 'Machine not found' });
    }
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// PUT update CPU architecture for a specific machine
app.put('/api/machines/:id/update-cpu_arch', async (req, res) => {
  const machineId = req.params.id;
  const { cpu_arch } = req.body;

  // Validate UUID format for machine ID
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid machine UUID format' });
  }

  // Validate that cpu_arch is provided
  if (typeof cpu_arch !== 'string' || !cpu_arch.trim()) {
    return res.status(400).json({ error: 'CPU architecture must be a non-empty string' });
  }

  try {
    await db.query('UPDATE machines SET cpu_arch = ? WHERE id = ?', [cpu_arch, machineId]);

    // Check if any rows were affected
    const [result] = await db.query('SELECT ROW_COUNT() AS count');
    if (result[0].count > 0) {
      res.json({ message: 'CPU architecture updated' });
    } else {
      res.status(404).json({ error: 'Machine not found' });
    }
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// PUT update disk info for a specific machine
app.put('/api/machines/:id/update-disk_info', async (req, res) => {
  const machineId = req.params.id;
  const { disk_info } = req.body;

  // Validate UUID format for machine ID
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid machine UUID format' });
  }

  // Validate that disk_info is provided
  if (typeof disk_info !== 'string' || !disk_info.trim()) {
    return res.status(400).json({ error: 'Disk info must be a non-empty string' });
  }

  try {
    await db.query('UPDATE machines SET disk_info = ? WHERE id = ?', [disk_info, machineId]);

    // Check if any rows were affected
    const [result] = await db.query('SELECT ROW_COUNT() AS count');
    if (result[0].count > 0) {
      res.json({ message: 'Disk info updated' });
    } else {
      res.status(404).json({ error: 'Machine not found' });
    }
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// PUT update OS name for a specific machine
app.put('/api/machines/:id/update-os_name', async (req, res) => {
  const machineId = req.params.id;
  const { os_name } = req.body;

  // Validate UUID format for machine ID
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid machine UUID format' });
  }

  // Validate that os_name is provided
  if (typeof os_name !== 'string' || !os_name.trim()) {
    return res.status(400).json({ error: 'OS name must be a non-empty string' });
  }

  try {
    await db.query('UPDATE machines SET os_name = ? WHERE id = ?', [os_name, machineId]);

    // Check if any rows were affected
    const [result] = await db.query('SELECT ROW_COUNT() AS count');
    if (result[0].count > 0) {
      res.json({ message: 'OS name updated' });
    } else {
      res.status(404).json({ error: 'Machine not found' });
    }
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// PUT update MAC address for a specific interface
app.put('/api/machines/:machineId/interfaces/:interfaceName/update-mac_address', async (req, res) => {
  const machineId = req.params.machineId;
  const interfaceName = req.params.interfaceName;
  const { mac_address } = req.body;

  // Validate UUID format for machine ID
  if (!/^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$/.test(machineId)) {
    return res.status(400).json({ error: 'Invalid machine UUID format' });
  }

  // Validate MAC address
  if (typeof mac_address !== 'string' || !mac_address.trim()) {
    return res.status(400).json({ error: 'MAC address must be a non-empty string' });
  }

  try {
    await db.query(
      'UPDATE interfaces SET mac_address = ? WHERE machine_id = ? AND name = ?',
      [mac_address, machineId, interfaceName]
    );

    // Check if any rows were affected
    const [result] = await db.query('SELECT ROW_COUNT() AS count');
    if (result[0].count > 0) {
      res.json({ message: 'MAC address updated' });
    } else {
      res.status(404).json({ error: 'Interface not found for this machine' });
    }
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// GET IP addresses with dns_register flag set to ON, grouped by hostname
app.get('/api/dns-register', async (req, res) => {
  try {
    const query = `
      SELECT m.hostname, GROUP_CONCAT(ip.ip_address ORDER BY ip.ip_address SEPARATOR ', ') AS ips
      FROM machines m
      JOIN interfaces i ON m.id = i.machine_id
      JOIN interface_ips ip ON i.id = ip.interface_id AND ip.dns_register = TRUE
      GROUP BY m.hostname
    `;

    const [results] = await db.query(query);

    // Format the response as an array of objects with hostname and ips arrays
    const formattedResults = results.map(row => ({
      hostname: row.hostname,
      ips: row.ips.split(', ').filter(ip => ip !== '')
    }));

    res.json(formattedResults);
  } catch (err) {
    console.error('Error fetching DNS registered IPs:', err.message);
    res.status(500).json({ error: 'Internal server error' });
  }
});

// Wrap app.listen in a try/catch to handle any startup errors
try {
  app.listen(port, '0.0.0.0', () => {
    console.log(`API server running on http://0.0.0.0:${port}`);
  });
} catch (err) {
  console.error('Failed to start server:', err);
}
