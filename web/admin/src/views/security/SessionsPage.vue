<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Popconfirm, Message,
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
})

const searchForm = reactive({
  username: '',
  ip_address: '',
})

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 80 },
  {
    title: '用户',
    dataIndex: 'username',
    width: 160,
    render({ record }) {
      const name = record.display_name || record.username
      const suffix = record.display_name ? ` (${record.username})` : ''
      return h('span', {}, [
        name,
        suffix ? h('span', { style: 'color: var(--ta-text-tertiary)' }, suffix) : null,
      ])
    },
  },
  { title: 'IP 地址', dataIndex: 'ip_address', width: 140 },
  {
    title: '设备/浏览器',
    dataIndex: 'device_info',
    ellipsis: true,
    render({ record }) {
      try {
        const info = JSON.parse(record.device_info || '{}')
        return info.user_agent || record.device_info || '未知'
      } catch {
        return record.device_info || '未知'
      }
    },
  },
  {
    title: '状态',
    dataIndex: 'is_current',
    width: 100,
    render({ record }) {
      if (record.is_current) {
        return h(Tag, { color: 'green', size: 'small' }, () => '当前')
      }
      return h(Tag, { size: 'small' }, () => '活跃')
    },
  },
  { title: '创建时间', dataIndex: 'created_at', width: 180 },
  { title: '过期时间', dataIndex: 'expires_at', width: 180 },
  {
    title: '操作',
    dataIndex: 'actions',
    width: 100,
    fixed: 'right',
    render({ record }) {
      if (record.is_current) return null
      return h(Popconfirm, {
        content: '确定撤销该会话？',
        onOk: () => revokeSession(record.id),
      }, () => h(Button, { size: 'small', status: 'danger' }, () => '撤销'))
    },
  },
]

async function fetchSessions() {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: pagination.current,
      page_size: pagination.pageSize,
    }
    if (searchForm.username) params.username = searchForm.username
    if (searchForm.ip_address) params.ip_address = searchForm.ip_address

    const res: any = await request.get('/admin/auth/sessions', { params })
    if (res.data?.code === 0) {
      data.value = res.data.data.list || []
      pagination.total = res.data.data.total || 0
    }
  } catch {
    data.value = []
  } finally {
    loading.value = false
  }
}

async function revokeSession(sessionId: number) {
  try {
    await request.delete(`/admin/auth/sessions/${sessionId}`)
    Message.success('会话已撤销')
    fetchSessions()
  } catch {
    // error toast shown by interceptor
  }
}

function onSearch() {
  pagination.current = 1
  fetchSessions()
}

function onReset() {
  searchForm.username = ''
  searchForm.ip_address = ''
  pagination.current = 1
  fetchSessions()
}

function onPageChange(page: number) {
  pagination.current = page
  fetchSessions()
}

onMounted(() => {
  fetchSessions()
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="会话管理" description="查看和管理所有管理员的活跃会话">
      <template #actions>
        <AButton size="small" @click="fetchSessions">刷新</AButton>
      </template>
    </PageHeader>

    <ACard :bordered="false" style="margin-bottom: 16px">
      <AForm :model="searchForm" layout="inline" @submit="onSearch">
        <AFormItem label="用户名">
          <AInput v-model="searchForm.username" placeholder="搜索用户名" allow-clear style="width: 160px" />
        </AFormItem>
        <AFormItem label="IP 地址">
          <AInput v-model="searchForm.ip_address" placeholder="搜索 IP" allow-clear style="width: 160px" />
        </AFormItem>
        <AFormItem>
          <ASpace>
            <AButton type="primary" html-type="submit">搜索</AButton>
            <AButton @click="onReset">重置</AButton>
          </ASpace>
        </AFormItem>
      </AForm>
    </ACard>

    <ACard :bordered="false">
      <ATable
        :columns="columns"
        :data="data"
        :loading="loading"
        :bordered="false"
        :stripe="true"
        row-key="id"
        :row-class="(record: any) => record?.is_current ? 'current-session-row' : ''"
        :pagination="{
          current: pagination.current,
          pageSize: pagination.pageSize,
          total: pagination.total,
          showTotal: true,
          onChange: onPageChange,
        }"
      />
    </ACard>
  </div>
</template>

<style scoped>
:deep(.current-session-row td) {
  background-color: rgba(16, 185, 129, 0.04);
}
</style>
