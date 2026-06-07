<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { showToast, showConfirmDialog } from 'vant'
import request from '@/utils/request'

const router = useRouter()

// ── State ──
const loading = ref(false)
const refreshing = ref(false)
const finished = ref(false)
const orders = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const activeStatus = ref('')

// ── Status filter chips ──
const statusFilters = [
  { key: '', label: '全部' },
  { key: 'pending', label: '待支付' },
  { key: 'paid', label: '已支付' },
  { key: 'fulfilled', label: '已完成' },
  { key: 'cancelled', label: '已取消' },
  { key: 'refunded', label: '已退款' },
]

// ── Status config ──
const statusConfig: Record<string, { label: string; color: string; bg: string }> = {
  pending: { label: '待支付', color: '#f59e0b', bg: 'rgba(245,158,11,0.1)' },
  paid: { label: '已支付', color: '#3b82f6', bg: 'rgba(59,130,246,0.1)' },
  fulfilled: { label: '已完成', color: '#10b981', bg: 'rgba(16,185,129,0.1)' },
  cancelled: { label: '已取消', color: '#94a3b8', bg: 'rgba(148,163,184,0.1)' },
  expired: { label: '已过期', color: '#94a3b8', bg: 'rgba(148,163,184,0.1)' },
  refunded: { label: '已退款', color: '#8b5cf6', bg: 'rgba(139,92,246,0.1)' },
}

const orderTypeMap: Record<string, string> = {
  recharge: '充值',
  plan_purchase: '套餐购买',
}

// ── Helpers ──
function formatCNY(n: number | undefined | null): string {
  if (n == null) return '¥0.00'
  const val = Number(n)
  const sign = val < 0 ? '-' : ''
  return `${sign}¥${Math.abs(val).toFixed(2)}`
}

function formatTime(dateStr: string | undefined): string {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  const pad = (v: number) => String(v).padStart(2, '0')
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function getStatusInfo(status: string) {
  return statusConfig[status] || { label: status, color: '#64748b', bg: 'rgba(100,116,139,0.1)' }
}

// ── Fetch ──
async function fetchOrders(append = false) {
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
    if (activeStatus.value) params.status = activeStatus.value

    const { data: res } = await request.get('/admin/orders', { params })
    const list = res.data?.list || []
    total.value = res.data?.total || 0

    if (append) {
      orders.value = [...orders.value, ...list]
    } else {
      orders.value = list
    }
    finished.value = orders.value.length >= total.value
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

// ── Handlers ──
async function onRefresh() {
  refreshing.value = true
  await fetchOrders(false)
}

async function onLoad() {
  if (finished.value) return
  page.value++
  await fetchOrders(true)
}

function setStatus(key: string) {
  activeStatus.value = key
  fetchOrders(false)
}

function goToDetail(item: any) {
  router.push({
    path: `/m/orders/${item.id}`,
    state: { order: item },
  })
}

async function completeOrder(item: any) {
  try {
    await showConfirmDialog({
      title: '确认完成',
      message: `确定要将订单 ${item.order_no} 标记为已完成吗？`,
    })
    await request.post(`/admin/orders/${item.id}/complete`)
    showToast('订单已完成')
    fetchOrders(false)
  } catch {
    // cancelled or error
  }
}

async function refundOrder(item: any) {
  try {
    await showConfirmDialog({
      title: '确认退款',
      message: `确定要对订单 ${item.order_no} 进行退款吗？此操作不可撤销。`,
    })
    await request.post(`/admin/orders/${item.id}/refund`, { reason: '管理员操作退款' })
    showToast('退款成功')
    fetchOrders(false)
  } catch {
    // cancelled or error
  }
}

async function cancelOrder(item: any) {
  try {
    await showConfirmDialog({
      title: '确认取消',
      message: `确定要取消订单 ${item.order_no} 吗？`,
    })
    await request.post(`/admin/orders/${item.id}/cancel`)
    showToast('订单已取消')
    fetchOrders(false)
  } catch {
    // cancelled or error
  }
}

onMounted(() => fetchOrders())
</script>

<template>
  <div class="order-page">

    <!-- ═══════ HERO ═══════ -->
    <div class="hero">
      <div class="hero-bg">
        <div class="hero-orb hero-orb--1" />
        <div class="hero-orb hero-orb--2" />
      </div>
      <div class="hero-content">
        <div class="hero-title-row">
          <h2 class="hero-title">订单管理</h2>
          <span class="hero-subtitle">订单流水总览</span>
        </div>
        <div class="hero-stats">
          <div class="hero-stat hero-stat--main">
            <div class="hero-stat__value">{{ total }}</div>
            <div class="hero-stat__label">订单总数</div>
          </div>
        </div>
      </div>
    </div>

    <!-- ═══════ STATUS FILTER CHIPS ═══════ -->
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

    <!-- ═══════ COUNT BAR ═══════ -->
    <div v-if="total > 0" class="count-bar">
      <span class="count-text">共 <b>{{ total }}</b> 笔订单</span>
    </div>

    <!-- ═══════ ORDER LIST ═══════ -->
    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <van-list v-model:loading="loading" :finished="finished" finished-text="" @load="onLoad">
        <div class="card-list">
          <van-swipe-cell v-for="(item, idx) in orders" :key="item.id">
            <div
              class="order-card"
              :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
              @click="goToDetail(item)"
            >
              <!-- Card header: order_no + status badge -->
              <div class="order-card__head">
                <div class="order-card__no">
                  <van-icon name="orders-o" size="13" class="order-card__no-icon" />
                  <span class="order-card__no-text">{{ item.order_no || '-' }}</span>
                </div>
                <span
                  class="order-card__status"
                  :style="{
                    color: getStatusInfo(item.status).color,
                    background: getStatusInfo(item.status).bg,
                  }"
                >
                  {{ getStatusInfo(item.status).label }}
                </span>
              </div>

              <!-- Amount (prominent) -->
              <div class="order-card__amount-row">
                <span class="order-card__amount">{{ formatCNY(item.final_amount) }}</span>
                <span class="order-card__type-badge">
                  {{ orderTypeMap[item.order_type] || item.order_type || '-' }}
                </span>
              </div>

              <!-- Discount info -->
              <div v-if="item.discount_amount && Number(item.discount_amount) > 0" class="order-card__discount">
                <span>原价 {{ formatCNY(item.amount) }}</span>
                <span class="order-card__discount-sep">·</span>
                <span>优惠 {{ formatCNY(item.discount_amount) }}</span>
              </div>

              <!-- Footer: time -->
              <div class="order-card__footer">
                <van-icon name="clock-o" size="11" />
                <span>{{ formatTime(item.created_at) }}</span>
              </div>
            </div>

            <!-- Swipe actions -->
            <template #right>
              <div class="swipe-actions">
                <div
                  v-if="item.status === 'paid'"
                  class="swipe-btn swipe-btn--success"
                  @click="completeOrder(item)"
                >
                  完成
                </div>
                <div
                  v-if="item.status === 'paid'"
                  class="swipe-btn swipe-btn--purple"
                  @click="refundOrder(item)"
                >
                  退款
                </div>
                <div
                  v-if="item.status === 'pending'"
                  class="swipe-btn swipe-btn--danger"
                  @click="cancelOrder(item)"
                >
                  取消
                </div>
              </div>
            </template>
          </van-swipe-cell>
        </div>

        <!-- Empty state -->
        <div v-if="!loading && !orders.length" class="empty-state">
          <van-icon name="orders-o" size="42" color="#cbd5e1" />
          <span class="empty-text">暂无订单数据</span>
          <span class="empty-hint">调整筛选条件查看更多</span>
        </div>
      </van-list>
    </van-pull-refresh>
  </div>
</template>

<style scoped>
.order-page {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(16px + env(safe-area-inset-bottom, 0px));
}

/* ═══════════════════════════════════════
   HERO — Blue Theme
   ═══════════════════════════════════════ */
.hero {
  position: relative;
  overflow: hidden;
  border-radius: 0 0 28px 28px;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #1e3a5f 0%, #2563eb 35%, #3b82f6 65%, #1d4ed8 100%);
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
  background: rgba(147, 197, 253, 0.2);
  top: -30px;
  right: -20px;
}
.hero-orb--2 {
  width: 140px;
  height: 140px;
  background: rgba(59, 130, 246, 0.2);
  bottom: 0;
  left: -20px;
}

.hero-content {
  position: relative;
  z-index: 1;
  padding: 24px 20px 22px;
}

.hero-title-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 16px;
}

.hero-title {
  font-size: 20px;
  font-weight: 700;
  color: #fff;
  margin: 0;
  letter-spacing: -0.02em;
}

.hero-subtitle {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.5);
  font-weight: 500;
}

.hero-stats {
  display: flex;
  gap: 10px;
}

.hero-stat {
  flex: 1;
  background: rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 14px;
  padding: 14px 12px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.hero-stat--main {
  background: rgba(255, 255, 255, 0.15);
}

.hero-stat__value {
  font-size: 26px;
  font-weight: 700;
  color: #fff;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.03em;
  line-height: 1.2;
}

.hero-stat__label {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.55);
  margin-top: 4px;
  font-weight: 500;
}

/* ═══════════════════════════════════════
   STATUS FILTER CHIPS
   ═══════════════════════════════════════ */
.chip-scroll {
  display: flex;
  gap: 8px;
  padding: 10px 16px 6px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  animation: fadeSlideUp 0.4s 0.06s both;
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
  background: #3b82f6;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.3);
}

/* ═══════════════════════════════════════
   COUNT BAR
   ═══════════════════════════════════════ */
.count-bar {
  padding: 4px 16px 6px;
  animation: fadeSlideUp 0.4s 0.1s both;
}
.count-text {
  font-size: 12px;
  color: var(--ta-text-tertiary, #94a3b8);
}
.count-text b {
  color: var(--ta-text-secondary, #475569);
  font-weight: 600;
}

/* ═══════════════════════════════════════
   CARD LIST
   ═══════════════════════════════════════ */
.card-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 12px;
}

.order-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  animation: fadeSlideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
  transition: transform 0.15s, box-shadow 0.15s;
  cursor: pointer;
}
.order-card:active {
  transform: scale(0.98);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
}

/* ── Card: Head ── */
.order-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 10px;
}

.order-card__no {
  display: flex;
  align-items: center;
  gap: 5px;
  flex: 1;
  min-width: 0;
}

.order-card__no-icon {
  color: #3b82f6;
  flex-shrink: 0;
}

.order-card__no-text {
  font-size: 13px;
  font-weight: 600;
  color: var(--ta-text-primary, #0f172a);
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  letter-spacing: -0.02em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.order-card__status {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  line-height: 1.4;
  white-space: nowrap;
}

/* ── Card: Amount Row ── */
.order-card__amount-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}

.order-card__amount {
  font-size: 24px;
  font-weight: 800;
  color: var(--ta-text-primary, #1e293b);
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.02em;
  line-height: 1.2;
}

.order-card__type-badge {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 10px;
  background: var(--ta-bg-secondary, #f1f5f9);
  color: var(--ta-text-tertiary, #64748b);
  line-height: 1.4;
  white-space: nowrap;
}

/* ── Card: Discount ── */
.order-card__discount {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  margin-bottom: 8px;
}

.order-card__discount-sep {
  color: #e2e8f0;
}

/* ── Card: Footer ── */
.order-card__footer {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
}

/* ═══════════════════════════════════════
   SWIPE ACTIONS
   ═══════════════════════════════════════ */
.swipe-actions {
  display: flex;
  height: 100%;
  border-radius: 0 14px 14px 0;
  overflow: hidden;
}

.swipe-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 60px;
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  cursor: pointer;
}
.swipe-btn--success {
  background: #10b981;
}
.swipe-btn--purple {
  background: #8b5cf6;
}
.swipe-btn--danger {
  background: #f59e0b;
}

/* ═══════════════════════════════════════
   EMPTY STATE
   ═══════════════════════════════════════ */
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
