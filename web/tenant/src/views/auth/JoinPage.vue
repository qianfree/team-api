<template>
	<AuthLayout>
		<!-- Loading state -->
		<div v-if="checking" class="py-8 flex justify-center">
			<div class="spinner h-6 w-6 border-primary-500"></div>
		</div>

		<!-- Invalid invitation -->
		<div v-else-if="!inviteValid" class="py-4 text-center">
			<div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-red-100">
				<Icon name="xCircle" size="xl" class="text-red-600" />
			</div>
			<h3 class="text-lg font-semibold text-gray-900">邀请链接无效</h3>
			<p class="mt-1 text-sm text-gray-500">该链接已过期、已被使用或已撤销</p>
			<router-link to="/tenant/login" class="btn btn-primary mt-6 inline-flex">
				前往登录
			</router-link>
		</div>

		<!-- Join form -->
		<template v-else-if="!joined">
			<div class="mb-6 text-center">
				<h2 class="text-xl font-bold text-gray-900">加入组织</h2>
				<p v-if="tenantName" class="mt-1 text-sm text-gray-500">
					您正在加入 <span class="font-medium text-gray-700">{{ tenantName }}</span>
				</p>
				<p v-else class="mt-1 text-sm text-gray-500">通过邀请链接加入</p>
				<div v-if="inviteRole" class="mt-2">
					<span class="badge badge-primary">角色：{{ inviteRole === 'admin' ? '管理员' : '成员' }}</span>
				</div>
			</div>

			<form class="space-y-5" @submit.prevent="handleJoin">
				<div>
					<label class="input-label">用户名 <span class="text-red-500">*</span></label>
					<div class="relative">
						<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
							<Icon name="user" size="sm" />
						</div>
						<input
							v-model="form.username"
							type="text"
							required
							placeholder="请输入用户名"
							class="input pl-11"
						/>
					</div>
				</div>

				<div>
					<label class="input-label">显示名称</label>
					<div class="relative">
						<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
							<Icon name="user" size="sm" />
						</div>
						<input
							v-model="form.display_name"
							type="text"
							placeholder="选填，如：张三"
							class="input pl-11"
						/>
					</div>
				</div>

				<div>
					<label class="input-label">邮箱 <span class="text-red-500">*</span></label>
					<div class="relative">
						<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
							<Icon name="mail" size="sm" />
						</div>
						<input
							v-model="form.email"
							type="email"
							required
							placeholder="请输入邮箱"
							class="input pl-11"
						/>
					</div>
				</div>

				<div>
					<label class="input-label">密码 <span class="text-red-500">*</span></label>
					<div class="relative">
						<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
							<Icon name="lock" size="sm" />
						</div>
						<input
							v-model="form.password"
							type="password"
							required
							placeholder="至少 8 位，含大小写字母和数字"
							class="input pl-11"
						/>
					</div>
				</div>

				<button
					type="submit"
					:disabled="loading"
					class="btn btn-primary btn-lg w-full"
				>
					<div v-if="loading" class="spinner h-4 w-4 border-white"></div>
					{{ loading ? '加入中...' : '加入组织' }}
				</button>
			</form>
		</template>

		<!-- Success State -->
		<div v-else class="py-4">
			<div class="text-center">
				<div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-emerald-100">
					<Icon name="checkCircle" size="xl" class="text-emerald-600" />
				</div>
				<h3 class="text-lg font-semibold text-gray-900">加入成功！</h3>
				<p class="mt-1 text-sm text-gray-500">您已成功加入组织，请记住以下登录信息</p>
			</div>

			<div class="mt-6 rounded-xl border border-gray-200 bg-gray-50 divide-y divide-gray-200">
				<div class="flex items-center justify-between px-4 py-3">
					<span class="text-sm text-gray-500">组织名称</span>
					<span class="text-sm font-medium text-gray-900">{{ joinResult.tenant_name }}</span>
				</div>
				<div class="flex items-center justify-between px-4 py-3">
					<span class="text-sm text-gray-500">组织代码</span>
					<span class="font-mono text-sm font-medium text-primary-600">{{ joinResult.tenant_code }}</span>
				</div>
				<div class="flex items-center justify-between px-4 py-3">
					<span class="text-sm text-gray-500">用户名</span>
					<span class="text-sm font-medium text-gray-900">{{ joinResult.username }}</span>
				</div>
				<div class="flex items-center justify-between px-4 py-3">
					<span class="text-sm text-gray-500">角色</span>
					<span class="badge badge-primary">{{ joinResult.role === 'admin' ? '管理员' : '成员' }}</span>
				</div>
				<div class="flex items-center justify-between px-4 py-3">
					<span class="text-sm text-gray-500">RAM 登录账号</span>
					<span class="font-mono text-sm font-medium text-gray-900">{{ joinResult.username }}@{{ joinResult.tenant_code }}</span>
				</div>
			</div>

			<div class="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3">
				<p class="text-xs text-amber-700">
					请牢记您的 RAM 登录账号 <strong>{{ joinResult.username }}@{{ joinResult.tenant_code }}</strong>，下次登录时选择「RAM 账号登录」并使用此账号。
				</p>
			</div>

			<button class="btn btn-primary btn-lg w-full mt-6" @click="router.push('/tenant')">
				进入控制台
			</button>
		</div>

		<template #footer>
			<p v-if="!joined && inviteValid && !checking" class="text-gray-500">
				已有账号？
				<router-link to="/tenant/login" class="text-primary-600 font-medium hover:text-primary-700 transition-colors">
					登录
				</router-link>
			</p>
		</template>
	</AuthLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AuthLayout from '@/components/layout/AuthLayout.vue'
import Icon from '@/components/common/Icon.vue'
import { toast } from '@/utils/toast'
import { useTenantAuthStore } from '@/stores/tenant-auth'

const route = useRoute()
const router = useRouter()
const authStore = useTenantAuthStore()

const checking = ref(true)
const inviteValid = ref(false)
const tenantName = ref('')
const inviteRole = ref('')
const loading = ref(false)
const joined = ref(false)

const joinResult = ref({
	tenant_name: '',
	tenant_code: '',
	username: '',
	role: '',
})

const form = ref({
	username: '',
	display_name: '',
	email: '',
	password: '',
})

onMounted(async () => {
	const code = route.query.code as string
	if (!code) {
		checking.value = false
		inviteValid.value = false
		return
	}
	try {
		const res = await fetch(`/api/tenant/members/invite-info?code=${encodeURIComponent(code)}`)
		const data = await res.json()
		if (data?.data?.valid) {
			inviteValid.value = true
			tenantName.value = data.data.tenant_name || ''
			inviteRole.value = data.data.role || ''
		} else {
			inviteValid.value = false
		}
	} catch {
		inviteValid.value = false
	} finally {
		checking.value = false
	}
})

async function handleJoin() {
	loading.value = true
	try {
		const res = await fetch('/api/tenant/members/join', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				code: route.query.code as string,
				username: form.value.username,
				display_name: form.value.display_name || undefined,
				email: form.value.email,
				password: form.value.password,
			}),
		})
		if (!res.ok) {
			const data = await res.json()
			toast.error(data?.error?.message || data?.message || '加入失败')
			return
		}
		const data = await res.json()
		authStore.applyTokensFrom2FA({
			access_token: data.data.access_token,
			refresh_token: data.data.refresh_token,
			expires_at: data.data.expires_at,
			tenant: {
				id: 0,
				name: data.data.tenant_name || '',
				code: data.data.tenant_code || '',
			},
			user: {
				id: 0,
				username: data.data.username || form.value.username,
				role: data.data.role || '',
			},
		})
		joinResult.value = {
			tenant_name: data.data.tenant_name || '',
			tenant_code: data.data.tenant_code || '',
			username: data.data.username || form.value.username,
			role: data.data.role || '',
		}
		joined.value = true
	} catch {
		toast.error('网络错误')
	} finally {
		loading.value = false
	}
}
</script>
