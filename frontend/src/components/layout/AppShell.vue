<script setup>
import { computed, onMounted, ref, watch } from "vue"
import { RouterLink, useRoute, useRouter } from "vue-router"

import { fetchCurrentUser, fetchHealth, fetchMeta, fetchNotifications, isApiError, logoutUser } from "../../services/api"
import { useAppStore } from "../../stores/app"
import AppIcon from "../ui/AppIcon.vue"

const navItems = [
  {
    label: "Home",
    description: "Overview and product direction.",
    to: "/",
    icon: "overview"
  },
  {
    label: "Feed",
    description: "Browse posts, threads, and discover accounts.",
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
    description: "Open the space reserved for private and group chat.",
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
const currentUserName = computed(() => {
  const user = store.state.currentUser
  if (!user) {
    return ""
  }

  return user.nickname || `${user.firstName} ${user.lastName}`.trim() || user.email
})

function isActive(item) {
  if (item.to === "/") {
    return route.path === "/"
  }

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
    await router.push("/")
  } catch (error) {
    authError.value = error instanceof Error ? error.message : "Could not log out"
  } finally {
    isLoggingOut.value = false
  }
}

onMounted(() => {
  void bootstrap()
})

watch(
  () => store.state.currentUser?.id,
  () => {
    void refreshNotifications()
  }
)
</script>

<template>
  <div class="shell">
    <aside class="shell__sidebar">
      <RouterLink to="/" class="shell__brand" title="Go to the home page">
        <img src="/nexo-logo.png" alt="NEXO logo" class="shell__brand-logo" />
        <span class="shell__brand-tagline">Share your world. your way.</span>
      </RouterLink>

      <nav class="shell__icon-rail" aria-label="Primary navigation">
        <RouterLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="shell__icon-link"
          :class="{ 'shell__icon-link--active': isActive(item) }"
          :title="`${item.label}: ${item.description}`"
        >
          <AppIcon :name="item.icon" :size="23" />
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
          <p class="eyebrow">Architecture</p>
          <img
            src="/nexo-logo.png"
            :alt="store.state.meta?.name || 'NEXO'"
            class="shell__header-logo"
          />
        </div>

        <div class="shell__header-actions">
          <template v-if="isAuthenticated">
            <RouterLink to="/profile/me" class="shell__user-chip">
              <strong>{{ currentUserName }}</strong>
              <span>{{ store.state.currentUser.email }}</span>
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
