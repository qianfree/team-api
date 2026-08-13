<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, h } from 'vue'
import { Message, Modal, Tag, Button, Space } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import TableStats from '@/components/TableStats.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import request from '@/utils/request'
import { hasPermission } from '@/utils/permission'

const loading = ref(false)
const data = ref<any[]>([])
const total = ref(0)
const pagination = reactive({ current: 1, pageSize: 20 })

const filterStatus = ref('')
const filterLevel = ref('')

const showResolveModal = ref(false)
const resolveEventId = ref<number | null>(null)
const resolveNotes = ref('')
const resolveLoading = ref(false)

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 60 },
  { title: '规则名称', dataIndex: 'rule_name', width: 150 },
  { title: '指标', dataIndex: 'metric_type', width: 150 },
  {
    title: '级别',
    dataIndex: 'level',
    width: 80,
    render({ record }: { record: any }) {
      const colorMap: any = { info: 'arcoblue', warning: 'orange', critical: 'red' }
      const labelMap: any = { info: '信息', warning: '警告', critical: '严重' }
      return h(Tag, { color: colorMap[record.level] || 'blue', size: 'small' }, () => labelMap[record.level] || record.level)
    },
  },
  {
    title: '状态',
    dataIndex: 'status',
    width: 90,
    render({ record }: { record: any }) {
      const colorMap: any = { firing: 'red', acknowledged: 'orange', resolved: 'green' }
      const labelMap: any = { firing: '触发中', acknowledged: '已确认', resolved: '已恢复' }
      return h(Tag, { color: colorMap[record.status] || 'blue', size: 'small' }, () => labelMap[record.status] || record.status)
    },
  },
  {
    title: '触发值',
    dataIndex: 'trigger_value',
    width: 100,
    render({ record }: { record: any }) {
      return record.trigger_value != null ? Number(record.trigger_value).toFixed(2) : '-'
    },
  },
  {
    title: '阈值',
    dataIndex: 'threshold_value',
    width: 100,
    render({ record }: { record: any }) {
      return record.threshold_value != null ? Number(record.threshold_value).toFixed(2) : '-'
    },
  },
  {
    title: '触发消息',
    dataIndex: 'trigger_message',
    ellipsis: true,
    width: 200,
  },
  {
    title: '触发时间',
    dataIndex: 'created_at',
    width: 160,
    render({ record }: { record: any }) {
      return new Date(record.created_at).toLocaleString('zh-CN')
    },
  },
  {
    title: '操作',
    width: 160,
    fixed: 'right',
    render({ record }: { record: any }) {
      const buttons: any[] = []
      if (record.status === 'firing') {
        buttons.push(h(Button, { size: 'small', type: 'primary', onClick: () => acknowledgeEvent(record.id) }, () => '确认'))
      }
      if (record.status === 'firing' || record.status === 'acknowledged') {
        buttons.push(h(Button, { size: 'small', status: 'success', onClick: () => openResolve(record.id) }, () => '解决'))
      }
      if (record.status === 'resolved' && record.resolve_notes) {
        buttons.push(h(Button, { size: 'small', onClick: () => Message.info(record.resolve_notes) }, () => '备注'))
      }
      return h(Space, { size: 4 }, () => buttons)
    },
  },
]

async function fetchData() {
  loading.value = true
  try {
    const params: any = { page: pagination.current, page_size: pagination.pageSize }
    if (filterStatus.value) params.status = filterStatus.value
    if (filterLevel.value) params.level = filterLevel.value
    const res = await request.get('/admin/alert/events', { params })
    data.value = res.data?.data?.list || []
    total.value = res.data?.data?.total || 0
  } catch {
    // error auto-shown by Axios interceptor
  } finally {
    loading.value = false
  }
}

async function acknowledgeEvent(id: number) {
  try {
    await request.put(`/admin/alert/events/${id}/acknowledge`)
    Message.success('已确认')
    fetchData()
  } catch {
    // error auto-shown by Axios interceptor
  }
}

function openResolve(id: number) {
  resolveEventId.value = id
  resolveNotes.value = ''
  showResolveModal.value = true
}

async function handleResolve() {
  if (!resolveEventId.value) return
  resolveLoading.value = true
  try {
    await request.put(`/admin/alert/events/${resolveEventId.value}/resolve`, { notes: resolveNotes.value })
    Message.success('已解决')
    showResolveModal.value = false
    fetchData()
  } catch {
    return false
  } finally {
    resolveLoading.value = false
  }
}

function handlePageChange(page: number) {
  pagination.current = page
  fetchData()
}

// 清空全部告警记录（硬删除）：异常频发时事件表会迅速膨胀拖累数据库，需二次确认。
function clearEvents() {
  Modal.confirm({
    title: '清空全部告警记录',
    content: '将硬删除全部告警记录（含已恢复的历史告警），此操作不可恢复，确定继续？',
    okText: '清空',
    okButtonProps: { status: 'danger' },
    cancelText: '取消',
    onOk: async () => {
      try {
        await request.delete('/admin/alert/events/clear')
        Message.success('告警记录已清空')
        fetchData()
      } catch {
        // error auto-shown by Axios interceptor
      }
    },
  })
}

onMounted(() => {
  fetchData()
})

onUnmounted(() => {
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="告警记录" description="查看和管理告警事件">
      <template #actions>
        <a-button v-if="hasPermission('monitor:edit')" status="danger" @click="clearEvents">
          清空记录
        </a-button>
      </template>
    </PageHeader>

    <a-card :bordered="false" class="mb-4">
      <a-space>
        <a-select v-model="filterStatus" placeholder="状态" allow-clear style="width: 120px">
          <a-option value="firing">触发中</a-option>
          <a-option value="acknowledged">已确认</a-option>
          <a-option value="resolved">已恢复</a-option>
        </a-select>
        <a-select v-model="filterLevel" placeholder="级别" allow-clear style="width: 120px">
          <a-option value="info">信息</a-option>
          <a-option value="warning">警告</a-option>
          <a-option value="critical">严重</a-option>
        </a-select>
        <a-button @click="fetchData">搜索</a-button>
      </a-space>
    </a-card>

    <a-card :bordered="false">
      <ResponsiveTable
        :data="data"
        :columns="columns"
        :loading="loading"
        row-key="id"
        card-title-key="rule_name"
        card-subtitle-key="metric_type"
        card-badge-key="status"
        :card-fields="[
          'level',
          'trigger_value',
          'threshold_value',
          { key: 'trigger_message', full: true },
          'created_at',
        ]"
      />
      <div class="table-footer">
        <TableStats :total="total" />
        <a-pagination :current="pagination.current" :page-size="pagination.pageSize" :total="total" @change="handlePageChange" />
      </div>
    </a-card>

    <a-modal v-model:visible="showResolveModal" title="解决告警" :on-before-ok="handleResolve" :ok-loading="resolveLoading">
      <a-form :model="{ notes: resolveNotes }" layout="vertical">
        <a-form-item label="处理备注">
          <a-textarea v-model="resolveNotes" placeholder="请输入处理说明..." :auto-size="{ minRows: 3, maxRows: 6 }" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<style scoped>
.mb-4 { margin-bottom: 16px; }
</style>
