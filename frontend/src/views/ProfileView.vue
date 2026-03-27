<script setup>
import { computed, reactive, ref, watch } from "vue"

import {
  acceptFollowRequest,
  declineFollowRequest,
  fetchFollowRequests,
  isApiError,
  updateProfileVisibility
} from "../services/api"
import { useAppStore } from "../stores/app"

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

const requests = ref([])
const profileError = ref("")
const requestError = ref("")
const isSavingPrivacy = ref(false)
const loadingRequests = ref(false)
const requestLoading = reactive({})

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
      </div>
    </template>
  </section>
</template>
