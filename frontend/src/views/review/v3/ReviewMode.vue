<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { ExternalLink, Keyboard, ShieldAlert } from '@lucide/vue'

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
import { fallbackVulnerability, holdDays, scheduledIntervalDays } from './vulnerability'
import trophyIcon from '@/assets/icons/animated/trophy.gif'
import backIcon from '@/assets/icons/hand/arrow-left.png'

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
      <header class="review-mode__head">
        <t-button variant="text" size="small" data-testid="review-v3-suspend" @click="suspend">
          <template #icon><img class="inline-hand-icon" :src="backIcon" alt="" /></template>
          {{ t('review.v3.suspendExit') }}
        </t-button>
        <span class="review-mode__progress" data-testid="review-v3-mode-progress">
          {{ t('review.v3.modeProgress', { current: progressCurrent, total }) }}
        </span>
        <span class="review-mode__queue">{{ queueLabel }}</span>
      </header>

      <section v-if="finished" class="review-mode__done" data-testid="review-v3-mode-done">
        <div class="review-mode__done-icon">
          <img :src="trophyIcon" alt="" />
        </div>
        <h2>{{ t('review.v3.modeDoneTitle') }}</h2>
        <p>{{ t('review.v3.modeDoneHint', { count: ratedCount }) }}</p>
        <div class="review-mode__counts">
          <div class="review-mode__count">
            <strong>{{ counts.forgot }}</strong>
            <span>{{ t('review.v3.rating.forgot') }}</span>
          </div>
          <div class="review-mode__count">
            <strong>{{ counts.fuzzy }}</strong>
            <span>{{ t('review.v3.rating.fuzzy') }}</span>
          </div>
          <div class="review-mode__count">
            <strong>{{ counts.remembered }}</strong>
            <span>{{ t('review.v3.rating.remembered') }}</span>
          </div>
          <div class="review-mode__count">
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
        <t-button theme="primary" @click="suspend">{{ t('review.v3.backReviewHome') }}</t-button>
      </section>

      <section v-else-if="currentCard" class="review-mode__card">
        <div v-if="isSevere" class="review-mode__severe" data-testid="review-v3-severe-banner">
          <ShieldAlert :size="16" />
          <span>{{ severeText }}</span>
          <ReviewLevelTag level="severe" />
        </div>
        <div v-else-if="isWarning" class="review-mode__weak" data-testid="review-v3-warning-tag">
          <ReviewLevelTag level="warning" />
          <span>{{ t('review.v3.recentWeak') }}</span>
        </div>

        <div class="review-mode__question" data-testid="review-v3-question">{{ currentCard.question }}</div>

        <div v-if="isSevere || isWarning" class="review-mode__params" data-testid="review-v3-params">
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
            <strong>
              {{ intervals.interval ? t('review.v3.daysValue', { count: intervals.interval }) : '—' }}
            </strong>
          </div>
        </div>

        <template v-if="!showingAnswer">
          <t-button
            class="review-mode__flip"
            theme="primary"
            size="large"
            data-testid="review-v3-show-answer"
            @click="showAnswer"
          >
            {{ t('review.v3.showAnswerSpace') }}
          </t-button>
          <p class="review-mode__hint">
            <Keyboard :size="13" />{{ t('review.v3.modeShortcutHint') }}
          </p>
        </template>

        <template v-else>
          <div class="review-mode__answer" data-testid="review-v3-answer">
            <div class="review-mode__answer-label">{{ t('review.v3.answer') }}</div>
            <div class="review-mode__markdown" v-html="answerHTML"></div>
            <template v-if="currentCard.explanation">
              <div class="review-mode__answer-label">{{ t('review.v3.explanation') }}</div>
              <div class="review-mode__markdown" v-html="explanationHTML"></div>
            </template>
            <div v-if="primarySource" class="review-mode__source" data-testid="review-v3-source">
              <div class="review-mode__source-head">
                <span>{{ t('review.v3.sourceExcerpt') }}</span>
                <t-button
                  size="small"
                  variant="text"
                  theme="primary"
                  :loading="sourceOpening"
                  @click="openSource"
                >
                  <template #icon><ExternalLink :size="13" /></template>
                  {{ t('review.v3.openSource') }}
                </t-button>
              </div>
              <p v-if="primarySource?.path" class="review-mode__source-path" :title="primarySource.path">
                {{ primarySource.path }}
              </p>
              <p>{{ sourceExcerpt || t('review.v3.sourceNoExcerpt') }}</p>
            </div>
          </div>

          <div v-if="ratingError" class="review-mode__error">
            <t-alert theme="error" :message="ratingError" />
            <t-button size="small" variant="outline" @click="skipCard">{{ t('review.v3.skipCard') }}</t-button>
          </div>

          <div class="review-mode__ratings" data-testid="review-v3-ratings">
            <t-button
              class="rating-btn rating-btn--forgot"
              theme="danger"
              :loading="pendingRating === 'forgot'"
              :disabled="Boolean(pendingRating)"
              data-testid="review-v3-rate-forgot"
              @click="rate('forgot')"
            >
              {{ t('review.v3.rating.forgot') }}<span class="rating-btn__key">1</span>
            </t-button>
            <t-button
              class="rating-btn rating-btn--fuzzy"
              theme="warning"
              :loading="pendingRating === 'fuzzy'"
              :disabled="Boolean(pendingRating)"
              data-testid="review-v3-rate-fuzzy"
              @click="rate('fuzzy')"
            >
              {{ t('review.v3.rating.fuzzy') }}<span class="rating-btn__key">2</span>
            </t-button>
            <t-button
              class="rating-btn rating-btn--remembered"
              theme="success"
              :loading="pendingRating === 'remembered'"
              :disabled="Boolean(pendingRating)"
              data-testid="review-v3-rate-remembered"
              @click="rate('remembered')"
            >
              {{ t('review.v3.rating.remembered') }}<span class="rating-btn__key">3</span>
            </t-button>
            <t-button
              class="rating-btn rating-btn--easy"
              theme="default"
              :loading="pendingRating === 'easy'"
              :disabled="Boolean(pendingRating)"
              data-testid="review-v3-rate-easy"
              @click="rate('easy')"
            >
              {{ t('review.v3.rating.easy') }}<span class="rating-btn__key">4</span>
            </t-button>
          </div>
          <p class="review-mode__hint">
            <Keyboard :size="13" />{{ t('review.v3.modeShortcutHint') }}
          </p>
        </template>
      </section>
    </div>
  </div>
</template>

<style lang="less" scoped>
.inline-hand-icon { width: 16px; height: 16px; object-fit: contain; }

.review-mode,
.review-mode *,
.review-mode *::before,
.review-mode *::after {
  box-sizing: border-box;
}

.review-mode {
  position: fixed;
  inset: 0;
  z-index: 2400;
  overflow-y: auto;
  background: var(--td-bg-color-page);
}

.review-mode__inner {
  width: min(760px, 100%);
  min-height: 100%;
  margin: 0 auto;
  padding: 18px 20px 40px;
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
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  color: var(--td-text-color-secondary);
}

.review-mode__queue {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.review-mode__card {
  margin-top: auto;
  margin-bottom: auto;
  padding: 22px 24px 20px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-medium);
  box-shadow: 0 6px 24px color-mix(in srgb, var(--td-text-color-primary) 5%, transparent);
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
  font-size: 12px;
  font-weight: 500;
}

.review-mode__weak {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
  font-size: 12px;
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
    font-size: 11px;
    color: var(--td-text-color-placeholder);
  }

  strong {
    font-size: 13px;
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
  font-size: 11px;
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
    font-size: 12px;
    line-height: 19px;
  }

  :deep(code) {
    font-family: var(--app-font-family-mono);
  }

  :deep(:not(pre) > code) {
    padding: 1px 4px;
    border-radius: 3px;
    background: var(--td-bg-color-component);
    font-size: 12px;
  }

  :deep(img) {
    max-width: 100%;
    border-radius: var(--td-radius-default);
  }

  :deep(table) {
    width: 100%;
    margin-bottom: 8px;
    border-collapse: collapse;
    font-size: 13px;
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
    font-size: 12px;
    line-height: 19px;
    color: var(--td-text-color-secondary);
  }
}

.review-mode__source-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 11px;
  color: var(--td-text-color-placeholder);
}

.review-mode__source-path {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  font-size: 11px !important;
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
  font-size: 11px;
  opacity: 0.7;
}

.review-mode__hint {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  margin: 12px 0 0;
  font-size: 11px;
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
  box-shadow: 0 6px 24px color-mix(in srgb, var(--td-text-color-primary) 5%, transparent);
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
    font-size: 13px;
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
    font-size: 11px;
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
    font-size: 11px;
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
