import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: '/',
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    emptyOutDir: true,
  },
  server: {
    port: 3000,
    proxy: {
      '/ws': {
        target: 'ws://localhost:80',
        ws: true,
      },
      '/questions': 'http://localhost:80',
      '/backup': 'http://localhost:80',
      '/config.json': 'http://localhost:80',
      '/files': 'http://localhost:80',
    }
  }
})
