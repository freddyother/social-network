<script setup>
import { computed, onBeforeUnmount, reactive, ref, watch } from "vue"

import {
  acceptFollowRequest,
  declineFollowRequest,
  fetchFollowRequests,
  isApiError,
  updateProfileVisibility,
  updateThemePreference
} from "../services/api"
import { realtimeClient } from "../services/realtime"
import { useAppStore } from "../stores/app"
import { THEME_OPTIONS } from "../theme"

const props = defineProps({
  handle: {
    type: String,
    default: "me"
  }
})

const store = useAppStore()
const currentProfile = computed(() => (props.handle === "me" ? store.state.currentUser : null))
const profileName = computed(() => {
  const user = currentProfile.value
  if (!user) {
    return props.handle
  }

  return user.nickname || `${user.firstName} ${user.lastName}`.trim() || user.email
})

const privacyForm = reactive({
  visibility: "public"
})

const themeOptions = THEME_OPTIONS
const requests = ref([])
const profileError = ref("")
const requestError = ref("")
const themeError = ref("")
const isSavingPrivacy = ref(false)
const isSavingTheme = ref("")
const loadingRequests = ref(false)
const requestLoading = reactive({})
const removeRealtimeListeners = []
const activeThemePreference = computed(() => currentProfile.value?.themePreference || store.state.themePreference)

function loadPrivacyValue() {
  privacyForm.visibility = currentProfile.value?.profileVisibility || "public"
}

async function loadFollowRequests() {
  if (!currentProfile.value) {
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

async function saveProfileVisibility() {
  if (!currentProfile.value) {
    return
  }

  isSavingPrivacy.value = true
  profileError.value = ""

  try {
    const updatedUser = await updateProfileVisibility(privacyForm.visibility)
    store.setCurrentUser(updatedUser)
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

function themePreviewStyle(option) {
  return {
    "--theme-preview-1": option.swatches[0],
    "--theme-preview-2": option.swatches[1],
    "--theme-preview-3": option.swatches[2],
    "--theme-preview-4": option.swatches[3]
  }
}

async function selectTheme(themePreference) {
  if (!currentProfile.value || activeThemePreference.value === themePreference) {
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
  } catch (error) {
    requestError.value = error instanceof Error ? error.message : "Could not update the request."
  } finally {
    requestLoading[requestId] = false
  }
}

function displayName(user) {
  if (!user) {
    return ""
  }

  return user.nickname || `${user.firstName} ${user.lastName}`.trim() || user.email
}

watch(
  () => currentProfile.value?.id,
  () => {
    loadPrivacyValue()
    void loadFollowRequests()
  },
  { immediate: true }
)

removeRealtimeListeners.push(
  realtimeClient.on("follow_request.created", () => {
    if (!currentProfile.value) {
      return
    }

    void loadFollowRequests()
  })
)

onBeforeUnmount(() => {
  removeRealtimeListeners.splice(0).forEach((dispose) => dispose())
})
</script>

<template>
  <section class="page">
    <div class="panel">
      <p class="eyebrow">Profile</p>
      <template v-if="currentProfile">
        <h2>{{ profileName }}</h2>
        <p>
          Signed in as {{ currentProfile.email }} with a {{ currentProfile.profileVisibility }} profile.
        </p>
      </template>
      <template v-else>
        <h2>{{ props.handle }}'s profile</h2>
        <p>
          This is where the user card, posts, followers/following, and the public/private profile toggle naturally fit.
        </p>
      </template>
    </div>

    <div class="grid grid--two">
      <article class="panel">
        <template v-if="currentProfile">
          <h3>Account details</h3>
          <p>Date of birth: {{ currentProfile.dateOfBirth }}</p>
          <p>Nickname: {{ currentProfile.nickname || "Not set yet" }}</p>
          <p>Email: {{ currentProfile.email }}</p>
        </template>
        <template v-else>
          <h3>Main sections</h3>
          <p>profile header, activity, followers, following, and pending requests.</p>
        </template>
      </article>
      <article class="panel">
        <template v-if="currentProfile">
          <h3>About me</h3>
          <p>{{ currentProfile.aboutMe || "Tell your story in a few lines to make this profile feel alive." }}</p>
        </template>
        <template v-else>
          <h3>Permissions</h3>
          <p>the UI can hide content when the profile is private and the visitor is not a follower.</p>
        </template>
      </article>
    </div>

    <template v-if="currentProfile">
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
            <button type="button" class="button" :disabled="isSavingPrivacy" @click="saveProfileVisibility">
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

      <section class="panel">
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
  </section>
</template>
