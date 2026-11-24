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
import { listKnowledgeBases, listKnowledgeFiles } from '@/api/knowledge-base'
import ReviewIcon from './ReviewIcon.vue'
import type { ReviewFlatGroup } from './types'

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
const kbLoading = ref(false)
const docOptions = ref<Array<{ label: string; value: string }>>([])
const docIds = ref<string[]>([])
const docsLoading = ref(false)
const form = reactive({ knowledgeBaseId: '', groupId: '', count: '5' })
const preview = ref<ReviewGenerationPreview | null>(null)
const selected = ref<number[]>([])
const generating = ref(false)
const importing = ref(false)
const error = ref('')
const editingIndex = ref(-1)
const candidateDraft = reactive({ question: '', answer: '', explanation: '' })

const countOptions = [5, 10, 20].map(value => ({ label: String(value), value: String(value) }))
const groupOptions = computed(() =>
  props.flatGroups.map(item => ({
    label: `${'　'.repeat(item.depth)}${item.group.name}`,
    value: item.group.id,
  })),
)

async function loadKnowledgeBases() {
  if (kbOptions.value.length) return
  kbLoading.value = true
  try {
    const response: any = await listKnowledgeBases()
    const list = response.data?.data || response.data || []
    kbOptions.value = (Array.isArray(list) ? list : []).map((kb: any) => ({
      label: kb.name || kb.id,
      value: kb.id,
    }))
  } catch {
    kbOptions.value = []
  } finally {
    kbLoading.value = false
  }
}

function reset() {
  form.knowledgeBaseId = ''
  form.groupId = props.defaultGroupId || ''
  form.count = '5'
  preview.value = null
  selected.value = []
  docOptions.value = []
  docIds.value = []
  error.value = ''
}

async function loadDocuments(kbId: string) {
  docOptions.value = []
  docIds.value = []
  if (!kbId) return
  docsLoading.value = true
  try {
    const response: any = await listKnowledgeFiles(kbId, { page: 1, page_size: 200, folder_recursive: true })
    const list = response.data || response?.data?.data || []
    docOptions.value = (Array.isArray(list) ? list : []).map((doc: any) => ({
      label: doc.title || doc.name || doc.file_name || doc.id,
      value: doc.id,
    }))
  } catch {
    docOptions.value = []
  } finally {
    docsLoading.value = false
  }
}

function toggleDoc(docId: string) {
  const index = docIds.value.indexOf(docId)
  if (index >= 0) docIds.value.splice(index, 1)
  else docIds.value.push(docId)
}

watch(
  () => props.visible,
  (visible) => {
    if (!visible) return
    reset()
    void loadKnowledgeBases()
  },
  { immediate: true },
)

watch(
  () => form.knowledgeBaseId,
  (kbId, previous) => {
    if (kbId === previous) return
    void loadDocuments(kbId)
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
      knowledge_ids: docIds.value.length ? docIds.value : undefined,
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

function startEditCandidate(index: number, card: GeneratedReviewCard) {
  editingIndex.value = index
  candidateDraft.question = card.question
  candidateDraft.answer = card.answer
  candidateDraft.explanation = card.explanation
}

function cancelEditCandidate() {
  editingIndex.value = -1
}

function saveEditCandidate(index: number) {
  const card = preview.value?.cards?.[index]
  if (!card) return
  const question = candidateDraft.question.trim()
  const answer = candidateDraft.answer.trim()
  if (!question || !answer) {
    MessagePlugin.warning(t('review.v3.generateEditRequired'))
    return
  }
  card.question = question
  card.answer = answer
  card.explanation = candidateDraft.explanation.trim()
  if (!card.duplicate && !selected.value.includes(index)) {
    toggleCandidate(index, true)
  }
  editingIndex.value = -1
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
  <section class="gen-page">
    <header class="gen-page__head">
      <h1>{{ t('review.generateTitle') }}</h1>
      <button type="button" class="rv-btn rv-btn--ghost rv-btn--sm" @click="dialogVisible = false">
        <ReviewIcon name="arrow-left" :size="13" />
        {{ t('review.v3.cardsPage.back') }}
      </button>
    </header>

    <div class="rv-props">
      <div class="rv-prop">
        <span class="rv-prop__key">流程</span>
        <span class="rv-tag rv-tag--blue">{{ t('review.v3.generateStepSelect') }}</span>
        <span class="rv-note">→</span>
        <span class="rv-tag rv-tag--green">{{ t('review.v3.generateStepPreview') }}</span>
        <span class="rv-note">→</span>
        <span class="rv-tag rv-tag--gray">{{ t('review.v3.generateStepImport') }}</span>
      </div>
    </div>

    <div class="gen-toolbar">
      <label class="gen-field">
        <span>{{ t('review.knowledgeBase') }}</span>
        <t-select
          v-model="form.knowledgeBaseId"
          :options="kbOptions"
          :loading="kbLoading"
          :placeholder="t('review.selectKnowledgeBase')"
          filterable
          data-testid="review-v3-generate-kb"
        />
      </label>
      <label class="gen-field">
        <span>{{ t('review.groupName') }}</span>
        <t-select
          v-model="form.groupId"
          :options="groupOptions"
          filterable
          data-testid="review-v3-generate-group"
        />
      </label>
      <label class="gen-field gen-field--count">
        <span>{{ t('review.cardCountLabel') }}</span>
        <t-select
          v-model="form.count"
          :options="countOptions"
          data-testid="review-v3-generate-count"
        />
      </label>
      <button type="button" class="rv-btn rv-btn--primary gen-toolbar__submit" :disabled="generating" data-testid="review-v3-generate-preview" @click="generate">
        {{ t('review.generatePreview') }}
      </button>
    </div>

    <t-alert v-if="error" theme="error" :message="error" class="gen-error" />

    <div class="gen-layout">
      <div class="gen-select">
        <div class="rv-h3">
          {{ t('review.v3.generateSelectContent') }}
          <span class="rv-note">{{ t('review.v3.generateSelectedDocs', { count: docIds.length }) }}</span>
        </div>
        <t-loading :loading="docsLoading" size="small">
          <div class="gen-doclist rv-block rv-wob-sm">
            <label
              v-for="doc in docOptions"
              :key="doc.value"
              class="gen-doc"
              :class="{ 'is-checked': docIds.includes(doc.value) }"
            >
              <input
                type="checkbox"
                :checked="docIds.includes(doc.value)"
                @change="toggleDoc(doc.value)"
              />
              <ReviewIcon :name="docIds.includes(doc.value) ? 'check' : 'file'" :size="15" :tilt="1" />
              <span class="gen-doc__name">{{ doc.label }}</span>
            </label>
            <p v-if="!docOptions.length" class="rv-note gen-doclist__empty">
              {{ form.knowledgeBaseId ? t('review.v3.generateNoDocs') : t('review.selectKnowledgeBase') }}
            </p>
          </div>
        </t-loading>
      </div>

      <div class="gen-result">
        <div class="rv-h3">
          {{ t('review.v3.generateResult') }}
          <span class="rv-note">
            {{ t('review.v3.generateCandidateCount', { total: preview?.cards?.length || 0, selected: selected.length }) }}
          </span>
        </div>
        <t-loading :loading="generating" size="small">
          <div v-if="preview?.cards?.length" class="gen-list" data-testid="review-v3-generate-list">
            <div
              v-for="(card, index) in preview.cards"
              :key="`${card.question}-${index}`"
              class="gen-item rv-wob-sm"
              :class="{
                'is-selected': selected.includes(index),
                'is-disabled': card.duplicate,
                'is-editing': editingIndex === index,
              }"
              @click="editingIndex !== index && !card.duplicate && toggleCandidate(index, !selected.includes(index))"
            >
              <input
                type="checkbox"
                :checked="selected.includes(index)"
                :disabled="card.duplicate"
                @click.stop
                @change="(event: Event) => toggleCandidate(index, Boolean((event.target as HTMLInputElement).checked))"
              />
              <ReviewIcon :name="selected.includes(index) ? 'check' : 'box'" :size="16" />
              <div class="gen-item__copy">
                <template v-if="editingIndex === index">
                  <div class="gen-edit" @click.stop>
                    <label>
                      <span>{{ t('review.v3.cardsPage.createQuestion') }}</span>
                      <textarea v-model="candidateDraft.question" rows="2" />
                    </label>
                    <label>
                      <span>{{ t('review.v3.cardsPage.createAnswer') }}</span>
                      <textarea v-model="candidateDraft.answer" rows="2" />
                    </label>
                    <label>
                      <span>{{ t('review.v3.cardsPage.createExplanation') }}</span>
                      <textarea v-model="candidateDraft.explanation" rows="2" />
                    </label>
                    <div class="gen-edit__actions">
                      <button type="button" class="rv-btn rv-btn--primary rv-btn--sm" @click="saveEditCandidate(index)">
                        {{ t('review.save') }}
                      </button>
                      <button type="button" class="rv-btn rv-btn--ghost rv-btn--sm" @click="cancelEditCandidate">
                        {{ t('review.cancel') }}
                      </button>
                    </div>
                  </div>
                </template>
                <template v-else>
                  <div class="gen-item__question">{{ card.question }}</div>
                  <div class="gen-item__answer">{{ card.answer }}</div>
                  <div class="gen-item__meta">
                    <span v-if="card.source_title" class="rv-note">{{ card.source_title }}</span>
                    <span v-if="card.duplicate" class="rv-tag rv-tag--orange">{{ t('review.duplicate') }}</span>
                    <span v-else class="rv-tag rv-tag--green">{{ t('review.v3.generateRecommended') }}</span>
                    <button
                      type="button"
                      class="rv-btn rv-btn--text rv-btn--sm gen-item__edit"
                      @click.stop="startEditCandidate(index, card)"
                    >
                      <ReviewIcon name="edit" :size="13" />
                      {{ t('review.v3.generateEdit') }}
                    </button>
                  </div>
                </template>
              </div>
            </div>
          </div>
          <p v-else-if="!generating" class="gen-empty">
            <ReviewIcon name="book" :size="28" />
            <span>{{ t('review.noGeneratedCards') }}</span>
          </p>
        </t-loading>
      </div>
    </div>

    <p class="rv-note gen-scope-hint">{{ t('review.v3.generateScopeHint') }}</p>

    <div class="gen-footer">
      <button type="button" class="rv-btn rv-btn--primary" :disabled="!selected.length || generating || importing" data-testid="review-v3-generate-import" @click="importSelected">
        {{ t('review.importSelected') }} ({{ selected.length }})
      </button>
      <button type="button" class="rv-btn rv-btn--ghost" @click="dialogVisible = false">{{ t('review.cancel') }}</button>
    </div>
  </section>
</template>

<style lang="less" scoped>
.gen-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 4px 0 24px;
}

.gen-page__head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--rvn-border);

  h2 {
    margin: 0;
    font-size: 22px;
    line-height: 30px;
    font-weight: 700;
  }
}

.gen-page__body {
  width: 100%;
}

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
    font-size: 14px;
    font-weight: 500;
    color: var(--td-text-color-primary);
  }

  &__answer {
    margin-top: 4px;
    font-size: 14px;
    line-height: 20px;
    color: var(--td-text-color-secondary);
  }

  &__meta {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 6px;
    font-size: 14px;
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
  font-size: 14px;
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

<style lang="less" scoped>
/* Paper-journal layout for knowledge-base card generation. */
.gen-page {
  gap: 0;
  padding: 0 0 24px;
}

.gen-page__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 0;
  margin-bottom: 22px;
  border-bottom: 0;

  h1 {
    margin: 0;
    font-size: 34px;
    font-weight: 700;
    line-height: 1.2;
    letter-spacing: -0.02em;
    color: var(--rvn-ink);
  }
}

.gen-toolbar {
  display: grid;
  grid-template-columns: minmax(220px, 1.4fr) minmax(180px, 1fr) 130px auto;
  gap: 16px;
  align-items: end;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--rvn-line);
}

.gen-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;

  > span {
    font-size: 14px;
    font-weight: 600;
    color: var(--rvn-ink);
  }

  :deep(.t-select) {
    width: 100%;
  }
}

.gen-toolbar__submit {
  height: 40px;
  white-space: nowrap;
}

.gen-layout {
  display: grid;
  grid-template-columns: 38% minmax(0, 1fr);
  gap: 24px;
  align-items: stretch;
  margin-top: 20px;
}

.gen-select,
.gen-result {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}

.gen-select :deep(.t-loading),
.gen-result :deep(.t-loading) {
  display: flex;
  flex: 1;
  flex-direction: column;
}

.gen-select :deep(.t-loading__parent),
.gen-result :deep(.t-loading__parent) {
  display: flex;
  flex: 1;
  flex-direction: column;
}

.gen-doclist {
  display: flex;
  flex: none;
  flex-direction: column;
  gap: 1px;
  height: 360px;
  min-height: 360px;
  max-height: 360px;
  overflow-y: auto;
}

.gen-doc {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 7px 8px;
  font-size: 14px;
  color: var(--rvn-ink-soft);
  cursor: pointer;
  border-radius: 4px;

  input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }

  &:hover {
    background: var(--rvn-paper);
  }

  &.is-checked {
    color: var(--rvn-brand-deep);
    background: var(--rvn-brand-bg);
  }
}

.gen-doc__name {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.gen-doclist__empty {
  padding: 12px 8px;
  margin: 0;
}

.gen-select__hint {
  margin-top: 10px;
}

.gen-scope-hint {
  margin: 12px 0 0;
}

.gen-list {
  display: flex;
  flex: none;
  flex-direction: column;
  gap: 10px;
  height: 360px;
  min-height: 360px;
  max-height: 360px;
  margin-top: 0;
  padding-right: 4px;
  overflow-y: auto;
}

.gen-item {
  display: flex;
  gap: 9px;
  align-items: flex-start;
  padding: 14px 16px;
  background: var(--rvn-paper);
  border: 1px solid var(--rvn-line);
  border-style: solid;

  input[type='checkbox'] {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }

  &.is-selected {
    border-color: var(--rvn-brand-line);
    background: var(--rvn-brand-bg);
  }

  &.is-disabled {
    background: var(--rvn-paper);
    border-style: dashed;
  }

  :deep(.t-checkbox) {
    margin-top: 2px;
  }
}

.gen-item__question {
  font-size: 14px;
  font-weight: 600;
  color: var(--rvn-ink);
}

.gen-item__answer {
  margin-top: 4px;
  font-size: 14px;
  line-height: 1.6;
  color: var(--rvn-ink-soft);
}

.gen-item__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin-top: 6px;
}

.gen-item__edit {
  margin-left: auto;
}

.gen-item.is-editing {
  cursor: default;
  background: var(--rvn-paper);
  border-style: solid;
}

.gen-edit {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;

  label {
    display: flex;
    flex-direction: column;
    gap: 5px;

    span {
      font-size: 14px;
      font-weight: 600;
      color: var(--rvn-ink-soft);
    }
  }

  textarea {
    width: 100%;
    padding: 8px 10px;
    font: inherit;
    font-size: 14px;
    line-height: 1.55;
    color: var(--rvn-ink);
    resize: vertical;
    background: var(--rvn-paper);
    border: 1px solid var(--rvn-line-strong);
    border-radius: 4px;
    outline: none;

    &:focus {
      border-color: var(--rvn-accent);
      box-shadow: 0 0 0 2px var(--rvn-accent-soft);
    }
  }
}

.gen-edit__actions {
  display: flex;
  gap: 8px;
}

.gen-empty {
  display: flex;
  flex: none;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 360px;
  min-height: 360px;
  max-height: 360px;
  padding: 44px 16px;
  color: var(--rvn-muted);
  background: var(--rvn-surface);
  border: 1px dashed var(--rvn-line-strong);
  border-radius: 7px 6px 8px 7px / 6px 8px 7px 6px;
}

.gen-footer {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-top: 26px;
  padding-top: 18px;
  border-top: 1px solid var(--rvn-line);
}

@media (max-width: 960px) {
  .gen-layout {
    grid-template-columns: minmax(0, 1fr);
  }

  .gen-toolbar {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .gen-toolbar__submit {
    width: 100%;
  }
}
</style>
