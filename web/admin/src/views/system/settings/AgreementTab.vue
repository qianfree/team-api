<script setup lang="ts">
import { useFormValues } from './useSettings'
const values = useFormValues()
</script>

<template>
	<div class="tab-content">
		<div class="section">
			<div class="section-title">用户协议</div>
			<div class="switch-row">
				<span class="switch-label">启用用户协议</span>
				<ASwitch
					:model-value="!!values['agreement_enabled']"
					@change="(v: string | number | boolean) => values['agreement_enabled'] = v"
				/>
			</div>
			<div class="section-desc">启用后，管理员可在「运营管理 → 用户协议」页面管理协议版本，用户登录时需接受最新协议</div>
		</div>
	</div>
</template>

<style scoped>
@import './common.css';
</style>
