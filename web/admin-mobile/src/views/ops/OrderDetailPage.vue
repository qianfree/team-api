<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { showToast, showConfirmDialog } from 'vant'
import request from '@/utils/request'

const router = useRouter()
const route = useRoute()

// ── State ──
const order = ref<any>(null)
const loading = ref(false)
const refundReason = ref('')
const showRefundDialog = ref(false)
const refunding = ref(false)

// ── Status config (reuse same map from list page) ──
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

// ── Computed ──
const statusInfo = computed(() => {
  if (!order.value) return { label: '-', color: '#64748b', bg: 'rgba(100,116,139,0.1)' }
  return statusConfig[order.value.status] || { label: order.value.status, color: '#64748b', bg: 'rgba(100,116,139,0.1)' }
})

const canComplete = computed(() => order.value?.status === 'paid')
const canRefund = computed(() => order.value?.status === 'paid')

// ── Helpers ──
function formatCNY(n: number | undefined | null): string {
  if (n == null) return '¥0.00'
  const val = Number(n)
  const sign = val < 0 ? '-' : ''
  return `${sign}¥${Math.abs(val).toFixed(2)}`
}

function formatDateTime(dateStr: string | undefined): string {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  const pad = (v: number) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

// ── Fetch ──
async function fetchOrder() {
  const id = route.params.id as string
  if (!id) return

  loading.value = true
  try {
    const { data: res } = await request.get(`/admin/orders/${id}`)
    order.value = res.data
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
  }
}

// ── Actions ──
async function completeOrder() {
  if (!order.value) return
  try {
    await showConfirmDialog({
      title: '确认完成',
      message: `确定要将订单 ${order.value.order_no} 标记为已完成吗？`,
    })
    await request.post(`/admin/orders/${order.value.id}/complete`)
    showToast('订单已完成')
    fetchOrder()
  } catch {
    // cancelled or error
  }
}

function openRefundDialog() {
  refundReason.value = ''
  showRefundDialog.value = true
}

async function confirmRefund(): Promise<boolean> {
  if (!order.value) return false
  if (!refundReason.value.trim()) {
    showToast('请输入退款原因')
    return false
  }
  refunding.value = true
  try {
    await request.post(`/admin/orders/${order.value.id}/refund`, {
      reason: refundReason.value.trim(),
    })
    showToast('退款成功')
    showRefundDialog.value = false
    fetchOrder()
    return true
  } catch {
    return false
  } finally {
    refunding.value = false
  }
}

// ── Info items ──
const infoItems = computed(() => {
  if (!order.value) return []
  return [
    { label: '订单类型', value: orderTypeMap[order.value.order_type] || order.value.order_type || '-' },
    { label: '支付渠道', value: order.value.payment_channel || '-' },
    { label: '支付方式', value: order.value.payment_method || '-' },
    { label: '支付流水号', value: order.value.payment_no || '-', mono: true },
    { label: '货币', value: order.value.currency || 'CNY' },
    { label: '创建时间', value: formatDateTime(order.value.created_at) },
    { label: '支付时间', value: formatDateTime(order.value.paid_at) },
    { label: '完成时间', value: formatDateTime(order.value.fulfilled_at) },
  ]
})

// ── Init ──
onMounted(() => {
  // Try history.state first, fallback to API
  const cached = history.state?.order
  if (cached) {
    order.value = cached
  }
  fetchOrder()
})
</script>

<template>
  <div class="detail-page">

    <!-- ═══════ HERO ═══════ -->
    <div class="hero">
      <div class="hero-bg">
        <div class="hero-orb hero-orb--1" />
        <div class="hero-orb hero-orb--2" />
      </div>
      <div class="hero-content">
        <div class="hero-nav" @click="router.back()">
          <van-icon name="arrow-left" size="18" color="#fff" />
          <span>订单详情</span>
        </div>

        <div v-if="order" class="hero-body">
          <div class="hero-no">{{ order.order_no || '-' }}</div>
          <span
            class="hero-status"
            :style="{
              color: statusInfo.color,
              background: statusInfo.bg,
            }"
          >
            {{ statusInfo.label }}
          </span>
        </div>
        <div v-else class="hero-skeleton">
          <div class="skel-bar skel-bar--lg" />
          <div class="skel-bar skel-bar--sm" />
        </div>
      </div>
    </div>

    <!-- ═══════ LOADING ═══════ -->
    <div v-if="loading && !order" class="loading-wrap">
      <van-loading size="24" color="#3b82f6" vertical>加载中...</van-loading>
    </div>

    <!-- ═══════ CONTENT ═══════ -->
    <template v-if="order">
      <!-- Stats Row -->
      <div class="stats-row">
        <div class="stat-card">
          <div class="stat-card__label">订单金额</div>
          <div class="stat-card__value">{{ formatCNY(order.amount) }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-card__label">优惠金额</div>
          <div class="stat-card__value stat-card__value--discount">{{ formatCNY(order.discount_amount) }}</div>
        </div>
        <div class="stat-card stat-card--highlight">
          <div class="stat-card__label">实付金额</div>
          <div class="stat-card__value stat-card__value--primary">{{ formatCNY(order.final_amount) }}</div>
        </div>
      </div>

      <!-- Info Section -->
      <div class="info-section">
        <div class="info-section__title">订单信息</div>
        <div class="info-section__card">
          <div
            v-for="(item, idx) in infoItems"
            :key="idx"
            class="info-row"
            :class="{ 'info-row--border': idx < infoItems.length - 1 }"
          >
            <span class="info-row__label">{{ item.label }}</span>
            <span
              class="info-row__value"
              :class="{ 'info-row__value--mono': item.mono }"
            >
              {{ item.value }}
            </span>
          </div>
        </div>
      </div>

      <!-- Tenant info -->
      <div v-if="order.tenant_name || order.tenant_id" class="info-section">
        <div class="info-section__title">租户信息</div>
        <div class="info-section__card">
          <div class="info-row">
            <span class="info-row__label">租户</span>
            <span class="info-row__value">{{ order.tenant_name || `租户 #${order.tenant_id}` }}</span>
          </div>
        </div>
      </div>

      <!-- Action Buttons -->
      <div v-if="canComplete || canRefund" class="action-bar">
        <van-button
          v-if="canComplete"
          type="success"
          round
          block
          size="small"
          class="action-btn"
          @click="completeOrder"
        >
          完成订单
        </van-button>
        <van-button
          v-if="canRefund"
          plain
          round
          block
          size="small"
          class="action-btn action-btn--refund"
          @click="openRefundDialog"
        >
          退款
        </van-button>
      </div>
    </template>

    <!-- ═══════ EMPTY ═══════ -->
    <div v-if="!loading && !order" class="empty-state">
      <van-icon name="orders-o" size="42" color="#cbd5e1" />
      <span class="empty-text">订单不存在或已被删除</span>
    </div>

    <!-- ═══════ REFUND DIALOG ═══════ -->
    <van-dialog
      v-model:show="showRefundDialog"
      title="退款"
      show-cancel-button
      :loading="refunding"
      :before-close="(action: string) => { if (action === 'confirm') return confirmRefund(); return true }"
    >
      <div class="dialog-form">
        <div class="form-field">
          <label class="form-label">退款原因 <span class="form-required">*</span></label>
          <van-field
            v-model="refundReason"
            type="textarea"
            placeholder="请输入退款原因..."
            rows="3"
            maxlength="200"
            show-word-limit
            autosize
            :border="false"
          />
        </div>
      </div>
    </van-dialog>
  </div>
</template>

<style scoped>
.detail-page {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(24px + env(safe-area-inset-bottom, 0px));
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
  padding: 20px 20px 24px;
}

.hero-nav {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 20px;
  cursor: pointer;
  -webkit-tap-highlight-color: transparent;
}
.hero-nav span {
  font-size: 15px;
  font-weight: 600;
  color: #fff;
}

.hero-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.hero-no {
  font-size: 18px;
  font-weight: 700;
  color: #fff;
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  letter-spacing: -0.02em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.hero-status {
  flex-shrink: 0;
  font-size: 12px;
  font-weight: 600;
  padding: 3px 10px;
  border-radius: 12px;
  line-height: 1.4;
  white-space: nowrap;
}

/* ── Hero Skeleton ── */
.hero-skeleton {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-top: 4px;
}

.skel-bar {
  height: 12px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.12);
}

.skel-bar--lg {
  width: 70%;
  height: 20px;
}

.skel-bar--sm {
  width: 30%;
  height: 10px;
}

/* ═══════════════════════════════════════
   STATS ROW
   ═══════════════════════════════════════ */
.stats-row {
  display: flex;
  gap: 8px;
  padding: 16px 12px 0;
  animation: fadeSlideUp 0.4s 0.06s both;
}

.stat-card {
  flex: 1;
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 12px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
}

.stat-card--highlight {
  background: linear-gradient(135deg, #eff6ff, #dbeafe);
  border: 1px solid rgba(59, 130, 246, 0.12);
}

.stat-card__label {
  font-size: 10px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
  margin-bottom: 4px;
}

.stat-card__value {
  font-size: 16px;
  font-weight: 700;
  color: var(--ta-text-primary, #1e293b);
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.02em;
}

.stat-card__value--discount {
  color: #f59e0b;
}

.stat-card__value--primary {
  color: #3b82f6;
}

/* ═══════════════════════════════════════
   INFO SECTION
   ═══════════════════════════════════════ */
.info-section {
  padding: 16px 12px 0;
  animation: fadeSlideUp 0.4s 0.1s both;
}

.info-section__title {
  font-size: 13px;
  font-weight: 600;
  color: var(--ta-text-secondary, #475569);
  margin-bottom: 8px;
  padding-left: 4px;
}

.info-section__card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 4px 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
}

.info-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 11px 0;
}

.info-row--border {
  border-bottom: 1px solid var(--ta-border-light, #f1f5f9);
}

.info-row__label {
  font-size: 13px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
  flex-shrink: 0;
  white-space: nowrap;
}

.info-row__value {
  font-size: 13px;
  font-weight: 500;
  color: var(--ta-text-primary, #0f172a);
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.info-row__value--mono {
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  font-size: 12px;
  letter-spacing: -0.02em;
}

/* ═══════════════════════════════════════
   ACTION BAR
   ═══════════════════════════════════════ */
.action-bar {
  display: flex;
  gap: 10px;
  padding: 20px 12px 0;
  animation: fadeSlideUp 0.4s 0.14s both;
}

.action-btn {
  flex: 1;
}

.action-btn--refund {
  color: #8b5cf6 !important;
  border-color: #8b5cf6 !important;
}

/* ═══════════════════════════════════════
   LOADING
   ═══════════════════════════════════════ */
.loading-wrap {
  display: flex;
  justify-content: center;
  padding: 48px 0;
}

/* ═══════════════════════════════════════
   EMPTY STATE
   ═══════════════════════════════════════ */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 64px 0 24px;
  animation: fadeSlideUp 0.4s both;
}

.empty-text {
  font-size: 14px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
}

/* ═══════════════════════════════════════
   REFUND DIALOG FORM
   ═══════════════════════════════════════ */
.dialog-form {
  padding: 8px 16px 0;
}

.form-field {
  margin-bottom: 12px;
}

.form-label {
  display: block;
  font-size: 12px;
  font-weight: 600;
  color: #475569;
  margin-bottom: 4px;
}

.form-required {
  color: #ef4444;
}

.form-field :deep(.van-cell) {
  background: #f8fafc;
  border-radius: 10px;
  padding: 8px 12px;
}

.form-field :deep(.van-field__control) {
  font-size: 14px;
}

.form-field :deep(.van-field__word-limit) {
  font-size: 10px;
  color: #94a3b8;
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
