<template>
  <header class="顶部栏">
    <el-button text @click="开关导航" aria-label="切换导航">
      <el-icon>
        <DArrowLeft v-if="!导航开关" />
        <DArrowRight v-else />
      </el-icon>
    </el-button>
    <div class="页面信息">
      <h1>{{ 页面标题 }}</h1>
      <el-tooltip :content="页面说明" placement="bottom" :show-after="300">
        <span class="页面说明" tabindex="0" :aria-label="页面说明">
          <span class="说明文字">{{ 页面说明 }}</span>
          <el-icon class="说明图标"><InfoFilled /></el-icon>
        </span>
      </el-tooltip>
    </div>

    <div class="顶部信息">
      <el-link v-if="访客链接" :href="访客链接" target="_blank" type="success">卡密查询页</el-link>
      <span>{{ 是代理账号 ? '小伙伴' : '管理员' }}：{{ 账号 }}({{ 用户id }})</span>
      <span v-if="是代理账号">余额：{{ 账号信息.balance ?? 0 }} 点</span>
      <span v-else>本小时请求：{{ api次数 }}</span>
      <el-button type="danger" plain size="small" @click="退出登录">退出登录</el-button>
    </div>
  </header>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { storeToRefs } from 'pinia'
import Cookies from 'js-cookie'
import { use登录状态Store } from '../stores/登录状态.js'

const route = useRoute()
const stores = use登录状态Store()
const { 导航开关, 账号, token, 用户id, api次数, 是代理账号, 账号信息 } = storeToRefs(stores)

const 页面标题 = computed(() => (是代理账号.value && route.meta.agentTitle) || route.meta.title || '卡密管理')
const 页面说明 = computed(() => (是代理账号.value && route.meta.agentDescription) || route.meta.description || '')

const 访客链接 = computed(() => {
  const centerID = 是代理账号.value ? 账号信息.value.center_id : 用户id.value
  return centerID ? `${window.location.origin}/usercard/index.html?center_id=${encodeURIComponent(centerID)}` : ''
})

const 开关导航 = function () {
  导航开关.value = !导航开关.value
}

const 清理本地登录状态 = function () {
  Cookies.remove('password')
  stores.清理登录状态()
}

const 退出登录 = function () {
  if (!token.value) {
    清理本地登录状态()
    return
  }
  // 先清理前端状态，确保网络异常或服务端暂时不可用时，用户点击退出也能立即回到登录页。
  // 请求仍在后台执行，用于尽快删除服务端内存令牌；即使失败也不影响本地退出结果。
  stores.post('/user_logout', {}).catch(() => {})
  清理本地登录状态()
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
.页面信息 h1 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  white-space: nowrap;
}
.页面信息 { display: flex; align-items: center; gap: 12px; flex: 1; min-width: 0; }
.页面说明 { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #aeb6c3; font-size: 12px; }
.说明图标 { display: none; }
.顶部信息 {
  flex-shrink: 0;
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 14px;
  color: #d7dce5;
  flex-wrap: wrap;
  justify-content: flex-end;
}
@media (max-width: 1200px) {
  .说明文字 { display: none; }
  .说明图标 { display: inline-flex; }
  .页面说明 { flex: none; }
}
@media (max-width: 640px) {
  .顶部栏 {
    flex-wrap: wrap;
    padding: 6px 10px;
  }
  .顶部信息 {
    flex: 1 1 100%;
    margin-left: 34px;
    justify-content: flex-start;
    gap: 6px 12px;
  }
}
</style>
