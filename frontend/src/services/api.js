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
