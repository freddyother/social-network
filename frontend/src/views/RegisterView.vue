<script setup>
import { onBeforeUnmount, reactive, ref, watch } from "vue"
import { useRouter } from "vue-router"

import { checkNicknameAvailability, isApiError, registerUser } from "../services/api"
import { toISODateInput } from "../utils/date"

const router = useRouter()

const form = reactive({
  firstName: "",
  lastName: "",
  email: "",
  password: "",
  dateOfBirth: "",
  nickname: "",
  aboutMe: ""
})

const fieldErrors = reactive({
  firstName: "",
  lastName: "",
  email: "",
  password: "",
  dateOfBirth: "",
  nickname: "",
  aboutMe: ""
})

const formError = ref("")
const isSubmitting = ref(false)
const isNicknameCheckPending = ref(false)
const nicknameAvailability = reactive({
  state: "idle",
  message: "",
  checkedNickname: ""
})

let nicknameCheckTimeoutId = 0
let nicknameCheckAbortController = null

function clearErrors() {
  formError.value = ""

  Object.keys(fieldErrors).forEach((key) => {
    fieldErrors[key] = ""
  })
}

function resetNicknameAvailability() {
  nicknameAvailability.state = "idle"
  nicknameAvailability.message = ""
  nicknameAvailability.checkedNickname = ""
}

function clearNicknameCheckTimeout() {
  if (!nicknameCheckTimeoutId) {
    return
  }

  clearTimeout(nicknameCheckTimeoutId)
  nicknameCheckTimeoutId = 0
}

function abortNicknameCheck() {
  if (!nicknameCheckAbortController) {
    return
  }

  nicknameCheckAbortController.abort()
  nicknameCheckAbortController = null
}

async function runNicknameAvailabilityCheck(nickname) {
  const normalizedNickname = nickname.trim()
  if (!normalizedNickname) {
    isNicknameCheckPending.value = false
    resetNicknameAvailability()
    return
  }

  nicknameAvailability.state = "checking"
  nicknameAvailability.message = "Checking nickname availability..."
  nicknameAvailability.checkedNickname = normalizedNickname
  isNicknameCheckPending.value = true

  const controller = new AbortController()
  nicknameCheckAbortController = controller

  try {
    const availability = await checkNicknameAvailability(normalizedNickname, controller.signal)
    if (nicknameCheckAbortController !== controller) {
      return
    }

    nicknameAvailability.state = availability.available ? "available" : "unavailable"
    nicknameAvailability.message = availability.available ? "Nickname is available." : ""
    nicknameAvailability.checkedNickname = normalizedNickname

    if (!availability.available) {
      fieldErrors.nickname = "That nickname is already in use."
    }
  } catch (error) {
    if (controller.signal.aborted) {
      return
    }

    if (isApiError(error, 422)) {
      fieldErrors.nickname = error.payload?.fields?.nickname || error.message
      resetNicknameAvailability()
      return
    }

    nicknameAvailability.state = "error"
    nicknameAvailability.message = "Could not verify nickname right now."
    nicknameAvailability.checkedNickname = normalizedNickname
  } finally {
    if (nicknameCheckAbortController === controller) {
      nicknameCheckAbortController = null
    }

    isNicknameCheckPending.value = false
  }
}

watch(
  () => form.nickname,
  (value) => {
    clearNicknameCheckTimeout()
    abortNicknameCheck()
    fieldErrors.nickname = ""
    isNicknameCheckPending.value = false

    const normalizedNickname = value.trim()
    if (!normalizedNickname) {
      resetNicknameAvailability()
      return
    }

    if (normalizedNickname.length > 80) {
      resetNicknameAvailability()
      fieldErrors.nickname = "Nickname must be 80 characters or fewer."
      return
    }

    resetNicknameAvailability()
    isNicknameCheckPending.value = true
    nicknameCheckTimeoutId = setTimeout(() => {
      nicknameCheckTimeoutId = 0
      void runNicknameAvailabilityCheck(normalizedNickname)
    }, 350)
  }
)

onBeforeUnmount(() => {
  clearNicknameCheckTimeout()
  abortNicknameCheck()
})

async function handleSubmit() {
  clearErrors()
  isSubmitting.value = true

  try {
    const normalizedDateOfBirth = toISODateInput(form.dateOfBirth)
    if (!normalizedDateOfBirth) {
      fieldErrors.dateOfBirth = "Use a valid date like 27/02/1987 or 1987-02-27."
      formError.value = "Please correct the highlighted fields."
      return
    }

    if (isNicknameCheckPending.value) {
      fieldErrors.nickname = "Wait a moment while we verify your nickname."
      formError.value = "Please correct the highlighted fields."
      return
    }

    if (nicknameAvailability.state === "unavailable") {
      fieldErrors.nickname = "That nickname is already in use."
      formError.value = "Please correct the highlighted fields."
      return
    }

    await registerUser({
      ...form,
      dateOfBirth: normalizedDateOfBirth
    })

    await router.push("/login")
  } catch (error) {
    if (isApiError(error)) {
      formError.value = error.message
      const apiFieldErrors = error.payload?.fields || {}

      Object.keys(fieldErrors).forEach((key) => {
        fieldErrors[key] = apiFieldErrors[key] || ""
      })
    } else {
      formError.value = "Could not create your account right now."
    }
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <section class="page">
    <div class="panel">
      <p class="eyebrow">Auth</p>
      <h2>Register</h2>
      <form class="form-grid" @submit.prevent="handleSubmit">
        <label>
          <span>First Name</span>
          <input
            v-model.trim="form.firstName"
            type="text"
            placeholder="Ada"
            autocomplete="given-name"
            :aria-invalid="Boolean(fieldErrors.firstName)"
            required
          />
          <p v-if="fieldErrors.firstName" class="form-error">{{ fieldErrors.firstName }}</p>
        </label>
        <label>
          <span>Last Name</span>
          <input
            v-model.trim="form.lastName"
            type="text"
            placeholder="Lovelace"
            autocomplete="family-name"
            :aria-invalid="Boolean(fieldErrors.lastName)"
            required
          />
          <p v-if="fieldErrors.lastName" class="form-error">{{ fieldErrors.lastName }}</p>
        </label>
        <label>
          <span>Email</span>
          <input
            v-model.trim="form.email"
            type="email"
            placeholder="ada@example.com"
            autocomplete="email"
            :aria-invalid="Boolean(fieldErrors.email)"
            required
          />
          <p v-if="fieldErrors.email" class="form-error">{{ fieldErrors.email }}</p>
        </label>
        <label>
          <span>Password</span>
          <input
            v-model="form.password"
            type="password"
            placeholder="********"
            autocomplete="new-password"
            minlength="8"
            :aria-invalid="Boolean(fieldErrors.password)"
            required
          />
          <p v-if="fieldErrors.password" class="form-error">{{ fieldErrors.password }}</p>
        </label>
        <label>
          <span>Date of Birth</span>
          <input
            v-model="form.dateOfBirth"
            type="text"
            inputmode="numeric"
            placeholder="dd/mm/yyyy"
            autocomplete="bday"
            pattern="(?:[0-9]{4}-[0-9]{2}-[0-9]{2}|[0-9]{1,2}/[0-9]{1,2}/[0-9]{4})"
            title="Use dd/mm/yyyy or yyyy-mm-dd"
            :aria-invalid="Boolean(fieldErrors.dateOfBirth)"
            required
          />
          <p v-if="fieldErrors.dateOfBirth" class="form-error">{{ fieldErrors.dateOfBirth }}</p>
        </label>
        <label>
          <span>Nickname</span>
          <input
            v-model.trim="form.nickname"
            type="text"
            placeholder="Choose a nickname"
            :aria-invalid="Boolean(fieldErrors.nickname)"
            required
          />
          <p v-if="fieldErrors.nickname" class="form-error">{{ fieldErrors.nickname }}</p>
          <p
            v-else-if="nicknameAvailability.message"
            :class="[
              'form-hint',
              nicknameAvailability.state === 'available' ? 'form-hint--success' : 'form-hint--muted'
            ]"
          >
            {{ nicknameAvailability.message }}
          </p>
        </label>
        <label class="form-grid__full">
          <span>About Me</span>
          <textarea
            v-model.trim="form.aboutMe"
            rows="4"
            placeholder="Tell your story in a few lines"
            :aria-invalid="Boolean(fieldErrors.aboutMe)"
          ></textarea>
          <p v-if="fieldErrors.aboutMe" class="form-error">{{ fieldErrors.aboutMe }}</p>
        </label>
        <p v-if="formError" class="form-error form-grid__full">{{ formError }}</p>
        <button
          type="submit"
          class="button form-grid__full"
          :disabled="isSubmitting || isNicknameCheckPending || nicknameAvailability.state === 'unavailable'"
        >
          {{ isSubmitting ? "Creating account..." : "Create account" }}
        </button>
      </form>
    </div>
  </section>
</template>
