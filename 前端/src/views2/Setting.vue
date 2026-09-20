<template>
  <section class="页面" v-loading="加载中">
    <div class="标题行">
      <div>
        <h2>账号设置</h2>
        <p class="说明">管理联系方式、公告和接口安全选项。</p>
      </div>
      <el-button :loading="加载中" @click="获取设置">刷新设置</el-button>
    </div>

    <el-row :gutter="16">
      <el-col :xs="24" :sm="12" :lg="8">
        <InfoCard 标题="联系方式" 帮助="用户可以通过这里填写的内容联系您；不需要时可以留空。">
          <el-input
            v-model="设置.contact_information"
            maxlength="500"
            show-word-limit
            placeholder="例如：QQ、邮箱或客服地址"
          />
          <el-button
            class="保存按钮"
            type="primary"
            @click="上传设置('contact_information', 设置.contact_information)"
          >
            保存联系方式
          </el-button>
        </InfoCard>
      </el-col>

      <el-col :xs="24" :sm="12" :lg="8">
        <InfoCard 标题="开发者公告" 帮助="公告可通过卡密接口读取，适合发布版本提示或维护通知。">
          <el-input
            v-model="设置.notice"
            type="textarea"
            :rows="4"
            maxlength="5000"
            show-word-limit
            placeholder="可选，留空表示不显示公告"
          />
          <el-button class="保存按钮" type="primary" @click="上传设置('notice', 设置.notice)">保存公告</el-button>
        </InfoCard>
      </el-col>

      <el-col :xs="24" :sm="12" :lg="8">
        <InfoCard
          标题="接口安全密码"
          帮助="开启接口安全模式时，客户端需要使用此密码参与签名。密码只用于接口校验，不会在页面中回显。"
        >
          <el-input
            v-model="设置.api_password"
            type="password"
            show-password
            maxlength="256"
            :placeholder="设置.api_password_set ? '已设置，输入新密码可替换' : '请输入安全密码'"
          />
          <div class="按钮组">
            <el-button type="primary" @click="保存安全密码">保存安全密码</el-button>
            <el-button v-if="设置.api_password_set && !设置.api_safe" type="danger" link @click="清除安全密码">
              清除
            </el-button>
          </div>
        </InfoCard>
      </el-col>

      <el-col :xs="24" :sm="12" :lg="8">
        <InfoCard 标题="接口安全模式" 帮助="开启后，客户端请求需要通过签名校验；接入方式请查看“接入帮助”。">
          <el-radio-group v-model="设置.api_safe">
            <el-radio :label="0">关闭</el-radio>
            <el-radio :label="1">开启</el-radio>
          </el-radio-group>
          <el-button class="保存按钮" type="primary" @click="上传设置('api_safe', 设置.api_safe)">
            保存安全模式
          </el-button>
        </InfoCard>
      </el-col>

      <el-col :xs="24" :sm="12" :lg="8">
        <InfoCard 标题="修改登录密码" 帮助="修改成功后旧登录令牌立即失效，当前页面会自动换成新令牌。">
          <el-form label-position="top" @submit.prevent>
            <el-form-item label="当前密码">
              <el-input
                v-model="密码表单.current"
                type="password"
                show-password
                maxlength="72"
                autocomplete="current-password"
              />
            </el-form-item>
            <el-form-item label="新密码">
              <el-input
                v-model="密码表单.next"
                type="password"
                show-password
                maxlength="72"
                autocomplete="new-password"
              />
            </el-form-item>
            <el-form-item label="确认新密码">
              <el-input
                v-model="密码表单.confirm"
                type="password"
                show-password
                maxlength="72"
                autocomplete="new-password"
              />
            </el-form-item>
            <el-button type="primary" @click="修改登录密码">修改密码</el-button>
          </el-form>
        </InfoCard>
      </el-col>
    </el-row>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import InfoCard from '../components/卡片.vue'
import { use登录状态Store } from '../stores/登录状态.js'
import { 获取接口错误提示 } from '../api/请求客户端.js'

const stores = use登录状态Store()
const post = stores.post
const 加载中 = ref(false)
const 设置 = reactive({
  contact_information: '',
  notice: '',
  api_password: '',
  api_password_set: false,
  api_safe: 0
})
const 密码表单 = reactive({ current: '', next: '', confirm: '' })

// 从服务端读取当前管理员设置，并把布尔值转成单选框使用的 0/1。
const 获取设置 = async function () {
  加载中.value = true
  try {
    const response = await post('/user_get_info', {})
    if (!response.data?.state) throw new Error(response.data?.msg || '读取设置失败')
    const data = response.data.data || {}
    Object.assign(设置, {
      contact_information: data.contact_information || '',
      notice: data.notice || '',
      // 管理端需要找回安全码以兼容旧客户端，因此回填当前 API 安全码。
      api_password: data.api_password || '',
      api_password_set: Boolean(data.api_password_set),
      api_safe: Number(data.api_safe) ? 1 : 0
    })
  } catch (error) {
    ElMessage.error(获取接口错误提示(error))
  } finally {
    加载中.value = false
  }
}

/**
 * 保存单个设置项，保存成功后重新读取一次，确保页面状态与服务端一致。
 */
const 上传设置 = async function (type, value) {
  if (typeof value !== 'string' && type !== 'api_safe') {
    ElMessage.warning('设置内容格式不正确')
    return
  }
  加载中.value = true
  try {
    const response = await post('/user_update_info', { type, value })
    if (!response.data?.state) throw new Error(response.data?.msg || '保存设置失败')
    ElMessage.success('保存成功')
    await 获取设置()
  } catch (error) {
    ElMessage.error(获取接口错误提示(error))
  } finally {
    加载中.value = false
  }
}

const 保存安全密码 = function () {
  if (!设置.api_password) {
    ElMessage.warning(设置.api_password_set ? '请输入新密码；如需清除请使用“清除”按钮' : '请输入安全密码')
    return
  }
  if (new TextEncoder().encode(设置.api_password).length > 256) {
    ElMessage.warning('接口安全密码不能超过256个字节')
    return
  }
  上传设置('api_password', 设置.api_password)
}

const 清除安全密码 = function () {
  ElMessageBox.confirm('清除后，开启接口安全模式前必须重新设置密码。确定清除？', '确认清除', { type: 'warning' })
    .then(() => 上传设置('api_password', ''))
    .catch(() => {})
}

/**
 * 修改管理员登录密码。前后端都按 UTF-8 字节数限制 6 至 72 字节，避免
 * 中文密码在浏览器字符数与 bcrypt 实际接收字节数之间产生差异。
 */
const 修改登录密码 = async function () {
  if (!密码表单.current || !密码表单.next) {
    ElMessage.warning('请填写当前密码和新密码')
    return
  }
  if (密码表单.next !== 密码表单.confirm) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  const bytes = new TextEncoder().encode(密码表单.next).length
  if (bytes < 6 || bytes > 72) {
    ElMessage.warning('新密码长度必须为6至72个字节')
    return
  }
  加载中.value = true
  try {
    const response = await post('/user_change_password', {
      current_password: 密码表单.current,
      new_password: 密码表单.next
    })
    if (!response.data?.state) throw new Error(response.data?.msg || '修改密码失败')
    // 服务端会撤销该账号的所有旧令牌并返回一个新令牌，当前页面必须
    // 立即替换，否则下一次管理请求会被判定为未登录。
    stores.token = response.data.token || ''
    Object.assign(密码表单, { current: '', next: '', confirm: '' })
    ElMessage.success('登录密码修改成功')
  } catch (error) {
    ElMessage.error(获取接口错误提示(error))
  } finally {
    加载中.value = false
  }
}

onMounted(获取设置)
</script>

<style scoped>
.页面 {
  padding: 16px;
  color: #e6eaf2;
}

.标题行 {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

h2 {
  margin: 0 0 6px;
}

.说明 {
  margin: 0;
  color: #aeb6c3;
  font-size: 13px;
}

.保存按钮 {
  display: block;
  margin-top: 12px;
}

.按钮组 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
}

@media (max-width: 600px) {
  .页面 {
    padding: 10px;
  }

  .标题行 {
    align-items: flex-start;
  }
}
</style>
