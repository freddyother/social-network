export const apiBaseUrl = (import.meta.env.VITE_API_BASE_URL || "http://localhost:8080/api/v1").replace(/\/$/, "")
const uploadsBaseUrl = apiBaseUrl.replace(/\/api\/v\d+$/, "")

export class ApiError extends Error {
  constructor(message, status, payload) {
    super(message)
    this.name = "ApiError"
    this.status = status
    this.payload = payload
  }
}

function buildUrl(path) {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`
  return `${apiBaseUrl}${normalizedPath}`
}

function resolveUploadUrl(path) {
  if (!path) {
    return ""
  }

  if (/^https?:\/\//i.test(path)) {
    return path
  }

  const normalizedPath = path.replace(/^\/+/, "")
  return `${uploadsBaseUrl}/uploads/${normalizedPath}`
}

function normalizeUser(user) {
  if (!user) {
    return null
  }

  return {
    ...user,
    avatarUrl: resolveUploadUrl(user.avatarUrl)
  }
}

function normalizeConversation(conversation) {
  if (!conversation) {
    return null
  }

  return {
    ...conversation,
    user: normalizeUser(conversation.user)
  }
}

async function request(path, options = {}) {
  const headers = new Headers({
    Accept: "application/json",
    ...(options.headers || {})
  })

  if (options.body && !(options.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json")
  }

  const response = await fetch(buildUrl(path), {
    credentials: "include",
    headers,
    ...options
  })

  let payload = null
  if (response.status !== 204) {
    const contentType = response.headers.get("content-type") || ""
    if (contentType.includes("application/json")) {
      payload = await response.json()
    } else {
      const text = await response.text()
      payload = text || null
    }
  }

  if (!response.ok) {
    const message = typeof payload === "string" ? payload : payload?.error || "Request failed"
    throw new ApiError(message, response.status, payload)
  }

  return payload
}

export function isApiError(error, status) {
  return error instanceof ApiError && (status === undefined || error.status === status)
}

export function fetchHealth() {
  return request("/health", { method: "GET" })
}

export function fetchMeta() {
  return request("/meta", { method: "GET" })
}

export async function fetchCurrentUser() {
  const payload = await request("/auth/me", { method: "GET" })
  return normalizeUser(payload?.user || null)
}

export async function loginUser(credentials) {
  const payload = await request("/auth/login", {
    method: "POST",
    body: JSON.stringify(credentials)
  })

  return normalizeUser(payload?.user || null)
}

export async function registerUser(details) {
  const payload = await request("/auth/register", {
    method: "POST",
    body: JSON.stringify(details)
  })

  return normalizeUser(payload?.user || null)
}

export function logoutUser() {
  return request("/auth/logout", { method: "POST" })
}

export async function fetchFeed() {
  const payload = await request("/posts", { method: "GET" })
  return payload?.posts || []
}

export async function fetchMyPosts() {
  const payload = await request("/posts/mine", { method: "GET" })
  return payload?.posts || []
}

export async function fetchChatConversations() {
  const payload = await request("/chat/conversations", { method: "GET" })
  return (payload?.conversations || []).map(normalizeConversation)
}

export async function fetchConversation(userId) {
  const payload = await request(`/chat/conversations/${userId}/messages`, { method: "GET" })

  return {
    user: normalizeUser(payload?.user || null),
    messages: payload?.messages || []
  }
}

export async function sendPrivateMessage(userId, message) {
  const payload = await request(`/chat/conversations/${userId}/messages`, {
    method: "POST",
    body: JSON.stringify(message)
  })

  return payload?.message || null
}

export async function markConversationRead(userId) {
  const payload = await request(`/chat/conversations/${userId}/read`, {
    method: "POST"
  })

  return payload?.conversation || {
    conversationUserId: userId,
    messageIds: [],
    readAt: null
  }
}

export async function fetchComments(postId) {
  const payload = await request(`/posts/${postId}/comments`, { method: "GET" })
  return payload?.comments || []
}

export async function createPost(post) {
  const formData = new FormData()
  formData.set("title", post.title)
  formData.set("body", post.body)
  formData.set("privacy", post.privacy)

  for (const image of post.images || []) {
    formData.append("images", image)
  }

  const payload = await request("/posts", {
    method: "POST",
    body: formData
  })

  return payload?.post || null
}

export async function updatePost(postId, post) {
  const payload = await request(`/posts/${postId}`, {
    method: "PATCH",
    body: JSON.stringify(post)
  })

  return payload?.post || null
}

export async function createComment(postId, comment) {
  const payload = await request(`/posts/${postId}/comments`, {
    method: "POST",
    body: JSON.stringify(comment)
  })

  return payload?.comment || null
}

export async function updateComment(postId, commentId, comment) {
  const payload = await request(`/posts/${postId}/comments/${commentId}`, {
    method: "PATCH",
    body: JSON.stringify(comment)
  })

  return payload?.comment || null
}

export async function fetchDiscoverUsers() {
  const payload = await request("/users/discover", { method: "GET" })
  return payload?.users || []
}

export function followUser(userId) {
  return request(`/users/${userId}/follow`, { method: "POST" })
}

export async function fetchFollowRequests() {
  const payload = await request("/follow-requests", { method: "GET" })
  return payload?.requests || []
}

export function acceptFollowRequest(requestId) {
  return request(`/follow-requests/${requestId}/accept`, { method: "POST" })
}

export function declineFollowRequest(requestId) {
  return request(`/follow-requests/${requestId}/decline`, { method: "POST" })
}

export async function updateProfileVisibility(visibility) {
  const payload = await request("/users/me/profile-visibility", {
    method: "PATCH",
    body: JSON.stringify({ visibility })
  })

  return normalizeUser(payload?.user || null)
}

export async function updateProfile(profile) {
  const payload = await request("/users/me/profile", {
    method: "PATCH",
    body: JSON.stringify(profile)
  })

  return normalizeUser(payload?.user || null)
}

export async function updateThemePreference(themePreference) {
  const payload = await request("/users/me/theme-preference", {
    method: "PATCH",
    body: JSON.stringify({ themePreference })
  })

  return normalizeUser(payload?.user || null)
}

export async function uploadProfileAvatar(file) {
  const formData = new FormData()
  formData.set("avatar", file)

  const payload = await request("/users/me/avatar", {
    method: "POST",
    body: formData
  })

  return normalizeUser(payload?.user || null)
}

export async function fetchNotifications() {
  const payload = await request("/notifications", { method: "GET" })
  return payload?.notifications || []
}

export function markNotificationRead(notificationId) {
  return request(`/notifications/${notificationId}/read`, { method: "POST" })
}
