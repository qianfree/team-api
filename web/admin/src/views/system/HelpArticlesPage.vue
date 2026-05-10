<script setup lang="ts">
import { ref, reactive, h, onMounted } from 'vue'
import { Tag, Button, Space, Message, Modal } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const articles = ref<any[]>([])
const categories = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })

const statusFilter = ref<string | undefined>(undefined)
const categoryFilter = ref<number | undefined>(undefined)

const statusOptions = [
  { label: '全部', value: '' },
  { label: '草稿', value: 'draft' },
  { label: '已发布', value: 'published' },
]

function categoryName(id: number) {
  return categories.value.find((c: any) => c.id === id)?.name || '-'
}

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 60 },
  { title: '标题', dataIndex: 'title', width: 200, ellipsis: true },
  { title: '分类', dataIndex: 'category_id', width: 120, render({ record }) { return categoryName(record.category_id) }},
  { title: 'Slug', dataIndex: 'slug', width: 140, ellipsis: true },
  { title: '状态', dataIndex: 'status', width: 80, render({ record }) {
    return h(Tag, { color: record.status === 'published' ? 'green' : undefined, size: 'small' }, () => record.status === 'published' ? '已发布' : '草稿')
  }},
  { title: '浏览量', dataIndex: 'view_count', width: 80 },
  { title: '排序', dataIndex: 'sort_order', width: 70 },
  { title: '发布时间', dataIndex: 'published_at', width: 160, render({ record }) { return record.published_at || '-' }},
  { title: '创建时间', dataIndex: 'created_at', width: 160 },
  {
    title: '操作', dataIndex: 'actions', width: 200, fixed: 'right',
    render({ record }) {
      return h(Space, { size: 'small' }, () => [
        h(Button, { size: 'small', type: 'text', onClick: () => openEdit(record) }, () => '编辑'),
        record.status === 'draft'
          ? h(Button, { size: 'small', type: 'text', status: 'success', onClick: () => publishArticle(record) }, () => '发布')
          : null,
        h(Button, { size: 'small', type: 'text', status: 'danger', onClick: () => deleteArticle(record) }, () => '删除'),
      ])
    },
  },
]

async function fetchCategories() {
  try {
    const res = await request.get('/admin/help-categories', { params: { page: 1, page_size: 100, parent_id: -1 } })
    categories.value = res.data?.data?.list || []
  } catch {}
}

async function fetchList() {
  loading.value = true
  try {
    const params: any = { page: pagination.current, page_size: pagination.pageSize }
    if (statusFilter.value) params.status = statusFilter.value
    if (categoryFilter.value) params.category_id = categoryFilter.value
    const res = await request.get('/admin/help-articles', { params })
    const data = res.data?.data
    articles.value = data?.list || []
    pagination.total = data?.total || 0
  } catch {} finally { loading.value = false }
}

// Create / Edit
const showDrawer = ref(false)
const editingId = ref<number | null>(null)
const formLoading = ref(false)
const detailLoading = ref(false)
const form = reactive({
  category_id: 0,
  title: '',
  slug: '',
  content: '',
  summary: '',
  status: 'draft' as string,
  sort_order: 0,
  keywords: [] as string[],
})
const keywordInput = ref('')

function resetForm() { Object.assign(form, { category_id: 0, title: '', slug: '', content: '', summary: '', status: 'draft', sort_order: 0, keywords: [] }); keywordInput.value = '' }

function openCreate() { editingId.value = null; resetForm(); showDrawer.value = true }

async function openEdit(row: any) {
  editingId.value = row.id
  detailLoading.value = true
  showDrawer.value = true
  try {
    const res = await request.get(`/admin/help-articles/${row.id}`)
    const d = res.data?.data
    Object.assign(form, { category_id: d.category_id, title: d.title, slug: d.slug, content: d.content || '', summary: d.summary || '', status: d.status || 'draft', sort_order: d.sort_order || 0, keywords: d.keywords || [] })
  } catch { showDrawer.value = false } finally { detailLoading.value = false }
}

function addKeyword() {
  const kw = keywordInput.value.trim()
  if (kw && !form.keywords.includes(kw)) form.keywords.push(kw)
  keywordInput.value = ''
}

function removeKeyword(idx: number) { form.keywords.splice(idx, 1) }

async function handleSubmit() {
  if (!form.title.trim()) { Message.warning('请输入标题'); return }
  if (!form.slug.trim()) { Message.warning('请输入 Slug'); return }
  if (!form.content.trim()) { Message.warning('请输入内容'); return }
  formLoading.value = true
  try {
    if (editingId.value) { await request.put(`/admin/help-articles/${editingId.value}`, form); Message.success('更新成功') }
    else { await request.post('/admin/help-articles', form); Message.success('创建成功') }
    showDrawer.value = false; fetchList()
  } catch {} finally { formLoading.value = false }
}

async function publishArticle(row: any) {
  Modal.confirm({ title: '确认发布', content: `确定发布文章「${row.title}」？`, okText: '发布', cancelText: '取消', onOk: async () => {
    try { await request.put(`/admin/help-articles/${row.id}`, { ...row, status: 'published' }); Message.success('发布成功'); fetchList() } catch {}
  }})
}

async function deleteArticle(row: any) {
  Modal.confirm({ title: '确认删除', content: `确定删除文章「${row.title}」？`, okText: '删除', cancelText: '取消', onOk: async () => {
    try { await request.delete(`/admin/help-articles/${row.id}`); Message.success('删除成功'); fetchList() } catch {}
  }})
}

onMounted(() => { fetchCategories(); fetchList() })
</script>

<template>
  <div>
    <PageHeader title="帮助文章" description="管理帮助中心的文章内容">
      <template #actions>
        <AButton type="primary" @click="openCreate">创建文章</AButton>
      </template>
    </PageHeader>

    <ACard :bordered="false">
      <template #title>
        <div class="flex items-center justify-between w-full">
          <span>文章列表</span>
          <Space>
            <ASelect v-model="categoryFilter" :options="categories.map((c: any) => ({ value: c.id, label: c.name }))" style="width: 140px" allow-clear placeholder="按分类筛选" @change="() => { pagination.current = 1; fetchList() }" />
            <ASelect v-model="statusFilter" :options="statusOptions" style="width: 120px" allow-clear placeholder="状态筛选" @change="() => { pagination.current = 1; fetchList() }" />
          </Space>
        </div>
      </template>
      <ATable :columns="columns" :data="articles" :loading="loading" row-key="id" :scroll="{ x: 1300 }" :pagination="false" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" show-page-size @change="fetchList" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchList() }" />
      </div>
    </ACard>

    <ADrawer v-model:visible="showDrawer" :title="editingId ? '编辑文章' : '创建文章'" :width="680" :mask-closable="false" :footer="true" unmount-on-close>
      <ASpin :loading="detailLoading" class="w-full">
        <AForm :model="form" :auto-label-width="true" layout="vertical">
          <AFormItem label="标题" required><AInput v-model="form.title" placeholder="文章标题" /></AFormItem>
          <AFormItem label="Slug" required><AInput v-model="form.slug" placeholder="url-friendly-identifier" /></AFormItem>
          <div class="flex gap-4">
            <AFormItem label="分类" required class="flex-1">
              <ASelect v-model="form.category_id" :options="categories.map((c: any) => ({ value: c.id, label: c.name }))" placeholder="选择分类" />
            </AFormItem>
            <AFormItem label="状态" class="w-40">
              <ASelect v-model="form.status" :options="[{ label: '草稿', value: 'draft' }, { label: '已发布', value: 'published' }]" />
            </AFormItem>
            <AFormItem label="排序" class="w-28">
              <AInputNumber v-model="form.sort_order" :min="0" class="w-full" />
            </AFormItem>
          </div>
          <AFormItem label="摘要"><ATextarea v-model="form.summary" :auto-size="{ minRows: 2, maxRows: 4 }" placeholder="文章摘要（可选）" /></AFormItem>
          <AFormItem label="关键词">
            <div class="flex gap-2 mb-2 flex-wrap" v-if="form.keywords.length">
              <ATag v-for="(kw, idx) in form.keywords" :key="idx" closable @close="removeKeyword(idx)">{{ kw }}</ATag>
            </div>
            <div class="flex gap-2">
              <AInput v-model="keywordInput" placeholder="输入关键词后回车" @press-enter="addKeyword" />
              <AButton @click="addKeyword">添加</AButton>
            </div>
          </AFormItem>
          <AFormItem label="内容" required>
            <ATextarea v-model="form.content" :auto-size="{ minRows: 12, maxRows: 40 }" placeholder="文章内容（支持 Markdown）" />
          </AFormItem>
        </AForm>
      </ASpin>
      <template #footer>
        <Space><AButton @click="showDrawer = false">取消</AButton><AButton type="primary" :loading="formLoading" @click="handleSubmit">确认</AButton></Space>
      </template>
    </ADrawer>
  </div>
</template>
