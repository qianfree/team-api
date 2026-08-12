<script setup lang="ts">
import { ref, reactive, computed, onMounted, h, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Tag, Button, Space, Popconfirm, Message } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import TableStats from '@/components/TableStats.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import request from '@/utils/request'
import { hasPermission } from '@/utils/permission'
import { useIsMobile } from '@/composables/useIsMobile'

const route = useRoute()
const router = useRouter()
const isMobile = useIsMobile()

const loading = ref(false)
const detail = ref<any>(null)
const activeTab = ref('info')

const tenantId = route.params.id as string

const statusTagColor: Record<string, string> = {
  active: 'green',
  suspended: 'orangered',
  closed: undefined,
}
const statusTagLabel: Record<string, string> = {
  active: '正常',
  suspended: '暂停',
  closed: '已关闭',
}

async function fetchDetail() {
  loading.value = true
  try {
    const res: any = await request.get(`/admin/tenants/${tenantId}`)
    detail.value = res.data?.data || res.data
  } catch {
  } finally {
    loading.value = false
  }
}

async function updateStatus(status: string) {
  try {
    await request.put(`/admin/tenants/${tenantId}/status`, { status })
    Message.success('状态更新成功')
    fetchDetail()
  } catch {
  }
}

function goBack() {
  router.push({ name: 'AdminTenants' })
}

// === Tab 1: Basic Info ===
const editLoading = ref(false)
const editForm = reactive({
  name: '',
  level: null as number | null,
  max_members: null as number | null,
  max_concurrency: null as number | null,
})

// Level options from backend
const levelOptions = ref<any[]>([])

async function fetchLevelOptions() {
  try {
    const res: any = await request.get('/admin/tenant-level-configs')
    levelOptions.value = res.data?.data?.list || res.data?.list || []
  } catch {
  }
}

// Find the effective max_members (custom or from level config)
function getEffectiveMaxMembers() {
  if (detail.value?.max_members != null) return detail.value.max_members
  if (detail.value?.level_config?.max_members != null) return detail.value.level_config.max_members
  const cfg = levelOptions.value.find((l: any) => l.level === detail.value?.level)
  return cfg?.max_members ?? null
}

// Find the effective max_concurrency (custom or from level config)
function getEffectiveMaxConcurrency() {
  if (detail.value?.max_concurrency != null) return detail.value.max_concurrency
  if (detail.value?.level_config?.max_concurrency != null) return detail.value.level_config.max_concurrency
  const cfg = levelOptions.value.find((l: any) => l.level === detail.value?.level)
  return cfg?.max_concurrency ?? null
}

function startEdit() {
  if (!detail.value) return
  editForm.name = detail.value.name || ''
  editForm.level = detail.value.level ?? null
  editForm.max_members = detail.value.max_members ?? null
  editForm.max_concurrency = detail.value.max_concurrency ?? null
}

async function saveEdit() {
  editLoading.value = true
  try {
    await request.put(`/admin/tenants/${tenantId}`, editForm)
    Message.success('更新成功')
    fetchDetail()
  } catch {
  } finally {
    editLoading.value = false
  }
}

// === Tab 2: Model Assignment ===
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
    const res: any = await request.get(`/admin/tenants/${tenantId}/models`)
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

// Assign modal
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
    const res: any = await request.post(`/admin/tenants/${tenantId}/models`, { assignments })
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

// Edit model modal
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
    await request.put(`/admin/tenants/${tenantId}/models/${editingModel.value.model_id}`, payload)
    Message.success('更新成功')
    showEditModelModal.value = false
    fetchTenantModels()
  } catch {
    // error handled by interceptor
  } finally {
    editModelLoading.value = false
  }
}

async function removeModel(record: any) {
  try {
    await request.delete(`/admin/tenants/${tenantId}/models/${record.model_id}`)
    Message.success('已移除')
    fetchTenantModels()
  } catch {
  }
}

// === Tab 2b: Group Assignment ===
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
    const res: any = await request.get(`/admin/tenants/${tenantId}/groups`)
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
    await request.put(`/admin/tenants/${tenantId}/groups`, { group_ids: selectedGroupIds.value })
    Message.success('分组更新成功')
    done()
    fetchTenantGroups()
    fetchTenantModels()
  } catch {
    return false
  } finally {
    groupAssignLoading.value = false
  }
}

// === Available Models Preview ===
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
    const res: any = await request.get(`/admin/tenants/${tenantId}/available-models`)
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

// Load data when switching to models tab
function onTabChange(key: string) {
  if (key === 'models') {
    fetchTenantModels()
    fetchAllModels()
    fetchTenantGroups()
  }
  if (key === 'wallet') {
    fetchWalletInfo()
    fetchFrozenItems()
  }
  if (key === 'transactions') {
    fetchWalletTransactions()
  }
}

// === Tab 3: Wallet Management ===
const walletInfo = ref<any>(null)
const walletLoading = ref(false)
const txData = ref<any[]>([])
const txPagination = reactive({
  current: 1, pageSize: 10, total: 0,
})
const txFilterType = ref('')
const showRechargeModal = ref(false)
const rechargeForm = reactive({ amount: 0, description: '' })
const rechargeLoading = ref(false)
const showOfflineRechargeModal = ref(false)
const offlineRechargeForm = reactive({ amount: 0, transaction_no: '', description: '' })
const offlineRechargeLoading = ref(false)
const showThresholdModal = ref(false)
const thresholdForm = reactive({ threshold: 0 })
const thresholdLoading = ref(false)

const txTypeLabel: Record<string, string> = {
  recharge: '充值', redemption: '兑换码', consume: '消费', pre_deduct: '预扣', settle: '结算',
  refund: '退款', adjust: '调整', freeze: '冻结', unfreeze: '解冻',
}
const txTypeColor: Record<string, string> = {
  recharge: 'green', redemption: 'green', consume: 'orangered', adjust: 'arcoblue', refund: 'cyan',
  pre_deduct: 'orangered', settle: 'orange', freeze: 'red', unfreeze: 'purple',
}

// 交易类型筛选选项（桌面端 RadioGroup 与移动端 ASelect 共用）
const txFilterOptions = [
  { label: '全部', value: '' },
  { label: '充值', value: 'recharge' },
  { label: '兑换码', value: 'redemption' },
  { label: '消费', value: 'consume' },
  { label: '调整', value: 'adjust' },
  { label: '预扣', value: 'pre_deduct' },
  { label: '结算', value: 'settle' },
  { label: '退款', value: 'refund' },
  { label: '冻结', value: 'freeze' },
  { label: '解冻', value: 'unfreeze' },
]

function onTxFilterChange() {
  txPagination.current = 1
  fetchWalletTransactions()
}

const txColumns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  {
    title: '类型', dataIndex: 'type', width: 80,
    render({ record }) {
      return h(Tag, { color: txTypeColor[record.type] || undefined, size: 'small' }, () => txTypeLabel[record.type] || record.type)
    },
  },
  {
    title: '金额', dataIndex: 'amount', width: 140,
    render({ record }) {
      const val = parseFloat(record.amount) || 0
      const color = val >= 0 ? 'rgb(var(--green-6))' : 'rgb(var(--red-6))'
      const prefix = val >= 0 ? '+' : ''
      return h('span', { style: { fontWeight: 600, color } }, `${prefix}${val.toFixed(6)}`)
    },
  },
  {
    title: '变动后余额', dataIndex: 'balance_after', width: 140,
    render({ record }) { return `$${(parseFloat(record.balance_after) || 0).toFixed(6)}` },
  },
  {
    title: '用户', dataIndex: 'username', width: 120,
    render({ record }) { return record.username || '--' },
  },
  { title: '请求ID', dataIndex: 'request_id', width: 140, ellipsis: true },
  { title: '模型', dataIndex: 'model_name', width: 130 },
  { title: '描述', dataIndex: 'description', ellipsis: true },
  { title: '时间', dataIndex: 'created_at', width: 170 },
]

async function fetchWalletInfo() {
  try {
    const res: any = await request.get(`/admin/wallets/${tenantId}`)
    walletInfo.value = res.data?.data || null
  } catch {
  }
}

async function fetchWalletTransactions() {
  walletLoading.value = true
  try {
    const res: any = await request.get(`/admin/wallets/${tenantId}/transactions`, {
      params: { page: txPagination.current, page_size: txPagination.pageSize, type: txFilterType.value || undefined },
    })
    const raw = res.data?.data
    txData.value = raw?.list || []
    txPagination.total = Number(raw?.total) || 0
  } catch {
  } finally {
    walletLoading.value = false
  }
}

async function fetchWalletDetail() {
  await Promise.all([fetchWalletInfo(), fetchWalletTransactions(), fetchFrozenItems()])
}

// === 冻结明细（预扣追踪，按笔释放）===
const frozenItems = ref<any[]>([])
const frozenLoading = ref(false)
const showReleaseModal = ref(false)
const releaseTarget = ref<any>(null)
const releaseForm = reactive({ reason: '' })
const releaseLoading = ref(false)

function formatFrozenAge(seconds: number): string {
  if (seconds < 60) return `${seconds} 秒`
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟`
  return `${Math.floor(seconds / 3600)} 小时 ${Math.floor((seconds % 3600) / 60)} 分`
}

const frozenColumns: TableColumnData[] = [
  { title: '请求ID', dataIndex: 'request_id', width: 200, ellipsis: true, tooltip: true },
  {
    title: '模型', dataIndex: 'model_name', width: 140,
    render({ record }) { return record.model_name || '--' },
  },
  {
    title: '冻结金额', dataIndex: 'amount', width: 130,
    render({ record }) {
      return h('span', { style: { color: 'rgb(var(--orange-6))', fontWeight: 600 } }, `$${(parseFloat(record.amount) || 0).toFixed(6)}`)
    },
  },
  { title: '冻结时间', dataIndex: 'created_at', width: 170 },
  {
    title: '已冻结时长', dataIndex: 'age_seconds', width: 110,
    render({ record }) { return formatFrozenAge(record.age_seconds) },
  },
  { title: '状态', slotName: 'frozenStatus' },
  { title: '操作', slotName: 'frozenAction', width: 110 },
]

async function fetchFrozenItems() {
  frozenLoading.value = true
  try {
    const res: any = await request.get(`/admin/wallets/${tenantId}/frozen-items`)
    frozenItems.value = res.data?.data?.list || []
  } catch {
  } finally {
    frozenLoading.value = false
  }
}

function openRelease(item: any) {
  releaseTarget.value = item
  releaseForm.reason = ''
  showReleaseModal.value = true
}

async function handleRelease(done: (closed: boolean) => void) {
  if (!releaseForm.reason.trim()) { Message.warning('请填写释放原因'); return }
  releaseLoading.value = true
  try {
    const res: any = await request.post(`/admin/wallets/${tenantId}/frozen-items/release`, {
      request_id: releaseTarget.value.request_id,
      force: !!releaseTarget.value.need_force,
      reason: releaseForm.reason.trim(),
    })
    const amt = parseFloat(res.data?.data?.released_amount || 0)
    Message.success(`已释放冻结 $${amt.toFixed(6)}`)
    done(true)
    await fetchWalletDetail()
  } catch {
    return false
  } finally {
    releaseLoading.value = false
  }
}

// === 一键释放全部冻结 ===
const showReleaseAllModal = ref(false)
const releaseAllForm = reactive({ reason: '', force: false })
const releaseAllLoading = ref(false)

function openReleaseAll() {
  releaseAllForm.reason = ''
  releaseAllForm.force = false
  showReleaseAllModal.value = true
}

async function handleReleaseAll(done: (closed: boolean) => void) {
  if (!releaseAllForm.reason.trim()) { Message.warning('请填写释放原因'); return }
  releaseAllLoading.value = true
  try {
    const res: any = await request.post(`/admin/wallets/${tenantId}/frozen-items/release-all`, {
      force: releaseAllForm.force,
      reason: releaseAllForm.reason.trim(),
    })
    const data = res.data?.data || {}
    const amt = parseFloat(data.released_amount || 0)
    const skipped = parseInt(data.skipped_count || 0, 10)
    Message.success(`已释放 ${data.released_count || 0} 笔冻结，合计 $${amt.toFixed(6)}${skipped ? `，跳过 ${skipped} 笔` : ''}`)
    if (skipped) {
      Message.warning(`跳过原因：${(data.skipped_reasons || []).join('；')}`)
    }
    done(true)
    await fetchWalletDetail()
  } catch {
    return false
  } finally {
    releaseAllLoading.value = false
  }
}

function openRecharge() {
  rechargeForm.amount = 0
  rechargeForm.description = ''
  showRechargeModal.value = true
}

async function handleRecharge(done: (closed: boolean) => void) {
  if (rechargeForm.amount === 0) { Message.warning('金额不能为零'); return }
  rechargeLoading.value = true
  try {
    await request.post(`/admin/wallets/${tenantId}/adjust`, {
      amount: rechargeForm.amount,
      description: rechargeForm.description || (rechargeForm.amount > 0 ? '管理员增加余额' : '管理员扣减余额'),
    })
    Message.success(rechargeForm.amount > 0 ? '余额已增加' : '余额已扣减')
    done(true)
    await fetchWalletDetail()
    await fetchDetail()
  } catch {
    return false
  } finally {
    rechargeLoading.value = false
  }
}

function openOfflineRecharge() {
  offlineRechargeForm.amount = 0
  offlineRechargeForm.transaction_no = ''
  offlineRechargeForm.description = ''
  showOfflineRechargeModal.value = true
}

async function handleOfflineRecharge(done: (closed: boolean) => void) {
  if (!offlineRechargeForm.amount || offlineRechargeForm.amount <= 0) { Message.warning('请输入大于 0 的入账金额'); return }
  if (!offlineRechargeForm.description.trim()) { Message.warning('请填写入账说明'); return }
  offlineRechargeLoading.value = true
  try {
    const res: any = await request.post(`/admin/wallets/${tenantId}/offline-recharge`, {
      amount: offlineRechargeForm.amount,
      transaction_no: offlineRechargeForm.transaction_no || undefined,
      description: offlineRechargeForm.description,
    })
    const data = res.data?.data || {}
    Message.success(`入账成功，到账 $${parseFloat(data.credited_usd || 0).toFixed(2)}（汇率 ${data.rate}）`)
    done(true)
    await fetchWalletDetail()
    await fetchDetail()
  } catch {
    return false
  } finally {
    offlineRechargeLoading.value = false
  }
}

function openThreshold() {
  thresholdForm.threshold = walletInfo.value?.warning_threshold ? parseFloat(walletInfo.value.warning_threshold) : 0
  showThresholdModal.value = true
}

async function handleThreshold(done: (closed: boolean) => void) {
  thresholdLoading.value = true
  try {
    await request.put(`/admin/wallets/${tenantId}/warning-threshold`, { threshold: thresholdForm.threshold })
    Message.success('预警阈值已更新')
    done(true)
    await fetchWalletDetail()
  } catch {
    return false
  } finally {
    thresholdLoading.value = false
  }
}

onMounted(() => {
  fetchDetail()
  fetchLevelOptions()
  startEdit()
})
</script>

<template>
  <div class="tenant-detail-page">
    <PageHeader :title="detail ? detail.name : '租户详情'" :description="detail ? `租户代码: ${detail.code}` : ''">
      <template #actions>
        <ASpace :wrap="isMobile" :size="isMobile ? 'small' : 'medium'">
          <AButton @click="goBack">返回列表</AButton>
          <template v-if="detail">
            <Popconfirm
              v-if="detail.status === 'active'"
              content="确定暂停该租户？暂停后租户下所有成员将无法使用服务"
              @ok="updateStatus('suspended')"
            >
              <AButton status="warning">暂停</AButton>
            </Popconfirm>
            <Popconfirm
              v-if="detail.status === 'suspended'"
              content="确定恢复该租户？"
              @ok="updateStatus('active')"
            >
              <AButton status="success">恢复</AButton>
            </Popconfirm>
            <Popconfirm
              v-if="detail.status !== 'closed'"
              content="确定关闭该租户？此操作不可逆"
              @ok="updateStatus('closed')"
            >
              <AButton status="danger">关闭</AButton>
            </Popconfirm>
          </template>
        </ASpace>
      </template>
    </PageHeader>

    <ASpin :loading="loading" class="w-full">
      <template v-if="detail">
        <ATabs v-model:active-key="activeTab" @change="onTabChange">
          <template #extra>
            <AButton v-if="activeTab === 'models'" type="outline" size="small" @click="openPreviewModal">查看可用模型</AButton>
          </template>
          <!-- Tab 1: Basic Info -->
          <ATabPane key="info" title="基本信息">
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <ACard :bordered="false" title="租户信息">
              <ADescriptions :column="isMobile ? 1 : 2" bordered size="medium">
                <ADescriptionsItem label="ID">{{ detail.id }}</ADescriptionsItem>
                <ADescriptionsItem label="租户名称">{{ detail.name }}</ADescriptionsItem>
                <ADescriptionsItem label="租户代码">
                  <ATag size="small">{{ detail.code }}</ATag>
                </ADescriptionsItem>
                <ADescriptionsItem label="状态">
                  <ATag :color="statusTagColor[detail.status]" size="small">
                    {{ statusTagLabel[detail.status] || detail.status }}
                  </ATag>
                </ADescriptionsItem>
                <ADescriptionsItem label="所有者">{{ detail.owner_name || '-' }}</ADescriptionsItem>
                <ADescriptionsItem label="成员数">
                  {{ detail.member_count || 0 }} / {{ getEffectiveMaxMembers() ?? '不限' }}
                  <ATag v-if="detail.max_members != null" size="small" color="arcoblue" style="margin-left: 4px">自定义</ATag>
                </ADescriptionsItem>
                <ADescriptionsItem label="钱包余额">
                  <span class="money">${{ parseFloat(detail.wallet_balance || '0').toFixed(2) }}</span>
                </ADescriptionsItem>
                <ADescriptionsItem label="并发上限">
                  {{ getEffectiveMaxConcurrency() ?? '不限' }}
                  <ATag v-if="detail.max_concurrency != null" size="small" color="arcoblue" style="margin-left: 4px">自定义</ATag>
                </ADescriptionsItem>
                <ADescriptionsItem label="等级">
                  <template v-if="detail.level_config">
                    {{ detail.level_config.name }}
                    <ATag v-if="detail.level_config.price_multiplier" size="small" color="green" style="margin-left: 4px">
                      {{ (detail.level_config.price_multiplier * 100).toFixed(0) }}%
                    </ATag>
                  </template>
                  <template v-else-if="detail.level">
                    Lv.{{ detail.level }}
                  </template>
                  <template v-else>-</template>
                </ADescriptionsItem>
                <ADescriptionsItem label="创建时间">{{ detail.created_at }}</ADescriptionsItem>
                <ADescriptionsItem label="更新时间">{{ detail.updated_at }}</ADescriptionsItem>
              </ADescriptions>
            </ACard>

            <ACard :bordered="false" title="编辑租户">
              <AForm :model="editForm" :auto-label-width="true" layout="vertical">
                <AFormItem label="租户名称">
                  <AInput v-model="editForm.name" class="w-full" />
                </AFormItem>
                <AFormItem label="最大成员数">
                  <AInputNumber v-model="editForm.max_members" :min="1" allow-clear placeholder="跟随等级" class="w-full" />
                </AFormItem>
                <AFormItem label="并发上限">
                  <AInputNumber v-model="editForm.max_concurrency" :min="0" allow-clear placeholder="跟随等级" class="w-full" />
                </AFormItem>
                <AFormItem label="等级">
                  <ASelect v-model="editForm.level" allow-clear placeholder="选择等级" class="w-full">
                    <AOption v-for="opt in levelOptions" :key="opt.level" :value="opt.level">
                      {{ opt.name }}（{{ (opt.price_multiplier * 100).toFixed(0) }}%）
                    </AOption>
                  </ASelect>
                </AFormItem>
                <AFormItem>
                  <AButton type="primary" :loading="editLoading" @click="saveEdit">保存</AButton>
                </AFormItem>
              </AForm>
            </ACard>
          </div>
          </ATabPane>

          <!-- Tab 2: Model Assignment -->
          <ATabPane key="models" title="模型分配">
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
          </ATabPane>

          <!-- Tab 3: Wallet -->
          <ATabPane key="wallet" title="钱包与计费">
            <ASpin :loading="walletLoading">
              <template v-if="walletInfo">
                <!-- Wallet Info Card -->
                <ACard :bordered="false" title="钱包信息" class="mb-4">
                  <template #extra>
                    <ASpace :wrap="isMobile" :size="isMobile ? 'small' : 'medium'">
                      <AButton type="primary" @click="openRecharge">调整余额</AButton>
                      <AButton status="success" @click="openOfflineRecharge">线下入账</AButton>
                      <AButton @click="openThreshold">预警设置</AButton>
                    </ASpace>
                  </template>
                  <ADescriptions :column="isMobile ? 1 : 3" bordered size="medium">
                    <ADescriptionsItem label="可用余额">
                      <span class="money">${{ (walletInfo.balance - walletInfo.frozen_balance).toFixed(6) }}</span>
                    </ADescriptionsItem>
                    <ADescriptionsItem label="总余额">
                      ${{ parseFloat(walletInfo.balance).toFixed(6) }}
                    </ADescriptionsItem>
                    <ADescriptionsItem label="冻结余额">
                      <span style="color: rgb(var(--orange-6))">${{ parseFloat(walletInfo.frozen_balance).toFixed(6) }}</span>
                    </ADescriptionsItem>
                    <ADescriptionsItem label="预警阈值">
                      {{ walletInfo.warning_threshold > 0 ? `$${parseFloat(walletInfo.warning_threshold).toFixed(6)}` : '关闭' }}
                    </ADescriptionsItem>
                  </ADescriptions>
                </ACard>

                <!-- Frozen Items（预扣冻结明细，运维逃生舱） -->
                <ACard :bordered="false" class="mb-4" title="冻结明细">
                  <template #extra>
                    <ASpace>
                      <ATooltip v-if="hasPermission('billing:refund')" content="一次性释放全部可释放的冻结项；任务关联/待结算自动跳过，保护期内需勾选强制">
                        <AButton size="small" :disabled="!frozenItems.length" @click="openReleaseAll">一键释放</AButton>
                      </ATooltip>
                      <AButton size="small" @click="fetchFrozenItems">刷新</AButton>
                    </ASpace>
                  </template>
                  <AAlert v-if="frozenItems.length" type="warning" class="mb-3">
                    以下金额处于预扣冻结中。正常情况下请求结束即自动释放，异常滞留最长约 2 小时由系统自动清理；仅当用户反馈冻结长时间未释放时才需手动处理。
                  </AAlert>
                  <ResponsiveTable
                    :columns="frozenColumns"
                    :data="frozenItems"
                    :loading="frozenLoading"
                    :stripe="true"
                    row-key="request_id"
                    size="small"
                    :card-fields="['amount', 'age_seconds', 'created_at']"
                  >
                    <!-- 桌面端：状态/操作列 slot 透传给内部 ATable -->
                    <template #frozenStatus="{ record }">
                      <ASpace size="mini">
                        <ATag v-if="record.task_status" color="arcoblue" size="small">任务 {{ record.task_status }}</ATag>
                        <span v-if="record.block_reason" style="color: var(--color-text-3); font-size: 12px">{{ record.block_reason }}</span>
                        <ATag v-else-if="record.need_force" color="orangered" size="small">保护期内</ATag>
                        <ATag v-else color="green" size="small">可释放</ATag>
                      </ASpace>
                    </template>
                    <template #frozenAction="{ record }">
                      <ATooltip v-if="hasPermission('billing:refund')" :content="record.block_reason || (record.need_force ? '冻结不足 10 分钟，对应请求可能仍在进行中' : '释放该笔冻结，金额退回可用余额')">
                        <AButton size="mini" :status="record.need_force ? 'danger' : 'warning'" :disabled="!record.releasable" @click="openRelease(record)">
                          {{ record.need_force ? '强制释放' : '释放' }}
                        </AButton>
                      </ATooltip>
                    </template>
                    <!-- 移动端：头部展示请求ID+模型+状态，操作栏放释放按钮 -->
                    <template #card-header="{ row }">
                      <div class="flex items-start justify-between gap-2">
                        <div class="min-w-0">
                          <div class="truncate font-mono text-sm text-[var(--color-text-1)]">{{ row.request_id }}</div>
                          <div class="mt-0.5 truncate text-xs text-[var(--color-text-3)]">{{ row.model_name || '模型未知' }}</div>
                        </div>
                        <div class="flex flex-shrink-0 items-center">
                          <ATag v-if="row.task_status" color="arcoblue" size="small">任务 {{ row.task_status }}</ATag>
                          <ATag v-else-if="row.block_reason" color="gray" size="small">阻塞</ATag>
                          <ATag v-else-if="row.need_force" color="orangered" size="small">保护期</ATag>
                          <ATag v-else color="green" size="small">可释放</ATag>
                        </div>
                      </div>
                      <div v-if="row.block_reason" class="mt-2 text-xs text-[var(--color-text-3)]">{{ row.block_reason }}</div>
                    </template>
                    <template #card-actions="{ row }">
                      <AButton v-if="hasPermission('billing:refund')" size="small" :status="row.need_force ? 'danger' : 'warning'" :disabled="!row.releasable" @click="openRelease(row)">
                        {{ row.need_force ? '强制释放' : '释放' }}
                      </AButton>
                    </template>
                  </ResponsiveTable>
                </ACard>
              </template>
            </ASpin>
          </ATabPane>

          <!-- Tab 4: Transactions -->
          <ATabPane key="transactions" title="交易记录">
            <ACard :bordered="false" title="交易记录">
              <template #extra>
                <ARadioGroup
                  v-if="!isMobile"
                  v-model="txFilterType"
                  type="button"
                  size="small"
                  @change="onTxFilterChange"
                >
                  <ARadio v-for="opt in txFilterOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</ARadio>
                </ARadioGroup>
                <ASelect
                  v-else
                  v-model="txFilterType"
                  size="small"
                  :options="txFilterOptions"
                  placeholder="筛选类型"
                  style="width: 140px"
                  @change="onTxFilterChange"
                />
              </template>
              <ResponsiveTable
                :columns="txColumns"
                :data="txData"
                :loading="walletLoading"
                :stripe="true"
                row-key="id"
                size="small"
                card-title-key="model_name"
                card-badge-key="type"
                :card-fields="[{ key: 'amount', full: true }, { key: 'balance_after' }, { key: 'username' }, { key: 'request_id', full: true }, { key: 'description', full: true }, { key: 'created_at', full: true }]"
              />
              <div class="table-footer">
                <TableStats :total="txPagination.total" />
                <APagination
                  v-model:current="txPagination.current"
                  v-model:page-size="txPagination.pageSize"
                  :total="txPagination.total"
                  :page-size-options="[10, 20, 50]"
                  :show-page-size="!isMobile"
                  :show-jumper="!isMobile"
                  :simple="isMobile"
                  @change="fetchWalletTransactions"
                  @page-size-change="() => { txPagination.current = 1; fetchWalletTransactions() }"
                />
              </div>
            </ACard>
          </ATabPane>
        </ATabs>
      </template>
    </ASpin>

    <!-- Assign Groups Modal -->
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

    <!-- Assign Models Modal -->
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

    <!-- Available Models Preview Modal -->
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

    <!-- Edit Model Config Drawer -->
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
                <template #suffix>$ / 1M</template>
              </AInputNumber>
            </AFormItem>
            <AFormItem label="输出价格">
              <AInputNumber v-model="editModelForm.custom_output_price" :min="0" :precision="4" placeholder="默认" class="w-full">
                <template #suffix>$ / 1M</template>
              </AInputNumber>
            </AFormItem>
            <AFormItem label="缓存读取价格">
              <AInputNumber v-model="editModelForm.custom_cache_read_price" :min="0" :precision="4" placeholder="默认" class="w-full">
                <template #suffix>$ / 1M</template>
              </AInputNumber>
            </AFormItem>
            <AFormItem label="缓存创建价格">
              <AInputNumber v-model="editModelForm.custom_cache_creation_price" :min="0" :precision="4" placeholder="默认" class="w-full">
                <template #suffix>$ / 1M</template>
              </AInputNumber>
            </AFormItem>
          </div>
        </template>

        <!-- 按次计费 -->
        <template v-if="editModelForm.billing_mode === 'per_request'">
          <ADivider margin="8px">按次定价</ADivider>
          <AFormItem label="按次单价">
            <AInputNumber v-model="editModelForm.per_request_price" :min="0" :precision="4" placeholder="每次调用价格" class="w-full">
              <template #suffix>$ / 次</template>
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
                  <template #suffix>$/1M</template>
                </AInputNumber>
              </AFormItem>
              <AFormItem label="输出价格">
                <AInputNumber v-model="tier.output_price" :min="0" :precision="4" class="w-full">
                  <template #suffix>$/1M</template>
                </AInputNumber>
              </AFormItem>
            </div>
            <div class="grid grid-cols-2 gap-x-3 mt-2">
              <AFormItem label="缓存读取价格">
                <AInputNumber v-model="tier.cache_read_price" :min="0" :precision="4" class="w-full">
                  <template #suffix>$/1M</template>
                </AInputNumber>
              </AFormItem>
              <AFormItem label="缓存创建价格">
                <AInputNumber v-model="tier.cache_creation_price" :min="0" :precision="4" class="w-full">
                  <template #suffix>$/1M</template>
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

    <!-- Adjust Balance Modal -->
    <AModal v-model:visible="showRechargeModal" title="调整余额" :width="440" :on-before-ok="handleRecharge" :ok-loading="rechargeLoading">
      <div class="mb-3 text-sm leading-6" style="color: var(--ta-text-secondary)">
        手动调整该租户的美元钱包余额（正数增加、负数扣减），用于运营补偿、余额纠错等场景。
        <br />
        总余额 <span class="money">${{ walletInfo ? parseFloat(walletInfo.balance).toFixed(6) : '0.000000' }}</span>
        ｜ 可用余额 <span class="money">${{ walletInfo ? (walletInfo.balance - walletInfo.frozen_balance).toFixed(6) : '0.000000' }}</span>
        <span style="color: var(--ta-text-tertiary)">（扣减上限）</span>
      </div>
      <AForm :model="rechargeForm" layout="vertical">
        <AFormItem label="调整金额（USD）" required>
          <AInputNumber v-model="rechargeForm.amount" :precision="2" :step="1" placeholder="正数=增加余额，负数=扣减余额" class="w-full">
            <template #prefix>$</template>
          </AInputNumber>
        </AFormItem>
        <AFormItem label="调整说明">
          <ATextarea v-model="rechargeForm.description" placeholder="如：运营补偿 / 余额纠错。建议填写，便于对账与审计" :auto-size="{ minRows: 2, maxRows: 4 }" />
        </AFormItem>
      </AForm>
      <div class="mt-2 text-xs" style="color: var(--ta-text-tertiary)">
        提示：扣减金额不能超过可用余额（不含冻结部分）；如需人民币线下转账到账，请使用「线下入账」
      </div>
    </AModal>

    <!-- Offline Recharge Modal -->
    <AModal v-model:visible="showOfflineRechargeModal" title="线下充值入账" :width="440" :on-before-ok="handleOfflineRecharge" :ok-loading="offlineRechargeLoading">
      <div class="mb-3 text-sm" style="color: var(--ta-text-tertiary)">
        用于线下银行转账入账：输入人民币金额，到账按平台汇率换算为美元，并累计到累计充值（影响租户等级）。
      </div>
      <AForm :model="offlineRechargeForm" layout="vertical">
        <AFormItem label="入账金额（人民币）" required>
          <AInputNumber v-model="offlineRechargeForm.amount" :min="0.01" :precision="2" placeholder="如 100" class="w-full">
            <template #prefix>¥</template>
          </AInputNumber>
        </AFormItem>
        <AFormItem label="转账流水号">
          <AInput v-model="offlineRechargeForm.transaction_no" placeholder="银行转账流水号（选填，用于对账与防重复入账）" />
        </AFormItem>
        <AFormItem label="入账说明" required>
          <ATextarea v-model="offlineRechargeForm.description" placeholder="如：客户名称 / 转账原因" :auto-size="{ minRows: 2, maxRows: 4 }" />
        </AFormItem>
      </AForm>
      <div class="mt-2 text-xs" style="color: var(--ta-text-tertiary)">
        到账金额 = 人民币金额 × 平台汇率，精确到 6 位小数
      </div>
    </AModal>

    <!-- Warning Threshold Modal -->
    <AModal v-model:visible="showThresholdModal" title="设置余额预警阈值" :width="400" :on-before-ok="handleThreshold" :ok-loading="thresholdLoading">
      <div class="mb-3 text-sm" style="color: var(--ta-text-tertiary)">
        当可用余额低于阈值时触发预警通知
      </div>
      <AForm :model="thresholdForm" layout="vertical">
        <AFormItem label="预警阈值">
          <AInputNumber v-model="thresholdForm.threshold" :precision="2" :min="0" placeholder="设为 0 表示不预警" class="w-full">
            <template #prefix>$</template>
          </AInputNumber>
        </AFormItem>
      </AForm>
    </AModal>

    <!-- Release All Frozen Modal -->
    <AModal v-model:visible="showReleaseAllModal" title="一键释放全部冻结" :width="520" :on-before-ok="handleReleaseAll" :ok-loading="releaseAllLoading" :ok-button-props="{ status: 'danger' }" ok-text="确认释放">
      <AAlert type="warning" class="mb-3">
        将一次性释放该租户全部<b>可释放</b>的冻结项（金额从冻结退回可用余额，不改变总余额）。
        任务关联/待结算的冻结自动跳过；保护期内（冻结不足 10 分钟）的冻结默认跳过。
      </AAlert>
      <AForm :model="releaseAllForm" layout="vertical">
        <AFormItem label="释放原因" required>
          <ATextarea v-model="releaseAllForm.reason" placeholder="如：用户工单反馈冻结未释放（工单号）/ 实例故障后遗留" :auto-size="{ minRows: 2, maxRows: 4 }" />
        </AFormItem>
        <AFormItem>
          <ACheckbox v-model="releaseAllForm.force">同时强制释放保护期内的冻结项（对应请求可能仍在进行中，释放后将失去超扣保护）</ACheckbox>
        </AFormItem>
      </AForm>
      <div class="mt-2 text-xs" style="color: var(--ta-text-tertiary)">
        释放为逐笔幂等操作：若某请求恰好同时完成结算，以结算结果为准，不会重复释放
      </div>
    </AModal>

    <!-- Release Frozen Modal -->
    <AModal v-model:visible="showReleaseModal" :title="releaseTarget?.need_force ? '强制释放冻结' : '释放冻结'" :width="460" :on-before-ok="handleRelease" :ok-loading="releaseLoading" :ok-button-props="{ status: releaseTarget?.need_force ? 'danger' : 'warning' }" :ok-text="releaseTarget?.need_force ? '确认强制释放' : '确认释放'">
      <template v-if="releaseTarget">
        <AAlert v-if="releaseTarget.need_force" type="error" class="mb-3">
          该笔冻结仅 {{ formatFrozenAge(releaseTarget.age_seconds) }}，对应请求（长流式/实时会话）可能仍在进行中。释放后该请求将失去超扣保护，并发消费可能导致余额扣成负数。请确认无在途请求后再操作。
        </AAlert>
        <div class="mb-3 text-sm leading-6" style="color: var(--ta-text-secondary)">
          请求ID：<span style="font-family: monospace">{{ releaseTarget.request_id }}</span>
          <br />
          冻结金额：<span style="color: rgb(var(--orange-6)); font-weight: 600">${{ (parseFloat(releaseTarget.amount) || 0).toFixed(6) }}</span>
          ｜ 已冻结 {{ formatFrozenAge(releaseTarget.age_seconds) }}
        </div>
        <AForm :model="releaseForm" layout="vertical">
          <AFormItem label="释放原因" required>
            <ATextarea v-model="releaseForm.reason" placeholder="如：用户工单反馈冻结未释放（工单号）/ 实例故障后遗留" :auto-size="{ minRows: 2, maxRows: 4 }" />
          </AFormItem>
        </AForm>
        <div class="mt-2 text-xs" style="color: var(--ta-text-tertiary)">
          释放为逐笔幂等操作：金额从冻结退回可用余额，不改变总余额；若该请求恰好同时完成结算，以结算结果为准，不会重复释放
        </div>
      </template>
    </AModal>
  </div>
</template>

<style scoped>
.money {
  font-weight: 600;
  color: var(--color-success-6);
}
.table-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--ta-border-light);
}
.table-footer .table-stats {
  margin-bottom: 0;
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
</style>
