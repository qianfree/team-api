import { ref, computed } from 'vue'
import { Message } from '@arco-design/web-vue'
import request, { getAccessToken, shouldRefresh } from '@/utils/request'
import { useAuthStore } from '@/stores/auth'

/**
 * AI 助手会话状态机：消息模型（parts 数组，代码走专用通道）+ SSE 流式解析
 * + localStorage 持久化 + 半自动修复环。
 *
 * SSE 事件协议见后端 internal/handler/admin/ai_assistant_chat.go：
 *   delta / code_start / code_delta / code_end / validate / usage / error / [DONE]
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

/** 消息分段：text 走 markdown 渲染；code 走专用代码卡片（不经 markdown，防反引号拆碎） */
export interface MessagePart {
  type: 'text' | 'code'
  content: string
  lang?: string
  report?: ValidateReport
}

export interface ChatMsg {
  id: number
  role: 'user' | 'assistant'
  parts: MessagePart[]
  skills?: string[]
  usage?: UsageInfo
  error?: string
  ts: number
}

/** 技能清单（与后端 internal/logic/aiassistant/skills 注册表对齐） */
export const SKILL_OPTIONS = [
  { key: 'billing_formula', label: '计费表达式' },
  { key: 'js_plugin', label: 'JS 插件' },
] as const

const STORAGE_PREFIX = 'admin_ai_assistant_v1_'
const MAX_PERSIST_MESSAGES = 100

let msgSeq = Date.now()

export function useAiAssistantChat() {
  const authStore = useAuthStore()
  const messages = ref<ChatMsg[]>([])
  const streaming = ref(false)
  const input = ref('')
  const activeSkills = ref<string[]>([])
  let abortController: AbortController | null = null

  const canSend = computed(() => !streaming.value && input.value.trim().length > 0)

  function storageKey(): string {
    return STORAGE_PREFIX + (authStore.user?.id ?? 0)
  }

  function persist() {
    try {
      const trimmed = messages.value.slice(-MAX_PERSIST_MESSAGES)
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
    try {
      localStorage.removeItem(storageKey())
    } catch {
      // ignore
    }
  }

  /** 把 assistant 的 parts 还原为上游可读的纯文本（代码重建为四反引号围栏，让模型看到自己上一轮的产物） */
  function flattenForApi(msgs: ChatMsg[]): { role: string; content: string }[] {
    return msgs.map((m) => {
      if (m.role === 'user') {
        return { role: 'user', content: m.parts.map((p) => p.content).join('') }
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

  function appendCodeDelta(msg: ChatMsg, delta: string) {
    const last = msg.parts[msg.parts.length - 1]
    if (last && last.type === 'code') {
      last.content += delta
    } else {
      msg.parts.push({ type: 'code', content: delta })
    }
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

  async function send(rawInput?: string) {
    const content = (rawInput ?? input.value).trim()
    if (!content || streaming.value) return
    input.value = ''

    const userMsg: ChatMsg = {
      id: ++msgSeq,
      role: 'user',
      parts: [{ type: 'text', content }],
      skills: [...activeSkills.value],
      ts: Date.now(),
    }
    const assistantMsg: ChatMsg = {
      id: ++msgSeq,
      role: 'assistant',
      parts: [],
      skills: [...activeSkills.value],
      ts: Date.now(),
    }
    messages.value.push(userMsg, assistantMsg)

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
      persist()
    }
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
        case 'delta':
          appendTextDelta(msg, String(payload.content ?? ''))
          break
        case 'code_start': {
          msg.parts.push({ type: 'code', content: '', lang: String(payload.lang ?? '') })
          break
        }
        case 'code_delta': {
          appendCodeDelta(msg, String(payload.content ?? ''))
          break
        }
        case 'code_end':
          break
        case 'validate': {
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
    canSend,
    restore,
    clearAll,
    send,
    stop,
    repair,
    toggleSkill,
  }
}
