<script setup>
import { computed, onBeforeUnmount, reactive, ref } from "vue"
import { useRouter } from "vue-router"

import { createPost, isApiError } from "../services/api"
import { useAppStore } from "../stores/app"

const store = useAppStore()
const router = useRouter()

const isAuthenticated = computed(() => Boolean(store.state.currentUser))
const hasPrivateAccount = computed(() => store.state.currentUser?.profileVisibility === "private")

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
const composerError = ref("")
const isSubmitting = ref(false)

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

async function submitPost() {
  clearComposerErrors()
  isSubmitting.value = true

  try {
    await createPost({
      title: composer.title,
      body: composer.body,
      privacy: composer.privacy,
      images: composer.images
    })

    await router.push("/feed")
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

onBeforeUnmount(() => {
  selectedPreviews.value.forEach((preview) => URL.revokeObjectURL(preview.url))
})
</script>

<template>
  <section class="page">
    <div class="panel">
      <p class="eyebrow">Create Post</p>
      <h2>Publish from the action rail</h2>
      <p>
        Start a new post from the `+` icon, add a title, caption, privacy, and your carousel images in one focused flow.
      </p>
    </div>

    <div v-if="!isAuthenticated" class="panel panel--narrow">
      <h3>Sign in to publish</h3>
      <p>
        Creating posts is only available after authentication. Use the login or register actions to continue.
      </p>
    </div>

    <section v-else class="panel create-post-panel">
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
            rows="5"
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

        <div class="create-post-panel__actions">
          <button type="button" class="button button--ghost" @click="router.push('/feed')">
            Cancel
          </button>
          <button type="submit" class="button" :disabled="isSubmitting">
            {{ isSubmitting ? "Publishing..." : "Publish post" }}
          </button>
        </div>
      </form>
    </section>
  </section>
</template>
