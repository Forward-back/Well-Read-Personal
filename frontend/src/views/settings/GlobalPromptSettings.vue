<template>
  <section class="block">
    <header class="block__header">
      <div>
        <h3>{{ $t('settingsPages.globalPrompt.title') }}</h3>
        <p>{{ $t('settingsPages.globalPrompt.desc') }}</p>
      </div>
      <div class="actions">
        <t-button variant="outline" theme="default" :loading="loading" @click="handleReset">{{ $t('settingsPages.globalPrompt.reset') }}</t-button>
        <t-button theme="primary" :loading="saving" :disabled="loading" @click="handleSave">{{ $t('settingsPages.globalPrompt.save') }}</t-button>
      </div>
    </header>

    <t-loading :loading="loading">
      <t-textarea
        v-model="prompt"
        class="prompt-editor"
        :autosize="{ minRows: 10, maxRows: 24 }"
        :placeholder="$t('settingsPages.globalPrompt.loadingPlaceholder')"
      />
      <p class="hint">
        {{ $t('settingsPages.globalPrompt.hint') }}
        <span v-if="isCustom" class="custom-badge">{{ $t('settingsPages.globalPrompt.customBadge') }}</span>
      </p>
    </t-loading>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import { getPromptTemplates, saveAgentSystemPrompt, type PromptTemplate } from '@/api/settings'

const { t } = useI18n()
const loading = ref(false)
const saving = ref(false)
const prompt = ref('')
const isCustom = ref(false)

function displayContent(templates?: PromptTemplate[]): string {
  if (!templates || templates.length === 0) return ''
  return (templates.find((item) => item.default) ?? templates[0]).content || ''
}

async function load() {
  loading.value = true
  try {
    const response = await getPromptTemplates()
    const templates = response?.data?.agent_system_prompt
    prompt.value = displayContent(templates)
    isCustom.value = templates?.length === 1 && templates[0]?.id === 'personal_global_override'
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('settingsPages.globalPrompt.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function persist(value: string) {
  saving.value = true
  try {
    const response = await saveAgentSystemPrompt(value)
    isCustom.value = Boolean(response?.data?.is_custom)
    if (!value.trim()) {
      await load()
    } else {
      prompt.value = response?.data?.agent_system_prompt || value
    }
    MessagePlugin.success(value.trim() ? t('settingsPages.globalPrompt.saved') : t('settingsPages.globalPrompt.defaultRestored'))
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('settingsPages.globalPrompt.saveFailed'))
  } finally {
    saving.value = false
  }
}

function handleSave() {
  void persist(prompt.value)
}

function handleReset() {
  const dialog = DialogPlugin.confirm({
    header: t('settingsPages.globalPrompt.resetTitle'),
    body: t('settingsPages.globalPrompt.resetBody'),
    onConfirm: async () => {
      dialog.destroy()
      await persist('')
    },
    onCancel: () => dialog.destroy(),
  })
}

onMounted(load)
</script>

<style scoped>
.block {
  border: 1px solid var(--td-component-border);
  border-radius: 8px;
  padding: 20px;
  background: var(--td-bg-color-container);
}

.block__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.block__header h3 {
  margin: 0 0 6px;
  font-size: 16px;
}

.block__header p {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.prompt-editor {
  width: 100%;
}

.hint {
  margin: 8px 0 0;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}

.custom-badge {
  margin-left: 8px;
  color: var(--td-brand-color);
}
</style>
