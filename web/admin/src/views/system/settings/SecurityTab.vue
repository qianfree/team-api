<script setup lang="ts">
import { ref, watch } from 'vue'
import { useFormValues } from './useSettings'
const values = useFormValues()

// 黑名单列表以标签形式编辑（一条规则一个标签），存库为 JSON 字符串数组。
// 标签数组与库值之间单向同步：输入只写回表单值，库值变化（加载/保存刷新）才反向同步。
const ipBlacklistTags = ref<string[]>([])

// 存库 JSON → 标签数组（脏格式容错为空）
function parseBlacklistTags(raw: unknown): string[] {
  try {
    const arr = JSON.parse(String(raw ?? '[]'))
    return Array.isArray(arr) ? arr.map(String) : []
  } catch {
    return []
  }
}

// 服务器加载/保存刷新时把库值同步进标签；与当前标签一致时跳过（自己写回的回环）
watch(() => values['ip_blacklist_list'], (raw) => {
  const tags = parseBlacklistTags(raw)
  if (JSON.stringify(tags) === JSON.stringify(ipBlacklistTags.value)) return
  ipBlacklistTags.value = tags
}, { immediate: true })

// 输入单向写回表单值
watch(ipBlacklistTags, (tags) => {
  values['ip_blacklist_list'] = JSON.stringify(tags)
})
</script>

<template>
	<div class="tab-content">
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
		<!-- 注册禁用词 -->
		<div class="section">
			<div class="section-title">注册禁用词</div>
			<div class="section-desc">组织名称、组织代码、用户名中包含这些词时禁止注册（不区分大小写），多个禁用词用英文逗号分隔</div>
			<div class="section-grid" style="grid-template-columns: 1fr">
				<AFormItem label="禁用词列表">
					<ATextarea
						:model-value="values['register_forbidden_words'] ?? ''"
						@update:model-value="(v: string) => values['register_forbidden_words'] = v"
						placeholder="admin,system,root,api,test,administrator,管理员,系统"
						:auto-size="{ minRows: 3, maxRows: 6 }"
					/>
				</AFormItem>
			</div>
		</div>

		<!-- IP 黑名单 -->
		<div class="section">
			<div class="section-title">IP 黑名单</div>
			<div class="section-desc">
				开启后，命中黑名单的 IP 访问任何端点（管理后台 / 租户控制台 / AI 代理）都会被直接拒绝，保存后即时生效。
				拦截次数统计在仪表盘展示（存于缓存，不落库）
			</div>
			<div class="section-grid">
				<AFormItem label="启用 IP 黑名单">
					<ASwitch
						:model-value="!!values['ip_blacklist_enabled']"
						@change="(v: string | number | boolean) => values['ip_blacklist_enabled'] = v"
					/>
				</AFormItem>
			</div>
			<div class="section-grid" style="grid-template-columns: 1fr">
				<AFormItem label="黑名单列表">
					<AInputTag
						v-model="ipBlacklistTags"
						class="w-full"
						placeholder="输入 IP 或网段后回车添加，如 1.2.3.4、10.0.0.0/8"
						unique-value
						allow-clear
					/>
					<template #extra>
						<span class="field-help">输入后回车添加为标签，点标签上的 × 删除；支持精确 IP（1.2.3.4）与 CIDR 网段（10.0.0.0/8），保存时校验格式</span>
					</template>
				</AFormItem>
			</div>
		</div>
	</div>
</template>

<style scoped>
@import "./common.css";
.section-desc {
	font-size: 12px;
	color: var(--color-text-3);
	margin-bottom: 12px;
}
.field-help {
	color: var(--color-text-3);
	font-size: 12px;
}
</style>
