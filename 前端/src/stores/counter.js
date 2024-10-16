import { ref, computed } from "vue";
import { defineStore } from "pinia";
import axios from "axios";

export const useCounterStore = defineStore("counter", () => {
  const count = ref(0);
  const doubleCount = computed(() => count.value * 2);
  function increment() {
    count.value++;
  }
  const 用户id = ref("");
  const 导航开关 = ref(false);
  const 账号 = ref("abc");
  const 密码 = ref("123456");
  const 登录状态 = ref(false);
  const api次数 = ref(0);
  const 是子账号 = ref(false);
  const 账号信息 = ref({});
  const post = function (链接, 参数) {
    参数.name = 账号.value;
    参数.password = 密码.value;
    // return axios.post("http://localhost:802/admin" + 链接, 参数, { headers: { "Content-Type": "application/json" } });
    let s = ":802/admin"
    if (是子账号.value) {
      s = ":802/admin_son"
    }
    return axios.post("http://" + window.location.hostname + s + 链接, 参数, { headers: { "Content-Type": "application/json" } });
  };



  return { post, 登录状态, 账号, 密码, 用户id, 导航开关, count, doubleCount, increment, api次数, 是子账号,账号信息 };
});
