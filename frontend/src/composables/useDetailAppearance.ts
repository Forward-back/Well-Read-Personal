import { ref } from 'vue'
import {
  loadPreference,
  savePreference,
  safeRemoveItem,
  userKey,
  migratePreferencesIntoUser,
} from './preferenceStorage'

export interface DetailColorPreset {
  value: string
  label: string
}

export const DETAIL_COLOR_PRESETS: DetailColorPreset[] = [
  { value: '#f7f5f0', label: '暖米纸' },
  { value: '#ffffff', label: '纯白' },
  { value: '#f2f3f5', label: '浅灰' },
  { value: '#eef7ee', label: '淡绿' },
]

export const DEFAULT_DETAIL_COLOR = '#f7f5f0'
export const DEFAULT_DETAIL_TRANSPARENCY = 0

// Shown when a level has no override yet. These approximate the built-in
// TDesign grayscale so the picker starts from a sensible value; the tokens
// themselves stay untouched until the owner actually changes a color.
export const DEFAULT_TEXT_COLORS = {
  primary: '#1a1a1a',
  secondary: '#666666',
  placeholder: '#999999',
} as const

export type DetailTextLevel = keyof typeof DEFAULT_TEXT_COLORS

const COLOR_KEY = 'detail_color'
const TRANSPARENCY_KEY = 'detail_transparency'
const TEXT_KEYS: Record<DetailTextLevel, string> = {
  primary: 'detail_text_primary',
  secondary: 'detail_text_secondary',
  placeholder: 'detail_text_placeholder',
}
const HEX_PATTERN = /^#[0-9a-f]{6}$/i

function loadColor(): string {
  const value = loadPreference(COLOR_KEY)
  return value && HEX_PATTERN.test(value) ? value : DEFAULT_DETAIL_COLOR
}

function loadTransparency(): number {
  const value = Number(loadPreference(TRANSPARENCY_KEY))
  if (!Number.isFinite(value)) return DEFAULT_DETAIL_TRANSPARENCY
  return Math.min(100, Math.max(0, Math.round(value)))
}

function loadHexPreference(key: string): string | null {
  const value = loadPreference(key)
  return value && HEX_PATTERN.test(value) ? value.toLowerCase() : null
}

const currentColor = ref(loadColor())
const currentTransparency = ref(loadTransparency())
const textPrimary = ref<string | null>(loadHexPreference(TEXT_KEYS.primary))
const textSecondary = ref<string | null>(loadHexPreference(TEXT_KEYS.secondary))
const textPlaceholder = ref<string | null>(loadHexPreference(TEXT_KEYS.placeholder))

function hexToRgb(hex: string): [number, number, number] {
  return [
    parseInt(hex.slice(1, 3), 16),
    parseInt(hex.slice(3, 5), 16),
    parseInt(hex.slice(5, 7), 16),
  ]
}

// The page canvas is a single translucent layer; cards and text keep TDesign
// container colors so lowering opacity never washes out readable content.
function applyDetailAppearance() {
  const [r, g, b] = hexToRgb(currentColor.value)
  const alpha = Math.round((100 - currentTransparency.value)) / 100
  const root = document.documentElement.style
  root.setProperty('--w-detail-color', currentColor.value)
  root.setProperty('--w-detail-bg', `rgba(${r}, ${g}, ${b}, ${alpha.toFixed(2)})`)

  const textOverrides: Array<[string, string | null]> = [
    ['--w-detail-text-primary', textPrimary.value],
    ['--w-detail-text-secondary', textSecondary.value],
    ['--w-detail-text-placeholder', textPlaceholder.value],
  ]
  for (const [name, value] of textOverrides) {
    if (value) {
      root.setProperty(name, value)
    } else {
      root.removeProperty(name)
    }
  }
}

export function useDetailAppearance() {
  function setDetailColor(value: string): boolean {
    if (!HEX_PATTERN.test(value)) return false
    currentColor.value = value.toLowerCase()
    savePreference(COLOR_KEY, currentColor.value)
    applyDetailAppearance()
    return true
  }

  function setDetailTransparency(value: number): boolean {
    const next = Math.round(Number(value))
    if (!Number.isFinite(next) || next < 0 || next > 100) return false
    currentTransparency.value = next
    savePreference(TRANSPARENCY_KEY, String(next))
    applyDetailAppearance()
    return true
  }

  function setDetailTextColor(level: DetailTextLevel, value: string): boolean {
    if (!HEX_PATTERN.test(value)) return false
    const next = value.toLowerCase()
    if (level === 'primary') textPrimary.value = next
    else if (level === 'secondary') textSecondary.value = next
    else textPlaceholder.value = next
    savePreference(TEXT_KEYS[level], next)
    applyDetailAppearance()
    return true
  }

  function resetDetailAppearance() {
    currentColor.value = DEFAULT_DETAIL_COLOR
    currentTransparency.value = DEFAULT_DETAIL_TRANSPARENCY
    textPrimary.value = null
    textSecondary.value = null
    textPlaceholder.value = null
    savePreference(COLOR_KEY, currentColor.value)
    savePreference(TRANSPARENCY_KEY, String(currentTransparency.value))
    for (const key of Object.values(TEXT_KEYS)) safeRemoveItem(userKey(key))
    applyDetailAppearance()
  }

  return {
    currentColor,
    currentTransparency,
    textPrimary,
    textSecondary,
    textPlaceholder,
    setDetailColor,
    setDetailTransparency,
    setDetailTextColor,
    resetDetailAppearance,
  }
}

/** Call once in main.ts before mount. */
export function initDetailAppearance() {
  currentColor.value = loadColor()
  currentTransparency.value = loadTransparency()
  applyDetailAppearance()
}

/** Re-read preferences after login / logout. */
export function reloadDetailAppearanceFromStorage() {
  migratePreferencesIntoUser()
  currentColor.value = loadColor()
  currentTransparency.value = loadTransparency()
  applyDetailAppearance()
}
