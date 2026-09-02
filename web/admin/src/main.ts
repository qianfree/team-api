import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ArcoVue from '@arco-design/web-vue'
import App from './App.vue'
import router from './router'
import './styles/main.css'

declare const __APP_VERSION__: string

console.log(
  '%c Team-API %c ' + __APP_VERSION__ + ' %c Admin ',
  'background: #14b8a6; color: white; padding: 4px 8px; border-radius: 4px 0 0 4px; font-weight: bold;',
  'background: #0d9488; color: white; padding: 4px 8px;',
  'background: #115e59; color: white; padding: 4px 8px; border-radius: 0 4px 4px 0;',
)
console.log(
  '%cAGPL-3.0-or-later 开源协议',
  'color: #666;',
)
console.log(
  '%cCopyright © 2026 qianfree',
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
app.use(createPinia())
app.use(ArcoVue)
app.use(router)
app.mount('#app')

// 页面成功启动后清除标记，后续每次发版仍可各自动刷新一次
sessionStorage.removeItem(CHUNK_RELOAD_KEY)
