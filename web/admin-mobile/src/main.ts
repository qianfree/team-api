import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { Lazyload } from 'vant'
import App from './App.vue'
import router from './router'
import './styles/main.css'

declare const __APP_VERSION__: string

console.log(
  '%c Team-API %c ' + __APP_VERSION__ + ' %c Mobile ',
  'background: #14b8a6; color: white; padding: 4px 8px; border-radius: 4px 0 0 4px; font-weight: bold;',
  'background: #0d9488; color: white; padding: 4px 8px;',
  'background: #115e59; color: white; padding: 4px 8px; border-radius: 0 4px 4px 0;',
)

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(Lazyload)
app.mount('#app')
