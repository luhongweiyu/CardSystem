import { createRouter, createWebHashHistory } from 'vue-router'
import { use登录状态Store } from '../stores/登录状态.js'

// 管理端只保留当前点卡系统实际使用的页面，旧的售卡、提现等无关入口
// 不再注册，避免用户进入一个没有后端实现的空页面。
const router = createRouter({
  history: createWebHashHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', redirect: '/index' },
    { path: '/index', name: 'index', component: () => import('../views/HomeView.vue') },
    { path: '/card', name: 'card', component: () => import('../views2/Card.vue') },
    { path: '/duration-card', name: 'duration-card', component: () => import('../views2/时长卡.vue') },
    { path: '/duration-recharge-card', name: 'duration-recharge-card', component: () => import('../views2/时长充值卡.vue') },
    { path: '/software', name: 'software', component: () => import('../views2/SoftWare.vue') },
    { path: '/point-ledger', name: 'point-ledger', meta: { adminOnly: true }, component: () => import('../views2/点数流水.vue') },
    { path: '/log', name: 'log', component: () => import('../views2/Log.vue') },
    { path: '/setting', name: 'setting', meta: { adminOnly: true }, component: () => import('../views2/Setting.vue') },
    { path: '/help', name: 'help', component: () => import('../views2/Help.vue') },
    { path: '/about', name: 'about', component: () => import('../views/About.vue') },
    { path: '/:pathMatch(.*)*', redirect: '/index' }
  ]
})

// 菜单隐藏之外也限制直接输入地址，避免代理进入没有对应接口的管理员页面。
router.beforeEach((to) => {
  const stores = use登录状态Store()
  if (stores.是代理账号 && to.meta.adminOnly) return '/card'
})

export default router
