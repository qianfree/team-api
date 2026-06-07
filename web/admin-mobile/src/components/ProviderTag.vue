<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ type: number }>()

const providerMap: Record<number, { name: string; color: string }> = {
  1: { name: 'OpenAI', color: '#10a37f' },
  2: { name: 'Azure', color: '#0078d4' },
  3: { name: 'Claude', color: '#d97706' },
  4: { name: 'Gemini', color: '#4285f4' },
  5: { name: '百度', color: '#2932e1' },
  6: { name: '阿里', color: '#ff6a00' },
  7: { name: '字节', color: '#3370ff' },
  8: { name: '智谱', color: '#4f46e5' },
  9: { name: '月之暗面', color: '#1a1a2e' },
  10: { name: 'MiniMax', color: '#6366f1' },
  11: { name: 'Groq', color: '#f55036' },
  14: { name: 'Cohere', color: '#39d353' },
  15: { name: 'Stability', color: '#7c3aed' },
  16: { name: 'DeepSeek', color: '#4f46e5' },
  24: { name: 'Mistral', color: '#f97316' },
}

const info = computed(() => providerMap[props.type] || { name: `类型${props.type}`, color: '#64748b' })
</script>

<template>
  <van-tag plain size="medium" :style="{ color: info.color, borderColor: info.color }">
    {{ info.name }}
  </van-tag>
</template>
