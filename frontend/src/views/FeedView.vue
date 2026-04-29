<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue"
import { RouterLink, useRoute } from "vue-router"

import {
  createComment,
  deletePost as deletePostRequest,
  fetchComments,
  fetchDiscoverUsers,
  fetchFeed,
  fetchMyPosts,
  fetchPost,
  followUser,
  isApiError,
  updateComment,
  updatePost
} from "../services/api"
import { realtimeClient } from "../services/realtime"
import { useAppStore } from "../stores/app"
import { formatRelativeTime } from "../utils/date"

const store = useAppStore()
const route = useRoute()
const props = defineProps({
  scope: {
    type: String,
    default: "all"
  }
})

const isAuthenticated = computed(() => Boolean(store.state.currentUser))
const currentUserId = computed(() => store.state.currentUser?.id || "")
const isMyPostsScope = computed(() => props.scope === "mine")
const showDiscoverPanel = computed(() => !isMyPostsScope.value)
const pageTitle = computed(() => (isMyPostsScope.value ? "My posts" : "Latest posts"))
const guestTitle = computed(() => (isMyPostsScope.value ? "Sign in to unlock your posts" : "Sign in to unlock the feed"))
const guestDescription = computed(() =>
  isMyPostsScope.value
    ? "This page keeps your own published posts in one place, including your comment threads and edit actions."
    : "The personalized feed, follow graph, dedicated create flow, and threaded comments are only available after authentication."
)
const posts = ref([])
const suggestedUsers = ref([])
const requestError = ref("")
const isLoading = ref(false)
const activeSlides = reactive({})
const followLoading = reactive({})
const expandedComments = reactive({})
const commentsByPost = reactive({})
const commentsLoading = reactive({})
const commentsLoaded = reactive({})
const commentErrorByPost = reactive({})
const commentForms = reactive({})
const replyForms = reactive({})
const commentSubmitting = reactive({})
const postEditForms = reactive({})
const postEditSaving = reactive({})
const postEditErrorByPost = reactive({})
const postDeleteLoading = reactive({})
const commentEditForms = reactive({})
const commentEditSaving = reactive({})
const commentEditErrors = reactive({})
const selectedPostId = ref("")
const previousBodyOverflow = ref("")
const removeRealtimeListeners = []
const subscribedPostIds = new Set()
const postsSummary = computed(() => {
  if (isLoading.value) {
    return isMyPostsScope.value ? "Refreshing your posts..." : "Refreshing the feed..."
  }

  return isMyPostsScope.value ? `${posts.value.length} of your posts` : `${posts.value.length} visible posts`
})
const emptyTitle = computed(() => (isMyPostsScope.value ? "You haven't posted yet" : "Your feed is empty for now"))
const emptyDescription = computed(() =>
  isMyPostsScope.value
    ? "Use the `+` action to publish your first post and build up your personal post library."
    : "Use the `+` action to create the first post or follow another public account to start filling this page."
)
const selectedPost = computed(() => posts.value.find((post) => post.id === selectedPostId.value) || null)
const selectedPostComments = computed(() => {
  const postId = selectedPost.value?.id
  return postId ? commentsByPost[postId] || [] : []
})
const isLoadingSelectedPostComments = computed(() => {
  const postId = selectedPost.value?.id
  return postId ? Boolean(commentsLoading[postId]) : false
})
const selectedPostCommentsError = computed(() => {
  const postId = selectedPost.value?.id
  return postId ? commentErrorByPost[postId] || "" : ""
})
const selectedPostMedia = computed(() => {
  if (!selectedPost.value?.media?.length) {
    return null
  }

  return selectedPost.value.media[currentSlide(selectedPost.value)] || selectedPost.value.media[0] || null
})
const hasSelectedPostMedia = computed(() => Boolean(selectedPostMedia.value))

function displayName(user) {
  if (!user) {
    return ""
  }

  return user.nickname || `${user.firstName} ${user.lastName}`.trim() || "Unknown user"
}

function commentCountLabel(count) {
  return Number(count || 0) === 1 ? "1 comment" : `${Number(count || 0)} comments`
}

function clearObject(object) {
  Object.keys(object).forEach((key) => {
    delete object[key]
  })
}

function routePostId() {
  const rawValue = route.query.post
  if (Array.isArray(rawValue)) {
    return String(rawValue[0] || "").trim()
  }

  return typeof rawValue === "string" ? rawValue.trim() : ""
}

function shouldOpenCommentsFromRoute() {
  const rawValue = route.query.comments
  if (Array.isArray(rawValue)) {
    return rawValue.includes("1")
  }

  return String(rawValue || "").trim() === "1"
}

function canEditAuthor(author) {
  return Boolean(author?.id && author.id === currentUserId.value)
}

function profileRoute(user) {
  if (!user) {
    return ""
  }

  if (user.id && user.id === currentUserId.value) {
    return "/profile/me"
  }

  const nickname = String(user.nickname || "").trim()
  return nickname ? `/profile/${encodeURIComponent(nickname)}` : ""
}

function commentTimestampLabel(comment) {
  if (!comment) {
    return ""
  }

  return comment.updatedAt !== comment.createdAt
    ? `Edited ${formatRelativeTime(comment.updatedAt)}`
    : formatRelativeTime(comment.createdAt)
}

function postTimestampLabel(post) {
  if (!post) {
    return ""
  }

  return post.updatedAt !== post.createdAt
    ? `Edited ${formatRelativeTime(post.updatedAt)}`
    : formatRelativeTime(post.createdAt)
}

function postPreviewText(post) {
  const text = String(post?.body || post?.title || "").trim()
  if (!text) {
    return "Open this post to see the media and the comment thread."
  }

  return text.length > 120 ? `${text.slice(0, 117)}...` : text
}

function unsubscribeAllPostRooms() {
  for (const postId of subscribedPostIds) {
    realtimeClient.unsubscribePost(postId)
  }

  subscribedPostIds.clear()
}

function ensureCommentState(postId) {
  if (!commentForms[postId]) {
    commentForms[postId] = { body: "" }
  }

  if (!commentsByPost[postId]) {
    commentsByPost[postId] = []
  }

  if (!(postId in commentsLoading)) {
    commentsLoading[postId] = false
  }

  if (!(postId in commentErrorByPost)) {
    commentErrorByPost[postId] = ""
  }

  if (!(postId in commentsLoaded)) {
    commentsLoaded[postId] = false
  }
}

function ensurePostEditForm(post) {
  if (!postEditForms[post.id]) {
    postEditForms[post.id] = {
      open: false,
      title: "",
      body: "",
      privacy: "public"
    }
  }

  if (!(post.id in postEditSaving)) {
    postEditSaving[post.id] = false
  }

  if (!(post.id in postEditErrorByPost)) {
    postEditErrorByPost[post.id] = ""
  }

  if (!(post.id in postDeleteLoading)) {
    postDeleteLoading[post.id] = false
  }

  return postEditForms[post.id]
}

function syncPostEditForm(post, options = {}) {
  const form = ensurePostEditForm(post)
  if (options.preserveDraft && form.open) {
    return form
  }

  form.title = post.title || ""
  form.body = post.body || ""
  form.privacy = post.privacy || "public"
  form.open = false
  return form
}

function closePostEdit(post) {
  syncPostEditForm(post)
  postEditErrorByPost[post.id] = ""
}

function togglePostEdit(post) {
  const form = ensurePostEditForm(post)
  if (form.open) {
    closePostEdit(post)
    return
  }

  syncPostEditForm(post)
  form.open = true
}

function ensureCommentEditForm(comment) {
  if (!commentEditForms[comment.id]) {
    commentEditForms[comment.id] = {
      open: false,
      body: ""
    }
  }

  if (!(comment.id in commentEditSaving)) {
    commentEditSaving[comment.id] = false
  }

  if (!(comment.id in commentEditErrors)) {
    commentEditErrors[comment.id] = ""
  }

  return commentEditForms[comment.id]
}

function syncCommentEditForm(comment, options = {}) {
  const form = ensureCommentEditForm(comment)
  if (options.preserveDraft && form.open) {
    return form
  }

  form.body = comment.body || ""
  form.open = false
  return form
}

function closeCommentEdit(comment) {
  syncCommentEditForm(comment)
  commentEditErrors[comment.id] = ""
}

function toggleCommentEdit(comment) {
  const form = ensureCommentEditForm(comment)
  if (form.open) {
    closeCommentEdit(comment)
    return
  }

  syncCommentEditForm(comment)
  form.open = true
}

function replyKey(postId, commentId) {
  return `${postId}:${commentId}`
}

function ensureReplyForm(postId, commentId) {
  const key = replyKey(postId, commentId)
  if (!replyForms[key]) {
    replyForms[key] = {
      body: "",
      open: false
    }
  }

  return replyForms[key]
}

function primeCommentThread(postId, comments) {
  for (const comment of comments || []) {
    ensureReplyForm(postId, comment.id)
    syncCommentEditForm(comment, { preserveDraft: true })

    for (const reply of comment.replies || []) {
      syncCommentEditForm(reply, { preserveDraft: true })
    }
  }
}

function applyUpdatedPost(updatedPost) {
  if (!updatedPost?.id) {
    return
  }

  posts.value = posts.value.map((item) =>
    item.id === updatedPost.id
      ? {
          ...item,
          ...updatedPost,
          commentsCount: updatedPost.commentsCount ?? item.commentsCount ?? 0
        }
      : item
  )

  const latestPost = posts.value.find((item) => item.id === updatedPost.id)
  if (latestPost) {
    syncPostEditForm(latestPost, { preserveDraft: true })
  }
}

function removePostFromState(postId) {
  if (!postId) {
    return
  }

  const hasPost = posts.value.some((item) => item.id === postId)
  if (!hasPost) {
    return
  }

  const comments = commentsByPost[postId] || []
  const queue = [...comments]
  while (queue.length) {
    const comment = queue.shift()
    if (!comment) {
      continue
    }

    delete commentEditForms[comment.id]
    delete commentEditSaving[comment.id]
    delete commentEditErrors[comment.id]

    for (const reply of comment.replies || []) {
      queue.push(reply)
    }
  }

  if (expandedComments[postId]) {
    realtimeClient.unsubscribePost(postId)
    subscribedPostIds.delete(postId)
  }

  posts.value = posts.value.filter((item) => item.id !== postId)
  if (selectedPostId.value === postId) {
    closePostModal()
  }

  delete activeSlides[postId]
  delete expandedComments[postId]
  delete commentsByPost[postId]
  delete commentsLoading[postId]
  delete commentsLoaded[postId]
  delete commentErrorByPost[postId]
  delete commentForms[postId]
  delete postEditForms[postId]
  delete postEditSaving[postId]
  delete postEditErrorByPost[postId]
  delete postDeleteLoading[postId]

  for (const key of Object.keys(replyForms)) {
    if (key.startsWith(`${postId}:`)) {
      delete replyForms[key]
    }
  }

  for (const key of Object.keys(commentSubmitting)) {
    if (key === postId || key.startsWith(`${postId}:`)) {
      delete commentSubmitting[key]
    }
  }
}

function applyUpdatedComment(postId, updatedComment) {
  if (!postId || !updatedComment?.id) {
    return false
  }

  ensureCommentState(postId)

  let updated = false
  commentsByPost[postId] = commentsByPost[postId].map((item) => {
    if (item.id === updatedComment.id) {
      updated = true
      return {
        ...item,
        ...updatedComment,
        replies: item.replies || []
      }
    }

    let replyWasUpdated = false
    const replies = (item.replies || []).map((reply) => {
      if (reply.id !== updatedComment.id) {
        return reply
      }

      replyWasUpdated = true
      updated = true
      return {
        ...reply,
        ...updatedComment
      }
    })

    if (!replyWasUpdated) {
      return item
    }

    return {
      ...item,
      replies
    }
  })

  if (updated) {
    syncCommentEditForm(updatedComment, { preserveDraft: true })
  }

  return updated
}

async function loadFeedData() {
  if (!isAuthenticated.value) {
    unsubscribeAllPostRooms()
    posts.value = []
    suggestedUsers.value = []
    selectedPostId.value = ""
    clearObject(activeSlides)
    clearObject(expandedComments)
    clearObject(commentsByPost)
    clearObject(commentsLoading)
    clearObject(commentsLoaded)
    clearObject(commentErrorByPost)
    clearObject(commentForms)
    clearObject(replyForms)
    clearObject(commentSubmitting)
    clearObject(postEditForms)
    clearObject(postEditSaving)
    clearObject(postEditErrorByPost)
    clearObject(postDeleteLoading)
    clearObject(commentEditForms)
    clearObject(commentEditSaving)
    clearObject(commentEditErrors)
    return
  }

  isLoading.value = true
  requestError.value = ""

  try {
    const [feedPosts, discoverUsers] = await Promise.all([
      isMyPostsScope.value ? fetchMyPosts() : fetchFeed(),
      showDiscoverPanel.value ? fetchDiscoverUsers() : Promise.resolve([])
    ])

    unsubscribeAllPostRooms()
    posts.value = feedPosts
    suggestedUsers.value = discoverUsers

    clearObject(activeSlides)
    clearObject(expandedComments)
    clearObject(commentsByPost)
    clearObject(commentsLoading)
    clearObject(commentsLoaded)
    clearObject(commentErrorByPost)
    clearObject(commentForms)
    clearObject(replyForms)
    clearObject(commentSubmitting)
    clearObject(postEditForms)
    clearObject(postEditSaving)
    clearObject(postEditErrorByPost)
    clearObject(postDeleteLoading)
    clearObject(commentEditForms)
    clearObject(commentEditSaving)
    clearObject(commentEditErrors)

    for (const post of feedPosts) {
      activeSlides[post.id] = 0
      ensureCommentState(post.id)
      syncPostEditForm(post)
    }

    await focusPostFromRoute()
  } catch (error) {
    requestError.value = error instanceof Error
      ? error.message
      : isMyPostsScope.value
        ? "Could not load your posts."
        : "Could not load the feed."
  } finally {
    isLoading.value = false
  }
}

async function loadComments(postId) {
  ensureCommentState(postId)
  commentsLoading[postId] = true
  commentErrorByPost[postId] = ""

  try {
    commentsByPost[postId] = await fetchComments(postId)
    primeCommentThread(postId, commentsByPost[postId])
    commentsLoaded[postId] = true
  } catch (error) {
    commentsLoaded[postId] = false
    commentErrorByPost[postId] = error instanceof Error ? error.message : "Could not load comments."
  } finally {
    commentsLoading[postId] = false
  }
}

async function toggleComments(post) {
  ensureCommentState(post.id)
  expandedComments[post.id] = !expandedComments[post.id]

  if (expandedComments[post.id]) {
    realtimeClient.subscribePost(post.id)
    subscribedPostIds.add(post.id)
  } else {
    realtimeClient.unsubscribePost(post.id)
    subscribedPostIds.delete(post.id)
  }

  if (expandedComments[post.id] && !commentsLoaded[post.id] && post.commentsCount >= 0) {
    await loadComments(post.id)
  }
}

function toggleReplyForm(postId, commentId) {
  const form = ensureReplyForm(postId, commentId)
  form.open = !form.open
  if (!form.open) {
    form.body = ""
  }
}

async function submitPostEdit(post) {
  const form = ensurePostEditForm(post)
  postEditSaving[post.id] = true
  postEditErrorByPost[post.id] = ""

  try {
    const updatedPost = await updatePost(post.id, {
      title: form.title,
      body: form.body,
      privacy: form.privacy
    })

    applyUpdatedPost(updatedPost)
    syncPostEditForm(updatedPost)
  } catch (error) {
    if (isApiError(error)) {
      postEditErrorByPost[post.id] =
        error.payload?.fields?.title ||
        error.payload?.fields?.body ||
        error.payload?.fields?.privacy ||
        error.message
    } else {
      postEditErrorByPost[post.id] = "Could not save the post right now."
    }
  } finally {
    postEditSaving[post.id] = false
  }
}

async function deletePost(post) {
  if (!post?.id || !canEditAuthor(post.author)) {
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

  postDeleteLoading[post.id] = true
  postEditErrorByPost[post.id] = ""
  requestError.value = ""

  try {
    await deletePostRequest(post.id)
    removePostFromState(post.id)
  } catch (error) {
    postEditErrorByPost[post.id] = error instanceof Error ? error.message : "Could not delete the post right now."
  } finally {
    if (post.id in postDeleteLoading) {
      postDeleteLoading[post.id] = false
    }
  }
}

async function submitComment(post, parentComment = null) {
  ensureCommentState(post.id)

  const key = parentComment ? replyKey(post.id, parentComment.id) : post.id
  const form = parentComment ? ensureReplyForm(post.id, parentComment.id) : commentForms[post.id]
  const body = (form.body || "").trim()

  commentSubmitting[key] = true
  commentErrorByPost[post.id] = ""

  try {
    const comment = await createComment(post.id, {
      body,
      parentCommentId: parentComment?.id || ""
    })

    const inserted = insertLiveComment(post.id, comment)

    if (parentComment) {
      form.body = ""
      form.open = false
    } else {
      form.body = ""
    }

    if (inserted) {
      posts.value = posts.value.map((item) =>
        item.id === post.id
          ? { ...item, commentsCount: (item.commentsCount || 0) + 1 }
          : item
      )
    }
    expandedComments[post.id] = true
  } catch (error) {
    if (isApiError(error)) {
      commentErrorByPost[post.id] =
        error.payload?.fields?.body ||
        error.payload?.fields?.parentCommentId ||
        error.message
    } else {
      commentErrorByPost[post.id] = "Could not publish the comment right now."
    }
  } finally {
    commentSubmitting[key] = false
  }
}

async function submitCommentEdit(postId, comment) {
  const form = ensureCommentEditForm(comment)
  commentEditSaving[comment.id] = true
  commentEditErrors[comment.id] = ""

  try {
    const updatedComment = await updateComment(postId, comment.id, {
      body: form.body
    })

    applyUpdatedComment(postId, updatedComment)
    syncCommentEditForm(updatedComment)
  } catch (error) {
    if (isApiError(error)) {
      commentEditErrors[comment.id] =
        error.payload?.fields?.body ||
        error.message
    } else {
      commentEditErrors[comment.id] = "Could not save the comment right now."
    }
  } finally {
    commentEditSaving[comment.id] = false
  }
}

async function handleFollow(userId) {
  followLoading[userId] = true

  try {
    const result = await followUser(userId)
    suggestedUsers.value = suggestedUsers.value.map((user) =>
      user.id === userId ? { ...user, relationshipStatus: result.status } : user
    )

    if (result.status === "following") {
      await loadFeedData()
    }
  } catch (error) {
    requestError.value = error instanceof Error ? error.message : "Could not update follow status."
  } finally {
    followLoading[userId] = false
  }
}

async function focusPostFromRoute() {
  const postId = routePostId()
  if (!postId || !isAuthenticated.value) {
    return
  }

  const targetPost = await ensureRoutePostLoaded(postId)
  if (!targetPost) {
    return
  }

  if (isMyPostsScope.value) {
    openPostModal(targetPost)
    return
  }

  if (shouldOpenCommentsFromRoute()) {
    ensureCommentState(targetPost.id)

    if (!expandedComments[targetPost.id]) {
      await toggleComments(targetPost)
    } else if (!commentsLoaded[targetPost.id]) {
      await loadComments(targetPost.id)
    }
  }

  await nextTick()
  document.getElementById(`post-${targetPost.id}`)?.scrollIntoView({
    behavior: "smooth",
    block: "start"
  })
}

async function ensureRoutePostLoaded(postId) {
  const existingPost = posts.value.find((post) => post.id === postId)
  if (existingPost) {
    return existingPost
  }

  try {
    const loadedPost = await fetchPost(postId)
    if (!loadedPost) {
      return null
    }

    if (isMyPostsScope.value && loadedPost.author?.id !== currentUserId.value) {
      return null
    }

    if (!posts.value.some((post) => post.id === loadedPost.id)) {
      posts.value = [loadedPost, ...posts.value]
      activeSlides[loadedPost.id] = 0
      ensureCommentState(loadedPost.id)
      syncPostEditForm(loadedPost)
    }

    return loadedPost
  } catch (error) {
    if (!isApiError(error, 403) && !isApiError(error, 404)) {
      requestError.value = error instanceof Error ? error.message : "Could not open that post."
    }

    return null
  }
}

function currentSlide(post) {
  return activeSlides[post.id] || 0
}

function nextSlide(post) {
  if (!post.media?.length) {
    return
  }

  activeSlides[post.id] = (currentSlide(post) + 1) % post.media.length
}

function previousSlide(post) {
  if (!post.media?.length) {
    return
  }

  activeSlides[post.id] = (currentSlide(post) - 1 + post.media.length) % post.media.length
}

function setSlide(post, index) {
  activeSlides[post.id] = index
}

async function openPostModal(post, mediaIndex = 0) {
  if (!post) {
    return
  }

  const mediaCount = post.media?.length || 0
  const normalizedIndex = mediaCount ? Math.max(0, Math.min(mediaIndex, mediaCount - 1)) : 0

  ensureCommentState(post.id)
  selectedPostId.value = post.id
  setSlide(post, normalizedIndex)

  if (!commentsLoaded[post.id] && !commentsLoading[post.id]) {
    await loadComments(post.id)
  }
}

function closePostModal() {
  selectedPostId.value = ""
}

function handleWindowKeydown(event) {
  if (!selectedPost.value) {
    return
  }

  if (event.key === "Escape") {
    closePostModal()
  } else if (event.key === "ArrowLeft") {
    previousSlide(selectedPost.value)
  } else if (event.key === "ArrowRight") {
    nextSlide(selectedPost.value)
  }
}

function insertLiveComment(postId, comment) {
  ensureCommentState(postId)

  if (commentsByPost[postId].some((item) => item.id === comment.id)) {
    return false
  }

  if (!comment.parentCommentId) {
    commentsByPost[postId] = [...commentsByPost[postId], comment]
    ensureReplyForm(postId, comment.id)
    syncCommentEditForm(comment)
    return true
  }

  let inserted = false
  commentsByPost[postId] = commentsByPost[postId].map((item) => {
    if (item.id !== comment.parentCommentId) {
      return item
    }

    if ((item.replies || []).some((reply) => reply.id === comment.id)) {
      return item
    }

    inserted = true
    syncCommentEditForm(comment)
    return {
      ...item,
      replies: [...(item.replies || []), comment]
    }
  })

  return inserted
}

function handleLiveCommentEvent(event) {
  const postId = event.payload?.postId
  const comment = event.payload?.comment

  if (!postId || !comment) {
    return
  }

  if (!insertLiveComment(postId, comment)) {
    return
  }

  posts.value = posts.value.map((item) =>
    item.id === postId
      ? { ...item, commentsCount: (item.commentsCount || 0) + 1 }
      : item
  )
}

function handleLivePostUpdatedEvent(event) {
  const updatedPost = event.payload?.post
  if (!updatedPost) {
    return
  }

  applyUpdatedPost(updatedPost)
}

function handleLivePostDeletedEvent(event) {
  const postID = event.payload?.postId
  if (!postID) {
    return
  }

  removePostFromState(postID)
}

function handleLiveCommentUpdatedEvent(event) {
  const postId = event.payload?.postId
  const comment = event.payload?.comment

  if (!postId || !comment) {
    return
  }

  applyUpdatedComment(postId, comment)
}

removeRealtimeListeners.push(
  realtimeClient.on("comment.created", handleLiveCommentEvent),
  realtimeClient.on("comment.reply.created", handleLiveCommentEvent),
  realtimeClient.on("post.updated", handleLivePostUpdatedEvent),
  realtimeClient.on("post.deleted", handleLivePostDeletedEvent),
  realtimeClient.on("comment.updated", handleLiveCommentUpdatedEvent)
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

  unsubscribeAllPostRooms()
  removeRealtimeListeners.splice(0).forEach((dispose) => dispose())
})

watch(
  () => store.state.currentUser?.id,
  () => {
    void loadFeedData()
  },
  { immediate: true }
)

watch(
  () => [route.query.post, route.query.comments],
  () => {
    void focusPostFromRoute()
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
</script>

<template>
  <section class="page">
    <p v-if="requestError" class="form-error">{{ requestError }}</p>

    <div v-if="!isAuthenticated" class="panel">
      <h3>{{ guestTitle }}</h3>
      <p>{{ guestDescription }}</p>
    </div>

    <template v-else>
      <div class="feed-layout" :class="{ 'feed-layout--single': !showDiscoverPanel }">
        <section class="page">
          <div class="feed-header">
            <h3>{{ pageTitle }}</h3>
            <p>{{ postsSummary }}</p>
          </div>

          <template v-if="isMyPostsScope">
            <div v-if="posts.length" class="profile-gallery">
              <button
                v-for="post in posts"
                :id="`post-${post.id}`"
                :key="post.id"
                type="button"
                class="profile-gallery__button"
                @click="openPostModal(post)"
              >
                <article class="profile-gallery__tile" :class="{ 'profile-gallery__tile--text': !post.media?.length }">
                  <img
                    v-if="post.media?.length"
                    :src="post.media[0].url"
                    :alt="post.title || 'My post'"
                    class="profile-gallery__image"
                  />

                  <div v-else class="profile-gallery__text">
                    <strong>{{ post.title || "Untitled post" }}</strong>
                    <p>{{ postPreviewText(post) }}</p>
                  </div>

                  <div class="profile-gallery__badges">
                    <span v-if="post.media?.length > 1" class="badge badge--neutral">{{ post.media.length }} photos</span>
                    <span class="badge">{{ post.privacy }}</span>
                    <span class="badge badge--soft">{{ commentCountLabel(post.commentsCount || 0) }}</span>
                  </div>

                  <div class="profile-gallery__overlay">
                    <strong>{{ post.title || "Untitled post" }}</strong>
                    <span>{{ formatRelativeTime(post.createdAt) }}</span>
                  </div>
                </article>
              </button>
            </div>

            <div v-else-if="!posts.length && !isLoading" class="panel">
              <h3>{{ emptyTitle }}</h3>
              <p>{{ emptyDescription }}</p>
            </div>
          </template>

          <template v-else>
            <article v-for="post in posts" :id="`post-${post.id}`" :key="post.id" class="panel post-card">
              <header class="post-card__header">
                <div class="post-card__header-main">
                  <p class="eyebrow">Post</p>
                  <template v-if="postEditForms[post.id]?.open">
                    <h3>Edit post</h3>
                    <p class="feed-note">Update the title, caption, or visibility. Images stay as they are for now.</p>
                  </template>
                  <template v-else>
                    <h3>{{ post.title }}</h3>
                    <p class="post-card__meta">
                      <RouterLink
                        v-if="profileRoute(post.author)"
                        :to="profileRoute(post.author)"
                        class="profile-inline-link"
                      >
                        <strong>{{ displayName(post.author) }}</strong>
                      </RouterLink>
                      <strong v-else>{{ displayName(post.author) }}</strong>
                      <span>{{ formatRelativeTime(post.createdAt) }}</span>
                      <span class="badge">{{ post.privacy }}</span>
                      <span class="badge badge--soft">{{ post.author.profileVisibility }}</span>
                      <span class="badge badge--neutral">{{ commentCountLabel(post.commentsCount || 0) }}</span>
                    </p>
                  </template>
                </div>
                <div class="post-card__timestamps">
                  <span>{{ formatRelativeTime(post.createdAt) }}</span>
                  <div v-if="canEditAuthor(post.author)" class="editor-actions">
                    <button
                      type="button"
                      class="button button--ghost button--small"
                      :disabled="postDeleteLoading[post.id]"
                      @click="togglePostEdit(post)"
                    >
                      {{ postEditForms[post.id]?.open ? "Cancel edit" : "Edit" }}
                    </button>
                    <button
                      type="button"
                      class="button button--ghost button--small"
                      :disabled="postDeleteLoading[post.id]"
                      @click="deletePost(post)"
                    >
                      {{ postDeleteLoading[post.id] ? "Deleting..." : "Delete" }}
                    </button>
                  </div>
                  <span v-if="post.updatedAt !== post.createdAt">Edited {{ formatRelativeTime(post.updatedAt) }}</span>
                </div>
              </header>

              <p v-if="postEditErrorByPost[post.id] && !postEditForms[post.id]?.open" class="form-error">
                {{ postEditErrorByPost[post.id] }}
              </p>

              <form v-if="postEditForms[post.id]?.open" class="post-editor" @submit.prevent="submitPostEdit(post)">
                <input
                  v-model.trim="postEditForms[post.id].title"
                  type="text"
                  maxlength="120"
                  placeholder="Post title"
                />
                <textarea
                  v-model.trim="postEditForms[post.id].body"
                  rows="4"
                  maxlength="3000"
                  placeholder="Post caption"
                ></textarea>
                <div class="post-editor__row">
                  <label class="post-editor__field">
                    <span>Visibility</span>
                    <select v-model="postEditForms[post.id].privacy">
                      <option value="public">Public</option>
                      <option value="followers">Followers</option>
                    </select>
                  </label>
                </div>
                <div class="post-editor__actions">
                  <p class="feed-note">Only the owner of the post can save these changes.</p>
                  <div class="editor-actions">
                    <button type="button" class="button button--ghost button--small" @click="closePostEdit(post)">
                      Cancel
                    </button>
                    <button type="submit" class="button button--small" :disabled="postEditSaving[post.id]">
                      {{ postEditSaving[post.id] ? "Saving..." : "Save post" }}
                    </button>
                  </div>
                </div>
                <p v-if="postEditErrorByPost[post.id]" class="form-error">{{ postEditErrorByPost[post.id] }}</p>
              </form>

              <p v-else class="post-card__body">{{ post.body }}</p>

              <div v-if="post.media?.length" class="carousel">
                <div class="carousel__frame">
                  <img
                    :src="post.media[currentSlide(post)].url"
                    :alt="`${post.title} image ${currentSlide(post) + 1}`"
                    class="carousel__image"
                  />
                </div>

                <div v-if="post.media.length > 1" class="carousel__controls">
                  <button type="button" class="button button--ghost" @click="previousSlide(post)">
                    Prev
                  </button>
                  <div class="carousel__dots">
                    <button
                      v-for="(media, index) in post.media"
                      :key="media.id"
                      type="button"
                      class="carousel__dot"
                      :class="{ 'carousel__dot--active': index === currentSlide(post) }"
                      @click="setSlide(post, index)"
                    ></button>
                  </div>
                  <button type="button" class="button button--ghost" @click="nextSlide(post)">
                    Next
                  </button>
                </div>
              </div>

              <section class="post-comments">
                <div class="post-comments__header">
                  <div>
                    <strong>Comments</strong>
                    <p>{{ commentCountLabel(post.commentsCount || 0) }}</p>
                  </div>
                  <button type="button" class="button button--ghost button--small" @click="toggleComments(post)">
                    {{ expandedComments[post.id] ? "Hide thread" : "Open thread" }}
                  </button>
                </div>

                <div v-if="expandedComments[post.id]" class="comment-thread">
                  <form class="comment-composer" @submit.prevent="submitComment(post)">
                    <textarea
                      v-model.trim="commentForms[post.id].body"
                      rows="2"
                      placeholder="Add a comment to this post"
                    ></textarea>
                    <div class="comment-composer__actions">
                      <p class="feed-note">Replies are limited to two levels for now.</p>
                      <button
                        type="submit"
                        class="button button--small"
                        :disabled="commentSubmitting[post.id]"
                      >
                        {{ commentSubmitting[post.id] ? "Posting..." : "Comment" }}
                      </button>
                    </div>
                  </form>

                  <p v-if="commentErrorByPost[post.id]" class="form-error">{{ commentErrorByPost[post.id] }}</p>
                  <p v-if="commentsLoading[post.id]" class="feed-note">Loading comments...</p>

                  <div v-else-if="commentsByPost[post.id]?.length" class="comment-stack">
                    <article v-for="comment in commentsByPost[post.id]" :key="comment.id" class="comment-card">
                      <header class="comment-card__header">
                        <div class="comment-card__header-main">
                          <strong>{{ displayName(comment.author) }}</strong>
                          <span>{{ commentTimestampLabel(comment) }}</span>
                        </div>
                        <button
                          v-if="canEditAuthor(comment.author)"
                          type="button"
                          class="button button--ghost button--small"
                          @click="toggleCommentEdit(comment)"
                        >
                          {{ commentEditForms[comment.id]?.open ? "Cancel edit" : "Edit" }}
                        </button>
                      </header>

                      <form
                        v-if="commentEditForms[comment.id]?.open"
                        class="comment-composer comment-composer--edit"
                        @submit.prevent="submitCommentEdit(post.id, comment)"
                      >
                        <textarea
                          v-model.trim="commentEditForms[comment.id].body"
                          rows="3"
                          maxlength="1000"
                          placeholder="Update your comment"
                        ></textarea>
                        <div class="comment-composer__actions">
                          <p class="feed-note">Your place in the thread stays the same after editing.</p>
                          <div class="editor-actions">
                            <button
                              type="button"
                              class="button button--ghost button--small"
                              @click="closeCommentEdit(comment)"
                            >
                              Cancel
                            </button>
                            <button
                              type="submit"
                              class="button button--small"
                              :disabled="commentEditSaving[comment.id]"
                            >
                              {{ commentEditSaving[comment.id] ? "Saving..." : "Save" }}
                            </button>
                          </div>
                        </div>
                        <p v-if="commentEditErrors[comment.id]" class="form-error">{{ commentEditErrors[comment.id] }}</p>
                      </form>

                      <p v-else class="comment-card__body">{{ comment.body }}</p>

                      <div class="comment-card__actions">
                        <div class="editor-actions">
                          <button
                            type="button"
                            class="button button--ghost button--small"
                            @click="toggleReplyForm(post.id, comment.id)"
                          >
                            {{
                              replyForms[replyKey(post.id, comment.id)]?.open
                                ? "Cancel reply"
                                : "Reply"
                            }}
                          </button>
                        </div>
                        <span class="feed-note">
                          {{ comment.replies?.length ? `${comment.replies.length} replies` : "No replies yet" }}
                        </span>
                      </div>

                      <form
                        v-if="replyForms[replyKey(post.id, comment.id)]?.open"
                        class="comment-composer comment-composer--reply"
                        @submit.prevent="submitComment(post, comment)"
                      >
                        <textarea
                          v-model.trim="replyForms[replyKey(post.id, comment.id)].body"
                          rows="2"
                          placeholder="Reply to this comment"
                        ></textarea>
                        <div class="comment-composer__actions">
                          <p class="feed-note">Second level only in the current UI.</p>
                          <button
                            type="submit"
                            class="button button--small"
                            :disabled="commentSubmitting[replyKey(post.id, comment.id)]"
                          >
                            {{
                              commentSubmitting[replyKey(post.id, comment.id)]
                                ? "Posting..."
                                : "Reply"
                            }}
                          </button>
                        </div>
                      </form>

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
                            <button
                              v-if="canEditAuthor(reply.author)"
                              type="button"
                              class="button button--ghost button--small"
                              @click="toggleCommentEdit(reply)"
                            >
                              {{ commentEditForms[reply.id]?.open ? "Cancel edit" : "Edit" }}
                            </button>
                          </header>

                          <form
                            v-if="commentEditForms[reply.id]?.open"
                            class="comment-composer comment-composer--edit"
                            @submit.prevent="submitCommentEdit(post.id, reply)"
                          >
                            <textarea
                              v-model.trim="commentEditForms[reply.id].body"
                              rows="3"
                              maxlength="1000"
                              placeholder="Update your reply"
                            ></textarea>
                            <div class="comment-composer__actions">
                              <p class="feed-note">Reply editing is only available to its author.</p>
                              <div class="editor-actions">
                                <button
                                  type="button"
                                  class="button button--ghost button--small"
                                  @click="closeCommentEdit(reply)"
                                >
                                  Cancel
                                </button>
                                <button
                                  type="submit"
                                  class="button button--small"
                                  :disabled="commentEditSaving[reply.id]"
                                >
                                  {{ commentEditSaving[reply.id] ? "Saving..." : "Save" }}
                                </button>
                              </div>
                            </div>
                            <p v-if="commentEditErrors[reply.id]" class="form-error">{{ commentEditErrors[reply.id] }}</p>
                          </form>

                          <p v-else class="comment-card__body">{{ reply.body }}</p>
                        </article>
                      </div>
                    </article>
                  </div>

                  <p v-else class="feed-note">
                    No comments yet. Start the thread with the first one.
                  </p>
                </div>
              </section>
            </article>

            <div v-if="!posts.length && !isLoading" class="panel">
              <h3>{{ emptyTitle }}</h3>
              <p>{{ emptyDescription }}</p>
            </div>
          </template>
        </section>

        <aside v-if="showDiscoverPanel" class="feed-side">
          <section class="panel">
            <p class="eyebrow">People</p>
            <h3>Discover accounts</h3>
            <div v-if="suggestedUsers.length" class="user-stack">
              <article v-for="user in suggestedUsers" :key="user.id" class="user-card">
                <div>
                  <RouterLink
                    v-if="profileRoute(user)"
                    :to="profileRoute(user)"
                    class="profile-inline-link profile-inline-link--name"
                  >
                    <strong>{{ displayName(user) }}</strong>
                  </RouterLink>
                  <strong v-else>{{ displayName(user) }}</strong>
                  <p>{{ user.aboutMe || "Fresh account ready to connect." }}</p>
                  <span class="badge">{{ user.profileVisibility }}</span>
                </div>
                <button
                  type="button"
                  class="button button--ghost"
                  :disabled="user.relationshipStatus !== 'not_following' || followLoading[user.id]"
                  @click="handleFollow(user.id)"
                >
                  {{
                    user.relationshipStatus === "following"
                      ? "Following"
                      : user.relationshipStatus === "requested"
                        ? "Requested"
                        : followLoading[user.id]
                          ? "Saving..."
                          : "Follow"
                  }}
                </button>
              </article>
            </div>
            <p v-else>No other accounts yet. Register a second user to test follow flows.</p>
          </section>
        </aside>
      </div>

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
                :alt="selectedPost.title || 'My post'"
                class="post-modal__image"
              />

              <div v-else class="post-modal__text-card">
                <h4>{{ selectedPost.title || "Untitled post" }}</h4>
                <p>{{ selectedPost.body || "This post does not include media, but you can still read the caption and comments here." }}</p>
              </div>
            </div>

            <div v-if="selectedPost.media?.length > 1" class="post-modal__controls">
              <button type="button" class="button button--ghost button--small" @click="previousSlide(selectedPost)">
                Prev
              </button>

              <div class="post-modal__dots">
                <button
                  v-for="(media, index) in selectedPost.media"
                  :key="media.id"
                  type="button"
                  class="post-modal__dot"
                  :class="{ 'post-modal__dot--active': index === currentSlide(selectedPost) }"
                  :aria-label="`Show image ${index + 1}`"
                  @click="setSlide(selectedPost, index)"
                ></button>
              </div>

              <button type="button" class="button button--ghost button--small" @click="nextSlide(selectedPost)">
                Next
              </button>
            </div>
          </section>

          <aside class="post-modal__sidebar">
            <header class="post-modal__sidebar-header">
              <div class="post-modal__author">
                <div class="user-avatar user-avatar--small">
                  <img
                    v-if="selectedPost.author?.avatarUrl"
                    :src="selectedPost.author.avatarUrl"
                    :alt="`${displayName(selectedPost.author)} profile photo`"
                    class="user-avatar__image"
                  />
                  <span v-else class="user-avatar__fallback">{{ displayName(selectedPost.author).slice(0, 1).toUpperCase() || "N" }}</span>
                </div>

                <div class="post-modal__author-copy">
                  <strong>{{ displayName(selectedPost.author) }}</strong>
                  <span>{{ postTimestampLabel(selectedPost) }}</span>
                </div>
              </div>

              <div class="post-modal__meta">
                <span class="badge">{{ selectedPost.privacy }}</span>
                <span class="badge badge--neutral">{{ commentCountLabel(selectedPost.commentsCount || 0) }}</span>
              </div>

              <button
                v-if="canEditAuthor(selectedPost.author)"
                type="button"
                class="button button--ghost button--small"
                :disabled="postDeleteLoading[selectedPost.id]"
                @click="togglePostEdit(selectedPost)"
              >
                {{ postEditForms[selectedPost.id]?.open ? "Cancel edit" : "Edit post" }}
              </button>
              <button
                v-if="canEditAuthor(selectedPost.author)"
                type="button"
                class="button button--ghost button--small"
                :disabled="postDeleteLoading[selectedPost.id]"
                @click="deletePost(selectedPost)"
              >
                {{ postDeleteLoading[selectedPost.id] ? "Deleting..." : "Delete post" }}
              </button>
            </header>

            <section class="post-modal__caption">
              <p v-if="postEditErrorByPost[selectedPost.id] && !postEditForms[selectedPost.id]?.open" class="form-error">
                {{ postEditErrorByPost[selectedPost.id] }}
              </p>
              <form
                v-if="postEditForms[selectedPost.id]?.open"
                class="post-editor"
                @submit.prevent="submitPostEdit(selectedPost)"
              >
                <input
                  v-model.trim="postEditForms[selectedPost.id].title"
                  type="text"
                  maxlength="120"
                  placeholder="Post title"
                />
                <textarea
                  v-model.trim="postEditForms[selectedPost.id].body"
                  rows="4"
                  maxlength="3000"
                  placeholder="Post caption"
                ></textarea>
                <div class="post-editor__row">
                  <label class="post-editor__field">
                    <span>Visibility</span>
                    <select v-model="postEditForms[selectedPost.id].privacy">
                      <option value="public">Public</option>
                      <option value="followers">Followers</option>
                    </select>
                  </label>
                </div>
                <div class="post-editor__actions">
                  <p class="feed-note">Only the owner of the post can save these changes.</p>
                  <div class="editor-actions">
                    <button
                      type="button"
                      class="button button--ghost button--small"
                      @click="closePostEdit(selectedPost)"
                    >
                      Cancel
                    </button>
                    <button type="submit" class="button button--small" :disabled="postEditSaving[selectedPost.id]">
                      {{ postEditSaving[selectedPost.id] ? "Saving..." : "Save post" }}
                    </button>
                  </div>
                </div>
                <p v-if="postEditErrorByPost[selectedPost.id]" class="form-error">{{ postEditErrorByPost[selectedPost.id] }}</p>
              </form>

              <template v-else>
                <h4>{{ selectedPost.title || "Untitled post" }}</h4>
                <p>{{ selectedPost.body || "This post only contains media for now." }}</p>
              </template>
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
                  @click="loadComments(selectedPost.id)"
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
    </template>
  </section>
</template>
