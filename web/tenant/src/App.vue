<template>
  <TopProgress />
  <router-view />
  <Toast />
  <ConfirmDialog />
</template>

<script setup lang="ts">
import TopProgress from '@/components/common/TopProgress.vue'
import Toast from '@/components/common/Toast.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
</script>
