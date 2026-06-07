<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { showToast, showConfirmDialog } from 'vant'
import request from '@/utils/request'

const loading = ref(false)
const plugins = ref<any[]>([])

const categoryFilter = ref('all')
const statusFilter = ref('all')

const categoryOptions = [
  { value: 'all', label: '全部' },
  { value: 'relay', label: '中继' },
  { value: 'middleware', label: '中间件' },
  { value: 'billing', label: '计费' },
  { value: 'notification', label: '通知' },
  { value: 'extension', label: '扩展' },
]

const statusOptions = [
  { value: 'all', label: '全部' },
  { value: 'registered', label: '已注册' },
  { value: 'installed', label: '已安装' },
  { value: 'enabled', label: '已启用' },
  { value: 'disabled', label: '已禁用' },
]

const stats = computed(() => {
  const list = plugins.value
  return {
    total: list.length,
    installed: list.filter(p => p.status === 'installed' || p.status === 'enabled').length,
    enabled: list.filter(p => p.status === 'enabled').length,
  }
})

const filteredPlugins = computed(() => {
  return plugins.value.filter(p => {
    if (categoryFilter.value !== 'all' && p.category !== categoryFilter.value) return false
    if (statusFilter.value !== 'all' && p.status !== statusFilter.value) return false
    return true
  })
})

async function fetchPlugins() {
  loading.value = true
  try {
    const { data: res } = await request.get('/admin/plugins')
    plugins.value = res.data?.list || []
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
  }
}

async function installPlugin(plugin: any) {
  try {
    await showConfirmDialog({
      title: '确认安装',
      message: `确定要安装插件「${plugin.label}」吗？`,
    })
    await request.post(`/admin/plugins/${plugin.name}/install`)
    plugin.status = 'installed'
    showToast('安装成功')
  } catch {
    // cancelled or error
  }
}

async function enablePlugin(plugin: any) {
  try {
    await request.post(`/admin/plugins/${plugin.name}/enable`)
    plugin.status = 'enabled'
    showToast('已启用')
  } catch {
    // handled by interceptor
  }
}

async function disablePlugin(plugin: any) {
  try {
    await request.post(`/admin/plugins/${plugin.name}/disable`)
    plugin.status = 'disabled'
    showToast('已禁用')
  } catch {
    // handled by interceptor
  }
}

async function uninstallPlugin(plugin: any) {
  try {
    await showConfirmDialog({
      title: '确认卸载',
      message: `确定要卸载插件「${plugin.label}」吗？卸载后需要重新安装。`,
    })
    await request.post(`/admin/plugins/${plugin.name}/uninstall`)
    plugin.status = 'registered'
    showToast('已卸载')
  } catch {
    // cancelled or error
  }
}

const categoryLabel: Record<string, string> = {
  relay: '中继',
  middleware: '中间件',
  billing: '计费',
  notification: '通知',
  extension: '扩展',
}

const statusConfig: Record<string, { label: string; color: string; bg: string }> = {
  registered: { label: '已注册', color: '#64748b', bg: 'rgba(100,116,139,0.1)' },
  installed: { label: '已安装', color: '#0d9488', bg: 'rgba(13,148,136,0.1)' },
  enabled: { label: '已启用', color: '#10b981', bg: 'rgba(16,185,129,0.1)' },
  disabled: { label: '已禁用', color: '#ef4444', bg: 'rgba(239,68,68,0.1)' },
  error: { label: '错误', color: '#f59e0b', bg: 'rgba(245,158,11,0.1)' },
}

function getStatusConf(status: string) {
  return statusConfig[status] || statusConfig.registered
}

onMounted(fetchPlugins)
</script>

<template>
  <div class="page">

    <!-- ═══════ HERO ═══════ -->
    <div class="hero">
      <div class="hero-bg">
        <div class="hero-orb hero-orb--1" />
        <div class="hero-orb hero-orb--2" />
      </div>

      <div class="hero-content">
        <div class="hero-greeting">
          <span class="hero-label">插件管理</span>
          <span class="hero-badge">
            <van-icon name="apps-o" size="14" />
          </span>
        </div>

        <div v-if="!loading" class="hero-metrics">
          <div class="hero-metric hero-metric--main">
            <div class="hero-metric__value">{{ stats.total }}</div>
            <div class="hero-metric__label">全部插件</div>
          </div>
          <div class="hero-metric">
            <div class="hero-metric__value">{{ stats.installed }}</div>
            <div class="hero-metric__label">已安装</div>
          </div>
          <div class="hero-metric">
            <div class="hero-metric__value">{{ stats.enabled }}</div>
            <div class="hero-metric__label">已启用</div>
          </div>
        </div>

        <!-- Skeleton -->
        <div v-else class="hero-skeleton">
          <div class="skeleton-bar skeleton-bar--lg" />
          <div class="skeleton-bar skeleton-bar--md" />
        </div>
      </div>
    </div>

    <!-- ═══════ FILTERS ═══════ -->
    <div class="filters">
      <div class="filter-row">
        <div class="filter-chips">
          <div
            v-for="opt in categoryOptions"
            :key="opt.value"
            class="chip"
            :class="{ 'chip--active': categoryFilter === opt.value }"
            @click="categoryFilter = opt.value"
          >
            {{ opt.label }}
          </div>
        </div>
      </div>
      <div class="filter-row">
        <div class="filter-chips">
          <div
            v-for="opt in statusOptions"
            :key="opt.value"
            class="chip chip--status"
            :class="{ 'chip--active-status': statusFilter === opt.value }"
            @click="statusFilter = opt.value"
          >
            {{ opt.label }}
          </div>
        </div>
      </div>
    </div>

    <!-- ═══════ PLUGIN CARDS ═══════ -->
    <div class="card-list">
      <div
        v-for="(plugin, idx) in filteredPlugins"
        :key="plugin.name"
        class="plugin-card"
        :style="{ animationDelay: `${Math.min(idx, 10) * 0.04}s` }"
      >
        <!-- Top: label + name -->
        <div class="plugin-card__header">
          <div class="plugin-card__title-group">
            <h4 class="plugin-card__label">{{ plugin.label }}</h4>
            <span class="plugin-card__name">{{ plugin.name }}</span>
          </div>
          <span
            class="plugin-card__status"
            :style="{ color: getStatusConf(plugin.status).color, background: getStatusConf(plugin.status).bg }"
          >
            {{ getStatusConf(plugin.status).label }}
          </span>
        </div>

        <!-- Description -->
        <p class="plugin-card__desc">{{ plugin.description }}</p>

        <!-- Badges row -->
        <div class="plugin-card__badges">
          <span v-if="plugin.version" class="badge badge--version">v{{ plugin.version }}</span>
          <span class="badge badge--category">{{ categoryLabel[plugin.category] || plugin.category }}</span>
        </div>

        <!-- Actions -->
        <div class="plugin-card__actions">
          <template v-if="plugin.status === 'registered'">
            <button class="action-btn action-btn--primary" @click="installPlugin(plugin)">安装</button>
          </template>
          <template v-else-if="plugin.status === 'installed'">
            <button class="action-btn action-btn--success" @click="enablePlugin(plugin)">启用</button>
          </template>
          <template v-else-if="plugin.status === 'enabled'">
            <button class="action-btn action-btn--danger" @click="disablePlugin(plugin)">禁用</button>
          </template>
          <template v-else-if="plugin.status === 'disabled'">
            <button class="action-btn action-btn--success" @click="enablePlugin(plugin)">启用</button>
            <button class="action-btn action-btn--warning" @click="uninstallPlugin(plugin)">卸载</button>
          </template>
        </div>
      </div>
    </div>

    <!-- ═══════ EMPTY STATE ═══════ -->
    <div v-if="!loading && filteredPlugins.length === 0" class="empty-state">
      <van-icon name="apps-o" size="40" color="#cbd5e1" />
      <span class="empty-text">{{ plugins.length ? '没有匹配的插件' : '暂无插件数据' }}</span>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="loading-wrap">
      <van-loading size="24" color="#8b5cf6" />
    </div>
  </div>
</template>

<style scoped>
.page {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(24px + env(safe-area-inset-bottom, 0px));
}

/* ═══════════════════════════════════════
   HERO — Purple Theme
   ═══════════════════════════════════════ */
.hero {
  position: relative;
  overflow: hidden;
  padding-bottom: 24px;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #2e1065 0%, #6d28d9 35%, #8b5cf6 65%, #7c3aed 100%);
}

.hero-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(60px);
  pointer-events: none;
}

.hero-orb--1 {
  width: 180px;
  height: 180px;
  background: rgba(167, 139, 250, 0.25);
  top: -30px;
  right: -20px;
}

.hero-orb--2 {
  width: 140px;
  height: 140px;
  background: rgba(124, 58, 237, 0.2);
  bottom: 10px;
  left: -30px;
}

.hero-content {
  position: relative;
  z-index: 1;
  padding: 24px 20px 0;
}

.hero-greeting {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 18px;
}

.hero-label {
  font-size: 20px;
  font-weight: 700;
  color: #fff;
  letter-spacing: -0.02em;
}

.hero-badge {
  width: 32px;
  height: 32px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.15);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.hero-metrics {
  display: flex;
  gap: 10px;
}

.hero-metric {
  flex: 1;
  background: rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 14px;
  padding: 14px;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.hero-metric--main {
  flex: 1.2;
  background: rgba(255, 255, 255, 0.14);
}

.hero-metric__value {
  font-size: 24px;
  font-weight: 700;
  color: #fff;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.03em;
  line-height: 1.2;
}

.hero-metric--main .hero-metric__value {
  font-size: 28px;
}

.hero-metric__label {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.55);
  margin-top: 4px;
  font-weight: 500;
}

/* Hero Skeleton */
.hero-skeleton {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 8px 0;
}

.skeleton-bar {
  height: 12px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.12);
}

.skeleton-bar--lg {
  width: 60%;
  height: 24px;
}

.skeleton-bar--md {
  width: 40%;
}

/* ═══════════════════════════════════════
   FILTERS
   ═══════════════════════════════════════ */
.filters {
  padding: 16px 16px 0;
  animation: fadeSlideUp 0.4s 0.1s both;
}

.filter-row {
  margin-bottom: 8px;
}

.filter-chips {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 4px;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.filter-chips::-webkit-scrollbar {
  display: none;
}

.chip {
  flex-shrink: 0;
  padding: 6px 14px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 600;
  color: var(--ta-text-secondary, #475569);
  background: var(--ta-bg-card, #fff);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
  cursor: pointer;
  transition: all 0.2s;
  -webkit-tap-highlight-color: transparent;
  white-space: nowrap;
}

.chip--active {
  background: #0d9488;
  color: #fff;
  box-shadow: 0 2px 8px rgba(13, 148, 136, 0.3);
}

.chip--status.chip--active-status {
  background: #8b5cf6;
  color: #fff;
  box-shadow: 0 2px 8px rgba(139, 92, 246, 0.3);
}

.chip:active {
  transform: scale(0.96);
}

/* ═══════════════════════════════════════
   CARD LIST
   ═══════════════════════════════════════ */
.card-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 16px 0;
}

.plugin-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 16px;
  padding: 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  animation: cardIn 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
  transition: transform 0.15s;
}

.plugin-card:active {
  transform: scale(0.99);
}

@keyframes cardIn {
  from {
    opacity: 0;
    transform: translateY(12px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Card Header */
.plugin-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
}

.plugin-card__title-group {
  flex: 1;
  min-width: 0;
}

.plugin-card__label {
  font-size: 15px;
  font-weight: 700;
  color: var(--ta-text-primary, #0f172a);
  margin: 0;
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-card__name {
  font-size: 11px;
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace;
  color: var(--ta-text-tertiary, #94a3b8);
  margin-top: 2px;
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Status Badge */
.plugin-card__status {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  padding: 3px 10px;
  border-radius: 10px;
  line-height: 1.4;
  white-space: nowrap;
}

/* Description */
.plugin-card__desc {
  font-size: 12px;
  color: var(--ta-text-secondary, #64748b);
  line-height: 1.6;
  margin: 0 0 10px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

/* Badges */
.plugin-card__badges {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
}

.badge {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 6px;
  line-height: 1.4;
  white-space: nowrap;
}

.badge--version {
  color: #6366f1;
  background: rgba(99, 102, 241, 0.1);
}

.badge--category {
  color: #0d9488;
  background: rgba(13, 148, 136, 0.1);
}

/* Actions */
.plugin-card__actions {
  display: flex;
  gap: 8px;
  padding-top: 12px;
  border-top: 1px solid var(--ta-bg-secondary, #f1f5f9);
}

.action-btn {
  flex: 1;
  padding: 8px 0;
  border: none;
  border-radius: 10px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  -webkit-tap-highlight-color: transparent;
}

.action-btn:active {
  transform: scale(0.97);
}

.action-btn--primary {
  background: rgba(13, 148, 136, 0.1);
  color: #0d9488;
}

.action-btn--primary:active {
  background: rgba(13, 148, 136, 0.18);
}

.action-btn--success {
  background: rgba(16, 185, 129, 0.1);
  color: #10b981;
}

.action-btn--success:active {
  background: rgba(16, 185, 129, 0.18);
}

.action-btn--danger {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}

.action-btn--danger:active {
  background: rgba(239, 68, 68, 0.18);
}

.action-btn--warning {
  background: rgba(245, 158, 11, 0.1);
  color: #f59e0b;
}

.action-btn--warning:active {
  background: rgba(245, 158, 11, 0.18);
}

/* ═══════════════════════════════════════
   EMPTY STATE
   ═══════════════════════════════════════ */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 56px 0 24px;
  animation: fadeSlideUp 0.4s 0.2s both;
}

.empty-text {
  font-size: 13px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
}

/* ═══════════════════════════════════════
   LOADING
   ═══════════════════════════════════════ */
.loading-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 0;
}

/* ═══════════════════════════════════════
   ANIMATIONS
   ═══════════════════════════════════════ */
@keyframes fadeSlideUp {
  from {
    opacity: 0;
    transform: translateY(14px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
