<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { createPlaygroundApi } from '@/utils/playgroundApi'
import { calculateCost } from './calculateCost'
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
}
const props = defineProps<{ models: ModelItem[]; apiKey: string }>()

const sending = ref(false)
const selectedModel = ref(props.models[0]?.model_id || '')
const selectedModelItem = computed(() =>
	props.models.find(m => m.model_id === selectedModel.value),
)
const prompt = ref('')
const size = ref('1920x1080')
const quality = ref('standard')
const modelOptions = computed(() => props.models.map(m => ({ value: m.model_id, label: m.model_name || m.model_id })))

const sizePresets = [
	{ value: '1280x720', label: '720p' },
	{ value: '1920x1080', label: '1080p' },
	{ value: '2560x1440', label: '2K' },
	{ value: '3840x2160', label: '4K' },
]

function applySizePreset(val: string) {
	size.value = val
}

interface ImageResult { b64_json?: string; url?: string; revised_prompt?: string }
const images = ref<ImageResult[]>([])
const tokenUsage = reactive({ promptTokens: 0, totalTokens: 0, cost: '' })

async function generate() {
	if (!prompt.value.trim() || !selectedModel.value) return
	sending.value = true
	images.value = []

	try {
		const api = createPlaygroundApi(props.apiKey)
		const res = await api.post('/v1/images/generations', {
			model: selectedModel.value,
			prompt: prompt.value,
			size: size.value,
			quality: quality.value,
		}, { timeout: 180_000 })

		const data = res.data
		images.value = data.data || []
		const usage = data.usage || {}
		tokenUsage.promptTokens = usage.prompt_tokens || 0
		tokenUsage.totalTokens = usage.total_tokens || 0
		tokenUsage.cost = calculateCost(selectedModelItem.value, usage) || ''
	} catch (e) {
		console.error(e)
	} finally {
		sending.value = false
	}
}

function downloadImage(img: ImageResult, idx: number) {
	let src = ''
	let filename = `image_${idx + 1}.png`
	if (img.b64_json) {
		src = 'data:image/png;base64,' + img.b64_json
	} else if (img.url) {
		src = img.url
		filename = `image_${idx + 1}.png`
	} else {
		return
	}
	const a = document.createElement('a')
	a.href = src
	a.download = filename
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
					<h3 class="text-sm font-semibold text-gray-900">图片生成参数</h3>
				</div>
				<div class="card-body space-y-4">
					<div>
						<label class="input-label">模型</label>
						<BaseSelect v-model="selectedModel" :options="modelOptions" />
					</div>
					<div>
						<label class="input-label">提示词</label>
						<textarea v-model="prompt" class="input" rows="4" placeholder="描述你想生成的图片..." />
					</div>
					<div>
						<label class="input-label">尺寸</label>
						<input v-model="size" class="input" placeholder="如 1920x1080" />
						<div class="flex flex-wrap gap-1.5 mt-2">
							<button
								v-for="p in sizePresets"
								:key="p.value"
								class="text-xs px-2 py-1 rounded-lg border transition-colors duration-150 cursor-pointer"
								:class="size === p.value
									? 'bg-primary-50 border-primary-300 text-primary-700'
									: 'bg-white border-gray-200 text-gray-600 hover:border-primary-200 hover:text-primary-600'"
								@click="applySizePreset(p.value)"
							>
								{{ p.label }}
							</button>
						</div>
					</div>
					<div>
						<label class="input-label">质量</label>
						<BaseSelect v-model="quality" :options="[{value:'standard',label:'标准'},{value:'hd',label:'高清'}]" />
					</div>
					<button class="btn btn-primary w-full" :disabled="sending || !prompt.trim()" @click="generate">
						{{ sending ? '生成中...' : '生成图片' }}
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
					<div v-if="images.length === 0 && !sending" class="empty-state">
						<div class="empty-state-icon"><Icon name="bookOpen" size="xl" /></div>
						<h3 class="empty-state-title">等待生成</h3>
						<p class="empty-state-description">输入提示词并点击生成</p>
					</div>
					<div v-if="sending" class="flex items-center justify-center py-12">
						<div class="spinner h-8 w-8 text-primary-600"></div>
						<span class="ml-3 text-sm text-gray-500">图片生成中...</span>
					</div>
					<div v-if="images.length > 0" class="grid grid-cols-1 sm:grid-cols-2 gap-4">
						<div v-for="(img, idx) in images" :key="idx" class="rounded-xl overflow-hidden border border-gray-200 relative group">
							<img v-if="img.b64_json" :src="'data:image/png;base64,' + img.b64_json" class="w-full" alt="Generated image" />
							<img v-else-if="img.url" :src="img.url" class="w-full" alt="Generated image" />
							<button
								class="absolute top-2 right-2 btn btn-sm bg-white/90 backdrop-blur-sm border border-gray-200 shadow-sm opacity-0 group-hover:opacity-100 transition-opacity duration-200"
								@click="downloadImage(img, idx)"
							>
								<Icon name="arrowDown" size="sm" />
								下载
							</button>
							<p v-if="img.revised_prompt" class="text-xs text-gray-500 p-3">{{ img.revised_prompt }}</p>
						</div>
					</div>
					<div v-if="tokenUsage.totalTokens > 0" class="mt-4 pt-3 border-t border-gray-100 text-xs text-gray-500 flex items-center justify-between">
						<span>Tokens: {{ tokenUsage.promptTokens }} / {{ tokenUsage.totalTokens }}</span>
						<span v-if="tokenUsage.cost" class="font-medium text-amber-600">{{ tokenUsage.cost }}</span>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>
