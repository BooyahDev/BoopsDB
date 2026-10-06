export function createGithubPublisher({ token, repository = 'BooyahDev/BoopsDB-Archive', branch = '', fetchImpl = fetch }) {
  if (!/^[\w.-]+\/[\w.-]+$/.test(repository)) throw new Error('Invalid GITHUB_ARCHIVE_REPOSITORY');
  const url = `https://api.github.com/repos/${repository}/contents/README.md`;
  async function request(method, body) {
    const response = await fetchImpl(url + (method === 'GET' && branch ? `?ref=${encodeURIComponent(branch)}` : ''), {
      method,
      headers: { Authorization: `Bearer ${token}`, Accept: 'application/vnd.github+json',
        'X-GitHub-Api-Version': '2022-11-28', 'Content-Type': 'application/json' },
      ...(body ? { body: JSON.stringify(body) } : {}),
      signal: AbortSignal.timeout(30000),
    });
    if (method === 'GET' && response.status === 404) return null;
    if (!response.ok) {
      // Do not log response bodies or request headers containing credentials/data.
      const error = new Error(`GitHub archive ${method} failed (HTTP ${response.status})`);
      error.status = response.status;
      const retryAfter = Number(response.headers.get('retry-after'));
      const reset = Number(response.headers.get('x-ratelimit-reset'));
      error.retryMs = Math.max(0, retryAfter * 1000 || 0,
        response.headers.get('x-ratelimit-remaining') === '0' ? reset * 1000 - Date.now() : 0);
      throw error;
    }
    return response.json();
  }
  return async markdown => {
    const existing = await request('GET');
    if (existing?.encoding === 'base64' && Buffer.from(existing.content, 'base64').toString('utf8') === markdown) return { updated: false };
    await request('PUT', {
      message: 'Update BoopsDB configuration archive', content: Buffer.from(markdown).toString('base64'),
      ...(existing ? { sha: existing.sha } : {}), ...(branch ? { branch } : {}),
    });
    return { updated: true };
  };
}
