import { fileURLToPath, URL } from "node:url";

import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import AutoImport from "unplugin-auto-import/vite";
// import Components from "unplugin-vue-components/vite";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    ,
    AutoImport({
      //注册
      imports: [
        "vue",
        "vue-router",
        "pinia",
        {
          axios: [
            // 默认导入
            ["default", "axios"] // import { default as axios } from 'axios',
          ]
        }
      ],
      dts: "./auto-imports.d.ts"
    })
  ],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url))
    }
  }, build: {
    rollupOptions: {
      input: {
        // 配置所有页面路径，使得所有页面都会被打包
        main: 'index.html',
        pageone: 'usercard/index.html',
        pageone: 'visitor/index.html',
        // pagetwo: resolve(__dirname, 'pagetwo/index.html')
      }
    }
  }
});
