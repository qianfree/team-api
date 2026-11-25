<script setup lang="ts">
// JS 插件生成工作台：大幅面左右分栏弹窗。
// 左栏多轮对话（js_plugin 技能固定激活，可上传文本类附件注入上下文），
// 右栏实时展示 AI 生成的 plugin.js 源码（版本切换 + 复制/下载 + 校验报告
// + 半自动修复），并可「用此源码上传」直接预填上传弹窗形成闭环。
// 复用全局助手同款链路：useAiAssistantChat（SSE/持久化/修复环）+ 后端
// /admin/ai-assistant/chat，会话以独立 storageKey 与全局助手隔离。
import { ref, computed, watch, nextTick } from 'vue'
import { Message, Modal, Select } from '@arco-design/web-vue'
import {
  IconAttachment, IconBulb, IconCheckCircle, IconClose, IconCloseCircle,
  IconCode, IconCopy, IconDownload, IconEdit, IconExclamationCircle, IconPause, IconRefresh,
  IconRobot, IconSend, IconUpload,
} from '@arco-design/web-vue/es/icon'
import { marked } from 'marked'
import hljs from 'highlight.js/lib/core'
import javascript from 'highlight.js/lib/languages/javascript'
import { sanitizeHtml } from '@/utils/sanitize'
import { hasPermission } from '@/utils/permission'
import { downloadTextFile } from '@/utils/download'
import { usePublicSettings } from '@/composables/usePublicSettings'
import {
  useAiAssistantChat,
  type ChatMsg,
  type MessagePart,
  type ValidateReport,
} from '@/components/aiassistant/useAiAssistantChat'

const props = defineProps<{
  visible: boolean
  /** 详情抽屉「AI 升级」载入的现有插件：打开时以此为基线开新会话（右栏显示现有源码，
   *  聊天描述升级需求，AI 经 read_code/edit_code 定点修改） */
  inject?: { key: string; version: string; source: string } | null
}>()
const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void
  (e: 'apply', source: string): void
}>()

const canConfig = hasPermission('ai_assistant:config')

const { settings: publicSettings, fetchSettings } = usePublicSettings()
const {
  messages,
  streaming,
  input,
  activeSkills,
  pendingFiles,
  canSend,
  restore,
  clearAll,
  send,
  stop,
  repair,
  continueCode,
  validatePart,
  persistNow,
  injectExistingPlugin,
  addFiles,
  removeFile,
  prettySize,
} = useAiAssistantChat({ storageKey: 'admin_plugin_gen_v1_', defaultSkills: ['js_plugin'] })

// 未启用判定：公共配置未开启（有配置权限的管理员可去顶部 AI 助手开启）
// 注意 settings 是 ref，script 里必须 .value 取值（模板才会自动解包）
const assistantDisabled = computed(() => publicSettings.value.ai_assistant_enabled !== true)

/** 空态引导问句：点一下填进输入框（不直接发送，留出改字空间）。
 *  JS 插件只接异步任务（提交 → 轮询 → 终态），不支持任务取消，也不做同步协议转换——
 *  引导问句不要许诺契约外的能力，否则模型会照着生成用不了的插件 */
const QUICK_ASKS = [
  '为某供应商生成任务插件：支持提交任务、查询进度',
  '参考我上传的 API 文档，生成对应的 JS 任务插件',
  '为视频模型生成插件：支持首尾帧与参考素材，可经标准端点 /v1/videos 提交',
]

/** 可见消息：隐藏的续写指令仍进上游上下文，但不渲染在会话里 */
const visibleMessages = computed(() => messages.value.filter((m) => !m.hidden))

// ── 版本收集：扫描全部消息里的 code parts，右栏按序号展示/切换 ──
interface CodeVersion {
  key: string // `${msgId}:${partIdx}`
  idx: number // 第 N 版（从 1 起）
  part: MessagePart
}

const codeVersions = computed<CodeVersion[]>(() => {
  const list: CodeVersion[] = []
  for (const m of messages.value) {
    m.parts.forEach((p, i) => {
      if (p.type === 'code') list.push({ key: `${m.id}:${i}`, idx: list.length + 1, part: p })
    })
  }
  return list
})

const codeIndexMap = computed(() => {
  const map = new Map<string, number>()
  codeVersions.value.forEach((v) => map.set(v.key, v.idx))
  return map
})

// 选中版本：0 = 跟随最新（默认）；>0 = 锁定历史版本
const selectedIdx = ref(0)
const currentVersion = computed<CodeVersion | null>(() => {
  const list = codeVersions.value
  if (!list.length) return null
  if (selectedIdx.value === 0 || selectedIdx.value > list.length) return list[list.length - 1]
  return list[selectedIdx.value - 1]
})

const isLatestSelected = computed(() => {
  const list = codeVersions.value
  return !list.length || currentVersion.value === list[list.length - 1]
})

const versionOptions = computed(() =>
  codeVersions.value.map((v) => ({
    label: `第 ${v.idx} 版${v.idx === codeVersions.value.length ? '（最新）' : ''}`,
    value: v.idx,
  })),
)

// 按需注册（core + javascript，不引入全量语言包与官方主题——token 配色走 main.css 双主题）
hljs.registerLanguage('javascript', javascript)

const currentSource = computed(() => currentVersion.value?.part.content || '')
const lineCount = computed(() =>
  currentSource.value ? currentSource.value.split(/\r?\n/).length : 0,
)

/** 右栏源码高亮：ignoreIllegals 容忍流式/截断的不完整片段；
 *  hljs 输出中的代码文本已转义（& < >），v-html 安全 */
const highlightedSource = computed(() => {
  if (!currentSource.value) return ''
  try {
    return hljs.highlight(currentSource.value, { language: 'javascript', ignoreIllegals: true }).value
  } catch {
    return currentSource.value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  }
})

/** 校验统计：报告标题右侧的 通过/警告/未通过 计数 */
const reportStats = computed(() => {
  const checks = currentVersion.value?.part.report?.checks || []
  return {
    pass: checks.filter((c) => c.status === 'pass').length,
    warn: checks.filter((c) => c.status === 'warn').length,
    fail: checks.filter((c) => c.status === 'fail').length,
  }
})

// 新一轮生成开始（新 code part 出现）时自动回到最新版
watch(
  () => codeVersions.value.length,
  (n, o) => {
    selectedIdx.value = 0
    // 版本集合变化（新生成/切换目标）：编辑态失效，避免 v-model 写到已被替换的版本
    if (n !== (o || 0)) editing.value = false
  },
)

// ── 手动编辑：切换式（默认高亮只读，编辑时纯文本 textarea）──────────────
// 轻量场景（改版本号/小修）不引入编辑器依赖；textarea 无法内嵌高亮，代价是编辑态无高亮
const editing = ref(false)

/** 生成中/版本切换时强制退出编辑（内容正在被替换，就地编辑会写错对象） */
watch(streaming, (s) => {
  if (s) editing.value = false
})
watch(
  () => currentVersion.value?.key,
  () => {
    editing.value = false
  },
)
// 编辑态退出即落盘：无论从哪条路径退出（完成/切版本/生成开始），手改内容都不丢
watch(editing, (now, was) => {
  if (was && !now) persistNow()
})

function startEdit() {
  const v = currentVersion.value
  if (!v || streaming.value) return
  // 内容即将变化：旧校验报告不再可信，进入编辑即作废
  v.part.report = undefined
  editing.value = true
  nextTick(() => {
    const el = editEl.value
    if (el) {
      el.focus()
      // 光标落到末尾（改版本号等小修多在文件头部，但全选感太强，末尾最中性）
      el.setSelectionRange(el.value.length, el.value.length)
    }
  })
}

/** 完成编辑：退出编辑态并对手改后的全文重新校验（报告直接呈现） */
async function finishEdit() {
  const v = currentVersion.value
  editing.value = false
  if (!v || !v.part.content.trim()) return
  validating.value = true
  try {
    await validatePart(v.part)
  } finally {
    validating.value = false
  }
}

/** 手动校验进行中（编辑完成按钮的反馈态） */
const validating = ref(false)

/** 编辑区 Tab 键插入两个空格（默认行为会跳出焦点，改代码时很打断） */
function onEditKeydown(ev: KeyboardEvent) {
  if (ev.key !== 'Tab') return
  ev.preventDefault()
  const el = ev.target as HTMLTextAreaElement
  const { selectionStart: s, selectionEnd: e, value } = el
  el.value = value.slice(0, s) + '  ' + value.slice(e)
  el.setSelectionRange(s + 2, s + 2)
  // 同步回响应式数据（绕过 v-model 的事件时机）
  const v = currentVersion.value
  if (v) v.part.content = el.value
}

const editEl = ref<HTMLTextAreaElement | null>(null)

// ── 渲染与工具 ────────────────────────────────────────────

function renderMarkdown(text: string): string {
  if (!text) return ''
  return sanitizeHtml(marked.parse(text) as string)
}

/** 工具调用的展示名（agent 循环里的 write/append/edit/validate） */
function toolLabel(name: string): string {
  switch (name) {
    case 'write_code': return '写入代码'
    case 'append_code': return '追加代码'
    case 'edit_code': return '修改代码'
    case 'validate_code': return '校验'
    default: return name || '工具调用'
  }
}

/** 助手等待首个 token：气泡位置显示打字指示器 */
const typingMsgId = computed(() => {
  if (!streaming.value) return 0
  const last = messages.value[messages.value.length - 1]
  if (!last || last.role !== 'assistant') return 0
  return last.parts.some((p) => p.content.length > 0) ? 0 : last.id
})

// ── 思考过程（推理型模型的 reasoning）────────────────────
// 正文/代码一旦开始输出，思考块自动收起；用户手动点开则以手动状态为准。
const thinkingLiveId = computed(() => {
  if (!streaming.value) return 0
  const last = messages.value[messages.value.length - 1]
  if (!last || last.role !== 'assistant' || !last.thinking) return 0
  return last.parts.some((p) => p.content.length > 0) ? 0 : last.id
})

const thinkOpen = ref<Record<number, boolean>>({})

function isThinkOpen(msg: ChatMsg): boolean {
  const stored = thinkOpen.value[msg.id]
  return stored === undefined ? msg.id === thinkingLiveId.value : stored
}

function toggleThink(msg: ChatMsg) {
  thinkOpen.value[msg.id] = !isThinkOpen(msg)
}

// 思考过程中的正文自动跟底（只跟正在思考的那条）
const thinkEl = ref<HTMLElement | null>(null)
function setThinkEl(el: unknown) {
  thinkEl.value = (el as HTMLElement) || null
}
watch(
  () => messages.value[messages.value.length - 1]?.thinking?.length || 0,
  async () => {
    if (!thinkingLiveId.value) return
    await nextTick()
    const el = thinkEl.value
    if (el) el.scrollTop = el.scrollHeight
  },
)

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    Message.success('已复制')
  } catch {
    Message.error('复制失败')
  }
}

function downloadCode() {
  if (!currentSource.value) return
  downloadTextFile(currentSource.value, 'plugin.js', 'text/javascript;charset=utf-8')
}

/** 用当前选中版本的源码预填上传弹窗（生成器保持打开，上传弹窗叠加在上层） */
function applySource() {
  if (!currentSource.value.trim()) return
  emit('apply', currentSource.value)
  Message.success('已带入上传弹窗，可直接试运行或上传')
}

function reportNeedsRepair(report?: ValidateReport): boolean {
  if (!report || report.ok) return false
  return Boolean(report.error) || (report.checks || []).some((c) => c.status === 'fail')
}

/** 续写被截断的代码：增量并回当前版本，结束后自动重新校验合并结果 */
async function onContinue() {
  const v = currentVersion.value
  if (!v || streaming.value) return
  await continueCode(v.part)
  await scrollChatToBottom()
  await scrollSrcToBottom()
}

async function onRepair(report: ValidateReport) {
  selectedIdx.value = 0
  await repair(report)
  await scrollChatToBottom()
}

// ── 滚动跟随 ─────────────────────────────────────────────

const listEl = ref<HTMLElement | null>(null)
const srcEl = ref<HTMLElement | null>(null)

async function scrollChatToBottom() {
  await nextTick()
  const el = listEl.value
  if (el) el.scrollTop = el.scrollHeight
}

async function scrollSrcToBottom() {
  await nextTick()
  const el = srcEl.value
  if (el) el.scrollTop = el.scrollHeight
}

// 左栏：新消息入列 / 流式期间跟底
watch(
  () => messages.value[messages.value.length - 1]?.parts.length,
  () => scrollChatToBottom(),
)
/** 流式进度计数：所有 part 正文 + 思考过程的总字符数（比逐 token 拼字符串便宜） */
const streamTick = computed(() => {
  let n = 0
  for (const m of messages.value) {
    n += m.thinking?.length || 0
    for (const p of m.parts) n += p.content.length
  }
  return n
})

watch(streamTick, () => {
  if (!streaming.value) return
  scrollChatToBottom()
  // 右栏跟随流式源码（仅当正在看最新版时）
  if (isLatestSelected.value) scrollSrcToBottom()
})

// ── 输入区 ───────────────────────────────────────────────

const textareaEl = ref<HTMLTextAreaElement | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const MAX_INPUT_H = 132

function autoGrow() {
  const el = textareaEl.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, MAX_INPUT_H)}px`
}

watch(input, () => nextTick(autoGrow))

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

async function onSend() {
  if (!canSend.value) return
  await send()
}

function pickFiles() {
  fileInput.value?.click()
}

function onFileChange(ev: Event) {
  const input = ev.target as HTMLInputElement
  if (input.files?.length) addFiles(input.files)
  input.value = ''
}

// 拖拽附件进输入区
function onComposerDrop(ev: DragEvent) {
  ev.preventDefault()
  if (ev.dataTransfer?.files?.length) addFiles(ev.dataTransfer.files)
}

function close() {
  if (streaming.value) stop()
  emit('update:visible', false)
}

watch(
  () => props.visible,
  async (v) => {
    if (v) {
      // 「AI 升级」注入优先于会话恢复：以现有源码为基线开新会话
      if (props.inject?.source) {
        injectExistingPlugin(props.inject)
      } else {
        restore()
      }
      fetchSettings()
      await scrollChatToBottom()
      await nextTick()
      autoGrow()
    }
  },
  { immediate: true },
)
</script>

<template>
  <Modal
    :visible="visible"
    :width="'min(1400px, 92vw)'"
    :mask-closable="false"
    modal-class="pgen-modal"
    title-align="start"
    :footer="false"
    @cancel="close"
    @update:visible="(v: boolean) => emit('update:visible', v)"
  >
    <template #title>
      <div class="pgen-head">
        <span class="pgen-head__mark"><IconRobot /></span>
        <span class="pgen-head__text">
          <span class="pgen-head__title">插件生成</span>
          <span class="pgen-head__sub">描述需求 → 生成 plugin.js → 同口径校验 → 一键上传</span>
        </span>
        <span class="pgen-head__chip" :class="{ 'is-off': assistantDisabled }">
          <i class="pgen-head__dot" />
          {{ assistantDisabled ? 'AI 助手未启用' : 'js_plugin 技能已激活' }}
        </span>
      </div>
    </template>

    <div class="pgen">
      <!-- ── 左栏：对话 ─────────────────────────────── -->
      <section class="pgen-pane pgen-pane--chat">
        <header class="pgen-pane__head">
          <div class="pgen-pane__ident">
            <span class="pgen-pane__label">对话</span>
            <span v-if="visibleMessages.length" class="pgen-pane__count">{{ visibleMessages.length }} 条</span>
          </div>
          <button
            v-if="visibleMessages.length > 0"
            type="button"
            class="pgen-ghostbtn"
            title="清空会话，重新开始"
            :disabled="streaming"
            @click="clearAll"
          >
            <IconRefresh />新对话
          </button>
        </header>

        <div ref="listEl" class="pgen-msgs">
          <!-- 空态 -->
          <div v-if="visibleMessages.length === 0" class="pgen-empty">
            <span class="pgen-empty__mark"><IconBulb /></span>
            <template v-if="assistantDisabled">
              <p class="pgen-empty__title">AI 助手未启用</p>
              <p class="pgen-empty__desc">
                {{ canConfig ? '请点击顶栏「AI 助手」，在助手设置中开启后回来。' : '请联系拥有「AI 助手配置」权限的管理员开启。' }}
              </p>
            </template>
            <template v-else>
              <p class="pgen-empty__title">描述需求，生成你的插件</p>
              <p class="pgen-empty__desc">
                描述插件要对接的供应商、任务类型与接口行为；可上传 API 文档作为参考，
                AI 将按插件契约生成源码并自动走上传同款校验。
              </p>
              <div class="pgen-empty__asks">
                <button v-for="q in QUICK_ASKS" :key="q" type="button" class="pgen-ask" @click="quickAsk(q)">
                  <span class="pgen-ask__text">{{ q }}</span>
                  <i class="pgen-ask__arrow">→</i>
                </button>
              </div>
            </template>
          </div>

          <!-- 消息列表 -->
          <div v-for="msg in visibleMessages" :key="msg.id" class="pgen-row" :class="`pgen-row--${msg.role}`">
            <span v-if="msg.role === 'assistant'" class="pgen-row__avatar"><IconRobot /></span>
            <div class="pgen-row__body">
              <!-- 思考过程：推理型模型正文前会持续输出 reasoning，实时展示避免干等 -->
              <div
                v-if="msg.role === 'assistant' && msg.thinking"
                class="pgen-think"
                :class="{ 'is-live': msg.id === thinkingLiveId }"
              >
                <button type="button" class="pgen-think__head" @click="toggleThink(msg)">
                  <i class="pgen-think__dot" />
                  <span class="pgen-think__label">
                    {{ msg.id === thinkingLiveId ? '正在思考…' : '思考过程' }}
                  </span>
                  <span class="pgen-think__meta">{{ msg.thinking.length.toLocaleString() }} 字</span>
                  <span class="pgen-think__chev">{{ isThinkOpen(msg) ? '收起' : '展开' }}</span>
                </button>
                <div
                  v-if="isThinkOpen(msg)"
                  :ref="(el) => { if (msg.id === thinkingLiveId) setThinkEl(el) }"
                  class="pgen-think__body"
                >{{ msg.thinking }}</div>
              </div>

              <template v-for="(part, idx) in msg.parts" :key="idx">
                <!-- 用户正文：纯文本插值（不过 marked，防 * / # 被当语法） -->
                <div v-if="part.type === 'text' && part.content" class="pgen-bubble" :class="{ 'md-body': msg.role === 'assistant' }">
                  <template v-if="msg.role === 'user'">{{ part.content }}</template>
                  <div v-else v-html="renderMarkdown(part.content)" />
                </div>
                <!-- 用户附件：紧凑卡片（全文已随消息注入对话） -->
                <div v-else-if="part.type === 'file'" class="pgen-attcard" :title="`${part.file?.name}（全文已注入对话）`">
                  <span class="pgen-attcard__icon"><IconAttachment /></span>
                  <span class="pgen-attcard__name">{{ part.file?.name }}</span>
                  <span class="pgen-attcard__size">{{ prettySize(part.file?.size || 0) }}<em v-if="part.file?.truncated"> · 已截断</em></span>
                </div>
                <!-- 助手代码：左栏不铺源码，指向右栏 -->
                <button
                  v-else-if="part.type === 'code'"
                  type="button"
                  class="pgen-codechip"
                  title="源码展示在右侧面板"
                  @click="selectedIdx = codeIndexMap.get(`${msg.id}:${idx}`) || 0"
                >
                  <span class="pgen-codechip__icon"><IconCode /></span>
                  <span class="pgen-codechip__text">
                    <b>已生成 plugin.js</b>
                    <em>第 {{ codeIndexMap.get(`${msg.id}:${idx}`) }} 版<template v-if="part.truncated || part.pendingMore"> · 未写完</template></em>
                  </span>
                  <span class="pgen-codechip__arrow">右侧查看 →</span>
                </button>
              </template>

              <!-- 工具调用行（agent 模式）：模型经 write/append/edit/validate 工具读写代码，宿主执行 -->
              <div v-if="msg.role === 'assistant' && msg.toolCalls?.length" class="pgen-tools">
                <span v-for="(tc, ti) in msg.toolCalls" :key="tc.id || ti" class="pgen-toolcall">
                  <i class="pgen-toolcall__dot" />
                  <b>{{ toolLabel(tc.name) }}</b>
                  <em>{{ tc.arguments.length.toLocaleString() }} 字符</em>
                </span>
              </div>

              <!-- 等待首 token -->
              <div v-if="msg.id === typingMsgId && !msg.thinking" class="pgen-typing"><i /><i /><i /></div>

              <div v-if="msg.error" class="pgen-row__error">
                <IconCloseCircle />
                <span>{{ msg.error }}</span>
              </div>
              <div v-if="msg.usage?.total_tokens" class="pgen-row__meta">
                <span>{{ msg.usage.model }}</span>
                <i class="pgen-row__dot" />
                <span>{{ msg.usage.total_tokens }} tokens</span>
              </div>
            </div>
          </div>
        </div>

        <!-- 输入区：整体一张卡（附件 / 输入 / 发送 同一视觉容器） -->
        <footer class="pgen-composer" @dragover.prevent @drop="onComposerDrop">
          <!-- 待发附件 -->
          <div v-if="pendingFiles.length" class="pgen-pendings">
            <div v-for="(f, i) in pendingFiles" :key="f.name + i" class="pgen-pending">
              <IconAttachment />
              <span class="pgen-pending__name">{{ f.name }}</span>
              <span class="pgen-pending__size">{{ prettySize(f.size) }}<em v-if="f.truncated"> · 截断</em></span>
              <button type="button" title="移除" @click="removeFile(i)"><IconClose /></button>
            </div>
          </div>

          <div class="pgen-card" :class="{ 'is-disabled': assistantDisabled }">
            <textarea
              ref="textareaEl"
              v-model="input"
              class="pgen-card__input"
              rows="1"
              placeholder="描述插件需求，可拖拽文档到此处或点「附件」上传…"
              :disabled="assistantDisabled"
              @input="autoGrow"
              @keydown.enter.exact.prevent="onSend"
            />
            <div class="pgen-card__bar">
              <button
                type="button"
                class="pgen-tool"
                title="上传文本类附件（md / txt / json / yaml / csv / js 等，≤ 200KB）"
                :disabled="streaming"
                @click="pickFiles"
              >
                <IconAttachment />附件
              </button>
              <input
                ref="fileInput"
                type="file"
                multiple
                accept=".txt,.md,.markdown,.json,.yaml,.yml,.csv,.js,.mjs,.cjs,.ts,.html,.htm,.xml"
                hidden
                @change="onFileChange"
              />
              <span class="pgen-card__hint">Enter 发送 · Shift + Enter 换行</span>
              <button v-if="streaming" type="button" class="pgen-send is-stop" title="停止生成" @click="stop">
                <IconPause />
              </button>
              <button v-else type="button" class="pgen-send" :disabled="!canSend" title="发送 (Enter)" @click="onSend">
                <IconSend />
              </button>
            </div>
          </div>
        </footer>
      </section>

      <!-- ── 右栏：源码 ─────────────────────────────── -->
      <section class="pgen-pane pgen-pane--code">
        <header class="pgen-pane__head">
          <div class="pgen-code__left">
            <span class="pgen-pane__label">生成的源码</span>
            <Select
              v-if="codeVersions.length > 1"
              v-model="selectedIdx"
              class="pgen-code__ver"
              size="small"
              :options="versionOptions"
            />
            <span v-else-if="codeVersions.length === 1" class="pgen-ver">第 1 版</span>
          </div>
          <div v-if="currentSource" class="pgen-code__actions">
            <button type="button" class="pgen-ghostbtn" title="下载为 plugin.js" @click="downloadCode">
              <IconDownload />下载
            </button>
            <button type="button" class="pgen-ghostbtn" @click="copyText(currentSource)">
              <IconCopy />复制
            </button>
            <button type="button" class="pgen-apply" title="带入上传弹窗：试运行 / 上传入库" @click="applySource">
              <IconUpload />
              <span class="pgen-apply__full">用此源码上传</span>
              <span class="pgen-apply__short">上传</span>
            </button>
          </div>
        </header>

        <div class="pgen-code__body">
          <!-- 截断 / 分段未完提示条：自动续写超限后转为手动一键续写（续写会合并进同一版本） -->
          <div v-if="currentVersion?.part.truncated || currentVersion?.part.pendingMore" class="pgen-trunc">
            <IconExclamationCircle />
            <span class="pgen-trunc__text">
              {{ currentVersion.part.truncated
                ? `输出达到长度上限，源码不完整（${lineCount} 行）`
                : '已达自动续写上限，还有后续段落未输出' }}
            </span>
            <button type="button" class="pgen-trunc__btn" :disabled="streaming" @click="onContinue">
              {{ streaming ? '续写中…' : '续写剩余部分' }}
            </button>
          </div>

          <!-- 源码主体：流式跟随 / 版本切换 -->
          <div v-if="currentVersion" class="pgen-src" :class="{ 'is-streaming': streaming && isLatestSelected }">
            <div class="pgen-src__bar">
              <span class="pgen-src__filegroup">
                <span class="pgen-src__file"><IconCode /><b>plugin.js</b></span>
                <span v-if="currentSource" class="pgen-src__meta" :title="`${lineCount} 行 · ${currentSource.length.toLocaleString()} 字符`">
                  {{ lineCount }} 行 · {{ currentSource.length.toLocaleString() }} 字符
                </span>
              </span>
              <span class="pgen-src__stategroup">
                <template v-if="editing">
                  <span class="pgen-src__state is-editing">编辑中</span>
                  <button type="button" class="pgen-src__done" :disabled="validating" @click="finishEdit">
                    {{ validating ? '校验中…' : '完成并校验' }}
                  </button>
                </template>
                <template v-else>
                  <span class="pgen-src__state">
                    <template v-if="streaming && isLatestSelected"><i class="pgen-src__pulse" />生成中…</template>
                    <template v-else>只读预览</template>
                  </span>
                  <button
                    v-if="currentSource && !streaming"
                    type="button"
                    class="pgen-src__edit"
                    title="手动修改当前版本（如版本号）"
                    @click="startEdit"
                  >
                    <IconEdit />编辑
                  </button>
                </template>
              </span>
            </div>
            <div ref="srcEl" class="pgen-src__scroll">
              <!-- 编辑态：纯文本 textarea（改动直接写入当前版本；Tab 插入两空格） -->
              <textarea
                v-if="editing && currentVersion"
                ref="editEl"
                v-model="currentVersion.part.content"
                class="pgen-src__edit-area"
                spellcheck="false"
                @keydown="onEditKeydown"
              ></textarea>
              <pre v-else><code class="hljs language-javascript" v-html="highlightedSource"></code></pre>
            </div>
          </div>
          <div v-else class="pgen-src pgen-src--empty">
            <span class="pgen-src__icon"><IconCode :size="24" /></span>
            <template v-if="streaming">
              <p class="pgen-src__title">{{ thinkingLiveId ? 'AI 正在思考…' : '正在等待代码输出…' }}</p>
              <p class="pgen-src__desc">
                {{ thinkingLiveId
                  ? '思考过程显示在左侧对话里；正文开始输出后，代码会实时流到这里。'
                  : 'AI 的回复中出现代码围栏时，源码将实时流式显示在这里。' }}
              </p>
            </template>
            <template v-else>
              <p class="pgen-src__title">还没有生成代码</p>
              <p class="pgen-src__desc">
                在左侧描述需求或上传 API 文档；生成的 plugin.js 会实时出现在这里，
                并附上传同款校验报告。
              </p>
            </template>
          </div>

          <!-- 校验报告（跟随当前选中版本） -->
          <footer v-if="currentVersion?.part.report" class="pgen-report" :class="currentVersion.part.report.ok ? 'is-ok' : 'is-fail'">
            <div class="pgen-report__head">
              <span class="pgen-report__icon">
                <IconCheckCircle v-if="currentVersion.part.report.ok" />
                <IconCloseCircle v-else />
              </span>
              <span class="pgen-report__title">{{ currentVersion.part.report.ok ? '校验通过' : '校验未通过' }}</span>
              <span class="pgen-report__hint">与插件上传自检同口径</span>
              <span class="pgen-report__stats">
                <i v-if="reportStats.pass" class="is-pass">{{ reportStats.pass }} 通过</i>
                <i v-if="reportStats.warn" class="is-warn">{{ reportStats.warn }} 警告</i>
                <i v-if="reportStats.fail" class="is-fail">{{ reportStats.fail }} 未通过</i>
              </span>
              <button
                v-if="!streaming && isLatestSelected && !currentVersion.part.truncated && reportNeedsRepair(currentVersion.part.report)"
                type="button"
                class="pgen-report__repair"
                @click="onRepair(currentVersion.part.report!)"
              >
                让 AI 修复
              </button>
            </div>
            <div v-if="currentVersion.part.report.error" class="pgen-report__error">
              {{ currentVersion.part.report.error }}
            </div>
            <ul v-if="currentVersion.part.report.checks?.length" class="pgen-report__checks">
              <li
                v-for="c in currentVersion.part.report.checks"
                :key="c.name + c.detail"
                class="pgen-report__check"
                :class="`is-${c.status}`"
              >
                <IconCheckCircle v-if="c.status === 'pass'" />
                <IconExclamationCircle v-else-if="c.status === 'warn'" />
                <IconCloseCircle v-else />
                <b>{{ c.name }}</b>
                <span :title="c.detail">{{ c.detail }}</span>
              </li>
            </ul>
          </footer>
        </div>
      </section>
    </div>
  </Modal>
</template>

<style scoped>
/* ────────────────────────────────────────────────────────
   Modal 外壳：去掉默认 body 内边距，交给分栏自己控制
   （Arco 把 modal 传送到 body，只能走 :global + .pgen-modal 锚点）
   ──────────────────────────────────────────────────────── */
:global(.pgen-modal) {
  border-radius: var(--ta-radius-xl);
  overflow: hidden;
  box-shadow:
    0 28px 72px -20px rgba(15, 23, 42, 0.34),
    0 8px 22px rgba(15, 23, 42, 0.07);
}
:global(.pgen-modal .arco-modal-header) {
  height: auto;
  padding: 13px 16px 13px 18px;
  gap: 12px;
  border-bottom: 1px solid var(--ta-border);
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--ta-primary) 6%, var(--ta-bg-card)) 0%,
    var(--ta-bg-card) 100%
  );
}
:global(.pgen-modal .arco-modal-header .arco-modal-title) {
  min-width: 0;
}
/* main.css 里 .arco-modal-wrapper .arco-modal .arco-modal-body 有 3 级特异性，
   这里用同级选择器压掉它的内边距与滚动 */
:global(.arco-modal-wrapper .pgen-modal .arco-modal-body) {
  padding: 0;
  overflow: hidden;
  min-height: 0;
}
:global(.pgen-modal .arco-modal-close-btn) {
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin-left: 0;
  border-radius: 8px;
  color: var(--ta-text-tertiary);
  transition: background var(--ta-duration-fast) var(--ta-ease), color var(--ta-duration-fast) var(--ta-ease);
}
:global(.pgen-modal .arco-modal-close-btn:hover) {
  background: var(--ta-bg-secondary);
  color: var(--ta-text-primary);
}

/* 标题栏 */
.pgen-head {
  display: flex;
  align-items: center;
  gap: 11px;
  width: 100%;
  min-width: 0;
}
.pgen-head__mark {
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  font-size: 17px;
  color: #fff;
  background: var(--ta-primary-gradient);
  box-shadow: 0 2px 8px color-mix(in srgb, var(--ta-primary) 34%, transparent);
}
.pgen-head__text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.pgen-head__title {
  font-size: 14.5px;
  font-weight: 600;
  line-height: 1.25;
  color: var(--ta-text-primary);
  letter-spacing: 0.2px;
}
.pgen-head__sub {
  font-size: 11.5px;
  line-height: 1.3;
  color: var(--ta-text-tertiary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.pgen-head__chip {
  margin-left: auto;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 24px;
  padding: 0 10px;
  border-radius: 999px;
  font-size: 11.5px;
  font-weight: 500;
  color: color-mix(in srgb, var(--ta-primary) 88%, #000);
  background: color-mix(in srgb, var(--ta-primary) 10%, transparent);
  border: 1px solid color-mix(in srgb, var(--ta-primary) 22%, transparent);
}
.pgen-head__dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--ta-primary);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--ta-primary) 18%, transparent);
}
.pgen-head__chip.is-off {
  color: var(--ta-text-tertiary);
  background: var(--ta-bg-secondary);
  border-color: var(--ta-border);
}
.pgen-head__chip.is-off .pgen-head__dot {
  background: var(--ta-text-tertiary);
  box-shadow: none;
}

/* ── 分栏容器 ───────────────────────────────────── */
.pgen {
  display: flex;
  height: min(80vh, 820px);
  background: var(--ta-bg-page);
}
.pgen-pane {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}
.pgen-pane--chat {
  flex: 0 0 clamp(360px, 40%, 520px);
  border-right: 1px solid var(--ta-border);
  background: var(--ta-bg-page);
}
.pgen-pane--code {
  flex: 1;
  background: var(--ta-bg-card);
}

/* 双栏统一的分区头：等高、白底、下边线 —— 两栏在视觉上对齐 */
.pgen-pane__head {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  height: 46px;
  padding: 0 12px 0 14px;
  background: var(--ta-bg-card);
  border-bottom: 1px solid var(--ta-border);
}
.pgen-pane__ident {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.pgen-pane__label {
  font-size: 13px;
  font-weight: 600;
  color: var(--ta-text-primary);
  white-space: nowrap;
  flex-shrink: 0;
}
.pgen-pane__count {
  padding: 1px 7px;
  border-radius: 999px;
  font-size: 11px;
  color: var(--ta-text-secondary);
  background: var(--ta-bg-secondary);
}

/* 幽灵按钮（下载 / 复制 / 新对话） */
.pgen-ghostbtn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 28px;
  padding: 0 9px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--ta-text-tertiary);
  font-size: 12px;
  cursor: pointer;
  transition: background var(--ta-duration-fast) var(--ta-ease), color var(--ta-duration-fast) var(--ta-ease);
}
.pgen-ghostbtn:hover:not(:disabled) {
  background: var(--ta-bg-secondary);
  color: var(--ta-text-primary);
}
.pgen-ghostbtn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

/* ── 消息列表 ───────────────────────────────────── */
.pgen-msgs {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  padding: 16px 16px 8px;
  scrollbar-width: thin;
  scrollbar-color: color-mix(in srgb, var(--ta-text-tertiary) 40%, transparent) transparent;
}
.pgen-msgs::-webkit-scrollbar {
  width: 6px;
}
.pgen-msgs::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, var(--ta-text-tertiary) 35%, transparent);
  background-clip: content-box;
  border: 2px solid transparent;
  border-radius: 999px;
}

/* 滚动容器是 flex column：子项禁止压缩，否则消息多时会被挤扁 */
.pgen-msgs > * {
  flex-shrink: 0;
}

/* 空态 */
.pgen-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 7px;
  /* 内容不足一屏时垂直居中；超出一屏则自然从顶部排 */
  margin: auto 0;
  padding: 28px 8px;
  text-align: center;
}
.pgen-empty__mark {
  width: 46px;
  height: 46px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 14px;
  font-size: 22px;
  color: #fff;
  background: var(--ta-primary-gradient);
  box-shadow:
    0 6px 16px -4px color-mix(in srgb, var(--ta-primary) 45%, transparent),
    inset 0 1px 0 rgba(255, 255, 255, 0.22);
}
.pgen-empty__title {
  margin: 8px 0 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--ta-text-primary);
}
.pgen-empty__desc {
  margin: 0;
  max-width: 380px;
  font-size: 12.5px;
  line-height: 1.72;
  color: var(--ta-text-tertiary);
}
.pgen-empty__asks {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 14px;
  width: 100%;
  max-width: 380px;
}
.pgen-ask {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 9px 12px;
  border: 1px solid var(--ta-border);
  border-radius: var(--ta-radius);
  background: var(--ta-bg-card);
  color: var(--ta-text-secondary);
  font-size: 12.5px;
  line-height: 1.55;
  text-align: left;
  cursor: pointer;
  transition:
    border-color var(--ta-duration-fast) var(--ta-ease),
    background var(--ta-duration-fast) var(--ta-ease),
    box-shadow var(--ta-duration-fast) var(--ta-ease),
    transform var(--ta-duration-fast) var(--ta-ease);
}
.pgen-ask__text {
  flex: 1;
  min-width: 0;
}
.pgen-ask__arrow {
  flex-shrink: 0;
  font-style: normal;
  font-size: 12px;
  color: var(--ta-text-tertiary);
  opacity: 0;
  transform: translateX(-4px);
  transition: opacity var(--ta-duration-fast) var(--ta-ease), transform var(--ta-duration-fast) var(--ta-ease);
}
.pgen-ask:hover {
  border-color: color-mix(in srgb, var(--ta-primary) 40%, transparent);
  background: color-mix(in srgb, var(--ta-primary) 4%, var(--ta-bg-card));
  box-shadow: 0 2px 10px -4px color-mix(in srgb, var(--ta-primary) 40%, transparent);
  transform: translateY(-1px);
}
.pgen-ask:hover .pgen-ask__arrow {
  opacity: 1;
  transform: translateX(0);
  color: var(--ta-primary);
}

/* 消息行 */
.pgen-row {
  display: flex;
  gap: 9px;
  margin: 0 0 18px;
}
.pgen-row:last-child {
  margin-bottom: 4px;
}
.pgen-row__avatar {
  width: 28px;
  height: 28px;
  margin-top: 1px;
  border-radius: 9px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 15px;
  color: #fff;
  background: var(--ta-primary-gradient);
  box-shadow: 0 2px 6px color-mix(in srgb, var(--ta-primary) 28%, transparent);
  flex-shrink: 0;
}
.pgen-row__body {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
.pgen-row--assistant .pgen-row__body {
  flex: 1;
}
.pgen-row--user {
  justify-content: flex-end;
}
.pgen-row--user .pgen-row__body {
  align-items: flex-end;
  max-width: 88%;
}

/* 气泡：助手用「无气泡」正文（长 markdown 更透气），用户用品牌色胶囊 */
.pgen-bubble {
  font-size: 13px;
  line-height: 1.75;
  word-break: break-word;
  overflow-wrap: anywhere;
}
.pgen-row--assistant .pgen-bubble {
  padding: 0 2px;
  color: var(--ta-text-primary);
}
.pgen-row--user .pgen-bubble {
  padding: 8px 13px;
  border-radius: 14px;
  border-bottom-right-radius: 5px;
  background: var(--ta-primary-gradient);
  color: #fff;
  white-space: pre-wrap;
  box-shadow: 0 6px 16px -6px color-mix(in srgb, var(--ta-primary) 55%, transparent);
}

/* 附件卡片（已随消息发出的） */
.pgen-attcard {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  max-width: 100%;
  padding: 6px 10px;
  border: 1px solid var(--ta-border);
  border-radius: 9px;
  background: var(--ta-bg-card);
  font-size: 12px;
  color: var(--ta-text-secondary);
}
.pgen-attcard__icon {
  color: var(--ta-primary);
  display: inline-flex;
}
.pgen-attcard__name {
  font-weight: 500;
  color: var(--ta-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 220px;
}
.pgen-attcard__size {
  color: var(--ta-text-tertiary);
  white-space: nowrap;
}
.pgen-attcard__size em {
  font-style: normal;
  color: rgb(var(--orange-5));
}

/* 左栏代码提示卡（指向右栏） */
.pgen-codechip {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 9px 11px;
  border: 1px solid color-mix(in srgb, var(--ta-primary) 28%, transparent);
  border-radius: 11px;
  background: color-mix(in srgb, var(--ta-primary) 5%, var(--ta-bg-card));
  cursor: pointer;
  text-align: left;
  transition: border-color var(--ta-duration-fast) var(--ta-ease), background var(--ta-duration-fast) var(--ta-ease);
}
.pgen-codechip:hover {
  border-color: color-mix(in srgb, var(--ta-primary) 50%, transparent);
  background: color-mix(in srgb, var(--ta-primary) 9%, var(--ta-bg-card));
}
.pgen-codechip__icon {
  width: 26px;
  height: 26px;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  color: var(--ta-primary);
  background: color-mix(in srgb, var(--ta-primary) 12%, transparent);
  font-size: 14px;
}
.pgen-codechip__text {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}
.pgen-codechip__text b {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--ta-text-primary);
}
.pgen-codechip__text em {
  font-style: normal;
  font-size: 11px;
  color: var(--ta-text-tertiary);
}
.pgen-codechip__arrow {
  margin-left: auto;
  flex-shrink: 0;
  font-size: 11.5px;
  color: var(--ta-primary);
}

/* 思考过程（reasoning）：折叠条 + 可滚动正文 */
.pgen-think {
  border: 1px solid var(--ta-border);
  border-radius: 11px;
  background: var(--ta-bg-secondary);
  overflow: hidden;
}
.pgen-think.is-live {
  border-color: color-mix(in srgb, var(--ta-primary) 32%, transparent);
  background: color-mix(in srgb, var(--ta-primary) 5%, var(--ta-bg-card));
}
.pgen-think__head {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  padding: 7px 10px;
  border: none;
  background: transparent;
  cursor: pointer;
  text-align: left;
}
.pgen-think__dot {
  width: 6px;
  height: 6px;
  flex-shrink: 0;
  border-radius: 50%;
  background: var(--ta-text-tertiary);
}
.pgen-think.is-live .pgen-think__dot {
  background: var(--ta-primary);
  animation: pgen-pulse 1.4s infinite ease-in-out;
}
.pgen-think__label {
  font-size: 12px;
  font-weight: 500;
  color: var(--ta-text-secondary);
}
.pgen-think.is-live .pgen-think__label {
  color: var(--ta-primary);
}
.pgen-think__meta {
  margin-left: auto;
  font-size: 11px;
  color: var(--ta-text-tertiary);
  font-variant-numeric: tabular-nums;
}
.pgen-think__chev {
  font-size: 11px;
  color: var(--ta-text-tertiary);
}
.pgen-think__head:hover .pgen-think__chev {
  color: var(--ta-primary);
}
.pgen-think__body {
  max-height: 190px;
  overflow-y: auto;
  padding: 0 12px 10px 23px;
  font-size: 12px;
  line-height: 1.75;
  color: var(--ta-text-secondary);
  white-space: pre-wrap;
  word-break: break-word;
  scrollbar-width: thin;
  scrollbar-color: color-mix(in srgb, var(--ta-text-tertiary) 40%, transparent) transparent;
}
.pgen-think__body::-webkit-scrollbar {
  width: 6px;
}
.pgen-think__body::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, var(--ta-text-tertiary) 35%, transparent);
  background-clip: content-box;
  border: 2px solid transparent;
  border-radius: 999px;
}


/* 工具调用行（agent 模式） */
.pgen-tools {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}
.pgen-toolcall {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 9px;
  border: 1px solid color-mix(in srgb, var(--ta-primary) 22%, transparent);
  border-radius: 999px;
  background: color-mix(in srgb, var(--ta-primary) 4%, var(--ta-bg-card));
  font-size: 11px;
  color: var(--ta-text-secondary);
}
.pgen-toolcall__dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--ta-primary);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--ta-primary) 15%, transparent);
}
.pgen-toolcall b {
  font-weight: 500;
  color: var(--ta-text-primary);
}
.pgen-toolcall em {
  font-style: normal;
  color: var(--ta-text-tertiary);
  font-variant-numeric: tabular-nums;
}

/* 打字指示器 */
.pgen-typing {
  display: inline-flex;
  gap: 4px;
  padding: 6px 2px;
}
.pgen-typing i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ta-text-tertiary);
  animation: pgen-blink 1.2s infinite ease-in-out;
}
.pgen-typing i:nth-child(2) {
  animation-delay: 0.18s;
}
.pgen-typing i:nth-child(3) {
  animation-delay: 0.36s;
}
@keyframes pgen-blink {
  0%,
  80%,
  100% {
    opacity: 0.25;
  }
  40% {
    opacity: 1;
  }
}

.pgen-row__error {
  display: flex;
  align-items: flex-start;
  gap: 5px;
  font-size: 12px;
  color: rgb(var(--red-5));
}
.pgen-row__meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--ta-text-tertiary);
}
.pgen-row__dot {
  width: 2px;
  height: 2px;
  border-radius: 50%;
  background: var(--ta-text-tertiary);
}

/* ── 输入卡 ─────────────────────────────────────── */
.pgen-composer {
  flex-shrink: 0;
  padding: 8px 12px 12px;
  background: var(--ta-bg-page);
}
.pgen-pendings {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 8px;
}
.pgen-pending {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 6px 4px 9px;
  border: 1px solid var(--ta-border);
  border-radius: 8px;
  background: var(--ta-bg-card);
  font-size: 12px;
  color: var(--ta-text-secondary);
}
.pgen-pending svg {
  color: var(--ta-primary);
}
.pgen-pending__name {
  max-width: 170px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--ta-text-primary);
}
.pgen-pending__size {
  color: var(--ta-text-tertiary);
  white-space: nowrap;
}
.pgen-pending__size em {
  font-style: normal;
  color: rgb(var(--orange-5));
}
.pgen-pending button {
  display: inline-flex;
  border: none;
  background: transparent;
  padding: 2px;
  border-radius: 5px;
  color: var(--ta-text-tertiary);
  cursor: pointer;
}
.pgen-pending button:hover {
  color: rgb(var(--red-5));
  background: var(--ta-bg-secondary);
}

/* 一张卡包住输入与工具栏：focus-within 时整卡高亮 */
.pgen-card {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 9px 9px 7px 12px;
  border: 1px solid var(--ta-border);
  border-radius: 14px;
  background: var(--ta-bg-card);
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
  transition: border-color var(--ta-duration-fast) var(--ta-ease), box-shadow var(--ta-duration-fast) var(--ta-ease);
}
.pgen-card:focus-within {
  border-color: color-mix(in srgb, var(--ta-primary) 55%, transparent);
  box-shadow:
    0 0 0 3px color-mix(in srgb, var(--ta-primary) 12%, transparent),
    0 1px 2px rgba(15, 23, 42, 0.04);
}
.pgen-card.is-disabled {
  background: var(--ta-bg-secondary);
  opacity: 0.85;
}
.pgen-card__input {
  width: 100%;
  min-height: 24px;
  max-height: 132px;
  padding: 3px 2px;
  border: none;
  background: transparent;
  color: var(--ta-text-primary);
  font-size: 13px;
  line-height: 1.62;
  font-family: inherit;
  resize: none;
  outline: none;
}
.pgen-card__input::placeholder {
  color: var(--ta-text-tertiary);
}
.pgen-card__bar {
  display: flex;
  align-items: center;
  gap: 6px;
}
.pgen-tool {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 28px;
  padding: 0 8px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--ta-text-tertiary);
  font-size: 12px;
  cursor: pointer;
  transition: background var(--ta-duration-fast) var(--ta-ease), color var(--ta-duration-fast) var(--ta-ease);
}
.pgen-tool:hover:not(:disabled) {
  background: var(--ta-bg-secondary);
  color: var(--ta-primary);
}
.pgen-tool:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.pgen-card__hint {
  margin-left: auto;
  font-size: 11px;
  color: var(--ta-text-tertiary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.pgen-send {
  width: 30px;
  height: 30px;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 9px;
  background: var(--ta-primary-gradient);
  color: #fff;
  font-size: 14px;
  cursor: pointer;
  box-shadow: 0 3px 10px -3px color-mix(in srgb, var(--ta-primary) 55%, transparent);
  transition: opacity var(--ta-duration-fast) var(--ta-ease), transform var(--ta-duration-fast) var(--ta-ease);
}
.pgen-send:hover:not(:disabled) {
  transform: translateY(-1px);
}
.pgen-send:disabled {
  background: var(--ta-bg-secondary);
  color: var(--ta-text-tertiary);
  box-shadow: none;
  cursor: not-allowed;
}
.pgen-send.is-stop {
  background: linear-gradient(135deg, rgb(var(--red-5)), rgb(var(--red-6)));
  box-shadow: 0 3px 10px -3px color-mix(in srgb, rgb(var(--red-6)) 55%, transparent);
}

/* ── 右栏：源码 ─────────────────────────────────── */
.pgen-code__left {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
}
.pgen-code__ver {
  width: 122px;
  flex-shrink: 0;
}
.pgen-ver {
  padding: 1px 7px;
  border-radius: 999px;
  font-size: 11px;
  color: var(--ta-text-secondary);
  background: var(--ta-bg-secondary);
  white-space: nowrap;
}
.pgen-code__actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  margin-left: auto;
  padding-left: 8px;
}
.pgen-apply {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 28px;
  margin-left: 4px;
  padding: 0 12px;
  border: none;
  border-radius: 8px;
  background: var(--ta-primary-gradient);
  color: #fff;
  font-size: 12px;
  cursor: pointer;
  box-shadow: 0 3px 10px -3px color-mix(in srgb, var(--ta-primary) 55%, transparent);
  transition: opacity var(--ta-duration-fast) var(--ta-ease), transform var(--ta-duration-fast) var(--ta-ease);
  white-space: nowrap;
}
.pgen-apply:hover {
  transform: translateY(-1px);
}
/* 主按钮长文案（窄屏收缩为「上传」） */
.pgen-apply__short {
  display: none;
}

.pgen-code__body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 14px 14px;
}

/* 源码卡：顶部文件名条 + 可滚动代码区（编辑器观感） */
.pgen-src {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--ta-border);
  border-radius: var(--ta-radius-lg);
  background: var(--ta-bg-secondary);
  overflow: hidden;
}
.pgen-src__bar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  height: 34px;
  padding: 0 12px;
  border-bottom: 1px solid var(--ta-border);
  background: var(--ta-bg-card);
}
.pgen-src__file {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11.5px;
  color: var(--ta-text-tertiary);
  white-space: nowrap;
  flex-shrink: 0;
}
.pgen-src__filegroup {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
/* 行数 / 字符统计放文件名条（头部一行只留标题与按钮，避免挤压换行） */
.pgen-src__meta {
  font-size: 11px;
  color: var(--ta-text-tertiary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-variant-numeric: tabular-nums;
}
.pgen-src__file svg {
  color: var(--ta-primary);
  font-size: 13px;
}
.pgen-src__file b {
  font-weight: 600;
  color: var(--ta-text-primary);
}
.pgen-src__state {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--ta-text-tertiary);
}
.pgen-src__state.is-editing {
  color: rgb(var(--orange-5));
}
.pgen-src__stategroup {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
/* 文件名条上的编辑/完成小按钮（贴近代码区，不挤头部） */
.pgen-src__edit,
.pgen-src__done {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 22px;
  padding: 0 8px;
  border-radius: 6px;
  font-size: 11px;
  cursor: pointer;
  white-space: nowrap;
  transition: background var(--ta-duration-fast) var(--ta-ease), color var(--ta-duration-fast) var(--ta-ease);
}
.pgen-src__edit {
  border: 1px solid var(--ta-border);
  background: var(--ta-bg-card);
  color: var(--ta-text-secondary);
}
.pgen-src__edit:hover {
  border-color: color-mix(in srgb, var(--ta-primary) 40%, transparent);
  color: var(--ta-primary);
}
.pgen-src__done {
  border: none;
  background: var(--ta-primary-gradient);
  color: #fff;
  box-shadow: 0 2px 8px -2px color-mix(in srgb, var(--ta-primary) 55%, transparent);
}
.pgen-src__done:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
/* 编辑态 textarea：与高亮 pre 的排版完全一致（等宽字体/行高/内边距），视觉无跳变 */
.pgen-src__edit-area {
  display: block;
  width: 100%;
  min-height: 100%;
  border: none;
  outline: none;
  resize: none;
  background: transparent;
  color: var(--ta-text-primary);
  font-family: ui-monospace, SFMono-Regular, Consolas, 'Courier New', monospace;
  font-size: 12.5px;
  line-height: 1.72;
  white-space: pre;
  overflow-x: auto;
  tab-size: 2;
  padding: 14px 16px;
  margin: 0;
}
.pgen-src__pulse {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ta-primary);
  animation: pgen-pulse 1.4s infinite ease-in-out;
}
@keyframes pgen-pulse {
  0%,
  100% {
    opacity: 0.3;
    transform: scale(0.85);
  }
  50% {
    opacity: 1;
    transform: scale(1.15);
  }
}
.pgen-src__scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
  scrollbar-width: thin;
  scrollbar-color: color-mix(in srgb, var(--ta-text-tertiary) 40%, transparent) transparent;
}
.pgen-src__scroll::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}
.pgen-src__scroll::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, var(--ta-text-tertiary) 35%, transparent);
  background-clip: content-box;
  border: 2px solid transparent;
  border-radius: 999px;
}
.pgen-src pre {
  margin: 0;
  padding: 14px 16px;
}
.pgen-src pre code {
  font-family: ui-monospace, SFMono-Regular, Consolas, 'Courier New', monospace;
  font-size: 12.5px;
  line-height: 1.72;
  color: var(--ta-text-primary);
  white-space: pre;
  tab-size: 2;
}
/* 流式期间尾部光标 */
.pgen-src.is-streaming pre code::after {
  content: '▋';
  color: var(--ta-primary);
  animation: pgen-blink 1s infinite ease-in-out;
}

/* 空态：点阵画布 + 居中提示 */
.pgen-src--empty {
  align-items: center;
  justify-content: center;
  gap: 7px;
  text-align: center;
  border-style: dashed;
  background-color: transparent;
  background-image: radial-gradient(
    circle,
    color-mix(in srgb, var(--ta-text-tertiary) 22%, transparent) 1px,
    transparent 1px
  );
  background-size: 18px 18px;
  background-position: center;
}
.pgen-src__icon {
  width: 42px;
  height: 42px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  color: var(--ta-text-tertiary);
  background: var(--ta-bg-card);
  border: 1px solid var(--ta-border);
}
.pgen-src__title {
  margin: 6px 0 0;
  font-size: 13.5px;
  font-weight: 600;
  color: var(--ta-text-primary);
}
.pgen-src__desc {
  margin: 0;
  max-width: 400px;
  padding: 0 16px;
  font-size: 12.5px;
  line-height: 1.7;
  color: var(--ta-text-tertiary);
}

/* 截断提示条 */
.pgen-trunc {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, rgb(var(--orange-5)) 32%, transparent);
  background: color-mix(in srgb, rgb(var(--orange-5)) 7%, transparent);
  font-size: 12px;
  color: var(--ta-text-secondary);
}
.pgen-trunc > svg {
  flex-shrink: 0;
  color: rgb(var(--orange-5));
}
.pgen-trunc__text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pgen-trunc__btn {
  flex-shrink: 0;
  padding: 4px 11px;
  border: none;
  border-radius: 7px;
  background: linear-gradient(135deg, rgb(var(--orange-5)), rgb(var(--orange-6)));
  color: #fff;
  font-size: 12px;
  cursor: pointer;
  transition: opacity var(--ta-duration-fast) var(--ta-ease);
}
.pgen-trunc__btn:hover:not(:disabled) {
  opacity: 0.9;
}
.pgen-trunc__btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

/* 校验报告 */
.pgen-report {
  flex-shrink: 0;
  max-height: 190px;
  overflow-y: auto;
  padding: 10px 12px;
  border-radius: 11px;
  border: 1px solid var(--ta-border);
  scrollbar-width: thin;
}
.pgen-report.is-ok {
  border-color: color-mix(in srgb, rgb(var(--green-6)) 32%, transparent);
  background: color-mix(in srgb, rgb(var(--green-6)) 6%, transparent);
}
.pgen-report.is-fail {
  border-color: color-mix(in srgb, rgb(var(--red-6)) 30%, transparent);
  background: color-mix(in srgb, rgb(var(--red-6)) 5%, transparent);
}
.pgen-report__head {
  display: flex;
  align-items: center;
  gap: 7px;
  flex-wrap: wrap;
}
.pgen-report__icon {
  display: inline-flex;
  font-size: 14px;
}
.pgen-report.is-ok .pgen-report__icon {
  color: rgb(var(--green-6));
}
.pgen-report.is-fail .pgen-report__icon {
  color: rgb(var(--red-5));
}
.pgen-report__title {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--ta-text-primary);
}
.pgen-report__hint {
  font-size: 11px;
  color: var(--ta-text-tertiary);
}
.pgen-report__stats {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
}
.pgen-report__stats i {
  font-style: normal;
  font-size: 11px;
  padding: 1px 7px;
  border-radius: 999px;
}
.pgen-report__stats .is-pass {
  color: rgb(var(--green-6));
  background: color-mix(in srgb, rgb(var(--green-6)) 12%, transparent);
}
.pgen-report__stats .is-warn {
  color: rgb(var(--orange-5));
  background: color-mix(in srgb, rgb(var(--orange-5)) 14%, transparent);
}
.pgen-report__stats .is-fail {
  color: rgb(var(--red-5));
  background: color-mix(in srgb, rgb(var(--red-5)) 12%, transparent);
}
.pgen-report__error {
  margin-top: 7px;
  font-size: 12px;
  line-height: 1.6;
  color: rgb(var(--red-5));
  word-break: break-all;
}
.pgen-report__checks {
  margin: 8px 0 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 5px;
}
.pgen-report__check {
  display: flex;
  align-items: baseline;
  gap: 6px;
  font-size: 12px;
  color: var(--ta-text-secondary);
}
.pgen-report__check svg {
  flex-shrink: 0;
  align-self: center;
}
.pgen-report__check b {
  font-weight: 500;
  color: var(--ta-text-primary);
  white-space: nowrap;
}
.pgen-report__check span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pgen-report__check.is-pass svg {
  color: rgb(var(--green-6));
}
.pgen-report__check.is-warn svg {
  color: rgb(var(--orange-5));
}
.pgen-report__check.is-fail svg {
  color: rgb(var(--red-5));
}
.pgen-report__repair {
  margin-left: auto;
  padding: 4px 12px;
  border: none;
  border-radius: 7px;
  background: linear-gradient(135deg, rgb(var(--orange-5)), rgb(var(--orange-6)));
  color: #fff;
  font-size: 12px;
  cursor: pointer;
  transition: opacity var(--ta-duration-fast) var(--ta-ease);
}
.pgen-report__repair:hover {
  opacity: 0.92;
}

/* markdown 排版（与全局助手同口径） */
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
  background: transparent;
}
.md-body :deep(table) {
  border-collapse: collapse;
  margin: 0 0 9px;
  font-size: 12px;
}
.md-body :deep(th),
.md-body :deep(td) {
  border: 1px solid var(--ta-border);
  padding: 5px 9px;
}
.md-body :deep(th) {
  background: var(--ta-bg-secondary);
}

/* ── 窄屏：不堆叠（弹窗仍是左右分栏），只压缩装饰与次级按钮 ── */
@media (max-width: 1100px) {
  .pgen-head__chip {
    display: none;
  }
  .pgen-card__hint {
    display: none;
  }
  /* 头部只保标题 + 版本 + 主按钮，下载/复制收进按钮省空间 */
  .pgen-code__actions .pgen-ghostbtn {
    display: none;
  }
}

/* ── 移动端：竖排堆叠 ───────────────────────────── */
@media (max-width: 768px) {
  .pgen {
    flex-direction: column;
    height: 86vh;
  }
  .pgen-pane--chat {
    flex: 1 1 52%;
    border-right: none;
    border-bottom: 1px solid var(--ta-border);
  }
  .pgen-pane--code {
    flex: 1 1 48%;
  }
  .pgen-empty {
    padding: 24px 8px 16px;
  }
  .pgen-head__sub {
    display: none;
  }
  /* 移动端：主按钮长文案收缩为「上传」，保住标题与版本选择器的空间 */
  .pgen-apply {
    padding: 0 10px;
  }
  .pgen-apply__full {
    display: none;
  }
  .pgen-apply__short {
    display: inline;
  }
}
</style>
