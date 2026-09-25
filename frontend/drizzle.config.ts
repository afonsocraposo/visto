import { config } from 'dotenv';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'drizzle-kit';

const projectRoot = fileURLToPath(new URL('..', import.meta.url));
config({ path: resolve(projectRoot, '.env') });

const databasePath = process.env.VISTO_DATABASE_PATH || 'data/visto.db';

export default defineConfig({
  dialect: 'sqlite',
  dbCredentials: {
    url: resolve(projectRoot, databasePath),
  },
});
