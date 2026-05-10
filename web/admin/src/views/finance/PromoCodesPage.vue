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
const promoCodes = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })

const showUsages = ref(false)
const usagesLoading = ref(false)
const usages = ref<any[]>([])
const usagesPagination = reactive({ current: 1, pageSize: 20, total: 0 })
const usagePromoCodeId = ref<number>(0)

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '名称', dataIndex: 'name', width: 140 },
  { title: '优惠码', dataIndex: 'code', width: 140, render({ record }) { return h('span', { class: 'font-mono text-xs font-bold' }, record.code) } },
  { title: '类型', dataIndex: 'type', width: 80, render({ record }) { return h(Tag, { size: 'small' }, () => record.type === 'percentage' ? '折扣%' : '立减') } },
  { title: '折扣值', dataIndex: 'discount_value', width: 90 },
  { title: '已用/总量', dataIndex: 'usage', width: 90, render({ record }) { return `${record.used_count}/${record.total_count || '不限'}` } },
  { title: '状态', dataIndex: 'status', width: 80, render({ record }) {
    const map: any = { active: 'green', disabled: 'orangered', expired: undefined }
    return h(Tag, { color: map[record.status], size: 'small' }, () => record.status)
  }},
  { title: '有效期', dataIndex: 'valid_to', width: 110, render({ record }) { return record.valid_to?.substring(0, 10) || '-' } },
  {
    title: '操作', dataIndex: 'actions', width: 150, fixed: 'right',
    render({ record }) {
      return h(Space, { size: 'small' }, () => [
        h(Button, { size: 'small', onClick: () => showUsageDetail(record) }, () => '使用记录'),
        h(Button, { size: 'small', onClick: () => toggleStatus(record) }, () => record.status === 'active' ? '禁用' : '启用'),
      ])
    },
  },
]

async function fetchList() {
  loading.value = true
  try {
    const res = await request.get('/admin/promo-codes', { params: { page: pagination.current, page_size: pagination.pageSize } })
    const data = res.data?.data
    promoCodes.value = data?.list || []; pagination.total = data?.total || 0
  } catch { /* interceptor handles error toast */ } finally { loading.value = false }
}

async function toggleStatus(row: any) {
  try { await request.put(`/admin/promo-codes/${row.id}`, { status: row.status === 'active' ? 'disabled' : 'active' }); Message.success('操作成功'); fetchList() }
  catch { /* interceptor handles error toast */ }
}

async function showUsageDetail(row: any) {
  usagePromoCodeId.value = row.id
  showUsages.value = true; usagesPagination.current = 1; await fetchUsages()
}

async function fetchUsages() {
  usagesLoading.value = true
  try {
    const res = await request.get(`/admin/promo-codes/${usagePromoCodeId.value}/usages`, { params: { page: usagesPagination.current, page_size: usagesPagination.pageSize } })
    const data = res.data?.data
    usages.value = data?.list || []; usagesPagination.total = data?.total || 0
  } catch { /* interceptor handles error toast */ } finally { usagesLoading.value = false }
}

// Create/Edit modal
const showFormModal = ref(false)
const editingId = ref<number | null>(null)
const formLoading = ref(false)
const form = reactive({
  name: '', code: '', type: 'fixed', discount_value: 0,
  min_amount: 0, max_discount: 0, total_count: 0, per_user_limit: 1,
  valid_from: '', valid_to: '', status: 'active',
})

function openCreate() {
  editingId.value = null
  Object.assign(form, { name: '', code: '', type: 'fixed', discount_value: 0, min_amount: 0, max_discount: 0, total_count: 0, per_user_limit: 1, valid_from: '', valid_to: '', status: 'active' })
  showFormModal.value = true
}

async function handleSubmit(done: () => void) {
  formLoading.value = true
  try {
    if (editingId.value) {
      await request.put(`/admin/promo-codes/${editingId.value}`, { ...form, valid_from: form.valid_from || null, valid_to: form.valid_to || null })
      Message.success('更新成功')
    } else {
      await request.post('/admin/promo-codes', { ...form, valid_from: form.valid_from || null, valid_to: form.valid_to || null })
      Message.success('创建成功')
    }
    done(); fetchList()
  } catch { return false } finally { formLoading.value = false }
}

onMounted(fetchList)

const { exporting, exportFile } = useExport({
  url: '/admin/promo-codes/export',
})
</script>

<template>
  <div>
    <PageHeader title="优惠码管理" description="创建和管理优惠码">
      <template #actions>
        <AButton type="primary" @click="openCreate">创建优惠码</AButton>
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
      <ATable :columns="columns" :data="promoCodes" :loading="loading" row-key="id" :scroll="{ x: 1200 }" :pagination="false" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchList" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchList() }" />
      </div>
    </ACard>

    <!-- Create/Edit Modal -->
    <AModal v-model:visible="showFormModal" :title="editingId ? '编辑优惠码' : '创建优惠码'" :width="500" :mask-closable="false" :on-before-ok="handleSubmit" :ok-loading="formLoading">
      <AForm :model="form" :auto-label-width="true" layout="vertical">
        <AFormItem label="名称" required><AInput v-model="form.name" /></AFormItem>
        <AFormItem label="优惠码" required><AInput v-model="form.code" :disabled="!!editingId" placeholder="留空自动生成" /></AFormItem>
        <AFormItem label="类型">
          <ASelect v-model="form.type" :options="[{ label: '折扣百分比', value: 'percentage' }, { label: '立减固定金额', value: 'fixed' }]" />
        </AFormItem>
        <AFormItem label="折扣值"><AInputNumber v-model="form.discount_value" :min="0" class="w-full" placeholder="百分比时 0-100" /></AFormItem>
        <AFormItem label="最低金额"><AInputNumber v-model="form.min_amount" :min="0" class="w-full" /></AFormItem>
        <AFormItem label="最大折扣"><AInputNumber v-model="form.max_discount" :min="0" class="w-full" placeholder="0=不限" /></AFormItem>
        <AFormItem label="总次数"><AInputNumber v-model="form.total_count" :min="0" class="w-full" placeholder="0=不限" /></AFormItem>
        <AFormItem label="每人限用"><AInputNumber v-model="form.per_user_limit" :min="0" /></AFormItem>
        <AFormItem label="开始日期"><AInput type="date" v-model="form.valid_from" /></AFormItem>
        <AFormItem label="结束日期"><AInput type="date" v-model="form.valid_to" /></AFormItem>
      </AForm>
    </AModal>

    <!-- Usages Modal -->
    <AModal v-model:visible="showUsages" title="使用记录" :width="700" :footer="false">
      <ATable :columns="[
        { title: '租户ID', dataIndex: 'tenant_id', width: 80 },
        { title: '订单ID', dataIndex: 'order_id', width: 80 },
        { title: '折扣金额', dataIndex: 'discount_amount', width: 100, render({ record }) { return '$' + Number(record.discount_amount).toFixed(2) } },
        { title: '时间', dataIndex: 'created_at', width: 160 },
      ]" :data="usages" :loading="usagesLoading" :pagination="false" row-key="id" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="usagesPagination.current" :total="usagesPagination.total" show-page-size @change="fetchUsages" />
      </div>
    </AModal>
  </div>
</template>
