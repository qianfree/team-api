<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import { Message, Tag, Button, Space, Popconfirm } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const data = ref<any[]>([])
const total = ref(0)
const pagination = reactive({ current: 1, pageSize: 20 })

const filterMetricType = ref('')
const filterLevel = ref('')
const filterEnabled = ref('')

const showModal = ref(false)
const modalTitle = ref('创建告警规则')
const editingId = ref<number | null>(null)
const editingVersion = ref(1)
const formLoading = ref(false)

const form = reactive({
  name: '',
  metric_type: 'system.cpu_percent',
  condition: 'gte',
  threshold: 85,
  duration_seconds: 120,
  notification_methods: ['email', 'in_app'] as string[],
  webhook_url: '',
  level: 'warning',
  cooldown_seconds: 300,
  notify_user_ids: [] as number[],
  version: 1,
})

const metricTypeOptions = ref<any[]>([])
const conditionOptions = ref<any[]>([])
const levelOptions = ref<any[]>([])
const adminUsers = ref<any[]>([])

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 60 },
  { title: '规则名称', dataIndex: 'name', width: 160 },
  { title: '指标', dataIndex: 'metric_type_label', width: 160 },
  {
    title: '条件',
    width: 150,
    render({ record }: { record: any }) {
      const cond = record.condition_label || record.condition
      return `${cond} ${Number(record.threshold).toFixed(2)}`
    },
  },
  {
    title: '级别',
    dataIndex: 'level',
    width: 80,
    render({ record }: { record: any }) {
      const colorMap: any = { info: 'arcoblue', warning: 'orange', critical: 'red' }
      const labelMap: any = { info: '信息', warning: '警告', critical: '严重' }
      return h(Tag, { color: colorMap[record.level] || 'blue', size: 'small' }, () => labelMap[record.level] || record.level)
    },
  },
  {
    title: '启用',
    dataIndex: 'is_enabled',
    width: 80,
    render({ record }: { record: any }) {
      return h('span', { style: { color: record.is_enabled ? '#00b42a' : '#c9cdd4' } }, record.is_enabled ? '启用' : '禁用')
    },
  },
  {
    title: '上次触发',
    dataIndex: 'last_triggered_at',
    width: 160,
    render({ record }: { record: any }) {
      return record.last_triggered_at ? new Date(record.last_triggered_at).toLocaleString('zh-CN') : '-'
    },
  },
  {
    title: '操作',
    width: 200,
    fixed: 'right',
    render({ record }: { record: any }) {
      return h(Space, { size: 4 }, () => [
        h(Button, { size: 'small', onClick: () => openEdit(record) }, () => '编辑'),
        h(Button, { size: 'small', type: record.is_enabled ? 'outline' : 'primary', onClick: () => toggleRule(record) }, () => record.is_enabled ? '禁用' : '启用'),
        h(Button, { size: 'small', status: 'success', onClick: () => testRule(record) }, () => '测试'),
        h(Popconfirm, { content: '确定删除该告警规则？此操作不可撤销。', onOk: () => deleteRule(record) }, () => h(Button, { size: 'small', status: 'danger' }, () => '删除')),
      ])
    },
  },
]

async function fetchData() {
  loading.value = true
  try {
    const params: any = { page: pagination.current, page_size: pagination.pageSize }
    if (filterMetricType.value) params.metric_type = filterMetricType.value
    if (filterLevel.value) params.level = filterLevel.value
    if (filterEnabled.value) params.enabled = filterEnabled.value
    const res = await request.get('/admin/alert/rules', { params })
    data.value = res.data?.data?.list || []
    total.value = res.data?.data?.total || 0
  } catch {
    // error auto-shown by Axios interceptor
  } finally {
    loading.value = false
  }
}

async function fetchOptions() {
  try {
    const res = await request.get('/admin/alert/options')
    const d = res.data?.data || {}
    metricTypeOptions.value = d.metric_types || []
    conditionOptions.value = d.conditions || []
    levelOptions.value = d.levels || []
    adminUsers.value = d.admin_users || []
  } catch { /* ignore */ }
}

function openCreate() {
  editingId.value = null
  modalTitle.value = '创建告警规则'
  Object.assign(form, {
    name: '', metric_type: 'system.cpu_percent', condition: 'gte', threshold: 85,
    duration_seconds: 120, notification_methods: ['email', 'in_app'], webhook_url: '',
    level: 'warning', cooldown_seconds: 300, notify_user_ids: [], version: 1,
  })
  showModal.value = true
}

function openEdit(record: any) {
  editingId.value = record.id
  editingVersion.value = record.version
  modalTitle.value = '编辑告警规则'
  Object.assign(form, {
    name: record.name,
    metric_type: record.metric_type,
    condition: record.condition,
    threshold: Number(record.threshold),
    duration_seconds: record.duration_seconds,
    notification_methods: record.notification_methods || ['email', 'in_app'],
    webhook_url: record.webhook_url || '',
    level: record.level,
    cooldown_seconds: record.cooldown_seconds,
    notify_user_ids: record.notify_user_ids || [],
    version: record.version,
  })
  showModal.value = true
}

async function handleSubmit(done: (closed: boolean) => void) {
  if (!form.name) { Message.warning('请输入规则名称'); done(false); return }
  formLoading.value = true
  try {
    if (editingId.value) {
      await request.put(`/admin/alert/rules/${editingId.value}`, { ...form, version: editingVersion.value })
      Message.success('更新成功')
    } else {
      await request.post('/admin/alert/rules', form)
      Message.success('创建成功')
    }
    done(true)
    fetchData()
  } catch {
    done(false)
  } finally {
    formLoading.value = false
  }
}

async function toggleRule(record: any) {
  try {
    await request.put(`/admin/alert/rules/${record.id}/toggle`)
    Message.success(record.is_enabled ? '已禁用' : '已启用')
    fetchData()
  } catch {
    // error auto-shown by Axios interceptor
  }
}

async function testRule(record: any) {
  try {
    await request.post(`/admin/alert/rules/${record.id}/test`)
    Message.success('测试通知已发送')
  } catch {
    // error auto-shown by Axios interceptor
  }
}

async function deleteRule(record: any) {
  try {
    await request.delete(`/admin/alert/rules/${record.id}`)
    Message.success('删除成功')
    fetchData()
  } catch {
    // error auto-shown by Axios interceptor
  }
}

function handlePageChange(page: number) {
  pagination.current = page
  fetchData()
}

onMounted(() => {
  fetchData()
  fetchOptions()
})
</script>

<template>
  <div>
    <PageHeader title="告警规则" description="配置系统告警规则">
      <template #actions>
        <a-button type="primary" @click="openCreate">创建规则</a-button>
      </template>
    </PageHeader>

    <a-card :bordered="false" class="mb-4">
      <a-space>
        <a-select v-model="filterMetricType" :options="metricTypeOptions" placeholder="指标类型" allow-clear style="width: 180px" value-key="value" />
        <a-select v-model="filterLevel" placeholder="级别" allow-clear style="width: 120px">
          <a-option value="info">信息</a-option>
          <a-option value="warning">警告</a-option>
          <a-option value="critical">严重</a-option>
        </a-select>
        <a-select v-model="filterEnabled" placeholder="状态" allow-clear style="width: 120px">
          <a-option value="true">启用</a-option>
          <a-option value="false">禁用</a-option>
        </a-select>
        <a-button @click="fetchData">搜索</a-button>
      </a-space>
    </a-card>

    <a-card :bordered="false">
      <a-table :data="data" :columns="columns" :loading="loading" :pagination="false" row-key="id" />
      <div style="display: flex; justify-content: flex-end; margin-top: 16px">
        <a-pagination :current="pagination.current" :page-size="pagination.pageSize" :total="total" @change="handlePageChange" />
      </div>
    </a-card>

    <a-modal v-model:visible="showModal" :title="modalTitle" :mask-closable="false" :width="600" :on-before-ok="handleSubmit" :ok-loading="formLoading">
      <a-form :model="form" layout="vertical">
        <a-form-item field="name" label="规则名称" required>
          <a-input v-model="form.name" placeholder="如：CPU使用率过高" />
        </a-form-item>
        <a-form-item field="metric_type" label="指标类型" required>
          <a-select v-model="form.metric_type" :options="metricTypeOptions" value-key="value" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item field="condition" label="条件" required>
              <a-select v-model="form.condition" :options="conditionOptions" value-key="value" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item field="threshold" label="阈值" required>
              <a-input-number v-model="form.threshold" :precision="2" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item field="level" label="级别" required>
              <a-select v-model="form.level" :options="levelOptions" value-key="value" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item field="duration_seconds" label="持续时间（秒）">
              <a-input-number v-model="form.duration_seconds" :min="0" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item field="cooldown_seconds" label="冷却时间（秒）">
              <a-input-number v-model="form.cooldown_seconds" :min="0" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item field="notification_methods" label="通知方式">
          <a-checkbox-group v-model="form.notification_methods">
            <a-checkbox value="email">邮件</a-checkbox>
            <a-checkbox value="in_app">站内信</a-checkbox>
            <a-checkbox value="webhook">Webhook</a-checkbox>
          </a-checkbox-group>
        </a-form-item>
        <a-form-item v-if="form.notification_methods.includes('webhook')" field="webhook_url" label="Webhook URL">
          <a-input v-model="form.webhook_url" placeholder="https://..." />
        </a-form-item>
        <a-form-item field="notify_user_ids" label="通知接收人">
          <a-select v-model="form.notify_user_ids" multiple placeholder="选择管理员" value-key="id">
            <a-option v-for="u in adminUsers" :key="u.id" :value="u.id">{{ u.display_name || u.username }}</a-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<style scoped>
.mb-4 { margin-bottom: 16px; }
</style>
