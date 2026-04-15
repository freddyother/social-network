<script setup>
import { reactive, ref } from "vue"

import { isApiError, requestPasswordReset } from "../services/api"

const form = reactive({
  email: ""
})

const fieldErrors = reactive({
  email: ""
})

const formError = ref("")
const successMessage = ref("")
const resetLink = ref("")
const isSubmitting = ref(false)

function clearMessages() {
  fieldErrors.email = ""
  formError.value = ""
  successMessage.value = ""
  resetLink.value = ""
}

async function handleSubmit() {
  clearMessages()
  isSubmitting.value = true

  try {
    const payload = await requestPasswordReset(form.email)
    successMessage.value = payload?.message || "If the account exists, we sent a password reset link."
    resetLink.value = payload?.resetLink || ""
  } catch (error) {
    if (isApiError(error)) {
      formError.value = error.message
      fieldErrors.email = error.payload?.fields?.email || ""
    } else {
      formError.value = "Could not start password recovery right now."
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
      <h2>Forgot password</h2>
      <form class="stack-form" @submit.prevent="handleSubmit">
        <label>
          <span>Email</span>
          <input
            v-model.trim="form.email"
            type="email"
            autocomplete="email"
            placeholder="you@example.com"
            :aria-invalid="Boolean(fieldErrors.email)"
            required
          />
          <p v-if="fieldErrors.email" class="form-error">{{ fieldErrors.email }}</p>
        </label>
        <p v-if="formError" class="form-error">{{ formError }}</p>
        <p v-if="successMessage" class="form-hint form-hint--success">{{ successMessage }}</p>
        <p v-if="resetLink" class="form-hint">
          Development reset link:
          <a :href="resetLink" class="auth-inline-link">{{ resetLink }}</a>
        </p>
        <button type="submit" class="button" :disabled="isSubmitting">
          {{ isSubmitting ? "Sending reset link..." : "Send reset link" }}
        </button>
        <RouterLink to="/login" class="auth-inline-link">
          Back to login
        </RouterLink>
      </form>
    </div>
  </section>
</template>
