<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
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
	// 图片模型是否必须走异步端点（DashScope 等）
	async_image?: boolean
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

onMounted(async () => {
	try {
		const res = await request.get('/tenant/models')
		if (res.data?.code === 0) {
			allModels.value = res.data.data.list || []
		}
	} catch (e) { console.error(e) }
})
</script>

<template>
	<div>
		<div class="page-header">
			<h1 class="page-title">API Playground</h1>
			<p class="page-description">在线调试 AI 模型。不同大模型参数不一致，此处仅做简单功能演示，验证资源可用。请还用API调用获取最佳体验。</p>
		</div>

		<!-- Toolbar: 分类标签 + API Key 选择器 -->
		<div class="card mb-6">
			<div class="flex flex-col gap-3 px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
				<!-- 分类标签 -->
				<div class="tabs">
					<button v-for="tab in tabs" :key="tab.key"
						class="tab"
						:class="{ 'tab-active': activeTab === tab.key }"
						@click="activeTab = tab.key">
						{{ tab.label }}
					</button>
				</div>

				<!-- API Key 区域 -->
				<div class="flex shrink-0 items-center gap-2">
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
						<div class="relative">
							<select
								:value="selectedKeyId"
								class="appearance-none rounded-lg border border-gray-200 bg-white py-1.5 pl-3 pr-8 text-sm text-gray-700 transition-all hover:border-gray-300 focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/30"
								@change="selectKey(Number(($event.target as HTMLSelectElement).value))"
							>
								<option v-for="key in apiKeys" :key="key.id" :value="key.id">
									{{ key.name }} ({{ key.key_prefix }}...)
								</option>
							</select>
							<Icon name="chevronDown" size="xs" class="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-gray-400" />
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
