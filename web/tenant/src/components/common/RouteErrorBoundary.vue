<script setup lang="ts">
import { onErrorCaptured, ref } from 'vue'
import { useRouter } from 'vue-router'
import Icon from '@/components/common/Icon.vue'

const router = useRouter()
const error = ref<unknown>(null)

onErrorCaptured((caughtError, _instance, info) => {
  error.value = caughtError
  console.error('[RouteErrorBoundary]', info, caughtError)
  return false
})

function reloadPage() {
  window.location.reload()
}

async function goToDashboard() {
  error.value = null
  await router.push({ name: 'TenantDashboard' })
}
</script>

<template>
  <slot v-if="!error" />
  <div v-else class="flex min-h-[min(520px,calc(100vh-10rem))] items-center justify-center px-4">
    <div class="w-full max-w-lg text-center">
      <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-red-50 text-red-500">
        <Icon name="exclamationCircle" size="xl" />
      </div>
      <h2 class="mt-5 text-lg font-semibold text-gray-900">页面加载失败</h2>
      <p class="mt-2 text-sm text-gray-500">页面数据异常，请重试或返回仪表盘</p>
      <div class="mt-6 flex flex-wrap justify-center gap-3">
        <button type="button" class="btn btn-primary" @click="reloadPage">
          <Icon name="refresh" size="sm" />
          重新加载
        </button>
        <button type="button" class="btn btn-secondary" @click="goToDashboard">
          <Icon name="chart" size="sm" />
          返回仪表盘
        </button>
      </div>
    </div>
  </div>
</template>
