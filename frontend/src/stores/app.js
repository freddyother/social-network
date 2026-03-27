import { computed, reactive } from "vue"

const state = reactive({
  apiStatus: "checking",
  meta: null,
  currentUser: null
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

  function clearCurrentUser() {
    state.currentUser = null
  }

  return {
    state,
    isAuthenticated,
    setApiStatus,
    setMeta,
    setCurrentUser,
    clearCurrentUser
  }
}
