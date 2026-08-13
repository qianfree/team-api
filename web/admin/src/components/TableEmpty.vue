<script setup lang="ts">
/**
 * 表格空状态：无数据时在表格中央显示插画 + 提示文字。
 * 通过 240px 最小高度避免空表高度塌陷为 0，并保证内容垂直水平居中。
 *
 * 作为两种场景的默认空态：
 *  1. ResponsiveTable 桌面/移动端 #empty 插槽兜底
 *  2. App.vue 里 AConfigProvider 的全局 empty 插槽（覆盖所有直接使用 a-table 的页面）
 *
 * 用法：
 *   <TableEmpty description="暂无数据" tip="可尝试调整筛选条件" />
 *   或传入 #action 插槽放置「新建/重置」按钮
 */
withDefaults(
  defineProps<{
    description?: string
    tip?: string
  }>(),
  {
    description: '暂无数据',
    tip: '',
  },
)
</script>

<template>
  <div class="table-empty">
    <img
      src="/empty-state.svg"
      alt=""
      aria-hidden="true"
      class="table-empty__illustration"
    />
    <p class="table-empty__description">{{ description }}</p>
    <p v-if="tip" class="table-empty__tip">{{ tip }}</p>
    <slot name="action" />
  </div>
</template>

<style scoped>
.table-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  width: 100%;
  min-height: 240px;
  padding: 24px 16px;
}

.table-empty__illustration {
  width: 136px;
  max-width: 46vw;
  height: auto;
  margin-bottom: 16px;
  animation: table-empty-bob 3s ease-in-out infinite;
}

@keyframes table-empty-bob {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-4px); }
}

@media (prefers-reduced-motion: reduce) {
  .table-empty__illustration {
    animation: none;
  }
}

.table-empty__description {
  margin: 0;
  font-size: 14px;
  line-height: 1.5;
  color: var(--color-text-2);
}

.table-empty__tip {
  margin: 4px 0 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--color-text-3);
}
</style>
