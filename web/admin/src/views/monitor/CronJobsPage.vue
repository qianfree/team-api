<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import { Message, Tag, Button, Space } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const jobs = ref<any[]>([])

// Execution history for expanded row
const expandedKeys = ref<string[]>([])
const execLoading = ref<Record<string, boolean>>({})
const execData = ref<Record<string, any[]>>({})
const execPagination = reactive<Record<string, { current: number; total: number }>>({})

const jobNameMap: Record<string, string> = {
  ops_system_collector: '系统指标采集',
  ops_alert_detector: '告警检测',
  ops_metrics_cleanup: '指标数据清理',
  partition_ensure: '分区维护',
  health_snapshot: '渠道健康快照',
  channel_auto_test: '渠道自动测试',
  model_sunset_check: '模型下线检查',
  data_cleanup: '过期数据清理',
  export_file_cleanup: '导出文件清理',
  file_retention_check: '文件保留检查',
  task_timeout_check: '任务超时检查',
  task_executor: '异步任务执行',
  usage_log_cleanup: '用量日志清理',
  oauth_token_refresh: 'OAuth 令牌刷新',
  cron_execution_cleanup: '执行记录清理',
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

const columns: TableColumnData[] = [
  {
    title: '任务名称',
    dataIndex: 'name',
    width: 200,
    render({ record }: { record: any }) {
      return h('span', jobNameMap[record.name] || record.name)
    },
  },
  {
    title: 'Cron 表达式',
    dataIndex: 'schedule',
    width: 140,
    render({ record }: { record: any }) {
      return h('span', { style: 'font-family: monospace; font-size: 13px' }, record.schedule)
    },
  },
  {
    title: '状态',
    dataIndex: 'is_running',
    width: 100,
    render({ record }: { record: any }) {
      if (record.is_running) {
        return h(Tag, { color: 'blue', size: 'small' }, () => '运行中')
      }
      return h(Tag, { color: 'gray', size: 'small' }, () => '空闲')
    },
  },
  {
    title: '上次执行',
    dataIndex: 'last_status',
    width: 100,
    render({ record }: { record: any }) {
      if (!record.last_status) return h('span', { style: 'color: var(--color-text-4)' }, '-')
      const color = record.last_status === 'succeeded' ? 'green' : 'red'
      const label = record.last_status === 'succeeded' ? '成功' : '失败'
      return h(Tag, { color, size: 'small' }, () => label)
    },
  },
  {
    title: '上次耗时',
    dataIndex: 'last_duration_ms',
    width: 100,
    render({ record }: { record: any }) {
      if (!record.last_duration_ms) return '-'
      return formatDuration(record.last_duration_ms)
    },
  },
  {
    title: '上次执行时间',
    dataIndex: 'last_started_at',
    width: 180,
    render({ record }: { record: any }) {
      if (!record.last_started_at) return '-'
      return new Date(record.last_at || record.last_started_at).toLocaleString('zh-CN')
    },
  },
  {
    title: '统计',
    width: 160,
    render({ record }: { record: any }) {
      if (!record.total_executions) return '-'
      const failColor = record.total_failures > 0 ? '#f53f3f' : 'var(--color-text-2)'
      return h('span', [
        `${record.total_executions} 次`,
        h('span', { style: `color: ${failColor}; margin-left: 8px` }, `失败 ${record.total_failures}`),
      ])
    },
  },
  {
    title: '操作',
    width: 100,
    fixed: 'right',
    render({ record }: { record: any }) {
      return h(
        Button,
        {
          size: 'small',
          type: 'text',
          onClick: () => triggerJob(record.name),
        },
        () => '手动触发',
      )
    },
  },
]

const execColumns: TableColumnData[] = [
  {
    title: '状态',
    dataIndex: 'status',
    width: 80,
    render({ record }: { record: any }) {
      const color = record.status === 'succeeded' ? 'green' : 'red'
      const label = record.status === 'succeeded' ? '成功' : '失败'
      return h(Tag, { color, size: 'small' }, () => label)
    },
  },
  {
    title: '耗时',
    dataIndex: 'duration_ms',
    width: 100,
    render({ record }: { record: any }) {
      return formatDuration(record.duration_ms)
    },
  },
  {
    title: '触发方式',
    dataIndex: 'triggered_by',
    width: 100,
    render({ record }: { record: any }) {
      const label = record.triggered_by === 'manual' ? '手动' : '自动'
      const color = record.triggered_by === 'manual' ? 'arcoblue' : 'gray'
      return h(Tag, { color, size: 'small' }, () => label)
    },
  },
  {
    title: '错误消息',
    dataIndex: 'error_message',
    ellipsis: true,
  },
  {
    title: '执行时间',
    dataIndex: 'started_at',
    width: 180,
    render({ record }: { record: any }) {
      return new Date(record.started_at).toLocaleString('zh-CN')
    },
  },
]

async function fetchJobs() {
  loading.value = true
  try {
    const res = await request.get('/admin/cron-jobs')
    jobs.value = res.data?.data?.list || []
  } catch {
    // error auto-shown
  } finally {
    loading.value = false
  }
}

async function fetchExecutions(jobName: string) {
  if (!execPagination[jobName]) {
    execPagination[jobName] = { current: 1, total: 0 }
  }
  execLoading.value[jobName] = true
  try {
    const res = await request.get(`/admin/cron-jobs/${jobName}/executions`, {
      params: {
        page: execPagination[jobName].current,
        page_size: 10,
      },
    })
    execData.value[jobName] = res.data?.data?.list || []
    execPagination[jobName].total = res.data?.data?.total || 0
  } catch {
    // error auto-shown
  } finally {
    execLoading.value[jobName] = false
  }
}

async function triggerJob(name: string) {
  try {
    await request.post(`/admin/cron-jobs/${name}/trigger`)
    Message.success('任务已触发')
    // Refresh after a short delay to pick up the running status
    setTimeout(() => fetchJobs(), 1000)
  } catch {
    // error auto-shown
  }
}

function handleExpand(keys: string[]) {
  const added = keys.find(k => !expandedKeys.value.includes(k))
  const removed = expandedKeys.value.find(k => !keys.includes(k))
  expandedKeys.value = keys
  if (added) {
    execPagination[added] = { current: 1, total: 0 }
    fetchExecutions(added)
  }
  if (removed) {
    delete execData.value[removed]
    delete execPagination[removed]
  }
}

function handleExecPageChange(jobName: string, page: number) {
  execPagination[jobName].current = page
  fetchExecutions(jobName)
}

onMounted(() => {
  fetchJobs()
})
</script>

<template>
  <div>
    <PageHeader title="定时任务" description="查看系统定时任务运行状态和执行历史">
      <template #actions>
        <a-button @click="fetchJobs">刷新</a-button>
      </template>
    </PageHeader>

    <a-card :bordered="false">
      <a-table
        :data="jobs"
        :columns="columns"
        :loading="loading"
        :pagination="false"
        :bordered="false"
        :stripe="true"
        row-key="name"
        :expanded-keys="expandedKeys"
        @expanded-change="handleExpand"
      >
        <template #expand-row="{ record }">
          <div style="padding: 8px 16px 16px 48px">
            <a-table
              :data="execData[record.name] || []"
              :columns="execColumns"
              :loading="execLoading[record.name] || false"
              :pagination="false"
              :bordered="true"
              size="small"
              row-key="id"
            />
            <div v-if="execPagination[record.name]" style="display: flex; justify-content: flex-end; margin-top: 8px">
              <a-pagination
                :current="execPagination[record.name].current"
                :page-size="10"
                :total="execPagination[record.name].total"
                size="small"
                @change="(page: number) => handleExecPageChange(record.name, page)"
              />
            </div>
          </div>
        </template>
      </a-table>
    </a-card>
  </div>
</template>
