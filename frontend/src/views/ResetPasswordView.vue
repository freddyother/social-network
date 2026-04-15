<script setup>
import { computed, reactive, ref, watch } from "vue"
import { useRoute, useRouter } from "vue-router"

import { isApiError, resetPassword } from "../services/api"

const route = useRoute()
const router = useRouter()

const form = reactive({
  token: typeof route.query.token === "string" ? route.query.token : "",
  newPassword: "",
  confirmPassword: ""
})

const fieldErrors = reactive({
  token: "",
  newPassword: "",
  confirmPassword: ""
})

const formError = ref("")
const successMessage = ref("")
const isSubmitting = ref(false)

watch(
  () => route.query.token,
  (token) => {
    form.token = typeof token === "string" ? token : ""
  }
)

const hasToken = computed(() => Boolean(form.token.trim()))

function clearMessages() {
  Object.keys(fieldErrors).forEach((key) => {
    fieldErrors[key] = ""
  })
  formError.value = ""
  successMessage.value = ""
}

async function handleSubmit() {
  clearMessages()

  if (!hasToken.value) {
    fieldErrors.token = "Reset token is required."
    formError.value = "That password reset link is incomplete."
    return
  }

  if (form.newPassword !== form.confirmPassword) {
    fieldErrors.confirmPassword = "Passwords do not match."
    formError.value = "Please correct the highlighted fields."
    return
  }

  isSubmitting.value = true

  try {
    const payload = await resetPassword({
      token: form.token,
      newPassword: form.newPassword
    })
    successMessage.value = payload?.message || "Your password has been updated. You can sign in now."
    form.newPassword = ""
    form.confirmPassword = ""
    setTimeout(() => {
      void router.push("/login")
    }, 1200)
  } catch (error) {
    if (isApiError(error)) {
      formError.value = error.message
      const apiFieldErrors = error.payload?.fields || {}
      fieldErrors.token = apiFieldErrors.token || ""
      fieldErrors.newPassword = apiFieldErrors.newPassword || ""
    } else {
      formError.value = "Could not reset your password right now."
    }
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <section class="page">
    <div class="panel panel--narrow">
      <p class="eyebrow">Auth</p>
      <h2>Reset password</h2>
      <form class="stack-form" @submit.prevent="handleSubmit">
        <p v-if="!hasToken" class="form-error">
          This page needs a valid reset link with a token.
        </p>
        <p v-if="fieldErrors.token" class="form-error">{{ fieldErrors.token }}</p>
        <label>
          <span>New password</span>
          <input
            v-model="form.newPassword"
            type="password"
            placeholder="********"
            autocomplete="new-password"
            :aria-invalid="Boolean(fieldErrors.newPassword)"
            required
          />
          <p v-if="fieldErrors.newPassword" class="form-error">{{ fieldErrors.newPassword }}</p>
        </label>
        <label>
          <span>Confirm password</span>
          <input
            v-model="form.confirmPassword"
            type="password"
            placeholder="********"
            autocomplete="new-password"
            :aria-invalid="Boolean(fieldErrors.confirmPassword)"
            required
          />
          <p v-if="fieldErrors.confirmPassword" class="form-error">{{ fieldErrors.confirmPassword }}</p>
        </label>
        <p v-if="formError" class="form-error">{{ formError }}</p>
        <p v-if="successMessage" class="form-hint form-hint--success">{{ successMessage }}</p>
        <button type="submit" class="button" :disabled="isSubmitting || !hasToken">
          {{ isSubmitting ? "Resetting password..." : "Reset password" }}
        </button>
        <RouterLink to="/login" class="auth-inline-link">
          Back to login
        </RouterLink>
      </form>
    </div>
  </section>
</template>
