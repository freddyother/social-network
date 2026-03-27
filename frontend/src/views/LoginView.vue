<script setup>
import { reactive, ref } from "vue"
import { useRouter } from "vue-router"

import { isApiError, loginUser } from "../services/api"
import { useAppStore } from "../stores/app"

const router = useRouter()
const store = useAppStore()

const form = reactive({
  email: "",
  password: ""
})

const fieldErrors = reactive({
  email: "",
  password: ""
})

const formError = ref("")
const isSubmitting = ref(false)

function clearErrors() {
  formError.value = ""
  fieldErrors.email = ""
  fieldErrors.password = ""
}

async function handleSubmit() {
  clearErrors()
  isSubmitting.value = true

  try {
    const user = await loginUser({
      email: form.email,
      password: form.password
    })

    store.setCurrentUser(user)
    await router.push("/feed")
  } catch (error) {
    if (isApiError(error)) {
      formError.value = error.message
      const apiFieldErrors = error.payload?.fields || {}
      fieldErrors.email = apiFieldErrors.email || ""
      fieldErrors.password = apiFieldErrors.password || ""
    } else {
      formError.value = "Could not sign in right now."
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
      <h2>Login</h2>
      <form class="stack-form" @submit.prevent="handleSubmit">
        <label>
          <span>Email</span>
          <input
            v-model.trim="form.email"
            type="email"
            placeholder="you@example.com"
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
            autocomplete="current-password"
            :aria-invalid="Boolean(fieldErrors.password)"
            required
          />
          <p v-if="fieldErrors.password" class="form-error">{{ fieldErrors.password }}</p>
        </label>
        <p v-if="formError" class="form-error">{{ formError }}</p>
        <button type="submit" class="button" :disabled="isSubmitting">
          {{ isSubmitting ? "Signing in..." : "Sign in" }}
        </button>
      </form>
    </div>
  </section>
</template>
