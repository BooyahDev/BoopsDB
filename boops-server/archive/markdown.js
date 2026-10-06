// Explicit fields keep heartbeat timestamps out of both the export and change detection.
const machineFields = ['id', 'hostname', 'model_info', 'purpose', 'usage_desc', 'memo', 'cpu_info', 'cpu_arch', 'memory_size', 'disk_info', 'os_name', 'is_virtual', 'parent_machine_id'];
const nicFields = ['interface_id', 'interface_name', 'mac_address', 'gateway', 'dns_servers'];
const ipFields = ['ip_id', 'ip_address', 'subnet_mask', 'dns_register'];

function cell(value) {
  return String(value ?? '').replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;')
    .replaceAll('|', '&#124;').replace(/\r\n|\r|\n/g, '<br>').replaceAll('`', '&#96;');
}
function table(fields, rows) {
  return [`| ${fields.join(' | ')} |`, `| ${fields.map(() => '---').join(' | ')} |`,
    ...rows.map(row => `| ${fields.map(field => cell(row[field])).join(' | ')} |`)].join('\n');
}
export function renderMarkdown(rows) {
  return '# BoopsDB Archive\n\nBoopsDB の構成情報。Heartbeat・作成日時・更新日時は含みません。\n\n'
    + '1 行につき 1 つの IP 設定を掲載します。複数の NIC・IP があるマシンは複数行で表示し、NIC・IP がない場合も掲載します。\n\n'
    + table([...machineFields, ...nicFields, ...ipFields], rows) + '\n';
}
export async function readMarkdown(db) {
  // A single SELECT gives a consistent statement snapshot, including machines without NICs.
  const [rows] = await db.query(`SELECT
    m.id, m.hostname, m.model_info, m.purpose, m.usage_desc, m.memo,
    m.cpu_info, m.cpu_arch, m.memory_size, m.disk_info, m.os_name, m.is_virtual, m.parent_machine_id,
    i.id AS interface_id, i.machine_id, i.name AS interface_name, i.mac_address, i.gateway, i.dns_servers,
    ip.id AS ip_id, ip.ip_address, ip.subnet_mask, ip.dns_register
    FROM machines m LEFT JOIN interfaces i ON i.machine_id = m.id
    LEFT JOIN interface_ips ip ON ip.interface_id = i.id
    ORDER BY m.id ASC, i.id ASC, ip.id ASC`);
  return renderMarkdown(rows);
}
