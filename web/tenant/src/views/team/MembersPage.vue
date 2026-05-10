<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import BaseModal from '@/components/common/BaseModal.vue'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'
import { toast } from '@/utils/toast'
import { useExport } from '@/composables/useExport'

const router = useRouter()

const showExportDropdown = ref(false)
const { exporting, exportFile } = useExport({
	url: '/tenant/members/export',
	getFilters: () => ({
		keyword: keyword.value,
	}),
})

interface Member {
	id: number
	username: string
	email: string
	display_name: string
	role: string
	status: string
	created_at: string
}


const members = ref<Member[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = 20
const keyword = ref('')

const showInviteModal = ref(false)
const inviteForm = reactive({
	role: 'member' as 'admin' | 'member',
	expires_days: 7,
	max_uses: 0,
})
const inviteLink = ref('')
const inviteLoading = ref(false)

// Create member
const showCreateModal = ref(false)
const createLoading = ref(false)
const createForm = reactive({
	username: '',
	password: '',
	email: '',
	display_name: '',
	role: 'member' as 'admin' | 'member',
})

const roleBadgeClass: Record<string, string> = {
	owner: 'badge-primary',
	admin: 'badge-warning',
	member: 'badge-gray',
}

const roleLabel: Record<string, string> = {
	owner: '所有者',
	admin: '管理员',
	member: '成员',
}

const statusBadgeClass: Record<string, string> = {
	active: 'badge-success',
	invited: 'badge-primary',
	disabled: 'badge-gray',
}

const statusLabel: Record<string, string> = {
	active: '已激活',
	invited: '已邀请',
	disabled: '已禁用',
}

const statusDotClass: Record<string, string> = {
	active: 'bg-emerald-500',
	invited: 'bg-primary-500',
	disabled: 'bg-gray-400',
}

async function fetchMembers() {
	loading.value = true
	try {
		const res: any = await request.get('/tenant/members', {
			params: { page: page.value, page_size: pageSize, keyword: keyword.value },
		})
		const raw = res.data?.data; members.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
		total.value = raw?.total || 0
	} catch {
		members.value = []
	} finally {
		loading.value = false
	}
}

async function generateInviteLink() {
	inviteLoading.value = true
	try {
		const res: any = await request.post('/tenant/members/invite', {
			role: inviteForm.role,
			expires_days: inviteForm.expires_days,
			max_uses: inviteForm.max_uses,
		})
		const raw = res.data?.data
		inviteLink.value = raw?.invite_url || ''
	} catch {
	} finally {
		inviteLoading.value = false
	}
}

function copyInviteLink() {
	if (inviteLink.value) {
		navigator.clipboard.writeText(inviteLink.value)
		toast.success('链接已复制到剪贴板')
	}
}

// Batch import
const showImportModal = ref(false)
const importLoading = ref(false)
const importResult = ref<any>(null)
const importFile = ref<File | null>(null)

function handleImportFile(e: Event) {
	const input = e.target as HTMLInputElement
	if (input.files?.length) {
		importFile.value = input.files[0]
	}
}

async function handleImport() {
	if (!importFile.value) return
	importLoading.value = true
	importResult.value = null
	try {
		const formData = new FormData()
		formData.append('file', importFile.value)
		const res: any = await request.post('/tenant/members/import', formData)
		importResult.value = res.data?.data
	} catch {
	} finally {
		importLoading.value = false
	}
}

function downloadTemplate() {
	const csv = 'username,display_name,email,role,models\nalice,Alice Chen,alice@example.com,member,\nbob,Bob Wang,bob@example.com,admin,gpt-4;claude-3'
	const blob = new Blob([csv], { type: 'text/csv' })
	const url = URL.createObjectURL(blob)
	const a = document.createElement('a')
	a.href = url
	a.download = 'member_import_template.csv'
	a.click()
	URL.revokeObjectURL(url)
}

function closeInviteModal() {
	showInviteModal.value = false
	inviteForm.role = 'member'
	inviteForm.expires_days = 7
	inviteForm.max_uses = 0
	inviteLink.value = ''
}

async function handleCreateMember() {
	createLoading.value = true
	try {
		await request.post('/tenant/members/create', {
			username: createForm.username,
			password: createForm.password,
			email: createForm.email,
			display_name: createForm.display_name || undefined,
			role: createForm.role,
		})
		toast.success('成员创建成功')
		closeCreateModal()
		fetchMembers()
	} catch {
	} finally {
		createLoading.value = false
	}
}

function closeCreateModal() {
	showCreateModal.value = false
	createForm.username = ''
	createForm.password = ''
	createForm.email = ''
	createForm.display_name = ''
	createForm.role = 'member'
}

function goDetail(memberId: number) {
	router.push(`/tenant/members/${memberId}`)
}

onMounted(() => {
	fetchMembers()
})
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="page-header flex items-center justify-between">
			<div>
				<h1 class="page-title">成员管理</h1>
				<p class="page-description">管理组织团队中的成员</p>
			</div>
			<div class="flex items-center gap-2">
				<router-link to="/tenant/members/invitations" class="btn btn-secondary">
					<Icon name="document" size="sm" />
					邀请记录
				</router-link>
				<button class="btn btn-secondary" @click="downloadTemplate">
					<Icon name="document" size="sm" />
					导入模板
				</button>
				<button class="btn btn-secondary" @click="showImportModal = true">
					<Icon name="plus" size="sm" />
					批量导入
				</button>
				<button class="btn btn-secondary" @click="showCreateModal = true">
					<Icon name="userPlus" size="sm" />
					创建成员
				</button>
				<button class="btn btn-primary" @click="showInviteModal = true">
					<Icon name="userPlus" size="sm" />
					邀请成员
				</button>
				<!-- Export dropdown -->
				<div class="relative inline-block">
					<button class="btn btn-secondary" :disabled="exporting" @click="showExportDropdown = !showExportDropdown">
						<svg v-if="!exporting" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3"/></svg>
						<svg v-else class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
						导出
					</button>
					<div v-if="showExportDropdown" class="absolute right-0 mt-2 w-36 bg-white rounded-xl border shadow-lg py-1 z-50">
						<div class="px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 cursor-pointer" @click="exportFile('csv'); showExportDropdown = false">导出 CSV</div>
						<div class="px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 cursor-pointer" @click="exportFile('xlsx'); showExportDropdown = false">导出 Excel</div>
					</div>
				</div>
			</div>
		</div>

		<!-- Members Table -->
		<div class="card">
			<div v-if="loading" class="p-8 flex justify-center">
				<div class="spinner h-6 w-6 border-primary-500"></div>
			</div>

			<div v-else-if="members.length > 0" class="table-container">
				<table class="table">
					<thead>
						<tr>
							<th>用户</th>
							<th>角色</th>
							<th>状态</th>
							<th>加入时间</th>
							<th class="text-right">操作</th>
						</tr>
					</thead>
					<tbody>
						<tr
							v-for="member in members"
							:key="member.id"
							class="cursor-pointer hover:bg-primary-50/50 transition-colors"
							@click="goDetail(member.id)"
						>
							<!-- User -->
							<td>
								<div class="flex items-center gap-3">
									<div
										class="h-9 w-9 rounded-full flex items-center justify-center text-white text-sm font-medium flex-shrink-0 bg-gradient-to-r from-primary-500 to-primary-600"
									>
										{{ member.username.charAt(0).toUpperCase() }}
									</div>
									<div>
										<p class="text-sm font-medium text-gray-900">{{ member.display_name || member.username }}</p>
										<p class="text-xs text-gray-500">{{ member.email || '--' }}</p>
									</div>
								</div>
							</td>

							<!-- Role -->
							<td>
								<span class="badge" :class="roleBadgeClass[member.role]">
									{{ roleLabel[member.role] || member.role }}
								</span>
							</td>

							<!-- Status -->
							<td>
								<span class="badge" :class="statusBadgeClass[member.status] || 'badge-gray'">
									<span class="h-1.5 w-1.5 rounded-full" :class="statusDotClass[member.status] || 'bg-gray-400'"></span>
									{{ statusLabel[member.status] || member.status }}
								</span>
							</td>

							<!-- Joined -->
							<td class="text-gray-500">
								{{ member.created_at || '--' }}
							</td>

							<!-- Actions -->
							<td>
								<div class="flex items-center justify-end">
									<button
										@click.stop="goDetail(member.id)"
										class="btn btn-ghost btn-sm text-primary-600 hover:text-primary-700 hover:bg-primary-50"
									>
										查看详情
										<Icon name="chevronRight" size="xs" />
									</button>
								</div>
							</td>
						</tr>
					</tbody>
				</table>
			</div>

			<!-- Empty state -->
			<div v-else class="empty-state">
				<Icon name="users" size="xl" class="empty-state-icon" />
				<p class="empty-state-title">暂无成员</p>
				<p class="empty-state-description">邀请第一位团队成员吧</p>
			</div>
		</div>

		<!-- Invite Modal -->
		<BaseModal
			:show="showInviteModal"
			title="邀请团队成员"
			width="narrow"
			@close="closeInviteModal"
		>
			<template #default>
				<p class="text-sm text-gray-500 mb-5">生成邀请链接，分发给团队成员完成注册</p>

				<div class="space-y-4">
					<div>
						<label class="input-label">角色</label>
						<select v-model="inviteForm.role" class="input bg-white">
							<option value="member">成员</option>
							<option value="admin">管理员</option>
						</select>
					</div>

					<div>
						<label class="input-label">有效期</label>
						<select v-model="inviteForm.expires_days" class="input bg-white">
							<option :value="1">1 天</option>
							<option :value="3">3 天</option>
							<option :value="7">7 天</option>
							<option :value="14">14 天</option>
							<option :value="30">30 天</option>
						</select>
					</div>

					<div>
						<label class="input-label">使用次数</label>
						<select v-model="inviteForm.max_uses" class="input bg-white">
							<option :value="0">不限次数</option>
							<option :value="1">1 次</option>
							<option :value="5">5 次</option>
							<option :value="10">10 次</option>
							<option :value="20">20 次</option>
							<option :value="50">50 次</option>
						</select>
					</div>

					<button
						v-if="!inviteLink"
						@click="generateInviteLink"
						:disabled="inviteLoading"
						class="btn btn-primary w-full"
					>
						{{ inviteLoading ? '生成中...' : '生成邀请链接' }}
					</button>

					<!-- Invite Link Display -->
					<div v-if="inviteLink" class="p-4 bg-primary-50/50 rounded-xl border border-primary-200">
						<p class="text-xs font-medium text-primary-700 mb-2">邀请链接已生成</p>
						<div class="flex items-start gap-2">
							<p class="flex-1 text-sm font-mono text-gray-700 break-all bg-white rounded-lg px-3 py-2 border border-gray-200">{{ inviteLink }}</p>
							<button
								@click="copyInviteLink"
								class="btn btn-primary btn-sm flex-shrink-0"
							>
								<Icon name="copy" size="xs" />
								复制
							</button>
						</div>
						<p class="text-xs text-gray-500 mt-2">有效期 {{ inviteForm.expires_days }} 天，角色：{{ roleLabel[inviteForm.role] || inviteForm.role }}，使用次数：{{ inviteForm.max_uses === 0 ? '不限' : inviteForm.max_uses + ' 次' }}</p>
					</div>

					<div class="pt-2 border-t border-gray-100">
						<button
							@click="closeInviteModal(); router.push('/tenant/members/invitations')"
							class="btn btn-ghost btn-sm text-gray-500 hover:text-primary-600 w-full"
						>
							<Icon name="document" size="xs" />
							查看邀请记录
						</button>
					</div>
				</div>
			</template>
		</BaseModal>


		<!-- Create Member Modal -->
		<BaseModal
			:show="showCreateModal"
			title="创建成员"
			width="narrow"
			@close="closeCreateModal"
		>
			<div class="space-y-4">
				<p class="text-sm text-gray-500">直接创建一个成员账号，成员可使用此账号登录租户控制台。</p>

				<div>
					<label class="input-label">用户名 <span class="text-red-500">*</span></label>
					<input
						v-model="createForm.username"
						type="text"
						placeholder="3-50 位字符"
						class="input"
					/>
				</div>

				<div>
					<label class="input-label">邮箱</label>
					<input
						v-model="createForm.email"
						type="email"
						placeholder="选填"
						class="input"
					/>
				</div>

				<div>
					<label class="input-label">密码 <span class="text-red-500">*</span></label>
					<input
						v-model="createForm.password"
						type="password"
						placeholder="至少 8 位，含大小写字母和数字"
						class="input"
					/>
				</div>

				<div>
					<label class="input-label">显示名称</label>
					<input
						v-model="createForm.display_name"
						type="text"
						placeholder="选填"
						class="input"
					/>
				</div>

				<div>
					<label class="input-label">角色</label>
					<select v-model="createForm.role" class="input bg-white">
						<option value="member">成员</option>
						<option value="admin">管理员</option>
					</select>
				</div>
			</div>
			<template #footer>
				<div class="flex justify-end gap-3">
					<button class="btn btn-secondary" @click="closeCreateModal">取消</button>
					<button
						class="btn btn-primary"
						:disabled="createLoading || !createForm.username || !createForm.password"
						@click="handleCreateMember"
					>
						{{ createLoading ? '创建中...' : '创建' }}
					</button>
				</div>
			</template>
		</BaseModal>

		<!-- Import Modal -->
		<BaseModal :show="showImportModal" title="批量导入成员" width="normal" @close="showImportModal = false; importResult = null; importFile = null">
			<div class="space-y-4">
				<p class="text-sm text-gray-500">上传 CSV 文件批量导入成员。单次最多 500 条。</p>
				<div v-if="!importResult">
					<label class="input-label">CSV 文件 <span class="text-red-500">*</span></label>
					<input type="file" accept=".csv" class="input" @change="handleImportFile" />
					<p class="input-hint">格式：username, display_name, email, role, models</p>
				</div>
				<div v-if="importResult" class="space-y-3">
					<div class="flex items-center gap-4 text-sm">
						<span class="text-emerald-600 font-medium">成功：{{ importResult.success_count }}</span>
						<span class="text-red-600 font-medium">失败：{{ importResult.fail_count }}</span>
						<span class="text-gray-500">跳过：{{ importResult.skip_count }}</span>
					</div>
					<div v-if="importResult.details" class="max-h-48 overflow-y-auto text-xs">
						<div v-for="(d, i) in importResult.details" :key="i" class="flex items-center gap-2 py-1 border-b border-gray-50">
							<span class="text-gray-400 w-6">#{{ d.row }}</span>
							<span class="w-24 truncate">{{ d.username }}</span>
							<span :class="d.status === 'success' ? 'text-emerald-600' : d.status === 'skip' ? 'text-amber-600' : 'text-red-600'" >
								{{ d.status === 'success' ? '成功' : d.status === 'skip' ? '跳过' : '失败' }}
							</span>
							<span v-if="d.error" class="text-gray-400 truncate">{{ d.error }}</span>
						</div>
					</div>
				</div>
			</div>
			<template #footer>
				<div class="flex justify-end gap-3">
					<button class="btn btn-secondary" @click="showImportModal = false; importResult = null; importFile = null">关闭</button>
					<button v-if="!importResult" class="btn btn-primary" :disabled="importLoading || !importFile" @click="handleImport">{{ importLoading ? '导入中...' : '开始导入' }}</button>
				</div>
			</template>
		</BaseModal>
	</div>
</template>
