export function getUpdateStatus(value, now = Date.now()) {
  const timestamp = value ? Date.parse(value) : NaN;
  if (!Number.isFinite(timestamp)) return { color: 'purple', label: '未更新' };
  const age = now - timestamp;
  if (age <= 5 * 60 * 1000) return { color: 'success', label: '5分以内' };
  if (age < 3 * 24 * 60 * 60 * 1000) return { color: 'warning', label: '5分超' };
  return { color: 'error', label: '3日以上' };
}
