<script setup lang="ts">
defineProps<{
  label: string
  value: string | number
  trend?: { value: number; positive: boolean }
  icon?: string
}>()
</script>

<template>
  <div class="bg-white rounded-xl p-3 shadow-sm">
    <div class="text-xs text-gray-500 mb-1">{{ label }}</div>
    <div class="text-lg font-bold text-gray-900">{{ value }}</div>
    <div
      v-if="trend"
      class="text-xs mt-1 flex items-center gap-0.5"
      :class="trend.positive ? 'text-emerald-500' : 'text-red-500'"
    >
      <span>{{ trend.positive ? '↑' : '↓' }}</span>
      <span>{{ Math.abs(trend.value) }}%</span>
    </div>
  </div>
</template>
