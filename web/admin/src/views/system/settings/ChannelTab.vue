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
				<span class="switch-desc">定期向活跃渠道发送测试请求，检测连通性并更新健康度（会消耗少量 Token）</span>
				<ASwitch
					:model-value="!!values['channel_auto_test_enabled']"
					@change="(v: string | number | boolean) => values['channel_auto_test_enabled'] = v"
				/>
			</div>
			<div class="switch-row" style="margin-top: 12px">
				<span class="switch-label">自动恢复探测</span>
				<span class="switch-desc">定期测试已自动禁用的渠道，测试通过则恢复启用（依赖自动探测开启）</span>
				<ASwitch
					:model-value="!!values['channel_auto_test_recovery_enabled']"
					@change="(v: string | number | boolean) => values['channel_auto_test_recovery_enabled'] = v"
				/>
			</div>
		</div>

		<!-- 自动禁用 -->
		<div class="section">
			<div class="section-title">自动禁用</div>
			<div class="switch-row">
				<span class="switch-label">渠道自动禁用</span>
				<span class="switch-desc">连续失败达到阈值时自动禁用渠道</span>
				<ASwitch
					:model-value="!!values['channel_auto_disable_enabled']"
					@change="(v: string | number | boolean) => values['channel_auto_disable_enabled'] = v"
				/>
			</div>
			<div class="section-grid" style="margin-top: 12px">
				<AFormItem label="自动禁用失败阈值">
					<AInputNumber
						:model-value="values['channel_auto_disable_threshold'] as number"
						@change="(v: number | undefined) => values['channel_auto_disable_threshold'] = v ?? 5"
						:min="2" :max="50" style="width: 100%"
					/>
				</AFormItem>
				<AFormItem label="健康快照保留天数">
					<AInputNumber
						:model-value="values['health_snapshot_retention_days'] as number"
						@change="(v: number | undefined) => values['health_snapshot_retention_days'] = v ?? 7"
						:min="1" :max="90" style="width: 100%"
					/>
				</AFormItem>
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
