<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { showToast, showConfirmDialog } from 'vant'
import request from '@/utils/request'

const loading = ref(false)
const refreshing = ref(false)
const finished = ref(false)
const plans = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const activeStatus = ref('')

const showForm = ref(false)
const formMode = ref<'create' | 'edit'>('create')
const formLoading = ref(false)

const defaultForm = () => ({
  id: 0,
  name: '',
  identifier: '',
  description: '',
  monthly_price: '',
  yearly_price: '',
  monthly_quota_tokens: '',
  is_recommended: false,
  sort_order: 0,
})

const form = ref(defaultForm())

const statusFilters = [
  { key: '', label: '全部' },
  { key: 'active', label: '启用' },
  { key: 'archived', label: '已归档' },
]

const activePlansCount = computed(() =>
  plans.value.filter(p => p.status === 'active').length
)

function formatPrice(n: number | undefined | null): string {
  if (n == null) return '¥0.00'
  return `¥${Number(n).toFixed(2)}`
}

function formatTokens(n: number | undefined | null): string {
  if (!n && n !== 0) return '-'
  const v = Number(n)
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`
  if (v >= 1_000) return `${(v / 1_000).toFixed(0)}K`
  return String(v)
}

async function fetchPlans(append = false) {
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

    const { data: res } = await request.get('/admin/plans', { params })
    const list = res.data?.list || []
    total.value = res.data?.total || 0

    if (append) {
      plans.value = [...plans.value, ...list]
    } else {
      plans.value = list
    }
    finished.value = plans.value.length >= total.value
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function onRefresh() {
  refreshing.value = true
  await fetchPlans(false)
}

async function onLoad() {
  if (finished.value) return
  page.value++
  await fetchPlans(true)
}

function setStatus(key: string) {
  activeStatus.value = key
  fetchPlans(false)
}

function openCreate() {
  formMode.value = 'create'
  form.value = defaultForm()
  showForm.value = true
}

function openEdit(item: any) {
  formMode.value = 'edit'
  form.value = {
    id: item.id,
    name: item.name || '',
    identifier: item.identifier || '',
    description: item.description || '',
    monthly_price: item.monthly_price != null ? String(item.monthly_price) : '',
    yearly_price: item.yearly_price != null ? String(item.yearly_price) : '',
    monthly_quota_tokens: item.monthly_quota_tokens != null ? String(item.monthly_quota_tokens) : '',
    is_recommended: !!item.is_recommended,
    sort_order: item.sort_order ?? 0,
  }
  showForm.value = true
}

async function submitForm() {
  if (!form.value.name.trim()) {
    showToast('请输入套餐名称')
    return
  }
  if (!form.value.identifier.trim()) {
    showToast('请输入套餐标识')
    return
  }

  formLoading.value = true
  try {
    const body: any = {
      name: form.value.name.trim(),
      identifier: form.value.identifier.trim(),
      description: form.value.description?.trim() || '',
      monthly_price: form.value.monthly_price ? Number(form.value.monthly_price) : 0,
      yearly_price: form.value.yearly_price ? Number(form.value.yearly_price) : 0,
      monthly_quota_tokens: form.value.monthly_quota_tokens ? Number(form.value.monthly_quota_tokens) : 0,
      is_recommended: form.value.is_recommended,
      sort_order: form.value.sort_order ?? 0,
    }

    if (formMode.value === 'create') {
      await request.post('/admin/plans', body)
      showToast('创建成功')
    } else {
      await request.put(`/admin/plans/${form.value.id}`, body)
      showToast('更新成功')
    }

    showForm.value = false
    await fetchPlans(false)
  } catch {
    // handled by interceptor
  } finally {
    formLoading.value = false
  }
}

async function archivePlan(item: any) {
  try {
    await showConfirmDialog({
      title: '确认归档',
      message: `确定要归档套餐「${item.name}」吗？`,
    })
    await request.delete(`/admin/plans/${item.id}`)
    showToast('已归档')
    await fetchPlans(false)
  } catch {
    // cancelled or error
  }
}

async function toggleRecommend(item: any) {
  try {
    await request.put(`/admin/plans/${item.id}/toggle-recommend`)
    item.is_recommended = !item.is_recommended
    showToast(item.is_recommended ? '已推荐' : '已取消推荐')
  } catch {
    // handled by interceptor
  }
}

onMounted(() => fetchPlans())
</script>

<template>
  <div class="plan-page">

    <!-- ═══════ HERO ═══════ -->
    <div class="hero">
      <div class="hero-bg">
        <div class="hero-orb hero-orb--1" />
        <div class="hero-orb hero-orb--2" />
      </div>
      <div class="hero-content">
        <div class="hero-label-row">
          <span class="hero-title">套餐管理</span>
          <span class="hero-subtitle">Plan &amp; Package</span>
        </div>
        <div class="hero-stats">
          <div class="hero-stat">
            <span class="hero-stat__value">{{ total }}</span>
            <span class="hero-stat__label">套餐总数</span>
          </div>
          <div class="hero-stat-divider" />
          <div class="hero-stat">
            <span class="hero-stat__value">{{ activePlansCount }}</span>
            <span class="hero-stat__label">启用中</span>
          </div>
        </div>
      </div>
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
      <span class="count-text">共 <b>{{ total }}</b> 个套餐</span>
    </div>

    <!-- Plan Cards -->
    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <van-list v-model:loading="loading" :finished="finished" finished-text="" @load="onLoad">
        <div class="card-list">
          <van-swipe-cell v-for="(item, idx) in plans" :key="item.id">
            <div
              class="plan-card"
              :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
              @click="openEdit(item)"
            >
              <!-- Top row: name + badges -->
              <div class="plan-card__top">
                <div class="plan-card__name-row">
                  <h4 class="plan-card__name">{{ item.name }}</h4>
                  <div class="plan-card__badges">
                    <van-icon
                      v-if="item.is_recommended"
                      name="star"
                      size="14"
                      color="#eab308"
                      class="plan-card__star"
                    />
                    <span
                      class="plan-card__status"
                      :class="{
                        'plan-card__status--active': item.status === 'active',
                        'plan-card__status--archived': item.status === 'archived',
                      }"
                    >
                      {{ item.status === 'active' ? '启用' : item.status === 'archived' ? '已归档' : item.status }}
                    </span>
                  </div>
                </div>
              </div>

              <!-- Identifier (monospace) -->
              <div class="plan-card__identifier">{{ item.identifier }}</div>

              <!-- Description (truncated) -->
              <div v-if="item.description" class="plan-card__desc">
                {{ item.description }}
              </div>

              <!-- Price + Tokens row -->
              <div class="plan-card__details">
                <div class="detail-item">
                  <span class="detail-label">月价</span>
                  <span class="detail-val detail-val--price">{{ formatPrice(item.monthly_price) }}</span>
                </div>
                <div class="detail-divider" />
                <div class="detail-item">
                  <span class="detail-label">年价</span>
                  <span class="detail-val detail-val--price">{{ formatPrice(item.yearly_price) }}</span>
                </div>
                <div class="detail-divider" />
                <div class="detail-item">
                  <span class="detail-label">Token 额度</span>
                  <span class="detail-val">{{ formatTokens(item.monthly_quota_tokens) }}</span>
                </div>
              </div>

              <!-- Footer: sort_order -->
              <div class="plan-card__footer">
                <span v-if="item.sort_order" class="plan-card__sort">排序: {{ item.sort_order }}</span>
              </div>
            </div>

            <!-- Swipe actions -->
            <template #right>
              <div class="swipe-actions">
                <div
                  v-if="item.status === 'active'"
                  class="swipe-btn swipe-btn--warn"
                  @click.stop="archivePlan(item)"
                >
                  归档
                </div>
                <div
                  class="swipe-btn swipe-btn--recommend"
                  @click.stop="toggleRecommend(item)"
                >
                  {{ item.is_recommended ? '取消推荐' : '推荐' }}
                </div>
              </div>
            </template>
          </van-swipe-cell>
        </div>

        <!-- Empty state -->
        <div v-if="!loading && !plans.length" class="empty-state">
          <van-icon name="label-o" size="42" color="#cbd5e1" />
          <span class="empty-text">暂无套餐数据</span>
          <span class="empty-hint">点击右下角按钮创建第一个套餐</span>
        </div>
      </van-list>
    </van-pull-refresh>

    <!-- Floating add button -->
    <div class="fab" @click="openCreate">
      <van-icon name="plus" size="24" color="#fff" />
    </div>

    <!-- ═══════ CREATE / EDIT FORM POPUP ═══════ -->
    <van-popup
      v-model:show="showForm"
      position="bottom"
      round
      :style="{ maxHeight: '85vh' }"
      closeable
      close-icon="cross"
    >
      <div class="form-popup">
        <h3 class="form-title">{{ formMode === 'create' ? '创建套餐' : '编辑套餐' }}</h3>

        <div class="form-body">
          <!-- Name -->
          <div class="form-group">
            <label class="form-label">套餐名称 <span class="form-required">*</span></label>
            <van-field
              v-model="form.name"
              placeholder="如：专业版"
              :border="false"
              class="form-field"
            />
          </div>

          <!-- Identifier -->
          <div class="form-group">
            <label class="form-label">套餐标识 <span class="form-required">*</span></label>
            <van-field
              v-model="form.identifier"
              placeholder="如：pro"
              :border="false"
              class="form-field form-field--mono"
            />
          </div>

          <!-- Description -->
          <div class="form-group">
            <label class="form-label">描述</label>
            <van-field
              v-model="form.description"
              type="textarea"
              placeholder="套餐功能说明..."
              rows="2"
              autosize
              :border="false"
              class="form-field"
            />
          </div>

          <!-- Price row -->
          <div class="form-row">
            <div class="form-group form-group--half">
              <label class="form-label">月价 (CNY)</label>
              <van-field
                v-model="form.monthly_price"
                type="number"
                placeholder="0.00"
                :border="false"
                class="form-field"
              />
            </div>
            <div class="form-group form-group--half">
              <label class="form-label">年价 (CNY)</label>
              <van-field
                v-model="form.yearly_price"
                type="number"
                placeholder="0.00"
                :border="false"
                class="form-field"
              />
            </div>
          </div>

          <!-- Token quota -->
          <div class="form-group">
            <label class="form-label">月 Token 额度</label>
            <van-field
              v-model="form.monthly_quota_tokens"
              type="number"
              placeholder="如 1000000"
              :border="false"
              class="form-field"
            />
          </div>

          <!-- Sort order -->
          <div class="form-group">
            <label class="form-label">排序权重</label>
            <van-field
              v-model="form.sort_order"
              type="digit"
              placeholder="0"
              :border="false"
              class="form-field"
            />
          </div>

          <!-- Recommend switch -->
          <div class="form-switch-row">
            <span class="form-label">推荐套餐</span>
            <van-switch v-model="form.is_recommended" size="20" active-color="#8b5cf6" />
          </div>
        </div>

        <!-- Submit button -->
        <div class="form-actions">
          <van-button
            block
            round
            type="primary"
            :loading="formLoading"
            loading-text="提交中..."
            class="form-submit"
            @click="submitForm"
          >
            {{ formMode === 'create' ? '创建套餐' : '保存修改' }}
          </van-button>
        </div>
      </div>
    </van-popup>
  </div>
</template>

<style scoped>
.plan-page {
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

/* ── Card List ── */
.card-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 12px;
}

.plan-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;
  animation: fadeSlideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
}
.plan-card:active {
  transform: scale(0.98);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
}

/* ── Card: Top ── */
.plan-card__top {
  margin-bottom: 4px;
}

.plan-card__name-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.plan-card__name {
  font-size: 15px;
  font-weight: 700;
  color: var(--ta-text-primary, #0f172a);
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.plan-card__badges {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.plan-card__star {
  flex-shrink: 0;
}

.plan-card__status {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  line-height: 1.4;
  background: rgba(16, 185, 129, 0.1);
  color: #10b981;
}
.plan-card__status--archived {
  background: rgba(100, 116, 139, 0.1);
  color: #94a3b8;
}

/* ── Card: Identifier ── */
.plan-card__identifier {
  font-size: 12px;
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  color: var(--ta-text-tertiary, #94a3b8);
  letter-spacing: -0.02em;
  margin-bottom: 4px;
}

/* ── Card: Description ── */
.plan-card__desc {
  font-size: 12px;
  color: var(--ta-text-tertiary, #64748b);
  line-height: 1.5;
  margin-bottom: 10px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

/* ── Card: Details Row ── */
.plan-card__details {
  display: flex;
  align-items: center;
  background: var(--ta-bg-secondary, #f8fafc);
  border-radius: 8px;
  padding: 8px 12px;
  margin-bottom: 6px;
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
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
}

.detail-val {
  font-size: 13px;
  font-weight: 700;
  color: var(--ta-text-primary, #1e293b);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.detail-val--price {
  color: #8b5cf6;
}

.detail-divider {
  width: 1px;
  height: 24px;
  background: var(--ta-border-light, #e2e8f0);
  margin: 0 8px;
  flex-shrink: 0;
}

/* ── Card: Footer ── */
.plan-card__footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
}

.plan-card__sort {
  font-size: 10px;
  color: var(--ta-text-quaternary, #cbd5e1);
  font-weight: 500;
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
.swipe-btn--warn {
  background: #f59e0b;
}
.swipe-btn--recommend {
  background: #8b5cf6;
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
