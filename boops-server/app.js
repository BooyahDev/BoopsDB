import express from 'express';
import bodyParser from 'body-parser';
import cors from 'cors';
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
        'SELECT name, ip_address, subnet_mask, gateway FROM interfaces WHERE machine_id = ?',
        [machine.id]
      );
      results.push({ ...machine, interfaces: interfaces.reduce((acc, cur) => {
        acc[cur.name] = {
          ip: cur.ip_address,
          subnet: cur.subnet_mask,
          gateway: cur.gateway
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
  const { hostname, model_info, usage_desc, memo, last_alive, interfaces } = req.body;

  const conn = await db.getConnection();
  try {
    await conn.beginTransaction();

    const [result] = await conn.query(
      'INSERT INTO machines (hostname, model_info, usage_desc, memo, last_alive) VALUES (?, ?, ?, ?, ?)',
      [hostname, model_info, usage_desc, memo, last_alive]
    );
    const machineId = result.insertId;

    for (const [name, { ip_address: ip, subnet_mask: subnet, gateway }] of Object.entries(interfaces)) {
      if (!ip) {
        return res.status(400).json({ error: `IP address cannot be null for interface ${name}` });
      }
      await conn.query(
        'INSERT INTO interfaces (machine_id, name, ip_address, subnet_mask, gateway) VALUES (?, ?, ?, ?, ?)',
        [machineId, name, ip, subnet || '', gateway || '']
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
  const { hostname, model_info, usage_desc, memo, last_alive, interfaces } = req.body;

  const conn = await db.getConnection();
  try {
    await conn.beginTransaction();

    await conn.query(
      'UPDATE machines SET hostname=?, model_info=?, usage_desc=?, memo=?, last_alive=? WHERE id=?',
      [hostname, model_info, usage_desc, memo, last_alive, machineId]
    );

    await conn.query('DELETE FROM interfaces WHERE machine_id = ?', [machineId]);

    for (const [name, { ip, subnet, gateway }] of Object.entries(interfaces)) {
      await conn.query(
        'INSERT INTO interfaces (machine_id, name, ip_address, subnet_mask, gateway) VALUES (?, ?, ?, ?, ?)',
        [machineId, name, ip, subnet, gateway]
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

app.listen(port, () => {
  console.log(`API server running on http://localhost:${port}`);
});
