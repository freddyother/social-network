function pad(value) {
  return String(value).padStart(2, "0")
}

function datePartsFromISODate(value) {
  const match = String(value || "").trim().match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (!match) {
    return null
  }

  const [, year, month, day] = match
  return {
    year,
    month,
    day
  }
}

function isValidDate(year, month, day) {
  const parsed = new Date(Number(year), Number(month) - 1, Number(day))
  return (
    !Number.isNaN(parsed.getTime()) &&
    parsed.getFullYear() === Number(year) &&
    parsed.getMonth() === Number(month) - 1 &&
    parsed.getDate() === Number(day)
  )
}

function parseDate(value) {
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? null : parsed
}

export function formatDate(value) {
  const isoParts = datePartsFromISODate(value)
  if (isoParts) {
    return `${isoParts.day}/${isoParts.month}/${isoParts.year}`
  }

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return String(value || "")
  }

  return `${pad(parsed.getDate())}/${pad(parsed.getMonth() + 1)}/${parsed.getFullYear()}`
}

export function formatRelativeTime(value, reference = Date.now()) {
  const parsed = parseDate(value)
  if (!parsed) {
    return String(value || "")
  }

  const referenceDate = parseDate(reference) || new Date()
  const diffMs = Math.max(0, referenceDate.getTime() - parsed.getTime())
  const minute = 60 * 1000
  const hour = 60 * minute
  const day = 24 * hour
  const week = 7 * day
  const month = 30 * day
  const year = 365 * day

  if (diffMs < minute) {
    return "now"
  }

  if (diffMs < hour) {
    return `${Math.floor(diffMs / minute)}m`
  }

  if (diffMs < day) {
    return `${Math.floor(diffMs / hour)}h`
  }

  if (diffMs < week) {
    return `${Math.floor(diffMs / day)}d`
  }

  if (diffMs < month) {
    return `${Math.floor(diffMs / week)}w`
  }

  if (diffMs < year) {
    return `${Math.floor(diffMs / month)}m`
  }

  return `${Math.floor(diffMs / year)}y`
}

export function formatDateTime(value) {
  const parsed = parseDate(value)
  if (!parsed) {
    return formatDate(value)
  }

  return `${formatDate(value)} ${pad(parsed.getHours())}:${pad(parsed.getMinutes())}`
}

export function formatTime(value) {
  const parsed = parseDate(value)
  if (!parsed) {
    return ""
  }

  return `${pad(parsed.getHours())}:${pad(parsed.getMinutes())}`
}

export function toISODateInput(value) {
  const normalized = String(value || "").trim()
  if (!normalized) {
    return ""
  }

  const isoParts = datePartsFromISODate(normalized)
  if (isoParts) {
    return isValidDate(isoParts.year, isoParts.month, isoParts.day)
      ? `${isoParts.year}-${isoParts.month}-${isoParts.day}`
      : ""
  }

  const displayMatch = normalized.match(/^(\d{1,2})\/(\d{1,2})\/(\d{4})$/)
  if (!displayMatch) {
    return ""
  }

  const [, day, month, year] = displayMatch
  if (!isValidDate(year, month, day)) {
    return ""
  }

  return `${year}-${pad(month)}-${pad(day)}`
}
