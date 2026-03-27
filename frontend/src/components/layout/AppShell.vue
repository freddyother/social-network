<script setup>
import { computed, onMounted, ref } from "vue"
import { RouterLink, useRouter } from "vue-router"

import { fetchCurrentUser, fetchHealth, fetchMeta, isApiError, logoutUser } from "../../services/api"
import { useAppStore } from "../../stores/app"

const links = [
  { label: "Overview", to: "/" },
  { label: "Feed", to: "/feed" },
  { label: "Profile", to: "/profile/me" },
  { label: "Groups", to: "/groups" },
  { label: "Notifications", to: "/notifications" },
  { label: "Chat", to: "/chat" }
]

const store = useAppStore()
const router = useRouter()
const requestError = ref("")
const authError = ref("")
const isLoggingOut = ref(false)
const isAuthenticated = computed(() => Boolean(store.state.currentUser))
const currentUserName = computed(() => {
  const user = store.state.currentUser
  if (!user) {
    return ""
  }

  return user.nickname || `${user.firstName} ${user.lastName}`.trim() || user.email
})

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
</script>

<template>
  <div class="shell">
    <aside class="shell__sidebar">
      <p class="eyebrow">Vue 3 + Go</p>
      <h1>Social Network</h1>
      <p class="shell__lede">
        SPA separated from the backend so it can scale cleanly into feed, groups, private profiles, WebSockets, and notifications.
      </p>

      <nav class="shell__nav" aria-label="Primary navigation">
        <RouterLink
          v-for="link in links"
          :key="link.to"
          :to="link.to"
          class="shell__nav-link"
        >
          {{ link.label }}
        </RouterLink>
      </nav>

      <div class="shell__status-card">
        <span class="status-dot" :class="`status-dot--${store.state.apiStatus}`"></span>
        <div>
          <p class="status-label">API status</p>
          <strong>{{ store.state.apiStatus }}</strong>
        </div>
      </div>

      <p v-if="requestError" class="shell__error">
        {{ requestError }}
      </p>
    </aside>

    <div class="shell__content">
      <header class="shell__header">
        <div>
          <p class="eyebrow">Architecture</p>
          <h2>{{ store.state.meta?.name || "Social Network" }}</h2>
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
