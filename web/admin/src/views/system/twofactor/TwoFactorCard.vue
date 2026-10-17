<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import request, { extractApiError } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'
import Setup2FAModal from './Setup2FAModal.vue'
import BackupCodesPanel from './BackupCodesPanel.vue'

const authStore = useAuthStore()
// me 接口的 totp_enabled 缺省（undefined）视为未开启
const enabled = computed(() => !!authStore.user?.totp_enabled)

// ── 开启向导 ──
const showSetup = ref(false)
function onSetupDone() {
	showSetup.value = false
	Message.success('两步验证已开启')
	// 路由守卫每会话只拉一次 me，必须显式刷新（同时同步 localStorage 镜像）
	authStore.fetchMe()
}

// ── 关闭两步验证 ──
const showDisable = ref(false)
const disableCode = ref('')
const disableLoading = ref(false)
const disableError = ref('')
watch(showDisable, (v) => {
	if (v) {
		disableCode.value = ''
		disableError.value = ''
	}
})

async function handleDisable(): Promise<boolean> {
	if (!disableCode.value.trim()) {
		disableError.value = '请输入动态验证码或恢复码'
		return false
	}
	disableLoading.value = true
	disableError.value = ''
	try {
		await request.post(
			'/admin/security/2fa/disable',
			{ code: disableCode.value.trim() },
			{ _suppressErrorMsg: true },
		)
		Message.success('两步验证已关闭')
		authStore.fetchMe()
		return true
	} catch (e) {
		disableError.value = extractApiError(e)?.message || '操作失败'
		return false
	} finally {
		disableLoading.value = false
	}
}

// ── 重新生成恢复码 ──
const showRegen = ref(false)
const regenCode = ref('')
const regenLoading = ref(false)
const regenError = ref('')
const regenCodes = ref<string[]>([])
watch(showRegen, (v) => {
	if (v) {
		regenCode.value = ''
		regenError.value = ''
		regenCodes.value = []
	}
})

async function handleRegen() {
	if (!regenCode.value.trim()) {
		regenError.value = '请输入动态验证码'
		return
	}
	regenLoading.value = true
	regenError.value = ''
	try {
		const { data } = await request.post(
			'/admin/security/2fa/backup-codes',
			{ code: regenCode.value.trim() },
			{ _suppressErrorMsg: true },
		)
		regenCodes.value = data.data.backup_codes ?? []
	} catch (e) {
		regenError.value = extractApiError(e)?.message || '操作失败'
	} finally {
		regenLoading.value = false
	}
}
</script>

<template>
	<!-- 本组件只渲染卡片「内容」，外壳（.profile-card / __header / __body）由 ProfilePage 渲染：
	     父级 scoped 样式只作用于子组件的根元素，子组件内部节点拿不到这些规则 -->
	<div class="tf-row">
		<!-- 状态图标：开启=盾牌带勾（绿），未开启=盾牌带叹号（橙） -->
		<div class="tf-icon" :class="enabled ? 'tf-icon--on' : 'tf-icon--off'">
			<svg v-if="enabled" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75m-3-7.036A11.959 11.959 0 0 1 3.598 6 11.99 11.99 0 0 0 3 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" />
			</svg>
			<!-- 与开启态同一套盾牌轮廓，把对勾换成叹号（保持两种状态外形一致） -->
			<svg v-else fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 8.75v4" />
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 15.75h.008v.008H12z" />
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 2.714A11.959 11.959 0 0 1 3.598 6 11.99 11.99 0 0 0 3 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" />
			</svg>
		</div>

		<!-- 状态文案：标题行（状态标签 + 一句话结论）与说明 -->
		<div class="tf-info">
			<div class="tf-title">
				<span class="tf-title__text">{{ enabled ? '登录时需输入验证器 6 位动态码' : '当前账号仅受密码保护' }}</span>
				<a-tag :color="enabled ? 'green' : 'orange'">{{ enabled ? '已开启' : '未开启' }}</a-tag>
			</div>
			<p class="tf-desc">
				<template v-if="enabled">支持阿里云虚拟MFA、Google Authenticator 等验证器 App；恢复码用于验证器不可用时登录，忘记或用尽时可在此重新生成（旧恢复码全部作废）。</template>
				<template v-else>开启后登录需额外输入验证器 App（阿里云虚拟MFA、Google Authenticator 等）中的动态码，可显著降低密码泄露风险。</template>
			</p>
		</div>

		<!-- 操作区 -->
		<div class="tf-actions">
			<template v-if="enabled">
				<a-button @click="showRegen = true">重新生成恢复码</a-button>
				<a-button status="danger" @click="showDisable = true">关闭两步验证</a-button>
			</template>
			<a-button v-else type="primary" @click="showSetup = true">开启两步验证</a-button>
		</div>
	</div>

	<!-- 开启向导 -->
	<Setup2FAModal v-model:visible="showSetup" @done="onSetupDone" />

	<!-- 关闭确认 -->
	<a-modal
		v-model:visible="showDisable"
		title="关闭两步验证"
		:width="420"
		:mask-closable="false"
		:on-before-ok="handleDisable"
		:ok-loading="disableLoading"
		ok-text="确认关闭"
	>
		<a-alert type="warning" class="mb16">关闭后账号仅受密码保护，安全性降低。</a-alert>
		<a-form layout="vertical">
			<a-form-item label="动态验证码或恢复码">
				<a-input v-model="disableCode" placeholder="验证器 6 位动态码，或任一未使用的恢复码" />
			</a-form-item>
		</a-form>
		<div v-if="disableError" class="field-error">{{ disableError }}</div>
	</a-modal>

	<!-- 重新生成恢复码 -->
	<a-modal
		v-model:visible="showRegen"
		title="重新生成恢复码"
		:width="560"
		:mask-closable="false"
		:footer="false"
	>
		<template v-if="regenCodes.length === 0">
			<a-alert type="warning" class="mb16">重新生成后旧的恢复码将全部作废，请确认手头验证器可用。</a-alert>
			<a-form layout="vertical" @submit="handleRegen">
				<a-form-item label="动态验证码">
					<a-input v-model="regenCode" placeholder="验证器当前显示的 6 位动态码" :max-length="6" />
				</a-form-item>
			</a-form>
			<div v-if="regenError" class="field-error">{{ regenError }}</div>
			<div class="regen-footer">
				<a-button type="primary" :loading="regenLoading" @click="handleRegen">生成新恢复码</a-button>
			</div>
		</template>
		<template v-else>
			<BackupCodesPanel :codes="regenCodes" />
		</template>
	</a-modal>
</template>

<style scoped>
/* ── 安全设置行：图标 + 文案在左，操作在右 ── */
.tf-row {
	display: flex;
	align-items: center;
	gap: 16px;
}

.tf-icon {
	width: 44px;
	height: 44px;
	flex-shrink: 0;
	display: flex;
	align-items: center;
	justify-content: center;
	border-radius: var(--ta-radius);
}
.tf-icon svg {
	width: 24px;
	height: 24px;
}
.tf-icon--on {
	color: var(--ta-success);
	background: var(--color-success-light-2);
}
.tf-icon--off {
	color: var(--ta-warning);
	background: var(--color-warning-light-2);
}

.tf-info {
	flex: 1;
	min-width: 0;
}
/* 标题行高度 + 间距 + 说明行高度 ≈ 图标高度，视觉上左右对齐 */
.tf-title {
	display: flex;
	align-items: center;
	gap: 8px;
	margin-bottom: 4px;
}
.tf-title__text {
	font-size: 14px;
	line-height: 1.45;
	font-weight: 600;
	color: var(--ta-text-primary);
}
.tf-title :deep(.arco-tag) {
	flex-shrink: 0;
}
.tf-desc {
	margin: 0;
	/* 限宽避免超宽卡片下说明被拉成一整行难以阅读 */
	max-width: 880px;
	font-size: 13px;
	line-height: 1.6;
	color: var(--ta-text-secondary);
}

.tf-actions {
	display: flex;
	flex-shrink: 0;
	gap: 8px;
}

/* 窄屏：操作按钮换行铺满 */
@media (max-width: 768px) {
	.tf-row {
		flex-wrap: wrap;
	}
	.tf-actions {
		width: 100%;
	}
	.tf-actions :deep(.arco-btn) {
		flex: 1;
	}
}

.mb16 {
	margin-bottom: 16px;
}
.field-error {
	margin-top: 4px;
	font-size: 13px;
	color: rgb(var(--danger-6));
}
.regen-footer {
	display: flex;
	justify-content: flex-end;
	margin-top: 12px;
}
</style>
