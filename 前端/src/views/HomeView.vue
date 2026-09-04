<template>
  <section class="首页">
    <el-card shadow="never" class="欢迎卡片">
      <h2>欢迎使用点卡系统</h2>
      <p>当前系统统一采用点卡余额计费：每张卡可以同时在多台设备使用，同一设备在授权周期内重复登录不会重复扣点。</p>
      <div class="快捷入口">
        <el-button type="primary" @click="$router.push('/card')">管理点卡</el-button>
        <el-button @click="$router.push('/software')">配置软件与周期</el-button>
        <el-button @click="$router.push('/point-ledger')">查看点数流水</el-button>
      </div>
    </el-card>
  </section>
</template>

<script setup>
// 首页只提供说明和常用入口，不额外读取业务数据，避免打开后台时产生无意义请求。
</script>

<style scoped>
.首页 {
  padding: 24px;
}
.欢迎卡片 {
  max-width: 820px;
}
h2 {
  margin-top: 0;
}
p {
  color: #aeb6c3;
  line-height: 1.8;
}
.快捷入口 {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 20px;
}
</style>
