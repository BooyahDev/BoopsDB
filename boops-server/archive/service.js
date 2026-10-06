import { mkdir, readFile, writeFile, rename } from 'node:fs/promises';
import path from 'node:path';
import { readMarkdown } from './markdown.js';
import { createGithubPublisher } from './github.js';
import { createNotionPublisher, NOTION_ARCHIVE_PAGE_ID } from './notion.js';

export function archiveMiddleware(archive) {
  return (req, res, next) => {
    if (['POST', 'PUT', 'DELETE'].includes(req.method)
      && /^\/api\/(machines|interfaces)(\/|$)/i.test(req.path)
      && !/\/update-last-alive\/?$/i.test(req.path)) {
      res.once('finish', () => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          // Backup failures must never affect the API response or process.
          try { archive.schedule(); } catch { console.error('Failed to schedule archive'); }
        }
      });
    }
    next();
  };
}

export function createArchiveService({ db, publish, directory, debounceMs = 2000, retryMs = 60000, intervalMs = 24 * 60 * 60 * 1000, logger = console, name = 'GitHub' }) {
  const file = path.join(directory, 'README.md');
  let timer, running = false, dirty = false, stopped = false, lastPublished, failures = 0, verifyRemote = false;
  function arm(delay) {
    if (stopped || timer) return;
    timer = setTimeout(() => { timer = undefined; void run(); }, delay);
    timer.unref?.();
  }
  function schedule({ verify = false } = {}) {
    dirty = true;
    verifyRemote ||= verify;
    arm(debounceMs);
  }
  // Queue verification through the same worker, preserving serialization and retry backoff.
  const dailyTimer = setInterval(() => schedule({ verify: true }), intervalMs);
  dailyTimer.unref?.();
  async function run({ verify: forceVerify = false, throwOnError = false } = {}) {
    if (running || stopped) return;
    running = true;
    dirty = false;
    const verify = forceVerify || verifyRemote;
    verifyRemote = false;
    let dbUnavailable = false;
    let stage = 'database-read';
    try {
      let markdown;
      try { markdown = await readMarkdown(db); }
      catch {
        logger.warn?.(`${name} archive: MySQL unavailable; using local snapshot`);
        dbUnavailable = true;
        stage = 'local-snapshot-read';
        markdown = await readFile(file, 'utf8');
      }
      if (!dbUnavailable) {
        stage = 'local-snapshot-write';
        await mkdir(directory, { recursive: true, mode: 0o700 });
        await writeFile(file + '.tmp', markdown, { mode: 0o600 });
        await rename(file + '.tmp', file);
      }
      if (verify || markdown !== lastPublished) {
        stage = `${name.toLowerCase()}-sync`;
        const result = await publish(markdown);
        lastPublished = markdown;
        logger.info?.(`${name} archive sync completed`, { updated: result?.updated ?? true, source: dbUnavailable ? 'local' : 'database' });
      }
      failures = 0;
      if (dbUnavailable) { dirty = true; verifyRemote ||= verify; arm(retryMs); }
    } catch (error) {
      dirty = true;
      verifyRemote ||= verify;
      failures++;
      // Avoid echoing arbitrary errors that could contain credentials or machine data.
      logger.error(`${name} archive failed; retry scheduled`, { attempt: failures, stage, ...(error.status ? { httpStatus: error.status } : {}), ...(error.code && /^[A-Z0-9_]+$/.test(error.code) ? { code: error.code } : {}) });
      clearTimeout(timer); timer = undefined;
      arm(Math.max(Math.min(retryMs * 2 ** Math.min(failures - 1, 6), 3600000), error.retryMs || 0));
      if (throwOnError) throw new Error(`Archive synchronization failed at ${stage}${error.status ? ` (HTTP ${error.status})` : ''}`);
    } finally {
      running = false;
      if (dirty) arm(debounceMs);
    }
  }
  return { schedule, run, stop() { stopped = true; clearInterval(dailyTimer); clearTimeout(timer); timer = undefined; } };
}

// Each destination has its own worker, snapshot directory and retry state.
// A Notion outage must not block GitHub (or vice versa).
export function archiveFromEnv(db, env = process.env) {
  const archives = [];
  for (const name of ['GitHub', 'Notion']) {
    const prefix = name === 'GitHub' ? 'GITHUB' : 'NOTION';
    if (env[`${prefix}_ARCHIVE_ENABLED`]?.trim().toLowerCase() !== 'true') {
      console.info(`${name} archive disabled: set ${prefix}_ARCHIVE_ENABLED=true to enable`);
      continue;
    }
    const token = env[`${prefix}_ARCHIVE_TOKEN`];
    if (!token) { console.error(`${name} archive disabled: ${prefix}_ARCHIVE_TOKEN is missing`); continue; }
    try {
      const publish = name === 'GitHub'
        ? createGithubPublisher({ token, repository: env.GITHUB_ARCHIVE_REPOSITORY || 'BooyahDev/BoopsDB-Archive', branch: env.GITHUB_ARCHIVE_BRANCH || '' })
        : createNotionPublisher({ token, pageId: env.NOTION_ARCHIVE_PAGE_ID || NOTION_ARCHIVE_PAGE_ID });
      archives.push(createArchiveService({ db, publish, name,
        directory: env[`${prefix}_ARCHIVE_DIR`] || path.resolve(name === 'GitHub' ? '.boops-archive' : '.boops-archive/notion'),
      }));
      console.info(`${name} archive enabled: startup sync and verification every 24 hours`);
    } catch { console.error(`Invalid ${name} archive configuration; archive disabled`); }
  }
  return archives.length ? combineArchives(archives) : null;
}

export function combineArchives(archives) {
  return {
    schedule(options) { for (const archive of archives) archive.schedule(options); },
    async run(options) {
      const results = await Promise.allSettled(archives.map(archive => archive.run(options)));
      const errors = results.filter(result => result.status === 'rejected').map(result => result.reason);
      if (errors.length) throw new AggregateError(errors, errors.map(error => error.message).join('; '));
    },
    stop() { for (const archive of archives) archive.stop(); },
  };
}
