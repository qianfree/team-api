<script setup lang="ts">
import { computed, defineComponent, useSlots } from 'vue'
import type { PropType, VNodeChild } from 'vue'
import type { TableColumnData } from '@arco-design/web-vue'
import { useIsMobile } from '@/composables/useIsMobile'

/**
 * 响应式数据表格：桌面端（≥ breakpoint）渲染 ATable，移动端渲染卡片列表。
 * 卡片字段 / 标题 / 徽章 / 副标题 / 操作列全部从 columns 派生，并复用各列 render 函数，
 * 因此页面只需把 <a-table> 换成 <ResponsiveTable> 并配置几个卡片 key 即可。
 *
 * 用法：
 *   <ResponsiveTable
 *     :columns="columns" :data="data" :loading="loading"
 *     :scroll="{ x: 1500 }" row-key="id"
 *     :row-selection="{ type: 'checkbox', showCheckedAll: true }"
 *     :selected-keys="selectedRowKeys"
 *     @selection-change="handleSelectionChange"
 *     card-title-key="model_name" card-subtitle-key="model_id"
 *     card-badge-key="status"
 *     :card-fields="['category', 'pricing', 'max_context_tokens', 'channels']"
 *   />
 *
 * 分页栏由页面自行渲染在组件外部（桌面端/移动端均可见），组件不接管分页。
 * 多选（row-selection）仅在桌面端生效；移动端卡片不渲染勾选框。
 *
 * 移动端卡片定制（作用域插槽，页面未提供时用默认布局）：
 *   #card-header="{ row }"              覆盖整张卡片头部（默认：标题+副标题+徽章）
 *   #card-field-{dataIndex}="{ row }"   覆盖某个字段的卡片渲染（默认：复用该列 render）
 *   #card-extra="{ row }"               在字段网格后、操作栏前插入额外内容（默认：无）
 *   #card-actions="{ row }"             覆盖卡片底部操作栏（默认：操作列 render）
 */

// 模板里无法直接输出 render 函数返回的 VNode，用一个小渲染组件承接
const VNodeRenderer = defineComponent({
  name: 'VNodeRenderer',
  props: {
    // 列 render 可能合法返回 null（VNodeChild 允许 null），故不限定运行时 type、不 required，
    // 否则 Vue 会对 null 触发 prop 校验告警。default:null 仅给运行时兜底，不影响渲染（null 即空）。
    node: { default: null } as PropType<VNodeChild>,
  },
  setup(props) {
    return () => props.node
  },
})

/** 卡片网格字段：key 对应列的 dataIndex，full=true 时占满整行 */
interface CardField {
  key: string
  title?: string
  full?: boolean
}

const props = withDefaults(
  defineProps<{
    // ---------- 透传给 ATable ----------
    columns: TableColumnData[]
    data: any[]
    loading?: boolean
    rowKey?: string
    scroll?: Record<string, number>
    bordered?: boolean
    stripe?: boolean
    size?: 'small' | 'medium' | 'large'
    // 多选（仅桌面端生效；移动端卡片不渲染勾选）
    rowSelection?: Record<string, any>
    selectedKeys?: (string | number)[]
    // ---------- 卡片模式配置 ----------
    cardTitleKey?: string // 卡片主标题列 dataIndex
    cardBadgeKey?: string // 卡片头部右侧徽章列 dataIndex（如 status）
    cardSubtitleKey?: string // 主标题下副标题列 dataIndex
    cardFields?: (CardField | string)[] // 卡片网格字段；缺省取除 标题/徽章/副标题/操作 外的全部列
    cardActionsKey?: string // 操作列 dataIndex，渲染在卡片底部（默认 'actions'）
    rowClick?: (row: any) => void // 提供后整张卡片可点击（仅移动端；桌面端用 row-click 事件）
    rowClass?: (row: any) => string // 自定义行类名（桌面端 ATable row-class，如高亮当前会话）
    breakpoint?: number // 卡片切换断点（默认 768 = md）
  }>(),
  {
    loading: false,
    rowKey: 'id',
    bordered: false,
    stripe: false,
    cardActionsKey: 'actions',
    breakpoint: 768,
  },
)

const emit = defineEmits<{
  'selection-change': [keys: (string | number)[]]
}>()

const isMobile = useIsMobile(props.breakpoint)

// 暴露给模板，用于检测页面是否提供了卡片自定义插槽（如操作栏插槽）
const slots = useSlots()

// 桌面端透传给内部 ATable 的具名 slot：取页面传入的所有 slot，排除移动端专用的 card-* 与已单独处理的 empty。
// 这样 slotName 列（如 frozenStatus/frozenAction）在桌面端 ATable 仍能正常渲染，移动端则走 card-* 插槽。
const tableSlotNames = computed(() =>
  Object.keys(slots).filter((n) => !n.startsWith('card-') && n !== 'empty'),
)

// 各区域对应列（Arco 列用 dataIndex 标识）
const titleCol = computed(() => props.columns.find((c) => c.dataIndex === props.cardTitleKey))
const badgeCol = computed(() => props.columns.find((c) => c.dataIndex === props.cardBadgeKey))
const subtitleCol = computed(() => props.columns.find((c) => c.dataIndex === props.cardSubtitleKey))
const actionsCol = computed(() => props.columns.find((c) => c.dataIndex === props.cardActionsKey))

// 桌面端行类名：合并页面自定义 rowClass + 可点击时的手型
const rowClassHandler = (record: any) => {
  const cls = typeof props.rowClass === 'function' ? props.rowClass(record) || '' : ''
  return [cls, props.rowClick ? 'cursor-pointer' : ''].filter(Boolean).join(' ') || undefined
}

// 网格字段：按 cardFields 顺序取；未传 cardFields（undefined）时用除 标题/徽章/副标题/操作 外的全部列。
// 注意：显式传 :card-fields="[]" 表示「不要字段网格」，必须与「未传」区分，不能按 truthy 判断。
const fields = computed(() => {
  const buildField = (f: { key: string; full?: boolean; title?: string }) => {
    const col = props.columns.find((c) => c.dataIndex === f.key) || null
    return {
      key: f.key,
      full: !!f.full,
      title: f.title ?? (col?.title as string | undefined) ?? f.key,
      col,
    }
  }
  if (props.cardFields === undefined) {
    const reserved = [props.cardTitleKey, props.cardBadgeKey, props.cardSubtitleKey, props.cardActionsKey]
    return props.columns
      .filter((c) => c.dataIndex && !reserved.includes(c.dataIndex))
      .map((c) => ({ key: c.dataIndex as string }))
      .map(buildField)
  }
  return props.cardFields
    .map((f) => (typeof f === 'string' ? { key: f } : f))
    .map(buildField)
    .filter((f) => f.col)
})

// 优先用列 render 输出（Arco render 签名 render({ record })），否则取原始值
function renderCell(row: any, col: TableColumnData | null): VNodeChild {
  if (col?.render) {
    return (col.render as any)({ record: row })
  }
  const v = row?.[col?.dataIndex ?? '']
  if (v == null || v === '') return '--'
  return String(v)
}

// 桌面端整行点击（避开按钮/链接等可交互元素）
function onRowClick(record: any, ev: MouseEvent) {
  if (!props.rowClick) return
  const target = ev.target as HTMLElement | null
  if (target?.closest('button, a, input, select, textarea, [role="button"]')) return
  props.rowClick(record)
}
</script>

<template>
  <!-- 桌面端（≥ breakpoint）：ATable 原样透传，分页由外部控制 -->
  <ATable
    v-if="!isMobile"
    :columns="columns"
    :data="data"
    :loading="loading"
    :scroll="scroll"
    :bordered="bordered"
    :stripe="stripe"
    :size="size"
    :pagination="false"
    :row-key="rowKey"
    :row-selection="rowSelection"
    :selected-keys="selectedKeys"
    :row-class="rowClassHandler"
    @selection-change="(keys: any) => emit('selection-change', keys)"
    @row-click="({ record, ev }: { record: any; ev: MouseEvent }) => onRowClick(record, ev)"
  >
    <template v-for="name in tableSlotNames" :key="name" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps ?? {}" />
    </template>
    <template #empty>
      <slot name="empty" />
    </template>
  </ATable>

  <!-- 移动端：每行 = 一张卡片 -->
  <div v-else class="space-y-3">
    <!-- 加载中 -->
    <div v-if="loading" class="flex justify-center py-6">
      <ASpin />
    </div>

    <!-- 空状态 -->
    <div v-else-if="data.length === 0">
      <slot name="empty">
        <AEmpty />
      </slot>
    </div>

    <template v-else>
      <div
        v-for="row in data"
        :key="row[rowKey]"
        class="rounded-xl border border-[var(--color-border-2)] bg-[var(--color-bg-2)] p-4"
        :class="{ 'cursor-pointer transition-transform active:scale-[0.995]': rowClick }"
        @click="rowClick && onRowClick(row, $event)"
      >
        <!-- 卡片头部：可被 #card-header 覆盖，未提供时用默认三段式 -->
        <slot name="card-header" :row="row">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <div v-if="titleCol" class="truncate text-base font-semibold text-[var(--color-text-1)]">
                <VNodeRenderer :node="renderCell(row, titleCol)" />
              </div>
              <div
                v-if="subtitleCol && subtitleCol.dataIndex !== titleCol?.dataIndex"
                class="mt-0.5 truncate text-xs text-[var(--color-text-3)]"
              >
                <VNodeRenderer :node="renderCell(row, subtitleCol)" />
              </div>
            </div>
            <div v-if="badgeCol" class="flex flex-shrink-0 items-center">
              <VNodeRenderer :node="renderCell(row, badgeCol)" />
            </div>
          </div>
        </slot>

        <!-- 字段网格：每个字段可被 #card-field-{dataIndex} 覆盖，否则用列 render -->
        <dl v-if="fields.length" class="mt-3 grid grid-cols-2 gap-x-3 gap-y-2.5">
          <div v-for="f in fields" :key="f.key" class="min-w-0" :class="{ 'col-span-2': f.full }">
            <dt class="text-xs text-[var(--color-text-3)]">{{ f.title }}</dt>
            <dd class="mt-0.5 text-sm text-[var(--color-text-1)]">
              <slot :name="`card-field-${f.key}`" :row="row" :field="f">
                <VNodeRenderer :node="renderCell(row, f.col)" />
              </slot>
            </dd>
          </div>
        </dl>

        <!-- 额外内容：字段网格后插入，由 #card-extra 提供，无默认不渲染 -->
        <slot name="card-extra" :row="row" />

        <!-- 操作栏：可被 #card-actions 覆盖，否则用操作列 render -->
        <div
          v-if="actionsCol || slots['card-actions']"
          class="mt-3 flex items-center justify-end gap-2 border-t border-[var(--color-fill-2)] pt-3"
          @click.stop
        >
          <slot name="card-actions" :row="row">
            <VNodeRenderer :node="renderCell(row, actionsCol)" />
          </slot>
        </div>
      </div>
    </template>
  </div>
</template>
