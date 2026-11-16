import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// `npm run dev` proxies /api to the Go server (go run ./cmd/server in backend/).
export default defineConfig({
  plugins: [react()],
  server: { port: 5173, proxy: { '/api': { target: 'http://localhost:8080', changeOrigin: true } } },
  build: { outDir: '../backend/web/dist', emptyOutDir: true, chunkSizeWarningLimit: 1200 },
})
