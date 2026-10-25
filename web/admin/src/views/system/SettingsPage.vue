<script setup lang="ts">
import { ref, computed, onMounted, type Component } from 'vue'
import { useRoute } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import {
	IconSettings, IconLock, IconEmail, IconSafe, IconHistory, IconSwap,
	IconThunderbolt, IconFilter, IconShareInternal, IconCloud, IconStorage, IconFile,
} from '@arco-design/web-vue/es/icon'
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

// 后端下发的分类 icon 名 → Arco 图标（未匹配时回落齿轮）
const iconMap: Record<string, Component> = {
	settings: IconSettings,
	lock: IconLock,
	email: IconEmail,
	shield: IconSafe,
	audit: IconHistory,
	payment: IconSwap,
	speed: IconThunderbolt,
	filter: IconFilter,
	channel: IconShareInternal,
	cloud: IconCloud,
	database: IconStorage,
	document: IconFile,
}

const categories = ref<SettingCategory[]>([])
const activeTab = ref('')
const route = useRoute()

const { formValues, loading, saving, refresh, save } = useSettings(() => activeTab.value)

// 未保存更改检测：refresh 后对表单值拍快照，任意修改即偏离快照
let snapshot = ''
const dirty = computed(() => snapshot !== '' && JSON.stringify(formValues) !== snapshot)

async function reload() {
	await refresh()
	snapshot = JSON.stringify(formValues)
}

async function handleSave() {
	const ok = await save()
	if (ok) snapshot = JSON.stringify(formValues)
}

// 存储配置：测试对象存储连通性（上传→下载→删除一张测试图片）
const testing = ref(false)
async function testStorage() {
	testing.value = true
	try {
		const { default: request } = await import('@/utils/request')
		const res: any = await request.post('/admin/settings/storage/test', { ...formValues })
		const d = res.data?.data || {}
		Message.success(d.message || '对象存储配置正常')
	} catch {
		// 失败提示由响应拦截器统一弹出（后端返回中文错误信息）
	} finally {
		testing.value = false
	}
}

// 邮件配置：发送一封测试邮件验证 SMTP 是否可用。
// 收件人留空由后端回落到当前管理员邮箱（登录态里不含邮箱，因此不预填，成功后回显实际收件人）。
const emailTestVisible = ref(false)
const emailTestTo = ref('')
async function testEmail(): Promise<boolean> {
	testing.value = true
	try {
		const { default: request } = await import('@/utils/request')
		const res: any = await request.post('/admin/settings/email/test', {
			...formValues,
			to: emailTestTo.value.trim(),
		})
		const d = res.data?.data || {}
		Message.success(d.message || '测试邮件已发送')
		return true
	} catch {
		// 失败提示由响应拦截器统一弹出（后端透出真实 SMTP 错误）；返回 false 保持弹窗打开便于改收件人重试
		return false
	} finally {
		testing.value = false
	}
}

const activeLabel = computed(
	() => categories.value.find(c => c.key === activeTab.value)?.label || ''
)

async function fetchCategories() {
	try {
		const { default: request } = await import('@/utils/request')
		const res: any = await request.get('/admin/settings/categories')
		categories.value = (res.data?.data?.list || []).sort((a: SettingCategory, b: SettingCategory) => a.order - b.order)
		if (categories.value.length > 0 && !activeTab.value) {
			// 支持 ?tab=xxx 直达指定分类（如支付渠道页引导跳转支付分类）
			const fromQuery = String(route.query.tab || '')
			activeTab.value = categories.value.some(c => c.key === fromQuery) ? fromQuery : categories.value[0].key
			await reload()
		}
	} catch {
		// silently fail
	}
}

async function handleTabChange(key: string | number) {
	activeTab.value = String(key)
	await reload()
}

onMounted(() => {
	fetchCategories()
})
</script>

<template>
	<div class="settings-page">
		<PageHeader title="系统设置" description="管理平台全局配置，按分类查看和编辑" />

		<div class="settings-layout">
			<!-- 左侧分类菜单（吸顶） -->
			<div class="settings-sidebar">
				<AMenu :selected-keys="[activeTab]" @menu-item-click="handleTabChange">
					<AMenuItem v-for="cat in categories" :key="cat.key">
						<template #icon>
							<component :is="iconMap[cat.icon] || IconSettings" />
						</template>
						{{ cat.label }}
					</AMenuItem>
				</AMenu>
			</div>

			<!-- 右侧内容区域：分区卡片流 + 吸底保存栏 -->
			<div class="settings-content">
				<ASpin :loading="loading" class="settings-spin">
					<component :is="categoryMap[activeTab]" v-if="categoryMap[activeTab]" />
				</ASpin>

				<div class="settings-footer">
					<div class="footer-status">
						<span class="footer-dot" :class="{ 'footer-dot--dirty': dirty }" />
						<span class="footer-label">
							{{ activeLabel }}{{ dirty ? ' · 有未保存的更改' : '' }}
						</span>
					</div>
					<div class="footer-actions">
						<AButton
							v-if="activeTab === 'storage'"
							:loading="testing"
							@click="testStorage"
						>测试连接</AButton>
						<AButton
							v-if="activeTab === 'email'"
							@click="emailTestVisible = true"
						>发送测试邮件</AButton>
						<AButton type="primary" :loading="saving" @click="handleSave">保存设置</AButton>
					</div>
				</div>
			</div>
		</div>

		<!-- 发送测试邮件 -->
		<AModal
			v-model:visible="emailTestVisible"
			title="发送测试邮件"
			:width="480"
			:ok-loading="testing"
			:on-before-ok="testEmail"
			ok-text="发送"
		>
			<AFormItem label="收件人">
				<AInput v-model="emailTestTo" placeholder="留空则发送给当前管理员邮箱" allow-clear />
			</AFormItem>
			<p class="email-test-hint">将使用当前表单中的邮件配置发送（无需先保存）；密码留空时使用已保存的值。</p>
		</AModal>
	</div>
</template>

<style scoped>
.email-test-hint {
	margin: 8px 0 0;
	font-size: 12px;
	color: var(--color-text-3);
}
.settings-layout {
	display: flex;
	gap: 24px;
	align-items: flex-start;
}
.settings-sidebar {
	width: 200px;
	flex-shrink: 0;
	/* 滚动容器 .admin-content 有 24px padding，sticky top:0 会钉在 padding 之后，
	   导致滚动内容从菜单上方 24px 缝隙穿过；负偏移贴住可视区真正顶边 */
	position: sticky;
	top: -24px;
	background: var(--ta-bg-card);
	border-radius: var(--ta-radius);
	box-shadow: var(--ta-shadow-card);
	padding: 8px;
}
.settings-sidebar :deep(.arco-menu) {
	border-radius: calc(var(--ta-radius) - 4px);
	padding: 4px 0;
}
.settings-sidebar :deep(.arco-menu-item) {
	border-radius: 8px;
	margin: 2px 4px;
}
.settings-content {
	flex: 1;
	min-width: 0;
	max-width: 1200px;
}
.settings-spin {
	display: block;
	width: 100%;
}

/* 吸底保存栏：滚动长表单时始终可见。负 margin 抵消滚动容器 24px padding，
   拉通满宽贴住可视区底边，避免滚动内容从底部缝隙穿过；
   底边贴视口边缘，仅上两角做小圆角（下角做圆角会漏出底层滚动内容） */
.settings-footer {
	position: sticky;
	bottom: -24px;
	z-index: 4;
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 12px;
	margin: 16px -24px -24px;
	padding: 12px 24px;
	background: var(--ta-bg-glass);
	backdrop-filter: blur(var(--ta-glass-blur));
	-webkit-backdrop-filter: blur(var(--ta-glass-blur));
	border-top: 1px solid var(--ta-border);
	border-radius: 8px 8px 0 0;
	box-shadow: 0 -4px 16px rgba(0, 0, 0, 0.04);
}
.footer-status {
	display: flex;
	align-items: center;
	gap: 8px;
	min-width: 0;
}
.footer-dot {
	width: 8px;
	height: 8px;
	border-radius: 50%;
	flex-shrink: 0;
	background: var(--ta-success);
	transition: background var(--ta-duration-normal) var(--ta-ease);
}
.footer-dot--dirty {
	background: var(--ta-warning);
}
.footer-label {
	font-size: 13px;
	color: var(--ta-text-secondary);
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
}
.footer-actions {
	display: flex;
	gap: 8px;
	flex-shrink: 0;
}

/* 移动端：左侧分类菜单转为顶部横向滚动条，内容区下沉为单列，避免 200px 固定侧栏挤压内容 */
@media (max-width: 768px) {
	.settings-layout {
		flex-direction: column;
		gap: 12px;
	}
	.settings-sidebar {
		width: 100%;
		position: sticky;
		/* 同桌面：-24px 抵消滚动容器 padding，贴住顶栏下缘 */
		top: -24px;
		z-index: 5;
	}
	/* 布局是 align-items:flex-start（桌面端 sidebar sticky 需要），column 方向下
	   子项不会自动拉伸满宽，内容区必须显式占满整行 */
	.settings-content {
		width: 100%;
	}
	.settings-sidebar :deep(.arco-menu-inner) {
		display: flex;
		flex-wrap: nowrap;
		overflow-x: auto;
		padding: 4px;
		/* 隐藏横向滚动条，保留可滑动 */
		scrollbar-width: none;
	}
	.settings-sidebar :deep(.arco-menu-inner)::-webkit-scrollbar {
		display: none;
	}
	.settings-sidebar :deep(.arco-menu-item) {
		flex-shrink: 0;
		white-space: nowrap;
		margin: 0 2px;
	}
	.settings-footer {
		flex-direction: column;
		align-items: stretch;
		gap: 10px;
		border-radius: 0;
	}
	.footer-label {
		display: none;
	}
	.footer-actions {
		/* 底部按钮在移动端填满整行，便于拇指触达 */
		width: 100%;
	}
	.footer-actions :deep(.arco-btn) {
		flex: 1;
	}
}
</style>
