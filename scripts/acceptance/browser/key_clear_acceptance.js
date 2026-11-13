/**
 * Browser acceptance for the direct-mode DeepSeek Key clear flow.
 *
 * Usage:
 *   NODE_PATH=frontend/node_modules CHROME_PATH="D:/chrome-for-test/.../chrome.exe" \
 *     node scripts/acceptance/browser/key_clear_acceptance.js
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
const apiKey = fs.readFileSync(path.join(ROOT, 'data/test-docs/deepseek.key'), 'utf8').trim()
const log = []
const record = (s, d) => { log.push({ s, d }); console.log(`[${s}] ${d}`) }

async function clickSectionButton(page, sectionText, buttonText) {
  const ok = await page.evaluate(({ sectionText, buttonText }) => {
    const blocks = [...document.querySelectorAll('.block')].filter((b) => b.innerText.includes(sectionText))
    const block = blocks[blocks.length - 1]
    if (!block) return false
    const btn = [...block.querySelectorAll('button')].find((b) => (b.innerText || '').trim() === buttonText)
    if (!btn) return false
    btn.click()
    return true
  }, { sectionText, buttonText })
  if (!ok) throw new Error(`section button ${sectionText}/${buttonText} missing`)
}

;(async () => {
  const browser = await puppeteer.launch({ executablePath: CHROME, headless: 'new', args: ['--no-sandbox'] })
  const page = await browser.newPage()
  await page.setViewport({ width: 1440, height: 960 })
  const consoleErrors = []
  const pageErrors = []
  page.on('console', (m) => { if (m.type() === 'error') consoleErrors.push(m.text()) })
  page.on('pageerror', (e) => pageErrors.push(String(e)))
  try {
    await page.goto(BASE + '/', { waitUntil: 'networkidle2' })
    await page.waitForSelector('input[type="password"]')
    const email = await page.$('input[type="text"]')
    const pass = await page.$('input[type="password"]')
    await email.evaluate((e) => { e.focus(); e.select() })
    await page.keyboard.type(env.OWNER_EMAIL)
    await pass.evaluate((e) => { e.focus() })
    await page.keyboard.type(env.OWNER_PASSWORD)
    await Promise.all([
      page.waitForFunction(() => location.pathname.includes('/platform')),
      page.evaluate(() => [...document.querySelectorAll('button')].find((b) => b.innerText.trim() === '登录').click()),
    ])

    await page.goto(BASE + '/platform/settings?section=account', { waitUntil: 'networkidle2' })
    await page.waitForFunction(() => document.body.innerText.includes('联网搜索'))
    const keyInput = await page.waitForSelector('.key-input input', { visible: true, timeout: 10000 })
    record('key.before', 'placeholder=' + (await keyInput.evaluate((e) => e.placeholder)))

    await keyInput.evaluate((e) => { e.focus(); e.select() })
    await page.keyboard.type(apiKey)
    await clickSectionButton(page, '联网搜索', '保存')
    await page.waitForFunction(() => document.body.innerText.includes('联网搜索配置已保存'))
    await page.waitForFunction(() => [...document.querySelectorAll('.key-status')].map((e) => e.innerText).join(' ').includes('Key 已配置'), { timeout: 15000 })
    const clearVisible = await page.evaluate(() => [...document.querySelectorAll('button')].some((b) => b.innerText.trim() === '清除已保存 Key' && (b.offsetWidth || b.offsetHeight)))
    if (!clearVisible) throw new Error('clear button not visible after saving key')
    record('key.saved', 'status=Key 已配置')

    await page.evaluate(() => [...document.querySelectorAll('button')].find((b) => b.innerText.trim() === '清除已保存 Key').click())
    await page.waitForFunction(() => document.body.innerText.includes('清除 DeepSeek Key'))
    const confirmLine = await page.evaluate(() => document.body.innerText.match(/清除后将回退到直连模型 Key[^\n]*/)?.[0] || 'none')
    if (!confirmLine.includes('回退到直连模型 Key')) throw new Error('confirm dialog missing fallback wording')
    record('key.confirm', confirmLine)
    await page.evaluate(() => {
      const btns = [...document.querySelectorAll('.t-dialog button')].filter((b) => b.offsetWidth || b.offsetHeight)
      const btn = btns.find((x) => x.innerText.trim() === '清除')
      if (!btn) throw new Error('clear confirm button missing')
      btn.click()
    })
    await page.waitForFunction(() => document.body.innerText.includes('已清除保存的 Key'))
    await page.waitForFunction(() => [...document.querySelectorAll('.key-status')].map((e) => e.innerText).join(' ').includes('未单独配置 Key'), { timeout: 15000 })
    const after = await page.evaluate(async () => {
      const token = localStorage.getItem('gaze_token')
      const res = await fetch('/api/v1/settings/kv/web-search-config', { headers: { Authorization: 'Bearer ' + token } })
      return (await res.json()).data
    })
    if (after.api_key_configured === true) throw new Error('api_key_configured should be false/omitted')
    if (after.provider !== 'deepseek') throw new Error('provider should stay deepseek')
    record('key.cleared', JSON.stringify(after))
    record('console', `errors=${consoleErrors.length} pageErrors=${pageErrors.length}`)
    await page.screenshot({ path: path.join(OUT, 'key-clear-acceptance.png'), fullPage: true })
  } catch (err) {
    await page.screenshot({ path: path.join(OUT, 'key-clear-failure.png'), fullPage: true }).catch(() => {})
    record('FAIL', String(err && err.stack ? err.stack.split('\n')[0] : err))
    process.exitCode = 1
  } finally {
    fs.writeFileSync(path.join(OUT, 'key-clear-acceptance-log.json'), JSON.stringify({ log, consoleErrors, pageErrors }, null, 2))
    await browser.close()
  }
})()
