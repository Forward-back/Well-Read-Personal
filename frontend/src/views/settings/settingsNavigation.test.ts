import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import { computed } from 'vue'

// Exercise the actual sidebar definition without mounting the settings panels.
const source = readFileSync(new URL('./Settings.vue', import.meta.url), 'utf8')
const script = source.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)![1]!
const ast = ts.createSourceFile('Settings.ts', script, ts.ScriptTarget.Latest, true)

function findVariable(name: string) {
  return ast.statements.find((statement) =>
    ts.isVariableStatement(statement) && statement.declarationList.declarations.some(
      (declaration) => ts.isIdentifier(declaration.name) && declaration.name.text === name,
    ),
  )
}

function evaluate(name: string): unknown {
  const statement = findVariable(name)
  assert.ok(statement, `Settings must define ${name}`)
  const compiled = ts.transpileModule(statement.getText(ast), {
    compilerOptions: { target: ts.ScriptTarget.ES2022 },
  }).outputText
  return runInNewContext(`${compiled}\n${name}`, { computed })
}

const navItems = evaluate('navItems') as { value: Array<{ key: string }> }
const legacySectionMap = evaluate('LEGACY_SECTION_MAP') as Record<string, string>

test('personal settings sidebar exposes exactly five panes', () => {
  assert.deepEqual(
    Array.from(navItems.value, (item) => item.key),
    ['account', 'models', 'integrations', 'health', 'about'],
  )
})

test('legacy section links map onto the five panes', () => {
  const paneKeys = new Set(Array.from(navItems.value, (item) => item.key))
  for (const target of Object.values(legacySectionMap)) {
    assert.ok(paneKeys.has(target), `legacy target ${target} must be a pane key`)
  }
})
