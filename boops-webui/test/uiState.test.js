import test from 'node:test';
import assert from 'node:assert/strict';
import { normalizeThemePreference, resolveTheme } from '../utils/themePreference.js';
import { createIpEditRows, toIpPayload } from '../utils/interfaceRows.js';
import { createMachineSearchParams } from '../utils/machineSearchParams.js';
import { createMachineLoader, refreshUpdatedMachine } from '../utils/machineLoader.js';
import { syncInterfaceDraft } from '../utils/interfaceDraft.js';

test('system follows OS while explicit theme ignores OS', () => {
  assert.equal(resolveTheme('system', true), 'dark');
  assert.equal(resolveTheme('system', false), 'light');
  assert.equal(resolveTheme('light', true), 'light');
  assert.equal(resolveTheme('dark', false), 'dark');
});

test('invalid or absent cookie resolves to system', () => {
  for (const value of [undefined, null, '', 'unknown', 'Dark']) {
    assert.equal(normalizeThemePreference(value), 'system');
    assert.equal(resolveTheme(value, true), 'dark');
  }
});

test('saved IP keeps database ID and edit does not mutate source', () => {
  const source = [{ id: 42, ip_address: '10.0.0.2', subnet_mask: '255.255.255.0', dns_register: 1 }];
  const rows = createIpEditRows(source, () => 'unused');
  rows[0].ip_address = '10.0.0.3';
  assert.equal(source[0].ip_address, '10.0.0.2');
  assert.equal(rows[0].rowId, 'saved-42');
  assert.deepEqual(toIpPayload(rows), [{ id: 42, ip_address: '10.0.0.3', subnet_mask: '255.255.255.0', dns_register: true }]);
});

test('draft row IDs survive removal and are never sent to API', () => {
  let sequence = 0;
  const nextId = () => `draft-${++sequence}`;
  const rows = createIpEditRows([{ ip_address: '10.0.0.2' }, { ip_address: '10.0.0.3' }], nextId);
  const secondKey = rows[1].rowId;
  rows.splice(0, 1);
  rows.push(...createIpEditRows([{ ip_address: '10.0.0.4' }], nextId));
  assert.equal(rows[0].rowId, secondKey);
  assert.notEqual(rows[0].rowId, rows[1].rowId);
  assert.equal('id' in toIpPayload(rows)[0], false);
  assert.equal('rowId' in toIpPayload(rows)[0], false);
  assert.equal(createIpEditRows([], nextId).length, 1);
});

test('initial list and empty or whitespace search request paginated all mode', () => {
  for (const query of [undefined, '', '   ']) {
    const params = createMachineSearchParams({ query });
    assert.equal(params.get('q'), '');
    assert.equal(params.get('all'), '1');
    assert.equal(params.get('limit'), '50');
    assert.equal(params.get('offset'), '0');
    assert.equal(params.has('sort'), false);
  }
});

test('nonempty search keeps query and server sort without all mode', () => {
  const params = createMachineSearchParams({ query: ' hostname:console-fixture ', sort: 'hostname', order: 'desc', limit: 25, offset: 50 });
  assert.equal(params.get('q'), 'hostname:console-fixture');
  assert.equal(params.has('all'), false);
  assert.equal(params.get('sort'), 'hostname');
  assert.equal(params.get('order'), 'desc');
  assert.equal(params.get('limit'), '25');
  assert.equal(params.get('offset'), '50');
});

test('clearing a search returns all mode and preserves explicit server sort', () => {
  const options = { query: 'ubuntu', sort: 'purpose', order: 'asc', limit: 100, offset: 0 };
  assert.equal(createMachineSearchParams(options).has('all'), false);
  options.query = '';
  const cleared = createMachineSearchParams(options);
  assert.equal(cleared.get('all'), '1');
  assert.equal(cleared.get('q'), '');
  assert.equal(cleared.get('sort'), 'purpose');
  assert.equal(cleared.get('limit'), '100');
  assert.equal(cleared.get('offset'), '0');
});

const deferred = () => {
  let resolve, reject;
  const promise = new Promise((done, fail) => { resolve = done; reject = fail; });
  return { promise, resolve, reject };
};

test('older machine GET cannot replace newer gateway snapshot', async () => {
  const oldRequest = deferred(), newRequest = deferred();
  const state = { machine: null, loading: false, error: '' };
  let requests = 0;
  const load = createMachineLoader({ getMachine: () => ++requests === 1 ? oldRequest.promise : newRequest.promise, getMachineId: () => 'machine-a', onMachine: machine => { state.machine = machine; }, onLoading: value => { state.loading = value; }, onError: value => { state.error = value; } });
  const first = load(), second = load();
  newRequest.resolve({ id: 'machine-a', gatewayNic: 12 });
  await second;
  oldRequest.resolve({ id: 'machine-a', gatewayNic: 11 });
  await first;
  assert.equal(state.machine.gatewayNic, 12);
  assert.equal(state.loading, false);
});

test('stale GET failure cannot clear latest loading or set error', async () => {
  const oldRequest = deferred(), newRequest = deferred();
  const state = { loading: false, error: '' };
  let requests = 0;
  const load = createMachineLoader({ getMachine: () => ++requests === 1 ? oldRequest.promise : newRequest.promise, getMachineId: () => 'machine-a', onMachine: () => {}, onLoading: value => { state.loading = value; }, onError: value => { state.error = value; } });
  const first = load(), second = load();
  oldRequest.reject(new Error('old request failed'));
  await first;
  assert.equal(state.loading, true);
  assert.equal(state.error, '');
  newRequest.resolve({ id: 'machine-a' });
  await second;
  assert.equal(state.loading, false);
});

test('GET for previous route identity cannot publish after route changes', async () => {
  const request = deferred();
  const state = { machine: null };
  let machineId = 'machine-a';
  const load = createMachineLoader({ getMachine: () => request.promise, getMachineId: () => machineId, onMachine: machine => { state.machine = machine; }, onLoading: () => {}, onError: () => {} });
  const pending = load();
  machineId = 'machine-b';
  request.resolve({ id: 'machine-a' });
  await pending;
  assert.equal(state.machine, null);
});

test('same NIC refresh preserves open draft during a pending gateway or DNS GET', async () => {
  let sequence = 0;
  const nextId = () => `draft-${++sequence}`;
  const draft = { wasOpen: false, interfaceId: null, name: '', ips: [] };
  const iface = { id: 12, name: 'eth1', ips: [{ id: 103, ip_address: '192.168.1.2', subnet_mask: '255.255.255.0' }] };
  syncInterfaceDraft(draft, true, iface, nextId);
  const refresh = deferred();
  const pending = refresh.promise.then(updated => syncInterfaceDraft(draft, true, updated, nextId));
  draft.name = 'draft-name';
  draft.ips[0].ip_address = '192.168.1.44';
  draft.ips.push(...createIpEditRows([{ ip_address: '192.168.1.45' }], nextId));
  const key = draft.ips[1].rowId;
  refresh.resolve({ ...iface, gateway: '192.168.1.1', dns_servers: ['1.1.1.1'], ips: structuredClone(iface.ips) });
  await pending;
  assert.equal(draft.name, 'draft-name');
  assert.equal(draft.ips[0].ip_address, '192.168.1.44');
  assert.equal(draft.ips.length, 2);
  assert.equal(draft.ips[1].rowId, key);
});

test('reopening or selecting another NIC seeds fresh server data', () => {
  const draft = { wasOpen: false, interfaceId: null, name: '', ips: [] };
  let sequence = 0;
  const nextId = () => `draft-${++sequence}`;
  const first = { id: 11, name: 'eth0', ips: [{ id: 101, ip_address: '10.0.0.2' }] };
  const second = { id: 12, name: 'eth1', ips: [{ id: 103, ip_address: '192.168.1.2' }] };
  syncInterfaceDraft(draft, true, first, nextId);
  draft.ips[0].ip_address = 'unsaved';
  syncInterfaceDraft(draft, true, second, nextId);
  assert.equal(draft.ips[0].id, 103);
  syncInterfaceDraft(draft, false, second, nextId);
  syncInterfaceDraft(draft, true, first, nextId);
  assert.equal(draft.ips[0].ip_address, '10.0.0.2');
});

test('basic save reloads canonical server ID on lowercase and uppercase UUID routes', async () => {
  const canonicalId = 'abcdef12-abcd-4abc-8def-abcdef123456';
  for (const routeId of [canonicalId, canonicalId.toUpperCase()]) {
    const state = { machine: null };
    let requests = 0;
    const load = createMachineLoader({
      getMachine: async requestedId => {
        assert.equal(requestedId, routeId);
        requests++;
        return { id: canonicalId, hostname: requests === 1 ? 'before' : 'after' };
      },
      getMachineId: () => routeId,
      onMachine: machine => { state.machine = machine; },
      onLoading: () => {}, onError: () => {},
    });
    await load();
    await refreshUpdatedMachine({ id: canonicalId, hostname: 'after' }, routeId, load);
    assert.equal(requests, 2, routeId);
    assert.equal(state.machine.hostname, 'after', routeId);
  }
});

test('basic save for another machine cannot refresh the current route', async () => {
  let requests = 0;
  await refreshUpdatedMachine({ id: 'abcdef12-abcd-4abc-8def-abcdef123456' }, 'abcdef12-abcd-4abc-8def-abcdef123457', () => { requests++; });
  assert.equal(requests, 0);
});
