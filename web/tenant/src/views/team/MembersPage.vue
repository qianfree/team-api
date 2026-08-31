<script setup lang="ts">
import { ref, reactive, onMounted, computed, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NTag } from 'naive-ui'
import { useRouter } from 'vue-router'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import BaseModal from '@/components/common/BaseModal.vue'
import ResponsiveDataTable from '@/components/common/ResponsiveDataTable.vue'
import TeamLockedBanner from '@/components/common/TeamLockedBanner.vue'
import BaseSelect from '../../components/common/BaseSelect.vue'
import TableFilterForm, { type FilterField } from '@/components/common/TableFilterForm.vue'
import Icon from '@/components/common/Icon.vue'
import { renderBadge, BADGE_TYPE_MAP, tableScrollX, formatMoney, formatDate } from '@/utils/renderUtils'
import request from '@/utils/request'
import { toast } from '@/utils/toast'
import { useExport } from '@/composables/useExport'

const router = useRouter()
const authStore = useTenantAuthStore()
const teamEnabled = computed(() => !!authStore.tenant?.team_enabled)

// 查询表单数据
const filters = ref({
	keyword: '',
})

// 查询表单字段配置
const filterFields: FilterField[] = [
	{
		type: 'input',
		key: 'keyword',
		label: '关键词',
		placeholder: '搜索用户名/邮箱/显示名',
		width: '240px',
	},
]

const showMoreMenu = ref(false)
const { exporting, exportFile } = useExport({
	url: '/tenant/members/export',
	getFilters: () => ({
		keyword: filters.value.keyword,
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
	// 额度限制
	quota_type: string
	quota_limit: number
	quota_period: string
	// 可用模型数
	model_count: number
	model_unlimited: boolean
	// 本月消费（本位币）
	month_cost: number
	// 最后更新时间
	updated_at: string
}


const members = ref<Member[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

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
			params: { page: page.value, page_size: pageSize.value, keyword: filters.value.keyword },
		})
		const raw = res.data?.data; members.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
		total.value = raw?.total || 0
	} catch {
		members.value = []
	} finally {
		loading.value = false
	}
}

function applyFilters() {
	page.value = 1
	fetchMembers()
}

function resetFilters() {
	filters.value.keyword = ''
	page.value = 1
	fetchMembers()
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

const periodLabel: Record<string, string> = {
	day: '按天',
	week: '按周',
	month: '按月',
}

// 额度限制列渲染：不限制 / 金额（周期性追加周期徽章）
function renderQuota(row: Member) {
	if (!row.quota_type || row.quota_type === 'none') {
		return h(NTag, { size: 'small', type: 'info', bordered: false }, { default: () => '不限制' })
	}
	const children: ReturnType<typeof h>[] = [
		h('span', { class: 'text-sm font-medium text-gray-900' }, formatMoney(row.quota_limit, { precision: 2 })),
	]
	if (row.quota_type === 'periodic' && row.quota_period) {
		children.push(
			h(NTag, { size: 'small', type: 'warning', bordered: false }, { default: () => periodLabel[row.quota_period] || row.quota_period })
		)
	}
	return h('div', { class: 'flex items-center gap-1.5' }, children)
}

// 可用模型数列渲染：不限 / 授权数量
function renderModelCount(row: Member) {
	if (row.model_unlimited) {
		return h(NTag, { size: 'small', type: 'info', bordered: false }, { default: () => '不限' })
	}
	return h('span', { class: 'text-sm font-medium text-gray-900' }, String(row.model_count))
}

// NDataTable 列定义
const columns = computed<DataTableColumns<Member>>(() => [
	{
		title: '用户',
		key: 'user',
		width: 200,
		render: (row) =>
			h('div', { class: 'flex items-center gap-3' }, [
				h(
					'div',
					{
						class:
							'h-9 w-9 rounded-full flex items-center justify-center text-white text-sm font-medium flex-shrink-0 bg-gradient-to-r from-primary-500 to-primary-600',
					},
					(row.username || '').charAt(0).toUpperCase()
				),
				h('div', {}, [
					h('p', { class: 'text-sm font-medium text-gray-900' }, row.display_name || row.username),
					h('p', { class: 'text-xs text-gray-500' }, row.email || '--'),
				]),
			]),
	},
	{
		title: '角色',
		key: 'role',
		width: 110,
		render: (row) => renderBadge(row.role, roleLabel, roleBadgeClass),
	},
	{
		title: '状态',
		key: 'status',
		width: 110,
		render: (row) =>
			h(
				NTag,
				{ size: 'small', type: BADGE_TYPE_MAP[statusBadgeClass[row.status] || 'badge-gray'] || 'default' },
				{
					icon: () =>
						h('span', {
							class: ['h-1.5 w-1.5 rounded-full', statusDotClass[row.status] || 'bg-gray-400'],
						}),
					default: () => statusLabel[row.status] || row.status,
				}
			),
	},
	{
		title: '额度限制',
		key: 'quota_type',
		width: 130,
		render: (row) => renderQuota(row),
	},
	{
		title: '可用模型',
		key: 'model_count',
		width: 100,
		render: (row) => renderModelCount(row),
	},
	{
		title: '本月消费',
		key: 'month_cost',
		width: 120,
		render: (row) => h('span', { class: 'text-sm font-semibold text-primary-600' }, formatMoney(row.month_cost, { precision: 2 })),
	},
	{
		title: '加入时间',
		key: 'created_at',
		width: 170,
		render: (row) => row.created_at || '--',
	},
	{
		title: '最后更新',
		key: 'updated_at',
		width: 170,
		render: (row) => formatDate(row.updated_at),
	},
	{
		title: '操作',
		key: 'actions',
		width: 120,
		align: 'right',
		render: (row) =>
			h(
				NButton,
				{
					text: true,
					type: 'primary',
					size: 'small',
					onClick: (e: MouseEvent) => {
						e.stopPropagation()
						goDetail(row.id)
					},
				},
				{ default: () => '查看详情' }
			),
	},
])

// pageSize 变化回第 1 页并刷新
function handlePageSizeChange() {
	page.value = 1
	fetchMembers()
}

onMounted(() => {
	fetchMembers()
})
</script>

<template>
	<TeamLockedBanner v-if="!teamEnabled" />
	<div v-else class="viewport-table-page space-y-6">
		<!-- Page Header -->
		<div class="page-header flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
			<div class="min-w-0">
				<h1 class="page-title">成员管理</h1>
				<p class="page-description">管理组织团队中的成员</p>
			</div>
			<!-- 桌面端操作区 -->
			<div class="hidden lg:flex flex-wrap items-center gap-2">
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
			</div>
			<!-- 移动端操作区 -->
			<div class="flex lg:hidden items-center gap-2">
				<button class="btn btn-primary flex-1" @click="showInviteModal = true">
					<Icon name="userPlus" size="sm" />
					邀请成员
				</button>
				<div class="relative">
					<button
						class="btn btn-secondary"
						aria-haspopup="true"
						:aria-expanded="showMoreMenu"
						@click="showMoreMenu = !showMoreMenu"
					>
						<Icon name="more" size="sm" />
						更多
					</button>
					<div v-if="showMoreMenu" class="fixed inset-0 z-40" @click="showMoreMenu = false"></div>
					<div v-if="showMoreMenu" class="dropdown right-0 mt-2 w-44">
						<router-link to="/tenant/members/invitations" class="dropdown-item" @click="showMoreMenu = false">
							<Icon name="document" size="sm" />
							邀请记录
						</router-link>
						<div class="dropdown-item" @click="downloadTemplate(); showMoreMenu = false">
							<Icon name="document" size="sm" />
							导入模板
						</div>
						<div class="dropdown-item" @click="showImportModal = true; showMoreMenu = false">
							<Icon name="plus" size="sm" />
							批量导入
						</div>
						<div class="dropdown-item" @click="showCreateModal = true; showMoreMenu = false">
							<Icon name="userPlus" size="sm" />
							创建成员
						</div>
						<div class="dropdown-item" @click="exportFile('csv'); showMoreMenu = false">
							<Icon name="download" size="sm" />
							导出 CSV
						</div>
						<div class="dropdown-item" @click="exportFile('xlsx'); showMoreMenu = false">
							<Icon name="download" size="sm" />
							导出 Excel
						</div>
					</div>
				</div>
			</div>
		</div>

		<!-- Search & Export -->
		<TableFilterForm
			v-model="filters"
			:fields="filterFields"
			:loading="loading"
			:show-export="true"
			:exporting="exporting"
			@search="applyFilters"
			@reset="resetFilters"
			@export="exportFile"
		/>

		<!-- Members Table -->
		<div class="viewport-table-panel">
			<ResponsiveDataTable
				remote
				fill-height
				v-model:page="page"
				v-model:page-size="pageSize"
				:item-count="total"
				:page-sizes="[10, 20, 50, 100]"
				show-size-picker
				:loading="loading"
				:columns="columns"
				:scroll-x="tableScrollX(columns)"
				:data="members"
				:row-key="(row: Member) => row.id"
				card-title-key="user"
				card-badge-key="status"
				card-subtitle-key="created_at"
				:card-fields="['role', 'quota_type', 'model_count', 'month_cost', 'updated_at']"
				card-actions-key="actions"
				:row-click="(row: Member) => goDetail(row.id)"
				@update:page="fetchMembers"
				@update:page-size="handlePageSizeChange"
			>
				<template #empty>
					<div class="empty-state">
						<Icon name="users" size="xl" class="empty-state-icon" />
						<p class="empty-state-title">暂无成员</p>
						<p class="empty-state-description">邀请第一位团队成员吧</p>
					</div>
				</template>
			</ResponsiveDataTable>
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
						<BaseSelect v-model="inviteForm.role" :options="[{value:'member',label:'成员'},{value:'admin',label:'管理员'}]" />
					</div>

					<div>
						<label class="input-label">有效期</label>
						<BaseSelect v-model="inviteForm.expires_days" :options="[{value:1,label:'1 天'},{value:3,label:'3 天'},{value:7,label:'7 天'},{value:14,label:'14 天'},{value:30,label:'30 天'}]" />
					</div>

					<div>
						<label class="input-label">使用次数</label>
						<BaseSelect v-model="inviteForm.max_uses" :options="[{value:0,label:'不限次数'},{value:1,label:'1 次'},{value:5,label:'5 次'},{value:10,label:'10 次'},{value:20,label:'20 次'},{value:50,label:'50 次'}]" />
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
					<n-input
						v-model:value="createForm.username"
						type="text"
						placeholder="3-50 位字符"
					/>
				</div>

				<div>
					<label class="input-label">邮箱</label>
					<n-input
						v-model:value="createForm.email"
						type="email"
						placeholder="选填"
					/>
				</div>

				<div>
					<label class="input-label">密码 <span class="text-red-500">*</span></label>
					<n-input
						v-model:value="createForm.password"
						type="password"
						show-password-on="click"
						placeholder="至少 8 位，含字母和数字"
					/>
				</div>

				<div>
					<label class="input-label">显示名称</label>
					<n-input
						v-model:value="createForm.display_name"
						type="text"
						placeholder="选填"
					/>
				</div>

				<div>
					<label class="input-label">角色</label>
					<BaseSelect v-model="createForm.role" :options="[{value:'member',label:'成员'},{value:'admin',label:'管理员'}]" />
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
