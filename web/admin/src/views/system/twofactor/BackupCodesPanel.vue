<script setup lang="ts">
import { Message } from '@arco-design/web-vue'

const props = defineProps<{ codes: string[] }>()

// 项目惯例：各页面局部 copyText（navigator.clipboard + Message 反馈）
async function copyText(text: string, label = '已复制') {
	try {
		await navigator.clipboard.writeText(text)
		Message.success(label)
	} catch {
		Message.error('复制失败，请手动选择复制')
	}
}

function downloadCodes() {
	const content = [
		'Team-API 管理后台 · 两步验证恢复码',
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
	<div class="backup-codes">
		<div class="codes-grid">
			<div
				v-for="codeItem in props.codes"
				:key="codeItem"
				class="code-item"
				title="点击复制"
				@click="copyText(codeItem, '恢复码已复制')"
			>
				{{ codeItem }}
			</div>
		</div>
		<div class="panel-actions">
			<a-button size="small" @click="copyText(props.codes.join('\n'), '全部恢复码已复制')">复制全部</a-button>
			<a-button size="small" @click="downloadCodes">下载 .txt</a-button>
		</div>
		<div class="warn-tip">
			恢复码仅显示这一次，关闭后无法再次查看；每个恢复码只能使用一次。全部丢失后，仍可用验证器中的动态码重新生成。
		</div>
	</div>
</template>

<style scoped>
.codes-grid {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 8px;
	margin-bottom: 12px;
}
.code-item {
	padding: 8px 10px;
	font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
	font-size: 14px;
	letter-spacing: 0.5px;
	text-align: center;
	background: var(--ta-bg-card);
	border: 1px solid var(--ta-border-light);
	border-radius: 6px;
	cursor: pointer;
	transition: border-color 0.2s;
	user-select: all;
}
.code-item:hover {
	border-color: #14b8a6;
}
.panel-actions {
	display: flex;
	gap: 8px;
	margin-bottom: 12px;
}
.warn-tip {
	padding: 8px 12px;
	font-size: 12px;
	line-height: 1.6;
	color: var(--ta-text-tertiary);
	background: var(--color-fill-1);
	border-radius: 6px;
}
</style>
