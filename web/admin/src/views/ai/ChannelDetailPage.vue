<script setup lang="ts">
import { ref, reactive, computed, onMounted, h, nextTick, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Tag, Button, Popconfirm, Message, RadioGroup, Radio } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import * as echarts from 'echarts'

const route = useRoute()
const router = useRouter()

const channelId = route.params.id as string

const loading = ref(false)
const detail = ref<any>(null)
const activeTab = ref('info')

const statusTagColor: Record<string, string> = { active: 'green', disabled: 'orangered', testing: 'arcoblue' }
const statusLabel: Record<string, string> = { active: '启用', disabled: '禁用', testing: '测试中' }
const healthColor = (score: number | null | undefined) => {
  if (score === null || score === undefined) return '#94a3b8'
  if (score >= 80) return '#10b981'
  if (score >= 50) return '#f59e0b'
  return '#ef4444'
}

async function fetchDetail() {
  loading.value = true
  try {
    const res: any = await request.get(`/admin/channels/${channelId}`)
    detail.value = res.data?.data || res.data
  } catch {
    // error handled by interceptor
  } finally {
    loading.value = false
  }
}

function goBack() {
  router.push({ name: 'AdminChannels' })
}

// === Tab 1: Basic Info ===
const showEditModal = ref(false)
const editLoading = ref(false)
const editForm = reactive({
  name: '',
  base_url: '',
  priority: 0,
  weight: 100,
  test_model: '',
  remark: '',
  status: 'active',
  is_vip: false,
  use_proxy: false,
  sharing_threshold: null as number | null,
  preemption_threshold: null as number | null,
  borrowing_cooldown_seconds: null as number | null,
})

function openEditModal() {
  if (!detail.value) return
  Object.assign(editForm, {
    name: detail.value.name || '',
    base_url: detail.value.base_url || '',
    priority: detail.value.priority || 0,
    weight: detail.value.weight || 100,
    test_model: detail.value.test_model || '',
    remark: detail.value.remark || '',
    status: detail.value.status || 'active',
    is_vip: detail.value.is_vip || false,
    use_proxy: detail.value.use_proxy || false,
    sharing_threshold: detail.value.sharing_threshold ?? null,
    preemption_threshold: detail.value.preemption_threshold ?? null,
    borrowing_cooldown_seconds: detail.value.borrowing_cooldown_seconds ?? null,
  })
  showEditModal.value = true
}

async function handleEditSubmit(done: () => void) {
  editLoading.value = true
  try {
    await request.put(`/admin/channels/${channelId}`, editForm)
    Message.success('更新成功')
    done()
    fetchDetail()
  } catch { return false } finally { editLoading.value = false }
}

async function updateStatus(status: string) {
  try {
    await request.put(`/admin/channels/${channelId}`, { status, version: detail.value.version })
    Message.success('状态更新成功')
    fetchDetail()
  } catch {
    // error handled by interceptor
  }
}

async function deleteChannel() {
  try {
    await request.delete(`/admin/channels/${channelId}`)
    Message.success('渠道已删除')
    goBack()
  } catch {
    // error handled by interceptor
  }
}

// === Tab 2: Model Abilities ===
const abilitiesLoading = ref(false)
const abilitiesData = ref<any[]>([])

const abilityColumns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 60 },
  { title: '平台模型名', dataIndex: 'model_name', width: 200 },
  { title: '上游模型名', dataIndex: 'upstream_model', width: 200 },
  {
    title: '状态', dataIndex: 'enabled', width: 80,
    render({ record }) {
      return h(Tag, {
        color: record.enabled ? 'green' : undefined,
        size: 'small',
        style: 'cursor:pointer',
        onClick: () => handleToggleAbility(record),
      }, () => record.enabled ? '启用' : '禁用')
    },
  },
  {
    title: '操作', dataIndex: 'actions', width: 80, fixed: 'right',
    render({ record }) {
      return h(Popconfirm, { content: '确定移除该能力？', onOk: () => handleDeleteAbility(record.id) }, () =>
        h(Button, { size: 'mini', status: 'danger' }, () => '移除')
      )
    },
  },
]

async function fetchAbilities() {
  abilitiesLoading.value = true
  try {
    const res: any = await request.get(`/admin/channels/${channelId}/abilities`)
    abilitiesData.value = res.data?.data?.list || res.data?.list || []
  } catch { abilitiesData.value = [] } finally { abilitiesLoading.value = false }
}

async function handleToggleAbility(ab: any) {
  const newList = abilitiesData.value.map(a => a.id === ab.id ? { ...a, enabled: !a.enabled } : a)
  try {
    await request.put(`/admin/channels/${channelId}/abilities`, {
      channel_id: Number(channelId),
      abilities: newList.map(a => ({ model_name: a.model_name, upstream_model: a.upstream_model || '', enabled: a.enabled })),
    })
    abilitiesData.value = newList
    Message.success('状态已更新')
  } catch { /* error handled by interceptor */ }
}

async function handleDeleteAbility(id: number) {
  const newList = abilitiesData.value.filter(a => a.id !== id)
  try {
    await request.put(`/admin/channels/${channelId}/abilities`, {
      channel_id: Number(channelId),
      abilities: newList.map(a => ({ model_name: a.model_name, upstream_model: a.upstream_model || '', enabled: a.enabled })),
    })
    Message.success('能力已移除')
    fetchAbilities()
  } catch { /* error handled by interceptor */ }
}

// Add Ability Modal
const showAddAbilityModal = ref(false)
const addAbilityForm = reactive({ model_name: '', upstream_model: '', enabled: true })
const modelsList = ref<any[]>([])
const modelsLoading = ref(false)
let modelSearchTimer: any = null

async function fetchModels(search?: string) {
  modelsLoading.value = true
  try {
    const res: any = await request.get('/admin/models', {
      params: { page: 1, page_size: 20, status: 'active', search: search || undefined },
    })
    modelsList.value = res.data?.data?.list || res.data?.list || []
  } catch { modelsList.value = [] } finally { modelsLoading.value = false }
}

function handleModelSearch(val: string) {
  clearTimeout(modelSearchTimer)
  modelSearchTimer = setTimeout(() => fetchModels(val), 300)
}

const availableModelOptions = computed(() => {
  const usedNames = new Set(abilitiesData.value.map(a => a.model_name))
  return modelsList.value
    .filter(m => !usedNames.has(m.model_id))
    .map(m => ({ label: m.model_name ? `${m.model_name} (${m.model_id})` : m.model_id, value: m.model_id }))
})

const allModelOptions = computed(() => {
  return modelsList.value
    .map(m => ({ label: m.model_name ? `${m.model_name} (${m.model_id})` : m.model_id, value: m.model_id }))
})

function openAddAbilityModal() {
  addAbilityForm.model_name = ''
  addAbilityForm.upstream_model = ''
  addAbilityForm.enabled = true
  showAddAbilityModal.value = true
  fetchModels()
}

async function handleAddAbility(done: () => void) {
  if (!addAbilityForm.model_name) { Message.warning('请输入模型名'); return }
  const newList = [...abilitiesData.value, { model_name: addAbilityForm.model_name, upstream_model: addAbilityForm.upstream_model, enabled: addAbilityForm.enabled }]
  try {
    await request.put(`/admin/channels/${channelId}/abilities`, {
      channel_id: Number(channelId),
      abilities: newList.map(a => ({ model_name: a.model_name, upstream_model: a.upstream_model || '', enabled: a.enabled })),
    })
    Message.success('能力已添加')
    done()
    addAbilityForm.model_name = ''
    addAbilityForm.upstream_model = ''
    addAbilityForm.enabled = true
    fetchAbilities()
  } catch { return false }
}

// === Tab 3: Test ===
const testModelName = ref('')
const testLoading = ref(false)
const testResult = ref<any>(null)

async function handleTest() {
  if (!testModelName.value) { Message.warning('请输入测试模型名'); return }
  testLoading.value = true
  testResult.value = null
  try {
    const res: any = await request.post(`/admin/channels/${channelId}/test`, { model_name: testModelName.value })
    const raw = res.data?.data ?? res.data
    // 尝试解析上游响应
    let parsedResponse = null
    if (raw?.response) {
      try { parsedResponse = JSON.parse(raw.response) } catch { parsedResponse = raw.response }
    }
    let content = null
    if (parsedResponse && typeof parsedResponse === 'object') {
      content = parsedResponse?.choices?.[0]?.message?.content
        || parsedResponse?.content?.[0]?.text
        || parsedResponse?.output?.text
        || null
    }
    testResult.value = {
      success: raw?.success ?? true,
      latency_ms: raw?.latency_ms ?? 0,
      model_name: raw?.model_name || testModelName.value,
      content,
      usage: (parsedResponse && typeof parsedResponse === 'object') ? parsedResponse?.usage : null,
      error: raw?.error || null,
      request: raw?.request || null,
      response: raw?.response || null,
    }
  } catch (err: any) {
    testResult.value = { success: false, message: err?.response?.data?.message || err?.message || '测试失败' }
  } finally { testLoading.value = false }
}

// === Tab 4: Health Trend ===
const trendLoading = ref(false)
const trendData = ref<any[]>([])
const trendHours = ref(24)
let trendChart: echarts.ECharts | null = null

async function fetchHealthTrend() {
  trendLoading.value = true
  try {
    const res: any = await request.get(`/admin/channels/${channelId}/health_trend`, { params: { hours: trendHours.value } })
    trendData.value = res.data?.data?.points || res.data?.points || []
    nextTick(() => renderTrendChart())
  } catch { trendData.value = [] } finally { trendLoading.value = false }
}

function renderTrendChart() {
  const el = document.getElementById('health-trend-chart')
  if (!el) return
  if (!trendChart) {
    trendChart = echarts.init(el)
  }
  if (trendData.value.length === 0) {
    trendChart.clear()
    return
  }
  const times = trendData.value.map((p: any) => p.snapshot_at)
  const scores = trendData.value.map((p: any) => p.health_score)
  const latencies = trendData.value.map((p: any) => p.latency_ms)
  trendChart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: ['健康度', '延迟(ms)'], bottom: 0 },
    grid: { left: 50, right: 50, top: 30, bottom: 45 },
    xAxis: { type: 'category', data: times, axisLabel: { fontSize: 11 } },
    yAxis: [
      { type: 'value', name: '健康度', min: 0, max: 100, splitLine: { show: false } },
      { type: 'value', name: '延迟(ms)', min: 0, splitLine: { show: false } },
    ],
    series: [
      {
        name: '健康度', type: 'line', data: scores, smooth: true, lineStyle: { width: 2 },
        itemStyle: { color: '#10b981' },
        areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: 'rgba(16,185,129,0.3)' },
          { offset: 1, color: 'rgba(16,185,129,0.02)' },
        ]) },
        markLine: { silent: true, data: [{ yAxis: 50, lineStyle: { color: '#f59e0b', type: 'dashed' } }] },
      },
      {
        name: '延迟(ms)', type: 'bar', yAxisIndex: 1, data: latencies, barWidth: 4,
        itemStyle: { color: 'rgba(99,102,241,0.25)', borderRadius: [2, 2, 0, 0] },
      },
    ],
  })
}

function handleTrendHoursChange(val: string | number | boolean) {
  trendHours.value = Number(val)
  fetchHealthTrend()
}

onBeforeUnmount(() => {
  if (trendChart) { trendChart.dispose(); trendChart = null }
})

function onTabChange(key: string) {
  if (key === 'abilities' && abilitiesData.value.length === 0) fetchAbilities()
  if (key === 'test' && modelsList.value.length === 0) fetchModels()
  if (key === 'health_trend' && trendData.value.length === 0) fetchHealthTrend()
}

// === Update Key ===
const showUpdateKeyModal = ref(false)
const newApiKey = ref('')
const updateKeyLoading = ref(false)

async function handleUpdateKey(done: () => void) {
  if (!newApiKey.value) { Message.warning('请输入新的 API Key'); return }
  updateKeyLoading.value = true
  try {
    await request.put(`/admin/channels/${channelId}`, { api_key: newApiKey.value })
    Message.success('Key 更新成功')
    done()
    newApiKey.value = ''
    fetchDetail()
  } catch { return false } finally { updateKeyLoading.value = false }
}

// === Clone Channel ===
const showCloneModal = ref(false)
const cloneLoading = ref(false)
const cloneForm = reactive({ name: '', api_key: '' })

function openCloneModal() {
  cloneForm.name = ''
  cloneForm.api_key = ''
  showCloneModal.value = true
}

async function handleClone(done: () => void) {
  if (!cloneForm.api_key) { Message.warning('请输入 API Key'); return }
  cloneLoading.value = true
  try {
    const res: any = await request.post(`/admin/channels/${channelId}/clone`, cloneForm)
    const newId = res.data?.data?.id || res.data?.id
    Message.success('克隆成功')
    done()
    router.push({ name: 'AdminChannelDetail', params: { id: newId } })
  } catch { return false } finally { cloneLoading.value = false }
}

onMounted(() => {
  fetchDetail()
})

function formatJson(str: string): string {
  if (!str) return ''
  try { return JSON.stringify(JSON.parse(str), null, 2) } catch { return str }
}

function formatHeaders(headers: Record<string, string>): string {
  if (!headers) return ''
  return Object.entries(headers).map(([k, v]) => `${k}: ${v}`).join('\n')
}
</script>

<template>
  <div class="channel-detail-page">
    <PageHeader :title="detail ? detail.name : '渠道详情'" :description="detail ? `${detail.type_name} · ID: ${detail.id}` : ''">
      <template #actions>
        <ASpace>
          <AButton @click="goBack">返回列表</AButton>
          <AButton type="outline" @click="openCloneModal">克隆渠道</AButton>
          <template v-if="detail">
            <Popconfirm v-if="detail.status === 'active'" content="确定禁用该渠道？禁用后该渠道将不再处理请求" @ok="updateStatus('disabled')">
              <AButton status="warning">禁用</AButton>
            </Popconfirm>
            <Popconfirm v-if="detail.status === 'disabled'" content="确定启用该渠道？" @ok="updateStatus('active')">
              <AButton status="success">启用</AButton>
            </Popconfirm>
            <Popconfirm content="确定删除该渠道？此操作不可逆，关联的 Key 和模型能力也会被删除" @ok="deleteChannel">
              <AButton status="danger">删除渠道</AButton>
            </Popconfirm>
          </template>
        </ASpace>
      </template>
    </PageHeader>

    <ASpin :loading="loading" class="w-full">
      <template v-if="detail">
        <ATabs v-model:active-key="activeTab" @change="onTabChange">
          <!-- Tab 1: Basic Info -->
          <ATabPane key="info" title="基本信息">
            <ACard :bordered="false" class="mb-4" title="渠道信息">
              <template #extra>
                <AButton type="outline" size="small" @click="openEditModal">编辑</AButton>
              </template>
              <ADescriptions :column="2" bordered size="medium">
                <ADescriptionsItem label="ID">{{ detail.id }}</ADescriptionsItem>
                <ADescriptionsItem label="渠道名称">{{ detail.name }}</ADescriptionsItem>
                <ADescriptionsItem label="供应商类型">
                  <ATag size="small">{{ detail.type_name }}</ATag>
                </ADescriptionsItem>
                <ADescriptionsItem label="状态">
                  <ATag :color="statusTagColor[detail.status]" size="small">{{ statusLabel[detail.status] || detail.status }}</ATag>
                </ADescriptionsItem>
                <ADescriptionsItem label="Base URL" :span="2">
                  <code style="font-size: 13px; word-break: break-all;">{{ detail.base_url || '-' }}</code>
                </ADescriptionsItem>
                <ADescriptionsItem label="优先级">{{ detail.priority }}</ADescriptionsItem>
                <ADescriptionsItem label="权重">{{ detail.weight }}</ADescriptionsItem>
                <ADescriptionsItem label="健康度">
                  <span v-if="detail.health_score != null" :style="{ color: healthColor(detail.health_score), fontWeight: 600 }">
                    {{ detail.health_score.toFixed(0) }}
                  </span>
                  <span v-else style="color: #94a3b8">N/A</span>
                </ADescriptionsItem>
                <ADescriptionsItem label="VIP">
                  <ATag v-if="detail.is_vip" color="gold" size="small">VIP</ATag>
                  <span v-else>-</span>
                </ADescriptionsItem>
                <ADescriptionsItem label="代理">
                  <ATag v-if="detail.use_proxy" color="arcoblue" size="small">已启用</ATag>
                  <span v-else>-</span>
                </ADescriptionsItem>
                <ADescriptionsItem label="创建时间">{{ detail.created_at }}</ADescriptionsItem>
                <ADescriptionsItem label="更新时间">{{ detail.updated_at }}</ADescriptionsItem>
                <ADescriptionsItem v-if="detail.remark" label="备注" :span="2">{{ detail.remark }}</ADescriptionsItem>
              </ADescriptions>
            </ACard>

            <ACard :bordered="false" class="mb-4" title="API Key">
              <ADescriptions :column="2" bordered size="medium">
                <ADescriptionsItem label="Key 名称">{{ detail.key_name || 'default' }}</ADescriptionsItem>
                <ADescriptionsItem label="Key 类型">
                  <ATag size="small">{{ detail.key_type === 'oauth' ? 'OAuth' : 'API Key' }}</ATag>
                </ADescriptionsItem>
                <ADescriptionsItem label="Key 状态">
                  <ATag :color="detail.key_status === 'active' ? 'green' : 'orangered'" size="small">
                    {{ detail.key_status === 'active' ? '正常' : detail.key_status || 'N/A' }}
                  </ATag>
                </ADescriptionsItem>
                <ADescriptionsItem v-if="detail.token_expires_at" label="Token 过期时间">
                  {{ detail.token_expires_at }}
                </ADescriptionsItem>
              </ADescriptions>
              <div style="margin-top: 12px;">
                <AButton type="outline" size="small" @click="showUpdateKeyModal = true">更新 Key</AButton>
              </div>
            </ACard>

          </ATabPane>

          <!-- Tab 2: Model Abilities -->
          <ATabPane key="abilities" title="模型能力">
            <ACard :bordered="false">
              <div class="flex items-center justify-between mb-4">
                <span style="color: var(--color-text-3)">已配置 {{ abilitiesData.length }} 个模型能力</span>
                <AButton type="primary" @click="openAddAbilityModal">添加能力</AButton>
              </div>
              <ATable
                :columns="abilityColumns"
                :data="abilitiesData"
                :loading="abilitiesLoading"
                :bordered="false"
                :stripe="true"
                :pagination="false"
                row-key="id"
              />
            </ACard>
          </ATabPane>

          <!-- Tab 3: Test -->
          <ATabPane key="test" title="测试">
            <ACard :bordered="false">
              <div class="flex gap-2 items-end mb-4">
                <AFormItem label="测试模型" class="flex-1 !mb-0">
                  <ASelect
                    v-model="testModelName"
                    :options="allModelOptions"
                    :loading="modelsLoading"
                    allow-search
                    allow-clear
                    :filter-option="false"
                    placeholder="输入关键词搜索模型"
                    :disabled="testLoading"
                    @search="handleModelSearch"
                    @keydown.enter="handleTest"
                  />
                </AFormItem>
                <AButton type="primary" :loading="testLoading" :disabled="!testModelName" @click="handleTest">
                  {{ testLoading ? '测试中...' : '开始测试' }}
                </AButton>
              </div>

              <!-- Loading -->
              <div v-if="testLoading" class="test-result-area">
                <ASpin dot />
              </div>

              <!-- Result -->
              <div v-else-if="testResult" class="test-result-area">
                <ACard :bordered="true" size="small" class="w-full">
                  <template #title>
                    <span :style="{ color: testResult.success ? '#00b42a' : '#f53f3f', fontWeight: 600 }">
                      {{ testResult.success ? '测试通过' : '测试失败' }}
                    </span>
                  </template>
                  <template #extra>
                    <ASpace>
                      <ATag size="small" color="arcoblue">{{ testResult.model_name }}</ATag>
                      <ATag v-if="testResult.latency_ms" size="small">{{ testResult.latency_ms }}ms</ATag>
                    </ASpace>
                  </template>

                  <!-- 成功时显示 AI 回复 -->
                  <div v-if="testResult.success && testResult.content" class="test-reply">{{ testResult.content }}</div>
                  <div v-else-if="testResult.success && !testResult.content" class="test-reply" style="color: var(--color-text-3);">（AI 无文本回复内容）</div>

                  <!-- 失败时显示错误 -->
                  <div v-if="!testResult.success && (testResult.error || testResult.message)" class="test-error">{{ testResult.error || testResult.message }}</div>

                  <!-- Token 用量 -->
                  <div v-if="testResult.usage" class="test-usage">
                    <ATag size="small">Prompt: {{ testResult.usage.prompt_tokens }}</ATag>
                    <ATag size="small">Completion: {{ testResult.usage.completion_tokens }}</ATag>
                    <ATag size="small">Total: {{ testResult.usage.total_tokens }}</ATag>
                  </div>

                  <!-- 请求详情 -->
                  <ACollapse :bordered="false" :default-active-key="testResult.success ? [] : ['request']">
                    <ACollapseItem header="请求详情" key="request" :header-style="{ fontSize: '12px', color: 'var(--color-text-3)' }">
                      <div v-if="testResult.request" class="test-debug-info">
                        <div class="test-debug-row">
                          <span class="test-debug-label">Method</span>
                          <ATag size="small" color="gray">{{ testResult.request.method }}</ATag>
                        </div>
                        <div class="test-debug-row">
                          <span class="test-debug-label">URL</span>
                          <code class="test-debug-value">{{ testResult.request.url }}</code>
                        </div>
                        <div class="test-debug-row">
                          <span class="test-debug-label">Headers</span>
                          <pre class="test-debug-code">{{ formatHeaders(testResult.request.headers) }}</pre>
                        </div>
                        <div class="test-debug-row">
                          <span class="test-debug-label">Body</span>
                          <pre class="test-debug-code">{{ formatJson(testResult.request.body) }}</pre>
                        </div>
                      </div>
                      <div v-else style="color: var(--color-text-3); font-size: 12px;">无请求信息</div>
                    </ACollapseItem>
                    <ACollapseItem v-if="testResult.response" header="响应内容" key="response" :header-style="{ fontSize: '12px', color: 'var(--color-text-3)' }">
                      <pre class="test-raw">{{ formatJson(testResult.response) }}</pre>
                    </ACollapseItem>
                  </ACollapse>
                </ACard>
              </div>
            </ACard>
          </ATabPane>

          <!-- Tab 4: Health Trend -->
          <ATabPane key="health_trend" title="健康趋势">
            <ACard :bordered="false" style="padding-bottom: 16px;">
              <div style="display: flex; justify-content: flex-end; margin-bottom: 12px;">
                <RadioGroup type="button" :model-value="trendHours" @change="handleTrendHoursChange">
                  <Radio :value="1">1h</Radio>
                  <Radio :value="6">6h</Radio>
                  <Radio :value="24">24h</Radio>
                  <Radio :value="72">72h</Radio>
                </RadioGroup>
              </div>
              <ASpin :loading="trendLoading" style="width: 100%;">
                <div id="health-trend-chart" style="width: 100%; height: 400px;"></div>
              </ASpin>
            </ACard>
          </ATabPane>
        </ATabs>
      </template>
    </ASpin>

    <!-- Edit Channel Modal -->
    <AModal v-model:visible="showEditModal" title="编辑渠道" :width="600" :mask-closable="false" :on-before-ok="handleEditSubmit" :ok-loading="editLoading">
      <AForm :model="editForm" :auto-label-width="true" layout="vertical">
        <ARow :gutter="16">
          <ACol :span="8">
            <AFormItem label="渠道名称"><AInput v-model="editForm.name" /></AFormItem>
          </ACol>
          <ACol :span="16">
            <AFormItem label="Base URL"><AInput v-model="editForm.base_url" /></AFormItem>
          </ACol>
        </ARow>
        <ARow :gutter="16">
          <ACol :span="6">
            <AFormItem label="优先级"><AInputNumber v-model="editForm.priority" :min="0" class="w-full" /></AFormItem>
          </ACol>
          <ACol :span="6">
            <AFormItem label="权重"><AInputNumber v-model="editForm.weight" :min="1" :max="100" class="w-full" /></AFormItem>
          </ACol>
          <ACol :span="6">
            <AFormItem label="测试模型"><AInput v-model="editForm.test_model" /></AFormItem>
          </ACol>
          <ACol :span="6">
            <AFormItem label="状态">
              <ASelect v-model="editForm.status" :options="[{ label: '启用', value: 'active' }, { label: '禁用', value: 'disabled' }, { label: '测试中', value: 'testing' }]" />
            </AFormItem>
          </ACol>
          <ACol :span="6">
            <AFormItem label="使用代理"><ASwitch v-model="editForm.use_proxy" /></AFormItem>
          </ACol>
        </ARow>
        <AFormItem label="备注"><AInput v-model="editForm.remark" type="textarea" :auto-size="{ minRows: 2, maxRows: 4 }" /></AFormItem>
        <ACollapse :bordered="false">
          <ACollapseItem header="VIP 设置" key="vip" :header-style="{ fontSize: '13px' }">
            <ARow :gutter="16">
              <ACol :span="6">
                <AFormItem label="VIP 渠道"><ASwitch v-model="editForm.is_vip" /></AFormItem>
              </ACol>
              <ACol v-if="editForm.is_vip" :span="6">
                <AFormItem label="共享阈值"><AInputNumber v-model="editForm.sharing_threshold" :min="0" :max="100" class="w-full" /></AFormItem>
              </ACol>
              <ACol v-if="editForm.is_vip" :span="6">
                <AFormItem label="抢占阈值"><AInputNumber v-model="editForm.preemption_threshold" :min="0" :max="100" class="w-full" /></AFormItem>
              </ACol>
              <ACol v-if="editForm.is_vip" :span="6">
                <AFormItem label="借用冷却(秒)"><AInputNumber v-model="editForm.borrowing_cooldown_seconds" :min="0" class="w-full" /></AFormItem>
              </ACol>
            </ARow>
          </ACollapseItem>
        </ACollapse>
      </AForm>
    </AModal>

    <!-- Update Key Modal -->
    <AModal v-model:visible="showUpdateKeyModal" title="更新 API Key" :width="450" :on-before-ok="handleUpdateKey" :ok-loading="updateKeyLoading">
      <AForm layout="vertical">
        <AFormItem label="新 API Key" required>
          <AInput v-model="newApiKey" type="textarea" :auto-size="{ minRows: 3, maxRows: 5 }" placeholder="输入新的 API Key" />
        </AFormItem>
      </AForm>
    </AModal>

    <!-- Clone Channel Modal -->
    <AModal v-model:visible="showCloneModal" title="克隆渠道" :width="500" :on-before-ok="handleClone" :ok-loading="cloneLoading">
      <AForm layout="vertical">
        <AFormItem label="新渠道名称">
          <AInput v-model="cloneForm.name" :placeholder="(detail?.name || '') + ' (副本)'" />
        </AFormItem>
        <AFormItem label="API Key" required>
          <AInput v-model="cloneForm.api_key" type="textarea" :auto-size="{ minRows: 3, maxRows: 5 }" placeholder="输入新渠道的 API Key" />
        </AFormItem>
      </AForm>
    </AModal>

    <!-- Add Ability Modal -->
    <AModal v-model:visible="showAddAbilityModal" title="添加模型能力" :width="450" :on-before-ok="handleAddAbility">
      <AForm :model="addAbilityForm" :auto-label-width="true" layout="vertical">
        <AFormItem label="平台模型名">
          <ASelect
            v-model="addAbilityForm.model_name"
            :options="availableModelOptions"
            :loading="modelsLoading"
            allow-search
            allow-clear
            :filter-option="false"
            placeholder="输入关键词搜索模型"
            :fallback-option="false"
            @search="handleModelSearch"
          />
        </AFormItem>
        <AFormItem label="上游模型名"><AInput v-model="addAbilityForm.upstream_model" placeholder="留空则与平台模型名相同" /></AFormItem>
        <AFormItem label="启用">
          <ASelect v-model="addAbilityForm.enabled" :options="[{ label: '启用', value: true }, { label: '禁用', value: false }]" style="width: 120px" />
        </AFormItem>
      </AForm>
    </AModal>
  </div>
</template>

<style scoped>
.test-result-area { min-height: 60px; display: flex; align-items: flex-start; }
.test-reply { white-space: pre-wrap; word-break: break-word; font-size: 14px; line-height: 1.6; color: var(--color-text-1); max-height: 300px; overflow-y: auto; }
.test-usage { margin-top: 12px; display: flex; gap: 6px; flex-wrap: wrap; }
.test-raw { font-size: 12px; background: var(--color-fill-1); padding: 8px; border-radius: 4px; max-height: 200px; overflow: auto; color: var(--color-text-3); white-space: pre-wrap; word-break: break-all; margin: 0; }
.test-error { color: #f53f3f; font-size: 14px; line-height: 1.6; word-break: break-word; }
.test-debug-info { display: flex; flex-direction: column; gap: 8px; }
.test-debug-row { display: flex; gap: 12px; align-items: flex-start; }
.test-debug-label { font-size: 12px; color: var(--color-text-3); min-width: 60px; flex-shrink: 0; padding-top: 2px; }
.test-debug-value { font-size: 12px; color: var(--color-text-2); word-break: break-all; background: var(--color-fill-1); padding: 2px 6px; border-radius: 3px; }
.test-debug-code { font-size: 12px; background: var(--color-fill-1); padding: 6px 8px; border-radius: 4px; max-height: 150px; overflow: auto; color: var(--color-text-3); white-space: pre-wrap; word-break: break-all; margin: 0; flex: 1; }
</style>
