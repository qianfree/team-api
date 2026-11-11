<!-- 渠道模型检测弹窗（渠道列表行「检测」与渠道详情模型能力行「测试」共用）：
     打开即拉取该渠道模型能力列表，逐模型快捷发起连通性测试，行内显示通过/失败，
     右栏展示完整响应信息。测试接口：POST /admin/channels/{id}/test（渠道详情原「测试」tab 已移除，统一走本弹窗）
     布局：左右两栏 + 固定高度。左栏模型列表、右栏结果详情各自滚动，
     检测结果出现时不再挤压模型列表（旧版单列堆叠会把模型顶出可视区）。 -->
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
// 结果面板展示的模型（最后一次发起测试的模型，或手动点选的行内结果）
const activeModel = ref('')
const activeResult = computed(() => (activeModel.value ? results[activeModel.value] || null : null))
const activeTesting = computed(() => !!activeModel.value && !!testing[activeModel.value])

// 已测 / 通过计数，展示在列表标题栏
const testedCount = computed(() => models.value.filter((m) => results[m.model_name]).length)
const passedCount = computed(() => models.value.filter((m) => results[m.model_name]?.success).length)
const testingCount = computed(() => models.value.filter((m) => testing[m.model_name]).length)

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
		// 解析上游响应：chat choices / responses content / output.text
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

// 点击行内已有结果，切回该模型的详情
function focusResult(modelName: string) {
	if (!results[modelName] || testing[modelName]) return
	activeModel.value = modelName
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
		:width="'min(1160px, 96vw)'"
		:footer="false"
		:body-style="{ maxHeight: '74vh', overflowY: 'auto' }"
		@cancel="emit('update:visible', false)"
	>
		<!-- 功能限制提示（原渠道详情「测试」tab 的说明，tab 移除后迁到此处，可关闭） -->
		<AAlert type="info" show-icon closable class="test-hint">
			<template #title>仅支持文本对话模型测试</template>
			<div class="test-hint-body">
				当前检测仅支持<strong>文本对话模型</strong>（如 Chat 系列）的基础连通测试。
				视频生成、图片生成、Embeddings 等其他类型暂不支持此方式，建议使用<strong>租户端的在线体验功能</strong>做端到端测试，或通过 <strong>API 直接调用</strong>验证。
			</div>
		</AAlert>

		<!-- 左右两栏：固定高度，左栏模型列表 / 右栏结果详情各自独立滚动 -->
		<div class="test-body">
			<!-- 左：模型列表 -->
			<section class="test-pane test-pane-list">
				<div class="pane-head">
					<span class="pane-title">模型列表</span>
					<span class="pane-meta">
						<template v-if="testingCount > 0">检测中 {{ testingCount }} · </template>
						<template v-if="testedCount > 0">通过 {{ passedCount }}/{{ testedCount }}</template>
						<template v-else>{{ filteredModels.length }} 个</template>
					</span>
				</div>
				<div class="list-body">
					<div v-if="models.length > 8" class="model-filter">
						<AInput v-model="filterText" size="small" allow-clear placeholder="搜索模型名 / 上游模型名" />
					</div>
					<div v-if="modelsLoading" class="model-list-state"><ASpin dot /></div>
					<div v-else-if="models.length === 0" class="model-list-state">该渠道尚未配置模型能力</div>
					<template v-else>
						<div
							v-for="m in filteredModels"
							:key="m.model_name"
							class="model-row"
							:class="{ 'model-row--active': activeModel === m.model_name }"
						>
							<div class="model-name-wrap">
								<span class="model-name" :title="m.model_name">{{ m.model_name }}</span>
								<span v-if="m.upstream_model && m.upstream_model !== m.model_name" class="model-upstream" :title="`上游模型名：${m.upstream_model}`">
									↳ {{ m.upstream_model }}
								</span>
								<ATag v-if="!m.enabled" size="small">已禁用</ATag>
							</div>
							<div class="model-row-actions">
								<ATag
									v-if="results[m.model_name]"
									:color="results[m.model_name].success ? 'green' : 'red'"
									size="small"
									class="row-result-tag"
									:title="'点击查看该模型检测详情'"
									@click="focusResult(m.model_name)"
								>
									{{ results[m.model_name].success ? `通过 ${results[m.model_name].latency_ms}ms` : '失败' }}
								</ATag>
								<AButton
									size="mini"
									type="outline"
									:loading="testing[m.model_name]"
									@click="runTest(m.model_name)"
								>测试</AButton>
							</div>
						</div>
						<div v-if="filteredModels.length === 0" class="model-list-state">无匹配模型</div>
					</template>
				</div>
			</section>

			<!-- 右：结果详情（最后一次发起测试 / 手动点选的模型） -->
			<section class="test-pane test-pane-result">
				<div class="pane-head">
					<span class="pane-title">检测结果</span>
					<span class="pane-meta">{{ activeModel || '未选择模型' }}</span>
				</div>
				<div class="result-body">
					<template v-if="activeResult">
						<ACard :key="activeModel" :bordered="false" size="small" class="result-card">
							<template #title>
								<span :style="{ color: activeResult.success ? '#00b42a' : '#f53f3f', fontWeight: 600 }">
									{{ activeResult.success ? '测试通过' : '测试失败' }}
								</span>
							</template>
							<template #extra>
								<ASpace>
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
					<div v-else-if="activeTesting" class="result-placeholder">
						<ASpin dot size="small" />
						<span>正在检测 {{ activeModel }} …</span>
					</div>
					<div v-else-if="!modelsLoading && models.length > 0" class="result-placeholder">点击左侧模型行的「测试」按钮发起检测，结果将在此展示</div>
					<div v-else class="result-placeholder">暂无检测结果</div>
				</div>
			</section>
		</div>
	</AModal>
</template>

<style scoped>
.test-hint {
	margin-bottom: 12px;
}

.test-hint-body {
	font-size: 12px;
	line-height: 1.6;
	color: var(--color-text-2);
}

/* === 两栏容器：固定高度，避免结果出现后撑高弹窗、把模型列表顶出视口 === */
.test-body {
	display: flex;
	align-items: stretch;
	gap: 12px;
	height: min(56vh, 520px);
}

.test-pane {
	display: flex;
	flex-direction: column;
	min-height: 0;
	border: 1px solid var(--color-border-2);
	border-radius: 8px;
	overflow: hidden;
	background: var(--color-bg-2);
}

.test-pane-list {
	/* 模型名可能很长：按比例放宽，宽屏下最多 480px，够放常见的长名（带日期后缀）单行显示 */
	flex: 0 0 clamp(340px, 42%, 480px);
}

.test-pane-result {
	flex: 1;
	min-width: 0;
}

.pane-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 8px;
	flex-shrink: 0;
	padding: 7px 12px;
	background: var(--color-fill-1);
	border-bottom: 1px solid var(--color-border-2);
}

.pane-title {
	font-size: 12px;
	font-weight: 600;
	color: var(--color-text-2);
	flex-shrink: 0;
}

.pane-meta {
	font-size: 12px;
	color: var(--color-text-3);
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
}

.list-body {
	flex: 1;
	min-height: 0;
	overflow-y: auto;
}

.model-filter {
	position: sticky;
	top: 0;
	z-index: 1;
	padding: 8px;
	background: var(--color-bg-2);
	border-bottom: 1px solid var(--color-fill-2);
}

.model-list-state {
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 8px;
	min-height: 80px;
	padding: 12px;
	font-size: 13px;
	color: var(--color-text-3);
	text-align: center;
}

.model-row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 10px;
	padding: 6px 12px;
	border-left: 2px solid transparent;
	transition: background-color 0.15s;
}

.model-row + .model-row {
	border-top: 1px solid var(--color-fill-2);
}

.model-row:hover {
	background: var(--color-fill-1);
}

.model-row--active {
	background: var(--color-primary-light-1);
	border-left-color: var(--color-primary-6);
}

.model-name-wrap {
	display: flex;
	align-items: center;
	gap: 6px;
	min-width: 0;
	flex-wrap: wrap;
}

.model-name {
	font-size: 13px;
	font-weight: 500;
	color: var(--color-text-1);
	/* 模型名多为 kebab-case：按连字符断行，别在词中间硬切 */
	word-break: normal;
	overflow-wrap: anywhere;
	/* 超长模型名最多两行，超出省略；完整名称走 title 悬停 */
	display: -webkit-box;
	-webkit-box-orient: vertical;
	-webkit-line-clamp: 2;
	overflow: hidden;
}

.model-upstream {
	font-size: 12px;
	color: var(--color-text-3);
	word-break: break-all;
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	max-width: 100%;
}

.model-row-actions {
	display: flex;
	align-items: center;
	gap: 6px;
	flex-shrink: 0;
}

.row-result-tag {
	flex-shrink: 0;
	cursor: pointer;
}

/* === 右栏结果区 === */
.result-body {
	flex: 1;
	min-height: 0;
	overflow-y: auto;
	padding: 4px 12px 12px;
}

.result-card {
	background: transparent;
}

.result-placeholder {
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 8px;
	height: 100%;
	min-height: 120px;
	padding: 14px;
	text-align: center;
	font-size: 12px;
	color: var(--color-text-3);
	background: var(--color-fill-1);
	border-radius: 6px;
}

/* 结果面板 */
.test-reply {
	white-space: pre-wrap;
	word-break: break-word;
	font-size: 14px;
	line-height: 1.6;
	color: var(--color-text-1);
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

/* 窄屏：退回上下堆叠，各自限高滚动，仍不挤压 */
@media (max-width: 760px) {
	.test-body {
		flex-direction: column;
		height: auto;
	}

	.test-pane-list {
		flex: none;
		max-height: 40vh;
	}

	.test-pane-result {
		max-height: 46vh;
	}
}
</style>
