<script setup lang="ts">
import TopProgress from '@/components/TopProgress.vue'
import TableEmpty from '@/components/TableEmpty.vue'
</script>

<template>
  <TopProgress />
  <AConfigProvider>
    <!-- 全局空态：表格（组件 tab 传 component="table"）统一显示插画 + 提示文字；
         其余场景（非 table 的裸 AEmpty）回退 Arco 默认，避免影响下拉框等小空态 -->
    <template #empty="{ component }">
      <TableEmpty v-if="component === 'table'" />
      <AEmpty v-else in-config-provider />
    </template>
    <router-view />
  </AConfigProvider>
</template>
