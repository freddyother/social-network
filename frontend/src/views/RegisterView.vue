<script setup>
import { reactive, ref } from "vue"
import { useRouter } from "vue-router"

import { isApiError, registerUser } from "../services/api"
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

function clearErrors() {
  formError.value = ""

  Object.keys(fieldErrors).forEach((key) => {
    fieldErrors[key] = ""
  })
}

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
            pattern="(?:\\d{4}-\\d{2}-\\d{2}|\\d{1,2}/\\d{1,2}/\\d{4})"
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
            placeholder="Optional"
            :aria-invalid="Boolean(fieldErrors.nickname)"
          />
          <p v-if="fieldErrors.nickname" class="form-error">{{ fieldErrors.nickname }}</p>
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
        <button type="submit" class="button form-grid__full" :disabled="isSubmitting">
          {{ isSubmitting ? "Creating account..." : "Create account" }}
        </button>
      </form>
    </div>
  </section>
</template>
