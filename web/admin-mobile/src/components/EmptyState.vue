<script setup lang="ts">
defineProps<{
  description?: string
}>()

const emit = defineEmits<{
  retry: []
}>()
</script>

<template>
  <div class="flex flex-col items-center justify-center py-16 px-8">
    <van-icon name="info-o" size="48" color="#94a3b8" />
    <p class="mt-3 text-sm text-gray-400">{{ description || '暂无数据' }}</p>
  </div>
</template>
