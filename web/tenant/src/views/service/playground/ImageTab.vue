<script setup lang="ts">
import { ref, reactive, computed, watch, onUnmounted } from 'vue'
import { NInput } from 'naive-ui'
import { createPlaygroundApi } from '@/utils/playgroundApi'
import { createPoller } from '@/composables/usePolling'
import { calculateCost } from './calculateCost'
import ParamRefModal from './ParamRefModal.vue'
import GenerationProgressCard from './GenerationProgressCard.vue'
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
	// async_image 为 true 时该图片模型的异步端点可用（真异步厂商如 DashScope，或同步厂商且
	// 后台「同步图片异步化」开启）：提交 /v1/images/generations/async 拿 task_id 后轮询取图。
	async_image?: boolean
	// image_sync_supported 为 true 时该图片模型的同步端点（/v1/images/generations，阻塞一次性
	// 返回）可用；「仅异步」厂商（阿里 image-synthesis 等）为 false。缺省（旧后端）视为可用。
	image_sync_supported?: boolean
}
const props = defineProps<{ models: ModelItem[]; apiKey: string }>()

const sending = ref(false)
const errorMessage = ref('')
const selectedModel = ref(props.models[0]?.model_id || '')
// models 由父组件异步加载，setup 阶段可能为空；列表变化时若当前选中模型不在列表内
//（如切换 API Key 后模型范围变化）则自动重置为第一个
watch(
	() => props.models,
	models => {
		if (!selectedModel.value || !models.some(m => m.model_id === selectedModel.value)) {
			selectedModel.value = models[0]?.model_id || ''
		}
	},
)
const selectedModelItem = computed(() =>
	props.models.find(m => m.model_id === selectedModel.value),
)
const prompt = ref('')
const modelOptions = computed(() => props.models.map(m => ({ value: m.model_id, label: m.model_name || m.model_id })))

// 空态里展示的模型名（列表由父组件异步加载，可能暂时查不到）
const selectedModelName = computed(() => selectedModelItem.value?.model_name || selectedModel.value)

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

// 参数值输入框的引导占位：无预设时默认给的是空值，直接显示「值」用户不知道填什么，
// 按参数名给出常见取值示例
const PARAM_HINTS: Record<string, string> = {
	size: '如 1024x1024',
	quality: '如 high / medium / low',
	style: '如 vivid / natural',
	response_format: '如 url / b64_json',
	output_format: '如 png / jpeg / webp',
	background: '如 transparent / opaque',
}

function paramPlaceholder(key: string) {
	return PARAM_HINTS[key.trim()] || '值'
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

// 模型能力：异步端点是否可用 / 同步端点是否可用（同步缺省视为可用，兼容旧后端）。
// 由 /tenant/models 的 async_image、image_sync_supported 决定，与后端端点 gate 同源。
const asyncSupported = computed(() => selectedModelItem.value?.async_image === true)
const syncSupported = computed(() => selectedModelItem.value?.image_sync_supported !== false)
// 仅当两种模式都可用时才让用户手动切换；否则锁定到唯一可用模式。
const canToggleMode = computed(() => asyncSupported.value && syncSupported.value)

// 用户选择的图片调用模式，默认异步（保留提交+轮询、不长时间挂连接的体验）。切模型时重置。
const imageMode = ref<'sync' | 'async'>('async')
// 生效模式：受能力约束——仅同步则锁同步，仅异步则锁异步，两者可用则听用户选择。
const effectiveMode = computed<'sync' | 'async'>(() => {
	if (!asyncSupported.value) return 'sync'
	if (!syncSupported.value) return 'async'
	return imageMode.value
})
// 优雅降级提示：异步被后台临时关闭时自动改走同步，给用户一条轻提示。
const fallbackNotice = ref('')

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
// 任务状态轮询：页面隐藏时暂停（任务在服务端继续执行），恢复可见时立即补拉
const taskPoller = createPoller(pollLoop, 3000, { immediate: true })
// 轮询上限：100 次实际轮询 ≈ 5 分钟（页面隐藏期间不计数），超过判定超时，防止无限轮询。
const MAX_POLL_ATTEMPTS = 100
let pollAttempts = 0

const statusLabel: Record<string, string> = {
	SUBMITTED: '已提交',
	IN_PROGRESS: '生成中',
	NOT_START: '排队中',
	QUEUED: '排队中',
	SUCCESS: '已完成',
	FAILURE: '失败',
}

// 结果卡右上角的状态徽标（无任务且无错误时不渲染）
const resultStatus = computed<{ label: string; cls: string } | null>(() => {
	if (errorMessage.value) return { label: '失败', cls: 'badge-danger' }
	if (sending.value) {
		return { label: effectiveMode.value === 'async' ? '提交中' : '生成中', cls: 'badge-primary' }
	}
	if (asyncTask.value) {
		if (asyncTask.value.status === 'SUCCESS') return { label: '已完成', cls: 'badge-success' }
		if (asyncTask.value.status === 'FAILURE') return { label: '失败', cls: 'badge-danger' }
		return { label: statusLabel[asyncTask.value.status] || asyncTask.value.status, cls: 'badge-warning' }
	}
	return null
})

// 切换模型时停止上一模型的轮询并清空结果，重置模式为默认（异步）
watch(selectedModel, () => {
	stopPolling()
	asyncTask.value = null
	images.value = []
	errorMessage.value = ''
	fallbackNotice.value = ''
	imageMode.value = 'async'
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
	fallbackNotice.value = ''
	images.value = []
	asyncTask.value = null
	stopPolling()

	try {
		const api = createPlaygroundApi(props.apiKey)
		const body = buildBody()

		if (effectiveMode.value === 'async') {
			try {
				await submitAsync(api, body)
			} catch (e) {
				// 优雅降级：后台此刻恰好关闭了「同步图片异步化」（翻转开关的时序窗口），异步端点
				// 返回 image_async_disabled。若该模型同步端点可用，自动改走同步，用户无感。
				const code = (e as any)?.relayError?.code
				if (code === 'image_async_disabled' && syncSupported.value) {
					fallbackNotice.value = '异步暂不可用，已自动切换为同步模式'
					await generateSync(api, body)
				} else {
					throw e
				}
			}
		} else {
			await generateSync(api, body)
		}
	} catch (e) {
		console.error(e)
		errorMessage.value = (e as any)?.message || '请求失败，请重试'
	} finally {
		sending.value = false
	}
}

// submitAsync 走异步端点：提交拿 task_id，非终态则启动轮询取图。
async function submitAsync(api: ReturnType<typeof createPlaygroundApi>, body: Record<string, any>) {
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
}

// generateSync 走同步端点：阻塞一次性返回图片与用量。
async function generateSync(api: ReturnType<typeof createPlaygroundApi>, body: Record<string, any>) {
	const res = await api.post('/v1/images/generations', body, { timeout: 300_000 })
	const data = res.data
	images.value = data.data || []
	const usage = data.usage || {}
	tokenUsage.promptTokens = usage.prompt_tokens || 0
	tokenUsage.totalTokens = usage.total_tokens || 0
	tokenUsage.cost = calculateCost(selectedModelItem.value, usage) || ''
}

function startPolling() {
	polling.value = true
	pollAttempts = 0
	taskPoller.start()
}

function stopPolling() {
	polling.value = false
	taskPoller.stop()
}

async function pollLoop() {
	if (!polling.value || !asyncTask.value?.id) return

	// 轮询次数上限保护：超过上限（约 5 分钟）判定为超时，停止轮询并提示，
	// 避免上游卡死 / 后端漏推终态时前端无限轮询。
	if (pollAttempts >= MAX_POLL_ATTEMPTS) {
		stopPolling()
		asyncTask.value = {
			...asyncTask.value,
			status: 'FAILURE',
			error: '生成超时，请稍后重试',
		}
		return
	}
	pollAttempts++

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
			stopPolling()
			// 多图：优先用 data 数组渲染全部图片；回退到单 url（向后兼容旧后端）。
			if (Array.isArray(data.data) && data.data.length > 0) {
				images.value = data.data
					.filter((d: any) => d && d.url)
					.map((d: any) => ({ url: d.url }))
			} else if (data.url) {
				images.value = [{ url: data.url }]
			}
			return
		}
		if (data.status === 'FAILURE') {
			stopPolling()
			return
		}
	} catch {
		// 轮询失败不中断，继续尝试
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
	<!-- 与对话/视频 Tab 同一套等高布局：左参数栏内部滚动，主操作常驻底部，右结果区撑满 -->
	<div class="flex flex-col lg:h-[calc(100vh-15rem)] lg:overflow-hidden">
		<div class="grid grid-cols-1 gap-4 lg:grid-cols-[22rem_minmax(0,1fr)] lg:grid-rows-[minmax(0,1fr)] lg:min-h-0 lg:flex-1">
			<!-- Left: 参数 -->
			<div class="flex lg:min-h-0">
				<div class="card flex w-full flex-col overflow-hidden lg:h-full">
					<div class="flex shrink-0 items-center gap-2 border-b border-gray-100 px-4 py-3">
						<Icon name="photo" size="sm" class="text-primary-500" />
						<h3 class="text-sm font-semibold text-gray-900">图片生成参数</h3>
					</div>

					<div class="flex-1 space-y-4 overflow-y-auto px-4 py-4">
						<!-- 模型 + 调用模式 -->
						<div>
							<div class="mb-2 flex items-center justify-between">
								<label class="text-xs font-semibold text-gray-500">模型</label>
								<span class="text-[11px] text-gray-400">共 {{ models.length }} 个可用</span>
							</div>
							<BaseSelect v-model="selectedModel" :options="modelOptions" />

							<!-- 调用模式：两种都可用时让用户切换，否则锁定唯一可用模式 -->
							<div class="mt-2">
								<div v-if="canToggleMode" class="flex items-center justify-between gap-2">
									<div class="tabs">
										<button
											class="tab"
											:class="{ 'tab-active': imageMode === 'async' }"
											@click="imageMode = 'async'"
										>异步</button>
										<button
											class="tab"
											:class="{ 'tab-active': imageMode === 'sync' }"
											@click="imageMode = 'sync'"
										>同步</button>
									</div>
									<span class="text-[11px] text-gray-400">
										{{ imageMode === 'async' ? '提交后轮询取图' : '一次性返回' }}
									</span>
								</div>
								<div v-else class="flex items-start gap-1.5">
									<span class="badge mt-0.5 shrink-0" :class="effectiveMode === 'async' ? 'badge-warning' : 'badge-gray'">
										{{ effectiveMode === 'async' ? '异步' : '同步' }}
									</span>
									<span class="text-[11px] leading-relaxed text-gray-400">
										{{ effectiveMode === 'async' ? '该模型仅支持异步（提交后轮询取图）' : '该模型仅支持同步（一次性返回，等待较久）' }}
									</span>
								</div>
							</div>
						</div>

						<!-- 提示词 -->
						<div class="border-t border-gray-100 pt-4">
							<label class="mb-2 block text-xs font-semibold text-gray-500">提示词</label>
							<n-input v-model:value="prompt" type="textarea" :rows="4" placeholder="描述你想生成的图片..." />
						</div>

						<!-- 自定义参数 -->
						<div class="border-t border-gray-100 pt-4">
							<div class="mb-3 flex items-center justify-between gap-2">
								<div class="flex items-center gap-2">
									<span class="text-xs font-semibold text-gray-500">自定义参数</span>
									<span v-if="validParamsCount > 0" class="badge badge-primary">{{ validParamsCount }}</span>
								</div>
								<div class="flex items-center gap-1">
									<button
										v-if="currentPreset && customParams.length > 0"
										class="rounded-md px-2 py-1 text-[11px] text-primary-600 transition-colors duration-150 hover:bg-primary-50"
										@click="applyPreset"
									>
										重置预设
									</button>
									<button
										class="flex items-center gap-1 rounded-md border border-gray-200 px-2 py-1 text-[11px] text-gray-600 transition-colors duration-150 hover:border-primary-300 hover:text-primary-600"
										@click="showParamRef = true"
									>
										<Icon name="bookOpen" size="xs" />
										参数参考
									</button>
								</div>
							</div>

							<!-- 空状态 -->
							<div
								v-if="customParams.length === 0"
								class="cursor-pointer rounded-xl border-2 border-dashed border-gray-200 py-6 text-center transition-colors duration-200 hover:border-primary-300 hover:bg-primary-50/30"
								@click="currentPreset ? applyPreset() : applyDefaults()"
							>
								<Icon name="plus" size="sm" class="mx-auto mb-2 text-gray-300" />
								<p class="text-xs text-gray-400">
									{{ currentPreset ? '点击应用 ' + currentPresetLabel + ' 预设' : '点击添加自定义参数' }}
								</p>
							</div>

							<!-- 参数行 -->
							<div v-else class="space-y-2">
								<div
									v-for="param in customParams"
									:key="param.id"
									class="group flex items-center gap-1.5"
								>
									<n-input
										v-model:value="param.key"
										class="flex-1"
										placeholder="参数名"
									/>
									<n-input
										v-model:value="param.value"
										class="flex-1"
										:placeholder="paramPlaceholder(param.key)"
									/>
									<button
										class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg text-gray-300 transition-colors duration-150 hover:bg-red-50 hover:text-red-500"
										title="删除该参数"
										@click="removeParam(param.id)"
									>
										<Icon name="x" size="sm" />
									</button>
								</div>
								<button
									class="w-full rounded-lg border border-dashed border-gray-200 py-1.5 text-xs text-gray-400 transition-colors duration-150 hover:border-primary-300 hover:text-primary-600"
									@click="addParam()"
								>
									+ 添加参数
								</button>
							</div>
						</div>
					</div>

					<!-- 主操作常驻卡片底部 -->
					<div class="shrink-0 border-t border-gray-100 px-4 py-3">
						<button class="btn btn-primary w-full" :disabled="sending || polling || !prompt.trim()" @click="generate">
							<Icon name="photo" size="sm" />
							{{ sending ? (effectiveMode === 'async' ? '提交中...' : '生成中...') : polling ? '生成中...' : '生成图片' }}
						</button>
					</div>
				</div>
			</div>

			<!-- Right: 结果 -->
			<div class="lg:min-h-0">
				<div class="card flex min-h-[420px] flex-col overflow-hidden lg:h-full">
					<div class="flex shrink-0 items-center justify-between gap-3 border-b border-gray-100 px-5 py-3">
						<div class="flex items-center gap-2">
							<Icon name="photo" size="sm" class="text-primary-500" />
							<h3 class="text-sm font-semibold text-gray-900">生成结果</h3>
						</div>
						<span v-if="resultStatus" class="badge" :class="resultStatus.cls">{{ resultStatus.label }}</span>
					</div>

					<div class="min-h-0 flex-1 overflow-y-auto p-5">
						<!-- 异步端点说明：轻量单行，不再用整块彩底卡片占高度 -->
						<div v-if="effectiveMode === 'async'" class="mb-4 flex items-start gap-2 rounded-lg bg-gray-50 px-3 py-2">
							<Icon name="infoCircle" size="xs" class="mt-0.5 shrink-0 text-gray-400" />
							<p class="text-[11px] leading-relaxed text-gray-500">
								异步任务模型：提交至 <code class="code text-[10px]">POST /v1/images/generations/async</code>
								拿到 task_id 后轮询 <code class="code text-[10px]">GET /v1/images/generations/async/{task_id}</code> 取图
							</p>
						</div>

						<!-- 优雅降级提示 -->
						<div v-if="fallbackNotice" class="mb-4 rounded-xl border border-amber-200 bg-amber-50 p-3">
							<div class="flex items-center gap-2 text-amber-700">
								<Icon name="infoCircle" size="sm" class="flex-shrink-0" />
								<span class="text-xs">{{ fallbackNotice }}</span>
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

						<!-- 空态：图标磁贴 + 说明，给首屏一个视觉锚点 -->
						<div v-if="images.length === 0 && !sending && !errorMessage && !asyncTask" class="flex h-full min-h-[320px] flex-col items-center justify-center px-4 text-center">
							<div class="mb-5 flex h-16 w-16 items-center justify-center rounded-3xl bg-gradient-to-br from-cyan-400 via-primary-500 to-emerald-500 text-white shadow-glow">
								<Icon name="photo" size="lg" />
							</div>
							<h3 class="text-lg font-semibold text-gray-900">等待生成</h3>
							<p class="mt-1.5 max-w-sm text-sm text-gray-500">
								<template v-if="selectedModel">
									当前模型 <span class="font-medium text-gray-700">{{ selectedModelName }}</span>，填写提示词后点击「生成图片」
								</template>
								<template v-else>请先在左侧选择一个图片模型</template>
							</p>
							<p class="mt-6 inline-flex items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-[11px] text-amber-700">
								<Icon name="exclamationTriangle" size="xs" />
								生成将真实调用模型并产生费用
							</p>
						</div>

						<!-- 提交中 / 同步生成中：卡片式加载占位 -->
						<GenerationProgressCard
							v-if="sending"
							kind="image"
							:label="effectiveMode === 'async' ? '正在提交生成任务...' : '图片生成中...'"
						/>

						<!-- 异步任务进行中：卡片式进度占位（成功由下方图片区接管） -->
						<GenerationProgressCard
							v-else-if="asyncTask && asyncTask.status !== 'SUCCESS' && asyncTask.status !== 'FAILURE'"
							kind="image"
							:progress="asyncTask.progress"
							:label="statusLabel[asyncTask.status] || asyncTask.status"
							:sublabel="asyncTask.id ? 'task_id: ' + asyncTask.id : ''"
						/>

						<!-- 任务失败 -->
						<div v-if="asyncTask && asyncTask.status === 'FAILURE'" class="rounded-xl border border-red-200 bg-red-50 p-4">
							<div class="flex items-center gap-2 text-red-700">
								<Icon name="xCircle" size="sm" />
								<span class="text-sm font-medium">生成失败</span>
							</div>
							<p v-if="asyncTask.error" class="mt-2 text-sm text-red-600">{{ asyncTask.error }}</p>
						</div>

						<div v-if="images.length > 0" class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
							<div v-for="(img, idx) in images" :key="idx" class="group relative overflow-hidden rounded-xl border border-gray-200 bg-gray-50">
								<img :src="imageSrc(img)" class="w-full cursor-zoom-in" alt="Generated image" @click="openZoom(img)" />
								<div class="absolute right-2 top-2 flex items-center gap-2 opacity-0 transition-opacity duration-200 group-hover:opacity-100">
									<button
										class="btn btn-sm border border-gray-200 bg-white/90 shadow-sm backdrop-blur-sm"
										title="放大查看"
										@click="openZoom(img)"
									>
										<Icon name="expand" size="sm" />
										放大
									</button>
									<button
										class="btn btn-sm border border-gray-200 bg-white/90 shadow-sm backdrop-blur-sm"
										title="下载图片"
										@click="downloadImage(img, idx)"
									>
										<Icon name="arrowDown" size="sm" />
										下载
									</button>
								</div>
								<p v-if="img.revised_prompt" class="border-t border-gray-100 bg-white p-3 text-xs text-gray-500">{{ img.revised_prompt }}</p>
							</div>
						</div>

						<!-- 用量条：与对话 Tab 保持同一排版 -->
						<div v-if="tokenUsage.totalTokens > 0" class="mt-4 flex items-center justify-between gap-3 border-t border-gray-100 pt-3 text-xs text-gray-500">
							<div class="flex items-center gap-4">
								<span class="flex items-center gap-1.5">Prompt <span class="font-medium text-gray-700">{{ tokenUsage.promptTokens }}</span></span>
								<span class="flex items-center gap-1.5">Total <span class="font-medium text-gray-700">{{ tokenUsage.totalTokens }}</span></span>
							</div>
							<span v-if="tokenUsage.cost" class="font-medium text-amber-600">{{ tokenUsage.cost }}</span>
						</div>
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
