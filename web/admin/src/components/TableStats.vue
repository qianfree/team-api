<script setup lang="ts">
/**
 * 表格左上角统计栏：统一展示「共 N 条记录」。
 * 通过 default slot 可追加额外统计项或筛选控件（带分隔符）。
 * 样式由全局 main.css 的 .table-stats 提供，无需 scoped。
 */
defineProps<{
  total?: number | null
  label?: string
}>()
</script>

<template>
  <div class="table-stats">
    <span>{{ label || '共' }}</span>
    <strong class="table-stats__value">{{ total ?? 0 }}</strong>
    <span>条</span>
    <template v-if="$slots.default">
      <span class="table-stats__divider" />
      <slot />
    </template>
  </div>
</template>
