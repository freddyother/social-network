import { computed, reactive } from "vue"

import {
  applyThemePreference,
  DEFAULT_THEME,
  loadStoredThemePreference,
  normalizeThemePreference,
  persistThemePreference
} from "../theme"

const state = reactive({
  apiStatus: "checking",
  meta: null,
  currentUser: null,
  notificationUnreadCount: 0,
  themePreference: loadStoredThemePreference()
})

export function useAppStore() {
  const isAuthenticated = computed(() => Boolean(state.currentUser))

  function setApiStatus(status) {
    state.apiStatus = status
  }

  function setMeta(meta) {
    state.meta = meta
  }

  function setThemePreference(themePreference) {
    const normalized = normalizeThemePreference(themePreference || state.currentUser?.themePreference || DEFAULT_THEME)
    state.themePreference = normalized
    applyThemePreference(normalized)
    persistThemePreference(normalized)
  }

  function setCurrentUser(user) {
    state.currentUser = user
    if (user?.themePreference) {
      setThemePreference(user.themePreference)
    }
  }

  function updateCurrentUser(patch) {
    if (!state.currentUser) {
      return
    }

    state.currentUser = {
      ...state.currentUser,
      ...patch
    }

    if (Object.prototype.hasOwnProperty.call(patch, "themePreference")) {
      setThemePreference(patch.themePreference)
    }
  }

  function setNotificationUnreadCount(count) {
    state.notificationUnreadCount = Math.max(0, Number(count) || 0)
  }

  function clearCurrentUser() {
    state.currentUser = null
    state.notificationUnreadCount = 0
  }

  return {
    state,
    isAuthenticated,
    setApiStatus,
    setMeta,
    setThemePreference,
    setCurrentUser,
    updateCurrentUser,
    setNotificationUnreadCount,
    clearCurrentUser
  }
}
