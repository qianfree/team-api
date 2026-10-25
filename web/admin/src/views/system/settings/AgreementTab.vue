<script setup lang="ts">
import SettingsSection from './SettingsSection.vue'
import { useFormValues } from './useSettings'
const values = useFormValues()
</script>

<template>
	<div class="tab-content">
		<SettingsSection title="用户协议">
			<div class="switch-list">
				<div class="switch-item">
					<div class="switch-item__text">
						<div class="switch-item__label">启用用户协议</div>
						<div class="switch-item__desc">启用后，管理员可在「运营管理 → 用户协议」页面管理协议版本，用户登录时需接受最新协议</div>
					</div>
					<ASwitch
						:model-value="!!values['agreement_enabled']"
						@change="(v: string | number | boolean) => values['agreement_enabled'] = v"
					/>
				</div>
			</div>
		</SettingsSection>
	</div>
</template>

<style scoped>
@import './common.css';
</style>
