/**
 * Acceptance for the V3 heatmap v2 + standalone cluster route.
 *
 * Mocks heatmap/cluster/root-cause APIs. Session and rating APIs stay real.
 *
 * Usage:
 *   NODE_PATH=frontend/node_modules WELLREAD_WEB=http://localhost:5174 \
 *     node scripts/acceptance/browser/review_v3_heatmap_acceptance.js
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

const reviewCard = (id, question, level, patch = {}) => ({
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
  vulnerability_reason: level === 'severe' ? '连续忘记 3 次' : level === 'warning' ? '最近一次复习忘记' : '',
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-10T00:00:00Z',
  ...patch,
})

const rows = [
  { id: 'row-1', group_id: 'g1', name: '考研数学', path: '考研 / 数学', kind: 'group', drillable: true, virtual: false, total_cards: 6 },
  { id: 'row-2', group_id: 'g2', name: '高数错题', path: '考研 / 数学 / 高数错题', kind: 'group', drillable: false, virtual: false, total_cards: 5 },
]
const columns = [
  { key: 'tag:极限', title: '极限', kind: 'tag', source_id: '', knowledge_base_id: '' },
  { key: 'tag:矩阵', title: '矩阵', kind: 'tag', source_id: '', knowledge_base_id: '' },
  { key: 'knowledge_base:kb-1', title: '高数知识库', kind: 'knowledge_base', source_id: '', knowledge_base_id: 'kb-1' },
  { key: 'uncategorized', title: '未分类', kind: 'uncategorized', source_id: '', knowledge_base_id: '' },
]
const cell = (rowId, columnKey, level, intensity, counts, clickable = true) => ({
  row_id: rowId,
  column_key: columnKey,
  level,
  intensity,
  total: counts.total,
  severe: counts.severe || 0,
  warning: counts.warning || 0,
  observe: counts.observe || 0,
  healthy: counts.healthy || 0,
  no_data: counts.no_data || 0,
  clickable,
})
const cells = [
  cell('row-1', 'tag:极限', 'severe', 0.72, { total: 5, severe: 3, warning: 1, healthy: 1 }),
  cell('row-1', 'tag:矩阵', 'warning', 0.45, { total: 2, warning: 1, healthy: 1 }),
  cell('row-1', 'knowledge_base:kb-1', 'healthy', 0.1, { total: 1, healthy: 1 }, false),
  cell('row-1', 'uncategorized', 'none', 0, { total: 1, no_data: 1 }, false),
  cell('row-2', 'tag:极限', 'severe', 0.8, { total: 4, severe: 3, warning: 1 }),
  cell('row-2', 'tag:矩阵', 'none', 0, { total: 2, no_data: 2 }, false),
]
const heatmapFixture = {
  rows,
  columns,
  cells,
  priorities: [cells[4], cells[0], cells[1]],
  observe_count: 0,
  insight: { kind: 'row', row_id: 'row-1', column_key: '', message: '「考研 / 数学」多个子主题同时告急' },
  scope_group_id: '',
  scope_path: '全部卡组',
  row_total: 12,
  column_total: 7,
  row_truncated: true,
  column_truncated: true,
  formula: {
    severe_weight: 1,
    warning_weight: 0.5,
    observe_weight: 0.25,
    denominator: 0,
    text: '漏洞强度 =（严重×1 + 警告×0.5 + 观察×0.25）÷ 卡数',
  },
}

const queueFixture = {
  due: [],
  new: [],
  completed_today: 0,
  limit: 100,
  new_limit: 20,
  streak_days: 4,
}

const clusterCards = [
  {
    card: reviewCard('card-1', '求极限 lim(x→0) (sin x)/x 的推导', 'severe', {
      stability: 2,
      forget_streak: 3,
      lapses: 3,
    }),
    level: 'severe',
    last_rating: 'forgot',
    suggested_action: 'relearn',
    source: { id: 'src-1', knowledge_id: 'k-1', chunk_id: 'chunk-1', excerpt: '极限推导原文', status: 'active', is_primary: true },
    knowledge_base_id: 'kb-1',
  },
  {
    card: reviewCard('card-2', '等价无穷小的常见替换', 'warning', { stability: 8, lapses: 1 }),
    level: 'warning',
    last_rating: 'fuzzy',
    suggested_action: 'relearn',
    source: null,
    knowledge_base_id: '',
  },
  {
    card: reviewCard('card-3', '夹逼定理的适用条件', 'observe', { stability: 12, lapses: 0 }),
    level: 'observe',
    last_rating: 'remembered',
    suggested_action: 'observe',
    source: { id: 'src-3', knowledge_id: 'k-2', chunk_id: 'chunk-2', excerpt: '夹逼定理原文', status: 'active', is_primary: true },
    knowledge_base_id: 'kb-2',
  },
  {
    card: reviewCard('card-4', '矩阵秩的定义', 'healthy', { stability: 30, lapses: 0 }),
    level: 'healthy',
    last_rating: 'easy',
    suggested_action: '',
    source: null,
    knowledge_base_id: '',
  },
]
const clusterFixture = {
  cell: cells[0],
  row: rows[0],
  column: columns[0],
  cards: clusterCards,
  reasons: ['连续忘记 3 次', '记忆可撑不足 3 天'],
  formula: {
    severe_weight: 1,
    warning_weight: 0.5,
    observe_weight: 0.25,
    denominator: 5,
    text: '漏洞强度 =（严重×1 + 警告×0.5 + 观察×0.25）÷ 卡数',
  },
  scope_group_id: 'g1',
}

let heatmapFixtureState = heatmapFixture
let heatmapStatus = 200
const heatmapRequests = []

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
    clickText(page, 'button', '登录', { exact: true }),
  ])
}

async function gotoV3(page) {
  await page.goto(BASE + '/platform/review', { waitUntil: 'networkidle2' })
  await page.waitForSelector('[data-testid="review-v3-task-bar"]', { timeout: 20000 })
  await page.waitForSelector('[data-testid="review-v3-heatmap"] .heatmap__cell', { timeout: 20000 })
}

async function overflow(page) {
  return page.evaluate(() => ({
    scroll: document.documentElement.scrollWidth,
    inner: window.innerWidth,
  }))
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
    if (url.pathname === '/api/v1/review/heatmap') {
      heatmapRequests.push(url.search)
      if (heatmapStatus !== 200) {
        req.respond({ status: heatmapStatus, contentType: 'application/json', body: JSON.stringify({ success: false, message: '模拟热力图失败' }) })
        return
      }
      req.respond({ status: 200, contentType: 'application/json', body: JSON.stringify({ success: true, data: heatmapFixtureState }) })
      return
    }
    if (url.pathname === '/api/v1/review/queue') {
      req.respond({ status: 200, contentType: 'application/json', body: JSON.stringify({ success: true, data: queueFixture }) })
      return
    }
    if (url.pathname === '/api/v1/review/clusters/detail') {
      req.respond({ status: 200, contentType: 'application/json', body: JSON.stringify({ success: true, data: clusterFixture }) })
      return
    }
    if (url.pathname === '/api/v1/review/clusters/root-cause') {
      req.respond({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            summary: 'AI 判断：把极限定义和等价替换混在一起。',
            reasons: ['极限定义不牢'],
            focus_suggestions: ['先回原文看定义', '再做 3 道替换题'],
            evidence_card_ids: ['card-1'],
            model_id: 'mock-model',
            source: 'llm',
          },
        }),
      })
      return
    }
    req.continue()
  })

  try {
    await login(page)
    record('login', 'owner logged in')
    await gotoV3(page)
    record('route', 'review main route rendered')

    const layout = await page.evaluate(() => ({
      cells: document.querySelectorAll('[data-testid="review-v3-heatmap"] .heatmap__cell').length,
      emphasized: document.querySelectorAll('[data-testid="review-v3-heatmap"] .heatmap__cell.is-emphasis').length,
      kbColumn: [...document.querySelectorAll('.heatmap__colhead')].some(el => el.innerText.includes('高数知识库')),
    }))
    if (layout.cells !== rows.length * columns.length) throw new Error(`expected ${rows.length * columns.length} cells, got ${layout.cells}`)
    if (layout.emphasized !== 3) throw new Error(`expected 3 emphasized cells, got ${layout.emphasized}`)
    if (!layout.kbColumn) throw new Error('knowledge-base fallback column missing')
    record('heatmap', `${layout.cells} cells, 3 emphasized, KB fallback column visible`)

    await page.evaluate(() => {
      document.querySelector('[data-testid="review-v3-heatmap-row"]')?.click()
    })
    await page.waitForFunction(() => location.search.includes('scope=g1'), { timeout: 10000 })
    await waitNode(() => heatmapRequests.some(q => q.includes('group_id=g1')), 10000, 'drill heatmap request')
    record('drill', 'row drill updates scope and reloads heatmap')

    const expand = await page.$('[data-testid="review-v3-heatmap-expand"]')
    if (!expand) throw new Error('expand button missing')
    await expand.click()
    await waitNode(() => heatmapRequests.some(q => q.includes('expand=1')), 10000, 'expand heatmap request')
    record('expand', 'expand requests uncapped heatmap')

    await page.evaluate(() => {
      const cells = [...document.querySelectorAll('[data-testid="review-v3-heatmap"] .heatmap__cell.is-clickable')]
      cells[0]?.click()
    })
    await page.waitForFunction(() => location.pathname.includes('/review/cluster/g1'), { timeout: 10000 })
    await page.waitForSelector('[data-testid="review-v3-cluster-page"]', { timeout: 15000 })
    await waitText(page, '求极限 lim(x→0) (sin x)/x 的推导')
    await waitText(page, '连续忘记 3 次')
    const cluster = await page.evaluate(() => ({
      rows: document.querySelectorAll('.cluster-table tbody tr').length,
      actions: document.querySelectorAll('[data-testid="review-v3-cluster-actions"] .cluster-action').length,
      relearn: Boolean(document.querySelector('[data-testid="review-v3-cluster-relearn"]')),
      formula: document.body.innerText.includes('72%'),
    }))
    if (cluster.rows !== 4) throw new Error(`expected 4 cluster cards, got ${cluster.rows}`)
    if (cluster.actions !== 4) throw new Error(`expected 4 action exits, got ${cluster.actions}`)
    if (!cluster.relearn || !cluster.formula) throw new Error('cluster header/relearn missing')
    record('cluster-route', 'standalone cluster page shows table, reasons, formula and 4 exits')
    await page.screenshot({ path: path.join(OUT, 'review_v3_cluster_page_1440.png'), fullPage: true })

    await page.click('[data-testid="review-v3-cluster-ai"]')
    await waitText(page, 'AI 判断：把极限定义和等价替换混在一起')
    record('root-cause-llm', 'AI root cause replaces the rule summary')

    await page.click('[data-testid="review-v3-cluster-back"]')
    await page.waitForSelector('[data-testid="review-v3-heatmap"]', { timeout: 15000 })
    await page.waitForFunction(() => location.pathname.includes('/platform/review') && location.search.includes('scope=g1'), { timeout: 10000 })
    record('cluster-back', 'back returns to heatmap with scope preserved')

    await page.setViewport({ width: 600, height: 900 })
    await page.goto(BASE + '/platform/review/cluster/g1?col=' + encodeURIComponent('tag:极限') + '&scope=g1', { waitUntil: 'networkidle2' })
    await page.waitForSelector('[data-testid="review-v3-cluster-page"]', { timeout: 20000 })
    const responsive = await overflow(page)
    if (responsive.scroll > responsive.inner + 1) throw new Error(`600px cluster overflow: ${responsive.scroll} > ${responsive.inner}`)
    await page.screenshot({ path: path.join(OUT, 'review_v3_cluster_page_600.png'), fullPage: true })
    record('cluster-600', 'cluster page has no horizontal overflow at 600px')

    if (consoleErrors.length || pageErrors.length) {
      throw new Error(`console errors: ${JSON.stringify([...consoleErrors, ...pageErrors].slice(0, 5))}`)
    }
    record('console', '0 console/page errors')
    fs.writeFileSync(path.join(OUT, 'review_v3_heatmap_log.json'), JSON.stringify({ log, heatmapRequests, consoleErrors, pageErrors }, null, 2))
    console.log(`DONE screenshots in ${OUT}`)
  } catch (err) {
    fs.writeFileSync(path.join(OUT, 'review_v3_heatmap_log.json'), JSON.stringify({ log, heatmapRequests, consoleErrors, pageErrors, error: String(err) }, null, 2))
    console.error('FAILED:', err)
    process.exitCode = 1
  } finally {
    await browser.close()
  }
}

main()
