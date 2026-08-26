import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createHead } from '@unhead/vue'
import naive from 'naive-ui'
import App from './App.vue'
import router from './router'
import './styles/main.css'

const head = createHead()

declare const __APP_VERSION__: string

console.log(
  '%c Team-API %c ' + __APP_VERSION__ + ' %c Tenant ',
  'background: #7b8ff5; color: white; padding: 4px 8px; border-radius: 4px 0 0 4px; font-weight: bold;',
  'background: #6d80f0; color: white; padding: 4px 8px;',
  'background: #4b58c0; color: white; padding: 4px 8px; border-radius: 0 4px 4px 0;',
)
console.log(
  '%cAGPL v3.0 开源协议',
  'color: #666;',
)
console.log(
  '%cCopyright © 2025-2026 Team-API Contributors',
  'color: #666;',
)
console.log(
  '%chttps://github.com/qianfree/team-api',
  'color: #666;',
)

// 构建发版后旧 hash chunk 失效时自动整页刷新加载新版本；标记防止刷新死循环
const CHUNK_RELOAD_KEY = 'spa_stale_chunk_reloaded'

function reloadOnStaleChunk() {
  if (!sessionStorage.getItem(CHUNK_RELOAD_KEY)) {
    sessionStorage.setItem(CHUNK_RELOAD_KEY, '1')
    window.location.reload()
  }
}

window.addEventListener('vite:preloadError', (event) => {
  event.preventDefault()
  reloadOnStaleChunk()
})

router.onError((error) => {
  const msg = String(error instanceof Error ? error.message : error)
  if (/Failed to fetch dynamically imported module|Importing a module script failed|error loading dynamically imported module/i.test(msg)) {
    reloadOnStaleChunk()
  }
})

const app = createApp(App)
app.config.errorHandler = (error, _instance, info) => {
  console.error('[VueError]', info, error)
}
app.use(head)
app.use(createPinia())
app.use(router)
app.use(naive)
app.mount('#app')

// 页面成功启动后清除标记，后续每次发版仍可各自动刷新一次
sessionStorage.removeItem(CHUNK_RELOAD_KEY)
