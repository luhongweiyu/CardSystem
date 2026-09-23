<template>
  <section class="页面" v-loading="加载中">
    <div class="标题行">
      <div>
        <h2>运行日志</h2>
        <p class="说明">这里只展示当前账号最近两个月的操作日志，不包含点数流水。</p>
      </div>
      <el-button type="primary" :loading="加载中" @click="查询">刷新日志</el-button>
    </div>
    <el-card shadow="never" class="日志卡片">
      <pre v-if="日志内容">{{ 日志内容 }}</pre>
      <el-empty v-else description="暂无日志" />
    </el-card>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { use登录状态Store } from '../stores/登录状态.js'
import { 获取接口错误提示 } from '../api/请求客户端.js'
import { 日志倒序 } from '../utils/日志工具.js'

const post = use登录状态Store().post
const 加载中 = ref(false)
const 日志内容 = ref('')

// 日志接口返回纯文本；异常时仍兼容后端的 JSON 错误结构。
const 查询 = async function () {
  加载中.value = true
  try {
    const response = await post('/query_log', {})
    if (typeof response.data === 'string') {
      日志内容.value = 日志倒序(response.data)
      return
    }
    if (!response.data?.state) {
      throw new Error(response.data?.msg || '查询日志失败')
    }
    日志内容.value = 日志倒序(response.data.data)
  } catch (error) {
    ElMessage.error(获取接口错误提示(error))
  } finally {
    加载中.value = false
  }
}

onMounted(查询)
</script>

<style scoped>
.页面 {
  padding: 16px;
  color: #e6eaf2;
}
.标题行 {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
h2 {
  margin: 0 0 6px;
}
.说明 {
  margin: 0;
  color: #aeb6c3;
  font-size: 13px;
}
.日志卡片 {
  min-height: 280px;
}
pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  color: #cbd3df;
  line-height: 1.6;
}
</style>
