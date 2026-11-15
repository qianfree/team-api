<script setup lang="ts">
import { ref, reactive, computed, nextTick, watch, onUnmounted } from 'vue'
import { NInput, NInputNumber, NSwitch } from 'naive-ui'
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
// 进行中请求的中止控制器；停止按钮的显隐由 sending 驱动，无需响应式
let currentAbort: AbortController | null = null
// 非流式超时触发的中止标记：与用户手动停止区分开
let abortByTimeout = false
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
	// null = 不发送 temperature 参数（Claude 新系列等模型已不支持该参数，传了会被上游拒绝）；默认留空即不发送
	temperature: null as number | null,
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
	/** 请求失败原因：非空时渲染为错误卡片并提供重试，不再伪装成回复正文 */
	error?: string
	/** 用户手动停止：与 error 互斥；无内容时渲染中性「已停止」卡片 */
	aborted?: boolean
}
const messages = ref<ChatMessage[]>([])
const inputMessage = ref('')
const tokenUsage = reactive({ prompt: 0, completion: 0, reasoning: 0, total: 0 })

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

// 恢复参数默认值：只重置生成参数，不动已选模型与对话内容
function resetParams() {
	params.temperature = null
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
		max_tokens: params.maxTokens,
		stream: params.stream,
	}
	// temperature 允许缺省：Claude 新系列等模型已不支持该参数，选「默认」时不发送
	if (params.temperature != null) {
		body.temperature = params.temperature
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

async function sendNonStream(requestBody: Record<string, any>, assistantIdx: number, signal?: AbortSignal) {
	const response = await fetch('/v1/chat/completions', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
			'Authorization': `Bearer ${props.apiKey}`,
		},
		body: JSON.stringify(requestBody),
		signal,
	})

	if (!response.ok) {
		let errMsg = `请求失败 (${response.status})`
		try {
			const errData = await response.json()
			if (errData?.error?.message) errMsg = errData.error.message
		} catch {}
		messages.value[assistantIdx].error = errMsg
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
}

async function sendStream(requestBody: Record<string, any>, assistantIdx: number, signal?: AbortSignal) {
	const response = await fetch('/v1/chat/completions', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
			'Authorization': `Bearer ${props.apiKey}`,
		},
		body: JSON.stringify(requestBody),
		signal,
	})

	if (!response.ok) {
		let errMsg = `请求失败 (${response.status})`
		try {
			const errData = await response.json()
			if (errData?.error?.message) errMsg = errData.error.message
		} catch {}
		messages.value[assistantIdx].error = errMsg
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
	messages.value.push({ role: 'assistant', content: '' })
	scrollToBottom()

	await dispatchRequest(assistantIdx)
}

// dispatchRequest 以 assistantIdx 为目标位发起请求，首发、失败重试与重新生成共用同一条链路；
// 网络层异常统一在此落为错误态，由模板渲染错误卡片而非伪装成回复正文
async function dispatchRequest(assistantIdx: number) {
	const apiMessages = messages.value
		.slice(0, assistantIdx)
		.map(m => ({ role: m.role, content: (m.parts ?? m.content) as string | ContentPart[] }))
	const requestBody = buildRequestBody(apiMessages)

	sending.value = true
	const abort = new AbortController()
	currentAbort = abort
	abortByTimeout = false
	// 非流式挂 60s 超时兜底：上游悬挂时自动中止，避免 sending 永远为 true；
	// 流式不设超时——推理模型首 token 可能等待数分钟，交给用户手动停止
	let timeoutTimer: number | null = null
	if (!params.stream) {
		timeoutTimer = window.setTimeout(() => {
			abortByTimeout = true
			abort.abort()
		}, 60_000)
	}
	try {
		if (params.stream) {
			await sendStream(requestBody, assistantIdx, abort.signal)
		} else {
			await sendNonStream(requestBody, assistantIdx, abort.signal)
		}
	} catch (e: any) {
		const failed = messages.value[assistantIdx]
		if (e?.name === 'AbortError') {
			if (abortByTimeout) {
				failed.error = '请求超时（60 秒无响应），已自动停止'
			} else {
				// 用户手动停止：保留已流出的内容，模板按中止态渲染
				failed.aborted = true
			}
		} else {
			failed.error = e?.message || '请求失败，请重试'
		}
	} finally {
		if (timeoutTimer) clearTimeout(timeoutTimer)
		if (currentAbort === abort) currentAbort = null
		sending.value = false
		scrollToBottom()
	}
}

// stopGeneration 用户手动停止当前请求：中止 fetch，已流出的内容保留
function stopGeneration() {
	currentAbort?.abort()
}

// regenerate 重新生成一条 assistant 消息：清掉错误/中止态与旧内容后对同一份历史重发
async function regenerate(idx: number) {
	if (sending.value) return
	const msg = messages.value[idx]
	if (!msg || msg.role !== 'assistant') return
	msg.error = undefined
	msg.aborted = undefined
	msg.content = ''
	msg.reasoningContent = ''
	await dispatchRequest(idx)
}

// copyMessage 复制回复原文（Markdown 源码），便于直接粘贴复用
async function copyMessage(msg: ChatMessage) {
	if (!msg.content) return
	try {
		await navigator.clipboard.writeText(msg.content)
		toast.success('已复制到剪贴板')
	} catch {
		toast.error('复制失败，请手动选择文本复制')
	}
}

function clearChat() {
	messages.value = []
	clearAttachments()
	tokenUsage.prompt = 0
	tokenUsage.completion = 0
	tokenUsage.reasoning = 0
	tokenUsage.total = 0
	expandedThinking.value = {}
}

// 路由离开时兜底中止进行中的请求（KeepAlive 下切 Tab 不卸载，仅路由离开触发）
onUnmounted(() => {
	currentAbort?.abort()
})
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
									<span class="text-[11px] text-gray-400">留空不发送</span>
								</div>
								<!-- 允许留空：null = 请求体不带 temperature，使用模型自身默认值（Claude 新系列等模型已不支持该参数） -->
								<n-input-number
									v-model:value="params.temperature"
									:min="0"
									:max="2"
									:step="0.1"
									placeholder="留空则使用模型默认"
									clearable
									class="w-full"
								/>
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
							<div v-for="(msg, idx) in messages" :key="idx" class="group flex gap-3 animate-slide-up" :class="{ 'justify-end': msg.role === 'user' }">
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
										<!-- 内容气泡：正在生成且尚无内容时显示打字占位 -->
										<div
											v-if="msg.content || (!msg.error && !msg.aborted && sending && idx === messages.length - 1)"
											class="rounded-2xl rounded-tl-md border border-gray-200 bg-white px-4 py-3 shadow-sm"
										>
											<MarkdownRenderer v-if="msg.content" :content="msg.content" />
											<div v-else class="flex items-center gap-1 py-1.5">
												<span class="typing-dot"></span>
												<span class="typing-dot"></span>
												<span class="typing-dot"></span>
											</div>
										</div>
										<!-- 错误态：独立红色卡片 + 重试 -->
										<div v-if="msg.error" class="rounded-xl border border-red-200 bg-red-50 px-4 py-3">
											<div class="flex items-center gap-2 text-red-700">
												<Icon name="xCircle" size="sm" />
												<span class="text-sm font-medium">请求失败</span>
											</div>
											<p class="mt-1.5 break-all text-sm text-red-600">{{ msg.error }}</p>
											<button
												class="mt-2 inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-2.5 py-1 text-xs font-medium text-red-600 transition-colors duration-150 hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-50"
												:disabled="sending"
												@click="regenerate(idx)"
											>
												<Icon name="refresh" size="xs" />
												重试
											</button>
										</div>
										<!-- 手动停止且无内容：中性提示 + 重新生成 -->
										<div v-if="msg.aborted && !msg.content" class="rounded-xl border border-gray-200 bg-gray-50 px-4 py-3">
											<div class="flex items-center gap-2 text-gray-600">
												<Icon name="infoCircle" size="sm" />
												<span class="text-sm">已停止生成</span>
											</div>
											<button
												class="mt-2 inline-flex items-center gap-1 rounded-lg border border-gray-200 bg-white px-2.5 py-1 text-xs font-medium text-gray-600 transition-colors duration-150 hover:border-primary-300 hover:text-primary-600 disabled:cursor-not-allowed disabled:opacity-50"
												:disabled="sending"
												@click="regenerate(idx)"
											>
												<Icon name="refresh" size="xs" />
												重新生成
											</button>
										</div>
										<!-- 消息操作：复制 / 重新生成（桌面 hover 浮现，移动端常显） -->
										<div v-if="msg.content && !msg.error" class="flex items-center gap-2 opacity-60 transition-opacity sm:opacity-0 sm:group-hover:opacity-100">
											<button class="msg-action" @click="copyMessage(msg)">
												<Icon name="copy" size="xs" />
												复制
											</button>
											<button class="msg-action" :disabled="sending" @click="regenerate(idx)">
												<Icon name="refresh" size="xs" />
												重新生成
											</button>
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
						</div>
					</div>

					<!-- Token usage bar -->
					<div v-if="tokenUsage.total > 0 || pendingBytes > 0" class="flex items-center justify-between gap-3 border-t border-gray-100 bg-gray-50/60 px-5 py-2 text-xs text-gray-500 lg:px-6">
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
									<!-- 生成中切换为停止按钮：中止当前请求，已流出的内容保留 -->
									<button v-if="sending" class="btn btn-danger btn-sm px-4" @click="stopGeneration">
										<Icon name="x" size="xs" />
										停止
									</button>
									<button v-else class="btn btn-primary btn-sm px-4" :disabled="!inputMessage.trim() && !attachmentList.length" @click="sendMessage">
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

/* ── 打字占位：首 token 到达前的三点动画，替代原先永不出现的「请求中」指示条 ── */
.typing-dot {
	display: inline-block;
	width: 6px;
	height: 6px;
	border-radius: 9999px;
	background: #94a3b8;
	animation: typing-bounce 1.2s infinite ease-in-out;
}
.typing-dot:nth-child(2) {
	animation-delay: 0.15s;
}
.typing-dot:nth-child(3) {
	animation-delay: 0.3s;
}
@keyframes typing-bounce {
	0%,
	60%,
	100% {
		transform: translateY(0);
		opacity: 0.4;
	}
	30% {
		transform: translateY(-4px);
		opacity: 1;
	}
}

/* ── 消息操作小按钮：复制 / 重新生成 ── */
.msg-action {
	display: inline-flex;
	align-items: center;
	gap: 0.25rem;
	padding: 0.25rem 0.5rem;
	border-radius: 0.5rem;
	font-size: 11px;
	color: #6b7280;
	transition: color 150ms ease, background-color 150ms ease;
}
.msg-action:hover {
	color: #0d9488;
	background: #f0fdfa;
}
.msg-action:disabled {
	cursor: not-allowed;
	opacity: 0.5;
}
</style>
