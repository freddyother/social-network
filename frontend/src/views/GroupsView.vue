<script setup>
import { computed, reactive, ref, watch } from "vue"
import { useRouter } from "vue-router"

import {
  createGroup,
  createGroupPost,
  fetchGroup,
  fetchGroupPosts,
  fetchGroups,
  isApiError,
  joinGroup
} from "../services/api"
import { useAppStore } from "../stores/app"
import { formatRelativeTime } from "../utils/date"

const props = defineProps({
  groupId: {
    type: String,
    default: ""
  }
})

const store = useAppStore()
const router = useRouter()

const isAuthenticated = computed(() => Boolean(store.state.currentUser))
const currentUserId = computed(() => store.state.currentUser?.id || "")
const groups = ref([])
const selectedGroup = ref(null)
const groupPosts = ref([])
const isLoading = ref(false)
const isLoadingDetail = ref(false)
const isLoadingGroupPosts = ref(false)
const isCreatingGroup = ref(false)
const isCreatingGroupPost = ref(false)
const createError = ref("")
const createGroupPostError = ref("")
const requestError = ref("")
const groupPostsError = ref("")
const joinLoading = reactive({})
const createForm = reactive({
  title: "",
  description: ""
})
const groupPostForm = reactive({
  body: ""
})

let groupPostsRequestToken = 0

const activeGroupId = computed(() => String(props.groupId || "").trim())
const joinedGroups = computed(() => groups.value.filter((group) => group.isMember))
const discoverGroups = computed(() => groups.value.filter((group) => !group.isMember))

function displayName(user) {
  if (!user) {
    return ""
  }

  return user.nickname || `${user.firstName} ${user.lastName}`.trim() || "NEXO member"
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

function groupSummary(group) {
  if (!group) {
    return ""
  }

  return `${group.membersCount || 0} members · ${group.postsCount || 0} posts · ${group.eventsCount || 0} events`
}

function groupPostTimestampLabel(post) {
  if (!post) {
    return ""
  }

  return post.updatedAt !== post.createdAt
    ? `Edited ${formatRelativeTime(post.updatedAt)}`
    : formatRelativeTime(post.createdAt)
}

function commentCountLabel(count) {
  return Number(count || 0) === 1 ? "1 reply" : `${Number(count || 0)} replies`
}

function sortGroups(items) {
  return [...items].sort((left, right) => {
    if (left.isMember !== right.isMember) {
      return Number(right.isMember) - Number(left.isMember)
    }

    return Date.parse(right.createdAt || 0) - Date.parse(left.createdAt || 0)
  })
}

function upsertGroup(nextGroup) {
  if (!nextGroup?.id) {
    return
  }

  const nextGroups = [...groups.value]
  const existingIndex = nextGroups.findIndex((group) => group.id === nextGroup.id)
  if (existingIndex >= 0) {
    nextGroups[existingIndex] = nextGroup
  } else {
    nextGroups.unshift(nextGroup)
  }

  groups.value = sortGroups(nextGroups)
}

function bumpGroupPostsCount(groupId, delta) {
  const normalizedGroupId = String(groupId || "").trim()
  if (!normalizedGroupId || !delta) {
    return
  }

  groups.value = groups.value.map((group) =>
    group.id === normalizedGroupId
      ? {
          ...group,
          postsCount: Math.max(0, Number(group.postsCount || 0) + delta)
        }
      : group
  )

  if (selectedGroup.value?.id === normalizedGroupId) {
    selectedGroup.value = {
      ...selectedGroup.value,
      postsCount: Math.max(0, Number(selectedGroup.value.postsCount || 0) + delta)
    }
  }
}

function clearGroupPostsState({ clearComposer = true } = {}) {
  groupPostsRequestToken += 1
  groupPosts.value = []
  groupPostsError.value = ""
  createGroupPostError.value = ""
  isLoadingGroupPosts.value = false

  if (clearComposer) {
    groupPostForm.body = ""
  }
}

async function openGroup(groupId = "") {
  const normalizedGroupId = String(groupId || "").trim()
  await router.push(normalizedGroupId ? { name: "groups", params: { groupId: normalizedGroupId } } : { name: "groups" })
}

async function loadGroupsList() {
  if (!isAuthenticated.value) {
    groups.value = []
    selectedGroup.value = null
    requestError.value = ""
    createError.value = ""
    clearGroupPostsState()
    return
  }

  isLoading.value = true
  requestError.value = ""

  try {
    const loadedGroups = await fetchGroups()
    groups.value = sortGroups(loadedGroups)

    if (!activeGroupId.value) {
      selectedGroup.value = groups.value[0] || null
    } else {
      await loadSelectedGroup(activeGroupId.value)
    }
  } catch (error) {
    requestError.value = error instanceof Error ? error.message : "Could not load groups right now."
  } finally {
    isLoading.value = false
  }
}

async function loadSelectedGroup(groupId) {
  if (!isAuthenticated.value) {
    return
  }

  const normalizedGroupId = String(groupId || "").trim()
  if (!normalizedGroupId) {
    selectedGroup.value = groups.value[0] || null
    return
  }

  const existingGroup = groups.value.find((group) => group.id === normalizedGroupId)
  if (existingGroup) {
    selectedGroup.value = existingGroup
  }

  isLoadingDetail.value = true
  requestError.value = ""

  try {
    const loadedGroup = await fetchGroup(normalizedGroupId)
    upsertGroup(loadedGroup)
    selectedGroup.value = loadedGroup
  } catch (error) {
    if (isApiError(error, 404)) {
      requestError.value = "This group was not found."
    } else {
      requestError.value = error instanceof Error ? error.message : "Could not open this group."
    }

    if (selectedGroup.value?.id === normalizedGroupId || !selectedGroup.value) {
      selectedGroup.value = groups.value[0] || null
    }
  } finally {
    isLoadingDetail.value = false
  }
}

async function loadGroupPosts(groupId) {
  const normalizedGroupId = String(groupId || "").trim()
  if (!normalizedGroupId) {
    clearGroupPostsState()
    return
  }

  const requestToken = ++groupPostsRequestToken
  isLoadingGroupPosts.value = true
  groupPostsError.value = ""
  createGroupPostError.value = ""
  groupPosts.value = []

  try {
    const loadedPosts = await fetchGroupPosts(normalizedGroupId)
    if (requestToken !== groupPostsRequestToken) {
      return
    }

    groupPosts.value = loadedPosts
  } catch (error) {
    if (requestToken !== groupPostsRequestToken) {
      return
    }

    groupPostsError.value = error instanceof Error ? error.message : "Could not load the group posts."
  } finally {
    if (requestToken === groupPostsRequestToken) {
      isLoadingGroupPosts.value = false
    }
  }
}

async function handleCreateGroup() {
  isCreatingGroup.value = true
  createError.value = ""
  requestError.value = ""

  try {
    const createdGroup = await createGroup({
      title: createForm.title,
      description: createForm.description
    })

    createForm.title = ""
    createForm.description = ""
    upsertGroup(createdGroup)
    selectedGroup.value = createdGroup
    await router.push({ name: "groups", params: { groupId: createdGroup.id } })
  } catch (error) {
    if (isApiError(error)) {
      createError.value =
        error.payload?.fields?.title ||
        error.payload?.fields?.description ||
        error.message
    } else {
      createError.value = "Could not create the group right now."
    }
  } finally {
    isCreatingGroup.value = false
  }
}

async function handleJoinGroup(groupId = selectedGroup.value?.id || "") {
  const normalizedGroupId = String(groupId || "").trim()
  if (!normalizedGroupId) {
    return
  }

  joinLoading[normalizedGroupId] = true
  requestError.value = ""

  try {
    const joinedGroup = await joinGroup(normalizedGroupId)
    upsertGroup(joinedGroup)
    if (!selectedGroup.value || selectedGroup.value.id === normalizedGroupId) {
      selectedGroup.value = joinedGroup
    }
  } catch (error) {
    requestError.value = error instanceof Error ? error.message : "Could not join this group."
  } finally {
    joinLoading[normalizedGroupId] = false
  }
}

async function handleCreateGroupPost() {
  const normalizedGroupId = String(selectedGroup.value?.id || "").trim()
  if (!normalizedGroupId) {
    return
  }

  isCreatingGroupPost.value = true
  createGroupPostError.value = ""
  requestError.value = ""

  try {
    const createdPost = await createGroupPost(normalizedGroupId, {
      body: groupPostForm.body
    })

    groupPosts.value = [createdPost, ...groupPosts.value.filter((post) => post.id !== createdPost.id)]
    groupPostForm.body = ""
    bumpGroupPostsCount(normalizedGroupId, 1)
  } catch (error) {
    if (isApiError(error)) {
      createGroupPostError.value =
        error.payload?.fields?.body ||
        error.message
    } else {
      createGroupPostError.value = "Could not publish in this group right now."
    }
  } finally {
    isCreatingGroupPost.value = false
  }
}

watch(
  () => store.state.currentUser?.id,
  () => {
    void loadGroupsList()
  },
  { immediate: true }
)

watch(
  () => activeGroupId.value,
  (groupId) => {
    if (!isAuthenticated.value) {
      return
    }

    if (!groupId && !groups.value.length) {
      return
    }

    void loadSelectedGroup(groupId)
  },
  { immediate: true }
)

watch(
  () => [selectedGroup.value?.id || "", selectedGroup.value?.isMember ? "1" : "0"],
  ([groupId, isMember], previous = []) => {
    if (!isAuthenticated.value) {
      clearGroupPostsState()
      return
    }

    const previousGroupId = previous[0] || ""
    if (groupId !== previousGroupId) {
      clearGroupPostsState()
    }

    if (!groupId || isMember !== "1") {
      if (!groupId || isMember !== "1") {
        groupPosts.value = []
        groupPostsError.value = ""
        isLoadingGroupPosts.value = false
      }
      return
    }

    void loadGroupPosts(groupId)
  },
  { immediate: true }
)
</script>

<template>
  <section class="page">
    <div class="panel">
      <p class="eyebrow">Groups</p>
      <h2>Community spaces now have an internal timeline</h2>
      <p>
        Phase 2 adds member-only group posts on top of creation, discovery, memberships, and global search. Events and group chat are the next layers.
      </p>
    </div>

    <div v-if="!isAuthenticated" class="panel panel--narrow">
      <h3>Login required</h3>
      <p>Sign in to create groups, join communities, and participate in each group's internal discussion.</p>
    </div>

    <template v-else>
      <p v-if="requestError" class="form-error">{{ requestError }}</p>

      <div class="grid grid--two groups-layout">
        <section class="panel">
          <div class="feed-header">
            <div>
              <p class="eyebrow">Create</p>
              <h3>Start a new group</h3>
            </div>
            <p>{{ groups.length }} total groups</p>
          </div>

          <form class="stack-form" @submit.prevent="handleCreateGroup">
            <label>
              <span>Group title</span>
              <input
                v-model.trim="createForm.title"
                type="text"
                maxlength="120"
                placeholder="Product builders in London"
              />
            </label>

            <label>
              <span>Description</span>
              <textarea
                v-model.trim="createForm.description"
                rows="5"
                maxlength="2000"
                placeholder="Say what this community is for, who should join, and what people can expect here."
              ></textarea>
            </label>

            <div class="profile-form__actions">
              <p class="feed-note">Groups stay open to join in this MVP so we can ship the community core quickly.</p>
              <button type="submit" class="button" :disabled="isCreatingGroup">
                {{ isCreatingGroup ? "Creating..." : "Create group" }}
              </button>
            </div>

            <p v-if="createError" class="form-error">{{ createError }}</p>
          </form>
        </section>

        <section class="panel">
          <template v-if="selectedGroup">
            <p class="eyebrow">Selected Group</p>

            <div class="groups-detail__hero">
              <span class="user-avatar user-avatar--small">
                <img
                  v-if="selectedGroup.creator?.avatarUrl"
                  :src="selectedGroup.creator.avatarUrl"
                  :alt="`${displayName(selectedGroup.creator)} avatar`"
                  class="user-avatar__image"
                />
                <span v-else class="user-avatar__fallback">{{ userInitials(selectedGroup.creator) }}</span>
              </span>

              <div class="groups-detail__copy">
                <h3>{{ selectedGroup.title }}</h3>
                <p>{{ selectedGroup.description }}</p>
                <p class="feed-note">
                  Created by {{ displayName(selectedGroup.creator) }} · {{ formatRelativeTime(selectedGroup.createdAt) }}
                </p>
              </div>
            </div>

            <div class="group-card__stats">
              <span class="badge badge--neutral">{{ selectedGroup.membersCount }} members</span>
              <span class="badge badge--neutral">{{ selectedGroup.postsCount }} posts</span>
              <span class="badge badge--neutral">{{ selectedGroup.eventsCount }} events</span>
              <span v-if="selectedGroup.isMember" class="badge">{{ selectedGroup.role || "member" }}</span>
            </div>

            <div class="profile-form__actions">
              <p class="feed-note">
                {{ selectedGroup.isMember ? "You can read and publish in this group's internal timeline now." : "Join this group to unlock its internal discussion timeline." }}
              </p>
              <button
                v-if="!selectedGroup.isMember"
                type="button"
                class="button"
                :disabled="joinLoading[selectedGroup.id]"
                @click="handleJoinGroup(selectedGroup.id)"
              >
                {{ joinLoading[selectedGroup.id] ? "Joining..." : "Join group" }}
              </button>
            </div>

            <p v-if="isLoadingDetail" class="feed-note">Refreshing group details...</p>
          </template>

          <div v-else class="profile-empty-state">
            <h3>No group selected yet</h3>
            <p>{{ isLoading ? "Loading the latest groups..." : "Create the first group or open one from the lists below." }}</p>
          </div>
        </section>
      </div>

      <section class="panel">
        <div class="feed-header">
          <div>
            <p class="eyebrow">Discussion</p>
            <h3>{{ selectedGroup ? `Inside ${selectedGroup.title}` : "Group timeline" }}</h3>
          </div>
          <p v-if="selectedGroup">{{ selectedGroup.postsCount || 0 }} posts</p>
        </div>

        <div v-if="!selectedGroup" class="profile-empty-state">
          <h3>Pick a group first</h3>
          <p>Open one of your groups or a discovery result to load its internal timeline.</p>
        </div>

        <template v-else-if="selectedGroup.isMember">
          <form class="stack-form group-post-composer" @submit.prevent="handleCreateGroupPost">
            <label>
              <span>Start a conversation</span>
              <textarea
                v-model.trim="groupPostForm.body"
                rows="4"
                maxlength="4000"
                placeholder="Share an update, ask for feedback, or kick off the next thread for this group."
              ></textarea>
            </label>

            <div class="profile-form__actions">
              <p class="feed-note">Only members can read and publish inside this phase-2 timeline.</p>
              <button type="submit" class="button" :disabled="isCreatingGroupPost">
                {{ isCreatingGroupPost ? "Publishing..." : "Publish in group" }}
              </button>
            </div>

            <p v-if="createGroupPostError" class="form-error">{{ createGroupPostError }}</p>
          </form>

          <p v-if="isLoadingGroupPosts" class="feed-note">Loading group posts...</p>
          <p v-else-if="groupPostsError" class="form-error">{{ groupPostsError }}</p>

          <div v-else-if="groupPosts.length" class="group-post-stack">
            <article v-for="post in groupPosts" :key="post.id" class="post-card group-post-card">
              <div class="post-card__header">
                <div class="post-card__header-main">
                  <span class="user-avatar user-avatar--small">
                    <img
                      v-if="post.author?.avatarUrl"
                      :src="post.author.avatarUrl"
                      :alt="`${displayName(post.author)} avatar`"
                      class="user-avatar__image"
                    />
                    <span v-else class="user-avatar__fallback">{{ userInitials(post.author) }}</span>
                  </span>

                  <div>
                    <strong>{{ displayName(post.author) }}</strong>
                    <div class="post-card__meta">
                      <span>{{ commentCountLabel(post.commentsCount) }}</span>
                      <span>{{ groupPostTimestampLabel(post) }}</span>
                    </div>
                  </div>
                </div>

                <span v-if="post.author?.id === currentUserId" class="badge">you</span>
              </div>

              <div class="post-card__body">
                <p>{{ post.body }}</p>
                <img v-if="post.imageUrl" :src="post.imageUrl" alt="" class="group-post-card__image" />
              </div>
            </article>
          </div>

          <div v-else class="profile-empty-state group-posts-empty">
            <h3>No posts yet</h3>
            <p>Be the first person to start the conversation inside this group.</p>
          </div>
        </template>

        <div v-else class="profile-empty-state group-posts-locked">
          <h3>Join to see the conversation</h3>
          <p>This group's internal timeline is only visible to members in this phase.</p>
          <button
            type="button"
            class="button"
            :disabled="joinLoading[selectedGroup.id]"
            @click="handleJoinGroup(selectedGroup.id)"
          >
            {{ joinLoading[selectedGroup.id] ? "Joining..." : "Join group" }}
          </button>
        </div>
      </section>

      <div class="grid grid--two">
        <section class="panel">
          <div class="feed-header">
            <div>
              <p class="eyebrow">Memberships</p>
              <h3>Your groups</h3>
            </div>
            <p>{{ joinedGroups.length }} joined</p>
          </div>

          <p v-if="isLoading" class="feed-note">Loading groups...</p>

          <div v-else-if="joinedGroups.length" class="user-stack">
            <article
              v-for="group in joinedGroups"
              :key="group.id"
              class="user-card group-card"
              :class="{ 'group-card--active': selectedGroup?.id === group.id }"
            >
              <div class="group-card__body">
                <strong>{{ group.title }}</strong>
                <p class="group-card__summary">{{ group.description }}</p>
                <div class="group-card__stats">
                  <span class="badge badge--neutral">{{ group.membersCount }} members</span>
                  <span class="badge badge--neutral">{{ group.postsCount }} posts</span>
                  <span class="badge">{{ group.role || "member" }}</span>
                </div>
              </div>

              <div class="user-card__actions">
                <button type="button" class="button button--ghost button--small" @click="openGroup(group.id)">
                  Open group
                </button>
              </div>
            </article>
          </div>

          <p v-else class="feed-note">You have not joined any groups yet. Create one or pick one from discovery.</p>
        </section>

        <section class="panel">
          <div class="feed-header">
            <div>
              <p class="eyebrow">Discover</p>
              <h3>Explore groups</h3>
            </div>
            <p>{{ discoverGroups.length }} available</p>
          </div>

          <p v-if="isLoading" class="feed-note">Loading groups...</p>

          <div v-else-if="discoverGroups.length" class="user-stack">
            <article
              v-for="group in discoverGroups"
              :key="group.id"
              class="user-card group-card"
              :class="{ 'group-card--active': selectedGroup?.id === group.id }"
            >
              <div class="group-card__body">
                <strong>{{ group.title }}</strong>
                <p class="group-card__summary">{{ group.description }}</p>
                <p class="feed-note">{{ groupSummary(group) }}</p>
              </div>

              <div class="user-card__actions">
                <button type="button" class="button button--ghost button--small" @click="openGroup(group.id)">
                  Open
                </button>
                <button
                  type="button"
                  class="button button--small"
                  :disabled="joinLoading[group.id]"
                  @click="handleJoinGroup(group.id)"
                >
                  {{ joinLoading[group.id] ? "Joining..." : "Join" }}
                </button>
              </div>
            </article>
          </div>

          <p v-else class="feed-note">Every existing group is already in your memberships. Create the next one to expand the map.</p>
        </section>
      </div>
    </template>
  </section>
</template>
