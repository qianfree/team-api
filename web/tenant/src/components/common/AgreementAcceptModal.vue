<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { marked } from 'marked'
import type { PendingAgreement } from '@/stores/tenant-auth'
import request from '@/utils/request'
import Icon from '@/components/common/Icon.vue'

const props = defineProps<{
	show: boolean
	agreements: PendingAgreement[]
}>()

const emit = defineEmits<{
	accepted: []
}>()

const selectedAgreement = ref<PendingAgreement | null>(null)
const agreementContent = ref('')
const contentLoading = ref(false)
const acceptLoading = ref(false)
const allChecked = ref(false)

const contentHtml = computed(() => {
	if (!agreementContent.value) return ''
	return marked(agreementContent.value) as string
})

watch(() => props.show, (val) => {
	if (val && props.agreements.length > 0) {
		selectedAgreement.value = props.agreements[0]
		loadContent(props.agreements[0])
		allChecked.value = false
	}
})

async function loadContent(item: PendingAgreement) {
	selectedAgreement.value = item
	agreementContent.value = ''
	contentLoading.value = true
	try {
		const res = await request.get(`/tenant/agreements/current/${item.code}`)
		agreementContent.value = res.data?.data?.content || ''
	} catch {
		agreementContent.value = '加载失败，请重试'
	} finally {
		contentLoading.value = false
	}
}

async function handleAccept() {
	if (!allChecked.value) return
	acceptLoading.value = true
	try {
		await request.post('/tenant/agreements/accept', {
			agreement_ids: props.agreements.map(a => a.id),
		})
		emit('accepted')
	} catch {
		// error auto-displayed by interceptor
	} finally {
		acceptLoading.value = false
	}
}
</script>

<template>
	<Teleport to="body">
		<Transition name="modal">
			<div v-if="show" class="modal-overlay">
				<div class="modal-content" style="max-width: 640px">
					<!-- Header -->
					<div class="modal-header">
						<div class="flex items-center gap-2">
							<div class="flex h-8 w-8 items-center justify-center rounded-lg bg-amber-100">
								<Icon name="document" size="md" class="text-amber-600" />
							</div>
							<h3 class="modal-title">请阅读并同意以下协议</h3>
						</div>
					</div>

					<!-- Body -->
					<div class="modal-body" style="padding: 0;">
						<!-- Tabs -->
						<div v-if="agreements.length > 1" class="flex gap-1 px-4 pt-4 sm:px-6 sm:pt-4">
							<button
								v-for="item in agreements"
								:key="item.id"
								type="button"
								class="rounded-lg px-3 py-1.5 text-sm font-medium transition-all"
								:class="selectedAgreement?.id === item.id
									? 'bg-primary-500 text-white'
									: 'text-gray-500 hover:bg-gray-100 hover:text-gray-700'"
								@click="loadContent(item)"
							>
								{{ item.title }}
							</button>
						</div>

						<!-- Content -->
						<div class="px-4 py-4 sm:px-6">
							<div class="agreement-scroll rounded-xl border border-gray-200 bg-gray-50 p-5">
								<div v-if="contentLoading" class="flex items-center justify-center py-12">
									<div class="spinner h-5 w-5 border-primary-500"></div>
								</div>
								<div v-else class="markdown-content" v-html="contentHtml" />
							</div>
						</div>
					</div>

					<!-- Footer -->
					<div class="modal-footer">
						<label class="flex items-center gap-2 cursor-pointer select-none flex-1">
							<input
								v-model="allChecked"
								type="checkbox"
								class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500/30 transition-colors cursor-pointer"
							/>
							<span class="text-sm text-gray-600">
								我已阅读并同意以上{{ agreements.length > 1 ? '所有' : '' }}协议
							</span>
						</label>
						<button
							type="button"
							class="btn btn-primary"
							:disabled="!allChecked || acceptLoading"
							@click="handleAccept"
						>
							<div v-if="acceptLoading" class="spinner h-4 w-4 border-white"></div>
							{{ acceptLoading ? '提交中...' : '同意并继续' }}
						</button>
					</div>
				</div>
			</div>
		</Transition>
	</Teleport>
</template>

<style scoped>
.agreement-scroll {
	max-height: 380px;
	overflow-y: auto;
	line-height: 1.8;
	font-size: 14px;
	color: #334155;
}

.agreement-scroll :deep(h1),
.agreement-scroll :deep(h2),
.agreement-scroll :deep(h3) {
	margin-top: 16px;
	margin-bottom: 8px;
	color: #0f172a;
	font-weight: 600;
}

.agreement-scroll :deep(h1) { font-size: 20px; }
.agreement-scroll :deep(h2) { font-size: 17px; }
.agreement-scroll :deep(h3) { font-size: 15px; }

.agreement-scroll :deep(ul),
.agreement-scroll :deep(ol) {
	padding-left: 20px;
}

.agreement-scroll :deep(p) {
	margin-bottom: 8px;
}
</style>
