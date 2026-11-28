type RGB = [number, number, number]

function parseCSSColor(value: string): RGB {
  const rgb = value.match(/rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)/i)
  if (rgb) return [Number(rgb[1]), Number(rgb[2]), Number(rgb[3])]
  const hex = value.trim().match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i)
  if (hex) {
    const raw = hex[1]
    const full = raw.length === 3 ? raw.split('').map((ch) => ch + ch).join('') : raw
    return [parseInt(full.slice(0, 2), 16), parseInt(full.slice(2, 4), 16), parseInt(full.slice(4, 6), 16)]
  }
  return [0, 0, 0]
}

export function cloneAndTintLottie(source: Record<string, unknown>, cssColor: string): Record<string, unknown> {
  const [red, green, blue] = parseCSSColor(cssColor)
  const color = [red / 255, green / 255, blue / 255]

  const tint = (value: unknown): unknown => {
    if (Array.isArray(value)) return value.map(tint)
    if (!value || typeof value !== 'object') return value
    const record = value as Record<string, unknown>
    const next: Record<string, unknown> = {}
    for (const [key, child] of Object.entries(record)) {
      if ((key === 'c' || key === 'sc' || key === 'fc') && child && typeof child === 'object') {
        const colorProp = child as Record<string, unknown>
        const raw = colorProp.k
        if (colorProp.a === 0 && Array.isArray(raw) && raw.length >= 3) {
          next[key] = {
            ...colorProp,
            k: [color[0], color[1], color[2], typeof raw[3] === 'number' ? raw[3] : 1],
          }
          continue
        }
      }
      next[key] = tint(child)
    }
    return next
  }

  return tint(source) as Record<string, unknown>
}
