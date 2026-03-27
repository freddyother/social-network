<script setup>
import { computed, onBeforeUnmount, reactive, ref, watch } from "vue"
import { useRouter } from "vue-router"

import { fetchNotifications, isApiError, markNotificationRead } from "../services/api"
import { realtimeClient } from "../services/realtime"
import { useAppStore } from "../stores/app"
import { formatDate as formatAppDate } from "../utils/date"

const store = useAppStore()
const router = useRouter()
const notifications = ref([])
const requestError = ref("")
const isLoading = ref(false)
const readLoading = reactive({})
const isAuthenticated = computed(() => Boolean(store.state.currentUser))
const unreadCount = computed(() => notifications.value.filter((item) => !item.isRead).length)
const removeRealtimeListeners = []

function formatDate(value) {
  return formatAppDate(value)
}

function typeLabel(type) {
  return type.replaceAll("_", " ")
}

async function loadNotificationsInbox() {
  if (!isAuthenticated.value) {
    notifications.value = []
    store.setNotificationUnreadCount(0)
    return
  }

  isLoading.value = true
  requestError.value = ""

  try {
    const items = await fetchNotifications()
    notifications.value = items
    store.setNotificationUnreadCount(items.filter((item) => !item.isRead).length)
  } catch (error) {
    requestError.value = error instanceof Error ? error.message : "Could not load notifications."
  } finally {
    isLoading.value = false
  }
}

async function handleMarkRead(notificationId) {
  return markNotificationAsRead(notificationId)
}

async function markNotificationAsRead(notificationId) {
  if (!notificationId) {
    return true
  }

  readLoading[notificationId] = true
  requestError.value = ""

  try {
    await markNotificationRead(notificationId)
    notifications.value = notifications.value.map((item) =>
      item.id === notificationId ? { ...item, isRead: true } : item
    )
    store.setNotificationUnreadCount(unreadCount.value)
    return true
  } catch (error) {
    if (isApiError(error)) {
      requestError.value = error.message
    } else {
      requestError.value = "Could not update this notification."
    }
    return false
  } finally {
    readLoading[notificationId] = false
  }
}

function notificationTarget(notification) {
  if (!notification) {
    return null
  }

  if (notification.entityType === "conversation" && notification.entityId) {
    return {
      path: "/chat",
      query: { user: notification.entityId }
    }
  }

  if (notification.entityType === "post" && notification.entityId) {
    return {
      path: "/feed",
      query: { post: notification.entityId, comments: "1" }
    }
  }

  if (notification.type === "follow_request_received") {
    return {
      path: "/profile/me",
      query: { section: "follow-requests" }
    }
  }

  if (notification.type === "follow_request_accepted") {
    return {
      path: "/feed"
    }
  }

  return null
}

function notificationActionLabel(notification) {
  if (notification?.entityType === "conversation") {
    return "Open chat"
  }

  if (notification?.entityType === "post") {
    return "Open post"
  }

  if (notification?.type === "follow_request_received") {
    return "Open requests"
  }

  if (notification?.type === "follow_request_accepted") {
    return "Open feed"
  }

  return ""
}

async function openNotification(notification) {
  const target = notificationTarget(notification)
  if (!target) {
    return
  }

  if (!notification.isRead) {
    await markNotificationAsRead(notification.id)
  }

  await router.push(target)
}

watch(
  () => store.state.currentUser?.id,
  () => {
    void loadNotificationsInbox()
  },
  { immediate: true }
)

removeRealtimeListeners.push(
  realtimeClient.on("notification.created", (event) => {
    const notification = event.payload?.notification
    if (!notification) {
      return
    }

    if (notifications.value.some((item) => item.id === notification.id)) {
      return
    }

    notifications.value = [notification, ...notifications.value]
    store.setNotificationUnreadCount(unreadCount.value)
  }),
  realtimeClient.on("notification.read", (event) => {
    const notificationId = event.payload?.notificationId
    if (!notificationId) {
      return
    }

    notifications.value = notifications.value.map((item) =>
      item.id === notificationId ? { ...item, isRead: true } : item
    )
    store.setNotificationUnreadCount(unreadCount.value)
  })
)

onBeforeUnmount(() => {
  removeRealtimeListeners.splice(0).forEach((dispose) => dispose())
})
</script>

<template>
  <section class="page">
    <div class="panel">
      <p class="eyebrow">Notifications</p>
      <h2>Global inbox</h2>
      <p>
        Follow requests, approvals, post comments, and replies now land here as persistent events separate from private chat.
      </p>
    </div>

    <p v-if="requestError" class="form-error">{{ requestError }}</p>

    <div v-if="!isAuthenticated" class="panel">
      <h3>Sign in to see your inbox</h3>
      <p>The notification center is only available once you are authenticated.</p>
    </div>

    <template v-else>
      <div class="grid grid--two">
        <article class="panel">
          <h3>Unread activity</h3>
          <p>{{ unreadCount }} pending notifications</p>
        </article>
        <article class="panel">
          <h3>What appears here</h3>
          <p>Private messages, follow requests, approvals, fresh comments on your posts, and direct replies to your comments.</p>
        </article>
      </div>

      <section class="panel">
        <div class="feed-header">
          <h3>Recent events</h3>
          <p>{{ isLoading ? "Refreshing inbox..." : `${notifications.length} notifications` }}</p>
        </div>

        <div v-if="isLoading" class="notification-list">
          <p class="feed-note">Loading notifications...</p>
        </div>

        <div v-else-if="notifications.length" class="notification-list">
          <article
            v-for="notification in notifications"
            :key="notification.id"
            class="notification-card"
            :class="{ 'notification-card--unread': !notification.isRead }"
          >
            <div class="notification-card__content">
              <div class="notification-card__meta">
                <span class="badge">{{ typeLabel(notification.type) }}</span>
                <span class="feed-note">{{ formatDate(notification.createdAt) }}</span>
                <span v-if="!notification.isRead" class="badge badge--neutral">Unread</span>
              </div>
              <h3>{{ notification.title }}</h3>
              <p>{{ notification.body }}</p>
              <button
                v-if="notificationTarget(notification)"
                type="button"
                class="notification-card__link notification-card__link-button"
                @click="openNotification(notification)"
              >
                {{ notificationActionLabel(notification) }}
              </button>
            </div>

            <button
              v-if="!notification.isRead"
              type="button"
              class="button button--ghost button--small"
              :disabled="readLoading[notification.id]"
              @click="handleMarkRead(notification.id)"
            >
              {{ readLoading[notification.id] ? "Saving..." : "Mark as read" }}
            </button>
          </article>
        </div>

        <div v-else class="panel panel--inset">
          <h3>No notifications yet</h3>
          <p>Use a second account to request follows, comment on posts, and generate activity in this inbox.</p>
        </div>
      </section>
    </template>
  </section>
</template>
