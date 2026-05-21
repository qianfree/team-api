<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Message,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

// === Billing Records ===
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

// === Wallets ===
const walletLoading = ref(false)
const walletData = ref<any[]>([])
const walletPagination = reactive({
  current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50],
})

const showAdjustModal = ref(false)
const adjustForm = reactive({ tenant_id: 0, amount: 0, description: '' })
const adjustLoading = ref(false)

const walletColumns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '租户ID', dataIndex: 'tenant_id', width: 90 },
  {
    title: '余额', dataIndex: 'balance', width: 120,
    render({ record }) { return h('span', { style: { fontWeight: 600 } }, `$${(record.balance || 0).toFixed(2)}`) },
  },
  {
    title: '冻结余额', dataIndex: 'frozen_balance', width: 110,
    render({ record }) { return `$${(record.frozen_balance || 0).toFixed(2)}` },
  },
  {
    title: '预警阈值', dataIndex: 'warning_threshold', width: 110,
    render({ record }) {
      if (record.warning_threshold === null || record.warning_threshold === undefined) return 'N/A'
      return `$${record.warning_threshold.toFixed(2)}`
    },
  },
  { title: '货币', dataIndex: 'currency', width: 70 },
  { title: '创建时间', dataIndex: 'created_at', width: 170 },
  { title: '更新时间', dataIndex: 'updated_at', width: 170 },
  {
    title: '操作', dataIndex: 'actions', width: 100, fixed: 'right',
    render({ record }) {
      return h(Button, { size: 'small', type: 'primary', onClick: () => openAdjust(record) }, () => '调整余额')
    },
  },
]

async function fetchWallets() {
  walletLoading.value = true
  try {
    const res: any = await request.get('/admin/wallets', { params: { page: walletPagination.current, page_size: walletPagination.pageSize } })
    const walletRaw = res.data?.data
    walletData.value = walletRaw?.list || []
    walletPagination.total = walletRaw?.total || 0
  } catch {
    walletData.value = []
    walletPagination.total = 0
  } finally { walletLoading.value = false }
}

function openAdjust(row: any) {
  adjustForm.tenant_id = row.tenant_id
  adjustForm.amount = 0
  adjustForm.description = ''
  showAdjustModal.value = true
}

async function handleAdjust(done: () => void) {
  if (adjustForm.amount === 0) { Message.warning('金额不能为零'); return }
  adjustLoading.value = true
  try {
    await request.post(`/admin/wallets/${adjustForm.tenant_id}/adjust`, { amount: adjustForm.amount, description: adjustForm.description })
    Message.success('余额调整成功')
    done()
    fetchWallets()
  } catch {
    return false
  } finally { adjustLoading.value = false }
}

onMounted(() => { fetchBillingRecords(); fetchWallets() })

const { exporting, exportFile } = useExport({
  url: '/admin/billing-records/export',
  getFilters: () => ({
    tenant_id: billFilterTenantId.value,
  }),
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="计费管理" description="查看计费记录和管理租户钱包">
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

    <ATabs default-active-key="records" type="line">
      <ATabPane key="records" title="计费记录">
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
      </ATabPane>

      <ATabPane key="wallets" title="钱包管理">
        <ACard :bordered="false" class="mt-4">
          <ATable :columns="walletColumns" :data="walletData" :loading="walletLoading" :scroll="{ x: 1200 }" :bordered="false" :stripe="true" size="small" :pagination="false" row-key="id" />
          <div class="table-footer">
            <APagination v-model:current="walletPagination.current" v-model:page-size="walletPagination.pageSize" :total="walletPagination.total" :page-size-options="walletPagination.pageSizeOptions" show-page-size @change="fetchWallets" @page-size-change="(s: number) => { walletPagination.pageSize = s; walletPagination.current = 1; fetchWallets() }" />
          </div>
        </ACard>
      </ATabPane>
    </ATabs>

    <!-- Adjust Balance Modal -->
    <AModal v-model:visible="showAdjustModal" title="调整余额" :width="400" :on-before-ok="handleAdjust" :ok-loading="adjustLoading">
      <div class="mb-2 text-sm" style="color: var(--ta-text-tertiary)">租户ID：{{ adjustForm.tenant_id }}</div>
      <AForm :model="adjustForm" :auto-label-width="true" layout="vertical">
        <AFormItem label="金额">
          <AInputNumber v-model="adjustForm.amount" :precision="2" placeholder="正数充值，负数扣减" class="w-full" />
        </AFormItem>
        <AFormItem label="描述">
          <AInput v-model="adjustForm.description" placeholder="调整原因（选填）" />
        </AFormItem>
      </AForm>
    </AModal>
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
