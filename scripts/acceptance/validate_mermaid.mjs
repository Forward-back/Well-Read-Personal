/**
 * Validate a Mermaid flowchart by rendering it with Mermaid v11 in headless Chrome.
 *
 * Usage:
 *   NODE_PATH=frontend/node_modules CHROME_PATH="D:/chrome-for-test/.../chrome.exe" \
 *     node scripts/acceptance/validate_mermaid.mjs path/to/diagram.mmd
 *
 * Prints {"ok":true} or {"ok":false,"error":"..."} and exits non-zero on failure.
 */
import fs from 'node:fs'
import http from 'node:http'
import path from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

const require = createRequire(import.meta.url)
const ROOT = process.env.WELLREAD_ROOT || path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const CHROME = process.env.CHROME_PATH || 'D:/chrome-for-test/chrome/win64-157.0.8103.38/chrome-win64/chrome.exe'
const puppeteer = require(process.env.PUPPETEER_CORE || path.join(ROOT, 'frontend/node_modules/puppeteer-core'))

const file = process.argv[2]
if (!file) {
  console.error('usage: node validate_mermaid.mjs <diagram-file>')
  process.exit(2)
}
const code = fs.readFileSync(file, 'utf8')

const dist = path.join(ROOT, 'frontend/node_modules/mermaid/dist')
const pageHtml = `<!doctype html><html><body><script type="module">
import mermaid from '/mermaid.esm.min.mjs'
mermaid.initialize({ startOnLoad: false, securityLevel: 'strict' })
window.__parse = async (code) => { await mermaid.parse(code); return true }
window.__ready = true
</script></body></html>`

const server = http.createServer((req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1')
  if (url.pathname === '/') {
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' })
    res.end(pageHtml)
    return
  }
  const target = path.join(dist, decodeURIComponent(url.pathname))
  if (!target.startsWith(dist) || !fs.existsSync(target)) {
    res.writeHead(404)
    res.end('not found')
    return
  }
  res.writeHead(200, { 'Content-Type': 'text/javascript; charset=utf-8' })
  fs.createReadStream(target).pipe(res)
})

await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
const port = server.address().port
let browser
try {
  browser = await puppeteer.launch({ executablePath: CHROME, headless: 'new', args: ['--no-sandbox'] })
  const page = await browser.newPage()
  page.on('console', (m) => { if (m.type() === 'error') console.error('page console:', m.text()) })
  page.on('pageerror', (e) => console.error('page error:', e.message))
  page.on('requestfailed', (r) => console.error('request failed:', r.url(), r.failure()?.errorText))
  await page.goto(`http://127.0.0.1:${port}/`, { waitUntil: 'networkidle0', timeout: 30000 })
  await page.waitForFunction(() => window.__ready === true, { timeout: 20000 })
  await page.evaluate((diagram) => window.__parse(diagram), code)
  console.log(JSON.stringify({ ok: true }))
} catch (err) {
  console.log(JSON.stringify({ ok: false, error: String(err && err.message ? err.message : err).slice(0, 500) }))
  process.exitCode = 1
} finally {
  if (browser) await browser.close()
  server.close()
}
