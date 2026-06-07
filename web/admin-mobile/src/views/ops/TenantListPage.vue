<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { showToast, showConfirmDialog } from 'vant'
import request from '@/utils/request'
import StatusTag from '@/components/StatusTag.vue'

const router = useRouter()

const loading = ref(false)
const refreshing = ref(false)
const finished = ref(false)
const tenants = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const search = ref('')
const statusFilter = ref('')

const statusFilters = [
  { label: '全部', value: '' },
  { label: '启用', value: 'active' },
  { label: '暂停', value: 'suspended' },
  { label: '关闭', value: 'closed' },
]

async function fetchTenants(append = false) {
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
    if (statusFilter.value) params.status = statusFilter.value

    const { data: res } = await request.get('/admin/tenants', { params })
    const list = res.data?.list || []
    total.value = res.data?.total || 0

    if (append) {
      tenants.value = [...tenants.value, ...list]
    } else {
      tenants.value = list
    }
    finished.value = tenants.value.length >= total.value
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function onRefresh() {
  refreshing.value = true
  await fetchTenants(false)
}

async function onLoad() {
  if (finished.value) return
  page.value++
  await fetchTenants(true)
}

function onSearch() {
  fetchTenants(false)
}

function setStatusFilter(val: string) {
  statusFilter.value = val
  fetchTenants(false)
}

async function toggleStatus(tenant: any) {
  const newStatus = tenant.status === 'active' ? 'suspended' : 'active'
  try {
    await showConfirmDialog({
      title: newStatus === 'suspended' ? '暂停租户' : '启用租户',
      message: newStatus === 'suspended'
        ? `确定要暂停租户「${tenant.name}」吗？暂停后该租户下的所有用户将无法使用服务。`
        : `确定要启用租户「${tenant.name}」吗？`,
    })
    await request.put(`/admin/tenants/${tenant.id}/status`, { status: newStatus })
    tenant.status = newStatus
    showToast(newStatus === 'active' ? '已启用' : '已暂停')
  } catch {
    // cancelled or error
  }
}

function formatBalance(val: string | number | undefined): string {
  if (val === undefined || val === null) return '$0.00'
  const n = typeof val === 'string' ? parseFloat(val) : val
  if (isNaN(n)) return '$0.00'
  return `$${n.toFixed(2)}`
}

function goDetail(item: any) {
  router.push({
    path: `/m/tenants/${item.id}`,
    state: { tenant: item },
  })
}

const activeCount = computed(() => tenants.value.filter(t => t.status === 'active').length)

onMounted(() => fetchTenants())
</script>

<template>
  <div class="page">

    <!-- Hero Header -->
    <div class="hero">
      <div class="hero-bg">
        <div class="hero-orb hero-orb--1" />
        <div class="hero-orb hero-orb--2" />
      </div>
      <div class="hero-content">
        <div class="hero-label">租户管理</div>
        <div class="hero-metric">
          <span class="hero-value">{{ total }}</span>
          <span class="hero-unit">个租户</span>
        </div>
        <div v-if="activeCount" class="hero-sub">
          <span class="hero-dot"></span>
          <span>{{ activeCount }} 个启用中</span>
        </div>
      </div>
    </div>

    <!-- Search -->
    <div class="search-wrap">
      <van-search
        v-model="search"
        placeholder="搜索租户名称或代码..."
        shape="round"
        @search="onSearch"
        @clear="onSearch"
      />
    </div>

    <!-- Status Filter Chips -->
    <div class="filter-chips">
      <div
        v-for="f in statusFilters"
        :key="f.value"
        class="filter-chip"
        :class="{ 'filter-chip--active': statusFilter === f.value }"
        @click="setStatusFilter(f.value)"
      >
        {{ f.label }}
      </div>
    </div>

    <!-- Count Bar -->
    <div v-if="total > 0" class="count-bar">
      <span class="count-text">共 <b>{{ total }}</b> 个租户</span>
    </div>

    <!-- Tenant Cards -->
    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <van-list v-model:loading="loading" :finished="finished" finished-text="" @load="onLoad">
        <div class="card-list">
          <van-swipe-cell v-for="(item, idx) in tenants" :key="item.id">
            <div
              class="tenant-card"
              :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
              @click="goDetail(item)"
            >
              <!-- Top: name + status -->
              <div class="tenant-card__top">
                <div class="tenant-card__name-row">
                  <h4 class="tenant-card__name">{{ item.name }}</h4>
                  <div class="tenant-card__badges">
                    <StatusTag :status="item.status" />
                  </div>
                </div>
                <span class="tenant-card__code">{{ item.code }}</span>
              </div>

              <!-- Mid: owner + level -->
              <div class="tenant-card__meta">
                <span v-if="item.owner_name" class="tenant-card__info tenant-card__owner">
                  <van-icon name="manager-o" size="13" />
                  <span>{{ item.owner_name }}</span>
                </span>
                <span v-if="item.level_name" class="tenant-card__level">{{ item.level_name }}</span>
              </div>

              <!-- Bottom: member + balance + concurrency -->
              <div class="tenant-card__stats">
                <div class="tenant-stat">
                  <span class="tenant-stat__value">{{ item.member_count ?? 0 }}</span>
                  <span class="tenant-stat__label">成员</span>
                </div>
                <div class="tenant-stat-divider"></div>
                <div class="tenant-stat">
                  <span class="tenant-stat__value tenant-stat__value--money">{{ formatBalance(item.wallet_balance) }}</span>
                  <span class="tenant-stat__label">余额</span>
                </div>
                <div class="tenant-stat-divider"></div>
                <div class="tenant-stat">
                  <span class="tenant-stat__value">{{ item.effective_max_concurrency ?? '-' }}</span>
                  <span class="tenant-stat__label">并发</span>
                </div>
              </div>
            </div>

            <!-- Swipe Actions -->
            <template #right>
              <div class="swipe-actions">
                <div
                  class="swipe-btn"
                  :class="item.status === 'active' ? 'swipe-btn--danger' : 'swipe-btn--success'"
                  @click="toggleStatus(item)"
                >
                  {{ item.status === 'active' ? '暂停' : '启用' }}
                </div>
              </div>
            </template>
          </van-swipe-cell>
        </div>

        <!-- Empty State -->
        <div v-if="!loading && !tenants.length" class="empty-state">
          <van-icon name="search" size="36" color="#cbd5e1" />
          <span>没有找到匹配的租户</span>
        </div>
      </van-list>
    </van-pull-refresh>
  </div>
</template>

<style scoped>
.page {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(24px + env(safe-area-inset-bottom, 0px));
}

/* ── Hero ── */
.hero {
  position: relative;
  overflow: hidden;
  padding-bottom: 20px;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #4338ca 0%, #6366f1 40%, #818cf8 70%, #4f46e5 100%);
  border-radius: 0 0 28px 28px;
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
  padding: 24px 20px 0;
  animation: fadeSlideUp 0.5s both;
}

.hero-label {
  font-size: 20px;
  font-weight: 700;
  color: #fff;
  letter-spacing: -0.02em;
  margin-bottom: 12px;
}

.hero-metric {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.hero-value {
  font-size: 36px;
  font-weight: 800;
  color: #fff;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.04em;
  line-height: 1;
}

.hero-unit {
  font-size: 14px;
  color: rgba(255, 255, 255, 0.6);
  font-weight: 500;
}

.hero-sub {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.55);
  font-weight: 500;
}

.hero-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #5eead4;
}

/* ── Search ── */
.search-wrap {
  padding: 0;
  margin-top: -8px;
  position: relative;
  z-index: 2;
}
.search-wrap :deep(.van-search) {
  padding: 8px 12px;
}

/* ── Filter Chips ── */
.filter-chips {
  display: flex;
  gap: 8px;
  padding: 4px 16px 8px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}
.filter-chips::-webkit-scrollbar { display: none; }

.filter-chip {
  flex-shrink: 0;
  padding: 6px 16px;
  border-radius: 20px;
  font-size: 13px;
  font-weight: 600;
  background: #fff;
  color: #64748b;
  border: 1px solid #e2e8f0;
  cursor: pointer;
  transition: all 0.2s;
  -webkit-tap-highlight-color: transparent;
}

.filter-chip:active {
  transform: scale(0.96);
}

.filter-chip--active {
  background: #6366f1;
  color: #fff;
  border-color: #6366f1;
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

.tenant-card {
  background: #fff;
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;
  animation: cardIn 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
}
.tenant-card:active {
  transform: scale(0.98);
}

@keyframes cardIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

/* ── Card: Top ── */
.tenant-card__top {
  margin-bottom: 6px;
}

.tenant-card__name-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.tenant-card__name {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.tenant-card__badges {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.tenant-card__code {
  font-size: 12px;
  color: #94a3b8;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  margin-top: 2px;
  display: block;
}

/* ── Card: Meta ── */
.tenant-card__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}

.tenant-card__info {
  display: flex;
  align-items: center;
  gap: 3px;
  font-size: 12px;
  color: #64748b;
}

.tenant-card__owner {
  flex-shrink: 0;
}

.tenant-card__level {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 6px;
  background: rgba(99, 102, 241, 0.1);
  color: #6366f1;
}

/* ── Card: Stats ── */
.tenant-card__stats {
  display: flex;
  align-items: center;
  background: #f8fafc;
  border-radius: 10px;
  padding: 10px 0;
}

.tenant-stat {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.tenant-stat__value {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
  font-variant-numeric: tabular-nums;
}

.tenant-stat__value--money {
  color: #0d9488;
  font-size: 14px;
}

.tenant-stat__label {
  font-size: 10px;
  color: #94a3b8;
  font-weight: 500;
}

.tenant-stat-divider {
  width: 1px;
  height: 20px;
  background: #e2e8f0;
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
}
.swipe-btn--success { background: #10b981; }
.swipe-btn--danger { background: #f59e0b; }

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
