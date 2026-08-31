<script setup lang="ts">
import { ref, reactive, watch, onMounted } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'
import { useIsMobile } from '@/composables/useIsMobile'
import { formatBilling } from '@/composables/useCurrency'

const props = defineProps<{
  tenantId: string
  detail: any | null
}>()

const emit = defineEmits<{
  'refresh-detail': []
}>()

const isMobile = useIsMobile()

const statusTagColor: Record<string, string> = {
  active: 'green',
  suspended: 'orangered',
  closed: undefined,
}
const statusTagLabel: Record<string, string> = {
  active: '正常',
  suspended: '暂停',
  closed: '已关闭',
}

const editLoading = ref(false)
const editForm = reactive({
  name: '',
  level: null as number | null,
  max_members: null as number | null,
  max_concurrency: null as number | null,
})

// 等级配置选项（后端接口）
const levelOptions = ref<any[]>([])

async function fetchLevelOptions() {
  try {
    const res: any = await request.get('/admin/tenant-level-configs')
    levelOptions.value = res.data?.data?.list || res.data?.list || []
  } catch {
  }
}

// 生效的最大成员数（自定义值优先，回退等级配置）
function getEffectiveMaxMembers() {
  if (props.detail?.max_members != null) return props.detail.max_members
  if (props.detail?.level_config?.max_members != null) return props.detail.level_config.max_members
  const cfg = levelOptions.value.find((l: any) => l.level === props.detail?.level)
  return cfg?.max_members ?? null
}

// 生效的并发上限（自定义值优先，回退等级配置）
function getEffectiveMaxConcurrency() {
  if (props.detail?.max_concurrency != null) return props.detail.max_concurrency
  if (props.detail?.level_config?.max_concurrency != null) return props.detail.level_config.max_concurrency
  const cfg = levelOptions.value.find((l: any) => l.level === props.detail?.level)
  return cfg?.max_concurrency ?? null
}

// detail 就绪或刷新后回填编辑表单（原版 onMounted 未 await fetchDetail 就回填导致表单永远为空）
watch(() => props.detail, (d) => {
  if (!d) return
  editForm.name = d.name || ''
  editForm.level = d.level ?? null
  editForm.max_members = d.max_members ?? null
  editForm.max_concurrency = d.max_concurrency ?? null
}, { immediate: true })

async function saveEdit() {
  editLoading.value = true
  try {
    await request.put(`/admin/tenants/${props.tenantId}`, editForm)
    Message.success('更新成功')
    emit('refresh-detail')
  } catch {
  } finally {
    editLoading.value = false
  }
}

onMounted(fetchLevelOptions)
</script>

<template>
  <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
    <ACard :bordered="false" title="租户信息">
      <ADescriptions :column="isMobile ? 1 : 2" bordered size="medium">
        <ADescriptionsItem label="ID">{{ detail.id }}</ADescriptionsItem>
        <ADescriptionsItem label="租户名称">{{ detail.name }}</ADescriptionsItem>
        <ADescriptionsItem label="租户代码">
          <ATag size="small">{{ detail.code }}</ATag>
        </ADescriptionsItem>
        <ADescriptionsItem label="状态">
          <ATag :color="statusTagColor[detail.status]" size="small">
            {{ statusTagLabel[detail.status] || detail.status }}
          </ATag>
        </ADescriptionsItem>
        <ADescriptionsItem label="所有者">{{ detail.owner_name || '-' }}</ADescriptionsItem>
        <ADescriptionsItem label="成员数">
          {{ detail.member_count || 0 }} / {{ getEffectiveMaxMembers() ?? '不限' }}
          <ATag v-if="detail.max_members != null" size="small" color="arcoblue" style="margin-left: 4px">自定义</ATag>
        </ADescriptionsItem>
        <ADescriptionsItem label="钱包余额">
          <span class="money">{{ formatBilling(detail.wallet_balance, 2) }}</span>
        </ADescriptionsItem>
        <ADescriptionsItem label="并发上限">
          {{ getEffectiveMaxConcurrency() ?? '不限' }}
          <ATag v-if="detail.max_concurrency != null" size="small" color="arcoblue" style="margin-left: 4px">自定义</ATag>
        </ADescriptionsItem>
        <ADescriptionsItem label="等级">
          <template v-if="detail.level_config">
            {{ detail.level_config.name }}
            <ATag v-if="detail.level_config.price_multiplier" size="small" color="green" style="margin-left: 4px">
              {{ (detail.level_config.price_multiplier * 100).toFixed(0) }}%
            </ATag>
          </template>
          <template v-else-if="detail.level">
            Lv.{{ detail.level }}
          </template>
          <template v-else>-</template>
        </ADescriptionsItem>
        <ADescriptionsItem label="创建时间">{{ detail.created_at }}</ADescriptionsItem>
        <ADescriptionsItem label="更新时间">{{ detail.updated_at }}</ADescriptionsItem>
      </ADescriptions>
    </ACard>

    <ACard :bordered="false" title="编辑租户">
      <AForm :model="editForm" :auto-label-width="true" layout="vertical">
        <AFormItem label="租户名称">
          <AInput v-model="editForm.name" class="w-full" />
        </AFormItem>
        <AFormItem label="最大成员数">
          <AInputNumber v-model="editForm.max_members" :min="1" allow-clear placeholder="跟随等级" class="w-full" />
        </AFormItem>
        <AFormItem label="并发上限">
          <AInputNumber v-model="editForm.max_concurrency" :min="0" allow-clear placeholder="跟随等级" class="w-full" />
        </AFormItem>
        <AFormItem label="等级">
          <ASelect v-model="editForm.level" allow-clear placeholder="选择等级" class="w-full">
            <AOption v-for="opt in levelOptions" :key="opt.level" :value="opt.level">
              {{ opt.name }}（{{ (opt.price_multiplier * 100).toFixed(0) }}%）
            </AOption>
          </ASelect>
        </AFormItem>
        <AFormItem>
          <AButton type="primary" :loading="editLoading" @click="saveEdit">保存</AButton>
        </AFormItem>
      </AForm>
    </ACard>
  </div>
</template>

<style scoped>
@import './common.css';
</style>
