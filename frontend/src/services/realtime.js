import { apiBaseUrl } from "./api"

function buildRealtimeUrl() {
  const explicitUrl = (import.meta.env.VITE_WS_URL || "").trim()
  if (explicitUrl) {
    return explicitUrl
  }

  const url = new URL(`${apiBaseUrl}/ws`)
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:"
  return url.toString()
}

class RealtimeClient {
  constructor() {
    this.url = buildRealtimeUrl()
    this.socket = null
    this.listeners = new Map()
    this.postSubscriptions = new Set()
    this.reconnectTimer = null
    this.desiredUserId = ""
  }

  connect(userId) {
    const normalizedUserId = (userId || "").trim()
    if (!normalizedUserId) {
      this.disconnect()
      return
    }

    if (this.desiredUserId && this.desiredUserId !== normalizedUserId) {
      this.disconnect()
    }

    this.desiredUserId = normalizedUserId

    if (this.socket && [WebSocket.OPEN, WebSocket.CONNECTING].includes(this.socket.readyState)) {
      return
    }

    this.clearReconnectTimer()

    this.socket = new WebSocket(this.url)
    this.socket.addEventListener("open", () => {
      this.flushPostSubscriptions()
    })

    this.socket.addEventListener("message", (event) => {
      this.handleMessage(event.data)
    })

    this.socket.addEventListener("close", (event) => {
      this.socket = null

      if (!this.desiredUserId) {
        return
      }

      if (event.code === 1008) {
        this.emit({ type: "ws.unauthorized", payload: null })
        return
      }

      this.reconnectTimer = window.setTimeout(() => {
        this.connect(this.desiredUserId)
      }, 1500)
    })
  }

  disconnect() {
    this.desiredUserId = ""
    this.clearReconnectTimer()

    if (this.socket) {
      const socket = this.socket
      this.socket = null
      socket.close(1000, "Client closed the connection")
    }
  }

  subscribePost(postId) {
    const normalizedPostId = (postId || "").trim()
    if (!normalizedPostId) {
      return
    }

    this.postSubscriptions.add(normalizedPostId)
    this.send({ type: "subscribe.post", postId: normalizedPostId })
  }

  unsubscribePost(postId) {
    const normalizedPostId = (postId || "").trim()
    if (!normalizedPostId) {
      return
    }

    this.postSubscriptions.delete(normalizedPostId)
    this.send({ type: "unsubscribe.post", postId: normalizedPostId })
  }

  on(type, listener) {
    const listeners = this.listeners.get(type) || new Set()
    listeners.add(listener)
    this.listeners.set(type, listeners)

    return () => {
      const currentListeners = this.listeners.get(type)
      if (!currentListeners) {
        return
      }

      currentListeners.delete(listener)
      if (!currentListeners.size) {
        this.listeners.delete(type)
      }
    }
  }

  clearReconnectTimer() {
    if (this.reconnectTimer) {
      window.clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }

  flushPostSubscriptions() {
    for (const postId of this.postSubscriptions) {
      this.send({ type: "subscribe.post", postId })
    }
  }

  send(message) {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      return
    }

    this.socket.send(JSON.stringify(message))
  }

  handleMessage(rawMessage) {
    try {
      const parsed = JSON.parse(rawMessage)
      this.emit(parsed)
    } catch {
      // Ignore malformed messages from the server.
    }
  }

  emit(event) {
    const typedListeners = this.listeners.get(event.type) || []
    for (const listener of typedListeners) {
      listener(event)
    }

    const globalListeners = this.listeners.get("*") || []
    for (const listener of globalListeners) {
      listener(event)
    }
  }
}

export const realtimeClient = new RealtimeClient()
