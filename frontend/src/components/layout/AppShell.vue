<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue"
import { RouterLink, useRoute, useRouter } from "vue-router"

import { fetchCurrentUser, fetchHealth, fetchMeta, fetchNotifications, isApiError, logoutUser } from "../../services/api"
import { realtimeClient } from "../../services/realtime"
import { useAppStore } from "../../stores/app"
import AppIcon from "../ui/AppIcon.vue"

const navItems = [
  {
    label: "Feed",
    description: "Open the main timeline and discover accounts.",
    to: "/feed",
    icon: "feed"
  },
  {
    label: "Create",
    description: "Publish a new carousel post.",
    to: "/create",
    icon: "create"
  },
  {
    label: "Profile",
    description: "Open your profile, privacy settings, and follow requests.",
    to: "/profile/me",
    icon: "profile"
  },
  {
    label: "My posts",
    description: "Browse only the posts you've published and reopen your own threads.",
    to: "/my-posts",
    icon: "myPosts"
  },
  {
    label: "Groups",
    description: "Explore the area reserved for communities and events.",
    to: "/groups",
    icon: "groups"
  },
  {
    label: "Notifications",
    description: "Review follow requests, approvals, comments, and replies.",
    to: "/notifications",
    icon: "notifications"
  },
  {
    label: "Chat",
    description: "Open your private conversations with live delivery and read receipts.",
    to: "/chat",
    icon: "chat"
  }
]

const store = useAppStore()
const router = useRouter()
const route = useRoute()
const requestError = ref("")
const authError = ref("")
const isLoggingOut = ref(false)
const isAuthenticated = computed(() => Boolean(store.state.currentUser))
const unreadNotifications = computed(() => store.state.notificationUnreadCount)
const removeRealtimeListeners = []
const currentUserName = computed(() => {
  const user = store.state.currentUser
  if (!user) {
    return ""
  }

  return user.nickname || `${user.firstName} ${user.lastName}`.trim() || user.email
})
const currentUserInitials = computed(() => {
  const user = store.state.currentUser
  if (!user) {
    return "N"
  }

  const source = user.nickname || `${user.firstName} ${user.lastName}`.trim() || user.email || "NEXO"
  return source
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() || "")
    .join("") || "N"
})

function isActive(item) {
  return route.path === item.to || route.path.startsWith(`${item.to}/`)
}

async function refreshNotifications() {
  if (!store.state.currentUser) {
    store.setNotificationUnreadCount(0)
    return
  }

  try {
    const notifications = await fetchNotifications()
    store.setNotificationUnreadCount(notifications.filter((item) => !item.isRead).length)
  } catch (error) {
    if (isApiError(error, 401)) {
      store.setNotificationUnreadCount(0)
      return
    }

    throw error
  }
}

async function bootstrap() {
  try {
    const [health, meta, currentUser] = await Promise.all([
      fetchHealth(),
      fetchMeta(),
      fetchCurrentUser().catch((error) => {
        if (isApiError(error, 401)) {
          return null
        }

        throw error
      })
    ])

    store.setApiStatus(health.status)
    store.setMeta(meta)
    store.setCurrentUser(currentUser)
    requestError.value = ""
    await refreshNotifications()
  } catch (error) {
    store.setApiStatus("down")
    requestError.value = error instanceof Error ? error.message : "Backend unavailable"
  }
}

async function handleLogout() {
  isLoggingOut.value = true
  authError.value = ""

  try {
    await logoutUser()
    store.clearCurrentUser()
    await router.push("/feed")
  } catch (error) {
    authError.value = error instanceof Error ? error.message : "Could not log out"
  } finally {
    isLoggingOut.value = false
  }
}

onMounted(() => {
  removeRealtimeListeners.push(
    realtimeClient.on("notification.created", () => {
      void refreshNotifications()
    }),
    realtimeClient.on("notification.read", () => {
      void refreshNotifications()
    }),
    realtimeClient.on("ws.unauthorized", () => {
      store.clearCurrentUser()
    })
  )

  void bootstrap()
})

onBeforeUnmount(() => {
  removeRealtimeListeners.splice(0).forEach((dispose) => dispose())
  realtimeClient.disconnect()
})

watch(
  () => store.state.currentUser?.id,
  (userID) => {
    if (userID) {
      realtimeClient.connect(userID)
    } else {
      realtimeClient.disconnect()
    }

    void refreshNotifications()
  }
)
</script>

<template>
  <div class="shell" :class="{ 'shell--guest': !isAuthenticated }">
    <aside v-if="isAuthenticated" class="shell__sidebar">
      <RouterLink to="/feed" class="shell__brand" title="Go to the feed">
        <img src="/nexo-mark.png" alt="NEXO mark" class="shell__brand-logo" />
      </RouterLink>

      <nav class="shell__icon-rail" aria-label="Primary navigation">
        <RouterLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="shell__icon-link"
          :class="{ 'shell__icon-link--active': isActive(item) }"
        >
          <AppIcon :name="item.icon" :size="26" />
          <span class="shell__icon-details">
            <strong>{{ item.label }}</strong>
            <small>{{ item.description }}</small>
          </span>
          <span
            v-if="item.to === '/notifications' && unreadNotifications"
            class="shell__icon-badge"
          >
            {{ unreadNotifications }}
          </span>
        </RouterLink>
      </nav>

      <div class="shell__sidebar-footer">
        <div class="shell__status-pill" :title="`API status: ${store.state.apiStatus}`">
          <span class="status-dot" :class="`status-dot--${store.state.apiStatus}`"></span>
        </div>
        <p v-if="requestError" class="shell__error">
          {{ requestError }}
        </p>
      </div>
    </aside>

    <div class="shell__content">
      <header class="shell__header">
        <div>
          <img
            src="/nexo-wordmark.png"
            :alt="`${store.state.meta?.name || 'NEXO'} wordmark`"
            class="shell__header-logo"
          />
        </div>

        <div class="shell__header-actions">
          <template v-if="isAuthenticated">
            <RouterLink to="/profile/me" class="shell__user-chip">
              <span class="user-avatar user-avatar--small">
                <img
                  v-if="store.state.currentUser.avatarUrl"
                  :src="store.state.currentUser.avatarUrl"
                  :alt="`${currentUserName} avatar`"
                  class="user-avatar__image"
                />
                <span v-else class="user-avatar__fallback">{{ currentUserInitials }}</span>
              </span>
              <strong class="shell__user-name">{{ currentUserName }}</strong>
            </RouterLink>
            <button
              type="button"
              class="button button--ghost"
              :disabled="isLoggingOut"
              @click="handleLogout"
            >
              {{ isLoggingOut ? "Signing out..." : "Logout" }}
            </button>
          </template>
          <template v-else>
            <RouterLink to="/login" class="button button--ghost">
              Login
            </RouterLink>
            <RouterLink to="/register" class="button">
              Register
            </RouterLink>
          </template>
        </div>
      </header>

      <p v-if="authError" class="shell__error">
        {{ authError }}
      </p>

      <main class="shell__main">
        <slot />
      </main>
    </div>
  </div>
</template>
