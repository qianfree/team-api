<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Space, Popconfirm, Message, Modal,
} from '@arco-design/web-vue'
import type { TableColumnData, FormInstance } from '@arco-design/web-vue'
import { IconSync } from '@arco-design/web-vue/es/icon'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

const loading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50],
})

const showModal = ref(false)
const modalTitle = ref('创建用户')
const formRef = ref<FormInstance | null>(null)
const formLoading = ref(false)
const editingId = ref<number | null>(null)

const showResetModal = ref(false)
const resetTarget = ref<any>(null)
const resetPasswordValue = ref('')
const resetLoading = ref(false)

const form = reactive({
  username: '',
  email: '',
  password: '',
  role: 'admin',
  status: 'active',
})

const roleOptions = [
  { label: '超级管理员', value: 'super_admin' },
  { label: '管理员', value: 'admin' },
]

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
]

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '用户名', dataIndex: 'username', width: 120 },
  { title: '邮箱', dataIndex: 'email', width: 200 },
  {
    title: '角色',
    dataIndex: 'role',
    width: 120,
    render({ record }) {
      const color = record.role === 'super_admin' ? 'red' : 'arcoblue'
      const label = record.role === 'super_admin' ? '超级管理员' : '管理员'
      return h(Tag, { color, size: 'small' }, () => label)
    },
  },
  {
    title: '状态',
    dataIndex: 'status',
    width: 80,
    render({ record }) {
      const color = record.status === 'active' ? 'green' : undefined
      const label = record.status === 'active' ? '启用' : '禁用'
      return h(Tag, { color, size: 'small' }, () => label)
    },
  },
  { title: '最后登录', dataIndex: 'last_login_at', width: 180 },
  {
    title: '操作',
    dataIndex: 'actions',
    width: 260,
    fixed: 'right',
    render({ record }) {
      return h(Space, { size: 4 }, () => [
        h(Button, { size: 'small', type: 'primary', onClick: () => openEdit(record) }, () => '编辑'),
        h(Popconfirm, {
          content: `确定${record.status === 'active' ? '禁用' : '启用'}该用户？`,
          onOk: () => toggleStatus(record),
        }, () => h(Button, { size: 'small' }, () => record.status === 'active' ? '禁用' : '启用')),
        h(Button, { size: 'small', onClick: () => openResetPassword(record) }, () => '重置密码'),
        h(Popconfirm, {
          content: '确定删除该用户？此操作不可撤销。',
          onOk: () => deleteUser(record),
        }, () => h(Button, { size: 'small', status: 'danger' }, () => '删除')),
      ])
    },
  },
]

async function fetchData() {
  loading.value = true
  try {
    const res: any = await request.get('/admin/users', {
      params: { page: pagination.current, page_size: pagination.pageSize },
    })
    data.value = res.data?.data?.list || res.data?.list || []
    pagination.total = res.data?.data?.total || res.data?.total || 0
  } catch {
    data.value = []
    pagination.total = 0
  } finally {
    loading.value = false
  }
}

function openCreate() {
  modalTitle.value = '创建用户'
  editingId.value = null
  form.username = ''
  form.email = ''
  form.password = ''
  form.role = 'admin'
  form.status = 'active'
  showModal.value = true
}

function openEdit(row: any) {
  modalTitle.value = '编辑用户'
  editingId.value = row.id
  form.username = row.username
  form.email = row.email
  form.password = ''
  form.role = row.role
  form.status = row.status
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
    if (editingId.value) {
      await request.put(`/admin/users/${editingId.value}`, form)
      Message.success('更新成功')
    } else {
      await request.post('/admin/users', form)
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

async function toggleStatus(row: any) {
  try {
    await request.put(`/admin/users/${row.id}/status`, { status: row.status === 'active' ? 'disabled' : 'active' })
    Message.success('状态更新成功')
    fetchData()
  } catch {
    // error handled by interceptor
  }
}

function generateRandomPassword(length = 16) {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789!@#$%'
  let password = ''
  for (let i = 0; i < length; i++) {
    password += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  return password
}

function openResetPassword(row: any) {
  resetTarget.value = row
  resetPasswordValue.value = ''
  showResetModal.value = true
}

function fillRandomPassword() {
  resetPasswordValue.value = generateRandomPassword()
}

async function handleResetPassword(done: () => void) {
  if (!resetPasswordValue.value || resetPasswordValue.value.length < 8) {
    Message.warning('请输入新密码（至少8位）')
    return false
  }
  resetLoading.value = true
  try {
    await request.put(`/admin/users/${resetTarget.value.id}/reset-password`, {
      new_password: resetPasswordValue.value,
    })
    const pwd = resetPasswordValue.value
    done()
    Modal.success({
      title: '密码重置成功',
      content: `用户 ${resetTarget.value.username} 的新密码：${pwd}`,
      okText: '复制并关闭',
      async onOk() {
        await navigator.clipboard.writeText(pwd)
        Message.success('已复制到剪贴板')
      },
    })
  } catch {
    return false
  } finally {
    resetLoading.value = false
  }
}

async function deleteUser(row: any) {
  try {
    await request.delete(`/admin/users/${row.id}`)
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
  url: '/admin/users/export',
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="用户管理" description="管理系统后台用户账号">
      <template #actions>
        <AButton type="primary" @click="openCreate">创建用户</AButton>
        <ADropdown trigger="hover">
          <AButton :loading="exporting">导出</AButton>
          <template #content>
            <ADoption @click="exportFile('csv')">导出 CSV</ADoption>
            <ADoption @click="exportFile('xlsx')">导出 Excel</ADoption>
          </template>
        </ADropdown>
      </template>
    </PageHeader>

    <!-- Table Card -->
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
      :width="500"
      :on-before-ok="handleSubmit"
      :ok-loading="formLoading"
    >
      <AForm ref="formRef" :model="form" :auto-label-width="true" layout="vertical">
        <AFormItem field="username" label="用户名" :rules="[{ required: true, message: '请输入用户名' }]">
          <AInput v-model="form.username" placeholder="请输入用户名" />
        </AFormItem>
        <AFormItem field="email" label="邮箱" :rules="[
          { required: true, message: '请输入邮箱' },
          { type: 'email', message: '邮箱格式不正确' },
        ]">
          <AInput v-model="form.email" placeholder="请输入邮箱" />
        </AFormItem>
        <AFormItem v-if="!editingId" field="password" label="密码">
          <AInput v-model="form.password" type="password" placeholder="请输入密码" />
        </AFormItem>
        <AFormItem field="role" label="角色" :rules="[{ required: true, message: '请选择角色' }]">
          <ASelect v-model="form.role" :options="roleOptions" placeholder="请选择角色" />
        </AFormItem>
        <AFormItem label="状态">
          <ASelect v-model="form.status" :options="statusOptions" placeholder="请选择状态" />
        </AFormItem>
      </AForm>
    </AModal>

    <!-- Reset Password Modal -->
    <AModal
      v-model:visible="showResetModal"
      title="重置密码"
      :mask-closable="false"
      :width="460"
      :on-before-ok="handleResetPassword"
      :ok-loading="resetLoading"
      ok-text="确认重置"
    >
      <AForm :auto-label-width="true" layout="vertical">
        <AFormItem field="new_password" label="新密码">
          <AInput
            v-model="resetPasswordValue"
            placeholder="请输入新密码（至少8位），或点击随机生成"
          >
            <template #append>
              <AButton type="text" @click="fillRandomPassword">
                <template #icon><IconSync /></template>
                随机
              </AButton>
            </template>
          </AInput>
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
