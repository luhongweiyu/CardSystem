<script setup>
import { ref, reactive, computed } from "vue";
import { RouterLink, RouterView } from "vue-router";
import Indexhtml from "./views/IndexView.vue";
import HeaderView from "./views/HeaderView.vue";
import Login from "./views/Login.vue";
import Home from "./views/HomeView.vue";
import HelloWorld from "./components/HelloWorld.vue";
import { useCounterStore } from "./stores/counter.js";
import { storeToRefs } from "pinia";
const stores = useCounterStore();

const { 账号, 密码, 登录状态 } = storeToRefs(stores);
// const isCollapse = dhkg();
const 导航宽 = computed(() => {
  if (stores.导航开关) {
    return "0px";
  } else {
    return "200px";
  }
});
</script>

<template>
  <Login v-if="!登录状态" />

  <div v-if="登录状态">
    <el-container>
      <el-header>
        <HeaderView />
      </el-header>
      <el-main>
        <div>
          <Indexhtml />
        </div>
        <div>
          <RouterView />
        </div>
        <!-- </div> -->
      </el-main>
    </el-container>
  </div>
</template>
<style type="text/css">
html,
body,
#app {
  top: 0px;
  padding: 0px;
  margin: 0px;
  height: 100%;
  color: #eee;
  background: #222;
  /* width: 100%; */
  /* overflow: auto; */
}
</style>

<style scoped>
.el-main {
  padding: 0px;
  width: 100%;
  display: flex;
  flex-direction: row;
}

.el-header {
  padding: 0px;
  margin: 0px;
  width: 100%;
  height:auto;
}

.left {
  float: left;
  top: 0px;
  /* margin: 0px; */
  /* padding: 0px; */
  /* position: sticky; */
}

.el-container {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
}
</style>
