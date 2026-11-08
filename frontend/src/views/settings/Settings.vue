<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="visible" class="settings-overlay">
        <div class="settings-modal">
          <button class="close-btn" @click="handleClose" :aria-label="$t('general.close')">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
              <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
            </svg>
          </button>

          <div class="settings-container">
            <div class="settings-sidebar">
              <div class="sidebar-header">
                <h2 class="sidebar-title">{{ $t('general.settings') }}</h2>
              </div>
              <div class="settings-nav">
                <div v-for="item in navItems" :key="item.key" :class="['nav-item', { active: currentSection === item.key }]"
                  @click="selectSection(item.key)">
                  <t-icon :name="item.icon" class="nav-icon" />
                  <span class="nav-label">{{ item.label }}</span>
                </div>
              </div>
            </div>

            <div class="settings-content">
              <div class="content-wrapper">
                <template v-if="currentSection === 'account'">
                  <div class="section"><UserProfile /></div>
                  <div class="section"><GeneralSettings /></div>
                  <div class="section"><AnswerModeSettings /></div>
                  <div class="section"><GlobalPromptSettings /></div>
                  <div class="section"><ChatHistorySettings /></div>
                </template>

                <template v-else-if="currentSection === 'models'">
                  <div class="section"><ModelSettings /></div>
                  <div class="section"><OllamaSettings /></div>
                  <div class="section"><ParserEngineSettings /></div>
                  <div class="section"><StorageEngineSettings /></div>
                </template>

                <div v-else-if="currentSection === 'integrations'" class="section section--wide">
                  <PersonalIntegrationSettings />
                </div>

                <template v-else-if="currentSection === 'health'">
                  <div class="section"><SystemInfo /></div>
                  <div class="section"><RuntimeQueues /></div>
                </template>

                <div v-else-if="currentSection === 'about'" class="section">
                  <AboutSettings />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUIStore } from '@/stores/ui'
import UserProfile from './UserProfile.vue'
import GeneralSettings from './GeneralSettings.vue'
import AnswerModeSettings from './AnswerModeSettings.vue'
import GlobalPromptSettings from './GlobalPromptSettings.vue'
import ChatHistorySettings from './ChatHistorySettings.vue'
import ModelSettings from './ModelSettings.vue'
import OllamaSettings from './OllamaSettings.vue'
import ParserEngineSettings from './ParserEngineSettings.vue'
import StorageEngineSettings from './StorageBackendSettings.vue'
import SystemInfo from './SystemInfo.vue'
import RuntimeQueues from '@/views/system/RuntimeQueues.vue'
import PersonalIntegrationSettings from './PersonalIntegrationSettings.vue'
import AboutSettings from './AboutSettings.vue'

type SectionKey = 'account' | 'models' | 'integrations' | 'health' | 'about'

const route = useRoute()
const router = useRouter()
const uiStore = useUIStore()

const currentSection = ref<SectionKey>('account')

const navItems = computed(() => [
  { key: 'account' as const, icon: 'user', label: '账户与助手' },
  { key: 'models' as const, icon: 'control-platform', label: '模型与引擎' },
  { key: 'integrations' as const, icon: 'secured', label: '外部接入' },
  { key: 'health' as const, icon: 'info-circle', label: '系统健康与队列' },
  { key: 'about' as const, icon: 'info-circle', label: '关于' },
])

const SECTIONS = new Set<SectionKey>(['account', 'models', 'integrations', 'health', 'about'])

// Legacy section keys (bookmarks, menu deep links) map onto the five panes.
const LEGACY_SECTION_MAP: Record<string, SectionKey> = {
  general: 'account',
  userprofile: 'account',
  chathistory: 'account',
  models: 'models',
  ollama: 'models',
  parser: 'models',
  storage: 'models',
  docsync: 'models',
  retrieval: 'models',
  integrations: 'integrations',
  system: 'health',
  'runtime-queues': 'health',
  'system-global': 'health',
  'platform-api-keys': 'health',
  about: 'about',
}

const normalizeSection = (value: unknown): SectionKey => {
  if (typeof value !== 'string') return 'account'
  if (SECTIONS.has(value as SectionKey)) return value as SectionKey
  if (value.startsWith('integration-')) return 'integrations'
  return LEGACY_SECTION_MAP[value] ?? 'account'
}

const visible = computed(() => route.path === '/platform/settings' || uiStore.showSettingsModal)

const syncRoute = (section: SectionKey) => {
  if (route.path !== '/platform/settings') return
  if (route.query.section === section) return
  void router.replace({ path: '/platform/settings', query: { ...route.query, section } })
}

const selectSection = (section: SectionKey) => {
  currentSection.value = section
  syncRoute(section)
}

const applyRouteSection = () => {
  if (route.path !== '/platform/settings') return
  currentSection.value = normalizeSection(route.query.section)
}

const handleClose = () => {
  if (document.activeElement instanceof HTMLElement) {
    document.activeElement.blur()
  }
  uiStore.closeSettings()
  if (route.path === '/platform/settings') {
    router.back()
  }
}

const handleEscape = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && visible.value) {
    handleClose()
  }
}

const handleSettingsNav = (event: Event) => {
  const detail = (event as CustomEvent).detail
  if (detail?.section) {
    selectSection(normalizeSection(detail.section))
  }
}

watch(
  () => [visible.value, route.path, route.query.section] as const,
  () => {
    if (visible.value) applyRouteSection()
  },
  { immediate: true },
)

watch(
  () => uiStore.settingsInitialSection,
  (section) => {
    if (section && visible.value) {
      selectSection(normalizeSection(section))
    }
  },
)

onMounted(() => {
  window.addEventListener('keydown', handleEscape)
  window.addEventListener('settings-nav', handleSettingsNav)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleEscape)
  window.removeEventListener('settings-nav', handleSettingsNav)
})
</script>

<style lang="less" scoped>
.settings-overlay {
  position: fixed;
  inset: 0;
  z-index: 1100;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  backdrop-filter: blur(4px);
}

.settings-modal {
  position: relative;
  width: 100%;
  max-width: 1080px;
  height: 780px;
  max-height: calc(100vh - 40px);
  background: var(--td-bg-color-container);
  border-radius: 12px;
  box-shadow: 0 6px 28px rgba(15, 23, 42, 0.08);
  overflow: hidden;
}

.close-btn {
  position: absolute;
  top: 16px;
  right: 16px;
  width: 32px;
  height: 32px;
  border: none;
  background: transparent;
  color: var(--td-text-color-secondary);
  cursor: pointer;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
  z-index: 10;

  &:hover {
    background: var(--td-bg-color-container-hover);
    color: var(--td-text-color-primary);
  }
}

.settings-container {
  display: flex;
  height: 100%;
}

.settings-sidebar {
  width: 220px;
  flex-shrink: 0;
  border-right: 1px solid var(--td-component-stroke);
  overflow-y: auto;
}

.sidebar-header {
  padding: 24px 20px 12px;
}

.sidebar-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}

.settings-nav {
  padding: 4px 10px 20px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  font-size: 14px;
  color: var(--td-text-color-primary);
  transition: background 0.2s ease;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }

  &.active {
    background: var(--td-brand-color-light);
    color: var(--td-brand-color);
    font-weight: 600;
  }
}

.nav-icon {
  font-size: 18px;
}

.settings-content {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
}

.content-wrapper {
  padding: 28px 32px 48px;
}

.section + .section {
  margin-top: 28px;
  padding-top: 28px;
  border-top: 1px solid var(--td-component-stroke);
}

.section--wide {
  padding-bottom: 8px;
}

.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
