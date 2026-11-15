<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { Message } from '@arco-design/web-vue'
import {
  IconRefresh,
  IconSettings,
  IconClose,
  IconSend,
  IconPause,
  IconLeft,
  IconRobot,
  IconCopy,
  IconDownload,
  IconCheck,
  IconCheckCircleFill,
  IconCloseCircleFill,
} from '@arco-design/web-vue/es/icon'
import { marked } from 'marked'
import request from '@/utils/request'
import { sanitizeHtml } from '@/utils/sanitize'
import { hasPermission } from '@/utils/permission'
import { useAuthStore } from '@/stores/auth'
import { usePublicSettings } from '@/composables/usePublicSettings'
import {
  useAiAssistantChat,
  SKILL_OPTIONS,
  type ChatMsg,
  type MessagePart,
  type ValidateReport,
} from './useAiAssistantChat'

/**
 * 全局 AI 助手浮动窗口：非模态（无遮罩，不挡页面操作），拖标题栏任意移动、
 * 右下角缩放，位置尺寸按管理员持久化；移动端回落为底部面板（免拖拽）。
 * 内含对话（技能注入 + 代码专用通道 + 校验报告 + 半自动修复）与助手配置面板
 * （独立端点 /admin/ai-assistant/*，同 ai_analysis 模式）。
 */

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{ (e: 'update:visible', v: boolean): void }>()

const authStore = useAuthStore()
const canUse = hasPermission('ai_assistant:use')
const canConfig = hasPermission('ai_assistant:config')

const { settings: publicSettings, fetchSettings } = usePublicSettings()
const chat = useAiAssistantChat()
const {
  messages,
  streaming,
  input,
  activeSkills,
  canSend,
  restore,
  clearAll,
  send,
  stop,
  repair,
  toggleSkill,
} = chat

const listEl = ref<HTMLElement | null>(null)
const textareaEl = ref<HTMLTextAreaElement | null>(null)
// 窗口内视图切换：chat=对话 / config=设置（设置占据整个窗口内容区，不挤压对话）
const activeView = ref<'chat' | 'config'>('chat')

// 未启用判定：公共配置未开启 且 无配置权限（有配置权限的人始终可进配置面板开启）
const assistantDisabled = computed(
  () => publicSettings.ai_assistant_enabled !== true && !canConfig
)

/** 空态引导问句：点一下填进输入框（不直接发送，留出改字空间） */
const QUICK_ASKS = [
  '平台支持哪些计费方式？',
  '帮我起草一条系统维护公告',
  '计费表达式该怎么写？',
]

/** 助手正在等待首个 token：气泡位置显示打字指示器 */
const typingMsgId = computed(() => {
  if (!streaming.value) return 0
  const last = messages.value[messages.value.length - 1]
  if (!last || last.role !== 'assistant') return 0
  return last.parts.some((p) => p.content.length > 0) ? 0 : last.id
})

function renderMarkdown(text: string): string {
  if (!text) return ''
  return sanitizeHtml(marked.parse(text) as string)
}

function skillLabel(key: string): string {
  return SKILL_OPTIONS.find((o) => o.key === key)?.label || key
}

function close() {
  emit('update:visible', false)
}

async function onSend() {
  if (!canSend.value) return
  await send()
}

/** 输入框自增高（1~6 行），发送后随 input 清空回落 */
const MAX_INPUT_H = 132
function autoGrow() {
  const el = textareaEl.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, MAX_INPUT_H)}px`
}

function quickAsk(text: string) {
  if (assistantDisabled.value) return
  input.value = text
  nextTick(() => {
    autoGrow()
    const el = textareaEl.value
    if (el) {
      el.focus()
      el.setSelectionRange(text.length, text.length)
    }
  })
}

/** 复制到剪贴板（代码卡片共用） */
async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    Message.success('已复制')
  } catch {
    Message.error('复制失败')
  }
}

/** 下载代码为文件（plugin.js / formula.txt） */
function downloadCode(part: MessagePart) {
  const name = part.lang === 'formula' ? 'formula.txt' : 'plugin.js'
  const blob = new Blob([part.content], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  a.click()
  URL.revokeObjectURL(url)
}

/** 展开状态：长代码默认收起（流式期间保持展开） */
function partCollapsed(msg: ChatMsg, idx: number): boolean {
  const part = msg.parts[idx]
  if (streaming.value) return false
  return part.content.split('\n').length > 60 && !expandedMap.value[`${msg.id}:${idx}`]
}

const expandedMap = ref<Record<string, boolean>>({})
function toggleExpand(key: string) {
  expandedMap.value[key] = !expandedMap.value[key]
}

function reportNeedsRepair(report?: ValidateReport): boolean {
  if (!report || report.ok) return false
  return Boolean(report.error) || (report.checks || []).some((c) => c.status === 'fail')
}

/** 修复按钮：把校验失败明细回喂下一轮（半自动修复环） */
async function onRepair(report: ValidateReport) {
  await repair(report)
  await scrollToBottom()
}

// 滚动到底部：窗口打开 / 新消息 / 流式期间
async function scrollToBottom() {
  await nextTick()
  const el = listEl.value
  if (el) el.scrollTop = el.scrollHeight
}

watch(
  () => messages.value[messages.value.length - 1]?.parts.length,
  () => scrollToBottom()
)
watch(
  () => messages.value.map((m) => m.parts.map((p) => p.content.length).join(',')).join('|'),
  () => {
    if (streaming.value) scrollToBottom()
  }
)
watch(input, () => nextTick(autoGrow))

// ── 浮动窗口：拖动 / 缩放 / 位置持久化 ────────────────────────

interface WinRect {
  x: number
  y: number
  w: number
  h: number
}

const MIN_W = 400
const MIN_H = 440
const mobileQuery = window.matchMedia('(max-width: 768px)')
const isMobile = ref(mobileQuery.matches)
const dragging = ref(false)
const resizing = ref(false)

function defaultRect(): WinRect {
  const w = Math.min(540, window.innerWidth - 32)
  const h = Math.min(700, window.innerHeight - 96)
  return {
    x: Math.max(12, window.innerWidth - w - 24),
    y: Math.max(64, Math.round((window.innerHeight - h) / 2)),
    w,
    h,
  }
}

const winRect = ref<WinRect>(defaultRect())

/** 位置约束：至少留 80px 宽/48px 高在视口内，防止窗口被拖飞找不回 */
function clampRect(r: WinRect): WinRect {
  const out = { ...r }
  out.w = Math.max(MIN_W, Math.min(r.w, window.innerWidth - 16))
  out.h = Math.max(MIN_H, Math.min(r.h, window.innerHeight - 16))
  out.x = Math.max(-(out.w - 80), Math.min(r.x, window.innerWidth - 80))
  out.y = Math.max(0, Math.min(r.y, window.innerHeight - 48))
  return out
}

function winStorageKey(): string {
  return `admin_ai_assistant_win_${authStore.user?.id ?? 0}`
}

function persistRect() {
  try {
    localStorage.setItem(winStorageKey(), JSON.stringify(winRect.value))
  } catch {
    // 存储不可用：位置仅本次会话有效
  }
}

function restoreRect() {
  winRect.value = defaultRect()
  try {
    const raw = localStorage.getItem(winStorageKey())
    if (raw) winRect.value = clampRect(JSON.parse(raw) as WinRect)
  } catch {
    // 损坏数据回落默认位置
  }
}

const winStyle = computed<Record<string, string>>(() => {
  if (isMobile.value) {
    // 移动端免拖拽：底部面板
    return { left: '0', bottom: '0', width: '100%', height: '78vh' }
  }
  const r = winRect.value
  return {
    left: `${r.x}px`,
    top: `${r.y}px`,
    width: `${r.w}px`,
    height: `${r.h}px`,
  }
})

// 拖动：pointer capture 保证移出标题栏/窗口仍持续跟踪
let dragStart: { px: number; py: number; rect: WinRect } | null = null

function onDragStart(e: PointerEvent) {
  if (isMobile.value) return
  // 任何按钮（新对话/设置/返回/关闭）不触发拖动
  if ((e.target as HTMLElement).closest('button')) return
  dragStart = { px: e.clientX, py: e.clientY, rect: { ...winRect.value } }
  dragging.value = true
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
}

function onDragMove(e: PointerEvent) {
  if (!dragStart) return
  winRect.value = clampRect({
    ...dragStart.rect,
    x: dragStart.rect.x + (e.clientX - dragStart.px),
    y: dragStart.rect.y + (e.clientY - dragStart.py),
  })
}

function onDragEnd() {
  if (!dragStart) return
  dragStart = null
  dragging.value = false
  persistRect()
}

// 缩放：右下角手柄
let resizeStart: { px: number; py: number; rect: WinRect } | null = null

function onResizeStart(e: PointerEvent) {
  if (isMobile.value) return
  resizeStart = { px: e.clientX, py: e.clientY, rect: { ...winRect.value } }
  resizing.value = true
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
}

function onResizeMove(e: PointerEvent) {
  if (!resizeStart) return
  winRect.value = clampRect({
    ...resizeStart.rect,
    w: resizeStart.rect.w + (e.clientX - resizeStart.px),
    h: resizeStart.rect.h + (e.clientY - resizeStart.py),
  })
}

function onResizeEnd() {
  if (!resizeStart) return
  resizeStart = null
  resizing.value = false
  persistRect()
}

// 视口尺寸变化：窗口夹回可视区（含移动端形态切换）
function onViewportChange() {
  isMobile.value = mobileQuery.matches
  if (!isMobile.value) winRect.value = clampRect(winRect.value)
}

// Esc 关闭（非模态窗口的键盘出口）
function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.visible) close()
}

watch(
  () => props.visible,
  async (v) => {
    if (v) {
      // 打开窗口默认回到对话视图（上次关在设置页不该直接落在设置）
      activeView.value = 'chat'
      restore()
      restoreRect()
      await fetchSettings()
      await scrollToBottom()
      await nextTick()
      autoGrow()
    }
  },
  { immediate: true }
)

onMounted(() => {
  mobileQuery.addEventListener('change', onViewportChange)
  window.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  mobileQuery.removeEventListener('change', onViewportChange)
  window.removeEventListener('keydown', onKeydown)
})

// ── 配置面板 ────────────────────────────────────────────────

interface AssistantConfigForm {
  ai_assistant_enabled: boolean
  ai_assistant_base_url: string
  ai_assistant_endpoint_mode: string
  ai_assistant_api_key: string
  ai_assistant_model: string
  ai_assistant_system_prompt: string
  ai_assistant_max_tokens: number
  ai_assistant_temperature: number
  ai_assistant_timeout_ms: number
  ai_assistant_history_limit: number
  ai_assistant_rate_limit_per_min: number
}

const configLoading = ref(false)
const configSaving = ref(false)
const testing = ref(false)
const testResult = ref<{ ok: boolean; latency_ms: number; reply: string; error: string; model: string } | null>(null)

function defaultConfig(): AssistantConfigForm {
  return {
    ai_assistant_enabled: false,
    ai_assistant_base_url: '',
    ai_assistant_endpoint_mode: 'base',
    ai_assistant_api_key: '',
    ai_assistant_model: '',
    ai_assistant_system_prompt: '',
    ai_assistant_max_tokens: 4096,
    ai_assistant_temperature: 0.7,
    ai_assistant_timeout_ms: 120000,
    ai_assistant_history_limit: 20,
    ai_assistant_rate_limit_per_min: 30,
  }
}

const configForm = reactive<AssistantConfigForm>(defaultConfig())

async function loadConfig() {
  configLoading.value = true
  try {
    const res: any = await request.get('/admin/ai-assistant/config')
    const list: Array<{ key: string; value: any }> = res.data?.data?.list || []
    const next = defaultConfig()
    for (const item of list) {
      if (item.key in next && item.key !== 'ai_assistant_api_key') {
        ;(next as any)[item.key] = item.value
      }
    }
    // 密钥掩码不回填表单：留空表示保持现值
    next.ai_assistant_api_key = ''
    Object.assign(configForm, next)
  } catch {
    // 错误提示由拦截器统一弹出
  } finally {
    configLoading.value = false
  }
}

async function saveConfig() {
  configSaving.value = true
  try {
    const payload: Record<string, unknown> = { ...configForm }
    // 密钥为空 = 保持现值：不发送该字段，避免把密钥清空
    if (!configForm.ai_assistant_api_key) delete payload.ai_assistant_api_key
    await request.put('/admin/ai-assistant/config', { settings: payload })
    Message.success('配置已保存')
    configForm.ai_assistant_api_key = ''
    await fetchSettings(true)
    await loadConfig()
  } catch {
    // ignore
  } finally {
    configSaving.value = false
  }
}

async function testConnection() {
  testing.value = true
  testResult.value = null
  try {
    const res: any = await request.post('/admin/ai-assistant/test')
    testResult.value = res.data?.data || null
  } catch {
    // ignore
  } finally {
    testing.value = false
  }
}

watch(activeView, (v) => {
  if (v === 'config' && !configLoading.value) loadConfig()
})
</script>

<template>
  <teleport to="body">
    <div v-if="visible" class="ai-win" :class="{ 'ai-win--dragging': dragging || resizing }" :style="winStyle">
      <!-- 头部：拖动区 -->
      <div
        class="ai-head"
        @pointerdown.prevent="onDragStart"
        @pointermove="onDragMove"
        @pointerup="onDragEnd"
        @pointercancel="onDragEnd"
      >
        <div class="ai-head__left">
          <button
            v-if="activeView === 'config'"
            type="button"
            class="ai-head__btn ai-head__btn--back"
            title="返回对话"
            @click="activeView = 'chat'"
          >
            <IconLeft />
          </button>
          <span v-else class="ai-head__mark"><IconRobot /></span>
          <div class="ai-head__titles">
            <span class="ai-head__title">{{ activeView === 'config' ? '助手设置' : 'AI 助手' }}</span>
            <span class="ai-head__sub">
              {{ activeView === 'config' ? '仅对当前管理员生效' : '运营问答 · 文案起草 · 排障' }}
            </span>
          </div>
        </div>
        <div class="ai-head__actions">
          <button
            v-if="activeView === 'chat' && messages.length > 0"
            type="button"
            class="ai-head__btn"
            title="新对话"
            @click="clearAll"
          >
            <IconRefresh />
          </button>
          <button
            v-if="activeView === 'chat' && canConfig"
            type="button"
            class="ai-head__btn"
            title="助手设置"
            @click="activeView = 'config'"
          >
            <IconSettings />
          </button>
          <button type="button" class="ai-head__btn" title="关闭 (Esc)" @click="close">
            <IconClose />
          </button>
        </div>
      </div>

      <!-- 设置视图：占据整个内容区（不挤压对话），表单滚动 + 底部固定操作栏 -->
      <div v-if="activeView === 'config' && canConfig" class="ai-config">
        <ASpin :loading="configLoading" class="ai-config__spin">
          <div class="cfg">
            <!-- 总开关 -->
            <section class="cfg-card">
              <div class="cfg-switch">
                <div class="cfg-switch__text">
                  <p class="cfg-switch__title">启用 AI 助手</p>
                  <p class="cfg-switch__desc">关闭后所有管理员都无法使用该助手</p>
                </div>
                <ASwitch v-model="configForm.ai_assistant_enabled" />
              </div>
            </section>

            <!-- 连接配置 -->
            <section class="cfg-card">
              <h3 class="cfg-card__title">连接配置</h3>
              <div class="cfg-field">
                <span class="cfg-label">端点模式</span>
                <ARadioGroup v-model="configForm.ai_assistant_endpoint_mode">
                  <ARadio value="base">基地址（自动拼 /chat/completions）</ARadio>
                  <ARadio value="full">完整地址</ARadio>
                </ARadioGroup>
                <p class="cfg-hint">
                  {{ configForm.ai_assistant_endpoint_mode === 'base'
                    ? '只填到 /v1 这一级，请求路径由服务端拼接。'
                    : '直接填写完整的请求地址，服务端不再拼接路径。' }}
                </p>
              </div>
              <div class="cfg-field">
                <span class="cfg-label">端点地址</span>
                <AInput
                  v-model="configForm.ai_assistant_base_url"
                  :placeholder="configForm.ai_assistant_endpoint_mode === 'base' ? '如 https://api.example.com/v1' : '完整请求地址'"
                />
              </div>
              <div class="cfg-field">
                <span class="cfg-label">密钥</span>
                <AInput
                  v-model="configForm.ai_assistant_api_key"
                  type="password"
                  autocomplete="new-password"
                  placeholder="留空保持现值"
                />
              </div>
              <div class="cfg-field">
                <span class="cfg-label">模型</span>
                <AInput v-model="configForm.ai_assistant_model" placeholder="如 deepseek-chat、gpt-4o-mini" />
              </div>
            </section>

            <!-- 生成参数 -->
            <section class="cfg-card">
              <h3 class="cfg-card__title">生成参数</h3>
              <div class="cfg-grid cfg-grid--3">
                <div class="cfg-cell">
                  <span class="cfg-label">输出上限 <em>tokens</em></span>
                  <AInputNumber v-model="configForm.ai_assistant_max_tokens" :min="64" :max="128000" hide-button />
                </div>
                <div class="cfg-cell">
                  <span class="cfg-label">温度 <em>0–2</em></span>
                  <AInputNumber v-model="configForm.ai_assistant_temperature" :min="0" :max="2" :step="0.1" hide-button />
                </div>
                <div class="cfg-cell">
                  <span class="cfg-label">超时 <em>ms</em></span>
                  <AInputNumber v-model="configForm.ai_assistant_timeout_ms" :min="1000" :max="600000" :step="1000" hide-button />
                </div>
              </div>
            </section>

            <!-- 会话与限流 -->
            <section class="cfg-card">
              <h3 class="cfg-card__title">会话与限流</h3>
              <div class="cfg-grid cfg-grid--2">
                <div class="cfg-cell">
                  <span class="cfg-label">历史上限 <em>条</em></span>
                  <AInputNumber v-model="configForm.ai_assistant_history_limit" :min="2" :max="200" hide-button />
                </div>
                <div class="cfg-cell">
                  <span class="cfg-label">限流 <em>次/分</em></span>
                  <AInputNumber v-model="configForm.ai_assistant_rate_limit_per_min" :min="0" :max="1000" hide-button />
                </div>
              </div>
              <p class="cfg-hint">历史上限决定每轮带入的对话条数；限流按管理员计，0 表示不限。</p>
            </section>

            <!-- 系统提示词 -->
            <section class="cfg-card">
              <h3 class="cfg-card__title">系统提示词</h3>
              <ATextarea
                v-model="configForm.ai_assistant_system_prompt"
                :auto-size="{ minRows: 3, maxRows: 8 }"
                placeholder="自定义助手人设与规则，留空使用内置默认"
              />
              <p class="cfg-hint">留空时使用内置人设；填写后整体覆盖内置规则。</p>
            </section>

            <!-- 连通性结果 -->
            <div
              v-if="testResult"
              class="cfg-result"
              :class="testResult.ok ? 'cfg-result--ok' : 'cfg-result--fail'"
            >
              <span class="cfg-result__icon">
                <IconCheckCircleFill v-if="testResult.ok" />
                <IconCloseCircleFill v-else />
              </span>
              <div class="cfg-result__text">
                <p class="cfg-result__title">
                  {{ testResult.ok ? '连通正常' : '连通失败' }}
                  <template v-if="testResult.ok">· {{ testResult.model }} · {{ testResult.latency_ms }}ms</template>
                </p>
                <p class="cfg-result__body">{{ testResult.ok ? testResult.reply : testResult.error }}</p>
              </div>
            </div>

            <p class="cfg-note">测试使用「已保存」的配置发起最小补全；技能参考与校验在服务端本地完成，对话内容不进租户审计。</p>
          </div>
        </ASpin>
        <!-- 固定操作栏：不随表单滚动 -->
        <div class="cfg-bar">
          <span class="cfg-bar__hint">修改后需保存生效</span>
          <div class="cfg-bar__actions">
            <AButton :loading="testing" @click="testConnection">测试连通性</AButton>
            <AButton type="primary" :loading="configSaving" @click="saveConfig">
              <template #icon><IconCheck /></template>
              保存配置
            </AButton>
          </div>
        </div>
      </div>

      <!-- 对话视图 -->
      <template v-if="activeView === 'chat'">
        <!-- 消息列表 -->
        <div ref="listEl" class="ai-list">
          <div v-if="messages.length === 0" class="ai-empty">
            <span class="ai-empty__mark"><IconRobot /></span>
            <template v-if="assistantDisabled">
              <p class="ai-empty__title">AI 助手未启用</p>
              <p class="ai-empty__desc">请联系拥有「AI 助手配置」权限的管理员开启。</p>
              <button v-if="canConfig" type="button" class="ai-empty__btn" @click="activeView = 'config'">去配置</button>
            </template>
            <template v-else>
              <p class="ai-empty__title">有什么可以帮你？</p>
              <p class="ai-empty__desc">
                运营问答、文案起草、排障思路都可以问；激活下方技能还能生成计费表达式与 JS 插件代码，并自动走上线同款校验。
              </p>
              <div class="ai-empty__asks">
                <button v-for="q in QUICK_ASKS" :key="q" type="button" class="ai-ask" @click="quickAsk(q)">
                  {{ q }}
                </button>
              </div>
            </template>
          </div>

          <div v-for="msg in messages" :key="msg.id" class="ai-row" :class="`ai-row--${msg.role}`">
            <span v-if="msg.role === 'assistant'" class="ai-row__avatar"><IconRobot /></span>
            <div class="ai-row__body">
              <div v-if="msg.role === 'user' && msg.skills?.length" class="ai-row__skills">
                <span v-for="s in msg.skills" :key="s" class="ai-chip">{{ skillLabel(s) }}</span>
              </div>
              <template v-for="(part, idx) in msg.parts" :key="idx">
                <!-- 正文：助手走 markdown；用户消息按纯文本插值（不过 marked，避免 <p> 默认外边距
                     把单行气泡撑高，也避免用户输入的 * / # 被当成语法） -->
                <template v-if="part.type === 'text' && part.content">
                  <div v-if="msg.role === 'user'" class="ai-bubble">{{ part.content }}</div>
                  <div v-else class="ai-bubble md-body" v-html="renderMarkdown(part.content)" />
                </template>
                <!-- 代码：专用通道渲染，不经 markdown（防反引号拆碎源码） -->
                <div v-else-if="part.type === 'code'" class="ai-code">
                  <div class="ai-code__head">
                    <span class="ai-code__lang">{{ part.lang === 'formula' ? '计费表达式' : (part.lang || '代码') }}</span>
                    <button type="button" class="ai-code__btn" @click="toggleExpand(`${msg.id}:${idx}`)">
                      {{ partCollapsed(msg, idx) ? '展开' : '收起' }}
                    </button>
                    <button type="button" class="ai-code__icon" title="下载" @click="downloadCode(part)">
                      <IconDownload />
                    </button>
                    <button type="button" class="ai-code__icon" title="复制" @click="copyText(part.content)">
                      <IconCopy />
                    </button>
                  </div>
                  <pre v-show="!partCollapsed(msg, idx)" class="ai-code__pre"><code>{{ part.content }}</code></pre>
                  <!-- 校验报告 -->
                  <div v-if="part.report" class="ai-report" :class="part.report.ok ? 'ai-report--ok' : 'ai-report--fail'">
                    <div class="ai-report__title">
                      {{ part.report.ok ? '✓ 校验通过' : '✗ 校验未通过' }}
                      <span v-if="part.report.skill === 'billing_formula'" class="ai-report__sub">（与管理端「校验并试算」同口径）</span>
                      <span v-else class="ai-report__sub">（与插件上传自检同口径）</span>
                    </div>
                    <div v-if="part.report.error" class="ai-report__error">{{ part.report.error }}</div>
                    <!-- 公式：烟测试算表 -->
                    <table v-if="part.report.preview?.length" class="ai-report__table">
                      <thead>
                        <tr><th>样例向量</th><th>价格系数</th></tr>
                      </thead>
                      <tbody>
                        <tr v-for="p in part.report.preview" :key="p.name">
                          <td>{{ p.name }}</td>
                          <td>{{ p.factor }}</td>
                        </tr>
                      </tbody>
                    </table>
                    <!-- 插件：检查项清单 -->
                    <ul v-if="part.report.checks?.length" class="ai-report__checks">
                      <li v-for="c in part.report.checks" :key="c.name + c.detail" :class="`ai-report__check--${c.status}`">
                        <b>{{ c.name }}</b> {{ c.detail }}
                      </li>
                    </ul>
                    <button
                      v-if="!streaming && reportNeedsRepair(part.report)"
                      type="button"
                      class="ai-report__repair"
                      @click="onRepair(part.report!)"
                    >让 AI 修复</button>
                  </div>
                </div>
              </template>

              <!-- 等待首 token -->
              <div v-if="msg.id === typingMsgId" class="ai-typing"><i /><i /><i /></div>

              <div v-if="msg.error" class="ai-row__error">
                <IconCloseCircleFill />
                <span>{{ msg.error }}</span>
              </div>
              <div v-if="msg.usage?.total_tokens" class="ai-row__meta">
                <span>{{ msg.usage.model }}</span>
                <i class="ai-row__dot" />
                <span>{{ msg.usage.total_tokens }} tokens</span>
              </div>
            </div>
          </div>
        </div>

        <!-- 输入区 -->
        <div class="ai-input">
          <div class="ai-composer">
            <div class="ai-composer__top">
              <span class="ai-composer__label">技能</span>
              <button
                v-for="s in SKILL_OPTIONS"
                :key="s.key"
                type="button"
                class="ai-skill"
                :class="{ 'ai-skill--on': activeSkills.includes(s.key) }"
                :disabled="streaming"
                @click="toggleSkill(s.key)"
              >
                <IconCheck v-if="activeSkills.includes(s.key)" class="ai-skill__tick" />
                {{ s.label }}
              </button>
            </div>
            <div class="ai-composer__main">
              <textarea
                ref="textareaEl"
                v-model="input"
                class="ai-composer__textarea"
                rows="1"
                placeholder="输入问题…"
                :disabled="assistantDisabled && !canConfig"
                @input="autoGrow"
                @keydown.enter.exact.prevent="onSend"
              />
              <button v-if="streaming" type="button" class="ai-send ai-send--stop" title="停止生成" @click="stop">
                <IconPause />
              </button>
              <button v-else type="button" class="ai-send" :disabled="!canSend" title="发送 (Enter)" @click="onSend">
                <IconSend />
              </button>
            </div>
            <div class="ai-composer__foot">
              <span>Enter 发送 · Shift + Enter 换行</span>
              <span v-if="activeSkills.length">{{ activeSkills.length }} 个技能已激活</span>
            </div>
          </div>
        </div>
      </template>

      <!-- 缩放手柄（桌面端） -->
      <div
        v-if="!isMobile"
        class="ai-win__resize"
        @pointerdown.prevent="onResizeStart"
        @pointermove="onResizeMove"
        @pointerup="onResizeEnd"
        @pointercancel="onResizeEnd"
      />
    </div>
  </teleport>
</template>

<style scoped>
/* 浮动窗口：非模态，无遮罩，z-index 压过顶栏 */
.ai-win {
  position: fixed;
  z-index: 1100;
  display: flex;
  flex-direction: column;
  background: var(--ta-bg-card);
  border: 1px solid var(--ta-border);
  border-radius: 16px;
  box-shadow:
    0 18px 56px -12px rgba(15, 23, 42, 0.26),
    0 4px 14px rgba(15, 23, 42, 0.06);
  overflow: hidden;
  /* Arco 主题变量是 RGB 三元组，全局未覆盖（仍是 arcoblue）：
     这里把窗口内 Arco 控件（Switch/Radio/Button/Spin/Input focus）统一到品牌 teal，
     避免"teal 气泡 + 蓝色开关"的配色打架 */
  --primary-5: 20, 184, 166;
  --primary-6: 13, 148, 136;
  --primary-7: 15, 118, 110;
}
.ai-win--dragging {
  user-select: none;
  cursor: grabbing;
}

/* 头部（拖动区） */
.ai-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px 10px 14px;
  border-bottom: 1px solid var(--ta-border);
  background: var(--ta-bg-card);
  flex-shrink: 0;
  cursor: grab;
  touch-action: none; /* pointer 拖动时阻止触摸滚动 */
  user-select: none;
}
.ai-head:active {
  cursor: grabbing;
}
.ai-head__left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.ai-head__mark {
  width: 30px;
  height: 30px;
  flex-shrink: 0;
  border-radius: 9px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  color: #fff;
  background: var(--ta-primary-gradient);
  box-shadow: 0 2px 8px color-mix(in srgb, var(--ta-primary) 32%, transparent);
}
.ai-head__titles {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.ai-head__title {
  font-size: 14px;
  font-weight: 600;
  line-height: 1.25;
  color: var(--ta-text-primary);
}
.ai-head__sub {
  font-size: 11px;
  line-height: 1.25;
  color: var(--ta-text-tertiary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.ai-head__actions {
  display: flex;
  gap: 2px;
  cursor: default;
  flex-shrink: 0;
}
.ai-head__btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--ta-text-tertiary);
  font-size: 15px;
  cursor: pointer;
  transition: background var(--ta-duration-fast) var(--ta-ease), color var(--ta-duration-fast) var(--ta-ease);
}
.ai-head__btn:hover {
  background: var(--ta-bg-secondary);
  color: var(--ta-text-primary);
}
.ai-head__btn--back {
  border: 1px solid var(--ta-border);
  background: var(--ta-bg-card);
  color: var(--ta-text-secondary);
}
.ai-head__btn--back:hover {
  border-color: color-mix(in srgb, var(--ta-primary) 40%, transparent);
  background: color-mix(in srgb, var(--ta-primary) 8%, transparent);
  color: var(--ta-primary);
}

/* 缩放手柄 */
.ai-win__resize {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 18px;
  height: 18px;
  cursor: nwse-resize;
  touch-action: none;
  background:
    linear-gradient(135deg, transparent 50%, var(--ta-border) 50%) no-repeat,
    linear-gradient(135deg, transparent 65%, var(--ta-border) 65%) no-repeat;
  background-size: 10px 10px, 10px 10px;
  background-position: bottom right;
}

/* ── 设置视图 ─────────────────────────────────────────── */
.ai-config {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--ta-bg-page);
}
.ai-config__spin {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: color-mix(in srgb, var(--ta-text-tertiary) 40%, transparent) transparent;
}
.ai-config__spin::-webkit-scrollbar {
  width: 8px;
}
.ai-config__spin::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, var(--ta-text-tertiary) 35%, transparent);
  background-clip: content-box;
  border: 2px solid transparent;
  border-radius: 999px;
}
.cfg {
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.cfg-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 12px 14px;
  border-radius: 12px;
  border: 1px solid var(--ta-border);
  background: var(--ta-bg-card);
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.03);
}
.cfg-card__title {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--ta-text-primary);
}
.cfg-switch {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.cfg-switch__title {
  margin: 0 0 3px;
  font-size: 13px;
  font-weight: 600;
  color: var(--ta-text-primary);
}
.cfg-switch__desc {
  margin: 0;
  font-size: 11.5px;
  color: var(--ta-text-tertiary);
}
.cfg-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.cfg-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--ta-text-secondary);
}
.cfg-label em {
  font-style: normal;
  font-size: 11px;
  color: var(--ta-text-tertiary);
}
.cfg-hint {
  margin: 0;
  font-size: 11px;
  line-height: 1.6;
  color: var(--ta-text-tertiary);
}
.cfg-field :deep(.arco-input-wrapper),
.cfg-field :deep(.arco-textarea-wrapper) {
  width: 100%;
  border-radius: 8px;
}
.cfg-grid {
  display: grid;
  gap: 10px;
}
.cfg-grid--3 {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}
.cfg-grid--2 {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.cfg-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
.cfg-cell :deep(.arco-input-number) {
  width: 100%;
  border-radius: 8px;
}
.cfg-result {
  display: flex;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid transparent;
  font-size: 12px;
  line-height: 1.6;
}
.cfg-result--ok {
  background: color-mix(in srgb, var(--ta-success) 8%, transparent);
  border-color: color-mix(in srgb, var(--ta-success) 22%, transparent);
}
.cfg-result--ok .cfg-result__icon {
  color: var(--ta-success);
}
.cfg-result--fail {
  background: color-mix(in srgb, var(--ta-danger) 7%, transparent);
  border-color: color-mix(in srgb, var(--ta-danger) 20%, transparent);
}
.cfg-result--fail .cfg-result__icon {
  color: var(--ta-danger);
}
.cfg-result__icon {
  flex-shrink: 0;
  font-size: 14px;
  line-height: 1.4;
}
.cfg-result__text {
  min-width: 0;
}
.cfg-result__title {
  margin: 0;
  font-weight: 600;
  color: var(--ta-text-primary);
}
.cfg-result__body {
  margin: 3px 0 0;
  font-size: 11.5px;
  color: var(--ta-text-secondary);
  word-break: break-word;
}
.cfg-note {
  margin: 0;
  padding: 0 2px;
  font-size: 11px;
  line-height: 1.7;
  color: var(--ta-text-tertiary);
}
.cfg-bar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 14px;
  background: var(--ta-bg-card);
  border-top: 1px solid var(--ta-border);
}
.cfg-bar__hint {
  font-size: 11px;
  color: var(--ta-text-tertiary);
}
.cfg-bar__actions {
  display: flex;
  gap: 8px;
}

/* ── 消息列表 ─────────────────────────────────────────── */
.ai-list {
  flex: 1;
  min-height: 0; /* flex 子项允许收缩滚动 */
  overflow-y: auto;
  padding: 18px 14px 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  background: var(--ta-bg-page);
  scrollbar-width: thin;
  scrollbar-color: color-mix(in srgb, var(--ta-text-tertiary) 40%, transparent) transparent;
}
.ai-list::-webkit-scrollbar {
  width: 8px;
}
.ai-list::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, var(--ta-text-tertiary) 35%, transparent);
  background-clip: content-box;
  border: 2px solid transparent;
  border-radius: 999px;
}
.ai-list::-webkit-scrollbar-thumb:hover {
  background: color-mix(in srgb, var(--ta-text-tertiary) 55%, transparent);
  background-clip: content-box;
}

/* 空态 */
.ai-empty {
  margin: auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  max-width: 320px;
  padding: 8px 4px 20px;
}
.ai-empty__mark {
  width: 52px;
  height: 52px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 16px;
  margin-bottom: 14px;
  font-size: 24px;
  color: var(--ta-primary);
  background: color-mix(in srgb, var(--ta-primary) 10%, transparent);
  border: 1px solid color-mix(in srgb, var(--ta-primary) 18%, transparent);
}
.ai-empty__title {
  margin: 0 0 6px;
  font-size: 15px;
  font-weight: 600;
  color: var(--ta-text-primary);
}
.ai-empty__desc {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.75;
  color: var(--ta-text-tertiary);
}
.ai-empty__btn {
  margin-top: 14px;
  padding: 6px 16px;
  font-size: 12.5px;
  border-radius: 8px;
  border: 1px solid color-mix(in srgb, var(--ta-primary) 45%, transparent);
  background: color-mix(in srgb, var(--ta-primary) 8%, transparent);
  color: var(--ta-primary);
  cursor: pointer;
  transition: background var(--ta-duration-fast) var(--ta-ease);
}
.ai-empty__btn:hover {
  background: color-mix(in srgb, var(--ta-primary) 14%, transparent);
}
.ai-empty__asks {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  margin-top: 16px;
}
.ai-ask {
  text-align: left;
  padding: 9px 12px;
  font-size: 12.5px;
  line-height: 1.5;
  border-radius: 10px;
  border: 1px solid var(--ta-border);
  background: var(--ta-bg-card);
  color: var(--ta-text-secondary);
  cursor: pointer;
  transition: all var(--ta-duration-fast) var(--ta-ease);
}
.ai-ask:hover {
  border-color: color-mix(in srgb, var(--ta-primary) 45%, transparent);
  color: var(--ta-primary);
  box-shadow: 0 2px 10px rgba(15, 23, 42, 0.06);
  transform: translateY(-1px);
}

/* 消息行 */
.ai-row {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  animation: aiRowIn var(--ta-duration-normal) var(--ta-ease) both;
}
.ai-row--user {
  justify-content: flex-end;
}
.ai-row__avatar {
  flex-shrink: 0;
  width: 26px;
  height: 26px;
  margin-top: 2px;
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  color: #fff;
  background: var(--ta-primary-gradient);
  box-shadow: 0 2px 6px color-mix(in srgb, var(--ta-primary) 28%, transparent);
}
.ai-row__body {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
  max-width: calc(100% - 35px);
}
.ai-row--assistant .ai-row__body {
  flex: 1;
}
.ai-row--user .ai-row__body {
  align-items: flex-end;
  max-width: 80%;
}
.ai-row__skills {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 5px;
}

/* 气泡 */
.ai-bubble {
  padding: 9px 13px;
  border-radius: 14px;
  font-size: 13px;
  line-height: 1.72;
  word-break: break-word;
  overflow-wrap: anywhere;
}
.ai-row--user .ai-bubble {
  border-bottom-right-radius: 5px;
  /* 比 --ta-primary-gradient 更深一档：白字在渐变起点的对比度约 3.9:1（起点 2.4:1 太浅） */
  background: linear-gradient(135deg, var(--ta-primary), var(--ta-primary-hover));
  color: #fff;
  white-space: pre-wrap;
  box-shadow: 0 4px 12px -4px color-mix(in srgb, var(--ta-primary) 48%, transparent);
}
.ai-row--assistant .ai-bubble {
  border-top-left-radius: 5px;
  background: var(--ta-bg-card);
  border: 1px solid var(--ta-border);
  color: var(--ta-text-primary);
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
}

/* markdown 排版 */
.md-body :deep(p) {
  margin: 0 0 9px;
}
.md-body :deep(p:last-child) {
  margin-bottom: 0;
}
.md-body :deep(strong) {
  font-weight: 600;
  color: var(--ta-text-primary);
}
.md-body :deep(h1),
.md-body :deep(h2),
.md-body :deep(h3),
.md-body :deep(h4) {
  margin: 14px 0 8px;
  font-size: 13.5px;
  font-weight: 600;
  line-height: 1.5;
  color: var(--ta-text-primary);
}
.md-body :deep(h1:first-child),
.md-body :deep(h2:first-child),
.md-body :deep(h3:first-child),
.md-body :deep(h4:first-child) {
  margin-top: 0;
}
.md-body :deep(ul),
.md-body :deep(ol) {
  margin: 0 0 9px;
  padding-left: 20px;
}
.md-body :deep(li) {
  margin: 3px 0;
}
.md-body :deep(li)::marker {
  color: var(--ta-text-tertiary);
}
.md-body :deep(blockquote) {
  margin: 0 0 9px;
  padding: 6px 12px;
  border-left: 3px solid color-mix(in srgb, var(--ta-primary) 40%, transparent);
  border-radius: 0 8px 8px 0;
  background: color-mix(in srgb, var(--ta-primary) 6%, transparent);
  color: var(--ta-text-secondary);
}
.md-body :deep(a) {
  color: var(--ta-primary);
  text-decoration: underline;
  text-underline-offset: 2px;
}
.md-body :deep(hr) {
  margin: 12px 0;
  border: none;
  border-top: 1px solid var(--ta-border);
}
.md-body :deep(code) {
  padding: 1px 5px;
  border-radius: 5px;
  font-size: 12px;
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  background: var(--ta-bg-secondary);
}
.md-body :deep(pre) {
  margin: 0 0 9px;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid var(--ta-border);
  background: var(--ta-bg-secondary);
  overflow-x: auto;
  font-size: 12px;
  line-height: 1.65;
}
.md-body :deep(pre code) {
  padding: 0;
  background: none;
}
.md-body :deep(table) {
  width: 100%;
  margin: 0 0 9px;
  border-collapse: collapse;
  font-size: 12px;
}
.md-body :deep(th),
.md-body :deep(td) {
  padding: 5px 8px;
  border: 1px solid var(--ta-border);
  text-align: left;
}
.md-body :deep(th) {
  background: var(--ta-bg-secondary);
  font-weight: 600;
}

/* 打字指示器 */
.ai-typing {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  width: fit-content;
  padding: 12px 14px;
  border-radius: 14px;
  border-top-left-radius: 5px;
  border: 1px solid var(--ta-border);
  background: var(--ta-bg-card);
}
.ai-typing i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ta-text-weakest, #cbd5e1);
  animation: aiDot 1.2s infinite ease-in-out;
}
.ai-typing i:nth-child(2) {
  animation-delay: 0.15s;
}
.ai-typing i:nth-child(3) {
  animation-delay: 0.3s;
}

/* 错误 / 用量 */
.ai-row__error {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  padding: 7px 10px;
  border-radius: 9px;
  font-size: 12px;
  line-height: 1.6;
  word-break: break-word;
  color: var(--ta-danger);
  background: color-mix(in srgb, var(--ta-danger) 7%, transparent);
  border: 1px solid color-mix(in srgb, var(--ta-danger) 18%, transparent);
}
.ai-row__error svg {
  flex-shrink: 0;
  margin-top: 1px;
  font-size: 13px;
}
.ai-row__meta {
  display: flex;
  align-items: center;
  gap: 6px;
  padding-left: 2px;
  font-size: 11px;
  color: var(--ta-text-tertiary);
}
.ai-row__dot {
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: currentColor;
  opacity: 0.6;
}

/* 技能 chip */
.ai-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  border-radius: 6px;
  border: 1px solid color-mix(in srgb, var(--ta-primary) 30%, transparent);
  background: color-mix(in srgb, var(--ta-primary) 8%, transparent);
  color: var(--ta-primary);
  font-size: 11px;
  line-height: 1.5;
}

/* 代码卡片：专用通道（不经 markdown） */
.ai-code {
  border: 1px solid var(--ta-border);
  border-radius: 12px;
  overflow: hidden;
  background: var(--ta-bg-card);
}
.ai-code__head {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 6px 8px 6px 12px;
  background: var(--ta-bg-secondary);
  border-bottom: 1px solid var(--ta-border);
}
.ai-code__lang {
  flex: 1;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--ta-text-secondary);
}
.ai-code__btn {
  padding: 2px 7px;
  border: none;
  border-radius: 5px;
  background: transparent;
  font-size: 11.5px;
  color: var(--ta-text-tertiary);
  cursor: pointer;
}
.ai-code__btn:hover {
  background: var(--ta-bg-card);
  color: var(--ta-text-primary);
}
.ai-code__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 5px;
  background: transparent;
  font-size: 13px;
  color: var(--ta-text-tertiary);
  cursor: pointer;
}
.ai-code__icon:hover {
  background: var(--ta-bg-card);
  color: var(--ta-primary);
}
.ai-code__pre {
  margin: 0;
  padding: 12px 14px;
  max-height: 420px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.65;
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  color: var(--ta-text-primary);
  white-space: pre;
}

/* 校验报告 */
.ai-report {
  padding: 10px 12px;
  border-top: 1px solid var(--ta-border);
  font-size: 12px;
}
.ai-report__title {
  margin-bottom: 6px;
  font-weight: 600;
}
.ai-report--ok .ai-report__title {
  color: var(--ta-success);
}
.ai-report--fail .ai-report__title {
  color: var(--ta-danger);
}
.ai-report__sub {
  font-weight: 400;
  font-size: 11px;
  color: var(--ta-text-tertiary);
}
.ai-report__error {
  margin-bottom: 6px;
  padding: 6px 10px;
  border-radius: 6px;
  word-break: break-all;
  color: var(--ta-danger);
  background: color-mix(in srgb, var(--ta-danger) 8%, transparent);
}
.ai-report__table {
  width: 100%;
  margin-top: 4px;
  border-collapse: collapse;
}
.ai-report__table th,
.ai-report__table td {
  padding: 4px 8px;
  border-bottom: 1px solid var(--ta-border);
  text-align: left;
  font-size: 12px;
}
.ai-report__checks {
  list-style: none;
  margin: 4px 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.ai-report__checks li {
  padding: 4px 8px;
  border-radius: 6px;
  background: var(--ta-bg-secondary);
}
.ai-report__check--fail {
  color: var(--ta-danger);
}
.ai-report__check--warn {
  color: var(--ta-warning);
}
.ai-report__check--pass b::before {
  content: '✓ ';
  color: var(--ta-success);
}
.ai-report__check--warn b::before {
  content: '△ ';
}
.ai-report__check--fail b::before {
  content: '✗ ';
}
.ai-report__repair {
  margin-top: 8px;
  padding: 5px 12px;
  font-size: 12px;
  border-radius: 7px;
  border: 1px solid color-mix(in srgb, var(--ta-primary) 45%, transparent);
  background: color-mix(in srgb, var(--ta-primary) 8%, transparent);
  color: var(--ta-primary);
  cursor: pointer;
}
.ai-report__repair:hover {
  background: color-mix(in srgb, var(--ta-primary) 14%, transparent);
}

/* ── 输入区 ───────────────────────────────────────────── */
.ai-input {
  flex-shrink: 0;
  padding: 10px 14px 12px;
  background: var(--ta-bg-card);
  border-top: 1px solid var(--ta-border);
}
.ai-composer {
  display: flex;
  flex-direction: column;
  padding: 8px 10px 6px;
  border-radius: 14px;
  border: 1px solid var(--ta-border);
  background: var(--ta-bg-card);
  transition: border-color var(--ta-duration-fast) var(--ta-ease), box-shadow var(--ta-duration-fast) var(--ta-ease);
}
.ai-composer:focus-within {
  border-color: color-mix(in srgb, var(--ta-primary) 55%, transparent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--ta-primary) 12%, transparent);
}
.ai-composer__top {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  padding: 0 2px 6px;
}
.ai-composer__label {
  font-size: 11px;
  color: var(--ta-text-tertiary);
}
.ai-skill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 5px 9px;
  border-radius: 6px;
  border: 1px solid var(--ta-border);
  background: var(--ta-bg-secondary);
  color: var(--ta-text-secondary);
  font-size: 11.5px;
  line-height: 1.2;
  cursor: pointer;
  transition: all var(--ta-duration-fast) var(--ta-ease);
}
.ai-skill:hover:not(:disabled) {
  border-color: color-mix(in srgb, var(--ta-primary) 40%, transparent);
  color: var(--ta-primary);
}
.ai-skill:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
.ai-skill--on {
  border-color: color-mix(in srgb, var(--ta-primary) 45%, transparent);
  background: color-mix(in srgb, var(--ta-primary) 10%, transparent);
  color: var(--ta-primary);
}
.ai-skill__tick {
  font-size: 11px;
}
.ai-composer__main {
  display: flex;
  align-items: flex-end;
  gap: 8px;
}
.ai-composer__textarea {
  flex: 1;
  min-height: 36px;
  max-height: 132px;
  padding: 7px 2px;
  border: none;
  outline: none;
  resize: none;
  background: transparent;
  font-family: inherit;
  font-size: 13px;
  line-height: 1.6;
  color: var(--ta-text-primary);
  scrollbar-width: thin;
}
.ai-composer__textarea::placeholder {
  color: var(--ta-text-tertiary);
}
.ai-composer__textarea:disabled {
  color: var(--ta-text-tertiary);
  cursor: not-allowed;
}
.ai-send {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  margin-bottom: 4px;
  border: none;
  border-radius: 10px;
  font-size: 16px;
  color: #fff;
  cursor: pointer;
  background: var(--ta-primary-gradient);
  box-shadow: 0 4px 10px -2px color-mix(in srgb, var(--ta-primary) 45%, transparent);
  transition: transform var(--ta-duration-fast) var(--ta-ease), box-shadow var(--ta-duration-fast) var(--ta-ease),
    background var(--ta-duration-fast) var(--ta-ease);
}
.ai-send:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 6px 14px -2px color-mix(in srgb, var(--ta-primary) 55%, transparent);
}
.ai-send:active:not(:disabled) {
  transform: translateY(0);
}
.ai-send:disabled {
  background: var(--ta-bg-secondary);
  color: var(--ta-text-weakest, #cbd5e1);
  box-shadow: none;
  cursor: not-allowed;
}
.ai-send--stop {
  background: var(--ta-danger);
  box-shadow: 0 4px 10px -2px color-mix(in srgb, var(--ta-danger) 40%, transparent);
}
.ai-composer__foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 4px 2px 0;
  font-size: 11px;
  color: var(--ta-text-tertiary);
}

@keyframes aiRowIn {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
@keyframes aiDot {
  0%,
  60%,
  100% {
    opacity: 0.35;
    transform: translateY(0);
  }
  30% {
    opacity: 1;
    transform: translateY(-3px);
  }
}
</style>
