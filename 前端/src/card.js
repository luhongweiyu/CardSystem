import './assets/main.css'

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import { DatePicker } from 'ant-design-vue';
import 'ant-design-vue/dist/reset.css';
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'

import VueCookies from 'vue-cookies'

import App from './Card.vue'
// import router from './router'

const app = createApp(App)

app.use(createPinia())
// app.use(router)
app.use(ElementPlus, {
  locale: zhCn,
})
app.use(ElementPlus)

app.use(DatePicker);
app.use(VueCookies)

app.config.globalProperties.$cookies = VueCookies;//全局挂载 同vue2.x Vue.prototype.$cookies
 


for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

app.mount('#app')


// npm install element-plus --save
// npm install ant-design-vue --save

