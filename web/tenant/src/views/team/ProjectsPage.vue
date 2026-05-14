<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import Icon from '@/components/common/Icon.vue'
import BaseModal from '@/components/common/BaseModal.vue'
import request from '@/utils/request'
import { useConfirm } from '@/composables/useConfirm'

const { confirm } = useConfirm()

const router = useRouter()
const loading = ref(false)
const projects = ref<any[]>([])
const page = ref(1)
const pageSize = 20
const total = ref(0)

const statusLabels: Record<string, string> = {
	active: '活跃', archived: '已归档', budget_exhausted: '预算耗尽',
}
const statusBadgeClasses: Record<string, string> = {
	active: 'badge-success', archived: 'badge-gray', budget_exhausted: 'badge-danger',
}

// Create/Edit modal
const showModal = ref(false)
const formLoading = ref(false)
const editingId = ref<number | null>(null)
const form = ref({ name: '', description: '', budget: '' as string })

function openCreate() {
	editingId.value = null
	form.value = { name: '', description: '', budget: '' }
	showModal.value = true
}

function openEdit(item: any) {
	editingId.value = item.id
	form.value = { name: item.name, description: item.description || '', budget: item.budget ? String(item.budget) : '' }
	showModal.value = true
}

async function handleSubmit() {
	if (!form.value.name.trim()) return
	formLoading.value = true
	try {
		const data: any = { name: form.value.name, description: form.value.description }
		if (form.value.budget && Number(form.value.budget) > 0) {
			data.budget = Number(form.value.budget)
		} else {
			data.budget = 0
		}
		if (editingId.value) {
			await request.put(`/tenant/projects/${editingId.value}`, data)
		} else {
			await request.post('/tenant/projects', data)
		}
		showModal.value = false
		await fetchProjects()
	} catch {
	} finally {
		formLoading.value = false
	}
}

async function handleArchive(item: any) {
	if (!await confirm({ message: `确定归档项目「${item.name}」？归档后所有 API Key 将失效。`, confirmText: '确认归档', danger: true })) return
	try {
		await request.post(`/tenant/projects/${item.id}/archive`)
		await fetchProjects()
	} catch {
	}
}

async function handleUnarchive(item: any) {
	try {
		await request.post(`/tenant/projects/${item.id}/unarchive`)
		await fetchProjects()
	} catch {
	}
}

async function fetchProjects() {
	loading.value = true
	try {
		const res: any = await request.get('/tenant/projects', {
			params: { page: page.value, page_size: pageSize }
		})
		const raw = res.data?.data
		projects.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
		total.value = raw?.total || 0
	} catch {
		// ignore
	} finally {
		loading.value = false
	}
}

onMounted(fetchProjects)

function goToDetail(item: any) {
	router.push(`/tenant/projects/${item.id}`)
}
</script>

<template>
	<div>
		<div class="page-header flex items-center justify-between">
			<div>
				<h1 class="page-title">项目管理</h1>
				<p class="page-description">创建和管理项目，设置预算上限</p>
			</div>
			<button class="btn btn-primary" @click="openCreate">
				<Icon name="plus" size="sm" />
				创建项目
			</button>
		</div>

		<div class="card">
			<div v-if="loading" class="flex items-center justify-center py-12">
				<div class="spinner h-6 w-6 text-primary-500"></div>
			</div>
			<div v-else-if="projects.length === 0" class="empty-state">
				<Icon name="project" size="xl" class="empty-state-icon" />
				<h3 class="empty-state-title">暂无项目</h3>
				<p class="empty-state-description">创建项目来组织你的 API Key 和资源</p>
			</div>
			<div v-else class="table-container">
				<table class="table">
					<thead>
						<tr>
							<th>项目名称</th>
							<th>描述</th>
							<th>状态</th>
							<th>预算上限</th>
							<th>创建时间</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						<tr v-for="p in projects" :key="p.id" class="cursor-pointer" @click="goToDetail(p)">
							<td class="font-medium text-gray-900">{{ p.name }}</td>
							<td class="text-gray-500 max-w-[200px] truncate">{{ p.description || '-' }}</td>
							<td>
								<span class="badge" :class="statusBadgeClasses[p.status] || 'badge-gray'">
									{{ statusLabels[p.status] || p.status }}
								</span>
							</td>
							<td class="font-mono">${{ p.budget ? Number(p.budget).toFixed(2) : '不限' }}</td>
							<td class="text-xs text-gray-400">{{ p.created_at?.substring(0, 16) }}</td>
							<td>
								<div class="flex items-center gap-1" @click.stop>
									<button class="btn-ghost btn-icon btn-sm" title="详情" @click="goToDetail(p)">
										<Icon name="eye" size="sm" class="text-primary-500" />
									</button>
									<button v-if="p.status === 'active'" class="btn-ghost btn-icon btn-sm" title="编辑" @click="openEdit(p)">
										<Icon name="edit" size="sm" class="text-gray-400" />
									</button>
									<button v-if="p.status === 'active'" class="btn-ghost btn-icon btn-sm" title="归档" @click="handleArchive(p)">
										<Icon name="x" size="sm" class="text-amber-500" />
									</button>
									<button v-if="p.status === 'archived'" class="btn-ghost btn-icon btn-sm" title="取消归档" @click="handleUnarchive(p)">
										<Icon name="refresh" size="sm" class="text-primary-500" />
									</button>
								</div>
							</td>
						</tr>
					</tbody>
				</table>
			</div>

			<!-- Pagination -->
			<div v-if="total > pageSize" class="card-footer flex justify-end">
				<div class="flex items-center gap-2">
					<button class="btn btn-ghost btn-sm" :disabled="page <= 1" @click="page--; fetchProjects()">上一页</button>
					<span class="text-sm text-gray-500">{{ page }} / {{ Math.ceil(total / pageSize) }}</span>
					<button class="btn btn-ghost btn-sm" :disabled="page * pageSize >= total" @click="page++; fetchProjects()">下一页</button>
				</div>
			</div>
		</div>

		<!-- Create/Edit Modal -->
		<BaseModal :show="showModal" :title="editingId ? '编辑项目' : '创建项目'" width="normal" @close="showModal = false">
			<div class="space-y-4">
				<div>
					<label class="input-label">项目名称 <span class="text-red-500">*</span></label>
					<input v-model="form.name" type="text" class="input" placeholder="输入项目名称" />
				</div>
				<div>
					<label class="input-label">描述</label>
					<textarea v-model="form.description" class="input" rows="2" placeholder="项目描述（选填）"></textarea>
				</div>
				<div>
					<label class="input-label">预算上限</label>
					<input v-model="form.budget" type="number" step="0.01" min="0" class="input" placeholder="0 = 不限制" />
					<p class="input-hint">设为 0 表示不限制。达到预算上限后，项目下所有 Key 将停止服务。</p>
				</div>
			</div>
			<template #footer>
				<div class="flex justify-end gap-3">
					<button class="btn btn-secondary" @click="showModal = false">取消</button>
					<button class="btn btn-primary" :disabled="formLoading || !form.name.trim()" @click="handleSubmit">
						{{ formLoading ? '保存中...' : (editingId ? '保存' : '创建') }}
					</button>
				</div>
			</template>
		</BaseModal>
	</div>
</template>
