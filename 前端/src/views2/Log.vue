<template>
  <div>
    <h1 @click="查询()">Log日志</h1>

    <pre>
      {{ 日志内容 }}

    </pre>
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


const post = useCounterStore().post;
const 查询 = () => {
  setTimeout(
    () => {
      post("/query_log", {}).then(function (res) { 日志内容.value = res.data })
    },
    5 * 1000)
};
</script>
<style scoped></style>
