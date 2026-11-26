<template>
  <section class="block">
    <header class="block__header">
      <div>
        <h3>{{ $t('settingsPages.webSearch.title') }}</h3>
        <p>{{ $t('settingsPages.webSearch.desc') }}</p>
      </div>
    </header>

    <div class="provider-list">
      <button
        v-for="provider in providers"
        :key="provider.value"
        type="button"
        class="provider-card"
        :class="{ active: form.provider === provider.value }"
        @click="form.provider = provider.value"
      >
        <span class="provider-title">{{ provider.label }}</span>
        <span class="provider-desc">{{ provider.description }}</span>
      </button>
    </div>

    <div v-if="form.provider === 'deepseek'" class="key-row">
      <t-input
        v-model="apiKeyInput"
        type="password"
        clearable
        class="key-input"
        :placeholder="keyConfigured ? $t('settingsPages.webSearch.savedKeyPlaceholder') : $t('settingsPages.webSearch.emptyKeyPlaceholder')"
      />
      <span class="key-status">{{ keyConfigured ? $t('settingsPages.webSearch.keyConfigured') : $t('settingsPages.webSearch.keyNotConfigured') }}</span>
      <t-button
        v-if="keyConfigured"
        variant="text"
        theme="danger"
        :loading="clearing"
        @click="clearKey"
      >
        {{ $t('settingsPages.webSearch.clearKey') }}
      </t-button>
    </div>

    <p v-if="fallbackWarning" class="fallback-warning">{{ fallbackWarning }}</p>

    <div class="action-row">
      <span class="current">{{ $t('settingsPages.webSearch.currentProvider', { provider: currentProviderLabel }) }}</span>
      <t-button theme="primary" :loading="saving" :disabled="saving || loading" @click="save">{{ $t('settingsPages.webSearch.save') }}</t-button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import {
  getTenantWebSearchConfig,
  updateTenantWebSearchConfig,
  type WebSearchConfig,
} from '@/api/web-search'

const { t } = useI18n()

const providers = computed(() => [
  { value: 'deepseek', label: 'DeepSeek', description: t('settingsPages.webSearch.deepseekDesc') },
  { value: 'duckduckgo', label: 'DuckDuckGo', description: t('settingsPages.webSearch.duckduckgoDesc') },
] as const)

const form = reactive({
  provider: 'deepseek',
  maxResults: 10,
  includeDate: false,
  compressionMethod: 'none',
  blacklist: [] as string[],
})

const apiKeyInput = ref('')
const keyConfigured = ref(false)
const fallbackAvailable = ref(false)
const fallbackProvider = ref('')
const loading = ref(false)
const saving = ref(false)
const clearing = ref(false)

const currentProviderLabel = computed(
  () => providers.value.find((item) => item.value === form.provider)?.label || form.provider,
)

const fallbackWarning = computed(() => {
  if (form.provider !== 'deepseek' || keyConfigured.value) return ''
  if (!fallbackAvailable.value) {
    return t('settingsPages.webSearch.warningNoFallback')
  }
  if (fallbackProvider.value && fallbackProvider.value.toLowerCase() !== 'deepseek') {
    return t('settingsPages.webSearch.warningProviderMismatch', { provider: fallbackProvider.value })
  }
  return ''
})

const applyConfig = (config: WebSearchConfig) => {
  form.provider = config.provider || 'deepseek'
  form.maxResults = config.max_results || 10
  form.includeDate = config.include_date || false
  form.compressionMethod = config.compression_method || 'none'
  form.blacklist = config.blacklist || []
  keyConfigured.value = config.api_key_configured === true
  fallbackAvailable.value = config.direct_fallback_available === true
  fallbackProvider.value = config.direct_fallback_provider || ''
  apiKeyInput.value = ''
}

const load = async () => {
  loading.value = true
  try {
    const response = await getTenantWebSearchConfig()
    applyConfig((response?.data || {}) as WebSearchConfig)
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('settingsPages.webSearch.loadFailed'))
  } finally {
    loading.value = false
  }
}

const save = async () => {
  const apiKey = apiKeyInput.value.trim()
  saving.value = true
  try {
    await updateTenantWebSearchConfig({
      provider: form.provider,
      api_key: apiKey,
      max_results: form.maxResults,
      include_date: form.includeDate,
      compression_method: form.compressionMethod,
      blacklist: form.blacklist,
    })
    MessagePlugin.success(t('settingsPages.webSearch.saved'))
    await load()
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('settingsPages.webSearch.saveFailed'))
  } finally {
    saving.value = false
  }
}

const clearKey = () => {
  const dialog = DialogPlugin.confirm({
    header: t('settingsPages.webSearch.clearTitle'),
    body: t('settingsPages.webSearch.clearBody'),
    confirmBtn: { content: t('settingsPages.webSearch.clearConfirm'), theme: 'danger' },
    onConfirm: async () => {
      clearing.value = true
      try {
        await updateTenantWebSearchConfig({
          provider: form.provider,
          api_key: '',
          clear_api_key: true,
          max_results: form.maxResults,
          include_date: form.includeDate,
          compression_method: form.compressionMethod,
          blacklist: form.blacklist,
        })
        MessagePlugin.success(t('settingsPages.webSearch.cleared'))
        dialog.hide()
        await load()
      } catch (e: any) {
        MessagePlugin.error(e?.message || t('settingsPages.webSearch.clearFailed'))
      } finally {
        clearing.value = false
      }
    },
  })
}

onMounted(load)
</script>

<style scoped>
.block {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.block__header h3 {
  margin: 0 0 6px;
  font-size: 16px;
  font-weight: 600;
}

.block__header p {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.provider-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.provider-card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  padding: 14px 16px;
  border: 1px solid var(--td-component-border);
  border-radius: 8px;
  background: var(--td-bg-color-container);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.2s ease, background 0.2s ease;
}

.provider-card:hover {
  border-color: var(--td-brand-color);
}

.provider-card.active {
  border-color: var(--td-brand-color);
  background: var(--td-brand-color-light);
}

.provider-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.provider-desc {
  font-size: 12px;
  line-height: 1.6;
  color: var(--td-text-color-secondary);
}

.key-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.key-input {
  width: 360px;
}

.key-status,
.current {
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.fallback-warning {
  margin: 0;
  padding: 8px 12px;
  border-radius: 6px;
  background: var(--td-warning-color-light);
  color: var(--td-text-color-primary);
  font-size: 12px;
  line-height: 1.6;
}

.action-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
</style>
