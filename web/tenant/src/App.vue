<template>
	<n-config-provider :theme-overrides="themeOverrides" :locale="zhCN" :date-locale="dateZhCN">
		<n-message-provider>
			<n-dialog-provider>
				<n-modal-provider>
					<TopProgress />
					<router-view />
				</n-modal-provider>
			</n-dialog-provider>
		</n-message-provider>
	</n-config-provider>
</template>

<script setup lang="ts">
import { zhCN, dateZhCN } from 'naive-ui'
import TopProgress from '@/components/common/TopProgress.vue'
import { themeOverrides } from '@/utils/naiveTheme'
</script>
