<template>
  <section class="页面" v-loading="加载中">
    <div class="页面标题行">
      <div>
        <h2>软件与点卡计费</h2>
        <p class="说明">客户端按分钟选择授权时长，实际扣点价格始终由服务端决定。</p>
      </div>
      <el-button v-if="!是代理账号" type="primary" @click="打开软件编辑">新增软件</el-button>
    </div>

    <el-table :data="软件列表" border stripe row-key="ID">
      <el-table-column prop="ID" label="ID" width="70" />
      <el-table-column prop="Software" label="软件名称" min-width="170" />
      <el-table-column label="默认授权时长" width="130">
        <template #default="scope">{{ scope.row.default_period_minutes }} 分钟</template>
      </el-table-column>
      <el-table-column label="心跳间隔" width="130">
        <template #default="scope">{{ 周期文本(scope.row.heartbeat_interval_seconds) }}</template>
      </el-table-column>
      <el-table-column label="自动离线时间" width="140">
        <template #default="scope">{{ scope.row.online_grace_minutes || 60 }} 分钟</template>
      </el-table-column>
      <el-table-column prop="Bulletin" label="公告" min-width="220" show-overflow-tooltip />
      <el-table-column v-if="!是代理账号" label="操作" width="230" fixed="right">
        <template #default="scope">
          <el-button link type="primary" @click="编辑软件(scope.row)">编辑</el-button>
          <el-button link type="success" @click="打开价格(scope.row)">点卡计费方案</el-button>
          <el-button link type="danger" @click="删除软件(scope.row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-card v-if="是代理账号" shadow="never" class="代理提示">
      渠道合伙人只能使用管理员配置的软件和点数价格。
    </el-card>

    <section v-if="!是代理账号" class="代理区">
      <div class="子标题行">
        <h3>渠道合伙人</h3>
        <el-button type="primary" plain @click="打开代理创建">新增渠道合伙人</el-button>
      </div>
      <el-table :data="代理列表" border stripe>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" label="账号" width="160" />
        <el-table-column prop="balance" label="余额（点）" width="120" />
        <el-table-column label="操作" min-width="260">
          <template #default="scope">
            <el-button link type="primary" @click="编辑代理(scope.row)">计费方案与密码</el-button>
            <el-button link type="success" @click="打开代理充值(scope.row)">充值点数</el-button>
            <el-button link type="danger" @click="删除代理(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <!-- 软件编辑 -->
    <el-dialog v-model="软件框.显示" :title="软件框.id ? '编辑软件' : '新增软件'" width="500px" destroy-on-close>
      <el-form label-width="125px" v-loading="软件框.加载中">
        <el-form-item label="软件名称" required>
          <el-input v-model="软件框.software" maxlength="64" />
        </el-form-item>
        <el-form-item label="默认授权时长（分钟）" required>
          <el-input-number
            v-model="软件框.default_period_minutes"
            :min="最小计费周期分钟"
            :max="最大计费周期分钟"
            :precision="0"
            controls-position="right"
          />
        </el-form-item>
        <el-form-item label="心跳间隔（秒）" required>
          <el-input-number
            v-model="软件框.heartbeat_interval_seconds"
            :min="1"
            :max="86400"
            :precision="0"
            controls-position="right"
          />
        </el-form-item>
        <el-form-item label="自动离线时间（分钟）" required>
          <el-input-number
            v-model="软件框.online_grace_minutes"
            :min="0"
            :max="4320"
            :precision="0"
            controls-position="right"
          />
          <div class="字段说明">0 表示默认 60 分钟后自动离线</div>
        </el-form-item>
        <el-form-item label="公告">
          <el-input v-model="软件框.bulletin" type="textarea" :rows="4" maxlength="5000" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="软件框.显示 = false">取消</el-button>
        <el-button type="primary" @click="保存软件">保存</el-button>
      </template>
    </el-dialog>

    <!-- 点卡计费方案 -->
    <el-dialog v-model="价格框.显示" title="点卡计费方案" width="760px" destroy-on-close>
      <div class="价格标题">
        <span>{{ 价格框.softwareName }}</span>
        <el-button type="primary" size="small" @click="新增价格">新增计费方案</el-button>
      </div>
      <el-table :data="价格框.rows" border>
        <el-table-column label="授权时长" width="150">
          <template #default="scope">{{ scope.row.period_minutes }} 分钟</template>
        </el-table-column>
        <el-table-column prop="cost" label="扣点" width="90" />
        <el-table-column label="状态" width="100">
          <template #default="scope">
            <el-tag :type="scope.row.enabled ? 'success' : 'info'">
              {{ scope.row.enabled ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="默认方案" width="90">
          <template #default="scope">{{ scope.row.is_default ? '是' : '' }}</template>
        </el-table-column>
        <el-table-column label="操作" min-width="150">
          <template #default="scope">
            <el-button link type="primary" @click="编辑价格(scope.row)">编辑</el-button>
            <el-button link type="danger" @click="删除价格(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!价格框.rows.length" description="尚未配置点卡计费方案" />
    </el-dialog>

    <el-dialog
      v-model="价格编辑框.显示"
      :title="价格编辑框.id ? '编辑点卡计费方案' : '新增点卡计费方案'"
      width="420px"
      destroy-on-close
    >
      <el-form label-width="120px">
        <el-form-item label="授权时长（分钟）">
          <el-input-number
            v-model="价格编辑框.period_minutes"
            :min="最小计费周期分钟"
            :max="最大计费周期分钟"
            :precision="0"
            controls-position="right"
            :disabled="!!价格编辑框.id"
          />
        </el-form-item>
        <el-form-item label="扣点数">
          <el-input-number
            v-model="价格编辑框.cost"
            :min="1"
            :max="1000000000"
            :precision="0"
            controls-position="right"
          />
        </el-form-item>
        <el-form-item label="启用"><el-switch v-model="价格编辑框.enabled" /></el-form-item>
        <el-form-item label="设为默认方案"><el-switch v-model="价格编辑框.is_default" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="价格编辑框.显示 = false">取消</el-button>
        <el-button type="primary" @click="保存价格">保存</el-button>
      </template>
    </el-dialog>

    <!-- 渠道合伙人 -->
    <el-dialog v-model="代理框.显示" title="新增渠道合伙人" width="420px" destroy-on-close>
      <el-form label-width="100px">
        <el-form-item label="账号"><el-input v-model="代理框.name" maxlength="32" /></el-form-item>
        <el-form-item label="密码">
          <el-input v-model="代理框.password" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="代理框.显示 = false">取消</el-button>
        <el-button type="primary" @click="创建代理">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="代理编辑框.显示" title="渠道合伙人设置" width="620px" destroy-on-close>
      <p>账号：{{ 代理编辑框.name }}　当前余额：{{ 代理编辑框.balance }} 点</p>
      <el-form label-width="150px">
        <el-form-item v-for="item in 软件列表" :key="item.ID" :label="`${item.Software}（每点价格）`">
          <el-input-number
            v-model="代理编辑框.prices[item.ID]"
            :min="0"
            :max="1000000000"
            :precision="2"
            controls-position="right"
          />
          <span class="价格说明">0 表示不授权该软件</span>
        </el-form-item>
        <el-form-item label="新密码（可选）">
          <el-input v-model="代理编辑框.password" type="password" show-password placeholder="留空表示不修改" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="代理编辑框.显示 = false">取消</el-button>
        <el-button type="primary" @click="保存代理">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="代理充值框.显示" title="给渠道合伙人充值" width="420px" destroy-on-close>
      <p>账号：{{ 代理充值框.name }}</p>
      <el-form label-width="90px">
        <el-form-item label="点数">
          <el-input-number
            v-model="代理充值框.amount"
            :min="1"
            :max="1000000000"
            :precision="0"
            controls-position="right"
          />
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="代理充值框.note" maxlength="200" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="代理充值框.显示 = false">取消</el-button>
        <el-button type="primary" @click="代理充值">确认充值</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { use登录状态Store } from '../stores/登录状态.js'
import { 获取接口错误提示 } from '../api/请求客户端.js'

const 最小计费周期分钟 = 5
const 最大计费周期分钟 = 3 * 24 * 60

const stores = use登录状态Store()
const post = stores.post
const 是代理账号 = computed(() => Boolean(stores.是代理账号))
const 加载中 = ref(false)
const 软件列表 = ref([])
const 代理列表 = ref([])
const 软件框 = reactive({
  显示: false,
  加载中: false,
  id: 0,
  software: '',
  bulletin: '',
  default_period_minutes: 60,
  heartbeat_interval_seconds: 300,
  online_grace_minutes: 60
})
const 价格框 = reactive({ 显示: false, software: 0, softwareName: '', heartbeatSeconds: 300, rows: [] })
const 价格编辑框 = reactive({
  显示: false,
  id: 0,
  software: 0,
  period_minutes: 60,
  cost: 1,
  enabled: true,
  is_default: false
})
const 代理框 = reactive({ 显示: false, name: '', password: '' })
const 代理编辑框 = reactive({ 显示: false, id: 0, name: '', balance: 0, prices: {}, password: '' })
const 代理充值框 = reactive({ 显示: false, id: 0, name: '', amount: 100, note: '' })

const 显示错误 = (error) => ElMessage.error(获取接口错误提示(error))
const 周期文本 = function (seconds) {
  const value = Number(seconds || 0)
  if (!value) return '-'
  if (value % 86400 === 0) return `${value / 86400} 天`
  if (value % 3600 === 0) return `${value / 3600} 小时`
  if (value % 60 === 0) return `${value / 60} 分钟`
  return `${value} 秒`
}
const 查询软件 = function () {
  return post('/user_query_soft_list', {}).then((res) => {
    if (!res.data?.state) throw new Error(res.data?.msg || '查询软件失败')
    软件列表.value = res.data.data || []
  })
}
const 查询代理 = function () {
  if (是代理账号.value) return Promise.resolve()
  return post('/查询代理账号', {}).then((res) => {
    if (!res.data?.state) throw new Error(res.data?.msg || '查询渠道合伙人失败')
    代理列表.value = res.data.data || []
  })
}
const 打开软件编辑 = function () {
  Object.assign(软件框, {
    显示: true,
    id: 0,
    software: '',
    bulletin: '',
    default_period_minutes: 60,
    heartbeat_interval_seconds: 300,
    online_grace_minutes: 60
  })
}
const 编辑软件 = function (row) {
  Object.assign(软件框, {
    显示: true,
    id: row.ID,
    software: row.Software,
    bulletin: row.Bulletin || '',
    default_period_minutes: Number(row.default_period_minutes || 60),
    heartbeat_interval_seconds: Number(row.heartbeat_interval_seconds || 300),
    online_grace_minutes: Number(row.online_grace_minutes || 60)
  })
}
const 保存软件 = function () {
  if (!软件框.software.trim()) {
    ElMessage.warning('请输入软件名称')
    return
  }
  if (
    !Number.isInteger(软件框.default_period_minutes) ||
    软件框.default_period_minutes < 最小计费周期分钟 ||
    软件框.default_period_minutes > 最大计费周期分钟
  ) {
    ElMessage.warning('默认授权时长必须在5至4320分钟之间')
    return
  }
  if (
    !Number.isInteger(软件框.online_grace_minutes) ||
    (软件框.online_grace_minutes !== 0 && 软件框.online_grace_minutes < 最小计费周期分钟) ||
    软件框.online_grace_minutes > 最大计费周期分钟
  ) {
    ElMessage.warning('自动离线时间必须为0或5至4320分钟，0表示默认60分钟')
    return
  }
  if (
    !Number.isInteger(软件框.heartbeat_interval_seconds) ||
    软件框.heartbeat_interval_seconds < 1 ||
    软件框.heartbeat_interval_seconds > 86400
  ) {
    ElMessage.warning('心跳间隔必须在1至86400秒之间')
    return
  }
  软件框.加载中 = true
  const url = 软件框.id ? '/user_modify_bulletin' : '/user_add_soft'
  post(url, {
    id: 软件框.id,
    software: 软件框.software,
    bulletin: 软件框.bulletin,
    default_period_minutes: 软件框.default_period_minutes,
    heartbeat_interval_seconds: 软件框.heartbeat_interval_seconds,
    online_grace_minutes: 软件框.online_grace_minutes
  })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '保存软件失败')
      ElMessage.success('保存成功')
      软件框.显示 = false
      return 查询软件()
    })
    .catch(显示错误)
    .finally(() => {
      软件框.加载中 = false
    })
}
const 删除软件 = function (row) {
  ElMessageBox.confirm(`删除软件“${row.Software}”会同时删除其点卡和设备会话，流水仅保留最近30天。继续？`, '确认删除', {
    type: 'warning'
  })
    .then(() => post('/user_del_soft', { id: row.ID }))
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '删除软件失败')
      ElMessage.success('删除成功')
      查询软件()
    })
    .catch((error) => {
      if (error !== 'cancel' && error !== 'close') 显示错误(error)
    })
}

const 打开价格 = function (row) {
  Object.assign(价格框, {
    显示: true,
    software: row.ID,
    softwareName: row.Software,
    heartbeatSeconds: Number(row.heartbeat_interval_seconds || 300),
    rows: []
  })
  查询价格()
}
const 查询价格 = function () {
  return post('/point_period_price/list', { software: 价格框.software })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '查询点卡计费方案失败')
      价格框.rows = res.data.data || []
    })
    .catch(显示错误)
}
const 新增价格 = function () {
  Object.assign(价格编辑框, {
    显示: true,
    id: 0,
    software: 价格框.software,
    period_minutes: 60,
    cost: 1,
    enabled: true,
    is_default: !价格框.rows.length
  })
}
const 编辑价格 = function (row) {
  Object.assign(价格编辑框, {
    显示: true,
    id: row.id,
    software: row.software,
    period_minutes: Number(row.period_minutes || 60),
    cost: Number(row.cost),
    enabled: Boolean(row.enabled),
    is_default: Boolean(row.is_default)
  })
}
const 保存价格 = function () {
  if (
    !Number.isInteger(价格编辑框.period_minutes) ||
    价格编辑框.period_minutes < 最小计费周期分钟 ||
    价格编辑框.period_minutes > 最大计费周期分钟
  ) {
    ElMessage.warning('授权时长必须在5至4320分钟之间')
    return
  }
  post('/point_period_price/save', {
    software: 价格编辑框.software,
    period_minutes: 价格编辑框.period_minutes,
    cost: 价格编辑框.cost,
    enabled: 价格编辑框.enabled,
    is_default: 价格编辑框.is_default
  })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '保存价格失败')
      ElMessage.success('保存成功')
      价格编辑框.显示 = false
      return Promise.all([查询价格(), 查询软件()])
    })
    .catch(显示错误)
}
const 删除价格 = function (row) {
  ElMessageBox.confirm('删除后使用该授权时长的登录会被拒绝，确定删除？', '确认删除', { type: 'warning' })
    .then(() => post('/point_period_price/delete', { id: row.id }))
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '删除失败')
      查询价格()
    })
    .catch((error) => {
      if (error !== 'cancel' && error !== 'close') 显示错误(error)
    })
}

const 创建代理 = function () {
  const passwordBytes = new TextEncoder().encode(代理框.password || '').length
  if (!/^[A-Za-z0-9_]{3,32}$/.test(代理框.name)) {
    ElMessage.warning('渠道合伙人账号只能使用3至32位字母、数字或下划线')
    return
  }
  if (passwordBytes < 6 || passwordBytes > 72) {
    ElMessage.warning('密码长度必须为6至72个字节')
    return
  }
  post('/创建代理账号', { agent_name: 代理框.name, agent_password: 代理框.password })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '创建失败')
      ElMessage.success('创建成功')
      代理框.显示 = false
      Object.assign(代理框, { name: '', password: '' })
      查询代理()
    })
    .catch(显示错误)
}
const 打开代理创建 = function () {
  Object.assign(代理框, { 显示: true, name: '', password: '' })
}
const 编辑代理 = function (row) {
  let prices = {}
  try {
    prices = typeof row.prices === 'string' ? JSON.parse(row.prices || '{}') : row.prices || {}
  } catch {
    prices = {}
  }
  const normalized = {}
  软件列表.value.forEach((item) => {
    const value = Number(prices[item.ID] ?? prices[String(item.ID)] ?? 0)
    normalized[item.ID] = Number.isFinite(value) && value >= 0 ? value : 0
  })
  Object.assign(代理编辑框, {
    显示: true,
    id: row.id,
    name: row.name,
    balance: row.balance,
    prices: normalized,
    password: ''
  })
}
const 保存代理 = function () {
  if (代理编辑框.password) {
    const passwordBytes = new TextEncoder().encode(代理编辑框.password).length
    if (passwordBytes < 6 || passwordBytes > 72) {
      ElMessage.warning('新密码长度必须为6至72个字节')
      return
    }
  }
  // 价格为 0 的软件不写入映射，缺少映射即表示代理无权为该软件发卡。
  const prices = {}
  Object.keys(代理编辑框.prices).forEach((key) => {
    const value = Number(代理编辑框.prices[key] || 0)
    if (Number.isFinite(value) && value > 0) prices[key] = value
  })
  post('/设置代理账号', {
    data: { id: 代理编辑框.id, password: 代理编辑框.password, prices: JSON.stringify(prices) }
  })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '保存渠道合伙人失败')
      ElMessage.success('保存成功')
      代理编辑框.显示 = false
      查询代理()
    })
    .catch(显示错误)
}
const 打开代理充值 = function (row) {
  Object.assign(代理充值框, { 显示: true, id: row.id, name: row.name, amount: 100, note: '' })
}
const 代理充值 = function () {
  if (!代理充值框.amount || 代理充值框.amount < 1) {
    ElMessage.warning('请输入充值点数')
    return
  }
  post('/代理账号充值', { id: 代理充值框.id, amount: 代理充值框.amount, note: 代理充值框.note })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '充值失败')
      ElMessage.success('充值成功')
      代理充值框.显示 = false
      查询代理()
    })
    .catch(显示错误)
}
const 删除代理 = function (row) {
  ElMessageBox.confirm(`确定删除渠道合伙人“${row.name}”？已生成的点卡和流水仅保留最近30天。`, '确认删除', {
    type: 'warning'
  })
    .then(() => post('/删除代理账号', { id: row.id }))
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '删除失败')
      ElMessage.success('删除成功')
      查询代理()
    })
    .catch((error) => {
      if (error !== 'cancel' && error !== 'close') 显示错误(error)
    })
}

onMounted(() => {
  Promise.all([查询软件(), 查询代理()]).catch(() => {})
})
</script>

<style scoped>
.页面 {
  padding: 16px;
  color: #e6eaf2;
}
.标题行,
.子标题行,
.价格标题 {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
h2,
h3 {
  margin: 0 0 6px;
}
.说明 {
  margin: 0 0 12px;
  color: #aeb6c3;
  font-size: 13px;
}
.代理区 {
  margin-top: 24px;
}
.代理提示 {
  margin-top: 20px;
  color: #aeb6c3;
}
.价格标题 {
  margin-bottom: 12px;
}
.价格说明 {
  margin-left: 10px;
  color: #9099a8;
  font-size: 12px;
}
.字段说明 {
  margin-left: 10px;
  color: #9099a8;
  font-size: 12px;
}
</style>
