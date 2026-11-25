import { ref, reactive, computed } from 'vue'
import { Message } from '@arco-design/web-vue'
import request, { getAccessToken, shouldRefresh } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'

/**
 * AI 助手会话状态机：消息模型（parts 数组，代码走专用通道）+ SSE 流式解析
 * + localStorage 持久化 + 半自动修复环 + 文本类附件注入。
 *
 * SSE 事件协议见后端 internal/handler/admin/ai_assistant_chat.go：
 *   thinking / delta / code_start / code_delta / code_end / truncated / notice / validate / usage / error / [DONE]
 */

export interface ValidateCheck {
  name: string
  status: string // pass | warn | fail
  detail: string
}

export interface EvalPreview {
  name: string
  factor: number
  env?: Record<string, unknown>
}

export interface ValidateReport {
  skill: string
  ok: boolean
  checks?: ValidateCheck[]
  preview?: EvalPreview[]
  error?: string
}

export interface UsageInfo {
  model?: string
  prompt_tokens?: number
  completion_tokens?: number
  total_tokens?: number
}

/** 附件 part 元信息（展示用；content 为注入全文） */
export interface AttachmentMeta {
  name: string
  size: number
  truncated?: boolean
}

/** 消息分段：text 走 markdown 渲染；code 走专用代码卡片（不经 markdown，防反引号拆碎）；
 *  file 为用户消息携带的附件全文（气泡里渲染成附件卡片） */
export interface MessagePart {
  type: 'text' | 'code' | 'file'
  content: string
  lang?: string
  report?: ValidateReport
  file?: AttachmentMeta
  /** 该代码段因 max_tokens 被截断（服务端 finish_reason=length），需续写才能完整 */
  truncated?: boolean
  /** 分段输出已达自动续写上限、还有后续段落未输出（手动点「续写」补完） */
  pendingMore?: boolean
}

/** 一次模型发起的工具调用（agent 轮聚合结果；与后端 aiassistant.ToolCall 同形态） */
export interface ToolCallInfo {
  id: string
  name: string
  arguments: string
}

export interface ChatMsg {
  id: number
  role: 'user' | 'assistant' | 'tool'
  parts: MessagePart[]
  /** assistant 消息：本轮模型发起的工具调用（工具模式 agent loop） */
  toolCalls?: ToolCallInfo[]
  /** tool 消息：对应的工具调用 ID（结果在 parts[0].content） */
  toolCallId?: string
  /** 隐藏消息（如续写指令/工具结果）：仍参与上游上下文，但不在会话里渲染 */
  hidden?: boolean
  /** 思考备忘消息（agent 续轮携带上轮思考尾部）：重建历史时只保留最新一条 */
  memo?: boolean
  /** 种子消息（升级已有插件时注入的基线源码）：只进本地版本视图，不进上游历史——
   *  模型经 read_code 工具按需读取，避免每轮请求重发整份源码 */
  seedOnly?: boolean
  /** 思考过程全文（推理型模型的 reasoning_content；仅助手消息） */
  thinking?: string
  /** 本条回复被输出上限截断（消息级标记：代码段可能尚未产生，截断信息无处可挂） */
  truncated?: boolean
  skills?: string[]
  usage?: UsageInfo
  error?: string
  ts: number
}

/** 待发送附件：已在前端读取为文本，随下一条用户消息一起发出 */
export interface PendingAttachment {
  name: string
  size: number
  text: string
  truncated: boolean
}

/** 技能清单（与后端 internal/logic/aiassistant/skills 注册表对齐） */
export const SKILL_OPTIONS = [
  { key: 'billing_formula', label: '计费表达式' },
  { key: 'js_plugin', label: 'JS 插件' },
] as const

export interface ChatOptions {
  /** localStorage 会话键前缀（默认全局助手键；插件生成工作台传独立键隔离会话） */
  storageKey?: string
  /** 初始激活技能（如插件生成器固定 js_plugin） */
  defaultSkills?: string[]
}

const STORAGE_PREFIX = 'admin_ai_assistant_v1_'
const MAX_PERSIST_MESSAGES = 100

// 附件读取约束：仅文本类、按字节截断（后端单条消息 256KB 上限，留出围栏包装余量）
const ATTACH_EXTS = /\.(txt|md|markdown|json|ya?ml|csv|js|mjs|cjs|ts|html?|xml)$/i
const MAX_ATTACH_BYTES = 200 * 1024
const MAX_ATTACH_COUNT = 5
// 持久化时附件只保留开头一小段（localStorage 5MB 限额；恢复后继续对话时附件为截断版）
const PERSIST_FILE_KEEP = 8 * 1024
// 思考过程同理：预览够用即可，全量留给当前会话
const PERSIST_THINKING_KEEP = 4000

let msgSeq = Date.now()

/** 续写指令：只在被截断时使用（隐藏消息，不污染会话，但要进上游上下文） */
const CONTINUE_PROMPT =
  '上一条回复因输出长度上限被截断。请从中断处继续输出剩余代码：直接接着写，' +
  '不要重复已输出的内容，不要重新开头，不要输出任何解释文字。'

/** 分段输出协议标记（与 js_plugin 技能提示词约定一致）：模型每输出一段（一个围栏）后，
 *  在围栏外另起一行标记是否还有后续段；各段按序拼接成完整 plugin.js */
const PLUGIN_SECTION_DONE = 'PLUGIN_SECTION_DONE'
const PLUGIN_COMPLETE = 'PLUGIN_COMPLETE'

/** 分段续写指令：请求下一段（隐藏消息），并重申协议避免模型忘记标记 */
const SECTION_CONTINUE_PROMPT =
  '继续输出下一段。仍然遵守分段输出协议：本段内容放在一个四反引号 js 围栏中，' +
  `围栏结束后另起一行写 ${PLUGIN_SECTION_DONE}（还有后续段）或 ${PLUGIN_COMPLETE}（全部输出完毕）。`

/** 挽救指令：模型把输出额度耗在分析文字上、一个代码围栏都没开出就被截断时，
 *  要求它放弃铺垫直接开始输出第一段代码（隐藏消息，计入自动续写次数上限） */
const BOOTSTRAP_CODE_PROMPT =
  '上一条回复在输出任何代码之前就被输出长度上限截断了。' +
  '不要再输出分析、规划、提纲或任何铺垫文字，立即开始输出插件源码的第一段：' +
  '直接以四反引号 js 围栏开头，围栏结束后另起一行写 ' + PLUGIN_SECTION_DONE +
  '（还有后续段）或 ' + PLUGIN_COMPLETE + '（全部输出完毕）。'

// 自动续写 / 自动修复轮次上限：防模型失控循环与 token 失控，超限后转为手动按钮
const MAX_AUTO_CONTINUATIONS = 8
const MAX_AUTO_REPAIRS = 2
// 工具模式（agent loop）单链止损三闸：
//   轮次上限 / 输出 token 预算 / 空转检测（连续多轮几乎不产出代码——推理模型的思考
//   不回传历史，每轮重新起草整个文件就是这种形态：单轮 30k token 的思考 + 一小段追加）
const MAX_AGENT_ROUNDS = 12
const MAX_AGENT_COMPLETION_TOKENS = 120_000
const MAX_AGENT_STALL_ROUNDS = 2
// 工具参数连续解析失败次数上限：命中基本可判定端点对流式 tool_calls 的支持不完整
//（分片聚合出的 arguments 是坏 JSON），继续重试只会无限空转
const MAX_AGENT_TOOL_FAILURES = 3

export function useAiAssistantChat(opts: ChatOptions = {}) {
  const authStore = useAuthStore()
  const messages = ref<ChatMsg[]>([])
  const streaming = ref(false)
  const input = ref('')
  const activeSkills = ref<string[]>([...(opts.defaultSkills || [])])
  const pendingFiles = ref<PendingAttachment[]>([])
  let abortController: AbortController | null = null
  // 续写目标：本轮增量应并回这个代码段（而不是新建版本）
  let continuationTarget: MessagePart | null = null
  // 上一轮 sendCore 实际使用的续写目标（驱动循环据此定位「跨轮累积的代码段」）
  let lastRoundContinuation: MessagePart | null = null
  // 会话链序号：用户发起新提问 / 清空会话时递增，进行中的自动驱动循环检测到变化即停
  let chainSeq = 0
  // 用户点击「停止」后置位：终止自动驱动循环（下次用户主动操作时复位）
  let stopFlag = false
  // 上一轮工具执行统计（driveRounds 的空转判定）：产出字符数 / 是否校验过 / 参数解析失败数
  let lastRoundProduced = 0
  let lastRoundHadValidate = false
  let lastRoundToolFailures = 0
  let lastRoundMutated = false

  const canSend = computed(
    () => !streaming.value && (input.value.trim().length > 0 || pendingFiles.value.length > 0),
  )

  function storageKey(): string {
    return (opts.storageKey || STORAGE_PREFIX) + (authStore.user?.id ?? 0)
  }

  function persist() {
    try {
      const trimmed = messages.value.slice(-MAX_PERSIST_MESSAGES).map((m) => ({
        ...m,
        thinking: m.thinking && m.thinking.length > PERSIST_THINKING_KEEP
          ? m.thinking.slice(0, PERSIST_THINKING_KEEP) + '\n…（思考过程已截断）'
          : m.thinking,
        parts: m.parts.map((p) =>
          p.type === 'file' && p.content.length > PERSIST_FILE_KEEP
            ? { ...p, content: p.content.slice(0, PERSIST_FILE_KEEP), file: { ...p.file, truncated: true } }
            : p,
        ),
      }))
      localStorage.setItem(storageKey(), JSON.stringify(trimmed))
    } catch {
      // 存储满/隐私模式：静默失败，会话仅存活于当前页面
    }
  }

  function restore() {
    try {
      const raw = localStorage.getItem(storageKey())
      if (raw) messages.value = JSON.parse(raw) as ChatMsg[]
    } catch {
      messages.value = []
    }
  }

  function clearAll() {
    chainSeq++ // 终止进行中的自动驱动循环
    messages.value = []
    pendingFiles.value = []
    try {
      localStorage.removeItem(storageKey())
    } catch {
      // ignore
    }
  }

  /** 把 parts 还原为上游可读的纯文本：
   *  - 用户附件重建为「附件名标注 + 四反引号围栏」（防文档内 ``` 拆围栏）
   *  - 助手代码重建为四反引号围栏（围栏降级模式：让模型看到自己上一轮的产物；
   *    工具模式下代码真相在工具调用历史里，不重建围栏）
   *  - 工具调用轮：assistant 携带 tool_calls、tool 消息携带 tool_call_id（OpenAI 兼容
   *    agent 协议，续轮请求的 messages 尾部即 tool 结果）
   *  - 思考备忘（推理模型的 reasoning 不回传历史，续轮会从头重新推理——相邻思考块
   *    开头逐字相同即此形态）只保留最新一条，避免历史膨胀
   *  - 分段续写轮的助手消息代码并回了旧段、标记已剥除，可能只剩空内容——跳过，
   *    避免向上游发送空 content 的 assistant 消息（严格端点会拒绝） */
  function flattenForApi(msgs: ChatMsg[]): { role: string; content: string; tool_calls?: unknown[]; tool_call_id?: string }[] {
    // 思考备忘只保留最后一条（旧备忘的结论已被后续轮吸收）
    let lastMemoIdx = -1
    for (let i = msgs.length - 1; i >= 0; i--) {
      if (msgs[i].memo) {
        lastMemoIdx = i
        break
      }
    }
    const out: { role: string; content: string; tool_calls?: unknown[]; tool_call_id?: string }[] = []
    msgs.forEach((m, i) => {
      if (m.seedOnly) return // 基线源码不进历史（read_code 工具按需提供）
      if (m.memo && i !== lastMemoIdx) return
      if (m.role === 'user') {
        const content = m.parts
          .map((p) => {
            if (p.type === 'file') {
              return `\n\n[附件 ${p.file?.name || '未命名'}]\n\`\`\`\`\n${p.content}\n\`\`\`\`\n`
            }
            return p.content
          })
          .join('')
        out.push({ role: 'user', content })
        return
      }
      if (m.role === 'tool') {
        // 工具结果消息：只能跟在对应 assistant 的 tool_calls 之后
        out.push({ role: 'tool', content: m.parts.map((p) => p.content).join(''), tool_call_id: m.toolCallId })
        return
      }
      const text = m.parts
        .map((p) => {
          if (p.type === 'text') return p.content
          return '\n````' + (p.lang || '') + '\n' + p.content + '\n````\n'
        })
        .join('')
        .trim()
      const entry: { role: string; content: string; tool_calls?: unknown[] } = { role: 'assistant', content: text }
      if (m.toolCalls?.length) {
        entry.tool_calls = m.toolCalls.map((tc) => ({
          id: tc.id,
          type: 'function',
          function: { name: tc.name, arguments: tc.arguments },
        }))
        if (!text) entry.content = ''
      }
      // 空内容且无工具调用的助手消息跳过（分段续写轮空壳）
      if (!entry.content && !entry.tool_calls?.length) return
      out.push(entry)
    })
    return out
  }

  function appendTextDelta(msg: ChatMsg, delta: string) {
    const last = msg.parts[msg.parts.length - 1]
    if (last && last.type === 'text') {
      last.content += delta
    } else {
      msg.parts.push({ type: 'text', content: delta })
    }
  }

  function lastCodePart(msg: ChatMsg): MessagePart | null {
    for (let i = msg.parts.length - 1; i >= 0; i--) {
      if (msg.parts[i].type === 'code') return msg.parts[i]
    }
    return null
  }

  function appendCodeDelta(msg: ChatMsg, delta: string) {
    const last = msg.parts[msg.parts.length - 1]
    if (last && last.type === 'code') {
      last.content += delta
    } else {
      msg.parts.push({ type: 'code', content: delta })
    }
  }

  // ── 附件：前端读取文本类文件，全文注入下一条用户消息 ──────────

  async function addFiles(files: File[] | FileList) {
    for (const f of Array.from(files)) {
      if (pendingFiles.value.length >= MAX_ATTACH_COUNT) {
        Message.warning(`单条消息最多携带 ${MAX_ATTACH_COUNT} 个附件`)
        return
      }
      if (!ATTACH_EXTS.test(f.name)) {
        Message.warning(`「${f.name}」不是支持的文本类文件（md / txt / json / yaml / csv / js 等）`)
        continue
      }
      if (f.size === 0) {
        Message.warning(`「${f.name}」是空文件`)
        continue
      }
      try {
        let blob: Blob = f
        let truncated = false
        if (f.size > MAX_ATTACH_BYTES) {
          blob = f.slice(0, MAX_ATTACH_BYTES)
          truncated = true
          Message.warning(`「${f.name}」较大，仅注入前 200KB`)
        }
        const text = await blob.text()
        if (!text.trim()) {
          Message.warning(`「${f.name}」没有可读文本内容`)
          continue
        }
        pendingFiles.value.push({ name: f.name, size: f.size, text, truncated })
      } catch {
        Message.error(`「${f.name}」读取失败`)
      }
    }
  }

  function removeFile(idx: number) {
    pendingFiles.value.splice(idx, 1)
  }

  function prettySize(bytes: number): string {
    return bytes < 1024 ? `${bytes} B` : `${(bytes / 1024).toFixed(1)} KB`
  }

  /** 发送前预热 token：axios 拦截器里已有完整的静默刷新机制，借一次轻量请求触发它 */
  async function ensureFreshToken() {
    if (!shouldRefresh()) return
    try {
      await request.get('/admin/auth/me', { _suppressErrorMsg: true } as never)
    } catch {
      // 刷新失败留给 fetch 的 401 分支兜底
    }
  }

  /** 发送一轮对话并驱动自动循环（工具 agent 续轮 → 分段续写 → 全文校验 → 自动修复）。
   *  用户入口：新提问开启新链；自动轮次由 driveRounds 直接调 sendCore，不经过这里 */
  async function send(rawInput?: string, opts: { hidden?: boolean; continuation?: MessagePart } = {}) {
    const fresh = !opts.hidden && !opts.continuation
    if (fresh) {
      chainSeq++
      stopFlag = false
    }
    const chain = chainSeq
    if (!(await sendCore(rawInput, opts))) return
    await driveRounds(chain)
  }

  /** 单轮流式请求：建消息 → SSE → 收尾 → 执行本轮工具调用并生成结果消息。
   *  返回 false 表示本轮未真正发起（空输入/流进行中）。
   *  agentRound：工具续轮——不消费输入框；rawInput 非空时作为思考备忘
   *  （隐藏 user 消息，携带上轮思考尾部）随本轮发出 */
  async function sendCore(
    rawInput?: string,
    opts: { hidden?: boolean; continuation?: MessagePart; agentRound?: boolean } = {},
  ): Promise<boolean> {
    const content = opts.agentRound ? (rawInput ?? '').trim() : (rawInput ?? input.value).trim()
    const files = opts.agentRound ? [] : [...pendingFiles.value]
    if ((!opts.agentRound && !content && files.length === 0) || streaming.value) return false
    if (!opts.agentRound) {
      input.value = ''
      pendingFiles.value = []
    }

    const parts: MessagePart[] = []
    if (content) parts.push({ type: 'text', content })
    for (const f of files) {
      parts.push({ type: 'file', content: f.text, file: { name: f.name, size: f.size, truncated: f.truncated } })
    }

    // 关键：必须用 reactive 包一层再持有引用。SSE 回调里是就地累加 parts /
    // 追加 thinking，若持有的是原始对象，写入绕过了响应式代理——computed
    // （如按 code part 汇总的版本列表）不会失效，界面也不会随 token 刷新，
    // 表现就是「生成中右侧空白、结束后点右侧查看仍无代码」。
    const userMsg =
      opts.agentRound && !content
        ? null
        : reactive<ChatMsg>({
            id: ++msgSeq,
            role: 'user',
            parts,
            hidden: opts.hidden || opts.agentRound,
            memo: opts.agentRound || undefined,
            skills: [...activeSkills.value],
            ts: Date.now(),
          })
    const assistantMsg = reactive<ChatMsg>({
      id: ++msgSeq,
      role: 'assistant',
      parts: [],
      skills: [...activeSkills.value],
      ts: Date.now(),
    })
    if (userMsg) messages.value.push(userMsg)
    messages.value.push(assistantMsg)

    // 续写：本轮代码增量并回目标代码段（不产生新版本）。
    // 先清掉上一轮的截断标记与报告——若本轮仍被截断，truncated 事件会重新打上；
    // 报告等本轮结束后用 dry-run 对合并全文重算。
    continuationTarget = opts.continuation || null
    if (continuationTarget) {
      continuationTarget.truncated = false
      continuationTarget.pendingMore = false
      continuationTarget.report = undefined
    }

    await ensureFreshToken()
    const token = getAccessToken()
    abortController = new AbortController()
    streaming.value = true

    try {
      const resp = await fetch('/api/admin/ai-assistant/chat', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({
          messages: flattenForApi(messages.value.slice(0, -1)),
          skills: activeSkills.value,
        }),
        signal: abortController.signal,
      })

      // 流开始前的失败：统一 JSON 错误体（未启用/未配置/限流/参数错误）
      if (!resp.ok || !resp.body) {
        let message = `请求失败（${resp.status}）`
        try {
          const data = await resp.json()
          if (data?.message) message = data.message
        } catch {
          // ignore
        }
        assistantMsg.error = message
        Message.error(message)
        return true
      }

      const reader = resp.body.getReader()
      const decoder = new TextDecoder('utf-8')
      let buffer = ''
      let done = false
      while (!done) {
        const { done: streamDone, value } = await reader.read()
        if (streamDone) break
        buffer += decoder.decode(value, { stream: true })
        // SSE 帧以空行分隔；保留不完整尾部等下一个分片
        let sep: number
        while ((sep = buffer.indexOf('\n\n')) >= 0) {
          const frame = buffer.slice(0, sep)
          buffer = buffer.slice(sep + 2)
          done = handleFrame(frame, assistantMsg) || done
        }
      }
    } catch (err) {
      if ((err as Error)?.name === 'AbortError') {
        appendTextDelta(assistantMsg, '\n\n（已停止生成）')
      } else {
        assistantMsg.error = (err as Error)?.message || '网络异常'
        Message.error(assistantMsg.error)
      }
    } finally {
      streaming.value = false
      abortController = null
      // 供驱动循环定位本轮累积的代码段；校验时机由 driveRounds 统一决定
      //（分段中途的片段过不了编译，此时校验只会误报）
      lastRoundContinuation = continuationTarget
      continuationTarget = null
      // 工具轮：流结束后逐个执行工具调用，结果以 role=tool 消息落历史（下一轮请求携带，
      // 模型据此决定继续写/修/收尾——Claude Code 式 agent loop 的「宿主代劳 + 结果回传」）
      if (assistantMsg.toolCalls?.length) {
        lastRoundProduced = 0
        lastRoundHadValidate = false
        lastRoundToolFailures = 0
        lastRoundMutated = false
        for (const tc of assistantMsg.toolCalls) {
          const result = await execTool(tc, assistantMsg)
          messages.value.push(
            reactive<ChatMsg>({
              id: ++msgSeq,
              role: 'tool',
              parts: [{ type: 'text', content: result }],
              toolCallId: tc.id,
              hidden: true,
              ts: Date.now(),
            }) as ChatMsg,
          )
        }
      }
      // 续写时正文与代码都并回了原片段，本轮助手消息可能是空壳：去掉，避免留一行空白
      if (!assistantMsg.parts.length && !assistantMsg.thinking && !assistantMsg.error && !assistantMsg.toolCalls?.length) {
        const idx = messages.value.indexOf(assistantMsg)
        if (idx >= 0) messages.value.splice(idx, 1)
      }
      persist()
    }
    return true
  }

  /** 跨消息找最后一个代码段（工具模式的活动版本：write/append/edit 作用于此） */
  function lastCodePartAll(): MessagePart | null {
    for (let i = messages.value.length - 1; i >= 0; i--) {
      const p = lastCodePart(messages.value[i])
      if (p) return p
    }
    return null
  }

  /** 执行一次工具调用，返回给模型的工具结果文本。
   *  宿主即「文件系统」：code part 是文件，write=新版本、append=续写、edit=定点替换、
   *  validate=dry-run 校验；结果文本经 tool 消息回传，模型据此自我纠正
   *  （如 old_text 不匹配时重新引用，参数被截断时改小分段重调） */
  async function execTool(tc: ToolCallInfo, hostMsg: ChatMsg): Promise<string> {
    let args: Record<string, unknown> = {}
    if (tc.arguments && tc.arguments.trim()) {
      try {
        args = JSON.parse(tc.arguments) as Record<string, unknown>
      } catch {
        lastRoundToolFailures++
        return '工具参数不是合法 JSON（可能因输出达到上限被截断，本次调用未执行）。请把分段改小后重新调用本工具。'
      }
    }
    const lineCount = (s: string) => s.split(/\r?\n/).length
    const strArg = (k: string) => (typeof args[k] === 'string' ? (args[k] as string) : '')
    const activePart = (): MessagePart | null => lastCodePartAll()

    switch (tc.name) {
      case 'write_code': {
        const code = strArg('code')
        if (!code.trim()) return 'code 参数为空，未写入。'
        const part: MessagePart = { type: 'code', content: code, lang: 'js' }
        hostMsg.parts.push(part)
        lastRoundProduced += code.length
        lastRoundMutated = true
        return `已写入 ${lineCount(code)} 行（产生新版本）。后续可继续 append_code 追加或调用 validate_code 校验。`
      }
      case 'append_code': {
        const code = strArg('code')
        if (!code) return 'code 参数为空，未追加。'
        const part = activePart()
        if (!part) return '当前没有源码可追加，请先用 write_code 写入第一段。'
        part.content += code
        lastRoundProduced += code.length
        lastRoundMutated = true
        return `已追加 ${lineCount(code)} 行，当前共 ${lineCount(part.content)} 行。${
          args.done === true ? '文件已完整，建议调用 validate_code 校验。' : '等待后续段。'
        }`
      }
      case 'edit_code': {
        const part = activePart()
        if (!part) return '当前没有源码可修改。'
        const oldText = strArg('old_text')
        const newText = strArg('new_text')
        if (!oldText) return 'old_text 参数为空。'
        const count = part.content.split(oldText).length - 1
        if (count === 0) {
          return `old_text 未在当前源码中找到（当前共 ${lineCount(part.content)} 行）。请先调用 read_code 获取当前源码，再逐字引用（注意空白与缩进），或改用 write_code 全量重写。`
        }
        if (count > 1) {
          return `old_text 在当前源码中匹配 ${count} 处，必须唯一。请扩大引用范围使其唯一。`
        }
        part.content = part.content.replace(oldText, newText)
        lastRoundProduced += newText.length
        lastRoundMutated = true
        return `已替换 1 处，当前共 ${lineCount(part.content)} 行。建议调用 validate_code 复验。`
      }
      case 'read_code': {
        const part = activePart()
        if (!part) return '当前没有源码（会话未载入基线，也尚未写入）。请用 write_code 写入。'
        return `当前完整源码（${lineCount(part.content)} 行）：\n${part.content}`
      }
      case 'validate_code': {
        const part = activePart()
        if (!part) return '当前没有源码可校验。'
        lastRoundHadValidate = true
        await revalidateMerged(part)
        if (!part.report) return '校验服务暂不可用，请稍后重试。'
        if (part.report.ok) return '校验通过（与插件上传自检同口径），可以收尾。'
        const lines: string[] = ['校验未通过（与插件上传自检同口径）：']
        for (const c of part.report.checks || []) lines.push(`- [${c.status}] ${c.name}: ${c.detail}`)
        return lines.join('\n')
      }
      default:
        return `未知工具 ${JSON.stringify(tc.name)}。可用工具：write_code / append_code / read_code / edit_code / validate_code。`
    }
  }

  /** 载入已有插件开启升级会话：新会话 + 右栏以现有源码为基线版本（不进上游历史，
   *  模型经 read_code 读取）+ 一条说明消息引导用户描述升级需求 */
  function injectExistingPlugin(info: { key: string; version: string; source: string }) {
    chainSeq++ // 终止可能在跑的自动循环
    stopFlag = false
    messages.value = []
    pendingFiles.value = []
    const base = reactive<ChatMsg>({
      id: ++msgSeq,
      role: 'assistant',
      parts: [{ type: 'code', content: info.source, lang: 'js' }],
      hidden: true,
      seedOnly: true,
      ts: Date.now(),
    })
    const note = reactive<ChatMsg>({
      id: ++msgSeq,
      role: 'user',
      parts: [{
        type: 'text',
        content: `已载入插件 ${info.key} v${info.version} 的现有源码（右侧「生成的源码」面板即为基线版本，AI 可经 read_code 工具读取）。请直接描述本次升级要改什么——AI 会在现有源码上定点修改（edit_code）而不是重写，完成后自动递增版本号并校验。`,
      }],
      ts: Date.now(),
    })
    messages.value.push(base, note)
    persist()
  }

  // ── 自动驱动循环：分段输出 → 全文校验 → 自动修复 ──────────────────
  //
  // 长插件单轮写不完（max_tokens 截断 / 长输出质量下降），改为多轮自动衔接：
  //   ① 模型按分段协议输出（每轮一个围栏 + PLUGIN_SECTION_DONE / PLUGIN_COMPLETE 标记），
  //      见到「还有后续段」或截断标记就自动发隐藏续写指令，增量并回同一代码段（同一版本）；
  //   ② 全部段拼完后对合并全文跑 dry-run 校验（单轮完整输出则沿用服务端 validate 报告）；
  //   ③ 校验失败自动把失败明细发回模型修复（新版本），修复轮次同样受分段协议保护。
  // 轮次有上限，超限转为手动「续写剩余部分」按钮；停止生成 / 新提问 / 清空会话都会终止循环。

  /** 读取助手消息里的分段协议标记（取最后一次出现的为准） */
  function sectionMarkerOf(msg: ChatMsg): 'section' | 'complete' | null {
    const text = msg.parts.filter((p) => p.type === 'text').map((p) => p.content).join('\n')
    const idxDone = text.lastIndexOf(PLUGIN_SECTION_DONE)
    const idxComplete = text.lastIndexOf(PLUGIN_COMPLETE)
    if (idxComplete < 0 && idxDone < 0) return null
    return idxComplete > idxDone ? 'complete' : 'section'
  }

  /** 从消息正文里剥掉分段标记行（纯协议信号，不进会话展示与上游历史），并清理因此变空的消息 */
  function stripSectionMarkers(msg: ChatMsg) {
    let changed = false
    msg.parts = msg.parts.filter((p) => {
      if (p.type !== 'text') return true
      const kept = p.content
        .split(/\r?\n/)
        .filter((l) => l.trim() !== PLUGIN_SECTION_DONE && l.trim() !== PLUGIN_COMPLETE)
        .join('\n')
      if (kept !== p.content) {
        p.content = kept
        changed = true
      }
      return p.content !== ''
    })
    if (changed && !msg.parts.length && !msg.thinking && !msg.error) {
      const idx = messages.value.indexOf(msg)
      if (idx >= 0) messages.value.splice(idx, 1)
    }
  }

  /** 校验报告是否存在需要修复的缺陷（fail 项或编译错误） */
  function reportNeedsFix(report?: ValidateReport): boolean {
    if (!report || report.ok) return false
    return Boolean(report.error) || (report.checks || []).some((c) => c.status === 'fail')
  }

  /** 组装修复指令：失败明细逐条列出，让模型定点修复（自动轮与手动按钮共用） */
  function buildRepairMessage(report: ValidateReport): string {
    const lines: string[] = ['上一轮生成的代码校验未通过，请修复后重新输出完整代码。失败明细：']
    if (report.error) lines.push(`- ${report.error}`)
    for (const c of report.checks || []) {
      if (c.status === 'fail' || c.status === 'warn') lines.push(`- [${c.name}] ${c.detail}`)
    }
    return lines.join('\n')
  }

  /** 自动驱动循环：chain 为本次链序号（新提问/清空会话后即失效退出）。
   *  双模式统一驱动：
   *  - 工具轮（assistant 带 toolCalls，sendCore 已执行并生成 tool 结果）→ 直接续轮（agent loop）
   *  - 围栏/纯文本轮 → 原分段续写 + 全文校验 + 自动修复逻辑（降级模式与收尾轮共用） */
  async function driveRounds(chain: number) {
    const isPluginSkill = activeSkills.value.includes('js_plugin')
    let continuations = 0
    let repairs = 0
    let agentRounds = 0
    let chainTokens = 0
    let stallRounds = 0
    let toolFailStreak = 0
    while (chain === chainSeq && !stopFlag) {
      // 从尾部回找最后一条助手消息：工具轮执行后消息尾部是 role=tool 的结果消息
      let msg: ChatMsg | undefined
      for (let i = messages.value.length - 1; i >= 0; i--) {
        if (messages.value[i].role === 'assistant') {
          msg = messages.value[i]
          break
        }
      }
      if (!msg) return

      // ── 工具模式：本轮带工具调用（工具已执行、结果已落 tool 消息）→ 续轮 ──
      // 模型看着工具结果决定继续 append/edit/validate 还是输出总结收尾。
      // 续轮前三道止损闸：推理模型的思考不回传历史，每轮重新起草整个文件时会呈现
      // 「单轮数万 token 的思考 + 极少代码产出」的空转形态，必须主动掐断
      if (msg.toolCalls?.length) {
        chainTokens += msg.usage?.completion_tokens || 0
        if (lastRoundToolFailures > 0) {
          toolFailStreak++
        } else {
          toolFailStreak = 0
        }
        // 参数解析失败的轮不计入空转（那是调用失败，归失败闸管）；
        // 空转专指「调用正常执行但几乎没有代码产出」——反复思考的形态
        if (!lastRoundMutated && !lastRoundHadValidate && lastRoundToolFailures === 0) {
          stallRounds++
        } else {
          stallRounds = 0
        }
        if (agentRounds >= MAX_AGENT_ROUNDS) {
          Message.warning(`已达单次生成的 ${MAX_AGENT_ROUNDS} 轮工具调用上限，已停止自动续轮；可继续对话让它接着改`)
          return
        }
        if (chainTokens >= MAX_AGENT_COMPLETION_TOKENS) {
          Message.warning(`本次生成已消耗约 ${Math.round(chainTokens / 1000)}k 输出 token，为控制成本已停止自动续轮；可继续对话接着改，或换用思考更短的模型`)
          return
        }
        if (toolFailStreak >= MAX_AGENT_TOOL_FAILURES) {
          Message.error('模型工具调用的参数连续解析失败，端点对流式 tool_calls 的支持可能不完整，已停止。建议在「AI 助手」设置中换用对 function calling 支持完整的模型/端点')
          return
        }
        if (stallRounds >= MAX_AGENT_STALL_ROUNDS) {
          Message.warning('模型连续多轮几乎没有产出新代码（可能陷入反复思考），已停止自动续轮；可重新生成，或在「AI 助手」设置中换用思考更短的模型')
          return
        }
        agentRounds++
        // 思考备忘：推理模型的 reasoning 不回传历史，续轮会面对几乎相同的上下文
        // 从头重新推理（相邻思考块开头逐字相同即此形态）——携带上轮思考尾部，
        // 让模型直接沿用结论而不是重新分析起草
        const tail = ((msg.thinking || '').trim().match(/[\s\S]{0,2000}$/) || [''])[0]
        const memo = tail
          ? `【上轮思考备忘】以下是你上一轮思考的结尾部分（含你已得出的结论），直接沿用即可，不要从头重新分析或重新起草已有代码：\n${tail}`
          : undefined
        await sendCore(memo, { agentRound: true })
        continue
      }

      const marker = isPluginSkill ? sectionMarkerOf(msg) : null
      if (marker) stripSectionMarkers(msg)
      // 本轮累积的代码段：续写轮并回了旧段（lastRoundContinuation），否则取本轮新建的代码段；
      // 工具链收尾轮（agentRounds>0）的活动代码在更早的消息里，跨消息回找
      const mergedThisRound = Boolean(lastRoundContinuation)
      const part = lastRoundContinuation || lastCodePart(msg) || (agentRounds > 0 ? lastCodePartAll() : null)

      // 没有代码段：模型把额度耗在分析文字上、围栏未开即被截断——发挽救指令要求
      // 直接开始输出代码；纯文字完整回复（如在向用户提问）不自动追问，正常结束
      if (!part) {
        if (msg.truncated && isPluginSkill && continuations < MAX_AUTO_CONTINUATIONS) {
          continuations++
          Message.warning('输出在生成代码前被截断，已要求直接输出代码，正在重试…')
          await sendCore(BOOTSTRAP_CODE_PROMPT, { hidden: true })
          continue
        }
        if (msg.truncated) {
          Message.warning('输出被长度上限截断且尚未产生代码，请重试，或在「AI 助手」设置中调高输出上限')
        }
        return
      }

      // ① 还有后续段（分段标记）或本轮被截断：自动续写，增量并回同一段
      if (part.truncated || marker === 'section') {
        if (continuations >= MAX_AUTO_CONTINUATIONS) {
          part.pendingMore = true
          Message.warning(`已自动续写 ${MAX_AUTO_CONTINUATIONS} 次仍未完成，可点击「续写剩余部分」手动继续`)
          return
        }
        continuations++
        if (continuations === 1) Message.info('输出较长，正在自动分段续写…')
        const prompt = marker === 'section' && !part.truncated ? SECTION_CONTINUE_PROMPT : CONTINUE_PROMPT
        await sendCore(prompt, { hidden: true, continuation: part })
        continue
      }

      // ② 输出完毕：跨轮拼接的全文用 dry-run 重算（服务端只校验过最后一段），
      //    单轮完整输出沿用服务端 validate 报告，避免重复请求
      if (mergedThisRound || !part.report) await revalidateMerged(part)

      // ③ 校验未过：自动把失败明细发回模型修复（有限次），手动按钮仍然可用
      if (isPluginSkill && reportNeedsFix(part.report) && repairs < MAX_AUTO_REPAIRS) {
        repairs++
        await sendCore(buildRepairMessage(part.report!), {})
        continue
      }
      return
    }
  }

  /** 对合并后的源码重新跑上传同款校验（dry-run 端点，不落库） */
  async function revalidateMerged(part: MessagePart) {
    if (!activeSkills.value.includes('js_plugin')) return
    if (!part.content.trim()) return
    try {
      const res: any = await request.post('/admin/js-plugins/dry-run', { source: part.content })
      // 载荷在 res.data.data：request 拦截器返回完整 axios 响应（与 JSPluginUploadModal 同款取法）。
      // 此前直接读 res.pass / res.checks 恒为 undefined，续写后的合并代码一律显示「校验未通过」
      const report = res?.data?.data
      part.report = {
        skill: 'js_plugin',
        ok: Boolean(report?.pass),
        checks: (report?.checks || []).map((c: any) => ({
          name: String(c?.name ?? ''),
          status: String(c?.status ?? ''),
          detail: String(c?.detail ?? ''),
        })),
      }
    } catch {
      // dry-run 失败不阻断：保留上一次报告，用户仍可「用此源码上传」走到同一套校验
    }
  }

  /** 续写被截断 / 还有后续段的代码段：隐藏指令 + 增量并回原段，结束后由驱动循环统一校验 */
  async function continueCode(part: MessagePart) {
    if (streaming.value) return
    stopFlag = false
    const prompt = part.pendingMore && !part.truncated ? SECTION_CONTINUE_PROMPT : CONTINUE_PROMPT
    if (!(await sendCore(prompt, { hidden: true, continuation: part }))) return
    await driveRounds(chainSeq)
  }

  /** 处理一帧 SSE；返回 true 表示收到 [DONE] */
  function handleFrame(frame: string, msg: ChatMsg): boolean {
    for (const line of frame.split('\n')) {
      if (!line.startsWith('data: ')) continue
      const data = line.slice(6)
      if (data === '[DONE]') return true
      let payload: Record<string, unknown>
      try {
        payload = JSON.parse(data)
      } catch {
        continue
      }
      switch (payload.type) {
        case 'thinking':
          // 思考过程单独累积（不进 parts，不参与 markdown / 代码切分）
          msg.thinking = (msg.thinking || '') + String(payload.content ?? '')
          break
        case 'delta':
          appendTextDelta(msg, String(payload.content ?? ''))
          break
        case 'code_start': {
          // 续写：不新建代码段，增量直接并回被截断的那一段
          if (continuationTarget) break
          msg.parts.push({ type: 'code', content: '', lang: String(payload.lang ?? '') })
          break
        }
        case 'code_delta': {
          const text = String(payload.content ?? '')
          if (continuationTarget) {
            continuationTarget.content += text
          } else {
            appendCodeDelta(msg, text)
          }
          break
        }
        case 'code_end':
          break
        case 'truncated': {
          // 上游达到 max_tokens：标记代码段与消息不完整（驱动循环会自动续写；
          // 超过自动续写上限时才提示手动，「修复」按钮换成「续写」由 UI 按 truncated 呈现）。
          // 消息级标记兜住「围栏还没开出就截断」的形态——此时没有 code part 可挂
          const target = continuationTarget || lastCodePart(msg)
          if (target) target.truncated = true
          msg.truncated = true
          break
        }
        case 'tool_start': {
          // 工具调用开始（agent 模式）：占位聚合项，参数增量随后拼入
          const idx = Number(payload.index || 0)
          if (!msg.toolCalls) msg.toolCalls = []
          while (msg.toolCalls.length <= idx) msg.toolCalls.push({ id: '', name: '', arguments: '' })
          if (payload.id) msg.toolCalls[idx].id = String(payload.id)
          if (payload.name) msg.toolCalls[idx].name = String(payload.name)
          break
        }
        case 'tool_delta': {
          const tc = msg.toolCalls?.[Number(payload.index || 0)]
          if (tc) tc.arguments += String(payload.text ?? '')
          break
        }
        case 'tool_end': {
          // 聚合完成事件：以服务端聚合的完整参数为准（增量拼接仅作流式展示）
          const idx = Number(payload.index || 0)
          if (!msg.toolCalls) msg.toolCalls = []
          while (msg.toolCalls.length <= idx) msg.toolCalls.push({ id: '', name: '', arguments: '' })
          msg.toolCalls[idx] = {
            id: String(payload.id ?? msg.toolCalls[idx].id ?? ''),
            name: String(payload.name ?? msg.toolCalls[idx].name ?? ''),
            arguments: String(payload.arguments ?? ''),
          }
          break
        }
        case 'notice':
          Message.info(String(payload.message ?? ''))
          break
        case 'validate': {
          // 续写时服务端只校验了片段，结论无意义（合并后的全文由 dry-run 重新校验）
          if (continuationTarget) break
          // 分段输出中间轮：服务端只看到这一段的围栏，校验的是残缺片段——
          // 跳过，等全部段拼接完由驱动循环对全文统一校验
          if (sectionMarkerOf(msg) === 'section') break
          const report = payload as unknown as ValidateReport
          for (let i = msg.parts.length - 1; i >= 0; i--) {
            if (msg.parts[i].type === 'code') {
              msg.parts[i].report = report
              break
            }
          }
          break
        }
        case 'usage':
          msg.usage = payload as unknown as UsageInfo
          break
        case 'error': {
          const message = String(payload.message ?? '生成失败')
          msg.error = message
          Message.error(message)
          break
        }
      }
    }
    return false
  }

  function stop() {
    stopFlag = true // 同时终止自动驱动循环（停止后不再自动续写/修复）
    abortController?.abort()
  }

  /** 半自动修复环：把校验失败明细拼成下一轮用户消息自动发送 */
  async function repair(report: ValidateReport) {
    if (streaming.value) return
    stopFlag = false
    if (!(await sendCore(buildRepairMessage(report), {}))) return
    await driveRounds(chainSeq)
  }

  function toggleSkill(key: string) {
    if (streaming.value) return
    const idx = activeSkills.value.indexOf(key)
    if (idx >= 0) {
      activeSkills.value.splice(idx, 1)
    } else {
      activeSkills.value.push(key)
    }
  }

  /** 手动校验一个代码段（编辑后重新验证用）：内部与自动校验同口径（dry-run），
   *  校验后持久化——手动编辑的内容不会经过 sendCore，不落盘刷新即丢 */
  async function validatePart(part: MessagePart) {
    await revalidateMerged(part)
    persist()
  }

  /** 外部就地修改（如手动编辑代码段）后的落盘钩子：不校验、幂等 */
  function persistNow() {
    persist()
  }

  return {
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
    toggleSkill,
    addFiles,
    removeFile,
    prettySize,
  }
}
