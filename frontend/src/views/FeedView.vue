<script setup>
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from "vue"
import { useRoute } from "vue-router"

import {
  createComment,
  fetchComments,
  fetchDiscoverUsers,
  fetchFeed,
  fetchMyPosts,
  followUser,
  isApiError,
  updateComment,
  updatePost
} from "../services/api"
import { realtimeClient } from "../services/realtime"
import { useAppStore } from "../stores/app"
import { formatDate as formatAppDate } from "../utils/date"

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
const commentErrorByPost = reactive({})
const commentForms = reactive({})
const replyForms = reactive({})
const commentSubmitting = reactive({})
const postEditForms = reactive({})
const postEditSaving = reactive({})
const postEditErrorByPost = reactive({})
const commentEditForms = reactive({})
const commentEditSaving = reactive({})
const commentEditErrors = reactive({})
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

function displayName(user) {
  if (!user) {
    return ""
  }

  return user.nickname || `${user.firstName} ${user.lastName}`.trim() || "Unknown user"
}

function formatDate(value) {
  return formatAppDate(value)
}

function commentCountLabel(count) {
  return count === 1 ? "1 comment" : `${count} comments`
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

function commentTimestampLabel(comment) {
  if (!comment) {
    return ""
  }

  return comment.updatedAt !== comment.createdAt
    ? `Edited ${formatDate(comment.updatedAt)}`
    : formatDate(comment.createdAt)
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
    clearObject(activeSlides)
    clearObject(expandedComments)
    clearObject(commentsByPost)
    clearObject(commentsLoading)
    clearObject(commentErrorByPost)
    clearObject(commentForms)
    clearObject(replyForms)
    clearObject(commentSubmitting)
    clearObject(postEditForms)
    clearObject(postEditSaving)
    clearObject(postEditErrorByPost)
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
    clearObject(commentErrorByPost)
    clearObject(commentForms)
    clearObject(replyForms)
    clearObject(commentSubmitting)
    clearObject(postEditForms)
    clearObject(postEditSaving)
    clearObject(postEditErrorByPost)
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
  } catch (error) {
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

  if (expandedComments[post.id] && !commentsByPost[post.id].length && post.commentsCount >= 0) {
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

  const targetPost = posts.value.find((post) => post.id === postId)
  if (!targetPost) {
    return
  }

  if (shouldOpenCommentsFromRoute()) {
    ensureCommentState(targetPost.id)

    if (!expandedComments[targetPost.id]) {
      await toggleComments(targetPost)
    } else if (!commentsByPost[targetPost.id]?.length) {
      await loadComments(targetPost.id)
    }
  }

  await nextTick()
  document.getElementById(`post-${targetPost.id}`)?.scrollIntoView({
    behavior: "smooth",
    block: "start"
  })
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
  realtimeClient.on("comment.updated", handleLiveCommentUpdatedEvent)
)

onBeforeUnmount(() => {
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
                    <strong>{{ displayName(post.author) }}</strong>
                    <span>{{ formatDate(post.createdAt) }}</span>
                    <span class="badge">{{ post.privacy }}</span>
                    <span class="badge badge--soft">{{ post.author.profileVisibility }}</span>
                    <span class="badge badge--neutral">{{ commentCountLabel(post.commentsCount || 0) }}</span>
                  </p>
                </template>
              </div>
              <div class="post-card__timestamps">
                <span>Created {{ formatDate(post.createdAt) }}</span>
                <button
                  v-if="canEditAuthor(post.author)"
                  type="button"
                  class="button button--ghost button--small"
                  @click="togglePostEdit(post)"
                >
                  {{ postEditForms[post.id]?.open ? "Cancel edit" : "Edit" }}
                </button>
                <span v-if="post.updatedAt !== post.createdAt">Updated {{ formatDate(post.updatedAt) }}</span>
              </div>
            </header>

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
        </section>

        <aside v-if="showDiscoverPanel" class="feed-side">
          <section class="panel">
            <p class="eyebrow">People</p>
            <h3>Discover accounts</h3>
            <div v-if="suggestedUsers.length" class="user-stack">
              <article v-for="user in suggestedUsers" :key="user.id" class="user-card">
                <div>
                  <strong>{{ displayName(user) }}</strong>
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
    </template>
  </section>
</template>
