<script setup>
import { computed, onMounted, ref } from "vue"

import { isApiError, oauthLogin } from "../../services/api"

const emit = defineEmits(["authenticated"])

const googleButton = ref(null)
const socialError = ref("")
const isGoogleReady = ref(false)
const isAppleLoading = ref(false)

const rawGoogleClientId = import.meta.env.VITE_GOOGLE_CLIENT_ID || ""
const rawAppleClientId = import.meta.env.VITE_APPLE_CLIENT_ID || ""
const googleClientId = normalizeGoogleClientId(rawGoogleClientId)
const appleClientId = normalizeAppleClientId(rawAppleClientId)
const appleRedirectURI = import.meta.env.VITE_APPLE_REDIRECT_URI || window.location.origin

const hasAnyProvider = computed(() => Boolean(googleClientId || appleClientId))
const oauthConfigHint = computed(() => {
  if (String(rawGoogleClientId).trim() && !googleClientId) {
    return "Google sign-in needs a real Web client ID ending in .apps.googleusercontent.com."
  }

  if (String(rawAppleClientId).trim() && !appleClientId) {
    return "Apple sign-in needs a real Services ID."
  }

  return ""
})

const scriptPromises = new Map()

function normalizeOAuthClientId(value) {
  const normalized = String(value || "").trim()
  const placeholderValues = new Set([
    "...",
    "changeme",
    "change-me",
    "replace-me",
    "todo",
    "your-client-id",
    "your-google-client-id",
    "your-apple-client-id"
  ])

  if (!normalized || placeholderValues.has(normalized.toLowerCase())) {
    return ""
  }

  return normalized
}

function normalizeGoogleClientId(value) {
  const normalized = normalizeOAuthClientId(value)
  if (!normalized || !normalized.toLowerCase().endsWith(".apps.googleusercontent.com")) {
    return ""
  }

  return normalized
}

function normalizeAppleClientId(value) {
  return normalizeOAuthClientId(value)
}

function loadScript(src) {
  if (scriptPromises.has(src)) {
    return scriptPromises.get(src)
  }

  const existingScript = document.querySelector(`script[src="${src}"]`)
  if (existingScript) {
    const promise = existingScript.dataset.loaded === "true"
      ? Promise.resolve()
      : new Promise((resolve, reject) => {
          existingScript.addEventListener("load", resolve, { once: true })
          existingScript.addEventListener("error", reject, { once: true })
        })

    scriptPromises.set(src, promise)
    return promise
  }

  const promise = new Promise((resolve, reject) => {
    const script = document.createElement("script")
    script.src = src
    script.async = true
    script.defer = true
    script.addEventListener("load", () => {
      script.dataset.loaded = "true"
      resolve()
    }, { once: true })
    script.addEventListener("error", reject, { once: true })
    document.head.appendChild(script)
  })

  scriptPromises.set(src, promise)
  return promise
}

function providerErrorMessage(error, fallback) {
  if (isApiError(error)) {
    return error.message
  }

  return fallback
}

async function completeOAuthLogin(details) {
  socialError.value = ""
  const user = await oauthLogin(details)
  emit("authenticated", user)
}

async function handleGoogleCredential(response) {
  const credential = response?.credential || ""
  if (!credential) {
    socialError.value = "Google did not return a sign-in token."
    return
  }

  try {
    await completeOAuthLogin({
      provider: "google",
      idToken: credential
    })
  } catch (error) {
    socialError.value = providerErrorMessage(error, "Could not continue with Google right now.")
  }
}

async function renderGoogleButton() {
  if (!googleClientId || !googleButton.value) {
    return
  }

  try {
    if (!window.google?.accounts?.id) {
      await loadScript("https://accounts.google.com/gsi/client")
    }

    window.google?.accounts?.id?.initialize({
      client_id: googleClientId,
      callback: handleGoogleCredential,
      ux_mode: "popup"
    })
    googleButton.value.innerHTML = ""
    window.google?.accounts?.id?.renderButton(googleButton.value, {
      type: "standard",
      theme: "outline",
      size: "large",
      shape: "pill",
      text: "continue_with",
      width: 280
    })
    isGoogleReady.value = true
  } catch (error) {
    socialError.value = "Google sign-in could not be loaded."
  }
}

async function handleAppleSignIn() {
  if (!appleClientId || isAppleLoading.value) {
    return
  }

  isAppleLoading.value = true
  socialError.value = ""

  try {
    if (!window.AppleID?.auth) {
      await loadScript("https://appleid.cdn-apple.com/appleauth/static/jsapi/appleid/1/en_US/appleid.auth.js")
    }

    window.AppleID?.auth?.init({
      clientId: appleClientId,
      scope: "name email",
      redirectURI: appleRedirectURI,
      usePopup: true
    })

    const response = await window.AppleID.auth.signIn()
    const idToken = response?.authorization?.id_token || ""
    if (!idToken) {
      socialError.value = "Apple did not return a sign-in token."
      return
    }

    await completeOAuthLogin({
      provider: "apple",
      idToken,
      firstName: response?.user?.name?.firstName || "",
      lastName: response?.user?.name?.lastName || ""
    })
  } catch (error) {
    if (error?.error === "popup_closed_by_user" || error?.error === "user_cancelled_authorize") {
      socialError.value = ""
      return
    }

    socialError.value = providerErrorMessage(error, "Could not continue with Apple right now.")
  } finally {
    isAppleLoading.value = false
  }
}

onMounted(() => {
  void renderGoogleButton()
})
</script>

<template>
  <div class="social-auth">
    <div v-if="hasAnyProvider" class="social-auth__divider">
      <span>or</span>
    </div>

    <div v-if="hasAnyProvider" class="social-auth__buttons">
      <div v-if="googleClientId" class="social-auth__google-wrap">
        <div ref="googleButton" class="social-auth__google"></div>
        <button v-if="!isGoogleReady" type="button" class="social-auth__button" disabled>
          Loading Google...
        </button>
      </div>

      <button
        v-if="appleClientId"
        type="button"
        class="social-auth__button social-auth__button--apple"
        :disabled="isAppleLoading"
        @click="handleAppleSignIn"
      >
        {{ isAppleLoading ? "Opening Apple..." : "Continue with Apple" }}
      </button>
    </div>

    <p v-if="socialError" class="form-error">{{ socialError }}</p>
    <p v-else-if="oauthConfigHint" class="form-hint form-hint--muted">{{ oauthConfigHint }}</p>
  </div>
</template>
