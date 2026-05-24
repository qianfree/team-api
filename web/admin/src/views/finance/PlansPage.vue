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
const allModels = ref<string[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })
const statusFilter = ref<string | undefined>(undefined)
const statusOptions = [
  { label: '全部', value: '' },
  { label: '上架', value: 'active' },
  { label: '下架', value: 'archived' },
]

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 60 },
  { title: '名称', dataIndex: 'name', width: 120 },
  { title: '标识', dataIndex: 'identifier', width: 90 },
  { title: '价格(CNY)', dataIndex: 'price', width: 100, render({ record }) { return `¥${Number(record.price).toFixed(2)}` } },
  { title: '额度(USD)', dataIndex: 'credit_amount', width: 100, render({ record }) { return `$${Number(record.credit_amount).toFixed(2)}` } },
  { title: '赠送(USD)', dataIndex: 'bonus_amount', width: 90, render({ record }) { return record.bonus_amount > 0 ? `$${Number(record.bonus_amount).toFixed(2)}` : '-' } },
  { title: '有效期(天)', dataIndex: 'validity_days', width: 90 },
  { title: '可用模型', dataIndex: 'allowed_models', width: 120, render({ record }) {
    const models = record.allowed_models
    if (!models || models.length === 0) return '全部'
    if (models.length <= 2) return models.join(', ')
    return `${models.slice(0, 2).join(', ')}... (共${models.length}个)`
  } },
  { title: '库存', dataIndex: 'stock', width: 70, render({ record }) { return record.stock === null || record.stock === 0 ? '不限' : record.stock } },
  { title: '已购', dataIndex: 'total_purchased', width: 60 },
  {
    title: '状态', dataIndex: 'status', width: 70,
    render({ record }) { return h(Tag, { color: record.status === 'active' ? 'green' : undefined, size: 'small' }, () => record.status === 'active' ? '上架' : '下架') },
  },
  {
    title: '推荐', dataIndex: 'is_recommended', width: 60,
    render({ record }) { return h(Tag, { color: record.is_recommended ? 'orangered' : undefined, size: 'small' }, () => record.is_recommended ? '推荐' : '-') },
  },
  { title: '排序', dataIndex: 'sort_order', width: 60 },
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

async function fetchAllModels() {
  try {
    const names: string[] = []
    let page = 1
    while (true) {
      const res = await request.get('/admin/models', { params: { page, page_size: 100 } })
      const list = res.data?.data?.list || []
      for (const m of list) names.push(m.model_id)
      const total = res.data?.data?.total || 0
      if (page * 100 >= total) break
      page++
    }
    allModels.value = names
  } catch { /* ignore */ }
}

const showModal = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance | null>(null)
const formLoading = ref(false)
const formData = reactive({
  name: '', identifier: '', description: '',
  price: 0, credit_amount: 0, bonus_amount: 0, validity_days: 30,
  allowed_models: [] as string[], purchase_limit: 0, purchase_limit_period: 'lifetime',
  stock: null as number | null, sort_order: 0, is_recommended: false,
})

function resetForm() {
  Object.assign(formData, {
    name: '', identifier: '', description: '',
    price: 0, credit_amount: 0, bonus_amount: 0, validity_days: 30,
    allowed_models: [], purchase_limit: 0, purchase_limit_period: 'lifetime',
    stock: null, sort_order: 0, is_recommended: false,
  })
}

function openCreateModal() {
  editingId.value = null
  resetForm()
  showModal.value = true
}

function openEditModal(row: any) {
  editingId.value = row.id
  Object.assign(formData, {
    name: row.name, identifier: row.identifier, description: row.description || '',
    price: row.price, credit_amount: row.credit_amount, bonus_amount: row.bonus_amount || 0,
    validity_days: row.validity_days, allowed_models: row.allowed_models || [],
    purchase_limit: row.purchase_limit || 0, purchase_limit_period: row.purchase_limit_period || 'lifetime',
    stock: row.stock, sort_order: row.sort_order || 0, is_recommended: row.is_recommended || false,
  })
  showModal.value = true
}

async function handleSubmit() {
  formLoading.value = true
  try {
    if (editingId.value) {
      await request.put(`/admin/plans/${editingId.value}`, { update: { ...formData } })
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

onMounted(() => {
  fetchPlans()
  fetchAllModels()
})

const { exporting, exportFile } = useExport({
  url: '/admin/plans/export',
  getFilters: () => ({
    status: statusFilter.value,
  }),
})
</script>

<template>
  <div>
    <PageHeader title="套餐管理" description="管理平台套餐（额度包）定义和定价">
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
      <ATable :columns="columns" :data="plans" :loading="loading" row-key="id" :scroll="{ x: 1400 }" :pagination="false" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchPlans" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchPlans() }" />
      </div>
    </ACard>

    <ADrawer v-model:visible="showModal" :title="editingId ? '编辑套餐' : '创建套餐'" :width="520" :mask-closable="false" :footer="true" unmount-on-close>
      <AForm ref="formRef" :model="formData" :auto-label-width="true" layout="vertical">
        <ADivider>基本信息</ADivider>
        <AFormItem label="名称" required><AInput v-model="formData.name" placeholder="如：基础套餐" /></AFormItem>
        <AFormItem label="标识" required><AInput v-model="formData.identifier" placeholder="如：basic" :disabled="!!editingId" /></AFormItem>
        <AFormItem label="描述"><AInput v-model="formData.description" type="textarea" :auto-size="{ minRows: 2, maxRows: 4 }" placeholder="面向用户的营销文案" /></AFormItem>

        <ADivider>定价与额度</ADivider>
        <AFormItem label="价格(CNY)" required><AInputNumber v-model="formData.price" :min="0.01" :precision="2" class="w-full" placeholder="购买价格" /></AFormItem>
        <AFormItem label="额度(USD)" required><AInputNumber v-model="formData.credit_amount" :min="0.01" :precision="2" class="w-full" placeholder="包含的 USD 额度" /></AFormItem>
        <AFormItem label="赠送额度(USD)"><AInputNumber v-model="formData.bonus_amount" :min="0" :precision="2" class="w-full" placeholder="额外赠送额度" /></AFormItem>
        <AFormItem label="有效天数" required><AInputNumber v-model="formData.validity_days" :min="1" class="w-full" placeholder="从激活时起算" /></AFormItem>

        <ADivider>使用限制</ADivider>
        <AFormItem label="可用模型">
          <ASelect v-model="formData.allowed_models" :options="allModels.map(m => ({ label: m, value: m }))" multiple allow-clear filterable placeholder="留空 = 全部模型" />
        </AFormItem>
        <AGrid :cols="2" :col-gap="12">
          <AFormItem label="限购数量"><AInputNumber v-model="formData.purchase_limit" :min="0" class="w-full" placeholder="0 = 不限购" /></AFormItem>
          <AFormItem label="限购周期">
            <ASelect v-model="formData.purchase_limit_period">
              <AOption value="lifetime">终身</AOption>
              <AOption value="monthly">每月</AOption>
              <AOption value="yearly">每年</AOption>
            </ASelect>
          </AFormItem>
        </AGrid>
        <AFormItem label="库存"><AInputNumber v-model="formData.stock" :min="0" class="w-full" placeholder="留空 = 不限库存" /></AFormItem>

        <ADivider>其他设置</ADivider>
        <AGrid :cols="2" :col-gap="12">
          <AFormItem label="排序权重"><AInputNumber v-model="formData.sort_order" :min="0" class="w-full" placeholder="越小越靠前" /></AFormItem>
          <AFormItem label="推荐"><ASwitch v-model="formData.is_recommended" /></AFormItem>
        </AGrid>
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
