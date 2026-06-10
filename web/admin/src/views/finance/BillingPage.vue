<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Message,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

const billLoading = ref(false)
const billData = ref<any[]>([])
const billPagination = reactive({
  current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50],
})
const billFilterTenantId = ref('')

const billStatusTagColor: Record<string, string> = {
  settled: 'green', pending: 'orangered', failed: 'red',
}
const billStatusLabel: Record<string, string> = {
  settled: '已结算', pending: '待结算', failed: '失败',
}

const billColumns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 80 },
  { title: '租户', dataIndex: 'tenant_name', width: 120, ellipsis: true, tooltip: true },
  { title: '用户', dataIndex: 'user_name', width: 120, ellipsis: true, tooltip: true },
  { title: '渠道', dataIndex: 'channel_name', width: 120, ellipsis: true, tooltip: true },
  { title: '模型', dataIndex: 'model_name', width: 140, ellipsis: true },
  { title: '模式', dataIndex: 'relay_mode', width: 100 },
  { title: '输入Token', dataIndex: 'input_tokens', width: 100 },
  { title: '输出Token', dataIndex: 'output_tokens', width: 100 },
  {
    title: '总费用', dataIndex: 'total_cost', width: 100,
    render({ record }) { return `$${(record.total_cost || 0).toFixed(6)}` },
  },
  {
    title: '扣费来源', dataIndex: 'deduction_source', width: 120,
    render({ record }) {
      const tags = []
      if (record.plan_deduction > 0) tags.push(h(Tag, { size: 'small', type: 'info' }, () => '套餐抵扣'))
      if (record.wallet_deduction > 0) tags.push(h(Tag, { size: 'small' }, () => '钱包'))
      return tags.length > 0 ? h('div', { class: 'flex gap-1' }, tags) : '-'
    },
  },
  {
    title: '状态', dataIndex: 'status', width: 80,
    render({ record }) {
      return h(Tag, { color: billStatusTagColor[record.status], size: 'small' }, () => billStatusLabel[record.status] || record.status)
    },
  },
  { title: '结算时间', dataIndex: 'settled_at', width: 170 },
]

async function fetchBillingRecords() {
  billLoading.value = true
  try {
    const params: Record<string, any> = { page: billPagination.current, page_size: billPagination.pageSize }
    if (billFilterTenantId.value) params.tenant_id = parseInt(billFilterTenantId.value)
    const res: any = await request.get('/admin/billing-records', { params })
    const raw = res.data?.data
    billData.value = raw?.list || []
    billPagination.total = raw?.total || 0
  } catch {
    billData.value = []
    billPagination.total = 0
  } finally { billLoading.value = false }
}

onMounted(() => { fetchBillingRecords() })

const { exporting, exportFile } = useExport({
  url: '/admin/billing-records/export',
  getFilters: () => ({
    tenant_id: billFilterTenantId.value,
  }),
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="账单记录" description="查看计费记录">
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

    <ACard :bordered="false" class="mt-4">
      <div class="mb-4">
        <ASpace>
          <AInput v-model="billFilterTenantId" placeholder="租户ID" allow-clear style="width: 120px" @keydown.enter="fetchBillingRecords" />
          <AButton type="primary" @click="fetchBillingRecords">搜索</AButton>
        </ASpace>
      </div>
      <ATable :columns="billColumns" :data="billData" :loading="billLoading" :scroll="{ x: 1800 }" :bordered="false" :stripe="true" size="small" :pagination="false" row-key="id" />
      <div class="table-footer">
        <APagination v-model:current="billPagination.current" v-model:page-size="billPagination.pageSize" :total="billPagination.total" :page-size-options="billPagination.pageSizeOptions" show-page-size @change="fetchBillingRecords" @page-size-change="(s: number) => { billPagination.pageSize = s; billPagination.current = 1; fetchBillingRecords() }" />
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
