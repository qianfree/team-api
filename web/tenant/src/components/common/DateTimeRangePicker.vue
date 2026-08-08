<script setup lang="ts">
import { computed } from 'vue'

// 日期时间范围选择器：开始/结束各一个 datetime-local 输入（支持到分钟精度）。
// 对外暴露 v-model:start / v-model:end，值为后端格式字符串 YYYY-MM-DD HH:mm:ss（结束留空 = 到现在）。
const props = defineProps<{
	start?: string
	end?: string
}>()
const emit = defineEmits<{
	'update:start': [value: string]
	'update:end': [value: string]
}>()

// datetime-local 值（YYYY-MM-DDTHH:mm）与后端值（YYYY-MM-DD HH:mm:ss）互转
function toInputValue(v: string | undefined): string {
	if (!v) return ''
	return v.replace(' ', 'T').slice(0, 16)
}
function toBackendValue(v: string): string {
	if (!v) return ''
	const s = v.replace('T', ' ')
	return s.length === 16 ? s + ':00' : s
}

const startInput = computed({
	get: () => toInputValue(props.start),
	set: (v: string) => emit('update:start', toBackendValue(v)),
})
const endInput = computed({
	get: () => toInputValue(props.end),
	set: (v: string) => emit('update:end', toBackendValue(v)),
})
</script>

<template>
	<div class="flex items-center gap-2">
		<label class="text-sm text-gray-500 whitespace-nowrap">开始时间</label>
		<input v-model="startInput" type="datetime-local" class="input" style="width:190px" />
		<span class="text-gray-400 select-none">~</span>
		<label class="text-sm text-gray-500 whitespace-nowrap">结束时间</label>
		<input v-model="endInput" type="datetime-local" class="input" style="width:190px" />
	</div>
</template>
