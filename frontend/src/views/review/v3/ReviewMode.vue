<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'

import {
  getReviewCard,
  submitReview,
  type ReviewCard,
  type ReviewCardDetail,
  type ReviewRating,
  type ReviewSessionState,
} from '@/api/review'
import { getKnowledgeDetails } from '@/api/knowledge-base'
import { createChatMarkdownRenderer, renderChatMarkdown } from '@/utils/chatMarkdownRenderer'
import { sanitizeMarkdownHTML } from '@/utils/security'
import ReviewLevelTag from './ReviewLevelTag.vue'
import ReviewIcon from './ReviewIcon.vue'
import { fallbackVulnerability, holdDays, scheduledIntervalDays } from './vulnerability'

const props = defineProps<{
  state: ReviewSessionState
  queueLabel: string
  streakDays: number
}>()

const emit = defineEmits<{
  close: []
  completed: []
}>()

const { t } = useI18n()
const router = useRouter()

const markdownRenderer = createChatMarkdownRenderer()

const remaining = ref<ReviewCard[]>([])
const ratedCount = ref(0)
const counts = reactive({ forgot: 0, fuzzy: 0, remembered: 0, easy: 0 })
const showingAnswer = ref(false)
const pendingRating = ref<ReviewRating | ''>('')
const finished = ref(false)
const detail = ref<ReviewCardDetail | null>(null)
const ratingError = ref('')
const sourceOpening = ref(false)
let detailRequestSeq = 0

const currentCard = computed(() => remaining.value[0] ?? null)
const total = computed(() => props.state.session.planned_count)
const progressCurrent = computed(() => Math.min(ratedCount.value + 1, Math.max(total.value, 1)))

const vulnerability = computed(() =>
  detail.value?.card.vulnerability
  || currentCard.value?.vulnerability
  || fallbackVulnerability(currentCard.value),
)
const isSevere = computed(() => vulnerability.value === 'severe')
const isWarning = computed(() => vulnerability.value === 'warning')

const severeText = computed(() => {
  const card = currentCard.value
  if (!card) return ''
  if (card.forget_streak >= 2) return t('review.v3.severeStreak', { count: card.forget_streak })
  if (card.lapses > 0) return t('review.v3.severeLapse', { count: card.lapses })
  return t('review.v3.severeHold')
})

const intervals = computed(() => ({
  hold: holdDays(currentCard.value?.stability),
  interval: scheduledIntervalDays(currentCard.value?.due_at, currentCard.value?.last_reviewed_at),
}))

const answerHTML = computed(() => renderMarkdown(currentCard.value?.answer))
const explanationHTML = computed(() => renderMarkdown(currentCard.value?.explanation))

const cardTags = computed(() => {
  const raw = currentCard.value?.tags
  if (Array.isArray(raw)) return raw.filter(Boolean)
  if (typeof raw === 'string' && raw.trim()) {
    return raw.split(',').map(tag => tag.trim()).filter(Boolean)
  }
  return []
})

const sources = computed(() =>
  (detail.value?.sources ?? []).filter(source => source.status === 'active' && source.knowledge_id),
)
const primarySource = computed(() =>
  sources.value.find(source => source.is_primary) || sources.value[0] || null,
)
const sourceExcerpt = computed(() => primarySource.value?.excerpt?.trim() || '')

function renderMarkdown(value: string | undefined) {
  if (!value) return ''
  return renderChatMarkdown(value, {
    renderer: markdownRenderer,
    escapeMarkdown: markdown => markdown,
    sanitizeHtml: sanitizeMarkdownHTML,
  })
}

async function loadDetail() {
  const card = currentCard.value
  detail.value = null
  if (!card) return
  const seq = ++detailRequestSeq
  try {
    const response = await getReviewCard(card.id)
    if (seq !== detailRequestSeq) return
    detail.value = response.data || null
  } catch {
    if (seq === detailRequestSeq) detail.value = null
  }
}

function resetFromState(state: ReviewSessionState) {
  remaining.value = [...(state.cards ?? [])]
  ratedCount.value = state.session.completed_count ?? 0
  counts.forgot = state.session.forgot_count ?? 0
  counts.fuzzy = state.session.fuzzy_count ?? 0
  counts.remembered = state.session.remembered_count ?? 0
  counts.easy = state.session.easy_count ?? 0
  showingAnswer.value = false
  ratingError.value = ''
  finished.value = remaining.value.length === 0
  if (!finished.value) void loadDetail()
}

watch(() => props.state, resetFromState, { immediate: true })

async function rate(rating: ReviewRating) {
  const card = currentCard.value
  if (!card || !showingAnswer.value || pendingRating.value) return
  pendingRating.value = rating
  ratingError.value = ''
  try {
    await submitReview({
      card_id: card.id,
      session_id: props.state.session.id,
      rating,
      client_request_id: crypto.randomUUID(),
    })
    counts[rating] += 1
    ratedCount.value += 1
    advance()
  } catch (err: any) {
    ratingError.value = err?.message || t('review.actionFailed')
  } finally {
    pendingRating.value = ''
  }
}

function advance() {
  remaining.value = remaining.value.slice(1)
  showingAnswer.value = false
  ratingError.value = ''
  if (!remaining.value.length) {
    finished.value = true
    emit('completed')
    return
  }
  void loadDetail()
}

function skipCard() {
  if (!currentCard.value) return
  advance()
}

function showAnswer() {
  if (!currentCard.value || finished.value) return
  showingAnswer.value = true
}

function suspend() {
  emit('close')
}

async function openSource() {
  const source = primarySource.value
  if (!source?.knowledge_id || sourceOpening.value) return
  sourceOpening.value = true
  try {
    const response: any = await getKnowledgeDetails(source.knowledge_id)
    const doc = response?.data || response
    const knowledgeBaseId = doc?.knowledge_base_id
    if (!knowledgeBaseId) throw new Error('missing knowledge base')
    const query: Record<string, string> = { knowledge_id: source.knowledge_id }
    if (source.chunk_id) query.chunk_id = source.chunk_id
    const href = router.resolve({
      path: `/platform/knowledge-bases/${knowledgeBaseId}`,
      query,
    }).href
    window.open(href, '_blank', 'noopener')
  } catch {
    MessagePlugin.warning(t('review.v3.sourceUnavailable'))
  } finally {
    sourceOpening.value = false
  }
}

function onKeydown(event: KeyboardEvent) {
  const target = event.target as HTMLElement | null
  if (target && (['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName) || target.isContentEditable)) return
  if (event.code === 'Space' || event.code === 'Enter') {
    if (!showingAnswer.value && !finished.value && currentCard.value) {
      event.preventDefault()
      showAnswer()
    }
    return
  }
  if (event.key === 'Escape') {
    event.preventDefault()
    suspend()
    return
  }
  if (!showingAnswer.value || finished.value || pendingRating.value) return
  if (event.key === '1') void rate('forgot')
  else if (event.key === '2') void rate('fuzzy')
  else if (event.key === '3') void rate('remembered')
  else if (event.key === '4') void rate('easy')
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="review-mode rv-vuln-scope" data-testid="review-v3-mode">
    <div class="review-mode__inner">
      <div class="rv-crumb">正在复习：<b>{{ queueLabel }}</b></div>

      <header class="review-mode__head">
        <span class="review-mode__progress" data-testid="review-v3-mode-progress">
          {{ t('review.v3.modeProgress', { current: progressCurrent, total }) }}
        </span>
        <div class="review-mode__bar">
          <i :style="{ width: `${total ? Math.min(100, Math.round((ratedCount / total) * 100)) : 0}%` }" />
        </div>
        <span class="rv-note">{{ t('review.v3.streakDays', { count: streakDays }) }}</span>
        <button type="button" class="rv-btn rv-btn--ghost rv-btn--sm" data-testid="review-v3-suspend" @click="suspend">
          <ReviewIcon name="arrow-left" :size="13" />
          {{ t('review.v3.suspendExit') }}
        </button>
      </header>

      <section v-if="finished" class="review-mode__done" data-testid="review-v3-mode-done">
        <div class="review-mode__done-icon">
          <ReviewIcon name="check" :size="34" />
        </div>
        <h2>{{ t('review.v3.modeDoneTitle') }}</h2>
        <p>{{ t('review.v3.modeDoneHint', { count: ratedCount }) }}</p>
        <div class="review-mode__counts">
          <div class="review-mode__count is-forgot">
            <strong>{{ counts.forgot }}</strong>
            <span>{{ t('review.v3.rating.forgot') }}</span>
          </div>
          <div class="review-mode__count is-fuzzy">
            <strong>{{ counts.fuzzy }}</strong>
            <span>{{ t('review.v3.rating.fuzzy') }}</span>
          </div>
          <div class="review-mode__count is-remembered">
            <strong>{{ counts.remembered }}</strong>
            <span>{{ t('review.v3.rating.remembered') }}</span>
          </div>
          <div class="review-mode__count is-easy">
            <strong>{{ counts.easy }}</strong>
            <span>{{ t('review.v3.rating.easy') }}</span>
          </div>
        </div>
        <div class="review-mode__done-meta">
          <div>
            <span>{{ t('review.stats.streak') }}</span>
            <strong>{{ t('review.v3.streakDays', { count: streakDays }) }}</strong>
          </div>
          <div>
            <span>{{ t('review.v3.modeNextDue') }}</span>
            <strong>{{ t('review.v3.modeNextDueValue') }}</strong>
          </div>
        </div>
        <button type="button" class="rv-btn rv-btn--primary" @click="suspend">
          {{ t('review.v3.backReviewHome') }}
        </button>
      </section>

      <section v-else-if="currentCard" class="review-mode__card">
        <article class="qcard rv-wob">
          <div class="qcard__tags">
            <span class="rv-tag rv-tag--blue">{{ queueLabel }}</span>
            <span v-if="isSevere" class="rv-tag rv-tag--red" data-testid="review-v3-severe-banner">{{ severeText }}</span>
            <span v-else-if="isWarning" data-testid="review-v3-warning-tag">
              <ReviewLevelTag level="warning" />
            </span>
            <span v-for="tag in cardTags" :key="tag" class="rv-tag rv-tag--gray">#{{ tag }}</span>
          </div>
          <div class="qcard__question" data-testid="review-v3-question">{{ currentCard.question }}</div>
          <div v-if="isSevere || isWarning" class="qcard__params" data-testid="review-v3-params">
            <div>
              <span>{{ t('review.v3.holdMetric') }}</span>
              <strong>{{ t('review.v3.daysValue', { count: intervals.hold }) }}</strong>
            </div>
            <div>
              <span>{{ t('review.v3.lapsesMetric') }}</span>
              <strong>{{ t('review.v3.timesValue', { count: currentCard.lapses }) }}</strong>
            </div>
            <div>
              <span>{{ t('review.v3.intervalMetric') }}</span>
              <strong>{{ intervals.interval ? t('review.v3.daysValue', { count: intervals.interval }) : '—' }}</strong>
            </div>
          </div>
          <div class="qcard__meta">
            <span v-if="primarySource" class="rv-note">
              {{ t('review.v3.sourceExcerpt') }}
              <b class="qcard__source">{{ primarySource.path || t('review.v3.openSource') }}</b>
            </span>
          </div>
        </article>

        <div v-if="!showingAnswer" class="show-answer-wrap">
          <button
            type="button"
            class="rv-btn rv-btn--ghost"
            data-testid="review-v3-show-answer"
            @click="showAnswer"
          >
            {{ t('review.showAnswer') }}
          </button>
          <p class="review-mode__hint">
            <ReviewIcon name="help" :size="13" />
            {{ t('review.v3.modeShortcutHint') }}
          </p>
        </div>

        <template v-else>
          <div class="answer-zone rv-wob" data-testid="review-v3-answer">
            <div class="answer-zone__label">Answer · {{ t('review.v3.answer') }}</div>
            <div class="answer-zone__text review-mode__markdown" v-html="answerHTML"></div>
            <div v-if="currentCard.explanation" class="rv-callout rv-callout--gray answer-zone__explain">
              <ReviewIcon name="lightbulb" :size="18" :tilt="1" />
              <div class="review-mode__markdown" v-html="explanationHTML"></div>
            </div>
            <div v-if="primarySource" class="answer-zone__source" data-testid="review-v3-source">
              <div class="answer-zone__source-head">
                <span>{{ t('review.v3.sourceExcerpt') }}</span>
                <button
                  type="button"
                  class="rv-btn rv-btn--text rv-btn--sm"
                  :disabled="sourceOpening"
                  @click="openSource"
                >
                  {{ t('review.v3.openSource') }}
                  <ReviewIcon name="arrow-right" :size="12" />
                </button>
              </div>
              <p v-if="primarySource?.path" class="answer-zone__source-path" :title="primarySource.path">
                {{ primarySource.path }}
              </p>
              <p>{{ sourceExcerpt || t('review.v3.sourceNoExcerpt') }}</p>
            </div>
          </div>

          <div v-if="ratingError" class="review-mode__error">
            <t-alert theme="error" :message="ratingError" />
            <button type="button" class="rv-btn rv-btn--ghost rv-btn--sm" @click="skipCard">
              {{ t('review.v3.skipCard') }}
            </button>
          </div>

          <div class="rate-row" data-testid="review-v3-ratings">
            <button
              type="button"
              class="rate rate--forgot rv-wob-sm"
              :disabled="Boolean(pendingRating)"
              data-testid="review-v3-rate-forgot"
              @click="rate('forgot')"
            >
              <span class="rate__name"><i class="rate__dot" />{{ t('review.v3.rating.forgot') }}</span>
              <span class="rate__interval">{{ t('review.v3.intervalForgot') }}</span>
              <span class="rate__key">1</span>
            </button>
            <button
              type="button"
              class="rate rate--fuzzy rv-wob-sm"
              :disabled="Boolean(pendingRating)"
              data-testid="review-v3-rate-fuzzy"
              @click="rate('fuzzy')"
            >
              <span class="rate__name"><i class="rate__dot" />{{ t('review.v3.rating.fuzzy') }}</span>
              <span class="rate__interval">{{ t('review.v3.intervalFuzzy') }}</span>
              <span class="rate__key">2</span>
            </button>
            <button
              type="button"
              class="rate rate--remembered rv-wob-sm"
              :disabled="Boolean(pendingRating)"
              data-testid="review-v3-rate-remembered"
              @click="rate('remembered')"
            >
              <span class="rate__name"><i class="rate__dot" />{{ t('review.v3.rating.remembered') }}</span>
              <span class="rate__interval">{{ t('review.v3.intervalRemembered') }}</span>
              <span class="rate__key">3</span>
            </button>
            <button
              type="button"
              class="rate rate--easy rv-wob-sm"
              :disabled="Boolean(pendingRating)"
              data-testid="review-v3-rate-easy"
              @click="rate('easy')"
            >
              <span class="rate__name"><i class="rate__dot" />{{ t('review.v3.rating.easy') }}</span>
              <span class="rate__interval">{{ t('review.v3.intervalEasy') }}</span>
              <span class="rate__key">4</span>
            </button>
          </div>
          <p class="review-mode__hint">
            <ReviewIcon name="help" :size="13" />
            {{ t('review.v3.modeShortcutHint') }}
          </p>
        </template>
      </section>
    </div>
  </div>
</template>

<style lang="less" scoped>
.inline-hand-icon { width: 16px; height: 16px; object-fit: contain; }
.inline-hand-icon--13 { width: 13px; height: 13px; }

.review-mode,
.review-mode *,
.review-mode *::before,
.review-mode *::after {
  box-sizing: border-box;
}

.review-mode {
  width: 100%;
  min-height: 100%;
  background: transparent;
}

.review-mode__inner {
  width: 100%;
  min-height: 100%;
  margin: 0;
  padding: 18px 32px 40px;
  display: flex;
  flex-direction: column;
}

.review-mode__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--td-component-stroke);
}

.review-mode__progress {
  font-size: 14px;
  font-variant-numeric: tabular-nums;
  color: var(--td-text-color-secondary);
}

.review-mode__queue {
  font-size: 14px;
  color: var(--td-text-color-placeholder);
}

.review-mode__card {
  margin-top: auto;
  margin-bottom: auto;
  padding: 22px 24px 20px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-medium);
}

.review-mode__severe {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 14px;
  padding: 8px 12px;
  border-radius: var(--td-radius-default);
  background: color-mix(in srgb, var(--rv-vuln-severe) 10%, var(--td-bg-color-container));
  color: var(--rv-vuln-severe);
  font-size: 14px;
  font-weight: 500;
}

.review-mode__weak {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
  font-size: 14px;
  color: var(--td-text-color-secondary);
}

.review-mode__question {
  font-size: 22px;
  line-height: 34px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  white-space: pre-wrap;
  text-align: center;
}

.review-mode__params {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin-top: 16px;
  padding: 10px 12px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-default);
  background: var(--td-bg-color-container-hover);

  div {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  span {
    font-size: 14px;
    color: var(--td-text-color-placeholder);
  }

  strong {
    font-size: 14px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    color: var(--td-text-color-primary);
  }
}

.review-mode__flip {
  width: 100%;
  margin-top: 22px;
}

.review-mode__answer {
  max-height: min(48vh, 430px);
  overflow-y: auto;
  margin-top: 18px;
  padding: 14px 16px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-default);
  background: var(--td-bg-color-container-hover);
}

.review-mode__answer-label {
  margin: 12px 0 6px;
  font-size: 14px;
  color: var(--td-text-color-placeholder);

  &:first-child {
    margin-top: 0;
  }
}

.review-mode__markdown {
  font-size: 14px;
  line-height: 23px;
  color: var(--td-text-color-primary);
  word-break: break-word;

  :deep(p) {
    margin: 0 0 8px;
  }

  :deep(ul),
  :deep(ol) {
    margin: 0 0 8px;
    padding-left: 22px;
  }

  :deep(pre) {
    margin: 0 0 8px;
    padding: 10px 12px;
    overflow-x: auto;
    border-radius: var(--td-radius-default);
    background: var(--td-bg-color-container);
    border: 1px solid var(--td-component-stroke);
    font-size: 14px;
    line-height: 19px;
  }

  :deep(code) {
    font-family: var(--app-font-family-mono);
  }

  :deep(:not(pre) > code) {
    padding: 1px 4px;
    border-radius: 3px;
    background: var(--td-bg-color-component);
    font-size: 14px;
  }

  :deep(img) {
    max-width: 100%;
    border-radius: var(--td-radius-default);
  }

  :deep(table) {
    width: 100%;
    margin-bottom: 8px;
    border-collapse: collapse;
    font-size: 14px;
  }

  :deep(th),
  :deep(td) {
    padding: 6px 8px;
    border: 1px solid var(--td-component-stroke);
    text-align: left;
  }

  :deep(blockquote) {
    margin: 0 0 8px;
    padding-left: 10px;
    border-left: 3px solid var(--td-component-stroke);
    color: var(--td-text-color-secondary);
  }
}

.review-mode__source {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px solid var(--td-component-stroke);

  p {
    margin: 4px 0 0;
    font-size: 14px;
    line-height: 19px;
    color: var(--td-text-color-secondary);
  }
}

.review-mode__source-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 14px;
  color: var(--td-text-color-placeholder);
}

.review-mode__source-path {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  font-size: 14px !important;
  color: var(--td-text-color-placeholder) !important;
}

.review-mode__error {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 12px;

  :deep(.t-alert) {
    flex: 1;
    min-width: 0;
  }
}

.review-mode__ratings {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-top: 16px;
}

.rating-btn {
  width: 100%;
}

.rating-btn--easy {
  background: var(--rv-vuln-observe);
  border-color: var(--rv-vuln-observe);
  color: var(--td-text-color-anti);
}

.rating-btn--easy:hover {
  background: color-mix(in srgb, var(--rv-vuln-observe) 86%, black);
  border-color: color-mix(in srgb, var(--rv-vuln-observe) 86%, black);
}

.rating-btn__key {
  margin-left: 6px;
  font-size: 14px;
  opacity: 0.7;
}

.review-mode__hint {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  margin: 12px 0 0;
  font-size: 14px;
  color: var(--td-text-color-placeholder);
}

.review-mode__done {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  margin-top: auto;
  margin-bottom: auto;
  padding: 32px 24px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-medium);
  text-align: center;

  h2 {
    margin: 4px 0 0;
    font-size: 20px;
    line-height: 28px;
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  > p {
    margin: 0;
    font-size: 14px;
    color: var(--td-text-color-secondary);
  }

  > :deep(.t-button) {
    margin-top: 8px;
  }
}

.review-mode__done-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  border-radius: 50%;
  background: color-mix(in srgb, var(--td-success-color) 12%, var(--td-bg-color-container));
  color: var(--td-success-color);

  img {
    width: 44px;
    height: 44px;
    object-fit: contain;
  }
}

.review-mode__counts {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 1px;
  width: 100%;
  max-width: 460px;
  margin-top: 10px;
  overflow: hidden;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-default);
  background: var(--td-component-stroke);
}

.review-mode__count {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 10px 6px;
  background: var(--td-bg-color-container);

  strong {
    font-size: 18px;
    line-height: 26px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    color: var(--td-text-color-primary);
  }

  span {
    font-size: 14px;
    color: var(--td-text-color-secondary);
  }
}

.review-mode__done-meta {
  display: flex;
  justify-content: center;
  flex-wrap: wrap;
  gap: 10px 32px;
  margin: 6px 0 10px;

  div {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  span {
    font-size: 14px;
    color: var(--td-text-color-placeholder);
  }

  strong {
    font-size: 14px;
    font-weight: 600;
    color: var(--td-text-color-primary);
  }
}

@media (max-width: 600px) {
  .review-mode__inner {
    padding: 12px 12px 28px;
  }

  .review-mode__question {
    font-size: 19px;
    line-height: 29px;
  }

  .review-mode__card {
    padding: 16px 14px;
  }

  .review-mode__ratings {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .review-mode__params {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>

<style lang="less" scoped>
/* Paper-journal layout for the review overlay. */
.review-mode {
  color: var(--rvn-ink, #3f3a34);
  font-size: 15px;
  line-height: 1.5;
}

.review-mode__inner {
  width: 80%;
  max-width: none;
  margin: 0 auto;
  padding: 28px 0 56px;
}

.review-mode__head {
  display: flex;
  gap: 14px;
  align-items: center;
  margin-bottom: 26px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--rvn-line);
}

.review-mode__progress {
  flex: none;
  font-size: 14px;
  font-variant-numeric: tabular-nums;
  color: var(--rvn-ink-soft);
}

.review-mode__bar {
  flex: 1;
  height: 6px;
  overflow: hidden;
  background: var(--rvn-line);
  border-radius: 999px;

  i {
    display: block;
    height: 100%;
    background: var(--rvn-accent);
    border-radius: 999px;
    transition: width 0.25s ease;
  }
}

.review-mode__card {
  margin: 0;
  padding: 0;
  background: transparent;
  border: 0;
}

.qcard {
  padding: 28px 32px;
  background: var(--rvn-paper);
  border: 1.3px solid var(--rvn-line);
}

.qcard__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.qcard__question {
  margin: 12px 0 6px;
  font-size: 21px;
  font-weight: 600;
  line-height: 1.5;
  letter-spacing: -0.01em;
  color: var(--rvn-ink);
  white-space: pre-wrap;
}

.qcard__params {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin-top: 14px;
  padding: 10px 12px;
  background: var(--rvn-surface);
  border-radius: 7px;

  div {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  span {
    font-size: 14px;
    color: var(--rvn-muted);
  }

  strong {
    font-size: 14px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    color: var(--rvn-ink);
  }
}

.qcard__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin-top: 14px;
}

.qcard__source {
  font-weight: 500;
  color: var(--rvn-accent-deep);
  cursor: pointer;
}

.show-answer-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin: 22px 0 6px;

  .rv-btn {
    min-width: 128px;
    padding: 7px 20px;
  }
}

.answer-zone {
  padding: 24px 28px;
  margin-top: 14px;
  background: var(--rvn-surface);
  border: 1.3px dashed var(--rvn-line-strong);
}

.answer-zone__label {
  margin-bottom: 10px;
  font-size: 14px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--rvn-muted);
}

.answer-zone__text {
  font-size: 15px;
  line-height: 1.7;
  color: var(--rvn-ink);
}

.answer-zone__explain {
  margin: 16px 0 0;
  padding: 12px 14px;
  font-size: 14px;
}

.answer-zone__source {
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid var(--rvn-line);

  p {
    margin: 4px 0 0;
    font-size: 14px;
    line-height: 1.6;
    color: var(--rvn-ink-soft);
  }
}

.answer-zone__source-head {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
  font-size: 14px;
  color: var(--rvn-muted);
}

.answer-zone__source-path {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  font-size: 14px !important;
  color: var(--rvn-muted) !important;
}

.rate-row {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-top: 26px;
}

.rate {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 3px;
  align-items: center;
  padding: 12px 10px;
  font-family: inherit;
  cursor: pointer;
  background: var(--rvn-paper);
  border: 1.5px solid var(--rvn-line);
  transition: border-color 0.18s ease, background 0.18s ease, transform 0.12s ease;

  &:hover:not(:disabled) {
    border-color: var(--rvn-line-strong);
    background: var(--rvn-surface);
  }

  &:active:not(:disabled) {
    transform: translateY(1px);
  }

  &:disabled {
    cursor: progress;
    opacity: 0.65;
  }

  &:focus-visible {
    outline: 2px solid var(--rvn-brand);
    outline-offset: 2px;
  }
}

.rate__name {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  font-size: 14px;
  font-weight: 600;
}

.rate__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: currentColor;
}

.rate__interval {
  font-size: 14px;
  color: var(--rvn-muted);
}

.rate__key {
  position: absolute;
  top: 5px;
  right: 8px;
  font-size: 14px;
  color: var(--rvn-muted);
  opacity: 0.75;
}

.rate--forgot {
  color: var(--rvn-red-deep);
  background: var(--rvn-red-soft);
  border-color: var(--rvn-red);
}

.rate--fuzzy {
  color: var(--rvn-orange-deep);
  background: var(--rvn-orange-soft);
  border-color: var(--rvn-orange);
}

.rate--remembered {
  color: var(--rvn-green-deep);
  background: var(--rvn-green-soft);
  border-color: var(--rvn-green);
}

.rate--easy {
  color: var(--rvn-accent-deep);
  background: var(--rvn-accent-soft);
  border-color: var(--rvn-accent);
}

.review-mode__hint {
  display: flex;
  gap: 5px;
  align-items: center;
  justify-content: center;
  margin: 10px 0 0;
  font-size: 14px;
  color: var(--rvn-muted);
}

.review-mode__error {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 12px;

  :deep(.t-alert) {
    flex: 1;
    min-width: 0;
  }
}

.review-mode__done {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 36px 28px;
  text-align: center;
  background: var(--rvn-paper);
  border: 1.3px solid var(--rvn-line);
  border-radius: 12px 9px 13px 10px / 10px 13px 9px 12px;

  h2 {
    margin: 4px 0 0;
    font-size: 21px;
    font-weight: 700;
    color: var(--rvn-ink);
  }

  > p {
    margin: 0;
    font-size: 14px;
    color: var(--rvn-ink-soft);
  }

  .rv-btn {
    margin-top: 10px;
  }
}

.review-mode__done-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  color: var(--rvn-brand-deep);
  background: var(--rvn-brand-bg);
  border-radius: 50%;
}

.review-mode__counts {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  width: 100%;
  max-width: 460px;
  margin-top: 10px;
}

.review-mode__count {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 10px 6px;
  background: var(--rvn-surface);
  border-radius: 7px;

  strong {
    font-size: 19px;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
  }

  span {
    font-size: 14px;
    color: var(--rvn-ink-soft);
  }

  &.is-forgot strong { color: var(--rvn-red-deep); }
  &.is-fuzzy strong { color: var(--rvn-orange-deep); }
  &.is-remembered strong { color: var(--rvn-green-deep); }
  &.is-easy strong { color: var(--rvn-accent-deep); }
}

.review-mode__done-meta {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 10px 32px;
  margin: 6px 0 10px;

  div {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  span {
    font-size: 14px;
    color: var(--rvn-muted);
  }

  strong {
    font-size: 14px;
    font-weight: 600;
    color: var(--rvn-ink);
  }
}

@media (max-width: 600px) {
  .review-mode__inner {
    padding: 18px 14px 40px;
  }

  .qcard {
    padding: 18px 16px;
  }

  .qcard__question {
    font-size: 19px;
    line-height: 1.55;
  }

  .qcard__params,
  .rate-row {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .answer-zone {
    padding: 18px 16px;
  }
}
</style>
