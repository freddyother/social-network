<script setup>
import { computed, reactive, ref, watch } from "vue"
import { useRouter } from "vue-router"

import { createGroup, fetchGroup, fetchGroups, isApiError, joinGroup } from "../services/api"
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
const groups = ref([])
const selectedGroup = ref(null)
const isLoading = ref(false)
const isLoadingDetail = ref(false)
const isCreatingGroup = ref(false)
const createError = ref("")
const requestError = ref("")
const joinLoading = reactive({})
const createForm = reactive({
  title: "",
  description: ""
})

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
</script>

<template>
  <section class="page">
    <div class="panel">
      <p class="eyebrow">Groups</p>
      <h2>Community spaces are live</h2>
      <p>
        Phase 1 unlocks group creation, discovery, memberships, and searchable group pages. Internal posts, shared chat, invitations, and events land next.
      </p>
    </div>

    <div v-if="!isAuthenticated" class="panel panel--narrow">
      <h3>Login required</h3>
      <p>Sign in to create groups, join communities, and explore the new social spaces.</p>
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
              <p class="feed-note">Groups are open to join in this first phase so we can wire up the community core quickly.</p>
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
                {{ selectedGroup.isMember ? "You already belong to this group and can track its next verticals from here." : "Join now to be ready for internal posts, events, and group chat as the next phases arrive." }}
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
