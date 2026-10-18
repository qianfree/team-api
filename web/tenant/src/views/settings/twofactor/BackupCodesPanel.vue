<script setup lang="ts">
import { toast } from '@/utils/toast'

const props = defineProps<{ codes: string[] }>()

async function copyText(text: string, label = '已复制') {
	try {
		await navigator.clipboard.writeText(text)
		toast.success(label)
	} catch {
		toast.error('复制失败，请手动选择复制')
	}
}

// 格子是 div 模拟的按钮，键盘 Enter/Space 必须与 click 等价，否则键盘/读屏用户无法复制单个恢复码
function copyCode(codeItem: string) {
	copyText(codeItem, '恢复码已复制')
}

function downloadCodes() {
	const content = [
		'Team-API 租户控制台 · 两步验证恢复码',
		'每个恢复码只能使用一次，请妥善保存',
		'',
		...props.codes,
		'',
	].join('\n')
	const blob = new Blob([content], { type: 'text/plain;charset=utf-8' })
	const url = URL.createObjectURL(blob)
	const link = document.createElement('a')
	link.href = url
	link.download = 'team-api-backup-codes.txt'
	link.click()
	URL.revokeObjectURL(url)
}
</script>

<template>
	<div>
		<div class="grid grid-cols-1 sm:grid-cols-2 gap-2 mb-3">
			<div
				v-for="codeItem in props.codes"
				:key="codeItem"
				class="px-3 py-2 rounded-lg border border-gray-200 bg-gray-50 text-center font-mono text-sm tracking-wide cursor-pointer select-all hover:border-primary-400 hover:bg-primary-50/50 transition-colors"
				role="button"
				tabindex="0"
				:aria-label="`复制恢复码 ${codeItem}`"
				@click="copyCode(codeItem)"
				@keydown.enter.prevent="copyCode(codeItem)"
				@keydown.space.prevent="copyCode(codeItem)"
			>
				{{ codeItem }}
			</div>
		</div>
		<div class="flex gap-2 mb-3">
			<button class="btn btn-secondary btn-sm" @click="copyText(props.codes.join('\n'), '全部恢复码已复制')">
				复制全部
			</button>
			<button class="btn btn-secondary btn-sm" @click="downloadCodes">下载 .txt</button>
		</div>
		<p class="text-xs leading-relaxed text-amber-700 bg-amber-50 border border-amber-200 rounded-lg px-3 py-2.5">
			恢复码仅显示这一次，关闭后无法再次查看；每个恢复码只能使用一次。全部丢失后，仍可用验证器中的动态码重新生成。
		</p>
	</div>
</template>
