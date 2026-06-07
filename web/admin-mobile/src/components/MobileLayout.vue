<script setup lang="ts">
import { computed, ref, onMounted, provide } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { showToast } from 'vant'
import axios from 'axios'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const demoMode = ref(false)
provide('demoMode', demoMode)

const activeTab = computed(() => {
  const tab = route.meta?.tab as string | undefined
  if (tab === 'models') return 1
  if (tab === 'ops') return 2
  if (tab === 'profile') return 3
  return 0 // status
})

const showTabBar = computed(() => !route.meta?.hideTabBar)
const pageTitle = computed(() => (route.meta?.title as string) || '')

function onTabChange(index: number) {
  const routes = ['/m/', '/m/models', '/m/ops', '/m/profile']
  if (route.path !== routes[index]) {
    router.replace(routes[index])
  }
}

onMounted(() => {
  axios.get('/api/settings/public').then((res) => {
    const settings = res.data?.data?.settings
    if (settings) demoMode.value = !!settings.demo_mode
  }).catch(() => {})
})
</script>

<template>
  <div class="mobile-layout">
    <!-- Top Navbar (shown on subpages with back button) -->
    <van-nav-bar
      v-if="!showTabBar && pageTitle"
      :title="pageTitle"
      left-arrow
      fixed
      placeholder
      @click-left="router.back()"
    />

    <!-- Content Area -->
    <main class="mobile-content" :class="{ 'has-tabbar': showTabBar }">
      <router-view v-slot="{ Component }">
        <Transition name="slide-right" mode="out-in">
          <component :is="Component" />
        </Transition>
      </router-view>
    </main>

    <!-- Bottom Tab Bar -->
    <van-tabbar
      v-if="showTabBar"
      :model-value="activeTab"
      @change="onTabChange"
      fixed
      placeholder
      :safe-area-inset-bottom="true"
    >
      <van-tabbar-item icon="chart-trending-o">状态</van-tabbar-item>
      <van-tabbar-item icon="apps-o">模型</van-tabbar-item>
      <van-tabbar-item icon="setting-o">运营</van-tabbar-item>
      <van-tabbar-item icon="contact-o">我的</van-tabbar-item>
    </van-tabbar>
  </div>
</template>

<style scoped>
.mobile-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--ta-bg-page);
}

.mobile-content {
  flex: 1;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}
</style>
