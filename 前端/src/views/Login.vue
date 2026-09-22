<template>
  <div class="登录页">
    <el-card class="登录卡片" v-loading="加载中">
      <div class="标题区">
        <img src="/favicon.png" alt="卡密系统" class="标志" />
        <h2>卡密管理系统</h2>
        <p>{{ 注册界面 ? '创建管理员账号' : '请选择账号类型登录' }}</p>
      </div>

      <el-form label-position="top" @submit.prevent="提交登录">
        <el-form-item label="账号">
          <el-input v-model="账号" name="username" maxlength="32" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            v-model="密码"
            type="password"
            name="password"
            show-password
            maxlength="72"
            :autocomplete="注册界面 ? 'new-password' : 'current-password'"
          />
        </el-form-item>
        <el-form-item v-if="注册界面" label="确认密码">
          <el-input v-model="确认密码" name="confirm-password" type="password" autocomplete="new-password" show-password maxlength="72" />
        </el-form-item>

        <div class="按钮行" v-if="!注册界面">
          <el-button type="primary" :class="{ 默认登录: !当前登录类型是代理 }" :plain="当前登录类型是代理" @click="登录(false)">管理员登录</el-button>
          <el-button type="success" :class="{ 默认登录: 当前登录类型是代理 }" :plain="!当前登录类型是代理" @click="登录(true)">小伙伴登录</el-button>
        </div>
        <div class="按钮行" v-else>
          <el-button type="primary" native-type="submit">确认注册</el-button>
          <el-button @click="切换注册">返回登录</el-button>
        </div>
        <el-button v-if="!注册界面" link class="注册链接" @click="切换注册">注册管理员账号</el-button>
        <button v-if="!注册界面" type="submit" hidden>登录</button>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import Cookies from 'js-cookie'
import { ElMessage } from 'element-plus'
import apiClient, { 获取接口错误提示 } from '../api/请求客户端.js'
import { use登录状态Store } from '../stores/登录状态.js'

const router = useRouter()
const stores = use登录状态Store()
const { 账号, 密码, 登录状态, token, 用户id, api次数, 是代理账号, 账号信息 } = storeToRefs(stores)
const 加载中 = ref(false)
const 注册界面 = ref(false)
const 确认密码 = ref('')
// 只记住成功登录的身份，输错密码或误点另一按钮不会改变默认身份。
const 当前登录类型是代理 = ref(Cookies.get('login_role') === 'agent')

const 清空账号信息 = function () {
  stores.清理登录状态()
}

const 切换注册 = function () {
  注册界面.value = !注册界面.value
  确认密码.value = ''
}

const 保存登录结果 = function (data, agent) {
  stores.清理公共缓存()
  账号.value = data.name || 账号.value
  token.value = data.token || ''
  用户id.value = data.id || ''
  api次数.value = data.api || 0
  是代理账号.value = agent
  账号信息.value = { ...data }
  stores.登录到期时间 = data.expires_at || ''
  登录状态.value = true
  Cookies.set('name', 账号.value, { expires: 61, sameSite: 'Lax' })
  Cookies.set('login_role', agent ? 'agent' : 'admin', { expires: 61, sameSite: 'Lax' })
  当前登录类型是代理.value = agent
  Cookies.remove('password')
  密码.value = ''
  确认密码.value = ''
  // 代理账号没有管理员总览页，登录后直接进入它最常用的点卡页面。
  router.replace(agent ? '/card' : '/index')
}

const 登录 = function (agent) {
  if (加载中.value) return
  账号.value = (账号.value || '').trim()
  if (!账号.value || !密码.value) {
    ElMessage.error('请输入账号和密码')
    return
  }
  加载中.value = true
  const prefix = agent ? '/agent' : '/admin'
  apiClient
    .post(prefix + '/user_login', { name: 账号.value, password: 密码.value })
    .then((response) => {
      if (!response.data?.state) {
        清空账号信息()
        ElMessage.error(response.data?.msg || '登录失败')
        return
      }
      保存登录结果(response.data, agent)
      ElMessage.success('登录成功')
    })
    .catch((error) => {
      清空账号信息()
      ElMessage.error(获取接口错误提示(error))
    })
    .finally(() => {
      加载中.value = false
    })
}

const 提交登录 = function () {
  if (注册界面.value) 提交注册()
  else 登录(当前登录类型是代理.value)
}

const 提交注册 = function () {
  if (加载中.value) return
  账号.value = (账号.value || '').trim()
  if (密码.value !== 确认密码.value) {
    ElMessage.error('两次输入的密码不一致')
    return
  }
  if (!/^[A-Za-z0-9_]{3,32}$/.test(账号.value || '')) {
    ElMessage.error('用户名只能使用3至32位字母、数字或下划线')
    return
  }
  const bytes = new TextEncoder().encode(密码.value || '').length
  if (bytes < 6 || bytes > 72) {
    ElMessage.error('密码长度必须为6至72个字节')
    return
  }
  加载中.value = true
  apiClient
    .post('/user_register', { name: 账号.value, password: 密码.value })
    .then((response) => {
      if (!response.data?.state) {
        ElMessage.error(response.data?.msg || '注册失败')
        return
      }
      ElMessage.success('注册成功，请登录')
      注册界面.value = false
      确认密码.value = ''
    })
    .catch((error) => ElMessage.error(获取接口错误提示(error)))
    .finally(() => {
      加载中.value = false
    })
}

账号.value = Cookies.get('name') || ''
Cookies.remove('password')
密码.value = ''
</script>

<style scoped>
.登录页 {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  box-sizing: border-box;
}
.登录卡片 {
  width: min(420px, 100%);
  background: rgba(30, 35, 45, 0.94);
}
.标题区 {
  text-align: center;
  margin-bottom: 18px;
}
.标志 {
  width: 88px;
  height: 88px;
  object-fit: contain;
}
.标题区 h2 {
  margin: 8px 0 4px;
  color: #fff;
}
.标题区 p {
  margin: 0;
  color: #aeb6c3;
}
.按钮行 {
  display: flex;
  gap: 10px;
  align-items: center;
}
.按钮行 .el-button {
  flex: 1;
  margin-left: 0;
  min-width: 0;
}
.按钮行 .默认登录 {
  flex: 1.5;
  height: 46px;
  font-size: 16px;
  font-weight: 600;
}
.注册链接 {
  width: 100%;
  margin-top: 12px;
}
</style>
