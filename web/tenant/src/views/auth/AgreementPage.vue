<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { marked } from 'marked'
import request from '@/utils/request'
import AuthLayout from '@/components/layout/AuthLayout.vue'
import Icon from '@/components/common/Icon.vue'

const route = useRoute()
const code = computed(() => route.params.code as string)

const title = ref('')
const version = ref('')
const content = ref('')
const loading = ref(true)
const error = ref('')

const contentHtml = computed(() => {
	if (!content.value) return ''
	return marked(content.value) as string
})

onMounted(async () => {
	try {
		const res = await request.get(`/tenant/agreements/current/${code.value}`)
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
</script>

<template>
	<AuthLayout>
		<div class="animate-slide-up">
			<!-- Loading -->
			<div v-if="loading" class="flex items-center justify-center py-16">
				<div class="spinner h-6 w-6 border-primary-500"></div>
			</div>

			<!-- Error -->
			<div v-else-if="error" class="text-center py-12">
				<div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-gray-100">
					<Icon name="document" size="lg" class="text-gray-400" />
				</div>
				<p class="text-sm text-gray-500">{{ error }}</p>
			</div>

			<!-- Content -->
			<div v-else>
				<div class="mb-4">
					<h2 class="text-xl font-bold text-gray-900">{{ title }}</h2>
					<p v-if="version" class="mt-1 text-xs text-gray-400">版本 {{ version }}</p>
				</div>
				<div class="agreement-content rounded-xl border border-gray-200 bg-gray-50 p-5" v-html="contentHtml" />
			</div>
		</div>

		<template #footer>
			<p class="text-gray-500">
				<router-link to="/tenant/login" class="text-primary-600 font-medium hover:text-primary-700 transition-colors">
					返回登录
				</router-link>
			</p>
		</template>
	</AuthLayout>
</template>

<style scoped>
.agreement-content {
	max-height: 500px;
	overflow-y: auto;
	line-height: 1.8;
	font-size: 14px;
	color: #334155;
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
