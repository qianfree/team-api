<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted, h } from 'vue'
import { Message } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import request from '@/utils/request'
import { hasPermission } from '@/utils/permission'
import { useIsMobile } from '@/composables/useIsMobile'
import { displayCurrency, formatBilling } from '@/composables/useCurrency'

const props = defineProps<{
  tenantId: string
  active: boolean
}>()

const emit = defineEmits<{
  'refresh-detail': []
}>()

const isMobile = useIsMobile()

// 本位币符号：bil 层表单（调整余额/预警阈值）输入前缀跟随本位币，输入值仍为存储原值不折算
const currencySymbol = computed(() => (displayCurrency.value === 'CNY' ? '¥' : '$'))

// 线下充值入账弹窗文案：收款恒为人民币（银行转账），到账按本位币分支说明
const offlineRechargeIntro = computed(() =>
  displayCurrency.value === 'CNY'
    ? '用于线下银行转账入账：输入人民币金额直接入账钱包（本位币为人民币，无需换算），并累计到累计充值（影响租户等级）。'
    : '用于线下银行转账入账：输入人民币金额，到账按平台汇率换算为美元，并累计到累计充值（影响租户等级）。',
)
const offlineRechargeNote = computed(() =>
  displayCurrency.value === 'CNY'
    ? '到账金额 = 输入的人民币金额（本位币直接入账）'
    : '到账金额 = 人民币金额 × 平台汇率，精确到 6 位小数',
)

// === 钱包信息 ===
const walletInfo = ref<any>(null)
const walletLoading = ref(false)

const showRechargeModal = ref(false)
const rechargeForm = reactive({ amount: 0, description: '' })
const rechargeLoading = ref(false)
const showOfflineRechargeModal = ref(false)
const offlineRechargeForm = reactive({ amount: 0, transaction_no: '', description: '' })
const offlineRechargeLoading = ref(false)
const showThresholdModal = ref(false)
const thresholdForm = reactive({ threshold: 0 })
const thresholdLoading = ref(false)

async function fetchWalletInfo() {
  try {
    const res: any = await request.get(`/admin/wallets/${props.tenantId}`)
    walletInfo.value = res.data?.data || null
  } catch {
  }
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
  { title: '请求ID', dataIndex: 'request_id', width: 300, ellipsis: true, tooltip: true },
  {
    title: '模型', dataIndex: 'model_name', width: 160,
    render({ record }) { return record.model_name || '--' },
  },
  {
    title: '冻结金额', dataIndex: 'amount', width: 130,
    render({ record }) {
      return h('span', { style: { color: 'rgb(var(--orange-6))', fontWeight: 600 } }, formatBilling(parseFloat(record.amount) || 0, 6))
    },
  },
  { title: '冻结时间', dataIndex: 'created_at', width: 280 },
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
    const res: any = await request.get(`/admin/wallets/${props.tenantId}/frozen-items`)
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
    const res: any = await request.post(`/admin/wallets/${props.tenantId}/frozen-items/release`, {
      request_id: releaseTarget.value.request_id,
      force: !!releaseTarget.value.need_force,
      reason: releaseForm.reason.trim(),
    })
    const amt = parseFloat(res.data?.data?.released_amount || 0)
    Message.success(`已释放冻结 ${formatBilling(amt, 6)}`)
    done(true)
    await refreshAll()
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
    const res: any = await request.post(`/admin/wallets/${props.tenantId}/frozen-items/release-all`, {
      force: releaseAllForm.force,
      reason: releaseAllForm.reason.trim(),
    })
    const data = res.data?.data || {}
    const amt = parseFloat(data.released_amount || 0)
    const skipped = parseInt(data.skipped_count || 0, 10)
    Message.success(`已释放 ${data.released_count || 0} 笔冻结，合计 ${formatBilling(amt, 6)}${skipped ? `，跳过 ${skipped} 笔` : ''}`)
    if (skipped) {
      Message.warning(`跳过原因：${(data.skipped_reasons || []).join('；')}`)
    }
    done(true)
    await refreshAll()
  } catch {
    return false
  } finally {
    releaseAllLoading.value = false
  }
}

// === 调整余额 ===
function openRecharge() {
  rechargeForm.amount = 0
  rechargeForm.description = ''
  showRechargeModal.value = true
}

async function handleRecharge(done: (closed: boolean) => void) {
  if (rechargeForm.amount === 0) { Message.warning('金额不能为零'); return }
  rechargeLoading.value = true
  try {
    await request.post(`/admin/wallets/${props.tenantId}/adjust`, {
      amount: rechargeForm.amount,
      description: rechargeForm.description || (rechargeForm.amount > 0 ? '管理员增加余额' : '管理员扣减余额'),
    })
    Message.success(rechargeForm.amount > 0 ? '余额已增加' : '余额已扣减')
    done(true)
    await refreshAll()
    // 余额变动需同步父级 detail（页头/信息 Tab 的余额展示）
    emit('refresh-detail')
  } catch {
    return false
  } finally {
    rechargeLoading.value = false
  }
}

// === 线下充值入账 ===
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
    const res: any = await request.post(`/admin/wallets/${props.tenantId}/offline-recharge`, {
      amount: offlineRechargeForm.amount,
      transaction_no: offlineRechargeForm.transaction_no || undefined,
      description: offlineRechargeForm.description,
    })
    const data = res.data?.data || {}
    // credited_usd 为 bil 层本位币金额，按本位币符号展示（汇率信息保留）
    Message.success(`入账成功，到账 ${formatBilling(data.credited_usd, 2)}（汇率 ${data.rate}）`)
    done(true)
    await refreshAll()
    emit('refresh-detail')
  } catch {
    return false
  } finally {
    offlineRechargeLoading.value = false
  }
}

// === 预警阈值 ===
function openThreshold() {
  thresholdForm.threshold = walletInfo.value?.warning_threshold ? parseFloat(walletInfo.value.warning_threshold) : 0
  showThresholdModal.value = true
}

async function handleThreshold(done: (closed: boolean) => void) {
  thresholdLoading.value = true
  try {
    await request.put(`/admin/wallets/${props.tenantId}/warning-threshold`, { threshold: thresholdForm.threshold })
    Message.success('预警阈值已更新')
    done(true)
    await refreshAll()
  } catch {
    return false
  } finally {
    thresholdLoading.value = false
  }
}

// 刷新钱包信息 + 冻结明细（交易流水由 TransactionsTab 自管）
async function refreshAll() {
  walletLoading.value = true
  try {
    await Promise.all([fetchWalletInfo(), fetchFrozenItems()])
  } finally {
    walletLoading.value = false
  }
}

// 首挂拉取一次；之后每次切回该 Tab 都刷新（保持原 onTabChange 语义）
onMounted(refreshAll)
watch(() => props.active, (v) => { if (v) refreshAll() })
</script>

<template>
  <ASpin :loading="walletLoading">
    <template v-if="walletInfo">
      <!-- 钱包信息卡 -->
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
            <span class="money">{{ formatBilling(walletInfo.balance - walletInfo.frozen_balance, 6) }}</span>
          </ADescriptionsItem>
          <ADescriptionsItem label="总余额">
            {{ formatBilling(walletInfo.balance, 6) }}
          </ADescriptionsItem>
          <ADescriptionsItem label="冻结余额">
            <span style="color: rgb(var(--orange-6))">{{ formatBilling(walletInfo.frozen_balance, 6) }}</span>
          </ADescriptionsItem>
          <ADescriptionsItem label="预警阈值">
            {{ walletInfo.warning_threshold > 0 ? formatBilling(walletInfo.warning_threshold, 6) : '关闭' }}
          </ADescriptionsItem>
        </ADescriptions>
      </ACard>

      <!-- 冻结明细（预扣冻结明细，运维逃生舱） -->
      <ACard :bordered="false" class="mb-4" title="冻结明细" :body-style="{ minHeight: '200px' }">
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
        <!-- 冻结笔数多时卡片内滚动，不无限拉伸页面：桌面端走 ATable scroll.y（表头吸顶），移动端卡片列表靠该容器限高滚动 -->
        <div class="max-md:max-h-[480px] max-md:overflow-y-auto">
        <ResponsiveTable
          :columns="frozenColumns"
          :data="frozenItems"
          :loading="frozenLoading"
          :stripe="true"
          row-key="request_id"
          size="small"
          :scroll="{ y: 480 }"
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
        </div>
      </ACard>
    </template>
  </ASpin>

  <!-- 调整余额弹窗 -->
  <AModal v-model:visible="showRechargeModal" title="调整余额" :width="440" :on-before-ok="handleRecharge" :ok-loading="rechargeLoading">
    <div class="mb-3 text-sm leading-6" style="color: var(--ta-text-secondary)">
      手动调整该租户的{{ displayCurrency === 'USD' ? '美元' : '人民币' }}钱包余额（正数增加、负数扣减），用于运营补偿、余额纠错等场景。
      <br />
      总余额 <span class="money">{{ walletInfo ? formatBilling(walletInfo.balance, 6) : formatBilling(0, 6) }}</span>
      ｜ 可用余额 <span class="money">{{ walletInfo ? formatBilling(walletInfo.balance - walletInfo.frozen_balance, 6) : formatBilling(0, 6) }}</span>
      <span style="color: var(--ta-text-tertiary)">（扣减上限）</span>
    </div>
    <AForm :model="rechargeForm" layout="vertical">
      <AFormItem :label="`调整金额（${displayCurrency}）`" required>
        <AInputNumber v-model="rechargeForm.amount" :precision="2" :step="1" placeholder="正数=增加余额，负数=扣减余额" class="w-full">
          <template #prefix>{{ currencySymbol }}</template>
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

  <!-- 线下充值入账弹窗 -->
  <AModal v-model:visible="showOfflineRechargeModal" title="线下充值入账" :width="440" :on-before-ok="handleOfflineRecharge" :ok-loading="offlineRechargeLoading">
    <div class="mb-3 text-sm" style="color: var(--ta-text-tertiary)">
      {{ offlineRechargeIntro }}
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
      {{ offlineRechargeNote }}
    </div>
  </AModal>

  <!-- 预警阈值弹窗 -->
  <AModal v-model:visible="showThresholdModal" title="设置余额预警阈值" :width="400" :on-before-ok="handleThreshold" :ok-loading="thresholdLoading">
    <div class="mb-3 text-sm" style="color: var(--ta-text-tertiary)">
      当可用余额低于阈值时触发预警通知
    </div>
    <AForm :model="thresholdForm" layout="vertical">
      <AFormItem label="预警阈值">
        <AInputNumber v-model="thresholdForm.threshold" :precision="2" :min="0" placeholder="设为 0 表示不预警" class="w-full">
          <template #prefix>{{ currencySymbol }}</template>
        </AInputNumber>
      </AFormItem>
    </AForm>
  </AModal>

  <!-- 一键释放全部冻结弹窗 -->
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

  <!-- 逐笔释放冻结弹窗 -->
  <AModal v-model:visible="showReleaseModal" :title="releaseTarget?.need_force ? '强制释放冻结' : '释放冻结'" :width="460" :on-before-ok="handleRelease" :ok-loading="releaseLoading" :ok-button-props="{ status: releaseTarget?.need_force ? 'danger' : 'warning' }" :ok-text="releaseTarget?.need_force ? '确认强制释放' : '确认释放'">
    <template v-if="releaseTarget">
      <AAlert v-if="releaseTarget.need_force" type="error" class="mb-3">
        该笔冻结仅 {{ formatFrozenAge(releaseTarget.age_seconds) }}，对应请求（长流式/实时会话）可能仍在进行中。释放后该请求将失去超扣保护，并发消费可能导致余额扣成负数。请确认无在途请求后再操作。
      </AAlert>
      <div class="mb-3 text-sm leading-6" style="color: var(--ta-text-secondary)">
        请求ID：<span style="font-family: monospace">{{ releaseTarget.request_id }}</span>
        <br />
        冻结金额：<span style="color: rgb(var(--orange-6)); font-weight: 600">{{ formatBilling(releaseTarget.amount, 6) }}</span>
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
</template>

<style scoped>
@import './common.css';
</style>
