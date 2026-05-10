<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import request from '@/utils/request'
import Icon from '@/components/common/Icon.vue'

interface OpenApp {
  id: number
  name: string
  description: string
  app_id: string
  permissions: string[]
  status: string
  is_sandbox: boolean
  rate_limit: number
  last_used_at: string
  created_at: string
}

const loading = ref(false)
const apps = ref<OpenApp[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')

const showModal = ref(false)
const modalMode = ref<'create' | 'edit'>('create')
const saving = ref(false)
const form = reactive({
  id: 0,
  name: '',
  description: '',
  permissions: [] as string[],
  ip_whitelist: [] as string[],
  rate_limit: 60,
})

const showSecretModal = ref(false)
const newSecret = ref('')
const newAppId = ref('')

const availablePermissions = [
  'members:read', 'members:write',
  'keys:read', 'keys:write',
  'models:read',
  'usage:read',
  'billing:read',
]

onMounted(() => loadApps())

async function loadApps() {
  loading.value = true
  try {
    const res = await request.get('/tenant/open/apps', {
      params: { page: page.value, page_size: pageSize.value, keyword: keyword.value },
    })
    if (res.data?.code === 0) {
      apps.value = res.data.data.list || []
      total.value = res.data.data.total || 0
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
  form.description = ''
  form.permissions = ['members:read']
  form.ip_whitelist = []
  form.rate_limit = 60
  showModal.value = true
}

function openEditModal(app: OpenApp) {
  modalMode.value = 'edit'
  form.id = app.id
  form.name = app.name
  form.description = app.description
  form.permissions = [...app.permissions]
  form.rate_limit = app.rate_limit
  showModal.value = true
}

async function handleSave() {
  if (!form.name) return
  saving.value = true
  try {
    if (modalMode.value === 'create') {
      const res = await request.post('/tenant/open/apps', {
        name: form.name,
        description: form.description,
        permissions: form.permissions,
        ip_whitelist: form.ip_whitelist,
        rate_limit: form.rate_limit,
      })
      if (res.data?.code === 0) {
        newAppId.value = res.data.data.app_id
        newSecret.value = res.data.data.app_secret
        showSecretModal.value = true
        showModal.value = false
        await loadApps()
      }
    } else {
      const res = await request.put(`/tenant/open/apps/${form.id}`, {
        name: form.name,
        description: form.description,
        permissions: form.permissions,
        rate_limit: form.rate_limit,
      })
      if (res.data?.code === 0) {
        showModal.value = false
        await loadApps()
      }
    }
  } catch (e) {
    console.error(e)
  } finally {
    saving.value = false
  }
}

async function handleDelete(id: number) {
  if (!confirm('确定要删除此应用吗？此操作不可恢复。')) return
  try {
    const res = await request.delete(`/tenant/open/apps/${id}`)
    if (res.data?.code === 0) {
      await loadApps()
    }
  } catch (e) {
    console.error(e)
  }
}

async function handleResetSecret(id: number) {
  if (!confirm('确定要重置密钥吗？旧密钥将立即失效。')) return
  try {
    const res = await request.post(`/tenant/open/apps/${id}/reset-secret`)
    if (res.data?.code === 0) {
      newSecret.value = res.data.data.app_secret
      showSecretModal.value = true
    }
  } catch (e) {
    console.error(e)
  }
}

async function handleToggleStatus(app: OpenApp) {
  const newStatus = app.status === 'active' ? 'disabled' : 'active'
  try {
    const res = await request.put(`/tenant/open/apps/${app.id}/status`, { status: newStatus })
    if (res.data?.code === 0) {
      await loadApps()
    }
  } catch (e) {
    console.error(e)
  }
}

function togglePermission(perm: string) {
  const idx = form.permissions.indexOf(perm)
  if (idx >= 0) form.permissions.splice(idx, 1)
  else form.permissions.push(perm)
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text)
}
</script>

<template>
  <div>
    <div class="page-header flex items-center justify-between">
      <div>
        <h1 class="page-title">开放平台</h1>
        <p class="page-description">管理第三方应用接入，通过 Open API 集成您的服务</p>
      </div>
      <button class="btn btn-primary" @click="openCreateModal">
        <Icon name="plus" size="sm" />
        创建应用
      </button>
    </div>

    <!-- Search -->
    <div class="card mb-6">
      <div class="card-body">
        <div class="flex gap-3">
          <input v-model="keyword" type="text" class="input" placeholder="搜索应用名称..." @keydown.enter="loadApps" />
          <button class="btn btn-secondary" @click="loadApps">搜索</button>
        </div>
      </div>
    </div>

    <!-- App List -->
    <div class="space-y-4">
      <div v-for="app in apps" :key="app.id" class="card card-hover">
        <div class="card-body">
          <div class="flex items-start justify-between">
            <div class="flex-1">
              <div class="flex items-center gap-3 mb-1">
                <h3 class="text-lg font-semibold text-gray-900">{{ app.name }}</h3>
                <span v-if="app.is_sandbox" class="badge badge-warning">沙箱</span>
                <span :class="app.status === 'active' ? 'badge-success' : 'badge-danger'" class="badge">
                  {{ app.status === 'active' ? '已启用' : '已禁用' }}
                </span>
              </div>
              <p class="text-sm text-gray-500 mb-2">{{ app.description || '无描述' }}</p>
              <div class="flex items-center gap-4 text-xs text-gray-400">
                <span class="font-mono">{{ app.app_id }}</span>
                <span>限制: {{ app.rate_limit }}/min</span>
                <span>创建于: {{ app.created_at }}</span>
              </div>
              <div class="flex flex-wrap gap-1 mt-2">
                <span v-for="p in app.permissions" :key="p" class="badge badge-primary text-xs">{{ p }}</span>
              </div>
            </div>
            <div class="flex items-center gap-2 ml-4">
              <button class="btn btn-ghost btn-sm" @click="handleToggleStatus(app)">
                {{ app.status === 'active' ? '禁用' : '启用' }}
              </button>
              <button class="btn btn-ghost btn-sm" @click="openEditModal(app)">编辑</button>
              <button class="btn btn-ghost btn-sm" @click="handleResetSecret(app.id)">重置密钥</button>
              <button class="btn btn-ghost btn-sm text-red-600 hover:text-red-700" @click="handleDelete(app.id)">删除</button>
            </div>
          </div>
        </div>
      </div>

      <!-- Empty -->
      <div v-if="!loading && apps.length === 0" class="empty-state">
        <div class="empty-state-icon">
          <Icon name="cog" size="xl" />
        </div>
        <h3 class="empty-state-title">暂无应用</h3>
        <p class="empty-state-description">创建一个开放平台应用开始接入</p>
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <Teleport to="body">
      <div v-if="showModal" class="modal-overlay" @click.self="showModal = false">
        <div class="modal-content bg-white w-full max-w-lg">
          <div class="modal-header">
            <h3 class="modal-title">{{ modalMode === 'create' ? '创建应用' : '编辑应用' }}</h3>
            <button class="btn btn-ghost btn-icon" @click="showModal = false">
              <Icon name="x" size="sm" />
            </button>
          </div>
          <div class="modal-body space-y-4">
            <div>
              <label class="input-label">应用名称</label>
              <input v-model="form.name" type="text" class="input" placeholder="输入应用名称" />
            </div>
            <div>
              <label class="input-label">应用描述</label>
              <textarea v-model="form.description" class="input" rows="3" placeholder="输入应用描述"></textarea>
            </div>
            <div>
              <label class="input-label">权限范围</label>
              <div class="grid grid-cols-2 gap-2">
                <label v-for="perm in availablePermissions" :key="perm" class="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" :checked="form.permissions.includes(perm)" @change="togglePermission(perm)" class="rounded border-gray-300" />
                  <span class="text-sm">{{ perm }}</span>
                </label>
              </div>
            </div>
            <div>
              <label class="input-label">请求频率限制（每分钟）</label>
              <input v-model.number="form.rate_limit" type="number" class="input" min="1" max="10000" />
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

    <!-- Secret Display Modal -->
    <Teleport to="body">
      <div v-if="showSecretModal" class="modal-overlay" @click.self="showSecretModal = false">
        <div class="modal-content bg-white w-full max-w-lg">
          <div class="modal-header">
            <h3 class="modal-title">应用密钥</h3>
            <button class="btn btn-ghost btn-icon" @click="showSecretModal = false">
              <Icon name="x" size="sm" />
            </button>
          </div>
          <div class="modal-body">
            <div class="rounded-xl bg-amber-50 border border-amber-200 p-4 mb-4">
              <p class="text-sm text-amber-800 font-medium">请立即复制并安全保存此密钥，关闭后将无法再次查看。</p>
            </div>
            <div v-if="newAppId" class="mb-3">
              <label class="input-label">App ID</label>
              <div class="flex items-center gap-2">
                <code class="code-block flex-1 text-sm">{{ newAppId }}</code>
                <button class="btn btn-ghost btn-sm" @click="copyToClipboard(newAppId)">
                  <Icon name="copy" size="sm" />
                </button>
              </div>
            </div>
            <div>
              <label class="input-label">App Secret</label>
              <div class="flex items-center gap-2">
                <code class="code-block flex-1 text-sm break-all">{{ newSecret }}</code>
                <button class="btn btn-ghost btn-sm" @click="copyToClipboard(newSecret)">
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
  </div>
</template>
