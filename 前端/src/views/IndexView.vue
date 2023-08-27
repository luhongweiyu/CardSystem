<template>
  <el-menu :collapse="stores.导航开关" default-active="2" active-text-color="#ffd04b" background-color="#545c64" text-color="#fff" @open="handleOpen" @close="handleClose" router>
    <!-- <div class="info"><img src="assets/img/avatar.029cf3c7.jpg" style="width: 86px; height: 86px; border-radius: 50%; margin-bottom: 10px" /></div> -->

    <!-- 配置没有二级菜单的菜单路由内容 
        - 通过v-for循环遍历 noChildren 数据对象，分别取出:
          - 菜单路由item.path
          - 菜单icon
          - 菜单title名称
    -->
    <el-menu-item :index="item.path" v-for="item in noChildren" :key="item.path">
      <el-icon>
        <component :is="item.icon"></component>
      </el-icon>
      <span>{{ item.label }}</span>
    </el-menu-item>
    <!-- 配置有二级菜单的菜单路由内容
          - 通过v-for循环遍历 hasChildren 数据对象，分别取出：
            - 一级菜单的index
            - 一级菜单icon
            - 一级菜单名字
            通过遍历当前一级菜单的二级菜单数据对象，分别取出：
              - 二级菜单index
              - 二级菜单名字label
              - 二级菜单icon
     -->
    <el-sub-menu :index="item.label" v-for="(item, index) in hasChildren" :key="index">
      <template #title>
        <el-icon>
          <component :is="item.icon"></component>
        </el-icon>
        <span>{{ item.label }}</span>
      </template>
      <el-menu-item-group>
        <el-menu-item :index="subItem.path" v-for="(subItem, subIndex) in item.children" :key="subIndex">
          {{ subItem.label }}
        </el-menu-item>
      </el-menu-item-group>
    </el-sub-menu>
  </el-menu>
</template>

<script lang="ts" setup>
import { ref, reactive, computed } from "vue";
// import { Document, Menu as IconMenu, Location, Setting } from "@element-plus/icons-vue";

const handleOpen = (key: string, keyPath: string[]) => {
  console.log(key, keyPath);
};
const handleClose = (key: string, keyPath: string[]) => {
  console.log(key, keyPath);
};

// 配置菜单路由
const asideMenu = reactive([
  {
    // 首页
    path: "/index",
    name: "index",
    icon: "HomeFilled",
    label: "首页"
  },
  {
    // 软件
    path: "/software",
    name: "software",
    icon: "Iphone",
    label: "软件"
  },
  {
    // 卡密
    path: "/card",
    name: "card",
    icon: "Postcard",
    label: "卡密"
  },
  {
    // 帮助
    path: "/help",
    name: "help",
    icon: "QuestionFilled",
    label: "帮助"
  },
  // {
  //   // 文件
  //   path: "/file",
  //   name: "file",
  //   icon: "Files",
  //   label: "文件"
  // },
  // {
  //   // 用户
  //   path: "/user",
  //   name: "user",
  //   icon: "UserFilled",
  //   label: "用户"
  // },
  // {
  //   // 充值
  //   path: "/charge",
  //   name: "charge",
  //   icon: "Money",
  //   label: "充值"
  // },
  // {
  //   // 代理
  //   path: "/agent",
  //   name: "agent",
  //   icon: "Ship",
  //   label: "代理"
  // },
  // {
  //   // 数据
  //   path: "/data",
  //   name: "data",
  //   icon: "DataLine",
  //   label: "数据"
  // },
  // {
  //   // 日志
  //   path: "/log",
  //   name: "log",
  //   icon: "List",
  //   label: "日志"
  // },
  // {
  //   // 消息
  //   path: "/msg",
  //   name: "msg",
  //   icon: "ChatLineRound",
  //   label: "消息"
  // },
  // {
  //   // 推广
  //   path: "/promotion",
  //   name: "promotion",
  //   icon: "Promotion",
  //   label: "推广"
  // },
  {
    // 设置
    path: "/setting",
    name: "setting",
    icon: "setting",
    label: "设置"
  },
  {
    // 实名认证
    path: "/authentication",
    name: "authentication",
    icon: "VideoCameraFilled",
    label: "实名认证"
  },

  // {
  //   icon: "location",
  //   label: "资金",
  //   children: [
  //     {
  //       // 售卡日志
  //       path: "/sell_log",
  //       name: "sell_log",
  //       label: "售卡日志",
  //       icon: "setting"
  //     },
  //     {
  //       // 提现日志
  //       path: "/cash",
  //       name: "cash",
  //       label: "提现日志",
  //       icon: "setting"
  //     },
  //     {
  //       // 销售统计
  //       path: "/sell_count",
  //       name: "sell_count",
  //       label: "销售统计",
  //       icon: "setting"
  //     }
  //   ]
  // }
]);
const noChildren = computed(() => {
  return asideMenu.filter((item) => !item.children);
});
const hasChildren = computed(() => {
  return asideMenu.filter((item) => item.children);
});

import { useCounterStore } from "../stores/counter.js";
const stores = useCounterStore();
// const isCollapse = dhkg();
</script>
<style scoped>
.el-menu {
  /* width: 200px; */
  left: 0px;
  /* height: 100%; */
  border: none;
}
.el-menu-vertical-demo:not(.el-menu--collapse) {
  /* /* width: 100px; */
  /* min-height: 200%;  */
}
</style>
