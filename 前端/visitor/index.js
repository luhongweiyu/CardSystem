import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'

import App from './Card.vue'

const app = createApp(App)

app.use(ElementPlus, {
  locale: zhCn
})

app.mount('#app')
