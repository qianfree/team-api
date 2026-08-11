<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NButton } from 'naive-ui'
import { useRouter } from 'vue-router'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import Icon from '@/components/common/Icon.vue'
import TeamLockedBanner from '@/components/common/TeamLockedBanner.vue'
import { renderBadge } from '@/utils/renderUtils'
import request from '@/utils/request'
import { toast } from '@/utils/toast'

const router = useRouter()
const authStore = useTenantAuthStore()
const teamEnabled = computed(() => !!authStore.tenant?.team_enabled)

interface InvitationItem {
	id: number
	code: string
	role: string
	status: string
	invite_url: string
	expires_at: string
	max_uses: number
	use_count: number
	created_at: string
	creator_name: string
}

const invitations = ref<InvitationItem[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

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

const statusBadgeMap: Record<string, string> = {
	active: 'badge-primary',
	pending: 'badge-primary',
	exhausted: 'badge-success',
	used: 'badge-success',
	expired: 'badge-gray',
	revoked: 'badge-danger',
}
const statusLabelMap: Record<string, string> = {
	active: '使用中',
	pending: '待使用',
	exhausted: '已用完',
	used: '已使用',
	expired: '已过期',
	revoked: '已撤销',
}

async function fetchInvitations() {
	loading.value = true
	try {
		const res: any = await request.get('/tenant/members/invitations', {
			params: { page: page.value, page_size: pageSize.value },
		})
		const raw = res.data?.data
		invitations.value = raw?.list || []
		total.value = raw?.total || 0
	} catch {
		invitations.value = []
	} finally {
		loading.value = false
	}
}

function copyInvitationLink(url: string) {
	navigator.clipboard.writeText(url)
	toast.success('链接已复制到剪贴板')
}

async function revokeInvitation(id: number) {
	try {
		await request.delete(`/tenant/members/invitations/${id}`)
		toast.success('邀请已撤销')
		fetchInvitations()
	} catch {
	}
}

// NDataTable 列定义
const columns = computed<DataTableColumns<InvitationItem>>(() => [
	{ title: '邀请码', key: 'code', render: (row) => h('span', { class: 'font-mono text-xs' }, row.code) },
	{ title: '角色', key: 'role', render: (row) => renderBadge(row.role, roleLabel, roleBadgeClass) },
	{ title: '状态', key: 'status', render: (row) => renderBadge(row.status, statusLabelMap, statusBadgeMap) },
	{
		title: '已使用',
		key: 'use_count',
		render: (row) => h('span', { class: 'text-gray-600 text-sm' }, `${row.use_count} / ${row.max_uses === 0 ? '∞' : row.max_uses}`),
	},
	{ title: '创建者', key: 'creator_name', render: (row) => h('span', { class: 'text-gray-500' }, row.creator_name || '--') },
	{ title: '过期时间', key: 'expires_at', render: (row) => h('span', { class: 'text-gray-500 text-xs' }, row.expires_at || '永不过期') },
	{ title: '创建时间', key: 'created_at', render: (row) => h('span', { class: 'text-gray-500 text-xs' }, row.created_at) },
	{
		title: '操作',
		key: 'actions',
		align: 'right',
		render: (row) =>
			h('div', { class: 'flex items-center justify-end gap-1' }, [
				row.status === 'active' && row.invite_url
					? h(
							NButton,
							{ text: true, size: 'small', type: 'primary', onClick: () => copyInvitationLink(row.invite_url) },
							{ default: () => '复制' }
						)
					: null,
				row.status === 'active'
					? h(
							NButton,
							{ text: true, size: 'small', type: 'error', onClick: () => revokeInvitation(row.id) },
							{ default: () => '撤销' }
						)
					: null,
			]),
	},
])

function handlePageSizeChange() {
	page.value = 1
	fetchInvitations()
}

onMounted(() => {
	fetchInvitations()
})
</script>

<template>
	<TeamLockedBanner v-if="!teamEnabled" />
	<div v-else class="viewport-table-page space-y-6">
		<!-- Page Header -->
		<div class="page-header flex items-center justify-between">
			<div>
				<div class="flex items-center gap-2 mb-1">
					<button
						@click="router.push('/tenant/members')"
						class="btn btn-ghost btn-sm text-gray-500 hover:text-gray-700 -ml-2"
					>
						<Icon name="chevronLeft" size="sm" />
						成员管理
					</button>
				</div>
				<h1 class="page-title">邀请记录</h1>
				<p class="page-description">查看和管理邀请链接</p>
			</div>
		</div>

		<!-- Table -->
		<div class="viewport-table-panel">
			<n-data-table
				remote
				v-model:page="page"
				v-model:page-size="pageSize"
				:item-count="total"
				:page-sizes="[10, 20, 50, 100]"
				show-size-picker
				:loading="loading"
				:columns="columns"
				:data="invitations"
				:row-key="(row: InvitationItem) => row.id"
				@update:page="fetchInvitations"
				@update:page-size="handlePageSizeChange"
			>
				<template #empty>
					<div class="empty-state">
						<Icon name="document" size="xl" class="empty-state-icon" />
						<p class="empty-state-title">暂无邀请记录</p>
						<p class="empty-state-description">生成邀请链接后，记录会显示在这里</p>
					</div>
				</template>
			</n-data-table>
		</div>
	</div>
</template>
