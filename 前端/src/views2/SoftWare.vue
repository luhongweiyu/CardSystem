<template>
  <div>
    <h1>SoftWare软件</h1>
  </div>
  <el-button type="success" @click="软件名称输入显示 = true">添加软件</el-button>
  用户卡密查询链接:
  <el-link type="success" :href="用户卡密查询链接" target="_blank"> {{ 用户卡密查询链接 }}</el-link>
  <el-table :data="软件列表" style="width: 100%">
    <el-table-column prop="ID" label="软件ID" width="70" />
    <el-table-column prop="Software" label="软件名称" width="200">
      <template #default="scope">
        <el-input v-model="scope.row.Software" placeholder="请输入软件名" />
      </template>

    </el-table-column>
    <el-table-column prop="bulletin" label="公告" width="280">
      <template #default="scope">
        <el-input v-model="scope.row.Bulletin" autosize type="textarea" placeholder="请输入公告内容" style="display:inline"
          width="100px" />
      </template>
    </el-table-column>
    <el-table-column prop="bulletin" label="暂停扣时(天)" width="280">
      <template #default="scope">
        <el-input-number v-model="scope.row.暂停扣时" :min="-999" :max="999" />
      </template>
    </el-table-column>

    <el-table-column label="操作" width="200">
      <template #default="scope">
        <el-button link type="primary" size="small" @click="保存公告(scope.row)" style="display:inline">保存</el-button>
        <el-button link type="primary" size="small" @click="删除软件(scope.row.ID)">删除</el-button>
      </template>
    </el-table-column>
  </el-table>

  <el-button link type="primary" size="small" @click="查询子账号" style="display:inline">查询授权账号</el-button>

  <div v-for="user_1, k1 in 授权列表" :key="k1">
    {{ user_1.ID子账号 }}
    账号: <el-input v-model="user_1.name" style="width: 100px" placeholder="Please input" disabled/>
    密码:<el-input v-model="user_1.password" style="width: 100px" placeholder="Please input" />
    余额:<el-input v-model="user_1.余额" style="width: 100px" placeholder="Please input" />
    <el-button link type="primary" size="small" @click="保存授权设置(user_1)" style="display:inline">保存授权设置</el-button>
    <template v-for="(soft, k2) in 软件列表" :key="k2">
      <!-- <span v-for="(软件, k2) in [{ID:1,Software:'软件名'},{ID:2,Software:'软件名2'}]" :key="k2"> -->
      <span v-show="user_1.价格[(soft.ID) + ''] || user_1.价格[(soft.ID) + ''] == 0">
        {{ soft.Software }}
        <el-input-number v-model="user_1['价格'][soft.ID]" :min="-999" :max="999" />
      </span>
    </template>
  </div>

  <el-dialog v-model="软件名称输入显示" title="输入软件名称">
    {{ 新增软件名 }}
    <el-input v-model="新增软件名" placeholder="软件名称" />
    <div style="margin: 20px">
      <el-button type="success" @click="添加软件()">确定</el-button>

      <el-button type="info" @click="软件名称输入显示 = false"> 取消</el-button>
    </div>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ElMessage, ElMessageBox } from "element-plus";
import { useCounterStore } from "../stores/counter";
import { reactive, ref } from "vue";
import { tr } from "element-plus/es/locale";
const 软件列表 = ref([]);
const 新增软件名 = ref("");
const 加载中 = ref(false)
const 软件名称输入显示 = ref(false);
const 充值卡输入显示 = ref(false);
const 用户卡密查询链接 = ref("http://" + window.location.hostname + ((window.location.port && (":" + window.location.port)) || "") + "/visitor/index.html?center_id=" + useCounterStore().用户id)
const post = useCounterStore().post;
const 充值卡_新卡 = reactive({
  软件名: 0,
  num: 1,
  software: 0,
  add_time: 30,
  充值次数: 1,
  有效期至: new Date(),
  指定类型: 2,
  cards: ""
})
const 返回提示 = function (msg) {
  var s = '<pre> ' + msg + '</pre>'
  ElMessageBox.alert(s, {
    dangerouslyUseHTMLString: true,
  });
}
const 授权列表 = ref([]);
const 添加软件 = function () {
  软件名称输入显示.value = false;
  post("/user_add_soft", {
    software: 新增软件名.value
  }).then(function (res) {
    if (res.data.state) {
      ElMessage.success("添加成功");
      查询软件列表();
    } else {
      ElMessage.error(res.data.msg);
    }
  });
};
const 删除软件 = function (id) {
  var 确认数字 = Math.round(Math.random() * 10000000000)

  ElMessageBox.prompt('请输入数字:' + 确认数字 + '确认删除', '确认删除id为:' + id + "的软件吗?", {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
  })
    .then(({ value }) => {
      if (value == 确认数字) {
        post("/user_del_soft", { id: id }).then(function (res) {
          if (res.data.state) {
            ElMessage.success("删除成功");
            查询软件列表();
          } else {
            ElMessage.error(res.data.msg);
          }
        });
      } else {
        ElMessage.error("输入的数字错误");
      }
    })
};
const 保存公告 = function (row) {
  post("/user_modify_bulletin", { ID: row.ID, Software: row.Software, Bulletin: row.Bulletin, 暂停扣时: row.暂停扣时 }).then(function (res) {
    if (res.data.state) {
      ElMessage.success("修改成功");
      查询软件列表();
    } else {
      ElMessage.error(res.data.msg);
    }
  });
};
const 查询软件列表 = function () {
  post("/user_query_soft_list", {}).then(function (res) {
    if (res.data.state) {
      ElMessage.success("刷新软件列表获取成功");
      console.log(res.data.data);
      软件列表.value = res.data.data;
    } else {
      ElMessage.error(res.data.msg);
    }
    // 查询软件列表()
  });
};
const 查询子账号 = function () {
  post("/查询子账号", {}).then(function (res) {
    console.log(res.data)
    if (res.data.state) {
      ElMessage.success("刷新授权账号列表成功");
      console.log(res.data.data);
      // console.log(typeof(res.data.data[0]))

      for (let i = 0; i < res.data.data.length; i++) {
        res.data.data[i].价格 = res.data.data[i].价格 || "{}"
        res.data.data[i].价格 = JSON.parse(res.data.data[i].价格)
        // res.data.data[i].价格 = ref( JSON.parse(res.data.data[i].价格))
        for (let i2 = 0; i2 < 软件列表.value.length; i2++) {
          res.data.data[i].价格[软件列表.value[i2].ID] = res.data.data[i].价格[软件列表.value[i2].ID] || 0
          // res.data.data[i].价格.value[软件列表.value[i2].ID] = res.data.data[i].价格.value[软件列表.value[i2].ID] || 0
        }
      }
      console.log(res.data.data)

      // res.data.data[0].价格 = JSON.parse(res.data.data[0].价格)
      // if (!res.data.data[0].价格) {
      //   res.data.data[0].价格 = {}
      // }

      授权列表.value = res.data.data;
    } else {
      ElMessage.error(res.data.msg);
    }
  });
}
const 保存授权设置 = function (账号) {
  console.log(账号.价格)
  JSON.stringify(账号.价格)
  let data = {
    价格: JSON.stringify(账号.价格),
    ID子账号: 账号.ID子账号,
    name: 账号.name,
    password: 账号.password,
    余额: 账号.余额,
  }
  // console.log(data)
  post("/设置子账号", { data: data }).then(function (res) {
    console.log(res.data)
    if (res.data.state) {
      ElMessage.success("刷新授权账号列表成功");
      console.log(res.data.data);

    } else {
      ElMessage.error(res.data.msg);
    }
  });
}
// 查询子账号()
查询软件列表();
</script>
<style scoped></style>
