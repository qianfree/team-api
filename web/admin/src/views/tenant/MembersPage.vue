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
const data = ref<any[]>([])
const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50],
})

const filterStatus = ref<string | null>(null)
const filterRole = ref<string | null>(null)
const filterSearch = ref('')
const filterTenantId = ref<string | null>(null)
const filterTenantOptions = ref<{ label: string; value: number }[]>([])

async function fetchFilterTenantOptions(keyword: string) {
  try {
    const res: any = await request.get('/admin/tenants', {
      params: { page: 1, page_size: 20, keyword, status: 'active' },
    })
    const list = res.data?.data?.list || res.data?.list || []
    filterTenantOptions.value = list.map((t: any) => ({
      label: `${t.name}（${t.code}）`,
      value: t.id,
    }))
  } catch { /* ignore */ }
}

// Create member
const showCreateModal = ref(false)
const createFormRef = ref<FormInstance | null>(null)
const createFormLoading = ref(false)
const tenantOptions = ref<{ label: string; value: number }[]>([])

const createForm = reactive({
  tenant_id: null as number | null,
  username: '',
  email: '',
  password: '',
  display_name: '',
  role: 'member' as string,
})

const createRoleOptions = [
  { label: 'Admin', value: 'admin' },
  { label: 'Member', value: 'member' },
]

async function fetchTenantOptions(keyword: string) {
  try {
    const res: any = await request.get('/admin/tenants', {
      params: { page: 1, page_size: 20, keyword, status: 'active' },
    })
    const list = res.data?.data?.list || res.data?.list || []
    tenantOptions.value = list.map((t: any) => ({
      label: `${t.name}（${t.code}）`,
      value: t.id,
    }))
  } catch { /* ignore */ }
}

function openCreate() {
  createForm.tenant_id = null
  createForm.username = ''
  createForm.email = ''
  createForm.password = ''
  createForm.display_name = ''
  createForm.role = 'member'
  fetchTenantOptions('')
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
    await request.post('/admin/members', createForm)
    Message.success('成员添加成功')
    done()
    fetchData()
  } catch {
    return false
  } finally {
    createFormLoading.value = false
  }
}

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '正常', value: 'active' },
  { label: '禁用', value: 'disabled' },
  { label: '锁定', value: 'locked' },
]

const roleOptions = [
  { label: '全部角色', value: '' },
  { label: 'Owner', value: 'owner' },
  { label: 'Admin', value: 'admin' },
  { label: 'Member', value: 'member' },
]

const roleTagColor: Record<string, string> = {
  owner: 'arcoblue',
  admin: 'green',
  member: undefined,
}
const roleTagLabel: Record<string, string> = {
  owner: 'Owner',
  admin: 'Admin',
  member: 'Member',
}

const statusTagColor: Record<string, string> = {
  active: 'green',
  disabled: 'orangered',
  locked: 'red',
}
const statusTagLabel: Record<string, string> = {
  active: '正常',
  disabled: '禁用',
  locked: '锁定',
}

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '用户名', dataIndex: 'username', width: 120, ellipsis: true },
  { title: '显示名', dataIndex: 'display_name', width: 100, ellipsis: true },
  { title: '邮箱', dataIndex: 'email', width: 180, ellipsis: true },
  {
    title: '角色',
    dataIndex: 'role',
    width: 80,
    render({ record }) {
      return h(Tag, { color: roleTagColor[record.role], size: 'small' }, () => roleTagLabel[record.role] || record.role)
    },
  },
  {
    title: '状态',
    dataIndex: 'status',
    width: 70,
    render({ record }) {
      return h(Tag, { color: statusTagColor[record.status], size: 'small' }, () => statusTagLabel[record.status] || record.status)
    },
  },
  { title: '所属租户', dataIndex: 'tenant_name', width: 130, ellipsis: true },
  { title: '租户代码', dataIndex: 'tenant_code', width: 100 },
  { title: '最后登录', dataIndex: 'last_login_at', width: 180 },
  { title: '登录IP', dataIndex: 'last_login_ip', width: 130 },
  { title: '创建时间', dataIndex: 'created_at', width: 180 },
  {
    title: '操作',
    dataIndex: 'actions',
    width: 180,
    fixed: 'right',
    render({ record }) {
      const isDisabled = record.status === 'disabled'
      return h(Space, { size: 'mini' }, () => [
        h(Button, {
          type: 'text',
          size: 'mini',
          status: isDisabled ? 'success' : 'warning',
          onClick: () => handleToggleStatus(record),
        }, () => isDisabled ? '启用' : '禁用'),
        h(Button, {
          type: 'text',
          size: 'mini',
          onClick: () => handleResetPassword(record),
        }, () => '重置密码'),
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
    if (filterRole.value) params.role = filterRole.value
    if (filterSearch.value) params.keyword = filterSearch.value
    if (filterTenantId.value) params.tenant_id = filterTenantId.value

    const res: any = await request.get('/admin/members', { params })
    data.value = res.data?.data?.list || res.data?.list || []
    pagination.total = res.data?.data?.total || res.data?.total || 0
  } catch {
    // error toast already shown by interceptor
    data.value = []
    pagination.total = 0
  } finally {
    loading.value = false
  }
}

function handleToggleStatus(record: any) {
  const isDisabled = record.status === 'disabled'
  const action = isDisabled ? '启用' : '禁用'
  Modal.warning({
    title: `确认${action}`,
    content: `确定要${action}成员「${record.username}」吗？`,
    hideCancel: false,
    okText: '确定',
    cancelText: '取消',
    onOk: async () => {
      try {
        const endpoint = isDisabled
          ? `/admin/members/${record.id}/enable`
          : `/admin/members/${record.id}/disable`
        await request.put(endpoint)
        Message.success(`${action}成功`)
        fetchData()
      } catch {
        // error toast already shown by interceptor
      }
    },
  })
}

function handleResetPassword(record: any) {
  Modal.warning({
    title: '确认重置密码',
    content: `确定要重置成员「${record.username}」的密码吗？系统将生成新的随机密码。`,
    hideCancel: false,
    okText: '确定重置',
    cancelText: '取消',
    onOk: async () => {
      try {
        const res: any = await request.put(`/admin/members/${record.id}/reset-password`)
        const newPassword = res.data?.data?.new_password
        if (newPassword) {
          Modal.success({
            title: '密码已重置',
            content: `成员「${record.username}」的新密码为：${newPassword}\n\n请妥善保管，关闭后无法再次查看。`,
            okText: '我知道了',
          })
        } else {
          Message.success('密码重置成功')
        }
      } catch {
        // error toast already shown by interceptor
      }
    },
  })
}

function handleFilter() {
  pagination.current = 1
  fetchData()
}

onMounted(() => {
  fetchFilterTenantOptions('')
  fetchData()
})

const { exporting, exportFile } = useExport({
  url: '/admin/tenants/members/export',
  getFilters: () => ({
    status: filterStatus.value,
    role: filterRole.value,
    keyword: filterSearch.value,
    tenant_id: filterTenantId.value,
  }),
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="成员列表" subtitle="查看和管理所有租户下的成员">
      <template #actions>
        <AButton type="primary" @click="openCreate">添加成员</AButton>
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
          v-model="filterTenantId"
          :options="filterTenantOptions"
          placeholder="选择租户"
          allow-clear
          allow-search
          :filter-option="false"
          style="width: 220px"
          @search="fetchFilterTenantOptions"
          @change="handleFilter"
          @clear="handleFilter"
        />
        <ASelect
          v-model="filterStatus"
          :options="statusOptions"
          placeholder="状态"
          allow-clear
          style="width: 120px"
          @change="handleFilter"
        />
        <ASelect
          v-model="filterRole"
          :options="roleOptions"
          placeholder="角色"
          allow-clear
          style="width: 120px"
          @change="handleFilter"
        />
        <AInput
          v-model="filterSearch"
          placeholder="搜索用户名/邮箱..."
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
        :scroll="{ x: 1500 }"
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

    <!-- Create Member Modal -->
    <AModal
      v-model:visible="showCreateModal"
      title="添加成员"
      :mask-closable="false"
      :width="520"
      :on-before-ok="handleCreateSubmit"
      :ok-loading="createFormLoading"
    >
      <AForm ref="createFormRef" :model="createForm" :auto-label-width="true" layout="vertical">
        <AFormItem field="tenant_id" label="所属租户" :rules="[{ required: true, message: '请选择租户' }]">
          <ASelect
            v-model="createForm.tenant_id"
            :options="tenantOptions"
            placeholder="搜索租户名称/代码"
            allow-search
            :filter-option="false"
            @search="fetchTenantOptions"
          />
        </AFormItem>
        <AFormItem field="username" label="用户名" :rules="[{ required: true, message: '请输入用户名' }, { minLength: 3, maxLength: 50, message: '用户名长度为3-50位' }]">
          <AInput v-model="createForm.username" placeholder="登录用户名" />
        </AFormItem>
        <AFormItem field="email" label="邮箱" :rules="[{ required: true, message: '请输入邮箱' }, { type: 'email', message: '邮箱格式不正确' }]">
          <AInput v-model="createForm.email" placeholder="成员邮箱" />
        </AFormItem>
        <AFormItem field="password" label="密码" :rules="[{ required: true, message: '请输入密码' }, { minLength: 8, maxLength: 64, message: '密码长度为8-64位' }]">
          <AInput v-model="createForm.password" type="password" placeholder="需包含字母和数字，至少8位" />
        </AFormItem>
        <AFormItem field="display_name" label="显示名称">
          <AInput v-model="createForm.display_name" placeholder="可选，默认同用户名" />
        </AFormItem>
        <AFormItem field="role" label="角色" :rules="[{ required: true, message: '请选择角色' }]">
          <ASelect v-model="createForm.role" :options="createRoleOptions" />
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
</style>
