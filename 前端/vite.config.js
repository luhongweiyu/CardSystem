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
          const path = (req.url || '').split('?')[0]
          if (path === '/visitor' || path === '/visitor/' || path === '/visitor/index.html') {
            return req.url
          }
        }
      }
    }
  }
})
