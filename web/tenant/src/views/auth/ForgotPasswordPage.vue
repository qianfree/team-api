<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import AuthLayout from '@/components/layout/AuthLayout.vue'
import Icon from '@/components/common/Icon.vue'
import SlideCaptcha from '@/components/common/SlideCaptcha.vue'
import request, { extractApiError } from '@/utils/request'

const router = useRouter()

const step = ref<1 | 2>(1)
const loading = ref(false)
const codeSending = ref(false)
const countdown = ref(0)
let countdownTimer: ReturnType<typeof setInterval> | null = null

const form = reactive({
	email: '',
	code: '',
	newPassword: '',
	confirmPassword: '',
})

const errors = reactive<Record<string, string>>({})

// Captcha state
const captcha = reactive({ captchaKey: '', captchaX: 0 })

function clearErrors() {
	Object.keys(errors).forEach((k) => delete errors[k])
}

async function sendCode() {
	if (!form.email.trim() || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email)) {
		errors.email = '请输入有效的邮箱地址'
		return
	}

	codeSending.value = true
	try {
		await request.post('/tenant/email/send-code', {
			email: form.email,
			purpose: 'reset_password',
			captcha_key: captcha.captchaKey,
			captcha_x: captcha.captchaX,
		})
		startCountdown()
	} catch (err: any) {
		const apiErr = extractApiError(err)
		if (apiErr?.code === 10058) {
			errors.email = '滑块验证失败，请重新拖动'
			return
		}
		errors.email = apiErr?.message || '发送验证码失败'
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

function validateStep1(): boolean {
	clearErrors()
	if (!form.email.trim()) {
		errors.email = '请输入邮箱'
	} else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email)) {
		errors.email = '邮箱格式不正确'
	}
	return Object.keys(errors).length === 0
}

function validateStep2(): boolean {
	clearErrors()
	if (!form.code.trim()) {
		errors.code = '请输入验证码'
	}
	if (!form.newPassword) {
		errors.newPassword = '请输入新密码'
	} else if (form.newPassword.length < 8) {
		errors.newPassword = '密码长度至少 8 位'
	}
	if (!form.confirmPassword) {
		errors.confirmPassword = '请确认新密码'
	} else if (form.newPassword !== form.confirmPassword) {
		errors.confirmPassword = '两次输入的密码不一致'
	}
	return Object.keys(errors).length === 0
}

async function goToStep2() {
	if (!validateStep1()) return

	codeSending.value = true
	try {
		await request.post('/tenant/email/send-code', {
			email: form.email,
			purpose: 'reset_password',
			captcha_key: captcha.captchaKey,
			captcha_x: captcha.captchaX,
		})
		step.value = 2
		startCountdown()
	} catch (err: any) {
		const apiErr = extractApiError(err)
		if (apiErr?.code === 10058) {
			errors.email = '滑块验证失败，请重新拖动'
			return
		}
		errors.email = apiErr?.message || '发送验证码失败'
	} finally {
		codeSending.value = false
	}
}

async function handleReset() {
	if (!validateStep2()) return

	loading.value = true
	try {
		await request.post('/tenant/email/reset-password', {
			email: form.email,
			code: form.code,
			password: form.newPassword,
			captcha_key: captcha.captchaKey,
			captcha_x: captcha.captchaX,
		})
		router.push('/tenant/login')
	} catch (err: any) {
		const apiErr = extractApiError(err)
		if (apiErr?.code === 10058) {
			errors.code = '滑块验证失败，请重新拖动'
			return
		}
		errors.code = apiErr?.message || '重置密码失败'
	} finally {
		loading.value = false
	}
}

function goBack() {
	step.value = 1
	clearErrors()
}
</script>

<template>
	<AuthLayout>
		<div class="mb-6 text-center">
			<h2 class="text-xl font-bold text-gray-900">重置密码</h2>
			<p class="mt-1 text-sm text-gray-500">
				{{ step === 1 ? '输入邮箱以接收重置验证码' : '输入验证码和新密码' }}
			</p>
		</div>

		<!-- Step Indicator -->
		<div class="mb-6 flex items-center justify-center gap-3">
			<div
				class="flex h-8 w-8 items-center justify-center rounded-full text-sm font-medium transition-colors"
				:class="step === 1 ? 'bg-primary-500 text-white' : 'bg-primary-100 text-primary-600'"
			>
				1
			</div>
			<div class="h-0.5 w-8 rounded-full" :class="step === 2 ? 'bg-primary-500' : 'bg-gray-200'"></div>
			<div
				class="flex h-8 w-8 items-center justify-center rounded-full text-sm font-medium transition-colors"
				:class="step === 2 ? 'bg-primary-500 text-white' : 'bg-gray-100 text-gray-400'"
			>
				2
			</div>
		</div>

		<!-- Step 1: Email -->
		<form v-if="step === 1" @submit.prevent="goToStep2" class="space-y-5">
			<div>
				<label class="input-label">邮箱地址</label>
				<div class="relative">
					<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
						<Icon name="mail" size="sm" />
					</div>
					<input
						v-model="form.email"
						type="email"
						placeholder="you@example.com"
						class="input pl-11"
						:class="{ 'input-error': errors.email }"
					/>
				</div>
				<p v-if="errors.email" class="input-error-text">{{ errors.email }}</p>
			</div>

			<SlideCaptcha v-model="captcha" class="mb-4" />
			<button
				type="submit"
				:disabled="codeSending"
				class="btn btn-primary btn-lg w-full"
			>
				<div v-if="codeSending" class="spinner h-4 w-4 border-white"></div>
				{{ codeSending ? '发送中...' : '发送重置验证码' }}
			</button>
		</form>

		<!-- Step 2: Code + New Password -->
		<form v-else @submit.prevent="handleReset" class="space-y-5">
			<div>
				<label class="input-label">验证码</label>
				<div class="flex gap-2">
					<input
						v-model="form.code"
						type="text"
						placeholder="请输入 6 位验证码"
						maxlength="6"
						class="input flex-1"
						:class="{ 'input-error': errors.code }"
					/>
					<button
						type="button"
						@click="sendCode"
						:disabled="countdown > 0 || codeSending"
						class="btn btn-secondary flex-shrink-0 whitespace-nowrap"
					>
						{{ countdown > 0 ? `${countdown}s` : '重新发送' }}
					</button>
				</div>
				<p v-if="errors.code" class="input-error-text">{{ errors.code }}</p>
			</div>

			<div>
				<label class="input-label">新密码</label>
				<input
					v-model="form.newPassword"
					type="password"
					placeholder="至少 8 位字符"
					class="input"
					:class="{ 'input-error': errors.newPassword }"
				/>
				<p v-if="errors.newPassword" class="input-error-text">{{ errors.newPassword }}</p>
			</div>

			<div>
				<label class="input-label">确认新密码</label>
				<input
					v-model="form.confirmPassword"
					type="password"
					placeholder="请再次输入新密码"
					class="input"
					:class="{ 'input-error': errors.confirmPassword }"
				/>
				<p v-if="errors.confirmPassword" class="input-error-text">{{ errors.confirmPassword }}</p>
			</div>

			<div class="flex gap-3">
				<button
					type="button"
					@click="goBack"
					class="btn btn-secondary flex-1"
				>
					返回
				</button>
				<button
					type="submit"
					:disabled="loading"
					class="btn btn-primary flex-1"
				>
					<div v-if="loading" class="spinner h-4 w-4 border-white"></div>
					{{ loading ? '重置中...' : '重置密码' }}
				</button>
			</div>
		</form>

		<template #footer>
			<p class="text-gray-500">
				想起密码了？
				<router-link to="/tenant/login" class="text-primary-600 font-medium hover:text-primary-700 transition-colors">
					返回登录
				</router-link>
			</p>
		</template>
	</AuthLayout>
</template>
