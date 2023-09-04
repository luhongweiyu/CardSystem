

<template>
    <div class="backimg"></div>
    <el-container style="z-index: 9999">
        <el-main style="z-index: 9999">
            <div v-loading="加载中">
                <el-input v-model="input1" placeholder="请输入卡密">
                    <template #prepend>卡密:</template>
                </el-input>
                <el-button type="primary" :icon="Search" @click="查询详情()"> 查询 </el-button>
            </div>
            <pre>
                {{ 单个卡密详情 }}
            </pre>
        </el-main>
        <el-footer class="el-footer" style="z-index: 9999"><el-button type="warning" plain @click="显示关于 = true" >关于</el-button></el-footer>
    </el-container>
    <el-dialog v-model="显示关于" title="关于" width="80%" style="z-index: 9999">
        <About></About>
    </el-dialog>
</template>
<script setup>
import axios from "axios";
import { ref, reactive, computed } from "vue";



import {
    Check,
    Delete,
    Edit,
    Message,
    Search,
    Star,
} from '@element-plus/icons-vue'
import { ElMessage } from "element-plus";
import About from "./views/About.vue";
const input1 = ref("")
const 单个卡密详情 = ref("")
const 加载中 = ref(false)
const 显示关于 = ref(false)
function getUrlSearch(name) {
    // 未传参，返回空
    if (!name) return null;
    // 查询参数：先通过search取值，如果取不到就通过hash来取
    var after = window.location.search;
    after = after.substr(1) || window.location.hash.split('?')[1];
    // 地址栏URL没有查询参数，返回空
    if (!after) return null;
    // 如果查询参数中没有"name"，返回空
    if (after.indexOf(name) === -1) return null;

    var reg = new RegExp('(^|&)' + name + '=([^&]*)(&|$)');
    // 当地址栏参数存在中文时，需要解码，不然会乱码
    var r = decodeURI(after).match(reg);
    // 如果url中"name"没有值，返回空
    if (!r) return null;

    return r[2];
}
const 查询详情 = function () {
    单个卡密详情.value = ""
    let center_id = getUrlSearch("center_id")
    if (!center_id) {
        ElMessage.error("center_id错误.网页地址可能错误,请联系管理员获取")
        return
    }
    加载中.value = true
    axios.post("http://" + window.location.hostname + ":802/card/query?center_id=" + center_id + "&card=" + input1.value).then(function (res) {
        加载中.value = false
        console.log(res.data)
        if (res.data.state == true) {
            单个卡密详情.value = "\n" + res.data.data

        } else {
            ElMessage.error(res.data.msg)
        }

    })
}

</script>


<style >
html,
body {
    width: 100%;
    height: 100%;
    color: #fff;
    background-image: url('https://t.mwm.moe/pc');
    background-size: cover;
}

#app {
    top: 0px;
    padding: 50px;
    margin: 0px;
    width: 100%;
    height: 100%;
}

container,
el-main {
    width: 100%;
    height: 100%;

}

.backimg {
    top: 0;
    left: 0;
    position: absolute;
    margin: 0;
    padding: 0;
    width: 100%;
    height: 100%;
    background-color: rgb(22, 22, 22, 0.9);
    z-index: 0;
    pointer-events: none;

}



.el-button {
    margin: 50px;
}

.el-footer {
    height: 50px;
}
</style>
