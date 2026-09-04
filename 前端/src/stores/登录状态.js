import { ref } from 'vue'
import { defineStore } from 'pinia'
import apiClient from '../api/请求客户端.js'

export const use登录状态Store = defineStore('登录状态', () => {
  // 这些字段是整个管理端共享的最小登录状态。令牌只保存在内存中，
  // 页面刷新后重新登录，避免把管理密码或长期令牌写入本地存储。
  const 用户id = ref('')
  const 导航开关 = ref(false)
  const 账号 = ref('')
  const 密码 = ref('')
  const 登录状态 = ref(false)
  const api次数 = ref(0)
  const 是代理账号 = ref(false)
  const 账号信息 = ref({})
  const token = ref('')

  // 每次请求复制业务参数，避免向页面中的响应式表单对象写入账号或令牌字段。
  const post = function (链接, 参数) {
    const 请求参数 = { ...(参数 || {}) }
    if (!Object.prototype.hasOwnProperty.call(请求参数, 'name') && 账号.value) {
      请求参数.name = 账号.value
    }
    if (token.value) {
      请求参数.token = token.value
    }
    // 登录接口由登录页直接调用；其他接口只发送令牌，不重复发送明文密码。
    const 前缀 = 是代理账号.value ? '/agent' : '/admin'
    return apiClient.post(前缀 + 链接, 请求参数)
  }

  return { post, 登录状态, 账号, 密码, token, 用户id, 导航开关, api次数, 是代理账号, 账号信息 }
})
