<script setup lang="ts">
import { ref, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'
import { useTenantOptions } from '@/composables/useTenantOptions'

// 发起按需分析弹窗（ai-req-analyze:analyze 权限入口）。
// 租户远程搜索；目标成员按租户远程搜索，无 member:view 权限（接口 403）时
// 自动降级为手输成员 ID；时间范围后端校验（单次 ≤31 天、起止有序）。

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void
  (e: 'submitted'): void
}>()

const { tenantOptions, fetchTenantOptions, handleTenantSearch } = useTenantOptions()

const submitting = ref(false)
const form = ref({
  tenant_id: undefined as number | undefined,
  user_id: undefined as number | undefined,
  range: [] as string[],
})

// 目标成员下拉：选租户后远程搜索该租户成员；无权限时降级为手输用户 ID
const memberOptions = ref<{ label: string; value: number }[]>([])
const memberSearchFailed = ref(false)
let memberTimer: ReturnType<typeof setTimeout> | null = null

watch(
  () => props.visible,
  (visible) => {
    if (visible) fetchTenantOptions()
  },
)

async function fetchMembers(keyword = '') {
  if (!form.value.tenant_id) return
  try {
    const res: any = await request.get('/admin/members', {
      params: { page: 1, page_size: 50, tenant_id: form.value.tenant_id, keyword },
    })
    const list = res.data?.data?.list || []
    memberOptions.value = list.map((m: any) => ({
      label: `${m.username}${m.display_name && m.display_name !== m.username ? `（${m.display_name}）` : ''}`,
      value: m.id,
    }))
    memberSearchFailed.value = false
  } catch {
    memberSearchFailed.value = true
    memberOptions.value = []
  }
}

function handleMemberSearch(value: string) {
  if (memberTimer) clearTimeout(memberTimer)
  memberTimer = setTimeout(() => fetchMembers(value), 300)
}

async function submit() {
  if (!form.value.tenant_id) {
    Message.warning('请选择要分析的租户')
    return false
  }
  if (!form.value.range || form.value.range.length !== 2) {
    Message.warning('请选择分析时间范围')
    return false
  }
  submitting.value = true
  try {
    const payload: Record<string, any> = {
      tenant_id: form.value.tenant_id,
      time_start: form.value.range[0],
      time_end: form.value.range[1],
    }
    if (form.value.user_id) payload.user_id = form.value.user_id

    const res: any = await request.post('/admin/plugins/ai-req-analyze/analyze', payload)
    Message.success(`分析任务已创建（#${res.data?.data?.job_id}），可在任务列表查看进度`)
    emit('update:visible', false)
    emit('submitted')
    return true
  } catch {
    return false
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <AModal
    :visible="visible"
    title="发起按需分析"
    :mask-closable="false"
    :width="520"
    :ok-loading="submitting"
    :on-before-ok="submit"
    @update:visible="emit('update:visible', $event)"
    @cancel="emit('update:visible', false)"
  >
    <AForm :model="form" layout="vertical">
      <AFormItem label="目标租户" required>
        <ASelect
          v-model="form.tenant_id"
          :options="tenantOptions"
          placeholder="搜索并选择租户"
          allow-search allow-clear
          :filter-option="false"
          @search="handleTenantSearch"
          @change="() => { form.user_id = undefined; fetchMembers() }"
        />
      </AFormItem>
      <AFormItem label="目标成员（可选，不选 = 租户全员）">
        <ASelect
          v-if="!memberSearchFailed"
          v-model="form.user_id"
          :options="memberOptions"
          placeholder="搜索该租户的成员"
          allow-search allow-clear
          :filter-option="false"
          :disabled="!form.tenant_id"
          @search="handleMemberSearch"
        />
        <AInputNumber
          v-else
          v-model="form.user_id"
          placeholder="输入成员 ID（无成员查看权限时手动指定）"
          :min="1"
          style="width: 100%"
        />
      </AFormItem>
      <AFormItem label="分析时间范围" required extra="单次窗口最多 31 天；建议先小范围验证 AI 配置">
        <ARangePicker
          v-model="form.range"
          style="width: 100%"
          show-time
          value-format="YYYY-MM-DD HH:mm:ss"
        />
      </AFormItem>
    </AForm>
  </AModal>
</template>
