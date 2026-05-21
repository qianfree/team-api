<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import request from '@/utils/request'
import Icon from '@/components/common/Icon.vue'
import BaseSelect from '../../../components/common/BaseSelect.vue'

interface ModelItem { model_id: string; model_name: string; category: string }
const props = defineProps<{ models: ModelItem[] }>()

const sending = ref(false)
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
const tokenUsage = reactive({ promptTokens: 0, completionTokens: 0, totalTokens: 0, cost: '' })

async function synthesize() {
	if (!inputText.value.trim() || !selectedModel.value) return
	sending.value = true
	audioBase64.value = ''

	try {
		const res = await request.post('/tenant/playground/audio/tts', {
			model: selectedModel.value,
			input: inputText.value,
			voice: voice.value,
			response_format: responseFormat.value,
		})
		if (res.data?.code === 0) {
			const data = res.data.data
			audioBase64.value = data.audio_base64 || ''
			contentType.value = data.content_type || 'audio/mpeg'
			tokenUsage.promptTokens = data.prompt_tokens || 0
			tokenUsage.completionTokens = data.completion_tokens || 0
			tokenUsage.totalTokens = data.total_tokens || 0
			tokenUsage.cost = data.estimated_cost || ''
		}
	} catch (e) {
		console.error(e)
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
					<div v-if="!audioBase64 && !sending" class="empty-state">
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
						<div class="text-xs text-gray-500 flex items-center justify-between">
							<span>Tokens: {{ tokenUsage.promptTokens }} / {{ tokenUsage.totalTokens }}</span>
							<span v-if="tokenUsage.cost" class="font-medium text-amber-600">{{ tokenUsage.cost }}</span>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>
