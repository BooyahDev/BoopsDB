import test from 'node:test';
import assert from 'node:assert/strict';
import { renderMarkdown } from '../archive/markdown.js';
import { createNotionPublisher, markdownCells, NOTION_ARCHIVE_PAGE_ID } from '../archive/notion.js';
import { archiveFromEnv, combineArchives, createArchiveService } from '../archive/service.js';
import { mkdtemp, rm } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

function notionMock() {
  const blocks = new Map([[NOTION_ARCHIVE_PAGE_ID, { id: NOTION_ARCHIVE_PAGE_ID, children: ['user-note'] }],
    ['user-note', { id: 'user-note', type: 'paragraph', paragraph: { rich_text: [{ text: { content: 'Keep this note' } }] }, children: [] }]]);
  let next = 0;
  const calls = [];
  let failure;
  function add(parent, value) {
    const id = `block-${++next}`;
    const block = structuredClone(value);
    const nested = block[block.type]?.children || [];
    if (block[block.type]) delete block[block.type].children;
    blocks.set(id, { ...block, id, children: [] });
    blocks.get(parent).children.push(id);
    for (const child of nested) add(id, child);
    return blocks.get(id);
  }
  return { blocks, calls, fail(predicate) { failure = predicate; },
    fetchImpl: async (url, options) => {
      const target = new URL(url);
      if (target.pathname === `/v1/pages/${NOTION_ARCHIVE_PAGE_ID}`) return new Response(JSON.stringify({ id: NOTION_ARCHIVE_PAGE_ID }));
      const match = target.pathname.match(/\/blocks\/([^/]+)(\/children)?$/);
      assert.ok(match, url);
      const [, id, isChildren] = match;
      const body = options.body ? JSON.parse(options.body) : null;
      calls.push({ id, method: options.method, body });
      if (failure?.({ id, method: options.method, body })) { failure = null; return new Response('{}', { status: 503 }); }
      let result;
      if (options.method === 'GET') {
        const offset = Number(target.searchParams.get('start_cursor') || 0);
        const children = blocks.get(id).children.filter(child => !blocks.get(child).archived);
        result = { results: children.slice(offset, offset + 50).map(child => blocks.get(child)),
          has_more: children.length > offset + 50, next_cursor: String(offset + 50) };
      } else if (options.method === 'DELETE') {
        blocks.get(id).archived = true; result = blocks.get(id);
      } else if (isChildren) {
        result = { results: body.children.map(child => add(id, child)) };
      } else {
        Object.assign(blocks.get(id), body); result = blocks.get(id);
      }
      return new Response(JSON.stringify(result));
    },
  };
}
function containers(mock) {
  return mock.blocks.get(NOTION_ARCHIVE_PAGE_ID).children.map(id => mock.blocks.get(id)).filter(block => !block.archived && block.type === 'toggle');
}
function tableRows(mock) {
  const container = containers(mock).find(block => block.toggle.rich_text[0].text.content.endsWith('[managed]'));
  const table = mock.blocks.get(container.children[0]);
  return table.children.map(id => mock.blocks.get(id).table_row.cells.map(cell => cell.map(part => part.text.content).join('')));
}

const machine = { id: 'm1', hostname: 'test-server', cpu_info: 'CPU', interface_id: 1, interface_name: 'eth0',
  ip_id: 1, ip_address: '10.0.0.1', subnet_mask: '255.255.255.0', memo: '  memo | <tag> `\n&  ' };

test('Notion cells reproduce unified Markdown content and preserve whitespace/escapes', () => {
  const rows = markdownCells(renderMarkdown([machine]));
  assert.equal(rows[1][rows[0].indexOf('memo')], machine.memo);
  assert.equal(rows[1][rows[0].indexOf('hostname')], 'test-server');
  assert.ok(!rows[0].includes('last_alive'));
});

test('Notion creates one native table, skips identical content, repairs remote drift and deletes machines', async () => {
  const mock = notionMock();
  const publish = createNotionPublisher({ token: 'test', fetchImpl: mock.fetchImpl, requestIntervalMs: 0 });
  const markdown = renderMarkdown([machine]);
  assert.deepEqual(await publish(markdown), { updated: true });
  assert.deepEqual(tableRows(mock), markdownCells(markdown));
  const writes = mock.calls.filter(call => call.method !== 'GET').length;
  assert.deepEqual(await publish(markdown), { updated: false });
  assert.equal(mock.calls.filter(call => call.method !== 'GET').length, writes);
  const table = mock.blocks.get(containers(mock)[0].children[0]);
  mock.blocks.get(table.children[1]).table_row.cells[1][0].text.content = 'manual edit';
  assert.deepEqual(await publish(markdown), { updated: true });
  assert.equal(containers(mock).length, 1);
  assert.deepEqual(tableRows(mock), markdownCells(markdown));
  await publish(renderMarkdown([]));
  assert.equal(tableRows(mock).length, 1);
  assert.equal(mock.blocks.get('user-note').archived, undefined);
});

test('Notion batches over 100 rows, paginates reads and splits long text without truncating', async () => {
  const mock = notionMock();
  const publish = createNotionPublisher({ token: 'test', fetchImpl: mock.fetchImpl, requestIntervalMs: 0 });
  const rows = Array.from({ length: 205 }, (_, index) => ({ ...machine, id: `m${index}`, memo: 'あ'.repeat(4500) }));
  const markdown = renderMarkdown(rows);
  await publish(markdown);
  assert.deepEqual(tableRows(mock), markdownCells(markdown));
  for (const call of mock.calls.filter(call => call.body?.children)) {
    assert.ok(call.body.children.length <= 100);
    assert.ok(Buffer.byteLength(JSON.stringify(call.body)) <= 450000);
  }
  assert.deepEqual(await publish(markdown), { updated: false });
});

test('Notion failed replacement preserves old snapshot; next attempt cleans staging and completes', async () => {
  const mock = notionMock();
  const publish = createNotionPublisher({ token: 'test', fetchImpl: mock.fetchImpl, requestIntervalMs: 0 });
  const old = renderMarkdown([machine]);
  await publish(old);
  mock.fail(call => call.body?.children?.[0]?.type === 'table_row');
  const changed = renderMarkdown([{ ...machine, hostname: 'changed' }]);
  await assert.rejects(publish(changed), /HTTP 503/);
  assert.deepEqual(tableRows(mock), markdownCells(old));
  await publish(changed);
  assert.equal(containers(mock).length, 1);
  assert.deepEqual(tableRows(mock), markdownCells(changed));
});

test('Notion 429 respects Retry-After without exposing secrets and invalid page IDs are rejected', async () => {
  const publish = createNotionPublisher({ token: 'SECRET', fetchImpl: async () => new Response('SECRET', { status: 429, headers: { 'retry-after': '120' } }), requestIntervalMs: 0 });
  await assert.rejects(publish(renderMarkdown([])), error => error.status === 429 && error.retryMs === 120000 && !error.message.includes('SECRET'));
  assert.throws(() => createNotionPublisher({ token: 'x', pageId: '../invalid' }), /Invalid/);
});

test('Notion can be enabled alone and requires its own flag and token', t => {
  const db = { query() { assert.fail('unexpected DB read'); } };
  assert.equal(archiveFromEnv(db, { NOTION_ARCHIVE_TOKEN: 'test' }), null);
  assert.equal(archiveFromEnv(db, { NOTION_ARCHIVE_ENABLED: 'true' }), null);
  const archive = archiveFromEnv(db, { NOTION_ARCHIVE_ENABLED: 'true', NOTION_ARCHIVE_TOKEN: 'test' });
  assert.ok(archive);
  t.after(() => archive.stop());
});

test('independent destination workers keep successful GitHub from repeating when Notion fails', async t => {
  const directory = await mkdtemp(path.join(os.tmpdir(), 'boops-notion-'));
  let github = 0, notion = 0;
  const options = { db: { async query() { return [[machine]]; } }, debounceMs: 100000, logger: { error() {} } };
  const a = createArchiveService({ ...options, directory: path.join(directory, 'github'), publish: async () => { github++; } });
  const b = createArchiveService({ ...options, name: 'Notion', directory: path.join(directory, 'notion'), publish: async () => { if (++notion === 1) throw new Error('offline'); } });
  const archive = combineArchives([a, b]);
  t.after(async () => { archive.stop(); await rm(directory, { recursive: true, force: true }); });
  await assert.rejects(archive.run({ throwOnError: true }), /notion-sync/);
  assert.equal(github, 1);
  await archive.run({ throwOnError: true });
  assert.equal(github, 1);
  assert.equal(notion, 2);
});


test('database URL creates/reuses a managed entry without changing its schema or other entries', async () => {
  const mock = notionMock();
  let entry, creates = 0;
  const fetchImpl = async (url, options) => {
    const route = new URL(url).pathname;
    if (route === `/v1/pages/${NOTION_ARCHIVE_PAGE_ID}`) return new Response('{}', { status: 404 });
    if (route === `/v1/databases/${NOTION_ARCHIVE_PAGE_ID}`) return new Response(JSON.stringify({ data_sources: [{ id: 'source' }] }));
    if (route === '/v1/data_sources/source') return new Response(JSON.stringify({ properties: { Name: { id: 'title', type: 'title' } } }));
    if (route === '/v1/data_sources/source/query') {
      assert.equal(JSON.parse(options.body).filter.title.equals, 'BoopsDB Archive [managed]');
      return new Response(JSON.stringify({ results: entry ? [{ id: entry }] : [], has_more: false }));
    }
    if (route === '/v1/pages') {
      creates++;
      assert.deepEqual(JSON.parse(options.body).parent, { type: 'data_source_id', data_source_id: 'source' });
      entry = 'archive-entry';
      mock.blocks.set(entry, { id: entry, children: [] });
      return new Response(JSON.stringify({ id: entry }));
    }
    return mock.fetchImpl(url, options);
  };
  const publish = createNotionPublisher({ token: 'test', fetchImpl, requestIntervalMs: 0 });
  await publish(renderMarkdown([machine]));
  assert.equal(creates, 1);
  assert.deepEqual(await publish(renderMarkdown([machine])), { updated: false });
  assert.equal(creates, 1);
  assert.equal(mock.blocks.get(NOTION_ARCHIVE_PAGE_ID).children.length, 1, 'existing database content left intact');
  assert.equal(mock.blocks.get(entry).children.length, 1);
});
