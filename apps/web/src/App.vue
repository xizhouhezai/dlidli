<script setup lang="ts">
// 应用入口：只负责全局初始化（登录态资料拉取）与路由出口。
// 头部/内容布局见 layouts/MainLayout.vue，登录等全屏页为独立顶级路由。
import { onMounted } from 'vue'
import { RouterView } from 'vue-router'
import { useUserStore } from '@/stores/user'

const userStore = useUserStore()

onMounted(() => {
  userStore.fetchProfile()
})
</script>

<template>
  <RouterView />
</template>
