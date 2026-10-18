<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, h } from 'vue'
import {
  Tag, Button, Space, Popconfirm, Tooltip, Progress,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import { useRouter } from 'vue-router'
import { IconSettings } from '@arco-design/web-vue/es/icon'
import PageHeader from '@/components/PageHeader.vue'
import TableStats from '@/components/TableStats.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import request from '@/utils/request'
import { hasPermission } from '@/utils/permission'
import { useTenantOptions } from '@/composables/useTenantOptions'
import AnalyzeModal from './AnalyzeModal.vue'
import ReportDetailDrawer from './ReportDetailDrawer.vue'
import ConfigDrawer from './ConfigDrawer.vue'

// AI请求分析（ai-req-analyze 插件）页面入口：筛选 + 报告/任务双 Tab + 弹层协调。
// 弹层组件内聚各自逻辑（发起分析 / 报告详情 / 插件配置），本文件只管列表与状态。
//
// 权限：页面挂在 ai-req-analyze:view 下；发起分析/删除按 analyze、插件配置按 config。

const router = useRouter()
const canAnalyze = computed(() => hasPermission('ai-req-analyze:analyze'))
const canConfig = computed(() => hasPermission('ai-req-analyze:config'))

// ── 标签映射 ────────────────────────────────────────────────────────────────

const riskLabel: Record<string, string> = {
  high: '高风险', medium: '中风险', low: '低风险', clean: '未发现风险', unknown: '未知',
}
const riskColor: Record<string, string> = {
  high: 'red', medium: 'orange', low: 'arcoblue', clean: 'green', unknown: 'gray',
}
const sourceLabel: Record<string, string> = { manual: '手动分析', patrol: '定时巡查' }
const jobStatusLabel: Record<string, string> = {
  pending: '排队中', running: '执行中', completed: '已完成', failed: '失败',
}
const jobStatusColor: Record<string, string> = {
  pending: 'arcoblue', running: 'orange', completed: 'green', failed: 'red',
}

// 后端时间为 Asia/Shanghai 墙钟字符串，截取前 19 位统一展示（避免浏览器时区二次换算）
function fmtTime(s?: string | null): string {
  if (!s) return '-'
  return String(s).replace('T', ' ').slice(0, 19)
}

// ── 插件可用性 ──────────────────────────────────────────────────────────────

// 插件未安装/未启用时其路由未注册，业务接口表现为 404 —— 引导前往插件管理
const pluginUnavailable = ref(false)
function markUnavailableIf404(err: any) {
  if (err?.response?.status === 404 || (err as any)?.apiError?.code === 404) {
    pluginUnavailable.value = true
  }
}

// ── 弹层状态 ────────────────────────────────────────────────────────────────

const analyzeVisible = ref(false)
const configVisible = ref(false)
const detailReportId = ref<number | null>(null)
const detailVisible = ref(false)

function openDetail(id: number) {
  detailReportId.value = id
  detailVisible.value = true
}

// ── 报告列表 ────────────────────────────────────────────────────────────────

const activeTab = ref<'reports' | 'jobs'>('reports')
const reports = ref<any[]>([])
const reportLoading = ref(false)
const reportPage = ref(1)
const reportPageSize = ref(20)
const reportTotal = ref(0)

const { tenantOptions, fetchTenantOptions, handleTenantSearch } = useTenantOptions()

const reportFilter = ref({
  tenant_id: undefined as number | undefined,
  source: '',
  risk_level: '',
  start_date: '',
  end_date: '',
})

const riskOptions = [
  { label: '全部风险', value: '' },
  { label: '高风险', value: 'high' },
  { label: '中风险', value: 'medium' },
  { label: '低风险', value: 'low' },
  { label: '未发现风险', value: 'clean' },
]
const sourceOptions = [
  { label: '全部来源', value: '' },
  { label: '手动分析', value: 'manual' },
  { label: '定时巡查', value: 'patrol' },
]

async function fetchReports() {
  reportLoading.value = true
  try {
    const params: Record<string, any> = {
      page: reportPage.value, page_size: reportPageSize.value,
    }
    if (reportFilter.value.tenant_id) params.tenant_id = reportFilter.value.tenant_id
    if (reportFilter.value.source) params.source = reportFilter.value.source
    if (reportFilter.value.risk_level) params.risk_level = reportFilter.value.risk_level
    if (reportFilter.value.start_date) params.start_date = reportFilter.value.start_date
    if (reportFilter.value.end_date) params.end_date = reportFilter.value.end_date

    const res: any = await request.get('/admin/plugins/ai-req-analyze/reports', { params })
    const raw = res.data?.data
    reports.value = raw?.list || []
    reportTotal.value = raw?.total || 0
  } catch (err) {
    reports.value = []
    reportTotal.value = 0
    markUnavailableIf404(err)
  } finally {
    reportLoading.value = false
  }
}

function handleReportFilter() {
  reportPage.value = 1
  fetchReports()
}

async function deleteReport(id: number) {
  try {
    await request.delete(`/admin/plugins/ai-req-analyze/reports/${id}`)
    reports.value = reports.value.filter((r) => r.id !== id)
    reportTotal.value = Math.max(0, reportTotal.value - 1)
    fetchReports()
  } catch {
    // 拦截器已提示
  }
}

const reportColumns: TableColumnData[] = [
  {
    title: '风险等级',
    dataIndex: 'risk_level',
    width: 110,
    render: ({ record }) =>
      h(Tag, { color: riskColor[record.risk_level] || 'gray', size: 'small' },
        () => riskLabel[record.risk_level] || record.risk_level),
  },
  {
    title: '租户',
    dataIndex: 'tenant_name',
    width: 140,
    render: ({ record }) => h('span', {}, record.tenant_name || `#${record.tenant_id}`),
  },
  {
    title: '分析对象',
    dataIndex: 'target_username',
    width: 110,
    render: ({ record }) => h('span', {}, record.target_username || '全员'),
  },
  {
    title: '来源',
    dataIndex: 'source',
    width: 90,
    render: ({ record }) => h(Tag, { size: 'small' }, () => sourceLabel[record.source] || record.source),
  },
  {
    title: '分析窗口',
    dataIndex: 'time_start',
    width: 280,
    render: ({ record }) => h('span', { style: 'font-size:12px;white-space:nowrap;' },
      `${fmtTime(record.time_start)} ~ ${fmtTime(record.time_end)}`),
  },
  {
    title: '发现',
    dataIndex: 'finding_count',
    width: 80,
    render: ({ record }) => {
      if (!record.finding_count) return h('span', { style: 'color: var(--color-text-3);' }, '0')
      return h(Tooltip, { content: '点击查看详情' }, () =>
        h(Tag, { color: record.risk_level === 'high' ? 'red' : 'orange', size: 'small' },
          () => `${record.finding_count} 项`))
    },
  },
  {
    title: '创建时间',
    dataIndex: 'created_at',
    width: 160,
    render: ({ record }) => h('span', { style: 'font-size:12px;' }, fmtTime(record.created_at)),
  },
  {
    title: '操作',
    dataIndex: 'actions',
    width: 140,
    fixed: 'right',
    render: ({ record }) => {
      const btns: any[] = [
        h(Button, { size: 'small', type: 'primary', onClick: () => openDetail(record.id) }, () => '详情'),
      ]
      if (canAnalyze.value) {
        btns.push(
          h(Popconfirm, { content: '删除后报告与发现明细不可恢复，确定删除？', onOk: () => deleteReport(record.id) },
            () => h(Button, { size: 'small', status: 'danger' }, () => '删除')),
        )
      }
      return h(Space, { size: 4 }, () => btns)
    },
  },
]

// ── 任务列表 ────────────────────────────────────────────────────────────────

const jobs = ref<any[]>([])
const jobLoading = ref(false)
const jobPage = ref(1)
const jobPageSize = ref(20)
const jobTotal = ref(0)

const jobStatusOptions = [
  { label: '全部状态', value: '' },
  { label: '排队中', value: 'pending' },
  { label: '执行中', value: 'running' },
  { label: '已完成', value: 'completed' },
  { label: '失败', value: 'failed' },
]
const jobFilterStatus = ref('')

async function fetchJobs() {
  jobLoading.value = true
  try {
    const params: Record<string, any> = { page: jobPage.value, page_size: jobPageSize.value }
    if (jobFilterStatus.value) params.status = jobFilterStatus.value
    const res: any = await request.get('/admin/plugins/ai-req-analyze/jobs', { params })
    const raw = res.data?.data
    jobs.value = raw?.list || []
    jobTotal.value = raw?.total || 0
  } catch {
    jobs.value = []
    jobTotal.value = 0
  } finally {
    jobLoading.value = false
  }
}

// 有排队/执行中任务且处于任务 Tab 时轮询刷新进度
let jobTimer: ReturnType<typeof setInterval> | null = null
function hasActiveJob(): boolean {
  return jobs.value.some((j) => j.status === 'pending' || j.status === 'running')
}

const jobColumns: TableColumnData[] = [
  {
    title: '状态',
    dataIndex: 'status',
    width: 100,
    render: ({ record }) =>
      h(Tag, { color: jobStatusColor[record.status] || 'gray', size: 'small' },
        () => jobStatusLabel[record.status] || record.status),
  },
  {
    title: '租户',
    dataIndex: 'tenant_id',
    width: 100,
    render: ({ record }) => h('span', {}, `#${record.tenant_id}`),
  },
  {
    title: '来源',
    dataIndex: 'source',
    width: 90,
    render: ({ record }) => h(Tag, { size: 'small' }, () => sourceLabel[record.source] || record.source),
  },
  {
    title: '分析窗口',
    dataIndex: 'time_start',
    width: 270,
    render: ({ record }) => h('span', { style: 'font-size:12px;white-space:nowrap;' },
      `${fmtTime(record.time_start)} ~ ${fmtTime(record.time_end)}`),
  },
  {
    title: '进度',
    dataIndex: 'progress',
    width: 160,
    render: ({ record }) => {
      if (record.status === 'failed') {
        // 长错误信息（含内容审查拦截明细）完整展示：限宽换行，不截断
        return h(Tooltip, {
          content: () => h('div', { class: 'err-detail' }, record.error_msg || '未知原因'),
        }, () => h(Tag, { color: 'red', size: 'small' }, () => '失败原因'))
      }
      const pct = record.status === 'completed' ? 100 : (record.progress || 0)
      return h(Progress, {
        size: 'small', percent: pct / 100, showText: true,
        status: record.status === 'completed' ? 'success' : 'normal',
      })
    },
  },
  {
    title: '创建时间',
    dataIndex: 'created_at',
    width: 160,
    render: ({ record }) => h('span', { style: 'font-size:12px;' }, fmtTime(record.created_at)),
  },
]

// ── 生命周期 ────────────────────────────────────────────────────────────────

onMounted(() => {
  fetchTenantOptions()
  fetchReports()
  fetchJobs()
  jobTimer = setInterval(() => {
    if (hasActiveJob() && activeTab.value === 'jobs') fetchJobs()
  }, 5000)
})

onUnmounted(() => {
  if (jobTimer) clearInterval(jobTimer)
})

function onAnalyzeSubmitted() {
  activeTab.value = 'jobs'
  jobPage.value = 1
  fetchJobs()
}
</script>

<template>
  <div class="page-table">
    <PageHeader title="AI请求分析" description="基于会话关联的 AI 使用行为分析：合规风险与数据泄露双维度报告">
      <template #actions>
        <AButton v-if="canConfig" @click="configVisible = true">
          <template #icon><IconSettings /></template>
          插件配置
        </AButton>
        <AButton v-if="canAnalyze" type="primary" @click="analyzeVisible = true">发起分析</AButton>
      </template>
    </PageHeader>

    <!-- 插件不可用引导 -->
    <ACard v-if="pluginUnavailable" :bordered="false">
      <AEmpty description="AI请求分析插件未安装或未启用">
        <AButton type="primary" @click="router.push({ name: 'AdminPlugins' })">前往插件管理</AButton>
      </AEmpty>
    </ACard>

    <template v-else>
      <!-- 筛选 -->
      <ACard :bordered="false" class="mb-4">
        <div class="filter-bar">
          <ASelect
            v-model="reportFilter.tenant_id"
            :options="tenantOptions"
            placeholder="租户"
            allow-search allow-clear
            :filter-option="false"
            style="width: 200px"
            @search="handleTenantSearch"
            @change="handleReportFilter"
            @clear="handleReportFilter"
          />
          <ASelect v-model="reportFilter.source" :options="sourceOptions" placeholder="来源"
            allow-clear style="width: 120px" @change="handleReportFilter" @clear="handleReportFilter" />
          <ASelect v-model="reportFilter.risk_level" :options="riskOptions" placeholder="风险等级"
            allow-clear style="width: 130px" @change="handleReportFilter" @clear="handleReportFilter" />
          <ARangePicker
            v-model="reportFilter.range"
            style="width: 280px"
            value-format="YYYY-MM-DD"
            @change="(v: any) => {
              reportFilter.start_date = v?.[0] || ''
              reportFilter.end_date = v?.[1] || ''
              handleReportFilter()
            }"
          />
          <div class="filter-actions">
            <AButton @click="fetchReports">刷新</AButton>
          </div>
        </div>
      </ACard>

      <ACard :bordered="false">
        <ATabs v-model:active-key="activeTab" @change="(key: any) => key === 'jobs' && fetchJobs()">
          <ATabPane key="reports" title="分析报告">
            <ResponsiveTable
              :columns="reportColumns"
              :data="reports"
              :loading="reportLoading"
              :scroll="{ x: 1100 }"
              :stripe="true"
              row-key="id"
              card-title-key="tenant_name"
              card-subtitle-key="time_start"
              card-badge-key="risk_level"
              :card-fields="['source', 'target_username', 'finding_count', 'created_at']"
            />
            <AEmpty v-if="!reportLoading && reports.length === 0" style="margin-top: 40px">
              <template #description>
                暂无分析报告{{ canConfig ? '；首次使用请先通过右上角「插件配置」完成分析 AI 连接' : '' }}
              </template>
            </AEmpty>
            <div class="table-footer">
              <TableStats :total="reportTotal" :page="reportPage" :page-size="reportPageSize" />
              <APagination
                v-model:current="reportPage"
                v-model:page-size="reportPageSize"
                :total="reportTotal"
                show-total show-page-size
                @change="fetchReports"
                @page-size-change="handleReportFilter"
              />
            </div>
          </ATabPane>

          <ATabPane key="jobs" title="任务列表">
            <ResponsiveTable
              :columns="jobColumns"
              :data="jobs"
              :loading="jobLoading"
              :scroll="{ x: 900 }"
              :stripe="true"
              row-key="id"
              card-title-key="tenant_id"
              card-badge-key="status"
              :card-fields="['source', 'progress', 'time_start', 'created_at']"
            />
            <AEmpty v-if="!jobLoading && jobs.length === 0" style="margin-top: 40px">
              <template #description>暂无分析任务</template>
            </AEmpty>
            <div class="table-footer">
              <div class="job-filter">
                <ASelect v-model="jobFilterStatus" :options="jobStatusOptions" placeholder="任务状态"
                  allow-clear style="width: 120px"
                  @change="() => { jobPage = 1; fetchJobs() }" @clear="() => { jobPage = 1; fetchJobs() }" />
                <AButton size="small" @click="fetchJobs">刷新</AButton>
              </div>
              <APagination
                v-model:current="jobPage"
                v-model:page-size="jobPageSize"
                :total="jobTotal"
                show-total
                @change="fetchJobs"
              />
            </div>
          </ATabPane>
        </ATabs>
      </ACard>
    </template>

    <!-- 弹层：发起分析 / 报告详情 / 插件配置 -->
    <AnalyzeModal v-model:visible="analyzeVisible" @submitted="onAnalyzeSubmitted" />
    <ReportDetailDrawer
      v-model:visible="detailVisible"
      :report-id="detailReportId"
      :can-delete="canAnalyze"
      @deleted="fetchReports"
    />
    <ConfigDrawer v-model:visible="configVisible" />
  </div>
</template>

<style scoped>
.filter-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
}
.filter-actions {
  margin-left: auto;
  display: flex;
  gap: 8px;
}
.table-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: 16px;
  gap: 12px;
  flex-wrap: wrap;
}
.job-filter {
  display: flex;
  gap: 8px;
  align-items: center;
}
</style>

<!-- 非 scoped：tooltip 挂载在 body，样式需全局生效 -->
<style>
.err-detail {
  max-width: 420px;
  white-space: pre-wrap;
  word-break: break-all;
  line-height: 1.6;
}
</style>
