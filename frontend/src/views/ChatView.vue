<script setup>
import { computed, onBeforeUnmount, reactive, ref, watch } from "vue"
import { useRoute, useRouter } from "vue-router"

import {
  fetchChatConversations,
  fetchConversation,
  fetchDiscoverUsers,
  isApiError,
  markConversationRead,
  sendPrivateMessage
} from "../services/api"
import { realtimeClient } from "../services/realtime"
import { useAppStore } from "../stores/app"

const store = useAppStore()
const route = useRoute()
const router = useRouter()

const conversations = ref([])
const discoverUsers = ref([])
const conversationUser = ref(null)
const messages = ref([])
const pageError = ref("")
const threadError = ref("")
const isLoadingSidebar = ref(false)
const isLoadingThread = ref(false)
const isSendingMessage = ref(false)
const composer = reactive({
  body: ""
})

const removeRealtimeListeners = []
const isAuthenticated = computed(() => Boolean(store.state.currentUser))
const currentUserId = computed(() => store.state.currentUser?.id || "")
const activeConversationUserId = computed(() => routeConversationUserId())
let latestThreadRequest = 0

function routeConversationUserId(rawValue = route.query.user) {
  if (Array.isArray(rawValue)) {
    return String(rawValue[0] || "").trim()
  }

  return typeof rawValue === "string" ? rawValue.trim() : ""
}

function displayName(user) {
  if (!user) {
    return ""
  }

  return user.nickname || `${user.firstName} ${user.lastName}`.trim() || "Unknown account"
}

function userInitials(user) {
  const source = displayName(user)
  return source
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() || "")
    .join("") || "N"
}

function isMine(message) {
  return message?.senderId === currentUserId.value
}

function hasMessage(messageId) {
  return messages.value.some((message) => message.id === messageId)
}

function sortConversations(items) {
  return [...items].sort((left, right) => {
    const rightTime = Date.parse(right.updatedAt || right.lastMessage?.sentAt || 0)
    const leftTime = Date.parse(left.updatedAt || left.lastMessage?.sentAt || 0)
    return rightTime - leftTime
  })
}

function conversationPreview(message) {
  if (!message) {
    return "Start the conversation."
  }

  const prefix = isMine(message) ? "You: " : ""
  const body = String(message.body || "").trim()
  return `${prefix}${body}`.slice(0, 72)
}

function formatConversationTime(value) {
  if (!value) {
    return ""
  }

  const date = new Date(value)
  const now = new Date()
  const sameDay = date.toDateString() === now.toDateString()

  return new Intl.DateTimeFormat("en-GB", sameDay ? { hour: "2-digit", minute: "2-digit" } : {
    day: "2-digit",
    month: "short"
  }).format(date)
}

function formatMessageTime(value) {
  return new Intl.DateTimeFormat("en-GB", {
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(value))
}

function sameConversation(left, right) {
  return String(left || "").trim() === String(right || "").trim()
}

function unreadMessagesForActiveConversation(userId) {
  return messages.value.filter((message) => message.senderId === userId && !message.readAt)
}

function selectedConversation() {
  return conversations.value.find((conversation) => sameConversation(conversation.user.id, activeConversationUserId.value)) || null
}

function patchConversationSummary(user, message, unreadCount = null) {
  if (!user || !message) {
    return
  }

  const existing = conversations.value.find((conversation) => sameConversation(conversation.user.id, user.id))
  const nextUnreadCount = unreadCount === null ? existing?.unreadCount || 0 : unreadCount

  const nextSummary = {
    user,
    lastMessage: message,
    unreadCount: nextUnreadCount,
    updatedAt: message.sentAt
  }

  if (existing) {
    conversations.value = sortConversations(
      conversations.value.map((conversation) =>
        sameConversation(conversation.user.id, user.id) ? nextSummary : conversation
      )
    )
    return
  }

  conversations.value = sortConversations([nextSummary, ...conversations.value])
}

function applyConversationReadResult(result) {
  if (!result?.conversationUserId) {
    return
  }

  const readAt = result.readAt || null
  const messageIds = new Set(result.messageIds || [])

  if (sameConversation(activeConversationUserId.value, result.conversationUserId) && readAt && messageIds.size) {
    messages.value = messages.value.map((message) =>
      messageIds.has(message.id)
        ? { ...message, readAt }
        : message
    )
  }

  conversations.value = conversations.value.map((conversation) => {
    if (!sameConversation(conversation.user.id, result.conversationUserId)) {
      return conversation
    }

    const lastMessage = readAt && messageIds.has(conversation.lastMessage?.id)
      ? { ...conversation.lastMessage, readAt }
      : conversation.lastMessage

    return {
      ...conversation,
      unreadCount: 0,
      lastMessage
    }
  })
}

function appendRealtimeMessage(conversationUserId, message) {
  if (!message?.id || !conversationUserId) {
    return
  }

  const knownConversation = conversations.value.find((conversation) => sameConversation(conversation.user.id, conversationUserId))
  if (knownConversation) {
    const shouldIncreaseUnread = message.senderId === conversationUserId && !sameConversation(activeConversationUserId.value, conversationUserId)
    patchConversationSummary(
      knownConversation.user,
      message,
      shouldIncreaseUnread ? knownConversation.unreadCount + 1 : knownConversation.unreadCount
    )
  } else if (sameConversation(activeConversationUserId.value, conversationUserId) && conversationUser.value) {
    patchConversationSummary(conversationUser.value, message, 0)
  } else {
    void loadSidebarData()
  }

  if (!sameConversation(activeConversationUserId.value, conversationUserId)) {
    return
  }

  if (!hasMessage(message.id)) {
    messages.value = [...messages.value, message]
  }

  if (message.senderId === conversationUserId) {
    void markActiveConversationRead(conversationUserId)
  }
}

async function markActiveConversationRead(userId) {
  if (!userId) {
    return
  }

  const unreadMessages = unreadMessagesForActiveConversation(userId)
  if (!unreadMessages.length) {
    conversations.value = conversations.value.map((conversation) =>
      sameConversation(conversation.user.id, userId)
        ? { ...conversation, unreadCount: 0 }
        : conversation
    )
    return
  }

  try {
    const result = await markConversationRead(userId)
    applyConversationReadResult(result)
  } catch {
    // Keep the UI responsive even if the read receipt cannot be persisted immediately.
  }
}

async function loadSidebarData() {
  if (!isAuthenticated.value) {
    conversations.value = []
    discoverUsers.value = []
    pageError.value = ""
    return
  }

  isLoadingSidebar.value = true
  pageError.value = ""

  try {
    const [nextConversations, nextDiscoverUsers] = await Promise.all([
      fetchChatConversations(),
      fetchDiscoverUsers()
    ])

    conversations.value = sortConversations(nextConversations)
    discoverUsers.value = nextDiscoverUsers

    const userIdFromRoute = routeConversationUserId()
    if (!userIdFromRoute && nextConversations[0]?.user?.id) {
      await router.replace({
        path: "/chat",
        query: { user: nextConversations[0].user.id }
      })
    }
  } catch (error) {
    pageError.value = error instanceof Error ? error.message : "Could not load the chat list."
  } finally {
    isLoadingSidebar.value = false
  }
}

async function loadConversation(userId) {
  const normalizedUserId = String(userId || "").trim()
  latestThreadRequest += 1
  const requestID = latestThreadRequest

  if (!normalizedUserId) {
    conversationUser.value = null
    messages.value = []
    threadError.value = ""
    return
  }

  isLoadingThread.value = true
  threadError.value = ""

  try {
    const thread = await fetchConversation(normalizedUserId)
    if (requestID !== latestThreadRequest) {
      return
    }

    conversationUser.value = thread.user
    messages.value = thread.messages || []

    const latestMessage = thread.messages?.[thread.messages.length - 1]
    if (thread.user && latestMessage) {
      patchConversationSummary(thread.user, latestMessage, selectedConversation()?.unreadCount || 0)
    }

    await markActiveConversationRead(normalizedUserId)
  } catch (error) {
    if (requestID !== latestThreadRequest) {
      return
    }

    conversationUser.value = null
    messages.value = []
    threadError.value = error instanceof Error ? error.message : "Could not load this conversation."
  } finally {
    if (requestID === latestThreadRequest) {
      isLoadingThread.value = false
    }
  }
}

async function navigateToConversation(userId) {
  const normalizedUserId = String(userId || "").trim()
  if (!normalizedUserId) {
    return
  }

  await router.replace({
    path: "/chat",
    query: { user: normalizedUserId }
  })
}

async function submitMessage() {
  if (!activeConversationUserId.value) {
    return
  }

  const body = composer.body.trim()
  if (!body) {
    threadError.value = "Write a message before sending."
    return
  }

  isSendingMessage.value = true
  threadError.value = ""

  try {
    const message = await sendPrivateMessage(activeConversationUserId.value, { body })
    if (!hasMessage(message.id)) {
      messages.value = [...messages.value, message]
    }

    if (conversationUser.value) {
      patchConversationSummary(conversationUser.value, message, 0)
    }

    composer.body = ""
  } catch (error) {
    if (isApiError(error)) {
      threadError.value = error.payload?.fields?.body || error.message
    } else {
      threadError.value = "Could not send the message right now."
    }
  } finally {
    isSendingMessage.value = false
  }
}

const suggestedConversationUsers = computed(() => {
  const knownUserIDs = new Set(conversations.value.map((conversation) => conversation.user.id))
  return discoverUsers.value
    .filter((user) => !knownUserIDs.has(user.id) || sameConversation(user.id, activeConversationUserId.value))
    .slice(0, 8)
})

watch(
  () => store.state.currentUser?.id,
  (userID) => {
    if (!userID) {
      conversations.value = []
      discoverUsers.value = []
      conversationUser.value = null
      messages.value = []
      composer.body = ""
      return
    }

    void loadSidebarData()

    const userIdFromRoute = routeConversationUserId()
    if (userIdFromRoute) {
      void loadConversation(userIdFromRoute)
    }
  },
  { immediate: true }
)

watch(
  () => route.query.user,
  (queryUser) => {
    if (!isAuthenticated.value) {
      return
    }

    void loadConversation(routeConversationUserId(queryUser))
  },
  { immediate: true }
)

removeRealtimeListeners.push(
  realtimeClient.on("chat.message.created", (event) => {
    appendRealtimeMessage(event?.payload?.conversationUserId, event?.payload?.message)
  }),
  realtimeClient.on("chat.conversation.read", (event) => {
    applyConversationReadResult(event?.payload)
  })
)

onBeforeUnmount(() => {
  removeRealtimeListeners.splice(0).forEach((dispose) => dispose())
})
</script>

<template>
  <section class="page">
    <div class="panel">
      <p class="eyebrow">Chat</p>
      <h2>Private conversations in real time</h2>
      <p>
        Persistent threads, live delivery through WebSocket, and read receipts that keep both sides synchronized without leaving the page.
      </p>
    </div>

    <template v-if="isAuthenticated">
      <div class="chat-layout">
        <aside class="panel chat-sidebar">
          <div class="chat-sidebar__section">
            <div class="feed-header">
              <div>
                <p class="eyebrow">Inbox</p>
                <h3>Conversations</h3>
              </div>
              <span class="badge badge--neutral">{{ conversations.length }}</span>
            </div>

            <p v-if="pageError" class="form-error">{{ pageError }}</p>
            <p v-else-if="isLoadingSidebar" class="feed-note">Loading conversations...</p>

            <div v-else class="chat-thread-list">
              <button
                v-for="conversation in conversations"
                :key="conversation.user.id"
                type="button"
                class="chat-thread"
                :class="{ 'chat-thread--active': activeConversationUserId === conversation.user.id }"
                @click="navigateToConversation(conversation.user.id)"
              >
                <span class="user-avatar user-avatar--small">
                  <img
                    v-if="conversation.user.avatarUrl"
                    :src="conversation.user.avatarUrl"
                    :alt="`${displayName(conversation.user)} avatar`"
                    class="user-avatar__image"
                  />
                  <span v-else class="user-avatar__fallback">{{ userInitials(conversation.user) }}</span>
                </span>

                <span class="chat-thread__content">
                  <span class="chat-thread__head">
                    <strong>{{ displayName(conversation.user) }}</strong>
                    <small>{{ formatConversationTime(conversation.updatedAt) }}</small>
                  </span>
                  <span class="chat-thread__body">
                    <small>{{ conversationPreview(conversation.lastMessage) }}</small>
                    <span v-if="conversation.unreadCount" class="chat-thread__badge">
                      {{ conversation.unreadCount }}
                    </span>
                  </span>
                </span>
              </button>

              <p v-if="!conversations.length" class="feed-note">
                No conversations yet. Pick someone below and start the first one.
              </p>
            </div>
          </div>

          <div class="chat-sidebar__section">
            <div class="feed-header">
              <div>
                <p class="eyebrow">Start</p>
                <h3>Discover people</h3>
              </div>
              <span class="badge badge--soft">{{ suggestedConversationUsers.length }}</span>
            </div>

            <div class="user-stack">
              <button
                v-for="user in suggestedConversationUsers"
                :key="user.id"
                type="button"
                class="user-card chat-user-card"
                @click="navigateToConversation(user.id)"
              >
                <span class="chat-user-card__identity">
                  <span class="user-avatar user-avatar--small">
                    <img
                      v-if="user.avatarUrl"
                      :src="user.avatarUrl"
                      :alt="`${displayName(user)} avatar`"
                      class="user-avatar__image"
                    />
                    <span v-else class="user-avatar__fallback">{{ userInitials(user) }}</span>
                  </span>
                  <span>
                    <strong>{{ displayName(user) }}</strong>
                    <small>{{ user.aboutMe || "Open a private conversation." }}</small>
                  </span>
                </span>
                <span class="badge">{{ user.profileVisibility }}</span>
              </button>
            </div>
          </div>
        </aside>

        <section class="panel chat-panel">
          <template v-if="activeConversationUserId && conversationUser">
            <header class="chat-panel__header">
              <div class="chat-panel__identity">
                <span class="user-avatar user-avatar--small">
                  <img
                    v-if="conversationUser.avatarUrl"
                    :src="conversationUser.avatarUrl"
                    :alt="`${displayName(conversationUser)} avatar`"
                    class="user-avatar__image"
                  />
                  <span v-else class="user-avatar__fallback">{{ userInitials(conversationUser) }}</span>
                </span>

                <div>
                  <h3>{{ displayName(conversationUser) }}</h3>
                  <p class="feed-note">
                    {{ conversationUser.profileVisibility }} profile
                  </p>
                </div>
              </div>
              <span class="badge badge--neutral">{{ messages.length }} messages</span>
            </header>

            <p v-if="threadError" class="form-error">{{ threadError }}</p>

            <div class="chat-message-list">
              <p v-if="isLoadingThread" class="feed-note">Loading conversation...</p>
              <p v-else-if="!messages.length" class="feed-note">
                No messages yet. Say hello and start the thread.
              </p>

              <article
                v-for="message in messages"
                :key="message.id"
                class="chat-bubble"
                :class="{
                  'chat-bubble--mine': isMine(message),
                  'chat-bubble--theirs': !isMine(message)
                }"
              >
                <p>{{ message.body }}</p>
                <footer>
                  <span>{{ formatMessageTime(message.sentAt) }}</span>
                  <span v-if="isMine(message)">{{ message.readAt ? "Read" : "Sent" }}</span>
                </footer>
              </article>
            </div>

            <form class="chat-composer" @submit.prevent="submitMessage">
              <label>
                <span class="visually-hidden">Write a message</span>
                <textarea
                  v-model="composer.body"
                  rows="3"
                  maxlength="2000"
                  placeholder="Write a private message..."
                ></textarea>
              </label>

              <div class="chat-composer__actions">
                <p class="feed-note">Messages are delivered live through your NEXO realtime channel.</p>
                <button type="submit" class="button" :disabled="isSendingMessage">
                  {{ isSendingMessage ? "Sending..." : "Send message" }}
                </button>
              </div>
            </form>
          </template>

          <template v-else>
            <div class="chat-empty-state">
              <p class="eyebrow">Private chat</p>
              <h3>Choose a conversation</h3>
              <p>
                Open one of the recent threads or start a new conversation from the discover list.
              </p>
            </div>
          </template>
        </section>
      </div>
    </template>

    <div v-else class="panel panel--narrow">
      <p class="eyebrow">Chat</p>
      <h3>Login required</h3>
      <p>Sign in to unlock private conversations, live delivery, and read receipts.</p>
    </div>
  </section>
</template>
