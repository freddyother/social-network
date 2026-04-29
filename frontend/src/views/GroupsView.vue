<script setup>
import { computed, reactive, ref, watch } from "vue"
import { useRouter } from "vue-router"

import {
  createGroup,
  createGroupComment,
  createGroupEvent,
  createGroupPost,
  fetchDiscoverUsers,
  fetchGroup,
  fetchGroupComments,
  fetchGroupEvents,
  fetchGroupPosts,
  fetchGroups,
  inviteUserToGroup,
  isApiError,
  joinGroup,
  respondToGroupEvent
} from "../services/api"
import { useAppStore } from "../stores/app"
import { formatDateTime, formatRelativeTime } from "../utils/date"

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
const groupEvents = ref([])
const inviteUsers = ref([])
const isLoading = ref(false)
const isLoadingDetail = ref(false)
const isLoadingGroupPosts = ref(false)
const isLoadingGroupEvents = ref(false)
const isLoadingInviteUsers = ref(false)
const isCreatingGroup = ref(false)
const isCreatingGroupPost = ref(false)
const isCreatingGroupEvent = ref(false)
const isSendingInvite = ref(false)
const createError = ref("")
const createGroupPostError = ref("")
const createGroupEventError = ref("")
const inviteError = ref("")
const inviteSuccess = ref("")
const inviteUsersError = ref("")
const requestError = ref("")
const groupPostsError = ref("")
const groupEventsError = ref("")
const joinLoading = reactive({})
const groupCommentsExpanded = reactive({})
const groupCommentsByPost = reactive({})
const groupCommentsLoading = reactive({})
const groupCommentsLoaded = reactive({})
const groupCommentErrorByPost = reactive({})
const groupCommentSubmitting = reactive({})
const groupCommentForms = reactive({})
const eventResponseLoading = reactive({})
const createForm = reactive({
  title: "",
  description: ""
})
const groupPostForm = reactive({
  body: ""
})
const groupEventForm = reactive({
  title: "",
  description: "",
  startsAtLocal: ""
})
const inviteForm = reactive({
  recipientId: "",
  note: ""
})

let groupPostsRequestToken = 0
let groupEventsRequestToken = 0

const activeGroupId = computed(() => String(props.groupId || "").trim())
const joinedGroups = computed(() => groups.value.filter((group) => group.isMember))
const discoverGroups = computed(() => groups.value.filter((group) => !group.isMember))
const suggestedInviteUsers = computed(() =>
  inviteUsers.value.filter((user) => user.id !== currentUserId.value)
)

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

function groupCommentTimestampLabel(comment) {
  if (!comment) {
    return ""
  }

  return formatRelativeTime(comment.createdAt)
}

function commentCountLabel(count) {
  return Number(count || 0) === 1 ? "1 comment" : `${Number(count || 0)} comments`
}

function sortGroups(items) {
  return [...items].sort((left, right) => {
    if (left.isMember !== right.isMember) {
      return Number(right.isMember) - Number(left.isMember)
    }

    return Date.parse(right.createdAt || 0) - Date.parse(left.createdAt || 0)
  })
}

function sortGroupEvents(items) {
  return [...items].sort((left, right) => {
    const startsAtDiff = Date.parse(left.startsAt || 0) - Date.parse(right.startsAt || 0)
    if (startsAtDiff !== 0) {
      return startsAtDiff
    }

    return Date.parse(right.createdAt || 0) - Date.parse(left.createdAt || 0)
  })
}

function clearReactiveMap(target) {
  Object.keys(target).forEach((key) => {
    delete target[key]
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

function upsertGroupEvent(nextEvent) {
  if (!nextEvent?.id) {
    return
  }

  const nextEvents = [...groupEvents.value]
  const existingIndex = nextEvents.findIndex((event) => event.id === nextEvent.id)
  if (existingIndex >= 0) {
    nextEvents[existingIndex] = nextEvent
  } else {
    nextEvents.unshift(nextEvent)
  }

  groupEvents.value = sortGroupEvents(nextEvents)
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

function bumpGroupEventsCount(groupId, delta) {
  const normalizedGroupId = String(groupId || "").trim()
  if (!normalizedGroupId || !delta) {
    return
  }

  groups.value = groups.value.map((group) =>
    group.id === normalizedGroupId
      ? {
          ...group,
          eventsCount: Math.max(0, Number(group.eventsCount || 0) + delta)
        }
      : group
  )

  if (selectedGroup.value?.id === normalizedGroupId) {
    selectedGroup.value = {
      ...selectedGroup.value,
      eventsCount: Math.max(0, Number(selectedGroup.value.eventsCount || 0) + delta)
    }
  }
}

function bumpGroupPostCommentsCount(postId, delta) {
  const normalizedPostId = String(postId || "").trim()
  if (!normalizedPostId || !delta) {
    return
  }

  groupPosts.value = groupPosts.value.map((post) =>
    post.id === normalizedPostId
      ? {
          ...post,
          commentsCount: Math.max(0, Number(post.commentsCount || 0) + delta)
        }
      : post
  )
}

function ensureGroupCommentState(postId) {
  if (!groupCommentForms[postId]) {
    groupCommentForms[postId] = { body: "" }
  }

  if (!groupCommentsByPost[postId]) {
    groupCommentsByPost[postId] = []
  }

  if (!(postId in groupCommentsLoading)) {
    groupCommentsLoading[postId] = false
  }

  if (!(postId in groupCommentsLoaded)) {
    groupCommentsLoaded[postId] = false
  }

  if (!(postId in groupCommentsExpanded)) {
    groupCommentsExpanded[postId] = false
  }

  if (!(postId in groupCommentErrorByPost)) {
    groupCommentErrorByPost[postId] = ""
  }

  if (!(postId in groupCommentSubmitting)) {
    groupCommentSubmitting[postId] = false
  }
}

function clearGroupPostsState({ clearComposer = true } = {}) {
  groupPostsRequestToken += 1
  groupPosts.value = []
  groupPostsError.value = ""
  createGroupPostError.value = ""
  isLoadingGroupPosts.value = false
  clearReactiveMap(groupCommentsExpanded)
  clearReactiveMap(groupCommentsByPost)
  clearReactiveMap(groupCommentsLoading)
  clearReactiveMap(groupCommentsLoaded)
  clearReactiveMap(groupCommentErrorByPost)
  clearReactiveMap(groupCommentSubmitting)
  clearReactiveMap(groupCommentForms)

  if (clearComposer) {
    groupPostForm.body = ""
  }
}

function clearGroupEventsState({ clearComposer = true } = {}) {
  groupEventsRequestToken += 1
  groupEvents.value = []
  groupEventsError.value = ""
  createGroupEventError.value = ""
  isLoadingGroupEvents.value = false
  clearReactiveMap(eventResponseLoading)

  if (clearComposer) {
    groupEventForm.title = ""
    groupEventForm.description = ""
    groupEventForm.startsAtLocal = ""
  }
}

async function openGroup(groupId = "") {
  const normalizedGroupId = String(groupId || "").trim()
  await router.push(normalizedGroupId ? { name: "groups", params: { groupId: normalizedGroupId } } : { name: "groups" })
}

async function loadInviteUsers() {
  if (!isAuthenticated.value) {
    inviteUsers.value = []
    inviteUsersError.value = ""
    return
  }

  isLoadingInviteUsers.value = true
  inviteUsersError.value = ""

  try {
    inviteUsers.value = await fetchDiscoverUsers()

    if (!inviteForm.recipientId && inviteUsers.value[0]?.id) {
      inviteForm.recipientId = inviteUsers.value[0].id
    }
  } catch (error) {
    inviteUsersError.value = error instanceof Error ? error.message : "Could not load people to invite."
  } finally {
    isLoadingInviteUsers.value = false
  }
}

async function loadGroupsList() {
  if (!isAuthenticated.value) {
    groups.value = []
    selectedGroup.value = null
    requestError.value = ""
    createError.value = ""
    clearGroupPostsState()
    clearGroupEventsState()
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
    for (const post of loadedPosts) {
      ensureGroupCommentState(post.id)
    }
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

async function loadGroupEvents(groupId) {
  const normalizedGroupId = String(groupId || "").trim()
  if (!normalizedGroupId) {
    clearGroupEventsState()
    return
  }

  const requestToken = ++groupEventsRequestToken
  isLoadingGroupEvents.value = true
  groupEventsError.value = ""
  createGroupEventError.value = ""
  groupEvents.value = []

  try {
    const loadedEvents = await fetchGroupEvents(normalizedGroupId)
    if (requestToken !== groupEventsRequestToken) {
      return
    }

    groupEvents.value = sortGroupEvents(loadedEvents)
  } catch (error) {
    if (requestToken !== groupEventsRequestToken) {
      return
    }

    groupEventsError.value = error instanceof Error ? error.message : "Could not load the group events."
  } finally {
    if (requestToken === groupEventsRequestToken) {
      isLoadingGroupEvents.value = false
    }
  }
}

async function loadGroupComments(post) {
  const postId = String(post?.id || "").trim()
  const groupId = String(selectedGroup.value?.id || "").trim()
  if (!postId || !groupId) {
    return
  }

  ensureGroupCommentState(postId)
  groupCommentsLoading[postId] = true
  groupCommentErrorByPost[postId] = ""

  try {
    groupCommentsByPost[postId] = await fetchGroupComments(groupId, postId)
    groupCommentsLoaded[postId] = true
  } catch (error) {
    groupCommentsLoaded[postId] = false
    groupCommentErrorByPost[postId] = error instanceof Error ? error.message : "Could not load group comments."
  } finally {
    groupCommentsLoading[postId] = false
  }
}

async function toggleGroupComments(post) {
  const postId = String(post?.id || "").trim()
  if (!postId) {
    return
  }

  ensureGroupCommentState(postId)
  groupCommentsExpanded[postId] = !groupCommentsExpanded[postId]

  if (groupCommentsExpanded[postId] && !groupCommentsLoaded[postId]) {
    await loadGroupComments(post)
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
    ensureGroupCommentState(createdPost.id)
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

async function handleCreateGroupComment(post) {
  const postId = String(post?.id || "").trim()
  const groupId = String(selectedGroup.value?.id || "").trim()
  if (!postId || !groupId) {
    return
  }

  ensureGroupCommentState(postId)
  groupCommentSubmitting[postId] = true
  groupCommentErrorByPost[postId] = ""

  try {
    const comment = await createGroupComment(groupId, postId, {
      body: groupCommentForms[postId].body
    })

    groupCommentsByPost[postId] = [...(groupCommentsByPost[postId] || []), comment]
    groupCommentsLoaded[postId] = true
    groupCommentsExpanded[postId] = true
    groupCommentForms[postId].body = ""
    bumpGroupPostCommentsCount(postId, 1)
  } catch (error) {
    if (isApiError(error)) {
      groupCommentErrorByPost[postId] =
        error.payload?.fields?.body ||
        error.message
    } else {
      groupCommentErrorByPost[postId] = "Could not publish the group comment right now."
    }
  } finally {
    groupCommentSubmitting[postId] = false
  }
}

async function handleCreateGroupEvent() {
  const normalizedGroupId = String(selectedGroup.value?.id || "").trim()
  if (!normalizedGroupId) {
    return
  }

  isCreatingGroupEvent.value = true
  createGroupEventError.value = ""
  groupEventsError.value = ""

  try {
    const eventPayload = {
      title: groupEventForm.title,
      description: groupEventForm.description
    }

    if (groupEventForm.startsAtLocal) {
      eventPayload.startsAt = new Date(groupEventForm.startsAtLocal).toISOString()
    }

    const createdEvent = await createGroupEvent(normalizedGroupId, eventPayload)

    upsertGroupEvent(createdEvent)
    groupEventForm.title = ""
    groupEventForm.description = ""
    groupEventForm.startsAtLocal = ""
    bumpGroupEventsCount(normalizedGroupId, 1)
  } catch (error) {
    if (isApiError(error)) {
      createGroupEventError.value =
        error.payload?.fields?.title ||
        error.payload?.fields?.description ||
        error.payload?.fields?.startsAt ||
        error.message
    } else {
      createGroupEventError.value = "Could not create the event right now."
    }
  } finally {
    isCreatingGroupEvent.value = false
  }
}

async function handleRespondToGroupEvent(event, response) {
  const normalizedGroupId = String(selectedGroup.value?.id || "").trim()
  const normalizedEventId = String(event?.id || "").trim()
  if (!normalizedGroupId || !normalizedEventId) {
    return
  }

  eventResponseLoading[normalizedEventId] = true
  groupEventsError.value = ""

  try {
    const updatedEvent = await respondToGroupEvent(normalizedGroupId, normalizedEventId, response)
    upsertGroupEvent(updatedEvent)
  } catch (error) {
    groupEventsError.value = error instanceof Error ? error.message : "Could not update the RSVP right now."
  } finally {
    eventResponseLoading[normalizedEventId] = false
  }
}

async function handleInviteUserToGroup() {
  const normalizedGroupId = String(selectedGroup.value?.id || "").trim()
  if (!normalizedGroupId) {
    return
  }

  isSendingInvite.value = true
  inviteError.value = ""
  inviteSuccess.value = ""

  try {
    await inviteUserToGroup(normalizedGroupId, {
      recipientId: inviteForm.recipientId,
      note: inviteForm.note
    })

    const recipientName = displayName(
      suggestedInviteUsers.value.find((user) => user.id === inviteForm.recipientId)
    ) || "that person"

    inviteForm.note = ""
    inviteSuccess.value = `Invitation sent to ${recipientName}.`
  } catch (error) {
    if (isApiError(error)) {
      inviteError.value =
        error.payload?.fields?.recipientId ||
        error.payload?.fields?.note ||
        error.message
    } else {
      inviteError.value = "Could not send the invitation right now."
    }
  } finally {
    isSendingInvite.value = false
  }
}

watch(
  () => store.state.currentUser?.id,
  () => {
    void Promise.allSettled([loadGroupsList(), loadInviteUsers()])
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
      clearGroupEventsState()
      return
    }

    const previousGroupId = previous[0] || ""
    if (groupId !== previousGroupId) {
      clearGroupPostsState()
      clearGroupEventsState()
      inviteError.value = ""
      inviteSuccess.value = ""
    }

    if (!groupId || isMember !== "1") {
      groupPosts.value = []
      groupEvents.value = []
      groupPostsError.value = ""
      groupEventsError.value = ""
      isLoadingGroupPosts.value = false
      isLoadingGroupEvents.value = false
      return
    }

    void loadGroupPosts(groupId)
    void loadGroupEvents(groupId)
  },
  { immediate: true }
)
</script>

<template>
  <section class="page">
    <div class="panel">
      <p class="eyebrow">Groups</p>
      <h2>Group spaces now support conversation, events, and invites</h2>
      <p>
        This phase adds flat comments on group posts, shared event planning with RSVPs, and invite-by-message flows that open straight into the group from chat.
      </p>
    </div>

    <div v-if="!isAuthenticated" class="panel panel--narrow">
      <h3>Login required</h3>
      <p>Sign in to create groups, join communities, comment on group posts, plan events, and invite people in.</p>
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
                {{ selectedGroup.isMember ? "You can post, comment, plan events, and invite people into this space now." : "Join this group to unlock its internal timeline, events, and invitation flow." }}
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
              <p class="feed-note">Only members can read and publish inside this internal timeline.</p>
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

              <div class="group-post-card__actions">
                <button
                  type="button"
                  class="button button--ghost button--small"
                  @click="toggleGroupComments(post)"
                >
                  {{ groupCommentsExpanded[post.id] ? "Hide comments" : "Show comments" }}
                </button>
                <p class="feed-note">{{ commentCountLabel(post.commentsCount) }}</p>
              </div>

              <div v-if="groupCommentsExpanded[post.id]" class="post-comments">
                <div class="post-comments__header">
                  <p>{{ commentCountLabel(post.commentsCount) }}</p>
                  <p class="feed-note">
                    {{ groupCommentsLoading[post.id] ? "Loading..." : "Discussion stays visible to members only." }}
                  </p>
                </div>

                <p v-if="groupCommentErrorByPost[post.id]" class="form-error">{{ groupCommentErrorByPost[post.id] }}</p>

                <div v-if="(groupCommentsByPost[post.id] || []).length" class="comment-thread">
                  <article v-for="comment in groupCommentsByPost[post.id]" :key="comment.id" class="comment-card">
                    <div class="comment-card__header">
                      <div class="comment-card__header-main">
                        <strong>{{ displayName(comment.author) }}</strong>
                      </div>
                      <span>{{ groupCommentTimestampLabel(comment) }}</span>
                    </div>
                    <p class="comment-card__body">{{ comment.body }}</p>
                  </article>
                </div>

                <p v-else-if="!groupCommentsLoading[post.id]" class="feed-note">No comments yet. Start the first response.</p>

                <form class="comment-composer" @submit.prevent="handleCreateGroupComment(post)">
                  <label>
                    <span>Comment</span>
                    <textarea
                      v-model.trim="groupCommentForms[post.id].body"
                      rows="3"
                      maxlength="1000"
                      placeholder="Add context, feedback, or the next step for this thread."
                    ></textarea>
                  </label>

                  <div class="comment-composer__actions">
                    <p class="feed-note">Comments here are flat in this MVP to keep group discussion lightweight.</p>
                    <button type="submit" class="button button--small" :disabled="groupCommentSubmitting[post.id]">
                      {{ groupCommentSubmitting[post.id] ? "Posting..." : "Post comment" }}
                    </button>
                  </div>
                </form>
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
              <p class="eyebrow">Events</p>
              <h3>{{ selectedGroup ? `${selectedGroup.title} calendar` : "Group events" }}</h3>
            </div>
            <p v-if="selectedGroup">{{ selectedGroup.eventsCount || 0 }} events</p>
          </div>

          <div v-if="!selectedGroup" class="profile-empty-state">
            <h3>Select a group</h3>
            <p>Pick a group to plan events with its members.</p>
          </div>

          <template v-else-if="selectedGroup.isMember">
            <form class="stack-form group-event-form" @submit.prevent="handleCreateGroupEvent">
              <label>
                <span>Event title</span>
                <input
                  v-model.trim="groupEventForm.title"
                  type="text"
                  maxlength="160"
                  placeholder="Friday working session"
                />
              </label>

              <label>
                <span>Description</span>
                <textarea
                  v-model.trim="groupEventForm.description"
                  rows="4"
                  maxlength="2000"
                  placeholder="Share the agenda, location, and the outcome everyone should expect."
                ></textarea>
              </label>

              <label>
                <span>Starts at</span>
                <input
                  v-model="groupEventForm.startsAtLocal"
                  type="datetime-local"
                />
              </label>

              <div class="profile-form__actions">
                <p class="feed-note">Members can create lightweight events and RSVP inside the group.</p>
                <button type="submit" class="button" :disabled="isCreatingGroupEvent">
                  {{ isCreatingGroupEvent ? "Creating..." : "Create event" }}
                </button>
              </div>

              <p v-if="createGroupEventError" class="form-error">{{ createGroupEventError }}</p>
            </form>

            <p v-if="isLoadingGroupEvents" class="feed-note">Loading group events...</p>
            <p v-else-if="groupEventsError" class="form-error">{{ groupEventsError }}</p>

            <div v-else-if="groupEvents.length" class="group-event-stack">
              <article v-for="event in groupEvents" :key="event.id" class="group-event-card">
                <div class="group-event-card__header">
                  <div>
                    <h4>{{ event.title }}</h4>
                    <p class="feed-note">{{ formatDateTime(event.startsAt) }}</p>
                  </div>
                  <span v-if="event.viewerResponse" class="badge">{{ event.viewerResponse }}</span>
                </div>

                <p class="group-event-card__body">{{ event.description }}</p>

                <div class="group-card__stats">
                  <span class="badge badge--neutral">{{ event.goingCount }} going</span>
                  <span class="badge badge--neutral">{{ event.notGoingCount }} not going</span>
                  <span class="badge badge--neutral">By {{ displayName(event.creator) }}</span>
                </div>

                <div class="group-event-card__actions">
                  <button
                    type="button"
                    :class="['button', 'button--small', event.viewerResponse === 'going' ? null : 'button--ghost']"
                    :disabled="eventResponseLoading[event.id]"
                    @click="handleRespondToGroupEvent(event, 'going')"
                  >
                    {{ eventResponseLoading[event.id] && event.viewerResponse !== 'going' ? "Saving..." : "Going" }}
                  </button>
                  <button
                    type="button"
                    :class="['button', 'button--small', event.viewerResponse === 'not_going' ? null : 'button--ghost']"
                    :disabled="eventResponseLoading[event.id]"
                    @click="handleRespondToGroupEvent(event, 'not_going')"
                  >
                    {{ eventResponseLoading[event.id] && event.viewerResponse !== 'not_going' ? "Saving..." : "Not going" }}
                  </button>
                </div>
              </article>
            </div>

            <div v-else class="profile-empty-state">
              <h3>No events yet</h3>
              <p>Create the first meetup, call, or working session for this group.</p>
            </div>
          </template>

          <div v-else class="profile-empty-state">
            <h3>Join to plan events</h3>
            <p>Event planning stays inside the membership area in this phase.</p>
          </div>
        </section>

        <section class="panel">
          <div class="feed-header">
            <div>
              <p class="eyebrow">Invites</p>
              <h3>Invite people by private message</h3>
            </div>
            <p>{{ isLoadingInviteUsers ? "Refreshing..." : `${suggestedInviteUsers.length} people` }}</p>
          </div>

          <div v-if="!selectedGroup" class="profile-empty-state">
            <h3>Select a group</h3>
            <p>Open a group to invite someone into it through private chat.</p>
          </div>

          <template v-else-if="selectedGroup.isMember">
            <form class="stack-form" @submit.prevent="handleInviteUserToGroup">
              <label>
                <span>Recipient</span>
                <select v-model="inviteForm.recipientId">
                  <option value="">Choose a person</option>
                  <option v-for="user in suggestedInviteUsers" :key="user.id" :value="user.id">
                    {{ displayName(user) }}
                  </option>
                </select>
              </label>

              <label>
                <span>Optional note</span>
                <textarea
                  v-model.trim="inviteForm.note"
                  rows="4"
                  maxlength="500"
                  placeholder="Tell them why this group is relevant and what they should expect when they join."
                ></textarea>
              </label>

              <div class="profile-form__actions">
                <p class="feed-note">The invitation lands as a private message and opens into an actionable group card in chat.</p>
                <button type="submit" class="button" :disabled="isSendingInvite || isLoadingInviteUsers">
                  {{ isSendingInvite ? "Sending..." : "Send invite" }}
                </button>
              </div>

              <p v-if="inviteError" class="form-error">{{ inviteError }}</p>
              <p v-else-if="inviteUsersError" class="form-error">{{ inviteUsersError }}</p>
              <p v-else-if="inviteSuccess" class="form-hint form-hint--success">{{ inviteSuccess }}</p>
            </form>

            <div v-if="suggestedInviteUsers.length" class="user-stack group-invite-suggestions">
              <article
                v-for="user in suggestedInviteUsers.slice(0, 4)"
                :key="user.id"
                class="user-card"
              >
                <div class="group-invite-suggestions__identity">
                  <span class="user-avatar user-avatar--small">
                    <img
                      v-if="user.avatarUrl"
                      :src="user.avatarUrl"
                      :alt="`${displayName(user)} avatar`"
                      class="user-avatar__image"
                    />
                    <span v-else class="user-avatar__fallback">{{ userInitials(user) }}</span>
                  </span>

                  <div>
                    <strong>{{ displayName(user) }}</strong>
                    <p class="feed-note">{{ user.aboutMe || "Open a direct conversation through a group invite." }}</p>
                  </div>
                </div>

                <button type="button" class="button button--ghost button--small" @click="inviteForm.recipientId = user.id">
                  Select
                </button>
              </article>
            </div>

            <p v-else-if="!isLoadingInviteUsers" class="feed-note">No invite suggestions available right now, but the form will work again once discover users reloads.</p>
          </template>

          <div v-else class="profile-empty-state">
            <h3>Join to invite people</h3>
            <p>You need to be part of the group before inviting others into it.</p>
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
                  <span class="badge badge--neutral">{{ group.postsCount }} posts</span>
                  <span class="badge badge--neutral">{{ group.eventsCount }} events</span>
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
