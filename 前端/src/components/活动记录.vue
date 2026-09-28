<template>
  <div>
    <p class="记录说明">最近登录和心跳记录，最长保留 24 小时；</p>
    <el-table :data="显示记录" v-loading="props.loading" border stripe empty-text="暂无近期活动记录">
      <el-table-column label="时间" width="175">
        <template #default="scope">{{ 格式化时间(scope.row.time) }}</template>
      </el-table-column>
      <el-table-column prop="ip" label="IP" min-width="150" show-overflow-tooltip>
        <template #default="scope">{{ scope.row.ip || '-' }}</template>
      </el-table-column>
      <el-table-column prop="needle" label="needle" width="100" show-overflow-tooltip>
        <template #default="scope">{{ scope.row.needle || '-' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="70">
        <template #default="scope">{{ scope.row.operation === 'login' ? '登录' : '心跳' }}</template>
      </el-table-column>
      <el-table-column label="结果" width="70">
        <template #default="scope">
          <el-tag :type="scope.row.success ? 'success' : 'danger'">{{ scope.row.success ? '成功' : '失败' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="message" label="说明" min-width="200" show-overflow-tooltip />
    </el-table>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { 格式化时间 } from '../utils/时长工具.js'

const props = defineProps({
  rows: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false }
})

// 统一按记录时间倒序，保证最新记录始终显示在最上面。
const 显示记录 = computed(() => [...props.rows].sort((left, right) => {
  const leftTime = Date.parse(left?.time || '') || 0
  const rightTime = Date.parse(right?.time || '') || 0
  return rightTime - leftTime
}))
</script>

<style scoped>
.记录说明 {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.6;
}
</style>
