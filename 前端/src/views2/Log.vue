<template>
  <div>
    <h1>Log日志</h1>
    <div v-loading="加载中" :element-loading-text="加载提示" style="width: 100vw;">
      <el-button v-if="show" link type="primary" size="small" @click="查询()" style="display:inline">查询日志</el-button>
      <pre>
        {{ 日志内容 }}  
      </pre>

    </div>
  </div>
</template>

<script lang="ts" setup>


import { Check, Delete, Edit, Message, Search, Star } from "@element-plus/icons-vue";
import { reactive, ref } from "vue";
import { useCounterStore } from "../stores/counter";
import { ElMessage, ElMessageBox } from "element-plus";
import { tr } from "element-plus/es/locale";
import axios, { Axios } from "axios";


const 日志内容 = ref("日志");
const 加载中 = ref(true);
const show = ref(true);
const 加载提示 = ref("银河系统准备中...请稍候");

const post = useCounterStore().post;

const 查询 = () => {
  show.value = false
  加载中.value = true
  加载提示.value = '正在连接银河服务器为您查询日志...请稍候'
  日志内容.value = '查询中...请稍候...'
  setTimeout(
    () => {
      post("/query_log", {}).then(function (res) {
        加载中.value = false
        日志内容.value = res.data
      })
    },
    5 * 1000)
};
const 延迟显示 = () => {
  setTimeout(() => { 加载中.value = false }, 10 * 1000)
}
延迟显示()
</script>
<style scoped></style>
