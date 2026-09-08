import { createRouter, createWebHashHistory } from 'vue-router'

// 管理端只保留当前点卡系统实际使用的页面，旧的售卡、提现等无关入口
// 不再注册，避免用户进入一个没有后端实现的空页面。
const router = createRouter({
  history: createWebHashHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', redirect: '/index' },
    { path: '/index', name: 'index', component: () => import('../views/HomeView.vue') },
    { path: '/card', name: 'card', component: () => import('../views2/Card.vue') },
    { path: '/duration-card', name: 'duration-card', component: () => import('../views2/时长卡.vue') },
    { path: '/software', name: 'software', component: () => import('../views2/SoftWare.vue') },
    { path: '/point-ledger', name: 'point-ledger', component: () => import('../views2/点数流水.vue') },
    { path: '/log', name: 'log', component: () => import('../views2/Log.vue') },
    { path: '/setting', name: 'setting', component: () => import('../views2/Setting.vue') },
    { path: '/help', name: 'help', component: () => import('../views2/Help.vue') },
    { path: '/about', name: 'about', component: () => import('../views/About.vue') }
  ]
})

export default router
