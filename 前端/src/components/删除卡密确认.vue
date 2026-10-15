<template>
  <el-dialog
    v-model="显示"
    :title="`确认删除${卡片名称}`"
    width="980px"
    :show-close="!加载中"
    :close-on-click-modal="!加载中"
    :close-on-press-escape="!加载中"
    destroy-on-close
  >
    <p class="删除提示">即将删除 {{ rows.length }} 张{{ 卡片名称 }}。请核对下方卡片，删除后无法恢复。</p>
    <p v-if="说明" class="删除说明">{{ 说明 }}</p>
    <el-table :data="rows" border stripe max-height="360" row-key="card">
      <el-table-column
        v-for="column in columns"
        :key="column.label"
        :label="column.label"
        :width="column.width"
        :min-width="column.minWidth"
        show-overflow-tooltip
      >
        <template #default="{ row }">{{ column.text(row) }}</template>
      </el-table-column>
    </el-table>
    <div class="数量确认">
      <label for="删除确认数量">删除张数</label>
      <el-input id="删除确认数量" v-model="输入数量" :disabled="加载中" inputmode="numeric" autocomplete="off" :placeholder="`输入 ${rows.length}`" />
      <label for="删除随机确认">随机验证码：{{ 随机尾数 }}</label>
      <el-input id="删除随机确认" v-model="输入验证码" :disabled="加载中" inputmode="numeric" autocomplete="off" maxlength="3" placeholder="输入三位验证码" />
    </div>
    <template #footer>
      <el-button :disabled="加载中" @click="显示 = false">取消</el-button>
      <el-button type="danger" :loading="加载中" :disabled="!可提交" @click="确认">确认删除</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, ref, watch } from 'vue'

const props = defineProps({
  modelValue: Boolean,
  rows: { type: Array, required: true },
  columns: { type: Array, required: true },
  卡片名称: { type: String, required: true },
  说明: { type: String, default: '' },
  加载中: Boolean
})
const emit = defineEmits(['update:modelValue', 'confirm'])
const 显示 = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})
const 输入数量 = ref('')
const 输入验证码 = ref('')
const 随机尾数 = ref('')
const 可提交 = computed(() => !props.加载中 && props.rows.length > 0 && 输入数量.value === String(props.rows.length) && 随机尾数.value !== '' && 输入验证码.value === 随机尾数.value)
const 确认 = () => { if (可提交.value) emit('confirm') }
watch(() => props.modelValue, (open) => {
  if (open) {
    输入数量.value = ''
    输入验证码.value = ''
    随机尾数.value = String(Math.floor(Math.random() * 900) + 100)
  }
}, { immediate: true })
</script>

<style scoped>
.删除提示 { margin: 0 0 8px; font-weight: 600; }
.删除说明 { margin: 0 0 12px; color: #aeb6c3; font-size: 13px; }
.数量确认 { display: flex; align-items: center; flex-wrap: wrap; gap: 8px 12px; margin-top: 16px; }
.数量确认 .el-input { width: 160px; }
</style>
