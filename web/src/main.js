import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './style.css'
import { applyTheme, getTheme } from './theme'

applyTheme(getTheme())
createApp(App).use(router).mount('#app')
