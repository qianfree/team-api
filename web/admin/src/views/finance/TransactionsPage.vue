<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import { Tag } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import TableStats from '@/components/TableStats.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import request from '@/utils/request'
import { useDayShortcuts } from '@/composables/useDateRange'
import { useTenantOptions } from '@/composables/useTenantOptions'
import { formatBilling } from '@/composables/useCurrency'

const dayShortcuts = useDayShortcuts()
const { tenantOptions, fetchTenantOptions, handleTenantSearch } = useTenantOptions()

const loading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({
  current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50],
})

const filterTenantId = ref<number | undefined>(undefined)
const filterType = ref<string | undefined>(undefined)
// 时间范围（YYYY-MM-DD 闭区间），缺省查全部
const dateRange = ref<string[] | undefined>(undefined)

const typeOptions = [
  { label: '全部', value: '' },
  { label: '消费', value: 'consume' },
  { label: '充值', value: 'recharge' },
  { label: '兑换码', value: 'redemption' },
  { label: '退款', value: 'refund' },
  { label: '调整', value: 'adjust' },
]

const typeTagColor: Record<string, string> = {
  consume: 'orangered', recharge: 'green', redemption: 'green', refund: 'cyan', adjust: 'arcoblue',
}
const typeLabel: Record<string, string> = {
  consume: '消费', recharge: '充值', redemption: '兑换码', refund: '退款', adjust: '调整',
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
      // bil 层本位币金额：showSign 自动带 +/- 符号（同时修复负数缺失负号的问题）
      return h('span', { style: { color, fontWeight: 500, fontFamily: 'monospace' } }, formatBilling(amount, 6, true))
    },
  },
  {
    title: '变动后余额', dataIndex: 'balance_after', width: 130,
    render({ record }) { return formatBilling(record.balance_after || 0, 6) },
  },
  { title: '描述', dataIndex: 'description', width: 180, ellipsis: true, tooltip: true },
  { title: '模型', dataIndex: 'model_name', width: 140, ellipsis: true, tooltip: true },
  { title: '时间', dataIndex: 'created_at', width: 170 },
]

async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, any> = { page: pagination.current, page_size: pagination.pageSize }
    if (filterTenantId.value) params.tenant_id = filterTenantId.value
    if (filterType.value) params.type = filterType.value
    if (dateRange.value?.[0]) params.start_date = dateRange.value[0]
    if (dateRange.value?.[1]) params.end_date = dateRange.value[1]
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

function handleReset() {
  filterTenantId.value = undefined
  filterType.value = undefined
  dateRange.value = undefined
  resetAndFetch()
}

onMounted(() => {
  fetchTenantOptions()
  fetchData()
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="交易流水" description="查看所有租户的钱包交易记录" />

    <ACard :bordered="false" class="mb-4">
      <div class="filter-bar">
        <!-- 时间范围恒为首个筛选条件 -->
        <ARangePicker
          v-model="dateRange"
          format="YYYY-MM-DD"
          :shortcuts="dayShortcuts"
          shortcuts-position="bottom"
          style="width: 260px"
          allow-clear
          @change="resetAndFetch"
        />
        <ASelect
          v-model="filterTenantId"
          :options="tenantOptions"
          placeholder="租户"
          allow-search
          allow-clear
          :filter-option="false"
          style="width: 200px"
          @search="handleTenantSearch"
          @change="resetAndFetch"
          @clear="resetAndFetch"
        />
        <ASelect v-model="filterType" :options="typeOptions" placeholder="类型" style="width: 120px" allow-clear @change="resetAndFetch" />
        <div class="filter-actions">
          <AButton type="primary" @click="resetAndFetch">搜索</AButton>
          <AButton @click="handleReset">重置</AButton>
        </div>
      </div>
    </ACard>

    <ACard :bordered="false">
      <ResponsiveTable
        :columns="columns"
        :data="data"
        :loading="loading"
        :scroll="{ x: 1200 }"
        :bordered="false"
        :stripe="true"
        size="small"
        row-key="id"
        card-title-key="username"
        card-subtitle-key="tenant_name"
        card-badge-key="type"
        :card-fields="[
          { key: 'amount' },
          { key: 'balance_after' },
          { key: 'created_at' },
          { key: 'description', full: true },
          { key: 'model_name' },
        ]"
      />
      <div class="table-footer">
        <TableStats :total="pagination.total" />
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchData" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchData() }" />
      </div>
    </ACard>
  </div>
</template>

<style scoped>
/* 筛选栏：条件与按钮同流排布——空间足够时同行显示；不足时条件自动换行，按钮组始终落在末行右侧（右下角） */
.filter-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
/* 按钮组：margin-left:auto 在所在行内靠右；换行独占末行时仍靠右，形成右下角对齐 */
.filter-actions {
  margin-left: auto;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
/* 移动端：筛选条件各占整行，按钮组落到最后一行并靠右 */
@media (max-width: 768px) {
  .filter-bar > *:not(.filter-actions) {
    flex: 1 1 100%;
    width: 100% !important;
  }
}
.table-footer {
  display: flex;
  justify-content: flex-end;
  padding-top: 16px;
}
</style>
