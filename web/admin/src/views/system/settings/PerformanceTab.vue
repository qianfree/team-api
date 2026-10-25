<script setup lang="ts">
import SettingsSection from './SettingsSection.vue'
import { useFormValues } from './useSettings'
const values = useFormValues()

function num(key: string) {
	return Number(values[key]) || 0
}
function set(key: string, v: number | undefined): void {
	values[key] = v ?? 0
}
</script>

<template>
	<div class="perf-grid">
		<!-- QPS 限制 -->
		<SettingsSection title="QPS 限制">
			<div class="compact-row">
				<label>系统级</label>
				<AInputNumber :model-value="num('global_qps_limit')" @change="(v: number | undefined) => set('global_qps_limit', v)" :min="0" />
			</div>
			<div class="compact-row">
				<label>租户级</label>
				<AInputNumber :model-value="num('tenant_qps_limit')" @change="(v: number | undefined) => set('tenant_qps_limit', v)" :min="0" />
			</div>
			<div class="compact-row">
				<label>用户级</label>
				<AInputNumber :model-value="num('user_qps_limit')" @change="(v: number | undefined) => set('user_qps_limit', v)" :min="0" />
			</div>
			<div class="compact-row">
				<label>Key 级</label>
				<AInputNumber :model-value="num('key_qps_limit')" @change="(v: number | undefined) => set('key_qps_limit', v)" :min="0" />
			</div>
		</SettingsSection>

		<!-- 并发限制 -->
		<SettingsSection title="并发限制">
			<div class="compact-row">
				<label>系统级上限</label>
				<AInputNumber :model-value="num('global_concurrency_limit')" @change="(v: number | undefined) => set('global_concurrency_limit', v)" :min="0" />
			</div>
			<div class="compact-row">
				<label>租户级上限</label>
				<AInputNumber :model-value="num('tenant_concurrency_limit')" @change="(v: number | undefined) => set('tenant_concurrency_limit', v)" :min="0" />
			</div>
		</SettingsSection>

		<!-- 超时设置 -->
		<SettingsSection title="超时设置">
			<div class="compact-row">
				<label>请求超时(秒)</label>
				<AInputNumber :model-value="num('request_timeout_seconds')" @change="(v: number | undefined) => set('request_timeout_seconds', v)" :min="5" :max="600" />
			</div>
			<div class="compact-row">
				<label>流式空闲超时(秒)</label>
				<AInputNumber :model-value="num('streaming_timeout_seconds')" @change="(v: number | undefined) => set('streaming_timeout_seconds', v)" :min="30" :max="3600" />
			</div>
		</SettingsSection>

		<!-- 批量写入 -->
		<SettingsSection title="批量写入">
			<div class="compact-row">
				<label>写入大小</label>
				<AInputNumber :model-value="num('batch_write_size')" @change="(v: number | undefined) => set('batch_write_size', v)" :min="1" :max="1000" />
			</div>
			<div class="compact-row">
				<label>写入间隔(ms)</label>
				<AInputNumber :model-value="num('batch_write_interval_ms')" @change="(v: number | undefined) => set('batch_write_interval_ms', v)" :min="100" :max="30000" />
			</div>
		</SettingsSection>

		<!-- 服务器资源保护 -->
		<SettingsSection title="服务器资源保护">
			<div class="compact-row">
				<label>内存占用上限(%)</label>
				<AInputNumber :model-value="num('resource_memory_limit_percent')" @change="(v: number | undefined) => set('resource_memory_limit_percent', v)" :min="0" :max="100" />
			</div>
			<div class="compact-row">
				<label>磁盘占用上限(%)</label>
				<AInputNumber :model-value="num('resource_disk_limit_percent')" @change="(v: number | undefined) => set('resource_disk_limit_percent', v)" :min="0" :max="100" />
			</div>
		</SettingsSection>

		<!-- 调试 -->
		<SettingsSection title="调试">
			<div class="compact-row">
				<label>pprof 端口</label>
				<AInputNumber :model-value="num('pprof_port')" @change="(v: number | undefined) => set('pprof_port', v)" :min="0" :max="65535" />
			</div>
		</SettingsSection>
	</div>
</template>

<style scoped>
.perf-grid {
	display: grid;
	grid-template-columns: repeat(3, 1fr);
	gap: 16px;
	align-items: start;
}
/* 紧凑数字行卡片：减小内边距与标题间距，保持信息密度 */
.perf-grid :deep(.settings-section) {
	padding: 16px 20px 12px;
}
.perf-grid :deep(.settings-section__header) {
	margin-bottom: 8px;
}
.perf-grid :deep(.settings-section__body) {
	gap: 2px;
}
.compact-row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 12px;
	padding: 5px 0;
}
.compact-row label {
	font-size: 13px;
	color: var(--ta-text-primary);
	white-space: nowrap;
	flex-shrink: 0;
}
.compact-row :deep(.arco-input-number) {
	width: 160px;
}

/* 宽屏三列、中屏两列、窄屏单列；行内输入框在窄屏收窄并允许换行，避免横向溢出 */
@media (max-width: 1280px) {
	.perf-grid {
		grid-template-columns: repeat(2, 1fr);
	}
}
@media (max-width: 1024px) {
	.perf-grid {
		grid-template-columns: 1fr;
		gap: 12px;
	}
	.compact-row {
		flex-wrap: wrap;
	}
	.compact-row :deep(.arco-input-number) {
		width: 140px;
		max-width: 60%;
	}
}
</style>
