<template>
  <section class="统计页面" v-loading="加载中">
    <el-card shadow="never" class="筛选卡片">
      <el-form :inline="true" @submit.prevent="查询统计">
        <el-form-item label="统计对象">
          <el-select v-model="筛选.kind" style="width: 140px" @change="切换对象">
            <el-option label="点卡" value="point" />
            <el-option label="代理账户" value="agent" />
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
        <el-form-item v-if="!是代理账号" label="代理">
          <el-select v-model="筛选.agent_id" clearable filterable allow-create default-first-option placeholder="全部 / 输入历史账户ID" style="width: 220px">
            <el-option v-for="item in 代理列表" :key="item.id" :value="item.id" :label="`${item.name}（${item.id}）`" />
          </el-select>
        </el-form-item>
        <el-form-item label="日期">
          <el-date-picker v-model="日期" type="daterange" value-format="YYYY-MM-DD" start-placeholder="开始日期" end-placeholder="结束日期" :clearable="false" :shortcuts="快捷日期" @change="调整粒度" />
        </el-form-item>
        <el-form-item label="曲线">
          <el-select v-model="筛选.granularity" style="width: 110px">
            <el-option label="按小时" value="hour" :disabled="区间天数 > 31" />
            <el-option label="按天" value="day" />
          </el-select>
        </el-form-item>
        <el-form-item><el-button type="primary" native-type="submit">查询</el-button></el-form-item>
      </el-form>
    </el-card>

    <p class="统计说明">北京时间 · 最多366天，小时曲线最多31天 · 数据约延迟1分钟，异常时可能缺失</p>
    <el-alert v-if="错误提示" :title="错误提示" type="error" :closable="false" show-icon />
    <template v-if="结果">
      <p class="统计说明">{{ 结果标题 }} · 统计启用于 {{ 格式时间(结果.started_at) }}，此前数据不补算；启用当天仅包含启用后的变动。</p>
      <div class="汇总">
        <el-card shadow="never"><span>增加总额 · {{ 单位 }}</span><strong class="增加">{{ 曲线.length ? 格式金额(结果.increase) : '—' }}</strong></el-card>
        <el-card shadow="never"><span>扣除总额 · {{ 单位 }}</span><strong class="扣除">{{ 曲线.length ? 格式金额(结果.decrease) : '—' }}</strong></el-card>
      </div>
      <el-card shadow="never" class="曲线卡片">
        <div class="曲线标题"><h3>增加与扣除趋势</h3><div><span class="增加">● 增加</span><span class="扣除">● 扣除</span></div></div>
        <el-empty v-if="!曲线.length" description="所选区间没有统计覆盖，请选择启用后的日期" />
        <template v-else>
          <div ref="图表容器" class="图表容器">
            <svg :viewBox="`0 0 ${图宽} ${图高}`" role="img" aria-label="余额增加和扣除趋势，具体数值可在下方查看" @mousemove="定位悬浮" @mouseleave="悬浮序号 = -1">
              <g v-for="n in 5" :key="n">
                <line :x1="左边距" :y1="上边距 + (n - 1) * 绘图区高 / 4" :x2="图宽 - 右边距" :y2="上边距 + (n - 1) * 绘图区高 / 4" stroke="currentColor" opacity="0.12" />
                <text :x="左边距 - 10" :y="上边距 + (n - 1) * 绘图区高 / 4 + 4" text-anchor="end" class="坐标">{{ 刻度(最大值 * (5 - n) / 4) }}</text>
              </g>
              <polyline :points="增加折线" fill="none" stroke="#67c23a" stroke-width="2.5" stroke-linejoin="round" />
              <polyline :points="扣除折线" fill="none" stroke="#f56c6c" stroke-width="2.5" stroke-linejoin="round" />
              <g v-for="index in 横轴索引" :key="index">
                <text :x="横坐标(index)" :y="图高 - 9" text-anchor="middle" class="坐标">{{ 格式时间(曲线[index].time, true) }}</text>
              </g>
              <g v-if="选中点">
                <line :x1="横坐标(选中序号)" :y1="上边距" :x2="横坐标(选中序号)" :y2="上边距 + 绘图区高" stroke="currentColor" opacity="0.35" stroke-dasharray="4 4" />
                <circle :cx="横坐标(选中序号)" :cy="纵坐标(选中点.increase)" r="4" fill="#67c23a" />
                <circle :cx="横坐标(选中序号)" :cy="纵坐标(选中点.decrease)" r="4" fill="#f56c6c" />
              </g>
            </svg>
          </div>
          <div v-if="选中点" class="读数">
            <span>{{ 格式时间(选中点.time) }}</span>
            <span class="增加">增加 {{ 格式金额(选中点.increase) }}</span>
            <span class="扣除">扣除 {{ 格式金额(选中点.decrease) }}</span>
          </div>
          <el-collapse class="明细">
            <el-collapse-item title="查看各时段数值" name="details">
              <el-table :data="曲线" max-height="380" stripe>
                <el-table-column label="时间"><template #default="{ row }">{{ 格式时间(row.time) }}</template></el-table-column>
                <el-table-column label="增加" align="right"><template #default="{ row }">{{ 格式金额(row.increase) }}</template></el-table-column>
                <el-table-column label="扣除" align="right"><template #default="{ row }">{{ 格式金额(row.decrease) }}</template></el-table-column>
              </el-table>
            </el-collapse-item>
          </el-collapse>
        </template>
      </el-card>
    </template>
  </section>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
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
const 结果 = ref(null)
const 已查询条件 = ref(null)
const 加载中 = ref(false)
const 错误提示 = ref('')
const 悬浮序号 = ref(-1)
const 图表容器 = ref(null)
const 图宽 = ref(800)
const 图高 = ref(320)
const 左边距 = 72
const 右边距 = 22
const 上边距 = 18
const 下边距 = 30
const 绘图区宽 = computed(() => Math.max(1, 图宽.value - 左边距 - 右边距))
const 绘图区高 = computed(() => Math.max(1, 图高.value - 上边距 - 下边距))
let 请求序号 = 0
const 单位 = computed(() => 已查询条件.value?.kind === 'point' ? '点数' : '账户余额单位')
const 结果标题 = computed(() => {
  const q = 已查询条件.value
  const target = q.kind === 'point' ? `点卡${q.card ? ` ${q.card}` : ''}` : '代理账户'
  const software = q.software ? ` · 软件ID ${q.software}` : ''
  const agent = q.agent_id ? ` · 代理ID ${q.agent_id}` : ''
  return `${target}${software}${agent} · ${q.start_date} 至 ${q.end_date}`
})
const 曲线 = computed(() => 结果.value?.trend || [])
const 最大值 = computed(() => Math.max(1, ...曲线.value.flatMap((row) => [Number(row.increase), Number(row.decrease)])))
const 横坐标 = (index) => {
  if (曲线.value.length <= 1) return 左边距 + 绘图区宽.value / 2
  return 左边距 + index * 绘图区宽.value / (曲线.value.length - 1)
}
const 纵坐标 = (value) => 上边距 + 绘图区高.value * (1 - Number(value) / 最大值.value)
const 增加折线 = computed(() => 曲线.value.map((row, index) => `${横坐标(index)},${纵坐标(row.increase)}`).join(' '))
const 扣除折线 = computed(() => 曲线.value.map((row, index) => `${横坐标(index)},${纵坐标(row.decrease)}`).join(' '))
const 横轴索引 = computed(() => {
  const count = Math.min(曲线.value.length, Math.max(2, Math.floor(绘图区宽.value / 110) + 1))
  return Array.from({ length: count }, (_, i) => Math.round(i * (曲线.value.length - 1) / Math.max(1, count - 1)))
})
const 选中序号 = computed(() => 悬浮序号.value < 0 ? 曲线.value.length - 1 : 悬浮序号.value)
const 选中点 = computed(() => 曲线.value[选中序号.value])
const 定位悬浮 = (event) => {
  const bounds = event.currentTarget.getBoundingClientRect()
  const x = (event.clientX - bounds.left) / bounds.width * 图宽.value
  const index = Math.round((x - 左边距) / 绘图区宽.value * (曲线.value.length - 1))
  悬浮序号.value = Math.max(0, Math.min(曲线.value.length - 1, index))
}
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
  if (short) return 已查询条件.value?.granularity === 'hour' ? `${text.slice(5, 10)} ${text.slice(11, 13)}时` : text.slice(5, 10)
  return text.slice(0, 19).replace('T', ' ')
}

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
    错误提示.value = '代理ID应为正整数'
    加载中.value = false
    return
  }
  const params = {
    ...筛选, card: 筛选.card.trim(), software: Number(筛选.software || 0), agent_id: agentID,
    start_date: 日期.value[0], end_date: 日期.value[1]
  }
  加载中.value = true
  try {
    const response = await stores.post('/balance_stats', params)
    if (sequence !== 请求序号) return
    if (!response.data?.state) throw new Error(response.data?.msg || '查询统计失败')
    结果.value = response.data.data
    已查询条件.value = params
    悬浮序号.value = -1
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
.统计页面 { padding: 12px; color: #e6eaf2; }
.筛选卡片 { --el-card-padding: 10px; margin-bottom: 8px; }
.统计页面 :deep(.el-form--inline) { display: flex; flex-wrap: wrap; gap: 8px 14px; }
.统计页面 :deep(.el-form-item) { margin: 0; max-width: 100%; }
.统计页面 :deep(.el-form-item__content) { min-width: 0; }
.统计页面 :deep(.el-input), .统计页面 :deep(.el-select), .统计页面 :deep(.el-date-editor) { max-width: 100%; }
.曲线标题 { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
h3 { margin: 0; }
.统计说明 { margin: 7px 2px; color: #aeb6c3; font-size: 12px; line-height: 1.6; }
.汇总 { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; margin: 10px 0; }
.汇总 span { font-size: 14px; color: #aeb6c3; }
.汇总 strong { display: block; margin-top: 8px; font-size: clamp(20px, 3vw, 32px); overflow-wrap: anywhere; }
.增加 { color: #67c23a; }
.扣除 { color: #f56c6c; }
.曲线标题 span + span { margin-left: 20px; }
.图表容器 { width: 100%; height: clamp(220px, calc(100vh - 380px), 540px); overflow: hidden; margin-top: 12px; }
.图表容器 svg { display: block; width: 100%; height: 100%; color: #aeb6c3; }
.坐标 { fill: currentColor; font-size: 12px; }
.读数 { display: flex; justify-content: center; flex-wrap: wrap; gap: 20px; font-size: 13px; margin: 10px 0 20px; }
.明细 { margin-top: 16px; }
@media (max-width: 600px) {
  .汇总 { gap: 10px; }
  .统计页面 { padding: 10px; }
  .统计页面 :deep(.el-date-editor--daterange) { width: 270px; }
  .图表容器 { height: clamp(190px, calc(100vh - 380px), 420px); }
}
</style>
