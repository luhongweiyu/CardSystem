<template>
  <div>
    <div>
      <div v-loading="加载中">

        <el-input v-model="卡密" placeholder="卡密" style="width: auto" />
        <el-button type="success" :icon="Search" circle style="margin: 10px" @click="查询所有卡密()" />
        <el-input v-model="充值卡" placeholder="充值卡" style="width: auto" />
        <el-button type="success" :icon="Search" circle style="margin: 10px" @click="查询充值卡()" />

        <el-button style="border: 0px; margin: 0px" type="success" @click="续费按钮">续费 </el-button>
      </div>
    </div>

    <el-table :data="所有卡密" :cell-style="cellState" @selection-change="记录打钩的" border v-loading="加载中">
      <el-table-column type="selection" width="30" />
      <el-table-column :show-overflow-tooltip="true" prop="card" label="卡密" width="180px" />
      <el-table-column :show-overflow-tooltip="true" prop="address" label="状态" width="60px">
        <template #default="scope">
          <span v-if="scope.row.card_state == 4" style="color:red">冻结</span>
          <span v-else-if="判断到期(scope.row.end_time)" style="color:rgb(131, 71, 71)">到期</span>
          <span v-else-if="scope.row.end_time" style="color:rgb(9, 255, 0)">激活</span>
        </template>
      </el-table-column>

      <el-table-column :show-overflow-tooltip="true" prop="use_time" label="最近登录" width="160px">
        <template #default="scope"> {{ 时间转字符串(scope.row.use_time) }}</template>
      </el-table-column>
      <el-table-column :show-overflow-tooltip="true" prop="end_time" label="到期时间" width="160px">
        <template #default="scope"> {{ 时间转字符串(scope.row.end_time) }}</template>
      </el-table-column>
      <el-table-column :show-overflow-tooltip="true" prop="s" label="类型" width="50px" />
    </el-table>

    <el-dialog v-model="显示续费卡密界面" title="续费卡密" width="80%">
      <el-form label-position="right" label-width="100px" style="max-width: 460px" v-loading="加载中">
        <el-button style="border: 10px; margin: 10px" type="success" @click="确定续费卡密()">确定续费</el-button>
        <el-input v-model="充值卡" placeholder="充值卡" style="width: auto" />
        <el-button type="success" :icon="Search" circle style="margin: 10px" @click="查询充值卡()" />
        <el-form-item label="">
        </el-form-item>
        请核对需要续费的卡密:(共{{ 已勾的卡密.length }}个)
        <el-form-item label="卡密">
          <!--  v-if="新卡.指定类型 == 2" -->
          <el-input v-model="待续费卡密.续费卡密" :autosize="{ minRows: 3 }" type="textarea" placeholder="输入需要续费的卡密"
            style="width: 300px" :disabled="true" />
        </el-form-item>
      </el-form>
    </el-dialog>

  </div>
</template>
  
<script lang="ts" setup>
import { Check, Delete, Edit, Message, Search, Star } from "@element-plus/icons-vue";
import { reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { fa, tr } from "element-plus/es/locale";
import axios, { Axios } from "axios";



const post = function (链接, 参数) {
  // return axios.post("http://localhost:802/admin" + 链接, 参数, { headers: { "Content-Type": "application/json" } });
  return axios.post("http://" + window.location.hostname + ":802/visitor" + 链接 + window.location.search, 参数, { headers: { "Content-Type": "application/json" } });
};





const 状态列表 = reactive([
  [0, "全部状态"],
  [1, "未激活"],
  [2, "已激活"],
  [3, "到期"],
  [4, "正常"],
  [5, "冻结"],
]);
const 所有卡密_当前页 = ref(1)
const 所有卡密数量 = ref(0)
const 每页卡密数量 = ref(20)
const soft = ref(0);
const state = ref(0);
const 卡密 = ref("");
const 充值卡 = ref("");
const 所有卡密 = ref([]);
const 显示续费卡密界面 = ref(false);
const 加载中 = ref(false);
const 已勾的卡密 = ref([] as any[]);

const 待续费卡密 = ref({
  续费时间: 0,
  续费卡密: "",
})
const 判断到期 = function (a) {
  if (!a) {
    return false
  }
  const currentTime = new Date();
  const targetTime = new Date(a);
  return currentTime > targetTime
}

const 返回提示 = function (msg) {
  var s = '<pre> ' + msg + '</pre>'
  ElMessageBox.alert(s, {
    dangerouslyUseHTMLString: true,
  });
  // console.log("返回提示");

  // ElMessage({
  //   dangerouslyUseHTMLString: true,
  //   showClose: true,
  //   message: s,
  //   duration: 0,
  // })

}
const 警告提示 = function (msg) {
  // var s = '<pre> ' + msg + '</pre>'  
  ElMessage({
    // dangerouslyUseHTMLString: true,
    showClose: true,
    message: msg,
    duration: 0,
    type: 'error',
  })

}
const cellState = (row, rowIndex) => {
  return {
    padding: "2px",
  };
};
const 时间转字符串 = function (时间) {
  if (!时间 || 时间 == null) {
    return "";
  }
  return new Date(时间).toLocaleString();
  // return  new Date(时间).format('YYYY-MM-DD HH:mm:ss');
};
const 记录打钩的 = (val) => {
  // 已勾的卡密=[]
  var a = [] as any[];
  for (const key in val) {
    a.push(val[key].card);
  }
  已勾的卡密.value = a;
  console.log(a);
};
const 续费按钮 = function () {
  待续费卡密.value.续费卡密 = 已勾的卡密.value.join("\n")
  显示续费卡密界面.value = true
}
const 查询所有卡密 = function () {
  加载中.value = true
  post("/查询所有卡密", {
    card: 卡密.value
  }).then(function (res) {
    加载中.value = false
    console.log(res.data.data);
    所有卡密.value = res.data.data;
    // 所有卡密数量.value = res.data.num
  });
};
const 查询充值卡 = function () {
  加载中.value = true
  post("/查询充值卡", {
    Rechargeable_card: 充值卡.value
  }).then(function (res) {
    加载中.value = false
    console.log(res);
    if (!res.data.state) {
      警告提示(res.data.msg)
      return
    }
    let data = res.data.data
    let msg = "\n有效期:" + data.Expiration_date
      + "\n剩余:" + data.Balance + "次"
      + " 每次:" + data.AddTime + "天"
      + "\n类型:" + data.S
      + "  类型:" + data.S
      + "  类型:" + data.S
      + "\n使用记录:" + data.Record
    返回提示(msg)

  })

}
const 确定续费卡密 = function () {
  const res = 待续费卡密.value.续费卡密.match(/[a-zA-Z0-9]+/g)
  if (!res) {
    console.log("没有卡")
    警告提示("请选择需要续费的卡密")
    return
  }
  if (充值卡.value.length < 6) {
    警告提示("请输入正确的充值卡")
    return

  }
  加载中.value = true
  post("/续费卡密", { Rechargeable_card: 充值卡.value, cards: res }).then(
    function (res) {
      加载中.value = false
      if (!res.data.state) {
        警告提示(res.data.msg)
        return
      }
      返回提示(res.data.msg)
      查询所有卡密();

    }

  )


}
// 查询软件列表();
// 查询所有卡密();
</script>
<style scoped>
.el-table :deep(.cell) {
  /* white-space: nowrap; */
  padding: 0px;
}

/* :global(.cell) {
    color: #ff0;
  } */
</style>
  