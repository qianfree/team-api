<script setup lang="ts">
import { ref, reactive, watch, onMounted, h } from 'vue'
import { Tag } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import TableStats from '@/components/TableStats.vue'
import request from '@/utils/request'
import { useIsMobile } from '@/composables/useIsMobile'

const props = defineProps<{
  tenantId: string
  active: boolean
}>()

const isMobile = useIsMobile()

const txLoading = ref(false)
const txData = ref<any[]>([])
const txPagination = reactive({
  current: 1, pageSize: 20, total: 0,
})
const txFilterType = ref('')

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
  { title: '时间', dataIndex: 'created_at', width: 170 },

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
  { title: '请求ID', dataIndex: 'request_id', width: 200, ellipsis: true },
  { title: '模型', dataIndex: 'model_name', width: 200 },
  { title: '描述', dataIndex: 'description', ellipsis: true },
  { title: 'ID', dataIndex: 'id', width: 70 },
]

async function fetchWalletTransactions() {
  txLoading.value = true
  try {
    const res: any = await request.get(`/admin/wallets/${props.tenantId}/transactions`, {
      params: { page: txPagination.current, page_size: txPagination.pageSize, type: txFilterType.value || undefined },
    })
    const raw = res.data?.data
    txData.value = raw?.list || []
    txPagination.total = Number(raw?.total) || 0
  } catch {
  } finally {
    txLoading.value = false
  }
}

// 首挂拉取一次；之后每次切回该 Tab 都刷新（保持原 onTabChange 语义）
onMounted(fetchWalletTransactions)
watch(() => props.active, (v) => { if (v) fetchWalletTransactions() })
</script>

<template>
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
      :loading="txLoading"
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
</template>

<style scoped>
@import './common.css';
</style>
