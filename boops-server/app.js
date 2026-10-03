import db from './models/db.js';
import { createApp } from './httpApp.js';

const app = createApp(db);

// Global error handler for uncaught exceptions
process.on('uncaughtException', (err) => {
  console.error(`Uncaught Exception: ${err.message}`);
  console.error(err.stack);
  process.exit(1); // Exit with failure code
});

// Global error handler for unhandled promise rejections
process.on('unhandledRejection', (reason, promise) => {
  console.error(`Unhandled Rejection at: ${promise}`);
  console.error(`Reason: ${reason}`);
  process.exit(1); // Exit with failure code
});

const port = 3001;

// Wrap app.listen in a try/catch to handle any startup errors
try {
  app.listen(port, '0.0.0.0', () => {
    console.log(`API server running on http://0.0.0.0:${port}`);
    console.log('Available endpoints:');
    console.log('  GET  /api/machines - 全マシン一覧');
    console.log('  GET  /api/machines/search?q=<query> - マシン検索');
    console.log('  GET  /api/debug/search-patterns - 検索パターンテスト');
    console.log('');
    console.log('検索例:');
    console.log('  /api/machines/search?q=192.168.1 - 部分IPアドレス検索');
    console.log('  /api/machines/search?q=192.168.1.0/24 - CIDR検索');
    console.log('  /api/machines/search?q=192.168.1.100 - 完全IPアドレス検索');
  });
} catch (err) {
  console.error('Failed to start server:', err);
}
