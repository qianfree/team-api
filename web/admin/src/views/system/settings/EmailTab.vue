<script setup lang="ts">
import SettingsSection from './SettingsSection.vue'
import { useFormValues } from './useSettings'
const values = useFormValues()
</script>

<template>
	<div class="tab-content">
		<!-- SMTP 连接 -->
		<SettingsSection title="SMTP 连接" size="half">
			<div class="section-grid">
				<AFormItem label="SMTP 服务器">
					<AInput v-model="values['email_smtp_host']" placeholder="smtp.example.com" />
				</AFormItem>
				<AFormItem label="SMTP 端口">
					<AInputNumber
						:model-value="values['email_smtp_port'] as number"
						@change="(v: number | undefined) => values['email_smtp_port'] = v"
						:min="1" :max="65535"
						placeholder="587"
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
		</SettingsSection>

		<!-- 发件设置 -->
		<SettingsSection title="发件设置" size="half">
			<div class="section-grid">
				<AFormItem label="发件人地址" help="格式: noreply@example.com">
					<AInput v-model="values['email_smtp_from']" placeholder="noreply@example.com" />
				</AFormItem>
				<AFormItem label="发件人名称" help="收件人看到的发件人显示名，留空则使用站点名称">
					<AInput v-model="values['email_smtp_from_name']" placeholder="team-api" />
				</AFormItem>
			</div>
			<div class="switch-list">
				<div class="switch-item">
					<div class="switch-item__text">
						<div class="switch-item__label">启用 TLS</div>
						<div class="switch-item__desc">587/465 端口建议开启</div>
					</div>
					<ASwitch
						:model-value="!!values['email_smtp_tls']"
						@change="(v: string | number | boolean) => values['email_smtp_tls'] = v"
					/>
				</div>
			</div>
		</SettingsSection>
	</div>
</template>

<style scoped>
@import './common.css';
</style>
