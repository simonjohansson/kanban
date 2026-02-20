import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

const backendURL = process.env.KANBAN_BACKEND_PROXY_URL ?? 'http://127.0.0.1:18080';
const outDir = process.env.KANBAN_WEB_OUTDIR ?? 'dist';

export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir,
    emptyOutDir: true,
  },
  server: {
    host: '127.0.0.1',
    port: 4173,
    proxy: {
      '/projects': backendURL,
      '/ws': {
        target: backendURL,
        ws: true,
      },
      '/health': backendURL,
      '/client-config': backendURL,
      '/admin': backendURL,
      '/openapi': backendURL,
      '/schemas': backendURL,
    },
  },
});
