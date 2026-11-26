<template>
  <section class="block">
    <header class="block__header">
      <div>
        <h3>{{ $t('settingsPages.webSearchProvider.title') }}</h3>
        <p>{{ $t('settingsPages.webSearchProvider.desc') }}</p>
      </div>
      <t-button theme="primary" @click="openCreate">{{ $t('settingsPages.webSearchProvider.create') }}</t-button>
    </header>

    <t-loading :loading="loading">
      <div v-if="providers.length === 0" class="empty">
        {{ $t('settingsPages.webSearchProvider.empty') }}
      </div>
      <div v-else class="provider-list">
        <article v-for="provider in providers" :key="provider.id" class="provider-card">
          <div class="provider-main">
            <div class="provider-title-row">
              <span class="provider-name">{{ provider.name }}</span>
              <t-tag v-if="provider.is_default" theme="primary" variant="light" size="small">{{ $t('settingsPages.webSearchProvider.default') }}</t-tag>
              <t-tag variant="outline" size="small">{{ providerLabel(provider.provider) }}</t-tag>
            </div>
            <p v-if="provider.description" class="provider-desc">{{ provider.description }}</p>
            <p class="provider-meta">
              Key: {{ provider.credentials?.api_key?.configured ? $t('settingsPages.webSearchProvider.keyConfigured') : $t('settingsPages.webSearchProvider.keyMissing') }}
            </p>
          </div>
          <div class="provider-actions">
            <t-button size="small" variant="text" :loading="testingId === provider.id" @click="testProvider(provider)">
              {{ $t('settingsPages.webSearchProvider.test') }}
            </t-button>
            <t-button size="small" variant="text" @click="openEdit(provider)">{{ $t('settingsPages.webSearchProvider.edit') }}</t-button>
            <t-button
              v-if="!provider.is_default"
              size="small"
              variant="text"
              @click="setDefault(provider)"
            >
              {{ $t('settingsPages.webSearchProvider.setDefault') }}
            </t-button>
            <t-button size="small" variant="text" theme="danger" @click="removeProvider(provider)">
              {{ $t('settingsPages.webSearchProvider.delete') }}
            </t-button>
          </div>
        </article>
      </div>
    </t-loading>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="editing ? $t('settingsPages.webSearchProvider.editTitle') : $t('settingsPages.webSearchProvider.createTitle')"
      width="560px"
      :confirm-btn="{ content: $t('settingsPages.webSearchProvider.save'), theme: 'primary', loading: saving }"
      @confirm="save"
    >
      <div class="form">
        <label class="field">
          <span class="field-label">{{ $t('settingsPages.webSearchProvider.name') }}</span>
          <t-input v-model="form.name" :placeholder="$t('settingsPages.webSearchProvider.namePlaceholder')" />
        </label>
        <label class="field">
          <span class="field-label">{{ $t('settingsPages.webSearchProvider.provider') }}</span>
          <t-select v-model="form.provider" :disabled="editing">
            <t-option value="deepseek" label="DeepSeek" />
            <t-option value="duckduckgo" label="DuckDuckGo" />
          </t-select>
        </label>
        <label class="field">
          <span class="field-label">{{ $t('settingsPages.webSearchProvider.description') }}</span>
          <t-input v-model="form.description" :placeholder="$t('settingsPages.webSearchProvider.descriptionPlaceholder')" />
        </label>

        <div v-if="!editing" class="field">
          <span class="field-label">API Key</span>
          <t-input
            v-model="form.apiKey"
            type="password"
            :placeholder="form.provider === 'deepseek' ? 'DeepSeek API Key' : $t('settingsPages.webSearchProvider.apiKeyPlaceholder')"
          />
        </div>

        <div v-else class="field">
          <span class="field-label">API Key</span>
          <t-radio-group v-model="form.keyAction" variant="default-filled">
            <t-radio-button value="keep">{{ $t('settingsPages.webSearchProvider.keep') }}</t-radio-button>
            <t-radio-button value="replace">{{ $t('settingsPages.webSearchProvider.replace') }}</t-radio-button>
            <t-radio-button value="clear" :disabled="!editingKeyConfigured">{{ $t('settingsPages.webSearchProvider.clear') }}</t-radio-button>
          </t-radio-group>
          <t-input
            v-if="form.keyAction === 'replace'"
            v-model="form.apiKey"
            type="password"
            :placeholder="$t('settingsPages.webSearchProvider.newKeyPlaceholder')"
          />
          <p v-if="form.keyAction === 'clear'" class="danger-hint">
            {{ $t('settingsPages.webSearchProvider.clearHint') }}
          </p>
        </div>

        <label class="field field--row">
          <span class="field-label">{{ $t('settingsPages.webSearchProvider.setDefaultLabel') }}</span>
          <t-switch v-model="form.isDefault" />
        </label>
      </div>
    </t-dialog>
  </section>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import {
  createWebSearchProvider,
  deleteWebSearchProvider,
  deleteWebSearchProviderCredentialField,
  listWebSearchProviders,
  putWebSearchProviderCredentials,
  testWebSearchProvider,
  updateWebSearchProvider,
  type WebSearchProviderEntity,
} from '@/api/web-search-provider'

const { t } = useI18n()
const providers = ref<WebSearchProviderEntity[]>([])
const loading = ref(false)
const saving = ref(false)
const testingId = ref('')
const dialogVisible = ref(false)
const editing = ref<WebSearchProviderEntity | null>(null)
const editingKeyConfigured = ref(false)

const form = reactive({
  name: '',
  provider: 'deepseek' as WebSearchProviderEntity['provider'],
  description: '',
  apiKey: '',
  isDefault: false,
  keyAction: 'keep' as 'keep' | 'replace' | 'clear',
})

const providerLabel = (value: string) => (value === 'deepseek' ? 'DeepSeek' : 'DuckDuckGo')

const resetForm = () => {
  form.name = ''
  form.provider = 'deepseek'
  form.description = ''
  form.apiKey = ''
  form.isDefault = false
  form.keyAction = 'keep'
}

const load = async () => {
  loading.value = true
  try {
    const response: any = await listWebSearchProviders()
    providers.value = Array.isArray(response?.data) ? response.data : []
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('settingsPages.webSearchProvider.loadFailed'))
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  editing.value = null
  editingKeyConfigured.value = false
  resetForm()
  form.isDefault = providers.value.length === 0
  dialogVisible.value = true
}

const openEdit = (provider: WebSearchProviderEntity) => {
  editing.value = provider
  editingKeyConfigured.value = provider.credentials?.api_key?.configured === true
  resetForm()
  form.name = provider.name
  form.provider = provider.provider
  form.description = provider.description || ''
  form.isDefault = provider.is_default === true
  dialogVisible.value = true
}

const save = async () => {
  if (!form.name.trim()) {
    MessagePlugin.warning(t('settingsPages.webSearchProvider.nameRequired'))
    return
  }
  if (!editing.value && form.provider === 'deepseek' && !form.apiKey.trim()) {
    MessagePlugin.warning(t('settingsPages.webSearchProvider.deepseekKeyRequired'))
    return
  }
  if (editing.value && form.keyAction === 'replace' && !form.apiKey.trim()) {
    MessagePlugin.warning(t('settingsPages.webSearchProvider.newKeyRequired'))
    return
  }

  saving.value = true
  try {
    if (!editing.value) {
      await createWebSearchProvider({
        name: form.name.trim(),
        provider: form.provider,
        description: form.description.trim(),
        parameters: { api_key: form.apiKey.trim() },
        is_default: form.isDefault,
      })
    } else {
      await updateWebSearchProvider(editing.value.id!, {
        name: form.name.trim(),
        provider: form.provider,
        description: form.description.trim(),
        is_default: form.isDefault,
      })
      if (form.keyAction === 'clear') {
        await deleteWebSearchProviderCredentialField(editing.value.id!, 'api_key')
      } else if (form.keyAction === 'replace' && form.apiKey.trim()) {
        await putWebSearchProviderCredentials(editing.value.id!, { api_key: form.apiKey.trim() })
      }
    }
    MessagePlugin.success(t('settingsPages.webSearchProvider.saved'))
    dialogVisible.value = false
    await load()
  } catch (e: any) {
    MessagePlugin.error(e?.error?.message || e?.message || t('settingsPages.webSearchProvider.saveFailed'))
  } finally {
    saving.value = false
  }
}

const setDefault = async (provider: WebSearchProviderEntity) => {
  try {
    await updateWebSearchProvider(provider.id!, { is_default: true })
    await load()
  } catch (e: any) {
    MessagePlugin.error(e?.error?.message || e?.message || t('settingsPages.webSearchProvider.defaultFailed'))
  }
}

const testProvider = async (provider: WebSearchProviderEntity) => {
  testingId.value = provider.id || ''
  try {
    const response: any = await testWebSearchProvider(provider.id)
    const count = response?.data?.results ?? 0
    MessagePlugin.success(t('settingsPages.webSearchProvider.testSuccess', { count }))
  } catch (e: any) {
    MessagePlugin.error(e?.error?.message || e?.message || t('settingsPages.webSearchProvider.testFailed'))
  } finally {
    testingId.value = ''
  }
}

const removeProvider = (provider: WebSearchProviderEntity) => {
  const dialog = DialogPlugin.confirm({
    header: t('settingsPages.webSearchProvider.deleteTitle'),
    body: t('settingsPages.webSearchProvider.deleteBody', { name: provider.name }),
    confirmBtn: { content: t('settingsPages.webSearchProvider.delete'), theme: 'danger' },
    onConfirm: async () => {
      try {
        await deleteWebSearchProvider(provider.id!)
        MessagePlugin.success(t('settingsPages.webSearchProvider.deleted'))
        dialog.hide()
        await load()
      } catch (e: any) {
        MessagePlugin.error(e?.error?.message || e?.message || t('settingsPages.webSearchProvider.deleteFailed'))
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
  gap: 12px;
}

.block__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
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

.empty {
  padding: 16px;
  border: 1px dashed var(--td-component-border);
  border-radius: 8px;
  color: var(--td-text-color-secondary);
  font-size: 13px;
}

.provider-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.provider-card {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  border: 1px solid var(--td-component-border);
  border-radius: 8px;
}

.provider-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.provider-name {
  font-size: 14px;
  font-weight: 600;
}

.provider-desc,
.provider-meta {
  margin: 6px 0 0;
  color: var(--td-text-color-secondary);
  font-size: 12px;
}

.provider-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field--row {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.field-label {
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.danger-hint {
  margin: 0;
  color: var(--td-error-color);
  font-size: 12px;
}
</style>
