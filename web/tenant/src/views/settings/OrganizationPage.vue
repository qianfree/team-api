<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import Icon from '@/components/common/Icon.vue'
import BaseModal from '@/components/common/BaseModal.vue'
import request from '@/utils/request'
import { toast } from '@/utils/toast'

const authStore = useTenantAuthStore()

const loading = ref(false)
const orgInfo = ref<any>(null)

const editingName = ref(false)
const nameForm = reactive({
	name: '',
})
const nameSaving = ref(false)

const showTransferModal = ref(false)
const transferForm = reactive({
	new_owner_id: '',
	password: '',
})
const transferLoading = ref(false)

// 审计设置
const auditLevel = ref('masked')
const auditLoading = ref(false)
const auditSaving = ref(false)

const auditLevelOptions = [
	{ label: '全量文本', value: 'full_text', desc: '记录思考过程和回答内容，不含原始 SSE 数据' },
	{ label: '脱敏记录', value: 'masked', desc: '记录请求和响应，但敏感字段会脱敏处理' },
	{ label: '仅记录提问', value: 'question_only', desc: '仅记录用户提问内容，不记录模型回复' },
	{ label: '关闭', value: 'none', desc: '不记录任何审计日志' },
]

const statusLabel: Record<string, string> = {
	active: '正常',
	suspended: '已暂停',
	trial: '试用',
}

const statusBadgeClass: Record<string, string> = {
	active: 'badge-success',
	suspended: 'badge-warning',
	trial: 'badge-primary',
}

async function fetchOrgInfo() {
	loading.value = true
	try {
		const res: any = await request.get('/tenant/organization')
		orgInfo.value = res.data.data
		nameForm.name = res.data?.data?.name || ''
	} catch {
		// silent
	} finally {
		loading.value = false
	}
}

async function saveName() {
	if (!nameForm.name.trim()) return

	nameSaving.value = true
	try {
		await request.put('/tenant/organization', { name: nameForm.name })
		if (orgInfo.value) {
			orgInfo.value.name = nameForm.name
			authStore.tenant!.name = nameForm.name
		}
		editingName.value = false
	} catch {
	} finally {
		nameSaving.value = false
	}
}

function cancelEditName() {
	nameForm.name = orgInfo.value?.name || ''
	editingName.value = false
}

async function handleTransferOwnership() {
	if (!transferForm.new_owner_id.trim() || !transferForm.password.trim()) return

	transferLoading.value = true
	try {
		await request.post('/tenant/organization/transfer', {
			new_owner_id: Number(transferForm.new_owner_id),
			password: transferForm.password,
		})
		showTransferModal.value = false
		transferForm.new_owner_id = ''
		transferForm.password = ''
		fetchOrgInfo()
	} catch {
	} finally {
		transferLoading.value = false
	}
}

async function fetchAuditConfig() {
	auditLoading.value = true
	try {
		const res: any = await request.get('/tenant/audit/config')
		const data = res.data?.data
		auditLevel.value = (data?.audit_level === 'full' ? 'full_text' : data?.audit_level) || 'masked'
	} catch {
	} finally {
		auditLoading.value = false
	}
}

async function saveAuditConfig() {
	auditSaving.value = true
	try {
		await request.put('/tenant/audit/config', { audit_level: auditLevel.value })
		toast.success('审计设置已保存')
	} catch {
	} finally {
		auditSaving.value = false
	}
}

onMounted(() => {
	fetchOrgInfo()
	fetchAuditConfig()
})
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="page-header">
			<h1 class="page-title">组织设置</h1>
			<p class="page-description">管理组织设置和信息</p>
		</div>

		<!-- Skeleton -->
		<div v-if="loading" class="card p-6 space-y-4">
			<div class="skeleton h-5 w-24 mb-4"></div>
			<div class="grid grid-cols-1 md:grid-cols-2 gap-y-5 gap-x-8">
				<div v-for="i in 4" :key="i">
					<div class="skeleton h-4 w-16 mb-2"></div>
					<div class="skeleton h-5 w-32"></div>
				</div>
			</div>
		</div>

		<!-- Organization Info Card -->
		<div v-else-if="orgInfo" class="card">
			<div class="card-header">
				<h2 class="font-semibold text-gray-900">组织详情</h2>
			</div>
			<div class="card-body">
				<div class="grid grid-cols-1 md:grid-cols-2 gap-y-5 gap-x-8">
					<!-- Name -->
					<div>
						<p class="input-label">组织名称</p>
						<div v-if="!editingName" class="flex items-center gap-2">
							<span class="text-gray-900 font-medium">{{ orgInfo.name }}</span>
							<button
								@click="editingName = true"
								class="text-primary-600 hover:text-primary-500 transition-colors"
							>
								<Icon name="edit" size="sm" />
							</button>
						</div>
						<div v-else class="flex items-center gap-2">
							<input
								v-model="nameForm.name"
								type="text"
								class="input flex-1"
								@keyup.enter="saveName"
							/>
							<button
								@click="saveName"
								:disabled="nameSaving"
								class="btn btn-primary btn-sm"
							>
								保存
							</button>
							<button
								@click="cancelEditName"
								class="btn btn-secondary btn-sm"
							>
								取消
							</button>
						</div>
					</div>

					<!-- Code -->
					<div>
						<p class="input-label">组织代码</p>
						<div class="flex items-center gap-2">
							<span class="badge badge-gray font-mono">
								{{ orgInfo.code }}
							</span>
							<span class="input-hint">只读</span>
						</div>
					</div>

					<!-- Status -->
					<div>
						<p class="input-label">状态</p>
						<span
							class="badge"
							:class="statusBadgeClass[orgInfo.status] || 'badge-gray'"
						>
							{{ statusLabel[orgInfo.status] || orgInfo.status }}
						</span>
					</div>

					<!-- Level -->
					<div>
						<p class="input-label">等级</p>
						<span class="badge badge-primary">
							{{ orgInfo.level_name || 'LV' + orgInfo.level }}
						</span>
					</div>

					<!-- Member Count -->
					<div>
						<p class="input-label">成员数</p>
						<span class="text-gray-900 font-medium">{{ orgInfo.member_count || 0 }}</span>
						<span class="text-gray-400 text-sm">/ {{ orgInfo.max_members || '--' }}</span>
					</div>

					<!-- Created At -->
					<div>
						<p class="input-label">创建时间</p>
						<span class="text-gray-900 font-medium">{{ orgInfo.created_at || '--' }}</span>
					</div>
				</div>
			</div>
		</div>

		<!-- Audit Config -->
		<div class="card">
			<div class="card-header">
				<h2 class="text-lg font-semibold text-gray-900">审计设置</h2>
				<p class="text-sm text-gray-500 mt-0.5">配置组织的审计日志记录级别</p>
			</div>
			<div class="card-body space-y-4">
				<div v-if="auditLoading" class="space-y-3">
					<div class="skeleton h-4 w-16"></div>
					<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
						<div v-for="i in 4" :key="i" class="skeleton h-16 rounded-xl"></div>
					</div>
				</div>
				<template v-else>
					<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
						<div
							v-for="opt in auditLevelOptions"
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

					<div class="flex justify-end pt-2">
						<button
							class="btn btn-primary"
							:disabled="auditSaving"
							@click="saveAuditConfig"
						>
							{{ auditSaving ? '保存中...' : '保存配置' }}
						</button>
					</div>
				</template>
			</div>
		</div>

		<!-- Danger Zone -->
		<div v-if="authStore.isOwner" class="card border-red-200">
			<div class="card-header bg-red-50/50 border-red-100">
				<h2 class="font-semibold text-red-800">危险操作</h2>
				<p class="text-sm text-red-600/70 mt-0.5">不可逆的敏感操作</p>
			</div>
			<div class="card-body">
				<!-- Transfer Ownership -->
				<div class="flex items-center justify-between">
					<div>
						<p class="text-sm font-medium text-gray-900">转让所有权</p>
						<p class="text-sm text-gray-500 mt-0.5">
							将组织转让给其他成员，您将被降级为管理员。
						</p>
					</div>
					<button
						@click="showTransferModal = true"
						class="btn btn-sm text-red-600 border border-red-300 hover:bg-red-50"
					>
						转让
					</button>
				</div>
			</div>
		</div>

		<!-- Transfer Modal -->
		<BaseModal
			:show="showTransferModal"
			title="转让所有权"
			width="narrow"
			@close="showTransferModal = false"
		>
			<div class="space-y-4">
				<div class="flex items-center gap-3 mb-2">
					<div class="h-10 w-10 rounded-full bg-red-100 flex items-center justify-center flex-shrink-0">
						<Icon name="exclamationTriangle" size="md" class="text-red-600" />
					</div>
					<div>
						<p class="text-sm text-gray-500">此操作不可撤销</p>
					</div>
				</div>

				<div>
					<label class="input-label">新所有者用户 ID</label>
					<input
						v-model="transferForm.new_owner_id"
						type="number"
						placeholder="请输入成员的用户 ID"
						class="input"
					/>
				</div>

				<div>
					<label class="input-label">确认您的密码</label>
					<input
						v-model="transferForm.password"
						type="password"
						placeholder="请输入密码以确认操作"
						class="input"
					/>
				</div>
			</div>

			<template #footer>
				<button
					@click="showTransferModal = false"
					class="btn btn-secondary"
				>
					取消
				</button>
				<button
					@click="handleTransferOwnership"
					:disabled="transferLoading"
					class="btn btn-danger"
				>
					{{ transferLoading ? '转让中...' : '确认转让' }}
				</button>
			</template>
		</BaseModal>
	</div>
</template>
