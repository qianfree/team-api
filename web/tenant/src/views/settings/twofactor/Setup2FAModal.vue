<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { NInput, NCheckbox } from 'naive-ui'
import qrcode from 'qrcode-generator'
import BaseModal from '@/components/common/BaseModal.vue'
import request, { extractApiError } from '@/utils/request'
import { toast } from '@/utils/toast'
import BackupCodesPanel from './BackupCodesPanel.vue'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{
	(e: 'close'): void
	(e: 'done'): void
}>()

// ── 步骤状态：1 扫码绑定 / 2 安全确认(密码) / 3 验证启用 / 4 保存恢复码 ──
// 顺序与后端 Enable2FA 的校验顺序一致：先密码后验证码，密码输错不消耗验证码
const step = ref(1)
const setupLoading = ref(false)
const secret = ref('')
const otpauthUri = ref('')
const qrDataUrl = ref('')
const expiresAt = ref(0)
const now = ref(Date.now())
let timer: ReturnType<typeof setInterval> | null = null

const steps = [
	{ n: 1, label: '扫码绑定' },
	{ n: 2, label: '安全确认' },
	{ n: 3, label: '验证启用' },
	{ n: 4, label: '保存恢复码' },
]

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
	timer = setInterval(() => {
		now.value = Date.now()
	}, 1000)
}
function stopTimer() {
	if (timer) {
		clearInterval(timer)
		timer = null
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

async function copySecret() {
	try {
		await navigator.clipboard.writeText(secret.value)
		toast.success('密钥已复制')
	} catch {
		toast.error('复制失败，请手动选择复制')
	}
}

async function fetchSetup() {
	setupLoading.value = true
	try {
		const res: any = await request.post('/tenant/security/2fa/setup', {}, { _suppressErrorMsg: true })
		secret.value = res.data?.data?.secret || ''
		otpauthUri.value = res.data?.data?.uri || ''
		// 本地渲染二维码：otpauth URI 内嵌密钥，绝不能提交给第三方 QR 图片服务
		if (otpauthUri.value) {
			const qr = qrcode(0, 'M')
			qr.addData(otpauthUri.value)
			qr.make()
			qrDataUrl.value = qr.createDataURL(6, 8)
		}
		expiresAt.value = Date.now() + 300_000
	} catch (e) {
		toast.error(extractApiError(e)?.message || '生成密钥失败，请稍后重试')
		closeModal()
	} finally {
		setupLoading.value = false
	}
}

watch(
	() => props.show,
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
	emit('close')
}

function goStep2() {
	step.value = 2
}

function goStep3() {
	if (!password.value) {
		verifyError.value = '请输入当前密码'
		return
	}
	verifyError.value = ''
	step.value = 3
}

function goStep1() {
	// 上一步：密钥仍在有效期，无需重新 setup
	step.value = 1
	verifyError.value = ''
}

function backToStep2() {
	step.value = 2
	verifyError.value = ''
}

async function onEnable() {
	if (!/^\d{6}$/.test(code.value)) {
		verifyError.value = '请输入 6 位数字验证码'
		return
	}
	verifyLoading.value = true
	verifyError.value = ''
	try {
		const res: any = await request.post(
			'/tenant/security/2fa/enable',
			{ code: code.value, password: password.value },
			{ _suppressErrorMsg: true },
		)
		backupCodes.value = res.data?.data?.backup_codes ?? []
		step.value = 4
	} catch (e) {
		const msg = extractApiError(e)?.message || '启用失败'
		// pending secret 已过期（后端 Redis 无暂存）→ 回步骤 1 重新生成密钥
		if (msg.includes('请先执行2FA设置')) {
			toast.warning('密钥已过期，已为你重新生成，请重新扫码')
			step.value = 1
			code.value = ''
			password.value = ''
			fetchSetup()
		} else if (msg.includes('密码错误')) {
			// 后端先验密码：密码不对回到密码步骤，验证码未被消耗可直接重试
			verifyError.value = msg
			step.value = 2
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
	<BaseModal :show="props.show" title="开启两步验证" width="wide" :close-on-click-outside="false" @close="closeModal">
		<div class="space-y-5">
			<!-- 步骤指示器 -->
			<div class="flex items-center justify-between">
				<template v-for="(s, i) in steps" :key="s.n">
					<div class="flex items-center gap-2">
						<div
							class="h-7 w-7 rounded-full flex items-center justify-center text-xs font-semibold transition-colors"
							:class="step >= s.n ? 'bg-primary-500 text-white' : 'bg-gray-100 text-gray-400'"
						>
							{{ step > s.n ? '✓' : s.n }}
						</div>
						<span class="text-sm" :class="step >= s.n ? 'text-gray-900 font-medium' : 'text-gray-400'">{{ s.label }}</span>
					</div>
					<div v-if="i < steps.length - 1" class="flex-1 h-px mx-3" :class="step > s.n ? 'bg-primary-400' : 'bg-gray-200'" />
				</template>
			</div>

			<!-- 步骤 1：扫码绑定 -->
			<div v-if="step === 1" class="flex flex-col sm:flex-row gap-5">
				<div class="flex flex-col items-center gap-2 flex-shrink-0 mx-auto">
					<div class="relative h-[200px] w-[200px] rounded-xl border border-gray-200 bg-white overflow-hidden flex items-center justify-center">
						<div v-if="setupLoading" class="spinner text-primary-500"></div>
						<img v-else-if="qrDataUrl" :src="qrDataUrl" alt="TOTP 绑定二维码" class="h-[184px] w-[184px]" />
						<div
							v-if="expired"
							class="absolute inset-0 bg-white/95 flex flex-col items-center justify-center gap-2"
						>
							<p class="text-sm text-gray-500">二维码已过期</p>
							<button class="btn btn-primary btn-sm" :disabled="setupLoading" @click="fetchSetup">重新生成密钥</button>
						</div>
					</div>
					<p class="text-xs text-gray-400">密钥有效期剩余 {{ countdownText }}</p>
				</div>
				<div class="flex-1 min-w-0">
					<p class="text-sm leading-relaxed text-gray-600 mb-3">
						使用 <span class="font-medium text-gray-900">阿里云 App（虚拟MFA）</span>、Google Authenticator 等
						TOTP 验证器扫描左侧二维码；无法扫码时可手动输入下方密钥。
					</p>
					<div class="flex items-center gap-2 p-2.5 rounded-lg bg-gray-50 border border-gray-200 mb-3">
						<code class="flex-1 text-xs break-all font-mono text-gray-800 select-all">{{ secret }}</code>
						<button class="btn btn-secondary btn-sm whitespace-nowrap" @click="copySecret">复制</button>
					</div>
					<p class="text-xs text-gray-400">绑定后，登录时需输入验证器上显示的 6 位动态码。</p>
				</div>
			</div>

			<!-- 步骤 2：安全确认（密码） -->
			<div v-if="step === 2" class="space-y-4">
				<div class="p-3 rounded-xl bg-primary-50 border border-primary-100 text-sm text-primary-800">
					为确认是你本人操作，请输入当前登录密码。
				</div>
				<div>
					<label class="input-label">当前密码</label>
					<n-input
						v-model:value="password"
						type="password"
						show-password-on="click"
						placeholder="请输入当前密码"
						@keydown.enter="goStep3"
					/>
				</div>
				<p v-if="verifyError" class="input-error-text">{{ verifyError }}</p>
			</div>

			<!-- 步骤 3：验证启用 -->
			<div v-if="step === 3" class="space-y-4">
				<div class="p-3 rounded-xl bg-primary-50 border border-primary-100 text-sm text-primary-800">
					输入验证器 App 当前显示的 6 位动态码完成开启。
				</div>
				<div>
					<label class="input-label">动态验证码</label>
					<n-input
						v-model:value="code"
						placeholder="6 位数字"
						:maxlength="6"
						:input-props="{ inputmode: 'numeric' }"
						@keydown.enter="onEnable"
					/>
				</div>
				<p v-if="verifyError" class="input-error-text">{{ verifyError }}</p>
			</div>

			<!-- 步骤 4：保存恢复码 -->
			<div v-if="step === 4" class="space-y-4">
				<div class="p-3 rounded-xl bg-emerald-50 border border-emerald-200 text-sm text-emerald-700">
					两步验证已开启。请立即保存以下恢复码。
				</div>
				<BackupCodesPanel :codes="backupCodes" />
				<n-checkbox v-model:checked="savedConfirmed">我已妥善保存恢复码</n-checkbox>
			</div>
		</div>

		<template #footer>
			<template v-if="step === 1">
				<button class="btn btn-secondary" @click="closeModal">取消</button>
				<button class="btn btn-primary" :disabled="expired || setupLoading || !qrDataUrl" @click="goStep2">
					我已扫码，下一步
				</button>
			</template>
			<template v-else-if="step === 2">
				<button class="btn btn-secondary" @click="goStep1">上一步</button>
				<button class="btn btn-primary" @click="goStep3">下一步</button>
			</template>
			<template v-else-if="step === 3">
				<button class="btn btn-secondary" @click="backToStep2">上一步</button>
				<button class="btn btn-primary" :disabled="verifyLoading" @click="onEnable">
					{{ verifyLoading ? '开启中...' : '确认开启' }}
				</button>
			</template>
			<template v-else>
				<button class="btn btn-primary" :disabled="!savedConfirmed" @click="finish">完成</button>
			</template>
		</template>
	</BaseModal>
</template>
