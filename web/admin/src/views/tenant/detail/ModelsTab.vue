<script setup lang="ts">
import { ref, computed, watch, onMounted, h } from 'vue'
import { Tag, Button, Space, Popconfirm, Message } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import TableStats from '@/components/TableStats.vue'
import TenantModelPricingModal from '@/components/TenantModelPricingModal.vue'
import { modelCategoryLabelMap, modelCategoryTagColor } from '@/constants/modelCategory'
import request from '@/utils/request'

const props = defineProps<{
  tenantId: string
  active: boolean
}>()

// === 独立模型列表 ===
const modelsLoading = ref(false)
const modelsData = ref<any[]>([])
const allModels = ref<any[]>([])

// 是否存在任一定价覆盖（独立模型表「覆盖」列）：模式/价格/折扣/补丁三键/展示字段
function hasAnyOverride(r: any): boolean {
  if (r.billing_mode || r.discount_ratio) return true
  if (r.custom_input_price || r.custom_output_price || r.custom_cache_read_price || r.custom_cache_creation_price) return true
  if (r.per_request_price) return true
  if (r.custom_pricing_tiers?.length) return true
  if (Array.isArray(r.custom_time_segments) || Array.isArray(r.custom_param_multipliers)) return true
  if (r.custom_per_second_prices) return true
  if (r.price_note || r.discount_label || r.price_change_note) return true
  return false
}

const modelColumns: TableColumnData[] = [
  { title: '模型标识', dataIndex: 'model_code', width: 180, ellipsis: true },
  { title: '显示名', dataIndex: 'model_name', width: 150, ellipsis: true },
  {
    title: '分类', dataIndex: 'category', width: 80,
    render({ record }) {
      return h(Tag, { color: modelCategoryTagColor[record.category], size: 'small' }, () => modelCategoryLabelMap[record.category] || record.category)
    },
  },
  {
    title: '启用', dataIndex: 'enabled', width: 70,
    render({ record }) {
      return h(Tag, { color: record.enabled ? 'green' : undefined, size: 'small' }, () => record.enabled ? '是' : '否')
    },
  },
  {
    title: '计费', dataIndex: 'billing_mode', width: 80,
    render({ record }) {
      if (record.billing_mode === 'per_request') return h(Tag, { color: 'purple', size: 'small' }, () => '按次')
      if (record.billing_mode === 'tiered') return h(Tag, { color: 'orange', size: 'small' }, () => '阶梯')
      if (record.billing_mode === 'token') return h(Tag, { color: 'cyan', size: 'small' }, () => 'Token')
      return h(Tag, { color: 'arcoblue', size: 'small' }, () => '默认')
    },
  },
  {
    title: '折扣', dataIndex: 'discount_ratio', width: 80,
    render({ record }) {
      if (!record.discount_ratio || record.discount_ratio === 1) return '-'
      return `${(record.discount_ratio * 100).toFixed(0)}%`
    },
  },
  {
    title: '覆盖', dataIndex: '_override', width: 70,
    render({ record }) {
      return record._override
        ? h(Tag, { color: 'orangered', size: 'small' }, () => '有')
        : h('span', { style: 'color: var(--color-text-4)' }, '继承')
    },
  },
  {
    title: '并发', dataIndex: 'max_concurrency', width: 70,
    render({ record }) { return record.max_concurrency || '-' },
  },
  { title: '版本', dataIndex: 'version', width: 60 },
  {
    title: '操作', dataIndex: 'actions', width: 160, fixed: 'right',
    render({ record }) {
      return h(Space, { size: 4 }, () => [
        h(Button, { size: 'small', onClick: () => openPricingModal(record) }, () => '定价'),
        h(Popconfirm, { content: '确定移除该模型？', onOk: () => removeModel(record) }, () =>
          h(Button, { size: 'small', status: 'danger' }, () => '移除')
        ),
      ])
    },
  },
]

async function fetchTenantModels() {
  modelsLoading.value = true
  try {
    const res: any = await request.get(`/admin/tenants/${props.tenantId}/models`)
    const list = res.data?.data?.list || res.data?.list || []
    // 「覆盖」列标记注入（响应不含 _override，前端按覆盖字段推导）
    for (const row of list) row._override = hasAnyOverride(row)
    modelsData.value = list
  } catch {
  } finally {
    modelsLoading.value = false
  }
}

// 候选模型走专用不分页接口 /admin/models/options：一次拉回全部 active 模型，
// 不受 /admin/models 的 page_size=100 上限约束（平台模型超 100 个时旧写法会漏模型）
async function fetchAllModels() {
  try {
    const res: any = await request.get('/admin/models/options')
    allModels.value = res.data?.data?.list || []
  } catch (err: any) {
    console.error('fetchAllModels failed:', err)
  }
}

// === 定价弹窗（TenantModelPricingModal，参考平台定价弹窗形态）===
const showPricingModal = ref(false)
const pricingModel = ref<any>(null)

function openPricingModal(record: any) {
  pricingModel.value = record
  showPricingModal.value = true
}

function onPricingSaved() {
  fetchTenantModels()
}

// === 分配模型弹窗 ===
const showAssignModal = ref(false)
const assignLoading = ref(false)
const selectedModelIds = ref<string[]>([])

const transferOptions = computed(() => {
  const assignedIds = new Set(modelsData.value.map((m: any) => m.model_id))
  return allModels.value
    .filter((m: any) => !assignedIds.has(m.id))
    .map((m: any) => ({
      value: String(m.id),
      label: `${m.model_id}${m.model_name ? ` (${m.model_name})` : ''}`,
    }))
})

function openAssignModal() {
  selectedModelIds.value = []
  showAssignModal.value = true
  if (allModels.value.length === 0) {
    fetchAllModels()
  }
}

async function handleAssign(done: () => void) {
  if (selectedModelIds.value.length === 0) {
    Message.warning('请选择要分配的模型')
    return
  }
  assignLoading.value = true
  try {
    const assignments = selectedModelIds.value.map(id => ({ model_id: Number(id), enabled: true }))
    const res: any = await request.post(`/admin/tenants/${props.tenantId}/models`, { assignments })
    const assigned = res.data?.data?.assigned ?? res.data?.assigned ?? 0
    Message.success(`成功分配 ${assigned} 个模型`)
    done()
    fetchTenantModels()
  } catch {
    return false
  } finally {
    assignLoading.value = false
  }
}

async function removeModel(record: any) {
  try {
    await request.delete(`/admin/tenants/${props.tenantId}/models/${record.model_id}`)
    Message.success('已移除')
    fetchTenantModels()
  } catch {
  }
}

// === 模型分组 ===
const groupsLoading = ref(false)
const tenantGroups = ref<any[]>([])
const allGroups = ref<any[]>([])
const showGroupModal = ref(false)
const groupAssignLoading = ref(false)
const selectedGroupIds = ref<number[]>([])

const groupColumns: TableColumnData[] = [
  { title: '分组名称', dataIndex: 'name', width: 160, ellipsis: true },
  { title: '标识', dataIndex: 'code', width: 140, ellipsis: true },
  { title: '状态', dataIndex: 'status', width: 80,
    render({ record }) {
      const color = record.status === 'active' ? 'green' : undefined
      const label = record.status === 'active' ? '启用' : '禁用'
      return h(Tag, { color, size: 'small' }, () => label)
    },
  },
  { title: '模型数', dataIndex: 'model_count', width: 80 },
]

async function fetchTenantGroups() {
  groupsLoading.value = true
  try {
    const res: any = await request.get(`/admin/tenants/${props.tenantId}/groups`)
    tenantGroups.value = res.data?.data?.list || res.data?.list || []
  } catch {
  } finally {
    groupsLoading.value = false
  }
}

async function fetchAllGroups() {
  try {
    const res: any = await request.get('/admin/model-groups/options')
    allGroups.value = res.data?.data?.list || res.data?.list || []
  } catch {
  }
}

const groupTransferOptions = computed(() => {
  return allGroups.value.map((g: any) => ({
    value: g.id,
    label: `${g.name}（${g.code}）${g.model_count ? ` - ${g.model_count}个模型` : ''}`,
  }))
})

function openGroupModal() {
  selectedGroupIds.value = tenantGroups.value.map((g: any) => g.group_id)
  showGroupModal.value = true
  if (allGroups.value.length === 0) {
    fetchAllGroups()
  }
}

async function handleSaveGroups(done: () => void) {
  groupAssignLoading.value = true
  try {
    await request.put(`/admin/tenants/${props.tenantId}/groups`, { group_ids: selectedGroupIds.value })
    Message.success('分组更新成功')
    done()
    // 分组变化会影响独立模型表（分组模型并入可用集），两张表都刷新
    fetchTenantGroups()
    fetchTenantModels()
  } catch {
    return false
  } finally {
    groupAssignLoading.value = false
  }
}

// === 可用模型预览 ===
const showPreviewModal = ref(false)
const previewLoading = ref(false)
const previewData = ref<any[]>([])

const previewColumns: TableColumnData[] = [
  { title: '模型标识', dataIndex: 'model_id', width: 180, ellipsis: true },
  { title: '显示名', dataIndex: 'model_name', width: 150, ellipsis: true },
  {
    title: '分类', dataIndex: 'category', width: 80,
    render({ record }) {
      return h(Tag, { color: modelCategoryTagColor[record.category], size: 'small' }, () => modelCategoryLabelMap[record.category] || record.category)
    },
  },
  { title: '上下文', dataIndex: 'max_context_tokens', width: 100,
    render({ record }) { return record.max_context_tokens ? record.max_context_tokens.toLocaleString() : '-' },
  },
  { title: '最大输出', dataIndex: 'max_output_tokens', width: 100,
    render({ record }) { return record.max_output_tokens ? record.max_output_tokens.toLocaleString() : '-' },
  },
  { title: '来源', dataIndex: 'source', width: 80,
    render({ record }) {
      const color = record.source === 'explicit' ? 'arcoblue' : 'green'
      const label = record.source === 'explicit' ? '独立分配' : '分组'
      return h(Tag, { color, size: 'small' }, () => label)
    },
  },
]

async function fetchAvailableModels() {
  previewLoading.value = true
  try {
    const res: any = await request.get(`/admin/tenants/${props.tenantId}/available-models`)
    previewData.value = res.data?.data?.list || res.data?.list || []
  } catch {
  } finally {
    previewLoading.value = false
  }
}

function openPreviewModal() {
  showPreviewModal.value = true
  fetchAvailableModels()
}

// 激活时刷新：模型列表 + 全量模型 + 分组（复刻原 onTabChange('models') 三连请求）
function refresh() {
  fetchTenantModels()
  fetchAllModels()
  fetchTenantGroups()
}

// 首挂拉取一次；之后每次切回该 Tab 都刷新
onMounted(refresh)
watch(() => props.active, (v) => { if (v) refresh() })

// 供父级 ATabs #extra「查看可用模型」按钮调用
defineExpose({ openPreviewModal })
</script>

<template>
  <ACard :bordered="false" class="mb-4" title="模型分组">
    <template #extra>
      <AButton type="primary" size="small" @click="openGroupModal">分配分组</AButton>
    </template>
    <div v-if="tenantGroups.length === 0" style="color: var(--ta-text-tertiary)">
      暂未分配模型分组，租户可通过分组获取可用模型
    </div>
    <ResponsiveTable
      v-else
      :columns="groupColumns"
      :data="tenantGroups"
      :loading="groupsLoading"
      :stripe="true"
      row-key="group_id"
      size="small"
      card-title-key="name"
      card-badge-key="status"
      :card-fields="['code', 'model_count']"
    />
    <div class="table-footer">
      <TableStats :total="tenantGroups.length" />
    </div>
  </ACard>

  <ACard :bordered="false" title="独立模型">
    <template #extra>
      <AButton type="primary" size="small" @click="openAssignModal">分配模型</AButton>
    </template>
    <div v-if="modelsData.length === 0" style="color: var(--ta-text-tertiary)">
      暂无独立分配的模型
    </div>
    <ResponsiveTable
      v-else
      :columns="modelColumns"
      :data="modelsData"
      :loading="modelsLoading"
      :stripe="true"
      row-key="id"
      :scroll="{ x: 1200 }"
      card-title-key="model_code"
      card-subtitle-key="model_name"
      card-badge-key="enabled"
      :card-fields="['category', 'billing_mode', 'discount_ratio', 'max_concurrency', 'version']"
    />
    <div class="table-footer">
      <TableStats :total="modelsData.length" />
    </div>
  </ACard>

  <!-- 分配分组弹窗 -->
  <AModal
    v-model:visible="showGroupModal"
    title="分配模型分组"
    :width="700"
    :on-before-ok="handleSaveGroups"
    :ok-loading="groupAssignLoading"
  >
    <div class="mb-3 text-sm" style="color: var(--ta-text-tertiary)">
      选择分组后，分组内的所有模型自动对该租户可用
    </div>
    <ATransfer
      v-model="selectedGroupIds"
      :data="groupTransferOptions"
      :title="['可选分组', '已选分组']"
      searchable
      class="tall-transfer"
    />
  </AModal>

  <!-- 分配模型弹窗 -->
  <AModal
    v-model:visible="showAssignModal"
    title="分配模型"
    :width="750"
    :on-before-ok="handleAssign"
    :ok-loading="assignLoading"
  >
    <ATransfer
      v-model="selectedModelIds"
      :data="transferOptions"
      :title="['可分配模型', '已选模型']"
      searchable
      class="tall-transfer"
    />
  </AModal>

  <!-- 可用模型预览弹窗 -->
  <AModal v-model:visible="showPreviewModal" title="租户可用模型" :width="800" :footer="false">
    <div class="mb-3 text-sm" style="color: var(--ta-text-tertiary)">
      共 {{ previewData.length }} 个模型
    </div>
    <ATable
      :columns="previewColumns"
      :data="previewData"
      :loading="previewLoading"
      :bordered="false"
      :stripe="true"
      :pagination="false"
      row-key="model_id"
      size="small"
    />
  </AModal>

  <!-- 模型定价弹窗（参考平台定价弹窗形态：分节布局 + 覆盖三态 + 平台对照） -->
  <TenantModelPricingModal
    v-model:visible="showPricingModal"
    :tenant-id="tenantId"
    :model="pricingModel"
    @saved="onPricingSaved"
  />
</template>

<style scoped>
@import './common.css';
/* 分配弹窗穿梭框：默认 224px 过矮、200px 过窄，加高加宽以显示更多选项，并在弹窗内水平居中 */
.tall-transfer {
  justify-content: center;
}
.tall-transfer :deep(.arco-transfer-view) {
  width: 260px;
  height: 460px;
}
</style>
