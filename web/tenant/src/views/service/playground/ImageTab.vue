<script setup lang="ts">
import { ref, reactive, computed, watch, onUnmounted } from 'vue'
import { createPlaygroundApi } from '@/utils/playgroundApi'
import { calculateCost } from './calculateCost'
import ParamRefModal from './ParamRefModal.vue'
import Icon from '@/components/common/Icon.vue'
import BaseSelect from '../../../components/common/BaseSelect.vue'

interface ModelItem {
	model_id: string
	model_name: string
	category: string
	billing_mode?: string | null
	per_request_price?: number | null
	input_price?: number | null
	output_price?: number | null
	// async_image 为 true 时该图片模型必须走异步端点（DashScope 等）：
	// 提交 /v1/images/generations/async 拿 task_id 后轮询取图。
	async_image?: boolean
}
const props = defineProps<{ models: ModelItem[]; apiKey: string }>()

const sending = ref(false)
const errorMessage = ref('')
const selectedModel = ref(props.models[0]?.model_id || '')
const selectedModelItem = computed(() =>
	props.models.find(m => m.model_id === selectedModel.value),
)
const prompt = ref('')
const modelOptions = computed(() => props.models.map(m => ({ value: m.model_id, label: m.model_name || m.model_id })))

// ── 自定义参数系统 ──

interface CustomParam {
	id: number
	key: string
	value: string
}

const customParams = ref<CustomParam[]>([])
let nextParamId = 1
const showParamRef = ref(false)

// 模型预设模板
const MODEL_PRESETS: Record<string, Record<string, string>> = {
	'gpt-image-1': {
		size: '1024x1024',
		quality: 'auto',
		response_format: 'b64_json',
		output_format: 'png',
	},
	'dall-e-3': {
		size: '1024x1024',
		quality: 'standard',
		style: 'vivid',
		response_format: 'url',
	},
	'dall-e-2': {
		size: '1024x1024',
		response_format: 'url',
	},
}

// 当前模型的预设（支持子串匹配，如 openai/gpt-image-1）
const currentPreset = computed(() => {
	const id = selectedModel.value
	if (MODEL_PRESETS[id]) return MODEL_PRESETS[id]
	for (const key of Object.keys(MODEL_PRESETS)) {
		if (id.includes(key)) return MODEL_PRESETS[key]
	}
	return null
})

// 预设标签名（用于按钮显示）
const currentPresetLabel = computed(() => {
	const id = selectedModel.value
	for (const key of Object.keys(MODEL_PRESETS)) {
		if (id === key || id.includes(key)) {
			// 美化显示：gpt-image-1 → GPT Image 1
			return key.replace(/-/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
		}
	}
	return ''
})

// 有效参数数量
const validParamsCount = computed(() =>
	customParams.value.filter(p => p.key.trim() && p.value.trim()).length,
)

function addParam(key = '', value = '') {
	customParams.value.push({ id: nextParamId++, key, value })
}

function removeParam(id: number) {
	const idx = customParams.value.findIndex(p => p.id === id)
	if (idx !== -1) customParams.value.splice(idx, 1)
}

function applyPreset() {
	const preset = currentPreset.value
	if (!preset) return
	customParams.value = []
	for (const [key, value] of Object.entries(preset)) {
		addParam(key, value)
	}
}

// 智能类型推断：字符串 → boolean / number / string
function parseValue(raw: string): string | number | boolean {
	if (raw.toLowerCase() === 'true') return true
	if (raw.toLowerCase() === 'false') return false
	if (/^-?\d+(\.\d+)?$/.test(raw)) {
		const num = Number(raw)
		if (!isNaN(num) && isFinite(num)) return num
	}
	return raw
}

// 默认参数项（无预设时显示，用户自行填写值）
const DEFAULT_PARAMS = [
	{ key: 'size', value: '' },
	{ key: 'quality', value: '' },
]

function applyDefaults() {
	customParams.value = []
	for (const { key, value } of DEFAULT_PARAMS) {
		addParam(key, value)
	}
}

// 切换模型时自动应用预设，无预设则显示默认参数项
watch(selectedModel, () => {
	const preset = currentPreset.value
	if (preset) {
		applyPreset()
	} else {
		applyDefaults()
	}
}, { immediate: true })

// ── 图片生成 ──

interface ImageResult { b64_json?: string; url?: string; revised_prompt?: string }
const images = ref<ImageResult[]>([])
const tokenUsage = reactive({ promptTokens: 0, totalTokens: 0, cost: '' })

// 该模型是否必须走异步端点（DashScope 等）：由 /tenant/models 的 async_image 决定，
// 与后端同步端点拦截 gate 同源。异步模型走「提交 + 轮询」，同步模型一次性返回。
const isAsyncModel = computed(() => selectedModelItem.value?.async_image === true)

// 异步任务状态（仅异步模型使用）
interface AsyncTask {
	id: string
	status: string
	progress: string
	error?: string
	createdAt: number
}
const asyncTask = ref<AsyncTask | null>(null)
const polling = ref(false)
let pollTimer: ReturnType<typeof setTimeout> | null = null

const statusLabel: Record<string, string> = {
	SUBMITTED: '已提交',
	IN_PROGRESS: '生成中',
	NOT_START: '排队中',
	QUEUED: '排队中',
	SUCCESS: '已完成',
	FAILURE: '失败',
}
const statusColor: Record<string, string> = {
	SUBMITTED: 'badge-primary',
	IN_PROGRESS: 'badge-warning',
	NOT_START: 'badge-gray',
	QUEUED: 'badge-gray',
	SUCCESS: 'badge-success',
	FAILURE: 'badge-danger',
}

// 切换模型时停止上一模型的轮询并清空结果
watch(selectedModel, () => {
	stopPolling()
	asyncTask.value = null
	images.value = []
	errorMessage.value = ''
})

onUnmounted(() => {
	stopPolling()
	window.removeEventListener('keydown', onZoomKeydown)
})

// 组装请求体：固定字段 + 自定义参数（同步/异步共用）
function buildBody(): Record<string, any> {
	const body: Record<string, any> = {
		model: selectedModel.value,
		prompt: prompt.value,
	}
	for (const param of customParams.value) {
		const key = param.key.trim()
		const val = param.value.trim()
		if (key && val) {
			body[key] = parseValue(val)
		}
	}
	return body
}

async function generate() {
	if (!prompt.value.trim() || !selectedModel.value) return
	sending.value = true
	errorMessage.value = ''
	images.value = []
	asyncTask.value = null
	stopPolling()

	try {
		const api = createPlaygroundApi(props.apiKey)
		const body = buildBody()

		if (isAsyncModel.value) {
			// 异步模型：提交任务拿 task_id，随后轮询取图
			const res = await api.post('/v1/images/generations/async', body, { timeout: 60_000 })
			const data = res.data
			asyncTask.value = {
				id: data.id,
				status: data.status || 'SUBMITTED',
				progress: data.progress || '',
				createdAt: data.created_at || Math.floor(Date.now() / 1000),
			}
			if (data.status === 'SUCCESS') {
				if (data.url) images.value = [{ url: data.url }]
			} else if (data.status !== 'FAILURE') {
				startPolling()
			} else {
				asyncTask.value.error = data.error || '生成失败'
			}
		} else {
			// 同步模型：一次性返回
			const res = await api.post('/v1/images/generations', body, { timeout: 300_000 })
			const data = res.data
			images.value = data.data || []
			const usage = data.usage || {}
			tokenUsage.promptTokens = usage.prompt_tokens || 0
			tokenUsage.totalTokens = usage.total_tokens || 0
			tokenUsage.cost = calculateCost(selectedModelItem.value, usage) || ''
		}
	} catch (e) {
		console.error(e)
		errorMessage.value = (e as any)?.message || '请求失败，请重试'
	} finally {
		sending.value = false
	}
}

function startPolling() {
	polling.value = true
	pollLoop()
}

function stopPolling() {
	polling.value = false
	if (pollTimer) {
		clearTimeout(pollTimer)
		pollTimer = null
	}
}

async function pollLoop() {
	if (!polling.value || !asyncTask.value?.id) return

	try {
		const api = createPlaygroundApi(props.apiKey)
		const res = await api.get(`/v1/images/generations/async/${asyncTask.value.id}`)
		const data = res.data

		asyncTask.value = {
			...asyncTask.value,
			status: data.status,
			progress: data.progress || '',
			error: data.error || undefined,
		}

		if (data.status === 'SUCCESS') {
			polling.value = false
			if (data.url) images.value = [{ url: data.url }]
			return
		}
		if (data.status === 'FAILURE') {
			polling.value = false
			return
		}
	} catch {
		// 轮询失败不中断，继续尝试
	}

	if (polling.value) {
		pollTimer = setTimeout(pollLoop, 3000)
	}
}

function imageSrc(img: ImageResult): string {
	if (img.b64_json) return 'data:image/png;base64,' + img.b64_json
	if (img.url) return img.url
	return ''
}

function downloadImage(img: ImageResult, idx: number) {
	const src = imageSrc(img)
	if (!src) return
	const a = document.createElement('a')
	a.href = src
	a.download = `image_${idx + 1}.png`
	document.body.appendChild(a)
	a.click()
	document.body.removeChild(a)
}

// ── 图片放大预览 ──
const zoomedSrc = ref('')

function onZoomKeydown(e: KeyboardEvent) {
	if (e.key === 'Escape') closeZoom()
}

function openZoom(img: ImageResult) {
	const src = imageSrc(img)
	if (!src) return
	zoomedSrc.value = src
	window.addEventListener('keydown', onZoomKeydown)
}

function closeZoom() {
	zoomedSrc.value = ''
	window.removeEventListener('keydown', onZoomKeydown)
}
</script>

<template>
	<div class="grid grid-cols-1 lg:grid-cols-5 gap-6">
		<div class="lg:col-span-2">
			<div class="card sticky top-6">
				<div class="card-header">
					<h3 class="text-sm font-semibold text-gray-900">图片生成参数</h3>
				</div>
				<div class="card-body space-y-4">
					<!-- 模型 -->
					<div>
						<label class="input-label">模型</label>
						<BaseSelect v-model="selectedModel" :options="modelOptions" />
						<div v-if="isAsyncModel" class="mt-1.5 flex items-center gap-1.5">
							<span class="badge badge-warning">异步</span>
							<span class="text-xs text-gray-500">提交后轮询取图</span>
						</div>
					</div>

					<!-- 提示词 -->
					<div>
						<label class="input-label">提示词</label>
						<textarea v-model="prompt" class="input" rows="4" placeholder="描述你想生成的图片..." />
					</div>

					<!-- 自定义参数 -->
					<div class="border-t border-gray-100 pt-4">
						<div class="flex items-center justify-between mb-3">
							<div class="flex items-center gap-2">
								<h4 class="text-sm font-semibold text-gray-900">自定义参数</h4>
								<span v-if="validParamsCount > 0" class="badge badge-primary">{{ validParamsCount }}</span>
							</div>
							<div class="flex items-center gap-1.5">
								<button
									class="text-xs px-2.5 py-1.5 rounded-lg text-gray-600 hover:border-primary-300 hover:text-primary-600 transition-colors duration-150 cursor-pointer flex items-center gap-1.5 border border-gray-200"
									@click="showParamRef = true"
								>
									<Icon name="bookOpen" size="xs" />
									参数参考
								</button>
								<button
									v-if="currentPreset && customParams.length > 0"
									class="text-xs px-2 py-1 rounded-lg text-primary-600 hover:bg-primary-50 transition-colors duration-150 cursor-pointer"
									@click="applyPreset"
								>
									重置预设
								</button>
							</div>
						</div>

						<!-- 空状态 -->
						<div
							v-if="customParams.length === 0"
							class="rounded-xl border-2 border-dashed border-gray-200 py-6 text-center cursor-pointer hover:border-primary-300 hover:bg-primary-50/30 transition-colors duration-200"
							@click="currentPreset ? applyPreset() : applyDefaults()"
						>
							<Icon name="plus" size="sm" class="text-gray-300 mx-auto mb-2" />
							<p class="text-xs text-gray-400">
								{{ currentPreset ? '点击应用 ' + currentPresetLabel + ' 预设' : '点击添加自定义参数' }}
							</p>
						</div>

						<!-- 参数行 -->
						<div v-else class="space-y-2">
							<div
								v-for="param in customParams"
								:key="param.id"
								class="flex items-center gap-2 group"
							>
								<input
									v-model="param.key"
									class="input flex-1"
									placeholder="参数名"
								/>
								<input
									v-model="param.value"
									class="input flex-1"
									placeholder="值"
								/>
								<button
									class="h-9 w-9 rounded-xl flex items-center justify-center text-gray-300 hover:text-red-500 hover:bg-red-50 transition-colors duration-150 flex-shrink-0 cursor-pointer"
									@click="removeParam(param.id)"
								>
									<Icon name="x" size="sm" />
								</button>
							</div>
							<button
								class="w-full text-xs py-1.5 rounded-lg border border-dashed border-gray-200 text-gray-400 hover:border-primary-300 hover:text-primary-600 transition-colors duration-150 cursor-pointer"
								@click="addParam()"
							>
								+ 添加参数
							</button>
						</div>
					</div>

					<button class="btn btn-primary w-full" :disabled="sending || polling || !prompt.trim()" @click="generate">
						{{ sending ? (isAsyncModel ? '提交中...' : '生成中...') : polling ? '生成中...' : '生成图片' }}
					</button>
				</div>
			</div>
		</div>

		<div class="lg:col-span-3">
			<div class="card" style="min-height: 400px">
				<div class="card-header">
					<h3 class="text-sm font-semibold text-gray-900">生成结果</h3>
				</div>
				<div class="card-body">
					<!-- 异步模型说明 -->
					<div v-if="isAsyncModel" class="mb-4 rounded-xl border border-primary-200 bg-primary-50/60 p-3">
						<div class="flex items-start gap-2">
							<Icon name="infoCircle" size="sm" class="text-primary-500 flex-shrink-0 mt-0.5" />
							<p class="text-xs text-gray-600 leading-relaxed">
								<span class="font-medium text-gray-700">异步任务模型</span>：提交至
								<code class="code text-[11px]">POST /v1/images/generations/async</code>
								拿到 task_id 后轮询
								<code class="code text-[11px]">GET /v1/images/generations/async/{task_id}</code>
								取图。
							</p>
						</div>
					</div>

					<!-- 错误提示 -->
					<div
						v-if="errorMessage"
						class="mb-4 rounded-xl border border-red-200 bg-red-50 p-4"
					>
						<div class="flex items-center gap-2 text-red-700">
							<Icon name="xCircle" size="sm" />
							<span class="text-sm font-medium">生成失败</span>
						</div>
						<p class="mt-2 text-sm text-red-600">{{ errorMessage }}</p>
					</div>

					<div v-if="images.length === 0 && !sending && !errorMessage && !asyncTask" class="empty-state">
						<div class="empty-state-icon"><Icon name="bookOpen" size="xl" /></div>
						<h3 class="empty-state-title">等待生成</h3>
						<p class="empty-state-description">输入提示词并点击生成</p>
					</div>
					<div v-if="sending" class="flex items-center justify-center py-12">
						<div class="spinner h-8 w-8 text-primary-600"></div>
						<span class="ml-3 text-sm text-gray-500">{{ isAsyncModel ? '提交图片生成任务中...' : '图片生成中...' }}</span>
					</div>

					<!-- 异步任务状态 -->
					<div v-if="asyncTask && !sending" class="mb-4 space-y-3">
						<div class="flex items-center gap-3">
							<span class="badge" :class="statusColor[asyncTask.status] || 'badge-gray'">
								{{ statusLabel[asyncTask.status] || asyncTask.status }}
							</span>
							<span v-if="asyncTask.progress" class="text-xs text-gray-500">进度: {{ asyncTask.progress }}</span>
							<span v-if="polling" class="text-xs text-primary-600 flex items-center gap-1">
								<div class="spinner h-3 w-3"></div>
								轮询中
							</span>
							<span v-if="asyncTask.id" class="text-xs text-gray-400 ml-auto">task_id: {{ asyncTask.id }}</span>
						</div>
						<div v-if="asyncTask.status === 'IN_PROGRESS' || asyncTask.status === 'SUBMITTED' || asyncTask.status === 'QUEUED'" class="w-full bg-gray-100 rounded-full h-2 overflow-hidden">
							<div class="h-full rounded-full bg-primary-500 transition-all duration-500" :style="{ width: asyncTask.progress || '10%' }" />
						</div>
						<div v-if="asyncTask.status === 'FAILURE'" class="rounded-xl border border-red-200 bg-red-50 p-4">
							<div class="flex items-center gap-2 text-red-700">
								<Icon name="xCircle" size="sm" />
								<span class="text-sm font-medium">生成失败</span>
							</div>
							<p v-if="asyncTask.error" class="mt-2 text-sm text-red-600">{{ asyncTask.error }}</p>
						</div>
					</div>

					<div v-if="images.length > 0" class="grid grid-cols-1 sm:grid-cols-2 gap-4">
						<div v-for="(img, idx) in images" :key="idx" class="rounded-xl overflow-hidden border border-gray-200 relative group">
							<img :src="imageSrc(img)" class="w-full cursor-zoom-in" alt="Generated image" @click="openZoom(img)" />
							<div class="absolute top-2 right-2 flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity duration-200">
								<button
									class="btn btn-sm bg-white/90 backdrop-blur-sm border border-gray-200 shadow-sm"
									title="放大查看"
									@click="openZoom(img)"
								>
									<Icon name="expand" size="sm" />
									放大
								</button>
								<button
									class="btn btn-sm bg-white/90 backdrop-blur-sm border border-gray-200 shadow-sm"
									title="下载图片"
									@click="downloadImage(img, idx)"
								>
									<Icon name="arrowDown" size="sm" />
									下载
								</button>
							</div>
							<p v-if="img.revised_prompt" class="text-xs text-gray-500 p-3">{{ img.revised_prompt }}</p>
						</div>
					</div>
					<div v-if="tokenUsage.totalTokens > 0" class="mt-4 pt-3 border-t border-gray-100 text-xs text-gray-500 flex items-center justify-between">
						<span>Tokens: {{ tokenUsage.promptTokens }} / {{ tokenUsage.totalTokens }}</span>
						<span v-if="tokenUsage.cost" class="font-medium text-amber-600">{{ tokenUsage.cost }}</span>
					</div>
				</div>
			</div>
		</div>
	</div>

	<!-- 参数参考弹窗 -->
	<ParamRefModal :show="showParamRef" mode="image" @close="showParamRef = false" />

	<!-- 图片放大预览 -->
	<Teleport to="body">
		<Transition name="modal">
			<div v-if="zoomedSrc" class="modal-overlay" @click="closeZoom">
				<div class="relative max-h-[90vh] max-w-[90vw]" @click.stop>
					<img :src="zoomedSrc" class="max-h-[90vh] max-w-[90vw] rounded-xl object-contain shadow-2xl" alt="放大预览" />
					<button
						class="absolute -top-3 -right-3 flex h-9 w-9 items-center justify-center rounded-full border border-gray-200 bg-white text-gray-600 shadow-lg transition-colors hover:text-gray-900"
						title="关闭"
						@click="closeZoom"
					>
						<Icon name="x" size="sm" />
					</button>
				</div>
			</div>
		</Transition>
	</Teleport>
</template>
