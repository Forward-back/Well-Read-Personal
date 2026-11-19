/**
 * Smoke acceptance for the V3 review page skeleton (preview route).
 *
 * Prereqs: frontend dev server or built UI reachable at WELLREAD_WEB,
 * owner credentials in deployments/.env, puppeteer-core via NODE_PATH,
 * Chrome for Testing available.
 *
 * Usage:
 *   NODE_PATH=frontend/node_modules WELLREAD_WEB=http://localhost:5174 \
 *     node scripts/acceptance/browser/review_v3_skeleton_acceptance.js
 */
const puppeteer = require('puppeteer-core')
const fs = require('fs')
const os = require('os')
const path = require('path')

const ROOT = process.env.WELLREAD_ROOT || path.resolve(__dirname, '../../..')
const CHROME = process.env.CHROME_PATH || 'D:/chrome-for-test/chrome/win64-157.0.8103.38/chrome-win64/chrome.exe'
const BASE = process.env.WELLREAD_WEB || 'http://localhost:6151'
const OUT = process.env.ACCEPT_OUT || path.join(os.tmpdir(), 'wellread_accept')
fs.mkdirSync(OUT, { recursive: true })

const RUN_ID = `v3-${Date.now().toString(36)}`
const CREATED_GROUP = `V3骨架-${RUN_ID}`
const RENAMED_GROUP = `${CREATED_GROUP}-改`

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
  await page.waitForSelector('[data-testid="review-v3-group-rail"]', { timeout: 20000 })
}

async function overflow(page) {
  return page.evaluate(() => ({
    scroll: document.documentElement.scrollWidth,
    inner: window.innerWidth,
  }))
}

async function fillVisibleDialogInput(page, value) {
  await page.waitForFunction(() => {
    const dialogs = [...document.querySelectorAll('.t-dialog')].filter((el) => el.offsetWidth || el.offsetHeight)
    const input = dialogs[dialogs.length - 1]?.querySelector('.t-input__inner')
    return Boolean(input && document.activeElement !== null)
  }, { timeout: 5000 })
  await new Promise((resolve) => setTimeout(resolve, 250))
  const ok = await page.evaluate(() => {
    const dialogs = [...document.querySelectorAll('.t-dialog')].filter((el) => el.offsetWidth || el.offsetHeight)
    const input = dialogs[dialogs.length - 1]?.querySelector('.t-input__inner')
    if (!input) return false
    input.focus()
    input.select?.()
    return true
  })
  if (!ok) throw new Error('visible dialog input missing')
  await page.keyboard.press('Backspace')
  await page.keyboard.type(value, { delay: 5 })
}

async function clickVisibleDialogButton(page, text) {
  const ok = await page.evaluate((label) => {
    const dialogs = [...document.querySelectorAll('.t-dialog')].filter((el) => el.offsetWidth || el.offsetHeight)
    const dialog = dialogs[dialogs.length - 1]
    const button = [...(dialog?.querySelectorAll('button') || [])]
      .find((el) => (el.innerText || '').trim() === label)
    if (!button) return false
    button.click()
    return true
  }, text)
  if (!ok) throw new Error(`dialog button not found: ${text}`)
}

async function openGroupMenu(page, groupName, actionLabel) {
  const hovered = await page.evaluate((name) => {
    const row = [...document.querySelectorAll('[role="treeitem"]')]
      .find((item) => (item.innerText || '').includes(name))
    if (!row) return false
    row.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))
    const menu = row.querySelector('[data-testid="review-v3-group-menu"]')
    if (!menu) return false
    menu.click()
    return true
  }, groupName)
  if (!hovered) throw new Error(`group menu trigger not found: ${groupName}`)
  await page.waitForFunction((label) => {
    return [...document.querySelectorAll('.t-dropdown__item')]
      .some((el) => (el.innerText || '').trim() === label && (el.offsetWidth || el.offsetHeight))
  }, { timeout: 5000 }, actionLabel)
  await clickText(page, '.t-dropdown__item', actionLabel, { exact: true })
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
  try {
    await login(page)
    record('login', 'owner logged in')
    await gotoV3(page)
    record('route', '/platform/review main route rendered')

    const tabLabels = await page.$$eval('.t-tabs__nav-item', (nodes) => nodes.map((node) => node.innerText.trim()))
    if (!['今日', '卡片', '统计'].every((label) => tabLabels.includes(label))) {
      throw new Error(`tabs missing: ${tabLabels.join(',')}`)
    }
    record('tabs', 'today/cards/stats visible')

    await waitText(page, '全部卡组')
    await waitText(page, '漏洞热力图')
    await waitText(page, '今日优先补强')
    record('today', 'rail, heatmap placeholder and priorities visible')

    const groupRows = await page.$$('[role="treeitem"]')
    if (groupRows.length) {
      await groupRows[0].click()
      const active = await page.evaluate(() => Boolean(document.querySelector('[role="treeitem"].group-row--active')))
      if (!active) throw new Error('group selection did not activate')
      record('group', 'first group selectable')
    } else {
      record('group', 'no custom group yet; all-groups only')
    }

    await page.click('[data-testid="review-v3-group-menu-root"]')
    await page.waitForFunction(
      () => [...document.querySelectorAll('.t-dropdown__item')]
        .some((el) => (el.innerText || '').trim() === '新建卡组' && (el.offsetWidth || el.offsetHeight)),
      { timeout: 5000 },
    )
    await clickText(page, '.t-dropdown__item', '新建卡组', { exact: true })
    await fillVisibleDialogInput(page, CREATED_GROUP)
    await clickVisibleDialogButton(page, '保存')
    await waitText(page, CREATED_GROUP)
    record('group-create', 'created and selected test group')

    await openGroupMenu(page, CREATED_GROUP, '重命名')
    await fillVisibleDialogInput(page, RENAMED_GROUP)
    await clickVisibleDialogButton(page, '保存')
    await waitText(page, RENAMED_GROUP)
    record('group-rename', 'renamed test group')

    await openGroupMenu(page, RENAMED_GROUP, '删除卡组')
    await clickVisibleDialogButton(page, '确认')
    await page.waitForFunction((name) => {
      return ![...document.querySelectorAll('[role="treeitem"]')]
        .some((item) => (item.innerText || '').includes(name))
    }, { timeout: 10000 }, RENAMED_GROUP)
    record('group-delete', 'deleted test group')

    await page.waitForFunction(() => {
      const mask = document.querySelector('.t-loading__mask')
      return !mask || !(mask.offsetWidth || mask.offsetHeight)
    }, { timeout: 10000 })
    const cta = await page.$('[data-testid="review-v3-start"]')
    if (!cta) throw new Error('task CTA missing')
    const disabled = await cta.evaluate((el) => el.classList.contains('t-is-disabled') || el.disabled)
    if (!disabled) {
      await cta.evaluate((el) => el.click())
      await waitText(page, '复习模式将在下一批接入', 5000)
      record('cta', 'start click reports pending session mode')
    } else {
      record('cta', 'disabled because no remaining queue')
    }

    await clickText(page, '.t-tabs__nav-item', '卡片', { exact: true })
    await page.waitForSelector('[data-testid="review-v3-cards"]', { timeout: 20000 })
    record('cards-tab', 'placeholder visible')
    await clickText(page, '.t-tabs__nav-item', '统计', { exact: true })
    await page.waitForSelector('[data-testid="review-v3-stats"]', { timeout: 20000 })
    record('stats-tab', 'placeholder visible')
    await clickText(page, '.t-tabs__nav-item', '今日', { exact: true })
    await page.waitForSelector('[data-testid="review-v3-task-bar"]', { timeout: 10000 })
    await new Promise((resolve) => setTimeout(resolve, 3500))

    const railOverflow = await page.evaluate(() => {
      const rail = document.querySelector('[data-testid="review-v3-group-rail"]')
      return { scroll: rail.scrollWidth, client: rail.clientWidth }
    })
    if (railOverflow.scroll > railOverflow.client + 1) {
      throw new Error(`group rail horizontal overflow: ${railOverflow.scroll} > ${railOverflow.client}`)
    }
    record('rail-layout', `no rail overflow (${railOverflow.client}px)`)

    const wide = await overflow(page)
    if (wide.scroll > wide.inner + 1) throw new Error(`1440 horizontal overflow: ${wide.scroll} > ${wide.inner}`)
    record('layout-1440', `no horizontal overflow (${wide.inner}px)`)
    await page.screenshot({ path: path.join(OUT, 'review_v3_skeleton_1440.png'), fullPage: true })

    await page.setViewport({ width: 600, height: 900 })
    await page.reload({ waitUntil: 'networkidle2' })
    await page.waitForSelector('[data-testid="review-v3-task-bar"]', { timeout: 20000 })
    const responsive = await page.evaluate(() => ({
      rail: getComputedStyle(document.querySelector('[data-testid="review-v3-group-rail"]')).display,
      mobileGroup: getComputedStyle(document.querySelector('.today-view__mobile-group')).display,
      scroll: document.documentElement.scrollWidth,
      inner: window.innerWidth,
    }))
    if (responsive.rail !== 'none') throw new Error(`rail should collapse at 600px, got ${responsive.rail}`)
    if (responsive.mobileGroup === 'none') throw new Error('mobile group selector is hidden at 600px')
    if (responsive.scroll > responsive.inner + 1) {
      throw new Error(`600px horizontal overflow: ${responsive.scroll} > ${responsive.inner}`)
    }
    record('layout-600', 'rail collapsed, mobile selector visible, no horizontal overflow')
    await page.screenshot({ path: path.join(OUT, 'review_v3_skeleton_600.png'), fullPage: true })

    if (consoleErrors.length || pageErrors.length) {
      throw new Error(`console errors: ${JSON.stringify([...consoleErrors, ...pageErrors].slice(0, 5))}`)
    }
    record('console', '0 console/page errors')
    fs.writeFileSync(path.join(OUT, 'review_v3_skeleton_log.json'), JSON.stringify({ log, consoleErrors, pageErrors }, null, 2))
    console.log(`DONE screenshots in ${OUT}`)
  } catch (err) {
    fs.writeFileSync(path.join(OUT, 'review_v3_skeleton_log.json'), JSON.stringify({ log, consoleErrors, pageErrors, error: String(err) }, null, 2))
    console.error('FAILED:', err)
    process.exitCode = 1
  } finally {
    await browser.close()
  }
}

main()
