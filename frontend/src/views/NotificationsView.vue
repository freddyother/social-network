<script setup>
import { computed, reactive, ref, watch } from "vue"
import { RouterLink } from "vue-router"

import { fetchNotifications, isApiError, markNotificationRead } from "../services/api"
import { useAppStore } from "../stores/app"

const store = useAppStore()
const notifications = ref([])
const requestError = ref("")
const isLoading = ref(false)
const readLoading = reactive({})
const isAuthenticated = computed(() => Boolean(store.state.currentUser))
const unreadCount = computed(() => notifications.value.filter((item) => !item.isRead).length)

function formatDate(value) {
  return new Intl.DateTimeFormat("en-GB", {
    dateStyle: "medium",
    timeStyle: "short"
  }).format(new Date(value))
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
  readLoading[notificationId] = true
  requestError.value = ""

  try {
    await markNotificationRead(notificationId)
    notifications.value = notifications.value.map((item) =>
      item.id === notificationId ? { ...item, isRead: true } : item
    )
    store.setNotificationUnreadCount(unreadCount.value)
  } catch (error) {
    if (isApiError(error)) {
      requestError.value = error.message
    } else {
      requestError.value = "Could not update this notification."
    }
  } finally {
    readLoading[notificationId] = false
  }
}

watch(
  () => store.state.currentUser?.id,
  () => {
    void loadNotificationsInbox()
  },
  { immediate: true }
)
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
          <p>Private follow requests, approvals, fresh comments on your posts, and direct replies to your comments.</p>
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
              <RouterLink
                v-if="notification.entityType === 'post'"
                to="/feed"
                class="notification-card__link"
              >
                Open feed
              </RouterLink>
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
