<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Space, Message, Modal,
} from '@arco-design/web-vue'
import type { TableColumnData, FormInstance } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const levels = ref<any[]>([])

const columns: TableColumnData[] = [
  { title: '等级号', dataIndex: 'level', width: 80 },
  { title: '名称', dataIndex: 'name', width: 120 },
  {
    title: '累计充值阈值(USD)', dataIndex: 'cumulative_recharge_threshold', width: 180,
    render({ record }) { return `$${Number(record.cumulative_recharge_threshold).toFixed(2)}` },
  },
  {
    title: '最大成员数', dataIndex: 'max_members', width: 100,
    render({ record }) { return record.max_members === 0 ? '不限' : record.max_members },
  },
  {
    title: '最大并发数', dataIndex: 'max_concurrency', width: 100,
    render({ record }) { return record.max_concurrency === 0 ? '不限' : record.max_concurrency },
  },
  {
    title: '价格乘数', dataIndex: 'price_multiplier', width: 100,
    render({ record }) {
      const v = Number(record.price_multiplier)
      if (v === 1) return '-'
      return `${(v * 100).toFixed(1)}%`
    },
  },
  { title: '排序', dataIndex: 'sort_order', width: 70 },
  {
    title: '操作', dataIndex: 'actions', width: 150, fixed: 'right',
    render({ record }) {
      return h(Space, { size: 4 }, () => [
        h(Button, { size: 'small', type: 'primary', onClick: () => openEditModal(record) }, () => '编辑'),
        record.level === 1 ? null : h(Button, { size: 'small', status: 'danger', onClick: () => deleteLevel(record) }, () => '删除'),
      ])
    },
  },
]

async function fetchLevels() {
  loading.value = true
  try {
    const res = await request.get('/admin/tenant-level-configs')
    levels.value = res.data?.data?.list || []
  } catch { /* interceptor handles error toast */ } finally { loading.value = false }
}

const showModal = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance | null>(null)
const formLoading = ref(false)
const formData = reactive({
  level: 1,
  name: '',
  cumulative_recharge_threshold: 0,
  max_members: 5,
  max_concurrency: 10,
  price_multiplier: 1.0,
  sort_order: 0,
})

function resetForm() {
  formData.level = 1
  formData.name = ''
  formData.cumulative_recharge_threshold = 0
  formData.max_members = 5
  formData.max_concurrency = 10
  formData.price_multiplier = 1.0
  formData.sort_order = 0
}

function openCreateModal() {
  editingId.value = null
  resetForm()
  showModal.value = true
}

function openEditModal(row: any) {
  editingId.value = row.id
  formData.level = row.level
  formData.name = row.name
  formData.cumulative_recharge_threshold = row.cumulative_recharge_threshold
  formData.max_members = row.max_members
  formData.max_concurrency = row.max_concurrency
  formData.price_multiplier = row.price_multiplier
  formData.sort_order = row.sort_order
  showModal.value = true
}

async function handleSubmit() {
  formLoading.value = true
  try {
    if (editingId.value) {
      await request.put(`/admin/tenant-level-configs/${editingId.value}`, {
        name: formData.name,
        cumulative_recharge_threshold: formData.cumulative_recharge_threshold,
        max_members: formData.max_members,
        max_concurrency: formData.max_concurrency,
        price_multiplier: formData.price_multiplier,
        sort_order: formData.sort_order,
      })
      Message.success('更新成功')
    } else {
      await request.post('/admin/tenant-level-configs', formData)
      Message.success('创建成功')
    }
    showModal.value = false
    fetchLevels()
  } catch { /* interceptor handles error toast */ } finally { formLoading.value = false }
}

function deleteLevel(row: any) {
  Modal.warning({
    title: '确认删除',
    content: `确定要删除等级「${row.name}」吗？`,
    okText: '确认', cancelText: '取消',
    onOk: async () => {
      try { await request.delete(`/admin/tenant-level-configs/${row.id}`); Message.success('已删除'); fetchLevels() }
      catch { /* interceptor handles error toast */ }
    },
  })
}

onMounted(fetchLevels)
</script>

<template>
  <div>
    <PageHeader title="租户级别" description="配置租户等级体系，基于累计充值自动升级">
      <template #actions>
        <AButton type="primary" @click="openCreateModal">创建等级</AButton>
      </template>
    </PageHeader>

    <ACard :bordered="false">
      <template #title>等级配置列表</template>
      <ATable :columns="columns" :data="levels" :loading="loading" row-key="id" :scroll="{ x: 900 }" :pagination="false" />
    </ACard>

    <ADrawer v-model:visible="showModal" :title="editingId ? '编辑等级' : '创建等级'" :width="520" :mask-closable="false" :footer="true" unmount-on-close>
      <AForm ref="formRef" :model="formData" :auto-label-width="true" layout="vertical">
        <AFormItem label="等级号" required>
          <AInputNumber v-model="formData.level" :min="1" :max="99" class="w-full" :disabled="!!editingId" placeholder="1, 2, 3..." />
        </AFormItem>
        <AFormItem label="等级名称" required>
          <AInput v-model="formData.name" placeholder="如：LV2 白银用户" />
        </AFormItem>
        <AFormItem label="累计充值阈值(USD)" required>
          <AInputNumber v-model="formData.cumulative_recharge_threshold" :min="0" :precision="2" class="w-full" placeholder="达到此金额自动升级" />
        </AFormItem>
        <AFormItem label="最大成员数">
          <AInputNumber v-model="formData.max_members" :min="0" class="w-full" placeholder="0 = 不限制" />
        </AFormItem>
        <AFormItem label="最大并发数">
          <AInputNumber v-model="formData.max_concurrency" :min="0" class="w-full" placeholder="0 = 不限制" />
        </AFormItem>
        <AFormItem label="价格乘数">
          <AInputNumber v-model="formData.price_multiplier" :min="0.01" :max="2" :step="0.05" :precision="4" class="w-full" placeholder="1.0 = 原价，0.9 = 九折" />
        </AFormItem>
        <AFormItem label="排序权重">
          <AInputNumber v-model="formData.sort_order" :min="0" class="w-full" />
        </AFormItem>
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
