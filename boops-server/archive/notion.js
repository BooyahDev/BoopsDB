import { setTimeout as delay } from 'node:timers/promises';

export const NOTION_ARCHIVE_PAGE_ID = '3f1f663606fc8061a287f4838ff648c3';
const TITLE = 'BoopsDB Archive [managed]';
const STAGING = TITLE + ' [updating]';
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
      error.retryMs = Math.max(0, Number(response.headers.get('retry-after')) * 1000 || 0);
      throw error;
    }
    return response.json();
  }
  async function destinationPage() {
    try {
      const page = await request('GET', `pages/${pageId}`);
      if (page.archived || page.in_trash) throw new Error('Notion destination page is archived');
      return page.id;
    } catch (error) {
      if (error.status !== 404) throw error;
    }
    // Database links carry a view ID, but archives belong in an entry page's body.
    const database = await request('GET', `databases/${pageId}`);
    if (database.data_sources?.length !== 1) {
      const error = new Error('Notion archive requires a database with one data source or a page ID');
      error.code = 'NOTION_MULTIPLE_DATA_SOURCES';
      throw error;
    }
    const sourceId = database.data_sources[0].id;
    const source = await request('GET', `data_sources/${sourceId}`);
    const title = Object.values(source.properties).find(property => property.type === 'title');
    if (!title) throw new Error('Notion database title property missing');
    const existing = await request('POST', `data_sources/${sourceId}/query`, {
      filter: { property: title.id, title: { equals: TITLE } }, page_size: 2,
    });
    if (existing.results.length > 1 || existing.has_more) {
      const error = new Error('Multiple managed Notion archive pages found');
      error.code = 'NOTION_DUPLICATE_ARCHIVE_PAGES';
      throw error;
    }
    if (existing.results.length) return existing.results[0].id;
    const created = await request('POST', 'pages', {
      parent: { type: 'data_source_id', data_source_id: sourceId },
      properties: { [title.id]: { type: 'title', title: richText(TITLE) } },
    });
    return created.id;
  }
  async function children(id) {
    const blocks = [];
    let cursor;
    do {
      const result = await request('GET', `blocks/${id}/children?page_size=100${cursor ? `&start_cursor=${encodeURIComponent(cursor)}` : ''}`);
      blocks.push(...result.results);
      cursor = result.has_more ? result.next_cursor : null;
    } while (cursor);
    return blocks;
  }
  async function same(container, desired) {
    const contents = await children(container.id);
    if (contents.length !== 1 || contents[0].type !== 'table') return false;
    const table = contents[0];
    if (table.table.table_width !== desired[0].length || !table.table.has_column_header || table.table.has_row_header) return false;
    const rows = await children(table.id);
    return JSON.stringify(rows.map(row => row.type === 'table_row' ? row.table_row.cells.map(text) : null)) === JSON.stringify(desired);
  }
  async function cleanup(blocks, keepId) {
    for (const block of blocks) if (block.id !== keepId) await request('DELETE', `blocks/${block.id}`);
  }
  return async markdown => {
    const desired = markdownCells(markdown);
    const rowBlocks = desired.map(cells => ({ object: 'block', type: 'table_row', table_row: { cells: cells.map(richText) } }));
    const destination = await destinationPage();
    const managed = (await children(destination)).filter(block => block.type === 'toggle' && [TITLE, STAGING].includes(text(block.toggle.rich_text)));
    // Compare actual remote rows; a digest alone would miss manual edits.
    for (const block of managed.filter(block => text(block.toggle.rich_text) === TITLE).reverse()) {
      if (await same(block, desired)) { await cleanup(managed, block.id); return { updated: false }; }
    }
    // Build a replacement separately. Failed/ambiguous writes are rediscovered on retry.
    // Do not remove the last complete snapshot until the new table is fully populated.
    await cleanup(managed.filter(block => text(block.toggle.rich_text) === STAGING));
    const created = await request('PATCH', `blocks/${destination}/children`, {
      children: [{ object: 'block', type: 'toggle', toggle: { rich_text: richText(STAGING) } }],
    });
    const containerId = created.results[0].id;
    const tableResult = await request('PATCH', `blocks/${containerId}/children`, {
      children: [{ object: 'block', type: 'table', table: {
        table_width: desired[0].length, has_column_header: true, has_row_header: false, children: [rowBlocks[0]],
      } }],
    });
    const tableId = tableResult.results[0].id;
    let batch = [];
    async function flush() {
      if (batch.length) await request('PATCH', `blocks/${tableId}/children`, { children: batch });
      batch = [];
    }
    for (const row of rowBlocks.slice(1)) {
      if (batch.length === 100 || Buffer.byteLength(JSON.stringify({ children: [...batch, row] })) > 450000) await flush();
      if (Buffer.byteLength(JSON.stringify({ children: [row] })) > 450000) throw new Error('Notion row exceeds request size limit');
      batch.push(row);
    }
    await flush();
    await request('PATCH', `blocks/${containerId}`, { toggle: { rich_text: richText(TITLE) } });
    await cleanup(managed.filter(block => text(block.toggle.rich_text) === TITLE));
    return { updated: true };
  };
}
