<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'
import { toast } from '@/utils/toast'

interface ChannelPrefs {
  email: boolean
  in_app: boolean
}

interface CategoryPrefs {
  [key: string]: ChannelPrefs
}

const loading = ref(false)
const saving = ref(false)
const activeTab = ref<'user' | 'org'>('user')

const orgPrefs = reactive<CategoryPrefs>({})
const userPrefs = reactive<CategoryPrefs>({})

const categoryMeta: Record<string, { label: string; icon: string; description: string; lockable?: boolean }> = {
  security: {
    label: '安全通知',
    icon: 'shield',
    description: '登录异常、密码修改、API Key 变更等',
    lockable: true,
  },
  billing: {
    label: '账单通知',
    icon: 'creditCard',
    description: '扣费提醒、余额不足、充值成功等',
  },
  system: {
    label: '系统通知',
    icon: 'cog',
    description: '系统维护、版本更新、服务状态变更等',
  },
  member: {
    label: '成员通知',
    icon: 'users',
    description: '新成员加入、角色变更、成员移除等',
  },
  project: {
    label: '项目通知',
    icon: 'grid',
    description: '项目创建、配置变更、额度预警等',
  },
}

function defaultPrefs(): CategoryPrefs {
  return {
    security: { email: true, in_app: true },
    billing: { email: true, in_app: true },
    system: { email: false, in_app: true },
    member: { email: true, in_app: true },
    project: { email: true, in_app: true },
  }
}

function currentPrefs(): CategoryPrefs {
  return activeTab.value === 'org' ? orgPrefs : userPrefs
}

function getChannel(category: string, channel: 'email' | 'in_app'): boolean {
  const prefs = currentPrefs()
  return prefs[category]?.[channel] ?? true
}

function toggleChannel(category: string, channel: 'email' | 'in_app') {
  const prefs = currentPrefs()
  if (!prefs[category]) {
    prefs[category] = { email: true, in_app: true }
  }

  const newVal = !prefs[category][channel]

  // Security: cannot disable the last remaining channel
  if (categoryMeta[category]?.lockable && !newVal) {
    const other = channel === 'email' ? 'in_app' : 'email'
    if (!prefs[category][other]) return
  }

  prefs[category][channel] = newVal
}

async function fetchPreferences() {
  loading.value = true
  try {
    const res: any = await request.get('/tenant/notification-preferences')
    const data = res.data?.data || {}

    // Parse org preferences
    const org = data.org_preferences || {}
    Object.keys(org).forEach((k) => {
      const ch = org[k] as any
      orgPrefs[k] = { email: !!ch.email, in_app: !!ch.in_app }
    })
    // Fill missing categories with defaults
    const defs = defaultPrefs()
    Object.keys(defs).forEach((k) => {
      if (!orgPrefs[k]) orgPrefs[k] = { ...defs[k] }
    })

    // Parse user preferences
    const user = data.user_preferences || {}
    Object.keys(user).forEach((k) => {
      const ch = user[k] as any
      userPrefs[k] = { email: !!ch.email, in_app: !!ch.in_app }
    })
    Object.keys(defs).forEach((k) => {
      if (!userPrefs[k]) userPrefs[k] = { ...defs[k] }
    })
  } catch {
    Object.assign(orgPrefs, defaultPrefs())
    Object.assign(userPrefs, defaultPrefs())
  } finally {
    loading.value = false
  }
}

async function savePreferences() {
  saving.value = true
  try {
    const prefs = { ...currentPrefs() }
    await request.put('/tenant/notification-preferences', {
      scope: activeTab.value,
      preferences: prefs,
    })
    toast.success('保存成功')
  } catch {
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  fetchPreferences()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Page Header -->
    <div class="page-header flex items-center justify-between">
      <div>
        <h1 class="page-title">通知偏好</h1>
        <p class="page-description">管理您接收通知的方式和类型</p>
      </div>
      <button class="btn btn-primary" :disabled="saving" @click="savePreferences">
        <div v-if="saving" class="spinner h-4 w-4"></div>
        <Icon v-else name="check" size="sm" />
        {{ saving ? '保存中...' : '保存设置' }}
      </button>
    </div>

    <!-- Tabs -->
    <div class="tabs">
      <button class="tab" :class="{ 'tab-active': activeTab === 'user' }" @click="activeTab = 'user'">
        个人偏好
      </button>
      <button class="tab" :class="{ 'tab-active': activeTab === 'org' }" @click="activeTab = 'org'">
        组织偏好
      </button>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="card p-8 text-center">
      <div class="spinner mx-auto mb-3"></div>
      <p class="text-sm text-gray-500">加载中...</p>
    </div>

    <template v-else>
      <!-- Category List -->
      <div class="space-y-3">
        <div v-for="(meta, key) in categoryMeta" :key="key" class="card">
          <div class="px-6 py-5">
            <div class="flex items-center justify-between">
              <!-- Left: icon + info -->
              <div class="flex items-start gap-4">
                <div
                  class="h-10 w-10 rounded-xl flex items-center justify-center flex-shrink-0"
                  :class="{
                    'bg-red-100 text-red-600': key === 'security',
                    'bg-amber-100 text-amber-600': key === 'billing',
                    'bg-gray-100 text-gray-600': key === 'system',
                    'bg-emerald-100 text-emerald-600': key === 'member',
                    'bg-primary-100 text-primary-600': key === 'project',
                  }"
                >
                  <Icon :name="meta.icon" size="md" />
                </div>
                <div>
                  <div class="flex items-center gap-2">
                    <h3 class="text-sm font-semibold text-gray-900">{{ meta.label }}</h3>
                    <span v-if="meta.lockable" class="badge badge-danger text-[10px]">不可完全关闭</span>
                  </div>
                  <p class="text-xs text-gray-500 mt-0.5">{{ meta.description }}</p>
                </div>
              </div>

              <!-- Right: toggles -->
              <div class="flex items-center gap-6 flex-shrink-0">
                <!-- Email -->
                <label class="flex items-center gap-2 cursor-pointer select-none">
                  <span class="text-xs text-gray-500">邮件</span>
                  <button
                    class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
                    :class="getChannel(key, 'email') ? 'bg-primary-500' : 'bg-gray-200'"
                    @click="toggleChannel(key, 'email')"
                  >
                    <span
                      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                      :class="getChannel(key, 'email') ? 'translate-x-5' : 'translate-x-0'"
                    />
                  </button>
                </label>

                <!-- In-App -->
                <label class="flex items-center gap-2 cursor-pointer select-none">
                  <span class="text-xs text-gray-500">站内信</span>
                  <button
                    class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
                    :class="getChannel(key, 'in_app') ? 'bg-primary-500' : 'bg-gray-200'"
                    @click="toggleChannel(key, 'in_app')"
                  >
                    <span
                      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                      :class="getChannel(key, 'in_app') ? 'translate-x-5' : 'translate-x-0'"
                    />
                  </button>
                </label>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Info note -->
      <div class="card bg-primary-50 border-primary-200">
        <div class="px-6 py-4 flex items-start gap-3">
          <Icon name="infoCircle" size="md" class="text-primary-600 flex-shrink-0 mt-0.5" />
          <div>
            <p class="text-sm font-medium text-primary-800">偏好说明</p>
            <ul class="mt-1 text-xs text-primary-700 space-y-1 list-disc list-inside">
              <li><strong>组织偏好</strong>由管理员设置，对所有成员生效</li>
              <li><strong>个人偏好</strong>覆盖组织级别的设置</li>
              <li><strong>安全通知</strong>为重要告警，至少保留一个通知渠道</li>
            </ul>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.tabs {
  display: flex;
  gap: 0.25rem;
  padding: 0.25rem;
  border-radius: 0.75rem;
  background: #f3f4f6;
  width: fit-content;
}
.tab {
  border-radius: 0.5rem;
  padding: 0.5rem 1rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: #4b5563;
  transition: all 0.15s;
  cursor: pointer;
  border: none;
  background: transparent;
}
.tab-active {
  background: white;
  color: #111827;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
}
</style>
