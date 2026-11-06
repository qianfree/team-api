<script setup lang="ts">
// JS 插件运营统计抽屉：按天窗口（7/30/90，默认 30）聚合每插件的任务量/成功率/营收。
// 营收来自 bil_usage_logs 聚合（本位币，实扣优先口径），排序营收降序由后端完成；
// 前端只负责窗口切换重新拉取与展示（成功率百分比、本位币金额沿用全站格式化）。
import { h, ref, watch } from 'vue'
import { Button, Drawer, Tag } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import { IconRefresh } from '@arco-design/web-vue/es/icon'
import request from '@/utils/request'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import { formatBilling } from '@/composables/useCurrency'
import { useIsMobile } from '@/composables/useIsMobile'
import type { JSPluginOpsStatsItem } from './pluginMeta'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{ (e: 'update:visible', v: boolean): void }>()

const list = ref<JSPluginOpsStatsItem[]>([])
const days = ref(30)
const loading = ref(false)
const isMobile = useIsMobile()

const dayOptions = [7, 30, 90]

async function fetchData() {
  loading.value = true
  try {
    const res: any = await request.get('/admin/js-plugins/ops-stats', {
      params: { days: days.value },
    })
    list.value = res.data?.data?.list || res.data?.list || []
  } catch {
    // 拉取失败保留旧数据：清空会让开着的抽屉内容闪没
  } finally {
    loading.value = false
  }
}

// 切窗口天数重新拉取（同值点击不重复请求）
function setDays(v: number) {
  if (days.value === v) return
  days.value = v
  fetchData()
}

watch(
  () => props.visible,
  (v) => {
    if (v) fetchData()
  },
)

function fmtNum(v: number): string {
  return (v || 0).toLocaleString('zh-CN')
}

/** 成功率：无终态任务（success+failed=0）无意义，显示占位符 */
function renderSuccessRate(r: JSPluginOpsStatsItem) {
  const terminal = (r.tasks_success || 0) + (r.tasks_failed || 0)
  if (!terminal) return h('span', { style: 'color: var(--color-text-3);' }, '—')
  const pct = (r.success_rate || 0) * 100
  // 低于 50% 标红（沿用运行指标抽屉的错误率着色先例）
  const style = pct < 50 ? 'color: rgb(var(--red-6)); font-weight: 600;' : ''
  return h('span', { style }, `${pct.toFixed(1)}%`)
}

const columns: TableColumnData[] = [
  {
    title: '插件',
    dataIndex: 'plugin_key',
    width: 190,
    render: ({ record }: any) => {
      const r = record as JSPluginOpsStatsItem
      const tags: any[] = []
      if (r.orphan) tags.push(h(Tag, { size: 'small' }, () => '已无版本'))
      else tags.push(h(Tag, { size: 'small', color: r.enabled ? 'green' : 'gray' }, () => (r.enabled ? '服务中' : '未启用')))
      return h('div', { style: 'min-width: 0; line-height: 1.4;' }, [
        h('div', { style: 'display: flex; align-items: center; gap: 6px; min-width: 0;' }, [
          h('span', {
            style: 'font-size: 13px; font-weight: 500; color: var(--color-text-1); overflow: hidden; text-overflow: ellipsis; white-space: nowrap;',
            title: r.name || r.plugin_key,
          }, r.name || r.plugin_key),
          ...tags,
        ]),
        h('code', {
          style: 'font-family: JetBrains Mono, Consolas, monospace; font-size: 11.5px; color: var(--color-text-3);',
          title: r.plugin_key,
        }, r.plugin_key),
      ])
    },
  },
  {
    title: '任务量',
    dataIndex: 'tasks_total',
    width: 118,
    align: 'right',
    render: ({ record }: any) => {
      const r = record as JSPluginOpsStatsItem
      const nodes: any[] = [
        h('span', { style: 'font-variant-numeric: tabular-nums;' }, fmtNum(r.tasks_total || 0)),
      ]
      if ((r.tasks_processing || 0) > 0) {
        nodes.push(h('span', {
          style: 'font-size: 11.5px; color: var(--color-text-3);',
          title: '窗口内尚未到终态的任务数',
        }, `（处理中 ${fmtNum(r.tasks_processing)}）`))
      }
      return h('span', { style: 'white-space: nowrap;' }, nodes)
    },
  },
  {
    title: '成功',
    dataIndex: 'tasks_success',
    width: 90,
    align: 'right',
    render: ({ record }: any) =>
      h('span', { style: 'font-variant-numeric: tabular-nums;' }, fmtNum(record.tasks_success || 0)),
  },
  {
    title: '失败',
    dataIndex: 'tasks_failed',
    width: 90,
    align: 'right',
    render: ({ record }: any) =>
      h('span', {
        style: (record.tasks_failed || 0) > 0
          ? 'color: rgb(var(--red-6)); font-weight: 600; font-variant-numeric: tabular-nums;'
          : 'color: var(--color-text-3); font-variant-numeric: tabular-nums;',
      }, fmtNum(record.tasks_failed || 0)),
  },
  {
    title: '成功率',
    dataIndex: 'success_rate',
    width: 96,
    align: 'right',
    render: ({ record }: any) => renderSuccessRate(record as JSPluginOpsStatsItem),
  },
  {
    title: '营收',
    dataIndex: 'revenue',
    width: 140,
    align: 'right',
    render: ({ record }: any) =>
      h('span', { style: 'font-variant-numeric: tabular-nums;' }, formatBilling(record.revenue || 0)),
  },
  {
    title: '最近任务时间',
    dataIndex: 'last_task_time',
    width: 170,
    render: ({ record }: any) =>
      h('span', {
        style: record.last_task_time ? '' : 'color: var(--color-text-3);',
      }, record.last_task_time || '—'),
  },
]
</script>

<template>
  <Drawer
    :visible="visible"
    :width="isMobile ? '100%' : 1000"
    :footer="false"
    unmount-on-close
    @cancel="emit('update:visible', false)"
  >
    <template #title>
      <div class="jo-head">
        <span class="jo-title">运营统计</span>
        <span class="jo-sub">最近 {{ days }} 天</span>
      </div>
    </template>

    <div class="jo-toolbar">
      <ARadioGroup type="button" size="small" :model-value="days" @change="(v: any) => setDays(Number(v))">
        <ARadio v-for="d in dayOptions" :key="d" :value="d">{{ d }}天</ARadio>
      </ARadioGroup>
      <span class="jo-toolbar__hint">
        任务量/成功率来自任务记录，营收来自用量账单聚合（本位币）；无任务的插件也列出零值行。
      </span>
      <Button size="small" :loading="loading" @click="fetchData">
        <template #icon><IconRefresh /></template>刷新
      </Button>
    </div>

    <ResponsiveTable
      :columns="columns"
      :data="list"
      :loading="loading"
      :scroll="{ x: 894 }"
      :stripe="true"
      size="small"
      row-key="plugin_key"
      card-title-key="plugin_key"
      :card-fields="['tasks_total', 'tasks_success', 'tasks_failed', 'success_rate', 'revenue', 'last_task_time']"
    >
      <template #empty>
        <div class="jo-empty">窗口内暂无插件任务记录。插件被调用并产生任务后，这里会显示任务量与营收。</div>
      </template>
    </ResponsiveTable>
  </Drawer>
</template>

<style scoped>
.jo-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
}

.jo-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-1);
}

.jo-sub {
  font-size: 12px;
  font-weight: 400;
  color: var(--color-text-3);
}

.jo-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}

.jo-toolbar__hint {
  flex: 1;
  min-width: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-text-3);
}

.jo-empty {
  padding: 32px 0;
  text-align: center;
  font-size: 13px;
  color: var(--color-text-3);
}

@media (max-width: 900px) {
  .jo-toolbar {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
