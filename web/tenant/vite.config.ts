import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'
import { readFileSync } from 'node:fs'
import { execSync } from 'node:child_process'
import { resolve } from 'node:path'

function getVersion(): string {
  if (process.env.NODE_ENV === 'development') return 'dev'
  if (process.env.VITE_APP_VERSION) return process.env.VITE_APP_VERSION
  try {
    return execSync('git describe --tags --always', { encoding: 'utf-8' }).trim()
  } catch {
    try {
      return readFileSync(resolve(__dirname, '../../VERSION'), 'utf-8').trim()
    } catch {
      return 'dev'
    }
  }
}

const version = getVersion()

export default defineConfig({
  base: '/',
  define: {
    __APP_VERSION__: JSON.stringify(version),
  },
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    host: true,
    port: 4002,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:18888',
        changeOrigin: true,
      },
      '/v1': {
        target: 'http://127.0.0.1:18888',
        changeOrigin: true,
      }
    }
  }
})
