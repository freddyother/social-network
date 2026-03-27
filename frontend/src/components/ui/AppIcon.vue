<script setup>
import {
  CircleUserRound,
  Compass,
  Heart,
  House,
  Plus,
  Send
} from "lucide-vue-next"

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

const icons = {
  feed: House,
  create: Plus,
  profile: CircleUserRound,
  groups: Compass,
  notifications: Heart,
  chat: Send
}
</script>

<template>
  <component
    :is="icons[props.name] || House"
    :size="props.size"
    :stroke-width="2.1"
    aria-hidden="true"
  />
</template>
