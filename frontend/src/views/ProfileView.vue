<script setup>
import { computed } from "vue"

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
  </section>
</template>
