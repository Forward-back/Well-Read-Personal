<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ProtectedResourcePreview from '@/components/ProtectedResourcePreview.vue'
import ManualKnowledgeEditor from '@/components/manual-knowledge-editor.vue'
import UploadConfirmHost from '@/components/UploadConfirmHost.vue'
import { useSettingsStore } from '@/stores/settings'

import zhCNConfig from 'tdesign-vue-next/esm/locale/zh_CN'

const { locale } = useI18n()
const settingsStore = useSettingsStore()

const tdLocaleMap: Record<string, object> = {
  'zh-CN': zhCNConfig,
}

const tdGlobalConfig = computed(() => tdLocaleMap[locale.value] || zhCNConfig)

let updateCheckTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  setTimeout(() => {
    if (settingsStore.isAutoCheckUpdateEnabled) {
      // @ts-ignore
      if (window.go && window.go.main && window.go.main.App && window.go.main.App.AutoCheckForUpdates) {
        // @ts-ignore
        window.go.main.App.AutoCheckForUpdates()
      }
    }
  }, 2000)

  updateCheckTimer = setInterval(() => {
    if (settingsStore.isAutoCheckUpdateEnabled) {
      // @ts-ignore
      if (window.go && window.go.main && window.go.main.App && window.go.main.App.AutoCheckForUpdates) {
        // @ts-ignore
        window.go.main.App.AutoCheckForUpdates()
      }
    }
  }, 4 * 60 * 60 * 1000)
})

onUnmounted(() => {
  if (updateCheckTimer) {
    clearInterval(updateCheckTimer)
  }
})
</script>
<template>
  <t-config-provider :globalConfig="tdGlobalConfig">
    <div id="app">
      <RouterView />
      <ManualKnowledgeEditor />
      <ProtectedResourcePreview />
      <UploadConfirmHost />
    </div>
  </t-config-provider>
</template>
<style>
html {
  /* 提示 UA 使用对应配色绘制滚动条等，减少主题切换时的额外重绘 */
  color-scheme: light dark;
}

body,
html,
#app {
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 0;
  font-size: 14px;
  font-family: var(--app-font-family);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  background: var(--td-bg-color-page);
  color: var(--td-text-color-primary);
}

#app {
  /* 独立合成层，减轻 WebKit 全量重绘时整窗与内容的撕裂感（桌面 WebView 尤其明显） */
  isolation: isolate;
  transform: translateZ(0);
  backface-visibility: hidden;
}
</style>
