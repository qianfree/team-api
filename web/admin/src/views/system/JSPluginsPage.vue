<script setup lang="ts">
// JS 任务插件管理页：
// 上传 plugin.js 源码 → 激活版本 → 启用进 generation → /pjs/{key}/... 生效。
// 与 Go 插件管理页（PluginsPage）并列入口；权限：页面 plugin:upload，
// 启停/激活由后端 plugin:manage 校验。
//
// 布局：PageHeader(操作) → 工具栏(状态分段 + 搜索 + 来源 + 视图) → 卡片/表格内容区。
// 卡片、上传、详情、指标各自抽成 components/jsplugin/* 子组件，本页只留数据与编排。
import { computed, h, onMounted, ref } from 'vue'
import { Button, Input, Message, Popconfirm, Select, Space, Tag } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import {
  IconApps, IconBarChart, IconDashboard, IconList, IconRefresh, IconSearch, IconUpload,
} from '@arco-design/web-vue/es/icon'
import PageHeader from '@/components/PageHeader.vue'
import TableStats from '@/components/TableStats.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import JSPluginConfigModal from '@/components/JSPluginConfigModal.vue'
import JSPluginCard from '@/components/jsplugin/JSPluginCard.vue'
import JSPluginUploadModal from '@/components/jsplugin/JSPluginUploadModal.vue'
import JSPluginDetailDrawer from '@/components/jsplugin/JSPluginDetailDrawer.vue'
import JSPluginMetricsDrawer from '@/components/jsplugin/JSPluginMetricsDrawer.vue'
import JSPluginOpsDrawer from '@/components/jsplugin/JSPluginOpsDrawer.vue'
import { fetchModeLabel, pluginStatus, type JSPlugin } from '@/components/jsplugin/pluginMeta'
import request from '@/utils/request'

const loading = ref(false)
const data = ref<JSPlugin[]>([])

// ── 视图与筛选 ────────────────────────────────────────────
type ViewMode = 'card' | 'table'
const viewMode = ref<ViewMode>((localStorage.getItem('jsplugins_view') as ViewMode) || 'card')
function setViewMode(mode: ViewMode) {
  viewMode.value = mode
  localStorage.setItem('jsplugins_view', mode)
}

type StatusFilter = 'all' | 'live' | 'pending' | 'off'
const statusFilter = ref<StatusFilter>('all')
const keyword = ref('')
const originFilter = ref<'all' | 'builtin' | 'upload'>('all')

const statusTabs = computed(() => {
  const counts = { all: data.value.length, live: 0, pending: 0, off: 0 }
  for (const p of data.value) {
    const s = pluginStatus(p)
    counts[s.key]++
  }
  return [
    { value: 'all' as const, label: '全部', count: counts.all },
    { value: 'live' as const, label: '服务中', count: counts.live },
    { value: 'pending' as const, label: '待生效', count: counts.pending },
    { value: 'off' as const, label: '未启用', count: counts.off },
  ]
})

const originOptions = [
  { label: '全部来源', value: 'all' },
  { label: '内置', value: 'builtin' },
  { label: '上传', value: 'upload' },
]

// 列表顺序：服务中 → 待生效 → 未启用，同级按启用版本多的在前、名称升序
function rank(p: JSPlugin): number {
  return pluginStatus(p).key === 'live' ? 0 : pluginStatus(p).key === 'pending' ? 1 : 2
}

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  return data.value
    .filter((p) => {
      if (statusFilter.value !== 'all' && pluginStatus(p).key !== statusFilter.value) return false
      if (originFilter.value !== 'all' && p.origin !== originFilter.value) return false
      if (!kw) return true
      return [p.key, p.name, p.description, p.author, ...(p.models || [])]
        .join(' ')
        .toLowerCase()
        .includes(kw)
    })
    .sort((a, b) => {
      const d = rank(a) - rank(b)
      if (d !== 0) return d
      return (a.name || a.key).localeCompare(b.name || b.key, 'zh-CN')
    })
})

const isFiltering = computed(
  () => !!keyword.value.trim() || originFilter.value !== 'all' || statusFilter.value !== 'all',
)

function resetFilters() {
  keyword.value = ''
  originFilter.value = 'all'
  statusFilter.value = 'all'
}

// ── 数据 ──────────────────────────────────────────────────
async function fetchData() {
  loading.value = true
  try {
    const res: any = await request.get('/admin/js-plugins')
    data.value = res.data?.data?.list || res.data?.list || []
  } catch {
    // 拉取失败保留旧列表：清空会让开着的详情抽屉（以列表对象为数据源）内容闪没
  } finally {
    loading.value = false
  }
}

// ── 生命周期动作 ──────────────────────────────────────────
// 启停防抖：卡片/详情抽屉/表格三处入口可能连点，per-key busy 阻止并发请求
const enablingKeys = ref(new Set<string>())

async function doEnable(key: string, enabled: boolean) {
  if (enablingKeys.value.has(key)) return
  enablingKeys.value.add(key)
  try {
    await request.post(`/admin/js-plugins/${key}/enable`, { enabled })
    Message.success(enabled ? '已启用，路由将在 generation 重载后生效' : '已禁用，路由已摘除')
    fetchData()
  } catch {
    // 拦截器已提示
  } finally {
    enablingKeys.value.delete(key)
  }
}

// ── 弹层状态 ──────────────────────────────────────────────
const showUpload = ref(false)
const showMetrics = ref(false)
const showOps = ref(false)
const showConfigModal = ref(false)
const configPluginKey = ref('')

const detailKey = ref('')
const showDetail = ref(false)
// 详情以列表对象为唯一数据源：列表刷新后抽屉内容自动同步（激活/启停后无需另存副本）
const detailPlugin = computed(() => data.value.find((p) => p.key === detailKey.value) || null)

function openDetail(p: JSPlugin) {
  detailKey.value = p.key
  showDetail.value = true
}

function openConfig(key: string) {
  configPluginKey.value = key
  showConfigModal.value = true
}

// ── 表格列 ────────────────────────────────────────────────
const columns: TableColumnData[] = [
  {
    title: '名称',
    dataIndex: 'key',
    width: 176,
    render({ record }: any) {
      return h('div', { style: 'min-width: 0; line-height: 1.4;' }, [
        h('button', {
          style: 'display:block;max-width:100%;padding:0;border:none;background:none;cursor:pointer;font-family:inherit;font-size:13px;font-weight:500;color:var(--color-text-1);text-align:left;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;',
          onClick: () => openDetail(record),
          title: record.name || record.key,
        }, record.name || record.key),
        h('div', {
          style: 'font-family: JetBrains Mono, Consolas, monospace; font-size: 11.5px; color: var(--color-text-3); overflow:hidden;text-overflow:ellipsis;white-space:nowrap;',
          title: record.key,
        }, record.key),
      ])
    },
  },
  {
    title: '来源',
    dataIndex: 'origin',
    width: 76,
    render({ record }: any) {
      const builtin = record.origin === 'builtin'
      return h('span', {
        style: `display:inline-flex;align-items:center;gap:5px;font-size:12px;color:${builtin ? '#7c3aed' : 'var(--color-text-3)'};`,
      }, [
        h('span', {
          style: `width:5px;height:5px;border-radius:50%;flex-shrink:0;background:${builtin ? '#7c3aed' : 'var(--color-fill-5)'};`,
        }),
        builtin ? '内置' : '上传',
      ])
    },
  },
  {
    title: '描述',
    dataIndex: 'description',
    width: 300,
    ellipsis: true,
    tooltip: true,
    render: ({ record }: any) =>
      h('span', { style: record.description ? '' : 'color:var(--color-text-3);' },
        record.description || '未填写'),
  },
  {
    title: '适用模型',
    dataIndex: 'models',
    width: 186,
    render({ record }: any) {
      const models: string[] = record.models || []
      if (!models.length) return h('span', { style: 'color: var(--color-text-3);' }, '全部模型')
      // 单行 186px：按字符宽度估算法挑选，长名宁可只放一个也不折行
      const picked: string[] = []
      let used = 0
      for (const m of models) {
        const w = m.length * 6.4 + 14
        if (picked.length && used + w > 158) break
        if (w > 158) { picked.push(m); break }
        picked.push(m)
        used += w + 4
        if (picked.length === 2) break
      }
      const rest = models.length - picked.length
      const tags = picked.map((m) => h(Tag, { size: 'small', color: 'arcoblue' }, () => m))
      if (rest > 0) {
        tags.push(h('span', {
          style: 'font-size:12px;color:var(--color-text-3);line-height:20px;flex-shrink:0;',
          title: models.join('、'),
        }, `+${rest}`))
      }
      return h('span', {
        style: 'display:inline-flex;align-items:center;gap:4px;flex-wrap:nowrap;overflow:hidden;max-width:100%;',
      }, tags)
    },
  },
  {
    title: '状态',
    dataIndex: 'enabled',
    width: 88,
    render({ record }: any) {
      const s = pluginStatus(record)
      return h('span', { style: 'display: inline-flex; align-items: center; gap: 6px;' }, [
        h('span', {
          style: `width:6px;height:6px;border-radius:50%;flex-shrink:0;background:${
            s.key === 'live' ? '#10b981' : s.key === 'pending' ? '#f59e0b' : 'var(--color-text-3)'
          }`,
        }),
        h('span', { style: 'font-size: 13px;' }, s.label),
      ])
    },
  },
  {
    title: '激活版本',
    dataIndex: 'active_version',
    width: 112,
    render: ({ record }: any) =>
      h('div', { style: 'line-height: 1.35;' }, [
        h('code', {
          style: 'display:block;font-family: JetBrains Mono, Consolas, monospace; font-size: 12px;color:var(--color-text-1);',
        }, record.active_version || '—'),
        h('span', { style: 'font-size: 11px;color:var(--color-text-3);' },
          `${record.versions?.length || 0} 个版本`),
      ]),
  },
  {
    title: '轮询',
    dataIndex: 'fetch_mode',
    width: 92,
    render: ({ record }: any) => fetchModeLabel(record.fetch_mode),
  },
  {
    title: '操作',
    dataIndex: 'actions',
    width: 176,
    fixed: 'right',
    render({ record }: any) {
      const btns: any[] = []
      btns.push(
        record.enabled
          ? h(Popconfirm, {
            content: `禁用「${record.key}」？路由会被立即摘除。`,
            onOk: () => doEnable(record.key, false),
          }, () => h(Button, { size: 'small', type: 'outline', status: 'warning' }, () => '禁用'))
          : h(Popconfirm, {
            content: record.active_version
              ? `启用「${record.key}」？将挂载 /pjs/${record.key}/ 路由。`
              : `「${record.key}」还没有激活版本，启用后不会生效，仍要启用？`,
            onOk: () => doEnable(record.key, true),
          }, () => h(Button, { size: 'small', type: 'primary' }, () => '启用')),
      )
      btns.push(h(Button, { size: 'small', type: 'text', onClick: () => openDetail(record) }, () => '详情'))
      btns.push(h(Button, { size: 'small', type: 'text', onClick: () => openConfig(record.key) }, () => '配置'))
      return h(Space, { size: 2 }, () => btns)
    },
  },
]

onMounted(fetchData)
</script>

<template>
  <div class="page-table">
    <PageHeader
      title="JS 任务插件"
      description="上传 plugin.js 源码、管理版本与启用状态；启用后路由挂载在 /pjs/{key}/ 下"
    >
      <template #actions>
        <Button type="primary" @click="showUpload = true">
          <template #icon><IconUpload /></template>上传插件
        </Button>
        <Button type="secondary" @click="showMetrics = true">
          <template #icon><IconBarChart /></template>运行指标
        </Button>
        <Button type="secondary" @click="showOps = true">
          <template #icon><IconDashboard /></template>运营统计
        </Button>
        <Button :loading="loading" title="刷新列表" @click="fetchData">
          <template #icon><IconRefresh /></template>
        </Button>
      </template>
    </PageHeader>

    <!-- 工具栏：状态分段（带计数）+ 搜索 + 来源 + 视图切换 -->
    <div class="jp-toolbar">
      <div class="jp-seg">
        <button
          v-for="tab in statusTabs"
          :key="tab.value"
          type="button"
          class="jp-seg__item"
          :class="{ 'is-active': statusFilter === tab.value }"
          @click="statusFilter = tab.value"
        >
          {{ tab.label }}
          <span class="jp-seg__count">{{ tab.count }}</span>
        </button>
      </div>

      <div class="jp-toolbar__right">
        <Input
          v-model="keyword"
          class="jp-search"
          allow-clear
          placeholder="搜索名称 / 标识 / 描述 / 模型"
        >
          <template #prefix><IconSearch /></template>
        </Input>
        <Select
          v-model="originFilter"
          class="jp-origin"
          :options="originOptions"
          placeholder="来源"
        />
        <div class="jp-view">
          <button
            type="button"
            class="jp-view__btn"
            :class="{ 'is-active': viewMode === 'card' }"
            title="卡片视图"
            @click="setViewMode('card')"
          >
            <IconApps />
          </button>
          <button
            type="button"
            class="jp-view__btn"
            :class="{ 'is-active': viewMode === 'table' }"
            title="表格视图"
            @click="setViewMode('table')"
          >
            <IconList />
          </button>
        </div>
      </div>
    </div>

    <!-- 卡片视图 -->
    <div v-if="viewMode === 'card'" class="jp-cardarea">
      <div v-if="loading" class="jp-grid">
        <div v-for="i in 6" :key="i" class="jp-skeleton">
          <div class="jp-skeleton__row">
            <span class="jp-skeleton__avatar" />
            <span class="jp-skeleton__lines">
              <i style="width: 46%" />
              <i style="width: 28%" />
            </span>
          </div>
          <span class="jp-skeleton__block" />
          <span class="jp-skeleton__block" style="width: 62%" />
        </div>
      </div>

      <div v-else-if="filtered.length" class="jp-grid">
        <JSPluginCard
          v-for="p in filtered"
          :key="p.key"
          :plugin="p"
          @detail="openDetail(p)"
          @config="openConfig(p.key)"
          @toggle="(v: boolean) => doEnable(p.key, v)"
        />
      </div>

      <div v-else class="jp-empty">
        <div class="jp-empty__icon"><IconApps :size="26" /></div>
        <p class="jp-empty__title">{{ isFiltering ? '没有匹配的插件' : '还没有 JS 任务插件' }}</p>
        <p class="jp-empty__desc">
          {{ isFiltering ? '换个关键词，或清空筛选条件再看看。' : '上传 plugin.js 源码即可创建第一个插件，上传后默认未启用。' }}
        </p>
        <Button v-if="isFiltering" size="small" @click="resetFilters">清空筛选</Button>
        <Button v-else size="small" type="primary" @click="showUpload = true">上传插件</Button>
      </div>

      <div v-if="!loading && filtered.length" class="jp-footer">
        <TableStats :total="filtered.length">
          <span v-if="isFiltering">已筛选，共 {{ data.length }} 个</span>
          <span v-else>服务中 {{ statusTabs[1].count }} 个</span>
        </TableStats>
      </div>
    </div>

    <!-- 表格视图 -->
    <ACard v-else :bordered="false">
      <ResponsiveTable
        :columns="columns"
        :data="filtered"
        :loading="loading"
        :scroll="{ x: 1206 }"
        :bordered="false"
        :stripe="true"
        row-key="key"
        card-title-key="key"
        card-subtitle-key="description"
        card-badge-key="enabled"
        :card-fields="['origin', 'models', 'active_version', 'fetch_mode']"
      >
        <template #empty>
          <div class="jp-empty jp-empty--inline">
            <p class="jp-empty__title">{{ isFiltering ? '没有匹配的插件' : '还没有 JS 任务插件' }}</p>
            <Button v-if="isFiltering" size="small" @click="resetFilters">清空筛选</Button>
          </div>
        </template>
      </ResponsiveTable>
      <div class="table-footer">
        <TableStats :total="filtered.length">
          <span v-if="isFiltering">已筛选，共 {{ data.length }} 个</span>
          <span v-else>服务中 {{ statusTabs[1].count }} 个</span>
        </TableStats>
      </div>
    </ACard>

    <JSPluginUploadModal v-model:visible="showUpload" @uploaded="fetchData" />

    <JSPluginDetailDrawer
      v-model:visible="showDetail"
      :plugin="detailPlugin"
      @toggle="(v: boolean) => detailPlugin && doEnable(detailPlugin.key, v)"
      @config="detailPlugin && openConfig(detailPlugin.key)"
      @changed="fetchData"
    />

    <JSPluginMetricsDrawer v-model:visible="showMetrics" />

    <JSPluginOpsDrawer v-model:visible="showOps" />

    <JSPluginConfigModal
      v-model:visible="showConfigModal"
      :plugin-key="configPluginKey"
      @saved="fetchData"
    />
  </div>
</template>

<style scoped>
/* ── 工具栏 ─────────────────────────────────────── */
.jp-toolbar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
  padding: 8px 10px;
  border: 1px solid var(--color-border-2);
  border-radius: var(--ta-radius);
  background: var(--color-bg-1);
  box-shadow: var(--ta-shadow-card);
  flex-wrap: wrap;
}

.jp-seg {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 2px;
  border-radius: 8px;
  background: var(--color-fill-2);
}

.jp-seg__item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 11px;
  border: none;
  border-radius: 6px;
  background: transparent;
  cursor: pointer;
  font-size: 12.5px;
  color: var(--color-text-2);
  transition: background var(--ta-duration-fast) var(--ta-ease),
    color var(--ta-duration-fast) var(--ta-ease);
}

.jp-seg__item:hover {
  color: var(--color-text-1);
}

.jp-seg__item.is-active {
  background: var(--color-bg-1);
  color: var(--color-primary-6);
  font-weight: 500;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
}

.jp-seg__count {
  min-width: 18px;
  padding: 0 4px;
  border-radius: 9px;
  background: var(--color-fill-3);
  font-size: 11px;
  line-height: 16px;
  text-align: center;
  color: var(--color-text-3);
  font-variant-numeric: tabular-nums;
}

.jp-seg__item.is-active .jp-seg__count {
  background: var(--color-primary-1);
  color: var(--color-primary-6);
}

.jp-toolbar__right {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
  flex-wrap: nowrap;
}

.jp-search {
  width: 240px;
  flex: 0 0 240px;
}

.jp-origin {
  width: 120px;
  flex: 0 0 120px;
}

.jp-view {
  flex: 0 0 auto;
  display: flex;
  gap: 2px;
  padding: 2px;
  border-radius: 8px;
  background: var(--color-fill-2);
}

.jp-view__btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 24px;
  border: none;
  border-radius: 6px;
  background: transparent;
  cursor: pointer;
  color: var(--color-text-3);
  font-size: 14px;
  transition: background var(--ta-duration-fast) var(--ta-ease),
    color var(--ta-duration-fast) var(--ta-ease);
}

.jp-view__btn:hover {
  color: var(--color-text-1);
}

.jp-view__btn.is-active {
  background: var(--color-bg-1);
  color: var(--color-primary-6);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
}

/* ── 卡片区 ─────────────────────────────────────── */
.jp-cardarea {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.jp-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(330px, 1fr));
  /* 行高必须按内容算：auto 行在高度受限的滚动容器里会被均分，把卡片压扁 */
  grid-auto-rows: max-content;
  gap: 16px;
  align-content: start;
  overflow-y: auto;
  padding-bottom: 2px;
}

.jp-footer {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  padding-top: 12px;
}

.jp-footer :deep(.table-stats) {
  margin-bottom: 0;
}

/* 骨架 */
.jp-skeleton {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border: 1px solid var(--color-border-2);
  border-radius: var(--ta-radius);
  background: var(--color-bg-1);
}

.jp-skeleton__row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.jp-skeleton__avatar {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: var(--color-fill-2);
}

.jp-skeleton__lines {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.jp-skeleton__lines i,
.jp-skeleton__block {
  display: block;
  height: 11px;
  border-radius: 4px;
  background: var(--color-fill-2);
}

.jp-skeleton__block {
  height: 38px;
  border-radius: 8px;
}

.jp-skeleton__avatar,
.jp-skeleton__lines i,
.jp-skeleton__block {
  animation: jp-pulse 1.4s ease-in-out infinite;
}

@keyframes jp-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
}

/* 空态 */
.jp-empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 280px;
  padding: 40px 16px;
}

.jp-empty--inline {
  min-height: 180px;
}

.jp-empty__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 54px;
  height: 54px;
  margin-bottom: 4px;
  border-radius: 16px;
  color: var(--color-primary-6);
  background: var(--color-primary-1);
}

.jp-empty__title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-1);
}

.jp-empty__desc {
  margin: 0 0 6px;
  max-width: 360px;
  text-align: center;
  font-size: 12.5px;
  line-height: 1.65;
  color: var(--color-text-3);
}

/* 行内文字按钮统一到品牌色（Arco 默认 text 按钮是蓝，与 teal 主色不协调） */
.page-table :deep(.arco-btn-text) {
  color: var(--ta-primary);
}

.table-footer {
  flex-shrink: 0;
  padding-top: 12px;
}

/* 移动端：工具栏纵向堆叠，搜索与来源各占一半 */
@media (max-width: 768px) {
  .jp-toolbar {
    flex-direction: column;
    align-items: stretch;
  }
  .jp-toolbar__right {
    width: 100%;
    justify-content: space-between;
  }
  .jp-search {
    flex: 1 1 auto;
    width: auto;
    min-width: 120px;
  }
  .jp-origin {
    flex: 0 0 104px;
    width: 104px;
  }
  .jp-grid {
    grid-template-columns: 1fr;
  }
}
</style>
