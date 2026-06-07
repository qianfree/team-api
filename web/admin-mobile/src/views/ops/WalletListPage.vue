<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import request from '@/utils/request'

const router = useRouter()

const loading = ref(false)
const refreshing = ref(false)
const finished = ref(false)
const wallets = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const search = ref('')

function formatUSD(n: number | undefined | null): string {
  if (n == null) return '$0.00'
  const val = Number(n)
  const sign = val < 0 ? '-' : ''
  return `${sign}$${Math.abs(val).toFixed(2)}`
}

const totalBalance = computed(() => {
  return wallets.value.reduce((sum, w) => sum + Number(w.balance || 0), 0)
})

function isNearWarning(item: any): boolean {
  if (!item.warning_threshold || item.warning_threshold <= 0) return false
  return Number(item.balance) <= Number(item.warning_threshold)
}

function getAvailable(item: any): number {
  return Number(item.balance || 0) - Number(item.frozen_balance || 0)
}

function formatTime(t: string | undefined): string {
  if (!t) return '-'
  const d = new Date(t)
  const now = new Date()
  const diffMs = now.getTime() - d.getTime()
  const diffMin = Math.floor(diffMs / 60000)
  if (diffMin < 1) return '刚刚'
  if (diffMin < 60) return `${diffMin}分钟前`
  const diffH = Math.floor(diffMin / 60)
  if (diffH < 24) return `${diffH}小时前`
  const diffD = Math.floor(diffH / 24)
  if (diffD < 30) return `${diffD}天前`
  const mm = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  return `${mm}-${dd}`
}

async function fetchWallets(append = false) {
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

    const { data: res } = await request.get('/admin/wallets', { params })
    const list = res.data?.list || []
    total.value = res.data?.total || 0

    if (append) {
      wallets.value = [...wallets.value, ...list]
    } else {
      wallets.value = list
    }
    finished.value = wallets.value.length >= total.value
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function onRefresh() {
  refreshing.value = true
  await fetchWallets(false)
}

async function onLoad() {
  if (finished.value) return
  page.value++
  await fetchWallets(true)
}

function onSearch(val: string) {
  search.value = val
  fetchWallets(false)
}

function goToDetail(item: any) {
  router.push(`/m/wallets/${item.tenant_id}`)
}

onMounted(() => fetchWallets())
</script>

<template>
  <div class="page">

    <!-- Hero: Total Balance -->
    <div class="hero">
      <div class="hero-bg"></div>
      <div class="hero-content">
        <div class="hero-label">
          <van-icon name="balance-o" size="14" />
          <span>全平台钱包总额</span>
        </div>
        <div class="hero-value">{{ formatUSD(totalBalance) }}</div>
        <div class="hero-sub">USD · {{ wallets.length }} 个钱包已加载</div>
      </div>
    </div>

    <!-- Search -->
    <div class="search-wrap">
      <van-search
        v-model="search"
        placeholder="搜索租户名称..."
        shape="round"
        @search="onSearch"
        @clear="onSearch('')"
      />
    </div>

    <!-- Count -->
    <div v-if="total > 0" class="count-bar">
      <span class="count-text">共 <b>{{ total }}</b> 个钱包</span>
    </div>

    <!-- Wallet Cards -->
    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <van-list v-model:loading="loading" :finished="finished" finished-text="" @load="onLoad">
        <div class="card-list">
          <div
            v-for="(item, idx) in wallets"
            :key="item.id"
            class="wallet-card"
            :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
            @click="goToDetail(item)"
          >
            <!-- Warning banner -->
            <div v-if="isNearWarning(item)" class="wallet-card__warning">
              <van-icon name="warning-o" size="12" />
              <span>余额已接近预警阈值</span>
            </div>

            <!-- Top: tenant name + time -->
            <div class="wallet-card__top">
              <h4 class="wallet-card__name">{{ item.tenant_name || `租户 #${item.tenant_id}` }}</h4>
              <span class="wallet-card__time">{{ formatTime(item.updated_at) }}</span>
            </div>

            <!-- Balance (prominent) -->
            <div class="wallet-card__balance-row">
              <span
                class="wallet-card__balance"
                :class="{ 'wallet-card__balance--positive': Number(item.balance) > 0 }"
              >
                {{ formatUSD(item.balance) }}
              </span>
              <van-icon name="arrow" size="14" color="#cbd5e1" />
            </div>

            <!-- Sub-balance row: frozen + available -->
            <div class="wallet-card__details">
              <div class="detail-item">
                <span class="detail-label">冻结</span>
                <span class="detail-val detail-val--frozen">{{ formatUSD(item.frozen_balance) }}</span>
              </div>
              <div class="detail-divider" />
              <div class="detail-item">
                <span class="detail-label">可用</span>
                <span class="detail-val detail-val--available">{{ formatUSD(getAvailable(item)) }}</span>
              </div>
              <div class="detail-divider" />
              <div class="detail-item">
                <span class="detail-label">累计充值</span>
                <span class="detail-val">{{ formatUSD(item.cumulative_recharge) }}</span>
              </div>
            </div>
          </div>
        </div>

        <div v-if="!loading && !wallets.length" class="empty-state">
          <van-icon name="balance-list-o" size="40" color="#cbd5e1" />
          <span>暂无钱包数据</span>
        </div>
      </van-list>
    </van-pull-refresh>
  </div>
</template>

<style scoped>
.page {
  padding-bottom: 24px;
}

/* ── Hero ── */
.hero {
  position: relative;
  margin: 12px 12px 0;
  border-radius: 16px;
  overflow: hidden;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(135deg, #f59e0b 0%, #d97706 50%, #b45309 100%);
  z-index: 0;
}

.hero-content {
  position: relative;
  z-index: 1;
  padding: 20px 18px 18px;
}

.hero-label {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  font-weight: 500;
  color: rgba(255, 255, 255, 0.8);
  margin-bottom: 6px;
}

.hero-value {
  font-size: 32px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
  line-height: 1.2;
  font-variant-numeric: tabular-nums;
}

.hero-sub {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.6);
  margin-top: 6px;
  font-weight: 500;
}

/* ── Search ── */
.search-wrap {
  padding: 4px 0 0;
}
.search-wrap :deep(.van-search) {
  padding: 8px 12px;
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

.wallet-card {
  background: #fff;
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;
  animation: fadeSlideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
}
.wallet-card:active {
  transform: scale(0.98);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
}

@keyframes fadeSlideUp {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* ── Card: Warning ── */
.wallet-card__warning {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 5px 10px;
  border-radius: 8px;
  background: rgba(245, 158, 11, 0.08);
  color: #d97706;
  font-size: 11px;
  font-weight: 500;
  margin-bottom: 10px;
}

/* ── Card: Top ── */
.wallet-card__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.wallet-card__name {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.wallet-card__time {
  font-size: 11px;
  color: #94a3b8;
  flex-shrink: 0;
  white-space: nowrap;
}

/* ── Card: Balance ── */
.wallet-card__balance-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.wallet-card__balance {
  font-size: 26px;
  font-weight: 800;
  color: #334155;
  letter-spacing: -0.02em;
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
}
.wallet-card__balance--positive {
  color: #059669;
}

/* ── Card: Details ── */
.wallet-card__details {
  display: flex;
  align-items: center;
  gap: 0;
  background: #f8fafc;
  border-radius: 8px;
  padding: 8px 10px;
}

.detail-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.detail-label {
  font-size: 10px;
  color: #94a3b8;
  font-weight: 500;
}

.detail-val {
  font-size: 13px;
  font-weight: 700;
  color: #475569;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.detail-val--frozen {
  color: #d97706;
}

.detail-val--available {
  color: var(--ta-primary, #0d9488);
}

.detail-divider {
  width: 1px;
  height: 24px;
  background: #e2e8f0;
  margin: 0 6px;
  flex-shrink: 0;
}

/* ── Empty ── */
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
