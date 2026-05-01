<script setup>
import { computed, ref, watch } from "vue"

const props = defineProps({
  user: {
    type: Object,
    default: null
  },
  name: {
    type: String,
    default: ""
  },
  src: {
    type: String,
    default: ""
  },
  alt: {
    type: String,
    default: ""
  },
  size: {
    type: String,
    default: "small"
  }
})

const imageFailed = ref(false)

const avatarSrc = computed(() => props.src || props.user?.avatarUrl || "")
const displayLabel = computed(() => props.name || displayName(props.user) || "NEXO")
const fallbackInitial = computed(() => firstInitial(displayLabel.value))
const avatarColor = computed(() => colorFromKey(
  props.user?.id || props.user?.email || props.user?.nickname || displayLabel.value
))
const avatarClasses = computed(() => [
  "user-avatar",
  props.size === "profile" ? "user-avatar--profile" : "user-avatar--small"
])
const shouldShowImage = computed(() => Boolean(avatarSrc.value) && !imageFailed.value)

watch(avatarSrc, () => {
  imageFailed.value = false
})

function displayName(user) {
  if (!user) {
    return ""
  }

  return user.nickname || `${user.firstName || ""} ${user.lastName || ""}`.trim() || user.email || ""
}

function firstInitial(value) {
  return Array.from(String(value || "").trim())[0]?.toLocaleUpperCase() || "N"
}

function colorFromKey(value) {
  const palette = [
    "#d83b01",
    "#0078d4",
    "#107c10",
    "#8764b8",
    "#c239b3",
    "#ca5010",
    "#008575",
    "#5c2d91",
    "#b4009e",
    "#0b6a75"
  ]
  const key = String(value || "nexo")
  let hash = 0

  for (const char of key) {
    hash = ((hash << 5) - hash + char.codePointAt(0)) | 0
  }

  return palette[Math.abs(hash) % palette.length]
}
</script>

<template>
  <span :class="avatarClasses" :style="{ '--avatar-bg': avatarColor }">
    <img
      v-if="shouldShowImage"
      :src="avatarSrc"
      :alt="alt || `${displayLabel} avatar`"
      class="user-avatar__image"
      @error="imageFailed = true"
    />
    <span v-else class="user-avatar__fallback">{{ fallbackInitial }}</span>
  </span>
</template>
