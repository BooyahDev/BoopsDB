import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import mysql from 'mysql2/promise';
import { createApp } from '../httpApp.js';

// No production configuration is imported. Destructive fixture setup is allowed only on an explicit loopback test database.
const enabled = Boolean(process.env.TEST_DB_NAME);
test('network HTTP contracts against isolated MySQL', { skip: !enabled && 'set TEST_DB_NAME to a dedicated boops_test_* database' }, async t => {
  assert.match(process.env.TEST_DB_NAME, /^boops_test_[a-zA-Z0-9_]+$/);
  const host = process.env.TEST_DB_HOST || '127.0.0.1';
  assert.ok(['127.0.0.1', 'localhost', '::1'].includes(host), 'test DB must be loopback');
  const db = mysql.createPool({ host, port: Number(process.env.TEST_DB_PORT || 33306), user: process.env.TEST_DB_USER || 'root', password: process.env.TEST_DB_PASSWORD || '', database: process.env.TEST_DB_NAME, multipleStatements: true, connectionLimit: 5 });
  t.after(() => db.end());
  await db.query('DROP TABLE IF EXISTS interface_ips; DROP TABLE IF EXISTS interfaces; DROP TABLE IF EXISTS machines');
  const schema = (await fs.readFile(new URL('../sql/schema.sql', import.meta.url), 'utf8')).replace(/CREATE DATABASE[^;]*;\s*USE[^;]*;/, '');
  await db.query(schema);
  const transactions = [];
  const appDatabase = {
    query: db.query.bind(db),
    async getConnection() {
      const connection = await db.getConnection();
      const trace = { queries: [], releases: 0 };
      transactions.push(trace);
      return {
        beginTransaction: () => connection.beginTransaction(),
        commit: () => connection.commit(),
        rollback: () => connection.rollback(),
        query(sql, args) { trace.queries.push(sql); return connection.query(sql, args); },
        release() { trace.releases++; connection.release(); },
      };
    },
  };
  const server = createApp(appDatabase).listen(0, '127.0.0.1');
  await new Promise(resolve => server.once('listening', resolve));
  t.after(() => new Promise(resolve => server.close(resolve)));
  const base = `http://127.0.0.1:${server.address().port}`;
  async function request(method, path, body) {
    const response = await fetch(base + path, { method, headers: { 'Content-Type': 'application/json' }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) });
    return { status: response.status, body: await response.json() };
  }
  const ip = { ip_address: '10.0.0.2', subnet_mask: '255.255.255.0', dns_register: false };
  const payload = { hostname: 'same', interfaces: { eth0: { ips: [ip, ip], gateway: null }, eth1: { ips: [{ ...ip, ip_address: '10.0.1.2' }], gateway: '' } } };
  const created = await request('POST', '/api/machines', payload);
  assert.equal(created.status, 200);
  assert.equal(created.body.message, 'Inserted');
  const id = created.body.id;
  const detail = () => request('GET', `/api/machines/${id}`);
  const before = (await detail()).body;
  assert.equal(before.interfaces[0].gateway, '');
  assert.equal(before.interfaces[0].ips.length, 2);
  assert.ok(before.interfaces[0].ips.every(row => Number.isInteger(row.id)));

  await t.test('legacy PUT and duplicate IP occurrence preserve every existing ID', async () => {
    assert.equal((await request('PUT', `/api/machines/${id}`, payload)).status, 200);
    const after = (await detail()).body;
    assert.deepEqual(after.interfaces.map(row => row.id), before.interfaces.map(row => row.id));
    assert.deepEqual(after.interfaces[0].ips.map(row => row.id), before.interfaces[0].ips.map(row => row.id));
  });
  await t.test('explicit IDs support rename, edit, deletion and append without reordering old rows', async () => {
    const old = (await detail()).body;
    const edited = { hostname: 'same', interfaces: {
      eth1: { id: old.interfaces[1].id, ips: old.interfaces[1].ips },
      renamed: { id: old.interfaces[0].id, ips: [{ ...old.interfaces[0].ips[1], ip_address: '10.0.0.9' }, { ...ip, ip_address: '10.0.0.10' }] },
    } };
    assert.equal((await request('PUT', `/api/machines/${id}`, edited)).status, 200);
    const after = (await detail()).body;
    assert.deepEqual(after.interfaces.map(row => row.id), old.interfaces.map(row => row.id));
    assert.equal(after.interfaces[0].name, 'renamed');
    assert.equal(after.interfaces[0].ips[0].id, old.interfaces[0].ips[1].id);
    assert.ok(after.interfaces[0].ips[1].id > old.interfaces[0].ips[1].id);
    assert.equal((await request('PUT', `/api/interfaces/${id}/renamed/update-name`, { name: 'eth0' })).status, 200);
  });
  await t.test('bad payload and unknown IDs leave machine and all children unchanged', async () => {
    const old = (await detail()).body;
    for (const interfaces of [undefined, { eth0: { ips: [] } }, { eth0: { ips: [ip], gateway: '10.0.0.1' }, eth1: { ips: [ip], gateway: '10.0.1.1' } }, { eth0: { id: 999999, ips: [ip] } }, { eth0: { id: old.interfaces[0].id, ips: [{ ...ip, id: old.interfaces[1].ips[0].id }] } }]) {
      assert.equal((await request('PUT', `/api/machines/${id}`, { hostname: 'must-rollback', interfaces })).status, 400);
      assert.deepEqual((await detail()).body, old);
    }
  });
  await t.test('gateway clearing and same-value updates succeed; parallel switches leave one gateway', async () => {
    const path = name => `/api/interfaces/${id}/${name}/update-gateway`;
    assert.equal((await request('PUT', path('eth0'), { gateway: null })).status, 200);
    assert.equal((await request('PUT', path('eth0'), { gateway: '' })).status, 200);
    assert.equal((await request('PUT', path('eth0'), { gateway: '10.0.0.1' })).body.message, 'Gateway updated');
    assert.equal((await request('PUT', path('eth0'), { gateway: '10.0.0.1' })).status, 200);
    const concurrent = await Promise.all([request('PUT', path('eth0'), { gateway: '10.0.0.1' }), request('PUT', path('eth1'), { gateway: '10.0.1.1' })]);
    assert.ok(concurrent.every(r => r.status === 200));
    assert.equal((await detail()).body.interfaces.filter(row => row.gateway !== '').length, 1);
    assert.equal((await request('PUT', path('missing'), { gateway: '' })).status, 404);
  });
  await t.test('ambiguous names reject every name-key mutation; explicit IDs resolve duplicates', async () => {
    const [duplicate] = await db.query('INSERT INTO interfaces (machine_id, name, gateway) VALUES (?, ?, ?)', [id, 'eth0', '']);
    await db.query('INSERT INTO interface_ips (interface_id, ip_address, subnet_mask) VALUES (?, ?, ?)', [duplicate.insertId, '10.0.3.2', '']);
    for (const [method, path, body] of [
      ['PUT', `/api/interfaces/${id}/eth0/update-gateway`, { gateway: '' }],
      ['PUT', `/api/interfaces/${id}/eth0/update-name`, { name: 'fixed' }],
      ['PUT', `/api/interfaces/${id}/eth0/update-dns`, { dns_servers: [] }],
      ['PUT', `/api/machines/${id}/interfaces/eth0/update-mac_address`, { mac_address: '00:11:22:33:44:55' }],
      ['PUT', `/api/interfaces/${id}/eth0/ips`, { ips: [ip] }],
      ['DELETE', `/api/machines/${id}/interfaces/eth0`],
      ['PUT', `/api/machines/${id}`, payload],
    ]) assert.equal((await request(method, path, body)).status, 409);
    const current = (await detail()).body;
    const interfaces = Object.fromEntries(current.interfaces.map((row, index) => [`nic${index}`, { ...row, dns_servers: row.dns_servers ? row.dns_servers.split(',').map(value => value.trim()) : [], gateway: '' }]));
    assert.equal((await request('PUT', `/api/machines/${id}`, { hostname: 'same', interfaces })).status, 200);
  });
  await t.test('all-list and search retain registration order, tie IDs, and NULL dates last', async () => {
    const ids = ['00000000-0000-4000-8000-000000000003', '00000000-0000-4000-8000-000000000002', '00000000-0000-4000-8000-000000000001'];
    for (const machineId of ids) await db.query('INSERT INTO machines (id, hostname, created_at) VALUES (?, ?, ?)', [machineId, 'same', machineId.endsWith('1') ? null : '2020-01-01 00:00:00']);
    const expected = [ids[1], ids[0], id, ids[2]];
    assert.deepEqual((await request('GET', '/api/machines')).body.map(row => row.id), expected);
    assert.deepEqual((await request('GET', '/api/machines/search?q=hostname:same')).body.results.map(row => row.id), expected);
    const sorted = (await request('GET', '/api/machines/search?q=hostname:same&sort=hostname&order=desc&limit=2')).body;
    assert.deepEqual(sorted.results.map(row => row.id), [...ids, id].sort().slice(0, 2));
    assert.equal(sorted.pagination.hasMore, true);
  });
  await t.test('empty full replacement removes children and delete remains compatible', async () => {
    assert.equal((await request('PUT', `/api/machines/${id}`, { hostname: 'same', interfaces: {} })).status, 200);
    assert.deepEqual((await detail()).body.interfaces, []);
    assert.deepEqual((await request('DELETE', `/api/machines/${id}`)).body, { message: 'Deleted' });
  });
  await t.test('SQL insert failure rolls back whole machine, dedicated IP, NIC addition and machine creation', async () => {
    const machine = await request('POST', '/api/machines', { hostname: 'rollback-fixture', interfaces: { eth0: { ips: [ip], gateway: '10.0.0.1' } } });
    assert.equal(machine.status, 200);
    const fixtureId = machine.body.id;
    const old = (await request('GET', `/api/machines/${fixtureId}`)).body;
    const [[{ total }]] = await db.query('SELECT COUNT(*) AS total FROM machines');
    await db.query("CREATE TRIGGER boops_test_reject_ip BEFORE INSERT ON interface_ips FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'isolated test IP insert failure'");
    try {
      const changedIp = { ...ip, ip_address: '10.0.0.88' };
      for (const [path, body] of [
        [`/api/machines/${fixtureId}`, { hostname: 'must-rollback', interfaces: { renamed: { id: old.interfaces[0].id, ips: [changedIp], gateway: '' } } }],
        [`/api/interfaces/${fixtureId}/eth0/ips`, { ips: [changedIp] }],
      ]) {
        assert.equal((await request('PUT', path, body)).status, 500);
        assert.deepEqual((await request('GET', `/api/machines/${fixtureId}`)).body, old);
      }
      assert.equal((await request('POST', `/api/machines/${fixtureId}/interfaces`, { name: 'eth1', ips: [changedIp], gateway: '10.0.1.1' })).status, 500);
      assert.deepEqual((await request('GET', `/api/machines/${fixtureId}`)).body, old);
      assert.equal((await request('POST', '/api/machines', { hostname: 'must-not-exist', interfaces: { eth0: { ips: [changedIp] } } })).status, 500);
      const [[after]] = await db.query('SELECT COUNT(*) AS total FROM machines');
      assert.equal(after.total, total);
    } finally {
      await db.query('DROP TRIGGER boops_test_reject_ip');
    }
  });
  await t.test('NIC addition chooses gateway atomically and all dedicated edits retain IDs and order', async () => {
    const machine = await request('POST', '/api/machines', { hostname: 'child-fixture', interfaces: { eth0: { ips: [ip], gateway: '10.0.0.1' } } });
    assert.equal(machine.status, 200);
    const fixtureId = machine.body.id;
    const old = (await request('GET', `/api/machines/${fixtureId}`)).body;
    assert.equal((await request('POST', `/api/machines/${fixtureId}/interfaces`, { name: 'eth1', ips: [{ ...ip, ip_address: '10.0.1.2' }], gateway: '10.0.1.1' })).status, 200);
    let current = (await request('GET', `/api/machines/${fixtureId}`)).body;
    assert.equal(current.interfaces[0].gateway, '');
    assert.equal(current.interfaces[1].gateway, '10.0.1.1');
    const ipId = old.interfaces[0].ips[0].id;
    assert.equal((await request('PUT', `/api/interfaces/${fixtureId}/eth0/ips`, { ips: [{ ...ip, id: ipId, ip_address: '10.0.0.9' }, { ...ip, ip_address: '10.0.0.10' }] })).status, 200);
    assert.equal((await request('PUT', `/api/interfaces/${fixtureId}/eth0/update-dns`, { dns_servers: ['1.1.1.1', '8.8.8.8'] })).status, 200);
    assert.equal((await request('PUT', `/api/machines/${fixtureId}/interfaces/eth0/update-mac_address`, { mac_address: '00:11:22:33:44:55' })).status, 200);
    assert.equal((await request('PUT', `/api/interfaces/${fixtureId}/eth0/update-name`, { name: 'renamed' })).status, 200);
    current = (await request('GET', `/api/machines/${fixtureId}`)).body;
    assert.equal(current.interfaces[0].id, old.interfaces[0].id);
    assert.equal(current.interfaces[0].ips[0].id, ipId);
    assert.equal(current.interfaces[0].dns_servers, '1.1.1.1, 8.8.8.8');
    assert.equal(current.interfaces[0].mac_address, '00:11:22:33:44:55');
    assert.equal((await request('DELETE', `/api/machines/${fixtureId}/interfaces/renamed`)).status, 200);
    const [[removed]] = await db.query('SELECT COUNT(*) AS total FROM interface_ips WHERE interface_id = ?', [old.interfaces[0].id]);
    assert.equal(removed.total, 0);
    assert.equal((await request('DELETE', `/api/machines/${fixtureId}`)).status, 200);
    assert.equal((await request('PUT', `/api/interfaces/${fixtureId}/eth1/update-gateway`, { gateway: '' })).status, 404);
  });
  await t.test('every network mutation locks its machine before child SQL and releases its connection', () => {
    assert.ok(transactions.length > 20, 'all mutation paths must have exercised transactions');
    for (const trace of transactions) {
      assert.equal(trace.releases, 1);
      const lockIndex = trace.queries.findIndex(sql => /FROM machines WHERE id = \? FOR UPDATE/.test(sql));
      const childIndex = trace.queries.findIndex(sql => /(?:FROM|INTO|UPDATE|DELETE FROM) (?:interfaces|interface_ips)\b/.test(sql));
      if (childIndex !== -1) assert.ok(lockIndex !== -1 && lockIndex < childIndex, trace.queries.join('\n'));
    }
  });
});
