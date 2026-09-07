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
      '/card': 'http://127.0.0.1:802',
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
          const visitorAPI = ['/visitor/查询所有卡密', '/visitor/查询卡密', '/visitor/point_ledger/query']
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
