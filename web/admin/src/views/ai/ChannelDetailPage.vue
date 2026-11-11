<script setup lang="ts">
import { ref, reactive, computed, onMounted, h, nextTick, onBeforeUnmount, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Tag, Button, Popconfirm, Message, RadioGroup, Radio, InputNumber, Input, Switch, DatePicker } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import ChannelModelTestModal from '@/components/ChannelModelTestModal.vue'
import { providerTypeOptions, filterProviderOption } from '@/constants/channel'
import { usePluginOptions } from '@/composables/usePluginOptions'
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

// 凭证冷却剩余秒数 → 人类可读（后端返回 Redis TTL，最长为策略里的 credCooldownSeconds）
function formatCooldown(seconds: number): string {
  const s = Math.max(0, Math.floor(seconds || 0))
  if (s < 60) return `${s} 秒`
  return `${Math.floor(s / 60)} 分 ${String(s % 60).padStart(2, '0')} 秒`
}
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
  use_proxy: false,
  web_search_to_google_search: false,
  plugin_key: '',
})

// JS 任务插件渠道（type=43）绑定插件选项：打开编辑弹窗时懒加载
const { pluginOptions, loadPluginOptions } = usePluginOptions()
const editFormRules = computed(() => ({
  ...(editForm.type === 43 ? { plugin_key: [{ required: true, message: '请选择绑定的插件' }] } : {}),
}))

// 类型切走 43 时清掉残留的插件绑定，避免提交时带上无效 plugin_key
watch(() => editForm.type, (t) => {
  if (t !== 43) editForm.plugin_key = ''
  else loadPluginOptions()
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
    use_proxy: detail.value.use_proxy || false,
    web_search_to_google_search: detail.value.web_search_to_google_search || false,
    plugin_key: detail.value.plugin_key || '',
  })
  if (detail.value.type === 43) loadPluginOptions()
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
        size: 'small',
        onChange: (v: string | number | boolean) => handleToggleProtoFlag(record, 'supports_responses', Boolean(v)),
      })
    },
  },
  {
    title: 'Chat 经 Responses', dataIndex: 'chat_via_responses', width: 120,
    render({ record }) {
      return h(Switch, {
        modelValue: !!record.chat_via_responses,
        size: 'small',
        onChange: (v: string | number | boolean) => handleToggleProtoFlag(record, 'chat_via_responses', Boolean(v)),
      })
    },
  },
  {
    title: '启用', dataIndex: 'enabled', width: 80,
    render({ record }) {
      // 用开关而非文字标签：一眼能看出这是可切换项，而不是只读状态
      return h(Switch, {
        modelValue: !!record.enabled,
        size: 'small',
        onChange: (v: string | number | boolean) => handleToggleAbility(record, Boolean(v)),
      })
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
    title: '操作', dataIndex: 'actions', width: 210, fixed: 'right',
    render({ record }) {
      return h('div', { style: 'display: flex; gap: 8px' }, [
        h(Button, {
          size: 'mini',
          type: 'outline',
          onClick: () => openModelTest(record.model_name),
        }, () => '测试'),
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

// === 模型能力行快捷测试（共用 ChannelModelTestModal，打开即自动测试该模型） ===
const showModelTestModal = ref(false)
const autoTestModel = ref<string | null>(null)

function openModelTest(modelName: string) {
  autoTestModel.value = modelName
  showModelTestModal.value = true
}

// 关闭弹窗时清除自动测试目标，避免下次打开误触发
watch(showModelTestModal, (v) => {
  if (!v) autoTestModel.value = null
})

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

async function handleToggleAbility(ab: any, value: boolean) {
  const newList = abilitiesData.value.map(a => a.id === ab.id ? { ...a, enabled: value } : a)
  try {
    await request.put(`/admin/channels/${channelId}/abilities`, abilityPayload(newList))
    abilitiesData.value = newList
    Message.success(value ? '模型已启用' : '模型已禁用')
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
    await request.post(`/admin/channels/${channelId}/reset-health`, {
      model_name: record.model_name,
    })
    Message.success(`模型 ${record.model_name} 健康已重置`)
    fetchAbilities()
  } catch {
    // error handled by interceptor
  }
}

// === 添加模型能力弹窗 ===
// 待添加模型只有一份数据源 pendingModelNames（元素 = 上游模型名）：手动输入、粘贴、
// 从渠道列表点选都落到这个数组。平台模型名（映射）挂在 platformNameOverrides 上，
// 未改写即与上游同名——映射在弹窗内的「待添加清单」逐行就地填写，提交时一次落库，
// 不必先添加、关窗，再回能力表里逐行改。
const showAddAbilityModal = ref(false)
const modelsList = ref<any[]>([])

const pendingModelNames = ref<string[]>([])
// 标签输入区自绘（不用 AInputTag）：它内部另存一份 tag 状态、不监听外部数组变化，
// 从列表点选/取消时 UI 会滞后一拍。自绘后数组就是唯一数据源，点列表立刻同步。
const tagInputText = ref('')
// 上游名 → 平台名的改写；没有 key 表示「与上游同名」
const platformNameOverrides = reactive<Record<string, string>>({})
const tagFieldFocused = ref(false)
const tagInputRef = ref<HTMLInputElement>()

const upstreamModels = ref<{ id: string; owned_by?: string }[]>([])
const upstreamLoading = ref(false)
const upstreamError = ref('')
const upstreamFetched = ref(false)
const upstreamKeyword = ref('')
const upstreamPanelOpen = ref(false)

const existingModelNames = computed(() => new Set(abilitiesData.value.map(a => a.model_name)))
// 已被本渠道某条能力占用的上游模型名（含映射过的 upstream_model），上游列表据此标「已配置」
const existingUpstreamNames = computed(() => new Set(abilitiesData.value.map(a => a.upstream_model || a.model_name)))
// 平台模型库（/admin/models/options，含 active + deprecated）里的模型名。上游独有的模型名不在其中，
// 这类模型加了能力也上不了架、算不出价，在弹窗里明确提示而不是默默加进去。
// deprecated 模型网关仍放行（GetModelMapping 只拒 offline），必须算作「已收录」，否则会被误标。
const platformModelNames = computed(() => new Set(modelsList.value.map((m: any) => m.model_id as string)))
const deprecatedModelNames = computed(() => new Set(
  modelsList.value.filter((m: any) => m.status === 'deprecated').map((m: any) => m.model_id as string),
))
// 可用（active）模型名：映射建议与下拉候选只推荐它们，不引导把新渠道挂到已弃用模型上
const activeModelNames = computed(() => new Set(
  modelsList.value.filter((m: any) => m.status !== 'deprecated').map((m: any) => m.model_id as string),
))

// 是否「平台模型库未收录」。模型库列表拿不到时不做判断，
// 否则会把整屏模型误标成未收录。
function isUncataloged(name: string): boolean {
  return modelsList.value.length > 0 && !platformModelNames.value.has(name)
}

function isDeprecatedModel(name: string): boolean {
  return deprecatedModelNames.value.has(name)
}

// 平台模型名下拉候选（AutoComplete 自带按输入过滤）
const platformModelOptions = computed(() => [...activeModelNames.value])

function platformNameOf(upstream: string): string {
  const v = platformNameOverrides[upstream]
  return v && v.trim() ? v.trim() : upstream
}

function setPlatformName(upstream: string, value: string) {
  const v = (value || '').trim()
  if (!v || v === upstream) delete platformNameOverrides[upstream]
  else platformNameOverrides[upstream] = v
}

// 上游名不在平台模型库时，推断一个平台统一名作为映射建议：
// 1) 去掉 vendor/ 前缀后命中（如 openai/gpt-4o → gpt-4o）；
// 2) 最长前缀命中且其后紧跟分隔符（如 gpt-4o-2024-08-06 → gpt-4o；有 gpt-4o-mini 时 gpt-4o-mini-xxx 取更长的 gpt-4o-mini）。
// 只给建议、不自动改写——日期快照版与平台名是否等价要由运营判断。
function suggestPlatformName(upstream: string): string {
  if (!modelsList.value.length || platformModelNames.value.has(upstream)) return ''
  const set = activeModelNames.value
  const base = upstream.includes('/') ? upstream.slice(upstream.lastIndexOf('/') + 1) : upstream
  if (base !== upstream && set.has(base)) return base
  let best = ''
  for (const id of set) {
    if (id.length > best.length && base.length > id.length && base.startsWith(id) && /[-_.:@]/.test(base[id.length])) best = id
  }
  return best
}

type PendingRow = { upstream: string, platform: string, mapped: boolean, status: '' | 'exists' | 'dup' | 'uncataloged' | 'deprecated', suggestion: string }

// 待添加清单：每行一个上游模型及其映射，状态按「平台名」判定（能力唯一键是 渠道×平台模型名）
const pendingRows = computed<PendingRow[]>(() => {
  const count = new Map<string, number>()
  for (const u of pendingModelNames.value) {
    const p = platformNameOf(u)
    count.set(p, (count.get(p) || 0) + 1)
  }
  return pendingModelNames.value.map(u => {
    const platform = platformNameOf(u)
    let status: PendingRow['status'] = ''
    if (existingModelNames.value.has(platform)) status = 'exists'
    else if ((count.get(platform) || 0) > 1) status = 'dup'
    else if (isUncataloged(platform)) status = 'uncataloged'
    else if (isDeprecatedModel(platform)) status = 'deprecated'
    const suggestion = status === 'uncataloged' ? suggestPlatformName(platform) : ''
    return { upstream: u, platform, mapped: platform !== u, status, suggestion }
  })
})

const suggestableRows = computed(() => pendingRows.value.filter(r => r.suggestion))
const addableCount = computed(() => pendingRows.value.filter(r => r.status !== 'exists' && r.status !== 'dup').length)

function applyAllSuggestions() {
  for (const r of suggestableRows.value) setPlatformName(r.upstream, r.suggestion)
}

const filteredUpstreamModels = computed(() => {
  const kw = upstreamKeyword.value.trim().toLowerCase()
  if (!kw) return upstreamModels.value
  return upstreamModels.value.filter(m => m.id.toLowerCase().includes(kw))
})

function isPending(name: string): boolean {
  return pendingModelNames.value.includes(name)
}

// 列表点击 = 加入 / 移出标签输入框，选中状态直接由标签数组决定，
// 不额外维护一份勾选表，避免两边不同步。
function toggleUpstreamModel(id: string) {
  if (existingUpstreamNames.value.has(id)) return
  const idx = pendingModelNames.value.indexOf(id)
  if (idx >= 0) {
    pendingModelNames.value.splice(idx, 1)
    delete platformNameOverrides[id]
  } else pendingModelNames.value.push(id)
}

// 按分隔符把一段文本拆成多个模型名加进标签区。
// 不把 "/" 当分隔符——vendor/model 这类模型名会被拆坏。
function addPendingNames(raw: string): number {
  let added = 0
  for (const piece of raw.split(/[\s,，;；、]+/)) {
    const name = piece.trim()
    if (!name || pendingModelNames.value.includes(name)) continue
    pendingModelNames.value.push(name)
    added++
  }
  return added
}

function commitTagInput() {
  const text = tagInputText.value
  if (!text.trim()) return
  tagInputText.value = ''
  if (!addPendingNames(text)) Message.warning('该模型已在待添加清单中')
}

function handleTagInputKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') {
    e.preventDefault()
    commitTagInput()
  }
}

function handleTagPaste(e: ClipboardEvent) {
  const text = e.clipboardData?.getData('text') || ''
  // 单个名字走默认输入行为，含分隔符才接管，避免破坏手工逐字输入
  if (!/[\s,，;；、]/.test(text.trim())) return
  e.preventDefault()
  addPendingNames(text)
}

function removePendingName(name: string) {
  pendingModelNames.value = pendingModelNames.value.filter(n => n !== name)
  delete platformNameOverrides[name]
}

function focusTagInput() {
  tagInputRef.value?.focus()
}

// 失焦时把已输入的文本提交掉：用户输完直接点「确定」不会丢
function handleTagInputBlur() {
  tagFieldFocused.value = false
  commitTagInput()
}

async function loadUpstreamModels(silent = false) {
  upstreamLoading.value = true
  upstreamError.value = ''
  try {
    const res: any = await request.get(`/admin/channels/${channelId}/upstream_models`, { _suppressErrorMsg: silent } as any)
    const data = res.data?.data || res.data
    upstreamModels.value = data?.list || []
    upstreamFetched.value = true
    if (!upstreamModels.value.length) upstreamError.value = '上游未返回模型列表，可直接在上方输入框手动填写'
  } catch (e: any) {
    upstreamModels.value = []
    upstreamFetched.value = true
    upstreamError.value = e?.response?.data?.message || e?.message || '获取失败，可在上方输入框手动填写'
  } finally {
    upstreamLoading.value = false
  }
}

// 「从渠道获取」按钮：首次点击才发请求，再次点击收起列表（不重复打上游）
async function toggleUpstreamPanel() {
  if (upstreamPanelOpen.value) {
    upstreamPanelOpen.value = false
    return
  }
  upstreamPanelOpen.value = true
  if (upstreamFetched.value) return
  await loadUpstreamModels(true)
}

function clearPendingModels() {
  pendingModelNames.value = []
  for (const k of Object.keys(platformNameOverrides)) delete platformNameOverrides[k]
}

function resetAddAbility() {
  clearPendingModels()
  tagInputText.value = ''
  upstreamKeyword.value = ''
  upstreamError.value = ''
  upstreamFetched.value = false
  upstreamModels.value = []
  upstreamPanelOpen.value = false
}

// 拉取 active + deprecated 模型（不分页接口 /admin/models/options），
// 用来判断「平台模型库是否已收录」并给出未收录 / 已弃用提示。offline 不拉：网关对其按不存在处理。
async function fetchModels() {
  try {
    const res: any = await request.get('/admin/models/options', { params: { status: 'active,deprecated' } })
    modelsList.value = res.data?.data?.list || res.data?.list || []
  } catch { modelsList.value = [] }
}

function openAddAbilityModal() {
  resetAddAbility()
  showAddAbilityModal.value = true
  fetchModels()
}

async function handleAddAbility(done: (closed: boolean) => void) {
  // 输入框里还没回车的文本也算进来，用户输完直接点「确定」不会丢
  commitTagInput()
  const rows = pendingRows.value
  if (!rows.length) { Message.warning('请先输入模型名，或点「从渠道获取」选择'); return false }
  if (rows.some(r => r.status === 'dup')) { Message.warning('多个上游模型映射到了同一个平台模型名，请调整后再提交'); return false }

  // upstream_model 显式写入：留空虽然等价于「同名」，但之后在列表里改平台名时上游名会跟着变，容易打错上游。
  const additions = rows
    .filter(r => r.status !== 'exists')
    .map(r => ({ model_name: r.platform, upstream_model: r.upstream, enabled: true, cost_ratio: 1 }))
  if (!additions.length) { Message.warning('这些模型都已在本渠道配置中'); return false }

  const skipped = rows.length - additions.length
  try {
    await request.put(`/admin/channels/${channelId}/abilities`, abilityPayload([...abilitiesData.value, ...additions]))
    Message.success(
      skipped > 0
        ? `已添加 ${additions.length} 个模型能力，跳过 ${skipped} 个重复项`
        : `已添加 ${additions.length} 个模型能力`,
    )
    done(true)
    resetAddAbility()
    fetchAbilities()
  } catch { return false }
}

// === Tab 3: Health Trend ===
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

// === Tab 4: Debug Logs（渠道调试日志） ===
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
    debugDetail.value = res.data?.data || null
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
                  <ATag v-if="detail.type === 43 && detail.plugin_key" size="small" color="cyan" style="margin-left: 6px">
                    {{ detail.plugin_key }}
                  </ATag>
                  <ATag v-else-if="detail.type === 43" size="small" color="orangered" style="margin-left: 6px">未绑定插件</ATag>
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
                  <!-- 渠道分是各模型均值，单个模型全挂会被摊薄；最差项低于健康档时点名 -->
                  <span
                    v-if="detail.status === 'active' && detail.worst_model && detail.worst_model_score != null && detail.worst_model_score < 80"
                    :style="{ marginLeft: '8px', color: healthColor(detail.worst_model_score), fontSize: '12px' }"
                  >
                    ⚠ 最差模型 {{ detail.worst_model }} {{ detail.worst_model_score.toFixed(0) }} 分
                  </span>
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
                <ADescriptionsItem label="代理">
                  <ATag v-if="detail.use_proxy" color="arcoblue" size="small">已启用</ATag>
                  <span v-else>-</span>
                </ADescriptionsItem>
                <ADescriptionsItem label="搜索映射">
                  <ATooltip content="将客户端的 web_search 工具映射为 Gemini 原生 googleSearch（仅 Gemini 上游生效，搜索按次另行计价）">
                    <ATag v-if="detail.web_search_to_google_search" color="arcoblue" size="small">googleSearch</ATag>
                    <span v-else>-</span>
                  </ATooltip>
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
                <ADescriptionsItem label="凭证冷却">
                  <ATag v-if="(detail.key_cooldown_remaining ?? 0) > 0" color="orangered" size="small">
                    冷却中 · 剩余 {{ formatCooldown(detail.key_cooldown_remaining) }}
                  </ATag>
                  <ATag v-else color="green" size="small">正常</ATag>
                </ADescriptionsItem>
                <ADescriptionsItem v-if="detail.token_expires_at" label="Token 过期时间">
                  {{ detail.token_expires_at }}
                </ADescriptionsItem>
              </ADescriptions>
              <AAlert
                v-if="(detail.key_cooldown_remaining ?? 0) > 0"
                type="warning"
                style="margin-top: 12px;"
              >
                该 Key 因认证失败（401/403）被调度器冷却，冷却期内本渠道会被整体跳过且不会发起上游请求
                —— 此时渠道测试仍会成功（测试不走调度器），但实际调用会返回「无可用渠道」。
                更新 Key 或点击「重置健康度」都会立即解除冷却。
              </AAlert>
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
                  <div>
                    <span style="font-weight: 500;">启用：</span>关闭后该模型在本渠道不参与调度（请求会走其它已启用渠道），配置本身保留，可随时重新打开。
                  </div>
                </div>
              </div>
            </ACard>
          </ATabPane>

          <!-- Tab 3: Health Trend -->
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

          <!-- Tab 4: Debug Logs -->
          <ATabPane key="debug_logs" title="调试日志">
            <!-- 配置区块 -->
            <ACard :bordered="false" class="mb-4">
              <template #title>
                <div class="flex items-center" style="gap: 8px;">
                  <span>调试配置</span>
                  <span style="font-size: 12px; color: var(--color-text-3); font-weight: normal;">开启后记录每次请求尝试的四段完整报文（含脱敏凭证），排障完成请及时关闭并清理</span>
                </div>
              </template>
              <template #extra>
                <div class="flex items-center" style="gap: 8px;">
                  <span style="font-size: 13px; color: var(--color-text-2);">调试开关</span>
                  <Switch
                    :model-value="!!detail.debug_log_enabled"
                    :loading="debugSwitchLoading"
                    @change="toggleDebugLog"
                  />
                </div>
              </template>

              <!-- 捕捉范围配置 -->
              <div class="form-group-title">捕捉范围配置</div>
              <div style="padding: 12px; background: var(--color-fill-1); border-radius: 6px;">
                <div class="flex items-center flex-wrap" style="gap: 10px 12px;">
                  <div style="display: flex; align-items: center; gap: 8px;">
                    <span style="font-size: 13px; color: var(--color-text-2); min-width: 60px;">租户：</span>
                    <ASelect
                      :model-value="debugTarget.tenantId"
                      :options="debugTenantOptions"
                      placeholder="不限（捕捉所有租户）"
                      allow-search
                      allow-clear
                      size="small"
                      style="width: 240px"
                      @search="searchDebugTenants"
                      @change="handleDebugTenantChange"
                    />
                  </div>
                  <div style="display: flex; align-items: center; gap: 8px;">
                    <span style="font-size: 13px; color: var(--color-text-2); min-width: 60px;">成员：</span>
                    <ASelect
                      :model-value="debugTarget.userId"
                      :options="debugUserOptions"
                      placeholder="不限"
                      allow-search
                      allow-clear
                      size="small"
                      style="width: 220px"
                      :disabled="!debugTarget.tenantId"
                      @change="handleDebugUserChange"
                    />
                  </div>
                  <div style="display: flex; align-items: center; gap: 8px;">
                    <span style="font-size: 13px; color: var(--color-text-2); min-width: 60px;">密钥：</span>
                    <ASelect
                      :model-value="debugTarget.apiKeyId"
                      :options="debugKeyOptions"
                      placeholder="不限"
                      allow-search
                      allow-clear
                      size="small"
                      style="width: 260px"
                      :disabled="!debugTarget.tenantId"
                      @change="(v: any) => debugTarget.apiKeyId = v ?? null"
                    />
                  </div>
                  <AButton
                    size="small"
                    type="primary"
                    :loading="debugTargetSaving"
                    @click="saveDebugTarget"
                  >
                    <template #icon><icon-save /></template>
                    保存配置
                  </AButton>
                </div>
                <div style="margin-top: 8px; font-size: 12px; color: var(--color-text-3); padding-left: 2px;">
                  提示：未选择任何过滤条件时，捕捉该渠道的所有请求；选择后只捕捉匹配的请求。配置保存后立即生效。
                </div>
              </div>
            </ACard>

            <!-- 日志列表区块 -->
            <ACard :bordered="false">
              <template #title>
                <div class="flex items-center justify-between" style="width: 100%;">
                  <div class="flex items-center" style="gap: 8px;">
                    <span>日志记录</span>
                    <span v-if="debugStats && debugStats.total > 0" style="font-size: 12px; color: var(--color-text-3); font-weight: normal;">
                      累计 {{ debugStats.total }} 条 / {{ formatBytes(debugStats.total_bytes || 0) }}<span v-if="debugStats.oldest_at">，最早 {{ debugStats.oldest_at }}</span>，无自动过期请及时清理
                    </span>
                  </div>
                  <Popconfirm content="确定清空该渠道全部调试日志？此操作不可逆" @ok="clearDebugLogs">
                    <AButton status="danger" type="outline" size="small" :disabled="!debugStats || debugStats.total === 0">
                      <template #icon><icon-delete /></template>
                      清空日志
                    </AButton>
                  </Popconfirm>
                </div>
              </template>

              <!-- 查询过滤 -->
              <div class="flex items-center flex-wrap mb-3" style="gap: 8px;">
                <Input v-model="debugFilter.request_id" placeholder="Request ID" size="small" style="width: 200px" allow-clear @clear="handleDebugFilter" @press-enter="handleDebugFilter" />
                <Input v-model="debugFilter.model_name" placeholder="模型名" size="small" style="width: 160px" allow-clear @clear="handleDebugFilter" @press-enter="handleDebugFilter" />
                <DatePicker
                  :model-value="debugFilter.dateRange"
                  range
                  size="small"
                  style="width: 240px"
                  format="YYYY-MM-DD"
                  @change="(val: any) => { debugFilter.dateRange = val }"
                />
                <ASelect v-model="debugFilter.only_error" :options="[{ label: '全部记录', value: false }, { label: '仅错误', value: true }]" size="small" style="width: 110px" @change="handleDebugFilter" />
                <AButton type="primary" size="small" @click="handleDebugFilter">搜索</AButton>
                <AButton size="small" @click="resetDebugFilter">重置</AButton>
              </div>

              <!-- 日志表格 -->
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
    <AModal v-model:visible="showEditModal" title="编辑渠道" :width="920" :mask-closable="false" :on-before-ok="handleEditSubmit" :ok-loading="editLoading">
      <AForm :model="editForm" :rules="editFormRules" :auto-label-width="true" layout="vertical">
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
        <AFormItem v-if="editForm.type === 43" field="plugin_key" label="绑定插件" required>
          <ASelect
            v-model="editForm.plugin_key"
            :options="pluginOptions"
            placeholder="选择该渠道服务的 JS 插件"
            allow-search
          />
          <template #extra>
            <span class="field-help">ⓘ /pjs/{{ editForm.plugin_key || '{plugin_key}' }}/ 路由的请求只会调度到绑定了该插件的渠道</span>
          </template>
        </AFormItem>
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
          <ACol :span="12">
            <AFormItem label="web_search → googleSearch" tooltip="将客户端的 web_search 工具映射为 Gemini 原生 googleSearch。仅 Gemini 上游生效；搜索在 token 之外按次计价，确认上游账单口径后再开启">
              <ASwitch v-model="editForm.web_search_to_google_search" />
            </AFormItem>
          </ACol>
        </ARow>

        <AFormItem label="备注"><AInput v-model="editForm.remark" type="textarea" :auto-size="{ minRows: 2, maxRows: 4 }" /></AFormItem>
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

    <!-- Add Ability Modal：选上游模型 → 待添加清单里就地填映射（上游名 → 平台名）→ 一次提交 -->
    <AModal
      v-model:visible="showAddAbilityModal"
      title="添加模型能力"
      :width="820"
      :mask-closable="false"
      :on-before-ok="handleAddAbility"
      :ok-text="addableCount ? `添加 ${addableCount} 个模型` : '确定'"
      unmount-on-close
    >
      <div class="add-ability-body">
        <!-- ① 录入：手输 / 粘贴 / 从渠道获取，均以「上游模型名」进入待添加清单 -->
        <div class="model-picker-row">
          <div class="tag-field" :class="{ 'is-focus': tagFieldFocused }" @click="focusTagInput">
            <input
              ref="tagInputRef"
              v-model="tagInputText"
              class="tag-field-input"
              aria-label="上游模型名"
              placeholder="输入上游模型名后回车，可一次粘贴多个（逗号 / 空格 / 换行分隔）"
              @keydown="handleTagInputKeydown"
              @paste="handleTagPaste"
              @focus="tagFieldFocused = true"
              @blur="handleTagInputBlur"
            />
          </div>
          <AButton class="fetch-upstream-btn" :loading="upstreamLoading" @click="toggleUpstreamPanel">
            {{ upstreamPanelOpen ? '收起列表' : '从渠道获取' }}
          </AButton>
        </div>

        <div v-if="upstreamPanelOpen" class="upstream-panel">
          <div class="upstream-toolbar">
            <AInput v-model="upstreamKeyword" size="small" placeholder="搜索上游模型名" allow-clear style="width: 220px" />
            <AButton size="small" :loading="upstreamLoading" @click="loadUpstreamModels()">刷新</AButton>
            <span v-if="upstreamFetched && !upstreamError" class="upstream-meta">
              上游 {{ upstreamModels.length }} 个模型，点击加入 / 移出待添加清单
            </span>
          </div>

          <div v-if="upstreamError" class="upstream-error">{{ upstreamError }}</div>

          <ASpin :loading="upstreamLoading" style="width: 100%; display: block">
            <div v-if="filteredUpstreamModels.length" class="upstream-list">
              <div
                v-for="m in filteredUpstreamModels"
                :key="m.id"
                class="upstream-item"
                :class="{ 'is-existing': existingUpstreamNames.has(m.id), 'is-pending': isPending(m.id) }"
                @click="toggleUpstreamModel(m.id)"
              >
                <span class="upstream-item-check" :class="{ 'is-on': isPending(m.id) }">{{ isPending(m.id) ? '✓' : '+' }}</span>
                <span class="upstream-item-name">{{ m.id }}</span>
                <span v-if="existingUpstreamNames.has(m.id)" class="upstream-item-tag">已配置</span>
                <span v-else-if="isUncataloged(m.id)" class="upstream-item-tag is-warn">平台未收录</span>
                <span v-else-if="isDeprecatedModel(m.id)" class="upstream-item-tag is-warn">已弃用</span>
              </div>
            </div>
            <AEmpty v-else-if="upstreamFetched && !upstreamError" description="没有匹配的上游模型" />
            <div v-else-if="!upstreamError" class="upstream-empty">正在获取上游模型…</div>
          </ASpin>
        </div>

        <!-- ② 待添加清单：每行就地填写平台模型名（映射），留空 = 与上游同名 -->
        <div class="pending-section">
          <div class="pending-header">
            <span class="pending-title">待添加 <b>{{ pendingRows.length }}</b> 个模型</span>
            <span class="pending-sub">平台模型名留空即与上游同名，上游叫法不同时在此直接改写</span>
            <span class="upstream-toolbar-gap"></span>
            <AButton v-if="suggestableRows.length" type="text" size="mini" @click="applyAllSuggestions">
              应用全部建议映射（{{ suggestableRows.length }}）
            </AButton>
            <AButton v-if="pendingRows.length" type="text" size="mini" status="danger" @click="clearPendingModels">清空</AButton>
          </div>

          <div v-if="pendingRows.length" class="pending-list">
            <div class="pending-row pending-row-head">
              <span>上游模型名</span>
              <span></span>
              <span>平台模型名</span>
              <span>状态</span>
              <span></span>
            </div>
            <div v-for="r in pendingRows" :key="r.upstream" class="pending-row" :class="{ 'is-skip': r.status === 'exists' }">
              <span class="pending-upstream" :title="r.upstream">{{ r.upstream }}</span>
              <span class="pending-arrow" :class="{ 'is-mapped': r.mapped }" aria-hidden="true">→</span>
              <AAutoComplete
                :model-value="platformNameOverrides[r.upstream] || ''"
                :data="platformModelOptions"
                size="small"
                allow-clear
                :placeholder="r.upstream"
                :aria-label="`${r.upstream} 的平台模型名`"
                @update:model-value="(v: string) => setPlatformName(r.upstream, v)"
              />
              <span class="pending-status">
                <ATag v-if="r.status === 'exists'" size="small" color="gray">已配置，将跳过</ATag>
                <ATag v-else-if="r.status === 'dup'" size="small" color="red">平台名重复</ATag>
                <template v-else-if="r.status === 'uncataloged'">
                  <AButton
                    v-if="r.suggestion"
                    type="text"
                    size="mini"
                    class="pending-suggest"
                    :title="`平台模型库未收录 ${r.platform}，点击映射到 ${r.suggestion}`"
                    @click="setPlatformName(r.upstream, r.suggestion)"
                  >建议 → {{ r.suggestion }}</AButton>
                  <ATag v-else size="small" color="orange" title="平台模型库未收录：能力可配置，但无法定价与上架">平台未收录</ATag>
                </template>
                <ATag
                  v-else-if="r.status === 'deprecated'"
                  size="small"
                  color="orange"
                  :title="`平台模型 ${r.platform} 已弃用：仍可调用与计费，但计划下线，新渠道建议映射到替代模型`"
                >已弃用</ATag>
                <ATag v-else-if="r.mapped" size="small" color="arcoblue">已映射</ATag>
              </span>
              <AButton type="text" size="mini" :aria-label="`移除 ${r.upstream}`" @click="removePendingName(r.upstream)">移除</AButton>
            </div>
          </div>
          <div v-else class="pending-empty">还没有待添加的模型。在上方输入上游模型名，或点「从渠道获取」从上游列表中点选。</div>
        </div>
      </div>
    </AModal>

    <!-- 模型能力行快捷测试弹窗（行「测试」按钮入口，打开即自动测试对应模型） -->
    <ChannelModelTestModal
      v-model:visible="showModelTestModal"
      :channel-id="channelId"
      :channel-name="detail?.name"
      :auto-test-model="autoTestModel"
    />

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

/* === 添加模型能力弹窗 === */
.add-ability-body { display: flex; flex-direction: column; gap: 12px; }
.model-picker-row { display: flex; align-items: flex-start; gap: 8px; }
/* 待添加清单：上游名 → 平台名 映射逐行就地编辑 */
.pending-section { display: flex; flex-direction: column; gap: 8px; }
.pending-header { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.pending-title { font-size: 13px; color: var(--color-text-1); }
.pending-title b { color: rgb(var(--primary-6)); font-variant-numeric: tabular-nums; }
.pending-sub { font-size: 12px; color: var(--color-text-3); }
.pending-list {
  max-height: 300px;
  overflow-y: auto;
  border: 1px solid var(--color-border-2);
  border-radius: 6px;
}
.pending-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 20px minmax(0, 1fr) 150px 48px;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-top: 1px solid var(--color-border-1);
}
.pending-row:first-child { border-top: none; }
.pending-row-head {
  position: sticky;
  top: 0;
  z-index: 1;
  font-size: 12px;
  color: var(--color-text-3);
  background: var(--color-fill-1);
  padding-top: 4px;
  padding-bottom: 4px;
}
.pending-row.is-skip { opacity: 0.55; }
.pending-upstream {
  font-family: 'SFMono-Regular', Consolas, monospace;
  font-size: 12px;
  color: var(--color-text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pending-arrow { text-align: center; color: var(--color-text-4); }
.pending-arrow.is-mapped { color: rgb(var(--primary-6)); font-weight: 700; }
.pending-status { min-width: 0; overflow: hidden; }
.pending-suggest { max-width: 100%; padding: 0 4px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pending-empty {
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-text-3);
  background: var(--color-fill-1);
  border-radius: 6px;
  padding: 12px;
  text-align: center;
}
/* 标签输入区：外观对齐 Arco 输入框（同样的边框/圆角/内边距），内部是标签 + 原生 input */
.tag-field {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  min-height: 32px;
  padding: 3px 8px;
  border: 1px solid var(--color-border-2);
  border-radius: 4px;
  background: var(--color-bg-2);
  cursor: text;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.tag-field.is-focus {
  border-color: rgb(var(--primary-6));
  box-shadow: 0 0 0 2px var(--color-primary-light-1);
}
.tag-field-input {
  flex: 1;
  min-width: 140px;
  border: none;
  outline: none;
  background: transparent;
  font-size: 14px;
  line-height: 24px;
  color: var(--color-text-1);
  padding: 0;
}
.tag-field-input::placeholder { color: var(--color-text-3); }
/* 标签多时输入框会变成多行，按钮跟第一行对齐而不是被拉高 */
.fetch-upstream-btn { flex-shrink: 0; }
.upstream-panel { display: flex; flex-direction: column; gap: 8px; }
.upstream-toolbar { display: flex; align-items: center; gap: 8px; }
.upstream-toolbar-gap { flex: 1; }
.upstream-meta { font-size: 12px; color: var(--color-text-3); }
.upstream-error {
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-text-warning);
  background: var(--color-warning-light-1);
  border-radius: 6px;
  padding: 6px 10px;
}
.upstream-list {
  max-height: 220px;
  overflow-y: auto;
  border: 1px solid var(--color-border-2);
  border-radius: 6px;
  padding: 4px;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 2px;
}
.upstream-item {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  padding: 4px 6px;
  border-radius: 4px;
  cursor: pointer;
  border: 1px solid transparent;
}
.upstream-item:hover { background: var(--color-fill-1); }
.upstream-item.is-pending {
  background: var(--color-primary-light-1);
  border-color: var(--color-primary-light-3);
}
.upstream-item.is-existing { cursor: not-allowed; opacity: 0.55; }
.upstream-item.is-existing:hover { background: transparent; }
/* 选中标记：未选是浅色 +，已选是主色 ✓，一行内就能看出哪些已进输入框 */
.upstream-item-check {
  flex-shrink: 0;
  width: 12px;
  text-align: center;
  font-size: 12px;
  line-height: 1;
  color: var(--color-text-4);
}
.upstream-item-check.is-on { color: rgb(var(--primary-6)); font-weight: 700; }
.upstream-item-name {
  font-size: 12px;
  color: var(--color-text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}
.upstream-item-tag {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--color-text-3);
  border: 1px solid var(--color-border-2);
  border-radius: 3px;
  padding: 0 4px;
  line-height: 16px;
}
.upstream-item-tag.is-warn { color: var(--color-warning-6); border-color: var(--color-warning-3); }
.upstream-empty { font-size: 12px; color: var(--color-text-3); padding: 12px 0; text-align: center; }
</style>
