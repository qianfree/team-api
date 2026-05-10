import { ref } from 'vue'
import { getAccessToken } from './request'

export interface WsMessage {
  channel: string
  action: string
  payload?: any
  seq?: number
}

type MessageHandler = (msg: WsMessage) => void

const wsConnected = ref(false)

class WsManager {
  private ws: WebSocket | null = null
  private handlers = new Map<string, Set<MessageHandler>>()
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private reconnectDelay = 1000
  private maxReconnectDelay = 30000
  private disposed = false

  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN || this.ws?.readyState === WebSocket.CONNECTING) {
      return
    }

    const token = getAccessToken()
    if (!token) return

    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${location.host}/api/admin/ws?token=${encodeURIComponent(token)}`
    this.ws = new WebSocket(url)
    this.disposed = false

    this.ws.onopen = () => {
      this.reconnectDelay = 1000
      wsConnected.value = true
    }

    this.ws.onmessage = (event) => {
      try {
        const msg: WsMessage = JSON.parse(event.data)
        const channelHandlers = this.handlers.get(msg.channel)
        if (channelHandlers) {
          channelHandlers.forEach((handler) => handler(msg))
        }
        // Also notify wildcard handlers
        const wildcardHandlers = this.handlers.get('*')
        if (wildcardHandlers) {
          wildcardHandlers.forEach((handler) => handler(msg))
        }
      } catch {
        // ignore malformed messages
      }
    }

    this.ws.onclose = () => {
      wsConnected.value = false
      this.scheduleReconnect()
    }

    this.ws.onerror = () => {
      // onclose will fire after this
    }
  }

  private scheduleReconnect(): void {
    if (this.disposed) return
    if (this.reconnectTimer) return

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.connect()
    }, this.reconnectDelay)

    this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay)
  }

  disconnect(): void {
    this.disposed = true
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    if (this.ws) {
      this.ws.onclose = null
      this.ws.close()
      this.ws = null
    }
  }

  on(channel: string, handler: MessageHandler): void {
    if (!this.handlers.has(channel)) {
      this.handlers.set(channel, new Set())
    }
    this.handlers.get(channel)!.add(handler)
  }

  off(channel: string, handler: MessageHandler): void {
    const channelHandlers = this.handlers.get(channel)
    if (channelHandlers) {
      channelHandlers.delete(handler)
      if (channelHandlers.size === 0) {
        this.handlers.delete(channel)
      }
    }
  }

  get isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }
}

// Singleton instance
export const wsManager = new WsManager()

// Composable for Vue components
export function useWebSocket(): {
  on: (channel: string, handler: MessageHandler) => void
  off: (channel: string, handler: MessageHandler) => void
  connected: typeof wsConnected
} {
  // Auto-connect on first use if not already connected
  if (!wsManager.isConnected) {
    wsManager.connect()
  }

  return {
    on: (channel: string, handler: MessageHandler) => wsManager.on(channel, handler),
    off: (channel: string, handler: MessageHandler) => wsManager.off(channel, handler),
    connected: wsConnected,
  }
}
