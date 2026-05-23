<script setup lang="ts">
import { ref, reactive, onMounted, h, computed } from 'vue'
import { Tag, Button, Select, Input, DatePicker } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const data = ref<any[]>([])
const total = ref(0)
const pagination = reactive({ current: 1, pageSize: 20 })

// Filters
const filterChannelId = ref<number | undefined>(undefined)
const filterCategory = ref('')
const filterStatusCode = ref<number | undefined>(undefined)
const filterDateRange = ref<string[] | undefined>(undefined)
const filterKeyword = ref('')

// Stats
const stats = ref<any>(null)
const statsLoading = ref(false)

// Category options
const categoryOptions = ref<any[]>([])
const hoursRange = ref(24)

// Error category display
const categoryColorMap: Record<string, string> = {
  rate_limit: 'orange',
  auth_error: 'red',
  timeout: 'purple',
  upstream_error: 'arcoblue',
  server_error: 'red',
  network_error: 'magenta',
  unknown: 'gray',
}
const categoryLabelMap: Record<string, string> = {
  rate_limit: '频率限制',
  auth_error: '认证错误',
  timeout: '超时',
  upstream_error: '上游错误',
  server_error: '服务端错误',
  network_error: '网络错误',
  unknown: '未知',
}

const columns: TableColumnData[] = [
  {
    title: '时间',
    dataIndex: 'created_at',
    width: 160,
    render({ record }: { record: any }) {
      return new Date(record.created_at).toLocaleString('zh-CN')
    },
  },
  {
    title: '渠道',
    dataIndex: 'channel_name',
    width: 120,
    ellipsis: true,
  },
  {
    title: '供应商',
    dataIndex: 'provider',
    width: 90,
  },
  {
    title: '模型',
    dataIndex: 'model_name',
    width: 140,
    ellipsis: true,
  },
  {
    title: '分类',
    dataIndex: 'error_category',
    width: 100,
    render({ record }: { record: any }) {
      return h(Tag, { color: categoryColorMap[record.error_category] || 'gray', size: 'small' }, () => categoryLabelMap[record.error_category] || record.error_category)
    },
  },
  {
    title: '状态码',
    dataIndex: 'status_code',
    width: 80,
    render({ record }: { record: any }) {
      const color = record.status_code >= 500 ? 'red' : record.status_code >= 400 ? 'orange' : 'blue'
      return h(Tag, { color, size: 'small' }, () => String(record.status_code))
    },
  },
  {
    title: '耗时',
    dataIndex: 'latency_ms',
    width: 80,
    render({ record }: { record: any }) {
      if (!record.latency_ms) return '-'
      return `${Math.round(record.latency_ms)}ms`
    },
  },
  {
    title: '错误信息',
    dataIndex: 'error_message',
    ellipsis: true,
    tooltip: true,
  },
  {
    title: '重试',
    dataIndex: 'attempt',
    width: 80,
    render({ record }: { record: any }) {
      const text = record.is_final ? `第${record.attempt + 1}次(终)` : `第${record.attempt + 1}次`
      const color = record.is_final ? 'red' : 'orange'
      return h(Tag, { color, size: 'small' }, () => text)
    },
  },
  {
    title: '请求ID',
    dataIndex: 'request_id',
    width: 140,
    ellipsis: true,
    tooltip: true,
  },
]

async function fetchCategories() {
  try {
    const res = await request.get('/admin/monitor/channel-errors/categories')
    categoryOptions.value = res.data?.data?.data || []
  } catch {
    categoryOptions.value = []
  }
}

async function fetchStats() {
  statsLoading.value = true
  try {
    const res = await request.get('/admin/monitor/channel-errors/stats', {
      params: { hours: hoursRange.value },
    })
    stats.value = res.data?.data || null
  } catch {
    stats.value = null
  } finally {
    statsLoading.value = false
  }
}

async function fetchData() {
  loading.value = true
  try {
    const params: any = {
      page: pagination.current,
      page_size: pagination.pageSize,
    }
    if (filterChannelId.value) params.channel_id = filterChannelId.value
    if (filterCategory.value) params.error_category = filterCategory.value
    if (filterStatusCode.value) params.status_code = filterStatusCode.value
    if (filterKeyword.value) params.keyword = filterKeyword.value
    if (filterDateRange.value && filterDateRange.value.length === 2) {
      params.start_date = filterDateRange.value[0]
      params.end_date = filterDateRange.value[1]
    }

    const res = await request.get('/admin/monitor/channel-errors', { params })
    data.value = res.data?.data?.list || []
    total.value = res.data?.data?.total || 0
  } catch {
    data.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  fetchData()
  fetchStats()
}

function handleReset() {
  filterChannelId.value = undefined
  filterCategory.value = ''
  filterStatusCode.value = undefined
  filterDateRange.value = undefined
  filterKeyword.value = ''
  pagination.current = 1
  fetchData()
  fetchStats()
}

function handlePageChange(page: number) {
  pagination.current = page
  fetchData()
}

function handlePageSizeChange(pageSize: number) {
  pagination.pageSize = pageSize
  pagination.current = 1
  fetchData()
}

const statsCards = computed(() => {
  if (!stats.value) return []
  const rateLimitCount = stats.value.by_category?.find((c: any) => c.error_category === 'rate_limit')?.count || 0
  const serverErrorCount = stats.value.by_category?.find((c: any) => c.error_category === 'server_error')?.count || 0
  const topChannel = stats.value.top_channels?.[0]?.channel_name || '-'
  return [
    { label: `近${hoursRange.value}h错误总数`, value: stats.value.total || 0, color: '#f53f3f' },
    { label: '频率限制', value: rateLimitCount, color: '#ff7d00' },
    { label: '服务端错误', value: serverErrorCount, color: '#f53f3f' },
    { label: '最严重渠道', value: topChannel, color: '#722ed1' },
  ]
})

onMounted(() => {
  fetchCategories()
  fetchData()
  fetchStats()
})
</script>

<template>
  <div class="page-container">
    <PageHeader title="渠道错误监控" description="上游渠道错误事件记录与趋势分析" />

    <!-- Stats Cards -->
    <div class="stats-row">
      <div v-for="(card, idx) in statsCards" :key="idx" class="stat-card">
        <div class="stat-value" :style="{ color: card.color }">{{ card.value }}</div>
        <div class="stat-label">{{ card.label }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">时间范围</div>
        <Select v-model="hoursRange" size="small" style="width: 100px" @change="fetchStats">
          <Select.Option :value="1">1小时</Select.Option>
          <Select.Option :value="6">6小时</Select.Option>
          <Select.Option :value="24">24小时</Select.Option>
          <Select.Option :value="168">7天</Select.Option>
        </Select>
      </div>
    </div>

    <!-- Filters -->
    <ACard :bordered="false" class="filter-card">
      <ASpace wrap>
        <Input
          v-model="filterKeyword"
          placeholder="搜索渠道/模型/错误信息"
          style="width: 220px"
          allow-clear
          @press-enter="handleSearch"
        />
        <Select
          v-model="filterCategory"
          placeholder="错误分类"
          allow-clear
          style="width: 140px"
        >
          <Select.Option v-for="opt in categoryOptions" :key="opt.value" :value="opt.value">
            {{ opt.label }}
          </Select.Option>
        </Select>
        <Input
          :model-value="filterStatusCode != null ? String(filterStatusCode) : undefined"
          placeholder="状态码"
          style="width: 100px"
          allow-clear
          @input="(val: string) => filterStatusCode = val ? Number(val) : undefined"
        />
        <DatePicker
          :model-value="filterDateRange"
          range
          style="width: 240px"
          format="YYYY-MM-DD"
          @change="(val: any) => filterDateRange = val"
        />
        <Button type="primary" @click="handleSearch">搜索</Button>
        <Button @click="handleReset">重置</Button>
      </ASpace>
    </ACard>

    <!-- Table -->
    <ACard :bordered="false">
      <ATable
        :data="data"
        :loading="loading"
        :columns="columns"
        :pagination="{
          current: pagination.current,
          pageSize: pagination.pageSize,
          total: total,
          showTotal: true,
          showPageSize: true,
        }"
        row-key="id"
        :scroll="{ x: 1400 }"
        @page-change="handlePageChange"
        @page-size-change="handlePageSizeChange"
      />
    </ACard>
  </div>
</template>

<style scoped>
.page-container {
  padding: 20px;
}

.stats-row {
  display: flex;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.stat-card {
  background: var(--color-bg-2);
  border: 1px solid var(--color-border-2);
  border-radius: 8px;
  padding: 16px 24px;
  min-width: 160px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  line-height: 1.2;
}

.stat-label {
  font-size: 13px;
  color: var(--color-text-3);
}

.filter-card {
  margin-bottom: 16px;
}
</style>
