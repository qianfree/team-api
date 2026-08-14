<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NDropdown } from 'naive-ui'
import Icon from '@/components/common/Icon.vue'
import ResponsiveDataTable from '@/components/common/ResponsiveDataTable.vue'
import { renderBadge, formatMoney, tableScrollX } from '@/utils/renderUtils'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

const loading = ref(false)
const orders = ref<any[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const statusFilter = ref('')

// 导出格式下拉选项（NDropdown 默认 teleport 到 body，避免被 .card 的 backdrop-filter
// stacking context 困住而被下方表格面板遮挡）
const exportDropdownOptions = [
	{ label: '导出 CSV', key: 'csv' },
	{ label: '导出 Excel', key: 'xlsx' },
]

function handleExport(format: string | number) {
	exportFile(format as 'csv' | 'xlsx')
}
const { exporting, exportFile } = useExport({
	url: '/tenant/orders/export',
	getFilters: () => ({
		status: statusFilter.value,
	}),
})

const orderTypeLabel: Record<string, string> = {
  new_plan: '新购套餐',
  renew: '续费',
  upgrade: '升级',
  downgrade: '降级',
  recharge: '充值',
}

const statusLabel: Record<string, string> = {
  pending: '待支付',
  paid: '已支付',
  fulfilled: '已履约',
  expired: '已过期',
  cancelled: '已取消',
  refunding: '退款中',
  refunded: '已退款',
  refund_failed: '退款失败',
}

const statusBadgeClass: Record<string, string> = {
  pending: 'badge-warning',
  paid: 'badge-primary',
  fulfilled: 'badge-success',
  expired: 'badge-gray',
  cancelled: 'badge-gray',
  refunding: 'badge-warning',
  refunded: 'badge-danger',
  refund_failed: 'badge-danger',
}

const statusFilterOptions = [
  { label: '全部', value: '' },
  { label: '待支付', value: 'pending' },
  { label: '已完成', value: 'fulfilled' },
  { label: '已取消', value: 'cancelled' },
]

async function fetchOrders() {
  loading.value = true
  try {
    const params: any = { page: page.value, page_size: pageSize.value }
    if (statusFilter.value) params.status = statusFilter.value
    const res = await request.get('/tenant/orders', { params })
    const raw = res.data?.data
    orders.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
    total.value = raw?.total || 0
  } catch (e) {
    // interceptor handles error toast
  } finally {
    loading.value = false
  }
}

async function handlePay(order: any) {
  try {
    await request.post(`/tenant/orders/${order.id}/pay`)
    fetchOrders()
  } catch (e) {
    // interceptor handles error toast
  }
}

async function handleCancel(order: any) {
  try {
    await request.post(`/tenant/orders/${order.id}/cancel`)
    fetchOrders()
  } catch (e) {
    // interceptor handles error toast
  }
}

// NDataTable 列定义
const columns = computed<DataTableColumns<any>>(() => [
  {
    title: '订单号',
    key: 'order_no',
    width: 180,
    render: (row) => h('span', { class: 'font-mono text-xs text-gray-600' }, row.order_no),
  },
  {
    title: '类型',
    key: 'order_type',
    width: 110,
    render: (row) => h('span', { class: 'font-medium text-gray-900' }, orderTypeLabel[row.order_type] || row.order_type),
  },
  {
    title: '金额',
    key: 'final_amount',
    width: 130,
    render: (row) => h('span', { class: 'font-medium' }, formatMoney(row.final_amount, { currency: 'CNY', precision: 2 })),
  },
  {
    title: '状态',
    key: 'status',
    width: 110,
    render: (row) => renderBadge(row.status, statusLabel, statusBadgeClass),
  },
  {
    title: '创建时间',
    key: 'created_at',
    width: 170,
    render: (row) => h('span', { class: 'text-xs text-gray-500' }, row.created_at ? new Date(row.created_at).toLocaleString() : '-'),
  },
  {
    title: '操作',
    key: 'actions',
    width: 120,
    align: 'right',
    render: (row) =>
      h('div', { class: 'flex items-center justify-end gap-2' }, [
        row.status === 'pending'
          ? h(NButton, { size: 'small', type: 'primary', onClick: () => handlePay(row) }, { default: () => '支付' })
          : null,
        row.status === 'pending'
          ? h(NButton, { size: 'small', onClick: () => handleCancel(row) }, { default: () => '取消' })
          : null,
      ]),
  },
])

function handlePageSizeChange() {
  page.value = 1
  fetchOrders()
}

onMounted(() => {
  fetchOrders()
})
</script>

<template>
  <div class="viewport-table-page">
    <div class="page-header">
      <div class="flex items-center justify-between">
        <div>
          <h2 class="page-title">订单记录</h2>
          <p class="page-description">查看所有订阅和充值订单</p>
        </div>
        <!-- Export dropdown -->
        <n-dropdown trigger="click" :options="exportDropdownOptions" @select="handleExport">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="exporting">
            <Icon v-if="exporting" name="refresh" size="sm" class="animate-spin" />
            <Icon v-else name="download" size="sm" />
            导出
            <Icon name="chevronDown" size="xs" />
          </button>
        </n-dropdown>
      </div>
    </div>

    <!-- Filters -->
    <div class="flex items-center gap-3 mb-4">
      <div v-for="opt in statusFilterOptions" :key="opt.value">
        <button
          class="tab"
          :class="{ 'tab-active': statusFilter === opt.value }"
          @click="statusFilter = opt.value; page = 1; fetchOrders()"
        >
          {{ opt.label }}
        </button>
      </div>
    </div>

    <!-- Table -->
    <div class="viewport-table-panel p-0 overflow-hidden">
      <ResponsiveDataTable
        remote
        fill-height
        v-model:page="page"
        v-model:page-size="pageSize"
        :item-count="total"
        :page-sizes="[10, 20, 50, 100]"
        show-size-picker
        :loading="loading"
        :columns="columns"
        :scroll-x="tableScrollX(columns)"
        :data="orders"
        :row-key="(row: any) => row.id"
        card-title-key="order_no"
        card-badge-key="status"
        card-subtitle-key="created_at"
        :card-fields="['order_type', 'final_amount']"
        card-actions-key="actions"
        @update:page="fetchOrders"
        @update:page-size="handlePageSizeChange"
      >
        <template #empty>
          <div class="empty-state">
            <Icon name="document" size="xl" class="empty-state-icon text-gray-300" />
            <p class="empty-state-title">暂无订单</p>
            <p class="empty-state-description">您的订单记录将显示在这里</p>
          </div>
        </template>
      </ResponsiveDataTable>
    </div>
  </div>
</template>

<style scoped>
.tabs {
  display: flex;
  gap: 0.25rem;
  padding: 0.25rem;
  border-radius: 0.75rem;
  background: #f3f4f6;
}
.tab {
  border-radius: 0.5rem;
  padding: 0.5rem 1rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: #4b5563;
  transition: all 0.15s;
  cursor: pointer;
  border: none;
  background: transparent;
}
.tab-active {
  background: white;
  color: #111827;
  box-shadow: 0 1px 2px rgba(0,0,0,0.05);
}

.modal-enter-active,
.modal-leave-active {
  transition: opacity 250ms ease-out;
}
.modal-enter-active .modal-content,
.modal-leave-active .modal-content {
  transition: transform 250ms ease-out, opacity 250ms ease-out;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
.modal-enter-from .modal-content {
  transform: scale(0.95);
  opacity: 0;
}
.modal-leave-to .modal-content {
  transform: scale(0.95);
  opacity: 0;
}
</style>
