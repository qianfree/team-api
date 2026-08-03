<script setup lang="ts">
import { computed, ref } from 'vue'

const props = withDefaults(defineProps<{
	modelValue: number
	pageSize: number
	total: number
	pageSizeOptions?: number[]
	showSizeChanger?: boolean
}>(), {
	pageSizeOptions: () => [10, 20, 50, 100],
	showSizeChanger: true,
})

const emit = defineEmits<{
	'update:modelValue': [page: number]
	'update:pageSize': [size: number]
	change: [page: number]
}>()

const jumpPage = ref('')
const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))

// 当前 pageSize 不在候选列表时兜底追加，避免 select 找不到选中项
const sizeOptions = computed(() => {
	const opts = [...props.pageSizeOptions]
	if (!opts.includes(props.pageSize)) opts.push(props.pageSize)
	return opts.sort((a, b) => a - b)
})

const pageItems = computed(() => {
	const count = totalPages.value
	if (count <= 7) {
		return Array.from({ length: count }, (_, index) => ({
			key: `page-${index + 1}`,
			page: index + 1,
			label: String(index + 1),
		}))
	}

	const pages = new Set([1, count, props.modelValue - 1, props.modelValue, props.modelValue + 1])
	const visiblePages = [...pages].filter(page => page >= 1 && page <= count).sort((a, b) => a - b)
	const items: Array<{ key: string; page?: number; label: string }> = []

	visiblePages.forEach((page, index) => {
		const previous = visiblePages[index - 1]
		if (previous && page - previous > 1) {
			items.push({ key: `ellipsis-${previous}`, label: '...' })
		}
		items.push({ key: `page-${page}`, page, label: String(page) })
	})

	return items
})

function goToPage(page: number) {
	const target = Math.min(totalPages.value, Math.max(1, Math.trunc(page)))
	if (target === props.modelValue) return
	emit('update:modelValue', target)
	emit('change', target)
}

function changePageSize(size: number) {
	if (!Number.isFinite(size) || size === props.pageSize) return
	emit('update:pageSize', size)
	// 切换每页条数后回到第一页并触发刷新（即使原本就在第一页也要重新拉取）
	emit('update:modelValue', 1)
	emit('change', 1)
}

function submitJump() {
	const target = Number(jumpPage.value)
	if (!Number.isFinite(target)) return
	goToPage(target)
	jumpPage.value = ''
}
</script>

<template>
	<div v-if="total > 0" class="table-pagination">
		<div class="pagination-meta">
			<div v-if="showSizeChanger" class="pagination-size">
				<span>每页</span>
				<select
					class="pagination-size-select"
					:value="pageSize"
					aria-label="每页条数"
					@change="changePageSize(Number(($event.target as HTMLSelectElement).value))"
				>
					<option v-for="opt in sizeOptions" :key="opt" :value="opt">{{ opt }} 条/页</option>
				</select>
			</div>
			<span>共 {{ total }} 条记录</span>
		</div>

		<div class="pagination-controls">
			<button
				type="button"
				class="btn btn-ghost btn-sm"
				:disabled="modelValue <= 1"
				@click="goToPage(modelValue - 1)"
			>
				上一页
			</button>

			<nav class="pagination-pages" aria-label="分页">
				<template v-for="item in pageItems" :key="item.key">
					<button
						v-if="item.page"
						type="button"
						class="pagination-page"
						:class="{ 'pagination-page-active': item.page === modelValue }"
						:aria-current="item.page === modelValue ? 'page' : undefined"
						@click="goToPage(item.page)"
					>
						{{ item.label }}
					</button>
					<span v-else class="pagination-ellipsis">{{ item.label }}</span>
				</template>
			</nav>

			<button
				type="button"
				class="btn btn-ghost btn-sm"
				:disabled="modelValue >= totalPages"
				@click="goToPage(modelValue + 1)"
			>
				下一页
			</button>

			<form class="pagination-jump" @submit.prevent="submitJump">
				<label for="pagination-jump-input">跳至</label>
				<input
					id="pagination-jump-input"
					v-model="jumpPage"
					type="number"
					inputmode="numeric"
					:min="1"
					:max="totalPages"
					class="pagination-jump-input"
					aria-label="跳转页码"
				/>
				<span>页</span>
				<button type="submit" class="btn btn-secondary btn-sm">跳转</button>
			</form>
		</div>
	</div>
</template>

<style scoped>
.pagination-meta {
	display: flex;
	align-items: center;
	gap: 1rem;
	min-width: 0;
	font-size: 0.75rem;
	color: #64748b;
}

.pagination-size {
	display: flex;
	align-items: center;
	gap: 0.375rem;
	white-space: nowrap;
}

.pagination-size-select {
	height: 2rem;
	border: 1px solid rgba(203, 213, 225, 0.82);
	border-radius: 0.5rem;
	background: rgba(255, 255, 255, 0.72);
	padding: 0 0.5rem;
	font-size: 0.75rem;
	color: #334155;
	outline: none;
	cursor: pointer;
	transition: border-color 160ms ease, box-shadow 160ms ease;
}

.pagination-size-select:hover {
	border-color: rgba(20, 184, 166, 0.45);
}

.pagination-size-select:focus {
	border-color: #14b8a6;
	box-shadow: 0 0 0 3px rgba(20, 184, 166, 0.12);
}

.pagination-controls,
.pagination-pages,
.pagination-jump {
	display: flex;
	align-items: center;
}

.pagination-controls {
	justify-content: flex-end;
	gap: 0.5rem;
	min-width: 0;
}

.pagination-pages {
	gap: 0.25rem;
}

.pagination-page {
	display: inline-flex;
	height: 2rem;
	min-width: 2rem;
	align-items: center;
	justify-content: center;
	border: 1px solid rgba(226, 232, 240, 0.82);
	border-radius: 0.5rem;
	background: rgba(255, 255, 255, 0.62);
	padding: 0 0.5rem;
	font-size: 0.75rem;
	color: #64748b;
	transition: border-color 160ms ease, background-color 160ms ease, color 160ms ease;
}

.pagination-page:hover {
	border-color: rgba(20, 184, 166, 0.45);
	background: rgba(255, 255, 255, 0.9);
	color: #0d9488;
}

.pagination-page-active {
	border-color: #14b8a6;
	background: #14b8a6;
	color: white;
}

.pagination-page-active:hover {
	background: #0d9488;
	color: white;
}

.pagination-ellipsis {
	min-width: 1.5rem;
	text-align: center;
	font-size: 0.75rem;
	color: #94a3b8;
}

.pagination-jump {
	gap: 0.375rem;
	margin-left: 0.25rem;
	white-space: nowrap;
	font-size: 0.75rem;
	color: #64748b;
}

.pagination-jump-input {
	height: 2rem;
	width: 3.5rem;
	border: 1px solid rgba(203, 213, 225, 0.82);
	border-radius: 0.5rem;
	background: rgba(255, 255, 255, 0.72);
	padding: 0 0.375rem;
	text-align: center;
	font-size: 0.75rem;
	color: #334155;
	outline: none;
}

.pagination-jump-input:focus {
	border-color: #14b8a6;
	box-shadow: 0 0 0 3px rgba(20, 184, 166, 0.12);
}

@media (max-width: 900px) {
	.pagination-controls {
		flex-wrap: wrap;
	}

	.pagination-pages {
		order: 3;
		width: 100%;
		justify-content: flex-end;
	}
}

@media (max-width: 640px) {
	.pagination-meta {
		order: 1;
	}

	.pagination-controls,
	.pagination-jump {
		width: 100%;
	}

	.pagination-controls,
	.pagination-pages,
	.pagination-jump {
		justify-content: center;
	}
}
</style>
