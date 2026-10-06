import { mkdir, readFile, writeFile, rename } from 'node:fs/promises';
import path from 'node:path';
import { readMarkdown } from './markdown.js';
import { createGithubPublisher } from './github.js';

export function archiveMiddleware(archive) {
  return (req, res, next) => {
    if (['POST', 'PUT', 'DELETE'].includes(req.method)
      && /^\/api\/(machines|interfaces)(\/|$)/i.test(req.path)
      && !/\/update-last-alive\/?$/i.test(req.path)) {
      res.once('finish', () => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          // Backup failures must never affect the API response or process.
          try { archive.schedule(); } catch { console.error('Failed to schedule GitHub archive'); }
        }
      });
    }
    next();
  };
}

export function createArchiveService({ db, publish, directory, debounceMs = 2000, retryMs = 60000, intervalMs = 24 * 60 * 60 * 1000, logger = console }) {
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
  async function run() {
    if (running || stopped) return;
    running = true;
    dirty = false;
    const verify = verifyRemote;
    verifyRemote = false;
    let dbUnavailable = false;
    try {
      let markdown;
      try { markdown = await readMarkdown(db); }
      catch {
        dbUnavailable = true;
        markdown = await readFile(file, 'utf8');
      }
      if (!dbUnavailable) {
        await mkdir(directory, { recursive: true, mode: 0o700 });
        await writeFile(file + '.tmp', markdown, { mode: 0o600 });
        await rename(file + '.tmp', file);
      }
      if (verify || markdown !== lastPublished) { await publish(markdown); lastPublished = markdown; }
      failures = 0;
      if (dbUnavailable) { dirty = true; verifyRemote ||= verify; arm(retryMs); }
    } catch (error) {
      dirty = true;
      verifyRemote ||= verify;
      failures++;
      // Avoid echoing arbitrary errors that could contain credentials or machine data.
      logger.error('GitHub archive failed; retry scheduled', { attempt: failures });
      clearTimeout(timer); timer = undefined;
      arm(Math.max(Math.min(retryMs * 2 ** Math.min(failures - 1, 6), 3600000), error.retryMs || 0));
    } finally {
      running = false;
      if (dirty) arm(debounceMs);
    }
  }
  return { schedule, run, stop() { stopped = true; clearInterval(dailyTimer); clearTimeout(timer); timer = undefined; } };
}

export function archiveFromEnv(db, env = process.env) {
  if (env.GITHUB_ARCHIVE_ENABLED?.trim().toLowerCase() !== 'true' || !env.GITHUB_ARCHIVE_TOKEN) return null;
  try {
    return createArchiveService({ db,
    publish: createGithubPublisher({ token: env.GITHUB_ARCHIVE_TOKEN,
      repository: env.GITHUB_ARCHIVE_REPOSITORY || 'BooyahDev/BoopsDB-Archive', branch: env.GITHUB_ARCHIVE_BRANCH || '' }),
      directory: env.GITHUB_ARCHIVE_DIR || path.resolve('.boops-archive'),
    });
  } catch {
    console.error('Invalid GitHub archive configuration; archive disabled');
    return null;
  }
}
