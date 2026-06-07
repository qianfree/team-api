import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'
import { readFileSync } from 'node:fs'
import { execSync } from 'node:child_process'
import { resolve } from 'node:path'

function getVersion(): string {
  if (process.env.NODE_ENV === 'development') return 'dev'
  // 1. 环境变量注入（Docker 构建时通过 VITE_APP_VERSION 传入）
  if (process.env.VITE_APP_VERSION) return process.env.VITE_APP_VERSION
  // 2. git tag
  try {
    return execSync('git describe --tags --always', { encoding: 'utf-8' }).trim()
  } catch {
    // 3. VERSION 文件
    try {
      return readFileSync(resolve(__dirname, '../../VERSION'), 'utf-8').trim()
    } catch {
      return 'dev'
    }
  }
}

const version = getVersion()

export default defineConfig({
  base: '/admin/',
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
    port: 4001,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:18888',
        changeOrigin: true,
      }
    }
  }
})
