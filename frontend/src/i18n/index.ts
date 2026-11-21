import { createI18n } from 'vue-i18n'
import zhCN from './locales/zh-CN.ts'
import enUS from './locales/en-US.ts'
import enUSExtra from './locales/en-US-extra.ts'
import { BUILT_IN_DEFAULT, resolveDefaultLocale } from './resolveDefaultLocale.ts'

// Personal-edition strings added after the historical en-US locale live in a
// flat dot-path supplement; expand and merge them without mutating the base.
function expandFlatKeys(flat: Record<string, string>): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const [path, value] of Object.entries(flat)) {
    const parts = path.split('.')
    let cursor = out
    for (let i = 0; i < parts.length - 1; i += 1) {
      const part = parts[i]!
      const next = cursor[part]
      cursor = (next && typeof next === 'object' ? next : (cursor[part] = {})) as Record<string, unknown>
    }
    cursor[parts[parts.length - 1]!] = value
  }
  return out
}

function deepMerge(base: Record<string, unknown>, extra: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = { ...base }
  for (const [key, value] of Object.entries(extra)) {
    const current = out[key]
    out[key] =
      value && typeof value === 'object' && current && typeof current === 'object'
        ? deepMerge(current as Record<string, unknown>, value as Record<string, unknown>)
        : value
  }
  return out
}

const enMessages = deepMerge(
  enUS as unknown as Record<string, unknown>,
  expandFlatKeys(enUSExtra as Record<string, string>),
) as unknown as typeof zhCN

const messages = {
  'zh-CN': zhCN,
  'en-US': enMessages,
}

// User's explicit past choice wins; otherwise use the deployment default.
const savedLocale = resolveDefaultLocale(
  localStorage.getItem('locale'),
  window.__RUNTIME_CONFIG__?.DEFAULT_LOCALE || import.meta.env.VITE_DEFAULT_LOCALE,
)

const i18n = createI18n({
  legacy: false,
  locale: savedLocale,
  fallbackLocale: BUILT_IN_DEFAULT,
  globalInjection: true,
  // Some translations intentionally embed `<strong>` markup (e.g. agent step summaries).
  // We render them via v-html with our own sanitization, so silence vue-i18n's HTML warning
  // to avoid flooding the console and slowing renders during history loads.
  warnHtmlMessage: false,
  messages
})

export default i18n
