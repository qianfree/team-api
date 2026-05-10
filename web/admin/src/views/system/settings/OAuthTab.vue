<script setup lang="ts">
const values = defineModel<Record<string, any>>({ required: true })
</script>

<template>
	<div class="tab-content">
		<!-- 通用设置 -->
		<div class="section">
			<div class="section-title">通用设置</div>
			<div class="switch-row">
				<span class="switch-label">OAuth 自动注册</span>
				<span class="switch-desc">首次 OAuth 登录自动创建租户账号</span>
				<ASwitch
					:model-value="values['oauth_auto_register'] === 'true' || values['oauth_auto_register'] === '1'"
					@change="(v: string | number | boolean) => values['oauth_auto_register'] = String(v)"
				/>
			</div>
			<div class="section-grid" style="margin-top: 12px">
				<AFormItem label="OAuth 默认租户代码" help="OAuth 自动注册时使用的租户代码（需预先创建）" class="field-full">
					<AInput v-model="values['oauth_tenant_code']" placeholder="填写已有的租户代码" />
				</AFormItem>
			</div>
		</div>

		<!-- GitHub -->
		<div class="section">
			<div class="section-title">GitHub</div>
			<div class="switch-row">
				<span class="switch-label">启用 GitHub 登录</span>
				<span class="switch-desc"></span>
				<ASwitch
					:model-value="values['oauth_github_enabled'] === 'true' || values['oauth_github_enabled'] === '1'"
					@change="(v: string | number | boolean) => values['oauth_github_enabled'] = String(v)"
				/>
			</div>
			<div class="section-grid" style="margin-top: 12px">
				<AFormItem label="Client ID">
					<AInputPassword v-model="values['oauth_github_client_id']" placeholder="留空则不修改" />
				</AFormItem>
				<AFormItem label="Client Secret">
					<AInputPassword v-model="values['oauth_github_client_secret']" placeholder="留空则不修改" />
				</AFormItem>
			</div>
		</div>

		<!-- Google -->
		<div class="section">
			<div class="section-title">Google</div>
			<div class="switch-row">
				<span class="switch-label">启用 Google 登录</span>
				<span class="switch-desc"></span>
				<ASwitch
					:model-value="values['oauth_google_enabled'] === 'true' || values['oauth_google_enabled'] === '1'"
					@change="(v: string | number | boolean) => values['oauth_google_enabled'] = String(v)"
				/>
			</div>
			<div class="section-grid" style="margin-top: 12px">
				<AFormItem label="Client ID">
					<AInputPassword v-model="values['oauth_google_client_id']" placeholder="留空则不修改" />
				</AFormItem>
				<AFormItem label="Client Secret">
					<AInputPassword v-model="values['oauth_google_client_secret']" placeholder="留空则不修改" />
				</AFormItem>
			</div>
		</div>
	</div>
</template>

<style scoped>
@import './common.css';
</style>
