<template>
  <div>
    <p class="记录说明">最近登录和心跳记录，最长保留 24 小时；</p>
    <el-table :data="rows" v-loading="loading" border stripe empty-text="暂无近期活动记录">
      <el-table-column label="时间" width="175">
        <template #default="scope">{{ 格式化时间(scope.row.time) }}</template>
      </el-table-column>
      <el-table-column prop="ip" label="IP" min-width="150" show-overflow-tooltip>
        <template #default="scope">{{ scope.row.ip || '-' }}</template>
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
import { 格式化时间 } from '../utils/时长工具.js'

defineProps({
  rows: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false }
})
</script>

<style scoped>
.记录说明 {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.6;
}
</style>
