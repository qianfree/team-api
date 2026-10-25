<script setup lang="ts">
import SettingsSection from './SettingsSection.vue'
import { useFormValues } from './useSettings'
const values = useFormValues()
</script>

<template>
	<div class="tab-content">
		<!-- 通用设置 -->
		<SettingsSection title="通用设置">
			<div class="switch-list">
				<div class="switch-item">
					<div class="switch-item__text">
						<div class="switch-item__label">OAuth 自动注册</div>
						<div class="switch-item__desc">首次 OAuth 登录自动创建租户账号</div>
					</div>
					<ASwitch
						:model-value="!!values['oauth_auto_register']"
						@change="(v: string | number | boolean) => values['oauth_auto_register'] = v"
					/>
				</div>
			</div>
			<div class="section-grid">
				<AFormItem label="OAuth 默认租户代码" help="OAuth 自动注册时使用的租户代码（需预先创建）" class="field-full">
					<AInput v-model="values['oauth_tenant_code']" placeholder="填写已有的租户代码" />
				</AFormItem>
			</div>
		</SettingsSection>

		<!-- GitHub -->
		<SettingsSection title="GitHub" size="half">
			<div class="switch-list">
				<div class="switch-item">
					<div class="switch-item__text">
						<div class="switch-item__label">启用 GitHub 登录</div>
					</div>
					<ASwitch
						:model-value="!!values['oauth_github_enabled']"
						@change="(v: string | number | boolean) => values['oauth_github_enabled'] = v"
					/>
				</div>
			</div>
			<div class="section-grid">
				<AFormItem label="Client ID">
					<AInputPassword v-model="values['oauth_github_client_id']" placeholder="留空则不修改" />
				</AFormItem>
				<AFormItem label="Client Secret">
					<AInputPassword v-model="values['oauth_github_client_secret']" placeholder="留空则不修改" />
				</AFormItem>
			</div>
		</SettingsSection>

		<!-- Google -->
		<SettingsSection title="Google" size="half">
			<div class="switch-list">
				<div class="switch-item">
					<div class="switch-item__text">
						<div class="switch-item__label">启用 Google 登录</div>
					</div>
					<ASwitch
						:model-value="!!values['oauth_google_enabled']"
						@change="(v: string | number | boolean) => values['oauth_google_enabled'] = v"
					/>
				</div>
			</div>
			<div class="section-grid">
				<AFormItem label="Client ID">
					<AInputPassword v-model="values['oauth_google_client_id']" placeholder="留空则不修改" />
				</AFormItem>
				<AFormItem label="Client Secret">
					<AInputPassword v-model="values['oauth_google_client_secret']" placeholder="留空则不修改" />
				</AFormItem>
			</div>
		</SettingsSection>
	</div>
</template>

<style scoped>
@import './common.css';
</style>
