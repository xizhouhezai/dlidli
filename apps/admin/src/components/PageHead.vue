<script setup lang="ts">
// 后台页面通用标题栏：标题 + 副标题 + 右侧操作插槽（配合全局 .page-head 样式）
defineProps<{ title: string, sub?: string }>()
</script>

<template>
  <div class="page-head">
    <div>
      <h2 class="page-head__title">
        {{ title }}
      </h2>
      <p
        v-if="sub"
        class="page-head__sub"
      >
        {{ sub }}
      </p>
    </div>
    <slot name="actions" />
  </div>
</template>
