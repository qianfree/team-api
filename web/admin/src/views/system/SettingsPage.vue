<script setup lang="ts">
import { ref, onMounted, type Component } from 'vue'
import PageHeader from '@/components/PageHeader.vue'
import { useSettings } from './settings/useSettings'

import GeneralTab from './settings/GeneralTab.vue'
import OAuthTab from './settings/OAuthTab.vue'
import EmailTab from './settings/EmailTab.vue'
import SecurityTab from './settings/SecurityTab.vue'
import AuditTab from './settings/AuditTab.vue'
import PaymentTab from './settings/PaymentTab.vue'
import PerformanceTab from './settings/PerformanceTab.vue'
import ContentFilterTab from './settings/ContentFilterTab.vue'
import ChannelTab from './settings/ChannelTab.vue'
import StorageTab from './settings/StorageTab.vue'
import DataGovernanceTab from './settings/DataGovernanceTab.vue'
import AgreementTab from './settings/AgreementTab.vue'

interface SettingCategory {
	key: string
	label: string
	icon: string
	order: number
}

const categoryMap: Record<string, Component> = {
	general: GeneralTab,
	oauth: OAuthTab,
	email: EmailTab,
	security: SecurityTab,
	audit: AuditTab,
	payment: PaymentTab,
	performance: PerformanceTab,
	content_filter: ContentFilterTab,
	channel: ChannelTab,
	storage: StorageTab,
	data_governance: DataGovernanceTab,
	agreement: AgreementTab,
}

const categories = ref<SettingCategory[]>([])
const activeTab = ref('')

const { formValues, loading, saving, refresh, save } = useSettings(() => activeTab.value)

async function fetchCategories() {
	try {
		const { default: request } = await import('@/utils/request')
		const res: any = await request.get('/admin/settings/categories')
		categories.value = (res.data?.data?.list || []).sort((a: SettingCategory, b: SettingCategory) => a.order - b.order)
		if (categories.value.length > 0 && !activeTab.value) {
			activeTab.value = categories.value[0].key
			await refresh()
		}
	} catch {
		// silently fail
	}
}

async function handleTabChange(key: string | number) {
	activeTab.value = String(key)
	await refresh()
}

onMounted(() => {
	fetchCategories()
})
</script>

<template>
	<div>
		<PageHeader title="系统设置" description="管理平台全局配置，按分类查看和编辑" />

		<div class="settings-layout">
			<!-- 左侧分类菜单 -->
			<div class="settings-sidebar">
				<AMenu :selected-keys="[activeTab]" @menu-item-click="handleTabChange">
					<AMenuItem v-for="cat in categories" :key="cat.key">{{ cat.label }}</AMenuItem>
				</AMenu>
			</div>

			<!-- 右侧内容区域 -->
			<div class="settings-content">
				<ACard :bordered="false">
					<ASpin :loading="loading">
						<component :is="categoryMap[activeTab]" v-if="categoryMap[activeTab]" />

						<div class="settings-footer">
							<AButton type="primary" :loading="saving" @click="save">保存设置</AButton>
						</div>
					</ASpin>
				</ACard>
			</div>
		</div>
	</div>
</template>

<style scoped>
.settings-layout {
	display: flex;
	gap: 24px;
	align-items: flex-start;
}
.settings-sidebar {
	width: 200px;
	flex-shrink: 0;
	background: var(--color-bg-2);
	border-radius: 4px;
	border: 1px solid var(--color-border);
}
.settings-sidebar :deep(.arco-menu) {
	border-radius: 4px;
	padding: 4px 0;
}
.settings-sidebar :deep(.arco-menu-item) {
	border-radius: 4px;
	margin: 2px 4px;
}
.settings-content {
	flex: 1;
	min-width: 0;
	max-width: 800px;
}
.settings-footer {
	display: flex;
	justify-content: flex-end;
	margin-top: 16px;
}
</style>
