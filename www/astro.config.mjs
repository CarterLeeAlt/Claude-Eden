import { defineConfig } from 'astro/config';
export default defineConfig({
  site: process.env.SITE_URL || undefined,
  base: process.env.SITE_BASE || '/',
  trailingSlash: 'always',
  server: { port: 4822 },
  build: { inlineStylesheets: 'auto' },
});
