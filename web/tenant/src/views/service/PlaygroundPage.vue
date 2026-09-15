<script setup lang="ts">
import { ref, computed, watch, type Component } from 'vue'
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
	// 模型能力特性 JSON（如 {"vision":true,"audio_input":true}），用于门控附件上传
	capabilities?: string
}

// 每个 Tab 的图标，用于分段控件左侧的视觉锚点
const tabs = [
	{ key: 'chat', label: '对话', icon: 'chat' },
	{ key: 'image', label: '图像', icon: 'photo' },
	{ key: 'video', label: '视频', icon: 'film' },
	{ key: 'audio', label: '语音', icon: 'musicalNote' },
	{ key: 'embedding', label: '嵌入', icon: 'cube' },
	{ key: 'rerank', label: '重排', icon: 'chart' },
]

const activeTab = ref('chat')
const allModels = ref<ModelItem[]>([])

const { apiKeys, selectedKeyId, revealedKey, loading: keyLoading, error: keyError, selectKey } = usePlaygroundApiKey()

// Tab 与组件的映射：六个 Tab 的 props 签名一致（models + apiKey），配合 KeepAlive
// 按 key 缓存实例，切换 Tab 不销毁组件，对话/任务/参数等状态全部保留。
const tabComponents: Record<string, Component> = {
	chat: ChatTab,
	image: ImageTab,
	video: VideoTab,
	audio: AudioTab,
	embedding: EmbeddingTab,
	rerank: RerankTab,
}

// 当前 Tab 对应分类的模型列表（传给 KeepAlive 缓存的组件，切换回来时 props 保持响应）
const activeTabModels = computed(() =>
	allModels.value.filter(m => m.category === activeTab.value),
)

const modelsLoading = ref(false)
let loadingTimer: number | null = null
let loadSeq = 0

// 当前 Key 的显示名与掩码前缀，供工具栏的 Key 胶囊展示
const selectedKeyLabel = computed(() => {
	const key = apiKeys.value.find(k => k.id === selectedKeyId.value)
	if (!key) return ''
	return `${key.name} · ${key.key_prefix}···`
})

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
		<!-- 统一工具栏：左侧模型类型分段控件，右侧调用身份（API Key）与计费提示。
		     计费提示从原对话区黄色横幅迁移至此，不再占用对话区高度。 -->
		<div class="card mb-3">
			<div class="flex flex-wrap items-center gap-x-3 gap-y-2 px-3 py-2.5 sm:px-4">
				<!-- 左侧：模型类型分段控件 -->
				<div class="tabs flex-wrap" role="tablist">
					<button v-for="tab in tabs" :key="tab.key"
						role="tab"
						class="tab flex items-center gap-1.5"
						:class="{ 'tab-active': activeTab === tab.key }"
						:aria-selected="activeTab === tab.key"
						@click="activeTab = tab.key">
						<Icon :name="tab.icon" size="xs" />
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
					<!-- 真实计费提示：悬停展示完整说明 -->
					<span
						class="hidden items-center gap-1 rounded-full border border-amber-200 bg-amber-50 px-2.5 py-1 text-[11px] font-medium text-amber-700 xl:inline-flex"
						title="Playground 模式 — 使用真实 API Key 调用，产生实际费用"
					>
						<Icon name="bolt" size="xs" />
						真实计费
					</span>

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
						<div class="key-picker">
							<Icon name="key" size="xs" class="key-picker-icon" />
							<n-select
								:value="selectedKeyId"
								:options="apiKeys.map(k => ({ label: `${k.name} (${k.key_prefix}...)`, value: k.id }))"
								placeholder="选择 API Key"
								size="small"
								class="key-picker-select"
								:title="selectedKeyLabel"
								@update:value="selectKey"
							/>
						</div>
					</template>
				</div>
			</div>
		</div>

		<template v-if="revealedKey">
			<!-- KeepAlive 缓存各 Tab 实例：切换不销毁，切回即恢复（含进行中的任务轮询与已生成的结果） -->
			<KeepAlive>
				<component
					:is="tabComponents[activeTab]"
					:key="activeTab"
					:models="activeTabModels"
					:api-key="revealedKey"
				/>
			</KeepAlive>
		</template>
	</div>
</template>

<style scoped>
/* 分段控件：激活项用主色文字与更清晰的投影，提升可辨识度 */
.tab {
	display: inline-flex;
	align-items: center;
}
.tab-active {
	color: #0d9488;
	box-shadow: 0 1px 3px rgba(15, 23, 42, 0.08), 0 0 0 1px rgba(20, 184, 166, 0.18);
}

/* API Key 胶囊：key 图标 + 紧凑下拉，与左侧分段控件在同一视觉高度上 */
.key-picker {
	display: flex;
	align-items: center;
	gap: 0.25rem;
	height: 2.25rem;
	border: 1px solid #e5e7eb;
	border-radius: 0.75rem;
	background: #fff;
	padding-left: 0.55rem;
	padding-right: 0.25rem;
	box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
	transition: border-color 180ms ease, box-shadow 180ms ease;
}
.key-picker:hover,
.key-picker:focus-within {
	border-color: rgba(20, 184, 166, 0.45);
	box-shadow: 0 0 0 4px rgba(20, 184, 166, 0.08);
}
.key-picker-icon {
	color: #f59e0b;
	flex-shrink: 0;
}
.key-picker-select {
	width: 12.5rem;
}

.fade-enter-active,
.fade-leave-active {
	transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
	opacity: 0;
}

@media (max-width: 639px) {
	.key-picker-select {
		width: 9rem;
	}
}
</style>
