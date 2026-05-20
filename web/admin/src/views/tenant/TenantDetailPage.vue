<script setup lang="ts">
import { ref, reactive, computed, onMounted, h, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Tag, Button, Space, Popconfirm, Message } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const route = useRoute()
const router = useRouter()

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
  max_members: null as number | null,
  max_concurrency: null as number | null,
})

function startEdit() {
  if (!detail.value) return
  editForm.name = detail.value.name || ''
  editForm.max_members = detail.value.max_members || null
  editForm.max_concurrency = detail.value.max_concurrency || null
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

// Load data when switching to models tab
function onTabChange(key: string) {
  if (key === 'models') {
    fetchTenantModels()
    fetchAllModels()
  }
  if (key === 'wallet') {
    fetchWalletDetail()
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
const showThresholdModal = ref(false)
const thresholdForm = reactive({ threshold: 0 })
const thresholdLoading = ref(false)

const txTypeLabel: Record<string, string> = {
  recharge: '充值', consume: '消费', pre_deduct: '预扣', settle: '结算',
  refund: '退款', adjust: '调整', freeze: '冻结', unfreeze: '解冻',
}
const txTypeColor: Record<string, string> = {
  recharge: 'green', consume: 'orangered', adjust: 'arcoblue', refund: 'cyan',
  pre_deduct: 'orangered', settle: 'orange', freeze: 'red', unfreeze: 'purple',
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
    title: '金额', dataIndex: 'amount', width: 120,
    render({ record }) {
      const val = parseFloat(record.amount) || 0
      const color = val >= 0 ? 'rgb(var(--green-6))' : 'rgb(var(--red-6))'
      const prefix = val >= 0 ? '+' : ''
      return h('span', { style: { fontWeight: 600, color } }, `${prefix}${val.toFixed(4)}`)
    },
  },
  {
    title: '变动后余额', dataIndex: 'balance_after', width: 120,
    render({ record }) { return `$${(parseFloat(record.balance_after) || 0).toFixed(2)}` },
  },
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
  await Promise.all([fetchWalletInfo(), fetchWalletTransactions()])
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
      description: rechargeForm.description || (rechargeForm.amount > 0 ? '管理员充值' : '管理员扣减'),
    })
    Message.success(rechargeForm.amount > 0 ? '充值成功' : '扣减成功')
    done(true)
    await fetchWalletDetail()
    await fetchDetail()
  } catch {
    return false
  } finally {
    rechargeLoading.value = false
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
  startEdit()
})
</script>

<template>
  <div class="tenant-detail-page">
    <PageHeader :title="detail ? detail.name : '租户详情'" :description="detail ? `租户代码: ${detail.code}` : ''">
      <template #actions>
        <ASpace>
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
          <!-- Tab 1: Basic Info -->
          <ATabPane key="info" title="基本信息">
            <ACard :bordered="false" class="mb-4" title="租户信息">
              <ADescriptions :column="2" bordered size="medium">
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
                <ADescriptionsItem label="成员数">{{ detail.member_count || 0 }} / {{ detail.max_members }}</ADescriptionsItem>
                <ADescriptionsItem label="钱包余额">
                  <span class="money">${{ parseFloat(detail.wallet_balance || '0').toFixed(2) }}</span>
                </ADescriptionsItem>
                <ADescriptionsItem label="并发上限">{{ detail.max_concurrency || '不限' }}</ADescriptionsItem>
                <ADescriptionsItem label="创建时间">{{ detail.created_at }}</ADescriptionsItem>
                <ADescriptionsItem label="更新时间">{{ detail.updated_at }}</ADescriptionsItem>
              </ADescriptions>
            </ACard>

            <ACard :bordered="false" title="编辑租户">
              <AForm :model="editForm" :auto-label-width="true" layout="inline">
                <AFormItem label="租户名称">
                  <AInput v-model="editForm.name" style="width: 200px" />
                </AFormItem>
                <AFormItem label="最大成员数">
                  <AInputNumber v-model="editForm.max_members" :min="1" style="width: 120px" />
                </AFormItem>
                <AFormItem label="并发上限">
                  <AInputNumber v-model="editForm.max_concurrency" :min="0" placeholder="0=不限" style="width: 120px" />
                </AFormItem>
                <AFormItem>
                  <AButton type="primary" :loading="editLoading" @click="saveEdit">保存</AButton>
                </AFormItem>
              </AForm>
            </ACard>
          </ATabPane>

          <!-- Tab 2: Model Assignment -->
          <ATabPane key="models" title="模型分配">
            <ACard :bordered="false">
              <div class="flex items-center justify-between mb-4">
                <span style="color: var(--ta-text-tertiary)">已分配 {{ modelsData.length }} 个模型</span>
                <AButton type="primary" @click="openAssignModal">分配模型</AButton>
              </div>
              <ATable
                :columns="modelColumns"
                :data="modelsData"
                :loading="modelsLoading"
                :bordered="false"
                :stripe="true"
                :pagination="false"
                row-key="id"
                :scroll="{ x: 1100 }"
              />
            </ACard>
          </ATabPane>

          <!-- Tab 3: Wallet -->
          <ATabPane key="wallet" title="钱包与计费">
            <ASpin :loading="walletLoading">
              <template v-if="walletInfo">
                <!-- Wallet Info Card -->
                <ACard :bordered="false" title="钱包信息" class="mb-4">
                  <template #extra>
                    <ASpace>
                      <AButton type="primary" @click="openRecharge">充值 / 调整</AButton>
                      <AButton @click="openThreshold">预警设置</AButton>
                    </ASpace>
                  </template>
                  <ADescriptions :column="3" bordered size="medium">
                    <ADescriptionsItem label="可用余额">
                      <span class="money">${{ (walletInfo.balance - walletInfo.frozen_balance).toFixed(2) }}</span>
                    </ADescriptionsItem>
                    <ADescriptionsItem label="总余额">
                      ${{ parseFloat(walletInfo.balance).toFixed(2) }}
                    </ADescriptionsItem>
                    <ADescriptionsItem label="冻结余额">
                      <span style="color: rgb(var(--orange-6))">${{ parseFloat(walletInfo.frozen_balance).toFixed(2) }}</span>
                    </ADescriptionsItem>
                    <ADescriptionsItem label="预警阈值">
                      {{ walletInfo.warning_threshold != null ? `$${parseFloat(walletInfo.warning_threshold).toFixed(2)}` : '未设置' }}
                    </ADescriptionsItem>
                  </ADescriptions>
                </ACard>

                <!-- Transaction History -->
                <ACard :bordered="false" title="交易记录">
                  <div class="mb-4">
                    <ASpace>
                      <ASelect v-model="txFilterType" placeholder="全部类型" allow-clear style="width: 140px" @change="() => { txPagination.current = 1; fetchWalletDetail() }">
                        <AOption value="recharge">充值</AOption>
                        <AOption value="consume">消费</AOption>
                        <AOption value="adjust">调整</AOption>
                        <AOption value="pre_deduct">预扣</AOption>
                        <AOption value="settle">结算</AOption>
                        <AOption value="refund">退款</AOption>
                        <AOption value="freeze">冻结</AOption>
                        <AOption value="unfreeze">解冻</AOption>
                      </ASelect>
                    </ASpace>
                  </div>
                  <ATable :columns="txColumns" :data="txData" :loading="walletLoading" :bordered="false" :stripe="true" :pagination="false" row-key="id" size="small" />
                  <div class="table-footer">
                    <APagination v-model:current="txPagination.current" v-model:page-size="txPagination.pageSize" :total="txPagination.total" simple @change="fetchWalletDetail" />
                  </div>
                </ACard>
              </template>
            </ASpin>
          </ATabPane>
        </ATabs>
      </template>
    </ASpin>

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

    <!-- Recharge / Adjust Balance Modal -->
    <AModal v-model:visible="showRechargeModal" title="充值 / 调整余额" :width="420" :on-before-ok="handleRecharge" :ok-loading="rechargeLoading">
      <div class="mb-3 text-sm" style="color: var(--ta-text-tertiary)">
        当前余额：${{ walletInfo ? parseFloat(walletInfo.balance).toFixed(2) : '0.00' }}
      </div>
      <AForm :model="rechargeForm" layout="vertical">
        <AFormItem label="金额" required>
          <AInputNumber v-model="rechargeForm.amount" :precision="2" :step="1" placeholder="正数=充值，负数=扣减" class="w-full">
            <template #prefix>$</template>
          </AInputNumber>
        </AFormItem>
        <AFormItem label="描述">
          <ATextarea v-model="rechargeForm.description" placeholder="操作原因（选填，默认自动生成）" :auto-size="{ minRows: 2, maxRows: 4 }" />
        </AFormItem>
      </AForm>
      <div class="mt-2 text-xs" style="color: var(--ta-text-tertiary)">
        提示：输入正数充值余额，输入负数扣减余额
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
  </div>
</template>

<style scoped>
.money {
  font-weight: 600;
  color: var(--color-success-6);
}
.table-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--ta-border-light);
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
