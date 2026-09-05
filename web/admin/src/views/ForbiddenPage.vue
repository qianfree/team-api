<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { hasPermission } from '@/utils/permission'

const router = useRouter()
const auth = useAuthStore()

// 未分配任何角色的账号权限为空集，这是安全的默认值而非故障。
// 它与「有角色但缺某个权限」的提示应当不同，否则用户会以为是系统出错。
const hasNoRole = computed(
  () => !auth.isSuperAdmin && (auth.roles?.length ?? 0) === 0
)

const subtitle = computed(() => {
  if (hasNoRole.value) {
    return '你的账号尚未分配角色，请联系超级管理员分配后再访问。'
  }
  return '你的角色没有访问该页面的权限，如需使用请联系管理员调整角色权限。'
})

// 仪表盘对预置角色都可见；若某个自定义角色连它都没有，就退回个人信息页
const fallbackRoute = computed(() =>
  hasPermission('dashboard:view') ? 'AdminDashboard' : 'AdminProfile'
)

function goBack() {
  if (window.history.length > 1) {
    router.back()
    return
  }
  router.push({ name: fallbackRoute.value })
}

function goHome() {
  router.push({ name: fallbackRoute.value })
}
</script>

<template>
  <div class="forbidden-page">
    <AResult status="403" title="无权访问" :subtitle="subtitle">
      <template #extra>
        <ASpace>
          <AButton type="primary" @click="goHome">返回首页</AButton>
          <AButton @click="goBack">返回上一页</AButton>
        </ASpace>
      </template>
    </AResult>
  </div>
</template>

<style scoped>
.forbidden-page {
  display: flex;
  min-height: min(520px, calc(100vh - 160px));
  align-items: center;
  justify-content: center;
}
</style>
