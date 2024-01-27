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

    <el-table-column fixed="right" label="操作" width="200">
      <template #default="scope">
        <el-button link type="primary" size="small" @click="保存公告(scope.row)" style="display:inline">保存</el-button>
        <el-button link type="primary" size="small" @click="删除软件(scope.row.ID)">删除</el-button>
        <el-button link type="primary" size="small" @click="准备生成充值卡(scope.row)">生成充值卡</el-button>
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
  <el-dialog v-model="充值卡输入显示" title="输入充值卡信息" v-loading="加载中">
    id:{{ 充值卡_新卡.software }}名称:{{ 充值卡_新卡.软件名 }}

    <el-form-item label="生成数量">
      <el-input-number v-model="充值卡_新卡.num" :min="1" :max="50" style="width: 200px" />
    </el-form-item>
    <el-form-item label="充值次数">
      <el-input-number v-model="充值卡_新卡.充值次数" :min="1" :max="1000" style="width: 200px" />
    </el-form-item>
    <el-form-item label="充值天数">
      <el-input-number v-model="充值卡_新卡.add_time" :min="1" :max="1000" style="width: 200px" />
    </el-form-item>
    <el-form-item label="有效期至">
      <el-date-picker v-model="充值卡_新卡.有效期至" type="datetime" placeholder="选择时间" style="width: 200px" />
    </el-form-item>
    <el-form-item label="生成类型">
      <el-radio-group v-model="充值卡_新卡.指定类型" class="ml-4">
        <el-radio :label="1" size="large">随机生成</el-radio>
        <el-radio :label="2" size="large">指定卡密</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-form-item label="确定">
      <el-button style="border: 10px; margin: 10px" type="success" @click="确定生成充值卡()">生成充值卡</el-button>
    </el-form-item>
    <el-form-item label="卡密">
      <!--  v-if="新卡.指定类型 == 2" -->
      <el-input v-model="充值卡_新卡.cards" :autosize="{ minRows: 3 }" type="textarea" placeholder="输入自定义卡密内容"
        style="width: 300px" />
    </el-form-item>
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
  指定类型: 1,
  cards: ""
})
const 返回提示 = function (msg) {
  var s = '<pre> ' + msg + '</pre>'
  ElMessageBox.alert(s, {
    dangerouslyUseHTMLString: true,
  });
}
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
const 准备生成充值卡 = function (row) {
  充值卡输入显示.value = true
  充值卡_新卡.software = row.ID
  充值卡_新卡.软件名 = row.Software
  充值卡_新卡.有效期至 = new Date((new Date()).getTime() + 3600 * 1000 * 24 * 30)

}
const 确定生成充值卡 = function () {
  加载中.value = true
  post("/生成充值卡", 充值卡_新卡).then(
    function (res) {
      加载中.value = false
      if (!res.data.state) {
        ElMessage.error(res.data.msg)
        return
      }
      返回提示(res.data.msg)
    }
  )


}
查询软件列表();
</script>
<style scoped></style>
