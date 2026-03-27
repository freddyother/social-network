<script setup>
const props = defineProps({
  name: {
    type: String,
    required: true
  },
  size: {
    type: Number,
    default: 22
  }
})
</script>

<template>
  <svg
    :width="props.size"
    :height="props.size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <template v-if="props.name === 'overview'">
      <path d="M3 4h8v7H3z" />
      <path d="M13 4h8v4h-8z" />
      <path d="M13 10h8v10h-8z" />
      <path d="M3 13h8v7H3z" />
    </template>

    <template v-else-if="props.name === 'feed'">
      <rect x="3" y="4" width="18" height="16" rx="3" />
      <path d="M7 8h10" />
      <path d="M7 12h6" />
      <path d="M7 16h8" />
    </template>

    <template v-else-if="props.name === 'create'">
      <path d="M12 5v14" />
      <path d="M5 12h14" />
      <circle cx="12" cy="12" r="9" />
    </template>

    <template v-else-if="props.name === 'profile'">
      <path d="M18 20a6 6 0 0 0-12 0" />
      <circle cx="12" cy="8" r="4" />
    </template>

    <template v-else-if="props.name === 'groups'">
      <circle cx="8" cy="9" r="3" />
      <circle cx="16.5" cy="10.5" r="2.5" />
      <path d="M3.5 19a4.5 4.5 0 0 1 9 0" />
      <path d="M14 19a3.5 3.5 0 0 1 7 0" />
    </template>

    <template v-else-if="props.name === 'notifications'">
      <path d="M6 9a6 6 0 1 1 12 0c0 4 1.5 5.5 2 6H4c.5-.5 2-2 2-6" />
      <path d="M10 19a2 2 0 0 0 4 0" />
    </template>

    <template v-else-if="props.name === 'chat'">
      <path d="M5 6h14a2 2 0 0 1 2 2v7a2 2 0 0 1-2 2H9l-4 3v-3H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2z" />
      <path d="M8 11h8" />
      <path d="M8 14h5" />
    </template>
  </svg>
</template>
