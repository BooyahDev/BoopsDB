import { setTimeout as delay } from 'node:timers/promises';

export const NOTION_ARCHIVE_PAGE_ID = '3f1f663606fc8061a287f4838ff648c3';
const TITLE = 'BoopsDB Archive [managed]';
export const NOTION_ARCHIVE_KEY = 'BoopsDB同期キー';
const text = parts => (parts || []).map(part => part.plain_text ?? part.text?.content ?? '').join('');
function richText(value) {
  const parts = [];
  // Split on Unicode characters to avoid splitting surrogate pairs.
  const characters = Array.from(value);
  for (let i = 0; i < characters.length; i += 2000) parts.push({ type: 'text', text: { content: characters.slice(i, i + 2000).join('') } });
  if (parts.length > 100) throw new Error('Notion cell exceeds rich text limit');
  return parts;
}
export function markdownCells(markdown) {
  const rows = markdown.split('\n').filter(line => line.startsWith('|')).map(line =>
    line.split('|').slice(1, -1).map(cell => cell.slice(1, -1)
      .replaceAll('<br>', '\n').replaceAll('&#124;', '|').replaceAll('&#96;', '`')
      .replaceAll('&lt;', '<').replaceAll('&gt;', '>').replaceAll('&amp;', '&')));
  if (rows.length < 2) throw new Error('Archive table missing');
  rows.splice(1, 1); // Markdown separator is not a data row.
  if (rows.some(row => row.length !== rows[0].length)) throw new Error('Archive table width mismatch');
  return rows;
}

export function createNotionPublisher({ token, pageId = NOTION_ARCHIVE_PAGE_ID, fetchImpl = fetch, requestIntervalMs = 350 }) {
  if (!/^(?:[a-f0-9]{32}|[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})$/i.test(pageId)) throw new Error('Invalid NOTION_ARCHIVE_PAGE_ID');
  let nextRequest = 0;
  async function request(method, route, body) {
    await delay(Math.max(0, nextRequest - Date.now()));
    nextRequest = Date.now() + requestIntervalMs;
    const response = await fetchImpl(`https://api.notion.com/v1/${route}`, {
      method, headers: { Authorization: `Bearer ${token}`, 'Notion-Version': '2025-09-03', 'Content-Type': 'application/json' },
      ...(body ? { body: JSON.stringify(body) } : {}), signal: AbortSignal.timeout(30000),
    });
    if (!response.ok) {
      const error = new Error(`Notion archive request failed (HTTP ${response.status})`);
      error.status = response.status;
      const detail = await response.json().catch(() => ({}));
      error.retryMs = Math.max(0, Number(response.headers.get('retry-after')) * 1000 || 0);
      throw error;
    }
    return response.json();
  }
  async function queryAll(sourceId, filter) {
    const pages = [];
    let cursor;
    do {
      const result = await request('POST', `data_sources/${sourceId}/query`, {
        filter, page_size: 100, ...(cursor ? { start_cursor: cursor } : {}),
      });
      pages.push(...result.results);
      cursor = result.has_more ? result.next_cursor : null;
    } while (cursor);
    return pages;
  }
  async function propertyText(page, property) {
    const value = page.properties[property.name] || Object.values(page.properties).find(value => value.id === property.id);
    const parts = value?.[property.type] || [];
    if (parts.length < 25) return text(parts);
    // Notion can truncate inline property values. Read long properties with pagination.
    const result = [];
    let cursor;
    do {
      const body = await request('GET', `pages/${page.id}/properties/${encodeURIComponent(property.id)}?page_size=100${cursor ? `&start_cursor=${encodeURIComponent(cursor)}` : ''}`);
      if (body.object !== 'list') return text([body[property.type]]);
      result.push(...body.results.map(item => item[property.type]));
      cursor = body.has_more ? body.next_cursor : null;
    } while (cursor);
    return text(result);
  }
  function expectedType(field) {
    if (['is_virtual', 'dns_register'].includes(field)) return 'checkbox';
    if (['interface_id', 'ip_id'].includes(field)) return 'number';
    return 'rich_text';
  }
  function valueFor(type, value) {
    if (type === 'checkbox') return value === '1' || value === 'true';
    if (type === 'number') return value === '' ? null : Number(value);
    return richText(value);
  }
  return async markdown => {
    const [headers, ...rows] = markdownCells(markdown);
    if (!['id', 'hostname', 'interface_id', 'ip_id'].every(field => headers.includes(field))) throw new Error('Archive identity columns missing');
    // Validate all values before any remote mutation; never silently truncate content.
    const desired = rows.map(row => {
      const values = Object.fromEntries(headers.map((field, index) => [field, row[index]]));
      const key = 'boopsdb:' + JSON.stringify([values.id, values.interface_id, values.ip_id]);
      const fields = Object.fromEntries(headers.map(field => [field, valueFor(field === 'hostname' ? 'title' : expectedType(field), values[field])]));
      return { key, fields };
    });
    if (new Set(desired.map(row => row.key)).size !== desired.length) throw new Error('Duplicate archive row identities');
    const database = await request('GET', `databases/${pageId}`);
    if (database.data_sources?.length !== 1) {
      const error = new Error('Notion archive requires a database with one data source');
      error.code = 'NOTION_MULTIPLE_DATA_SOURCES'; throw error;
    }
    const sourceId = database.data_sources[0].id;
    let source = await request('GET', `data_sources/${sourceId}`);
    const title = Object.values(source.properties).find(property => property.type === 'title');
    if (!title) throw new Error('Notion database title property missing');
    const additions = {};
    for (const field of [...headers.filter(field => field !== 'hostname'), NOTION_ARCHIVE_KEY]) {
      const type = expectedType(field);
      const existing = source.properties[field];
      if (existing && existing.type !== type) {
        const error = new Error('Notion property type mismatch');
        error.code = 'NOTION_PROPERTY_TYPE_MISMATCH'; throw error;
      }
      if (!existing) additions[field] = { [type]: {} };
    }
    let updated = false;
    if (Object.keys(additions).length) {
      source = await request('PATCH', `data_sources/${sourceId}`, { properties: additions });
      updated = true;
    }
    const schema = Object.fromEntries(headers.map(field => [field, field === 'hostname'
      ? { ...title, name: title.name || Object.keys(source.properties).find(name => source.properties[name].id === title.id) }
      : { ...source.properties[field], name: field }]));
    const keyProperty = source.properties[NOTION_ARCHIVE_KEY];
    const existing = await queryAll(sourceId, { property: keyProperty.id, rich_text: { starts_with: 'boopsdb:' } });
    const groups = new Map();
    for (const page of existing) {
      const key = await propertyText(page, { ...keyProperty, name: NOTION_ARCHIVE_KEY });
      if (!groups.has(key)) groups.set(key, []);
      groups.get(key).push(page);
    }
    const obsolete = [];
    for (const row of desired) {
      const candidates = groups.get(row.key) || [];
      const page = candidates[0];
      groups.delete(row.key);
      obsolete.push(...candidates.slice(1));
      const properties = { [keyProperty.id]: { rich_text: richText(row.key) } };
      let changed = !page;
      for (const field of headers) {
        const property = schema[field];
        properties[property.id] = { [property.type]: row.fields[field] };
        if (page) {
          const actual = page.properties[property.name] || Object.values(page.properties).find(value => value.id === property.id);
          if (['title', 'rich_text'].includes(property.type)) {
            if (await propertyText(page, property) !== text(row.fields[field])) changed = true;
          } else if (actual?.[property.type] !== row.fields[field]) changed = true;
        }
      }
      if (Buffer.byteLength(JSON.stringify({ properties })) > 450000) throw new Error('Notion record exceeds request size limit');
      if (changed) {
        await request(page ? 'PATCH' : 'POST', page ? `pages/${page.id}` : 'pages', {
          ...(!page ? { parent: { type: 'data_source_id', data_source_id: sourceId } } : {}), properties,
        });
        updated = true;
      }
    }
    obsolete.push(...[...groups.values()].flat());
    // Only remove obsolete managed records after every desired row has been saved.
    for (const page of obsolete) { await request('PATCH', `pages/${page.id}`, { in_trash: true }); updated = true; }
    // Migrate only the old generated body-table entry, leaving user-created rows alone.
    const legacy = await queryAll(sourceId, { property: title.id, title: { equals: TITLE } });
    for (const page of legacy) {
      if (await propertyText(page, { ...keyProperty, name: NOTION_ARCHIVE_KEY })) continue;
      const body = await request('GET', `blocks/${page.id}/children?page_size=100`);
      if (!body.results.some(block => block.type === 'toggle' && text(block.toggle.rich_text) === TITLE)) continue;
      await request('PATCH', `pages/${page.id}`, { in_trash: true });
      updated = true;
    }
    return { updated };
  };
}
