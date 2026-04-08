<script setup>
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from "vue"
import { useRoute } from "vue-router"

import {
  acceptFollowRequest,
  declineFollowRequest,
  fetchFollowRequests,
  isApiError,
  updateProfile,
  uploadProfileAvatar,
  updateProfileVisibility,
  updateThemePreference
} from "../services/api"
import { realtimeClient } from "../services/realtime"
import { useAppStore } from "../stores/app"
import { THEME_OPTIONS } from "../theme"
import { formatDate } from "../utils/date"

const props = defineProps({
  handle: {
    type: String,
    default: "me"
  }
})

const store = useAppStore()
const route = useRoute()
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
const activeThemePreference = computed(() => currentProfile.value?.themePreference || store.state.themePreference)
const hasVisibilityChanges = computed(
  () => Boolean(currentProfile.value) && privacyForm.visibility !== (currentProfile.value?.profileVisibility || "public")
)
const hasProfileChanges = computed(() => {
  if (!currentProfile.value) {
    return false
  }

  return (
    profileForm.firstName !== (currentProfile.value.firstName || "") ||
    profileForm.lastName !== (currentProfile.value.lastName || "") ||
    profileForm.aboutMe !== (currentProfile.value.aboutMe || "")
  )
})
const profileInitials = computed(() => {
  const user = currentProfile.value
  if (!user) {
    return "N"
  }

  const source = user.nickname || `${user.firstName} ${user.lastName}`.trim() || user.email || "NEXO"
  return source
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() || "")
    .join("") || "N"
})

function loadPrivacyValue() {
  privacyForm.visibility = currentProfile.value?.profileVisibility || "public"
}

function loadProfileForm() {
  profileForm.firstName = currentProfile.value?.firstName || ""
  profileForm.lastName = currentProfile.value?.lastName || ""
  profileForm.aboutMe = currentProfile.value?.aboutMe || ""
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

async function saveProfile() {
  if (!currentProfile.value || !hasProfileChanges.value) {
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
  if (!currentProfile.value || !hasVisibilityChanges.value) {
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

function openAvatarPicker() {
  avatarInput.value?.click()
}

async function handleAvatarChange(event) {
  const [file] = Array.from(event.target?.files || [])
  if (!file || !currentProfile.value) {
    return
  }

  isUploadingAvatar.value = true
  avatarError.value = ""

  try {
    const updatedUser = await uploadProfileAvatar(file)
    store.setCurrentUser(updatedUser)
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

function routeSection() {
  const rawValue = route.query.section
  if (Array.isArray(rawValue)) {
    return String(rawValue[0] || "").trim()
  }

  return typeof rawValue === "string" ? rawValue.trim() : ""
}

async function focusRequestedSection() {
  if (!currentProfile.value) {
    return
  }

  if (routeSection() !== "follow-requests") {
    return
  }

  await nextTick()
  followRequestsSection.value?.scrollIntoView({
    behavior: "smooth",
    block: "start"
  })
}

watch(
  () => [
    currentProfile.value?.firstName,
    currentProfile.value?.lastName,
    currentProfile.value?.aboutMe,
    currentProfile.value?.profileVisibility
  ],
  () => {
    loadPrivacyValue()
    loadProfileForm()
  },
  { immediate: true }
)

watch(
  () => currentProfile.value?.id,
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
    <div class="panel profile-hero">
      <p class="eyebrow">Profile</p>
      <template v-if="currentProfile">
        <div class="profile-hero__body">
          <div class="profile-hero__avatar-column">
            <div class="user-avatar user-avatar--profile">
              <img
                v-if="currentProfile.avatarUrl"
                :src="currentProfile.avatarUrl"
                :alt="`${profileName} profile photo`"
                class="user-avatar__image"
              />
              <span v-else class="user-avatar__fallback">{{ profileInitials }}</span>
            </div>
            <input
              ref="avatarInput"
              class="visually-hidden"
              type="file"
              accept="image/png,image/jpeg,image/webp,image/gif"
              @change="handleAvatarChange"
            />
            <button
              type="button"
              class="button button--ghost button--small"
              :disabled="isUploadingAvatar"
              @click="openAvatarPicker"
            >
              {{ isUploadingAvatar ? "Uploading..." : currentProfile.avatarUrl ? "Change photo" : "Add photo" }}
            </button>
            <p class="feed-note">Images are optimized automatically when uploaded.</p>
            <p v-if="avatarError" class="form-error">{{ avatarError }}</p>
          </div>

          <div class="profile-hero__content">
            <h2>{{ profileName }}</h2>
            <p>
              Signed in as {{ currentProfile.email }} with a {{ currentProfile.profileVisibility }} profile.
            </p>
          </div>
        </div>
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
          <h3>Main sections</h3>
          <p>profile header, activity, followers, following, and pending requests.</p>
        </template>
      </article>
      <article class="panel">
        <template v-if="currentProfile">
          <h3>Locked fields</h3>
          <p>Date of birth: {{ formatDate(currentProfile.dateOfBirth) }}</p>
          <p>Nickname: {{ currentProfile.nickname || "Not set yet" }}</p>
          <p>Email: {{ currentProfile.email }}</p>
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
  </section>
</template>
