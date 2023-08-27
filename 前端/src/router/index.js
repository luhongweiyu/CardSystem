import { createRouter, createWebHashHistory } from "vue-router";
import HomeView from "../views/HomeView.vue";

const router = createRouter({
  history: createWebHashHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: "/",
      name: "home",
      component: HomeView
    },
    {
      path: "/about",
      name: "about",
      // route level code-splitting
      // this generates a separate chunk (About.[hash].js) for this route
      // which is lazy-loaded when the route is visited.
      component: () => import("../views/AboutView.vue")
    },
    {
      // 首页
      path: "/index",
      name: "index",
      component: HomeView
    },
    {
      // 卡密
      path: "/card",
      name: "card",
      component: () => import("../views2/Card.vue")
    },
    {
      // 软件
      path: "/software",
      name: "software",
      component: () => import("../views2/SoftWare.vue")
    },
    {
      // 帮助
      path: "/help",
      name: "help",
      component: () => import("../views2/Help.vue")
    },
    {
      // 文件
      path: "/file",
      name: "file",
      component: () => import("../views2/File.vue")
    },
    {
      // 用户
      path: "/user",
      name: "user",
      component: () => import("../views2/User.vue")
    },
    {
      // 充值
      path: "/charge",
      name: "charge",
      component: () => import("../views2/Charge.vue")
    },
    {
      // 代理
      path: "/agent",
      name: "agent",
      component: () => import("../views2/Agent.vue")
    },
    {
      // 数据
      path: "/data",
      name: "data",
      component: () => import("../views2/Data.vue")
    },
    {
      // 日志
      path: "/log",
      name: "log",
      component: () => import("../views2/Log.vue")
    },
    {
      // 消息
      path: "/msg",
      name: "msg",
      component: () => import("../views2/Msg.vue")
    },
    {
      // 推广
      path: "/promotion",
      name: "promotion",
      component: () => import("../views2/Promotion.vue")
    },
    {
      // 设置
      path: "/setting",
      name: "setting",
      component: () => import("../views2/Setting.vue")
    },
    {
      // 实名认证
      path: "/authentication",
      name: "authentication",
      component: () => import("../views2/Authentication.vue")
    },
    {
      // 售卡日志
      path: "/sell_log",
      name: "sell_log",
      component: () => import("../views2/SellLog.vue")
    },
    {
      // 提现日志
      path: "/cash",
      name: "cash",
      component: () => import("../views2/Cash.vue")
    },
    {
      // 销售统计
      path: "/sell_count",
      name: "sell_count",
      component: () => import("../views2/SellCount.vue")
    }
    // {
    //   //
    //   path: '/data',
    //   name: 'data',
    //   component: () => import('../views/AboutView.vue')
    // },
  ]
});

export default router;
