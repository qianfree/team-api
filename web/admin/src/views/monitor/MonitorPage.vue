<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { useWebSocket } from '@/utils/websocket'
import type { WsMessage } from '@/utils/websocket'

const loading = ref(false)
const timeRange = ref(5) // minutes
const dashboardData = ref<any>({})

async function fetchDashboard() {
  loading.value = true
  try {
    const res = await request.get('/admin/monitor/dashboard', { params: { minutes: timeRange.value } })
    dashboardData.value = res.data?.data?.data || {}
  } catch {
    // error auto-shown by Axios interceptor
  } finally {
    loading.value = false
  }
}

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

const { on: wsOn, off: wsOff } = useWebSocket()

function handleMonitorMessage(msg: WsMessage) {
  if (msg.action === 'dashboard' && msg.payload) {
    dashboardData.value = msg.payload
  }
}

onMounted(() => {
  fetchDashboard() // initial load via HTTP
  wsOn('monitor', handleMonitorMessage)
})

onUnmounted(() => {
  wsOff('monitor', handleMonitorMessage)
})
</script>

<template>
  <div>
    <PageHeader title="系统监控" description="实时监控系统运行状态">
      <template #actions>
        <a-radio-group v-model="timeRange" type="button" @change="fetchDashboard">
          <a-radio :value="5">5分钟</a-radio>
          <a-radio :value="15">15分钟</a-radio>
          <a-radio :value="30">30分钟</a-radio>
          <a-radio :value="60">1小时</a-radio>
        </a-radio-group>
      </template>
    </PageHeader>

    <a-spin :loading="loading" style="width: 100%">
      <!-- System resource cards -->
      <div class="grid grid-cols-4 gap-4 mb-4">
        <a-card :bordered="false">
          <a-statistic title="CPU 使用率" :value="num(dashboardData.system?.cpu?.percent)" :precision="1">
            <template #suffix>
              <span style="font-size: 14px">%</span>
            </template>
          </a-statistic>
          <div style="margin-top: 8px">
            <a-progress :percent="pct(dashboardData.system?.cpu?.percent)" :show-text="false"
              :color="colorThreshold(dashboardData.system?.cpu?.percent, 60, 85)" />
          </div>
        </a-card>
        <a-card :bordered="false">
          <a-statistic title="内存使用率" :value="num(dashboardData.system?.memory?.used_percent)" :precision="0">
            <template #suffix>
              <span style="font-size: 14px">%</span>
            </template>
          </a-statistic>
          <div style="margin-top: 4px; font-size: 12px; color: var(--color-text-3)">
            {{ num(dashboardData.system?.memory?.used_mb, 0).toFixed(0) }} / {{ num(dashboardData.system?.memory?.total_mb, 0).toFixed(0) }} MB
          </div>
          <div style="margin-top: 8px">
            <a-progress :percent="pct(dashboardData.system?.memory?.used_percent)" :show-text="false"
              :color="colorThreshold(dashboardData.system?.memory?.used_percent, 60, 85)" />
          </div>
        </a-card>
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
      </div>

      <!-- API Metrics row -->
      <div class="grid grid-cols-4 gap-4 mb-4">
        <a-card :bordered="false">
          <a-statistic title="QPS (请求/秒)" :value="num(dashboardData.api?.qps)" :precision="2" />
        </a-card>
        <a-card :bordered="false">
          <a-statistic title="TPM (Token/分钟)" :value="num(dashboardData.api?.tpm)" :precision="0" />
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
</style>
