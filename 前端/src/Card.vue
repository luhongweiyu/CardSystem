<template>
  <main class="查询页" v-loading="加载中">
    <el-card class="查询卡片" shadow="never">
      <h2>点卡查询</h2>
      <p class="说明">输入卡密可查看当前点数和有效授权设备数量。</p>
      <el-input v-model="卡密" clearable placeholder="请输入卡密" @keyup.enter="查询详情">
        <template #prepend>卡密</template>
      </el-input>
      <el-button class="查询按钮" type="primary" @click="查询详情">查询</el-button>
    </el-card>

    <el-card v-if="详情" class="结果卡片" shadow="never">
      <div class="信息网格">
        <span>卡密</span>
        <strong>{{ 详情.card }}</strong>
        <span>软件</span>
        <strong>#{{ 详情.software }}</strong>
        <span>点数余额</span>
        <strong>{{ 详情.point_balance }} 点</strong>
        <span>授权设备</span>
        <strong>{{ 详情.authorized_device_count }}</strong>
        <span>状态</span>
        <el-tag :type="详情.card_state === 4 ? 'danger' : 'success'">
          {{ 详情.card_state === 4 ? '冻结' : '正常' }}
        </el-tag>
      </div>
      <el-button type="primary" plain @click="打开流水">查看点数流水</el-button>
    </el-card>

    <el-dialog v-model="流水框.显示" title="点数流水" width="900px">
      <div class="流水摘要">当前余额：{{ 流水框.balance }} 点</div>
      <el-table :data="流水框.rows" border v-loading="流水框.加载中">
        <el-table-column prop="created_at" label="时间" width="175" />
        <el-table-column label="类型" width="80">
          <template #default="scope">{{ 事件名称(scope.row.event_type) }}</template>
        </el-table-column>
        <el-table-column label="变动" width="80">
          <template #default="scope">
            <span :class="scope.row.change > 0 ? '增加' : '扣除'">
              {{ scope.row.change > 0 ? '+' : '' }}{{ scope.row.change }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="余额" width="130">
          <template #default="scope">{{ scope.row.balance_before }} → {{ scope.row.balance_after }}</template>
        </el-table-column>
        <el-table-column prop="remark" label="备注（含设备信息）" min-width="340" show-overflow-tooltip />
      </el-table>
      <el-pagination
        v-model:current-page="流水框.page"
        v-model:page-size="流水框.page_size"
        class="分页"
        layout="total, prev, pager, next"
        :total="流水框.total"
        @current-change="查询流水(false)"
      />
    </el-dialog>
  </main>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import apiClient, { 获取接口错误提示, 规范化卡密 } from './api/请求客户端.js'

const centerID = new URLSearchParams(window.location.search).get('center_id') || ''
const 卡密 = ref('')
const 加载中 = ref(false)
const 详情 = ref(null)
const 流水框 = reactive({ 显示: false, 加载中: false, rows: [], balance: 0, page: 1, page_size: 20, total: 0 })
const 错误 = (error) => ElMessage.error(获取接口错误提示(error))
const 事件名称 = (type) => (type === 'debit' ? '扣点' : type === 'credit' ? '补点' : type || '')

const 查询详情 = function () {
  详情.value = null
  if (!centerID) {
    ElMessage.error('查询链接缺少 center_id，请联系管理员获取完整链接')
    return
  }
  const card = 规范化卡密(卡密.value)
  if (!card) {
    ElMessage.warning('请输入7至63位完整卡密，只能包含字母、数字、下划线或短横线')
    return
  }
  卡密.value = card
  加载中.value = true
  apiClient
    .post('/card/query', { center_id: centerID, card })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '查询失败')
      详情.value = {
        card,
        software: Number(res.data.software || 0),
        point_balance: Number(res.data.point_balance || 0),
        card_state: Number(res.data.card_state || 0),
        authorized_device_count: Number(res.data.authorized_device_count || 0)
      }
    })
    .catch(错误)
    .finally(() => {
      加载中.value = false
    })
}
const 打开流水 = function () {
  流水框.显示 = true
  流水框.page = 1
  流水框.rows = []
  查询流水(true)
}
const 查询流水 = function (resetPage = false) {
  if (!详情.value) return
  if (resetPage) 流水框.page = 1
  流水框.加载中 = true
  apiClient
    .post('/card/point_ledger/query', {
      center_id: centerID,
      card: 详情.value.card,
      software: 详情.value.software,
      page: 流水框.page,
      page_size: 流水框.page_size
    })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '查询流水失败')
      流水框.rows = res.data.data || []
      流水框.total = Number(res.data.num || 0)
      流水框.balance = Number(res.data.balance ?? 详情.value.point_balance)
    })
    .catch(错误)
    .finally(() => {
      流水框.加载中 = false
    })
}
</script>

<style>
html,
body,
#app {
  min-height: 100%;
  margin: 0;
  background: #20242d;
}
.查询页 {
  max-width: 900px;
  margin: 0 auto;
  padding: 36px 18px;
}
.查询卡片,
.结果卡片 {
  margin-bottom: 16px;
}
.查询按钮 {
  margin-top: 12px;
}
.说明 {
  color: #9da7b5;
}
.信息网格 {
  display: grid;
  grid-template-columns: 110px 1fr;
  gap: 12px;
  margin-bottom: 18px;
  align-items: center;
}
.信息网格 span {
  color: #9da7b5;
}
.信息网格 strong {
  word-break: break-all;
}
.流水摘要 {
  margin-bottom: 12px;
}
.分页 {
  margin-top: 12px;
  justify-content: flex-end;
}
.增加 {
  color: #67c23a;
}
.扣除 {
  color: #f56c6c;
}
</style>
