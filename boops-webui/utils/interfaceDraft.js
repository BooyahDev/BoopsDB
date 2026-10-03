import { createIpEditRows } from './interfaceRows.js';

export function syncInterfaceDraft(draft, open, iface, nextId) {
  if (!open || !iface) { draft.wasOpen = false; return false; }
  if (draft.wasOpen && draft.interfaceId === iface.id) return false;
  draft.wasOpen = true;
  draft.interfaceId = iface.id;
  draft.name = iface.name;
  draft.ips = createIpEditRows(iface.ips, nextId);
  return true;
}
