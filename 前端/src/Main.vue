<script setup>
import { RouterView } from 'vue-router'
import { storeToRefs } from 'pinia'
import { use登录状态Store } from './stores/登录状态.js'
import HeaderView from './views/HeaderView.vue'
import IndexView from './views/IndexView.vue'
import Login from './views/Login.vue'

const stores = use登录状态Store()
const { 登录状态 } = storeToRefs(stores)
</script>

<template>
  <Login v-if="!登录状态" />
  <el-container v-else class="应用">
    <el-header class="头部"><HeaderView /></el-header>
    <el-container class="主体">
      <el-aside :width="stores.导航开关 ? '64px' : '200px'" class="侧栏"><IndexView /></el-aside>
      <el-main class="内容"><RouterView /></el-main>
    </el-container>
  </el-container>
</template>

<style>
html,
body,
#app {
  width: 100%;
  min-height: 100%;
  height: 100%;
  margin: 0;
  background: #20242d;
  color: #e6eaf2;
}
.应用 {
  min-height: 100vh;
  background: rgba(24, 28, 36, 0.92);
}
.头部 {
  height: auto;
  min-height: 48px;
  padding: 0;
  border-bottom: 1px solid #343b48;
}
.主体 {
  min-height: calc(100vh - 49px);
}
.侧栏 {
  transition: width 0.2s;
  background: #545c64;
  overflow: hidden;
}
.内容 {
  padding: 0;
  overflow: auto;
}
</style>
