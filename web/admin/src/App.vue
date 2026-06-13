<script setup lang="ts">
import TopProgress from '@/components/TopProgress.vue'
</script>

<template>
  <TopProgress />
  <AConfigProvider>
    <router-view />
  </AConfigProvider>
</template>
