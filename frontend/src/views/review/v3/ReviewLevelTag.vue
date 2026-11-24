<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { normalizeVulnerability, vulnerabilityLabelKey } from './vulnerability'

const props = defineProps<{
  level: string
}>()

const { t } = useI18n()

const normalized = computed(() => normalizeVulnerability(props.level))
</script>

<template>
  <span class="rv-level-tag" :class="`rv-level-tag--${normalized}`">
    {{ t(vulnerabilityLabelKey(normalized)) }}
  </span>
</template>

<style lang="less" scoped>
.rv-level-tag {
  display: inline-flex;
  align-items: center;
  height: 20px;
  padding: 0 8px;
  font-size: 14px;
  font-weight: 500;
  line-height: 18px;
  white-space: nowrap;
  border-radius: 999px;
}

.rv-level-tag--severe {
  color: var(--rvn-red-deep, #8a4e47);
  background: var(--rvn-red-soft, #ebdcd7);
}

.rv-level-tag--warning {
  color: var(--rvn-orange-deep, #7e5e42);
  background: var(--rvn-orange-soft, #ece3d8);
}

.rv-level-tag--observe {
  color: var(--rvn-accent-deep, #47677a);
  background: var(--rvn-accent-soft, #dfe7ea);
}

.rv-level-tag--healthy {
  color: var(--rvn-green-deep, #5e7261);
  background: var(--rvn-green-soft, #e3e9df);
}

.rv-level-tag--none {
  color: var(--rvn-ink-soft, #6e675e);
  background: var(--rvn-gray-soft, #ede7dd);
}
</style>
