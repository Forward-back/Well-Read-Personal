import { reactive, ref } from 'vue'
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

// Primary action colour per feature page. Unset means the shared default;
// the UI settings page writes an override for the pages the owner wants to
// tint differently. Agent defaults to VS Code High Contrast blue.
export const FEATURE_ACCENT_KEYS = ['chat', 'knowledge', 'reading', 'review', 'skills', 'agent'] as const
export type FeatureAccentKey = (typeof FEATURE_ACCENT_KEYS)[number]
export const DEFAULT_FEATURE_ACCENT = '#07c05f'
export const DEFAULT_FEATURE_ACCENTS: Record<FeatureAccentKey, string> = {
  chat: DEFAULT_FEATURE_ACCENT,
  knowledge: DEFAULT_FEATURE_ACCENT,
  reading: DEFAULT_FEATURE_ACCENT,
  review: DEFAULT_FEATURE_ACCENT,
  skills: DEFAULT_FEATURE_ACCENT,
  agent: '#0f4a85',
}

export const AGENT_APPEARANCE_KEYS = ['surface', 'raised', 'border'] as const
export type AgentAppearanceKey = (typeof AGENT_APPEARANCE_KEYS)[number]
export const DEFAULT_AGENT_APPEARANCE: Record<AgentAppearanceKey, string> = {
  surface: '#ffffff',
  raised: '#ffffff',
  border: '#0f4a85',
}

const COLOR_KEY = 'detail_color'
const TRANSPARENCY_KEY = 'detail_transparency'
const TEXT_KEYS: Record<DetailTextLevel, string> = {
  primary: 'detail_text_primary',
  secondary: 'detail_text_secondary',
  placeholder: 'detail_text_placeholder',
}
const FEATURE_ACCENT_SUFFIXES: Record<FeatureAccentKey, string> = {
  chat: 'accent_chat',
  knowledge: 'accent_knowledge',
  reading: 'accent_reading',
  review: 'accent_review',
  skills: 'accent_skills',
  agent: 'accent_agent',
}
const AGENT_APPEARANCE_SUFFIXES: Record<AgentAppearanceKey, string> = {
  surface: 'agent_surface',
  raised: 'agent_raised',
  border: 'agent_border',
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
const featureAccents = reactive<Record<FeatureAccentKey, string | null>>({
  chat: loadHexPreference(FEATURE_ACCENT_SUFFIXES.chat),
  knowledge: loadHexPreference(FEATURE_ACCENT_SUFFIXES.knowledge),
  reading: loadHexPreference(FEATURE_ACCENT_SUFFIXES.reading),
  review: loadHexPreference(FEATURE_ACCENT_SUFFIXES.review),
  skills: loadHexPreference(FEATURE_ACCENT_SUFFIXES.skills),
  agent: loadHexPreference(FEATURE_ACCENT_SUFFIXES.agent),
})
const agentAppearance = reactive<Record<AgentAppearanceKey, string | null>>({
  surface: loadHexPreference(AGENT_APPEARANCE_SUFFIXES.surface),
  raised: loadHexPreference(AGENT_APPEARANCE_SUFFIXES.raised),
  border: loadHexPreference(AGENT_APPEARANCE_SUFFIXES.border),
})

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

  for (const key of FEATURE_ACCENT_KEYS) {
    const value = featureAccents[key]
    if (value) {
      root.setProperty(`--w-accent-${key}`, value)
    } else {
      root.removeProperty(`--w-accent-${key}`)
    }
  }

  for (const key of AGENT_APPEARANCE_KEYS) {
    root.setProperty(`--w-agent-${key}`, agentAppearance[key] || DEFAULT_AGENT_APPEARANCE[key])
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

  function setFeatureAccent(key: FeatureAccentKey, value: string): boolean {
    if (!HEX_PATTERN.test(value)) return false
    featureAccents[key] = value.toLowerCase()
    savePreference(FEATURE_ACCENT_SUFFIXES[key], featureAccents[key] as string)
    applyDetailAppearance()
    return true
  }

  function resetFeatureAccent(key: FeatureAccentKey) {
    featureAccents[key] = null
    safeRemoveItem(userKey(FEATURE_ACCENT_SUFFIXES[key]))
    applyDetailAppearance()
  }

  function setAgentAppearance(key: AgentAppearanceKey, value: string): boolean {
    if (!HEX_PATTERN.test(value)) return false
    agentAppearance[key] = value.toLowerCase()
    savePreference(AGENT_APPEARANCE_SUFFIXES[key], agentAppearance[key] as string)
    applyDetailAppearance()
    return true
  }

  function resetAgentAppearance(key: AgentAppearanceKey) {
    agentAppearance[key] = null
    safeRemoveItem(userKey(AGENT_APPEARANCE_SUFFIXES[key]))
    applyDetailAppearance()
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
    for (const key of FEATURE_ACCENT_KEYS) {
      featureAccents[key] = null
      safeRemoveItem(userKey(FEATURE_ACCENT_SUFFIXES[key]))
    }
    for (const key of AGENT_APPEARANCE_KEYS) {
      agentAppearance[key] = null
      safeRemoveItem(userKey(AGENT_APPEARANCE_SUFFIXES[key]))
    }
    applyDetailAppearance()
  }

  return {
    currentColor,
    currentTransparency,
    textPrimary,
    textSecondary,
    textPlaceholder,
    featureAccents,
    agentAppearance,
    setDetailColor,
    setDetailTransparency,
    setDetailTextColor,
    setFeatureAccent,
    resetFeatureAccent,
    setAgentAppearance,
    resetAgentAppearance,
    resetDetailAppearance,
  }
}

/** Call once in main.ts before mount. */
export function initDetailAppearance() {
  currentColor.value = loadColor()
  currentTransparency.value = loadTransparency()
  loadFeatureAccents()
  loadAgentAppearance()
  applyDetailAppearance()
}

/** Re-read preferences after login / logout. */
export function reloadDetailAppearanceFromStorage() {
  migratePreferencesIntoUser()
  currentColor.value = loadColor()
  currentTransparency.value = loadTransparency()
  loadFeatureAccents()
  loadAgentAppearance()
  applyDetailAppearance()
}

function loadFeatureAccents() {
  for (const key of FEATURE_ACCENT_KEYS) {
    featureAccents[key] = loadHexPreference(FEATURE_ACCENT_SUFFIXES[key])
  }
}

function loadAgentAppearance() {
  for (const key of AGENT_APPEARANCE_KEYS) {
    agentAppearance[key] = loadHexPreference(AGENT_APPEARANCE_SUFFIXES[key])
  }
}
