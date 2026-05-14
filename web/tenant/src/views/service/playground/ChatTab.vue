<script setup lang="ts">
import { ref, reactive, computed, nextTick } from 'vue'
import request from '@/utils/request'
import Icon from '@/components/common/Icon.vue'
import MarkdownRenderer from '@/components/common/MarkdownRenderer.vue'

interface ModelItem { model_id: string; model_name: string; category: string }
const props = defineProps<{
	models: ModelItem[]
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
})

const imageConfig = reactive({
	aspectRatio: '1:1',
	imageSize: '1K',
})
const aspectRatioOptions = ['1:1', '3:4', '4:3', '16:9', '9:16', '2:3', '3:2', '4:5', '5:4', '21:9']
const imageSizeOptions = ['512', '1K', '2K', '4K']

interface ChatMessage { role: string; content: string }
const messages = ref<ChatMessage[]>([])
const inputMessage = ref('')
const tokenUsage = reactive({ prompt: 0, completion: 0, total: 0, cost: '' })

const messagesRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLTextAreaElement | null>(null)

function autoResize() {
	const el = inputRef.value
	if (!el) return
	el.style.height = 'auto'
	el.style.height = Math.min(el.scrollHeight, 160) + 'px'
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

	const apiMessages = messages.value.map(m => ({ role: m.role, content: m.content }))
	const requestBody: Record<string, any> = {
		model: selectedModel.value,
		messages: apiMessages,
		temperature: params.temperature,
		max_tokens: params.maxTokens,
		stream: false,
	}

	if (isImageModel.value) {
		requestBody.image_config = {
			aspect_ratio: imageConfig.aspectRatio,
			image_size: imageConfig.imageSize,
		}
	}

	try {
		const res = await request.post('/tenant/playground/chat', requestBody, { _suppressErrorMsg: true } as any)
		if (res.data?.code === 0) {
			const data = res.data.data
			messages.value.push({ role: 'assistant', content: data.content || '(无响应内容)' })
			tokenUsage.prompt = data.prompt_tokens || 0
			tokenUsage.completion = data.completion_tokens || 0
			tokenUsage.total = data.total_tokens || 0
			tokenUsage.cost = data.estimated_cost || ''
		}
	} catch (e: any) {
		const errMsg = e?.message || '请求失败，请重试'
		messages.value.push({ role: 'assistant', content: errMsg })
	} finally {
		sending.value = false
		await nextTick()
		if (messagesRef.value) messagesRef.value.scrollTop = messagesRef.value.scrollHeight
	}
}

function clearChat() {
	messages.value = []
	tokenUsage.prompt = 0
	tokenUsage.completion = 0
	tokenUsage.total = 0
	tokenUsage.cost = ''
}
</script>

<template>
	<div>
		<div class="flex items-center justify-end mb-4">
			<button class="btn btn-secondary btn-sm" @click="clearChat">
				<Icon name="refresh" size="sm" />
				清空对话
			</button>
		</div>

		<div class="grid grid-cols-1 lg:grid-cols-4 gap-6">
			<!-- Left: Parameters -->
			<div class="lg:col-span-1">
				<div class="card sticky top-6">
					<div class="card-header">
						<h3 class="text-sm font-semibold text-gray-900">参数配置</h3>
					</div>
					<div class="card-body space-y-4">
						<div>
							<label class="input-label">模型</label>
							<select v-model="selectedModel" class="input">
								<option v-for="m in models" :key="m.model_id" :value="m.model_id">
									{{ m.model_name || m.model_id }}
								</option>
							</select>
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
						<template v-if="isImageModel">
							<div class="border-t border-gray-100 pt-4">
								<h4 class="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3">图片参数</h4>
							</div>
							<div>
								<label class="input-label">宽高比</label>
								<select v-model="imageConfig.aspectRatio" class="input">
									<option v-for="ratio in aspectRatioOptions" :key="ratio" :value="ratio">{{ ratio }}</option>
								</select>
							</div>
							<div>
								<label class="input-label">分辨率</label>
								<select v-model="imageConfig.imageSize" class="input">
									<option v-for="size in imageSizeOptions" :key="size" :value="size">{{ size }}</option>
								</select>
							</div>
						</template>
					</div>
				</div>
			</div>

			<!-- Right: Chat -->
			<div class="lg:col-span-3">
				<div class="card flex flex-col" style="min-height: 500px">
					<div class="px-6 py-2 bg-amber-50 border-b border-amber-200 text-center">
						<span class="text-xs text-amber-700 font-medium">Playground 模式 — 使用真实 API Key 调用，产生实际费用</span>
					</div>
					<div ref="messagesRef" class="flex-1 overflow-y-auto p-6 space-y-4">
						<div v-if="messages.length === 0" class="empty-state">
							<div class="empty-state-icon"><Icon name="bookOpen" size="xl" /></div>
							<h3 class="empty-state-title">开始对话</h3>
							<p class="empty-state-description">选择模型，输入消息，开始测试</p>
						</div>
						<div v-for="(msg, idx) in messages" :key="idx" class="flex gap-3 animate-slide-up" :class="{ 'justify-end': msg.role === 'user' }">
							<div v-if="msg.role === 'assistant'" class="flex gap-3 max-w-[80%]">
								<div class="h-8 w-8 rounded-xl bg-primary-100 flex items-center justify-center flex-shrink-0">
									<Icon name="cog" size="sm" class="text-primary-600" />
								</div>
								<div class="rounded-2xl rounded-tl-sm bg-gray-100 px-4 py-3">
									<MarkdownRenderer :content="msg.content" />
								</div>
							</div>
							<div v-else-if="msg.role === 'user'" class="max-w-[80%]">
								<div class="rounded-2xl rounded-tr-sm bg-primary-500 text-white px-4 py-3">
									<p class="text-sm whitespace-pre-wrap">{{ msg.content }}</p>
								</div>
							</div>
							<div v-else-if="msg.role === 'system' && idx === 0" class="text-xs text-gray-400 italic">
								System: {{ msg.content.substring(0, 50) }}{{ msg.content.length > 50 ? '...' : '' }}
							</div>
						</div>
						<div v-if="sending" class="flex gap-3">
							<div class="h-8 w-8 rounded-xl bg-primary-100 flex items-center justify-center">
								<div class="spinner h-4 w-4 text-primary-600"></div>
							</div>
							<div class="rounded-2xl rounded-tl-sm bg-gray-100 px-4 py-3">
								<p class="text-sm text-gray-500">请求中...</p>
							</div>
						</div>
					</div>
					<div v-if="tokenUsage.total > 0 || tokenUsage.cost" class="px-6 py-2 border-t border-gray-100 flex items-center justify-between text-xs text-gray-500">
						<div class="flex items-center gap-4">
							<span>Prompt: {{ tokenUsage.prompt }}</span>
							<span>Completion: {{ tokenUsage.completion }}</span>
							<span>Total: {{ tokenUsage.total }}</span>
						</div>
						<span v-if="tokenUsage.cost" class="font-medium text-amber-600">{{ tokenUsage.cost }}</span>
					</div>
					<div class="p-4 border-t border-gray-200">
						<div class="flex gap-3">
							<textarea ref="inputRef" v-model="inputMessage" class="input flex-1 resize-none" rows="1" placeholder="输入消息..." :disabled="sending" @keydown.enter.exact.prevent="sendMessage" @input="autoResize" />
							<button class="btn btn-primary self-end" :disabled="sending || !inputMessage.trim()" @click="sendMessage">
								<Icon name="edit" size="sm" />
								发送
							</button>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>
