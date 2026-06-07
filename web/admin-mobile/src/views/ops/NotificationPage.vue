<script setup lang="ts">
import { ref } from 'vue'
import { showToast, showConfirmDialog } from 'vant'
import request from '@/utils/request'

// ═══════════════════════════════════════
// Tab state
// ═══════════════════════════════════════
const activeTab = ref(0)
const tabInited = ref([true, false, false])

function onTabChange({ name }: { name: number | string }) {
  const idx = Number(name)
  if (!tabInited.value[idx]) {
    tabInited.value[idx] = true
    if (idx === 0) fetchMessages()
    else if (idx === 1) fetchTemplates()
    else if (idx === 2) fetchAnnouncements()
  }
}

// ═══════════════════════════════════════
// Helpers
// ═══════════════════════════════════════
function formatTime(dateStr: string): string {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  const pad = (v: number) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const typeMap: Record<string, { label: string; color: string; bg: string }> = {
  billing:      { label: '账单', color: '#f59e0b', bg: 'rgba(245,158,11,0.1)' },
  system:       { label: '系统', color: '#3b82f6', bg: 'rgba(59,130,246,0.1)' },
  security:     { label: '安全', color: '#ef4444', bg: 'rgba(239,68,68,0.1)' },
  invitation:   { label: '邀请', color: '#8b5cf6', bg: 'rgba(139,92,246,0.1)' },
  announcement: { label: '公告', color: '#0d9488', bg: 'rgba(13,148,136,0.1)' },
}

const annStatusMap: Record<string, { label: string; color: string; bg: string }> = {
  draft:     { label: '草稿', color: '#64748b', bg: 'rgba(100,116,139,0.1)' },
  published: { label: '已发布', color: '#10b981', bg: 'rgba(16,185,129,0.1)' },
  archived:  { label: '已归档', color: '#94a3b8', bg: 'rgba(148,163,184,0.1)' },
}

const tplStatusMap: Record<string, { label: string; color: string; bg: string }> = {
  active:   { label: '启用', color: '#10b981', bg: 'rgba(16,185,129,0.1)' },
  inactive: { label: '停用', color: '#94a3b8', bg: 'rgba(148,163,184,0.1)' },
}

// ═══════════════════════════════════════
// Tab 1 — 消息
// ═══════════════════════════════════════
const msgLoading = ref(false)
const msgRefreshing = ref(false)
const msgFinished = ref(false)
const msgList = ref<any[]>([])
const msgPage = ref(1)
const msgTotal = ref(0)
const msgTypeFilter = ref('')

const msgTypeFilters = [
  { key: '', label: '全部' },
  { key: 'billing', label: '账单' },
  { key: 'system', label: '系统' },
  { key: 'security', label: '安全' },
  { key: 'invitation', label: '邀请' },
  { key: 'announcement', label: '公告' },
]

async function fetchMessages(append = false) {
  if (!append) {
    msgPage.value = 1
    msgFinished.value = false
  }
  msgLoading.value = true
  try {
    const params: any = { page: msgPage.value, page_size: 20 }
    if (msgTypeFilter.value) params.type = msgTypeFilter.value
    const { data: res } = await request.get('/admin/notification/messages', { params })
    const list = res.data?.list || []
    msgTotal.value = res.data?.total || 0
    msgList.value = append ? [...msgList.value, ...list] : list
    msgFinished.value = msgList.value.length >= msgTotal.value
  } catch {
    // handled by interceptor
  } finally {
    msgLoading.value = false
    msgRefreshing.value = false
  }
}

async function onMsgRefresh() {
  msgRefreshing.value = true
  await fetchMessages(false)
}

async function onMsgLoad() {
  if (msgFinished.value) return
  msgPage.value++
  await fetchMessages(true)
}

function setMsgType(key: string) {
  msgTypeFilter.value = key
  fetchMessages(false)
}

// Send / Broadcast dialogs
const showMsgAction = ref(false)
const showSendDialog = ref(false)
const showBroadcastDialog = ref(false)

const sendForm = ref({ tenant_id: '', user_id: '', title: '', content: '' })
const broadcastForm = ref({ tenant_id: '', title: '', content: '' })

function openMsgAction() {
  showMsgAction.value = true
}

function onMsgActionSelect(action: { name: string }) {
  if (action.name === '发送消息') {
    sendForm.value = { tenant_id: '', user_id: '', title: '', content: '' }
    showSendDialog.value = true
  } else if (action.name === '广播消息') {
    broadcastForm.value = { tenant_id: '', title: '', content: '' }
    showBroadcastDialog.value = true
  }
}

async function doSendMessage() {
  if (!sendForm.value.tenant_id || !sendForm.value.title || !sendForm.value.content) {
    showToast('请填写必填项')
    return
  }
  try {
    const body: any = {
      tenant_id: Number(sendForm.value.tenant_id),
      title: sendForm.value.title,
      content: sendForm.value.content,
    }
    if (sendForm.value.user_id) body.user_id = Number(sendForm.value.user_id)
    await request.post('/admin/notification/messages/send', body)
    showToast('发送成功')
    showSendDialog.value = false
    fetchMessages(false)
  } catch {
    // handled by interceptor
  }
}

async function doBroadcast() {
  if (!broadcastForm.value.title || !broadcastForm.value.content) {
    showToast('请填写必填项')
    return
  }
  try {
    const body: any = {
      title: broadcastForm.value.title,
      content: broadcastForm.value.content,
    }
    if (broadcastForm.value.tenant_id) body.tenant_id = Number(broadcastForm.value.tenant_id)
    await request.post('/admin/notification/messages/broadcast', body)
    showToast('广播成功')
    showBroadcastDialog.value = false
    fetchMessages(false)
  } catch {
    // handled by interceptor
  }
}

// ═══════════════════════════════════════
// Tab 2 — 模板
// ═══════════════════════════════════════
const tplLoading = ref(false)
const tplList = ref<any[]>([])
const tplTotal = ref(0)
const tplExpandedId = ref<number | null>(null)

async function fetchTemplates() {
  tplLoading.value = true
  try {
    const { data: res } = await request.get('/admin/notification/templates', {
      params: { page: 1, page_size: 50 },
    })
    tplList.value = res.data?.list || []
    tplTotal.value = res.data?.total || 0
  } catch {
    // handled by interceptor
  } finally {
    tplLoading.value = false
  }
}

function toggleTplExpand(id: number) {
  tplExpandedId.value = tplExpandedId.value === id ? null : id
}

// ═══════════════════════════════════════
// Tab 3 — 公告
// ═══════════════════════════════════════
const annLoading = ref(false)
const annRefreshing = ref(false)
const annFinished = ref(false)
const annList = ref<any[]>([])
const annPage = ref(1)
const annTotal = ref(0)
const annStatusFilter = ref('')

const annStatusFilters = [
  { key: '', label: '全部' },
  { key: 'draft', label: '草稿' },
  { key: 'published', label: '已发布' },
  { key: 'archived', label: '已归档' },
]

async function fetchAnnouncements(append = false) {
  if (!append) {
    annPage.value = 1
    annFinished.value = false
  }
  annLoading.value = true
  try {
    const params: any = { page: annPage.value, page_size: 20 }
    if (annStatusFilter.value) params.status = annStatusFilter.value
    const { data: res } = await request.get('/admin/announcements', { params })
    const list = res.data?.list || []
    annTotal.value = res.data?.total || 0
    annList.value = append ? [...annList.value, ...list] : list
    annFinished.value = annList.value.length >= annTotal.value
  } catch {
    // handled by interceptor
  } finally {
    annLoading.value = false
    annRefreshing.value = false
  }
}

async function onAnnRefresh() {
  annRefreshing.value = true
  await fetchAnnouncements(false)
}

async function onAnnLoad() {
  if (annFinished.value) return
  annPage.value++
  await fetchAnnouncements(true)
}

function setAnnStatus(key: string) {
  annStatusFilter.value = key
  fetchAnnouncements(false)
}

async function publishAnnouncement(item: any) {
  try {
    await showConfirmDialog({ title: '发布公告', message: `确定要发布「${item.title}」吗？` })
    await request.put(`/admin/announcements/${item.id}/publish`)
    showToast('已发布')
    fetchAnnouncements(false)
  } catch {
    // cancelled or error
  }
}

async function archiveAnnouncement(item: any) {
  try {
    await showConfirmDialog({ title: '归档公告', message: `确定要归档「${item.title}」吗？` })
    await request.put(`/admin/announcements/${item.id}/archive`)
    showToast('已归档')
    fetchAnnouncements(false)
  } catch {
    // cancelled or error
  }
}

// Create announcement popup
const showAnnCreate = ref(false)
const annForm = ref({
  title: '',
  type: '',
  content: '',
  is_pinned: false,
  effective_at: '',
  expires_at: '',
})

function openAnnCreate() {
  annForm.value = { title: '', type: '', content: '', is_pinned: false, effective_at: '', expires_at: '' }
  showAnnCreate.value = true
}

async function doCreateAnnouncement() {
  if (!annForm.value.title || !annForm.value.content) {
    showToast('请填写标题和内容')
    return
  }
  try {
    const body: any = {
      title: annForm.value.title,
      content: annForm.value.content,
      is_pinned: annForm.value.is_pinned,
    }
    if (annForm.value.type) body.type = annForm.value.type
    if (annForm.value.effective_at) body.effective_at = annForm.value.effective_at
    if (annForm.value.expires_at) body.expires_at = annForm.value.expires_at
    await request.post('/admin/announcements', body)
    showToast('创建成功')
    showAnnCreate.value = false
    fetchAnnouncements(false)
  } catch {
    // handled by interceptor
  }
}

// Init
fetchMessages()
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
        <div class="hero-icon-row">
          <div class="hero-icon-wrap">
            <van-icon name="bell" size="22" color="#fff" />
          </div>
          <span class="hero-title">通知消息</span>
        </div>
        <span class="hero-subtitle">消息推送 / 模板管理 / 公告发布</span>
      </div>
    </div>

    <!-- ═══════ TABS ═══════ -->
    <div class="tabs-wrap">
      <van-tabs
        v-model:active="activeTab"
        lazy-render
        animated
        color="#0d9488"
        title-active-color="#0d9488"
        title-inactive-color="#94a3b8"
        line-width="28"
        line-height="3"
        @change="onTabChange"
      >
        <!-- ─── Tab 1: 消息 ─── -->
        <van-tab title="消息">
          <!-- Type filter chips -->
          <div class="chip-scroll">
            <div
              v-for="f in msgTypeFilters"
              :key="f.key"
              class="chip"
              :class="{ 'chip--active': msgTypeFilter === f.key }"
              @click="setMsgType(f.key)"
            >
              {{ f.label }}
            </div>
          </div>

          <!-- Count -->
          <div v-if="msgTotal > 0" class="count-bar">
            <span class="count-text">共 <b>{{ msgTotal }}</b> 条消息</span>
          </div>

          <!-- List -->
          <van-pull-refresh v-model="msgRefreshing" @refresh="onMsgRefresh">
            <van-list v-model:loading="msgLoading" :finished="msgFinished" finished-text="" @load="onMsgLoad">
              <div class="card-list">
                <div
                  v-for="(item, idx) in msgList"
                  :key="item.id"
                  class="msg-card"
                  :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
                >
                  <!-- Unread dot + Title -->
                  <div class="msg-card__head">
                    <div class="msg-card__title-row">
                      <span v-if="!item.is_read" class="msg-card__unread" />
                      <span class="msg-card__title">{{ item.title }}</span>
                    </div>
                    <div class="msg-card__badges">
                      <span
                        v-if="item.type"
                        class="badge"
                        :style="{ color: typeMap[item.type]?.color || '#64748b', background: typeMap[item.type]?.bg || 'rgba(100,116,139,0.1)' }"
                      >
                        {{ typeMap[item.type]?.label || item.type }}
                      </span>
                      <span v-if="item.is_broadcast" class="badge badge--broadcast">广播</span>
                    </div>
                  </div>

                  <!-- Content preview -->
                  <div class="msg-card__content">{{ item.content }}</div>

                  <!-- Tenant + time -->
                  <div class="msg-card__footer">
                    <div class="msg-card__tenant">
                      <van-icon name="friends-o" size="12" />
                      <span>{{ item.tenant_name || '全部租户' }}</span>
                    </div>
                    <span class="msg-card__time">{{ formatTime(item.created_at) }}</span>
                  </div>
                </div>
              </div>

              <!-- Empty -->
              <div v-if="!msgLoading && !msgList.length" class="empty-state">
                <van-icon name="chat-o" size="42" color="#cbd5e1" />
                <span class="empty-text">暂无消息</span>
                <span class="empty-hint">点击右下角按钮发送消息</span>
              </div>
            </van-list>
          </van-pull-refresh>

          <!-- FAB -->
          <div class="fab" @click="openMsgAction">
            <van-icon name="plus" size="22" color="#fff" />
          </div>

          <!-- Action Sheet -->
          <van-action-sheet
            v-model:show="showMsgAction"
            :actions="[
              { name: '发送消息' },
              { name: '广播消息' },
            ]"
            cancel-text="取消"
            @select="onMsgActionSelect"
          />

          <!-- Send Dialog -->
          <van-dialog
            v-model:show="showSendDialog"
            title="发送消息"
            show-cancel-button
            @confirm="doSendMessage"
          >
            <div class="dialog-form">
              <div class="form-field">
                <label class="form-label">租户 ID <span class="form-required">*</span></label>
                <van-field v-model="sendForm.tenant_id" type="digit" placeholder="输入租户 ID" />
              </div>
              <div class="form-field">
                <label class="form-label">用户 ID</label>
                <van-field v-model="sendForm.user_id" type="digit" placeholder="可选，留空则发给全租户" />
              </div>
              <div class="form-field">
                <label class="form-label">标题 <span class="form-required">*</span></label>
                <van-field v-model="sendForm.title" placeholder="消息标题" />
              </div>
              <div class="form-field">
                <label class="form-label">内容 <span class="form-required">*</span></label>
                <van-field v-model="sendForm.content" type="textarea" rows="3" placeholder="消息内容" />
              </div>
            </div>
          </van-dialog>

          <!-- Broadcast Dialog -->
          <van-dialog
            v-model:show="showBroadcastDialog"
            title="广播消息"
            show-cancel-button
            @confirm="doBroadcast"
          >
            <div class="dialog-form">
              <div class="form-field">
                <label class="form-label">租户 ID</label>
                <van-field v-model="broadcastForm.tenant_id" type="digit" placeholder="留空则广播全部租户" />
              </div>
              <div class="form-field">
                <label class="form-label">标题 <span class="form-required">*</span></label>
                <van-field v-model="broadcastForm.title" placeholder="广播标题" />
              </div>
              <div class="form-field">
                <label class="form-label">内容 <span class="form-required">*</span></label>
                <van-field v-model="broadcastForm.content" type="textarea" rows="3" placeholder="广播内容" />
              </div>
            </div>
          </van-dialog>
        </van-tab>

        <!-- ─── Tab 2: 模板 ─── -->
        <van-tab title="模板">
          <div v-if="tplLoading" class="loading-state">
            <van-loading size="24" color="#0d9488" />
            <span>加载中...</span>
          </div>

          <div v-else-if="tplList.length" class="card-list">
            <div
              v-for="(item, idx) in tplList"
              :key="item.id"
              class="tpl-card"
              :class="{ 'tpl-card--expanded': tplExpandedId === item.id }"
              :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
              @click="toggleTplExpand(item.id)"
            >
              <!-- Header -->
              <div class="tpl-card__head">
                <div class="tpl-card__code-row">
                  <span class="tpl-card__code">{{ item.code }}</span>
                  <span
                    v-if="item.channel"
                    class="badge"
                    :style="{ color: '#3b82f6', background: 'rgba(59,130,246,0.1)' }"
                  >
                    {{ item.channel }}
                  </span>
                </div>
                <span
                  class="badge"
                  :style="{ color: tplStatusMap[item.status]?.color || '#64748b', background: tplStatusMap[item.status]?.bg || 'rgba(100,116,139,0.1)' }"
                >
                  {{ tplStatusMap[item.status]?.label || item.status }}
                </span>
              </div>

              <!-- Subject -->
              <div class="tpl-card__subject">{{ item.subject }}</div>

              <!-- Expanded body -->
              <div v-if="tplExpandedId === item.id && item.body_template" class="tpl-card__body">
                <div class="tpl-card__body-label">模板内容</div>
                <div class="tpl-card__body-text">{{ item.body_template }}</div>
                <div v-if="item.variables" class="tpl-card__vars">
                  <span class="tpl-card__vars-label">变量：</span>
                  <span class="tpl-card__vars-text">{{ item.variables }}</span>
                </div>
              </div>
            </div>
          </div>

          <div v-else class="empty-state">
            <van-icon name="notes-o" size="42" color="#cbd5e1" />
            <span class="empty-text">暂无模板</span>
          </div>
        </van-tab>

        <!-- ─── Tab 3: 公告 ─── -->
        <van-tab title="公告">
          <!-- Status filter -->
          <div class="chip-scroll">
            <div
              v-for="f in annStatusFilters"
              :key="f.key"
              class="chip"
              :class="{ 'chip--active': annStatusFilter === f.key }"
              @click="setAnnStatus(f.key)"
            >
              {{ f.label }}
            </div>
          </div>

          <div v-if="annTotal > 0" class="count-bar">
            <span class="count-text">共 <b>{{ annTotal }}</b> 条公告</span>
          </div>

          <van-pull-refresh v-model="annRefreshing" @refresh="onAnnRefresh">
            <van-list v-model:loading="annLoading" :finished="annFinished" finished-text="" @load="onAnnLoad">
              <div class="card-list">
                <van-swipe-cell v-for="(item, idx) in annList" :key="item.id">
                  <div
                    class="ann-card"
                    :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
                  >
                    <!-- Title row -->
                    <div class="ann-card__head">
                      <div class="ann-card__title-row">
                        <van-icon v-if="item.is_pinned" name="star" size="14" color="#f59e0b" class="ann-card__pin" />
                        <span class="ann-card__title">{{ item.title }}</span>
                      </div>
                      <div class="ann-card__badges">
                        <span
                          v-if="item.type"
                          class="badge"
                          :style="{ color: '#8b5cf6', background: 'rgba(139,92,246,0.1)' }"
                        >
                          {{ item.type }}
                        </span>
                        <span
                          class="badge"
                          :style="{ color: annStatusMap[item.status]?.color || '#64748b', background: annStatusMap[item.status]?.bg || 'rgba(100,116,139,0.1)' }"
                        >
                          {{ annStatusMap[item.status]?.label || item.status }}
                        </span>
                      </div>
                    </div>

                    <!-- Content preview -->
                    <div class="ann-card__content">{{ item.content }}</div>

                    <!-- Time info -->
                    <div class="ann-card__footer">
                      <span v-if="item.effective_at" class="ann-card__time">
                        生效：{{ formatTime(item.effective_at) }}
                      </span>
                      <span class="ann-card__time ann-card__time--created">
                        创建：{{ formatTime(item.created_at) }}
                      </span>
                    </div>
                  </div>

                  <!-- Swipe actions -->
                  <template #right>
                    <div class="swipe-actions">
                      <div
                        v-if="item.status === 'draft'"
                        class="swipe-btn swipe-btn--success"
                        @click="publishAnnouncement(item)"
                      >
                        发布
                      </div>
                      <div
                        v-if="item.status === 'published'"
                        class="swipe-btn swipe-btn--warning"
                        @click="archiveAnnouncement(item)"
                      >
                        归档
                      </div>
                    </div>
                  </template>
                </van-swipe-cell>
              </div>

              <div v-if="!annLoading && !annList.length" class="empty-state">
                <van-icon name="bullhorn-o" size="42" color="#cbd5e1" />
                <span class="empty-text">暂无公告</span>
                <span class="empty-hint">点击右下角按钮发布公告</span>
              </div>
            </van-list>
          </van-pull-refresh>

          <!-- FAB -->
          <div class="fab" @click="openAnnCreate">
            <van-icon name="plus" size="22" color="#fff" />
          </div>

          <!-- Create Popup -->
          <van-popup
            v-model:show="showAnnCreate"
            position="bottom"
            round
            :style="{ maxHeight: '85vh' }"
          >
            <div class="popup-sheet">
              <div class="popup-sheet__header">
                <span class="popup-sheet__title">发布公告</span>
                <van-icon name="cross" size="18" color="#94a3b8" @click="showAnnCreate = false" />
              </div>

              <div class="popup-sheet__body">
                <div class="form-field">
                  <label class="form-label">标题 <span class="form-required">*</span></label>
                  <van-field v-model="annForm.title" placeholder="公告标题" />
                </div>
                <div class="form-field">
                  <label class="form-label">类型</label>
                  <van-field v-model="annForm.type" placeholder="如：更新、维护、活动" />
                </div>
                <div class="form-field">
                  <label class="form-label">内容 <span class="form-required">*</span></label>
                  <van-field v-model="annForm.content" type="textarea" rows="4" placeholder="公告内容" />
                </div>
                <div class="form-field form-field--row">
                  <label class="form-label">置顶</label>
                  <van-switch v-model="annForm.is_pinned" size="20px" active-color="#0d9488" />
                </div>
                <div class="form-field">
                  <label class="form-label">生效时间</label>
                  <van-field v-model="annForm.effective_at" placeholder="YYYY-MM-DD HH:mm" />
                </div>
                <div class="form-field">
                  <label class="form-label">过期时间</label>
                  <van-field v-model="annForm.expires_at" placeholder="YYYY-MM-DD HH:mm" />
                </div>
              </div>

              <div class="popup-sheet__footer">
                <van-button block round color="#0d9488" @click="doCreateAnnouncement">
                  创建公告
                </van-button>
              </div>
            </div>
          </van-popup>
        </van-tab>
      </van-tabs>
    </div>
  </div>
</template>

<style scoped>
.page {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(24px + env(safe-area-inset-bottom, 0px));
}

/* ═══════════════════════════════════════
   HERO — Pink Theme
   ═══════════════════════════════════════ */
.hero {
  position: relative;
  overflow: hidden;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #9d174d 0%, #ec4899 40%, #f472b6 70%, #db2777 100%);
  border-radius: 0 0 28px 28px;
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
  background: rgba(244, 114, 182, 0.3);
  top: -30px;
  right: -20px;
}
.hero-orb--2 {
  width: 140px;
  height: 140px;
  background: rgba(236, 72, 153, 0.2);
  bottom: -10px;
  left: -30px;
}

.hero-content {
  position: relative;
  z-index: 1;
  padding: 24px 20px 22px;
  animation: fadeSlideUp 0.5s both;
}

.hero-icon-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 6px;
}

.hero-icon-wrap {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.2);
  display: flex;
  align-items: center;
  justify-content: center;
  backdrop-filter: blur(8px);
}

.hero-title {
  font-size: 20px;
  font-weight: 700;
  color: #fff;
  letter-spacing: -0.02em;
}

.hero-subtitle {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.55);
  font-weight: 500;
}

/* ═══════════════════════════════════════
   TABS
   ═══════════════════════════════════════ */
.tabs-wrap {
  position: relative;
  z-index: 2;
  margin-top: -8px;
}

.tabs-wrap :deep(.van-tabs__wrap) {
  background: #fff;
  border-radius: 14px 14px 0 0;
  box-shadow: 0 -2px 12px rgba(0, 0, 0, 0.04);
}

.tabs-wrap :deep(.van-tabs__nav) {
  padding: 0 16px;
}

/* ═══════════════════════════════════════
   FILTER CHIPS
   ═══════════════════════════════════════ */
.chip-scroll {
  display: flex;
  gap: 8px;
  padding: 12px 16px 8px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  animation: fadeSlideUp 0.4s 0.06s both;
}
.chip-scroll::-webkit-scrollbar {
  display: none;
}

.chip {
  flex-shrink: 0;
  padding: 5px 14px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
  color: #64748b;
  background: #f1f5f9;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
  -webkit-tap-highlight-color: transparent;
}
.chip:active {
  transform: scale(0.96);
}
.chip--active {
  color: #fff;
  background: #0d9488;
  box-shadow: 0 2px 8px rgba(13, 148, 136, 0.3);
}

/* ═══════════════════════════════════════
   COUNT BAR
   ═══════════════════════════════════════ */
.count-bar {
  padding: 2px 16px 6px;
  animation: fadeSlideUp 0.4s 0.1s both;
}
.count-text {
  font-size: 12px;
  color: var(--ta-text-tertiary, #94a3b8);
}
.count-text b {
  color: var(--ta-text-secondary, #475569);
  font-weight: 600;
}

/* ═══════════════════════════════════════
   BADGE
   ═══════════════════════════════════════ */
.badge {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  line-height: 1.4;
}
.badge--broadcast {
  color: #ec4899;
  background: rgba(236, 72, 153, 0.1);
}

/* ═══════════════════════════════════════
   CARD LIST
   ═══════════════════════════════════════ */
.card-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 12px 12px;
}

/* ── Message Card ── */
.msg-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  animation: fadeSlideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
  transition: transform 0.15s;
}
.msg-card:active {
  transform: scale(0.98);
}

.msg-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.msg-card__title-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 0;
}

.msg-card__unread {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #ec4899;
  flex-shrink: 0;
  box-shadow: 0 0 0 2px rgba(236, 72, 153, 0.2);
}

.msg-card__title {
  font-size: 14px;
  font-weight: 700;
  color: var(--ta-text-primary, #0f172a);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.msg-card__badges {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.msg-card__content {
  font-size: 13px;
  color: var(--ta-text-secondary, #475569);
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  margin-bottom: 10px;
}

.msg-card__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.msg-card__tenant {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
}

.msg-card__time {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
}

/* ── Template Card ── */
.tpl-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  animation: fadeSlideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
  cursor: pointer;
  transition: transform 0.15s;
}
.tpl-card:active {
  transform: scale(0.98);
}

.tpl-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}

.tpl-card__code-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 0;
}

.tpl-card__code {
  font-size: 13px;
  font-weight: 600;
  color: var(--ta-text-primary, #0f172a);
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  letter-spacing: -0.02em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tpl-card__subject {
  font-size: 13px;
  color: var(--ta-text-secondary, #475569);
  margin-bottom: 4px;
}

.tpl-card__body {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--ta-border-light, #f1f5f9);
}

.tpl-card__body-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--ta-text-tertiary, #94a3b8);
  margin-bottom: 6px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.tpl-card__body-text {
  font-size: 12px;
  color: var(--ta-text-secondary, #475569);
  line-height: 1.6;
  background: var(--ta-bg-secondary, #f8fafc);
  border-radius: 8px;
  padding: 10px;
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  white-space: pre-wrap;
  word-break: break-all;
}

.tpl-card__vars {
  margin-top: 8px;
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.tpl-card__vars-label {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
}

.tpl-card__vars-text {
  font-size: 11px;
  color: #0d9488;
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
}

/* ── Announcement Card ── */
.ann-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  animation: fadeSlideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
}

.ann-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.ann-card__title-row {
  display: flex;
  align-items: center;
  gap: 5px;
  flex: 1;
  min-width: 0;
}

.ann-card__pin {
  flex-shrink: 0;
}

.ann-card__title {
  font-size: 14px;
  font-weight: 700;
  color: var(--ta-text-primary, #0f172a);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ann-card__badges {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.ann-card__content {
  font-size: 13px;
  color: var(--ta-text-secondary, #475569);
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  margin-bottom: 10px;
}

.ann-card__footer {
  display: flex;
  align-items: center;
  gap: 12px;
}

.ann-card__time {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
}

/* ═══════════════════════════════════════
   SWIPE ACTIONS
   ═══════════════════════════════════════ */
.swipe-actions {
  display: flex;
  height: 100%;
  border-radius: 0 14px 14px 0;
  overflow: hidden;
}

.swipe-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  cursor: pointer;
}
.swipe-btn--success {
  background: #10b981;
}
.swipe-btn--warning {
  background: #f59e0b;
}

/* ═══════════════════════════════════════
   FAB
   ═══════════════════════════════════════ */
.fab {
  position: fixed;
  right: 20px;
  bottom: calc(24px + env(safe-area-inset-bottom, 0px));
  width: 50px;
  height: 50px;
  border-radius: 16px;
  background: linear-gradient(135deg, #0d9488, #14b8a6);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 16px rgba(13, 148, 136, 0.4);
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
  padding: 16px 20px;
}

.form-field {
  margin-bottom: 12px;
}
.form-field--row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 0;
}

.form-label {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: var(--ta-text-secondary, #475569);
  margin-bottom: 4px;
}

.form-required {
  color: #ef4444;
}

.dialog-form :deep(.van-cell) {
  padding: 8px 12px;
  border-radius: 10px;
  background: var(--ta-bg-secondary, #f8fafc);
}

.dialog-form :deep(.van-field__control) {
  font-size: 14px;
}

/* ═══════════════════════════════════════
   BOTTOM POPUP SHEET
   ═══════════════════════════════════════ */
.popup-sheet {
  display: flex;
  flex-direction: column;
  max-height: 85vh;
}

.popup-sheet__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px 8px;
}

.popup-sheet__title {
  font-size: 17px;
  font-weight: 700;
  color: var(--ta-text-primary, #0f172a);
}

.popup-sheet__body {
  flex: 1;
  overflow-y: auto;
  padding: 8px 20px;
}

.popup-sheet__footer {
  padding: 12px 20px calc(12px + env(safe-area-inset-bottom, 0px));
}

.popup-sheet .form-field {
  margin-bottom: 14px;
}

.popup-sheet :deep(.van-cell) {
  padding: 8px 12px;
  border-radius: 10px;
  background: var(--ta-bg-secondary, #f8fafc);
}

/* ═══════════════════════════════════════
   EMPTY & LOADING
   ═══════════════════════════════════════ */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 56px 0 24px;
  animation: fadeSlideUp 0.4s both;
}

.empty-text {
  font-size: 14px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
}

.empty-hint {
  font-size: 12px;
  color: var(--ta-text-quaternary, #cbd5e1);
}

.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 48px 0;
  font-size: 13px;
  color: var(--ta-text-tertiary, #94a3b8);
}

/* ═══════════════════════════════════════
   ANIMATION
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
