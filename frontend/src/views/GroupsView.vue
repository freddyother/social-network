<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue"
import { useRouter } from "vue-router"

import {
  acceptGroupJoinRequest,
  createGroup,
  createGroupComment,
  createGroupEvent,
  createGroupPost,
  declineGroupJoinRequest,
  fetchGroup,
  fetchGroupComments,
  fetchGroupEvents,
  fetchGroupInviteCandidates,
  fetchGroupJoinRequests,
  fetchGroupMessages,
  fetchGroupPosts,
  fetchGroups,
  inviteUserToGroup,
  isApiError,
  joinGroup,
  normalizeGroupMessage,
  respondToGroupEvent,
  sendGroupMessage
} from "../services/api"
import { realtimeClient } from "../services/realtime"
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
const groupMessages = ref([])
const inviteUsers = ref([])
const groupJoinRequests = ref([])
const activeSidebarSection = ref("")
const previousBodyOverflow = ref("")
const groupMessageList = ref(null)
const isLoading = ref(false)
const isLoadingDetail = ref(false)
const isLoadingGroupPosts = ref(false)
const isLoadingGroupEvents = ref(false)
const isLoadingGroupMessages = ref(false)
const isLoadingInviteUsers = ref(false)
const isLoadingGroupJoinRequests = ref(false)
const isCreatingGroup = ref(false)
const isCreatingGroupPost = ref(false)
const isCreatingGroupEvent = ref(false)
const isSendingGroupMessage = ref(false)
const isSendingInvite = ref(false)
const createError = ref("")
const createGroupPostError = ref("")
const createGroupEventError = ref("")
const groupMessageError = ref("")
const inviteError = ref("")
const inviteSuccess = ref("")
const inviteUsersError = ref("")
const requestError = ref("")
const groupPostsError = ref("")
const groupEventsError = ref("")
const groupMessagesError = ref("")
const groupJoinRequestsError = ref("")
const joinLoading = reactive({})
const groupJoinRequestActionLoading = reactive({})
const groupCommentsExpanded = reactive({})
const groupCommentsByPost = reactive({})
const groupCommentsLoading = reactive({})
const groupCommentsLoaded = reactive({})
const groupCommentErrorByPost = reactive({})
const groupCommentSubmitting = reactive({})
const groupCommentForms = reactive({})
const activeGroupPostSlides = reactive({})
const eventResponseLoading = reactive({})
const createForm = reactive({
  title: "",
  description: ""
})
const groupPostForm = reactive({
  body: "",
  images: []
})
const groupPostPreviews = ref([])
const groupEventForm = reactive({
  title: "",
  description: "",
  startsAtLocal: ""
})
const groupMessageForm = reactive({
  body: ""
})
const inviteForm = reactive({
  recipientId: "",
  note: ""
})

let groupPostsRequestToken = 0
let groupEventsRequestToken = 0
let groupMessagesRequestToken = 0
let groupJoinRequestsRequestToken = 0
const removeRealtimeListeners = []

const activeGroupId = computed(() => String(props.groupId || "").trim())
const joinedGroups = computed(() => groups.value.filter((group) => group.isMember))
const discoverGroups = computed(() => groups.value.filter((group) => !group.isMember))
const canManageJoinRequests = computed(() => selectedGroup.value?.role === "creator")
const isSelectedGroupMember = computed(() => Boolean(selectedGroup.value?.isMember))
const nextUpcomingGroupEvent = computed(() => groupEvents.value[0] || null)
const latestGroupMessage = computed(() => groupMessages.value[groupMessages.value.length - 1] || null)
const isSidebarModalOpen = computed(() => Boolean(activeSidebarSection.value))
const suggestedInviteUsers = computed(() =>
  inviteUsers.value.filter((user) => user.id !== currentUserId.value)
)
const sidebarCards = computed(() => {
  const cards = []

  if (selectedGroup.value) {
    cards.push({
      id: "about",
      eyebrow: "Group",
      title: selectedGroup.value.title,
      meta: selectedGroup.value.isMember
        ? `${selectedGroup.value.postsCount || 0} posts · ${selectedGroup.value.eventsCount || 0} events`
        : selectedGroupAccessCopy(selectedGroup.value)
    })
  }

  if (selectedGroup.value?.isMember) {
    cards.push({
      id: "create-post",
      eyebrow: "Post",
      title: "Create post",
      meta: groupPosts.value.length
        ? `${groupPosts.value.length} posts in this feed`
        : "Start the first thread for this group"
    })
    cards.push({
      id: "events",
      eyebrow: "Events",
      title: "Event board",
      meta: nextUpcomingGroupEvent.value
        ? formatDateTime(nextUpcomingGroupEvent.value.startsAt)
        : "No events planned yet"
    })
    cards.push({
      id: "chat",
      eyebrow: "Chat",
      title: "Group messages",
      meta: isLoadingGroupMessages.value
        ? "Refreshing chat..."
        : latestGroupMessage.value
          ? `${displayName(latestGroupMessage.value.sender)} · ${formatRelativeTime(latestGroupMessage.value.sentAt)}`
          : "Start the group thread"
    })
    cards.push({
      id: "create-event",
      eyebrow: "Plan",
      title: "Create event",
      meta: "Schedule a meetup or working session"
    })
    cards.push({
      id: "invite",
      eyebrow: "Invite",
      title: "Invite people",
      meta: isLoadingInviteUsers.value
        ? "Refreshing candidates..."
        : `${suggestedInviteUsers.value.length} people available`
    })
  }

  if (canManageJoinRequests.value) {
    cards.push({
      id: "requests",
      eyebrow: "Requests",
      title: "Join approvals",
      meta: isLoadingGroupJoinRequests.value
        ? "Refreshing requests..."
        : `${groupJoinRequests.value.length} pending`
    })
  }

  if (joinedGroups.value.length || !selectedGroup.value) {
    cards.push({
      id: "memberships",
      eyebrow: "Memberships",
      title: "Your groups",
      meta: `${joinedGroups.value.length} joined`
    })
  }

  if (discoverGroups.value.length || !selectedGroup.value) {
    cards.push({
      id: "discover",
      eyebrow: "Discover",
      title: "Explore groups",
      meta: `${discoverGroups.value.length} available`
    })
  }

  cards.push({
    id: "create-group",
    eyebrow: "Create",
    title: "Start a group",
    meta: `${groups.value.length} total groups`
  })

  return cards
})

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

  const parts = [
    `${group.membersCount || 0} members`,
    `${group.postsCount || 0} posts`,
    `${group.eventsCount || 0} events`
  ]

  if (Number(group.pendingRequestsCount || 0) > 0) {
    parts.push(`${group.pendingRequestsCount} pending`)
  }

  return parts.join(" · ")
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

function groupMessageTimestampLabel(message) {
  if (!message) {
    return ""
  }

  return formatDateTime(message.sentAt)
}

function isOwnGroupMessage(message) {
  return message?.senderId === currentUserId.value
}

function commentCountLabel(count) {
  return Number(count || 0) === 1 ? "1 comment" : `${Number(count || 0)} comments`
}

function normalizedGroupPostMedia(post) {
  if (post?.media?.length) {
    return post.media
  }

  if (post?.imageUrl) {
    return [
      {
        id: `${post.id}-cover`,
        url: post.imageUrl,
        sortOrder: 1
      }
    ]
  }

  return []
}

function currentGroupPostSlide(post) {
  const media = normalizedGroupPostMedia(post)
  if (!media.length) {
    return 0
  }

  const currentIndex = Number(activeGroupPostSlides[post.id] || 0)
  return Math.max(0, Math.min(currentIndex, media.length - 1))
}

function selectedGroupPostMedia(post) {
  const media = normalizedGroupPostMedia(post)
  return media[currentGroupPostSlide(post)] || null
}

function setGroupPostSlide(post, index) {
  const media = normalizedGroupPostMedia(post)
  if (!post?.id || !media.length) {
    return
  }

  activeGroupPostSlides[post.id] = Math.max(0, Math.min(index, media.length - 1))
}

function previousGroupPostSlide(post) {
  const media = normalizedGroupPostMedia(post)
  if (!media.length) {
    return
  }

  setGroupPostSlide(post, currentGroupPostSlide(post) - 1)
}

function nextGroupPostSlide(post) {
  const media = normalizedGroupPostMedia(post)
  if (!media.length) {
    return
  }

  setGroupPostSlide(post, currentGroupPostSlide(post) + 1)
}

function revokeGroupPostPreviewURLs() {
  groupPostPreviews.value.forEach((preview) => URL.revokeObjectURL(preview.url))
}

function syncGroupPostPreviews(files) {
  revokeGroupPostPreviewURLs()
  groupPostPreviews.value = files.map((file) => ({
    name: file.name,
    url: URL.createObjectURL(file)
  }))
}

function resetGroupPostComposer() {
  groupPostForm.body = ""
  groupPostForm.images = []
  revokeGroupPostPreviewURLs()
  groupPostPreviews.value = []
}

function handleGroupPostImageSelection(event) {
  const files = Array.from(event.target.files || [])
  groupPostForm.images = files
  syncGroupPostPreviews(files)
}

function selectSidebarSection(sectionId) {
  activeSidebarSection.value = sectionId
}

function closeSidebarModal() {
  activeSidebarSection.value = ""
}

function handleWindowKeydown(event) {
  if (!isSidebarModalOpen.value) {
    return
  }

  if (event.key === "Escape") {
    closeSidebarModal()
  }
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
  clearReactiveMap(activeGroupPostSlides)
  clearReactiveMap(groupCommentsExpanded)
  clearReactiveMap(groupCommentsByPost)
  clearReactiveMap(groupCommentsLoading)
  clearReactiveMap(groupCommentsLoaded)
  clearReactiveMap(groupCommentErrorByPost)
  clearReactiveMap(groupCommentSubmitting)
  clearReactiveMap(groupCommentForms)

  if (clearComposer) {
    resetGroupPostComposer()
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

function clearGroupMessagesState({ clearComposer = true } = {}) {
  groupMessagesRequestToken += 1
  groupMessages.value = []
  groupMessagesError.value = ""
  groupMessageError.value = ""
  isLoadingGroupMessages.value = false

  if (clearComposer) {
    groupMessageForm.body = ""
  }
}

function clearGroupJoinRequestsState() {
  groupJoinRequestsRequestToken += 1
  groupJoinRequests.value = []
  groupJoinRequestsError.value = ""
  isLoadingGroupJoinRequests.value = false
  clearReactiveMap(groupJoinRequestActionLoading)
}

function joinButtonLabel(group) {
  if (joinLoading[group?.id]) {
    return "Sending..."
  }

  if (group?.joinRequestStatus === "pending") {
    return "Request pending"
  }

  return "Request to join"
}

function selectedGroupAccessCopy(group) {
  if (!group) {
    return ""
  }

  if (group.isMember) {
    return "You can post, comment, plan events, and invite people into this space now."
  }

  if (group.joinRequestStatus === "pending") {
    return "Your join request is pending review by the group creator."
  }

  return "Request access to this group and wait for the creator to approve you."
}

async function openGroup(groupId = "") {
  const normalizedGroupId = String(groupId || "").trim()
  await router.push(normalizedGroupId ? { name: "groups", params: { groupId: normalizedGroupId } } : { name: "groups" })
}

async function loadInviteUsers() {
  const normalizedGroupId = String(selectedGroup.value?.id || "").trim()
  if (!isAuthenticated.value || !normalizedGroupId || !selectedGroup.value?.isMember) {
    inviteUsers.value = []
    inviteForm.recipientId = ""
    inviteUsersError.value = ""
    return
  }

  isLoadingInviteUsers.value = true
  inviteUsersError.value = ""

  try {
    inviteUsers.value = await fetchGroupInviteCandidates(normalizedGroupId)

    const hasSelectedRecipient = inviteUsers.value.some((user) => user.id === inviteForm.recipientId)
    if (!hasSelectedRecipient) {
      inviteForm.recipientId = ""
    }

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
    clearGroupMessagesState()
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

async function scrollGroupMessagesToBottom() {
  await nextTick()
  const container = groupMessageList.value
  if (!container) {
    return
  }

  container.scrollTop = container.scrollHeight
}

async function loadGroupMessages(groupId) {
  const normalizedGroupId = String(groupId || "").trim()
  if (!normalizedGroupId) {
    clearGroupMessagesState()
    return
  }

  const requestToken = ++groupMessagesRequestToken
  isLoadingGroupMessages.value = true
  groupMessagesError.value = ""
  groupMessageError.value = ""
  groupMessages.value = []

  try {
    const loadedMessages = await fetchGroupMessages(normalizedGroupId)
    if (requestToken !== groupMessagesRequestToken) {
      return
    }

    groupMessages.value = loadedMessages
    await scrollGroupMessagesToBottom()
  } catch (error) {
    if (requestToken !== groupMessagesRequestToken) {
      return
    }

    groupMessagesError.value = error instanceof Error ? error.message : "Could not load group messages."
  } finally {
    if (requestToken === groupMessagesRequestToken) {
      isLoadingGroupMessages.value = false
    }
  }
}

async function loadGroupJoinRequests(groupId) {
  const normalizedGroupId = String(groupId || "").trim()
  if (!normalizedGroupId) {
    clearGroupJoinRequestsState()
    return
  }

  const requestToken = ++groupJoinRequestsRequestToken
  isLoadingGroupJoinRequests.value = true
  groupJoinRequestsError.value = ""
  groupJoinRequests.value = []

  try {
    const requests = await fetchGroupJoinRequests(normalizedGroupId)
    if (requestToken !== groupJoinRequestsRequestToken) {
      return
    }

    groupJoinRequests.value = requests
  } catch (error) {
    if (requestToken !== groupJoinRequestsRequestToken) {
      return
    }

    groupJoinRequestsError.value = error instanceof Error ? error.message : "Could not load join requests."
  } finally {
    if (requestToken === groupJoinRequestsRequestToken) {
      isLoadingGroupJoinRequests.value = false
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

function upsertGroupMessage(message, { scroll = false } = {}) {
  const normalizedMessageId = String(message?.id || "").trim()
  const normalizedGroupId = String(message?.groupId || "").trim()
  if (!normalizedMessageId || !normalizedGroupId || normalizedGroupId !== selectedGroup.value?.id) {
    return
  }

  const nextMessages = groupMessages.value.some((item) => item.id === normalizedMessageId)
    ? groupMessages.value.map((item) => (item.id === normalizedMessageId ? message : item))
    : [...groupMessages.value, message]

  groupMessages.value = nextMessages.sort((left, right) => {
    const sentAtDiff = Date.parse(left.sentAt || 0) - Date.parse(right.sentAt || 0)
    if (sentAtDiff !== 0) {
      return sentAtDiff
    }

    return String(left.id || "").localeCompare(String(right.id || ""))
  })

  if (scroll) {
    void scrollGroupMessagesToBottom()
  }
}

function handleRealtimeGroupMessage(payload) {
  const message = normalizeGroupMessage(payload?.message)
  if (!message) {
    return
  }

  upsertGroupMessage(message, {
    scroll: activeSidebarSection.value === "chat"
  })
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
    activeSidebarSection.value = "about"
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

  const existingGroup = groups.value.find((group) => group.id === normalizedGroupId)
  if (existingGroup?.joinRequestStatus === "pending") {
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
    activeSidebarSection.value = "about"
  } catch (error) {
    requestError.value = error instanceof Error ? error.message : "Could not send the join request."
  } finally {
    joinLoading[normalizedGroupId] = false
  }
}

async function handleRespondToGroupJoinRequest(joinRequest, accept) {
  const normalizedGroupId = String(selectedGroup.value?.id || "").trim()
  const normalizedRequestId = String(joinRequest?.id || "").trim()
  if (!normalizedGroupId || !normalizedRequestId) {
    return
  }

  groupJoinRequestActionLoading[normalizedRequestId] = true
  groupJoinRequestsError.value = ""

  try {
    if (accept) {
      await acceptGroupJoinRequest(normalizedGroupId, normalizedRequestId)
    } else {
      await declineGroupJoinRequest(normalizedGroupId, normalizedRequestId)
    }

    groupJoinRequests.value = groupJoinRequests.value.filter((request) => request.id !== normalizedRequestId)

    const refreshedGroup = await fetchGroup(normalizedGroupId)
    upsertGroup(refreshedGroup)
    if (selectedGroup.value?.id === normalizedGroupId) {
      selectedGroup.value = refreshedGroup
    }
  } catch (error) {
    groupJoinRequestsError.value = error instanceof Error ? error.message : "Could not update this join request."
  } finally {
    groupJoinRequestActionLoading[normalizedRequestId] = false
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
      body: groupPostForm.body,
      images: groupPostForm.images
    })

    groupPosts.value = [createdPost, ...groupPosts.value.filter((post) => post.id !== createdPost.id)]
    ensureGroupCommentState(createdPost.id)
    setGroupPostSlide(createdPost, 0)
    resetGroupPostComposer()
    bumpGroupPostsCount(normalizedGroupId, 1)
    closeSidebarModal()
  } catch (error) {
    if (isApiError(error)) {
      createGroupPostError.value =
        error.payload?.fields?.body ||
        error.payload?.fields?.images ||
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
    closeSidebarModal()
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

async function handleSendGroupMessage() {
  const normalizedGroupId = String(selectedGroup.value?.id || "").trim()
  if (!normalizedGroupId) {
    return
  }

  isSendingGroupMessage.value = true
  groupMessageError.value = ""
  groupMessagesError.value = ""

  try {
    const message = await sendGroupMessage(normalizedGroupId, {
      body: groupMessageForm.body
    })

    groupMessageForm.body = ""
    upsertGroupMessage(message, { scroll: true })
  } catch (error) {
    if (isApiError(error)) {
      groupMessageError.value =
        error.payload?.fields?.body ||
        error.message
    } else {
      groupMessageError.value = "Could not send the group message right now."
    }
  } finally {
    isSendingGroupMessage.value = false
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
    await loadInviteUsers()
    activeSidebarSection.value = "invite"
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
      clearGroupEventsState()
      clearGroupMessagesState()
      clearGroupJoinRequestsState()
      return
    }

    const previousGroupId = previous[0] || ""
    if (groupId !== previousGroupId) {
      clearGroupPostsState()
      clearGroupEventsState()
      clearGroupMessagesState()
      inviteUsers.value = []
      inviteForm.recipientId = ""
      clearGroupJoinRequestsState()
      inviteError.value = ""
      inviteSuccess.value = ""
    }

    if (!groupId || isMember !== "1") {
      groupPosts.value = []
      groupEvents.value = []
      groupMessages.value = []
      groupPostsError.value = ""
      groupEventsError.value = ""
      groupMessagesError.value = ""
      isLoadingGroupPosts.value = false
      isLoadingGroupEvents.value = false
      isLoadingGroupMessages.value = false
    } else {
      void loadGroupPosts(groupId)
      void loadGroupEvents(groupId)
      void loadGroupMessages(groupId)
    }

    if (selectedGroup.value?.role === "creator") {
      void loadGroupJoinRequests(groupId)
    } else {
      clearGroupJoinRequestsState()
    }

    void loadInviteUsers()
  },
  { immediate: true }
)

watch(
  () => sidebarCards.value.map((card) => card.id).join("|"),
  (value) => {
    const availableIds = value ? value.split("|").filter(Boolean) : []
    if (!availableIds.length || !availableIds.includes(activeSidebarSection.value)) {
      activeSidebarSection.value = ""
    }
  },
  { immediate: true }
)

onMounted(() => {
  if (typeof window !== "undefined") {
    window.addEventListener("keydown", handleWindowKeydown)
  }

  removeRealtimeListeners.push(
    realtimeClient.on("group.message.created", (event) => {
      handleRealtimeGroupMessage(event?.payload)
    })
  )
})

onBeforeUnmount(() => {
  removeRealtimeListeners.splice(0).forEach((dispose) => dispose())

  if (typeof window !== "undefined") {
    window.removeEventListener("keydown", handleWindowKeydown)
  }

  if (typeof document !== "undefined") {
    document.body.style.overflow = previousBodyOverflow.value
  }

  revokeGroupPostPreviewURLs()
})

watch(
  () => isSidebarModalOpen.value,
  (isOpen) => {
    if (typeof document === "undefined") {
      return
    }

    if (isOpen) {
      previousBodyOverflow.value = document.body.style.overflow
      document.body.style.overflow = "hidden"
    } else {
      document.body.style.overflow = previousBodyOverflow.value
    }
  }
)

watch(
  () => activeSidebarSection.value,
  (section) => {
    if (section === "chat") {
      void scrollGroupMessagesToBottom()
    }
  }
)
</script>

<template>
  <section class="page">
    <div v-if="!isAuthenticated" class="panel panel--narrow">
      <h3>Login required</h3>
      <p>Sign in to create groups, join communities, comment on group posts, plan events, and invite people in.</p>
    </div>

    <template v-else>
      <p v-if="requestError" class="form-error">{{ requestError }}</p>

      <div class="feed-layout groups-feed-layout">
        <section class="page groups-main-feed">
          <div class="feed-header groups-main-feed__header">
            <div class="groups-main-feed__summary">
              <p class="eyebrow">{{ selectedGroup ? "Group feed" : "Groups" }}</p>
              <h3>{{ selectedGroup ? selectedGroup.title : "Pick a group" }}</h3>
              <p class="feed-note">
                {{
                  selectedGroup
                    ? selectedGroup.isMember
                      ? "Only the internal timeline lives in this main column. Group tools stay on the right."
                      : selectedGroupAccessCopy(selectedGroup)
                    : isLoading
                      ? "Loading your groups and discovery spaces..."
                      : "Open one of your groups from the right rail to focus this feed."
                }}
              </p>
            </div>

            <div v-if="selectedGroup" class="groups-main-feed__actions">
              <span class="badge badge--neutral">{{ selectedGroup.postsCount || 0 }} posts</span>
              <span v-if="selectedGroup.isMember" class="badge badge--soft">{{ selectedGroup.role || "member" }}</span>
              <span v-else-if="selectedGroup.joinRequestStatus === 'pending'" class="badge badge--neutral">pending</span>
              <button type="button" class="button button--ghost button--small" @click="selectSidebarSection('about')">
                About group
              </button>
            </div>
          </div>

          <div v-if="!selectedGroup" class="panel profile-empty-state">
            <h3>No group selected yet</h3>
            <p>{{ isLoading ? "Loading the latest groups..." : "Choose one from the right side to load its feed here." }}</p>
          </div>

          <div v-else-if="!isSelectedGroupMember" class="panel profile-empty-state group-posts-locked">
            <h3>Join to unlock the feed</h3>
            <p>This main column only shows the internal posts once you are inside the group.</p>
            <div class="group-side-sheet__actions">
              <button
                type="button"
                class="button"
                :disabled="joinLoading[selectedGroup.id] || selectedGroup.joinRequestStatus === 'pending'"
                @click="handleJoinGroup(selectedGroup.id)"
              >
                {{ joinButtonLabel(selectedGroup) }}
              </button>
              <button type="button" class="button button--ghost button--small" @click="selectSidebarSection('about')">
                View group info
              </button>
            </div>
          </div>

          <div v-else-if="isLoadingGroupPosts" class="panel profile-empty-state">
            <h3>Loading group posts...</h3>
            <p>Pulling the latest discussion into the feed.</p>
          </div>

          <p v-else-if="groupPostsError" class="form-error">{{ groupPostsError }}</p>

          <div v-else-if="groupPosts.length" class="groups-main-feed__stack">
            <article v-for="post in groupPosts" :key="post.id" class="panel post-card">
              <header class="post-card__header">
                <div class="post-card__header-main">
                  <div class="groups-main-feed__author">
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
                      <p class="eyebrow">Group post</p>
                      <h3>{{ displayName(post.author) }}</h3>
                      <p class="post-card__meta">
                        <span>{{ groupPostTimestampLabel(post) }}</span>
                        <span class="badge badge--neutral">{{ commentCountLabel(post.commentsCount) }}</span>
                      </p>
                    </div>
                  </div>
                </div>

                <div class="post-card__timestamps">
                  <span class="badge badge--soft">{{ selectedGroup.title }}</span>
                  <span v-if="post.author?.id === currentUserId">You</span>
                </div>
              </header>

              <div class="post-card__body">
                <p v-if="post.body">{{ post.body }}</p>

                <div v-if="normalizedGroupPostMedia(post).length" class="carousel group-post-card__carousel">
                  <div class="carousel__frame">
                    <img
                      :src="selectedGroupPostMedia(post)?.url"
                      :alt="post.body || `${displayName(post.author)} group post image`"
                      class="group-post-card__image carousel__image"
                    />
                  </div>

                  <div v-if="normalizedGroupPostMedia(post).length > 1" class="carousel__controls">
                    <button
                      type="button"
                      class="button button--ghost button--small"
                      :disabled="currentGroupPostSlide(post) === 0"
                      @click="previousGroupPostSlide(post)"
                    >
                      Previous
                    </button>

                    <div class="carousel__dots">
                      <button
                        v-for="(media, index) in normalizedGroupPostMedia(post)"
                        :key="media.id"
                        type="button"
                        class="carousel__dot"
                        :class="{ 'carousel__dot--active': index === currentGroupPostSlide(post) }"
                        :aria-label="`View group image ${index + 1}`"
                        @click="setGroupPostSlide(post, index)"
                      ></button>
                    </div>

                    <button
                      type="button"
                      class="button button--ghost button--small"
                      :disabled="currentGroupPostSlide(post) >= normalizedGroupPostMedia(post).length - 1"
                      @click="nextGroupPostSlide(post)"
                    >
                      Next
                    </button>
                  </div>
                </div>
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
                    <p class="feed-note">Comments stay flat in this MVP so the feed remains lightweight.</p>
                    <button type="submit" class="button button--small" :disabled="groupCommentSubmitting[post.id]">
                      {{ groupCommentSubmitting[post.id] ? "Posting..." : "Post comment" }}
                    </button>
                  </div>
                </form>
              </div>
            </article>
          </div>

          <div v-else class="panel profile-empty-state group-posts-empty">
            <h3>No posts yet</h3>
            <p>Use the right rail to create the first post and start the conversation.</p>
          </div>
        </section>

        <aside class="feed-side groups-feed-side">
          <section class="panel">
            <div class="feed-header">
              <div>
                <p class="eyebrow">Workspace</p>
                <h3>{{ selectedGroup ? "Group controls" : "Group navigation" }}</h3>
              </div>
              <p>{{ sidebarCards.length }} sections</p>
            </div>

            <div class="groups-side-actions">
              <button
                v-for="card in sidebarCards"
                :key="card.id"
                type="button"
                class="group-side-launcher"
                :class="{ 'group-side-launcher--active': activeSidebarSection === card.id }"
                @click="selectSidebarSection(card.id)"
              >
                <p class="eyebrow">{{ card.eyebrow }}</p>
                <div class="group-side-launcher__title">
                  <strong>{{ card.title }}</strong>
                </div>
                <p class="group-side-launcher__meta">{{ card.meta }}</p>
              </button>
            </div>
          </section>

          <p class="feed-note groups-feed-side__hint">Tap a card to open the full workspace in a modal.</p>
        </aside>
      </div>

      <div
        v-if="isSidebarModalOpen"
        class="group-modal"
        role="dialog"
        aria-modal="true"
        @click.self="closeSidebarModal"
      >
        <div class="group-modal__dialog">
          <section class="panel panel--inset group-side-sheet group-modal__sheet">
            <header class="group-modal__topbar">
              <p class="feed-note">Group workspace</p>
              <button type="button" class="button button--ghost button--small" @click="closeSidebarModal">
                Close
              </button>
            </header>

            <template v-if="activeSidebarSection === 'about'">
              <div class="group-side-sheet__header">
                <div>
                  <p class="eyebrow">Group</p>
                  <h3>{{ selectedGroup ? selectedGroup.title : "Group details" }}</h3>
                </div>
                <p v-if="selectedGroup">{{ isLoadingDetail ? "Refreshing..." : "Overview" }}</p>
              </div>

              <template v-if="selectedGroup">
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
                  <span v-if="selectedGroup.pendingRequestsCount" class="badge badge--neutral">
                    {{ selectedGroup.pendingRequestsCount }} pending
                  </span>
                  <span v-if="selectedGroup.isMember" class="badge">{{ selectedGroup.role || "member" }}</span>
                  <span v-else-if="selectedGroup.joinRequestStatus === 'pending'" class="badge badge--neutral">pending</span>
                </div>

                <p class="feed-note">{{ selectedGroupAccessCopy(selectedGroup) }}</p>

                <div class="group-side-sheet__actions">
                  <button
                    v-if="!selectedGroup.isMember"
                    type="button"
                    class="button"
                    :disabled="joinLoading[selectedGroup.id] || selectedGroup.joinRequestStatus === 'pending'"
                    @click="handleJoinGroup(selectedGroup.id)"
                  >
                    {{ joinButtonLabel(selectedGroup) }}
                  </button>
                  <button
                    v-else
                    type="button"
                    class="button button--ghost button--small"
                    @click="selectSidebarSection('create-post')"
                  >
                    Create post
                  </button>
                  <button
                    v-if="selectedGroup.isMember"
                    type="button"
                    class="button button--ghost button--small"
                    @click="selectSidebarSection('events')"
                  >
                    View events
                  </button>
                </div>
              </template>

              <div v-else class="profile-empty-state">
                <h3>No group open</h3>
                <p>Select a membership or discovery result to load the group details here.</p>
              </div>
            </template>

            <template v-else-if="activeSidebarSection === 'create-post'">
              <div class="group-side-sheet__header">
                <div>
                  <p class="eyebrow">Post</p>
                  <h3>Create a group post</h3>
                </div>
                <p>{{ selectedGroup ? selectedGroup.title : "" }}</p>
              </div>

              <div v-if="!selectedGroup || !selectedGroup.isMember" class="profile-empty-state">
                <h3>Join to publish</h3>
                <p>The post composer appears here once you are part of the selected group.</p>
              </div>

              <form v-else class="stack-form" @submit.prevent="handleCreateGroupPost">
                <label>
                  <span>Start a conversation</span>
                  <textarea
                    v-model.trim="groupPostForm.body"
                    rows="5"
                    maxlength="4000"
                    placeholder="Share an update, ask for feedback, or post images from inside this group."
                  ></textarea>
                </label>

                <label>
                  <span>Images</span>
                  <input
                    type="file"
                    accept="image/png,image/jpeg,image/webp,image/gif"
                    multiple
                    @change="handleGroupPostImageSelection"
                  />
                </label>

                <div v-if="groupPostPreviews.length" class="preview-strip">
                  <figure v-for="preview in groupPostPreviews" :key="preview.url" class="preview-strip__item">
                    <img :src="preview.url" :alt="preview.name" />
                  </figure>
                </div>

                <div class="profile-form__actions">
                  <p class="feed-note">Only members can read this timeline. Images are optimized automatically and you can attach up to 6.</p>
                  <button type="submit" class="button" :disabled="isCreatingGroupPost">
                    {{ isCreatingGroupPost ? "Publishing..." : "Publish in group" }}
                  </button>
                </div>

                <p v-if="createGroupPostError" class="form-error">{{ createGroupPostError }}</p>
              </form>
            </template>

            <template v-else-if="activeSidebarSection === 'events'">
              <div class="group-side-sheet__header">
                <div>
                  <p class="eyebrow">Events</p>
                  <h3>{{ selectedGroup ? `${selectedGroup.title} calendar` : "Group events" }}</h3>
                </div>
                <div class="group-side-sheet__actions">
                  <button
                    v-if="selectedGroup?.isMember"
                    type="button"
                    class="button button--ghost button--small"
                    @click="selectSidebarSection('create-event')"
                  >
                    Create event
                  </button>
                </div>
              </div>

              <div v-if="!selectedGroup" class="profile-empty-state">
                <h3>Select a group</h3>
                <p>Pick a group to see its upcoming events.</p>
              </div>

              <div v-else-if="!selectedGroup.isMember" class="profile-empty-state">
                <h3>Join to plan events</h3>
                <p>Event planning stays inside the membership area in this phase.</p>
              </div>

              <template v-else>
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
                  <p>Create the first meetup, call, or working session for this group from the planner card.</p>
                </div>
              </template>
            </template>

            <template v-else-if="activeSidebarSection === 'create-event'">
              <div class="group-side-sheet__header">
                <div>
                  <p class="eyebrow">Plan</p>
                  <h3>Create an event</h3>
                </div>
                <p>{{ selectedGroup ? selectedGroup.title : "" }}</p>
              </div>

              <div v-if="!selectedGroup || !selectedGroup.isMember" class="profile-empty-state">
                <h3>Join to create events</h3>
                <p>The event planner appears here once you are a member of the selected group.</p>
              </div>

              <form v-else class="stack-form group-event-form" @submit.prevent="handleCreateGroupEvent">
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
            </template>

            <template v-else-if="activeSidebarSection === 'chat'">
              <div class="group-side-sheet__header">
                <div>
                  <p class="eyebrow">Chat</p>
                  <h3>{{ selectedGroup ? `${selectedGroup.title} messages` : "Group messages" }}</h3>
                </div>
                <p>{{ isLoadingGroupMessages ? "Refreshing..." : `${groupMessages.length} messages` }}</p>
              </div>

              <div v-if="!selectedGroup" class="profile-empty-state">
                <h3>Select a group</h3>
                <p>Open a group to read its member messages.</p>
              </div>

              <div v-else-if="!selectedGroup.isMember" class="profile-empty-state">
                <h3>Join to chat</h3>
                <p>Group messages stay visible to members only.</p>
              </div>

              <template v-else>
                <p v-if="isLoadingGroupMessages" class="feed-note">Loading group messages...</p>
                <p v-else-if="groupMessagesError" class="form-error">{{ groupMessagesError }}</p>

                <div ref="groupMessageList" class="group-message-list">
                  <article
                    v-for="message in groupMessages"
                    :key="message.id"
                    class="group-message"
                    :class="{ 'group-message--mine': isOwnGroupMessage(message) }"
                  >
                    <span v-if="!isOwnGroupMessage(message)" class="user-avatar user-avatar--small">
                      <img
                        v-if="message.sender?.avatarUrl"
                        :src="message.sender.avatarUrl"
                        :alt="`${displayName(message.sender)} avatar`"
                        class="user-avatar__image"
                      />
                      <span v-else class="user-avatar__fallback">{{ userInitials(message.sender) }}</span>
                    </span>

                    <div class="group-message__body">
                      <header>
                        <strong>{{ isOwnGroupMessage(message) ? "You" : displayName(message.sender) }}</strong>
                        <span>{{ groupMessageTimestampLabel(message) }}</span>
                      </header>
                      <p>{{ message.body }}</p>
                    </div>
                  </article>

                  <p v-if="!groupMessages.length && !isLoadingGroupMessages" class="feed-note">
                    No messages yet. Start the group thread.
                  </p>
                </div>

                <form class="stack-form group-message-composer" @submit.prevent="handleSendGroupMessage">
                  <label>
                    <span>Message</span>
                    <textarea
                      v-model.trim="groupMessageForm.body"
                      rows="3"
                      maxlength="2000"
                      placeholder="Write to everyone in this group."
                    ></textarea>
                  </label>

                  <div class="profile-form__actions">
                    <p class="feed-note">Messages are visible to current group members.</p>
                    <button type="submit" class="button" :disabled="isSendingGroupMessage">
                      {{ isSendingGroupMessage ? "Sending..." : "Send message" }}
                    </button>
                  </div>

                  <p v-if="groupMessageError" class="form-error">{{ groupMessageError }}</p>
                </form>
              </template>
            </template>

            <template v-else-if="activeSidebarSection === 'invite'">
              <div class="group-side-sheet__header">
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

                <p v-else-if="!isLoadingInviteUsers" class="feed-note">No invite suggestions available right now.</p>
              </template>

              <div v-else class="profile-empty-state">
                <h3>Join to invite people</h3>
                <p>You need to be part of the group before inviting others into it.</p>
              </div>
            </template>

            <template v-else-if="activeSidebarSection === 'requests'">
              <div class="group-side-sheet__header">
                <div>
                  <p class="eyebrow">Requests</p>
                  <h3>Pending join requests</h3>
                </div>
                <p>{{ isLoadingGroupJoinRequests ? "Refreshing..." : `${groupJoinRequests.length} pending` }}</p>
              </div>

              <p v-if="groupJoinRequestsError" class="form-error">{{ groupJoinRequestsError }}</p>
              <p v-else-if="isLoadingGroupJoinRequests" class="feed-note">Loading join requests...</p>

              <div v-else-if="groupJoinRequests.length" class="user-stack">
                <article
                  v-for="joinRequest in groupJoinRequests"
                  :key="joinRequest.id"
                  class="user-card group-join-request-card"
                >
                  <div class="group-invite-suggestions__identity">
                    <span class="user-avatar user-avatar--small">
                      <img
                        v-if="joinRequest.requester?.avatarUrl"
                        :src="joinRequest.requester.avatarUrl"
                        :alt="`${displayName(joinRequest.requester)} avatar`"
                        class="user-avatar__image"
                      />
                      <span v-else class="user-avatar__fallback">{{ userInitials(joinRequest.requester) }}</span>
                    </span>

                    <div>
                      <strong>{{ displayName(joinRequest.requester) }}</strong>
                      <p class="feed-note">Requested {{ formatRelativeTime(joinRequest.createdAt) }}</p>
                    </div>
                  </div>

                  <div class="user-card__actions">
                    <button
                      type="button"
                      class="button button--ghost button--small"
                      :disabled="groupJoinRequestActionLoading[joinRequest.id]"
                      @click="handleRespondToGroupJoinRequest(joinRequest, false)"
                    >
                      {{ groupJoinRequestActionLoading[joinRequest.id] ? "Saving..." : "Decline" }}
                    </button>
                    <button
                      type="button"
                      class="button button--small"
                      :disabled="groupJoinRequestActionLoading[joinRequest.id]"
                      @click="handleRespondToGroupJoinRequest(joinRequest, true)"
                    >
                      {{ groupJoinRequestActionLoading[joinRequest.id] ? "Saving..." : "Accept" }}
                    </button>
                  </div>
                </article>
              </div>

              <div v-else class="profile-empty-state">
                <h3>No pending requests</h3>
                <p>New requests to join this group will appear here for approval.</p>
              </div>
            </template>

            <template v-else-if="activeSidebarSection === 'memberships'">
              <div class="group-side-sheet__header">
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
                      <span v-if="group.pendingRequestsCount" class="badge badge--neutral">{{ group.pendingRequestsCount }} pending</span>
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
            </template>

            <template v-else-if="activeSidebarSection === 'discover'">
              <div class="group-side-sheet__header">
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
                      :disabled="joinLoading[group.id] || group.joinRequestStatus === 'pending'"
                      @click="handleJoinGroup(group.id)"
                    >
                      {{ joinButtonLabel(group) }}
                    </button>
                  </div>
                </article>
              </div>

              <p v-else class="feed-note">Every existing group is already in your memberships. Create the next one to expand the map.</p>
            </template>

            <template v-else-if="activeSidebarSection === 'create-group'">
              <div class="group-side-sheet__header">
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
            </template>
          </section>
        </div>
      </div>
    </template>
  </section>
</template>
