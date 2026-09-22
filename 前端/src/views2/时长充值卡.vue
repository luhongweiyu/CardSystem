<template>
  <section class="页面" v-loading="加载中">
    <div class="标题行">
      <div>
        <h2>时长充值卡</h2>
        <p class="说明">充值卡不能登录，只能给同软件的已激活或已暂停时长卡增加时长。</p>
      </div>
      <el-button type="primary" @click="打开生成">生成充值卡</el-button>
    </div>

    <el-card shadow="never" class="筛选卡片">
      <el-form :inline="true" @submit.prevent>
        <el-form-item label="软件">
          <el-select v-model="筛选.software" clearable placeholder="全部软件" style="width: 180px" @change="查询列表(true)">
            <el-option v-for="item in 软件列表" :key="item.ID" :label="item.Software" :value="item.ID" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="筛选.card_state" clearable placeholder="全部状态" style="width: 130px" @change="查询列表(true)">
            <el-option label="正常" :value="2" />
            <el-option label="冻结" :value="4" />
          </el-select>
        </el-form-item>
        <el-form-item label="卡密">
          <el-input v-model="筛选.card" clearable placeholder="支持模糊搜索" style="width: 220px" @keyup.enter="查询列表(true)" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="查询列表(true)">查询</el-button>
          <el-button @click="重置筛选">重置</el-button>
        </el-form-item>
      </el-form>
      <div class="提示行">当前账号：{{ 是代理账号 ? '小伙伴' : '管理员' }}；充值卡使用后会扣减剩余次数，不能恢复。</div>
    </el-card>

    <el-table :data="列表" border stripe row-key="card">
      <el-table-column prop="card" label="卡密" min-width="190" show-overflow-tooltip />
      <el-table-column prop="software" label="软件" width="150" show-overflow-tooltip>
        <template #default="scope">{{ 软件名称(scope.row.software) }}</template>
      </el-table-column>
      <el-table-column v-if="!是代理账号" label="归属代理" width="140">
        <template #default="scope">{{ 代理名称(scope.row.agent_id) }}</template>
      </el-table-column>
      <el-table-column prop="duration_minutes" label="每次增加" width="115">
        <template #default="scope">{{ 时长文本(scope.row.duration_minutes) }}</template>
      </el-table-column>
      <el-table-column prop="initial_uses" label="初始次数" width="90" />
      <el-table-column prop="remaining_uses" label="剩余次数" width="90" />
      <el-table-column label="状态" width="95">
        <template #default="scope">
          <el-tag :type="状态类型(scope.row)">{{ 状态文本(scope.row) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="expires_at" label="有效期" width="180">
        <template #default="scope">{{ 格式化时间(scope.row.expires_at) || '长期有效' }}</template>
      </el-table-column>
      <el-table-column prop="create_time" label="生成时间" width="175">
        <template #default="scope">{{ 格式化时间(scope.row.create_time) }}</template>
      </el-table-column>
      <el-table-column prop="notes" label="备注" min-width="160" show-overflow-tooltip />
      <el-table-column label="操作" width="190" fixed="right">
        <template #default="scope">
          <el-button link type="primary" @click="查看详情(scope.row)">详情</el-button>
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
      @size-change="查询列表(true)"
      @current-change="查询列表(false)"
    />

    <el-dialog v-model="生成框.显示" title="生成时长充值卡" width="560px" destroy-on-close>
      <el-form label-width="120px" v-loading="生成框.提交中">
        <el-form-item label="所属软件" required>
          <el-select v-model="生成框.software" placeholder="请选择软件" style="width: 280px">
            <el-option v-for="item in 可发卡软件列表" :key="item.ID" :label="item.Software" :value="item.ID" />
          </el-select>
        </el-form-item>
        <el-form-item label="每次增加时长" required>
          <el-input-number v-model="生成框.duration_minutes" :min="最小时长分钟" :max="最大时长分钟" :precision="0" controls-position="right" />
          <span class="单位">分钟（{{ 时长文本(生成框.duration_minutes) }}）</span>
        </el-form-item>
        <el-form-item label="快捷时长">
          <el-select v-model="生成框.duration_minutes" style="width: 220px">
            <el-option v-for="item in 时长预设" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="每张使用次数" required>
          <el-input-number v-model="生成框.uses" :min="1" :max="1000" :precision="0" controls-position="right" />
          <span class="单位">次</span>
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
        <el-form-item label="有效期">
          <el-date-picker v-model="生成框.expires_at" type="datetime" value-format="YYYY-MM-DD HH:mm:ss" placeholder="不设置表示长期有效" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="生成框.notes" maxlength="500" show-word-limit />
        </el-form-item>
        <el-form-item v-if="生成结果" label="生成结果">
          <el-input v-model="生成结果" type="textarea" :rows="5" readonly />
          <el-button class="复制按钮" @click="复制文本(生成结果)">复制</el-button>
        </el-form-item>
        <el-alert v-if="是代理账号" type="info" :closable="false" title="代理余额会在服务端按充值时长、次数和数量重新计价并一次性扣除。" />
      </el-form>
      <template #footer>
        <el-button @click="生成框.显示 = false">取消</el-button>
        <el-button type="primary" :loading="生成框.提交中" @click="生成">确认生成</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="编辑框.显示" title="编辑时长充值卡" width="480px" destroy-on-close>
      <el-form label-width="100px" v-loading="编辑框.加载中">
        <el-form-item label="卡密"><span class="卡密文本">{{ 编辑框.card }}</span></el-form-item>
        <el-form-item label="每次增加"><span>{{ 时长文本(编辑框.duration_minutes) }}</span></el-form-item>
        <el-form-item label="剩余次数"><span>{{ 编辑框.remaining_uses }} / {{ 编辑框.initial_uses }}</span></el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="编辑框.card_state">
            <el-radio :label="2">正常</el-radio>
            <el-radio :label="4">冻结</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="编辑框.notes" maxlength="500" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="编辑框.显示 = false">取消</el-button>
        <el-button type="primary" :loading="编辑框.加载中" @click="保存编辑">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="详情框.显示" title="时长充值卡详情" width="720px" destroy-on-close>
      <el-descriptions :column="2" border v-loading="详情框.加载中">
        <el-descriptions-item label="卡密">{{ 详情框.data.card }}</el-descriptions-item>
        <el-descriptions-item label="软件">{{ 软件名称(详情框.data.software) }}</el-descriptions-item>
        <el-descriptions-item label="每次增加">{{ 时长文本(详情框.data.duration_minutes) }}</el-descriptions-item>
        <el-descriptions-item label="使用次数">{{ 详情框.data.remaining_uses }} / {{ 详情框.data.initial_uses }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ 状态文本(详情框.data) }}</el-descriptions-item>
        <el-descriptions-item label="有效期">{{ 格式化时间(详情框.data.expires_at) || '长期有效' }}</el-descriptions-item>
        <el-descriptions-item label="生成时间">{{ 格式化时间(详情框.data.create_time) }}</el-descriptions-item>
        <el-descriptions-item label="备注" :span="2">{{ 详情框.data.notes || '-' }}</el-descriptions-item>
        <el-descriptions-item label="使用记录" :span="2">
          <pre class="记录">{{ 详情框.data.record || '暂无使用记录' }}</pre>
        </el-descriptions-item>
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
const 生成结果 = ref('')
const 筛选 = reactive({ software: '', card_state: '', card: '' })
const 分页 = reactive({ page: 1, page_size: 50, total: 0 })
const 生成框 = reactive({ 显示: false, 提交中: false, software: 0, duration_minutes: 1440, uses: 1, num: 1, random: true, cards: '', expires_at: '', notes: '' })
const 编辑框 = reactive({ 显示: false, 加载中: false, card: '', duration_minutes: 0, initial_uses: 0, remaining_uses: 0, card_state: 2, notes: '' })
const 详情框 = reactive({ 显示: false, 加载中: false, data: {} })
const 可发卡软件列表 = computed(() => {
  if (!是代理账号.value) return 软件列表.value
  const ids = new Set(代理价格列表.value.map((item) => Number(item.software)))
  return 软件列表.value.filter((item) => ids.has(Number(item.ID)))
})

const 显示错误 = (error) => ElMessage.error(获取接口错误提示(error))
const 软件名称 = (id) => 查找软件名称(id, 软件列表.value)
const 代理名称 = (id) => 格式化代理归属(id, 代理列表.value)
const 状态文本 = (row = {}) => {
  if (Number(row.card_state) === 4) return '冻结'
  if (Number(row.remaining_uses || 0) <= 0) return '已用完'
  if (row.expires_at && new Date(row.expires_at).getTime() <= Date.now()) return '已过期'
  return '正常'
}
const 状态类型 = (row) => ({ 正常: 'success', 冻结: 'danger', 已用完: 'info', 已过期: 'warning' })[状态文本(row)] || 'info'

const 查询软件 = () => stores.查询软件列表().then(() => {
  if (!软件列表.value.some((item) => Number(item.ID) === Number(生成框.software))) {
    生成框.software = 可发卡软件列表.value[0]?.ID || 软件列表.value[0]?.ID || 0
  }
})
const 查询代理价格 = () => {
  if (!是代理账号.value) return Promise.resolve()
  return stores.查询代理时长价格列表().then(() => {
    if (!可发卡软件列表.value.some((item) => Number(item.ID) === Number(生成框.software))) {
      生成框.software = 可发卡软件列表.value[0]?.ID || 0
    }
  })
}
const 查询代理 = () => {
  if (是代理账号.value) return Promise.resolve()
  return stores.查询代理列表()
}
const 查询列表 = (resetPage = false) => {
  if (resetPage) 分页.page = 1
  加载中.value = true
  return post('/duration_recharge_card/list', { ...筛选, page: 分页.page, page_size: 分页.page_size })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '查询充值卡失败')
      列表.value = res.data.data || []
      分页.total = Number(res.data.num || 0)
    })
    .catch(显示错误)
    .finally(() => { 加载中.value = false })
}
const 重置筛选 = () => {
  Object.assign(筛选, { software: '', card_state: '', card: '' })
  查询列表(true)
}
const 打开生成 = () => {
  Object.assign(生成框, { 显示: true, 提交中: false, software: 可发卡软件列表.value[0]?.ID || 0, duration_minutes: 1440, uses: 1, num: 1, random: true, cards: '', expires_at: '', notes: '' })
  生成结果.value = ''
}
const 时间转ISO = (value) => {
  if (!value) return null
  const date = new Date(String(value).replace(' ', 'T'))
  return Number.isNaN(date.getTime()) ? null : date.toISOString()
}
const 生成 = () => {
  if (生成框.提交中) return
  if (!生成框.software || !Number.isInteger(生成框.duration_minutes) || 生成框.duration_minutes < 最小时长分钟 || 生成框.duration_minutes > 最大时长分钟 || !Number.isInteger(生成框.uses) || 生成框.uses < 1 || 生成框.uses > 1000 || !Number.isInteger(生成框.num) || 生成框.num < 1 || 生成框.num > 500) {
    ElMessage.warning('请填写有效的软件、时长、次数和数量')
    return
  }
  if (!生成框.random && !生成框.cards.trim()) {
    ElMessage.warning('请输入指定卡密')
    return
  }
  生成框.提交中 = true
  post('/duration_recharge_card/create', {
    software: 生成框.software,
    duration_minutes: 生成框.duration_minutes,
    uses: 生成框.uses,
    num: 生成框.num,
    cards: 生成框.cards,
    random: 生成框.random,
    expires_at: 时间转ISO(生成框.expires_at),
    notes: 生成框.notes
  })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '生成充值卡失败')
      生成结果.value = res.data.data || ''
      ElMessage.success(res.data.msg || '生成成功')
      查询列表(true)
    })
    .catch(显示错误)
    .finally(() => { 生成框.提交中 = false })
}
const 打开编辑 = (row) => Object.assign(编辑框, {
  显示: true,
  加载中: false,
  card: row.card,
  duration_minutes: Number(row.duration_minutes || 0),
  initial_uses: Number(row.initial_uses || 0),
  remaining_uses: Number(row.remaining_uses || 0),
  card_state: Number(row.card_state) === 4 ? 4 : 2,
  notes: row.notes || ''
})
const 保存编辑 = () => {
  if (编辑框.加载中) return
  编辑框.加载中 = true
  post('/duration_recharge_card/save', { card: 编辑框.card, card_state: 编辑框.card_state, notes: 编辑框.notes })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '保存失败')
      ElMessage.success('保存成功')
      编辑框.显示 = false
      查询列表(false)
    })
    .catch(显示错误)
    .finally(() => { 编辑框.加载中 = false })
}
const 查看详情 = (row) => {
  详情框.显示 = true
  详情框.加载中 = true
  post('/duration_recharge_card/detail', { card: row.card })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '读取详情失败')
      详情框.data = res.data.data || {}
    })
    .catch(显示错误)
    .finally(() => { 详情框.加载中 = false })
}
const 删除单张 = (row) => ElMessageBox.confirm(`确定删除时长充值卡“${row.card}”？已消耗的充值记录不会恢复。`, '确认删除', { type: 'warning' })
  .then(() => post('/duration_recharge_card/delete', { cards: [row.card] }))
  .then((res) => {
    if (!res.data?.state) throw new Error(res.data?.msg || '删除失败')
    ElMessage.success(res.data.msg || '删除成功')
    查询列表(false)
  })
  .catch((error) => { if (error !== 'cancel' && error !== 'close') 显示错误(error) })
const 复制文本 = async (value) => {
  try {
    await navigator.clipboard.writeText(value || '')
    ElMessage.success('已复制')
  } catch {
    ElMessage.warning('复制失败，请手动复制')
  }
}

onMounted(() => {
  Promise.all([查询软件(), 查询代理价格(), 查询代理(), 查询列表(true)]).catch(() => {})
})
</script>

<style scoped>
.页面 { padding: 16px; color: #e6eaf2; }
.标题行 { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 14px; }
h2 { margin: 0 0 6px; }
.说明 { margin: 0; color: #aeb6c3; font-size: 13px; }
.筛选卡片 { margin-bottom: 14px; }
.提示行 { color: #8f9bad; font-size: 12px; }
.分页 { justify-content: flex-end; margin-top: 16px; }
.页面 :deep(.el-table .cell) { white-space: nowrap; word-break: normal; }
.单位 { margin-left: 8px; color: #8c98aa; font-size: 12px; }
.卡密文本 { color: #79b4ff; font-family: monospace; }
.复制按钮 { margin-top: 8px; }
.记录 { max-height: 220px; margin: 0; overflow: auto; white-space: pre-wrap; word-break: break-all; }
</style>
