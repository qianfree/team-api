<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { Tag } from '@arco-design/web-vue'
import { IconInfoCircle } from '@arco-design/web-vue/es/icon'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import TableStats from '@/components/TableStats.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import request from '@/utils/request'

const loading = ref(false)
const data = ref<any[]>([])
// 默认只查当天（配合后端 Redis 当日实时热桶，打开即可看到最新的模型性能）
const dateRange = ref<[string, string] | null>(defaultRange(0))
// 指标说明弹窗
const showMetricsGuide = ref(false)

// 渠道/模型可选筛选：选定渠道后按渠道查看其下各模型性能；清空下拉 = 全部
const filterChannelId = ref<number | undefined>(undefined)
const filterModelName = ref<string | undefined>(undefined)
const channelOptions = ref<{ label: string; value: number }[]>([])
const modelOptions = ref<{ label: string; value: string }[]>([])

// 渠道下拉数据：渠道量级为几十，一次性拉全量（page_size 放大即可）
async function fetchChannelOptions() {
  try {
    const res: any = await request.get('/admin/channels', { params: { page: 1, page_size: 200 } })
    channelOptions.value = (res.data?.data?.list || res.data?.list || []).map((c: any) => ({
      label: c.name ? `${c.name} (#${c.id})` : `#${c.id}`,
      value: c.id,
    }))
  } catch {
    channelOptions.value = []
  }
}

// 模型下拉数据：专用不分页接口 /admin/models/options（model_id 与统计口径 model_name 同名）
async function fetchModelOptions() {
  try {
    const res: any = await request.get('/admin/models/options')
    modelOptions.value = (res.data?.data?.list || res.data?.list || []).map((m: any) => ({
      label: m.model_name ? `${m.model_name} (${m.model_id})` : m.model_id,
      value: m.model_id,
    }))
  } catch {
    modelOptions.value = []
  }
}

// 页头描述：追加当前筛选提示，避免「跨渠道」文案与实际筛选状态不符
const pageDescription = computed(() => {
  const base =
    '各模型跨渠道的成功率 / 延迟 / 吞吐 / 缓存命中（历史数据源自 bil_usage_daily 每日聚合，当天实时统计；延迟为总延迟/总请求数均值。Token 命中率对 OpenAI 原生渠道为保守值）'
  const parts: string[] = []
  if (filterChannelId.value) {
    const ch = channelOptions.value.find((c) => c.value === filterChannelId.value)
    parts.push(`渠道：${ch ? ch.label : `#${filterChannelId.value}`}`)
  }
  if (filterModelName.value) parts.push(`模型：${filterModelName.value}`)
  return parts.length ? `${base}。当前筛选 — ${parts.join('，')}` : base
})

function toDateStr(d: Date): string {
  const m = `${d.getMonth() + 1}`.padStart(2, '0')
  const day = `${d.getDate()}`.padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

function defaultRange(days: number): [string, string] {
  const end = new Date()
  const start = new Date()
  start.setDate(start.getDate() - days)
  return [toDateStr(start), toDateStr(end)]
}

// 快捷时间范围：start/end 为相对今天的偏移天数（0=今天，-1=昨天...）
const QUICK_RANGES = [
  { label: '今天', start: 0, end: 0 },
  { label: '昨天', start: -1, end: -1 },
  { label: '最近3天', start: -2, end: 0 },
  { label: '最近一周', start: -6, end: 0 },
  { label: '最近一月', start: -29, end: 0 },
]

function dateOffsetStr(offset: number): string {
  const d = new Date()
  d.setDate(d.getDate() + offset)
  return toDateStr(d)
}

function applyQuickRange(q: { start: number; end: number }) {
  dateRange.value = [dateOffsetStr(q.start), dateOffsetStr(q.end)]
  fetchData()
}

// 当前选择的日期范围与某快捷范围一致时将其按钮高亮（primary）
function isActive(q: { start: number; end: number }): boolean {
  return (
    !!dateRange.value &&
    dateRange.value[0] === dateOffsetStr(q.start) &&
    dateRange.value[1] === dateOffsetStr(q.end)
  )
}

function fmtNum(v: any): number {
  return typeof v === 'number' ? v : 0
}

// 成功率分级标签：excellent≥99 / good≥95 / warning≥90 / critical<90
const gradeColor: Record<string, string> = {
  excellent: 'green',
  good: 'arcoblue',
  warning: 'orange',
  critical: 'red',
}

const columns: TableColumnData[] = [
  { title: '模型', dataIndex: 'model_name', ellipsis: true, tooltip: true, width: 220, sortable: { sortDirections: ['ascend', 'descend'] } },
  {
    title: '请求数',
    dataIndex: 'request_count',
    width: 110,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => fmtNum(record.request_count).toLocaleString(),
  },
  {
    title: '成功率',
    dataIndex: 'success_rate',
    width: 120,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) =>
      h(Tag, { color: gradeColor[record.grade] || 'blue', size: 'small' }, () => `${fmtNum(record.success_rate).toFixed(2)}%`),
  },
  {
    title: '平均延迟',
    dataIndex: 'avg_latency_ms',
    width: 120,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => `${fmtNum(record.avg_latency_ms).toFixed(0)} ms`,
  },
  {
    title: '平均首Token',
    dataIndex: 'avg_first_token_ms',
    width: 130,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => `${fmtNum(record.avg_first_token_ms).toFixed(0)} ms`,
  },
  {
    title: '吞吐 TPS',
    dataIndex: 'tps',
    width: 110,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => `${fmtNum(record.tps).toFixed(1)} t/s`,
  },
  {
    title: '缓存命中',
    dataIndex: 'cache_read_tokens',
    width: 170,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => {
      // 无缓存活动（读/写均为 0）显示 —，embedding/图像类模型不参与缓存统计
      if (fmtNum(record.cache_read_tokens) === 0 && fmtNum(record.cache_creation_tokens) === 0) return '—'
      return `${fmtNum(record.cache_read_tokens).toLocaleString()} (${fmtNum(record.cache_hit_rate).toFixed(1)}%)`
    },
  },
  {
    title: '请求缓存命中率',
    dataIndex: 'cache_hit_request_rate',
    width: 140,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => {
      if (fmtNum(record.cache_read_tokens) === 0 && fmtNum(record.cache_creation_tokens) === 0) return '—'
      return `${fmtNum(record.cache_hit_request_rate).toFixed(1)}%`
    },
  },
  {
    title: '总 Token',
    dataIndex: 'total_tokens',
    width: 130,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => fmtNum(record.total_tokens).toLocaleString(),
  },
  {
    title: '总成本 (USD)',
    dataIndex: 'total_cost',
    width: 140,
    align: 'right',
    sortable: { sortDirections: ['ascend', 'descend'] },
    render: ({ record }: any) => `$${fmtNum(record.total_cost).toFixed(6)}`,
  },
]

// 成功率分级（与后端 gradeSuccessRate 同阈值）：excellent≥99 / good≥95 / warning≥90 / critical<90
function rateGrade(rate: number): string {
  if (rate >= 99) return 'excellent'
  if (rate >= 95) return 'good'
  if (rate >= 90) return 'warning'
  return 'critical'
}

// 概览卡颜色：按成功率分级映射（红色仅用于 critical）
const gradeCardClass: Record<string, string> = {
  excellent: 'metric-card--green',
  good: 'metric-card--green',
  warning: 'metric-card--orange',
  critical: 'metric-card--red',
}

// 顶部概览统计：从当前列表聚合（无额外请求）
const stats = computed(() => {
  let totalRequests = 0
  let totalSuccess = 0
  let totalCost = 0
  let totalCacheHitReq = 0
  let totalCacheTokens = 0
  for (const item of data.value) {
    totalRequests += fmtNum(item.request_count)
    totalSuccess += fmtNum(item.success_count)
    totalCost += fmtNum(item.total_cost)
    totalCacheHitReq += fmtNum(item.cache_hit_request_count)
    totalCacheTokens += fmtNum(item.cache_read_tokens) + fmtNum(item.cache_creation_tokens)
  }
  return {
    modelCount: data.value.length,
    totalRequests,
    overallRate: totalRequests > 0 ? (totalSuccess / totalRequests) * 100 : 0,
    totalCost,
    // 全局请求缓存命中率（无任何缓存活动时为 null，页面显示 —）
    cacheHitReqRate: totalCacheTokens > 0 && totalRequests > 0 ? (totalCacheHitReq / totalRequests) * 100 : null,
  }
})

async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, any> = {}
    if (dateRange.value && dateRange.value.length === 2) {
      params.start_date = dateRange.value[0]
      params.end_date = dateRange.value[1]
    }
    // 可选筛选：清空下拉（undefined）即不传参 = 全部
    if (filterChannelId.value) params.channel_id = filterChannelId.value
    if (filterModelName.value) params.model_name = filterModelName.value
    const res = await request.get('/admin/monitor/model-performance', { params })
    data.value = res.data?.data?.data?.list || []
  } catch {
    // 错误由 Axios 拦截器统一提示
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
  // 下拉数据与列表并行加载，失败仅选项为空，不影响主列表
  fetchChannelOptions()
  fetchModelOptions()
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="模型性能" :description="pageDescription">
      <template #actions>
        <!-- 渠道/模型筛选：清空 = 全部；选定渠道后当天数据由后端从请求明细实时聚合 -->
        <a-select
          v-model="filterChannelId"
          :options="channelOptions"
          placeholder="全部渠道"
          allow-clear
          allow-search
          style="width: 190px"
          @change="fetchData"
        />
        <a-select
          v-model="filterModelName"
          :options="modelOptions"
          placeholder="全部模型"
          allow-clear
          allow-search
          style="width: 250px"
          @change="fetchData"
        />
        <a-button
          size="small"
          v-for="q in QUICK_RANGES"
          :key="q.label"
          :type="isActive(q) ? 'primary' : undefined"
          @click="applyQuickRange(q)"
        >
          {{ q.label }}
        </a-button>
        <a-range-picker v-model="dateRange" value-format="YYYY-MM-DD" style="width: 260px" @change="fetchData" />
        <a-button size="small" :loading="loading" @click="fetchData">刷新</a-button>
      </template>
    </PageHeader>

    <!-- 指标说明入口（点击弹窗） -->
    <div class="metrics-hint" @click="showMetricsGuide = true">
      <IconInfoCircle /> 了解各指标的含义与计算公式 →
    </div>

    <!-- 指标说明弹窗 -->
    <a-modal v-model:visible="showMetricsGuide" title="模型性能指标说明" :width="720" :footer="false">
      <div class="metrics-guide">
        <p class="guide-intro">
          以下指标按所选时间区间逐模型聚合：历史数据（昨天及以前）源自 <code>bil_usage_daily</code> 每日聚合，
          当天数据来自 Redis 实时热桶（小时粒度）；选择渠道筛选后，当天数据改为从请求明细（<code>bil_usage_logs</code>）实时聚合。
          两段口径一致、合并计算。顶部概览卡为当前筛选范围的汇总。
        </p>

        <div class="guide-group">基础指标</div>
        <div class="metric-item">
          <div class="metric-name">请求数</div>
          <div class="metric-body">
            <p>区间内该模型的总请求数（含失败请求，重试成功按最终一次成功计）。</p>
            <p><code>请求数 = Σ request_count</code></p>
          </div>
        </div>
        <div class="metric-item">
          <div class="metric-name">成功率</div>
          <div class="metric-body">
            <p>成功请求占总请求的比例。标签按阈值分级：≥99 优秀（绿）/ ≥95 良好（蓝）/ ≥90 警告（橙）/ 其余严重（红）。</p>
            <p><code>成功率 = Σ 成功请求数 ÷ Σ 总请求数 × 100%</code></p>
          </div>
        </div>
        <div class="metric-item">
          <div class="metric-name">总 Token</div>
          <div class="metric-body">
            <p>输入与输出 token 之和（不含缓存写入部分）。</p>
            <p><code>总 Token = Σ input_tokens + Σ output_tokens</code></p>
          </div>
        </div>
        <div class="metric-item">
          <div class="metric-name">总成本</div>
          <div class="metric-body">
            <p>区间内该模型实际计费成本合计（USD），已含输入、输出与缓存各项费用。</p>
            <p><code>总成本 = Σ total_cost</code></p>
          </div>
        </div>

        <div class="guide-group">延迟与吞吐</div>
        <div class="metric-item">
          <div class="metric-name">平均延迟</div>
          <div class="metric-body">
            <p>请求从发起到响应完成的全程耗时均值（毫秒），流式请求计至最后一个分片。</p>
            <p><code>平均延迟 = Σ 请求延迟 ÷ Σ 总请求数</code></p>
          </div>
        </div>
        <div class="metric-item">
          <div class="metric-name">平均首Token</div>
          <div class="metric-body">
            <p>流式请求从发出到收到首个 token 的耗时均值（毫秒）。非流式 / 无首 token 的请求按 0 计入分母，会拉低该均值。</p>
            <p><code>平均首Token = Σ 首 token 延迟 ÷ Σ 总请求数</code></p>
          </div>
        </div>
        <div class="metric-item">
          <div class="metric-name">吞吐 TPS</div>
          <div class="metric-body">
            <p>平均每秒输出的 token 数，衡量模型生成速度。</p>
            <p><code>TPS = Σ 输出 token ÷ (Σ 请求延迟 ÷ 1000)</code></p>
          </div>
        </div>

        <div class="guide-group">缓存指标</div>
        <div class="metric-item">
          <div class="metric-name">缓存命中</div>
          <div class="metric-body">
            <p>
              从上游提示词缓存读取的 token 数，括号内为 Token 级命中率。某次请求的缓存读取 token &gt; 0 即视为命中；
              无任何缓存读写活动的模型（如 embedding / 图像类）显示 —。
            </p>
            <p><code>缓存命中 Token = Σ cache_read_tokens</code></p>
          </div>
        </div>
        <div class="metric-item">
          <div class="metric-name">缓存命中率<br />（Token 级）</div>
          <div class="metric-body">
            <p>
              缓存命中 token 占总输入 token 的比例，分母为归一化总输入（未命中输入 + 缓存写入 + 缓存读取）。
              注意：OpenAI 原生渠道的 input_tokens 已包含缓存部分，该口径下命中率为保守值（偏低），Claude 渠道精确。
            </p>
            <p><code>缓存命中率 = Σ cache_read_tokens ÷ (Σ input_tokens + Σ cache_creation_tokens + Σ cache_read_tokens) × 100%</code></p>
          </div>
        </div>
        <div class="metric-item">
          <div class="metric-name">请求缓存命中率</div>
          <div class="metric-body">
            <p>
              命中缓存的请求数占总请求数的比例。仅写入缓存（首次建立缓存、未读取）不计命中。
              与 Token 级命中率互补：本指标高说明缓存覆盖面广，Token 级高说明单次请求复用的前缀长。
            </p>
            <p><code>请求缓存命中率 = Σ 命中缓存的请求数 ÷ Σ 总请求数 × 100%</code></p>
          </div>
        </div>
      </div>
    </a-modal>

    <div class="stats-row">
      <div class="metric-card metric-card--blue">
        <div class="metric-card__label">模型数</div>
        <div class="metric-card__value">{{ stats.modelCount }}</div>
      </div>
      <div class="metric-card metric-card--purple">
        <div class="metric-card__label">总请求数</div>
        <div class="metric-card__value">{{ stats.totalRequests.toLocaleString() }}</div>
      </div>
      <div class="metric-card" :class="stats.totalRequests > 0 ? gradeCardClass[rateGrade(stats.overallRate)] : ''">
        <div class="metric-card__label">整体成功率</div>
        <div class="metric-card__value">{{ stats.overallRate.toFixed(2) }}%</div>
      </div>
      <div class="metric-card metric-card--orange">
        <div class="metric-card__label">总成本 (USD)</div>
        <div class="metric-card__value">${{ stats.totalCost.toFixed(6) }}</div>
      </div>
      <div class="metric-card metric-card--green">
        <div class="metric-card__label">请求缓存命中率</div>
        <div class="metric-card__value">{{ stats.cacheHitReqRate === null ? '—' : `${stats.cacheHitReqRate.toFixed(2)}%` }}</div>
      </div>
    </div>

    <a-card :bordered="false">
      <ResponsiveTable
        :columns="columns"
        :data="data"
        :loading="loading"
        row-key="model_name"
        :scroll="{ x: 1510 }"
        size="large"
        stripe
        :border="{ wrapper: true, headerCell: true }"
        card-title-key="model_name"
        card-subtitle-key="request_count"
        card-badge-key="success_rate"
        :card-fields="['avg_latency_ms', 'avg_first_token_ms', 'tps', 'total_tokens', 'total_cost']"
      >
        <template #empty>
          <a-empty description="所选区间暂无模型性能数据" />
        </template>
      </ResponsiveTable>
      <div class="table-footer">
        <TableStats :total="data.length" />
      </div>
    </a-card>
  </div>
</template>

<style scoped>
/* 指标说明入口（复用渠道页 scheduling-hint 风格） */
.metrics-hint {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 16px;
  font-size: 13px;
  color: var(--color-text-3);
  cursor: pointer;
  transition: color 0.2s;
  user-select: none;
}
.metrics-hint:hover {
  color: rgb(var(--arcoblue-6));
}

/* 指标说明弹窗 */
.metrics-guide {
  color: var(--color-text-1);
  font-size: 14px;
  line-height: 1.7;
}
.guide-intro {
  margin: 0 0 14px;
  color: var(--color-text-2);
}
.guide-group {
  margin: 18px 0 10px;
  padding-bottom: 6px;
  font-weight: 600;
  border-bottom: 1px solid var(--color-border);
}
.metric-item {
  display: grid;
  grid-template-columns: 110px 1fr;
  gap: 4px 14px;
  padding: 6px 0;
}
.metric-name {
  font-weight: 600;
  color: var(--color-text-2);
}
.metric-body p {
  margin: 0;
  color: var(--color-text-2);
}
.metric-body p + p {
  margin-top: 2px;
}
.metrics-guide code {
  padding: 1px 4px;
  border-radius: 3px;
  color: rgb(var(--arcoblue-6));
  background: var(--color-primary-light-1);
  word-break: break-all;
}
@media (max-width: 600px) {
  .metric-item {
    grid-template-columns: 1fr;
  }
}

/* 顶部概览卡片：与实时监控页 metric-card 风格保持一致（左色条 + 渐变底） */
.stats-row {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}
.metric-card {
  background: var(--color-bg-2);
  border: 1px solid var(--color-border);
  border-radius: var(--border-radius-medium);
  padding: 18px 20px;
  border-left: 4px solid transparent;
}
.metric-card--blue {
  border-left-color: #165dff;
  background: linear-gradient(135deg, rgba(22, 93, 255, 0.05), var(--color-bg-2) 70%);
}
.metric-card--purple {
  border-left-color: #722ed1;
  background: linear-gradient(135deg, rgba(114, 46, 209, 0.05), var(--color-bg-2) 70%);
}
.metric-card--green {
  border-left-color: #00b42a;
  background: linear-gradient(135deg, rgba(0, 180, 42, 0.05), var(--color-bg-2) 70%);
}
.metric-card--orange {
  border-left-color: #ff7d00;
  background: linear-gradient(135deg, rgba(255, 125, 0, 0.05), var(--color-bg-2) 70%);
}
.metric-card--red {
  border-left-color: #f53f3f;
  background: linear-gradient(135deg, rgba(245, 63, 63, 0.05), var(--color-bg-2) 70%);
}
.metric-card__label {
  font-size: 13px;
  color: var(--color-text-3);
  margin-bottom: 8px;
}
.metric-card__value {
  font-size: 26px;
  font-weight: 600;
  color: var(--color-text-1);
  font-variant-numeric: tabular-nums;
}
@media (max-width: 768px) {
  .stats-row {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
