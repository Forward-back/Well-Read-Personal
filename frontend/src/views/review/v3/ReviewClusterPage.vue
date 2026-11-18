<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  getActiveReviewSession,
  getReviewClusterDetail,
  getReviewRootCause,
  startReviewSession,
  type ReviewClusterCard,
  type ReviewClusterDetail,
  type ReviewRootCause,
  type ReviewSessionState,
} from '@/api/review'
import ReviewCardAIDialog from './ReviewCardAIDialog.vue'
import ReviewLevelTag from './ReviewLevelTag.vue'
import type { ReviewSourceTarget } from './types'
import { formatIntensity, holdDays, ratingLabelKey } from './vulnerability'
import brainIcon from '@/assets/icons/animated/brain.gif'
import bookIcon from '@/assets/icons/animated/book.gif'
import rocketIcon from '@/assets/icons/animated/rocket.gif'
import sparklesIcon from '@/assets/icons/animated/sparkles.gif'
import goalIcon from '@/assets/icons/animated/goal.gif'
import backIcon from '@/assets/icons/hand/arrow-left.png'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const detail = ref<ReviewClusterDetail | null>(null)
const loading = ref(true)
const error = ref('')
const selectedCardId = ref('')
const aiRootCause = ref<ReviewRootCause | null>(null)
const aiLoading = ref(false)
const aiError = ref('')
const sourceOpening = ref(false)
const conflictVisible = ref(false)
const conflictBusy = ref(false)
const aiDialogVisible = ref(false)
const aiDialogMode = ref<'split' | 'angles'>('split')
let loadSeq = 0

const rowGroupId = computed(() => {
  const value = route.params.rowGroupId
  return Array.isArray(value) ? value[0] || '' : value || ''
})
const columnKey = computed(() => (typeof route.query.col === 'string' ? route.query.col : ''))
const scopeGroupId = computed(() => (typeof route.query.scope === 'string' ? route.query.scope : ''))
const direct = computed(() => route.query.direct === '1')

const abnormalCards = computed(() =>
  (detail.value?.cards ?? []).filter(item =>
    item.level === 'severe' || item.level === 'warning' || item.level === 'observe',
  ),
)
const relearnCards = computed(() =>
  (detail.value?.cards ?? []).filter(item => item.level === 'severe' || item.level === 'warning'),
)
const selectedCard = computed(() =>
  detail.value?.cards.find(item => item.card.id === selectedCardId.value)
  || abnormalCards.value[0]
  || detail.value?.cards[0]
  || null,
)
const selectedTarget = computed(() => (selectedCard.value ? sourceTarget(selectedCard.value) : null))

const reasons = computed(() =>
  aiRootCause.value?.reasons?.length ? aiRootCause.value.reasons : detail.value?.reasons ?? [],
)
const suggestions = computed(() => aiRootCause.value?.focus_suggestions ?? [])
const rootCauseSummary = computed(() => {
  if (aiRootCause.value?.summary) return aiRootCause.value.summary
  if (detail.value?.reasons?.length) return detail.value.reasons.join('；')
  return t('review.v3.clusterPage.noReasons')
})
const rootCauseSource = computed(() => {
  if (!aiRootCause.value) return t('review.v3.clusterPage.ruleSource')
  return aiRootCause.value.source === 'llm'
    ? t('review.v3.clusterPage.aiSource')
    : t('review.v3.clusterPage.ruleSource')
})
const evidenceCards = computed(() => {
  const ids = new Set(aiRootCause.value?.evidence_card_ids ?? [])
  return (detail.value?.cards ?? []).filter(item => ids.has(item.card.id)).slice(0, 4)
})

const intensityPercent = computed(() => Math.min(100, Math.max(0, Math.round((detail.value?.cell.intensity || 0) * 100))))

async function loadDetail() {
  const seq = ++loadSeq
  loading.value = true
  error.value = ''
  aiRootCause.value = null
  aiError.value = ''
  try {
    const response = await getReviewClusterDetail({
      scope_group_id: scopeGroupId.value || undefined,
      row_group_id: rowGroupId.value,
      column_key: columnKey.value,
      direct: direct.value || undefined,
    })
    if (seq !== loadSeq) return
    detail.value = response.data || null
    const first = abnormalCards.value[0] || detail.value?.cards?.[0]
    selectedCardId.value = first?.card.id || ''
  } catch (err: any) {
    if (seq !== loadSeq) return
    detail.value = null
    error.value = err?.message || t('review.v3.clusterPage.loadFailed')
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

onMounted(loadDetail)
watch(() => route.fullPath, loadDetail)

function goBack() {
  if (window.history.length > 1) router.back()
  else void router.push({ name: 'review', query: scopeGroupId.value ? { scope: scopeGroupId.value } : {} })
}

function ratingLabel(value: string | undefined) {
  const key = ratingLabelKey(value)
  return key ? t(key) : '—'
}

function suggestionLabel(value: string | undefined) {
  if (value === 'relearn') return t('review.v3.suggestionRelearn')
  if (value === 'observe') return t('review.v3.suggestionObserve')
  return '—'
}

function sourceTarget(item: ReviewClusterCard): ReviewSourceTarget | null {
  const source = item.source
  if (!source?.knowledge_id || !item.knowledge_base_id) return null
  return {
    knowledgeBaseId: item.knowledge_base_id,
    knowledgeId: source.knowledge_id,
    chunkId: source.chunk_id || '',
  }
}

function openSource(target: ReviewSourceTarget | null) {
  if (!target || sourceOpening.value) return
  sourceOpening.value = true
  try {
    const query: Record<string, string> = { knowledge_id: target.knowledgeId }
    if (target.chunkId) query.chunk_id = target.chunkId
    const href = router.resolve({
      path: `/platform/knowledge-bases/${target.knowledgeBaseId}`,
      query,
    }).href
    window.open(href, '_blank', 'noopener')
  } finally {
    sourceOpening.value = false
  }
}

function selectCard(item: ReviewClusterCard) {
  selectedCardId.value = item.card.id
}

async function analyzeRootCause() {
  if (aiLoading.value) return
  aiLoading.value = true
  aiError.value = ''
  try {
    const response = await getReviewRootCause({
      scope_group_id: scopeGroupId.value || undefined,
      row_group_id: rowGroupId.value,
      column_key: columnKey.value,
      direct: direct.value || undefined,
    })
    aiRootCause.value = response.data || null
    if (response.data?.source === 'rule') {
      MessagePlugin.info(t('review.v3.clusterPage.aiFailed'))
    }
  } catch (err: any) {
    aiError.value = err?.message || t('review.v3.clusterPage.aiFailed')
  } finally {
    aiLoading.value = false
  }
}

function openReviewWithResume() {
  const query: Record<string, string> = { resume: '1' }
  if (scopeGroupId.value) query.scope = scopeGroupId.value
  void router.push({ name: 'review', query })
}

async function startRelearn() {
  if (!relearnCards.value.length || conflictBusy.value) return
  conflictBusy.value = true
  try {
    const active = await getActiveReviewSession()
    if (active.data) {
      conflictVisible.value = true
      return
    }
    await beginRelearn(false)
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.loadFailed'))
  } finally {
    conflictBusy.value = false
  }
}

async function beginRelearn(forceNew: boolean) {
  const response = await startReviewSession({
    mode: 'manual',
    group_id: detail.value?.row.group_id || undefined,
    card_ids: relearnCards.value.map(item => item.card.id),
    force_new: forceNew || undefined,
  })
  if (!response.data) throw new Error(t('review.loadFailed'))
  conflictVisible.value = false
  openReviewWithResume()
}

async function abandonAndRelearn() {
  if (conflictBusy.value) return
  conflictBusy.value = true
  try {
    await beginRelearn(true)
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.loadFailed'))
  } finally {
    conflictBusy.value = false
  }
}

function continueCurrent() {
  conflictVisible.value = false
  openReviewWithResume()
}

function openAIDialog(mode: 'split' | 'angles') {
  if (!selectedCard.value) return
  aiDialogMode.value = mode
  aiDialogVisible.value = true
}

function onAIDialogApplied() {
  void loadDetail()
}
</script>

<template>
  <div class="cluster-page rv-vuln-scope" data-testid="review-v3-cluster-page">
    <div class="cluster-page__inner">
      <div class="cluster-page__topbar">
        <t-button variant="text" size="small" data-testid="review-v3-cluster-back" @click="goBack">
          <template #icon><img class="inline-hand-icon" :src="backIcon" alt="" /></template>
          {{ t('review.v3.clusterPage.backToday') }}
        </t-button>
      </div>

      <t-loading :loading="loading" size="small">
        <div v-if="error" class="cluster-page__state">
          <t-alert theme="error" :message="error" />
          <t-button variant="outline" size="small" @click="loadDetail">{{ t('common.retry') }}</t-button>
        </div>

        <template v-else-if="detail">
          <section class="cluster-hero">
            <div class="cluster-hero__main">
              <div class="cluster-hero__eyebrow">
                <ReviewLevelTag :level="detail.cell.level" />
                <span>{{ detail.row.path }}</span>
              </div>
              <div class="cluster-hero__title-row">
                <img class="rv-icon rv-icon--lg" :src="goalIcon" alt="" />
                <h1>{{ detail.column.title }}</h1>
              </div>
              <p class="cluster-hero__subtitle">{{ detail.row.path }} · {{ t('review.v3.clusterPage.vulnerabilityCluster') }}</p>
              <div class="cluster-formula">
                <span>{{ t('review.v3.clusterPage.formula') }}</span>
                <strong>{{ formatIntensity(detail.cell.intensity) }}</strong>
                <span class="cluster-formula__detail">
                  ({{ detail.cell.severe }}×1 + {{ detail.cell.warning }}×0.5 + {{ detail.cell.observe }}×0.25) ÷ {{ detail.cell.total }}
                </span>
              </div>
              <div class="cluster-bar"><i :style="{ width: `${intensityPercent}%` }" /></div>
            </div>
            <t-button
              theme="primary"
              size="large"
              :disabled="!relearnCards.length || conflictBusy"
              data-testid="review-v3-cluster-relearn"
              @click="startRelearn"
            >
              <img class="rv-icon rv-icon--sm cluster-cta-icon" :src="rocketIcon" alt="" />
              {{ t('review.v3.relearnCount', { count: relearnCards.length }) }}
            </t-button>
          </section>

          <div v-if="detail.reasons.length" class="cluster-reasons" data-testid="review-v3-cluster-reasons">
            <span class="cluster-reasons__label">{{ t('review.v3.clusterPage.reasonsTitle') }}</span>
            <span v-for="reason in detail.reasons" :key="reason" class="cluster-reason">{{ reason }}</span>
          </div>

          <section class="cluster-panel">
            <div v-if="!abnormalCards.length" class="cluster-empty">
              <p>{{ t('review.v3.clusterPage.noCards') }}</p>
              <p>{{ t('review.v3.clusterPage.noCardsHint') }}</p>
              <t-button variant="outline" size="small" @click="goBack">
                {{ t('review.v3.clusterPage.backToday') }}
              </t-button>
            </div>

            <div v-else class="cluster-table-wrap">
              <table class="cluster-table">
                <thead>
                  <tr>
                    <th class="cluster-table__select">{{ t('review.v3.clusterTableSelect') }}</th>
                    <th class="cluster-table__question">{{ t('review.v3.clusterTableQuestion') }}</th>
                    <th>{{ t('review.v3.clusterTableHold') }}</th>
                    <th>{{ t('review.v3.clusterTableLapses') }}</th>
                    <th>{{ t('review.v3.clusterTableStreak') }}</th>
                    <th>{{ t('review.v3.clusterTableLastRating') }}</th>
                    <th>{{ t('review.v3.clusterTableStatus') }}</th>
                    <th>{{ t('review.v3.clusterTableSuggestion') }}</th>
                    <th>{{ t('review.v3.clusterTableAction') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="item in detail.cards"
                    :key="item.card.id"
                    :class="{ 'is-selected': item.card.id === selectedCard?.card.id }"
                    @click="selectCard(item)"
                  >
                    <td class="cluster-table__select">
                      <t-radio
                        :checked="item.card.id === selectedCard?.card.id"
                        @change="selectCard(item)"
                      />
                    </td>
                    <td class="cluster-table__question" :title="item.card.question">
                      <span class="cluster-table__question-text">{{ item.card.question }}</span>
                    </td>
                    <td>{{ t('review.v3.clusterHoldDays', { count: holdDays(item.card.stability) }) }}</td>
                    <td>{{ t('review.v3.clusterLapses', { count: item.card.lapses }) }}</td>
                    <td>{{ t('review.v3.clusterLapses', { count: item.card.forget_streak }) }}</td>
                    <td>{{ ratingLabel(item.last_rating) }}</td>
                    <td>
                      <t-tooltip v-if="item.card.vulnerability_reason" :content="item.card.vulnerability_reason" placement="top">
                        <span><ReviewLevelTag :level="item.level" /></span>
                      </t-tooltip>
                      <ReviewLevelTag v-else :level="item.level" />
                    </td>
                    <td>{{ suggestionLabel(item.suggested_action) }}</td>
                    <td>
                      <t-button
                        v-if="sourceTarget(item)"
                        size="small"
                        variant="text"
                        theme="primary"
                        @click.stop="openSource(sourceTarget(item))"
                      >
                        {{ t('review.v3.openSource') }}
                      </t-button>
                      <t-tooltip v-else :content="t('review.v3.sourceUnavailable')" placement="top">
                        <span class="cluster-table__disabled">
                          <t-button size="small" variant="text" disabled>{{ t('review.v3.openSource') }}</t-button>
                        </span>
                      </t-tooltip>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section class="cluster-root-cause" data-testid="review-v3-cluster-root-cause">
            <div class="cluster-root-cause__head">
              <div class="cluster-root-cause__title">
                <img class="rv-icon" :src="brainIcon" alt="" />
                <h2>{{ t('review.v3.clusterPage.rootCause') }}</h2>
                <span class="cluster-root-cause__source">{{ rootCauseSource }}</span>
              </div>
              <t-button
                size="small"
                variant="outline"
                :loading="aiLoading"
                data-testid="review-v3-cluster-ai"
                @click="analyzeRootCause"
              >
                <img v-if="!aiLoading" class="rv-icon rv-icon--sm" :src="sparklesIcon" alt="" />
                {{ aiLoading ? t('review.v3.clusterPage.analyzing') : t('review.v3.clusterPage.analyze') }}
              </t-button>
            </div>
            <p class="cluster-root-cause__summary">{{ rootCauseSummary }}</p>
            <div v-if="reasons.length" class="cluster-root-cause__chips">
              <span v-for="reason in reasons" :key="reason">{{ reason }}</span>
            </div>
            <div v-if="suggestions.length" class="cluster-root-cause__suggestions">
              <span>{{ t('review.v3.clusterPage.suggestions') }}</span>
              <ul>
                <li v-for="item in suggestions" :key="item">{{ item }}</li>
              </ul>
            </div>
            <div v-if="evidenceCards.length" class="cluster-root-cause__evidence">
              <span>{{ t('review.v3.clusterPage.evidence') }}</span>
              <ul>
                <li v-for="item in evidenceCards" :key="item.card.id">{{ item.card.question }}</li>
              </ul>
            </div>
            <t-alert v-if="aiError" theme="error" :message="aiError" class="cluster-root-cause__error" />
          </section>

          <section class="cluster-actions" data-testid="review-v3-cluster-actions">
            <button
              type="button"
              class="cluster-action cluster-action--primary"
              :disabled="!relearnCards.length || conflictBusy"
              data-testid="review-v3-action-relearn"
              @click="startRelearn"
            >
              <img class="rv-icon rv-icon--lg cluster-action__icon" :src="rocketIcon" alt="" />
              <strong>{{ t('review.v3.actions.relearn', { count: relearnCards.length }) }}</strong>
              <span>{{ t('review.v3.actions.relearnHint') }}</span>
            </button>
            <button
              type="button"
              class="cluster-action"
              :disabled="!selectedTarget"
              :title="selectedTarget ? undefined : t('review.v3.clusterPage.noSource')"
              data-testid="review-v3-action-source"
              @click="openSource(selectedTarget)"
            >
              <img class="rv-icon rv-icon--lg cluster-action__icon" :src="bookIcon" alt="" />
              <strong>{{ t('review.v3.actions.openSource') }}</strong>
              <span>{{ t('review.v3.actions.openSourceHint') }}</span>
            </button>
            <button
              type="button"
              class="cluster-action"
              :disabled="!selectedTarget"
              data-testid="review-v3-action-split"
              @click="openAIDialog('split')"
            >
              <img class="rv-icon rv-icon--lg cluster-action__icon" :src="sparklesIcon" alt="" />
              <strong>{{ t('review.v3.actions.split') }}</strong>
              <span>{{ t('review.v3.actions.splitHint') }}</span>
            </button>
            <button
              type="button"
              class="cluster-action"
              :disabled="!selectedTarget"
              data-testid="review-v3-action-angles"
              @click="openAIDialog('angles')"
            >
              <img class="rv-icon rv-icon--lg cluster-action__icon" :src="sparklesIcon" alt="" />
              <strong>{{ t('review.v3.actions.angles') }}</strong>
              <span>{{ t('review.v3.actions.anglesHint') }}</span>
            </button>
          </section>
        </template>

        <div v-else class="cluster-page__state">
          <t-empty :description="t('review.v3.clusterPage.notFound')" />
          <t-button variant="outline" size="small" @click="goBack">
            {{ t('review.v3.clusterPage.backToday') }}
          </t-button>
        </div>
      </t-loading>

      <footer class="rv-attribution">
        <a href="https://icons8.com" target="_blank" rel="noopener noreferrer">Animated icons by Icons8</a>
      </footer>

      <ReviewCardAIDialog
        v-model:visible="aiDialogVisible"
        :mode="aiDialogMode"
        :card="selectedCard?.card || null"
        @applied="onAIDialogApplied"
      />

      <t-dialog
        v-model:visible="conflictVisible"
        :header="t('review.v3.sessionConflictTitle')"
        :footer="false"
        width="460px"
      >
        <p class="cluster-dialog-text">{{ t('review.v3.sessionConflictHint') }}</p>
        <div class="cluster-dialog-actions">
          <t-button variant="outline" :disabled="conflictBusy" @click="continueCurrent">
            {{ t('review.v3.continueCurrent') }}
          </t-button>
          <t-button theme="danger" :loading="conflictBusy" @click="abandonAndRelearn">
            {{ t('review.v3.abandonAndRelearn') }}
          </t-button>
        </div>
      </t-dialog>
    </div>
  </div>
</template>

<style lang="less" scoped>
.inline-hand-icon { width: 16px; height: 16px; object-fit: contain; }

.cluster-page,
.cluster-page *,
.cluster-page *::before,
.cluster-page *::after {
  box-sizing: border-box;
}

.cluster-page {
  min-height: 100%;
  overflow-y: auto;
  background: var(--td-bg-color-page);
}

.cluster-page__inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 18px 28px 44px;
}

.cluster-page__topbar {
  margin-bottom: 10px;
}

.cluster-page__state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 36px 16px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-medium);
}

.cluster-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  padding: 20px 24px 18px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-medium);
}

.cluster-hero__main {
  min-width: 0;
}

.cluster-hero__eyebrow {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.cluster-hero__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.cluster-hero h1 {
  margin: 6px 0 2px;
  font-size: 20px;
  line-height: 28px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.cluster-hero__subtitle {
  margin: 0 0 10px;
  font-size: 12px;
  line-height: 18px;
  color: var(--td-text-color-secondary);
}

.cluster-cta-icon {
  margin-right: 4px;
}

.cluster-formula {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 12px;
  color: var(--td-text-color-secondary);

  strong {
    font-size: 18px;
    line-height: 26px;
    color: var(--td-text-color-primary);
    font-variant-numeric: tabular-nums;
  }
}

.cluster-formula__detail {
  color: var(--td-text-color-placeholder);
  font-variant-numeric: tabular-nums;
}

.cluster-bar {
  width: min(520px, 100%);
  height: 8px;
  margin-top: 10px;
  overflow: hidden;
  border-radius: var(--td-radius-round);
  background: var(--td-bg-color-component);

  i {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: linear-gradient(90deg, var(--td-warning-color), var(--rv-vuln-severe));
  }
}

.cluster-reasons {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  padding: 9px 14px;
  border: 1px solid color-mix(in srgb, var(--td-warning-color) 28%, transparent);
  border-radius: var(--td-radius-default);
  background: color-mix(in srgb, var(--td-warning-color) 7%, var(--td-bg-color-container));
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.cluster-reasons__label {
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.cluster-reason {
  padding: 1px 8px;
  border-radius: var(--td-radius-round);
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
}

.cluster-panel {
  margin-top: 14px;
  overflow: hidden;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-medium);
}

.cluster-table-wrap {
  overflow-x: auto;
}

.cluster-table {
  width: 100%;
  min-width: 980px;
  border-collapse: collapse;
  font-size: 13px;
  color: var(--td-text-color-primary);
}

.cluster-table th,
.cluster-table td {
  height: 56px;
  padding: 0 12px;
  text-align: left;
  white-space: nowrap;
  border-bottom: 1px solid var(--td-component-stroke);
}

.cluster-table th {
  height: 40px;
  font-size: 11px;
  font-weight: 500;
  color: var(--td-text-color-placeholder);
  background: var(--td-bg-color-container);
}

.cluster-table tbody tr {
  cursor: pointer;

  &:hover td {
    background: var(--td-bg-color-container-hover);
  }

  &.is-selected td {
    background: var(--td-brand-color-light);
  }
}

.cluster-table__select {
  width: 56px;
  text-align: center !important;
}

.cluster-table__question {
  min-width: 280px;
  max-width: 420px;
}

.cluster-table__question-text {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  white-space: normal;
  line-height: 19px;
}

.cluster-table__disabled {
  display: inline-flex;
}

.cluster-root-cause {
  margin-top: 14px;
  padding: 16px 20px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-medium);
}

.cluster-root-cause__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.cluster-root-cause__title {
  display: flex;
  align-items: center;
  gap: 7px;
  color: var(--td-text-color-primary);

  h2 {
    margin: 0;
    font-size: 15px;
    line-height: 22px;
    font-weight: 600;
  }
}

.cluster-root-cause__source {
  padding: 1px 7px;
  border-radius: var(--td-radius-round);
  background: var(--td-bg-color-container-hover);
  font-size: 10px;
  font-weight: 400;
  color: var(--td-text-color-placeholder);
}

.cluster-root-cause__summary {
  margin: 10px 0 0;
  font-size: 13px;
  line-height: 22px;
  color: var(--td-text-color-secondary);
}

.cluster-root-cause__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;

  span {
    padding: 1px 8px;
    border-radius: var(--td-radius-round);
    background: color-mix(in srgb, var(--td-error-color) 8%, var(--td-bg-color-container));
    border: 1px solid color-mix(in srgb, var(--td-error-color) 20%, transparent);
    font-size: 11px;
    color: var(--rv-vuln-severe);
  }
}

.cluster-root-cause__suggestions,
.cluster-root-cause__evidence {
  margin-top: 10px;
  font-size: 12px;
  color: var(--td-text-color-secondary);

  > span {
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  ul {
    margin: 4px 0 0;
    padding-left: 20px;
  }

  li {
    line-height: 20px;
  }
}

.cluster-root-cause__error {
  margin-top: 10px;
}

.cluster-actions {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-top: 14px;
}

.cluster-action {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  min-height: 112px;
  padding: 14px 16px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-medium);
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
  font-family: inherit;
  text-align: left;
  cursor: pointer;

  svg {
    color: var(--td-text-color-secondary);
  }

  &__icon {
    margin-bottom: 2px;
  }

  strong {
    margin-top: 4px;
    font-size: 14px;
    line-height: 21px;
    font-weight: 600;
  }

  span {
    font-size: 11px;
    line-height: 18px;
    color: var(--td-text-color-placeholder);
  }

  &:hover:not(:disabled) {
    border-color: var(--td-component-border);
    background: var(--td-bg-color-container-hover);
  }

  &:focus-visible {
    outline: 2px solid var(--td-brand-color);
    outline-offset: 1px;
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
}

.cluster-action--primary {
  background: color-mix(in srgb, var(--td-brand-color) 6%, var(--td-bg-color-container));
  border-color: color-mix(in srgb, var(--td-brand-color) 35%, transparent);

  svg,
  strong {
    color: var(--td-brand-color);
  }
}

.cluster-dialog-text {
  margin: 0 0 12px;
  font-size: 13px;
  line-height: 21px;
  color: var(--td-text-color-secondary);
}

.cluster-dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.cluster-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 30px 16px;
  text-align: center;
  color: var(--td-text-color-secondary);

  p {
    margin: 0;
    font-size: 13px;
  }

  p + p {
    font-size: 12px;
    color: var(--td-text-color-placeholder);
  }
}

@media (max-width: 900px) {
  .cluster-page__inner {
    padding: 14px 16px 32px;
  }

  .cluster-hero {
    flex-direction: column;
  }

  .cluster-actions {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 600px) {
  .cluster-page__inner {
    padding: 12px 12px 28px;
  }

  .cluster-hero h1 {
    font-size: 20px;
    line-height: 29px;
  }

  .cluster-actions {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
