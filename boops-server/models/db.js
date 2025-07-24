import mysql from 'mysql2/promise';
import dotenv from 'dotenv';
dotenv.config();

// Check if environment variables are set
if (!process.env.DB_HOST || !process.env.DB_USER || !process.env.DB_PASS || !process.env.DB_NAME || !process.env.DB_PORT) {
  console.error('Database connection error: Missing required environment variables');
  process.exit(1);
}

const pool = mysql.createPool({
  host: process.env.DB_HOST,
  user: process.env.DB_USER,
  password: process.env.DB_PASS,
  database: process.env.DB_NAME,
  port: Number(process.env.DB_PORT),
  waitForConnections: true,
  connectionLimit: 10
});

// Add error event listener to the pool
pool.on('error', (err) => {
  console.error('Database pool error:', err.message);
});

export default pool;
