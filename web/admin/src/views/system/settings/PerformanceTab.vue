<script setup lang="ts">
import { useFormValues } from './useSettings'
const values = useFormValues()

function num(key: string) {
	return Number(values[key]) || 0
}
function set(key: string, v: number | undefined): void {
	values[key] = String(v ?? 0)
}
</script>

<template>
	<div class="perf-grid">
		<!-- QPS 限制 -->
		<div class="group">
			<div class="group-title">QPS 限制</div>
			<div class="compact-row">
				<label>系统级</label>
				<AInputNumber :model-value="num('global_qps_limit')" @change="(v: number | undefined) => set('global_qps_limit', v)" :min="100" :max="1000000" />
			</div>
			<div class="compact-row">
				<label>租户级</label>
				<AInputNumber :model-value="num('tenant_qps_limit')" @change="(v: number | undefined) => set('tenant_qps_limit', v)" :min="10" :max="100000" />
			</div>
			<div class="compact-row">
				<label>用户级</label>
				<AInputNumber :model-value="num('user_qps_limit')" @change="(v: number | undefined) => set('user_qps_limit', v)" :min="1" :max="10000" />
			</div>
			<div class="compact-row">
				<label>Key 级</label>
				<AInputNumber :model-value="num('key_qps_limit')" @change="(v: number | undefined) => set('key_qps_limit', v)" :min="1" :max="10000" />
			</div>
		</div>

		<!-- 并发限制 -->
		<div class="group">
			<div class="group-title">并发限制</div>
			<div class="compact-row">
				<label>系统级上限</label>
				<AInputNumber :model-value="num('global_concurrency_limit')" @change="(v: number | undefined) => set('global_concurrency_limit', v)" :min="10" :max="100000" />
			</div>
			<div class="compact-row">
				<label>租户级上限</label>
				<AInputNumber :model-value="num('tenant_concurrency_limit')" @change="(v: number | undefined) => set('tenant_concurrency_limit', v)" :min="1" :max="10000" />
			</div>
		</div>

		<!-- 超时设置 -->
		<div class="group">
			<div class="group-title">超时设置</div>
			<div class="compact-row">
				<label>请求超时(秒)</label>
				<AInputNumber :model-value="num('request_timeout_seconds')" @change="(v: number | undefined) => set('request_timeout_seconds', v)" :min="5" :max="600" />
			</div>
			<div class="compact-row">
				<label>流式超时(秒)</label>
				<AInputNumber :model-value="num('streaming_timeout_seconds')" @change="(v: number | undefined) => set('streaming_timeout_seconds', v)" :min="30" :max="3600" />
			</div>
		</div>

		<!-- 缓存设置 -->
		<div class="group">
			<div class="group-title">缓存设置</div>
			<div class="compact-row">
				<label>启用缓存</label>
				<ASwitch
					:model-value="values['cache_enabled'] === 'true' || values['cache_enabled'] === '1'"
					@change="(v: string | number | boolean) => values['cache_enabled'] = String(v)"
				/>
			</div>
			<div class="compact-row">
				<label>过期时间(秒)</label>
				<AInputNumber :model-value="num('cache_ttl_seconds')" @change="(v: number | undefined) => set('cache_ttl_seconds', v)" :min="30" :max="86400" />
			</div>
		</div>

		<!-- 批量写入 -->
		<div class="group">
			<div class="group-title">批量写入</div>
			<div class="compact-row">
				<label>写入大小</label>
				<AInputNumber :model-value="num('batch_write_size')" @change="(v: number | undefined) => set('batch_write_size', v)" :min="1" :max="1000" />
			</div>
			<div class="compact-row">
				<label>写入间隔(ms)</label>
				<AInputNumber :model-value="num('batch_write_interval_ms')" @change="(v: number | undefined) => set('batch_write_interval_ms', v)" :min="100" :max="30000" />
			</div>
		</div>

		<!-- 渠道调度 -->
		<div class="group">
			<div class="group-title">渠道调度</div>
			<div class="compact-row">
				<label>亲和性 TTL(秒)</label>
				<AInputNumber :model-value="num('channel_affinity_ttl')" @change="(v: number | undefined) => set('channel_affinity_ttl', v)" :min="0" :max="3600" />
			</div>
			<div class="compact-row">
				<label>测试间隔(秒)</label>
				<AInputNumber :model-value="num('auto_test_interval')" @change="(v: number | undefined) => set('auto_test_interval', v)" :min="60" :max="86400" />
			</div>
		</div>

		<!-- 沙箱模式 -->
		<div class="group">
			<div class="group-title">沙箱模式</div>
			<div class="compact-row">
				<label>启用沙箱</label>
				<ASwitch
					:model-value="values['sandbox_enabled'] === 'true' || values['sandbox_enabled'] === '1'"
					@change="(v: string | number | boolean) => values['sandbox_enabled'] = String(v)"
				/>
			</div>
			<div class="compact-row">
				<label>月默认额度</label>
				<AInputNumber :model-value="num('sandbox_default_quota')" @change="(v: number | undefined) => set('sandbox_default_quota', v)" :min="0" :max="100000" />
			</div>
		</div>

		<!-- 调试 -->
		<div class="group">
			<div class="group-title">调试</div>
			<div class="compact-row">
				<label>pprof 端口</label>
				<AInputNumber :model-value="num('pprof_port')" @change="(v: number | undefined) => set('pprof_port', v)" :min="0" :max="65535" />
			</div>
		</div>
	</div>
</template>

<style scoped>
.perf-grid {
	display: grid;
	grid-template-columns: repeat(3, 1fr);
	gap: 20px;
}
.group {
	border: 1px solid var(--color-fill-2);
	border-radius: 6px;
	padding: 14px 16px 10px;
}
.group-title {
	font-size: 13px;
	font-weight: 600;
	color: var(--color-text-2);
	margin-bottom: 10px;
}
.compact-row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 12px;
	padding: 4px 0;
}
.compact-row label {
	font-size: 13px;
	color: var(--color-text-1);
	white-space: nowrap;
	flex-shrink: 0;
}
.compact-row :deep(.arco-input-number) {
	width: 160px;
}
</style>
