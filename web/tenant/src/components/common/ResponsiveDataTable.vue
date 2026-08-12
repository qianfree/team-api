<script setup lang="ts">
import { computed, defineComponent } from 'vue'
import type { PropType, VNodeChild } from 'vue'
import type { DataTableColumn, DataTableColumns, PaginationProps } from 'naive-ui'
import BasePagination from '@/components/common/BasePagination.vue'
import Icon from '@/components/common/Icon.vue'
import { useIsMobile } from '@/composables/useIsMobile'

/**
 * 响应式数据列表：桌面端（≥ breakpoint）渲染 NDataTable，移动端渲染卡片列表。
 * 卡片字段 / 标题 / 徽章 / 副标题 / 操作列全部从 columns 派生，并复用各列 render 函数，
 * 因此页面只需把 <n-data-table> 换成 <ResponsiveDataTable> 并配置几个卡片 key 即可。
 *
 * 页面原有 pagination / loading / #empty 插槽的用法保持不变：
 *   <ResponsiveDataTable
 *     remote v-model:page="page" v-model:page-size="pageSize"
 *     :item-count="total" :columns="columns" :data="rows"
 *     :row-key="(row) => row.id"
 *     card-title-key="model" card-badge-key="status"
 *     card-subtitle-key="created_at"
*      :card-fields="[{ key: 'token', full: true }, 'cost']"
 *     card-actions-key="actions" :row-click="openDetail"
 *   >
 *     <template #empty>...</template>
 *   </ResponsiveDataTable>
 */

// 模板里无法直接输出 render 函数返回的 VNode，用一个小渲染组件承接
const VNodeRenderer = defineComponent({
	name: 'VNodeRenderer',
	props: {
		node: {
			type: [Object, String, Number, Boolean, Array] as PropType<VNodeChild>,
			required: true,
		},
	},
	setup(props) {
		return () => props.node
	},
})

/** 卡片网格字段：key 对应列，full=true 时占满整行 */
interface CardField {
	key: string
	title?: string
	full?: boolean
}

const props = withDefaults(
	defineProps<{
		// ---------- 透传给 NDataTable ----------
		columns: DataTableColumns<any>
		data: any[]
		loading?: boolean
		rowKey: (row: any) => string | number
		remote?: boolean
		itemCount?: number
		page?: number
		pageSize?: number
		pageSizes?: number[]
			showSizePicker?: boolean
			scrollX?: number
			showPagination?: boolean // false 时桌面端和移动端均不显示分页（用于固定数量列表）
		// ---------- 卡片模式配置 ----------
		cardTitleKey?: string // 卡片主标题列 key
		cardBadgeKey?: string // 卡片头部右侧徽章列 key（如 status）
		cardSubtitleKey?: string // 主标题下副标题列 key（如 created_at）
		cardFields?: (CardField | string)[] // 卡片网格字段；缺省取除 title/badge/subtitle/actions 外全部列
		cardActionsKey?: string // 操作列 key，渲染在卡片底部（默认 'actions'）
		rowClick?: (row: any) => void // 提供后整张卡片可点击
			breakpoint?: number // 卡片切换断点（默认 1024 = lg，与 viewport-table 桌面布局一致）
		fillHeight?: boolean // 桌面端让表格撑满父容器高度、body 内部滚动（配合 viewport-table-panel 使用）
	}>(),
	{
		loading: false,
		remote: true,
		itemCount: 0,
		page: 1,
		pageSize: 20,
		pageSizes: () => [10, 20, 50, 100],
			showSizePicker: true,
			scrollX: 0,
			showPagination: true,
		cardActionsKey: 'actions',
			breakpoint: 1024,
		fillHeight: false,
	},
)

const emit = defineEmits<{
	'update:page': [page: number]
	'update:page-size': [size: number]
}>()

const { isMobile } = useIsMobile(props.breakpoint)

// 各区域对应列
const titleCol = computed(() => props.columns.find((c) => c.key === props.cardTitleKey))
const badgeCol = computed(() => props.columns.find((c) => c.key === props.cardBadgeKey))
const subtitleCol = computed(() => props.columns.find((c) => c.key === props.cardSubtitleKey))
const actionsCol = computed(() => props.columns.find((c) => c.key === props.cardActionsKey))

// NDataTable 的分页状态必须完整放在 pagination 对象中，顶层同名属性不会生效。
const paginationProp = computed<false | PaginationProps>(() => {
	if (!props.showPagination) return false
	return {
		page: props.page,
		pageSize: props.pageSize,
		itemCount: props.itemCount,
		pageSizes: props.pageSizes,
		showSizePicker: props.showSizePicker,
		prefix: ({ itemCount }) => `共 ${itemCount ?? 0} 条`,
	}
})

function desktopRowProps(row: any) {
	if (!props.rowClick) return {}
	return {
		style: 'cursor: pointer',
		onClick: (event: MouseEvent) => {
			const target = event.target as HTMLElement | null
			if (target?.closest('button, a, input, select, textarea, [role="button"]')) return
			props.rowClick?.(row)
		},
	}
}

// 网格字段：按 cardFields 顺序取，未指定则用除标题/徽章/副标题/操作外的全部列
const fields = computed(() => {
	const keys = props.cardFields?.length
		? props.cardFields.map((f) => (typeof f === 'string' ? { key: f } : f))
		: props.columns
			.filter((c) => ![props.cardTitleKey, props.cardBadgeKey, props.cardSubtitleKey, props.cardActionsKey].includes(c.key))
			.map((c) => ({ key: c.key }))

	return keys
		.map((f) => {
			const col = props.columns.find((c) => c.key === f.key) || null
			return {
				key: f.key,
				full: !!f.full,
				title: f.title ?? col?.title ?? f.key,
				col,
			}
		})
		.filter((f) => f.col)
})

// 优先用列 render 输出，否则取原始值
function renderCell(row: any, col: DataTableColumn<any> | null): VNodeChild {
	if (col?.render) return col.render(row)
	const v = row?.[col?.key ?? '']
	if (v == null || v === '') return '--'
	return String(v)
}
</script>

<template>
	<!-- 桌面端（≥ breakpoint）：NDataTable 原样透传 -->
	<n-data-table
		v-if="!isMobile"
		:flex-height="fillHeight"
		:class="{ 'flex-1 min-h-0': fillHeight }"
		:remote="remote"
		:pagination="paginationProp"
		:loading="loading"
		:columns="columns"
		:scroll-x="scrollX"
		:data="data"
		:row-key="rowKey"
		:row-props="desktopRowProps"
		@update:page="(p: number) => emit('update:page', p)"
		@update:page-size="(s: number) => emit('update:page-size', s)"
	>
		<template #empty>
			<slot name="empty" />
		</template>
	</n-data-table>

	<!-- 移动端：每行 = 一张卡片 -->
	<div v-else class="space-y-3">
		<!-- 加载中 -->
		<div v-if="loading" class="flex justify-center py-6">
			<div class="spinner h-5 w-5 text-primary-500"></div>
		</div>

		<!-- 空状态 -->
		<div v-else-if="data.length === 0">
			<slot name="empty">
				<div class="empty-state">
					<p class="empty-state-title">暂无数据</p>
				</div>
			</slot>
		</div>

		<template v-else>
			<div
				v-for="row in data"
				:key="rowKey(row)"
				class="card p-4"
				:class="{ 'cursor-pointer transition-transform active:scale-[0.995]': rowClick }"
				@click="rowClick && rowClick(row)"
			>
				<!-- 卡片头部：主标题 + 副标题 + 状态徽章 + 可点击箭头 -->
				<div class="flex items-start justify-between gap-3">
					<div class="min-w-0 flex-1">
						<VNodeRenderer v-if="titleCol" :node="renderCell(row, titleCol)" />
						<VNodeRenderer
							v-if="subtitleCol && subtitleCol.key !== titleCol?.key"
							:node="renderCell(row, subtitleCol)"
						/>
					</div>
					<div v-if="badgeCol" class="flex flex-shrink-0 items-center">
						<VNodeRenderer :node="renderCell(row, badgeCol)" />
					</div>
					<Icon
						v-if="rowClick"
						name="chevronRight"
						size="sm"
						class="mt-0.5 flex-shrink-0 text-gray-300"
					/>
				</div>

				<!-- 字段网格：label:value，full 列占整行 -->
				<dl v-if="fields.length" class="mt-3 grid grid-cols-2 gap-x-3 gap-y-2.5">
					<div v-for="f in fields" :key="f.key" class="min-w-0" :class="{ 'col-span-2': f.full }">
						<dt class="text-xs text-gray-400">{{ f.title }}</dt>
						<dd class="mt-0.5">
							<VNodeRenderer :node="renderCell(row, f.col)" />
						</dd>
					</div>
				</dl>

				<!-- 操作列：卡片底部操作栏 -->
				<div v-if="actionsCol" class="mt-3 flex items-center justify-end gap-2 border-t border-gray-100 pt-3" @click.stop>
					<VNodeRenderer :node="renderCell(row, actionsCol)" />
				</div>
			</div>

			<!-- 分页 -->
			<div v-if="showPagination && itemCount > 0" class="flex justify-center pt-1">
				<BasePagination
					:model-value="page"
					:page-size="pageSize"
					:total="itemCount"
					:page-size-options="pageSizes"
					:show-size-changer="showSizePicker"
					@update:model-value="(p: number) => emit('update:page', p)"
					@update:page-size="(s: number) => emit('update:page-size', s)"
				/>
			</div>
		</template>
	</div>
</template>
