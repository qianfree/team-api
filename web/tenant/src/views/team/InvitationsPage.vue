<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import Icon from '@/components/common/Icon.vue'
import TeamLockedBanner from '@/components/common/TeamLockedBanner.vue'
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
const pageSize = 20
const totalPages = computed(() => Math.ceil(total.value / pageSize))

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
			params: { page: page.value, page_size: pageSize },
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

onMounted(() => {
	fetchInvitations()
})
</script>

<template>
	<TeamLockedBanner v-if="!teamEnabled" />
	<div v-else class="space-y-6">
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
		<div class="card">
			<div v-if="loading" class="p-8 flex justify-center">
				<div class="spinner h-6 w-6 border-primary-500"></div>
			</div>

			<div v-else-if="invitations.length > 0" class="table-container">
				<table class="table">
					<thead>
						<tr>
							<th>邀请码</th>
							<th>角色</th>
							<th>状态</th>
							<th>已使用</th>
							<th>创建者</th>
							<th>过期时间</th>
							<th>创建时间</th>
							<th class="text-right">操作</th>
						</tr>
					</thead>
					<tbody>
						<tr v-for="inv in invitations" :key="inv.id">
							<td class="font-mono text-xs">{{ inv.code }}</td>
							<td>
								<span class="badge" :class="roleBadgeClass[inv.role]">{{ roleLabel[inv.role] || inv.role }}</span>
							</td>
							<td>
								<span class="badge" :class="statusBadgeMap[inv.status] || 'badge-gray'">
									{{ statusLabelMap[inv.status] || inv.status }}
								</span>
							</td>
							<td class="text-gray-600 text-sm">
								{{ inv.use_count }} / {{ inv.max_uses === 0 ? '∞' : inv.max_uses }}
							</td>
							<td class="text-gray-500">{{ inv.creator_name || '--' }}</td>
							<td class="text-gray-500 text-xs">{{ inv.expires_at || '永不过期' }}</td>
							<td class="text-gray-500 text-xs">{{ inv.created_at }}</td>
							<td>
								<div class="flex items-center justify-end gap-1">
									<button
										v-if="inv.status === 'active' && inv.invite_url"
										@click="copyInvitationLink(inv.invite_url)"
										class="btn btn-ghost btn-sm text-primary-600"
									>
										<Icon name="copy" size="xs" />
										复制
									</button>
									<button
										v-if="inv.status === 'active'"
										@click="revokeInvitation(inv.id)"
										class="btn btn-ghost btn-sm text-red-600 hover:text-red-700 hover:bg-red-50"
									>
										撤销
									</button>
								</div>
							</td>
						</tr>
					</tbody>
				</table>
			</div>

			<!-- Empty state -->
			<div v-else class="empty-state">
				<Icon name="document" size="xl" class="empty-state-icon" />
				<p class="empty-state-title">暂无邀请记录</p>
				<p class="empty-state-description">生成邀请链接后，记录会显示在这里</p>
			</div>

			<!-- Pagination -->
			<div v-if="totalPages > 1" class="flex items-center justify-between px-6 py-4 border-t border-gray-100">
				<p class="text-xs text-gray-500">共 {{ total }} 条记录</p>
				<div class="flex items-center gap-2">
					<button
						class="btn btn-secondary btn-sm"
						:disabled="page <= 1"
						@click="page--; fetchInvitations()"
					>上一页</button>
					<span class="text-sm text-gray-600">{{ page }} / {{ totalPages }}</span>
					<button
						class="btn btn-secondary btn-sm"
						:disabled="page >= totalPages"
						@click="page++; fetchInvitations()"
					>下一页</button>
				</div>
			</div>
		</div>
	</div>
</template>
