<script setup>
import { reactive, ref } from "vue"
import { useRouter } from "vue-router"

import SocialAuthButtons from "../components/ui/SocialAuthButtons.vue"
import { isApiError, loginUser } from "../services/api"
import { useAppStore } from "../stores/app"

const router = useRouter()
const store = useAppStore()

const form = reactive({
  identifier: "",
  password: ""
})

const fieldErrors = reactive({
  identifier: "",
  password: ""
})

const formError = ref("")
const isSubmitting = ref(false)

function clearErrors() {
  formError.value = ""
  fieldErrors.identifier = ""
  fieldErrors.password = ""
}

async function handleSubmit() {
  clearErrors()
  isSubmitting.value = true

  try {
    const user = await loginUser({
      identifier: form.identifier,
      password: form.password
    })

    store.setCurrentUser(user)
    await router.push("/feed")
  } catch (error) {
    if (isApiError(error)) {
      formError.value = error.message
      const apiFieldErrors = error.payload?.fields || {}
      fieldErrors.identifier = apiFieldErrors.identifier || apiFieldErrors.email || ""
      fieldErrors.password = apiFieldErrors.password || ""
    } else {
      formError.value = "Could not sign in right now."
    }
  } finally {
    isSubmitting.value = false
  }
}

async function handleSocialAuthenticated(user) {
  store.setCurrentUser(user)
  await router.push("/feed")
}
</script>

<template>
  <section class="page">
    <div class="panel panel--narrow">
      <p class="eyebrow">Auth</p>
      <h2>Login</h2>
      <form class="stack-form" @submit.prevent="handleSubmit">
        <label>
          <span>Nickname or Email</span>
          <input
            v-model.trim="form.identifier"
            type="text"
            autocomplete="username"
            :aria-invalid="Boolean(fieldErrors.identifier)"
            required
          />
          <p v-if="fieldErrors.identifier" class="form-error">{{ fieldErrors.identifier }}</p>
        </label>
        <label>
          <span>Password</span>
          <input
            v-model="form.password"
            type="password"
            placeholder="********"
            autocomplete="current-password"
            :aria-invalid="Boolean(fieldErrors.password)"
            required
          />
          <p v-if="fieldErrors.password" class="form-error">{{ fieldErrors.password }}</p>
        </label>
        <RouterLink to="/forgot-password" class="auth-inline-link auth-inline-link--end">
          Forgot password?
        </RouterLink>
        <p v-if="formError" class="form-error">{{ formError }}</p>
        <button type="submit" class="button" :disabled="isSubmitting">
          {{ isSubmitting ? "Signing in..." : "Sign in" }}
        </button>
      </form>
      <SocialAuthButtons @authenticated="handleSocialAuthenticated" />
    </div>
  </section>
</template>
