<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Space, Message, Modal,
} from '@arco-design/web-vue'
import type { TableColumnData, FormInstance } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

const loading = ref(false)
const plans = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })
const statusFilter = ref<string | undefined>(undefined)
const statusOptions = [
  { label: '全部', value: '' },
  { label: '上架', value: 'active' },
  { label: '下架', value: 'archived' },
]

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '名称', dataIndex: 'name', width: 120 },
  { title: '标识', dataIndex: 'identifier', width: 100 },
  { title: '月价', dataIndex: 'monthly_price', width: 100, render({ record }) { return `¥${Number(record.monthly_price).toFixed(2)}` } },
  { title: '年价', dataIndex: 'yearly_price', width: 100, render({ record }) { return `¥${Number(record.yearly_price).toFixed(2)}` } },
  {
    title: '状态', dataIndex: 'status', width: 80,
    render({ record }) { return h(Tag, { color: record.status === 'active' ? 'green' : undefined, size: 'small' }, () => record.status === 'active' ? '上架' : '下架') },
  },
  {
    title: '推荐', dataIndex: 'is_recommended', width: 70,
    render({ record }) { return h(Tag, { color: record.is_recommended ? 'orangered' : undefined, size: 'small' }, () => record.is_recommended ? '推荐' : '-') },
  },
  { title: '排序', dataIndex: 'sort_order', width: 70 },
  {
    title: '操作', dataIndex: 'actions', width: 200, fixed: 'right',
    render({ record }) {
      return h(Space, { size: 4 }, () => [
        h(Button, { size: 'small', type: 'primary', onClick: () => openEditModal(record) }, () => '编辑'),
        h(Button, { size: 'small', onClick: () => toggleRecommend(record) }, () => record.is_recommended ? '取消推荐' : '设为推荐'),
        record.status === 'active' ? h(Button, { size: 'small', status: 'danger', onClick: () => archivePlan(record) }, () => '下架') : null,
      ])
    },
  },
]

async function fetchPlans() {
  loading.value = true
  try {
    const params: any = { page: pagination.current, page_size: pagination.pageSize }
    if (statusFilter.value) params.status = statusFilter.value
    const res = await request.get('/admin/plans', { params })
    const data = res.data?.data
    plans.value = data?.list || []
    pagination.total = data?.total || 0
  } catch { /* interceptor handles error toast */ } finally { loading.value = false }
}

const showModal = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance | null>(null)
const formLoading = ref(false)
const formData = reactive({
  name: '', identifier: '', description: '', monthly_price: 0, yearly_price: 0,
  sort_order: 0, is_recommended: false,
})

function openCreateModal() {
  editingId.value = null
  showModal.value = true
}

function openEditModal(row: any) {
  editingId.value = row.id
  showModal.value = true
}

async function handleSubmit() {
  formLoading.value = true
  try {
    if (editingId.value) {
      await request.put(`/admin/plans/${editingId.value}`, { ...formData, status: undefined, created_at: undefined, updated_at: undefined, id: undefined, version: undefined })
      Message.success('更新成功')
    } else {
      await request.post('/admin/plans', formData)
      Message.success('创建成功')
    }
    showModal.value = false
    fetchPlans()
  } catch { /* interceptor handles error toast */ } finally { formLoading.value = false }
}

async function toggleRecommend(row: any) {
  try { await request.put(`/admin/plans/${row.id}/toggle-recommend`); Message.success('操作成功'); fetchPlans() }
  catch { /* interceptor handles error toast */ }
}

function archivePlan(row: any) {
  Modal.warning({
    title: '确认下架',
    content: `确定要下架套餐「${row.name}」吗？`,
    okText: '确认', cancelText: '取消',
    onOk: async () => {
      try { await request.delete(`/admin/plans/${row.id}`); Message.success('已下架'); fetchPlans() }
      catch { /* interceptor handles error toast */ }
    },
  })
}

onMounted(fetchPlans)

const { exporting, exportFile } = useExport({
  url: '/admin/plans/export',
  getFilters: () => ({
    status: statusFilter.value,
  }),
})
</script>

<template>
  <div>
    <PageHeader title="套餐管理" description="管理平台套餐定义和定价">
      <template #actions>
        <AButton type="primary" @click="openCreateModal">创建套餐</AButton>
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
          <span>套餐列表</span>
          <ASelect v-model="statusFilter" :options="statusOptions" style="width: 120px" allow-clear @change="() => { pagination.current = 1; fetchPlans() }" />
        </div>
      </template>
      <ATable :columns="columns" :data="plans" :loading="loading" row-key="id" :scroll="{ x: 1200 }" :pagination="false" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchPlans" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchPlans() }" />
      </div>
    </ACard>

    <ADrawer v-model:visible="showModal" :title="editingId ? '编辑套餐' : '创建套餐'" :width="520" :mask-closable="false" :footer="true" unmount-on-close>
      <AForm ref="formRef" :model="formData" :auto-label-width="true" layout="vertical">
        <AFormItem label="名称" required><AInput v-model="formData.name" placeholder="如：专业版" /></AFormItem>
        <AFormItem label="标识" required><AInput v-model="formData.identifier" placeholder="如：pro" :disabled="!!editingId" /></AFormItem>
        <AFormItem label="描述"><AInput v-model="formData.description" type="textarea" :auto-size="{ minRows: 2, maxRows: 4 }" placeholder="面向用户的营销文案" /></AFormItem>
        <AFormItem label="月价格(CNY)"><AInputNumber v-model="formData.monthly_price" :min="0" :precision="2" class="w-full" /></AFormItem>
        <AFormItem label="年价格(CNY)"><AInputNumber v-model="formData.yearly_price" :min="0" :precision="2" class="w-full" /></AFormItem>
        <AFormItem label="月Token配额"><AInputNumber v-model="formData.monthly_quota_tokens" :min="0" class="w-full" placeholder="0 = 不限制" /></AFormItem>
        <AFormItem label="排序权重"><AInputNumber v-model="formData.sort_order" :min="0" class="w-full" placeholder="越小越靠前" /></AFormItem>
        <AFormItem label="推荐"><ASwitch v-model="formData.is_recommended" /></AFormItem>
      </AForm>
      <template #footer>
        <Space>
          <AButton @click="showModal = false">取消</AButton>
          <AButton type="primary" :loading="formLoading" @click="handleSubmit">确认</AButton>
        </Space>
      </template>
    </ADrawer>
  </div>
</template>
