import assert from 'node:assert/strict'
import { test } from 'node:test'

import { BUILT_IN_DEFAULT, resolveDefaultLocale } from './resolveDefaultLocale.ts'

test('resolveDefaultLocale accepts zh-CN and en-US only', () => {
  assert.equal(resolveDefaultLocale('zh-CN', undefined), 'zh-CN')
  assert.equal(resolveDefaultLocale('en-US', undefined), 'en-US')
  assert.equal(resolveDefaultLocale('', 'en-US'), 'en-US')
  assert.equal(resolveDefaultLocale(undefined, undefined), BUILT_IN_DEFAULT)
})

test('resolveDefaultLocale rejects other and unknown values', () => {
  assert.equal(resolveDefaultLocale('ru-RU', 'zh-CN'), BUILT_IN_DEFAULT)
  assert.equal(resolveDefaultLocale('fr-FR'), BUILT_IN_DEFAULT)
  assert.equal(resolveDefaultLocale('en-US"};alert(1);//'), BUILT_IN_DEFAULT)
  assert.equal(resolveDefaultLocale('   '), BUILT_IN_DEFAULT)
})
