<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'

import {
  applyCardSplit,
  importReviewCards,
  previewCardAngles,
  previewCardSplit,
  type CreateReviewCardRequest,
  type GeneratedReviewCard,
  type ReviewCard,
  type ReviewGenerationPreview,
} from '@/api/review'

const props = defineProps<{
  visible: boolean
  mode: 'split' | 'angles'
  card: ReviewCard | null
}>()

const emit = defineEmits<{
  'update:visible': [visible: boolean]
  applied: []
}>()

const { t } = useI18n()

const dialogVisible = computed({
  get: () => props.visible,
  set: value => emit('update:visible', value),
})

const preview = ref<ReviewGenerationPreview | null>(null)
const selected = ref<number[]>([])
const loading = ref(false)
const applying = ref(false)
const error = ref('')
let loadSeq = 0

const title = computed(() =>
  props.mode === 'split' ? t('review.v3.actions.split') : t('review.v3.actions.angles'),
)
const hint = computed(() =>
  props.mode === 'split' ? t('review.v3.clusterPage.splitHint') : t('review.v3.clusterPage.anglesHint'),
)
const confirmLabel = computed(() =>
  props.mode === 'split' ? t('review.v3.clusterPage.applySplit') : t('review.v3.clusterPage.importAngles'),
)

function tagsOf(card: ReviewCard | null): string[] {
  if (!card) return []
  if (Array.isArray(card.tags)) return card.tags
  if (typeof card.tags === 'string') {
    try {
      const parsed = JSON.parse(card.tags)
      return Array.isArray(parsed) ? parsed.map(String) : []
    } catch {
      return []
    }
  }
  return []
}

async function loadPreview() {
  const card = props.card
  if (!card) return
  const seq = ++loadSeq
  loading.value = true
  error.value = ''
  preview.value = null
  selected.value = []
  try {
    const response = props.mode === 'split'
      ? await previewCardSplit(card.id, 2)
      : await previewCardAngles(card.id, 1)
    if (seq !== loadSeq) return
    preview.value = response.data || null
    selected.value = (preview.value?.cards ?? [])
      .map((item, index) => (item.duplicate ? -1 : index))
      .filter(index => index >= 0)
  } catch (err: any) {
    if (seq !== loadSeq) return
    error.value = err?.message || t('review.v3.clusterPage.aiDialogFailed')
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

watch(
  () => [props.visible, props.card?.id, props.mode],
  () => {
    if (props.visible && props.card) void loadPreview()
  },
)

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
    group_id: props.card?.group_id,
    question: card.question,
    answer: card.answer,
    explanation: card.explanation,
    tags: tagsOf(props.card),
    sources,
  }
}

async function apply() {
  if (!props.card || !preview.value || !selected.value.length || applying.value) return
  const cards = selected.value
    .map(index => preview.value?.cards[index])
    .filter((item): item is GeneratedReviewCard => Boolean(item))
    .map(toRequest)
  if (!cards.length) return
  applying.value = true
  error.value = ''
  try {
    if (props.mode === 'split') {
      const response = await applyCardSplit(props.card.id, cards)
      const count = response.data?.created?.length ?? cards.length
      MessagePlugin.success(t('review.v3.clusterPage.splitDone', { count }))
    } else {
      const response = await importReviewCards({
        group_id: props.card.group_id,
        cards,
      })
      MessagePlugin.success(t('review.v3.clusterPage.anglesImported', { count: response.data?.imported ?? cards.length }))
    }
    dialogVisible.value = false
    emit('applied')
  } catch (err: any) {
    error.value = err?.message || t('review.v3.clusterPage.aiDialogFailed')
  } finally {
    applying.value = false
  }
}
</script>

<template>
  <t-dialog
    v-model:visible="dialogVisible"
    :header="title"
    :footer="false"
    width="720px"
    data-testid="review-v3-ai-dialog"
  >
    <p class="ai-dialog__hint">{{ hint }}</p>

    <t-loading :loading="loading" size="small">
      <div v-if="error" class="ai-dialog__error">
        <t-alert theme="error" :message="error" />
        <t-button size="small" variant="outline" @click="loadPreview">{{ t('common.retry') }}</t-button>
      </div>
      <div v-else-if="preview?.cards?.length" class="ai-dialog__list">
        <label
          v-for="(item, index) in preview.cards"
          :key="`${item.question}-${index}`"
          class="ai-dialog__item"
          :class="{ 'is-disabled': item.duplicate }"
        >
          <t-checkbox
            :model-value="selected.includes(index)"
            :disabled="item.duplicate"
            @change="(value: boolean) => toggleCandidate(index, value)"
          />
          <div class="ai-dialog__copy">
            <div class="ai-dialog__question">{{ item.question }}</div>
            <div class="ai-dialog__answer">{{ item.answer }}</div>
            <div v-if="item.duplicate" class="ai-dialog__duplicate">
              {{ t('review.v3.clusterPage.duplicate') }}
            </div>
          </div>
        </label>
      </div>
      <div v-else-if="!loading" class="ai-dialog__empty">{{ t('review.v3.clusterPage.aiDialogEmpty') }}</div>
    </t-loading>

    <div class="ai-dialog__footer">
      <t-button variant="outline" @click="dialogVisible = false">{{ t('review.v3.clusterPage.cancel') }}</t-button>
      <t-button
        theme="primary"
        :loading="applying"
        :disabled="!selected.length || loading"
        @click="apply"
      >
        {{ confirmLabel }}
      </t-button>
    </div>
  </t-dialog>
</template>

<style lang="less" scoped>
.ai-dialog__hint {
  margin: 0 0 12px;
  font-size: 14px;
  line-height: 20px;
  color: var(--td-text-color-secondary);
}

.ai-dialog__list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 52vh;
  overflow-y: auto;
}

.ai-dialog__item {
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
}

.ai-dialog__copy {
  min-width: 0;
}

.ai-dialog__question {
  font-size: 14px;
  line-height: 20px;
  font-weight: 500;
  color: var(--td-text-color-primary);
}

.ai-dialog__answer {
  margin-top: 2px;
  font-size: 14px;
  line-height: 19px;
  color: var(--td-text-color-secondary);
  white-space: pre-wrap;
}

.ai-dialog__duplicate {
  margin-top: 4px;
  font-size: 14px;
  color: var(--td-warning-color);
}

.ai-dialog__empty {
  padding: 28px 0;
  text-align: center;
  font-size: 14px;
  color: var(--td-text-color-placeholder);
}

.ai-dialog__error {
  display: flex;
  align-items: center;
  gap: 10px;

  :deep(.t-alert) {
    flex: 1;
    min-width: 0;
  }
}

.ai-dialog__footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 16px;
}
</style>
