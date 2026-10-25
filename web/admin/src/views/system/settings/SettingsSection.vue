<script setup lang="ts">
defineProps<{
	title: string
	desc?: string
	/** full=整行大卡（默认），half=半宽小卡片（与相邻小卡片并排） */
	size?: 'full' | 'half'
}>()
</script>

<template>
	<section class="settings-section" :class="{ 'settings-section--half': size === 'half' }">
		<header class="settings-section__header">
			<div class="settings-section__heading">
				<h3 class="settings-section__title">{{ title }}</h3>
				<p v-if="desc" class="settings-section__desc">{{ desc }}</p>
			</div>
			<div v-if="$slots.extra" class="settings-section__extra">
				<slot name="extra" />
			</div>
		</header>
		<div class="settings-section__body">
			<slot />
		</div>
	</section>
</template>

<style scoped>
.settings-section {
	/* .tab-content 是双列网格，默认整行；half 小卡片只占一列 */
	grid-column: 1 / -1;
	background: var(--ta-bg-card);
	border-radius: var(--ta-radius);
	box-shadow: var(--ta-shadow-card);
	padding: 20px 24px;
	transition: box-shadow var(--ta-duration-normal) var(--ta-ease);
}
.settings-section--half {
	grid-column: auto;
	padding: 20px;
}
.settings-section__header {
	display: flex;
	align-items: flex-start;
	justify-content: space-between;
	gap: 16px;
	margin-bottom: 16px;
}
.settings-section__title {
	margin: 0;
	font-size: 15px;
	font-weight: 600;
	color: var(--ta-text-primary);
	line-height: 1.4;
}
.settings-section__desc {
	margin: 4px 0 0;
	font-size: 12px;
	color: var(--ta-text-tertiary);
	line-height: 1.6;
}
.settings-section__extra {
	flex-shrink: 0;
	display: flex;
	align-items: center;
	gap: 8px;
}
.settings-section__body {
	/* 容器查询锚点：内部字段网格按卡片实际宽度决定列数（common.css） */
	container-type: inline-size;
	display: flex;
	flex-direction: column;
	gap: 16px;
}
/* 卡片内紧跟开关列表的字段网格去掉首行间距，避免与分隔线堆叠 */
.settings-section__body > .switch-list + .section-grid {
	margin-top: 0;
}

/* 表单项 label 固定宽度左列，各行控件起点对齐（Arco FormItem 默认横排） */
.settings-section :deep(.arco-form-item-label-col) {
	width: 140px;
	flex: 0 0 140px;
	padding-right: 12px;
	line-height: 32px;
}
.settings-section :deep(.arco-form-item-wrapper-col) {
	flex: 1;
	min-width: 0;
}

/* 移动端：label 回到控件上方，并重置横排默认的靠右对齐 */
@media (max-width: 768px) {
	.settings-section :deep(.arco-form-item) {
		flex-direction: column;
		align-items: stretch;
	}
	.settings-section :deep(.arco-form-item-label-col) {
		width: auto;
		flex: none;
		justify-content: flex-start;
		white-space: normal;
		padding-right: 0;
		margin-bottom: 4px;
		line-height: 20px;
	}
}
</style>
