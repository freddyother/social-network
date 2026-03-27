const apiBaseUrl = (import.meta.env.VITE_API_BASE_URL || "http://localhost:8080/api/v1").replace(/\/$/, "")

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
  return payload?.user || null
}

export async function loginUser(credentials) {
  const payload = await request("/auth/login", {
    method: "POST",
    body: JSON.stringify(credentials)
  })

  return payload?.user || null
}

export async function registerUser(details) {
  const payload = await request("/auth/register", {
    method: "POST",
    body: JSON.stringify(details)
  })

  return payload?.user || null
}

export function logoutUser() {
  return request("/auth/logout", { method: "POST" })
}

export async function fetchFeed() {
  const payload = await request("/posts", { method: "GET" })
  return payload?.posts || []
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

export async function createComment(postId, comment) {
  const payload = await request(`/posts/${postId}/comments`, {
    method: "POST",
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

  return payload?.user || null
}

export async function fetchNotifications() {
  const payload = await request("/notifications", { method: "GET" })
  return payload?.notifications || []
}

export function markNotificationRead(notificationId) {
  return request(`/notifications/${notificationId}/read`, { method: "POST" })
}
