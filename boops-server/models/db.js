import mysql from 'mysql2/promise';
import dotenv from 'dotenv';
dotenv.config();

// Check if environment variables are set
if (!process.env.DB_HOST || !process.env.DB_USER || !process.env.DB_PASS || !process.env.DB_NAME || !process.env.DB_PORT) {
  console.error('Database connection error: Missing required environment variables');
  process.exit(1);
}

const poolConfig = {
  host: process.env.DB_HOST,
  user: process.env.DB_USER,
  password: process.env.DB_PASS,
  database: process.env.DB_NAME,
  port: Number(process.env.DB_PORT),
  waitForConnections: true,
  connectionLimit: 10,
};

// Errors that indicate the connection target may have changed (e.g. DNS IP change)
const RECONNECT_ERRORS = new Set([
  'ECONNREFUSED',
  'ENOTFOUND',
  'ETIMEDOUT',
  'ECONNRESET',
  'PROTOCOL_CONNECTION_LOST',
  'PROTOCOL_ENQUEUE_AFTER_FATAL_ERROR',
]);

function createPool() {
  const p = mysql.createPool(poolConfig);
  p.on('error', (err) => {
    console.error('Database pool error:', err.message);
  });
  return p;
}

let pool = createPool();
let recreating = false;

async function recreatePool() {
  if (recreating) return;
  recreating = true;
  console.warn('Recreating database connection pool due to connection error...');
  const oldPool = pool;
  pool = createPool();
  recreating = false;
  oldPool.end().catch(() => {});
}

const db = {
  async query(...args) {
    try {
      return await pool.query(...args);
    } catch (err) {
      if (RECONNECT_ERRORS.has(err.code)) {
        await recreatePool();
        return await pool.query(...args);
      }
      throw err;
    }
  },
  async getConnection() {
    try {
      return await pool.getConnection();
    } catch (err) {
      if (RECONNECT_ERRORS.has(err.code)) {
        await recreatePool();
        return await pool.getConnection();
      }
      throw err;
    }
  },
};

export default db;
