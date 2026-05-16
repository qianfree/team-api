<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Space, Message,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const messages = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })

const typeFilter = ref<string | undefined>(undefined)
const typeOptions = [
  { label: '全部', value: '' },
  { label: '系统', value: 'system' },
  { label: '通知', value: 'notification' },
  { label: '告警', value: 'alert' },
]

const channelLabelMap: Record<string, string> = { email: '邮件', in_app: '站内信', sms: '短信', webhook: 'Webhook' }
const typeColorMap: Record<string, string> = { system: 'arcoblue', notification: 'green', alert: 'orangered' }
const typeLabelMap: Record<string, string> = { system: '系统', notification: '通知', alert: '告警' }

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 60 },
  { title: '租户', dataIndex: 'tenant_name', width: 140, render({ record }) { return record.tenant_name || '-' } },
  { title: '用户', dataIndex: 'user_name', width: 120, render({ record }) { return record.user_name || (record.user_id ? `用户#${record.user_id}` : '全部') } },
  { title: '类型', dataIndex: 'type', width: 80, render({ record }) {
    return h(Tag, { color: typeColorMap[record.type] || undefined, size: 'small' }, () => typeLabelMap[record.type] || record.type)
  }},
  { title: '标题', dataIndex: 'title', width: 180, ellipsis: true, tooltip: true },
  { title: '渠道', dataIndex: 'channel', width: 90, render({ record }) {
    return h(Tag, { size: 'small' }, () => channelLabelMap[record.channel] || record.channel)
  }},
  { title: '已读', dataIndex: 'is_read', width: 70, render({ record }) {
    return h(Tag, { color: record.is_read ? 'green' : undefined, size: 'small' }, () => record.is_read ? '已读' : '未读')
  }},
  { title: '广播', dataIndex: 'is_broadcast', width: 70, render({ record }) {
    return h(Tag, { color: record.is_broadcast ? 'orangered' : undefined, size: 'small' }, () => record.is_broadcast ? '广播' : '-')
  }},
  { title: '创建时间', dataIndex: 'created_at', width: 170 },
]

async function fetchList() {
  loading.value = true
  try {
    const params: any = { page: pagination.current, page_size: pagination.pageSize }
    if (typeFilter.value) params.type = typeFilter.value
    const res = await request.get('/admin/notification/messages', { params })
    const data = res.data?.data
    messages.value = data?.list || []
    pagination.total = data?.total || 0
  } catch {
    // error auto-displayed by Axios interceptor
  } finally {
    loading.value = false
  }
}

// === Tenant selector (shared by send & broadcast modals) ===
const tenantOptions = ref<{ value: number; label: string }[]>([])
const tenantLoading = ref(false)
const tenantKeyword = ref('')
const tenantPage = ref(1)
const tenantHasMore = ref(false)

let tenantSearchTimer: ReturnType<typeof setTimeout> | null = null

async function fetchTenantOptions(reset = true) {
  tenantLoading.value = true
  try {
    const params: any = { page: tenantPage.value, page_size: 20 }
    if (tenantKeyword.value) params.keyword = tenantKeyword.value
    const res = await request.get('/admin/tenants/select', { params })
    const d = res.data?.data
    const list: { id: number; name: string; code: string }[] = d?.list || []
    const items = list.map(t => ({ value: t.id, label: `${t.name} (${t.code})` }))
    if (reset) {
      tenantOptions.value = items
    } else {
      tenantOptions.value = [...tenantOptions.value, ...items]
    }
    const total: number = d?.total || 0
    tenantHasMore.value = tenantOptions.value.length < total
  } catch {
    // ignore
  } finally {
    tenantLoading.value = false
  }
}

function handleTenantSearch(val: string) {
  if (tenantSearchTimer) clearTimeout(tenantSearchTimer)
  tenantSearchTimer = setTimeout(() => {
    tenantKeyword.value = val
    tenantPage.value = 1
    fetchTenantOptions(true)
  }, 300)
}

function handleTenantScrollReachEdge(e: Event) {
  if (!tenantHasMore.value || tenantLoading.value) return
  const el = e.target as HTMLElement
  const bottom = el.scrollHeight - el.scrollTop - el.clientHeight
  if (bottom < 30) {
    tenantPage.value++
    fetchTenantOptions(false)
  }
}

function openTenantDropdown() {
  if (tenantOptions.value.length === 0) {
    tenantKeyword.value = ''
    tenantPage.value = 1
    fetchTenantOptions(true)
  }
}

// === User selector (for send modal, depends on tenant) ===
const userOptions = ref<{ value: number; label: string }[]>([])
const userLoading = ref(false)
const userKeyword = ref('')
const userPage = ref(1)
const userHasMore = ref(false)

let userSearchTimer: ReturnType<typeof setTimeout> | null = null

async function fetchUserOptions(tenantId: number, reset = true) {
  userLoading.value = true
  try {
    const params: any = { page: userPage.value, page_size: 20, tenant_id: tenantId }
    if (userKeyword.value) params.keyword = userKeyword.value
    const res = await request.get('/admin/members', { params })
    const d = res.data?.data
    const list: { id: number; username: string; display_name: string; email: string }[] = d?.list || []
    const items = list.map(u => ({ value: u.id, label: u.display_name ? `${u.display_name} (${u.username})` : u.username }))
    if (reset) {
      userOptions.value = items
    } else {
      userOptions.value = [...userOptions.value, ...items]
    }
    const total: number = d?.total || 0
    userHasMore.value = userOptions.value.length < total
  } catch {
    // ignore
  } finally {
    userLoading.value = false
  }
}

function handleUserSearch(val: string) {
  if (!sendForm.tenant_id) return
  if (userSearchTimer) clearTimeout(userSearchTimer)
  userSearchTimer = setTimeout(() => {
    userKeyword.value = val
    userPage.value = 1
    fetchUserOptions(sendForm.tenant_id!, true)
  }, 300)
}

function handleUserScrollReachEdge(e: Event) {
  if (!sendForm.tenant_id || !userHasMore.value || userLoading.value) return
  const el = e.target as HTMLElement
  const bottom = el.scrollHeight - el.scrollTop - el.clientHeight
  if (bottom < 30) {
    userPage.value++
    fetchUserOptions(sendForm.tenant_id, false)
  }
}

function handleSendTenantChange(val: number | undefined) {
  sendForm.user_id = undefined
  userOptions.value = []
  userKeyword.value = ''
  userPage.value = 1
  if (val) {
    fetchUserOptions(val, true)
  }
}

function openUserDropdown() {
  if (sendForm.tenant_id && userOptions.value.length === 0) {
    userKeyword.value = ''
    userPage.value = 1
    fetchUserOptions(sendForm.tenant_id, true)
  }
}

// Send message modal
const showSendModal = ref(false)
const sendLoading = ref(false)
const sendForm = reactive({
  tenant_id: undefined as number | undefined,
  user_id: undefined as number | undefined,
  title: '',
  content: '',
  channel: 'in_app',
})

const channelOptions = [
  { label: '站内信', value: 'in_app' },
  { label: '邮件', value: 'email' },
  { label: '短信', value: 'sms' },
]

function openSendModal() {
  Object.assign(sendForm, { tenant_id: undefined, user_id: undefined, title: '', content: '', channel: 'in_app' })
  tenantKeyword.value = ''
  tenantPage.value = 1
  tenantOptions.value = []
  userOptions.value = []
  userKeyword.value = ''
  userPage.value = 1
  showSendModal.value = true
}

async function handleSendSubmit(done: () => void) {
  if (!sendForm.tenant_id) {
    Message.warning('请选择租户')
    return
  }
  if (!sendForm.title.trim()) {
    Message.warning('请输入标题')
    return
  }
  if (!sendForm.content.trim()) {
    Message.warning('请输入内容')
    return
  }
  sendLoading.value = true
  try {
    const payload: any = {
      tenant_id: sendForm.tenant_id,
      title: sendForm.title,
      content: sendForm.content,
      channel: sendForm.channel,
    }
    if (sendForm.user_id) payload.user_id = sendForm.user_id
    await request.post('/admin/notification/messages/send', payload)
    Message.success('发送成功')
    done()
    fetchList()
  } catch {
    return false
  } finally {
    sendLoading.value = false
  }
}

// Broadcast modal
const showBroadcastModal = ref(false)
const broadcastLoading = ref(false)
const broadcastForm = reactive({
  tenant_id: undefined as number | undefined,
  title: '',
  content: '',
  target_roles: [] as string[],
})

const targetRoleOptions = [
  { label: 'Owner（所有者）', value: 'owner' },
  { label: 'Admin（管理员）', value: 'admin' },
  { label: 'Member（成员）', value: 'member' },
]

function openBroadcastModal() {
  Object.assign(broadcastForm, { tenant_id: undefined, title: '', content: '', target_roles: [] })
  tenantKeyword.value = ''
  tenantPage.value = 1
  tenantOptions.value = []
  showBroadcastModal.value = true
}

async function handleBroadcastSubmit(done: () => void) {
  if (!broadcastForm.tenant_id) {
    Message.warning('请选择租户')
    return
  }
  if (!broadcastForm.title.trim()) {
    Message.warning('请输入标题')
    return
  }
  if (!broadcastForm.content.trim()) {
    Message.warning('请输入内容')
    return
  }
  broadcastLoading.value = true
  try {
    await request.post('/admin/notification/messages/broadcast', {
      tenant_id: broadcastForm.tenant_id,
      title: broadcastForm.title,
      content: broadcastForm.content,
      target_roles: broadcastForm.target_roles.length > 0 ? broadcastForm.target_roles.join(',') : '',
    })
    Message.success('广播成功')
    done()
    fetchList()
  } catch {
    return false
  } finally {
    broadcastLoading.value = false
  }
}

onMounted(fetchList)
</script>

<template>
  <div>
    <PageHeader title="消息管理" description="查看通知消息、发送定向消息和租户广播">
      <template #actions>
        <Space>
          <AButton @click="openSendModal">发送消息</AButton>
          <AButton type="primary" @click="openBroadcastModal">广播消息</AButton>
        </Space>
      </template>
    </PageHeader>

    <ACard :bordered="false">
      <template #title>
        <div class="flex items-center justify-between w-full">
          <span>消息列表</span>
          <ASelect v-model="typeFilter" :options="typeOptions" style="width: 120px" allow-clear placeholder="类型筛选" @change="() => { pagination.current = 1; fetchList() }" />
        </div>
      </template>
      <ATable :columns="columns" :data="messages" :loading="loading" row-key="id" :scroll="{ x: 1100 }" :pagination="false" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchList" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchList() }" />
      </div>
    </ACard>

    <!-- Send Message Modal -->
    <AModal v-model:visible="showSendModal" title="发送消息" :width="520" :mask-closable="false" :on-before-ok="handleSendSubmit" :ok-loading="sendLoading">
      <AForm :model="sendForm" :auto-label-width="true" layout="vertical">
        <AFormItem label="租户" required>
          <ASelect
            v-model="sendForm.tenant_id"
            :options="tenantOptions"
            :loading="tenantLoading"
            placeholder="输入租户名称或代码搜索"
            allow-search
            allow-clear
            :filter-option="false"
            :fallback-option="true"
            @change="handleSendTenantChange"
            @search="handleTenantSearch"
            @popup-visible-change="(visible: boolean) => { if (visible) openTenantDropdown() }"
            @scroll-reach-edge="handleTenantScrollReachEdge"
          />
        </AFormItem>
        <AFormItem label="用户" extra="不选则发送给租户所有人">
          <ASelect
            v-model="sendForm.user_id"
            :options="userOptions"
            :loading="userLoading"
            :disabled="!sendForm.tenant_id"
            placeholder="请先选择租户"
            allow-search
            allow-clear
            :filter-option="false"
            :fallback-option="true"
            @search="handleUserSearch"
            @popup-visible-change="(visible: boolean) => { if (visible) openUserDropdown() }"
            @scroll-reach-edge="handleUserScrollReachEdge"
          />
        </AFormItem>
        <AFormItem label="标题" required>
          <AInput v-model="sendForm.title" placeholder="消息标题" />
        </AFormItem>
        <AFormItem label="内容" required>
          <ATextarea v-model="sendForm.content" :auto-size="{ minRows: 3, maxRows: 8 }" placeholder="消息正文" />
        </AFormItem>
        <AFormItem label="渠道">
          <ASelect v-model="sendForm.channel" :options="channelOptions" />
        </AFormItem>
      </AForm>
    </AModal>

    <!-- Broadcast Modal -->
    <AModal v-model:visible="showBroadcastModal" title="广播消息" :width="520" :mask-closable="false" :on-before-ok="handleBroadcastSubmit" :ok-loading="broadcastLoading">
      <AForm :model="broadcastForm" :auto-label-width="true" layout="vertical">
        <AFormItem label="租户" required>
          <ASelect
            v-model="broadcastForm.tenant_id"
            :options="tenantOptions"
            :loading="tenantLoading"
            placeholder="输入租户名称或代码搜索"
            allow-search
            allow-clear
            :filter-option="false"
            :fallback-option="true"
            @search="handleTenantSearch"
            @popup-visible-change="(visible: boolean) => { if (visible) openTenantDropdown() }"
            @scroll-reach-edge="handleTenantScrollReachEdge"
          />
        </AFormItem>
        <AFormItem label="标题" required>
          <AInput v-model="broadcastForm.title" placeholder="广播标题" />
        </AFormItem>
        <AFormItem label="内容" required>
          <ATextarea v-model="broadcastForm.content" :auto-size="{ minRows: 3, maxRows: 8 }" placeholder="广播正文" />
        </AFormItem>
        <AFormItem label="目标角色" extra="不选则所有角色可见">
          <ASelect v-model="broadcastForm.target_roles" :options="targetRoleOptions" multiple allow-clear placeholder="全部角色" />
        </AFormItem>
      </AForm>
    </AModal>
  </div>
</template>
