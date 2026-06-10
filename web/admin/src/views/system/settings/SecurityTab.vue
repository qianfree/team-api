<script setup lang="ts">
import { useFormValues } from './useSettings'
const values = useFormValues()
</script>

<template>
	<div class="tab-content">
		<!-- 会话管理 -->
		<div class="section">
			<div class="section-title">会话管理</div>
			<div class="section-grid">
				<AFormItem label="租户用户最大会话数">
					<AInputNumber
						:model-value="values['max_sessions_per_user'] as number"
						@change="(v: number | undefined) => values['max_sessions_per_user'] = v ?? 10"
						:min="1" :max="100" style="width: 100%"
					/>
				</AFormItem>
				<AFormItem label="管理员最大会话数">
					<AInputNumber
						:model-value="values['admin_max_sessions'] as number"
						@change="(v: number | undefined) => values['admin_max_sessions'] = v ?? 5"
						:min="1" :max="50" style="width: 100%"
					/>
				</AFormItem>
			</div>
		</div>

		<!-- 登录安全 -->
		<div class="section">
			<div class="section-title">登录安全</div>
			<div class="section-grid">
				<AFormItem label="登录最大尝试次数">
					<AInputNumber
						:model-value="values['login_max_attempts'] as number"
						@change="(v: number | undefined) => values['login_max_attempts'] = v ?? 5"
						:min="1" :max="30" style="width: 100%"
					/>
				</AFormItem>
				<AFormItem label="登录锁定时长(分钟)">
					<AInputNumber
						:model-value="values['login_lockout_minutes'] as number"
						@change="(v: number | undefined) => values['login_lockout_minutes'] = v ?? 30"
						:min="1" :max="1440" style="width: 100%"
					/>
				</AFormItem>
			</div>
		</div>

		<!-- 密码策略 -->
		<div class="section">
			<div class="section-title">密码策略</div>
			<div class="section-grid">
				<AFormItem label="密码最小长度">
					<AInputNumber
						:model-value="values['password_min_length'] as number"
						@change="(v: number | undefined) => values['password_min_length'] = v ?? 8"
						:min="6" :max="32" style="width: 100%"
					/>
				</AFormItem>
			</div>
		</div>

		<!-- 验证码有效期 -->
		<div class="section">
			<div class="section-title">滑块验证码</div>
			<div class="section-desc">登录、注册、重置密码时必须完成滑块验证</div>
			<div class="section-grid">
				<AFormItem label="验证码有效期(秒)">
					<AInputNumber
						:model-value="values['captcha_expire_seconds'] as number"
						@change="(v: number | undefined) => values['captcha_expire_seconds'] = v ?? 300"
						:min="60" :max="600" style="width: 100%"
					/>
				</AFormItem>
			</div>
		</div>

		<!-- Turnstile 人机验证 -->
		<div class="section">
			<div class="section-title">Turnstile 人机验证</div>
			<div class="section-desc">集成 Cloudflare Turnstile，防止自动化攻击和暴力破解</div>
			<div class="section-grid">
				<AFormItem label="启用 Turnstile">
					<ASwitch
						:model-value="!!values['turnstile_enabled']"
						@change="(v: string | number | boolean) => values['turnstile_enabled'] = v"
					/>
				</AFormItem>
				<AFormItem label="新设备登录通知">
					<ASwitch
						:model-value="!!values['new_device_notification']"
						@change="(v: string | number | boolean) => values['new_device_notification'] = v"
					/>
				</AFormItem>
			</div>
			<div class="section-grid">
				<AFormItem label="Site Key">
					<AInput
						:model-value="values['turnstile_site_key'] ?? ''"
						@update:model-value="(v: string) => values['turnstile_site_key'] = v"
						placeholder="Cloudflare Turnstile Site Key"
					/>
				</AFormItem>
				<AFormItem label="Secret Key">
					<AInputPassword
						:model-value="values['turnstile_secret_key'] ?? ''"
						@update:model-value="(v: string) => values['turnstile_secret_key'] = v"
						placeholder="Cloudflare Turnstile Secret Key"
					/>
				</AFormItem>
			</div>
		</div>
	</div>
</template>

<style scoped>
@import './common.css';
.section-desc {
	font-size: 12px;
	color: var(--color-text-3);
	margin-bottom: 12px;
}
</style>
