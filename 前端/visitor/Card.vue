<template>
  <main class="访客页" v-loading="加载中">
    <el-card shadow="never" class="查询区">
      <h2>点卡查询</h2>
      <p>只展示卡密基础状态和点数余额，不会公开管理端配置。</p>
      <el-input v-model="筛选" clearable placeholder="请输入完整卡密" @keyup.enter="查询列表;" />
      <el-button type="primary" class="按钮" @click="查询列表;">查询</el-button>
    </el-card>

    <el-table :data="列表" border stripe>
      <el-table-column prop="card" label="卡密" min-width="200" show-overflow-tooltip />
      <el-table-column prop="software" label="软件ID" width="90" />
      <el-table-column label="余额" width="110">
        <template #default="scope">{{ scope.row.point_balance }} 点</template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="scope">
          <el-tag :type="scope.row.card_state === 4 ? 'danger' : 'success'">
            {{ scope.row.card_state === 4 ? '冻结' : '正常' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="120">
        <template #default="scope">
          <el-button link type="primary" @click="查看流水(scope.row)">查看流水</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-if="!加载中 && !列表.length" description="暂无匹配的点卡" />

    <el-dialog v-model="流水框.显示" title="点数流水" width="900px">
      <div class="流水摘要">卡密：{{ 流水框.card }}　当前余额：{{ 流水框.balance }} 点</div>
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
import apiClient, { 获取接口错误提示, 规范化卡密 } from '../src/api/请求客户端.js'

const centerID = new URLSearchParams(window.location.search).get('center_id') || ''
const 筛选 = ref('')
const 列表 = ref([])
const 加载中 = ref(false)
const 流水框 = reactive({
  显示: false,
  加载中: false,
  card: '',
  software: 0,
  balance: 0,
  rows: [],
  page: 1,
  page_size: 20,
  total: 0
})
const 错误 = (error) => ElMessage.error(获取接口错误提示(error))
const 事件名称 = (type) => (type === 'debit' ? '扣点' : type === 'credit' ? '补点' : type || '')
const 请求 = (path, data = {}) => apiClient.post('/visitor' + path, { ...data, center_id: centerID })

const 查询列表 = function () {
  列表.value = []
  if (!centerID) {
    ElMessage.error('查询链接缺少 center_id')
    return
  }
  const card = 规范化卡密(筛选.value)
  if (!card) {
    ElMessage.warning('请输入7至63位完整卡密，只能包含字母、数字、下划线或短横线')
    return
  }
  筛选.value = card
  加载中.value = true
  请求('/查询所有卡密', { card })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '查询失败')
      列表.value = res.data.data || []
    })
    .catch(错误)
    .finally(() => {
      加载中.value = false
    })
}
const 查看流水 = function (row) {
  Object.assign(流水框, {
    显示: true,
    card: row.card,
    software: Number(row.software || 0),
    balance: Number(row.point_balance || 0),
    rows: [],
    page: 1,
    total: 0
  })
  查询流水(true)
}
const 查询流水 = function (resetPage = false) {
  if (!流水框.card) return
  if (resetPage) 流水框.page = 1
  流水框.加载中 = true
  请求('/point_ledger/query', {
    card: 流水框.card,
    software: 流水框.software,
    page: 流水框.page,
    page_size: 流水框.page_size
  })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '查询流水失败')
      流水框.rows = res.data.data || []
      流水框.total = Number(res.data.num || 0)
      流水框.balance = Number(res.data.balance ?? 流水框.balance)
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
.访客页 {
  max-width: 1100px;
  margin: 0 auto;
  padding: 28px 18px;
}
.查询区 {
  margin-bottom: 16px;
}
.查询区 p {
  color: #9da7b5;
}
.按钮 {
  margin-top: 12px;
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
