/**
 * Real-backend acceptance for the V2-parity features ported into review V3:
 * card generation, merge, orphan rebind, JSON export, plus a real daily
 * session start -> stash -> resume -> complete flow.
 *
 * Usage:
 *   NODE_PATH=frontend/node_modules \
 *     node scripts/acceptance/browser/review_v3_parity_acceptance.js
 */
const puppeteer = require('puppeteer-core')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { spawnSync } = require('child_process')

const ROOT = process.env.WELLREAD_ROOT || path.resolve(__dirname, '../../..')
const CHROME = process.env.CHROME_PATH || 'D:/chrome-for-test/chrome/win64-157.0.8103.38/chrome-win64/chrome.exe'
const BASE = process.env.WELLREAD_WEB || 'http://localhost:5174'
const OUT = process.env.ACCEPT_OUT || path.join(os.tmpdir(), 'wellread_accept')
const DOWNLOADS = path.join(OUT, 'downloads')
fs.mkdirSync(DOWNLOADS, { recursive: true })

const RUN_ID = `v3parity-${Date.now().toString(36)}`
const SCOPE_NAME = `V3对等验收-${RUN_ID}`
const DAILY_NAME = `${SCOPE_NAME}-每日`
const record = (step, detail) => console.log(`[${step}] ${detail}`)

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

function psql(sql) {
  const res = spawnSync(
    'docker',
    ['exec', 'WellRead-Postgres', 'psql', '-U', env.DB_USER || 'root', '-d', env.DB_NAME || 'WellRead', '-t', '-A', '-c', sql],
    { encoding: 'utf8' },
  )
  if (res.status !== 0) throw new Error((res.stderr || res.stdout || 'psql failed').trim())
  return (res.stdout || '').trim()
}

const sleep = (ms) => new Promise(resolve => setTimeout(resolve, ms))

async function setInput(page, selector, value) {
  const handle = await page.$(selector)
  if (!handle) throw new Error(`input not found: ${selector}`)
  await handle.evaluate((e) => { e.focus(); e.select() })
  await page.keyboard.press('Backspace')
  await page.keyboard.type(value, { delay: 5 })
}

async function clickText(page, selector, text, { exact = false } = {}) {
  const ok = await page.evaluate(({ selector, text, exact }) => {
    const els = [...document.querySelectorAll(selector)].filter(e => e.offsetWidth || e.offsetHeight)
    const el = els.find((e) => {
      const value = (e.innerText || e.value || '').trim()
      return exact ? value === text : value.includes(text)
    })
    if (!el) return false
    el.click()
    return true
  }, { selector, text, exact })
  if (!ok) throw new Error(`clickText not found: ${selector} "${text}"`)
}

async function api(page, method, url, body) {
  return page.evaluate(async (m, u, b) => {
    const token = localStorage.getItem('gaze_token')
    const headers = { Authorization: 'Bearer ' + token }
    if (b !== null && b !== undefined) headers['Content-Type'] = 'application/json'
    const res = await fetch(u, {
      method: m,
      headers,
      body: b === null || b === undefined ? undefined : JSON.stringify(b),
    })
    let data = null
    try { data = await res.json() } catch (e) {}
    return { status: res.status, data }
  }, method, url, body)
}

async function login(page) {
  await page.goto(BASE + '/', { waitUntil: 'networkidle2' })
  await page.waitForSelector('input[type="password"]', { timeout: 20000 })
  await setInput(page, 'input[type="text"]', env.OWNER_EMAIL)
  await setInput(page, 'input[type="password"]', env.OWNER_PASSWORD)
  await Promise.all([
    page.waitForFunction(() => location.pathname === '/', { timeout: 20000 }),
    clickText(page, 'button', '登录', { exact: true }),
  ])
}

async function pickSelect(page, testId, text) {
  const selector = `[data-testid="${testId}"] input`
  const box = await page.evaluate((id) => {
    const el = document.querySelector(`[data-testid="${id}"]`)
    if (!el) return null
    el.scrollIntoView({ block: 'center' })
    const rect = el.getBoundingClientRect()
    return { x: rect.x + rect.width / 2, y: rect.y + rect.height / 2 }
  }, testId)
  if (!box) throw new Error(`select not found: ${testId}`)
  await sleep(120)
  await page.mouse.click(box.x, box.y)
  await page.waitForFunction(
    () => [...document.querySelectorAll('.t-select-option')].some(el => el.offsetHeight > 0),
    { timeout: 10000 },
  )
  const readOnly = await page.$eval(selector, el => el.readOnly)
  if (!readOnly) await setInput(page, selector, text)
  await page.waitForFunction(
    value => [...document.querySelectorAll('.t-select-option')].some(el => el.offsetHeight > 0 && el.innerText.includes(value)),
    { timeout: 15000 },
    text,
  )
  const clicked = await page.evaluate((value) => {
    const option = [...document.querySelectorAll('.t-select-option')]
      .find(el => el.offsetHeight > 0 && el.innerText.includes(value))
    if (!option) return false
    option.click()
    return true
  }, text)
  if (!clicked) throw new Error(`select option not found: ${text}`)
  await sleep(200)
}

async function openCardsTab(page) {
  await page.goto(BASE + '/platform/review', { waitUntil: 'networkidle2' })
  await page.waitForSelector('[data-testid="review-v3-task-bar"]', { timeout: 20000 })
  await page.evaluate(() => {
    [...document.querySelectorAll('.t-tabs__nav-item')].find(el => el.innerText.trim() === '卡片')?.click()
  })
  await page.waitForSelector('[data-testid="review-v3-cards"]', { timeout: 20000 })
}

async function seed(page) {
  const sourceRow = psql(
    "select knowledge_id || '|' || chunk_id from review_card_sources where source_type = 'knowledge_chunk' and status = 'active' limit 1",
  )
  if (!sourceRow) throw new Error('no active knowledge_chunk source to reuse')
  const [knowledgeID, chunkID] = sourceRow.split('|')

  const scopeRes = await api(page, 'POST', '/api/v1/review/groups', { name: SCOPE_NAME })
  if (scopeRes.status !== 201) throw new Error(`create scope failed: ${scopeRes.status}`)
  const scopeID = scopeRes.data.data.id
  const dailyRes = await api(page, 'POST', '/api/v1/review/groups', { name: DAILY_NAME, parent_id: scopeID })
  if (dailyRes.status !== 201) throw new Error(`create daily group failed: ${dailyRes.status}`)
  const dailyID = dailyRes.data.data.id

  const createCard = async (groupID, question, tag) => {
    const res = await api(page, 'POST', '/api/v1/review/cards', {
      group_id: groupID,
      question,
      answer: `${question} 的答案。`,
      explanation: `${question} 的解析。`,
      tags: tag ? [tag] : [],
      sources: [{
        source_type: 'knowledge_chunk',
        knowledge_id: knowledgeID,
        chunk_id: chunkID,
        excerpt: `${question} 的来源摘录。`,
        is_primary: true,
      }],
    })
    if (res.status !== 201) throw new Error(`create card failed: ${res.status} ${JSON.stringify(res.data)}`)
    return res.data.data
  }

  const mergeA = await createCard(scopeID, `[${RUN_ID}] 合并卡 A`, `${RUN_ID}-merge`)
  const mergeB = await createCard(scopeID, `[${RUN_ID}] 合并卡 B`, `${RUN_ID}-merge`)
  const orphan = await createCard(scopeID, `[${RUN_ID}] 孤儿卡`, `${RUN_ID}-orphan`)
  const dailyA = await createCard(dailyID, `[${RUN_ID}] 每日卡 A`, `${RUN_ID}-daily`)
  const dailyB = await createCard(dailyID, `[${RUN_ID}] 每日卡 B`, `${RUN_ID}-daily`)

  const orphanDetail = await api(page, 'GET', `/api/v1/review/cards/${orphan.id}`)
  const orphanSource = orphanDetail.data?.data?.sources?.[0]
  if (!orphanSource) throw new Error('orphan source missing')
  const unavailable = await api(page, 'POST', `/api/v1/review/cards/${orphan.id}/sources/${orphanSource.id}/unavailable`)
  if (unavailable.status !== 200) throw new Error(`mark source unavailable failed: ${unavailable.status}`)

  record('seed', `scope=${SCOPE_NAME} merge=${mergeA.id}/${mergeB.id} orphan=${orphan.id} daily=${dailyA.id}/${dailyB.id}`)
  return {
    scopeID,
    dailyID,
    mergeA,
    mergeB,
    orphan,
    orphanSourceID: orphanSource.id,
    dailyA,
    dailyB,
  }
}

function cleanup(seeded) {
  if (!seeded) return
  const ids = [seeded.scopeID, seeded.dailyID]
  const quoted = ids.map(id => `'${id}'`).join(',')
  psql(`delete from review_logs where card_id in (select id from review_cards where group_id in (${quoted})) or session_id in (select id from review_sessions where group_id in (${quoted}))`)
  psql(`delete from review_sessions where group_id in (${quoted})`)
  psql(`delete from review_card_sources where card_id in (select id from review_cards where group_id in (${quoted}))`)
  psql(`delete from review_cards where group_id in (${quoted})`)
  psql(`delete from review_groups where id in (${quoted})`)
  const left = psql(`select count(*) from review_groups where id in (${quoted})`)
  if (left !== '0') throw new Error(`cleanup left ${left} groups`)
  record('cleanup', 'seeded groups, cards, sources, logs and sessions removed')
}

async function mergeChecks(page, seeded) {
  await openCardsTab(page)
  await setInput(page, '[data-testid="review-v3-cards-search"] input', `[${RUN_ID}] 合并卡`)
  await page.keyboard.press('Enter')
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="review-v3-card-row"]').length === 2, { timeout: 15000 })
  const ids = await page.evaluate(() => {
    const rows = [...document.querySelectorAll('[data-testid="review-v3-card-row"]')]
    rows.forEach(row => row.querySelector('.t-checkbox')?.click())
    return rows.map(row => row.getAttribute('data-card-id'))
  })
  await page.waitForSelector('[data-testid="review-v3-card-batch"]', { timeout: 10000 })
  await page.click('[data-testid="review-v3-cards-batch-merge"]')
  await page.waitForFunction(
    () => [...document.querySelectorAll('.t-dialog')].some(el => el.offsetHeight > 0 && el.innerText.includes('合并卡片')),
    { timeout: 10000 },
  )
  await clickText(page, '.t-dialog button', '合并卡片', { exact: true })
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="review-v3-card-row"]').length === 1, { timeout: 15000 })
  const target = await api(page, 'GET', `/api/v1/review/cards/${ids[0]}`)
  if (target.data?.data?.card?.status !== 'active') throw new Error('merge target not active')
  if ((target.data?.data?.sources?.length || 0) < 2) throw new Error('merge target lost sources')
  record('merge', `target=${ids[0]} sources=${target.data.data.sources.length}`)
}

async function rebindChecks(page, seeded) {
  await setInput(page, '[data-testid="review-v3-cards-search"] input', `[${RUN_ID}] 孤儿卡`)
  await page.keyboard.press('Enter')
  await pickSelect(page, 'review-v3-cards-status', '孤儿')
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="review-v3-card-row"]').length === 1, { timeout: 15000 })
  await page.click('[data-testid="review-v3-card-row"] .card-row__actions button')
  await page.waitForFunction(
    () => [...document.querySelectorAll('.t-dropdown__item')].some(el => el.offsetHeight > 0 && el.innerText.includes('重绑来源')),
    { timeout: 10000 },
  )
  await page.evaluate(() => {
    const item = [...document.querySelectorAll('.t-dropdown__item')]
      .find(el => el.offsetHeight > 0 && el.innerText.includes('重绑来源'))
    item?.click()
  })
  await page.waitForSelector('[data-testid="review-v3-cards-rebind-dialog"]', { visible: true, timeout: 10000 })
  await setInput(page, '[data-testid="review-v3-cards-rebind-chunk"] input', `rebound-${RUN_ID}`)
  await clickText(page, '.t-dialog button', '保存', { exact: true })
  await sleep(1000)
  const rebounded = await api(page, 'GET', `/api/v1/review/cards/${seeded.orphan.id}`)
  if (rebounded.data?.data?.card?.status !== 'suspended') {
    throw new Error(`rebind status=${rebounded.data?.data?.card?.status}`)
  }
  record('rebind', 'orphan rebuilt and stays suspended')
}

async function generateAndExport(page, seeded) {
  const before = await api(page, 'GET', `/api/v1/review/cards?group_id=${seeded.scopeID}&include_subtree=true&limit=200`)
  const beforeCount = (before.data?.data || []).length

  await page.click('[data-testid="review-v3-more"]')
  await page.waitForFunction(
    () => [...document.querySelectorAll('.t-dropdown__item')].some(el => el.offsetHeight > 0 && el.innerText.includes('生成卡片')),
    { timeout: 10000 },
  )
  await page.evaluate(() => {
    const item = [...document.querySelectorAll('.t-dropdown__item')]
      .find(el => el.offsetHeight > 0 && el.innerText.includes('生成卡片'))
    item?.click()
  })
  await page.waitForSelector('[data-testid="review-v3-generate-dialog"]', { visible: true, timeout: 10000 })
  await sleep(1200)
  const kbRes = await api(page, 'GET', '/api/v1/knowledge-bases')
  const kb = (kbRes.data?.data || [])[0]
  if (!kb) throw new Error('no knowledge base for generation')
  await pickSelect(page, 'review-v3-generate-kb', kb.name)
  await pickSelect(page, 'review-v3-generate-group', SCOPE_NAME)
  await page.click('[data-testid="review-v3-generate-preview"]')
  await page.waitForSelector('[data-testid="review-v3-generate-list"] .gen-item', { timeout: 60000 })
  const candidates = await page.evaluate(() => document.querySelectorAll('[data-testid="review-v3-generate-list"] .gen-item').length)
  await page.click('[data-testid="review-v3-generate-import"]')
  await page.waitForFunction(
    () => {
      const dialog = document.querySelector('[data-testid="review-v3-generate-dialog"]')
      return !dialog || !dialog.classList.contains('t-dialog--open')
    },
    { timeout: 30000 },
  )
  await sleep(800)
  const after = await api(page, 'GET', `/api/v1/review/cards?group_id=${seeded.scopeID}&include_subtree=true&limit=200`)
  const afterCount = (after.data?.data || []).length
  if (afterCount <= beforeCount) throw new Error(`generation imported nothing (${beforeCount} -> ${afterCount})`)
  record('generate', `candidates=${candidates} cards ${beforeCount} -> ${afterCount}`)

  for (const name of fs.readdirSync(DOWNLOADS)) {
    if (name.startsWith('wellread-review-') && name.endsWith('.json')) {
      fs.unlinkSync(path.join(DOWNLOADS, name))
    }
  }
  const client = await page.createCDPSession()
  await client.send('Page.setDownloadBehavior', { behavior: 'allow', downloadPath: DOWNLOADS })
  await page.click('[data-testid="review-v3-more"]')
  await page.waitForFunction(
    () => [...document.querySelectorAll('.t-dropdown__item')].some(el => el.offsetHeight > 0 && el.innerText.includes('导出 JSON')),
    { timeout: 10000 },
  )
  await page.evaluate(() => {
    const item = [...document.querySelectorAll('.t-dropdown__item')]
      .find(el => el.offsetHeight > 0 && el.innerText.includes('导出 JSON'))
    item?.click()
  })
  const deadline = Date.now() + 15000
  let jsonFile = ''
  while (Date.now() < deadline) {
    const next = fs.readdirSync(DOWNLOADS).find(name => name.startsWith('wellread-review-') && name.endsWith('.json'))
    if (next) { jsonFile = path.join(DOWNLOADS, next); break }
    await sleep(300)
  }
  if (!jsonFile) throw new Error('JSON export download not observed')
  const payload = JSON.parse(fs.readFileSync(jsonFile, 'utf8'))
  if (!payload || typeof payload !== 'object') throw new Error('JSON export payload invalid')
  record('export-json', `file=${path.basename(jsonFile)} keys=${Object.keys(payload).slice(0, 5).join(',')}`)
}

async function rateCurrent(page, rating) {
  await page.waitForSelector('[data-testid="review-v3-question"]', { timeout: 15000 })
  await page.click('[data-testid="review-v3-show-answer"]')
  await page.waitForSelector('[data-testid="review-v3-answer"]', { timeout: 10000 })
  await page.click(`[data-testid="review-v3-rate-${rating}"]`)
}

async function dailyChecks(page, seeded) {
  const active = await api(page, 'GET', '/api/v1/review/sessions/active')
  if (active.data?.data?.session) {
    record('daily-skip', `active session ${active.data.data.session.id} exists; daily e2e skipped without touching it`)
    return
  }
  const queueRes = await api(page, 'GET', `/api/v1/review/queue?group_id=${seeded.dailyID}`)
  const queue = queueRes.data?.data
  const queueCount = (queue?.due?.length || 0) + (queue?.new?.length || 0)
  if (queueCount !== 2) throw new Error(`daily queue count=${queueCount}`)
  record('daily-queue', `cards=${queueCount}`)

  await page.goto(`${BASE}/platform/review?scope=${seeded.dailyID}`, { waitUntil: 'networkidle2' })
  await page.waitForSelector('[data-testid="review-v3-task-bar"]', { timeout: 20000 })
  await page.waitForFunction(
    () => document.body.innerText.includes('2'),
    { timeout: 15000 },
  )
  await page.click('[data-testid="review-v3-start"]')
  await page.waitForSelector('[data-testid="review-v3-mode"]', { timeout: 20000 })
  await rateCurrent(page, 'easy')
  try {
    await page.waitForFunction(() => {
      const progress = document.querySelector('[data-testid="review-v3-mode-progress"]')?.innerText || ''
      return progress.includes('2 / 2')
    }, { timeout: 15000 })
  } catch (err) {
    const debug = await page.evaluate(() => ({
      progress: document.querySelector('[data-testid="review-v3-mode-progress"]')?.innerText || '',
      question: document.querySelector('[data-testid="review-v3-question"]')?.innerText || '',
      done: Boolean(document.querySelector('[data-testid="review-v3-mode-done"]')),
      mode: Boolean(document.querySelector('[data-testid="review-v3-mode"]')),
      body: document.body.innerText.slice(0, 300),
    }))
    const active = await api(page, 'GET', '/api/v1/review/sessions/active')
    debug.activeCards = active.data?.data?.cards?.length
    debug.activeLogs = active.data?.data?.logs?.length
    throw new Error(`daily first rating state unexpected: ${JSON.stringify(debug)}`)
  }
  await page.click('[data-testid="review-v3-suspend"]')
  await page.waitForSelector('[data-testid="review-v3-task-bar"]', { timeout: 20000 })
  await page.click('[data-testid="review-v3-start"]')
  await page.waitForSelector('[data-testid="review-v3-mode"]', { timeout: 20000 })
  await rateCurrent(page, 'remembered')
  try {
    await page.waitForSelector('[data-testid="review-v3-mode-done"]', { timeout: 20000 })
  } catch (err) {
    const debug = await page.evaluate(() => ({
      progress: document.querySelector('[data-testid="review-v3-mode-progress"]')?.innerText || '',
      question: document.querySelector('[data-testid="review-v3-question"]')?.innerText || '',
      done: Boolean(document.querySelector('[data-testid="review-v3-mode-done"]')),
      body: document.body.innerText.slice(0, 200),
    }))
    const active = await api(page, 'GET', '/api/v1/review/sessions/active')
    debug.activeCards = active.data?.data?.cards?.length
    debug.activeLogs = active.data?.data?.logs?.length
    await page.screenshot({ path: path.join(OUT, 'review_v3_daily_fail.png'), fullPage: true })
    throw new Error(`daily completion missing: ${JSON.stringify(debug)}`)
  }
  const countText = await page.$eval('[data-testid="review-v3-mode-done"]', el => el.innerText)
  if (!countText.includes('2')) throw new Error(`completion count unexpected: ${countText.slice(0, 80)}`)
  await page.screenshot({ path: path.join(OUT, 'review_v3_daily_done.png'), fullPage: true })
  const sessions = await api(page, 'GET', '/api/v1/review/sessions?limit=5')
  const latest = (sessions.data?.data || []).find(item => item.group_id === seeded.dailyID)
  if (!latest || latest.status !== 'completed' || latest.completed_count !== 2) {
    throw new Error(`daily session state unexpected: ${JSON.stringify(latest)}`)
  }
  record('daily', `session=${latest.id} status=completed completed=2`)
  await page.click('[data-testid="review-v3-suspend"]')
  await page.waitForSelector('[data-testid="review-v3-task-bar"]', { timeout: 20000 })
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
  page.on('console', m => { if (m.type() === 'error') consoleErrors.push(m.text()) })
  page.on('pageerror', e => pageErrors.push(String(e)))
  let seeded = null
  try {
    await login(page)
    record('login', 'owner logged in')
    seeded = await seed(page)
    await mergeChecks(page, seeded)
    await rebindChecks(page, seeded)
    await generateAndExport(page, seeded)
    await dailyChecks(page, seeded)
    if (consoleErrors.length || pageErrors.length) {
      throw new Error(`console errors: ${JSON.stringify(consoleErrors.concat(pageErrors).slice(0, 5))}`)
    }
    record('console', '0 console/page errors')
  } finally {
    try {
      cleanup(seeded)
    } catch (err) {
      console.error('[cleanup.FAIL]', err && err.message ? err.message : err)
      process.exitCode = 1
    }
    await browser.close()
  }
}

main().catch((err) => {
  console.error('[FAIL]', err && err.stack ? err.stack : err)
  process.exit(1)
})
