<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ score: number }>()

const color = computed(() => {
  if (props.score >= 80) return '#10b981'
  if (props.score >= 50) return '#f59e0b'
  return '#ef4444'
})

const level = computed(() => {
  if (props.score >= 80) return '健康'
  if (props.score >= 50) return '一般'
  return '异常'
})
</script>

<template>
  <div class="flex items-center gap-1">
    <span
      class="inline-block w-2 h-2 rounded-full"
      :style="{ backgroundColor: color }"
    />
    <span class="text-xs" :style="{ color }">{{ score }} {{ level }}</span>
  </div>
</template>
