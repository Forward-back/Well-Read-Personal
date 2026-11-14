import assert from 'node:assert/strict'
import test from 'node:test'
import { extractReviewSelfTests } from './reviewSelfTest.ts'

test('extracts marked self-test blocks', () => {
  const content = `### 自测 2 题
<!--REVIEW_SELF_TEST_START-->
1. 题目：什么是栈？
答案：后进先出。
2. 题目：什么是队列？
答案：先进先出。
<!--REVIEW_SELF_TEST_END-->`
  assert.deepEqual(extractReviewSelfTests(content), [
    { question: '什么是栈？', answer: '后进先出。' },
    { question: '什么是队列？', answer: '先进先出。' },
  ])
})

test('falls back to the section after 自测', () => {
  const content = `**自测 2 题**
1. 什么是 TCP？
答案：面向连接、可靠的传输层协议。
2. 什么是 UDP？
答案：无连接、不保证可靠的传输层协议。`
  assert.deepEqual(extractReviewSelfTests(content), [
    { question: '什么是 TCP？', answer: '面向连接、可靠的传输层协议。' },
    { question: '什么是 UDP？', answer: '无连接、不保证可靠的传输层协议。' },
  ])
})

test('ignores answers without a question', () => {
  assert.deepEqual(extractReviewSelfTests('答案：没有题目'), [])
})
