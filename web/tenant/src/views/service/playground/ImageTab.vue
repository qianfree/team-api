<script setup lang="ts">
import { ref, computed, watch, onUnmounted, nextTick } from 'vue'
import { NInput, NInputNumber } from 'naive-ui'
import { createPlaygroundApi } from '@/utils/playgroundApi'
import ParamRefModal from './ParamRefModal.vue'
import GenerationProgressCard from './GenerationProgressCard.vue'
import AttachmentBar from './AttachmentBar.vue'
import PromptInput from './PromptInput.vue'
import Icon from '@/components/common/Icon.vue'
import BaseSelect from '../../../components/common/BaseSelect.vue'
import { useAttachments, formatBytes } from './useAttachments'
import { compileImageReferences } from './attachmentParts'

interface ModelItem {
	model_id: string
	model_name: string
	category: string
	billing_mode?: string | null
	per_request_price?: number | null
	input_price?: number | null
	output_price?: number | null
	// image_sync_supported 为 false 表示「仅异步」厂商（阿里 image-synthesis 等），无法经
	// OpenAI 协议同步端点 /v1/images/generations 调用，在线体验过滤不展示（仍可通过异步端点
	// API 调用）。缺省（旧后端）视为可用。
	image_sync_supported?: boolean
}
const props = defineProps<{ models: ModelItem[]; apiKey: string }>()

const sending = ref(false)
// 在线体验统一走 OpenAI 协议同步端点 /v1/images/generations：「仅异步」厂商
// （image_sync_supported=false）无法经该端点调用，过滤不展示。缺省（旧后端）视为可用。
const syncModels = computed(() => props.models.filter(m => m.image_sync_supported !== false))
const selectedModel = ref(syncModels.value[0]?.model_id || '')
// 列表由父组件异步加载，setup 阶段可能为空；变化时若当前选中模型不在列表内
//（如切换 API Key 后模型范围变化）则自动重置为第一个
watch(
	syncModels,
	models => {
		if (!selectedModel.value || !models.some(m => m.model_id === selectedModel.value)) {
			selectedModel.value = models[0]?.model_id || ''
		}
	},
)
const selectedModelItem = computed(() =>
	syncModels.value.find(m => m.model_id === selectedModel.value),
)
const prompt = ref('')
// ── 标准 OpenAI 字段：留空（''）/ null = 不发送，请求体按条件写入 ──
const size = ref('')
const quality = ref('')
const outputFormat = ref('')
// complexity：gpt-image-2.6 新增的规划深度（low/medium/high，API 侧默认 high），所有格式可用
const complexity = ref('')
// GIF 专属参数：仅在 output_format=gif 时发送（切走格式后残留值不应进入请求体）
const gifFrameCount = ref<number | null>(null)
const gifFrameDurationMs = ref<number | null>(null)

// 尺寸预设：覆盖 gpt-image 系列与 dall-e 系列的常用档位，非预设值走自定义参数透传
const sizeOptions = [
	{ value: '', label: '不设置（不发送）' },
	{ value: 'auto', label: 'auto（模型自动）' },
	{ value: '1024x1024', label: '1024x1024 方形' },
	{ value: '1536x1024', label: '1536x1024 横版' },
	{ value: '1024x1536', label: '1024x1536 竖版' },
	{ value: '1792x1024', label: '1792x1024 宽横（dall-e-3）' },
	{ value: '1024x1792', label: '1024x1792 长竖（dall-e-3）' },
	{ value: '512x512', label: '512x512 小图（dall-e-2）' },
]

// 质量预设：gpt-image 系列用 low/medium/high/auto，dall-e-3 用 standard/hd
const qualityOptions = [
	{ value: '', label: '不设置（不发送）' },
	{ value: 'auto', label: 'auto（模型自动）' },
	{ value: 'low', label: 'low 低' },
	{ value: 'medium', label: 'medium 中' },
	{ value: 'high', label: 'high 高' },
	{ value: 'standard', label: 'standard 标准（dall-e-3）' },
	{ value: 'hd', label: 'hd 高清（dall-e-3）' },
]

// 输出格式：svg 矢量 / gif 动图为 gpt-image-2.6 起新增能力，png/jpeg/webp 为通用档
const outputFormatOptions = [
	{ value: '', label: '不设置（不发送）' },
	{ value: 'png', label: 'png（默认）' },
	{ value: 'jpeg', label: 'jpeg' },
	{ value: 'webp', label: 'webp' },
	{ value: 'svg', label: 'svg 矢量（gpt-image-2.6+）' },
	{ value: 'gif', label: 'gif 动图（gpt-image-2.6+）' },
]

// 规划深度：gpt-image-2.6 新增，控制渲染前的规划量；留空 = 不发送（上游默认 high）
const complexityOptions = [
	{ value: '', label: '不设置（不发送，默认 high）' },
	{ value: 'low', label: 'low 快（局部修改）' },
	{ value: 'medium', label: 'medium 中' },
	{ value: 'high', label: 'high 精细（复杂排版）' },
]

const modelOptions = computed(() => syncModels.value.map(m => ({ value: m.model_id, label: m.model_name || m.model_id })))

// 空态里展示的模型名（列表由父组件异步加载，可能暂时查不到）
const selectedModelName = computed(() => selectedModelItem.value?.model_name || selectedModel.value)

// ── 参考图 ──
// 图生图/图像编辑的参考素材：以 base64 data URL 内联进 JSON 请求体的 image 字段
// （单张为字符串、多张为数组，与网关 ImageRequest 及主流上游——Seedream、gpt-image edits——
// 的取值形态一致，OpenAI 兼容渠道原样透传）。只收图片：协议里没有承载其他素材的位置。
// 数量上限 4 张——低于 Seedream 的 10 张上限，但附件总量另有 6MB 闸门（见 useAttachments），
// 参考图过大时先压缩，压不动即拒绝。
const {
	list: attachmentList,
	totalBytes: attachmentBytes,
	accept: acceptTypes,
	max: attachmentMax,
	addFiles: addAttachments,
	remove: removeAttachment,
	clear: clearAttachments,
} = useAttachments({ kinds: ['image'], max: 4 })

// ── 输入区（右侧，对话式）：拖放高亮 / 文件选择器触发 ──
const promptInput = ref<InstanceType<typeof PromptInput> | null>(null)
const attachmentBar = ref<InstanceType<typeof AttachmentBar> | null>(null)
const dragging = ref(false)

function openFilePicker() {
	attachmentBar.value?.pick()
}

function clearAllAttachments() {
	clearAttachments()
}

// 输入容器承接拖放：与粘贴共用同一条上传链路
function onDropFiles(e: DragEvent) {
	dragging.value = false
	if (e.dataTransfer?.files?.length) addAttachments(Array.from(e.dataTransfer.files))
}

// dragleave 会在子元素之间移动时也冒泡触发，直接置 false 会让拖放提示闪烁。
// 只有真正离开容器（relatedTarget 不在容器内）才收起提示。
function onDragLeave(e: DragEvent) {
	const el = e.currentTarget as HTMLElement | null
	const to = e.relatedTarget as Node | null
	if (el && to && el.contains(to)) return
	dragging.value = false
}

// 空态快捷提示词：点一下直接填进输入框
const quickPrompts = [
	'赛博朋克风格的城市夜景，霓虹灯与雨夜反射',
	'水彩风格的猫咪插画，柔和光线',
	'极简主义的产品海报，纯色背景',
]

// ── 自定义参数系统 ──

interface CustomParam {
	id: number
	key: string
	value: string
}

const customParams = ref<CustomParam[]>([])
let nextParamId = 1
const showParamRef = ref(false)

// 模型预设模板：standard 三字段写入标准下拉，custom 仍走自定义参数行。
// gpt-image 系列不支持 response_format（总返回 b64_json），预设里不带。
interface ModelPreset {
	size?: string
	quality?: string
	outputFormat?: string
	custom: Record<string, string>
}

const MODEL_PRESETS: Record<string, ModelPreset> = {
	// gpt-image-2.6：svg 矢量 / gif 动图为该代起新增能力，预设直接选中 svg 便于快速体验。
	// 键必须排在 gpt-image-2 之前：预设按子串匹配，更长的键先命中，
	// 避免 gpt-image-2.6 的变体名（如 gpt-image-2.6-mini）误中 gpt-image-2 的预设
	'gpt-image-2.6': {
		size: '1024x1024',
		quality: 'auto',
		outputFormat: 'svg',
		custom: {},
	},
	// gpt-image-2：预设常用档位，零配置即可快速体验
	'gpt-image-2': {
		size: '1024x1024',
		quality: 'auto',
		outputFormat: 'png',
		custom: {},
	},
	'gpt-image-1': {
		size: '1024x1024',
		quality: 'auto',
		outputFormat: 'png',
		custom: {},
	},
	'dall-e-3': {
		size: '1024x1024',
		quality: 'standard',
		custom: { style: 'vivid', response_format: 'url' },
	},
	'dall-e-2': {
		size: '1024x1024',
		custom: { response_format: 'url' },
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

// 有效参数数量
const validParamsCount = computed(() =>
	customParams.value.filter(p => p.key.trim() && p.value.trim()).length,
)

// 参数摘要：左侧参数面板可独立滚动，尺寸/格式很容易滚出视野。沉浸式输入区不再有
// 独立信息行，摘要改挂到生成按钮的 title 上，零视觉占用但仍能在悬停时核对。
const paramSummary = computed(() => {
	const parts: string[] = []
	if (size.value) parts.push(size.value)
	if (quality.value) parts.push(quality.value)
	if (outputFormat.value) parts.push(outputFormat.value)
	if (complexity.value) parts.push(complexity.value)
	// GIF 专属参数只在 gif 输出时真的进请求体，摘要同口径
	if (outputFormat.value === 'gif') {
		if (gifFrameCount.value != null) parts.push(`${gifFrameCount.value} 帧`)
		if (gifFrameDurationMs.value != null) parts.push(`帧 ${gifFrameDurationMs.value}ms`)
	}
	if (validParamsCount.value > 0) parts.push(`自定义 ${validParamsCount.value}`)
	return parts.join(' · ')
})

// 可提交：有提示词且没有进行中的请求（生成中发现按钮同时承担「进行中」指示）
const canGenerate = computed(() => !!prompt.value.trim() && !sending.value)

const sendTitle = computed(() => {
	if (sending.value) return '正在生成，请稍候'
	if (!prompt.value.trim()) return '先描述画面内容，再按 Enter 或点击生成'
	const base = '生成图片（Enter）· Shift + Enter 换行'
	return paramSummary.value ? `${base} · 参数 ${paramSummary.value}` : base
})

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
	size.value = preset.size || ''
	quality.value = preset.quality || ''
	outputFormat.value = preset.outputFormat || ''
	customParams.value = []
	for (const [key, value] of Object.entries(preset.custom)) {
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

// 无预设模型的兜底：标准字段留空（不发送），自定义参数区留给用户自行添加
function applyDefaults() {
	size.value = ''
	quality.value = ''
	outputFormat.value = ''
	customParams.value = []
}

// 参数值输入框的引导占位：无预设时默认给的是空值，直接显示「值」用户不知道填什么，
// 按参数名给出常见取值示例
const PARAM_HINTS: Record<string, string> = {
	style: '如 vivid / natural',
	response_format: '如 url / b64_json',
	background: '如 transparent / opaque',
	n: '生成张数，如 1',
	frame_count: 'GIF 帧数 2-16',
	frame_duration_ms: 'GIF 每帧毫秒 20-2000',
	complexity: '如 low / medium / high',
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

// ── 图片生成（对话式记录）──

interface ImageResult { b64_json?: string; url?: string; revised_prompt?: string }

// 生成记录：user = 提示词与参考图，assistant = 生成过程与结果，与对话 Tab 同构
interface GalleryMsg {
	id: number
	role: 'user' | 'assistant'
	/** user：提示词文本 */
	text?: string
	/** user：随请求发出的参考图（data URL，供气泡缩略与重新生成复用） */
	refPreviews?: { url: string; name: string }[]
	/** assistant：生成状态机（OpenAI 协议同步端点，阻塞一次性返回） */
	status?: 'generating' | 'success' | 'failed'
	/** 结果 */
	images?: ImageResult[]
	error?: string
	/** 返回的用量 */
	promptTokens?: number
	totalTokens?: number
	/** 本条记录起点（ms），进度卡计时用 */
	startAt?: number
}

const messages = ref<GalleryMsg[]>([])
let nextMsgId = 1

// 头部状态徽标：取最近一条生成记录的状态（无记录时不渲染）
const resultStatus = computed<{ label: string; cls: string } | null>(() => {
	const last = messages.value[messages.value.length - 1]
	if (!last || last.role !== 'assistant' || !last.status) return null
	switch (last.status) {
		case 'success': return { label: '已完成', cls: 'badge-success' }
		case 'failed': return { label: '失败', cls: 'badge-danger' }
		case 'generating': return { label: '生成中', cls: 'badge-primary' }
	}
	return null
})

onUnmounted(() => {
	window.removeEventListener('keydown', onZoomKeydown)
})

// 组装请求体：提示词 + 参考图（data URL）+ 标准字段 + 自定义参数。
// 参考图走系统 OpenAI 协议约定字段：单张 image（string），多张 images（string[] 数组）——
// 网关各图片适配器按此归一，透传渠道（如豆包 Ark）由适配器转换
function buildBody(promptText: string, refImages: string[]): Record<string, any> {
	const body: Record<string, any> = {
		model: selectedModel.value,
		prompt: promptText,
	}
	if (refImages.length === 1) {
		body.image = refImages[0]
	} else if (refImages.length > 1) {
		body.images = refImages
	}
	// 标准字段：留空不发送；自定义参数后写、可覆盖标准字段（透传语义）
	if (size.value) body.size = size.value
	if (quality.value) body.quality = quality.value
	if (outputFormat.value) body.output_format = outputFormat.value
	if (complexity.value) body.complexity = complexity.value
	// GIF 专属参数：仅在 gif 输出时携带，其它格式下的残留值不进请求体
	if (outputFormat.value === 'gif') {
		if (gifFrameCount.value != null) body.frame_count = gifFrameCount.value
		if (gifFrameDurationMs.value != null) body.frame_duration_ms = gifFrameDurationMs.value
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

// scrollToBottom / isNearBottom：与对话 Tab 同一套滚动策略（40px 容差）
const messagesRef = ref<HTMLElement | null>(null)

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

// runGenerate 一次生成的执行段（首次发送与重新生成共用）：结果写回 msg
async function runGenerate(promptText: string, refImages: string[], msg: GalleryMsg) {
	sending.value = true
	msg.startAt = Date.now()
	scrollToBottom(true)

	try {
		const api = createPlaygroundApi(props.apiKey)
		const body = buildBody(promptText, refImages)
		await generateSync(api, body, msg)
	} catch (e) {
		console.error(e)
		msg.status = 'failed'
		msg.error = (e as any)?.message || '请求失败，请重试'
	} finally {
		sending.value = false
		scrollToBottom()
	}
}

// generate 从输入区发起一次生成：提示词与参考图入列成 user 消息，结果以 assistant 消息呈现
async function generate() {
	const raw = prompt.value.trim()
	if (!raw || !selectedModel.value || sending.value) return

	const refs = compileImageReferences(attachmentList.value)
	const refPreviews = attachmentList.value.map(a => ({ url: a.dataUrl, name: a.name }))

	messages.value.push({ id: nextMsgId++, role: 'user', text: raw, refPreviews })
	prompt.value = ''
	clearAttachments()

	const msg: GalleryMsg = {
		id: nextMsgId++,
		role: 'assistant',
		status: 'generating',
		images: [],
	}
	messages.value.push(msg)
	await runGenerate(raw, refs.images, msg)
}

// regenerate 从一条历史记录重发：复用其提示词与参考图，追加新的生成记录
async function regenerate(msg: GalleryMsg) {
	if (sending.value) return
	const idx = messages.value.indexOf(msg)
	const userMsg = idx > 0 ? messages.value[idx - 1] : null
	if (!userMsg || userMsg.role !== 'user' || !userMsg.text) return

	const refs = (userMsg.refPreviews || []).map(p => p.url)
	messages.value.push({
		id: nextMsgId++,
		role: 'user',
		text: userMsg.text,
		refPreviews: userMsg.refPreviews,
	})
	const newMsg: GalleryMsg = {
		id: nextMsgId++,
		role: 'assistant',
		status: 'generating',
		images: [],
	}
	messages.value.push(newMsg)
	await runGenerate(userMsg.text, refs, newMsg)
}

// generateSync 走 OpenAI 协议同步端点 /v1/images/generations：阻塞一次性返回图片与用量。
// 超时 600s 与后端图片模式强制下限（relay ImagesGenerationTimeoutSecs）对齐：
// b64_json 响应体达 MB 级，慢网络下载耗时会远超生成耗时，前端超时须不早于后端放弃。
async function generateSync(api: ReturnType<typeof createPlaygroundApi>, body: Record<string, any>, msg: GalleryMsg) {
	const res = await api.post('/v1/images/generations', body, { timeout: 600_000 })
	const data = res.data
	msg.status = 'success'
	msg.images = data.data || []
	const usage = data.usage || {}
	msg.promptTokens = usage.prompt_tokens || 0
	msg.totalTokens = usage.total_tokens || 0
}

// b64_json 不携带 MIME 信息，output_format 参数也未必与实际编码一致，直接嗅探 base64 头部魔法数
function sniffImageMime(b64: string): string {
	if (b64.startsWith('iVBORw0KGgo')) return 'image/png'
	if (b64.startsWith('/9j/')) return 'image/jpeg'
	if (b64.startsWith('UklGR')) return 'image/webp'
	if (b64.startsWith('R0lGOD')) return 'image/gif'
	// svg：'<svg' 或 '<?xml' 开头（flare 系列矢量输出）
	if (b64.startsWith('PHN2Zy') || b64.startsWith('PD94bWw')) return 'image/svg+xml'
	return 'image/png'
}

const MIME_EXT: Record<string, string> = {
	'image/png': 'png',
	'image/jpeg': 'jpg',
	'image/webp': 'webp',
	'image/gif': 'gif',
	'image/svg+xml': 'svg',
}

function imageSrc(img: ImageResult): string {
	if (img.b64_json) return 'data:' + sniffImageMime(img.b64_json) + ';base64,' + img.b64_json
	if (img.url) return img.url
	return ''
}

function downloadImage(img: ImageResult, idx: number) {
	const src = imageSrc(img)
	if (!src) return
	const a = document.createElement('a')
	a.href = src
	// 扩展名按实际格式推断：b64 走魔法数嗅探，URL 从路径后缀取，都取不到再兜底 png
	let ext = 'png'
	if (img.b64_json) {
		ext = MIME_EXT[sniffImageMime(img.b64_json)] || 'png'
	} else {
		const m = src.match(/\.(png|jpe?g|webp)(?:\?|#|$)/i)
		if (m) ext = m[1].toLowerCase() === 'jpeg' ? 'jpg' : m[1].toLowerCase()
	}
	// 文件名带时间戳，避免不同批次的生成结果重名（序号保留，防止同秒内多张图冲突）
	const now = new Date()
	const pad = (n: number) => String(n).padStart(2, '0')
	const stamp = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}_${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`
	a.download = `image_${stamp}_${idx + 1}.${ext}`
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
	zoomIn(src)
}

// 参考图缩略片点击 → 放大预览（AttachmentBar 的 insert 事件按名字回调）
function previewAttachment(name: string) {
	const att = attachmentList.value.find(a => a.name === name)
	if (att) zoomIn(att.dataUrl)
}

function zoomIn(src: string) {
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
									<span class="text-[11px] text-gray-400">共 {{ syncModels.length }} 个可用</span>
							</div>
							<BaseSelect v-model="selectedModel" :options="modelOptions" />
						</div>

						<!-- 标准 OpenAI 字段：尺寸 / 质量 / 输出格式，留空不发送 -->
						<div class="border-t border-gray-100 pt-4">
							<div class="mb-2 flex items-center justify-between">
								<label class="text-xs font-semibold text-gray-500">标准参数</label>
								<span class="text-[11px] text-gray-400">留空不发送</span>
							</div>
							<div class="space-y-3">
								<div>
									<label class="mb-1.5 block text-xs font-medium text-gray-500">尺寸 size</label>
									<BaseSelect v-model="size" :options="sizeOptions" />
								</div>
								<div>
									<label class="mb-1.5 block text-xs font-medium text-gray-500">质量 quality</label>
									<BaseSelect v-model="quality" :options="qualityOptions" />
								</div>
								<div>
									<label class="mb-1.5 block text-xs font-medium text-gray-500">输出格式 output_format</label>
									<BaseSelect v-model="outputFormat" :options="outputFormatOptions" />
									<p class="mt-1 text-[11px] leading-relaxed text-gray-400">svg 矢量 / gif 动图为 gpt-image-2.6 起新增能力</p>
								</div>
								<div>
									<label class="mb-1.5 block text-xs font-medium text-gray-500">规划深度 complexity</label>
									<BaseSelect v-model="complexity" :options="complexityOptions" />
									<p class="mt-1 text-[11px] leading-relaxed text-gray-400">gpt-image-2.6 新增：渲染前规划量，low 更快，high 适合复杂排版</p>
								</div>
							</div>

							<!-- 格式联动配置：选 gif / svg 时展开对应配置项 -->
							<div v-if="outputFormat === 'gif'" class="mt-3 rounded-xl border border-primary-100 bg-primary-50/40 p-3">
								<div class="mb-2 flex items-center gap-1.5">
									<Icon name="film" size="xs" class="text-primary-600" />
									<span class="text-xs font-semibold text-gray-600">GIF 动图配置</span>
									<span class="text-[11px] text-gray-400">仅 gif 输出生效</span>
								</div>
								<div class="space-y-2">
									<div>
										<label class="mb-1 block text-[11px] text-gray-500">帧数 frame_count</label>
										<n-input-number v-model:value="gifFrameCount" :min="2" :max="16" :step="1" placeholder="默认 9" size="small" clearable class="w-full" />
									</div>
									<div>
										<label class="mb-1 block text-[11px] text-gray-500">帧时长(ms) frame_duration_ms</label>
										<n-input-number v-model:value="gifFrameDurationMs" :min="20" :max="2000" :step="10" placeholder="默认 200" size="small" clearable class="w-full" />
									</div>
								</div>
								<p class="mt-2 text-[11px] leading-relaxed text-gray-400">
									2–16 帧，每帧 20–2000ms（10ms 步进）；循环总时长 = 帧数 × 帧时长。提示词写明「末帧平滑回到首帧」可生成无缝循环
								</p>
							</div>
							<div v-else-if="outputFormat === 'svg'" class="mt-3 rounded-xl border border-primary-100 bg-primary-50/40 p-3">
								<div class="mb-1.5 flex items-center gap-1.5">
									<Icon name="infoCircle" size="xs" class="text-primary-600" />
									<span class="text-xs font-semibold text-gray-600">SVG 矢量输出</span>
								</div>
								<p class="text-[11px] leading-relaxed text-gray-500">
									输出可二次编辑的矢量图形（路径 / 形状 / 颜色），适合图标、logo 与印刷；仅支持单图（n=1），不支持流式返回
								</p>
							</div>
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
										v-if="currentPreset"
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

						<!-- 空状态：点击直接添加一行空参数（预设由切模型/重置预设按钮负责，不再在这里触发） -->
							<div
								v-if="customParams.length === 0"
								class="cursor-pointer rounded-xl border-2 border-dashed border-gray-200 py-6 text-center transition-colors duration-200 hover:border-primary-300 hover:bg-primary-50/30"
								@click="addParam()"
							>
								<Icon name="plus" size="sm" class="mx-auto mb-2 text-gray-300" />
								<p class="text-xs text-gray-400">
									点击添加自定义参数，透传给上游
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
										aria-label="删除该参数"
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
				</div>
			</div>

			<!-- Right: 生成记录 + 输入区（对话式：提示词在下方输入，发送后入列并展示生成结果） -->
			<div class="h-[62vh] lg:h-auto lg:min-h-0">
				<div class="card flex h-full flex-col overflow-hidden">
					<div class="flex shrink-0 items-center justify-between gap-3 border-b border-gray-100 px-5 py-3">
						<div class="flex items-center gap-2">
							<Icon name="photo" size="sm" class="text-primary-500" />
							<h3 class="text-sm font-semibold text-gray-900">生成记录</h3>
						</div>
						<span v-if="resultStatus" class="badge" :class="resultStatus.cls">{{ resultStatus.label }}</span>
					</div>

					<!-- 消息区 -->
					<div ref="messagesRef" class="min-h-0 flex-1 overflow-y-auto px-5 py-5 lg:px-6">
						<!-- 空态：图标磁贴 + 说明，给首屏一个视觉锚点 -->
						<div v-if="messages.length === 0" class="flex min-h-full flex-col items-center justify-center px-4 text-center">
							<div class="mb-5 flex h-16 w-16 items-center justify-center rounded-3xl bg-gradient-to-br from-cyan-400 via-primary-500 to-emerald-500 text-white shadow-glow">
								<Icon name="photo" size="lg" />
							</div>
							<h3 class="text-lg font-semibold text-gray-900">开始创作</h3>
							<p class="mt-1.5 max-w-sm text-sm text-gray-500">
								<template v-if="selectedModel">
									当前模型 <span class="font-medium text-gray-700">{{ selectedModelName }}</span>，在下方描述想生成的图片
								</template>
								<template v-else>请先在左侧选择一个图片模型</template>
							</p>

							<!-- 快捷提示词：点一下即填入输入框 -->
							<div v-if="selectedModel" class="mt-6 flex max-w-lg flex-wrap justify-center gap-2">
								<button
									v-for="p in quickPrompts"
									:key="p"
									class="rounded-full border border-gray-200 bg-white px-3 py-1.5 text-xs text-gray-600 transition-all duration-150 hover:-translate-y-0.5 hover:border-primary-300 hover:text-primary-600 hover:shadow-sm"
									@click="prompt = p; promptInput?.focus()"
								>
									{{ p }}
								</button>
							</div>

							<p class="mt-7 inline-flex items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-[11px] text-amber-700">
								<Icon name="exclamationTriangle" size="xs" />
								生成将真实调用模型并产生费用
							</p>
						</div>

						<!-- 消息列表 -->
						<div v-else class="space-y-5">
							<template v-for="msg in messages" :key="msg.id">
								<!-- 提示词（右对齐，附参考图缩略） -->
								<div v-if="msg.role === 'user'" class="flex justify-end">
									<div class="max-w-[80%] space-y-1.5">
										<div v-if="msg.refPreviews?.length" class="flex flex-wrap justify-end gap-1.5">
											<img
												v-for="(p, i) in msg.refPreviews"
												:key="i"
												:src="p.url"
												class="h-20 w-20 cursor-zoom-in overflow-hidden rounded-xl border border-primary-200 object-cover"
												:title="p.name"
												alt=""
												@click="zoomIn(p.url)"
											/>
										</div>
										<div class="rounded-2xl rounded-tr-md bg-gradient-to-br from-primary-500 to-primary-600 px-4 py-3 text-white shadow-sm shadow-primary-500/20">
											<p class="text-sm whitespace-pre-wrap">{{ msg.text }}</p>
										</div>
									</div>
								</div>

								<!-- 生成结果（左对齐） -->
								<div v-else class="flex max-w-[85%] gap-3">
									<div class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-primary-400 to-primary-600 text-white shadow-sm shadow-primary-500/25">
										<Icon name="photo" size="sm" />
									</div>
									<div class="min-w-0 flex-1 space-y-2">
										<!-- 进行中 -->
										<GenerationProgressCard
											v-if="msg.status === 'generating'"
											kind="image"
											label="图片生成中..."
											:since="msg.startAt"
										/>

										<!-- 失败：错误卡 + 重新生成 -->
										<div v-else-if="msg.status === 'failed'" class="rounded-xl border border-red-200 bg-red-50 p-4">
											<div class="flex items-center gap-2 text-red-700">
												<Icon name="xCircle" size="sm" />
												<span class="text-sm font-medium">生成失败</span>
											</div>
											<p v-if="msg.error" class="mt-2 break-all text-sm text-red-600">{{ msg.error }}</p>
											<button
												class="mt-3 inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-2.5 py-1 text-xs font-medium text-red-600 transition-colors duration-150 hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-50"
												:disabled="sending"
												@click="regenerate(msg)"
											>
												<Icon name="refresh" size="xs" />
												重新生成
											</button>
										</div>

										<!-- 成功：图片网格 -->
										<template v-else-if="msg.status === 'success'">
											<div v-if="msg.images?.length" class="grid grid-cols-1 gap-3 sm:grid-cols-2">
												<div v-for="(img, idx) in msg.images" :key="idx" class="group relative overflow-hidden rounded-xl border border-gray-200 bg-gray-50">
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
											<p v-else class="text-sm text-gray-400">(无图片返回)</p>
											<!-- 用量 + 重新生成 -->
											<div class="flex items-center justify-between gap-3">
												<div v-if="msg.totalTokens" class="flex items-center gap-4 text-xs text-gray-500">
													<span class="flex items-center gap-1.5">Prompt <span class="font-medium text-gray-700">{{ msg.promptTokens }}</span></span>
													<span class="flex items-center gap-1.5">Total <span class="font-medium text-gray-700">{{ msg.totalTokens }}</span></span>
												</div>
												<button class="msg-action" :disabled="sending" @click="regenerate(msg)">
													<Icon name="refresh" size="xs" />
													重新生成
												</button>
											</div>
										</template>
									</div>
								</div>
							</template>
						</div>
					</div>

					<!-- 输入区：单块 composer。参考图、提示词、工具与发送都在同一个圆角画布内，
					     工具条绝对定位贴在画布底部，输入区不再被拆成「输入框 + 按钮条」两层 -->
					<div class="shrink-0 p-3 lg:p-4">
						<div
							class="composer relative rounded-[1.35rem] border bg-white transition-all duration-200"
							:class="dragging
								? 'border-dashed border-primary-300 bg-primary-50/60'
								: 'border-gray-200 shadow-sm focus-within:border-primary-400 focus-within:ring-4 focus-within:ring-primary-500/10'"
							@dragover.prevent="dragging = true"
							@dragleave="onDragLeave"
							@drop.prevent="onDropFiles"
						>
							<!-- 参考图缩略：嵌在画布顶部 -->
							<div v-if="attachmentList.length" class="px-3 pt-2.5">
								<AttachmentBar
									ref="attachmentBar"
									variant="bare"
									:attachments="attachmentList"
									:accept="acceptTypes"
									:max="attachmentMax"
									click-hint="点击放大预览 {name}"
									@add="addAttachments"
									@remove="removeAttachment"
									@insert="previewAttachment"
								/>
							</div>

							<!-- 提示词：底部预留工具条高度，长文本不会压在按钮下面 -->
							<div class="px-3.5 pb-12" :class="attachmentList.length ? 'pt-2' : 'pt-3'">
								<PromptInput
									ref="promptInput"
									v-model="prompt"
									bare
									:mentions="false"
									:max-rows="8"
									:attachments="attachmentList"
									placeholder="描述画面主体、风格与构图，可拖拽或粘贴参考图做图生图 / 编辑…"
									@submit="generate"
									@paste="addAttachments"
								/>
							</div>

							<!-- 内嵌工具条：贴着画布底部悬浮，与输入区共用一块背景 -->
							<div class="absolute inset-x-1.5 bottom-1.5 flex items-center justify-between gap-2">
								<div class="flex min-w-0 items-center gap-0.5">
									<button
										class="input-tool"
										title="添加参考图（图生图 / 图像编辑，也可拖拽到此处或直接粘贴）"
										@mousedown.prevent
										@click="openFilePicker"
									>
										<Icon name="paperclip" size="xs" />
										参考图
									</button>
									<button
										v-if="attachmentList.length"
										class="input-tool"
										title="清空已选参考图"
										@mousedown.prevent
										@click="clearAllAttachments"
									>
										<Icon name="x" size="xs" />
										清空
									</button>
									<span v-if="attachmentList.length" class="ml-0.5 hidden truncate text-[11px] text-gray-400 sm:inline">
										{{ attachmentList.length }}/{{ attachmentMax }} · {{ formatBytes(attachmentBytes) }}
									</span>
								</div>

								<button
									class="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-full px-4 text-sm font-medium transition-all duration-200"
									:class="sending
										? 'cursor-wait bg-gradient-to-r from-primary-500 to-primary-600 text-white opacity-80 shadow-md shadow-primary-500/25'
										: canGenerate
											? 'bg-gradient-to-r from-primary-500 to-primary-600 text-white shadow-md shadow-primary-500/25 hover:from-primary-600 hover:to-primary-700 hover:shadow-lg hover:shadow-primary-500/30 active:scale-[0.98]'
											: 'cursor-not-allowed bg-gray-100 text-gray-400'"
									:disabled="!canGenerate"
									:title="sendTitle"
									@mousedown.prevent
									@click="generate"
								>
									<span v-if="sending" class="h-3.5 w-3.5 animate-spin rounded-full border-2 border-white/40 border-t-white"></span>
									<Icon v-else name="photo" size="xs" />
									{{ sending ? '生成中' : '生成' }}
									<span v-if="canGenerate" class="ml-0.5 text-[10px] opacity-70">↵</span>
								</button>
							</div>

							<!-- 拖放进行中的提示：壳子只变边框，这里补一句明确的落点说明 -->
							<div
								v-if="dragging"
								class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center rounded-[1.35rem] bg-white/75 text-xs font-medium text-primary-600 backdrop-blur-[1px]"
							>
								<Icon name="paperclip" size="sm" class="mr-1.5" />
								松开即可添加参考图（最多 {{ attachmentMax }} 张）
							</div>
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
						aria-label="关闭预览"
						@click="closeZoom"
					>
						<Icon name="x" size="sm" />
					</button>
				</div>
			</div>
		</Transition>
	</Teleport>
</template>

<style scoped>
/* 全局 `:focus-visible`（styles/main.css 末尾，无层级）会给聚焦的 textarea 描一圈
   2px 品牌色外框。在「整块画布 + 悬浮工具条」的沉浸式输入区里，这圈框只框住文字区，
   看起来像框里套框，破坏整体感。这里靠选择器特异性（类 + scoped 属性 > 单伪类）压掉它，
   聚焦反馈统一交给 .composer 自己的 focus-within 环。 */
.composer :deep(textarea:focus-visible) {
	outline: none;
	outline-offset: 0;
}

/* ── 消息操作小按钮：重新生成（与对话 Tab 同名同样式） ── */
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
