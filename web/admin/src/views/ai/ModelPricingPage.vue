<script setup lang="ts">
import { ref, reactive, onMounted, h, computed, watch } from 'vue'
import {
  Tag, Button, Space, Message, Popconfirm, Empty,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

// ============================================================
// Model List
// ============================================================
const modelLoading = ref(false)
const models = ref<any[]>([])
const selectedModelId = ref<number | null>(null)
const modelPagination = reactive({
  current: 1,
  pageSize: 50,
  total: 0,
  showPageSize: false,
})

const filterCategory = ref<string | null>(null)
const filterSearch = ref('')

const categoryOptions = [
  { label: '全部分类', value: '' },
  { label: '对话', value: 'chat' },
  { label: '向量', value: 'embedding' },
  { label: '图像', value: 'image' },
  { label: '音频', value: 'audio' },
  { label: '重排', value: 'rerank' },
]

const categoryTagLabel: Record<string, string> = {
  chat: '对话',
  embedding: '向量',
  image: '图像',
  audio: '音频',
  rerank: '重排',
}

const categoryTagColor: Record<string, string> = {
  chat: 'arcoblue',
  embedding: 'green',
  image: 'orangered',
  audio: 'red',
  rerank: 'arcoblue',
}

const modelColumns: TableColumnData[] = [
  { title: '模型标识', dataIndex: 'model_id', width: 200, ellipsis: true },
  { title: '显示名', dataIndex: 'model_name', width: 160, ellipsis: true },
  {
    title: '分类',
    dataIndex: 'category',
    width: 80,
    render({ record }) {
      return h(Tag, { color: categoryTagColor[record.category], size: 'small' }, () => categoryTagLabel[record.category] || record.category)
    },
  },
  {
    title: '计费模式',
    dataIndex: 'billing_mode',
    width: 100,
    render({ record }) {
      if (!record.pricing_mode) return h('span', { style: 'color: var(--color-text-4)' }, '未设置')
      const label: Record<string, string> = { token: '按量计费', per_request: '按次计费', tiered: '阶梯计费' }
      return label[record.pricing_mode] || record.pricing_mode
    },
  },
  {
    title: '输入价格',
    dataIndex: 'input_price',
    width: 120,
    render({ record }) {
      if (!record.pricing_mode || record.pricing_mode === 'per_request') return '-'
      return `$${record.input_price?.toFixed(2) ?? '0'}/1M`
    },
  },
  {
    title: '输出价格',
    dataIndex: 'output_price',
    width: 120,
    render({ record }) {
      if (!record.pricing_mode || record.pricing_mode === 'per_request') return '-'
      return `$${record.output_price?.toFixed(2) ?? '0'}/1M`
    },
  },
  {
    title: '操作',
    dataIndex: 'actions',
    width: 80,
    fixed: 'right',
    render({ record }) {
      return h(Button, { size: 'small', type: 'primary', onClick: () => openEditor(record) }, () => '定价')
    },
  },
]

async function fetchModels() {
  modelLoading.value = true
  try {
    const params: Record<string, any> = {
      page: modelPagination.current,
      page_size: modelPagination.pageSize,
    }
    if (filterCategory.value) params.category = filterCategory.value
    if (filterSearch.value) params.search = filterSearch.value
    const res: any = await request.get('/admin/models/pricing', { params })
    const raw = res.data?.data
    models.value = (raw?.list || []).filter(Boolean)
    modelPagination.total = Number(raw?.total) || 0
  } catch {
    models.value = []
    modelPagination.total = 0
  } finally {
    modelLoading.value = false
  }
}

function handleFilter() {
  modelPagination.current = 1
  fetchModels()
}

// ============================================================
// Pricing Editor
// ============================================================
const showEditor = ref(false)
const editorLoading = ref(false)
const editorSaving = ref(false)
const editorModel = ref<any>(null)
const editorBillingMode = ref('token')
const editorItems = reactive<any[]>([])

const billingModeOptions = [
  { label: '按量计费 (Token)', value: 'token' },
  { label: '按次计费', value: 'per_request' },
  { label: '阶梯计费', value: 'tiered' },
]

function createEmptyItem(mode: string) {
  return {
    billing_mode: mode,
    min_tokens: 0,
    max_tokens: null,
    input_price: 0,
    output_price: 0,
    per_request_price: null,
    cache_read_price: 0,
    cache_creation_price: 0,
  }
}

function resetEditorDefaults() {
  editorBillingMode.value = 'token'
  editorItems.length = 0
  editorItems.push(createEmptyItem('token'))
}

async function openEditor(record: any) {
  editorModel.value = record
  editorItems.length = 0
  editorBillingMode.value = 'token'
  resetOfficialData()
  showEditor.value = true
  editorLoading.value = true

  try {
    const res: any = await request.get(`/admin/models/${record.id}/pricing`)
    const list: any[] = res.data?.data?.list || []
    if (list.length > 0) {
      editorBillingMode.value = list[0].billing_mode || 'token'
      editorItems.length = 0
      for (const item of list) {
        editorItems.push({
          billing_mode: item.billing_mode,
          min_tokens: item.min_tokens || 0,
          max_tokens: item.max_tokens ?? null,
          input_price: item.input_price ?? 0,
          output_price: item.output_price ?? 0,
          per_request_price: item.per_request_price ?? null,
          cache_read_price: item.cache_read_price ?? 0,
          cache_creation_price: item.cache_creation_price ?? 0,
        })
      }
    } else {
      resetEditorDefaults()
    }
  } catch {
    resetEditorDefaults()
  } finally {
    editorLoading.value = false
  }
}

watch(editorBillingMode, (mode) => {
  if (mode === 'tiered') {
    if (editorItems.length === 0) {
      editorItems.push(createEmptyItem('tiered'))
    }
  } else {
    if (editorItems.length === 0) {
      editorItems.push(createEmptyItem(mode))
    } else {
      editorItems[0].billing_mode = mode
    }
  }
})

function addTier() {
  const last = editorItems[editorItems.length - 1]
  const newMin = last?.max_tokens ?? 0
  editorItems.push(createEmptyItem('tiered'))
  const newItem = editorItems[editorItems.length - 1]
  newItem.min_tokens = newMin
  if (last && last.max_tokens === null) {
    last.max_tokens = newMin
  }
}

function removeTier(index: number) {
  editorItems.splice(index, 1)
  if (editorItems.length > 0) {
    editorItems[editorItems.length - 1].max_tokens = null
  }
}

async function savePricing() {
  if (!editorModel.value) return
  editorSaving.value = true
  try {
    const items = editorItems.map((item, index) => ({
      billing_mode: editorBillingMode.value,
      min_tokens: editorBillingMode.value === 'tiered' ? item.min_tokens : 0,
      max_tokens: editorBillingMode.value === 'tiered' ? (index < editorItems.length - 1 ? item.max_tokens : null) : null,
      input_price: item.input_price,
      output_price: item.output_price,
      per_request_price: editorBillingMode.value === 'per_request' ? (item.per_request_price ?? null) : null,
      cache_read_price: item.cache_read_price,
      cache_creation_price: item.cache_creation_price,
    }))
    await request.put(`/admin/models/${editorModel.value.id}/pricing`, { items })
    Message.success('定价已保存')
    showEditor.value = false
    fetchModels()
  } catch {
    // error handled by interceptor
  } finally {
    editorSaving.value = false
  }
}

// ============================================================
// Official Pricing Fetch
// ============================================================
const officialLoading = ref(false)
const officialData = ref<any>(null)
const officialError = ref('')

async function fetchOfficialPricing() {
  if (!editorModel.value) return
  officialLoading.value = true
  officialError.value = ''
  officialData.value = null
  try {
    const res: any = await request.get(`/admin/models/${editorModel.value.id}/official-pricing`)
    officialData.value = res.data?.data
  } catch {
    officialError.value = '获取官方定价失败'
  } finally {
    officialLoading.value = false
  }
}

function applyOfficialPricing(pricing: any) {
  if (!pricing) return
  if (editorItems.length > 0) {
    editorItems[0].input_price = pricing.input_price
    editorItems[0].output_price = pricing.output_price
    editorItems[0].cache_read_price = pricing.cache_read_price
    editorItems[0].cache_creation_price = pricing.cache_creation_price
  }
  if (pricing.billing_mode && pricing.billing_mode !== editorBillingMode.value) {
    editorBillingMode.value = pricing.billing_mode
  }
  Message.success('已填入官方参考定价')
}

// Reset official data when editor opens for a different model
function resetOfficialData() {
  officialData.value = null
  officialError.value = ''
}

// ============================================================
// Quick Stats
// ============================================================
const stats = computed(() => {
  const total = models.value.length
  const priced = models.value.filter((m: any) => m.pricing_mode).length
  return { total, priced, unpriced: total - priced }
})

// ============================================================
// Batch reset
// ============================================================
async function batchReset() {
  if (!selectedModelId.value) return
  try {
    await request.put(`/admin/models/${selectedModelId.value}/pricing`, {
      items: [{ billing_mode: 'token', input_price: 0, output_price: 0, cache_read_price: 0, cache_creation_price: 0 }],
    })
    Message.success('已重置')
    fetchModels()
  } catch {
    // handled by interceptor
  }
}

onMounted(() => {
  fetchModels()
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="模型定价" description="管理各模型的计费价格和梯度定价策略" />

    <!-- Stats -->
    <div class="stats-bar">
      <div class="stat-item">
        <span class="stat-value">{{ stats.total }}</span>
        <span class="stat-label">模型总数</span>
      </div>
      <div class="stat-item stat-item--success">
        <span class="stat-value">{{ stats.priced }}</span>
        <span class="stat-label">已设置定价</span>
      </div>
      <div class="stat-item stat-item--warn">
        <span class="stat-value">{{ stats.unpriced }}</span>
        <span class="stat-label">未设置定价</span>
      </div>
    </div>

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
        <AInput
          v-model="filterSearch"
          placeholder="搜索模型名..."
          allow-clear
          style="width: 200px"
          @keydown.enter="handleFilter"
          @clear="handleFilter"
        />
        <AButton type="primary" @click="handleFilter">搜索</AButton>
      </ASpace>
    </ACard>

    <!-- Model Table -->
    <ACard :bordered="false">
      <ATable
        :columns="modelColumns"
        :data="models"
        :loading="modelLoading"
        :scroll="{ x: 900 }"
        :bordered="false"
        :stripe="true"
        :pagination="false"
        row-key="id"
      />
      <div class="table-footer">
        <APagination
          v-model:current="modelPagination.current"
          v-model:page-size="modelPagination.pageSize"
          :total="modelPagination.total"
          :page-size-options="[20, 50, 100]"
          show-page-size
          show-total
          @change="fetchModels"
          @page-size-change="(size: number) => { modelPagination.pageSize = size; modelPagination.current = 1; fetchModels() }"
        />
      </div>
    </ACard>

    <!-- Pricing Editor Drawer -->
    <ADrawer
      v-model:visible="showEditor"
      :width="720"
      :title="editorModel ? `定价设置 - ${editorModel.model_id}` : '定价设置'"
      :mask-closable="false"
    >
      <ASpin :loading="editorLoading" style="width: 100%">
        <div v-if="editorModel" class="pricing-editor">
          <!-- Official Pricing Reference -->
          <div class="editor-section">
            <div class="editor-section-header">
              <h3>官方参考定价</h3>
              <span class="section-hint">数据来源: LiteLLM + models.dev</span>
            </div>
            <AButton
              type="outline"
              size="small"
              :loading="officialLoading"
              @click="fetchOfficialPricing"
            >拉取官方定价</AButton>

            <div v-if="officialData" class="official-pricing-card mt-3">
              <div v-for="src in officialData.sources" :key="src.source" class="official-source-block">
                <div class="official-source-header">
                  <ATag size="small" :color="src.source === 'litellm' ? 'arcoblue' : 'green'">{{ src.source === 'litellm' ? 'LiteLLM' : 'models.dev' }}</ATag>
                  <template v-if="src.found && src.pricing">
                    <ATag v-if="src.provider" size="small" color="orangered">{{ src.provider }}</ATag>
                    <ATag v-if="src.mode" size="small">{{ src.mode }}</ATag>
                    <span v-if="src.max_context_tokens" class="official-meta-text">上下文: {{ (src.max_context_tokens / 1000).toFixed(0) }}K</span>
                    <span v-if="src.max_output_tokens" class="official-meta-text">输出上限: {{ (src.max_output_tokens / 1000).toFixed(0) }}K</span>
                  </template>
                </div>
                <template v-if="src.found && src.pricing">
                  <div class="official-prices mt-2">
                    <div class="official-price-row">
                      <span class="official-price-label">输入价格</span>
                      <span class="official-price-value">${{ src.pricing.input_price.toFixed(4) }} / 1M</span>
                    </div>
                    <div class="official-price-row">
                      <span class="official-price-label">输出价格</span>
                      <span class="official-price-value">${{ src.pricing.output_price.toFixed(4) }} / 1M</span>
                    </div>
                    <div class="official-price-row">
                      <span class="official-price-label">缓存读取</span>
                      <span class="official-price-value">{{ src.pricing.cache_read_price ? '$' + src.pricing.cache_read_price.toFixed(4) + ' / 1M' : '—' }}</span>
                    </div>
                    <div class="official-price-row">
                      <span class="official-price-label">缓存创建</span>
                      <span class="official-price-value">{{ src.pricing.cache_creation_price ? '$' + src.pricing.cache_creation_price.toFixed(4) + ' / 1M' : '—' }}</span>
                    </div>
                  </div>
                  <AButton type="primary" size="small" class="mt-2" @click="applyOfficialPricing(src.pricing)">应用此价格</AButton>
                </template>
                <template v-else>
                  <div class="official-not-found">未找到定价数据</div>
                </template>
              </div>
            </div>
            <div v-if="officialError" class="official-error mt-2">{{ officialError }}</div>
          </div>

          <!-- Billing Mode -->
          <div class="editor-section">
            <div class="editor-section-header">
              <h3>计费模式</h3>
            </div>
            <ARadioGroup v-model="editorBillingMode" type="button">
              <ARadio v-for="opt in billingModeOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</ARadio>
            </ARadioGroup>
          </div>

          <!-- Per-request Mode -->
          <template v-if="editorBillingMode === 'per_request' && editorItems.length > 0">
            <div class="editor-section">
              <div class="editor-section-header">
                <h3>按次定价</h3>
              </div>
              <div class="form-row">
                <AFormItem label="按次单价" class="flex-1">
                  <AInputNumber
                    v-model="editorItems[0].per_request_price"
                    :min="0"
                    :precision="4"
                    placeholder="每次调用价格"
                    class="w-full"
                  >
                    <template #suffix>$ / 次</template>
                  </AInputNumber>
                </AFormItem>
              </div>
            </div>
          </template>

          <!-- Token Mode -->
          <template v-else-if="editorBillingMode === 'token' && editorItems.length > 0">
            <div class="editor-section">
              <div class="editor-section-header">
                <h3>按量定价</h3>
                <span class="section-hint">价格单位为 $ / 1M Token</span>
              </div>
              <div class="form-row form-row--two">
                <AFormItem label="输入价格" class="flex-1">
                  <AInputNumber
                    v-model="editorItems[0].input_price"
                    :min="0"
                    :precision="4"
                    placeholder="0"
                    class="w-full"
                  >
                    <template #suffix>$ / 1M</template>
                  </AInputNumber>
                </AFormItem>
                <AFormItem label="输出价格" class="flex-1">
                  <AInputNumber
                    v-model="editorItems[0].output_price"
                    :min="0"
                    :precision="4"
                    placeholder="0"
                    class="w-full"
                  >
                    <template #suffix>$ / 1M</template>
                  </AInputNumber>
                </AFormItem>
              </div>
              <div class="form-row form-row--two mt-3">
                <AFormItem label="缓存读取价格" class="flex-1">
                  <AInputNumber
                    v-model="editorItems[0].cache_read_price"
                    :min="0"
                    :precision="4"
                    placeholder="0"
                    class="w-full"
                  >
                    <template #suffix>$ / 1M</template>
                  </AInputNumber>
                </AFormItem>
                <AFormItem label="缓存创建价格" class="flex-1">
                  <AInputNumber
                    v-model="editorItems[0].cache_creation_price"
                    :min="0"
                    :precision="4"
                    placeholder="0"
                    class="w-full"
                  >
                    <template #suffix>$ / 1M</template>
                  </AInputNumber>
                </AFormItem>
              </div>
            </div>
          </template>

          <!-- Tiered Mode -->
          <template v-else>
            <div class="editor-section">
              <div class="editor-section-header">
                <h3>阶梯定价</h3>
                <span class="section-hint">按 Token 用量分段设置不同价格</span>
              </div>
              <div v-for="(tier, index) in editorItems" :key="index" class="tier-card">
                <div class="tier-header">
                  <span class="tier-label">第 {{ index + 1 }} 梯</span>
                  <AButton
                    v-if="editorItems.length > 1"
                    size="mini"
                    status="danger"
                    @click="removeTier(index)"
                  >删除</AButton>
                </div>
                <div class="form-row form-row--two">
                  <AFormItem label="起始 Token" class="flex-1">
                    <AInputNumber
                      v-model="tier.min_tokens"
                      :min="0"
                      :step="1000"
                      placeholder="0"
                      class="w-full"
                    />
                  </AFormItem>
                  <AFormItem label="结束 Token" class="flex-1">
                    <AInputNumber
                      v-if="index < editorItems.length - 1"
                      v-model="tier.max_tokens"
                      :min="0"
                      :step="1000"
                      placeholder="上限"
                      class="w-full"
                    />
                    <AInput v-else model-value="无上限" disabled class="w-full" />
                  </AFormItem>
                </div>
                <div class="form-row form-row--two mt-2">
                  <AFormItem label="输入价格" class="flex-1">
                    <AInputNumber
                      v-model="tier.input_price"
                      :min="0"
                      :precision="4"
                      class="w-full"
                    >
                      <template #suffix>$/1M</template>
                    </AInputNumber>
                  </AFormItem>
                  <AFormItem label="输出价格" class="flex-1">
                    <AInputNumber
                      v-model="tier.output_price"
                      :min="0"
                      :precision="4"
                      class="w-full"
                    >
                      <template #suffix>$/1M</template>
                    </AInputNumber>
                  </AFormItem>
                </div>
                <div class="form-row form-row--two mt-2">
                  <AFormItem label="缓存读取价格" class="flex-1">
                    <AInputNumber
                      v-model="tier.cache_read_price"
                      :min="0"
                      :precision="4"
                      class="w-full"
                    >
                      <template #suffix>$/1M</template>
                    </AInputNumber>
                  </AFormItem>
                  <AFormItem label="缓存创建价格" class="flex-1">
                    <AInputNumber
                      v-model="tier.cache_creation_price"
                      :min="0"
                      :precision="4"
                      class="w-full"
                    >
                      <template #suffix>$/1M</template>
                    </AInputNumber>
                  </AFormItem>
                </div>
              </div>
              <AButton type="dashed" long @click="addTier" class="mt-3">+ 添加梯度</AButton>
            </div>
          </template>

          <!-- Save -->
          <div class="editor-actions">
            <AButton @click="showEditor = false">取消</AButton>
            <AButton type="primary" :loading="editorSaving" @click="savePricing">保存定价</AButton>
          </div>
        </div>
      </ASpin>
    </ADrawer>
  </div>
</template>

<style scoped>
.stats-bar {
  display: flex;
  gap: 16px;
  margin-bottom: 16px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  padding: 12px 20px;
  background: var(--ta-bg-card);
  border: 1px solid var(--ta-border-light);
  border-radius: 8px;
  min-width: 120px;
}

.stat-value {
  font-size: 22px;
  font-weight: 700;
  color: var(--ta-text-primary);
}

.stat-label {
  font-size: 12px;
  color: var(--ta-text-tertiary);
  margin-top: 2px;
}

.editor-section {
  margin-bottom: 24px;
}

.editor-section-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.editor-section-header h3 {
  font-size: 14px;
  font-weight: 600;
  color: var(--ta-text-primary);
  margin: 0;
}

.section-hint {
  font-size: 12px;
  color: var(--ta-text-tertiary);
}

.form-row {
  display: flex;
  gap: 16px;
}

.form-row--two {
  flex-wrap: wrap;
}

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

.editor-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 20px;
  border-top: 1px solid var(--ta-border-light);
}

.official-pricing-card {
  padding: 12px 16px;
  background: var(--color-fill-1);
  border-radius: 8px;
  border: 1px solid var(--ta-border-light);
}

.official-source-block {
  padding: 10px 0;
  border-bottom: 1px dashed var(--ta-border-light);
}

.official-source-block:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.official-source-block:first-child {
  padding-top: 0;
}

.official-source-header {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.official-meta-text {
  font-size: 12px;
  color: var(--ta-text-tertiary);
}

.official-price-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 0;
}

.official-price-label {
  font-size: 13px;
  color: var(--ta-text-secondary);
}

.official-price-value {
  font-size: 13px;
  font-weight: 500;
  color: var(--ta-text-primary);
  font-family: monospace;
}

.official-not-found {
  font-size: 13px;
  color: var(--ta-text-tertiary);
}

.official-error {
  font-size: 13px;
  color: rgb(var(--danger-6));
}

.table-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--ta-border-light);
}

@media (max-width: 640px) {
  .stats-bar {
    flex-wrap: wrap;
  }
  .stat-item {
    flex: 1 1 100px;
  }
}
</style>
