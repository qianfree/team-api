<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { NInput, NInputNumber } from 'naive-ui'
import { createPlaygroundApi } from '@/utils/playgroundApi'
import { createPoller } from '@/composables/usePolling'
import Icon from '@/components/common/Icon.vue'
import BaseSelect from '../../../components/common/BaseSelect.vue'

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

const modelOptions = computed(() => props.models.map(m => ({ value: m.model_id, label: m.model_name || m.model_id })))

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

const statusColor: Record<string, string> = {
	queued: 'badge-gray',
	in_progress: 'badge-warning',
	completed: 'badge-success',
	failed: 'badge-danger',
}

function applyResolutionPreset(val: string) {
	resolution.value = val
}

async function submitTask() {
	if (!prompt.value.trim() || !selectedModel.value) return
	submitting.value = true
	currentTask.value = null
	stopPolling()
	releaseVideo()

	try {
		const api = createPlaygroundApi(props.apiKey)
		// OpenAI Videos 协议：size 承载分辨率（像素串或命名档位如 768P/2K），seconds 承载时长
		const body: Record<string, any> = {
			model: selectedModel.value,
			prompt: prompt.value,
		}
		if (resolution.value) {
			body.size = resolution.value
		}
		if (duration.value) {
			body.seconds = String(duration.value)
		}

		const res = await api.post('/v1/videos', body, { timeout: 60_000 })
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
	<div class="grid grid-cols-1 lg:grid-cols-5 gap-6">
		<div class="lg:col-span-2">
			<div class="card sticky top-6">
				<div class="card-header">
					<h3 class="text-sm font-semibold text-gray-900">视频生成参数</h3>
				</div>
				<div class="card-body space-y-4">
					<div>
						<label class="input-label">模型</label>
						<BaseSelect v-model="selectedModel" :options="modelOptions" />
					</div>
					<div>
						<label class="input-label">提示词</label>
						<n-input v-model:value="prompt" type="textarea" :rows="4" placeholder="描述你想生成的视频..." />
					</div>
					<div>
						<label class="input-label">分辨率</label>
						<n-input v-model:value="resolution" placeholder="如 1280x720" />
						<div class="flex flex-wrap gap-1.5 mt-2">
							<button
								v-for="p in resolutionPresets"
								:key="p.value"
								class="text-xs px-2 py-1 rounded-lg border transition-colors duration-150 cursor-pointer"
								:class="resolution === p.value
									? 'bg-primary-50 border-primary-300 text-primary-700'
									: 'bg-white border-gray-200 text-gray-600 hover:border-primary-200 hover:text-primary-600'"
								@click="applyResolutionPreset(p.value)"
							>
								{{ p.label }}
							</button>
						</div>
					</div>
					<div>
						<label class="input-label">时长（{{ duration }} 秒）</label>
						<!-- min=4 覆盖 MiniMax-H3 的 4 秒档；其他模型档位由上游校验 -->
						<n-input-number v-model:value="duration" :min="4" :max="15" :step="1" class="w-full" />
					</div>
					<button class="btn btn-primary w-full" :disabled="submitting || polling || !prompt.trim()" @click="submitTask">
						{{ submitting ? '提交中...' : polling ? '生成中...' : '生成视频' }}
					</button>
					<button v-if="currentTask" class="btn btn-secondary w-full" @click="resetTask">
						重新生成
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
					<div v-if="!currentTask && !submitting" class="empty-state">
						<div class="empty-state-icon"><Icon name="bookOpen" size="xl" /></div>
						<h3 class="empty-state-title">等待生成</h3>
						<p class="empty-state-description">输入提示词并点击生成</p>
					</div>

					<!-- 提交中 -->
					<div v-if="submitting" class="flex items-center justify-center py-12">
						<div class="spinner h-8 w-8 text-primary-600"></div>
						<span class="ml-3 text-sm text-gray-500">提交视频生成任务中...</span>
					</div>

					<!-- 任务状态 -->
					<div v-if="currentTask" class="space-y-4">
						<!-- 状态信息 -->
						<div class="flex items-center gap-3">
							<span class="badge" :class="statusColor[currentTask.status] || 'badge-gray'">
								{{ statusLabel[currentTask.status] || currentTask.status }}
							</span>
							<span v-if="currentTask.progress" class="text-xs text-gray-500">进度: {{ currentTask.progress }}</span>
							<span v-if="polling" class="text-xs text-primary-600 flex items-center gap-1">
								<div class="spinner h-3 w-3"></div>
								轮询中
							</span>
						</div>

						<!-- 进度条 -->
						<div v-if="currentTask.status === 'in_progress' || currentTask.status === 'queued'" class="w-full bg-gray-100 rounded-full h-2 overflow-hidden">
							<div
								class="h-full rounded-full bg-primary-500 transition-all duration-500"
								:style="{ width: currentTask.progress || '10%' }"
							/>
						</div>

						<!-- 视频结果 -->
						<div v-if="currentTask.status === 'completed' && videoUrl" class="space-y-3">
							<video controls class="w-full" :src="videoUrl" />
							<button class="btn btn-secondary btn-sm" @click="downloadVideo">
								<Icon name="arrowDown" size="sm" />
								下载视频
							</button>
						</div>
						<!-- 成品加载中（content 端点拉流转 blob） -->
						<div v-else-if="currentTask.status === 'completed'" class="flex items-center justify-center py-10">
							<div class="spinner h-6 w-6 text-primary-600"></div>
							<span class="ml-3 text-sm text-gray-500">视频加载中...</span>
						</div>

						<!-- 错误信息 -->
						<div v-if="currentTask.status === 'failed'" class="rounded-xl border border-red-200 bg-red-50 p-4">
							<div class="flex items-center gap-2 text-red-700">
								<Icon name="xCircle" size="sm" />
								<span class="text-sm font-medium">生成失败</span>
							</div>
							<p v-if="currentTask.error" class="mt-2 text-sm text-red-600">{{ currentTask.error }}</p>
						</div>

						<!-- 任务详情 -->
						<div class="text-xs text-gray-500 space-y-1 pt-3 border-t border-gray-100">
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

</template>
