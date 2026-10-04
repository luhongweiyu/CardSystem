<template>
  <section class="页面">
    <h2>操作日志</h2>
    <BusinessLogViewer :query="查询日志" :show-agent="!stores.是代理账号" :agents="代理列表" />
  </section>
</template>

<script setup>
import { onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import BusinessLogViewer from '../components/业务日志查看.vue'
import { use登录状态Store } from '../stores/登录状态.js'

const stores = use登录状态Store()
const { 代理列表, 是代理账号 } = storeToRefs(stores)
const 查询日志 = (params) => {
  const agentId = Number(params.agent_id || 0)
  if (agentId > 0 && !是代理账号.value) {
    return stores.post('/查询代理账号日志', { ...params, id: agentId })
  }
  return stores.post('/query_log', params)
}

onMounted(() => {
  if (!是代理账号.value) stores.查询代理列表()
})
</script>

<style scoped>
.页面 {
  padding: 16px;
  color: #e6eaf2;
}
h2 {
  margin: 0 0 16px;
}
</style>
