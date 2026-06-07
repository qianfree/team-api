<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import request from '@/utils/request'
import Icon from '@/components/common/Icon.vue'
import { useConfirm } from '@/composables/useConfirm'

const { confirm } = useConfirm()

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
  event_type: string
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
const logsPage = ref(1)
const logsPageSize = 20
const logsTotal = ref(0)
const logsStatusFilter = ref('')

const logsTotalPages = computed(() => Math.max(1, Math.ceil(logsTotal.value / logsPageSize)))

const availableEvents = [
  'wallet.low_balance',
]

const showPayloadGuide = ref(false)

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
  form.events = ['wallet.low_balance']
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
  if (!await confirm({ message: '确定要删除此 Webhook 配置吗？', confirmText: '确认删除', danger: true })) return
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
  logsPage.value = 1
  logsStatusFilter.value = ''
  showLogsModal.value = true
  await fetchLogs()
}

async function fetchLogs() {
  logsLoading.value = true
  try {
    const params: Record<string, string | number> = {
      page: logsPage.value,
      page_size: logsPageSize,
    }
    if (logsStatusFilter.value) {
      params.status = logsStatusFilter.value
    }
    const res = await request.get(`/tenant/open/webhooks/${logsConfigId.value}/logs`, { params })
    if (res.data?.code === 0) {
      logs.value = res.data.data.list || []
      logsTotal.value = res.data.data.total || 0
    }
  } catch (e) {
    console.error(e)
  } finally {
    logsLoading.value = false
  }
}

function setLogsFilter(status: string) {
  logsStatusFilter.value = status
  logsPage.value = 1
  fetchLogs()
}

function goToLogsPage(page: number) {
  if (page < 1 || page > logsTotalPages.value) return
  logsPage.value = page
  fetchLogs()
}

async function handleRetry(eventId: number) {
  try {
    const res = await request.post(`/tenant/open/webhooks/events/${eventId}/retry`)
    if (res.data?.code === 0) await fetchLogs()
  } catch (e) {
    console.error(e)
  }
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text)
}

function isLogSuccess(log: DeliveryLog) {
  return log.response_status >= 200 && log.response_status < 300
}

function formatStatus(status: number) {
  if (!status) return '---'
  return String(status)
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
              <button class="btn btn-ghost btn-sm" @click="loadLogs(cfg.id)">投递记录</button>
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

    <!-- Payload Format Guide -->
    <div class="card mt-6">
      <div class="card-body">
        <div class="flex items-center justify-between cursor-pointer select-none" @click="showPayloadGuide = !showPayloadGuide">
          <div>
            <h3 class="text-lg font-semibold text-gray-900">推送格式说明</h3>
            <p class="text-sm text-gray-500 mt-0.5">了解 Webhook 请求的 Headers 和 Body 格式</p>
          </div>
          <Icon :name="showPayloadGuide ? 'chevronDown' : 'chevronRight'" size="md" class="text-gray-400 transition-transform duration-200" />
        </div>

        <div v-if="showPayloadGuide" class="mt-5 space-y-5 animate-fade-in">
          <!-- HTTP Request -->
          <div>
            <h4 class="text-sm font-semibold text-gray-800 mb-2">HTTP 请求</h4>
            <div class="code-block text-xs leading-relaxed">
              <span class="text-emerald-400">POST</span> <span class="text-amber-300">{your_webhook_url}</span>
            </div>
          </div>

          <!-- Headers -->
          <div>
            <h4 class="text-sm font-semibold text-gray-800 mb-2">请求 Headers</h4>
            <div class="code-block text-xs leading-relaxed">
<pre class="whitespace-pre-wrap"><span class="text-gray-400">Content-Type:</span> <span class="text-amber-300">application/json</span>
<span class="text-gray-400">X-Webhook-ID:</span>      <span class="text-amber-300">evt_1a2b3c4d5e6f...</span>        <span class="text-gray-600">  // 事件唯一 ID</span>
<span class="text-gray-400">X-Webhook-Event:</span>   <span class="text-amber-300">wallet.low_balance</span>          <span class="text-gray-600">  // 事件类型</span>
<span class="text-gray-400">X-Webhook-Timestamp:</span> <span class="text-amber-300">1717740600</span>                 <span class="text-gray-600">  // Unix 时间戳</span>
<span class="text-gray-400">X-Webhook-Signature:</span> <span class="text-amber-300">a3f8b2c1d4e5...</span>            <span class="text-gray-600">  // HMAC-SHA256 签名</span></pre>
            </div>
          </div>

          <!-- Signature Verification -->
          <div>
            <h4 class="text-sm font-semibold text-gray-800 mb-2">签名验证</h4>
            <p class="text-xs text-gray-600 mb-2">使用创建 Webhook 时返回的 Secret Key 验证请求真实性，防止伪造。</p>
            <div class="code-block text-xs leading-relaxed">
<pre class="whitespace-pre-wrap"><span class="text-gray-400">// 签名算法</span>
<span class="text-sky-400">signature</span> = <span class="text-emerald-400">HMAC-SHA256</span>(<span class="text-sky-400">secret_key</span>, <span class="text-sky-400">timestamp</span> + <span class="text-amber-300">"."</span> + <span class="text-sky-400">request_body</span>)

<span class="text-gray-400">// 验证方式</span>
<span class="text-violet-400">if</span> (<span class="text-sky-400">your_signature</span> === <span class="text-sky-400">header['X-Webhook-Signature']</span>) → <span class="text-emerald-400">✓ 合法</span></pre>
            </div>
          </div>

          <!-- Response Handling -->
          <div>
            <h4 class="text-sm font-semibold text-gray-800 mb-2">响应处理</h4>
            <div class="text-xs text-gray-600 space-y-1">
              <p>• 返回 HTTP <span class="font-mono text-emerald-600 bg-emerald-50 px-1.5 py-0.5 rounded">2xx</span> 视为投递成功</p>
              <p>• 非 2xx 响应将触发自动重试（最多 5 次，间隔递增：1min → 5min → 15min → 1h → 6h）</p>
              <p>• 连续失败达到上限后自动禁用此 Webhook，需手动启用</p>
            </div>
          </div>

          <!-- Event Payloads -->
          <div>
            <h4 class="text-sm font-semibold text-gray-800 mb-3">事件 Payload 示例</h4>

            <div class="space-y-4">
              <!-- wallet.low_balance -->
              <div>
                <div class="flex items-center gap-2 mb-2">
                  <span class="badge badge-primary text-xs">wallet.low_balance</span>
                  <span class="text-xs text-gray-500">余额低于预警线时触发</span>
                </div>
                <div class="code-block text-xs leading-relaxed">
<pre class="whitespace-pre-wrap">{
  <span class="text-gray-400">"event_id"</span>:   <span class="text-amber-300">"evt_1a2b3c4d5e6f..."</span>,
  <span class="text-gray-400">"event_type"</span>: <span class="text-amber-300">"wallet.low_balance"</span>,
  <span class="text-gray-400">"tenant_id"</span>:  <span class="text-violet-400">213</span>,
  <span class="text-gray-400">"timestamp"</span>:  <span class="text-amber-300">"2025-01-15T10:30:00Z"</span>,
  <span class="text-gray-400">"data"</span>: {
    <span class="text-gray-400">"tenant_code"</span>:       <span class="text-amber-300">"acme"</span>,
    <span class="text-gray-400">"tenant_name"</span>:      <span class="text-amber-300">"ACME Corp"</span>,
    <span class="text-gray-400">"available_balance"</span>: <span class="text-violet-400">87.116193</span>,
    <span class="text-gray-400">"warning_threshold"</span>: <span class="text-violet-400">87.180000</span>,
    <span class="text-gray-400">"wallet_id"</span>:         <span class="text-violet-400">456</span>
  }
}</pre>
                </div>
              </div>
            </div>
          </div>
        </div>
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
              <div class="flex items-center gap-2 mt-1">
                <span class="badge badge-primary">wallet.low_balance</span>
                <span class="text-xs text-gray-400">余额低于预警线时触发</span>
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
        <div class="modal-content bg-white w-full max-w-3xl max-h-[85vh]">
          <div class="modal-header">
            <h3 class="modal-title">投递记录</h3>
            <button class="btn btn-ghost btn-icon" @click="showLogsModal = false">
              <Icon name="x" size="sm" />
            </button>
          </div>

          <!-- Filter tabs -->
          <div class="px-4 sm:px-6 pt-4">
            <div class="tabs">
              <button
                class="tab"
                :class="{ 'tab-active': logsStatusFilter === '' }"
                @click="setLogsFilter('')"
              >全部</button>
              <button
                class="tab"
                :class="{ 'tab-active': logsStatusFilter === 'delivered' }"
                @click="setLogsFilter('delivered')"
              >已送达</button>
              <button
                class="tab"
                :class="{ 'tab-active': logsStatusFilter === 'failed' }"
                @click="setLogsFilter('failed')"
              >失败</button>
            </div>
          </div>

          <div class="modal-body overflow-y-auto">
            <!-- Loading -->
            <div v-if="logsLoading" class="text-center py-12">
              <div class="spinner h-6 w-6 mx-auto"></div>
            </div>

            <!-- Empty -->
            <div v-else-if="logs.length === 0" class="text-center py-12 text-gray-500">
              <p>暂无投递记录</p>
              <p class="text-xs mt-1 text-gray-400">事件触发后将在此显示投递详情</p>
            </div>

            <!-- Log list -->
            <div v-else class="space-y-2">
              <div
                v-for="log in logs"
                :key="log.id"
                class="rounded-xl border p-3 transition-colors"
                :class="isLogSuccess(log) ? 'border-gray-200 hover:bg-gray-50' : 'border-red-200 bg-red-50/30 hover:bg-red-50/50'"
              >
                <!-- Row 1: event type + status badge + time -->
                <div class="flex items-center justify-between mb-2">
                  <div class="flex items-center gap-2">
                    <span class="badge text-xs" :class="isLogSuccess(log) ? 'badge-success' : 'badge-danger'">
                      {{ isLogSuccess(log) ? '成功' : '失败' }}
                    </span>
                    <span class="badge badge-primary text-xs">{{ log.event_type || '未知事件' }}</span>
                  </div>
                  <span class="text-xs text-gray-400">{{ log.created_at }}</span>
                </div>

                <!-- Row 2: details -->
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-4 text-xs text-gray-500">
                    <span>
                      HTTP
                      <span class="font-mono font-medium" :class="isLogSuccess(log) ? 'text-emerald-600' : 'text-red-600'">
                        {{ formatStatus(log.response_status) }}
                      </span>
                    </span>
                    <span>第 {{ log.attempt }} 次尝试</span>
                    <span v-if="log.response_time_ms">
                      <span :class="log.response_time_ms > 3000 ? 'text-amber-600 font-medium' : ''">{{ log.response_time_ms }}ms</span>
                    </span>
                  </div>
                  <div class="flex items-center gap-1">
                    <button
                      v-if="!isLogSuccess(log)"
                      class="btn btn-ghost btn-sm text-xs"
                      @click="handleRetry(log.event_id)"
                    >重试</button>
                  </div>
                </div>

                <!-- Row 3: error message -->
                <p v-if="log.error_message" class="mt-2 text-xs text-red-600 bg-red-100/60 rounded-lg px-2 py-1 break-all">
                  {{ log.error_message }}
                </p>
              </div>
            </div>
          </div>

          <!-- Pagination -->
          <div v-if="logsTotal > logsPageSize" class="modal-footer justify-between">
            <span class="text-xs text-gray-400">
              共 {{ logsTotal }} 条记录，第 {{ logsPage }} / {{ logsTotalPages }} 页
            </span>
            <div class="flex items-center gap-2">
              <button
                class="btn btn-ghost btn-sm"
                :disabled="logsPage <= 1"
                @click="goToLogsPage(logsPage - 1)"
              >上一页</button>
              <button
                class="btn btn-ghost btn-sm"
                :disabled="logsPage >= logsTotalPages"
                @click="goToLogsPage(logsPage + 1)"
              >下一页</button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
