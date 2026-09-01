<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted, h } from 'vue'
import { Tag, Button, Space, Popconfirm, Message } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import TableStats from '@/components/TableStats.vue'
import request from '@/utils/request'
// 本位币符号：定价输入控件后缀跟随本位币，输入值仍为 bil 层存储原值不折算
import { currencySymbol } from '@/composables/useCurrency'

const props = defineProps<{
  tenantId: string
  active: boolean
}>()

// === 独立模型列表 ===
const modelsLoading = ref(false)
const modelsData = ref<any[]>([])
const allModels = ref<any[]>([])

const modelColumns: TableColumnData[] = [
  { title: '模型标识', dataIndex: 'model_code', width: 180, ellipsis: true },
  { title: '显示名', dataIndex: 'model_name', width: 150, ellipsis: true },
  { title: '分类', dataIndex: 'category', width: 80 },
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
    title: '并发', dataIndex: 'max_concurrency', width: 70,
    render({ record }) { return record.max_concurrency || '-' },
  },
  { title: '版本', dataIndex: 'version', width: 60 },
  {
    title: '操作', dataIndex: 'actions', width: 160, fixed: 'right',
    render({ record }) {
      return h(Space, { size: 4 }, () => [
        h(Button, { size: 'small', onClick: () => openEditModel(record) }, () => '编辑'),
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
    modelsData.value = res.data?.data?.list || res.data?.list || []
  } catch {
  } finally {
    modelsLoading.value = false
  }
}

async function fetchAllModels() {
  try {
    const res: any = await request.get('/admin/models', { params: { page: 1, page_size: 100, status: 'active' } })
    allModels.value = res.data?.data?.list || res.data?.list || []
  } catch (err: any) {
    console.error('fetchAllModels failed:', err)
  }
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

// === 编辑模型抽屉 ===
const showEditModelModal = ref(false)
const editModelLoading = ref(false)
const editingModel = ref<any>(null)
const editModelForm = reactive({
  enabled: true,
  billing_mode: null as string | null,
  per_request_price: null as number | null,
  discount_ratio: null as number | null,
  max_concurrency: 5 as number | null,
  custom_input_price: null as number | null,
  custom_output_price: null as number | null,
  custom_cache_read_price: null as number | null,
  custom_cache_creation_price: null as number | null,
  custom_pricing_tiers: [] as any[],
})

let suppressBillingWatch = false

function openEditModel(record: any) {
  suppressBillingWatch = true
  editingModel.value = record
  editModelForm.enabled = record.enabled
  editModelForm.billing_mode = record.billing_mode || null
  editModelForm.per_request_price = record.per_request_price || null
  editModelForm.discount_ratio = record.discount_ratio || null
  editModelForm.max_concurrency = record.max_concurrency ?? 5
  editModelForm.custom_input_price = record.custom_input_price || null
  editModelForm.custom_output_price = record.custom_output_price || null
  editModelForm.custom_cache_read_price = record.custom_cache_read_price || null
  editModelForm.custom_cache_creation_price = record.custom_cache_creation_price || null
  editModelForm.custom_pricing_tiers = record.custom_pricing_tiers?.length > 0 ? [...record.custom_pricing_tiers] : []
  showEditModelModal.value = true
  suppressBillingWatch = false
}

function addTenantTier() {
  const tiers = editModelForm.custom_pricing_tiers
  const last = tiers[tiers.length - 1]
  const newMin = last?.max_tokens ?? 0
  tiers.push({
    min_tokens: newMin,
    max_tokens: null,
    input_price: 0,
    output_price: 0,
    cache_read_price: 0,
    cache_creation_price: 0,
  })
  if (last && last.max_tokens === null) {
    last.max_tokens = newMin
  }
}

// 计费模式切换时清理互斥字段
watch(() => editModelForm.billing_mode, (mode) => {
  if (suppressBillingWatch) return
  if (!mode) {
    // 默认模式：清空自定义价格，保留折扣
    editModelForm.custom_input_price = null
    editModelForm.custom_output_price = null
    editModelForm.custom_cache_read_price = null
    editModelForm.custom_cache_creation_price = null
    editModelForm.per_request_price = null
    editModelForm.custom_pricing_tiers = []
  } else {
    // 自定义价格模式：清空折扣比例
    editModelForm.discount_ratio = null
  }
})

async function handleEditModel() {
  if (!editingModel.value) return
  editModelLoading.value = true
  try {
    const payload: any = {
      ...editModelForm,
      version: editingModel.value.version,
    }
    await request.put(`/admin/tenants/${props.tenantId}/models/${editingModel.value.model_id}`, payload)
    Message.success('更新成功')
    showEditModelModal.value = false
    fetchTenantModels()
  } catch {
    // 错误已由拦截器统一提示
  } finally {
    editModelLoading.value = false
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
  { title: '分类', dataIndex: 'category', width: 80 },
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
      :scroll="{ x: 1100 }"
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

  <!-- 编辑模型配置抽屉 -->
  <ADrawer
    v-model:visible="showEditModelModal"
    :title="`编辑模型 - ${editingModel?.model_name || ''}`"
    :width="640"
    :mask-closable="false"
    :footer="true"
  >
    <AForm :model="editModelForm" :auto-label-width="true" layout="vertical">
      <AFormItem label="启用">
        <ASwitch v-model="editModelForm.enabled" />
      </AFormItem>
      <AFormItem label="单模型并发上限">
        <AInputNumber v-model="editModelForm.max_concurrency" :min="0" placeholder="默认5" class="w-full" />
      </AFormItem>

      <AFormItem label="计费模式">
        <ARadioGroup v-model="editModelForm.billing_mode" type="button">
          <ARadio value="">默认</ARadio>
          <ARadio value="token">Token</ARadio>
          <ARadio value="per_request">按次</ARadio>
          <ARadio value="tiered">阶梯</ARadio>
        </ARadioGroup>
      </AFormItem>

      <!-- 默认模式：仅折扣比例 -->
      <template v-if="!editModelForm.billing_mode">
        <ADivider margin="8px">折扣</ADivider>
        <div style="color: var(--ta-text-tertiary); font-size: 12px; margin-bottom: 12px">使用模型基础定价 × 折扣比例</div>
        <AFormItem label="折扣比例">
          <AInputNumber v-model="editModelForm.discount_ratio" :min="0" :max="1" :step="0.05" :precision="2" placeholder="如 0.8 = 8折" class="w-full" />
        </AFormItem>
      </template>

      <!-- Token 模式：自定义价格 -->
      <template v-if="editModelForm.billing_mode === 'token'">
        <ADivider margin="8px">自定义价格</ADivider>
        <div style="color: var(--ta-text-tertiary); font-size: 12px; margin-bottom: 12px">留空则使用模型基础定价</div>
        <div class="grid grid-cols-2 gap-x-3">
          <AFormItem label="输入价格">
            <AInputNumber v-model="editModelForm.custom_input_price" :min="0" :precision="4" placeholder="默认" class="w-full">
              <template #suffix>{{ currencySymbol }} / 1M</template>
            </AInputNumber>
          </AFormItem>
          <AFormItem label="输出价格">
            <AInputNumber v-model="editModelForm.custom_output_price" :min="0" :precision="4" placeholder="默认" class="w-full">
              <template #suffix>{{ currencySymbol }} / 1M</template>
            </AInputNumber>
          </AFormItem>
          <AFormItem label="缓存读取价格">
            <AInputNumber v-model="editModelForm.custom_cache_read_price" :min="0" :precision="4" placeholder="默认" class="w-full">
              <template #suffix>{{ currencySymbol }} / 1M</template>
            </AInputNumber>
          </AFormItem>
          <AFormItem label="缓存创建价格">
            <AInputNumber v-model="editModelForm.custom_cache_creation_price" :min="0" :precision="4" placeholder="默认" class="w-full">
              <template #suffix>{{ currencySymbol }} / 1M</template>
            </AInputNumber>
          </AFormItem>
        </div>
      </template>

      <!-- 按次计费 -->
      <template v-if="editModelForm.billing_mode === 'per_request'">
        <ADivider margin="8px">按次定价</ADivider>
        <AFormItem label="按次单价">
          <AInputNumber v-model="editModelForm.per_request_price" :min="0" :precision="4" placeholder="每次调用价格" class="w-full">
            <template #suffix>{{ currencySymbol }} / 次</template>
          </AInputNumber>
        </AFormItem>
      </template>

      <!-- 阶梯计费 -->
      <template v-if="editModelForm.billing_mode === 'tiered'">
        <ADivider margin="8px">阶梯定价</ADivider>
        <div style="color: var(--ta-text-tertiary); font-size: 12px; margin-bottom: 12px">按 Token 用量分段设置不同价格，留空则使用模型基础阶梯定价</div>
        <div v-for="(tier, index) in editModelForm.custom_pricing_tiers" :key="index" class="tier-card">
          <div class="tier-header">
            <span class="tier-label">第 {{ index + 1 }} 梯</span>
            <AButton v-if="editModelForm.custom_pricing_tiers.length > 1" size="mini" status="danger" @click="editModelForm.custom_pricing_tiers.splice(index, 1)">删除</AButton>
          </div>
          <div class="grid grid-cols-2 gap-x-3">
            <AFormItem label="起始 Token">
              <AInputNumber v-model="tier.min_tokens" :min="0" :step="1000" placeholder="0" class="w-full" />
            </AFormItem>
            <AFormItem label="结束 Token">
              <AInputNumber v-if="index < editModelForm.custom_pricing_tiers.length - 1" v-model="tier.max_tokens" :min="0" :step="1000" placeholder="上限" class="w-full" />
              <AInput v-else model-value="无上限" disabled class="w-full" />
            </AFormItem>
          </div>
          <div class="grid grid-cols-2 gap-x-3 mt-2">
            <AFormItem label="输入价格">
              <AInputNumber v-model="tier.input_price" :min="0" :precision="4" class="w-full">
                <template #suffix>{{ currencySymbol }}/1M</template>
              </AInputNumber>
            </AFormItem>
            <AFormItem label="输出价格">
              <AInputNumber v-model="tier.output_price" :min="0" :precision="4" class="w-full">
                <template #suffix>{{ currencySymbol }}/1M</template>
              </AInputNumber>
            </AFormItem>
          </div>
          <div class="grid grid-cols-2 gap-x-3 mt-2">
            <AFormItem label="缓存读取价格">
              <AInputNumber v-model="tier.cache_read_price" :min="0" :precision="4" class="w-full">
                <template #suffix>{{ currencySymbol }}/1M</template>
              </AInputNumber>
            </AFormItem>
            <AFormItem label="缓存创建价格">
              <AInputNumber v-model="tier.cache_creation_price" :min="0" :precision="4" class="w-full">
                <template #suffix>{{ currencySymbol }}/1M</template>
              </AInputNumber>
            </AFormItem>
          </div>
        </div>
        <AButton type="dashed" long @click="addTenantTier" class="mt-2">+ 添加梯度</AButton>
      </template>

    </AForm>
    <template #footer>
      <ASpace>
        <AButton @click="showEditModelModal = false">取消</AButton>
        <AButton type="primary" :loading="editModelLoading" @click="handleEditModel">保存</AButton>
      </ASpace>
    </template>
  </ADrawer>
</template>

<style scoped>
@import './common.css';
.tier-card {
  padding: 12px 16px;
  background: var(--color-fill-1);
  border-radius: 8px;
  margin-bottom: 8px;
}
.tier-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.tier-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--ta-text-secondary);
}
</style>
