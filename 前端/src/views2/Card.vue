<template>
  <section class="页面" v-loading="加载中">
    <div class="页面标题行">
      <div>
        <h2>点卡管理</h2>
        <p class="说明">一张卡可在多台设备使用；同一设备在授权时长内重复登录不会重复扣点。</p>
      </div>
      <el-button type="primary" @click="打开生成">生成卡密</el-button>
    </div>

    <el-card shadow="never" class="筛选卡片">
      <el-form :inline="true" @submit.prevent>
        <el-form-item label="软件">
          <el-select
            v-model="筛选.software"
            clearable
            placeholder="全部软件"
            style="width: 170px"
            @change="查询卡密(true)"
          >
            <el-option v-for="item in 软件列表" :key="item.ID" :label="item.Software" :value="item.ID" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select
            v-model="筛选.card_state"
            clearable
            placeholder="全部状态"
            style="width: 130px"
            @change="查询卡密(true)"
          >
            <el-option label="正常" :value="2" />
            <el-option label="冻结" :value="4" />
          </el-select>
        </el-form-item>
        <el-form-item label="卡密">
          <el-input
            v-model="筛选.card"
            clearable
            placeholder="支持模糊搜索"
            style="width: 210px"
            @keyup.enter="查询卡密(true)"
          />
        </el-form-item>
        <el-form-item label="备注">
          <el-input
            v-model="筛选.notes"
            clearable
            placeholder="支持模糊搜索"
            style="width: 180px"
            @keyup.enter="查询卡密(true)"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="查询卡密(true)">查询</el-button>
          <el-button @click="重置筛选">重置</el-button>
        </el-form-item>
      </el-form>
      <div class="批量操作">
        <span>已选 {{ 已选卡密.length }} 张</span>
        <el-button size="small" type="warning" :disabled="!已选卡密.length" @click="批量修改状态(4)">
          冻结
        </el-button>
        <el-button size="small" type="success" :disabled="!已选卡密.length" @click="批量修改状态(2)">
          解冻
        </el-button>
        <el-button size="small" type="danger" :disabled="!已选卡密.length" @click="批量删除">删除</el-button>
        <el-button size="small" :disabled="!卡密列表.length" @click="导出当前页">导出当前页</el-button>
      </div>
    </el-card>

    <el-table
      ref="卡密表格"
      :data="卡密列表"
      border
      stripe
      row-key="card"
      @selection-change="选择变化"
      @sort-change="卡密排序变化"
    >
      <el-table-column type="selection" width="44" />
      <el-table-column
        prop="card"
        label="卡密"
        min-width="190"
        sortable="custom"
        :sort-orders="['ascending', 'descending']"
        show-overflow-tooltip
      />
      <el-table-column
        prop="software"
        label="软件"
        width="150"
        sortable="custom"
        :sort-orders="['ascending', 'descending']"
        show-overflow-tooltip
      >
        <template #default="scope">{{ 软件名称(scope.row.software) }}</template>
      </el-table-column>
      <el-table-column
        prop="point_balance"
        label="余额"
        width="100"
        align="right"
        sortable="custom"
        :sort-orders="['ascending', 'descending']"
      >
        <template #default="scope">{{ scope.row.point_balance }} 点</template>
      </el-table-column>
      <el-table-column
        prop="card_state"
        label="状态"
        width="80"
        sortable="custom"
        :sort-orders="['ascending', 'descending']"
      >
        <template #default="scope">
          <el-tag :type="scope.row.card_state === 4 ? 'danger' : 'success'">
            {{ scope.row.card_state === 4 ? '冻结' : '正常' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column
        prop="authorized_device_count"
        label="授权设备"
        width="90"
        align="right"
        sortable="custom"
        :sort-orders="['ascending', 'descending']"
      />
      <el-table-column
        prop="create_time"
        label="生成时间"
        width="170"
        sortable="custom"
        :sort-orders="['ascending', 'descending']"
      >
        <template #default="scope">{{ 格式化时间(scope.row.create_time) }}</template>
      </el-table-column>
      <el-table-column
        prop="use_time"
        label="最近扣点"
        width="170"
        sortable="custom"
        :sort-orders="['ascending', 'descending']"
      >
        <template #default="scope">{{ 格式化时间(scope.row.use_time) }}</template>
      </el-table-column>
      <el-table-column prop="notes" label="备注" min-width="160" show-overflow-tooltip />
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="scope">
          <el-button link type="primary" @click="打开流水(scope.row)">流水</el-button>
          <el-button link type="warning" @click="打开编辑(scope.row)">编辑</el-button>
          <el-button link type="danger" @click="删除单张(scope.row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-model:current-page="分页.page"
      v-model:page-size="分页.page_size"
      class="分页"
      background
      layout="total, sizes, prev, pager, next, jumper"
      :page-sizes="[20, 50, 100, 200]"
      :total="分页.total"
      @size-change="查询卡密(true)"
      @current-change="查询卡密(false)"
    />

    <!-- 生成卡密 -->
    <el-dialog v-model="生成框.显示" title="生成卡密" width="560px" destroy-on-close>
      <el-form label-width="100px" v-loading="生成框.加载中">
        <el-form-item label="所属软件" required>
          <el-select v-model="生成框.software" placeholder="请选择软件" style="width: 280px">
            <el-option v-for="item in 可发卡软件列表" :key="item.ID" :label="item.Software" :value="item.ID" />
          </el-select>
        </el-form-item>
        <el-form-item label="每张点数" required>
          <el-input-number
            v-model="生成框.points"
            :min="1"
            :max="1000000000"
            :precision="0"
            controls-position="right"
          />
          <span class="单位">点</span>
        </el-form-item>
        <el-form-item label="生成数量" required>
          <el-input-number v-model="生成框.num" :min="1" :max="1000" :precision="0" controls-position="right" />
          <span v-if="是代理账号" class="费用提示">预计消耗合伙人余额 {{ 预计代理费用 }} 点</span>
        </el-form-item>
        <el-form-item label="生成方式">
          <el-radio-group v-model="生成框.random">
            <el-radio :label="true">随机生成</el-radio>
            <el-radio :label="false">指定卡密</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="!生成框.random" label="卡密内容" required>
          <el-input v-model="生成框.cards" type="textarea" :rows="4" placeholder="每行一张，也可用逗号分隔" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="生成框.notes" maxlength="500" show-word-limit />
        </el-form-item>
        <el-form-item label="配置">
          <el-input v-model="生成框.config_content" type="textarea" :rows="3" maxlength="200" show-word-limit placeholder="留空表示无配置" />
        </el-form-item>
        <el-form-item v-if="生成框结果" label="生成结果">
          <el-input v-model="生成框结果" type="textarea" :rows="5" readonly />
          <el-button class="复制按钮" @click="复制文本(生成框结果)">复制</el-button>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="生成框.显示 = false">取消</el-button>
        <el-button type="primary" @click="生成卡密">生成</el-button>
      </template>
    </el-dialog>

    <!-- 编辑点卡及管理员补扣点 -->
    <el-dialog v-model="编辑框.显示" title="编辑点卡" width="520px" destroy-on-close>
      <el-form label-width="100px" v-loading="编辑框.加载中">
        <el-form-item label="卡密">
          <span class="卡密文本">{{ 编辑框.card }}</span>
        </el-form-item>
        <el-form-item label="所属软件">
          <span>{{ 软件名称(编辑框.software) }}</span>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="编辑框.card_state">
            <el-radio :label="2">正常</el-radio>
            <el-radio :label="4">冻结</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="编辑框.notes" maxlength="500" /></el-form-item>
        <el-form-item label="配置">
          <el-input v-model="编辑框.config_content" type="textarea" :rows="3" maxlength="200" show-word-limit />
        </el-form-item>
        <template v-if="!是代理账号">
          <el-divider content-position="left">调整余额</el-divider>
          <el-form-item label="当前余额">
            <span>{{ 编辑框.point_balance }} 点</span>
          </el-form-item>
          <el-form-item label="变动点数">
            <el-input-number
              v-model="调整.amount"
              :min="-1000000000"
              :max="1000000000"
              :precision="0"
              controls-position="right"
            />
          </el-form-item>
          <el-form-item label="调整原因">
            <el-input v-model="调整.reason" maxlength="255" placeholder="例如：人工补点" />
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="编辑框.显示 = false">取消</el-button>
        <el-button v-if="!是代理账号" type="warning" :disabled="!调整.amount" @click="调整余额">
          调整余额
        </el-button>
        <el-button type="primary" @click="保存编辑">保存</el-button>
      </template>
    </el-dialog>

    <!-- 点数流水 -->
    <el-dialog v-model="流水框.显示" title="点数流水" width="920px" destroy-on-close>
      <div class="流水摘要">卡密：{{ 流水框.card }}　当前余额：{{ 流水框.balance }} 点</div>
      <el-table :data="流水框.rows" border v-loading="流水框.加载中">
        <el-table-column prop="created_at" label="时间" width="170" />
        <el-table-column label="类型" width="90">
          <template #default="scope">{{ 事件名称(scope.row.event_type) }}</template>
        </el-table-column>
        <el-table-column prop="change" label="变动" width="80" align="right">
          <template #default="scope">
            <span :class="scope.row.change > 0 ? '增加' : '扣除'">
              {{ scope.row.change > 0 ? '+' : '' }}{{ scope.row.change }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="余额" width="130">
          <template #default="scope">{{ scope.row.balance_before }} → {{ scope.row.balance_after }}</template>
        </el-table-column>
        <el-table-column prop="remark" label="备注（含设备信息）" min-width="330" show-overflow-tooltip />
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
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { use登录状态Store } from '../stores/登录状态.js'
import { 获取接口错误提示 } from '../api/请求客户端.js'

const stores = use登录状态Store()
const post = stores.post
const 是代理账号 = computed(() => stores.是代理账号)
const 账号信息 = stores.账号信息
const 加载中 = ref(false)
const 卡密表格 = ref(null)
const 软件列表 = ref([])
const 卡密列表 = ref([])
const 已选卡密 = ref([])
const 分页 = reactive({ page: 1, page_size: 50, total: 0 })
const 筛选 = reactive({ software: '', card_state: '', card: '', notes: '' })
// 卡密是特殊排序键：单独点击卡密时只按卡密；点击其他字段时，卡密作为
// 第二排序键。card_order 独立保存，保证用户切换主排序后卡密方向不丢失。
const 排序 = reactive({ sort_by: '', sort_order: '', card_order: 'asc' })
const 生成框 = reactive({
  显示: false,
  加载中: false,
  software: 0,
  points: 100,
  num: 1,
  random: true,
  cards: '',
  notes: '',
  config_content: ''
})
const 生成框结果 = ref('')
const 编辑框 = reactive({
  显示: false,
  加载中: false,
  card: '',
  software: 0,
  card_state: 2,
  point_balance: 0,
  notes: '',
  config_content: ''
})
const 调整 = reactive({ amount: 0, reason: '' })
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

const 预计代理费用 = computed(() => {
  const prices = 账号信息.prices || {}
  const value = Number(prices[String(生成框.software)] ?? prices[生成框.software] ?? 0)
  if (!Number.isFinite(value)) return 0
  // 与后端一致：先把浮点乘法修正到分，再将整批费用按整数点向上取整。
  const total = value * Number(生成框.points || 0) * Number(生成框.num || 0)
  return Math.ceil(Math.round(total * 100) / 100)
})
const 可发卡软件列表 = computed(() => {
  if (!是代理账号.value) return 软件列表.value
  const prices = 账号信息.prices || {}
  return 软件列表.value.filter((item) => Number(prices[String(item.ID)] ?? prices[item.ID] ?? 0) > 0)
})

const 格式化时间 = (value) => {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) || date.getFullYear() <= 1 ? '' : date.toLocaleString()
}
const 软件名称 = (id) => 软件列表.value.find((item) => Number(item.ID) === Number(id))?.Software || `软件#${id}`
const 事件名称 = (type) => (type === 'debit' ? '扣点' : type === 'credit' ? '补点' : type || '')
const 显示错误 = (error) => ElMessage.error(获取接口错误提示(error))

const 查询软件 = function () {
  return post('/user_query_soft_list', {}).then((res) => {
    if (!res.data?.state) throw new Error(res.data?.msg || '查询软件失败')
    软件列表.value = res.data.data || []
    if (是代理账号.value) {
      账号信息.balance = Number(res.data.balance || 0)
      账号信息.prices = res.data.prices || {}
    }
    if (!可发卡软件列表.value.some((item) => Number(item.ID) === Number(生成框.software))) {
      生成框.software = 可发卡软件列表.value[0]?.ID || 0
    }
  })
}

const 查询卡密 = function (resetPage = false) {
  if (resetPage) 分页.page = 1
  加载中.value = true
  return post('/user_query_card', {
    ...筛选,
    ...排序,
    page: 分页.page,
    page_size: 分页.page_size
  })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '查询卡密失败')
      卡密列表.value = res.data.data || []
      分页.total = Number(res.data.num || 0)
      已选卡密.value = []
    })
    .catch(显示错误)
    .finally(() => {
      加载中.value = false
    })
}

const 卡密排序变化 = function ({ prop, order }) {
  if (!prop || !order) return
  const direction = order === 'ascending' ? 'asc' : 'desc'
  排序.sort_by = prop
  排序.sort_order = direction
  if (prop === 'card') 排序.card_order = direction
  查询卡密(true)
}

const 重置筛选 = function () {
  Object.assign(筛选, { software: '', card_state: '', card: '', notes: '' })
  Object.assign(排序, { sort_by: '', sort_order: '', card_order: 'asc' })
  卡密表格.value?.clearSort()
  查询卡密(true)
}
const 选择变化 = (rows) => {
  已选卡密.value = rows.map((row) => row.card)
}

const 打开生成 = function () {
  if (!可发卡软件列表.value.length) {
    ElMessage.warning(是代理账号.value ? '管理员尚未为该渠道合伙人配置可发卡软件' : '请先创建软件')
    return
  }
  Object.assign(生成框, {
    显示: true,
    software: 生成框.software || 可发卡软件列表.value[0].ID,
    points: 100,
    num: 1,
    random: true,
    cards: '',
    notes: '',
    config_content: ''
  })
  生成框结果.value = ''
}
const 生成卡密 = function () {
  if (!生成框.software || 生成框.points <= 0 || 生成框.num <= 0) {
    ElMessage.warning('请选择软件并填写有效的点数和数量')
    return
  }
  if (!生成框.random && !生成框.cards.trim()) {
    ElMessage.warning('请输入指定卡密')
    return
  }
  生成框.加载中 = true
  post('/add_new_card', {
    software: 生成框.software,
    points: 生成框.points,
    num: 生成框.num,
    cards: 生成框.cards,
    random: 生成框.random,
    notes: 生成框.notes,
    config_content: 生成框.config_content
  })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '生成失败')
      生成框结果.value = res.data.data || ''
      if (是代理账号.value && res.data.balance !== undefined) {
        // 代理生成卡密的扣款与卡密写入在同一事务完成，直接采用服务端
        // 返回的最终余额，避免顶部余额一直停留在登录时的旧值。
        账号信息.balance = Number(res.data.balance || 0)
      }
      ElMessage.success(res.data.msg || '生成成功')
      查询卡密(true)
    })
    .catch(显示错误)
    .finally(() => {
      生成框.加载中 = false
    })
}

const 打开编辑 = function (row) {
  Object.assign(编辑框, {
    显示: true,
    card: row.card,
    software: row.software,
    card_state: row.card_state,
    point_balance: Number(row.point_balance || 0),
    notes: row.notes || '',
    config_content: row.config_content || ''
  })
  Object.assign(调整, { amount: 0, reason: '' })
}
const 保存编辑 = function () {
  编辑框.加载中 = true
  post('/modify_card', {
    card: 编辑框.card,
    card_state: 编辑框.card_state,
    notes: 编辑框.notes,
    config_content: 编辑框.config_content
  })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '保存失败')
      ElMessage.success('保存成功')
      编辑框.显示 = false
      查询卡密(false)
    })
    .catch(显示错误)
    .finally(() => {
      编辑框.加载中 = false
    })
}
const 调整余额 = function () {
  if (!调整.amount || !调整.reason.trim()) {
    ElMessage.warning('请填写变动点数和调整原因')
    return
  }
  编辑框.加载中 = true
  post('/point_card/adjust', { card: 编辑框.card, amount: 调整.amount, reason: 调整.reason })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '调整失败')
      编辑框.point_balance = Number(res.data.balance || 0)
      调整.amount = 0
      调整.reason = ''
      ElMessage.success('余额调整成功')
      查询卡密(false)
    })
    .catch(显示错误)
    .finally(() => {
      编辑框.加载中 = false
    })
}

const 批量修改状态 = function (state) {
  if (!已选卡密.value.length) return
  post('/冻卡s', { cards: 已选卡密.value, card_state: state })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '操作失败')
      ElMessage.success(res.data.msg || '操作成功')
      查询卡密(false)
    })
    .catch(显示错误)
}
const 删除单张 = function (row) {
  ElMessageBox.confirm(`确定删除卡密 ${row.card}？删除后同名卡密可以再次生成，历史流水会保留。`, '确认删除', {
    type: 'warning'
  })
    .then(() => post('/delete_card', { cards: [row.card] }))
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '删除失败')
      ElMessage.success('删除成功')
      查询卡密(false)
    })
    .catch((error) => {
      if (error !== 'cancel' && error !== 'close') 显示错误(error)
    })
}
const 批量删除 = function () {
  ElMessageBox.confirm(`确定删除已选的 ${已选卡密.value.length} 张点卡？`, '确认删除', { type: 'warning' })
    .then(() => post('/delete_card', { cards: 已选卡密.value }))
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '删除失败')
      ElMessage.success(res.data.msg || '删除完成')
      查询卡密(false)
    })
    .catch((error) => {
      if (error !== 'cancel' && error !== 'close') 显示错误(error)
    })
}

const 打开流水 = function (row) {
  Object.assign(流水框, {
    显示: true,
    card: row.card,
    software: row.software,
    balance: Number(row.point_balance || 0),
    rows: [],
    page: 1,
    total: 0
  })
  查询流水(true)
}
const 查询流水 = function (resetPage = false) {
  if (resetPage) 流水框.page = 1
  流水框.加载中 = true
  // 按卡密查看时不附带当前软件筛选，保证删除后重用同名卡密时，
  // 旧一代流水（即使属于旧软件）也能完整显示。
  return post('/point_ledger/query', { card: 流水框.card, page: 流水框.page, page_size: 流水框.page_size })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '查询流水失败')
      流水框.rows = res.data.data || []
      流水框.total = Number(res.data.num || 0)
      流水框.balance = Number(res.data.balance ?? 流水框.balance)
    })
    .catch(显示错误)
    .finally(() => {
      流水框.加载中 = false
    })
}

const 复制文本 = async function (value) {
  try {
    await navigator.clipboard.writeText(value || '')
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = value || ''
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    textarea.remove()
  }
  ElMessage.success('已复制')
}
const 导出当前页 = function () {
  const text = 卡密列表.value.map((row) => row.card).join('\n')
  if (!text) return
  // 直接下载文本文件比只复制到剪贴板更适合批量发卡；同时保留复制按钮
  // 给临时使用场景，文件名带日期便于管理员归档。
  const blob = new Blob([text + '\n'], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `点卡-${new Date().toISOString().slice(0, 10)}.txt`
  document.body.appendChild(link)
  link.click()
  link.remove()
  // 部分浏览器需要在 click 返回后再释放对象 URL，避免下载内容为空。
  window.setTimeout(() => URL.revokeObjectURL(url), 0)
  ElMessage.success('已导出当前页')
}

onMounted(() => {
  Promise.all([查询软件(), 查询卡密(true)]).catch(() => {})
})
</script>

<style scoped>
.页面 {
  padding: 16px;
  color: #e6eaf2;
}
.标题行 {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
h2 {
  margin: 0 0 6px;
}
.说明 {
  margin: 0;
  color: #aeb6c3;
  font-size: 13px;
}
.筛选卡片 {
  margin-bottom: 12px;
}
.批量操作 {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  color: #9099a8;
}
.批量操作 span {
  margin-right: 6px;
}
.分页 {
  margin-top: 14px;
  justify-content: flex-end;
}
.单位,
.费用提示 {
  margin-left: 8px;
  color: #9099a8;
}
.卡密文本 {
  font-family: monospace;
  color: #67c23a;
}
.复制按钮 {
  margin-top: 8px;
}
.流水摘要 {
  margin-bottom: 10px;
  color: #c7ced9;
}
.增加 {
  color: #67c23a;
}
.扣除 {
  color: #f56c6c;
}
</style>
