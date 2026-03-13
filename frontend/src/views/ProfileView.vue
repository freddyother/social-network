<script setup>
defineProps({
  handle: {
    type: String,
    default: "me"
  }
})
</script>

<template>
  <section class="page">
    <div class="panel">
      <p class="eyebrow">Profile</p>
      <h2>{{ handle }}'s profile</h2>
      <p>
        This is where the user card, posts, followers/following, and the public/private profile toggle naturally fit.
      </p>
    </div>

    <div class="grid grid--two">
      <article class="panel">
        <h3>Main sections</h3>
        <p>profile header, activity, followers, following, and pending requests.</p>
      </article>
      <article class="panel">
        <h3>Permissions</h3>
        <p>the UI can hide content when the profile is private and the visitor is not a follower.</p>
      </article>
    </div>
  </section>
</template>
