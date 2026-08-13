<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { NForm, NFormItem, NInput, NInputNumber, NDatePicker, NButton } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
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
	expires_at: null as number | null,
	rate_limit_qps: null as number | null,
	rate_limit_concurrency: null as number | null,
	ip_whitelist_text: '',
	total_quota: null as number | null,
})
const formRef = ref<FormInst | null>(null)
const rules: FormRules = {
	name: [{ required: true, message: '请输入密钥名称', trigger: ['input', 'blur'] }],
}
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

// 过期时间：内部以时间戳存储，提交/回填时与后端字符串互转
function pad2(n: number): string {
	return String(n).padStart(2, '0')
}
function tsToLocalInput(ts: number): string {
	const d = new Date(ts)
	return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}T${pad2(d.getHours())}:${pad2(d.getMinutes())}`
}
function parseToTs(s: string | null): number | null {
	if (!s) return null
	const v = new Date(s.replace(' ', 'T')).getTime()
	return Number.isNaN(v) ? null : v
}

// NDatePicker 面板快捷选项
const expiresShortcuts = {
	'1天': () => Date.now() + 24 * 60 * 60 * 1000,
	'1周': () => Date.now() + 7 * 24 * 60 * 60 * 1000,
	'1月': () => Date.now() + 30 * 24 * 60 * 60 * 1000,
}

function copyKey() {
	if (!createdKey.value) return
	navigator.clipboard.writeText(createdKey.value).then(() => {
		toast.success('密钥已复制到剪贴板')
	})
}

function resetForm() {
	form.name = ''
	form.expires_at = null
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
			form.expires_at = parseToTs(props.apiKey.expires_at)
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
	try {
		await formRef.value?.validate()
	} catch {
		// 校验失败：错误信息已由 NFormItem 内联展示
		return
	}
	loading.value = true
	try {
		if (props.mode === 'create') {
			const body: any = {
				name: form.name,
				scope: 'full',
				model_names: selectedModelNames.value,
			}
			applyLimitFields(body)
			if (form.expires_at) body.expires_at = tsToLocalInput(form.expires_at)
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
			if (form.expires_at) body.expires_at = tsToLocalInput(form.expires_at)
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
		<div v-if="isFormMode">
			<n-form ref="formRef" :model="form" :rules="rules" label-placement="top">
				<div class="grid grid-cols-1 items-start gap-x-4 sm:grid-cols-2">
					<n-form-item path="name" label="名称">
						<n-input
							v-model:value="form.name"
							:maxlength="64"
							clearable
							:placeholder="mode === 'create' ? '例如：生产环境密钥' : '密钥名称'"
						/>
					</n-form-item>

					<n-form-item label="过期时间">
						<n-date-picker
							v-model:value="form.expires_at"
							type="datetime"
							clearable
							:format="'yyyy-MM-dd HH:mm'"
							:shortcuts="expiresShortcuts"
							:actions="['clear', 'confirm']"
							placeholder="选择过期时间（留空表示用不过期）"
							style="width: 100%"
						/>
					</n-form-item>
				</div>

				<n-form-item label="可用模型">
					<div class="w-full">
						<p class="mb-2 text-xs text-gray-500">留空表示不限制。</p>
						<div class="mb-2 flex items-center gap-2">
							<div class="flex-1">
								<n-input v-model:value="modelSearch" clearable placeholder="搜索模型...">
									<template #prefix>
										<Icon name="search" size="sm" class="text-gray-400" />
									</template>
								</n-input>
							</div>
							<n-button size="small" secondary @click="selectAllModels">全选</n-button>
							<n-button size="small" secondary @click="clearAllModels">清空</n-button>
						</div>
						<div class="max-h-60 overflow-y-auto divide-y divide-gray-100 rounded-xl border border-gray-200">
							<template v-for="(items, cat) in groupedFilteredModels" :key="cat">
								<div class="sticky top-0 px-4 py-2 bg-gray-50 text-xs font-semibold text-gray-500 uppercase tracking-wider">
									{{ categoryLabel[cat as string] || cat }}
									<span class="text-gray-300">({{ items.length }})</span>
								</div>
								<label
									v-for="m in items"
									:key="m.id"
									class="flex cursor-pointer items-center gap-3 px-4 py-2 hover:bg-gray-50"
								>
									<input
										type="checkbox"
										:checked="selectedModelNames.includes(m.model_id)"
										@change="toggleModelName(m.model_id)"
										class="h-4 w-4 rounded border-gray-300 text-primary-500 focus:ring-primary-500/30"
									/>
									<div class="min-w-0 flex-1">
										<p class="truncate text-sm font-medium text-gray-900">{{ m.model_name || m.model_id }}</p>
										<p class="truncate font-mono text-xs text-gray-400">{{ m.model_id }}</p>
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
						<p class="mt-1 text-xs text-gray-500">
							已选择 <span class="font-medium text-gray-700">{{ selectedModelNames.length }}</span> 个模型
							<template v-if="selectedModelNames.length === 0">（不限制）</template>
						</p>
					</div>
				</n-form-item>

				<div class="grid grid-cols-1 gap-x-4 sm:grid-cols-3">
					<n-form-item label="QPS 限速" class="!mb-0">
						<n-input-number
							v-model:value="form.rate_limit_qps"
							:min="0"
							:show-button="false"
							clearable
							placeholder="留空使用默认"
							style="width: 100%"
						/>
					</n-form-item>
					<n-form-item label="并发限制" class="!mb-0">
						<n-input-number
							v-model:value="form.rate_limit_concurrency"
							:min="0"
							:show-button="false"
							clearable
							placeholder="0 = 不限制"
							style="width: 100%"
						/>
					</n-form-item>
					<n-form-item label="总额度 (USD)" class="!mb-0">
						<n-input-number
							v-model:value="form.total_quota"
							:min="0"
							:show-button="false"
							clearable
							placeholder="0 = 不限制"
							style="width: 100%"
						/>
					</n-form-item>
				</div>
				<p v-if="apiKey && apiKey.used_quota" class="input-hint mb-2">已使用 ${{ apiKey.used_quota?.toFixed(4) }}</p>

				<n-form-item label="IP 白名单">
					<div class="w-full">
						<n-input
							v-model:value="form.ip_whitelist_text"
							type="textarea"
							:rows="3"
							placeholder="每行一个 IP 或 CIDR，留空表示不限制"
						/>
						<p class="input-hint">支持 192.168.1.10 或 10.0.0.0/24，也可用逗号分隔。</p>
					</div>
				</n-form-item>
			</n-form>
		</div>

		<div v-else class="space-y-4">
			<div class="flex items-center gap-3 mb-2">
				<div class="h-10 w-10 rounded-full bg-emerald-100 flex items-center justify-center flex-shrink-0">
					<Icon name="checkCircle" size="md" class="text-emerald-600" />
				</div>
				<div>
					<p class="text-sm text-gray-500">请复制并妥善保存密钥，之后仍可在密钥列表中重新复制</p>
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
				<div class="flex items-center justify-end gap-2">
					<button @click="handleClose" class="btn btn-secondary">取消</button>
					<button @click="handleSubmit" :disabled="loading" class="btn btn-primary">
						{{ loading ? '创建中...' : '创建' }}
					</button>
				</div>
			</template>
			<template v-else-if="mode === 'create' && createdKey">
				<button @click="handleClose" class="btn btn-secondary w-full">完成</button>
			</template>
			<template v-else>
				<div class="flex items-center justify-end gap-2">
					<button @click="handleClose" class="btn btn-secondary">取消</button>
					<button @click="handleSubmit" :disabled="loading" class="btn btn-primary">
						{{ loading ? '保存中...' : '保存' }}
					</button>
				</div>
			</template>
		</template>
	</BaseModal>
</template>