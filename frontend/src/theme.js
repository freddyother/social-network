export const DEFAULT_THEME = "nexo-blue"

export const THEME_OPTIONS = [
  {
    value: "nexo-blue",
    label: "NEXO Blue",
    description: "Electric blue and cyan over graphite black.",
    swatches: ["#05070b", "#11141a", "#18b6ff", "#5bc8ff"]
  },
  {
    value: "nexo-ice",
    label: "NEXO Ice",
    description: "Cool cyan highlights with a crisp night backdrop.",
    swatches: ["#04070b", "#10161d", "#63e6ff", "#9be8ff"]
  },
  {
    value: "graphite-gold",
    label: "Graphite Gold",
    description: "Soft gold accents for a warmer premium look.",
    swatches: ["#050403", "#161513", "#d4a84f", "#f2c14e"]
  },
  {
    value: "nexo-cloud",
    label: "NEXO Cloud",
    description: "Soft misty blues with a clean daylight canvas.",
    swatches: ["#edf4fa", "#f5faff", "#2f8dff", "#58d6f5"]
  },
  {
    value: "nexo-harbor",
    label: "NEXO Harbor",
    description: "Muted blue-grays for a calm premium look without black.",
    swatches: ["#cfdbe8", "#344760", "#38a7ff", "#63d9ef"]
  }
]

const THEME_VALUES = new Set(THEME_OPTIONS.map((option) => option.value))
const STORAGE_KEY = "nexo.themePreference"

export function normalizeThemePreference(themePreference) {
  if (typeof themePreference !== "string") {
    return DEFAULT_THEME
  }

  const normalized = themePreference.trim().toLowerCase()
  return THEME_VALUES.has(normalized) ? normalized : DEFAULT_THEME
}

export function applyThemePreference(themePreference) {
  const normalized = normalizeThemePreference(themePreference)
  if (typeof document !== "undefined") {
    document.documentElement.dataset.theme = normalized
  }

  return normalized
}

export function loadStoredThemePreference() {
  if (typeof window === "undefined") {
    return DEFAULT_THEME
  }

  return normalizeThemePreference(window.localStorage.getItem(STORAGE_KEY))
}

export function persistThemePreference(themePreference) {
  const normalized = normalizeThemePreference(themePreference)
  if (typeof window !== "undefined") {
    window.localStorage.setItem(STORAGE_KEY, normalized)
  }

  return normalized
}
