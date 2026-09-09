<template>
  <section class="页面" v-loading="加载中">
    <div class="标题行">
      <div>
        <h2>点数流水</h2>
        <p class="说明">每一行代表一次真实余额变动；设备 ID 和别名保存在备注快照中。</p>
      </div>
    </div>
    <el-card shadow="never" class="筛选卡片">
      <el-form :inline="true" @submit.prevent>
        <el-form-item label="软件">
          <el-select v-model="筛选.software" clearable placeholder="全部软件" style="width: 160px">
            <el-option v-for="item in 软件列表" :key="item.ID" :label="item.Software" :value="item.ID" />
          </el-select>
        </el-form-item>
        <el-form-item label="卡密">
          <el-input v-model="筛选.card" clearable placeholder="模糊搜索" @keyup.enter="查询流水(true)" />
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="筛选.event_type" clearable placeholder="全部" style="width: 120px">
            <el-option label="扣点" value="debit" />
            <el-option label="补点" value="credit" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="查询流水(true)">查询</el-button>
          <el-button @click="重置">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>
    <el-table :data="列表" border stripe>
      <el-table-column prop="id" label="流水ID" width="90" />
      <el-table-column prop="created_at" label="时间" width="175" />
      <el-table-column prop="card" label="卡密" width="190" show-overflow-tooltip />
      <el-table-column label="软件" width="140" show-overflow-tooltip>
        <template #default="scope">{{ 软件名称(scope.row.software) }}</template>
      </el-table-column>
      <el-table-column label="类型" width="80">
        <template #default="scope">{{ 事件名称(scope.row.event_type) }}</template>
      </el-table-column>
      <el-table-column label="变动" width="80" align="right">
        <template #default="scope">
          <span :class="scope.row.change > 0 ? '增加' : '扣除'">
            {{ scope.row.change > 0 ? '+' : '' }}{{ scope.row.change }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="余额" width="130">
        <template #default="scope">{{ scope.row.balance_before }} → {{ scope.row.balance_after }}</template>
      </el-table-column>
      <el-table-column prop="remark" label="备注（含设备信息）" min-width="360" show-overflow-tooltip />
    </el-table>
    <el-pagination
      v-model:current-page="分页.page"
      v-model:page-size="分页.page_size"
      class="分页"
      background
      layout="total, sizes, prev, pager, next, jumper"
      :page-sizes="[20, 50, 100, 200]"
      :total="分页.total"
      @size-change="查询流水(true)"
      @current-change="查询流水(false)"
    />
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { use登录状态Store } from '../stores/登录状态.js'
import { 获取接口错误提示 } from '../api/请求客户端.js'

const stores = use登录状态Store()
const post = stores.post
const 加载中 = ref(false)
const 软件列表 = ref([])
const 列表 = ref([])
const 筛选 = reactive({ software: '', card: '', event_type: '' })
const 分页 = reactive({ page: 1, page_size: 50, total: 0 })
const 事件名称 = (type) => (type === 'debit' ? '扣点' : type === 'credit' ? '补点' : type || '')
const 软件名称 = (id) => 软件列表.value.find((item) => Number(item.ID) === Number(id))?.Software || `软件#${id}`
const 错误 = (error) => ElMessage.error(获取接口错误提示(error))
const 查询软件 = () =>
  post('/user_query_soft_list', {}).then((res) => {
    if (!res.data?.state) throw new Error(res.data?.msg || '查询软件失败')
    软件列表.value = res.data.data || []
  })
const 查询流水 = function (resetPage = false) {
  if (resetPage) 分页.page = 1
  加载中.value = true
  post('/point_card/ledger', { ...筛选, page: 分页.page, page_size: 分页.page_size })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '查询流水失败')
      列表.value = res.data.data || []
      分页.total = Number(res.data.num || 0)
    })
    .catch(错误)
    .finally(() => {
      加载中.value = false
    })
}
const 重置 = () => {
  Object.assign(筛选, { software: '', card: '', event_type: '' })
  查询流水(true)
}
onMounted(() => {
  Promise.all([查询软件(), 查询流水(true)]).catch(() => {})
})
</script>

<style scoped>
.页面 {
  padding: 16px;
  color: #e6eaf2;
}
.标题行 {
  display: flex;
  justify-content: space-between;
}
h2 {
  margin: 0 0 6px;
}
.说明 {
  margin: 0 0 12px;
  color: #aeb6c3;
  font-size: 13px;
}
.筛选卡片 {
  margin-bottom: 12px;
}
.分页 {
  margin-top: 14px;
  justify-content: flex-end;
}
.增加 {
  color: #67c23a;
}
.扣除 {
  color: #f56c6c;
}
</style>
