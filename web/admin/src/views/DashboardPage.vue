<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import {
  IconThunderbolt,
  IconFire,
  IconCalendar,
  IconUserGroup,
  IconApps,
  IconBarChart,
  IconCheckCircle,
} from '@arco-design/web-vue/es/icon'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import * as echarts from 'echarts'

const router = useRouter()

// === Data State ===
const loading = ref(false)
const trendDays = ref(30)

const dashboard = ref<any>(null)
const trends = ref<any[]>([])
const topTenants = ref<any[]>([])
const modelDist = ref<any[]>([])
const channelHealth = ref<any[]>([])
const recentAlerts = ref<any[]>([])

// === Charts ===
let trendChart: echarts.ECharts | null = null
let tenantTrendChart: echarts.ECharts | null = null
let modelChart: echarts.ECharts | null = null

// === Growth Calculation ===
function calcGrowth(current: number, previous: number): { value: number; positive: boolean } | undefined {
  if (previous === 0 && current === 0) return undefined
  if (previous === 0) return { value: current > 0 ? 100 : 0, positive: current > 0 }
  const growth = ((current - previous) / previous) * 100
  return { value: Math.round(growth * 10) / 10, positive: growth >= 0 }
}

function formatMoney(val: any) {
  const n = Number(val)
  if (!n && n !== 0) return '$0.00'
  return '$' + n.toFixed(2)
}

function formatNumber(val: any) {
  const n = Number(val)
  if (!n && n !== 0) return '0'
  return n.toLocaleString()
}

// === Metric Cards ===
interface MetricCard {
  label: string
  value: string
  icon: any
  gradient: string
  growth?: { value: number; positive: boolean }
}

const metricCards = computed<MetricCard[]>(() => {
  const t = dashboard.value?.today
  const y = dashboard.value?.yesterday
  const m = dashboard.value?.month
  const d = dashboard.value
  if (!t) return []

  const todayTokens = (t.input_tokens || 0) + (t.output_tokens || 0)
  const yesterdayTokens = (y?.input_tokens || 0) + (y?.output_tokens || 0)

  return [
    {
      label: '今日请求',
      value: formatNumber(t.requests),
      icon: IconThunderbolt,
      gradient: 'linear-gradient(135deg, #f59e0b, #fbbf24)',
      growth: calcGrowth(t.requests || 0, y?.requests || 0),
    },
    {
      label: '今日费用',
      value: formatMoney(t.total_cost),
      icon: IconFire,
      gradient: 'linear-gradient(135deg, #ef4444, #f87171)',
      growth: calcGrowth(t.total_cost || 0, y?.total_cost || 0),
    },
    {
      label: '本月收入',
      value: formatMoney(m?.revenue),
      icon: IconBarChart,
      gradient: 'linear-gradient(135deg, #0d9488, #14b8a6)',
    },
    {
      label: '本月请求',
      value: formatNumber(m?.requests),
      icon: IconCalendar,
      gradient: 'linear-gradient(135deg, #6366f1, #818cf8)',
    },
    {
      label: '活跃租户',
      value: formatNumber(t.active_tenants),
      icon: IconUserGroup,
      gradient: 'linear-gradient(135deg, #10b981, #34d399)',
      growth: calcGrowth(t.active_tenants || 0, y?.active_tenants || 0),
    },
    {
      label: '活跃渠道',
      value: formatNumber(d?.active_channels),
      icon: IconApps,
      gradient: 'linear-gradient(135deg, #06b6d4, #22d3ee)',
    },
    {
      label: '今日 Token',
      value: formatNumber(todayTokens),
      icon: IconBarChart,
      gradient: 'linear-gradient(135deg, #8b5cf6, #a78bfa)',
      growth: calcGrowth(todayTokens, yesterdayTokens),
    },
    {
      label: '成功率',
      value: (t.success_rate || 0).toFixed(1) + '%',
      icon: IconCheckCircle,
      gradient: 'linear-gradient(135deg, #059669, #10b981)',
      growth: calcGrowth(t.success_rate || 0, y?.success_rate || 0),
    },
  ]
})

// === Top Tenants Table ===
const tenantColumns: TableColumnData[] = [
  { title: '#', dataIndex: 'rank', width: 50, render: ({ rowIndex }) => rowIndex + 1 },
  { title: '租户', dataIndex: 'tenant_name', ellipsis: true },
  { title: '请求数', dataIndex: 'requests', width: 100, render: ({ record }) => formatNumber(record.requests) },
  { title: '活跃成员', dataIndex: 'active_members', width: 90 },
  {
    title: '费用',
    dataIndex: 'total_cost',
    width: 120,
    render: ({ record }) => formatMoney(record.total_cost),
  },
]

// === Channel Health Helpers ===
function healthColor(score: number | null | undefined) {
  if (score === null || score === undefined) return 'gray'
  if (score >= 80) return 'green'
  if (score >= 50) return 'orange'
  return 'red'
}

// === Alert Helpers ===
const alertLevelColor: Record<string, string> = { info: 'arcoblue', warning: 'orange', critical: 'red' }
const alertLevelLabel: Record<string, string> = { info: '信息', warning: '警告', critical: '严重' }
const alertStatusLabel: Record<string, string> = { firing: '触发中', acknowledged: '已确认', resolved: '已恢复' }

function formatTimeAgo(dateStr: string) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - d.getTime()
  const diffMin = Math.floor(diffMs / 60000)
  if (diffMin < 1) return '刚刚'
  if (diffMin < 60) return `${diffMin}分钟前`
  const diffHour = Math.floor(diffMin / 60)
  if (diffHour < 24) return `${diffHour}小时前`
  const diffDay = Math.floor(diffHour / 24)
  if (diffDay < 30) return `${diffDay}天前`
  return dateStr.slice(0, 10)
}

// === Data Fetching ===
async function fetchAll() {
  loading.value = true
  const results = await Promise.allSettled([
    request.get('/admin/dashboard'),
    request.get('/admin/dashboard/trends', { params: { days: trendDays.value } }),
    request.get('/admin/dashboard/top-tenants', { params: { days: 30 } }),
    request.get('/admin/dashboard/model-distribution', { params: { days: 30 } }),
    request.get('/admin/dashboard/channel-health'),
    request.get('/admin/dashboard/recent-alerts'),
  ])
  if (results[0].status === 'fulfilled') dashboard.value = results[0].value.data?.data
  if (results[1].status === 'fulfilled') trends.value = results[1].value.data?.data?.list || []
  if (results[2].status === 'fulfilled') topTenants.value = results[2].value.data?.data?.list || []
  if (results[3].status === 'fulfilled') modelDist.value = results[3].value.data?.data?.list || []
  if (results[4].status === 'fulfilled') channelHealth.value = results[4].value.data?.data?.list || []
  if (results[5].status === 'fulfilled') recentAlerts.value = results[5].value.data?.data?.list || []
  loading.value = false

  await nextTick()
  renderAllCharts()
}

async function fetchTrendData() {
  const results = await Promise.allSettled([
    request.get('/admin/dashboard/trends', { params: { days: trendDays.value } }),
    request.get('/admin/dashboard/top-tenants', { params: { days: trendDays.value } }),
    request.get('/admin/dashboard/model-distribution', { params: { days: trendDays.value } }),
  ])
  if (results[0].status === 'fulfilled') trends.value = results[0].value.data?.data?.list || []
  if (results[1].status === 'fulfilled') topTenants.value = results[1].value.data?.data?.list || []
  if (results[2].status === 'fulfilled') modelDist.value = results[2].value.data?.data?.list || []

  await nextTick()
  renderAllCharts()
}

function onTrendDaysChange(val: any) {
  trendDays.value = val
  fetchTrendData()
}

// === Chart Rendering ===
function renderAllCharts() {
  renderTrendChart()
  renderTenantTrendChart()
  renderModelChart()
}

function renderTrendChart() {
  const el = document.getElementById('trend-chart')
  if (!el) return
  if (!trendChart) trendChart = echarts.init(el)
  if (trends.value.length === 0) {
    trendChart.clear()
    return
  }
  const dates = trends.value.map((r: any) => r.date?.slice(5) || '')
  const revenues = trends.value.map((r: any) => Number(r.revenue) || 0)
  const requests = trends.value.map((r: any) => Number(r.requests) || 0)

  trendChart.setOption({
    tooltip: { trigger: 'axis', axisPointer: { type: 'cross' } },
    legend: { data: ['收入 ($)', '请求数'], bottom: 0 },
    grid: { left: 60, right: 60, top: 15, bottom: 36 },
    xAxis: { type: 'category', data: dates, axisLabel: { fontSize: 11 } },
    yAxis: [
      { type: 'value', name: '收入 ($)', axisLabel: { formatter: (v: number) => '$' + v } },
      { type: 'value', name: '请求数', splitLine: { show: false } },
    ],
    series: [
      {
        name: '收入 ($)',
        type: 'bar',
        data: revenues,
        barWidth: 12,
        itemStyle: { color: '#0d9488', borderRadius: [3, 3, 0, 0] },
      },
      {
        name: '请求数',
        type: 'line',
        yAxisIndex: 1,
        data: requests,
        smooth: true,
        lineStyle: { width: 2 },
        itemStyle: { color: '#6366f1' },
        symbol: 'none',
      },
    ],
  }, true)
}

function renderTenantTrendChart() {
  const el = document.getElementById('tenant-trend-chart')
  if (!el) return
  if (!tenantTrendChart) tenantTrendChart = echarts.init(el)
  if (trends.value.length === 0) {
    tenantTrendChart.clear()
    return
  }
  const dates = trends.value.map((r: any) => r.date?.slice(5) || '')
  const activeTenants = trends.value.map((r: any) => Number(r.active_tenants) || 0)

  tenantTrendChart.setOption({
    tooltip: { trigger: 'axis' },
    grid: { left: 50, right: 20, top: 15, bottom: 30 },
    xAxis: { type: 'category', data: dates, axisLabel: { fontSize: 11 } },
    yAxis: { type: 'value', name: '租户数', minInterval: 1 },
    series: [{
      type: 'line',
      data: activeTenants,
      smooth: true,
      itemStyle: { color: '#f59e0b' },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: 'rgba(245,158,11,0.3)' },
          { offset: 1, color: 'rgba(245,158,11,0.02)' },
        ]),
      },
      symbol: 'none',
    }],
  }, true)
}

function renderModelChart() {
  const el = document.getElementById('model-chart')
  if (!el) return
  if (!modelChart) modelChart = echarts.init(el)
  if (modelDist.value.length === 0) {
    modelChart.clear()
    return
  }
  const top10 = modelDist.value.slice(0, 10)
  const names = top10.map((r: any) => r.model_name || '').reverse()
  const costs = top10.map((r: any) => Number(r.total_cost) || 0).reverse()

  modelChart.setOption({
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 130, right: 50, top: 10, bottom: 36 },
    xAxis: { type: 'value', name: '费用 ($)', nameLocation: 'middle', nameGap: 22 },
    yAxis: {
      type: 'category',
      data: names,
      axisLabel: { width: 110, overflow: 'truncate', fontSize: 11 },
    },
    series: [{
      type: 'bar',
      data: costs,
      barWidth: 16,
      itemStyle: { color: '#6366f1', borderRadius: [0, 3, 3, 0] },
    }],
  }, true)
}

// === Resize ===
const resizeObserver = new ResizeObserver(() => {
  trendChart?.resize()
  tenantTrendChart?.resize()
  modelChart?.resize()
})

let resizeTimer: ReturnType<typeof setTimeout> | null = null

function onWindowResize() {
  if (resizeTimer) clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    trendChart?.resize()
    tenantTrendChart?.resize()
    modelChart?.resize()
  }, 200)
}

// === Lifecycle ===
onMounted(() => {
  fetchAll()
  window.addEventListener('resize', onWindowResize)
  nextTick(() => {
    const trendEl = document.getElementById('trend-chart')
    const tenantEl = document.getElementById('tenant-trend-chart')
    const modelEl = document.getElementById('model-chart')
    if (trendEl) resizeObserver.observe(trendEl)
    if (tenantEl) resizeObserver.observe(tenantEl)
    if (modelEl) resizeObserver.observe(modelEl)
  })
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onWindowResize)
  if (resizeTimer) clearTimeout(resizeTimer)
  resizeObserver.disconnect()
  if (trendChart) { trendChart.dispose(); trendChart = null }
  if (tenantTrendChart) { tenantTrendChart.dispose(); tenantTrendChart = null }
  if (modelChart) { modelChart.dispose(); modelChart = null }
})
</script>

<template>
  <ASpin :loading="loading" style="width: 100%">
    <div style="width: 100%">
      <PageHeader title="仪表盘" description="平台运营概览" />

      <!-- Row 1: Key Metric Cards -->
      <div class="grid grid-cols-4 gap-5 mb-6 max-lg:grid-cols-2 max-md:grid-cols-1 max-md:gap-3">
        <ACard
          v-for="(stat, idx) in metricCards"
          :key="idx"
          class="stat-card animate-cardAppear"
          :bordered="false"
          :style="{ animationDelay: `${idx * 0.06}s` }"
        >
          <div class="stat-card__inner">
            <div>
              <div class="stat-card__label">{{ stat.label }}</div>
              <div class="stat-card__value">{{ stat.value }}</div>
              <div v-if="stat.growth" class="stat-card__growth" :class="stat.growth.positive ? 'is-up' : 'is-down'">
                <span class="growth-arrow">{{ stat.growth.positive ? '&#9650;' : '&#9660;' }}</span>
                <span>{{ Math.abs(stat.growth.value) }}%</span>
                <span class="growth-label">较昨日</span>
              </div>
            </div>
            <div class="stat-card__icon" :style="{ background: stat.gradient }">
              <component :is="stat.icon" :size="24" style="color: #fff" />
            </div>
          </div>
        </ACard>
      </div>

      <!-- Row 2: Trend Charts -->
      <div class="grid grid-cols-2 gap-5 mb-6 max-md:grid-cols-1 max-md:gap-3">
        <ACard :bordered="false">
          <template #title>
            <span>收入与请求趋势</span>
          </template>
          <template #extra>
            <ARadioGroup type="button" size="small" :model-value="trendDays" @change="onTrendDaysChange">
              <ARadio :value="7">7天</ARadio>
              <ARadio :value="30">30天</ARadio>
              <ARadio :value="90">90天</ARadio>
            </ARadioGroup>
          </template>
          <div id="trend-chart" style="width: 100%; height: 320px;" />
        </ACard>
        <ACard :bordered="false" title="活跃租户趋势">
          <div id="tenant-trend-chart" style="width: 100%; height: 320px;" />
        </ACard>
      </div>

      <!-- Row 3: Model Distribution + Top Tenants -->
      <div class="grid grid-cols-2 gap-5 mb-6 max-md:grid-cols-1 max-md:gap-3">
        <ACard :bordered="false" title="模型用量分布（按费用）">
          <div id="model-chart" style="width: 100%; height: 350px;" />
        </ACard>
        <ACard :bordered="false" title="TOP 10 租户（按费用）">
          <template #extra>
            <ALink @click="router.push({ name: 'AdminTenants' })">查看全部</ALink>
          </template>
          <ATable
            :columns="tenantColumns"
            :data="topTenants"
            :pagination="false"
            :bordered="false"
            size="small"
            row-key="tenant_id"
            :scroll="{ x: 400 }"
          />
        </ACard>
      </div>

      <!-- Row 4: Channel Health + Recent Alerts -->
      <div class="grid grid-cols-2 gap-5 mb-6 max-md:grid-cols-1 max-md:gap-3">
        <ACard :bordered="false" title="渠道健康概览">
          <template #extra>
            <ALink @click="router.push({ name: 'AdminChannels' })">查看全部</ALink>
          </template>
          <div v-if="channelHealth.length > 0" class="info-list">
            <div v-for="ch in channelHealth" :key="ch.channel_id" class="info-item">
              <span class="info-name">{{ ch.channel_name }}</span>
              <ATag :color="healthColor(ch.health_score)" size="small">
                {{ ch.health_score != null ? ch.health_score.toFixed(0) + '分' : 'N/A' }}
              </ATag>
              <span class="info-meta">
                成功率 {{ ch.success_rate != null ? ch.success_rate.toFixed(1) + '%' : 'N/A' }}
                / 延迟 {{ ch.latency_ms != null ? ch.latency_ms + 'ms' : 'N/A' }}
              </span>
            </div>
          </div>
          <AEmpty v-else description="所有渠道运行正常" />
        </ACard>
        <ACard :bordered="false" title="最近告警">
          <template #extra>
            <ALink @click="router.push({ name: 'AdminAlertEvents' })">查看全部</ALink>
          </template>
          <div v-if="recentAlerts.length > 0" class="info-list">
            <div v-for="alert in recentAlerts" :key="alert.id" class="info-item">
              <ATag :color="alertLevelColor[alert.level] || 'blue'" size="small">
                {{ alertLevelLabel[alert.level] || alert.level }}
              </ATag>
              <ATag v-if="alert.status === 'firing'" color="red" size="small">
                {{ alertStatusLabel[alert.status] }}
              </ATag>
              <span class="info-msg">{{ alert.trigger_message }}</span>
              <span class="info-time">{{ formatTimeAgo(alert.created_at) }}</span>
            </div>
          </div>
          <AEmpty v-else description="暂无告警" />
        </ACard>
      </div>
    </div>
  </ASpin>
</template>

<style scoped>
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

.stat-card__growth {
  margin-top: 8px;
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 3px;
}

.stat-card__growth .growth-arrow {
  font-size: 10px;
}

.stat-card__growth .growth-label {
  color: var(--color-text-3);
  margin-left: 2px;
}

.stat-card__growth.is-up {
  color: #10b981;
}

.stat-card__growth.is-down {
  color: #ef4444;
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

.info-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.info-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}

.info-name {
  flex-shrink: 0;
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
  color: var(--color-text-1);
}

.info-meta {
  color: var(--color-text-3);
  font-size: 12px;
  white-space: nowrap;
}

.info-msg {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-text-2);
}

.info-time {
  flex-shrink: 0;
  color: var(--color-text-3);
  font-size: 12px;
  white-space: nowrap;
}

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

.grid {
  display: grid;
}

/* Mobile responsive */
@media (max-width: 767px) {
  .stat-card__value {
    font-size: 22px;
  }

  .stat-card__icon {
    width: 40px;
    height: 40px;
    border-radius: 10px;
  }

  .info-item {
    flex-wrap: wrap;
  }

  .info-name {
    max-width: 100px;
  }

  .info-meta {
    white-space: normal;
    word-break: break-all;
  }

  #trend-chart,
  #tenant-trend-chart {
    height: 240px !important;
  }

  #model-chart {
    height: 260px !important;
  }
}
</style>
