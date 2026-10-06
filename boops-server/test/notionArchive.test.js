import test from 'node:test';
import assert from 'node:assert/strict';
import { renderMarkdown } from '../archive/markdown.js';
import { createNotionPublisher, markdownCells, NOTION_ARCHIVE_PAGE_ID, NOTION_ARCHIVE_KEY } from '../archive/notion.js';
import { archiveFromEnv, combineArchives, createArchiveService } from '../archive/service.js';
import { mkdtemp, rm } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
const text = parts => (parts || []).map(part => part.text?.content || '').join('');
const rt = content => [{ type: 'text', text: { content } }];
const machine = { id: 'm1', hostname: 'test-server', cpu_info: 'CPU', interface_id: 1, interface_name: 'eth0',
  ip_id: 1, ip_address: '10.0.0.1', subnet_mask: '255.255.255.0', is_virtual: 1, dns_register: 0, memo: '  memo | <tag> `\n&  ' };

function notionMock() {
  const schema = { 名前: { id: 'title', name: '名前', type: 'title', title: {} } };
  const pages = new Map();
  const calls = [];
  let count = 0, failure;
  function save(properties) {
    return Object.fromEntries(Object.entries(schema).map(([name, p]) => {
      const value = properties[p.id] || properties[name];
      return [name, { id: p.id, type: p.type, [p.type]: value?.[p.type] ?? (p.type === 'checkbox' ? false : p.type === 'number' ? null : []) }];
    }));
  }
  return { schema, pages, calls, fail(predicate) { failure = predicate; }, fetchImpl: async (url, options) => {
    const route = new URL(url).pathname.slice('/v1/'.length);
    const body = options.body ? JSON.parse(options.body) : null;
    calls.push({ route, method: options.method, body });
    if (failure?.({ route, method: options.method, body })) { failure = null; return new Response('{}', { status: 503 }); }
    let result;
    if (route === `databases/${NOTION_ARCHIVE_PAGE_ID}`) result = { data_sources: [{ id: 'source' }] };
    else if (route === 'data_sources/source' && options.method === 'GET') result = { properties: schema };
    else if (route === 'data_sources/source' && options.method === 'PATCH') {
      for (const [name, p] of Object.entries(body.properties)) {
        const type = Object.keys(p)[0]; schema[name] = { id: `p${++count}`, name, type, [type]: {} };
      }
      result = { properties: schema };
    } else if (route === 'data_sources/source/query') {
      const property = Object.values(schema).find(p => p.id === body.filter.property);
      const filter = body.filter[property.type];
      const found = [...pages.values()].filter(p => !p.in_trash && (filter.starts_with
        ? text(p.properties[property.name]?.[property.type]).startsWith(filter.starts_with)
        : text(p.properties[property.name]?.[property.type]) === filter.equals));
      const offset = Number(body.start_cursor || 0);
      result = { results: found.slice(offset, offset + 50), has_more: found.length > offset + 50, next_cursor: String(offset + 50) };
    } else if (route === 'pages' && options.method === 'POST') {
      assert.equal(body.parent.data_source_id, 'source');
      assert.equal(body.children, undefined, 'database records have no body tables');
      const id = `page${++count}`; result = { id, properties: save(body.properties) }; pages.set(id, result);
    } else if (/^pages\/[^/]+$/.test(route) && options.method === 'PATCH') {
      result = pages.get(route.split('/')[1]);
      if (body.properties) Object.assign(result.properties, save(body.properties));
      if (body.in_trash) result.in_trash = true;
    } else if (/^pages\/[^/]+\/properties\//.test(route)) {
      const [, id, , propertyId] = route.split('/');
      const property = Object.values(schema).find(p => p.id === decodeURIComponent(propertyId));
      const parts = pages.get(id).properties[property.name][property.type];
      result = { object: 'list', results: parts.map(part => ({ [property.type]: part })), has_more: false };
    } else if (/^blocks\/[^/]+\/children$/.test(route)) {
      assert.equal(options.method, 'GET');
      result = { results: [{ type: 'toggle', toggle: { rich_text: rt('BoopsDB Archive [managed]') } }] };
    } else assert.fail(`Unexpected ${options.method} ${route}`);
    return new Response(JSON.stringify(result));
  } };
}
function managed(mock) { return [...mock.pages.values()].filter(p => !p.in_trash && text(p.properties[NOTION_ARCHIVE_KEY]?.rich_text).startsWith('boopsdb:')); }
function publisher(mock) { return createNotionPublisher({ token: 'test', fetchImpl: mock.fetchImpl, requestIntervalMs: 0 }); }

test('Notion values reproduce unified Markdown including whitespace and escapes', () => {
  const [headers, row] = markdownCells(renderMarkdown([machine]));
  assert.equal(row[headers.indexOf('memo')], machine.memo);
  assert.ok(!headers.includes('last_alive'));
});

test('direct database sync adds columns and upserts records, skips equal content, repairs drift and removes deleted rows', async () => {
  const mock = notionMock(), publish = publisher(mock);
  mock.pages.set('user-note', { id: 'user-note', properties: { 名前: { title: rt('Keep me') } } });
  const rows = [machine, { ...machine, ip_id: 2, ip_address: '10.0.0.2' }, { id: 'no-nic', hostname: 'no-nic' }];
  assert.deepEqual(await publish(renderMarkdown(rows)), { updated: true });
  assert.equal(managed(mock).length, 3);
  assert.equal(mock.schema.名前.type, 'title');
  assert.equal(mock.schema.cpu_info.type, 'rich_text');
  assert.equal(mock.schema.is_virtual.type, 'checkbox');
  assert.equal(mock.schema.interface_id.type, 'number');
  const first = managed(mock)[0];
  assert.equal(text(first.properties.名前.title), machine.hostname);
  assert.equal(text(first.properties.memo.rich_text), machine.memo);
  assert.equal(first.properties.is_virtual.checkbox, true);
  assert.equal(first.properties.dns_register.checkbox, false);
  assert.deepEqual(await publish(renderMarkdown(rows)), { updated: false });
  first.properties.cpu_info.rich_text = rt('manual edit');
  assert.deepEqual(await publish(renderMarkdown(rows)), { updated: true });
  assert.equal(text(first.properties.cpu_info.rich_text), machine.cpu_info);
  await publish(renderMarkdown([{ ...machine, hostname: 'renamed' }]));
  assert.equal(managed(mock).length, 1);
  assert.equal(managed(mock)[0].id, first.id, 'existing row identity preserved');
  assert.equal(text(first.properties.名前.title), 'renamed');
  await publish(renderMarkdown([]));
  assert.equal(managed(mock).length, 0);
  assert.equal(mock.pages.get('user-note').in_trash, undefined);
});

test('database reads paginate and long text is retained without truncation', async () => {
  const mock = notionMock(), publish = publisher(mock);
  const rows = Array.from({ length: 105 }, (_, index) => ({ ...machine, id: `m${index}`, memo: index === 0 ? 'あ'.repeat(51000) : '' }));
  await publish(renderMarkdown(rows));
  assert.equal(managed(mock).length, 105);
  assert.equal(text(managed(mock)[0].properties.memo.rich_text).length, 51000);
  assert.deepEqual(await publish(renderMarkdown(rows)), { updated: false });
  assert.ok(mock.calls.some(call => call.route.includes('/properties/')));
});

test('failed upsert leaves obsolete rows intact; retry does not duplicate prior creates', async () => {
  const mock = notionMock(), publish = publisher(mock);
  await publish(renderMarkdown([machine]));
  mock.fail(call => call.route === 'pages' && call.method === 'POST');
  await assert.rejects(publish(renderMarkdown([{ ...machine, id: 'new' }])), /503/);
  assert.equal(managed(mock).length, 1);
  assert.equal(text(managed(mock)[0].properties.id.rich_text), 'm1');
  await publish(renderMarkdown([{ ...machine, id: 'new' }]));
  assert.equal(managed(mock).length, 1);
  assert.equal(text(managed(mock)[0].properties.id.rich_text), 'new');
});

test('legacy generated body-table entry is archived only after records are synchronized', async () => {
  const mock = notionMock(), publish = publisher(mock);
  mock.pages.set('legacy', { id: 'legacy', properties: { 名前: { title: rt('BoopsDB Archive [managed]') } } });
  mock.fail(call => call.route === 'pages');
  await assert.rejects(publish(renderMarkdown([machine])));
  assert.equal(mock.pages.get('legacy').in_trash, undefined);
  await publish(renderMarkdown([machine]));
  assert.equal(mock.pages.get('legacy').in_trash, true);
  assert.equal(managed(mock).length, 1);
});

test('schema type conflicts fail without changing existing columns or records', async () => {
  const mock = notionMock();
  mock.schema.cpu_info = { id: 'cpu', type: 'number', name: 'cpu_info' };
  await assert.rejects(publisher(mock)(renderMarkdown([machine])), error => error.code === 'NOTION_PROPERTY_TYPE_MISMATCH');
  assert.ok(!mock.calls.some(call => call.method === 'PATCH' || call.route === 'pages'));
});

test('Notion 429 honors Retry-After and does not expose credentials', async () => {
  const publish = createNotionPublisher({ token: 'SECRET', requestIntervalMs: 0, fetchImpl: async () => new Response('SECRET', { status: 429, headers: { 'retry-after': '120' } }) });
  await assert.rejects(publish(renderMarkdown([])), error => error.retryMs === 120000 && !error.message.includes('SECRET'));
  assert.throws(() => createNotionPublisher({ token: 'x', pageId: '../invalid' }));
});

test('Notion can be enabled independently with its own flag and token', t => {
  const db = { query() { assert.fail(); } };
  assert.equal(archiveFromEnv(db, { NOTION_ARCHIVE_TOKEN: 'test' }), null);
  assert.equal(archiveFromEnv(db, { NOTION_ARCHIVE_ENABLED: 'true' }), null);
  const archive = archiveFromEnv(db, { NOTION_ARCHIVE_ENABLED: 'true', NOTION_ARCHIVE_TOKEN: 'test' });
  assert.ok(archive); t.after(() => archive.stop());
});

test('independent destinations keep GitHub synchronized while Notion retries', async t => {
  const directory = await mkdtemp(path.join(os.tmpdir(), 'boops-notion-'));
  let github = 0, notion = 0;
  const options = { db: { async query() { return [[machine]]; } }, debounceMs: 100000, logger: { error() {} } };
  const archive = combineArchives([
    createArchiveService({ ...options, directory: path.join(directory, 'github'), publish: async () => { github++; } }),
    createArchiveService({ ...options, name: 'Notion', directory: path.join(directory, 'notion'), publish: async () => { if (++notion === 1) throw new Error('offline'); } }),
  ]);
  t.after(async () => { archive.stop(); await rm(directory, { recursive: true, force: true }); });
  await assert.rejects(archive.run({ throwOnError: true }), /notion-sync/);
  await archive.run({ throwOnError: true });
  assert.equal(github, 1); assert.equal(notion, 2);
});
