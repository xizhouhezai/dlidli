<script setup lang="ts">
// 主布局：全局头部 + 内容区。业务页面均为本布局的子路由；
// 登录/找回密码等全屏页为顶级路由，不经过本布局。
import { RouterView } from 'vue-router'
import AppHeader from '@/components/layout/AppHeader.vue'
</script>

<template>
  <AppHeader />
  <main class="mx-auto max-w-1440px px-6 py-5">
    <RouterView />
  </main>
</template>
