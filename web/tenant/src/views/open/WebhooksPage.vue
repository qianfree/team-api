<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import request from '@/utils/request'
import Icon from '@/components/common/Icon.vue'

interface WebhookConfig {
  id: number
  name: string
  url: string
  events: string[]
  is_active: boolean
  consecutive_failures: number
  max_consecutive_failures: number
  last_delivery_at: string
  created_at: string
}

interface DeliveryLog {
  id: number
  event_id: number
  attempt: number
  response_status: number
  response_time_ms: number
  error_message: string
  created_at: string
}

const loading = ref(false)
const configs = ref<WebhookConfig[]>([])

const showModal = ref(false)
const modalMode = ref<'create' | 'edit'>('create')
const saving = ref(false)
const form = reactive({
  id: 0,
  name: '',
  url: '',
  events: [] as string[],
  max_consecutive_failures: 10,
})

const showSecretModal = ref(false)
const newSecretKey = ref('')

const showLogsModal = ref(false)
const logs = ref<DeliveryLog[]>([])
const logsLoading = ref(false)
const logsConfigId = ref(0)

const availableEvents = [
  'member.created', 'member.updated', 'member.deleted',
  'key.created', 'key.deleted',
  'order.paid', 'order.refunded',
  'plan.subscribed', 'plan.expired',
  'wallet.topup', 'wallet.low_balance',
]

onMounted(() => loadConfigs())

async function loadConfigs() {
  loading.value = true
  try {
    const res = await request.get('/tenant/open/webhooks')
    if (res.data?.code === 0) {
      configs.value = res.data.data.list || []
    }
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

function openCreateModal() {
  modalMode.value = 'create'
  form.id = 0
  form.name = ''
  form.url = ''
  form.events = []
  form.max_consecutive_failures = 10
  showModal.value = true
}

function openEditModal(cfg: WebhookConfig) {
  modalMode.value = 'edit'
  form.id = cfg.id
  form.name = cfg.name
  form.url = cfg.url
  form.events = [...cfg.events]
  form.max_consecutive_failures = cfg.max_consecutive_failures
  showModal.value = true
}

async function handleSave() {
  if (!form.name || !form.url || form.events.length === 0) return
  saving.value = true
  try {
    if (modalMode.value === 'create') {
      const res = await request.post('/tenant/open/webhooks', {
        name: form.name,
        url: form.url,
        events: form.events,
        max_consecutive_failures: form.max_consecutive_failures,
      })
      if (res.data?.code === 0) {
        newSecretKey.value = res.data.data.secret_key
        showSecretModal.value = true
        showModal.value = false
        await loadConfigs()
      }
    } else {
      const res = await request.put(`/tenant/open/webhooks/${form.id}`, {
        name: form.name,
        url: form.url,
        events: form.events,
        max_consecutive_failures: form.max_consecutive_failures,
      })
      if (res.data?.code === 0) {
        showModal.value = false
        await loadConfigs()
      }
    }
  } catch (e) {
    console.error(e)
  } finally {
    saving.value = false
  }
}

async function handleDelete(id: number) {
  if (!confirm('确定要删除此 Webhook 配置吗？')) return
  try {
    const res = await request.delete(`/tenant/open/webhooks/${id}`)
    if (res.data?.code === 0) await loadConfigs()
  } catch (e) {
    console.error(e)
  }
}

async function handleToggle(cfg: WebhookConfig) {
  try {
    const res = await request.put(`/tenant/open/webhooks/${cfg.id}`, {
      is_active: !cfg.is_active,
    })
    if (res.data?.code === 0) await loadConfigs()
  } catch (e) {
    console.error(e)
  }
}

async function loadLogs(configId: number) {
  logsConfigId.value = configId
  showLogsModal.value = true
  logsLoading.value = true
  try {
    const res = await request.get(`/tenant/open/webhooks/${configId}/logs`)
    if (res.data?.code === 0) {
      logs.value = res.data.data.list || []
    }
  } catch (e) {
    console.error(e)
  } finally {
    logsLoading.value = false
  }
}

async function handleRetry(eventId: number) {
  try {
    const res = await request.post(`/tenant/open/webhooks/events/${eventId}/retry`)
    if (res.data?.code === 0) await loadLogs(logsConfigId.value)
  } catch (e) {
    console.error(e)
  }
}

function toggleEvent(evt: string) {
  const idx = form.events.indexOf(evt)
  if (idx >= 0) form.events.splice(idx, 1)
  else form.events.push(evt)
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text)
}
</script>

<template>
  <div>
    <div class="page-header flex items-center justify-between">
      <div>
        <h1 class="page-title">Webhook 管理</h1>
        <p class="page-description">配置事件推送，实时接收平台事件通知</p>
      </div>
      <button class="btn btn-primary" @click="openCreateModal">
        <Icon name="plus" size="sm" />
        创建 Webhook
      </button>
    </div>

    <!-- Config List -->
    <div class="space-y-4">
      <div v-for="cfg in configs" :key="cfg.id" class="card card-hover">
        <div class="card-body">
          <div class="flex items-start justify-between">
            <div class="flex-1">
              <div class="flex items-center gap-3 mb-1">
                <h3 class="text-lg font-semibold text-gray-900">{{ cfg.name }}</h3>
                <span :class="cfg.is_active ? 'badge-success' : 'badge-danger'" class="badge">
                  {{ cfg.is_active ? '已启用' : '已禁用' }}
                </span>
                <span v-if="cfg.consecutive_failures > 0" class="badge badge-warning">
                  连续失败 {{ cfg.consecutive_failures }}
                </span>
              </div>
              <p class="text-sm text-gray-500 mb-2 font-mono">{{ cfg.url }}</p>
              <div class="flex flex-wrap gap-1 mb-2">
                <span v-for="evt in cfg.events" :key="evt" class="badge badge-primary text-xs">{{ evt }}</span>
              </div>
              <div class="text-xs text-gray-400">
                <span v-if="cfg.last_delivery_at">最后投递: {{ cfg.last_delivery_at }}</span>
                <span v-else>尚未投递</span>
                <span class="ml-3">最大失败: {{ cfg.max_consecutive_failures }}</span>
              </div>
            </div>
            <div class="flex items-center gap-2 ml-4">
              <button class="btn btn-ghost btn-sm" @click="loadLogs(cfg.id)">日志</button>
              <button class="btn btn-ghost btn-sm" @click="handleToggle(cfg)">
                {{ cfg.is_active ? '禁用' : '启用' }}
              </button>
              <button class="btn btn-ghost btn-sm" @click="openEditModal(cfg)">编辑</button>
              <button class="btn btn-ghost btn-sm text-red-600 hover:text-red-700" @click="handleDelete(cfg.id)">删除</button>
            </div>
          </div>
        </div>
      </div>

      <!-- Empty -->
      <div v-if="!loading && configs.length === 0" class="empty-state">
        <div class="empty-state-icon">
          <Icon name="link" size="xl" />
        </div>
        <h3 class="empty-state-title">暂无 Webhook</h3>
        <p class="empty-state-description">创建 Webhook 配置以接收实时事件通知</p>
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <Teleport to="body">
      <div v-if="showModal" class="modal-overlay" @click.self="showModal = false">
        <div class="modal-content bg-white w-full max-w-lg">
          <div class="modal-header">
            <h3 class="modal-title">{{ modalMode === 'create' ? '创建 Webhook' : '编辑 Webhook' }}</h3>
            <button class="btn btn-ghost btn-icon" @click="showModal = false">
              <Icon name="x" size="sm" />
            </button>
          </div>
          <div class="modal-body space-y-4">
            <div>
              <label class="input-label">配置名称</label>
              <input v-model="form.name" type="text" class="input" placeholder="如：生产环境通知" />
            </div>
            <div>
              <label class="input-label">回调 URL（HTTPS）</label>
              <input v-model="form.url" type="url" class="input" placeholder="https://example.com/webhook" />
            </div>
            <div>
              <label class="input-label">订阅事件</label>
              <div class="grid grid-cols-2 gap-2">
                <label v-for="evt in availableEvents" :key="evt" class="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" :checked="form.events.includes(evt)" @change="toggleEvent(evt)" class="rounded border-gray-300" />
                  <span class="text-sm">{{ evt }}</span>
                </label>
              </div>
            </div>
            <div>
              <label class="input-label">最大连续失败次数</label>
              <input v-model.number="form.max_consecutive_failures" type="number" class="input" min="1" max="100" />
              <p class="input-hint">超过后自动禁用此 Webhook</p>
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-secondary" @click="showModal = false">取消</button>
            <button class="btn btn-primary" :disabled="saving" @click="handleSave">
              <div v-if="saving" class="spinner h-4 w-4 border-white"></div>
              {{ saving ? '保存中...' : '保存' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Secret Key Modal -->
    <Teleport to="body">
      <div v-if="showSecretModal" class="modal-overlay" @click.self="showSecretModal = false">
        <div class="modal-content bg-white w-full max-w-lg">
          <div class="modal-header">
            <h3 class="modal-title">签名密钥</h3>
            <button class="btn btn-ghost btn-icon" @click="showSecretModal = false">
              <Icon name="x" size="sm" />
            </button>
          </div>
          <div class="modal-body">
            <div class="rounded-xl bg-amber-50 border border-amber-200 p-4 mb-4">
              <p class="text-sm text-amber-800 font-medium">请立即复制并安全保存此签名密钥，关闭后将无法再次查看。</p>
            </div>
            <div>
              <label class="input-label">Secret Key</label>
              <div class="flex items-center gap-2">
                <code class="code-block flex-1 text-sm break-all">{{ newSecretKey }}</code>
                <button class="btn btn-ghost btn-sm" @click="copyToClipboard(newSecretKey)">
                  <Icon name="copy" size="sm" />
                </button>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-primary" @click="showSecretModal = false">我已安全保存</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Delivery Logs Modal -->
    <Teleport to="body">
      <div v-if="showLogsModal" class="modal-overlay" @click.self="showLogsModal = false">
        <div class="modal-content bg-white w-full max-w-2xl max-h-[80vh]">
          <div class="modal-header">
            <h3 class="modal-title">投递日志</h3>
            <button class="btn btn-ghost btn-icon" @click="showLogsModal = false">
              <Icon name="x" size="sm" />
            </button>
          </div>
          <div class="modal-body overflow-y-auto">
            <div v-if="logsLoading" class="text-center py-8">
              <div class="spinner h-6 w-6 mx-auto"></div>
            </div>
            <div v-else-if="logs.length === 0" class="text-center py-8 text-gray-500">
              暂无投递日志
            </div>
            <div v-else class="space-y-3">
              <div v-for="log in logs" :key="log.id" class="rounded-xl border border-gray-200 p-3">
                <div class="flex items-center justify-between mb-1">
                  <div class="flex items-center gap-2">
                    <span :class="log.response_status >= 200 && log.response_status < 300 ? 'text-emerald-600' : 'text-red-600'" class="text-sm font-medium">
                      {{ log.response_status || '---' }}
                    </span>
                    <span class="text-xs text-gray-400">第 {{ log.attempt }} 次</span>
                  </div>
                  <div class="flex items-center gap-2">
                    <span class="text-xs text-gray-400">{{ log.response_time_ms }}ms</span>
                    <span class="text-xs text-gray-400">{{ log.created_at }}</span>
                    <button class="btn btn-ghost btn-sm text-xs" @click="handleRetry(log.event_id)">重试</button>
                  </div>
                </div>
                <p v-if="log.error_message" class="text-xs text-red-500">{{ log.error_message }}</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
