<script setup lang="ts">
import { ref, reactive, computed, nextTick, watch } from 'vue'
import { NInput, NInputNumber, NSwitch } from 'naive-ui'
import { calculateCost } from './calculateCost'
import Icon from '@/components/common/Icon.vue'
import MarkdownRenderer from '@/components/common/MarkdownRenderer.vue'
import BaseSelect from '../../../components/common/BaseSelect.vue'
import AttachmentBar from './AttachmentBar.vue'
import PromptInput from './PromptInput.vue'
import { useAttachments, formatBytes, MAX_TOTAL_BYTES, type AttachmentKind } from './useAttachments'
import { compileChatParts, type ContentPart, type AttachmentPreview } from './attachmentParts'
import { toast } from '@/utils/toast'

interface ModelItem {
	model_id: string
	model_name: string
	category: string
	billing_mode?: string | null
	per_request_price?: number | null
	input_price?: number | null
	output_price?: number | null
	// 能力特性 JSON（如 {"vision":true}），决定附件上传是否开放
	capabilities?: string
}
const props = defineProps<{
	models: ModelItem[]
	apiKey: string
}>()

const sending = ref(false)
const selectedModel = ref('')
const selectedModelItem = computed(() =>
	props.models.find(m => m.model_id === selectedModel.value),
)
const isImageModel = computed(() =>
	selectedModelItem.value?.category === 'image',
)

// models 由父组件异步加载，setup 阶段可能为空；用 watch（immediate）保证
// 首次加载完成或列表变化时，若当前未选择或选中模型不在列表内
//（如切换 API Key 后模型范围变化），自动选中第一个模型
watch(
	() => props.models,
	models => {
		if (!selectedModel.value || !models.some(m => m.model_id === selectedModel.value)) {
			selectedModel.value = models[0]?.model_id || ''
		}
	},
	{ immediate: true },
)

const params = reactive({
	temperature: 0.7,
	maxTokens: 2048,
	systemPrompt: '',
	topP: 1.0,
	stream: true,
	thinking: false,
})

const imageConfig = reactive({
	aspectRatio: '1:1',
	imageSize: '1K',
})
const aspectRatioOptions = ['1:1', '3:4', '4:3', '16:9', '9:16', '2:3', '3:2', '4:5', '5:4', '21:9']
const imageSizeOptions = ['512', '1K', '2K', '4K']
const modelOptions = computed(() => props.models.map(m => ({ value: m.model_id, label: m.model_name || m.model_id })))
const aspectRatioSelectOptions = computed(() => aspectRatioOptions.map(r => ({ value: r, label: r })))
const imageSizeSelectOptions = computed(() => imageSizeOptions.map(s => ({ value: s, label: s })))

interface ChatMessage {
	role: string
	/** 展示文本：[@图片1] 已换成纯文字标签 */
	content: string
	reasoningContent?: string
	/** 多模态消息发给 API 的完整形态；纯文本消息为空，回传时退化为 content 字符串 */
	parts?: ContentPart[]
	/** 气泡里的附件缩略图 */
	previews?: AttachmentPreview[]
}
const messages = ref<ChatMessage[]>([])
const inputMessage = ref('')
const tokenUsage = reactive({ prompt: 0, completion: 0, reasoning: 0, total: 0, cost: '' })

// ── 附件 ──

// capabilities 是 admin 侧存的 JSON 对象字符串，解析失败按「无能力」处理
const modelCapabilities = computed<Record<string, boolean>>(() => {
	const raw = selectedModelItem.value?.capabilities
	if (!raw) return {}
	try {
		const parsed = JSON.parse(raw)
		return parsed && typeof parsed === 'object' ? parsed : {}
	} catch {
		return {}
	}
})

// 对话只收图片与音频：ContentPart.File 虽在 DTO 里，但各协议转换器普遍不处理，
// 传文档会被静默丢弃还照样计费。类型随模型能力动态收放。
const attachmentKinds = computed<AttachmentKind[]>(() => {
	const kinds: AttachmentKind[] = []
	if (modelCapabilities.value.vision) kinds.push('image')
	if (modelCapabilities.value.audio_input) kinds.push('audio')
	return kinds
})

const {
	list: attachmentList,
	totalBytes: attachmentBytes,
	accept: acceptTypes,
	max: attachmentMax,
	addFiles: addAttachments,
	remove: removeAttachment,
	clear: clearAttachments,
} = useAttachments({ kinds: () => attachmentKinds.value, max: 9 })

const promptInput = ref<InstanceType<typeof PromptInput> | null>(null)
const attachmentBar = ref<InstanceType<typeof AttachmentBar> | null>(null)
// 输入容器的拖拽高亮状态（附件栏在 bare 变体下不再自带拖放区，统一由输入容器承接）
const dragging = ref(false)

// 空态快捷提示词：点一下直接填进输入框，减少首次使用的空白焦虑
const quickPrompts = [
	'用三句话介绍你能做什么',
	'写一个 Python 快速排序函数',
	'把下面这段话翻译成英文：',
]

// 非 vision / 非音频输入的模型禁用上传：传了也是白花 token 再拿上游报错
const uploadDisabled = computed(() => attachmentKinds.value.length === 0)
const uploadDisabledReason = computed(() =>
	uploadDisabled.value ? '该模型不支持图片/音频输入' : '',
)

// 历史消息里的 base64 每轮都会重新发送，多轮下很容易撞后端 8MB 的 body 上限，
// 这里把「历史 + 待发」的合计体积摆到台面上，并在超限时拦住发送。
const historyAttachmentBytes = computed(() =>
	messages.value.reduce((sum, m) => {
		if (!m.parts) return sum
		return sum + m.parts.reduce((s, p) => {
			if (p.type === 'image_url') return s + (p.image_url?.url.length || 0)
			if (p.type === 'input_audio') return s + (p.input_audio?.data.length || 0)
			return s
		}, 0)
	}, 0),
)
const pendingBytes = computed(() => historyAttachmentBytes.value + attachmentBytes.value)
const overBudget = computed(() => pendingBytes.value > MAX_TOTAL_BYTES)

function insertMention(name: string) {
	promptInput.value?.insertMention(name)
}

// 粘贴图片走同一条上传链路；模型不支持时给出明确原因，而非落到「不支持的文件类型」
function onPasteFiles(files: File[]) {
	if (uploadDisabled.value) {
		toast.warning(uploadDisabledReason.value)
		return
	}
	addAttachments(files)
}

// 输入容器承接拖放：与粘贴共用同一校验与上传链路
function onDropFiles(e: DragEvent) {
	dragging.value = false
	if (uploadDisabled.value) {
		toast.warning(uploadDisabledReason.value)
		return
	}
	if (e.dataTransfer?.files?.length) addAttachments(Array.from(e.dataTransfer.files))
}

function openFilePicker() {
	if (uploadDisabled.value) {
		toast.warning(uploadDisabledReason.value)
		return
	}
	attachmentBar.value?.pick()
}

// 一键清空附件（模型切换导致能力不匹配时尤其有用）
function clearAllAttachments() {
	clearAttachments()
}

// Temperature 滑块的已填充比例，用于把轨道左段染成主色
const rangePercent = computed(() => `${(params.temperature / 2) * 100}%`)

// 恢复参数默认值：只重置生成参数，不动已选模型与对话内容
function resetParams() {
	params.temperature = 0.7
	params.maxTokens = 2048
	params.systemPrompt = ''
	params.thinking = false
	params.stream = true
	imageConfig.aspectRatio = '1:1'
	imageConfig.imageSize = '1K'
	toast.success('参数已恢复默认值')
}

// 记录每条 assistant 消息的思考内容折叠状态（用 message index 作为 key）
const expandedThinking = ref<Record<number, boolean>>({})

function toggleThinking(idx: number) {
	expandedThinking.value[idx] = !expandedThinking.value[idx]
}

const messagesRef = ref<HTMLElement | null>(null)

// 判断用户是否在消息区域底部（允许 40px 容差）
function isNearBottom(): boolean {
	const el = messagesRef.value
	if (!el) return true
	return el.scrollHeight - el.scrollTop - el.clientHeight < 40
}

function scrollToBottom(force = false) {
	nextTick(() => {
		if (messagesRef.value && (force || isNearBottom())) {
			messagesRef.value.scrollTop = messagesRef.value.scrollHeight
		}
	})
}

function buildRequestBody(apiMessages: { role: string; content: string | ContentPart[] }[]) {
	const body: Record<string, any> = {
		model: selectedModel.value,
		messages: apiMessages,
		temperature: params.temperature,
		max_tokens: params.maxTokens,
		stream: params.stream,
	}
	// 思考控制：通过 thinking 参数开关（{"type": "enabled"} / {"type": "disabled"}），图片模型不参与
	if (!isImageModel.value) {
		body.thinking = { type: params.thinking ? 'enabled' : 'disabled' }
	}
	if (params.stream) {
		body.stream_options = { include_usage: true }
	}
	if (isImageModel.value) {
		body.image_config = {
			aspect_ratio: imageConfig.aspectRatio,
			image_size: imageConfig.imageSize,
		}
	}
	return body
}

async function sendNonStream(requestBody: Record<string, any>, assistantIdx: number) {
	const response = await fetch('/v1/chat/completions', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
			'Authorization': `Bearer ${props.apiKey}`,
		},
		body: JSON.stringify(requestBody),
	})

	if (!response.ok) {
		let errMsg = `请求失败 (${response.status})`
		try {
			const errData = await response.json()
			if (errData?.error?.message) errMsg = errData.error.message
		} catch {}
		messages.value[assistantIdx] = { role: 'assistant', content: errMsg }
		return
	}

	const data = await response.json()
	const choice = data.choices?.[0]?.message
	messages.value[assistantIdx] = {
		role: 'assistant',
		content: choice?.content || '(无响应内容)',
		reasoningContent: choice?.reasoning_content || undefined,
	}

	const usage = data.usage || {}
	tokenUsage.prompt = usage.prompt_tokens || 0
	tokenUsage.completion = usage.completion_tokens || 0
	tokenUsage.reasoning = usage.completion_tokens_details?.reasoning_tokens || 0
	tokenUsage.total = usage.total_tokens || 0

	const model = selectedModelItem.value
	if (model) {
		tokenUsage.cost = calculateCost(model, usage) || ''
	}
}

async function sendStream(requestBody: Record<string, any>, assistantIdx: number) {
	const response = await fetch('/v1/chat/completions', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
			'Authorization': `Bearer ${props.apiKey}`,
		},
		body: JSON.stringify(requestBody),
	})

	if (!response.ok) {
		let errMsg = `请求失败 (${response.status})`
		try {
			const errData = await response.json()
			if (errData?.error?.message) errMsg = errData.error.message
		} catch {}
		messages.value[assistantIdx] = { role: 'assistant', content: errMsg }
		return
	}

	const reader = response.body?.getReader()
	if (!reader) {
		messages.value[assistantIdx] = { role: 'assistant', content: '浏览器不支持流式响应' }
		return
	}

	const decoder = new TextDecoder()
	let sseBuf = ''
	let contentBuf = ''
	let reasoningBuf = ''

	while (true) {
		const { done, value } = await reader.read()
		if (done) break

		sseBuf += decoder.decode(value, { stream: true })
		const lines = sseBuf.split('\n')
		sseBuf = lines.pop() || ''

		for (const line of lines) {
			const trimmed = line.trim()
			if (!trimmed || trimmed.startsWith(':')) continue
			if (trimmed === 'data: [DONE]') continue

			if (trimmed.startsWith('data: ')) {
				const jsonStr = trimmed.slice(6)
				try {
					const chunk = JSON.parse(jsonStr)
					const delta = chunk.choices?.[0]?.delta
					if (delta) {
						if (delta.content) {
							contentBuf += delta.content
							messages.value[assistantIdx].content = contentBuf
							scrollToBottom()
						}
						if (delta.reasoning_content) {
							reasoningBuf += delta.reasoning_content
							messages.value[assistantIdx].reasoningContent = reasoningBuf
						}
					}
					if (chunk.usage) {
						const usage = chunk.usage
						tokenUsage.prompt = usage.prompt_tokens || 0
						tokenUsage.completion = usage.completion_tokens || 0
						tokenUsage.reasoning = usage.completion_tokens_details?.reasoning_tokens || 0
						tokenUsage.total = usage.total_tokens || 0

						const model = selectedModelItem.value
						if (model) {
							tokenUsage.cost = calculateCost(model, usage) || ''
						}
					}
				} catch {}
			}
		}
	}

	if (!contentBuf) {
		messages.value[assistantIdx].content = '(无响应内容)'
	}
	if (!reasoningBuf) {
		messages.value[assistantIdx].reasoningContent = undefined
	}
}

async function sendMessage() {
	const raw = inputMessage.value.trim()
	if ((!raw && !attachmentList.value.length) || !selectedModel.value || sending.value) return

	if (overBudget.value) {
		toast.error(`附件合计 ${formatBytes(pendingBytes.value)} 已超过 ${formatBytes(MAX_TOTAL_BYTES)}，请清空对话或删除部分附件`)
		return
	}

	if (params.systemPrompt && messages.value.length === 0) {
		messages.value.push({ role: 'system', content: params.systemPrompt })
	}

	// 正文里的 [@图片1] 换成文字标签，并在同一位置插入对应的 content part；
	// 未被引用的附件追加到末尾。纯文本消息不带 parts，回传时退化为字符串。
	const compiled = compileChatParts(raw, attachmentList.value)
	messages.value.push({
		role: 'user',
		content: compiled.displayText,
		parts: attachmentList.value.length ? compiled.parts : undefined,
		previews: attachmentList.value.length ? compiled.previews : undefined,
	})
	inputMessage.value = ''
	clearAttachments()
	sending.value = true

	const assistantIdx = messages.value.length
	messages.value.push({ role: 'assistant', content: '', reasoningContent: '' })
	scrollToBottom()

	const apiMessages = messages.value
		.slice(0, assistantIdx)
		.map(m => ({ role: m.role, content: (m.parts ?? m.content) as string | ContentPart[] }))
	const requestBody = buildRequestBody(apiMessages)

	try {
		if (params.stream) {
			await sendStream(requestBody, assistantIdx)
		} else {
			await sendNonStream(requestBody, assistantIdx)
		}
	} catch (e: any) {
		const errMsg = e?.message || '请求失败，请重试'
		messages.value[assistantIdx] = { role: 'assistant', content: errMsg }
	} finally {
		sending.value = false
		scrollToBottom()
	}
}

function clearChat() {
	messages.value = []
	clearAttachments()
	tokenUsage.prompt = 0
	tokenUsage.completion = 0
	tokenUsage.reasoning = 0
	tokenUsage.total = 0
	tokenUsage.cost = ''
	expandedThinking.value = {}
}
</script>

<template>
	<!-- 高度偏移 ≈ 顶栏与页边距 + 上方工具栏卡片，保证对话卡片完整落在视口内 -->
	<div class="flex flex-col lg:h-[calc(100vh-15rem)] lg:overflow-hidden">
		<!-- grid-rows minmax(0,1fr) 强制行高等于容器高，左列卡片内部滚动，
		     使参数面板与对话区等高对齐，不再出现左栏半截卡片下的大片空白。
		     左列 22rem：参数区控件较多（模型下拉、Max Tokens、System Prompt、两个开关），
		     19rem 下 System Prompt 与下拉框偏窄，加宽后可读性更好 -->
		<div class="grid grid-cols-1 gap-4 lg:grid-cols-[22rem_minmax(0,1fr)] lg:grid-rows-[minmax(0,1fr)] lg:min-h-0 lg:flex-1">
			<!-- Left: Parameters -->
			<div class="flex lg:col-span-1 lg:min-h-0">
				<div class="card flex w-full flex-col overflow-hidden lg:h-full">
					<div class="flex shrink-0 items-center justify-between border-b border-gray-100 px-4 py-3">
						<div class="flex items-center gap-2">
							<Icon name="cog" size="sm" class="text-primary-500" />
							<h3 class="text-sm font-semibold text-gray-900">参数配置</h3>
						</div>
						<button
							class="flex items-center gap-1 rounded-lg px-2 py-1 text-xs text-gray-400 transition-colors duration-150 hover:bg-gray-50 hover:text-primary-600"
							title="恢复默认参数"
							@click="resetParams"
						>
							<Icon name="refresh" size="xs" />
							重置
						</button>
					</div>

					<div class="flex-1 space-y-4 overflow-y-auto px-4 py-4">
						<!-- 模型 -->
						<div>
							<div class="mb-2 flex items-center justify-between">
								<label class="text-xs font-semibold text-gray-500">模型</label>
								<span class="text-[11px] text-gray-400">共 {{ models.length }} 个可用</span>
							</div>
							<BaseSelect v-model="selectedModel" :options="modelOptions" />
							<div v-if="isImageModel" class="mt-2 flex items-center gap-1.5">
								<span class="badge badge-primary">图片模型</span>
								<span class="text-xs text-gray-500">支持生成图片</span>
							</div>
						</div>

						<!-- 生成参数 -->
						<div class="space-y-4 border-t border-gray-100 pt-4">
							<div>
								<div class="mb-2 flex items-center justify-between">
									<label class="text-xs font-semibold text-gray-500">Temperature</label>
									<span class="rounded-md bg-primary-50 px-1.5 py-0.5 font-mono text-xs font-semibold text-primary-600">
										{{ params.temperature.toFixed(1) }}
									</span>
								</div>
								<input
									v-model.number="params.temperature"
									type="range"
									min="0"
									max="2"
									step="0.1"
									class="range-slider"
									:style="{ '--pct': rangePercent }"
								/>
								<div class="mt-1.5 flex justify-between text-[11px] text-gray-400">
									<span>严谨</span><span>均衡</span><span>发散</span>
								</div>
							</div>

							<div>
								<label class="mb-2 block text-xs font-semibold text-gray-500">Max Tokens</label>
								<n-input-number v-model:value="params.maxTokens" :min="1" :max="128000" class="w-full" />
							</div>
						</div>

						<!-- 系统提示词 -->
						<div class="border-t border-gray-100 pt-4">
							<label class="mb-2 block text-xs font-semibold text-gray-500">System Prompt</label>
							<n-input v-model:value="params.systemPrompt" type="textarea" :rows="3" placeholder="设置系统提示词..." />
						</div>

						<!-- 开关 -->
						<div class="space-y-3 border-t border-gray-100 pt-4">
							<div v-if="!isImageModel" class="flex items-start justify-between gap-3">
								<div class="min-w-0">
									<p class="text-sm font-medium text-gray-700">深度思考</p>
									<p class="mt-0.5 text-[11px] leading-relaxed text-gray-400">thinking 参数控制，仅部分模型支持</p>
								</div>
								<n-switch v-model:value="params.thinking" size="small" class="shrink-0" />
							</div>
							<div class="flex items-start justify-between gap-3">
								<div class="min-w-0">
									<p class="text-sm font-medium text-gray-700">流式响应</p>
									<p class="mt-0.5 text-[11px] leading-relaxed text-gray-400">逐字返回，实时看到输出过程</p>
								</div>
								<n-switch v-model:value="params.stream" size="small" class="shrink-0" />
							</div>
						</div>

						<!-- 图片模型专属参数 -->
						<div v-if="isImageModel" class="space-y-4 border-t border-gray-100 pt-4">
							<p class="text-xs font-semibold text-gray-500">图片参数</p>
							<div>
								<label class="mb-2 block text-xs font-medium text-gray-500">宽高比</label>
								<BaseSelect v-model="imageConfig.aspectRatio" :options="aspectRatioSelectOptions" />
							</div>
							<div>
								<label class="mb-2 block text-xs font-medium text-gray-500">分辨率</label>
								<BaseSelect v-model="imageConfig.imageSize" :options="imageSizeSelectOptions" />
							</div>
						</div>
					</div>
				</div>
			</div>

			<!-- Right: Chat -->
			<div class="h-[62vh] lg:h-auto lg:min-h-0">
				<div class="card flex h-full flex-col overflow-hidden">
					<!-- 消息区 -->
					<div ref="messagesRef" class="min-h-0 flex-1 overflow-y-auto px-5 py-5 lg:px-6">
						<!-- 空态：图标磁贴 + 快捷提示词，给首屏一个视觉锚点 -->
						<div v-if="messages.length === 0" class="flex min-h-full flex-col items-center justify-center px-4 text-center">
							<div class="mb-5 flex h-16 w-16 items-center justify-center rounded-3xl bg-gradient-to-br from-cyan-400 via-primary-500 to-emerald-500 text-white shadow-glow">
								<Icon name="chat" size="lg" />
							</div>
							<h3 class="text-lg font-semibold text-gray-900">开始对话</h3>
							<p class="mt-1.5 max-w-sm text-sm text-gray-500">
								<template v-if="selectedModelItem">
									当前模型 <span class="font-medium text-gray-700">{{ selectedModelItem.model_name || selectedModelItem.model_id }}</span>，输入消息即可测试
								</template>
								<template v-else>请先在左侧选择一个模型</template>
							</p>

							<!-- 快捷提示词：点一下即填入输入框 -->
							<div v-if="selectedModel" class="mt-6 flex max-w-lg flex-wrap justify-center gap-2">
								<button
									v-for="p in quickPrompts"
									:key="p"
									class="rounded-full border border-gray-200 bg-white px-3 py-1.5 text-xs text-gray-600 transition-all duration-150 hover:-translate-y-0.5 hover:border-primary-300 hover:text-primary-600 hover:shadow-sm"
									@click="inputMessage = p; promptInput?.focus()"
								>
									{{ p }}
								</button>
							</div>

							<p class="mt-7 inline-flex items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-[11px] text-amber-700">
								<Icon name="exclamationTriangle" size="xs" />
								发送将真实调用模型并产生费用
							</p>
						</div>

						<!-- 消息列表 -->
						<div v-else class="space-y-5">
							<div v-for="(msg, idx) in messages" :key="idx" class="flex gap-3 animate-slide-up" :class="{ 'justify-end': msg.role === 'user' }">
								<!-- Assistant message -->
								<div v-if="msg.role === 'assistant'" class="flex max-w-[85%] gap-3">
									<div class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-primary-400 to-primary-600 text-white shadow-sm shadow-primary-500/25">
										<Icon name="cog" size="sm" />
									</div>
									<div class="min-w-0 flex-1 space-y-2">
										<!-- Thinking / Reasoning section -->
										<div v-if="msg.reasoningContent" class="overflow-hidden rounded-xl border border-amber-200 bg-amber-50/60">
											<button
												class="flex w-full items-center gap-2 px-3 py-2 text-left transition-colors duration-150 hover:bg-amber-100/60"
												@click="toggleThinking(idx)"
											>
												<svg
													class="h-3.5 w-3.5 shrink-0 text-amber-500 transition-transform duration-200"
													:class="{ 'rotate-90': expandedThinking[idx] }"
													fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
												>
													<path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
												</svg>
												<span class="text-xs font-medium text-amber-700">思考过程</span>
												<span class="text-xs text-amber-500">· 消耗 Reasoning Tokens</span>
											</button>
											<div v-if="expandedThinking[idx]" class="px-3 pb-3">
												<div class="max-h-80 overflow-y-auto rounded-lg border border-amber-100 bg-white/70 px-3 py-2 text-sm leading-relaxed text-gray-600 whitespace-pre-wrap">
													{{ msg.reasoningContent }}
												</div>
											</div>
										</div>
										<!-- Main content -->
										<div class="rounded-2xl rounded-tl-md border border-gray-200 bg-white px-4 py-3 shadow-sm">
											<MarkdownRenderer :content="msg.content" />
										</div>
									</div>
								</div>
								<!-- User message -->
								<div v-else-if="msg.role === 'user'" class="max-w-[80%] space-y-1.5">
									<!-- 随消息发出的附件：用 data URL 展示（输入区的 objectURL 发送后即被释放） -->
									<div v-if="msg.previews?.length" class="flex flex-wrap justify-end gap-1.5">
										<div
											v-for="p in msg.previews"
											:key="p.name"
											class="overflow-hidden rounded-xl border border-primary-200 bg-white"
											:title="p.name"
										>
											<img v-if="p.kind === 'image'" :src="p.url" class="h-20 w-20 object-cover" alt="" />
											<div v-else class="flex h-20 w-20 flex-col items-center justify-center gap-1 text-gray-500">
												<Icon :name="p.kind === 'audio' ? 'musicalNote' : 'film'" size="md" />
												<span class="text-[11px]">{{ p.name }}</span>
											</div>
										</div>
									</div>
									<div v-if="msg.content" class="rounded-2xl rounded-tr-md bg-gradient-to-br from-primary-500 to-primary-600 px-4 py-3 text-white shadow-sm shadow-primary-500/20">
										<p class="text-sm whitespace-pre-wrap">{{ msg.content }}</p>
									</div>
								</div>
								<!-- System message -->
								<div v-else-if="msg.role === 'system' && idx === 0" class="text-xs text-gray-400 italic">
									System: {{ msg.content.substring(0, 50) }}{{ msg.content.length > 50 ? '...' : '' }}
								</div>
							</div>
							<!-- Sending indicator -->
							<div v-if="sending && (params.stream ? (messages.length > 0 && messages[messages.length - 1].role === 'user') : true)" class="flex gap-3">
								<div class="flex h-8 w-8 items-center justify-center rounded-xl bg-gradient-to-br from-primary-400 to-primary-600">
									<div class="spinner h-4 w-4 text-white"></div>
								</div>
								<div class="rounded-2xl rounded-tl-md border border-gray-200 bg-white px-4 py-3 shadow-sm">
									<p class="text-sm text-gray-500">请求中...</p>
								</div>
							</div>
						</div>
					</div>

					<!-- Token usage bar -->
					<div v-if="tokenUsage.total > 0 || tokenUsage.cost || pendingBytes > 0" class="flex items-center justify-between gap-3 border-t border-gray-100 bg-gray-50/60 px-5 py-2 text-xs text-gray-500 lg:px-6">
						<div class="flex items-center gap-4">
							<span class="flex items-center gap-1.5">Prompt <span class="font-medium text-gray-700">{{ tokenUsage.prompt }}</span></span>
							<span class="flex items-center gap-1.5">Completion <span class="font-medium text-gray-700">{{ tokenUsage.completion }}</span></span>
							<span v-if="tokenUsage.reasoning > 0" class="flex items-center gap-1.5 text-amber-600">Reasoning <span class="font-medium">{{ tokenUsage.reasoning }}</span></span>
							<span class="flex items-center gap-1.5">Total <span class="font-medium text-gray-700">{{ tokenUsage.total }}</span></span>
						</div>
						<div class="flex items-center gap-4">
							<!-- 历史附件每轮都会重发，体积直接决定还能不能继续对话 -->
							<span v-if="pendingBytes > 0" :class="overBudget ? 'font-medium text-red-600' : ''">
								附件 {{ formatBytes(pendingBytes) }} / {{ formatBytes(MAX_TOTAL_BYTES) }}
							</span>
							<span v-if="tokenUsage.cost" class="font-medium text-amber-600">{{ tokenUsage.cost }}</span>
						</div>
					</div>

					<!-- 输入区：附件片 + 输入框 + 操作按钮整合进同一个圆角盒子，聚焦时整块高亮 -->
					<div class="shrink-0 border-t border-gray-100 p-3 lg:p-4">
						<div
							class="input-shell"
							:class="{ 'input-shell-dragging': dragging }"
							@dragover.prevent="dragging = true"
							@dragleave="dragging = false"
							@drop.prevent="onDropFiles"
						>
							<!-- 附件缩略片（bare：无边框无上传按钮，融入本容器）；
							     文件选择器仍挂载于此，由下方「附件」按钮通过 ref 触发 -->
							<AttachmentBar
								ref="attachmentBar"
								variant="bare"
								:class="attachmentList.length ? 'mb-2' : ''"
								:attachments="attachmentList"
								:accept="acceptTypes"
								:max="attachmentMax"
								:disabled="uploadDisabled"
								:disabled-reason="uploadDisabledReason"
								@add="addAttachments"
								@remove="removeAttachment"
								@insert="insertMention"
							/>

							<PromptInput
								ref="promptInput"
								v-model="inputMessage"
								bare
								:attachments="attachmentList"
								placeholder="输入消息，输入 @ 引用已上传的附件..."
								@submit="sendMessage"
								@paste="onPasteFiles"
							/>

							<!-- 操作行 -->
							<div class="mt-1.5 flex items-center justify-between gap-3">
								<div class="flex min-w-0 items-center gap-1">
									<button
										class="input-tool"
										:disabled="uploadDisabled"
										:title="uploadDisabled ? uploadDisabledReason : '上传附件（也可拖拽到此处或直接粘贴）'"
										@click="openFilePicker"
									>
										<Icon name="paperclip" size="xs" />
										附件
									</button>
									<button
										v-if="attachmentList.length"
										class="input-tool"
										title="清空已选附件"
										@click="clearAllAttachments"
									>
										<Icon name="x" size="xs" />
										清空附件
									</button>
									<span class="ml-1 hidden truncate text-[11px] text-gray-400 sm:inline">
										Enter 发送 · Shift + Enter 换行
									</span>
								</div>
								<div class="flex shrink-0 items-center gap-2">
									<button class="btn btn-ghost btn-sm text-gray-500" :disabled="sending" @click="clearChat">
										<Icon name="refresh" size="xs" />
										清空对话
									</button>
									<button class="btn btn-primary btn-sm px-4" :disabled="sending || (!inputMessage.trim() && !attachmentList.length)" @click="sendMessage">
										<Icon name="edit" size="xs" />
										发送
									</button>
								</div>
							</div>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped>
/* 输入容器（.input-shell / .input-tool）已提到 styles/main.css，
   与视频 Tab 共用一份定义 */

/* ── Temperature 滑块：替换浏览器默认的紫色原生样式 ── */
.range-slider {
	-webkit-appearance: none;
	appearance: none;
	width: 100%;
	height: 6px;
	border-radius: 9999px;
	outline: none;
	cursor: pointer;
	background: linear-gradient(
		to right,
		#14b8a6 0%,
		#14b8a6 var(--pct, 35%),
		#e2e8f0 var(--pct, 35%),
		#e2e8f0 100%
	);
}
.range-slider::-webkit-slider-thumb {
	-webkit-appearance: none;
	appearance: none;
	height: 16px;
	width: 16px;
	border: 2px solid #14b8a6;
	border-radius: 9999px;
	background: #fff;
	box-shadow: 0 2px 6px rgba(13, 148, 136, 0.35);
	transition: transform 150ms ease;
}
.range-slider::-webkit-slider-thumb:hover {
	transform: scale(1.15);
}
.range-slider::-moz-range-thumb {
	height: 14px;
	width: 14px;
	border: 2px solid #14b8a6;
	border-radius: 9999px;
	background: #fff;
	box-shadow: 0 2px 6px rgba(13, 148, 136, 0.35);
}
.range-slider::-moz-range-track {
	height: 6px;
	border-radius: 9999px;
	background: #e2e8f0;
}
.range-slider:focus-visible {
	box-shadow: 0 0 0 4px rgba(20, 184, 166, 0.15);
}
</style>
