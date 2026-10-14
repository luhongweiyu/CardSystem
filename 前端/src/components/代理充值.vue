<template>
  <el-button size="small" type="primary" plain @click="打开充值">充值</el-button>
  <el-dialog v-model="显示" class="充值弹窗" title="账户充值" width="min(640px, 92vw)" append-to-body
    :close-on-click-modal="!提交中" :close-on-press-escape="!提交中" :show-close="!提交中">
    <div v-loading="加载中" class="充值内容">
      <template v-if="已开放">
        <div class="充值主体">
          <el-image class="收款码" :src="收款码" fit="contain">
            <template #error><span>收款码加载失败，请联系管理员</span></template>
          </el-image>
          <div class="充值表单">
            <div class="充值表单内容">
              <p class="充值说明">扫码付款后，填写实际金额和支付宝订单号后 6 位。<br>最低充值 {{ 起充金额 }} 元。</p>
              <el-form label-position="top" @submit.prevent="确认充值">
                <el-form-item label="付款金额（元）">
                  <el-input v-model="金额" class="充值输入框" inputmode="decimal" maxlength="20" placeholder="例如：10.00" :disabled="提交中" />
                </el-form-item>
                <el-form-item label="支付宝订单号后 6 位">
                  <el-input v-model="尾号" class="充值输入框" inputmode="numeric" maxlength="6" placeholder="请输入6位数字" :disabled="提交中" />
                </el-form-item>
              </el-form>
              <div class="充值提示区">
                <el-alert v-if="提示 || 当前输入提示" :title="提示 || 当前输入提示" type="warning" :closable="false" />
              </div>
            </div>
            <div class="充值操作区">
              <el-button :disabled="提交中" @click="显示 = false">关闭</el-button>
              <el-button type="primary" :loading="提交中" :disabled="!可确认充值" @click="确认充值">
                {{ 按钮文字 }}
              </el-button>
            </div>
          </div>
        </div>
      </template>
      <el-empty v-else-if="!加载中" :description="提示 || '自动充值暂未开放，请联系管理员'" :image-size="70" />
    </div>
    <template v-if="!已开放" #footer>
      <el-button :disabled="提交中" @click="显示 = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { use登录状态Store } from '../stores/登录状态.js'
import { 获取接口错误提示 } from '../api/请求客户端.js'

const stores = use登录状态Store()
const 显示 = ref(false)
const 加载中 = ref(false)
const 提交中 = ref(false)
const 已开放 = ref(false)
const 收款码 = ref('')
const 起充金额 = ref('0.01')
const 金额 = ref('')
const 尾号 = ref('')
const 提示 = ref('')
const 冷却秒数 = ref(0)
const 冷却截止时间 = ref(0)
let 冷却定时器 = 0

const 金额格式 = /^([0-9]{1,17})(?:\.([0-9]{1,2}))?$/
const 最大金额分 = 9223372036854775807n

// 使用整数分比较金额，避免大金额经过 Number 后丢失精度。
const 金额转分 = function (value) {
  const match = String(value || '').trim().match(金额格式)
  if (!match) return null
  try {
    return BigInt(match[1]) * 100n + BigInt((match[2] || '').padEnd(2, '0'))
  } catch {
    return null
  }
}

const 输入金额分 = computed(() => 金额转分(金额.value))
const 起充金额分 = computed(() => 金额转分(起充金额.value) || 1n)
const 输入有效 = computed(() =>
  输入金额分.value !== null && 输入金额分.value > 0n && 输入金额分.value <= 最大金额分 && 输入金额分.value >= 起充金额分.value && /^[0-9]{6}$/.test(尾号.value.trim())
)
const 当前输入提示 = computed(() => {
  if (金额.value.trim()) {
    if (输入金额分.value === null || 输入金额分.value <= 0n) return '请输入大于0的金额，最多两位小数。'
    if (输入金额分.value > 最大金额分) return '金额超出可充值范围。'
    if (输入金额分.value < 起充金额分.value) return `最低充值 ${起充金额.value} 元。`
  }
  if (尾号.value && !/^[0-9]{6}$/.test(尾号.value.trim())) return '订单号后6位需要填写6个数字。'
  return ''
})
const 可确认充值 = computed(() => 已开放.value && !加载中.value && !提交中.value && 冷却秒数.value === 0 && 输入有效.value)
const 按钮文字 = computed(() => 冷却秒数.value > 0 ? `${冷却秒数.value}秒后再试` : '确认充值')

const 清理冷却 = function () {
  if (冷却定时器) window.clearInterval(冷却定时器)
  冷却定时器 = 0
}

const 开始冷却 = function (seconds) {
  清理冷却()
  const value = Number(seconds)
  const duration = Math.max(30, Number.isFinite(value) ? Math.ceil(value) : 0)
  冷却截止时间.value = Date.now() + duration * 1000
  冷却秒数.value = duration
  const 更新冷却 = () => {
    const remaining = Math.ceil((冷却截止时间.value - Date.now()) / 1000)
    if (remaining <= 0) {
      冷却秒数.value = 0
      冷却截止时间.value = 0
      清理冷却()
      return
    }
    冷却秒数.value = remaining
  }
  冷却定时器 = window.setInterval(更新冷却, 500)
}

const 简化提示 = function (message) {
  const value = String(message || '')
  if (value.toLowerCase().includes('network error') || value.toLowerCase().includes('timeout')) return '连接服务器超时，请稍后重试。'
  return value || '充值失败，请稍后重试。'
}

watch([金额, 尾号], () => { 提示.value = '' })

async function 打开充值() {
  if (加载中.value || 提交中.value) return
  显示.value = true
  加载中.value = true
  已开放.value = false
  提示.value = ''
  try {
    const { data } = await stores.post('/recharge/info', {})
    if (!data?.state) throw new Error(data?.msg || '读取充值设置失败')
    已开放.value = data.enabled === true
    收款码.value = data.qr_code || ''
    起充金额.value = data.min_amount || '0.01'
  } catch (error) {
    提示.value = 简化提示(获取接口错误提示(error))
  } finally {
    加载中.value = false
  }
}

async function 确认充值() {
  if (!可确认充值.value) return
  const amount = 金额.value.trim()
  const suffix = 尾号.value.trim()
  提交中.value = true
  提示.value = ''
  try {
    // 核对可能等待支付宝响应；只延长本接口超时，不自动重试付款认领。
    const { data } = await stores.post('/recharge/verify', { amount, suffix }, 25000)
    if (!data?.state) {
      提示.value = 简化提示(data?.msg)
      // 未确认到账时至少等待30秒；后端返回更长时间则按后端时间等待。
      开始冷却(data?.cooldown)
      return
    }
    ElMessage.success(`充值成功，增加 ${data.credited} 余额`)
    金额.value = ''
    尾号.value = ''
    显示.value = false
  } catch {
    提示.value = '暂时无法查询付款，请稍后重试。'
    开始冷却()
  } finally {
    提交中.value = false
  }
}

onBeforeUnmount(清理冷却)
</script>

<style scoped>
.充值内容 { min-width: 0; }
.充值主体 { display: flex; gap: 20px; align-items: stretch; }
.收款码 { flex: 0 0 auto; display: flex; width: 400px; height: 400px; align-items: center; justify-content: center; background: #fff; }
.充值表单 { flex: 0 0 180px; min-width: 0; max-width: 180px; min-height: 400px; display: flex; flex-direction: column; justify-content: space-between; }
.充值输入框 { width: 125px; max-width: 100%; }
.充值操作区 { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
.充值说明 { min-height: 42px; margin: 0 0 10px; line-height: 1.6; }
.充值提示区 { height: 52px; max-height: 52px; overflow: auto; }
.充值提示区 :deep(.el-alert) { min-height: 42px; box-sizing: border-box; }
:global(.充值弹窗 .el-dialog__body) { max-height: min(58vh, 480px); overflow-y: auto; }
@media (max-width: 640px) {
  .充值主体 { display: block; }
  .收款码 { width: 220px; height: 220px; margin: 0 auto 12px; }
  .充值表单 { width: 100%; max-width: 320px; min-height: 0; margin: 0 auto; }
  .充值输入框 { width: 100%; }
}
</style>
