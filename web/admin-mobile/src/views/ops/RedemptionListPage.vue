<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { showToast, showConfirmDialog } from 'vant'
import request from '@/utils/request'

// ─── State ───
const activeTab = ref(0)

// Redemption list state
const loading = ref(false)
const refreshing = ref(false)
const finished = ref(false)
const items = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const totalCount = ref(0)
const activeCount = ref(0)

const activeStatus = ref('')
const activeType = ref('')

// Usage records state
const usageLoading = ref(false)
const usageRefreshing = ref(false)
const usageFinished = ref(false)
const usageItems = ref<any[]>([])
const usagePage = ref(1)
const usageTotal = ref(0)

// Form state
const showForm = ref(false)
const formLoading = ref(false)

const defaultForm = () => ({
  count: 10,
  type: 'quota',
  value: '',
  plan_id: '',
  duration_days: '',
  expires_at: '',
  max_uses: '',
})

const form = ref(defaultForm())

// ─── Filters ───
const statusFilters = [
  { key: '', label: '全部' },
  { key: 'active', label: '有效' },
  { key: 'used', label: '已使用' },
  { key: 'disabled', label: '已禁用' },
  { key: 'expired', label: '已过期' },
]

const typeFilters = [
  { key: '', label: '全部' },
  { key: 'quota', label: '额度' },
  { key: 'plan', label: '套餐' },
  { key: 'duration', label: '时长' },
]

// ─── Computed ───
const activeItemsCount = computed(() =>
  items.value.filter(i => i.status === 'active').length
)

// ─── Formatters ───
function typeLabel(type: string | undefined): string {
  if (type === 'quota') return '额度'
  if (type === 'plan') return '套餐'
  if (type === 'duration') return '时长'
  return type || '-'
}

function typeColor(type: string | undefined): string {
  if (type === 'quota') return '#10b981'
  if (type === 'plan') return '#3b82f6'
  if (type === 'duration') return '#8b5cf6'
  return '#94a3b8'
}

function typeBg(type: string | undefined): string {
  if (type === 'quota') return 'rgba(16,185,129,0.1)'
  if (type === 'plan') return 'rgba(59,130,246,0.1)'
  if (type === 'duration') return 'rgba(139,92,246,0.1)'
  return 'rgba(148,163,184,0.08)'
}

function statusLabel(status: string | undefined): string {
  if (status === 'active') return '有效'
  if (status === 'used') return '已使用'
  if (status === 'disabled') return '已禁用'
  if (status === 'expired') return '已过期'
  return status || '-'
}

function statusColor(status: string | undefined): string {
  if (status === 'active') return '#10b981'
  if (status === 'used') return '#94a3b8'
  if (status === 'disabled') return '#ef4444'
  if (status === 'expired') return '#f59e0b'
  return '#94a3b8'
}

function statusBg(status: string | undefined): string {
  if (status === 'active') return 'rgba(16,185,129,0.1)'
  if (status === 'used') return 'rgba(148,163,184,0.08)'
  if (status === 'disabled') return 'rgba(239,68,68,0.08)'
  if (status === 'expired') return 'rgba(245,158,11,0.08)'
  return 'rgba(148,163,184,0.08)'
}

function formatValue(item: any): string {
  if (item.type === 'quota') {
    if (item.value == null) return '-'
    return `$${Number(item.value).toFixed(2)}`
  }
  if (item.type === 'plan') {
    return item.plan_id ? `套餐#${item.plan_id}` : '-'
  }
  if (item.type === 'duration') {
    return item.duration_days ? `${item.duration_days}天` : '-'
  }
  return '-'
}

function formatTime(dateStr: string | undefined): string {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const hour = String(d.getHours()).padStart(2, '0')
  const min = String(d.getMinutes()).padStart(2, '0')
  return `${month}-${day} ${hour}:${min}`
}

function usagePercent(item: any): number {
  if (!item.max_uses || item.max_uses <= 0) return 0
  return Math.min((item.used_count || 0) / item.max_uses * 100, 100)
}

// ─── Data fetching ───
async function fetchRedemptions(append = false) {
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
    if (activeType.value) params.type = activeType.value

    const { data: res } = await request.get('/admin/redemptions', { params })
    const list = res.data?.list || []
    total.value = res.data?.total || 0

    if (append) {
      items.value = [...items.value, ...list]
    } else {
      items.value = list
    }
    finished.value = items.value.length >= total.value
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function fetchStats() {
  try {
    const [totalRes, activeRes] = await Promise.all([
      request.get('/admin/redemptions', { params: { page: 1, page_size: 1 } }),
      request.get('/admin/redemptions', { params: { page: 1, page_size: 1, status: 'active' } }),
    ])
    totalCount.value = totalRes.data?.data?.total || 0
    activeCount.value = activeRes.data?.data?.total || 0
  } catch {
    // non-critical
  }
}

async function fetchUsages(append = false) {
  if (!append) {
    usagePage.value = 1
    usageFinished.value = false
  }

  usageLoading.value = true
  try {
    const params: any = {
      page: usagePage.value,
      page_size: 20,
    }

    const { data: res } = await request.get('/admin/redemptions/usages', { params })
    const list = res.data?.list || []
    usageTotal.value = res.data?.total || 0

    if (append) {
      usageItems.value = [...usageItems.value, ...list]
    } else {
      usageItems.value = list
    }
    usageFinished.value = usageItems.value.length >= usageTotal.value
  } catch {
    // handled by interceptor
  } finally {
    usageLoading.value = false
    usageRefreshing.value = false
  }
}

// ─── Refresh / Load ───
async function onRefresh() {
  refreshing.value = true
  if (activeTab.value === 0) {
    await Promise.all([fetchRedemptions(false), fetchStats()])
  } else {
    usageRefreshing.value = true
    await fetchUsages(false)
  }
}

async function onLoad() {
  if (activeTab.value === 0) {
    if (finished.value) return
    page.value++
    await fetchRedemptions(true)
  } else {
    if (usageFinished.value) return
    usagePage.value++
    await fetchUsages(true)
  }
}

// ─── Filter handlers ───
function setStatus(key: string) {
  activeStatus.value = key
  fetchRedemptions(false)
}

function setType(key: string) {
  activeType.value = key
  fetchRedemptions(false)
}

function onTabChange(index: number) {
  if (index === 1 && !usageItems.value.length && !usageLoading.value) {
    fetchUsages(false)
  }
}

// ─── Form ───
function openCreate() {
  form.value = defaultForm()
  showForm.value = true
}

async function submitForm() {
  if (!form.value.count || form.value.count < 1 || form.value.count > 1000) {
    showToast('生成数量需在 1-1000 之间')
    return
  }
  if (!form.value.type) {
    showToast('请选择类型')
    return
  }
  if (form.value.type === 'quota' && !form.value.value) {
    showToast('请输入额度值')
    return
  }
  if (form.value.type === 'plan' && !form.value.plan_id) {
    showToast('请输入套餐 ID')
    return
  }
  if (form.value.type === 'duration' && !form.value.duration_days) {
    showToast('请输入天数')
    return
  }

  formLoading.value = true
  try {
    const body: any = {
      count: Number(form.value.count),
      type: form.value.type,
    }
    if (form.value.type === 'quota') {
      body.value = Number(form.value.value)
    }
    if (form.value.type === 'plan') {
      body.plan_id = Number(form.value.plan_id)
    }
    if (form.value.type === 'duration') {
      body.duration_days = Number(form.value.duration_days)
    }
    if (form.value.expires_at) {
      body.expires_at = form.value.expires_at
    }
    if (form.value.max_uses) {
      body.max_uses = Number(form.value.max_uses)
    }

    await request.post('/admin/redemptions', body)
    showToast('创建成功')
    showForm.value = false
    await Promise.all([fetchRedemptions(false), fetchStats()])
  } catch {
    // handled by interceptor
  } finally {
    formLoading.value = false
  }
}

async function disableCode(item: any) {
  try {
    await showConfirmDialog({
      title: '确认禁用',
      message: `确定要禁用兑换码「${item.code}」吗？`,
    })
    await request.put(`/admin/redemptions/${item.id}/disable`)
    showToast('已禁用')
    await Promise.all([fetchRedemptions(false), fetchStats()])
  } catch {
    // cancelled or error
  }
}

onMounted(() => {
  fetchStats()
  fetchRedemptions()
})
</script>

<template>
  <div class="redemption-page">

    <!-- ═══════ HERO — Pink Theme ═══════ -->
    <div class="hero">
      <div class="hero-bg">
        <div class="hero-orb hero-orb--1" />
        <div class="hero-orb hero-orb--2" />
      </div>
      <div class="hero-content">
        <div class="hero-label-row">
          <span class="hero-title">兑换码管理</span>
          <span class="hero-subtitle">Redemption Code</span>
        </div>
        <div class="hero-stats">
          <div class="hero-stat">
            <span class="hero-stat__value">{{ totalCount }}</span>
            <span class="hero-stat__label">兑换码总数</span>
          </div>
          <div class="hero-stat-divider" />
          <div class="hero-stat">
            <span class="hero-stat__value">{{ activeCount }}</span>
            <span class="hero-stat__label">有效数量</span>
          </div>
        </div>
      </div>
    </div>

    <!-- ═══════ TABS ═══════ -->
    <van-tabs
      v-model:active="activeTab"
      animated
      swipeable
      class="main-tabs"
      @change="onTabChange"
    >
      <!-- ──── Tab 1: 兑换码 ──── -->
      <van-tab title="兑换码">
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

        <!-- Type filter chips -->
        <div class="chip-scroll">
          <div
            v-for="t in typeFilters"
            :key="t.key"
            class="chip chip--type"
            :class="{ 'chip--active-type': activeType === t.key }"
            @click="setType(t.key)"
          >
            {{ t.label }}
          </div>
        </div>

        <!-- Count bar -->
        <div v-if="total > 0" class="count-bar">
          <span class="count-text">共 <b>{{ total }}</b> 个兑换码</span>
        </div>

        <!-- Redemption cards -->
        <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
          <van-list v-model:loading="loading" :finished="finished" finished-text="" @load="onLoad">
            <div class="card-list">
              <van-swipe-cell v-for="(item, idx) in items" :key="item.id">
                <div
                  class="redeem-card"
                  :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
                >
                  <!-- Top row: code + badges -->
                  <div class="redeem-card__top">
                    <div class="redeem-card__code">{{ item.code }}</div>
                    <div class="redeem-card__badges">
                      <span
                        class="redeem-card__type"
                        :style="{ color: typeColor(item.type), background: typeBg(item.type) }"
                      >
                        {{ typeLabel(item.type) }}
                      </span>
                      <span
                        class="redeem-card__status"
                        :style="{ color: statusColor(item.status), background: statusBg(item.status) }"
                      >
                        {{ statusLabel(item.status) }}
                      </span>
                    </div>
                  </div>

                  <!-- Value display -->
                  <div class="redeem-card__value">
                    <span class="value-label">面值</span>
                    <span class="value-num">{{ formatValue(item) }}</span>
                  </div>

                  <!-- Usage progress -->
                  <div v-if="item.max_uses" class="redeem-card__progress">
                    <div class="progress-info">
                      <span class="progress-label">使用进度</span>
                      <span class="progress-count">{{ item.used_count || 0 }} / {{ item.max_uses }}</span>
                    </div>
                    <div class="progress-bar">
                      <div
                        class="progress-bar__fill"
                        :style="{ width: `${usagePercent(item)}%` }"
                      />
                    </div>
                  </div>

                  <!-- Footer -->
                  <div class="redeem-card__footer">
                    <span v-if="item.batch_no" class="footer-tag">批次: {{ item.batch_no }}</span>
                    <span v-if="item.expires_at" class="footer-tag">到期: {{ formatTime(item.expires_at) }}</span>
                    <span class="footer-tag footer-tag--muted">{{ formatTime(item.created_at) }}</span>
                  </div>
                </div>

                <!-- Swipe action: disable -->
                <template #right>
                  <div class="swipe-actions">
                    <div
                      v-if="item.status === 'active'"
                      class="swipe-btn swipe-btn--danger"
                      @click.stop="disableCode(item)"
                    >
                      禁用
                    </div>
                  </div>
                </template>
              </van-swipe-cell>
            </div>

            <!-- Empty state -->
            <div v-if="!loading && !items.length" class="empty-state">
              <van-icon name="gift-o" size="42" color="#cbd5e1" />
              <span class="empty-text">暂无兑换码</span>
              <span class="empty-hint">点击右下角按钮批量生成</span>
            </div>
          </van-list>
        </van-pull-refresh>
      </van-tab>

      <!-- ──── Tab 2: 使用记录 ──── -->
      <van-tab title="使用记录">
        <div v-if="usageTotal > 0" class="count-bar">
          <span class="count-text">共 <b>{{ usageTotal }}</b> 条记录</span>
        </div>

        <van-pull-refresh v-model="usageRefreshing" @refresh="onRefresh">
          <van-list v-model:loading="usageLoading" :finished="usageFinished" finished-text="" @load="onLoad">
            <div class="card-list">
              <div
                v-for="(item, idx) in usageItems"
                :key="item.id"
                class="usage-card"
                :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
              >
                <div class="usage-card__top">
                  <span class="usage-card__code">{{ item.code }}</span>
                  <span
                    class="usage-card__type"
                    :style="{ color: typeColor(item.type), background: typeBg(item.type) }"
                  >
                    {{ typeLabel(item.type) }}
                  </span>
                </div>

                <div class="usage-card__details">
                  <div class="usage-detail">
                    <span class="usage-detail__label">租户</span>
                    <span class="usage-detail__val">{{ item.tenant_name || '-' }}</span>
                  </div>
                  <div class="usage-detail">
                    <span class="usage-detail__label">用户</span>
                    <span class="usage-detail__val">{{ item.username || '-' }}</span>
                  </div>
                  <div class="usage-detail">
                    <span class="usage-detail__label">面值</span>
                    <span class="usage-detail__val usage-detail__val--highlight">{{ formatValue(item) }}</span>
                  </div>
                </div>

                <div class="usage-card__footer">
                  <span class="footer-tag footer-tag--muted">{{ formatTime(item.created_at) }}</span>
                </div>
              </div>
            </div>

            <!-- Empty state -->
            <div v-if="!usageLoading && !usageItems.length" class="empty-state">
              <van-icon name="records" size="42" color="#cbd5e1" />
              <span class="empty-text">暂无使用记录</span>
            </div>
          </van-list>
        </van-pull-refresh>
      </van-tab>
    </van-tabs>

    <!-- Floating add button -->
    <div class="fab" @click="openCreate">
      <van-icon name="plus" size="24" color="#fff" />
    </div>

    <!-- ═══════ CREATE FORM POPUP ═══════ -->
    <van-popup
      v-model:show="showForm"
      position="bottom"
      round
      :style="{ maxHeight: '85vh' }"
      closeable
      close-icon="cross"
    >
      <div class="form-popup">
        <h3 class="form-title">批量生成兑换码</h3>

        <div class="form-body">
          <!-- Count -->
          <div class="form-group">
            <label class="form-label">生成数量 <span class="form-required">*</span></label>
            <van-field
              v-model="form.count"
              type="digit"
              placeholder="1-1000"
              :border="false"
              class="form-field"
            />
          </div>

          <!-- Type -->
          <div class="form-group">
            <label class="form-label">类型 <span class="form-required">*</span></label>
            <van-radio-group v-model="form.type" direction="horizontal" class="form-radio-group">
              <van-radio name="quota" checked-color="#10b981">额度</van-radio>
              <van-radio name="plan" checked-color="#3b82f6">套餐</van-radio>
              <van-radio name="duration" checked-color="#8b5cf6">时长</van-radio>
            </van-radio-group>
          </div>

          <!-- Value (for quota) -->
          <div v-if="form.type === 'quota'" class="form-group">
            <label class="form-label">额度值 (USD) <span class="form-required">*</span></label>
            <van-field
              v-model="form.value"
              type="number"
              placeholder="如 10.00"
              :border="false"
              class="form-field"
            />
          </div>

          <!-- Plan ID (for plan) -->
          <div v-if="form.type === 'plan'" class="form-group">
            <label class="form-label">套餐 ID <span class="form-required">*</span></label>
            <van-field
              v-model="form.plan_id"
              type="digit"
              placeholder="如 1"
              :border="false"
              class="form-field"
            />
          </div>

          <!-- Duration days (for duration) -->
          <div v-if="form.type === 'duration'" class="form-group">
            <label class="form-label">天数 <span class="form-required">*</span></label>
            <van-field
              v-model="form.duration_days"
              type="digit"
              placeholder="如 30"
              :border="false"
              class="form-field"
            />
          </div>

          <!-- Expires at -->
          <div class="form-group">
            <label class="form-label">过期时间</label>
            <van-field
              v-model="form.expires_at"
              placeholder="如 2026-12-31"
              :border="false"
              class="form-field"
            />
          </div>

          <!-- Max uses -->
          <div class="form-group">
            <label class="form-label">最大使用次数</label>
            <van-field
              v-model="form.max_uses"
              type="digit"
              placeholder="如 1"
              :border="false"
              class="form-field"
            />
          </div>
        </div>

        <!-- Submit button -->
        <div class="form-actions">
          <van-button
            block
            round
            type="primary"
            :loading="formLoading"
            loading-text="生成中..."
            class="form-submit"
            @click="submitForm"
          >
            批量生成
          </van-button>
        </div>
      </div>
    </van-popup>
  </div>
</template>

<style scoped>
.redemption-page {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(16px + env(safe-area-inset-bottom, 0px));
}

/* ═══════════════════════════════════════
   HERO — Pink Theme
   ═══════════════════════════════════════ */
.hero {
  position: relative;
  overflow: hidden;
  border-radius: 0 0 28px 28px;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #831843 0%, #db2777 40%, #ec4899 70%, #be185d 100%);
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
  background: rgba(244, 114, 182, 0.2);
  top: -30px;
  right: -20px;
}
.hero-orb--2 {
  width: 140px;
  height: 140px;
  background: rgba(236, 72, 153, 0.15);
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

.hero-stats {
  display: flex;
  align-items: center;
  gap: 0;
  background: rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 14px;
  padding: 14px 16px;
}

.hero-stat {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  align-items: center;
}

.hero-stat__value {
  font-size: 28px;
  font-weight: 800;
  color: #fff;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.03em;
  line-height: 1.2;
}

.hero-stat__label {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.55);
  font-weight: 500;
}

.hero-stat-divider {
  width: 1px;
  height: 32px;
  background: rgba(255, 255, 255, 0.15);
  flex-shrink: 0;
}

/* ── Tabs ── */
.main-tabs :deep(.van-tabs__wrap) {
  background: #fff;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.04);
}

.main-tabs :deep(.van-tab--active) {
  color: #ec4899;
  font-weight: 600;
}

.main-tabs :deep(.van-tabs__line) {
  background: #ec4899;
  width: 32px;
  border-radius: 2px;
}

/* ── Status filter chips ── */
.chip-scroll {
  display: flex;
  gap: 8px;
  padding: 12px 16px 8px;
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
  background: #ec4899;
  box-shadow: 0 2px 8px rgba(236, 72, 153, 0.3);
}
.chip--active-type {
  color: #fff;
  background: #0d9488;
  box-shadow: 0 2px 8px rgba(13, 148, 136, 0.3);
}

/* ── Count Bar ── */
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

/* ── Card List ── */
.card-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 12px;
}

/* ── Redemption Card ── */
.redeem-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  transition: transform 0.15s, box-shadow 0.15s;
  animation: fadeSlideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
}
.redeem-card:active {
  transform: scale(0.98);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
}

.redeem-card__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.redeem-card__code {
  font-size: 16px;
  font-weight: 700;
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  color: var(--ta-text-primary, #0f172a);
  letter-spacing: -0.01em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.redeem-card__badges {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.redeem-card__type,
.redeem-card__status {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  line-height: 1.4;
  white-space: nowrap;
}

/* ── Value display ── */
.redeem-card__value {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}

.value-label {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
}

.value-num {
  font-size: 14px;
  font-weight: 700;
  color: var(--ta-text-primary, #1e293b);
  font-variant-numeric: tabular-nums;
}

/* ── Usage progress ── */
.redeem-card__progress {
  margin-bottom: 8px;
}

.progress-info {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}

.progress-label {
  font-size: 10px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
}

.progress-count {
  font-size: 10px;
  color: var(--ta-text-secondary, #475569);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.progress-bar {
  height: 4px;
  background: var(--ta-bg-secondary, #f1f5f9);
  border-radius: 2px;
  overflow: hidden;
}

.progress-bar__fill {
  height: 100%;
  background: linear-gradient(90deg, #0d9488, #14b8a6);
  border-radius: 2px;
  transition: width 0.3s ease;
}

/* ── Card footer ── */
.redeem-card__footer {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.footer-tag {
  font-size: 10px;
  color: var(--ta-text-tertiary, #64748b);
  font-weight: 500;
}

.footer-tag--muted {
  color: var(--ta-text-quaternary, #cbd5e1);
  margin-left: auto;
}

/* ── Usage Card (Tab 2) ── */
.usage-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  animation: fadeSlideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
  transition: transform 0.15s;
}
.usage-card:active {
  transform: scale(0.98);
}

.usage-card__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 10px;
}

.usage-card__code {
  font-size: 14px;
  font-weight: 700;
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  color: var(--ta-text-primary, #0f172a);
  letter-spacing: -0.01em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.usage-card__type {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  line-height: 1.4;
  white-space: nowrap;
  flex-shrink: 0;
}

.usage-card__details {
  display: flex;
  align-items: center;
  background: var(--ta-bg-secondary, #f8fafc);
  border-radius: 8px;
  padding: 8px 12px;
  margin-bottom: 8px;
}

.usage-detail {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.usage-detail__label {
  font-size: 10px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
}

.usage-detail__val {
  font-size: 13px;
  font-weight: 600;
  color: var(--ta-text-primary, #1e293b);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.usage-detail__val--highlight {
  color: #0d9488;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.usage-card__footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
}

/* ── Swipe Actions ── */
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
  width: 64px;
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  cursor: pointer;
  writing-mode: vertical-lr;
  letter-spacing: 0.05em;
}
.swipe-btn--danger {
  background: #ef4444;
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

/* ── FAB ── */
.fab {
  position: fixed;
  right: 20px;
  bottom: calc(24px + env(safe-area-inset-bottom, 0px));
  width: 52px;
  height: 52px;
  border-radius: 16px;
  background: linear-gradient(135deg, #ec4899, #db2777);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 14px rgba(236, 72, 153, 0.4), 0 2px 6px rgba(0, 0, 0, 0.1);
  cursor: pointer;
  z-index: 100;
  transition: transform 0.2s, box-shadow 0.2s;
}
.fab:active {
  transform: scale(0.92);
  box-shadow: 0 2px 8px rgba(236, 72, 153, 0.3);
}

/* ═══════════════════════════════════════
   FORM POPUP
   ═══════════════════════════════════════ */
.form-popup {
  padding: 20px 16px calc(16px + env(safe-area-inset-bottom, 0px));
}

.form-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--ta-text-primary, #0f172a);
  margin: 0 0 16px;
  text-align: center;
  letter-spacing: -0.01em;
}

.form-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 60vh;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  padding-right: 4px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--ta-text-secondary, #475569);
  padding-left: 2px;
}

.form-required {
  color: #ef4444;
}

.form-field {
  background: var(--ta-bg-secondary, #f8fafc);
  border-radius: 10px;
  padding: 4px 8px;
}

.form-field :deep(.van-field__control) {
  font-size: 14px;
  color: var(--ta-text-primary, #0f172a);
}

.form-radio-group {
  display: flex;
  gap: 16px;
  padding: 8px 0 4px;
}

.form-radio-group :deep(.van-radio__label) {
  font-size: 14px;
  color: var(--ta-text-primary, #0f172a);
}

.form-actions {
  margin-top: 16px;
}

.form-submit {
  height: 44px;
  font-size: 15px;
  font-weight: 600;
  background: linear-gradient(135deg, #ec4899, #db2777);
  border: none;
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
