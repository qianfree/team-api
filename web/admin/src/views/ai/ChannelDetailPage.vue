<script setup lang="ts">
import { ref, reactive, computed, onMounted, h, nextTick, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Tag, Button, Popconfirm, Message, RadioGroup, Radio, InputNumber, Input, Switch, Alert, DatePicker } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { providerTypeOptions, filterProviderOption } from '@/constants/channel'
import * as echarts from 'echarts'

const route = useRoute()
const router = useRouter()

const channelId = route.params.id as string

const loading = ref(false)
const detail = ref<any>(null)
const activeTab = ref('info')

const statusTagColor: Record<string, string> = { active: 'green', disabled: 'orangered', testing: 'arcoblue' }
const statusLabel: Record<string, string> = { active: '启用', disabled: '禁用', testing: '测试中' }
const breakerTagColor: Record<number, string> = { 0: 'green', 1: 'red', 2: 'orange' }
const breakerLabel: Record<number, string> = { 0: '正常', 1: '熔断', 2: '半开' }
const tierLabel: Record<string, string> = { primary: '首选', secondary: '备用', reserve: '保底' }
const tierTagColor: Record<string, string> = { primary: 'arcoblue', secondary: 'orange', reserve: 'gray' }
const tierOptions = [
  { label: '首选（承接主要流量）', value: 'primary' },
  { label: '备用（零星保温流量，主力饱和时溢出承接）', value: 'secondary' },
  { label: '保底（仅前两层不可用时使用）', value: 'reserve' },
]
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
  type: 0,
  name: '',
  base_url: '',
  priority: 0,
  weight: 100,
  max_concurrency: 100,
  tier: 'primary',
  strict_capacity: false,
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
    type: detail.value.type ?? 0,
    name: detail.value.name || '',
    base_url: detail.value.base_url || '',
    priority: detail.value.priority || 0,
    weight: detail.value.weight,
    max_concurrency: detail.value.max_concurrency ?? 100,
    tier: detail.value.tier || 'primary',
    strict_capacity: detail.value.strict_capacity || false,
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
  { title: '平台模型名', dataIndex: 'model_name', width: 180 },

  // 配置操作列
  {
    title: '上游模型名', dataIndex: 'upstream_model', width: 200,
    render({ record }) {
      return h(Input, {
        modelValue: record.upstream_model || '',
        size: 'mini',
        placeholder: '留空则同名',
        'onUpdate:modelValue': (v: string) => handleUpstreamModelChange(record, v),
        onChange: () => scheduleAbilitySave('上游模型名已更新'),
      })
    },
  },
  {
    title: '成本比例', dataIndex: 'cost_ratio', width: 130,
    render({ record }) {
      return h(InputNumber, {
        modelValue: record.cost_ratio ?? 1,
        min: 0.0001,
        max: 100,
        step: 0.05,
        size: 'mini',
        onChange: (v: number) => handleCostRatioChange(record, v),
      })
    },
  },
  {
    title: 'Responses 协议', dataIndex: 'supports_responses', width: 110,
    render({ record }) {
      return h(Switch, {
        modelValue: !!record.supports_responses,
        size: 'mini',
        onChange: (v: boolean) => handleToggleProtoFlag(record, 'supports_responses', v),
      })
    },
  },
  {
    title: 'Chat 经 Responses', dataIndex: 'chat_via_responses', width: 120,
    render({ record }) {
      return h(Switch, {
        modelValue: !!record.chat_via_responses,
        size: 'mini',
        onChange: (v: boolean) => handleToggleProtoFlag(record, 'chat_via_responses', v),
      })
    },
  },
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

  // 健康监控列（放在右侧，操作按钮前面）
  {
    title: '健康状态', dataIndex: 'health_score', width: 120,
    render({ record }) {
      return renderHealthBadge(record)
    },
  },
  {
    title: '熔断', dataIndex: 'breaker_state', width: 80,
    render({ record }) {
      if (record.breaker_state === null || record.breaker_state === undefined) {
        return h('span', { style: 'color: var(--color-text-4); font-size: 12px' }, '—')
      }
      const color = breakerTagColor[record.breaker_state]
      const label = breakerLabel[record.breaker_state]
      return h(Tag, { color, size: 'small' }, () => label)
    },
  },

  // 操作列（固定在最右侧）
  {
    title: '操作', dataIndex: 'actions', width: 150, fixed: 'right',
    render({ record }) {
      return h('div', { style: 'display: flex; gap: 8px' }, [
        h(Button, {
          size: 'mini',
          onClick: () => handleResetModelHealth(record),
          disabled: !record.health_score && record.breaker_state === null,
        }, () => '重置健康'),
        h(Popconfirm, { content: '确定移除该能力？', onOk: () => handleDeleteAbility(record.id) }, () =>
          h(Button, { size: 'mini', status: 'danger' }, () => '移除')
        ),
      ])
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

// abilityPayload 组装能力批量提交体（cost_ratio / 协议开关必须随行提交，否则会被重置为默认值）
function abilityPayload(list: any[]) {
  return {
    channel_id: Number(channelId),
    abilities: list.map(a => ({
      model_name: a.model_name,
      upstream_model: a.upstream_model || '',
      enabled: a.enabled,
      cost_ratio: a.cost_ratio ?? 1,
      supports_responses: !!a.supports_responses,
      chat_via_responses: !!a.chat_via_responses,
    })),
  }
}

async function handleToggleAbility(ab: any) {
  const newList = abilitiesData.value.map(a => a.id === ab.id ? { ...a, enabled: !a.enabled } : a)
  try {
    await request.put(`/admin/channels/${channelId}/abilities`, abilityPayload(newList))
    abilitiesData.value = newList
    Message.success('状态已更新')
  } catch { /* error handled by interceptor */ }
}

// 能力表内联编辑：成本比例输入过程中 600ms 防抖整表提交；
// 上游模型名仅失焦/回车时提交（避免逐字保存）。提交体 abilityPayload 已包含 upstream_model。
let abilitySaveTimer: any = null
function scheduleAbilitySave(successMsg: string) {
  clearTimeout(abilitySaveTimer)
  abilitySaveTimer = setTimeout(async () => {
    try {
      await request.put(`/admin/channels/${channelId}/abilities`, abilityPayload(abilitiesData.value))
      Message.success(successMsg)
    } catch { /* error handled by interceptor */ }
  }, 600)
}

function handleCostRatioChange(record: any, value: number) {
  if (!value || value <= 0) return
  record.cost_ratio = value
  scheduleAbilitySave('成本比例已更新')
}

function handleUpstreamModelChange(record: any, value: string) {
  record.upstream_model = value
}

// 协议能力开关（supports_responses / chat_via_responses）：即时整表提交
async function handleToggleProtoFlag(ab: any, flag: 'supports_responses' | 'chat_via_responses', value: boolean) {
  const newList = abilitiesData.value.map(a => a.id === ab.id ? { ...a, [flag]: value } : a)
  try {
    await request.put(`/admin/channels/${channelId}/abilities`, abilityPayload(newList))
    abilitiesData.value = newList
    Message.success(flag === 'supports_responses' ? 'Responses 协议支持已更新' : 'Chat 经 Responses 已更新')
  } catch { /* error handled by interceptor */ }
}

async function handleDeleteAbility(id: number) {
  const newList = abilitiesData.value.filter(a => a.id !== id)
  try {
    await request.put(`/admin/channels/${channelId}/abilities`, abilityPayload(newList))
    Message.success('能力已移除')
    fetchAbilities()
  } catch { /* error handled by interceptor */ }
}

// 新增：健康徽章渲染函数
function renderHealthBadge(record: any) {
  if (record.health_score === null || record.health_score === undefined) {
    return h('div', { style: 'display: flex; align-items: center; gap: 6px' }, [
      h('div', {
        style: 'width: 8px; height: 8px; border-radius: 50%; background: #94a3b8'
      }),
      h('span', { style: 'color: var(--color-text-3); font-size: 13px' }, '无数据'),
    ])
  }

  const score = Math.round(record.health_score)
  const color = healthColor(score)
  const statusText = score >= 80 ? '健康' : score >= 50 ? '降级' : '故障'

  return h('div', { style: 'display: flex; align-items: center; gap: 6px' }, [
    h('div', {
      style: `width: 8px; height: 8px; border-radius: 50%; background: ${color}`
    }),
    h('span', { style: `color: ${color}; font-weight: 500; font-size: 13px` }, statusText),
    h('span', { style: 'color: var(--color-text-3); font-size: 12px; margin-left: 2px' }, `(${score})`),
  ])
}

// 新增：重置模型级健康
async function handleResetModelHealth(record: any) {
  try {
    await request.post(`/admin/channels/${channelId}/reset_health`, {
      model_name: record.model_name,
    })
    Message.success(`模型 ${record.model_name} 健康已重置`)
    fetchAbilities()
  } catch {
    // error handled by interceptor
  }
}

// Add Ability Modal
const showAddAbilityModal = ref(false)
const addAbilityForm = reactive({ model_name: '', upstream_model: '', enabled: true })
const modelsList = ref<any[]>([])
const modelsLoading = ref(false)

// 拉取全部 active 模型（下拉专用、不分页接口 /admin/models/options）。
// 此前用分页接口 /admin/models 且 page_size=20，导致平台模型超过 20 个时
// “添加能力”下拉只显示最新 20 条，看不到其余模型。
async function fetchModels() {
  modelsLoading.value = true
  try {
    const res: any = await request.get('/admin/models/options')
    modelsList.value = res.data?.data?.list || res.data?.list || []
  } catch { modelsList.value = [] } finally { modelsLoading.value = false }
}

const availableModelOptions = computed(() => {
  const usedNames = new Set(abilitiesData.value.map(a => a.model_name))
  return modelsList.value
    .filter(m => !usedNames.has(m.model_id))
    .map(m => ({ label: m.model_name ? `${m.model_name} (${m.model_id})` : m.model_id, value: m.model_id }))
})

const allModelOptions = computed(() => {
  // 测试模型下拉列表只显示该渠道已配置的能力模型
  return abilitiesData.value
    .filter(a => a.enabled)
    .map(a => ({ label: a.model_name, value: a.model_name }))
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
  const newList = [...abilitiesData.value, { model_name: addAbilityForm.model_name, upstream_model: addAbilityForm.upstream_model, enabled: addAbilityForm.enabled, cost_ratio: 1 }]
  try {
    await request.put(`/admin/channels/${channelId}/abilities`, abilityPayload(newList))
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
let resizeTimer: ReturnType<typeof setTimeout> | null = null

function onWindowResize() {
  if (resizeTimer) clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    trendChart?.resize()
  }, 200)
}

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
  const scores = trendData.value.map((p: any) => Math.round(Number(p.health_score) || 0))
  const latencies = trendData.value.map((p: any) => Math.round(Number(p.latency_ms) || 0))
  trendChart.setOption({
    // 健康度、延迟统一取整展示
    tooltip: { trigger: 'axis', valueFormatter: (value: any) => String(Math.round(Number(value) || 0)) },
    legend: { data: ['健康度', '延迟(ms)'], bottom: 0 },
    grid: { left: 50, right: 50, top: 30, bottom: 45 },
    xAxis: { type: 'category', data: times, axisLabel: { fontSize: 11 } },
    yAxis: [
      { type: 'value', name: '健康度', min: 0, max: 100, splitLine: { show: false } },
      { type: 'value', name: '延迟(ms)', min: 0, splitLine: { show: false } },
    ],
    series: [
      {
        name: '健康度', type: 'line', data: scores, smooth: true, showSymbol: false, lineStyle: { width: 2 },
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

// === Tab 5: Debug Logs（渠道调试日志） ===
const debugLogs = ref<any[]>([])
const debugLoading = ref(false)
const debugPage = ref(1)
const debugPageSize = 20
const debugTotal = ref(0)
const debugFilter = reactive({
  request_id: '',
  model_name: '',
  only_error: false,
  dateRange: undefined as string[] | undefined,
})
const debugStats = ref<{ total: number; total_bytes: number; oldest_at: string } | null>(null)
const debugSwitchLoading = ref(false)

// 详情抽屉
const showDebugDetail = ref(false)
const debugDetailLoading = ref(false)
const debugDetail = ref<any>(null)

function formatBytes(n: number | null | undefined): string {
  if (n === null || n === undefined) return '-'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(2)} MB`
}

async function fetchDebugStats() {
  try {
    const res: any = await request.get(`/admin/channels/${channelId}/debug-logs/stats`)
    debugStats.value = res.data?.data || null
  } catch {
    debugStats.value = null
  }
}

async function fetchDebugLogs() {
  debugLoading.value = true
  try {
    const params: Record<string, any> = { page: debugPage.value, page_size: debugPageSize }
    if (debugFilter.request_id) params.request_id = debugFilter.request_id
    if (debugFilter.model_name) params.model_name = debugFilter.model_name
    if (debugFilter.only_error) params.only_error = true
    if (debugFilter.dateRange && debugFilter.dateRange.length === 2) {
      params.start_date = debugFilter.dateRange[0]
      params.end_date = debugFilter.dateRange[1]
    }
    const res: any = await request.get(`/admin/channels/${channelId}/debug-logs`, { params })
    const raw = res.data?.data
    debugLogs.value = Array.isArray(raw?.list) ? raw.list : []
    debugTotal.value = raw?.total || 0
  } catch {
    debugLogs.value = []
    debugTotal.value = 0
  } finally {
    debugLoading.value = false
  }
  fetchDebugStats()
}

function handleDebugFilter() {
  debugPage.value = 1
  fetchDebugLogs()
}

function resetDebugFilter() {
  debugFilter.request_id = ''
  debugFilter.model_name = ''
  debugFilter.only_error = false
  debugFilter.dateRange = undefined
  debugPage.value = 1
  fetchDebugLogs()
}

function handleDebugPageChange(page: number) {
  debugPage.value = page
  fetchDebugLogs()
}

// 调试开关：乐观更新 + 失败回滚
async function toggleDebugLog(value: string | number | boolean) {
  const enabled = Boolean(value)
  if (!detail.value) return
  debugSwitchLoading.value = true
  try {
    await request.put(`/admin/channels/${channelId}`, { debug_log_enabled: enabled })
    detail.value.debug_log_enabled = enabled
    Message.success(enabled ? '调试日志已开启，新的请求将记录四段完整报文' : '调试日志已关闭')
    fetchDetail()
  } catch {
    // 失败保持原状态，interceptor 已提示
  } finally {
    debugSwitchLoading.value = false
  }
}

async function openDebugDetail(row: any) {
  showDebugDetail.value = true
  debugDetailLoading.value = true
  debugDetail.value = null
  try {
    const res: any = await request.get(`/admin/channels/${channelId}/debug-logs/${row.id}`)
    debugDetail.value = res.data?.data?.data || null
  } catch {
    debugDetail.value = null
  } finally {
    debugDetailLoading.value = false
  }
}

async function deleteDebugLog(row: any) {
  try {
    await request.delete(`/admin/channels/${channelId}/debug-logs/${row.id}`)
    Message.success('已删除')
    fetchDebugLogs()
  } catch {
    // error handled by interceptor
  }
}

async function clearDebugLogs() {
  try {
    await request.delete(`/admin/channels/${channelId}/debug-logs`)
    Message.success('调试日志已清空')
    fetchDebugLogs()
  } catch {
    // error handled by interceptor
  }
}

// 详情四段定义（标题/字段前缀），供抽屉 collapse 渲染
const debugSegments = computed(() => {
  const d = debugDetail.value
  if (!d) return []
  const seg = (prefix: string, title: string) => {
    const headers = d[`${prefix}_headers`]
    const ct = headers?.['Content-Type'] || headers?.['content-type'] || ''
    return {
      key: prefix,
      title,
      headers,
      body: d[`${prefix}_body`] || '',
      encoding: d[`${prefix}_encoding`] || 'plain',
      bytes: d[`${prefix}_bytes`],
      contentType: ct,
      // base64 且内容类型为图片时可预览（JSON 内嵌 b64 图片不自动预览，可复制后查看）
      imageUri: d[`${prefix}_encoding`] === 'base64' && ct.startsWith('image/')
        ? `data:${ct};base64,${d[`${prefix}_body`]}` : '',
      bodyText: d[`${prefix}_encoding`] === 'base64' ? '' : prettyJson(d[`${prefix}_body`]),
    }
  }
  return [
    seg('client_req', '① 客户端 → 系统（请求）'),
    seg('upstream_req', '② 系统 → 上游（请求，协议转换后 · 凭证已脱敏）'),
    seg('upstream_resp', '③ 上游 → 系统（响应原文）'),
    seg('client_resp', '④ 系统 → 客户端（响应，协议转换后）'),
  ]
})

function prettyJson(body: string): string {
  if (!body) return ''
  try { return JSON.stringify(JSON.parse(body), null, 2) } catch { return body }
}

// === 调试目标过滤（租户 → 成员 → 密钥 联动，全部用 ID，不选 = 不限） ===
interface DebugSelectOption { label: string; value: number }
const debugTarget = reactive({
  tenantId: null as number | null,
  userId: null as number | null,
  apiKeyId: null as number | null,
})
const debugTenantOptions = ref<DebugSelectOption[]>([])
const debugUserOptions = ref<DebugSelectOption[]>([])
const debugKeyOptions = ref<DebugSelectOption[]>([])
const debugTargetSaving = ref(false)
const debugTargetLoaded = ref(false)

// mergeOptions 合并下拉选项并去重，保留已选中项避免回显丢失
function mergeOptions(target: { value: DebugSelectOption[] }, incoming: DebugSelectOption[]) {
  const map = new Map(target.value.map(o => [o.value, o]))
  for (const o of incoming) map.set(o.value, o)
  target.value = Array.from(map.values())
}

async function searchDebugTenants(keyword: string) {
  try {
    const res: any = await request.get('/admin/tenants/select', { params: { keyword: keyword || '', page_size: 50 } })
    const list = res.data?.data?.list || []
    mergeOptions(debugTenantOptions, list.map((t: any) => ({ label: `${t.name}（${t.code}）`, value: t.id })))
  } catch {
    // error handled by interceptor
  }
}

async function loadDebugUsers(tenantId: number) {
  try {
    const res: any = await request.get('/admin/members', { params: { tenant_id: tenantId, page_size: 200 } })
    const list = res.data?.data?.list || []
    debugUserOptions.value = list.map((m: any) => ({
      label: m.display_name && m.display_name !== m.username ? `${m.display_name}（${m.username}）` : m.username,
      value: m.id,
    }))
  } catch {
    debugUserOptions.value = []
  }
}

async function loadDebugKeys(tenantId: number, userId: number | null) {
  try {
    const params: Record<string, any> = { page_size: 200 }
    if (userId) params.user_id = userId
    const res: any = await request.get(`/admin/tenants/${tenantId}/api-keys/select`, { params })
    const list = res.data?.data?.list || []
    debugKeyOptions.value = list.map((k: any) => ({
      label: `${k.name || '未命名'}（${k.key_prefix}…${k.status && k.status !== 'active' ? ' · 已停用' : ''}）`,
      value: k.id,
    }))
  } catch {
    debugKeyOptions.value = []
  }
}

function handleDebugTenantChange(v: any) {
  debugTarget.tenantId = v ?? null
  // 级联清空下游选择与选项
  debugTarget.userId = null
  debugTarget.apiKeyId = null
  debugUserOptions.value = []
  debugKeyOptions.value = []
  if (debugTarget.tenantId) {
    loadDebugUsers(debugTarget.tenantId)
    loadDebugKeys(debugTarget.tenantId, null)
  }
}

function handleDebugUserChange(v: any) {
  debugTarget.userId = v ?? null
  debugTarget.apiKeyId = null
  if (debugTarget.tenantId) loadDebugKeys(debugTarget.tenantId, debugTarget.userId)
}

async function saveDebugTarget() {
  debugTargetSaving.value = true
  try {
    await request.put(`/admin/channels/${channelId}`, {
      debug_log_tenant_id: debugTarget.tenantId || 0,
      debug_log_user_id: debugTarget.userId || 0,
      debug_log_api_key_id: debugTarget.apiKeyId || 0,
    })
    Message.success('调试过滤条件已保存，新请求按过滤条件捕捉')
    fetchDetail()
  } catch {
    // error handled by interceptor
  } finally {
    debugTargetSaving.value = false
  }
}

// initDebugTarget 进入调试 tab 时回显已保存的过滤条件（解析名称，查不到显示 #ID）
async function initDebugTarget() {
  if (debugTargetLoaded.value || !detail.value) return
  debugTargetLoaded.value = true
  searchDebugTenants('')
  const t = detail.value.debug_log_tenant_id || 0
  const u = detail.value.debug_log_user_id || 0
  const k = detail.value.debug_log_api_key_id || 0
  if (!t && !u && !k) return
  debugTarget.tenantId = t || null
  debugTarget.userId = u || null
  debugTarget.apiKeyId = k || null
  if (!t) return
  // 回显租户名称
  try {
    const res: any = await request.get(`/admin/tenants/${t}`)
    const item = res.data?.data || res.data
    if (item?.name) debugTenantOptions.value = [{ label: `${item.name}（${item.code || ''}）`, value: t }]
  } catch {
    debugTenantOptions.value = [{ label: `租户 #${t}`, value: t }]
  }
  await Promise.all([loadDebugUsers(t), loadDebugKeys(t, u || null)])
  // 成员/密钥可能翻页外或已删除，兜底显示 #ID
  if (u && !debugUserOptions.value.some(o => o.value === u)) {
    debugUserOptions.value.unshift({ label: `成员 #${u}`, value: u })
  }
  if (k && !debugKeyOptions.value.some(o => o.value === k)) {
    debugKeyOptions.value.unshift({ label: `密钥 #${k}`, value: k })
  }
}

// 捕捉范围描述（警告条展示）
const debugTargetDesc = computed(() => {
  const label = (opts: DebugSelectOption[], v: number | null) =>
    opts.find(o => o.value === v)?.label || `#${v}`
  const parts: string[] = []
  if (debugTarget.tenantId) parts.push(`租户 ${label(debugTenantOptions.value, debugTarget.tenantId)}`)
  if (debugTarget.userId) parts.push(`成员 ${label(debugUserOptions.value, debugTarget.userId)}`)
  if (debugTarget.apiKeyId) parts.push(`密钥 ${label(debugKeyOptions.value, debugTarget.apiKeyId)}`)
  return parts.length ? parts.join(' / ') : '全部请求（未设过滤）'
})

// 复制到剪切板（含非安全上下文降级）
function copyText(text: string, label = '内容') {
  if (!text) { Message.warning('内容为空'); return }
  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(text).then(() => {
      Message.success(`${label}已复制`)
    }).catch(() => {
      Message.error('复制失败，请手动选择复制')
    })
    return
  }
  // 剪切板 API 不可用（如非 HTTPS 环境），降级 execCommand
  const ta = document.createElement('textarea')
  ta.value = text
  ta.style.position = 'fixed'
  ta.style.opacity = '0'
  document.body.appendChild(ta)
  ta.select()
  const ok = document.execCommand('copy')
  document.body.removeChild(ta)
  if (ok) Message.success(`${label}已复制`)
  else Message.error('复制失败，请手动选择复制')
}

// 段体复制：base64 段展示的只是提示文案，复制完整原始内容（base64 串）
function copySegBody(seg: { body: string; bodyText: string; encoding: string }) {
  copyText(seg.encoding === 'base64' ? seg.body : seg.bodyText, '内容')
}

// 协议转换信息解析（JSONB 列可能以字符串返回）
function parseConversion(v: any): any {
  if (!v) return null
  if (typeof v === 'string') {
    try { return JSON.parse(v) } catch { return null }
  }
  return v
}

const debugBridgeLabels: Record<string, string> = {
  responses_api: 'Responses 桥接',
  responses_direct: 'Responses 直连',
  pass_through: '直传无转换',
}

// 列表紧凑展示：openai→claude（同构时单值）
function conversionShort(v: any): string {
  const c = parseConversion(v)
  if (!c) return ''
  if (!c.client_format || !c.upstream_format) return ''
  return c.client_format === c.upstream_format ? c.client_format : `${c.client_format}→${c.upstream_format}`
}

// 详情元信息条目（右侧统一带复制按钮）
const debugMetaItems = computed(() => {
  const d = debugDetail.value
  if (!d) return []
  const items: { label: string; value: string; span?: number; color?: string; mono?: boolean }[] = [
    { label: 'Request ID', value: d.request_id || '' },
    { label: '时间', value: String(d.created_at || '') },
    { label: '模型', value: d.model_name || '' },
    { label: '上游模型', value: d.upstream_model || '' },
    { label: '转发模式', value: d.relay_mode || '' },
    { label: '入站路径', value: d.inbound_path || '' },
    { label: '尝试轮次', value: `#${d.retry_index}${d.is_final ? '（最终）' : '（中间失败）'}` },
    { label: '流式', value: d.is_stream ? '流式' : '非流式' },
    { label: '上游状态码', value: d.upstream_status_code ? String(d.upstream_status_code) : '', color: statusCodeColor(d.upstream_status_code) },
    { label: '客户端状态码', value: d.client_status_code ? String(d.client_status_code) : '', color: statusCodeColor(d.client_status_code) },
    { label: '上游耗时', value: d.upstream_latency_ms != null ? `${d.upstream_latency_ms}ms` : '' },
    {
      label: '总耗时 / 首字节',
      value: `${d.total_latency_ms != null ? `${d.total_latency_ms}ms` : '-'} / ${d.first_token_ms != null ? `${d.first_token_ms}ms` : '-'}`,
    },
    { label: '上游 URL', value: d.upstream_url || '', span: 2, mono: true },
  ]
  // 协议转换条目：openai → claude（桥接方式 + 完整链路）
  const conv = parseConversion(d.conversion)
  if (conv && (conv.client_format || conv.upstream_format)) {
    let convValue = `${conv.client_format || '-'} → ${conv.upstream_format || '-'}`
    if (conv.bridge) convValue += `（${debugBridgeLabels[conv.bridge] || conv.bridge}）`
    if (Array.isArray(conv.chain) && conv.chain.length > 2) convValue += `，链路 ${conv.chain.join(' → ')}`
    items.push({ label: '协议转换', value: convValue, span: 2 })
  }
  if (d.error) {
    items.push({ label: '错误', value: d.error, span: 2, color: '#f53f3f' })
  }
  return items
})

const debugColumns: TableColumnData[] = [
  { title: '时间', dataIndex: 'created_at', width: 165, render: ({ record }) => h('span', { style: 'font-size:12px' }, String(record.created_at || '').replace('T', ' ').slice(0, 19)) },
  { title: '模型', dataIndex: 'model_name', width: 160, ellipsis: true, tooltip: true },
  {
    title: '尝试', dataIndex: 'retry_index', width: 75,
    render: ({ record }) => h(Tag, {
      color: record.is_final ? 'green' : 'orange', size: 'small',
    }, { default: () => `#${record.retry_index}${record.is_final ? ' 终' : ''}` }),
  },
  {
    title: '流式', dataIndex: 'is_stream', width: 60,
    render: ({ record }) => h(Tag, { size: 'small' }, { default: () => record.is_stream ? '流' : '非流' }),
  },
  {
    title: '上游', dataIndex: 'upstream_status_code', width: 65,
    render: ({ record }) => h('span', { style: statusCodeColor(record.upstream_status_code) }, record.upstream_status_code || '—'),
  },
  {
    title: '客户端', dataIndex: 'client_status_code', width: 65,
    render: ({ record }) => h('span', { style: statusCodeColor(record.client_status_code) }, record.client_status_code || '—'),
  },
  { title: '上游耗时', dataIndex: 'upstream_latency_ms', width: 85, render: ({ record }) => record.upstream_latency_ms != null ? `${record.upstream_latency_ms}ms` : '-' },
  {
    title: '协议转换', width: 125,
    render: ({ record }) => {
      const txt = conversionShort(record.conversion)
      if (!txt) return '-'
      return h(Tag, { size: 'small', color: 'arcoblue' }, { default: () => txt })
    },
  },
  { title: '总耗时', dataIndex: 'total_latency_ms', width: 85, render: ({ record }) => record.total_latency_ms != null ? `${record.total_latency_ms}ms` : '-' },
  {
    title: '报文体积', width: 150,
    render: ({ record }) => h('span', { style: 'font-size:12px;color:var(--color-text-3)' },
      `①${formatBytes(record.client_req_body_size)} ②${formatBytes(record.upstream_req_body_size)} ③${formatBytes(record.upstream_resp_body_size)} ④${formatBytes(record.client_resp_body_size)}`),
  },
  {
    title: '错误', dataIndex: 'error', ellipsis: true, tooltip: true, width: 140,
    render: ({ record }) => record.error ? h('span', { style: 'color:#f53f3f;font-size:12px' }, record.error) : '-',
  },
  {
    title: '操作', width: 120, fixed: 'right',
    render: ({ record }) =>
      h('div', { style: 'display:flex;gap:6px' }, [
        h(Button, { size: 'mini', type: 'outline', onClick: () => openDebugDetail(record) }, { default: () => '详情' }),
        h(Button, { size: 'mini', status: 'danger', type: 'text', onClick: () => deleteDebugLog(record) }, { default: () => '删除' }),
      ]),
  },
]

function statusCodeColor(code: number | null | undefined): string {
  if (!code) return 'color:var(--color-text-3)'
  if (code >= 200 && code < 300) return 'color:#00b42a;font-weight:600'
  if (code >= 400 && code < 500) return 'color:#ff7d00;font-weight:600'
  return 'color:#f53f3f;font-weight:600'
}

onBeforeUnmount(() => {
  window.removeEventListener('resize', onWindowResize)
  if (resizeTimer) clearTimeout(resizeTimer)
  if (abilitySaveTimer) clearTimeout(abilitySaveTimer)
  if (trendChart) { trendChart.dispose(); trendChart = null }
})

function onTabChange(key: string) {
  if (key === 'abilities' && abilitiesData.value.length === 0) fetchAbilities()
  if (key === 'test' && abilitiesData.value.length === 0) fetchAbilities()
  if (key === 'health_trend' && trendData.value.length === 0) fetchHealthTrend()
  if (key === 'debug_logs') {
    if (debugLogs.value.length === 0) fetchDebugLogs()
    initDebugTarget()
  }
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
  window.addEventListener('resize', onWindowResize)
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
                <ADescriptionsItem label="最大并发">
                  <span v-if="detail.max_concurrency > 0">{{ detail.max_concurrency }}</span>
                  <span v-else style="color: #94a3b8">自动</span>
                </ADescriptionsItem>
                <ADescriptionsItem label="调度层级">
                  <ATag :color="tierTagColor[detail.tier] || 'gray'" size="small">{{ tierLabel[detail.tier] || detail.tier || '-' }}</ATag>
                </ADescriptionsItem>
                <ADescriptionsItem label="严格容量">
                  <ATag v-if="detail.strict_capacity" color="purple" size="small">fail-closed</ATag>
                  <span v-else>-</span>
                </ADescriptionsItem>
                <ADescriptionsItem label="健康度">
                  <span v-if="detail.health_score != null" :style="{ color: healthColor(detail.health_score), fontWeight: 600 }">
                    {{ detail.health_score.toFixed(0) }}
                  </span>
                  <span v-else style="color: #94a3b8">N/A</span>
                </ADescriptionsItem>
                <ADescriptionsItem label="调度状态">
                  <!-- disabled / testing 渠道不在目录快照中，熔断状态无意义，置灰展示 -->
                  <template v-if="detail.status === 'active'">
                    <ATag :color="breakerTagColor[detail.breaker_state ?? 0]" size="small">
                      {{ breakerLabel[detail.breaker_state ?? 0] }}
                    </ATag>
                    <span v-if="(detail.breaker_models ?? 0) > 0" style="margin-left: 6px; color: #f59e0b; font-size: 12px; font-weight: 600">
                      {{ detail.breaker_models }} 个模型受影响
                    </span>
                  </template>
                  <span v-else style="color: #94a3b8">—</span>
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
              <template #title>
                <div class="flex items-center justify-between">
                  <span>模型能力配置</span>
                  <AButton type="primary" @click="openAddAbilityModal">添加能力</AButton>
                </div>
              </template>
              <div class="flex items-center justify-between mb-4">
                <span style="color: var(--color-text-3)">已配置 {{ abilitiesData.length }} 个模型能力</span>
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
              <!-- 字段功能说明（面向运营者） -->
              <div class="mt-3 text-xs" style="color: var(--color-text-3); line-height: 1.7;">
                <div class="mb-1"><strong>字段说明</strong></div>
                <div class="flex flex-wrap gap-x-6 gap-y-1">
                  <div>
                    <span style="font-weight: 500;">上游模型名：</span>该渠道转发请求时实际调用的上游模型名称。仅当上游名称与平台标准模型名不同时填写（如平台名 gpt-4、上游名 gpt-4-0314）；留空表示与平台模型名相同。
                  </div>
                  <div>
                    <span style="font-weight: 500;">成本比例：</span>该渠道相对基准价的成本系数，1.0 为标准。小于 1（如 0.8）表示更便宜，多渠道择优时更优先调度；大于 1（如 1.5）表示更贵，优先级降低。仅用于渠道调度，不影响对用户的计费。
                  </div>
                  <div>
                    <span style="font-weight: 500;">Responses 协议：</span>该模型在上游支持 OpenAI Responses API（/v1/responses）。开启后 /v1/responses 端点请求直连上游原生协议转发（不经 chat 转换），且调度时优先选择此类渠道。
                  </div>
                  <div>
                    <span style="font-weight: 500;">Chat 经 Responses：</span>上游仅有 Responses 协议（responses-only，如 Codex 类中转）。开启后 /v1/chat/completions 请求自动经桥接转换发送到 /v1/responses。
                  </div>
                </div>
              </div>
            </ACard>
          </ATabPane>

          <!-- Tab 3: Test -->
          <ATabPane key="test" title="测试">
            <!-- 功能限制提示 -->
            <AAlert type="info" class="mb-4" show-icon closable>
              <template #title>仅支持文本对话模型测试</template>
              <div style="font-size: 12px; line-height: 1.6; color: var(--color-text-2);">
                <div>当前渠道测试功能仅支持<strong>文本对话模型</strong>（如 Chat 系列）的基础测试。</div>
                <div style="margin-top: 6px;">其他模型类型（如视频生成、图片生成、Embeddings 等）暂不支持此测试方式，建议：</div>
                <ul style="margin: 6px 0 0 20px; padding: 0;">
                  <li>使用<strong>租户端的在线体验功能</strong>进行端到端测试</li>
                  <li>通过 <strong>API 直接调用</strong>进行验证测试</li>
                </ul>
              </div>
            </AAlert>

            <ACard :bordered="false">
              <div class="flex gap-2 items-end mb-4">
                <AFormItem label="测试模型" class="flex-1 !mb-0">
                  <ASelect
                    v-model="testModelName"
                    :options="allModelOptions"
                    :loading="abilitiesLoading"
                    allow-search
                    allow-clear
                    placeholder="选择渠道支持的模型"
                    :disabled="testLoading"
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

          <!-- Tab 5: Debug Logs -->
          <ATabPane key="debug_logs" title="调试日志">
            <ACard :bordered="false" class="mb-4">
              <div class="flex items-center justify-between flex-wrap" style="gap: 12px;">
                <div class="flex items-center" style="gap: 10px;">
                  <span style="font-weight: 600;">调试开关</span>
                  <Switch
                    :model-value="!!detail.debug_log_enabled"
                    :loading="debugSwitchLoading"
                    @change="toggleDebugLog"
                  />
                  <span style="color: var(--color-text-3); font-size: 12px;">
                    开启后记录经该渠道每次请求尝试的四段完整报文（含脱敏凭证），排障完成请及时关闭并清理
                  </span>
                </div>
                <Popconfirm content="确定清空该渠道全部调试日志？此操作不可逆" @ok="clearDebugLogs">
                  <AButton status="danger" type="outline" size="small" :disabled="!debugStats || debugStats.total === 0">清空日志</AButton>
                </Popconfirm>
              </div>
              <div class="flex items-center flex-wrap mt-3" style="gap: 8px;">
                <span style="font-size: 12px; color: var(--color-text-3); flex-shrink: 0;">捕捉范围（不选 = 不限）：</span>
                <ASelect
                  :model-value="debugTarget.tenantId"
                  :options="debugTenantOptions"
                  placeholder="租户不限"
                  allow-search
                  allow-clear
                  size="small"
                  style="width: 220px"
                  @search="searchDebugTenants"
                  @change="handleDebugTenantChange"
                />
                <ASelect
                  :model-value="debugTarget.userId"
                  :options="debugUserOptions"
                  placeholder="成员不限"
                  allow-search
                  allow-clear
                  size="small"
                  style="width: 200px"
                  :disabled="!debugTarget.tenantId"
                  @change="handleDebugUserChange"
                />
                <ASelect
                  :model-value="debugTarget.apiKeyId"
                  :options="debugKeyOptions"
                  placeholder="密钥不限"
                  allow-search
                  allow-clear
                  size="small"
                  style="width: 240px"
                  :disabled="!debugTarget.tenantId"
                  @change="(v: any) => debugTarget.apiKeyId = v ?? null"
                />
                <AButton size="small" type="primary" :loading="debugTargetSaving" @click="saveDebugTarget">保存过滤条件</AButton>
              </div>
              <Alert
                v-if="detail.debug_log_enabled"
                type="warning"
                class="mt-3"
              >
                调试日志完整保留请求/响应体（可能数 MB/条），无自动过期。当前捕捉范围：{{ debugTargetDesc }}；累计
                {{ debugStats ? debugStats.total : 0 }} 条 / {{ formatBytes(debugStats?.total_bytes || 0) }}
                <template v-if="debugStats && debugStats.oldest_at">，最早 {{ debugStats.oldest_at }}</template>
                —— 请及时清理，避免存储膨胀。
              </Alert>
            </ACard>

            <ACard :bordered="false">
              <div class="flex items-center flex-wrap mb-4" style="gap: 8px;">
                <Input v-model="debugFilter.request_id" placeholder="Request ID" style="width: 200px" allow-clear @clear="handleDebugFilter" @press-enter="handleDebugFilter" />
                <Input v-model="debugFilter.model_name" placeholder="模型名" style="width: 160px" allow-clear @clear="handleDebugFilter" @press-enter="handleDebugFilter" />
                <DatePicker
                  :model-value="debugFilter.dateRange"
                  range
                  style="width: 240px"
                  format="YYYY-MM-DD"
                  @change="(val: any) => { debugFilter.dateRange = val }"
                />
                <Switch v-model="debugFilter.only_error" size="small">
                  <template #checked>仅错误</template>
                  <template #unchecked>仅错误</template>
                </Switch>
                <AButton type="primary" size="small" @click="handleDebugFilter">搜索</AButton>
                <AButton size="small" @click="resetDebugFilter">重置</AButton>
              </div>
              <ATable
                :columns="debugColumns"
                :data="debugLogs"
                :loading="debugLoading"
                :bordered="false"
                :scroll-x="1380"
                size="small"
                row-key="id"
                :pagination="{
                  total: debugTotal,
                  current: debugPage,
                  pageSize: debugPageSize,
                  showTotal: true,
                }"
                @page-change="handleDebugPageChange"
              />
            </ACard>
          </ATabPane>
        </ATabs>
      </template>
    </ASpin>

    <!-- Edit Channel Modal -->
    <AModal v-model:visible="showEditModal" title="编辑渠道" :width="680" :mask-closable="false" :on-before-ok="handleEditSubmit" :ok-loading="editLoading">
      <AForm :model="editForm" :auto-label-width="true" layout="vertical">
        <!-- 基础信息 -->
        <div class="form-group-title">基础信息</div>
        <ARow :gutter="16">
          <ACol :span="12">
            <AFormItem label="供应商类型" required>
              <ASelect
                v-model="editForm.type"
                :options="providerTypeOptions"
                placeholder="搜索或选择供应商"
                allow-search
                :filter-option="filterProviderOption"
              />
            </AFormItem>
          </ACol>
          <ACol :span="12">
            <AFormItem label="渠道名称"><AInput v-model="editForm.name" /></AFormItem>
          </ACol>
        </ARow>
        <AFormItem label="Base URL">
          <AInput v-model="editForm.base_url" placeholder="留空使用供应商默认地址" />
          <template #extra>
            <span class="field-help">ⓘ 修改供应商类型后，请同步检查 Base URL 与「模型能力」配置</span>
          </template>
        </AFormItem>

        <!-- 调度与容量 -->
        <div class="form-group-title">调度与容量</div>
        <ARow :gutter="16">
          <ACol :span="12">
            <AFormItem label="调度层级">
              <ASelect v-model="editForm.tier" :options="tierOptions" />
              <template #extra><span class="field-help">三档固定层级，替代旧版数值优先级</span></template>
            </AFormItem>
          </ACol>
          <ACol :span="12">
            <AFormItem label="权重">
              <AInputNumber v-model="editForm.weight" :min="0" :max="100" class="w-full" />
              <template #extra><span class="field-help">同层级内按权重比例分配</span></template>
            </AFormItem>
          </ACol>
        </ARow>
        <ARow :gutter="16">
          <ACol :span="12">
            <AFormItem label="最大并发">
              <AInputNumber v-model="editForm.max_concurrency" :min="0" class="w-full" />
              <template #extra><span class="field-help">0 = 自动估算（按上游 429 水位动态调整）</span></template>
            </AFormItem>
          </ACol>
          <ACol :span="12">
            <AFormItem label="严格容量">
              <ASwitch v-model="editForm.strict_capacity" />
              <template #extra><span class="field-help">Redis 故障时按保守限额拒绝新请求（高成本渠道）</span></template>
            </AFormItem>
          </ACol>
        </ARow>
        <ARow :gutter="16">
          <ACol :span="12">
            <AFormItem label="状态">
              <ASelect v-model="editForm.status" :options="[{ label: '启用', value: 'active' }, { label: '禁用', value: 'disabled' }, { label: '测试中', value: 'testing' }]" />
            </AFormItem>
          </ACol>
          <ACol :span="12">
            <AFormItem label="测试模型"><AInput v-model="editForm.test_model" /></AFormItem>
          </ACol>
        </ARow>
        <ARow :gutter="16">
          <ACol :span="12">
            <AFormItem label="使用代理">
              <ASwitch v-model="editForm.use_proxy" />
            </AFormItem>
          </ACol>
        </ARow>

        <AFormItem label="备注"><AInput v-model="editForm.remark" type="textarea" :auto-size="{ minRows: 2, maxRows: 4 }" /></AFormItem>

        <ACollapse :bordered="false">
          <ACollapseItem header="VIP 设置" key="vip" :header-style="{ fontSize: '13px' }">
            <ARow :gutter="16">
              <ACol :span="12">
                <AFormItem label="VIP 渠道"><ASwitch v-model="editForm.is_vip" /></AFormItem>
              </ACol>
              <ACol v-if="editForm.is_vip" :span="12">
                <AFormItem label="共享阈值"><AInputNumber v-model="editForm.sharing_threshold" :min="0" :max="100" class="w-full" /></AFormItem>
              </ACol>
            </ARow>
            <ARow v-if="editForm.is_vip" :gutter="16">
              <ACol :span="12">
                <AFormItem label="抢占阈值"><AInputNumber v-model="editForm.preemption_threshold" :min="0" :max="100" class="w-full" /></AFormItem>
              </ACol>
              <ACol :span="12">
                <AFormItem label="借用冷却(秒)"><AInputNumber v-model="editForm.borrowing_cooldown_seconds" :min="0" class="w-full" /></AFormItem>
              </ACol>
            </ARow>
          </ACollapseItem>
        </ACollapse>
      </AForm>
    </AModal>

    <!-- Update Key Modal -->
    <AModal v-model:visible="showUpdateKeyModal" title="更新 API Key" :width="450" :on-before-ok="handleUpdateKey" :ok-loading="updateKeyLoading">
      <AForm :model="{ key: newApiKey }" layout="vertical">
        <AFormItem label="新 API Key" required>
          <AInput v-model="newApiKey" type="textarea" :auto-size="{ minRows: 3, maxRows: 5 }" placeholder="输入新的 API Key" />
        </AFormItem>
      </AForm>
    </AModal>

    <!-- Clone Channel Modal -->
    <AModal v-model:visible="showCloneModal" title="克隆渠道" :width="500" :on-before-ok="handleClone" :ok-loading="cloneLoading">
      <AForm :model="cloneForm" layout="vertical">
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
            :loading="abilitiesLoading"
            allow-search
            allow-clear
            
            placeholder="选择渠道支持的模型"
            :fallback-option="false"
          />
        </AFormItem>
        <AFormItem label="上游模型名"><AInput v-model="addAbilityForm.upstream_model" placeholder="留空则与平台模型名相同" /></AFormItem>
        <AFormItem label="启用">
          <ASelect v-model="addAbilityForm.enabled" :options="[{ label: '启用', value: true }, { label: '禁用', value: false }]" style="width: 120px" />
        </AFormItem>
      </AForm>
    </AModal>

    <!-- Debug Log Detail Drawer -->
    <ADrawer v-model:visible="showDebugDetail" title="调试日志详情" :width="880" unmount-on-close>
      <ASpin :loading="debugDetailLoading" style="width: 100%; display: block;">
        <template v-if="debugDetail">
          <ADescriptions :column="2" bordered size="small" class="mb-4">
            <ADescriptionsItem v-for="item in debugMetaItems" :key="item.label" :label="item.label" :span="item.span || 1">
              <div class="debug-meta-row">
                <span class="debug-meta-text" :class="{ mono: item.mono }" :style="item.color ? { color: item.color, fontWeight: 600 } : undefined">
                  {{ item.value || '-' }}
                </span>
                <Button v-if="item.value" size="mini" type="text" class="debug-copy-btn" @click="copyText(item.value, item.label)">复制</Button>
              </div>
            </ADescriptionsItem>
          </ADescriptions>

          <ACollapse :bordered="false" :default-active-key="['client_req']">
            <ACollapseItem v-for="seg in debugSegments" :key="seg.key" :header="`${seg.title}（${formatBytes(seg.bytes)}）`" :header-style="{ fontSize: '13px', fontWeight: 600 }">
              <div class="debug-seg-label">
                <span>请求/响应头</span>
                <Button v-if="seg.headers" size="mini" type="text" class="debug-copy-btn" @click="copyText(formatHeaders(seg.headers), '请求/响应头')">复制</Button>
              </div>
              <pre class="debug-pre">{{ seg.headers ? formatHeaders(seg.headers) : '（无）' }}</pre>

              <div class="debug-seg-label" style="margin-top: 10px;">
                <span>
                  内容（{{ seg.contentType || '未知类型' }}<template v-if="seg.encoding === 'base64'"> · base64 编码</template>）
                </span>
                <Button v-if="seg.body" size="mini" type="text" class="debug-copy-btn" @click="copySegBody(seg)">复制</Button>
              </div>
              <template v-if="seg.encoding === 'base64'">
                <div class="debug-binary-note">
                  二进制内容（{{ formatBytes(seg.bytes) }}，base64 存储{{ seg.imageUri ? '，下方预览' : '，点上方复制获取完整 base64' }}）
                </div>
                <img v-if="seg.imageUri" :src="seg.imageUri" style="max-width: 100%; max-height: 320px; border-radius: 6px; margin-top: 8px; border: 1px solid var(--color-border-2);" alt="binary preview" />
              </template>
              <pre v-else class="debug-pre">{{ seg.bodyText || '（无内容）' }}</pre>
            </ACollapseItem>
          </ACollapse>
        </template>
      </ASpin>
    </ADrawer>
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
.field-help { color: var(--color-text-3); font-size: 12px; }
.debug-seg-label {
  display: flex; align-items: center; justify-content: space-between; gap: 8px;
  font-size: 12px; font-weight: 600; color: var(--color-text-2); margin-bottom: 6px;
}
.debug-meta-row { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-width: 0; }
.debug-meta-text { min-width: 0; word-break: break-all; }
.debug-meta-text.mono { font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace; font-size: 12px; }
.debug-copy-btn { flex-shrink: 0; color: var(--color-text-3); }
.debug-copy-btn:hover { color: rgb(var(--primary-6)); }
.debug-pre {
  font-size: 12px; line-height: 1.6; margin: 0; padding: 10px; border-radius: 6px;
  background: var(--color-fill-1); color: var(--color-text-2);
  white-space: pre-wrap; word-break: break-all; max-height: 360px; overflow-y: auto;
}
.debug-binary-note {
  font-size: 12px; color: var(--color-text-3); padding: 8px 10px; border-radius: 6px;
  background: var(--color-fill-1);
}
.form-group-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-2);
  margin: 12px 0 12px;
  padding-left: 8px;
  border-left: 3px solid #165dff;
  line-height: 1.2;
}
</style>
