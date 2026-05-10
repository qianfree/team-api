<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Space, Popconfirm, Message,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50],
})

const filterStatus = ref<string | null>(null)
const filterHandler = ref('')
const filterDateRange = ref<string[]>([])

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '待执行', value: 'pending' },
  { label: '执行中', value: 'running' },
  { label: '成功', value: 'succeeded' },
  { label: '失败', value: 'failed' },
  { label: '已取消', value: 'cancelled' },
]

const statusTagColor: Record<string, string | undefined> = {
  pending: 'gray',
  running: 'arcoblue',
  succeeded: 'green',
  failed: 'red',
  cancelled: 'orangered',
}

const statusTagLabel: Record<string, string> = {
  pending: '待执行',
  running: '执行中',
  succeeded: '成功',
  failed: '失败',
  cancelled: '已取消',
}

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '任务名称', dataIndex: 'name', width: 180, ellipsis: true },
  { title: 'Handler', dataIndex: 'handler', width: 220, ellipsis: true },
  {
    title: '状态',
    dataIndex: 'status',
    width: 100,
    render({ record }) {
      return h(Tag, {
        color: statusTagColor[record.status],
        size: 'small',
      }, () => statusTagLabel[record.status] || record.status)
    },
  },
  {
    title: '重试',
    dataIndex: 'retry_count',
    width: 80,
    render({ record }) {
      return `${record.retry_count}/${record.max_retries}`
    },
  },
  { title: '开始时间', dataIndex: 'started_at', width: 170 },
  { title: '完成时间', dataIndex: 'finished_at', width: 170 },
  { title: '创建时间', dataIndex: 'created_at', width: 170 },
  {
    title: '操作',
    dataIndex: 'actions',
    width: 160,
    fixed: 'right',
    render({ record }) {
      const buttons: any[] = [
        h(Button, { size: 'small', type: 'text', onClick: () => openDetail(record) }, () => '详情'),
      ]
      if (record.status === 'pending' || record.status === 'running') {
        buttons.push(
          h(Popconfirm, {
            content: '确定取消该任务？',
            onOk: () => cancelTask(record),
          }, () => h(Button, { size: 'small', type: 'text', status: 'warning' }, () => '取消')),
        )
      }
      if (record.status === 'failed') {
        buttons.push(
          h(Popconfirm, {
            content: '确定重试该任务？',
            onOk: () => retryTask(record),
          }, () => h(Button, { size: 'small', type: 'text', status: 'success' }, () => '重试')),
        )
      }
      return h(Space, { size: 0 }, () => buttons)
    },
  },
]

async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: pagination.current,
      page_size: pagination.pageSize,
    }
    if (filterStatus.value) params.status = filterStatus.value
    if (filterHandler.value) params.handler = filterHandler.value
    if (filterDateRange.value.length === 2) {
      params.start_date = filterDateRange.value[0]
      params.end_date = filterDateRange.value[1]
    }
    const res: any = await request.get('/admin/tasks', { params })
    const raw = res.data?.data
    data.value = (raw?.list || []).filter(Boolean)
    pagination.total = Number(raw?.total) || 0
  } catch {
    data.value = []
    pagination.total = 0
  } finally {
    loading.value = false
  }
}

function handleFilter() {
  pagination.current = 1
  fetchData()
}

function resetFilter() {
  filterStatus.value = null
  filterHandler.value = ''
  filterDateRange.value = []
  pagination.current = 1
  fetchData()
}

async function cancelTask(row: any) {
  try {
    await request.post(`/admin/tasks/${row.id}/cancel`)
    Message.success('任务已取消')
    fetchData()
  } catch {
    // error handled by interceptor
  }
}

async function retryTask(row: any) {
  try {
    await request.post(`/admin/tasks/${row.id}/retry`)
    Message.success('任务已重新排队')
    fetchData()
  } catch {
    // error handled by interceptor
  }
}

// === Detail Drawer ===
const showDetail = ref(false)
const detailLoading = ref(false)
const detailData = ref<any>(null)
const detailExecutions = ref<any[]>([])
const detailLogs = ref<any[]>([])

async function openDetail(row: any) {
  detailData.value = row
  detailExecutions.value = []
  detailLogs.value = []
  showDetail.value = true
  detailLoading.value = true
  try {
    const res: any = await request.get(`/admin/tasks/${row.id}`)
    const raw = res.data?.data
    if (raw) {
      detailData.value = raw.task || row
      detailExecutions.value = (raw.executions || []).filter(Boolean)
      detailLogs.value = (raw.recent_logs || []).filter(Boolean)
    }
  } catch {
    // error handled by interceptor
  } finally {
    detailLoading.value = false
  }
}

const logLevelColor: Record<string, string | undefined> = {
  info: 'arcoblue',
  warn: 'orangered',
  error: 'red',
}

const execColumns: TableColumnData[] = [
  { title: '#', dataIndex: 'execution_order', width: 50 },
  {
    title: '状态',
    dataIndex: 'status',
    width: 80,
    render({ record }) {
      return h(Tag, {
        color: statusTagColor[record.status],
        size: 'small',
      }, () => record.status)
    },
  },
  { title: '开始时间', dataIndex: 'started_at', width: 170 },
  { title: '完成时间', dataIndex: 'finished_at', width: 170 },
  {
    title: '耗时',
    dataIndex: 'duration_ms',
    width: 100,
    render({ record }) {
      if (!record.duration_ms) return '-'
      if (record.duration_ms < 1000) return `${record.duration_ms}ms`
      return `${(record.duration_ms / 1000).toFixed(1)}s`
    },
  },
  { title: '错误信息', dataIndex: 'error_message', ellipsis: true },
]

onMounted(() => {
  fetchData()
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="任务日志" description="系统异步任务的执行记录与管理" />

    <!-- Filters -->
    <ACard :bordered="false" class="mb-4">
      <ASpace wrap>
        <ASelect
          v-model="filterStatus"
          :options="statusOptions"
          placeholder="状态"
          allow-clear
          style="width: 130px"
          @change="handleFilter"
        />
        <AInput
          v-model="filterHandler"
          placeholder="搜索 Handler..."
          allow-clear
          style="width: 220px"
          @keydown.enter="handleFilter"
          @clear="handleFilter"
        />
        <ARangePicker
          v-model="filterDateRange"
          style="width: 260px"
          format="YYYY-MM-DD"
          @change="handleFilter"
        />
        <AButton type="primary" @click="handleFilter">搜索</AButton>
        <AButton @click="resetFilter">重置</AButton>
      </ASpace>
    </ACard>

    <!-- Table -->
    <ACard :bordered="false">
      <ATable
        :columns="columns"
        :data="data"
        :loading="loading"
        :scroll="{ x: 1300 }"
        :bordered="false"
        :stripe="true"
        :pagination="false"
        row-key="id"
      />
      <div class="table-footer">
        <APagination
          v-model:current="pagination.current"
          v-model:page-size="pagination.pageSize"
          :total="pagination.total"
          :page-size-options="pagination.pageSizeOptions"
          show-page-size
          @change="fetchData"
          @page-size-change="(size: number) => { pagination.pageSize = size; pagination.current = 1; fetchData() }"
        />
      </div>
    </ACard>

    <!-- Detail Drawer -->
    <ADrawer v-model:visible="showDetail" :width="620" title="任务详情">
      <ASpin :loading="detailLoading">
        <template v-if="detailData">
          <!-- Basic Info -->
          <h3 class="detail-section-title">基本信息</h3>
          <ADescriptions :column="2" bordered size="small">
            <ADescriptionsItem label="任务名称">{{ detailData.name }}</ADescriptionsItem>
            <ADescriptionsItem label="状态">
              <Tag :color="statusTagColor[detailData.status]" size="small">
                {{ statusTagLabel[detailData.status] || detailData.status }}
              </Tag>
            </ADescriptionsItem>
            <ADescriptionsItem label="Handler" :span="2">{{ detailData.handler }}</ADescriptionsItem>
            <ADescriptionsItem label="重试次数">{{ detailData.retry_count }} / {{ detailData.max_retries }}</ADescriptionsItem>
            <ADescriptionsItem label="计划时间">{{ detailData.scheduled_at || '-' }}</ADescriptionsItem>
            <ADescriptionsItem label="开始时间">{{ detailData.started_at || '-' }}</ADescriptionsItem>
            <ADescriptionsItem label="完成时间">{{ detailData.finished_at || '-' }}</ADescriptionsItem>
            <ADescriptionsItem label="创建时间">{{ detailData.created_at }}</ADescriptionsItem>
            <ADescriptionsItem label="错误信息" :span="2">
              <span v-if="detailData.error_message" style="color: var(--color-danger-6)">{{ detailData.error_message }}</span>
              <span v-else style="color: var(--color-text-4)">-</span>
            </ADescriptionsItem>
          </ADescriptions>

          <!-- Executions -->
          <h3 class="detail-section-title">执行记录</h3>
          <ATable
            v-if="detailExecutions.length > 0"
            :columns="execColumns"
            :data="detailExecutions"
            :bordered="false"
            :stripe="true"
            :pagination="false"
            size="small"
            row-key="execution_order"
          />
          <p v-else style="color: var(--color-text-4); font-size: 13px;">暂无执行记录</p>

          <!-- Logs -->
          <h3 class="detail-section-title">最近日志</h3>
          <div v-if="detailLogs.length > 0" class="log-list">
            <div v-for="log in detailLogs" :key="log.created_at + log.message" class="log-item">
              <Tag :color="logLevelColor[log.level]" size="small" class="log-level-tag">{{ log.level }}</Tag>
              <span class="log-message">{{ log.message }}</span>
              <span class="log-time">{{ log.created_at }}</span>
            </div>
          </div>
          <p v-else style="color: var(--color-text-4); font-size: 13px;">暂无日志</p>
        </template>
      </ASpin>
    </ADrawer>
  </div>
</template>

<style scoped>
.table-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--ta-border-light);
}

.detail-section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ta-text-primary);
  margin: 20px 0 10px;
}

.detail-section-title:first-child {
  margin-top: 0;
}

.log-list {
  background: var(--color-fill-1);
  border-radius: 6px;
  padding: 8px;
  max-height: 300px;
  overflow-y: auto;
}

.log-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 6px 4px;
  border-bottom: 1px solid var(--color-fill-3);
  font-size: 13px;
}

.log-item:last-child {
  border-bottom: none;
}

.log-level-tag {
  flex-shrink: 0;
  margin-top: 1px;
}

.log-message {
  flex: 1;
  color: var(--ta-text-primary);
  word-break: break-all;
}

.log-time {
  flex-shrink: 0;
  color: var(--color-text-4);
  font-size: 12px;
  white-space: nowrap;
}
</style>
