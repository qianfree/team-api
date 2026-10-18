<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { NInput } from 'naive-ui'
import { createPlaygroundApi } from '@/utils/playgroundApi'
import Icon from '@/components/common/Icon.vue'
import BaseSelect from '../../../components/common/BaseSelect.vue'

interface ModelItem {
	model_id: string
	model_name: string
	category: string
}
const props = defineProps<{ models: ModelItem[]; apiKey: string }>()

// 转写文件大小上限（与 OpenAI whisper 一致），超限直接前端提示
const maxFileBytes = 25 * 1024 * 1024

const sending = ref(false)
const errorMessage = ref('')
const selectedModel = ref(props.models[0]?.model_id || '')
watch(
	() => props.models,
	models => {
		if (!selectedModel.value || !models.some(m => m.model_id === selectedModel.value)) {
			selectedModel.value = models[0]?.model_id || ''
		}
	},
)

const language = ref('')
const responseFormat = ref('json')
const formatOptions = [
	{ value: 'json', label: 'json（纯文本）' },
	{ value: 'verbose_json', label: 'verbose_json（含时长与分段）' },
	{ value: 'text', label: 'text' },
	{ value: 'srt', label: 'srt（字幕）' },
	{ value: 'vtt', label: 'vtt（字幕）' },
]
const modelOptions = computed(() => props.models.map(m => ({ value: m.model_id, label: m.model_name || m.model_id })))

const fileInput = ref<HTMLInputElement | null>(null)
const audioFile = ref<File | null>(null)
const fileError = ref('')
// 上传文件的本地预览地址（objectURL，替换/组件销毁时回收）
const previewUrl = ref('')

interface TranscriptionSegment {
	id?: number
	begin?: number
	end?: number
	text?: string
}
const resultText = ref('')
const resultRaw = ref('')
const resultDuration = ref<number | null>(null)
const resultLanguage = ref('')
const resultSegments = ref<TranscriptionSegment[]>([])

const selectedModelName = computed(() => {
	const m = props.models.find(x => x.model_id === selectedModel.value)
	return m?.model_name || selectedModel.value
})

const resultStatus = computed<{ label: string; cls: string } | null>(() => {
	if (errorMessage.value) return { label: '失败', cls: 'badge-danger' }
	if (sending.value) return { label: '转写中', cls: 'badge-primary' }
	if (resultText.value || resultRaw.value) return { label: '已完成', cls: 'badge-success' }
	return null
})

function formatSize(bytes: number): string {
	if (bytes >= 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
	return Math.max(1, Math.round(bytes / 1024)) + ' KB'
}

// 秒 → mm:ss，供分段列表展示
function formatTime(seconds?: number): string {
	if (seconds == null || Number.isNaN(seconds)) return '-'
	const m = Math.floor(seconds / 60)
	const s = Math.floor(seconds % 60)
	return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

function pickFile() {
	fileInput.value?.click()
}

function onFileChange(e: Event) {
	const input = e.target as HTMLInputElement
	fileError.value = ''
	const file = input.files?.[0]
	input.value = '' // 允许重复选择同一文件
	if (!file) return
	if (!file.type.startsWith('audio/') && !/\.(mp3|mp4|mpeg|mpga|m4a|wav|webm|amr|aac|opus|flac|ogg)$/i.test(file.name)) {
		fileError.value = '请选择音频文件（mp3/wav/m4a/webm/amr/aac/opus/flac 等）'
		return
	}
	if (file.size > maxFileBytes) {
		fileError.value = `文件过大（${formatSize(file.size)}），单个音频不超过 25 MB`
		return
	}
	setFile(file)
}

function setFile(file: File) {
	if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
	audioFile.value = file
	previewUrl.value = URL.createObjectURL(file)
}

function clearFile() {
	if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
	previewUrl.value = ''
	audioFile.value = null
	fileError.value = ''
}

onBeforeUnmount(() => {
	if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
})

async function transcribe() {
	const file = audioFile.value
	if (!file || !selectedModel.value) return
	sending.value = true
	errorMessage.value = ''
	resultText.value = ''
	resultRaw.value = ''
	resultDuration.value = null
	resultLanguage.value = ''
	resultSegments.value = []

	try {
		const api = createPlaygroundApi(props.apiKey)
		// Whisper 风格 multipart 表单：axios 检测到 FormData 会交由浏览器自动
		// 生成含 boundary 的 Content-Type（实例默认的 JSON 头会被移除）
		const form = new FormData()
		form.append('file', file)
		form.append('model', selectedModel.value)
		form.append('response_format', responseFormat.value)
		if (language.value.trim()) form.append('language', language.value.trim())

		const res = await api.post('/v1/audio/transcriptions', form, { timeout: 180_000 })

		// json/verbose_json 由 axios 解析为对象；text/srt/vtt 保持原始字符串
		if (typeof res.data === 'string') {
			resultRaw.value = res.data
		} else if (res.data && typeof res.data === 'object') {
			resultText.value = res.data.text || ''
			resultDuration.value = typeof res.data.duration === 'number' ? res.data.duration : null
			resultLanguage.value = res.data.language || ''
			resultSegments.value = Array.isArray(res.data.segments) ? res.data.segments : []
		} else {
			resultRaw.value = String(res.data ?? '')
		}
	} catch (e) {
		console.error(e)
		errorMessage.value = (e as any)?.message || '请求失败，请重试'
	} finally {
		sending.value = false
	}
}
</script>

<template>
	<!-- 与其余 Tab 同一套等高布局：左参数栏内部滚动，主操作常驻底部，右结果区撑满 -->
	<div class="flex flex-col lg:h-[calc(100vh-15rem)] lg:overflow-hidden">
		<div class="grid grid-cols-1 gap-4 lg:grid-cols-[22rem_minmax(0,1fr)] lg:grid-rows-[minmax(0,1fr)] lg:min-h-0 lg:flex-1">
			<!-- Left: 参数 -->
			<div class="flex lg:min-h-0">
				<div class="card flex w-full flex-col overflow-hidden lg:h-full">
					<div class="flex shrink-0 items-center gap-2 border-b border-gray-100 px-4 py-3">
						<Icon name="document" size="sm" class="text-primary-500" />
						<h3 class="text-sm font-semibold text-gray-900">语音转写参数</h3>
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

						<!-- 音频文件 -->
						<div class="border-t border-gray-100 pt-4">
							<label class="mb-2 block text-xs font-semibold text-gray-500">音频文件</label>
							<input
								ref="fileInput"
								type="file"
								accept="audio/*,.mp3,.mp4,.mpeg,.mpga,.m4a,.wav,.webm,.amr,.aac,.opus,.flac,.ogg"
								class="hidden"
								@change="onFileChange"
							/>
							<!-- 已选文件：试听 + 元信息 + 移除 -->
							<div v-if="audioFile" class="rounded-xl border border-gray-200 bg-gray-50 p-3">
								<div class="flex items-center justify-between gap-2">
									<div class="min-w-0">
										<p class="truncate text-sm font-medium text-gray-700" :title="audioFile.name">{{ audioFile.name }}</p>
										<p class="mt-0.5 text-xs text-gray-400">{{ formatSize(audioFile.size) }}</p>
									</div>
									<button class="btn btn-ghost shrink-0 !px-2 !py-1 text-xs text-red-600" @click="clearFile">
										<Icon name="trash" size="xs" />
										移除
									</button>
								</div>
								<audio v-if="previewUrl" controls :src="previewUrl" class="mt-2 w-full" />
							</div>
							<button v-else class="w-full rounded-xl border border-dashed border-gray-300 px-4 py-6 text-sm text-gray-500 transition hover:border-primary-400 hover:text-primary-600" @click="pickFile">
								<Icon name="musicalNote" size="sm" class="mr-1" />
								点击选择音频文件（≤25 MB）
							</button>
							<p v-if="fileError" class="mt-1.5 text-[11px] text-red-500">{{ fileError }}</p>
						</div>

						<!-- 转写参数 -->
						<div class="space-y-4 border-t border-gray-100 pt-4">
							<div>
								<label class="mb-2 block text-xs font-semibold text-gray-500">语言（可选）</label>
								<n-input v-model:value="language" placeholder="如 zh、en；多语种音频建议留空" />
							</div>
							<div>
								<label class="mb-2 block text-xs font-semibold text-gray-500">输出格式</label>
								<BaseSelect v-model="responseFormat" :options="formatOptions" />
							</div>
						</div>
					</div>

					<!-- 主操作常驻卡片底部 -->
					<div class="shrink-0 border-t border-gray-100 px-4 py-3">
						<button class="btn btn-primary w-full" :disabled="sending || !audioFile || !!fileError" @click="transcribe">
							<Icon name="play" size="sm" />
							{{ sending ? '转写中...' : '开始转写' }}
						</button>
					</div>
				</div>
			</div>

			<!-- Right: 结果 -->
			<div class="lg:min-h-0">
				<div class="card flex min-h-[420px] flex-col overflow-hidden lg:h-full">
					<div class="flex shrink-0 items-center justify-between gap-3 border-b border-gray-100 px-5 py-3">
						<div class="flex items-center gap-2">
							<Icon name="document" size="sm" class="text-primary-500" />
							<h3 class="text-sm font-semibold text-gray-900">转写结果</h3>
						</div>
						<span v-if="resultStatus" class="badge" :class="resultStatus.cls">{{ resultStatus.label }}</span>
					</div>

					<div class="min-h-0 flex-1 overflow-y-auto p-5">
						<!-- 错误提示 -->
						<div v-if="errorMessage" class="mb-4 rounded-xl border border-red-200 bg-red-50 p-4">
							<div class="flex items-center gap-2 text-red-700">
								<Icon name="xCircle" size="sm" />
								<span class="text-sm font-medium">转写失败</span>
							</div>
							<p class="mt-2 text-sm text-red-600">{{ errorMessage }}</p>
						</div>

						<!-- 空态 -->
						<div v-if="!resultText && !resultRaw && !sending && !errorMessage" class="flex h-full min-h-[320px] flex-col items-center justify-center px-4 text-center">
							<div class="mb-5 flex h-16 w-16 items-center justify-center rounded-3xl bg-gradient-to-br from-cyan-400 via-primary-500 to-emerald-500 text-white shadow-glow">
								<Icon name="document" size="lg" />
							</div>
							<h3 class="text-lg font-semibold text-gray-900">等待转写</h3>
							<p class="mt-1.5 max-w-sm text-sm text-gray-500">
								<template v-if="selectedModel">
									当前模型 <span class="font-medium text-gray-700">{{ selectedModelName }}</span>，选择音频文件后点击「开始转写」
								</template>
								<template v-else>请先在左侧选择一个转写模型</template>
							</p>
							<p class="mt-6 inline-flex items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-[11px] text-amber-700">
								<Icon name="exclamationTriangle" size="xs" />
								转写将真实调用模型并产生费用
							</p>
						</div>

						<!-- 转写中 -->
						<div v-if="sending" class="flex h-full min-h-[320px] flex-col items-center justify-center gap-3">
							<div class="spinner h-8 w-8 text-primary-600"></div>
							<span class="text-sm text-gray-500">语音转写中...</span>
						</div>

						<!-- 结果：结构化（json / verbose_json） -->
						<div v-if="resultText || resultSegments.length" class="space-y-3">
							<div class="flex flex-wrap items-center gap-2">
								<span class="badge badge-success">转写完成</span>
								<span v-if="resultDuration != null" class="badge">时长 {{ formatTime(resultDuration) }}</span>
								<span v-if="resultLanguage" class="badge">语言 {{ resultLanguage }}</span>
							</div>
							<div class="rounded-xl border border-gray-200 bg-gray-50 p-4 text-sm leading-relaxed text-gray-800">
								{{ resultText }}
							</div>
							<!-- verbose_json 分段（字级时间戳依模型支持返回） -->
							<div v-if="resultSegments.length" class="rounded-xl border border-gray-200">
								<div class="border-b border-gray-100 px-4 py-2 text-xs font-semibold text-gray-500">分段（{{ resultSegments.length }} 段）</div>
								<ul class="max-h-72 divide-y divide-gray-100 overflow-y-auto">
									<li v-for="(seg, i) in resultSegments" :key="seg.id ?? i" class="flex gap-3 px-4 py-2 text-sm">
										<span class="shrink-0 font-mono text-xs text-gray-400">{{ formatTime(seg.begin) }} - {{ formatTime(seg.end) }}</span>
										<span class="min-w-0 text-gray-700">{{ seg.text }}</span>
									</li>
								</ul>
							</div>
						</div>

						<!-- 结果：原始文本（text / srt / vtt） -->
						<div v-else-if="resultRaw" class="space-y-3">
							<span class="badge badge-success">转写完成</span>
							<pre class="overflow-x-auto whitespace-pre-wrap rounded-xl border border-gray-200 bg-gray-50 p-4 text-sm leading-relaxed text-gray-800">{{ resultRaw }}</pre>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>
