<script setup>
import { onMounted, ref } from "vue"
import { RouterLink } from "vue-router"

import { fetchHealth, fetchMeta } from "../../services/api"
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
const requestError = ref("")

async function bootstrap() {
  try {
    const [health, meta] = await Promise.all([fetchHealth(), fetchMeta()])

    store.setApiStatus(health.status)
    store.setMeta(meta)
    requestError.value = ""
  } catch (error) {
    store.setApiStatus("down")
    requestError.value = error instanceof Error ? error.message : "Backend unavailable"
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
          <RouterLink to="/login" class="button button--ghost">
            Login
          </RouterLink>
          <RouterLink to="/register" class="button">
            Register
          </RouterLink>
        </div>
      </header>

      <main class="shell__main">
        <slot />
      </main>
    </div>
  </div>
</template>
