<template>
  <div
    ref="dropzone"
    class="app-shell"
    :class="{ 'app-shell--detail': isDetailRoute, 'app-shell--fullscreen': isDetailRoute && fullscreen }"
  >
    <div
      class="app-shell__bg"
      aria-hidden="true"
      :style="{ backgroundImage: `url(${grassBackground})` }"
    />

    <header class="app-topbar">
      <div class="app-topbar__inner">
        <div class="app-topbar__left">
          <button
            type="button"
            class="app-topbar__menu"
            aria-label="打开导航"
            @click="mobileNavVisible = true"
          >
            <svg viewBox="0 0 20 20" width="18" height="18" fill="none" aria-hidden="true">
              <path d="M3 5h14M3 10h14M3 15h14" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
            </svg>
          </button>
          <button type="button" class="app-brand" aria-label="WellRead 首页" @click="goHome">
            <span class="app-brand__name">WellRead</span>
          </button>
        </div>
        <div class="app-topbar__right">
          <t-tooltip v-if="isDetailRoute" :content="fullscreen ? '退出全屏' : '全屏'" placement="bottom">
            <button
              type="button"
              class="app-fullscreen-btn"
              :aria-label="fullscreen ? '退出全屏' : '全屏'"
              @click="toggleFullscreen"
            >
              <svg v-if="!fullscreen" viewBox="0 0 20 20" width="18" height="18" fill="none" aria-hidden="true">
                <rect x="3.5" y="3.5" width="13" height="13" rx="2.5" stroke="currentColor" stroke-width="1.5" />
              </svg>
              <svg v-else viewBox="0 0 20 20" width="18" height="18" fill="none" aria-hidden="true">
                <path d="M5 5l10 10M15 5L5 15" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
              </svg>
            </button>
          </t-tooltip>
          <t-tooltip content="个人设置" placement="bottom">
            <button type="button" class="app-profile-btn" aria-label="个人设置" @click="goSettings">
              <img v-if="profileAvatar" class="app-profile-btn__img" :src="profileAvatar" alt="" />
              <span v-else class="app-profile-btn__initial">{{ profileInitial }}</span>
            </button>
          </t-tooltip>
        </div>
      </div>
    </header>

    <aside class="app-rail app-rail--left" aria-label="功能区">
      <ul class="app-rail__list">
        <li v-for="item in primaryNav" :key="item.key" class="app-rail__cell">
          <button
            type="button"
            class="app-rail__item"
            :class="{ 'is-active': isPrimaryActive(item) }"
            @click="handlePrimary(item)"
          >
            <span class="app-rail__icon"><component :is="item.icon" /></span>
            <span class="app-rail__label">{{ item.label }}</span>
          </button>
        </li>
      </ul>
    </aside>

    <aside class="app-rail app-rail--right" aria-label="辅助入口">
      <ul class="app-rail__list">
        <li v-for="item in rightRail" :key="item.key" class="app-rail__cell">
          <button type="button" class="app-rail__item" @click="item.action">
            <span class="app-rail__icon"><component :is="item.icon" /></span>
            <span class="app-rail__label">{{ item.label }}</span>
          </button>
        </li>
      </ul>
    </aside>

    <main class="app-stage">
      <section class="app-window" aria-label="WellRead 工作区">
        <div ref="contentEl" class="app-window__content">
          <RouterView v-if="isRouterAlive" />
        </div>
      </section>
    </main>

    <SessionDrawer v-model:visible="sessionDrawerVisible" />

    <Transition name="mobile-nav">
      <div v-if="mobileNavVisible" class="app-mobile-nav" role="dialog" aria-label="导航">
        <div class="app-mobile-nav__head">
          <span>导航</span>
          <button type="button" aria-label="关闭" @click="mobileNavVisible = false">
            <CloseIcon />
          </button>
        </div>
        <nav class="app-mobile-nav__list">
          <button
            v-for="item in primaryNav"
            :key="item.key"
            type="button"
            :class="{ 'is-active': isPrimaryActive(item) }"
            @click="handleMobilePrimary(item)"
          >
            <component :is="item.icon" />
            <span>{{ item.label }}</span>
          </button>
        </nav>
        <button type="button" class="app-mobile-nav__logout" @click="handleLogout">
          <LogoutIcon />
          <span>退出登录</span>
        </button>
      </div>
    </Transition>

    <div v-show="ismask" class="upload-mask">
      <UploadMask />
    </div>
    <!-- 全局设置模态框：设置路由自身已挂载一份，避免重复 -->
    <Settings v-if="route.path !== '/platform/settings'" />
    <GlobalCommandPalette />
    <AppHelpDialog v-model:visible="helpVisible" />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, provide, ref, watch, type Component } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  ArrowUpIcon,
  BookOpenIcon,
  ChartIcon,
  ChatIcon,
  CloseIcon,
  FolderIcon,
  HelpCircleIcon,
  HomeIcon,
  LogoutIcon,
  LogoGithubIcon,
  MoonIcon,
  SettingIcon,
  StarIcon,
  SunnyIcon,
} from 'tdesign-icons-vue-next'
import AppHelpDialog from '@/components/layout/AppHelpDialog.vue'
import SessionDrawer from '@/components/layout/SessionDrawer.vue'
import { collectDroppedFiles } from '@/components/layout/collectDroppedFiles'
import UploadMask from '@/components/upload-mask.vue'
import Settings from '@/views/settings/Settings.vue'
import GlobalCommandPalette from '@/components/GlobalCommandPalette.vue'
import { useCommandPaletteStore } from '@/stores/commandPalette'
import { useChatResourcesStore } from '@/stores/chatResources'
import { useAuthStore } from '@/stores/auth'
import { useTheme } from '@/composables/useTheme'
import { getKnowledgeBaseById } from '@/api/knowledge-base/index'
import { logout as logoutApi } from '@/api/auth'
import { useI18n } from 'vue-i18n'
import grassBackground from '@/assets/img/app-bg/grass.jpg'

type PrimaryNavItem = { key: string; label: string; icon: Component; path?: string }

const route = useRoute()
const router = useRouter()
const isDetailRoute = computed(() => route.path.startsWith('/platform'))
const { t } = useI18n()
const commandPaletteStore = useCommandPaletteStore()
const authStore = useAuthStore()
const { currentTheme, setTheme } = useTheme()

const contentEl = ref<HTMLElement | null>(null)
const helpVisible = ref(false)
const sessionDrawerVisible = ref(false)
const mobileNavVisible = ref(false)
const fullscreen = ref(false)
const ismask = ref(false)
const isRouterAlive = ref(true)
const systemDark = ref(false)

const media = window.matchMedia('(prefers-color-scheme: dark)')
const updateSystemDark = () => {
  systemDark.value = media.matches
}
updateSystemDark()

const effectiveDark = computed(
  () => currentTheme.value === 'dark' || (currentTheme.value === 'system' && systemDark.value),
)

const profileAvatar = computed(() => authStore.user?.avatar || '')
const profileInitial = computed(() => {
  const source = authStore.user?.username || authStore.user?.email || 'W'
  return source.trim().slice(0, 1).toUpperCase()
})

const primaryNav = computed<PrimaryNavItem[]>(() => [
  { key: 'home', label: t('menu.welcome'), icon: HomeIcon, path: '/' },
  { key: 'chat', label: t('menu.chat'), icon: ChatIcon },
  { key: 'knowledge', label: t('menu.knowledgeBase'), icon: FolderIcon, path: '/platform/knowledge-bases' },
  { key: 'reading', label: t('menu.reading'), icon: BookOpenIcon, path: '/platform/reading' },
  { key: 'review', label: t('menu.review'), icon: ChartIcon, path: '/platform/review' },
  { key: 'skills', label: t('menu.skills'), icon: StarIcon, path: '/platform/skills' },
])

const rightRail = computed(() => [
  {
    key: 'help',
    label: '帮助',
    icon: HelpCircleIcon,
    action: () => {
      helpVisible.value = true
    },
  },
  {
    key: 'settings',
    label: '设置',
    icon: SettingIcon,
    action: goSettings,
  },
  {
    key: 'theme',
    label: effectiveDark.value ? '浅色' : '深色',
    icon: effectiveDark.value ? SunnyIcon : MoonIcon,
    action: toggleTheme,
  },
  {
    key: 'github',
    label: 'GitHub',
    icon: LogoGithubIcon,
    action: openGithub,
  },
  {
    key: 'top',
    label: '回到顶部',
    icon: ArrowUpIcon,
    action: scrollToTop,
  },
  {
    key: 'logout',
    label: '退出登录',
    icon: LogoutIcon,
    action: handleLogout,
  },
])

function toggleTheme() {
  setTheme(effectiveDark.value ? 'light' : 'dark')
}

function toggleFullscreen() {
  fullscreen.value = !fullscreen.value
  if (fullscreen.value) {
    sessionDrawerVisible.value = false
    mobileNavVisible.value = false
  }
}

function goHome() {
  router.push('/')
  mobileNavVisible.value = false
}

function goSettings() {
  router.push('/platform/settings')
  mobileNavVisible.value = false
}

function openGithub() {
  window.open('https://github.com/Forward-back', '_blank', 'noopener,noreferrer')
}

function handlePrimary(item: PrimaryNavItem) {
  if (item.key === 'chat') {
    sessionDrawerVisible.value = !sessionDrawerVisible.value
    return
  }
  if (item.path) {
    sessionDrawerVisible.value = false
    router.push(item.path)
  }
}

function handleMobilePrimary(item: PrimaryNavItem) {
  mobileNavVisible.value = false
  if (item.key === 'chat') {
    sessionDrawerVisible.value = true
    return
  }
  if (item.path) {
    sessionDrawerVisible.value = false
    router.push(item.path)
  }
}

const isChatRoute = computed(() => {
  const name = String(route.name || '')
  return (
    name === 'chat' ||
    name === 'globalCreatChat' ||
    name === 'kbCreatChat' ||
    route.path === '/platform/creatChat' ||
    route.path.startsWith('/platform/chat/')
  )
})

function isPrimaryActive(item: PrimaryNavItem) {
  if (item.key === 'home') return route.path === '/'
  if (item.key === 'chat') return isChatRoute.value
  if (item.key === 'knowledge') return route.path.startsWith('/platform/knowledge-bases')
  if (item.key === 'reading') return route.path.startsWith('/platform/reading')
  if (item.key === 'review') return route.path.startsWith('/platform/review')
  if (item.key === 'skills') return route.path.startsWith('/platform/skills')
  return false
}

function scrollToTop() {
  const root = contentEl.value
  if (!root) return
  const landingScroller = root.querySelector<HTMLElement>('.welcome-home__scroll')
  if (landingScroller) {
    landingScroller.scrollTo({ top: 0, behavior: 'smooth' })
    return
  }
  const scroller = Array.from(root.querySelectorAll<HTMLElement>('*')).find(el => {
    const style = getComputedStyle(el)
    return (style.overflowY === 'auto' || style.overflowY === 'scroll') && el.scrollHeight > el.clientHeight + 2
  })
  scroller?.scrollTo({ top: 0, behavior: 'smooth' })
}

async function handleLogout() {
  sessionDrawerVisible.value = false
  mobileNavVisible.value = false
  try {
    await logoutApi()
  } catch {
    /* local logout still proceeds */
  }
  authStore.logout()
  router.push('/login')
}

const reloadApp = () => {
  isRouterAlive.value = false
  nextTick(() => {
    isRouterAlive.value = true
  })
}
provide('app:reload', reloadApp)

// Wails desktop only: soft reload instead of a full page reload.
// @ts-ignore
const isWailsDesktop = typeof window !== 'undefined' && !!(window as any).runtime?.EventsOn

const handleGlobalKeyDown = (e: KeyboardEvent) => {
  if (!isWailsDesktop) return
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'r') {
    e.preventDefault()
    reloadApp()
  }
}

let dragCounter = 0

const getCurrentKbId = (): string | null => {
  return (route.params as any)?.kbId as string || null
}

const CHAT_DROP_ROUTE_NAMES = new Set(['chat', 'globalCreatChat', 'kbCreatChat'])
const isChatDropRoute = () => CHAT_DROP_ROUTE_NAMES.has(String(route.name || ''))

const checkKnowledgeBaseInitialization = async (): Promise<boolean> => {
  const currentKbId = getCurrentKbId()
  if (!currentKbId) {
    MessagePlugin.error(t('knowledgeBase.missingId'))
    return false
  }
  try {
    const kbResponse = await getKnowledgeBaseById(currentKbId)
    const kb = kbResponse.data
    if (!kb.summary_model_id) {
      MessagePlugin.warning(t('knowledgeBase.notInitialized'))
      return false
    }
    const strategy = kb.indexing_strategy
    const needsEmbedding = !strategy || strategy.vector_enabled || strategy.keyword_enabled
    if (needsEmbedding && !kb.embedding_model_id) {
      MessagePlugin.warning(t('knowledgeBase.notInitialized'))
      return false
    }
    return true
  } catch {
    MessagePlugin.error(t('knowledgeBase.getInfoFailed'))
    return false
  }
}

const isFileDrag = (event: DragEvent): boolean => {
  const types = event.dataTransfer?.types
  if (!types) return false
  return Array.from(types).includes('Files')
}

const handleGlobalDragEnter = (event: DragEvent) => {
  if (!isFileDrag(event)) return
  event.preventDefault()
  dragCounter++
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'all'
  ismask.value = true
}

const handleGlobalDragOver = (event: DragEvent) => {
  if (!isFileDrag(event)) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
}

const handleGlobalDragLeave = (event: DragEvent) => {
  if (!isFileDrag(event)) return
  event.preventDefault()
  dragCounter--
  if (dragCounter === 0) ismask.value = false
}

const handleGlobalDrop = async (event: DragEvent) => {
  if (!isFileDrag(event)) return
  event.preventDefault()
  dragCounter = 0
  ismask.value = false
  const droppedFiles = await collectDroppedFiles(event)
  if (droppedFiles.length === 0) {
    MessagePlugin.warning(t('knowledgeBase.dragFileNotText'))
    return
  }
  if (isChatDropRoute()) {
    event.stopPropagation()
    window.dispatchEvent(new CustomEvent('gaze:chat-file-drop', { detail: { files: droppedFiles } }))
    return
  }
  const isInitialized = await checkKnowledgeBaseInitialization()
  if (!isInitialized) return
  window.dispatchEvent(new CustomEvent('gaze:knowledge-file-drop', {
    detail: { kbId: getCurrentKbId(), files: droppedFiles },
  }))
}

function maybeOpenCmdkFromRoute() {
  if (!('cmdk' in route.query)) return
  const q = String(route.query.cmdk ?? '')
  commandPaletteStore.openPalette(q)
  const newQuery = { ...route.query }
  delete (newQuery as any).cmdk
  router.replace({ path: route.path, query: newQuery, hash: route.hash })
}

watch(() => route.query.cmdk, () => {
  maybeOpenCmdkFromRoute()
})

watch(() => route.fullPath, () => {
  mobileNavVisible.value = false
  sessionDrawerVisible.value = false
})

watch(isDetailRoute, detail => {
  if (!detail) fullscreen.value = false
})

onMounted(() => {
  document.addEventListener('dragenter', handleGlobalDragEnter, true)
  document.addEventListener('dragover', handleGlobalDragOver, true)
  document.addEventListener('dragleave', handleGlobalDragLeave, true)
  document.addEventListener('drop', handleGlobalDrop, true)
  media.addEventListener('change', updateSystemDark)
  if (isWailsDesktop) {
    window.addEventListener('keydown', handleGlobalKeyDown)
    // @ts-ignore
    window.runtime.EventsOn('app:reload', () => {
      reloadApp()
    })
  }
  maybeOpenCmdkFromRoute()
  void useChatResourcesStore().prefetchChatInput()
})

onUnmounted(() => {
  document.removeEventListener('dragenter', handleGlobalDragEnter, true)
  document.removeEventListener('dragover', handleGlobalDragOver, true)
  document.removeEventListener('dragleave', handleGlobalDragLeave, true)
  document.removeEventListener('drop', handleGlobalDrop, true)
  media.removeEventListener('change', updateSystemDark)
  if (isWailsDesktop) {
    window.removeEventListener('keydown', handleGlobalKeyDown)
    // @ts-ignore
    if (window.runtime?.EventsOff) {
      // @ts-ignore
      window.runtime.EventsOff('app:reload')
    }
  }
  dragCounter = 0
})
</script>

<style lang="less">
.app-shell {
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
  --td-brand-color-hover: #08dd6e;
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

.app-shell *,
.app-shell *::before,
.app-shell *::after {
  box-sizing: border-box;
}

/* Hide scrollbars inside the app shell while keeping wheel/touch scrolling. */
.app-shell,
.app-shell * {
  scrollbar-width: none !important;
  -ms-overflow-style: none !important;
}

.app-shell::-webkit-scrollbar,
.app-shell *::-webkit-scrollbar {
  width: 0 !important;
  height: 0 !important;
  display: none !important;
}

html[theme-mode="dark"] .app-shell {
  --w-base: #141a12;
  --w-topbar: #273321;
  --w-window: #1e261b;
  --w-cream: #252d21;
  --w-paper: #2c3527;
  --w-accent: #384232;
  --w-ink: #f2f6ec;
  --w-body: #c6d0bd;
  --w-muted: #9aa891;
  --w-line: #4a5542;
  --w-green: #53d98d;
  --w-green-dark: #7be0a7;
  --w-green-deep: #a5efc2;
  --w-green-soft: #1d3a2a;
  --w-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.55);
  --td-brand-color: #53d98d;
  --td-brand-color-hover: #7be0a7;
  --td-brand-color-active: #a5efc2;
  --td-brand-color-focus: rgba(83, 217, 141, 0.24);
  --td-brand-color-light: #1d3a2a;
}

.app-shell__bg {
  position: fixed;
  inset: 0;
  z-index: 0;
  background-color: var(--w-base);
  background-position: center;
  background-repeat: no-repeat;
  background-size: cover;
}

html[theme-mode="dark"] .app-shell__bg {
  filter: brightness(0.32) saturate(0.7);
}

.app-topbar {
  position: fixed;
  top: 8px;
  left: 8px;
  right: 8px;
  z-index: 50;
  height: 40px;
  background: var(--w-topbar);
  border-radius: 4px;
  box-shadow: var(--w-shadow);
}

.app-topbar__inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  height: 100%;
  padding: 0 8px;
}

.app-topbar__left,
.app-topbar__right {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.app-topbar__menu {
  display: none;
  place-items: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--w-ink);
  cursor: pointer;
}

.app-brand {
  display: inline-flex;
  align-items: center;
  height: 32px;
  padding: 0 4px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--w-ink);
  cursor: pointer;

  &:hover {
    background: rgba(35, 37, 29, 0.07);
  }
}

html[theme-mode="dark"] .app-brand:hover {
  background: rgba(255, 255, 255, 0.08);
}

.app-brand__name {
  font-family: "WellReadSerif", Georgia, serif;
  font-size: 24px;
  font-weight: 400;
  letter-spacing: 0;
  line-height: 1;
  white-space: nowrap;
}

.app-profile-btn {
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
    box-shadow: 0 0 0 2px rgba(7, 192, 95, 0.2);
  }
}

html[theme-mode="dark"] .app-profile-btn {
  border-color: rgba(255, 255, 255, 0.28);
}

.app-profile-btn__img {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: cover;
}

.app-profile-btn__initial {
  color: var(--w-green-dark);
  font-size: 12px;
  font-weight: 700;
  line-height: 1;
}

.app-fullscreen-btn {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--w-muted);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;

  &:hover {
    background: rgba(35, 37, 29, 0.07);
    color: var(--w-ink);
  }
}

html[theme-mode="dark"] .app-fullscreen-btn:hover {
  background: rgba(255, 255, 255, 0.08);
}

.app-rail {
  position: fixed;
  top: 66px;
  bottom: 8px;
  z-index: 20;
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 112px;
}

.app-rail--left {
  left: 4px;
}

.app-rail--right {
  right: 4px;
}

.app-rail__list {
  width: 100%;
  margin: 0;
  padding: 0;
  list-style: none;
}

.app-rail__cell {
  width: 100%;
}

.app-rail__item {
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

  &.is-active::before {
    opacity: 1;
  }
}

.app-rail__icon {
  display: grid;
  place-items: center;
  font-size: 36px;
  line-height: 1;
  filter: drop-shadow(0 1px 2px rgba(20, 30, 15, 0.35));

  svg {
    width: 1em;
    height: 1em;
  }
}

.app-rail__label {
  max-width: 96px;
  color: #fff;
  font-size: 13px;
  font-weight: 500;
  line-height: 16px;
  text-align: center;
  text-shadow: 0 1px 3px rgba(15, 24, 10, 0.65);
}

.app-stage {
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

.app-window {
  width: 80%;
  max-width: 1200px;
  height: 95%;
  overflow: hidden;
  border-radius: 20px;
  background: #f7f5f0;
  box-shadow:
    0 30px 60px -20px rgba(31, 41, 26, 0.36),
    0 0 0 1px rgba(158, 160, 150, 0.35);
  pointer-events: auto;
}

html[theme-mode="dark"] .app-window {
  background: var(--w-window);
}

.app-window__content {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* Detail pages use a warm off-white canvas; the landing keeps the green window. */
.app-shell--detail {
  --td-bg-color-page: #f7f5f0;
}

.app-shell--detail .app-window,
.app-shell--detail .app-window__content,
.app-shell--detail .app-window__content > * {
  background-color: #f7f5f0;
}

.app-shell--detail .app-window__content > * {
  background-color: #f7f5f0 !important;
}

html[theme-mode="dark"] .app-shell--detail {
  --td-bg-color-page: var(--w-window);
}

html[theme-mode="dark"] .app-shell--detail .app-window,
html[theme-mode="dark"] .app-shell--detail .app-window__content,
html[theme-mode="dark"] .app-shell--detail .app-window__content > * {
  background-color: var(--w-window);
}

html[theme-mode="dark"] .app-shell--detail .app-window__content > * {
  background-color: var(--w-window) !important;
}

.app-shell--fullscreen .app-topbar {
  border-radius: 4px 4px 0 0;
}

.app-shell--fullscreen .app-stage {
  top: 48px;
  right: 8px;
  bottom: 0;
  left: 8px;
  align-items: stretch;
  justify-content: stretch;
}

.app-shell--fullscreen .app-window {
  width: 100%;
  max-width: none;
  height: 100%;
  border-radius: 0;
}

.app-shell--fullscreen .app-rail {
  display: none;
}

.app-mobile-nav {
  position: fixed;
  top: 56px;
  right: 8px;
  bottom: 8px;
  left: 8px;
  z-index: 70;
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 18px;
  overflow-y: auto;
  border: 1px solid var(--w-line);
  border-radius: 16px;
  background: var(--w-cream);
  box-shadow: var(--w-shadow);
  color: var(--w-ink);
}

.app-mobile-nav__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--w-ink);
  font-size: 14px;
  font-weight: 700;

  button {
    display: grid;
    place-items: center;
    width: 28px;
    height: 28px;
    padding: 0;
    border: 0;
    border-radius: 6px;
    background: rgba(35, 37, 29, 0.08);
    color: var(--w-ink);
    cursor: pointer;
  }
}

.app-mobile-nav__list {
  display: flex;
  flex-direction: column;
  gap: 4px;

  button {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--w-body);
    font: inherit;
    font-size: 15px;
    font-weight: 500;
    text-align: left;
    cursor: pointer;

    svg {
      width: 22px;
      height: 22px;
      color: var(--w-green-dark);
    }

    &:hover,
    &.is-active {
      background: var(--w-green-soft);
      color: var(--w-ink);
    }
  }
}

.app-mobile-nav__logout {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: auto;
  padding: 12px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--w-body);
  font: inherit;
  font-size: 15px;
  font-weight: 500;
  text-align: left;
  cursor: pointer;

  svg {
    width: 22px;
    height: 22px;
    color: var(--w-green-dark);
  }

  &:hover {
    background: rgba(255, 0, 0, 0.06);
  }
}

.mobile-nav-enter-active,
.mobile-nav-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.mobile-nav-enter-from,
.mobile-nav-leave-to {
  opacity: 0;
  transform: translateY(8px);
}

@media (max-width: 1100px) {
  .app-window {
    width: 88%;
  }

  .app-rail--right {
    display: none;
  }
}

@media (max-width: 900px) {
  .app-topbar__menu {
    display: grid;
  }

  .app-rail--left {
    display: none;
  }

  .app-window {
    width: 94%;
  }
}

@media (max-width: 600px) {
  .app-window {
    width: calc(100% - 8px);
    height: calc(100% - 4px);
    border-radius: 16px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .app-shell *,
  .app-shell *::before,
  .app-shell *::after {
    scroll-behavior: auto !important;
    transition-duration: 0.001ms !important;
    animation-duration: 0.001ms !important;
    animation-iteration-count: 1 !important;
  }
}
</style>
