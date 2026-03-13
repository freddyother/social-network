<script setup>
defineProps({
  title: {
    type: String,
    required: true
  },
  tag: {
    type: String,
    default: ""
  },
  description: {
    type: String,
    required: true
  }
})
</script>

<template>
  <article class="feature-card">
    <p v-if="tag" class="feature-card__tag">{{ tag }}</p>
    <h3>{{ title }}</h3>
    <p>{{ description }}</p>
    <slot />
  </article>
</template>
