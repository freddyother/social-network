<script setup>
import { Smile, X } from "lucide-vue-next"
import { computed, onBeforeUnmount, onMounted, ref } from "vue"

const props = defineProps({
  label: {
    type: String,
    default: "Add emoji"
  }
})

const emit = defineEmits(["select"])

const root = ref(null)
const isOpen = ref(false)
const searchTerm = ref("")

const emojiGroups = [
  {
    name: "Smileys",
    emojis: ["😀", "😄", "😊", "😍", "😂", "🥳", "😎", "🤔", "😅", "🥹", "🙌", "👏"]
  },
  {
    name: "Gestures",
    emojis: ["👍", "👎", "👌", "✌️", "🤝", "🙏", "💪", "🫶", "👀", "💅", "🤟", "👋"]
  },
  {
    name: "Feelings",
    emojis: ["❤️", "🧡", "💛", "💚", "💙", "💜", "🖤", "✨", "🔥", "💯", "🎉", "⭐"]
  },
  {
    name: "Objects",
    emojis: ["📌", "📷", "🎧", "💡", "🚀", "☕", "🍕", "🌈", "🌍", "🏆", "✅", "⚡"]
  }
]

const emojiKeywords = {
  "😀": "grin smile happy",
  "😄": "laugh smile happy",
  "😊": "smile blush happy",
  "😍": "love heart eyes",
  "😂": "laugh tears funny",
  "🥳": "party celebrate",
  "😎": "cool sunglasses",
  "🤔": "thinking question",
  "😅": "sweat nervous laugh",
  "🥹": "touched emotional",
  "🙌": "hands celebrate",
  "👏": "clap applause",
  "👍": "thumbs up yes like",
  "👎": "thumbs down no",
  "👌": "ok perfect",
  "✌️": "peace victory",
  "🤝": "handshake deal",
  "🙏": "pray thanks please",
  "💪": "strong muscle",
  "🫶": "heart hands care",
  "👀": "eyes look",
  "💅": "nails polished",
  "🤟": "love you sign",
  "👋": "wave hello",
  "❤️": "heart love red",
  "🧡": "heart love orange",
  "💛": "heart love yellow",
  "💚": "heart love green",
  "💙": "heart love blue",
  "💜": "heart love purple",
  "🖤": "heart love black",
  "✨": "sparkles shine",
  "🔥": "fire hot",
  "💯": "hundred perfect",
  "🎉": "party celebration",
  "⭐": "star favorite",
  "📌": "pin note",
  "📷": "camera photo",
  "🎧": "headphones music",
  "💡": "idea lightbulb",
  "🚀": "rocket launch",
  "☕": "coffee",
  "🍕": "pizza food",
  "🌈": "rainbow",
  "🌍": "earth world",
  "🏆": "trophy win",
  "✅": "check done",
  "⚡": "zap energy"
}

const visibleGroups = computed(() => {
  const query = searchTerm.value.trim().toLowerCase()
  if (!query) {
    return emojiGroups
  }

  return emojiGroups
    .map((group) => ({
      ...group,
      emojis: group.emojis.filter((emoji) =>
        `${emoji} ${emojiKeywords[emoji] || ""} ${group.name}`.toLowerCase().includes(query)
      )
    }))
    .filter((group) => group.emojis.length > 0)
})

function togglePicker() {
  isOpen.value = !isOpen.value
}

function closePicker() {
  isOpen.value = false
  searchTerm.value = ""
}

function selectEmoji(emoji) {
  emit("select", emoji)
  closePicker()
}

function handleDocumentPointerDown(event) {
  if (!isOpen.value || root.value?.contains(event.target)) {
    return
  }

  closePicker()
}

function handleDocumentKeydown(event) {
  if (event.key === "Escape") {
    closePicker()
  }
}

onMounted(() => {
  document.addEventListener("pointerdown", handleDocumentPointerDown)
  document.addEventListener("keydown", handleDocumentKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", handleDocumentPointerDown)
  document.removeEventListener("keydown", handleDocumentKeydown)
})
</script>

<template>
  <div ref="root" class="emoji-picker" :class="{ 'emoji-picker--open': isOpen }">
    <button
      type="button"
      class="emoji-picker__trigger"
      :aria-expanded="isOpen"
      :aria-label="props.label"
      :title="props.label"
      @click="togglePicker"
    >
      <Smile :size="18" aria-hidden="true" />
    </button>

    <div v-if="isOpen" class="emoji-picker__panel" role="dialog" :aria-label="props.label">
      <div class="emoji-picker__header">
        <strong>Emoji</strong>
        <button type="button" class="emoji-picker__close" aria-label="Close emoji picker" @click="closePicker">
          <X :size="16" aria-hidden="true" />
        </button>
      </div>

      <input
        v-model="searchTerm"
        type="search"
        class="emoji-picker__search"
        placeholder="Search emoji"
        autocomplete="off"
      />

      <div v-if="visibleGroups.length" class="emoji-picker__groups">
        <section v-for="group in visibleGroups" :key="group.name" class="emoji-picker__group">
          <p>{{ group.name }}</p>
          <div class="emoji-picker__grid">
            <button
              v-for="emoji in group.emojis"
              :key="`${group.name}-${emoji}`"
              type="button"
              class="emoji-picker__option"
              :aria-label="`Insert ${emoji}`"
              @click="selectEmoji(emoji)"
            >
              {{ emoji }}
            </button>
          </div>
        </section>
      </div>

      <p v-else class="feed-note">No emoji found.</p>
    </div>
  </div>
</template>
