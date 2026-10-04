import http from 'node:http';

// Local, in-memory browser fixture. Never connects to the deployed API or a DB.
const firstId = '11111111-1111-4111-8111-111111111111';
const secondId = '22222222-2222-4222-8222-222222222222';
const seed = [
  { id: firstId, hostname: 'console-fixture', os_name: 'Ubuntu 24.04', cpu_info: 'Fixture CPU', cpu_arch: 'x86_64', memory_size: '16 GB', disk_info: '512 GB', purpose: 'WebUI 検証', is_virtual: false, parent_machine_id: null, memo: '## 検証用マシン\n- 2 NIC\n- 複数 IP', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-02T00:00:00Z', last_alive: '2026-01-02T00:00:00Z', interfaces: [
    { id: 11, name: 'eth0', mac_address: '02:00:00:00:00:11', gateway: '10.0.0.1', dns_servers: ['1.1.1.1'], ips: [{ id: 101, ip_address: '10.0.0.2', subnet_mask: '255.255.255.0', dns_register: true }, { id: 102, ip_address: '10.0.0.3', subnet_mask: '255.255.255.0', dns_register: false }] },
    { id: 12, name: 'eth1', mac_address: '02:00:00:00:00:12', gateway: '192.168.1.1', dns_servers: ['8.8.8.8'], ips: [{ id: 103, ip_address: '192.168.1.2', subnet_mask: '255.255.255.0', dns_register: false }] },
  ] },
  { id: secondId, hostname: 'console-fixture', os_name: 'Debian 12', cpu_info: 'Fixture CPU', cpu_arch: 'arm64', memory_size: '8 GB', purpose: '同名マシンの検証', is_virtual: true, parent_machine_id: firstId, memo: '', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-02T00:00:00Z', last_alive: null, interfaces: [{ id: 13, name: 'ens3', mac_address: '02:00:00:00:00:13', gateway: '', dns_servers: [], ips: [{ id: 104, ip_address: '10.2.0.2', subnet_mask: '255.255.255.0', dns_register: false }] }] },
  { id: '33333333-3333-4333-8333-333333333333', hostname: 'legacy-no-date', os_name: 'Linux', purpose: '登録日時 NULL', is_virtual: false, created_at: null, interfaces: [] },
];
let machines = structuredClone(seed), nextNicId = 20, nextIpId = 200;
const requests = [];
const reply = (res, status, data) => {
  res.writeHead(status, { 'Content-Type': 'application/json', 'Access-Control-Allow-Origin': '*', 'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS', 'Access-Control-Allow-Headers': 'Content-Type' });
  res.end(JSON.stringify(data));
};
const failure = message => { throw new Error(message); };
const gatewayValue = value => !value?.trim() || value.trim() === '0.0.0.0' ? '' : value.trim();
const ipv4 = value => /^\d+\.\d+\.\d+\.\d+$/.test(value) && value.split('.').every(part => Number(part) <= 255);
const prepareIps = (ips, previous = []) => {
  if (!Array.isArray(ips) || !ips.length) failure('最低 1 個の IP が必要です。');
  const seen = new Set();
  return ips.map(ip => {
    if (!ipv4(ip.ip_address) || !ipv4(ip.subnet_mask)) failure('IP / サブネットの形式が不正です。');
    if ('rowId' in ip) failure('画面 ID が API に送信されています。');
    if (ip.id != null && (!previous.some(old => old.id === ip.id) || seen.has(ip.id))) failure('未知または重複した IP ID です。');
    if (ip.id != null) seen.add(ip.id);
    return { id: ip.id ?? nextIpId++, ip_address: ip.ip_address, subnet_mask: ip.subnet_mask, dns_register: !!ip.dns_register };
  }).sort((a, b) => a.id - b.id);
};
const server = http.createServer(async (req, res) => {
  if (req.method === 'OPTIONS') return reply(res, 200, {});
  const url = new URL(req.url, 'http://127.0.0.1');
  let body = null;
  try {
    if (req.method !== 'GET') {
      let raw = '';
      for await (const chunk of req) raw += chunk;
      body = raw ? JSON.parse(raw) : {};
      requests.push({ method: req.method, path: url.pathname, body });
    }
    if (url.pathname === '/__requests') return reply(res, 200, requests);
    if (url.pathname === '/__reset' && req.method === 'POST') { machines = structuredClone(seed); requests.length = 0; nextNicId = 20; nextIpId = 200; return reply(res, 200, { reset: true }); }
    if (url.pathname === '/api/search/suggestions') return reply(res, 200, { suggestions: { hostnames: ['console-fixture', 'legacy-no-date'], osNames: ['Ubuntu 24.04', 'Debian 12'], purposes: ['WebUI 検証'] }, statistics: { total_machines: machines.length, virtual_machines: 1, physical_machines: machines.length - 1, alive_last_day: 0 } });
    if (url.pathname === '/api/machines/search') {
      const q = (url.searchParams.get('q') || '').trim();
      const sort = url.searchParams.get('sort');
      const direction = url.searchParams.get('order') === 'desc' ? -1 : 1;
      const limit = Number(url.searchParams.get('limit')) || 50;
      const offset = Number(url.searchParams.get('offset')) || 0;
      let result = !q && url.searchParams.get('all') !== '1'
        ? []
        : machines.filter(machine => !q || JSON.stringify(machine).toLowerCase().includes(q.replace(/^hostname:/, '').toLowerCase()));
      result = [...result].sort((a, b) => {
        if (sort) return String(a[sort] ?? '').localeCompare(String(b[sort] ?? '')) * direction || a.id.localeCompare(b.id);
        if (a.created_at == null !== (b.created_at == null)) return a.created_at == null ? 1 : -1;
        return String(a.created_at ?? '').localeCompare(String(b.created_at ?? '')) || a.id.localeCompare(b.id);
      });
      return reply(res, 200, { results: result.slice(offset, offset + limit), pagination: { total: result.length, limit, offset, hasMore: offset + limit < result.length } });
    }
    if (url.pathname === '/api/machines' && req.method === 'GET') return reply(res, 200, machines);
    if (url.pathname === '/api/machines' && req.method === 'POST') {
      if (!body.hostname || !body.interfaces) failure('ホスト名と NIC が必要です。');
      if (Object.values(body.interfaces).filter(iface => gatewayValue(iface.gateway)).length > 1) failure('ゲートウェイは 1 NIC にだけ設定できます。');
      const machine = { ...body, id: `44444444-4444-4444-8444-${String(machines.length).padStart(12, '0')}`, created_at: new Date().toISOString(), interfaces: Object.entries(body.interfaces).map(([name, iface]) => ({ ...iface, id: nextNicId++, name, gateway: gatewayValue(iface.gateway), ips: prepareIps(iface.ips) })) };
      machines.push(machine); return reply(res, 201, { id: machine.id });
    }
    const parts = url.pathname.split('/').filter(Boolean).map(decodeURIComponent);
    const machine = machines.find(item => item.id === parts[2]);
    if (!machine) return reply(res, 404, { error: 'マシンが見つかりません。' });
    if (parts[1] === 'machines' && parts.length === 3) {
      if (req.method === 'GET') return reply(res, 200, machine);
      if (req.method === 'DELETE') { machines = machines.filter(item => item !== machine); return reply(res, 200, { success: true }); }
    }
    if (parts[1] === 'machines' && parts[3]?.startsWith('update-')) {
      const fields = { 'update-hostname': 'hostname', 'update-purpose': 'purpose', 'update-memo': 'memo', 'update-parent-id': 'parent_machine_id' };
      const field = fields[parts[3]];
      if (field) machine[field] = body[field];
      else if (parts[3] === 'update-vm-status') { machine.is_virtual = body.is_virtual; machine.parent_machine_id = body.parent_machine_id; }
      else return reply(res, 404, { error: 'Unknown fixture route' });
      return reply(res, 200, { success: true });
    }
    if (parts[1] === 'machines' && parts[3] === 'interfaces' && parts.length === 4 && req.method === 'POST') {
      if (machine.interfaces.some(iface => iface.name === body.name)) failure('NIC 名が重複しています。');
      const iface = { ...body, id: nextNicId++, ips: prepareIps(body.ips), gateway: gatewayValue(body.gateway) };
      if (iface.gateway) machine.interfaces.forEach(other => { other.gateway = ''; });
      machine.interfaces.push(iface); return reply(res, 201, { id: iface.id });
    }
    const nicName = parts[1] === 'interfaces' ? parts[3] : parts[4];
    const iface = machine.interfaces.find(item => item.name === nicName);
    if (!iface) return reply(res, 404, { error: 'NIC が見つかりません。' });
    if (req.method === 'DELETE') { machine.interfaces = machine.interfaces.filter(item => item !== iface); return reply(res, 200, { success: true }); }
    const action = parts[4];
    if (req.method === 'PUT' && parts[1] === 'interfaces') {
      if (action === 'update-gateway') {
        const value = gatewayValue(body.gateway);
        if (value && !ipv4(value)) failure('ゲートウェイの IPv4 アドレスが不正です。');
        if (value) machine.interfaces.forEach(other => { other.gateway = ''; });
        iface.gateway = value;
      } else if (action === 'update-dns') iface.dns_servers = body.dns_servers;
      else if (action === 'ips') iface.ips = prepareIps(body.ips, iface.ips);
      else if (action === 'update-name') {
        if (!body.name?.trim() || machine.interfaces.some(other => other !== iface && other.name === body.name)) failure('NIC 名が空か重複しています。');
        iface.name = body.name;
      } else return reply(res, 404, { error: 'Unknown fixture route' });
      return reply(res, 200, { success: true });
    }
    return reply(res, 404, { error: 'Unknown fixture route' });
  } catch (error) { return reply(res, 400, { error: error.message }); }
});
server.listen(Number(process.env.FIXTURE_PORT) || 4318, '127.0.0.1', () => console.log(`Fixture API: http://127.0.0.1:${server.address().port}/api; detail: ${firstId}`));
