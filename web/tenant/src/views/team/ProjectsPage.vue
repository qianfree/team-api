<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NInput, NInputNumber } from 'naive-ui'
import { useRouter } from 'vue-router'
import Icon from '@/components/common/Icon.vue'
import BaseModal from '@/components/common/BaseModal.vue'
import { renderBadge } from '@/utils/renderUtils'
import request from '@/utils/request'
import { useConfirm } from '@/composables/useConfirm'

const { confirm } = useConfirm()

const router = useRouter()
const loading = ref(false)
const projects = ref<any[]>([])
const page = ref(1)
const pageSize = ref(20)
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
const form = ref({ name: '', description: '', budget: null as number | null })

function openCreate() {
	editingId.value = null
	form.value = { name: '', description: '', budget: null }
	showModal.value = true
}

function openEdit(item: any) {
	editingId.value = item.id
	form.value = { name: item.name, description: item.description || '', budget: item.budget ? Number(item.budget) : null }
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
			params: { page: page.value, page_size: pageSize.value }
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

// NDataTable 列定义
const columns = computed<DataTableColumns<any>>(() => [
	{ title: '项目名称', key: 'name', render: (row) => h('span', { class: 'font-medium text-gray-900' }, row.name) },
	{
		title: '描述',
		key: 'description',
		render: (row) =>
			h('span', { class: 'text-gray-500 max-w-[200px] truncate block' }, row.description || '-'),
	},
	{
		title: '状态',
		key: 'status',
		render: (row) => renderBadge(row.status, statusLabels, statusBadgeClasses),
	},
	{
		title: '预算上限',
		key: 'budget',
		render: (row) =>
			h('span', { class: 'font-mono' }, row.budget ? `$${Number(row.budget).toFixed(2)}` : '不限'),
	},
	{
		title: '创建时间',
		key: 'created_at',
		render: (row) => h('span', { class: 'text-xs text-gray-400' }, row.created_at?.substring(0, 16)),
	},
	{
		title: '操作',
		key: 'actions',
		align: 'right',
		render: (row) =>
			h('div', { class: 'flex items-center gap-1 justify-end' }, [
				h(
					NButton,
					{ quaternary: true, size: 'small', title: '详情', onClick: (e: MouseEvent) => { e.stopPropagation(); goToDetail(row) } },
					{ icon: () => h(Icon, { name: 'eye', size: 'sm' }) }
				),
				row.status === 'active'
					? h(
							NButton,
							{ quaternary: true, size: 'small', title: '编辑', onClick: (e: MouseEvent) => { e.stopPropagation(); openEdit(row) } },
							{ icon: () => h(Icon, { name: 'edit', size: 'sm' }) }
						)
					: null,
				row.status === 'active'
					? h(
							NButton,
							{ quaternary: true, size: 'small', title: '归档', onClick: (e: MouseEvent) => { e.stopPropagation(); handleArchive(row) } },
							{ icon: () => h(Icon, { name: 'x', size: 'sm' }) }
						)
					: null,
				row.status === 'archived'
					? h(
							NButton,
							{ quaternary: true, size: 'small', title: '取消归档', onClick: (e: MouseEvent) => { e.stopPropagation(); handleUnarchive(row) } },
							{ icon: () => h(Icon, { name: 'refresh', size: 'sm' }) }
						)
					: null,
			]),
	},
])

function handleRowProps(row: any) {
	return {
		style: 'cursor: pointer',
		onClick: () => goToDetail(row),
	}
}

function handlePageSizeChange() {
	page.value = 1
	fetchProjects()
}
</script>

<template>
	<div class="viewport-table-page">
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

		<div class="viewport-table-panel card">
			<n-data-table
				remote
				v-model:page="page"
				v-model:page-size="pageSize"
				:item-count="total"
				:page-sizes="[10, 20, 50, 100]"
				show-size-picker
				:loading="loading"
				:columns="columns"
				:data="projects"
				:row-key="(row: any) => row.id"
				:row-props="handleRowProps"
				@update:page="fetchProjects"
				@update:page-size="handlePageSizeChange"
			>
				<template #empty>
					<div class="empty-state">
						<Icon name="project" size="xl" class="empty-state-icon" />
						<h3 class="empty-state-title">暂无项目</h3>
						<p class="empty-state-description">创建项目来组织你的 API Key 和资源</p>
					</div>
				</template>
			</n-data-table>
		</div>

		<!-- Create/Edit Modal -->
		<BaseModal :show="showModal" :title="editingId ? '编辑项目' : '创建项目'" width="normal" @close="showModal = false">
			<div class="space-y-4">
				<div>
					<label class="input-label">项目名称 <span class="text-red-500">*</span></label>
					<n-input v-model:value="form.name" type="text" placeholder="输入项目名称" />
				</div>
				<div>
					<label class="input-label">描述</label>
					<n-input
						v-model:value="form.description"
						type="textarea"
						:rows="2"
						placeholder="项目描述（选填）"
					/>
				</div>
				<div>
					<label class="input-label">预算上限</label>
					<n-input-number v-model:value="form.budget" :min="0" :step="0.01" placeholder="0 = 不限制" style="width: 100%" />
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
