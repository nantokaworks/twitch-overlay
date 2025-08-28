import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// 環境変数からポートを取得
const BACKEND_PORT = process.env.VITE_BACKEND_PORT || '8080';
const FRONTEND_PORT = process.env.VITE_FRONTEND_PORT ? parseInt(process.env.VITE_FRONTEND_PORT) : 5174;

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../dist/public',
    emptyOutDir: true,
  },
  server: {
    port: FRONTEND_PORT,
    proxy: {
      '/api': {
        target: `http://localhost:${BACKEND_PORT}`,
        changeOrigin: true,
      },
      '/events': `http://localhost:${BACKEND_PORT}`,
      '/fax': `http://localhost:${BACKEND_PORT}`,
      '/status': `http://localhost:${BACKEND_PORT}`,
      '/debug': {
        target: `http://localhost:${BACKEND_PORT}`,
        changeOrigin: true,
      },
    }
  }
});