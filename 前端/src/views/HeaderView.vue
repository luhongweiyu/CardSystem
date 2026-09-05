<template>
  <header class="顶部栏">
    <el-button text @click="开关导航;" aria-label="切换导航">
      <el-icon>
        <DArrowLeft v-if="!导航开关" />
        <DArrowRight v-else />
      </el-icon>
    </el-button>
    <span class="系统名称">卡密管理</span>

    <div class="顶部信息">
      <el-link v-if="访客链接" :href="访客链接" target="_blank" type="success">访客查询页</el-link>
      <span>{{ 是代理账号 ? '渠道合伙人' : '管理员' }}：{{ 账号 }}</span>
      <span v-if="是代理账号">余额：{{ 账号信息.balance ?? 0 }} 点</span>
      <span v-else>本小时请求：{{ api次数 }}</span>
      <el-button type="danger" plain size="small" @click="退出登录;">退出登录</el-button>
    </div>
  </header>
</template>

<script setup>
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import Cookies from 'js-cookie'
import { use登录状态Store } from '../stores/登录状态.js'

const stores = use登录状态Store()
const { 导航开关, 账号, 密码, token, 登录状态, 用户id, api次数, 是代理账号, 账号信息 } = storeToRefs(stores)

const 访客链接 = computed(() => {
  const centerID = 是代理账号.value ? 账号信息.value.center_id : 用户id.value
  return centerID ? `${window.location.origin}/visitor/index.html?center_id=${encodeURIComponent(centerID)}` : ''
})

const 开关导航 = function () {
  导航开关.value = !导航开关.value
}

const 清理本地登录状态 = function () {
  Cookies.remove('password')
  密码.value = ''
  token.value = ''
  登录状态.value = false
  用户id.value = ''
  api次数.value = 0
  是代理账号.value = false
  账号信息.value = {}
}

const 退出登录 = function () {
  if (!token.value) {
    清理本地登录状态()
    return
  }
  stores.post('/user_logout', {}).finally(清理本地登录状态)
}
</script>

<style scoped>
.顶部栏 {
  min-height: 48px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  box-sizing: border-box;
}
.系统名称 {
  font-size: 18px;
  font-weight: 600;
  white-space: nowrap;
}
.顶部信息 {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 14px;
  color: #d7dce5;
  flex-wrap: wrap;
  justify-content: flex-end;
}
</style>
