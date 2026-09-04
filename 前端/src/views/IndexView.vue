<template>
  <el-menu
    class="侧边菜单"
    :collapse="导航开关"
    :default-active="$route.path"
    background-color="#545c64"
    text-color="#fff"
    active-text-color="#ffd04b"
    router
  >
    <el-menu-item v-for="item in 菜单" :key="item.path" :index="item.path">
      <el-icon><component :is="item.icon" /></el-icon>
      <template #title>{{ item.label }}</template>
    </el-menu-item>
  </el-menu>
</template>

<script setup>
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { use登录状态Store } from '../stores/登录状态.js'

const stores = use登录状态Store()
const { 导航开关, 是代理账号 } = storeToRefs(stores)

const 管理员菜单 = [
  { path: '/index', label: '首页', icon: 'HomeFilled' },
  { path: '/card', label: '点卡', icon: 'Postcard' },
  { path: '/software', label: '软件与价格', icon: 'Iphone' },
  { path: '/point-ledger', label: '点数流水', icon: 'Tickets' },
  { path: '/setting', label: '设置', icon: 'Setting' },
  { path: '/help', label: '接入帮助', icon: 'QuestionFilled' },
  { path: '/log', label: '运行日志', icon: 'List' },
  { path: '/about', label: '关于', icon: 'InfoFilled' }
]
const 代理菜单 = [
  { path: '/card', label: '点卡', icon: 'Postcard' },
  { path: '/help', label: '接入帮助', icon: 'QuestionFilled' },
  { path: '/log', label: '运行日志', icon: 'List' },
  { path: '/about', label: '关于', icon: 'InfoFilled' }
]
const 菜单 = computed(() => (是代理账号.value ? 代理菜单 : 管理员菜单))
</script>

<style scoped>
.侧边菜单 {
  min-height: calc(100vh - 48px);
  border-right: 0;
}
</style>
