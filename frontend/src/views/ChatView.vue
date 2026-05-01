<script setup>
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from "vue"
import { useRoute, useRouter } from "vue-router"

import {
  fetchChatConversations,
  fetchDiscoverUsers,
  fetchGroup,
  fetchGroupMembers,
  fetchGroupMessages,
  fetchGroups,
  fetchNotifications,
  isApiError,
  markNotificationRead,
  normalizeGroupMessage,
  sendGroupMessage,
  sendPrivateMessage
} from "../services/api"
import EmojiPicker from "../components/ui/EmojiPicker.vue"
import { realtimeClient } from "../services/realtime"
import { useAppStore } from "../stores/app"
import { formatDate as formatAppDate, formatTime } from "../utils/date"

const store = useAppStore()
const route = useRoute()
const router = useRouter()

const conversations = ref([])
const groupChats = ref([])
const discoverUsers = ref([])
const conversationUser = ref(null)
const activeGroup = ref(null)
const groupMembers = ref([])
const messages = ref([])
const pageError = ref("")
const threadError = ref("")
const isLoadingSidebar = ref(false)
const isLoadingThread = ref(false)
const isLoadingGroupMembers = ref(false)
const isSendingMessage = ref(false)
const isLoadingHistory = ref(false)
const historyHasMore = ref(false)
const activeHistoryRequestId = ref("")
const pendingHistoryAdjustments = reactive({
  prepend: false,
  previousHeight: 0,
  previousTop: 0
})
const messageList = ref(null)
const composer = reactive({
  body: ""
})

const removeRealtimeListeners = []
const isAuthenticated = computed(() => Boolean(store.state.currentUser))
const currentUserId = computed(() => store.state.currentUser?.id || "")
const activeConversationUserId = computed(() => routeConversationUserId())
const activeGroupId = computed(() => routeGroupId())
const isGroupThread = computed(() => Boolean(activeGroupId.value))
const hasSelectedThread = computed(() => Boolean(activeGroupId.value || activeConversationUserId.value))
const notificationCleanupLoading = reactive({})
const GROUP_INVITE_MESSAGE_PREFIX = "[nexo-group-invite]?"

const MESSAGE_GROUP_WINDOW_MS = 2 * 60 * 1000
const HISTORY_PAGE_SIZE = 10

function routeConversationUserId(rawValue = route.query.user) {
  if (Array.isArray(rawValue)) {
    return String(rawValue[0] || "").trim()
  }

  return typeof rawValue === "string" ? rawValue.trim() : ""
}

function routeGroupId(rawValue = route.query.group) {
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

function initialsFromText(value, fallback = "N") {
  return String(value || "")
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() || "")
    .join("") || fallback
}

function userInitials(user) {
  return initialsFromText(displayName(user), "N")
}

function groupInitials(group) {
  return initialsFromText(group?.title, "G")
}

function groupMemberCountLabel(group = activeGroup.value) {
  const count = group?.membersCount || groupMembers.value.length || 0
  return `${count} ${count === 1 ? "member" : "members"}`
}

function groupMemberRoleLabel(member) {
  return member?.role === "creator" ? "Creator" : "Member"
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

function sortGroupChats(items) {
  return [...items].sort((left, right) => {
    const rightTime = Date.parse(right.lastMessage?.sentAt || right.updatedAt || right.createdAt || 0)
    const leftTime = Date.parse(left.lastMessage?.sentAt || left.updatedAt || left.createdAt || 0)
    return rightTime - leftTime
  })
}

function conversationPreview(message) {
  if (!message) {
    return "Start the conversation."
  }

  const invite = messageGroupInvite(message)
  if (invite) {
    const prefix = isMine(message) ? "You: " : ""
    return `${prefix}Group invite: ${invite.title}`.slice(0, 72)
  }

  const prefix = isMine(message) ? "You: " : ""
  const body = String(message.body || "").trim()
  return `${prefix}${body}`.slice(0, 72)
}

function groupChatPreview(group) {
  if (group?.lastMessage) {
    const message = group.lastMessage
    const author = message.senderId === currentUserId.value ? "You" : displayName(message.sender)
    return `${author}: ${String(message.body || "").trim()}`.slice(0, 72)
  }

  return `${group?.membersCount || 0} members · ${group?.postsCount || 0} posts`
}

function groupChatUpdatedAt(group) {
  return group?.lastMessage?.sentAt || group?.updatedAt || group?.createdAt
}

function parseGroupInviteMessage(body) {
  const normalizedBody = String(body || "").trim()
  if (!normalizedBody.startsWith(GROUP_INVITE_MESSAGE_PREFIX)) {
    return null
  }

  const query = normalizedBody.slice(GROUP_INVITE_MESSAGE_PREFIX.length)
  const params = new URLSearchParams(query)
  const groupId = String(params.get("groupId") || "").trim()
  const title = String(params.get("title") || "").trim()
  const note = String(params.get("note") || "").trim()

  if (!groupId || !title) {
    return null
  }

  return {
    groupId,
    title,
    note
  }
}

function messageGroupInvite(message) {
  return parseGroupInviteMessage(message?.body)
}

function openInvitedGroup(groupId) {
  const normalizedGroupId = String(groupId || "").trim()
  if (!normalizedGroupId) {
    return
  }

  void router.push({
    name: "groups",
    params: { groupId: normalizedGroupId }
  })
}

function formatConversationTime(value) {
  return formatAppDate(value)
}

function formatMessageTime(value) {
  return formatTime(value)
}

function formatMessageDay(value) {
  return formatAppDate(value)
}

function dayKey(value) {
  const date = new Date(value)
  return `${date.getFullYear()}-${date.getMonth() + 1}-${date.getDate()}`
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

function knownConversationUser(userId) {
  const normalizedUserId = String(userId || "").trim()
  if (!normalizedUserId) {
    return null
  }

  return (
    conversations.value.find((conversation) => sameConversation(conversation.user.id, normalizedUserId))?.user ||
    discoverUsers.value.find((user) => sameConversation(user.id, normalizedUserId)) ||
    (sameConversation(conversationUser.value?.id, normalizedUserId) ? conversationUser.value : null)
  )
}

function knownGroup(groupId) {
  const normalizedGroupId = String(groupId || "").trim()
  if (!normalizedGroupId) {
    return null
  }

  return (
    groupChats.value.find((group) => sameConversation(group.id, normalizedGroupId)) ||
    (sameConversation(activeGroup.value?.id, normalizedGroupId) ? activeGroup.value : null)
  )
}

async function clearConversationNotifications(conversationUserId) {
  const normalizedConversationUserId = String(conversationUserId || "").trim()
  if (!normalizedConversationUserId || !isAuthenticated.value || notificationCleanupLoading[normalizedConversationUserId]) {
    return
  }

  notificationCleanupLoading[normalizedConversationUserId] = true

  try {
    const notifications = await fetchNotifications()
    const directMessageNotifications = notifications.filter(
      (notification) =>
        !notification.isRead &&
        notification.type === "direct_message" &&
        notification.entityType === "conversation" &&
        sameConversation(notification.entityId, normalizedConversationUserId)
    )

    if (!directMessageNotifications.length) {
      return
    }

    await Promise.allSettled(
      directMessageNotifications.map((notification) => markNotificationRead(notification.id))
    )
  } catch {
    // Ignore notification cleanup failures to keep the chat accessible.
  } finally {
    notificationCleanupLoading[normalizedConversationUserId] = false
  }
}

const groupedMessageDays = computed(() => {
  const days = []
  let currentDay = null
  let currentGroup = null

  for (const message of messages.value) {
    const currentDayKey = dayKey(message.sentAt)
    const mine = isMine(message)

    if (!currentDay || currentDay.key !== currentDayKey) {
      currentDay = {
        key: currentDayKey,
        label: formatMessageDay(message.sentAt),
        groups: []
      }
      days.push(currentDay)
      currentGroup = null
    }

    const previousMessage = currentGroup?.messages?.[currentGroup.messages.length - 1]
    const withinGroupWindow = previousMessage
      ? new Date(message.sentAt).getTime() - new Date(previousMessage.sentAt).getTime() <= MESSAGE_GROUP_WINDOW_MS
      : false
    const sameSender = previousMessage ? previousMessage.senderId === message.senderId : false

    if (currentGroup && currentGroup.mine === mine && sameSender && withinGroupWindow) {
      currentGroup.messages.push(message)
      currentGroup.timeLabel = formatMessageTime(message.sentAt)
      currentGroup.deliveryState = mine ? deliveryState(message) : ""
      continue
    }

    currentGroup = {
      id: message.id,
      mine,
      messages: [message],
      timeLabel: formatMessageTime(message.sentAt),
      deliveryState: mine ? deliveryState(message) : ""
    }

    currentDay.groups.push(currentGroup)
  }

  return days
})

function deliveryState(message) {
  if (message?.readAt) {
    return "read"
  }

  if (message?.deliveredAt) {
    return "delivered"
  }

  return "sent"
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

function patchGroupChatSummary(groupId, message) {
  const normalizedGroupId = String(groupId || "").trim()
  if (!normalizedGroupId || !message) {
    return
  }

  const existingGroup = groupChats.value.find((group) => sameConversation(group.id, normalizedGroupId))
  const nextGroup = {
    ...(existingGroup || activeGroup.value || { id: normalizedGroupId, title: "Group chat" }),
    lastMessage: message,
    updatedAt: message.sentAt
  }

  if (activeGroup.value?.id === normalizedGroupId) {
    activeGroup.value = {
      ...activeGroup.value,
      lastMessage: message,
      updatedAt: message.sentAt
    }
  }

  groupChats.value = sortGroupChats(
    existingGroup
      ? groupChats.value.map((group) => (sameConversation(group.id, normalizedGroupId) ? nextGroup : group))
      : [nextGroup, ...groupChats.value]
  )
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
        ? {
            ...message,
            deliveredAt: message.deliveredAt || readAt,
            readAt
          }
        : message
    )
  }

  conversations.value = conversations.value.map((conversation) => {
    if (!sameConversation(conversation.user.id, result.conversationUserId)) {
      return conversation
    }

    const lastMessage = readAt && messageIds.has(conversation.lastMessage?.id)
      ? {
          ...conversation.lastMessage,
          deliveredAt: conversation.lastMessage?.deliveredAt || readAt,
          readAt
        }
      : conversation.lastMessage

    return {
      ...conversation,
      unreadCount: 0,
      lastMessage
    }
  })
}

function applyConversationDeliveredEvent(result) {
  if (!result?.conversationUserId) {
    return
  }

  const deliveredAt = result.deliveredAt || null
  const messageIds = new Set(result.messageIds || [])
  if (!deliveredAt || !messageIds.size) {
    return
  }

  if (sameConversation(activeConversationUserId.value, result.conversationUserId)) {
    messages.value = messages.value.map((message) =>
      messageIds.has(message.id)
        ? {
            ...message,
            deliveredAt: message.deliveredAt || deliveredAt
          }
        : message
    )
  }

  conversations.value = conversations.value.map((conversation) => {
    if (!sameConversation(conversation.user.id, result.conversationUserId)) {
      return conversation
    }

    const lastMessage = messageIds.has(conversation.lastMessage?.id)
      ? {
          ...conversation.lastMessage,
          deliveredAt: conversation.lastMessage?.deliveredAt || deliveredAt
        }
      : conversation.lastMessage

    return {
      ...conversation,
      lastMessage
    }
  })
}

function mergePrependedMessages(olderMessages) {
  const existingIds = new Set(messages.value.map((message) => message.id))
  const dedupedOlder = olderMessages.filter((message) => !existingIds.has(message.id))
  messages.value = [...dedupedOlder, ...messages.value]
}

function replaceMessageHistory(nextMessages) {
  const deduped = []
  const seen = new Set()
  for (const message of nextMessages) {
    if (seen.has(message.id)) {
      continue
    }

    seen.add(message.id)
    deduped.push(message)
  }

  messages.value = deduped
}

function acknowledgeLoadedMessages(loadedMessages, conversationUserId) {
  if (!conversationUserId || !loadedMessages.length) {
    return
  }

  let shouldMarkRead = false
  for (const message of loadedMessages) {
    if (message.senderId !== conversationUserId) {
      continue
    }

    if (!message.deliveredAt) {
      realtimeClient.ackChatDelivered(message.id)
    }

    if (!message.readAt) {
      shouldMarkRead = true
    }
  }

  if (shouldMarkRead) {
    realtimeClient.ackChatRead(conversationUserId)
  }
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
    void scrollMessagesToBottom()
  }

  if (message.senderId === conversationUserId) {
    realtimeClient.ackChatDelivered(message.id)

    if (sameConversation(activeConversationUserId.value, conversationUserId)) {
      realtimeClient.ackChatRead(conversationUserId)
    }
  }
}

function appendRealtimeGroupMessage(payload) {
  const message = normalizeGroupMessage(payload?.message)
  const groupId = String(payload?.groupId || message?.groupId || "").trim()
  if (!message?.id || !groupId) {
    return
  }

  patchGroupChatSummary(groupId, message)

  if (!sameConversation(activeGroupId.value, groupId)) {
    return
  }

  if (!hasMessage(message.id)) {
    messages.value = [...messages.value, message]
    void scrollMessagesToBottom()
  }
}

function markActiveConversationRead(userId) {
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

  realtimeClient.ackChatRead(userId)
}

async function loadSidebarData() {
  if (!isAuthenticated.value) {
    conversations.value = []
    groupChats.value = []
    discoverUsers.value = []
    pageError.value = ""
    return
  }

  isLoadingSidebar.value = true
  pageError.value = ""

  try {
    const [nextConversations, nextDiscoverUsers, nextGroups] = await Promise.all([
      fetchChatConversations(),
      fetchDiscoverUsers(),
      fetchGroups()
    ])

    conversations.value = sortConversations(nextConversations)
    groupChats.value = sortGroupChats(nextGroups.filter((group) => group.isMember))
    discoverUsers.value = nextDiscoverUsers

    const userIdFromRoute = routeConversationUserId()
    const groupIdFromRoute = routeGroupId()
    if (!userIdFromRoute && !groupIdFromRoute && nextConversations[0]?.user?.id) {
      await router.replace({
        path: "/chat",
        query: { user: nextConversations[0].user.id }
      })
    } else if (!userIdFromRoute && !groupIdFromRoute && nextGroups.find((group) => group.isMember)?.id) {
      await router.replace({
        path: "/chat",
        query: { group: nextGroups.find((group) => group.isMember).id }
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

  if (!normalizedUserId) {
    conversationUser.value = null
    groupMembers.value = []
    isLoadingGroupMembers.value = false
    messages.value = []
    threadError.value = ""
    historyHasMore.value = false
    isLoadingHistory.value = false
    activeHistoryRequestId.value = ""
    return
  }

  isLoadingThread.value = true
  isLoadingHistory.value = false
  threadError.value = ""
  historyHasMore.value = false
  activeHistoryRequestId.value = ""
  messages.value = []
  activeGroup.value = null
  groupMembers.value = []
  isLoadingGroupMembers.value = false
  conversationUser.value = knownConversationUser(normalizedUserId)

  requestConversationHistory(normalizedUserId, {
    beforeMessageId: "",
    prepend: false
  })
}

async function loadGroupThread(groupId) {
  const normalizedGroupId = String(groupId || "").trim()

  if (!normalizedGroupId) {
    activeGroup.value = null
    groupMembers.value = []
    isLoadingGroupMembers.value = false
    messages.value = []
    threadError.value = ""
    historyHasMore.value = false
    isLoadingHistory.value = false
    activeHistoryRequestId.value = ""
    return
  }

  isLoadingThread.value = true
  isLoadingHistory.value = false
  threadError.value = ""
  historyHasMore.value = false
  activeHistoryRequestId.value = ""
  messages.value = []
  groupMembers.value = []
  conversationUser.value = null
  activeGroup.value = knownGroup(normalizedGroupId)
  isLoadingGroupMembers.value = true

  try {
    const [group, members, groupMessages] = await Promise.all([
      activeGroup.value ? Promise.resolve(activeGroup.value) : fetchGroup(normalizedGroupId),
      fetchGroupMembers(normalizedGroupId),
      fetchGroupMessages(normalizedGroupId)
    ])

    if (!sameConversation(activeGroupId.value, normalizedGroupId)) {
      return
    }

    activeGroup.value = group
    groupMembers.value = members
    replaceMessageHistory(groupMessages)
    if (groupMessages.length) {
      patchGroupChatSummary(normalizedGroupId, groupMessages[groupMessages.length - 1])
    }
    await scrollMessagesToBottom()
  } catch (error) {
    if (isApiError(error, 403)) {
      threadError.value = "Only group members can open this chat."
    } else {
      threadError.value = error instanceof Error ? error.message : "Could not load this group chat."
    }
  } finally {
    if (sameConversation(activeGroupId.value, normalizedGroupId)) {
      isLoadingThread.value = false
      isLoadingGroupMembers.value = false
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

async function navigateToGroupChat(groupId) {
  const normalizedGroupId = String(groupId || "").trim()
  if (!normalizedGroupId) {
    return
  }

  await router.replace({
    path: "/chat",
    query: { group: normalizedGroupId }
  })
}

async function scrollMessagesToBottom() {
  await nextTick()

  const container = messageList.value
  if (!container) {
    return
  }

  container.scrollTop = container.scrollHeight
}

async function restorePrependedScrollPosition() {
  await nextTick()

  const container = messageList.value
  if (!container) {
    return
  }

  container.scrollTop =
    container.scrollHeight - pendingHistoryAdjustments.previousHeight + pendingHistoryAdjustments.previousTop
}

async function ensureScrollableHistory(conversationUserId) {
  await nextTick()

  const container = messageList.value
  if (!container || !historyHasMore.value || isLoadingHistory.value) {
    return
  }

  if (container.scrollHeight <= container.clientHeight && messages.value.length > 0) {
    requestConversationHistory(conversationUserId, {
      beforeMessageId: messages.value[0]?.id || "",
      prepend: true
    })
  }
}

function requestConversationHistory(conversationUserId, options = {}) {
  const normalizedConversationUserId = String(conversationUserId || "").trim()
  if (!normalizedConversationUserId || isLoadingHistory.value) {
    return
  }

  const requestID = options.requestID
    ? `chat-history:${options.requestID}:${Date.now()}`
    : `chat-history:${Date.now()}:${Math.random().toString(36).slice(2, 8)}`

  const container = messageList.value
  activeHistoryRequestId.value = requestID
  isLoadingHistory.value = true
  if (!options.prepend) {
    isLoadingThread.value = true
  }
  pendingHistoryAdjustments.prepend = Boolean(options.prepend)
  pendingHistoryAdjustments.previousHeight = container?.scrollHeight || 0
  pendingHistoryAdjustments.previousTop = container?.scrollTop || 0

  realtimeClient.requestChatHistory({
    conversationUserId: normalizedConversationUserId,
    beforeMessageId: options.beforeMessageId || "",
    limit: HISTORY_PAGE_SIZE,
    requestId: requestID
  })
}

async function handleHistoryLoaded(payload) {
  if (!payload?.requestId || payload.requestId !== activeHistoryRequestId.value) {
    return
  }

  const history = payload.history
  const conversationUserId = history?.conversationUserId || ""
  isLoadingHistory.value = false
  isLoadingThread.value = false
  activeHistoryRequestId.value = ""

  if (!sameConversation(activeConversationUserId.value, conversationUserId)) {
    return
  }

  historyHasMore.value = Boolean(history?.hasMore)
  conversationUser.value = history?.user || knownConversationUser(conversationUserId)

  const loadedMessages = history?.messages || []
  if (pendingHistoryAdjustments.prepend) {
    mergePrependedMessages(loadedMessages)
    await restorePrependedScrollPosition()
  } else {
    replaceMessageHistory(loadedMessages)
    await scrollMessagesToBottom()
  }

  const latestMessage = messages.value[messages.value.length - 1]
  if (conversationUser.value && latestMessage) {
    patchConversationSummary(conversationUser.value, latestMessage, selectedConversation()?.unreadCount || 0)
  }

  acknowledgeLoadedMessages(loadedMessages, conversationUserId)
  await ensureScrollableHistory(conversationUserId)
}

function handleHistoryError(payload) {
  if (!payload?.requestId || payload.requestId !== activeHistoryRequestId.value) {
    return
  }

  isLoadingHistory.value = false
  isLoadingThread.value = false
  activeHistoryRequestId.value = ""
  threadError.value = payload.message || "Could not load this conversation."
}

function handleMessageListScroll() {
  if (isGroupThread.value) {
    return
  }

  const container = messageList.value
  if (!container || isLoadingHistory.value || !historyHasMore.value || !activeConversationUserId.value) {
    return
  }

  if (container.scrollTop <= 64) {
    requestConversationHistory(activeConversationUserId.value, {
      beforeMessageId: messages.value[0]?.id || "",
      prepend: true
    })
  }
}

function appendEmojiToMessage(emoji) {
  if (!emoji) {
    return
  }

  composer.body = `${composer.body || ""}${emoji}`
}

async function submitMessage() {
  if (!activeConversationUserId.value && !activeGroupId.value) {
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
    if (activeGroupId.value) {
      const message = await sendGroupMessage(activeGroupId.value, { body })
      if (!hasMessage(message.id)) {
        messages.value = [...messages.value, message]
        void scrollMessagesToBottom()
      }

      patchGroupChatSummary(activeGroupId.value, message)
      composer.body = ""
      return
    }

    const message = await sendPrivateMessage(activeConversationUserId.value, { body })
    if (!hasMessage(message.id)) {
      messages.value = [...messages.value, message]
      void scrollMessagesToBottom()
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
      groupChats.value = []
      discoverUsers.value = []
      conversationUser.value = null
      activeGroup.value = null
      groupMembers.value = []
      isLoadingGroupMembers.value = false
      messages.value = []
      composer.body = ""
      return
    }

    void loadSidebarData()

    const userIdFromRoute = routeConversationUserId()
    const groupIdFromRoute = routeGroupId()
    if (userIdFromRoute) {
      void loadConversation(userIdFromRoute)
    } else if (groupIdFromRoute) {
      void loadGroupThread(groupIdFromRoute)
    }
  },
  { immediate: true }
)

watch(
  () => [route.query.user, route.query.group],
  ([queryUser, queryGroup]) => {
    if (!isAuthenticated.value) {
      return
    }

    const groupId = routeGroupId(queryGroup)
    if (groupId) {
      realtimeClient.leaveChatView()
      void loadGroupThread(groupId)
      return
    }

    activeGroup.value = null
    groupMembers.value = []
    isLoadingGroupMembers.value = false
    void loadConversation(routeConversationUserId(queryUser))
  },
  { immediate: true }
)

watch(
  () => activeConversationUserId.value,
  (conversationUserId, previousConversationUserId) => {
    if (previousConversationUserId && previousConversationUserId !== conversationUserId) {
      realtimeClient.leaveChatView()
    }

    if (conversationUserId && !activeGroupId.value) {
      realtimeClient.enterChatView(conversationUserId)
      void clearConversationNotifications(conversationUserId)
    } else {
      realtimeClient.leaveChatView()
    }

    void scrollMessagesToBottom()
  },
  { immediate: true }
)

removeRealtimeListeners.push(
  realtimeClient.on("chat.history.loaded", (event) => {
    void handleHistoryLoaded(event?.payload)
  }),
  realtimeClient.on("chat.history.error", (event) => {
    handleHistoryError(event?.payload)
  }),
  realtimeClient.on("chat.message.created", (event) => {
    appendRealtimeMessage(event?.payload?.conversationUserId, event?.payload?.message)
  }),
  realtimeClient.on("chat.message.delivered", (event) => {
    applyConversationDeliveredEvent(event?.payload)
  }),
  realtimeClient.on("chat.conversation.read", (event) => {
    applyConversationReadResult(event?.payload)
  }),
  realtimeClient.on("group.message.created", (event) => {
    appendRealtimeGroupMessage(event?.payload)
  })
)

onBeforeUnmount(() => {
  realtimeClient.leaveChatView()
  removeRealtimeListeners.splice(0).forEach((dispose) => dispose())
})
</script>

<template>
  <section class="page">
    <div class="panel">
      <p class="eyebrow">Chat</p>
      <h2>Messages</h2>
      <p>
        Private conversations and member-only group chats stay together in one realtime inbox.
      </p>
    </div>

    <template v-if="isAuthenticated">
      <div class="chat-layout">
        <aside class="panel chat-sidebar">
          <div class="chat-sidebar__section">
            <div class="feed-header">
              <div>
                <p class="eyebrow">Inbox</p>
                <h3>Private</h3>
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
                <p class="eyebrow">Groups</p>
                <h3>Group chats</h3>
              </div>
              <span class="badge badge--soft">{{ groupChats.length }}</span>
            </div>

            <div class="chat-thread-list">
              <button
                v-for="group in groupChats"
                :key="group.id"
                type="button"
                class="chat-thread"
                :class="{ 'chat-thread--active': activeGroupId === group.id }"
                @click="navigateToGroupChat(group.id)"
              >
                <span class="user-avatar user-avatar--small">
                  <span class="user-avatar__fallback">{{ groupInitials(group) }}</span>
                </span>

                <span class="chat-thread__content">
                  <span class="chat-thread__head">
                    <strong>{{ group.title }}</strong>
                    <small>{{ formatConversationTime(groupChatUpdatedAt(group)) }}</small>
                  </span>
                  <span class="chat-thread__body">
                    <small>{{ groupChatPreview(group) }}</small>
                  </span>
                </span>
              </button>

              <p v-if="!groupChats.length" class="feed-note">
                Group chats appear here once you are a member.
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
          <template v-if="hasSelectedThread">
            <header class="chat-panel__header">
              <div v-if="isGroupThread" class="chat-panel__identity">
                <span class="user-avatar user-avatar--small">
                  <span class="user-avatar__fallback">{{ groupInitials(activeGroup) }}</span>
                </span>

                <div>
                  <h3>{{ activeGroup?.title || "Group chat" }}</h3>
                  <div class="chat-members" tabindex="0">
                    <p class="feed-note chat-members__trigger">
                      {{ activeGroup ? groupMemberCountLabel(activeGroup) : "Loading group chat..." }}
                    </p>

                    <div v-if="activeGroup" class="chat-members__popover" role="tooltip">
                      <div class="chat-members__popover-head">
                        <strong>Group members</strong>
                        <span>{{ isLoadingGroupMembers ? "Loading..." : groupMemberCountLabel(activeGroup) }}</span>
                      </div>

                      <div class="chat-members__list">
                        <div
                          v-for="member in groupMembers"
                          :key="member.id"
                          class="chat-members__item"
                        >
                          <span class="user-avatar user-avatar--small">
                            <img
                              v-if="member.avatarUrl"
                              :src="member.avatarUrl"
                              :alt="`${displayName(member)} avatar`"
                              class="user-avatar__image"
                            />
                            <span v-else class="user-avatar__fallback">{{ userInitials(member) }}</span>
                          </span>

                          <span class="chat-members__details">
                            <strong>{{ displayName(member) }}</strong>
                            <small>{{ groupMemberRoleLabel(member) }}</small>
                          </span>
                        </div>

                        <p v-if="!isLoadingGroupMembers && !groupMembers.length" class="feed-note">
                          No members loaded.
                        </p>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <div v-else class="chat-panel__identity">
                <span class="user-avatar user-avatar--small">
                  <img
                    v-if="conversationUser?.avatarUrl"
                    :src="conversationUser.avatarUrl"
                    :alt="`${displayName(conversationUser)} avatar`"
                    class="user-avatar__image"
                  />
                  <span v-else class="user-avatar__fallback">{{ userInitials(conversationUser) }}</span>
                </span>

                <div>
                  <h3>{{ displayName(conversationUser) }}</h3>
                  <p class="feed-note">
                    {{ conversationUser?.profileVisibility ? `${conversationUser.profileVisibility} profile` : "Loading conversation..." }}
                  </p>
                </div>
              </div>
              <span class="badge badge--neutral">{{ messages.length }} messages</span>
            </header>

            <p v-if="threadError" class="form-error">{{ threadError }}</p>

            <div ref="messageList" class="chat-message-list" @scroll="handleMessageListScroll">
              <p v-if="isLoadingHistory && messages.length" class="chat-history-loading">
                Loading older messages...
              </p>
              <p v-if="isLoadingThread" class="feed-note">
                {{ isGroupThread ? "Loading group chat..." : "Loading conversation..." }}
              </p>
              <p v-else-if="!messages.length" class="feed-note">
                {{ isGroupThread ? "No group messages yet. Start the member thread." : "No messages yet. Say hello and start the thread." }}
              </p>

              <section
                v-for="day in groupedMessageDays"
                :key="day.key"
                class="chat-day"
              >
                <div class="chat-day__label">
                  <span>{{ day.label }}</span>
                </div>

                <article
                  v-for="group in day.groups"
                  :key="group.id"
                  class="chat-cluster"
                  :class="{
                    'chat-cluster--mine': group.mine,
                    'chat-cluster--theirs': !group.mine
                  }"
                >
                  <div class="chat-cluster__row">
                    <div class="chat-cluster__messages">
                      <div
                        v-for="message in group.messages"
                        :key="message.id"
                        class="chat-bubble"
                        :class="{
                          'chat-bubble--mine': group.mine,
                          'chat-bubble--theirs': !group.mine
                        }"
                      >
                        <small v-if="isGroupThread && !group.mine" class="chat-bubble__sender">
                          {{ displayName(group.messages[0]?.sender) }}
                        </small>
                        <div v-if="!isGroupThread && messageGroupInvite(message)" class="chat-invite-card">
                          <p class="eyebrow">Group invite</p>
                          <strong>{{ messageGroupInvite(message).title }}</strong>
                          <p>
                            {{ messageGroupInvite(message).note || "Open the group to review it and decide whether to join." }}
                          </p>
                          <button
                            type="button"
                            class="button button--small"
                            @click="openInvitedGroup(messageGroupInvite(message).groupId)"
                          >
                            Open group
                          </button>
                        </div>
                        <p v-else>{{ message.body }}</p>
                      </div>
                    </div>

                    <footer class="chat-cluster__meta">
                      <span>{{ group.timeLabel }}</span>
                      <span
                        v-if="group.mine && !isGroupThread"
                        class="chat-checks"
                        :class="`chat-checks--${group.deliveryState}`"
                        :title="group.deliveryState"
                      >
                        <span class="chat-check">✓</span>
                        <span v-if="group.deliveryState !== 'sent'" class="chat-check">✓</span>
                      </span>
                    </footer>
                  </div>
                </article>
              </section>
            </div>

            <form class="chat-composer" @submit.prevent="submitMessage">
              <label>
                <span class="visually-hidden">Write a message</span>
                <textarea
                  v-model="composer.body"
                  rows="3"
                  maxlength="2000"
                  :placeholder="isGroupThread ? 'Write to everyone in this group...' : 'Write a private message...'"
                ></textarea>
              </label>

              <div class="composer-toolbar">
                <EmojiPicker label="Add emoji to message" @select="appendEmojiToMessage" />
              </div>

              <div class="chat-composer__actions">
                <p class="feed-note">
                  {{ isGroupThread ? "Group messages are visible only to members." : "Messages are delivered live through your NEXO realtime channel." }}
                </p>
                <button type="submit" class="button" :disabled="isSendingMessage">
                  {{ isSendingMessage ? "Sending..." : "Send message" }}
                </button>
              </div>
            </form>
          </template>

          <template v-else>
            <div class="chat-empty-state">
              <p class="eyebrow">Messages</p>
              <h3>Choose a thread</h3>
              <p>
                Open a private conversation or one of your member group chats.
              </p>
            </div>
          </template>
        </section>
      </div>
    </template>

    <div v-else class="panel panel--narrow">
      <p class="eyebrow">Chat</p>
      <h3>Login required</h3>
      <p>Sign in to unlock private conversations and member-only group chats.</p>
    </div>
  </section>
</template>
