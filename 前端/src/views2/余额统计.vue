<template>
  <section class="统计页面" v-loading="加载中">
    <el-card shadow="never" class="筛选卡片">
      <el-form :inline="true" @submit.prevent="查询统计">
        <el-form-item label="统计对象">
          <el-select v-model="筛选.kind" style="width: 140px" @change="切换对象">
            <el-option label="点卡" value="point" />
            <el-option label="小伙伴账户" value="agent" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="筛选.kind === 'point'" label="软件">
          <el-select v-model="筛选.software" clearable filterable placeholder="全部软件" style="width: 160px">
            <el-option v-for="item in 软件列表" :key="item.ID" :value="item.ID" :label="item.Software" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="筛选.kind === 'point'" label="卡密">
          <el-input v-model="筛选.card" clearable placeholder="留空统计全部，填写完整卡密" style="width: 240px" />
        </el-form-item>
        <el-form-item v-if="!是代理账号" label="小伙伴">
          <el-select v-model="筛选.agent_id" clearable filterable allow-create default-first-option placeholder="全部 / 输入历史账户ID" style="width: 220px">
            <el-option v-for="item in 代理列表" :key="item.id" :value="item.id" :label="`${item.name}（${item.id}）`" />
          </el-select>
        </el-form-item>
        <el-form-item label="日期" class="日期筛选">
          <el-date-picker v-model="日期" type="daterange" value-format="YYYY-MM-DD" start-placeholder="开始日期" end-placeholder="结束日期" :clearable="false" :shortcuts="快捷日期" @change="调整粒度" />
        </el-form-item>
        <el-form-item label="统计间隔">
          <el-select v-model="筛选.granularity" style="width: 110px">
            <el-option label="按小时" value="hour" :disabled="区间天数 > 31" />
            <el-option label="按2小时" value="two_hour" :disabled="区间天数 > 31" />
            <el-option label="按天" value="day" />
          </el-select>
        </el-form-item>
        <el-form-item><el-button type="primary" native-type="submit" :loading="加载中">查询</el-button></el-form-item>
      </el-form>
      <div class="快捷栏">
        <div class="快捷日期">
          <span>快捷日期</span>
          <button v-for="item in 快捷日期" :key="item.text" type="button" :class="{ 当前: 日期?.join() === item.value().join() }" @click="选择快捷日期(item)">{{ item.text }}</button>
        </div>
        <div v-if="结果" class="查询摘要 次要文字">
          <span class="结果范围" :title="结果标题">{{ 结果标题 }}</span>
          <span class="更新时间" title="北京时间 · 数据约延迟1分钟">{{ 更新时间 }} 更新</span>
        </div>
        <span v-else class="次要文字">北京时间 · 数据约延迟1分钟</span>
      </div>
    </el-card>

    <el-alert v-if="错误提示" :title="错误提示" type="error" :closable="false" show-icon />
    <template v-if="结果">
      <div class="汇总">
        <article v-for="item in 指标" :key="item.key" class="指标卡" :class="item.key">
          <div class="指标标题"><span>{{ item.label }}总额</span><span class="指标图标" aria-hidden="true">{{ item.key === 'increase' ? '↗' : '↘' }}</span></div>
          <div class="指标数值"><strong>{{ 曲线.length ? 格式金额(结果[item.key]) : '—' }}</strong><span>{{ 单位 }}</span></div>
          <div class="指标底部"><span>{{ 间隔名称 }}峰值</span><b>{{ 曲线.length ? 格式金额(峰值[item.key]?.[item.key]) : '—' }}</b><span v-if="峰值[item.key] && 峰值[item.key][item.key] !== '0'" class="峰值时间">{{ 格式时间(峰值[item.key].time, true) }}</span></div>
        </article>
      </div>
      <el-card shadow="never" class="曲线卡片">
        <div class="曲线标题">
          <div class="趋势标题文字"><h3>增加与扣除趋势</h3><span class="次要文字">每{{ 间隔名称 }}汇总 · {{ 单位 }}</span></div>
          <div class="图例" aria-label="选择显示的曲线">
            <button v-for="item in 指标" :key="item.key" type="button" :class="[item.key, { 隐藏曲线: !显示曲线[item.key] }]" :aria-pressed="显示曲线[item.key]" @click="切换曲线(item.key)"><i></i>{{ item.label }}</button>
          </div>
        </div>
        <el-empty v-if="!曲线.length" description="所选区间没有统计覆盖，请选择启用后的日期" />
        <template v-else>
          <div ref="图表容器" class="图表容器">
            <svg :viewBox="`0 0 ${图宽} ${图高}`" role="img" tabindex="0" aria-label="余额增加和扣除趋势，移动或点击查看数值，左右方向键查看时段" @pointermove="定位悬浮" @click="定位悬浮" @pointerleave="离开图表" @pointercancel="悬浮序号 = -1" @blur="悬浮序号 = -1" @keydown.esc="悬浮序号 = -1" @keydown.left.prevent="移动读数(-1)" @keydown.right.prevent="移动读数(1)">
              <defs>
                <linearGradient id="balance-increase-fill" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stop-color="#45d6b0" stop-opacity="0.22" /><stop offset="100%" stop-color="#45d6b0" stop-opacity="0.01" /></linearGradient>
                <linearGradient id="balance-decrease-fill" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stop-color="#f5ad66" stop-opacity="0.18" /><stop offset="100%" stop-color="#f5ad66" stop-opacity="0.01" /></linearGradient>
              </defs>
              <g v-for="n in 5" :key="n">
                <line :x1="左边距" :y1="上边距 + (n - 1) * 绘图区高 / 4" :x2="图宽 - 右边距" :y2="上边距 + (n - 1) * 绘图区高 / 4" stroke="currentColor" opacity="0.12" />
                <text :x="左边距 - 10" :y="上边距 + (n - 1) * 绘图区高 / 4 + 4" text-anchor="end" class="坐标">{{ 刻度(最大值 * (5 - n) / 4) }}</text>
              </g>
              <template v-for="item in 指标" :key="item.key">
                <g v-if="显示曲线[item.key]">
                  <polygon :points="面积点(item.key)" :fill="`url(#balance-${item.key}-fill)`" />
                  <polyline :points="折线点(item.key)" fill="none" :stroke="item.color" stroke-width="2.5" stroke-linejoin="round" />
                </g>
              </template>
              <g v-for="index in 横轴索引" :key="index">
                <text :x="横坐标(index)" :y="图高 - 9" :text-anchor="曲线.length === 1 ? 'middle' : index === 0 ? 'start' : index === 曲线.length - 1 ? 'end' : 'middle'" class="坐标">{{ 格式时间(曲线[index].time, true) }}</text>
              </g>
              <g v-if="选中点">
                <line :x1="横坐标(选中序号)" :y1="上边距" :x2="横坐标(选中序号)" :y2="上边距 + 绘图区高" stroke="currentColor" opacity="0.35" stroke-dasharray="4 4" />
                <template v-for="item in 指标" :key="item.key"><circle v-if="显示曲线[item.key]" :cx="横坐标(选中序号)" :cy="纵坐标(选中点[item.key])" r="4.5" :fill="item.color" stroke="#1b2330" stroke-width="2" /></template>
              </g>
            </svg>
            <div v-if="悬浮序号 >= 0 && 选中点" ref="提示框" class="读数" :style="提示位置" role="tooltip" aria-live="polite">
              <strong class="读数时间">{{ 时段名称(选中点.time) }}</strong>
              <div v-for="item in 指标" :key="item.key" class="读数项" :class="item.key"><span>{{ item.label }}</span><strong>{{ 格式金额(选中点[item.key]) }}</strong></div>
              <span class="次要文字">{{ 单位 }}</span>
            </div>
          </div>
          <p v-if="没有变动" class="无变动提示">当前区间已纳入统计，暂未记录到增加或扣除。</p>
        </template>
      </el-card>
      <el-card v-if="曲线.length" shadow="never" class="明细卡片">
        <div class="明细标题"><div><h3>时段明细</h3><span class="次要文字">共 {{ 曲线.length }} 个时段 · {{ 单位 }}</span></div><el-switch v-model="仅看变动" active-text="只看有变动" @change="明细页 = 1" /></div>
        <el-table :data="当前明细" stripe @sort-change="排序明细" :default-sort="{ prop: 'time', order: 'descending' }" empty-text="当前区间暂无变动">
          <el-table-column prop="time" label="时段（北京时间）" min-width="200" sortable="custom"><template #default="{ row }">{{ 时段名称(row.time) }}</template></el-table-column>
          <el-table-column prop="increase" label="增加" min-width="140" align="right" sortable="custom"><template #default="{ row }"><span class="表格数值 increase">{{ 格式金额(row.increase) }}</span></template></el-table-column>
          <el-table-column prop="decrease" label="扣除" min-width="140" align="right" sortable="custom"><template #default="{ row }"><span class="表格数值 decrease">{{ 格式金额(row.decrease) }}</span></template></el-table-column>
        </el-table>
        <el-pagination v-model:current-page="明细页" v-model:page-size="每页数量" :page-sizes="[8, 16, 32]" :total="明细数据.length" layout="total, sizes, prev, pager, next" small @size-change="明细页 = 1" />
      </el-card>
      <p class="统计说明">统计启用于 {{ 格式时间(结果.started_at) }}，此前数据不补算；首尾时段可能不完整，异常时可能缺失。最多查询366天，按小时或2小时最多31天。</p>
    </template>
  </section>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch, watchPostEffect } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute } from 'vue-router'
import { use登录状态Store } from '../stores/登录状态.js'
import { 获取接口错误提示 } from '../api/请求客户端.js'

const stores = use登录状态Store()
const route = useRoute()
const { 是代理账号, 软件列表, 代理列表 } = storeToRefs(stores)
const 筛选 = reactive({ kind: 'agent', card: '', software: '', agent_id: '', granularity: 'day' })
// 默认日期取北京时间的日历日期，浏览器所在时区不会改变后台的分日边界。
const 今天 = new Date(Date.now() + 8 * 3600000).toISOString().slice(0, 10)
const 前几天 = (days) => new Date(Date.parse(`${今天}T00:00:00Z`) - days * 86400000).toISOString().slice(0, 10)
const 日期 = ref([前几天(6), 今天])
const 快捷日期 = [
  { text: '今天', value: () => [今天, 今天] },
  { text: '昨天', value: () => [前几天(1), 前几天(1)] },
  { text: '近7天', value: () => [前几天(6), 今天] },
  { text: '近30天', value: () => [前几天(29), 今天] }
]
const 区间天数 = computed(() => 日期.value?.length === 2 ? (Date.parse(日期.value[1]) - Date.parse(日期.value[0])) / 86400000 + 1 : 0)
const 调整粒度 = () => { if (区间天数.value > 31) 筛选.granularity = 'day' }
function 选择快捷日期(item) {
  日期.value = item.value()
  调整粒度()
  查询统计()
}
const 结果 = ref(null)
const 已查询条件 = ref(null)
const 更新时间 = ref('')
const 加载中 = ref(false)
const 错误提示 = ref('')
const 悬浮序号 = ref(-1)
const 图表容器 = ref(null)
const 图宽 = ref(800)
const 图高 = ref(260)
const 提示框 = ref(null)
const 提示锚点 = reactive({ x: 0, y: 0 })
const 提示位置 = ref({})
const 左边距 = 72
const 右边距 = 22
const 上边距 = 24
const 下边距 = 34
const 绘图区宽 = computed(() => Math.max(1, 图宽.value - 左边距 - 右边距))
const 绘图区高 = computed(() => Math.max(1, 图高.value - 上边距 - 下边距))
let 请求序号 = 0
const 单位 = computed(() => 已查询条件.value?.kind === 'point' ? '点数' : '账户余额单位')
const 指标 = [
  { key: 'increase', label: '增加', color: '#45d6b0' },
  { key: 'decrease', label: '扣除', color: '#f5ad66' }
]
const 显示曲线 = reactive({ increase: true, decrease: true })
const 间隔名称 = computed(() => ({ hour: '小时', two_hour: '2小时', day: '天' })[已查询条件.value?.granularity] || '天')
function 切换曲线(key) {
  const other = key === 'increase' ? 'decrease' : 'increase'
  if (显示曲线[key] && !显示曲线[other]) 显示曲线[other] = true
  显示曲线[key] = !显示曲线[key]
}
const 结果标题 = computed(() => {
  const q = 已查询条件.value
  const target = q.kind === 'point' ? `点卡${q.card ? ` ${q.card}` : ''}` : '小伙伴账户'
  const software = q.software ? ` · 软件ID ${q.software}` : ''
  const agent = q.agent_id ? ` · 小伙伴ID ${q.agent_id}` : ''
  return `${target}${software}${agent} · ${q.start_date} 至 ${q.end_date}`
})
const 曲线 = computed(() => {
  const rows = 结果.value?.trend || []
  if (已查询条件.value?.granularity !== 'two_hour') return rows
  // 沿用小时接口，按北京时间 00、02、04 时分桶；金额用 BigInt 相加，保留完整精度。
  const buckets = new Map()
  for (const row of rows) {
    const time = Math.floor((row.time + 28800) / 7200) * 7200 - 28800
    const bucket = buckets.get(time) || { time, increase: 0n, decrease: 0n }
    bucket.increase += BigInt(row.increase)
    bucket.decrease += BigInt(row.decrease)
    buckets.set(time, bucket)
  }
  return Array.from(buckets.values(), (row) => ({ ...row, increase: String(row.increase), decrease: String(row.decrease) }))
})
const 没有变动 = computed(() => 曲线.value.every((row) => BigInt(row.increase) === 0n && BigInt(row.decrease) === 0n))
const 峰值 = computed(() => Object.fromEntries(指标.map(({ key }) => [key, 曲线.value.reduce((peak, row) =>
  !peak || BigInt(row[key]) > BigInt(peak[key]) ? row : peak, null)])))
const 最大值 = computed(() => {
  const max = Math.max(1, ...曲线.value.flatMap((row) => 指标.filter(({ key }) => 显示曲线[key]).map(({ key }) => Number(row[key]))))
  // 给峰值留出顶部空间，并将纵轴向上取整，避免最高点紧贴边框。
  const step = 10 ** Math.floor(Math.log10(max)) / 2
  return Math.ceil(max * 1.1 / step) * step
})
const 横坐标 = (index) => {
  if (曲线.value.length <= 1) return 左边距 + 绘图区宽.value / 2
  return 左边距 + index * 绘图区宽.value / (曲线.value.length - 1)
}
const 纵坐标 = (value) => 上边距 + 绘图区高.value * (1 - Number(value) / 最大值.value)
const 折线点 = (key) => 曲线.value.map((row, index) => `${横坐标(index)},${纵坐标(row[key])}`).join(' ')
const 面积点 = (key) => `${横坐标(0)},${纵坐标(0)} ${折线点(key)} ${横坐标(曲线.value.length - 1)},${纵坐标(0)}`
const 横轴索引 = computed(() => {
  const count = Math.min(曲线.value.length, Math.max(2, Math.floor(绘图区宽.value / 110) + 1))
  return Array.from({ length: count }, (_, i) => Math.round(i * (曲线.value.length - 1) / Math.max(1, count - 1)))
})
const 选中序号 = computed(() => 悬浮序号.value < 0 ? 曲线.value.length - 1 : 悬浮序号.value)
const 选中点 = computed(() => 曲线.value[选中序号.value])
const 移动读数 = (step) => {
  悬浮序号.value = Math.max(0, Math.min(曲线.value.length - 1, 选中序号.value + step))
  if (!选中点.value) return
  提示锚点.x = 横坐标(选中序号.value)
  提示锚点.y = Math.min(...指标.filter(({ key }) => 显示曲线[key]).map(({ key }) => 纵坐标(选中点.value[key])))
}
const 定位悬浮 = (event) => {
  const bounds = event.currentTarget.getBoundingClientRect()
  const x = (event.clientX - bounds.left) / bounds.width * 图宽.value
  const index = Math.round((x - 左边距) / 绘图区宽.value * (曲线.value.length - 1))
  悬浮序号.value = Math.max(0, Math.min(曲线.value.length - 1, index))
  提示锚点.x = x
  提示锚点.y = (event.clientY - bounds.top) / bounds.height * 图高.value
}
const 离开图表 = (event) => { if (event.pointerType !== 'touch') 悬浮序号.value = -1 }
// 按实际内容尺寸避让鼠标并限制在图内；触屏点击后保留读数，方向键则定位到曲线点。
watchPostEffect(() => {
  const point = 选中点.value
  const { x, y } = 提示锚点
  const width = 图宽.value, height = 图高.value
  if (!提示框.value || !point) return
  const boxWidth = 提示框.value.offsetWidth, boxHeight = 提示框.value.offsetHeight
  const left = x + 14 + boxWidth <= width - 8 ? x + 14 : x - boxWidth - 14
  const top = y + 14 + boxHeight <= height - 8 ? y + 14 : y - boxHeight - 14
  提示位置.value = {
    left: `${Math.max(8, Math.min(left, width - boxWidth - 8))}px`,
    top: `${Math.max(8, Math.min(top, height - boxHeight - 8))}px`
  }
})
// SVG 坐标跟随容器重算，窗口缩放时文字大小保持不变，横轴标签按可用宽度减少。
const 更新图表尺寸 = (element) => {
  if (!element) return
  const bounds = element.getBoundingClientRect()
  if (bounds.width > 0) 图宽.value = Math.round(bounds.width)
  if (bounds.height > 0) 图高.value = Math.round(bounds.height)
}
let 尺寸观察器
watch(图表容器, (element, previous) => {
  if (previous) 尺寸观察器?.unobserve(previous)
  if (!element) return
  更新图表尺寸(element)
  尺寸观察器 ||= new ResizeObserver((entries) => 更新图表尺寸(entries[0]?.target))
  尺寸观察器.observe(element)
}, { flush: 'post' })
onBeforeUnmount(() => 尺寸观察器?.disconnect())
const 格式金额 = (value) => BigInt(value || '0').toLocaleString('zh-CN')
const 刻度 = (value) => new Intl.NumberFormat('zh-CN', { notation: 'compact', maximumFractionDigits: 1 }).format(value)
const 格式时间 = (seconds, short = false) => {
  const text = new Date(seconds * 1000 + 8 * 3600000).toISOString()
  if (short) return 已查询条件.value?.granularity !== 'day' ? `${text.slice(5, 10)} ${text.slice(11, 13)}时` : text.slice(5, 10)
  return text.slice(0, 19).replace('T', ' ')
}
const 时段名称 = (seconds) => {
  const start = 格式时间(seconds)
  if (已查询条件.value?.granularity === 'day') return start.slice(0, 10)
  const hours = 已查询条件.value?.granularity === 'two_hour' ? 2 : 1
  const endHour = Number(start.slice(11, 13)) + hours
  return `${start.slice(0, 16)}–${String(endHour).padStart(2, '0')}:00`
}
const 明细页 = ref(1)
const 每页数量 = ref(8)
const 仅看变动 = ref(true)
const 明细排序 = ref({ prop: 'time', order: 'descending' })
function 排序明细(sort) {
  明细排序.value = sort.order ? sort : { prop: 'time', order: 'descending' }
  明细页.value = 1
}
const 明细数据 = computed(() => {
  const { prop, order } = 明细排序.value
  const rows = 曲线.value.filter((row) => !仅看变动.value || BigInt(row.increase) !== 0n || BigInt(row.decrease) !== 0n)
  return rows.sort((a, b) => {
    const left = BigInt(a[prop]), right = BigInt(b[prop])
    return (left === right ? 0 : left > right ? 1 : -1) * (order === 'ascending' ? 1 : -1)
  })
})
const 当前明细 = computed(() => 明细数据.value.slice((明细页.value - 1) * 每页数量.value, 明细页.value * 每页数量.value))

async function 查询统计() {
  const sequence = ++请求序号
  错误提示.value = ''
  if (!日期.value || 区间天数.value < 1 || 区间天数.value > 366) {
    错误提示.value = '请选择有效日期范围，每次最多查询366天'
    加载中.value = false
    return
  }
  const agentID = Number(筛选.agent_id || 0)
  if (!Number.isSafeInteger(agentID) || agentID < 0) {
    错误提示.value = '小伙伴ID应为正整数'
    加载中.value = false
    return
  }
  const params = {
    ...筛选, card: 筛选.card.trim(), software: Number(筛选.software || 0), agent_id: agentID,
    start_date: 日期.value[0], end_date: 日期.value[1]
  }
  加载中.value = true
  try {
    const response = await stores.post('/balance_stats', { ...params, granularity: params.granularity === 'two_hour' ? 'hour' : params.granularity })
    if (sequence !== 请求序号) return
    if (!response.data?.state) throw new Error(response.data?.msg || '查询统计失败')
    结果.value = response.data.data
    已查询条件.value = params
    悬浮序号.value = -1
    明细页.value = 1
    更新时间.value = 格式时间(Date.now() / 1000).slice(11)
  } catch (error) {
    if (sequence === 请求序号) {
      结果.value = null
      错误提示.value = 获取接口错误提示(error)
    }
  } finally {
    if (sequence === 请求序号) 加载中.value = false
  }
}
function 切换对象() {
  筛选.card = ''
  筛选.software = ''
  查询统计()
}
watch(() => route.query, (query) => {
  // 点卡列表的统计入口传入完整卡密和归属；菜单进入时恢复账户统计默认视图。
  筛选.kind = query.kind === 'point' ? 'point' : 'agent'
  筛选.card = 筛选.kind === 'point' && typeof query.card === 'string' ? query.card : ''
  筛选.software = 筛选.kind === 'point' && typeof query.software === 'string' ? query.software : ''
  筛选.agent_id = typeof query.agent_id === 'string' ? query.agent_id : ''
  查询统计()
}, { immediate: true })
onMounted(() => {
  stores.查询软件列表().catch(() => {})
  if (!是代理账号.value) stores.查询代理列表().catch(() => {})
})
</script>

<style scoped>
.统计页面 { padding: 12px; color: #edf2fa; font-variant-numeric: tabular-nums; }
/* 统计区提高底色不透明度，背景图片不会干扰坐标、读数和说明文字。 */
.统计页面 :deep(.el-card__body) { background: rgba(24, 31, 43, 0.96); }
.统计页面 :deep(.el-card) { border-color: #394557; border-radius: 12px; }
.统计页面 .筛选卡片 { --el-card-padding: 10px; margin-bottom: 8px; }
.统计页面 :deep(.el-form--inline) { display: flex; flex-wrap: wrap; gap: 8px 14px; }
.统计页面 :deep(.el-form-item) { margin: 0; max-width: 100%; }
.统计页面 :deep(.el-form-item__content) { min-width: 0; }
.统计页面 :deep(.el-input), .统计页面 :deep(.el-select), .统计页面 :deep(.el-date-editor) { max-width: 100%; }
.统计页面 :deep(.el-form-item__label) { color: #d4dce8; }
.次要文字 { color: #b4c0d2; font-size: 12px; line-height: 1.7; }
.快捷栏, .快捷日期, .查询摘要, .指标标题, .指标底部, .曲线标题, .明细标题 { display: flex; align-items: center; gap: 10px; }
.快捷栏 { justify-content: space-between; flex-wrap: wrap; border-top: 1px solid #354052; margin-top: 8px; padding-top: 6px; }
.快捷日期 { flex-wrap: wrap; gap: 6px; font-size: 12px; color: #b4c0d2; }
.快捷日期 > span { margin-right: 6px; }
.快捷日期 button, .图例 button { font: inherit; cursor: pointer; border: 1px solid transparent; border-radius: 6px; background: transparent; color: #c3cede; padding: 5px 12px; }
.快捷日期 button:hover, .快捷日期 .当前 { color: #c7e1ff; border-color: #43658b; background: #253d59; }
.查询摘要 { flex: 1; justify-content: flex-end; min-width: 0; }
.结果范围 { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.更新时间 { flex-shrink: 0; }
.汇总 { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; margin-bottom: 8px; }
.指标卡 { min-width: 0; padding: 10px 16px 8px; border: 1px solid #3a4e5e; border-radius: 12px; background: linear-gradient(110deg, #1c3436, #1b2431 75%); }
.指标卡.decrease { border-color: #51483e; background: linear-gradient(110deg, #352c26, #1b2431 75%); }
.指标标题 { justify-content: space-between; color: #e0e8f1; font-size: 14px; font-weight: 600; }
.指标图标 { display: grid; place-items: center; width: 24px; height: 24px; border-radius: 6px; color: #45d6b0; background: #284941; font-size: 19px; }
.decrease .指标图标 { color: #f5ad66; background: #4b3b2e; }
.指标数值 { display: flex; align-items: baseline; flex-wrap: wrap; gap: 8px; margin: 4px 0 8px; }
.指标数值 strong { font-size: clamp(24px, 2.6vw, 34px); font-weight: 650; letter-spacing: -0.8px; line-height: 1.3; overflow-wrap: anywhere; min-width: 0; }
.指标数值 > span { font-size: 12px; color: #bcc8d8; }
.指标底部 { flex-wrap: wrap; gap: 4px 8px; padding-top: 6px; border-top: 1px solid #ffffff18; font-size: 12px; color: #b4c0d2; }
.指标底部 b { color: #edf2fa; overflow-wrap: anywhere; }
.峰值时间 { margin-left: auto; }
.increase { color: #45d6b0; }
.decrease { color: #f5ad66; }
.曲线卡片, .明细卡片 { --el-card-padding: 10px 12px; margin-bottom: 8px; }
.曲线标题, .明细标题 { justify-content: space-between; flex-wrap: wrap; }
.趋势标题文字 { display: flex; align-items: baseline; flex-wrap: wrap; gap: 2px 10px; }
.趋势标题文字 h3 { margin: 0; }
h3 { margin: 0 0 3px; color: #edf2fa; font-size: 16px; font-weight: 600; }
.图例 { display: flex; gap: 8px; }
.图例 button { display: flex; align-items: center; gap: 8px; border-color: #3b485a; color: #d9e3ef; font-size: 13px; }
.图例 i { width: 18px; height: 3px; border-radius: 2px; background: #45d6b0; }
.图例 .decrease i { background: #f5ad66; }
.图例 .隐藏曲线 { opacity: 0.5; text-decoration: line-through; }
.图表容器 { position: relative; min-width: 0; width: 100%; height: 260px; margin-top: 6px; overflow: hidden; }
.图表容器 svg { display: block; width: 100%; height: 100%; color: #bac6d7; outline-offset: -2px; }
.坐标 { fill: currentColor; font-size: 12px; }
.读数 { position: absolute; z-index: 1; pointer-events: none; width: 240px; max-width: calc(100% - 16px); padding: 10px 12px; border: 1px solid #52647d; border-radius: 8px; background: #202c3d; box-shadow: 0 4px 16px #0006; }
.读数时间 { display: block; font-size: 13px; color: #edf2fa; line-height: 1.8; }
.读数项 { display: flex; align-items: baseline; justify-content: space-between; gap: 10px; margin: 6px 0; }
.读数项 span { font-size: 13px; }
.读数项 strong { min-width: 0; font-size: 16px; font-weight: 600; overflow-wrap: anywhere; text-align: right; line-height: 1.4; }
.无变动提示 { text-align: center; color: #c2cfe0; font-size: 13px; margin: 6px 0 0; }
.明细标题 { margin-bottom: 8px; }
.表格数值 { font-weight: 600; font-size: 14px; }
.明细卡片 :deep(.el-pagination) { justify-content: flex-end; margin-top: 8px; padding: 0; border: 0; background: transparent; }
.统计说明 { margin: 6px 2px; padding: 6px 10px; border-radius: 8px; background: rgba(24, 31, 43, 0.96); color: #b4c0d2; font-size: 12px; line-height: 1.7; }
@media (max-width: 600px) {
  .统计页面 { padding: 8px; }
  .汇总 { gap: 8px; }
  .指标卡 { padding: 8px 10px; }
  .指标图标 { display: none; }
  .指标数值 strong { font-size: 25px; }
  .指标数值 { gap: 2px; flex-direction: column; }
  .峰值时间 { width: 100%; margin: 0; }
  .曲线卡片, .明细卡片 { --el-card-padding: 10px; }
  .日期筛选 { width: 100%; }
  .统计页面 :deep(.日期筛选 .el-form-item__content) { flex: 1; }
  .统计页面 :deep(.el-date-editor--daterange) { width: 100%; min-width: 0; }
  .图表容器 { height: 250px; }
  .查询摘要 { flex-basis: 100%; justify-content: space-between; }
}
</style>
