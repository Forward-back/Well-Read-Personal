<template>
  <t-dialog v-model:visible="visible" :header="$t('settingsPages.help.header')" :footer="false" width="520px">
    <div class="help-body">
      <section>
        <h3>{{ $t('settingsPages.help.shortcuts') }}</h3>
        <ul>
          <li><kbd>Ctrl K</kbd> {{ $t('settingsPages.help.shortcutSearch') }}</li>
          <li><kbd>{{ $t('settingsPages.help.spaceKey') }}</kbd> {{ $t('settingsPages.help.shortcutFlip') }}</li>
          <li><kbd>1</kbd> {{ $t('settingsPages.help.ratingForgot') }} · <kbd>2</kbd> {{ $t('settingsPages.help.ratingSlow') }} · <kbd>3</kbd> {{ $t('settingsPages.help.ratingRecalled') }} · <kbd>4</kbd> {{ $t('settingsPages.help.ratingEasy') }}</li>
          <li><kbd>Esc</kbd> {{ $t('settingsPages.help.shortcutExit') }}</li>
        </ul>
      </section>
      <section>
        <h3>{{ $t('settingsPages.help.project') }}</h3>
        <p>{{ $t('settingsPages.help.projectDesc') }}</p>
        <p class="help-muted">{{ $t('settingsPages.help.versionLine', { version, commit }) }}</p>
        <a href="https://github.com/Forward-back" target="_blank" rel="noopener noreferrer">github.com/Forward-back</a>
      </section>
      <section>
        <h3>{{ $t('settingsPages.help.assets') }}</h3>
        <p class="help-muted">
          {{ $t('settingsPages.help.assetsBefore') }}
          <a href="https://icons8.com" target="_blank" rel="noopener noreferrer">Icons8</a>{{ $t('settingsPages.help.assetsAfter') }}
        </p>
      </section>
    </div>
  </t-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{ (e: 'update:visible', value: boolean): void }>()
const visible = computed({
  get: () => props.visible,
  set: value => emit('update:visible', value),
})
const version = typeof __FRONTEND_VERSION__ !== 'undefined' ? __FRONTEND_VERSION__ : 'dev'
const commit = typeof __FRONTEND_COMMIT__ !== 'undefined' ? __FRONTEND_COMMIT__ : 'local'
</script>

<style scoped>
.help-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
  font-size: 13px;
  line-height: 21px;
  color: var(--td-text-color-secondary);
}

h3 {
  margin: 0 0 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

ul {
  margin: 0;
  padding-left: 18px;
}

p { margin: 0 0 4px; }

kbd {
  padding: 1px 5px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 4px;
  background: var(--td-bg-color-secondarycontainer);
  font-family: inherit;
  font-size: 11px;
}

.help-muted { color: var(--td-text-color-placeholder); }

a { color: var(--td-brand-color); }
</style>
