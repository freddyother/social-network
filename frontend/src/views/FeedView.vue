<script setup>
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from "vue"
import { useRoute } from "vue-router"

import {
  createComment,
  fetchComments,
  fetchDiscoverUsers,
  fetchFeed,
  followUser,
  isApiError
} from "../services/api"
import { realtimeClient } from "../services/realtime"
import { useAppStore } from "../stores/app"
import { formatDateTime } from "../utils/date"

const store = useAppStore()
const route = useRoute()

const isAuthenticated = computed(() => Boolean(store.state.currentUser))
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
const removeRealtimeListeners = []
const subscribedPostIds = new Set()

function displayName(user) {
  if (!user) {
    return ""
  }

  return user.nickname || `${user.firstName} ${user.lastName}`.trim() || "Unknown user"
}

function formatDate(value) {
  return formatDateTime(value)
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

async function loadFeedData() {
  if (!isAuthenticated.value) {
    unsubscribeAllPostRooms()
    posts.value = []
    suggestedUsers.value = []
    clearObject(expandedComments)
    clearObject(commentsByPost)
    clearObject(commentsLoading)
    clearObject(commentErrorByPost)
    clearObject(commentForms)
    clearObject(replyForms)
    clearObject(commentSubmitting)
    return
  }

  isLoading.value = true
  requestError.value = ""

  try {
    const [feedPosts, discoverUsers] = await Promise.all([fetchFeed(), fetchDiscoverUsers()])

    unsubscribeAllPostRooms()
    posts.value = feedPosts
    suggestedUsers.value = discoverUsers

    clearObject(expandedComments)
    clearObject(commentsByPost)
    clearObject(commentsLoading)
    clearObject(commentErrorByPost)
    clearObject(commentForms)
    clearObject(replyForms)
    clearObject(commentSubmitting)

    for (const post of feedPosts) {
      activeSlides[post.id] = 0
      ensureCommentState(post.id)
    }

    await focusPostFromRoute()
  } catch (error) {
    requestError.value = error instanceof Error ? error.message : "Could not load the feed."
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
    for (const comment of commentsByPost[postId]) {
      ensureReplyForm(postId, comment.id)
    }
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

removeRealtimeListeners.push(
  realtimeClient.on("comment.created", handleLiveCommentEvent),
  realtimeClient.on("comment.reply.created", handleLiveCommentEvent)
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
    <div class="panel">
      <p class="eyebrow">Feed</p>
      <h2>Posts, threads, and privacy rules</h2>
      <p>
        Browse the latest posts, open comment threads directly in the feed, and keep replies intentionally capped at two levels while the data model stays ready for deeper trees later.
      </p>
    </div>

    <p v-if="requestError" class="form-error">{{ requestError }}</p>

    <div v-if="!isAuthenticated" class="panel">
      <h3>Sign in to unlock the feed</h3>
      <p>
        The personalized feed, follow graph, dedicated create flow, and threaded comments are only available after authentication.
      </p>
    </div>

    <template v-else>
      <div class="feed-layout">
        <section class="page">
          <div class="panel panel--inset">
            <p class="eyebrow">Posting flow</p>
            <h3>Use the + action</h3>
            <p>
              The feed is now focused on reading and discussion. Use the `+` icon in the side rail to publish a new post.
            </p>
          </div>

          <div class="feed-header">
            <h3>Latest posts</h3>
            <p>{{ isLoading ? "Refreshing the feed..." : `${posts.length} visible posts` }}</p>
          </div>

          <article v-for="post in posts" :id="`post-${post.id}`" :key="post.id" class="panel post-card">
            <header class="post-card__header">
              <div>
                <p class="eyebrow">Post</p>
                <h3>{{ post.title }}</h3>
                <p class="post-card__meta">
                  <strong>{{ displayName(post.author) }}</strong>
                  <span>{{ formatDate(post.createdAt) }}</span>
                  <span class="badge">{{ post.privacy }}</span>
                  <span class="badge badge--soft">{{ post.author.profileVisibility }}</span>
                  <span class="badge badge--neutral">{{ commentCountLabel(post.commentsCount || 0) }}</span>
                </p>
              </div>
              <div class="post-card__timestamps">
                <span>Created {{ formatDate(post.createdAt) }}</span>
                <span v-if="post.updatedAt !== post.createdAt">Updated {{ formatDate(post.updatedAt) }}</span>
              </div>
            </header>

            <p class="post-card__body">{{ post.body }}</p>

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
                      <strong>{{ displayName(comment.author) }}</strong>
                      <span>{{ formatDate(comment.createdAt) }}</span>
                    </header>
                    <p class="comment-card__body">{{ comment.body }}</p>
                    <div class="comment-card__actions">
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
                          <strong>{{ displayName(reply.author) }}</strong>
                          <span>{{ formatDate(reply.createdAt) }}</span>
                        </header>
                        <p class="comment-card__body">{{ reply.body }}</p>
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
            <h3>Your feed is empty for now</h3>
            <p>Use the `+` action to create the first post or follow another public account to start filling this page.</p>
          </div>
        </section>

        <aside class="feed-side">
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
