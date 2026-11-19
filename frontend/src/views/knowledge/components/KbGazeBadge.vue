<template>
  <t-tooltip :content="t('knowledgeList.features.gaze')" placement="top">
    <span class="kb-gaze-badge" role="img" :aria-label="t('knowledgeList.features.gaze')">
      <img class="kb-gaze-icon" :src="bookIcon" alt="" aria-hidden="true" />
    </span>
  </t-tooltip>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import bookIcon from '@/assets/icons/hand/book.png'

const { t } = useI18n()
</script>

<style scoped lang="less">
.kb-gaze-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: var(--td-text-color-secondary);

  .kb-gaze-icon {
    width: 15px;
    height: 15px;
    object-fit: contain;
    display: block;
  }
}
</style>
