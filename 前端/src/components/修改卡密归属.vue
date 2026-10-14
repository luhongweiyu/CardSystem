<template>
  <el-button size="small" :disabled="!cards.length" @click="打开">修改归属</el-button>
  <el-dialog v-model="显示" title="修改归属" width="min(480px, 94vw)"
    :close-on-click-modal="!提交中" :close-on-press-escape="!提交中" :show-close="!提交中" append-to-body>
    <p>将修改已选的 {{ 待修改.length }} 张{{ kind === 'point_card' ? '点卡' : '时长卡' }}。</p>
    <el-form label-width="80px" :disabled="提交中 || !!结果">
      <el-form-item label="归属账号">
        <el-select v-model="目标代理" filterable placeholder="请选择归属账号" style="width: 100%">
          <el-option label="管理员名下" :value="0" />
          <el-option v-for="agent in stores.代理列表" :key="agent.id"
            :label="`${agent.name}（ID:${agent.id}）`" :value="agent.id" />
        </el-select>
      </el-form-item>
    </el-form>
    <p>余额、时长和在线授权保持不变。</p>
    <p v-if="kind === 'point_card'">归属变更后，账户代扣将跟随新代理的设置。</p>
    <template v-if="结果">
      <el-alert :title="结果.msg" :type="结果.failed.length ? 'warning' : 'success'" :closable="false" />
      <el-collapse>
        <el-collapse-item v-if="结果.failed.length" :title="`失败卡密（${结果.failed.length}）`" name="failed">
          <div class="卡密清单">{{ 结果.failed.join('\n') }}</div>
        </el-collapse-item>
        <el-collapse-item v-if="结果.success.length" :title="`成功卡密（${结果.success.length}）`" name="success">
          <div class="卡密清单">{{ 结果.success.join('\n') }}</div>
        </el-collapse-item>
      </el-collapse>
    </template>
    <template #footer>
      <el-button :disabled="提交中" @click="显示 = false">{{ 结果 ? '关闭' : '取消' }}</el-button>
      <el-button v-if="!结果" type="primary" :loading="提交中" :disabled="目标代理 === null" @click="提交">确认修改</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { use登录状态Store } from '../stores/登录状态.js'
import { 获取接口错误提示 } from '../api/请求客户端.js'

const props = defineProps({
  cards: { type: Array, required: true },
  kind: { type: String, required: true }
})
const emit = defineEmits(['changed'])
const stores = use登录状态Store()
const 显示 = ref(false)
const 提交中 = ref(false)
const 待修改 = ref([])
const 目标代理 = ref(null)
const 结果 = ref(null)

const 打开 = async () => {
  待修改.value = [...props.cards]
  目标代理.value = null
  结果.value = null
  try {
    await stores.查询代理列表(true)
    显示.value = true
  } catch (error) {
    ElMessage.error(获取接口错误提示(error, '加载代理列表失败'))
  }
}

const 提交 = async () => {
  if (提交中.value || 目标代理.value === null || !待修改.value.length) return
  提交中.value = true
  try {
    const res = await stores.post(`/${props.kind}/owner`, { cards: 待修改.value, agent_id: 目标代理.value })
    if (!res.data?.state) throw new Error(res.data?.msg || '修改归属失败')
    结果.value = res.data
    emit('changed')
  } catch (error) {
    ElMessage.error(获取接口错误提示(error, '修改归属失败'))
  } finally {
    提交中.value = false
  }
}
</script>

<style scoped>
.卡密清单 { max-height: 200px; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; }
</style>
