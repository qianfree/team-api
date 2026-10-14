<!-- 渠道模型检测弹窗（渠道列表行「检测」与渠道详情模型能力行「测试」共用）：
     打开即拉取该渠道模型能力列表，逐模型快捷发起连通性测试，行内显示通过/失败，
     下方面板展示完整响应信息。测试接口与详情页测试 tab 相同（POST /admin/channels/{id}/test） -->
<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import request from '@/utils/request'

const props = defineProps<{
	visible: boolean
	channelId: number | string | null
	channelName?: string
	// 打开后自动发起测试的模型（渠道详情模型能力行入口传入）；每次打开仅生效一次
	autoTestModel?: string | null
}>()

const emit = defineEmits<{ 'update:visible': [value: boolean] }>()

// === 模型能力列表 ===
const modelsLoading = ref(false)
const models = ref<any[]>([])
const filterText = ref('')

const filteredModels = computed(() => {
	const kw = filterText.value.trim().toLowerCase()
	if (!kw) return models.value
	return models.value.filter(
		(m) => (m.model_name || '').toLowerCase().includes(kw) || (m.upstream_model || '').toLowerCase().includes(kw),
	)
})

// === 逐模型测试状态：testing[model] 进行中；results[model] 最新结果 ===
const testing = reactive<Record<string, boolean>>({})
const results = reactive<Record<string, any>>({})
// 结果面板展示的模型（最后一次发起测试的模型）
const activeModel = ref('')
const activeResult = computed(() => (activeModel.value ? results[activeModel.value] || null : null))

watch(
	() => props.visible,
	(v) => {
		if (!v) return
		models.value = []
		filterText.value = ''
		activeModel.value = ''
		for (const k of Object.keys(testing)) delete testing[k]
		for (const k of Object.keys(results)) delete results[k]
		fetchModels().then(() => {
			if (props.autoTestModel) runTest(props.autoTestModel)
		})
	},
)

async function fetchModels() {
	if (!props.channelId) return
	modelsLoading.value = true
	try {
		const res: any = await request.get(`/admin/channels/${props.channelId}/abilities`)
		models.value = res.data?.data?.list || res.data?.list || []
	} catch {
		models.value = []
	} finally {
		modelsLoading.value = false
	}
}

async function runTest(modelName: string) {
	if (!modelName || testing[modelName]) return
	testing[modelName] = true
	activeModel.value = modelName
	delete results[modelName]
	try {
		const res: any = await request.post(`/admin/channels/${props.channelId}/test`, { model_name: modelName })
		const raw = res.data?.data ?? res.data
		// 解析上游响应（与渠道详情测试 tab 同口径：chat choices / responses content / output.text）
		let parsedResponse: any = null
		if (raw?.response) {
			try { parsedResponse = JSON.parse(raw.response) } catch { parsedResponse = raw.response }
		}
		let content: string | null = null
		if (parsedResponse && typeof parsedResponse === 'object') {
			content = parsedResponse?.choices?.[0]?.message?.content
				|| parsedResponse?.content?.[0]?.text
				|| parsedResponse?.output?.text
				|| null
		}
		results[modelName] = {
			success: raw?.success ?? true,
			latency_ms: raw?.latency_ms ?? 0,
			model_name: raw?.model_name || modelName,
			content,
			usage: parsedResponse && typeof parsedResponse === 'object' ? parsedResponse?.usage : null,
			error: raw?.error || null,
			request: raw?.request || null,
			response: raw?.response || null,
		}
	} catch (err: any) {
		results[modelName] = { success: false, model_name: modelName, error: err?.response?.data?.message || err?.message || '测试失败' }
	} finally {
		testing[modelName] = false
	}
}

function formatJson(str: string): string {
	if (!str) return ''
	try { return JSON.stringify(JSON.parse(str), null, 2) } catch { return str }
}

function formatHeaders(headers: Record<string, string>): string {
	if (!headers) return ''
	return Object.entries(headers).map(([k, v]) => `${k}: ${v}`).join('\n')
}
</script>

<template>
	<AModal
		:visible="visible"
		:title="`模型检测${channelName ? ' - ' + channelName : ''}`"
		:width="'min(760px, 94vw)'"
		:footer="false"
		:body-style="{ maxHeight: '68vh', overflowY: 'auto' }"
		@cancel="emit('update:visible', false)"
	>
		<!-- 功能限制提示（与详情页测试 tab 同文案口径，精简为一行） -->
		<div class="test-hint">仅支持文本对话模型的基础连通测试；视频 / 图片生成、Embeddings 等请用租户端在线体验或 API 调用验证。</div>

		<!-- 模型列表 -->
		<AInput
			v-if="models.length > 8"
			v-model="filterText"
			size="small"
			allow-clear
			placeholder="搜索模型名 / 上游模型名"
			class="model-filter"
		/>
		<div v-if="modelsLoading" class="model-list-state"><ASpin dot /></div>
		<div v-else-if="models.length === 0" class="model-list-state" style="color: var(--color-text-3)">该渠道尚未配置模型能力</div>
		<div v-else class="model-list">
			<div v-for="m in filteredModels" :key="m.model_name" class="model-row">
				<div class="model-name-wrap">
					<span class="model-name">{{ m.model_name }}</span>
					<span v-if="m.upstream_model && m.upstream_model !== m.model_name" class="model-upstream" :title="`上游模型名：${m.upstream_model}`">
						↳ {{ m.upstream_model }}
					</span>
					<ATag v-if="!m.enabled" size="small">已禁用</ATag>
				</div>
				<div class="model-row-actions">
					<template v-if="results[m.model_name]">
						<ATag :color="results[m.model_name].success ? 'green' : 'red'" size="small" class="row-result-tag">
							{{ results[m.model_name].success ? `通过 ${results[m.model_name].latency_ms}ms` : '失败' }}
						</ATag>
					</template>
					<AButton
						size="mini"
						type="outline"
						:loading="testing[m.model_name]"
						@click="runTest(m.model_name)"
					>测试</AButton>
				</div>
			</div>
			<div v-if="filteredModels.length === 0" class="model-list-state" style="color: var(--color-text-3)">无匹配模型</div>
		</div>

		<!-- 结果面板（最后一次发起测试的模型） -->
		<template v-if="activeResult">
			<ADivider margin="8px">
				<span class="result-divider-text">{{ activeResult.model_name }} 的测试结果</span>
			</ADivider>
			<ACard :bordered="true" size="small" class="w-full">
				<template #title>
					<span :style="{ color: activeResult.success ? '#00b42a' : '#f53f3f', fontWeight: 600 }">
						{{ activeResult.success ? '测试通过' : '测试失败' }}
					</span>
				</template>
				<template #extra>
					<ASpace>
						<ATag size="small" color="arcoblue">{{ activeResult.model_name }}</ATag>
						<ATag v-if="activeResult.latency_ms" size="small">{{ activeResult.latency_ms }}ms</ATag>
					</ASpace>
				</template>

				<!-- 成功时显示 AI 回复 -->
				<div v-if="activeResult.success && activeResult.content" class="test-reply">{{ activeResult.content }}</div>
				<div v-else-if="activeResult.success && !activeResult.content" class="test-reply" style="color: var(--color-text-3)">（AI 无文本回复内容）</div>

				<!-- 失败时显示错误 -->
				<div v-if="!activeResult.success && (activeResult.error || activeResult.message)" class="test-error">{{ activeResult.error || activeResult.message }}</div>

				<!-- Token 用量 -->
				<div v-if="activeResult.usage" class="test-usage">
					<ATag size="small">Prompt: {{ activeResult.usage.prompt_tokens }}</ATag>
					<ATag size="small">Completion: {{ activeResult.usage.completion_tokens }}</ATag>
					<ATag size="small">Total: {{ activeResult.usage.total_tokens }}</ATag>
				</div>

				<!-- 请求详情 / 响应原文 -->
				<ACollapse :bordered="false" :default-active-key="activeResult.success ? [] : ['request']">
					<ACollapseItem header="请求详情" key="request" :header-style="{ fontSize: '12px', color: 'var(--color-text-3)' }">
						<div v-if="activeResult.request" class="test-debug-info">
							<div class="test-debug-row">
								<span class="test-debug-label">Method</span>
								<ATag size="small" color="gray">{{ activeResult.request.method }}</ATag>
							</div>
							<div class="test-debug-row">
								<span class="test-debug-label">URL</span>
								<code class="test-debug-value">{{ activeResult.request.url }}</code>
							</div>
							<div class="test-debug-row">
								<span class="test-debug-label">Headers</span>
								<pre class="test-debug-code">{{ formatHeaders(activeResult.request.headers) }}</pre>
							</div>
							<div class="test-debug-row">
								<span class="test-debug-label">Body</span>
								<pre class="test-debug-code">{{ formatJson(activeResult.request.body) }}</pre>
							</div>
						</div>
						<div v-else style="color: var(--color-text-3); font-size: 12px;">无请求信息</div>
					</ACollapseItem>
					<ACollapseItem v-if="activeResult.response" header="响应内容" key="response" :header-style="{ fontSize: '12px', color: 'var(--color-text-3)' }">
						<pre class="test-raw">{{ formatJson(activeResult.response) }}</pre>
					</ACollapseItem>
				</ACollapse>
			</ACard>
		</template>
		<div v-else-if="!modelsLoading && models.length > 0" class="result-placeholder">点击模型行的「测试」按钮发起检测，结果将在此展示</div>
	</AModal>
</template>

<style scoped>
.test-hint {
	font-size: 12px;
	color: var(--color-text-3);
	margin-bottom: 12px;
	line-height: 1.6;
}

.model-filter {
	margin-bottom: 8px;
}

.model-list {
	border: 1px solid var(--color-border-2);
	border-radius: 6px;
	max-height: 300px;
	overflow-y: auto;
}

.model-list-state {
	display: flex;
	align-items: center;
	justify-content: center;
	min-height: 80px;
	font-size: 13px;
}

.model-row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 12px;
	padding: 7px 12px;
}

.model-row + .model-row {
	border-top: 1px solid var(--color-fill-2);
}

.model-name-wrap {
	display: flex;
	align-items: center;
	gap: 8px;
	min-width: 0;
	flex-wrap: wrap;
}

.model-name {
	font-size: 13px;
	font-weight: 500;
	color: var(--color-text-1);
	word-break: break-all;
}

.model-upstream {
	font-size: 12px;
	color: var(--color-text-3);
	word-break: break-all;
}

.model-row-actions {
	display: flex;
	align-items: center;
	gap: 8px;
	flex-shrink: 0;
}

.row-result-tag {
	flex-shrink: 0;
}

.result-divider-text {
	font-size: 12px;
	color: var(--color-text-3);
}

.result-placeholder {
	margin-top: 12px;
	padding: 14px;
	text-align: center;
	font-size: 12px;
	color: var(--color-text-3);
	background: var(--color-fill-1);
	border-radius: 6px;
}

/* 结果面板（与渠道详情测试 tab 同样式） */
.test-reply {
	white-space: pre-wrap;
	word-break: break-word;
	font-size: 14px;
	line-height: 1.6;
	color: var(--color-text-1);
	max-height: 300px;
	overflow-y: auto;
}

.test-usage {
	margin-top: 12px;
	display: flex;
	gap: 6px;
	flex-wrap: wrap;
}

.test-raw {
	font-size: 12px;
	background: var(--color-fill-1);
	padding: 8px;
	border-radius: 4px;
	max-height: 200px;
	overflow: auto;
	color: var(--color-text-3);
	white-space: pre-wrap;
	word-break: break-all;
	margin: 0;
}

.test-error {
	color: #f53f3f;
	font-size: 14px;
	line-height: 1.6;
	word-break: break-word;
}

.test-debug-info {
	display: flex;
	flex-direction: column;
	gap: 8px;
}

.test-debug-row {
	display: flex;
	gap: 12px;
	align-items: flex-start;
}

.test-debug-label {
	font-size: 12px;
	color: var(--color-text-3);
	min-width: 60px;
	flex-shrink: 0;
	padding-top: 2px;
}

.test-debug-value {
	font-size: 12px;
	color: var(--color-text-2);
	word-break: break-all;
	background: var(--color-fill-1);
	padding: 2px 6px;
	border-radius: 3px;
}

.test-debug-code {
	font-size: 12px;
	background: var(--color-fill-1);
	padding: 6px 8px;
	border-radius: 4px;
	max-height: 150px;
	overflow: auto;
	color: var(--color-text-3);
	white-space: pre-wrap;
	word-break: break-all;
	margin: 0;
	flex: 1;
}
</style>
