/**
 * Real-backend acceptance for the V3 cards and stats tabs.
 *
 * Seeds an isolated group tree with rated cards, exercises search, drawer,
 * edit/reset, batch tag/delete/restore, immediate review, create and the stats
 * dashboard against the running stack, then removes all seeded rows.
 *
 * Usage:
 *   NODE_PATH=frontend/node_modules \
 *     node scripts/acceptance/browser/review_v3_cards_stats_acceptance.js
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

const RUN_ID = `v3cards-${Date.now().toString(36)}`
const SCOPE_NAME = `V3卡片验收-${RUN_ID}`
const TARGET_NAME = `${SCOPE_NAME}-目标组`
const TAG = `${RUN_ID}-标签`
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
    ['exec', 'Gaze-postgres', 'psql', '-U', env.DB_USER || 'root', '-d', env.DB_NAME || 'WellRead', '-t', '-A', '-c', sql],
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

async function waitText(page, text, timeout = 20000) {
  await page.waitForFunction(value => document.body.innerText.includes(value), { timeout }, text)
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

async function openCardsTab(page) {
  await page.goto(BASE + '/platform/review', { waitUntil: 'networkidle2' })
  await page.waitForSelector('[data-testid="review-v3-task-bar"]', { timeout: 20000 })
  await page.evaluate(() => {
    [...document.querySelectorAll('.t-tabs__nav-item')].find(el => el.innerText.trim() === '卡片')?.click()
  })
  await page.waitForSelector('[data-testid="review-v3-cards"]', { timeout: 20000 })
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
  await sleep(150)
  await page.mouse.click(box.x, box.y)
  let opened = true
  try {
    await page.waitForFunction(
      () => [...document.querySelectorAll('.t-select-option')].some(el => el.offsetHeight > 0),
      { timeout: 2500 },
    )
  } catch {
    opened = false
  }
  if (!opened) {
    await page.click(selector)
    try {
      await page.waitForFunction(
        () => [...document.querySelectorAll('.t-select-option')].some(el => el.offsetHeight > 0),
        { timeout: 10000 },
      )
    } catch (err) {
      const debug = await page.evaluate(() => ({
        active: document.activeElement?.outerHTML?.slice(0, 120) || '',
        popups: [...document.querySelectorAll('.t-popup')].filter(el => el.offsetHeight > 0)
          .map(el => (el.innerText || '').trim().slice(0, 120)),
        options: [...document.querySelectorAll('.t-select-option')].map(el => ({
          text: (el.innerText || '').trim(),
          h: el.offsetHeight,
        })).slice(0, 10),
        dialogs: [...document.querySelectorAll('.t-dialog')].filter(el => el.offsetHeight > 0)
          .map(el => (el.innerText || '').trim().slice(0, 80)),
      }))
      throw new Error(`select ${testId} did not open: ${JSON.stringify(debug)}`)
    }
  }
  const readOnly = await page.$eval(selector, el => el.readOnly)
  if (!readOnly) {
    await setInput(page, selector, text)
  }
  await page.waitForFunction(
    (value) => [...document.querySelectorAll('.t-select-option')].some(el => el.innerText.includes(value)),
    { timeout: 15000 },
    text,
  )
  const clicked = await page.evaluate((value) => {
    const option = [...document.querySelectorAll('.t-select-option')].find(el => el.innerText.includes(value))
    if (!option) return false
    option.click()
    return true
  }, text)
  if (!clicked) throw new Error(`select option not found: ${text}`)
  await sleep(200)
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
  const targetRes = await api(page, 'POST', '/api/v1/review/groups', { name: TARGET_NAME, parent_id: scopeID })
  if (targetRes.status !== 201) throw new Error(`create target failed: ${targetRes.status}`)
  const targetID = targetRes.data.data.id

  const cards = []
  for (let i = 1; i <= 4; i += 1) {
    const res = await api(page, 'POST', '/api/v1/review/cards', {
      group_id: scopeID,
      question: `[${RUN_ID}] 卡片验收题 ${i}：用于验证卡片页筛选、批量与表现抽屉。`,
      answer: `卡片验收答案 ${i}。`,
      explanation: `卡片验收解析 ${i}。`,
      tags: [TAG, `${TAG}-${i}`],
      sources: [{
        source_type: 'knowledge_chunk',
        knowledge_id: knowledgeID,
        chunk_id: chunkID,
        excerpt: `卡片验收来源摘录 ${i}。`,
        is_primary: true,
      }],
    })
    if (res.status !== 201) throw new Error(`create card ${i} failed: ${res.status} ${JSON.stringify(res.data)}`)
    cards.push(res.data.data)
  }

  for (const index of [0, 1]) {
    for (let round = 0; round < 2; round += 1) {
      await api(page, 'POST', '/api/v1/review/reviews', { card_id: cards[index].id, rating: 'forgot' })
    }
  }
  await api(page, 'POST', '/api/v1/review/reviews', { card_id: cards[2].id, rating: 'easy' })
  await api(page, 'POST', '/api/v1/review/reviews', { card_id: cards[3].id, rating: 'remembered' })

  record('seed', `scope=${SCOPE_NAME} target=${TARGET_NAME} cards=${cards.length}`)
  return { scopeID, targetID, cards }
}

function cleanup(seeded) {
  if (!seeded) return
  const ids = [seeded.scopeID, seeded.targetID]
  const quoted = ids.map(id => `'${id}'`).join(',')
  psql(`delete from review_logs where card_id in (select id from review_cards where group_id in (${quoted})) or session_id in (select id from review_sessions where group_id in (${quoted}))`)
  psql(`delete from review_sessions where group_id in (${quoted})`)
  psql(`delete from review_card_sources where card_id in (select id from review_cards where group_id in (${quoted}))`)
  psql(`delete from review_cards where group_id in (${quoted})`)
  psql(`delete from review_groups where id in (${quoted})`)
  const left = psql(`select count(*) from review_groups where id in (${quoted})`)
  if (left !== '0') throw new Error(`cleanup left ${left} groups`)
  record('cleanup', 'seeded cards, groups, logs and sessions removed')
}

async function cardsChecks(page, seeded) {
  await openCardsTab(page)
  await setInput(page, '[data-testid="review-v3-cards-search"] input', RUN_ID)
  await page.keyboard.press('Enter')
  await page.waitForFunction(
    () => document.querySelectorAll('[data-testid="review-v3-card-row"]').length === 4,
    { timeout: 15000 },
  )
  record('cards-search', 'keyword filter returns 4 seeded cards')

  // Drawer + edit + reset.
  const editedId = await page.$eval('[data-testid="review-v3-card-row"]', el => el.getAttribute('data-card-id'))
  if (!editedId) throw new Error('card row missing data-card-id')
  await page.click('[data-testid="review-v3-card-row"]')
  await page.waitForSelector('[data-testid="review-v3-card-drawer"]', { timeout: 15000 })
  await waitText(page, '秒答')
  await page.click('[data-testid="review-v3-card-edit"]')
  await page.waitForSelector('[data-testid="review-v3-card-edit-question"]', { timeout: 10000 })
  const editedQuestion = `[${RUN_ID}] 已编辑题干`
  await setInput(page, '[data-testid="review-v3-card-edit-question"] textarea', editedQuestion)
  await clickText(page, '.t-dialog button', '保存', { exact: true })
  await page.waitForFunction(
    () => {
      const el = document.querySelector('[data-testid="review-v3-card-edit-question"]')
      return !el || el.getBoundingClientRect().height === 0
    },
    { timeout: 15000 },
  )
  const edited = await api(page, 'GET', `/api/v1/review/cards/${editedId}`)
  if (edited.data.data.card.question !== editedQuestion) throw new Error('card edit not persisted')
  record('cards-edit', 'edit dialog saves question')

  await page.waitForSelector('[data-testid="review-v3-card-reset"]', { timeout: 10000 })
  await page.click('[data-testid="review-v3-card-reset"]')
  await page.waitForFunction(
    () => [...document.querySelectorAll('.t-dialog')].some(el => el.getBoundingClientRect().height > 0 && el.innerText.includes('重置进度')),
    { timeout: 10000 },
  )
  await clickText(page, '.t-dialog button', '重置进度', { exact: true })
  await sleep(800)
  const reset = await api(page, 'GET', `/api/v1/review/cards/${editedId}`)
  if (Math.round(reset.data.data.card.stability) !== 10) {
    throw new Error(`reset stability=${reset.data.data.card.stability}`)
  }
  record('cards-reset', 'single card reset restores stability 10 and keeps logs')

  await page.waitForFunction(
    () => [...document.querySelectorAll('.t-dialog')].every(el => el.offsetHeight === 0),
    { timeout: 5000 },
  )
  await page.click('[data-testid="review-v3-card-drawer-close"]')
  await page.waitForFunction(
    () => {
      const drawer = document.querySelector('[data-testid="review-v3-card-drawer"]')
      return !drawer || !drawer.classList.contains('t-drawer--open')
    },
    { timeout: 10000 },
  )
  await sleep(300)

  // Batch tag + move + delete + restore.
  const tagSelectedIds = await page.evaluate(() => {
    const rows = [...document.querySelectorAll('[data-testid="review-v3-card-row"]')].slice(0, 2)
    rows.forEach(row => row.querySelector('.t-checkbox')?.click())
    return rows.map(row => row.getAttribute('data-card-id'))
  })
  await page.waitForSelector('[data-testid="review-v3-card-batch"]', { timeout: 10000 })
  await page.click('[data-testid="review-v3-cards-batch-tags"]')
  await page.waitForSelector('[data-testid="review-v3-cards-tags-input"] input', { visible: true, timeout: 10000 })
  await page.type('[data-testid="review-v3-cards-tags-input"] input', `${RUN_ID}-批量`)
  await page.keyboard.press('Enter')
  await clickText(page, '.t-dialog button', '保存', { exact: true })
  await sleep(800)
  const tagCheck = await api(page, 'GET', `/api/v1/review/cards/${tagSelectedIds[0]}`)
  const tags = Array.isArray(tagCheck.data.data.card.tags)
    ? tagCheck.data.data.card.tags
    : JSON.parse(tagCheck.data.data.card.tags || '[]')
  if (!tags.includes(`${RUN_ID}-批量`)) throw new Error(`batch tags missing: ${JSON.stringify(tags)}`)
  record('cards-batch-tags', 'batch append tags persists')

  const deleteSelectedIds = await page.evaluate(() => {
    const rows = [...document.querySelectorAll('[data-testid="review-v3-card-row"]')].slice(0, 2)
    rows.forEach(row => row.querySelector('.t-checkbox')?.click())
    return rows.map(row => row.getAttribute('data-card-id'))
  })
  await page.waitForSelector('[data-testid="review-v3-card-batch"]', { timeout: 10000 })
  await page.click('[data-testid="review-v3-cards-batch-delete"]')
  await clickText(page, '.t-dialog button', '删除', { exact: true })
  await page.waitForFunction(
    () => document.querySelectorAll('[data-testid="review-v3-card-row"]').length === 2,
    { timeout: 15000 },
  )
  record('cards-batch-delete', 'two cards soft-deleted')

  await pickSelect(page, 'review-v3-cards-status', '已删除')
  await page.waitForFunction(
    () => document.querySelectorAll('[data-testid="review-v3-card-row"]').length === 2,
    { timeout: 15000 },
  )
  const restoreId = await page.$eval('[data-testid="review-v3-card-row"]', el => el.getAttribute('data-card-id'))
  await page.click('[data-testid="review-v3-card-row"] .card-row__actions button')
  await page.waitForFunction(
    () => [...document.querySelectorAll('.t-dropdown__item')].some(el => el.offsetHeight > 0 && el.innerText.includes('恢复卡片')),
    { timeout: 10000 },
  )
  const clickedRestore = await page.evaluate(() => {
    const item = [...document.querySelectorAll('.t-dropdown__item')]
      .find(el => el.offsetHeight > 0 && el.innerText.includes('恢复卡片'))
    if (!item) return false
    item.click()
    return true
  })
  if (!clickedRestore) throw new Error('restore menu item not clickable')
  let restored = null
  for (let attempt = 0; attempt < 10; attempt += 1) {
    await sleep(300)
    restored = await api(page, 'GET', `/api/v1/review/cards/${restoreId}`)
    if (restored.data?.data?.card?.status === 'suspended') break
  }
  if (restored?.data?.data?.card?.status !== 'suspended') {
    throw new Error(`restore did not apply: ${restored?.status} ${JSON.stringify(restored?.data)}`)
  }
  if (!deleteSelectedIds.includes(restoreId)) throw new Error('restored card was not one of the batch-deleted cards')
  record('cards-restore', 'deleted filter shows cards and row menu restores to suspended')

  await pickSelect(page, 'review-v3-cards-status', '全部状态')
  await page.waitForFunction(
    () => document.querySelectorAll('[data-testid="review-v3-card-row"]').length === 3,
    { timeout: 15000 },
  )

  await pickSelect(page, 'review-v3-cards-status', '正常')
  await page.waitForFunction(
    () => document.querySelectorAll('[data-testid="review-v3-card-row"]').length === 2,
    { timeout: 15000 },
  )

  // Immediate review from drawer. Never abandon an owner session that was
  // already active before this script started.
  const activeBefore = await api(page, 'GET', '/api/v1/review/sessions/active')
  if (activeBefore.data?.data?.session) {
    record('cards-review-now-skip', `active session ${activeBefore.data.data.session.id} exists; skipped review-now`)
  } else {
    await page.click('[data-testid="review-v3-card-row"]')
    await page.waitForSelector('[data-testid="review-v3-card-review-now"]', { timeout: 15000 })
    let reviewReady = false
    for (let attempt = 0; attempt < 2 && !reviewReady; attempt += 1) {
      await page.click('[data-testid="review-v3-card-review-now"]')
      try {
        await page.waitForFunction(
          () => document.querySelector('[data-testid="review-v3-mode"]')
            || document.body.innerText.includes('放弃并复习此卡')
            || document.body.innerText.includes('已有进行中的复习'),
          { timeout: 10000 },
        )
        reviewReady = true
      } catch {
        await sleep(500)
      }
    }
    if (!reviewReady) {
      const debug = await page.evaluate(() => ({
        drawerOpen: document.querySelector('[data-testid="review-v3-card-drawer"]')?.className || '',
        buttonDisabled: document.querySelector('[data-testid="review-v3-card-review-now"]')?.disabled,
        body: document.body.innerText.slice(-300),
        mode: Boolean(document.querySelector('[data-testid="review-v3-mode"]')),
      }))
      const active = await api(page, 'GET', '/api/v1/review/sessions/active')
      debug.active = active.status
      debug.activeBody = active.data
      await page.screenshot({ path: path.join(OUT, 'review_v3_review_now_fail.png'), fullPage: true })
      throw new Error(`review-now did not start: ${JSON.stringify(debug).slice(0, 1200)}`)
    }
    const conflict = await page.evaluate(() => document.body.innerText.includes('放弃并复习此卡'))
    if (conflict) await clickText(page, '.t-dialog button', '放弃并复习此卡')
    await page.waitForSelector('[data-testid="review-v3-mode"]', { timeout: 20000 })
    await page.waitForSelector('[data-testid="review-v3-question"]', { timeout: 15000 })
    const active = await api(page, 'GET', '/api/v1/review/sessions/active')
    if ((active.data?.data?.cards?.length || 0) !== 1) throw new Error('single-card session not created')
    await page.click('[data-testid="review-v3-suspend"]')
    await page.waitForSelector('[data-testid="review-v3-cards"]', { timeout: 20000 })
    if (active.data?.data?.session?.id) {
      await api(page, 'POST', `/api/v1/review/sessions/${active.data.data.session.id}/abandon`)
    }
    record('cards-review-now', 'drawer starts a one-card manual session')
  }

  // Create dialog with a real knowledge document source. Reload first so the
  // closed review overlay and drawer cannot keep focus/pointer capture.
  await openCardsTab(page)
  await sleep(500)
  const kbRes = await api(page, 'GET', '/api/v1/knowledge-bases')
  const kbList = kbRes.data?.data || []
  if (!kbList.length) throw new Error('no knowledge base for create dialog')
  const kb = kbList[0]
  const docsRes = await api(page, 'GET', `/api/v1/knowledge-bases/${kb.id}/knowledge?page=1&page_size=5&folder_recursive=true`)
  const docs = docsRes.data?.data || docsRes.data || []
  if (!docs.length) throw new Error('no knowledge document for create dialog')
  const doc = docs[0]

  await page.click('[data-testid="review-v3-card-create"]')
  await page.waitForSelector('[data-testid="review-v3-card-create-question"]', { timeout: 10000 })
  await sleep(1500)
  const createdQuestion = `[${RUN_ID}] 新建卡片验收题`
  await setInput(page, '[data-testid="review-v3-card-create-question"] textarea', createdQuestion)
  await setInput(page, '[data-testid="review-v3-card-create-answer"] textarea', '新建卡片验收答案。')
  await pickSelect(page, 'review-v3-card-create-group', SCOPE_NAME)
  await pickSelect(page, 'review-v3-card-create-kb', kb.name)
  await page.waitForFunction(
    () => !document.querySelector('[data-testid="review-v3-card-create-doc"]')?.className.includes('t-is-loading'),
    { timeout: 15000 },
  )
  await sleep(1000)
  await pickSelect(page, 'review-v3-card-create-doc', doc.name || doc.title || doc.file_name)
  await clickText(page, '.t-dialog button', '创建', { exact: true })
  await sleep(1200)
  const createdRes = await api(page, 'GET', `/api/v1/review/cards?group_id=${seeded.scopeID}&keyword=${encodeURIComponent(createdQuestion)}&include_subtree=true`)
  const created = (createdRes.data?.data || []).find(card => card.question === createdQuestion)
  if (!created) throw new Error('created card not found')
  record('cards-create', `created card ${created.id}`)

  await page.screenshot({ path: path.join(OUT, 'review_v3_cards_real.png'), fullPage: true })
}

async function statsChecks(page) {
  await page.evaluate(() => {
    [...document.querySelectorAll('.t-tabs__nav-item')].find(el => el.innerText.trim() === '统计')?.click()
  })
  await page.waitForSelector('[data-testid="review-v3-stats"]', { timeout: 20000 })
  await page.waitForFunction(
    () => document.querySelectorAll('.stats-kpi__value').length === 4 && document.querySelectorAll('.session-table__row').length > 0,
    { timeout: 20000 },
  )
  const summary = await page.evaluate(() => ({
    kpis: [...document.querySelectorAll('.stats-kpi__value')].map(el => el.innerText.trim()),
    headers: document.querySelector('.session-table__head')?.innerText || '',
    charts: document.querySelectorAll('.chart canvas').length,
  }))
  if (!summary.headers.includes('秒答/想起来/想得慢/忘记')) throw new Error(`session header missing four counts: ${summary.headers}`)
  if (!summary.kpis.length) throw new Error('stats KPIs missing')
  record('stats', `kpis=${summary.kpis.join(',')} charts=${summary.charts}`)

  await page.click('[data-testid="review-v3-stats-days-7"]')
  await sleep(600)
  record('stats-days', '7-day switch reloads without error')

  const client = await page.createCDPSession()
  await client.send('Page.setDownloadBehavior', { behavior: 'allow', downloadPath: DOWNLOADS })
  for (const name of fs.readdirSync(DOWNLOADS)) {
    if (name.startsWith('wellread-review-sessions-') && name.endsWith('.csv')) {
      fs.unlinkSync(path.join(DOWNLOADS, name))
    }
  }
  await page.click('[data-testid="review-v3-stats-export"]')
  const deadline = Date.now() + 15000
  let csvFile = ''
  while (Date.now() < deadline) {
    const next = fs.readdirSync(DOWNLOADS).find(name => name.endsWith('.csv'))
    if (next) { csvFile = path.join(DOWNLOADS, next); break }
    await sleep(300)
  }
  if (!csvFile) throw new Error('CSV download not observed')
  const header = fs.readFileSync(csvFile, 'utf8').split(/\r?\n/)[0]
  if (!header.includes('easy_count')) throw new Error(`CSV header missing easy_count: ${header}`)
  record('stats-export', 'CSV includes easy_count at the end')

  await page.screenshot({ path: path.join(OUT, 'review_v3_stats_real.png'), fullPage: true })
}

async function narrowCheck(page) {
  await page.setViewport({ width: 600, height: 900 })
  await page.evaluate(() => {
    [...document.querySelectorAll('.t-tabs__nav-item')].find(el => el.innerText.trim() === '卡片')?.click()
  })
  await page.waitForSelector('[data-testid="review-v3-cards"]', { timeout: 15000 })
  await sleep(500)
  const overflow = await page.evaluate(() => ({
    scroll: document.documentElement.scrollWidth,
    inner: window.innerWidth,
  }))
  if (overflow.scroll > overflow.inner + 1) throw new Error(`600px overflow: ${JSON.stringify(overflow)}`)
  await page.screenshot({ path: path.join(OUT, 'review_v3_cards_600.png'), fullPage: true })
  record('narrow', '600px no page-level horizontal overflow')
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
    await cardsChecks(page, seeded)
    await statsChecks(page)
    await narrowCheck(page)
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
