<script setup lang="ts">
defineProps<{
  title: string
  description?: string
}>()
</script>

<template>
  <div class="page-header">
    <div>
      <h1 class="page-header__title">{{ title }}</h1>
      <p v-if="description" class="page-header__desc">{{ description }}</p>
    </div>
    <div class="page-header__actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 20px;
}

.page-header__title {
  font-size: 20px;
  font-weight: 700;
  color: var(--ta-text-primary);
  margin-bottom: 4px;
}

.page-header__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.page-header__desc {
  font-size: 14px;
  color: var(--ta-text-tertiary);
}

/* 移动端：标题与操作区纵向堆叠，操作按钮自动换行，避免横向溢出 */
@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
  }
  .page-header__actions {
    width: 100%;
    flex-wrap: wrap;
  }
}
</style>
