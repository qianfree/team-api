<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { marked } from 'marked'
import request from '@/utils/request'
import Icon from '@/components/common/Icon.vue'

const props = defineProps<{
	show: boolean
	code: string
}>()

const title = ref('')
const version = ref('')
const content = ref('')
const loading = ref(false)
const error = ref('')

const contentHtml = computed(() => {
	if (!content.value) return ''
	return marked(content.value) as string
})

watch(() => props.show, async (val) => {
	if (!val || !props.code) return
	title.value = ''
	version.value = ''
	content.value = ''
	error.value = ''
	loading.value = true
	try {
		const res = await request.get(`/tenant/agreements/current/${props.code}`)
		const data = res.data?.data
		if (data) {
			title.value = data.title
			version.value = data.version
			content.value = data.content || ''
		} else {
			error.value = '未找到该协议'
		}
	} catch {
		error.value = '加载协议内容失败'
	} finally {
		loading.value = false
	}
})

const emit = defineEmits<{ close: [] }>()
</script>

<template>
	<Transition name="modal">
		<div v-if="show" class="modal-overlay" @click.self="emit('close')">
			<div class="modal-content max-w-2xl">
				<!-- Header -->
				<div class="modal-header">
					<h3 class="modal-title">{{ title || '协议详情' }}</h3>
					<button class="btn-icon rounded-lg p-1.5 text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors" @click="emit('close')">
						<Icon name="x" size="sm" />
					</button>
				</div>

				<!-- Body -->
				<div class="modal-body">
					<!-- Loading -->
					<div v-if="loading" class="flex items-center justify-center py-12">
						<div class="spinner h-5 w-5 border-primary-500"></div>
					</div>

					<!-- Error -->
					<div v-else-if="error" class="text-center py-10">
						<div class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-2xl bg-gray-100">
							<Icon name="document" size="lg" class="text-gray-400" />
						</div>
						<p class="text-sm text-gray-500">{{ error }}</p>
					</div>

					<!-- Content -->
					<div v-else>
						<p v-if="version" class="mb-3 text-xs text-gray-400">版本 {{ version }}</p>
						<div class="agreement-content" v-html="contentHtml" />
					</div>
				</div>

				<!-- Footer -->
				<div class="modal-footer">
					<button class="btn btn-secondary" @click="emit('close')">关闭</button>
				</div>
			</div>
		</div>
	</Transition>
</template>

<style scoped>
.agreement-content {
	max-height: 60vh;
	overflow-y: auto;
	line-height: 1.8;
	font-size: 14px;
	color: #334155;
	padding-right: 4px;
}
.agreement-content :deep(h1),
.agreement-content :deep(h2),
.agreement-content :deep(h3) {
	margin-top: 16px;
	margin-bottom: 8px;
	color: #0f172a;
	font-weight: 600;
}
.agreement-content :deep(h1) { font-size: 20px; }
.agreement-content :deep(h2) { font-size: 17px; }
.agreement-content :deep(h3) { font-size: 15px; }
.agreement-content :deep(ul),
.agreement-content :deep(ol) {
	padding-left: 20px;
}
.agreement-content :deep(p) {
	margin-bottom: 8px;
}
</style>
