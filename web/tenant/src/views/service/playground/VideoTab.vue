<script setup lang="ts">
import { ref, computed } from 'vue'
import { createPlaygroundApi } from '@/utils/playgroundApi'
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
const prompt = ref('')
const resolution = ref('1280x720')
const duration = ref(5)

const modelOptions = computed(() => props.models.map(m => ({ value: m.model_id, label: m.model_name || m.model_id })))

const resolutionPresets = [
	{ value: '854x480', label: '480p 横' },
	{ value: '480x854', label: '480p 竖' },
	{ value: '1280x720', label: '720p 横' },
	{ value: '720x1280', label: '720p 竖' },
	{ value: '1920x1080', label: '1080p 横' },
	{ value: '1080x1920', label: '1080p 竖' },
]

interface TaskInfo {
	id: string
	status: string
	progress: string
	model: string
	url?: string
	error?: string
	createdAt: number
	completedAt?: number
}
const currentTask = ref<TaskInfo | null>(null)
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

function applyResolutionPreset(val: string) {
	resolution.value = val
}

async function submitTask() {
	if (!prompt.value.trim() || !selectedModel.value) return
	submitting.value = true
	currentTask.value = null
	stopPolling()

	try {
		const api = createPlaygroundApi(props.apiKey)
		const metadata: Record<string, any> = {}
		if (resolution.value) {
			metadata.resolution = resolution.value
		}
		if (duration.value) {
			metadata.duration = duration.value
		}
		const body: Record<string, any> = {
			model: selectedModel.value,
			prompt: prompt.value,
		}
		if (Object.keys(metadata).length > 0) {
			body.metadata = metadata
		}

		const res = await api.post('/v1/video/generations', body, { timeout: 60_000 })
		const data = res.data

		currentTask.value = {
			id: data.id,
			status: data.status || 'SUBMITTED',
			progress: '',
			model: data.model || selectedModel.value,
			createdAt: data.created_at || Math.floor(Date.now() / 1000),
		}

		// 开始轮询
		if (data.status !== 'SUCCESS' && data.status !== 'FAILURE') {
			startPolling()
		}
	} catch (e: any) {
		currentTask.value = {
			id: '',
			status: 'FAILURE',
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
	if (!polling.value || !currentTask.value?.id) return

	try {
		const api = createPlaygroundApi(props.apiKey)
		const res = await api.get(`/v1/video/generations/${currentTask.value.id}`)
		const data = res.data

		currentTask.value = {
			...currentTask.value,
			status: data.status,
			progress: data.progress || '',
			url: data.url || undefined,
			error: data.error || undefined,
			completedAt: data.completed_at || undefined,
		}

		if (data.status === 'SUCCESS' || data.status === 'FAILURE') {
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

function resetTask() {
	stopPolling()
	currentTask.value = null
}

function downloadVideo() {
	if (!currentTask.value?.url) return
	const a = document.createElement('a')
	a.href = currentTask.value.url
	a.download = `video_${currentTask.value.id || 'output'}.mp4`
	document.body.appendChild(a)
	a.click()
	document.body.removeChild(a)
}
</script>

<template>
	<div class="grid grid-cols-1 lg:grid-cols-4 gap-6">
		<div class="lg:col-span-1">
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
						<textarea v-model="prompt" class="input" rows="4" placeholder="描述你想生成的视频..." />
					</div>
					<div>
						<label class="input-label">分辨率</label>
						<input v-model="resolution" class="input" placeholder="如 1280x720" />
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
						<input v-model.number="duration" type="number" class="input" min="5" max="15" step="1" />
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
						<div v-if="currentTask.status === 'IN_PROGRESS' || currentTask.status === 'SUBMITTED'" class="w-full bg-gray-100 rounded-full h-2 overflow-hidden">
							<div
								class="h-full rounded-full bg-primary-500 transition-all duration-500"
								:style="{ width: currentTask.progress || '10%' }"
							/>
						</div>

						<!-- 视频结果 -->
						<div v-if="currentTask.status === 'SUCCESS' && currentTask.url" class="space-y-3">
							<video controls class="w-full" :src="currentTask.url" />
							<button class="btn btn-secondary btn-sm" @click="downloadVideo">
								<Icon name="arrowDown" size="sm" />
								下载视频
							</button>
						</div>

						<!-- 错误信息 -->
						<div v-if="currentTask.status === 'FAILURE'" class="rounded-xl border border-red-200 bg-red-50 p-4">
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
