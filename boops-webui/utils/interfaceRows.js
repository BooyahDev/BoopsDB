export function createIpEditRows(ips, nextId) {
  const source = ips?.length ? ips : [{ ip_address: '', subnet_mask: '255.255.255.0' }];
  return source.map(ip => ({
    ...ip,
    rowId: ip.id != null ? `saved-${ip.id}` : nextId(),
    subnet_mask: ip.subnet_mask || '255.255.255.0',
    dns_register: !!ip.dns_register,
  }));
}

export function toIpPayload(rows) {
  return rows.map(row => ({
    ...(row.id != null ? { id: row.id } : {}),
    ip_address: row.ip_address.trim(),
    subnet_mask: row.subnet_mask?.trim() || '255.255.255.0',
    dns_register: !!row.dns_register,
  }));
}
