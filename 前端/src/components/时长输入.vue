<template>
  <div class="时长输入">
    <div class="时长分项" role="group" aria-label="时长（天、时、分）">
      <label v-for="item in 分项" :key="item.key" class="时长单位">
        <el-input-number
          :model-value="当前分项[item.key]"
          :min="0"
          :max="item.max"
          :precision="0"
          :controls="false"
          :disabled="disabled"
          :size="size"
          :aria-label="item.label"
          :class="{ 天数输入: item.key === 'days' }"
          @update:model-value="更新分项(item.key, $event)"
        />
        <span>{{ item.label }}</span>
      </label>
    </div>
    <div v-if="范围提示" class="范围提示" role="status">{{ 范围提示 }}</div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { 格式化时长, 永久时长分钟 } from '../utils/时长工具.js'

const props = defineProps({
  modelValue: { type: Number, default: 0 },
  min: { type: Number, default: 0 },
  max: { type: Number, required: true },
  allowZero: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
  size: { type: String, default: 'default' }
})
const emit = defineEmits(['update:modelValue'])
const 分项 = computed(() => [
  { key: 'days', label: '天', max: Math.floor(props.max / 1440) },
  { key: 'hours', label: '时', max: 23 },
  { key: 'minutes', label: '分', max: 59 }
])

// 直接从总分钟数回填，快捷选项、推荐时长和重新打开编辑框都使用同一份值。
const 当前分项 = computed(() => {
  const value = Number.isSafeInteger(props.modelValue) && props.modelValue >= 0 ? props.modelValue : 0
  return { days: Math.floor(value / 1440), hours: Math.floor(value % 1440 / 60), minutes: value % 60 }
})
const 更新分项 = (key, value) => {
  const parts = { ...当前分项.value, [key]: value ?? 0 }
  emit('update:modelValue', parts.days * 1440 + parts.hours * 60 + parts.minutes)
}
const 边界文本 = (value) => value === 0 ? '0分' : value === 永久时长分钟 ? '36500天' : 格式化时长(value)
const 范围提示 = computed(() => {
  const value = props.modelValue
  if (props.allowZero && value === 0) return ''
  if (Number.isSafeInteger(value) && value >= props.min && value <= props.max) return ''
  // 组合编辑时允许暂时越界，不自动改写其他分项；保存仍由页面按原业务限制校验。
  return `时长须在${边界文本(props.min)}至${边界文本(props.max)}之间${props.allowZero ? '，或设为0' : ''}`
})
</script>

<style scoped>
.时长输入 {
  display: inline-flex;
  flex-direction: column;
  max-width: 100%;
  vertical-align: middle;
}
.时长分项 {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}
.时长单位 {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}
.时长单位 .el-input-number {
  width: 50px;
}
.时长单位 .天数输入 {
  width: 78px;
}
.范围提示 {
  color: var(--el-color-danger);
  font-size: 12px;
  line-height: 1.5;
  white-space: normal;
  margin-top: 4px;
}
</style>
