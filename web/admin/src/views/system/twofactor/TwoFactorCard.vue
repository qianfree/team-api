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
	<div class="profile-card twofactor-card">
		<div class="profile-card__header">两步验证（TOTP）</div>
		<div class="profile-card__body">
			<!-- 未开启态 -->
			<template v-if="!enabled">
				<a-alert type="warning">
					当前账号仅受密码保护。开启后登录需额外输入验证器 App（阿里云虚拟MFA、Google Authenticator 等）中的动态码，可显著降低密码泄露风险。
				</a-alert>
				<a-button type="primary" class="mt16" @click="showSetup = true">开启两步验证</a-button>
			</template>

			<!-- 已开启态 -->
			<template v-else>
				<div class="status-row">
					<a-tag color="green">已开启</a-tag>
					<span class="status-desc">登录时需输入验证器 App 中的 6 位动态码</span>
				</div>
				<p class="regen-hint">恢复码用于验证器不可用时登录，忘记或用尽时可在此重新生成（旧恢复码全部作废）。</p>
				<div class="btn-row">
					<a-button size="small" @click="showRegen = true">重新生成恢复码</a-button>
					<a-button size="small" status="danger" @click="showDisable = true">关闭两步验证</a-button>
				</div>
			</template>
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
	</div>
</template>

<style scoped>
.twofactor-card {
	grid-column: 1 / -1;
}
.mt16 {
	margin-top: 16px;
}
.mb16 {
	margin-bottom: 16px;
}
.status-row {
	display: flex;
	align-items: center;
	gap: 10px;
	margin-bottom: 10px;
}
.status-desc {
	font-size: 13px;
	color: var(--ta-text-secondary, #4e5969);
}
.regen-hint {
	font-size: 12px;
	line-height: 1.6;
	color: var(--ta-text-tertiary);
	margin: 0 0 12px;
}
.btn-row {
	display: flex;
	gap: 8px;
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
