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
}

export interface ChatMsg {
  id: number
  role: 'user' | 'assistant'
  parts: MessagePart[]
  /** 隐藏消息（如「续写剩余部分」的指令）：仍参与上游上下文，但不在会话里渲染 */
  hidden?: boolean
  /** 思考过程全文（推理型模型的 reasoning_content；仅助手消息） */
  thinking?: string
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
   *  - 助手代码重建为四反引号围栏（让模型看到自己上一轮的产物） */
  function flattenForApi(msgs: ChatMsg[]): { role: string; content: string }[] {
    return msgs.map((m) => {
      if (m.role === 'user') {
        const content = m.parts
          .map((p) => {
            if (p.type === 'file') {
              return `\n\n[附件 ${p.file?.name || '未命名'}]\n\`\`\`\`\n${p.content}\n\`\`\`\`\n`
            }
            return p.content
          })
          .join('')
        return { role: 'user', content }
      }
      const text = m.parts
        .map((p) => {
          if (p.type === 'text') return p.content
          return '\n````' + (p.lang || '') + '\n' + p.content + '\n````\n'
        })
        .join('')
      return { role: 'assistant', content: text.trim() }
    })
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

  async function send(rawInput?: string, opts: { hidden?: boolean; continuation?: MessagePart } = {}) {
    const content = (rawInput ?? input.value).trim()
    const files = [...pendingFiles.value]
    if ((!content && files.length === 0) || streaming.value) return
    input.value = ''
    pendingFiles.value = []

    const parts: MessagePart[] = []
    if (content) parts.push({ type: 'text', content })
    for (const f of files) {
      parts.push({ type: 'file', content: f.text, file: { name: f.name, size: f.size, truncated: f.truncated } })
    }

    // 关键：必须用 reactive 包一层再持有引用。SSE 回调里是就地累加 parts /
    // 追加 thinking，若持有的是原始对象，写入绕过了响应式代理——computed
    // （如按 code part 汇总的版本列表）不会失效，界面也不会随 token 刷新，
    // 表现就是「生成中右侧空白、结束后点右侧查看仍无代码」。
    const userMsg = reactive<ChatMsg>({
      id: ++msgSeq,
      role: 'user',
      parts,
      hidden: opts.hidden,
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
    messages.value.push(userMsg, assistantMsg)

    // 续写：本轮代码增量并回目标代码段（不产生新版本）。
    // 先清掉上一轮的截断标记与报告——若本轮仍被截断，truncated 事件会重新打上；
    // 报告等本轮结束后用 dry-run 对合并全文重算。
    continuationTarget = opts.continuation || null
    if (continuationTarget) {
      continuationTarget.truncated = false
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
        return
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
      const target = continuationTarget
      continuationTarget = null
      if (target) {
        // 续写完成的代码只存在于前端（服务端只见过片段），必须对合并后的全文重新校验
        await revalidateMerged(target)
      }
      // 续写时正文与代码都并回了原片段，本轮助手消息可能是空壳：去掉，避免留一行空白
      if (!assistantMsg.parts.length && !assistantMsg.thinking && !assistantMsg.error) {
        const idx = messages.value.indexOf(assistantMsg)
        if (idx >= 0) messages.value.splice(idx, 1)
      }
      persist()
    }
  }

  /** 对合并后的源码重新跑上传同款校验（dry-run 端点，不落库） */
  async function revalidateMerged(part: MessagePart) {
    if (!activeSkills.value.includes('js_plugin')) return
    if (!part.content.trim()) return
    try {
      const res: any = await request.post('/admin/js-plugins/dry-run', { source: part.content })
      part.report = {
        skill: 'js_plugin',
        ok: Boolean(res?.pass),
        checks: (res?.checks || []).map((c: any) => ({
          name: String(c?.name ?? ''),
          status: String(c?.status ?? ''),
          detail: String(c?.detail ?? ''),
        })),
      }
    } catch {
      // dry-run 失败不阻断：保留上一次报告，用户仍可「用此源码上传」走到同一套校验
    }
  }

  /** 续写被截断的代码段：隐藏指令 + 增量并回原段 + 结束后合并校验 */
  async function continueCode(part: MessagePart) {
    if (streaming.value) return
    await send(CONTINUE_PROMPT, { hidden: true, continuation: part })
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
          // 上游达到 max_tokens：标记代码段不完整，UI 把「修复」换成「续写」
          const target = continuationTarget || lastCodePart(msg)
          if (target) target.truncated = true
          Message.warning('输出达到长度上限，代码不完整，可点「续写剩余部分」接着写')
          break
        }
        case 'notice':
          Message.info(String(payload.message ?? ''))
          break
        case 'validate': {
          // 续写时服务端只校验了片段，结论无意义（合并后的全文由 dry-run 重新校验）
          if (continuationTarget) break
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
    abortController?.abort()
  }

  /** 半自动修复环：把校验失败明细拼成下一轮用户消息自动发送 */
  async function repair(report: ValidateReport) {
    const lines: string[] = ['上一轮生成的代码校验未通过，请修复后重新输出完整代码。失败明细：']
    if (report.error) lines.push(`- ${report.error}`)
    for (const c of report.checks || []) {
      if (c.status === 'fail' || c.status === 'warn') {
        lines.push(`- [${c.name}] ${c.detail}`)
      }
    }
    await send(lines.join('\n'))
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
    toggleSkill,
    addFiles,
    removeFile,
    prettySize,
  }
}
