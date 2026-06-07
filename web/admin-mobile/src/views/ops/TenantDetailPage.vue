<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { showToast, showConfirmDialog } from 'vant'
import request from '@/utils/request'

const route = useRoute()

const tenantId = computed(() => route.params.id as string)
const loading = ref(true)
const tenant = ref<any>(null)

// Edit dialog state
const showEditDialog = ref(false)
const editForm = ref({
  name: '',
  max_members: 0,
  max_concurrency: 0,
  level: 0,
})

async function fetchTenant() {
  loading.value = true
  try {
    // Prefer history.state from list page
    if (history.state?.tenant) {
      tenant.value = history.state.tenant
    }

    // API fallback
    if (!tenant.value && tenantId.value) {
      const { data: res } = await request.get(`/admin/tenants/${tenantId.value}`)
      tenant.value = res.data
    }
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
  }
}

function formatBalance(val: string | number | undefined): string {
  if (val === undefined || val === null) return '$0.000000'
  const n = typeof val === 'string' ? parseFloat(val) : val
  if (isNaN(n)) return '$0.000000'
  return `$${n.toFixed(6)}`
}

function formatDate(dateStr: string | undefined): string {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

// Status color map
const statusColorMap: Record<string, string> = {
  active: '#10b981',
  suspended: '#f59e0b',
  closed: '#ef4444',
  trial: '#3b82f6',
  past_due: '#f97316',
  frozen: '#8b5cf6',
}

const statusBgMap: Record<string, string> = {
  active: 'rgba(16,185,129,0.1)',
  suspended: 'rgba(245,158,11,0.1)',
  closed: 'rgba(239,68,68,0.1)',
  trial: 'rgba(59,130,246,0.1)',
  past_due: 'rgba(249,115,22,0.1)',
  frozen: 'rgba(139,92,246,0.1)',
}

const statusLabelMap: Record<string, string> = {
  active: '启用',
  suspended: '已暂停',
  closed: '已关闭',
  trial: '试用中',
  past_due: '逾期',
  frozen: '已冻结',
}

function statusColor(status: string): string {
  return statusColorMap[status] || '#94a3b8'
}

function statusBg(status: string): string {
  return statusBgMap[status] || 'rgba(148,163,184,0.1)'
}

function statusLabel(status: string): string {
  return statusLabelMap[status] || status
}

// Toggle status
async function toggleStatus() {
  if (!tenant.value) return
  const current = tenant.value.status
  let newStatus: string
  let confirmMsg: string

  if (current === 'active') {
    newStatus = 'suspended'
    confirmMsg = `确定要暂停租户「${tenant.value.name}」吗？暂停后该租户下所有用户将无法使用服务。`
  } else {
    newStatus = 'active'
    confirmMsg = `确定要启用租户「${tenant.value.name}」吗？`
  }

  try {
    await showConfirmDialog({
      title: newStatus === 'suspended' ? '暂停租户' : '启用租户',
      message: confirmMsg,
    })
    await request.put(`/admin/tenants/${tenantId.value}/status`, { status: newStatus })
    tenant.value.status = newStatus
    showToast(newStatus === 'active' ? '已启用' : '已暂停')
  } catch {
    // cancelled or error
  }
}

// Edit dialog
function openEditDialog() {
  if (!tenant.value) return
  editForm.value = {
    name: tenant.value.name || '',
    max_members: tenant.value.effective_max_members ?? tenant.value.max_members ?? 0,
    max_concurrency: tenant.value.effective_max_concurrency ?? tenant.value.max_concurrency ?? 0,
    level: tenant.value.level ?? 0,
  }
  showEditDialog.value = true
}

async function saveEdit() {
  try {
    await request.put(`/admin/tenants/${tenantId.value}`, {
      name: editForm.value.name,
      max_members: editForm.value.max_members,
      max_concurrency: editForm.value.max_concurrency,
      level: editForm.value.level,
    })
    // Update local data
    tenant.value.name = editForm.value.name
    tenant.value.max_members = editForm.value.max_members
    tenant.value.max_concurrency = editForm.value.max_concurrency
    tenant.value.level = editForm.value.level
    showEditDialog.value = false
    showToast('保存成功')
  } catch {
    // handled by interceptor
  }
}

onMounted(fetchTenant)
</script>

<template>
  <div class="tenant-detail">
    <!-- Loading -->
    <template v-if="loading">
      <div class="skeleton-hero">
        <div class="skeleton-circle-lg"></div>
        <div class="skeleton-lines">
          <div class="skeleton-line w70"></div>
          <div class="skeleton-line w50"></div>
          <div class="skeleton-line w30"></div>
        </div>
      </div>
    </template>

    <template v-if="tenant">
      <!-- Hero Header -->
      <div class="detail-hero">
        <div class="hero-bg">
          <div class="hero-orb hero-orb--1" />
          <div class="hero-orb hero-orb--2" />
        </div>
        <div class="hero-content">
          <div class="hero-top">
            <div class="hero-icon">
              <van-icon name="shop-o" size="26" />
            </div>
            <div class="hero-status">
              <span
                class="status-badge"
                :style="{ background: statusBg(tenant.status), color: statusColor(tenant.status) }"
              >
                {{ statusLabel(tenant.status) }}
              </span>
            </div>
          </div>
          <h2 class="hero-title">{{ tenant.name }}</h2>
          <div class="hero-meta">
            <span class="hero-code">{{ tenant.code }}</span>
            <span class="meta-divider">·</span>
            <span v-if="tenant.level_name" class="hero-level">{{ tenant.level_name }}</span>
          </div>
        </div>
      </div>

      <!-- Stats Row -->
      <div class="stats-row">
        <div class="stat-card">
          <div class="stat-icon" style="--c: #6366f1">
            <van-icon name="friends-o" size="18" />
          </div>
          <div class="stat-body">
            <span class="stat-value">{{ tenant.member_count ?? 0 }}</span>
            <span class="stat-label">成员数</span>
          </div>
        </div>
        <div class="stat-card">
          <div class="stat-icon" style="--c: #0d9488">
            <van-icon name="balance-o" size="18" />
          </div>
          <div class="stat-body">
            <span class="stat-value stat-value--money">{{ formatBalance(tenant.wallet_balance) }}</span>
            <span class="stat-label">钱包余额</span>
          </div>
        </div>
        <div class="stat-card">
          <div class="stat-icon" style="--c: #f59e0b">
            <van-icon name="expand-o" size="18" />
          </div>
          <div class="stat-body">
            <span class="stat-value">{{ tenant.effective_max_concurrency ?? '-' }}</span>
            <span class="stat-label">并发上限</span>
          </div>
        </div>
      </div>

      <!-- Quick Actions -->
      <div class="quick-actions">
        <button
          class="action-btn"
          :class="tenant.status === 'active' ? 'action-suspend' : 'action-activate'"
          @click="toggleStatus"
        >
          <van-icon :name="tenant.status === 'active' ? 'pause-circle-o' : 'play-circle-o'" size="16" />
          <span>{{ tenant.status === 'active' ? '暂停租户' : '启用租户' }}</span>
        </button>
        <button class="action-btn action-edit" @click="openEditDialog">
          <van-icon name="edit" size="16" />
          <span>编辑信息</span>
        </button>
      </div>

      <!-- Info Section -->
      <div class="detail-section" style="--si: 0">
        <div class="section-header">
          <span class="section-dot" style="--c: #6366f1"></span>
          <span class="section-title">基本信息</span>
        </div>
        <div class="section-card">
          <div class="info-row">
            <span class="info-label">租户名称</span>
            <span class="info-value">{{ tenant.name }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">租户代码</span>
            <span class="info-value info-value--mono">{{ tenant.code }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">所有者</span>
            <span class="info-value">{{ tenant.owner_name || '-' }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">最大成员数</span>
            <span class="info-value">{{ tenant.effective_max_members ?? tenant.max_members ?? '-' }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">最大并发</span>
            <span class="info-value">{{ tenant.effective_max_concurrency ?? tenant.max_concurrency ?? '-' }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">租户等级</span>
            <span class="info-value">
              <span v-if="tenant.level_name" class="level-chip">{{ tenant.level_name }}</span>
              <span v-else>{{ tenant.level ?? '-' }}</span>
            </span>
          </div>
          <div class="info-row">
            <span class="info-label">创建时间</span>
            <span class="info-value info-value--sm">{{ formatDate(tenant.created_at) }}</span>
          </div>
        </div>
      </div>

      <!-- Edit Dialog -->
      <van-dialog
        v-model:show="showEditDialog"
        title="编辑租户信息"
        show-cancel-button
        confirm-button-color="#6366f1"
        @confirm="saveEdit"
      >
        <div class="edit-form">
          <div class="edit-field">
            <label class="edit-label">租户名称</label>
            <van-field
              v-model="editForm.name"
              placeholder="请输入租户名称"
              border
            />
          </div>
          <div class="edit-field">
            <label class="edit-label">最大成员数</label>
            <van-field
              v-model="editForm.max_members"
              type="digit"
              placeholder="请输入最大成员数"
              border
            />
          </div>
          <div class="edit-field">
            <label class="edit-label">最大并发数</label>
            <van-field
              v-model="editForm.max_concurrency"
              type="digit"
              placeholder="请输入最大并发数"
              border
            />
          </div>
          <div class="edit-field">
            <label class="edit-label">租户等级</label>
            <van-field
              v-model="editForm.level"
              type="digit"
              placeholder="请输入租户等级"
              border
            />
          </div>
        </div>
      </van-dialog>
    </template>
  </div>
</template>

<style scoped>
.tenant-detail {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(24px + env(safe-area-inset-bottom, 0px));
}

/* ===== Skeleton ===== */
.skeleton-hero {
  padding: 32px 20px;
}

.skeleton-circle-lg {
  width: 52px;
  height: 52px;
  border-radius: 14px;
  background: #e2e8f0;
  margin-bottom: 14px;
  animation: pulse 1.5s ease-in-out infinite;
}

.skeleton-lines {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.skeleton-line {
  height: 14px;
  border-radius: 6px;
  background: #e2e8f0;
  animation: pulse 1.5s ease-in-out infinite;
}

.skeleton-line.w70 { width: 70%; }
.skeleton-line.w50 { width: 50%; }
.skeleton-line.w30 { width: 30%; }

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

/* ===== Hero Header ===== */
.detail-hero {
  position: relative;
  padding: 24px 20px 28px;
  overflow: hidden;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #4338ca 0%, #6366f1 40%, #818cf8 70%, #4f46e5 100%);
  border-radius: 0 0 28px 28px;
}

.hero-bg::after {
  content: '';
  position: absolute;
  inset: 0;
  background:
    radial-gradient(ellipse 180px 180px at 10% 50%, rgba(255, 255, 255, 0.1) 0%, transparent 70%),
    radial-gradient(ellipse 120px 120px at 90% 80%, rgba(255, 255, 255, 0.05) 0%, transparent 70%);
}

.hero-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(60px);
  pointer-events: none;
}
.hero-orb--1 {
  width: 200px; height: 200px;
  background: rgba(129, 140, 248, 0.3);
  top: -40px; right: -30px;
}
.hero-orb--2 {
  width: 160px; height: 160px;
  background: rgba(99, 102, 241, 0.2);
  bottom: 10px; left: -20px;
}

.hero-content {
  position: relative;
  z-index: 1;
  animation: fadeSlideUp 0.5s both;
}

.hero-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.hero-icon {
  width: 52px;
  height: 52px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.15);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
}

.status-badge {
  font-size: 12px;
  font-weight: 600;
  padding: 4px 12px;
  border-radius: 12px;
}

.hero-title {
  font-size: 20px;
  font-weight: 700;
  color: #fff;
  margin: 0 0 8px;
  letter-spacing: -0.2px;
  line-height: 1.3;
}

.hero-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.hero-code {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.6);
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
}

.meta-divider {
  color: rgba(255, 255, 255, 0.25);
  font-size: 12px;
}

.hero-level {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 10px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.15);
  color: rgba(255, 255, 255, 0.8);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

/* ===== Stats Row ===== */
.stats-row {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 8px;
  padding: 0 16px;
  margin-top: -8px;
  position: relative;
  z-index: 2;
  animation: fadeSlideUp 0.45s 0.12s both;
}

.stat-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 12px 8px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  box-shadow:
    0 2px 8px rgba(0, 0, 0, 0.04),
    0 1px 2px rgba(0, 0, 0, 0.03);
}

.stat-icon {
  width: 32px;
  height: 32px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--c) 10%, transparent);
  color: var(--c);
  display: flex;
  align-items: center;
  justify-content: center;
}

.stat-body {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1px;
}

.stat-value {
  font-size: 14px;
  font-weight: 700;
  color: var(--ta-text-primary, #1e293b);
  letter-spacing: -0.3px;
  font-variant-numeric: tabular-nums;
}

.stat-value--money {
  color: #0d9488;
  font-size: 13px;
}

.stat-label {
  font-size: 10px;
  color: var(--ta-text-tertiary, #94a3b8);
}

/* ===== Quick Actions ===== */
.quick-actions {
  display: flex;
  gap: 10px;
  padding: 16px 16px 0;
  animation: fadeSlideUp 0.45s 0.18s both;
}

.action-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 12px 16px;
  border-radius: 14px;
  font-size: 13px;
  font-weight: 600;
  border: none;
  cursor: pointer;
  transition: all 0.2s;
  -webkit-tap-highlight-color: transparent;
}

.action-btn:active {
  transform: scale(0.96);
}

.action-suspend {
  background: #fffbeb;
  color: #d97706;
  border: 1px solid #fef3c7;
}

.action-suspend:active {
  background: #fef3c7;
}

.action-activate {
  background: #f0fdf4;
  color: #16a34a;
  border: 1px solid #bbf7d0;
}

.action-activate:active {
  background: #dcfce7;
}

.action-edit {
  background: #eef2ff;
  color: #4f46e5;
  border: 1px solid #e0e7ff;
}

.action-edit:active {
  background: #e0e7ff;
}

/* ===== Sections ===== */
.detail-section {
  padding: 16px 16px 0;
  animation: fadeSlideUp 0.45s calc(0.2s + var(--si, 0) * 0.06s) both;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 4px 10px;
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

.section-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 4px 0;
  box-shadow:
    0 1px 3px rgba(0, 0, 0, 0.03),
    0 1px 2px rgba(0, 0, 0, 0.04);
}

/* ===== Info Rows ===== */
.info-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 13px 16px;
  position: relative;
}

.info-row:not(:last-child)::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 16px;
  right: 16px;
  height: 1px;
  background: var(--ta-border-light, #f1f5f9);
}

.info-label {
  font-size: 13px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
  flex-shrink: 0;
}

.info-value {
  font-size: 13px;
  font-weight: 500;
  color: var(--ta-text-primary, #1e293b);
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-left: 16px;
}

.info-value--mono {
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  font-size: 12px;
}

.info-value--sm {
  font-size: 12px;
}

.level-chip {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 10px;
  border-radius: 8px;
  background: rgba(99, 102, 241, 0.1);
  color: #6366f1;
}

/* ===== Edit Dialog Form ===== */
.edit-form {
  padding: 16px 20px 8px;
}

.edit-field {
  margin-bottom: 12px;
}

.edit-label {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: #334155;
  margin-bottom: 6px;
}

.edit-field :deep(.van-cell) {
  padding: 8px 12px;
  border-radius: 10px;
  background: #f8fafc;
}

.edit-field :deep(.van-field__control) {
  font-size: 14px;
}

/* ===== Animation ===== */
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
