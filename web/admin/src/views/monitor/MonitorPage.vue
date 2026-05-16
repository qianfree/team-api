<script setup lang="ts">
import { ref, shallowRef, onMounted } from 'vue'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, MarkLineComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

use([CanvasRenderer, LineChart, GridComponent, TooltipComponent, MarkLineComponent])

const loading = ref(false)
const timeRange = ref(5) // minutes
const dashboardData = ref<any>({})
const systemHistory = ref<any[]>([])
const trafficData = ref<any[]>([])

function num(v: any, fallback = 0): number {
  return typeof v === 'number' ? v : fallback
}

function pct(v: any): number {
  return typeof v === 'number' ? v / 100 : 0
}

function colorThreshold(v: any, warnAt: number, critAt: number): string {
  const n = typeof v === 'number' ? v : 0
  if (n > critAt) return '#f53f3f'
  if (n > warnAt) return '#ff7d00'
  return '#00b42a'
}

// --- time formatting ---

function fmtTimeSec(ts: string) {
  const d = new Date(ts)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`
}

function fmtTimeMin(ts: string) {
  const d = new Date(ts)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

// --- full chart options with time axis ---

interface ChartPoint { time: string; value: number }

function buildTimeSeriesChart(
  points: ChartPoint[],
  opts: {
    color: string
    unit: string
    yMin?: number
    yMax?: number
    decimals?: number
    warnLine?: number
    critLine?: number
  },
) {
  const times = points.map(p => p.time)
  const values = points.map(p => p.value)
  const dec = opts.decimals ?? 1
  const maxVal = Math.max(...values, 1)

  const markLine: any = { silent: true, symbol: 'none', data: [] as any[] }
  if (opts.warnLine != null) {
    markLine.data.push({ yAxis: opts.warnLine, lineStyle: { color: '#ff7d00', type: 'dashed', width: 1 }, label: { formatter: '警告', fontSize: 11, color: '#ff7d00' } })
  }
  if (opts.critLine != null) {
    markLine.data.push({ yAxis: opts.critLine, lineStyle: { color: '#f53f3f', type: 'dashed', width: 1 }, label: { formatter: '严重', fontSize: 11, color: '#f53f3f' } })
  }

  return {
    tooltip: {
      trigger: 'axis',
      formatter(params: any) {
        const p = params?.[0]
        if (!p) return ''
        return `${p.axisValue}<br/><span style="color:${opts.color}">●</span> ${p.value.toFixed(dec)} ${opts.unit}`
      },
    },
    grid: { left: 52, right: 16, top: 12, bottom: 28 },
    xAxis: {
      type: 'category',
      data: times,
      boundaryGap: false,
      axisLabel: { fontSize: 11, color: '#86909c', interval: 'auto' as const },
      axisLine: { lineStyle: { color: '#e5e6eb' } },
      axisTick: { show: false },
    },
    yAxis: {
      type: 'value',
      min: opts.yMin ?? 0,
      max: opts.yMax ?? (maxVal * 1.3),
      axisLabel: { fontSize: 11, color: '#86909c', formatter: (v: number) => v.toFixed(v >= 100 ? 0 : dec) },
      splitLine: { lineStyle: { type: 'dashed', color: '#e5e6eb' } },
    },
    series: [{
      type: 'line',
      data: values,
      smooth: true,
      showSymbol: false,
      lineStyle: { width: 2, color: opts.color },
      areaStyle: {
        color: {
          type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: opts.color + '25' },
            { offset: 1, color: opts.color + '05' },
          ],
        },
      },
      markLine: markLine.data.length > 0 ? markLine : undefined,
    }],
  }
}

const cpuChartOption = shallowRef<any>({})
const memChartOption = shallowRef<any>({})
const qpsChartOption = shallowRef<any>({})
const tpmChartOption = shallowRef<any>({})

function updateCharts() {
  // CPU trend
  if (systemHistory.value.length > 1) {
    cpuChartOption.value = buildTimeSeriesChart(
      systemHistory.value.map((h: any) => ({
        time: fmtTimeSec(h.timestamp),
        value: h.cpu?.percent ?? 0,
      })),
      { color: '#165DFF', unit: '%', decimals: 1, yMax: 100, warnLine: 60, critLine: 85 },
    )
  }

  // Memory trend
  if (systemHistory.value.length > 1) {
    memChartOption.value = buildTimeSeriesChart(
      systemHistory.value.map((h: any) => ({
        time: fmtTimeSec(h.timestamp),
        value: h.memory?.used_percent ?? 0,
      })),
      { color: '#00B42A', unit: '%', decimals: 0, yMax: 100, warnLine: 60, critLine: 85 },
    )
  }

  // QPS trend
  if (trafficData.value.length > 1) {
    qpsChartOption.value = buildTimeSeriesChart(
      trafficData.value.map((t: any) => ({
        time: fmtTimeMin(t.time),
        value: (t.requests ?? 0) / 60,
      })),
      { color: '#722ED1', unit: 'req/s', decimals: 2 },
    )
  }

  // TPM trend
  if (trafficData.value.length > 1) {
    tpmChartOption.value = buildTimeSeriesChart(
      trafficData.value.map((t: any) => ({
        time: fmtTimeMin(t.time),
        value: t.tokens ?? 0,
      })),
      { color: '#F77234', unit: 'tokens', decimals: 0 },
    )
  }
}

// --- data fetching ---

async function fetchDashboard() {
  try {
    const res = await request.get('/admin/monitor/dashboard', { params: { minutes: timeRange.value } })
    dashboardData.value = res.data?.data?.data || {}
  } catch {
    // error auto-shown by Axios interceptor
  }
}

async function fetchSystemHistory() {
  try {
    const res = await request.get('/admin/monitor/system', { params: { minutes: timeRange.value } })
    systemHistory.value = res.data?.data?.data?.history || []
  } catch {
    // keep previous data
  }
}

async function fetchTraffic() {
  try {
    const res = await request.get('/admin/monitor/traffic', { params: { minutes: timeRange.value } })
    trafficData.value = res.data?.data?.data || []
  } catch {
    // keep previous data
  }
}

async function fetchAll() {
  loading.value = true
  try {
    await Promise.all([fetchDashboard(), fetchSystemHistory(), fetchTraffic()])
    updateCharts()
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchAll()
})
</script>

<template>
  <div>
    <PageHeader title="系统监控" description="实时监控系统运行状态">
      <template #actions>
        <a-radio-group v-model="timeRange" type="button" @change="fetchAll">
          <a-radio :value="5">5分钟</a-radio>
          <a-radio :value="15">15分钟</a-radio>
          <a-radio :value="30">30分钟</a-radio>
          <a-radio :value="60">1小时</a-radio>
        </a-radio-group>
      </template>
    </PageHeader>

    <a-spin :loading="loading" style="width: 100%">
      <!-- System resource trends -->
      <div class="grid grid-cols-2 gap-4 mb-4">
        <a-card :bordered="false">
          <template #title>
            <div class="card-title">
              <span>CPU 使用率</span>
              <a-tag :color="colorThreshold(dashboardData.system?.cpu?.percent, 60, 85) === '#00b42a' ? 'green' : colorThreshold(dashboardData.system?.cpu?.percent, 60, 85) === '#ff7d00' ? 'orangered' : 'red'" size="small">
                {{ num(dashboardData.system?.cpu?.percent).toFixed(1) }}%
              </a-tag>
            </div>
          </template>
          <v-chart v-if="systemHistory.length > 1" :option="cpuChartOption" style="height: 220px" autoresize />
          <a-empty v-else description="暂无数据" style="padding: 60px 0" />
        </a-card>
        <a-card :bordered="false">
          <template #title>
            <div class="card-title">
              <span>内存使用率</span>
              <a-tag :color="colorThreshold(dashboardData.system?.memory?.used_percent, 60, 85) === '#00b42a' ? 'green' : colorThreshold(dashboardData.system?.memory?.used_percent, 60, 85) === '#ff7d00' ? 'orangered' : 'red'" size="small">
                {{ num(dashboardData.system?.memory?.used_percent).toFixed(0) }}%
                ({{ num(dashboardData.system?.memory?.used_mb, 0).toFixed(0) }} / {{ num(dashboardData.system?.memory?.total_mb, 0).toFixed(0) }} MB)
              </a-tag>
            </div>
          </template>
          <v-chart v-if="systemHistory.length > 1" :option="memChartOption" style="height: 220px" autoresize />
          <a-empty v-else description="暂无数据" style="padding: 60px 0" />
        </a-card>
      </div>

      <!-- API traffic trends -->
      <div class="grid grid-cols-2 gap-4 mb-4">
        <a-card :bordered="false">
          <template #title>
            <div class="card-title">
              <span>QPS (请求/秒)</span>
              <a-tag color="arcoblue" size="small">{{ num(dashboardData.api?.qps).toFixed(2) }}</a-tag>
            </div>
          </template>
          <v-chart v-if="trafficData.length > 1" :option="qpsChartOption" style="height: 220px" autoresize />
          <a-empty v-else description="暂无数据" style="padding: 60px 0" />
        </a-card>
        <a-card :bordered="false">
          <template #title>
            <div class="card-title">
              <span>TPM (Token/分钟)</span>
              <a-tag color="orangered" size="small">{{ num(dashboardData.api?.tpm).toFixed(0) }}</a-tag>
            </div>
          </template>
          <v-chart v-if="trafficData.length > 1" :option="tpmChartOption" style="height: 220px" autoresize />
          <a-empty v-else description="暂无数据" style="padding: 60px 0" />
        </a-card>
      </div>

      <!-- Quick stats row -->
      <div class="grid grid-cols-4 gap-4 mb-4">
        <a-card :bordered="false">
          <a-statistic title="磁盘使用率" :value="num(dashboardData.system?.disk?.used_percent)" :precision="1">
            <template #suffix>
              <span style="font-size: 14px">%</span>
            </template>
          </a-statistic>
          <div style="margin-top: 4px; font-size: 12px; color: var(--color-text-3)">
            {{ num(dashboardData.system?.disk?.used_gb, 0).toFixed(0) }} / {{ num(dashboardData.system?.disk?.total_gb, 0).toFixed(0) }} GB
          </div>
          <div style="margin-top: 8px">
            <a-progress :percent="pct(dashboardData.system?.disk?.used_percent)" :show-text="false"
              :color="colorThreshold(dashboardData.system?.disk?.used_percent, 70, 90)" />
          </div>
        </a-card>
        <a-card :bordered="false">
          <a-statistic title="Goroutine 数" :value="num(dashboardData.system?.runtime?.goroutine_count)" />
          <div style="margin-top: 4px; font-size: 12px; color: var(--color-text-3)">
            Heap: {{ num(dashboardData.system?.runtime?.heap_alloc_mb, 0).toFixed(0) }} MB
          </div>
        </a-card>
        <a-card :bordered="false">
          <a-statistic title="P99 延迟 (ms)" :value="num(dashboardData.api?.latency?.p99)" :precision="0" />
        </a-card>
        <a-card :bordered="false">
          <a-statistic title="错误率" :value="num(dashboardData.api?.error_rates?.total)" :precision="2">
            <template #suffix>
              <span style="font-size: 14px">%</span>
            </template>
          </a-statistic>
        </a-card>
      </div>

      <!-- DB and Redis pool -->
      <div class="grid grid-cols-2 gap-4 mb-4">
        <a-card :bordered="false" title="数据库连接池">
          <div class="grid grid-cols-4 gap-4">
            <a-statistic title="活跃连接" :value="num(dashboardData.db_pool?.active_connections)" />
            <a-statistic title="空闲连接" :value="num(dashboardData.db_pool?.idle_connections)" />
            <a-statistic title="总连接" :value="num(dashboardData.db_pool?.total_connections)" />
            <a-statistic title="最大连接" :value="num(dashboardData.db_pool?.max_connections)" />
          </div>
        </a-card>
        <a-card :bordered="false" title="Redis 缓存">
          <div class="grid grid-cols-4 gap-4">
            <a-statistic title="连接数" :value="num(dashboardData.redis_pool?.connected_clients)" />
            <a-statistic title="内存使用" :value="num(dashboardData.redis_pool?.used_memory_mb)" :precision="1">
              <template #suffix>
                <span style="font-size: 14px">MB</span>
              </template>
            </a-statistic>
            <a-statistic title="OPS" :value="num(dashboardData.redis_pool?.instantaneous_ops)" />
            <a-statistic title="命中率" :value="num(dashboardData.redis_pool?.hit_rate)" :precision="1">
              <template #suffix>
                <span style="font-size: 14px">%</span>
              </template>
            </a-statistic>
          </div>
        </a-card>
      </div>

      <!-- Alerts summary -->
      <a-card :bordered="false" title="告警概要" class="mb-4">
        <div class="grid grid-cols-4 gap-4">
          <a-statistic title="触发中" :value="num(dashboardData.alerts?.total_firing)"
            :value-style="{ color: num(dashboardData.alerts?.total_firing) > 0 ? '#f53f3f' : undefined }" />
          <a-statistic title="已确认" :value="num(dashboardData.alerts?.status_counts?.acknowledged)" />
          <a-statistic title="严重" :value="num(dashboardData.alerts?.level_counts?.critical)"
            :value-style="{ color: num(dashboardData.alerts?.level_counts?.critical) > 0 ? '#f53f3f' : undefined }" />
          <a-statistic title="警告" :value="num(dashboardData.alerts?.level_counts?.warning)"
            :value-style="{ color: num(dashboardData.alerts?.level_counts?.warning) > 0 ? '#ff7d00' : undefined }" />
        </div>
      </a-card>
    </a-spin>
  </div>
</template>

<style scoped>
.grid {
  display: grid;
}
.grid-cols-2 { grid-template-columns: repeat(2, 1fr); }
.grid-cols-4 { grid-template-columns: repeat(4, 1fr); }
.gap-4 { gap: 16px; }
.mb-4 { margin-bottom: 16px; }
.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
