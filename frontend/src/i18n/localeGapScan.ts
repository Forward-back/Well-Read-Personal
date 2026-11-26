import zhCN from './locales/zh-CN.ts'
import enUS from './locales/en-US.ts'
import enUSExtra from './locales/en-US-extra.ts'

type Dict = Record<string, unknown>

function expandFlatKeys(flat: Record<string, string>): Dict {
  const out: Dict = {}
  for (const [path, value] of Object.entries(flat)) {
    const parts = path.split('.')
    let cursor = out
    for (let i = 0; i < parts.length - 1; i += 1) {
      const part = parts[i]!
      const next = cursor[part]
      cursor = (next && typeof next === 'object' ? next : (cursor[part] = {})) as Dict
    }
    cursor[parts[parts.length - 1]!] = value
  }
  return out
}

function deepMerge(base: Dict, extra: Dict): Dict {
  const out: Dict = { ...base }
  for (const [key, value] of Object.entries(extra)) {
    const current = out[key]
    out[key] =
      value && typeof value === 'object' && current && typeof current === 'object'
        ? deepMerge(current as Dict, value as Dict)
        : value
  }
  return out
}

function flatten(obj: Dict, prefix = ''): string[] {
  const keys: string[] = []
  for (const [key, value] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (value && typeof value === 'object') keys.push(...flatten(value as Dict, path))
    else keys.push(path)
  }
  return keys
}

const enMessages = deepMerge(
  enUS as unknown as Dict,
  expandFlatKeys(enUSExtra as unknown as Record<string, string>),
)
const zhKeys = new Set(flatten(zhCN as unknown as Dict))
const enKeys = new Set(flatten(enMessages))
const missing = [...zhKeys].filter((key) => !enKeys.has(key)).sort()
const extra = [...enKeys].filter((key) => !zhKeys.has(key)).sort()

console.log(`Missing en-US keys: ${missing.length}`)
for (const key of missing) console.log(`  ${key}`)

if (extra.length) {
  console.log(`en-US-only keys (enterprise leftovers): ${extra.length}`)
  if (process.argv.includes('--extra')) {
    for (const key of extra) console.log(`  ${key}`)
  }
}

if (missing.length) process.exitCode = 1
