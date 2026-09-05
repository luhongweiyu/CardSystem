<template>
  <section class="页面">
    <h2>接入帮助</h2>
    <p class="说明">客户端按“登录 → 按心跳间隔发送心跳 → 退出”的流程使用点卡。</p>
    <el-card shadow="never" class="说明卡片">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="登录">
          POST /card/card_login，必须提交 card、software、device_id；可选 period_seconds 和 device_alias。
        </el-descriptions-item>
        <el-descriptions-item label="心跳">
          POST /card/card_ping，提交登录返回的 needle；建议按软件设置的心跳间隔发送。
        </el-descriptions-item>
        <el-descriptions-item label="退出">
          POST /card/card_logout，提交 software 与 device_id；可同时提交 needle
          做附加校验，退出后立即释放设备会话。
        </el-descriptions-item>
        <el-descriptions-item label="点卡计费方案">
          GET/POST /card/period_prices，可读取当前软件启用的授权时长和扣点数。
        </el-descriptions-item>
        <el-descriptions-item label="流水查询">
          GET/POST /card/point_ledger/query，只能查询当前卡密自己的点数流水。
        </el-descriptions-item>
      </el-descriptions>
      <p class="提示">
        device_id 应由客户端生成并持久化；device_alias 只是展示名称，不参与设备唯一性判断。系统支持
        HTTP，接口安全模式可额外校验签名，但签名不提供传输加密。
      </p>
    </el-card>
  </section>
</template>

<script setup>
// 帮助页直接展示当前实现的接口，不依赖外部文档站点，避免链接失效。
</script>

<style scoped>
.页面 {
  padding: 16px;
  color: #e6eaf2;
}
h2 {
  margin-top: 0;
}
.说明 {
  color: #aeb6c3;
}
.说明卡片 {
  max-width: 980px;
}
.提示 {
  margin: 18px 0 0;
  color: #aeb6c3;
  line-height: 1.8;
}
</style>
