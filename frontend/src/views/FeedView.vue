<script setup>
import { computed, onBeforeUnmount, reactive, ref, watch } from "vue"

import {
  createPost,
  fetchDiscoverUsers,
  fetchFeed,
  followUser,
  isApiError
} from "../services/api"
import { useAppStore } from "../stores/app"

const store = useAppStore()

const isAuthenticated = computed(() => Boolean(store.state.currentUser))
const composer = reactive({
  title: "",
  body: "",
  privacy: "public",
  images: []
})

const composerErrors = reactive({
  title: "",
  body: "",
  privacy: "",
  images: ""
})

const selectedPreviews = ref([])
const posts = ref([])
const suggestedUsers = ref([])
const requestError = ref("")
const composerError = ref("")
const isLoading = ref(false)
const isSubmitting = ref(false)
const activeSlides = reactive({})
const followLoading = reactive({})

const hasPrivateAccount = computed(() => store.state.currentUser?.profileVisibility === "private")

function displayName(user) {
  if (!user) {
    return ""
  }

  return user.nickname || `${user.firstName} ${user.lastName}`.trim() || "Unknown user"
}

function formatDate(value) {
  return new Intl.DateTimeFormat("en-GB", {
    dateStyle: "medium",
    timeStyle: "short"
  }).format(new Date(value))
}

function clearComposerErrors() {
  composerError.value = ""
  Object.keys(composerErrors).forEach((key) => {
    composerErrors[key] = ""
  })
}

function syncSelectedImages(files) {
  selectedPreviews.value.forEach((preview) => URL.revokeObjectURL(preview.url))
  selectedPreviews.value = files.map((file) => ({
    name: file.name,
    url: URL.createObjectURL(file)
  }))
}

function handleImageSelection(event) {
  const files = Array.from(event.target.files || [])
  composer.images = files
  syncSelectedImages(files)
}

function resetComposer() {
  composer.title = ""
  composer.body = ""
  composer.privacy = "public"
  composer.images = []
  syncSelectedImages([])
}

async function loadFeedData() {
  if (!isAuthenticated.value) {
    posts.value = []
    suggestedUsers.value = []
    return
  }

  isLoading.value = true
  requestError.value = ""

  try {
    const [feedPosts, discoverUsers] = await Promise.all([fetchFeed(), fetchDiscoverUsers()])
    posts.value = feedPosts
    suggestedUsers.value = discoverUsers
  } catch (error) {
    requestError.value = error instanceof Error ? error.message : "Could not load the feed."
  } finally {
    isLoading.value = false
  }
}

async function submitPost() {
  clearComposerErrors()
  isSubmitting.value = true

  try {
    const post = await createPost({
      title: composer.title,
      body: composer.body,
      privacy: composer.privacy,
      images: composer.images
    })

    posts.value = [post, ...posts.value]
    activeSlides[post.id] = 0
    resetComposer()
  } catch (error) {
    if (isApiError(error)) {
      composerError.value = error.message
      const fieldErrors = error.payload?.fields || {}
      Object.keys(composerErrors).forEach((key) => {
        composerErrors[key] = fieldErrors[key] || ""
      })
    } else {
      composerError.value = "Could not publish the post right now."
    }
  } finally {
    isSubmitting.value = false
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
      posts.value = await fetchFeed()
    }
  } catch (error) {
    requestError.value = error instanceof Error ? error.message : "Could not update follow status."
  } finally {
    followLoading[userId] = false
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

watch(
  () => store.state.currentUser?.id,
  () => {
    void loadFeedData()
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  selectedPreviews.value.forEach((preview) => URL.revokeObjectURL(preview.url))
})
</script>

<template>
  <section class="page">
    <div class="panel">
      <p class="eyebrow">Feed</p>
      <h2>Posts, carousels, and privacy rules</h2>
      <p>
        Publish multi-image posts with titles and captions, decide whether each post is public or followers-only, and discover people to follow across public and private accounts.
      </p>
    </div>

    <p v-if="requestError" class="form-error">{{ requestError }}</p>

    <div v-if="!isAuthenticated" class="panel">
      <h3>Sign in to unlock the feed</h3>
      <p>
        The personalized feed, follow graph, and carousel composer are only available after authentication.
      </p>
    </div>

    <template v-else>
      <div class="feed-layout">
        <section class="panel feed-composer">
          <p class="eyebrow">Create post</p>
          <h3>Share a moment</h3>
          <form class="stack-form" @submit.prevent="submitPost">
            <label>
              <span>Title</span>
              <input
                v-model.trim="composer.title"
                type="text"
                placeholder="Golden hour in Lisbon"
                :aria-invalid="Boolean(composerErrors.title)"
                required
              />
              <p v-if="composerErrors.title" class="form-error">{{ composerErrors.title }}</p>
            </label>

            <label>
              <span>Caption</span>
              <textarea
                v-model.trim="composer.body"
                rows="4"
                placeholder="What made this post worth remembering?"
                :aria-invalid="Boolean(composerErrors.body)"
                required
              ></textarea>
              <p v-if="composerErrors.body" class="form-error">{{ composerErrors.body }}</p>
            </label>

            <div class="feed-composer__row">
              <label>
                <span>Post privacy</span>
                <select v-model="composer.privacy">
                  <option value="public">Visible to everyone</option>
                  <option value="followers">Followers only</option>
                </select>
                <p v-if="composerErrors.privacy" class="form-error">{{ composerErrors.privacy }}</p>
              </label>

              <label>
                <span>Images</span>
                <input
                  type="file"
                  accept="image/png,image/jpeg,image/webp,image/gif"
                  multiple
                  @change="handleImageSelection"
                />
                <p v-if="composerErrors.images" class="form-error">{{ composerErrors.images }}</p>
              </label>
            </div>

            <p class="feed-note">
              {{ hasPrivateAccount ? "Your account is private, so followers are required to see any post." : "Public accounts can still mark individual posts as followers-only." }}
            </p>

            <div v-if="selectedPreviews.length" class="preview-strip">
              <figure v-for="preview in selectedPreviews" :key="preview.url" class="preview-strip__item">
                <img :src="preview.url" :alt="preview.name" />
              </figure>
            </div>

            <p v-if="composerError" class="form-error">{{ composerError }}</p>

            <button type="submit" class="button" :disabled="isSubmitting">
              {{ isSubmitting ? "Publishing..." : "Publish post" }}
            </button>
          </form>
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

      <section class="page">
        <div class="feed-header">
          <h3>Latest posts</h3>
          <p>{{ isLoading ? "Refreshing the feed..." : `${posts.length} visible posts` }}</p>
        </div>

        <article v-for="post in posts" :key="post.id" class="panel post-card">
          <header class="post-card__header">
            <div>
              <p class="eyebrow">Post</p>
              <h3>{{ post.title }}</h3>
              <p class="post-card__meta">
                <strong>{{ displayName(post.author) }}</strong>
                <span>{{ formatDate(post.createdAt) }}</span>
                <span class="badge">{{ post.privacy }}</span>
                <span class="badge badge--soft">{{ post.author.profileVisibility }}</span>
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
        </article>

        <div v-if="!posts.length && !isLoading" class="panel">
          <h3>Your feed is empty for now</h3>
          <p>Create the first post or follow another public account to start filling this page.</p>
        </div>
      </section>
    </template>
  </section>
</template>
