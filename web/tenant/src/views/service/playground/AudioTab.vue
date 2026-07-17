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

const sending = ref(false)
const errorMessage = ref('')
const selectedModel = ref(props.models[0]?.model_id || '')
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
	<div class="grid grid-cols-1 lg:grid-cols-4 gap-6">
		<div class="lg:col-span-1">
			<div class="card sticky top-6">
				<div class="card-header">
					<h3 class="text-sm font-semibold text-gray-900">语音合成参数</h3>
				</div>
				<div class="card-body space-y-4">
					<div>
						<label class="input-label">模型</label>
						<BaseSelect v-model="selectedModel" :options="modelOptions" />
					</div>
					<div>
						<label class="input-label">文本内容</label>
						<textarea v-model="inputText" class="input" rows="4" placeholder="输入要合成的文本..." />
					</div>
					<div>
						<label class="input-label">语音</label>
						<BaseSelect v-model="voice" :options="voiceSelectOptions" />
					</div>
					<div>
						<label class="input-label">格式</label>
						<BaseSelect v-model="responseFormat" :options="formatSelectOptions" />
					</div>
					<button class="btn btn-primary w-full" :disabled="sending || !inputText.trim()" @click="synthesize">
						{{ sending ? '合成中...' : '合成语音' }}
					</button>
				</div>
			</div>
		</div>

		<div class="lg:col-span-3">
			<div class="card" style="min-height: 400px">
				<div class="card-header">
					<h3 class="text-sm font-semibold text-gray-900">合成结果</h3>
				</div>
				<div class="card-body">
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

					<div v-if="!audioBase64 && !sending && !errorMessage" class="empty-state">
						<div class="empty-state-icon"><Icon name="bookOpen" size="xl" /></div>
						<h3 class="empty-state-title">等待合成</h3>
						<p class="empty-state-description">输入文本并点击合成</p>
					</div>
					<div v-if="sending" class="flex items-center justify-center py-12">
						<div class="spinner h-8 w-8 text-primary-600"></div>
						<span class="ml-3 text-sm text-gray-500">语音合成中...</span>
					</div>
					<div v-if="audioBase64" class="space-y-4">
						<audio controls class="w-full" :src="'data:' + contentType + ';base64,' + audioBase64" />
					</div>
				</div>
			</div>
		</div>
	</div>
</template>
