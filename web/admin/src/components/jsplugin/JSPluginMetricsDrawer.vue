<script setup lang="ts">
// JS 插件运行指标抽屉：内存钩子指标（进程内累计，重启归零）+ 任务/产物统计（DB）。
// 打开时立即拉一次，之后 30s 静默轮询；关闭即停止。
import { computed, h, onBeforeUnmount, ref, watch } from 'vue'
import { Button, Drawer, Table, Tag } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import { IconRefresh } from '@arco-design/web-vue/es/icon'
import request from '@/utils/request'
import { latestHookError, type JSPluginMetricsItem } from './pluginMeta'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{ (e: 'update:visible', v: boolean): void }>()

const list = ref<JSPluginMetricsItem[]>([])
const since = ref('')
const loading = ref(false)
const updatedAt = ref('')

let timer: ReturnType<typeof setInterval> | null = null

async function fetchData() {
  loading.value = true
  try {
    const res: any = await request.get('/admin/js-plugins/metrics')
    list.value = res.data?.data?.list || res.data?.list || []
    since.value = res.data?.data?.since || ''
    updatedAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
  } catch {
    list.value = []
  } finally {
    loading.value = false
  }
}

function startTimer() {
  stopTimer()
  timer = setInterval(fetchData, 30000)
}

function stopTimer() {
  if (timer) clearInterval(timer)
  timer = null
}

watch(
  () => props.visible,
  (v) => {
    if (v) {
      fetchData()
      startTimer()
    } else {
      stopTimer()
    }
  },
)

onBeforeUnmount(stopTimer)

const summary = computed(() => {
  let calls = 0
  let errors = 0
  let weighted = 0
  let weightedCalls = 0
  let live = 0
  let tasks = 0
  let artifacts = 0
  for (const m of list.value) {
    calls += m.total_calls || 0
    errors += m.total_errors || 0
    if (m.total_calls > 0) {
      weighted += (m.avg_ms || 0) * m.total_calls
      weightedCalls += m.total_calls
    }
    if (m.in_service) live++
    tasks += m.tasks_success || 0
    artifacts += m.artifacts_stored || 0
  }
  return {
    calls,
    errors,
    errRate: calls > 0 ? (errors / calls) * 100 : 0,
    avgMs: weightedCalls > 0 ? weighted / weightedCalls : 0,
    live,
    tasks,
    artifacts,
  }
})

function fmtMs(v: number): string {
  if (!v || v < 0) return '—'
  return v >= 1000 ? `${(v / 1000).toFixed(2)} s` : `${v.toFixed(1)} ms`
}

function fmtNum(v: number): string {
  return (v || 0).toLocaleString('zh-CN')
}

const columns: TableColumnData[] = [
  {
    title: '插件',
    dataIndex: 'key',
    width: 150,
    render: ({ record }: any) =>
      h('span', { style: 'display: inline-flex; align-items: center; gap: 6px;' }, [
        h('span', {
          style: `width:6px;height:6px;border-radius:50%;flex-shrink:0;background:${
            record.in_service ? '#10b981' : 'var(--color-text-3)'
          }`,
        }),
        h('code', { style: 'font-family: JetBrains Mono, Consolas, monospace; font-size: 12px;' }, record.key),
      ]),
  },
  {
    title: '调用',
    dataIndex: 'total_calls',
    width: 84,
    align: 'right',
    render: ({ record }: any) => fmtNum(record.total_calls),
  },
  {
    title: '错误',
    dataIndex: 'total_errors',
    width: 72,
    align: 'right',
    render: ({ record }: any) =>
      h('span', {
        style: record.total_errors > 0
          ? 'color: rgb(var(--red-6)); font-weight: 600;'
          : 'color: var(--color-text-3);',
      }, fmtNum(record.total_errors)),
  },
  {
    title: '错误率',
    dataIndex: 'err_rate',
    width: 88,
    align: 'right',
    render: ({ record }: any) => {
      if (!record.total_calls) return '—'
      const rate = (record.total_errors / record.total_calls) * 100
      return h('span', {
        style: rate >= 5 ? 'color: rgb(var(--red-6)); font-weight: 600;' : '',
      }, `${rate.toFixed(1)}%`)
    },
  },
  {
    title: '平均耗时',
    dataIndex: 'avg_ms',
    width: 100,
    align: 'right',
    render: ({ record }: any) => fmtMs(record.avg_ms),
  },
  {
    title: '任务 成/进/败',
    dataIndex: 'tasks',
    width: 140,
    align: 'right',
    render: ({ record }: any) =>
      h('span', {}, [
        h('span', { style: 'color: rgb(var(--green-6));' }, String(record.tasks_success ?? 0)),
        h('span', { style: 'color: var(--color-text-3);' }, ' / '),
        h('span', {}, String(record.tasks_active ?? 0)),
        h('span', { style: 'color: var(--color-text-3);' }, ' / '),
        h('span', { style: record.tasks_failed > 0 ? 'color: rgb(var(--red-6));' : 'color: var(--color-text-3);' },
          String(record.tasks_failed ?? 0)),
      ]),
  },
  {
    title: '产物存/败',
    dataIndex: 'artifacts',
    width: 110,
    align: 'right',
    render: ({ record }: any) =>
      `${record.artifacts_stored ?? 0} / ${record.artifacts_failed ?? 0}`,
  },
  {
    title: '最近错误',
    dataIndex: 'last_err',
    width: 192,
    render: ({ record }: any) => {
      const err = latestHookError(record)
      if (!err) return h('span', { style: 'color: var(--color-text-3);' }, '—')
      const text = `${err.hook}: ${err.message}`
      return h(Tag, {
        size: 'small',
        color: 'red',
        title: text,
        style: 'max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: inline-block; vertical-align: bottom;',
      }, () => text)
    },
  },
]
</script>

<template>
  <Drawer
    :visible="visible"
    :width="1000"
    :footer="false"
    unmount-on-close
    @cancel="emit('update:visible', false)"
  >
    <template #title>
      <div class="jm-head">
        <span class="jm-title">运行指标</span>
        <span class="jm-since">自 {{ since || '进程启动' }} 起累计</span>
        <span class="jm-updated" v-if="updatedAt">最近刷新 {{ updatedAt }}</span>
      </div>
    </template>

    <div class="jm-toolbar">
      <div class="jm-toolbar__hint">
        钩子指标为<strong>进程内存计数</strong>，重启归零；任务与产物统计来自数据库。每 30 秒自动刷新。
      </div>
      <Button size="small" :loading="loading" @click="fetchData">
        <template #icon><IconRefresh /></template>立即刷新
      </Button>
    </div>

    <div class="jm-summary">
      <div class="jm-stat">
        <span class="jm-stat__label">服务中插件</span>
        <span class="jm-stat__value">{{ summary.live }}<em>/ {{ list.length }}</em></span>
      </div>
      <div class="jm-stat">
        <span class="jm-stat__label">钩子调用</span>
        <span class="jm-stat__value">{{ fmtNum(summary.calls) }}</span>
      </div>
      <div class="jm-stat" :class="{ 'is-warn': summary.errRate >= 5 }">
        <span class="jm-stat__label">错误率</span>
        <span class="jm-stat__value">{{ summary.calls ? summary.errRate.toFixed(1) + '%' : '—' }}</span>
      </div>
      <div class="jm-stat">
        <span class="jm-stat__label">平均耗时</span>
        <span class="jm-stat__value">{{ fmtMs(summary.avgMs) }}</span>
      </div>
      <div class="jm-stat">
        <span class="jm-stat__label">任务成功</span>
        <span class="jm-stat__value">{{ fmtNum(summary.tasks) }}</span>
      </div>
    </div>

    <Table
      :columns="columns"
      :data="list"
      :loading="loading"
      :pagination="false"
      size="small"
      row-key="key"
      stripe
    >
      <template #empty>
        <div class="jm-empty">暂无运行数据。插件启用并有调用后，这里会显示钩子耗时与错误。</div>
      </template>
    </Table>
  </Drawer>
</template>

<style scoped>
.jm-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
}

.jm-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-1);
}

.jm-since,
.jm-updated {
  font-size: 12px;
  font-weight: 400;
  color: var(--color-text-3);
}

.jm-updated::before {
  content: '·';
  margin-right: 6px;
}

.jm-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.jm-toolbar__hint {
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-text-3);
}

.jm-summary {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 16px;
}

.jm-stat {
  padding: 10px 12px;
  border: 1px solid var(--color-border-2);
  border-radius: var(--ta-radius);
  background: var(--color-fill-1);
}

.jm-stat__label {
  display: block;
  font-size: 11.5px;
  color: var(--color-text-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.jm-stat__value {
  display: block;
  margin-top: 3px;
  font-size: 18px;
  font-weight: 700;
  color: var(--color-text-1);
  line-height: 1.2;
}

.jm-stat__value em {
  margin-left: 3px;
  font-size: 12px;
  font-style: normal;
  font-weight: 400;
  color: var(--color-text-3);
}

.jm-stat.is-warn .jm-stat__value {
  color: rgb(var(--red-6));
}

.jm-empty {
  padding: 32px 0;
  text-align: center;
  font-size: 13px;
  color: var(--color-text-3);
}

@media (max-width: 900px) {
  .jm-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .jm-toolbar {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
