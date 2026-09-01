<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { NInput, NCheckbox } from 'naive-ui'
import { useRouter } from 'vue-router'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import { usePublicSettings } from '@/composables/usePublicSettings'
import AuthLayout from '@/components/layout/AuthLayout.vue'
import SlideCaptcha from '@/components/common/SlideCaptcha.vue'
import Icon from '@/components/common/Icon.vue'
import AgreementViewModal from '@/components/common/AgreementViewModal.vue'
import PasswordStrengthMeter from '@/components/common/PasswordStrengthMeter.vue'
import request from '@/utils/request'
import { extractApiError } from '@/utils/request'

const router = useRouter()
const authStore = useTenantAuthStore()
const { settings, fetchSettings } = usePublicSettings()

const loading = ref(false)
const codeSending = ref(false)
const countdown = ref(0)
const emailVerification = ref(true)
const captcha = ref<{ captchaKey: string; captchaX: number }>({ captchaKey: '', captchaX: 0 })
const captchaRef = ref<InstanceType<typeof SlideCaptcha> | null>(null)
let countdownTimer: ReturnType<typeof setInterval> | null = null

const userForm = reactive({
	email: '',
	code: '',
	password: '',
	confirmPassword: '',
	username: '',
	agreed: false,
})

const userErrors = reactive<Record<string, string>>({})

// 注册限流状态（按 IP 统计的小时/天窗口剩余次数，来自公开端点）
interface RateLimitStatus {
	hourly_limit: number
	hourly_remaining: number
	hourly_reset_seconds: number
	daily_limit: number
	daily_remaining: number
	daily_reset_seconds: number
}
const rateLimit = ref<RateLimitStatus | null>(null)

async function fetchRateLimit() {
	try {
		const res: any = await request.get('/tenant/auth/register-rate-limit')
		rateLimit.value = res.data?.data || null
	} catch {
		// 限流状态查询失败不影响注册流程
	}
}

function formatReset(seconds: number): string {
	if (seconds <= 0) return '稍后'
	if (seconds < 3600) return `约 ${Math.ceil(seconds / 60)} 分钟`
	return `约 ${Math.ceil(seconds / 3600)} 小时`
}

const rateLimitNotice = computed<{ type: 'blocked' | 'info'; text: string } | null>(() => {
	const s = rateLimit.value
	if (!s) return null
	if (s.daily_limit > 0 && s.daily_remaining <= 0) {
		return { type: 'blocked', text: `今日注册次数已达上限（每日 ${s.daily_limit} 次），${formatReset(s.daily_reset_seconds)}后重置` }
	}
	if (s.hourly_limit > 0 && s.hourly_remaining <= 0) {
		return { type: 'blocked', text: `注册已达每小时上限（${s.hourly_limit} 次/小时），${formatReset(s.hourly_reset_seconds)}后重置` }
	}
	if (s.hourly_limit > 0 || s.daily_limit > 0) {
		const parts: string[] = []
		if (s.hourly_limit > 0) parts.push(`本小时 ${s.hourly_remaining}/${s.hourly_limit} 次`)
		if (s.daily_limit > 0) parts.push(`今日 ${s.daily_remaining}/${s.daily_limit} 次`)
		return { type: 'info', text: `剩余注册次数：${parts.join('，')}` }
	}
	return null
})

// Agreement view modal
const showAgreementModal = ref(false)
const agreementModalCode = ref('')

function openAgreement(code: string) {
	agreementModalCode.value = code
	showAgreementModal.value = true
}

function proceedAfterRegister() {
	router.push('/tenant/dashboard')
}

onMounted(async () => {
	// Check if already logged in — if yes, redirect to dashboard
	if (authStore.isLoggedIn) {
		router.replace('/tenant/dashboard')
		return
	}

	await fetchSettings()
	emailVerification.value = settings.value.register_email_verification === true
	fetchRateLimit()
})

function validateUser(): boolean {
	Object.keys(userErrors).forEach((k) => delete userErrors[k])

	if (!userForm.email.trim()) {
		userErrors.email = '请输入邮箱'
	} else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(userForm.email)) {
		userErrors.email = '邮箱格式不正确'
	}

	if (emailVerification.value) {
		if (!userForm.code.trim()) {
			userErrors.code = '请输入验证码'
		}
	} else {
		if (!captcha.value.captchaKey) {
			userErrors.code = '请完成滑块验证'
		}
	}

	if (!userForm.password) {
		userErrors.password = '请输入密码'
	} else if (userForm.password.length < 8) {
		userErrors.password = '密码长度至少 8 位'
	}

	if (!userForm.confirmPassword) {
		userErrors.confirmPassword = '请确认密码'
	} else if (userForm.password !== userForm.confirmPassword) {
		userErrors.confirmPassword = '两次输入的密码不一致'
	}

	if (!userForm.username.trim()) {
		userErrors.username = '请输入用户名'
	} else if (/[^a-zA-Z0-9]/.test(userForm.username)) {
		userErrors.username = '用户名仅支持英文字母和数字'
	} else if (/^\d+$/.test(userForm.username)) {
		userErrors.username = '用户名不能为纯数字'
	} else if (userForm.username.length < 3) {
		userErrors.username = '用户名长度至少 3 位'
	}

	return Object.keys(userErrors).length === 0
}

function validateUsernameRealtime() {
	const val = userForm.username
	if (!val) {
		delete userErrors.username
		return
	}
	if (/[^a-zA-Z0-9]/.test(val)) {
		userErrors.username = '用户名仅支持英文字母和数字'
	} else if (/^\d+$/.test(val)) {
		userErrors.username = '用户名不能为纯数字'
	} else if (val.length < 3) {
		userErrors.username = '用户名长度至少 3 位'
	} else {
		delete userErrors.username
	}
}

async function sendCode() {
	if (!userForm.email.trim() || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(userForm.email)) {
		userErrors.email = '请先输入有效的邮箱地址'
		return
	}
	delete userErrors.code
	delete userErrors.email
	codeSending.value = true
	try {
		await request.post('/tenant/email/send-code', {
			email: userForm.email,
			purpose: 'register',
		})
		startCountdown()
	} catch (err: any) {
		const apiErr = extractApiError(err)
		userErrors.email = apiErr?.message || '发送验证码失败'
	} finally {
		codeSending.value = false
	}
}

function startCountdown() {
	countdown.value = 60
	if (countdownTimer) clearInterval(countdownTimer)
	countdownTimer = setInterval(() => {
		countdown.value--
		if (countdown.value <= 0) {
			clearInterval(countdownTimer!)
			countdownTimer = null
		}
	}, 1000)
}

async function handleRegister() {
	if (!validateUser()) return

	loading.value = true
	try {
		// 简化注册：仅提交账号信息，组织信息由后端自动生成（个人模式，团队功能后续开启）
		const payload = {
			email: userForm.email,
			password: userForm.password,
			username: userForm.username,
			code: emailVerification.value ? userForm.code : undefined,
			captcha_key: emailVerification.value ? undefined : captcha.value.captchaKey,
			captcha_x: emailVerification.value ? undefined : captcha.value.captchaX,
		}

		await authStore.register(payload)
		proceedAfterRegister()
	} catch (err: any) {
		captchaRef.value?.resetCaptcha()
		// 本次尝试已消耗限流计数，刷新剩余次数提示
		fetchRateLimit()
		const apiErr = extractApiError(err)
		const msg = apiErr?.message || '注册失败'
		if (msg.includes('邮箱') || msg.includes('email')) {
			userErrors.email = msg
		} else if (msg.includes('验证码') || msg.includes('code') || msg.includes('滑块') || msg.includes('captcha')) {
			userErrors.code = msg
		} else if (msg.includes('密码') || msg.includes('password')) {
			userErrors.password = msg
		} else if (msg.includes('用户名') || msg.includes('username')) {
			userErrors.username = msg
		} else {
			userErrors.email = msg
		}
	} finally {
		loading.value = false
	}
}
</script>

<template>
	<AuthLayout>
		<!-- Registration disabled -->
		<div v-if="settings.register_enabled === false" class="animate-slide-up text-center">
			<div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-gray-100">
				<Icon name="lock" size="lg" class="text-gray-400" />
			</div>
			<h2 class="text-xl font-bold text-gray-900">注册暂未开放</h2>
			<p class="mt-2 text-sm text-gray-500">当前未开放新用户注册，请联系管理员获取账号</p>
			<router-link to="/tenant/login" class="btn btn-secondary btn-md mt-6 inline-flex">
				返回登录
			</router-link>
		</div>

		<!-- Registration form -->
		<div v-else class="animate-slide-up">
			<!-- Header -->
			<div class="mb-6 text-center">
				<h2 class="text-xl font-bold text-gray-900">创建账号</h2>
				<p class="mt-1.5 text-sm text-gray-500">几分钟即可开始使用，团队功能可后续开启</p>
			</div>

			<!-- 注册限流提示 -->
			<div
				v-if="rateLimitNotice"
				class="mb-4 flex items-start gap-2 rounded-xl border px-4 py-2.5 text-xs"
				:class="rateLimitNotice.type === 'blocked'
					? 'border-red-200 bg-red-50 text-red-600'
					: 'border-amber-200 bg-amber-50 text-amber-700'"
			>
				<Icon
					:name="rateLimitNotice.type === 'blocked' ? 'exclamationTriangle' : 'infoCircle'"
					size="sm"
					class="mt-0.5 shrink-0"
				/>
				<span>{{ rateLimitNotice.text }}</span>
			</div>

			<form @submit.prevent="handleRegister" class="space-y-4">
				<!-- Username -->
				<div>
					<label class="input-label">用户名</label>
					<n-input
						v-model:value="userForm.username"
						type="text"
						placeholder="仅支持英文字母和数字，不能为纯数字"
						:status="userErrors.username ? 'error' : undefined"
						@update:value="validateUsernameRealtime"
					>
						<template #prefix><Icon name="user" size="sm" class="text-gray-400" /></template>
						<template #feedback v-if="userErrors.username">{{ userErrors.username }}</template>
					</n-input>
				</div>

				<!-- Email -->
				<div>
					<label class="input-label">邮箱</label>
					<div class="flex gap-2">
						<div class="flex-1">
							<n-input
								v-model:value="userForm.email"
								type="email"
								placeholder="you@example.com"
								:status="userErrors.email ? 'error' : undefined"
							>
								<template #prefix><Icon name="mail" size="sm" class="text-gray-400" /></template>
							</n-input>
						</div>
						<button
							v-if="emailVerification"
							type="button"
							@click="sendCode"
							:disabled="countdown > 0 || codeSending"
							class="btn btn-secondary btn-sm whitespace-nowrap">
							{{ countdown > 0 ? `${countdown}s` : codeSending ? '发送中...' : '发送验证码' }}
						</button>
					</div>
					<p v-if="userErrors.email" class="input-error-text">{{ userErrors.email }}</p>
				</div>

				<!-- Email Verification Code (shown when email verification is enabled) -->
				<template v-if="emailVerification">
					<div>
						<label class="input-label">验证码</label>
						<n-input
							v-model:value="userForm.code"
							type="text"
							placeholder="请输入 6 位验证码"
							maxlength="6"
							:status="userErrors.code ? 'error' : undefined"
						>
							<template #feedback v-if="userErrors.code">{{ userErrors.code }}</template>
						</n-input>
					</div>
				</template>

				<!-- Slide Captcha (shown when email verification is disabled) -->
				<template v-else>
					<SlideCaptcha ref="captchaRef" v-model="captcha" />
					<p v-if="userErrors.code" class="input-error-text">{{ userErrors.code }}</p>
				</template>

				<!-- Password -->
				<div>
					<label class="input-label">密码</label>
					<n-input
						v-model:value="userForm.password"
						type="password"
						show-password-on="click"
						placeholder="至少 8 位字符"
						:status="userErrors.password ? 'error' : undefined"
					>
						<template #feedback v-if="userErrors.password">{{ userErrors.password }}</template>
					</n-input>
					<PasswordStrengthMeter :password="userForm.password" />
				</div>

				<!-- Confirm Password -->
				<div>
					<label class="input-label">确认密码</label>
					<n-input
						v-model:value="userForm.confirmPassword"
						type="password"
						show-password-on="click"
						placeholder="请再次输入密码"
						:status="userErrors.confirmPassword ? 'error' : undefined"
					>
						<template #feedback v-if="userErrors.confirmPassword">{{ userErrors.confirmPassword }}</template>
					</n-input>
				</div>

				<!-- Agreement Checkbox -->
				<div class="flex items-start gap-2.5">
					<n-checkbox v-model:checked="userForm.agreed" class="mt-0.5" />
					<span class="text-xs text-gray-500 leading-relaxed">
						我已阅读并同意
						<button type="button" class="text-primary-600 hover:text-primary-700 underline underline-offset-2" @click.prevent="openAgreement('terms')">服务条款</button>
						和
						<button type="button" class="text-primary-600 hover:text-primary-700 underline underline-offset-2" @click.prevent="openAgreement('privacy')">隐私政策</button>
					</span>
				</div>

				<!-- Submit -->
				<button type="submit" :disabled="loading || !userForm.agreed || rateLimitNotice?.type === 'blocked'" class="btn btn-primary btn-lg w-full disabled:opacity-50 disabled:cursor-not-allowed">
					<div v-if="loading" class="spinner h-4 w-4 border-white"></div>
					{{ loading ? '创建中...' : '创建账号' }}
				</button>
			</form>
		</div>

		<template #footer>
			<p class="text-gray-500">
				已有账号？
				<router-link to="/tenant/login" class="text-primary-600 font-medium hover:text-primary-700 transition-colors">
					立即登录
				</router-link>
			</p>
		</template>
	</AuthLayout>

	<AgreementViewModal
		:show="showAgreementModal"
		:code="agreementModalCode"
		@close="showAgreementModal = false"
	/>
</template>
