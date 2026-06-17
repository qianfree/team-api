<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import Icon from '@/components/common/Icon.vue'
import BaseModal from '@/components/common/BaseModal.vue'
import request from '@/utils/request'
import { usePublicSettings } from '@/composables/usePublicSettings'

const loading = ref(false)
const userInfo = ref<any>(null)
const sessions = ref<any[]>([])
const { settings } = usePublicSettings()

// OAuth state
const oauthProviders = ref<any[]>([])
const oauthLoading = ref(false)
const linkLoading = ref(false)
const linkError = ref('')

const editingDisplayName = ref(false)
const displayNameForm = reactive({
	display_name: '',
})
const displayNameSaving = ref(false)

const passwordForm = reactive({
	old_password: '',
	new_password: '',
})
const passwordErrors = reactive<Record<string, string>>({})
const passwordSaving = ref(false)
const passwordSuccess = ref(false)
const showPasswordModal = ref(false)

// Email (set / change) modal state
const showEmailModal = ref(false)
const emailForm = reactive({
	new_email: '',
	code: '',
})
const emailSending = ref(false)
const emailSaving = ref(false)
const emailCooldown = ref(0)
const emailError = ref('')
let emailCooldownTimer: ReturnType<typeof setInterval> | null = null

const roleLabel: Record<string, string> = {
	owner: '所有者',
	admin: '管理员',
	member: '成员',
}

async function fetchProfile() {
	loading.value = true
	try {
		const res: any = await request.get('/tenant/profile')
		userInfo.value = res.data.data
		displayNameForm.display_name = res.data?.data?.display_name || ''
	} catch {
		// silent
	} finally {
		loading.value = false
	}
}

async function fetchSessions() {
	try {
		const res: any = await request.get('/tenant/auth/sessions')
		const raw = res.data?.data; sessions.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
	} catch {
		sessions.value = []
	}
}

async function saveDisplayName() {
	if (!displayNameForm.display_name.trim()) return

	displayNameSaving.value = true
	try {
		await request.put('/tenant/profile', {
			display_name: displayNameForm.display_name,
		})
		if (userInfo.value) {
			userInfo.value.display_name = displayNameForm.display_name
		}
		editingDisplayName.value = false
	} catch {
	} finally {
		displayNameSaving.value = false
	}
}

function cancelEditDisplayName() {
	displayNameForm.display_name = userInfo.value?.display_name || ''
	editingDisplayName.value = false
}

function validatePassword(): boolean {
	Object.keys(passwordErrors).forEach((k) => delete passwordErrors[k])

	if (!passwordForm.old_password) {
		passwordErrors.old_password = '请输入当前密码'
	}
	if (!passwordForm.new_password) {
		passwordErrors.new_password = '请输入新密码'
	} else if (passwordForm.new_password.length < 8) {
		passwordErrors.new_password = '密码长度至少 8 位'
	}

	return Object.keys(passwordErrors).length === 0
}

function openPasswordModal() {
	passwordForm.old_password = ''
	passwordForm.new_password = ''
	Object.keys(passwordErrors).forEach((k) => delete passwordErrors[k])
	showPasswordModal.value = true
}

async function handleChangePassword() {
	if (!validatePassword()) return

	passwordSaving.value = true
	passwordSuccess.value = false
	try {
		await request.put('/tenant/auth/change-password', {
			old_password: passwordForm.old_password,
			new_password: passwordForm.new_password,
		})
		passwordForm.old_password = ''
		passwordForm.new_password = ''
		showPasswordModal.value = false
		passwordSuccess.value = true
		setTimeout(() => {
			passwordSuccess.value = false
		}, 3000)
	} catch {
	} finally {
		passwordSaving.value = false
	}
}

function openEmailModal() {
	emailForm.new_email = userInfo.value?.email || ''
	emailForm.code = ''
	emailError.value = ''
	emailCooldown.value = 0
	if (emailCooldownTimer) {
		clearInterval(emailCooldownTimer)
		emailCooldownTimer = null
	}
	showEmailModal.value = true
}

async function sendEmailCode() {
	emailError.value = ''
	if (!emailForm.new_email.trim()) {
		emailError.value = '请输入邮箱'
		return
	}
	emailSending.value = true
	try {
		await request.post('/tenant/email/send-change-email-code', {
			new_email: emailForm.new_email,
		})
		emailCooldown.value = 60
		emailCooldownTimer = setInterval(() => {
			emailCooldown.value--
			if (emailCooldown.value <= 0 && emailCooldownTimer) {
				clearInterval(emailCooldownTimer)
				emailCooldownTimer = null
			}
		}, 1000)
	} catch (e: any) {
		emailError.value = e?.response?.data?.message || '验证码发送失败'
	} finally {
		emailSending.value = false
	}
}

async function handleChangeEmail() {
	emailError.value = ''
	if (!emailForm.new_email.trim()) {
		emailError.value = '请输入邮箱'
		return
	}
	if (!emailForm.code.trim()) {
		emailError.value = '请输入验证码'
		return
	}
	emailSaving.value = true
	try {
		await request.post('/tenant/email/change-email', {
			new_email: emailForm.new_email,
			code: emailForm.code,
		})
		if (userInfo.value) {
			userInfo.value.email = emailForm.new_email
		}
		showEmailModal.value = false
		if (emailCooldownTimer) {
			clearInterval(emailCooldownTimer)
			emailCooldownTimer = null
		}
	} catch (e: any) {
		emailError.value = e?.response?.data?.message || '设置失败'
	} finally {
		emailSaving.value = false
	}
}

async function revokeSession(sessionId: number) {
	try {
		await request.delete(`/tenant/auth/sessions/${sessionId}`)
		sessions.value = sessions.value.filter((s) => s.id !== sessionId)
	} catch {
	}
}

function parseDeviceInfo(deviceInfo: string): string {
	try {
		const info = JSON.parse(deviceInfo || '{}')
		return info.user_agent || deviceInfo || '未知'
	} catch {
		return deviceInfo || '未知'
	}
}

onMounted(() => {
	fetchProfile()
	fetchSessions()
	fetchOAuthProviders()
})

onUnmounted(() => {
	if (emailCooldownTimer) {
		clearInterval(emailCooldownTimer)
		emailCooldownTimer = null
	}
})

async function fetchOAuthProviders() {
	oauthLoading.value = true
	try {
		const res: any = await request.get('/tenant/oauth/providers')
		oauthProviders.value = res.data?.data?.list || []
	} catch {
		oauthProviders.value = []
	} finally {
		oauthLoading.value = false
	}
}

const hasOAuthEnabled = computed(() => {
	return settings.value['oauth_github_enabled'] || settings.value['oauth_google_enabled']
})

function isProviderLinked(provider: string): boolean {
	return oauthProviders.value.some((p: any) => p.provider === provider)
}

function getLinkedProvider(provider: string) {
	return oauthProviders.value.find((p: any) => p.provider === provider)
}

async function handleOAuthLink(provider: string) {
	linkLoading.value = true
	linkError.value = ''
	try {
		const res: any = await request.get('/tenant/oauth/authorize', { params: { provider } })
		const { authorize_url } = res.data?.data || {}
		if (authorize_url) {
			// Open OAuth in popup window
			const width = 600
			const height = 700
			const left = (window.innerWidth - width) / 2
			const top = (window.innerHeight - height) / 2
			const popup = window.open(
				authorize_url,
				'oauth',
				`width=${width},height=${height},left=${left},top=${top}`
			)
			// Listen for callback message
			const handleMessage = async (event: MessageEvent) => {
				if (event.data?.type === 'oauth-callback') {
					window.removeEventListener('message', handleMessage)
					if (event.data.code) {
						try {
							await request.post('/tenant/oauth/link', {
								provider,
								code: event.data.code,
							})
							await fetchOAuthProviders()
						} catch (e: any) {
							const res = e?.response?.data
							linkError.value = res?.message || '绑定失败'
						}
					}
					popup?.close()
					linkLoading.value = false
				}
			}
			window.addEventListener('message', handleMessage)
		}
	} catch (e: any) {
		const res = e?.response?.data
		linkError.value = res?.message || '绑定失败'
	} finally {
		linkLoading.value = false
	}
}

async function handleOAuthUnlink(provider: string) {
	try {
		await request.post('/tenant/oauth/unlink', { provider })
		await fetchOAuthProviders()
	} catch {
		// error toast shown by interceptor
	}
}
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="flex items-start justify-between">
			<div>
				<h1 class="page-title">个人设置</h1>
				<p class="page-description">管理您的个人信息和安全设置</p>
			</div>
			<div class="flex items-center gap-2">
				<router-link to="/tenant/login-history" class="btn btn-secondary btn-sm">
					<Icon name="clock" size="sm" />
					登录历史
				</router-link>
				<button class="btn btn-secondary btn-sm" @click="openPasswordModal">
					<Icon name="shield" size="sm" />
					修改密码
				</button>
			</div>
		</div>

		<!-- Success message -->
		<div
			v-if="passwordSuccess"
			class="p-3 bg-emerald-50 border border-emerald-200 rounded-xl flex items-center gap-2"
		>
			<Icon name="checkCircle" size="md" class="text-emerald-500 flex-shrink-0" />
			<span class="text-sm text-emerald-700 font-medium">密码修改成功</span>
		</div>

		<!-- Skeleton -->
		<div v-if="loading" class="card p-6 space-y-4">
			<div class="skeleton h-5 w-24 mb-4"></div>
			<div class="grid grid-cols-1 md:grid-cols-2 gap-y-5 gap-x-8">
				<div v-for="i in 4" :key="i">
					<div class="skeleton h-4 w-16 mb-2"></div>
					<div class="skeleton h-5 w-32"></div>
				</div>
			</div>
		</div>

		<!-- Personal Info Card -->
		<div v-else-if="userInfo" class="card">
			<div class="card-header">
				<h2 class="font-semibold text-gray-900">个人信息</h2>
			</div>
			<div class="card-body">
				<div class="grid grid-cols-1 md:grid-cols-2 gap-y-5 gap-x-8">
					<!-- Display Name -->
					<div>
						<p class="input-label">显示名称</p>
						<div v-if="!editingDisplayName" class="flex items-center gap-2">
							<span class="text-gray-900 font-medium">{{ userInfo.display_name || userInfo.username }}</span>
							<button
								@click="editingDisplayName = true"
								class="text-primary-600 hover:text-primary-500 transition-colors"
							>
								<Icon name="edit" size="sm" />
							</button>
						</div>
						<div v-else class="flex items-center gap-2">
							<input
								v-model="displayNameForm.display_name"
								type="text"
								class="input flex-1"
								@keyup.enter="saveDisplayName"
							/>
							<button
								@click="saveDisplayName"
								:disabled="displayNameSaving"
								class="btn btn-primary btn-sm"
							>
								保存
							</button>
							<button
								@click="cancelEditDisplayName"
								class="btn btn-secondary btn-sm"
							>
								取消
							</button>
						</div>
					</div>

					<!-- Username -->
					<div>
						<p class="input-label">用户名</p>
						<span class="text-gray-900 font-medium">{{ userInfo.username }}</span>
					</div>

					<!-- Email -->
					<div>
						<p class="input-label">邮箱</p>
						<div class="flex items-center gap-2">
							<span class="text-gray-900 font-medium">{{ userInfo.email || '未设置' }}</span>
							<button
								@click="openEmailModal"
								class="text-primary-600 hover:text-primary-500 transition-colors"
								title="设置/修改邮箱"
							>
								<Icon name="edit" size="sm" />
							</button>
						</div>
					</div>

					<!-- Role -->
					<div>
						<p class="input-label">角色</p>
						<span class="badge badge-primary">
							{{ roleLabel[userInfo.role] || userInfo.role }}
						</span>
					</div>

					<!-- Status -->
					<div>
						<p class="input-label">状态</p>
						<span class="badge badge-success">{{ userInfo.status || 'active' }}</span>
					</div>

					<!-- Joined -->
					<div>
						<p class="input-label">加入时间</p>
						<span class="text-gray-900 font-medium">{{ userInfo.created_at || '--' }}</span>
					</div>
				</div>
			</div>
		</div>


		<!-- OAuth Linked Accounts Card -->
		<div v-if="hasOAuthEnabled" class="card">
			<div class="card-header">
				<h2 class="font-semibold text-gray-900">第三方账号</h2>
			</div>
			<div class="divide-y divide-gray-100">
				<div v-if="settings['oauth_github_enabled']" class="px-6 py-4 flex items-center justify-between">
					<div class="flex items-center gap-3">
						<div class="h-10 w-10 rounded-xl bg-gray-100 flex items-center justify-center">
							<svg class="h-5 w-5 text-gray-700" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
						</div>
						<div>
							<p class="text-sm font-medium text-gray-900">GitHub</p>
							<p v-if="isProviderLinked('github')" class="text-xs text-gray-500 mt-0.5">已绑定：{{ getLinkedProvider('github')?.provider_username || getLinkedProvider('github')?.provider_user_id }}</p>
							<p v-else class="text-xs text-gray-500 mt-0.5">未绑定</p>
						</div>
					</div>
					<button v-if="isProviderLinked('github')" class="btn btn-ghost btn-sm text-red-600 hover:bg-red-50" @click="handleOAuthUnlink('github')">解绑</button>
					<button v-else class="btn btn-secondary btn-sm" :disabled="linkLoading" @click="handleOAuthLink('github')">绑定</button>
				</div>
				<div v-if="settings['oauth_google_enabled']" class="px-6 py-4 flex items-center justify-between">
					<div class="flex items-center gap-3">
						<div class="h-10 w-10 rounded-xl bg-gray-100 flex items-center justify-center">
							<svg class="h-5 w-5" viewBox="0 0 24 24"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/></svg>
						</div>
						<div>
							<p class="text-sm font-medium text-gray-900">Google</p>
							<p v-if="isProviderLinked('google')" class="text-xs text-gray-500 mt-0.5">已绑定：{{ getLinkedProvider('google')?.provider_username || getLinkedProvider('google')?.provider_user_id }}</p>
							<p v-else class="text-xs text-gray-500 mt-0.5">未绑定</p>
						</div>
					</div>
					<button v-if="isProviderLinked('google')" class="btn btn-ghost btn-sm text-red-600 hover:bg-red-50" @click="handleOAuthUnlink('google')">解绑</button>
					<button v-else class="btn btn-secondary btn-sm" :disabled="linkLoading" @click="handleOAuthLink('google')">绑定</button>
				</div>
			</div>
			<p v-if="linkError" class="px-6 pb-4 text-xs text-red-500">{{ linkError }}</p>
		</div>

		<!-- Active Sessions Card -->
		<div class="card">
			<div class="card-header">
				<div class="flex items-center justify-between w-full">
					<div>
						<h2 class="font-semibold text-gray-900">活跃会话</h2>
						<p class="text-sm text-gray-500 mt-0.5">管理您当前登录的设备</p>
					</div>
					<button class="btn btn-ghost btn-sm" @click="fetchSessions">
						<Icon name="refresh" size="sm" />
						刷新
					</button>
				</div>
			</div>
			<div class="divide-y divide-gray-100">
				<div
					v-for="session in sessions"
					:key="session.id"
					class="px-6 py-4 flex items-center justify-between hover:bg-gray-50/50 transition-colors"
				>
					<div class="flex items-center gap-4">
						<!-- Device Icon -->
						<div class="h-10 w-10 rounded-xl bg-gray-100 flex items-center justify-center flex-shrink-0">
							<Icon name="clipboard" size="md" class="text-gray-500" />
						</div>
						<div>
							<div class="flex items-center gap-2">
								<p class="text-sm font-medium text-gray-900">{{ parseDeviceInfo(session.device_info) }}</p>
								<span
									v-if="session.is_current"
									class="badge badge-success"
								>
									当前
								</span>
							</div>
							<p class="text-xs text-gray-500 mt-0.5">
								{{ session.ip_address }} &middot; {{ session.created_at }}
							</p>
						</div>
					</div>

					<button
						v-if="!session.is_current"
						@click="revokeSession(session.id)"
						class="btn btn-ghost btn-sm text-red-600 hover:bg-red-50"
					>
						撤销
					</button>
				</div>

				<!-- Empty state -->
				<div v-if="sessions.length === 0" class="px-6 py-8 text-center text-sm text-gray-500">
					暂无会话
				</div>
			</div>
		</div>

		<!-- Change Password Modal -->
		<BaseModal :show="showPasswordModal" title="修改密码" width="narrow" @close="showPasswordModal = false">
			<form @submit.prevent="handleChangePassword" class="space-y-4">
				<div>
					<label class="input-label">当前密码</label>
					<input
						v-model="passwordForm.old_password"
						type="password"
						placeholder="请输入当前密码"
						class="input"
						:class="{ 'input-error': passwordErrors.old_password }"
					/>
					<p v-if="passwordErrors.old_password" class="input-error-text">{{ passwordErrors.old_password }}</p>
				</div>

				<div>
					<label class="input-label">新密码</label>
					<input
						v-model="passwordForm.new_password"
						type="password"
						placeholder="至少 8 位字符"
						class="input"
						:class="{ 'input-error': passwordErrors.new_password }"
					/>
					<p v-if="passwordErrors.new_password" class="input-error-text">{{ passwordErrors.new_password }}</p>
				</div>
			</form>

			<template #footer>
				<button class="btn btn-secondary" @click="showPasswordModal = false">取消</button>
				<button
					class="btn btn-primary"
					:disabled="passwordSaving"
					@click="handleChangePassword"
				>
					{{ passwordSaving ? '修改中...' : '确认修改' }}
				</button>
			</template>
		</BaseModal>

	<!-- Email Modal -->
	<BaseModal :show="showEmailModal" :title="userInfo?.email ? '修改邮箱' : '设置邮箱'" width="narrow" @close="showEmailModal = false">
		<form @submit.prevent="handleChangeEmail" class="space-y-4">
			<div>
				<label class="input-label">新邮箱</label>
				<div class="flex gap-2">
					<input
						v-model="emailForm.new_email"
						type="email"
						placeholder="请输入邮箱"
						class="input flex-1"
					/>
					<button
						type="button"
						class="btn btn-secondary whitespace-nowrap"
						:disabled="emailSending || emailCooldown > 0"
						@click="sendEmailCode"
					>
						{{ emailCooldown > 0 ? `${emailCooldown}s` : (emailSending ? '发送中...' : '发送验证码') }}
					</button>
				</div>
			</div>

			<div>
				<label class="input-label">验证码</label>
				<input
					v-model="emailForm.code"
					type="text"
					maxlength="6"
					placeholder="请输入6位验证码"
					class="input"
				/>
			</div>

			<p v-if="emailError" class="input-error-text">{{ emailError }}</p>
		</form>

		<template #footer>
			<button class="btn btn-secondary" @click="showEmailModal = false">取消</button>
			<button
				class="btn btn-primary"
				:disabled="emailSaving"
				@click="handleChangeEmail"
			>
				{{ emailSaving ? '设置中...' : '确认' }}
			</button>
		</template>
	</BaseModal>
	</div>
</template>
