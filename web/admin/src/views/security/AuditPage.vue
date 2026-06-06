<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

// Helper: parse detail JSONB field
function parseDetail(record: any): any {
  if (record.detail && typeof record.detail === 'string') {
    try { return JSON.parse(record.detail) } catch { return {} }
  }
  return record.detail || {}
}

// ============================================================
// Tab 1: Operation Logs
// ============================================================
const opLoading = ref(false)
const opData = ref<any[]>([])
const opPagination = reactive({
  current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50],
})
const opFilter = reactive({
  user_id: '',
  action: '',
  date_range: [] as string[],
})

const opColumns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '用户ID', dataIndex: 'user_id', width: 80 },
  {
    title: '类型', dataIndex: 'user_type', width: 70,
    render({ record }) {
      const color = record.user_type === 'admin' ? 'arcoblue' : 'green'
      const label = record.user_type === 'admin' ? '管理员' : '租户'
      return h(Tag, { color, size: 'small' }, () => label)
    },
  },
  { title: '操作', dataIndex: 'action', width: 140, ellipsis: true },
  {
    title: '资源', dataIndex: 'resource_type', width: 100,
    render({ record }) {
      if (!record.resource_type) return h('span', { style: 'color:var(--color-text-4)' }, '-')
      const label = record.resource_type
      const id = record.resource_id ? ` #${record.resource_id}` : ''
      return h(Tag, { size: 'small' }, () => label + id)
    },
  },
  {
    title: '路径', dataIndex: 'detail', width: 220, ellipsis: true,
    render({ record }) {
      const d = parseDetail(record)
      return h('span', {}, d.path || '-')
    },
  },
  { title: 'IP', dataIndex: 'ip_address', width: 130 },
  { title: '时间', dataIndex: 'created_at', width: 170 },
  {
    title: '', dataIndex: 'actions', width: 60, fixed: 'right',
    render({ record }) {
      return h(Button, {
        size: 'mini', type: 'text',
        onClick: () => { opDetailRecord.value = record; showOpDetail.value = true },
      }, () => '详情')
    },
  },
]

async function fetchOperationLogs() {
  opLoading.value = true
  try {
    const params: Record<string, any> = {
      page: opPagination.current,
      page_size: opPagination.pageSize,
    }
    if (opFilter.user_id) params.user_id = parseInt(opFilter.user_id)
    if (opFilter.action) params.action = opFilter.action
    if (opFilter.date_range.length === 2) {
      params.start_date = opFilter.date_range[0]
      params.end_date = opFilter.date_range[1]
    }
    const res: any = await request.get('/admin/audit/operation-logs', { params })
    const raw = res.data?.data
    opData.value = (raw?.list || []).filter(Boolean)
    opPagination.total = Number(raw?.total) || 0
  } catch {
    opData.value = []
    opPagination.total = 0
  } finally {
    opLoading.value = false
  }
}

function resetOpFilter() {
  opFilter.user_id = ''
  opFilter.action = ''
  opFilter.date_range = []
  opPagination.current = 1
  fetchOperationLogs()
}

// ============================================================
// Tab 2: Sensitive Access Logs
// ============================================================
const sensLoading = ref(false)
const sensData = ref<any[]>([])
const sensPagination = reactive({
  current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50],
})
const sensFilter = reactive({
  user_id: '',
  resource_type: '',
  date_range: [] as string[],
})

const sensColumns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 100 },
  { title: '用户ID', dataIndex: 'user_id', width: 90 },
  {
    title: '用户类型', dataIndex: 'user_type', width: 100,
    render({ record }) {
      const color = record.user_type === 'admin' ? 'arcoblue' : 'green'
      const label = record.user_type === 'admin' ? '管理员' : '租户'
      return h(Tag, { color, size: 'small' }, () => label)
    },
  },
  { title: '资源类型', dataIndex: 'resource_type', width: 120 },
  { title: '操作', dataIndex: 'action', width: 140, ellipsis: true },
  { title: '访问原因', dataIndex: 'reason', width: 180, ellipsis: true },
  { title: 'IP 地址', dataIndex: 'ip_address', width: 140 },
  { title: '创建时间', dataIndex: 'created_at', width: 180 },
]

async function fetchSensitiveLogs() {
  sensLoading.value = true
  try {
    const params: Record<string, any> = {
      page: sensPagination.current,
      page_size: sensPagination.pageSize,
    }
    if (sensFilter.user_id) params.user_id = parseInt(sensFilter.user_id)
    if (sensFilter.resource_type) params.resource_type = sensFilter.resource_type
    if (sensFilter.date_range.length === 2) {
      params.start_date = sensFilter.date_range[0]
      params.end_date = sensFilter.date_range[1]
    }
    const res: any = await request.get('/admin/audit/sensitive-logs', { params })
    const raw = res.data?.data
    sensData.value = (raw?.list || []).filter(Boolean)
    sensPagination.total = Number(raw?.total) || 0
  } catch {
    sensData.value = []
    sensPagination.total = 0
  } finally {
    sensLoading.value = false
  }
}

function resetSensFilter() {
  sensFilter.user_id = ''
  sensFilter.resource_type = ''
  sensFilter.date_range = []
  sensPagination.current = 1
  fetchSensitiveLogs()
}

// ============================================================
// Active tab tracking
// ============================================================
const activeTab = ref('logs')

// Operation log detail drawer
const showOpDetail = ref(false)
const opDetailRecord = ref<any>(null)

function onTabChange(key: string | number) {
  if (key === 'sensitive' && sensData.value.length === 0) {
    fetchSensitiveLogs()
  }
}

onMounted(() => {
  fetchOperationLogs()
})

const { exporting, exportFile } = useExport({
  url: '/admin/audit/operation-logs/export',
  getFilters: () => ({
    user_id: opFilter.user_id || undefined,
    action: opFilter.action || undefined,
    start_date: opFilter.date_range[0] || undefined,
    end_date: opFilter.date_range[1] || undefined,
  }),
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="操作日志" description="查看操作日志和敏感访问日志">
      <template #actions>
        <ADropdown trigger="hover">
          <AButton :loading="exporting">导出</AButton>
          <template #content>
            <ADoption @click="exportFile('csv')">导出 CSV</ADoption>
            <ADoption @click="exportFile('xlsx')">导出 Excel</ADoption>
          </template>
        </ADropdown>
      </template>
    </PageHeader>

    <ATabs v-model:active-key="activeTab" type="line" @change="onTabChange">
      <!-- Tab 1: Operation Logs -->
      <ATabPane key="logs" title="操作日志">
        <ACard :bordered="false" class="mt-4">
          <div class="mb-4">
            <ASpace wrap>
              <AInput
                v-model="opFilter.user_id"
                placeholder="用户ID"
                allow-clear
                style="width: 120px"
              />
              <AInput
                v-model="opFilter.action"
                placeholder="操作类型"
                allow-clear
                style="width: 140px"
              />
              <ARangePicker
                v-model="opFilter.date_range"
                style="width: 260px"
                format="YYYY-MM-DD"
              />
              <AButton type="primary" @click="() => { opPagination.current = 1; fetchOperationLogs() }">搜索</AButton>
              <AButton @click="resetOpFilter">重置</AButton>
            </ASpace>
          </div>

          <ATable
            :columns="opColumns"
            :data="opData"
            :loading="opLoading"
            :scroll="{ x: 1100 }"
            :bordered="false"
            :stripe="true"
            size="small"
            :pagination="false"
            row-key="id"
          />
          <div class="table-footer">
            <APagination
              v-model:current="opPagination.current"
              v-model:page-size="opPagination.pageSize"
              :total="opPagination.total"
              :page-size-options="opPagination.pageSizeOptions"
              show-page-size
              @change="fetchOperationLogs"
              @page-size-change="(s: number) => { opPagination.pageSize = s; opPagination.current = 1; fetchOperationLogs() }"
            />
          </div>
        </ACard>
      </ATabPane>

      <!-- Tab 2: Sensitive Access Logs -->
      <ATabPane key="sensitive" title="敏感访问日志">
        <ACard :bordered="false" class="mt-4">
          <div class="mb-4">
            <ASpace wrap>
              <AInput
                v-model="sensFilter.user_id"
                placeholder="用户ID"
                allow-clear
                style="width: 120px"
              />
              <AInput
                v-model="sensFilter.resource_type"
                placeholder="资源类型"
                allow-clear
                style="width: 140px"
              />
              <ARangePicker
                v-model="sensFilter.date_range"
                style="width: 260px"
                format="YYYY-MM-DD"
              />
              <AButton type="primary" @click="() => { sensPagination.current = 1; fetchSensitiveLogs() }">搜索</AButton>
              <AButton @click="resetSensFilter">重置</AButton>
            </ASpace>
          </div>

          <ATable
            :columns="sensColumns"
            :data="sensData"
            :loading="sensLoading"
            :scroll="{ x: 1200 }"
            :bordered="false"
            :stripe="true"
            size="small"
            :pagination="false"
            row-key="id"
          />
          <div class="table-footer">
            <APagination
              v-model:current="sensPagination.current"
              v-model:page-size="sensPagination.pageSize"
              :total="sensPagination.total"
              :page-size-options="sensPagination.pageSizeOptions"
              show-page-size
              @change="fetchSensitiveLogs"
              @page-size-change="(s: number) => { sensPagination.pageSize = s; sensPagination.current = 1; fetchSensitiveLogs() }"
            />
          </div>
        </ACard>
      </ATabPane>
    </ATabs>

    <!-- Operation Log Detail Drawer -->
    <ADrawer v-model:visible="showOpDetail" :width="500" title="操作详情">
      <template v-if="opDetailRecord">
        <ADescriptions :column="1" bordered size="medium">
          <ADescriptionsItem label="操作">{{ opDetailRecord.action }}</ADescriptionsItem>
          <ADescriptionsItem label="资源">{{ opDetailRecord.resource_type || '-' }} {{ opDetailRecord.resource_id ? '#' + opDetailRecord.resource_id : '' }}</ADescriptionsItem>
          <ADescriptionsItem label="用户">{{ opDetailRecord.user_type }} #{{ opDetailRecord.user_id }}</ADescriptionsItem>
          <ADescriptionsItem label="IP">{{ opDetailRecord.ip_address }}</ADescriptionsItem>
          <ADescriptionsItem label="时间">{{ opDetailRecord.created_at }}</ADescriptionsItem>
        </ADescriptions>
        <div v-if="parseDetail(opDetailRecord).path" class="mt-4">
          <h4 style="margin-bottom:8px;color:var(--ta-text-primary)">请求信息</h4>
          <ADescriptions :column="1" bordered size="small">
            <ADescriptionsItem label="路径">{{ parseDetail(opDetailRecord).path }}</ADescriptionsItem>
            <ADescriptionsItem label="方法">{{ parseDetail(opDetailRecord).method }}</ADescriptionsItem>
            <ADescriptionsItem label="状态码">{{ parseDetail(opDetailRecord).status_code }}</ADescriptionsItem>
            <ADescriptionsItem label="User-Agent">{{ parseDetail(opDetailRecord).user_agent }}</ADescriptionsItem>
            <ADescriptionsItem v-if="parseDetail(opDetailRecord).request_id" label="Request ID">{{ parseDetail(opDetailRecord).request_id }}</ADescriptionsItem>
          </ADescriptions>
        </div>
        <div v-if="opDetailRecord.changes_json" class="mt-4">
          <h4 style="margin-bottom:8px;color:var(--ta-text-primary)">变更内容</h4>
          <pre style="background:var(--color-fill-2);padding:12px;border-radius:6px;font-size:12px;max-height:300px;overflow:auto;white-space:pre-wrap;word-break:break-all">{{ typeof opDetailRecord.changes_json === 'string' ? opDetailRecord.changes_json : JSON.stringify(opDetailRecord.changes_json, null, 2) }}</pre>
        </div>
        <div v-else-if="parseDetail(opDetailRecord).request_body" class="mt-4">
          <h4 style="margin-bottom:8px;color:var(--ta-text-primary)">请求体</h4>
          <pre style="background:var(--color-fill-2);padding:12px;border-radius:6px;font-size:12px;max-height:300px;overflow:auto;white-space:pre-wrap;word-break:break-all">{{ parseDetail(opDetailRecord).request_body }}</pre>
        </div>
      </template>
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
</style>
