<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import request from '@/utils/request'
import StatusTag from '@/components/StatusTag.vue'

const router = useRouter()

const loading = ref(false)
const refreshing = ref(false)
const finished = ref(false)
const tickets = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const statusFilter = ref('')
const categoryFilter = ref('')

const statusFilters = [
  { label: '全部', value: '' },
  { label: '待处理', value: 'pending' },
  { label: '处理中', value: 'processing' },
  { label: '已回复', value: 'replied' },
  { label: '已关闭', value: 'closed' },
  { label: '已重开', value: 'reopened' },
]

const categoryFilters = [
  { label: '全部', value: '' },
  { label: '账单', value: 'billing' },
  { label: '技术', value: 'technical' },
  { label: '功能', value: 'feature_request' },
  { label: '其他', value: 'other' },
]

const categoryConfig: Record<string, { label: string; color: string; bg: string }> = {
  billing: { label: '账单', color: '#d97706', bg: 'rgba(217,119,6,0.1)' },
  technical: { label: '技术', color: '#2563eb', bg: 'rgba(37,99,235,0.1)' },
  feature_request: { label: '功能', color: '#7c3aed', bg: 'rgba(124,58,237,0.1)' },
  other: { label: '其他', color: '#64748b', bg: 'rgba(100,116,139,0.1)' },
}

const urgencyConfig: Record<string, { label: string; color: string; bg: string }> = {
  low: { label: '低', color: '#10b981', bg: 'rgba(16,185,129,0.1)' },
  medium: { label: '中', color: '#3b82f6', bg: 'rgba(59,130,246,0.1)' },
  high: { label: '高', color: '#f59e0b', bg: 'rgba(245,158,11,0.1)' },
  critical: { label: '紧急', color: '#ef4444', bg: 'rgba(239,68,68,0.1)' },
}

const ticketStatusConfig: Record<string, { label: string; color: string; bg: string }> = {
  pending: { label: '待处理', color: '#f59e0b', bg: 'rgba(245,158,11,0.1)' },
  processing: { label: '处理中', color: '#3b82f6', bg: 'rgba(59,130,246,0.1)' },
  replied: { label: '已回复', color: '#0d9488', bg: 'rgba(13,148,136,0.1)' },
  closed: { label: '已关闭', color: '#94a3b8', bg: 'rgba(148,163,184,0.1)' },
  reopened: { label: '已重开', color: '#8b5cf6', bg: 'rgba(139,92,246,0.1)' },
}

function getCategoryInfo(cat: string) {
  return categoryConfig[cat] || { label: cat, color: '#64748b', bg: 'rgba(100,116,139,0.1)' }
}

function getUrgencyInfo(urg: string) {
  return urgencyConfig[urg] || { label: urg, color: '#64748b', bg: 'rgba(100,116,139,0.1)' }
}

function getTicketStatusInfo(status: string) {
  return ticketStatusConfig[status] || { label: status, color: '#64748b', bg: 'rgba(100,116,139,0.1)' }
}

function formatDateTime(dateStr: string | undefined): string {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  const pad = (v: number) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

async function fetchTickets(append = false) {
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
    if (statusFilter.value) params.status = statusFilter.value
    if (categoryFilter.value) params.category = categoryFilter.value

    const { data: res } = await request.get('/admin/tickets', { params })
    const list = res.data?.list || []
    total.value = res.data?.total || 0

    if (append) {
      tickets.value = [...tickets.value, ...list]
    } else {
      tickets.value = list
    }
    finished.value = tickets.value.length >= total.value
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function onRefresh() {
  refreshing.value = true
  await fetchTickets(false)
}

async function onLoad() {
  if (finished.value) return
  page.value++
  await fetchTickets(true)
}

function setStatusFilter(val: string) {
  statusFilter.value = val
  fetchTickets(false)
}

function setCategoryFilter(val: string) {
  categoryFilter.value = val
  fetchTickets(false)
}

function goDetail(item: any) {
  router.push(`/m/tickets/${item.id}`)
}

const activeCount = computed(() => tickets.value.filter(t => t.status !== 'closed').length)

onMounted(() => fetchTickets())
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
        <div class="hero-label">工单管理</div>
        <div class="hero-metric">
          <span class="hero-value">{{ total }}</span>
          <span class="hero-unit">个工单</span>
        </div>
        <div v-if="activeCount" class="hero-sub">
          <span class="hero-dot"></span>
          <span>{{ activeCount }} 个进行中</span>
        </div>
      </div>
    </div>

    <!-- ═══════ STATUS FILTER CHIPS ═══════ -->
    <div class="filter-chips">
      <div
        v-for="f in statusFilters"
        :key="'s-' + f.value"
        class="filter-chip"
        :class="{ 'filter-chip--active': statusFilter === f.value }"
        @click="setStatusFilter(f.value)"
      >
        {{ f.label }}
      </div>
    </div>

    <!-- ═══════ CATEGORY FILTER CHIPS ═══════ -->
    <div class="filter-chips">
      <div
        v-for="f in categoryFilters"
        :key="'c-' + f.value"
        class="filter-chip filter-chip--cat"
        :class="{ 'filter-chip--active-cat': categoryFilter === f.value }"
        @click="setCategoryFilter(f.value)"
      >
        {{ f.label }}
      </div>
    </div>

    <!-- ═══════ COUNT BAR ═══════ -->
    <div v-if="total > 0" class="count-bar">
      <span class="count-text">共 <b>{{ total }}</b> 个工单</span>
    </div>

    <!-- ═══════ TICKET CARDS ═══════ -->
    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <van-list v-model:loading="loading" :finished="finished" finished-text="" @load="onLoad">
        <div class="card-list">
          <div
            v-for="(item, idx) in tickets"
            :key="item.id"
            class="ticket-card"
            :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
            @click="goDetail(item)"
          >
            <!-- Top: title + badges -->
            <div class="ticket-card__top">
              <h4 class="ticket-card__title">{{ item.title }}</h4>
              <div class="ticket-card__badges">
                <span
                  class="mini-badge"
                  :style="{
                    color: getUrgencyInfo(item.urgency).color,
                    background: getUrgencyInfo(item.urgency).bg,
                  }"
                >
                  {{ getUrgencyInfo(item.urgency).label }}
                </span>
                <span
                  class="mini-badge"
                  :style="{
                    color: getTicketStatusInfo(item.status).color,
                    background: getTicketStatusInfo(item.status).bg,
                  }"
                >
                  {{ getTicketStatusInfo(item.status).label }}
                </span>
              </div>
            </div>

            <!-- User info -->
            <div class="ticket-card__meta">
              <span class="ticket-card__info">
                <van-icon name="manager-o" size="13" />
                <span>{{ item.tenant_name || '-' }}</span>
              </span>
              <span v-if="item.user_display_name" class="ticket-card__info">
                <van-icon name="user-o" size="13" />
                <span>{{ item.user_display_name }}</span>
              </span>
            </div>

            <!-- Bottom row -->
            <div class="ticket-card__bottom">
              <span
                class="cat-badge"
                :style="{
                  color: getCategoryInfo(item.category).color,
                  background: getCategoryInfo(item.category).bg,
                }"
              >
                {{ getCategoryInfo(item.category).label }}
              </span>
              <div class="ticket-card__bottom-right">
                <span v-if="item.assigned_admin_name" class="ticket-card__admin">
                  <van-icon name="service-o" size="12" />
                  <span>{{ item.assigned_admin_name }}</span>
                </span>
                <span class="ticket-card__time">{{ formatDateTime(item.created_at) }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- ═══════ EMPTY STATE ═══════ -->
        <div v-if="!loading && !tickets.length" class="empty-state">
          <van-icon name="orders-o" size="42" color="#cbd5e1" />
          <span class="empty-text">暂无工单</span>
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

/* ═══════════════════════════════════════
   HERO — Orange Theme
   ═══════════════════════════════════════ */
.hero {
  position: relative;
  overflow: hidden;
  padding-bottom: 20px;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #c2410c 0%, #ea580c 30%, #f97316 55%, #fb923c 80%, #ea580c 100%);
  border-radius: 0 0 28px 28px;
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
  background: rgba(251, 146, 60, 0.35);
  top: -40px;
  right: -30px;
}
.hero-orb--2 {
  width: 160px;
  height: 160px;
  background: rgba(234, 88, 12, 0.25);
  bottom: 10px;
  left: -20px;
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
  color: rgba(255, 255, 255, 0.65);
  font-weight: 500;
}

.hero-sub {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.6);
  font-weight: 500;
}

.hero-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #5eead4;
}

/* ═══════════════════════════════════════
   FILTER CHIPS
   ═══════════════════════════════════════ */
.filter-chips {
  display: flex;
  gap: 8px;
  padding: 4px 16px 4px;
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
  background: #f97316;
  color: #fff;
  border-color: #f97316;
}

.filter-chip--cat {
  padding: 5px 14px;
  font-size: 12px;
}

.filter-chip--active-cat {
  background: var(--ta-primary, #0d9488);
  color: #fff;
  border-color: var(--ta-primary, #0d9488);
}

/* ═══════════════════════════════════════
   COUNT BAR
   ═══════════════════════════════════════ */
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

/* ═══════════════════════════════════════
   CARD LIST
   ═══════════════════════════════════════ */
.card-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 12px;
}

.ticket-card {
  background: #fff;
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;
  animation: cardIn 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
}
.ticket-card:active {
  transform: scale(0.98);
}

@keyframes cardIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

/* ── Card: Top ── */
.ticket-card__top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}

.ticket-card__title {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  line-height: 1.35;
  flex: 1;
}

.ticket-card__badges {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  padding-top: 2px;
}

.mini-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 8px;
  font-size: 10px;
  font-weight: 600;
  line-height: 1.5;
  white-space: nowrap;
}

/* ── Card: Meta ── */
.ticket-card__meta {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.ticket-card__info {
  display: flex;
  align-items: center;
  gap: 3px;
  font-size: 12px;
  color: #64748b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ── Card: Bottom ── */
.ticket-card__bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.cat-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}

.ticket-card__bottom-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.ticket-card__admin {
  display: flex;
  align-items: center;
  gap: 2px;
  font-size: 11px;
  color: #64748b;
}

.ticket-card__time {
  font-size: 11px;
  color: #94a3b8;
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
  padding: 64px 0 24px;
  animation: fadeSlideUp 0.4s both;
}

.empty-text {
  font-size: 14px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
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
