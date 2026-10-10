<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Space, Message,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import TableStats from '@/components/TableStats.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'
import { formatBilling } from '@/composables/useCurrency'

const loading = ref(false)
const redemptions = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })
const statusFilter = ref<string | undefined>('')

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '兑换码', dataIndex: 'code', width: 150, render({ record }) { return h('span', { class: 'font-mono text-xs' }, record.code) } },
  { title: '类型', dataIndex: 'type', width: 80, render({ record }) {
    const labels: any = { quota: '额度', plan: '套餐', duration: '时长' }
    return h(Tag, { size: 'small' }, () => labels[record.type] || record.type)
  }},
  { title: '值', dataIndex: 'value', width: 100, render({ record }) { return record.type === 'quota' ? record.value.toLocaleString() : record.value } },
  { title: '套餐ID', dataIndex: 'plan_id', width: 80 },
  { title: '时长(天)', dataIndex: 'duration_days', width: 80 },
  { title: '批次', dataIndex: 'batch_no', width: 120 },
  { title: '已用/总', dataIndex: 'usage', width: 90, render({ record }) { return `${record.used_count || 0}/${record.max_uses}` } },
  { title: '过期时间', dataIndex: 'expires_at', width: 170, render({ record }) { return record.expires_at?.substring(0, 19) || '-' } },
  { title: '状态', dataIndex: 'status', width: 80, render({ record }) {
    const map: any = { active: 'green', disabled: 'orangered', expired: undefined }
    return h(Tag, { color: map[record.status], size: 'small' }, () => record.status)
  }},
  {
    title: '操作', dataIndex: 'actions', width: 160, fixed: 'right',
    render({ record }) {
      const btns = [
        h(Button, { size: 'small', onClick: () => showUsageDetail(record) }, () => '使用记录'),
      ]
      if (record.status === 'active') {
        btns.push(h(Button, { size: 'small', status: 'warning', onClick: () => disableCode(record) }, () => '禁用'))
      }
      return h(Space, { size: 'small' }, () => btns)
    },
  },
]

async function fetchRedemptions() {
  loading.value = true
  try {
    const params: any = { page: pagination.current, page_size: pagination.pageSize }
    if (statusFilter.value) params.status = statusFilter.value
    const res = await request.get('/admin/redemptions', { params })
    const payload = res.data?.data
    const list = payload?.list || payload?.data || []
    redemptions.value = Array.isArray(list) ? list.filter(Boolean) : []
    pagination.total = payload?.total || 0
  } catch { /* interceptor handles error toast */ } finally { loading.value = false }
}

function disableCode(row: any) {
  request.put(`/admin/redemptions/${row.id}/disable`).then(() => {
    Message.success('已禁用'); fetchRedemptions()
  }).catch(() => { /* interceptor handles error toast */ })
}

// Batch Create
const showCreateModal = ref(false)
const createForm = reactive({ count: 10, type: 'quota', value: 100, plan_id: null, duration_days: 30, max_uses: 1, expires_days: 90 })
const createLoading = ref(false)

async function handleCreate(done: () => void) {
  createLoading.value = true
  try {
    await request.post('/admin/redemptions', createForm)
    Message.success(`成功生成 ${createForm.count} 个兑换码`)
    done(); fetchRedemptions()
  } catch { return false } finally { createLoading.value = false }
}

// === 使用记录（弹窗，按兑换码查看） ===
const showUsages = ref(false)
const usagesLoading = ref(false)
const usages = ref<any[]>([])
const usagesPagination = reactive({ current: 1, pageSize: 20, total: 0 })
const usageRedemptionId = ref<number>(0)
const usageCode = ref('')

function showUsageDetail(row: any) {
  usageRedemptionId.value = row.id
  usageCode.value = row.code
  showUsages.value = true; usagesPagination.current = 1; fetchUsages()
}

async function fetchUsages() {
  usagesLoading.value = true
  try {
    const res = await request.get('/admin/redemptions/usages', {
      params: { page: usagesPagination.current, page_size: usagesPagination.pageSize, redemption_id: usageRedemptionId.value },
    })
    const payload = res.data?.data
    usages.value = payload?.list || []
    usagesPagination.total = payload?.total || 0
  } catch { /* interceptor handles error toast */ } finally { usagesLoading.value = false }
}

// 弹窗内同一兑换码的类型/面值恒定，只保留兑换方与时间信息
const usageColumns: TableColumnData[] = [
  { title: '组织名称', dataIndex: 'tenant_name', width: 160 },
  { title: '兑换人', dataIndex: 'username', width: 130 },
  { title: '面值', dataIndex: 'value', width: 120, render({ record }) { return record.type === 'quota' ? formatBilling(record.value, 6, true) : '-' } },
  { title: '时间', dataIndex: 'created_at', width: 180, render({ record }) { return record.created_at?.substring(0, 19) || '-' } },
]

onMounted(fetchRedemptions)

const { exporting, exportFile } = useExport({
  url: '/admin/redemptions/export',
  getFilters: () => ({
    status: statusFilter.value || undefined,
  }),
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="兑换码管理" description="管理兑换码的生成、查看和禁用">
      <template #actions>
        <ASelect v-model="statusFilter" :options="[
          { label: '全部', value: '' }, { label: '可用', value: 'active' },
          { label: '已禁用', value: 'disabled' }, { label: '已过期', value: 'expired' },
        ]" style="width: 120px" @change="() => { pagination.current = 1; fetchRedemptions() }" />
        <ADropdown trigger="hover">
          <AButton :loading="exporting">导出</AButton>
          <template #content>
            <ADoption @click="exportFile('csv')">导出 CSV</ADoption>
            <ADoption @click="exportFile('xlsx')">导出 Excel</ADoption>
          </template>
        </ADropdown>
        <AButton type="primary" @click="showCreateModal = true">兑换码生成</AButton>
      </template>
    </PageHeader>

    <ACard :bordered="false">
      <ResponsiveTable
        :columns="columns"
        :data="redemptions"
        :loading="loading"
        row-key="id"
        :scroll="{ x: 1270 }"
        card-title-key="code"
        card-subtitle-key="type"
        card-badge-key="status"
        :card-fields="['value', 'usage', 'batch_no', 'duration_days', 'plan_id']"
      />
      <div class="table-footer">
        <TableStats :total="pagination.total" />
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchRedemptions" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchRedemptions() }" />
      </div>
    </ACard>

    <AModal v-model:visible="showCreateModal" title="批量生成兑换码" :width="450" :mask-closable="false" :on-before-ok="handleCreate" :ok-loading="createLoading">
      <AForm :model="createForm" :auto-label-width="true" layout="vertical">
        <AFormItem label="数量"><AInputNumber v-model="createForm.count" :min="1" :max="1000" class="w-full" /></AFormItem>
        <AFormItem label="类型">
          <ASelect v-model="createForm.type" :options="[
            { label: '额度', value: 'quota' }, { label: '套餐时长', value: 'plan' }, { label: '时长(天)', value: 'duration' },
          ]" />
        </AFormItem>
        <AFormItem v-if="createForm.type === 'quota'" label="额度值"><AInputNumber v-model="createForm.value" :min="0" class="w-full" /></AFormItem>
        <AFormItem v-if="createForm.type === 'plan'" label="套餐ID"><AInputNumber v-model="createForm.plan_id" :min="1" class="w-full" placeholder="套餐ID" /></AFormItem>
        <AFormItem v-if="createForm.type === 'duration'" label="天数"><AInputNumber v-model="createForm.duration_days" :min="1" class="w-full" /></AFormItem>
        <AFormItem label="单码可用次数" extra="大于 1 为多人共享码，每个组织限兑一次">
          <AInputNumber v-model="createForm.max_uses" :min="1" :max="100000" class="w-full" />
        </AFormItem>
        <AFormItem label="有效期(天)"><AInputNumber v-model="createForm.expires_days" :min="1" :max="730" class="w-full" /></AFormItem>
      </AForm>
    </AModal>

    <!-- Usages Modal -->
    <AModal v-model:visible="showUsages" :title="`使用记录 · ${usageCode}`" :width="700" :footer="false">
      <ATable :columns="usageColumns" :data="usages" :loading="usagesLoading" :pagination="false" row-key="id" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="usagesPagination.current" v-model:page-size="usagesPagination.pageSize" :total="usagesPagination.total" show-page-size @change="fetchUsages" @page-size-change="() => { usagesPagination.current = 1; fetchUsages() }" />
      </div>
    </AModal>
  </div>
</template>
