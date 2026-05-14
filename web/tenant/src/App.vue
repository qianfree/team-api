<template>
  <router-view />
  <Toast />
  <ConfirmDialog />
</template>

<script setup lang="ts">
import Toast from '@/components/common/Toast.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
</script>
