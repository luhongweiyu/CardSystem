<template>
  <div>
    <h1>charge充值</h1>

    <div>
      <el-select v-model="soft" placeholder="所属软件" style="width: 100px">
        <!-- <el-option label="Zone one" value="shanghai" /> -->
        <el-option v-for="(item, index) in 软件列表" :label="item.Software" :value="item.ID" />
      </el-select>

      <el-input v-model="卡密" placeholder="卡密" style="width: auto"  @keypress.enter="查询所有卡密()"/>

      <el-button type="success" :icon="Search" circle style="margin: 10px" @click="查询所有卡密()" />

      <!-- <el-button style="border: 0px; margin: 0px" type="danger" disabled>删除</el-button> -->
      <el-button style="border: 0px; margin: 0px" type="success" @click=" 准备生成充值卡()">生成充值卡</el-button>
    </div>

    <el-table :data="所有卡密" :cell-style="cellState" @selection-change="记录打钩的" border>
      <el-table-column type="selection" width="30" />
      <!-- 卡密,类型,在线状态,所属软件,生成日期,最近使用,到期时间,备注,操作 -->
      <el-table-column :show-overflow-tooltip="true" prop="card" label="卡密" width="180px" />
      <el-table-column :show-overflow-tooltip="true" label="所属软件" width="100px">
        <template #default="scope">{{ 计算所属软件(scope.row.software) }}</template>
      </el-table-column>
      <el-table-column :show-overflow-tooltip="true" prop="notes" label="备注" width="100px" />
      <el-table-column :show-overflow-tooltip="true" label="状态" width="100px">
        <template #default="scope">
          <span v-if="scope.row.state == 4" style="color: red;"> 冻结</span>
        </template>
      </el-table-column>
      <el-table-column :show-overflow-tooltip="true" prop="add_time" label="时间" width="60px" />
      <el-table-column :show-overflow-tooltip="true" prop="face_value" label="次数" width="60px" />
      <el-table-column :show-overflow-tooltip="true" prop="balance" label="剩余" width="60px" />
      <el-table-column :show-overflow-tooltip="true" prop="expiration_date" label="有效期" width="200px" />
      <el-table-column :show-overflow-tooltip="true" prop="create_time" label="创建时间" width="200px" />
      <el-table-column :show-overflow-tooltip="true" prop="record" label="使用记录" width="1600px">
        <template #default="scope">
          <el-button style="border: 0px; margin: 0px" size="small" type="warning" @click="查看充值记录(scope.row)"> 查看记录
          </el-button>
          <el-button style="border: 0px; margin: 0px" size="small" type="warning" @click="修改充值卡(scope.row, '2')">
            解除冻结
          </el-button>
          <el-button style="border: 0px; margin: 0px" size="small" type="danger" @click="修改充值卡(scope.row, '4')">
            冻结
          </el-button>
          <el-button style="border: 0px; margin: 0px" size="small" type="danger" @click="修改充值卡(scope.row, 'del')">
            删除
          </el-button>
          {{ scope.row.ID子账号 }}
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="充值卡输入显示" title="输入充值卡信息">
      <div v-loading="加载中">
        id:{{ 充值卡_新卡.software }}
        <el-form-item label="所属软件">
          <el-select v-model="充值卡_新卡.software" placeholder="所属软件" style="width: 200px">
            <!-- <el-option label="Zone one" value="shanghai" /> -->
            <el-option v-for="( item, index ) in  软件列表 " :label="item.Software" :value="item.ID" />
          </el-select>
        </el-form-item>
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
        <el-form-item label="备注">
          <el-input v-model="充值卡_新卡.备注" style="width: 200px" />
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
      </div>
    </el-dialog>
    <el-dialog v-model="充值记录显示" title="充值记录" width="90%">
      <span v-html="充值记录显示_内容"></span>
    </el-dialog>

  </div>
</template>

<script lang="ts" setup>
import { Check, Delete, Edit, Message, Search, Star } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from 'element-plus'
import { useCounterStore } from '../stores/counter'
import { reactive, ref } from 'vue'
const 加载中 = ref(false)
import { tr } from 'element-plus/es/locale'
const 软件列表 = ref([] as any[])
const 所有卡密 = ref([])
const 卡密 = ref('')
const post = useCounterStore().post
const soft = ref(0)

const 计算所属软件 = function (id) {
  for (const key in 软件列表.value) {
    if (软件列表.value[key].ID == id) {
      return 软件列表.value[key].Software
    }
  }
  return id
}

const cellState = (row, rowIndex) => {
  return {
    padding: "2px",
  };
};
const 已勾的卡密 = ref([] as any[]);
const 记录打钩的 = (val) => {
  // 已勾的卡密=[]
  var a = [] as any[];
  for (const key in val) {
    a.push(val[key].card);
  }
  已勾的卡密.value = a;
  console.log(a);
};
const 查询软件列表 = function () {
  post('/user_query_soft_list', {}).then(function (res) {
    if (res.data.state) {
      ElMessage.success('刷新软件列表获取成功')
      console.log(res.data.data)
      res.data.data.push({ ID: 0, Software: '全部软件' })
      软件列表.value = res.data.data
    } else {
      ElMessage.error(res.data.msg)
    }
    // 查询软件列表()
  })
}
const 查询所有卡密 = function () {
  post('/充值卡_查询', {
    software: soft.value,
    card: 卡密.value,
    similarity: 0
  }).then(function (res) {
    console.log(res.data)
    所有卡密.value = res.data.data
  })
}
const 充值记录显示 = ref(false)
const 充值记录显示_内容 = ref("")
const 查看充值记录 = function (a) {
  console.log(a.card)
  post('/充值卡_查询', {
    software: soft.value,
    card: a.card,
    similarity: 1
  }).then(function (res) {
    let data = res.data.data[0].record
    充值记录显示_内容.value = '<pre>' + data + '</pre>'
    充值记录显示.value = true
  })
}
const 修改充值卡 = function (a, command) {
  // 删除,冻结
  if (command == 'del') {
    var 确认数字 = Math.round(Math.random() * 10000)
    ElMessageBox.prompt('请输入数字:' + 确认数字 + '确认删除', '确认删除卡密:' + a.card + "吗?", {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
    })
      .then(({ value }) => {
        if (value == 确认数字) {
          post('/充值卡_修改', {
            software: soft.value,
            card: a.card,
            command: command
          })
        } else {
          ElMessage.error("输入的数字错误");
        }
      })
    return
  }
  post('/充值卡_修改', {
    software: soft.value,
    card: a.card,
    command: command
  })
}
查询软件列表()
查询所有卡密()



// 下面是充值卡 

const 充值卡输入显示 = ref(false);
const 充值卡_新卡 = reactive({
  软件名: 0,
  num: 1,
  software: 0,
  add_time: 30,
  充值次数: 1,
  有效期至: new Date(),
  备注: "",
  指定类型: 2,
  cards: ""
})
const 返回提示 = function (msg) {
  var s = '<pre> ' + msg + '</pre>'
  ElMessageBox.alert(s, {
    dangerouslyUseHTMLString: true,
  });
}
const 准备生成充值卡 = function () {
  充值卡输入显示.value = true
  充值卡_新卡.software = soft.value
  充值卡_新卡.软件名 = 计算所属软件(soft.value)
  充值卡_新卡.有效期至 = new Date((new Date()).getTime() + 3600 * 1000 * 24 * 30)

}
const 确定生成充值卡 = function () {
  console.log(充值卡_新卡.软件名);

  if (充值卡_新卡.software == 0) {
    return
  }
  加载中.value = true
  post("/充值卡_生成", 充值卡_新卡).then(
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

</script>
<style scoped></style>
