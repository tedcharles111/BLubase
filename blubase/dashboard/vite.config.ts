import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3008,
    proxy: {
      '/api/auth': 'http://localhost:3001',
      '/api/projects': 'http://localhost:3002',
      '/api/sql': 'http://localhost:3007',
      '/api/storage': 'http://localhost:3004',
      '/api/ai': 'http://localhost:3006',
      '/api/edge': 'http://localhost:3005',
      '/ws': {
        target: 'ws://localhost:4000',
        ws: true
      }
    }
  }
})
