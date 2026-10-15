import { createRouter, createWebHashHistory } from 'vue-router'
import { use登录状态Store } from '../stores/登录状态.js'

// 管理端只保留当前点卡系统实际使用的页面，旧的售卡、提现等无关入口
// 不再注册，避免用户进入一个没有后端实现的空页面。
const router = createRouter({
  history: createWebHashHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', redirect: '/index' },
    // 后台页面的标题与简介统一由顶部栏展示，页面只保留操作区。
    {
      path: '/index', name: 'index',
      meta: { title: '卡密管理工作台', agentTitle: '小伙伴工作台', description: '从这里快速进入点卡、时长卡、软件计费和系统设置。' },
      component: () => import('../views/HomeView.vue')
    },
    {
      path: '/card', name: 'card',
      meta: { title: '点卡管理', description: '一张卡可在多台设备使用；同一设备在授权时长内重复登录不会重复扣点。' },
      component: () => import('../views2/Card.vue')
    },
    {
      path: '/duration-card', name: 'duration-card',
      meta: {
        title: '时长卡管理',
        description: '时长卡独立于点卡；首次登录激活，按卡面固定时长使用。',
        agentDescription: '生成并管理自己创建的时长卡，服务端会在发卡时重新核算费用。'
      },
      component: () => import('../views2/时长卡.vue')
    },
    {
      path: '/duration-recharge-card', name: 'duration-recharge-card',
      meta: { title: '时长充值卡', description: '充值卡不能登录，只能给同软件的已激活或已暂停时长卡增加时长；使用次数扣减后不能恢复。' },
      component: () => import('../views2/时长充值卡.vue')
    },
    {
      path: '/software', name: 'software',
      meta: {
        title: '软件管理', agentTitle: '软件设置',
        description: '客户端按分钟选择授权时长，实际扣点价格始终由服务端决定。',
        agentDescription: '查看管理员配置的软件和时长卡价格，设置自己的点卡代扣总开关。'
      },
      component: () => import('../views2/SoftWare.vue')
    },
    {
      path: '/point-ledger', name: 'point-ledger',
      meta: { adminOnly: true, title: '点数流水', description: '每一行代表一次真实余额变动；设备 ID 和别名保存在备注快照中。' },
      component: () => import('../views2/点数流水.vue')
    },
    {
      path: '/balance-stats', name: 'balance-stats',
      meta: { title: '余额统计', description: '查看小伙伴账户余额或点卡点数的增加与扣除。' },
      component: () => import('../views2/余额统计.vue')
    },
    {
      path: '/log', name: 'log',
      meta: { title: '操作日志', description: '查看当前账号的操作记录和小伙伴余额变化。' },
      component: () => import('../views2/Log.vue')
    },
    {
      path: '/setting', name: 'setting',
      meta: { adminOnly: true, title: '账号设置', description: '管理联系方式、公告和接口安全选项。' },
      component: () => import('../views2/Setting.vue')
    },
    {
      path: '/help', name: 'help',
      meta: { title: '接入帮助', description: '客户接入和管理员自动化接口；展开即可查看参数和返回值。' },
      component: () => import('../views2/Help.vue')
    },
    {
      path: '/about', name: 'about',
      meta: { title: '关于系统', description: '版本与支持信息。' },
      component: () => import('../views/About.vue')
    },
    { path: '/:pathMatch(.*)*', redirect: '/index' }
  ]
})

// 菜单隐藏之外也限制直接输入地址，避免代理进入没有对应接口的管理员页面。
router.beforeEach((to) => {
  const stores = use登录状态Store()
  if (stores.是代理账号 && to.meta.adminOnly) return '/card'
})

export default router
