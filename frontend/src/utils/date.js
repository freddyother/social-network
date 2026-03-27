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

export function formatDateTime(value) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return formatDate(value)
  }

  return `${formatDate(value)} ${pad(parsed.getHours())}:${pad(parsed.getMinutes())}`
}

export function formatTime(value) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
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

  const displayMatch = normalized.match(/^(\d{2})\/(\d{2})\/(\d{4})$/)
  if (!displayMatch) {
    return ""
  }

  const [, day, month, year] = displayMatch
  if (!isValidDate(year, month, day)) {
    return ""
  }

  return `${year}-${month}-${day}`
}
