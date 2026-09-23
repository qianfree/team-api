<script setup lang="ts">
import { ref, computed, watch } from 'vue'
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
const inputText = ref('')
const voice = ref('alloy')
const responseFormat = ref('mp3')

const voiceOptions = ['alloy', 'echo', 'fable', 'onyx', 'nova', 'shimmer']
const formatOptions = ['mp3', 'wav', 'opus', 'aac', 'flac']
const modelOptions = computed(() => props.models.map(m => ({ value: m.model_id, label: m.model_name || m.model_id })))
const voiceSelectOptions = computed(() => voiceOptions.map(v => ({ value: v, label: v })))
const formatSelectOptions = computed(() => formatOptions.map(f => ({ value: f, label: f.toUpperCase() })))

const audioBase64 = ref('')
const contentType = ref('')

// 空态里展示的模型名（列表由父组件异步加载，可能暂时查不到）
const selectedModelName = computed(() => {
	const m = props.models.find(x => x.model_id === selectedModel.value)
	return m?.model_name || selectedModel.value
})

// 结果卡右上角的状态徽标（无任务且无错误时不渲染）
const resultStatus = computed<{ label: string; cls: string } | null>(() => {
	if (errorMessage.value) return { label: '失败', cls: 'badge-danger' }
	if (sending.value) return { label: '合成中', cls: 'badge-primary' }
	if (audioBase64.value) return { label: '已完成', cls: 'badge-success' }
	return null
})

function arrayBufferToBase64(buffer: ArrayBuffer): string {
	const bytes = new Uint8Array(buffer)
	const chunkSize = 8192
	let binary = ''
	for (let i = 0; i < bytes.length; i += chunkSize) {
		const chunk = bytes.subarray(i, Math.min(i + chunkSize, bytes.length))
		binary += String.fromCharCode.apply(null, Array.from(chunk))
	}
	return btoa(binary)
}

async function synthesize() {
	if (!inputText.value.trim() || !selectedModel.value) return
	sending.value = true
	errorMessage.value = ''
	audioBase64.value = ''

	try {
		const api = createPlaygroundApi(props.apiKey)
		const res = await api.post('/v1/audio/speech', {
			model: selectedModel.value,
			input: inputText.value,
			voice: voice.value,
			response_format: responseFormat.value,
		}, { responseType: 'arraybuffer', timeout: 180_000 })

		// /v1/audio/speech 返回原始二进制音频数据
		const buffer = res.data as ArrayBuffer
		audioBase64.value = arrayBufferToBase64(buffer)
		contentType.value = String(res.headers['content-type'] || 'audio/mpeg')
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
						<Icon name="musicalNote" size="sm" class="text-primary-500" />
						<h3 class="text-sm font-semibold text-gray-900">语音合成参数</h3>
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

						<!-- 文本 -->
						<div class="border-t border-gray-100 pt-4">
							<label class="mb-2 block text-xs font-semibold text-gray-500">文本内容</label>
							<n-input v-model:value="inputText" type="textarea" :rows="5" placeholder="输入要合成的文本..." />
						</div>

						<!-- 输出参数 -->
						<div class="space-y-4 border-t border-gray-100 pt-4">
							<div>
								<label class="mb-2 block text-xs font-semibold text-gray-500">语音</label>
								<BaseSelect v-model="voice" :options="voiceSelectOptions" />
							</div>
							<div>
								<label class="mb-2 block text-xs font-semibold text-gray-500">格式</label>
								<BaseSelect v-model="responseFormat" :options="formatSelectOptions" />
							</div>
						</div>
					</div>

					<!-- 主操作常驻卡片底部 -->
					<div class="shrink-0 border-t border-gray-100 px-4 py-3">
						<button class="btn btn-primary w-full" :disabled="sending || !inputText.trim()" @click="synthesize">
							<Icon name="play" size="sm" />
							{{ sending ? '合成中...' : '合成语音' }}
						</button>
					</div>
				</div>
			</div>

			<!-- Right: 结果 -->
			<div class="lg:min-h-0">
				<div class="card flex min-h-[420px] flex-col overflow-hidden lg:h-full">
					<div class="flex shrink-0 items-center justify-between gap-3 border-b border-gray-100 px-5 py-3">
						<div class="flex items-center gap-2">
							<Icon name="musicalNote" size="sm" class="text-primary-500" />
							<h3 class="text-sm font-semibold text-gray-900">合成结果</h3>
						</div>
						<span v-if="resultStatus" class="badge" :class="resultStatus.cls">{{ resultStatus.label }}</span>
					</div>

					<div class="min-h-0 flex-1 overflow-y-auto p-5">
						<!-- 错误提示 -->
						<div
							v-if="errorMessage"
							class="mb-4 rounded-xl border border-red-200 bg-red-50 p-4"
						>
							<div class="flex items-center gap-2 text-red-700">
								<Icon name="xCircle" size="sm" />
								<span class="text-sm font-medium">合成失败</span>
							</div>
							<p class="mt-2 text-sm text-red-600">{{ errorMessage }}</p>
						</div>

						<!-- 空态：图标磁贴 + 说明，给首屏一个视觉锚点 -->
						<div v-if="!audioBase64 && !sending && !errorMessage" class="flex h-full min-h-[320px] flex-col items-center justify-center px-4 text-center">
							<div class="mb-5 flex h-16 w-16 items-center justify-center rounded-3xl bg-gradient-to-br from-cyan-400 via-primary-500 to-emerald-500 text-white shadow-glow">
								<Icon name="musicalNote" size="lg" />
							</div>
							<h3 class="text-lg font-semibold text-gray-900">等待合成</h3>
							<p class="mt-1.5 max-w-sm text-sm text-gray-500">
								<template v-if="selectedModel">
									当前模型 <span class="font-medium text-gray-700">{{ selectedModelName }}</span>，输入文本后点击「合成语音」
								</template>
								<template v-else>请先在左侧选择一个语音模型</template>
							</p>
							<p class="mt-6 inline-flex items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-[11px] text-amber-700">
								<Icon name="exclamationTriangle" size="xs" />
								合成将真实调用模型并产生费用
							</p>
						</div>

						<!-- 合成中 -->
						<div v-if="sending" class="flex h-full min-h-[320px] flex-col items-center justify-center gap-3">
							<div class="spinner h-8 w-8 text-primary-600"></div>
							<span class="text-sm text-gray-500">语音合成中...</span>
						</div>

						<!-- 结果：播放器放进卡片容器，与其它 Tab 的结果区块观感一致 -->
						<div v-if="audioBase64" class="space-y-3">
							<div class="flex items-center gap-2">
								<span class="badge badge-success">合成完成</span>
								<span class="text-xs text-gray-500">{{ contentType }}</span>
							</div>
							<div class="rounded-xl border border-gray-200 bg-gray-50 p-4">
								<audio controls class="w-full" :src="'data:' + contentType + ';base64,' + audioBase64" />
							</div>
							<p class="text-xs text-gray-400">音频以 base64 内嵌在响应中，可直接播放或右键另存</p>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>
