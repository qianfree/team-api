<script setup lang="ts">
import { ref, onMounted, computed, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { showToast, showConfirmDialog } from 'vant'
import request from '@/utils/request'
import StatusTag from '@/components/StatusTag.vue'

const router = useRouter()
const route = useRoute()

// ── State ──
const ticket = ref<any>(null)
const replies = ref<any[]>([])
const loading = ref(false)
const replyContent = ref('')
const sendingReply = ref(false)
const showAssignPicker = ref(false)
const adminOptions = [{ text: '管理员A', value: 'a' }, { text: '管理员B', value: 'b' }]
const selectedAdmin = ref('')

// ── Config Maps ──
const categoryConfig: Record<string, { label: string; color: string; bg: string }> = {
  billing: { label: '账单', color: '#d97706', bg: 'rgba(217,119,6,0.1)' },
  technical: { label: '技术', color: '#2563eb', bg: 'rgba(37,99,235,0.1)' },
  feature_request: { label: '功能', color: '#7c3aed', bg: 'rgba(124,58,237,0.1)' },
  other: { label: '其他', color: '#64748b', bg: 'rgba(100,116,139,0.1)' },
}

const urgencyConfig: Record<string, { label: string; color: string; bg: string }> = {
  low: { label: '低', color: '#10b981', bg: 'rgba(16,185,129,0.1)' },
  medium: { label: '中', color: '#3b82f6', bg: 'rgba(59,130,246,0.1)' },
  high: { label: '高', color: '#f59e0b', bg: 'rgba(245,158,11,0.1)' },
  critical: { label: '紧急', color: '#ef4444', bg: 'rgba(239,68,68,0.1)' },
}

const ticketStatusConfig: Record<string, { label: string; color: string; bg: string }> = {
  pending: { label: '待处理', color: '#f59e0b', bg: 'rgba(245,158,11,0.1)' },
  processing: { label: '处理中', color: '#3b82f6', bg: 'rgba(59,130,246,0.1)' },
  replied: { label: '已回复', color: '#0d9488', bg: 'rgba(13,148,136,0.1)' },
  closed: { label: '已关闭', color: '#94a3b8', bg: 'rgba(148,163,184,0.1)' },
  reopened: { label: '已重开', color: '#8b5cf6', bg: 'rgba(139,92,246,0.1)' },
}

function getCategoryInfo(cat: string) {
  return categoryConfig[cat] || { label: cat, color: '#64748b', bg: 'rgba(100,116,139,0.1)' }
}

function getUrgencyInfo(urg: string) {
  return urgencyConfig[urg] || { label: urg, color: '#64748b', bg: 'rgba(100,116,139,0.1)' }
}

function getStatusInfo(status: string) {
  return ticketStatusConfig[status] || { label: status, color: '#64748b', bg: 'rgba(100,116,139,0.1)' }
}

// ── Computed ──
const statusInfo = computed(() => {
  if (!ticket.value) return { label: '-', color: '#64748b', bg: 'rgba(100,116,139,0.1)' }
  return getStatusInfo(ticket.value.status)
})

const urgencyInfo = computed(() => {
  if (!ticket.value) return { label: '-', color: '#64748b', bg: 'rgba(100,116,139,0.1)' }
  return getUrgencyInfo(ticket.value.urgency)
})

const isOpen = computed(() => {
  if (!ticket.value) return false
  return ['pending', 'processing', 'replied', 'reopened'].includes(ticket.value.status)
})

// ── Helpers ──
function formatDateTime(dateStr: string | undefined): string {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  const pad = (v: number) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// ── Fetch ──
async function fetchTicket() {
  const id = route.params.id as string
  if (!id) return

  loading.value = true
  try {
    const { data: res } = await request.get(`/admin/tickets/${id}`)
    ticket.value = res.data
    replies.value = res.data?.replies || []
    await nextTick()
    scrollToBottom()
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
  }
}

function scrollToBottom() {
  const el = document.querySelector('.replies-scroll')
  if (el) {
    el.scrollTop = el.scrollHeight
  }
}

// ── Actions ──
async function sendReply() {
  if (!replyContent.value.trim()) {
    showToast('请输入回复内容')
    return
  }
  const id = route.params.id as string
  sendingReply.value = true
  try {
    await request.post(`/admin/tickets/${id}/reply`, {
      content: replyContent.value.trim(),
    })
    replyContent.value = ''
    showToast('回复成功')
    await fetchTicket()
  } catch {
    // handled by interceptor
  } finally {
    sendingReply.value = false
  }
}

async function assignAdmin() {
  if (!selectedAdmin.value) {
    showToast('请选择处理人')
    return
  }
  const id = route.params.id as string
  try {
    await request.put(`/admin/tickets/${id}/assign`, {
      admin_id: selectedAdmin.value,
    })
    showToast('分配成功')
    showAssignPicker.value = false
    await fetchTicket()
  } catch {
    // handled by interceptor
  }
}

function onAssignConfirm({ selectedOptions }: { selectedOptions: any[] }) {
  const selected = selectedOptions[0]
  selectedAdmin.value = selected?.text || ''
  if (selected?.value) {
    assignAdmin()
  }
}

async function closeTicket() {
  const id = route.params.id as string
  try {
    await showConfirmDialog({
      title: '关闭工单',
      message: '确定要关闭此工单吗？',
    })
    await request.put(`/admin/tickets/${id}/status`, { status: 'closed' })
    showToast('工单已关闭')
    await fetchTicket()
  } catch {
    // cancelled or error
  }
}

async function reopenTicket() {
  const id = route.params.id as string
  try {
    await showConfirmDialog({
      title: '重开工单',
      message: '确定要重新打开此工单吗？',
    })
    await request.put(`/admin/tickets/${id}/status`, { status: 'reopened' })
    showToast('工单已重开')
    await fetchTicket()
  } catch {
    // cancelled or error
  }
}

// ── Init ──
onMounted(() => {
  fetchTicket()
})
</script>

<template>
  <div class="detail-page">

    <!-- ═══════ HERO ═══════ -->
    <div class="hero">
      <div class="hero-bg">
        <div class="hero-orb hero-orb--1" />
        <div class="hero-orb hero-orb--2" />
      </div>
      <div class="hero-content">
        <div class="hero-nav" @click="router.back()">
          <van-icon name="arrow-left" size="18" color="#fff" />
          <span>工单详情</span>
        </div>

        <div v-if="ticket" class="hero-body">
          <div class="hero-title-wrap">
            <div class="hero-title">{{ ticket.title }}</div>
            <div class="hero-badges">
              <span
                class="hero-badge"
                :style="{
                  color: statusInfo.color,
                  background: statusInfo.bg,
                }"
              >
                {{ statusInfo.label }}
              </span>
              <span
                class="hero-badge"
                :style="{
                  color: urgencyInfo.color,
                  background: urgencyInfo.bg,
                }"
              >
                {{ urgencyInfo.label }}
              </span>
            </div>
          </div>
        </div>
        <div v-else class="hero-skeleton">
          <div class="skel-bar skel-bar--lg" />
          <div class="skel-bar skel-bar--sm" />
        </div>
      </div>
    </div>

    <!-- ═══════ LOADING ═══════ -->
    <div v-if="loading && !ticket" class="loading-wrap">
      <van-loading size="24" color="#f97316" vertical>加载中...</van-loading>
    </div>

    <!-- ═══════ CONTENT ═══════ -->
    <template v-if="ticket">
      <!-- Info Card -->
      <div class="info-section">
        <div class="info-section__title">工单信息</div>
        <div class="info-card">
          <div class="info-row info-row--border">
            <span class="info-row__label">租户</span>
            <span class="info-row__value">{{ ticket.tenant_name || '-' }}</span>
          </div>
          <div class="info-row info-row--border">
            <span class="info-row__label">用户</span>
            <span class="info-row__value">{{ ticket.user_display_name || '-' }}</span>
          </div>
          <div class="info-row info-row--border">
            <span class="info-row__label">分类</span>
            <span
              class="mini-badge"
              :style="{
                color: getCategoryInfo(ticket.category).color,
                background: getCategoryInfo(ticket.category).bg,
              }"
            >
              {{ getCategoryInfo(ticket.category).label }}
            </span>
          </div>
          <div class="info-row info-row--border">
            <span class="info-row__label">处理人</span>
            <span class="info-row__value">{{ ticket.assigned_admin_name || '未分配' }}</span>
          </div>
          <div class="info-row">
            <span class="info-row__label">创建时间</span>
            <span class="info-row__value">{{ formatDateTime(ticket.created_at) }}</span>
          </div>
        </div>
      </div>

      <!-- Description Card -->
      <div v-if="ticket.description" class="info-section">
        <div class="info-section__title">问题描述</div>
        <div class="desc-card">
          <p class="desc-text">{{ ticket.description }}</p>
        </div>
      </div>

      <!-- ═══════ REPLY TIMELINE ═══════ -->
      <div class="info-section">
        <div class="info-section__title">
          对话记录
          <span v-if="replies.length" class="reply-count">{{ replies.length }}</span>
        </div>

        <!-- Replies Scroll Area -->
        <div class="replies-scroll" :class="{ 'replies-scroll--with-bar': isOpen }">
          <div v-if="replies.length" class="replies-list">
            <div
              v-for="reply in replies"
              :key="reply.id"
              class="reply-row"
              :class="reply.user_type === 'admin' ? 'reply-row--admin' : 'reply-row--tenant'"
            >
              <!-- Tenant bubble: left -->
              <div v-if="reply.user_type !== 'admin'" class="bubble bubble--tenant">
                <div class="bubble__name">{{ reply.user_name }}</div>
                <div class="bubble__content">{{ reply.content }}</div>
                <div class="bubble__time">{{ formatDateTime(reply.created_at) }}</div>
              </div>

              <!-- Spacer -->
              <div class="bubble-spacer" />

              <!-- Admin bubble: right -->
              <div v-if="reply.user_type === 'admin'" class="bubble bubble--admin">
                <div class="bubble__name">{{ reply.user_name }}</div>
                <div class="bubble__content">{{ reply.content }}</div>
                <div class="bubble__time">{{ formatDateTime(reply.created_at) }}</div>
              </div>
            </div>
          </div>

          <!-- Empty replies -->
          <div v-else class="replies-empty">
            <van-icon name="chat-o" size="32" color="#cbd5e1" />
            <span>暂无对话记录</span>
          </div>
        </div>
      </div>

      <!-- Bottom spacer for action bar -->
      <div v-if="isOpen" class="bottom-spacer" />
    </template>

    <!-- ═══════ EMPTY STATE ═══════ -->
    <div v-if="!loading && !ticket" class="empty-state">
      <van-icon name="orders-o" size="42" color="#cbd5e1" />
      <span class="empty-text">工单不存在或已被删除</span>
    </div>

    <!-- ═══════ BOTTOM ACTION BAR ═══════ -->
    <div v-if="isOpen && ticket" class="action-bar">
      <div class="action-bar__input-row">
        <van-field
          v-model="replyContent"
          placeholder="输入回复内容..."
          :border="false"
          class="reply-input"
          type="textarea"
          rows="1"
          autosize
          maxlength="1000"
        />
        <van-button
          type="primary"
          size="small"
          round
          class="send-btn"
          :loading="sendingReply"
          :disabled="!replyContent.trim()"
          @click="sendReply"
        >
          发送
        </van-button>
      </div>
      <div class="action-bar__btn-row">
        <van-button
          size="small"
          round
          plain
          class="action-btn"
          icon="manager-o"
          @click="showAssignPicker = true"
        >
          分配处理人
        </van-button>
        <van-button
          v-if="isOpen && ticket.status !== 'closed'"
          size="small"
          round
          plain
          class="action-btn action-btn--danger"
          icon="cross"
          @click="closeTicket"
        >
          关闭工单
        </van-button>
        <van-button
          v-if="ticket.status === 'closed'"
          size="small"
          round
          plain
          class="action-btn action-btn--reopen"
          icon="replay"
          @click="reopenTicket"
        >
          重开工单
        </van-button>
      </div>
    </div>

    <!-- ═══════ ASSIGN PICKER ═══════ -->
    <van-popup v-model:show="showAssignPicker" position="bottom" round>
      <van-picker
        :columns="adminOptions"
        title="选择处理人"
        @confirm="onAssignConfirm"
        @cancel="showAssignPicker = false"
      />
    </van-popup>
  </div>
</template>

<style scoped>
.detail-page {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(24px + env(safe-area-inset-bottom, 0px));
}

/* ═══════════════════════════════════════
   HERO — Orange Theme
   ═══════════════════════════════════════ */
.hero {
  position: relative;
  overflow: hidden;
  border-radius: 0 0 28px 28px;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #c2410c 0%, #ea580c 30%, #f97316 55%, #fb923c 80%, #ea580c 100%);
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
  background: rgba(251, 146, 60, 0.35);
  top: -30px;
  right: -20px;
}
.hero-orb--2 {
  width: 140px;
  height: 140px;
  background: rgba(234, 88, 12, 0.25);
  bottom: 0;
  left: -20px;
}

.hero-content {
  position: relative;
  z-index: 1;
  padding: 20px 20px 24px;
}

.hero-nav {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 20px;
  cursor: pointer;
  -webkit-tap-highlight-color: transparent;
}
.hero-nav span {
  font-size: 15px;
  font-weight: 600;
  color: #fff;
}

.hero-body {
  animation: fadeSlideUp 0.4s both;
}

.hero-title-wrap {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.hero-title {
  font-size: 17px;
  font-weight: 700;
  color: #fff;
  line-height: 1.35;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.hero-badges {
  display: flex;
  align-items: center;
  gap: 6px;
}

.hero-badge {
  display: inline-flex;
  align-items: center;
  padding: 3px 10px;
  border-radius: 10px;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}

/* ── Hero Skeleton ── */
.hero-skeleton {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-top: 4px;
}

.skel-bar {
  height: 12px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.12);
}

.skel-bar--lg {
  width: 70%;
  height: 20px;
}

.skel-bar--sm {
  width: 30%;
  height: 10px;
}

/* ═══════════════════════════════════════
   INFO SECTION
   ═══════════════════════════════════════ */
.info-section {
  padding: 16px 12px 0;
  animation: fadeSlideUp 0.4s 0.06s both;
}

.info-section__title {
  font-size: 13px;
  font-weight: 600;
  color: var(--ta-text-secondary, #475569);
  margin-bottom: 8px;
  padding-left: 4px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.reply-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 9px;
  background: var(--ta-primary, #0d9488);
  color: #fff;
  font-size: 10px;
  font-weight: 700;
}

.info-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 4px 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
}

.info-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 11px 0;
}

.info-row--border {
  border-bottom: 1px solid var(--ta-border-light, #f1f5f9);
}

.info-row__label {
  font-size: 13px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
  flex-shrink: 0;
  white-space: nowrap;
}

.info-row__value {
  font-size: 13px;
  font-weight: 500;
  color: var(--ta-text-primary, #0f172a);
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mini-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}

/* ── Description Card ── */
.desc-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
}

.desc-text {
  margin: 0;
  font-size: 14px;
  color: var(--ta-text-primary, #1e293b);
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
}

/* ═══════════════════════════════════════
   REPLIES — Chat Bubble Style
   ═══════════════════════════════════════ */
.replies-scroll {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  overflow: hidden;
}

.replies-scroll--with-bar {
  max-height: calc(100vh - 480px);
}

.replies-list {
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.reply-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.reply-row--admin {
  justify-content: flex-end;
}

.reply-row--tenant {
  justify-content: flex-start;
}

.bubble-spacer {
  flex: 0;
}

/* ── Bubble: Tenant (left) ── */
.bubble--tenant {
  max-width: 78%;
  background: #f8fafc;
  border: 1px solid #f1f5f9;
  border-radius: 16px 16px 16px 4px;
  padding: 10px 12px;
}

.bubble--tenant .bubble__name {
  color: #0f172a;
  font-weight: 600;
}

.bubble--tenant .bubble__content {
  color: #334155;
}

/* ── Bubble: Admin (right) ── */
.bubble--admin {
  max-width: 78%;
  background: #f0fdfa;
  border: 1px solid rgba(13, 148, 136, 0.1);
  border-radius: 16px 16px 4px 16px;
  padding: 10px 12px;
}

.bubble--admin .bubble__name {
  color: #0d9488;
  font-weight: 600;
}

.bubble--admin .bubble__content {
  color: #115e59;
}

.bubble__name {
  font-size: 12px;
  margin-bottom: 4px;
}

.bubble__content {
  font-size: 14px;
  line-height: 1.55;
  word-break: break-word;
  white-space: pre-wrap;
}

.bubble__time {
  font-size: 10px;
  color: #94a3b8;
  margin-top: 4px;
  text-align: right;
}

/* ── Replies Empty ── */
.replies-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 32px 0;
  color: #94a3b8;
  font-size: 13px;
}

/* ═══════════════════════════════════════
   BOTTOM ACTION BAR
   ═══════════════════════════════════════ */
.bottom-spacer {
  height: 130px;
}

.action-bar {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 100;
  background: #fff;
  border-top: 1px solid #f1f5f9;
  padding: 10px 12px calc(10px + env(safe-area-inset-bottom, 0px));
  display: flex;
  flex-direction: column;
  gap: 8px;
  box-shadow: 0 -2px 12px rgba(0, 0, 0, 0.06);
}

.action-bar__input-row {
  display: flex;
  align-items: flex-end;
  gap: 8px;
}

.reply-input {
  flex: 1;
  background: #f8fafc;
  border-radius: 10px;
  padding: 0;
}

.reply-input :deep(.van-cell) {
  background: transparent;
  padding: 8px 12px;
  border-radius: 10px;
}

.reply-input :deep(.van-field__control) {
  font-size: 14px;
  line-height: 1.4;
}

.send-btn {
  flex-shrink: 0;
  height: 36px;
  padding: 0 16px;
}

.action-bar__btn-row {
  display: flex;
  gap: 8px;
}

.action-btn {
  flex: 1;
  font-size: 12px !important;
}

.action-btn--danger {
  color: #ef4444 !important;
  border-color: #ef4444 !important;
}

.action-btn--reopen {
  color: #8b5cf6 !important;
  border-color: #8b5cf6 !important;
}

/* ═══════════════════════════════════════
   LOADING
   ═══════════════════════════════════════ */
.loading-wrap {
  display: flex;
  justify-content: center;
  padding: 48px 0;
}

/* ═══════════════════════════════════════
   EMPTY STATE
   ═══════════════════════════════════════ */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 64px 0 24px;
  animation: fadeSlideUp 0.4s both;
}

.empty-text {
  font-size: 14px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
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
