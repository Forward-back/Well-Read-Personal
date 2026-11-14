import assert from 'node:assert/strict'
import test from 'node:test'
import { cleanReviewExcerpt } from './reviewExcerpt.ts'

test('removes images and keeps link text', () => {
  assert.equal(
    cleanReviewExcerpt('## 标题\n![截图](../../images/a.png)\n见 [文档](https://example.com)'),
    '标题\n\n见 文档',
  )
})

test('removes list, quote, code markers and emphasis', () => {
  assert.equal(
    cleanReviewExcerpt('> 引用\n- **重点**\n```md\n代码\n```'),
    '引用\n重点\n\n代码',
  )
})

test('returns empty for empty input', () => {
  assert.equal(cleanReviewExcerpt(''), '')
  assert.equal(cleanReviewExcerpt(null), '')
})
