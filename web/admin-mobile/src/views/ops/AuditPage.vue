<script setup lang="ts">
import { ref, onMounted } from 'vue'
import request from '@/utils/request'

// ═══════════════════════════════════════
// Tab state
// ═══════════════════════════════════════
const activeTab = ref(0)

// ── Tab 1: Operation Logs ──
const opLoading = ref(false)
const opFinished = ref(false)
const opPage = ref(1)
const opData = ref<any[]>([])
const opTotal = ref(0)
const opRefreshing = ref(false)
const opAction = ref('')
const expandedOp = ref<Set<number>>(new Set())

// ── Tab 2: Sensitive Logs ──
const sensLoading = ref(false)
const sensFinished = ref(false)
const sensPage = ref(1)
const sensData = ref<any[]>([])
const sensTotal = ref(0)
const sensRefreshing = ref(false)
const sensResourceType = ref('')

// ── Tab 3: Request Logs ──
const reqLoading = ref(false)
const reqFinished = ref(false)
const reqPage = ref(1)
const reqData = ref<any[]>([])
const reqTotal = ref(0)
const reqRefreshing = ref(false)
const reqMethod = ref('')

// ═══════════════════════════════════════
// Filter options
// ═══════════════════════════════════════
const actionFilters = [
  { key: '', label: '全部' },
  { key: 'create', label: '创建' },
  { key: 'update', label: '更新' },
  { key: 'delete', label: '删除' },
  { key: 'login', label: '登录' },
]

const resourceTypeFilters = [
  { key: '', label: '全部' },
  { key: 'user', label: '用户' },
  { key: 'tenant', label: '租户' },
  { key: 'key', label: '密钥' },
  { key: 'channel', label: '渠道' },
]

const methodFilters = [
  { key: '', label: '全部' },
  { key: 'GET', label: 'GET' },
  { key: 'POST', label: 'POST' },
  { key: 'PUT', label: 'PUT' },
  { key: 'DELETE', label: 'DELETE' },
]

// ═══════════════════════════════════════
// Fetch functions
// ═══════════════════════════════════════
async function fetchOpLogs(append = false) {
  if (!append) {
    opPage.value = 1
    opFinished.value = false
  }
  opLoading.value = true
  try {
    const params: any = { page: opPage.value, page_size: 20 }
    if (opAction.value) params.action = opAction.value
    const { data: res } = await request.get('/admin/audit/operation-logs', { params })
    const list = res.data?.list || []
    opTotal.value = res.data?.total || 0
    opData.value = append ? [...opData.value, ...list] : list
    opFinished.value = opData.value.length >= opTotal.value
  } catch {
    // handled by interceptor
  } finally {
    opLoading.value = false
    opRefreshing.value = false
  }
}

async function fetchSensLogs(append = false) {
  if (!append) {
    sensPage.value = 1
    sensFinished.value = false
  }
  sensLoading.value = true
  try {
    const params: any = { page: sensPage.value, page_size: 20 }
    if (sensResourceType.value) params.resource_type = sensResourceType.value
    const { data: res } = await request.get('/admin/audit/sensitive-logs', { params })
    const list = res.data?.list || []
    sensTotal.value = res.data?.total || 0
    sensData.value = append ? [...sensData.value, ...list] : list
    sensFinished.value = sensData.value.length >= sensTotal.value
  } catch {
    // handled by interceptor
  } finally {
    sensLoading.value = false
    sensRefreshing.value = false
  }
}

async function fetchReqLogs(append = false) {
  if (!append) {
    reqPage.value = 1
    reqFinished.value = false
  }
  reqLoading.value = true
  try {
    const params: any = { page: reqPage.value, page_size: 20 }
    if (reqMethod.value) params.method = reqMethod.value
    const { data: res } = await request.get('/admin/audit/request-logs', { params })
    const list = res.data?.list || []
    reqTotal.value = res.data?.total || 0
    reqData.value = append ? [...reqData.value, ...list] : list
    reqFinished.value = reqData.value.length >= reqTotal.value
  } catch {
    // handled by interceptor
  } finally {
    reqLoading.value = false
    reqRefreshing.value = false
  }
}

// ═══════════════════════════════════════
// Tab change handler
// ═══════════════════════════════════════
function onTabChange(index: number) {
  if (index === 0 && opData.value.length === 0) fetchOpLogs()
  if (index === 1 && sensData.value.length === 0) fetchSensLogs()
  if (index === 2 && reqData.value.length === 0) fetchReqLogs()
}

// ═══════════════════════════════════════
// Load more handlers
// ═══════════════════════════════════════
function onOpLoad() {
  if (opFinished.value) return
  opPage.value++
  fetchOpLogs(true)
}
function onSensLoad() {
  if (sensFinished.value) return
  sensPage.value++
  fetchSensLogs(true)
}
function onReqLoad() {
  if (reqFinished.value) return
  reqPage.value++
  fetchReqLogs(true)
}

// ═══════════════════════════════════════
// Refresh handlers
// ═══════════════════════════════════════
function onOpRefresh() {
  opRefreshing.value = true
  fetchOpLogs(false)
}
function onSensRefresh() {
  sensRefreshing.value = true
  fetchSensLogs(false)
}
function onReqRefresh() {
  reqRefreshing.value = true
  fetchReqLogs(false)
}

// ═══════════════════════════════════════
// Filter setters
// ═══════════════════════════════════════
function setOpAction(key: string) {
  opAction.value = key
  fetchOpLogs(false)
}
function setSensResourceType(key: string) {
  sensResourceType.value = key
  fetchSensLogs(false)
}
function setReqMethod(key: string) {
  reqMethod.value = key
  fetchReqLogs(false)
}

// ═══════════════════════════════════════
// Expand / collapse for operation log detail
// ═══════════════════════════════════════
function toggleOpExpand(id: number) {
  if (expandedOp.value.has(id)) {
    expandedOp.value.delete(id)
  } else {
    expandedOp.value.add(id)
  }
}

// ═══════════════════════════════════════
// Formatting helpers
// ═══════════════════════════════════════
function formatTime(dateStr: string | undefined): string {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const hour = String(d.getHours()).padStart(2, '0')
  const min = String(d.getMinutes()).padStart(2, '0')
  const sec = String(d.getSeconds()).padStart(2, '0')
  return `${month}-${day} ${hour}:${min}:${sec}`
}

function actionColor(action: string): string {
  const map: Record<string, string> = {
    create: '#10b981',
    update: '#3b82f6',
    delete: '#ef4444',
    login: '#8b5cf6',
  }
  return map[action] || '#64748b'
}

function actionBg(action: string): string {
  const map: Record<string, string> = {
    create: 'rgba(16,185,129,0.1)',
    update: 'rgba(59,130,246,0.1)',
    delete: 'rgba(239,68,68,0.08)',
    login: 'rgba(139,92,246,0.1)',
  }
  return map[action] || 'rgba(100,116,139,0.08)'
}

function actionLabel(action: string): string {
  const map: Record<string, string> = {
    create: '创建',
    update: '更新',
    delete: '删除',
    login: '登录',
  }
  return map[action] || action || '-'
}

function methodColor(method: string): string {
  const map: Record<string, string> = {
    GET: '#10b981',
    POST: '#3b82f6',
    PUT: '#f59e0b',
    DELETE: '#ef4444',
  }
  return map[method] || '#64748b'
}

function methodBg(method: string): string {
  const map: Record<string, string> = {
    GET: 'rgba(16,185,129,0.1)',
    POST: 'rgba(59,130,246,0.1)',
    PUT: 'rgba(245,158,11,0.1)',
    DELETE: 'rgba(239,68,68,0.08)',
  }
  return map[method] || 'rgba(100,116,139,0.08)'
}

function statusColor(code: number | undefined): string {
  if (!code) return '#94a3b8'
  if (code < 400) return '#10b981'
  if (code < 500) return '#f59e0b'
  return '#ef4444'
}

function statusBg(code: number | undefined): string {
  if (!code) return 'rgba(148,163,184,0.08)'
  if (code < 400) return 'rgba(16,185,129,0.08)'
  if (code < 500) return 'rgba(245,158,11,0.08)'
  return 'rgba(239,68,68,0.08)'
}

function latencyColor(ms: number | undefined): string {
  if (ms === undefined || ms === null) return '#94a3b8'
  if (ms < 500) return '#10b981'
  if (ms < 2000) return '#f59e0b'
  return '#ef4444'
}

function formatDetail(detail: any): string {
  if (!detail) return ''
  if (typeof detail === 'string') return detail
  try {
    return JSON.stringify(detail, null, 2)
  } catch {
    return String(detail)
  }
}

onMounted(() => {
  fetchOpLogs()
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
        <div class="hero-icon-row">
          <div class="hero-icon">
            <van-icon name="shield-o" size="24" color="#fff" />
          </div>
          <h2 class="hero-title">安全审计</h2>
        </div>
      </div>
    </div>

    <!-- ═══════ TABS ═══════ -->
    <div class="tabs-wrap">
      <van-tabs
        v-model:active="activeTab"
        :lazy-render="true"
        :animated="true"
        :swipeable="true"
        @change="onTabChange"
      >
        <!-- ── Tab 1: Operation Logs ── -->
        <van-tab title="操作日志">
          <!-- Action filter chips -->
          <div class="filter-scroll">
            <div
              v-for="f in actionFilters"
              :key="f.key"
              class="filter-chip"
              :class="{ 'filter-chip--active': opAction === f.key }"
              @click="setOpAction(f.key)"
            >
              {{ f.label }}
            </div>
          </div>

          <!-- Count -->
          <div v-if="opTotal > 0" class="count-bar">
            <span class="count-text">共 <b>{{ opTotal }}</b> 条记录</span>
          </div>

          <!-- List -->
          <van-pull-refresh v-model="opRefreshing" @refresh="onOpRefresh">
            <van-list v-model:loading="opLoading" :finished="opFinished" finished-text="" @load="onOpLoad">
              <div class="card-list">
                <div
                  v-for="(item, idx) in opData"
                  :key="item.id"
                  class="audit-card"
                  :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
                  @click="toggleOpExpand(item.id)"
                >
                  <!-- Top: user + action badge -->
                  <div class="audit-card__top">
                    <span class="audit-card__user">{{ item.user_name || '-' }}</span>
                    <span
                      class="audit-card__badge"
                      :style="{ color: actionColor(item.action), background: actionBg(item.action) }"
                    >
                      {{ actionLabel(item.action) }}
                    </span>
                  </div>

                  <!-- Resource -->
                  <div class="audit-card__resource">
                    <span class="audit-card__res-type">{{ item.resource_type || '-' }}</span>
                    <span v-if="item.resource_id" class="audit-card__res-id">#{{ item.resource_id }}</span>
                  </div>

                  <!-- IP -->
                  <div class="audit-card__ip">{{ item.ip_address || '-' }}</div>

                  <!-- Detail (expandable) -->
                  <div v-if="expandedOp.has(item.id) && item.detail" class="audit-card__detail">
                    <pre class="detail-pre">{{ formatDetail(item.detail) }}</pre>
                  </div>

                  <!-- Time -->
                  <div class="audit-card__bottom">
                    <span class="audit-card__time">{{ formatTime(item.created_at) }}</span>
                    <van-icon
                      :name="expandedOp.has(item.id) ? 'arrow-up' : 'arrow-down'"
                      size="12"
                      color="#94a3b8"
                    />
                  </div>
                </div>
              </div>

              <div v-if="!opLoading && !opData.length" class="empty-state">
                <van-icon name="records" size="40" color="#cbd5e1" />
                <span>暂无操作日志</span>
              </div>
            </van-list>
          </van-pull-refresh>
        </van-tab>

        <!-- ── Tab 2: Sensitive Logs ── -->
        <van-tab title="敏感日志">
          <!-- Resource type filter chips -->
          <div class="filter-scroll">
            <div
              v-for="f in resourceTypeFilters"
              :key="f.key"
              class="filter-chip"
              :class="{ 'filter-chip--active': sensResourceType === f.key }"
              @click="setSensResourceType(f.key)"
            >
              {{ f.label }}
            </div>
          </div>

          <!-- Count -->
          <div v-if="sensTotal > 0" class="count-bar">
            <span class="count-text">共 <b>{{ sensTotal }}</b> 条记录</span>
          </div>

          <!-- List -->
          <van-pull-refresh v-model="sensRefreshing" @refresh="onSensRefresh">
            <van-list v-model:loading="sensLoading" :finished="sensFinished" finished-text="" @load="onSensLoad">
              <div class="card-list">
                <div
                  v-for="(item, idx) in sensData"
                  :key="item.id"
                  class="audit-card"
                  :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
                >
                  <!-- Top: user + resource type badge -->
                  <div class="audit-card__top">
                    <span class="audit-card__user">{{ item.user_name || '-' }}</span>
                    <span class="audit-card__badge" style="color:#8b5cf6;background:rgba(139,92,246,0.1)">
                      {{ item.resource_type || '-' }}
                    </span>
                  </div>

                  <!-- Action -->
                  <div class="audit-card__action">{{ item.action || '-' }}</div>

                  <!-- Reason -->
                  <div v-if="item.reason" class="audit-card__reason">{{ item.reason }}</div>

                  <!-- IP -->
                  <div class="audit-card__ip">{{ item.ip_address || '-' }}</div>

                  <!-- Time -->
                  <div class="audit-card__bottom">
                    <span class="audit-card__time">{{ formatTime(item.created_at) }}</span>
                  </div>
                </div>
              </div>

              <div v-if="!sensLoading && !sensData.length" class="empty-state">
                <van-icon name="shield-o" size="40" color="#cbd5e1" />
                <span>暂无敏感日志</span>
              </div>
            </van-list>
          </van-pull-refresh>
        </van-tab>

        <!-- ── Tab 3: Request Logs ── -->
        <van-tab title="请求日志">
          <!-- Method filter chips -->
          <div class="filter-scroll">
            <div
              v-for="f in methodFilters"
              :key="f.key"
              class="filter-chip"
              :class="{ 'filter-chip--active': reqMethod === f.key }"
              @click="setReqMethod(f.key)"
            >
              {{ f.label }}
            </div>
          </div>

          <!-- Count -->
          <div v-if="reqTotal > 0" class="count-bar">
            <span class="count-text">共 <b>{{ reqTotal }}</b> 条记录</span>
          </div>

          <!-- List -->
          <van-pull-refresh v-model="reqRefreshing" @refresh="onReqRefresh">
            <van-list v-model:loading="reqLoading" :finished="reqFinished" finished-text="" @load="onReqLoad">
              <div class="card-list">
                <div
                  v-for="(item, idx) in reqData"
                  :key="item.id"
                  class="audit-card"
                  :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
                >
                  <!-- Method badge + path -->
                  <div class="audit-card__method-row">
                    <span
                      class="audit-card__method"
                      :style="{ color: methodColor(item.method), background: methodBg(item.method) }"
                    >
                      {{ item.method || '-' }}
                    </span>
                    <span class="audit-card__path">{{ item.path || '-' }}</span>
                  </div>

                  <!-- Status + latency -->
                  <div class="audit-card__meta-row">
                    <span
                      class="audit-card__status"
                      :style="{ color: statusColor(item.status_code), background: statusBg(item.status_code) }"
                    >
                      {{ item.status_code || '-' }}
                    </span>
                    <span
                      class="audit-card__latency"
                      :style="{ color: latencyColor(item.latency_ms) }"
                    >
                      {{ item.latency_ms !== undefined ? `${item.latency_ms}ms` : '-' }}
                    </span>
                  </div>

                  <!-- Tenant + user -->
                  <div class="audit-card__user-row">
                    <span v-if="item.tenant_name" class="audit-card__tenant">{{ item.tenant_name }}</span>
                    <span v-if="item.username" class="audit-card__username">{{ item.username }}</span>
                  </div>

                  <!-- Time -->
                  <div class="audit-card__bottom">
                    <span class="audit-card__time">{{ formatTime(item.created_at) }}</span>
                  </div>
                </div>
              </div>

              <div v-if="!reqLoading && !reqData.length" class="empty-state">
                <van-icon name="records" size="40" color="#cbd5e1" />
                <span>暂无请求日志</span>
              </div>
            </van-list>
          </van-pull-refresh>
        </van-tab>
      </van-tabs>
    </div>
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

.hero-icon-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.hero-icon {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.15);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.12);
  display: flex;
  align-items: center;
  justify-content: center;
}

.hero-title {
  font-size: 22px;
  font-weight: 700;
  color: #fff;
  margin: 0;
  letter-spacing: -0.02em;
}

/* ═══════════════════════════════════════
   TABS WRAP
   ═══════════════════════════════════════ */
.tabs-wrap {
  margin-top: -8px;
  position: relative;
  z-index: 2;
}

/* Override Vant tabs to teal active color */
.tabs-wrap :deep(.van-tabs__nav) {
  background: var(--ta-bg-card, #fff);
}
.tabs-wrap :deep(.van-tab--active) {
  color: var(--ta-primary, #0d9488);
}
.tabs-wrap :deep(.van-tabs__line) {
  background: var(--ta-primary, #0d9488);
}
.tabs-wrap :deep(.van-tab) {
  font-size: 13px;
  font-weight: 500;
  color: var(--ta-text-secondary, #64748b);
}

/* ═══════════════════════════════════════
   FILTER CHIPS
   ═══════════════════════════════════════ */
.filter-scroll {
  display: flex;
  gap: 8px;
  padding: 10px 16px 6px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
}
.filter-scroll::-webkit-scrollbar {
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
  background: var(--ta-primary, #0d9488);
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

.audit-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  box-shadow:
    0 1px 2px rgba(0, 0, 0, 0.03),
    0 2px 8px rgba(0, 0, 0, 0.04);
  animation: cardIn 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
  transition: transform 0.15s;
}
.audit-card:active {
  transform: scale(0.99);
}

/* ── Card: Top row (user + badge) ── */
.audit-card__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}

.audit-card__user {
  font-size: 14px;
  font-weight: 600;
  color: var(--ta-text-primary, #1e293b);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.audit-card__badge {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  line-height: 1.4;
  white-space: nowrap;
}

/* ── Card: Resource row ── */
.audit-card__resource {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 4px;
  font-size: 12px;
}
.audit-card__res-type {
  color: var(--ta-text-secondary, #475569);
  font-weight: 500;
}
.audit-card__res-id {
  color: var(--ta-text-tertiary, #94a3b8);
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  font-size: 11px;
}

/* ── Card: IP address (monospace) ── */
.audit-card__ip {
  font-size: 11px;
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  color: var(--ta-text-tertiary, #94a3b8);
  letter-spacing: -0.02em;
  margin-bottom: 2px;
}

/* ── Card: Expandable detail ── */
.audit-card__detail {
  margin: 8px 0;
  background: var(--ta-bg-secondary, #f8fafc);
  border-radius: 8px;
  padding: 10px;
  overflow-x: auto;
}
.detail-pre {
  margin: 0;
  font-size: 11px;
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  color: var(--ta-text-secondary, #475569);
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
}

/* ── Card: Action row (sensitive logs) ── */
.audit-card__action {
  font-size: 12px;
  font-weight: 500;
  color: var(--ta-text-secondary, #475569);
  margin-bottom: 2px;
}

/* ── Card: Reason (sensitive logs) ── */
.audit-card__reason {
  font-size: 12px;
  color: var(--ta-text-tertiary, #94a3b8);
  line-height: 1.5;
  margin-bottom: 2px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

/* ── Card: Method row (request logs) ── */
.audit-card__method-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.audit-card__method {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 6px;
  letter-spacing: 0.03em;
}
.audit-card__path {
  font-size: 12px;
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  color: var(--ta-text-primary, #1e293b);
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  letter-spacing: -0.02em;
}

/* ── Card: Status + latency row (request logs) ── */
.audit-card__meta-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.audit-card__status {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 6px;
  font-variant-numeric: tabular-nums;
}
.audit-card__latency {
  font-size: 11px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

/* ── Card: User row (request logs) ── */
.audit-card__user-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 2px;
  font-size: 12px;
}
.audit-card__tenant {
  color: var(--ta-text-secondary, #475569);
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.audit-card__username {
  color: var(--ta-text-tertiary, #94a3b8);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ── Card: Bottom row ── */
.audit-card__bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 4px;
}

.audit-card__time {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 400;
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
