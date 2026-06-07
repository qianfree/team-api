<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { showToast, showConfirmDialog } from 'vant'
import request from '@/utils/request'

// ── State ──
const loading = ref(false)
const refreshing = ref(false)
const finished = ref(false)
const users = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const search = ref('')

const roleFilter = ref('all')
const statusFilter = ref('all')

// Dialogs
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const showResetPwdDialog = ref(false)
const showActionSheet = ref(false)

const actionSheetUser = ref<any>(null)
const editingUser = ref<any>(null)
const resetPwdUser = ref<any>(null)

// Create form
const createForm = ref({
  username: '',
  password: '',
  email: '',
  role: 'admin',
})

// Edit form
const editForm = ref({
  display_name: '',
  email: '',
  role: 'admin',
})

// Reset password form
const resetPwdForm = ref({
  new_password: '',
})

// ── Filter options ──
const roleOptions = [
  { value: 'all', label: '全部' },
  { value: 'super_admin', label: '超级管理员' },
  { value: 'admin', label: '管理员' },
]

const statusOptions = [
  { value: 'all', label: '全部' },
  { value: 'active', label: '启用' },
  { value: 'disabled', label: '禁用' },
]

// ── Helpers ──
const roleLabel: Record<string, string> = {
  super_admin: '超级管理员',
  admin: '管理员',
}

const roleConfig: Record<string, { color: string; bg: string }> = {
  super_admin: { color: '#7c3aed', bg: 'rgba(124,58,237,0.1)' },
  admin: { color: '#3b82f6', bg: 'rgba(59,130,246,0.1)' },
}

const statusConfig: Record<string, { label: string; color: string; bg: string }> = {
  active: { label: '启用', color: '#10b981', bg: 'rgba(16,185,129,0.1)' },
  disabled: { label: '禁用', color: '#ef4444', bg: 'rgba(239,68,68,0.1)' },
}

function getRoleConf(role: string) {
  return roleConfig[role] || roleConfig.admin
}

function getStatusConf(status: string) {
  return statusConfig[status] || statusConfig.active
}

function formatTime(t: string | undefined | null): string {
  if (!t) return '-'
  const d = new Date(t)
  const now = new Date()
  const diffMs = now.getTime() - d.getTime()
  const diffMin = Math.floor(diffMs / 60000)
  if (diffMin < 1) return '刚刚'
  if (diffMin < 60) return `${diffMin}分钟前`
  const diffH = Math.floor(diffMin / 60)
  if (diffH < 24) return `${diffH}小时前`
  const diffD = Math.floor(diffH / 24)
  if (diffD < 30) return `${diffD}天前`
  const mm = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  return `${mm}-${dd}`
}

function displayName(item: any): string {
  return item.display_name || item.username || '-'
}

// ── API ──
async function fetchUsers(append = false) {
  if (!append) {
    page.value = 1
    finished.value = false
  }

  loading.value = true
  try {
    const params: any = {
      page: page.value,
      page_size: 20,
    }
    if (search.value) params.keyword = search.value
    if (roleFilter.value !== 'all') params.role = roleFilter.value
    if (statusFilter.value !== 'all') params.status = statusFilter.value

    const { data: res } = await request.get('/admin/users', { params })
    const list = res.data?.list || []
    total.value = res.data?.total || 0

    if (append) {
      users.value = [...users.value, ...list]
    } else {
      users.value = list
    }
    finished.value = users.value.length >= total.value
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function onRefresh() {
  refreshing.value = true
  await fetchUsers(false)
}

async function onLoad() {
  if (finished.value) return
  page.value++
  await fetchUsers(true)
}

function onSearch() {
  fetchUsers(false)
}

function onFilterChange() {
  fetchUsers(false)
}

// ── CRUD ──
async function createUser() {
  const f = createForm.value
  if (!f.username || f.username.length < 3 || f.username.length > 50) {
    showToast('用户名需 3-50 个字符')
    return
  }
  if (!f.password || f.password.length < 8 || f.password.length > 64) {
    showToast('密码需 8-64 个字符')
    return
  }
  try {
    await request.post('/admin/users', f)
    showToast('创建成功')
    showCreateDialog.value = false
    createForm.value = { username: '', password: '', email: '', role: 'admin' }
    await fetchUsers(false)
  } catch {
    // handled by interceptor
  }
}

function openEditDialog(user: any) {
  editingUser.value = user
  editForm.value = {
    display_name: user.display_name || '',
    email: user.email || '',
    role: user.role || 'admin',
  }
  showEditDialog.value = true
}

async function updateUser() {
  if (!editingUser.value) return
  try {
    await request.put(`/admin/users/${editingUser.value.id}`, editForm.value)
    showToast('更新成功')
    showEditDialog.value = false
    await fetchUsers(false)
  } catch {
    // handled by interceptor
  }
}

async function toggleStatus(user: any) {
  const newStatus = user.status === 'active' ? 'disabled' : 'active'
  const action = newStatus === 'active' ? '启用' : '禁用'
  try {
    await showConfirmDialog({
      title: `确认${action}`,
      message: `确定要${action}用户「${displayName(user)}」吗？`,
    })
    await request.put(`/admin/users/${user.id}/status`, { status: newStatus })
    showToast(`已${action}`)
    await fetchUsers(false)
  } catch {
    // cancelled or error
  }
}

async function deleteUser(user: any) {
  try {
    await showConfirmDialog({
      title: '确认删除',
      message: `确定要删除用户「${displayName(user)}」吗？此操作不可恢复。`,
    })
    await request.delete(`/admin/users/${user.id}`)
    showToast('已删除')
    await fetchUsers(false)
  } catch {
    // cancelled or error
  }
}

function openResetPwdDialog(user: any) {
  resetPwdUser.value = user
  resetPwdForm.value = { new_password: '' }
  showResetPwdDialog.value = true
}

async function resetPassword() {
  if (!resetPwdUser.value) return
  const pwd = resetPwdForm.value.new_password
  if (!pwd || pwd.length < 8 || pwd.length > 64) {
    showToast('密码需 8-64 个字符')
    return
  }
  try {
    await request.put(`/admin/users/${resetPwdUser.value.id}/reset-password`, {
      new_password: pwd,
    })
    showToast('密码已重置')
    showResetPwdDialog.value = false
  } catch {
    // handled by interceptor
  }
}

// ── Long-press actions ──
const actionSheetActions = computed(() => {
  const user = actionSheetUser.value
  if (!user) return []
  const actions = [
    { name: '编辑', color: '#0d9488', action: 'edit' },
    { name: '重置密码', color: '#f59e0b', action: 'resetPwd' },
  ]
  if (user.status === 'active') {
    actions.push({ name: '禁用', color: '#ef4444', action: 'disable' })
  } else {
    actions.push({ name: '启用', color: '#10b981', action: 'enable' })
  }
  actions.push({ name: '删除', color: '#ef4444', action: 'delete' })
  return actions
})

function onLongPress(user: any) {
  actionSheetUser.value = user
  showActionSheet.value = true
}

function onActionSelect(action: any, index: number) {
  showActionSheet.value = false
  const user = actionSheetUser.value
  if (!user) return

  switch (action.action) {
    case 'edit':
      openEditDialog(user)
      break
    case 'resetPwd':
      openResetPwdDialog(user)
      break
    case 'enable':
      toggleStatus(user)
      break
    case 'disable':
      toggleStatus(user)
      break
    case 'delete':
      deleteUser(user)
      break
  }
}

onMounted(() => fetchUsers())
</script>

<template>
  <div class="page">

    <!-- ═══════ HERO ═══════ -->
    <div class="hero">
      <div class="hero-bg">
        <div class="hero-orb hero-orb--1" />
        <div class="hero-orb hero-orb--2" />
      </div>

      <div class="hero-content">
        <div class="hero-greeting">
          <span class="hero-label">用户管理</span>
          <span class="hero-badge">
            <van-icon name="manager-o" size="14" />
          </span>
        </div>

        <div v-if="!loading" class="hero-metrics">
          <div class="hero-metric hero-metric--main">
            <div class="hero-metric__value">{{ total }}</div>
            <div class="hero-metric__label">全部用户</div>
          </div>
          <div class="hero-metric">
            <div class="hero-metric__value">{{ users.filter(u => u.role === 'super_admin').length }}</div>
            <div class="hero-metric__label">超管</div>
          </div>
          <div class="hero-metric">
            <div class="hero-metric__value">{{ users.filter(u => u.status === 'active').length }}</div>
            <div class="hero-metric__label">启用</div>
          </div>
        </div>

        <div v-else class="hero-skeleton">
          <div class="skeleton-bar skeleton-bar--lg" />
          <div class="skeleton-bar skeleton-bar--md" />
        </div>
      </div>
    </div>

    <!-- ═══════ SEARCH ═══════ -->
    <div class="search-wrap">
      <van-search
        v-model="search"
        placeholder="搜索用户名或邮箱..."
        shape="round"
        @search="onSearch"
        @clear="onSearch"
      />
    </div>

    <!-- ═══════ FILTERS ═══════ -->
    <div class="filters">
      <div class="filter-row">
        <div class="filter-chips">
          <div
            v-for="opt in roleOptions"
            :key="opt.value"
            class="chip"
            :class="{ 'chip--active': roleFilter === opt.value }"
            @click="roleFilter = opt.value; onFilterChange()"
          >
            {{ opt.label }}
          </div>
        </div>
      </div>
      <div class="filter-row">
        <div class="filter-chips">
          <div
            v-for="opt in statusOptions"
            :key="opt.value"
            class="chip chip--status"
            :class="{ 'chip--active-status': statusFilter === opt.value }"
            @click="statusFilter = opt.value; onFilterChange()"
          >
            {{ opt.label }}
          </div>
        </div>
      </div>
    </div>

    <!-- ═══════ COUNT BAR ═══════ -->
    <div v-if="total > 0" class="count-bar">
      <span class="count-text">共 <b>{{ total }}</b> 个用户</span>
    </div>

    <!-- ═══════ USER CARDS ═══════ -->
    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <van-list v-model:loading="loading" :finished="finished" finished-text="" @load="onLoad">
        <div class="card-list">
          <van-swipe-cell
            v-for="(item, idx) in users"
            :key="item.id"
          >
            <div
              class="user-card"
              :style="{ animationDelay: `${Math.min(idx, 10) * 0.04}s` }"
              @longpress="onLongPress(item)"
            >
              <!-- Top: name + badges -->
              <div class="user-card__top">
                <h4 class="user-card__name">{{ displayName(item) }}</h4>
                <div class="user-card__badges">
                  <span
                    class="badge"
                    :style="{ color: getRoleConf(item.role).color, background: getRoleConf(item.role).bg }"
                  >
                    {{ roleLabel[item.role] || item.role }}
                  </span>
                  <span
                    class="badge"
                    :style="{ color: getStatusConf(item.status).color, background: getStatusConf(item.status).bg }"
                  >
                    {{ getStatusConf(item.status).label }}
                  </span>
                </div>
              </div>

              <!-- Info lines -->
              <div class="user-card__info">
                <div class="info-line">
                  <van-icon name="user-o" size="13" color="#94a3b8" />
                  <span class="info-text info-text--mono">{{ item.username }}</span>
                </div>
                <div v-if="item.email" class="info-line">
                  <van-icon name="envelop-o" size="13" color="#94a3b8" />
                  <span class="info-text info-text--muted">{{ item.email }}</span>
                </div>
              </div>

              <!-- Bottom: last login -->
              <div class="user-card__bottom">
                <div class="login-info">
                  <span v-if="item.last_login_at" class="login-text">
                    <van-icon name="clock-o" size="11" />
                    {{ formatTime(item.last_login_at) }}
                  </span>
                  <span v-else class="login-text login-text--never">从未登录</span>
                  <span v-if="item.last_login_ip" class="login-ip">{{ item.last_login_ip }}</span>
                </div>
              </div>
            </div>

            <!-- Swipe right: enable/disable -->
            <template #right>
              <van-button
                v-if="item.status === 'active'"
                square
                type="danger"
                text="禁用"
                class="swipe-btn"
                @click="toggleStatus(item)"
              />
              <van-button
                v-else
                square
                type="success"
                text="启用"
                class="swipe-btn"
                @click="toggleStatus(item)"
              />
            </template>

            <!-- Swipe left: delete -->
            <template #left>
              <van-button
                square
                type="danger"
                text="删除"
                class="swipe-btn"
                @click="deleteUser(item)"
              />
            </template>
          </van-swipe-cell>
        </div>

        <!-- Empty state -->
        <div v-if="!loading && !users.length" class="empty-state">
          <van-icon name="friends-o" size="40" color="#cbd5e1" />
          <span class="empty-text">暂无用户数据</span>
        </div>
      </van-list>
    </van-pull-refresh>

    <!-- ═══════ FAB ═══════ -->
    <div class="fab" @click="showCreateDialog = true">
      <van-icon name="plus" size="24" color="#fff" />
    </div>

    <!-- ═══════ CREATE DIALOG ═══════ -->
    <van-dialog
      v-model:show="showCreateDialog"
      title="新建用户"
      show-cancel-button
      :before-close="(_action: string, done: () => void) => { if (_action === 'confirm') { createUser().then(done) } else { done() } }"
    >
      <div class="dialog-form">
        <van-field
          v-model="createForm.username"
          label="用户名"
          placeholder="3-50 个字符"
          :maxlength="50"
          required
        />
        <van-field
          v-model="createForm.password"
          label="密码"
          type="password"
          placeholder="8-64 个字符"
          :maxlength="64"
          required
        />
        <van-field
          v-model="createForm.email"
          label="邮箱"
          placeholder="可选"
          type="email"
        />
        <div class="dialog-field-label">角色</div>
        <van-radio-group v-model="createForm.role" direction="horizontal" class="dialog-radio-group">
          <van-radio name="super_admin">超级管理员</van-radio>
          <van-radio name="admin">管理员</van-radio>
        </van-radio-group>
      </div>
    </van-dialog>

    <!-- ═══════ EDIT DIALOG ═══════ -->
    <van-dialog
      v-model:show="showEditDialog"
      title="编辑用户"
      show-cancel-button
      :before-close="(_action: string, done: () => void) => { if (_action === 'confirm') { updateUser().then(done) } else { done() } }"
    >
      <div class="dialog-form">
        <van-field
          v-model="editForm.display_name"
          label="显示名"
          placeholder="可选"
        />
        <van-field
          v-model="editForm.email"
          label="邮箱"
          placeholder="可选"
          type="email"
        />
        <div class="dialog-field-label">角色</div>
        <van-radio-group v-model="editForm.role" direction="horizontal" class="dialog-radio-group">
          <van-radio name="super_admin">超级管理员</van-radio>
          <van-radio name="admin">管理员</van-radio>
        </van-radio-group>
      </div>
    </van-dialog>

    <!-- ═══════ RESET PASSWORD DIALOG ═══════ -->
    <van-dialog
      v-model:show="showResetPwdDialog"
      title="重置密码"
      show-cancel-button
      :before-close="(_action: string, done: () => void) => { if (_action === 'confirm') { resetPassword().then(done) } else { done() } }"
    >
      <div class="dialog-form">
        <div class="reset-hint">
          为用户「{{ resetPwdUser ? displayName(resetPwdUser) : '' }}」设置新密码
        </div>
        <van-field
          v-model="resetPwdForm.new_password"
          label="新密码"
          type="password"
          placeholder="8-64 个字符"
          :maxlength="64"
          required
        />
      </div>
    </van-dialog>

    <!-- ═══════ ACTION SHEET (long-press) ═══════ -->
    <van-action-sheet
      v-model:show="showActionSheet"
      :actions="actionSheetActions"
      cancel-text="取消"
      close-on-click-action
      @select="onActionSelect"
    />
  </div>
</template>

<style scoped>
.page {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(80px + env(safe-area-inset-bottom, 0px));
}

/* ═══════════════════════════════════════
   HERO — Slate Theme
   ═══════════════════════════════════════ */
.hero {
  position: relative;
  overflow: hidden;
  padding-bottom: 24px;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #1e293b 0%, #475569 35%, #64748b 65%, #475569 100%);
}

.hero-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(60px);
  pointer-events: none;
}

.hero-orb--1 {
  width: 180px;
  height: 180px;
  background: rgba(148, 163, 184, 0.25);
  top: -30px;
  right: -20px;
}

.hero-orb--2 {
  width: 140px;
  height: 140px;
  background: rgba(100, 116, 139, 0.3);
  bottom: 10px;
  left: -30px;
}

.hero-content {
  position: relative;
  z-index: 1;
  padding: 24px 20px 0;
}

.hero-greeting {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 18px;
}

.hero-label {
  font-size: 20px;
  font-weight: 700;
  color: #fff;
  letter-spacing: -0.02em;
}

.hero-badge {
  width: 32px;
  height: 32px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.15);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.hero-metrics {
  display: flex;
  gap: 10px;
}

.hero-metric {
  flex: 1;
  background: rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 14px;
  padding: 14px;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.hero-metric--main {
  flex: 1.2;
  background: rgba(255, 255, 255, 0.14);
}

.hero-metric__value {
  font-size: 24px;
  font-weight: 700;
  color: #fff;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.03em;
  line-height: 1.2;
}

.hero-metric--main .hero-metric__value {
  font-size: 28px;
}

.hero-metric__label {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.55);
  margin-top: 4px;
  font-weight: 500;
}

/* Hero Skeleton */
.hero-skeleton {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 8px 0;
}

.skeleton-bar {
  height: 12px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.12);
}

.skeleton-bar--lg {
  width: 60%;
  height: 24px;
}

.skeleton-bar--md {
  width: 40%;
}

/* ═══════════════════════════════════════
   SEARCH
   ═══════════════════════════════════════ */
.search-wrap {
  padding: 4px 0 0;
}
.search-wrap :deep(.van-search) {
  padding: 8px 12px;
}

/* ═══════════════════════════════════════
   FILTERS
   ═══════════════════════════════════════ */
.filters {
  padding: 4px 16px 0;
  animation: fadeSlideUp 0.4s 0.1s both;
}

.filter-row {
  margin-bottom: 8px;
}

.filter-chips {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 4px;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.filter-chips::-webkit-scrollbar {
  display: none;
}

.chip {
  flex-shrink: 0;
  padding: 6px 14px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 600;
  color: var(--ta-text-secondary, #475569);
  background: var(--ta-bg-card, #fff);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
  cursor: pointer;
  transition: all 0.2s;
  -webkit-tap-highlight-color: transparent;
  white-space: nowrap;
}

.chip--active {
  background: #0d9488;
  color: #fff;
  box-shadow: 0 2px 8px rgba(13, 148, 136, 0.3);
}

.chip--status.chip--active-status {
  background: #64748b;
  color: #fff;
  box-shadow: 0 2px 8px rgba(100, 116, 139, 0.3);
}

.chip:active {
  transform: scale(0.96);
}

/* ═══════════════════════════════════════
   COUNT BAR
   ═══════════════════════════════════════ */
.count-bar {
  padding: 4px 16px 6px;
}

.count-text {
  font-size: 12px;
  color: #94a3b8;
}

.count-text b {
  color: #475569;
  font-weight: 600;
}

/* ═══════════════════════════════════════
   CARD LIST
   ═══════════════════════════════════════ */
.card-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 8px 12px 0;
}

.user-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 16px;
  padding: 14px 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  animation: fadeSlideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
  transition: transform 0.15s;
}

.user-card:active {
  transform: scale(0.99);
}

/* Card Top */
.user-card__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.user-card__name {
  font-size: 15px;
  font-weight: 700;
  color: var(--ta-text-primary, #0f172a);
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.user-card__badges {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.badge {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 8px;
  line-height: 1.5;
  white-space: nowrap;
}

/* Card Info */
.user-card__info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 8px;
}

.info-line {
  display: flex;
  align-items: center;
  gap: 6px;
}

.info-text {
  font-size: 12px;
  color: var(--ta-text-secondary, #64748b);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.info-text--mono {
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace;
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
}

.info-text--muted {
  color: var(--ta-text-tertiary, #94a3b8);
  font-size: 11px;
}

/* Card Bottom */
.user-card__bottom {
  padding-top: 8px;
  border-top: 1px solid var(--ta-bg-secondary, #f1f5f9);
}

.login-info {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.login-text {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: #94a3b8;
}

.login-text--never {
  color: #cbd5e1;
  font-style: italic;
}

.login-ip {
  font-size: 10px;
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace;
  color: #cbd5e1;
}

/* Swipe cell overrides */
.swipe-btn {
  height: 100% !important;
}

/* ═══════════════════════════════════════
   EMPTY STATE
   ═══════════════════════════════════════ */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 56px 0 24px;
  animation: fadeSlideUp 0.4s 0.2s both;
}

.empty-text {
  font-size: 13px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
}

/* ═══════════════════════════════════════
   FAB
   ═══════════════════════════════════════ */
.fab {
  position: fixed;
  right: 20px;
  bottom: calc(24px + env(safe-area-inset-bottom, 0px));
  width: 52px;
  height: 52px;
  border-radius: 16px;
  background: linear-gradient(135deg, #0d9488, #0f766e);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 14px rgba(13, 148, 136, 0.4), 0 2px 6px rgba(0, 0, 0, 0.1);
  cursor: pointer;
  z-index: 100;
  transition: transform 0.2s, box-shadow 0.2s;
  -webkit-tap-highlight-color: transparent;
}

.fab:active {
  transform: scale(0.92);
  box-shadow: 0 2px 8px rgba(13, 148, 136, 0.3);
}

/* ═══════════════════════════════════════
   DIALOG FORM
   ═══════════════════════════════════════ */
.dialog-form {
  padding: 12px 16px 4px;
}

.dialog-form :deep(.van-field) {
  padding: 10px 0;
}

.dialog-form :deep(.van-field__label) {
  width: 4em;
  color: #475569;
  font-weight: 500;
}

.dialog-field-label {
  font-size: 13px;
  font-weight: 500;
  color: #475569;
  padding: 10px 0 6px;
}

.dialog-radio-group {
  display: flex;
  gap: 16px;
  padding-bottom: 4px;
}

.dialog-radio-group :deep(.van-radio__label) {
  font-size: 13px;
  color: #334155;
}

.reset-hint {
  font-size: 12px;
  color: #94a3b8;
  margin-bottom: 8px;
  line-height: 1.5;
}

/* ═══════════════════════════════════════
   ANIMATIONS
   ═══════════════════════════════════════ */
@keyframes fadeSlideUp {
  from {
    opacity: 0;
    transform: translateY(14px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
