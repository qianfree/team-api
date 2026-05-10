<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import request from '@/utils/request'
import ChatTab from './playground/ChatTab.vue'
import ImageTab from './playground/ImageTab.vue'
import AudioTab from './playground/AudioTab.vue'
import EmbeddingTab from './playground/EmbeddingTab.vue'
import RerankTab from './playground/RerankTab.vue'

interface ModelItem { model_id: string; model_name: string; category: string }

const tabs = [
	{ key: 'chat', label: '对话' },
	{ key: 'image', label: '图像' },
	{ key: 'audio', label: '语音' },
	{ key: 'embedding', label: '嵌入' },
	{ key: 'rerank', label: '重排' },
]

const activeTab = ref('chat')
const allModels = ref<ModelItem[]>([])

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
			<p class="page-description">在线调试 AI 模型，测试不同参数组合</p>
		</div>

		<div class="tabs mb-6">
			<button v-for="tab in tabs" :key="tab.key"
				class="tab"
				:class="{ 'tab-active': activeTab === tab.key }"
				@click="activeTab = tab.key">
				{{ tab.label }}
			</button>
		</div>

		<ChatTab v-if="activeTab === 'chat'" :models="modelsByCategory('chat').value" />
		<ImageTab v-if="activeTab === 'image'" :models="modelsByCategory('image').value" />
		<AudioTab v-if="activeTab === 'audio'" :models="modelsByCategory('audio').value" />
		<EmbeddingTab v-if="activeTab === 'embedding'" :models="modelsByCategory('embedding').value" />
		<RerankTab v-if="activeTab === 'rerank'" :models="modelsByCategory('rerank').value" />
	</div>
</template>
