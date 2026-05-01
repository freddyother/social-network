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

function normalizePublicProfile(profile) {
  if (!profile) {
    return null
  }

  return {
    ...profile,
    avatarUrl: resolveUploadUrl(profile.avatarUrl)
  }
}

function normalizeGroup(group) {
  if (!group) {
    return null
  }

  return {
    ...group,
    creator: normalizeUser(group.creator)
  }
}

function normalizeGroupMember(member) {
  if (!member) {
    return null
  }

  return normalizeUser(member)
}

function normalizeGroupPost(post) {
  if (!post) {
    return null
  }

  return {
    ...post,
    imageUrl: resolveUploadUrl(post.imageUrl),
    media: (post.media || []).map((item) => ({
      ...item,
      url: resolveUploadUrl(item.url)
    })),
    author: normalizeUser(post.author)
  }
}

function normalizeGroupComment(comment) {
  if (!comment) {
    return null
  }

  return {
    ...comment,
    author: normalizeUser(comment.author)
  }
}

function normalizeGroupEvent(event) {
  if (!event) {
    return null
  }

  return {
    ...event,
    creator: normalizeUser(event.creator)
  }
}

export function normalizeGroupMessage(message) {
  if (!message) {
    return null
  }

  return {
    ...message,
    sender: normalizeUser(message.sender)
  }
}

function normalizeGroupJoinRequest(request) {
  if (!request) {
    return null
  }

  return {
    ...request,
    requester: normalizeUser(request.requester)
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

export function checkNicknameAvailability(nickname, signal) {
  const params = new URLSearchParams({ nickname })
  return request(`/auth/nickname-availability?${params.toString()}`, {
    method: "GET",
    signal
  })
}

export function requestPasswordReset(email) {
  return request("/auth/forgot-password", {
    method: "POST",
    body: JSON.stringify({ email })
  })
}

export function resetPassword(details) {
  return request("/auth/reset-password", {
    method: "POST",
    body: JSON.stringify(details)
  })
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

export async function fetchPost(postId) {
  const payload = await request(`/posts/${postId}`, { method: "GET" })
  return payload?.post || null
}

export async function fetchUserProfile(handle) {
  const payload = await request(`/users/${encodeURIComponent(handle)}`, { method: "GET" })

  return {
    profile: normalizePublicProfile(payload?.profile || null),
    posts: payload?.posts || []
  }
}

export async function fetchGroups() {
  const payload = await request("/groups", { method: "GET" })
  return (payload?.groups || []).map(normalizeGroup)
}

export async function fetchGroup(groupId) {
  const payload = await request(`/groups/${groupId}`, { method: "GET" })
  return normalizeGroup(payload?.group || null)
}

export async function fetchGroupMembers(groupId) {
  const payload = await request(`/groups/${groupId}/members`, { method: "GET" })
  return (payload?.members || []).map(normalizeGroupMember).filter(Boolean)
}

export async function createGroup(group) {
  const payload = await request("/groups", {
    method: "POST",
    body: JSON.stringify(group)
  })

  return normalizeGroup(payload?.group || null)
}

export async function joinGroup(groupId) {
  const payload = await request(`/groups/${groupId}/join`, {
    method: "POST"
  })

  return normalizeGroup(payload?.group || null)
}

export async function fetchGroupJoinRequests(groupId) {
  const payload = await request(`/groups/${groupId}/join-requests`, { method: "GET" })
  return (payload?.requests || []).map(normalizeGroupJoinRequest)
}

export async function acceptGroupJoinRequest(groupId, requestId) {
  const payload = await request(`/groups/${groupId}/join-requests/${requestId}/accept`, {
    method: "POST"
  })

  return normalizeGroupJoinRequest(payload?.request || null)
}

export async function declineGroupJoinRequest(groupId, requestId) {
  const payload = await request(`/groups/${groupId}/join-requests/${requestId}/decline`, {
    method: "POST"
  })

  return normalizeGroupJoinRequest(payload?.request || null)
}

export async function fetchGroupPosts(groupId) {
  const payload = await request(`/groups/${groupId}/posts`, { method: "GET" })
  return (payload?.posts || []).map(normalizeGroupPost)
}

export async function setGroupPostReaction(groupId, postId, reaction = "like") {
  const payload = await request(`/groups/${groupId}/posts/${postId}/reaction`, {
    method: "PUT",
    body: JSON.stringify({ reaction })
  })

  return payload?.reaction || null
}

export async function clearGroupPostReaction(groupId, postId) {
  const payload = await request(`/groups/${groupId}/posts/${postId}/reaction`, {
    method: "DELETE"
  })

  return payload?.reaction || null
}

export async function createGroupPost(groupId, post) {
  const formData = new FormData()
  formData.set("body", post.body || "")

  for (const image of post.images || []) {
    formData.append("images", image)
  }

  const payload = await request(`/groups/${groupId}/posts`, {
    method: "POST",
    body: formData
  })

  return normalizeGroupPost(payload?.post || null)
}

export async function fetchGroupComments(groupId, postId) {
  const payload = await request(`/groups/${groupId}/posts/${postId}/comments`, { method: "GET" })
  return (payload?.comments || []).map(normalizeGroupComment)
}

export async function createGroupComment(groupId, postId, comment) {
  const payload = await request(`/groups/${groupId}/posts/${postId}/comments`, {
    method: "POST",
    body: JSON.stringify(comment)
  })

  return normalizeGroupComment(payload?.comment || null)
}

export async function setGroupCommentReaction(groupId, postId, commentId, reaction = "like") {
  const payload = await request(`/groups/${groupId}/posts/${postId}/comments/${commentId}/reaction`, {
    method: "PUT",
    body: JSON.stringify({ reaction })
  })

  return payload?.reaction || null
}

export async function clearGroupCommentReaction(groupId, postId, commentId) {
  const payload = await request(`/groups/${groupId}/posts/${postId}/comments/${commentId}/reaction`, {
    method: "DELETE"
  })

  return payload?.reaction || null
}

export async function fetchGroupEvents(groupId) {
  const payload = await request(`/groups/${groupId}/events`, { method: "GET" })
  return (payload?.events || []).map(normalizeGroupEvent)
}

export async function createGroupEvent(groupId, event) {
  const payload = await request(`/groups/${groupId}/events`, {
    method: "POST",
    body: JSON.stringify(event)
  })

  return normalizeGroupEvent(payload?.event || null)
}

export async function respondToGroupEvent(groupId, eventId, response) {
  const payload = await request(`/groups/${groupId}/events/${eventId}/respond`, {
    method: "POST",
    body: JSON.stringify({ response })
  })

  return normalizeGroupEvent(payload?.event || null)
}

export async function fetchGroupMessages(groupId) {
  const payload = await request(`/groups/${groupId}/messages`, { method: "GET" })
  return (payload?.messages || []).map(normalizeGroupMessage)
}

export async function sendGroupMessage(groupId, message) {
  const payload = await request(`/groups/${groupId}/messages`, {
    method: "POST",
    body: JSON.stringify(message)
  })

  return normalizeGroupMessage(payload?.message || null)
}

export async function fetchGroupInviteCandidates(groupId) {
  const payload = await request(`/groups/${groupId}/invite-candidates`, { method: "GET" })
  return payload?.users || []
}

export async function inviteUserToGroup(groupId, invitation) {
  const payload = await request(`/groups/${groupId}/invite`, {
    method: "POST",
    body: JSON.stringify(invitation)
  })

  return payload?.message || null
}

export async function searchAll(query) {
  const params = new URLSearchParams({ q: query })
  const payload = await request(`/search?${params.toString()}`, { method: "GET" })

  return {
    query: payload?.query || query,
    users: (payload?.users || []).map(normalizeUser),
    posts: payload?.posts || [],
    groups: (payload?.groups || []).map(normalizeGroup)
  }
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

export async function setPostReaction(postId, reaction = "like") {
  const payload = await request(`/posts/${postId}/reaction`, {
    method: "PUT",
    body: JSON.stringify({ reaction })
  })

  return payload?.reaction || null
}

export async function clearPostReaction(postId) {
  const payload = await request(`/posts/${postId}/reaction`, {
    method: "DELETE"
  })

  return payload?.reaction || null
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

export function deletePost(postId) {
  return request(`/posts/${postId}`, { method: "DELETE" })
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

export async function setCommentReaction(postId, commentId, reaction = "like") {
  const payload = await request(`/posts/${postId}/comments/${commentId}/reaction`, {
    method: "PUT",
    body: JSON.stringify({ reaction })
  })

  return payload?.reaction || null
}

export async function clearCommentReaction(postId, commentId) {
  const payload = await request(`/posts/${postId}/comments/${commentId}/reaction`, {
    method: "DELETE"
  })

  return payload?.reaction || null
}

export async function fetchDiscoverUsers() {
  const payload = await request("/users/discover", { method: "GET" })
  return payload?.users || []
}

export function followUser(userId) {
  return request(`/users/${userId}/follow`, { method: "POST" })
}

export function unfollowUser(userId) {
  return request(`/users/${userId}/follow`, { method: "DELETE" })
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
