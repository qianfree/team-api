<script setup lang="ts">
import { ref, reactive, computed, nextTick } from 'vue'
import { calculateCost } from './calculateCost'
import Icon from '@/components/common/Icon.vue'
import MarkdownRenderer from '@/components/common/MarkdownRenderer.vue'
import BaseSelect from '../../../components/common/BaseSelect.vue'

interface ModelItem {
	model_id: string
	model_name: string
	category: string
	billing_mode?: string | null
	per_request_price?: number | null
	input_price?: number | null
	output_price?: number | null
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

if (props.models.length > 0) {
	selectedModel.value = props.models[0].model_id
}

const params = reactive({
	temperature: 0.7,
	maxTokens: 2048,
	systemPrompt: '',
	topP: 1.0,
	stream: true,
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
	content: string
	reasoningContent?: string
}
const messages = ref<ChatMessage[]>([])
const inputMessage = ref('')
const tokenUsage = reactive({ prompt: 0, completion: 0, reasoning: 0, total: 0, cost: '' })

// 记录每条 assistant 消息的思考内容折叠状态（用 message index 作为 key）
const expandedThinking = ref<Record<number, boolean>>({})

function toggleThinking(idx: number) {
	expandedThinking.value[idx] = !expandedThinking.value[idx]
}

const messagesRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLTextAreaElement | null>(null)

function autoResize() {
	const el = inputRef.value
	if (!el) return
	el.style.height = 'auto'
	el.style.height = Math.min(el.scrollHeight, 160) + 'px'
}

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

function buildRequestBody(apiMessages: { role: string; content: string }[]) {
	const body: Record<string, any> = {
		model: selectedModel.value,
		messages: apiMessages,
		temperature: params.temperature,
		max_tokens: params.maxTokens,
		stream: params.stream,
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
	const content = inputMessage.value.trim()
	if (!content || !selectedModel.value) return

	if (params.systemPrompt && messages.value.length === 0) {
		messages.value.push({ role: 'system', content: params.systemPrompt })
	}

	messages.value.push({ role: 'user', content })
	inputMessage.value = ''
	sending.value = true
	if (inputRef.value) inputRef.value.style.height = 'auto'

	const assistantIdx = messages.value.length
	messages.value.push({ role: 'assistant', content: '', reasoningContent: '' })
	scrollToBottom()

	const apiMessages = messages.value.slice(0, assistantIdx).map(m => ({ role: m.role, content: m.content }))
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
	tokenUsage.prompt = 0
	tokenUsage.completion = 0
	tokenUsage.reasoning = 0
	tokenUsage.total = 0
	tokenUsage.cost = ''
	expandedThinking.value = {}
}
</script>

<template>
	<div class="h-[calc(100vh-20rem)] flex flex-col overflow-hidden">
		<div class="grid grid-cols-1 lg:grid-cols-4 gap-6 min-h-0 flex-1">
			<!-- Left: Parameters -->
			<div class="lg:col-span-1">
				<div class="card sticky top-6">
					<div class="card-header">
						<h3 class="text-sm font-semibold text-gray-900">参数配置</h3>
					</div>
					<div class="card-body space-y-4">
						<div>
							<label class="input-label">模型</label>
							<BaseSelect v-model="selectedModel" :options="modelOptions" />
							<div v-if="isImageModel" class="mt-1.5 flex items-center gap-1.5">
								<span class="badge badge-primary">图片模型</span>
								<span class="text-xs text-gray-500">支持生成图片</span>
							</div>
						</div>
						<div>
							<label class="input-label">Temperature: {{ params.temperature.toFixed(1) }}</label>
							<input v-model.number="params.temperature" type="range" min="0" max="2" step="0.1" class="w-full" />
						</div>
						<div>
							<label class="input-label">Max Tokens</label>
							<input v-model.number="params.maxTokens" type="number" class="input" min="1" max="128000" />
						</div>
						<div>
							<label class="input-label">System Prompt</label>
							<textarea v-model="params.systemPrompt" class="input" rows="3" placeholder="设置系统提示词..."></textarea>
						</div>
						<div class="flex items-center justify-between">
							<label class="input-label mb-0">流式响应</label>
							<button
								type="button"
								role="switch"
								:aria-checked="params.stream"
								class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500/50"
								:class="params.stream ? 'bg-primary-500' : 'bg-gray-200'"
								@click="params.stream = !params.stream"
							>
								<span
									class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
									:class="params.stream ? 'translate-x-5' : 'translate-x-0'"
								/>
							</button>
						</div>
						<template v-if="isImageModel">
							<div class="border-t border-gray-100 pt-4">
								<h4 class="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3">图片参数</h4>
							</div>
							<div>
								<label class="input-label">宽高比</label>
								<BaseSelect v-model="imageConfig.aspectRatio" :options="aspectRatioSelectOptions" />
							</div>
							<div>
								<label class="input-label">分辨率</label>
								<BaseSelect v-model="imageConfig.imageSize" :options="imageSizeSelectOptions" />
							</div>
						</template>
					</div>
				</div>
			</div>

			<!-- Right: Chat -->
			<div class="lg:col-span-3 min-h-0">
				<div class="card flex flex-col h-full overflow-hidden">
					<div class="px-6 py-2 bg-amber-50 border-b border-amber-200 text-center">
						<span class="text-xs text-amber-700 font-medium">Playground 模式 — 使用真实 API Key 调用，产生实际费用</span>
					</div>
					<div ref="messagesRef" class="flex-1 overflow-y-auto min-h-0 p-6 space-y-4">
						<div v-if="messages.length === 0" class="empty-state">
							<div class="empty-state-icon"><Icon name="bookOpen" size="xl" /></div>
							<h3 class="empty-state-title">开始对话</h3>
							<p class="empty-state-description">选择模型，输入消息，开始测试</p>
						</div>
						<div v-for="(msg, idx) in messages" :key="idx" class="flex gap-3 animate-slide-up" :class="{ 'justify-end': msg.role === 'user' }">
							<!-- Assistant message -->
							<div v-if="msg.role === 'assistant'" class="flex gap-3 max-w-[85%]">
								<div class="h-8 w-8 rounded-xl bg-primary-100 flex items-center justify-center flex-shrink-0">
									<Icon name="cog" size="sm" class="text-primary-600" />
								</div>
								<div class="min-w-0 flex-1 space-y-2">
									<!-- Thinking / Reasoning section -->
									<div v-if="msg.reasoningContent" class="rounded-xl border border-amber-200 bg-amber-50/60 overflow-hidden">
										<button
											class="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-amber-100/60 transition-colors duration-150"
											@click="toggleThinking(idx)"
										>
											<svg
												class="h-3.5 w-3.5 text-amber-500 transition-transform duration-200 flex-shrink-0"
												:class="{ 'rotate-90': expandedThinking[idx] }"
												fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
											>
												<path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
											</svg>
											<span class="text-xs font-medium text-amber-700">思考过程</span>
											<span class="text-xs text-amber-500">· 消耗 Reasoning Tokens</span>
										</button>
										<div v-if="expandedThinking[idx]" class="px-3 pb-3">
											<div class="rounded-lg bg-white/70 border border-amber-100 px-3 py-2 text-sm text-gray-600 whitespace-pre-wrap leading-relaxed max-h-80 overflow-y-auto">
												{{ msg.reasoningContent }}
											</div>
										</div>
									</div>
									<!-- Main content -->
									<div class="rounded-2xl rounded-tl-sm bg-gray-100 px-4 py-3">
										<MarkdownRenderer :content="msg.content" />
									</div>
								</div>
							</div>
							<!-- User message -->
							<div v-else-if="msg.role === 'user'" class="max-w-[80%]">
								<div class="rounded-2xl rounded-tr-sm bg-primary-500 text-white px-4 py-3">
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
							<div class="h-8 w-8 rounded-xl bg-primary-100 flex items-center justify-center">
								<div class="spinner h-4 w-4 text-primary-600"></div>
							</div>
							<div class="rounded-2xl rounded-tl-sm bg-gray-100 px-4 py-3">
								<p class="text-sm text-gray-500">请求中...</p>
							</div>
						</div>
					</div>
					<!-- Token usage bar -->
					<div v-if="tokenUsage.total > 0 || tokenUsage.cost" class="px-6 py-2 border-t border-gray-100 flex items-center justify-between text-xs text-gray-500">
						<div class="flex items-center gap-4">
							<span>Prompt: {{ tokenUsage.prompt }}</span>
							<span>Completion: {{ tokenUsage.completion }}</span>
							<span v-if="tokenUsage.reasoning > 0" class="text-amber-600">Reasoning: {{ tokenUsage.reasoning }}</span>
							<span>Total: {{ tokenUsage.total }}</span>
						</div>
						<span v-if="tokenUsage.cost" class="font-medium text-amber-600">{{ tokenUsage.cost }}</span>
					</div>
					<!-- Input area -->
					<div class="p-4 border-t border-gray-200">
						<div class="flex gap-3">
							<textarea ref="inputRef" v-model="inputMessage" class="input flex-1 resize-none" rows="1" placeholder="输入消息..." :disabled="sending" @keydown.enter.exact.prevent="sendMessage" @input="autoResize" />
							<button class="btn btn-primary self-end" :disabled="sending || !inputMessage.trim()" @click="sendMessage">
								<Icon name="edit" size="sm" />
								发送
							</button>
							<button class="btn btn-secondary self-end" :disabled="sending" @click="clearChat">
								<Icon name="refresh" size="sm" />
								清空
							</button>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>
