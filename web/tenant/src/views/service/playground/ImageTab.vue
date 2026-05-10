<script setup lang="ts">
import { ref, reactive } from 'vue'
import request from '@/utils/request'
import Icon from '@/components/common/Icon.vue'

interface ModelItem { model_id: string; model_name: string; category: string }
const props = defineProps<{ models: ModelItem[] }>()

const sending = ref(false)
const selectedModel = ref(props.models[0]?.model_id || '')
const prompt = ref('')
const n = ref(1)
const size = ref('1024x1024')
const quality = ref('standard')

interface ImageResult { b64_json?: string; url?: string; revised_prompt?: string }
const images = ref<ImageResult[]>([])
const tokenUsage = reactive({ promptTokens: 0, totalTokens: 0, cost: '' })

async function generate() {
	if (!prompt.value.trim() || !selectedModel.value) return
	sending.value = true
	images.value = []

	try {
		const res = await request.post('/tenant/playground/image', {
			model: selectedModel.value,
			prompt: prompt.value,
			n: n.value,
			size: size.value,
			quality: quality.value,
			}, { timeout: 180_000 })
		if (res.data?.code === 0) {
			const data = res.data.data
			images.value = data.images || []
			tokenUsage.promptTokens = data.prompt_tokens || 0
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
					<h3 class="text-sm font-semibold text-gray-900">图片生成参数</h3>
				</div>
				<div class="card-body space-y-4">
					<div>
						<label class="input-label">模型</label>
						<select v-model="selectedModel" class="input">
							<option v-for="m in models" :key="m.model_id" :value="m.model_id">{{ m.model_name || m.model_id }}</option>
						</select>
					</div>
					<div>
						<label class="input-label">提示词</label>
						<textarea v-model="prompt" class="input" rows="4" placeholder="描述你想生成的图片..." />
					</div>
					<div>
						<label class="input-label">数量: {{ n }}</label>
						<input v-model.number="n" type="range" min="1" max="4" step="1" class="w-full" />
					</div>
					<div>
						<label class="input-label">尺寸</label>
						<select v-model="size" class="input">
							<option value="1024x1024">1024×1024</option>
							<option value="1792x1024">1792×1024</option>
							<option value="1024x1792">1024×1792</option>
							<option value="512x512">512×512</option>
						</select>
					</div>
					<div>
						<label class="input-label">质量</label>
						<select v-model="quality" class="input">
							<option value="standard">标准</option>
							<option value="hd">高清</option>
						</select>
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
						<div v-for="(img, idx) in images" :key="idx" class="rounded-xl overflow-hidden border border-gray-200">
							<img v-if="img.b64_json" :src="'data:image/png;base64,' + img.b64_json" class="w-full" alt="Generated image" />
							<img v-else-if="img.url" :src="img.url" class="w-full" alt="Generated image" />
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
