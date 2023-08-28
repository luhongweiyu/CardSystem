<template>
  <div>
    <div>
      <el-row>
        <el-col :span="12"> 卡密列表 </el-col>
        <el-col :span="12">
          <el-button style="border: 0px; margin: 0px" type="success" @click="显示生成卡密界面 = true"> 添加卡密</el-button>
        </el-col>
      </el-row>
    </div>
    <div>
      <el-select v-model="soft" placeholder="所属软件" style="width: 100px">
        <!-- <el-option label="Zone one" value="shanghai" /> -->
        <el-option v-for="(item, index) in 软件列表" :label="item.Software" :value="item.ID" />
      </el-select>

      <el-select v-model="state" placeholder="状态" style="width: 100px">
        <el-option v-for="(item, index) in 状态列表" :label="item[1]" :value="item[0]" />
      </el-select>

      <el-select v-model="类型" placeholder="类型" style="width: 100px" allow-create>
        <!-- <el-input-number v-model="类型" :min="-1" :max="365" controls-position="right" style="width: 100px"  placeholder="类型天数"  /> -->
        <!-- <el-input v-model="类型" placeholder="类型天数" style="width: 100px" /> -->
        <el-input-number v-model="类型" placeholder="类型天数" style="width: 200px" />
        <el-option v-for="(item, index) in 类型列表" :label="item[1]" :value="item[0]" />
      </el-select>

      <el-input v-model="卡密" placeholder="卡密" style="width: auto" />
      <el-input v-model="备注" placeholder="备注" style="width: auto" />
      <el-button type="success" :icon="Search" circle style="margin: 10px" @click="查询所有卡密()" />

      <el-button style="border: 0px; margin: 0px" type="primary">导出</el-button>
      <el-button style="border: 0px; margin: 0px" type="success" @click="续费按钮">续费</el-button>
      <el-button style="border: 0px; margin: 0px" type="danger" @click="删除选择的卡密">删除</el-button>
      <el-button style="border: 0px; margin: 0px" type="danger" disabled>删除</el-button>
    </div>

    <el-table :data="所有卡密" :cell-style="cellState" @selection-change="记录打钩的" border>
      <el-table-column type="selection" width="30" />
      <!-- 卡密,类型,在线状态,所属软件,生成日期,最近使用,到期时间,备注,操作 -->
      <el-table-column :show-overflow-tooltip="true" prop="card" label="卡密" width="180px" />
      <el-table-column :show-overflow-tooltip="true" label="类型" width="60px">
        <template #default="scope"> {{ 计算卡密类型(scope.row.available_time) }}</template>
      </el-table-column>
      <el-table-column :show-overflow-tooltip="true" prop="address" label="状态" width="60px">
        <template #default="scope">
          <span v-if="scope.row.card_state == 4" style="color:red">冻结</span>
          <span v-else-if="判断到期(scope.row.end_time)" style="color:rgb(131, 71, 71)">到期</span>
          <span v-else-if="scope.row.end_time" style="color:rgb(9, 255, 0)">激活</span>


        </template>
      </el-table-column>
      <el-table-column :show-overflow-tooltip="true" label="所属软件" width="100px">
        <template #default="scope"> {{ 计算所属软件(scope.row.software) }}</template>
      </el-table-column>
      <el-table-column :show-overflow-tooltip="true" prop="create_time" label="生成日期" width="130px">
        <template #default="scope"> {{ 时间转字符串(scope.row.create_time) }}</template>
      </el-table-column>
      <el-table-column :show-overflow-tooltip="true" prop="use_time" label="最近使用" width="130px">
        <template #default="scope"> {{ 时间转字符串(scope.row.use_time) }}</template>
      </el-table-column>
      <el-table-column :show-overflow-tooltip="true" prop="end_time" label="到期时间" width="130px">
        <template #default="scope"> {{ 时间转字符串(scope.row.end_time) }}</template>
      </el-table-column>
      <el-table-column :show-overflow-tooltip="true" prop="address" label="操作" width="100px">
        <template #default="scope">
          <el-button style="border: 0px; margin: 0px" size="small" type="warning"
            @click="修改单个卡密(scope.row)">修改</el-button>
          <el-button style="border: 0px; margin: 0px" size="small" type="danger" @click="删除单个卡密(scope.row)">删除</el-button>
        </template>
      </el-table-column>
      <el-table-column :show-overflow-tooltip="true" prop="notes" label="备注" />
      <el-table-column :show-overflow-tooltip="true" prop="config_content" label="配置" />
    </el-table>

    <el-pagination v-model:current-page="所有卡密_当前页" v-model:page-size="每页卡密数量" :page-sizes="[20, 100, 200, 300, 400, 500]"
      background layout="total, sizes,prev, pager, next,jumper" :total="所有卡密数量" @size-change="查询所有卡密"
      @current-change="查询所有卡密" />
    <!-- 所属软件  -->
    <!-- 全部状态 -->
    <!-- 选择类型  -->
    <!-- 代理人id -->
    <!-- 卡密备注 -->
    <!-- 操作 -->
    <!-- 批量操作 -->
    <!-- 导出 -->
    <!-- 续期 -->
    <!-- 删除 -->
    <!-- 生成卡密界面 -->
    <el-dialog v-model="显示生成卡密界面" title="生成卡密" width="80%">
      <el-form label-position="right" label-width="100px" style="max-width: 460px" v-loading="加载中">
        <el-form-item label="所属软件">
          <el-select v-model="新卡.software" placeholder="所属软件" style="width: 200px">
            <!-- <el-option label="Zone one" value="shanghai" /> -->
            <el-option v-for="( item, index ) in  软件列表 " :label="item.Software" :value="item.ID" />
          </el-select>
        </el-form-item>
        <el-form-item label="卡密类型">
          <el-select v-model="新卡.available_time" placeholder="类型" style="width: 200px" allow-create>
            <!-- <el-input-number v-model="类型" :min="-1" :max="365" controls-position="right" style="width: 100px"  placeholder="类型天数"  /> -->
            <!-- <el-input v-model="新卡.available_time" placeholder="类型天数" style="width: 200px" /> -->
            <el-input-number v-model="新卡.available_time" placeholder="类型天数" style="width: 200px" />
            <el-option v-for="( item, index ) in  类型列表 " :label="item[1]" :value="item[0]" />
          </el-select>
        </el-form-item>
        <el-form-item label="生成数量">
          <el-input-number v-model="新卡.num" :min="1" :max="1000" style="width: 200px" />
        </el-form-item>
        <el-form-item label="配置">
          <el-input v-model="新卡.config_content" placeholder="配置" style="width: 200px" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="新卡.notes" placeholder="备注" style="width: 200px" />
        </el-form-item>
        <el-form-item label="最晚激活时间"> <el-input-number v-model="新卡.latest_activation_time" :min="-1" :max="1000"
            style="width: 100px" />天内(-1表示不限制,0表示立即激活) </el-form-item>

        <el-form-item label="生成类型">
          <el-radio-group v-model="新卡.指定类型" class="ml-4">
            <el-radio :label="1" size="large">随机生成</el-radio>
            <el-radio :label="2" size="large">指定卡密</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="确定">
          <el-button style="border: 10px; margin: 10px" type="success" @click="确定生成卡密()">生成卡密</el-button>
          <el-button style="border: 10px; margin: 10px" @click="复制生成的卡密()">复制卡密</el-button>
        </el-form-item>

        <el-form-item label="卡密">
          <!--  v-if="新卡.指定类型 == 2" -->
          <el-input v-model="新卡.cards" :autosize="{ minRows: 3 }" type="textarea" placeholder="输入自定义卡密内容"
            style="width: 300px" />
        </el-form-item>
      </el-form>
    </el-dialog>
    <!-- 修改卡密界面 -->
    <el-dialog v-model="显示修改卡密界面" title="修改卡密" width="80%">
      <el-form label-position="right" label-width="100px" style="max-width: 460px;display: inline-block;" v-loading="加载中">
        <el-form-item label=" ">
          {{ 待修改卡密.card }}
        </el-form-item>
        <el-form-item label="创建日期">
          {{ 待修改卡密.create_time }}
        </el-form-item>
        <el-form-item label="所属软件">
          <el-select v-model="待修改卡密.software" placeholder="所属软件" style="width: 200px">
            <el-option v-for="( item, index ) in  软件列表 " :label="item.Software" :value="item.ID" />
          </el-select>
        </el-form-item>
        <el-form-item label="到期时间">
          <el-date-picker v-model="待修改卡密.end_time" type="datetime" placeholder="选择时间" style="width: 200px"
            :default-time="new Date()" :shortcuts="shortcuts" />

        </el-form-item>
        <el-form-item label="配置">
          <el-input v-model="待修改卡密.config_content" placeholder="配置" style="width: 200px" />

        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="待修改卡密.notes" placeholder="备注" style="width: 200px" />
        </el-form-item>
        <el-form-item label="冻结状态">
          <el-radio-group v-model="待修改卡密.card_state">
            <el-radio :label="2">正常</el-radio>
            <el-radio :label="4">冻结</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label=" ">
          <el-button style="border: 0px; margin: 0px" type="success" @click="保存修改的卡密()">保存修改</el-button>
        </el-form-item>
      </el-form>
      <div style="display: inline-block; margin: 50px; vertical-align: top">
        <pre>
          {{ 单个卡密详情 }}
        </pre>
      </div>
    </el-dialog>
    <!-- 续费界面 -->
    <!-- 续费时间 -->
    <el-dialog v-model="显示续费卡密界面" title="续费卡密" width="80%">
      <el-form label-position="right" label-width="100px" style="max-width: 460px" v-loading="加载中">
        <el-form-item label="续费时间">
          <el-input-number v-model="待续费卡密.续费时间" :min="-1000" :max="1000" style="width: 200px" />天
        </el-form-item>
        <el-form-item label="">
          <el-button style="border: 10px; margin: 10px" type="success" @click="确定续费卡密()">确定续费</el-button>
        </el-form-item>
        <el-form-item label="卡密">
          <!--  v-if="新卡.指定类型 == 2" -->
          <el-input v-model="待续费卡密.续费卡密" :autosize="{ minRows: 3 }" type="textarea" placeholder="输入需要续费的卡密"
            style="width: 300px" />
        </el-form-item>
      </el-form>
    </el-dialog>

  </div>
</template>

<script lang="ts" setup>
import { Check, Delete, Edit, Message, Search, Star } from "@element-plus/icons-vue";
import { reactive, ref } from "vue";
import { useCounterStore } from "../stores/counter";
import { ElMessage, ElMessageBox } from "element-plus";
import { tr } from "element-plus/es/locale";
import axios, { Axios } from "axios";


const post = useCounterStore().post;
const 软件列表 = ref([] as any[]);
const 状态列表 = reactive([
  [0, "全部状态"],
  [1, "未激活"],
  [2, "正常"],
  [3, "到期"],
  [4, "冻结"],
]);
const 类型列表 = reactive([
  [0, "全部类型"],
  [1, "天卡"],
  [7, "周卡"],
  [30, "月卡"],
  [91, "季卡"],
  [365, "年卡"],
  [36500, "永久卡"],
]);
const 所有卡密_当前页 = ref(1)
const 所有卡密数量 = ref(0)
const 每页卡密数量 = ref(20)
const soft = ref(0);
const state = ref(0);
const 类型 = ref(0);
const 卡密 = ref("");
const 备注 = ref("");
const 新卡 = ref({
  software: null,
  available_time: 1,
  num: 1,
  config_content: "",
  notes: "",
  latest_activation_time: -1,
  cards: "",
  指定类型: 1,
});
const 单个卡密详情 = ref("")
const 所有卡密 = ref([]);
const 显示生成卡密界面 = ref(false);
const 显示修改卡密界面 = ref(false);
const 显示续费卡密界面 = ref(false);
const 加载中 = ref(false);
const 已勾的卡密 = ref([] as any[]);
const 待修改卡密 = ref({
  create_time: null as string | null,
  software: null,
  end_time: null,
  config_content: null,
  notes: null,
  // latest_activation_time: null,
  card: null,
  card_state: null,

})
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
}
const 查询所有卡密 = function () {
  post("/user_query_card", {
    software: soft.value,
    card_state: state.value,
    available_time: 类型.value,
    card: 卡密.value,
    notes: 备注.value,
    每页: 每页卡密数量.value,
    当前页: 所有卡密_当前页.value
  }).then(function (res) {
    console.log(res.data);
    所有卡密.value = res.data.data;
    所有卡密数量.value = res.data.num
  });
};
const 查询软件列表 = function () {
  post("/user_query_soft_list", {}).then(function (res) {
    if (res.data.state) {
      ElMessage.success("刷新软件列表获取成功");
      console.log(res.data.data);
      res.data.data.push({ ID: 0, Software: "全部软件" });
      软件列表.value = res.data.data;
    } else {
      ElMessage.error(res.data.msg);
    }
    // 查询软件列表()
  });
};
const 确定生成卡密 = function () {
  console.log(新卡.value);
  if (!新卡.value.software) {
    ElMessage.error("请选择所属软件");
    return;
  }
  加载中.value = true;
  post("/add_new_card", 新卡.value).then(function (res) {
    // let msg = res.data.msg.replace(/\n/g, "<br/>");
    let msg = res.data.msg;
    if (res.data.state == true) {
      加载中.value = false;
      ElMessage.success(res.data.msg);

      // 显示生成卡密界面.value = false;
      查询所有卡密();
      if (新卡.value.指定类型 == 1) {
        新卡.value.cards = res.data.data;
      }
    } else {
      加载中.value = false;
      ElMessage.error(res.data.msg);
    }
    返回提示(msg)
  });
};
const 复制生成的卡密 = function () {
  try {
    navigator.clipboard.writeText(新卡.value.cards);
    ElMessage.warning("复制成功");
  } catch (err) { }
};
const rowState = (row, rowIndex) => {
  console.log(row);
  return {
    // ["display"]:"inline-block"
    // backgroundColor: "pink",
    // color: "#fff"
  };
};
const cellState = (row, rowIndex) => {
  return {
    padding: "2px",
  };
};
const 添加卡密 = () => { };
const 时间转字符串 = function (时间) {
  console.log(时间)
  if (!时间 || 时间 == null) {
    return "";
  }
  return new Date(时间).toLocaleString();
  // return  new Date(时间).format('YYYY-MM-DD HH:mm:ss');
};
const 计算卡密类型 = function (t) {
  var a = {
    [36500]: "永久卡",
    0.5: "半日",
    1: "日卡",
    3.5: "半周卡",
    7: "周卡",
    15: "半月卡",
    30: "月卡",
    91: "季卡",
    182: "半年卡",
    365: "年卡",
  };
  return a[t] || t + "天";
};
const 计算所属软件 = function (id) {
  for (const key in 软件列表.value) {
    if (软件列表.value[key].ID == id) {
      return 软件列表.value[key].Software;
    }
  }
  return id;
};

const 修改单个卡密 = function (row) {
  console.log(row);
  待修改卡密.value.create_time = 时间转字符串(row.create_time)
  待修改卡密.value.software = row.software
  待修改卡密.value.end_time = row.end_time
  待修改卡密.value.config_content = row.config_content
  待修改卡密.value.notes = row.notes
  待修改卡密.value.card = row.card
  待修改卡密.value.card_state = row.card_state

  显示修改卡密界面.value = true
  console.log(待修改卡密.value)


  单个卡密详情.value = "加载中... "

  axios.post("http://" + window.location.hostname + ":802/card/query?center_id=" + useCounterStore().用户id + "&card=" + row.card).then(function (res) {
    console.log(res.data)
    单个卡密详情.value = res.data.data

  })


};
const 保存修改的卡密 = function () {
  加载中.value = true
  post("/modify_card", 待修改卡密.value).then(function (res) {
    if (res.data.state == true) {
      ElMessage.success("修改成功")
    } else {
      返回提示(res.data.state)
    }
    加载中.value = false
  })
};
const 删除卡密 = function (cards) {
  // [row.card] 
  // 已勾的卡密.value
  post("/delete_card", { cards: cards }).then(function (res) {
    返回提示(res.data.msg)
    查询所有卡密();
  });

}
const 删除单个卡密 = function (row) {
  删除卡密([row.card])
};

const 删除选择的卡密 = function () {
  删除卡密(已勾的卡密.value)
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
const 确定续费卡密 = function () {
  const res = 待续费卡密.value.续费卡密.match(/[a-zA-Z0-9]+/g)
  if (!res) {
    console.log("没有卡")
    return
  }
  加载中.value = true
  post("/add_card_time", { add_time: 待续费卡密.value.续费时间, cards: res }).then(
    function (res) {
      加载中.value = false
      if (!res.data.state) {
        ElMessage.error(res.data.msg)
        return
      }
      返回提示(res.data.msg)
      查询所有卡密();

    }

  )


}
const shortcuts = [
  {
    text: '明天',
    value: () => {
      const date = new Date()
      date.setTime(date.getTime() + 3600 * 1000 * 24)
      return date
    },
  },
  {
    text: '下周',
    value: () => {
      const date = new Date()
      date.setTime(date.getTime() + 3600 * 1000 * 24 * 7)
      return date
    },
  },
  {
    text: '下月',
    value: () => {
      const date = new Date()
      date.setTime(date.getTime() + 3600 * 1000 * 24 * 7 * 30)
      return date
    },
  },
]
查询软件列表();
查询所有卡密();
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
