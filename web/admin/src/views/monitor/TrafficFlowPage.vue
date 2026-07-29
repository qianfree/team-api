<script setup lang="ts">
import { ref, shallowRef, onMounted } from 'vue'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { SankeyChart } from 'echarts/charts'
import { TooltipComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

// 桑基图无需 GridComponent（无笛卡尔坐标轴）
use([CanvasRenderer, SankeyChart, TooltipComponent])

const loading = ref(false)
const metric = ref<'cost' | 'tokens' | 'requests'>('cost')
// 默认近 30 天（backend 也会兜底夹紧）
const dateRange = ref<[string, string] | null>(defaultRange(29))

interface FlowNode { name: string; depth?: number }
interface FlowLink { source: string; target: string; value: number }

const nodes = ref<FlowNode[]>([])
const links = ref<FlowLink[]>([])
const sankeyOption = shallowRef<any>({})

const metricLabel: Record<string, string> = {
  cost: '成本',
  tokens: 'Token',
  requests: '请求数',
}

// --- 日期辅助（native，避免引入 dayjs 依赖） ---

function toDateStr(d: Date): string {
  const m = `${d.getMonth() + 1}`.padStart(2, '0')
  const day = `${d.getDate()}`.padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

function defaultRange(days: number): [string, string] {
  const end = new Date()
  const start = new Date()
  start.setDate(start.getDate() - days)
  return [toDateStr(start), toDateStr(end)]
}

function formatValue(v: number): string {
  if (metric.value === 'cost') return `$${Number(v).toFixed(6)}`
  return Number(v).toLocaleString()
}

// --- ECharts Sankey 组装 ---

function buildSankey() {
  return {
    tooltip: {
      triggerOn: 'mousemove',
      formatter: (p: any) => {
        if (p.dataType === 'edge') {
          return `${p.data.source} → ${p.data.target}<br/><b>${formatValue(p.data.value)}</b>`
        }
        return p.data.name
      },
    },
    series: [
      {
        type: 'sankey',
        data: nodes.value,
        links: links.value,
        orient: 'horizontal',
        nodeAlign: 'left',
        layoutIterations: 32,
        top: 16,
        bottom: 16,
        left: 8,
        right: 120,
        label: { fontSize: 11, color: '#1d2129' },
        lineStyle: { color: 'gradient', opacity: 0.35, curveness: 0.5 },
        emphasis: { focus: 'adjacency' },
      },
    ],
  }
}

// --- 数据获取 ---

async function fetchFlow() {
  loading.value = true
  try {
    const params: Record<string, string> = { metric: metric.value }
    if (dateRange.value && dateRange.value.length === 2) {
      params.start_date = dateRange.value[0]
      params.end_date = dateRange.value[1]
    }
    const res = await request.get('/admin/monitor/traffic-flow', { params })
    const payload = res.data?.data?.data || {}
    nodes.value = payload.nodes || []
    links.value = payload.links || []
    if (nodes.value.length > 0) {
      sankeyOption.value = buildSankey()
    } else {
      sankeyOption.value = {}
    }
  } catch {
    // 错误由 Axios 拦截器统一提示
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchFlow()
})
</script>

<template>
  <div>
    <PageHeader title="流量流向" description="按 租户 → 模型 → 渠道 → 状态 展示流量与成本分布（数据源自 bil_usage_daily，每日聚合）">
      <template #actions>
        <a-space>
          <a-range-picker v-model="dateRange" value-format="YYYY-MM-DD" style="width: 260px" @change="fetchFlow" />
          <a-radio-group v-model="metric" type="button" @change="fetchFlow">
            <a-radio value="cost">成本</a-radio>
            <a-radio value="tokens">Token</a-radio>
            <a-radio value="requests">请求数</a-radio>
          </a-radio-group>
        </a-space>
      </template>
    </PageHeader>

    <a-spin :loading="loading" style="width: 100%">
      <a-card :bordered="false">
        <template #title>
          <div class="card-title">
            <span>流量流向桑基图</span>
            <a-tag color="arcoblue" size="small">指标：{{ metricLabel[metric] }}</a-tag>
            <a-tag v-if="dateRange && dateRange.length === 2" size="small">{{ dateRange[0] }} ~ {{ dateRange[1] }}</a-tag>
          </div>
        </template>
        <v-chart v-if="nodes.length > 0" :option="sankeyOption" style="height: 560px" autoresize />
        <a-empty v-else description="所选区间暂无用量数据" style="padding: 120px 0" />
      </a-card>
    </a-spin>
  </div>
</template>

<style scoped>
.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
