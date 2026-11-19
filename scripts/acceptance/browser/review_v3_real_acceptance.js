/**
 * Real-backend acceptance for the V3 review page.
 *
 * Unlike the mock suites, this script talks to the running stack and the
 * configured LLM: it seeds an isolated group tree, creates real ratings,
 * calls root-cause/split/angles for real, verifies the UI against the real
 * aggregate, then removes every seeded row by group id.
 *
 * Prereqs: stack running with V3 backend image, Vite dev server on
 * http://localhost:5174, owner credentials in deployments/.env, puppeteer-core
 * via NODE_PATH, Chrome for Testing available.
 *
 * Usage:
 *   NODE_PATH=frontend/node_modules \
 *     node scripts/acceptance/browser/review_v3_real_acceptance.js
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
fs.mkdirSync(OUT, { recursive: true })

const RUN_ID = `v3real-${Date.now().toString(36)}`
const SCOPE_NAME = `V3真实验收-${RUN_ID}`
const TAG_PREFIX = `V3真实验收-${RUN_ID}-T`
const TAGS = Array.from({ length: 11 }, (_, i) => `${TAG_PREFIX}${i + 1}`)
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
    ['exec', 'Gaze-postgres', 'psql', '-U', env.DB_USER || 'root', '-d', env.DB_NAME || 'WellRead', '-t', '-A', '-c', sql],
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
    page.waitForFunction(() => location.pathname === '/', { timeout: 20000 }),
    clickText(page, 'button', '登录', { exact: true }),
  ])
}

async function waitForHeatmap(page, timeout = 20000) {
  await page.waitForSelector('[data-testid="review-v3-task-bar"]', { timeout })
  await page.waitForSelector('[data-testid="review-v3-heatmap"] .heatmap__cell', { timeout })
}

async function readHeatmapDom(page) {
  return page.evaluate(() => {
    const cols = [...document.querySelectorAll('.heatmap__colhead')].map((el) => el.innerText.trim())
    const rows = [...document.querySelectorAll('.heatmap__rowhead')].map((el) => el.innerText.trim())
    const otherRow = rows.some((t) => t.includes('其他卡组'))
    const otherCol = cols.some((t) => t.includes('其他主题'))
    const expand = Boolean(document.querySelector('[data-testid="review-v3-heatmap-expand"]'))
    return { cols, rows, otherRow, otherCol, expand }
  })
}

async function clickHeatmapCell(page, rowText, colText) {
  const ok = await page.evaluate(({ rowText, colText }) => {
    const colheads = [...document.querySelectorAll('.heatmap__colhead')]
    const colIndex = colheads.findIndex((el) => el.innerText.trim() === colText)
    const rowheads = [...document.querySelectorAll('.heatmap__rowhead')]
    const rowIndex = rowheads.findIndex((el) => el.innerText.includes(rowText))
    if (colIndex < 0 || rowIndex < 0) return { ok: false, reason: `row=${rowIndex} col=${colIndex}` }
    const wraps = [...document.querySelectorAll('.heatmap__cellwrap')]
    const wrap = wraps[rowIndex * colheads.length + colIndex]
    const cell = wrap?.querySelector('.heatmap__cell')
    if (!cell) return { ok: false, reason: 'cell missing' }
    const clickable = cell.classList.contains('is-clickable')
    cell.click()
    return { ok: true, clickable }
  }, { rowText, colText })
  if (!ok.ok) throw new Error(`heatmap cell not found (${rowText} x ${colText}): ${ok.reason}`)
  return ok.clickable
}

async function seed(page) {
  const sourceRow = psql(
    "select knowledge_id || '|' || chunk_id from review_card_sources where source_type = 'knowledge_chunk' and status = 'active' limit 1",
  )
  if (!sourceRow) throw new Error('no active knowledge_chunk source to reuse')
  const [knowledgeID, chunkID] = sourceRow.split('|')

  const scopeRes = await api(page, 'POST', '/api/v1/review/groups', { name: SCOPE_NAME })
  if (scopeRes.status !== 201) throw new Error(`create scope failed: ${scopeRes.status} ${JSON.stringify(scopeRes.data)}`)
  const scopeID = scopeRes.data.data.id

  const groups = []
  const cards = []
  for (let i = 1; i <= 10; i += 1) {
    const groupRes = await api(page, 'POST', '/api/v1/review/groups', {
      name: `${SCOPE_NAME}-G${i}`,
      parent_id: scopeID,
    })
    if (groupRes.status !== 201) throw new Error(`create group ${i} failed: ${groupRes.status}`)
    const groupID = groupRes.data.data.id
    // Card 10 stays untagged so the knowledge-base fallback column is real.
    const tags = i === 1 ? [TAGS[0], TAGS[1]]
      : i === 2 ? [TAGS[0], TAGS[2]]
        : i === 3 ? [TAGS[3]]
          : i === 4 ? [TAGS[4]]
            : i === 5 ? [TAGS[5]]
              : i === 6 ? [TAGS[6]]
                : i === 7 ? [TAGS[7]]
                  : i === 8 ? [TAGS[8]]
                    : i === 9 ? [TAGS[9], TAGS[10]]
                      : []
    const cardRes = await api(page, 'POST', '/api/v1/review/cards', {
      group_id: groupID,
      question: `[${RUN_ID}] 真实验收题 ${i}：第 ${i} 个用于验证真实热力图与 AI 出口的知识点。`,
      answer: `第 ${i} 题答案：真实验收答案 ${i}。`,
      explanation: `真实验收解析 ${i}，用于验证真实后端聚合、拆分与新角度。`,
      tags,
      sources: [{
        source_type: 'knowledge_chunk',
        knowledge_id: knowledgeID,
        chunk_id: chunkID,
        excerpt: `第 ${i} 题的真实来源摘录。`,
        is_primary: true,
      }],
    })
    if (cardRes.status !== 201) throw new Error(`create card ${i} failed: ${cardRes.status} ${JSON.stringify(cardRes.data)}`)
    groups.push({ id: groupID, name: `${SCOPE_NAME}-G${i}` })
    cards.push({ id: cardRes.data.data.id, groupID, tags })
  }

  // Two consecutive forgets make a severe card; both share the first tag so
  // the seeded cluster has a real severe cell.
  for (const index of [0, 1]) {
    for (let round = 0; round < 2; round += 1) {
      const res = await api(page, 'POST', '/api/v1/review/reviews', {
        card_id: cards[index].id,
        rating: 'forgot',
      })
      if (res.status !== 200) throw new Error(`rate ${index + 1}/${round + 1} failed: ${res.status} ${JSON.stringify(res.data)}`)
    }
  }
  record('seed', `scope=${SCOPE_NAME} groups=${groups.length} cards=${cards.length} severe=2`)
  return { scopeID, groups, cards }
}

async function verifyBackend(page, seeded) {
  const scoped = await api(page, 'GET', `/api/v1/review/heatmap?group_id=${seeded.scopeID}`)
  if (scoped.status !== 200) throw new Error(`scoped heatmap failed: ${scoped.status}`)
  const data = scoped.data.data
  if (!data.row_truncated || !data.column_truncated) {
    throw new Error(`expected truncation, got row=${data.row_truncated} column=${data.column_truncated}`)
  }
  if (!data.rows.some((row) => row.kind === 'other')) throw new Error('other row missing')
  if (!data.columns.some((col) => col.key === 'other_column')) throw new Error('other column missing')
  const severeCell = data.cells.find((cell) => cell.row_id === data.rows[0].id && cell.level === 'severe')
  if (!severeCell || !severeCell.clickable) throw new Error('severe clickable cell missing')
  record('api-heatmap', `rows=${data.rows.length} columns=${data.columns.length} clickable severe cell present`)

  const expanded = await api(page, 'GET', `/api/v1/review/heatmap?group_id=${seeded.scopeID}&row_limit=50&column_limit=30`)
  if (expanded.status !== 200) throw new Error(`expanded heatmap failed: ${expanded.status}`)
  const full = expanded.data.data
  if (full.row_truncated || full.column_truncated) throw new Error('expand still truncated')
  if (full.rows.length !== 10 || full.columns.length !== 12) {
    throw new Error(`expanded shape rows=${full.rows.length} columns=${full.columns.length}`)
  }
  if (!full.columns.some((col) => col.kind === 'knowledge_base')) throw new Error('knowledge-base fallback column missing')
  record('api-expand', `rows=${full.rows.length} columns=${full.columns.length} KB fallback visible`)

  const tagColumn = full.columns.find((col) => col.kind === 'tag' && col.title === TAGS[0])
  if (!tagColumn) throw new Error('first tag column missing')
  const rowGroup = full.rows.find((row) => row.group_id === seeded.cards[0].groupID)
  if (!rowGroup) throw new Error('first seeded row missing')
  const cell = full.cells.find((item) => item.row_id === rowGroup.id && item.column_key === tagColumn.key)
  if (!cell || cell.level !== 'severe' || !cell.clickable) throw new Error('seeded severe cell not clickable')
  record('api-cell', `severe cell ${TAGS[0]} intensity=${cell.intensity}`)

  const detailRes = await api(
    page,
    'GET',
    `/api/v1/review/clusters/detail?scope_group_id=${seeded.scopeID}&row_group_id=${rowGroup.group_id}&column_key=${encodeURIComponent(tagColumn.key)}`,
  )
  if (detailRes.status !== 200) throw new Error(`cluster detail failed: ${detailRes.status}`)
  const detail = detailRes.data.data
  if (detail.cell.level !== 'severe') throw new Error(`cluster level=${detail.cell.level}`)
  if (!detail.cards.some((item) => item.card.id === seeded.cards[0].id)) throw new Error('seeded card missing from cluster')
  if (!detail.reasons.length) throw new Error('cluster reasons empty')
  record('api-cluster', `level=${detail.cell.level} cards=${detail.cards.length} reasons=${detail.reasons.length}`)

  let root = null
  let rootAttempts = 0
  const rootStarted = Date.now()
  for (; rootAttempts < 3; rootAttempts += 1) {
    const rootRes = await api(page, 'POST', '/api/v1/review/clusters/root-cause', {
      scope_group_id: seeded.scopeID,
      row_group_id: rowGroup.group_id,
      column_key: tagColumn.key,
    })
    if (rootRes.status !== 200) throw new Error(`root cause failed: ${rootRes.status}`)
    root = rootRes.data.data
    if (root.source === 'llm' && root.summary) break
    await sleep(3000)
  }
  if (!root || root.source !== 'llm' || !root.summary) {
    throw new Error(`expected real LLM root cause after ${rootAttempts} attempts, last source=${root?.source}`)
  }
  record('api-root-cause', `source=llm model=${root.model_id} attempts=${rootAttempts + 1} ms=${Date.now() - rootStarted}`)

  const sessionRes = await api(page, 'POST', '/api/v1/review/sessions', {
    mode: 'manual',
    group_id: rowGroup.group_id,
    card_ids: [seeded.cards[0].id],
    force_new: true,
  })
  if (![200, 201].includes(sessionRes.status)) throw new Error(`relearn session failed: ${sessionRes.status} ${JSON.stringify(sessionRes.data)}`)
  const state = sessionRes.data.data
  if (!state?.session?.id || state.cards.length !== 1) throw new Error('relearn session shape unexpected')
  await api(page, 'POST', `/api/v1/review/sessions/${state.session.id}/abandon`)
  record('api-relearn', `session=${state.session.id} cards=${state.cards.length} abandoned`)
}

async function verifyBrowser(page, browser, seeded) {
  const consoleErrors = []
  const pageErrors = []
  page.on('console', (m) => { if (m.type() === 'error') consoleErrors.push(m.text()) })
  page.on('pageerror', (e) => pageErrors.push(String(e)))

  await page.goto(BASE + '/platform/review', { waitUntil: 'networkidle2' })
  await waitForHeatmap(page)

  const rootDom = await readHeatmapDom(page)
  if (!rootDom.rows.some((row) => row.includes(SCOPE_NAME))) throw new Error('seeded scope missing from root heatmap')
  if (!rootDom.cols.some((col) => col.includes('测试库'))) throw new Error('KB fallback column missing from root heatmap')
  if (rootDom.expand) {
    await page.click('[data-testid="review-v3-heatmap-expand"]')
    await page.waitForFunction(
      () => !document.querySelector('[data-testid="review-v3-heatmap-expand"]')
        && document.querySelectorAll('.heatmap__cell').length > 0,
      { timeout: 15000 },
    )
  }
  record('ui-root', 'seeded scope and KB fallback visible')

  await page.waitForFunction((name) => {
    return [...document.querySelectorAll('.heatmap__rowhead')].some((el) => el.innerText.includes(name))
  }, { timeout: 15000 }, SCOPE_NAME)
  const drillState = await page.evaluate((name) => {
    const rowhead = [...document.querySelectorAll('.heatmap__rowhead')].find((el) => el.innerText.includes(name))
    if (!rowhead) return { found: false, html: document.querySelector('.heatmap__grid')?.innerHTML?.slice(0, 800) || '' }
    const tag = rowhead.tagName
    const drillable = tag === 'BUTTON'
    if (drillable) rowhead.click()
    return { found: true, tag, drillable }
  }, SCOPE_NAME)
  if (!drillState.found) throw new Error(`seeded scope row missing: ${drillState.html}`)
  if (!drillState.drillable) throw new Error(`seeded scope row rendered as ${drillState.tag}, not drillable`)
  await page.waitForFunction((id) => location.search.includes(`scope=${id}`), { timeout: 15000 }, seeded.scopeID)
  await waitForHeatmap(page)
  try {
    await page.waitForFunction(
      () => [...document.querySelectorAll('.heatmap__rowhead')].some((el) => el.innerText.includes('-G1')),
      { timeout: 15000 },
    )
  } catch (err) {
    const debug = await page.evaluate(() => ({
      url: location.href,
      rows: [...document.querySelectorAll('.heatmap__rowhead')].map((el) => el.innerText.trim()),
      error: document.querySelector('.heatmap__error')?.innerText || '',
      empty: document.querySelector('[data-testid="review-v3-heatmap-empty"]')?.innerText || '',
      scope: document.querySelector('.heatmap__scope')?.innerText || '',
    }))
    await page.screenshot({ path: path.join(OUT, 'review_v3_real_drill_fail.png'), fullPage: true })
    throw new Error(`scoped heatmap rows unexpected: ${JSON.stringify(debug)}`)
  }

  const scopedDom = await readHeatmapDom(page)
  if (!scopedDom.otherRow || !scopedDom.otherCol) throw new Error('other buckets missing after drill')
  if (!scopedDom.expand) throw new Error('expand button missing after drill')
  record('ui-drill', `rows=${scopedDom.rows.length} columns=${scopedDom.cols.length} other buckets visible`)

  await page.click('[data-testid="review-v3-heatmap-expand"]')
  await page.waitForFunction(() => {
    const rows = document.querySelectorAll('.heatmap__rowhead').length
    const cols = document.querySelectorAll('.heatmap__colhead').length
    return rows === 10 && cols === 12
  }, { timeout: 15000 })
  const expandedDom = await readHeatmapDom(page)
  if (expandedDom.otherRow || expandedDom.otherCol) throw new Error('other buckets still visible after expand')
  record('ui-expand', `rows=${expandedDom.rows.length} columns=${expandedDom.cols.length}`)

  const t1Title = expandedDom.cols.find((title) => title === TAGS[0])
  if (!t1Title) throw new Error('first tag column missing in UI')
  const clickable = await clickHeatmapCell(page, '-G1', TAGS[0])
  if (!clickable) throw new Error('seeded severe cell not clickable in UI')
  await page.waitForSelector('[data-testid="review-v3-cluster-page"]', { timeout: 15000 })
  const clusterUrl = page.url()
  await page.waitForFunction(() => document.querySelector('.cluster-formula')?.innerText.includes('%'), { timeout: 15000 })
  record('ui-cluster', `route=${new URL(clusterUrl).pathname}`)

  const actions = await page.evaluate(() => ({
    relearn: !document.querySelector('[data-testid="review-v3-cluster-relearn"]')?.disabled,
    source: !document.querySelector('[data-testid="review-v3-action-source"]')?.disabled,
    split: !document.querySelector('[data-testid="review-v3-action-split"]')?.disabled,
    angles: !document.querySelector('[data-testid="review-v3-action-angles"]')?.disabled,
    formula: document.querySelector('.cluster-formula')?.innerText || '',
  }))
  if (!actions.relearn || !actions.source || !actions.split || !actions.angles) {
    throw new Error(`cluster exits disabled: ${JSON.stringify(actions)}`)
  }
  record('ui-exits', 'relearn/source/split/angles enabled')

  await page.screenshot({ path: path.join(OUT, 'review_v3_real_cluster.png'), fullPage: true })

  let uiRootCause = null
  let uiAttempts = 0
  const aiStarted = Date.now()
  for (; uiAttempts < 3; uiAttempts += 1) {
    const rootCauseResponse = page.waitForResponse((res) => res.url().includes('/api/v1/review/clusters/root-cause'), { timeout: 60000 })
    await page.click('[data-testid="review-v3-cluster-ai"]')
    const aiRes = await rootCauseResponse
    const aiJSON = await aiRes.json()
    uiRootCause = aiJSON?.data || null
    if (uiRootCause?.source === 'llm') break
    await page.waitForFunction(
      () => document.querySelector('[data-testid="review-v3-cluster-ai"]')?.innerText.includes('AI 深入分析'),
      { timeout: 20000 },
    )
    await sleep(3000)
  }
  if (uiRootCause?.source !== 'llm') throw new Error(`UI root cause source=${uiRootCause?.source} after ${uiAttempts} attempts`)
  await page.waitForFunction(() => document.body.innerText.includes('AI 分析'), { timeout: 15000 })
  record('ui-root-cause', `source=llm attempts=${uiAttempts + 1} ms=${Date.now() - aiStarted}`)

  const newTarget = new Promise((resolve) => browser.once('targetcreated', (target) => resolve(target)))
  await page.click('[data-testid="review-v3-action-source"]')
  const target = await newTarget
  const sourcePage = await target.page()
  await sourcePage.waitForFunction(() => location.pathname.includes('/platform/knowledge-bases/'), { timeout: 20000 })
  const sourceUrl = sourcePage.url()
  if (!sourceUrl.includes('knowledge_id=') || !sourceUrl.includes('chunk_id=')) {
    throw new Error(`source tab missing query: ${sourceUrl}`)
  }
  record('ui-source', 'source opens real knowledge chunk tab')
  await sourcePage.close()

  const conflictText = '放弃并开始补强'
  await page.click('[data-testid="review-v3-cluster-relearn"]')
  await sleep(1500)
  if (await page.evaluate((text) => document.body.innerText.includes(text), conflictText)) {
    await clickText(page, 'button', conflictText)
  }
  await page.waitForSelector('[data-testid="review-v3-mode"]', { timeout: 20000 })
  await page.waitForSelector('[data-testid="review-v3-question"]', { timeout: 20000 })
  const active = await api(page, 'GET', '/api/v1/review/sessions/active')
  const activeCards = active.data?.data?.cards?.length || 0
  if (activeCards !== 1) throw new Error(`relearn session cards=${activeCards}`)
  await page.click('[data-testid="review-v3-suspend"]')
  await page.waitForSelector('[data-testid="review-v3-task-bar"]', { timeout: 20000 })
  if (active.data?.data?.session?.id) {
    await api(page, 'POST', `/api/v1/review/sessions/${active.data.data.session.id}/abandon`)
  }
  record('ui-relearn', 'relearn starts a 1-card session and suspends cleanly')

  await page.goto(clusterUrl, { waitUntil: 'networkidle2' })
  await page.waitForSelector('[data-testid="review-v3-cluster-page"]', { timeout: 20000 })

  await page.waitForSelector('[data-testid="review-v3-action-split"]', { timeout: 15000 })
  await page.click('[data-testid="review-v3-action-split"]')
  await page.waitForSelector('[data-testid="review-v3-ai-dialog"] .ai-dialog__item', { timeout: 60000 })
  const splitItems = await page.$$eval('[data-testid="review-v3-ai-dialog"] .ai-dialog__item', (els) => els.length)
  if (!splitItems) throw new Error('split dialog has no candidates')
  await clickText(page, '[data-testid="review-v3-ai-dialog"] .ai-dialog__footer button', '创建拆分卡')
  try {
    await page.waitForFunction(() => {
      const dialog = document.querySelector('[data-testid="review-v3-ai-dialog"]')
      return !dialog || dialog.getBoundingClientRect().height === 0
    }, { timeout: 30000 })
  } catch (err) {
    const debug = await page.evaluate(() => ({
      alert: document.querySelector('[data-testid="review-v3-ai-dialog"] .ai-dialog__error')?.innerText || '',
      buttons: [...document.querySelectorAll('[data-testid="review-v3-ai-dialog"] .ai-dialog__footer button')]
        .map((el) => ({ text: el.innerText.trim(), disabled: el.disabled })),
      selected: document.querySelectorAll('[data-testid="review-v3-ai-dialog"] .t-checkbox.t-is-checked').length,
      items: document.querySelectorAll('[data-testid="review-v3-ai-dialog"] .ai-dialog__item').length,
    }))
    const status = await api(page, 'GET', `/api/v1/review/cards/${seeded.cards[0].id}`)
    debug.cardStatus = status.data?.data?.card?.status
    await page.screenshot({ path: path.join(OUT, 'review_v3_real_split_fail.png'), fullPage: true })
    throw new Error(`split dialog did not close: ${JSON.stringify(debug)}`)
  }
  await page.waitForFunction(() => !document.querySelector('.cluster-table__question-text')?.innerText.includes('真实验收题 1'), { timeout: 20000 })
  const splitCheck = await api(page, 'GET', `/api/v1/review/cards/${seeded.cards[0].id}`)
  if (splitCheck.data.data.card.status !== 'suspended') throw new Error(`UI split original status=${splitCheck.data.data.card.status}`)
  record('ui-split', `candidates=${splitItems} original suspended`)

  await page.waitForSelector('[data-testid="review-v3-action-angles"]', { timeout: 15000 })
  await page.click('[data-testid="review-v3-action-angles"]')
  await page.waitForSelector('[data-testid="review-v3-ai-dialog"] .ai-dialog__item', { timeout: 60000 })
  await clickText(page, '[data-testid="review-v3-ai-dialog"] .ai-dialog__footer button', '导入新角度卡')
  await page.waitForFunction(() => {
    const dialog = document.querySelector('[data-testid="review-v3-ai-dialog"]')
    return !dialog || dialog.getBoundingClientRect().height === 0
  }, { timeout: 30000 })
  await sleep(800)
  const angleCheck = await api(page, 'GET', `/api/v1/review/cards/${seeded.cards[1].id}`)
  if (angleCheck.data.data.card.status !== 'active') throw new Error(`UI angles original status=${angleCheck.data.data.card.status}`)
  record('ui-angles', 'new angle imported, original stays active')

  await page.screenshot({ path: path.join(OUT, 'review_v3_real_cluster_after.png'), fullPage: true })

  await page.goto(`${BASE}/platform/review?scope=${seeded.scopeID}`, { waitUntil: 'networkidle2' })
  await waitForHeatmap(page)
  await page.setViewport({ width: 600, height: 900 })
  await sleep(500)
  const overflow = await page.evaluate(() => ({
    scroll: document.documentElement.scrollWidth,
    inner: window.innerWidth,
  }))
  if (overflow.scroll > overflow.inner + 1) throw new Error(`600px horizontal overflow: ${JSON.stringify(overflow)}`)
  await page.screenshot({ path: path.join(OUT, 'review_v3_real_600.png'), fullPage: true })
  record('ui-narrow', '600px no page-level horizontal overflow')

  if (consoleErrors.length || pageErrors.length) {
    throw new Error(`console errors: ${JSON.stringify(consoleErrors.concat(pageErrors).slice(0, 5))}`)
  }
  record('ui-console', '0 console/page errors')
}

function cleanup(seeded) {
  if (!seeded) return
  const groupIDs = [seeded.scopeID, ...seeded.groups.map((group) => group.id)]
  const quoted = groupIDs.map((id) => `'${id}'`).join(',')
  psql(`delete from review_logs where card_id in (select id from review_cards where group_id in (${quoted})) or session_id in (select id from review_sessions where group_id in (${quoted}))`)
  psql(`delete from review_sessions where group_id in (${quoted})`)
  psql(`delete from review_card_sources where card_id in (select id from review_cards where group_id in (${quoted}))`)
  psql(`delete from review_cards where group_id in (${quoted})`)
  psql(`delete from review_groups where id in (${quoted})`)
  const left = psql(`select count(*) from review_groups where id in (${quoted})`)
  if (left !== '0') throw new Error(`cleanup left ${left} groups`)
  record('cleanup', 'seeded groups, cards, sources, logs and sessions removed')
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
  let seeded = null
  try {
    await login(page)
    record('login', 'owner logged in')
    seeded = await seed(page)
    await verifyBackend(page, seeded)
    await verifyBrowser(page, browser, seeded)
  } finally {
    try {
      cleanup(seeded)
    } catch (err) {
      console.error('[cleanup.FAIL]', err && err.message ? err.message : err)
      process.exitCode = 1
    }
    fs.writeFileSync(path.join(OUT, 'review_v3_real_acceptance.json'), JSON.stringify(log, null, 2))
    await browser.close()
  }
}

main().catch((err) => {
  console.error('[FAIL]', err && err.stack ? err.stack : err)
  process.exit(1)
})
