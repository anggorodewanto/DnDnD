import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  base: '/portal/app/',
  build: {
    outDir: '../../internal/portal/assets',
    emptyOutDir: true,
  },
  test: {
    environment: 'node',
  },
});
