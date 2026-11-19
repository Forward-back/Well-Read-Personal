/**
 * Acceptance for the V3 review mode layer (four ratings, keyboard, source bar,
 * cluster relearn conflict). Session and rating APIs are mocked so the script
 * never changes real review schedules.
 *
 * Usage:
 *   NODE_PATH=frontend/node_modules WELLREAD_WEB=http://localhost:5174 \
 *     node scripts/acceptance/browser/review_v3_mode_acceptance.js
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

const card = (id, question, patch = {}) => ({
  id,
  tenant_id: 1,
  group_id: 'g1',
  card_type: 'qa',
  question,
  answer: '答案',
  explanation: '',
  tags: [],
  status: 'active',
  review_level: 2,
  stability: 20,
  forget_streak: 0,
  due_at: '2026-11-10T04:00:00Z',
  lapses: 0,
  review_count: 2,
  question_hash: id,
  last_reviewed_at: '2026-11-10T04:00:00Z',
  source_updated: false,
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-10T00:00:00Z',
  ...patch,
})

const severeCard = card('card-1', '中断驱动方式与程序查询方式的主要区别是什么？', {
  answer: '**核心区别**：程序查询方式让 CPU 空转等待，中断驱动方式让 CPU 继续执行直到设备发起中断。\n\n```text\n程序查询：CPU 等待\n中断驱动：CPU 继续执行\n```',
  explanation: '答题时先说明触发方式，再说明 CPU 等待期间的差异。',
  stability: 2,
  forget_streak: 2,
  lapses: 3,
  review_count: 4,
  vulnerability: 'severe',
})
const warningCard = card('card-2', '单级中断处理流程中，中断隐指令主要完成哪三件事？', {
  answer: '关中断、保存断点、引出中断服务程序。',
  stability: 8,
  lapses: 1,
  review_count: 3,
  vulnerability: 'warning',
})

const session = (id, mode, cards, completed = 0, counts = {}) => ({
  id,
  tenant_id: 1,
  mode,
  status: 'active',
  group_id: '',
  planned_card_ids: cards.map(item => item.id),
  planned_count: cards.length,
  completed_count: completed,
  remembered_count: counts.remembered || 0,
  easy_count: counts.easy || 0,
  fuzzy_count: counts.fuzzy || 0,
  forgot_count: counts.forgot || 0,
  review_day: '2026-11-16',
  started_at: '2026-11-16T10:00:00Z',
  completed_at: null,
  created_at: '2026-11-16T10:00:00Z',
  updated_at: '2026-11-16T10:00:00Z',
})

const stateOf = sess => ({ session: sess, cards: [severeCard, warningCard].slice(0, sess.planned_count), logs: [] })

const dailyState = () => stateOf(session('sess-daily', 'daily', [severeCard, warningCard]))
const existingState = () => {
  const sess = session('sess-existing', 'manual', [warningCard])
  return { session: sess, cards: [warningCard], logs: [] }
}

const heatmapCell = {
  row_id: 'row-1',
  column_key: 'tag:中断',
  level: 'severe',
  intensity: 0.8,
  total: 2,
  severe: 1,
  warning: 1,
  observe: 0,
  healthy: 0,
  no_data: 0,
  clickable: true,
}
const heatmapFixture = {
  rows: [{
    id: 'row-1', group_id: 'g1', name: '408', path: '考研 / 408',
    kind: 'group', drillable: true, virtual: false, total_cards: 2,
  }],
  columns: [{ key: 'tag:中断', title: '中断', kind: 'tag', source_id: '', knowledge_base_id: '' }],
  cells: [heatmapCell],
  priorities: [heatmapCell],
  observe_count: 0,
  insight: null,
  scope_group_id: '',
  scope_path: '全部卡组',
  row_total: 1,
  column_total: 1,
  row_truncated: false,
  column_truncated: false,
  formula: { severe_weight: 1, warning_weight: 0.5, observe_weight: 0.25, denominator: 2, text: '' },
}
const clusterFixture = {
  cell: heatmapCell,
  row: heatmapFixture.rows[0],
  column: heatmapFixture.columns[0],
  cards: [
    { card: severeCard, level: 'severe', last_rating: 'forgot', suggested_action: 'relearn', source: null, knowledge_base_id: '' },
    { card: warningCard, level: 'warning', last_rating: 'fuzzy', suggested_action: 'relearn', source: null, knowledge_base_id: '' },
  ],
  reasons: ['连续忘记 2 次'],
  formula: { severe_weight: 1, warning_weight: 0.5, observe_weight: 0.25, denominator: 2, text: '' },
  scope_group_id: '',
}

const queueFixture = {
  due: [severeCard, warningCard],
  new: [],
  completed_today: 0,
  limit: 100,
  new_limit: 20,
  streak_days: 4,
}

const detailFor = (id) => {
  const base = id === severeCard.id ? severeCard : warningCard
  return {
    data: {
      card: base,
      sources: id === severeCard.id
        ? [{
            id: 'src-1',
            knowledge_id: 'k-1',
            chunk_id: 'chunk-1',
            excerpt: '中断驱动方式在设备准备数据期间让 CPU 继续执行其他程序。',
            status: 'active',
            is_primary: true,
            source_type: 'knowledge_chunk',
          }]
        : [],
    },
  }
}

let activeState = null
let submitted = 0
const startBodies = []

async function setInput(page, handle, value) {
  await handle.evaluate((e) => { e.focus(); e.select() })
  await page.keyboard.press('Backspace')
  await page.keyboard.type(value, { delay: 5 })
}

async function clickText(page, selector, text, { exact = false } = {}) {
  const ok = await page.evaluate(({ selector, text, exact }) => {
    const els = [...document.querySelectorAll(selector)].filter((e) => e.offsetWidth || e.offsetHeight)
    const el = els.find((e) => {
      const t = (e.innerText || e.value || '').trim()
      return exact ? t === text : t.includes(text)
    })
    if (!el) return false
    el.click()
    return true
  }, { selector, text, exact })
  if (!ok) throw new Error(`clickText not found: ${selector} "${text}"`)
}

async function waitText(page, text, timeout = 20000) {
  await page.waitForFunction((t) => document.body.innerText.includes(t), { timeout }, text)
}

async function login(page) {
  await page.goto(BASE + '/', { waitUntil: 'networkidle2' })
  await page.waitForSelector('input[type="password"]', { timeout: 20000 })
  await setInput(page, await page.$('input[type="text"]'), env.OWNER_EMAIL)
  await setInput(page, await page.$('input[type="password"]'), env.OWNER_PASSWORD)
  await Promise.all([
    page.waitForFunction(() => location.pathname === '/', { timeout: 20000 }),
    clickText(page, 'button', '登录', { exact: true }),
  ])
}

async function gotoV3(page) {
  await page.goto(BASE + '/platform/review', { waitUntil: 'networkidle2' })
  await page.waitForSelector('[data-testid="review-v3-task-bar"]', { timeout: 20000 })
  await page.waitForSelector('[data-testid="review-v3-heatmap"] .heatmap__cell', { timeout: 20000 })
}

async function clickStart(page) {
  await page.waitForFunction(() => {
    const mask = document.querySelector('.t-loading__mask')
    return !mask || !(mask.offsetWidth || mask.offsetHeight)
  }, { timeout: 15000 })
  await page.evaluate(() => {
    document.querySelector('[data-testid="review-v3-start"]')?.click()
  })
  await page.waitForSelector('[data-testid="review-v3-mode"]', { timeout: 15000 })
}

async function openCluster(page) {
  await page.evaluate(() => {
    document.querySelector('[data-testid="review-v3-heatmap"] .heatmap__cell.is-clickable')?.click()
  })
  await page.waitForFunction(() => location.pathname.includes('/review/cluster/'), { timeout: 15000 })
  await page.waitForSelector('[data-testid="review-v3-cluster-page"]', { timeout: 15000 })
  await page.waitForSelector('[data-testid="review-v3-cluster-relearn"]', { timeout: 15000 })
}

async function main() {
  if (!env.OWNER_EMAIL || !env.OWNER_PASSWORD) throw new Error('OWNER_EMAIL/OWNER_PASSWORD missing in deployments/.env')
  const browser = await puppeteer.launch({
    executablePath: CHROME,
    headless: 'new',
    args: ['--no-sandbox', '--disable-dev-shm-usage', '--window-size=1440,960'],
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
    const path = url.pathname
    const json = (data, status = 200) => req.respond({
      status,
      contentType: 'application/json',
      body: JSON.stringify(data),
    })

    if (path === '/api/v1/review/sessions/active' && req.method() === 'GET') {
      json({ success: true, data: activeState })
      return
    }
    if (path === '/api/v1/review/sessions' && req.method() === 'POST') {
      const body = JSON.parse(req.postData() || '{}')
      startBodies.push(body)
      if (activeState && !body.force_new) {
        json({ success: false, message: '已有进行中的复习会话' }, 409)
        return
      }
      const cards = body.mode === 'manual'
        ? (body.card_ids || []).map(id => (id === severeCard.id ? severeCard : warningCard))
        : [severeCard, warningCard]
      const nextState = stateOf(session(`sess-${startBodies.length}`, body.mode || 'daily', cards))
      activeState = nextState
      json({ success: true, data: nextState })
      return
    }
    if (path === '/api/v1/review/reviews' && req.method() === 'POST') {
      submitted += 1
      if (submitted >= 2) activeState = null
      json({ success: true, data: severeCard })
      return
    }
    if (path.startsWith('/api/v1/review/cards/') && req.method() === 'GET') {
      json({ success: true, ...detailFor(path.split('/').pop()) })
      return
    }
    if (path === '/api/v1/review/queue' && req.method() === 'GET') {
      json({ success: true, data: queueFixture })
      return
    }
    if (path === '/api/v1/review/heatmap' && req.method() === 'GET') {
      json({ success: true, data: heatmapFixture })
      return
    }
    if (path === '/api/v1/review/clusters/detail' && req.method() === 'GET') {
      json({ success: true, data: clusterFixture })
      return
    }
    if (path === '/api/v1/knowledge/k-1' && req.method() === 'GET') {
      json({ success: true, data: { id: 'k-1', knowledge_base_id: 'kb-1' } })
      return
    }
    req.continue()
  })

  try {
    await login(page)
    record('login', 'owner logged in')
    await gotoV3(page)
    record('route', 'review main route ready with mocked queue/session/heatmap')

    // 1. Daily start, severe card, parameters, keyboard flow.
    await clickStart(page)
    await waitText(page, '连续忘记 2 次')
    await waitText(page, '记忆可撑')
    const params = await page.$$eval('[data-testid="review-v3-params"] > div', (nodes) => nodes.length)
    if (params !== 3) throw new Error(`expected 3 parameter metrics, got ${params}`)
    const progress = await page.$eval('[data-testid="review-v3-mode-progress"]', el => el.innerText.trim())
    if (!progress.startsWith('1 / 2')) throw new Error(`unexpected progress: ${progress}`)
    record('mode-severe', 'daily session opened, severe banner and 3 parameters visible')
    await page.screenshot({ path: path.join(OUT, 'review_v3_mode_question.png'), fullPage: true })

    await page.keyboard.press('Space')
    await page.waitForSelector('[data-testid="review-v3-answer"]', { timeout: 10000 })
    const answer = await page.evaluate(() => {
      const root = document.querySelector('[data-testid="review-v3-answer"]')
      return {
        strong: Boolean(root?.querySelector('strong')),
        pre: Boolean(root?.querySelector('pre')),
        source: Boolean(document.querySelector('[data-testid="review-v3-source"]')),
        ratings: document.querySelectorAll('[data-testid="review-v3-ratings"] button').length,
      }
    })
    if (!answer.strong || !answer.pre) throw new Error('markdown bold/code block missing')
    if (!answer.source) throw new Error('source bar missing')
    if (answer.ratings !== 4) throw new Error(`expected 4 rating buttons, got ${answer.ratings}`)
    await waitText(page, '来源摘录')
    record('mode-answer', 'markdown, source bar and 4 rating buttons visible')
    await page.screenshot({ path: path.join(OUT, 'review_v3_mode_answer.png'), fullPage: true })

    // Source opens in a new tab with knowledge chunk query.
    const newTarget = new Promise(resolve => browser.once('targetcreated', target => resolve(target)))
    const sourceClicked = await page.evaluate(() => {
      const root = document.querySelector('[data-testid="review-v3-source"]')
      const btn = root?.querySelector('button')
      if (!btn) return false
      btn.click()
      return true
    })
    if (!sourceClicked) {
      const html = await page.$eval('[data-testid="review-v3-source"]', el => el.outerHTML)
      throw new Error(`source button missing: ${html}`)
    }
    const target = await newTarget
    const sourcePage = await target.page()
    await sourcePage.waitForFunction(() => location.pathname.includes('/platform/knowledge-bases/kb-1'), { timeout: 15000 })
    const sourceUrl = sourcePage.url()
    if (!sourceUrl.includes('knowledge_id=k-1') || !sourceUrl.includes('chunk_id=chunk-1')) {
      throw new Error(`source tab missing chunk ids: ${sourceUrl}`)
    }
    await sourcePage.close()
    record('mode-source', 'source opens a new tab with knowledge_id and chunk_id')

    await page.keyboard.press('1')
    await waitText(page, '单级中断处理流程')
    const warningTag = await page.evaluate(() => Boolean(document.querySelector('[data-testid="review-v3-warning-tag"]')))
    if (!warningTag) throw new Error('warning tag missing on the second card')
    const progress2 = await page.$eval('[data-testid="review-v3-mode-progress"]', el => el.innerText.trim())
    if (!progress2.startsWith('2 / 2')) throw new Error(`unexpected progress: ${progress2}`)
    record('mode-second', 'forgot rating advances to warning card')

    await page.keyboard.press('Space')
    await page.waitForSelector('[data-testid="review-v3-ratings"]', { timeout: 10000 })
    await page.keyboard.press('4')
    await page.waitForSelector('[data-testid="review-v3-mode-done"]', { timeout: 10000 })
    const done = await page.evaluate(() => {
      const counts = [...document.querySelectorAll('.review-mode__count strong')].map(el => el.innerText.trim())
      return { counts, text: document.querySelector('[data-testid="review-v3-mode-done"]')?.innerText || '' }
    })
    if (done.counts.join(',') !== '1,0,0,1') throw new Error(`unexpected completion counts: ${done.counts.join(',')}`)
    if (!done.text.includes('连续 4 天')) throw new Error('streak missing on completion page')
    record('mode-done', 'completion page shows four-tier counts and streak')
    await page.screenshot({ path: path.join(OUT, 'review_v3_mode_done.png'), fullPage: true })

    await clickText(page, '[data-testid="review-v3-mode-done"] button', '返回复习首页')
    await page.waitForFunction(() => !document.querySelector('[data-testid="review-v3-mode"]'), { timeout: 10000 })
    record('mode-close', 'returned to today view')

    // 2. Existing session conflict from cluster relearn.
    activeState = existingState()
    await gotoV3(page)
    await openCluster(page)
    await clickText(page, '[data-testid="review-v3-cluster-relearn"]', '重学这 2 张')
    await page.waitForFunction(() => document.body.innerText.includes('已有进行中的复习'), { timeout: 10000 })
    record('conflict-dialog', 'relearn with active session asks continue or abandon')

    await clickText(page, '.t-dialog button', '继续当前')
    await page.waitForSelector('[data-testid="review-v3-mode"]', { timeout: 10000 })
    const continuedProgress = await page.$eval('[data-testid="review-v3-mode-progress"]', el => el.innerText.trim())
    if (!continuedProgress.startsWith('1 / 1')) throw new Error(`unexpected continued progress: ${continuedProgress}`)
    await page.keyboard.press('Escape')
    await page.waitForFunction(() => !document.querySelector('[data-testid="review-v3-mode"]'), { timeout: 10000 })
    record('conflict-continue', 'continues the existing session and Esc suspends')

    await openCluster(page)
    await clickText(page, '[data-testid="review-v3-cluster-relearn"]', '重学这 2 张')
    await page.waitForFunction(() => document.body.innerText.includes('已有进行中的复习'), { timeout: 10000 })
    await clickText(page, '.t-dialog button', '放弃并开始补强')
    await page.waitForSelector('[data-testid="review-v3-mode"]', { timeout: 10000 })
    await waitText(page, '漏洞补强')
    const forced = startBodies[startBodies.length - 1]
    if (!forced?.force_new || forced.mode !== 'manual') {
      throw new Error(`force_new manual session missing: ${JSON.stringify(forced)}`)
    }
    record('conflict-abandon', 'abandons old session and starts a manual relearn session')

    // 3. 600px layout of the mode layer.
    await page.setViewport({ width: 600, height: 900 })
    await page.keyboard.press('Space')
    await page.waitForSelector('[data-testid="review-v3-ratings"]', { timeout: 10000 })
    const narrow = await page.evaluate(() => ({
      scroll: document.documentElement.scrollWidth,
      inner: window.innerWidth,
      ratingColumns: getComputedStyle(document.querySelector('[data-testid="review-v3-ratings"]')).gridTemplateColumns.split(' ').length,
      overlayScroll: document.querySelector('[data-testid="review-v3-mode"]').scrollWidth,
      overlayWidth: document.querySelector('[data-testid="review-v3-mode"]').clientWidth,
    }))
    if (narrow.scroll > narrow.inner + 1) throw new Error(`600px page overflow: ${narrow.scroll} > ${narrow.inner}`)
    if (narrow.overlayScroll > narrow.overlayWidth + 1) throw new Error('mode overlay horizontal overflow')
    if (narrow.ratingColumns !== 2) throw new Error(`expected 2 rating columns at 600px, got ${narrow.ratingColumns}`)
    record('mode-600', 'no overflow and 2x2 rating grid at 600px')
    await page.mouse.move(0, 0)
    await page.evaluate(() => {
      if (document.activeElement instanceof HTMLElement) document.activeElement.blur()
    })
    await page.screenshot({ path: path.join(OUT, 'review_v3_mode_600.png'), fullPage: true })

    if (consoleErrors.length || pageErrors.length) {
      throw new Error(`console errors: ${JSON.stringify([...consoleErrors, ...pageErrors].slice(0, 5))}`)
    }
    record('console', '0 console/page errors')

    fs.writeFileSync(path.join(OUT, 'review_v3_mode_log.json'), JSON.stringify({ log, consoleErrors, pageErrors }, null, 2))
    console.log(`DONE screenshots in ${OUT}`)
  } catch (err) {
    fs.writeFileSync(path.join(OUT, 'review_v3_mode_log.json'), JSON.stringify({ log, consoleErrors, pageErrors, error: String(err) }, null, 2))
    console.error('FAILED:', err)
    process.exitCode = 1
  } finally {
    await browser.close()
  }
}

main()
