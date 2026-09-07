<script setup lang="ts">
import { ref, computed, onMounted, type Component } from 'vue'
import { useRouter } from 'vue-router'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import EpayChannelCard from './payment-channels/EpayChannelCard.vue'

// 渠道类型 → 配置卡片组件映射：后端新增渠道类型时在此登记对应组件即可，
// Tab 按后端返回的渠道列表生成；未登记的渠道类型选中后显示占位提示
const channelCards: Record<string, Component> = {
	epay: EpayChannelCard,
}

// 渠道中文名（侧栏 Tab 与占位提示展示用）
const channelNames: Record<string, string> = {
	epay: '易支付',
}

interface ChannelItem {
	channel: string
	is_enabled: boolean
	config: any
}

const channels = ref<ChannelItem[]>([])
const activeChannel = ref('')
const loading = ref(false)
const callbackBaseURL = ref('')
const router = useRouter()

onMounted(async () => {
	loading.value = true
	try {
		// 渠道列表与支付设置并行拉取：回调地址是启用渠道的前置条件，为空时提示先去配置
		const [chRes, settingsRes] = await Promise.all([
			request.get('/admin/payment-channels'),
			request.get('/admin/payment-settings'),
		])
		channels.value = chRes.data?.data?.list || []
		callbackBaseURL.value = settingsRes.data?.data?.callback_base_url || ''
		// 默认选中第一个渠道
		if (channels.value.length > 0) activeChannel.value = channels.value[0].channel
	} catch {
		// 错误提示由拦截器统一弹出
	} finally {
		loading.value = false
	}
})

const activeChannelItem = computed(() => channels.value.find(c => c.channel === activeChannel.value))

function gotoPaymentSettings() {
	router.push({ name: 'AdminSettings', query: { tab: 'payment' } })
}
</script>

<template>
	<div>
		<PageHeader title="支付渠道" description="管理租户充值可用的支付方式，按渠道类型独立配置和启停" />

		<!-- 启用任一渠道前必须先配置回调地址（后端保存时强校验），为空时提前引导 -->
		<AAlert v-if="!loading && !callbackBaseURL" type="warning" class="callback-alert">
			支付回调基础 URL 未配置，启用渠道前请先前往「系统设置 → 支付」完成配置，否则支付成功后无法回调入账。
			<AButton type="text" size="mini" @click="gotoPaymentSettings">前往配置</AButton>
		</AAlert>

		<div class="channels-layout">
			<!-- 左侧渠道 Tab：后端已注册渠道逐个列出，末尾固定一个「敬请期待」占位 -->
			<div class="channels-sidebar">
				<AMenu :selected-keys="[activeChannel]">
					<AMenuItem v-for="ch in channels" :key="ch.channel">
						{{ channelNames[ch.channel] || ch.channel }}
					</AMenuItem>
					<!-- 更多渠道接入开发中：禁用态占位，点击无响应 -->
					<AMenuItem key="coming-soon" disabled>敬请期待</AMenuItem>
				</AMenu>
			</div>

			<!-- 右侧当前渠道配置卡片 -->
			<div class="channels-content">
				<ACard :bordered="false">
					<ASpin :loading="loading" style="display: block">
						<template v-if="activeChannelItem">
							<component
								:is="channelCards[activeChannelItem.channel]"
								v-if="channelCards[activeChannelItem.channel]"
								:key="activeChannelItem.channel"
								:initial-config="activeChannelItem.config"
							/>
							<div v-else class="unsupported-card">
								{{ channelNames[activeChannelItem.channel] || activeChannelItem.channel }}渠道已注册，但当前版本暂未提供配置界面
							</div>
						</template>
						<div v-else-if="!loading" class="unsupported-card">暂无可用支付渠道</div>
					</ASpin>
				</ACard>
			</div>
		</div>
	</div>
</template>

<style scoped>
.callback-alert {
	margin-bottom: 16px;
}

.channels-layout {
	display: flex;
	gap: 24px;
	align-items: flex-start;
}

.channels-sidebar {
	width: 200px;
	flex-shrink: 0;
	background: var(--color-bg-2);
	border-radius: 4px;
	border: 1px solid var(--color-border);
}

.channels-sidebar :deep(.arco-menu) {
	border-radius: 4px;
	padding: 4px 0;
}

.channels-sidebar :deep(.arco-menu-item) {
	border-radius: 4px;
	margin: 2px 4px;
}

.channels-content {
	flex: 1;
	min-width: 0;
	max-width: 1000px;
}

.unsupported-card {
	padding: 20px;
	border: 1px dashed var(--color-border-2);
	border-radius: 4px;
	color: var(--color-text-3);
	font-size: 13px;
	text-align: center;
}

/* 移动端：侧栏 Tab 转为顶部横向滚动条，内容区下沉为单列，避免 200px 固定侧栏挤压内容 */
@media (max-width: 768px) {
	.channels-layout {
		flex-direction: column;
		gap: 12px;
	}
	.channels-sidebar {
		width: 100%;
		position: sticky;
		top: 0;
		z-index: 5;
	}
	.channels-sidebar :deep(.arco-menu-inner) {
		display: flex;
		flex-wrap: nowrap;
		overflow-x: auto;
		padding: 4px;
		/* 隐藏横向滚动条，保留可滑动 */
		scrollbar-width: none;
	}
	.channels-sidebar :deep(.arco-menu-inner)::-webkit-scrollbar {
		display: none;
	}
	.channels-sidebar :deep(.arco-menu-item) {
		flex-shrink: 0;
		white-space: nowrap;
		margin: 0 2px;
	}
}
</style>
