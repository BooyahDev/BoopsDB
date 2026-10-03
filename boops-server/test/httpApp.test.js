import test from 'node:test';
import assert from 'node:assert/strict';
import { createApp } from '../httpApp.js';

async function serve(t, db) {
  const server = createApp(db).listen(0, '127.0.0.1');
  await new Promise(resolve => server.once('listening', resolve));
  t.after(() => new Promise(resolve => server.close(resolve)));
  return `http://127.0.0.1:${server.address().port}`;
}

test('factory handles invalid network requests without touching the database', async t => {
  const base = await serve(t, { async query() { assert.fail('unexpected DB query'); }, async getConnection() { assert.fail('invalid payload acquired DB connection'); } });
  for (const [path, body] of [
    ['/api/machines', { hostname: 'test', interfaces: { eth0: { ips: [{ ip_address: '10.0.0.2' }], gateway: '10.0.0.1' }, eth1: { ips: [{ ip_address: '10.0.1.2' }], gateway: '10.0.1.1' } } }],
    ['/api/machines/00000000-0000-4000-8000-000000000001', { hostname: 'test' }],
    ['/api/interfaces/00000000-0000-4000-8000-000000000001/eth0/update-gateway', { gateway: 'bad' }],
  ]) {
    const response = await fetch(base + path, { method: path === '/api/machines' ? 'POST' : 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    assert.equal(response.status, 400);
  }
  assert.equal((await fetch(base + '/api/machines/00000000-0000-4000-8000-000000000001', { method: 'POST' })).status, 404);
});

test('list reads IP IDs and normalized gateways with stable SQL order', async t => {
  const queries = [];
  const base = await serve(t, { async query(sql) {
    queries.push(sql);
    if (/FROM machines/.test(sql)) return [[{ id: 'a', hostname: 'same' }]];
    if (/FROM interfaces/.test(sql)) return [[{ id: 7, name: 'eth0', gateway: ' 0.0.0.0 ' }]];
    if (/FROM interface_ips/.test(sql)) return [[{ id: 11, ip_address: '10.0.0.2', subnet_mask: '' }]];
    assert.fail(sql);
  } });
  const response = await fetch(base + '/api/machines');
  assert.equal(response.status, 200);
  const [machine] = await response.json();
  assert.equal(machine.interfaces[0].gateway, '');
  assert.equal(machine.interfaces[0].ips[0].id, 11);
  assert.match(queries[0], /ORDER BY created_at IS NULL ASC, created_at ASC, id ASC/);
  assert.match(queries[1], /ORDER BY id ASC/);
  assert.match(queries[2], /SELECT id,.*ORDER BY id ASC/);
});
