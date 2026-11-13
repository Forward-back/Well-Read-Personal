/**
 * Browser acceptance for Agent web-search providers and the direct-mode Key clear flow.
 *
 * Prereqs: stack running, owner credentials in deployments/.env, a DeepSeek key in
 * data/test-docs/deepseek.key (gitignored), puppeteer-core available via NODE_PATH.
 *
 * Usage:
 *   NODE_PATH=frontend/node_modules CHROME_PATH="D:/chrome-for-test/.../chrome.exe" \
 *     node scripts/acceptance/browser/provider_acceptance.js
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

const DEEPSEEK_PROVIDER = '验收-DeepSeek'
const DEEPSEEK_PROVIDER_RENAMED = '验收-DeepSeek-改'
const DUCK_PROVIDER = '验收-DuckDuckGo'
const AGENT_ID = 'accept-agent-1'

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
const keyFile = path.join(ROOT, 'data/test-docs/deepseek.key')
const apiKey = fs.existsSync(keyFile) ? fs.readFileSync(keyFile, 'utf8').trim() : ''
const log = []
const record = (step, detail) => {
  log.push({ step, detail })
  console.log(`[${step}] ${detail}`)
}

async function visibleHandles(page, selector) {
  const handles = await page.$$(selector)
  const out = []
  for (const handle of handles) {
    if (await handle.boundingBox().catch(() => null)) out.push(handle)
  }
  return out
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

async function clickDialogButton(page, text) {
  const ok = await page.evaluate((text) => {
    const btns = [...document.querySelectorAll('.t-dialog button')].filter((b) => b.offsetWidth || b.offsetHeight)
    const btn = btns.find((b) => (b.innerText || '').trim() === text)
    if (!btn) return false
    btn.click()
    return true
  }, text)
  if (!ok) throw new Error(`dialog button not found: ${text}`)
}

async function waitText(page, text, timeout = 20000) {
  await page.waitForFunction((t) => document.body.innerText.includes(t), { timeout }, text)
}

async function waitDialogsClosed(page) {
  await page.waitForFunction(() => ![...document.querySelectorAll('.t-dialog')].some((d) => d.offsetWidth || d.offsetHeight), { timeout: 15000 })
}

async function clickInCard(page, name, text) {
  const result = await page.evaluate(({ name, text }) => {
    const card = [...document.querySelectorAll('.provider-card')].find((c) => c.innerText.includes(name))
    if (!card) return 'no card'
    const btn = [...card.querySelectorAll('button')].find((b) => (b.innerText || '').includes(text))
    if (!btn) return 'no button'
    btn.click()
    return 'ok'
  }, { name, text })
  if (result !== 'ok') throw new Error(`clickInCard ${name} "${text}": ${result}`)
}

async function setInput(page, handle, value) {
  await handle.evaluate((e) => { e.focus(); e.select() })
  await page.keyboard.press('Backspace')
  await page.keyboard.type(value, { delay: 10 })
}

async function api(page, method, url) {
  return page.evaluate(async (m, u) => {
    const token = localStorage.getItem('gaze_token')
    const res = await fetch(u, { method: m, headers: { Authorization: 'Bearer ' + token } })
    let data = null
    try { data = await res.json() } catch (e) {}
    return { status: res.status, data }
  }, method, url)
}

function psql(sql) {
  const res = spawnSync('docker', ['exec', 'Gaze-postgres', 'psql', '-U', 'root', '-d', 'WellRead', '-t', '-A', '-c', sql], { encoding: 'utf8' })
  if (res.status !== 0) throw new Error((res.stderr || res.stdout || 'psql failed').trim())
  return (res.stdout || '').trim()
}

async function login(page) {
  await page.goto(BASE + '/', { waitUntil: 'networkidle2' })
  await page.waitForSelector('input[type="password"]', { timeout: 20000 })
  const email = await page.$('input[type="text"]')
  const pass = await page.$('input[type="password"]')
  await setInput(page, email, env.OWNER_EMAIL)
  await setInput(page, pass, env.OWNER_PASSWORD)
  await Promise.all([
    page.waitForFunction(() => location.pathname.includes('/platform'), { timeout: 20000 }),
    clickText(page, 'button', '登录', { exact: true }),
  ])
  await page.goto(BASE + '/platform/settings?section=integrations', { waitUntil: 'networkidle2' })
  await waitText(page, 'Agent 联网搜索配置')
}

async function createProvider(page, name, kind) {
  await clickText(page, 'button', '新建配置', { exact: true })
  const nameInput = await page.waitForSelector('input[placeholder="例如：DeepSeek 搜索"]', { visible: true, timeout: 10000 })
  await setInput(page, nameInput, name)
  if (kind === 'deepseek') {
    const keyInput = await page.waitForSelector('input[placeholder="DeepSeek API Key"]', { visible: true, timeout: 10000 })
    await setInput(page, keyInput, apiKey)
  } else {
    const select = await page.waitForSelector('input[placeholder="请选择"]', { visible: true, timeout: 10000 })
    await select.click()
    await page.waitForSelector('.t-select-option', { visible: true, timeout: 10000 })
    await page.evaluate(() => {
      const option = [...document.querySelectorAll('.t-select-option')].find((o) => o.innerText.includes('DuckDuckGo'))
      if (!option) throw new Error('DuckDuckGo option missing')
      option.click()
    })
  }
  await clickDialogButton(page, '保存')
  await waitText(page, '联网搜索配置已保存')
  await page.waitForFunction((n) => [...document.querySelectorAll('.provider-card')].some((c) => c.innerText.includes(n)), { timeout: 15000 }, name)
  await waitDialogsClosed(page)
}

async function deleteProvider(page, name) {
  await clickInCard(page, name, '删除')
  await waitText(page, '确定删除')
  await clickDialogButton(page, '删除')
  await waitText(page, '配置已删除')
  await page.waitForFunction((n) => ![...document.querySelectorAll('.provider-card')].some((c) => c.innerText.includes(n)), { timeout: 15000 }, name)
  await waitDialogsClosed(page)
}

;(async () => {
  if (!env.OWNER_EMAIL || !env.OWNER_PASSWORD) throw new Error('OWNER_EMAIL/OWNER_PASSWORD missing in deployments/.env')
  if (!apiKey) throw new Error('data/test-docs/deepseek.key missing')
  const browser = await puppeteer.launch({
    executablePath: CHROME,
    headless: 'new',
    args: ['--no-sandbox', '--disable-dev-shm-usage', '--window-size=1440,960'],
    defaultViewport: { width: 1440, height: 960 },
  })
  const page = await browser.newPage()
  const consoleErrors = []
  const pageErrors = []
  const deleteResponses = []
  page.on('console', (m) => { if (m.type() === 'error') consoleErrors.push(m.text()) })
  page.on('pageerror', (e) => pageErrors.push(String(e)))
  page.on('response', (r) => {
    if (r.request().method() === 'DELETE' && r.url().includes('/web-search-providers/')) {
      deleteResponses.push({ url: r.url(), status: r.status() })
    }
  })
  let agentInserted = false
  try {
    await login(page)
    record('panel', 'Agent 联网搜索配置 visible')

    await createProvider(page, DEEPSEEK_PROVIDER, 'deepseek')
    const cardText = await page.evaluate((n) => [...document.querySelectorAll('.provider-card')].find((c) => c.innerText.includes(n))?.innerText || '', DEEPSEEK_PROVIDER)
    if (!cardText.includes('Key：已配置') || !cardText.includes('默认')) throw new Error('first provider should be default with key configured')
    record('create.deepseek', cardText.replace(/\n+/g, ' | '))

    await clickInCard(page, DEEPSEEK_PROVIDER, '测试连接')
    await page.waitForFunction(() => document.body.innerText.includes('连接正常') || document.body.innerText.includes('连接测试失败'), { timeout: 45000 })
    const body = await page.evaluate(() => document.body.innerText)
    const testLine = (body.match(/连接正常[^\n]*|连接测试失败[^\n]*/) || ['none'])[0]
    if (!testLine.startsWith('连接正常')) throw new Error('connection test failed: ' + testLine)
    record('test', testLine)

    await clickInCard(page, DEEPSEEK_PROVIDER, '编辑')
    const editInput = await page.waitForSelector('input[placeholder="例如：DeepSeek 搜索"]', { visible: true, timeout: 10000 })
    await setInput(page, editInput, DEEPSEEK_PROVIDER_RENAMED)
    await clickDialogButton(page, '保存')
    await waitText(page, '联网搜索配置已保存')
    await page.waitForFunction((n) => [...document.querySelectorAll('.provider-card')].some((c) => c.innerText.includes(n)), { timeout: 15000 }, DEEPSEEK_PROVIDER_RENAMED)
    await waitDialogsClosed(page)
    record('edit', 'renamed')

    await createProvider(page, DUCK_PROVIDER, 'duckduckgo')
    await clickInCard(page, DUCK_PROVIDER, '设为默认')
    await page.waitForFunction(() => {
      const cards = [...document.querySelectorAll('.provider-card')]
      const duck = cards.find((c) => c.innerText.includes('验收-DuckDuckGo'))
      const deep = cards.find((c) => c.innerText.includes('验收-DeepSeek-改'))
      const hasDefaultTag = (c) => c && [...c.querySelectorAll('.t-tag')].some((t) => t.innerText.trim() === '默认')
      return duck && deep && hasDefaultTag(duck) && !hasDefaultTag(deep)
    }, { timeout: 15000 })
    record('default', 'switched to DuckDuckGo')

    const list = await api(page, 'GET', '/api/v1/web-search-providers')
    const providerId = (list.data.data || []).find((p) => p.name === DEEPSEEK_PROVIDER_RENAMED)?.id
    if (!providerId) throw new Error('provider id not found')
    const tenant = psql('select id from tenants order by id limit 1')
    psql(`insert into custom_agents (id,name,tenant_id,config,is_builtin,runnable_by_viewer) values ('${AGENT_ID}','验收 Agent',${tenant},'{"web_search_enabled":true,"web_search_provider_id":"${providerId}"}',false,true)`)
    agentInserted = true

    await clickInCard(page, DEEPSEEK_PROVIDER_RENAMED, '删除')
    await waitText(page, '确定删除')
    await clickDialogButton(page, '删除')
    await waitText(page, '正在被 Agent 使用')
    const guardLine = (await page.evaluate(() => document.body.innerText)).match(/该配置正在被 Agent 使用[^\n]*/)?.[0] || 'none'
    if (!guardLine.includes('验收 Agent')) throw new Error('409 message missing agent name')
    if (!deleteResponses.some((r) => r.status === 409)) throw new Error('no 409 response observed')
    record('guard', guardLine)
    await clickDialogButton(page, '取消')
    await waitDialogsClosed(page)

    psql(`delete from custom_agents where id='${AGENT_ID}'`)
    agentInserted = false
    await deleteProvider(page, DEEPSEEK_PROVIDER_RENAMED)
    await deleteProvider(page, DUCK_PROVIDER)
    if (await page.evaluate(() => document.querySelectorAll('.provider-card').length) !== 0) throw new Error('providers remain after cleanup')
    record('cleanup', 'both providers deleted')
    record('console', `errors=${consoleErrors.filter((e) => !e.includes('409')).length} pageErrors=${pageErrors.length}`)
    await page.screenshot({ path: path.join(OUT, 'provider-acceptance.png'), fullPage: true })
  } catch (err) {
    await page.screenshot({ path: path.join(OUT, 'provider-acceptance-failure.png'), fullPage: true }).catch(() => {})
    record('FAIL', String(err && err.stack ? err.stack.split('\n')[0] : err))
    process.exitCode = 1
  } finally {
    if (agentInserted) {
      try { psql(`delete from custom_agents where id='${AGENT_ID}'`) } catch (e) {}
    }
    fs.writeFileSync(path.join(OUT, 'provider-acceptance-log.json'), JSON.stringify({ log, consoleErrors, pageErrors, deleteResponses }, null, 2))
    await browser.close()
  }
})()
