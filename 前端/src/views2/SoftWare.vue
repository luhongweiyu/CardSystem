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

    <el-table-column fixed="right" label="操作" width="120">
      <template #default="scope">
        <el-button link type="primary" size="small" @click="保存公告(scope.row)" style="display:inline">保存</el-button>
        <el-button link type="primary" size="small" @click="删除软件(scope.row.ID)">删除</el-button>
        <!-- <el-button link type="primary" size="small">修改</el-button> -->
      </template>
    </el-table-column>
  </el-table>


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
const 软件列表 = ref([]);
const 新增软件名 = ref("");
const 软件名称输入显示 = ref(false);
const 用户卡密查询链接 = ref("http://" + window.location.hostname + ((window.location.port && (":" + window.location.port)) || "") + "/usercard/index.html?center_id=" + useCounterStore().用户id)
const post = useCounterStore().post;
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
  ElMessageBox.confirm('确认删除id为:' + id + "的软件吗?").then(() => {
    post("/user_del_soft", { id: id }).then(function (res) {
      if (res.data.state) {
        ElMessage.success("删除成功");
        查询软件列表();
      } else {
        ElMessage.error(res.data.msg);
      }
    });
  })
};
const 保存公告 = function (row) {
  post("/user_modify_bulletin", { id: row.ID, software: row.Software, bulletin: row.Bulletin }).then(function (res) {
    if (res.data.state) {
      ElMessage.success("删除成功");
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
查询软件列表();
</script>
<style scoped></style>
