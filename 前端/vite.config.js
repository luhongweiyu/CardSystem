import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  build: {
    rollupOptions: {
      input: {
        main: 'index.html',
        usercard: 'usercard/index.html',
        visitor: 'visitor/index.html'
      }
    }
  },
  server: {
    proxy: {
      '/admin': 'http://127.0.0.1:802',
      '/agent': 'http://127.0.0.1:802',
      // 管理端页面虽然仍使用 /card 作为前端路由，但客户端 API 已经
      // 明确分成 /point_card 和 /duration_card，避免开发代理把页面路由
      // 误转发到后端旧接口。
      '/point_card': 'http://127.0.0.1:802',
      '/duration_card': 'http://127.0.0.1:802',
      // /visitor 同时是访客多页面入口和后端 API 前缀，入口文件不能被代理到后端。
      '/visitor': {
        target: 'http://127.0.0.1:802',
        bypass(req) {
          const rawPath = (req.url || '').split('?')[0]
          let path = rawPath
          try {
            path = decodeURIComponent(rawPath)
          } catch {
            // URL 编码异常时保留原路径，后续会按静态资源处理并返回正常的 404。
          }
          const visitorAPI = [
            '/visitor/查询所有卡密',
            '/visitor/查询卡密',
            '/visitor/查询时长卡',
            '/visitor/point_ledger/query',
            '/visitor/duration_recharge_card/query',
            '/visitor/duration_recharge_card/redeem',
            '/visitor/duration_card/pause',
            '/visitor/duration_card/resume',
            '/visitor/查询充值卡',
            '/visitor/续费卡密',
            '/visitor/暂停时长',
            '/visitor/恢复时长'
          ]
          // 访客页面的 HTML、JS 和 Vue 源文件必须由 Vite 本地提供；只有访客 API
          // 继续代理到后端，否则 /visitor/index.js 会被后端的 API 前缀拦截并返回 404。
          if (!visitorAPI.includes(path)) {
            return req.url
          }
        }
      }
    }
  }
})
