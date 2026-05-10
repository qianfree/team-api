<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import { Message, Tag, Button, Space } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const data = ref<any[]>([])
const total = ref(0)
const pagination = reactive({ current: 1, pageSize: 20 })

const filterSource = ref('')
const filterErrorCode = ref('')
const filterResolved = ref('')
const filterDateRange = ref<string[]>([])
const filterKeyword = ref('')

// Stats
const stats = ref<any>(null)
const statsLoading = ref(false)

// Detail drawer
const detailVisible = ref(false)
const detailData = ref<any>(null)
const detailLoading = ref(false)

// Batch resolve
const selectedKeys = ref<number[]>([])

const sourceColorMap: Record<string, string> = {
  api: 'arcoblue',
  panic: 'red',
  cron: 'orange',
  background: 'purple',
}
const sourceLabelMap: Record<string, string> = {
  api: 'API',
  panic: 'Panic',
  cron: '定时任务',
  background: '后台任务',
}

const columns: TableColumnData[] = [
  {
    title: 'ID',
    dataIndex: 'id',
    width: 60,
  },
  {
    title: '来源',
    dataIndex: 'source',
    width: 100,
    render({ record }: { record: any }) {
      return h(Tag, { color: sourceColorMap[record.source] || 'blue', size: 'small' }, () => sourceLabelMap[record.source] || record.source)
    },
  },
  {
    title: '错误码',
    dataIndex: 'error_code',
    width: 80,
    render({ record }: { record: any }) {
      const color = record.error_code >= 500 ? 'red' : 'orange'
      return h(Tag, { color, size: 'small' }, () => String(record.error_code))
    },
  },
  {
    title: '错误消息',
    dataIndex: 'error_message',
    ellipsis: true,
    width: 260,
  },
  {
    title: '请求路径',
    dataIndex: 'request_path',
    ellipsis: true,
    width: 180,
  },
  {
    title: '状态',
    dataIndex: 'resolved',
    width: 80,
    render({ record }: { record: any }) {
      if (record.resolved) {
        return h(Tag, { color: 'green', size: 'small' }, () => '已处理')
      }
      return h(Tag, { color: 'red', size: 'small' }, () => '未处理')
    },
  },
  {
    title: '时间',
    dataIndex: 'created_at',
    width: 160,
    render({ record }: { record: any }) {
      return new Date(record.created_at).toLocaleString('zh-CN')
    },
  },
  {
    title: '操作',
    width: 120,
    fixed: 'right',
    render({ record }: { record: any }) {
      const buttons: any[] = []
      buttons.push(h(Button, { size: 'small', type: 'text', onClick: () => openDetail(record.id) }, () => '详情'))
      if (!record.resolved) {
        buttons.push(h(Button, { size: 'small', type: 'text', status: 'success', onClick: () => resolveSingle(record.id) }, () => '处理'))
      }
      return h(Space, { size: 4 }, () => buttons)
    },
  },
]

async function fetchData() {
  loading.value = true
  try {
    const params: any = {
      page: pagination.current,
      page_size: pagination.pageSize,
    }
    if (filterSource.value) params.source = filterSource.value
    if (filterErrorCode.value) params.error_code = parseInt(filterErrorCode.value)
    if (filterResolved.value) params.resolved = filterResolved.value
    if (filterKeyword.value) params.keyword = filterKeyword.value
    if (filterDateRange.value?.length === 2) {
      params.start_date = filterDateRange.value[0]
      params.end_date = filterDateRange.value[1]
    }
    const res = await request.get('/admin/error-logs', { params })
    data.value = res.data?.data?.list || []
    total.value = res.data?.data?.total || 0
  } catch {
    // error auto-shown
  } finally {
    loading.value = false
  }
}

async function fetchStats() {
  statsLoading.value = true
  try {
    const res = await request.get('/admin/error-logs/stats')
    stats.value = res.data?.data?.data
  } catch {
    // error auto-shown
  } finally {
    statsLoading.value = false
  }
}

async function openDetail(id: number) {
  detailVisible.value = true
  detailLoading.value = true
  try {
    const res = await request.get(`/admin/error-logs/${id}`)
    detailData.value = res.data?.data?.data
  } catch {
    // error auto-shown
  } finally {
    detailLoading.value = false
  }
}

async function resolveSingle(id: number) {
  try {
    await request.put(`/admin/error-logs/${id}/resolve`)
    Message.success('已标记为已处理')
    fetchData()
    fetchStats()
  } catch {
    // error auto-shown
  }
}

async function batchResolve() {
  if (!selectedKeys.value.length) {
    Message.warning('请先选择要处理的记录')
    return
  }
  try {
    await request.put('/admin/error-logs/batch-resolve', { ids: selectedKeys.value })
    Message.success(`已处理 ${selectedKeys.value.length} 条记录`)
    selectedKeys.value = []
    fetchData()
    fetchStats()
  } catch {
    // error auto-shown
  }
}

function handleFilter() {
  pagination.current = 1
  fetchData()
}

function resetFilter() {
  filterSource.value = ''
  filterErrorCode.value = ''
  filterResolved.value = ''
  filterDateRange.value = []
  filterKeyword.value = ''
  pagination.current = 1
  fetchData()
}

function handlePageChange(page: number) {
  pagination.current = page
  fetchData()
}

function handleSelection(keys: number[]) {
  selectedKeys.value = keys
}

onMounted(() => {
  fetchData()
  fetchStats()
})
</script>

<template>
  <div>
    <PageHeader title="系统错误日志" description="查看和管理系统运行中的错误记录">
      <template #actions>
        <a-button
          v-if="selectedKeys.length"
          type="primary"
          status="success"
          @click="batchResolve"
        >
          批量处理 ({{ selectedKeys.length }})
        </a-button>
      </template>
    </PageHeader>

    <!-- Stats Cards -->
    <div v-if="stats" class="stats-row">
      <a-card :bordered="false" class="stat-card">
        <a-statistic title="未处理错误" :value="stats.unresolved" :value-style="{ color: stats.unresolved > 0 ? '#f53f3f' : '#00b42a' }" />
      </a-card>
      <a-card :bordered="false" class="stat-card">
        <a-statistic title="今日新增" :value="stats.today_count" />
      </a-card>
      <a-card :bordered="false" class="stat-card">
        <div class="stat-title">来源分布（近7天）</div>
        <div class="stat-tags">
          <Tag v-for="item in stats.by_source || []" :key="item.source" :color="sourceColorMap[item.source] || 'blue'" size="small">
            {{ sourceLabelMap[item.source] || item.source }}: {{ item.count }}
          </Tag>
          <span v-if="!stats.by_source?.length" class="stat-empty">暂无数据</span>
        </div>
      </a-card>
      <a-card :bordered="false" class="stat-card">
        <div class="stat-title">错误码 Top（近7天）</div>
        <div class="stat-tags">
          <Tag v-for="item in stats.by_error_code || []" :key="item.error_code" color="orangered" size="small">
            {{ item.error_code }}: {{ item.count }}次
          </Tag>
          <span v-if="!stats.by_error_code?.length" class="stat-empty">暂无数据</span>
        </div>
      </a-card>
    </div>

    <!-- Filters -->
    <a-card :bordered="false" class="mb-4">
      <a-space wrap>
        <a-select v-model="filterSource" placeholder="错误来源" allow-clear style="width: 130px">
          <a-option value="api">API</a-option>
          <a-option value="panic">Panic</a-option>
          <a-option value="cron">定时任务</a-option>
          <a-option value="background">后台任务</a-option>
        </a-select>
        <a-input-number v-model="filterErrorCode" placeholder="错误码" allow-clear style="width: 120px" :min="1" />
        <a-select v-model="filterResolved" placeholder="处理状态" allow-clear style="width: 120px">
          <a-option value="false">未处理</a-option>
          <a-option value="true">已处理</a-option>
        </a-select>
        <a-range-picker v-model="filterDateRange" style="width: 260px" />
        <a-input v-model="filterKeyword" placeholder="搜索错误消息/路径" allow-clear style="width: 200px" />
        <a-button type="primary" @click="handleFilter">搜索</a-button>
        <a-button @click="resetFilter">重置</a-button>
      </a-space>
    </a-card>

    <!-- Table -->
    <a-card :bordered="false">
      <a-table
        :data="data"
        :columns="columns"
        :loading="loading"
        :pagination="false"
        :bordered="false"
        :stripe="true"
        row-key="id"
        :row-selection="{ type: 'checkbox', showCheckedAll: true, selectedRowKeys: selectedKeys, onSelect: handleSelection }"
      />
      <div style="display: flex; justify-content: flex-end; margin-top: 16px">
        <a-pagination
          :current="pagination.current"
          :page-size="pagination.pageSize"
          :total="total"
          show-page-size
          :page-size-options="[10, 20, 50]"
          @change="handlePageChange"
          @page-size-change="(ps: number) => { pagination.pageSize = ps; pagination.current = 1; fetchData() }"
        />
      </div>
    </a-card>

    <!-- Detail Drawer -->
    <a-drawer
      v-model:visible="detailVisible"
      title="错误详情"
      :width="640"
      :footer="false"
    >
      <a-spin :loading="detailLoading" style="width: 100%">
        <template v-if="detailData">
          <a-descriptions :column="2" bordered size="medium">
            <a-descriptions-item label="ID">{{ detailData.id }}</a-descriptions-item>
            <a-descriptions-item label="Request ID">
              <a-typography-text copyable code style="font-size: 12px">{{ detailData.request_id || '-' }}</a-typography-text>
            </a-descriptions-item>
            <a-descriptions-item label="来源">
              <Tag :color="sourceColorMap[detailData.source] || 'blue'" size="small">
                {{ sourceLabelMap[detailData.source] || detailData.source }}
              </Tag>
            </a-descriptions-item>
            <a-descriptions-item label="错误码">
              <Tag :color="detailData.error_code >= 500 ? 'red' : 'orange'" size="small">
                {{ detailData.error_code }}
              </Tag>
            </a-descriptions-item>
            <a-descriptions-item label="请求方法">{{ detailData.http_method || '-' }}</a-descriptions-item>
            <a-descriptions-item label="状态">
              <Tag :color="detailData.resolved ? 'green' : 'red'" size="small">
                {{ detailData.resolved ? '已处理' : '未处理' }}
              </Tag>
            </a-descriptions-item>
            <a-descriptions-item label="请求路径" :span="2">{{ detailData.request_path || '-' }}</a-descriptions-item>
            <a-descriptions-item label="错误消息" :span="2">
              <div style="color: #f53f3f; word-break: break-all">{{ detailData.error_message }}</div>
            </a-descriptions-item>
            <a-descriptions-item label="请求体" :span="2">
              <pre v-if="detailData.request_body" class="stack-trace">{{ detailData.request_body }}</pre>
              <span v-else>-</span>
            </a-descriptions-item>
            <a-descriptions-item label="堆栈跟踪" :span="2">
              <pre v-if="detailData.stack_trace" class="stack-trace">{{ detailData.stack_trace }}</pre>
              <span v-else>-</span>
            </a-descriptions-item>
            <a-descriptions-item label="发生时间">{{ detailData.created_at ? new Date(detailData.created_at).toLocaleString('zh-CN') : '-' }}</a-descriptions-item>
            <a-descriptions-item label="处理时间">{{ detailData.resolved_at ? new Date(detailData.resolved_at).toLocaleString('zh-CN') : '-' }}</a-descriptions-item>
          </a-descriptions>

          <div v-if="!detailData.resolved" style="margin-top: 16px; text-align: right">
            <a-button type="primary" status="success" @click="resolveSingle(detailData.id); detailVisible = false">
              标记为已处理
            </a-button>
          </div>
        </template>
      </a-spin>
    </a-drawer>
  </div>
</template>

<style scoped>
.mb-4 { margin-bottom: 16px; }

.stats-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 16px;
}
.stat-card { text-align: center; }
.stat-title {
  font-size: 12px;
  color: var(--color-text-3);
  margin-bottom: 8px;
}
.stat-tags { display: flex; flex-wrap: wrap; gap: 6px; justify-content: center; }
.stat-empty { color: var(--color-text-4); font-size: 12px; }

.stack-trace {
  background: #1e293b;
  color: #e2e8f0;
  padding: 12px;
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.6;
  max-height: 400px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}

@media (max-width: 900px) {
  .stats-row { grid-template-columns: repeat(2, 1fr); }
}
</style>
