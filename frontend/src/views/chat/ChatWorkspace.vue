<template>
  <div class="chat-workspace">
    <aside v-if="!narrow && layout.railVisible" class="chat-workspace__rail" aria-label="会话栏">
      <SessionDrawer :visible="true" mode="rail" @update:visible="handleRailVisible" />
    </aside>

    <div class="chat-workspace__main" :class="{ 'is-search-open': layout.searchOpen }">
      <button
        v-if="!narrow && !referencesStore.visible"
        type="button"
        class="chat-workspace__toggle"
        :aria-label="layout.railVisible ? '收起会话栏' : '展开会话栏'"
        :title="layout.railVisible ? '收起会话栏' : '展开会话栏'"
        @click="layout.toggleRail()"
      >
        <t-icon :name="layout.railVisible ? 'chevron-left' : 'chevron-right'" size="18px" />
      </button>
      <RouterView />
      <ChatHistorySearch v-if="layout.searchOpen" @close="layout.closeSearch()" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import SessionDrawer from '@/components/layout/SessionDrawer.vue'
import ChatHistorySearch from './ChatHistorySearch.vue'
import { useChatLayoutStore } from '@/stores/chatLayout'
import { useChatReferencesStore } from '@/stores/chatReferences'

const layout = useChatLayoutStore()
const referencesStore = useChatReferencesStore()
const route = useRoute()
const narrow = ref(false)
let media: MediaQueryList | null = null

watch(() => route.fullPath, () => layout.closeSearch())

function updateNarrow() {
  narrow.value = media?.matches ?? false
}

function handleRailVisible(value: boolean) {
  if (!value) layout.setRailVisible(false)
}

onMounted(() => {
  media = window.matchMedia('(max-width: 900px)')
  updateNarrow()
  media.addEventListener('change', updateNarrow)
})

onUnmounted(() => {
  media?.removeEventListener('change', updateNarrow)
})
</script>

<style lang="less" scoped>
.chat-workspace {
  position: relative;
  flex: 1;
  min-height: 0;
  min-width: 0;
  display: flex;
  overflow: hidden;
}

.chat-workspace__rail {
  flex: 0 0 260px;
  width: 260px;
  min-height: 0;
  background: transparent;
}

.chat-workspace__main {
  position: relative;
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.chat-workspace__main :deep(.chat) {
  padding-left: 0;
}

.chat-workspace__main.is-search-open :deep(.input-container) {
  visibility: hidden;
}

@media (min-width: 901px) {
  .chat-workspace__main :deep(.page-topbar) {
    padding-left: 56px;
  }
}

.chat-workspace__main :deep(.dialogue-wrap) {
  padding-left: 0;
}

:global(.app-shell:not(.app-shell--fullscreen) .chat-workspace__main .chat:not(.has-references-panel):not(.has-sandbox-panel)) {
  padding-left: 32px;
  padding-right: 32px;
}

:global(.app-shell:not(.app-shell--fullscreen) .chat-workspace__main .dialogue-wrap) {
  padding-left: 32px;
  padding-right: 32px;
}

.chat-workspace__toggle {
  position: absolute;
  top: 10px;
  left: 10px;
  z-index: 7;
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--w-muted);
  cursor: pointer;

  &:hover {
    color: var(--w-ink);
    background: var(--w-accent);
  }
}
</style>
