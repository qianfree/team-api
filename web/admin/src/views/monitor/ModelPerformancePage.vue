<script setup lang="ts">
import { ref, shallowRef, onMounted, h } from 'vue'
import { Tag } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import TableStats from '@/components/TableStats.vue'
import request from '@/utils/request'

const loading = ref(false)
const data = ref<any[]>([])
const dateRange = ref<[string, string] | null>(defaultRange(29))

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
  { title: '模型', dataIndex: 'model_name', ellipsis: true, width: 220 },
  {
    title: '请求数',
    dataIndex: 'request_count',
    width: 110,
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => fmtNum(record.request_count).toLocaleString(),
  },
  {
    title: '成功率',
    dataIndex: 'success_rate',
    width: 110,
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) =>
      h(Tag, { color: gradeColor[record.grade] || 'blue', size: 'small' }, () => `${fmtNum(record.success_rate).toFixed(2)}%`),
  },
  {
    title: '平均延迟',
    dataIndex: 'avg_latency_ms',
    width: 110,
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => `${fmtNum(record.avg_latency_ms).toFixed(0)} ms`,
  },
  {
    title: '平均首Token',
    dataIndex: 'avg_first_token_ms',
    width: 120,
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => `${fmtNum(record.avg_first_token_ms).toFixed(0)} ms`,
  },
  {
    title: '吞吐 TPS',
    dataIndex: 'tps',
    width: 110,
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => `${fmtNum(record.tps).toFixed(1)} t/s`,
  },
  {
    title: '总 Token',
    dataIndex: 'total_tokens',
    width: 130,
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => fmtNum(record.total_tokens).toLocaleString(),
  },
  {
    title: '总成本 (USD)',
    dataIndex: 'total_cost',
    width: 130,
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => `$${fmtNum(record.total_cost).toFixed(4)}`,
  },
]

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
    <PageHeader title="模型性能" description="各模型跨渠道的成功率 / 延迟 / 吞吐（数据源自 bil_usage_daily，每日聚合；延迟为总延迟/总请求数均值）">
      <template #actions>
        <a-range-picker v-model="dateRange" value-format="YYYY-MM-DD" style="width: 260px" @change="fetchData" />
      </template>
    </PageHeader>

    <a-card :bordered="false">
      <TableStats :total="data.length" />
      <a-table
        :columns="columns"
        :data="data"
        :loading="loading"
        row-key="model_name"
        :pagination="false"
        :scroll="{ x: 1200 }"
        size="medium"
      />
    </a-card>
  </div>
</template>
