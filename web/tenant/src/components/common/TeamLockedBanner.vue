<script setup lang="ts">
import Icon from '@/components/common/Icon.vue'
</script>

<template>
	<div class="card p-10 flex flex-col items-center justify-center text-center animate-slide-up">
		<div class="h-16 w-16 rounded-2xl bg-gray-100 flex items-center justify-center mb-4">
			<Icon name="lock" size="lg" class="text-gray-400" />
		</div>
		<h3 class="text-lg font-semibold text-gray-900 mb-2">团队功能未启用</h3>
		<p class="text-sm text-gray-500 max-w-sm mb-6 leading-relaxed">
			当前为个人模式。请先设置组织代码，即可启用成员管理、RAM 账号、邀请、成员额度等团队功能。
		</p>
		<router-link to="/tenant/organization" class="btn btn-primary">
			<Icon name="cog" size="sm" />
			前往设置组织代码
		</router-link>
	</div>
</template>
