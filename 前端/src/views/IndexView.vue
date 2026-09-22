<template>
  <aside class="导航栏" :class="{ 折叠: 导航开关 }">
    <div class="品牌区">
      <img class="品牌图标" src="../assets/logo.svg" alt="卡密管理" />
      <div v-show="!导航开关" class="品牌文字">
        <strong>卡密管理</strong>
        <span>卡密控制台</span>
      </div>
    </div>

    <div v-show="!导航开关" class="账号提示">
      <span>{{ 是代理账号 ? '小伙伴账号' : '管理员账号' }}</span>
      <strong>{{ 账号 || '当前账号' }}</strong>
    </div>

    <el-menu
      class="侧边菜单"
      :collapse="导航开关"
      :default-active="$route.path"
      :collapse-transition="false"
      background-color="transparent"
      text-color="#aeb9c9"
      active-text-color="#ffffff"
      router
    >
      <el-menu-item v-for="item in 菜单" :key="item.path" :index="item.path">
        <el-icon><component :is="item.icon" /></el-icon>
        <template #title>{{ item.label }}</template>
      </el-menu-item>
    </el-menu>

    <div v-show="!导航开关" class="底部提示">
      <el-icon><CircleCheckFilled /></el-icon>
      <span>系统已连接</span>
    </div>
  </aside>
</template>

<script setup>
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { use登录状态Store } from '../stores/登录状态.js'

const stores = use登录状态Store()
const { 导航开关, 账号, 是代理账号 } = storeToRefs(stores)

const 管理员菜单 = [
  { path: '/index', label: '首页', icon: 'HomeFilled' },
  { path: '/software', label: '软件管理', icon: 'Iphone' },
  { path: '/card', label: '点卡', icon: 'Postcard' },
  { path: '/duration-card', label: '时长卡', icon: 'Timer' },
  { path: '/duration-recharge-card', label: '充值卡', icon: 'CreditCard' },
  { path: '/point-ledger', label: '点数流水', icon: 'Tickets' },
  { path: '/setting', label: '设置', icon: 'Setting' },
  { path: '/help', label: '接入帮助', icon: 'QuestionFilled' },
  { path: '/log', label: '运行日志', icon: 'List' },
  { path: '/about', label: '关于', icon: 'InfoFilled' }
]
const 代理菜单 = [
  { path: '/software', label: '软件设置', icon: 'Iphone' },
  { path: '/card', label: '点卡', icon: 'Postcard' },
  { path: '/duration-card', label: '时长卡', icon: 'Timer' },
  { path: '/duration-recharge-card', label: '充值卡', icon: 'CreditCard' },
  { path: '/help', label: '接入帮助', icon: 'QuestionFilled' },
  { path: '/log', label: '运行日志', icon: 'List' },
  { path: '/about', label: '关于', icon: 'InfoFilled' }
]
const 菜单 = computed(() => (是代理账号.value ? 代理菜单 : 管理员菜单))
</script>

<style scoped>
.导航栏 {
  display: flex;
  flex-direction: column;
  width: 100%;
  min-height: calc(100vh - 49px);
  overflow: hidden;
  border-right: 1px solid #343b48;
  background: #252b36;
  transition: background-color 0.2s;
}

.品牌区 {
  display: flex;
  align-items: center;
  gap: 11px;
  min-height: 64px;
  padding: 0 18px;
  border-bottom: 1px solid rgba(128, 145, 170, 0.16);
}

.品牌图标 {
  flex: none;
  width: 30px;
  height: 30px;
}

.品牌文字 {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 1px;
}

.品牌文字 strong {
  color: #f5f7fb;
  font-size: 16px;
  line-height: 1.3;
  white-space: nowrap;
}

.品牌文字 span {
  color: #7f8c9f;
  font-size: 11px;
  letter-spacing: 0.08em;
  white-space: nowrap;
}

.账号提示 {
  display: flex;
  flex-direction: column;
  gap: 3px;
  margin: 12px 12px 7px;
  padding: 9px 13px;
  border: 1px solid rgba(129, 153, 187, 0.18);
  border-radius: 10px;
  background: rgba(13, 18, 27, 0.24);
}

.账号提示 span {
  color: #8491a4;
  font-size: 11px;
}

.账号提示 strong {
  overflow: hidden;
  color: #dce5f2;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.侧边菜单 {
  flex: 1;
  min-width: 0;
  padding: 7px 8px;
  border-right: 0;
}

:deep(.el-menu-item) {
  height: 46px;
  margin: 3px 0;
  border-radius: 9px;
  line-height: 46px;
  transition: color 0.18s ease, background-color 0.18s ease;
}

:deep(.el-menu-item:hover) {
  color: #e7effc !important;
  background: rgba(91, 126, 173, 0.2) !important;
}

:deep(.el-menu-item.is-active) {
  color: #fff !important;
  background: linear-gradient(90deg, #397bc7, #3567a1) !important;
  box-shadow: 0 5px 14px rgba(39, 92, 157, 0.2);
}

:deep(.el-menu-item .el-icon) {
  margin-right: 11px;
  font-size: 18px;
}

.折叠 .品牌区 {
  justify-content: center;
  padding: 0;
}

.折叠 .品牌图标 {
  width: 29px;
  height: 29px;
}

.折叠 .侧边菜单 {
  padding-right: 7px;
  padding-left: 7px;
}

:deep(.el-menu--collapse .el-menu-item) {
  justify-content: center;
  padding: 0 !important;
}

:deep(.el-menu--collapse .el-menu-item .el-icon) {
  margin-right: 0;
}

.底部提示 {
  display: flex;
  align-items: center;
  gap: 7px;
  min-height: 46px;
  padding: 0 19px;
  border-top: 1px solid rgba(128, 145, 170, 0.16);
  color: #748398;
  font-size: 11px;
}

.底部提示 .el-icon {
  color: #54b58c;
}
</style>
