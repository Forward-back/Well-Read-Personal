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
  padding: 0 7px;
  border-radius: var(--td-radius-round);
  font-size: 11px;
  line-height: 18px;
  font-weight: 500;
  white-space: nowrap;
}

.rv-level-tag--severe {
  color: var(--rv-vuln-severe);
  background: color-mix(in srgb, var(--rv-vuln-severe) 12%, var(--td-bg-color-container));
  border: 1px solid color-mix(in srgb, var(--rv-vuln-severe) 32%, transparent);
}

.rv-level-tag--warning {
  color: var(--rv-vuln-warning);
  background: color-mix(in srgb, var(--rv-vuln-warning) 12%, var(--td-bg-color-container));
  border: 1px solid color-mix(in srgb, var(--rv-vuln-warning) 32%, transparent);
}

.rv-level-tag--observe {
  color: var(--rv-vuln-observe);
  background: color-mix(in srgb, var(--rv-vuln-observe) 10%, var(--td-bg-color-container));
  border: 1px solid color-mix(in srgb, var(--rv-vuln-observe) 28%, transparent);
}

.rv-level-tag--healthy {
  color: var(--rv-vuln-healthy);
  background: color-mix(in srgb, var(--rv-vuln-healthy) 10%, var(--td-bg-color-container));
  border: 1px solid color-mix(in srgb, var(--rv-vuln-healthy) 28%, transparent);
}

.rv-level-tag--none {
  color: var(--td-text-color-placeholder);
  background: var(--td-bg-color-container-hover);
  border: 1px solid var(--td-component-stroke);
}
</style>
