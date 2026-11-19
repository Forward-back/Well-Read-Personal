/**
 * Acceptance for the standalone cluster page AI exits (split / new angle).
 * Mocks cluster detail and generation APIs; sessions stay real but are not started.
 */
const puppeteer = require('puppeteer-core')
const fs = require('fs')
const os = require('os')
const path = require('path')

const ROOT = process.env.WELLREAD_ROOT || path.resolve(__dirname, '../../..')
const CHROME = process.env.CHROME_PATH || 'D:/chrome-for-test/chrome/win64-157.0.8103.38/chrome-win64/chrome.exe'
const BASE = process.env.WELLREAD_WEB || 'http://localhost:5174'
const OUT = process.env.ACCEPT_OUT || path.join(os.tmpdir(), 'wellread_accept')
fs.mkdirSync(OUT, { recursive: true })

const log = []
const record = (step, detail) => {
  log.push({ step, detail })
  console.log(`[${step}] ${detail}`)
}

function readEnv() {
  const env = {}
  const file = path.join(ROOT, 'deployments/.env')
  if (!fs.existsSync(file)) return env
  for (const line of fs.readFileSync(file, 'utf8').split(/\r?\n/)) {
    const m = line.match(/^([A-Z_]+)=(.*)$/)
    if (m) env[m[1]] = m[2].replace(/^['"]|['"]$/g, '')
  }
  return env
}
const env = readEnv()

const card = (id, question, level, patch = {}) => ({
  id,
  tenant_id: 1,
  group_id: 'g1',
  card_type: 'qa',
  question,
  answer: '答案',
  explanation: '',
  tags: ['极限'],
  status: 'active',
  review_level: 2,
  stability: 20,
  forget_streak: 0,
  due_at: '2026-11-10T04:00:00Z',
  lapses: 0,
  review_count: 4,
  question_hash: id,
  last_reviewed_at: '2026-11-10T04:00:00Z',
  source_updated: false,
  vulnerability: level,
  vulnerability_reason: level === 'severe' ? '连续忘记 3 次' : '',
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-10T00:00:00Z',
  ...patch,
})

const clusterCard = (item, level, action, sourceId) => ({
  card: item,
  level,
  last_rating: level === 'severe' ? 'forgot' : 'fuzzy',
  suggested_action: action,
  source: sourceId
    ? { id: sourceId, knowledge_id: 'k-1', chunk_id: 'chunk-1', excerpt: '原文摘录', status: 'active', is_primary: true }
    : null,
  knowledge_base_id: sourceId ? 'kb-1' : '',
})

const row = { id: 'g1', group_id: 'g1', name: '考研数学', path: '考研 / 数学', kind: 'group', drillable: true, virtual: false, total_cards: 2 }
const column = { key: 'tag:极限', title: '极限', kind: 'tag', source_id: '', knowledge_base_id: '' }
const cell = {
  row_id: 'g1', column_key: 'tag:极限', level: 'severe', intensity: 0.75,
  total: 2, severe: 1, warning: 1, observe: 0, healthy: 0, no_data: 0, clickable: true,
}
const detail = {
  cell,
  row,
  column,
  cards: [
    clusterCard(card('card-1', '求极限的推导', 'severe', { stability: 2, forget_streak: 3, lapses: 3 }), 'severe', 'relearn', 'src-1'),
    clusterCard(card('card-2', '等价无穷小替换', 'warning', { stability: 8, lapses: 1 }), 'warning', 'relearn', 'src-2'),
  ],
  reasons: ['连续忘记 3 次'],
  formula: { severe_weight: 1, warning_weight: 0.5, observe_weight: 0.25, denominator: 2, text: '' },
  scope_group_id: 'g1',
}

const generated = (question, duplicate = false) => ({
  question,
  answer: '候选答案',
  explanation: '',
  source_knowledge_id: 'k-1',
  source_chunk_id: 'chunk-1',
  source_title: '高数笔记',
  source_excerpt: '原文摘录',
  duplicate,
  existing_card_id: duplicate ? 'existing' : '',
  sources: [{
    source_type: 'knowledge_chunk',
    knowledge_id: 'k-1',
    chunk_id: 'chunk-1',
    excerpt: '原文摘录',
  }],
})

const splitBodies = []
const angleBodies = []
let detailRequests = 0

async function setInput(page, handle, value) {
  await handle.evaluate((e) => { e.focus(); e.select() })
  await page.keyboard.press('Backspace')
  await page.keyboard.type(value, { delay: 5 })
}
async function clickText(page, selector, text) {
  const ok = await page.evaluate(({ selector, text }) => {
    const el = [...document.querySelectorAll(selector)]
      .filter((e) => e.offsetWidth || e.offsetHeight)
      .find((e) => (e.innerText || '').trim().includes(text))
    if (!el) return false
    el.click()
    return true
  }, { selector, text })
  if (!ok) throw new Error(`missing ${selector} "${text}"`)
}

async function waitNode(predicate, timeout = 10000, label = 'node predicate') {
  const start = Date.now()
  while (Date.now() - start < timeout) {
    if (predicate()) return
    await new Promise(resolve => setTimeout(resolve, 100))
  }
  throw new Error(`timeout waiting for ${label}`)
}

async function login(page) {
  await page.goto(BASE + '/', { waitUntil: 'networkidle2' })
  await page.waitForSelector('input[type="password"]', { timeout: 20000 })
  await setInput(page, await page.$('input[type="text"]'), env.OWNER_EMAIL)
  await setInput(page, await page.$('input[type="password"]'), env.OWNER_PASSWORD)
  await Promise.all([
    page.waitForFunction(() => location.pathname === '/', { timeout: 20000 }),
    clickText(page, 'button', '登录'),
  ])
}

async function main() {
  if (!env.OWNER_EMAIL || !env.OWNER_PASSWORD) throw new Error('OWNER_EMAIL/OWNER_PASSWORD missing')
  const browser = await puppeteer.launch({
    executablePath: CHROME,
    headless: 'new',
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
    defaultViewport: { width: 1440, height: 960 },
  })
  const page = await browser.newPage()
  const consoleErrors = []
  const pageErrors = []
  page.on('console', (m) => { if (m.type() === 'error') consoleErrors.push(m.text()) })
  page.on('pageerror', (e) => pageErrors.push(String(e)))

  await page.setRequestInterception(true)
  page.on('request', (req) => {
    const url = new URL(req.url())
    const json = (data, status = 200) => req.respond({ status, contentType: 'application/json', body: JSON.stringify(data) })
    if (url.pathname === '/api/v1/review/clusters/detail') {
      detailRequests += 1
      json({ success: true, data: detail })
      return
    }
    if (url.pathname === '/api/v1/review/cards/card-1/split/preview') {
      json({ success: true, data: { model_id: 'mock', cards: [generated('拆分后的极限推导'), generated('重复题', true)] } })
      return
    }
    if (url.pathname === '/api/v1/review/cards/card-1/split/apply') {
      splitBodies.push(JSON.parse(req.postData() || '{}'))
      json({ success: true, data: { created: [{ id: 'new-1' }], archived_original: true } })
      return
    }
    if (url.pathname === '/api/v1/review/cards/card-1/angles/preview') {
      json({ success: true, data: { model_id: 'mock', cards: [generated('换问法：极限定义是什么？')] } })
      return
    }
    if (url.pathname === '/api/v1/review/generations/import') {
      angleBodies.push(JSON.parse(req.postData() || '{}'))
      json({ success: true, data: { imported: 1 } })
      return
    }
    if (url.pathname === '/api/v1/review/sessions/active') {
      json({ success: true, data: null })
      return
    }
    req.continue()
  })

  try {
    await login(page)
    record('login', 'owner logged in')
    await page.goto(BASE + '/platform/review/cluster/g1?col=' + encodeURIComponent('tag:极限') + '&scope=g1', { waitUntil: 'networkidle2' })
    await page.waitForSelector('[data-testid="review-v3-cluster-page"]', { timeout: 20000 })
    await page.waitForFunction(() => document.body.innerText.includes('求极限的推导'), { timeout: 15000 })
    record('cluster', 'standalone cluster page loaded from URL')

    const actions = await page.evaluate(() => ({
      count: document.querySelectorAll('.cluster-action').length,
      sourceEnabled: !document.querySelector('[data-testid="review-v3-action-source"]')?.disabled,
      splitEnabled: !document.querySelector('[data-testid="review-v3-action-split"]')?.disabled,
      anglesEnabled: !document.querySelector('[data-testid="review-v3-action-angles"]')?.disabled,
    }))
    if (actions.count !== 4 || !actions.sourceEnabled || !actions.splitEnabled || !actions.anglesEnabled) {
      throw new Error(`cluster actions unavailable: ${JSON.stringify(actions)}`)
    }
    record('actions', 'all four exits enabled for the selected card')

    await page.click('[data-testid="review-v3-action-split"]')
    await page.waitForSelector('[data-testid="review-v3-ai-dialog"]', { timeout: 10000 })
    await page.waitForFunction(() => document.body.innerText.includes('拆分后的极限推导'), { timeout: 10000 })
    const duplicateDisabled = await page.evaluate(() => {
      const candidates = [...document.querySelectorAll('.ai-dialog__item.is-disabled')]
      return candidates.length
    })
    if (duplicateDisabled !== 1) throw new Error(`expected 1 disabled duplicate candidate, got ${duplicateDisabled}`)
    await clickText(page, '.ai-dialog__footer button', '创建拆分卡')
    await waitNode(() => splitBodies.length > 0, 10000, 'split apply')
    if (splitBodies[0].cards?.length !== 1) throw new Error(`split apply should send 1 selected card, got ${splitBodies[0].cards?.length}`)
    record('split', 'duplicate candidate disabled; one split card applied')

    await page.waitForFunction(() => {
      const el = document.querySelector('[data-testid="review-v3-ai-dialog"]')
      return !el || !(el.offsetWidth || el.offsetHeight)
    }, { timeout: 10000 })
    await page.click('[data-testid="review-v3-action-angles"]')
    await page.waitForFunction(() => {
      const el = document.querySelector('[data-testid="review-v3-ai-dialog"]')
      return el && (el.offsetWidth || el.offsetHeight)
    }, { timeout: 10000 })
    await page.waitForFunction(() => document.body.innerText.includes('换问法：极限定义是什么？'), { timeout: 10000 })
    await clickText(page, '.ai-dialog__footer button', '导入新角度卡')
    await waitNode(() => angleBodies.length > 0, 10000, 'angle import')
    const body = angleBodies[0]
    if (body.group_id !== 'g1') throw new Error(`angle import group mismatch: ${body.group_id}`)
    if (!body.cards?.[0]?.sources?.length) throw new Error('angle import lost source')
    if (!body.cards?.[0]?.tags?.includes('极限')) throw new Error('angle import lost tags')
    record('angles', 'new-angle card imported with group, tags and sources')

    await waitNode(() => detailRequests >= 3, 10000, 'cluster detail reload after AI actions')
    await page.setViewport({ width: 600, height: 900 })
    const responsive = await page.evaluate(() => ({
      scroll: document.documentElement.scrollWidth,
      inner: window.innerWidth,
    }))
    if (responsive.scroll > responsive.inner + 1) throw new Error('600px cluster overflow')
    await page.mouse.move(0, 0)
    await page.screenshot({ path: path.join(OUT, 'review_v3_cluster_ai_600.png'), fullPage: true })
    record('cluster-600', 'cluster AI page has no horizontal overflow at 600px')

    if (consoleErrors.length || pageErrors.length) {
      throw new Error(`console errors: ${JSON.stringify([...consoleErrors, ...pageErrors].slice(0, 5))}`)
    }
    record('console', '0 console/page errors')
    fs.writeFileSync(path.join(OUT, 'review_v3_cluster_log.json'), JSON.stringify({ log, splitBodies, angleBodies, detailRequests, consoleErrors, pageErrors }, null, 2))
    console.log(`DONE screenshots in ${OUT}`)
  } catch (err) {
    fs.writeFileSync(path.join(OUT, 'review_v3_cluster_log.json'), JSON.stringify({ log, splitBodies, angleBodies, detailRequests, consoleErrors, pageErrors, error: String(err) }, null, 2))
    console.error('FAILED:', err)
    process.exitCode = 1
  } finally {
    await browser.close()
  }
}

main()
