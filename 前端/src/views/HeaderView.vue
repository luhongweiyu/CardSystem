<template>
  <div style="width:100%;height: 40px;">
    <div style="display: inline-block;">
      <h1 v-if="导航开关" @click="开关导航()">
        <el-icon>
          <DArrowRight />
        </el-icon>
        <!-- <el-icon><DArrowRight /></el-icon> -->
      </h1>
      <h1 v-else="导航开关" @click="开关导航()">
        <el-icon>
          <DArrowLeft />
        </el-icon>
        <!-- <el-icon><DArrowLeft /></el-icon> -->
      </h1>
      <!-- <el-switch v-model="导航开关" /> -->
      <!-- <el-form-item label="导航栏"> </el-form-item> -->
    </div>
    <span style="">
    </span>
    用户卡密查询或充值链接: <el-link type="success" :href="用户卡密查询链接" target="_blank"> {{ 用户卡密查询链接 }}</el-link>
    <span style="float: right;">
      欢迎用户:{{ 账号 }}( id:{{ 用户id }}),
      <span v-if="!账号信息.id2">
        api请求次数:{{ api次数 }}
      </span>
      <span v-if="账号信息.id2">
        余额:{{ 账号信息.余额 }}
        <el-link type="success" @click="查询价格()">价格</el-link>
      </span>
      <el-button style="border: 10px; margin: 10px" type="" @click="退出登录()">退出登录</el-button>
    </span>
  </div>
</template>

<script lang="ts" setup>
import { useCounterStore } from "../stores/counter.js";
import { ElMessageBox } from 'element-plus'
import Cookies from "js-cookie";
const stores = useCounterStore();
import { reactive, ref } from "vue";
import { storeToRefs } from "pinia";
const { 导航开关, 用户id, 账号, 登录状态, api次数, 是子账号 } = storeToRefs(stores);
const 账号信息 = useCounterStore().账号信息
const 用户卡密查询链接 = ref("http://" + window.location.hostname + ((window.location.port && (":" + window.location.port)) || "") + "/visitor/index.html?center_id=" + (账号信息.id2 || 账号信息.id))

const 开关导航 = function () {
  导航开关.value = !导航开关.value;
};
const 退出登录 = function () {
  // Cookies.remove('name')
  Cookies.remove('password')
  登录状态.value = false
}
const post = useCounterStore().post;
const 查询软件列表 = function () {
  post("/user_query_soft_list", {}).then(function (res) {
    if (res.data.state) {
      let list = res.data.data
      for (let i = list.length - 1; i >= 0; i--) {
        let 价格 = 账号信息.价格[list[i].ID]
        if (Object.keys(价格).length === 0) {
          list.splice(i, 1);
        } else {
          list[i].价格 = 价格
        }
      }
      list.push({ ID: 0, Software: "全部软件" });
      账号信息.软件列表 = list;
    }
  });
};
const 查询价格 = function () {
  let s: any[] = []
  for (let i = 0; i < 账号信息.软件列表.length; i++) {
    let 价格 = 账号信息.软件列表[i].价格
    if (价格) {
      let 价格s: any[] = []
      for (let key in 价格) {
        价格s.push(`${价格[key]}点/${key}天`);
      }
      s.push(账号信息.软件列表[i].Software + ":<br>" + 价格s.join(","))
    }
  }
  ElMessageBox.alert(s.join('<br/>'),
    {
      dangerouslyUseHTMLString: true,
    }
  )
}
if (是子账号.value == true) {
  查询软件列表()
}
</script>
<style></style>
