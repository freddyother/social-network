import { computed, reactive } from "vue"

const state = reactive({
  apiStatus: "checking",
  meta: null,
  currentUser: null,
  notificationUnreadCount: 0
})

export function useAppStore() {
  const isAuthenticated = computed(() => Boolean(state.currentUser))

  function setApiStatus(status) {
    state.apiStatus = status
  }

  function setMeta(meta) {
    state.meta = meta
  }

  function setCurrentUser(user) {
    state.currentUser = user
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
    setCurrentUser,
    setNotificationUnreadCount,
    clearCurrentUser
  }
}
