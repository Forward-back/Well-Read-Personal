<template>
  <div ref="pageEl" class="welcome-home">
    <div ref="scrollEl" class="welcome-home__scroll">
          <article class="w-article">
            <section id="hero" class="w-hero" data-reveal>
              <div class="w-hero__copy">
                <p class="w-eyebrow">WellRead Personal</p>
                <h1 class="w-hero__title">能用起来的，才叫知识</h1>
                <p class="w-hero__sub">WellRead，让知识流动起来</p>

                <div class="w-hero__actions">
                  <t-button class="w-btn" theme="primary" size="medium" @click="goChat">开始提问</t-button>
                  <t-button class="w-btn" theme="default" variant="outline" size="medium" @click="scrollToSection('how')">
                    看看怎么用
                  </t-button>
                </div>

                <form class="w-ask" @submit.prevent="submitAsk">
                  <input
                    ref="inputEl"
                    v-model="askQuery"
                    class="w-ask__input"
                    type="text"
                    placeholder="问点什么，比如「总结我某篇论文」"
                    aria-label="向 WellRead 提问"
                    autocomplete="off"
                  />
                  <button type="submit" class="w-ask__send" aria-label="发送问题">
                    <SendIcon />
                  </button>
                </form>

                <div class="w-chips" aria-label="建议问题">
                  <button
                    v-for="q in suggestions"
                    :key="q"
                    type="button"
                    class="w-chip"
                    @click="useSuggestion(q)"
                  >
                    {{ q }}
                  </button>
                </div>

                <div class="w-hero__proof">
                  <span>本地部署</span>
                  <span>单 owner</span>
                  <span>引用可回原文</span>
                </div>
              </div>

              <aside class="w-setup-card">
                <p class="w-eyebrow">三步开始</p>
                <h2 class="w-setup-card__title">把资料交给它</h2>
                <ul class="w-check-list">
                  <li><CheckCircleIcon /> 把文档、网页、代码收进知识库</li>
                  <li><CheckCircleIcon /> 提问时给出可追溯的引用</li>
                  <li><CheckCircleIcon /> 学过的内容按节奏复习</li>
                </ul>
                <div class="w-setup-card__actions">
                  <t-button class="w-btn" theme="primary" size="medium" @click="goChat">进入问答</t-button>
                  <t-button class="w-btn" theme="default" variant="outline" size="medium" @click="go('/platform/knowledge-bases')">
                    创建知识库
                  </t-button>
                </div>
              </aside>
            </section>

            <section id="demo" class="w-demo" data-reveal>
              <div class="w-demo__tabs" role="tablist" aria-label="功能演示">
                <button
                  v-for="(tab, index) in tabs"
                  :key="tab.key"
                  type="button"
                  class="w-demo__tab"
                  role="tab"
                  :aria-selected="activeTab === tab.key"
                  :tabindex="activeTab === tab.key ? 0 : -1"
                  @click="activeTab = tab.key"
                  @keydown="onTabKeydown($event, index)"
                >
                  {{ tab.label }}
                </button>
              </div>

              <div class="w-demo__card" role="tabpanel" :aria-label="activeTabData.title">
                <div :key="activeTab" class="w-demo__body">
                  <div class="w-demo__visual">
                    <div class="w-paper">
                      <template v-if="activeTab === 'ask'">
                        <div class="w-mock">
                          <div class="w-mock__bar">
                            <span class="w-mock__dot" />
                            <span>问答</span>
                            <span class="w-mock__badge">引用 2</span>
                          </div>
                          <div class="w-bubble w-bubble--user">帮我找出「注意力机制」在论文里的定义</div>
                          <div class="w-bubble w-bubble--ai">
                            <p>论文把注意力描述为对输入加权求和：每个位置根据相关性获得权重。</p>
                            <span class="w-cite"><LinkIcon /> 论文.pdf · 第 3 页 · 分块 12</span>
                          </div>
                          <div class="w-mock__input">
                            <span>继续追问…</span>
                            <SendIcon />
                          </div>
                        </div>
                      </template>
                      <template v-else-if="activeTab === 'ingest'">
                        <div class="w-mock">
                          <div class="w-mock__bar">
                            <span class="w-mock__dot" />
                            <span>知识库入库</span>
                            <span class="w-mock__badge">3 个来源</span>
                          </div>
                          <div v-for="file in ingestFiles" :key="file.name" class="w-file-row">
                            <span class="w-file-row__icon"><FileIcon /></span>
                            <span class="w-file-row__name">{{ file.name }}</span>
                            <span class="w-file-row__meta">{{ file.meta }}</span>
                            <span class="w-file-row__state">{{ file.state }}</span>
                          </div>
                          <div class="w-chunk-row">
                            <span>分块索引</span>
                            <span class="w-chunk-row__bars">
                              <i v-for="n in 8" :key="n" :class="{ on: n <= 6 }" />
                            </span>
                            <strong>6 / 8</strong>
                          </div>
                        </div>
                      </template>
                      <template v-else>
                        <div class="w-mock">
                          <div class="w-mock__bar">
                            <span class="w-mock__dot" />
                            <span>今日复习 · 漏洞补强</span>
                            <span class="w-mock__badge">3 / 12</span>
                          </div>
                          <div class="w-review-card">
                            <span class="w-review-card__tag">严重</span>
                            <p>注意力机制的核心作用是什么？</p>
                            <div class="w-rate-row">
                              <span>忘记</span>
                              <span>想得慢</span>
                              <span>想起来</span>
                              <span>秒答</span>
                            </div>
                          </div>
                          <div class="w-review-foot">今天还有 9 张</div>
                        </div>
                      </template>
                    </div>
                  </div>

                  <div class="w-demo__copy">
                    <p class="w-eyebrow">能力</p>
                    <h2>{{ activeTabData.title }}</h2>
                    <p class="w-demo__desc">{{ activeTabData.desc }}</p>
                    <ul class="w-check-list">
                      <li v-for="bullet in activeTabData.bullets" :key="bullet">
                        <CheckCircleIcon /> {{ bullet }}
                      </li>
                    </ul>
                    <t-button class="w-btn" theme="primary" size="medium" @click="go(activeTabData.path)">
                      {{ activeTabData.cta }}
                    </t-button>
                  </div>
                </div>
              </div>
            </section>

            <section id="sources" class="w-section" data-reveal>
              <div class="w-section__head">
                <p class="w-eyebrow">使用场景</p>
                <h2 class="w-section__title">一个人也能用得起来</h2>
              </div>
              <div class="w-scenario-grid">
                <article v-for="scenario in scenarios" :key="scenario.title" class="w-scenario-card">
                  <span class="w-scenario-card__icon"><component :is="scenario.icon" /></span>
                  <h3>{{ scenario.title }}</h3>
                  <p>{{ scenario.desc }}</p>
                </article>
              </div>
            </section>

            <section id="how" class="w-section" data-reveal>
              <div class="w-section__head">
                <p class="w-eyebrow">工作方式</p>
                <h2 class="w-section__title">不是把整本书丢给模型</h2>
              </div>
              <div class="w-how">
                <ol class="w-steps">
                  <li v-for="(step, index) in steps" :key="step.title" class="w-steps__item">
                    <button
                      type="button"
                      class="w-steps__button"
                      :class="{ 'is-active': activeStep === index }"
                      :aria-pressed="activeStep === index"
                      @click="activeStep = index"
                    >
                      <span class="w-steps__num">0{{ index + 1 }}</span>
                      <span class="w-steps__text">
                        <strong>{{ step.title }}</strong>
                        <small>{{ step.desc }}</small>
                      </span>
                    </button>
                  </li>
                </ol>

                <div class="w-how__visual">
                  <div class="w-paper w-paper--how">
                    <div :key="activeStep" class="w-mock">
                      <div class="w-mock__bar">
                        <span class="w-mock__dot" />
                        <span>{{ steps[activeStep].mockTitle }}</span>
                        <span class="w-mock__badge">{{ steps[activeStep].mockBadge }}</span>
                      </div>

                      <template v-if="activeStep === 0">
                        <div v-for="file in ingestFiles" :key="file.name" class="w-file-row">
                          <span class="w-file-row__icon"><FileIcon /></span>
                          <span class="w-file-row__name">{{ file.name }}</span>
                          <span class="w-file-row__meta">{{ file.meta }}</span>
                          <span class="w-file-row__state">{{ file.state }}</span>
                        </div>
                      </template>

                      <template v-else-if="activeStep === 1">
                        <div class="w-chunk-list">
                          <div v-for="chunk in chunks" :key="chunk.id" class="w-chunk">
                            <span class="w-chunk__id">{{ chunk.id }}</span>
                            <span class="w-chunk__text">{{ chunk.text }}</span>
                          </div>
                        </div>
                      </template>

                      <template v-else-if="activeStep === 2">
                        <div class="w-search-row">注意力机制的定义</div>
                        <div v-for="hit in searchHits" :key="hit.title" class="w-hit">
                          <span class="w-hit__score">{{ hit.score }}</span>
                          <span class="w-hit__body">
                            <strong>{{ hit.title }}</strong>
                            <small>{{ hit.snippet }}</small>
                          </span>
                        </div>
                      </template>

                      <template v-else>
                        <div class="w-bubble w-bubble--user">注意力机制在论文里怎么定义？</div>
                        <div class="w-bubble w-bubble--ai">
                          <p>它把输入看作一组键值对，按相关性给每个位置分配权重，再加权求和。</p>
                          <span class="w-cite"><LinkIcon /> 论文.pdf · 第 3 页 · 分块 12</span>
                        </div>
                      </template>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            <section id="stats" class="w-stats" data-reveal>
              <div class="w-section__head">
                <p class="w-eyebrow">它现在就能做到</p>
                <h2 class="w-section__title">一套本地知识库，闭环到你每天的使用</h2>
              </div>
              <div class="w-stats__grid">
                <div v-for="stat in stats" :key="stat.label" class="w-stat">
                  <strong>{{ stat.value }}</strong>
                  <span>{{ stat.label }}</span>
                </div>
              </div>
            </section>

            <section id="start" class="w-cta" data-reveal>
              <div class="w-cta__joke">
                <span class="w-cta__badge">WellRead Personal</span>
                <strong class="w-cta__number">1</strong>
                <p>一个 owner，一套知识库，一台自己的机器。</p>
              </div>
              <div class="w-cta__action">
                <h2>现在就把资料变成第二大脑</h2>
                <p>本地部署，单 owner，数据留在自己的机器上。</p>
                <div class="w-cta__actions">
                  <t-button class="w-btn" theme="primary" size="medium" @click="goChat">进入问答</t-button>
                  <t-button class="w-btn" theme="default" variant="outline" size="medium" @click="go('/platform/knowledge-bases')">
                    创建知识库
                  </t-button>
                </div>
              </div>
            </section>

            <footer class="w-footer">
              <div class="w-footer__brand">
                <strong>WellRead</strong>
                <span>Personal · 本地部署</span>
              </div>
              <nav class="w-footer__nav" aria-label="产品入口">
                <button type="button" @click="goChat">问答</button>
                <button type="button" @click="go('/platform/knowledge-bases')">知识库</button>
                <button type="button" @click="go('/platform/reading')">精读</button>
                <button type="button" @click="go('/platform/review')">复习</button>
                <button type="button" @click="go('/platform/skills')">技能</button>
                <button type="button" @click="go('/platform/settings')">设置</button>
              </nav>
              <div class="w-footer__meta">
                <button type="button" @click="go('/platform/settings')">系统健康</button>
                <span>v{{ version }}</span>
              </div>
            </footer>
          </article>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  BookOpenIcon,
  CheckCircleIcon,
  FileIcon,
  FolderIcon,
  LinkIcon,
  SendIcon,
  StarIcon,
} from 'tdesign-icons-vue-next'
import { useMenuStore } from '@/stores/menu'

const router = useRouter()
const menuStore = useMenuStore()

const pageEl = ref<HTMLElement | null>(null)
const scrollEl = ref<HTMLElement | null>(null)
const inputEl = ref<HTMLInputElement | null>(null)
const askQuery = ref('')
const activeTab = ref<'ask' | 'ingest' | 'review'>('ask')
const activeStep = ref(0)
let revealObserver: IntersectionObserver | null = null
const reducedMotion = ref(false)

const suggestions = [
  '总结我最近读过的文档',
  '找出我复习里最薄弱的知识块',
  '对比两篇论文的方法差异',
]

const tabs = [
  {
    key: 'ask',
    label: '提问',
    title: '问你的知识库',
    desc: '带着引用回答。检索增强与直连双模式，回答里的引用可以回到原文。',
    bullets: ['知识库检索 + 直连模型', '流式回答，引用可定位', '建议问题一键填入'],
    cta: '去问答',
    path: '/platform/creatChat',
  },
  {
    key: 'ingest',
    label: '入库',
    title: '先把资料收进来',
    desc: '上传文档、网页或代码项目，解析、分块并建立向量与关键词索引。',
    bullets: ['文档、网页、Markdown、代码项目', '自动解析与分块', '向量 + BM25 双路索引'],
    cta: '创建知识库',
    path: '/platform/knowledge-bases',
  },
  {
    key: 'review',
    label: '复习',
    title: '让知识定期回来',
    desc: '四档评分与漏洞热力图，每天补强最该补的那几张。',
    bullets: ['间隔重复与每日队列', '四档评分与漏洞视图', '卡片、统计与会话记录'],
    cta: '去复习',
    path: '/platform/review',
  },
] as const

const activeTabData = computed(() => tabs.find(tab => tab.key === activeTab.value) ?? tabs[0])

const steps = [
  { title: '收进来', desc: '上传文档、网页或代码项目。', mockTitle: '入库', mockBadge: '3 个来源' },
  { title: '切开来', desc: '解析、分块，建立向量与关键词索引。', mockTitle: '分块', mockBadge: '6 块' },
  { title: '查得到', desc: '提问时先检索到最相关的分块。', mockTitle: '检索', mockBadge: 'Top 3' },
  { title: '说得清', desc: '回答附带可定位的引用，能回到原文。', mockTitle: '回答', mockBadge: '引用 1' },
] as const

const ingestFiles = [
  { name: '论文.pdf', meta: '42 页', state: '已解析' },
  { name: '课程笔记.md', meta: '18 块', state: '已索引' },
  { name: '项目文档', meta: '96 文件', state: '已索引' },
]

const chunks = [
  { id: '#12', text: '注意力机制对输入序列的每个位置计算相关性权重…' },
  { id: '#13', text: '权重经过 softmax 归一化后，与值向量加权求和…' },
  { id: '#14', text: '多头注意力把表示空间拆成多个子空间并行计算…' },
]

const searchHits = [
  { score: '0.92', title: '论文.pdf · 第 3 页', snippet: '注意力机制把输入看作键值对…' },
  { score: '0.87', title: '课程笔记.md · 第 6 块', snippet: '注意力权重反映位置之间的相关性…' },
  { score: '0.61', title: '项目文档 · 检索说明', snippet: '向量召回与 BM25 合并排序…' },
]

const scenarios = [
  { title: '读论文', desc: '先精读方法，再把关键细节放进复习队列。', icon: BookOpenIcon },
  { title: '写代码', desc: '把项目文档和代码库收进知识库，随时可查。', icon: FolderIcon },
  { title: '长期学习', desc: '把每次问答沉淀成卡片，隔几天再见一次。', icon: StarIcon },
]

const stats = [
  { value: '6', label: '个功能区' },
  { value: '17', label: '个只读 MCP 工具' },
  { value: '2', label: '路召回：向量 + BM25' },
  { value: '1', label: '个 owner，本地部署' },
]

const version = typeof __FRONTEND_VERSION__ !== 'undefined' ? __FRONTEND_VERSION__ : 'dev'

function go(path: string) {
  router.push(path)
}

function goChat() {
  router.push('/platform/creatChat')
}

function scrollToSection(id: string) {
  const target = scrollEl.value?.querySelector<HTMLElement>(`#${id}`)
  target?.scrollIntoView({ behavior: reducedMotion.value ? 'auto' : 'smooth', block: 'start' })
}

function useSuggestion(value: string) {
  askQuery.value = value
  nextTick(() => inputEl.value?.focus())
}

function submitAsk() {
  const query = askQuery.value.trim()
  if (!query) {
    inputEl.value?.focus()
    return
  }
  menuStore.setPrefillQuery(query)
  router.push('/platform/creatChat')
}

function onTabKeydown(event: KeyboardEvent, index: number) {
  const keys = ['ArrowLeft', 'ArrowRight', 'Home', 'End']
  if (!keys.includes(event.key)) return
  event.preventDefault()
  let nextIndex = index
  if (event.key === 'ArrowLeft') nextIndex = (index - 1 + tabs.length) % tabs.length
  if (event.key === 'ArrowRight') nextIndex = (index + 1) % tabs.length
  if (event.key === 'Home') nextIndex = 0
  if (event.key === 'End') nextIndex = tabs.length - 1
  activeTab.value = tabs[nextIndex].key
  nextTick(() => {
    const tablist = (event.currentTarget as HTMLElement | null)?.parentElement
    tablist?.querySelectorAll<HTMLButtonElement>('[role="tab"]')[nextIndex]?.focus()
  })
}

function setupReveal() {
  reducedMotion.value = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const items = pageEl.value?.querySelectorAll<HTMLElement>('[data-reveal]')
  if (!items?.length) return
  if (reducedMotion.value || !scrollEl.value) {
    items.forEach(item => item.classList.add('is-visible'))
    return
  }
  pageEl.value?.classList.add('reveal-ready')
  revealObserver = new IntersectionObserver(
    entries => {
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          entry.target.classList.add('is-visible')
          revealObserver?.unobserve(entry.target)
        }
      })
    },
    { root: scrollEl.value, rootMargin: '0px 0px -8% 0px', threshold: 0.08 },
  )
  items.forEach(item => revealObserver?.observe(item))
}

onMounted(() => {
  nextTick(setupReveal)
})

onBeforeUnmount(() => {
  revealObserver?.disconnect()
  revealObserver = null
})
</script>

<style lang="less" scoped>
.welcome-home {
  height: 100%;
  min-height: 0;
}

.welcome-home__scroll {
  height: 100%;
  overflow-y: auto;
  overscroll-behavior: contain;
  scroll-behavior: smooth;
}

@font-face {
  font-family: "DM Sans";
  font-style: normal;
  font-weight: 400 700;
  font-display: swap;
  src: url("../../assets/fonts/dm-sans-latin.woff2") format("woff2");
  unicode-range: U+0000-00FF, U+0131, U+0152-0153, U+02BB-02BC, U+02C6, U+02DA, U+02DC, U+0304, U+0308, U+0329,
    U+2000-206F, U+20AC, U+2122, U+2191, U+2193, U+2212, U+2215, U+FEFF, U+FFFD;
}

@font-face {
  font-family: "DM Sans";
  font-style: normal;
  font-weight: 400 700;
  font-display: swap;
  src: url("../../assets/fonts/dm-sans-latin-ext.woff2") format("woff2");
  unicode-range: U+0100-02BA, U+02BD-02C5, U+02C7-02CC, U+02CE-02D7, U+02DD-02FF, U+0304, U+0308, U+0329,
    U+1D00-1DBF, U+1E00-1E9F, U+1EF2-1EFF, U+2020, U+20A0-20AB, U+20AD-20C4, U+2113, U+2C60-2C7F, U+A720-A7FF;
}

.welcome-page {
  --w-base: #eeefe9;
  --w-topbar: #cbd5b9;
  --w-window: #d3dac6;
  --w-cream: #fdfdf8;
  --w-paper: #ffffff;
  --w-accent: #e5e7e0;
  --w-ink: #111827;
  --w-body: #374151;
  --w-muted: #5f6b5f;
  --w-line: #bfc1b7;
  --w-green: #07c05f;
  --w-green-dark: #07c05f;
  --w-green-deep: #06b04d;
  --w-green-soft: #e9f8ec;
  --w-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.25);
  --w-font: "DM Sans", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", system-ui, sans-serif;

  --td-brand-color: var(--w-green-dark);
  --td-brand-color-hover: #06b04d;
  --td-brand-color-active: var(--w-green-deep);
  --td-brand-color-focus: rgba(7, 192, 95, 0.18);
  --td-brand-color-light: var(--w-green-soft);
  --td-brand-color-disabled: #8ce0af;

  position: fixed;
  inset: 0;
  z-index: 10;
  overflow: hidden;
  color: var(--w-body);
  font-family: var(--w-font);
  background: var(--w-base);
  -webkit-font-smoothing: antialiased;
}

.welcome-page *,
.welcome-page *::before,
.welcome-page *::after {
  box-sizing: border-box;
}

.welcome-page__bg {
  position: fixed;
  inset: 0;
  z-index: 0;
  background-color: var(--w-base);
  background-position: center;
  background-repeat: no-repeat;
  background-size: cover;
}

.w-topbar {
  position: fixed;
  top: 8px;
  left: 8px;
  right: 8px;
  z-index: 50;
  height: 40px;
  background: var(--w-topbar);
  border-radius: 4px;
  box-shadow: var(--w-shadow);
  transition: box-shadow 0.2s ease, background 0.2s ease;

  &.is-scrolled {
    box-shadow: 0 18px 40px -14px rgba(0, 0, 0, 0.35);
    background: #c6d2b2;
  }
}

.w-topbar__inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  height: 100%;
  padding: 0 8px;
}

.w-topbar__left,
.w-topbar__right {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.w-brand {
  display: inline-flex;
  align-items: center;
  height: 32px;
  padding: 0 4px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: #23251d;
  cursor: pointer;

  &:hover {
    background: rgba(35, 37, 29, 0.07);
  }
}

.w-brand__name {
  font-family: "WellReadSerif", Georgia, serif;
  font-size: 24px;
  font-weight: 400;
  letter-spacing: 0;
  line-height: 1;
  white-space: nowrap;
}

.w-btn {
  border-radius: 6px;
  font-weight: 600;
}

.w-profile-btn {
  width: 28px;
  height: 28px;
  display: grid;
  place-items: center;
  padding: 0;
  overflow: hidden;
  border: 1px solid rgba(35, 37, 29, 0.25);
  border-radius: 8px;
  background: var(--w-cream);
  cursor: pointer;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;

  &:hover {
    border-color: var(--w-green-dark);
    box-shadow: 0 0 0 2px rgba(7, 192, 95, 0.16);
  }
}

.w-profile-btn__img {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: cover;
}

.w-profile-btn__initial {
  color: var(--w-green-dark);
  font-size: 12px;
  font-weight: 700;
  line-height: 1;
}

.w-rail {
  position: fixed;
  top: 66px;
  bottom: 8px;
  z-index: 20;
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 112px;
}

.w-rail--left {
  left: 4px;
}

.w-rail--right {
  right: 4px;
}

.w-rail__list {
  width: 100%;
  margin: 0;
  padding: 0;
  list-style: none;
}

.w-rail__cell {
  width: 100%;
}

.w-rail__item {
  position: relative;
  top: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 112px;
  min-height: 84px;
  padding: 8px 4px;
  border: 0;
  background: transparent;
  color: #fff;
  font: inherit;
  cursor: pointer;
  transition: top 0.15s cubic-bezier(0.4, 0, 0.2, 1);

  &::before {
    content: "";
    position: absolute;
    inset: 8px 18px 20px;
    border-radius: 40%;
    background: radial-gradient(circle at 50% 45%, rgba(7, 192, 95, 0.62), rgba(4, 79, 39, 0.22) 58%, transparent 76%);
    filter: blur(12px);
    opacity: 0;
    transition: opacity 0.2s ease;
    pointer-events: none;
  }

  &:hover {
    top: -0.5px;

    &::before {
      opacity: 1;
    }
  }

  &:active {
    top: 0.5px;
  }
}

.w-rail__item--bottom {
  margin-top: auto;
  min-height: 76px;
}

.w-rail__icon {
  display: grid;
  place-items: center;
  font-size: 36px;
  line-height: 1;
  filter: drop-shadow(0 1px 2px rgba(20, 30, 15, 0.35));

  :deep(svg) {
    width: 1em;
    height: 1em;
  }
}

.w-rail__label {
  max-width: 96px;
  color: #fff;
  font-size: 13px;
  font-weight: 500;
  line-height: 16px;
  text-align: center;
  text-shadow: 0 1px 3px rgba(15, 24, 10, 0.65);
}

.w-stage {
  position: fixed;
  top: 48px;
  right: 8px;
  bottom: 0;
  left: 8px;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

.w-window {
  width: 80%;
  max-width: 1200px;
  height: 95%;
  overflow: hidden;
  border-radius: 20px;
  background: var(--w-window);
  box-shadow:
    0 30px 60px -20px rgba(31, 41, 26, 0.36),
    0 0 0 1px rgba(158, 160, 150, 0.35);
  pointer-events: auto;
}

.w-window__scroll {
  height: 100%;
  overflow-y: auto;
  overscroll-behavior: contain;
  scroll-behavior: smooth;

  &::-webkit-scrollbar {
    width: 10px;
  }

  &::-webkit-scrollbar-thumb {
    border: 3px solid transparent;
    border-radius: 999px;
    background: rgba(35, 48, 28, 0.28);
    background-clip: padding-box;
  }

  &::-webkit-scrollbar-thumb:hover {
    background: rgba(35, 48, 28, 0.4);
    background-clip: padding-box;
  }
}

.w-article {
  display: flex;
  flex-direction: column;
  gap: 48px;
  padding: 48px 80px 16px;
  color: var(--w-body);
  font-size: 18px;
  font-weight: 500;
  line-height: 32px;

  > section {
    scroll-margin-top: 16px;
  }
}

.w-eyebrow {
  margin: 0 0 8px;
  color: var(--w-muted);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.12em;
  line-height: 18px;
  text-transform: uppercase;
}

.w-hero {
  display: grid;
  grid-template-columns: 1.4fr 1fr;
  gap: 32px;
  align-items: start;
  padding-top: 8px;
}

.w-hero__title {
  margin: 0;
  color: var(--w-ink);
  font-size: 36px;
  font-weight: 800;
  letter-spacing: -0.9px;
  line-height: 40px;
}

.w-hero__sub {
  margin: 12px 0 0;
  color: var(--w-body);
  font-size: 17px;
  font-weight: 500;
  line-height: 25.5px;
}

.w-hero__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 20px;
}

.w-ask {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 20px;
  padding: 4px;
  border: 1px solid var(--w-line);
  border-radius: 8px;
  background: var(--w-cream);
  box-shadow: 0 8px 18px -14px rgba(31, 41, 26, 0.45);
  transition: border-color 0.15s ease, box-shadow 0.15s ease;

  &:focus-within {
    border-color: var(--w-green-dark);
    box-shadow: 0 0 0 3px rgba(7, 192, 95, 0.14);
  }
}

.w-ask__input {
  flex: 1;
  min-width: 0;
  height: 36px;
  padding: 0 10px;
  border: 0;
  outline: none;
  background: transparent;
  color: var(--w-ink);
  font: inherit;
  font-size: 15px;
  font-weight: 500;

  &::placeholder {
    color: #8a948a;
  }
}

.w-ask__send {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: var(--w-green-dark);
  color: #fff;
  font-size: 18px;
  cursor: pointer;
  transition: background 0.15s ease;

  &:hover {
    background: #06b04d;
  }
}

.w-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
}

.w-chip {
  padding: 4px 10px;
  border: 1px solid #c9d1c0;
  border-radius: 999px;
  background: rgba(253, 253, 248, 0.62);
  color: #4b5563;
  font: inherit;
  font-size: 12px;
  line-height: 18px;
  cursor: pointer;
  transition: border-color 0.15s ease, color 0.15s ease, background 0.15s ease;

  &:hover {
    border-color: var(--w-green-dark);
    background: var(--w-cream);
    color: var(--w-green-dark);
  }
}

.w-hero__proof {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin-top: 18px;
  color: var(--w-muted);
  font-size: 12px;
  font-weight: 500;
  line-height: 18px;

  span {
    display: inline-flex;
    align-items: center;
    gap: 6px;

    &::before {
      content: "";
      width: 5px;
      height: 5px;
      border-radius: 50%;
      background: var(--w-green-dark);
    }
  }
}

.w-setup-card {
  width: 100%;
  max-width: 448px;
  justify-self: end;
  padding: 24px;
  border: 1px solid var(--w-line);
  border-radius: 6px;
  background: var(--w-cream);
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.18);
}

.w-setup-card__title {
  margin: 0 0 14px;
  color: var(--w-ink);
  font-size: 24px;
  font-weight: 700;
  letter-spacing: -0.6px;
  line-height: 32px;
}

.w-check-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;

  li {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    color: var(--w-body);
    font-size: 14px;
    font-weight: 500;
    line-height: 22px;
  }

  :deep(svg) {
    flex: none;
    margin-top: 3px;
    color: var(--w-green-dark);
    font-size: 16px;
  }
}

.w-setup-card__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 20px;
}

.w-demo__tabs {
  display: flex;
  gap: 2px;
  border-bottom: 1px solid var(--w-line);
}

.w-demo__tab {
  padding: 10px 16px;
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: var(--w-muted);
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  line-height: 20px;
  cursor: pointer;
  transition: color 0.15s ease, border-color 0.15s ease, background 0.15s ease;

  &:hover {
    color: var(--w-ink);
  }

  &[aria-selected="true"] {
    border-bottom-color: var(--w-green-dark);
    background: rgba(253, 253, 248, 0.65);
    color: var(--w-ink);
  }
}

.w-demo__card {
  overflow: hidden;
  border: 1px solid var(--w-line);
  border-top: 0;
  border-radius: 0 0 6px 6px;
  background: var(--w-cream);
  box-shadow: var(--w-shadow);
}

.w-demo__body {
  display: grid;
  grid-template-columns: 1.4fr 1fr;
  min-height: 420px;
  animation: w-fade-up 0.24s ease both;
}

.w-demo__visual {
  display: grid;
  place-items: center;
  padding: 24px;
  border-right: 1px solid rgba(191, 193, 183, 0.6);
}

.w-demo__copy {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: flex-start;
  gap: 12px;
  padding: 28px;

  h2 {
    margin: 0;
    color: var(--w-ink);
    font-size: 24px;
    font-weight: 700;
    letter-spacing: -0.6px;
    line-height: 32px;
  }
}

.w-demo__desc {
  margin: 0;
  color: var(--w-body);
  font-size: 15px;
  font-weight: 500;
  line-height: 24px;
}

.w-paper {
  position: relative;
  display: flex;
  flex-direction: column;
  width: 100%;
  min-height: 360px;
  padding: 18px;
  border: 1px solid var(--w-line);
  border-radius: 4px;
  background-color: #eef0e7;
  background-image:
    radial-gradient(72% 72% at 50% 38%, rgba(7, 192, 95, 0.07), transparent 70%),
    radial-gradient(rgba(18, 18, 18, 0.055) 1.4px, transparent 0);
  background-size: auto, 14px 14px;
}

.w-paper--how {
  min-height: 420px;
}

.w-mock {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
  animation: w-fade-up 0.22s ease both;
}

.w-mock__bar {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--w-ink);
  font-size: 13px;
  font-weight: 700;
  line-height: 18px;
}

.w-mock__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--w-green-dark);
  box-shadow: 0 0 0 3px rgba(7, 192, 95, 0.14);
}

.w-mock__badge {
  margin-left: auto;
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--w-green-soft);
  color: var(--w-green-dark);
  font-size: 11px;
  font-weight: 600;
  line-height: 18px;
}

.w-bubble {
  max-width: 92%;
  padding: 10px 12px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  line-height: 20px;

  p {
    margin: 0;
  }
}

.w-bubble--user {
  align-self: flex-end;
  background: #dfe7d4;
  color: #26331f;
}

.w-bubble--ai {
  align-self: flex-start;
  border: 1px solid #d9dfd0;
  background: var(--w-paper);
  color: var(--w-body);
}

.w-cite {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  margin-top: 8px;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--w-green-soft);
  color: var(--w-green-dark);
  font-size: 11px;
  font-weight: 600;
  line-height: 18px;
}

.w-mock__input {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: auto;
  padding: 8px 10px;
  border: 1px solid #d9dfd0;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.72);
  color: #8a948a;
  font-size: 12px;
  line-height: 18px;

  :deep(svg) {
    color: var(--w-green-dark);
    font-size: 16px;
  }
}

.w-file-row {
  display: grid;
  grid-template-columns: 26px minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid #d9dfd0;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.7);
  font-size: 12px;
  font-weight: 500;
  line-height: 18px;
}

.w-file-row__icon {
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  border-radius: 6px;
  background: var(--w-green-soft);
  color: var(--w-green-dark);
  font-size: 15px;
}

.w-file-row__name {
  overflow: hidden;
  color: var(--w-ink);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.w-file-row__meta {
  color: var(--w-muted);
}

.w-file-row__state {
  padding: 1px 7px;
  border-radius: 999px;
  background: var(--w-green-soft);
  color: var(--w-green-dark);
  font-size: 11px;
  font-weight: 600;
}

.w-chunk-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: auto;
  padding-top: 8px;
  color: var(--w-muted);
  font-size: 12px;
  font-weight: 500;

  strong {
    margin-left: auto;
    color: var(--w-ink);
    font-size: 12px;
  }
}

.w-chunk-row__bars {
  display: inline-flex;
  gap: 3px;

  i {
    width: 10px;
    height: 10px;
    border-radius: 2px;
    background: #d5dccb;

    &.on {
      background: var(--w-green-dark);
    }
  }
}

.w-review-card {
  padding: 14px;
  border: 1px solid #d9dfd0;
  border-radius: 8px;
  background: var(--w-paper);
  box-shadow: 0 12px 24px -18px rgba(31, 41, 26, 0.5);

  p {
    margin: 8px 0 14px;
    color: var(--w-ink);
    font-size: 14px;
    font-weight: 600;
    line-height: 22px;
  }
}

.w-review-card__tag {
  display: inline-block;
  padding: 1px 7px;
  border-radius: 999px;
  background: #fdecec;
  color: #b42318;
  font-size: 11px;
  font-weight: 600;
  line-height: 18px;
}

.w-rate-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 6px;

  span {
    padding: 5px 2px;
    border: 1px solid #d9dfd0;
    border-radius: 4px;
    background: #f6f8f1;
    color: #4b5563;
    font-size: 11px;
    font-weight: 600;
    line-height: 16px;
    text-align: center;
  }

  span:last-child {
    border-color: var(--w-green-dark);
    background: var(--w-green-soft);
    color: var(--w-green-dark);
  }
}

.w-review-foot {
  margin-top: auto;
  color: var(--w-muted);
  font-size: 12px;
  font-weight: 500;
}

.w-chunk-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.w-chunk {
  display: grid;
  grid-template-columns: 38px minmax(0, 1fr);
  gap: 8px;
  padding: 9px 10px;
  border: 1px solid #d9dfd0;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.7);
}

.w-chunk__id {
  color: var(--w-green-dark);
  font-size: 11px;
  font-weight: 700;
  line-height: 18px;
}

.w-chunk__text {
  overflow: hidden;
  color: var(--w-body);
  font-size: 12px;
  font-weight: 500;
  line-height: 18px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.w-search-row {
  padding: 8px 10px;
  border: 1px solid var(--w-green-dark);
  border-radius: 6px;
  background: var(--w-cream);
  color: var(--w-ink);
  font-size: 12px;
  font-weight: 600;
  line-height: 18px;
}

.w-hit {
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr);
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid #d9dfd0;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.7);
}

.w-hit__score {
  color: var(--w-green-dark);
  font-size: 12px;
  font-weight: 700;
  line-height: 18px;
}

.w-hit__body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;

  strong {
    color: var(--w-ink);
    font-size: 12px;
    font-weight: 600;
    line-height: 18px;
  }

  small {
    overflow: hidden;
    color: var(--w-muted);
    font-size: 11px;
    font-weight: 500;
    line-height: 17px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.w-section__head {
  margin-bottom: 20px;
}

.w-section__title {
  margin: 0;
  color: var(--w-ink);
  font-size: 24px;
  font-weight: 700;
  letter-spacing: -0.6px;
  line-height: 32px;
}

.w-scenario-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.w-scenario-card {
  padding: 24px;
  border: 1px solid var(--w-line);
  border-radius: 6px;
  background: var(--w-cream);
  box-shadow: 0 16px 32px -26px rgba(31, 41, 26, 0.55);

  &:nth-child(3) {
    grid-column: span 2;
  }

  h3 {
    margin: 12px 0 6px;
    color: var(--w-ink);
    font-size: 18px;
    font-weight: 700;
    line-height: 26px;
  }

  p {
    margin: 0;
    color: var(--w-body);
    font-size: 14px;
    font-weight: 500;
    line-height: 22px;
  }
}

.w-scenario-card__icon {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: var(--w-green-soft);
  color: var(--w-green-dark);
  font-size: 24px;
}

.w-how {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 32px;
  align-items: start;
}

.w-steps {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.w-steps__button {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  width: 100%;
  padding: 14px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease, box-shadow 0.15s ease;

  &:hover {
    background: rgba(253, 253, 248, 0.55);
  }

  &.is-active {
    border-color: var(--w-line);
    background: var(--w-cream);
    box-shadow: 0 16px 32px -26px rgba(31, 41, 26, 0.55);
  }
}

.w-steps__num {
  color: var(--w-green-dark);
  font-size: 22px;
  font-weight: 800;
  line-height: 28px;
}

.w-steps__text {
  display: flex;
  flex-direction: column;
  gap: 2px;

  strong {
    color: var(--w-ink);
    font-size: 16px;
    font-weight: 700;
    line-height: 24px;
  }

  small {
    color: var(--w-muted);
    font-size: 13px;
    font-weight: 500;
    line-height: 20px;
  }
}

.w-how__visual {
  position: sticky;
  top: 16px;
}

.w-stats {
  padding: 28px;
  border: 1px solid var(--w-line);
  border-radius: 6px;
  background: var(--w-cream);
  box-shadow: 0 16px 32px -26px rgba(31, 41, 26, 0.55);
}

.w-stats__grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  margin-top: 22px;
}

.w-stat {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding-top: 14px;
  border-top: 1px solid #dfe4d7;

  strong {
    color: var(--w-ink);
    font-size: 40px;
    font-weight: 800;
    letter-spacing: -1.2px;
    line-height: 1;
  }

  span {
    color: var(--w-muted);
    font-size: 13px;
    font-weight: 500;
    line-height: 20px;
  }
}

.w-cta {
  display: grid;
  grid-template-columns: 1fr 1.2fr;
  overflow: hidden;
  border: 1px solid var(--w-line);
  border-radius: 6px;
  background: var(--w-cream);
  box-shadow: var(--w-shadow);
}

.w-cta__joke {
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 28px;
  background: linear-gradient(150deg, #e9f8ec, #d9f0e1);

  p {
    margin: 8px 0 0;
    color: #31523d;
    font-size: 14px;
    font-weight: 500;
    line-height: 22px;
  }
}

.w-cta__badge {
  align-self: flex-start;
  padding: 2px 8px;
  border: 1px solid rgba(7, 192, 95, 0.28);
  border-radius: 999px;
  color: var(--w-green-dark);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.06em;
  line-height: 18px;
  text-transform: uppercase;
}

.w-cta__number {
  margin-top: 6px;
  color: var(--w-green-dark);
  font-size: 96px;
  font-weight: 800;
  letter-spacing: -4px;
  line-height: 1;
}

.w-cta__action {
  padding: 32px;

  h2 {
    margin: 0;
    color: var(--w-ink);
    font-size: 28px;
    font-weight: 800;
    letter-spacing: -0.8px;
    line-height: 36px;
  }

  p {
    margin: 10px 0 0;
    color: var(--w-body);
    font-size: 15px;
    font-weight: 500;
    line-height: 24px;
  }
}

.w-cta__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 20px;
}

.w-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 0 8px;
  border-top: 1px solid rgba(191, 193, 183, 0.7);
  color: var(--w-muted);
  font-size: 12px;
  font-weight: 500;
  line-height: 18px;
}

.w-footer__brand {
  display: flex;
  flex-direction: column;
  gap: 1px;

  strong {
    color: var(--w-ink);
    font-size: 13px;
    font-weight: 700;
  }
}

.w-footer__nav,
.w-footer__meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;

  button {
    padding: 0;
    border: 0;
    background: transparent;
    color: inherit;
    font: inherit;
    cursor: pointer;

    &:hover {
      color: var(--w-green-dark);
    }
  }
}

.w-footer__meta {
  gap: 10px;
}

.welcome-home.reveal-ready [data-reveal] {
  opacity: 0;
  transform: translateY(14px);
  transition: opacity 0.5s ease, transform 0.5s ease;

  &.is-visible {
    opacity: 1;
    transform: none;
  }
}

@keyframes w-fade-up {
  from {
    opacity: 0;
    transform: translateY(6px);
  }

  to {
    opacity: 1;
    transform: none;
  }
}

@media (max-width: 1280px) {
  .w-window {
    width: 84%;
  }

  .w-article {
    padding: 48px 48px 16px;
  }
}

@media (max-width: 1100px) {
  .w-rail--right {
    display: none;
  }

  .w-window {
    width: 88%;
  }
}

@media (max-width: 900px) {
  .w-rail--left {
    display: none;
  }

  .w-window {
    width: 94%;
  }

  .w-article {
    padding: 32px 24px 16px;
    font-size: 16px;
    line-height: 28px;
  }

  .w-hero {
    grid-template-columns: 1fr;
  }

  .w-setup-card {
    max-width: none;
    justify-self: stretch;
  }

  .w-demo__body,
  .w-how {
    grid-template-columns: 1fr;
  }

  .w-demo__visual {
    border-right: 0;
    border-bottom: 1px solid rgba(191, 193, 183, 0.6);
  }

  .w-how__visual {
    position: static;
  }

  .w-stats__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .w-cta {
    grid-template-columns: 1fr;
  }

  .w-cta__joke {
    padding-bottom: 18px;
  }

  .w-cta__number {
    font-size: 64px;
  }

  .w-footer {
    flex-wrap: wrap;
  }
}

@media (max-width: 600px) {
  .w-window {
    width: calc(100% - 8px);
    height: calc(100% - 4px);
    border-radius: 16px;
  }

  .w-article {
    padding: 24px 16px 12px;
  }

  .w-hero__title {
    font-size: 30px;
    line-height: 36px;
    letter-spacing: -0.6px;
  }

  .w-hero__sub {
    font-size: 15px;
    line-height: 23px;
  }

  .w-demo__visual {
    padding: 14px;
  }

  .w-paper {
    min-height: 300px;
    padding: 14px;
  }

  .w-stats {
    padding: 20px;
  }

  .w-stat strong {
    font-size: 32px;
  }

  .w-cta__action {
    padding: 22px;
  }

  .w-cta__action h2 {
    font-size: 24px;
    line-height: 32px;
  }

  .w-footer__nav,
  .w-footer__meta {
    gap: 8px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .welcome-page *,
  .welcome-page *::before,
  .welcome-page *::after {
    scroll-behavior: auto !important;
    transition-duration: 0.001ms !important;
    animation-duration: 0.001ms !important;
    animation-iteration-count: 1 !important;
  }

  .welcome-home.reveal-ready [data-reveal] {
    opacity: 1;
    transform: none;
  }
}
</style>
