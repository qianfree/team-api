<script setup lang="ts">
import { useFormValues } from './useSettings'
const values = useFormValues()
</script>

<template>
	<div class="tab-content">
		<!-- SMTP 连接 -->
		<div class="section">
			<div class="section-title">SMTP 连接</div>
			<div class="section-grid">
				<AFormItem label="SMTP 服务器">
					<AInput v-model="values['email_smtp_host']" placeholder="smtp.example.com" />
				</AFormItem>
				<AFormItem label="SMTP 端口">
					<AInputNumber
						:model-value="values['email_smtp_port'] as number"
						@change="(v: number | undefined) => values['email_smtp_port'] = String(v ?? 587)"
						:min="1" :max="65535"
						style="width: 100%"
					/>
				</AFormItem>
				<AFormItem label="SMTP 用户名">
					<AInput v-model="values['email_smtp_username']" placeholder="user@example.com" />
				</AFormItem>
				<AFormItem label="SMTP 密码">
					<AInputPassword v-model="values['email_smtp_password']" placeholder="留空则不修改" />
				</AFormItem>
			</div>
		</div>

		<!-- 发件设置 -->
		<div class="section">
			<div class="section-title">发件设置</div>
			<div class="section-grid">
				<AFormItem label="发件人地址" help="格式: noreply@example.com">
					<AInput v-model="values['email_smtp_from']" placeholder="noreply@example.com" />
				</AFormItem>
				<div class="switch-row">
					<span class="switch-label">启用 TLS</span>
					<span class="switch-desc">587/465 端口建议开启</span>
					<ASwitch
						:model-value="!!values['email_smtp_tls']"
						@change="(v: string | number | boolean) => values['email_smtp_tls'] = v"
					/>
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped>
@import './common.css';
</style>
