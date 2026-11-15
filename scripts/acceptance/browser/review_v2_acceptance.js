/**
 * Acceptance for the shadcn-vue review page: tabs, review flow, sessions,
 * warnings, source invalidation, expiry and statistics charts.
 *
 * Prereqs: stack running, owner credentials in deployments/.env,
 * puppeteer-core available via NODE_PATH, Chrome for Testing available.
 *
 * Usage:
 *   NODE_PATH=frontend/node_modules CHROME_PATH=".../chrome.exe" \
 *     node scripts/acceptance/browser/review_v2_acceptance.js
 */
const puppeteer = require('puppeteer-core')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { spawnSync } = require('child_process')

const ROOT = process.env.WELLREAD_ROOT || path.resolve(__dirname, '../../..')
const CHROME = process.env.CHROME_PATH || 'D:/chrome-for-test/chrome/win64-157.0.8103.38/chrome-win64/chrome.exe'
const BASE = process.env.WELLREAD_WEB || 'http://localhost:6151'
const OUT = process.env.ACCEPT_OUT || path.join(os.tmpdir(), 'wellread_accept')
fs.mkdirSync(OUT, { recursive: true })

const RUN_ID = `v2-${Date.now().toString(36)}`
const GROUP_NAME = `验收V2-${RUN_ID}`
const CARD_COUNT = 20
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

function psql(sql) {
  const res = spawnSync(
    'docker',
    ['exec', 'Gaze-postgres', 'psql', '-U', 'root', '-d', 'WellRead', '-t', '-A', '-c', sql],
    { encoding: 'utf8' },
  )
  if (res.status !== 0) throw new Error((res.stderr || res.stdout || 'psql failed').trim())
  return (res.stdout || '').trim()
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

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
  await setInput(page, await page.$('input[type="text"]'), env.OWNER_EMAIL)
  await setInput(page, await page.$('input[type="password"]'), env.OWNER_PASSWORD)
  await Promise.all([
    page.waitForFunction(() => location.pathname.includes('/platform'), { timeout: 20000 }),
    clickText(page, 'button', '登录', { exact: true }),
  ])
}

async function gotoReview(page) {
  await page.goto(BASE + '/platform/review', { waitUntil: 'networkidle2' })
  await page.waitForSelector('[data-testid="review-page"]', { timeout: 20000 })
  await page.waitForFunction((name) => {
    return [...document.querySelectorAll('[data-testid="review-group-row"]')]
      .some((row) => row.getAttribute('data-group-name') === name)
  }, { timeout: 20000 }, GROUP_NAME)
}

async function selectGroup(page) {
  const ok = await page.evaluate((name) => {
    const row = [...document.querySelectorAll('[data-testid="review-group-row"]')]
      .find((item) => item.getAttribute('data-group-name') === name)
    const button = row?.querySelector('button')
    if (!button) return false
    button.click()
    return true
  }, GROUP_NAME)
  if (!ok) throw new Error('group row not found')
  await page.waitForFunction((name) => {
    const row = [...document.querySelectorAll('[data-testid="review-group-row"]')]
      .find((item) => item.getAttribute('data-group-name') === name)
    return row?.getAttribute('data-active') === 'true'
  }, { timeout: 10000 }, GROUP_NAME)
}

async function openTab(page, tab) {
  await page.click(`[data-testid="review-tab-${tab}"]`)
  if (tab === 'cards') {
    await page.waitForSelector('[data-testid="review-card-row"]', { timeout: 20000 })
  }
  if (tab === 'stats') {
    await page.waitForSelector('[data-testid="review-stats"]', { timeout: 30000 })
  }
}

async function seed(page) {
  const sourceRow = psql(
    "select knowledge_id || '|' || chunk_id from review_card_sources where source_type = 'knowledge_chunk' limit 1",
  )
  const [knowledgeID, chunkID] = sourceRow ? sourceRow.split('|') : ['v2-knowledge', 'v2-chunk']
  const groupRes = await api(page, 'POST', '/api/v1/review/groups', { name: GROUP_NAME })
  if (groupRes.status !== 201) throw new Error(`create group failed: ${groupRes.status}`)
  const groupID = groupRes.data.data.id
  const cardIDs = []
  for (let i = 1; i <= CARD_COUNT; i += 1) {
    const res = await api(page, 'POST', '/api/v1/review/cards', {
      group_id: groupID,
      question: `[${RUN_ID}] V2 验收题 ${i}：这是第 ${i} 个用于验收新复习页的知识点。`,
      answer: `第 ${i} 题的标准答案：V2 验收答案 ${i}。`,
      explanation: `第 ${i} 题解析：用于验证三标签、会话恢复、预警与统计。`,
      tags: ['验收V2'],
      sources: [{
        source_type: 'knowledge_chunk',
        knowledge_id: knowledgeID,
        chunk_id: chunkID,
        excerpt: `第 ${i} 题的来源摘录。`,
        is_primary: true,
      }],
    })
    if (res.status !== 201) throw new Error(`create card ${i} failed: ${res.status} ${JSON.stringify(res.data)}`)
    cardIDs.push(res.data.data.id)
  }
  record('seed', `group=${GROUP_NAME} cards=${cardIDs.length}`)
  return { groupID, cardIDs }
}

async function rateCurrent(page, key) {
  const question = await page.$eval('[data-testid="review-question"]', (el) => el.innerText)
  const show = await page.$('[data-testid="review-show-answer"]')
  if (show) await show.click()
  await page.waitForSelector('[data-testid="review-answer"]', { timeout: 5000 })
  await page.click(`[data-testid="review-rate-${key}"]`)
  await page.waitForFunction((prev) => {
    const current = document.querySelector('[data-testid="review-question"]')?.innerText || ''
    return !document.querySelector('[data-testid="review-run"]') || current !== prev || document.body.innerText.includes('本轮完成')
  }, { timeout: 10000 }, question)
}

async function runRound(page, plan, roundName) {
  await page.waitForSelector('[data-testid="review-run"]', { timeout: 15000 })
  let steps = 0
  while (steps < 80) {
    const running = await page.evaluate((done) => ({
      run: !!document.querySelector('[data-testid="review-run"]'),
      finished: document.body.innerText.includes(done),
      question: document.querySelector('[data-testid="review-question"]')?.innerText || '',
    }), '本轮完成')
    if (!running.run || running.finished) break
    const match = running.question.match(/题\s*(\d+)/)
    const index = match ? Number(match[1]) : 0
    const keyMap = { 1: 'forgot', 2: 'fuzzy', 3: 'remembered' }
    await rateCurrent(page, keyMap[plan(index)])
    steps += 1
  }
  const finished = await page.evaluate(() => document.body.innerText.includes('本轮完成'))
  if (!finished) throw new Error(`${roundName} did not finish`)
  record(roundName, `completed ${steps} ratings`)
}

async function finishAndHome(page) {
  await waitText(page, '本轮完成')
  await clickText(page, 'button', '返回复习首页', { exact: true })
  await page.waitForSelector('[data-testid="review-page"]', { timeout: 15000 })
  await selectGroup(page)
}

async function selectAllCardsInOrder(page) {
  const count = await page.evaluate((runId) => {
    const rows = [...document.querySelectorAll('[data-testid="review-card-row"]')]
    const first = rows.find((row) => row.innerText.includes(`[${runId}] V2 验收题 1：`))
    if (first) first.querySelector('[data-slot="checkbox"]')?.click()
    rows.filter((row) => row !== first).forEach((row) => row.querySelector('[data-slot="checkbox"]')?.click())
    return rows.length
  }, RUN_ID)
  if (count !== CARD_COUNT) throw new Error(`card rows=${count}, expected ${CARD_COUNT}`)
}

async function findCardRowText(page, index) {
  return page.evaluate((runId, i) => {
    const row = [...document.querySelectorAll('[data-testid="review-card-row"]')]
      .find((item) => item.innerText.includes(`[${runId}] V2 验收题 ${i}：`))
    return row?.innerText || ''
  }, RUN_ID, index)
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
  let seeded = null
  try {
    await login(page)
    record('login', 'owner logged in')
    seeded = await seed(page)
    await gotoReview(page)
    await selectGroup(page)
    record('home', 'group selected')

    const tabIds = await page.$$eval('[data-testid^="review-tab-"]', (nodes) => nodes.map((node) => node.getAttribute('data-testid')))
    if (!['review-tab-review', 'review-tab-cards', 'review-tab-stats'].every((id) => tabIds.includes(id))) {
      throw new Error(`three tabs missing: ${tabIds.join(',')}`)
    }
    record('tabs', 'review/cards/stats visible')

    await page.click('[data-testid="review-start"]')
    await runRound(page, (i) => (i <= 2 ? 1 : i <= 18 ? 3 : 2), 'round1')
    await finishAndHome(page)
    record('round1', 'daily session completed')

    await openTab(page, 'cards')
    await selectAllCardsInOrder(page)
    await waitText(page, `开始复习所选 (${CARD_COUNT})`)
    await clickText(page, 'button', `开始复习所选 (${CARD_COUNT})`)
    await page.waitForSelector('[data-testid="review-run"]', { timeout: 15000 })
    await rateCurrent(page, 'forgot')
    await rateCurrent(page, 'remembered')
    await page.reload({ waitUntil: 'networkidle2' })
    await waitText(page, '有未完成会话：2/20')
    record('resume', 'session recovered after reload at 2/20')
    await page.click('[data-testid="review-active-session"] button')
    await page.waitForSelector('[data-testid="review-run"]', { timeout: 15000 })
    const progress = await page.$eval('[data-testid="review-progress"]', (el) => el.innerText)
    if (!progress.includes('第 3 / 20 张')) throw new Error(`resume progress wrong: ${progress}`)
    record('resume', progress.replace(/\n+/g, ' | '))
    await runRound(page, () => 3, 'round2')
    await finishAndHome(page)

    await openTab(page, 'cards')
    const warningRow = await findCardRowText(page, 1)
    if (!warningRow.includes('易忘')) throw new Error(`warning badge missing: ${warningRow}`)
    record('warning', 'card 1 shows 易忘')

    const opened = await page.evaluate((runId) => {
      const row = [...document.querySelectorAll('[data-testid="review-card-row"]')]
        .find((item) => item.innerText.includes(`[${runId}] V2 验收题 1：`))
      const button = row?.querySelector('[data-testid="review-card-open"]')
      if (!button) return false
      button.click()
      return true
    }, RUN_ID)
    if (!opened) throw new Error('card 1 performance trigger missing')
    await page.waitForSelector('[data-slot="sheet-content"]', { timeout: 10000 })
    await waitText(page, '卡片表现')
    const sheetText = await page.$eval('[data-slot="sheet-content"]', (el) => el.innerText)
    if (!sheetText.includes('易忘') || !sheetText.includes('稳定度') || !sheetText.includes('复习历史')) {
      throw new Error(`performance sheet incomplete: ${sheetText.slice(0, 400)}`)
    }
    await page.screenshot({ path: path.join(OUT, 'review-v2-performance.png'), fullPage: true })
    await page.keyboard.press('Escape')
    await page.waitForSelector('[data-slot="sheet-content"]', { hidden: true, timeout: 5000 }).catch(() => {})
    record('performance', 'sheet shows warning, stability, history')

    const pauseSession = await api(page, 'POST', '/api/v1/review/sessions', {
      mode: 'manual',
      group_id: seeded.groupID,
      card_ids: [seeded.cardIDs[0]],
    })
    if (pauseSession.status !== 200 && pauseSession.status !== 201) throw new Error(`create pause session failed: ${pauseSession.status}`)
    await page.reload({ waitUntil: 'networkidle2' })
    await waitText(page, '有未完成会话：0/1')
    await page.click('[data-testid="review-active-session"] button')
    await page.waitForSelector('[data-testid="review-run"]', { timeout: 15000 })
    await rateCurrent(page, 'forgot')
    await finishAndHome(page)
    await openTab(page, 'cards')
    const pausedRow = await findCardRowText(page, 1)
    if (!pausedRow.includes('暂停')) throw new Error(`auto-pause missing: ${pausedRow}`)
    record('pause', 'third forget auto-suspends card 1')

    const card20 = seeded.cardIDs[19]
    const detail = await api(page, 'GET', `/api/v1/review/cards/${card20}`)
    const sourceID = detail.data?.data?.sources?.[0]?.id
    if (!sourceID) throw new Error('card 20 source missing')
    const mark = await api(page, 'POST', `/api/v1/review/cards/${card20}/sources/${sourceID}/unavailable`)
    if (mark.status !== 200) throw new Error(`mark source unavailable failed: ${mark.status}`)
    const orphan = await api(page, 'GET', `/api/v1/review/cards/${card20}`)
    if (orphan.data?.data?.card?.status !== 'orphaned') throw new Error(`card 20 status=${orphan.data?.data?.card?.status}`)
    await page.reload({ waitUntil: 'networkidle2' })
    await selectGroup(page)
    await openTab(page, 'cards')
    const orphanRow = await findCardRowText(page, 20)
    if (!orphanRow.includes('孤儿')) throw new Error(`orphan tag missing: ${orphanRow}`)
    record('source', 'all sources unavailable => orphaned status visible')

    const expiry = await api(page, 'POST', '/api/v1/review/sessions', {
      mode: 'manual',
      group_id: seeded.groupID,
      card_ids: seeded.cardIDs.slice(1, 3),
    })
    if (expiry.status !== 200 && expiry.status !== 201) throw new Error(`create expiry session failed: ${expiry.status}`)
    const expirySession = expiry.data.data.session
    psql(`update review_sessions set review_day = review_day - interval '1 day', started_at = started_at - interval '1 day', updated_at = now() where id = '${expirySession.id}'`)
    const active = await api(page, 'GET', '/api/v1/review/sessions/active')
    if (active.data?.data) throw new Error('active session should be expired')
    const expired = await api(page, 'GET', `/api/v1/review/sessions/${expirySession.id}`)
    if (expired.data?.data?.session?.status !== 'expired') throw new Error(`session status=${expired.data?.data?.session?.status}`)
    await page.reload({ waitUntil: 'networkidle2' })
    if ((await page.evaluate(() => document.body.innerText)).includes('有未完成会话')) throw new Error('expired session still visible')
    record('expiry', 'backdated session expired and hidden')

    await openTab(page, 'stats')
    const statsText = await page.$eval('[data-testid="review-stats"]', (el) => el.innerText)
    if (!statsText.includes('复习次数') || !statsText.includes('保留率') || !statsText.includes('会话历史')) {
      throw new Error(`stats content incomplete: ${statsText.slice(0, 400)}`)
    }
    await page.waitForFunction(() => document.querySelectorAll('[data-testid="review-stats"] canvas').length >= 4, { timeout: 15000 })
    await clickText(page, 'button', '7 天', { exact: true })
    await sleep(600)
    await clickText(page, 'button', '90 天', { exact: true })
    await sleep(600)
    record('stats', '7/30/90 controls and charts rendered')
    const desktopOverflow = await page.$eval('[data-testid="review-page"]', (el) => el.scrollWidth - el.clientWidth)
    if (desktopOverflow > 1) throw new Error(`review page overflows at 1440px: ${desktopOverflow}px`)
    record('layout', `1440px overflow=${desktopOverflow}px`)
    await page.screenshot({ path: path.join(OUT, 'review-v2-stats.png'), fullPage: true })

    await page.click('[data-testid="review-tab-review"]')
    const groupRow = await page.$('[data-testid="review-group-row"][data-group-name="' + GROUP_NAME + '"]')
    if (!groupRow) throw new Error('group row missing for context menu')
    await groupRow.click({ button: 'right' })
    await page.waitForSelector('[data-slot="context-menu-content"]', { timeout: 5000 })
    await waitText(page, '新建子卡组')
    await page.keyboard.press('Escape')
    record('context-menu', 'group context menu opened')

    await page.evaluate(() => {
      document.documentElement.setAttribute('theme-mode', 'dark')
      document.documentElement.classList.add('dark')
    })
    await sleep(400)
    await page.screenshot({ path: path.join(OUT, 'review-v2-dark.png'), fullPage: true })
    await page.evaluate(() => {
      document.documentElement.setAttribute('theme-mode', 'light')
      document.documentElement.classList.remove('dark')
    })
    await page.setViewport({ width: 600, height: 900 })
    await sleep(400)
    const overflow = await page.$eval('[data-testid="review-page"]', (el) => el.scrollWidth - el.clientWidth)
    await page.screenshot({ path: path.join(OUT, 'review-v2-narrow.png'), fullPage: true })
    record('responsive', `600px overflow=${overflow}px`)
    if (overflow > 1) throw new Error(`review page overflows at 600px: ${overflow}px`)
    await page.setViewport({ width: 1440, height: 960 })
    await sleep(200)

    record('console', `errors=${consoleErrors.length} pageErrors=${pageErrors.length}`)
    if (consoleErrors.length || pageErrors.length) {
      record('console.detail', JSON.stringify({ consoleErrors, pageErrors }).slice(0, 1200))
    }
    record('result', 'PASS')
  } catch (err) {
    await page.screenshot({ path: path.join(OUT, 'review-v2-failure.png'), fullPage: true }).catch(() => {})
    record('FAIL', String(err && err.stack ? err.stack.split('\n')[0] : err))
    process.exitCode = 1
  } finally {
    if (seeded) {
      try {
        psql(`delete from review_logs where card_id in (select id from review_cards where group_id = '${seeded.groupID}') or session_id in (select id from review_sessions where group_id = '${seeded.groupID}')`)
        psql(`delete from review_sessions where group_id = '${seeded.groupID}'`)
        psql(`delete from review_card_sources where card_id in (select id from review_cards where group_id = '${seeded.groupID}')`)
        psql(`delete from review_cards where group_id = '${seeded.groupID}'`)
        psql(`delete from review_groups where id = '${seeded.groupID}'`)
        record('cleanup', 'test group and review data removed')
      } catch (err) {
        record('cleanup.FAIL', String(err && err.message ? err.message : err))
      }
    }
    fs.writeFileSync(path.join(OUT, 'review-v2-report.json'), JSON.stringify({ runId: RUN_ID, log }, null, 2))
    await browser.close()
  }
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
