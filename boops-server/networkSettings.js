import { isIP } from 'node:net';

export function apiError(status, message) {
  return Object.assign(new Error(message), { status });
}

export function normalizeGateway(value) {
  if (value == null) return '';
  if (typeof value !== 'string') throw apiError(400, 'Gateway must be an IPv4 address or empty');
  const gateway = value.trim();
  return gateway === '0.0.0.0' ? '' : gateway;
}

export function validateGateway(value) {
  const gateway = normalizeGateway(value);
  if (gateway && isIP(gateway) !== 4) throw apiError(400, 'Gateway must be an IPv4 address');
  return gateway;
}

function optionalId(value, label) {
  if (value === undefined) return undefined;
  if (!Number.isSafeInteger(value) || value < 1) throw apiError(400, `${label} must be a positive integer`);
  return value;
}

export function validateInterfaceName(name) {
  if (typeof name !== 'string' || !name.trim() || name.length > 50 || name !== name.trim() || /[\s\x00-\x1f\/]/.test(name)) {
    throw apiError(400, 'Interface name must be a non-empty name of at most 50 characters');
  }
  return name;
}

export function validateDns(value) {
  if (value == null) return [];
  if (!Array.isArray(value) || value.some(ip => typeof ip !== 'string' || isIP(ip.trim()) !== 4)) {
    throw apiError(400, 'DNS servers must be an array of IPv4 addresses');
  }
  return value.map(ip => ip.trim());
}

export function validateMac(value) {
  if (value == null || value === '') return '';
  if (typeof value !== 'string' || !/^([0-9a-f]{2}[:-]){5}[0-9a-f]{2}$/i.test(value)) {
    throw apiError(400, 'MAC address must be a valid MAC address or empty');
  }
  return value;
}

export function validateIps(ips) {
  if (!Array.isArray(ips) || ips.length === 0) throw apiError(400, 'At least one IP address is required');
  const ids = new Set();
  return ips.map(ip => {
    if (!ip || typeof ip !== 'object' || Array.isArray(ip) || typeof ip.ip_address !== 'string' || isIP(ip.ip_address.trim()) !== 4) {
      throw apiError(400, 'IP address must be an IPv4 address');
    }
    const subnet = ip.subnet_mask == null ? '' : ip.subnet_mask;
    if (typeof subnet !== 'string' || (subnet !== '' && isIP(subnet) !== 4)) throw apiError(400, 'Subnet mask must be an IPv4 mask or empty');
    const id = optionalId(ip.id, 'IP ID');
    if (id !== undefined && ids.has(id)) throw apiError(400, 'Duplicate IP ID');
    ids.add(id);
    if (ip.dns_register !== undefined && ![true, false, 0, 1].includes(ip.dns_register)) throw apiError(400, 'DNS registration must be boolean');
    return { id, ip_address: ip.ip_address.trim(), subnet_mask: subnet, dns_register: Boolean(ip.dns_register) };
  });
}

export function validateInterfacePayloads(interfaces) {
  if (!interfaces || typeof interfaces !== 'object' || Array.isArray(interfaces)) throw apiError(400, 'Interfaces must be a name-keyed object');
  const nicIds = new Set();
  const ipIds = new Set();
  const normalized = Object.entries(interfaces).map(([name, iface]) => {
    validateInterfaceName(name);
    if (!iface || typeof iface !== 'object' || Array.isArray(iface)) throw apiError(400, 'Interface payload must be an object');
    const id = optionalId(iface.id, 'Interface ID');
    if (id !== undefined && nicIds.has(id)) throw apiError(400, 'Duplicate interface ID');
    nicIds.add(id);
    const ips = validateIps(iface.ips);
    for (const ip of ips) {
      if (ip.id !== undefined && ipIds.has(ip.id)) throw apiError(400, 'Duplicate IP ID');
      ipIds.add(ip.id);
    }
    return { id, name, ips, gateway: validateGateway(iface.gateway), dns_servers: validateDns(iface.dns_servers), mac_address: validateMac(iface.mac_address) };
  });
  if (normalized.filter(iface => iface.gateway !== '').length > 1) throw apiError(400, 'Only one interface may have a gateway');
  return normalized;
}

export async function withMachineTransaction(database, machineId, callback) {
  const connection = await database.getConnection();
  try {
    await connection.beginTransaction();
    const [machines] = await connection.query('SELECT id FROM machines WHERE id = ? FOR UPDATE', [machineId]);
    if (!machines.length) throw apiError(404, 'Machine not found');
    const result = await callback(connection);
    await connection.commit();
    return result;
  } catch (error) {
    await connection.rollback();
    throw error;
  } finally {
    connection.release();
  }
}

export async function findInterfaceByName(connection, machineId, name) {
  const [interfaces] = await connection.query('SELECT * FROM interfaces WHERE machine_id = ? AND name = ? ORDER BY id ASC FOR UPDATE', [machineId, name]);
  if (!interfaces.length) throw apiError(404, 'Interface not found for this machine');
  if (interfaces.length > 1) throw apiError(409, 'Multiple interfaces have this name; use explicit IDs in the full machine PUT');
  return interfaces[0];
}

function planIps(existing, ips) {
  const reserved = new Set(ips.filter(ip => ip.id !== undefined).map(ip => ip.id));
  const used = new Set();
  const planned = ips.map(ip => {
    let row;
    if (ip.id !== undefined) {
      row = existing.find(candidate => candidate.id === ip.id);
      if (!row) throw apiError(400, 'Unknown IP ID or IP ID belongs to another interface');
    } else {
      row = existing.find(candidate => !used.has(candidate.id) && !reserved.has(candidate.id) && candidate.ip_address === ip.ip_address && candidate.subnet_mask === ip.subnet_mask);
    }
    if (row) used.add(row.id);
    return { ...ip, id: row?.id };
  });
  return { planned, removed: existing.filter(row => !used.has(row.id)) };
}

async function saveIps(connection, interfaceId, { planned, removed }) {
  for (const row of removed) await connection.query('DELETE FROM interface_ips WHERE id = ?', [row.id]);
  for (const ip of planned) {
    if (ip.id !== undefined) {
      await connection.query('UPDATE interface_ips SET ip_address = ?, subnet_mask = ?, dns_register = ? WHERE id = ?', [ip.ip_address, ip.subnet_mask, ip.dns_register, ip.id]);
    } else {
      await connection.query('INSERT INTO interface_ips (interface_id, ip_address, subnet_mask, dns_register) VALUES (?, ?, ?, ?)', [interfaceId, ip.ip_address, ip.subnet_mask, ip.dns_register]);
    }
  }
}

export async function reconcileIps(connection, interfaceId, ips) {
  const [existing] = await connection.query('SELECT id, ip_address, subnet_mask, dns_register FROM interface_ips WHERE interface_id = ? ORDER BY id ASC FOR UPDATE', [interfaceId]);
  await saveIps(connection, interfaceId, planIps(existing, ips));
}

export async function reconcileInterfaces(connection, machineId, interfaces) {
  const [existing] = await connection.query('SELECT * FROM interfaces WHERE machine_id = ? ORDER BY id ASC FOR UPDATE', [machineId]);
  const reserved = new Set(interfaces.filter(iface => iface.id !== undefined).map(iface => iface.id));
  const used = new Set();
  const planned = [];
  // Resolve ownership and the entire final set before issuing the first write.
  for (const iface of interfaces) {
    let row;
    if (iface.id !== undefined) {
      row = existing.find(candidate => candidate.id === iface.id);
      if (!row) throw apiError(400, 'Unknown interface ID or ID belongs to another machine');
    } else {
      const sameName = existing.filter(candidate => candidate.name === iface.name);
      if (sameName.length > 1) throw apiError(409, 'Multiple interfaces have this name; provide explicit IDs');
      row = sameName[0];
      if (row && reserved.has(row.id)) throw apiError(400, 'Interface ID selected more than once');
    }
    if (row && used.has(row.id)) throw apiError(400, 'Interface ID selected more than once');
    if (row) used.add(row.id);
    const [ips] = row ? await connection.query('SELECT id, ip_address, subnet_mask, dns_register FROM interface_ips WHERE interface_id = ? ORDER BY id ASC FOR UPDATE', [row.id]) : [[]];
    planned.push({ ...iface, id: row?.id, ipPlan: planIps(ips, iface.ips) });
  }
  // Child rows must be removed before their parent NIC rows.
  for (const row of existing.filter(iface => !used.has(iface.id))) {
    await connection.query('DELETE FROM interface_ips WHERE interface_id = ?', [row.id]);
    await connection.query('DELETE FROM interfaces WHERE id = ?', [row.id]);
  }
  for (const iface of planned) {
    let id = iface.id;
    if (id !== undefined) {
      await connection.query('UPDATE interfaces SET name = ?, gateway = ?, dns_servers = ?, mac_address = ? WHERE id = ?', [iface.name, iface.gateway, iface.dns_servers.join(','), iface.mac_address, id]);
    } else {
      const [result] = await connection.query('INSERT INTO interfaces (machine_id, name, gateway, dns_servers, mac_address) VALUES (?, ?, ?, ?, ?)', [machineId, iface.name, iface.gateway, iface.dns_servers.join(','), iface.mac_address]);
      id = result.insertId;
    }
    await saveIps(connection, id, iface.ipPlan);
  }
}
