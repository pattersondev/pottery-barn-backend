import pkg from 'pg';
const { Pool } = pkg;
import dotenv from 'dotenv';

dotenv.config();

const pool = new Pool({
  host: process.env.DB_HOST || 'dpg-d4ficjqli9vc73ag07qg-a.ohio-postgres.render.com',
  port: process.env.DB_PORT || 5432,
  database: process.env.DB_NAME || 'potterybarn',
  user: process.env.DB_USER || 'potterybarn_user',
  password: process.env.DB_PASSWORD || '07qYYLJ81qN8uumJIj8p0fGxZ4P59a1w',
  ssl: {
    rejectUnauthorized: false
  }
});

// Test connection
pool.on('connect', () => {
  console.log('Connected to PostgreSQL database');
});

pool.on('error', (err) => {
  console.error('Unexpected error on idle client', err);
  process.exit(-1);
});

export default pool;

