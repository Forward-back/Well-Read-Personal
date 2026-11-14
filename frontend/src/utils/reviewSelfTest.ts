export interface ReviewSelfTestCandidate {
  question: string
  answer: string
}

const MARKER_START = /<!--\s*REVIEW_SELF_TEST_START\s*-->/i
const MARKER_END = /<!--\s*REVIEW_SELF_TEST_END\s*-->/i

function cleanLine(line: string): string {
  return line
    .trim()
    .replace(/^[-*]\s+/, '')
    .replace(/^\d+[.、)]\s+/, '')
    .replace(/^\*\*|\*\*$/g, '')
    .replace(/^题目[:：]\s*/, '')
    .trim()
}

function sectionAfterSelfTest(content: string): string {
  const lines = content.split(/\r?\n/)
  const start = lines.findIndex(line => line.includes('自测'))
  if (start < 0) return ''
  return lines.slice(start + 1).join('\n')
}

function parseBlock(block: string): ReviewSelfTestCandidate[] {
  const candidates: ReviewSelfTestCandidate[] = []
  let current: ReviewSelfTestCandidate | null = null
  const pushCurrent = () => {
    if (current && current.question && current.answer) {
      candidates.push(current)
    }
    current = null
  }
  for (const rawLine of block.split(/\r?\n/)) {
    const line = rawLine.trim()
    if (!line) continue
    if (line.includes('自测') || line.includes('REVIEW_SELF_TEST_')) continue
    const answerMatch = line.match(/^(?:\*\*)?(?:答案|答)(?:\*\*)?[:：]\s*(.+)$/)
    if (answerMatch) {
      if (current) {
        current.answer = cleanLine(answerMatch[1])
        pushCurrent()
      }
      continue
    }
    const questionMatch = line.match(/^(?:\*\*)?(?:题目)(?:\*\*)?[:：]\s*(.+)$/) ||
      line.match(/^\d+[.、)]\s+(.+)$/) ||
      line.match(/^[-*]\s+(.+)$/)
    if (questionMatch) {
      pushCurrent()
      current = { question: cleanLine(questionMatch[1]), answer: '' }
    }
  }
  pushCurrent()
  return candidates.slice(0, 3)
}

/**
 * Extract self-test question/answer pairs from a review answer. The primary
 * format is the hidden marker block emitted by the review prompt; older
 * answers fall back to the section after "自测".
 */
export function extractReviewSelfTests(content: string): ReviewSelfTestCandidate[] {
  if (!content) return []
  const start = content.search(MARKER_START)
  if (start >= 0) {
    const afterStart = content.slice(start).replace(MARKER_START, '')
    const end = afterStart.search(MARKER_END)
    const block = end >= 0 ? afterStart.slice(0, end) : afterStart
    const parsed = parseBlock(block)
    if (parsed.length) return parsed
  }
  return parseBlock(sectionAfterSelfTest(content))
}
