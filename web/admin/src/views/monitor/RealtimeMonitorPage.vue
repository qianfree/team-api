<script setup lang="ts">
import { ref, onMounted, onUnmounted, shallowRef, computed } from 'vue'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, BarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

use([CanvasRenderer, LineChart, BarChart, GridComponent, TooltipComponent, LegendComponent])

const data = ref<any>({})
const loading = ref(true)
const lastUpdated = ref<Date | null>(null)
const now = ref(Date.now())
let elapsedTimer: ReturnType<typeof setInterval> | null = null

const concurrencyOption = shallowRef({})
const bandwidthOption = shallowRef({})
const gcPauseOption = shallowRef({})

async function fetchRealtime() {
  try {
    const res = await request.get('/admin/monitor/realtime')
    data.value = res.data?.data?.data || {}
    lastUpdated.value = new Date()
    updateCharts()
  } catch {
    // error auto-shown by interceptor
  } finally {
    loading.value = false
  }
}

function formatTime(ts: string) {
  const d = new Date(ts)
  return `${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`
}

function updateCharts() {
  const history = data.value.history || []
  const bwHistory = data.value.bandwidth_history || []
  const go = data.value.go_runtime || {}

  concurrencyOption.value = {
    tooltip: { trigger: 'axis' },
    legend: { data: ['总数', '流式', '非流式'], top: 0, right: 0, textStyle: { fontSize: 12 } },
    grid: { left: 40, right: 16, top: 36, bottom: 24 },
    xAxis: {
      type: 'category',
      data: history.map((h: any) => formatTime(h.timestamp)),
      axisLabel: { fontSize: 11 },
      boundaryGap: false,
    },
    yAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { type: 'dashed' } } },
    series: [
      { name: '总数', type: 'line', data: history.map((h: any) => h.total_active), smooth: true, lineStyle: { width: 2 }, itemStyle: { color: '#165DFF' }, areaStyle: { color: 'rgba(22,93,255,0.06)' } },
      { name: '流式', type: 'line', data: history.map((h: any) => h.streaming_active), smooth: true, lineStyle: { width: 1.5 }, itemStyle: { color: '#00B42A' } },
      { name: '非流式', type: 'line', data: history.map((h: any) => h.non_streaming_active), smooth: true, lineStyle: { width: 1.5 }, itemStyle: { color: '#FF7D00' } },
    ],
  }

  bandwidthOption.value = {
    tooltip: {
      trigger: 'axis',
      valueFormatter: (v: number) => formatBytes(v) + '/s',
    },
    legend: { data: ['入站', '出站'], top: 0, right: 0, textStyle: { fontSize: 12 } },
    grid: { left: 64, right: 16, top: 36, bottom: 24 },
    xAxis: {
      type: 'category',
      data: bwHistory.map((h: any) => formatTime(h.timestamp)),
      axisLabel: { fontSize: 11 },
      boundaryGap: false,
    },
    yAxis: {
      type: 'value',
      axisLabel: { formatter: (v: number) => formatBytes(v) + '/s', fontSize: 11 },
      splitLine: { lineStyle: { type: 'dashed' } },
    },
    series: [
      { name: '入站', type: 'line', areaStyle: { color: 'rgba(22,93,255,0.08)' }, data: bwHistory.map((h: any) => h.bytes_in_per_sec), smooth: true, lineStyle: { width: 2 }, itemStyle: { color: '#165DFF' } },
      { name: '出站', type: 'line', areaStyle: { color: 'rgba(0,180,42,0.08)' }, data: bwHistory.map((h: any) => h.bytes_out_per_sec), smooth: true, lineStyle: { width: 2 }, itemStyle: { color: '#00B42A' } },
    ],
  }

  // GC pause bar chart
  const gcPauses = (go.gc?.recent_pause_ms || []).slice().reverse()
  gcPauseOption.value = {
    tooltip: { trigger: 'axis', valueFormatter: (v: number) => v?.toFixed(2) + ' ms' },
    grid: { left: 50, right: 16, top: 16, bottom: 24 },
    xAxis: { type: 'category', data: gcPauses.map((_: any, i: number) => `#${(go.gc?.num_gc || 0) - gcPauses.length + i + 1}`), axisLabel: { fontSize: 11 } },
    yAxis: { type: 'value', axisLabel: { formatter: (v: number) => v.toFixed(1) + 'ms', fontSize: 11 }, splitLine: { lineStyle: { type: 'dashed' } } },
    series: [{ type: 'bar', data: gcPauses, itemStyle: { color: '#722ED1', borderRadius: [3, 3, 0, 0] }, barMaxWidth: 24 }],
  }
}

function formatBytes(v: number): string {
  if (v < 1024) return v.toFixed(0) + ' B'
  if (v < 1024 * 1024) return (v / 1024).toFixed(1) + ' KB'
  return (v / 1024 / 1024).toFixed(1) + ' MB'
}

function formatDuration(startTime: string): string {
  const ms = now.value - new Date(startTime).getTime()
  if (ms < 1000) return ms + 'ms'
  if (ms < 60000) return (ms / 1000).toFixed(1) + 's'
  const m = Math.floor(ms / 60000)
  const s = Math.floor((ms % 60000) / 1000)
  if (m < 60) return `${m}m ${s}s`
  return `${Math.floor(m / 60)}h ${m % 60}m ${s}s`
}

function fmtMB(v: number | undefined, decimals = 1): string {
  if (v === undefined || v === null) return '0'
  return v.toFixed(decimals)
}

function fmtPercent(fraction: number | undefined): string {
  if (fraction === undefined || fraction === null) return '0%'
  return (fraction * 100).toFixed(2) + '%'
}

function fmtLastUpdated(): string {
  if (!lastUpdated.value) return '-'
  const d = lastUpdated.value
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`
}

const conc = computed(() => data.value.concurrency || {})
const bw = computed(() => data.value.bandwidth || {})
const rt = computed(() => data.value.runtime || {})
const goRt = computed(() => data.value.go_runtime || {})
const mem = computed(() => goRt.value.memory || {})
const gc = computed(() => goRt.value.gc || {})
const build = computed(() => goRt.value.build || {})

const activeRequests = computed(() => {
  const reqs: any[] = data.value.active_requests || []
  return reqs.sort((a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())
})

const byModel = computed(() => {
  const m: Record<string, number> = data.value.by_model || {}
  const entries = Object.entries(m).sort(([, a], [, b]) => (b as number) - (a as number)).slice(0, 10)
  const max = entries.length > 0 ? (entries[0][1] as number) : 1
  return entries.map(([k, v]) => ({ model: k, count: v as number, pct: ((v as number) / max) * 100 }))
})

const byChannel = computed(() => {
  const m: Record<string, number> = data.value.by_channel || {}
  const entries = Object.entries(m).sort(([, a], [, b]) => (b as number) - (a as number)).slice(0, 10)
  const max = entries.length > 0 ? (entries[0][1] as number) : 1
  return entries.map(([k, v]) => ({ channel: k, count: v as number, pct: ((v as number) / max) * 100 }))
})

// Heap usage percentage for progress bar
const heapUsagePercent = computed(() => {
  const inuse = mem.value.heap_inuse_mb
  const sys = mem.value.heap_sys_mb
  if (!sys) return 0
  return Math.min(100, (inuse / sys) * 100)
})

let pollTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  fetchRealtime()
  pollTimer = setInterval(fetchRealtime, 10000)
  elapsedTimer = setInterval(() => { now.value = Date.now() }, 1000)
})

onUnmounted(() => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  if (elapsedTimer) {
    clearInterval(elapsedTimer)
    elapsedTimer = null
  }
})
</script>

<template>
  <div>
    <PageHeader title="实时监控面板" description="并发请求、网络流量与运行时状态实时监控">
      <template #actions>
        <div class="header-status">
          <a-tag color="green" size="small">
            自动刷新 10s
          </a-tag>
          <span v-if="lastUpdated" class="last-updated">最后更新 {{ fmtLastUpdated() }}</span>
        </div>
      </template>
    </PageHeader>

    <!-- Loading skeleton -->
    <a-spin :loading="loading" style="width: 100%">
    <div v-if="!loading">

    <!-- Row 1: Concurrency & Bandwidth Metrics -->
    <div class="row row-4 mb-16">
      <a-card :bordered="false" class="metric-card">
        <a-statistic title="当前并发" :value="conc.total_active || 0" :value-style="{ color: (conc.total_active || 0) > 0 ? '#165DFF' : undefined }" />
      </a-card>
      <a-card :bordered="false" class="metric-card">
        <div class="dual-stat">
          <div class="dual-stat-item">
            <div class="dual-stat-dot" style="background: #00B42A" />
            <span class="dual-stat-label">流式请求</span>
            <span class="dual-stat-value" style="color: #00B42A">{{ conc.streaming_active || 0 }}</span>
          </div>
          <div class="dual-stat-divider" />
          <div class="dual-stat-item">
            <div class="dual-stat-dot" style="background: #FF7D00" />
            <span class="dual-stat-label">非流式请求</span>
            <span class="dual-stat-value" style="color: #FF7D00">{{ conc.non_streaming_active || 0 }}</span>
          </div>
        </div>
      </a-card>
      <a-card :bordered="false" class="metric-card">
        <div class="dual-stat">
          <div class="dual-stat-item">
            <div class="dual-stat-dot" style="background: #165DFF" />
            <span class="dual-stat-label">入站带宽</span>
            <span class="dual-stat-value">{{ formatBytes(bw.bytes_in_per_sec || 0) }}/s</span>
          </div>
          <div class="dual-stat-divider" />
          <div class="dual-stat-item">
            <div class="dual-stat-dot" style="background: #00B42A" />
            <span class="dual-stat-label">出站带宽</span>
            <span class="dual-stat-value">{{ formatBytes(bw.bytes_out_per_sec || 0) }}/s</span>
          </div>
        </div>
      </a-card>
      <a-card :bordered="false" class="metric-card">
        <a-statistic title="Goroutine" :value="rt.goroutine_count || 0" />
        <div class="metric-sub">Heap: {{ (rt.heap_alloc_mb || 0).toFixed(0) }} MB</div>
      </a-card>
    </div>

    <!-- Row 2: Go Runtime Info -->
    <div class="row row-3 mb-16">
      <!-- Basic Info -->
      <a-card :bordered="false" title="运行时信息">
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">Go 版本</span>
            <span class="info-value mono">{{ goRt.go_version || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">CPU 核数</span>
            <span class="info-value">{{ goRt.num_cpu || 0 }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">Goroutine</span>
            <span class="info-value">{{ goRt.num_goroutine || 0 }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">CGO 调用</span>
            <span class="info-value">{{ goRt.num_cgo_call || 0 }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">运行时间</span>
            <span class="info-value">{{ goRt.uptime || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">内存限制</span>
            <span class="info-value">{{ fmtMB(gc.memory_limit_mb) }} MB</span>
          </div>
        </div>
        <div class="build-info" v-if="build.commit || build.build_time">
          <span class="info-label">构建</span>
          <span class="info-value mono" v-if="build.commit">{{ build.commit }}</span>
          <span class="info-value" v-if="build.build_time">{{ build.build_time }}</span>
        </div>
      </a-card>

      <!-- Memory Breakdown -->
      <a-card :bordered="false" title="内存分配">
        <div class="mem-summary">
          <div class="mem-stat">
            <span class="mem-stat-value">{{ fmtMB(mem.alloc_mb) }}</span>
            <span class="mem-stat-label">使用中 (Alloc)</span>
          </div>
          <div class="mem-stat">
            <span class="mem-stat-value">{{ fmtMB(mem.sys_mb) }}</span>
            <span class="mem-stat-label">从 OS 获取 (Sys)</span>
          </div>
          <div class="mem-stat">
            <span class="mem-stat-value">{{ fmtMB(mem.total_alloc_mb) }}</span>
            <span class="mem-stat-label">累计分配</span>
          </div>
        </div>
        <div class="mem-bar-section">
          <div class="mem-bar-label">
            <span>堆内存</span>
            <span class="mono">{{ fmtMB(mem.heap_inuse_mb) }} / {{ fmtMB(mem.heap_sys_mb) }} MB</span>
          </div>
          <a-progress :percent="heapUsagePercent" :stroke-width="8" :show-text="false" color="#165DFF" />
        </div>
        <div class="info-grid" style="margin-top: 12px;">
          <div class="info-item">
            <span class="info-label">堆空闲</span>
            <span class="info-value">{{ fmtMB(mem.heap_idle_mb) }} MB</span>
          </div>
          <div class="info-item">
            <span class="info-label">已释放 OS</span>
            <span class="info-value">{{ fmtMB(mem.heap_released_mb) }} MB</span>
          </div>
          <div class="info-item">
            <span class="info-label">堆对象数</span>
            <span class="info-value">{{ mem.heap_objects?.toLocaleString() || 0 }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">栈使用</span>
            <span class="info-value">{{ fmtMB(mem.stack_inuse_mb) }} MB</span>
          </div>
          <div class="info-item">
            <span class="info-label">MSpan</span>
            <span class="info-value">{{ fmtMB(mem.mspan_inuse_mb) }} MB</span>
          </div>
          <div class="info-item">
            <span class="info-label">MCache</span>
            <span class="info-value">{{ fmtMB(mem.mcache_inuse_mb) }} MB</span>
          </div>
          <div class="info-item">
            <span class="info-label">GC 元数据</span>
            <span class="info-value">{{ fmtMB(mem.gc_sys_mb) }} MB</span>
          </div>
          <div class="info-item">
            <span class="info-label">其他</span>
            <span class="info-value">{{ fmtMB(mem.other_sys_mb) }} MB</span>
          </div>
        </div>
      </a-card>

      <!-- GC Stats -->
      <a-card :bordered="false" title="GC 统计">
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">GC 次数</span>
            <span class="info-value">{{ gc.num_gc || 0 }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">强制 GC</span>
            <span class="info-value">{{ gc.num_forced_gc || 0 }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">GC CPU 占比</span>
            <span class="info-value">{{ fmtPercent(gc.gc_cpu_fraction) }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">累计暂停</span>
            <span class="info-value">{{ fmtMB(gc.pause_total_ms) }} ms</span>
          </div>
          <div class="info-item">
            <span class="info-label">上次暂停</span>
            <span class="info-value">{{ fmtMB(gc.pause_last_ms, 2) }} ms</span>
          </div>
          <div class="info-item">
            <span class="info-label">下次 GC 阈值</span>
            <span class="info-value">{{ fmtMB(gc.next_gc_mb) }} MB</span>
          </div>
        </div>
        <div class="gc-chart-title">最近 GC 暂停时长</div>
        <v-chart :option="gcPauseOption" style="height: 120px" autoresize />
      </a-card>
    </div>

    <!-- Row 3: Charts -->
    <div class="row row-2 mb-16">
      <a-card :bordered="false" title="并发趋势">
        <v-chart :option="concurrencyOption" style="height: 260px" autoresize />
      </a-card>
      <a-card :bordered="false" title="带宽趋势">
        <v-chart :option="bandwidthOption" style="height: 260px" autoresize />
      </a-card>
    </div>

    <!-- Row 4: Breakdown with visual bars -->
    <div class="row row-2 mb-16">
      <a-card :bordered="false" title="按模型分布">
        <div v-if="byModel.length === 0" class="empty-hint">暂无活跃请求</div>
        <div v-for="item in byModel" :key="item.model" class="dist-row">
          <div class="dist-label">
            <span class="dist-name" :title="item.model">{{ item.model }}</span>
            <span class="dist-count">{{ item.count }}</span>
          </div>
          <div class="dist-bar-track">
            <div class="dist-bar-fill" :style="{ width: item.pct + '%' }" />
          </div>
        </div>
      </a-card>
      <a-card :bordered="false" title="按渠道分布">
        <div v-if="byChannel.length === 0" class="empty-hint">暂无活跃请求</div>
        <div v-for="item in byChannel" :key="item.channel" class="dist-row">
          <div class="dist-label">
            <span class="dist-name" :title="item.channel">{{ item.channel }}</span>
            <span class="dist-count">{{ item.count }}</span>
          </div>
          <div class="dist-bar-track">
            <div class="dist-bar-fill dist-bar-fill--green" :style="{ width: item.pct + '%' }" />
          </div>
        </div>
      </a-card>
    </div>

    <!-- Row 5: Active Requests -->
    <a-card :bordered="false" title="活跃请求" class="mb-16">
      <template #extra>
        <a-tag>{{ activeRequests.length }} 个请求</a-tag>
      </template>
      <div class="table-scroll-wrapper">
        <a-table :data="activeRequests" :pagination="{ pageSize: 20 }" size="small" :bordered="false" :scroll="{ x: 800 }">
        <template #columns>
          <a-table-column title="Request ID" data-index="request_id" :width="200" ellipsis>
            <template #cell="{ record }">
              <span class="mono">{{ record.request_id }}</span>
            </template>
          </a-table-column>
          <a-table-column title="租户 ID" data-index="tenant_id" :width="90" />
          <a-table-column title="模型" data-index="model_name" :width="180" ellipsis />
          <a-table-column title="渠道" data-index="channel_name" :width="150" ellipsis>
            <template #cell="{ record }">
              {{ record.channel_name || '-' }}
            </template>
          </a-table-column>
          <a-table-column title="类型" :width="80" align="center">
            <template #cell="{ record }">
              <a-tag :color="record.is_stream ? 'green' : 'orangered'" size="small">
                {{ record.is_stream ? '流式' : '非流式' }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column title="已耗时" :width="100" align="right">
            <template #cell="{ record }">
              <span class="mono">{{ formatDuration(record.start_time) }}</span>
            </template>
          </a-table-column>
          <a-table-column title="路径" data-index="path" ellipsis />
        </template>
      </a-table>
      </div>
    </a-card>

    </div>
    </a-spin>
  </div>
</template>

<style scoped>
.row {
  display: grid;
  gap: 16px;
}
.row-2 { grid-template-columns: repeat(2, 1fr); }
.row-3 { grid-template-columns: repeat(3, 1fr); }
.row-4 { grid-template-columns: repeat(4, 1fr); }
.mb-16 { margin-bottom: 16px; }

.table-scroll-wrapper {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}

/* Header status */
.header-status {
  display: flex;
  align-items: center;
  gap: 8px;
}
.last-updated {
  font-size: 12px;
  color: var(--color-text-3);
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
}

/* Metric card base */
.metric-card { min-height: 88px; }
.metric-card :deep(.arco-card-body) {
  display: flex;
  flex-direction: column;
  justify-content: center;
}

/* Dual stat card */
.dual-stat {
  display: flex;
  align-items: stretch;
}
.dual-stat-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 0 8px;
}
.dual-stat-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-bottom: 2px;
}
.dual-stat-label {
  font-size: 12px;
  color: var(--color-text-3);
  line-height: 1;
}
.dual-stat-value {
  font-size: 20px;
  font-weight: 600;
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
  line-height: 1.4;
  margin-top: 4px;
}
.dual-stat-divider {
  width: 1px;
  align-self: stretch;
  background: var(--color-border);
  flex-shrink: 0;
  margin: 4px 0;
}

.metric-sub {
  margin-top: 4px;
  font-size: 12px;
  color: var(--color-text-3);
}
.mono {
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
  font-size: 12px;
}

/* Runtime info styles */
.info-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px 16px;
}
.info-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 0;
}
.info-label {
  font-size: 13px;
  color: var(--color-text-3);
}
.info-value {
  font-size: 13px;
  font-weight: 500;
}

/* Memory summary */
.mem-summary {
  display: flex;
  gap: 24px;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--color-border);
}
.mem-stat {
  text-align: center;
  flex: 1;
}
.mem-stat-value {
  display: block;
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-1);
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
}
.mem-stat-label {
  display: block;
  font-size: 11px;
  color: var(--color-text-3);
  margin-top: 2px;
}

/* Heap bar */
.mem-bar-section {
  margin-top: 8px;
}
.mem-bar-label {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: var(--color-text-2);
  margin-bottom: 4px;
}

/* Build info */
.build-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--color-border);
  font-size: 12px;
}
.build-info .info-label {
  flex-shrink: 0;
}

/* GC chart */
.gc-chart-title {
  font-size: 12px;
  color: var(--color-text-3);
  margin: 12px 0 4px;
}

/* Distribution bars */
.dist-row {
  margin-bottom: 10px;
}
.dist-row:last-child {
  margin-bottom: 0;
}
.dist-label {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}
.dist-name {
  font-size: 13px;
  color: var(--color-text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 75%;
}
.dist-count {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-1);
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
  flex-shrink: 0;
}
.dist-bar-track {
  height: 6px;
  background: var(--color-fill-2);
  border-radius: 3px;
  overflow: hidden;
}
.dist-bar-fill {
  height: 100%;
  border-radius: 3px;
  background: #165DFF;
  transition: width 0.3s ease;
}
.dist-bar-fill--green {
  background: #00B42A;
}

.empty-hint {
  text-align: center;
  padding: 24px 0;
  color: var(--color-text-3);
  font-size: 13px;
}

/* Responsive */
@media (max-width: 1200px) {
  .row-3 { grid-template-columns: 1fr 1fr; }
}
@media (max-width: 768px) {
  .row-4 { grid-template-columns: 1fr 1fr; }
  .row-3 { grid-template-columns: 1fr; }
  .row-2 { grid-template-columns: 1fr; }
}
@media (max-width: 480px) {
  .row-4 { grid-template-columns: 1fr; }
  .info-grid { grid-template-columns: 1fr; }
  .mem-summary { flex-direction: column; gap: 12px; }
  .dual-stat-value { font-size: 16px; }
}
</style>
