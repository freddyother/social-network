const apiBaseUrl = (import.meta.env.VITE_API_BASE_URL || "http://localhost:8080/api/v1").replace(/\/$/, "")

function buildUrl(path) {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`
  return `${apiBaseUrl}${normalizedPath}`
}

async function request(path, options = {}) {
  const response = await fetch(buildUrl(path), {
    credentials: "include",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      ...(options.headers || {})
    },
    ...options
  })

  const contentType = response.headers.get("content-type") || ""
  const payload = contentType.includes("application/json") ? await response.json() : await response.text()

  if (!response.ok) {
    const message = typeof payload === "string" ? payload : payload.error || "Request failed"
    throw new Error(message)
  }

  return payload
}

export function fetchHealth() {
  return request("/health", { method: "GET" })
}

export function fetchMeta() {
  return request("/meta", { method: "GET" })
}
