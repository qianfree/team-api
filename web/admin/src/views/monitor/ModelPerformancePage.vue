<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { Tag } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import TableStats from '@/components/TableStats.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import request from '@/utils/request'

const loading = ref(false)
const data = ref<any[]>([])
// 默认只查当天（配合后端 Redis 当日实时热桶，打开即可看到最新的模型性能）
const dateRange = ref<[string, string] | null>(defaultRange(0))

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

// 快捷时间范围：start/end 为相对今天的偏移天数（0=今天，-1=昨天...）
const QUICK_RANGES = [
  { label: '今天', start: 0, end: 0 },
  { label: '昨天', start: -1, end: -1 },
  { label: '最近3天', start: -2, end: 0 },
  { label: '最近一周', start: -6, end: 0 },
  { label: '最近一月', start: -29, end: 0 },
]

function dateOffsetStr(offset: number): string {
  const d = new Date()
  d.setDate(d.getDate() + offset)
  return toDateStr(d)
}

function applyQuickRange(q: { start: number; end: number }) {
  dateRange.value = [dateOffsetStr(q.start), dateOffsetStr(q.end)]
  fetchData()
}

// 当前选择的日期范围与某快捷范围一致时将其按钮高亮（primary）
function isActive(q: { start: number; end: number }): boolean {
  return (
    !!dateRange.value &&
    dateRange.value[0] === dateOffsetStr(q.start) &&
    dateRange.value[1] === dateOffsetStr(q.end)
  )
}

function fmtNum(v: any): number {
  return typeof v === 'number' ? v : 0
}

// 成功率分级标签：excellent≥99 / good≥95 / warning≥90 / critical<90
const gradeColor: Record<string, string> = {
  excellent: 'green',
  good: 'arcoblue',
  warning: 'orange',
  critical: 'red',
}

const columns: TableColumnData[] = [
  { title: '模型', dataIndex: 'model_name', ellipsis: true, tooltip: true, width: 220, sortable: { sortDirections: ['ascend', 'descend'] } },
  {
    title: '请求数',
    dataIndex: 'request_count',
    width: 110,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => fmtNum(record.request_count).toLocaleString(),
  },
  {
    title: '成功率',
    dataIndex: 'success_rate',
    width: 120,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) =>
      h(Tag, { color: gradeColor[record.grade] || 'blue', size: 'small' }, () => `${fmtNum(record.success_rate).toFixed(2)}%`),
  },
  {
    title: '平均延迟',
    dataIndex: 'avg_latency_ms',
    width: 120,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => `${fmtNum(record.avg_latency_ms).toFixed(0)} ms`,
  },
  {
    title: '平均首Token',
    dataIndex: 'avg_first_token_ms',
    width: 130,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => `${fmtNum(record.avg_first_token_ms).toFixed(0)} ms`,
  },
  {
    title: '吞吐 TPS',
    dataIndex: 'tps',
    width: 110,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => `${fmtNum(record.tps).toFixed(1)} t/s`,
  },
  {
    title: '总 Token',
    dataIndex: 'total_tokens',
    width: 130,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => fmtNum(record.total_tokens).toLocaleString(),
  },
  {
    title: '总成本 (USD)',
    dataIndex: 'total_cost',
    width: 140,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => `$${fmtNum(record.total_cost).toFixed(6)}`,
  },
]

// 成功率分级（与后端 gradeSuccessRate 同阈值）：excellent≥99 / good≥95 / warning≥90 / critical<90
function rateGrade(rate: number): string {
  if (rate >= 99) return 'excellent'
  if (rate >= 95) return 'good'
  if (rate >= 90) return 'warning'
  return 'critical'
}

// 概览卡颜色：按成功率分级映射（红色仅用于 critical）
const gradeCardClass: Record<string, string> = {
  excellent: 'metric-card--green',
  good: 'metric-card--green',
  warning: 'metric-card--orange',
  critical: 'metric-card--red',
}

// 顶部概览统计：从当前列表聚合（无额外请求）
const stats = computed(() => {
  let totalRequests = 0
  let totalSuccess = 0
  let totalCost = 0
  for (const item of data.value) {
    totalRequests += fmtNum(item.request_count)
    totalSuccess += fmtNum(item.success_count)
    totalCost += fmtNum(item.total_cost)
  }
  return {
    modelCount: data.value.length,
    totalRequests,
    overallRate: totalRequests > 0 ? (totalSuccess / totalRequests) * 100 : 0,
    totalCost,
  }
})

async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, string> = {}
    if (dateRange.value && dateRange.value.length === 2) {
      params.start_date = dateRange.value[0]
      params.end_date = dateRange.value[1]
    }
    const res = await request.get('/admin/monitor/model-performance', { params })
    data.value = res.data?.data?.data?.list || []
  } catch {
    // 错误由 Axios 拦截器统一提示
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="模型性能" description="各模型跨渠道的成功率 / 延迟 / 吞吐（历史数据源自 bil_usage_daily 每日聚合，当天实时统计；延迟为总延迟/总请求数均值）">
      <template #actions>
        <a-button
          size="small"
          v-for="q in QUICK_RANGES"
          :key="q.label"
          :type="isActive(q) ? 'primary' : undefined"
          @click="applyQuickRange(q)"
        >
          {{ q.label }}
        </a-button>
        <a-range-picker v-model="dateRange" value-format="YYYY-MM-DD" style="width: 260px" @change="fetchData" />
        <a-button size="small" :loading="loading" @click="fetchData">刷新</a-button>
      </template>
    </PageHeader>

    <div class="stats-row">
      <div class="metric-card metric-card--blue">
        <div class="metric-card__label">模型数</div>
        <div class="metric-card__value">{{ stats.modelCount }}</div>
      </div>
      <div class="metric-card metric-card--purple">
        <div class="metric-card__label">总请求数</div>
        <div class="metric-card__value">{{ stats.totalRequests.toLocaleString() }}</div>
      </div>
      <div class="metric-card" :class="stats.totalRequests > 0 ? gradeCardClass[rateGrade(stats.overallRate)] : ''">
        <div class="metric-card__label">整体成功率</div>
        <div class="metric-card__value">{{ stats.overallRate.toFixed(2) }}%</div>
      </div>
      <div class="metric-card metric-card--orange">
        <div class="metric-card__label">总成本 (USD)</div>
        <div class="metric-card__value">${{ stats.totalCost.toFixed(6) }}</div>
      </div>
    </div>

    <a-card :bordered="false">
      <ResponsiveTable
        :columns="columns"
        :data="data"
        :loading="loading"
        row-key="model_name"
        :scroll="{ x: 1200 }"
        size="large"
        stripe
        :border="{ wrapper: true, headerCell: true }"
        card-title-key="model_name"
        card-subtitle-key="request_count"
        card-badge-key="success_rate"
        :card-fields="['avg_latency_ms', 'avg_first_token_ms', 'tps', 'total_tokens', 'total_cost']"
      >
        <template #empty>
          <a-empty description="所选区间暂无模型性能数据" />
        </template>
      </ResponsiveTable>
      <div class="table-footer">
        <TableStats :total="data.length" />
      </div>
    </a-card>
  </div>
</template>

<style scoped>
/* 顶部概览卡片：与实时监控页 metric-card 风格保持一致（左色条 + 渐变底） */
.stats-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}
.metric-card {
  background: var(--color-bg-2);
  border: 1px solid var(--color-border);
  border-radius: var(--border-radius-medium);
  padding: 18px 20px;
  border-left: 4px solid transparent;
}
.metric-card--blue {
  border-left-color: #165dff;
  background: linear-gradient(135deg, rgba(22, 93, 255, 0.05), var(--color-bg-2) 70%);
}
.metric-card--purple {
  border-left-color: #722ed1;
  background: linear-gradient(135deg, rgba(114, 46, 209, 0.05), var(--color-bg-2) 70%);
}
.metric-card--green {
  border-left-color: #00b42a;
  background: linear-gradient(135deg, rgba(0, 180, 42, 0.05), var(--color-bg-2) 70%);
}
.metric-card--orange {
  border-left-color: #ff7d00;
  background: linear-gradient(135deg, rgba(255, 125, 0, 0.05), var(--color-bg-2) 70%);
}
.metric-card--red {
  border-left-color: #f53f3f;
  background: linear-gradient(135deg, rgba(245, 63, 63, 0.05), var(--color-bg-2) 70%);
}
.metric-card__label {
  font-size: 13px;
  color: var(--color-text-3);
  margin-bottom: 8px;
}
.metric-card__value {
  font-size: 26px;
  font-weight: 600;
  color: var(--color-text-1);
  font-variant-numeric: tabular-nums;
}
@media (max-width: 768px) {
  .stats-row {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
