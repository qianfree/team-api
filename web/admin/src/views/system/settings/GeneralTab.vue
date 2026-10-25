<script setup lang="ts">
import { computed } from 'vue'
import SettingsSection from './SettingsSection.vue'
import { useFormValues } from './useSettings'
const values = useFormValues()

// 维护时长必须是 Go duration 格式（如 2h、30m、1h30m）：
// API 维护中间件按此格式解析 Retry-After，输入中文（如「2小时」）会静默回退默认 300s
const DURATION_PATTERN = /^(\d+(\.\d+)?(ms|s|m|h))+$/
const durationInvalid = computed(() => {
	const v = (values['maintenance_duration'] ?? '').toString().trim()
	return v !== '' && !DURATION_PATTERN.test(v)
})
const durationHelp = computed(() =>
	durationInvalid.value ? '维护时长格式不正确，示例：2h、30m、1h30m' : 'Go 时长格式，如 2h、30m、1h30m'
)

// 关闭维护模式时自动清空提示信息和时长
function handleMaintenanceModeChange(enabled: boolean) {
	values['maintenance_mode'] = enabled
	if (!enabled) {
		values['maintenance_message'] = ''
		values['maintenance_duration'] = ''
	}
}
</script>

<template>
	<div class="tab-content">
		<!-- 基本信息 -->
		<SettingsSection title="基本信息" desc="站点对外展示与回调地址等基础参数">
			<div class="section-grid">
				<AFormItem label="站点名称">
					<AInput v-model="values['site_name']" placeholder="Team-API" />
				</AFormItem>
				<AFormItem label="计费时区" help="模型时段定价（峰谷/促销）按此时区解释，IANA 时区名">
					<AInput v-model="values['pricing_time_timezone']" placeholder="Asia/Shanghai" />
				</AFormItem>
				<AFormItem label="站点描述" class="field-full">
					<AInput v-model="values['site_description']" placeholder="简短描述平台用途" />
				</AFormItem>
				<AFormItem label="租户控制台地址" help="用于生成邀请链接等，如 https://console.example.com" class="field-full">
					<AInput v-model="values['tenant_console_url']" placeholder="https://console.example.com" />
				</AFormItem>
				<AFormItem label="站点后端地址" help="后端 API 对外地址，构造 OAuth 登录回调用，须与 GitHub/Google 应用后台登记的回调域名一致；为空时按当前请求 Host 推断" class="field-full">
					<AInput v-model="values['site_url']" placeholder="https://api.example.com" />
				</AFormItem>
			</div>
		</SettingsSection>

		<!-- 注册设置 -->
		<SettingsSection title="注册设置" size="half">
			<div class="switch-list">
				<div class="switch-item">
					<div class="switch-item__text">
						<div class="switch-item__label">开放注册</div>
						<div class="switch-item__desc">是否允许新用户注册</div>
					</div>
					<ASwitch
						:model-value="!!values['register_enabled']"
						@change="(v: string | number | boolean) => values['register_enabled'] = v"
					/>
				</div>
				<div class="switch-item">
					<div class="switch-item__text">
						<div class="switch-item__label">注册邮箱验证</div>
						<div class="switch-item__desc">注册时是否需要邮箱验证码，关闭时使用滑块验证</div>
					</div>
					<ASwitch
						:model-value="!!values['register_email_verification']"
						@change="(v: string | number | boolean) => values['register_email_verification'] = v"
					/>
				</div>
			</div>
			<div class="grid-3">
				<AFormItem label="IP每小时注册限制" help="同一IP每小时最多注册次数（0表示不限制）">
					<AInputNumber
						:model-value="values['register_ip_limit_per_hour'] as number"
						@change="(v: number | undefined) => values['register_ip_limit_per_hour'] = v ?? 50"
						:min="0" :max="200" style="width: 100%"
					/>
				</AFormItem>
				<AFormItem label="IP每天注册限制" help="同一IP每天最多注册次数（0表示不限制）">
					<AInputNumber
						:model-value="values['register_ip_limit_per_day'] as number"
						@change="(v: number | undefined) => values['register_ip_limit_per_day'] = v ?? 500"
						:min="0" :max="1000" style="width: 100%"
					/>
				</AFormItem>
				<AFormItem label="全局每分钟注册限制" help="全系统每分钟最多注册次数（0表示不限制）">
					<AInputNumber
						:model-value="values['register_global_limit_per_minute'] as number"
						@change="(v: number | undefined) => values['register_global_limit_per_minute'] = v ?? 50"
						:min="0" :max="200" style="width: 100%"
					/>
				</AFormItem>
			</div>
		</SettingsSection>

		<!-- 维护模式 -->
		<SettingsSection title="维护模式" size="half">
			<div class="switch-list">
				<div class="switch-item">
					<div class="switch-item__text">
						<div class="switch-item__label">启用维护模式</div>
						<div class="switch-item__desc">开启后控制台显示维护提示</div>
					</div>
					<ASwitch
						:model-value="!!values['maintenance_mode']"
						@change="handleMaintenanceModeChange"
					/>
				</div>
				<div class="switch-item">
					<div class="switch-item__text">
						<div class="switch-item__label">全局 API 维护</div>
						<div class="switch-item__desc">开启后 API 代理返回 503</div>
					</div>
					<ASwitch
						:model-value="!!values['api_maintenance_enabled']"
						@change="(v: string | number | boolean) => values['api_maintenance_enabled'] = v"
					/>
				</div>
			</div>
			<div class="section-grid">
				<AFormItem label="维护提示信息">
					<AInput v-model="values['maintenance_message']" placeholder="系统维护中，请稍后访问" />
				</AFormItem>
				<AFormItem label="预计维护时长" :validate-status="durationInvalid ? 'error' : undefined" :help="durationHelp">
					<AInput v-model="values['maintenance_duration']" placeholder="如：2h、30m、1h30m" allow-clear />
				</AFormItem>
			</div>
		</SettingsSection>
	</div>
</template>

<style scoped>
@import './common.css';
</style>
