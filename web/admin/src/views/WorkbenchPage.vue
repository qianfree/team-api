<script setup lang="ts">
/**
 * 工作台（运营收件箱）
 *
 * 与仪表盘的分工：仪表盘回答「现在怎么样」，工作台回答「我该做什么」。
 * 只有满足「有人要处理 + 不处理会恶化 + 能被清空归零」三条的事才进这里。
 *
 * 待办由后端从各业务源表实时派生，前端不缓存、不自己判定严重度，也不持久化任何状态。
 * 待办没有「标记完成」——唯一的出路是点进源头把问题解决掉，下一轮收集自然就没有它了。
 */
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import {
  IconThunderbolt,
  IconBranch,
  IconIdcard,
  IconMessage,
  IconSafe,
  IconRefresh,
  IconRight,
  IconCheckCircleFill,
  IconExclamationCircle,
  IconFire,
  IconUserGroup,
  IconBarChart,
} from '@arco-design/web-vue/es/icon'
import request from '@/utils/request'
import { formatBilling } from '@/composables/useCurrency'

const router = useRouter()

// ===== 常量 =====
type Severity = 'p0' | 'p1' | 'p2'
type DomainKey = 'availability' | 'money' | 'customer' | 'system'

const DOMAINS: Record<DomainKey, { label: string; icon: any; color: string }> = {
  availability: { label: '服务可用性', icon: IconBranch, color: '#f77234' },
  money: { label: '资金安全', icon: IconIdcard, color: '#14b8a6' },
  customer: { label: '客户运营', icon: IconMessage, color: '#3491fa' },
  system: { label: '系统安全', icon: IconSafe, color: '#8b5cf6' },
}

const SEVERITY: Record<Severity, { label: string; color: string; bg: string }> = {
  p0: { label: '紧急', color: '#f53f3f', bg: 'rgba(245, 63, 63, 0.06)' },
  p1: { label: '今日', color: '#ff7d00', bg: 'transparent' },
  p2: { label: '本周', color: '#86909c', bg: 'transparent' },
}

// 指标图标/配色按 key 固定，不跟随后端返回顺序
const METRIC_STYLE: Record<string, { icon: any; gradient: string }> = {
  today_revenue: { icon: IconBarChart, gradient: 'linear-gradient(135deg, #0d9488, #14b8a6)' },
  error_rate_5m: { icon: IconThunderbolt, gradient: 'linear-gradient(135deg, #ef4444, #f87171)' },
  low_balance_tenants: { icon: IconUserGroup, gradient: 'linear-gradient(135deg, #f59e0b, #fbbf24)' },
  pending_total: { icon: IconFire, gradient: 'linear-gradient(135deg, #6366f1, #818cf8)' },
}

// 路径参数路由：跳转时要走 params 而非 query
const PARAM_ROUTES: Record<string, string[]> = {
  AdminChannelDetail: ['id'],
  AdminTenantDetail: ['id'],
}

// ===== 状态 =====
const loading = ref(false)
const summary = ref<any>(null)
const onlyUrgent = ref(false)
const activeDomain = ref<DomainKey | 'all'>('all')
const lastRefresh = ref<Date | null>(null)

let pollTimer: ReturnType<typeof setInterval> | null = null

// ===== 派生 =====
const items = computed<any[]>(() => summary.value?.items || [])

const filtered = computed(() =>
  items.value.filter(i => {
    if (onlyUrgent.value && i.severity !== 'p0') return false
    if (activeDomain.value !== 'all' && i.domain !== activeDomain.value) return false
    return true
  }),
)

const urgentCount = computed(() => items.value.filter(i => i.severity === 'p0').length)

const domainStats = computed(() => {
  const raw: any[] = summary.value?.domains || []
  return (Object.keys(DOMAINS) as DomainKey[]).map(key => {
    const hit = raw.find(d => d.domain === key)
    return { key, ...DOMAINS[key], total: hit?.total || 0, urgent: hit?.urgent || 0 }
  })
})

// 首屏数字：后端三个 + 前端补一个「待处理事项」（它就是列表长度，没必要让后端再算一遍）
const metrics = computed(() => {
  const list: any[] = (summary.value?.metrics || []).map((m: any) => ({
    key: m.key,
    label: m.label,
    value: formatMetric(m),
    sub: m.sub,
    growth:
      m.growth === null || m.growth === undefined
        ? undefined
        : { value: Math.abs(Math.round(m.growth * 10) / 10), positive: m.growth >= 0 },
    ...(METRIC_STYLE[m.key] || METRIC_STYLE.pending_total),
  }))

  const p1 = items.value.filter(i => i.severity === 'p1').length
  list.push({
    key: 'pending_total',
    label: '待处理事项',
    value: String(items.value.length),
    sub: `紧急 ${urgentCount.value} · 今日 ${p1}`,
    ...METRIC_STYLE.pending_total,
  })
  return list
})

function formatMetric(m: any): string {
  const v = Number(m.value) || 0
  if (m.unit === 'money') return formatBilling(v, 2)
  if (m.unit === 'percent') return v.toFixed(1) + '%'
  return v.toLocaleString()
}

const refreshText = computed(() =>
  lastRefresh.value ? lastRefresh.value.toLocaleTimeString('zh-CN', { hour12: false }) : '—',
)

function timeAgo(iso: string) {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const diffMin = Math.floor((Date.now() - d.getTime()) / 60000)
  if (diffMin < 1) return '刚刚'
  if (diffMin < 60) return `${diffMin} 分钟前`
  const h = Math.floor(diffMin / 60)
  if (h < 24) return `${h} 小时前`
  return `${Math.floor(h / 24)} 天前`
}

// ===== 数据 =====
async function fetchSummary(silent = false) {
  if (!silent) loading.value = true
  try {
    const res = await request.get('/admin/workbench/summary')
    summary.value = res.data?.data || null
    lastRefresh.value = new Date()
  } catch (e: any) {
    if (!silent) Message.error(e.response?.data?.message || '加载工作台失败')
  } finally {
    if (!silent) loading.value = false
  }
}

// 跳转时自动带上筛选条件，让管理员落地即看到问题现场，而不是一个空列表
function goAction(item: any) {
  if (!item.action_route) return
  const q = item.action_query || {}
  const paramKeys = PARAM_ROUTES[item.action_route]
  if (paramKeys) {
    const params: Record<string, string> = {}
    const query: Record<string, string> = {}
    Object.entries(q).forEach(([k, v]) => {
      if (paramKeys.includes(k)) params[k] = String(v)
      else query[k] = String(v)
    })
    router.push({ name: item.action_route, params, query })
    return
  }
  router.push({ name: item.action_route, query: q })
}

function pickDomain(key: DomainKey | 'all') {
  activeDomain.value = activeDomain.value === key ? 'all' : key
}

function availClass(m: any) {
  if (m.available === 0 && m.total > 0) return 'is-dead'
  if (m.available < m.total) return 'is-warn'
  return 'is-ok'
}

onMounted(() => {
  fetchSummary()
  // 后端整份结果有 30s Redis 缓存，前端 60s 轮询不会额外压库
  pollTimer = setInterval(() => fetchSummary(true), 60000)
})

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<template>
  <ASpin :loading="loading" style="width: 100%">
    <div style="width: 100%">
      <!-- 页头 -->
      <div class="wb-header">
        <div>
          <div class="wb-header__title">工作台</div>
          <div class="wb-header__desc">需要你动手处理的事。处理完就清空 —— 这里应该经常是空的。</div>
        </div>
        <div class="wb-header__ops">
          <span class="wb-header__time">更新于 {{ refreshText }}</span>
          <ASwitch v-model="onlyUrgent" size="small" />
          <span class="wb-header__switch-label">仅看紧急</span>
          <AButton size="small" @click="fetchSummary()">
            <template #icon><IconRefresh /></template>
            刷新
          </AButton>
        </div>
      </div>

      <!-- 首屏关键数字 -->
      <div class="grid grid-cols-4 gap-5 mb-5 max-lg:grid-cols-2 max-md:grid-cols-2 max-md:gap-3">
        <ACard
          v-for="(m, idx) in metrics"
          :key="m.key"
          class="stat-card animate-cardAppear"
          :bordered="false"
          :style="{ animationDelay: `${idx * 0.06}s` }"
        >
          <div class="stat-card__inner">
            <div>
              <div class="stat-card__label">{{ m.label }}</div>
              <div class="stat-card__value">{{ m.value }}</div>
              <div class="stat-card__meta">
                <span
                  v-if="m.growth"
                  class="stat-card__growth"
                  :class="m.growth.positive ? 'is-up' : 'is-down'"
                >
                  {{ m.growth.positive ? '&#9650;' : '&#9660;' }} {{ m.growth.value }}%
                </span>
                <span class="stat-card__sub">{{ m.sub }}</span>
              </div>
            </div>
            <div class="stat-card__icon" :style="{ background: m.gradient }">
              <component :is="m.icon" :size="24" style="color: #fff" />
            </div>
          </div>
        </ACard>
      </div>

      <!-- 紧急横幅：只有 P0 才弹，其余静默计数 -->
      <div v-if="urgentCount > 0" class="wb-banner" @click="onlyUrgent = true">
        <IconExclamationCircle :size="18" />
        <span class="wb-banner__text">
          <b>{{ urgentCount }} 项紧急事项</b>
          正在影响服务可用性或资金安全，建议立即处理
        </span>
        <span class="wb-banner__link">只看这些 <IconRight /></span>
      </div>

      <div class="wb-body">
        <!-- 左：统一待办流（严重度优先，不按菜单打散） -->
        <ACard :bordered="false" class="wb-flow">
          <template #title>
            <span>待办事项</span>
            <span class="wb-flow__count">{{ filtered.length }}</span>
          </template>
          <template #extra>
            <ATag v-if="activeDomain !== 'all'" closable size="small" @close="activeDomain = 'all'">
              {{ DOMAINS[activeDomain as DomainKey].label }}
            </ATag>
          </template>

          <TransitionGroup name="wb-item" tag="div" class="wb-list">
            <div
              v-for="item in filtered"
              :key="item.key"
              class="wb-item"
              :style="{
                borderLeftColor: SEVERITY[item.severity as Severity].color,
                background: SEVERITY[item.severity as Severity].bg,
              }"
            >
              <div class="wb-item__head">
                <ATag
                  size="small"
                  :style="{
                    background: SEVERITY[item.severity as Severity].color + '1a',
                    color: SEVERITY[item.severity as Severity].color,
                    border: 'none',
                  }"
                >
                  {{ SEVERITY[item.severity as Severity].label }}
                </ATag>
                <span class="wb-item__domain" :style="{ color: DOMAINS[item.domain as DomainKey].color }">
                  <component :is="DOMAINS[item.domain as DomainKey].icon" :size="12" />
                  {{ DOMAINS[item.domain as DomainKey].label }}
                </span>
                <span class="wb-item__title">{{ item.title }}</span>
                <span v-if="item.occurred_at" class="wb-item__time">{{ timeAgo(item.occurred_at) }}</span>
              </div>
              <div class="wb-item__desc">{{ item.desc }}</div>
              <div class="wb-item__actions">
                <AButton v-if="item.action_route" size="mini" type="primary" @click="goAction(item)">
                  {{ item.action_text || '去处理' }}
                </AButton>
              </div>
            </div>
          </TransitionGroup>

          <div v-if="filtered.length === 0" class="wb-empty">
            <IconCheckCircleFill :size="44" style="color: #10b981" />
            <div class="wb-empty__title">全部处理完了</div>
            <div class="wb-empty__desc">工作台清空是正常状态，不代表没数据。</div>
          </div>
        </ACard>

        <!-- 右：分域计数 + 可用性速览 -->
        <div class="wb-side">
          <ACard :bordered="false" title="按域分布">
            <div class="wb-domains">
              <div
                v-for="d in domainStats"
                :key="d.key"
                class="wb-domain"
                :class="{ 'is-active': activeDomain === d.key }"
                @click="pickDomain(d.key)"
              >
                <span class="wb-domain__icon" :style="{ background: d.color + '1a', color: d.color }">
                  <component :is="d.icon" :size="14" />
                </span>
                <span class="wb-domain__label">{{ d.label }}</span>
                <span v-if="d.urgent > 0" class="wb-domain__urgent">{{ d.urgent }}</span>
                <span class="wb-domain__total">{{ d.total }}</span>
              </div>
            </div>
          </ACard>

          <ACard :bordered="false" title="模型可用渠道">
            <template #extra>
              <ALink @click="router.push({ name: 'AdminChannels' })">全部</ALink>
            </template>
            <div v-if="(summary?.models || []).length" class="wb-avail">
              <div v-for="m in (summary?.models || []).slice(0, 8)" :key="m.model" class="wb-avail__row">
                <span class="wb-avail__name">{{ m.model }}</span>
                <span class="wb-avail__val" :class="availClass(m)">{{ m.available }} / {{ m.total }}</span>
              </div>
            </div>
            <AEmpty v-else description="调度目录尚未初始化" />
          </ACard>

          <ACard :bordered="false" title="正在熔断">
            <div v-if="(summary?.breakers || []).length" class="wb-breaker">
              <div v-for="(b, i) in summary.breakers" :key="i" class="wb-breaker__row">
                <div class="wb-breaker__main">
                  <span class="wb-breaker__ch">{{ b.channel_name }}</span>
                  <span class="wb-breaker__model">{{ b.model }}</span>
                </div>
                <div class="wb-breaker__meta">
                  <ATag size="small" :color="b.channel_level ? 'red' : 'orange'">
                    {{ b.channel_level ? '渠道级' : '模型级' }}
                  </ATag>
                  <ATag v-if="b.half_open" size="small" color="arcoblue">探测中</ATag>
                </div>
              </div>
            </div>
            <AEmpty v-else description="无渠道处于熔断" />
          </ACard>
        </div>
      </div>

    </div>
  </ASpin>
</template>

<style scoped>
.grid {
  display: grid;
}

/* ===== 页头 ===== */
.wb-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  margin-bottom: 20px;
  gap: 16px;
  flex-wrap: wrap;
}

.wb-header__title {
  font-size: 22px;
  font-weight: 700;
  color: var(--color-text-1);
  letter-spacing: -0.02em;
}

.wb-header__desc {
  margin-top: 4px;
  font-size: 13px;
  color: var(--color-text-3);
}

.wb-header__ops {
  display: flex;
  align-items: center;
  gap: 8px;
}

.wb-header__time {
  font-size: 12px;
  color: var(--color-text-3);
  margin-right: 4px;
}

.wb-header__switch-label {
  font-size: 13px;
  color: var(--color-text-2);
  margin-right: 4px;
}

/* ===== 指标卡（复用仪表盘视觉语言）===== */
.stat-card {
  padding: 4px 0;
}

.stat-card__inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.stat-card__label {
  font-size: 13px;
  color: var(--color-text-3);
  margin-bottom: 10px;
  font-weight: 500;
}

.stat-card__value {
  font-size: 26px;
  font-weight: 700;
  color: var(--color-text-1);
  letter-spacing: -0.02em;
  line-height: 1;
}

.stat-card__meta {
  margin-top: 8px;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.stat-card__growth.is-up {
  color: #10b981;
}

.stat-card__growth.is-down {
  color: #ef4444;
}

.stat-card__sub {
  color: var(--color-text-3);
}

.stat-card__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 14px;
  flex-shrink: 0;
}

/* ===== 紧急横幅 ===== */
.wb-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  margin-bottom: 20px;
  border-radius: 8px;
  background: rgba(245, 63, 63, 0.08);
  border: 1px solid rgba(245, 63, 63, 0.2);
  color: #f53f3f;
  cursor: pointer;
  transition: background 0.2s;
}

.wb-banner:hover {
  background: rgba(245, 63, 63, 0.12);
}

.wb-banner__text {
  flex: 1;
  font-size: 13px;
  color: var(--color-text-1);
}

.wb-banner__text b {
  color: #f53f3f;
}

.wb-banner__link {
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
}

/* ===== 主体两列 ===== */
.wb-body {
  display: grid;
  grid-template-columns: minmax(0, 2.2fr) minmax(0, 1fr);
  gap: 20px;
  align-items: start;
}

.wb-side {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* ===== 待办流 ===== */
.wb-flow__count {
  display: inline-block;
  margin-left: 8px;
  padding: 0 7px;
  height: 18px;
  line-height: 18px;
  border-radius: 9px;
  background: var(--color-fill-3);
  color: var(--color-text-2);
  font-size: 12px;
  font-weight: 600;
}

.wb-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  position: relative;
}

.wb-item {
  padding: 12px 14px;
  border-radius: 6px;
  border: 1px solid var(--color-border-2);
  border-left-width: 3px;
  transition: box-shadow 0.2s;
}

.wb-item:hover {
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.wb-item__head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.wb-item__domain {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: 12px;
  font-weight: 500;
}

.wb-item__title {
  flex: 1;
  min-width: 200px;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-1);
}

.wb-item__time {
  font-size: 12px;
  color: var(--color-text-3);
  white-space: nowrap;
}

.wb-item__desc {
  margin-top: 6px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-text-3);
}

.wb-item__actions {
  margin-top: 10px;
  display: flex;
  align-items: center;
  gap: 8px;
}

/* ===== 空态：清空是正常状态 ===== */
.wb-empty {
  padding: 48px 0;
  text-align: center;
}

.wb-empty__title {
  margin-top: 12px;
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-1);
}

.wb-empty__desc {
  margin-top: 4px;
  font-size: 13px;
  color: var(--color-text-3);
}

/* ===== 右栏：按域分布 ===== */
.wb-domains {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.wb-domain {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.2s;
}

.wb-domain:hover {
  background: var(--color-fill-1);
}

.wb-domain.is-active {
  background: var(--color-fill-2);
}

.wb-domain__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 6px;
  flex-shrink: 0;
}

.wb-domain__label {
  flex: 1;
  font-size: 13px;
  color: var(--color-text-1);
}

.wb-domain__urgent {
  min-width: 18px;
  height: 18px;
  line-height: 18px;
  text-align: center;
  border-radius: 9px;
  background: #f53f3f;
  color: #fff;
  font-size: 11px;
  font-weight: 600;
  padding: 0 5px;
}

.wb-domain__total {
  min-width: 22px;
  text-align: right;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-2);
}

/* ===== 右栏：模型可用渠道 ===== */
.wb-avail {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.wb-avail__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13px;
  gap: 8px;
}

.wb-avail__name {
  color: var(--color-text-2);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.wb-avail__val {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}

.wb-avail__val.is-ok {
  color: #10b981;
}

.wb-avail__val.is-warn {
  color: #ff7d00;
}

.wb-avail__val.is-dead {
  color: #f53f3f;
}

/* ===== 右栏：熔断 ===== */
.wb-breaker {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.wb-breaker__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.wb-breaker__main {
  min-width: 0;
}

.wb-breaker__ch {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-1);
}

.wb-breaker__model {
  display: block;
  font-size: 11px;
  color: var(--color-text-3);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.wb-breaker__meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

/* ===== 动画 ===== */
@keyframes cardAppear {
  from {
    opacity: 0;
    transform: translateY(12px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.animate-cardAppear {
  animation: cardAppear 0.4s ease both;
}

.wb-item-leave-active {
  transition: all 0.3s ease;
  position: absolute;
  width: 100%;
}

.wb-item-leave-to {
  opacity: 0;
  transform: translateX(24px);
}

.wb-item-move {
  transition: transform 0.3s ease;
}

/* ===== 响应式 ===== */
@media (max-width: 1200px) {
  .wb-body {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 767px) {
  .stat-card__value {
    font-size: 22px;
  }

  .stat-card__icon {
    width: 40px;
    height: 40px;
    border-radius: 10px;
  }

  .wb-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .wb-item__title {
    min-width: 100%;
  }
}
</style>
