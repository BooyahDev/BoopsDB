import test from 'node:test';
import assert from 'node:assert/strict';
import { normalizeGateway, validateInterfacePayloads, withMachineTransaction, reconcileInterfaces } from '../networkSettings.js';

const ip = { ip_address: '10.0.0.2', subnet_mask: '255.255.255.0' };
const machineId = '00000000-0000-4000-8000-000000000001';

test('legacy empty gateways normalize to an empty string', () => {
  for (const value of [null, undefined, '', '  ', ' 0.0.0.0 ']) assert.equal(normalizeGateway(value), '');
  assert.equal(normalizeGateway(' 10.0.0.1 '), '10.0.0.1');
});

test('complete payload rejects multiple gateways, malformed values and duplicate IDs', () => {
  assert.throws(() => validateInterfacePayloads({ eth0: { ips: [ip], gateway: '10.0.0.1' }, eth1: { ips: [ip], gateway: '10.0.1.1' } }), { status: 400 });
  for (const interfaces of [undefined, [], null, { eth0: { ips: [] } }, { eth0: { ips: [ip], gateway: 'invalid' } }, { eth0: { ips: [{ ...ip, id: -1 }] } }, { eth0: { ips: [{ ...ip, id: 1 }, { ...ip, id: 1 }] } }, { eth0: { ips: [{ ...ip, ip_address: '999.0.0.1' }] } }]) {
    assert.throws(() => validateInterfacePayloads(interfaces), { status: 400 });
  }
  assert.deepEqual(validateInterfacePayloads({}), []);
  assert.equal(validateInterfacePayloads({ eth0: { id: 1, ips: [ip], gateway: null } })[0].gateway, '');
});

function connectionFixture(query) {
  const events = [];
  return {
    events,
    async beginTransaction() { events.push('begin'); },
    async commit() { events.push('commit'); },
    async rollback() { events.push('rollback'); },
    release() { events.push('release'); },
    async query(sql, args) { events.push({ sql, args }); return query(sql, args); },
  };
}

test('machine transaction locks the parent before child work and releases on success', async () => {
  const conn = connectionFixture(() => [[{ id: machineId }]]);
  assert.equal(await withMachineTransaction({ async getConnection() { return conn; } }, machineId, async c => {
    await c.query('child work'); return 'saved';
  }), 'saved');
  assert.match(conn.events[1].sql, /FROM machines WHERE id = \? FOR UPDATE/);
  assert.deepEqual(conn.events.slice(-2), ['commit', 'release']);
});

test('machine transaction rolls back and releases on callback or missing-machine failure', async () => {
  for (const exists of [true, false]) {
    const conn = connectionFixture(() => [exists ? [{ id: machineId }] : []]);
    await assert.rejects(withMachineTransaction({ async getConnection() { return conn; } }, machineId, () => { throw new Error('SQL failed'); }), exists ? /SQL failed/ : { status: 404 });
    assert.deepEqual(conn.events.slice(-2), ['rollback', 'release']);
  }
});

test('reconciliation preserves IDs of identical legacy duplicate IPs without reusing one row', async () => {
  const writes = [];
  const conn = connectionFixture((sql, args) => {
    if (/SELECT .*FROM interfaces/.test(sql)) return [[{ id: 7, name: 'eth0' }]];
    if (/SELECT .*FROM interface_ips/.test(sql)) return [[{ id: 11, interface_id: 7, ...ip }, { id: 12, interface_id: 7, ...ip }]];
    writes.push({ sql, args }); return [{ affectedRows: 1 }];
  });
  await reconcileInterfaces(conn, machineId, validateInterfacePayloads({ eth0: { ips: [ip, ip], gateway: '' } }));
  assert.equal(writes.filter(w => /INSERT|DELETE/.test(w.sql)).length, 0);
  assert.deepEqual(writes.filter(w => /UPDATE interface_ips/.test(w.sql)).map(w => w.args.at(-1)), [11, 12]);
});

test('ownership checks reject unknown and foreign IP IDs before writes', async () => {
  let writes = 0;
  const conn = connectionFixture(sql => {
    if (/SELECT .*FROM interfaces/.test(sql)) return [[{ id: 7, name: 'eth0' }]];
    if (/SELECT .*FROM interface_ips/.test(sql)) return [[{ id: 11, interface_id: 7, ...ip }]];
    writes++; return [{}];
  });
  for (const interfaces of [{ eth0: { id: 99, ips: [ip] } }, { eth0: { id: 7, ips: [{ ...ip, id: 99 }] } }]) {
    await assert.rejects(reconcileInterfaces(conn, machineId, validateInterfacePayloads(interfaces)), { status: 400 });
  }
  assert.equal(writes, 0);
});

test('ambiguous legacy NIC name returns conflict before writes', async () => {
  const conn = connectionFixture(sql => {
    if (/FROM interfaces/.test(sql)) return [[{ id: 7, name: 'eth0' }, { id: 8, name: 'eth0' }]];
    if (/FROM interface_ips/.test(sql)) return [[]];
    assert.fail('ambiguous NIC must not be mutated');
  });
  await assert.rejects(reconcileInterfaces(conn, machineId, validateInterfacePayloads({ eth0: { ips: [ip] } })), { status: 409 });
});
