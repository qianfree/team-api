<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { NSelect } from 'naive-ui'
import request from '@/utils/request'
import ChatTab from './playground/ChatTab.vue'
import ImageTab from './playground/ImageTab.vue'
import VideoTab from './playground/VideoTab.vue'
import AudioTab from './playground/AudioTab.vue'
import EmbeddingTab from './playground/EmbeddingTab.vue'
import RerankTab from './playground/RerankTab.vue'
import { usePlaygroundApiKey } from './playground/usePlaygroundApiKey'
import Icon from '@/components/common/Icon.vue'

interface ModelItem {
	model_id: string
	model_name: string
	category: string
	billing_mode?: string | null
	per_request_price?: number | null
	input_price?: number | null
	output_price?: number | null
	// 图片模型异步端点是否可用（真异步厂商，或同步厂商且后台「同步图片异步化」开启）
	async_image?: boolean
	// 图片模型同步端点是否可用（「仅异步」厂商为 false）
	image_sync_supported?: boolean
}

const tabs = [
	{ key: 'chat', label: '对话' },
	{ key: 'image', label: '图像' },
	{ key: 'video', label: '视频' },
	{ key: 'audio', label: '语音' },
	{ key: 'embedding', label: '嵌入' },
	{ key: 'rerank', label: '重排' },
]

const activeTab = ref('chat')
const allModels = ref<ModelItem[]>([])

const { apiKeys, selectedKeyId, revealedKey, loading: keyLoading, error: keyError, selectKey } = usePlaygroundApiKey()

const modelsByCategory = (category: string) =>
	computed(() => allModels.value.filter(m => m.category === category))

const modelsLoading = ref(false)
let loadingTimer: number | null = null
let loadSeq = 0

// 按右上角选中的 API Key 拉取其可用模型，使左侧模型列表跟随 Key 切换。
// 切换期间保留旧列表避免空白闪烁；加载提示延迟 200ms 才出现，请求更快时
// 完全不显示，杜绝 spinner 瞬闪；loadSeq 丢弃过期响应，防止快速切换 Key 时旧结果覆盖。
async function loadModels() {
	const seq = ++loadSeq
	if (loadingTimer) { clearTimeout(loadingTimer); loadingTimer = null }
	if (!selectedKeyId.value) {
		allModels.value = []
		modelsLoading.value = false
		return
	}
	loadingTimer = window.setTimeout(() => { modelsLoading.value = true }, 200)
	try {
		const res = await request.get('/tenant/models', {
			params: { api_key_id: selectedKeyId.value },
		})
		if (seq !== loadSeq) return
		allModels.value = res.data?.code === 0 ? (res.data.data.list || []) : []
	} catch (e) {
		if (seq !== loadSeq) return
		console.error(e)
		allModels.value = []
	} finally {
		if (seq !== loadSeq) return
		if (loadingTimer) { clearTimeout(loadingTimer); loadingTimer = null }
		modelsLoading.value = false
	}
}

watch(selectedKeyId, loadModels, { immediate: true })
</script>

<template>
	<div>
		<!-- 紧凑工具栏：左侧模型类型标签 + 右侧 API Key 单行收纳 -->
		<div class="card mb-3">
			<div class="flex flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3 sm:px-5">
				<!-- 左侧：模型类型标签 -->
				<div class="tabs flex-wrap">
					<button v-for="tab in tabs" :key="tab.key"
						class="tab"
						:class="{ 'tab-active': activeTab === tab.key }"
						@click="activeTab = tab.key">
						{{ tab.label }}
					</button>
				</div>

				<!-- 模型加载指示内联在工具栏中，避免独立行造成的布局跳动 -->
				<transition name="fade">
					<div v-if="modelsLoading" class="flex shrink-0 items-center gap-1.5 text-xs text-gray-400">
						<div class="spinner h-3.5 w-3.5 text-primary-600"></div>
						加载模型...
					</div>
				</transition>

				<!-- 右侧：API Key 区域 -->
				<div class="ml-auto flex shrink-0 items-center gap-2">
					<!-- 加载中 -->
					<template v-if="keyLoading">
						<div class="spinner h-4 w-4 text-primary-600"></div>
						<span class="text-sm text-gray-500">加载 API Key...</span>
					</template>
					<!-- 错误提示 -->
					<div v-else-if="keyError" class="flex items-center gap-1.5 rounded-lg bg-amber-50 px-3 py-1.5 text-xs text-amber-700">
						<Icon name="exclamationTriangle" size="xs" />
						<span>{{ keyError }}</span>
						<router-link to="/tenant/api-keys" class="font-medium text-primary-600 underline hover:no-underline">去创建</router-link>
					</div>
					<!-- Key 选择器 -->
					<template v-else>
						<span class="text-xs font-medium text-gray-400">API Key</span>
						<div class="w-56">
							<n-select
								:value="selectedKeyId"
								:options="apiKeys.map(k => ({ label: `${k.name} (${k.key_prefix}...)`, value: k.id }))"
								placeholder="选择 API Key"
								@update:value="selectKey"
							/>
						</div>
					</template>
				</div>
			</div>
		</div>

		<template v-if="revealedKey">
			<ChatTab v-if="activeTab === 'chat'" :models="modelsByCategory('chat').value" :api-key="revealedKey" />
			<ImageTab v-if="activeTab === 'image'" :models="modelsByCategory('image').value" :api-key="revealedKey" />
			<VideoTab v-if="activeTab === 'video'" :models="modelsByCategory('video').value" :api-key="revealedKey" />
			<AudioTab v-if="activeTab === 'audio'" :models="modelsByCategory('audio').value" :api-key="revealedKey" />
			<EmbeddingTab v-if="activeTab === 'embedding'" :models="modelsByCategory('embedding').value" :api-key="revealedKey" />
			<RerankTab v-if="activeTab === 'rerank'" :models="modelsByCategory('rerank').value" :api-key="revealedKey" />
		</template>
	</div>
</template>
