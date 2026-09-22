<template>
  <section class="页面" v-loading="加载中">
    <div class="页面标题行">
      <div>
        <h2>时长卡管理</h2>
        <p class="说明">{{ 是代理账号 ? '生成并管理自己创建的时长卡，服务端会在发卡时重新核算费用。' : '时长卡独立于点卡；卡密首次登录激活，按卡面固定时长使用。' }}</p>
      </div>
      <el-button type="primary" @click="打开生成">生成时长卡</el-button>
    </div>

    <el-card shadow="never" class="筛选卡片">
      <el-form :inline="true" @submit.prevent>
        <el-form-item label="软件">
          <el-select v-model="筛选.software" clearable placeholder="全部软件" style="width: 170px" @change="查询列表(true)">
            <el-option v-for="item in 软件列表" :key="item.ID" :label="item.Software" :value="item.ID" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="筛选.card_state" clearable placeholder="全部状态" style="width: 130px" @change="查询列表(true)">
            <el-option label="未激活" :value="1" />
            <el-option label="已激活" :value="2" />
            <el-option label="已到期" :value="3" />
            <el-option label="冻结" :value="4" />
            <el-option label="已暂停" :value="5" />
            <el-option label="已用于充值" :value="6" />
          </el-select>
        </el-form-item>
        <el-form-item label="卡密">
          <el-input v-model="筛选.card" clearable placeholder="支持模糊搜索" style="width: 210px" @keyup.enter="查询列表(true)" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="筛选.notes" clearable placeholder="支持模糊搜索" style="width: 180px" @keyup.enter="查询列表(true)" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="查询列表(true)">查询</el-button>
          <el-button @click="重置筛选">重置</el-button>
        </el-form-item>
      </el-form>
      <div class="批量操作">
        <span>已选 {{ 已选.length }} 张</span>
        <el-button size="small" type="warning" :disabled="!已选.length" @click="批量修改状态(4)">冻结</el-button>
        <el-button size="small" type="success" :disabled="!已选.length" @click="批量修改状态(2)">解冻</el-button>
        <el-button size="small" type="danger" :disabled="!已选.length" @click="批量删除">删除</el-button>
        <el-button size="small" type="success" :disabled="!已选.length" @click="打开续费">批量续费</el-button>
        <el-button size="small" :disabled="!列表.length" @click="导出当前页">导出当前页</el-button>
      </div>
    </el-card>

    <el-table :data="列表" border stripe row-key="card" @selection-change="选择变化" @sort-change="排序变化">
      <el-table-column type="selection" width="44" />
      <el-table-column prop="card" label="卡密" min-width="190" sortable="custom" show-overflow-tooltip />
      <el-table-column prop="software" label="软件" width="140" sortable="custom">
        <template #default="scope">{{ 软件名称(scope.row.software) }}</template>
      </el-table-column>
      <el-table-column v-if="!是代理账号" label="归属代理" width="140">
        <template #default="scope">{{ 代理名称(scope.row.agent_id) }}</template>
      </el-table-column>
      <el-table-column prop="duration_minutes" label="卡面时长" width="115" sortable="custom">
        <template #default="scope">{{ 时长文本(scope.row.duration_minutes) }}</template>
      </el-table-column>
      <el-table-column label="暂停剩余" width="105">
        <template #default="scope">
          <span v-if="Number(scope.row.card_state) === 5">{{ 时长文本(scope.row.paused_remaining_minutes) }}</span>
          <span v-else class="弱文本">-</span>
        </template>
      </el-table-column>
      <el-table-column prop="card_state" label="状态" width="95">
        <template #default="scope">
          <el-tag :type="状态类型(scope.row)">{{ 状态文本(scope.row) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="end_time" label="到期时间" width="180" sortable="custom">
        <template #default="scope">{{ 格式化时间(scope.row.end_time) || '-' }}</template>
      </el-table-column>
      <el-table-column label="在线" width="75">
        <template #default="scope">
          <el-tag v-if="scope.row.online" type="success">在线</el-tag>
          <span v-else class="弱文本">-</span>
        </template>
      </el-table-column>
      <el-table-column prop="create_time" label="生成时间" width="175" sortable="custom">
        <template #default="scope">{{ 格式化时间(scope.row.create_time) }}</template>
      </el-table-column>
      <el-table-column prop="notes" label="备注" min-width="160" show-overflow-tooltip />
      <el-table-column label="操作" width="260" fixed="right">
        <template #default="scope">
          <el-button link type="primary" @click="查看详情(scope.row)">详情</el-button>
          <el-button v-if="可续费状态(scope.row)" link type="success" @click="打开单张续费(scope.row)">续费</el-button>
          <el-button v-if="可编辑状态(scope.row)" link type="warning" @click="打开编辑(scope.row)">编辑</el-button>
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
      @size-change="查询列表(true)"
      @current-change="查询列表(false)"
    />
    <el-dialog v-model="生成框.显示" :title="是代理账号 ? '小伙伴生成时长卡' : '生成时长卡'" width="560px" destroy-on-close>
      <el-form label-width="120px" v-loading="生成框.加载中">
        <el-form-item label="所属软件" required>
          <el-select v-model="生成框.software" placeholder="请选择软件" style="width: 280px">
            <el-option v-for="item in (是代理账号 ? 可发卡软件列表 : 软件列表)" :key="item.ID" :label="item.Software" :value="item.ID" />
          </el-select>
        </el-form-item>
        <el-form-item label="卡面时长" required>
          <el-input-number v-model="生成框.duration_minutes" :min="最小时长分钟" :max="最大时长分钟" :precision="0" controls-position="right" />
          <span class="单位">分钟（{{ 时长文本(生成框.duration_minutes) }}）</span>
          <span v-if="是代理账号" class="价格提醒">
            <el-tag size="small" :type="代理时长价格提醒.type">{{ 代理时长价格提醒.text }}</el-tag>
            <el-button
              v-if="代理时长价格提醒.recommendedDuration"
              link
              type="warning"
              size="small"
              @click="套用推荐时长"
            >改为{{ 时长文本(代理时长价格提醒.recommendedDuration) }}</el-button>
          </span>
        </el-form-item>
        <el-form-item label="快捷时长">
          <el-select v-model="生成框.duration_minutes" style="width: 220px">
            <el-option v-for="item in 时长预设" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="生成数量" required>
          <el-input-number v-model="生成框.num" :min="1" :max="500" :precision="0" controls-position="right" />
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
        <el-form-item label="最晚激活">
          <el-input-number v-model="生成框.latest_activation_days" :min="-1" :max="36500" :precision="0" controls-position="right" />
          <span class="单位">天（-1不限，0立即激活）</span>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="生成框.notes" maxlength="500" show-word-limit /></el-form-item>
        <el-form-item label="配置"><el-input v-model="生成框.config_content" type="textarea" :rows="3" maxlength="200" show-word-limit /></el-form-item>
        <el-form-item v-if="生成结果" label="生成结果">
          <el-input v-model="生成结果" type="textarea" :rows="5" readonly />
          <el-button class="复制按钮" @click="复制文本(生成结果)">复制</el-button>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="生成框.显示 = false">取消</el-button>
        <el-button type="primary" :disabled="生成框.提交中" :loading="生成框.提交中" @click="生成">
          {{ 是代理账号 ? '确认生成' : '生成' }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="编辑框.显示" title="编辑时长卡" width="520px" destroy-on-close>
      <el-form label-width="100px" v-loading="编辑框.加载中">
        <el-form-item label="卡密"><span class="卡密文本">{{ 编辑框.card }}</span></el-form-item>
        <el-form-item label="卡面时长"><span>{{ 时长文本(编辑框.duration_minutes) }}</span></el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="编辑框.card_state">
            <el-radio :label="2">正常</el-radio>
            <el-radio :label="4">冻结</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="编辑框.notes" maxlength="500" /></el-form-item>
        <el-form-item label="配置"><el-input v-model="编辑框.config_content" type="textarea" :rows="3" maxlength="200" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="编辑框.显示 = false">取消</el-button>
        <el-button type="primary" :loading="编辑框.加载中" @click="保存编辑">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="续费框.显示" title="续费时长卡" width="430px" destroy-on-close>
      <el-form label-width="100px" v-loading="续费框.加载中">
        <el-form-item label="已选择">
          <span>{{ 续费框.cards.length }} 张时长卡</span>
        </el-form-item>
        <el-form-item label="增加时长">
          <el-input-number v-model="续费框.duration_minutes" :min="最小时长分钟" :max="最大时长分钟" :precision="0" controls-position="right" />
          <span class="单位">分钟</span>
        </el-form-item>
        <el-form-item label="快捷时长">
          <el-select v-model="续费框.duration_minutes" style="width: 220px">
            <el-option v-for="item in 时长预设" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="续费框.显示 = false">取消</el-button>
        <el-button type="primary" :loading="续费框.加载中" @click="续费">确认续费</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="详情框.显示" title="时长卡详情" width="620px" destroy-on-close>
      <el-descriptions v-loading="详情框.加载中" :column="2" border>
        <el-descriptions-item label="卡密">{{ 详情框.data.card }}</el-descriptions-item>
        <el-descriptions-item label="软件">{{ 软件名称(详情框.data.software) }}</el-descriptions-item>
        <el-descriptions-item label="卡面时长">{{ 时长文本(详情框.data.duration_minutes) }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ 详情框.data.status || '-' }}</el-descriptions-item>
        <el-descriptions-item label="生成时间">{{ 格式化时间(详情框.data.create_time) }}</el-descriptions-item>
        <el-descriptions-item label="使用时间">{{ 格式化时间(详情框.data.use_time) || '-' }}</el-descriptions-item>
        <el-descriptions-item label="到期时间">{{ 格式化时间(详情框.data.end_time) || '-' }}</el-descriptions-item>
        <el-descriptions-item label="暂停剩余">{{ 时长文本(详情框.data.paused_remaining_minutes) }}</el-descriptions-item>
        <el-descriptions-item label="在线">{{ 详情框.data.online ? '在线' : '不在线' }}</el-descriptions-item>
        <el-descriptions-item label="needle" :span="2">{{ 详情框.data.needle || '-' }}</el-descriptions-item>
        <el-descriptions-item label="备注" :span="2">{{ 详情框.data.notes || '-' }}</el-descriptions-item>
      </el-descriptions>
    </el-dialog>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { storeToRefs } from 'pinia'
import { use登录状态Store } from '../stores/登录状态.js'
import { 获取接口错误提示 } from '../api/请求客户端.js'
import {
  查找软件名称,
  格式化代理归属,
  格式化时间,
  格式化时长 as 时长文本,
  最大时长分钟,
  最小时长分钟,
  时长预设
} from '../utils/时长工具.js'

const stores = use登录状态Store()
const post = stores.post
const { 软件列表, 代理列表, 代理时长价格列表: 代理价格列表 } = storeToRefs(stores)
const 是代理账号 = computed(() => Boolean(stores.是代理账号))
const 加载中 = ref(false)
const 列表 = ref([])
const 已选 = ref([])
const 生成结果 = ref('')
const 代理价格软件 = ref(0)
const 分页 = reactive({ page: 1, page_size: 50, total: 0 })
const 筛选 = reactive({ software: '', card_state: '', card: '', notes: '' })
const 排序 = reactive({ sort_by: '', sort_order: '' })
const 生成框 = reactive({ 显示: false, 加载中: false, 提交中: false, software: 0, duration_minutes: 1440, num: 1, random: true, cards: '', latest_activation_days: -1, notes: '', config_content: '' })
const 编辑框 = reactive({ 显示: false, 加载中: false, card: '', duration_minutes: 0, card_state: 2, notes: '', config_content: '' })
const 续费框 = reactive({ 显示: false, 加载中: false, cards: [], duration_minutes: 1440 })
const 详情框 = reactive({ 显示: false, 加载中: false, data: {} })
const 可发卡软件列表 = computed(() => {
  const ids = new Set(代理价格列表.value.map((item) => Number(item.software)))
  return 软件列表.value.filter((item) => ids.has(Number(item.ID)))
})
const 显示错误 = (error) => ElMessage.error(获取接口错误提示(error))
const 软件名称 = (id) => 查找软件名称(id, 软件列表.value)
const 代理名称 = (id) => 格式化代理归属(id, 代理列表.value)
const 状态文本 = (row) => {
  const state = Number(row.card_state)
  if (state === 4) return '冻结'
  if (state === 5) return '已暂停'
  if (state === 6) return '已用于充值'
  if (!row.end_time) return '未激活'
  return new Date(row.end_time).getTime() > Date.now() ? '已激活' : '已到期'
}
const 状态类型 = (row) => ({ 未激活: 'info', 已激活: 'success', 已到期: 'warning', 冻结: 'danger', 已暂停: 'warning', 已用于充值: 'info' })[状态文本(row)] || 'info'

const 查询软件 = () => stores.查询软件列表().then(() => {
  if (!是代理账号.value && !软件列表.value.some((item) => Number(item.ID) === Number(生成框.software))) {
    生成框.software = 软件列表.value[0]?.ID || 0
  }
  if (是代理账号.value && !代理价格软件.value && 可发卡软件列表.value.length) {
    代理价格软件.value = 可发卡软件列表.value[0].ID
    生成框.software = 代理价格软件.value
  }
})
const 查询代理 = () => {
  if (是代理账号.value) return Promise.resolve()
  return stores.查询代理列表()
}
// 代理价格接口一次返回全部软件的启用锚点。页面只把有价格的软​​件放入
// 发卡选择框，避免代理选中软件后才发现没有发卡权限。
const 查询代理价格 = () => {
  if (!是代理账号.value) return Promise.resolve()
  return stores.查询代理时长价格列表()
    .then(() => {
      if (!可发卡软件列表.value.some((item) => Number(item.ID) === Number(代理价格软件.value))) {
        代理价格软件.value = 可发卡软件列表.value[0]?.ID || 0
      }
      if (!可发卡软件列表.value.some((item) => Number(item.ID) === Number(生成框.software))) {
        生成框.software = 代理价格软件.value || 0
      }
    })
    .catch(显示错误)
}
const 查询列表 = (resetPage = false) => {
  if (resetPage) 分页.page = 1
  加载中.value = true
  return post('/duration_card/list', { ...筛选, ...排序, page: 分页.page, page_size: 分页.page_size })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '查询时长卡失败')
      列表.value = res.data.data || []
      分页.total = Number(res.data.num || 0)
      已选.value = []
    })
    .catch(显示错误)
    .finally(() => { 加载中.value = false })
}
const 排序变化 = ({ prop, order }) => {
  if (!prop || !order) return
  排序.sort_by = prop
  排序.sort_order = order === 'ascending' ? 'asc' : 'desc'
  查询列表(true)
}
const 重置筛选 = () => {
  Object.assign(筛选, { software: '', card_state: '', card: '', notes: '' })
  Object.assign(排序, { sort_by: '', sort_order: '' })
  查询列表(true)
}
const 选择变化 = (rows) => { 已选.value = rows.map((row) => row.card) }
const 打开生成 = () => {
  const software = 是代理账号.value ? 代理价格软件.value || 可发卡软件列表.value[0]?.ID || 0 : 软件列表.value[0]?.ID || 0
  Object.assign(生成框, { 显示: true, 加载中: false, 提交中: false, software, duration_minutes: 1440, num: 1, random: true, cards: '', latest_activation_days: -1, notes: '', config_content: '' })
  生成结果.value = ''
}
const 代理时长价格提醒 = computed(() => {
  if (!是代理账号.value) return { type: 'success', text: '' }
  const duration = Number(生成框.duration_minutes)
  const count = Number(生成框.num)
  const prices = 代理价格列表.value
    .filter((item) => Number(item.software) === Number(生成框.software))
    .sort((a, b) => Number(a.duration_minutes) - Number(b.duration_minutes))
  if (!Number.isInteger(duration) || duration < 最小时长分钟 || duration > 最大时长分钟 || !Number.isInteger(count) || count < 1 || count > 500) {
    return { type: 'danger', text: '时长或数量不正确' }
  }
  if (!prices.length) return { type: 'danger', text: '暂无可用价格' }

  let pricePerCardCents
  let charge
  const exact = prices.find((item) => Number(item.duration_minutes) === duration)
  if (exact) {
    pricePerCardCents = Math.round(Number(exact.price) * 100)
    charge = Math.ceil(pricePerCardCents * count / 100)
  } else {
    if (duration < Number(prices[0].duration_minutes) || duration > Number(prices[prices.length - 1].duration_minutes)) {
      return { type: 'danger', text: `时长需在${时长文本(prices[0].duration_minutes)}至${时长文本(prices[prices.length - 1].duration_minutes)}之间` }
    }
    let lower
    let upper
    for (const price of prices) {
      if (Number(price.duration_minutes) < duration) lower = price
      if (Number(price.duration_minutes) > duration) {
        upper = price
        break
      }
    }
    if (!lower || !upper) return { type: 'danger', text: '没有可用的相邻价格锚点' }
    const lowerCents = Math.round(Number(lower.price) * 100)
    const upperCents = Math.round(Number(upper.price) * 100)
    const source = lowerCents * Number(upper.duration_minutes) >= upperCents * Number(lower.duration_minutes) ? lower : upper
    const sourceCents = Math.round(Number(source.price) * 100)
    pricePerCardCents = Math.round(duration * sourceCents / Number(source.duration_minutes))
    charge = Math.ceil(duration * sourceCents * count / (Number(source.duration_minutes) * 100))
  }
  const next = prices.find((item) => Number(item.duration_minutes) > duration)
  if (next && charge >= Math.ceil(Math.round(Number(next.price) * 100) * count / 100)) {
    const nextCharge = Math.ceil(Math.round(Number(next.price) * 100) * count / 100)
    const difference = charge - nextCharge
    const advantage = difference > 0 ? `还能省${difference}点` : '无需加价'
    return {
      type: 'danger',
      text: `当前${时长文本(duration)}共${charge}点，${时长文本(next.duration_minutes)}${nextCharge}点，${advantage}，升级更划算`,
      recommendedDuration: Number(next.duration_minutes)
    }
  }
  return { type: 'success', text: `单张${(pricePerCardCents / 100).toFixed(2)}点，共${charge}点` }
})
const 套用推荐时长 = () => {
  if (代理时长价格提醒.value.recommendedDuration) {
    生成框.duration_minutes = 代理时长价格提醒.value.recommendedDuration
  }
}
const 检查生成参数 = () => {
  if (!生成框.software || !Number.isInteger(生成框.duration_minutes) || 生成框.duration_minutes < 最小时长分钟 || 生成框.duration_minutes > 最大时长分钟 || !Number.isInteger(生成框.num) || 生成框.num < 1 || 生成框.num > 500) {
    ElMessage.warning('请选择软件并填写有效的时长和数量')
    return false
  }
  if (!生成框.random && !生成框.cards.trim()) {
    ElMessage.warning('请输入指定卡密')
    return false
  }
  return true
}
const 生成 = async () => {
  if (生成框.提交中) return
  if (!检查生成参数()) return
  const 价格提醒 = 代理时长价格提醒.value
  if (是代理账号.value && 价格提醒.type === 'danger' && !价格提醒.recommendedDuration) {
    ElMessage.warning(价格提醒.text)
    return
  }
  生成框.提交中 = true
  if (是代理账号.value && 价格提醒.recommendedDuration) {
    try {
      await ElMessageBox.confirm(
        `${价格提醒.text}。仍要按当前时长生成吗？`,
        '价格提醒',
        { type: 'warning', confirmButtonText: '仍然生成', cancelButtonText: '返回调整' }
      )
    } catch {
      生成框.提交中 = false
      return
    }
  }
  生成框.加载中 = true
  const latestActivationMinutes = 生成框.latest_activation_days < 0 ? -1 : 生成框.latest_activation_days * 1440
  // 只提交后端定义的时长卡字段，避免把弹窗状态和预览对象混入请求。
  post('/duration_card/create', {
    software: 生成框.software,
    duration_minutes: 生成框.duration_minutes,
    num: 生成框.num,
    cards: 生成框.cards,
    random: 生成框.random,
    notes: 生成框.notes,
    config_content: 生成框.config_content,
    latest_activation_minutes: latestActivationMinutes
  })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '生成时长卡失败')
      生成结果.value = res.data.data || ''
      ElMessage.success(res.data.msg || '生成成功')
      查询列表(true)
    })
    .catch(显示错误)
    .finally(() => {
      生成框.加载中 = false
      生成框.提交中 = false
    })
}
const 可编辑状态 = (row) => [1, 2, 3, 4].includes(Number(row.card_state))
const 可续费状态 = (row) => {
  const state = Number(row.card_state)
  return (state === 2 && row.end_time) || (state === 5 && Number(row.paused_remaining_minutes || 0) > 0)
}
const 打开编辑 = (row) => Object.assign(编辑框, { 显示: true, 加载中: false, card: row.card, duration_minutes: row.duration_minutes, card_state: row.card_state === 4 ? 4 : 2, notes: row.notes || '', config_content: row.config_content || '' })
const 保存编辑 = () => {
  if (编辑框.加载中) return
  编辑框.加载中 = true
  return post('/duration_card/save', { card: 编辑框.card, card_state: 编辑框.card_state, notes: 编辑框.notes, config_content: 编辑框.config_content })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '保存失败')
      ElMessage.success('保存成功')
      编辑框.显示 = false
      查询列表(false)
    })
    .catch(显示错误)
    .finally(() => {
      编辑框.加载中 = false
    })
}
const 批量修改状态 = (state) => post('/duration_card/state', { cards: 已选.value, card_state: state })
  .then((res) => { if (!res.data?.state) throw new Error(res.data?.msg || '修改失败'); ElMessage.success(res.data.msg || '修改成功'); 查询列表(false) })
  .catch(显示错误)
const 批量删除 = () => ElMessageBox.confirm(`确定删除已选的 ${已选.value.length} 张时长卡？`, '确认删除', { type: 'warning' })
  .then(() => post('/duration_card/delete', { cards: 已选.value }))
  .then((res) => { if (!res.data?.state) throw new Error(res.data?.msg || '删除失败'); ElMessage.success(res.data.msg || '删除成功'); 查询列表(false) })
  .catch((error) => { if (error !== 'cancel' && error !== 'close') 显示错误(error) })
const 打开单张续费 = (row) => { Object.assign(续费框, { 显示: true, 加载中: false, cards: [row.card], duration_minutes: 1440 }) }
const 打开续费 = () => { if (!已选.value.length) return; Object.assign(续费框, { 显示: true, 加载中: false, cards: [...已选.value], duration_minutes: 1440 }) }
const 续费 = () => {
  if (续费框.加载中) return
  if (!续费框.cards.length || !Number.isInteger(续费框.duration_minutes) || 续费框.duration_minutes < 最小时长分钟 || 续费框.duration_minutes > 最大时长分钟) {
    ElMessage.warning('请填写有效的续费时长')
    return
  }
  续费框.加载中 = true
  return post('/duration_card/renew', { cards: 续费框.cards, duration_minutes: 续费框.duration_minutes })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '续费失败')
      ElMessage.success(res.data.msg || '续费成功')
      续费框.显示 = false
      查询列表(false)
    })
    .catch(显示错误)
    .finally(() => {
      续费框.加载中 = false
    })
}
const 删除单张 = (row) => ElMessageBox.confirm(`确定删除时长卡“${row.card}”？`, '确认删除', { type: 'warning' })
  .then(() => post('/duration_card/delete', { cards: [row.card] }))
  .then((res) => { if (!res.data?.state) throw new Error(res.data?.msg || '删除失败'); ElMessage.success('删除成功'); 查询列表(false) })
  .catch((error) => { if (error !== 'cancel' && error !== 'close') 显示错误(error) })
const 查看详情 = (row) => {
  详情框.显示 = true
  详情框.加载中 = true
  post('/duration_card/detail', { card: row.card }).then((res) => {
    if (!res.data?.state) throw new Error(res.data?.msg || '读取详情失败')
    详情框.data = res.data.data || {}
  }).catch(显示错误).finally(() => { 详情框.加载中 = false })
}
const 导出当前页 = () => {
  const text = 列表.value.map((row) => row.card).join('\n')
  const blob = new Blob([text], { type: 'text/plain;charset=utf-8' })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = `时长卡-${new Date().toISOString().slice(0, 10)}.txt`
  link.click()
  URL.revokeObjectURL(link.href)
}
const 复制文本 = async (text) => {
  try { await navigator.clipboard.writeText(text); ElMessage.success('已复制') } catch { ElMessage.warning('复制失败，请手动复制') }
}

onMounted(() => {
  if (是代理账号.value) {
    Promise.all([查询软件(), 查询代理价格(), 查询列表(true)]).catch(() => {})
  } else {
    Promise.all([查询软件(), 查询代理(), 查询列表(true)]).catch(() => {})
  }
})
</script>

<style scoped>
.页面 { padding: 16px; color: #e6eaf2; }
.页面标题行, .批量操作 { display: flex; align-items: center; gap: 12px; }
.页面标题行 { justify-content: space-between; margin-bottom: 14px; }
h2 { margin: 0 0 6px; }
.说明 { margin: 0; color: #aeb6c3; font-size: 13px; }
.筛选卡片 { margin-bottom: 14px; }
.批量操作 { margin-top: 2px; color: #aeb6c3; }
.批量操作 span { margin-right: 4px; }
.分页 { justify-content: flex-end; margin-top: 16px; }
.单位, .弱文本 { margin-left: 8px; color: #8c98aa; font-size: 12px; }
.价格提醒 { display: inline-flex; align-items: center; gap: 4px; margin-left: 8px; max-width: 100%; flex-wrap: wrap; }
.页面 :deep(.el-table .cell) { white-space: nowrap; word-break: normal; }
.卡密文本 { color: #79b4ff; font-family: monospace; }
.复制按钮 { margin-top: 8px; }
</style>
