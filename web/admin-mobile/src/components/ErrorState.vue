<script setup lang="ts">
defineProps<{
  message?: string
}>()

const emit = defineEmits<{
  retry: []
}>()
</script>

<template>
  <div class="flex flex-col items-center justify-center py-16 px-8">
    <van-icon name="warning-o" size="48" color="#ef4444" />
    <p class="mt-3 text-sm text-gray-500">{{ message || '加载失败' }}</p>
    <van-button
      type="primary"
      size="small"
      round
      class="mt-4"
      @click="emit('retry')"
    >
      重试
    </van-button>
  </div>
</template>
