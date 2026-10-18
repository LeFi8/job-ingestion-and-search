import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

const proxy = {
  '/api': { target: process.env.API_URL ?? 'http://localhost:8080', changeOrigin: true },
}

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: { proxy },
  preview: { proxy },
  test: { environment: 'jsdom', setupFiles: ['./src/test/setup.ts'] },
})
