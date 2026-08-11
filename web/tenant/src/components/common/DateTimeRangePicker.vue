<script setup lang="ts">
import { computed } from 'vue'

// 日期时间范围选择器：基于 Naive NDatePicker（datetimerange）+ 快捷选择按钮（今日/昨天/近3天/近一周）。
// 对外暴露 v-model:start / v-model:end，值为后端格式字符串 YYYY-MM-DD HH:mm:ss（结束留空 = 到现在）。
const props = defineProps<{
	start?: string
	end?: string
}>()
const emit = defineEmits<{
	'update:start': [value: string]
	'update:end': [value: string]
}>()

// timestamp(ms) ↔ 后端字符串 YYYY-MM-DD HH:mm:ss
function tsToStr(ts: number): string {
	const d = new Date(ts)
	const pad = (n: number) => String(n).padStart(2, '0')
	return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
function strToTs(v: string | undefined): number | null {
	if (!v) return null
	const t = new Date(v.replace(' ', 'T')).getTime()
	return Number.isNaN(t) ? null : t
}

// NDatePicker 显示值：仅当 start/end 均有效时返回 [s, e]
const rangeValue = computed<[number, number] | null>(() => {
	const s = strToTs(props.start)
	const e = strToTs(props.end)
	return s != null && e != null ? [s, e] : null
})

function onRangeChange(v: [number, number] | null) {
	if (v) {
		emit('update:start', tsToStr(v[0]))
		emit('update:end', tsToStr(v[1]))
	} else {
		emit('update:start', '')
		emit('update:end', '')
	}
}

// 以本地时区"天"为粒度的快捷范围
function dayStart(offsetDays: number): number {
	const d = new Date()
	d.setDate(d.getDate() + offsetDays)
	d.setHours(0, 0, 0, 0)
	return d.getTime()
}
function dayEnd(offsetDays: number): number {
	const d = new Date()
	d.setDate(d.getDate() + offsetDays)
	d.setHours(23, 59, 59, 999)
	return d.getTime()
}

// NDatePicker 面板内的快捷选项（点击后自动设置 start/end）
// 键为面板内显示的标签，值为 [startTs, endTs] 或返回该元组的函数
const shortcuts = {
	今日: () => [dayStart(0), dayEnd(0)] as [number, number],
	昨天: () => [dayStart(-1), dayEnd(-1)] as [number, number],
	近3天: () => [dayStart(-2), dayEnd(0)] as [number, number],
	近一周: () => [dayStart(-6), dayEnd(0)] as [number, number],
}
</script>

<template>
	<n-date-picker
		type="datetimerange"
		:value="rangeValue"
		:shortcuts="shortcuts"
		:clearable="true"
		:actions="['clear', 'confirm']"
		style="width: 400px"
		@update:value="onRangeChange"
	/>
</template>