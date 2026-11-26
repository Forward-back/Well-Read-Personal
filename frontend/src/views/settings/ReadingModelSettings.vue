<template>
  <div class="reading-model-settings">
    <header class="block__header">
      <div>
        <h3>{{ $t('settingsPages.readingModel.title') }}</h3>
        <p>{{ $t('settingsPages.readingModel.desc') }}</p>
      </div>
    </header>

    <div class="form-row">
      <label class="form-label">{{ $t('settingsPages.readingModel.fallbackLabel') }}</label>
      <t-select
        v-model="selectedFallbackId"
        :options="allOptions"
        :loading="loading"
        :placeholder="allOptions.length ? $t('settingsPages.readingModel.fallbackPlaceholder') : $t('settingsPages.readingModel.noModels')"
        clearable
        class="model-select"
      />
    </div>
    <div class="form-row">
      <label class="form-label">{{ $t('settingsPages.readingModel.extractionLabel') }}</label>
      <t-select
        v-model="selectedExtractionId"
        :options="allOptions"
        :loading="loading"
        :placeholder="$t('settingsPages.readingModel.optionalPlaceholder')"
        clearable
        class="model-select"
      />
    </div>
    <div class="form-row">
      <label class="form-label">{{ $t('settingsPages.readingModel.synthesisLabel') }}</label>
      <t-select
        v-model="selectedSynthesisId"
        :options="thinkingOptions"
        :loading="loading"
        :placeholder="$t('settingsPages.readingModel.synthesisPlaceholder')"
        clearable
        class="model-select"
      />
    </div>
    <div class="form-row">
      <t-button theme="primary" :loading="saving" :disabled="!dirty || saving" @click="save">{{ $t('settingsPages.readingModel.save') }}</t-button>
    </div>
    <p v-if="currentFallbackId" class="current">{{ $t('settingsPages.readingModel.currentFallback', { model: modelName(currentFallbackId) }) }}</p>
    <p v-else class="current current--empty">{{ $t('settingsPages.readingModel.currentFallbackEmpty') }}</p>
    <p v-if="currentExtractionId" class="current">{{ $t('settingsPages.readingModel.currentExtraction', { model: modelName(currentExtractionId) }) }}</p>
    <p v-if="currentSynthesisId" class="current">{{ $t('settingsPages.readingModel.currentSynthesis', { model: modelName(currentSynthesisId) }) }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import { listModels, type ModelConfig } from '@/api/model'
import { getReadingConfig, updateReadingConfig } from '@/api/reading'
import { modelSupportsThinking } from '@/utils/thinkingControl'

const { t } = useI18n()
const loading = ref(false)
const saving = ref(false)
const models = ref<ModelConfig[]>([])
const currentFallbackId = ref('')
const currentExtractionId = ref('')
const currentSynthesisId = ref('')
const selectedFallbackId = ref('')
const selectedExtractionId = ref('')
const selectedSynthesisId = ref('')

const questionModels = computed(() =>
  models.value.filter((m) => m.id && (m.type || '').toLowerCase() === 'knowledgeqa'),
)
const thinkingModels = computed(() => questionModels.value.filter((m) => modelSupportsThinking(m)))
const allOptions = computed(() =>
  questionModels.value.map((m) => ({ label: m.display_name || m.name, value: m.id as string })),
)
const thinkingOptions = computed(() =>
  thinkingModels.value.map((m) => ({ label: m.display_name || m.name, value: m.id as string })),
)
const modelName = (id: string) => {
  const hit = models.value.find((m) => m.id === id)
  return hit ? hit.display_name || hit.name : id
}
const dirty = computed(
  () =>
    (selectedFallbackId.value || '') !== currentFallbackId.value ||
    (selectedExtractionId.value || '') !== currentExtractionId.value ||
    (selectedSynthesisId.value || '') !== currentSynthesisId.value,
)

const load = async () => {
  loading.value = true
  try {
    const [modelList, config] = await Promise.all([listModels(), getReadingConfig()])
    models.value = modelList || []
    const data = config?.data
    currentFallbackId.value = data?.reading_model_id || ''
    currentExtractionId.value = data?.reading_extraction_model_id || ''
    currentSynthesisId.value = data?.reading_synthesis_model_id || ''
    selectedFallbackId.value = currentFallbackId.value
    selectedExtractionId.value = currentExtractionId.value
    selectedSynthesisId.value = currentSynthesisId.value
  } catch {
    MessagePlugin.error(t('settingsPages.readingModel.loadFailed'))
  } finally {
    loading.value = false
  }
}

const save = async () => {
  saving.value = true
  try {
    const result = await updateReadingConfig({
      reading_model_id: selectedFallbackId.value || '',
      reading_extraction_model_id: selectedExtractionId.value || '',
      reading_synthesis_model_id: selectedSynthesisId.value || '',
    })
    const data = result?.data
    currentFallbackId.value = data?.reading_model_id || ''
    currentExtractionId.value = data?.reading_extraction_model_id || ''
    currentSynthesisId.value = data?.reading_synthesis_model_id || ''
    MessagePlugin.success(t('settingsPages.readingModel.saved'))
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('settingsPages.readingModel.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style lang="less" scoped>
.reading-model-settings {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.block__header {
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

.form-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.form-label {
  flex: none;
  width: 112px;
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.model-select {
  width: 360px;
}

.current {
  margin: 0;
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.current--empty {
  color: var(--td-warning-color);
}
</style>
