<script setup lang="ts">
import { ref, reactive, computed, onMounted, h } from 'vue'
import {
  Tag, Button, Space, Message, Modal, Dropdown, Doption,
} from '@arco-design/web-vue'
import type { TableColumnData, FormInstance } from '@arco-design/web-vue'
import { IconSync } from '@arco-design/web-vue/es/icon'
import PageHeader from '@/components/PageHeader.vue'
import TableStats from '@/components/TableStats.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import request from '@/utils/request'
import { isSuperAdmin } from '@/utils/permission'
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
const resetForm = reactive({ new_password: '' })
const resetLoading = ref(false)

const form = reactive({
  username: '',
  email: '',
  password: '',
  role: 'admin',
  status: 'active',
  role_ids: [] as number[],
})

// 分配角色属于角色管理，仅超管可用（后端同样硬性拦截）。
// 非超管仍可管理账号本身：创建、禁用、改密、删除。
const canAssignRoles = computed(() => isSuperAdmin())

// 可分配的角色（来自 sys_admin_roles，用户可自由增删改）
const availableRoles = ref<any[]>([])
const roleSelectOptions = computed(() =>
  availableRoles.value.map(r => ({
    label: r.is_enabled ? r.name : `${r.name}（已禁用）`,
    value: r.id,
  }))
)

// 角色分配弹窗（已有账号的快捷入口）
const showRoleModal = ref(false)
const roleTarget = ref<any>(null)
const roleAssignIds = ref<number[]>([])
const roleAssignLoading = ref(false)

async function fetchRoles() {
  // 角色列表接口本身就是超管专属，非超管调用只会拿到 403
  if (!canAssignRoles.value) return
  try {
    const res: any = await request.get('/admin/roles')
    availableRoles.value = res.data?.data?.list || res.data?.list || []
  } catch {
    availableRoles.value = []
  }
}

function openAssignRoles(row: any) {
  roleTarget.value = row
  roleAssignIds.value = (row.roles || []).map((r: any) => r.id)
  showRoleModal.value = true
}

async function submitAssignRoles(done: (closed?: boolean) => void) {
  roleAssignLoading.value = true
  try {
    await request.put(`/admin/users/${roleTarget.value.id}/roles`, {
      role_ids: roleAssignIds.value,
    })
    Message.success('角色已更新')
    done()
    fetchData()
  } catch {
    return false
  } finally {
    roleAssignLoading.value = false
  }
}

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
    // 特权标记（sys_admin_users.role），不是业务角色 —— 业务角色在下一列。
    // 与编辑表单的 label 保持一致，避免表格里出现两个「角色」列。
    title: '账号类型',
    dataIndex: 'role',
    width: 110,
    render({ record }) {
      const color = record.role === 'super_admin' ? 'red' : 'arcoblue'
      const label = record.role === 'super_admin' ? '超级管理员' : '普通账号'
      return h(Tag, { color, size: 'small' }, () => label)
    },
  },
  {
    title: '角色',
    dataIndex: 'roles',
    width: 200,
    render({ record }) {
      // 超管权限来自账号属性而非角色，不展示角色标签
      if (record.role === 'super_admin') {
        return h(Tag, { color: 'red', size: 'small' }, () => '全部权限')
      }
      const list = record.roles || []
      // 未分配角色 = 零权限，只能访问自助接口。这是安全的默认值，但要让人一眼看见
      if (list.length === 0) {
        return h(Tag, { color: 'orange', size: 'small' }, () => '未分配角色')
      }
      return h(Space, { size: [4, 4], wrap: true }, () =>
        list.map((r: any) =>
          h(Tag, { size: 'small', color: r.is_enabled ? 'arcoblue' : 'gray' },
            () => r.is_enabled ? r.name : `${r.name}（已禁用）`)
        )
      )
    },
  },
  {
    title: '状态',
    dataIndex: 'status',
    width: 80,
    render({ record }) {
      const locked = !!record.locked_until && new Date(record.locked_until) > new Date()
      if (locked) {
        return h(Tag, { color: 'red', size: 'small' }, () => '锁定')
      }
      const color = record.status === 'active' ? 'green' : undefined
      const label = record.status === 'active' ? '启用' : '禁用'
      return h(Tag, { color, size: 'small' }, () => label)
    },
  },
  { title: '最后登录', dataIndex: 'last_login_at', width: 180 },
  {
    title: '操作',
    dataIndex: 'actions',
    width: 170,
    fixed: 'right',
    render({ record }) {
      const locked = !!record.locked_until && new Date(record.locked_until) > new Date()

      // 只有「编辑」常驻，其余收进「更多」：这一列最多能有 6 个动作
      // （编辑/分配角色/禁用/重置密码/解锁/删除），平铺必然换行、把行高撑成两倍。
      const items = []
      if (record.role !== 'super_admin' && canAssignRoles.value) {
        items.push(h(Doption, { onClick: () => openAssignRoles(record) }, () => '分配角色'))
      }
      items.push(h(Doption, {
        onClick: () => confirmAction(
          `确定${record.status === 'active' ? '禁用' : '启用'}该用户？`,
          () => toggleStatus(record)),
      }, () => record.status === 'active' ? '禁用' : '启用'))
      items.push(h(Doption, { onClick: () => openResetPassword(record) }, () => '重置密码'))
      if (locked) {
        items.push(h(Doption, {
          onClick: () => confirmAction('确定解除该用户的登录锁定？', () => handleUnlock(record)),
        }, () => '解锁'))
      }
      // 删除不可逆，在下拉里标红，避免顺手点到
      items.push(h(Doption, {
        class: 'doption-danger',
        onClick: () => confirmAction('确定删除该用户？此操作不可撤销。', () => deleteUser(record), true),
      }, () => '删除'))

      return h(Space, { size: 4 }, () => [
        h(Button, { size: 'small', type: 'primary', onClick: () => openEdit(record) }, () => '编辑'),
        h(Dropdown, { trigger: 'click' }, {
          default: () => h(Button, { size: 'small' }, () => '更多'),
          content: () => items,
        }),
      ])
    },
  },
]

/**
 * 下拉菜单里的二次确认。
 *
 * 原先这些动作用 Popconfirm 包按钮，但 Popconfirm 依附于触发元素，
 * 放进下拉里会随菜单收起一起消失。改用 Modal.confirm，与页面其他确认框一致。
 */
function confirmAction(content: string, onOk: () => void, danger = false) {
  Modal.confirm({
    title: '确认操作',
    content,
    okText: '确定',
    cancelText: '取消',
    okButtonProps: danger ? { status: 'danger' } : undefined,
    onOk,
  })
}

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
  form.role_ids = []
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
  form.role_ids = (row.roles || []).map((r: any) => r.id)
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
      // 账号属性与角色关联是两个接口：前者改 sys_admin_users，后者改 sys_admin_user_roles
      const { role_ids, ...userForm } = form
      await request.put(`/admin/users/${editingId.value}`, userForm)
      if (form.role !== 'super_admin' && canAssignRoles.value) {
        await request.put(`/admin/users/${editingId.value}/roles`, { role_ids })
      }
      Message.success('更新成功')
    } else {
      // 创建时角色随账号一起写入，避免新账号短暂处于零权限状态
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

async function handleUnlock(row: any) {
  try {
    await request.put(`/admin/users/${row.id}/unlock`)
    Message.success('解锁成功')
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
  resetForm.new_password = ''
  showResetModal.value = true
}

function fillRandomPassword() {
  resetForm.new_password = generateRandomPassword()
}

async function handleResetPassword(done: () => void) {
  if (!resetForm.new_password || resetForm.new_password.length < 8) {
    Message.warning('请输入新密码（至少8位）')
    return false
  }
  resetLoading.value = true
  try {
    await request.put(`/admin/users/${resetTarget.value.id}/reset-password`, {
      new_password: resetForm.new_password,
    })
    const pwd = resetForm.new_password
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
  fetchRoles()
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
      <ResponsiveTable
        :columns="columns"
        :data="data"
        :loading="loading"
        :scroll="{ x: 1200 }"
        :bordered="false"
        :stripe="true"
        row-key="id"
        card-title-key="username"
        card-subtitle-key="email"
        card-badge-key="role"
        :card-fields="['status', 'last_login_at']"
      />
      <div class="table-footer">
        <TableStats :total="pagination.total" />
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
        <AFormItem field="role" label="账号类型" :rules="[{ required: true, message: '请选择账号类型' }]">
          <ASelect v-model="form.role" :options="roleOptions" placeholder="请选择账号类型" />
          <template #extra>
            <span v-if="form.role === 'super_admin'">超级管理员拥有全部权限，不需要也不使用角色</span>
          </template>
        </AFormItem>
        <AFormItem v-if="form.role !== 'super_admin' && canAssignRoles" field="role_ids" label="角色">
          <ASelect
            v-model="form.role_ids"
            :options="roleSelectOptions"
            multiple
            allow-clear
            placeholder="请选择角色（可多选）"
          />
          <template #extra>
            <span v-if="form.role_ids.length === 0" class="role-empty-hint">
              未分配角色的账号没有任何权限，登录后只能访问个人信息
            </span>
            <span v-else>权限取所选角色的并集</span>
          </template>
        </AFormItem>
        <AFormItem label="状态">
          <ASelect v-model="form.status" :options="statusOptions" placeholder="请选择状态" />
        </AFormItem>
      </AForm>
    </AModal>

    <!-- 分配角色 -->
    <AModal
      v-model:visible="showRoleModal"
      title="分配角色"
      :mask-closable="false"
      :width="460"
      :on-before-ok="submitAssignRoles"
      :ok-loading="roleAssignLoading"
    >
      <AForm :model="{}" layout="vertical">
        <AFormItem label="账号">
          <span>{{ roleTarget?.username }}</span>
        </AFormItem>
        <AFormItem label="角色">
          <ASelect
            v-model="roleAssignIds"
            :options="roleSelectOptions"
            multiple
            allow-clear
            placeholder="请选择角色（可多选）"
          />
          <template #extra>
            <span v-if="roleAssignIds.length === 0" class="role-empty-hint">
              清空角色后该账号将失去全部权限，登录后只能访问个人信息
            </span>
            <span v-else>权限取所选角色的并集</span>
          </template>
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
      <AForm :model="resetForm" :auto-label-width="true" layout="vertical">
        <AFormItem field="new_password" label="新密码">
          <AInput
            v-model="resetForm.new_password"
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

.role-empty-hint {
  color: rgb(var(--orange-6));
}

/* 下拉里的危险动作：颜色是唯一的视觉区分，必须标出来 */
:deep(.doption-danger) {
  color: rgb(var(--red-6));
}
</style>
