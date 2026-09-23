<template>
  <main class="查询页" v-loading="加载中">
    <el-card class="查询卡片" shadow="never">
      <h2>卡密查询</h2>
      <p class="说明">完整卡密精确查询；前缀查询请在前缀后加 ***（固定匹配末尾 3 位）。多个完整卡密可用逗号分隔。</p>
      <el-radio-group v-model="模式" class="模式选择" :disabled="加载中 || !!操作中 || 充值框.加载中" @change="切换模式">
        <el-radio-button label="point">点卡</el-radio-button>
        <el-radio-button label="duration">时长卡</el-radio-button>
      </el-radio-group>
      <el-input v-model="卡密" clearable :disabled="加载中 || !!操作中 || 充值框.加载中" placeholder="完整卡密或前缀***；多个完整卡密用逗号分隔" @keyup.enter="查询卡密">
        <template #prepend>卡密</template>
      </el-input>
      <el-button class="查询按钮" type="primary" :disabled="充值框.加载中 || !!操作中" @click="查询卡密">查询</el-button>
    </el-card>

    <el-alert
      v-if="批量精确查询 && 已查询 && 批量未找到.length"
      class="批量未找到"
      type="warning"
      :closable="false"
      title="以下卡密未找到"
      :description="批量未找到.join('、')"
    />

    <el-card v-if="模式 === 'point' && (批量精确查询 || 查卡分页.total > 1 || (已查询 && !加载中 && 候选列表.length && !详情))" class="结果卡片" shadow="never">
      <h3>匹配的点卡（共 {{ 查卡分页.total }} 张）</h3>
      <el-table :data="候选列表" border stripe>
        <el-table-column prop="card" label="卡密" min-width="190" show-overflow-tooltip />
        <el-table-column label="软件" width="80">
          <template #default="scope">#{{ scope.row.software }}</template>
        </el-table-column>
        <el-table-column label="点数余额" width="110">
          <template #default="scope">{{ scope.row.point_balance }} 点</template>
        </el-table-column>
        <el-table-column label="状态" width="85">
          <template #default="scope">
            <el-tag :type="scope.row.card_state === 4 ? 'danger' : 'success'">
              {{ scope.row.card_state === 4 ? '冻结' : '正常' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160">
          <template #default="scope">
            <el-button link type="primary" @click="选择点卡(scope.row)">设备授权</el-button>
            <el-button link type="primary" @click="打开流水(scope.row)">查看流水</el-button>
          </template>
        </el-table-column>
      </el-table>
      <p class="说明">选择一张点卡查看设备授权。</p>
    </el-card>

    <el-card v-if="详情?.mode === 'point'" class="结果卡片" shadow="never">
      <el-table :data="[详情]" border stripe>
        <el-table-column prop="card" label="卡密" min-width="190" show-overflow-tooltip />
        <el-table-column label="软件" width="80">
          <template #default="scope">#{{ scope.row.software }}</template>
        </el-table-column>
        <el-table-column label="点数余额" width="110">
          <template #default="scope">{{ scope.row.point_balance }} 点</template>
        </el-table-column>
        <el-table-column prop="authorized_device_count" label="授权设备" width="95" />
        <el-table-column prop="online_device_count" label="在线设备" width="95" />
        <el-table-column label="状态" width="85">
          <template #default="scope">
            <el-tag :type="scope.row.card_state === 4 ? 'danger' : 'success'">
              {{ scope.row.card_state === 4 ? '冻结' : '正常' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="95">
          <template #default="scope">
            <el-button link type="primary" @click="打开流水(scope.row)">查看流水</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-divider content-position="left">设备授权</el-divider>
      <el-table :data="详情.devices" border stripe empty-text="暂无设备会话">
        <el-table-column prop="device_id" label="设备 ID" min-width="180" show-overflow-tooltip />
        <el-table-column prop="device_alias" label="设备别名" min-width="130" show-overflow-tooltip />
        <el-table-column label="授权到期" width="180">
          <template #default="scope">{{ 格式化时间(scope.row.authorized_until) || '-' }}</template>
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
        @current-change="page => 选择点卡(详情.card, page)"
      />
    </el-card>

    <el-card v-if="模式 === 'duration' && 候选列表.length" class="结果卡片" shadow="never">
      <div class="结果标题">
        <h3>时长卡（共 {{ 查卡分页.total }} 张）</h3>
        <el-button type="primary" :disabled="!已选时长卡.length || 已选时长卡.length > 500 || !!操作中 || 充值框.加载中" @click="打开批量充值">批量充值（{{ 已选时长卡.length }}）</el-button>
      </div>
      <el-table :key="时长卡表版本" :data="候选列表" border stripe row-key="card" @selection-change="已选时长卡 = $event">
        <el-table-column type="selection" width="48" :selectable="可充值时长卡" reserve-selection />
        <el-table-column prop="card" label="卡密" min-width="190" show-overflow-tooltip />
        <el-table-column label="软件" width="75">
          <template #default="scope">#{{ scope.row.software }}</template>
        </el-table-column>
        <el-table-column label="卡面时长" width="125">
          <template #default="scope">{{ 时长文本(scope.row.duration_minutes) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="scope">
            <el-tag :type="scope.row.card_state === 4 ? 'danger' : scope.row.status === '已激活' ? 'success' : 'info'">
              {{ scope.row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="使用时间" width="170">
          <template #default="scope">{{ 格式化时间(scope.row.use_time) || '-' }}</template>
        </el-table-column>
        <el-table-column label="到期时间" width="170">
          <template #default="scope">{{ 格式化时间(scope.row.end_time) || '-' }}</template>
        </el-table-column>
        <el-table-column label="暂停剩余" width="120">
          <template #default="scope">{{ scope.row.status === '已暂停' ? 时长文本(scope.row.paused_remaining_minutes) : '-' }}</template>
        </el-table-column>
        <el-table-column label="在线" width="70">
          <template #default="scope">{{ scope.row.online ? '在线' : '离线' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="190" fixed="right">
          <template #default="scope">
            <el-button v-if="scope.row.status === '已激活'" link type="warning" :loading="操作中 === `pause:${scope.row.card}`" :disabled="!!操作中 || 充值框.加载中" @click="暂停时长卡(scope.row)">暂停</el-button>
            <el-button v-if="scope.row.status === '已暂停'" link type="success" :loading="操作中 === `resume:${scope.row.card}`" :disabled="!!操作中 || 充值框.加载中" @click="恢复时长卡(scope.row)">恢复</el-button>
            <el-button v-if="可充值时长卡(scope.row)" link type="primary" :disabled="!!操作中 || 充值框.加载中" @click="打开充值(scope.row)">使用充值卡</el-button>
          </template>
        </el-table-column>
      </el-table>
      <p class="说明">已激活、已到期或有暂停剩余时长的卡可充值；批量最多 500 张且须属于同一软件，翻页时已选卡会保留。</p>
    </el-card>

    <el-empty
      v-if="已查询 && !加载中 && !候选列表.length && 查卡分页.total === 0"
      :description="模式 === 'point' ? '没有找到匹配的点卡' : '没有找到匹配的时长卡'"
    />

    <el-pagination
      v-if="查卡分页.total > 查卡分页.page_size"
      v-model:current-page="查卡分页.page"
      class="分页 查卡分页"
      layout="total, prev, pager, next"
      :total="查卡分页.total"
      :page-size="查卡分页.page_size"
      :disabled="加载中 || !!操作中 || 充值框.加载中"
      @current-change="page => 查询卡密(page, true)"
    />

    <el-card v-if="模式 === 'duration'" class="结果卡片" shadow="never">
      <h3>查询时长充值卡</h3>
      <div class="充值卡查询行">
        <el-input v-model="充值卡查询.card" clearable placeholder="请输入完整充值卡卡密" @input="充值卡查询.detail = null" @keyup.enter="查询充值卡" />
        <el-button :loading="充值卡查询.加载中" @click="查询充值卡">查询</el-button>
      </div>
      <el-descriptions v-if="充值卡查询.detail" :column="3" border class="充值卡信息">
        <el-descriptions-item label="卡密">{{ 充值卡查询.detail.card }}</el-descriptions-item>
        <el-descriptions-item label="软件">#{{ 充值卡查询.detail.software }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ 充值卡查询.detail.status }}</el-descriptions-item>
        <el-descriptions-item label="每次增加">{{ 时长文本(充值卡查询.detail.duration_minutes) }}</el-descriptions-item>
        <el-descriptions-item label="初始次数">{{ 充值卡查询.detail.initial_uses }}</el-descriptions-item>
        <el-descriptions-item label="剩余次数">{{ 充值卡查询.detail.remaining_uses }}</el-descriptions-item>
        <el-descriptions-item label="到期时间">{{ 格式化时间(充值卡查询.detail.expires_at) || '不限' }}</el-descriptions-item>
      </el-descriptions>
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

    <el-dialog v-model="充值框.显示" title="使用时长充值卡" width="520px" :close-on-click-modal="!充值框.加载中" :close-on-press-escape="!充值框.加载中" :show-close="!充值框.加载中">
      <p class="对话框说明">目标卡 {{ 充值框.cards.length }} 张：{{ 充值框.cards.join('、') }}；充值卡须与目标卡同软件。</p>
      <el-input v-model="充值框.card" clearable :disabled="充值框.加载中 || !!充值框.result" placeholder="请输入充值卡卡密" @keyup.enter="使用充值卡" />
      <el-alert v-if="充值框.result" class="充值结果" :type="充值框.result.failed.length ? 'warning' : 'success'" :closable="false">
        <template #title>成功 {{ 充值框.result.success.length }} 张，失败 {{ 充值框.result.failed.length }} 张；充值卡剩余 {{ 充值框.result.remaining_uses }} 次</template>
        <div v-if="充值框.result.failed.length">失败卡密：{{ 充值框.result.failed.join('、') }}</div>
      </el-alert>
      <template #footer>
        <el-button :disabled="充值框.加载中" @click="充值框.显示 = false">{{ 充值框.result ? '关闭' : '取消' }}</el-button>
        <el-button v-if="!充值框.result" type="primary" :loading="充值框.加载中" @click="使用充值卡">确认充值</el-button>
      </template>
    </el-dialog>
  </main>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import apiClient, { 获取接口错误提示, 规范化卡密 } from './api/请求客户端.js'
import { 格式化时间, 格式化时长 as 时长文本 } from './utils/时长工具.js'

const centerID = new URLSearchParams(window.location.search).get('center_id') || ''
const 读取上次卡种 = function () {
  try {
    return window.localStorage.getItem('card_query_mode') === 'duration' ? 'duration' : 'point'
  } catch {
    return 'point'
  }
}
const 卡密 = ref('')
const 模式 = ref(读取上次卡种())
const 加载中 = ref(false)
const 已查询 = ref(false)
const 候选列表 = ref([])
const 已选时长卡 = ref([])
const 时长卡表版本 = ref(0)
const 批量精确查询 = ref(false)
const 批量未找到 = ref([])
const 详情 = ref(null)
const 操作中 = ref('')
const 查卡分页 = reactive({ page: 1, page_size: 20, total: 0 })
const 流水框 = reactive({ 显示: false, 加载中: false, card: '', software: 0, rows: [], balance: 0, page: 1, page_size: 20, total: 0 })
const 设备分页 = reactive({ page: 1, page_size: 50, total: 0 })
const 充值卡查询 = reactive({ card: '', 加载中: false, detail: null })
const 充值框 = reactive({ 显示: false, 加载中: false, card: '', cards: [], rows: [], result: null })
let 查询序号 = 0
const 错误 = (error) => ElMessage.error(获取接口错误提示(error))
const 事件名称 = (type) => (type === 'debit' ? '扣点' : type === 'credit' ? '补点' : type || '')

const 切换模式 = function () {
  try {
    window.localStorage.setItem('card_query_mode', 模式.value)
  } catch {
    // 浏览器禁用本地存储时仍可正常切换和查询。
  }
  查询序号++
  加载中.value = false
  已查询.value = false
  候选列表.value = []
  已选时长卡.value = []
  批量精确查询.value = false
  批量未找到.value = []
  查卡分页.page = 1
  查卡分页.total = 0
  时长卡表版本.value++
  详情.value = null
  流水框.显示 = false
  充值卡查询.detail = null
}

const 读取点卡详情 = async function (card, page, sequence) {
  const res = await apiClient.post('/point_card/query', { center_id: centerID, card, page, page_size: 设备分页.page_size })
  if (!res.data?.state) throw new Error(res.data?.msg || '查询点卡失败')
  if (sequence !== 查询序号 || 模式.value !== 'point') return
  详情.value = {
    mode: 'point', card,
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
}

const 查询卡密 = async function (page = 1, 保留当前结果 = false) {
  if (加载中.value || 操作中.value || 充值框.加载中) return
  if (!centerID) {
    ElMessage.error('查询链接缺少 center_id，请联系管理员获取完整链接')
    return
  }
  const rawKeyword = String(卡密.value ?? '').trim().toLowerCase()
  const isBatch = rawKeyword.includes(',') || rawKeyword.includes('，')
  let batchCards = []
  let keyword = rawKeyword
  if (isBatch) {
    batchCards = [...new Set(rawKeyword.split(/[,，]/).map((item) => 规范化卡密(item)).filter(Boolean))]
    if (!batchCards.length || batchCards.length > 20 || rawKeyword.split(/[,，]/).some((item) => item.trim() && !规范化卡密(item))) {
      ElMessage.warning('批量查询请用逗号分隔1至20张完整卡密')
      return
    }
    keyword = batchCards.join(',')
  } else if (!/^[a-z0-9_-]{4,60}\*{3}$/.test(keyword) && !规范化卡密(keyword)) {
    ElMessage.warning('请输入完整卡密，或输入至少4位前缀并加上***')
    return
  }
  卡密.value = keyword
  const sequence = ++查询序号
  const mode = 模式.value
  加载中.value = true
  if (保留当前结果) {
    查卡分页.page = page
  } else {
    page = 1
    已查询.value = false
    候选列表.value = []
    已选时长卡.value = []
    批量精确查询.value = isBatch
    批量未找到.value = []
    查卡分页.page = 1
    查卡分页.total = 0
    时长卡表版本.value++
    详情.value = null
  }
  try {
    const res = await apiClient.post('/visitor/查询所有卡密', {
      center_id: centerID, card: keyword, mode, page, page_size: 查卡分页.page_size
    })
    if (!res.data?.state) throw new Error(res.data?.msg || '查询卡密失败')
    if (sequence !== 查询序号) return
    候选列表.value = Array.isArray(res.data.data) ? res.data.data : []
    if (res.data.num !== undefined && res.data.num !== null) {
      查卡分页.total = Number(res.data.num)
    }
    查卡分页.page = Number(res.data.page || page)
    查卡分页.page_size = Number(res.data.page_size || 查卡分页.page_size)
    已查询.value = true
    批量未找到.value = isBatch
      ? batchCards.filter((card) => !候选列表.value.some((row) => row.card === card))
      : []
    // 只有明确选中的一张点卡才请求设备详情，避免批量前缀查询逐张读取设备。
    if (!isBatch && mode === 'point' && 查卡分页.total === 1 && 候选列表.value.length === 1) {
      await 读取点卡详情(候选列表.value[0].card, 1, sequence)
    }
  } catch (error) {
    if (sequence === 查询序号) 错误(error)
  } finally {
    if (sequence === 查询序号) 加载中.value = false
  }
}

const 选择点卡 = async function (row, page = 1) {
  if (加载中.value || 操作中.value) return
  const card = typeof row === 'string' ? row : row?.card
  if (!card) return
  const sequence = ++查询序号
  if (详情.value?.card !== card) 详情.value = null
  加载中.value = true
  try {
    await 读取点卡详情(card, page, sequence)
  } catch (error) {
    if (sequence === 查询序号) 错误(error)
  } finally {
    if (sequence === 查询序号) 加载中.value = false
  }
}

const 打开流水 = function (row) {
  if (!row?.card) return
  流水框.card = row.card
  流水框.software = row.software
  流水框.balance = Number(row.point_balance || 0)
  流水框.显示 = true
  流水框.page = 1
  流水框.rows = []
  流水框.total = 0
  查询流水(true)
}

const 查询流水 = async function (resetPage = false) {
  if (!流水框.card) return
  if (resetPage) 流水框.page = 1
  const card = 流水框.card
  const page = 流水框.page
  流水框.加载中 = true
  try {
    const res = await apiClient.post('/point_card/point_ledger/query', {
      center_id: centerID, card, software: 流水框.software,
      page, page_size: 流水框.page_size
    })
    if (!res.data?.state) throw new Error(res.data?.msg || '查询流水失败')
    if (流水框.card !== card || 流水框.page !== page) return
    流水框.rows = Array.isArray(res.data.data) ? res.data.data : []
    流水框.total = Number(res.data.num || 0)
    流水框.balance = Number(res.data.balance ?? 流水框.balance)
  } catch (error) {
    if (流水框.card === card) 错误(error)
  } finally {
    if (流水框.card === card) 流水框.加载中 = false
  }
}

// 服务端允许正常且已激活/已到期的卡，或仍有暂停余额的卡充值。
const 可充值时长卡 = (row) => ['已激活', '已到期'].includes(row?.status) || (row?.status === '已暂停' && Number(row.paused_remaining_minutes) > 0)

const 刷新时长卡详情 = async function (card) {
  const res = await apiClient.post('/visitor/查询时长卡', { center_id: centerID, card })
  if (!res.data?.state) throw new Error(res.data?.msg || '刷新时长卡失败')
  if (模式.value !== 'duration') return
  候选列表.value = 候选列表.value.map((row) => row.card === card ? res.data.data : row)
}

const 执行时长卡操作 = async function (row, action) {
  if (!row?.card || 操作中.value) return
  操作中.value = `${action}:${row.card}`
  try {
    const res = await apiClient.post(`/visitor/duration_card/${action}`, { center_id: centerID, card: row.card })
    if (!res.data?.state) throw new Error(res.data?.msg || '操作失败')
    ElMessage.success(res.data.msg || '操作成功')
    await 刷新时长卡详情(row.card)
  } catch (error) {
    错误(error)
  } finally {
    操作中.value = ''
  }
}
const 暂停时长卡 = (row) => 执行时长卡操作(row, 'pause')
const 恢复时长卡 = (row) => 执行时长卡操作(row, 'resume')

const 读取充值卡信息 = async function (rawCard) {
  const card = 规范化卡密(rawCard)
  if (!card) throw new Error('请输入7至63位完整充值卡卡密')
  if (!centerID) throw new Error('查询链接缺少 center_id')
  const res = await apiClient.post('/visitor/duration_recharge_card/query', { center_id: centerID, card })
  if (!res.data?.state) throw new Error(res.data?.msg || '查询充值卡失败')
  return res.data.data
}

const 查询充值卡 = async function () {
  if (充值卡查询.加载中) return
  充值卡查询.detail = null
  充值卡查询.加载中 = true
  try {
    const info = await 读取充值卡信息(充值卡查询.card)
    充值卡查询.card = info.card
    充值卡查询.detail = info
  } catch (error) {
    错误(error)
  } finally {
    充值卡查询.加载中 = false
  }
}

const 打开充值 = function (row) {
  if (!可充值时长卡(row)) return
  充值框.cards = [row.card]
  充值框.rows = [row]
  充值框.card = ''
  充值框.result = null
  充值框.显示 = true
}

const 打开批量充值 = function () {
  if (!已选时长卡.value.length) return
  if (已选时长卡.value.length > 500) {
    ElMessage.warning('批量充值最多选择500张时长卡')
    return
  }
  const software = 已选时长卡.value[0].software
  if (已选时长卡.value.some((row) => row.software !== software)) {
    ElMessage.warning('批量充值只能选择同一软件的时长卡')
    return
  }
  充值框.cards = 已选时长卡.value.map((row) => row.card)
  充值框.rows = [...已选时长卡.value]
  充值框.card = ''
  充值框.result = null
  充值框.显示 = true
}

const 刷新时长卡列表 = async function () {
  const keyword = String(卡密.value ?? '').trim().toLowerCase()
  const res = await apiClient.post('/visitor/查询所有卡密', {
    center_id: centerID, card: keyword, mode: 'duration',
    page: 查卡分页.page, page_size: 查卡分页.page_size
  })
  if (!res.data?.state) throw new Error(res.data?.msg || '刷新时长卡失败')
  候选列表.value = Array.isArray(res.data.data) ? res.data.data : []
  if (res.data.num !== undefined && res.data.num !== null) {
    查卡分页.total = Number(res.data.num)
  }
  查卡分页.page = Number(res.data.page || 查卡分页.page)
  查卡分页.page_size = Number(res.data.page_size || 查卡分页.page_size)
  已选时长卡.value = []
  时长卡表版本.value++
}

const 使用充值卡 = async function () {
  if (充值框.加载中 || 充值框.result) return
  const sourceCard = 规范化卡密(充值框.card)
  const cards = [...充值框.cards]
  if (!sourceCard || !cards.length) {
    ElMessage.warning('请输入格式正确的充值卡，并选择目标时长卡')
    return
  }
  充值框.card = sourceCard
  充值框.加载中 = true
  try {
    const info = await 读取充值卡信息(sourceCard)
    if (info.status !== '正常') throw new Error(`充值卡${info.status}，不能使用`)
    if (Number(info.remaining_uses) < cards.length) throw new Error(`充值卡剩余次数不足，需要 ${cards.length} 次`)
    const targets = 充值框.rows
    if (targets.length !== cards.length || targets.some((row) => !可充值时长卡(row) || row.software !== info.software)) {
      throw new Error('目标时长卡状态或所属软件与充值卡不匹配，请重新查询')
    }
    try {
      await ElMessageBox.confirm(
        `给 ${cards.length} 张卡各增加 ${时长文本(info.duration_minutes)}，预计使用 ${cards.length} 次；充值卡当前剩余 ${info.remaining_uses} 次。确认充值？`,
        '确认时长充值', { type: 'warning', confirmButtonText: '确认充值', cancelButtonText: '取消' }
      )
    } catch {
      return
    }
    const res = await apiClient.post('/visitor/duration_recharge_card/redeem', {
      center_id: centerID, recharge_card: sourceCard, cards
    })
    if (!res.data?.state) throw new Error(res.data?.msg || '充值失败')
    充值框.result = {
      success: Array.isArray(res.data.success) ? res.data.success : [],
      failed: Array.isArray(res.data.failed) ? res.data.failed : [],
      remaining_uses: Number(res.data.remaining_uses || 0)
    }
    if (充值框.result.failed.length) ElMessage.warning(res.data.msg || '部分卡密充值失败')
    else ElMessage.success(res.data.msg || '充值成功')
    if (充值卡查询.detail?.card === sourceCard) 充值卡查询.detail.remaining_uses = 充值框.result.remaining_uses
    if (充值框.result.success.length) {
      try {
        await 刷新时长卡列表()
      } catch {
        ElMessage.warning('充值已完成，但列表刷新失败，请重新查询')
      }
    }
  } catch (error) {
    错误(error)
  } finally {
    充值框.加载中 = false
  }
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
  max-width: 1280px;
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
.结果标题 {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}
.结果标题 h3 {
  margin: 0;
}
.充值卡查询行 {
  display: flex;
  gap: 12px;
}
.充值卡信息,
.充值结果 {
  margin-top: 16px;
}
.对话框说明 {
  margin: 0 0 12px;
  color: #9da7b5;
  line-height: 1.6;
  overflow-wrap: anywhere;
}
.模式选择 {
  display: flex;
  margin-bottom: 12px;
}
.说明 {
  color: #9da7b5;
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
@media (max-width: 600px) {
  .结果标题,
  .充值卡查询行 {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
