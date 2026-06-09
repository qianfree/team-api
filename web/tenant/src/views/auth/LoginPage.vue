<script setup lang="ts">
import { ref, reactive, onMounted, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import { usePublicSettings } from '@/composables/usePublicSettings'
import { extractApiError } from '@/utils/request'
import request from '@/utils/request'
import AuthLayout from '@/components/layout/AuthLayout.vue'
import Icon from '@/components/common/Icon.vue'
import SlideCaptcha from '@/components/common/SlideCaptcha.vue'
import Turnstile from '@/components/common/Turnstile.vue'

const router = useRouter()
const route = useRoute()
const authStore = useTenantAuthStore()
const { settings, fetchSettings } = usePublicSettings()

type LoginMode = 'admin' | 'ram'

const mode = ref<LoginMode>('admin')
const loading = ref(false)
const showPassword = ref(false)
const errorMsg = ref('')

const adminForm = reactive({ email: '', password: '', remember: false })
const ramForm = reactive({ account: '', password: '', remember: false })

const adminErrors = reactive<Record<string, string>>({})
const ramErrors = reactive<Record<string, string>>({})

// 2FA state
const show2FA = ref(false)
const provisionalToken = ref('')
const totpCode = ref('')
const totpLoading = ref(false)

// Captcha state — always required
const captcha = reactive({ captchaKey: '', captchaX: 0 })
	const turnstileToken = ref('')
const captchaRef = ref<InstanceType<typeof SlideCaptcha> | null>(null)

const emailInput = ref<HTMLInputElement | null>(null)
const accountInput = ref<HTMLInputElement | null>(null)

onMounted(async () => {
	nextTick(() => emailInput.value?.focus())
	await fetchSettings()
})

function switchMode(m: LoginMode) {
	mode.value = m
	errorMsg.value = ''
	showPassword.value = false
	nextTick(() => {
		if (m === 'admin') emailInput.value?.focus()
		else accountInput.value?.focus()
	})
}

function validateAdmin(): boolean {
	Object.keys(adminErrors).forEach((k) => delete adminErrors[k])
	errorMsg.value = ''

	if (!adminForm.email.trim()) {
		adminErrors.email = '请输入邮箱'
	} else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(adminForm.email)) {
		adminErrors.email = '邮箱格式不正确'
	}

	if (!adminForm.password) {
		adminErrors.password = '请输入密码'
	}

	return Object.keys(adminErrors).length === 0
}

function validateRam(): boolean {
	Object.keys(ramErrors).forEach((k) => delete ramErrors[k])
	errorMsg.value = ''

	if (!ramForm.account.trim()) {
		ramErrors.account = '请输入 RAM 用户名'
	} else if (!ramForm.account.includes('@')) {
		ramErrors.account = '格式：用户名@组织代码'
	}

	if (!ramForm.password) {
		ramErrors.password = '请输入密码'
	}

	return Object.keys(ramErrors).length === 0
}

async function handleAdminLogin() {
	if (!validateAdmin()) return
	if (!captcha.captchaKey || !captcha.captchaX) {
		errorMsg.value = '请先完成滑块验证'
		return
	}

	loading.value = true
	errorMsg.value = ''
	try {
		const captchaPayload = { captchaKey: captcha.captchaKey, captchaX: captcha.captchaX }
		const res = await authStore.login(adminForm.email, adminForm.password, 'admin', captchaPayload)
		if (res?.totp_required) {
			provisionalToken.value = res.provisional_token
			show2FA.value = true
			return
		}
		const redirect = (route.query.redirect as string) || '/tenant/dashboard'
		await router.push(redirect)
	} catch (err) {
		const apiErr = extractApiError(err)
		if (apiErr?.code === 10058) {
			errorMsg.value = '滑块验证失败，请重新拖动'
		} else {
			errorMsg.value = apiErr?.message || '登录失败，请检查邮箱和密码'
		}
		captchaRef.value?.resetCaptcha()
	} finally {
		loading.value = false
	}
}

async function handleRamLogin() {
	if (!validateRam()) return
	if (!captcha.captchaKey || !captcha.captchaX) {
		errorMsg.value = '请先完成滑块验证'
		return
	}

	loading.value = true
	errorMsg.value = ''
	try {
		const captchaPayload = { captchaKey: captcha.captchaKey, captchaX: captcha.captchaX }
		const res = await authStore.login(ramForm.account, ramForm.password, 'ram', captchaPayload)
		if (res?.totp_required) {
			provisionalToken.value = res.provisional_token
			show2FA.value = true
			return
		}
		const redirect = (route.query.redirect as string) || '/tenant/dashboard'
		await router.push(redirect)
	} catch (err) {
		const apiErr = extractApiError(err)
		if (apiErr?.code === 10058) {
			errorMsg.value = '滑块验证失败，请重新拖动'
		} else {
			errorMsg.value = apiErr?.message || '登录失败，请检查用户名和密码'
		}
		captchaRef.value?.resetCaptcha()
	} finally {
		loading.value = false
	}
}

async function handle2FAVerify() {
	if (!totpCode.value) return
	totpLoading.value = true
	try {
		const res = await request.post('/tenant/auth/2fa/verify', {
			provisional_token: provisionalToken.value,
			code: totpCode.value,
		}, { _suppressErrorMsg: true } as any)
		authStore.applyTokensFrom2FA(res.data.data)
		const redirect = (route.query.redirect as string) || '/tenant/dashboard'
		await router.push(redirect)
	} catch (e: any) {
		const apiErr = extractApiError(e)
		errorMsg.value = apiErr?.message || '验证失败'
	} finally {
		totpLoading.value = false
	}
}

function clearAdminError(field: string) {
	delete adminErrors[field]
	if (errorMsg.value) errorMsg.value = ''
}

function clearRamError(field: string) {
	delete ramErrors[field]
	if (errorMsg.value) errorMsg.value = ''
}
</script>

<template>
	<AuthLayout>
		<!-- 2FA Verification -->
		<div v-if="show2FA" class="animate-slide-up">
			<div class="mb-6 text-center">
				<h2 class="text-xl font-bold text-gray-900">双因素认证</h2>
				<p class="mt-1.5 text-sm text-gray-500">请输入身份验证器中的 6 位数字码</p>
			</div>
			<div v-if="errorMsg" class="mb-4 px-3 py-2 rounded-lg bg-red-50 text-red-600 text-sm">
				{{ errorMsg }}
			</div>
			<div class="space-y-4">
				<div>
					<input v-model="totpCode" type="text" class="input text-center text-lg tracking-widest" placeholder="000000" maxlength="6" @keydown.enter="handle2FAVerify" />
				</div>
				<button class="btn btn-primary w-full" :disabled="totpLoading" @click="handle2FAVerify">
					{{ totpLoading ? '验证中...' : '验证并登录' }}
				</button>
				<button type="button" class="w-full text-sm text-gray-500 hover:text-gray-700 transition-colors py-2" @click="show2FA = false; totpCode = ''; errorMsg = ''">
					返回登录
				</button>
			</div>
		</div>

		<div v-else class="animate-slide-up">
			<!-- Header -->
			<div class="mb-6 text-center">
				<h2 class="text-xl font-bold text-gray-900">欢迎回来</h2>
				<p class="mt-1.5 text-sm text-gray-500">登录您的组织账户以继续</p>
			</div>

			<!-- Login Mode Tabs -->
			<div class="tabs mb-6">
				<button
					type="button"
					class="tab flex-1"
					:class="{ 'tab-active': mode === 'admin' }"
					@click="switchMode('admin')"
				>
					管理员登录
				</button>
				<button
					type="button"
					class="tab flex-1"
					:class="{ 'tab-active': mode === 'ram' }"
					@click="switchMode('ram')"
				>
					RAM 账号登录
				</button>
			</div>

			<!-- Admin Login Form -->
			<form v-if="mode === 'admin'" @submit.prevent="handleAdminLogin" class="space-y-5" novalidate>
				<!-- Email -->
				<div>
					<label for="admin-email" class="input-label">邮箱</label>
					<div class="relative">
						<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
							<Icon name="mail" size="sm" />
						</div>
						<input
							ref="emailInput"
							id="admin-email"
							v-model="adminForm.email"
							type="email"
							placeholder="admin@example.com"
							autocomplete="email"
							class="input pl-11"
							:class="{ 'input-error': adminErrors.email }"
							@input="clearAdminError('email')"
						/>
					</div>
					<p v-if="adminErrors.email" class="input-error-text">{{ adminErrors.email }}</p>
				</div>

				<!-- Password -->
				<div>
					<label for="admin-password" class="input-label">密码</label>
					<div class="relative">
						<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
							<Icon name="lock" size="sm" />
						</div>
						<input
							id="admin-password"
							v-model="adminForm.password"
							:type="showPassword ? 'text' : 'password'"
							placeholder="请输入密码"
							autocomplete="current-password"
							class="input pl-11 pr-11"
							:class="{ 'input-error': adminErrors.password }"
							@input="clearAdminError('password')"
						/>
						<button
							type="button"
							@click="showPassword = !showPassword"
							class="absolute inset-y-0 right-0 flex items-center pr-3 text-gray-400 hover:text-gray-600 transition-colors"
							:aria-label="showPassword ? '隐藏密码' : '显示密码'"
						>
							<Icon :name="showPassword ? 'eyeOff' : 'eye'" size="sm" />
						</button>
					</div>
					<p v-if="adminErrors.password" class="input-error-text">{{ adminErrors.password }}</p>
				</div>

				<!-- Captcha -->
				<SlideCaptcha ref="captchaRef" v-model="captcha" />

				<!-- Error Message -->
				<transition name="slide-fade">
					<div v-if="errorMsg" class="rounded-xl border border-red-200 bg-red-50 px-4 py-3">
						<div class="flex items-start gap-2">
							<Icon name="exclamationCircle" size="sm" class="mt-0.5 text-red-500 flex-shrink-0" />
							<p class="text-sm text-red-600">{{ errorMsg }}</p>
						</div>
					</div>
				</transition>

				<!-- Remember & Forgot -->
				<div class="flex items-center justify-between">
					<label class="flex items-center gap-2 cursor-pointer select-none">
						<input
							v-model="adminForm.remember"
							type="checkbox"
							class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500/30 transition-colors cursor-pointer"
						/>
						<span class="text-sm text-gray-600">记住登录</span>
					</label>
					<router-link
						to="/tenant/forgot-password"
						class="text-sm text-primary-600 font-medium hover:text-primary-700 transition-colors"
					>
						忘记密码？
					</router-link>
				</div>

				<!-- Submit -->
				<button type="submit" :disabled="loading" class="btn btn-primary btn-lg w-full">
					<div v-if="loading" class="spinner h-4 w-4 border-white"></div>
					{{ loading ? '登录中...' : '登录' }}
				</button>
			</form>

			<!-- RAM Login Form -->
			<form v-else @submit.prevent="handleRamLogin" class="space-y-5" novalidate>
				<!-- RAM Account -->
				<div>
					<label for="ram-account" class="input-label">RAM 用户名</label>
					<div class="relative">
						<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
							<Icon name="user" size="sm" />
						</div>
						<input
							ref="accountInput"
							id="ram-account"
							v-model="ramForm.account"
							type="text"
							placeholder="用户名@组织代码"
							autocomplete="username"
							class="input pl-11"
							:class="{ 'input-error': ramErrors.account }"
							@input="clearRamError('account')"
						/>
					</div>
					<p class="input-hint">格式：用户名@组织代码</p>
					<p v-if="ramErrors.account" class="input-error-text">{{ ramErrors.account }}</p>
				</div>

				<!-- Password -->
				<div>
					<label for="ram-password" class="input-label">密码</label>
					<div class="relative">
						<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
							<Icon name="lock" size="sm" />
						</div>
						<input
							id="ram-password"
							v-model="ramForm.password"
							:type="showPassword ? 'text' : 'password'"
							placeholder="请输入密码"
							autocomplete="current-password"
							class="input pl-11 pr-11"
							:class="{ 'input-error': ramErrors.password }"
							@input="clearRamError('password')"
						/>
						<button
							type="button"
							@click="showPassword = !showPassword"
							class="absolute inset-y-0 right-0 flex items-center pr-3 text-gray-400 hover:text-gray-600 transition-colors"
							:aria-label="showPassword ? '隐藏密码' : '显示密码'"
						>
							<Icon :name="showPassword ? 'eyeOff' : 'eye'" size="sm" />
						</button>
					</div>
					<p v-if="ramErrors.password" class="input-error-text">{{ ramErrors.password }}</p>
				</div>

				<!-- Captcha -->
				<SlideCaptcha ref="captchaRef" v-model="captcha" />

				<!-- Error Message -->
				<transition name="slide-fade">
					<div v-if="errorMsg" class="rounded-xl border border-red-200 bg-red-50 px-4 py-3">
						<div class="flex items-start gap-2">
							<Icon name="exclamationCircle" size="sm" class="mt-0.5 text-red-500 flex-shrink-0" />
							<p class="text-sm text-red-600">{{ errorMsg }}</p>
						</div>
					</div>
				</transition>

				<!-- Remember & Forgot -->
				<div class="flex items-center justify-between">
					<label class="flex items-center gap-2 cursor-pointer select-none">
						<input
							v-model="ramForm.remember"
							type="checkbox"
							class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500/30 transition-colors cursor-pointer"
						/>
						<span class="text-sm text-gray-600">记住登录</span>
					</label>
					<router-link
						to="/tenant/forgot-password"
						class="text-sm text-primary-600 font-medium hover:text-primary-700 transition-colors"
					>
						忘记密码？
					</router-link>
				</div>

				<!-- Submit -->
				<button type="submit" :disabled="loading" class="btn btn-primary btn-lg w-full">
					<div v-if="loading" class="spinner h-4 w-4 border-white"></div>
					{{ loading ? '登录中...' : '登录' }}
				</button>
			</form>

		</div>

		<template v-if="settings.register_enabled" #footer>
			<p class="text-gray-500">
				还没有组织？
				<router-link to="/tenant/register" class="text-primary-600 font-medium hover:text-primary-700 transition-colors">
					立即创建
				</router-link>
			</p>
		</template>
	</AuthLayout>
</template>

<style scoped>
.slide-fade-enter-active {
	transition: all 0.25s ease-out;
}
.slide-fade-leave-active {
	transition: all 0.2s ease-in;
}
.slide-fade-enter-from {
	opacity: 0;
	transform: translateY(-6px);
}
.slide-fade-leave-to {
	opacity: 0;
	transform: translateY(-4px);
}
</style>
