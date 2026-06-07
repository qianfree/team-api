<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import request from '@/utils/request'

const loading = ref(false)
const refreshing = ref(false)
const finished = ref(false)
const records = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const search = ref('')
const activeStatus = ref('')
const totalCost = ref(0)

const statusFilters = [
  { key: '', label: '全部' },
  { key: 'pre_deducted', label: '预扣' },
  { key: 'settled', label: '已结算' },
  { key: 'refunded', label: '已退款' },
]

const statusMap: Record<string, { label: string; color: string; bg: string }> = {
  pre_deducted: { label: '预扣', color: '#3b82f6', bg: 'rgba(59,130,246,0.1)' },
  settled: { label: '已结算', color: '#10b981', bg: 'rgba(16,185,129,0.1)' },
  refunded: { label: '已退款', color: '#f59e0b', bg: 'rgba(245,158,11,0.1)' },
}

function formatTokens(n: number | undefined): string {
  if (!n && n !== 0) return '-'
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`
  return String(n)
}

function formatCost(n: number | undefined): string {
  if (!n && n !== 0) return '$0.000000'
  return `$${n.toFixed(6)}`
}

function formatTime(dateStr: string): string {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  const pad = (v: number) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

async function fetchBilling(append = false) {
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

    const { data: res } = await request.get('/admin/billing-records', { params })
    const list = res.data?.list || []
    total.value = res.data?.total || 0

    // Calculate total cost from records for hero display
    if (!append && res.data?.total_cost !== undefined) {
      totalCost.value = res.data.total_cost
    } else if (!append) {
      totalCost.value = 0
    }

    if (append) {
      records.value = [...records.value, ...list]
    } else {
      records.value = list
    }
    finished.value = records.value.length >= total.value
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function onRefresh() {
  refreshing.value = true
  await fetchBilling(false)
}

async function onLoad() {
  if (finished.value) return
  page.value++
  await fetchBilling(true)
}

function onSearch(val: string) {
  search.value = val
  fetchBilling(false)
}

function setStatus(key: string) {
  activeStatus.value = key
  fetchBilling(false)
}

onMounted(() => fetchBilling())
</script>

<template>
  <div class="billing-page">

    <!-- ═══════ HERO ═══════ -->
    <div class="hero">
      <div class="hero-bg">
        <div class="hero-orb hero-orb--1" />
        <div class="hero-orb hero-orb--2" />
      </div>
      <div class="hero-content">
        <div class="hero-label-row">
          <span class="hero-title">账单记录</span>
          <span class="hero-subtitle">计费流水总览</span>
        </div>
        <div class="hero-cost">
          <span class="hero-cost__value">{{ formatCost(totalCost) }}</span>
          <span class="hero-cost__label">累计计费总额</span>
        </div>
      </div>
    </div>

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
    <div class="chip-scroll">
      <div
        v-for="s in statusFilters"
        :key="s.key"
        class="chip"
        :class="{ 'chip--active': activeStatus === s.key }"
        @click="setStatus(s.key)"
      >
        {{ s.label }}
      </div>
    </div>

    <!-- Count bar -->
    <div v-if="total > 0" class="count-bar">
      <span class="count-text">共 <b>{{ total }}</b> 条记录</span>
    </div>

    <!-- Record Cards -->
    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <van-list v-model:loading="loading" :finished="finished" finished-text="" @load="onLoad">
        <div class="card-list">
          <div
            v-for="(item, idx) in records"
            :key="item.id"
            class="record-card"
            :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
          >
            <!-- Card header: tenant + status -->
            <div class="record-card__head">
              <div class="record-card__tenant">
                <van-icon name="friends-o" size="13" class="record-card__tenant-icon" />
                <span class="record-card__tenant-name">{{ item.tenant_name || '-' }}</span>
              </div>
              <span
                class="record-card__status"
                :style="{
                  color: statusMap[item.status]?.color || '#64748b',
                  background: statusMap[item.status]?.bg || 'rgba(100,116,139,0.1)',
                }"
              >
                {{ statusMap[item.status]?.label || item.status }}
              </span>
            </div>

            <!-- Model name -->
            <div class="record-card__model">
              <van-icon name="cluster-o" size="12" />
              <span>{{ item.model_name || '-' }}</span>
            </div>

            <!-- Token stats -->
            <div class="record-card__tokens">
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
              <div class="token-item token-item--cost">
                <span class="token-label">费用</span>
                <span class="token-val token-val--cost">{{ formatCost(item.total_cost) }}</span>
              </div>
            </div>

            <!-- Footer: time -->
            <div class="record-card__footer">
              <van-icon name="clock-o" size="11" />
              <span>{{ formatTime(item.created_at) }}</span>
            </div>
          </div>
        </div>

        <!-- Empty state -->
        <div v-if="!loading && !records.length" class="empty-state">
          <van-icon name="balance-list-o" size="42" color="#cbd5e1" />
          <span class="empty-text">暂无账单记录</span>
          <span class="empty-hint">调整筛选条件查看更多</span>
        </div>
      </van-list>
    </van-pull-refresh>
  </div>
</template>

<style scoped>
.billing-page {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(16px + env(safe-area-inset-bottom, 0px));
}

/* ═══════════════════════════════════════
   HERO — Green Theme
   ═══════════════════════════════════════ */
.hero {
  position: relative;
  overflow: hidden;
  border-radius: 0 0 28px 28px;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #064e3b 0%, #059669 40%, #10b981 70%, #047857 100%);
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
  background: rgba(52, 211, 153, 0.2);
  top: -30px;
  right: -20px;
}
.hero-orb--2 {
  width: 140px;
  height: 140px;
  background: rgba(16, 185, 129, 0.15);
  bottom: -10px;
  left: -30px;
}

.hero-content {
  position: relative;
  z-index: 1;
  padding: 24px 20px 22px;
}

.hero-label-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 16px;
}

.hero-title {
  font-size: 20px;
  font-weight: 700;
  color: #fff;
  letter-spacing: -0.02em;
}

.hero-subtitle {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.5);
  font-weight: 500;
}

.hero-cost {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.hero-cost__value {
  font-size: 28px;
  font-weight: 800;
  color: #fff;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.03em;
}

.hero-cost__label {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.55);
  font-weight: 500;
}

/* ── Search ── */
.search-wrap {
  padding: 4px 0 0;
  animation: fadeSlideUp 0.4s 0.06s both;
}
.search-wrap :deep(.van-search) {
  padding: 8px 12px;
}

/* ── Status Filter Chips ── */
.chip-scroll {
  display: flex;
  gap: 8px;
  padding: 4px 16px 8px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  animation: fadeSlideUp 0.4s 0.1s both;
}
.chip-scroll::-webkit-scrollbar {
  display: none;
}

.chip {
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
.chip--active {
  color: #fff;
  background: #10b981;
  box-shadow: 0 2px 8px rgba(16, 185, 129, 0.3);
}

/* ── Count Bar ── */
.count-bar {
  padding: 4px 16px 6px;
  animation: fadeSlideUp 0.4s 0.14s both;
}
.count-text {
  font-size: 12px;
  color: var(--ta-text-tertiary, #94a3b8);
}
.count-text b {
  color: var(--ta-text-secondary, #475569);
  font-weight: 600;
}

/* ── Card List ── */
.card-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 12px;
}

.record-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  animation: fadeSlideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
  transition: transform 0.15s, box-shadow 0.15s;
}
.record-card:active {
  transform: scale(0.98);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
}

/* ── Card: Head ── */
.record-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.record-card__tenant {
  display: flex;
  align-items: center;
  gap: 5px;
  flex: 1;
  min-width: 0;
}

.record-card__tenant-icon {
  color: #10b981;
  flex-shrink: 0;
}

.record-card__tenant-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--ta-text-primary, #0f172a);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.record-card__status {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  line-height: 1.4;
}

/* ── Card: Model ── */
.record-card__model {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--ta-text-tertiary, #94a3b8);
  margin-bottom: 10px;
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  letter-spacing: -0.02em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ── Card: Tokens Row ── */
.record-card__tokens {
  display: flex;
  align-items: center;
  background: var(--ta-bg-secondary, #f8fafc);
  border-radius: 8px;
  padding: 8px 12px;
  margin-bottom: 8px;
}

.token-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}

.token-label {
  font-size: 10px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
}

.token-val {
  font-size: 13px;
  font-weight: 700;
  color: var(--ta-text-primary, #1e293b);
  font-variant-numeric: tabular-nums;
}

.token-val--cost {
  color: #10b981;
}

.token-divider {
  width: 1px;
  height: 24px;
  background: var(--ta-border-light, #e2e8f0);
  margin: 0 10px;
}

/* ── Card: Footer ── */
.record-card__footer {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
}

/* ── Empty State ── */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 56px 0 24px;
  animation: fadeSlideUp 0.4s both;
}

.empty-text {
  font-size: 14px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
}

.empty-hint {
  font-size: 12px;
  color: var(--ta-text-quaternary, #cbd5e1);
}

/* ── Animation ── */
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
