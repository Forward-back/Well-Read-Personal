/**
 * Browser smoke for login entry, redirects, shared shell and settings route.
 *
 * Covers the gap left by the review-only acceptance scripts:
 * - unauthenticated business route -> /login
 * - login success -> /
 * - /welcome -> /
 * - logged-in /login -> /
 * - left/right rails, settings detail route, 600px overflow
 *
 * Read-only: logs in with the owner account from deployments/.env, does not
 * write review data or change settings.
 *
 * Usage:
 *   NODE_PATH=frontend/node_modules \
 *     WELLREAD_WEB=http://localhost:6151 \
 *     node scripts/acceptance/browser/shell_auth_acceptance.js
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
const log = []
const record = (step, detail) => {
  log.push({ step, detail })
  console.log(`[${step}] ${detail}`)
}

async function setInput(page, selector, value) {
  const input = await page.waitForSelector(selector, { timeout: 20000 })
  await input.click({ clickCount: 3 })
  await input.type(value)
}

async function clickText(page, selector, text, exact = false) {
  const ok = await page.evaluate(({ selector, text, exact }) => {
    const els = [...document.querySelectorAll(selector)]
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

async function waitPath(page, pathname, timeout = 20000) {
  await page.waitForFunction((target) => location.pathname === target, { timeout }, pathname)
}

async function login(page) {
  await page.goto(BASE + '/', { waitUntil: 'networkidle2' })
  await page.waitForSelector('input[type="password"]', { timeout: 20000 })
  await setInput(page, 'input[type="text"]', env.OWNER_EMAIL)
  await setInput(page, 'input[type="password"]', env.OWNER_PASSWORD)
  await Promise.all([
    waitPath(page, '/'),
    clickText(page, 'button', '登录', true),
  ])
}

async function railLabels(page, ariaLabel) {
  return page.$$eval(`aside[aria-label="${ariaLabel}"] .app-rail__label`, (nodes) =>
    nodes.map((node) => node.innerText.trim()),
  )
}

async function overflow(page) {
  return page.evaluate(() => ({
    scroll: document.documentElement.scrollWidth,
    inner: window.innerWidth,
  }))
}

async function main() {
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
    await page.goto(BASE + '/platform/review', { waitUntil: 'networkidle2' })
    await waitPath(page, '/login')
    record('guard', 'unauthenticated business route redirected to /login')

    await login(page)
    record('login', 'owner logged in and landed on /')
    await page.waitForSelector('.app-shell', { timeout: 20000 })
    await page.waitForSelector('aside[aria-label="功能区"]', { timeout: 20000 })
    await page.waitForSelector('aside[aria-label="辅助入口"]', { timeout: 20000 })
    const leftLabels = await railLabels(page, '功能区')
    const rightLabels = await railLabels(page, '辅助入口')
    if (leftLabels.length !== 6) throw new Error(`left rail expected 6 items, got ${leftLabels.length}`)
    if (rightLabels.length !== 6) throw new Error(`right rail expected 6 items, got ${rightLabels.length}`)
    if (!leftLabels.includes('首页') || !leftLabels.includes('复习')) {
      throw new Error(`left rail labels unexpected: ${leftLabels.join(',')}`)
    }
    if (!rightLabels.includes('设置') || !rightLabels.includes('退出登录')) {
      throw new Error(`right rail labels unexpected: ${rightLabels.join(',')}`)
    }
    record('shell', `left=${leftLabels.join('/')} right=${rightLabels.join('/')}`)

    await page.goto(BASE + '/welcome', { waitUntil: 'networkidle2' })
    await waitPath(page, '/')
    record('redirect', '/welcome -> /')

    await page.goto(BASE + '/login', { waitUntil: 'networkidle2' })
    await waitPath(page, '/')
    record('redirect', 'logged-in /login -> /')

    await page.goto(BASE + '/', { waitUntil: 'networkidle2' })
    await clickText(page, 'aside[aria-label="功能区"] button', '知识库')
    await waitPath(page, '/platform/knowledge-bases')
    record('nav', 'left rail -> /platform/knowledge-bases')

    await clickText(page, 'aside[aria-label="功能区"] button', '复习')
    await waitPath(page, '/platform/review')
    await page.waitForSelector('[data-testid="review-v3-task-bar"]', { timeout: 20000 })
    record('nav', 'left rail -> /platform/review (V3 today view rendered)')

    await clickText(page, 'aside[aria-label="功能区"] button', '首页')
    await waitPath(page, '/')
    record('nav', 'left rail -> /')

    await clickText(page, 'aside[aria-label="辅助入口"] button', '设置')
    await waitPath(page, '/platform/settings')
    await page.waitForFunction(() => document.body.innerText.includes('账户与助手'), { timeout: 20000 })
    record('nav', 'right rail -> /platform/settings detail page')

    const avatar = await page.evaluate(() => {
      const el = document.querySelector('.app-profile-btn')
      if (!el) return null
      return el.querySelector('img') ? 'image' : 'initial'
    })
    if (!avatar) throw new Error('topbar profile entry missing')
    record('avatar', `topbar profile entry present (${avatar})`)

    const desktop = await overflow(page)
    if (desktop.scroll > desktop.inner + 1) {
      throw new Error(`desktop overflow: ${desktop.scroll} > ${desktop.inner}`)
    }
    await page.setViewport({ width: 600, height: 900 })
    await page.goto(BASE + '/', { waitUntil: 'networkidle2' })
    const narrow = await overflow(page)
    if (narrow.scroll > narrow.inner + 1) {
      throw new Error(`600px overflow: ${narrow.scroll} > ${narrow.inner}`)
    }
    record('layout', `no horizontal overflow at 1440 and 600 (${narrow.scroll}/${narrow.inner})`)

    await page.screenshot({ path: path.join(OUT, 'shell-auth-600.png'), fullPage: false })
    if (consoleErrors.length || pageErrors.length) {
      throw new Error(`console/page errors: ${consoleErrors.concat(pageErrors).join(' | ')}`)
    }
    record('console', '0 console errors, 0 page errors')
    fs.writeFileSync(path.join(OUT, 'shell_auth_acceptance.json'), JSON.stringify({ ok: true, log }, null, 2))
    console.log('SHELL_AUTH_ACCEPTANCE_OK')
  } catch (err) {
    fs.writeFileSync(path.join(OUT, 'shell_auth_acceptance.json'), JSON.stringify({ ok: false, log, error: String(err) }, null, 2))
    console.error('SHELL_AUTH_ACCEPTANCE_FAILED:', err)
    process.exitCode = 1
  } finally {
    await browser.close()
  }
}

main()
