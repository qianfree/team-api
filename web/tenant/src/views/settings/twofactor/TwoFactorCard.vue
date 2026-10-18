<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { NInput } from 'naive-ui'
import BaseModal from '@/components/common/BaseModal.vue'
import Icon from '@/components/common/Icon.vue'
import request, { extractApiError } from '@/utils/request'
import { toast } from '@/utils/toast'
import Setup2FAModal from './Setup2FAModal.vue'
import BackupCodesPanel from './BackupCodesPanel.vue'

const props = defineProps<{ enabled: boolean }>()
const emit = defineEmits<{
	(e: 'changed'): void
}>()

// ── 开启向导 ──
const showSetup = ref(false)
function onSetupDone() {
	showSetup.value = false
	toast.success('两步验证已开启')
	// 通知父组件刷新 profile（totp_enabled 状态）
	emit('changed')
}

// ── 关闭两步验证 ──
const showDisable = ref(false)
const disableCode = ref('')
const disableLoading = ref(false)
const disableError = ref('')
watch(showDisable, (v) => {
	if (v) {
		disableCode.value = ''
		disableError.value = ''
	}
})

async function handleDisable() {
	if (!disableCode.value.trim()) {
		disableError.value = '请输入动态验证码或恢复码'
		return
	}
	disableLoading.value = true
	disableError.value = ''
	try {
		await request.post(
			'/tenant/security/2fa/disable',
			{ code: disableCode.value.trim() },
			{ _suppressErrorMsg: true },
		)
		toast.success('两步验证已关闭')
		emit('changed')
		showDisable.value = false
	} catch (e) {
		disableError.value = extractApiError(e)?.message || '操作失败'
	} finally {
		disableLoading.value = false
	}
}

// ── 重新生成恢复码 ──
const showRegen = ref(false)
const regenCode = ref('')
const regenLoading = ref(false)
const regenError = ref('')
const regenCodes = ref<string[]>([])
watch(showRegen, (v) => {
	if (v) {
		regenCode.value = ''
		regenError.value = ''
		regenCodes.value = []
	}
})

async function handleRegen() {
	if (!regenCode.value.trim()) {
		regenError.value = '请输入动态验证码'
		return
	}
	regenLoading.value = true
	regenError.value = ''
	try {
		const res: any = await request.post(
			'/tenant/security/2fa/backup-codes',
			{ code: regenCode.value.trim() },
			{ _suppressErrorMsg: true },
		)
		regenCodes.value = res.data?.data?.backup_codes ?? []
	} catch (e) {
		regenError.value = extractApiError(e)?.message || '操作失败'
	} finally {
		regenLoading.value = false
	}
}

// 生成成功后表单整块换成恢复码列表，焦点必须跟着送进去，否则读屏用户不知道页面变了
const regenCodesEl = ref<HTMLElement | null>(null)
watch(regenCodes, (v) => {
	if (v.length > 0) nextTick(() => regenCodesEl.value?.focus())
})
</script>

<template>
	<!-- 两步验证卡片 -->
	<div class="card">
		<div class="card-header">
			<div class="flex items-center gap-2">
				<Icon name="shield" size="md" class="text-primary-500" />
				<h2 class="font-semibold text-gray-900">两步验证（TOTP）</h2>
			</div>
		</div>
		<div class="card-body">
			<!-- 未开启态 -->
			<template v-if="!props.enabled">
				<div class="p-3 rounded-xl bg-amber-50 border border-amber-200 text-sm text-amber-700 mb-4">
					当前账号仅受密码保护。开启后登录需额外输入验证器 App（阿里云虚拟MFA、Google Authenticator
					等）中的动态码，可显著降低密码泄露风险。
				</div>
				<button class="btn btn-primary" @click="showSetup = true">
					<Icon name="shield" size="sm" />
					开启两步验证
				</button>
			</template>

			<!-- 已开启态 -->
			<template v-else>
				<div class="flex items-center gap-2.5 mb-2">
					<span class="badge badge-success">已开启</span>
					<span class="text-sm text-gray-600">登录时需输入验证器 App 中的 6 位动态码</span>
				</div>
				<p class="text-xs leading-relaxed text-gray-500 mb-4">
					恢复码用于验证器不可用时登录，忘记或用尽时可在此重新生成（旧恢复码全部作废）。
				</p>
				<div class="flex gap-2">
					<button class="btn btn-secondary btn-sm" @click="showRegen = true">重新生成恢复码</button>
					<button class="btn btn-ghost btn-sm text-red-600 hover:bg-red-50" @click="showDisable = true">
						关闭两步验证
					</button>
				</div>
			</template>
		</div>
	</div>

	<!-- 开启向导 -->
	<Setup2FAModal :show="showSetup" @close="showSetup = false" @done="onSetupDone" />

	<!-- 关闭确认 -->
	<BaseModal :show="showDisable" title="关闭两步验证" width="narrow" @close="showDisable = false">
		<div class="space-y-4">
			<div class="p-3 rounded-xl bg-red-50 border border-red-200 text-sm text-red-700">
				关闭后账号仅受密码保护，安全性降低。
			</div>
			<div>
				<label class="input-label" for="twofa-disable-code">动态验证码或恢复码</label>
				<n-input
					v-model:value="disableCode"
					placeholder="验证器 6 位动态码，或任一未使用的恢复码"
					:input-props="{ id: 'twofa-disable-code' }"
				/>
			</div>
			<p v-if="disableError" role="alert" class="input-error-text">{{ disableError }}</p>
		</div>
		<template #footer>
			<button class="btn btn-secondary" @click="showDisable = false">取消</button>
			<button class="btn btn-danger" :disabled="disableLoading" @click="handleDisable">
				{{ disableLoading ? '关闭中...' : '确认关闭' }}
			</button>
		</template>
	</BaseModal>

	<!-- 重新生成恢复码 -->
	<BaseModal :show="showRegen" title="重新生成恢复码" width="wide" :close-on-click-outside="false" @close="showRegen = false">
		<template v-if="regenCodes.length === 0">
			<div class="space-y-4">
				<div class="p-3 rounded-xl bg-amber-50 border border-amber-200 text-sm text-amber-700">
					重新生成后旧的恢复码将全部作废，请确认手头验证器可用。
				</div>
				<div>
					<label class="input-label" for="twofa-regen-code">动态验证码</label>
					<n-input
						v-model:value="regenCode"
						placeholder="验证器当前显示的 6 位动态码"
						:maxlength="6"
						:input-props="{ id: 'twofa-regen-code', inputmode: 'numeric', autocomplete: 'one-time-code' }"
					/>
				</div>
				<p v-if="regenError" role="alert" class="input-error-text">{{ regenError }}</p>
			</div>
		</template>
		<div v-else ref="regenCodesEl" tabindex="-1" role="group" aria-label="新恢复码" class="outline-none">
			<BackupCodesPanel :codes="regenCodes" />
		</div>
		<template #footer>
			<template v-if="regenCodes.length === 0">
				<button class="btn btn-secondary" @click="showRegen = false">取消</button>
				<button class="btn btn-primary" :disabled="regenLoading" @click="handleRegen">
					{{ regenLoading ? '生成中...' : '生成新恢复码' }}
				</button>
			</template>
			<button v-else class="btn btn-primary" @click="showRegen = false">我已保存，关闭</button>
		</template>
	</BaseModal>
</template>
