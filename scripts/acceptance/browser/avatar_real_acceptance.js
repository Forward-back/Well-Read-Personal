/**
 * Real avatar round-trip acceptance: upload, topbar render, clear, restore.
 *
 * Prereqs: deployed frontend/backend, owner credentials in deployments/.env.
 * Usage:
 *   NODE_PATH=frontend/node_modules WELLREAD_WEB=http://localhost:6151 \
 *     node scripts/acceptance/browser/avatar_real_acceptance.js
 */
const puppeteer = require('puppeteer-core')
const fs = require('fs')
const path = require('path')

const ROOT = process.env.WELLREAD_ROOT || path.resolve(__dirname, '../../..')
const CHROME = process.env.CHROME_PATH || 'D:/chrome-for-test/chrome/win64-157.0.8103.38/chrome-win64/chrome.exe'
const BASE = process.env.WELLREAD_WEB || 'http://localhost:6151'
const PNG = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

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

async function main() {
  const browser = await puppeteer.launch({
    executablePath: CHROME,
    headless: 'new',
    defaultViewport: { width: 1280, height: 900 },
  })
  const page = await browser.newPage()
  const errors = []
  page.on('console', (msg) => {
    if (msg.type() === 'error') errors.push(msg.text())
  })
  page.on('pageerror', (err) => errors.push(err.message))

  const api = (p, init) =>
    page.evaluate(
      async (url, options) => {
        const token = localStorage.getItem('gaze_token')
        const headers = { ...(options.headers || {}) }
        if (token) headers.Authorization = `Bearer ${token}`
        const response = await fetch(url, { credentials: 'include', ...options, headers })
        return { status: response.status, body: await response.json().catch(() => null) }
      },
      p,
      init || {},
    )
  const me = async () => (await api('/api/v1/auth/me')).body?.data?.user
  const setAvatar = (avatar) =>
    api('/api/v1/auth/me/avatar', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ avatar }),
    })

  let originalAvatar = ''
  try {
    await page.goto(BASE + '/', { waitUntil: 'networkidle2' })
    await page.waitForSelector('input[type="password"]')
    const email = await page.$('input[type="text"]')
    await email.click({ clickCount: 3 })
    await email.type(env.OWNER_EMAIL)
    const password = await page.$('input[type="password"]')
    await password.click({ clickCount: 3 })
    await password.type(env.OWNER_PASSWORD)
    await Promise.all([
      page.waitForFunction(() => location.pathname === '/'),
      page.evaluate(() => [...document.querySelectorAll('button')]
        .find((button) => button.innerText.trim() === '登录')?.click()),
    ])

    const before = await me()
    const original = before?.avatar || ''
    originalAvatar = original
    console.log(`[avatar] original=${original ? original.slice(0, 24) : '<empty>'}`)

    const upload = await setAvatar(PNG)
    if (upload.status !== 200) throw new Error(`upload status ${upload.status}`)
    const afterUpload = await me()
    if (afterUpload?.avatar !== PNG) throw new Error('uploaded avatar not stored')
    console.log('[avatar] upload stored')

    await page.reload({ waitUntil: 'networkidle2' })
    await page.waitForSelector('.app-profile-btn')
    await page.waitForFunction((src) => {
      const img = document.querySelector('.app-profile-btn__img')
      return img && img.getAttribute('src') === src
    }, { timeout: 15000 }, PNG)
    console.log('[avatar] topbar renders uploaded image')

    const clear = await setAvatar('')
    if (clear.status !== 200) throw new Error(`clear status ${clear.status}`)
    const afterClear = await me()
    if (afterClear?.avatar === PNG) throw new Error('avatar still uploaded after clear')
    await page.reload({ waitUntil: 'networkidle2' })
    await page.waitForSelector('.app-profile-btn')
    if (!original || original.startsWith('data:')) {
      await page.waitForSelector('.app-profile-btn__initial', { timeout: 15000 })
      console.log('[avatar] clear falls back to initial')
    } else {
      await page.waitForFunction((src) => {
        const img = document.querySelector('.app-profile-btn__img')
        return img && img.getAttribute('src') === src
      }, { timeout: 15000 }, original)
      console.log('[avatar] clear falls back to original legacy avatar')
    }

    if (original.startsWith('data:')) {
      const restore = await setAvatar(original)
      if (restore.status !== 200) throw new Error(`restore status ${restore.status}`)
    }
    const restored = await me()
    if ((restored?.avatar || '') !== original) {
      throw new Error('original avatar not restored')
    }
    console.log('[avatar] original state restored')
    originalAvatar = ''

    if (errors.length) throw new Error(`console errors: ${errors.slice(0, 5).join(' | ')}`)
    console.log('[avatar] no console errors')
  } finally {
    if (originalAvatar.startsWith('data:')) {
      try {
        await setAvatar(originalAvatar)
        console.log('[avatar] safety restore executed')
      } catch (err) {
        console.error('[avatar] safety restore failed:', err.message)
      }
    }
    await browser.close()
  }
}

main().catch((err) => {
  console.error('AVATAR ACCEPTANCE FAILED:', err.message)
  process.exit(1)
})
