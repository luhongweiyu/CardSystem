<template>
  <el-card class="box-card">
    <div>
      <div class="卡片标题">
        {{ 标题 }}
        <el-popover placement="top-start" title="说明" :width="260" trigger="hover" :content="帮助">
          <template #reference>
            <el-icon><QuestionFilled /></el-icon>
          </template>
        </el-popover>
      </div>
      <slot></slot>
      <!-- 插槽出口 -->
    </div>
  </el-card>
</template>
<script setup>
defineProps({
  标题: {
    type: String,
    required: true
  },
  帮助: {
    type: String,
    required: true
  }
})
</script>

<style scoped>
.text {
  font-size: 14px;
}

.item {
  padding: 18px 0;
}

.box-card {
  width: 100%;
  margin: 0 0 16px;
  box-sizing: border-box;
  background-color: #545c64;
  color: #fff;
}

.卡片标题 {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 14px;
  font-weight: 600;
}
</style>
