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

// 团队功能激活/管理
const teamForm = reactive({
	name: '',
	code: '',
})
const teamSaving = ref(false)
const editingTeamCode = ref(false)
const codeError = ref('')

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
		teamForm.name = res.data?.data?.name || ''
		teamForm.code = res.data?.data?.code || ''
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

// 组织代码格式校验：3-30 位，小写字母/数字/连字符，字母数字开头结尾
function validateTeamCode(val: string): boolean {
	if (!val.trim()) {
		codeError.value = '请输入组织代码'
		return false
	}
	if (!/^[a-z0-9][a-z0-9-]*[a-z0-9]$/.test(val) || val.length < 3 || val.length > 30) {
		codeError.value = '3-30 位，小写字母、数字、连字符，字母数字开头结尾'
		return false
	}
	codeError.value = ''
	return true
}

// 启用团队功能：同时设置组织名称和组织代码（首次激活，后端置 team_enabled=true）
async function enableTeam() {
	if (!teamForm.name.trim()) {
		toast.error('请输入组织名称')
		return
	}
	if (!validateTeamCode(teamForm.code)) return
	teamSaving.value = true
	try {
		await request.put('/tenant/organization', { name: teamForm.name, code: teamForm.code })
		toast.success('团队功能已启用')
		await fetchOrgInfo()
		await authStore.refreshOrgInfo()
	} catch {
	} finally {
		teamSaving.value = false
	}
}

// 已激活后修改组织代码（RAM 登录账号格式同步变化）
async function saveTeamCode() {
	if (!validateTeamCode(teamForm.code)) return
	teamSaving.value = true
	try {
		await request.put('/tenant/organization', { code: teamForm.code })
		toast.success('组织代码已更新，RAM 登录账号格式已同步变化')
		editingTeamCode.value = false
		await fetchOrgInfo()
		await authStore.refreshOrgInfo()
	} catch {
	} finally {
		teamSaving.value = false
	}
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

// 等级权益
const levelBenefitsLoading = ref(false)
const levelBenefits = ref<any>(null)

function formatMembers(val: number) {
	return val === 0 ? '无限制' : String(val)
}

function formatConcurrency(val: number) {
	return val === 0 ? '无限制' : String(val)
}

function formatDiscount(multiplier: number) {
	if (multiplier >= 1) return '无折扣'
	const discount = Math.round((1 - multiplier) * 100)
	return `${discount === 0 ? '无折扣' : discount + '% 折扣'}`
}

function formatThreshold(val: number) {
	if (val === 0) return '—'
	return '$' + val.toFixed(2)
}

function getNextLevelThreshold() {
	if (!levelBenefits.value) return null
	const { current_level: cur, list } = levelBenefits.value
	const next = list.find((item: any) => item.level === cur + 1)
	return next ? next.cumulative_recharge_threshold : null
}

function getUpgradePercent() {
	if (!levelBenefits.value) return 0
	const curThreshold = levelBenefits.value.list.find(
		(item: any) => item.level === levelBenefits.value.current_level
	)?.cumulative_recharge_threshold || 0
	const nextThreshold = getNextLevelThreshold()
	if (!nextThreshold) return 100
	const range = nextThreshold - curThreshold
	if (range <= 0) return 100
	const progress = levelBenefits.value.cumulative_recharge - curThreshold
	return Math.min(Math.round((progress / range) * 100), 100)
}

async function fetchLevelBenefits() {
	levelBenefitsLoading.value = true
	try {
		const res: any = await request.get('/tenant/level-benefits')
		levelBenefits.value = res.data?.data || null
	} catch {
		// silent
	} finally {
		levelBenefitsLoading.value = false
	}
}

onMounted(() => {
	fetchOrgInfo()
	fetchAuditConfig()
	fetchLevelBenefits()
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

		<!-- Team Feature Card（团队功能激活/管理）-->
		<div v-if="orgInfo" class="card">
			<div class="card-header">
				<div class="flex items-center justify-between">
					<div>
						<h2 class="font-semibold text-gray-900">团队功能</h2>
						<p class="text-sm text-gray-500 mt-0.5">启用后可邀请成员、创建 RAM 账号、分配成员额度</p>
					</div>
					<span class="badge" :class="orgInfo.team_enabled ? 'badge-success' : 'badge-gray'">
						{{ orgInfo.team_enabled ? '已启用' : '未启用' }}
					</span>
				</div>
			</div>
			<div class="card-body space-y-4">
				<!-- 个人模式：激活表单 -->
				<template v-if="!orgInfo.team_enabled">
					<div class="rounded-xl bg-amber-50 border border-amber-200 p-4 flex items-start gap-3">
						<Icon name="exclamationTriangle" size="md" class="text-amber-600 flex-shrink-0 mt-0.5" />
						<div class="text-sm text-amber-800">
							当前为个人模式。设置组织名称和组织代码后即可启用团队功能，成员可通过
							<code class="code">用户名@组织代码</code> 登录。
						</div>
					</div>

					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						<div>
							<label class="input-label">组织名称</label>
							<input v-model="teamForm.name" type="text" placeholder="例如：某某科技" class="input" />
						</div>
						<div>
							<label class="input-label">组织代码</label>
							<input
								v-model="teamForm.code"
								type="text"
								placeholder="例如：my-team"
								class="input"
								:class="{ 'input-error': codeError }"
								@input="codeError = ''"
							/>
							<p class="input-hint">3-30 位，小写字母、数字、连字符，字母数字开头结尾</p>
							<p v-if="codeError" class="input-error-text">{{ codeError }}</p>
						</div>
					</div>

					<div class="flex justify-end">
						<button class="btn btn-primary" :disabled="teamSaving" @click="enableTeam">
							{{ teamSaving ? '启用中...' : '启用团队功能' }}
						</button>
					</div>
				</template>

				<!-- 团队模式：管理组织代码 -->
				<template v-else>
					<div class="rounded-xl bg-emerald-50 border border-emerald-200 p-4 flex items-start gap-3">
						<Icon name="checkCircle" size="md" class="text-emerald-600 flex-shrink-0 mt-0.5" />
						<div class="text-sm text-emerald-800">
							团队功能已启用。成员通过 <code class="code">用户名@{{ orgInfo.code }}</code> 登录。
						</div>
					</div>

					<div>
						<p class="input-label">组织代码</p>
						<div v-if="!editingTeamCode" class="flex items-center gap-2">
							<span class="badge badge-gray font-mono">{{ orgInfo.code }}</span>
							<button
								@click="editingTeamCode = true; teamForm.code = orgInfo.code"
								class="text-primary-600 hover:text-primary-500 transition-colors"
							>
								<Icon name="edit" size="sm" />
							</button>
						</div>
						<div v-else class="space-y-2">
							<input
								v-model="teamForm.code"
								type="text"
								class="input"
								:class="{ 'input-error': codeError }"
								@input="codeError = ''"
							/>
							<p v-if="codeError" class="input-error-text">{{ codeError }}</p>
							<p class="input-hint">修改后 RAM 登录账号格式将同步变化，请通知成员</p>
							<div class="flex items-center gap-2">
								<button class="btn btn-primary btn-sm" :disabled="teamSaving" @click="saveTeamCode">保存</button>
								<button class="btn btn-secondary btn-sm" @click="editingTeamCode = false; codeError = ''">取消</button>
							</div>
						</div>
					</div>
				</template>
			</div>
		</div>

		<!-- Organization Info Card -->
		<div v-if="orgInfo" class="card">
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
							<span v-if="orgInfo.team_enabled" class="badge badge-gray font-mono">{{ orgInfo.code }}</span>
							<template v-else>
								<span class="badge badge-gray">未设置</span>
								<span class="input-hint">个人模式</span>
							</template>
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

		<!-- Level Benefits -->
		<div class="card">
			<div class="card-header">
				<h2 class="text-lg font-semibold text-gray-900">等级权益</h2>
				<p class="text-sm text-gray-500 mt-0.5">各等级权益对比与升级进度</p>
			</div>
			<div class="card-body space-y-5">
				<!-- Loading -->
				<div v-if="levelBenefitsLoading" class="space-y-3">
					<div class="skeleton h-10 w-full rounded-xl"></div>
					<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
						<div v-for="i in 5" :key="i" class="skeleton h-40 rounded-xl"></div>
					</div>
				</div>

				<template v-else-if="levelBenefits">
					<!-- Summary Bar -->
					<div class="flex flex-col sm:flex-row items-start sm:items-center gap-3 p-4 rounded-xl bg-primary-50/60 border border-primary-100">
						<div class="flex items-center gap-2">
							<Icon name="shield" size="md" class="text-primary-600" />
							<span class="badge badge-primary text-sm font-semibold">
								{{ levelBenefits.current_level_name || 'LV' + levelBenefits.current_level }}
							</span>
						</div>
						<div class="flex items-center gap-2 text-sm text-gray-600">
							<Icon name="currencyDollar" size="sm" class="text-gray-400" />
							<span>累计充值 <strong class="text-gray-900">${{ levelBenefits.cumulative_recharge.toFixed(2) }}</strong></span>
						</div>
						<div v-if="getNextLevelThreshold() !== null" class="flex-1 w-full sm:w-auto">
							<div class="flex items-center justify-between text-xs text-gray-500 mb-1">
								<span>升级进度</span>
								<span>{{ getUpgradePercent() }}%</span>
							</div>
							<div class="h-2 bg-primary-100 rounded-full overflow-hidden">
								<div
									class="h-full bg-gradient-to-r from-primary-400 to-primary-600 rounded-full transition-all duration-500"
									:style="{ width: getUpgradePercent() + '%' }"
								></div>
							</div>
							<p class="text-xs text-gray-400 mt-1">
								再充值 ${{ (getNextLevelThreshold()! - levelBenefits.cumulative_recharge).toFixed(2) }} 即可升级
							</p>
						</div>
						<div v-else class="text-xs text-primary-600 font-medium">
							已达最高等级
						</div>
					</div>

					<!-- Level Grid -->
					<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
						<div
							v-for="item in levelBenefits.list"
							:key="item.level"
							class="p-4 rounded-xl border-2 transition-all duration-200"
							:class="[
								item.level === levelBenefits.current_level
									? 'border-primary-500 bg-primary-50/50 shadow-glow'
									: 'border-gray-200 hover:border-gray-300',
							]"
						>
							<!-- Level Header -->
							<div class="flex items-center justify-between mb-3">
								<span class="text-sm font-semibold" :class="item.level === levelBenefits.current_level ? 'text-primary-700' : 'text-gray-900'">
									{{ item.name }}
								</span>
								<span
									v-if="item.level === levelBenefits.current_level"
									class="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-primary-500 text-white"
								>
									当前
								</span>
							</div>

							<!-- Benefits -->
							<div class="space-y-2.5">
								<div class="flex items-center gap-2">
									<Icon name="currencyDollar" size="xs" class="text-gray-400 flex-shrink-0" />
									<div>
										<p class="text-[10px] text-gray-400">升级门槛</p>
										<p class="text-xs font-medium text-gray-700">{{ formatThreshold(item.cumulative_recharge_threshold) }}</p>
									</div>
								</div>
								<div class="flex items-center gap-2">
									<Icon name="users" size="xs" class="text-gray-400 flex-shrink-0" />
									<div>
										<p class="text-[10px] text-gray-400">成员上限</p>
										<p class="text-xs font-medium text-gray-700">{{ formatMembers(item.max_members) }}</p>
									</div>
								</div>
								<div class="flex items-center gap-2">
									<Icon name="bolt" size="xs" class="text-gray-400 flex-shrink-0" />
									<div>
										<p class="text-[10px] text-gray-400">并发上限</p>
										<p class="text-xs font-medium text-gray-700">{{ formatConcurrency(item.max_concurrency) }}</p>
									</div>
								</div>
								<div class="flex items-center gap-2">
									<Icon name="shield" size="xs" class="text-gray-400 flex-shrink-0" />
									<div>
										<p class="text-[10px] text-gray-400">价格折扣</p>
										<p class="text-xs font-medium text-gray-700">{{ formatDiscount(item.price_multiplier) }}</p>
									</div>
								</div>
							</div>
						</div>
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
