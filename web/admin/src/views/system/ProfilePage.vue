<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { Form, FormItem, Input, Button, Message } from '@arco-design/web-vue'
import type { FormInstance } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()

const user = computed(() => authStore.user)
const roleLabel = computed(() => {
	const role = user.value?.role
	if (role === 'super_admin') return '超级管理员'
	if (role === 'admin') return '管理员'
	return role || ''
})

const pwdFormRef = ref<FormInstance | null>(null)
const pwdLoading = ref(false)
const pwdForm = reactive({
	old_password: '',
	new_password: '',
	confirm_password: '',
})

async function handleChangePassword() {
	const err = await pwdFormRef.value?.validate()
	if (err) return
	pwdLoading.value = true
	try {
		await request.put('/admin/auth/change-password', {
			old_password: pwdForm.old_password,
			new_password: pwdForm.new_password,
		})
		Message.success('密码修改成功')
		pwdForm.old_password = ''
		pwdForm.new_password = ''
		pwdForm.confirm_password = ''
	} catch (e: any) {
		Message.error(e?.response?.data?.message || '密码修改失败')
	} finally {
		pwdLoading.value = false
	}
}
</script>

<template>
	<div>
		<PageHeader title="个人信息" description="查看账号信息和修改密码" />

		<div class="profile-grid">
			<div class="profile-card">
				<div class="profile-card__header">账号信息</div>
				<div class="profile-card__body">
					<div class="profile-field">
						<span class="profile-field__label">用户名</span>
						<span class="profile-field__value">{{ user?.username || '-' }}</span>
					</div>
					<div class="profile-field">
						<span class="profile-field__label">显示名称</span>
						<span class="profile-field__value">{{ user?.display_name || '-' }}</span>
					</div>
					<div class="profile-field">
						<span class="profile-field__label">角色</span>
						<span class="profile-field__value">{{ roleLabel }}</span>
					</div>
				</div>
			</div>

			<div class="profile-card">
				<div class="profile-card__header">修改密码</div>
				<div class="profile-card__body">
					<Form ref="pwdFormRef" :model="pwdForm" layout="vertical" @submit-success="handleChangePassword">
						<FormItem field="old_password" label="当前密码" :rules="[{ required: true, message: '请输入当前密码' }]" AsteriskPosition="end">
							<Input v-model="pwdForm.old_password" type="password" placeholder="请输入当前密码" />
						</FormItem>
						<FormItem field="new_password" label="新密码" :rules="[{ required: true, message: '请输入新密码' }, { minLength: 8, message: '密码长度至少8位' }]" AsteriskPosition="end">
							<Input v-model="pwdForm.new_password" type="password" placeholder="请输入新密码（至少8位）" />
						</FormItem>
						<FormItem field="confirm_password" label="确认新密码" :rules="[{ required: true, message: '请确认新密码' }, {
							validator: (value, cb) => {
								if (value !== pwdForm.new_password) cb('两次输入的密码不一致')
								else cb()
							},
						}]" AsteriskPosition="end">
							<Input v-model="pwdForm.confirm_password" type="password" placeholder="请再次输入新密码" />
						</FormItem>
						<FormItem>
							<Button type="primary" html-type="submit" :loading="pwdLoading">修改密码</Button>
						</FormItem>
					</Form>
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped>
.profile-grid {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 20px;
}

@media (max-width: 768px) {
	.profile-grid {
		grid-template-columns: 1fr;
	}
}

.profile-card {
	background: var(--ta-bg-card);
	border-radius: 12px;
	border: 1px solid var(--ta-border-light);
}

.profile-card__header {
	padding: 16px 20px;
	font-size: 15px;
	font-weight: 600;
	color: var(--ta-text-primary);
	border-bottom: 1px solid var(--ta-border-light);
}

.profile-card__body {
	padding: 20px;
}

.profile-field {
	display: flex;
	align-items: center;
	padding: 10px 0;
}

.profile-field + .profile-field {
	border-top: 1px solid var(--ta-border-lighter);
}

.profile-field__label {
	width: 80px;
	flex-shrink: 0;
	font-size: 13px;
	color: var(--ta-text-tertiary);
}

.profile-field__value {
	font-size: 14px;
	color: var(--ta-text-primary);
}
</style>
