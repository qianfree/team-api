<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { Message } from '@arco-design/web-vue'
import qrcode from 'qrcode-generator'
import request, { extractApiError } from '@/utils/request'
import BackupCodesPanel from './BackupCodesPanel.vue'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{
	(e: 'update:visible', v: boolean): void
	(e: 'done'): void
}>()

// ── 步骤状态：1 扫码绑定 / 2 验证启用 / 3 保存恢复码 ──
const step = ref(1)
const setupLoading = ref(false)
const secret = ref('')
const otpauthUri = ref('')
const qrDataUrl = ref('')
const expiresAt = ref(0)
const now = ref(Date.now())
let timer: number | undefined

const countdownText = computed(() => {
	const left = Math.max(0, Math.floor((expiresAt.value - now.value) / 1000))
	return `${Math.floor(left / 60)}:${String(left % 60).padStart(2, '0')}`
})
// pending secret 在后端 Redis 仅存 5 分钟，归零后需重新生成（后端覆盖旧键，安全）
const expired = computed(() => step.value === 1 && expiresAt.value > 0 && now.value >= expiresAt.value)

// ── 步骤 2 ──
const code = ref('')
const password = ref('')
const verifyLoading = ref(false)
const verifyError = ref('')

// ── 步骤 3 ──
const backupCodes = ref<string[]>([])
const savedConfirmed = ref(false)

function startTimer() {
	stopTimer()
	timer = window.setInterval(() => {
		now.value = Date.now()
	}, 1000)
}
function stopTimer() {
	if (timer !== undefined) {
		window.clearInterval(timer)
		timer = undefined
	}
}

function reset() {
	step.value = 1
	secret.value = ''
	otpauthUri.value = ''
	qrDataUrl.value = ''
	expiresAt.value = 0
	code.value = ''
	password.value = ''
	verifyError.value = ''
	backupCodes.value = []
	savedConfirmed.value = false
}

async function copyText(text: string, label = '已复制') {
	try {
		await navigator.clipboard.writeText(text)
		Message.success(label)
	} catch {
		Message.error('复制失败，请手动选择复制')
	}
}

async function fetchSetup() {
	setupLoading.value = true
	try {
		const { data } = await request.post('/admin/security/2fa/setup', {}, { _suppressErrorMsg: true })
		secret.value = data.data.secret
		otpauthUri.value = data.data.uri
		// 本地渲染二维码：otpauth URI 内嵌密钥，绝不能提交给第三方 QR 图片服务
		const qr = qrcode(0, 'M')
		qr.addData(otpauthUri.value)
		qr.make()
		qrDataUrl.value = qr.createDataURL(6, 8)
		expiresAt.value = Date.now() + 300_000
	} catch (e) {
		Message.error(extractApiError(e)?.message || '生成密钥失败，请稍后重试')
		closeModal()
	} finally {
		setupLoading.value = false
	}
}

watch(
	() => props.visible,
	(v) => {
		if (v) {
			reset()
			startTimer()
			fetchSetup()
		} else {
			stopTimer()
		}
	},
)
onUnmounted(stopTimer)

function closeModal() {
	emit('update:visible', false)
}

function goStep2() {
	step.value = 2
}

function goStep1() {
	// 上一步：密钥仍在有效期，无需重新 setup
	step.value = 1
	verifyError.value = ''
}

async function onEnable() {
	if (!/^\d{6}$/.test(code.value)) {
		verifyError.value = '请输入 6 位数字验证码'
		return
	}
	if (!password.value) {
		verifyError.value = '请输入当前密码'
		return
	}
	verifyLoading.value = true
	verifyError.value = ''
	try {
		const { data } = await request.post(
			'/admin/security/2fa/enable',
			{ code: code.value, password: password.value },
			{ _suppressErrorMsg: true },
		)
		backupCodes.value = data.data.backup_codes ?? []
		step.value = 3
	} catch (e) {
		const msg = extractApiError(e)?.message || '启用失败'
		// pending secret 已过期（后端 Redis 无暂存）→ 回步骤 1 重新生成密钥
		if (msg.includes('请先执行2FA设置')) {
			Message.warning('密钥已过期，已为你重新生成，请重新扫码')
			step.value = 1
			code.value = ''
			password.value = ''
			fetchSetup()
		} else {
			verifyError.value = msg
		}
	} finally {
		verifyLoading.value = false
	}
}

function finish() {
	closeModal()
	emit('done')
}
</script>

<template>
	<a-modal
		:visible="props.visible"
		title="开启两步验证"
		:width="640"
		:mask-closable="false"
		:footer="false"
		@update:visible="(v: boolean) => emit('update:visible', v)"
	>
		<div class="setup-wizard">
			<a-steps :current="step" size="small" class="wizard-steps" change-on-click>
				<a-step>扫码绑定</a-step>
				<a-step>验证启用</a-step>
				<a-step>保存恢复码</a-step>
			</a-steps>

			<!-- 步骤 1：扫码绑定 -->
			<div v-if="step === 1" class="step-body">
				<div class="qr-col">
					<div class="qr-box" :class="{ 'is-expired': expired }">
						<a-spin v-if="setupLoading" />
						<img v-else-if="qrDataUrl" :src="qrDataUrl" alt="TOTP 绑定二维码" />
						<div v-if="expired" class="qr-mask">
							<div class="qr-mask__text">二维码已过期</div>
							<a-button size="small" type="primary" :loading="setupLoading" @click="fetchSetup">重新生成密钥</a-button>
						</div>
					</div>
					<div class="qr-countdown">密钥有效期剩余 {{ countdownText }}</div>
				</div>
				<div class="secret-col">
					<p class="hint">
						使用 <b>阿里云 App（虚拟MFA）</b>、Google Authenticator 等 TOTP 验证器扫描左侧二维码；
						无法扫码时可手动输入下方密钥。
					</p>
					<div class="secret-box">
						<code>{{ secret }}</code>
						<a-button size="mini" @click="copyText(secret, '密钥已复制')">复制</a-button>
					</div>
					<p class="hint dim">绑定后，登录时需输入验证器上显示的 6 位动态码。</p>
				</div>
			</div>

			<!-- 步骤 2：验证启用 -->
			<div v-if="step === 2" class="step-body">
				<a-alert type="info" class="mb16">
					输入验证器 App 当前显示的 6 位动态码，并输入当前密码完成安全确认。
				</a-alert>
				<a-form layout="vertical">
					<a-form-item label="动态验证码">
						<a-input
							v-model="code"
							placeholder="6 位数字"
							:max-length="6"
							:input-attrs="{ inputmode: 'numeric' }"
						/>
					</a-form-item>
					<a-form-item label="当前密码">
						<a-input-password v-model="password" placeholder="请输入当前密码" allow-clear />
					</a-form-item>
				</a-form>
				<div v-if="verifyError" class="field-error">{{ verifyError }}</div>
			</div>

			<!-- 步骤 3：保存恢复码 -->
			<div v-if="step === 3" class="step-body">
				<a-alert type="success" class="mb16">两步验证已开启。请立即保存以下恢复码。</a-alert>
				<BackupCodesPanel :codes="backupCodes" />
				<a-checkbox v-model="savedConfirmed" class="saved-check">我已妥善保存恢复码</a-checkbox>
			</div>

			<div class="wizard-footer">
				<template v-if="step === 1">
					<a-button @click="closeModal">取消</a-button>
					<a-button type="primary" :disabled="expired || setupLoading || !qrDataUrl" @click="goStep2">
						我已扫码，下一步
					</a-button>
				</template>
				<template v-else-if="step === 2">
					<a-button @click="goStep1">上一步</a-button>
					<a-button type="primary" :loading="verifyLoading" @click="onEnable">确认开启</a-button>
				</template>
				<template v-else>
					<a-button type="primary" :disabled="!savedConfirmed" @click="finish">完成</a-button>
				</template>
			</div>
		</div>
	</a-modal>
</template>

<style scoped>
.wizard-steps {
	margin-bottom: 20px;
}
.step-body {
	display: flex;
	gap: 20px;
	min-height: 240px;
	flex-direction: row;
}
.qr-col {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 8px;
	flex-shrink: 0;
}
.qr-box {
	position: relative;
	width: 200px;
	height: 200px;
	display: flex;
	align-items: center;
	justify-content: center;
	background: #fff;
	border: 1px solid var(--ta-border-light);
	border-radius: 8px;
	overflow: hidden;
}
.qr-box img {
	width: 184px;
	height: 184px;
}
.qr-mask {
	position: absolute;
	inset: 0;
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	gap: 10px;
	background: rgba(255, 255, 255, 0.92);
}
.qr-mask__text {
	font-size: 13px;
	color: var(--ta-text-secondary, #86909c);
}
.qr-countdown {
	font-size: 12px;
	color: var(--ta-text-tertiary);
}
.secret-col {
	flex: 1;
	min-width: 0;
}
.hint {
	font-size: 13px;
	line-height: 1.7;
	color: var(--ta-text-secondary, #4e5969);
	margin: 0 0 12px;
}
.hint.dim {
	color: var(--ta-text-tertiary);
}
.secret-box {
	display: flex;
	align-items: center;
	gap: 8px;
	padding: 10px 12px;
	background: var(--color-fill-1);
	border-radius: 6px;
	margin-bottom: 12px;
}
.secret-box code {
	flex: 1;
	font-size: 13px;
	word-break: break-all;
	user-select: all;
}
.field-error {
	margin-top: 4px;
	font-size: 13px;
	color: rgb(var(--danger-6));
}
.saved-check {
	margin-top: 4px;
}
.mb16 {
	margin-bottom: 16px;
}
.wizard-footer {
	display: flex;
	justify-content: flex-end;
	gap: 8px;
	margin-top: 16px;
	padding-top: 16px;
	border-top: 1px solid var(--ta-border-lighter);
}

@media (max-width: 600px) {
	.step-body {
		flex-direction: column;
		align-items: center;
	}
}
</style>
