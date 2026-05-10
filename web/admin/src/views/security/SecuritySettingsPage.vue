<template>
	<div class="security-settings-page">
		<a-card title="双因素认证（2FA）" :bordered="false" class="mb-4">
			<template v-if="!totpEnabled">
				<a-alert type="info" class="mb-4">
					启用双因素认证后，登录时需要输入动态验证码，大幅提升账号安全性。
				</a-alert>
				<a-button type="primary" @click="handleSetup" :loading="setupLoading">
					开启 2FA
				</a-button>
			</template>
			<template v-else>
				<a-alert type="success" class="mb-4">已启用双因素认证</a-alert>
				<a-space>
					<a-button @click="showRegenerateModal = true">重新生成恢复码</a-button>
					<a-button status="danger" @click="showDisableModal = true">禁用 2FA</a-button>
				</a-space>
			</template>
		</a-card>

		<!-- Setup Modal -->
		<a-modal v-model:visible="showSetupModal" title="设置双因素认证" :maskClosable="false" :footer="false" width="480px">
			<div v-if="setupStep === 1">
				<a-alert type="info" class="mb-4">请使用身份验证器 App（如 Google Authenticator、Microsoft Authenticator）扫描下方二维码。</a-alert>
				<div class="qr-container">
					<img :src="qrCodeUrl" alt="QR Code" v-if="qrCodeUrl" class="qr-image" />
				</div>
				<div class="secret-key">
					<span class="label">手动输入密钥：</span>
					<a-typography-text copyable code>{{ setupSecret }}</a-typography-text>
				</div>
				<a-button type="primary" long @click="setupStep = 2" class="mt-4">我已扫描，下一步</a-button>
			</div>
			<div v-if="setupStep === 2">
				<a-form :model="enableForm" @submit-success="handleEnable" layout="vertical">
					<a-form-item label="输入验证码" field="code" :rules="[{ required: true, message: '请输入6位验证码' }, { length: 6, message: '验证码为6位' }]">
						<a-input v-model="enableForm.code" placeholder="6位数字验证码" maxlength="6" />
					</a-form-item>
					<a-form-item label="当前密码" field="password" :rules="[{ required: true, message: '请输入密码' }]">
						<a-input-password v-model="enableForm.password" placeholder="请输入当前密码以确认" />
					</a-form-item>
					<a-button type="primary" html-type="submit" long :loading="enableLoading">确认启用</a-button>
				</a-form>
			</div>
		</a-modal>

		<!-- Backup Codes Modal -->
		<a-modal v-model:visible="showBackupCodesModal" title="备用恢复码" :maskClosable="false" width="400px">
			<a-alert type="warning" class="mb-4">请妥善保存以下恢复码。每个恢复码只能使用一次。关闭后无法再次查看。</a-alert>
			<div class="backup-codes-grid">
				<a-tag v-for="code in backupCodes" :key="code" size="large" class="backup-code">{{ code }}</a-tag>
			</div>
			<template #footer>
				<a-button type="primary" @click="showBackupCodesModal = false">我已保存</a-button>
			</template>
		</a-modal>

		<!-- Disable Modal -->
		<a-modal v-model:visible="showDisableModal" title="禁用双因素认证" @ok="handleDisable" :maskClosable="false">
			<a-form :model="disableForm" layout="vertical">
				<a-form-item label="输入验证码或恢复码">
					<a-input v-model="disableForm.code" placeholder="6位验证码或8位恢复码" />
				</a-form-item>
			</a-form>
		</a-modal>

		<!-- Regenerate Backup Codes Modal -->
		<a-modal v-model:visible="showRegenerateModal" title="重新生成恢复码" @ok="handleRegenerate" :maskClosable="false">
			<a-form layout="vertical">
				<a-form-item label="输入验证码">
					<a-input v-model="regenerateCode" placeholder="6位数字验证码" maxlength="6" />
				</a-form-item>
			</a-form>
		</a-modal>
	</div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'

const totpEnabled = ref(false)
const setupLoading = ref(false)
const enableLoading = ref(false)
const showSetupModal = ref(false)
const showBackupCodesModal = ref(false)
const showDisableModal = ref(false)
const showRegenerateModal = ref(false)
const setupStep = ref(1)
const setupSecret = ref('')
const qrCodeUrl = ref('')
const backupCodes = ref<string[]>([])

const enableForm = reactive({ code: '', password: '' })
const disableForm = reactive({ code: '' })
const regenerateCode = ref('')

onMounted(() => {
	check2FAStatus()
})

async function check2FAStatus() {
	try {
		const res = await request.get('/api/admin/auth/sessions')
		// If we can make authenticated requests, check profile for 2FA status
		// For now, we'll get this from a separate endpoint
		const profileRes = await request.get('/api/admin/users/profile')
		if (profileRes.data?.code === 0) {
			totpEnabled.value = profileRes.data.data?.totp_enabled || false
		}
	} catch {
		// Ignore - might not have a profile endpoint
	}
}

async function handleSetup() {
	setupLoading.value = true
	try {
		const res = await request.post('/api/admin/security/2fa/setup')
		setupSecret.value = res.data.data.secret
		// Generate QR code URL using a chart API
		qrCodeUrl.value = `https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(res.data.data.uri)}`
		setupStep.value = 1
		showSetupModal.value = true
	} catch {
		// error toast already shown by interceptor
	} finally {
		setupLoading.value = false
	}
}

async function handleEnable() {
	enableLoading.value = true
	try {
		const res = await request.post('/api/admin/security/2fa/enable', enableForm)
		totpEnabled.value = true
		showSetupModal.value = false
		backupCodes.value = res.data.data.backup_codes || []
		showBackupCodesModal.value = true
		Message.success('双因素认证已启用')
	} catch {
		// error toast already shown by interceptor
	} finally {
		enableLoading.value = false
	}
}

async function handleDisable() {
	if (!disableForm.code) {
		Message.warning('请输入验证码')
		return
	}
	try {
		const res = await request.post('/api/admin/security/2fa/disable', disableForm)
		totpEnabled.value = false
		showDisableModal.value = false
		disableForm.code = ''
		Message.success('双因素认证已禁用')
	} catch {
		// error toast already shown by interceptor
	}
}

async function handleRegenerate() {
	if (!regenerateCode.value) {
		Message.warning('请输入验证码')
		return
	}
	try {
		const res = await request.post('/api/admin/security/2fa/backup-codes', { code: regenerateCode.value })
		regenerateCode.value = ''
		showRegenerateModal.value = false
		backupCodes.value = res.data.data.backup_codes || []
		showBackupCodesModal.value = true
		Message.success('恢复码已重新生成')
	} catch {
		// error toast already shown by interceptor
	}
}
</script>

<style scoped>
.security-settings-page {
	padding: 0;
}
.mb-4 {
	margin-bottom: 16px;
}
.mt-4 {
	margin-top: 16px;
}
.qr-container {
	display: flex;
	justify-content: center;
	padding: 16px;
}
.qr-image {
	width: 200px;
	height: 200px;
}
.secret-key {
	text-align: center;
	padding: 8px 0;
}
.secret-key .label {
	color: var(--color-text-2);
	margin-right: 8px;
}
.backup-codes-grid {
	display: grid;
	grid-template-columns: repeat(2, 1fr);
	gap: 8px;
}
.backup-code {
	font-family: monospace;
	font-size: 16px;
	text-align: center;
}
</style>
