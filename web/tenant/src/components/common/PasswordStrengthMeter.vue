<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
	password?: string
}>()

// 强度档位：弱 / 中等 / 强 / 很强，颜色遵循租户端设计规范语义色
const levels = [
	{ label: '弱', bar: 'bg-red-500', text: 'text-red-500' },
	{ label: '中等', bar: 'bg-amber-500', text: 'text-amber-600' },
	{ label: '强', bar: 'bg-primary-500', text: 'text-primary-600' },
	{ label: '很强', bar: 'bg-emerald-500', text: 'text-emerald-600' },
] as const

// 评分规则：长度 + 字符多样性，映射到 1~4 档；空密码返回 0（不渲染）
const score = computed(() => {
	const pw = props.password ?? ''
	if (!pw) return 0

	let s = 0
	if (pw.length >= 8) s++
	if (pw.length >= 12) s++
	if (pw.length >= 16) s++
	if (/[a-zA-Z]/.test(pw)) s++
	if (/\d/.test(pw)) s++
	if (/[^a-zA-Z0-9]/.test(pw)) s++

	if (s <= 1) return 1
	if (s <= 3) return 2
	if (s <= 4) return 3
	return 4
})

const current = computed(() => levels[score.value - 1])
</script>

<template>
	<div v-if="score > 0" class="mt-1.5 flex items-center gap-2">
		<div class="flex flex-1 gap-1">
			<div
				v-for="i in 4"
				:key="i"
				class="h-1 flex-1 rounded-full transition-colors duration-200"
				:class="i <= score ? current.bar : 'bg-gray-200'"
			/>
		</div>
		<span class="w-8 text-right text-xs font-medium" :class="current.text">
			{{ current.label }}
		</span>
	</div>
</template>
