<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { Tag } from '@arco-design/web-vue'
import request from '@/utils/request'

// 审计日志会话视图抽屉：从锚点记录还原完整对话（段/分支/缺失标注齐全）。
// 关联置信度透明展示（精确/内容/推断分层），压缩断点可见摘要，
// 每条消息脚注可跳转对应审计详情（审核追溯闭环）。

const props = withDefaults(
	defineProps<{
		visible: boolean
		recordId: number | null
		// 自定义数据端点：默认走审计模块会话视图；ai-req-analyze 报告等调用方
		// 未必有 audit:view 权限，传自己的端点（参数为 recordId，其语义由调用方定义）
		fetchUrl?: (recordId: number) => string
	}>(),
	{
		fetchUrl: (id: number) => `/admin/audit/request-logs/${id}/conversation`,
	},
)
const emit = defineEmits<{
	(e: 'update:visible', v: boolean): void
	(e: 'open-record', id: number): void
}>()

const loading = ref(false)
const view = ref<any>(null)

watch(
	() => [props.visible, props.recordId] as const,
	([visible, id]) => {
		if (visible && id) fetchView(id)
		if (!visible) view.value = null
	},
)

async function fetchView(id: number) {
	loading.value = true
	view.value = null
	try {
		const res: any = await request.get(props.fetchUrl(id))
		view.value = res.data?.data || null
	} catch {
		view.value = null
	} finally {
		loading.value = false
	}
}

const modeLabel: Record<string, string> = {
	session_key: '精确关联（会话键）',
	content_chain: '内容关联（消息衔接验证）',
	identity_inferred: '身份推断（同用户同Key同时间段）',
}
const modeColor: Record<string, string> = {
	session_key: 'green',
	content_chain: 'arcoblue',
	identity_inferred: 'orange',
}
const resetLabel: Record<string, string> = {
	context_compression: '上下文压缩/续接',
	context_rewrite: '历史改写/话题新建',
	inferred_gap: '推断间隔超窗',
}
const missingLabel: Record<string, string> = {
	response_not_recorded: '该轮回复未按当前审计级别记录',
	body_truncated: '请求体记录被截断',
	unaligned_inferred: '推断模式：与对话流内容无关联（可能为并发其他会话），未并入',
}
const roleLabel: Record<string, string> = {
	system: '系统提示',
	user: '用户',
	assistant: '助手',
	tool: '工具结果',
}

// 段边界标记：渲染序列里 segment_index 变化处插入分隔
const segmentBreaks = computed(() => {
	const map = new Map<number, any>()
	if (!view.value?.segments) return map
	for (const seg of view.value.segments) {
		map.set(seg.segment_index, seg)
	}
	return map
})

function formatMs(ms: number) {
	if (!ms && ms !== 0) return '-'
	if (ms < 1000) return `${ms}ms`
	return `${(ms / 1000).toFixed(2)}s`
}

function openRecord(id: number) {
	emit('open-record', id)
}
</script>

<template>
	<a-drawer
		:visible="visible"
		:width="760"
		title="会话视图"
		:footer="false"
		@cancel="emit('update:visible', false)"
	>
		<a-spin :loading="loading" class="w-full">
			<template v-if="view">
				<!-- 头部：置信度与范围 -->
				<div class="conv-header">
					<Tag :color="modeColor[view.correlation_mode] || 'gray'" size="small">
						{{ modeLabel[view.correlation_mode] || view.correlation_mode }}
					</Tag>
					<span v-if="view.session_key_masked" class="conv-meta">会话键 {{ view.session_key_masked }}</span>
					<span class="conv-meta">共 {{ view.record_count }} 条记录</span>
					<Tag v-if="view.truncated" color="red" size="small">已截断（仅保留最近部分）</Tag>
				</div>

				<!-- 缺失提示 -->
				<a-alert v-if="view.missing?.length" class="mb-3" type="warning">
					{{ view.missing.length }} 处内容缺失：
					{{
						view.missing
							.map((m: any) => `#${m.record_id} ${missingLabel[m.reason] || m.reason}`)
							.join('；')
					}}
				</a-alert>

				<!-- 段时间线 -->
				<div v-if="view.segments?.length > 1" class="mb-4">
					<div class="conv-seg-line">
						<template v-for="(seg, i) in view.segments" :key="seg.segment_index">
							<span v-if="i > 0" class="conv-seg-break">⚡{{ resetLabel[seg.reset_reason] || seg.reset_reason }} →</span>
							<span class="conv-seg">段{{ seg.segment_index }}（{{ seg.records.length }}轮）</span>
						</template>
					</div>
					<!-- 压缩摘要折叠 -->
					<details v-for="seg in view.segments.filter((s: any) => s.summary_text)" :key="'sum' + seg.segment_index" class="conv-summary">
						<summary>段{{ seg.segment_index }} 压缩摘要原文</summary>
						<pre class="conv-pre">{{ seg.summary_text }}</pre>
					</details>
				</div>

				<!-- 消息流 -->
				<div class="conv-flow">
					<template v-for="(msg, i) in view.messages" :key="i">
						<!-- 段边界 -->
						<div
							v-if="i > 0 && msg.segment_index !== view.messages[i - 1].segment_index"
							class="conv-boundary"
						>
							⚡ {{ resetLabel[segmentBreaks.get(msg.segment_index)?.reset_reason] || '上下文重置' }}
						</div>

						<!-- 系统提示：居中说明条 -->
						<div v-if="msg.role === 'system'" class="conv-system">
							<span class="conv-role-tag">{{ roleLabel.system }}</span>
							<span class="conv-system-text">{{ msg.text }}</span>
						</div>

						<!-- 工具结果：独立卡片（内容对审核关键） -->
						<div v-else-if="msg.role === 'tool'" class="conv-row conv-row-tool">
							<div class="conv-tool-card">
								<div class="conv-tool-head">
									<span class="conv-role-tag">⚙ {{ roleLabel.tool }}</span>
									<span v-if="msg.tool_call_id" class="conv-tool-id">{{ msg.tool_call_id }}</span>
								</div>
								<pre class="conv-pre">{{ msg.text || '（空结果）' }}</pre>
							</div>
							<a class="conv-source" @click="openRecord(msg.source_record_id)">#{{ msg.source_record_id }}</a>
						</div>

						<!-- 用户/助手：气泡 -->
						<div v-else :class="['conv-row', msg.role === 'user' ? 'conv-row-user' : 'conv-row-asst']">
							<div :class="['conv-bubble', msg.role === 'user' ? 'conv-bubble-user' : 'conv-bubble-asst']">
								<div class="conv-text">{{ msg.text || '（无文本内容）' }}</div>
								<!-- 工具调用折叠 -->
								<details v-if="msg.tool_calls?.length" class="conv-tools">
									<summary>{{ msg.tool_calls.length }} 次工具调用</summary>
									<div v-for="(tc, j) in msg.tool_calls" :key="j" class="conv-tool-item">
										<span class="conv-tool-name">{{ tc.name || tc.id }}</span>
										<pre class="conv-pre">{{ tc.args }}</pre>
									</div>
								</details>
							</div>
							<a class="conv-source" @click="openRecord(msg.source_record_id)">#{{ msg.source_record_id }}</a>
						</div>
					</template>
				</div>

				<!-- 段记录明细（可追溯） -->
				<div class="mt-4">
					<details class="conv-summary">
						<summary>记录明细（{{ view.record_count }} 条）</summary>
						<div class="conv-records">
							<div v-for="rec in view.segments.flatMap((s: any) => s.records)" :key="rec.id" class="conv-record-row">
								<a class="conv-source" @click="openRecord(rec.id)">#{{ rec.id }}</a>
								<span class="conv-meta">{{ rec.created_at }}</span>
								<span class="conv-meta">{{ rec.model_name || '-' }}</span>
								<span class="conv-meta">{{ formatMs(rec.latency_ms) }}</span>
								<Tag v-if="rec.is_anchor" color="arcoblue" size="small">锚点</Tag>
								<Tag v-if="rec.is_branch" color="orange" size="small">分支</Tag>
							</div>
						</div>
					</details>
				</div>
			</template>
			<a-empty v-else-if="!loading" description="无会话数据" />
		</a-spin>
	</a-drawer>
</template>

<style scoped>
.conv-header {
	display: flex;
	align-items: center;
	flex-wrap: wrap;
	gap: 8px;
	margin-bottom: 12px;
}
.conv-meta {
	font-size: 12px;
	color: var(--color-text-3, #86909c);
}
.conv-seg-line {
	display: flex;
	align-items: center;
	flex-wrap: wrap;
	gap: 6px;
	font-size: 12px;
	color: var(--color-text-2, #4e5969);
}
.conv-seg-break {
	color: var(--color-warning-6, #ff7d00);
}
.conv-summary {
	border: 1px solid var(--color-border-2, #e5e6eb);
	border-radius: 6px;
	padding: 8px 12px;
	margin-top: 8px;
	font-size: 12px;
}
.conv-summary summary {
	cursor: pointer;
	color: var(--color-text-2, #4e5969);
	user-select: none;
}
.conv-flow {
	display: flex;
	flex-direction: column;
	gap: 10px;
}
.conv-boundary {
	text-align: center;
	font-size: 12px;
	color: var(--color-warning-6, #ff7d00);
	background: var(--color-warning-1, #fff7e8);
	border-radius: 6px;
	padding: 4px 0;
}
.conv-system {
	display: flex;
	align-items: baseline;
	gap: 8px;
	background: var(--color-fill-2, #f7f8fa);
	border-radius: 6px;
	padding: 8px 12px;
	font-size: 12px;
}
.conv-system-text {
	color: var(--color-text-2, #4e5969);
	white-space: pre-wrap;
	word-break: break-word;
	max-height: 120px;
	overflow: auto;
}
.conv-role-tag {
	flex-shrink: 0;
	font-weight: 600;
	color: var(--color-text-3, #86909c);
}
/* 工具结果卡片：等宽字体展示返回内容，头部带归属调用 ID */
.conv-row-tool {
	align-items: flex-start;
}
.conv-tool-card {
	max-width: 92%;
	border: 1px solid var(--color-border-2, #e5e6eb);
	border-left: 3px solid var(--color-primary-6, #165dff);
	border-radius: 6px;
	padding: 8px 12px;
	background: var(--color-fill-1, #f2f3f5);
}
.conv-tool-head {
	display: flex;
	align-items: center;
	gap: 8px;
	margin-bottom: 4px;
}
.conv-tool-id {
	font-size: 11px;
	font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
	color: var(--color-text-4, #c9cdd4);
}
.conv-tool-card .conv-pre {
	max-height: 220px;
	background: var(--color-fill-2, #f7f8fa);
}
.conv-row {
	display: flex;
	flex-direction: column;
	align-items: flex-start;
	gap: 2px;
}
.conv-row-user {
	align-items: flex-end;
}
.conv-bubble {
	max-width: 86%;
	border-radius: 10px;
	padding: 8px 12px;
	font-size: 13px;
	line-height: 1.6;
}
.conv-bubble-user {
	background: var(--color-primary-light-1, #e8f3ff);
	color: var(--color-text-1, #1d2129);
}
.conv-bubble-asst {
	background: var(--color-fill-2, #f7f8fa);
	color: var(--color-text-1, #1d2129);
}
.conv-text {
	white-space: pre-wrap;
	word-break: break-word;
}
.conv-tools {
	margin-top: 6px;
	font-size: 12px;
}
.conv-tools summary {
	cursor: pointer;
	color: var(--color-text-3, #86909c);
	user-select: none;
}
.conv-tool-item {
	margin-top: 4px;
}
.conv-tool-name {
	font-weight: 600;
	color: var(--color-text-2, #4e5969);
}
.conv-pre {
	background: var(--color-fill-1, #f2f3f5);
	padding: 6px;
	border-radius: 4px;
	font-size: 12px;
	max-height: 160px;
	overflow: auto;
	white-space: pre-wrap;
	word-break: break-all;
	font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
	margin: 4px 0 0;
}
.conv-source {
	font-size: 11px;
	color: var(--color-text-4, #c9cdd4);
	cursor: pointer;
}
.conv-source:hover {
	color: var(--color-link-6, #165dff);
}
.conv-records {
	max-height: 260px;
	overflow: auto;
	margin-top: 8px;
	display: flex;
	flex-direction: column;
	gap: 4px;
}
.conv-record-row {
	display: flex;
	align-items: center;
	gap: 8px;
	font-size: 12px;
}
/* 移动端：气泡占满宽度 */
@media (max-width: 768px) {
	.conv-bubble {
		max-width: 100%;
	}
}
</style>
