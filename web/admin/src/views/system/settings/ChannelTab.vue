<script setup lang="ts">
import { useFormValues } from './useSettings'
const values = useFormValues()
</script>

<template>
	<div class="tab-content">
		<!-- 自动探测 -->
		<div class="section">
			<div class="section-title">自动探测</div>
			<div class="switch-row">
				<span class="switch-label">渠道自动探测</span>
				<span class="switch-desc">定期向活跃渠道发送测试请求，检测连通性并更新健康度（会消耗少量 Token）。禁用渠道不自动探测，由管理员手动测试确认后再启用</span>
				<ASwitch
					:model-value="!!values['channel_auto_test_enabled']"
					@change="(v: string | number | boolean) => values['channel_auto_test_enabled'] = v"
				/>
			</div>
		</div>

		<!-- 自动禁用 -->
		<div class="section">
			<div class="section-title">自动禁用</div>
			<div class="switch-row">
				<span class="switch-label">渠道自动禁用</span>
				<span class="switch-desc">渠道熔断持续超过路由策略 breaker.autoDisableAfterSeconds（默认 10 分钟）未恢复时自动禁用</span>
				<ASwitch
					:model-value="!!values['channel_auto_disable_enabled']"
					@change="(v: string | number | boolean) => values['channel_auto_disable_enabled'] = v"
				/>
			</div>
			<div class="section-grid" style="margin-top: 12px">
				<AFormItem label="健康快照保留天数">
					<AInputNumber
						:model-value="values['health_snapshot_retention_days'] as number"
						@change="(v: number | undefined) => values['health_snapshot_retention_days'] = v ?? 7"
						:min="1" :max="90" style="width: 100%"
					/>
				</AFormItem>
			</div>
		</div>

		<!-- 路由策略 -->
		<div class="section">
			<div class="section-title">路由策略</div>
			<AFormItem label="路由策略覆盖（JSON）">
				<ATextarea
					:model-value="values['channel_routing_policy'] as string"
					@input="(v: string) => values['channel_routing_policy'] = v"
					:auto-size="{ minRows: 4, maxRows: 12 }"
					placeholder='{"tierFactors":{"secondary":0.3},"breaker":{"failThreshold":5}}'
					allow-clear
				/>
			</AFormItem>
			<div class="section-desc">
				渠道调度引擎策略，部分字段覆盖内置默认值，保存后 30 秒内热生效；留空使用全部默认。
				保存时会校验 JSON 合法性与取值范围。字段说明见 docs/reference/渠道调度运维手册.md。
			</div>
		</div>

		<!-- 代理设置 -->
		<div class="section">
			<div class="section-title">代理设置</div>
			<AFormItem label="代理地址">
				<AInput
					:model-value="values['channel_proxy_url'] as string"
					@input="(v: string) => values['channel_proxy_url'] = v"
					placeholder="http://127.0.0.1:7890"
					allow-clear
				/>
			</AFormItem>
			<div class="section-desc">
				全局代理 URL，支持 http:// 和 socks5:// 协议。在渠道编辑中开启"使用代理"即可生效。
			</div>
		</div>

		<!-- 同步图片异步化 -->
		<div class="section">
			<div class="section-title">同步图片异步化</div>
			<div class="switch-row">
				<span class="switch-label">同步图片厂商异步化</span>
				<span class="switch-desc">同步阻塞返回的图片厂商（OpenAI/DALL·E 等）走 /v1/images/generations/async 时改由后台 worker 池异步处理，客户端提交即拿 task_id 后轮询取图；关闭则该端点对同步厂商返回不支持</span>
				<ASwitch
					:model-value="!!values['sync_image_async_enabled']"
					@change="(v: string | number | boolean) => values['sync_image_async_enabled'] = v"
				/>
			</div>
			<div class="switch-row" style="margin-top: 12px">
				<span class="switch-label">图片 URL 转存对象存储</span>
				<span class="switch-desc">上游返回图片 URL 时下载并转存对象存储（返回 24h 稳定链接，需已配置存储）；关闭则直接透传上游 URL（部分厂商约 1h 过期）。b64_json 始终转存</span>
				<ASwitch
					:model-value="!!values['sync_image_rehost_url']"
					@change="(v: string | number | boolean) => values['sync_image_rehost_url'] = v"
				/>
			</div>
		</div>

		<!-- 协议转换选项 -->
		<div class="section">
			<div class="section-title">协议转换选项</div>
			<div class="switch-row">
				<span class="switch-label">Claude thinking 后缀适配</span>
				<span class="switch-desc">OpenAI 入站 × Claude 上游把模型名 -thinking/effort 后缀转换为扩展思考请求（注入 thinking 配置）</span>
				<ASwitch
					:model-value="!!values['relay_claude_thinking_adapter_enabled']"
					@change="(v: string | number | boolean) => values['relay_claude_thinking_adapter_enabled'] = v"
				/>
			</div>
			<div class="section-grid" style="margin-top: 12px">
				<AFormItem label="Claude 思考预算比例">
					<AInputNumber
						:model-value="values['relay_claude_thinking_budget_percentage'] as number"
						@change="(v: number | undefined) => values['relay_claude_thinking_budget_percentage'] = v ?? 0.5"
						:min="0.1" :max="0.9" :step="0.1" style="width: 100%"
					/>
				</AFormItem>
			</div>
			<div class="switch-row" style="margin-top: 12px">
				<span class="switch-label">Gemini thinking 后缀适配</span>
				<span class="switch-desc">OpenAI 入站 × Gemini 上游把模型名 -thinking/-nothinking/effort 后缀映射到 thinkingConfig</span>
				<ASwitch
					:model-value="!!values['relay_gemini_thinking_adapter_enabled']"
					@change="(v: string | number | boolean) => values['relay_gemini_thinking_adapter_enabled'] = v"
				/>
			</div>
			<div class="section-grid" style="margin-top: 12px">
				<AFormItem label="Gemini 思考预算比例">
					<AInputNumber
						:model-value="values['relay_gemini_thinking_budget_percentage'] as number"
						@change="(v: number | undefined) => values['relay_gemini_thinking_budget_percentage'] = v ?? 0.5"
						:min="0.1" :max="0.9" :step="0.1" style="width: 100%"
					/>
				</AFormItem>
			</div>
			<div class="switch-row" style="margin-top: 12px">
				<span class="switch-label">Gemini thoughtSignature 透传</span>
				<span class="switch-desc">Gemini 上游的 function-call parts 附带 thoughtSignature 绕过值（多轮工具调用校验所需）</span>
				<ASwitch
					:model-value="!!values['relay_gemini_thought_signature_enabled']"
					@change="(v: string | number | boolean) => values['relay_gemini_thought_signature_enabled'] = v"
				/>
			</div>
			<AFormItem label="Gemini 安全阈值（JSON）" style="margin-top: 12px">
				<ATextarea
					:model-value="values['relay_gemini_safety_setting'] as string"
					@input="(v: string) => values['relay_gemini_safety_setting'] = v"
					:auto-size="{ minRows: 2, maxRows: 8 }"
					placeholder='{"HARM_CATEGORY_HARASSMENT":"BLOCK_ONLY_HIGH"}'
					allow-clear
				/>
			</AFormItem>
			<div class="section-desc">类别 → 伤害阈值映射，转换后的 Gemini 请求按此附带 safetySettings；留空不附带。</div>
			<AFormItem label="保留 thinking 后缀的模型" style="margin-top: 12px">
				<AInput
					:model-value="values['relay_preserve_thinking_suffix_models'] as string"
					@input="(v: string) => values['relay_preserve_thinking_suffix_models'] = v"
					placeholder="gemini-2.5-pro, gpt-4*, claude-3-5-sonnet"
					allow-clear
				/>
			</AFormItem>
			<div class="section-desc">逗号分隔的模型名列表，列表内模型（支持尾部 * 前缀匹配）在发往上游的模型名上保留 -thinking/-nothinking/effort 后缀；留空对所有模型剥离后缀。</div>
		</div>
	</div>
</template>

<style scoped>
@import './common.css';
.section-desc {
	color: var(--color-text-3);
	font-size: 13px;
	line-height: 1.6;
	margin-top: -8px;
}
</style>
