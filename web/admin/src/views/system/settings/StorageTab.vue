<script setup lang="ts">
import { useFormValues } from './useSettings'
const values = useFormValues()
</script>

<template>
	<div class="tab-content">
		<!-- 连接配置 -->
		<div class="section">
			<div class="section-title">连接配置</div>
			<div class="section-grid">
				<AFormItem label="存储供应商">
					<ASelect v-model="values['storage_provider']">
						<AOption value="minio" label="MinIO" />
						<AOption value="s3" label="AWS S3" />
						<AOption value="oss" label="阿里云 OSS" />
						<AOption value="cos" label="腾讯云 COS" />
					</ASelect>
				</AFormItem>
				<AFormItem label="存储桶名称">
					<AInput v-model="values['storage_bucket']" placeholder="my-bucket" />
				</AFormItem>
				<AFormItem label="存储端点" class="field-full"
					help="S3/MinIO: https://s3.amazonaws.com, OSS: https://oss-cn-hangzhou.aliyuncs.com">
					<AInput v-model="values['storage_endpoint']" placeholder="https://s3.amazonaws.com" />
				</AFormItem>
				<AFormItem label="存储区域">
					<AInput v-model="values['storage_region']" placeholder="us-east-1" />
				</AFormItem>
				<AFormItem label="路径前缀" help="存储路径前缀，用于隔离不同环境">
					<AInput v-model="values['storage_path_prefix']" placeholder="team-api" />
				</AFormItem>
			</div>
		</div>

		<!-- 认证 -->
		<div class="section">
			<div class="section-title">认证</div>
			<div class="section-grid">
				<AFormItem label="Access Key ID">
					<AInputPassword v-model="values['storage_access_key_id']" placeholder="留空则不修改" />
				</AFormItem>
				<AFormItem label="Access Key Secret">
					<AInputPassword v-model="values['storage_access_key_secret']" placeholder="留空则不修改" />
				</AFormItem>
			</div>
			<div class="switch-row" style="margin-top: 8px">
				<span class="switch-label">启用 SSL</span>
				<ASwitch
					:model-value="values['storage_use_ssl'] === 'true' || values['storage_use_ssl'] === '1'"
					@change="(v: string | number | boolean) => values['storage_use_ssl'] = String(v)"
				/>
			</div>
		</div>
	</div>
</template>

<style scoped>
@import './common.css';
</style>
