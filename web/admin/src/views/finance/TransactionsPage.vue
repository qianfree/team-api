<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import { Tag } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({
  current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50],
})

const filterTenantId = ref('')
const filterType = ref<string | undefined>(undefined)
const filterUsername = ref('')
const filterStartDate = ref('')
const filterEndDate = ref('')

const typeOptions = [
  { label: '全部', value: '' },
  { label: '消费', value: 'consume' },
  { label: '充值', value: 'recharge' },
  { label: '调整', value: 'adjust' },
]

const typeTagColor: Record<string, string> = {
  consume: 'orangered', recharge: 'green', adjust: 'arcoblue',
}
const typeLabel: Record<string, string> = {
  consume: '消费', recharge: '充值', adjust: '调整',
}

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 80 },
  { title: '租户', dataIndex: 'tenant_name', width: 120, ellipsis: true, tooltip: true },
  { title: '用户', dataIndex: 'username', width: 120, ellipsis: true, tooltip: true },
  {
    title: '类型', dataIndex: 'type', width: 80,
    render({ record }) {
      return h(Tag, { color: typeTagColor[record.type], size: 'small' }, () => typeLabel[record.type] || record.type)
    },
  },
  {
    title: '金额', dataIndex: 'amount', width: 130,
    render({ record }) {
      const amount = record.amount || 0
      const color = amount > 0 ? 'rgb(var(--green-6))' : amount < 0 ? 'rgb(var(--red-6))' : undefined
      const prefix = amount > 0 ? '+' : ''
      return h('span', { style: { color, fontWeight: 500, fontFamily: 'monospace' } }, `${prefix}$${Math.abs(amount).toFixed(6)}`)
    },
  },
  {
    title: '变动后余额', dataIndex: 'balance_after', width: 130,
    render({ record }) { return `$${(record.balance_after || 0).toFixed(6)}` },
  },
  { title: '描述', dataIndex: 'description', width: 180, ellipsis: true, tooltip: true },
  { title: '模型', dataIndex: 'model_name', width: 140, ellipsis: true, tooltip: true },
  { title: '时间', dataIndex: 'created_at', width: 170 },
]

async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, any> = { page: pagination.current, page_size: pagination.pageSize }
    if (filterTenantId.value) params.tenant_id = parseInt(filterTenantId.value)
    if (filterType.value) params.type = filterType.value
    if (filterUsername.value) params.username = filterUsername.value
    if (filterStartDate.value) params.start_date = filterStartDate.value
    if (filterEndDate.value) params.end_date = filterEndDate.value
    const res: any = await request.get('/admin/transactions', { params })
    const raw = res.data?.data
    data.value = raw?.list || []
    pagination.total = raw?.total || 0
  } catch {
    data.value = []
    pagination.total = 0
  } finally { loading.value = false }
}

function resetAndFetch() {
  pagination.current = 1
  fetchData()
}

onMounted(fetchData)
</script>

<template>
  <div class="page-table">
    <PageHeader title="交易流水" description="查看所有租户的钱包交易记录" />

    <ACard :bordered="false" class="mt-4">
      <div class="mb-4">
        <ASpace wrap>
          <AInput v-model="filterTenantId" placeholder="租户ID" allow-clear style="width: 120px" @keydown.enter="resetAndFetch" />
          <ASelect v-model="filterType" :options="typeOptions" style="width: 120px" allow-clear @change="resetAndFetch" />
          <AInput v-model="filterUsername" placeholder="用户名" allow-clear style="width: 120px" @keydown.enter="resetAndFetch" />
          <ADatePicker v-model="filterStartDate" placeholder="开始日期" style="width: 150px" @change="resetAndFetch" />
          <ADatePicker v-model="filterEndDate" placeholder="结束日期" style="width: 150px" @change="resetAndFetch" />
          <AButton type="primary" @click="resetAndFetch">搜索</AButton>
        </ASpace>
      </div>
      <ATable :columns="columns" :data="data" :loading="loading" :scroll="{ x: 1200 }" :bordered="false" :stripe="true" size="small" :pagination="false" row-key="id" />
      <div class="table-footer">
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchData" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchData() }" />
      </div>
    </ACard>
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
