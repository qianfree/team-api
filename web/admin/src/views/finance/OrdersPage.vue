<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Space, Message, Modal,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import TableStats from '@/components/TableStats.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'
import { useDayShortcuts } from '@/composables/useDateRange'
import { useTenantOptions } from '@/composables/useTenantOptions'
import { formatOrder } from '@/composables/useCurrency'
import { hasPermission } from '@/utils/permission'

const dayShortcuts = useDayShortcuts()
const { tenantOptions, fetchTenantOptions, handleTenantSearch } = useTenantOptions()

const loading = ref(false)
const orders = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })
const statusFilter = ref<string | undefined>(undefined)
const tenantFilter = ref<number | undefined>(undefined)
const orderNoFilter = ref('')
// 时间范围（YYYY-MM-DD 闭区间），缺省查全部
const dateRange = ref<string[] | undefined>(undefined)
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
  new_plan: '新购', renew: '续费', upgrade: '升级', downgrade: '降级', recharge: '充值',
}

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '订单号', dataIndex: 'order_no', width: 180, ellipsis: true, tooltip: true },
  { title: '租户', dataIndex: 'tenant_name', width: 120, ellipsis: true, tooltip: true },
  {
    title: '类型', dataIndex: 'order_type', width: 80,
    render({ record }) { return h(Tag, { size: 'small' }, () => orderTypeLabel[record.order_type] || record.order_type) },
  },
  {
    title: '金额', dataIndex: 'final_amount', width: 100,
    render({ record }) { return formatOrder(record.final_amount, 2) },
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
      // 退款/手动完成同一权限点（服务端 rbac 对 /refund 与 /complete 均映射 order:refund）
      if (hasPermission('order:refund')) {
        if (record.status === 'pending') {
          btns.push(h(Button, { size: 'small', status: 'success', onClick: () => handleComplete(record) }, () => '手动完成'))
        }
        if (record.status === 'paid' || record.status === 'fulfilled') {
          btns.push(h(Button, { size: 'small', status: 'warning', onClick: () => openRefundModal(record) }, () => '退款'))
        }
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
    if (orderNoFilter.value.trim()) params.order_no = orderNoFilter.value.trim()
    if (dateRange.value?.[0]) params.start_date = dateRange.value[0]
    if (dateRange.value?.[1]) params.end_date = dateRange.value[1]
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

function openRefundModal(row: any) {
  refundOrderId.value = row.id
  refundAmount.value = Number(row.final_amount)
  refundForm.reason = ''
  showRefundModal.value = true
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

// 手动完成订单：标记已支付并立即履约（充值按原价入账 / 套餐立即生效），
// 适用于线下收款等已实际收款的场景，操作会以 ADMIN_管理员ID 记入支付流水号
function handleComplete(row: any) {
  const typeText = orderTypeLabel[row.order_type] || row.order_type
  Modal.confirm({
    title: '手动完成订单',
    content: `订单 ${row.order_no}（${typeText}，实付 ${formatOrder(row.final_amount, 2)}）将标记为已支付并立即履约。请确认已通过线下等方式实际收到款项。`,
    okText: '确认完成',
    okButtonProps: { status: 'warning' },
    cancelText: '取消',
    onOk: async () => {
      try {
        await request.post(`/admin/orders/${row.id}/complete`)
        Message.success('订单已完成并履约')
        fetchOrders()
      } catch { /* interceptor handles error toast */ }
    },
  })
}

function resetAndFetch() {
  pagination.current = 1
  fetchOrders()
}

function handleReset() {
  orderNoFilter.value = ''
  tenantFilter.value = undefined
  statusFilter.value = undefined
  dateRange.value = undefined
  resetAndFetch()
}

onMounted(() => {
  fetchTenantOptions()
  fetchOrders()
})

const { exporting, exportFile } = useExport({
  url: '/admin/orders/export',
  getFilters: () => ({
    status: statusFilter.value,
    tenant_id: tenantFilter.value,
    order_no: orderNoFilter.value.trim(),
    start_date: dateRange.value?.[0],
    end_date: dateRange.value?.[1],
  }),
})
</script>

<template>
  <div class="page-table">
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
          v-model="tenantFilter"
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
        <AInput v-model="orderNoFilter" placeholder="订单号" allow-clear style="width: 200px" @clear="resetAndFetch" @keydown.enter="resetAndFetch" />
        <ASelect v-model="statusFilter" :options="statusOptions" placeholder="状态" style="width: 130px" allow-clear @change="resetAndFetch" />
        <div class="filter-actions">
          <AButton type="primary" @click="resetAndFetch">搜索</AButton>
          <AButton @click="handleReset">重置</AButton>
        </div>
      </div>
    </ACard>

    <ACard :bordered="false">
      <ResponsiveTable
        :columns="columns"
        :data="orders"
        :loading="loading"
        row-key="id"
        :scroll="{ x: 1100 }"
        card-title-key="order_no"
        card-badge-key="status"
        card-subtitle-key="created_at"
        :card-fields="['order_type', 'final_amount', 'payment_channel', 'tenant_name']"
      />
      <div class="table-footer">
        <TableStats :total="pagination.total" />
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchOrders" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchOrders() }" />
      </div>
    </ACard>

    <!-- Refund Modal -->
    <AModal v-model:visible="showRefundModal" title="发起退款" :width="450" :mask-closable="false" :on-before-ok="handleRefund" :ok-loading="refundLoading">
      <AForm :model="refundForm" :auto-label-width="true" layout="vertical">
        <AFormItem label="退款金额">
          <span class="text-red-500 font-medium">{{ formatOrder(refundAmount, 2) }}</span>
        </AFormItem>
        <AFormItem label="退款原因" required>
          <AInput v-model="refundForm.reason" type="textarea" :auto-size="{ minRows: 3, maxRows: 5 }" placeholder="请填写退款原因" />
        </AFormItem>
      </AForm>
    </AModal>
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
</style>
