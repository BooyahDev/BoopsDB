import express from 'express';
import bodyParser from 'body-parser';
import cors from 'cors';
import { v4 as uuidv4 } from 'uuid';
import db from './models/db.js';

const app = express();
app.use(cors());
app.use(bodyParser.json());

const port = 3001;

// GET all machines with interfaces
app.get('/api/machines', async (req, res) => {
  try {
    const [machines] = await db.query('SELECT * FROM machines');
    const results = [];

    for (const machine of machines) {
      const [interfaces] = await db.query(
        'SELECT name, ip_address, subnet_mask, gateway, dns_servers, mac_address FROM interfaces WHERE machine_id = ?',
        [machine.id]
      );
      results.push({ ...machine, interfaces: interfaces.reduce((acc, cur) => {
        acc[cur.name] = {
          ip: cur.ip_address,
          subnet: cur.subnet_mask,
          gateway: cur.gateway,
          dns_servers: cur.dns_servers ? cur.dns_servers.split(',').map(s => s.trim()) : [],
          mac_address: cur.mac_address || ''
        };
        return acc;
      }, {}) });
    }

    res.json(results);
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// POST new machine with interfaces
app.post('/api/machines', async (req, res) => {
  const { hostname, model_info, usage_desc, memo, last_alive, cpu_info, cpu_arch, memory_size, disk_info, os_name, is_virtual, parent_machine_id, interfaces } = req.body;

  const conn = await db.getConnection();
  try {
    await conn.beginTransaction();

    const machineId = uuidv4();

    await conn.query(
      'INSERT INTO machines (id, hostname, model_info, usage_desc, memo, last_alive, cpu_info, cpu_arch, memory_size, disk_info, os_name, is_virtual, parent_machine_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)',
      [machineId, hostname, model_info, usage_desc, memo, last_alive, cpu_info || '', cpu_arch || '', memory_size || '', disk_info || '', os_name || '', is_virtual === true, parent_machine_id || null]
    );

    for (const [name, { ip_address: ip, subnet_mask: subnet, gateway, dns_servers, mac_address }] of Object.entries(interfaces)) {
      if (!ip) {
        return res.status(400).json({ error: `IP address cannot be null for interface ${name}` });
      }
      await conn.query(
        'INSERT INTO interfaces (machine_id, name, ip_address, subnet_mask, gateway, dns_servers, mac_address) VALUES (?, ?, ?, ?, ?, ?, ?)',
        [machineId, name, ip, subnet || '', gateway || '', Array.isArray(dns_servers) ? dns_servers.join(',') : '', mac_address || '']
      );
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

// PUT update machine with interfaces
app.put('/api/machines/:id', async (req, res) => {
    const machineId = req.params.id;
    const { hostname, model_info, usage_desc, memo, last_alive, cpu_info, cpu_arch, memory_size, disk_info, os_name, is_virtual, parent_machine_id, interfaces } = req.body;

    const conn = await db.getConnection();
    try {
      await conn.beginTransaction();

      await conn.query(
        'UPDATE machines SET hostname=?, model_info=?, usage_desc=?, memo=?, last_alive=?, cpu_info=?, cpu_arch=?, memory_size=?, disk_info=?, os_name=?, is_virtual=?, parent_machine_id=? WHERE id=?',
        [hostname, model_info, usage_desc, memo, last_alive, cpu_info || '', cpu_arch || '', memory_size || '', disk_info || '', os_name || '', is_virtual === true, parent_machine_id || null, machineId]
      );

      await conn.query('DELETE FROM interfaces WHERE machine_id = ?', [machineId]);

      for (const [name, { ip_address: ip, subnet_mask: subnet, gateway, dns_servers, mac_address }] of Object.entries(interfaces)) {
        if (!ip) {
          return res.status(400).json({ error: `IP address cannot be null for interface ${name}` });
        }
        await conn.query(
          'INSERT INTO interfaces (machine_id, name, ip_address, subnet_mask, gateway, dns_servers, mac_address) VALUES (?, ?, ?, ?, ?, ?, ?)',
          [machineId, name, ip, subnet || '', gateway || '', Array.isArray(dns_servers) ? dns_servers.join(',') : '', mac_address || '']
        );
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

// DELETE machine and interfaces
app.delete('/api/machines/:id', async (req, res) => {
  try {
    await db.query('DELETE FROM machines WHERE id = ?', [req.params.id]);
    res.json({ message: 'Deleted' });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// SEARCH machines by any field
app.get('/api/machines/search', async (req, res) => {
  const query = req.query.q || '';

  if (!query.trim()) {
    return res.json([]);
  }

  try {
    let results = [];

    // First search for exact matches on machine fields
    const [machines] = await db.query(
      'SELECT * FROM machines WHERE hostname LIKE ? OR model_info LIKE ? OR usage_desc LIKE ? OR memo LIKE ? OR cpu_info LIKE ? OR cpu_arch LIKE ? OR memory_size LIKE ? OR disk_info LIKE ? OR os_name LIKE ? OR is_virtual LIKE ? OR parent_machine_id LIKE ?',
      [`%${query}%`, `%${query}%`, `%${query}%`, `%${query}%`, `%${query}%`, `%${query}%`, `%${query}%`, `%${query}%`, `%${query}%`, `%${query}%`, `%${query}%`]
    );

    if (machines.length > 0) {
      for (const machine of machines) {
        const [interfaces] = await db.query(
          'SELECT name, ip_address, subnet_mask, gateway, dns_servers, mac_address FROM interfaces WHERE machine_id = ?',
          [machine.id]
        );
        results.push({ ...machine, interfaces: interfaces.reduce((acc, cur) => {
          acc[cur.name] = {
            ip: cur.ip_address,
            subnet: cur.subnet_mask,
            gateway: cur.gateway,
            dns_servers: cur.dns_servers ? cur.dns_servers.split(',').map(s => s.trim()) : [],
            mac_address: cur.mac_address || ''
          };
          return acc;
        }, {}) });
      }
    } else {
      // If no matches, search for hostname
      const [hostnameMachines] = await db.query(
        'SELECT * FROM machines WHERE hostname LIKE ?',
        [`%${query}%`]
      );

      if (hostnameMachines.length > 0) {
        for (const machine of hostnameMachines) {
          const [interfaces] = await db.query(
            'SELECT name, ip_address, subnet_mask, gateway, dns_servers, mac_address FROM interfaces WHERE machine_id = ?',
            [machine.id]
          );
          results.push({ ...machine, interfaces: interfaces.reduce((acc, cur) => {
            acc[cur.name] = {
              ip: cur.ip_address,
              subnet: cur.subnet_mask,
              gateway: cur.gateway,
              dns_servers: cur.dns_servers ? cur.dns_servers.split(',').map(s => s.trim()) : [],
              mac_address: cur.mac_address || ''
            };
            return acc;
          }, {}) });
        }
      } else {
        // If no hostname matches, search for interfaces by IP
        const [interfaces] = await db.query(
          'SELECT * FROM interfaces WHERE ip_address LIKE ?',
          [`%${query}%`]
        );

        if (interfaces.length > 0) {
          for (const interfaceData of interfaces) {
            const machineId = interfaceData.machine_id;
            const [machine] = await db.query(
              'SELECT * FROM machines WHERE id = ?',
              [machineId]
            );
            if (machine.length > 0) {
              const [allInterfaces] = await db.query(
                'SELECT name, ip_address, subnet_mask, gateway, dns_servers, mac_address FROM interfaces WHERE machine_id = ?',
                [machineId]
              );

              results.push({ ...machine[0], interfaces: allInterfaces.reduce((acc, cur) => {
                acc[cur.name] = {
                  ip: cur.ip_address,
                  subnet: cur.subnet_mask,
                  gateway: cur.gateway,
                  dns_servers: cur.dns_servers ? cur.dns_servers.split(',').map(s => s.trim()) : [],
                  mac_address: cur.mac_address || ''
                };
                return acc;
              }, {}) });
            }
          }
        }
      }
    }

    res.json(results);
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
      'SELECT name, ip_address, subnet_mask, gateway, dns_servers, mac_address FROM interfaces WHERE machine_id = ?',
      [machine.id]
    );

    const result = {
      ...machine,
      interfaces: interfaces.reduce((acc, cur) => {
        acc[cur.name] = {
          ip: cur.ip_address,
          subnet: cur.subnet_mask,
          gateway: cur.gateway,
          dns_servers: cur.dns_servers ? cur.dns_servers.split(',').map(s => s.trim()) : [],
          mac_address: cur.mac_address || ''
        };
        return acc;
      }, {})
    };

    res.json(result);
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// PUT update machine's last_alive timestamp
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

app.listen(port, () => {
  console.log(`API server running on http://localhost:${port}`);
});
