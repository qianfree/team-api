<script setup lang="ts">
import { useFormValues } from './useSettings'
const values = useFormValues()
</script>

<template>
	<div class="tab-content">
		<!-- 基本信息 -->
		<div class="section">
			<div class="section-title">基本信息</div>
			<div class="section-grid">
				<AFormItem label="站点名称">
					<AInput v-model="values['site_name']" placeholder="Team-API" />
				</AFormItem>
				<AFormItem label="站点描述">
					<AInput v-model="values['site_description']" placeholder="简短描述平台用途" />
				</AFormItem>
				<AFormItem label="租户控制台地址" help="用于生成邀请链接等，如 https://console.example.com" class="field-full">
					<AInput v-model="values['tenant_console_url']" placeholder="https://console.example.com" />
				</AFormItem>
			</div>
		</div>

		<!-- 注册设置 -->
		<div class="section">
			<div class="section-title">注册设置</div>
			<div class="switch-row">
				<span class="switch-label">开放注册</span>
				<span class="switch-desc">是否允许新用户注册</span>
				<ASwitch
					:model-value="!!values['register_enabled']"
					@change="(v: string | number | boolean) => values['register_enabled'] = v"
				/>
			</div>
			<div class="switch-row">
				<span class="switch-label">注册邮箱验证</span>
				<span class="switch-desc">注册时是否需要邮箱验证码，关闭时使用滑块验证</span>
				<ASwitch
					:model-value="!!values['register_email_verification']"
					@change="(v: string | number | boolean) => values['register_email_verification'] = v"
				/>
			</div>
		</div>

		<!-- 维护模式 -->
		<div class="section">
			<div class="section-title">维护模式</div>
			<div class="switch-row">
				<span class="switch-label">启用维护模式</span>
				<span class="switch-desc">开启后控制台显示维护提示</span>
				<ASwitch
					:model-value="!!values['maintenance_mode']"
					@change="(v: string | number | boolean) => values['maintenance_mode'] = v"
				/>
			</div>
			<div class="section-grid" style="margin-top: 12px">
				<AFormItem label="维护提示信息">
					<AInput v-model="values['maintenance_message']" placeholder="系统维护中，请稍后访问" />
				</AFormItem>
				<AFormItem label="预计维护时长">
					<AInput v-model="values['maintenance_duration']" placeholder="如：2小时" />
				</AFormItem>
			</div>
			<div class="switch-row" style="margin-top: 8px">
				<span class="switch-label">全局 API 维护</span>
				<span class="switch-desc">开启后 API 代理返回 503</span>
				<ASwitch
					:model-value="!!values['api_maintenance_enabled']"
					@change="(v: string | number | boolean) => values['api_maintenance_enabled'] = v"
				/>
			</div>
		</div>
	</div>
</template>

<style scoped>
@import './common.css';
</style>
