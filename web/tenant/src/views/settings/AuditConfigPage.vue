<script setup lang="ts">
import { ref, onMounted } from 'vue'
import request from '@/utils/request'
import { toast } from '@/utils/toast'

const auditLevel = ref('masked')
const levelLoading = ref(false)
const saveLoading = ref(false)

const levelOptions = [
	{ label: '完整记录', value: 'full', desc: '记录所有请求的完整内容（含请求体和响应体）' },
	{ label: '全量文本', value: 'full_text', desc: '记录思考过程和回答内容，不含原始 SSE 数据' },
	{ label: '脱敏记录', value: 'masked', desc: '记录请求和响应，但敏感字段会脱敏处理' },
	{ label: '仅记录提问', value: 'question_only', desc: '仅记录用户提问内容，不记录模型回复' },
	{ label: '关闭', value: 'none', desc: '不记录任何审计日志' },
]

async function fetchAuditConfig() {
	levelLoading.value = true
	try {
		const res: any = await request.get('/tenant/audit/config')
		const data = res.data?.data
		auditLevel.value = data?.audit_level || 'masked'
	} catch {
	} finally {
		levelLoading.value = false
	}
}

async function saveAuditConfig() {
	saveLoading.value = true
	try {
		await request.put('/tenant/audit/config', { audit_level: auditLevel.value })
		toast.success('保存成功')
	} catch {
	} finally {
		saveLoading.value = false
	}
}

onMounted(() => {
	fetchAuditConfig()
})
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="page-header">
			<div>
				<h1 class="page-title">审计设置</h1>
				<p class="page-description">配置组织的审计日志记录级别</p>
			</div>
		</div>

		<div class="card">
			<div class="card-header">
				<h2 class="text-lg font-semibold text-gray-900">审计级别</h2>
			</div>
			<div class="card-body space-y-4">
				<!-- Level selector -->
				<div class="space-y-3">
					<label class="input-label">选择审计级别</label>
					<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
						<div
							v-for="opt in levelOptions"
							:key="opt.value"
							class="p-4 rounded-xl border-2 cursor-pointer transition-all duration-200"
							:class="[
								auditLevel === opt.value
									? 'border-primary-500 bg-primary-50'
									: 'border-gray-200 hover:border-gray-300',
							]"
							@click="auditLevel = opt.value"
						>
							<div class="flex items-center gap-2">
								<div
									class="h-4 w-4 rounded-full border-2 flex items-center justify-center"
									:class="auditLevel === opt.value ? 'border-primary-500' : 'border-gray-300'"
								>
									<div v-if="auditLevel === opt.value" class="h-2 w-2 rounded-full bg-primary-500"></div>
								</div>
								<span class="text-sm font-medium" :class="auditLevel === opt.value ? 'text-primary-700' : 'text-gray-700'">
									{{ opt.label }}
								</span>
							</div>
							<p class="text-xs text-gray-500 mt-1 ml-6">{{ opt.desc }}</p>
						</div>
					</div>
				</div>

				<div class="flex justify-end pt-2">
					<button
						class="btn btn-primary"
						:disabled="saveLoading"
						@click="saveAuditConfig"
					>
						{{ saveLoading ? '保存中...' : '保存配置' }}
					</button>
				</div>
			</div>
		</div>
	</div>
</template>
