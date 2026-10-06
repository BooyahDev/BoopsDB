import dotenv from 'dotenv';
import { archiveFromEnv } from './service.js';

dotenv.config();
let archive;
try {
  // Validate configuration before importing the production database module.
  archive = archiveFromEnv({ query() { throw new Error('Database not initialized'); } });
  if (!archive) throw new Error('Archive disabled; check GITHUB_ARCHIVE_ENABLED and GITHUB_ARCHIVE_TOKEN');
  archive.stop();
  const { default: db } = await import('../models/db.js');
  archive = archiveFromEnv(db);
  await archive.run({ verify: true, throwOnError: true });
  archive.stop();
  process.exit(0);
} catch (error) {
  archive?.stop();
  console.error(error.message);
  process.exit(1);
}
