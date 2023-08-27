<template>
  <div>
    <h1>setting设置</h1>
    <el-row>
      <el-col :span="8">
        <card 标题="联系方式" 帮助="如果您想让您的用户能够联系到您，请在这里填写有效联系方式。如果您使用自动售卡等服务，此项必填，否则有封号风险，不填或错填都会引起账号异常。没有使用自动售卡功能可以不填。">
          <el-input v-model="devinfo.contact_information" placeholder="联系方式" style="width: 150px" />
          <br />
          <el-button type="primary" @click="上传设置('contact_information', devinfo.contact_information)">确认修改</el-button>
        </card>
      </el-col>
      <el-col :span="8">
        <card 标题="开发者公告" 帮助="公告内容可以通过软件接口读取到，以便实时更新软件公告，没有用到软件公告功能请忽略。">
          <el-input v-model="devinfo.notice" placeholder="开发者公告" style="width: 150px" />
          <br />
          <el-button type="primary" @click="上传设置('notice', devinfo.notice)">确认修改</el-button>
        </card>
      </el-col>
      <el-col :span="8">
        <card 标题="接口安全密码" 帮助="如果您想让您的用户能够联系到您，请在这里填写有效联系方式。如果您使用自动售卡等服务，此项必填，否则有封号风险，不填或错填都会引起账号异常。没有使用自动售卡功能可以不填。">
          <el-input v-model="devinfo.api_password" placeholder="安全密码" style="width: 150px" />
          <br />
          <el-button type="primary" @click="上传设置('api_password', devinfo.api_password)">确认修改</el-button>
        </card>
      </el-col>
      <el-col :span="8">
        <card 标题="是否开启api安全模式" 帮助="打开此开关后，软件发送至服务器的所有请求，会开启安全验证，服务器会验证发送来的信息是否正确，防止破解。关闭此开关，会对您造成一定的安全隐患。具体请查看接入帮助。">
          <!-- <el-input v-model="devinfo.api_safe" placeholder="安全密码开关" style="width: 150px" /> -->



          <el-radio-group v-model="devinfo.api_safe"  style="width: 150px" >
            <el-radio :label="0" size="large">关闭</el-radio>
            <el-radio :label="1" size="large">开启</el-radio>
          </el-radio-group>






          <br />
          <el-button type="primary" @click="上传设置('api_safe', devinfo.api_safe)">确认修改</el-button>
        </card>
      </el-col>
      <el-col :span="8">
        <!-- <card 标题="自动售卡手续费支付方"
          帮助="如果选择买家，用户购卡时付款为卡密原价，购卡成功后，开发者和代理人收到的金额为扣除手续费（2.1%）后的金额。如果选择卖家，用户购卡时，付款价格会比原价贵3%，购卡成功后，开发者会收到卡密原价的金额，代理人仍扣除2.1%费用">
          <el-radio-group v-model="devinfo.手续费支付方"  style="width: 150px" >
            <el-radio :label="false" size="large">卖家</el-radio>
            <el-radio :label="true" size="large">买家</el-radio>
          </el-radio-group>
          <br />
          <el-button type="primary" @click="上传设置('手续费支付方', 手续费支付方)">确认修改</el-button>
        </card> -->
      </el-col>
    </el-row>
    <el-button type="primary" @click="获取设置()">下载设置</el-button>
  </div>
</template>

<script lang="ts" setup>
import axios from "axios";
import card from "../components/卡片.vue";
import { reactive, ref } from "vue";
import { useCounterStore } from "../stores/counter";
import { storeToRefs } from "pinia";
import { ElMessage } from "element-plus";
const post = useCounterStore().post;
const 手续费支付方 = ref();
const 支付宝结算方式 = ref();
const devinfo = reactive({});

const 获取设置 = function () {
  post("/user_get_info", {}).then(function (res) {
    console.log(res.data);
    console.log(devinfo);
    if (res.data.state) {
      for (var a in res.data.data) {
        console.log(a);
        devinfo[a] = res.data.data[a];
      }
      console.log(devinfo);
      ElMessage.success("刷新设置成功");
    } else {
      ElMessage.error(res.data.msg);
    }
  });
};
const 上传设置 = function (a, b) {
  if (!b && b != "") {
    console.log(a, "修改的值为空", b);
    return;
  }
  console.log(a, b);
  post("/user_update_info", { type: a, value: b })
    .then(function (response) {
      console.log(response.data);
      if (response.data.state) {
        ElMessage.success("修改成功");
      } else {
        ElMessage.error(response.data.msg);
      }
      获取设置();
    })
    .catch(function (error) {
      console.log(error);
    });
};
获取设置();
</script>
<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.text {
  font-size: 14px;
}

.item {
  margin-bottom: 18px;
}

.box-card {
  width: 350px;
}
</style>
