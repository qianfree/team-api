<script setup lang="ts">
import { onErrorCaptured, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const error = ref<unknown>(null)

onErrorCaptured((caughtError, _instance, info) => {
  error.value = caughtError
  console.error('[RouteErrorBoundary]', info, caughtError)
  return false
})

// 边界不再随路由 :key 重建，需在路由切换时主动清除上一个页面的错误状态，
// 避免新页面继续展示错误兜底。
watch(() => route.fullPath, () => {
  error.value = null
})

function reloadPage() {
  window.location.reload()
}

async function goToDashboard() {
  error.value = null
  await router.push({ name: 'AdminDashboard' })
}
</script>

<template>
  <slot v-if="!error" />
  <div v-else class="route-error">
    <AResult status="error" title="页面加载失败" subtitle="页面数据异常，请重试或返回仪表盘">
      <template #extra>
        <ASpace>
          <AButton type="primary" @click="reloadPage">重新加载</AButton>
          <AButton @click="goToDashboard">返回仪表盘</AButton>
        </ASpace>
      </template>
    </AResult>
  </div>
</template>

<style scoped>
.route-error {
  display: flex;
  min-height: min(520px, calc(100vh - 160px));
  align-items: center;
  justify-content: center;
}
</style>
