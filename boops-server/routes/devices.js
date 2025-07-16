import express from 'express';
import db from '../models/db.js';
import { v4 as uuidv4 } from 'uuid';

const router = express.Router();

// 登録
router.post('/register', async (req, res) => {
  const id = uuidv4();
  const d = req.body;
  try {
    await db.execute(`
      INSERT INTO devices
        (id, username, groupname, ssh_key, is_virtual, parent_name, os_name, hostname,
         interfaces, ipv4_addresses, default_gateways, macaddrs,
         ports, dns, smtp, arch, cpu, mem, disk,
         machine_group, usage, model_info, memo, last_alive)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      [
        id,
        d.username, d.groupname, d.ssh_key, d.is_virtual, d.parent_name, d.os_name, d.hostname,
        JSON.stringify(d.interfaces), JSON.stringify(d.ipv4_addresses),
        JSON.stringify(d.default_gateways), JSON.stringify(d.macaddrs),
        d.ports, d.dns, d.smtp, d.arch, d.cpu, d.mem, d.disk,
        d.machine_group, d.usage, d.model_info, d.memo, d.last_alive
      ]
    );
    res.json({ success: true, id });
  } catch (e) {
    res.status(500).json({ error: e.message });
  }
});

// 一覧取得
router.get('/', async (req, res) => {
  const [rows] = await db.query('SELECT * FROM devices ORDER BY created_at DESC');
  res.json(rows);
});

// 単体取得
router.get('/:id', async (req, res) => {
  const [rows] = await db.query('SELECT * FROM devices WHERE id = ?', [req.params.id]);
  if (!rows.length) return res.status(404).json({ error: 'Not found' });
  res.json(rows[0]);
});

// 更新
router.put('/:id', async (req, res) => {
  const d = req.body;
  const updates = [
    d.username, d.groupname, d.ssh_key, d.is_virtual, d.parent_name,
    d.os_name, d.hostname,
    JSON.stringify(d.interfaces), JSON.stringify(d.ipv4_addresses),
    JSON.stringify(d.default_gateways), JSON.stringify(d.macaddrs),
    d.ports, d.dns, d.smtp, d.arch, d.cpu, d.mem, d.disk,
    d.machine_group, d.usage, d.model_info, d.memo, d.last_alive,
    req.params.id
  ];
  try {
    await db.execute(`
      UPDATE devices SET
        username=?, groupname=?, ssh_key=?, is_virtual=?, parent_name=?,
        os_name=?, hostname=?, interfaces=?, ipv4_addresses=?, default_gateways=?, macaddrs=?,
        ports=?, dns=?, smtp=?, arch=?, cpu=?, mem=?, disk=?,
        machine_group=?, usage=?, model_info=?, memo=?, last_alive=?
      WHERE id = ?`, updates);
    res.json({ success: true });
  } catch (e) {
    res.status(500).json({ error: e.message });
  }
});

// 削除
router.delete('/:id', async (req, res) => {
  try {
    await db.execute('DELETE FROM devices WHERE id = ?', [req.params.id]);
    res.json({ success: true });
  } catch (e) {
    res.status(500).json({ error: e.message });
  }
});

export default router;

