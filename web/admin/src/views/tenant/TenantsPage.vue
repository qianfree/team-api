<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import {
  Tag, Button, Space, Message, InputNumber,
} from '@arco-design/web-vue'
import type { TableColumnData, FormInstance } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

const router = useRouter()

const loading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50],
})

const filterStatus = ref<string | null>(null)
const filterSearch = ref('')

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '正常', value: 'active' },
  { label: '暂停', value: 'suspended' },
  { label: '已关闭', value: 'closed' },
]

const statusTagColor: Record<string, string> = {
  active: 'green',
  suspended: 'orangered',
  closed: undefined,
}
const statusTagLabel: Record<string, string> = {
  active: '正常',
  suspended: '暂停',
  closed: '已关闭',
}

// Level config options
const levelOptions = ref<any[]>([])

// Edit modal
const showModal = ref(false)
const modalTitle = ref('编辑租户')
const formRef = ref<FormInstance | null>(null)
const formLoading = ref(false)
const editingId = ref<number | null>(null)
const editingVersion = ref(0)

const form = reactive({
  name: '',
  max_members: null as number | null,
  max_concurrency: null as number | null,
  level: null as number | null,
})

// Create modal
const showCreateModal = ref(false)
const createFormRef = ref<FormInstance | null>(null)
const createFormLoading = ref(false)

const createForm = reactive({
  tenant_name: '',
  tenant_code: '',
  username: '',
  email: '',
  password: '',
  max_members: 10 as number | null,
  max_concurrency: 0 as number | null,
})

function openCreate() {
  createForm.tenant_name = ''
  createForm.tenant_code = ''
  createForm.username = ''
  createForm.email = ''
  createForm.password = ''
  createForm.max_members = null
  createForm.max_concurrency = null
  showCreateModal.value = true
}

async function handleCreateSubmit(done: () => void) {
  try {
    const errors = await createFormRef.value?.validate()
    if (errors) return
  } catch {
    return
  }

  createFormLoading.value = true
  try {
    const payload: Record<string, any> = {
      tenant_name: createForm.tenant_name,
      tenant_code: createForm.tenant_code,
      username: createForm.username,
      email: createForm.email,
      password: createForm.password,
    }
    if (createForm.max_members != null) payload.max_members = createForm.max_members
    if (createForm.max_concurrency != null) payload.max_concurrency = createForm.max_concurrency
    await request.post('/admin/tenants', payload)
    Message.success('租户创建成功')
    done()
    fetchData()
  } catch {
    return false
  } finally {
    createFormLoading.value = false
  }
}

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '租户名', dataIndex: 'name', width: 160, ellipsis: true },
  { title: '代码', dataIndex: 'code', width: 120 },
  {
    title: '状态',
    dataIndex: 'status',
    width: 80,
    render({ record }) {
      return h(Tag, { color: statusTagColor[record.status], size: 'small' }, () => statusTagLabel[record.status] || record.status)
    },
  },
  { title: '所有者', dataIndex: 'owner_name', width: 100, ellipsis: true },
  { title: '成员', dataIndex: 'member_count', width: 70 },
  {
    title: '成员上限',
    dataIndex: 'effective_max_members',
    width: 100,
    render({ record }) {
      const val = record.effective_max_members ?? record.max_members
      const custom = record.max_members != null ? ' (自定义)' : ''
      return `${val}${custom}`
    },
  },
  {
    title: '并发上限',
    dataIndex: 'effective_max_concurrency',
    width: 100,
    render({ record }) {
      const val = record.effective_max_concurrency ?? record.max_concurrency
      const custom = record.max_concurrency != null ? ' (自定义)' : ''
      return val ? `${val}${custom}` : `不限${custom}`
    },
  },
  {
    title: '等级',
    dataIndex: 'level',
    width: 130,
    render({ record }) {
      const lv = record.level
      const lvConfig = levelOptions.value.find((o: any) => o.level === lv)
      const name = lvConfig?.name || (lv != null ? `LV${lv}` : '-')
      const tags = [
        h(Tag, { color: 'blue', size: 'small' }, () => name),
      ]
      if (lvConfig && lvConfig.price_multiplier != null && lvConfig.price_multiplier < 1) {
        const discount = Math.round((1 - lvConfig.price_multiplier) * 100)
        tags.push(h(Tag, { color: 'green', size: 'small' }, () => `${discount}%折扣`))
      }
      return h(Space, { size: 4 }, () => tags)
    },
  },
  {
    title: '钱包余额(美元)',
    dataIndex: 'wallet_balance',
    width: 120,
    render({ record }) {
      const val = parseFloat(record.wallet_balance || '0')
      return `\$${val.toFixed(2)}`
    },
  },
  { title: '创建时间', dataIndex: 'created_at', width: 170 },
  {
    title: '操作',
    dataIndex: 'actions',
    width: 140,
    fixed: 'right',
    render({ record }) {
      return h(Space, { size: 4 }, () => [
        h(Button, { size: 'small', type: 'primary', onClick: () => openDetail(record) }, () => '详情'),
        h(Button, { size: 'small', onClick: () => openEdit(record) }, () => '编辑'),
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
    if (filterStatus.value) params.status = filterStatus.value
    if (filterSearch.value) params.keyword = filterSearch.value

    const res: any = await request.get('/admin/tenants', { params })
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

function openEdit(row: any) {
  modalTitle.value = '编辑租户'
  editingId.value = row.id
  editingVersion.value = row.version || 0
  form.name = row.name
  form.max_members = row.max_members ?? null
  form.max_concurrency = row.max_concurrency ?? null
  form.level = row.level ?? null
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
    await request.put(`/admin/tenants/${editingId.value}`, {
      name: form.name,
      max_members: form.max_members,
      max_concurrency: form.max_concurrency,
      level: form.level,
    })
    Message.success('更新成功')
    done()
    fetchData()
  } catch {
    return false
  } finally {
    formLoading.value = false
  }
}

function openDetail(row: any) {
  router.push({ name: 'AdminTenantDetail', params: { id: row.id } })
}

onMounted(() => {
  fetchData()
  fetchLevelConfigs()
})

async function fetchLevelConfigs() {
  try {
    const res: any = await request.get('/admin/tenant-level-configs')
    levelOptions.value = res.data?.data?.list || res.data?.list || res.data?.data || []
  } catch {
    levelOptions.value = []
  }
}

const { exporting, exportFile } = useExport({
  url: '/admin/tenants/export',
  getFilters: () => ({
    status: filterStatus.value,
    keyword: filterSearch.value,
  }),
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="租户列表" subtitle="管理平台上的所有租户组织">
      <template #actions>
        <AButton type="primary" @click="openCreate">创建租户</AButton>
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
          v-model="filterStatus"
          :options="statusOptions"
          placeholder="状态"
          allow-clear
          style="width: 120px"
          @change="handleFilter"
        />
        <AInput
          v-model="filterSearch"
          placeholder="搜索租户名/代码..."
          allow-clear
          style="width: 220px"
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
        :scroll="{ x: 1300 }"
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

    <!-- Edit Modal -->
    <AModal
      v-model:visible="showModal"
      :title="modalTitle"
      :mask-closable="false"
      :width="480"
      :on-before-ok="handleSubmit"
      :ok-loading="formLoading"
    >
      <AForm ref="formRef" :model="form" :auto-label-width="true" layout="vertical">
        <AFormItem field="name" label="租户名称" :rules="[{ required: true, message: '请输入租户名称' }]">
          <AInput v-model="form.name" placeholder="请输入租户名称" />
        </AFormItem>
        <AFormItem field="level" label="租户等级">
          <ASelect
            v-model="form.level"
            placeholder="请选择等级"
            allow-clear
            class="w-full"
          >
            <AOption v-for="opt in levelOptions" :key="opt.level" :value="opt.level">
              {{ opt.name }}（{{ opt.price_multiplier != null ? Math.round((1 - opt.price_multiplier) * 100) : 0 }}%）
            </AOption>
          </ASelect>
        </AFormItem>
        <ASpace :size="16" class="w-full">
          <AFormItem field="max_members" label="最大成员数" extra="留空跟随等级" class="flex-1">
            <AInputNumber v-model="form.max_members" :min="1" :max="10000" placeholder="最大成员数" allow-clear class="w-full" />
          </AFormItem>
          <AFormItem field="max_concurrency" label="并发上限" extra="留空跟随等级" class="flex-1">
            <AInputNumber v-model="form.max_concurrency" :min="0" placeholder="并发上限" allow-clear class="w-full" />
          </AFormItem>
        </ASpace>
      </AForm>
    </AModal>

    <!-- Create Tenant Modal -->
    <AModal
      v-model:visible="showCreateModal"
      title="创建租户"
      :mask-closable="false"
      :width="560"
      :on-before-ok="handleCreateSubmit"
      :ok-loading="createFormLoading"
    >
      <AForm ref="createFormRef" :model="createForm" :auto-label-width="true" layout="vertical">
        <AFormItem field="tenant_name" label="租户名称" :rules="[{ required: true, message: '请输入租户名称' }, { minLength: 2, maxLength: 100, message: '租户名称长度为2-100位' }]">
          <AInput v-model="createForm.tenant_name" placeholder="如：示例科技有限公司" />
        </AFormItem>
        <AFormItem field="tenant_code" label="租户代码" :rules="[{ required: true, message: '请输入租户代码' }, { minLength: 3, maxLength: 30, message: '租户代码为3-30位' }, { match: /^[a-z0-9][a-z0-9-]*[a-z0-9]$/, message: '仅允许小写字母、数字、中划线' }]">
          <AInput v-model="createForm.tenant_code" placeholder="如：my-company" />
        </AFormItem>
        <AFormItem field="username" label="管理员用户名" :rules="[{ required: true, message: '请输入管理员用户名' }, { minLength: 3, maxLength: 50, message: '用户名长度为3-50位' }]">
          <AInput v-model="createForm.username" placeholder="租户管理员登录用户名" />
        </AFormItem>
        <AFormItem field="email" label="管理员邮箱" :rules="[{ required: true, message: '请输入邮箱' }, { type: 'email', message: '邮箱格式不正确' }]">
          <AInput v-model="createForm.email" placeholder="管理员邮箱" />
        </AFormItem>
        <AFormItem field="password" label="管理员密码" :rules="[{ required: true, message: '请输入密码' }, { minLength: 8, maxLength: 64, message: '密码长度为8-64位' }]">
          <AInput v-model="createForm.password" type="password" placeholder="需包含字母和数字，至少8位" />
        </AFormItem>
        <ASpace :size="16" class="w-full">
          <AFormItem field="max_members" label="最大成员数" extra="留空跟随等级" class="flex-1">
            <AInputNumber v-model="createForm.max_members" :min="1" :max="10000" placeholder="留空跟随等级" allow-clear class="w-full" />
          </AFormItem>
          <AFormItem field="max_concurrency" label="并发上限" extra="留空跟随等级" class="flex-1">
            <AInputNumber v-model="createForm.max_concurrency" :min="0" placeholder="留空跟随等级" allow-clear class="w-full" />
          </AFormItem>
        </ASpace>
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
</style>
