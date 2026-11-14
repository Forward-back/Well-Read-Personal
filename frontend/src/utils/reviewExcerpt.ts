export function cleanReviewExcerpt(text: string | null | undefined): string {
  let cleaned = String(text ?? '').trim()
  cleaned = cleaned.replace(/<!--[\s\S]*?-->/g, '')
  cleaned = cleaned.replace(/!\[[^\]]*\]\([^)]*\)/g, '')
  cleaned = cleaned.replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
  cleaned = cleaned.replace(/^\s*```.*$/gm, '')
  cleaned = cleaned.replace(/^\s{0,3}#{1,6}\s*/gm, '')
  cleaned = cleaned.replace(/^\s*[-*+]\s+/gm, '')
  cleaned = cleaned.replace(/^\s*>\s?/gm, '')
  cleaned = cleaned.replace(/<[^>]+>/g, '')
  cleaned = cleaned.replace(/\*\*/g, '')
  cleaned = cleaned.replace(/`/g, '')
  cleaned = cleaned.replace(/\n{3,}/g, '\n\n').trim()
  return cleaned.length > 500 ? `${cleaned.slice(0, 500)}…` : cleaned
}
