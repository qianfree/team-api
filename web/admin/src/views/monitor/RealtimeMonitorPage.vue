<script setup lang="ts">
import { ref, onMounted, onUnmounted, shallowRef, computed } from 'vue'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, BarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { IconThunderbolt, IconSwap, IconClockCircle, IconCode, IconBarChart, IconLayers, IconApps, IconUserGroup, IconFire } from '@arco-design/web-vue/es/icon'

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
  const go = data.value.go_runtime || {}
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

const heapUsagePercent = computed(() => {
  const inuse = mem.value.heap_inuse_mb
  const sys = mem.value.heap_sys_mb
  if (!sys) return 0
  return Math.min(100, (inuse / sys) * 100)
})

const concurrencyChart = ref()
const bandwidthChart = ref()
const gcPauseChart = ref()

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

let pollTimer: ReturnType<typeof setInterval> | null = null
let resizeTimer: ReturnType<typeof setTimeout> | null = null

function onWindowResize() {
  if (resizeTimer) clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    concurrencyChart.value?.resize?.()
    bandwidthChart.value?.resize?.()
    gcPauseChart.value?.resize?.()
  }, 200)
}

onMounted(() => {
  fetchRealtime()
  pollTimer = setInterval(fetchRealtime, 10000)
  elapsedTimer = setInterval(() => { now.value = Date.now() }, 1000)
  window.addEventListener('resize', onWindowResize)
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
  window.removeEventListener('resize', onWindowResize)
  if (resizeTimer) clearTimeout(resizeTimer)
})
</script>

<template>
  <div>
    <PageHeader title="实时监控面板" description="并发请求、网络流量与运行状态">
      <template #actions>
        <div class="header-status">
          <a-tag color="green" size="small">自动刷新 10s</a-tag>
          <span v-if="lastUpdated" class="last-updated">最后更新 {{ fmtLastUpdated() }}</span>
        </div>
      </template>
    </PageHeader>

    <a-spin :loading="loading" style="width: 100%">
    <div v-if="!loading">

    <!-- Key Metrics Cards -->
    <div class="metrics-grid mb-16">
      <div class="metric-card metric-card--blue">
        <div class="mc-icon mc-icon--blue"><IconThunderbolt :size="22" /></div>
        <div class="mc-block">
          <div class="mc-label">当前并发</div>
          <div class="mc-value mc-value--blue">{{ conc.total_active || 0 }}</div>
        </div>
        <div class="mc-sep" />
        <div class="mc-block mc-block--sub">
          <div class="mc-pair"><span class="mc-dot" style="background:#00B42A" /> 流式 <strong>{{ conc.streaming_active || 0 }}</strong></div>
          <div class="mc-pair"><span class="mc-dot" style="background:#FF7D00" /> 非流式 <strong>{{ conc.non_streaming_active || 0 }}</strong></div>
        </div>
      </div>
      <div class="metric-card metric-card--green">
        <div class="mc-icon mc-icon--green"><IconSwap :size="22" /></div>
        <div class="mc-block">
          <div class="mc-label">↓ 入站</div>
          <div class="mc-value mc-value--blue">{{ formatBytes(bw.bytes_in_per_sec || 0) }}/s</div>
        </div>
        <div class="mc-sep" />
        <div class="mc-block mc-block--sub">
          <div class="mc-label">↑ 出站</div>
          <div class="mc-value mc-value--green">{{ formatBytes(bw.bytes_out_per_sec || 0) }}/s</div>
        </div>
      </div>
      <div class="metric-card metric-card--purple">
        <div class="mc-icon mc-icon--purple"><IconClockCircle :size="22" /></div>
        <div class="mc-block">
          <div class="mc-label">运行时间</div>
          <div class="mc-value">{{ goRt.uptime || '-' }}</div>
        </div>
        <div class="mc-sep" />
        <div class="mc-block mc-block--sub">
          <div class="mc-tag-row"><span class="mc-tag">Go {{ goRt.go_version || '-' }}</span></div>
          <div class="mc-tag-row"><span class="mc-tag">{{ goRt.num_cpu || 0 }} 核</span></div>
        </div>
      </div>
      <div class="metric-card metric-card--orange">
        <div class="mc-icon mc-icon--orange"><IconCode :size="22" /></div>
        <div class="mc-block">
          <div class="mc-label">Goroutine</div>
          <div class="mc-value mc-value--orange">{{ goRt.num_goroutine || 0 }}</div>
        </div>
        <div class="mc-sep" />
        <div class="mc-block mc-block--sub">
          <div class="mc-label">Heap 内存</div>
          <div class="mc-value mc-value--orange">{{ (rt.heap_alloc_mb || 0).toFixed(0) }} MB</div>
        </div>
      </div>
    </div>

    <!-- Charts -->
    <div class="row row-2 mb-16">
      <a-card :bordered="false">
        <template #title><span class="card-title"><IconBarChart :size="16" style="color:#165DFF" /> 并发趋势</span></template>
        <v-chart ref="concurrencyChart" :option="concurrencyOption" style="height: 280px" autoresize />
      </a-card>
      <a-card :bordered="false">
        <template #title><span class="card-title"><IconSwap :size="16" style="color:#00B42A" /> 带宽趋势</span></template>
        <v-chart ref="bandwidthChart" :option="bandwidthOption" style="height: 280px" autoresize />
      </a-card>
    </div>

    <!-- Memory & GC -->
    <div class="row row-2 mb-16">
      <a-card :bordered="false">
        <template #title><span class="card-title"><IconLayers :size="16" style="color:#722ED1" /> 内存分配</span></template>
        <div class="mem-summary">
          <div class="mem-stat">
            <span class="mem-stat-value">{{ fmtMB(mem.alloc_mb) }}</span>
            <span class="mem-stat-label">使用中</span>
          </div>
          <div class="mem-stat">
            <span class="mem-stat-value">{{ fmtMB(mem.sys_mb) }}</span>
            <span class="mem-stat-label">从 OS 获取</span>
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
        <div class="info-grid" style="margin-top: 12px">
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
        </div>
      </a-card>
      <a-card :bordered="false">
        <template #title><span class="card-title"><IconCode :size="16" style="color:#FF7D00" /> GC 统计</span></template>
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
        <v-chart ref="gcPauseChart" :option="gcPauseOption" style="height: 120px" autoresize />
      </a-card>
    </div>

    <!-- Distribution -->
    <div class="row row-2 mb-16">
      <a-card :bordered="false">
        <template #title><span class="card-title"><IconApps :size="16" style="color:#165DFF" /> 按模型分布</span></template>
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
      <a-card :bordered="false">
        <template #title><span class="card-title"><IconUserGroup :size="16" style="color:#00B42A" /> 按渠道分布</span></template>
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

    <!-- Active Requests -->
    <a-card :bordered="false" class="mb-16">
      <template #title><span class="card-title"><IconFire :size="16" style="color:#F53F3F" /> 活跃请求</span></template>
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

.mono {
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
  font-size: 12px;
}

/* Metrics cards */
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}
.metric-card {
  background: var(--color-bg-2);
  border: 1px solid var(--color-border);
  border-radius: var(--border-radius-medium);
  padding: 20px;
  display: flex;
  align-items: center;
  border-left: 4px solid transparent;
}
.metric-card--blue {
  border-left-color: #165DFF;
  background: linear-gradient(135deg, rgba(22, 93, 255, 0.04), var(--color-bg-2) 70%);
}
.metric-card--green {
  border-left-color: #00B42A;
  background: linear-gradient(135deg, rgba(0, 180, 42, 0.04), var(--color-bg-2) 70%);
}
.metric-card--purple {
  border-left-color: #722ED1;
  background: linear-gradient(135deg, rgba(114, 46, 209, 0.04), var(--color-bg-2) 70%);
}
.metric-card--orange {
  border-left-color: #FF7D00;
  background: linear-gradient(135deg, rgba(255, 125, 0, 0.04), var(--color-bg-2) 70%);
}
.mc-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 14px;
  flex-shrink: 0;
  margin-right: 16px;
}
.mc-icon :deep(svg) {
  color: #fff;
}
.mc-icon--blue { background: linear-gradient(135deg, #165DFF, #4080FF); }
.mc-icon--green { background: linear-gradient(135deg, #00B42A, #34D399); }
.mc-icon--purple { background: linear-gradient(135deg, #722ED1, #9254DE); }
.mc-icon--orange { background: linear-gradient(135deg, #FF7D00, #FFA940); }
.mc-block {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.mc-block--sub {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 6px;
}
.mc-sep {
  width: 1px;
  align-self: stretch;
  background: var(--color-border);
  flex-shrink: 0;
  margin: 4px 16px;
}
.mc-label {
  font-size: 12px;
  color: var(--color-text-3);
  line-height: 1;
}
.mc-value {
  font-size: 24px;
  font-weight: 600;
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
  line-height: 1.3;
  color: var(--color-text-1);
}
.mc-value--blue { color: #165DFF; }
.mc-value--green { color: #00B42A; }
.mc-value--orange { color: #FF7D00; }
.mc-pair {
  font-size: 13px;
  color: var(--color-text-2);
  display: flex;
  align-items: center;
  gap: 4px;
}
.mc-pair strong {
  font-weight: 600;
  color: var(--color-text-1);
}
.mc-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
}
.mc-tag-row {
  display: flex;
}
.mc-tag {
  display: inline-block;
  font-size: 12px;
  color: var(--color-text-2);
  background: var(--color-fill-2);
  padding: 2px 8px;
  border-radius: 3px;
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
}

/* Memory & GC */
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
.gc-chart-title {
  font-size: 12px;
  color: var(--color-text-3);
  margin: 12px 0 4px;
}

.card-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 15px;
  font-weight: 500;
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
@media (max-width: 900px) {
  .metrics-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
@media (max-width: 768px) {
  .row-2 { grid-template-columns: 1fr; }
}
@media (max-width: 480px) {
  .metrics-grid {
    grid-template-columns: 1fr;
  }
}
</style>
