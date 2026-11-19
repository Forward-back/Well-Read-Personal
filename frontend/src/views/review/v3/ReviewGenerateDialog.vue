<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'

import {
  importReviewCards,
  previewReviewGeneration,
  type CreateReviewCardRequest,
  type GeneratedReviewCard,
  type ReviewGenerationPreview,
} from '@/api/review'
import { listKnowledgeBases } from '@/api/knowledge-base'
import type { ReviewFlatGroup } from './types'
import sparklesIcon from '@/assets/icons/static/sparkles.png'
import bookIcon from '@/assets/icons/static/book.png'

const props = defineProps<{
  visible: boolean
  flatGroups: ReviewFlatGroup[]
  defaultGroupId: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void
  (e: 'imported'): void
}>()

const { t } = useI18n()

const dialogVisible = computed({
  get: () => props.visible,
  set: value => emit('update:visible', value),
})

const kbOptions = ref<Array<{ label: string; value: string }>>([])
const form = reactive({ knowledgeBaseId: '', groupId: '', count: '5' })
const preview = ref<ReviewGenerationPreview | null>(null)
const selected = ref<number[]>([])
const generating = ref(false)
const importing = ref(false)
const error = ref('')

const countOptions = [5, 10, 20].map(value => ({ label: String(value), value: String(value) }))
const groupOptions = computed(() =>
  props.flatGroups.map(item => ({
    label: `${'　'.repeat(item.depth)}${item.group.name}`,
    value: item.group.id,
  })),
)

async function loadKnowledgeBases() {
  if (kbOptions.value.length) return
  try {
    const response: any = await listKnowledgeBases()
    const list = response.data?.data || response.data || []
    kbOptions.value = (Array.isArray(list) ? list : []).map((kb: any) => ({
      label: kb.name || kb.id,
      value: kb.id,
    }))
  } catch {
    kbOptions.value = []
  }
}

function reset() {
  form.knowledgeBaseId = ''
  form.groupId = props.defaultGroupId || ''
  form.count = '5'
  preview.value = null
  selected.value = []
  error.value = ''
}

watch(
  () => props.visible,
  (visible) => {
    if (!visible) return
    reset()
    void loadKnowledgeBases()
  },
)

async function generate() {
  if (!form.knowledgeBaseId) {
    MessagePlugin.warning(t('review.selectKnowledgeBase'))
    return
  }
  generating.value = true
  error.value = ''
  try {
    const response = await previewReviewGeneration({
      knowledge_base_id: form.knowledgeBaseId,
      group_id: form.groupId || undefined,
      count: Number(form.count),
    })
    preview.value = response.data || null
    selected.value = (preview.value?.cards || [])
      .map((item, index) => (item.duplicate ? -1 : index))
      .filter(index => index >= 0)
  } catch (err: any) {
    error.value = err?.message || t('review.generateFailed')
  } finally {
    generating.value = false
  }
}

function toggleCandidate(index: number, checked: boolean) {
  const next = new Set(selected.value)
  if (checked) next.add(index)
  else next.delete(index)
  selected.value = [...next].sort((a, b) => a - b)
}

function toRequest(card: GeneratedReviewCard): CreateReviewCardRequest {
  const sources = card.sources?.length
    ? card.sources
    : card.source_knowledge_id
      ? [{
          source_type: 'knowledge_chunk',
          knowledge_id: card.source_knowledge_id,
          chunk_id: card.source_chunk_id,
          excerpt: card.source_excerpt,
        }]
      : []
  return {
    group_id: form.groupId || undefined,
    question: card.question,
    answer: card.answer,
    explanation: card.explanation,
    tags: [],
    sources,
  }
}

async function importSelected() {
  if (!preview.value || !selected.value.length || importing.value) return
  const cards = selected.value
    .map(index => preview.value?.cards[index])
    .filter((item): item is GeneratedReviewCard => Boolean(item))
    .map(toRequest)
  importing.value = true
  error.value = ''
  try {
    const response = await importReviewCards({ group_id: form.groupId || undefined, cards })
    MessagePlugin.success(t('review.importedCount', { count: response.data?.imported ?? cards.length }))
    dialogVisible.value = false
    emit('imported')
  } catch (err: any) {
    error.value = err?.message || t('review.importFailed')
  } finally {
    importing.value = false
  }
}
</script>

<template>
  <t-dialog
    v-model:visible="dialogVisible"
    :footer="false"
    width="760px"
    data-testid="review-v3-generate-dialog"
  >
    <template #header>
      <div class="gen-head"><img :src="sparklesIcon" alt="" />{{ t('review.generateTitle') }}</div>
    </template>
    <div class="gen-form">
      <t-select
        v-model="form.knowledgeBaseId"
        :label="t('review.knowledgeBase')"
        :options="kbOptions"
        :placeholder="t('review.selectKnowledgeBase')"
        filterable
        data-testid="review-v3-generate-kb"
      />
      <t-select
        v-model="form.groupId"
        :label="t('review.groupName')"
        :options="groupOptions"
        filterable
        data-testid="review-v3-generate-group"
      />
      <t-select
        v-model="form.count"
        :label="t('review.cardCountLabel')"
        :options="countOptions"
        data-testid="review-v3-generate-count"
      />
      <t-button theme="primary" :loading="generating" data-testid="review-v3-generate-preview" @click="generate">
        {{ t('review.generatePreview') }}
      </t-button>
    </div>

    <t-alert v-if="error" theme="error" :message="error" class="gen-error" />

    <t-loading :loading="generating" size="small">
      <div v-if="preview?.cards?.length" class="gen-list" data-testid="review-v3-generate-list">
        <label
          v-for="(card, index) in preview.cards"
          :key="`${card.question}-${index}`"
          class="gen-item"
          :class="{ 'is-disabled': card.duplicate }"
        >
          <t-checkbox
            :checked="selected.includes(index)"
            :disabled="card.duplicate"
            @change="(value: boolean) => toggleCandidate(index, Boolean(value))"
          />
          <div class="gen-item__copy">
            <div class="gen-item__question">{{ card.question }}</div>
            <div class="gen-item__answer">{{ card.answer }}</div>
            <div class="gen-item__meta">
              <span v-if="card.source_title">{{ card.source_title }}</span>
              <t-tag v-if="card.duplicate" size="small" theme="warning" variant="light">{{ t('review.duplicate') }}</t-tag>
            </div>
          </div>
        </label>
      </div>
      <p v-else-if="!generating" class="gen-empty">
        <img :src="bookIcon" alt="" />
        <span>{{ t('review.noGeneratedCards') }}</span>
      </p>
    </t-loading>

    <div class="gen-footer">
      <t-button variant="outline" @click="dialogVisible = false">{{ t('review.cancel') }}</t-button>
      <t-button
        theme="primary"
        :loading="importing"
        :disabled="!selected.length || generating"
        data-testid="review-v3-generate-import"
        @click="importSelected"
      >
        {{ t('review.importSelected') }} ({{ selected.length }})
      </t-button>
    </div>
  </t-dialog>
</template>

<style lang="less" scoped>
.gen-form {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr)) auto;
  gap: 12px;
  align-items: end;
}

.gen-error {
  margin-top: 12px;
}

.gen-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 52vh;
  margin-top: 14px;
  overflow-y: auto;
}

.gen-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-default);
  cursor: pointer;

  &.is-disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  &__copy {
    min-width: 0;
    flex: 1;
  }

  &__question {
    font-size: 13px;
    font-weight: 500;
    color: var(--td-text-color-primary);
  }

  &__answer {
    margin-top: 4px;
    font-size: 12px;
    line-height: 20px;
    color: var(--td-text-color-secondary);
  }

  &__meta {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 6px;
    font-size: 12px;
    color: var(--td-text-color-placeholder);
  }
}

.gen-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  margin: 0;
  padding: 28px;
  font-size: 13px;
  color: var(--td-text-color-placeholder);
  text-align: center;

  img {
    width: 40px;
    height: 40px;
    object-fit: contain;
  }
}

.gen-head {
  display: inline-flex;
  align-items: center;
  gap: 8px;

  img {
    width: 22px;
    height: 22px;
    object-fit: contain;
  }
}

.gen-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}

@media (max-width: 760px) {
  .gen-form {
    grid-template-columns: 1fr;
  }
}
</style>
