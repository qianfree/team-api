<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import {
  Tag, Button, Space, Popconfirm, Message, Tooltip,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const data = ref<any[]>([])

const filterCategory = ref<string>('')
const filterStatus = ref<string>('')

const categoryOptions = [
  { label: '全部分类', value: '' },
  { label: '代理扩展', value: 'relay' },
  { label: '中间件', value: 'middleware' },
  { label: '计费', value: 'billing' },
  { label: '通知', value: 'notification' },
  { label: '通用扩展', value: 'extension' },
]

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '已注册', value: 'registered' },
  { label: '已安装', value: 'installed' },
  { label: '已启用', value: 'enabled' },
  { label: '已禁用', value: 'disabled' },
  { label: '异常', value: 'error' },
]

const categoryTagColor: Record<string, string> = {
  relay: 'arcoblue',
  middleware: 'purple',
  billing: 'green',
  notification: 'orangered',
  extension: 'gray',
}

const categoryTagLabel: Record<string, string> = {
  relay: '代理扩展',
  middleware: '中间件',
  billing: '计费',
  notification: '通知',
  extension: '通用扩展',
}

const statusTagColor: Record<string, string | undefined> = {
  registered: 'arcoblue',
  installed: 'cyan',
  enabled: 'green',
  disabled: undefined,
  error: 'red',
}

const statusTagLabel: Record<string, string> = {
  registered: '已注册',
  installed: '已安装',
  enabled: '已启用',
  disabled: '已禁用',
  error: '异常',
}

// --- Config modal ---
const showConfigModal = ref(false)
const configPluginName = ref('')
const configSchema = ref<any[]>([])
const configForm = ref<Record<string, any>>({})
const configLoading = ref(false)
const configSubmitting = ref(false)

async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, any> = {}
    if (filterCategory.value) params.category = filterCategory.value
    if (filterStatus.value) params.status = filterStatus.value

    const res: any = await request.get('/admin/plugins', { params })
    data.value = res.data?.data?.list || res.data?.list || []
  } catch {
    data.value = []
  } finally {
    loading.value = false
  }
}

function handleFilter() {
  fetchData()
}

// --- Actions ---

async function doAction(action: string, name: string, extra?: Record<string, any>) {
  try {
    await request.post(`/admin/plugins/${name}/${action}`, extra)
    Message.success('操作成功')
    fetchData()
  } catch {
    // error handled by interceptor
  }
}

async function openConfig(name: string) {
  configPluginName.value = name
  configLoading.value = true
  showConfigModal.value = true
  configForm.value = {}
  configSchema.value = []

  try {
    const [schemaRes, configRes] = await Promise.all([
      request.get(`/admin/plugins/${name}/config-schema`),
      request.get(`/admin/plugins/${name}`),
    ])
    configSchema.value = schemaRes.data?.data?.list || []
    const currentConfig = configRes.data?.data?.config || {}
    // init form with defaults then overlay current values
    const form: Record<string, any> = {}
    for (const field of configSchema.value) {
      form[field.key] = currentConfig[field.key] ?? field.default
    }
    configForm.value = form
  } catch {
    Message.error('加载配置失败')
    showConfigModal.value = false
  } finally {
    configLoading.value = false
  }
}

async function submitConfig() {
  configSubmitting.value = true
  try {
    await request.put(`/admin/plugins/${configPluginName.value}/config`, {
      config: configForm.value,
    })
    Message.success('配置已保存')
    showConfigModal.value = false
    fetchData()
  } catch {
    // error handled by interceptor
  } finally {
    configSubmitting.value = false
  }
}

// --- Columns ---

const columns: TableColumnData[] = [
  {
    title: '名称',
    dataIndex: 'name',
    width: 140,
    render({ record }) {
      return h('div', [
        h('div', { style: 'font-weight: 500' }, record.label || record.name),
        h('div', { style: 'font-size: 12px; color: var(--color-text-3)' }, record.name),
      ])
    },
  },
  {
    title: '描述',
    dataIndex: 'description',
    width: 220,
    ellipsis: true,
  },
  {
    title: '版本',
    dataIndex: 'version',
    width: 80,
  },
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
    width: 100,
    render({ record }) {
      const children: any[] = [
        h(Tag, { color: statusTagColor[record.status], size: 'small' }, () => statusTagLabel[record.status] || record.status),
      ]
      if (record.status === 'error' && record.error_msg) {
        children.push(
          h(Tooltip, { content: record.error_msg }, () =>
            h('span', { style: 'font-size: 12px; color: var(--color-text-3); cursor: help; margin-left: 4px;' }, '查看原因')
          )
        )
      }
      return h('span', { style: 'display: inline-flex; gap: 4px; align-items: center;' }, children)
    },
  },
  {
    title: '作者',
    dataIndex: 'author',
    width: 100,
  },
  {
    title: '操作',
    dataIndex: 'actions',
    width: 260,
    fixed: 'right',
    render({ record }) {
      const btns: any[] = []
      const s = record.status

      if (s === 'registered') {
        btns.push(
          h(Popconfirm, { content: `确定安装插件「${record.label}」？`, onOk: () => doAction('install', record.name) },
            () => h(Button, { size: 'small', type: 'primary' }, () => '安装')
          )
        )
      }

      if (s === 'installed' || s === 'disabled') {
        btns.push(
          h(Popconfirm, { content: `确定启用插件「${record.label}」？`, onOk: () => doAction('enable', record.name) },
            () => h(Button, { size: 'small', type: 'primary' }, () => '启用')
          )
        )
      }

      if (s === 'installed' || s === 'disabled') {
        btns.push(
          h(Popconfirm, { content: `确定卸载插件「${record.label}」？数据将被删除。`, onOk: () => doAction('uninstall', record.name) },
            () => h(Button, { size: 'small', status: 'danger' }, () => '卸载')
          )
        )
      }

      if (s === 'enabled') {
        btns.push(
          h(Popconfirm, { content: `确定禁用插件「${record.label}」？`, onOk: () => doAction('disable', record.name) },
            () => h(Button, { size: 'small' }, () => '禁用')
          )
        )
        btns.push(
          h(Button, { size: 'small', type: 'outline', onClick: () => openConfig(record.name) }, () => '配置')
        )
        btns.push(
          h(Popconfirm, { content: `确定升级插件「${record.label}」？`, onOk: () => doAction('upgrade', record.name) },
            () => h(Button, { size: 'small', type: 'outline' }, () => '升级')
          )
        )
      }

      if (s === 'error') {
        btns.push(
          h(Popconfirm, { content: `确定禁用插件「${record.label}」？`, onOk: () => doAction('disable', record.name) },
            () => h(Button, { size: 'small' }, () => '禁用')
          )
        )
        btns.push(
          h(Popconfirm, { content: `确定卸载插件「${record.label}」？`, onOk: () => doAction('uninstall', record.name) },
            () => h(Button, { size: 'small', status: 'danger' }, () => '卸载')
          )
        )
      }

      return h(Space, { size: 4 }, () => btns)
    },
  },
]

onMounted(() => {
  fetchData()
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="插件管理" description="管理系统插件，查看安装状态并进行生命周期管理" />

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
        <AButton @click="handleFilter">刷新</AButton>
      </ASpace>
    </ACard>

    <!-- Table -->
    <ACard :bordered="false">
      <ATable
        :columns="columns"
        :data="data"
        :loading="loading"
        :scroll="{ x: 1000 }"
        :bordered="false"
        :stripe="true"
        :pagination="false"
        row-key="name"
      />
      <AEmpty v-if="!loading && data.length === 0" style="margin-top: 40px">
        <template #description>暂无已注册的插件</template>
      </AEmpty>
    </ACard>

    <!-- Config Modal -->
    <AModal
      v-model:visible="showConfigModal"
      :title="`配置 - ${configPluginName}`"
      :mask-closable="false"
      :width="550"
      :ok-loading="configSubmitting"
      :on-before-ok="submitConfig"
      @cancel="showConfigModal = false"
    >
      <ASpin :loading="configLoading" style="width: 100%">
        <AForm :model="configForm" layout="vertical" v-if="configSchema.length > 0">
          <AFormItem
            v-for="field in configSchema"
            :key="field.key"
            :label="field.label"
            :required="field.required"
          >
            <!-- string -->
            <AInput
              v-if="field.type === 'string'"
              v-model="configForm[field.key]"
              :placeholder="field.description"
            />
            <!-- int -->
            <AInputNumber
              v-else-if="field.type === 'int'"
              v-model="configForm[field.key]"
              :placeholder="field.description"
              style="width: 100%"
            />
            <!-- bool -->
            <ASwitch
              v-else-if="field.type === 'bool'"
              v-model="configForm[field.key]"
            />
            <!-- select -->
            <ASelect
              v-else-if="field.type === 'select'"
              v-model="configForm[field.key]"
              :options="field.options?.map((o: string) => ({ label: o, value: o }))"
              :placeholder="field.description"
            />
            <!-- json -->
            <ATextarea
              v-else-if="field.type === 'json'"
              v-model="configForm[field.key]"
              :placeholder="field.description"
              :auto-size="{ minRows: 3, maxRows: 8 }"
            />
            <!-- fallback -->
            <AInput
              v-else
              v-model="configForm[field.key]"
              :placeholder="field.description"
            />
            <div v-if="field.description" style="font-size: 12px; color: var(--color-text-3); margin-top: 4px;">
              {{ field.description }}
            </div>
          </AFormItem>
        </AForm>
        <AEmpty v-else description="该插件无可配置项" />
      </ASpin>
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
