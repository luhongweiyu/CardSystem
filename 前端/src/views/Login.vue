<template>
  <div v-loading="loading" class="juzhong">
    <div>      
      <img src="/favicon.png" style=" width: 200px;">
      <div>
        <el-input v-model="账号" placeholder="请输入账号">
          <template #prepend>账号</template>
        </el-input>
      </div>
      <div class="container">
        <el-input v-model="密码" type="password" placeholder="请输入密码" show-password>
          <template #prepend>密码</template>
        </el-input>
      </div>
      <div class="container" v-if="注册界面">
        <el-input v-model="再次确认密码" type="password" placeholder="再次输入密码" show-password>
          <template #prepend>密码</template>
        </el-input>
      </div>
    </div>
    <div>
      <el-row :span="12">
        <el-col :span="6"> <el-button @click="登录()">登录</el-button></el-col>
        <el-col :span="6"><el-button @click="注册()">注册</el-button></el-col>
      </el-row>
    </div>
  </div>
</template>

<script lang="ts" setup>
// import axios from "axios";
import { ElMessage } from "element-plus";
import { ref, reactive, computed } from "vue";

import { useCounterStore } from "../stores/counter.js";

import { storeToRefs } from "pinia";
import axios from "axios";
import Cookies from "js-cookie";
import { tr } from "element-plus/es/locale";
const stores = useCounterStore();
const { 账号, 密码, 登录状态, 用户id,api次数 } = storeToRefs(stores);

const loading = ref(false);
const 注册界面 = ref(false)
const 再次确认密码 = ref("")

// const 密码 = ref(stores.密码);
// const 登录状态 = ref(stores.登录状态);
const 登录 = function () {
  if (注册界面.value) {
    注册界面.value = false
    return
  }
  loading.value = true;
  console.log(window.location)
  axios
    .post(
      "http://" + window.location.hostname + ":802/admin/user_login",
      {
        name: 账号.value,
        password: 密码.value
      },
      {
        headers: {
          "Content-Type": "application/json"
        }
      }
    )
    .then(function (response) {
      console.log(response.data);
      if (response.data.state) {
        ElMessage.success("登录成功");
        登录状态.value = true;
        loading.value = false;
        用户id.value = response.data.id
        api次数.value = response.data.api
        Cookies.set('name', 账号.value, { expires: 61 })
        Cookies.set('password', 密码.value, { expires: 61 })
      } else {
        ElMessage.error(response.data.msg);
        登录状态.value = false;
        loading.value = false;
      }
    })
    .catch(function (error) {
      console.log(error);
    });
};
const 注册 = function () {
  if (!注册界面.value) {
    注册界面.value = true
    return
  }
  if (再次确认密码.value != 密码.value) {
    ElMessage.error("两次输入的密码不一样")
    return
  }
  loading.value = true;
  axios
    .post(
      "http://" + window.location.hostname + ":802/admin/user_register",
      {
        name: 账号.value,
        password: 密码.value
      },
      {
        headers: {
          "Content-Type": "application/json"
        }
      }
    )
    .then(function (response) {
      console.log(response.data);
      if (response.data.state) {
        ElMessage.success("注册成功");
        注册界面.value = false
        登录();
      } else {
        ElMessage.error(response.data.msg);
        loading.value = false;
      }
    })
    .catch(function (error) {
      console.log(error);
    });
};

账号.value = Cookies.get('name', 账号.value)
密码.value = Cookies.get('password', 密码.value)

</script>
<style scoped>
.juzhong {
  align: "center";
  position: absolute;
  top: 30%;
  /* bottom: 0; */
  left: 0;
  right: 0;
  margin: auto;
  width: 300px;
}
</style>
