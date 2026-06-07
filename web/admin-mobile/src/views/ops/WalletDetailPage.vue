<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { showToast } from 'vant'
import request from '@/utils/request'

const route = useRoute()

const tenantId = computed(() => route.params.tenantId as string)

// ── Wallet state ──
const wallet = ref<any>(null)
const walletLoading = ref(true)

// ── Transaction list state ──
const transactions = ref<any[]>([])
const txLoading = ref(false)
const txRefreshing = ref(false)
const txFinished = ref(false)
const txPage = ref(1)
const txTotal = ref(0)

// ── Filter state ──
const activeType = ref('')

const typeFilters = [
  { key: '', label: '全部' },
  { key: 'recharge', label: '充值' },
  { key: 'consumption', label: '消费' },
  { key: 'refund', label: '退款' },
  { key: 'adjustment', label: '调整' },
]

// ── Dialog state ──
const showAdjustDialog = ref(false)
const adjustAmount = ref('')
const adjustDescription = ref('')
const adjustDirection = ref('recharge') // recharge | deduct
const adjustSubmitting = ref(false)

// ── Computed ──
const availableBalance = computed(() => {
  if (!wallet.value) return 0
  return Number(wallet.value.balance || 0) - Number(wallet.value.frozen_balance || 0)
})

// ── Fetch wallet ──
async function fetchWallet() {
  walletLoading.value = true
  try {
    const { data: res } = await request.get(`/admin/wallets/${tenantId.value}`)
    wallet.value = res.data
  } catch {
    // handled by interceptor
  } finally {
    walletLoading.value = false
  }
}

// ── Fetch transactions ──
async function fetchTransactions(append = false) {
  if (!append) {
    txPage.value = 1
    txFinished.value = false
  }

  txLoading.value = true
  try {
    const params: any = {
      page: txPage.value,
      page_size: 20,
    }
    if (activeType.value) params.type = activeType.value

    const { data: res } = await request.get(`/admin/wallets/${tenantId.value}/transactions`, { params })
    const list = res.data?.list || []
    txTotal.value = res.data?.total || 0
    transactions.value = append ? [...transactions.value, ...list] : list
    txFinished.value = transactions.value.length >= txTotal.value
  } catch {
    // handled by interceptor
  } finally {
    txLoading.value = false
    txRefreshing.value = false
  }
}

// ── Handlers ──
async function onRefresh() {
  txRefreshing.value = true
  await Promise.all([fetchWallet(), fetchTransactions(false)])
}

async function onLoad() {
  if (txFinished.value) return
  txPage.value++
  await fetchTransactions(true)
}

function setType(key: string) {
  activeType.value = key
  fetchTransactions(false)
}

function openAdjustDialog() {
  adjustAmount.value = ''
  adjustDescription.value = ''
  adjustDirection.value = 'recharge'
  showAdjustDialog.value = true
}

async function submitAdjust() {
  const amount = Number(adjustAmount.value)
  if (!amount || amount <= 0) {
    showToast('请输入有效金额')
    return
  }
  if (!adjustDescription.value.trim()) {
    showToast('请输入说明')
    return
  }

  adjustSubmitting.value = true
  try {
    const finalAmount = adjustDirection.value === 'deduct' ? -amount : amount
    await request.post(`/admin/wallets/${tenantId.value}/adjust`, {
      amount: finalAmount,
      description: adjustDescription.value.trim(),
    })
    showAdjustDialog.value = false
    showToast(adjustDirection.value === 'recharge' ? '充值成功' : '扣减成功')
    await fetchWallet()
    await fetchTransactions(false)
  } catch {
    // handled by interceptor
  } finally {
    adjustSubmitting.value = false
  }
}

// ── Helpers ──
function formatUSD(n: number | undefined | null): string {
  if (n == null) return '$0.00'
  const val = Number(n)
  const sign = val < 0 ? '-' : ''
  return `${sign}$${Math.abs(val).toFixed(2)}`
}

function formatUSD6(n: number | undefined | null): string {
  if (n == null) return '$0.000000'
  const val = Number(n)
  const sign = val < 0 ? '-' : ''
  return `${sign}$${Math.abs(val).toFixed(6)}`
}

function typeColor(type: string): string {
  const map: Record<string, string> = {
    recharge: '#10b981',
    consumption: '#ef4444',
    refund: '#3b82f6',
    adjustment: '#8b5cf6',
    freeze: '#f59e0b',
    unfreeze: '#f59e0b',
  }
  return map[type] || '#64748b'
}

function typeLabel(type: string): string {
  const map: Record<string, string> = {
    recharge: '充值',
    consumption: '消费',
    refund: '退款',
    adjustment: '调整',
    freeze: '冻结',
    unfreeze: '解冻',
  }
  return map[type] || type || '-'
}

function amountSign(item: any): string {
  const amount = Number(item.amount || 0)
  if (amount > 0) return '+'
  if (amount < 0) return ''
  return ''
}

function amountColor(item: any): string {
  const amount = Number(item.amount || 0)
  if (amount > 0) return '#10b981'
  if (amount < 0) return '#ef4444'
  return '#64748b'
}

function formatDate(dateStr: string): string {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const hours = String(d.getHours()).padStart(2, '0')
  const mins = String(d.getMinutes()).padStart(2, '0')
  return `${month}-${day} ${hours}:${mins}`
}

onMounted(() => {
  fetchWallet()
  fetchTransactions(false)
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
        <div class="hero-title-row">
          <h2 class="hero-title">钱包详情</h2>
        </div>

        <!-- Skeleton -->
        <div v-if="walletLoading && !wallet" class="hero-skeleton">
          <div class="skel-bar skel-bar--xl" />
          <div class="skel-bar skel-bar--md" />
          <div class="skel-bar skel-bar--sm" />
        </div>

        <template v-if="wallet">
          <div class="balance-group">
            <!-- Total balance -->
            <div class="balance-item balance-item--main">
              <span class="balance-label">账户余额</span>
              <span class="balance-value" :class="{ 'balance-value--positive': Number(wallet.balance) > 0 }">
                {{ formatUSD6(wallet.balance) }}
              </span>
            </div>
            <!-- Frozen & Available row -->
            <div class="balance-row">
              <div class="balance-item">
                <span class="balance-label">冻结余额</span>
                <span class="balance-value balance-value--frozen">
                  {{ formatUSD6(wallet.frozen_balance) }}
                </span>
              </div>
              <div class="balance-divider" />
              <div class="balance-item">
                <span class="balance-label">可用余额</span>
                <span class="balance-value balance-value--available">
                  {{ formatUSD6(availableBalance) }}
                </span>
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>

    <!-- ═══════ QUICK ACTION ═══════ -->
    <div class="quick-action-wrap" v-if="wallet">
      <button class="adjust-btn" @click="openAdjustDialog">
        <van-icon name="edit" size="16" />
        <span>调整余额</span>
      </button>
    </div>

    <!-- ═══════ TRANSACTIONS SECTION ═══════ -->
    <div class="tx-section" style="--si: 0">
      <div class="section-header">
        <span class="section-dot" style="--c: #0d9488"></span>
        <span class="section-title">交易流水</span>
        <span class="section-count" v-if="txTotal > 0">{{ txTotal }}</span>
      </div>

      <!-- Type filters -->
      <div class="filters">
        <div class="filter-group">
          <div
            v-for="f in typeFilters"
            :key="f.key"
            class="filter-chip"
            :class="{ 'filter-chip--active': activeType === f.key }"
            @click="setType(f.key)"
          >
            {{ f.label }}
          </div>
        </div>
      </div>

      <!-- Pull refresh + infinite scroll -->
      <van-pull-refresh v-model="txRefreshing" @refresh="onRefresh">
        <van-list v-model:loading="txLoading" :finished="txFinished" finished-text="" @load="onLoad">
          <div class="card-list">
            <div
              v-for="(item, idx) in transactions"
              :key="item.id"
              class="tx-card"
              :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
            >
              <!-- Card header: type badge + amount -->
              <div class="tx-card__head">
                <div class="tx-card__badges">
                  <span
                    class="badge badge--type"
                    :style="{
                      background: `${typeColor(item.type)}14`,
                      color: typeColor(item.type),
                    }"
                  >
                    {{ typeLabel(item.type) }}
                  </span>
                </div>
                <span class="tx-card__amount" :style="{ color: amountColor(item) }">
                  {{ amountSign(item) }}{{ formatUSD6(item.amount) }}
                </span>
              </div>

              <!-- Description -->
              <div class="tx-card__desc" v-if="item.description">
                {{ item.description }}
              </div>

              <!-- Meta row: username / model_name -->
              <div class="tx-card__meta" v-if="item.username || item.model_name">
                <span v-if="item.username" class="tx-card__tag">
                  <van-icon name="manager-o" size="11" />
                  {{ item.username }}
                </span>
                <span v-if="item.model_name" class="tx-card__tag">
                  <van-icon name="cluster-o" size="11" />
                  {{ item.model_name }}
                </span>
              </div>

              <!-- Footer: balance_after + time -->
              <div class="tx-card__footer">
                <span class="tx-card__balance">
                  余额 {{ formatUSD6(item.balance_after) }}
                </span>
                <span class="tx-card__time">
                  {{ formatDate(item.created_at) }}
                </span>
              </div>
            </div>
          </div>

          <!-- Empty state -->
          <div v-if="!txLoading && !transactions.length" class="empty-state">
            <van-icon name="orders-o" size="40" color="#cbd5e1" />
            <span>暂无交易记录</span>
          </div>
        </van-list>
      </van-pull-refresh>
    </div>

    <!-- ═══════ ADJUST DIALOG ═══════ -->
    <van-dialog
      v-model:show="showAdjustDialog"
      title="调整余额"
      show-cancel-button
      :confirm-button-color="'#0d9488'"
      :before-close="adjustSubmitting ? () => {} : undefined"
      @confirm="submitAdjust"
    >
      <div class="adjust-form">
        <!-- Direction toggle -->
        <div class="adjust-direction">
          <div
            class="direction-chip"
            :class="{ 'direction-chip--active direction-chip--recharge': adjustDirection === 'recharge' }"
            @click="adjustDirection = 'recharge'"
          >
            <van-icon name="add-o" size="14" />
            <span>充值</span>
          </div>
          <div
            class="direction-chip"
            :class="{ 'direction-chip--active direction-chip--deduct': adjustDirection === 'deduct' }"
            @click="adjustDirection = 'deduct'"
          >
            <van-icon name="minus" size="14" />
            <span>扣减</span>
          </div>
        </div>

        <div class="adjust-field">
          <label class="adjust-label">金额 (USD)</label>
          <van-field
            v-model="adjustAmount"
            type="number"
            placeholder="请输入金额"
            :border="false"
            class="adjust-input"
          />
        </div>

        <div class="adjust-field">
          <label class="adjust-label">说明</label>
          <van-field
            v-model="adjustDescription"
            type="textarea"
            placeholder="请输入调整说明"
            rows="2"
            :border="false"
            class="adjust-input"
          />
        </div>
      </div>
    </van-dialog>
  </div>
</template>

<style scoped>
.page {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(24px + env(safe-area-inset-bottom, 0px));
}

/* ═══════════════════════════════════════
   HERO — Amber Theme
   ═══════════════════════════════════════ */
.hero {
  position: relative;
  overflow: hidden;
  border-radius: 0 0 28px 28px;
  padding-bottom: 24px;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #78350f 0%, #b45309 30%, #f59e0b 65%, #d97706 100%);
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
  background: rgba(251, 191, 36, 0.25);
  top: -40px;
  right: -30px;
}
.hero-orb--2 {
  width: 150px;
  height: 150px;
  background: rgba(217, 119, 6, 0.2);
  bottom: -20px;
  left: -30px;
}

.hero-content {
  position: relative;
  z-index: 1;
  padding: 24px 20px 0;
  animation: fadeSlideUp 0.5s both;
}

.hero-title-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 18px;
}

.hero-title {
  font-size: 22px;
  font-weight: 700;
  color: #fff;
  margin: 0;
  letter-spacing: -0.02em;
}

/* ── Balance group ── */
.balance-group {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.balance-item--main {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.balance-row {
  display: flex;
  align-items: center;
  gap: 12px;
  background: rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 14px;
  padding: 12px 16px;
}

.balance-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.balance-label {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.55);
  font-weight: 500;
}

.balance-value {
  font-size: 22px;
  font-weight: 700;
  color: #fff;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.03em;
  line-height: 1.2;
}

.balance-item--main .balance-value {
  font-size: 30px;
}

.balance-value--positive {
  color: #86efac;
}

.balance-value--frozen {
  color: #fcd34d;
}

.balance-value--available {
  color: #5eead4;
}

.balance-divider {
  width: 1px;
  height: 28px;
  background: rgba(255, 255, 255, 0.15);
  flex-shrink: 0;
}

/* ── Hero Skeleton ── */
.hero-skeleton {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.skel-bar {
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.12);
  animation: pulse 1.5s ease-in-out infinite;
}

.skel-bar--xl {
  width: 55%;
  height: 30px;
}

.skel-bar--md {
  width: 80%;
  height: 44px;
  border-radius: 14px;
}

.skel-bar--sm {
  width: 40%;
  height: 14px;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

/* ═══════════════════════════════════════
   QUICK ACTION
   ═══════════════════════════════════════ */
.quick-action-wrap {
  padding: 0 16px;
  margin-top: -8px;
  position: relative;
  z-index: 2;
  animation: fadeSlideUp 0.45s 0.12s both;
}

.adjust-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 100%;
  padding: 12px 20px;
  border-radius: 14px;
  font-size: 13px;
  font-weight: 600;
  border: 1px solid #99f6e4;
  background: #f0fdfa;
  color: #0d9488;
  cursor: pointer;
  transition: all 0.2s;
  -webkit-tap-highlight-color: transparent;
  box-shadow: 0 2px 8px rgba(13, 148, 136, 0.08);
}

.adjust-btn:active {
  transform: scale(0.98);
  background: #ccfbf1;
}

/* ═══════════════════════════════════════
   TRANSACTION SECTION
   ═══════════════════════════════════════ */
.tx-section {
  padding-top: 16px;
  animation: fadeSlideUp 0.45s calc(0.2s + var(--si, 0) * 0.06s) both;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 20px 10px;
}

.section-dot {
  width: 8px;
  height: 8px;
  border-radius: 3px;
  background: var(--c);
  flex-shrink: 0;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ta-text-primary, #1e293b);
  flex: 1;
}

.section-count {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  background: var(--ta-bg-secondary, #f1f5f9);
  padding: 1px 7px;
  border-radius: 8px;
}

/* ═══════════════════════════════════════
   FILTERS
   ═══════════════════════════════════════ */
.filters {
  padding: 0 16px 8px;
  animation: fadeSlideUp 0.4s 0.06s both;
}

.filter-group {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.filter-group::-webkit-scrollbar {
  display: none;
}

.filter-chip {
  flex-shrink: 0;
  padding: 5px 14px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
  color: var(--ta-text-secondary, #64748b);
  background: var(--ta-bg-secondary, #f1f5f9);
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}

.filter-chip--active {
  color: #fff;
  background: #0d9488;
  box-shadow: 0 2px 8px rgba(13, 148, 136, 0.3);
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

.tx-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  box-shadow:
    0 1px 2px rgba(0, 0, 0, 0.03),
    0 2px 8px rgba(0, 0, 0, 0.04);
  animation: cardIn 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
  transition: transform 0.15s, box-shadow 0.15s;
}

.tx-card:active {
  transform: scale(0.98);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
}

/* ── Card: Head ── */
.tx-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}

.tx-card__badges {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.badge {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  line-height: 1.4;
  white-space: nowrap;
}

.badge--type {
  /* dynamic color from inline style */
}

.tx-card__amount {
  font-size: 16px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.02em;
  flex-shrink: 0;
}

/* ── Card: Description ── */
.tx-card__desc {
  font-size: 13px;
  font-weight: 500;
  color: var(--ta-text-primary, #1e293b);
  line-height: 1.5;
  margin-bottom: 6px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

/* ── Card: Meta tags ── */
.tx-card__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.tx-card__tag {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  background: var(--ta-bg-secondary, #f1f5f9);
  padding: 2px 8px;
  border-radius: 8px;
}

/* ── Card: Footer ── */
.tx-card__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.tx-card__balance {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-variant-numeric: tabular-nums;
}

.tx-card__time {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  flex-shrink: 0;
  white-space: nowrap;
}

/* ═══════════════════════════════════════
   EMPTY STATE
   ═══════════════════════════════════════ */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 48px 0 24px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-size: 13px;
}

/* ═══════════════════════════════════════
   ADJUST DIALOG
   ═══════════════════════════════════════ */
.adjust-form {
  padding: 12px 20px 4px;
}

.adjust-direction {
  display: flex;
  gap: 10px;
  margin-bottom: 16px;
}

.direction-chip {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 10px 0;
  border-radius: 12px;
  font-size: 13px;
  font-weight: 600;
  color: var(--ta-text-secondary, #64748b);
  background: var(--ta-bg-secondary, #f1f5f9);
  cursor: pointer;
  transition: all 0.2s;
  border: 2px solid transparent;
}

.direction-chip--active.direction-chip--recharge {
  color: #10b981;
  background: #ecfdf5;
  border-color: #10b981;
}

.direction-chip--active.direction-chip--deduct {
  color: #ef4444;
  background: #fef2f2;
  border-color: #ef4444;
}

.adjust-field {
  margin-bottom: 12px;
}

.adjust-label {
  display: block;
  font-size: 12px;
  font-weight: 600;
  color: var(--ta-text-secondary, #475569);
  margin-bottom: 6px;
}

.adjust-input {
  background: var(--ta-bg-secondary, #f8fafc);
  border-radius: 10px;
  padding: 0;
}

.adjust-input :deep(.van-field__control) {
  font-size: 14px;
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
</style>
