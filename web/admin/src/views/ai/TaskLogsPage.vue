<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import { useRoute } from 'vue-router'
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
const filterPlatform = ref<string | null>(null)
const filterTaskId = ref('')

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '未开始', value: 'NOT_START' },
  { label: '已提交', value: 'SUBMITTED' },
  { label: '进行中', value: 'IN_PROGRESS' },
  { label: '成功', value: 'SUCCESS' },
  { label: '失败', value: 'FAILURE' },
]

const platformOptions = [
  { label: '全部平台', value: '' },
  { label: 'Sora', value: 'sora' },
  { label: 'Kling', value: 'kling' },
  { label: 'Midjourney', value: 'midjourney' },
  { label: 'Suno', value: 'suno' },
]

const statusTagColor: Record<string, string | undefined> = {
  NOT_START: 'gray',
  SUBMITTED: 'arcoblue',
  IN_PROGRESS: 'arcoblue',
  SUCCESS: 'green',
  FAILURE: 'red',
}

const statusTagLabel: Record<string, string> = {
  NOT_START: '未开始',
  SUBMITTED: '已提交',
  IN_PROGRESS: '进行中',
  SUCCESS: '成功',
  FAILURE: '失败',
}

const platformLabel: Record<string, string> = {
  sora: 'Sora',
  kling: 'Kling',
  midjourney: 'Midjourney',
  suno: 'Suno',
}

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '任务ID', dataIndex: 'public_task_id', width: 160, ellipsis: true, tooltip: true },
  {
    title: '平台',
    dataIndex: 'platform',
    width: 110,
    render({ record }) {
      return platformLabel[record.platform] || record.platform
    },
  },
  {
    title: '动作',
    dataIndex: 'action',
    width: 120,
    ellipsis: true,
    tooltip: true,
  },
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
    title: '进度',
    dataIndex: 'progress',
    width: 80,
  },
  {
    title: '模型',
    dataIndex: 'model_name',
    width: 150,
    ellipsis: true,
    tooltip: true,
  },
  {
    title: '费用',
    dataIndex: 'actual_cost',
    width: 100,
    render({ record }) {
      if (record.billing_settled && record.actual_cost > 0) {
        return `$${record.actual_cost.toFixed(6)}`
      }
      if (record.pre_deduct_amount > 0) {
        return `$${record.pre_deduct_amount.toFixed(6)} (预扣)`
      }
      return '-'
    },
  },
  { title: '提交时间', dataIndex: 'submit_time', width: 170 },
  { title: '完成时间', dataIndex: 'finish_time', width: 170 },
  {
    title: '操作',
    dataIndex: 'actions',
    width: 120,
    fixed: 'right',
    render({ record }) {
      const buttons: any[] = [
        h(Button, { size: 'small', type: 'text', onClick: () => openDetail(record) }, () => '详情'),
      ]
      if (record.status === 'NOT_START' || record.status === 'SUBMITTED' || record.status === 'IN_PROGRESS') {
        buttons.push(
          h(Popconfirm, {
            content: '确定取消该任务？',
            onOk: () => cancelTask(record),
          }, () => h(Button, { size: 'small', type: 'text', status: 'warning' }, () => '取消')),
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
    if (filterPlatform.value) params.platform = filterPlatform.value
    if (filterTaskId.value) params.public_task_id = filterTaskId.value
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
  filterPlatform.value = null
  filterTaskId.value = ''
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

// === Detail Drawer ===
const showDetail = ref(false)
const detailLoading = ref(false)
const detailData = ref<any>(null)

async function openDetail(row: any) {
  detailData.value = row
  showDetail.value = true
  detailLoading.value = true
  try {
    const res: any = await request.get(`/admin/tasks/${row.id}`)
    const raw = res.data?.data
    if (raw?.task) {
      detailData.value = raw.task
    }
  } catch {
    // error handled by interceptor
  } finally {
    detailLoading.value = false
  }
}

onMounted(() => {
  const route = useRoute()
  if (route.query.public_task_id) {
    filterTaskId.value = String(route.query.public_task_id)
  }
  fetchData()
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="任务日志" description="大模型异步生成任务（视频/图片/音乐）的执行记录与管理" />

    <!-- Filters -->
    <ACard :bordered="false" class="mb-4">
      <ASpace wrap>
        <AInput
          v-model="filterTaskId"
          placeholder="任务ID"
          allow-clear
          style="width: 200px"
          @keydown.enter="handleFilter"
        />
        <ASelect
          v-model="filterStatus"
          :options="statusOptions"
          placeholder="状态"
          allow-clear
          style="width: 130px"
          @change="handleFilter"
        />
        <ASelect
          v-model="filterPlatform"
          :options="platformOptions"
          placeholder="平台"
          allow-clear
          style="width: 130px"
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
        :scroll="{ x: 1400 }"
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
          <h3 class="detail-section-title">基本信息</h3>
          <ADescriptions :column="2" bordered size="small">
            <ADescriptionsItem label="任务ID">{{ detailData.public_task_id }}</ADescriptionsItem>
            <ADescriptionsItem label="状态">
              <Tag :color="statusTagColor[detailData.status]" size="small">
                {{ statusTagLabel[detailData.status] || detailData.status }}
              </Tag>
            </ADescriptionsItem>
            <ADescriptionsItem label="平台">{{ platformLabel[detailData.platform] || detailData.platform }}</ADescriptionsItem>
            <ADescriptionsItem label="动作">{{ detailData.action }}</ADescriptionsItem>
            <ADescriptionsItem label="模型">{{ detailData.model_name }}</ADescriptionsItem>
            <ADescriptionsItem label="进度">{{ detailData.progress }}</ADescriptionsItem>
            <ADescriptionsItem label="预扣金额">
              <span v-if="detailData.pre_deduct_amount > 0">${{ detailData.pre_deduct_amount.toFixed(4) }}</span>
              <span v-else>-</span>
            </ADescriptionsItem>
            <ADescriptionsItem label="实际费用">
              <span v-if="detailData.billing_settled">${{ detailData.actual_cost.toFixed(4) }}</span>
              <span v-else>未结算</span>
            </ADescriptionsItem>
            <ADescriptionsItem label="提交时间">{{ detailData.submit_time || '-' }}</ADescriptionsItem>
            <ADescriptionsItem label="完成时间">{{ detailData.finish_time || '-' }}</ADescriptionsItem>
            <ADescriptionsItem label="创建时间">{{ detailData.created_at }}</ADescriptionsItem>
            <ADescriptionsItem label="租户ID">{{ detailData.tenant_id }}</ADescriptionsItem>
            <ADescriptionsItem label="用户ID">{{ detailData.user_id }}</ADescriptionsItem>
            <ADescriptionsItem v-if="detailData.fail_reason" label="失败原因" :span="2">
              <span style="color: var(--color-danger-6)">{{ detailData.fail_reason }}</span>
            </ADescriptionsItem>
          </ADescriptions>

          <!-- Result -->
          <template v-if="detailData.result_url">
            <h3 class="detail-section-title">结果</h3>
            <div class="result-preview">
              <img
                v-if="detailData.result_url && /\.(jpg|jpeg|png|gif|webp)(\?|$)/i.test(detailData.result_url)"
                :src="detailData.result_url"
                alt="任务结果"
                style="max-width: 100%; border-radius: 6px;"
              />
              <a v-else :href="detailData.result_url" target="_blank" class="result-link">
                {{ detailData.result_url }}
              </a>
            </div>
          </template>
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

.result-preview {
  background: var(--color-fill-1);
  border-radius: 6px;
  padding: 12px;
}

.result-link {
  color: rgb(var(--arcoblue-6));
  word-break: break-all;
}
</style>
