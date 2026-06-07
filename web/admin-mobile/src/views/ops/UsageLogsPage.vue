<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { showToast } from 'vant'
import request from '@/utils/request'

const loading = ref(false)
const refreshing = ref(false)
const finished = ref(false)
const logs = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const search = ref('')

const statusFilters = [
  { key: '', label: '全部' },
  { key: 'success', label: '成功' },
  { key: 'error', label: '错误' },
  { key: 'timeout', label: '超时' },
]
const activeStatus = ref('')

// Hero stats
const todayCount = ref(0)
const todayCost = ref(0)

async function fetchUsageLogs(append = false) {
  if (!append) {
    page.value = 1
    finished.value = false
  }

  loading.value = true
  try {
    const params: any = {
      page: page.value,
      page_size: 20,
    }
    if (search.value) params.keyword = search.value
    if (activeStatus.value) params.status = activeStatus.value

    const { data: res } = await request.get('/admin/usage-logs', { params })
    const list = res.data?.list || []
    total.value = res.data?.total || 0

    if (append) {
      logs.value = [...logs.value, ...list]
    } else {
      logs.value = list
    }
    finished.value = logs.value.length >= total.value
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function fetchTodayStats() {
  try {
    const today = new Date().toISOString().slice(0, 10)
    const { data: res } = await request.get('/admin/usage-logs', {
      params: { page: 1, page_size: 1, start_date: today, end_date: today },
    })
    todayCount.value = res.data?.total || 0
    // Total cost for today — if API returns it at top level
    if (res.data?.today_cost !== undefined) {
      todayCost.value = res.data.today_cost
    }
  } catch {
    // non-critical
  }
}

async function onRefresh() {
  refreshing.value = true
  await Promise.all([fetchUsageLogs(false), fetchTodayStats()])
}

async function onLoad() {
  if (finished.value) return
  page.value++
  await fetchUsageLogs(true)
}

function onSearch(val: string) {
  search.value = val
  fetchUsageLogs(false)
}

function setStatus(key: string) {
  activeStatus.value = key
  fetchUsageLogs(false)
}

function formatCost(n: number | undefined): string {
  if (n === undefined || n === null) return '-'
  return `$${n.toFixed(6)}`
}

function formatCostCompact(n: number | undefined): string {
  if (n === undefined || n === null) return '$0.00'
  if (n >= 1) return `$${n.toFixed(2)}`
  if (n >= 0.01) return `$${n.toFixed(4)}`
  return `$${n.toFixed(6)}`
}

function formatTokens(n: number | undefined): string {
  if (!n) return '0'
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

function latencyColor(ms: number | undefined): string {
  if (ms === undefined || ms === null) return '#94a3b8'
  if (ms < 500) return '#10b981'
  if (ms < 2000) return '#f59e0b'
  return '#ef4444'
}

function latencyBg(ms: number | undefined): string {
  if (ms === undefined || ms === null) return 'rgba(148,163,184,0.08)'
  if (ms < 500) return 'rgba(16,185,129,0.08)'
  if (ms < 2000) return 'rgba(245,158,11,0.08)'
  return 'rgba(239,68,68,0.08)'
}

function statusLabel(status: string | undefined): string {
  if (!status) return '-'
  if (status === 'success') return '成功'
  if (status === 'error') return '错误'
  if (status === 'timeout') return '超时'
  return status
}

function statusColor(status: string | undefined): string {
  if (status === 'success') return '#10b981'
  if (status === 'error') return '#ef4444'
  if (status === 'timeout') return '#f59e0b'
  return '#94a3b8'
}

function statusBg(status: string | undefined): string {
  if (status === 'success') return 'rgba(16,185,129,0.1)'
  if (status === 'error') return 'rgba(239,68,68,0.08)'
  if (status === 'timeout') return 'rgba(245,158,11,0.08)'
  return 'rgba(148,163,184,0.08)'
}

function formatTime(dateStr: string | undefined): string {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const hour = String(d.getHours()).padStart(2, '0')
  const min = String(d.getMinutes()).padStart(2, '0')
  const sec = String(d.getSeconds()).padStart(2, '0')
  return `${month}-${day} ${hour}:${min}:${sec}`
}

function formatCount(n: number | undefined): string {
  if (n === undefined || n === null) return '0'
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toLocaleString()
}

onMounted(() => {
  fetchTodayStats()
  fetchUsageLogs()
})
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
        <div class="hero-label">使用日志</div>
        <div class="hero-metrics">
          <div class="hero-metric hero-metric--main">
            <div class="hero-metric__value">{{ formatCount(todayCount) }}</div>
            <div class="hero-metric__label">今日请求</div>
          </div>
          <div class="hero-metric">
            <div class="hero-metric__value">{{ formatCostCompact(todayCost) }}</div>
            <div class="hero-metric__label">今日费用</div>
          </div>
        </div>
      </div>
    </div>

    <!-- ═══════ CONTENT ═══════ -->
    <div class="content">

      <!-- Search -->
      <div class="search-wrap">
        <van-search
          v-model="search"
          placeholder="搜索租户或模型..."
          shape="round"
          @search="onSearch"
          @clear="onSearch('')"
        />
      </div>

      <!-- Status filter chips -->
      <div class="filter-scroll">
        <div
          v-for="f in statusFilters"
          :key="f.key"
          class="filter-chip"
          :class="{ 'filter-chip--active': activeStatus === f.key }"
          @click="setStatus(f.key)"
        >
          {{ f.label }}
        </div>
      </div>

      <!-- Count -->
      <div v-if="total > 0" class="count-bar">
        <span class="count-text">共 <b>{{ total }}</b> 条记录</span>
      </div>

      <!-- Log Cards -->
      <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
        <van-list v-model:loading="loading" :finished="finished" finished-text="" @load="onLoad">
          <div class="card-list">
            <div
              v-for="(item, idx) in logs"
              :key="item.id"
              class="log-card"
              :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
            >
              <!-- Top: tenant + status badge -->
              <div class="log-card__top">
                <span class="log-card__tenant">{{ item.tenant_name || '-' }}</span>
                <span
                  class="log-card__status"
                  :style="{ color: statusColor(item.status), background: statusBg(item.status) }"
                >
                  {{ statusLabel(item.status) }}
                </span>
              </div>

              <!-- Model name (monospace) -->
              <div class="log-card__model">{{ item.model_name || '-' }}</div>

              <!-- Token stats -->
              <div class="log-card__tokens">
                <div class="token-item">
                  <span class="token-label">输入</span>
                  <span class="token-val">{{ formatTokens(item.input_tokens) }}</span>
                </div>
                <div class="token-divider" />
                <div class="token-item">
                  <span class="token-label">输出</span>
                  <span class="token-val">{{ formatTokens(item.output_tokens) }}</span>
                </div>
                <div class="token-divider" />
                <div class="token-item">
                  <span class="token-label">费用</span>
                  <span class="token-val token-val--cost">{{ formatCost(item.total_cost) }}</span>
                </div>
              </div>

              <!-- Bottom: latency + time -->
              <div class="log-card__bottom">
                <span
                  class="log-card__latency"
                  :style="{ color: latencyColor(item.latency_ms), background: latencyBg(item.latency_ms) }"
                >
                  {{ item.latency_ms !== undefined ? `${item.latency_ms}ms` : '-' }}
                </span>
                <span class="log-card__time">{{ formatTime(item.created_at) }}</span>
              </div>
            </div>
          </div>

          <div v-if="!loading && !logs.length" class="empty-state">
            <van-icon name="records" size="40" color="#cbd5e1" />
            <span>暂无使用日志</span>
          </div>
        </van-list>
      </van-pull-refresh>
    </div>
  </div>
</template>

<style scoped>
.page {
  min-height: 100vh;
  background: #f5f7fa;
}

/* ═══════════════════════════════════════
   HERO — Cyan Gradient
   ═══════════════════════════════════════ */
.hero {
  position: relative;
  overflow: hidden;
  padding-bottom: 20px;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #0e4f5c 0%, #0891b2 40%, #06b6d4 70%, #155e75 100%);
}

.hero-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(60px);
  pointer-events: none;
}
.hero-orb--1 {
  width: 200px;
  height: 200px;
  background: rgba(103, 232, 249, 0.2);
  top: -40px;
  right: -30px;
}
.hero-orb--2 {
  width: 160px;
  height: 160px;
  background: rgba(6, 182, 212, 0.15);
  bottom: 10px;
  left: -20px;
}

.hero-content {
  position: relative;
  z-index: 1;
  padding: 24px 20px 0;
}

.hero-label {
  font-size: 20px;
  font-weight: 700;
  color: #fff;
  letter-spacing: -0.02em;
  margin-bottom: 16px;
  display: block;
}

.hero-metrics {
  display: flex;
  gap: 12px;
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

/* ═══════════════════════════════════════
   CONTENT
   ═══════════════════════════════════════ */
.content {
  padding: 0 0 24px;
  margin-top: -8px;
  position: relative;
  z-index: 2;
}

/* ── Search ── */
.search-wrap {
  padding: 4px 0 0;
}
.search-wrap :deep(.van-search) {
  padding: 8px 12px;
}

/* ── Status filter chips ── */
.filter-scroll {
  display: flex;
  gap: 8px;
  padding: 4px 16px 8px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}
.filter-scroll::-webkit-scrollbar {
  display: none;
}

.filter-chip {
  flex-shrink: 0;
  padding: 5px 14px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
  color: #64748b;
  background: #f1f5f9;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}
.filter-chip--active {
  color: #fff;
  background: #06b6d4;
  box-shadow: 0 2px 8px rgba(6, 182, 212, 0.3);
}

/* ── Count ── */
.count-bar {
  padding: 4px 16px 6px;
}
.count-text {
  font-size: 12px;
  color: #94a3b8;
}
.count-text b {
  color: #475569;
  font-weight: 600;
}

/* ── Card List ── */
.card-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 12px;
}

.log-card {
  background: #fff;
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  animation: cardIn 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
  transition: transform 0.15s;
}
.log-card:active {
  transform: scale(0.99);
}

@keyframes cardIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* ── Card: Top row ── */
.log-card__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}

.log-card__tenant {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.log-card__status {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  line-height: 1.4;
}

/* ── Card: Model name ── */
.log-card__model {
  font-size: 12px;
  font-weight: 500;
  color: #64748b;
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  letter-spacing: -0.02em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-bottom: 10px;
}

/* ── Card: Token stats ── */
.log-card__tokens {
  display: flex;
  align-items: center;
  background: #f8fafc;
  border-radius: 8px;
  padding: 8px 12px;
  margin-bottom: 10px;
}

.token-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}

.token-label {
  font-size: 10px;
  color: #94a3b8;
  font-weight: 500;
}

.token-val {
  font-size: 13px;
  font-weight: 700;
  color: #1e293b;
  font-variant-numeric: tabular-nums;
}
.token-val--cost {
  color: #0d9488;
}

.token-divider {
  width: 1px;
  height: 24px;
  background: #e2e8f0;
  margin: 0 10px;
}

/* ── Card: Bottom row ── */
.log-card__bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.log-card__latency {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 6px;
  font-variant-numeric: tabular-nums;
}

.log-card__time {
  font-size: 11px;
  color: #94a3b8;
  font-weight: 400;
}

/* ── Empty state ── */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 48px 0 24px;
  color: #94a3b8;
  font-size: 13px;
}
</style>
