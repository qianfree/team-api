<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import Icon from '@/components/common/Icon.vue'
import BaseModal from '@/components/common/BaseModal.vue'
import request from '@/utils/request'

const loading = ref(false)
const userInfo = ref<any>(null)
const sessions = ref<any[]>([])

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
})
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
						<span class="text-gray-900 font-medium">{{ userInfo.email || '--' }}</span>
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
	</div>
</template>
