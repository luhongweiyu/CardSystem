<template>
  <div v-loading="加载中" class="日志查看">
    <div class="日志工具栏">
      <el-select
        v-if="showAgent && agents.length"
        v-model="代理ID"
        :disabled="加载中"
        clearable
        placeholder="全部代理"
        aria-label="代理筛选"
      >
        <el-option label="全部代理" :value="0" />
        <el-option v-for="agent in agents" :key="agent.id" :label="`${agent.name}（ID:${agent.id}）`" :value="agent.id" />
      </el-select>
      <el-select v-model="类型" :disabled="加载中" clearable placeholder="全部类型" aria-label="日志类型">
        <el-option label="普通操作" value="operation" />
        <el-option label="代理余额" value="agent_balance" />
      </el-select>
      <el-button type="primary" :icon="Search" :loading="加载中" @click="查询(true)">查询</el-button>
      <el-tooltip content="刷新日志">
        <el-button :icon="Refresh" :loading="加载中" aria-label="刷新日志" @click="查询()" />
      </el-tooltip>
    </div>
    <el-table :data="记录" row-key="id" border stripe>
      <el-table-column label="时间" width="180">
        <template #default="{ row }">{{ 格式化时间(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="类型" width="100">
        <template #default="{ row }">{{ row.type === 'agent_balance' ? '代理余额' : '普通操作' }}</template>
      </el-table-column>
      <el-table-column v-if="showAgent" label="代理 ID" width="90">
        <template #default="{ row }">{{ row.agent_id || '-' }}</template>
      </el-table-column>
      <el-table-column label="内容" min-width="280">
        <template #default="{ row }"><pre class="日志文本">{{ row.log }}</pre></template>
      </el-table-column>
    </el-table>
    <el-pagination
      v-model:current-page="分页.page"
      v-model:page-size="分页.page_size"
      class="日志分页"
      :disabled="加载中"
      :total="分页.total"
      :page-sizes="[20, 50, 100, 200]"
      layout="total, sizes, prev, pager, next"
      background
      @size-change="查询(true)"
      @current-change="查询()"
    />
  </div>
</template>

<script setup>
import { onBeforeUnmount, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
import { 获取接口错误提示 } from '../api/请求客户端.js'

const props = defineProps({
  query: { type: Function, required: true },
  showAgent: { type: Boolean, default: true },
  agents: { type: Array, default: () => [] }
})
const 加载中 = ref(false)
const 类型 = ref('')
const 代理ID = ref(0)
const 记录 = ref([])
const 分页 = reactive({ page: 1, page_size: 50, total: 0 })
let 请求序号 = 0

const 格式化时间 = (value) => {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false })
}

// 页面和代理弹窗使用相同协议；切换代理或关闭弹窗后忽略旧请求的结果。
const 查询 = async (resetPage = false) => {
  if (resetPage) 分页.page = 1
  const current = ++请求序号
  加载中.value = true
  try {
    const response = await props.query({ type: 类型.value, agent_id: 代理ID.value, page: 分页.page, page_size: 分页.page_size })
    if (current !== 请求序号) return
    if (!response.data?.state) throw new Error(response.data?.msg || '查询日志失败')
    记录.value = response.data.data || []
    分页.total = Number(response.data.num || 0)
  } catch (error) {
    if (current === 请求序号) ElMessage.error(获取接口错误提示(error))
  } finally {
    if (current === 请求序号) 加载中.value = false
  }
}

onBeforeUnmount(() => { 请求序号++ })
</script>

<style scoped>
.日志查看 { min-width: 0; }
.日志工具栏 { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.日志工具栏 :deep(.el-select) { width: 190px; }
.日志文本 { margin: 0; font: inherit; line-height: 1.6; white-space: pre-wrap; overflow-wrap: anywhere; }
.日志分页 { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 14px; }
@media (max-width: 600px) {
  .日志分页 :deep(.el-pagination__sizes) { margin-right: 0; }
}
</style>
