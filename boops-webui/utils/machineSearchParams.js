export function createMachineSearchParams({ query = '', sort = '', order = 'asc', limit = 50, offset = 0 } = {}) {
  const params = new URLSearchParams({ q: query?.trim() || '', limit: String(limit), offset: String(offset) });
  if (!params.get('q')) params.set('all', '1');
  if (sort) { params.set('sort', sort); params.set('order', order); }
  return params;
}
