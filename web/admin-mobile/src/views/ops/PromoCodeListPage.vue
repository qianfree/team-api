<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { showToast, showConfirmDialog } from 'vant'
import request from '@/utils/request'

// ═══════ TAB STATE ═══════
const activeTab = ref(0)

// ═══════ PROMO CODE STATE ═══════
const loading = ref(false)
const refreshing = ref(false)
const finished = ref(false)
const promoCodes = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const activeStatus = ref('')

// ═══════ EXPAND STATE ═══════
const expandedId = ref<number | null>(null)

// ═══════ FORM STATE ═══════
const showForm = ref(false)
const formLoading = ref(false)

const defaultForm = () => ({
  code: '',
  name: '',
  type: 'percentage' as 'percentage' | 'fixed',
  discount_value: '',
  min_amount: '',
  max_discount: '',
  total_count: '',
  per_user_limit: '',
  valid_from: '',
  valid_to: '',
  plan_ids: '',
  status: true,
})

const form = ref(defaultForm())
const showDatePicker = ref(false)
const datePickerTarget = ref<'valid_from' | 'valid_to'>('valid_from')
const currentDate = ref(new Date().toISOString())

// ═══════ USAGE RECORDS STATE ═══════
const usages = ref<any[]>([])
const usageLoading = ref(false)
const usageRefreshing = ref(false)
const usageFinished = ref(false)
const usagePage = ref(1)
const usageTotal = ref(0)

// ═══════ COMPUTED ═══════
const activeCodesCount = computed(() =>
  promoCodes.value.filter(p => p.status === 'active').length
)

// ═══════ STATUS FILTERS ═══════
const statusFilters = [
  { key: '', label: '全部' },
  { key: 'active', label: '启用' },
  { key: 'inactive', label: '禁用' },
  { key: 'expired', label: '已过期' },
]

// ═══════ HELPERS ═══════
function formatDate(dateStr: string | undefined | null): string {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function formatDateTime(dateStr: string | undefined | null): string {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const h = String(d.getHours()).padStart(2, '0')
  const min = String(d.getMinutes()).padStart(2, '0')
  return `${m}-${day} ${h}:${min}`
}

function statusColor(status: string): string {
  const map: Record<string, string> = {
    active: '#10b981',
    inactive: '#ef4444',
    expired: '#f59e0b',
  }
  return map[status] || '#64748b'
}

function statusBg(status: string): string {
  const map: Record<string, string> = {
    active: 'rgba(16,185,129,0.1)',
    inactive: 'rgba(239,68,68,0.1)',
    expired: 'rgba(245,158,11,0.1)',
  }
  return map[status] || 'rgba(100,116,139,0.1)'
}

function statusLabel(status: string): string {
  const map: Record<string, string> = {
    active: '启用',
    inactive: '禁用',
    expired: '已过期',
  }
  return map[status] || status || '-'
}

function discountDisplay(item: any): { text: string; color: string } {
  if (item.type === 'percentage') {
    return { text: `${Number(item.discount_value || 0)}% OFF`, color: '#8b5cf6' }
  }
  return { text: `¥${Number(item.discount_value || 0).toFixed(2)} 减免`, color: '#0d9488' }
}

function usageProgress(item: any): number {
  const total = Number(item.total_count || 0)
  const used = Number(item.used_count || 0)
  if (total <= 0) return 0
  return Math.min(Math.round((used / total) * 100), 100)
}

function generateCode(): string {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789'
  let code = ''
  for (let i = 0; i < 8; i++) {
    code += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  return code
}

// ═══════ FETCH PROMO CODES ═══════
async function fetchPromoCodes(append = false) {
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

    const { data: res } = await request.get('/admin/promo-codes', { params })
    const list = res.data?.list || []
    total.value = res.data?.total || 0

    if (append) {
      promoCodes.value = [...promoCodes.value, ...list]
    } else {
      promoCodes.value = list
    }
    finished.value = promoCodes.value.length >= total.value
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

// ═══════ FETCH USAGE RECORDS ═══════
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

    const { data: res } = await request.get('/admin/promo-codes/usages', { params })
    const list = res.data?.list || []
    usageTotal.value = res.data?.total || 0

    if (append) {
      usages.value = [...usages.value, ...list]
    } else {
      usages.value = list
    }
    usageFinished.value = usages.value.length >= usageTotal.value
  } catch {
    // handled by interceptor
  } finally {
    usageLoading.value = false
    usageRefreshing.value = false
  }
}

// ═══════ PROMO CODE HANDLERS ═══════
async function onRefresh() {
  refreshing.value = true
  await fetchPromoCodes(false)
}

async function onLoad() {
  if (finished.value) return
  page.value++
  await fetchPromoCodes(true)
}

function setStatus(key: string) {
  activeStatus.value = key
  fetchPromoCodes(false)
}

function toggleExpand(id: number) {
  expandedId.value = expandedId.value === id ? null : id
}

function openCreate() {
  form.value = defaultForm()
  showForm.value = true
}

function autoGenerateCode() {
  form.value.code = generateCode()
}

async function submitForm() {
  if (!form.value.code.trim()) {
    showToast('请输入优惠码')
    return
  }
  if (!form.value.name.trim()) {
    showToast('请输入名称')
    return
  }
  if (!form.value.discount_value || Number(form.value.discount_value) <= 0) {
    showToast('请输入有效的优惠值')
    return
  }

  formLoading.value = true
  try {
    const planIds = form.value.plan_ids
      ? form.value.plan_ids.split(',').map((s: string) => Number(s.trim())).filter((n: number) => !isNaN(n) && n > 0)
      : []

    const body: any = {
      code: form.value.code.trim().toUpperCase(),
      name: form.value.name.trim(),
      type: form.value.type,
      discount_value: Number(form.value.discount_value),
      min_amount: form.value.min_amount ? Number(form.value.min_amount) : 0,
      max_discount: form.value.max_discount ? Number(form.value.max_discount) : 0,
      total_count: form.value.total_count ? Number(form.value.total_count) : 0,
      per_user_limit: form.value.per_user_limit ? Number(form.value.per_user_limit) : 0,
      valid_from: form.value.valid_from || '',
      valid_to: form.value.valid_to || '',
      plan_ids: planIds,
      status: form.value.status ? 'active' : 'inactive',
    }

    await request.post('/admin/promo-codes', body)
    showToast('创建成功')
    showForm.value = false
    await fetchPromoCodes(false)
  } catch {
    // handled by interceptor
  } finally {
    formLoading.value = false
  }
}

async function toggleStatus(item: any) {
  const newStatus = item.status === 'active' ? 'inactive' : 'active'
  const actionText = newStatus === 'active' ? '启用' : '禁用'
  try {
    await showConfirmDialog({
      title: `确认${actionText}`,
      message: `确定要${actionText}优惠码「${item.code}」吗？`,
    })
    await request.put(`/admin/promo-codes/${item.id}`, {
      update: { status: newStatus },
    })
    showToast(`已${actionText}`)
    await fetchPromoCodes(false)
  } catch {
    // cancelled or error
  }
}

// ═══════ DATE PICKER ═══════
function openDatePicker(target: 'valid_from' | 'valid_to') {
  datePickerTarget.value = target
  const current = form.value[target]
  if (current) {
    currentDate.value = new Date(current).toISOString()
  } else {
    currentDate.value = new Date().toISOString()
  }
  showDatePicker.value = true
}

function onDateConfirm({ selectedValues }: any) {
  const [year, month, day] = selectedValues
  const dateStr = `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`
  form.value[datePickerTarget.value] = dateStr
  showDatePicker.value = false
}

// ═══════ USAGE HANDLERS ═══════
async function onUsageRefresh() {
  usageRefreshing.value = true
  await fetchUsages(false)
}

async function onUsageLoad() {
  if (usageFinished.value) return
  usagePage.value++
  await fetchUsages(true)
}

// ═══════ TAB CHANGE ═══════
function onTabChange({ name }: any) {
  if (name === 1 && usages.value.length === 0) {
    fetchUsages(false)
  }
}

// ═══════ EXPORT ═══════
async function handleExport() {
  try {
    const { data: res } = await request.get('/admin/promo-codes/export', {
      responseType: 'blob',
    })
    const url = window.URL.createObjectURL(new Blob([res]))
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', 'promo-codes.csv')
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.URL.revokeObjectURL(url)
    showToast('导出成功')
  } catch {
    // handled by interceptor
  }
}

onMounted(() => fetchPromoCodes())
</script>

<template>
  <div class="promo-page">

    <!-- ═══════ HERO ═══════ -->
    <div class="hero">
      <div class="hero-bg">
        <div class="hero-orb hero-orb--1" />
        <div class="hero-orb hero-orb--2" />
      </div>
      <div class="hero-content">
        <div class="hero-label-row">
          <span class="hero-title">优惠码管理</span>
          <span class="hero-subtitle">Promo Code</span>
        </div>
        <div class="hero-stats">
          <div class="hero-stat">
            <span class="hero-stat__value">{{ total }}</span>
            <span class="hero-stat__label">优惠码总数</span>
          </div>
          <div class="hero-stat-divider" />
          <div class="hero-stat">
            <span class="hero-stat__value">{{ activeCodesCount }}</span>
            <span class="hero-stat__label">启用中</span>
          </div>
        </div>
      </div>
    </div>

    <!-- ═══════ TABS ═══════ -->
    <van-tabs
      v-model:active="activeTab"
      :line-width="28"
      :line-height="3"
      color="#8b5cf6"
      title-active-color="#8b5cf6"
      title-inactive-color="#94a3b8"
      class="main-tabs"
      sticky
      offset-top="0"
      @change="onTabChange"
    >
      <!-- ═══════ TAB: 优惠码 ═══════ -->
      <van-tab title="优惠码" :badge="total > 0 ? String(total) : ''">

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
          <span class="count-text">共 <b>{{ total }}</b> 个优惠码</span>
        </div>

        <!-- Promo Code Cards -->
        <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
          <van-list v-model:loading="loading" :finished="finished" finished-text="" @load="onLoad">
            <div class="card-list">
              <van-swipe-cell v-for="(item, idx) in promoCodes" :key="item.id">
                <div
                  class="promo-card"
                  :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
                  @click="toggleExpand(item.id)"
                >
                  <!-- Top row: code (monospace) + status badge -->
                  <div class="promo-card__top">
                    <span class="promo-card__code">{{ item.code }}</span>
                    <span
                      class="promo-card__status"
                      :style="{
                        color: statusColor(item.status),
                        background: statusBg(item.status),
                      }"
                    >
                      {{ statusLabel(item.status) }}
                    </span>
                  </div>

                  <!-- Name -->
                  <div class="promo-card__name">{{ item.name || '-' }}</div>

                  <!-- Discount display -->
                  <div class="promo-card__discount">
                    <span
                      class="promo-card__discount-val"
                      :style="{ color: discountDisplay(item).color }"
                    >
                      {{ discountDisplay(item).text }}
                    </span>
                  </div>

                  <!-- Detail row -->
                  <div class="promo-card__details">
                    <!-- Min amount -->
                    <div v-if="item.min_amount && Number(item.min_amount) > 0" class="detail-tag">
                      最低 ¥{{ Number(item.min_amount).toFixed(2) }}
                    </div>
                    <!-- Max discount -->
                    <div v-if="item.max_discount && Number(item.max_discount) > 0" class="detail-tag">
                      最高优惠 ¥{{ Number(item.max_discount).toFixed(2) }}
                    </div>
                  </div>

                  <!-- Usage progress -->
                  <div class="promo-card__progress-row">
                    <div class="promo-card__progress-text">
                      已使用 {{ item.used_count || 0 }} / {{ item.total_count || 0 }}
                    </div>
                    <div class="promo-card__progress-bar">
                      <div
                        class="promo-card__progress-fill"
                        :style="{ width: `${usageProgress(item)}%` }"
                      />
                    </div>
                  </div>

                  <!-- Valid period -->
                  <div class="promo-card__period">
                    <van-icon name="clock-o" size="11" />
                    <span>{{ formatDate(item.valid_from) }} ~ {{ formatDate(item.valid_to) }}</span>
                  </div>

                  <!-- Expanded content -->
                  <transition name="expand">
                    <div v-if="expandedId === item.id" class="promo-card__expanded">
                      <div class="expanded-row">
                        <span class="expanded-label">每人限用</span>
                        <span class="expanded-value">{{ item.per_user_limit || '不限' }} 次</span>
                      </div>
                      <div v-if="item.plan_ids && item.plan_ids.length" class="expanded-row">
                        <span class="expanded-label">适用套餐</span>
                        <span class="expanded-value">{{ item.plan_ids.join(', ') }}</span>
                      </div>
                      <div class="expanded-row">
                        <span class="expanded-label">创建时间</span>
                        <span class="expanded-value">{{ formatDateTime(item.created_at) }}</span>
                      </div>
                    </div>
                  </transition>
                </div>

                <!-- Swipe actions -->
                <template #right>
                  <div class="swipe-actions">
                    <div
                      class="swipe-btn"
                      :class="item.status === 'active' ? 'swipe-btn--danger' : 'swipe-btn--success'"
                      @click.stop="toggleStatus(item)"
                    >
                      {{ item.status === 'active' ? '禁用' : '启用' }}
                    </div>
                  </div>
                </template>
              </van-swipe-cell>
            </div>

            <!-- Empty state -->
            <div v-if="!loading && !promoCodes.length" class="empty-state">
              <van-icon name="coupon-o" size="42" color="#cbd5e1" />
              <span class="empty-text">暂无优惠码</span>
              <span class="empty-hint">点击右下角按钮创建第一个优惠码</span>
            </div>
          </van-list>
        </van-pull-refresh>
      </van-tab>

      <!-- ═══════ TAB: 使用记录 ═══════ -->
      <van-tab title="使用记录" :badge="usageTotal > 0 ? String(usageTotal) : ''">
        <van-pull-refresh v-model="usageRefreshing" @refresh="onUsageRefresh">
          <van-list v-model:loading="usageLoading" :finished="usageFinished" finished-text="" @load="onUsageLoad">
            <div class="card-list">
              <div
                v-for="(item, idx) in usages"
                :key="item.id"
                class="usage-card"
                :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
              >
                <!-- Header -->
                <div class="usage-card__head">
                  <div class="usage-card__id-group">
                    <span class="usage-card__label">优惠码 ID</span>
                    <span class="usage-card__val usage-card__val--mono">{{ item.promo_code_id }}</span>
                  </div>
                  <span class="usage-card__amount">¥{{ Number(item.discount_amount || 0).toFixed(2) }}</span>
                </div>

                <!-- Meta row -->
                <div class="usage-card__meta">
                  <div class="usage-card__meta-item">
                    <span class="usage-card__label">租户</span>
                    <span class="usage-card__val">{{ item.tenant_id || '-' }}</span>
                  </div>
                  <div class="usage-card__meta-item">
                    <span class="usage-card__label">订单</span>
                    <span class="usage-card__val usage-card__val--mono">{{ item.order_id || '-' }}</span>
                  </div>
                </div>

                <!-- Footer -->
                <div class="usage-card__footer">
                  <span class="usage-card__label">用户</span>
                  <span class="usage-card__val">{{ item.user_id || '-' }}</span>
                  <span class="usage-card__time">{{ formatDateTime(item.created_at) }}</span>
                </div>
              </div>
            </div>

            <!-- Empty state -->
            <div v-if="!usageLoading && !usages.length" class="empty-state">
              <van-icon name="orders-o" size="42" color="#cbd5e1" />
              <span class="empty-text">暂无使用记录</span>
              <span class="empty-hint">优惠码被使用后将在此显示</span>
            </div>
          </van-list>
        </van-pull-refresh>
      </van-tab>
    </van-tabs>

    <!-- Floating add button -->
    <div v-if="activeTab === 0" class="fab" @click="openCreate">
      <van-icon name="plus" size="24" color="#fff" />
    </div>

    <!-- ═══════ CREATE FORM POPUP ═══════ -->
    <van-popup
      v-model:show="showForm"
      position="bottom"
      round
      :style="{ maxHeight: '88vh' }"
      closeable
      close-icon="cross"
    >
      <div class="form-popup">
        <h3 class="form-title">创建优惠码</h3>

        <div class="form-body">
          <!-- Code -->
          <div class="form-group">
            <label class="form-label">优惠码 <span class="form-required">*</span></label>
            <div class="form-field-with-action">
              <van-field
                v-model="form.code"
                placeholder="如：SUMMER2025"
                :border="false"
                class="form-field form-field--mono"
              />
              <button class="field-action-btn" @click="autoGenerateCode">自动生成</button>
            </div>
          </div>

          <!-- Name -->
          <div class="form-group">
            <label class="form-label">名称 <span class="form-required">*</span></label>
            <van-field
              v-model="form.name"
              placeholder="如：夏季促销"
              :border="false"
              class="form-field"
            />
          </div>

          <!-- Type -->
          <div class="form-group">
            <label class="form-label">优惠类型</label>
            <van-radio-group v-model="form.type" direction="horizontal" class="form-radio-group">
              <van-radio name="percentage" checked-color="#8b5cf6">百分比</van-radio>
              <van-radio name="fixed" checked-color="#8b5cf6">固定金额</van-radio>
            </van-radio-group>
          </div>

          <!-- Discount value -->
          <div class="form-group">
            <label class="form-label">{{ form.type === 'percentage' ? '折扣百分比 (%)' : '优惠金额 (¥)' }}</label>
            <van-field
              v-model="form.discount_value"
              type="number"
              :placeholder="form.type === 'percentage' ? '如：20' : '如：50.00'"
              :border="false"
              class="form-field"
            />
          </div>

          <!-- Min amount + Max discount row -->
          <div class="form-row">
            <div class="form-group form-group--half">
              <label class="form-label">最低消费 (¥)</label>
              <van-field
                v-model="form.min_amount"
                type="number"
                placeholder="0"
                :border="false"
                class="form-field"
              />
            </div>
            <div class="form-group form-group--half">
              <label class="form-label">最高优惠 (¥)</label>
              <van-field
                v-model="form.max_discount"
                type="number"
                placeholder="0"
                :border="false"
                class="form-field"
              />
            </div>
          </div>

          <!-- Total count + Per user limit row -->
          <div class="form-row">
            <div class="form-group form-group--half">
              <label class="form-label">总发行量</label>
              <van-field
                v-model="form.total_count"
                type="digit"
                placeholder="0"
                :border="false"
                class="form-field"
              />
            </div>
            <div class="form-group form-group--half">
              <label class="form-label">每人限用</label>
              <van-field
                v-model="form.per_user_limit"
                type="digit"
                placeholder="0"
                :border="false"
                class="form-field"
              />
            </div>
          </div>

          <!-- Valid period -->
          <div class="form-row">
            <div class="form-group form-group--half">
              <label class="form-label">开始日期</label>
              <div class="form-field-readonly" @click="openDatePicker('valid_from')">
                <span :class="{ 'placeholder': !form.valid_from }">
                  {{ form.valid_from || '选择日期' }}
                </span>
                <van-icon name="calendar-o" size="16" color="#94a3b8" />
              </div>
            </div>
            <div class="form-group form-group--half">
              <label class="form-label">结束日期</label>
              <div class="form-field-readonly" @click="openDatePicker('valid_to')">
                <span :class="{ 'placeholder': !form.valid_to }">
                  {{ form.valid_to || '选择日期' }}
                </span>
                <van-icon name="calendar-o" size="16" color="#94a3b8" />
              </div>
            </div>
          </div>

          <!-- Plan IDs -->
          <div class="form-group">
            <label class="form-label">适用套餐 ID（逗号分隔）</label>
            <van-field
              v-model="form.plan_ids"
              placeholder="如：1,2,3（留空为全部）"
              :border="false"
              class="form-field"
            />
          </div>

          <!-- Status switch -->
          <div class="form-switch-row">
            <span class="form-label">启用状态</span>
            <van-switch v-model="form.status" size="20" active-color="#8b5cf6" />
          </div>
        </div>

        <!-- Submit button -->
        <div class="form-actions">
          <van-button
            block
            round
            type="primary"
            :loading="formLoading"
            loading-text="创建中..."
            class="form-submit"
            @click="submitForm"
          >
            创建优惠码
          </van-button>
        </div>
      </div>
    </van-popup>

    <!-- ═══════ DATE PICKER POPUP ═══════ -->
    <van-popup v-model:show="showDatePicker" position="bottom" round>
      <van-date-picker
        title="选择日期"
        :min-date="new Date(2024, 0, 1)"
        :max-date="new Date(2030, 11, 31)"
        @confirm="onDateConfirm"
        @cancel="showDatePicker = false"
      />
    </van-popup>
  </div>
</template>

<style scoped>
.promo-page {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(16px + env(safe-area-inset-bottom, 0px));
}

/* ═══════════════════════════════════════
   HERO — Purple Theme
   ═══════════════════════════════════════ */
.hero {
  position: relative;
  overflow: hidden;
  border-radius: 0 0 28px 28px;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #3b1f6e 0%, #7c3aed 40%, #8b5cf6 70%, #6d28d9 100%);
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
  background: rgba(167, 139, 250, 0.2);
  top: -30px;
  right: -20px;
}
.hero-orb--2 {
  width: 140px;
  height: 140px;
  background: rgba(139, 92, 246, 0.15);
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

/* ═══════════════════════════════════════
   TABS
   ═══════════════════════════════════════ */
.main-tabs :deep(.van-tabs__wrap) {
  background: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
}

.main-tabs :deep(.van-tab__text) {
  font-size: 14px;
  font-weight: 600;
}

.main-tabs :deep(.van-info) {
  background: #8b5cf6;
  font-size: 9px;
}

/* ── Status Filter Chips ── */
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
  background: #8b5cf6;
  box-shadow: 0 2px 8px rgba(139, 92, 246, 0.3);
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

/* ═══════════════════════════════════════
   CARD LIST
   ═══════════════════════════════════════ */
.card-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 12px;
}

/* ── Promo Code Card ── */
.promo-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;
  animation: fadeSlideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
}
.promo-card:active {
  transform: scale(0.98);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
}

/* Card: Top row — code + status */
.promo-card__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 2px;
}

.promo-card__code {
  font-size: 18px;
  font-weight: 800;
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  color: var(--ta-text-primary, #0f172a);
  letter-spacing: 0.02em;
  line-height: 1.3;
}

.promo-card__status {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  line-height: 1.4;
  white-space: nowrap;
}

/* Card: Name */
.promo-card__name {
  font-size: 13px;
  color: var(--ta-text-tertiary, #64748b);
  margin-bottom: 10px;
  font-weight: 500;
}

/* Card: Discount display */
.promo-card__discount {
  margin-bottom: 10px;
}

.promo-card__discount-val {
  font-size: 16px;
  font-weight: 800;
  letter-spacing: -0.02em;
}

/* Card: Detail tags */
.promo-card__details {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 10px;
}

.detail-tag {
  font-size: 11px;
  font-weight: 500;
  color: var(--ta-text-secondary, #475569);
  background: var(--ta-bg-secondary, #f1f5f9);
  padding: 3px 10px;
  border-radius: 8px;
  white-space: nowrap;
}

/* Card: Usage progress */
.promo-card__progress-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.promo-card__progress-text {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
  white-space: nowrap;
  flex-shrink: 0;
}

.promo-card__progress-bar {
  flex: 1;
  height: 4px;
  background: var(--ta-bg-secondary, #e2e8f0);
  border-radius: 2px;
  overflow: hidden;
}

.promo-card__progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #8b5cf6, #a78bfa);
  border-radius: 2px;
  transition: width 0.3s ease;
  min-width: 0;
}

/* Card: Valid period */
.promo-card__period {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
}

/* Card: Expanded content */
.promo-card__expanded {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--ta-border-light, #f1f5f9);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.expanded-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.expanded-label {
  font-size: 12px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
}

.expanded-value {
  font-size: 12px;
  color: var(--ta-text-primary, #1e293b);
  font-weight: 600;
}

/* Expand transition */
.expand-enter-active,
.expand-leave-active {
  transition: all 0.25s ease;
  overflow: hidden;
}
.expand-enter-from,
.expand-leave-to {
  opacity: 0;
  max-height: 0;
  margin-top: 0;
  padding-top: 0;
}
.expand-enter-to,
.expand-leave-from {
  opacity: 1;
  max-height: 200px;
}

/* ═══════════════════════════════════════
   USAGE RECORD CARD
   ═══════════════════════════════════════ */
.usage-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  animation: fadeSlideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
}

.usage-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 10px;
}

.usage-card__id-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.usage-card__amount {
  font-size: 16px;
  font-weight: 800;
  color: #8b5cf6;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.02em;
}

.usage-card__meta {
  display: flex;
  gap: 16px;
  margin-bottom: 8px;
}

.usage-card__meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.usage-card__label {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
}

.usage-card__val {
  font-size: 12px;
  color: var(--ta-text-primary, #1e293b);
  font-weight: 600;
}

.usage-card__val--mono {
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  letter-spacing: -0.02em;
  font-size: 11px;
}

.usage-card__footer {
  display: flex;
  align-items: center;
  gap: 4px;
}

.usage-card__time {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  margin-left: auto;
  white-space: nowrap;
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

.swipe-btn--success {
  background: #10b981;
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

/* ── FAB ── */
.fab {
  position: fixed;
  right: 20px;
  bottom: calc(24px + env(safe-area-inset-bottom, 0px));
  width: 52px;
  height: 52px;
  border-radius: 16px;
  background: linear-gradient(135deg, #8b5cf6, #7c3aed);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 14px rgba(139, 92, 246, 0.4), 0 2px 6px rgba(0, 0, 0, 0.1);
  cursor: pointer;
  z-index: 100;
  transition: transform 0.2s, box-shadow 0.2s;
}
.fab:active {
  transform: scale(0.92);
  box-shadow: 0 2px 8px rgba(139, 92, 246, 0.3);
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
  max-height: 62vh;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  padding-right: 4px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-group--half {
  flex: 1;
}

.form-row {
  display: flex;
  gap: 12px;
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

.form-field--mono :deep(.van-field__control) {
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  letter-spacing: -0.02em;
  text-transform: uppercase;
}

/* Field with action button */
.form-field-with-action {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--ta-bg-secondary, #f8fafc);
  border-radius: 10px;
  padding-right: 4px;
}

.form-field-with-action :deep(.van-field) {
  flex: 1;
  background: transparent;
  padding: 4px 8px;
}

.field-action-btn {
  flex-shrink: 0;
  font-size: 11px;
  font-weight: 600;
  color: #8b5cf6;
  background: rgba(139, 92, 246, 0.08);
  border: none;
  padding: 6px 10px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.2s;
  white-space: nowrap;
}
.field-action-btn:active {
  background: rgba(139, 92, 246, 0.16);
}

/* Radio group */
.form-radio-group {
  display: flex;
  gap: 16px;
  padding: 8px 4px;
}

.form-radio-group :deep(.van-radio__label) {
  font-size: 13px;
  color: var(--ta-text-primary, #1e293b);
  font-weight: 500;
}

/* Readonly date field */
.form-field-readonly {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  background: var(--ta-bg-secondary, #f8fafc);
  border-radius: 10px;
  padding: 10px 12px;
  cursor: pointer;
  transition: background 0.15s;
}

.form-field-readonly:active {
  background: var(--ta-bg-page, #f0f4f8);
}

.form-field-readonly span {
  font-size: 14px;
  color: var(--ta-text-primary, #0f172a);
  font-weight: 500;
}

.form-field-readonly .placeholder {
  color: var(--ta-text-quaternary, #cbd5e1);
}

.form-switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 2px 0;
}

.form-actions {
  margin-top: 16px;
}

.form-submit {
  height: 44px;
  font-size: 15px;
  font-weight: 600;
  background: linear-gradient(135deg, #8b5cf6, #7c3aed);
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
