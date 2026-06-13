<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { createPlaygroundApi } from '@/utils/playgroundApi'
import { calculateCost } from './calculateCost'
import ParamRefModal from './ParamRefModal.vue'
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
const modelOptions = computed(() => props.models.map(m => ({ value: m.model_id, label: m.model_name || m.model_id })))

// ── 自定义参数系统 ──

interface CustomParam {
	id: number
	key: string
	value: string
}

const customParams = ref<CustomParam[]>([])
let nextParamId = 1
const showParamRef = ref(false)

// 模型预设模板
const MODEL_PRESETS: Record<string, Record<string, string>> = {
	'gpt-image-1': {
		size: '1024x1024',
		quality: 'auto',
		response_format: 'b64_json',
		output_format: 'png',
	},
	'dall-e-3': {
		size: '1024x1024',
		quality: 'standard',
		style: 'vivid',
		response_format: 'url',
	},
	'dall-e-2': {
		size: '1024x1024',
		response_format: 'url',
	},
}

// 当前模型的预设（支持子串匹配，如 openai/gpt-image-1）
const currentPreset = computed(() => {
	const id = selectedModel.value
	if (MODEL_PRESETS[id]) return MODEL_PRESETS[id]
	for (const key of Object.keys(MODEL_PRESETS)) {
		if (id.includes(key)) return MODEL_PRESETS[key]
	}
	return null
})

// 预设标签名（用于按钮显示）
const currentPresetLabel = computed(() => {
	const id = selectedModel.value
	for (const key of Object.keys(MODEL_PRESETS)) {
		if (id === key || id.includes(key)) {
			// 美化显示：gpt-image-1 → GPT Image 1
			return key.replace(/-/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
		}
	}
	return ''
})

// 有效参数数量
const validParamsCount = computed(() =>
	customParams.value.filter(p => p.key.trim() && p.value.trim()).length,
)

function addParam(key = '', value = '') {
	customParams.value.push({ id: nextParamId++, key, value })
}

function removeParam(id: number) {
	const idx = customParams.value.findIndex(p => p.id === id)
	if (idx !== -1) customParams.value.splice(idx, 1)
}

function applyPreset() {
	const preset = currentPreset.value
	if (!preset) return
	customParams.value = []
	for (const [key, value] of Object.entries(preset)) {
		addParam(key, value)
	}
}

// 智能类型推断：字符串 → boolean / number / string
function parseValue(raw: string): string | number | boolean {
	if (raw.toLowerCase() === 'true') return true
	if (raw.toLowerCase() === 'false') return false
	if (/^-?\d+(\.\d+)?$/.test(raw)) {
		const num = Number(raw)
		if (!isNaN(num) && isFinite(num)) return num
	}
	return raw
}

// 切换模型时自动应用预设
watch(selectedModel, () => {
	const preset = currentPreset.value
	if (preset) {
		applyPreset()
	}
}, { immediate: true })

// ── 图片生成 ──

interface ImageResult { b64_json?: string; url?: string; revised_prompt?: string }
const images = ref<ImageResult[]>([])
const tokenUsage = reactive({ promptTokens: 0, totalTokens: 0, cost: '' })

async function generate() {
	if (!prompt.value.trim() || !selectedModel.value) return
	sending.value = true
	images.value = []

	try {
		const api = createPlaygroundApi(props.apiKey)

		// 组装请求体：固定字段 + 自定义参数
		const body: Record<string, any> = {
			model: selectedModel.value,
			prompt: prompt.value,
		}
		for (const param of customParams.value) {
			const key = param.key.trim()
			const val = param.value.trim()
			if (key && val) {
				body[key] = parseValue(val)
			}
		}

		const res = await api.post('/v1/images/generations', body, { timeout: 300_000 })

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
					<!-- 模型 -->
					<div>
						<label class="input-label">模型</label>
						<BaseSelect v-model="selectedModel" :options="modelOptions" />
					</div>

					<!-- 提示词 -->
					<div>
						<label class="input-label">提示词</label>
						<textarea v-model="prompt" class="input" rows="4" placeholder="描述你想生成的图片..." />
					</div>

					<!-- 自定义参数 -->
					<div class="border-t border-gray-100 pt-4">
						<div class="flex items-center justify-between mb-3">
							<div class="flex items-center gap-2">
								<h4 class="text-sm font-semibold text-gray-900">自定义参数</h4>
								<span v-if="validParamsCount > 0" class="badge badge-primary">{{ validParamsCount }}</span>
							</div>
							<div class="flex items-center gap-1.5">
								<button
									class="text-xs px-2.5 py-1.5 rounded-lg text-gray-600 hover:border-primary-300 hover:text-primary-600 transition-colors duration-150 cursor-pointer flex items-center gap-1.5 border border-gray-200"
									@click="showParamRef = true"
								>
									<Icon name="bookOpen" size="xs" />
									参数参考
								</button>
								<button
									v-if="currentPreset && customParams.length > 0"
									class="text-xs px-2 py-1 rounded-lg text-primary-600 hover:bg-primary-50 transition-colors duration-150 cursor-pointer"
									@click="applyPreset"
								>
									重置预设
								</button>
								<button
									class="btn btn-sm btn-secondary"
									@click="addParam()"
								>
									<Icon name="plus" size="xs" />
									添加
								</button>
							</div>
						</div>

						<!-- 空状态 -->
						<div
							v-if="customParams.length === 0"
							class="rounded-xl border-2 border-dashed border-gray-200 py-6 text-center cursor-pointer hover:border-primary-300 hover:bg-primary-50/30 transition-colors duration-200"
							@click="currentPreset ? applyPreset() : addParam()"
						>
							<Icon name="plus" size="sm" class="text-gray-300 mx-auto mb-2" />
							<p class="text-xs text-gray-400">
								{{ currentPreset ? '点击应用 ' + currentPresetLabel + ' 预设' : '点击添加自定义参数' }}
							</p>
						</div>

						<!-- 参数行 -->
						<div v-else class="space-y-2">
							<div
								v-for="param in customParams"
								:key="param.id"
								class="flex items-center gap-2 group"
							>
								<input
									v-model="param.key"
									class="input flex-1"
									placeholder="参数名"
								/>
								<input
									v-model="param.value"
									class="input flex-1"
									placeholder="值"
								/>
								<button
									class="h-9 w-9 rounded-xl flex items-center justify-center text-gray-300 hover:text-red-500 hover:bg-red-50 transition-colors duration-150 flex-shrink-0 cursor-pointer"
									@click="removeParam(param.id)"
								>
									<Icon name="x" size="sm" />
								</button>
							</div>
							<button
								class="w-full text-xs py-1.5 rounded-lg border border-dashed border-gray-200 text-gray-400 hover:border-primary-300 hover:text-primary-600 transition-colors duration-150 cursor-pointer"
								@click="addParam()"
							>
								+ 添加参数
							</button>
						</div>
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

	<!-- 参数参考弹窗 -->
	<ParamRefModal :show="showParamRef" mode="image" @close="showParamRef = false" />
</template>
