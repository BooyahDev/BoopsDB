import test from 'node:test';
import assert from 'node:assert/strict';
import { normalizeThemePreference, resolveTheme } from '../utils/themePreference.js';
import { createIpEditRows, toIpPayload } from '../utils/interfaceRows.js';

test('system follows OS while explicit theme ignores OS', () => {
  assert.equal(resolveTheme('system', true), 'dark');
  assert.equal(resolveTheme('system', false), 'light');
  assert.equal(resolveTheme('light', true), 'light');
  assert.equal(resolveTheme('dark', false), 'dark');
});

test('invalid or absent cookie resolves to system', () => {
  for (const value of [undefined, null, '', 'unknown', 'Dark']) {
    assert.equal(normalizeThemePreference(value), 'system');
    assert.equal(resolveTheme(value, true), 'dark');
  }
});

test('saved IP keeps database ID and edit does not mutate source', () => {
  const source = [{ id: 42, ip_address: '10.0.0.2', subnet_mask: '255.255.255.0', dns_register: 1 }];
  const rows = createIpEditRows(source, () => 'unused');
  rows[0].ip_address = '10.0.0.3';
  assert.equal(source[0].ip_address, '10.0.0.2');
  assert.equal(rows[0].rowId, 'saved-42');
  assert.deepEqual(toIpPayload(rows), [{ id: 42, ip_address: '10.0.0.3', subnet_mask: '255.255.255.0', dns_register: true }]);
});

test('draft row IDs survive removal and are never sent to API', () => {
  let sequence = 0;
  const nextId = () => `draft-${++sequence}`;
  const rows = createIpEditRows([{ ip_address: '10.0.0.2' }, { ip_address: '10.0.0.3' }], nextId);
  const secondKey = rows[1].rowId;
  rows.splice(0, 1);
  rows.push(...createIpEditRows([{ ip_address: '10.0.0.4' }], nextId));
  assert.equal(rows[0].rowId, secondKey);
  assert.notEqual(rows[0].rowId, rows[1].rowId);
  assert.equal('id' in toIpPayload(rows)[0], false);
  assert.equal('rowId' in toIpPayload(rows)[0], false);
  assert.equal(createIpEditRows([], nextId).length, 1);
});
