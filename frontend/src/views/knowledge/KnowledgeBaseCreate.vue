<template>
  <div class="kb-create-page">
    <KnowledgeBaseEditorModal
      :visible="true"
      mode="create"
      page
      :initial-type="initialType"
      @update:visible="handleClose"
      @success="handleSuccess"
    />
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import KnowledgeBaseEditorModal from './KnowledgeBaseEditorModal.vue'
import { useUIStore } from '@/stores/ui'
import { useChatResourcesStore } from '@/stores/chatResources'

const route = useRoute()
const router = useRouter()
const uiStore = useUIStore()
const chatResources = useChatResourcesStore()
const createdId = ref('')
const initialType = (route.query.type === 'faq' ? 'faq' : 'document') as 'document' | 'faq'

uiStore.currentKBId = null
uiStore.kbEditorMode = 'create'
uiStore.kbEditorType = initialType
uiStore.kbEditorInitialSection = typeof route.query.section === 'string' ? route.query.section : null

function handleSuccess(kbId: string) {
  createdId.value = kbId
  chatResources.invalidateKnowledgeBaseDetail(kbId)
}

function handleClose() {
  const kbId = createdId.value
  router.push(kbId ? `/platform/knowledge-bases/${kbId}` : '/platform/knowledge-bases')
}

onBeforeUnmount(() => {
  uiStore.closeKBEditor()
})
</script>

<style scoped lang="less">
.kb-create-page {
  flex: 1;
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--td-bg-color-page);
}

.kb-create-page :deep(.settings-overlay.is-page) {
  flex: 1;
  min-height: 0;
}
</style>
