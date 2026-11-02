<template>
  <div class="skills-page">
    <div class="skills-page__inner">
      <SkillCatalogSettings />
    </div>
  </div>
</template>

<script setup lang="ts">
import SkillCatalogSettings from '@/views/settings/SkillCatalogSettings.vue'
</script>

<style lang="less" scoped>
.skills-page {
  height: 100%;
  overflow-y: auto;
  background: var(--td-bg-color-page);
}

.skills-page__inner {
  max-width: 960px;
  margin: 0 auto;
  padding: 32px 28px 48px;
}
</style>
