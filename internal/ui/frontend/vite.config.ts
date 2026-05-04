import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    // Proxy /api/* to the Go backend during `npm run dev`. Production builds
    // are served by Go directly via embed.FS, so no proxy is needed.
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:3939',
        changeOrigin: false,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
