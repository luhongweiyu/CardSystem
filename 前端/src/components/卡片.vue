<template>
  <el-card class="box-card">
    <div>
      <div>
        {{ 标题 }}
        <el-popover placement="top-start" title="Title" :width="200" trigger="hover" :content="帮助">
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
});
</script>

<style scoped>
.text {
  font-size: 14px;
}

.item {
  padding: 18px 0;
}

.box-card {
  width: 480px;
  margin: 10px;
  background-color: #545c64;
  text-color: #fff;
  color: #fff;
}
</style>
