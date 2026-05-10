import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ArcoVue from '@arco-design/web-vue'
import App from './App.vue'
import router from './router'
import './styles/main.css'

console.log(
  '%c Team-API %c v0.1.0 %c Admin ',
  'background: #14b8a6; color: white; padding: 4px 8px; border-radius: 4px 0 0 4px; font-weight: bold;',
  'background: #0d9488; color: white; padding: 4px 8px;',
  'background: #115e59; color: white; padding: 4px 8px; border-radius: 0 4px 4px 0;',
)
console.log(
  '%cLicensed under GNU AGPL v3.0 | Copyright © 2025-2026 Team-API Contributors',
  'color: #666;',
)

const app = createApp(App)
app.use(createPinia())
app.use(ArcoVue)
app.use(router)
app.mount('#app')
