<script setup>
import FeatureCard from "../components/ui/FeatureCard.vue"

const reasons = [
  {
    title: "Reactive SPA",
    tag: "Vue 3",
    description: "A strong fit for feed, chat, notifications, and forms with a lot of shared state."
  },
  {
    title: "Clean separation",
    tag: "Architecture",
    description: "Vue 3 frontend and Go backend decoupled through HTTP, cookies, and CORS."
  },
  {
    title: "Scalable",
    tag: "Large project",
    description: "The structure leaves room for auth, profiles, followers, groups, events, and WebSockets."
  }
]

const milestones = [
  "registration and login with cookies",
  "public feed, followers, and profile privacy",
  "groups, events, and persistent notifications",
  "private and group chat through WebSockets"
]
</script>

<template>
  <section class="page">
    <div class="hero-panel">
      <p class="eyebrow">Recommended choice</p>
      <img src="/nexo-logo.png" alt="NEXO logo" class="hero-panel__logo" />
      <p>
        The main reason is simple: this project needs a complex, maintainable SPA, not a framework that tries to absorb the backend as well.
      </p>
    </div>

    <section class="grid grid--three">
      <FeatureCard
        v-for="reason in reasons"
        :key="reason.title"
        :title="reason.title"
        :tag="reason.tag"
        :description="reason.description"
      />
    </section>

    <section class="panel">
      <p class="eyebrow">Initial roadmap</p>
      <h3>Foundation already aligned with the project modules</h3>
      <ul class="check-list">
        <li v-for="milestone in milestones" :key="milestone">
          {{ milestone }}
        </li>
      </ul>
    </section>
  </section>
</template>
