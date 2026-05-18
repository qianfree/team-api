<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Space, Popconfirm, Message,
} from '@arco-design/web-vue'
import type { TableColumnData, FormInstance } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

const loading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50],
})

const filterCategory = ref<string | null>(null)
const filterStatus = ref<string | null>(null)
const filterSearch = ref('')

const categoryOptions = [
  { label: '全部分类', value: '' },
  { label: '对话', value: 'chat' },
  { label: '向量', value: 'embedding' },
  { label: '图像', value: 'image' },
  { label: '音频', value: 'audio' },
  { label: '视频', value: 'video' },
  { label: '重排', value: 'rerank' },
]

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '启用', value: 'active' },
  { label: '弃用', value: 'deprecated' },
  { label: '下线', value: 'offline' },
]

const showModal = ref(false)
const modalTitle = ref('创建模型')
const formRef = ref<FormInstance | null>(null)
const formLoading = ref(false)
const editingId = ref<number | null>(null)
const editingVersion = ref(0)

const form = reactive({
  model_id: '',
  model_name: '',
  category: 'chat',
  max_context_tokens: null as number | null,
  max_output_tokens: null as number | null,
  capabilities: {} as Record<string, boolean>,
  description: '',
  tags: [] as string[],
  status: 'active',
  sunset_date: '',
  replacement_model: '',
})

const fetchingInfo = ref(false)

const capabilityOptions = [
  { label: '图片识别', value: 'vision' },
  { label: '工具调用', value: 'function_calling' },
  { label: '并行工具调用', value: 'parallel_function_calling' },
  { label: '工具选择', value: 'tool_choice' },
  { label: '结构化输出', value: 'response_schema' },
  { label: '系统消息', value: 'system_messages' },
  { label: '提示词缓存', value: 'prompt_caching' },
  { label: '音频输入', value: 'audio_input' },
  { label: '音频输出', value: 'audio_output' },
  { label: 'PDF 输入', value: 'pdf_input' },
  { label: '图像嵌入', value: 'embedding_image' },
  { label: '深度思考', value: 'reasoning' },
  { label: '联网搜索', value: 'web_search' },
]

async function fetchOfficialInfo() {
  const modelName = form.model_id?.trim()
  if (!modelName) {
    Message.warning('请先输入模型标识')
    return
  }
  fetchingInfo.value = true
  try {
    const res: any = await request.get('/admin/models/official-info', { params: { model_name: modelName } })
    const info = res.data?.data || res.data
    if (!info?.found) {
      if (info?.error) {
        Message.warning(`获取远程数据失败: ${info.error}`)
      } else {
        Message.warning('未找到该模型的官方数据')
      }
      return
    }
    if (info.max_context_tokens) form.max_context_tokens = info.max_context_tokens
    if (info.max_output_tokens) form.max_output_tokens = info.max_output_tokens
    if (info.capabilities) form.capabilities = { ...info.capabilities }
    Message.success('已填入官方数据')
  } catch {
    Message.error('拉取失败')
  } finally {
    fetchingInfo.value = false
  }
}

const categoryTagColor: Record<string, string> = {
  chat: 'arcoblue',
  embedding: 'green',
  image: 'orangered',
  audio: 'red',
  video: 'purple',
  rerank: 'arcoblue',
}

const statusTagColor: Record<string, string | undefined> = {
  active: 'green',
  deprecated: 'orangered',
  offline: undefined,
}

const categoryTagLabel: Record<string, string> = {
  chat: '对话',
  embedding: '向量',
  image: '图像',
  audio: '音频',
  video: '视频',
  rerank: '重排',
}

const statusTagLabel: Record<string, string> = {
  active: '启用',
  deprecated: '弃用',
  offline: '下线',
}

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '模型标识', dataIndex: 'model_id', width: 180, ellipsis: true },
  { title: '显示名', dataIndex: 'model_name', width: 160, ellipsis: true },
  {
    title: '分类',
    dataIndex: 'category',
    width: 100,
    render({ record }) {
      return h(Tag, { color: categoryTagColor[record.category], size: 'small' }, () => categoryTagLabel[record.category] || record.category)
    },
  },
  {
    title: '状态',
    dataIndex: 'status',
    width: 160,
    render({ record }) {
      const tags = [
        h(Tag, { color: statusTagColor[record.status], size: 'small' }, () => statusTagLabel[record.status] || record.status),
      ]
      if (record.status === 'deprecated' && record.sunset_date) {
        tags.push(h(Tag, { color: 'red', size: 'small' }, () => `下线: ${record.sunset_date}`))
      }
      return h('span', { style: 'display: inline-flex; gap: 4px; align-items: center;' }, tags)
    },
  },
  {
    title: '弃用信息',
    dataIndex: 'deprecation',
    width: 180,
    ellipsis: true,
    render({ record }) {
      if (record.status !== 'deprecated') return null
      const parts: string[] = []
      if (record.deprecated_at) parts.push(`弃用于 ${record.deprecated_at}`)
      if (record.replacement_model) parts.push(`替代: ${record.replacement_model}`)
      return h('span', { style: 'font-size: 12px; color: var(--color-text-3); line-height: 1.4;' }, parts.join('\n'))
    },
  },
  { title: '上下文', dataIndex: 'max_context_tokens', width: 100 },
  { title: '输出上限', dataIndex: 'max_output_tokens', width: 100 },
  { title: '更新时间', dataIndex: 'updated_at', width: 170 },
  {
    title: '操作',
    dataIndex: 'actions',
    width: 140,
    fixed: 'right',
    render({ record }) {
      return h(Space, { size: 4 }, () => [
        h(Button, { size: 'small', onClick: () => openEdit(record) }, () => '编辑'),
        h(Popconfirm, {
          content: '确定删除该模型？',
          onOk: () => deleteModel(record),
        }, () => h(Button, { size: 'small', status: 'danger' }, () => '删除')),
      ])
    },
  },
]

async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: pagination.current,
      page_size: pagination.pageSize,
    }
    if (filterCategory.value) params.category = filterCategory.value
    if (filterStatus.value) params.status = filterStatus.value
    if (filterSearch.value) params.search = filterSearch.value

    const res: any = await request.get('/admin/models', { params })
    data.value = res.data?.data?.list || res.data?.list || []
    pagination.total = res.data?.data?.total || res.data?.total || 0
  } catch {
    data.value = []
    pagination.total = 0
  } finally {
    loading.value = false
  }
}

function handleFilter() {
  pagination.current = 1
  fetchData()
}

function openCreate() {
  modalTitle.value = '创建模型'
  editingId.value = null
  editingVersion.value = 0
  form.model_id = ''
  form.model_name = ''
  form.category = 'chat'
  form.max_context_tokens = null
  form.max_output_tokens = null
  form.capabilities = {}
  form.description = ''
  form.tags = []
  form.status = 'active'
  form.sunset_date = ''
  form.replacement_model = ''
  showModal.value = true
}

function openEdit(row: any) {
  modalTitle.value = '编辑模型'
  editingId.value = row.id
  editingVersion.value = row.version || 0
  form.model_id = row.model_id
  form.model_name = row.model_name || ''
  form.category = row.category
  form.max_context_tokens = row.max_context_tokens || null
  form.max_output_tokens = row.max_output_tokens || null
  form.capabilities = row.capabilities || {}
  form.description = row.description || ''
  form.tags = row.tags || []
  form.status = row.status
  form.sunset_date = row.sunset_date || ''
  form.replacement_model = row.replacement_model || ''
  showModal.value = true
}

async function handleSubmit(done: () => void) {
  try {
    const errors = await formRef.value?.validate()
    if (errors) return
  } catch {
    return
  }

  formLoading.value = true
  try {
    const payload: any = { ...form }
    payload.max_context_tokens = form.max_context_tokens ?? 0
    payload.max_output_tokens = form.max_output_tokens ?? 0
    if (editingId.value) {
      payload.version = editingVersion.value
      await request.put(`/admin/models/${editingId.value}`, payload)
      Message.success('更新成功')
    } else {
      await request.post('/admin/models', payload)
      Message.success('创建成功')
    }
    done()
    fetchData()
  } catch {
    return false
  } finally {
    formLoading.value = false
  }
}

async function deleteModel(row: any) {
  try {
    await request.delete(`/admin/models/${row.id}`)
    Message.success('删除成功')
    fetchData()
  } catch {
    // error handled by interceptor
  }
}

onMounted(() => {
  fetchData()
})

const { exporting, exportFile } = useExport({
  url: '/admin/models/export',
  getFilters: () => ({
    category: filterCategory.value,
    status: filterStatus.value,
    search: filterSearch.value,
  }),
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="模型管理" description="管理 AI 模型配置">
      <template #actions>
        <AButton type="primary" @click="openCreate">创建模型</AButton>
        <ADropdown trigger="hover">
          <AButton :loading="exporting">导出</AButton>
          <template #content>
            <ADoption @click="exportFile('csv')">导出 CSV</ADoption>
            <ADoption @click="exportFile('xlsx')">导出 Excel</ADoption>
          </template>
        </ADropdown>
      </template>
    </PageHeader>

    <!-- Filters -->
    <ACard :bordered="false" class="mb-4">
      <ASpace>
        <ASelect
          v-model="filterCategory"
          :options="categoryOptions"
          placeholder="分类"
          allow-clear
          style="width: 140px"
          @change="handleFilter"
        />
        <ASelect
          v-model="filterStatus"
          :options="statusOptions"
          placeholder="状态"
          allow-clear
          style="width: 120px"
          @change="handleFilter"
        />
        <AInput
          v-model="filterSearch"
          placeholder="搜索模型名..."
          allow-clear
          style="width: 200px"
          @keydown.enter="handleFilter"
          @clear="handleFilter"
        />
        <AButton @click="handleFilter">搜索</AButton>
      </ASpace>
    </ACard>

    <!-- Table -->
    <ACard :bordered="false">
      <ATable
        :columns="columns"
        :data="data"
        :loading="loading"
        :scroll="{ x: 1200 }"
        :bordered="false"
        :stripe="true"
        :pagination="false"
        row-key="id"
      />
      <div class="table-footer">
        <APagination
          v-model:current="pagination.current"
          v-model:page-size="pagination.pageSize"
          :total="pagination.total"
          :page-size-options="pagination.pageSizeOptions"
          show-page-size
          @change="fetchData"
          @page-size-change="(size: number) => { pagination.pageSize = size; pagination.current = 1; fetchData() }"
        />
      </div>
    </ACard>

    <!-- Create/Edit Modal -->
    <AModal
      v-model:visible="showModal"
      :title="modalTitle"
      :mask-closable="false"
      :width="550"
      :on-before-ok="handleSubmit"
      :ok-loading="formLoading"
    >
      <AForm ref="formRef" :model="form" :auto-label-width="true" layout="vertical">
        <AFormItem field="model_id" label="模型标识" :rules="[{ required: true, message: '请输入模型标识' }]">
          <AInput v-model="form.model_id" placeholder="gpt-4o" :disabled="!!editingId" />
        </AFormItem>
        <AFormItem label="显示名">
          <AInput v-model="form.model_name" placeholder="GPT-4o" />
        </AFormItem>
        <div style="display: flex; gap: 16px;">
          <AFormItem field="category" label="分类" :rules="[{ required: true, message: '请选择分类' }]" style="flex: 1;">
            <ASelect
              v-model="form.category"
              :options="categoryOptions.filter(o => o.value)"
              placeholder="请选择分类"
            />
          </AFormItem>
          <AFormItem label="官方数据" style="flex: 1;">
            <AButton type="outline" :loading="fetchingInfo" long @click="fetchOfficialInfo">
              拉取官方数据
            </AButton>
          </AFormItem>
        </div>
        <div style="display: flex; gap: 16px;">
          <AFormItem label="最大上下文" style="flex: 1;">
            <AInputNumber v-model="form.max_context_tokens" :min="0" placeholder="如 128000" class="w-full" />
          </AFormItem>
          <AFormItem label="最大输出" style="flex: 1;">
            <AInputNumber v-model="form.max_output_tokens" :min="0" placeholder="如 16384" class="w-full" />
          </AFormItem>
        </div>
        <AFormItem label="模型能力">
          <div class="cap-grid">
            <ACheckbox
              v-for="cap in capabilityOptions"
              :key="cap.value"
              :model-value="!!form.capabilities[cap.value]"
              @change="(v: boolean | (string | number | boolean)[]) => { form.capabilities[cap.value] = v as boolean }"
            >
              {{ cap.label }}
            </ACheckbox>
          </div>
        </AFormItem>
        <AFormItem v-if="editingId" label="状态">
          <ASelect v-model="form.status" :options="statusOptions.filter(o => o.value)" />
        </AFormItem>
        <AFormItem v-if="editingId && form.status === 'deprecated'" label="下线日期">
          <ADatePicker v-model="form.sunset_date" style="width: 100%" placeholder="模型将在此日期自动下线" />
        </AFormItem>
        <AFormItem v-if="editingId && form.status === 'deprecated'" label="替代模型">
          <AInput v-model="form.replacement_model" placeholder="推荐用户迁移到的模型名" />
        </AFormItem>
        <AFormItem label="标签">
          <AInputTag v-model="form.tags" placeholder="输入后回车添加标签" allow-clear />
        </AFormItem>
        <AFormItem label="描述">
          <AInput v-model="form.description" type="textarea" :auto-size="{ minRows: 2, maxRows: 4 }" placeholder="模型描述（选填）" />
        </AFormItem>
      </AForm>
    </AModal>
  </div>
</template>

<style scoped>
.table-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--ta-border-light);
}
.cap-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px 0;
  width: 100%;
}
</style>
