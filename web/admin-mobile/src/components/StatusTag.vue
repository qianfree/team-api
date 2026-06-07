<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ status: string }>()

const colorMap: Record<string, string> = {
  active: 'success',
  enabled: 'success',
  disabled: 'danger',
  testing: 'primary',
  suspended: 'warning',
  closed: 'danger',
  pending: 'warning',
}

const labelMap: Record<string, string> = {
  active: '启用',
  enabled: '启用',
  disabled: '禁用',
  testing: '测试中',
  suspended: '已暂停',
  closed: '已关闭',
  pending: '待处理',
}

const type = computed(() => (colorMap[props.status] || 'default') as 'success' | 'danger' | 'primary' | 'warning' | 'default')
const label = computed(() => labelMap[props.status] || props.status)
</script>

<template>
  <van-tag :type="type" size="medium" round>{{ label }}</van-tag>
</template>
