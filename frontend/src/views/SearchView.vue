<script setup>
import { computed, reactive, ref, watch } from "vue"
import { RouterLink, useRoute, useRouter } from "vue-router"

import UserAvatar from "../components/ui/UserAvatar.vue"
import { isApiError, searchAll } from "../services/api"
import { useAppStore } from "../stores/app"
import { formatRelativeTime } from "../utils/date"

const store = useAppStore()
const route = useRoute()
const router = useRouter()

const isAuthenticated = computed(() => Boolean(store.state.currentUser))
const currentUserId = computed(() => store.state.currentUser?.id || "")
const isLoading = ref(false)
const requestError = ref("")
const searchForm = reactive({
  query: ""
})
const results = ref({
  query: "",
  users: [],
  posts: [],
  groups: []
})

const hasQuery = computed(() => Boolean(routeQuery()))
const totalResults = computed(() => (
  results.value.users.length +
  results.value.posts.length +
  results.value.groups.length
))

function routeQuery() {
  const rawValue = route.query.q
  if (Array.isArray(rawValue)) {
    return String(rawValue[0] || "").trim()
  }

  return typeof rawValue === "string" ? rawValue.trim() : ""
}

function displayName(user) {
  if (!user) {
    return ""
  }

  return user.nickname || `${user.firstName} ${user.lastName}`.trim() || "NEXO member"
}

function relationshipLabel(status) {
  switch (status) {
    case "self":
      return "You"
    case "following":
      return "Following"
    case "requested":
      return "Requested"
    default:
      return "Not following"
  }
}

function profileTarget(user) {
  if (!user) {
    return ""
  }

  if (user.id === currentUserId.value) {
    return "/profile/me"
  }

  const nickname = String(user.nickname || "").trim()
  return nickname ? `/profile/${encodeURIComponent(nickname)}` : ""
}

function groupTarget(group) {
  return `/groups/${group.id}`
}

function postTarget(post) {
  return {
    path: "/feed",
    query: { post: post.id }
  }
}

function postPreview(post) {
  const text = String(post?.body || post?.title || "").trim()
  if (!text) {
    return "Open this result to inspect the full post and its comment thread."
  }

  return text.length > 180 ? `${text.slice(0, 177)}...` : text
}

function groupSummary(group) {
  if (!group) {
    return ""
  }

  return `${group.membersCount || 0} members · ${group.postsCount || 0} posts · ${group.eventsCount || 0} events`
}

async function submitSearch() {
  const query = searchForm.query.trim()
  await router.push(query ? { path: "/search", query: { q: query } } : { path: "/search" })
}

async function loadSearch() {
  const query = routeQuery()
  searchForm.query = query
  requestError.value = ""

  if (!isAuthenticated.value || !query) {
    results.value = {
      query,
      users: [],
      posts: [],
      groups: []
    }
    return
  }

  isLoading.value = true
  results.value = {
    query,
    users: [],
    posts: [],
    groups: []
  }

  try {
    results.value = await searchAll(query)
  } catch (error) {
    results.value = {
      query,
      users: [],
      posts: [],
      groups: []
    }

    if (isApiError(error)) {
      requestError.value = error.payload?.fields?.q || error.message
    } else {
      requestError.value = error instanceof Error ? error.message : "Could not load search results."
    }
  } finally {
    isLoading.value = false
  }
}

watch(
  () => [store.state.currentUser?.id, route.query.q],
  () => {
    void loadSearch()
  },
  { immediate: true }
)
</script>

<template>
  <section class="page">
    <div class="panel">
      <p class="eyebrow">Search</p>
      <h2>Global search across NEXO</h2>
      <p>Scan users, visible posts, and groups from one query without leaving the app shell.</p>

      <form v-if="isAuthenticated" class="search-form" @submit.prevent="submitSearch">
        <label>
          <span>Search query</span>
          <input
            v-model.trim="searchForm.query"
            type="search"
            maxlength="80"
            placeholder="Search users, posts, and groups"
          />
        </label>
        <button type="submit" class="button">
          Search now
        </button>
      </form>
    </div>

    <div v-if="!isAuthenticated" class="panel panel--narrow">
      <h3>Login required</h3>
      <p>Sign in to search across profiles, visible posts, and groups in one place.</p>
    </div>

    <template v-else>
      <p v-if="requestError" class="form-error">{{ requestError }}</p>

      <div v-if="!hasQuery" class="panel panel--inset search-empty">
        <h3>Start with a name, keyword, or community topic</h3>
        <p>Search works across user profiles, the posts you are allowed to see, and the new groups directory.</p>
      </div>

      <template v-else>
        <div class="grid grid--three">
          <article class="panel">
            <p class="eyebrow">Users</p>
            <h3>{{ results.users.length }}</h3>
            <p>matching profiles</p>
          </article>
          <article class="panel">
            <p class="eyebrow">Posts</p>
            <h3>{{ results.posts.length }}</h3>
            <p>visible post matches</p>
          </article>
          <article class="panel">
            <p class="eyebrow">Groups</p>
            <h3>{{ results.groups.length }}</h3>
            <p>community matches</p>
          </article>
        </div>

        <div v-if="isLoading" class="panel panel--inset search-empty">
          <h3>Searching...</h3>
          <p>Refreshing every section for “{{ results.query }}”.</p>
        </div>

        <div v-else-if="!totalResults" class="panel panel--inset search-empty">
          <h3>No results yet</h3>
          <p>Nothing matched “{{ results.query }}”. Try a broader keyword or a shorter phrase.</p>
        </div>

        <template v-else>
          <div class="grid grid--two">
            <section class="panel">
              <div class="feed-header">
                <div>
                  <p class="eyebrow">Users</p>
                  <h3>People</h3>
                </div>
                <p>{{ results.users.length }} matches</p>
              </div>

              <div v-if="results.users.length" class="user-stack">
                <article v-for="user in results.users" :key="user.id" class="user-card">
                  <span class="chat-user-card__identity">
                    <UserAvatar :user="user" :name="displayName(user)" :alt="`${displayName(user)} avatar`" />
                    <span>
                      <strong>{{ displayName(user) }}</strong>
                      <small>{{ user.aboutMe || "Profile ready to explore." }}</small>
                    </span>
                  </span>

                  <div class="user-card__actions">
                    <span class="badge badge--neutral">{{ relationshipLabel(user.relationshipStatus) }}</span>
                    <RouterLink v-if="profileTarget(user)" :to="profileTarget(user)" class="button button--ghost button--small">
                      Open profile
                    </RouterLink>
                  </div>
                </article>
              </div>

              <p v-else class="feed-note">No users matched this query.</p>
            </section>

            <section class="panel">
              <div class="feed-header">
                <div>
                  <p class="eyebrow">Groups</p>
                  <h3>Communities</h3>
                </div>
                <p>{{ results.groups.length }} matches</p>
              </div>

              <div v-if="results.groups.length" class="user-stack">
                <article v-for="group in results.groups" :key="group.id" class="user-card group-card">
                  <div class="group-card__body">
                    <strong>{{ group.title }}</strong>
                    <p class="group-card__summary">{{ group.description }}</p>
                    <div class="group-card__stats">
                      <span class="badge badge--neutral">{{ group.membersCount }} members</span>
                      <span v-if="group.isMember" class="badge">{{ group.role || "member" }}</span>
                    </div>
                    <p class="feed-note">{{ groupSummary(group) }}</p>
                  </div>

                  <div class="user-card__actions">
                    <RouterLink :to="groupTarget(group)" class="button button--ghost button--small">
                      Open group
                    </RouterLink>
                  </div>
                </article>
              </div>

              <p v-else class="feed-note">No groups matched this query.</p>
            </section>
          </div>

          <section class="panel">
            <div class="feed-header">
              <div>
                <p class="eyebrow">Posts</p>
                <h3>Visible posts</h3>
              </div>
              <p>{{ results.posts.length }} matches</p>
            </div>

            <div v-if="results.posts.length" class="search-post-list">
              <article v-for="post in results.posts" :key="post.id" class="search-post-card">
                <div class="search-post-card__preview">
                  <img
                    v-if="post.media?.length"
                    :src="post.media[0].url"
                    :alt="post.title || `${displayName(post.author)} post`"
                    class="search-post-card__thumb"
                  />

                  <div class="search-post-card__copy">
                    <div class="search-post-card__header">
                      <div>
                        <strong>{{ post.title || "Untitled post" }}</strong>
                        <p class="search-post-card__summary">
                          by {{ displayName(post.author) }} · {{ formatRelativeTime(post.createdAt) }}
                        </p>
                      </div>
                      <div class="group-card__stats">
                        <span class="badge">{{ post.privacy }}</span>
                        <span class="badge badge--neutral">{{ post.commentsCount || 0 }} comments</span>
                      </div>
                    </div>

                    <p class="group-card__summary">{{ postPreview(post) }}</p>
                  </div>
                </div>

                <div class="search-post-card__actions">
                  <p class="feed-note">
                    {{ post.media?.length ? `${post.media.length} media item${post.media.length === 1 ? "" : "s"}` : "Text-first post" }}
                  </p>
                  <RouterLink :to="postTarget(post)" class="button button--ghost button--small">
                    Open post
                  </RouterLink>
                </div>
              </article>
            </div>

            <p v-else class="feed-note">No visible posts matched this query.</p>
          </section>
        </template>
      </template>
    </template>
  </section>
</template>
