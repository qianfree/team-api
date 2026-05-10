<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'

const values = defineModel<Record<string, any>>({ required: true })

// 2FA state
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
		const profileRes = await request.get('/api/admin/users/profile')
		if (profileRes.data?.code === 0) {
			totpEnabled.value = profileRes.data.data?.totp_enabled || false
		}
	} catch {
		// ignore
	}
}

async function handleSetup() {
	setupLoading.value = true
	try {
		const res = await request.post('/api/admin/security/2fa/setup')
		setupSecret.value = res.data.data.secret
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
		await request.post('/api/admin/security/2fa/disable', disableForm)
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

<template>
	<div class="tab-content">
		<!-- 会话管理 -->
		<div class="section">
			<div class="section-title">会话管理</div>
			<div class="section-grid">
				<AFormItem label="租户用户最大会话数">
					<AInputNumber
						:model-value="values['max_sessions_per_user'] as number"
						@change="(v: number | undefined) => values['max_sessions_per_user'] = v ?? 10"
						:min="1" :max="100" style="width: 100%"
					/>
				</AFormItem>
				<AFormItem label="管理员最大会话数">
					<AInputNumber
						:model-value="values['admin_max_sessions'] as number"
						@change="(v: number | undefined) => values['admin_max_sessions'] = v ?? 5"
						:min="1" :max="50" style="width: 100%"
					/>
				</AFormItem>
			</div>
		</div>

		<!-- 登录安全 -->
		<div class="section">
			<div class="section-title">登录安全</div>
			<div class="section-grid">
				<AFormItem label="登录最大尝试次数">
					<AInputNumber
						:model-value="values['login_max_attempts'] as number"
						@change="(v: number | undefined) => values['login_max_attempts'] = v ?? 5"
						:min="1" :max="30" style="width: 100%"
					/>
				</AFormItem>
				<AFormItem label="登录锁定时长(分钟)">
					<AInputNumber
						:model-value="values['login_lockout_minutes'] as number"
						@change="(v: number | undefined) => values['login_lockout_minutes'] = v ?? 30"
						:min="1" :max="1440" style="width: 100%"
					/>
				</AFormItem>
			</div>
		</div>

		<!-- 密码策略 -->
		<div class="section">
			<div class="section-title">密码策略</div>
			<div class="section-grid">
				<AFormItem label="密码最小长度">
					<AInputNumber
						:model-value="values['password_min_length'] as number"
						@change="(v: number | undefined) => values['password_min_length'] = v ?? 8"
						:min="6" :max="32" style="width: 100%"
					/>
				</AFormItem>
			</div>
		</div>

		<!-- 验证码有效期 -->
		<div class="section">
			<div class="section-title">滑块验证码</div>
			<div class="section-desc">登录、注册、重置密码时必须完成滑块验证</div>
			<div class="section-grid">
				<AFormItem label="验证码有效期(秒)">
					<AInputNumber
						:model-value="values['captcha_expire_seconds'] as number"
						@change="(v: number | undefined) => values['captcha_expire_seconds'] = v ?? 300"
						:min="60" :max="600" style="width: 100%"
					/>
				</AFormItem>
			</div>
		</div>

		<!-- Turnstile 人机验证 -->
		<div class="section">
			<div class="section-title">Turnstile 人机验证</div>
			<div class="section-desc">集成 Cloudflare Turnstile，防止自动化攻击和暴力破解</div>
			<div class="section-grid">
				<AFormItem label="启用 Turnstile">
					<ASwitch
						:model-value="!!values['turnstile_enabled']"
						@change="(v: string | number | boolean) => values['turnstile_enabled'] = v"
					/>
				</AFormItem>
				<AFormItem label="新设备登录通知">
					<ASwitch
						:model-value="values['new_device_notification'] !== false"
						@change="(v: string | number | boolean) => values['new_device_notification'] = v"
					/>
				</AFormItem>
			</div>
			<div class="section-grid">
				<AFormItem label="Site Key">
					<AInput
						:model-value="values['turnstile_site_key'] ?? ''"
						@update:model-value="(v: string) => values['turnstile_site_key'] = v"
						placeholder="Cloudflare Turnstile Site Key"
					/>
				</AFormItem>
				<AFormItem label="Secret Key">
					<AInputPassword
						:model-value="values['turnstile_secret_key'] ?? ''"
						@update:model-value="(v: string) => values['turnstile_secret_key'] = v"
						placeholder="Cloudflare Turnstile Secret Key"
					/>
				</AFormItem>
			</div>
		</div>

		<!-- 双因素认证 -->
		<div class="section">
			<div class="section-title">双因素认证（2FA）</div>
			<template v-if="!totpEnabled">
				<div class="section-desc">启用双因素认证后，登录时需要输入动态验证码，大幅提升账号安全性。</div>
				<AButton type="primary" @click="handleSetup" :loading="setupLoading">开启 2FA</AButton>
			</template>
			<template v-else>
				<AAlert type="success" class="mb-4">已启用双因素认证</AAlert>
				<ASpace>
					<AButton @click="showRegenerateModal = true">重新生成恢复码</AButton>
					<AButton status="danger" @click="showDisableModal = true">禁用 2FA</AButton>
				</ASpace>
			</template>
		</div>

		<!-- 2FA Setup Modal -->
		<AModal v-model:visible="showSetupModal" title="设置双因素认证" :maskClosable="false" :footer="false" width="480px">
			<div v-if="setupStep === 1">
				<AAlert type="info" class="mb-4">请使用身份验证器 App（如 Google Authenticator、Microsoft Authenticator）扫描下方二维码。</AAlert>
				<div class="qr-container">
					<img :src="qrCodeUrl" alt="QR Code" v-if="qrCodeUrl" class="qr-image" />
				</div>
				<div class="secret-key">
					<span class="label">手动输入密钥：</span>
					<ATypographyText copyable code>{{ setupSecret }}</ATypographyText>
				</div>
				<AButton type="primary" long @click="setupStep = 2" class="mt-4">我已扫描，下一步</AButton>
			</div>
			<div v-if="setupStep === 2">
				<AForm :model="enableForm" @submit-success="handleEnable" layout="vertical">
					<AFormItem label="输入验证码" field="code" :rules="[{ required: true, message: '请输入6位验证码' }, { length: 6, message: '验证码为6位' }]">
						<AInput v-model="enableForm.code" placeholder="6位数字验证码" maxlength="6" />
					</AFormItem>
					<AFormItem label="当前密码" field="password" :rules="[{ required: true, message: '请输入密码' }]">
						<AInputPassword v-model="enableForm.password" placeholder="请输入当前密码以确认" />
					</AFormItem>
					<AButton type="primary" html-type="submit" long :loading="enableLoading">确认启用</AButton>
				</AForm>
			</div>
		</AModal>

		<!-- Backup Codes Modal -->
		<AModal v-model:visible="showBackupCodesModal" title="备用恢复码" :maskClosable="false" width="400px">
			<AAlert type="warning" class="mb-4">请妥善保存以下恢复码。每个恢复码只能使用一次。关闭后无法再次查看。</AAlert>
			<div class="backup-codes-grid">
				<ATag v-for="code in backupCodes" :key="code" size="large" class="backup-code">{{ code }}</ATag>
			</div>
			<template #footer>
				<AButton type="primary" @click="showBackupCodesModal = false">我已保存</AButton>
			</template>
		</AModal>

		<!-- Disable Modal -->
		<AModal v-model:visible="showDisableModal" title="禁用双因素认证" @ok="handleDisable" :maskClosable="false">
			<AForm :model="disableForm" layout="vertical">
				<AFormItem label="输入验证码或恢复码">
					<AInput v-model="disableForm.code" placeholder="6位验证码或8位恢复码" />
				</AFormItem>
			</AForm>
		</AModal>

		<!-- Regenerate Backup Codes Modal -->
		<AModal v-model:visible="showRegenerateModal" title="重新生成恢复码" @ok="handleRegenerate" :maskClosable="false">
			<AForm layout="vertical">
				<AFormItem label="输入验证码">
					<AInput v-model="regenerateCode" placeholder="6位数字验证码" maxlength="6" />
				</AFormItem>
			</AForm>
		</AModal>
	</div>
</template>

<style scoped>
@import './common.css';
.section-desc {
	font-size: 12px;
	color: var(--color-text-3);
	margin-bottom: 12px;
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
