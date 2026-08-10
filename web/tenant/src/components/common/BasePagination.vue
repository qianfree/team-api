<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
	modelValue: number
	pageSize: number
	total: number
	pageSizeOptions?: number[]
	showSizeChanger?: boolean
}>(), {
	pageSizeOptions: () => [10, 20, 50, 100],
	showSizeChanger: true,
})

const emit = defineEmits<{
	'update:modelValue': [page: number]
	'update:pageSize': [size: number]
	change: [page: number]
}>()

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
	<n-pagination
		v-if="total > 0"
		:page="modelValue"
		:page-size="pageSize"
		:item-count="total"
		:page-sizes="sizeOptions"
		:show-size-picker="showSizeChanger"
		@update:page="handlePage"
		@update:page-size="handlePageSize"
	/>
</template>