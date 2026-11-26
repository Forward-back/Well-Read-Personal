/**
 * Browser acceptance for the reading agent P0 UI surface.
 *
 * Covers: structure skeleton on the plan card, quiet-mode tag, self-test
 * preview, group selection, import idempotency, and cleanup.
 *
 * Seeds a temporary run copied from an existing finished v3 run, then drives
 * the real frontend/backend. No LLM calls and no writes to owner review data.
 *
 * Usage:
 *   NODE_PATH=frontend/node_modules WELLREAD_WEB=http://localhost:5173 \
 *     node scripts/acceptance/browser/reading_agent_ui_acceptance.js
 */
const puppeteer = require('puppeteer-core')
const { execSync } = require('child_process')
const crypto = require('crypto')
const fs = require('fs')
const os = require('os')
const path = require('path')

const ROOT = process.env.WELLREAD_ROOT || path.resolve(__dirname, '../../..')
const CHROME = process.env.CHROME_PATH || 'D:/chrome-for-test/chrome/win64-157.0.8103.38/chrome-win64/chrome.exe'
const BASE = process.env.WELLREAD_WEB || 'http://localhost:6151'
const OUT = process.env.ACCEPT_OUT || path.join(os.tmpdir(), 'wellread_accept')
fs.mkdirSync(OUT, { recursive: true })

const RUN = `reading-agent-ui-${Date.now().toString(36)}`
const GROUP_NAME = `验收自测卡组-${RUN}`
const SELF_TEST_IDS = [crypto.randomUUID(), crypto.randomUUID()]

const results = []
const consoleErrors = []
const record = (step, detail, data) => {
  results.push({ step, detail, data: data ?? null, at: new Date().toISOString() })
  console.log(`[${step}] ${detail}${data !== undefined ? ' ' + JSON.stringify(data) : ''}`)
}
const fail = (step, detail) => {
  record(step, `FAIL ${detail}`)
  throw new Error(`${step}: ${detail}`)
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

function sql(query) {
  return execSync(
    `docker exec WellRead-Postgres psql -U root -d WellRead -tAc ${JSON.stringify(query.replace(/\s+/g, ' ').trim())}`,
    { encoding: 'utf8' },
  ).trim()
}

async function main() {
  const browser = await puppeteer.launch({
    executablePath: CHROME,
    headless: 'new',
    args: ['--no-sandbox', '--disable-dev-shm-usage', '--window-size=1440,960'],
    defaultViewport: { width: 1440, height: 960 },
  })
  const page = await browser.newPage()
  page.setDefaultTimeout(30000)
  page.on('console', (msg) => {
    if (msg.type() === 'error') consoleErrors.push(`console: ${msg.text()}`)
  })
  page.on('pageerror', (err) => consoleErrors.push(`pageerror: ${err.message}`))

  const visible = async (selector) =>
    (await page.$(selector)) !== null &&
    (await page.$eval(selector, (el) => !!(el.offsetWidth || el.offsetHeight)))

  const waitFor = async (fn, timeout, label) => {
    const started = Date.now()
    while (Date.now() - started < timeout) {
      if (await fn()) return true
      await new Promise((resolve) => setTimeout(resolve, 250))
    }
    fail('wait', `${label} timed out after ${timeout}ms`)
  }

  const waitText = (text, timeout = 20000) =>
    waitFor(async () => (await page.evaluate(() => document.body.innerText)).includes(text), timeout, `text ${text}`)

  const waitVisible = (selector, timeout = 20000) => waitFor(() => visible(selector), timeout, selector)

  const setInput = async (selector, value) => {
    const handle = await page.$(selector)
    if (!handle) fail('input', `missing ${selector}`)
    await handle.click({ clickCount: 3 })
    await page.keyboard.press('Backspace')
    await handle.type(value, { delay: 5 })
  }

  const clickText = async (selector, text, { exact = false } = {}) => {
    const ok = await page.evaluate(
      ({ selector, text, exact }) => {
        const els = [...document.querySelectorAll(selector)]
          .filter((e) => e.offsetWidth || e.offsetHeight)
        const el = els.find((e) => {
          const t = (e.innerText || e.value || '').trim()
          return exact ? t === text : t.includes(text)
        })
        if (!el) return false
        el.click()
        return true
      },
      { selector, text, exact },
    )
    if (!ok) fail('click', `not found ${selector} "${text}"`)
  }

  const api = (p, init) =>
    page.evaluate(
      async (url, options) => {
        const token = localStorage.getItem('gaze_token')
        const headers = { ...(options.headers || {}) }
        if (token) headers.Authorization = `Bearer ${token}`
        const response = await fetch(url, { credentials: 'include', ...options, headers })
        const body = await response.json().catch(() => null)
        return { status: response.status, body }
      },
      p,
      init || {},
    )

  const login = async () => {
    await page.goto(BASE + '/', { waitUntil: 'networkidle2' })
    await page.waitForSelector('input[type="password"]', { timeout: 30000 })
    await setInput('input[type="text"]', env.OWNER_EMAIL)
    await setInput('input[type="password"]', env.OWNER_PASSWORD)
    await Promise.all([
      page.waitForFunction(() => location.pathname === '/', { timeout: 30000 }),
      clickText('button', '登录', { exact: true }),
    ])
    record('login', 'owner signed in')
  }

  const runID = crypto.randomUUID()
  let groupID = ''
  let seeded = false

  const seedRun = () => {
    const source = sql(
      "select id from reading_runs where status='finished' and plan->>'state_version'='3' " +
        "and jsonb_array_length(coalesce(plan->'structure','[]'::jsonb)) > 0 " +
        'order by created_at desc limit 1',
    )
    if (!source) fail('seed', 'no finished v3 run with structure to copy')
    sql(
      `insert into reading_runs (
         id, tenant_id, session_id, user_id, message_id, document_ids, skill_id,
         reading_model_id, extraction_model_id, synthesis_model_id, phase, status,
         plan, notes, final, current_section_id, step_count, token_usage
       )
       select '${runID}', tenant_id, session_id, user_id, message_id, document_ids, skill_id,
         reading_model_id, extraction_model_id, synthesis_model_id, 'planning', 'planning',
         plan, notes, final, '', 0, 0
       from reading_runs where id='${source}'`,
    )
    const sectionID = sql(`select coalesce(plan->'sections'->0->>'id','') from reading_runs where id='${runID}'`)
    if (!sectionID) fail('seed', 'copied plan has no sections')
    seeded = true
    record('seed', 'temporary planning run created', { runId: runID, source, sectionID })
    return sectionID
  }

  const seedSelfTests = (sectionID) => {
    for (const [index, testID] of SELF_TEST_IDS.entries()) {
      const label = String.fromCharCode(65 + index)
      sql(
        `insert into reading_self_tests
           (id, run_id, tenant_id, section_id, question, answer, explanation, evidence, status)
         select '${testID}', '${runID}', tenant_id, '${sectionID}',
           '验收自测题 ${label}：该节的核心论点是什么？',
           '验收自测答案 ${label}：核心论点已由原文证据支撑。',
           '验收自测解析 ${label}。',
           coalesce(
             (select evidence from reading_self_tests where status='candidate' order by created_at desc limit 1),
             '[]'::jsonb
           ),
           'candidate'
         from reading_runs where id='${runID}'`,
      )
    }
    record('selftest', 'seeded candidate self tests', { count: SELF_TEST_IDS.length })
  }

  const cleanup = async () => {
    if (groupID) {
      await api(`/api/v1/review/groups/${groupID}?delete_cards=true`, { method: 'DELETE' }).catch(() => {})
      sql(
        `delete from review_logs where card_id in (select id from review_cards where group_id='${groupID}')`,
      )
      sql(
        `delete from review_card_sources where card_id in (select id from review_cards where group_id='${groupID}')`,
      )
      sql(`delete from review_cards where group_id='${groupID}'`)
      sql(`delete from review_groups where id='${groupID}'`)
    }
    if (seeded) {
      sql(`delete from reading_self_tests where run_id='${runID}'`)
      sql(`delete from reading_run_memories where run_id='${runID}'`)
      sql(`delete from reading_run_steps where run_id='${runID}'`)
      sql(`delete from reading_runs where id='${runID}'`)
    }
    await page.evaluate(() => localStorage.removeItem('gaze_active_reading_run_id')).catch(() => {})
  }

  try {
    await login()
    const sectionID = seedRun()

    // ---- Structure skeleton on the plan card -----------------------------
    await page.goto(`${BASE}/platform/reading?run_id=${runID}`, { waitUntil: 'networkidle2' })
    await waitVisible('.rv-plan-card', 30000)
    await waitText('结构预读', 30000)
    const structure = await page.$$eval('.rv-plan-structure__row', (rows) =>
      rows.map((row) => ({
        section: row.querySelector('.rv-plan-structure__section')?.innerText.trim() || '',
        type: row.querySelector('.rv-plan-structure__type')?.innerText.trim() || '',
        thesis: row.querySelector('p')?.innerText.trim() || '',
      })),
    )
    if (!structure.length) fail('structure', 'no structure rows rendered')
    if (!structure.every((row) => row.type && row.thesis)) {
      fail('structure', `structure rows missing type/thesis: ${JSON.stringify(structure)}`)
    }
    record('structure', 'plan card renders structure skeleton', { rows: structure.length, first: structure[0] })

    // ---- Quiet mode tag ---------------------------------------------------
    sql(
      `update reading_runs set status='reading', phase='reading', current_section_id='${sectionID}' where id='${runID}'`,
    )
    sql(
      `insert into reading_run_memories (run_id, tenant_id, knowledge, user_model, task)
       select '${runID}', tenant_id, '{}'::jsonb, '{"quiet":true}'::jsonb, '{}'::jsonb
       from reading_runs where id='${runID}'
       on conflict (run_id) do update set user_model='{"quiet":true}'::jsonb, updated_at=now()`,
    )
    await page.reload({ waitUntil: 'networkidle2' })
    await waitVisible('.rv-quiet-tag', 30000)
    const quietText = await page.$eval('.rv-quiet-tag', (el) => el.innerText.trim())
    if (!quietText.includes('安静模式')) fail('quiet', `unexpected quiet tag: ${quietText}`)
    if (await page.$('.rv-plan-card')) fail('quiet', 'plan card still visible after reading started')
    record('quiet', 'quiet-mode tag rendered', { text: quietText })

    // ---- Self-test preview and import ------------------------------------
    const groupRes = await api('/api/v1/review/groups', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: GROUP_NAME }),
    })
    if (groupRes.status !== 201) fail('selftest', `create temp group failed: ${groupRes.status}`)
    groupID = groupRes.body?.data?.id
    if (!groupID) fail('selftest', 'temp group id missing')

    sql(`update reading_runs set status='finished', phase='done' where id='${runID}'`)
    seedSelfTests(sectionID)

    await page.reload({ waitUntil: 'networkidle2' })
    await waitVisible('.rv-selftests', 30000)
    await waitFor(async () => (await page.$$('.rv-selftest')).length === SELF_TEST_IDS.length, 30000, 'self-test rows')
    const candidateStatuses = await page.$$eval('.rv-selftest__status', (els) => els.map((el) => el.innerText.trim()))
    if (!candidateStatuses.every((text) => text === '待导入')) {
      fail('selftest', `unexpected initial statuses: ${JSON.stringify(candidateStatuses)}`)
    }
    const checkedCount = await page.$$eval('.rv-selftest input[type="checkbox"]', (els) => els.filter((el) => el.checked).length)
    if (checkedCount !== SELF_TEST_IDS.length) fail('selftest', `expected all candidates checked, got ${checkedCount}`)
    record('selftest', 'preview lists candidates checked by default', { count: checkedCount })

    await waitFor(
      async () =>
        page.evaluate(
          (id) => [...document.querySelectorAll('.rv-selftests__group option')].some((option) => option.value === id),
          groupID,
        ),
      30000,
      'temp review group option',
    )
    await page.select('.rv-selftests__group', groupID)
    await clickText('.rv-selftests__toolbar button', '导入复习')
    await waitFor(
      async () => {
        const statuses = await page.$$eval('.rv-selftest__status', (els) => els.map((el) => el.innerText.trim()))
        return statuses.length === SELF_TEST_IDS.length && statuses.every((text) => text === '已导入')
      },
      60000,
      'self tests imported in UI',
    )
    const disabledCount = await page.$$eval('.rv-selftest input[type="checkbox"]', (els) => els.filter((el) => el.disabled).length)
    if (disabledCount !== SELF_TEST_IDS.length) fail('selftest', 'imported rows are not disabled')
    record('selftest', 'self tests imported into the selected group', { groupId: groupID })

    const cards = sql(`select count(*) from review_cards where group_id='${groupID}' and deleted_at is null`)
    const imported = sql(`select count(*) from reading_self_tests where run_id='${runID}' and status='imported'`)
    if (cards !== String(SELF_TEST_IDS.length)) fail('selftest', `expected ${SELF_TEST_IDS.length} cards, got ${cards}`)
    if (imported !== String(SELF_TEST_IDS.length)) fail('selftest', `expected ${SELF_TEST_IDS.length} imported tests, got ${imported}`)
    record('selftest', 'database reflects imported cards and statuses', { cards, imported })

    // ---- Import idempotency ----------------------------------------------
    const replay = await api(`/api/v1/reading/runs/${runID}/self-tests/import`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ group_id: groupID, self_test_ids: SELF_TEST_IDS }),
    })
    if (replay.status !== 200) fail('selftest', `replay import failed: ${replay.status}`)
    if (replay.body?.data?.imported !== 0 || replay.body?.data?.skipped !== SELF_TEST_IDS.length) {
      fail('selftest', `replay import not idempotent: ${JSON.stringify(replay.body)}`)
    }
    const cardsAfterReplay = sql(`select count(*) from review_cards where group_id='${groupID}' and deleted_at is null`)
    if (cardsAfterReplay !== String(SELF_TEST_IDS.length)) {
      fail('selftest', `replay created extra cards: ${cardsAfterReplay}`)
    }
    record('selftest', 'replayed import is idempotent', replay.body?.data)

    await page.screenshot({ path: path.join(OUT, `reading_agent_ui_${RUN}_final.png`), fullPage: false })
    if (consoleErrors.length) fail('console', `${consoleErrors.length} console errors: ${consoleErrors.slice(0, 3).join(' | ')}`)
    record('console', 'no console errors')
  } finally {
    await cleanup()
    fs.writeFileSync(
      path.join(OUT, `reading_agent_ui_${RUN}.json`),
      JSON.stringify({ run: RUN, runId: runID, groupId: groupID, results, consoleErrors }, null, 2),
    )
    await browser.close()
  }

  record('cleanup', 'temporary run, self tests and review group removed')
  record('done', 'reading agent UI acceptance passed')
  console.log(`\nAcceptance evidence: ${path.join(OUT, `reading_agent_ui_${RUN}.json`)}`)
}

main().catch((err) => {
  console.error('\nACCEPTANCE FAILED:', err.message)
  process.exit(1)
})
