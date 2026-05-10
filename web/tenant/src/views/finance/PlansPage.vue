<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'
import { extractApiError } from '@/utils/request'

const router = useRouter()
const loading = ref(false)
const plans = ref<any[]>([])
const currentPlan = ref<any>(null)
const showConfirm = ref(false)
const selectedPlan = ref<any>(null)
const selectedMonths = ref(1)
const confirmLoading = ref(false)

const monthsOptions = [
  { label: '1 个月', value: 1 },
  { label: '3 个月', value: 3 },
  { label: '6 个月', value: 6 },
  { label: '12 个月（优惠）', value: 12 },
]

const currentPlanId = computed(() => currentPlan.value?.plan_id)

async function fetchPlans() {
  loading.value = true
  try {
    const [plansRes, currentRes] = await Promise.all([
      request.get('/tenant/plans'),
      request.get('/tenant/plan/current').catch(() => ({ data: { data: null } })),
    ])
    const raw = plansRes.data?.data; plans.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
    currentPlan.value = currentRes.data?.data || null
  } catch (e) {
    // interceptor handles error toast
  } finally {
    loading.value = false
  }
}

function calcPrice(plan: any, months: number) {
  if (months >= 12) return Number(plan.yearly_price || 0)
  return Number(plan.monthly_price || 0) * months
}

function openConfirm(plan: any) {
  selectedPlan.value = plan
  selectedMonths.value = 1
  showConfirm.value = true
}

async function handleSubscribe() {
  confirmLoading.value = true
  try {
    const res = await request.post('/tenant/orders/create', {
      order_type: 'new_plan',
      plan_id: selectedPlan.value.id,
      months: selectedMonths.value,
    })
    const order = res.data?.data
    if (order?.id) {
      // Mock pay immediately
      await request.post(`/tenant/orders/${order.id}/pay`)
    }
    showConfirm.value = false
    await fetchPlans()
  } catch (e) {
    // interceptor handles error toast
  } finally {
    confirmLoading.value = false
  }
}

function isCurrentPlan(plan: any) {
  return currentPlan.value && currentPlan.value.plan_id === plan.id
}

onMounted(fetchPlans)
</script>

<template>
  <div>
    <!-- Page Header -->
    <div class="page-header">
      <h2 class="page-title">套餐方案</h2>
      <p class="page-description">选择适合您团队的方案，如需更多方案，可联系管理员定制</p>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      <div v-for="i in 3" :key="i" class="card p-6 animate-pulse">
        <div class="h-6 bg-gray-200 rounded w-1/2 mb-4"></div>
        <div class="h-8 bg-gray-200 rounded w-3/4 mb-6"></div>
        <div class="space-y-3">
          <div class="h-4 bg-gray-200 rounded"></div>
          <div class="h-4 bg-gray-200 rounded w-5/6"></div>
          <div class="h-4 bg-gray-200 rounded w-4/6"></div>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else-if="plans.length === 0" class="empty-state">
      <Icon name="exclamationCircle" size="xl" class="empty-state-icon text-gray-300" />
      <p class="empty-state-title">暂无可用套餐</p>
      <p class="empty-state-description">请联系管理员配置套餐方案</p>
    </div>

    <!-- Plan Cards -->
    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      <div
        v-for="plan in plans"
        :key="plan.id"
        class="card card-hover relative flex flex-col"
        :class="{ 'ring-2 ring-primary-500 shadow-glow': plan.is_recommended }"
      >
        <!-- Recommended Badge -->
        <div
          v-if="plan.is_recommended"
          class="absolute -top-3 left-1/2 -translate-x-1/2 badge badge-primary"
        >
          推荐
        </div>

        <!-- Current Plan Badge -->
        <div
          v-if="isCurrentPlan(plan)"
          class="absolute top-4 right-4 badge badge-success"
        >
          当前套餐
        </div>

        <div class="card-body flex-1 flex flex-col">
          <!-- Plan Name -->
          <h3 class="text-lg font-semibold text-gray-900 mb-1">{{ plan.name }}</h3>
          <p class="text-xs text-gray-500 mb-4">{{ plan.description || '' }}</p>

          <!-- Price -->
          <div class="mb-6">
            <div class="flex items-baseline gap-1">
              <span class="text-3xl font-bold text-gray-900">¥{{ Number(plan.monthly_price).toFixed(0) }}</span>
              <span class="text-sm text-gray-500">/月</span>
            </div>
            <p v-if="Number(plan.yearly_price) > 0" class="text-xs text-gray-400 mt-1">
              年付 ¥{{ Number(plan.yearly_price).toFixed(0) }}（省 ¥{{ (Number(plan.monthly_price) * 12 - Number(plan.yearly_price)).toFixed(0) }}）
            </p>
          </div>

          <!-- Features -->
          <div class="space-y-2.5 text-sm text-gray-600 flex-1">
            <div v-if="plan.monthly_quota_tokens > 0" class="flex items-center gap-2">
              <Icon name="check" size="sm" class="text-primary-500 flex-shrink-0" />
              <span>{{ plan.monthly_quota_tokens.toLocaleString() }} Tokens/月</span>
            </div>
            <div v-else class="flex items-center gap-2">
              <Icon name="check" size="sm" class="text-primary-500 flex-shrink-0" />
              <span>不限 Token 用量</span>
            </div>
          </div>
        </div>

        <!-- Action -->
        <div class="card-footer">
          <button
            v-if="isCurrentPlan(plan)"
            class="btn btn-secondary w-full"
            disabled
          >
            当前方案
          </button>
          <button
            v-else-if="Number(plan.monthly_price) === 0"
            class="btn btn-primary w-full"
            @click="openConfirm(plan)"
          >
            免费开通
          </button>
          <button
            v-else
            class="btn btn-primary w-full"
            @click="openConfirm(plan)"
          >
            立即购买
          </button>
        </div>
      </div>
    </div>

    <!-- Confirm Modal -->
    <Teleport to="body">
      <transition name="modal">
        <div v-if="showConfirm" class="modal-overlay" @click.self="showConfirm = false">
          <div class="modal-content w-full max-w-md">
            <div class="modal-header">
              <h3 class="modal-title">确认订阅</h3>
              <button @click="showConfirm = false" class="btn-ghost btn-icon">
                <Icon name="x" size="md" />
              </button>
            </div>
            <div class="modal-body">
              <div v-if="selectedPlan" class="space-y-4">
                <div class="flex items-center justify-between p-3 bg-gray-50 rounded-xl">
                  <span class="font-medium text-gray-900">{{ selectedPlan.name }}</span>
                  <span class="badge badge-primary">{{ selectedPlan.identifier }}</span>
                </div>

                <div>
                  <label class="input-label">订阅时长</label>
                  <div class="grid grid-cols-2 gap-2 mt-1.5">
                    <button
                      v-for="opt in monthsOptions"
                      :key="opt.value"
                      class="rounded-xl px-3 py-2 text-sm font-medium border transition-all"
                      :class="selectedMonths === opt.value
                        ? 'border-primary-500 bg-primary-50 text-primary-700'
                        : 'border-gray-200 text-gray-600 hover:border-gray-300'"
                      @click="selectedMonths = opt.value"
                    >
                      {{ opt.label }}
                    </button>
                  </div>
                </div>

                <div class="flex items-center justify-between p-3 bg-primary-50 rounded-xl">
                  <span class="text-sm text-gray-600">应付金额</span>
                  <span class="text-xl font-bold text-primary-600">
                    ¥{{ calcPrice(selectedPlan, selectedMonths).toFixed(2) }}
                  </span>
                </div>
              </div>
            </div>
            <div class="modal-footer">
              <button @click="showConfirm = false" class="btn btn-secondary">取消</button>
              <button
                class="btn btn-primary"
                :disabled="confirmLoading"
                @click="handleSubscribe"
              >
                {{ confirmLoading ? '处理中...' : '确认订阅' }}
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>
  </div>
</template>
