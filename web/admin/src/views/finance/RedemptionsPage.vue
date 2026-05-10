<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Message,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

const loading = ref(false)
const redemptions = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })
const statusFilter = ref<string | undefined>(undefined)
const activeTab = ref('codes')

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
  { title: '状态', dataIndex: 'status', width: 80, render({ record }) {
    const map: any = { active: 'green', disabled: 'orangered', expired: undefined }
    return h(Tag, { color: map[record.status], size: 'small' }, () => record.status)
  }},
  {
    title: '操作', dataIndex: 'actions', width: 100, fixed: 'right',
    render({ record }) {
      return record.status === 'active'
        ? h(Button, { size: 'small', status: 'warning', onClick: () => disableCode(record) }, () => '禁用')
        : null
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
const createForm = reactive({ count: 10, type: 'quota', value: 100, plan_id: null, duration_days: 30 })
const createLoading = ref(false)

async function handleCreate(done: () => void) {
  createLoading.value = true
  try {
    await request.post('/admin/redemptions', createForm)
    Message.success(`成功生成 ${createForm.count} 个兑换码`)
    done(); fetchRedemptions()
  } catch { return false } finally { createLoading.value = false }
}

// === Usage Records ===
const usageLoading = ref(false)
const usages = ref<any[]>([])
const usagePagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })

const usageColumns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '兑换码', dataIndex: 'code', width: 150, render({ record }) { return h('span', { class: 'font-mono text-xs' }, record.code) } },
  { title: '类型', dataIndex: 'type', width: 80, render({ record }) {
    const labels: any = { quota: '额度', plan: '套餐', duration: '时长' }
    const colors: any = { quota: 'green', plan: 'blue', duration: 'orangered' }
    return h(Tag, { color: colors[record.type], size: 'small' }, () => labels[record.type] || record.type)
  }},
  { title: '面值', dataIndex: 'value', width: 100, render({ record }) { return record.type === 'quota' ? `+${Number(record.value).toFixed(2)}` : '-' } },
  { title: '组织名称', dataIndex: 'tenant_name', width: 140 },
  { title: '兑换人', dataIndex: 'username', width: 120 },
  { title: '时间', dataIndex: 'created_at', width: 170, render({ record }) { return record.created_at?.substring(0, 19) || '-' } },
]

async function fetchUsages() {
  usageLoading.value = true
  try {
    const res = await request.get('/admin/redemptions/usages', {
      params: { page: usagePagination.current, page_size: usagePagination.pageSize }
    })
    const payload = res.data?.data
    usages.value = payload?.list || []
    usagePagination.total = payload?.total || 0
  } catch { /* interceptor handles error toast */ } finally { usageLoading.value = false }
}

function onTabChange(key: string) {
  if (key === 'usages' && usages.value.length === 0) {
    fetchUsages()
  }
}

onMounted(fetchRedemptions)

const { exporting, exportFile } = useExport({
  url: '/admin/redemptions/export',
  getFilters: () => ({
    status: statusFilter.value,
  }),
})
</script>

<template>
  <div>
    <PageHeader title="兑换码管理" description="管理兑换码的生成、查看和禁用">
      <template #actions>
        <ADropdown trigger="hover">
          <AButton :loading="exporting">导出</AButton>
          <template #content>
            <ADoption @click="exportFile('csv')">导出 CSV</ADoption>
            <ADoption @click="exportFile('xlsx')">导出 Excel</ADoption>
          </template>
        </ADropdown>
        <AButton type="primary" @click="showCreateModal = true">批量生成</AButton>
      </template>
    </PageHeader>

    <ACard :bordered="false">
      <ATabs v-model:active-key="activeTab" @change="onTabChange">
        <ATabPane key="codes" title="兑换码列表">
          <div class="flex items-center justify-between mb-4">
            <span />
            <ASelect v-model="statusFilter" :options="[
              { label: '全部', value: '' }, { label: '可用', value: 'active' },
              { label: '已禁用', value: 'disabled' }, { label: '已过期', value: 'expired' },
            ]" style="width: 120px" allow-clear @change="() => { pagination.current = 1; fetchRedemptions() }" />
          </div>
          <ATable :columns="columns" :data="redemptions" :loading="loading" row-key="id" :scroll="{ x: 1100 }" :pagination="false" />
          <div class="mt-4 flex justify-end">
            <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchRedemptions" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchRedemptions() }" />
          </div>
        </ATabPane>

        <ATabPane key="usages" title="使用记录">
          <ATable :columns="usageColumns" :data="usages" :loading="usageLoading" row-key="id" :scroll="{ x: 900 }" :pagination="false" />
          <div class="mt-4 flex justify-end">
            <APagination v-model:current="usagePagination.current" v-model:page-size="usagePagination.pageSize" :total="usagePagination.total" :page-size-options="usagePagination.pageSizeOptions" show-page-size @change="fetchUsages" @page-size-change="(s: number) => { usagePagination.pageSize = s; usagePagination.current = 1; fetchUsages() }" />
          </div>
        </ATabPane>
      </ATabs>
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
      </AForm>
    </AModal>
  </div>
</template>
