<script setup lang="ts">
import { computed, watch } from 'vue'
import { IconCheck } from '@arco-design/web-vue/es/icon'
import SettingsSection from './SettingsSection.vue'
import { useFormValues } from './useSettings'
const values = useFormValues()

// Cloudflare R2 不分区，但 AWS SDK 签名要求非空 region，选中 R2 时自动填 auto
watch(
	() => values['storage_provider'],
	(provider) => {
		if (provider === 'r2' && !values['storage_region']) {
			values['storage_region'] = 'auto'
		}
	}
)

// /v1/files 允许文件类型快捷标签：点选把条目并入输入框，再点移出。
// 取值与后端 matchRelayFileType 条目形态对齐（MIME 前缀 / 扩展名）。
const typePresets = [
	{ label: '常用图片', entries: ['image/*'] },
	{ label: '视频', entries: ['video/*'] },
	{ label: '音频', entries: ['audio/*'] },
	{ label: '文档', entries: ['pdf', 'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx', 'txt', 'csv', 'md'] },
	{ label: '压缩包', entries: ['zip', 'rar', '7z', 'tar', 'gz'] },
]

function parsedTypes(): string[] {
	return (values['relay_files_allowed_types'] ?? '')
		.toString()
		.split(',')
		.map((s: string) => s.trim().toLowerCase())
		.filter(Boolean)
}

const presetStates = computed(() =>
	typePresets.map((p) => ({ ...p, active: parsedTypes().every((e) => p.entries.includes(e)) }))
)

function togglePreset(entries: string[]): void {
	const cur = parsedTypes()
	const active = entries.every((e) => cur.includes(e))
	const next = active
		? cur.filter((e) => !entries.includes(e))
		: [...cur, ...entries.filter((e) => !cur.includes(e))]
	values['relay_files_allowed_types'] = next.join(',')
}
</script>

<template>
	<div class="tab-content">
		<!-- 连接配置 -->
		<SettingsSection title="连接配置">
			<div class="section-grid">
				<AFormItem label="存储供应商">
					<ASelect v-model="values['storage_provider']">
						<AOption value="minio" label="MinIO" />
						<AOption value="s3" label="AWS S3" />
						<AOption value="r2" label="Cloudflare R2" />
						<AOption value="oss" label="阿里云 OSS" />
						<AOption value="cos" label="腾讯云 COS" />
					</ASelect>
				</AFormItem>
				<AFormItem label="存储桶名称">
					<AInput v-model="values['storage_bucket']" placeholder="my-bucket" />
				</AFormItem>
				<AFormItem label="存储端点" class="field-full"
					help="S3/MinIO: https://s3.amazonaws.com, OSS: https://oss-cn-hangzhou.aliyuncs.com, R2: https://<account_id>.r2.cloudflarestorage.com">
					<AInput v-model="values['storage_endpoint']" placeholder="https://s3.amazonaws.com" />
				</AFormItem>
				<AFormItem label="存储区域" help="R2 固定填 auto">
					<AInput v-model="values['storage_region']" placeholder="us-east-1" />
				</AFormItem>
				<AFormItem label="路径前缀" help="存储路径前缀，用于隔离不同环境">
					<AInput v-model="values['storage_path_prefix']" placeholder="team-api" />
				</AFormItem>
			</div>
		</SettingsSection>

		<!-- 认证 -->
		<SettingsSection title="认证">
			<div class="section-grid">
				<AFormItem label="Access Key ID">
					<AInputPassword v-model="values['storage_access_key_id']" placeholder="留空则不修改" />
				</AFormItem>
				<AFormItem label="Access Key Secret">
					<AInputPassword v-model="values['storage_access_key_secret']" placeholder="留空则不修改" />
				</AFormItem>
			</div>
			<div class="switch-list">
				<div class="switch-item">
					<div class="switch-item__text">
						<div class="switch-item__label">启用 SSL</div>
					</div>
					<ASwitch
						:model-value="!!values['storage_use_ssl']"
						@change="(v: string | number | boolean) => values['storage_use_ssl'] = v"
					/>
				</div>
			</div>
		</SettingsSection>

		<!-- /v1/files 素材上传 -->
		<SettingsSection title="素材上传（/v1/files）">
			<div class="section-grid">
				<AFormItem label="允许的文件类型" class="field-full"
					help="逗号分隔；支持 image/*（MIME 前缀）、application/pdf（完整 MIME）、.txt（扩展名）；留空 = 不限制">
					<div class="types-field">
						<AInput v-model="values['relay_files_allowed_types']" placeholder="image/*" allow-clear />
						<div class="type-presets">
							<span class="type-presets-label">快捷填入</span>
							<button v-for="p in presetStates" :key="p.label" type="button" class="preset-chip"
								:class="{ active: p.active }" @click="togglePreset(p.entries)">
								<IconCheck v-if="p.active" :size="12" />
								<span>{{ p.label }}</span>
							</button>
						</div>
					</div>
				</AFormItem>
			</div>
			<div class="grid-3">
				<AFormItem label="单文件大小上限(MB)" help="实际生效上限为该值与全局请求体上限的较小者">
					<AInputNumber :model-value="Number(values['relay_files_max_size_mb']) || 0"
						@change="(v: number | undefined) => values['relay_files_max_size_mb'] = v ?? 0"
						:min="1" :max="2048" style="width: 100%" />
				</AFormItem>
				<AFormItem label="每租户上传上限(MB)" help="0 = 不限制">
					<AInputNumber :model-value="Number(values['relay_files_max_tenant_mb']) || 0"
						@change="(v: number | undefined) => values['relay_files_max_tenant_mb'] = v ?? 0"
						:min="0" :max="1048576" style="width: 100%" />
				</AFormItem>
				<AFormItem label="签名URL有效期(分钟)" help="上传/查询返回的预签名 URL 有效期">
					<AInputNumber :model-value="Number(values['relay_files_url_ttl_minutes']) || 0"
						@change="(v: number | undefined) => values['relay_files_url_ttl_minutes'] = v ?? 0"
						:min="5" :max="10080" style="width: 100%" />
				</AFormItem>
			</div>
		</SettingsSection>
	</div>
</template>

<style scoped>
@import './common.css';

/* 输入框独占一行，快捷标签作为独立子行铺开，避免与输入框横向争位 */
.types-field {
	display: flex;
	flex-direction: column;
	gap: 8px;
	width: 100%;
}

.type-presets {
	display: flex;
	align-items: center;
	flex-wrap: wrap;
	gap: 8px;
}

.type-presets-label {
	font-size: 12px;
	color: var(--color-text-3);
	margin-right: 4px;
}

.preset-chip {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	padding: 2px 12px;
	border-radius: 999px;
	font-size: 12px;
	line-height: 20px;
	color: var(--color-text-2);
	background: var(--color-fill-2);
	border: 1px solid var(--color-border-2);
	cursor: pointer;
	transition: color 0.2s ease, background-color 0.2s ease, border-color 0.2s ease;
}

.preset-chip:hover {
	color: rgb(var(--primary-6));
	border-color: rgb(var(--primary-6));
	background: var(--color-fill-1);
}

.preset-chip.active {
	color: #fff;
	background: rgb(var(--primary-6));
	border-color: rgb(var(--primary-6));
}

.preset-chip.active:hover {
	background: rgb(var(--primary-5));
	border-color: rgb(var(--primary-5));
}
</style>
