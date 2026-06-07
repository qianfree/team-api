<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import request from '@/utils/request'

const router = useRouter()

const loading = ref(true)
const stats = ref<any>(null)
const channelHealth = ref<any[]>([])
const recentAlerts = ref<any[]>([])
const monitorData = ref<any>(null)

function ensureArray(data: any): any[] {
  if (Array.isArray(data)) return data
  if (data && typeof data === 'object' && Array.isArray(data.list)) return data.list
  return []
}

async function fetchDashboard() {
  loading.value = true
  try {
    const [dashRes, healthRes, alertsRes, monitorRes] = await Promise.allSettled([
      request.get('/admin/dashboard'),
      request.get('/admin/dashboard/channel-health'),
      request.get('/admin/dashboard/recent-alerts'),
      request.get('/admin/monitor/dashboard', { params: { minutes: 5 } }),
    ])

    if (dashRes.status === 'fulfilled') {
      stats.value = dashRes.value.data?.data
    }
    if (healthRes.status === 'fulfilled') {
      channelHealth.value = ensureArray(healthRes.value.data?.data)
    }
    if (alertsRes.status === 'fulfilled') {
      recentAlerts.value = ensureArray(alertsRes.value.data?.data)
    }
    if (monitorRes.status === 'fulfilled') {
      monitorData.value = monitorRes.value.data?.data
    }
  } catch {
    // error handled by interceptor
  } finally {
    loading.value = false
  }
}

function formatNumber(n: number | undefined): string {
  if (n === undefined || n === null) return '-'
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toLocaleString()
}

function formatCost(n: number | undefined): string {
  if (n === undefined || n === null) return '-'
  return `$${n.toFixed(2)}`
}

function formatCompact(n: number | undefined): string {
  if (n === undefined || n === null) return '0'
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

const successRate = computed(() => {
  const r = stats.value?.today?.success_rate
  return r ? Math.min(100, Math.max(0, r)) : 0
})

const successDashArray = computed(() => {
  const circumference = 2 * Math.PI * 28
  const offset = circumference * (1 - successRate.value / 100)
  return `${circumference - offset} ${circumference}`
})

function healthColor(score: number): string {
  if (score >= 80) return '#10b981'
  if (score >= 50) return '#f59e0b'
  return '#ef4444'
}

function healthBg(score: number): string {
  if (score >= 80) return 'rgba(16,185,129,0.1)'
  if (score >= 50) return 'rgba(245,158,11,0.1)'
  return 'rgba(239,68,68,0.1)'
}

function alertSeverityColor(severity: string): string {
  if (severity === 'critical') return '#ef4444'
  if (severity === 'warning') return '#f59e0b'
  return '#0d9488'
}

function alertSeverityBg(severity: string): string {
  if (severity === 'critical') return 'rgba(239,68,68,0.08)'
  if (severity === 'warning') return 'rgba(245,158,11,0.08)'
  return 'rgba(13,148,136,0.08)'
}

function alertSeverityLabel(severity: string): string {
  if (severity === 'critical') return '严重'
  if (severity === 'warning') return '警告'
  return '信息'
}

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

// System monitoring helpers
function usageColor(pct: number | undefined): string {
  if (pct === undefined || pct === null) return '#94a3b8'
  if (pct >= 90) return '#ef4444'
  if (pct >= 70) return '#f59e0b'
  return '#10b981'
}

function pct(p: number | undefined): string {
  if (p === undefined || p === null) return '-'
  return `${p.toFixed(1)}%`
}

function mb(m: number | undefined): string {
  if (m === undefined || m === null) return '-'
  if (m >= 1024) return `${(m / 1024).toFixed(1)}G`
  return `${m.toFixed(0)}M`
}

function ms(m: number | undefined): string {
  if (m === undefined || m === null) return '-'
  if (m >= 1000) return `${(m / 1000).toFixed(1)}s`
  return `${m.toFixed(0)}ms`
}

onMounted(fetchDashboard)
</script>

<template>
  <div class="dash">
    <van-pull-refresh v-model="loading" @refresh="fetchDashboard">

      <!-- ═══════ HERO ═══════ -->
      <div class="hero">
        <div class="hero-bg">
          <div class="hero-orb hero-orb--1" />
          <div class="hero-orb hero-orb--2" />
          <div class="hero-orb hero-orb--3" />
        </div>

        <div class="hero-content">
          <!-- Greeting -->
          <div class="hero-greeting">
            <span class="hero-label">状态概览</span>
            <span class="hero-time">{{ new Date().toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'short' }) }}</span>
          </div>

          <!-- Hero Metrics Row -->
          <div v-if="stats" class="hero-metrics">
            <!-- Requests -->
            <div class="hero-metric hero-metric--main">
              <div class="hero-metric__value">{{ formatCompact(stats.today?.requests) }}</div>
              <div class="hero-metric__label">今日请求</div>
            </div>

            <!-- Cost -->
            <div class="hero-metric">
              <div class="hero-metric__value">{{ formatCost(stats.today?.cost) }}</div>
              <div class="hero-metric__label">今日费用</div>
            </div>

            <!-- Success Rate Ring -->
            <div class="hero-ring-wrap">
              <svg class="hero-ring" viewBox="0 0 64 64">
                <circle cx="32" cy="32" r="28" fill="none" stroke="rgba(255,255,255,0.15)" stroke-width="4" />
                <circle
                  cx="32" cy="32" r="28" fill="none"
                  stroke="#5eead4" stroke-width="4"
                  stroke-linecap="round"
                  :stroke-dasharray="successDashArray"
                  transform="rotate(-90 32 32)"
                  class="hero-ring__progress"
                />
              </svg>
              <div class="hero-ring__text">
                <span class="hero-ring__value">{{ stats.today?.success_rate ? stats.today.success_rate.toFixed(1) : '-' }}</span>
                <span class="hero-ring__unit">%</span>
              </div>
              <div class="hero-metric__label" style="margin-top:4px">成功率</div>
            </div>
          </div>

          <!-- Skeleton -->
          <div v-if="loading && !stats" class="hero-skeleton">
            <div class="skeleton-bar skeleton-bar--lg" />
            <div class="skeleton-bar skeleton-bar--md" />
            <div class="skeleton-bar skeleton-bar--sm" />
          </div>
        </div>
      </div>

      <!-- ═══════ CONTENT ═══════ -->
      <div class="content">

        <!-- ── Quick Stats ── -->
        <div v-if="stats" class="quick-stats stagger-1">
          <div class="quick-stat">
            <div class="quick-stat__icon" style="background:rgba(99,102,241,0.1);color:#6366f1">
              <van-icon name="friends-o" size="18" />
            </div>
            <div>
              <div class="quick-stat__value">{{ stats.today?.active_tenants ?? '-' }}</div>
              <div class="quick-stat__label">活跃租户</div>
            </div>
          </div>
          <div class="quick-stat">
            <div class="quick-stat__icon" style="background:rgba(14,165,233,0.1);color:#0ea5e9">
              <van-icon name="chart-trending-o" size="18" />
            </div>
            <div>
              <div class="quick-stat__value">{{ formatCompact(stats.month?.requests) }}</div>
              <div class="quick-stat__label">月请求</div>
            </div>
          </div>
          <div class="quick-stat">
            <div class="quick-stat__icon" style="background:rgba(16,185,129,0.1);color:#10b981">
              <van-icon name="coin-o" size="18" />
            </div>
            <div>
              <div class="quick-stat__value">{{ formatCost(stats.month?.cost) }}</div>
              <div class="quick-stat__label">月费用</div>
            </div>
          </div>
        </div>

        <!-- ── Channel Health ── -->
        <div class="section stagger-2">
          <div class="section-head">
            <h3 class="section-title">渠道健康</h3>
            <span class="section-link" @click="router.push('/m/channel-health')">全部 ›</span>
          </div>

          <!-- Horizontal scroll -->
          <div class="health-scroll">
            <div
              v-for="ch in channelHealth.slice(0, 8)"
              :key="ch.id || ch.channel_id"
              class="health-pill"
              :style="{ background: healthBg(ch.health_score ?? ch.score ?? 0) }"
              @click="router.push(`/m/channels/${ch.id || ch.channel_id}`)"
            >
              <span class="health-dot" :style="{ background: healthColor(ch.health_score ?? ch.score ?? 0) }" />
              <span class="health-name">{{ ch.name || ch.channel_name }}</span>
              <span class="health-score" :style="{ color: healthColor(ch.health_score ?? ch.score ?? 0) }">
                {{ ch.health_score ?? ch.score ?? '-' }}
              </span>
            </div>
            <div v-if="!channelHealth.length" class="health-pill health-pill--empty">
              暂无数据
            </div>
          </div>
        </div>

        <!-- ── System Monitor ── -->
        <div v-if="monitorData" class="section stagger-3">
          <div class="section-head">
            <h3 class="section-title">系统监控</h3>
          </div>

          <!-- Resource bars -->
          <div class="monitor-grid">
            <div class="monitor-tile">
              <div class="monitor-tile__head">
                <span class="monitor-tile__label">CPU</span>
                <span class="monitor-tile__value" :style="{ color: usageColor(monitorData.system?.cpu?.percent) }">
                  {{ pct(monitorData.system?.cpu?.percent) }}
                </span>
              </div>
              <div class="monitor-bar">
                <div
                  class="monitor-bar__fill"
                  :style="{ width: `${Math.min(100, monitorData.system?.cpu?.percent ?? 0)}%`, background: usageColor(monitorData.system?.cpu?.percent) }"
                />
              </div>
            </div>

            <div class="monitor-tile">
              <div class="monitor-tile__head">
                <span class="monitor-tile__label">内存</span>
                <span class="monitor-tile__value" :style="{ color: usageColor(monitorData.system?.memory?.used_percent) }">
                  {{ pct(monitorData.system?.memory?.used_percent) }}
                </span>
              </div>
              <div class="monitor-bar">
                <div
                  class="monitor-bar__fill"
                  :style="{ width: `${Math.min(100, monitorData.system?.memory?.used_percent ?? 0)}%`, background: usageColor(monitorData.system?.memory?.used_percent) }"
                />
              </div>
              <div class="monitor-tile__sub">{{ mb(monitorData.system?.memory?.used_mb) }} / {{ mb(monitorData.system?.memory?.total_mb) }}</div>
            </div>

            <div class="monitor-tile">
              <div class="monitor-tile__head">
                <span class="monitor-tile__label">磁盘</span>
                <span class="monitor-tile__value" :style="{ color: usageColor(monitorData.system?.disk?.used_percent) }">
                  {{ pct(monitorData.system?.disk?.used_percent) }}
                </span>
              </div>
              <div class="monitor-bar">
                <div
                  class="monitor-bar__fill"
                  :style="{ width: `${Math.min(100, monitorData.system?.disk?.used_percent ?? 0)}%`, background: usageColor(monitorData.system?.disk?.used_percent) }"
                />
              </div>
              <div class="monitor-tile__sub">{{ monitorData.system?.disk?.used_gb?.toFixed(1) ?? '-' }}G / {{ monitorData.system?.disk?.total_gb?.toFixed(1) ?? '-' }}G</div>
            </div>
          </div>

          <!-- Runtime / DB / Redis / API row -->
          <div class="monitor-chips">
            <div class="m-chip">
              <span class="m-chip__dot" style="background:#6366f1" />
              <span class="m-chip__label">Goroutine</span>
              <span class="m-chip__val">{{ monitorData.system?.runtime?.goroutine_count ?? monitorData.api?.qps?.toFixed(1) ?? '-' }}</span>
            </div>
            <div class="m-chip">
              <span class="m-chip__dot" style="background:#0ea5e9" />
              <span class="m-chip__label">QPS</span>
              <span class="m-chip__val">{{ monitorData.api?.qps?.toFixed(1) ?? '-' }}</span>
            </div>
            <div class="m-chip">
              <span class="m-chip__dot" style="background:#f59e0b" />
              <span class="m-chip__label">P95</span>
              <span class="m-chip__val">{{ ms(monitorData.api?.latency?.p95) }}</span>
            </div>
            <div class="m-chip">
              <span class="m-chip__dot" style="background:#10b981" />
              <span class="m-chip__label">DB 连接</span>
              <span class="m-chip__val">{{ monitorData.db_pool?.active_connections ?? 0 }}/{{ monitorData.db_pool?.max_connections ?? '-' }}</span>
            </div>
            <div class="m-chip">
              <span class="m-chip__dot" style="background:#ef4444" />
              <span class="m-chip__label">Redis</span>
              <span class="m-chip__val">{{ mb(monitorData.redis_pool?.used_memory_mb) }}</span>
            </div>
            <div class="m-chip">
              <span class="m-chip__dot" style="background:#8b5cf6" />
              <span class="m-chip__label">命中率</span>
              <span class="m-chip__val">{{ monitorData.redis_pool?.hit_rate !== undefined ? `${monitorData.redis_pool.hit_rate.toFixed(1)}%` : '-' }}</span>
            </div>
          </div>
        </div>

        <!-- ── Recent Alerts ── -->
        <div class="section" :class="monitorData ? 'stagger-4' : 'stagger-3'">
          <div class="section-head">
            <h3 class="section-title">最近告警</h3>
            <span class="section-link" @click="router.push('/m/alert-events')">全部 ›</span>
          </div>

          <div class="alert-list">
            <div
              v-for="alert in recentAlerts.slice(0, 5)"
              :key="alert.id"
              class="alert-card"
              :style="{ borderLeftColor: alertSeverityColor(alert.severity), background: alertSeverityBg(alert.severity) }"
            >
              <div class="alert-card__head">
                <span class="alert-card__title">{{ alert.rule_name || alert.title || '告警' }}</span>
                <span class="alert-card__badge" :style="{ background: alertSeverityColor(alert.severity), color: '#fff' }">
                  {{ alertSeverityLabel(alert.severity) }}
                </span>
              </div>
              <div v-if="alert.message || alert.description" class="alert-card__msg">
                {{ alert.message || alert.description }}
              </div>
              <div v-if="alert.created_at || alert.triggered_at" class="alert-card__time">
                {{ timeAgo(alert.created_at || alert.triggered_at) }}
              </div>
            </div>

            <div v-if="!recentAlerts.length" class="alert-empty">
              <van-icon name="checked" size="28" color="#10b981" />
              <span>一切正常，暂无告警</span>
            </div>
          </div>
        </div>

      </div>
    </van-pull-refresh>
  </div>
</template>

<style scoped>
/* ═══════════════════════════════════════
   DASHBOARD — Deep Ocean Aesthetic
   ═══════════════════════════════════════ */

.dash {
  background: #f5f7fa;
  position: relative;
  z-index: 1;
}

/* ── HERO ── */
.hero {
  position: relative;
  overflow: hidden;
  padding-bottom: 20px;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #042f2e 0%, #0f766e 40%, #0d9488 70%, #115e59 100%);
}

.hero-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(60px);
  pointer-events: none;
}
.hero-orb--1 {
  width: 200px; height: 200px;
  background: rgba(94, 234, 212, 0.2);
  top: -40px; right: -30px;
}
.hero-orb--2 {
  width: 160px; height: 160px;
  background: rgba(20, 184, 166, 0.15);
  bottom: 10px; left: -20px;
}
.hero-orb--3 {
  width: 100px; height: 100px;
  background: rgba(6, 182, 212, 0.12);
  top: 30%; left: 50%;
}

.hero-content {
  position: relative;
  z-index: 1;
  padding: 24px 20px 0;
}

.hero-greeting {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 20px;
}
.hero-label {
  font-size: 20px;
  font-weight: 700;
  color: #fff;
  letter-spacing: -0.02em;
}
.hero-time {
  font-size: 12px;
  color: rgba(255,255,255,0.5);
  font-weight: 500;
}

/* ── Hero Metrics ── */
.hero-metrics {
  display: flex;
  gap: 12px;
  align-items: stretch;
}

.hero-metric {
  flex: 1;
  background: rgba(255,255,255,0.1);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(255,255,255,0.12);
  border-radius: 14px;
  padding: 14px;
  display: flex;
  flex-direction: column;
  justify-content: center;
}
.hero-metric--main {
  flex: 1.2;
  background: rgba(255,255,255,0.14);
}

.hero-metric__value {
  font-size: 22px;
  font-weight: 700;
  color: #fff;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.03em;
  line-height: 1.2;
}
.hero-metric--main .hero-metric__value {
  font-size: 26px;
}

.hero-metric__label {
  font-size: 11px;
  color: rgba(255,255,255,0.55);
  margin-top: 4px;
  font-weight: 500;
}

/* ── Success Rate Ring ── */
.hero-ring-wrap {
  flex: 0 0 90px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: rgba(255,255,255,0.1);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(255,255,255,0.12);
  border-radius: 14px;
  padding: 8px 6px 10px;
}

.hero-ring {
  width: 48px;
  height: 48px;
}
.hero-ring__progress {
  transition: stroke-dasharray 0.8s cubic-bezier(0.4, 0, 0.2, 1);
}
.hero-ring__text {
  position: absolute;
  /* We use a relative approach instead */
  margin-top: -38px;
  display: flex;
  align-items: baseline;
  justify-content: center;
  pointer-events: none;
}
.hero-ring-wrap {
  position: relative;
}
.hero-ring__text {
  position: absolute;
  top: 14px;
  left: 50%;
  transform: translateX(-50%);
}
.hero-ring__value {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
  font-variant-numeric: tabular-nums;
}
.hero-ring__unit {
  font-size: 10px;
  color: rgba(255,255,255,0.6);
}

/* ── Hero Skeleton ── */
.hero-skeleton {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 8px 0;
}
.skeleton-bar {
  height: 12px;
  border-radius: 6px;
  background: rgba(255,255,255,0.12);
}
.skeleton-bar--lg { width: 60%; height: 24px; }
.skeleton-bar--md { width: 40%; }
.skeleton-bar--sm { width: 30%; height: 8px; }

/* ═══════════════════════════════════════
   CONTENT
   ═══════════════════════════════════════ */
.content {
  padding: 0 16px 80px;
  margin-top: -8px;
  position: relative;
  z-index: 2;
}

/* ── Quick Stats ── */
.quick-stats {
  display: flex;
  gap: 10px;
  margin-bottom: 20px;
}

.quick-stat {
  flex: 1;
  background: #fff;
  border-radius: 14px;
  padding: 12px 10px;
  display: flex;
  align-items: center;
  gap: 10px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.04), 0 4px 12px rgba(0,0,0,0.03);
}

.quick-stat__icon {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.quick-stat__value {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.02em;
}
.quick-stat__label {
  font-size: 10px;
  color: #94a3b8;
  margin-top: 1px;
}

/* ── Section ── */
.section {
  margin-bottom: 20px;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
  padding: 0 2px;
}

.section-title {
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
  letter-spacing: -0.01em;
}

.section-link {
  font-size: 12px;
  color: #0d9488;
  font-weight: 600;
  cursor: pointer;
}

/* ── Channel Health Horizontal Scroll ── */
.health-scroll {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 4px;
  scroll-snap-type: x mandatory;
  -webkit-overflow-scrolling: touch;
}
.health-scroll::-webkit-scrollbar { display: none; }

.health-pill {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 14px;
  border-radius: 12px;
  scroll-snap-align: start;
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;
}
.health-pill:active {
  transform: scale(0.97);
}

.health-pill--empty {
  color: #94a3b8;
  font-size: 13px;
  cursor: default;
}

.health-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.health-name {
  font-size: 13px;
  font-weight: 600;
  color: #1e293b;
  max-width: 80px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.health-score {
  font-size: 13px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

/* ── Alert Cards ── */
.alert-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.alert-card {
  border-radius: 12px;
  padding: 12px 14px;
  border-left: 3px solid;
  transition: transform 0.15s;
}
.alert-card:active {
  transform: scale(0.99);
}

.alert-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.alert-card__title {
  font-size: 13px;
  font-weight: 600;
  color: #1e293b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.alert-card__badge {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  line-height: 1.4;
}

.alert-card__msg {
  font-size: 12px;
  color: #64748b;
  margin-top: 6px;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.alert-card__time {
  font-size: 11px;
  color: #94a3b8;
  margin-top: 6px;
}

.alert-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 28px 0;
  color: #64748b;
  font-size: 13px;
}

/* ═══════════════════════════════════════
   SYSTEM MONITOR
   ═══════════════════════════════════════ */

.monitor-grid {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 12px;
}

.monitor-tile {
  background: #fff;
  border-radius: 12px;
  padding: 12px 14px 10px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.04), 0 4px 12px rgba(0,0,0,0.03);
}

.monitor-tile__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.monitor-tile__label {
  font-size: 12px;
  font-weight: 600;
  color: #475569;
}

.monitor-tile__value {
  font-size: 13px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.monitor-tile__sub {
  font-size: 11px;
  color: #94a3b8;
  margin-top: 4px;
}

.monitor-bar {
  height: 4px;
  background: #f1f5f9;
  border-radius: 2px;
  overflow: hidden;
}

.monitor-bar__fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.6s cubic-bezier(0.16, 1, 0.3, 1);
}

/* Monitor chips row */
.monitor-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.m-chip {
  display: flex;
  align-items: center;
  gap: 5px;
  background: #fff;
  border-radius: 10px;
  padding: 8px 12px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.04), 0 4px 12px rgba(0,0,0,0.03);
  flex: 1 1 calc(33% - 6px);
  min-width: calc(33% - 6px);
}

.m-chip__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

.m-chip__label {
  font-size: 10px;
  color: #94a3b8;
  font-weight: 500;
}

.m-chip__val {
  font-size: 12px;
  font-weight: 700;
  color: #1e293b;
  font-variant-numeric: tabular-nums;
  margin-left: auto;
}

/* ═══════════════════════════════════════
   STAGGER ANIMATIONS
   ═══════════════════════════════════════ */
.stagger-1, .stagger-2, .stagger-3, .stagger-4 {
  animation: staggerUp 0.5s cubic-bezier(0.16, 1, 0.3, 1) both;
}
.stagger-1 { animation-delay: 0.08s; }
.stagger-2 { animation-delay: 0.16s; }
.stagger-3 { animation-delay: 0.24s; }
.stagger-4 { animation-delay: 0.32s; }

@keyframes staggerUp {
  from {
    opacity: 0;
    transform: translateY(16px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
