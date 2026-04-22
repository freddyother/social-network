<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue"
import { useRoute, useRouter } from "vue-router"

import {
  acceptFollowRequest,
  declineFollowRequest,
  deletePost as deletePostRequest,
  fetchComments,
  fetchFollowRequests,
  fetchUserProfile,
  followUser,
  isApiError,
  unfollowUser,
  updateProfile,
  updateProfileVisibility,
  updateThemePreference,
  uploadProfileAvatar
} from "../services/api"
import { realtimeClient } from "../services/realtime"
import { useAppStore } from "../stores/app"
import { THEME_OPTIONS } from "../theme"
import { formatDate, formatRelativeTime } from "../utils/date"

const props = defineProps({
  handle: {
    type: String,
    default: "me"
  }
})

const store = useAppStore()
const route = useRoute()
const router = useRouter()

const requestedHandle = computed(() => {
  const trimmed = typeof props.handle === "string" ? props.handle.trim() : ""
  return trimmed || "me"
})
const wantsOwnProfile = computed(() => requestedHandle.value.toLowerCase() === "me")
const effectiveHandle = computed(() => (wantsOwnProfile.value ? store.state.currentUser?.nickname || "" : requestedHandle.value))
const isAuthenticated = computed(() => Boolean(store.state.currentUser))

const viewedProfile = ref(null)
const viewedPosts = ref([])
const isLoadingProfile = ref(false)
const profileLoadError = ref("")
const profileActionError = ref("")
const isFollowingProfile = ref(false)
const selectedPost = ref(null)
const selectedPostSlide = ref(0)
const deletingPostId = ref("")
const modalCommentsByPost = reactive({})
const modalCommentsLoading = reactive({})
const modalCommentsErrorByPost = reactive({})
const modalCommentsLoaded = reactive({})
const previousBodyOverflow = ref("")

const privacyForm = reactive({
  visibility: "public"
})

const profileForm = reactive({
  firstName: "",
  lastName: "",
  aboutMe: ""
})

const themeOptions = THEME_OPTIONS
const requests = ref([])
const avatarInput = ref(null)
const profileError = ref("")
const profileDetailsError = ref("")
const postDeleteError = ref("")
const requestError = ref("")
const themeError = ref("")
const avatarError = ref("")
const isUploadingAvatar = ref(false)
const isSavingProfile = ref(false)
const isSavingPrivacy = ref(false)
const isSavingTheme = ref("")
const loadingRequests = ref(false)
const requestLoading = reactive({})
const removeRealtimeListeners = []
const followRequestsSection = ref(null)

const editableProfile = computed(() => (viewedProfile.value?.isViewer ? store.state.currentUser : null))
const isOwnProfile = computed(() => Boolean(viewedProfile.value?.isViewer))
const activeThemePreference = computed(() => editableProfile.value?.themePreference || store.state.themePreference)
const relationshipStatus = computed(() => viewedProfile.value?.relationshipStatus || "not_following")
const canViewPosts = computed(() => Boolean(viewedProfile.value?.canViewPosts))
const canMessage = computed(() => Boolean(viewedProfile.value?.canMessage))
const showOwnProfilePrompt = computed(
  () => wantsOwnProfile.value && !isAuthenticated.value && store.state.apiStatus !== "checking" && !isLoadingProfile.value
)
const hasVisibilityChanges = computed(
  () => Boolean(editableProfile.value) && privacyForm.visibility !== (editableProfile.value?.profileVisibility || "public")
)
const hasProfileChanges = computed(() => {
  if (!editableProfile.value) {
    return false
  }

  return (
    profileForm.firstName !== (editableProfile.value.firstName || "") ||
    profileForm.lastName !== (editableProfile.value.lastName || "") ||
    profileForm.aboutMe !== (editableProfile.value.aboutMe || "")
  )
})
const profileName = computed(() => {
  const user = viewedProfile.value || editableProfile.value || store.state.currentUser
  if (!user) {
    return wantsOwnProfile.value ? "Your profile" : requestedHandle.value
  }

  return displayName(user)
})
const profileInitials = computed(() => {
  const user = viewedProfile.value || editableProfile.value || store.state.currentUser
  if (!user) {
    return "N"
  }

  const source = displayName(user) || user.email || "NEXO"
  return source
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() || "")
    .join("") || "N"
})
const followButtonLabel = computed(() => {
  if (isFollowingProfile.value) {
    return "Saving..."
  }

  if (relationshipStatus.value === "following") {
    return "Unfollow"
  }

  if (relationshipStatus.value === "requested") {
    return "Requested"
  }

  return "Follow"
})
const canRunRelationshipAction = computed(
  () => relationshipStatus.value === "not_following" || relationshipStatus.value === "following"
)
const showMessageButton = computed(() => isAuthenticated.value && canMessage.value)
const timelineTitle = computed(() => (isOwnProfile.value ? "Your timeline" : `${profileName.value}'s timeline`))
const timelineSummary = computed(() => {
  if (!viewedProfile.value) {
    return ""
  }

  if (!canViewPosts.value) {
    return "Locked timeline"
  }

  return `${viewedPosts.value.length} visible posts`
})
const emptyPostTitle = computed(() => (isOwnProfile.value ? "You haven't posted yet" : "No visible posts yet"))
const emptyPostDescription = computed(() =>
  isOwnProfile.value
    ? "Use the create flow to publish your first post and start filling this profile."
    : "This account hasn't shared any posts you can see yet."
)
const selectedPostComments = computed(() => {
  const postId = selectedPost.value?.id
  return postId ? modalCommentsByPost[postId] || [] : []
})
const isLoadingSelectedPostComments = computed(() => {
  const postId = selectedPost.value?.id
  return postId ? Boolean(modalCommentsLoading[postId]) : false
})
const selectedPostCommentsError = computed(() => {
  const postId = selectedPost.value?.id
  return postId ? modalCommentsErrorByPost[postId] || "" : ""
})
const selectedPostMedia = computed(() => {
  if (!selectedPost.value?.media?.length) {
    return null
  }

  return selectedPost.value.media[selectedPostSlide.value] || selectedPost.value.media[0] || null
})
const hasSelectedPostMedia = computed(() => Boolean(selectedPostMedia.value))
const isDeletingSelectedPost = computed(
  () => Boolean(selectedPost.value?.id) && deletingPostId.value === selectedPost.value.id
)

function displayName(user) {
  if (!user) {
    return ""
  }

  return user.nickname || `${user.firstName} ${user.lastName}`.trim() || user.email || "Unknown account"
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

function profileSubtitle() {
  if (isOwnProfile.value && editableProfile.value) {
    return `Signed in as ${editableProfile.value.email} with a ${editableProfile.value.profileVisibility} profile.`
  }

  if (!viewedProfile.value) {
    return ""
  }

  const nickname = viewedProfile.value.nickname ? `@${viewedProfile.value.nickname}` : "NEXO member"
  return `${nickname} · ${viewedProfile.value.profileVisibility} account`
}

function accessDescription() {
  if (!viewedProfile.value) {
    return ""
  }

  if (canViewPosts.value) {
    return "You can currently see the posts this account shares with you."
  }

  if (isAuthenticated.value) {
    return "This account is private. Follow to send an access request before posts appear here."
  }

  return "This account is private. Sign in first if you want to follow and request access."
}

function postTimestamp(post) {
  if (!post) {
    return ""
  }

  return post.updatedAt !== post.createdAt
    ? `Edited ${formatRelativeTime(post.updatedAt)}`
    : formatRelativeTime(post.createdAt)
}

function commentCountLabel(count) {
  return Number(count || 0) === 1 ? "1 comment" : `${Number(count || 0)} comments`
}

function commentTimestampLabel(comment) {
  if (!comment) {
    return ""
  }

  return comment.updatedAt !== comment.createdAt
    ? `Edited ${formatRelativeTime(comment.updatedAt)}`
    : formatRelativeTime(comment.createdAt)
}

function postPreviewText(post) {
  const text = String(post?.body || post?.title || "").trim()
  if (!text) {
    return "Open this post to see its full details and comment thread."
  }

  return text.length > 120 ? `${text.slice(0, 117)}...` : text
}

function ensureModalCommentState(postId) {
  if (!postId) {
    return
  }

  if (!modalCommentsByPost[postId]) {
    modalCommentsByPost[postId] = []
  }

  if (!(postId in modalCommentsLoading)) {
    modalCommentsLoading[postId] = false
  }

  if (!(postId in modalCommentsErrorByPost)) {
    modalCommentsErrorByPost[postId] = ""
  }

  if (!(postId in modalCommentsLoaded)) {
    modalCommentsLoaded[postId] = false
  }
}

async function loadPostComments(postId, options = {}) {
  if (!postId) {
    return
  }

  const { force = false } = options
  ensureModalCommentState(postId)

  if (modalCommentsLoading[postId] || (modalCommentsLoaded[postId] && !force)) {
    return
  }

  modalCommentsLoading[postId] = true
  modalCommentsErrorByPost[postId] = ""

  try {
    modalCommentsByPost[postId] = await fetchComments(postId)
    modalCommentsLoaded[postId] = true
  } catch (error) {
    modalCommentsErrorByPost[postId] = error instanceof Error ? error.message : "Could not load comments for this post."
  } finally {
    modalCommentsLoading[postId] = false
  }
}

function openPostModal(post, mediaIndex = 0) {
  if (!post) {
    return
  }

  const mediaCount = post.media?.length || 0
  const normalizedIndex = mediaCount ? Math.max(0, Math.min(mediaIndex, mediaCount - 1)) : 0

  postDeleteError.value = ""
  selectedPost.value = post
  selectedPostSlide.value = normalizedIndex
  void loadPostComments(post.id)
}

function closePostModal() {
  selectedPost.value = null
  selectedPostSlide.value = 0
}

function removeViewedPost(postId) {
  if (!postId) {
    return
  }

  const hasPost = viewedPosts.value.some((post) => post.id === postId)
  if (!hasPost) {
    return
  }

  viewedPosts.value = viewedPosts.value.filter((post) => post.id !== postId)

  if (viewedProfile.value) {
    viewedProfile.value = {
      ...viewedProfile.value,
      postsCount: Math.max(0, Number(viewedProfile.value.postsCount || 0) - 1)
    }
  }

  delete modalCommentsByPost[postId]
  delete modalCommentsLoading[postId]
  delete modalCommentsErrorByPost[postId]
  delete modalCommentsLoaded[postId]

  if (selectedPost.value?.id === postId) {
    closePostModal()
  }
}

function setSelectedPostSlide(index) {
  const mediaCount = selectedPost.value?.media?.length || 0
  if (!mediaCount) {
    selectedPostSlide.value = 0
    return
  }

  selectedPostSlide.value = Math.max(0, Math.min(index, mediaCount - 1))
}

function previousSelectedPostSlide() {
  const mediaCount = selectedPost.value?.media?.length || 0
  if (mediaCount < 2) {
    return
  }

  selectedPostSlide.value = (selectedPostSlide.value - 1 + mediaCount) % mediaCount
}

function nextSelectedPostSlide() {
  const mediaCount = selectedPost.value?.media?.length || 0
  if (mediaCount < 2) {
    return
  }

  selectedPostSlide.value = (selectedPostSlide.value + 1) % mediaCount
}

function routeSection() {
  const rawValue = route.query.section
  if (Array.isArray(rawValue)) {
    return String(rawValue[0] || "").trim()
  }

  return typeof rawValue === "string" ? rawValue.trim() : ""
}

function sameProfileId(left, right) {
  return String(left || "").trim() === String(right || "").trim()
}

function loadPrivacyValue() {
  privacyForm.visibility = editableProfile.value?.profileVisibility || "public"
}

function loadProfileForm() {
  profileForm.firstName = editableProfile.value?.firstName || ""
  profileForm.lastName = editableProfile.value?.lastName || ""
  profileForm.aboutMe = editableProfile.value?.aboutMe || ""
}

async function loadViewedProfile() {
  if (wantsOwnProfile.value && !store.state.currentUser?.nickname) {
    if (store.state.apiStatus === "checking") {
      return
    }

    viewedProfile.value = null
    viewedPosts.value = []
    profileLoadError.value = ""
    return
  }

  const handle = effectiveHandle.value.trim()
  if (!handle) {
    viewedProfile.value = null
    viewedPosts.value = []
    profileLoadError.value = wantsOwnProfile.value ? "" : "This profile was not found."
    return
  }

  isLoadingProfile.value = true
  profileLoadError.value = ""
  profileActionError.value = ""

  try {
    const payload = await fetchUserProfile(handle)
    viewedProfile.value = payload.profile
    viewedPosts.value = payload.posts
  } catch (error) {
    viewedProfile.value = null
    viewedPosts.value = []
    if (isApiError(error, 404)) {
      profileLoadError.value = "This profile was not found."
    } else {
      profileLoadError.value = error instanceof Error ? error.message : "Could not load this profile."
    }
  } finally {
    isLoadingProfile.value = false
  }
}

async function loadFollowRequests() {
  if (!isOwnProfile.value || !editableProfile.value) {
    requests.value = []
    return
  }

  loadingRequests.value = true
  requestError.value = ""

  try {
    requests.value = await fetchFollowRequests()
  } catch (error) {
    requestError.value = error instanceof Error ? error.message : "Could not load follow requests."
  } finally {
    loadingRequests.value = false
  }
}

async function saveProfile() {
  if (!editableProfile.value || !hasProfileChanges.value) {
    return
  }

  isSavingProfile.value = true
  profileDetailsError.value = ""

  try {
    const updatedUser = await updateProfile({
      firstName: profileForm.firstName,
      lastName: profileForm.lastName,
      aboutMe: profileForm.aboutMe
    })

    store.setCurrentUser(updatedUser)
    await loadViewedProfile()
  } catch (error) {
    if (isApiError(error)) {
      profileDetailsError.value =
        error.payload?.fields?.firstName ||
        error.payload?.fields?.lastName ||
        error.payload?.fields?.aboutMe ||
        error.message
    } else {
      profileDetailsError.value = "Could not update your profile details."
    }
  } finally {
    isSavingProfile.value = false
  }
}

async function saveProfileVisibility() {
  if (!editableProfile.value || !hasVisibilityChanges.value) {
    return
  }

  isSavingPrivacy.value = true
  profileError.value = ""

  try {
    const updatedUser = await updateProfileVisibility(privacyForm.visibility)
    store.setCurrentUser(updatedUser)
    await loadViewedProfile()
  } catch (error) {
    if (isApiError(error)) {
      profileError.value = error.message
    } else {
      profileError.value = "Could not update profile visibility."
    }
  } finally {
    isSavingPrivacy.value = false
  }
}

function openAvatarPicker() {
  avatarInput.value?.click()
}

async function handleAvatarChange(event) {
  const [file] = Array.from(event.target?.files || [])
  if (!file || !editableProfile.value) {
    return
  }

  isUploadingAvatar.value = true
  avatarError.value = ""

  try {
    const updatedUser = await uploadProfileAvatar(file)
    store.setCurrentUser(updatedUser)
    await loadViewedProfile()
  } catch (error) {
    if (isApiError(error)) {
      avatarError.value = error.message
    } else {
      avatarError.value = "Could not upload your profile photo."
    }
  } finally {
    isUploadingAvatar.value = false
    if (event.target) {
      event.target.value = ""
    }
  }
}

function themePreviewStyle(option) {
  return {
    "--theme-preview-1": option.swatches[0],
    "--theme-preview-2": option.swatches[1],
    "--theme-preview-3": option.swatches[2],
    "--theme-preview-4": option.swatches[3]
  }
}

async function selectTheme(themePreference) {
  if (!editableProfile.value || activeThemePreference.value === themePreference) {
    return
  }

  const previousThemePreference = activeThemePreference.value
  isSavingTheme.value = themePreference
  themeError.value = ""
  store.updateCurrentUser({ themePreference })

  try {
    const updatedUser = await updateThemePreference(themePreference)
    store.setCurrentUser(updatedUser)
  } catch (error) {
    store.updateCurrentUser({ themePreference: previousThemePreference })
    if (isApiError(error)) {
      themeError.value = error.message
    } else {
      themeError.value = "Could not update theme preference."
    }
  } finally {
    isSavingTheme.value = ""
  }
}

async function handleRelationshipAction() {
  if (!viewedProfile.value || !isAuthenticated.value || !canRunRelationshipAction.value) {
    return
  }

  isFollowingProfile.value = true
  profileActionError.value = ""

  try {
    if (relationshipStatus.value === "following") {
      await unfollowUser(viewedProfile.value.id)
    } else {
      await followUser(viewedProfile.value.id)
    }
    await loadViewedProfile()
  } catch (error) {
    profileActionError.value = error instanceof Error ? error.message : "Could not update follow status."
  } finally {
    isFollowingProfile.value = false
  }
}

async function respondToRequest(requestId, accept) {
  requestLoading[requestId] = true
  requestError.value = ""

  try {
    if (accept) {
      await acceptFollowRequest(requestId)
    } else {
      await declineFollowRequest(requestId)
    }

    requests.value = requests.value.filter((request) => request.id !== requestId)
    await loadViewedProfile()
  } catch (error) {
    requestError.value = error instanceof Error ? error.message : "Could not update the request."
  } finally {
    requestLoading[requestId] = false
  }
}

async function focusRequestedSection() {
  if (!isOwnProfile.value || routeSection() !== "follow-requests") {
    return
  }

  await nextTick()
  followRequestsSection.value?.scrollIntoView({
    behavior: "smooth",
    block: "start"
  })
}

function openMessage() {
  if (!viewedProfile.value?.id || !canMessage.value) {
    return
  }

  void router.push({
    path: "/chat",
    query: {
      user: viewedProfile.value.id
    }
  })
}

function openLogin() {
  void router.push("/login")
}

async function deleteSelectedPost() {
  if (!isOwnProfile.value || !selectedPost.value?.id) {
    return
  }

  const confirmed =
    typeof window === "undefined"
      ? true
      : window.confirm(
          "Delete this post permanently?\n\nThis action is irreversible and will permanently remove the post and all of its comments."
        )
  if (!confirmed) {
    return
  }

  deletingPostId.value = selectedPost.value.id
  postDeleteError.value = ""

  try {
    await deletePostRequest(selectedPost.value.id)
    removeViewedPost(selectedPost.value.id)
  } catch (error) {
    postDeleteError.value = error instanceof Error ? error.message : "Could not delete the post right now."
  } finally {
    deletingPostId.value = ""
  }
}

function handleWindowKeydown(event) {
  if (!selectedPost.value) {
    return
  }

  if (event.key === "Escape") {
    closePostModal()
  } else if (event.key === "ArrowLeft") {
    previousSelectedPostSlide()
  } else if (event.key === "ArrowRight") {
    nextSelectedPostSlide()
  }
}

watch(
  () => [
    editableProfile.value?.firstName,
    editableProfile.value?.lastName,
    editableProfile.value?.aboutMe,
    editableProfile.value?.profileVisibility
  ],
  () => {
    loadPrivacyValue()
    loadProfileForm()
  },
  { immediate: true }
)

watch(
  () => [effectiveHandle.value, wantsOwnProfile.value, store.state.apiStatus, store.state.currentUser?.id],
  () => {
    void loadViewedProfile()
  },
  { immediate: true }
)

watch(
  () => [viewedProfile.value?.id, isOwnProfile.value],
  async () => {
    profileDetailsError.value = ""
    await loadFollowRequests()
    await focusRequestedSection()
  },
  { immediate: true }
)

watch(
  () => route.query.section,
  () => {
    void focusRequestedSection()
  }
)

watch(
  () => [effectiveHandle.value, wantsOwnProfile.value],
  () => {
    closePostModal()
  }
)

watch(
  () => Boolean(selectedPost.value),
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

removeRealtimeListeners.push(
  realtimeClient.on("follow_request.created", () => {
    if (!isOwnProfile.value) {
      return
    }

    void loadFollowRequests()
  }),
  realtimeClient.on("follow_request.accepted", (event) => {
    const acceptedProfileID = event.payload?.recipientId
    if (!acceptedProfileID || !viewedProfile.value || isOwnProfile.value) {
      return
    }

    if (sameProfileId(viewedProfile.value.id, acceptedProfileID)) {
      void loadViewedProfile()
    }
  }),
  realtimeClient.on("post.deleted", (event) => {
    const postID = String(event.payload?.postId || "").trim()
    if (!postID) {
      return
    }

    removeViewedPost(postID)
  })
)

onMounted(() => {
  if (typeof window !== "undefined") {
    window.addEventListener("keydown", handleWindowKeydown)
  }
})

onBeforeUnmount(() => {
  if (typeof window !== "undefined") {
    window.removeEventListener("keydown", handleWindowKeydown)
  }

  if (typeof document !== "undefined") {
    document.body.style.overflow = previousBodyOverflow.value
  }

  removeRealtimeListeners.splice(0).forEach((dispose) => dispose())
})
</script>

<template>
  <section class="page">
    <div class="panel profile-hero">
      <p class="eyebrow">Profile</p>

      <div v-if="isLoadingProfile && !viewedProfile" class="profile-empty-state">
        <h2>Loading profile...</h2>
        <p>We are fetching the latest public details and visible posts for this account.</p>
      </div>

      <div v-else-if="showOwnProfilePrompt" class="profile-empty-state">
        <h2>Sign in to open your profile</h2>
        <p>Your editable profile, theme settings, and follow requests only appear after authentication.</p>
        <button type="button" class="button" @click="openLogin">Go to login</button>
      </div>

      <div v-else-if="profileLoadError && !viewedProfile" class="profile-empty-state">
        <h2>Profile unavailable</h2>
        <p>{{ profileLoadError }}</p>
      </div>

      <template v-else-if="viewedProfile">
        <div class="profile-hero__body">
          <div class="profile-hero__avatar-column">
            <div class="user-avatar user-avatar--profile">
              <img
                v-if="viewedProfile.avatarUrl"
                :src="viewedProfile.avatarUrl"
                :alt="`${profileName} profile photo`"
                class="user-avatar__image"
              />
              <span v-else class="user-avatar__fallback">{{ profileInitials }}</span>
            </div>

            <input
              v-if="isOwnProfile && editableProfile"
              ref="avatarInput"
              class="visually-hidden"
              type="file"
              accept="image/png,image/jpeg,image/webp,image/gif"
              @change="handleAvatarChange"
            />

            <button
              v-if="isOwnProfile && editableProfile"
              type="button"
              class="button button--ghost button--small"
              :disabled="isUploadingAvatar"
              @click="openAvatarPicker"
            >
              {{ isUploadingAvatar ? "Uploading..." : viewedProfile.avatarUrl ? "Change photo" : "Add photo" }}
            </button>

            <p v-if="isOwnProfile && editableProfile" class="feed-note">Images are optimized automatically when uploaded.</p>
            <p v-if="avatarError" class="form-error">{{ avatarError }}</p>
          </div>

          <div class="profile-hero__content">
            <div class="profile-hero__header">
              <div class="profile-hero__identity">
                <h2>{{ profileName }}</h2>
                <p>{{ profileSubtitle() }}</p>
              </div>

              <div v-if="!isOwnProfile" class="profile-hero__actions">
                <template v-if="isAuthenticated">
                  <button
                    type="button"
                    class="button"
                    :disabled="!canRunRelationshipAction || isFollowingProfile"
                    @click="handleRelationshipAction"
                  >
                    {{ followButtonLabel }}
                  </button>
                  <button
                    v-if="showMessageButton"
                    type="button"
                    class="button button--ghost"
                    @click="openMessage"
                  >
                    Message
                  </button>
                </template>

                <button v-else type="button" class="button" @click="openLogin">
                  Login to follow
                </button>
              </div>
            </div>

            <p class="profile-hero__about">
              {{ viewedProfile.aboutMe || (isOwnProfile ? "Add a short introduction so your profile feels alive." : "This profile has not added an introduction yet.") }}
            </p>

            <div class="profile-hero__meta">
              <span class="badge">{{ viewedProfile.profileVisibility }}</span>
              <span class="badge badge--neutral">{{ relationshipLabel(relationshipStatus) }}</span>
            </div>

            <div class="profile-stats">
              <article class="profile-stat-card">
                <strong>{{ viewedProfile.followersCount }}</strong>
                <span>Followers</span>
              </article>
              <article class="profile-stat-card">
                <strong>{{ viewedProfile.followingCount }}</strong>
                <span>Following</span>
              </article>
              <article class="profile-stat-card">
                <strong>{{ viewedProfile.postsCount }}</strong>
                <span>Posts</span>
              </article>
            </div>

            <p v-if="profileActionError" class="form-error">{{ profileActionError }}</p>
          </div>
        </div>
      </template>
    </div>

    <template v-if="viewedProfile">
      <div class="grid grid--two">
        <article class="panel">
          <template v-if="isOwnProfile && editableProfile">
            <h3>Edit profile</h3>
            <p>Update your public identity here. Photo changes stay in the profile header above.</p>
            <div class="stack-form">
              <div class="form-grid">
                <label>
                  <span>First name</span>
                  <input v-model.trim="profileForm.firstName" type="text" placeholder="First name" />
                </label>
                <label>
                  <span>Last name</span>
                  <input v-model.trim="profileForm.lastName" type="text" placeholder="Last name" />
                </label>
                <label class="form-grid__full">
                  <span>About me</span>
                  <textarea
                    v-model="profileForm.aboutMe"
                    rows="5"
                    maxlength="500"
                    placeholder="Tell your story in a few lines to make this profile feel alive."
                  ></textarea>
                </label>
              </div>
              <div class="profile-form__actions">
                <p class="feed-note">Email, nickname, and date of birth stay locked for now.</p>
                <button
                  type="button"
                  class="button"
                  :disabled="isSavingProfile || !hasProfileChanges"
                  @click="saveProfile"
                >
                  {{ isSavingProfile ? "Saving..." : "Save profile" }}
                </button>
              </div>
              <p v-if="profileDetailsError" class="form-error">{{ profileDetailsError }}</p>
            </div>
          </template>

          <template v-else>
            <h3>Profile overview</h3>
            <p>{{ viewedProfile.aboutMe || "This account has not written an introduction yet." }}</p>
            <p class="feed-note">{{ accessDescription() }}</p>
          </template>
        </article>

        <article class="panel">
          <template v-if="isOwnProfile && editableProfile">
            <h3>Locked fields</h3>
            <p>Date of birth: {{ formatDate(editableProfile.dateOfBirth) }}</p>
            <p>Nickname: {{ editableProfile.nickname || "Not set yet" }}</p>
            <p>Email: {{ editableProfile.email }}</p>
          </template>

          <template v-else>
            <h3>Access</h3>
            <p>{{ accessDescription() }}</p>
            <p class="feed-note">
              {{ canViewPosts ? "You are seeing the posts this account currently shares with you." : "The timeline stays hidden until this account approves the relationship." }}
            </p>
          </template>
        </article>
      </div>

      <template v-if="isOwnProfile && editableProfile">
        <div class="grid grid--two">
          <section class="panel">
            <p class="eyebrow">Privacy</p>
            <h3>Account visibility</h3>
            <p>
              Public accounts can publish fully public posts. Private accounts only reveal content to approved followers.
            </p>
            <div class="privacy-row">
              <select v-model="privacyForm.visibility">
                <option value="public">Public account</option>
                <option value="private">Private account</option>
              </select>
              <button
                type="button"
                class="button"
                :disabled="isSavingPrivacy || !hasVisibilityChanges"
                @click="saveProfileVisibility"
              >
                {{ isSavingPrivacy ? "Saving..." : "Save visibility" }}
              </button>
            </div>
            <p v-if="profileError" class="form-error">{{ profileError }}</p>
          </section>

          <section class="panel">
            <p class="eyebrow">Appearance</p>
            <h3>Color theme</h3>
            <p>Pick the NEXO palette you want to use. The change is applied immediately.</p>
            <div class="theme-picker" role="radiogroup" aria-label="Theme preference">
              <button
                v-for="option in themeOptions"
                :key="option.value"
                type="button"
                class="theme-option"
                :class="{ 'theme-option--active': activeThemePreference === option.value }"
                :style="themePreviewStyle(option)"
                :disabled="Boolean(isSavingTheme)"
                :aria-pressed="activeThemePreference === option.value"
                @click="selectTheme(option.value)"
              >
                <span class="theme-option__swatches" aria-hidden="true">
                  <span class="theme-option__swatch theme-option__swatch--1"></span>
                  <span class="theme-option__swatch theme-option__swatch--2"></span>
                  <span class="theme-option__swatch theme-option__swatch--3"></span>
                  <span class="theme-option__swatch theme-option__swatch--4"></span>
                </span>
                <span class="theme-option__meta">
                  <strong>{{ option.label }}</strong>
                  <small>{{ option.description }}</small>
                </span>
                <span class="badge" :class="activeThemePreference === option.value ? '' : 'badge--neutral'">
                  {{ isSavingTheme === option.value ? "Saving..." : activeThemePreference === option.value ? "Active" : "Apply" }}
                </span>
              </button>
            </div>
            <p v-if="themeError" class="form-error">{{ themeError }}</p>
          </section>
        </div>

        <section ref="followRequestsSection" class="panel">
          <p class="eyebrow">Followers</p>
          <h3>Pending follow requests</h3>
          <p v-if="requestError" class="form-error">{{ requestError }}</p>
          <p v-if="loadingRequests">Loading requests...</p>
          <div v-else-if="requests.length" class="user-stack">
            <article v-for="request in requests" :key="request.id" class="user-card">
              <div>
                <strong>{{ displayName(request.sender) }}</strong>
                <p>{{ request.sender.aboutMe || "This person wants access to your private posts." }}</p>
              </div>
              <div class="user-card__actions">
                <button
                  type="button"
                  class="button button--ghost"
                  :disabled="requestLoading[request.id]"
                  @click="respondToRequest(request.id, false)"
                >
                  Decline
                </button>
                <button
                  type="button"
                  class="button"
                  :disabled="requestLoading[request.id]"
                  @click="respondToRequest(request.id, true)"
                >
                  Accept
                </button>
              </div>
            </article>
          </div>
          <p v-else>No pending follow requests right now.</p>
        </section>
      </template>

      <section class="panel">
        <div class="feed-header">
          <div>
            <p class="eyebrow">Posts</p>
            <h3>{{ timelineTitle }}</h3>
          </div>
          <p>{{ timelineSummary }}</p>
        </div>

        <div v-if="!canViewPosts" class="profile-empty-state profile-empty-state--framed">
          <h3>This timeline is private</h3>
          <p>{{ accessDescription() }}</p>
          <button
            v-if="!isAuthenticated"
            type="button"
            class="button"
            @click="openLogin"
          >
            Login to follow
          </button>
        </div>

        <div v-else-if="viewedPosts.length" class="profile-gallery">
          <button
            v-for="post in viewedPosts"
            :key="post.id"
            type="button"
            class="profile-gallery__button"
            @click="openPostModal(post)"
          >
            <article class="profile-gallery__tile" :class="{ 'profile-gallery__tile--text': !post.media?.length }">
              <img
                v-if="post.media?.length"
                :src="post.media[0].url"
                :alt="post.title || `${profileName} post`"
                class="profile-gallery__image"
              />

              <div v-else class="profile-gallery__text">
                <strong>{{ post.title || "Untitled post" }}</strong>
                <p>{{ postPreviewText(post) }}</p>
              </div>

              <div class="profile-gallery__badges">
                <span v-if="post.media?.length > 1" class="badge badge--neutral">{{ post.media.length }} photos</span>
                <span class="badge">{{ post.commentsCount || 0 }} comments</span>
              </div>

              <div class="profile-gallery__overlay">
                <strong>{{ post.title || "Untitled post" }}</strong>
                <span>{{ formatRelativeTime(post.createdAt) }}</span>
              </div>
            </article>
          </button>
        </div>

        <div v-else class="profile-empty-state profile-empty-state--framed">
          <h3>{{ emptyPostTitle }}</h3>
          <p>{{ emptyPostDescription }}</p>
        </div>
      </section>
    </template>

    <div
      v-if="selectedPost"
      class="post-modal"
      role="dialog"
      aria-modal="true"
      :aria-label="selectedPost.title || 'Post details'"
      @click.self="closePostModal"
    >
      <div class="post-modal__dialog">
        <section class="post-modal__media-panel">
          <button type="button" class="button button--ghost button--small post-modal__close" @click="closePostModal">
            Close
          </button>

          <div class="post-modal__media-frame" :class="{ 'post-modal__media-frame--text': !hasSelectedPostMedia }">
            <img
              v-if="selectedPostMedia"
              :src="selectedPostMedia.url"
              :alt="selectedPost.title || `${profileName} post`"
              class="post-modal__image"
            />

            <div v-else class="post-modal__text-card">
              <h4>{{ selectedPost.title || "Untitled post" }}</h4>
              <p>{{ selectedPost.body || "This post does not include media, but you can still read its details and comments here." }}</p>
            </div>
          </div>

          <div v-if="selectedPost.media?.length > 1" class="post-modal__controls">
            <button type="button" class="button button--ghost button--small" @click="previousSelectedPostSlide">
              Prev
            </button>

            <div class="post-modal__dots">
              <button
                v-for="(media, index) in selectedPost.media"
                :key="media.id"
                type="button"
                class="post-modal__dot"
                :class="{ 'post-modal__dot--active': index === selectedPostSlide }"
                :aria-label="`Show image ${index + 1}`"
                @click="setSelectedPostSlide(index)"
              ></button>
            </div>

            <button type="button" class="button button--ghost button--small" @click="nextSelectedPostSlide">
              Next
            </button>
          </div>
        </section>

        <aside class="post-modal__sidebar">
          <header class="post-modal__sidebar-header">
            <div class="post-modal__author">
              <div class="user-avatar user-avatar--small">
                <img
                  v-if="viewedProfile?.avatarUrl"
                  :src="viewedProfile.avatarUrl"
                  :alt="`${profileName} profile photo`"
                  class="user-avatar__image"
                />
                <span v-else class="user-avatar__fallback">{{ profileInitials }}</span>
              </div>

              <div class="post-modal__author-copy">
                <strong>{{ profileName }}</strong>
                <span>{{ postTimestamp(selectedPost) }}</span>
              </div>
            </div>

            <div class="post-modal__meta">
              <span class="badge">{{ selectedPost.privacy }}</span>
              <span class="badge badge--neutral">{{ commentCountLabel(selectedPost.commentsCount || 0) }}</span>
            </div>

            <button
              v-if="isOwnProfile"
              type="button"
              class="button button--ghost button--small"
              :disabled="isDeletingSelectedPost"
              @click="deleteSelectedPost"
            >
              {{ isDeletingSelectedPost ? "Deleting..." : "Delete post" }}
            </button>
          </header>

          <section class="post-modal__caption">
            <p v-if="postDeleteError" class="form-error">{{ postDeleteError }}</p>
            <h4>{{ selectedPost.title || "Untitled post" }}</h4>
            <p>{{ selectedPost.body || "This post only contains media for now." }}</p>
          </section>

          <section class="post-modal__comments">
            <div class="post-modal__comments-header">
              <div>
                <strong>Comments</strong>
                <p>{{ commentCountLabel(selectedPost.commentsCount || 0) }}</p>
              </div>

              <button
                v-if="selectedPostCommentsError"
                type="button"
                class="button button--ghost button--small"
                @click="loadPostComments(selectedPost.id, { force: true })"
              >
                Retry
              </button>
            </div>

            <p v-if="selectedPostCommentsError" class="form-error">{{ selectedPostCommentsError }}</p>
            <p v-else-if="isLoadingSelectedPostComments" class="feed-note">Loading comments...</p>

            <div v-else-if="selectedPostComments.length" class="comment-stack post-modal__comment-stack">
              <article v-for="comment in selectedPostComments" :key="comment.id" class="comment-card">
                <header class="comment-card__header">
                  <div class="comment-card__header-main">
                    <strong>{{ displayName(comment.author) }}</strong>
                    <span>{{ commentTimestampLabel(comment) }}</span>
                  </div>
                </header>

                <p class="comment-card__body">{{ comment.body }}</p>

                <div v-if="comment.replies?.length" class="reply-stack">
                  <article
                    v-for="reply in comment.replies"
                    :key="reply.id"
                    class="comment-card comment-card--reply"
                  >
                    <header class="comment-card__header">
                      <div class="comment-card__header-main">
                        <strong>{{ displayName(reply.author) }}</strong>
                        <span>{{ commentTimestampLabel(reply) }}</span>
                      </div>
                    </header>

                    <p class="comment-card__body">{{ reply.body }}</p>
                  </article>
                </div>
              </article>
            </div>

            <p v-else class="feed-note">
              No comments yet. When the conversation starts, it will appear in this panel.
            </p>
          </section>
        </aside>
      </div>
    </div>
  </section>
</template>
