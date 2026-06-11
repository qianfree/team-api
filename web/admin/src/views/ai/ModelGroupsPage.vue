<script setup lang="ts">
import { ref, reactive, computed, onMounted, h } from 'vue'
import { Tag, Button, Space, Popconfirm, Message } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50],
})

const filterSearch = ref('')
const filterStatus = ref<string | null>(null)

const hasDefaultGroup = computed(() => {
  return data.value.some((item: any) => item.is_default && item.status === 'active')
})

async function fetchData() {
  loading.value = true
  try {
    const params: any = {
      page: pagination.current,
      page_size: pagination.pageSize,
    }
    if (filterSearch.value) params.search = filterSearch.value
    if (filterStatus.value) params.status = filterStatus.value
    const res: any = await request.get('/admin/model-groups', { params })
    data.value = res.data?.data?.list || res.data?.list || []
    pagination.total = res.data?.data?.total || res.data?.total || 0
  } catch {
  } finally {
    loading.value = false
  }
}

function handlePageChange(page: number) {
  pagination.current = page
  fetchData()
}

function handlePageSizeChange(pageSize: number) {
  pagination.pageSize = pageSize
  pagination.current = 1
  fetchData()
}

function resetFilter() {
  filterSearch.value = ''
  filterStatus.value = null
  pagination.current = 1
  fetchData()
}

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 60 },
  { title: '分组名称', dataIndex: 'name', width: 160, ellipsis: true },
  { title: '标识', dataIndex: 'code', width: 140, ellipsis: true },
  { title: '描述', dataIndex: 'description', width: 200, ellipsis: true },
  {
    title: '状态', dataIndex: 'status', width: 80,
    render({ record }) {
      const color = record.status === 'active' ? 'green' : undefined
      const label = record.status === 'active' ? '启用' : '禁用'
      return h(Tag, { color, size: 'small' }, () => label)
    },
  },
  {
    title: '默认', dataIndex: 'is_default', width: 70,
    render({ record }) {
      return record.is_default ? h(Tag, { color: 'blue', size: 'small' }, () => '默认') : '-'
    },
  },
  { title: '模型数', dataIndex: 'model_count', width: 80 },
  { title: '租户数', dataIndex: 'tenant_count', width: 80 },
  {
    title: '创建时间', dataIndex: 'created_at', width: 170,
    render({ record }) {
      return record.created_at ? new Date(record.created_at).toLocaleString() : '-'
    },
  },
  {
    title: '操作', dataIndex: 'actions', width: 220, fixed: 'right',
    render({ record }) {
      return h(Space, { size: 4 }, () => [
        h(Button, { size: 'small', onClick: () => openModelsModal(record) }, () => '管理模型'),
        h(Button, { size: 'small', onClick: () => openEdit(record) }, () => '编辑'),
        h(Popconfirm, { content: '确定删除该分组？', onOk: () => handleDelete(record) }, () =>
          h(Button, { size: 'small', status: 'danger' }, () => '删除')
        ),
      ])
    },
  },
]

// ==================== 分组 CRUD ====================

const showModal = ref(false)
const modalTitle = ref('创建分组')
const formLoading = ref(false)
const editingId = ref<number | null>(null)

const form = reactive({
  name: '',
  code: '',
  description: '',
  is_default: false,
})

function resetForm() {
  form.name = ''
  form.code = ''
  form.description = ''
  form.is_default = false
}

function openCreate() {
  resetForm()
  editingId.value = null
  modalTitle.value = '创建分组'
  showModal.value = true
}

function openEdit(row: any) {
  form.name = row.name || ''
  form.code = row.code || ''
  form.description = row.description || ''
  form.is_default = !!row.is_default
  editingId.value = row.id
  modalTitle.value = '编辑分组'
  showModal.value = true
}

async function handleSubmit(done: () => void) {
  if (!form.name.trim()) {
    Message.warning('请输入分组名称')
    return
  }
  formLoading.value = true
  try {
    const payload: any = { name: form.name, description: form.description, is_default: form.is_default }
    if (editingId.value) {
      await request.put(`/admin/model-groups/${editingId.value}`, payload)
      Message.success('更新成功')
    } else {
      if (!form.code.trim()) {
        Message.warning('请输入分组标识')
        formLoading.value = false
        return
      }
      payload.code = form.code
      await request.post('/admin/model-groups', payload)
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

async function handleDelete(row: any) {
  try {
    await request.delete(`/admin/model-groups/${row.id}`)
    Message.success('删除成功')
    fetchData()
  } catch {
  }
}

// ==================== 模型管理弹窗 ====================

const showModelsModal = ref(false)
const modelsLoading = ref(false)
const groupModels = ref<any[]>([])
const allModels = ref<any[]>([])
const selectedModelIds = ref<number[]>([])
const currentGroup = ref<any>(null)

async function fetchAllModels() {
  try {
    const res: any = await request.get('/admin/models/options')
    allModels.value = res.data?.data?.list || res.data?.list || []
  } catch {
  }
}

async function openModelsModal(row: any) {
  currentGroup.value = row
  selectedModelIds.value = []
  showModelsModal.value = true
  modelsLoading.value = true
  try {
    const [groupRes] = await Promise.all([
      request.get(`/admin/model-groups/${row.id}/models`),
      allModels.value.length === 0 ? fetchAllModels() : Promise.resolve(),
    ])
    groupModels.value = (groupRes as any).data?.data?.list || (groupRes as any).data?.list || []
    selectedModelIds.value = groupModels.value.map((m: any) => {
      const found = allModels.value.find((am: any) => am.model_id === m.model_id)
      return found ? found.id : 0
    }).filter((id: number) => id > 0)
  } catch {
  } finally {
    modelsLoading.value = false
  }
}

const transferOptions = computed(() => {
  return allModels.value.map((m: any) => ({
    value: m.id,
    label: `${m.model_id}${m.model_name ? ` (${m.model_name})` : ''}`,
  }))
})

async function handleSaveModels(done: () => void) {
  if (!currentGroup.value) return
  modelsLoading.value = true
  try {
    await request.put(`/admin/model-groups/${currentGroup.value.id}/models`, { model_ids: selectedModelIds.value })
    Message.success('模型更新成功')
    done()
    fetchData()
  } catch {
    return false
  } finally {
    modelsLoading.value = false
  }
}

onMounted(() => {
  fetchData()
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="模型分组" description="通过分组批量管理租户可用的模型，新增模型加入分组后自动对组内租户生效">
    </PageHeader>

    <AAlert v-if="!loading && !hasDefaultGroup" type="warning" class="mb-4" closable>
      当前未设置默认模型组，新注册租户将无法使用任何模型。请创建一个分组并标记为「默认」。
    </AAlert>

    <ACard :bordered="false" class="mb-4">
      <ARow :gutter="16" align="center">
        <ACol :span="8">
          <AInput v-model="filterSearch" placeholder="搜索分组名称或标识" allow-clear @clear="resetFilter" @press-enter="() => { pagination.current = 1; fetchData() }" />
        </ACol>
        <ACol :span="4">
          <ASelect v-model="filterStatus" placeholder="全部状态" allow-clear @change="() => { pagination.current = 1; fetchData() }">
            <AOption value="active">启用</AOption>
            <AOption value="disabled">禁用</AOption>
          </ASelect>
        </ACol>
        <ACol :span="4">
          <AButton @click="() => { pagination.current = 1; fetchData() }">查询</AButton>
        </ACol>
        <ACol :span="8" style="text-align: right">
          <AButton type="primary" @click="openCreate">创建分组</AButton>
        </ACol>
      </ARow>
    </ACard>

    <ACard :bordered="false">
      <ATable
        :columns="columns"
        :data="data"
        :loading="loading"
        :bordered="false"
        :stripe="true"
        :pagination="false"
        row-key="id"
        :scroll="{ x: 1200 }"
      />
      <div class="table-footer">
        <APagination
          v-model:current="pagination.current"
          v-model:page-size="pagination.pageSize"
          :total="pagination.total"
          :show-page-size="pagination.showPageSize"
          :page-size-options="pagination.pageSizeOptions"
          show-total
          @change="handlePageChange"
          @page-size-change="handlePageSizeChange"
        />
      </div>
    </ACard>

    <!-- 创建/编辑分组弹窗 -->
    <AModal
      v-model:visible="showModal"
      :title="modalTitle"
      :on-before-ok="handleSubmit"
      :ok-loading="formLoading"
    >
      <AForm :model="form" layout="vertical">
        <AFormItem label="分组名称" required>
          <AInput v-model="form.name" placeholder="如：全量模型、基础对话" />
        </AFormItem>
        <AFormItem v-if="!editingId" label="分组标识" required>
          <AInput v-model="form.code" placeholder="如：full_access、basic_chat" />
        </AFormItem>
        <AFormItem label="描述">
          <ATextarea v-model="form.description" placeholder="可选，描述分组用途" :auto-size="{ minRows: 2, maxRows: 4 }" />
        </AFormItem>
        <AFormItem label="设为新租户默认分组">
          <ASwitch v-model="form.is_default" />
          <span style="margin-left: 8px; color: var(--color-text-3); font-size: 12px">开启后，新注册的租户将自动关联此分组</span>
        </AFormItem>
      </AForm>
    </AModal>

    <!-- 模型管理弹窗 -->
    <AModal
      v-model:visible="showModelsModal"
      :title="`管理模型 - ${currentGroup?.name || ''}`"
      :width="750"
      :on-before-ok="handleSaveModels"
      :ok-loading="modelsLoading"
    >
      <ASpin :loading="modelsLoading">
        <ATransfer
          v-model="selectedModelIds"
          :data="transferOptions"
          :title="['可选模型', '已选模型']"
          searchable
        />
      </ASpin>
    </AModal>
  </div>
</template>

<style scoped>
.page-table {
  padding: 0 20px 20px;
}
.table-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
.mb-4 {
  margin-bottom: 16px;
}
</style>
