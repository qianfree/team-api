<script setup lang="ts">
import { computed, ref } from 'vue'

const props = defineProps<{
	modelValue: number
	pageSize: number
	total: number
}>()

const emit = defineEmits<{
	'update:modelValue': [page: number]
	change: [page: number]
}>()

const jumpPage = ref('')
const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))

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

function submitJump() {
	const target = Number(jumpPage.value)
	if (!Number.isFinite(target)) return
	goToPage(target)
	jumpPage.value = ''
}
</script>

<template>
	<div v-if="total > 0" class="table-pagination">
		<span>共 {{ total }} 条记录</span>

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
	border-color: rgba(117, 104, 248, 0.45);
	background: rgba(255, 255, 255, 0.9);
	color: #6558ea;
}

.pagination-page-active {
	border-color: #7568f8;
	background: #7568f8;
	color: white;
}

.pagination-page-active:hover {
	background: #6558ea;
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
	border-color: #7568f8;
	box-shadow: 0 0 0 3px rgba(117, 104, 248, 0.12);
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
