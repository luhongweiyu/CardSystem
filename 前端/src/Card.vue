<template>
  <main class="查询页" v-loading="加载中">
    <el-card class="查询卡片" shadow="never">
      <h2>卡密查询</h2>
      <p class="说明">先选择卡密模式，再输入完整卡密查询。</p>
      <el-radio-group v-model="模式" class="模式选择" @change="详情 = null">
        <el-radio-button label="point">点卡</el-radio-button>
        <el-radio-button label="duration">时长卡</el-radio-button>
      </el-radio-group>
      <el-input v-model="卡密" clearable placeholder="请输入卡密" @keyup.enter="查询详情">
        <template #prepend>卡密</template>
      </el-input>
      <el-button class="查询按钮" type="primary" @click="查询详情">查询</el-button>
    </el-card>

    <el-card v-if="详情?.mode === 'point'" class="结果卡片" shadow="never">
      <div class="信息网格">
        <span>卡密</span>
        <strong>{{ 详情.card }}</strong>
        <span>软件</span>
        <strong>#{{ 详情.software }}</strong>
        <span>点数余额</span>
        <strong>{{ 详情.point_balance }} 点</strong>
        <span>授权设备</span>
        <strong>{{ 详情.authorized_device_count }}</strong>
        <span>在线设备</span>
        <strong>{{ 详情.online_device_count }}</strong>
        <span>状态</span>
        <el-tag :type="详情.card_state === 4 ? 'danger' : 'success'">
          {{ 详情.card_state === 4 ? '冻结' : '正常' }}
        </el-tag>
      </div>
      <el-divider content-position="left">设备授权</el-divider>
      <el-table :data="详情.devices" border stripe empty-text="暂无设备会话">
        <el-table-column prop="device_id" label="设备 ID" min-width="180" show-overflow-tooltip />
        <el-table-column prop="device_alias" label="设备别名" min-width="130" show-overflow-tooltip />
        <el-table-column label="授权到期" width="180">
          <template #default="scope">{{ 格式化时间(scope.row.authorized_until) }}</template>
        </el-table-column>
        <el-table-column label="授权状态" width="90">
          <template #default="scope">
            <el-tag :type="scope.row.authorized ? 'success' : 'info'">
              {{ scope.row.authorized ? '未到期' : '已到期' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="在线状态" width="90">
          <template #default="scope">
            <el-tag :type="scope.row.online ? 'success' : 'info'">
              {{ scope.row.forced_offline ? '已下线' : scope.row.online ? '在线' : '离线' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="needle" label="needle" min-width="220" show-overflow-tooltip />
      </el-table>
      <el-pagination
        v-if="设备分页.total > 0"
        v-model:current-page="设备分页.page"
        v-model:page-size="设备分页.page_size"
        class="分页"
        layout="total, prev, pager, next"
        :total="设备分页.total"
        @current-change="查询详情(设备分页.page)"
      />
      <el-button type="primary" plain @click="打开流水">查看点数流水</el-button>
    </el-card>

    <el-card v-if="详情?.mode === 'duration'" class="结果卡片" shadow="never">
      <div class="信息网格">
        <span>卡密</span>
        <strong>{{ 详情.card }}</strong>
        <span>软件</span>
        <strong>#{{ 详情.software }}</strong>
        <span>卡面时长</span>
        <strong>{{ 时长文本(详情.duration_minutes) }}</strong>
        <span>状态</span>
        <el-tag :type="详情.card_state === 4 ? 'danger' : 详情.status === '已激活' ? 'success' : 'info'">
          {{ 详情.status }}
        </el-tag>
        <span>使用时间</span>
        <strong>{{ 格式化时间(详情.use_time) }}</strong>
        <span>到期时间</span>
        <strong>{{ 格式化时间(详情.end_time) }}</strong>
        <span>暂停剩余</span>
        <strong>{{ 详情.status === '已暂停' ? 时长文本(详情.paused_remaining_minutes) : '-' }}</strong>
        <span>在线状态</span>
        <strong>{{ 详情.online ? '在线' : '不在线' }}</strong>
      </div>
      <div class="操作行">
        <el-button v-if="详情.status === '已激活'" type="warning" @click="暂停时长卡">暂停时长</el-button>
        <el-button v-if="详情.status === '已暂停'" type="success" @click="恢复时长卡">恢复时长</el-button>
        <el-button v-if="可充值" type="primary" plain @click="打开充值">使用充值卡</el-button>
      </div>
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

    <el-dialog v-model="充值框.显示" title="使用时长充值卡" width="460px" destroy-on-close>
      <p class="对话框说明">目标卡：{{ 详情?.card }}；充值卡必须与目标卡属于同一软件。</p>
      <el-input v-model="充值框.card" clearable placeholder="请输入充值卡卡密" @keyup.enter="使用充值卡" />
      <template #footer>
        <el-button @click="充值框.显示 = false">取消</el-button>
        <el-button type="primary" :loading="充值框.加载中" @click="使用充值卡">确认充值</el-button>
      </template>
    </el-dialog>
  </main>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import apiClient, { 获取接口错误提示, 规范化卡密 } from './api/请求客户端.js'

const centerID = new URLSearchParams(window.location.search).get('center_id') || ''
const 卡密 = ref('')
const 模式 = ref('point')
const 加载中 = ref(false)
const 详情 = ref(null)
const 流水框 = reactive({ 显示: false, 加载中: false, rows: [], balance: 0, page: 1, page_size: 20, total: 0 })
const 设备分页 = reactive({ page: 1, page_size: 50, total: 0 })
const 充值框 = reactive({ 显示: false, 加载中: false, card: '' })
const 错误 = (error) => ElMessage.error(获取接口错误提示(error))
const 事件名称 = (type) => (type === 'debit' ? '扣点' : type === 'credit' ? '补点' : type || '')
const 格式化时间 = (value) => (value ? new Date(value).toLocaleString('zh-CN') : '-')
const 时长文本 = (minutes) => {
  const value = Number(minutes || 0)
  if (value === 52560000) return '永久卡'
  if (value % 1440 === 0) return `${value / 1440} 天`
  if (value % 60 === 0) return `${value / 60} 小时`
  return `${value} 分钟`
}
// 只有仍在使用或暂停保留时长的目标卡允许充值；已到期卡需要先按管理端续费，
// 不能把独立充值卡直接用于已结束的卡，保持与服务端业务条件一致。
const 可充值 = computed(() => ['已激活', '已暂停'].includes(详情.value?.status))

const 查询详情 = function (page = 1) {
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
  if (模式.value === 'duration') {
    加载中.value = true
    apiClient
      .post('/visitor/查询时长卡', { center_id: centerID, card })
      .then((res) => {
        if (!res.data?.state) throw new Error(res.data?.msg || '查询失败')
        详情.value = { mode: 'duration', ...(res.data.data || {}) }
      })
      .catch(错误)
      .finally(() => { 加载中.value = false })
    return
  }
  设备分页.page = page
  加载中.value = true
  apiClient
    .post('/point_card/query', { center_id: centerID, card, page: 设备分页.page, page_size: 设备分页.page_size })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '查询失败')
      详情.value = {
        mode: 'point',
        card,
        software: Number(res.data.software || 0),
        point_balance: Number(res.data.point_balance || 0),
        card_state: Number(res.data.card_state || 0),
        authorized_device_count: Number(res.data.authorized_device_count || 0),
        online_device_count: Number(res.data.online_device_count || 0),
        devices: Array.isArray(res.data.devices) ? res.data.devices : []
      }
      设备分页.total = Number(res.data.device_total || 0)
      设备分页.page = Number(res.data.device_page || page)
      设备分页.page_size = Number(res.data.device_page_size || 设备分页.page_size)
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
const 打开充值 = function () {
  充值框.card = ''
  充值框.显示 = true
}
const 刷新时长卡详情 = function () {
  if (!详情.value?.card) return Promise.resolve()
  const card = 详情.value.card
  加载中.value = true
  return apiClient
    .post('/visitor/查询时长卡', { center_id: centerID, card })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '查询失败')
      详情.value = { mode: 'duration', ...(res.data.data || {}) }
    })
    .catch(错误)
    .finally(() => { 加载中.value = false })
}
const 暂停时长卡 = function () {
  if (!详情.value?.card) return
  apiClient
    .post('/visitor/duration_card/pause', { center_id: centerID, card: 详情.value.card })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '暂停失败')
      ElMessage.success(res.data.msg || '暂停成功')
      return 刷新时长卡详情()
    })
    .catch(错误)
}
const 恢复时长卡 = function () {
  if (!详情.value?.card) return
  apiClient
    .post('/visitor/duration_card/resume', { center_id: centerID, card: 详情.value.card })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '恢复失败')
      ElMessage.success(res.data.msg || '恢复成功')
      return 刷新时长卡详情()
    })
    .catch(错误)
}
const 使用充值卡 = function () {
  const sourceCard = 规范化卡密(充值框.card)
  if (!sourceCard || !详情.value?.card) {
    ElMessage.warning('请输入格式正确的充值卡')
    return
  }
  充值框.card = sourceCard
  充值框.加载中 = true
  apiClient
    .post('/visitor/duration_recharge_card/redeem', {
      center_id: centerID,
      recharge_card: sourceCard,
      cards: [详情.value.card]
    })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '充值失败')
      if (Array.isArray(res.data.failed) && res.data.failed.length) {
        throw new Error(res.data.failed.includes(详情.value.card) ? '目标时长卡充值失败' : (res.data.msg || '充值失败'))
      }
      ElMessage.success(res.data.msg || '充值成功')
      充值框.显示 = false
      return 刷新时长卡详情()
    })
    .catch(错误)
    .finally(() => { 充值框.加载中 = false })
}
const 查询流水 = function (resetPage = false) {
  if (!详情.value) return
  if (resetPage) 流水框.page = 1
  流水框.加载中 = true
  apiClient
    .post('/point_card/point_ledger/query', {
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
.操作行 {
  display: flex;
  gap: 10px;
  margin-top: 4px;
}
.对话框说明 {
  margin: 0 0 12px;
  color: #9da7b5;
  line-height: 1.6;
}
.模式选择 {
  display: flex;
  margin-bottom: 12px;
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
