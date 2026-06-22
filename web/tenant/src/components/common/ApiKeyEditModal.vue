<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import BaseModal from '@/components/common/BaseModal.vue'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'
import { toast } from '@/utils/toast'

export interface ApiKeyData {
	id: number
	name: string
	expires_at: string | null
	rate_limit_qps: number | null
	rate_limit_concurrency: number | null
	ip_whitelist: string[] | null
	total_quota: number | null
	used_quota: number | null
}

const props = withDefaults(defineProps<{
	show: boolean
	mode: 'create' | 'edit'
	apiKey?: ApiKeyData | null
	projectId?: number
}>(), {
	apiKey: null,
	projectId: undefined,
})

const emit = defineEmits<{
	'update:show': [value: boolean]
	saved: []
}>()

const form = reactive({
	name: '',
	expires_at: '',
	rate_limit_qps: null as number | null,
	rate_limit_concurrency: null as number | null,
	ip_whitelist_text: '',
	total_quota: null as number | null,
})
const loading = ref(false)
const createdKey = ref('')

// Model selection
interface ModelItem {
	id: number
	model_id: string
	model_name: string
	category: string
}
const allModels = ref<ModelItem[]>([])
const selectedModelNames = ref<string[]>([])
const modelSearch = ref('')

const categoryLabel: Record<string, string> = {
	chat: '对话',
	embedding: '嵌入',
	image: '图像',
	audio: '语音',
	video: '视频',
	rerank: '重排',
}
const categoryBadgeClass: Record<string, string> = {
	chat: 'badge-primary',
	embedding: 'badge-purple',
	image: 'badge-warning',
	audio: 'badge-success',
	video: 'badge-purple',
	rerank: 'badge-gray',
}

const filteredModels = computed(() => {
	if (!modelSearch.value) return allModels.value
	const q = modelSearch.value.toLowerCase()
	return allModels.value.filter(
		m => m.model_id.toLowerCase().includes(q) || m.model_name.toLowerCase().includes(q)
	)
})

const groupedFilteredModels = computed(() => {
	const groups: Record<string, ModelItem[]> = {}
	for (const m of filteredModels.value) {
		const cat = m.category || 'other'
		if (!groups[cat]) groups[cat] = []
		groups[cat].push(m)
	}
	return groups
})

function toggleModelName(modelId: string) {
	const idx = selectedModelNames.value.indexOf(modelId)
	if (idx >= 0) {
		selectedModelNames.value.splice(idx, 1)
	} else {
		selectedModelNames.value.push(modelId)
	}
}

function selectAllModels() {
	selectedModelNames.value = allModels.value.map(m => m.model_id)
}

function clearAllModels() {
	selectedModelNames.value = []
}

async function fetchAllModels() {
	try {
		const res: any = await request.get('/tenant/models')
		const raw = res.data?.data
		allModels.value = Array.isArray(raw) ? raw : (raw?.list || [])
	} catch {
		allModels.value = []
	}
}

function quickFillExpires(days: number) {
	const d = new Date()
	d.setDate(d.getDate() + days)
	form.expires_at = d.toISOString().slice(0, 16)
}

function formatDate(d: string | null): string {
	if (!d) return '永不过期'
	return d.replace('T', ' ').substring(0, 16)
}

function copyKey() {
	if (!createdKey.value) return
	navigator.clipboard.writeText(createdKey.value).then(() => {
		toast.success('密钥已复制到剪贴板')
	})
}

function resetForm() {
	form.name = ''
	form.expires_at = ''
	form.rate_limit_qps = null
	form.rate_limit_concurrency = null
	form.ip_whitelist_text = ''
	form.total_quota = null
	selectedModelNames.value = []
	modelSearch.value = ''
	createdKey.value = ''
}

function parseIPWhitelist(): string[] {
	return form.ip_whitelist_text
		.split(/[\n,]/)
		.map(item => item.trim())
		.filter(Boolean)
}

function isNonNegativeNumber(value: number | null): value is number {
	return typeof value === 'number' && Number.isFinite(value) && value >= 0
}

function applyLimitFields(body: Record<string, any>) {
	if (isNonNegativeNumber(form.rate_limit_qps)) {
		body.rate_limit_qps = form.rate_limit_qps
	}
	if (isNonNegativeNumber(form.rate_limit_concurrency)) {
		body.rate_limit_concurrency = form.rate_limit_concurrency
	}
	if (isNonNegativeNumber(form.total_quota)) {
		body.total_quota = form.total_quota
	}
	body.ip_whitelist = parseIPWhitelist()
}

const modalTitle = computed(() => {
	if (props.mode === 'create') {
		return createdKey.value ? '密钥创建成功' : (props.projectId ? '创建项目密钥' : '创建 API 密钥')
	}
	return props.projectId ? '编辑项目密钥' : '编辑 API 密钥'
})

// Load model scopes when editing
async function loadModelScopes(keyId: number) {
	try {
		const res: any = await request.get(`/tenant/api-keys/${keyId}/model-scopes`)
		selectedModelNames.value = res.data?.data?.model_names || []
	} catch {
		selectedModelNames.value = []
	}
}

// Watch show to init/reset form
watch(() => props.show, (val) => {
	if (val) {
		resetForm()
		fetchAllModels()
		if (props.mode === 'edit' && props.apiKey) {
			form.name = props.apiKey.name
			form.rate_limit_qps = props.apiKey.rate_limit_qps
			form.rate_limit_concurrency = props.apiKey.rate_limit_concurrency
			form.ip_whitelist_text = (props.apiKey.ip_whitelist || []).join('\n')
			form.total_quota = props.apiKey.total_quota
			loadModelScopes(props.apiKey.id)
		}
	}
})

function handleClose() {
	emit('update:show', false)
}

async function handleSubmit() {
	if (!form.name.trim()) return
	loading.value = true
	try {
		if (props.mode === 'create') {
			const body: any = {
				name: form.name,
				scope: 'full',
				model_names: selectedModelNames.value,
			}
			applyLimitFields(body)
			if (form.expires_at) body.expires_at = form.expires_at
			const url = props.projectId
				? `/tenant/projects/${props.projectId}/api-keys`
				: '/tenant/api-keys'
			const res: any = await request.post(url, body)
			createdKey.value = (res.data?.data?.data || res.data?.data)?.key || ''
		} else {
			const body: any = {
				name: form.name,
				model_names: selectedModelNames.value,
			}
			applyLimitFields(body)
			if (form.expires_at) body.expires_at = form.expires_at
			await request.put(`/tenant/api-keys/${props.apiKey!.id}`, body)
			toast.success('更新成功')
			emit('update:show', false)
		}
		emit('saved')
	} catch {
	} finally {
		loading.value = false
	}
}

const isFormMode = computed(() => {
	if (props.mode === 'create') return !createdKey.value
	return true
})
</script>

<template>
	<BaseModal
		:show="show"
		:title="modalTitle"
		width="wide"
		@close="handleClose"
	>
		<div v-if="isFormMode" class="space-y-4">
			<div>
				<label class="input-label">名称</label>
				<input
					v-model="form.name"
					type="text"
					class="input"
					:placeholder="mode === 'create' ? '例如：生产环境密钥' : '密钥名称'"
				/>
			</div>

			<div>
				<label class="input-label">可用模型</label>
				<p class="text-xs text-gray-500 mb-2">留空表示不限制。</p>
				<div class="flex items-center gap-3 mb-2">
					<div class="relative flex-1">
						<Icon name="search" size="sm" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
						<input v-model="modelSearch" type="text" class="input pl-9" placeholder="搜索模型..." />
					</div>
					<button class="btn btn-ghost btn-sm" @click="selectAllModels">全选</button>
					<button class="btn btn-ghost btn-sm" @click="clearAllModels">清空</button>
				</div>
				<div class="max-h-60 overflow-y-auto border border-gray-200 rounded-xl divide-y divide-gray-100">
					<template v-for="(items, cat) in groupedFilteredModels" :key="cat">
						<div class="px-4 py-2 bg-gray-50 text-xs font-semibold text-gray-500 uppercase tracking-wider sticky top-0">
							{{ categoryLabel[cat as string] || cat }}
							<span class="text-gray-300">({{ items.length }})</span>
						</div>
						<label
							v-for="m in items"
							:key="m.id"
							class="flex items-center gap-3 px-4 py-2 hover:bg-gray-50 cursor-pointer"
						>
							<input
								type="checkbox"
								:checked="selectedModelNames.includes(m.model_id)"
								@change="toggleModelName(m.model_id)"
								class="h-4 w-4 rounded border-gray-300 text-primary-500 focus:ring-primary-500/30"
							/>
							<div class="min-w-0 flex-1">
								<p class="text-sm font-medium text-gray-900 truncate">{{ m.model_name || m.model_id }}</p>
								<p class="text-xs text-gray-400 font-mono truncate">{{ m.model_id }}</p>
							</div>
							<span class="badge shrink-0" :class="categoryBadgeClass[m.category] || 'badge-gray'">
								{{ categoryLabel[m.category] || m.category }}
							</span>
						</label>
					</template>
					<div v-if="filteredModels.length === 0" class="px-4 py-6 text-center text-sm text-gray-400">
						没有匹配的模型
					</div>
				</div>
				<p class="text-xs text-gray-500 mt-1">
					已选择 <span class="font-medium text-gray-700">{{ selectedModelNames.length }}</span> 个模型
					<template v-if="selectedModelNames.length === 0">（不限制）</template>
				</p>
			</div>

			<div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
				<div>
					<label class="input-label">QPS 限速</label>
					<input v-model.number="form.rate_limit_qps" type="number" min="0" class="input" placeholder="留空使用默认" />
				</div>
				<div>
					<label class="input-label">并发限制</label>
					<input v-model.number="form.rate_limit_concurrency" type="number" min="0" class="input" placeholder="0 = 不限制" />
				</div>
				<div>
					<label class="input-label">总额度 (USD)</label>
					<input v-model.number="form.total_quota" type="number" step="0.01" min="0" class="input" placeholder="0 = 不限制" />
					<p v-if="apiKey && apiKey.used_quota" class="input-hint">已使用 ${{ apiKey.used_quota?.toFixed(4) }}</p>
				</div>
			</div>

			<div>
				<label class="input-label">IP 白名单</label>
				<textarea
					v-model="form.ip_whitelist_text"
					class="input"
					rows="3"
					placeholder="每行一个 IP 或 CIDR，留空表示不限制"
				></textarea>
				<p class="input-hint">支持 192.168.1.10 或 10.0.0.0/24，也可用逗号分隔。</p>
			</div>

			<div>
				<label class="input-label">过期时间</label>
				<div class="flex items-center gap-2">
					<input
						v-model="form.expires_at"
						type="datetime-local"
						class="input flex-1"
						:placeholder="mode === 'create' ? '留空表示永不过期' : '留空表示不修改'"
					/>
					<div class="inline-flex rounded-lg overflow-hidden border border-gray-200 shrink-0">
						<button type="button" @click="quickFillExpires(1)" class="px-3.5 py-2 text-sm font-medium text-emerald-700 bg-emerald-50 hover:bg-emerald-100 transition-colors border-r border-gray-200">1天</button>
						<button type="button" @click="quickFillExpires(7)" class="px-3.5 py-2 text-sm font-medium text-amber-700 bg-amber-50 hover:bg-amber-100 transition-colors border-r border-gray-200">1周</button>
						<button type="button" @click="quickFillExpires(30)" class="px-3.5 py-2 text-sm font-medium text-violet-700 bg-violet-50 hover:bg-violet-100 transition-colors">1月</button>
					</div>
				</div>
				<p v-if="mode === 'edit' && apiKey && apiKey.expires_at" class="input-hint">当前过期时间：{{ formatDate(apiKey.expires_at) }}</p>
				<p v-else-if="mode === 'edit'" class="input-hint">当前：永不过期</p>
				<p v-else class="input-hint">留空表示永不过期</p>
			</div>
		</div>

		<div v-else class="space-y-4">
			<div class="flex items-center gap-3 mb-2">
				<div class="h-10 w-10 rounded-full bg-emerald-100 flex items-center justify-center flex-shrink-0">
					<Icon name="checkCircle" size="md" class="text-emerald-600" />
				</div>
				<div>
					<p class="text-sm text-gray-500">请立即复制密钥，关闭后将无法再次查看</p>
				</div>
			</div>
			<div class="p-3 bg-gray-900 rounded-xl">
				<p class="text-sm font-mono text-emerald-400 break-all select-all">{{ createdKey }}</p>
			</div>
			<button @click="copyKey" class="btn btn-primary w-full">
				<Icon name="copy" size="sm" />
				复制密钥
			</button>
		</div>

		<template #footer>
			<template v-if="mode === 'create' && !createdKey">
				<button @click="handleClose" class="btn btn-secondary">取消</button>
				<button @click="handleSubmit" :disabled="loading || !form.name.trim()" class="btn btn-primary">
					{{ loading ? '创建中...' : '创建' }}
				</button>
			</template>
			<template v-else-if="mode === 'create' && createdKey">
				<button @click="handleClose" class="btn btn-secondary w-full">完成</button>
			</template>
			<template v-else>
				<button @click="handleClose" class="btn btn-secondary">取消</button>
				<button @click="handleSubmit" :disabled="loading || !form.name.trim()" class="btn btn-primary">
					{{ loading ? '保存中...' : '保存' }}
				</button>
			</template>
		</template>
	</BaseModal>
</template>
