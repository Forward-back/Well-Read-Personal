<template>
  <div class="kb-settings-page">
    <KnowledgeBaseEditorModal
      :visible="true"
      mode="edit"
      page
      :kb-id="kbId"
      @update:visible="handleClose"
      @success="handleSuccess"
    />
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import KnowledgeBaseEditorModal from './KnowledgeBaseEditorModal.vue'
import { useUIStore } from '@/stores/ui'
import { useChatResourcesStore } from '@/stores/chatResources'

const route = useRoute()
const router = useRouter()
const uiStore = useUIStore()
const chatResources = useChatResourcesStore()
const kbId = String(route.params.kbId || '')

uiStore.currentKBId = kbId
uiStore.kbEditorMode = 'edit'
uiStore.kbEditorType = 'document'
uiStore.kbEditorInitialSection = typeof route.query.section === 'string' ? route.query.section : null

function handleSuccess(id: string) {
  chatResources.invalidateKnowledgeBaseDetail(id)
}

function handleClose() {
  router.push(`/platform/knowledge-bases/${kbId}`)
}

onBeforeUnmount(() => {
  uiStore.closeKBEditor()
})
</script>

<style scoped lang="less">
.kb-settings-page {
  flex: 1;
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--td-bg-color-page);
}

.kb-settings-page :deep(.settings-overlay.is-page) {
  flex: 1;
  min-height: 0;
}
</style>
