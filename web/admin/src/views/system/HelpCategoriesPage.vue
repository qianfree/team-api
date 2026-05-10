<script setup lang="ts">
import { ref, reactive, h, onMounted } from 'vue'
import { Tag, Button, Space, Message, Modal } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const categories = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 60 },
  { title: '名称', dataIndex: 'name', width: 160, ellipsis: true },
  { title: 'Slug', dataIndex: 'slug', width: 140, ellipsis: true },
  { title: '描述', dataIndex: 'description', width: 200, ellipsis: true },
  { title: '排序', dataIndex: 'sort_order', width: 70 },
  { title: '文章数', dataIndex: 'article_count', width: 80 },
  { title: '可见', dataIndex: 'is_visible', width: 70, render({ record }) {
    return h(Tag, { color: record.is_visible ? 'green' : undefined, size: 'small' }, () => record.is_visible ? '可见' : '隐藏')
  }},
  { title: '创建时间', dataIndex: 'created_at', width: 160 },
  {
    title: '操作', dataIndex: 'actions', width: 160, fixed: 'right',
    render({ record }) {
      return h(Space, { size: 'small' }, () => [
        h(Button, { size: 'small', type: 'text', onClick: () => openEdit(record) }, () => '编辑'),
        h(Button, { size: 'small', type: 'text', status: 'danger', onClick: () => deleteCategory(record) }, () => '删除'),
      ])
    },
  },
]

async function fetchList() {
  loading.value = true
  try {
    const res = await request.get('/admin/help-categories', { params: { page: pagination.current, page_size: pagination.pageSize, parent_id: -1 } })
    const data = res.data?.data
    categories.value = data?.list || []
    pagination.total = data?.total || 0
  } catch {} finally { loading.value = false }
}

// Create / Edit
const showDrawer = ref(false)
const editingId = ref<number | null>(null)
const formLoading = ref(false)
const form = reactive({ parent_id: 0, name: '', slug: '', description: '', sort_order: 0, icon: '', is_visible: true as boolean })

function resetForm() { Object.assign(form, { parent_id: 0, name: '', slug: '', description: '', sort_order: 0, icon: '', is_visible: true }) }

function openCreate() { editingId.value = null; resetForm(); showDrawer.value = true }

function openEdit(row: any) {
  editingId.value = row.id
  Object.assign(form, { parent_id: row.parent_id || 0, name: row.name, slug: row.slug, description: row.description || '', sort_order: row.sort_order || 0, icon: row.icon || '', is_visible: row.is_visible !== false })
  showDrawer.value = true
}

async function handleSubmit() {
  if (!form.name.trim()) { Message.warning('请输入名称'); return }
  if (!form.slug.trim()) { Message.warning('请输入 Slug'); return }
  formLoading.value = true
  try {
    if (editingId.value) { await request.put(`/admin/help-categories/${editingId.value}`, form); Message.success('更新成功') }
    else { await request.post('/admin/help-categories', form); Message.success('创建成功') }
    showDrawer.value = false; fetchList()
  } catch {} finally { formLoading.value = false }
}

async function deleteCategory(row: any) {
  Modal.confirm({ title: '确认删除', content: `确定删除分类「${row.name}」？`, okText: '删除', cancelText: '取消', onOk: async () => {
    try { await request.delete(`/admin/help-categories/${row.id}`); Message.success('删除成功'); fetchList() } catch {}
  }})
}

onMounted(fetchList)
</script>

<template>
  <div>
    <PageHeader title="帮助分类" description="管理帮助中心的文章分类">
      <template #actions>
        <AButton type="primary" @click="openCreate">创建分类</AButton>
      </template>
    </PageHeader>
    <ACard :bordered="false">
      <ATable :columns="columns" :data="categories" :loading="loading" row-key="id" :scroll="{ x: 1100 }" :pagination="false" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" show-page-size @change="fetchList" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchList() }" />
      </div>
    </ACard>

    <ADrawer v-model:visible="showDrawer" :title="editingId ? '编辑分类' : '创建分类'" :width="480" :mask-closable="false" :footer="true" unmount-on-close>
      <AForm :model="form" :auto-label-width="true" layout="vertical">
        <AFormItem label="名称" required><AInput v-model="form.name" placeholder="分类名称" /></AFormItem>
        <AFormItem label="Slug" required><AInput v-model="form.slug" placeholder="url-friendly-identifier" /></AFormItem>
        <AFormItem label="描述"><ATextarea v-model="form.description" :auto-size="{ minRows: 2, maxRows: 4 }" placeholder="分类描述" /></AFormItem>
        <AFormItem label="排序"><AInputNumber v-model="form.sort_order" :min="0" placeholder="0" class="w-full" /></AFormItem>
        <AFormItem label="图标"><AInput v-model="form.icon" placeholder="图标名称（可选）" /></AFormItem>
        <AFormItem label="对外可见"><ASwitch v-model="form.is_visible" /></AFormItem>
      </AForm>
      <template #footer>
        <Space><AButton @click="showDrawer = false">取消</AButton><AButton type="primary" :loading="formLoading" @click="handleSubmit">确认</AButton></Space>
      </template>
    </ADrawer>
  </div>
</template>
