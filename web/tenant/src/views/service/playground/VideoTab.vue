<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { NInput, NInputNumber } from 'naive-ui'
import { createPlaygroundApi } from '@/utils/playgroundApi'
import { createPoller } from '@/composables/usePolling'
import Icon from '@/components/common/Icon.vue'
import GenerationProgressCard from './GenerationProgressCard.vue'
import BaseSelect from '../../../components/common/BaseSelect.vue'
import AttachmentBar from './AttachmentBar.vue'
import PromptInput from './PromptInput.vue'
import { useAttachments } from './useAttachments'
import { compileVideoPrompt } from './attachmentParts'

interface ModelItem {
	model_id: string
	model_name: string
	category: string
}
const props = defineProps<{ models: ModelItem[]; apiKey: string }>()

const submitting = ref(false)
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
const resolution = ref('1280x720')
const duration = ref(5)

// ── 参考素材 ──
// 前端按多素材形态实现（图/视/音皆可上传、@ 列表按类型命名），但本期只有首帧参考图
// 能随请求发出：/v1/videos 的 input_reference 仅承载一张图，其余素材待后端扩协议。
const {
	list: attachmentList,
	accept: acceptTypes,
	max: attachmentMax,
	addFiles: addAttachments,
	remove: removeAttachment,
} = useAttachments({ kinds: ['image', 'video', 'audio'], max: 6 })

const promptInput = ref<InstanceType<typeof PromptInput> | null>(null)
// 提交时未能随请求发出的素材名，用于结果区提示
const deferredNames = ref<string[]>([])

function insertMention(name: string) {
	promptInput.value?.insertMention(name)
}

const modelOptions = computed(() => props.models.map(m => ({ value: m.model_id, label: m.model_name || m.model_id })))

// 空态与任务卡里展示的模型名（列表由父组件异步加载，可能暂时查不到）
const selectedModelName = computed(() => {
	const m = props.models.find(x => x.model_id === selectedModel.value)
	return m?.model_name || selectedModel.value
})

// MiniMax-H3 系列使用官方分辨率档位（768P/2K，Max 追加 480P；与后端计费矩阵键一致），
// 其他模型维持像素串词汇——档位词汇不能混入其他厂商（会产生非法 resolution）
const pixelResolutionPresets = [
	{ value: '854x480', label: '480p 横' },
	{ value: '480x854', label: '480p 竖' },
	{ value: '1280x720', label: '720p 横' },
	{ value: '720x1280', label: '720p 竖' },
	{ value: '1920x1080', label: '1080p 横' },
	{ value: '1080x1920', label: '1080p 竖' },
]

const isMiniMaxH3 = computed(() => selectedModel.value.toLowerCase().startsWith('minimax-h3'))

const resolutionPresets = computed(() => {
	if (isMiniMaxH3.value) {
		const presets = [
			{ value: '768P', label: '768P' },
			{ value: '2K', label: '2K' },
		]
		if (selectedModel.value.toLowerCase().startsWith('minimax-h3-max')) {
			presets.unshift({ value: '480P', label: '480P' })
		}
		return presets
	}
	return pixelResolutionPresets
})

// 模型切换后当前分辨率不在该模型预设内时重置（避免像素串打到 MiniMax 或档位词汇打到其他厂商）
watch(resolutionPresets, presets => {
	if (!presets.some(p => p.value === resolution.value)) {
		resolution.value = presets[0]?.value || ''
	}
})

interface TaskInfo {
	id: string
	// OpenAI Videos 协议状态机：queued / in_progress / completed / failed
	status: string
	progress: string
	model: string
	error?: string
	createdAt: number
	completedAt?: number
}
const currentTask = ref<TaskInfo | null>(null)
const polling = ref(false)

// 结果卡右上角的状态徽标配色
const statusBadgeClass = computed(() => {
	switch (currentTask.value?.status) {
		case 'completed': return 'badge-success'
		case 'failed': return 'badge-danger'
		case 'in_progress': return 'badge-primary'
		default: return 'badge-warning'
	}
})
// 成品视频地址：/v1/videos/{id}/content 需带 API Key 拉流，取回后转 blob URL 供播放/下载
const videoUrl = ref('')
// 任务状态轮询：页面隐藏时暂停（任务在服务端继续执行），恢复可见时立即补拉
const taskPoller = createPoller(pollLoop, 3000, { immediate: true })

const statusLabel: Record<string, string> = {
	queued: '排队中',
	in_progress: '生成中',
	completed: '已完成',
	failed: '失败',
}

function applyResolutionPreset(val: string) {
	resolution.value = val
}

async function submitTask() {
	if (!prompt.value.trim() || !selectedModel.value) return
	submitting.value = true
	currentTask.value = null
	deferredNames.value = []
	stopPolling()
	releaseVideo()

	try {
		const api = createPlaygroundApi(props.apiKey)
		// 正文里的 [@图片1] 换成纯文字标签；首帧参考图取第一张图片附件
		const compiled = compileVideoPrompt(prompt.value, attachmentList.value)
		deferredNames.value = compiled.deferred.map(a => a.name)

		// OpenAI Videos 协议：size 承载分辨率（像素串或命名档位如 768P/2K），seconds 承载时长
		const body: Record<string, any> = {
			model: selectedModel.value,
			prompt: compiled.prompt,
		}
		if (resolution.value) {
			body.size = resolution.value
		}
		if (duration.value) {
			body.seconds = String(duration.value)
		}
		if (compiled.inputReference) {
			// 走 JSON 引用对象形态：后端 parseVideosInputReferenceJSON 接受 data URL，
			// 无需切 multipart，也不用跟 createPlaygroundApi 固定的 Content-Type 打架
			body.input_reference = { image_url: compiled.inputReference }
		}

		const res = await api.post('/v1/videos', body, { timeout: 120_000 })
		const data = res.data

		currentTask.value = {
			id: data.id,
			status: data.status || 'queued',
			progress: '',
			model: data.model || selectedModel.value,
			createdAt: data.created_at || Math.floor(Date.now() / 1000),
		}

		// 开始轮询
		if (data.status !== 'completed' && data.status !== 'failed') {
			startPolling()
		} else if (data.status === 'completed') {
			loadVideoContent()
		}
	} catch (e: any) {
		currentTask.value = {
			id: '',
			status: 'failed',
			progress: '',
			model: selectedModel.value,
			error: e?.message || '提交失败',
			createdAt: Math.floor(Date.now() / 1000),
		}
	} finally {
		submitting.value = false
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
	if (!polling.value || !currentTask.value?.id) return

	try {
		const api = createPlaygroundApi(props.apiKey)
		const res = await api.get(`/v1/videos/${currentTask.value.id}`)
		const data = res.data

		currentTask.value = {
			...currentTask.value,
			status: data.status,
			// OpenAI Videos 的 progress 是 0-100 整数，归一为百分比字符串供模板直用
			progress: data.progress != null ? `${data.progress}%` : currentTask.value.progress,
			error: data.error?.message || undefined,
			completedAt: data.completed_at || undefined,
		}

		if (data.status === 'completed' || data.status === 'failed') {
			stopPolling()
			if (data.status === 'completed') {
				loadVideoContent()
			}
			return
		}
	} catch {
		// 轮询失败不中断，继续尝试
	}
}

// loadVideoContent 拉取成品视频：content 端点要求 API Key 鉴权，浏览器无法直接用 URL 播放，
// 取回二进制后转 blob URL 供 <video> 播放与下载
async function loadVideoContent() {
	if (!currentTask.value?.id || videoUrl.value) return
	try {
		const api = createPlaygroundApi(props.apiKey)
		const res = await api.get(`/v1/videos/${currentTask.value.id}/content`, {
			responseType: 'blob',
			timeout: 300_000,
		})
		videoUrl.value = URL.createObjectURL(res.data)
	} catch {
		// 拉取失败不改变任务状态，下载按钮可重试（downloadVideo 内再次触发加载）
		currentTask.value = {
			...currentTask.value,
			error: currentTask.value?.error || '视频加载失败，请点击下载重试',
		}
	}
}

// releaseVideo 释放 blob URL，避免内存泄漏
function releaseVideo() {
	if (videoUrl.value) {
		URL.revokeObjectURL(videoUrl.value)
		videoUrl.value = ''
	}
}

// 组件卸载时停止轮询并释放资源，避免路由离开后仍持续请求任务状态
onUnmounted(() => {
	stopPolling()
	releaseVideo()
})

function resetTask() {
	stopPolling()
	releaseVideo()
	currentTask.value = null
}

function downloadVideo() {
	if (!videoUrl.value) {
		loadVideoContent()
		return
	}
	const a = document.createElement('a')
	a.href = videoUrl.value
	a.download = `video_${currentTask.value?.id || 'output'}.mp4`
	document.body.appendChild(a)
	a.click()
	document.body.removeChild(a)
}
</script>

<template>
	<!-- 与对话 Tab 同一套等高布局：左参数栏内部滚动，底部主操作常驻，右结果区撑满 -->
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

						<!-- 输入：参考素材 + 提示词 -->
						<div class="space-y-4 border-t border-gray-100 pt-4">
							<div>
								<label class="mb-2 block text-xs font-semibold text-gray-500">参考素材</label>
								<AttachmentBar
									:attachments="attachmentList"
									:accept="acceptTypes"
									:max="attachmentMax"
									@add="addAttachments"
									@remove="removeAttachment"
									@insert="insertMention"
								/>
							</div>
							<div>
								<label class="mb-2 block text-xs font-semibold text-gray-500">提示词</label>
								<PromptInput
									ref="promptInput"
									v-model="prompt"
									:attachments="attachmentList"
									:min-rows="4"
									placeholder="描述你想生成的视频，输入 @ 引用已上传的素材..."
									@paste="addAttachments"
								/>
							</div>
						</div>

						<!-- 输出规格 -->
						<div class="space-y-4 border-t border-gray-100 pt-4">
							<div>
								<div class="mb-2 flex items-center justify-between">
									<label class="text-xs font-semibold text-gray-500">分辨率</label>
									<span class="text-[11px] text-gray-400">也可直接输入自定义值</span>
								</div>
								<n-input v-model:value="resolution" placeholder="如 1280x720" />
								<div class="mt-2 flex flex-wrap gap-1.5">
									<button
										v-for="p in resolutionPresets"
										:key="p.value"
										class="rounded-lg border px-2.5 py-1 text-xs transition-all duration-150"
										:class="resolution === p.value
											? 'border-primary-400 bg-primary-50 font-medium text-primary-700'
											: 'border-gray-200 bg-white text-gray-600 hover:border-primary-200 hover:text-primary-600'"
										@click="applyResolutionPreset(p.value)"
									>
										{{ p.label }}
									</button>
								</div>
							</div>

							<div>
								<div class="mb-2 flex items-center justify-between">
									<label class="text-xs font-semibold text-gray-500">时长</label>
									<span class="rounded-md bg-primary-50 px-1.5 py-0.5 font-mono text-xs font-semibold text-primary-600">
										{{ duration }} 秒
									</span>
								</div>
								<!-- min=4 覆盖 MiniMax-H3 的 4 秒档；其他模型档位由上游校验 -->
								<n-input-number v-model:value="duration" :min="4" :max="15" :step="1" class="w-full" />
							</div>
						</div>
					</div>

					<!-- 主操作常驻卡片底部：参数再长也不用来回滚动找按钮 -->
					<div class="shrink-0 space-y-2 border-t border-gray-100 px-4 py-3">
						<button class="btn btn-primary w-full" :disabled="submitting || polling || !prompt.trim()" @click="submitTask">
							<Icon name="play" size="sm" />
							{{ submitting ? '提交中...' : polling ? '生成中...' : '生成视频' }}
						</button>
						<button v-if="currentTask" class="btn btn-secondary w-full" @click="resetTask">
							<Icon name="refresh" size="sm" />
							重新生成
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
						<span v-if="currentTask" class="badge" :class="statusBadgeClass">
							{{ statusLabel[currentTask.status] || currentTask.status }}
						</span>
					</div>

					<div class="min-h-0 flex-1 overflow-y-auto p-5">
						<!-- 空态：图标磁贴 + 说明，给首屏一个视觉锚点 -->
						<div v-if="!currentTask && !submitting" class="flex h-full min-h-[320px] flex-col items-center justify-center px-4 text-center">
							<div class="mb-5 flex h-16 w-16 items-center justify-center rounded-3xl bg-gradient-to-br from-cyan-400 via-primary-500 to-emerald-500 text-white shadow-glow">
								<Icon name="film" size="lg" />
							</div>
							<h3 class="text-lg font-semibold text-gray-900">等待生成</h3>
							<p class="mt-1.5 max-w-sm text-sm text-gray-500">
								<template v-if="selectedModel">
									当前模型 <span class="font-medium text-gray-700">{{ selectedModelName }}</span>，填写提示词后点击「生成视频」
								</template>
								<template v-else>请先在左侧选择一个视频模型</template>
							</p>
							<p class="mt-6 inline-flex items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-[11px] text-amber-700">
								<Icon name="exclamationTriangle" size="xs" />
								生成将真实调用模型并产生费用
							</p>
						</div>

						<!-- 本期协议带不动的素材：提交后明确告知，避免用户以为已生效 -->
						<div v-if="deferredNames.length" class="mb-4 rounded-xl border border-amber-200 bg-amber-50 p-3">
							<div class="flex items-center gap-2 text-amber-700">
								<Icon name="exclamationTriangle" size="sm" />
								<span class="text-sm font-medium">部分素材未随本次请求发送</span>
							</div>
							<p class="mt-1 text-xs text-amber-600">
								{{ deferredNames.join('、') }} — 当前仅支持一张首帧参考图，多素材引用待后端支持
							</p>
						</div>

						<!-- 提交中：卡片式占位 -->
						<GenerationProgressCard v-if="submitting" kind="video" label="正在提交生成任务..." />

						<!-- 任务状态 -->
						<div v-if="currentTask" class="space-y-4">
							<!-- 任务进行中：按成品 16:9 形状的卡片式进度占位 -->
							<GenerationProgressCard
								v-if="currentTask.status === 'queued' || currentTask.status === 'in_progress'"
								kind="video"
								:progress="currentTask.progress"
								:label="statusLabel[currentTask.status] || currentTask.status"
								:sublabel="currentTask.id ? 'task_id: ' + currentTask.id : ''"
							/>

							<!-- 视频结果 -->
							<div v-if="currentTask.status === 'completed' && videoUrl" class="space-y-3">
								<video controls class="w-full rounded-xl border border-gray-200 bg-black" :src="videoUrl" />
								<button class="btn btn-secondary btn-sm" @click="downloadVideo">
									<Icon name="arrowDown" size="sm" />
									下载视频
								</button>
							</div>
							<!-- 成品加载中（content 端点拉流转 blob） -->
							<GenerationProgressCard
								v-else-if="currentTask.status === 'completed'"
								kind="video"
								label="视频加载中..."
							/>

							<!-- 错误信息 -->
							<div v-if="currentTask.status === 'failed'" class="rounded-xl border border-red-200 bg-red-50 p-4">
								<div class="flex items-center gap-2 text-red-700">
									<Icon name="xCircle" size="sm" />
									<span class="text-sm font-medium">生成失败</span>
								</div>
								<p v-if="currentTask.error" class="mt-2 text-sm text-red-600">{{ currentTask.error }}</p>
							</div>

							<!-- 任务详情 -->
							<div class="space-y-1 border-t border-gray-100 pt-3 text-xs text-gray-500">
								<div class="flex items-center justify-between">
									<span>任务 ID: {{ currentTask.id }}</span>
									<span>模型: {{ currentTask.model }}</span>
								</div>
							</div>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>
