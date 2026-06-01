import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createHead } from '@unhead/vue'
import App from './App.vue'
import router from './router'
import './styles/main.css'

const head = createHead()

declare const __APP_VERSION__: string

console.log(
  '%c Team-API %c ' + __APP_VERSION__ + ' %c Tenant ',
  'background: #14b8a6; color: white; padding: 4px 8px; border-radius: 4px 0 0 4px; font-weight: bold;',
  'background: #0d9488; color: white; padding: 4px 8px;',
  'background: #115e59; color: white; padding: 4px 8px; border-radius: 0 4px 4px 0;',
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

const app = createApp(App)
app.use(head)
app.use(createPinia())
app.use(router)
app.mount('#app')
