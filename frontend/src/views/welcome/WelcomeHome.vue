<template>
  <div ref="pageEl" class="landing">
    <div ref="scrollEl" class="landing__scroll">
      <div class="landing__flow" aria-hidden="true">
        <svg viewBox="0 0 1440 2200" preserveAspectRatio="none">
          <defs>
            <linearGradient id="flow-gradient" x1="0" y1="0" x2="1" y2="1">
              <stop offset="0%" stop-color="#07c05f" stop-opacity="0" />
              <stop offset="35%" stop-color="#07c05f" stop-opacity="0.45" />
              <stop offset="70%" stop-color="#8fdcc0" stop-opacity="0.35" />
              <stop offset="100%" stop-color="#07c05f" stop-opacity="0" />
            </linearGradient>
            <linearGradient id="flow-gradient-2" x1="0" y1="1" x2="1" y2="0">
              <stop offset="0%" stop-color="#e8b86d" stop-opacity="0" />
              <stop offset="50%" stop-color="#e8b86d" stop-opacity="0.28" />
              <stop offset="100%" stop-color="#07c05f" stop-opacity="0" />
            </linearGradient>
          </defs>
          <path
            class="landing__flow-line"
            d="M-80 260 C 220 80, 430 430, 720 270 S 1180 70, 1520 330"
            stroke="url(#flow-gradient)"
          />
          <path
            class="landing__flow-line landing__flow-line--slow"
            d="M-100 640 C 250 460, 520 780, 820 560 S 1250 420, 1540 700"
            stroke="url(#flow-gradient-2)"
          />
          <path
            class="landing__flow-line landing__flow-line--faint"
            d="M-120 1500 C 300 1300, 520 1720, 860 1480 S 1280 1300, 1560 1560"
            stroke="url(#flow-gradient)"
          />
        </svg>
      </div>

      <div class="landing__inner">
        <section class="hero" data-reveal>
          <h1 class="hero__title">
            {{ $t('welcomeHome.titlePrefix') }}<span>{{ $t('welcomeHome.titleAccent') }}</span>
          </h1>
          <p class="hero__sub">{{ $t('welcomeHome.subtitle') }}</p>

          <div class="hero__actions">
            <t-button class="landing-btn landing-btn--primary" theme="primary" size="large" @click="goChat">
              {{ $t('welcomeHome.primaryCta') }}
            </t-button>
            <t-button class="landing-btn" theme="default" variant="outline" size="large" @click="scrollToSection('features')">
              {{ $t('welcomeHome.secondaryCta') }}
            </t-button>
          </div>

          <form class="hero__ask" @submit.prevent="submitAsk">
            <span class="hero__ask-icon" aria-hidden="true">
              <svg viewBox="0 0 20 20" fill="none">
                <circle cx="9" cy="9" r="5.5" stroke="currentColor" stroke-width="1.6" />
                <path d="M13 13l4 4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
              </svg>
            </span>
            <input
              ref="inputEl"
              v-model="askQuery"
              type="text"
              class="hero__ask-input"
              :placeholder="$t('welcomeHome.askPlaceholder')"
              :aria-label="$t('welcomeHome.askAria')"
              autocomplete="off"
            />
            <button type="submit" class="hero__ask-send" :aria-label="$t('welcomeHome.sendAria')">
              <SendIcon />
            </button>
          </form>

          <div class="hero__chips">
            <button
              v-for="q in suggestions"
              :key="q"
              type="button"
              class="hero__chip"
              @click="useSuggestion(q)"
            >
              {{ q }}
            </button>
          </div>

          <div class="hero__stage">
            <svg class="hero__stage-lines" viewBox="0 0 1100 520" aria-hidden="true">
              <path d="M70 410 C 210 300, 330 420, 470 320 S 760 190, 1010 300" />
              <path d="M130 180 C 300 260, 380 120, 560 210 S 830 320, 980 180" />
            </svg>

            <div class="product">
              <aside class="product__side">
                <div class="product__brand">
                  <span>WellRead</span>
                </div>
                <div class="product__nav">
                  <span class="product__nav-row is-active"><ChatIcon /> {{ $t('menu.chat') }}</span>
                  <span class="product__nav-row"><FolderIcon /> {{ $t('menu.knowledgeBase') }}</span>
                  <span class="product__nav-row"><BookOpenIcon /> {{ $t('menu.reading') }}</span>
                  <span class="product__nav-row"><ChartIcon /> {{ $t('menu.review') }}</span>
                  <span class="product__nav-row"><StarIcon /> {{ $t('menu.skills') }}</span>
                </div>
                <div class="product__side-foot">
                  <LockOnIcon />
                  <span>{{ $t('welcomeHome.preview.localNote') }}</span>
                </div>
              </aside>

              <div class="product__main">
                <div class="product__head">
                  <span class="product__title">{{ $t('welcomeHome.preview.chatTitle') }}</span>
                  <span class="product__status">{{ $t('welcomeHome.preview.cite') }}</span>
                </div>
                <div class="product__chat">
                  <div class="product__bubble product__bubble--user">
                    {{ $t('welcomeHome.preview.question') }}
                  </div>
                  <div class="product__bubble product__bubble--ai">
                    <p>{{ $t('welcomeHome.preview.answer') }}</p>
                    <span class="product__cite"><LinkIcon /> {{ $t('welcomeHome.preview.cite') }}</span>
                  </div>
                  <div class="product__composer">
                    <span>{{ $t('welcomeHome.preview.composer') }}</span>
                    <SendIcon />
                  </div>
                </div>
              </div>
            </div>

            <div class="float-card float-card--kb">
              <span class="float-card__icon float-card__icon--green"><FolderIcon /></span>
              <div>
                <strong>{{ $t('welcomeHome.preview.kbTitle') }}</strong>
                <small>{{ $t('welcomeHome.preview.kbMeta') }}</small>
              </div>
              <span class="float-card__dots"><i class="on" /><i class="on" /><i class="on" /><i /></span>
            </div>

            <div class="float-card float-card--review">
              <span class="float-card__icon float-card__icon--amber"><ChartIcon /></span>
              <div>
                <strong>{{ $t('welcomeHome.preview.reviewTitle') }}</strong>
                <small>{{ $t('welcomeHome.preview.reviewMeta') }}</small>
              </div>
              <span class="float-card__heat">
                <i v-for="n in 12" :key="n" :class="'heat-' + ((n * 5) % 4)" />
              </span>
            </div>
          </div>
        </section>

        <section id="features" class="bento-section" data-reveal>
          <header class="section-head">
            <h2>{{ $t('welcomeHome.features.title') }}</h2>
          </header>

          <div class="bento">
            <article
              v-for="feature in features"
              :key="feature.key"
              class="bento__card"
              :class="`bento__card--${feature.key}`"
            >
              <div class="bento__head">
                <span class="bento__icon" :class="`bento__icon--${feature.tone}`">
                  <component :is="feature.icon" />
                </span>
                <span class="bento__tag">{{ feature.tag }}</span>
              </div>
              <h3>{{ feature.title }}</h3>
              <p class="bento__desc">{{ feature.desc }}</p>

              <div v-if="feature.key === 'ask'" class="bento-visual bento-visual--ask">
                <div class="mini-chat">
                  <span class="mini-chat__user">{{ $t('welcomeHome.preview.question') }}</span>
                  <span class="mini-chat__ai">
                    {{ $t('welcomeHome.preview.answer') }}
                    <em><LinkIcon /> {{ $t('welcomeHome.preview.cite') }}</em>
                  </span>
                </div>
              </div>

              <div v-else-if="feature.key === 'kb'" class="bento-visual bento-visual--kb">
                <span class="mini-file"><FileIcon /> paper.pdf <em>42p</em></span>
                <span class="mini-file"><FileIcon /> notes.md <em>18c</em></span>
                <span class="mini-file"><CodeIcon /> /src <em>96f</em></span>
                <span class="mini-index">
                  <i v-for="n in 10" :key="n" :class="{ on: n <= 7 }" />
                  <em>7 / 10</em>
                </span>
              </div>

              <div v-else-if="feature.key === 'review'" class="bento-visual bento-visual--review">
                <div class="mini-heat">
                  <i v-for="n in 28" :key="n" :class="'heat-' + ((n * 7) % 4)" />
                </div>
                <div class="mini-review-foot">
                  <span>1</span><span>2</span><span>3</span><span>4</span>
                </div>
              </div>

              <div v-else-if="feature.key === 'reading'" class="bento-visual bento-visual--reading">
                <span class="mini-chapter"><em>01</em><i style="width: 100%" /></span>
                <span class="mini-chapter"><em>02</em><i style="width: 82%" /></span>
                <span class="mini-chapter"><em>03</em><i style="width: 46%" /></span>
                <span class="mini-chapter-note">{{ feature.meta }}</span>
              </div>

              <div v-else class="bento-visual bento-visual--mcp">
                <span class="mini-code">GET /api/v1/sessions</span>
                <span class="mini-code mini-code--dim">authorization: Bearer ••••</span>
                <span class="mini-badge"><LockOnIcon /> {{ feature.meta }}</span>
              </div>
            </article>
          </div>
        </section>

        <section id="architecture" class="architecture" data-reveal>
          <header class="section-head">
            <h2>{{ $t('welcomeHome.arch.title') }}</h2>
          </header>

          <div class="architecture__switch" role="tablist" :aria-label="$t('welcomeHome.arch.title')">
            <button
              type="button"
              role="tab"
              class="architecture__tab"
              :class="{ 'is-active': activeDiagram === 'platform' }"
              :aria-selected="activeDiagram === 'platform'"
              @click="activeDiagram = 'platform'"
            >
              {{ $t('welcomeHome.arch.platform') }}
            </button>
            <button
              type="button"
              role="tab"
              class="architecture__tab"
              :class="{ 'is-active': activeDiagram === 'rag' }"
              :aria-selected="activeDiagram === 'rag'"
              @click="activeDiagram = 'rag'"
            >
              {{ $t('welcomeHome.arch.rag') }}
            </button>
          </div>

          <div class="architecture__frame">
            <iframe
              ref="diagramFrame"
              :key="`${activeDiagram}-${diagramTheme}`"
              :src="diagramSrc"
              :title="activeDiagram === 'platform' ? $t('welcomeHome.arch.platform') : $t('welcomeHome.arch.rag')"
              loading="lazy"
              allowtransparency="true"
              :style="{ height: `${diagramHeight}px` }"
              @load="handleDiagramLoad"
            />
          </div>
        </section>

        <section id="how" class="how-section" data-reveal>
          <header class="section-head">
            <h2>{{ $t('welcomeHome.how.title') }}</h2>
          </header>

          <div class="how">
            <svg class="how__line" viewBox="0 0 960 120" preserveAspectRatio="none" aria-hidden="true">
              <path d="M80 60 C 220 0, 300 120, 480 60 S 740 0, 880 60" />
            </svg>
            <article v-for="(step, index) in steps" :key="step.key" class="how__step">
              <span class="how__num">0{{ index + 1 }}</span>
              <span class="how__icon" :class="`how__icon--${step.tone}`"><component :is="step.icon" /></span>
              <h3>{{ step.title }}</h3>
              <p>{{ step.desc }}</p>
            </article>
          </div>
        </section>

        <section class="scenarios" data-reveal>
          <header class="section-head">
            <h2>{{ $t('welcomeHome.scenarios.title') }}</h2>
          </header>
          <div class="scenarios__grid">
            <button
              v-for="scenario in scenarios"
              :key="scenario.key"
              type="button"
              class="scenario"
              @click="go(scenario.path)"
            >
              <span class="scenario__icon" :class="`scenario__icon--${scenario.tone}`">
                <component :is="scenario.icon" />
              </span>
              <span class="scenario__copy">
                <strong>{{ scenario.title }}</strong>
                <small>{{ scenario.desc }}</small>
              </span>
              <span class="scenario__arrow"><ChevronRightIcon /></span>
            </button>
          </div>
        </section>

        <section class="final-cta" data-reveal>
          <span class="final-cta__brand">WellRead</span>
          <h2>{{ $t('welcomeHome.finalCta.title') }}</h2>
          <div class="final-cta__actions">
            <t-button class="landing-btn landing-btn--primary" theme="primary" size="large" @click="goChat">
              {{ $t('welcomeHome.finalCta.primary') }}
            </t-button>
            <t-button class="landing-btn" theme="default" variant="outline" size="large" @click="go('/platform/knowledge-bases')">
              {{ $t('welcomeHome.finalCta.secondary') }}
            </t-button>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  BookOpenIcon,
  ChartIcon,
  ChatIcon,
  ChevronRightIcon,
  CodeIcon,
  FileIcon,
  FolderIcon,
  LinkIcon,
  LockOnIcon,
  SendIcon,
  StarIcon,
} from 'tdesign-icons-vue-next'
import { useMenuStore } from '@/stores/menu'
import { useTheme } from '@/composables/useTheme'

const router = useRouter()
const menuStore = useMenuStore()
const { t } = useI18n()
const { currentTheme } = useTheme()

const pageEl = ref<HTMLElement | null>(null)
const scrollEl = ref<HTMLElement | null>(null)
const inputEl = ref<HTMLInputElement | null>(null)
const askQuery = ref('')
const activeDiagram = ref<'platform' | 'rag'>('platform')
const systemDark = ref(false)
const media = window.matchMedia('(prefers-color-scheme: dark)')
const updateSystemDark = () => { systemDark.value = media.matches }
updateSystemDark()
const diagramTheme = computed(() =>
  currentTheme.value === 'dark' || (currentTheme.value === 'system' && systemDark.value) ? 'dark' : 'light',
)
const diagramSrc = computed(
  () => `/diagrams/${activeDiagram.value}-architecture.html?embed=1&theme=${diagramTheme.value}`,
)
const diagramFrame = ref<HTMLIFrameElement | null>(null)
const diagramHeight = ref(620)
let diagramResizeObserver: ResizeObserver | null = null

function syncDiagramHeight() {
  const win = diagramFrame.value?.contentWindow
  const doc = win?.document
  if (!win || !doc) return
  const container = doc.querySelector<HTMLElement>('.container')
  const measured = Math.ceil(container?.getBoundingClientRect().height || doc.body.scrollHeight || 0)
  if (measured > 0) {
    diagramHeight.value = Math.max(560, measured + 16)
  }
}

function handleDiagramLoad() {
  syncDiagramHeight()
  window.setTimeout(syncDiagramHeight, 350)
  diagramResizeObserver?.disconnect()
  diagramResizeObserver = null
  const win = diagramFrame.value?.contentWindow
  const container = win?.document.querySelector<HTMLElement>('.container')
  const ResizeObserverCtor = (win as unknown as { ResizeObserver?: typeof ResizeObserver } | null)?.ResizeObserver
  if (win && container && ResizeObserverCtor) {
    diagramResizeObserver = new ResizeObserverCtor(syncDiagramHeight)
    diagramResizeObserver.observe(container)
  }
}

let revealObserver: IntersectionObserver | null = null
const reducedMotion = ref(false)

const suggestions = computed(() => [
  t('welcomeHome.suggestions.0'),
  t('welcomeHome.suggestions.1'),
  t('welcomeHome.suggestions.2'),
])
const features = computed(() => [
  {
    key: 'ask',
    tag: t('welcomeHome.features.askTag'),
    title: t('welcomeHome.features.askTitle'),
    desc: t('welcomeHome.features.askDesc'),
    tone: 'green',
    icon: ChatIcon,
  },
  {
    key: 'kb',
    tag: t('welcomeHome.features.kbTag'),
    title: t('welcomeHome.features.kbTitle'),
    desc: t('welcomeHome.features.kbDesc'),
    tone: 'blue',
    icon: FolderIcon,
  },
  {
    key: 'review',
    tag: t('welcomeHome.features.reviewTag'),
    title: t('welcomeHome.features.reviewTitle'),
    desc: t('welcomeHome.features.reviewDesc'),
    tone: 'amber',
    icon: ChartIcon,
  },
  {
    key: 'reading',
    tag: t('welcomeHome.features.readingTag'),
    title: t('welcomeHome.features.readingTitle'),
    desc: t('welcomeHome.features.readingDesc'),
    meta: t('welcomeHome.features.readingMeta'),
    tone: 'lilac',
    icon: BookOpenIcon,
  },
  {
    key: 'mcp',
    tag: t('welcomeHome.features.mcpTag'),
    title: t('welcomeHome.features.mcpTitle'),
    desc: t('welcomeHome.features.mcpDesc'),
    meta: t('welcomeHome.features.mcpMeta'),
    tone: 'ink',
    icon: LockOnIcon,
  },
])
const steps = computed(() => [
  { key: 'capture', title: t('welcomeHome.how.step1Title'), desc: t('welcomeHome.how.step1Desc'), tone: 'green', icon: FileIcon },
  { key: 'find', title: t('welcomeHome.how.step2Title'), desc: t('welcomeHome.how.step2Desc'), tone: 'blue', icon: ChatIcon },
  { key: 'remember', title: t('welcomeHome.how.step3Title'), desc: t('welcomeHome.how.step3Desc'), tone: 'amber', icon: ChartIcon },
])
const scenarios = computed(() => [
  { key: 'paper', title: t('welcomeHome.scenarios.paperTitle'), desc: t('welcomeHome.scenarios.paperDesc'), tone: 'green', icon: BookOpenIcon, path: '/platform/reading' },
  { key: 'code', title: t('welcomeHome.scenarios.codeTitle'), desc: t('welcomeHome.scenarios.codeDesc'), tone: 'blue', icon: CodeIcon, path: '/platform/knowledge-bases' },
  { key: 'learn', title: t('welcomeHome.scenarios.learnTitle'), desc: t('welcomeHome.scenarios.learnDesc'), tone: 'amber', icon: StarIcon, path: '/platform/review' },
])

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

function setupReveal() {
  reducedMotion.value = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const items = pageEl.value?.querySelectorAll<HTMLElement>('[data-reveal]')
  if (!items?.length) return
  if (reducedMotion.value || !scrollEl.value) {
    items.forEach((item) => item.classList.add('is-visible'))
    return
  }
  pageEl.value?.classList.add('reveal-ready')
  revealObserver = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          entry.target.classList.add('is-visible')
          revealObserver?.unobserve(entry.target)
        }
      })
    },
    { root: scrollEl.value, rootMargin: '0px 0px -8% 0px', threshold: 0.08 },
  )
  items.forEach((item) => revealObserver?.observe(item))
}

onMounted(() => {
  setupReveal()
  media.addEventListener('change', updateSystemDark)
})

onBeforeUnmount(() => {
  revealObserver?.disconnect()
  revealObserver = null
  media.removeEventListener('change', updateSystemDark)
  diagramResizeObserver?.disconnect()
  diagramResizeObserver = null
})
</script>

<style lang="less" scoped>
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

.landing {
  height: 100%;
  min-height: 0;
  position: relative;
  background: #fbfaf8;
  color: #191918;
  font-family: "DM Sans", var(--app-font-family);
  overflow: hidden;
}

.landing__scroll {
  position: relative;
  height: 100%;
  overflow-y: auto;
  overscroll-behavior: contain;
  scroll-behavior: smooth;
}

.landing__flow {
  position: absolute;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
  z-index: 0;

  svg {
    width: 100%;
    height: 1700px;
    display: block;
  }
}

.landing__flow-line {
  fill: none;
  stroke-width: 1.4;
  stroke-dasharray: 12 18;
  animation: landing-flow 16s linear infinite;
}

.landing__flow-line--slow {
  stroke-dasharray: 4 22;
  animation-duration: 24s;
  animation-direction: reverse;
}

.landing__flow-line--faint {
  opacity: 0.45;
  animation-duration: 30s;
}

@keyframes landing-flow {
  to {
    stroke-dashoffset: -300;
  }
}

.landing__inner {
  position: relative;
  z-index: 1;
  width: min(1160px, calc(100% - 96px));
  margin: 0 auto;
  padding: 76px 0 48px;
}

:global(.app-shell--fullscreen .landing__inner) {
  width: min(1600px, calc(100% - 48px));
}

:global(.app-shell--fullscreen .hero__stage) {
  width: min(1240px, 100%);
}

.hero {
  position: relative;
  text-align: center;
  padding: 24px 0 0;
}

.hero__title {
  max-width: 920px;
  margin: 0 auto;
  font-size: clamp(52px, 7vw, 92px);
  line-height: 0.98;
  letter-spacing: -0.055em;
  font-weight: 600;
  color: #171716;

  span {
    position: relative;
    white-space: nowrap;

    &::after {
      content: "";
      position: absolute;
      left: 1%;
      right: 1%;
      bottom: 0.09em;
      height: 0.13em;
      border-radius: 99px;
      background: rgba(7, 192, 95, 0.28);
      z-index: -1;
    }
  }
}

.hero__sub {
  max-width: 640px;
  margin: 26px auto 0;
  color: #6f6d66;
  font-size: 17px;
  line-height: 1.65;
}

.hero__actions {
  display: flex;
  justify-content: center;
  gap: 10px;
  margin-top: 30px;
}

.landing-btn {
  border-radius: 10px !important;
  font-weight: 500;
}

.landing-btn--primary {
  box-shadow: 0 10px 24px -12px rgba(7, 192, 95, 0.7);
}

.hero__ask {
  display: flex;
  align-items: center;
  gap: 8px;
  width: min(620px, 100%);
  margin: 30px auto 0;
  padding: 7px 8px 7px 16px;
  border: 1px solid #e4e1da;
  border-radius: 14px;
  background: #fff;
  box-shadow: 0 14px 40px -24px rgba(25, 25, 24, 0.28);
  transition: border-color 0.2s ease, box-shadow 0.2s ease;

  &:focus-within {
    border-color: rgba(7, 192, 95, 0.55);
    box-shadow: 0 16px 40px -24px rgba(7, 192, 95, 0.4);
  }
}

.hero__ask-icon {
  display: flex;
  color: #a5a39b;

  svg {
    width: 18px;
    height: 18px;
  }
}

.hero__ask-input {
  flex: 1;
  min-width: 0;
  border: 0;
  outline: none;
  background: transparent;
  color: #191918;
  font: inherit;
  font-size: 14px;

  &::placeholder {
    color: #a5a39b;
  }
}

.hero__ask-send {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border: 0;
  border-radius: 9px;
  background: #07c05f;
  color: #fff;
  cursor: pointer;
  transition: background 0.2s ease, transform 0.2s ease;

  &:hover {
    background: #06b04d;
    transform: translateY(-1px);
  }
}

.hero__chips {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
  margin-top: 14px;
}

.hero__chip {
  padding: 6px 11px;
  border: 1px solid #e9e6df;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.7);
  color: #77746c;
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
  transition: border-color 0.2s ease, color 0.2s ease, background 0.2s ease;

  &:hover {
    border-color: #cfccc3;
    color: #191918;
    background: #fff;
  }
}

.hero__stage {
  position: relative;
  width: min(1020px, 100%);
  margin: 58px auto 0;
  padding: 0 46px;
}

.hero__stage-lines {
  position: absolute;
  inset: -24px 0 0;
  width: 100%;
  height: 120%;
  pointer-events: none;
  overflow: visible;

  path {
    fill: none;
    stroke: #e5e3dc;
    stroke-width: 1;
    stroke-dasharray: 6 10;
    animation: landing-flow 22s linear infinite;
  }
}

.product {
  position: relative;
  display: flex;
  width: 100%;
  min-height: 370px;
  overflow: hidden;
  border: 1px solid #e7e4dd;
  border-radius: 22px;
  background: #fff;
  box-shadow:
    0 40px 90px -40px rgba(25, 25, 24, 0.35),
    0 2px 0 rgba(255, 255, 255, 0.8) inset;
  text-align: left;
}

.product__side {
  display: flex;
  flex-direction: column;
  width: 188px;
  flex-shrink: 0;
  padding: 18px 14px;
  border-right: 1px solid #efede7;
  background: #f8f7f4;
}

.product__brand {
  display: flex;
  align-items: center;
  color: #282722;
  font-family: "WellReadSerif", Georgia, serif;
  font-size: 17px;
  font-weight: 400;
  line-height: 1;
  padding: 0 8px 16px;
}

.product__nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.product__nav-row {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 8px 9px;
  border-radius: 7px;
  color: #76736b;
  font-size: 12px;

  svg {
    width: 15px;
    height: 15px;
  }

  &.is-active {
    background: #e9f8ec;
    color: #067a3d;
    font-weight: 600;
  }
}

.product__side-foot {
  display: flex;
  align-items: center;
  gap: 7px;
  margin-top: auto;
  padding: 10px 9px 0;
  border-top: 1px solid #ece9e2;
  color: #9a978f;
  font-size: 11px;

  svg {
    width: 13px;
    height: 13px;
  }
}

.product__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.product__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 15px 20px;
  border-bottom: 1px solid #f0eee9;
  font-size: 12px;
}

.product__title {
  font-weight: 600;
  color: #33322d;
}

.product__status {
  color: #a5a29a;
  font-size: 11px;
}

.product__chat {
  flex: 1;
  padding: 22px 22px 18px;
  background:
    radial-gradient(circle at 80% 0%, rgba(233, 248, 236, 0.7), transparent 46%),
    #fff;
}

.product__bubble {
  max-width: 82%;
  padding: 11px 14px;
  border-radius: 12px;
  font-size: 12.5px;
  line-height: 1.6;
}

.product__bubble--user {
  margin-left: auto;
  background: #f1efe9;
  color: #4f4d46;
}

.product__bubble--ai {
  margin-top: 12px;
  border: 1px solid #ebe8e1;
  background: #fff;
  color: #4f4d46;
  box-shadow: 0 8px 24px -20px rgba(25, 25, 24, 0.5);

  p {
    margin: 0;
  }
}

.product__cite {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  margin-top: 9px;
  color: #078347;
  font-size: 11px;

  svg {
    width: 12px;
    height: 12px;
  }
}

.product__composer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 18px;
  padding: 10px 13px;
  border: 1px solid #e9e6df;
  border-radius: 10px;
  color: #aaa79f;
  font-size: 12px;

  svg {
    width: 15px;
    height: 15px;
    color: #07c05f;
  }
}

.float-card {
  position: absolute;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 13px;
  border: 1px solid #e8e5de;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 18px 40px -26px rgba(25, 25, 24, 0.45);
  backdrop-filter: blur(10px);
  animation: float-card 7s ease-in-out infinite;

  strong,
  small {
    display: block;
  }

  strong {
    font-size: 11.5px;
    color: #33322d;
  }

  small {
    margin-top: 2px;
    color: #9d9a92;
    font-size: 10px;
  }
}

.float-card--kb {
  top: 42px;
  right: -6px;
}

.float-card--review {
  left: -6px;
  bottom: 34px;
  animation-delay: -3.4s;
}

.float-card__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 9px;

  svg {
    width: 15px;
    height: 15px;
  }
}

.float-card__icon--green {
  background: #e9f8ec;
  color: #067a3d;
}

.float-card__icon--amber {
  background: #fff4e2;
  color: #a26b0b;
}

.float-card__dots {
  display: flex;
  gap: 3px;
  margin-left: 2px;

  i {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: #e3e0d9;

    &.on {
      background: #07c05f;
    }
  }
}

.float-card__heat {
  display: grid;
  grid-template-columns: repeat(4, 5px);
  gap: 3px;
  margin-left: 2px;

  i {
    width: 5px;
    height: 5px;
    border-radius: 1.5px;
    background: #eae7e0;
  }

  .heat-1 {
    background: #d5f2df;
  }

  .heat-2 {
    background: #93dcb2;
  }

  .heat-3 {
    background: #f0c98a;
  }
}

@keyframes float-card {
  0%,
  100% {
    transform: translateY(0);
  }

  50% {
    transform: translateY(-7px);
  }
}

.section-head {
  max-width: 760px;
  margin: 0 auto;
  text-align: center;

  h2 {
    margin: 0;
    font-size: clamp(32px, 4vw, 48px);
    line-height: 1.08;
    letter-spacing: -0.045em;
    font-weight: 600;
    color: #191918;
  }
}

.bento-section,
.how-section,
.scenarios,
.architecture {
  padding-top: 118px;
}

.architecture__switch {
  display: flex;
  justify-content: center;
  gap: 6px;
  margin-top: 32px;
}

.architecture__tab {
  padding: 8px 16px;
  border: 1px solid #e7e3da;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.7);
  color: #77746c;
  font-family: inherit;
  font-size: 12.5px;
  cursor: pointer;
  transition: background 0.2s ease, color 0.2s ease, border-color 0.2s ease;

  &:hover {
    border-color: #cfcac0;
    color: #191918;
  }

  &.is-active {
    border-color: #191918;
    background: #191918;
    color: #fff;
  }
}

.architecture__frame {
  min-height: 620px;
  margin-top: 18px;
  padding: 4px 0;
  border-top: 1px solid #ece9e2;
  border-bottom: 1px solid #ece9e2;
  background: transparent;

  iframe {
    display: block;
    width: 100%;
    height: 620px;
    border: 0;
    background: transparent;
    transition: height 0.2s ease;
  }
}


.bento {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 16px;
  margin-top: 46px;
}

.bento__card {
  position: relative;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 24px;
  border: 1px solid #e9e6df;
  border-radius: 20px;
  background: #fff;
  transition: transform 0.25s ease, border-color 0.25s ease, box-shadow 0.25s ease;

  &:hover {
    transform: translateY(-3px);
    border-color: #ddd9d1;
    box-shadow: 0 24px 50px -42px rgba(25, 25, 24, 0.5);
  }

  h3 {
    margin: 16px 0 8px;
    font-size: 20px;
    line-height: 1.2;
    letter-spacing: -0.025em;
    color: #22211d;
  }
}

.bento__card--ask {
  grid-column: span 4;
}

.bento__card--kb {
  grid-column: span 2;
}

.bento__card--review {
  grid-column: span 2;
}

.bento__card--reading {
  grid-column: span 2;
}

.bento__card--mcp {
  grid-column: span 2;
}

.bento__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.bento__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border-radius: 10px;

  svg {
    width: 18px;
    height: 18px;
  }
}

.bento__icon--green,
.how__icon--green,
.scenario__icon--green {
  background: #e9f8ec;
  color: #067a3d;
}

.bento__icon--blue,
.how__icon--blue,
.scenario__icon--blue {
  background: #eaf1ff;
  color: #3867b7;
}

.bento__icon--amber,
.how__icon--amber,
.scenario__icon--amber {
  background: #fff4e2;
  color: #a26b0b;
}

.bento__icon--lilac,
.scenario__icon--lilac {
  background: #f2ecff;
  color: #7650b8;
}

.bento__icon--ink {
  background: #f0efec;
  color: #4c4a44;
}

.bento__tag {
  color: #9a978f;
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.bento__desc {
  margin: 0;
  color: #77746c;
  font-size: 13.5px;
  line-height: 1.6;
}

.bento-visual {
  margin-top: 22px;
}

.bento-visual--ask {
  padding: 16px;
  border-radius: 14px;
  background: #f8f7f4;
}

.mini-chat {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.mini-chat__user {
  align-self: flex-end;
  max-width: 78%;
  padding: 8px 11px;
  border-radius: 9px;
  background: #edeae3;
  color: #55534c;
  font-size: 11.5px;
}

.mini-chat__ai {
  max-width: 88%;
  padding: 9px 11px;
  border: 1px solid #ebe7df;
  border-radius: 9px;
  background: #fff;
  color: #5d5a52;
  font-size: 11.5px;
  line-height: 1.55;

  em {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    margin-top: 7px;
    color: #078347;
    font-size: 10.5px;
    font-style: normal;

    svg {
      width: 11px;
      height: 11px;
    }
  }
}

.bento-visual--kb {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.mini-file {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 7px 9px;
  border: 1px solid #efede8;
  border-radius: 8px;
  background: #fbfaf8;
  color: #55534c;
  font-size: 11px;

  svg {
    width: 13px;
    height: 13px;
    color: #8d8a82;
  }

  em {
    margin-left: auto;
    color: #a5a29a;
    font-style: normal;
  }
}

.mini-index {
  display: flex;
  align-items: center;
  gap: 3px;
  margin-top: 6px;

  i {
    width: 5px;
    height: 12px;
    border-radius: 2px;
    background: #eae7e0;

    &.on {
      background: #07c05f;
    }
  }

  em {
    margin-left: auto;
    color: #9d9a92;
    font-size: 10px;
    font-style: normal;
  }
}

.mini-heat {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 4px;

  i {
    aspect-ratio: 1;
    border-radius: 3px;
    background: #eeeae3;
  }

  .heat-1 {
    background: #d5f2df;
  }

  .heat-2 {
    background: #8fdbb0;
  }

  .heat-3 {
    background: #f1c985;
  }
}

.mini-review-foot {
  display: flex;
  justify-content: space-between;
  margin-top: 10px;

  span {
    flex: 1;
    padding: 5px 0;
    border: 1px solid #eeeae3;
    border-radius: 6px;
    color: #9d9a92;
    font-size: 10px;
    text-align: center;

    &:not(:last-child) {
      margin-right: 4px;
    }
  }
}

.mini-chapter {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;

  em {
    width: 18px;
    color: #a5a29a;
    font-size: 10px;
    font-style: normal;
  }

  i {
    display: block;
    height: 6px;
    border-radius: 99px;
    background: #d8d4ca;
  }

  &:nth-child(3) i {
    background: #b9a7e6;
  }
}

.mini-chapter-note {
  display: block;
  margin-top: 8px;
  color: #9d9a92;
  font-size: 10.5px;
}

.bento-visual--mcp {
  display: flex;
  flex-direction: column;
  gap: 5px;
  padding: 12px;
  border-radius: 10px;
  background: #191918;
}

.mini-code {
  color: #d8f5e5;
  font-family: var(--app-font-family-mono);
  font-size: 10.5px;

  &--dim {
    color: #777b78;
  }
}

.mini-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-top: 6px;
  color: #9fdcbc;
  font-size: 10px;

  svg {
    width: 11px;
    height: 11px;
  }
}

.how {
  position: relative;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 56px;
  margin-top: 58px;
}

.how__line {
  position: absolute;
  top: 30px;
  left: 8%;
  width: 84%;
  height: 90px;
  pointer-events: none;

  path {
    fill: none;
    stroke: #e3e0d8;
    stroke-width: 1;
    stroke-dasharray: 5 9;
    animation: landing-flow 18s linear infinite;
  }
}

.how__step {
  position: relative;
  z-index: 1;
  text-align: center;

  h3 {
    margin: 16px 0 8px;
    font-size: 19px;
    letter-spacing: -0.02em;
    color: #22211d;
  }

  p {
    max-width: 290px;
    margin: 0 auto;
    color: #77746c;
    font-size: 13.5px;
    line-height: 1.65;
  }
}

.how__num {
  display: block;
  color: #b9b5ac;
  font-size: 11px;
  letter-spacing: 0.1em;
}

.how__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 46px;
  height: 46px;
  margin-top: 14px;
  border-radius: 14px;

  svg {
    width: 22px;
    height: 22px;
  }
}

.scenarios__grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 14px;
  margin-top: 44px;
}

.scenario {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 18px;
  border: 1px solid #e9e6df;
  border-radius: 16px;
  background: #fff;
  text-align: left;
  font-family: inherit;
  cursor: pointer;
  transition: transform 0.22s ease, border-color 0.22s ease;

  &:hover {
    transform: translateY(-2px);
    border-color: #d8d4cb;
  }
}

.scenario__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  flex-shrink: 0;
  border-radius: 11px;

  svg {
    width: 19px;
    height: 19px;
  }
}

.scenario__copy {
  flex: 1;
  min-width: 0;

  strong {
    display: block;
    color: #2b2a25;
    font-size: 14px;
  }

  small {
    display: block;
    margin-top: 4px;
    color: #85827a;
    font-size: 12px;
    line-height: 1.5;
  }
}

.scenario__arrow {
  display: flex;
  color: #b7b3aa;
}

.final-cta {
  padding: 126px 0 108px;
  text-align: center;

  h2 {
    margin: 18px 0 0;
    font-size: clamp(36px, 4.6vw, 56px);
    line-height: 1.05;
    letter-spacing: -0.05em;
    font-weight: 600;
    color: #171716;
  }
}

.final-cta__brand {
  color: #171716;
  font-family: "WellReadSerif", Georgia, serif;
  font-size: 32px;
  font-weight: 400;
  line-height: 1;
  letter-spacing: 0;
}

.final-cta__actions {
  display: flex;
  justify-content: center;
  gap: 10px;
  margin-top: 28px;
}

.reveal-ready [data-reveal] {
  opacity: 0;
  transform: translateY(18px);
  transition: opacity 0.55s ease, transform 0.55s ease;
}

.reveal-ready [data-reveal].is-visible {
  opacity: 1;
  transform: translateY(0);
}

/* Dark theme keeps the landing readable while preserving the light-paper
   structure of the page. */
:global(html[theme-mode="dark"] .landing) {
  background: #131413;
  color: #f1f0ea;
}

:global(html[theme-mode="dark"] .hero__chip),
:global(html[theme-mode="dark"] .hero__ask) {
  border-color: #343530;
  background: #1c1d1a;
  color: #c9c7bf;
}

:global(html[theme-mode="dark"] .hero__title),
:global(html[theme-mode="dark"] .section-head h2),
:global(html[theme-mode="dark"] .final-cta h2),
:global(html[theme-mode="dark"] .final-cta__brand),
:global(html[theme-mode="dark"] .bento__card h3),
:global(html[theme-mode="dark"] .how__step h3),
:global(html[theme-mode="dark"] .scenario__copy strong) {
  color: #f1f0ea;
}

:global(html[theme-mode="dark"] .hero__sub),
:global(html[theme-mode="dark"] .bento__desc),
:global(html[theme-mode="dark"] .how__step p),
:global(html[theme-mode="dark"] .scenario__copy small) {
  color: #a3a198;
}

:global(html[theme-mode="dark"] .hero__ask-input) {
  color: #f1f0ea;
}

:global(html[theme-mode="dark"] .hero__ask-input::placeholder) {
  color: #78776f;
}

:global(html[theme-mode="dark"] .hero__chip:hover) {
  border-color: #55564f;
  color: #f1f0ea;
  background: #242520;
}

:global(html[theme-mode="dark"] .product) {
  border-color: #2e2f2a;
  background: #1c1d1a;
  box-shadow: 0 40px 90px -40px rgba(0, 0, 0, 0.8);
}

:global(html[theme-mode="dark"] .product__side) {
  border-color: #2c2d28;
  background: #171815;
}

:global(html[theme-mode="dark"] .product__brand),
:global(html[theme-mode="dark"] .product__title) {
  color: #e6e4dc;
}

:global(html[theme-mode="dark"] .product__nav-row) {
  color: #99978f;
}

:global(html[theme-mode="dark"] .product__nav-row.is-active) {
  background: #123a25;
  color: #7de2a9;
}

:global(html[theme-mode="dark"] .product__side-foot),
:global(html[theme-mode="dark"] .product__head) {
  border-color: #2c2d28;
}

:global(html[theme-mode="dark"] .product__chat) {
  background: #1c1d1a;
}

:global(html[theme-mode="dark"] .product__bubble--user) {
  background: #2a2b27;
  color: #c9c7bf;
}

:global(html[theme-mode="dark"] .product__bubble--ai) {
  border-color: #33342f;
  background: #22231f;
  color: #c9c7bf;
}

:global(html[theme-mode="dark"] .product__composer) {
  border-color: #33342f;
  color: #7c7b73;
}

:global(html[theme-mode="dark"] .float-card) {
  border-color: #33342f;
  background: rgba(30, 31, 27, 0.92);
}

:global(html[theme-mode="dark"] .float-card strong) {
  color: #e6e4dc;
}

:global(html[theme-mode="dark"] .float-card small) {
  color: #918f87;
}

:global(html[theme-mode="dark"] .bento__card),
:global(html[theme-mode="dark"] .scenario) {
  border-color: #2e2f2a;
  background: #1b1c19;
}

:global(html[theme-mode="dark"] .bento__card:hover),
:global(html[theme-mode="dark"] .scenario:hover) {
  border-color: #44453f;
}

:global(html[theme-mode="dark"] .bento-visual--ask) {
  background: #22231f;
}

:global(html[theme-mode="dark"] .mini-chat__user) {
  background: #2a2b27;
  color: #c9c7bf;
}

:global(html[theme-mode="dark"] .mini-chat__ai),
:global(html[theme-mode="dark"] .mini-file) {
  border-color: #33342f;
  background: #1c1d1a;
  color: #c9c7bf;
}

:global(html[theme-mode="dark"] .mini-file em),
:global(html[theme-mode="dark"] .mini-chapter em),
:global(html[theme-mode="dark"] .mini-chapter-note),
:global(html[theme-mode="dark"] .bento__tag) {
  color: #8f8d85;
}

:global(html[theme-mode="dark"] .mini-heat i),
:global(html[theme-mode="dark"] .mini-review-foot span),
:global(html[theme-mode="dark"] .mini-index i) {
  border-color: #33342f;
  background: #2a2b27;
}

:global(html[theme-mode="dark"] .mini-chapter i) {
  background: #3b3c35;
}

:global(html[theme-mode="dark"] .mini-review-foot span) {
  color: #8f8d85;
}

:global(html[theme-mode="dark"] .architecture__tab) {
  border-color: #343530;
  background: #1c1d1a;
  color: #a3a198;
}

:global(html[theme-mode="dark"] .architecture__tab.is-active) {
  border-color: #e9e4da;
  background: #e9e4da;
  color: #171816;
}

:global(html[theme-mode="dark"] .architecture__frame) {
  border-color: #2c2d28;
}

@media (max-width: 1100px) {
  .landing__inner {
    width: calc(100% - 56px);
  }

  .hero__stage {
    padding: 0 12px;
  }

  .float-card--kb {
    right: 0;
  }

  .float-card--review {
    left: 0;
  }

  .bento {
    grid-template-columns: repeat(4, 1fr);
  }

  .bento__card--ask {
    grid-column: span 4;
  }

  .bento__card--kb,
  .bento__card--review,
  .bento__card--reading,
  .bento__card--mcp {
    grid-column: span 2;
  }
}

@media (max-width: 860px) {
  .landing__inner {
    width: calc(100% - 36px);
    padding-top: 52px;
  }

  .hero__title {
    font-size: clamp(42px, 11vw, 64px);
  }

  .hero__sub {
    font-size: 15px;
  }

  .product__side {
    display: none;
  }

  .hero__stage {
    padding: 0;
  }

  .float-card {
    display: none;
  }

  .bento {
    grid-template-columns: 1fr;
  }

  .bento__card--ask,
  .bento__card--kb,
  .bento__card--review,
  .bento__card--reading,
  .bento__card--mcp {
    grid-column: span 1;
  }

  .how {
    grid-template-columns: 1fr;
    gap: 30px;
  }

  .how__line {
    display: none;
  }

  .scenarios__grid {
    grid-template-columns: 1fr;
  }

  .bento-section,
  .how-section,
  .scenarios,
  .architecture {
    padding-top: 84px;
  }

  .architecture__frame {
    min-height: 480px;
  }

  .final-cta {
    padding: 90px 0 72px;
  }
}

@media (max-width: 600px) {
  .hero__actions,
  .final-cta__actions {
    flex-direction: column;
    align-items: stretch;
  }

  .hero__ask {
    border-radius: 12px;
  }

  .hero__chips {
    justify-content: flex-start;
  }

  .product {
    min-height: 330px;
  }

  .product__chat {
    padding: 16px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .landing__flow-line,
  .hero__stage-lines path,
  .how__line path,
  .float-card {
    animation: none !important;
  }

  .reveal-ready [data-reveal] {
    opacity: 1;
    transform: none;
    transition: none;
  }

  .landing__scroll {
    scroll-behavior: auto;
  }
}
</style>
