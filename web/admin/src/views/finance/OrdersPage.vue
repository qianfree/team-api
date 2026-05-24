<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Space, Message,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

const loading = ref(false)
const orders = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })
const statusFilter = ref<string | undefined>(undefined)
const tenantFilter = ref('')
const statusOptions = [
  { label: '全部', value: '' },
  { label: '待支付', value: 'pending' }, { label: '已支付', value: 'paid' },
  { label: '已履约', value: 'fulfilled' }, { label: '已过期', value: 'expired' },
  { label: '已取消', value: 'cancelled' }, { label: '退款中', value: 'refunding' },
  { label: '已退款', value: 'refunded' },
]

const statusTagColor: Record<string, string> = {
  pending: 'orangered', paid: 'arcoblue', fulfilled: 'green',
  expired: undefined, cancelled: undefined, refunding: 'orangered', refunded: 'red', refund_failed: 'red',
}
const statusLabel: Record<string, string> = {
  pending: '待支付', paid: '已支付', fulfilled: '已履约', expired: '已过期',
  cancelled: '已取消', refunding: '退款中', refunded: '已退款', refund_failed: '退款失败',
}
const orderTypeLabel: Record<string, string> = {
  new_plan: '套餐购买', renew: '续费', upgrade: '升级', downgrade: '降级', recharge: '充值',
}

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '订单号', dataIndex: 'order_no', width: 180, ellipsis: true },
  { title: '租户ID', dataIndex: 'tenant_id', width: 80 },
  {
    title: '类型', dataIndex: 'order_type', width: 80,
    render({ record }) { return h(Tag, { size: 'small' }, () => orderTypeLabel[record.order_type] || record.order_type) },
  },
  {
    title: '金额', dataIndex: 'final_amount', width: 100,
    render({ record }) { return `¥${Number(record.final_amount).toFixed(2)}` },
  },
  { title: '支付渠道', dataIndex: 'payment_channel', width: 90 },
  {
    title: '状态', dataIndex: 'status', width: 90,
    render({ record }) { return h(Tag, { color: statusTagColor[record.status], size: 'small' }, () => statusLabel[record.status] || record.status) },
  },
  {
    title: '创建时间', dataIndex: 'created_at', width: 170,
    render({ record }) { return record.created_at ? new Date(record.created_at).toLocaleString() : '-' },
  },
  {
    title: '操作', dataIndex: 'actions', width: 120, fixed: 'right',
    render({ record }) {
      const btns: any[] = []
      if (record.status === 'paid' || record.status === 'fulfilled') {
        btns.push(h(Button, { size: 'small', status: 'warning', onClick: () => openRefundModal(record) }, () => '退款'))
      }
      return h(Space, { size: 'small' }, () => btns)
    },
  },
]

async function fetchOrders() {
  loading.value = true
  try {
    const params: any = { page: pagination.current, page_size: pagination.pageSize }
    if (statusFilter.value) params.status = statusFilter.value
    if (tenantFilter.value) params.tenant_id = tenantFilter.value
    const res = await request.get('/admin/orders', { params })
    const data = res.data?.data
    orders.value = data?.list || []
    pagination.total = data?.total || 0
  } catch { /* interceptor handles error toast */ } finally { loading.value = false }
}

// === Refund Modal ===
const showRefundModal = ref(false)
const refundOrderId = ref<number | null>(null)
const refundAmount = ref(0)
const refundForm = reactive({ reason: '' })
const refundLoading = ref(false)
const refundPreview = ref<any>(null)
const refundPreviewLoading = ref(false)

async function openRefundModal(row: any) {
  refundOrderId.value = row.id
  refundForm.reason = ''
  refundAmount.value = Number(row.final_amount)
  refundPreview.value = null
  showRefundModal.value = true

  // Fetch refund preview
  refundPreviewLoading.value = true
  try {
    const res = await request.get(`/admin/orders/${row.id}/refund-preview`)
    refundPreview.value = res.data?.data
    if (refundPreview.value?.can_refund && refundPreview.value?.refund_amount >= 0) {
      refundAmount.value = refundPreview.value.refund_amount
    }
  } catch {
    // Fallback to order final_amount
  } finally {
    refundPreviewLoading.value = false
  }
}

async function handleRefund(done: () => void) {
  if (!refundForm.reason.trim()) { Message.warning('请填写退款原因'); return }
  refundLoading.value = true
  try {
    await request.post(`/admin/orders/${refundOrderId.value}/refund`, { reason: refundForm.reason })
    Message.success('退款已发起')
    done(); fetchOrders()
  } catch { return false } finally { refundLoading.value = false }
}

onMounted(fetchOrders)

const { exporting, exportFile } = useExport({
  url: '/admin/orders/export',
  getFilters: () => ({
    status: statusFilter.value,
    tenant_id: tenantFilter.value,
  }),
})
</script>

<template>
  <div>
    <PageHeader title="订单管理" description="查看和管理所有订单">
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

    <ACard :bordered="false">
      <template #title>
        <div class="flex items-center justify-between w-full">
          <span>订单列表</span>
          <ASpace>
            <AInput v-model="tenantFilter" placeholder="租户ID" allow-clear style="width: 120px" @clear="() => { pagination.current = 1; fetchOrders() }" />
            <ASelect v-model="statusFilter" :options="statusOptions" style="width: 120px" allow-clear @change="() => { pagination.current = 1; fetchOrders() }" />
          </ASpace>
        </div>
      </template>
      <ATable :columns="columns" :data="orders" :loading="loading" row-key="id" :scroll="{ x: 1100 }" :pagination="false" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchOrders" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchOrders() }" />
      </div>
    </ACard>

    <!-- Refund Modal -->
    <AModal v-model:visible="showRefundModal" title="发起退款" :width="450" :mask-closable="false" :on-before-ok="handleRefund" :ok-loading="refundLoading">
      <AForm :model="refundForm" :auto-label-width="true" layout="vertical">
        <AFormItem label="退款金额">
          <div v-if="refundPreviewLoading" class="text-gray-400 text-sm">计算中...</div>
          <template v-else-if="refundPreview && !refundPreview.can_refund">
            <span class="text-gray-400">{{ refundPreview.message || '不可退款' }}</span>
          </template>
          <template v-else>
            <span class="text-red-500 font-medium">¥{{ refundAmount.toFixed(2) }}</span>
            <span v-if="refundPreview?.order_type === 'new_plan'" class="text-xs text-gray-400 ml-2">
              (套餐按比例退款)
            </span>
          </template>
        </AFormItem>
        <AFormItem label="退款原因" required>
          <AInput v-model="refundForm.reason" type="textarea" :auto-size="{ minRows: 3, maxRows: 5 }" placeholder="请填写退款原因" />
        </AFormItem>
      </AForm>
    </AModal>
  </div>
</template>
