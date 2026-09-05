<script setup lang="ts">
import { computed } from 'vue'
import { useIsMobile } from '@/composables/useIsMobile'

const props = withDefaults(defineProps<{
	modelValue: number
	pageSize: number
	total: number
	pageSizeOptions?: number[]
	showSizeChanger?: boolean
	pageSlot?: number // 显示的页码个数（默认 7，比 naive-ui 默认的 9 更紧凑）
}>(), {
	pageSizeOptions: () => [10, 20, 50, 100],
	showSizeChanger: true,
	pageSlot: 7,
})

const emit = defineEmits<{
	'update:modelValue': [page: number]
	'update:pageSize': [size: number]
	change: [page: number]
}>()

const { isMobile } = useIsMobile()

// 当前 pageSize 不在候选列表时兜底追加，避免 select 找不到选中项
const sizeOptions = computed(() => {
	const opts = [...props.pageSizeOptions]
	if (!opts.includes(props.pageSize)) opts.push(props.pageSize)
	return opts.sort((a, b) => a - b)
})

function handlePage(page: number) {
	emit('update:modelValue', page)
	emit('change', page)
}

// 切换每页条数后回到第一页并触发刷新（与旧实现语义一致）
function handlePageSize(size: number) {
	if (!Number.isFinite(size) || size === props.pageSize) return
	emit('update:pageSize', size)
	emit('update:modelValue', 1)
	emit('change', 1)
}
</script>

<template>
	<div v-if="total > 0" class="flex w-full min-w-0 flex-col items-center justify-center gap-2 sm:flex-row">
		<span v-if="isMobile" class="text-xs text-gray-500">共 {{ total }} 条</span>
		<n-pagination
			class="max-w-full"
			:page="modelValue"
			:page-size="pageSize"
			:item-count="total"
			:page-sizes="sizeOptions"
			:page-slot="pageSlot"
			:simple="isMobile"
			:show-size-picker="showSizeChanger && !isMobile"
			:show-quick-jumper="!isMobile"
			@update:page="handlePage"
			@update:page-size="handlePageSize"
		/>
	</div>
</template>
