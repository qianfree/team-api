<script setup lang="ts">
import { ref, computed, watch, onUnmounted, nextTick } from 'vue'
import { NInputNumber } from 'naive-ui'
import { createPlaygroundApi } from '@/utils/playgroundApi'
import { createPoller } from '@/composables/usePolling'
import Icon from '@/components/common/Icon.vue'
import GenerationProgressCard from './GenerationProgressCard.vue'
import BaseSelect from '../../../components/common/BaseSelect.vue'
import AttachmentBar from './AttachmentBar.vue'
import PromptInput from './PromptInput.vue'
import { useAttachments, formatBytes, type Attachment } from './useAttachments'
import { compileVideoPrompt, stripMentions, type AttachmentPreview } from './attachmentParts'
import { uploadVideoAttachments } from './relayFileUpload'
import { usePublicSettings } from '@/composables/usePublicSettings'
import { toast } from '@/utils/toast'

interface ModelItem {
	model_id: string
	model_name: string
	category: string
}
const props = defineProps<{ models: ModelItem[]; apiKey: string }>()

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
const prompt = ref('')

// 分辨率档位下拉：取值与网关 per_second 计费矩阵的档位键一致（480P/720P/768P/1080P/2K/4K），
// 经 OpenAI Videos 协议的 size 字段透传，网关归一进 metadata.size 由各渠道适配器消费；
// 具体型号不支持的档位由上游 400 明示（费用全额退款），故无需按模型收窄选项。默认 720P。
const resolution = ref('720P')
const resolutionOptions = [
	{ value: '480P', label: '480P' },
	{ value: '720P', label: '720P' },
	{ value: '768P', label: '768P' },
	{ value: '1080P', label: '1080P' },
	{ value: '2K', label: '2K' },
	{ value: '4K', label: '4K' },
]

// 时长 1-30s 自由填写：网关对 seconds 只做形态归一不做白名单（normalizeVideosSeconds），
// 各模型实际支持的档位由上游校验
const duration = ref(5)

// 屏幕比例：取值原样透传给上游（网关不做换算与白名单），默认 16:9 —— 与 MiniMax-H3
// 纯文本生成时的官方默认档一致，因此对现有请求语义无变化
const aspectRatio = ref('16:9')
const aspectRatioOptions = [
	{ value: '16:9', label: '16:9 横屏' },
	{ value: '9:16', label: '9:16 竖屏' },
	{ value: '1:1', label: '1:1 方形' },
	{ value: '4:3', label: '4:3' },
	{ value: '3:4', label: '3:4' },
	{ value: '21:9', label: '21:9 宽银幕' },
]

// ── 参考素材 ──
// 全部素材随请求经 references[] 以参考形式发出（compileVideoPrompt 决定角色）：图片一律
// reference_image、音频 reference_audio、视频 reference_video——不做「单图转首帧」，
// 首帧与参考图是上游两种语义（MiniMax H3 / grsai 等上游首尾帧须成对且与参考图互斥），
// 仅多模态参考模型承接，不支持时上游明确 400。素材引用形态由上传通道可用性决定（见
// runGenerate）：云存储已配置时先经 /v1/files 上传换 file_id（网关解析为对象存储签名 URL
// 发上游），否则/失败回退 base64 内联（视频素材无法内联，未上传成功则不发并明确提示）。
const {
	list: attachmentList,
	totalBytes: attachmentBytes,
	accept: acceptTypes,
	max: attachmentMax,
	addFiles: addAttachments,
	remove: removeAttachment,
	clear: clearAttachments,
} = useAttachments({ kinds: ['image', 'video', 'audio'], max: 6 })

// /v1/files 上传通道可用性（全局单例 + 30s 缓存，无需每次提交重新拉取）
const { settings: publicSettings, fetchSettings: fetchPublicSettings } = usePublicSettings()
const uploadEnabled = computed(() => publicSettings.value.relay_files_upload_enabled === true)
fetchPublicSettings()

const promptInput = ref<InstanceType<typeof PromptInput> | null>(null)
// bare 变体的附件片不自带上传入口，文件选择器由输入区的「素材」按钮通过 ref 触发
const attachmentBar = ref<InstanceType<typeof AttachmentBar> | null>(null)
// 输入容器的拖拽高亮状态（与对话 Tab 一致，拖放由 input-shell 承接）
const dragging = ref(false)

function insertMention(name: string) {
	promptInput.value?.insertMention(name)
}

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

const modelOptions = computed(() => props.models.map(m => ({ value: m.model_id, label: m.model_name || m.model_id })))

// 空态与任务卡里展示的模型名（列表由父组件异步加载，可能暂时查不到）
const selectedModelName = computed(() => {
	const m = props.models.find(x => x.model_id === selectedModel.value)
	return m?.model_name || selectedModel.value
})

// ── 生成记录（对话式，与图片/对话 Tab 同构）──
// user = 提示词与参考素材快照，assistant = 该次生成的等待动画/成品/错误，
// 发送即入列并清空输入框，等待动画落在配对的气泡里，与提示词的关联一目了然。

interface VideoTaskInfo {
	id: string
	// OpenAI Videos 协议状态机：queued / in_progress / completed / failed
	status: string
	progress: string
	model: string
	error?: string
	createdAt: number
	completedAt?: number
	// completed 时的用量摘要（网关扩展字段，渠道上报了哪些字段才出现哪些）
	usage?: Record<string, any>
}

/** 一次生成的参数快照：气泡详情展示与重新生成都以此为准（不读当前面板值） */
interface VideoSpec {
	model: string
	resolution: string
	duration: number
	aspectRatio: string
}

interface VideoUserMsg {
	id: number
	role: 'user'
	/** 展示文本：[@图片1] 已换成纯文字标签 */
	text: string
	/** 气泡里的素材缩略图 */
	previews: AttachmentPreview[]
	/** 素材完整快照（dataUrl/blob/fileId 缓存），重新生成时复用 */
	attachments: Attachment[]
}

interface VideoAssistantMsg {
	id: number
	role: 'assistant'
	/** 任务创建前的提交阶段：上传素材 / 提交任务；'' = 已进入任务态 */
	phase: '' | 'uploading' | 'submitting'
	task: VideoTaskInfo | null
	/** 本条生成起点（ms）：进度卡计时用，跨「上传 → 提交 → 排队/生成中」连续计时 */
	startAt: number
	/** 成品播放地址：video_url 免鉴权直链，或 content 端点拉回的 blob: */
	videoUrl: string
	/** 下载进行中：经 /content 端点按需拉流，防重复点击 */
	downloading?: boolean
	/** 提交时未能随请求发出的素材名，气泡内提示 */
	deferredNames: string[]
	spec: VideoSpec
}

type VideoMsg = VideoUserMsg | VideoAssistantMsg

const messages = ref<VideoMsg[]>([])
let nextMsgId = 1

// 当前进行中的 assistant 消息（提交中或轮询中）：同一时间至多一条，是轮询的目标
let activeMsg: VideoAssistantMsg | null = null

const sending = ref(false)
const polling = ref(false)
const busy = computed(() => sending.value || polling.value)

// 任务状态轮询：10s 一次——任务态查询是轻量 GET，生成普遍耗时分钟级，低频足够；
// 页面隐藏时暂停（任务在服务端继续执行），恢复可见时立即补拉
const taskPoller = createPoller(pollLoop, 10_000, { immediate: true })

const statusLabel: Record<string, string> = {
	queued: '排队中',
	in_progress: '生成中',
	completed: '已完成',
	failed: '失败',
}

// 头部状态徽标：取最近一条生成记录的状态（无记录时不渲染）
const resultStatus = computed<{ label: string; cls: string } | null>(() => {
	const last = messages.value[messages.value.length - 1]
	if (!last || last.role !== 'assistant') return null
	if (last.phase) return { label: '提交中', cls: 'badge-warning' }
	switch (last.task?.status) {
		case 'completed': return { label: '已完成', cls: 'badge-success' }
		case 'failed': return { label: '失败', cls: 'badge-danger' }
		case 'in_progress': return { label: '生成中', cls: 'badge-primary' }
		case 'queued': return { label: '排队中', cls: 'badge-warning' }
	}
	return null
})

// 提交按钮文案用：当前提交阶段（上传素材/提交任务）
const sendingPhaseLabel = computed(() => {
	if (!sending.value) return ''
	const last = messages.value[messages.value.length - 1]
	if (last?.role === 'assistant' && last.phase) {
		return last.phase === 'uploading' ? '上传素材中...' : '提交中...'
	}
	return ''
})

// deferred 提示文案：区分「上传通道未启用」与「通道可用但上传失败」两种成因
const deferredNotice = computed(() =>
	uploadEnabled.value
		? '视频素材需先上传对象存储后以 URL 引用，本次上传失败（或类型未被服务端允许），未随请求发送'
		: '视频素材需以 URL 引用，当前服务端未启用对象存储上传通道，未随请求发送',
)

// 用量摘要（completed 时后端附带，字段随渠道而异、缺者不出现）
function usageText(task: VideoTaskInfo | null): string {
	const u = task?.usage
	if (!u) return ''
	const parts: string[] = []
	if (u.output_video_seconds != null) parts.push(`${u.output_video_seconds} 秒`)
	if (u.resolution) parts.push(String(u.resolution))
	if (u.total_tokens != null) parts.push(`${u.total_tokens} tokens`)
	return parts.join(' · ')
}

// scrollToBottom / isNearBottom：与对话/图片 Tab 同一套滚动策略（40px 容差）
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

// generate 从输入区发起一次生成：提示词与素材快照入列成 user 消息，随即清空输入框与素材区；
// 结果以配对的 assistant 消息呈现（等待动画 → 成品/错误都在该气泡里）
async function generate() {
	const raw = prompt.value.trim()
	if (!raw || !selectedModel.value || busy.value) return

	const attachments = [...attachmentList.value]
	messages.value.push({
		id: nextMsgId++,
		role: 'user',
		text: stripMentions(raw),
		previews: attachments.map(a => ({ name: a.name, kind: a.kind, url: a.dataUrl })),
		attachments,
	})
	prompt.value = ''
	clearAttachments()

	const msg: VideoAssistantMsg = {
		id: nextMsgId++,
		role: 'assistant',
		phase: 'uploading',
		task: null,
		startAt: Date.now(),
		videoUrl: '',
		deferredNames: [],
		spec: {
			model: selectedModel.value,
			resolution: resolution.value,
			duration: duration.value,
			aspectRatio: aspectRatio.value,
		},
	}
	messages.value.push(msg)
	scrollToBottom(true)

	await runGenerate(raw, attachments, msg)
}

// regenerate 从一条历史记录重发：复用其提示词、素材与参数快照，追加新的生成记录
async function regenerate(msg: VideoAssistantMsg) {
	if (busy.value) return
	const idx = messages.value.indexOf(msg)
	const userMsg = idx > 0 ? messages.value[idx - 1] : null
	if (!userMsg || userMsg.role !== 'user') return

	messages.value.push({
		id: nextMsgId++,
		role: 'user',
		text: userMsg.text,
		previews: userMsg.previews,
		attachments: userMsg.attachments,
	})
	const newMsg: VideoAssistantMsg = {
		id: nextMsgId++,
		role: 'assistant',
		phase: 'uploading',
		task: null,
		startAt: Date.now(),
		videoUrl: '',
		deferredNames: [],
		spec: { ...msg.spec },
	}
	messages.value.push(newMsg)
	scrollToBottom(true)

	await runGenerate(userMsg.text, userMsg.attachments, newMsg)
}

// runGenerate 一次生成的执行段（首次发送与重新生成共用）：结果写回 msg
async function runGenerate(text: string, attachments: Attachment[], msg: VideoAssistantMsg) {
	sending.value = true
	activeMsg = msg
	msg.startAt = Date.now()
	msg.phase = 'uploading'
	msg.task = null
	releaseVideoUrl(msg)
	msg.deferredNames = []
	scrollToBottom(true)

	try {
		const api = createPlaygroundApi(props.apiKey)

		// 上传通道可用时参考素材先上传换 file_id（同一附件多次提交复用 att.fileId 缓存）；
		// 失败不阻断提交——图片/音频在编译阶段自动回退 base64 内联，视频进 deferred
		let fileIds: Map<number, string> | undefined
		if (attachments.length && uploadEnabled.value) {
			fileIds = await uploadVideoAttachments(api, attachments)
			if (fileIds.size < attachments.length) {
				toast.warning('部分素材上传对象存储失败，图片/音频将以内联方式发送')
			}
		}
		msg.phase = 'submitting'

		// 正文里的 [@图片1] 换成纯文字标签；参考素材全部以 reference_* 角色发出
		const compiled = compileVideoPrompt(text, attachments, fileIds)
		msg.deferredNames = compiled.deferred.map(a => a.name)

		// OpenAI Videos 协议：size 承载分辨率档位（如 720P/2K），seconds 承载时长；
		// aspect_ratio 是网关侧的协议扩展字段（官方 Videos 协议无此项），网关归一后双写
		// 进任务体 metadata，由各渠道适配器按自己的词汇消费。
		const body: Record<string, any> = {
			model: msg.spec.model,
			prompt: compiled.prompt,
		}
		if (msg.spec.resolution) {
			body.size = msg.spec.resolution
		}
		if (msg.spec.duration) {
			body.seconds = String(msg.spec.duration)
		}
		if (msg.spec.aspectRatio) {
			body.aspect_ratio = msg.spec.aspectRatio
		}
		// 参考素材唯一通道，角色由 compileVideoPrompt 决定（图片一律 reference_image）
		if (compiled.references.length) {
			body.references = compiled.references
		}

		const res = await api.post('/v1/videos', body, { timeout: 120_000 })
		const data = res.data

		msg.task = {
			id: data.id,
			status: data.status || 'queued',
			progress: '',
			model: data.model || msg.spec.model,
			createdAt: data.created_at || Math.floor(Date.now() / 1000),
		}
		msg.phase = ''

		// 开始轮询
		if (data.status !== 'completed' && data.status !== 'failed') {
			startPolling()
		} else {
			activeMsg = null
			if (data.status === 'completed') {
				applyVideoResult(msg, data.video_url)
			}
		}
	} catch (e: any) {
		msg.phase = ''
		activeMsg = null
		msg.task = {
			id: '',
			status: 'failed',
			progress: '',
			model: msg.spec.model,
			error: e?.message || '提交失败',
			createdAt: Math.floor(Date.now() / 1000),
		}
	} finally {
		sending.value = false
		scrollToBottom()
	}
}

function startPolling() {
	polling.value = true
	taskPoller.start()
}

function stopPolling() {
	polling.value = false
	taskPoller.stop()
}

async function pollLoop() {
	const msg = activeMsg
	if (!polling.value || !msg?.task?.id) return

	try {
		const api = createPlaygroundApi(props.apiKey)
		const res = await api.get(`/v1/videos/${msg.task!.id}`)
		const data = res.data

		msg.task = {
			...msg.task,
			status: data.status,
			// OpenAI Videos 的 progress 是 0-100 整数，归一为百分比字符串供模板直用
			progress: data.progress != null ? `${data.progress}%` : msg.task.progress,
			error: data.error?.message || undefined,
			completedAt: data.completed_at || undefined,
			usage: data.usage || undefined,
		}

		if (data.status === 'completed' || data.status === 'failed') {
			stopPolling()
			activeMsg = null
			if (data.status === 'completed') {
				applyVideoResult(msg, data.video_url)
			}
		}
	} catch {
		// 轮询失败不中断，继续尝试
	}
}

// applyVideoResult completed 时的成品地址接入：有 video_url 直链直接作为播放源（免鉴权，
// 已转存时直连对象存储）；无直链（旧后端未签发等）兜底走 content 端点代理拉流
function applyVideoResult(msg: VideoAssistantMsg, url?: string) {
	if (url) {
		msg.videoUrl = url
		return
	}
	loadVideoContent(msg)
}

// loadVideoContent 兜底拉流：content 端点要求 API Key 鉴权，浏览器无法直接用 URL 播放，
// 取回二进制后转 blob URL 供 <video> 播放
async function loadVideoContent(msg: VideoAssistantMsg) {
	if (!msg.task?.id || msg.videoUrl) return
	try {
		const api = createPlaygroundApi(props.apiKey)
		const res = await api.get(`/v1/videos/${msg.task.id}/content`, {
			responseType: 'blob',
			timeout: 300_000,
		})
		msg.videoUrl = URL.createObjectURL(res.data)
	} catch {
		// 拉取失败不改变任务状态，可经下载按钮重试
		if (msg.task) {
			msg.task = { ...msg.task, error: msg.task.error || '视频加载失败，可尝试下载' }
		}
	}
}

// releaseVideoUrl 释放 blob URL，避免内存泄漏（直链地址 revoke 无副作用，仅清理 blob:）
function releaseVideoUrl(msg: VideoAssistantMsg) {
	if (msg.videoUrl.startsWith('blob:')) {
		URL.revokeObjectURL(msg.videoUrl)
	}
	msg.videoUrl = ''
}

function releaseAllVideos() {
	for (const m of messages.value) {
		if (m.role === 'assistant') releaseVideoUrl(m)
	}
}

// downloadVideo 下载成品：经 /content 端点按需拉流转 blob 下载。video_url 直链多为跨域
//（对象存储域名），浏览器会忽略 <a download> 属性（变成打开播放）、未配 CORS 时 fetch 也
// 受阻；content 端点同源可靠且可指定文件名。流量仅在用户点击下载时发生
async function downloadVideo(msg: VideoAssistantMsg) {
	if (!msg.task?.id || msg.downloading) return
	msg.downloading = true
	try {
		const api = createPlaygroundApi(props.apiKey)
		const res = await api.get(`/v1/videos/${msg.task.id}/content`, {
			responseType: 'blob',
			timeout: 300_000,
		})
		const blobUrl = URL.createObjectURL(res.data)
		const a = document.createElement('a')
		a.href = blobUrl
		a.download = `video_${msg.task.id}.mp4`
		document.body.appendChild(a)
		a.click()
		document.body.removeChild(a)
		// 浏览器拿到 blob 引用后即可保存，延迟释放兜底极快场景下被过早回收
		window.setTimeout(() => URL.revokeObjectURL(blobUrl), 30_000)
	} catch (e: any) {
		toast.error(e?.message || '视频下载失败，请重试')
	} finally {
		msg.downloading = false
	}
}

// 清空对话：停止轮询并释放所有 blob URL，进行中任务在服务端继续（记录一并丢弃）
function clearHistory() {
	if (busy.value) return
	stopPolling()
	activeMsg = null
	releaseAllVideos()
	messages.value = []
}

// 组件卸载时停止轮询并释放资源，避免路由离开后仍持续请求任务状态
onUnmounted(() => {
	stopPolling()
	releaseAllVideos()
})
</script>

<template>
	<!-- 与对话/图片 Tab 同一套等高布局：左参数栏内部滚动，右侧为对话式生成记录、
	     消息区撑满滚动，提示词与素材入口收在卡片底部，主操作按钮落在输入区操作行 -->
	<div class="flex flex-col lg:h-[calc(100vh-15rem)] lg:overflow-hidden">
		<div class="grid grid-cols-1 gap-4 lg:grid-cols-[22rem_minmax(0,1fr)] lg:grid-rows-[minmax(0,1fr)] lg:min-h-0 lg:flex-1">
			<!-- Left: 参数 -->
			<div class="flex lg:min-h-0">
				<div class="card flex w-full flex-col overflow-hidden lg:h-full">
					<div class="flex shrink-0 items-center gap-2 border-b border-gray-100 px-4 py-3">
						<Icon name="film" size="sm" class="text-primary-500" />
						<h3 class="text-sm font-semibold text-gray-900">视频生成参数</h3>
					</div>

					<div class="flex-1 space-y-4 overflow-y-auto px-4 py-4">
						<!-- 模型 -->
						<div>
							<div class="mb-2 flex items-center justify-between">
								<label class="text-xs font-semibold text-gray-500">模型</label>
								<span class="text-[11px] text-gray-400">共 {{ models.length }} 个可用</span>
							</div>
							<BaseSelect v-model="selectedModel" :options="modelOptions" />
						</div>

						<!-- 输出规格 -->
						<div class="space-y-4 border-t border-gray-100 pt-4">
							<div>
								<label class="mb-2 block text-xs font-semibold text-gray-500">分辨率</label>
								<BaseSelect v-model="resolution" :options="resolutionOptions" />
								<p class="mt-1 text-[11px] text-gray-400">档位经 size 字段透传，各模型支持范围以实际报错为准</p>
							</div>

							<div>
								<label class="mb-2 block text-xs font-semibold text-gray-500">屏幕比例</label>
								<BaseSelect v-model="aspectRatio" :options="aspectRatioOptions" />
							</div>

							<div>
								<div class="mb-2 flex items-center justify-between">
									<label class="text-xs font-semibold text-gray-500">时长</label>
									<span class="rounded-md bg-primary-50 px-1.5 py-0.5 font-mono text-xs font-semibold text-primary-600">
										{{ duration }} 秒
									</span>
								</div>
								<!-- 1-30s 自由填写：档位校验交由上游，网关只透传 seconds -->
								<n-input-number v-model:value="duration" :min="1" :max="30" :step="1" class="w-full" />
							</div>
						</div>
					</div>
				</div>
			</div>

			<!-- Right: 生成记录 + 输入（对话式：提示词发送后入列，等待动画与成品都在配对的气泡里） -->
			<div class="h-[62vh] min-h-[420px] lg:h-auto lg:min-h-0">
				<div class="card flex h-full flex-col overflow-hidden">
					<div class="flex shrink-0 items-center justify-between gap-3 border-b border-gray-100 px-5 py-3">
						<div class="flex items-center gap-2">
							<Icon name="film" size="sm" class="text-primary-500" />
							<h3 class="text-sm font-semibold text-gray-900">生成记录</h3>
						</div>
						<span v-if="resultStatus" class="badge" :class="resultStatus.cls">{{ resultStatus.label }}</span>
					</div>

					<!-- 消息区 -->
					<div ref="messagesRef" class="min-h-0 flex-1 overflow-y-auto px-5 py-5 lg:px-6">
						<!-- 空态：图标磁贴 + 说明，给首屏一个视觉锚点 -->
						<div v-if="messages.length === 0" class="flex min-h-full flex-col items-center justify-center px-4 text-center">
							<div class="mb-5 flex h-16 w-16 items-center justify-center rounded-3xl bg-gradient-to-br from-cyan-400 via-primary-500 to-emerald-500 text-white shadow-glow">
								<Icon name="film" size="lg" />
							</div>
							<h3 class="text-lg font-semibold text-gray-900">开始生成</h3>
							<p class="mt-1.5 max-w-sm text-sm text-gray-500">
								<template v-if="selectedModel">
									当前模型 <span class="font-medium text-gray-700">{{ selectedModelName }}</span>，在下方输入提示词后点击「生成视频」
								</template>
								<template v-else>请先在左侧选择一个视频模型</template>
							</p>
							<p class="mt-7 inline-flex items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-[11px] text-amber-700">
								<Icon name="exclamationTriangle" size="xs" />
								生成将真实调用模型并产生费用
							</p>
						</div>

						<!-- 消息列表 -->
						<div v-else class="space-y-5">
							<template v-for="msg in messages" :key="msg.id">
								<!-- 提示词（右对齐，附素材缩略） -->
								<div v-if="msg.role === 'user'" class="flex animate-slide-up justify-end">
									<div class="max-w-[80%] space-y-1.5">
										<!-- 随消息发出的素材：用 data URL 展示（输入区的 objectURL 发送后即被释放） -->
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
										<div class="rounded-2xl rounded-tr-md bg-gradient-to-br from-primary-500 to-primary-600 px-4 py-3 text-white shadow-sm shadow-primary-500/20">
											<p class="text-sm whitespace-pre-wrap">{{ msg.text }}</p>
										</div>
									</div>
								</div>

								<!-- 生成消息（左对齐）：等待动画/成品/错误都在同一条气泡里，与提示词配对 -->
								<div v-else class="flex max-w-[85%] animate-slide-up gap-3">
									<div class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-primary-400 to-primary-600 text-white shadow-sm shadow-primary-500/25">
										<Icon name="film" size="sm" />
									</div>
									<div class="min-w-0 flex-1 space-y-2">
										<!-- 提交中 / 排队 / 生成中：等待动画作为气泡内容，关联性一目了然 -->
										<GenerationProgressCard
											v-if="msg.phase || msg.task?.status === 'queued' || msg.task?.status === 'in_progress'"
											kind="video"
											compact
											:progress="msg.phase || !msg.task ? '' : msg.task.progress"
											:label="msg.phase === 'uploading'
												? '正在上传参考素材...'
												: msg.phase === 'submitting'
													? '正在提交生成任务...'
													: (statusLabel[msg.task!.status] || msg.task!.status)"
											:sublabel="!msg.phase && msg.task?.id ? 'task_id: ' + msg.task.id : ''"
											:since="msg.startAt"
										/>

										<!-- 成品：视频 + 操作 -->
										<div v-else-if="msg.task?.status === 'completed' && msg.videoUrl" class="space-y-2">
											<video controls class="bubble-video" :src="msg.videoUrl" />
											<div class="flex items-center gap-2">
												<button class="btn btn-secondary btn-sm" :disabled="msg.downloading" @click="downloadVideo(msg)">
													<Icon name="arrowDown" size="sm" />
													{{ msg.downloading ? '下载中...' : '下载视频' }}
												</button>
												<button class="msg-action" :disabled="busy" @click="regenerate(msg)">
													<Icon name="refresh" size="xs" />
													重新生成
												</button>
											</div>
										</div>

										<!-- 成品加载中（content 端点拉流转 blob） -->
										<GenerationProgressCard
											v-else-if="msg.task?.status === 'completed'"
											kind="video"
											compact
											label="视频加载中..."
										/>

										<!-- 错误信息 -->
										<div v-else-if="msg.task?.status === 'failed'" class="rounded-xl border border-red-200 bg-red-50 p-4">
											<div class="flex items-center gap-2 text-red-700">
												<Icon name="xCircle" size="sm" />
												<span class="text-sm font-medium">生成失败</span>
											</div>
											<p v-if="msg.task.error" class="mt-2 break-all text-sm text-red-600">{{ msg.task.error }}</p>
											<button
												class="mt-3 inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-2.5 py-1 text-xs font-medium text-red-600 transition-colors duration-150 hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-50"
												:disabled="busy"
												@click="regenerate(msg)"
											>
												<Icon name="refresh" size="xs" />
												重试
											</button>
										</div>

										<!-- 协议带不动的素材（视频附件）：提交后明确告知，避免用户以为已生效 -->
										<div v-if="msg.deferredNames.length" class="rounded-xl border border-amber-200 bg-amber-50 p-3">
											<div class="flex items-center gap-2 text-amber-700">
												<Icon name="exclamationTriangle" size="sm" />
												<span class="text-sm font-medium">部分素材未随本次请求发送</span>
											</div>
											<p class="mt-1 text-xs text-amber-600">
												{{ msg.deferredNames.join('、') }} — {{ deferredNotice }}
											</p>
										</div>

										<!-- 任务详情：参数快照 + 实际用量 -->
										<div v-if="msg.task?.id" class="space-y-1 text-xs text-gray-400">
											<div class="flex items-center justify-between gap-3">
												<span class="min-w-0 truncate">任务 {{ msg.task.id }}</span>
												<span class="shrink-0">{{ msg.spec.resolution }} · {{ msg.spec.aspectRatio }} · {{ msg.spec.duration }}s</span>
											</div>
											<div v-if="usageText(msg.task)" class="flex items-center justify-between gap-3">
												<span class="min-w-0 truncate">用量 {{ usageText(msg.task) }}</span>
												<span class="shrink-0">实际生成规格（多退少补计费）</span>
											</div>
										</div>
									</div>
								</div>
							</template>
						</div>
					</div>

					<!-- 输入区：参考素材片 + 提示词 + 操作行整合进同一个圆角盒子（与对话 Tab 同构） -->
					<div class="shrink-0 border-t border-gray-100 p-3 lg:p-4">
						<div
							class="input-shell"
							:class="{ 'input-shell-dragging': dragging }"
							@dragover.prevent="dragging = true"
							@dragleave="dragging = false"
							@drop.prevent="onDropFiles"
						>
							<!-- 素材片（bare：无边框无上传按钮，融入本容器）；
							     文件选择器仍挂载于此，由下方「素材」按钮通过 ref 触发 -->
							<AttachmentBar
								ref="attachmentBar"
								variant="bare"
								:class="attachmentList.length ? 'mb-2' : ''"
								:attachments="attachmentList"
								:accept="acceptTypes"
								:max="attachmentMax"
								@add="addAttachments"
								@remove="removeAttachment"
								@insert="insertMention"
							/>

							<PromptInput
								ref="promptInput"
								v-model="prompt"
								bare
								:attachments="attachmentList"
								:max-rows="3"
								placeholder="描述你想生成的视频，输入 @ 引用已上传的素材..."
								@submit="generate"
								@paste="addAttachments"
							/>

							<!-- 操作行 -->
							<div class="mt-1.5 flex items-center justify-between gap-3">
								<div class="flex min-w-0 items-center gap-1">
									<button
										class="input-tool"
										title="添加参考素材（图片/视频/音频，也可拖拽到此处或直接粘贴）"
										@click="openFilePicker"
									>
										<Icon name="paperclip" size="xs" />
										素材
									</button>
									<button
										v-if="attachmentList.length"
										class="input-tool"
										title="清空已选素材"
										@click="clearAllAttachments"
									>
										<Icon name="x" size="xs" />
										清空素材
									</button>
									<span class="ml-1 hidden truncate text-[11px] text-gray-400 sm:inline">
										Enter 生成 · Shift + Enter 换行<template v-if="attachmentList.length"> · 素材 {{ formatBytes(attachmentBytes) }}</template>
									</span>
								</div>
								<div class="flex shrink-0 items-center gap-2">
									<button v-if="messages.length" class="btn btn-ghost btn-sm text-gray-500" :disabled="busy" @click="clearHistory">
										<Icon name="refresh" size="xs" />
										清空对话
									</button>
									<button class="btn btn-primary btn-sm px-4" :disabled="busy || !prompt.trim()" @click="generate">
										<Icon name="play" size="xs" />
										{{ sending ? (sendingPhaseLabel || '提交中...') : polling ? '生成中...' : '生成视频' }}
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
/* 气泡内成品视频：宽度上限收敛在消息列内，高度封顶避免竖屏视频把消息区撑出长滚动条 */
.bubble-video {
	display: block;
	width: min(100%, 26rem);
	max-height: min(56vh, 30rem);
	border-radius: 0.75rem;
	border: 1px solid #e5e7eb;
	background: #000;
}

/* ── 消息操作小按钮：重新生成（与对话/图片 Tab 同名同样式） ── */
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
