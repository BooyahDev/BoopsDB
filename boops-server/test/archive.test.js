import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { renderMarkdown, readMarkdown } from '../archive/markdown.js';
import { createGithubPublisher } from '../archive/github.js';
import { createArchiveService, archiveFromEnv } from '../archive/service.js';
import { createApp } from '../httpApp.js';

const row = { id: 'machine-1', hostname: 'archive-host', cpu_info: 'CPU', cpu_arch: 'arm64',
  memory_size: '16 GB', disk_info: 'SSD', model_info: 'model', os_name: 'Linux', is_virtual: 0,
  interface_id: 1, machine_id: 'machine-1', interface_name: 'eth0', mac_address: 'aa:bb:cc:dd:ee:ff',
  gateway: '10.0.0.1', dns_servers: '1.1.1.1', ip_id: 2, ip_address: '10.0.0.2', subnet_mask: '255.255.255.0', dns_register: 1 };

test('Markdown contains hardware, NIC and every IP, escapes cells, excludes volatile timestamps', async () => {
  const output = renderMarkdown([{ ...row, memo: '<b>|hello\nworld`', last_alive: 'SECRET_TIME', updated_at: 'SECRET_TIME' },
    { ...row, memo: '<b>|hello\nworld`', ip_id: 3, ip_address: '10.0.0.3' }, { id: 'machine-2', hostname: 'no-nic' }]);
  for (const value of ['CPU', 'arm64', '16 GB', 'SSD', 'Linux', 'eth0', 'aa:bb:cc:dd:ee:ff', '10.0.0.1', '1.1.1.1', '10.0.0.2', '10.0.0.3', 'no-nic']) assert.ok(output.includes(value));
  assert.ok(output.includes('&lt;b&gt;&#124;hello<br>world&#96;'));
  assert.ok(!output.includes('SECRET_TIME'));
  assert.equal(renderMarkdown([row]), renderMarkdown([{ ...row, last_alive: 'changed', updated_at: 'changed' }]));
  assert.ok(!renderMarkdown([]).includes('archive-host'));
  await readMarkdown({ async query(sql) {
    assert.ok(!/last_alive|updated_at|created_at/.test(sql));
    assert.match(sql, /LEFT JOIN/);
    return [[row]];
  } });
});

test('single table preserves machine, NIC and IP associations including empty children', () => {
  const output = renderMarkdown([row, { ...row, ip_id: 3, ip_address: '10.0.0.3' },
    { ...row, interface_id: 4, interface_name: 'eth1', ip_id: null, ip_address: null, subnet_mask: null, dns_register: null },
    { id: 'machine-2', hostname: 'no-nic' }]);
  const lines = output.split('\n').filter(line => line.startsWith('|'));
  assert.equal(lines.length, 6, 'one header, one separator and four rows');
  const headers = lines[0].split('|').slice(1, -1).map(cell => cell.trim());
  assert.equal(new Set(headers).size, headers.length);
  const records = lines.slice(2).map(line => Object.fromEntries(line.split('|').slice(1, -1).map((cell, index) => [headers[index], cell.trim()])));
  assert.deepEqual(records.map(record => [record.hostname, record.interface_name, record.ip_address]), [
    ['archive-host', 'eth0', '10.0.0.2'], ['archive-host', 'eth0', '10.0.0.3'],
    ['archive-host', 'eth1', ''], ['no-nic', '', ''],
  ]);
  for (const record of records.slice(0, 3)) assert.equal(record.cpu_info, 'CPU');
  assert.equal(renderMarkdown([]).split('\n').filter(line => line.startsWith('|')).length, 2);
});

test('GitHub initializes empty repo and updates with SHA; skips equal remote content', async () => {
  const calls = [];
  let existing = null;
  const publish = createGithubPublisher({ token: 'test-token', fetchImpl: async (url, options) => {
    calls.push({ url, ...options });
    if (options.method === 'GET') return new Response(existing ? JSON.stringify(existing) : '', { status: existing ? 200 : 404 });
    return new Response('{}', { status: 201 });
  } });
  await publish('first');
  let body = JSON.parse(calls[1].body);
  assert.equal(Buffer.from(body.content, 'base64').toString(), 'first');
  assert.equal(body.sha, undefined);
  existing = { sha: 'old-sha', encoding: 'base64', content: Buffer.from('first').toString('base64') };
  await publish('first');
  assert.equal(calls.length, 3);
  await publish('second');
  assert.equal(JSON.parse(calls.at(-1).body).sha, 'old-sha');
  assert.match(calls[0].url, /BooyahDev\/BoopsDB-Archive\/contents\/README.md$/);
});

test('GitHub errors honor rate limit reset and never include tokens in errors', async () => {
  const publish = createGithubPublisher({ token: 'SECRET', fetchImpl: async () => new Response('SECRET', {
    status: 403, headers: { 'retry-after': '120' },
  }) });
  await assert.rejects(publish('x'), error => error.retryMs >= 120000 && !error.message.includes('SECRET'));
});

async function service(t, options = {}) {
  const directory = await mkdtemp(path.join(os.tmpdir(), 'boops-archive-'));
  const sent = [];
  let rows = [row];
  const db = { async query() { return [rows]; } };
  const archive = createArchiveService({ directory, db, publish: async content => sent.push(content),
    debounceMs: 100000, retryMs: 100000, logger: { error() {} }, ...options });
  t.after(async () => { archive.stop(); await rm(directory, { recursive: true, force: true }); });
  return { archive, sent, directory, db, setRows(value) { rows = value; } };
}

test('service saves locally, skips identical updates and exports edits/deletions', async t => {
  const s = await service(t);
  await s.archive.run();
  assert.equal(await readFile(path.join(s.directory, 'README.md'), 'utf8'), s.sent[0]);
  await s.archive.run();
  assert.equal(s.sent.length, 1);
  s.setRows([{ ...row, cpu_info: 'new CPU' }]);
  await s.archive.run();
  assert.equal(s.sent.length, 2);
  s.setRows([]);
  await s.archive.run();
  assert.equal(s.sent.length, 3);
  assert.ok(!s.sent[2].includes('archive-host'));
});

test('failed push retries saved snapshot even when MySQL becomes unavailable', async t => {
  let fail = true;
  const sent = [];
  const s = await service(t, { publish: async content => { if (fail) throw new Error('offline'); sent.push(content); } });
  await s.archive.run();
  assert.ok((await readFile(path.join(s.directory, 'README.md'), 'utf8')).includes('archive-host'));
  s.db.query = async () => { throw new Error('DB offline'); };
  fail = false;
  await s.archive.run();
  assert.equal(sent.length, 1);
});

test('concurrent runs serialize publication and coalesce schedules', async t => {
  let release;
  let calls = 0;
  const s = await service(t, { publish: async () => { calls++; await new Promise(resolve => { release = resolve; }); } });
  const first = s.archive.run();
  while (!release) await new Promise(resolve => setImmediate(resolve));
  s.archive.schedule(); s.archive.schedule();
  await s.archive.run();
  assert.equal(calls, 1);
  release(); await first;
});

test('HTTP hook preserves responses, ignores heartbeat, GET and failed writes', async t => {
  let scheduled = 0;
  const db = { async query(sql) {
    if (sql.includes('ROW_COUNT')) return [[{ count: 1 }]];
    if (sql.startsWith('SELECT *')) return [[]];
    return [{ affectedRows: 1 }];
  } };
  const server = createApp(db, { archive: { schedule() { scheduled++; } } }).listen(0, '127.0.0.1');
  await new Promise(resolve => server.once('listening', resolve));
  t.after(() => new Promise(resolve => server.close(resolve)));
  const base = `http://127.0.0.1:${server.address().port}`;
  const id = '00000000-0000-4000-8000-000000000001';
  async function put(suffix, body) {
    return fetch(`${base}/api/machines/${id}/${suffix}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  }
  assert.deepEqual(await (await put('update-last-alive', {})).json(), { message: 'Last alive timestamp updated' });
  assert.equal((await put('UPDATE-LAST-ALIVE', {})).status, 200);
  await fetch(base + '/api/machines');
  assert.equal((await put('update-hostname', { hostname: '' })).status, 400);
  assert.equal(scheduled, 0);
  assert.deepEqual(await (await put('update-hostname', { hostname: 'new' })).json(), { message: 'Hostname updated' });
  assert.equal(scheduled, 1);
  db.query = async () => { throw new Error('offline'); };
  assert.equal((await put('update-hostname', { hostname: 'new' })).status, 500);
  assert.equal(scheduled, 1);
});


test('environment flag disables all archive activity unless explicitly true with a token', t => {
  const db = { query() { assert.fail('disabled archive queried DB'); } };
  for (const value of [undefined, '', 'false', '0', 'yes']) {
    assert.equal(archiveFromEnv(db, { GITHUB_ARCHIVE_ENABLED: value, GITHUB_ARCHIVE_TOKEN: 'test' }), null);
  }
  assert.equal(archiveFromEnv(db, { GITHUB_ARCHIVE_ENABLED: 'true' }), null);
  const archive = archiveFromEnv(db, { GITHUB_ARCHIVE_ENABLED: 'true', GITHUB_ARCHIVE_TOKEN: 'test' });
  assert.ok(archive);
  t.after(() => archive.stop());
});

test('daily timer verifies remote even when DB content has not changed, and stop cancels it', async t => {
  let remote = '';
  let reads = 0, writes = 0, checked;
  const verification = new Promise(resolve => { checked = resolve; });
  const publish = createGithubPublisher({ token: 'test', fetchImpl: async (url, options) => {
    if (options.method === 'GET') {
      reads++;
      if (reads === 3) checked();
      return new Response(JSON.stringify({ sha: 'sha', encoding: 'base64', content: Buffer.from(remote).toString('base64') }));
    }
    writes++;
    remote = Buffer.from(JSON.parse(options.body).content, 'base64').toString();
    return new Response('{}', { status: 200 });
  } });
  const s = await service(t, { publish, intervalMs: 30, debounceMs: 1 });
  await s.archive.run();
  assert.equal(writes, 1);
  remote = 'remote drift';
  await verification;
  s.archive.stop();
  assert.equal(writes, 2, 'remote drift repaired; subsequent identical remote did not commit');
  assert.equal(remote, renderMarkdown([row]));
  await new Promise(resolve => setTimeout(resolve, 60));
  assert.equal(reads, 3);
});

test('failed daily verification retains force flag across retries for unchanged DB', async t => {
  let calls = 0;
  const s = await service(t, { publish: async () => { if (++calls === 2) throw new Error('GitHub offline'); } });
  await s.archive.run();
  s.archive.schedule({ verify: true });
  await s.archive.run();
  assert.equal(calls, 2);
  await s.archive.run();
  assert.equal(calls, 3);
});
