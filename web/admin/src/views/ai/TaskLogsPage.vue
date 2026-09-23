<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted, onUnmounted, h } from 'vue'
import { useRoute } from 'vue-router'
import {
  Tag, Button, Space, Popconfirm, Message,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import TableStats from '@/components/TableStats.vue'
import request from '@/utils/request'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import { useDateRange } from '@/composables/useDateRange'
import { formatBilling } from '@/composables/useCurrency'

const { defaultEnd, defaultTodayRange, quickDateRanges } = useDateRange()

const loading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50],
})

const filterStatus = ref<string | null>(null)
const filterPlatform = ref<string | null>(null)
const filterTaskId = ref('')
const filterDateRange = ref<string[]>(defaultTodayRange())
const filterModel = ref('')
const filterTenantId = ref<number | undefined>(undefined)
const filterUserId = ref<number | undefined>(undefined)

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '未开始', value: 'NOT_START' },
  { label: '已提交', value: 'SUBMITTED' },
  { label: '进行中', value: 'IN_PROGRESS' },
  { label: '成功', value: 'SUCCESS' },
  { label: '失败', value: 'FAILURE' },
]

const platformOptions = [
  { label: '全部平台', value: '' },
  { label: 'Sora', value: 'sora' },
  { label: 'Kling', value: 'kling' },
  { label: 'Midjourney', value: 'midjourney' },
  { label: 'Suno', value: 'suno' },
  { label: '火山引擎', value: 'volcengine' },
  { label: '阿里', value: 'ali' },
  { label: 'MiniMax', value: 'minimax' },
]

const statusTagColor: Record<string, string | undefined> = {
  NOT_START: 'gray',
  SUBMITTED: 'arcoblue',
  IN_PROGRESS: 'arcoblue',
  SUCCESS: 'green',
  FAILURE: 'red',
}

const statusTagLabel: Record<string, string> = {
  NOT_START: '未开始',
  SUBMITTED: '已提交',
  IN_PROGRESS: '进行中',
  SUCCESS: '成功',
  FAILURE: '失败',
}

const platformLabel: Record<string, string> = {
  sora: 'Sora',
  kling: 'Kling',
  midjourney: 'Midjourney',
  suno: 'Suno',
  volcengine: '火山引擎',
  ali: '阿里',
  minimax: 'MiniMax',
}

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '任务ID', dataIndex: 'public_task_id', width: 160, ellipsis: true, tooltip: true },
  {
    title: '平台',
    dataIndex: 'platform',
    width: 110,
    render({ record }) {
      return platformLabel[record.platform] || record.platform
    },
  },
  {
    title: '动作',
    dataIndex: 'action',
    width: 120,
    ellipsis: true,
    tooltip: true,
  },
  {
    title: '状态',
    dataIndex: 'status',
    width: 100,
    render({ record }) {
      return h(Tag, {
        color: statusTagColor[record.status],
        size: 'small',
      }, () => statusTagLabel[record.status] || record.status)
    },
  },
  {
    title: '进度',
    dataIndex: 'progress',
    width: 80,
  },
  {
    title: '模型',
    dataIndex: 'model_name',
    width: 150,
    ellipsis: true,
    tooltip: true,
  },
  {
    title: '费用',
    dataIndex: 'actual_cost',
    width: 100,
    render({ record }) {
      if (record.billing_settled && record.actual_cost > 0) {
        return formatBilling(record.actual_cost, 6)
      }
      if (record.pre_deduct_amount > 0) {
        return `${formatBilling(record.pre_deduct_amount, 6)} (预扣)`
      }
      return '-'
    },
  },
  { title: '提交时间', dataIndex: 'submit_time', width: 170 },
  { title: '完成时间', dataIndex: 'finish_time', width: 170 },
  {
    title: '操作',
    dataIndex: 'actions',
    width: 120,
    fixed: 'right',
    render({ record }) {
      const buttons: any[] = [
        h(Button, { size: 'small', type: 'text', onClick: () => openDetail(record) }, () => '详情'),
      ]
      if (record.status === 'NOT_START' || record.status === 'SUBMITTED' || record.status === 'IN_PROGRESS') {
        buttons.push(
          h(Popconfirm, {
            content: '确定取消该任务？',
            onOk: () => cancelTask(record),
          }, () => h(Button, { size: 'small', type: 'text', status: 'warning' }, () => '取消')),
        )
      }
      return h(Space, { size: 0 }, () => buttons)
    },
  },
]

async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: pagination.current,
      page_size: pagination.pageSize,
    }
    if (filterStatus.value) params.status = filterStatus.value
    if (filterPlatform.value) params.platform = filterPlatform.value
    if (filterTaskId.value) params.public_task_id = filterTaskId.value
    if (filterDateRange.value && filterDateRange.value.length === 2) {
      params.start_date = filterDateRange.value[0]
      // 截止时间为默认「现在」时不传 end_date（后端按「到现在」实时处理），仅手动选择后才显式下发
      if (filterDateRange.value[1] && filterDateRange.value[1] !== defaultEnd) {
        params.end_date = filterDateRange.value[1]
      }
    }
    if (filterModel.value) params.model_name = filterModel.value
    if (filterTenantId.value) params.tenant_id = filterTenantId.value
    if (filterUserId.value) params.user_id = filterUserId.value
    const res: any = await request.get('/admin/tasks', { params })
    const raw = res.data?.data
    data.value = (raw?.list || []).filter(Boolean)
    pagination.total = Number(raw?.total) || 0
  } catch {
    data.value = []
    pagination.total = 0
  } finally {
    loading.value = false
  }
}

function handleFilter() {
  pagination.current = 1
  fetchData()
}

function resetFilter() {
  filterStatus.value = null
  filterPlatform.value = null
  filterTaskId.value = ''
  filterDateRange.value = defaultTodayRange()
  filterModel.value = ''
  filterTenantId.value = undefined
  filterUserId.value = undefined
  pagination.current = 1
  fetchData()
}

// 刷新：清空所有筛选条件，仅按当天起始时间查询最新记录（截止留空 = 到现在）
function handleRefresh() {
  resetFilter()
}

async function cancelTask(row: any) {
  try {
    await request.post(`/admin/tasks/${row.id}/cancel`)
    Message.success('任务已取消')
    fetchData()
  } catch {
    // error handled by interceptor
  }
}

// === Detail Drawer ===
const showDetail = ref(false)
const detailLoading = ref(false)
const detailData = ref<any>(null)

// 进行中任务在抽屉内按秒刷新「已运行」时长
const nowTs = ref(Date.now())
let heroTicker: ReturnType<typeof setInterval> | undefined

function startHeroTicker() {
  stopHeroTicker()
  heroTicker = setInterval(() => { nowTs.value = Date.now() }, 1000)
}

function stopHeroTicker() {
  if (heroTicker) {
    clearInterval(heroTicker)
    heroTicker = undefined
  }
}

async function openDetail(row: any) {
  detailData.value = row
  showDetail.value = true
  await reloadDetail(row.id)
}

async function reloadDetail(id?: number) {
  const taskId = id ?? detailData.value?.id
  if (!taskId) return
  detailLoading.value = true
  try {
    const res: any = await request.get(`/admin/tasks/${taskId}`)
    const raw = res.data?.data
    if (raw?.task) {
      detailData.value = raw.task
    }
  } catch {
    // error handled by interceptor
  } finally {
    detailLoading.value = false
  }
}

// 状态语义色：抬头区状态点、进度条与时间线节点共用（与列表 Tag 的色彩语义一致）
const statusVisual: Record<string, { label: string; dot: string; bar: string }> = {
  NOT_START: { label: '未开始', dot: 'var(--color-text-4)', bar: 'var(--color-text-4)' },
  SUBMITTED: { label: '已提交', dot: 'rgb(var(--arcoblue-6))', bar: 'rgb(var(--arcoblue-6))' },
  IN_PROGRESS: { label: '进行中', dot: 'rgb(var(--arcoblue-6))', bar: 'rgb(var(--arcoblue-6))' },
  SUCCESS: { label: '成功', dot: 'var(--ta-success)', bar: 'var(--ta-success)' },
  FAILURE: { label: '失败', dot: 'var(--ta-danger)', bar: 'var(--ta-danger)' },
}

// 平台 → 任务类型，用于抬头区类型标签（覆盖范围与 platformOptions 一致）
const platformKind: Record<string, string> = {
  sora: '视频',
  kling: '视频',
  volcengine: '视频',
  ali: '视频',
  minimax: '视频',
  midjourney: '图片',
  suno: '音乐',
}

const detailVisual = computed(() => {
  const status = String(detailData.value?.status || '')
  return statusVisual[status] || { label: status || '-', dot: 'var(--color-text-4)', bar: 'var(--color-text-4)' }
})

// 进度条比值：Arco ProgressLine 的 percent 是 0~1 小数（内部 `width: percent * 100%`），
// 本仓库其余用法（AdminLayout/RealtimeMonitorPage/MonitorPage）也都传比值；传 0~100 会被
// .arco-progress-line-bar 的 max-width:100% 夹成满格，出现「失败任务进度条画满整条」的假象
const progressRatio = computed(() => {
  if (detailData.value?.status === 'SUCCESS') return 1
  const n = parseInt(String(detailData.value?.progress || '').replace('%', ''), 10)
  if (Number.isNaN(n)) return 0
  return Math.min(100, Math.max(0, n)) / 100
})

// 后端时间为 Asia/Shanghai 墙钟（YYYY-MM-DD HH:mm:ss），此处只用来算时间差，不做时区换算；
// 把 - 换成 / 以兼容 Safari 的 Date 解析
function parseTime(v?: string): number | null {
  if (!v) return null
  const t = new Date(String(v).replace(/-/g, '/')).getTime()
  return Number.isNaN(t) ? null : t
}

function diffMs(start?: string, end?: string): number | null {
  const a = parseTime(start)
  const b = parseTime(end)
  if (a === null || b === null) return null
  return b >= a ? b - a : null
}

function formatDuration(ms: number): string {
  const s = Math.floor(ms / 1000)
  if (s < 60) return `${s} 秒`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m} 分 ${s % 60} 秒`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h} 小时 ${m % 60} 分`
  return `${Math.floor(h / 24)} 天 ${h % 24} 小时`
}

// 终态显示总耗时，进行中显示至今已运行时长
const runningOrTotalText = computed(() => {
  const d = detailData.value || {}
  if (d.status === 'SUCCESS' || d.status === 'FAILURE') {
    const total = diffMs(d.created_at, d.finish_time)
    return total === null ? '' : `总耗时 ${formatDuration(total)}`
  }
  const since = parseTime(d.start_time) ?? parseTime(d.submit_time) ?? parseTime(d.created_at)
  if (since === null) return ''
  return `已运行 ${formatDuration(Math.max(0, nowTs.value - since))}`
})

const timelineNodes = computed(() => {
  const d = detailData.value || {}
  const status = String(d.status || '')
  const active = status === 'FAILURE'
    ? 'var(--ta-danger)'
    : status === 'SUCCESS' ? 'var(--ta-success)' : 'rgb(var(--arcoblue-6))'
  const mk = (label: string, time?: string) => ({
    label,
    time: time || '',
    short: time ? String(time).slice(11, 19) : '',
    color: time ? active : 'var(--color-text-4)',
  })
  return [
    mk('创建', d.created_at),
    mk('提交上游', d.submit_time),
    mk('开始执行', d.start_time),
    mk('完成', d.finish_time),
  ]
})

const durationList = computed(() => {
  const d = detailData.value || {}
  const items: { label: string; ms: number | null }[] = [
    { label: '排队耗时', ms: diffMs(d.created_at, d.submit_time) },
    { label: '执行耗时', ms: diffMs(d.start_time || d.submit_time, d.finish_time) },
    { label: '总耗时', ms: diffMs(d.created_at, d.finish_time) },
  ]
  return items
    .filter((i): i is { label: string; ms: number } => i.ms !== null)
    .map(i => ({ label: i.label, value: formatDuration(i.ms) }))
})

// 结果资源类型：按扩展名判定，视频/音频内联播放，图片走缩略图，其余回退为链接
const resultKind = computed<'image' | 'video' | 'audio' | 'file'>(() => {
  const url = String(detailData.value?.result_url || '')
  if (/\.(jpg|jpeg|png|gif|webp|bmp|svg)([?#]|$)/i.test(url)) return 'image'
  if (/\.(mp4|mov|webm|m4v|mkv)([?#]|$)/i.test(url)) return 'video'
  if (/\.(mp3|wav|m4a|aac|ogg|flac)([?#]|$)/i.test(url)) return 'audio'
  return 'file'
})

// 复制上游排障标识到剪贴板（与上游沟通时直接粘贴）
async function copyText(text: string) {
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    Message.success('已复制')
  } catch {
    Message.error('复制失败，请手动选择复制')
  }
}

watch(
  [showDetail, () => detailData.value?.status],
  ([visible, status]) => {
    const running = visible && ['NOT_START', 'SUBMITTED', 'IN_PROGRESS'].includes(String(status || ''))
    if (running) startHeroTicker()
    else stopHeroTicker()
  },
  { immediate: true },
)

onUnmounted(stopHeroTicker)

onMounted(() => {
  const route = useRoute()
  if (route.query.public_task_id) {
    filterTaskId.value = String(route.query.public_task_id)
  }
  fetchData()
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="任务日志" description="大模型异步生成任务（视频/图片/音乐）的执行记录与管理" />

    <!-- Filters -->
    <ACard :bordered="false" class="mb-4">
      <ASpace wrap>
        <ARangePicker
          v-model="filterDateRange"
          show-time
          :shortcuts="quickDateRanges"
          shortcuts-position="bottom"
          style="width: 340px"
          @change="handleFilter"
        />
        <AInput
          v-model="filterTaskId"
          placeholder="任务ID"
          allow-clear
          style="width: 200px"
          @keydown.enter="handleFilter"
        />
        <ASelect
          v-model="filterPlatform"
          :options="platformOptions"
          placeholder="平台"
          allow-clear
          style="width: 130px"
          @change="handleFilter"
        />
        <ASelect
          v-model="filterStatus"
          :options="statusOptions"
          placeholder="状态"
          allow-clear
          style="width: 130px"
          @change="handleFilter"
        />
        <AInput
          v-model="filterModel"
          placeholder="模型"
          allow-clear
          style="width: 160px"
          @keydown.enter="handleFilter"
        />
        <AInputNumber
          v-model="filterTenantId"
          placeholder="租户ID"
          :min="1"
          allow-clear
          style="width: 120px"
          @change="handleFilter"
          @clear="handleFilter"
        />
        <AInputNumber
          v-model="filterUserId"
          placeholder="用户ID"
          :min="1"
          allow-clear
          style="width: 120px"
          @change="handleFilter"
          @clear="handleFilter"
        />
        <AButton type="primary" @click="handleFilter">搜索</AButton>
        <AButton @click="resetFilter">重置</AButton>
        <AButton @click="handleRefresh">刷新</AButton>
      </ASpace>
    </ACard>

    <!-- Table -->
    <ACard :bordered="false">
      <ResponsiveTable
        :columns="columns"
        :data="data"
        :loading="loading"
        :scroll="{ x: 1400 }"
        :stripe="true"
        row-key="id"
        card-title-key="public_task_id"
        card-subtitle-key="model_name"
        card-badge-key="status"
        :card-fields="['platform', 'action', 'progress', 'actual_cost']"
      />
      <div class="table-footer">
        <TableStats :total="pagination.total" />
        <APagination
          v-model:current="pagination.current"
          v-model:page-size="pagination.pageSize"
          :total="pagination.total"
          :page-size-options="pagination.pageSizeOptions"
          show-page-size
          @change="fetchData"
          @page-size-change="(size: number) => { pagination.pageSize = size; pagination.current = 1; fetchData() }"
        />
      </div>
    </ACard>

    <!-- Detail Drawer -->
    <ADrawer v-model:visible="showDetail" :width="760" title="任务详情" unmount-on-close>
      <ASpin class="detail-spin" :loading="detailLoading">
        <template v-if="detailData">
          <!-- 状态抬头区：打开抽屉第一眼就能判断任务处境 -->
          <div class="detail-hero">
            <div class="detail-hero__top">
              <div class="detail-hero__status">
                <span class="detail-hero__dot" :style="{ background: detailVisual.dot }"></span>
                <span class="detail-hero__text">{{ detailVisual.label }}</span>
                <span class="detail-hero__sub">{{ detailData.model_name || '-' }} · {{ detailData.action || '-' }}</span>
              </div>
              <ASpace :size="6">
                <ATag v-if="platformKind[detailData.platform]" size="small">{{ platformKind[detailData.platform] }}</ATag>
                <ATag size="small" color="arcoblue">{{ platformLabel[detailData.platform] || detailData.platform }}</ATag>
              </ASpace>
            </div>
            <AProgress
              class="detail-hero__progress"
              :percent="progressRatio"
              :show-text="false"
              size="small"
              :color="detailVisual.bar"
            />
            <div class="detail-hero__foot">
              <span>进度 {{ detailData.progress || '0%' }}</span>
              <span v-if="runningOrTotalText">{{ runningOrTotalText }}</span>
            </div>
          </div>

          <!-- 失败原因：紧跟抬头区，避免埋在字段表末尾被忽略 -->
          <div v-if="detailData.fail_reason" class="detail-alert">
            <span class="detail-alert__title">失败原因</span>
            <span class="detail-alert__body">{{ detailData.fail_reason }}</span>
          </div>

          <!-- 任务ID：等宽 + 单行省略，长 ID 不再折行破坏整体对齐 -->
          <div class="detail-idbar">
            <div class="detail-idbar__main">
              <span class="detail-idbar__label">任务ID</span>
              <ATooltip :content="detailData.public_task_id" position="top" :disabled="!detailData.public_task_id">
                <span class="detail-idbar__value">{{ detailData.public_task_id || '-' }}</span>
              </ATooltip>
              <AButton
                v-if="detailData.public_task_id"
                size="mini"
                type="text"
                @click="copyText(detailData.public_task_id)"
              >复制</AButton>
            </div>
            <AButton size="mini" type="text" :loading="detailLoading" @click="reloadDetail()">刷新</AButton>
          </div>

          <h3 class="detail-section-title">任务实例</h3>
          <div class="detail-grid">
            <span class="detail-grid__k">平台</span>
            <span class="detail-grid__v">{{ platformLabel[detailData.platform] || detailData.platform || '-' }}</span>
            <span class="detail-grid__k">动作</span>
            <span class="detail-grid__v">{{ detailData.action || '-' }}</span>
            <span class="detail-grid__k">模型</span>
            <span class="detail-grid__v detail-mono">{{ detailData.model_name || '-' }}</span>
            <span class="detail-grid__k">上游模型</span>
            <span class="detail-grid__v detail-mono">{{ detailData.upstream_model || '-' }}</span>
            <span class="detail-grid__k">租户ID</span>
            <span class="detail-grid__v">#{{ detailData.tenant_id }}</span>
            <span class="detail-grid__k">用户ID</span>
            <span class="detail-grid__v">#{{ detailData.user_id }}</span>
          </div>

          <h3 class="detail-section-title">计费</h3>
          <div class="detail-metrics">
            <div class="detail-metric">
              <div class="detail-metric__label">预扣金额</div>
              <div class="detail-metric__value">
                {{ detailData.pre_deduct_amount > 0 ? formatBilling(detailData.pre_deduct_amount, 4) : '-' }}
              </div>
            </div>
            <div class="detail-metric">
              <div class="detail-metric__label">实际费用</div>
              <div class="detail-metric__value">
                {{ detailData.billing_settled ? formatBilling(detailData.actual_cost, 4) : '未结算' }}
              </div>
            </div>
            <div class="detail-metric">
              <div class="detail-metric__label">结算状态</div>
              <div class="detail-metric__value">{{ detailData.billing_settled ? '已结算' : '待结算' }}</div>
            </div>
          </div>

          <h3 class="detail-section-title">时间线</h3>
          <div class="detail-timeline">
            <span class="detail-timeline__track"></span>
            <div v-for="n in timelineNodes" :key="n.label" class="detail-timeline__node">
              <div class="detail-timeline__label">{{ n.label }}</div>
              <span class="detail-timeline__dot" :style="{ background: n.color }"></span>
              <div class="detail-timeline__time" :class="{ 'is-empty': !n.time }">{{ n.short || '—' }}</div>
            </div>
          </div>
          <div v-if="durationList.length" class="detail-durations">
            <span v-for="i in durationList" :key="i.label">{{ i.label }} {{ i.value }}</span>
          </div>

          <h3 class="detail-section-title">排障标识</h3>
          <div class="detail-grid detail-grid--wide">
            <span class="detail-grid__k">请求ID</span>
            <span class="detail-grid__v">
              <template v-if="detailData.request_id">
                <ATooltip :content="detailData.request_id" position="top">
                  <span class="detail-mono detail-ellipsis">{{ detailData.request_id }}</span>
                </ATooltip>
                <AButton size="mini" type="text" @click="copyText(detailData.request_id)">复制</AButton>
              </template>
              <span v-else>-</span>
            </span>
            <span class="detail-grid__k">上游任务ID</span>
            <span class="detail-grid__v">
              <template v-if="detailData.upstream_task_id">
                <ATooltip :content="detailData.upstream_task_id" position="top">
                  <span class="detail-mono detail-ellipsis">{{ detailData.upstream_task_id }}</span>
                </ATooltip>
                <AButton size="mini" type="text" @click="copyText(detailData.upstream_task_id)">复制</AButton>
              </template>
              <span v-else>-</span>
            </span>
            <span class="detail-grid__k">上游请求ID</span>
            <span class="detail-grid__v">
              <template v-if="detailData.upstream_request_id">
                <ATooltip :content="detailData.upstream_request_id" position="top">
                  <span class="detail-mono detail-ellipsis">{{ detailData.upstream_request_id }}</span>
                </ATooltip>
                <AButton size="mini" type="text" @click="copyText(detailData.upstream_request_id)">复制</AButton>
              </template>
              <span v-else>-</span>
            </span>
          </div>

          <!-- 结果：图片缩略图 / 视频音频内联播放 / 其余回退为链接 -->
          <template v-if="detailData.result_url">
            <h3 class="detail-section-title">结果</h3>
            <div class="result-preview">
              <a
                v-if="resultKind === 'image'"
                :href="detailData.result_url"
                target="_blank"
                rel="noopener"
                title="点击查看原图"
              >
                <img
                  :src="detailData.result_thumb_url || detailData.result_url"
                  alt="任务结果"
                  class="result-image"
                />
              </a>
              <video
                v-else-if="resultKind === 'video'"
                class="result-video"
                :src="detailData.result_url"
                controls
                preload="metadata"
              ></video>
              <audio
                v-else-if="resultKind === 'audio'"
                class="result-audio"
                :src="detailData.result_url"
                controls
                preload="metadata"
              ></audio>
              <a v-else :href="detailData.result_url" target="_blank" rel="noopener" class="result-link">
                {{ detailData.result_url }}
              </a>
            </div>
            <div class="result-actions">
              <a :href="detailData.result_url" target="_blank" rel="noopener" class="result-action">打开原文件</a>
              <a class="result-action" @click="copyText(detailData.result_url)">复制链接</a>
            </div>
          </template>
        </template>
      </ASpin>
    </ADrawer>
  </div>
</template>

<style scoped>
.table-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 16px;
}
/* 统计栏移入底部后，去掉全局样式的下边距，与分页栏垂直居中 */
.table-footer :deep(.table-stats) {
  margin-bottom: 0;
}

/* ===== 任务详情抽屉 ===== */

/* Arco Spin 根节点是 display:inline-block（arco.css: .arco-spin{display:inline-block}），
   会把内容压成「最长不可换行内容」决定的收缩宽度（本页长任务ID 等 nowrap 文本 → 约 418px），
   于是内部所有块级元素（hero / 告警 / idbar / 指标卡 / grid）在 720px 的抽屉里只占一半，
   右侧留大片空白。这里强制撑满抽屉内容宽度。 */
.detail-spin {
  display: block;
  width: 100%;
}

/* 状态抬头区：状态 + 进度 + 耗时，打开抽屉第一屏即给出结论 */
.detail-hero {
  background: var(--ta-bg-secondary);
  border-radius: var(--ta-radius);
  padding: 12px 14px;
}

.detail-hero__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.detail-hero__status {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.detail-hero__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.detail-hero__text {
  font-size: 15px;
  font-weight: 600;
  color: var(--ta-text-primary);
  flex-shrink: 0;
}

.detail-hero__sub {
  font-size: 12px;
  color: var(--ta-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-hero__progress {
  margin-top: 10px;
}

.detail-hero__foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 6px;
  font-size: 12px;
  color: var(--ta-text-secondary);
}

/* 失败原因告警块 */
.detail-alert {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-top: 10px;
  padding: 10px 12px;
  border-radius: var(--ta-radius);
  background: var(--color-danger-light-1);
  border: 1px solid var(--color-danger-light-2);
}

.detail-alert__title {
  flex-shrink: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--ta-danger);
  line-height: 1.6;
}

.detail-alert__body {
  font-size: 12px;
  line-height: 1.6;
  color: var(--ta-text-primary);
  word-break: break-all;
}

/* 任务ID：等宽 + 单行省略，避免长 ID 折行把整行行高撑成两倍 */
.detail-idbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 10px;
  padding: 5px 10px;
  border: 1px solid var(--ta-border);
  border-radius: var(--ta-radius);
}

.detail-idbar__main {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.detail-idbar__label {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--ta-text-tertiary);
}

.detail-idbar__value {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: monospace;
  font-size: 12px;
  color: var(--ta-text-primary);
}

.detail-section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ta-text-primary);
  margin: 20px 0 10px;
}

.detail-section-title:first-child {
  margin-top: 0;
}

/* 字段表：两列 label/value，比 bordered 表格更紧凑，且不产生空单元格 */
.detail-grid {
  display: grid;
  grid-template-columns: 64px minmax(0, 1fr) 64px minmax(0, 1fr);
  gap: 9px 16px;
  align-items: baseline;
}

.detail-grid--wide {
  grid-template-columns: 76px minmax(0, 1fr);
}

.detail-grid__k {
  font-size: 13px;
  color: var(--ta-text-tertiary);
}

.detail-grid__v {
  font-size: 13px;
  color: var(--ta-text-primary);
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  word-break: break-all;
}

.detail-mono {
  font-family: monospace;
}

.detail-ellipsis {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
}

/* 计费指标卡 */
.detail-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.detail-metric {
  background: var(--ta-bg-secondary);
  border-radius: var(--ta-radius);
  padding: 10px 12px;
}

.detail-metric__label {
  font-size: 12px;
  color: var(--ta-text-tertiary);
}

.detail-metric__value {
  margin-top: 4px;
  font-size: 15px;
  font-weight: 600;
  color: var(--ta-text-primary);
}

/* 时间线：四节点等分，节点连线作为底轨 */
.detail-timeline {
  position: relative;
  display: flex;
}

.detail-timeline__track {
  position: absolute;
  top: 34px;
  left: 12.5%;
  right: 12.5%;
  height: 1px;
  background: var(--ta-border);
}

.detail-timeline__node {
  position: relative;
  flex: 1;
  min-width: 0;
  padding-top: 46px;
  text-align: center;
}

.detail-timeline__label {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  font-size: 12px;
  color: var(--ta-text-tertiary);
}

.detail-timeline__dot {
  position: absolute;
  top: 30px;
  left: 50%;
  width: 8px;
  height: 8px;
  margin-left: -4px;
  border-radius: 50%;
}

.detail-timeline__time {
  font-size: 12px;
  color: var(--ta-text-primary);
  font-family: monospace;
}

.detail-timeline__time.is-empty {
  color: var(--ta-text-tertiary);
}

.detail-durations {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 16px;
  margin-top: 10px;
  font-size: 12px;
  color: var(--ta-text-secondary);
}

.result-preview {
  background: var(--color-fill-1);
  border-radius: 6px;
  padding: 12px;
}

.result-image {
  display: block;
  max-width: 100%;
  border-radius: 6px;
  cursor: zoom-in;
}

.result-video {
  display: block;
  width: 100%;
  max-height: 400px;
  border-radius: 6px;
  background: #000;
}

.result-audio {
  display: block;
  width: 100%;
}

.result-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  margin-top: 8px;
  font-size: 12px;
}

.result-action {
  color: rgb(var(--arcoblue-6));
  cursor: pointer;
}

.result-link {
  color: rgb(var(--arcoblue-6));
  word-break: break-all;
}

@media (max-width: 767px) {
  .detail-grid,
  .detail-grid--wide {
    grid-template-columns: 68px minmax(0, 1fr);
  }
  .detail-metrics {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
