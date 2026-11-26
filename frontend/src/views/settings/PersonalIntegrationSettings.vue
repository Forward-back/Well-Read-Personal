<template>
  <div class="personal-integration">
    <section class="block">
      <header class="block__header">
        <div>
          <h3>{{ $t('settingsPages.personalIntegration.title') }}</h3>
          <p>{{ $t('settingsPages.personalIntegration.desc') }}</p>
        </div>
        <t-button v-if="!key" theme="primary" :loading="creating" @click="handleCreate">{{ $t('settingsPages.personalIntegration.create') }}</t-button>
        <div v-else class="actions">
          <t-button variant="outline" theme="default" :loading="resetting" @click="handleReset">{{ $t('settingsPages.personalIntegration.reset') }}</t-button>
          <t-button variant="outline" theme="danger" :loading="deleting" @click="handleDelete">{{ $t('settingsPages.personalIntegration.delete') }}</t-button>
        </div>
      </header>

      <t-loading :loading="loading">
        <div v-if="key" class="key-card">
          <div class="key-row">
            <span class="key-label">{{ $t('settingsPages.personalIntegration.name') }}</span>
            <span>{{ key.name }}</span>
          </div>
          <div class="key-row">
            <span class="key-label">{{ $t('settingsPages.personalIntegration.key') }}</span>
            <code>{{ key.masked_key }}</code>
          </div>
          <div class="key-row">
            <span class="key-label">{{ $t('settingsPages.personalIntegration.createdAt') }}</span>
            <span>{{ formatDate(key.created_at) }}</span>
          </div>
          <div class="key-row">
            <span class="key-label">{{ $t('settingsPages.personalIntegration.lastUsed') }}</span>
            <span>{{ key.last_used_at ? formatDate(key.last_used_at) : $t('settingsPages.personalIntegration.neverUsed') }}</span>
          </div>
        </div>
        <div v-else class="empty">{{ $t('settingsPages.personalIntegration.empty') }}</div>
      </t-loading>
    </section>

    <section class="block">
      <header class="block__header">
        <div>
          <h3>{{ $t('settingsPages.personalIntegration.usageTitle') }}</h3>
          <p>{{ $t('settingsPages.personalIntegration.usageDesc') }}</p>
        </div>
      </header>
      <pre class="snippet">X-API-Key: {{ key ? $t('settingsPages.personalIntegration.snippetKey') : $t('settingsPages.personalIntegration.snippetEmpty') }}</pre>
      <p class="hint">{{ $t('settingsPages.personalIntegration.hint') }}</p>
    </section>

    <WebSearchProviderSettings />

    <t-dialog
      v-model:visible="tokenVisible"
      :header="$t('settingsPages.personalIntegration.tokenTitle')"
      :footer="false"
      width="560px"
    >
      <p class="token-warning">{{ $t('settingsPages.personalIntegration.tokenWarning') }}</p>
      <div class="token-box">
        <code>{{ issuedToken }}</code>
        <t-button size="small" variant="outline" @click="copyToken">{{ $t('settingsPages.personalIntegration.copy') }}</t-button>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import {
  createPersonalAPIKey,
  deletePersonalAPIKey,
  getPersonalAPIKey,
  resetPersonalAPIKey,
  type PersonalAPIKey,
} from '@/api/settings'
import WebSearchProviderSettings from './WebSearchProviderSettings.vue'

const { t } = useI18n()
const loading = ref(false)
const creating = ref(false)
const resetting = ref(false)
const deleting = ref(false)
const key = ref<PersonalAPIKey | null>(null)
const issuedToken = ref('')
const tokenVisible = ref(false)

const formatDate = (value?: string) => (value ? new Date(value).toLocaleString() : '—')

const refresh = async () => {
  loading.value = true
  try {
    const response = await getPersonalAPIKey()
    key.value = response.success && response.data?.exists ? response.data.key ?? null : null
  } catch {
    MessagePlugin.error(t('settingsPages.personalIntegration.readFailed'))
  } finally {
    loading.value = false
  }
}

const showToken = (token: string) => {
  issuedToken.value = token
  tokenVisible.value = true
}

const handleCreate = async () => {
  creating.value = true
  try {
    const response = await createPersonalAPIKey()
    if (response.success && response.data) {
      key.value = response.data
      showToken(response.data.token)
      MessagePlugin.success(t('settingsPages.personalIntegration.created'))
    } else {
      MessagePlugin.error(t('settingsPages.personalIntegration.createFailed'))
    }
  } catch {
    MessagePlugin.error(t('settingsPages.personalIntegration.createFailed'))
  } finally {
    creating.value = false
  }
}

const handleReset = () => {
  const dialog = DialogPlugin.confirm({
    header: t('settingsPages.personalIntegration.resetTitle'),
    body: t('settingsPages.personalIntegration.resetBody'),
    confirmBtn: { content: t('settingsPages.personalIntegration.reset'), theme: 'danger' },
    onConfirm: async () => {
      dialog.destroy()
      resetting.value = true
      try {
        const response = await resetPersonalAPIKey()
        if (response.success && response.data) {
          key.value = response.data
          showToken(response.data.token)
          MessagePlugin.success(t('settingsPages.personalIntegration.resetSuccess'))
        } else {
          MessagePlugin.error(t('settingsPages.personalIntegration.resetFailed'))
        }
      } catch {
        MessagePlugin.error(t('settingsPages.personalIntegration.resetFailed'))
      } finally {
        resetting.value = false
      }
    },
  })
}

const handleDelete = () => {
  const dialog = DialogPlugin.confirm({
    header: t('settingsPages.personalIntegration.deleteTitle'),
    body: t('settingsPages.personalIntegration.deleteBody'),
    confirmBtn: { content: t('settingsPages.personalIntegration.delete'), theme: 'danger' },
    onConfirm: async () => {
      dialog.destroy()
      deleting.value = true
      try {
        await deletePersonalAPIKey()
        key.value = null
        MessagePlugin.success(t('settingsPages.personalIntegration.deleteSuccess'))
      } catch {
        MessagePlugin.error(t('settingsPages.personalIntegration.deleteFailed'))
      } finally {
        deleting.value = false
      }
    },
  })
}

const copyToken = async () => {
  try {
    await navigator.clipboard.writeText(issuedToken.value)
    MessagePlugin.success(t('settingsPages.personalIntegration.copied'))
  } catch {
    MessagePlugin.error(t('settingsPages.personalIntegration.copyFailed'))
  }
}

onMounted(refresh)
</script>

<style lang="less" scoped>
.personal-integration {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.block {
  border: 1px solid var(--td-component-stroke);
  border-radius: 10px;
  padding: 20px;
  background: var(--td-bg-color-container);
}

.block__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;

  h3 {
    margin: 0 0 6px;
    font-size: 16px;
    font-weight: 600;
  }

  p {
    margin: 0;
    font-size: 13px;
    line-height: 1.6;
    color: var(--td-text-color-secondary);
  }
}

.actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.key-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
  border-radius: 8px;
  background: var(--td-bg-color-secondarycontainer);
}

.key-row {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;

  code {
    font-family: var(--td-font-family-mono, monospace);
  }
}

.key-label {
  width: 72px;
  flex-shrink: 0;
  color: var(--td-text-color-secondary);
}

.empty {
  padding: 20px;
  text-align: center;
  font-size: 13px;
  color: var(--td-text-color-placeholder);
  border: 1px dashed var(--td-component-stroke);
  border-radius: 8px;
}

.snippet {
  margin: 0;
  padding: 14px 16px;
  border-radius: 8px;
  background: var(--td-bg-color-secondarycontainer);
  font-family: var(--td-font-family-mono, monospace);
  font-size: 13px;
  overflow-x: auto;
}

.hint {
  margin: 10px 0 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--td-text-color-secondary);
}

.token-warning {
  margin: 0 0 12px;
  font-size: 13px;
  color: var(--td-warning-color);
}

.token-box {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border-radius: 8px;
  background: var(--td-bg-color-secondarycontainer);

  code {
    flex: 1;
    word-break: break-all;
    font-family: var(--td-font-family-mono, monospace);
    font-size: 13px;
  }
}
</style>
