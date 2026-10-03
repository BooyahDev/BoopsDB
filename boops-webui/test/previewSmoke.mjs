import assert from 'node:assert/strict';

// Run after building and starting the production preview with the local fixture override.
const preview = 'http://127.0.0.1:4317';
for (const path of ['/', '/machines', '/machines/register', '/machines/11111111-1111-4111-8111-111111111111']) {
  for (const [cookie, expected] of [['', 'system'], ['boops-theme=dark', 'dark'], ['boops-theme=light', 'light'], ['boops-theme=invalid', 'system']]) {
    const response = await fetch(`${preview}${path}`, { headers: { cookie } });
    assert.equal(response.status, 200, path);
    const html = await response.text();
    assert.ok(html.includes(`data-theme="${expected}"`), `${path}: SSR fallback ${expected}`);
    assert.ok(html.includes('http://127.0.0.1:4318/api'), 'loopback runtime API override');
    assert.ok(!html.includes('https://boopsdb-api.booyah.dev/api'), 'no production API URL in runtime payload');
  }
}
console.log('Preview SSR passed: 4 routes × 4 cookie states; loopback runtime API override.');
