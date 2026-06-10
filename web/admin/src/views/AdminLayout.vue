<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, provide } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import axios from 'axios'
import { marked } from 'marked'
import request from '@/utils/request'
import { useWatermark } from '@/composables/useWatermark'
import { useSiteName } from '@/composables/useSiteName'
import {
  IconDashboard,
  IconUserGroup,
  IconApps,
  IconBranch,
  IconFile,
  IconCommand,
  IconIdcard,
  IconStorage,
  IconGift,
  IconTag,
  IconCodeBlock,
  IconClockCircle,
  IconSafe,
  IconSettings,
  IconMenuFold,
  IconMenuUnfold,
  IconMessage,
  IconNotification,
  IconSend,
  IconRight,
  IconCalendar,
  IconLayers,
  IconHome,
  IconUser,
  IconCheckCircleFill,
} from '@arco-design/web-vue/es/icon'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { siteName } = useSiteName()

const collapsed = ref(false)
const mobileMenuOpen = ref(false)
const collapsedGroups = reactive<Record<string, boolean>>({})
const isMobile = ref(false)
const demoMode = ref(false)
const demoMessage = ref('')

// --- Version & Update ---
const appVersion = ref(__APP_VERSION__ || 'dev')
const hasUpdate = ref(false)
const latestVersion = ref('')
const updateModalVisible = ref(false)
const updateLoading = ref(false)
const executing = ref(false)
const rollingBack = ref(false)
const updateStatus = ref<any>(null)
const releaseNotes = ref('')
const releaseUrl = ref('')
const updatePollTimer = ref<ReturnType<typeof setInterval> | null>(null)

const releaseNotesHtml = computed(() => {
  if (!releaseNotes.value) return ''
  return marked(releaseNotes.value) as string
})

const isUpdating = computed(() => updateStatus.value?.updating === true)
const isFailed = computed(() => updateStatus.value?.update_progress?.phase === 'failed')
const isComplete = computed(() => updateStatus.value?.update_progress?.phase === 'complete')
const isDocker = computed(() => updateStatus.value?.deployment_mode === 'docker')
const progress = computed(() => updateStatus.value?.update_progress)
const rollbackAvailable = computed(() => updateStatus.value?.rollback_available === true)
const backupVersion = computed(() => updateStatus.value?.backup_version || '')

async function checkUpdate(force = false) {
  try {
    const res = await request.get('/admin/update/check', { params: { force } })
    const data = res.data?.data
    if (data) {
      hasUpdate.value = data.has_update
      latestVersion.value = data.latest_version || ''
      releaseNotes.value = data.release_notes || ''
      releaseUrl.value = data.release_url || ''
    }
  } catch {
    // silent
  }
}

async function fetchUpdateStatus() {
  try {
    const res = await request.get('/admin/update/status')
    updateStatus.value = res.data?.data
  } catch {
    // silent
  }
}

function openUpdateModal() {
  updateModalVisible.value = true
  fetchUpdateStatus()
}

function closeUpdateModal() {
  updateModalVisible.value = false
  stopPolling()
}

function startPolling() {
  stopPolling()
  updatePollTimer.value = setInterval(fetchUpdateStatus, 2000)
}

function stopPolling() {
  if (updatePollTimer.value) {
    clearInterval(updatePollTimer.value)
    updatePollTimer.value = null
  }
}

async function executeUpdate() {
  if (!latestVersion.value) return
  executing.value = true
  try {
    await request.post('/admin/update/execute', { version: latestVersion.value })
    Message.success('系统更新已启动')
    startPolling()
  } catch (e: any) {
    Message.error(e.response?.data?.message || '启动更新失败')
  } finally {
    executing.value = false
  }
}

async function doRollback() {
  rollingBack.value = true
  try {
    await request.post('/admin/update/rollback')
    Message.success('回滚已启动，系统将重启...')
    startPolling()
  } catch (e: any) {
    Message.error(e.response?.data?.message || '回滚失败')
  } finally {
    rollingBack.value = false
  }
}

function getPhaseLabel(phase: string): string {
  const map: Record<string, string> = {
    downloading: '下载中', verifying: '校验中', backing_up: '备份中',
    replacing: '替换中', restarting: '重启中', complete: '完成', failed: '失败',
  }
  return map[phase] || phase
}

function getStepStatus(phase: string): string {
  if (!progress.value) return 'wait'
  const phases = ['downloading', 'verifying', 'backing_up', 'replacing', 'restarting']
  const currentIdx = phases.indexOf(progress.value.phase)
  const stepIdx = phases.indexOf(phase)
  if (isFailed.value) return stepIdx < currentIdx ? 'finish' : stepIdx === currentIdx ? 'error' : 'wait'
  if (isComplete.value) return 'finish'
  if (stepIdx < currentIdx) return 'finish'
  if (stepIdx === currentIdx) return 'process'
  return 'wait'
}

// --- End Version & Update ---

provide('demoMode', demoMode)

const { mount: mountWatermark, unmount: unmountWatermark } = useWatermark(demoMessage)

function updateMobile() {
  isMobile.value = window.innerWidth <= 768
}

const hoveredGroup = ref<string | null>(null)
const popupTop = ref(0)
let popupHideTimer: ReturnType<typeof setTimeout> | null = null

const activeKey = computed(() => route.name as string)

function toggleGroup(group: string) {
  if (collapsed.value) {
    collapsed.value = false
    collapsedGroups[group] = false
    return
  }
  collapsedGroups[group] = !isGroupCollapsed(group)
}

function isGroupCollapsed(group: string): boolean {
  if (group in collapsedGroups) return collapsedGroups[group]
  return group !== 'dashboard' && group !== 'ai' && group !== 'tenants'
}

function onGroupMouseEnter(groupKey: string, event: MouseEvent) {
  if (!collapsed.value) return
  clearTimeout(popupHideTimer!)
  hoveredGroup.value = groupKey
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  const groupData = menuGroups.find(g => g.key === groupKey)
  const estimatedHeight = 32 + (groupData?.items.length || 0) * 36
  popupTop.value = Math.max(8, Math.min(rect.top, window.innerHeight - estimatedHeight - 8))
}

function onGroupMouseLeave() {
  popupHideTimer = setTimeout(() => {
    hoveredGroup.value = null
  }, 120)
}

function onPopupMouseEnter() {
  clearTimeout(popupHideTimer!)
}

function onPopupMouseLeave() {
  popupHideTimer = setTimeout(() => {
    hoveredGroup.value = null
  }, 120)
}

const hoveredGroupData = computed(() => {
  return menuGroups.find(g => g.key === hoveredGroup.value)
})

function handlePopupItemClick(key: string) {
  hoveredGroup.value = null
  router.push({ name: key })
}

function onCollapseEnter(el: Element) {
  const e = el as HTMLElement
  e.style.height = '0'
  e.style.overflow = 'hidden'
  // eslint-disable-next-line @typescript-eslint/no-unused-expressions
  e.offsetHeight // force reflow
  e.style.height = e.scrollHeight + 'px'
}

function onCollapseAfterEnter(el: Element) {
  const e = el as HTMLElement
  e.style.height = 'auto'
  e.style.overflow = ''
}

function onCollapseLeave(el: Element) {
  const e = el as HTMLElement
  e.style.height = e.scrollHeight + 'px'
  e.style.overflow = 'hidden'
  // eslint-disable-next-line @typescript-eslint/no-unused-expressions
  e.offsetHeight // force reflow
  e.style.height = '0'
}

function onCollapseAfterLeave(el: Element) {
  const e = el as HTMLElement
  e.style.height = ''
  e.style.overflow = ''
}

const menuGroups = [
  {
    key: 'dashboard',
    label: '数据看板',
    icon: IconDashboard,
    items: [
      { name: 'AdminDashboard', label: '仪表盘', icon: IconDashboard },
      { name: 'AdminRealtimeMonitor', label: '实时监控', icon: IconCommand },
    ],
  },
  {
    key: 'ai',
    label: '大模型',
    icon: IconApps,
    items: [
      { name: 'AdminModels', label: '模型列表', icon: IconApps },
      { name: 'AdminModelGroups', label: '模型分组', icon: IconStorage },
      { name: 'AdminChannels', label: '渠道管理', icon: IconBranch },
      { name: 'AdminTaskLogs', label: '任务日志', icon: IconCalendar },
    ],
  },
  {
    key: 'tenants',
    label: '租户管理',
    icon: IconUserGroup,
    items: [
      { name: 'AdminTenants', label: '租户列表', icon: IconHome },
      { name: 'AdminTenantLevels', label: '租户级别', icon: IconLayers },
      { name: 'AdminMembers', label: '成员列表', icon: IconUser },
      { name: 'AdminUsageLogs', label: '用量日志', icon: IconFile },
    ],
  },
  {
    key: 'finance',
    label: '财务中心',
    icon: IconIdcard,
    items: [
      { name: 'AdminPlans', label: '套餐管理', icon: IconIdcard },
      { name: 'AdminOrders', label: '订单管理', icon: IconCodeBlock },
      { name: 'AdminTransactions', label: '交易流水', icon: IconCommand },
      { name: 'AdminRedemptions', label: '兑换码管理', icon: IconGift },
      { name: 'AdminPromoCodes', label: '优惠码管理', icon: IconTag },
    ],
  },
  {
    key: 'security',
    label: '安全审计',
    icon: IconSafe,
    items: [
      { name: 'AdminLoginHistory', label: '登录历史', icon: IconClockCircle },
      { name: 'AdminTenantLoginHistory', label: '租户登录历史', icon: IconClockCircle },
      { name: 'AdminSessions', label: '会话管理', icon: IconClockCircle },
      { name: 'AdminPermissions', label: '权限管理', icon: IconSafe },
      { name: 'AdminAudit', label: '操作日志', icon: IconFile },
      { name: 'AdminRequestAuditLogs', label: '请求审计日志', icon: IconCommand },
    ],
  },
  {
    key: 'operations',
    label: '运营管理',
    icon: IconNotification,
    items: [
      { name: 'AdminNotificationTemplates', label: '通知模板', icon: IconNotification },
      { name: 'AdminMessages', label: '消息管理', icon: IconSend },
      { name: 'AdminAnnouncements', label: '公告管理', icon: IconMessage },
      { name: 'AdminTickets', label: '工单管理', icon: IconCommand },
      { name: 'AdminFeedback', label: '反馈管理', icon: IconMessage },
    ],
  },
  {
    key: 'monitoring',
    label: '运维监控',
    icon: IconCommand,
    items: [
      { name: 'AdminMonitor', label: '系统监控', icon: IconCommand },
      { name: 'AdminAlertRules', label: '告警规则', icon: IconNotification },
      { name: 'AdminAlertEvents', label: '告警记录', icon: IconFile },
      { name: 'AdminErrorLogs', label: '错误日志', icon: IconFile },
      { name: 'AdminChannelErrors', label: '渠道错误监控', icon: IconFile },
      { name: 'AdminContentFilterLogs', label: '拦截日志', icon: IconFile },
      { name: 'AdminCronJobs', label: '定时任务', icon: IconFile },
    ],
  },
  {
    key: 'system',
    label: '系统',
    icon: IconSettings,
    items: [
      { name: 'AdminUsers', label: '用户管理', icon: IconUserGroup },
      { name: 'AdminPlugins', label: '插件管理', icon: IconCodeBlock },
      { name: 'AdminSettings', label: '系统设置', icon: IconSettings },
      { name: 'AdminPaymentSettings', label: '支付设置', icon: IconStorage },
      { name: 'AdminHelpCategories', label: '帮助分类', icon: IconLayers },
      { name: 'AdminHelpArticles', label: '帮助文章', icon: IconFile },
      { name: 'AdminChangelogs', label: '更新日志', icon: IconFile },
    ],
  },
]

const breadcrumbItems = computed(() => {
  return route.matched
    .filter(item => item.meta?.title)
    .map(item => item.meta.title as string)
})

function handleMenuUpdate(key: string) {
  if (collapsed.value) {
    collapsed.value = false
    return
  }
  router.push({ name: key })
  mobileMenuOpen.value = false
}

async function handleLogout() {
  try {
    await authStore.logout()
  } catch {
    // best-effort
  }
  Message.success('已退出登录')
  router.push({ name: 'AdminLogin' })
}

const displayLabel = computed(() => authStore.user?.display_name || authStore.user?.username || '管理员')

const userMenuVisible = ref(false)

function toggleUserMenu() {
  userMenuVisible.value = !userMenuVisible.value
}

function closeUserMenu() {
  userMenuVisible.value = false
}

function handleUserDropdown(key: string) {
  userMenuVisible.value = false
  if (key === 'profile') {
    router.push({ name: 'AdminProfile' })
  } else if (key === 'logout') {
    handleLogout()
  }
}

function handleClickOutside(e: MouseEvent) {
  const target = e.target as HTMLElement
  if (!target.closest('.admin-header__user-wrapper')) {
    userMenuVisible.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  updateMobile()
  window.addEventListener('resize', updateMobile)

  // Check for updates on mount
  checkUpdate()

  axios.get('/api/settings/public').then((res) => {
    const settings = res.data?.data?.settings
    if (settings) {
      demoMode.value = !!settings.demo_mode
      demoMessage.value = settings.demo_message || '演示环境，数据不可修改'
      if (demoMode.value) {
        mountWatermark(document.body)
      }
    }
  }).catch(() => {})
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  window.removeEventListener('resize', updateMobile)
  clearTimeout(popupHideTimer!)
  stopPolling()
  unmountWatermark()
})
</script>

<template>
  <div class="admin-layout">
    <!-- Desktop Sidebar -->
    <div class="admin-sidebar" :class="{ 'admin-sidebar--collapsed': collapsed }">
      <div class="admin-sidebar__logo" @click="openUpdateModal" style="cursor: pointer;">
        <img src="/favicon.png" alt="Logo" class="admin-sidebar__logo-icon" />
        <Transition name="fade">
          <span v-if="!collapsed" class="admin-sidebar__logo-text">
            {{ siteName || 'Team-API' }}
            <span class="admin-sidebar__version">
              {{ appVersion === 'dev' ? 'dev' : 'v' + appVersion }}
              <span v-if="hasUpdate" class="admin-sidebar__update-dot" title="有新版本可用"></span>
            </span>
          </span>
        </Transition>
      </div>

      <div class="admin-sidebar__menu">
        <div v-for="group in menuGroups" :key="group.key">
          <div
            class="admin-sidebar__group-header"
            :class="{ 'admin-sidebar__group-header--hovered': hoveredGroup === group.key }"
            @click="toggleGroup(group.key)"
            @mouseenter="onGroupMouseEnter(group.key, $event)"
            @mouseleave="onGroupMouseLeave"
          >
            <component :is="group.icon" class="admin-sidebar__group-icon" />
            <span class="admin-sidebar__group-label">{{ group.label }}</span>
            <IconRight class="admin-sidebar__group-chevron" :class="{ 'admin-sidebar__group-chevron--collapsed': isGroupCollapsed(group.key) }" />
          </div>
          <Transition name="collapse" @enter="onCollapseEnter" @after-enter="onCollapseAfterEnter" @leave="onCollapseLeave" @after-leave="onCollapseAfterLeave">
            <div v-show="!isGroupCollapsed(group.key)" class="admin-sidebar__group-items">
              <div
                v-for="item in group.items"
                :key="item.name"
                class="admin-sidebar__item"
                :class="{ 'admin-sidebar__item--active': activeKey === item.name }"
                @click="handleMenuUpdate(item.name)"
              >
                <component :is="item.icon" class="admin-sidebar__icon" />
                <span class="admin-sidebar__text">{{ item.label }}</span>
              </div>
            </div>
          </Transition>
        </div>
      </div>

    </div>

    <!-- Collapsed Popup Menu -->
    <Teleport to="body">
      <Transition name="fade">
        <div
          v-if="hoveredGroup && collapsed && hoveredGroupData"
          class="sidebar-popup"
          :style="{ top: popupTop + 'px' }"
          @mouseenter="onPopupMouseEnter"
          @mouseleave="onPopupMouseLeave"
        >
          <div class="sidebar-popup__title">{{ hoveredGroupData.label }}</div>
          <div
            v-for="item in hoveredGroupData.items"
            :key="item.name"
            class="sidebar-popup__item"
            :class="{ 'sidebar-popup__item--active': activeKey === item.name }"
            @click="handlePopupItemClick(item.name)"
          >
            <component :is="item.icon" class="sidebar-popup__item-icon" />
            <span>{{ item.label }}</span>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- Mobile Menu Overlay -->
    <Teleport to="body">
      <Transition name="fade">
        <div
          v-if="mobileMenuOpen"
          class="mobile-overlay"
          @click="mobileMenuOpen = false"
        />
      </Transition>
      <Transition name="slide">
        <div v-if="mobileMenuOpen" class="admin-sidebar admin-sidebar--mobile">
          <div class="admin-sidebar__logo">
            <img src="/favicon.png" alt="Logo" class="admin-sidebar__logo-icon" />
            <span class="admin-sidebar__logo-text">{{ siteName || 'Team-API' }}</span>
          </div>
          <div class="admin-sidebar__menu">
            <div v-for="group in menuGroups" :key="group.key">
              <div class="admin-sidebar__group-header" @click="toggleGroup(group.key)">
                <component :is="group.icon" class="admin-sidebar__group-icon" />
                <span class="admin-sidebar__group-label">{{ group.label }}</span>
                <IconRight class="admin-sidebar__group-chevron" :class="{ 'admin-sidebar__group-chevron--collapsed': isGroupCollapsed(group.key) }" />
              </div>
              <Transition name="collapse" @enter="onCollapseEnter" @after-enter="onCollapseAfterEnter" @leave="onCollapseLeave" @after-leave="onCollapseAfterLeave">
                <div v-show="!isGroupCollapsed(group.key)" class="admin-sidebar__group-items">
                  <div
                    v-for="item in group.items"
                    :key="item.name"
                    class="admin-sidebar__item"
                    :class="{ 'admin-sidebar__item--active': activeKey === item.name }"
                    @click="handleMenuUpdate(item.name)"
                  >
                    <component :is="item.icon" class="admin-sidebar__icon" />
                    <span class="admin-sidebar__text">{{ item.label }}</span>
                  </div>
                </div>
              </Transition>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- Main Area -->
    <div class="admin-main">
      <!-- Header -->
      <header class="admin-header">
        <div class="admin-header__left">
          <button class="admin-header__menu-btn" @click="isMobile ? (mobileMenuOpen = !mobileMenuOpen) : (collapsed = !collapsed)">
            <IconMenuFold v-if="!isMobile && !collapsed" class="admin-header__menu-icon" />
            <IconMenuUnfold v-else-if="!isMobile && collapsed" class="admin-header__menu-icon" />
            <svg v-else class="admin-header__menu-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          </button>
          <div v-if="breadcrumbItems.length" class="admin-header__breadcrumbs">
            <template v-for="(item, index) in breadcrumbItems" :key="index">
              <span class="admin-header__breadcrumb">{{ item }}</span>
              <span v-if="index < breadcrumbItems.length - 1" class="admin-header__breadcrumb-sep">/</span>
            </template>
          </div>
        </div>

        <div class="admin-header__right">
          <div class="admin-header__user-wrapper" style="position: relative;">
            <div class="admin-header__user" @click.stop="toggleUserMenu">
              <div class="admin-header__avatar">{{ displayLabel.charAt(0) }}</div>
              <span class="hidden sm:inline">{{ displayLabel }}</span>
              <svg class="hidden sm:inline h-4 w-4" :style="{ transform: userMenuVisible ? 'rotate(180deg)' : '', transition: 'transform 0.2s', color: 'var(--ta-text-tertiary)' }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
              </svg>
            </div>
            <Transition name="fade">
              <div v-if="userMenuVisible" class="admin-header__user-menu">
                <div class="admin-header__user-menu-item" @click="handleUserDropdown('profile')">
                  <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" /></svg>
                  <span>个人信息</span>
                </div>
                <div class="admin-header__user-menu-divider" />
                <div class="admin-header__user-menu-item admin-header__user-menu-item--danger" @click="handleUserDropdown('logout')">
                  <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" /></svg>
                  <span>退出登录</span>
                </div>
              </div>
            </Transition>
          </div>
        </div>
      </header>

      <!-- Content -->
      <main class="admin-content page-bg">
        <router-view v-slot="{ Component }">
          <Transition name="page-fade" mode="out-in">
            <component :is="Component" />
          </Transition>
        </router-view>
      </main>
    </div>

    <!-- Update Modal -->
    <a-modal
      v-model:visible="updateModalVisible"
      :footer="false"
      :closable="true"
      :mask-closable="true"
      title-align="start"
      width="520px"
      @cancel="closeUpdateModal"
    >
      <template #title>
        <div style="display: flex; align-items: center; gap: 8px;">
          <span>系统更新</span>
          <a-tag v-if="isDocker" color="blue" size="small">Docker</a-tag>
          <a-tag v-else color="green" size="small">二进制</a-tag>
        </div>
      </template>

      <!-- Version info -->
      <div style="display: flex; gap: 16px; margin-bottom: 20px;">
        <a-card :bordered="false" class="update-modal-stat" size="small">
          <div class="update-modal-stat__title">当前版本</div>
          <div class="update-modal-stat__value">v{{ appVersion }}</div>
        </a-card>
        <a-card :bordered="false" class="update-modal-stat" size="small">
          <div class="update-modal-stat__title">最新版本</div>
          <div class="update-modal-stat__value" :style="hasUpdate ? { color: '#00b42a' } : {}">
            {{ latestVersion ? 'v' + latestVersion : '--' }}
          </div>
        </a-card>
      </div>

      <!-- Docker mode hint -->
      <template v-if="isDocker">
        <a-alert type="info" style="margin-bottom: 16px;">
          Docker 部署模式不支持在线自动更新，请手动执行：
        </a-alert>
        <a-input
          model-value="docker compose pull && docker compose up -d"
          read-only
          style="font-family: monospace; margin-bottom: 16px;"
        >
          <template #append>
            <a-button type="text" size="mini" @click="() => { navigator.clipboard.writeText('docker compose pull && docker compose up -d'); Message.success('已复制') }">复制</a-button>
          </template>
        </a-input>
      </template>

      <!-- Update available -->
      <template v-if="hasUpdate && !isUpdating && !isFailed && !isComplete && !isDocker">
        <a-divider style="margin: 16px 0;" />
        <div style="margin-bottom: 12px;">
          <a-typography-text bold>更新说明</a-typography-text>
        </div>
        <div class="update-modal-notes" v-html="releaseNotesHtml"></div>
        <a-divider style="margin: 16px 0;" />
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <a-typography-text type="secondary" style="font-size: 12px;">⚠️ 更新过程中服务将短暂中断</a-typography-text>
          <a-button type="primary" :loading="executing" @click="executeUpdate">
            {{ executing ? '启动中...' : '立即更新' }}
          </a-button>
        </div>
      </template>

      <!-- No update -->
      <template v-if="!hasUpdate && !isUpdating">
        <a-result>
          <template #icon><icon-check-circle-fill style="color: #00b42a; font-size: 32px;" /></template>
          <template #title><span style="color: #00b42a; font-size: 14px;">当前已是最新版本</span></template>
        </a-result>
      </template>

      <!-- Update progress -->
      <template v-if="isUpdating">
        <a-divider style="margin: 16px 0;" />
        <a-steps :current="1" size="small" style="margin-bottom: 16px;">
          <a-step title="下载" :status="getStepStatus('downloading')" />
          <a-step title="校验" :status="getStepStatus('verifying')" />
          <a-step title="备份" :status="getStepStatus('backing_up')" />
          <a-step title="替换" :status="getStepStatus('replacing')" />
          <a-step title="重启" :status="getStepStatus('restarting')" />
        </a-steps>
        <a-progress
          :percent="(progress?.percentage || 0) / 100"
          :status="isFailed ? 'danger' : isComplete ? 'success' : 'normal'"
        />
        <a-typography-text :type="isFailed ? 'danger' : 'secondary'" style="font-size: 13px;">
          {{ progress?.message }}
        </a-typography-text>
      </template>

      <!-- Failed with rollback -->
      <template v-if="isFailed">
        <a-divider style="margin: 16px 0;" />
        <a-alert type="error" style="margin-bottom: 12px;">更新失败：{{ progress?.error || '未知错误' }}</a-alert>
        <div v-if="rollbackAvailable" style="text-align: right;">
          <a-button status="danger" :loading="rollingBack" @click="doRollback">回滚到 v{{ backupVersion }}</a-button>
        </div>
      </template>

      <!-- Complete with rollback option -->
      <template v-if="isComplete && rollbackAvailable">
        <a-divider style="margin: 16px 0;" />
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <a-typography-text type="secondary" style="font-size: 12px;">如遇问题可回滚到 v{{ backupVersion }}</a-typography-text>
          <a-button status="warning" :loading="rollingBack" @click="doRollback">回滚</a-button>
        </div>
      </template>

      <template v-if="releaseUrl && hasUpdate">
        <a-divider style="margin: 8px 0 0;" />
        <div style="text-align: center; padding-top: 8px;">
          <a-link :href="releaseUrl" target="_blank" style="font-size: 12px;">在 GitHub 上查看完整发布说明</a-link>
        </div>
      </template>
    </a-modal>
  </div>
</template>

<style scoped>
/* ===== Layout ===== */
.admin-layout {
  display: flex;
  height: 100vh;
  background: var(--ta-bg-page);
}

/* ===== Sidebar ===== */
.admin-sidebar {
  width: 220px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--ta-sidebar-bg);
  transition: width var(--ta-duration-slow) var(--ta-ease);
  overflow: hidden;
}

.admin-sidebar--collapsed {
  width: 64px;
}

.admin-sidebar--mobile {
  position: fixed;
  top: 0;
  bottom: 0;
  left: 0;
  z-index: 50;
  width: 264px;
}

.admin-sidebar__logo {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 56px;
  padding: 0 16px;
  border-bottom: 1px solid var(--ta-sidebar-divider);
  flex-shrink: 0;
}

.admin-sidebar__logo-icon {
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  object-fit: contain;
}

.admin-sidebar__logo-text {
  color: #f1f5f9;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: -0.02em;
  white-space: nowrap;
  overflow: hidden;
}

.admin-sidebar--collapsed .admin-sidebar__logo {
  justify-content: center;
  padding: 0;
}

.admin-sidebar__menu {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
  overflow-x: hidden;
}

/* ===== Collapsible Group Header ===== */
.admin-sidebar__group-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 20px 16px 6px;
  cursor: pointer;
  user-select: none;
}

.admin-sidebar__group-header:hover .admin-sidebar__group-label {
  color: rgba(255, 255, 255, 0.55);
}

.admin-sidebar__group-header:hover .admin-sidebar__group-icon {
  color: rgba(255, 255, 255, 0.55);
}

.admin-sidebar__group-icon {
  flex-shrink: 0;
  font-size: 14px;
  color: rgba(255, 255, 255, 0.35);
  transition: color var(--ta-duration-fast) var(--ta-ease);
}

.admin-sidebar__group-label {
  flex: 1;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: rgba(255, 255, 255, 0.35);
  white-space: nowrap;
  overflow: hidden;
  transition: color 0.2s;
}

.admin-sidebar__group-chevron {
  font-size: 10px;
  color: rgba(255, 255, 255, 0.25);
  transition: transform 0.25s ease;
  flex-shrink: 0;
}

.admin-sidebar__group-chevron--collapsed {
  transform: rotate(-90deg);
}

/* ===== Collapsible Group Items ===== */
.admin-sidebar__group-items {
  overflow: hidden;
}

.collapse-enter-active,
.collapse-leave-active {
  transition: height 0.25s ease;
}

.admin-sidebar__item {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 32px;
  margin: 1px 0;
  padding: 0 12px 0 24px;
  border-radius: 8px;
  color: var(--ta-sidebar-text);
  cursor: pointer;
  transition: all var(--ta-duration-fast) var(--ta-ease);
  white-space: nowrap;
  overflow: hidden;
}

.admin-sidebar__item:hover {
  color: rgba(255, 255, 255, 0.85);
  background: rgba(255, 255, 255, 0.06);
}

.admin-sidebar__item--active {
  color: #5eead4 !important;
  background: var(--ta-sidebar-active) !important;
}

.admin-sidebar__icon {
  flex-shrink: 0;
  font-size: 18px;
}

.admin-sidebar__text {
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* ===== Collapsed Sidebar Overrides ===== */
.admin-sidebar--collapsed .admin-sidebar__group-header {
  justify-content: center;
  padding: 12px 0;
  cursor: pointer;
}

.admin-sidebar--collapsed .admin-sidebar__group-header--hovered {
  color: rgba(255, 255, 255, 0.85);
  background: rgba(255, 255, 255, 0.06);
  border-radius: 8px;
}

.admin-sidebar--collapsed .admin-sidebar__group-icon {
  font-size: 18px;
  color: var(--ta-sidebar-text);
}

.admin-sidebar--collapsed .admin-sidebar__group-label,
.admin-sidebar--collapsed .admin-sidebar__group-chevron {
  display: none;
}

.admin-sidebar--collapsed .admin-sidebar__group-items {
  display: none;
}

.admin-sidebar--collapsed .admin-sidebar__text {
  display: none;
}

/* ===== Header ===== */
.admin-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 64px;
  padding: 0 24px;
  background: var(--ta-bg-card);
  border-bottom: 1px solid var(--ta-border-light);
  box-shadow: var(--ta-shadow-card);
  z-index: 10;
  flex-shrink: 0;
}

.admin-header__left {
  display: flex;
  align-items: center;
  gap: 16px;
  min-width: 0;
}

.admin-header__menu-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  color: var(--ta-text-secondary);
  cursor: pointer;
  transition: all var(--ta-duration-fast) var(--ta-ease);
  background: none;
  border: none;
}

.admin-header__menu-btn:hover {
  background: var(--ta-bg-secondary);
  color: var(--ta-text-primary);
}

.admin-header__menu-icon {
  width: 20px;
  height: 20px;
}

.admin-header__breadcrumbs {
  display: flex;
  align-items: center;
  gap: 0;
  font-size: 13px;
  color: var(--ta-text-tertiary);
  overflow: hidden;
  white-space: nowrap;
  min-width: 0;
}

.admin-header__breadcrumb {
  color: var(--ta-text-secondary);
}

.admin-header__breadcrumb-sep {
  margin: 0 6px;
  color: var(--ta-border);
}

.admin-header__right {
  display: flex;
  align-items: center;
  flex-shrink: 0;
}

.admin-header__user {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-radius: 8px;
  cursor: pointer;
  color: var(--ta-text-secondary);
  font-size: 14px;
  transition: all var(--ta-duration-fast) var(--ta-ease);
}

.admin-header__user:hover {
  background: var(--ta-bg-secondary);
}

.admin-header__avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: var(--ta-primary-gradient);
  color: #fff;
  font-weight: 600;
  font-size: 13px;
}

.admin-header__user-menu {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 160px;
  background: var(--ta-bg-card);
  border: 1px solid var(--ta-border-light);
  border-radius: 10px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  padding: 4px;
  z-index: 100;
}

.admin-header__user-menu-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  color: var(--ta-text-secondary);
  transition: all var(--ta-duration-fast) var(--ta-ease);
}

.admin-header__user-menu-item:hover {
  background: var(--ta-bg-secondary);
  color: var(--ta-text-primary);
}

.admin-header__user-menu-item--danger:hover {
  background: #fef2f2;
  color: #ef4444;
}

.admin-header__user-menu-divider {
  height: 1px;
  margin: 4px 8px;
  background: var(--ta-border-light);
}

/* ===== Main Area ===== */
.admin-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* ===== Content ===== */
.admin-content {
  background: var(--ta-bg-page);
  padding: 24px;
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

/* ===== Transitions ===== */
.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--ta-duration-normal) var(--ta-ease);
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.slide-enter-active,
.slide-leave-active {
  transition: transform var(--ta-duration-slow) var(--ta-ease);
}
.slide-enter-from,
.slide-leave-to {
  transform: translateX(-100%);
}

.mobile-overlay {
  position: fixed;
  inset: 0;
  z-index: 40;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(4px);
}

/* ===== Collapsed Popup Menu ===== */
.sidebar-popup {
  position: fixed;
  left: 72px;
  min-width: 160px;
  background: var(--ta-bg-card, #fff);
  border: 1px solid var(--ta-border-light, #e5e7eb);
  border-radius: 10px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  padding: 8px 4px;
  z-index: 100;
}

.sidebar-popup__title {
  padding: 4px 12px 8px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--ta-text-tertiary, #94a3b8);
}

.sidebar-popup__item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 12px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  color: var(--ta-text-secondary, #64748b);
  transition: all 0.15s;
}

.sidebar-popup__item:hover {
  background: var(--ta-bg-secondary, #f1f5f9);
  color: var(--ta-text-primary, #1e293b);
}

.sidebar-popup__item--active {
  color: var(--ta-primary, #0d9488);
  background: rgba(13, 148, 136, 0.08);
}

.sidebar-popup__item-icon {
  flex-shrink: 0;
  font-size: 16px;
}

/* ===== Responsive ===== */
@media (max-width: 768px) {
  .admin-sidebar:not(.admin-sidebar--mobile) {
    display: none;
  }
  .admin-sidebar--mobile {
    display: flex;
  }
}

/* ===== Version Badge & Update Dot ===== */
.admin-sidebar__version {
  display: inline-flex;
  align-items: center;
  font-size: 10px;
  font-weight: 500;
  color: rgba(255, 255, 255, 0.55);
  background: rgba(255, 255, 255, 0.1);
  border-radius: 10px;
  padding: 1px 7px;
  margin-left: 6px;
  line-height: 18px;
  letter-spacing: 0.02em;
}

.admin-sidebar__update-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #00b42a;
  margin-left: 4px;
  vertical-align: middle;
  position: relative;
  top: -1px;
  animation: pulse-dot 2s ease-in-out infinite;
}

@keyframes pulse-dot {
  0%, 100% { box-shadow: 0 0 0 0 rgba(0, 180, 42, 0.4); }
  50% { box-shadow: 0 0 0 4px rgba(0, 180, 42, 0); }
}

/* ===== Update Modal ===== */
.update-modal-stat {
  flex: 1;
  background: var(--ta-bg-secondary);
}

.update-modal-stat__title {
  font-size: 12px;
  color: var(--ta-text-tertiary);
  margin-bottom: 4px;
}

.update-modal-stat__value {
  font-size: 18px;
  font-weight: 600;
  color: var(--ta-text-primary);
}

.update-modal-notes {
  max-height: 260px;
  overflow-y: auto;
  line-height: 1.8;
  font-size: 13px;
  color: var(--ta-text-secondary);
}

.update-modal-notes :deep(h1),
.update-modal-notes :deep(h2),
.update-modal-notes :deep(h3) {
  margin-top: 12px;
  margin-bottom: 6px;
  font-size: 15px;
}

.update-modal-notes :deep(ul),
.update-modal-notes :deep(ol) {
  padding-left: 20px;
}

.update-modal-notes :deep(code) {
  background: var(--ta-fill-2);
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 12px;
}
</style>
