<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { showDialog, showToast } from 'vant'
import request from '@/utils/request'
import StatusTag from '@/components/StatusTag.vue'

// ── State ──
const loading = ref(false)
const refreshing = ref(false)
const finished = ref(false)
const data = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const search = ref('')
const activeSource = ref('')
const resolvedFilter = ref<string>('') // '' = all, 'false' = unresolved, 'true' = resolved
const stats = ref<any>(null)

// ── Source filter options ──
const sourceFilters = [
  { key: '', label: '全部' },
  { key: 'api', label: 'API' },
  { key: 'panic', label: 'Panic' },
  { key: 'cron', label: 'Cron' },
]

const resolvedFilters = [
  { key: '', label: '全部' },
  { key: 'false', label: '未解决' },
  { key: 'true', label: '已解决' },
]

// ── Fetch stats ──
async function fetchStats() {
  try {
    const { data: res } = await request.get('/admin/error-logs/stats')
    stats.value = res.data
  } catch {
    // handled by interceptor
  }
}

// ── Fetch list ──
async function fetchData(append = false) {
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
    if (activeSource.value) params.source = activeSource.value
    if (resolvedFilter.value !== '') params.resolved = resolvedFilter.value
    if (startDate.value) params.start_date = startDate.value
    if (endDate.value) params.end_date = endDate.value

    const { data: res } = await request.get('/admin/error-logs', { params })
    const list = res.data?.list || []
    total.value = res.data?.total || 0
    data.value = append ? [...data.value, ...list] : list
    finished.value = data.value.length >= total.value
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

// ── Date filters ──
const startDate = ref('')
const endDate = ref('')

// ── Handlers ──
async function onRefresh() {
  refreshing.value = true
  await Promise.all([fetchData(false), fetchStats()])
}

async function onLoad() {
  if (finished.value) return
  page.value++
  await fetchData(true)
}

function onSearch(val: string) {
  search.value = val
  fetchData(false)
}

function setSource(key: string) {
  activeSource.value = key
  fetchData(false)
}

function setResolved(key: string) {
  resolvedFilter.value = key
  fetchData(false)
}

async function resolveLog(item: any) {
  try {
    await showDialog({
      title: '标记为已解决',
      message: '请输入处理备注（可选）',
      showCancelButton: true,
      showInput: true,
      inputPlaceholder: '处理备注...',
      confirmButtonColor: '#0d9488',
    })
    // @ts-ignore — Vant dialog input value
    const note = (window as any).__vant_dialog_input_value || ''
    await request.put(`/admin/error-logs/${item.id}/resolve`, {
      resolution_note: note,
    })
    showToast('已标记为已解决')
    fetchData(false)
    fetchStats()
  } catch {
    // cancelled or error
  }
}

// ── Helpers ──
function timeAgo(dateStr: string): string {
  if (!dateStr) return ''
  const diff = Date.now() - new Date(dateStr).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return '刚刚'
  if (mins < 60) return `${mins}分钟前`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}小时前`
  return `${Math.floor(hours / 24)}天前`
}

function sourceColor(source: string): string {
  const map: Record<string, string> = {
    api: '#3b82f6',
    panic: '#ef4444',
    cron: '#f59e0b',
    background: '#8b5cf6',
  }
  return map[source] || '#64748b'
}

function sourceLabel(source: string): string {
  const map: Record<string, string> = {
    api: 'API',
    panic: 'Panic',
    cron: 'Cron',
    background: 'Background',
  }
  return map[source] || source || '-'
}

onMounted(() => {
  fetchData(false)
  fetchStats()
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
          <h2 class="hero-title">错误日志</h2>
          <span class="hero-time">{{ new Date().toLocaleDateString('zh-CN', { month: 'long', day: 'numeric' }) }}</span>
        </div>

        <div v-if="stats" class="hero-stats">
          <div class="hero-stat hero-stat--main">
            <div class="hero-stat__value">{{ stats.unresolved ?? 0 }}</div>
            <div class="hero-stat__label">未解决</div>
          </div>
          <div class="hero-stat">
            <div class="hero-stat__value">{{ stats.today_count ?? 0 }}</div>
            <div class="hero-stat__label">今日新增</div>
          </div>
          <div class="hero-stat">
            <div class="hero-stat__value">{{ stats.total ?? 0 }}</div>
            <div class="hero-stat__label">总计</div>
          </div>
        </div>

        <!-- Skeleton -->
        <div v-if="!stats" class="hero-skeleton">
          <div class="skel-bar skel-bar--lg" />
          <div class="skel-bar skel-bar--sm" />
        </div>
      </div>
    </div>

    <!-- ═══════ SEARCH ═══════ -->
    <div class="search-wrap">
      <van-search
        v-model="search"
        placeholder="搜索错误信息..."
        shape="round"
        @search="onSearch"
        @clear="onSearch('')"
      />
    </div>

    <!-- ═══════ FILTERS ═══════ -->
    <div class="filters">
      <div class="filter-group">
        <div
          v-for="s in sourceFilters"
          :key="s.key"
          class="filter-chip"
          :class="{ 'filter-chip--active': activeSource === s.key }"
          @click="setSource(s.key)"
        >
          {{ s.label }}
        </div>
      </div>
      <div class="filter-divider" />
      <div class="filter-group">
        <div
          v-for="r in resolvedFilters"
          :key="r.key"
          class="filter-chip filter-chip--resolved"
          :class="{ 'filter-chip--active': resolvedFilter === r.key }"
          @click="setResolved(r.key)"
        >
          {{ r.label }}
        </div>
      </div>
    </div>

    <!-- ═══════ COUNT BAR ═══════ -->
    <div v-if="total > 0" class="count-bar">
      <span class="count-text">共 <b>{{ total }}</b> 条</span>
    </div>

    <!-- ═══════ LIST ═══════ -->
    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <van-list v-model:loading="loading" :finished="finished" finished-text="" @load="onLoad">
        <div class="card-list">
          <van-swipe-cell v-for="(item, idx) in data" :key="item.id">
            <div
              class="error-card"
              :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
            >
              <!-- Card header: error code + source + resolved -->
              <div class="error-card__head">
                <div class="error-card__badges">
                  <span class="badge badge--code">
                    {{ item.error_code || 'ERR' }}
                  </span>
                  <span
                    class="badge badge--source"
                    :style="{
                      background: `${sourceColor(item.source)}14`,
                      color: sourceColor(item.source),
                    }"
                  >
                    {{ sourceLabel(item.source) }}
                  </span>
                  <span v-if="item.resolved" class="badge badge--resolved">
                    已解决
                  </span>
                </div>
                <span class="error-card__time">{{ timeAgo(item.created_at) }}</span>
              </div>

              <!-- Error message -->
              <div class="error-card__message">
                {{ item.error_message || item.message || '-' }}
              </div>

              <!-- Request path -->
              <div v-if="item.request_path" class="error-card__path">
                {{ item.request_path }}
              </div>
            </div>

            <!-- Swipe action -->
            <template #right>
              <van-button
                v-if="!item.resolved"
                square
                type="success"
                text="解决"
                class="swipe-btn"
                @click="resolveLog(item)"
              />
            </template>
          </van-swipe-cell>
        </div>

        <!-- Empty -->
        <div v-if="!loading && !data.length" class="empty-state">
          <van-icon name="checked" size="40" color="#cbd5e1" />
          <span>暂无错误日志</span>
        </div>
      </van-list>
    </van-pull-refresh>
  </div>
</template>

<style scoped>
.page {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(16px + env(safe-area-inset-bottom, 0px));
}

/* ═══════════════════════════════════════
   HERO — Red Theme
   ═══════════════════════════════════════ */
.hero {
  position: relative;
  overflow: hidden;
  border-radius: 0 0 28px 28px;
  padding-bottom: 22px;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #450a0a 0%, #b91c1c 35%, #ef4444 65%, #dc2626 100%);
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
  background: rgba(252, 165, 165, 0.2);
  top: -30px;
  right: -20px;
}
.hero-orb--2 {
  width: 140px;
  height: 140px;
  background: rgba(239, 68, 68, 0.2);
  bottom: 0;
  left: -20px;
}

.hero-content {
  position: relative;
  z-index: 1;
  padding: 24px 20px 0;
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

.hero-time {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.5);
  font-weight: 500;
}

/* ── Hero Stats ── */
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
  font-size: 22px;
  font-weight: 700;
  color: #fff;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.03em;
  line-height: 1.2;
}

.hero-stat--main .hero-stat__value {
  font-size: 26px;
}

.hero-stat__label {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.55);
  margin-top: 4px;
  font-weight: 500;
}

/* ── Hero Skeleton ── */
.hero-skeleton {
  display: flex;
  gap: 10px;
  padding: 4px 0;
}

.skel-bar {
  height: 12px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.12);
}

.skel-bar--lg {
  width: 60%;
  height: 24px;
}

.skel-bar--sm {
  width: 30%;
  height: 8px;
}

/* ═══════════════════════════════════════
   SEARCH
   ═══════════════════════════════════════ */
.search-wrap {
  padding: 4px 0 0;
}

.search-wrap :deep(.van-search) {
  padding: 8px 12px;
}

/* ═══════════════════════════════════════
   FILTERS
   ═══════════════════════════════════════ */
.filters {
  padding: 4px 16px 6px;
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

.filter-divider {
  height: 6px;
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
  background: #ef4444;
  box-shadow: 0 2px 8px rgba(239, 68, 68, 0.3);
}

.filter-chip--resolved.filter-chip--active {
  background: #0d9488;
  box-shadow: 0 2px 8px rgba(13, 148, 136, 0.3);
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

.error-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  box-shadow:
    0 1px 2px rgba(0, 0, 0, 0.03),
    0 2px 8px rgba(0, 0, 0, 0.04);
  animation: cardIn 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
  transition: transform 0.15s, box-shadow 0.15s;
}

.error-card:active {
  transform: scale(0.98);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
}

/* ── Card: Head ── */
.error-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.error-card__badges {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  overflow: hidden;
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

.badge--code {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}

.badge--source {
  /* dynamic color from inline style */
}

.badge--resolved {
  background: rgba(16, 185, 129, 0.1);
  color: #10b981;
}

.error-card__time {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  flex-shrink: 0;
  white-space: nowrap;
}

/* ── Card: Message ── */
.error-card__message {
  font-size: 13px;
  font-weight: 500;
  color: var(--ta-text-primary, #1e293b);
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  margin-bottom: 6px;
}

/* ── Card: Path ── */
.error-card__path {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  letter-spacing: -0.02em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  background: var(--ta-bg-secondary, #f8fafc);
  padding: 4px 8px;
  border-radius: 6px;
}

/* ── Swipe button ── */
.swipe-btn {
  height: 100% !important;
  border-radius: 0 14px 14px 0 !important;
}

/* ═══════════════════════════════════════
   EMPTY
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
