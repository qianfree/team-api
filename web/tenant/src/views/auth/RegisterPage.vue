<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import { usePublicSettings } from '@/composables/usePublicSettings'
import AuthLayout from '@/components/layout/AuthLayout.vue'
import SlideCaptcha from '@/components/common/SlideCaptcha.vue'
import Turnstile from '@/components/common/Turnstile.vue'
import Icon from '@/components/common/Icon.vue'
import AgreementViewModal from '@/components/common/AgreementViewModal.vue'
import request from '@/utils/request'
import { extractApiError } from '@/utils/request'

const router = useRouter()
const authStore = useTenantAuthStore()
const { settings, fetchSettings } = usePublicSettings()

const step = ref(1)
const loading = ref(false)
const codeSending = ref(false)
const countdown = ref(0)
const emailVerification = ref(true)
const captcha = ref<{ captchaKey: string; captchaX: number }>({ captchaKey: '', captchaX: 0 })
const captchaRef = ref<InstanceType<typeof SlideCaptcha> | null>(null)
	const turnstileToken = ref('')
let countdownTimer: ReturnType<typeof setInterval> | null = null

const orgForm = reactive({ orgName: '', orgCode: '' })
const userForm = reactive({
	email: '',
	code: '',
	password: '',
	confirmPassword: '',
	username: '',
	agreed: false,
})

const orgErrors = reactive<Record<string, string>>({})
const userErrors = reactive<Record<string, string>>({})

// Pending agreements

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

const stepTitles = ['组织信息', '管理员信息']

onMounted(async () => {
	await fetchSettings()
	emailVerification.value = settings.value.register_email_verification === true
})

function validateOrg(): boolean {
	Object.keys(orgErrors).forEach((k) => delete orgErrors[k])

	if (!orgForm.orgName.trim()) {
		orgErrors.orgName = '请输入组织名称'
	}

	if (!orgForm.orgCode.trim()) {
		orgErrors.orgCode = '请输入组织代码'
	} else if (!/^[a-z][a-z0-9-]{2,28}[a-z0-9]$/.test(orgForm.orgCode)) {
		orgErrors.orgCode = '仅支持小写字母、数字和连字符（3-30 位）'
	}

	return Object.keys(orgErrors).length === 0
}

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

function goNext() {
	if (!validateOrg()) return
	step.value = 2
}

function goBack() {
	step.value = 1
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
		const payload = {
			email: userForm.email,
			password: userForm.password,
			tenant_name: orgForm.orgName,
			tenant_code: orgForm.orgCode,
			username: userForm.username,
			code: emailVerification.value ? userForm.code : undefined,
			captcha_key: emailVerification.value ? undefined : captcha.value.captchaKey,
			captcha_x: emailVerification.value ? undefined : captcha.value.captchaX,
		}

		await authStore.register(payload)
		proceedAfterRegister()
	} catch (err: any) {
			captchaRef.value?.resetCaptcha()
		const apiErr = extractApiError(err)
		const msg = apiErr?.message || '注册失败'
		if (msg.includes('邮箱') || msg.includes('email')) {
			userErrors.email = msg
		} else if (msg.includes('验证码') || msg.includes('code') || msg.includes('滑块') || msg.includes('captcha')) {
			userErrors.code = msg
		} else if (msg.includes('密码') || msg.includes('password')) {
			userErrors.password = msg
		} else if (msg.includes('组织') || msg.includes('tenant')) {
			orgErrors.orgName = msg
			step.value = 1
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
				<h2 class="text-xl font-bold text-gray-900">创建您的组织</h2>
				<p class="mt-1.5 text-sm text-gray-500">几分钟即可开始使用 Team API</p>
			</div>

			<!-- Step Indicator -->
			<div class="mb-6 flex items-center gap-3">
				<div
					v-for="(title, i) in stepTitles"
					:key="i"
					class="flex flex-1 items-center gap-2"
				>
					<div
						class="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full text-xs font-semibold transition-all duration-300"
						:class="[
							step > i + 1
								? 'bg-primary-500 text-white'
								: step === i + 1
									? 'bg-primary-500 text-white'
									: 'bg-gray-100 text-gray-400',
						]"
					>
						<Icon v-if="step > i + 1" name="check" size="xs" />
						<span v-else>{{ i + 1 }}</span>
					</div>
					<span
						class="text-xs font-medium transition-colors duration-300"
						:class="step >= i + 1 ? 'text-gray-900' : 'text-gray-400'"
					>
						{{ title }}
					</span>
					<div
						v-if="i < stepTitles.length - 1"
						class="h-px flex-1 transition-colors duration-300"
						:class="step > i + 1 ? 'bg-primary-500' : 'bg-gray-200'"
					/>
				</div>
			</div>

			<!-- Step 1: Organization Info -->
			<form v-if="step === 1" @submit.prevent="goNext" class="space-y-4">
				<div>
					<label class="input-label">组织名称</label>
					<input
						v-model="orgForm.orgName"
						type="text"
						placeholder="例如：某某科技"
						class="input"
						:class="{ 'input-error': orgErrors.orgName }"
					/>
					<p v-if="orgErrors.orgName" class="input-error-text">{{ orgErrors.orgName }}</p>
				</div>

				<div>
					<label class="input-label">组织代码</label>
					<input
						v-model="orgForm.orgCode"
						type="text"
						placeholder="例如：acme-corp"
						class="input"
						:class="{ 'input-error': orgErrors.orgCode }"
					/>
					<p class="input-hint">组织的唯一标识符，仅支持小写字母、数字和连字符</p>
					<p v-if="orgErrors.orgCode" class="input-error-text">{{ orgErrors.orgCode }}</p>
				</div>

				<button type="submit" class="btn btn-primary btn-lg w-full">
					下一步
					<Icon name="arrowRight" size="sm" />
				</button>
			</form>

			<!-- Step 2: Personal Info -->
			<form v-else @submit.prevent="handleRegister" class="space-y-4">
				<!-- Org Summary -->
				<div class="rounded-xl bg-gray-50 px-4 py-3 flex items-center justify-between">
					<div class="min-w-0">
						<p class="text-sm font-medium text-gray-900 truncate">{{ orgForm.orgName }}</p>
						<p class="text-xs text-gray-500">{{ orgForm.orgCode }}</p>
					</div>
					<button type="button" @click="goBack" class="text-xs text-primary-600 font-medium hover:text-primary-700 transition-colors flex-shrink-0 ml-3">
						修改
					</button>
				</div>

				<!-- Username -->
				<div>
					<label class="input-label">用户名</label>
					<div class="relative">
						<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
							<Icon name="user" size="sm" />
						</div>
						<input
							v-model="userForm.username"
							type="text"
							placeholder="仅支持英文字母和数字，不能为纯数字"
							class="input pl-11"
							:class="{ 'input-error': userErrors.username }"
						@input="validateUsernameRealtime"
					/>
					</div>
					<p v-if="userErrors.username" class="input-error-text">{{ userErrors.username }}</p>
				</div>

				<!-- Email -->
				<div>
					<label class="input-label">邮箱</label>
					<div class="flex gap-2">
						<div class="relative flex-1">
							<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
								<Icon name="mail" size="sm" />
							</div>
							<input
								v-model="userForm.email"
								type="email"
								placeholder="you@example.com"
								class="input pl-11"
								:class="{ 'input-error': userErrors.email }"
							/>
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
						<input
							v-model="userForm.code"
							type="text"
							placeholder="请输入 6 位验证码"
							maxlength="6"
							class="input"
							:class="{ 'input-error': userErrors.code }"
						/>
						<p v-if="userErrors.code" class="input-error-text">{{ userErrors.code }}</p>
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
					<input
						v-model="userForm.password"
						type="password"
						placeholder="至少 8 位字符"
						class="input"
						:class="{ 'input-error': userErrors.password }"
					/>
					<p v-if="userErrors.password" class="input-error-text">{{ userErrors.password }}</p>
				</div>

				<!-- Confirm Password -->
				<div>
					<label class="input-label">确认密码</label>
					<input
						v-model="userForm.confirmPassword"
						type="password"
						placeholder="请再次输入密码"
						class="input"
						:class="{ 'input-error': userErrors.confirmPassword }"
					/>
					<p v-if="userErrors.confirmPassword" class="input-error-text">{{ userErrors.confirmPassword }}</p>
				</div>

				<!-- Agreement Checkbox -->
				<div class="flex items-start gap-2.5">
					<label class="mt-0.5 cursor-pointer">
						<input
							v-model="userForm.agreed"
							type="checkbox"
							class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500/30 transition-colors cursor-pointer"
						/>
					</label>
					<span class="text-xs text-gray-500 leading-relaxed">
						我已阅读并同意
						<button type="button" class="text-primary-600 hover:text-primary-700 underline underline-offset-2" @click.prevent="openAgreement('terms')">服务条款</button>
						和
						<button type="button" class="text-primary-600 hover:text-primary-700 underline underline-offset-2" @click.prevent="openAgreement('privacy')">隐私政策</button>
					</span>
				</div>

				<!-- Submit -->
				<button type="submit" :disabled="loading || !userForm.agreed" class="btn btn-primary btn-lg w-full disabled:opacity-50 disabled:cursor-not-allowed">
					<div v-if="loading" class="spinner h-4 w-4 border-white"></div>
					{{ loading ? '创建中...' : '创建组织' }}
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
