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
import ReviewIcon from './ReviewIcon.vue'
import ReviewLevelTag from './ReviewLevelTag.vue'
import type { ReviewSourceTarget } from './types'
import { formatIntensity, holdDays, ratingLabelKey } from './vulnerability'
import './review-ui.css'

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
        <div class="rv-crumb">
          <span>复习 / 漏洞热力图 /</span>
          <b>{{ detail?.column.title || t('review.v3.clusterPage.vulnerabilityCluster') }}</b>
        </div>
        <button type="button" class="rv-btn rv-btn--ghost rv-btn--sm" data-testid="review-v3-cluster-back" @click="goBack">
          <ReviewIcon name="arrow-left" :size="13" />
          {{ t('review.v3.clusterPage.backToday') }}
        </button>
      </div>

      <t-loading :loading="loading" size="small">
        <div v-if="error" class="cluster-page__state">
          <t-alert theme="error" :message="error" />
          <t-button variant="outline" size="small" @click="loadDetail">{{ t('common.retry') }}</t-button>
        </div>

        <template v-else-if="detail">
          <section class="cluster-hero">
            <div class="cluster-hero__main">
              <h1>
                {{ detail.column.title }}
                <ReviewLevelTag :level="detail.cell.level" />
              </h1>
              <div class="rv-props">
                <div class="rv-prop">
                  <span class="rv-prop__key">{{ t('review.v3.clusterPage.cardsCount') }}</span>
                  <b>{{ detail.cell.total }}</b>
                </div>
                <div class="rv-prop">
                  <span class="rv-prop__key">{{ t('review.v3.clusterPage.formula') }}</span>
                  <strong>{{ formatIntensity(detail.cell.intensity) }}</strong>
                  <span class="rv-note">
                    ({{ detail.cell.severe }}×1 + {{ detail.cell.warning }}×0.5 + {{ detail.cell.observe }}×0.25) ÷ {{ detail.cell.total }}
                  </span>
                </div>
                <div class="rv-prop">
                  <span class="rv-prop__key">{{ t('review.v3.clusterPage.scope') }}</span>
                  <b>{{ detail.row.path }}</b>
                </div>
              </div>
              <div class="cluster-bar"><i :style="{ width: `${intensityPercent}%` }" /></div>
            </div>
            <button
              type="button"
              class="rv-btn rv-btn--primary cluster-hero__cta"
              :disabled="!relearnCards.length || conflictBusy"
              data-testid="review-v3-cluster-relearn"
              @click="startRelearn"
            >
              {{ t('review.v3.relearnCount', { count: relearnCards.length }) }}
              <ReviewIcon name="arrow-right" :size="13" />
            </button>
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
                <ReviewIcon name="lightbulb" :size="18" :tilt="1" />
                <h2>{{ t('review.v3.clusterPage.rootCause') }}</h2>
                <span class="cluster-root-cause__source">{{ rootCauseSource }}</span>
              </div>
              <button
                type="button"
                class="rv-btn rv-btn--ghost rv-btn--sm"
                :disabled="aiLoading"
                data-testid="review-v3-cluster-ai"
                @click="analyzeRootCause"
              >
                <ReviewIcon name="refresh" :size="13" />
                {{ aiLoading ? t('review.v3.clusterPage.analyzing') : t('review.v3.clusterPage.analyze') }}
              </button>
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
              class="cluster-action cluster-action--relearn"
              :disabled="!relearnCards.length || conflictBusy"
              data-testid="review-v3-action-relearn"
              @click="startRelearn"
            >
              <ReviewIcon name="arrow-right" :size="18" />
              <strong>{{ t('review.v3.actions.relearn', { count: relearnCards.length }) }}</strong>
              <span>{{ t('review.v3.actions.relearnHint') }}</span>
            </button>
            <button
              type="button"
              class="cluster-action cluster-action--source"
              :disabled="!selectedTarget"
              :title="selectedTarget ? undefined : t('review.v3.clusterPage.noSource')"
              data-testid="review-v3-action-source"
              @click="openSource(selectedTarget)"
            >
              <ReviewIcon name="book" :size="18" />
              <strong>{{ t('review.v3.actions.openSource') }}</strong>
              <span>{{ t('review.v3.actions.openSourceHint') }}</span>
            </button>
            <button
              type="button"
              class="cluster-action cluster-action--split"
              :disabled="!selectedTarget"
              data-testid="review-v3-action-split"
              @click="openAIDialog('split')"
            >
              <ReviewIcon name="scissors" :size="18" />
              <strong>{{ t('review.v3.actions.split') }}</strong>
              <span>{{ t('review.v3.actions.splitHint') }}</span>
            </button>
            <button
              type="button"
              class="cluster-action cluster-action--angles"
              :disabled="!selectedTarget"
              data-testid="review-v3-action-angles"
              @click="openAIDialog('angles')"
            >
              <ReviewIcon name="edit" :size="18" />
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
        <a href="https://icons8.com" target="_blank" rel="noopener noreferrer">Icons by Icons8</a>
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
  font-size: 14px;
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
  font-size: 14px;
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
  font-size: 14px;
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
  font-size: 14px;
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
  font-size: 14px;
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
  font-size: 14px;
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
  font-size: 14px;
  font-weight: 400;
  color: var(--td-text-color-placeholder);
}

.cluster-root-cause__summary {
  margin: 10px 0 0;
  font-size: 14px;
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
    font-size: 14px;
    color: var(--rv-vuln-severe);
  }
}

.cluster-root-cause__suggestions,
.cluster-root-cause__evidence {
  margin-top: 10px;
  font-size: 14px;
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
    font-size: 14px;
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
  font-size: 14px;
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
    font-size: 14px;
  }

  p + p {
    font-size: 14px;
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

<style lang="less" scoped>
/* Paper-journal presentation of the cluster detail page. */
.cluster-page {
  min-height: 100%;
  background: transparent;
  color: var(--rvn-ink, #3f3a34);
  font-size: 15px;
  --rvn-paper: #fbf9f5;
  --rvn-surface: #f0ece5;
  --rvn-surface-deep: #e9e4db;
  --rvn-ink: #3f3a34;
  --rvn-ink-soft: #6e675e;
  --rvn-muted: #857c70;
  --rvn-line: #e0dbd1;
  --rvn-line-strong: rgba(63, 58, 52, 0.15);
  --rvn-gray-soft: #ede7dd;
  --rvn-accent: #6e93a8;
  --rvn-accent-deep: #47677a;
  --rvn-accent-soft: #dfe7ea;
  --rvn-red: #b0716b;
  --rvn-red-deep: #8a4e47;
  --rvn-red-soft: #ebdcd7;
  --rvn-orange: #b18e6e;
  --rvn-orange-deep: #7e5e42;
  --rvn-orange-soft: #ece3d8;
  --rvn-yellow: #b3a26e;
  --rvn-yellow-deep: #7c6e42;
  --rvn-yellow-soft: #ede8da;
  --rvn-green: #8ca08c;
  --rvn-green-deep: #5e7261;
  --rvn-green-soft: #e3e9df;
  --rvn-purple-deep: #6b5f7a;
  --rvn-purple-soft: #e6e1ea;
  --rvn-brand: #5a7688;
  --rvn-brand-deep: #4c6678;
  --rvn-brand-bg: #e7ecef;
  --rvn-brand-line: #c9d4db;
}

.cluster-page__inner {
  max-width: none;
  margin: 0;
  padding: 24px 10% 48px;
  background: transparent;
  border: 0;
  border-radius: 0;
}

.cluster-page__topbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.cluster-page__topbar .rv-crumb {
  margin-bottom: 0;
}

.cluster-hero {
  display: flex;
  flex-wrap: wrap;
  gap: 20px;
  align-items: flex-end;
  justify-content: space-between;
  padding: 0;
  margin-bottom: 8px;
  background: transparent;
  border: 0;
}

.cluster-hero__main {
  min-width: 0;
}

.cluster-hero h1 {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  margin: 0 0 14px;
  font-size: 34px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: -0.02em;
  color: var(--rvn-ink);
}

.cluster-hero__cta {
  flex: none;
  padding: 8px 18px;
}

.cluster-bar {
  height: 6px;
  margin-top: 4px;
  overflow: hidden;
  background: var(--rvn-line);
  border-radius: 999px;

  i {
    display: block;
    height: 100%;
    background: var(--rvn-red);
    border-radius: 999px;
  }
}

.cluster-reasons {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  padding: 12px 14px;
  margin: 16px 0;
  background: var(--rvn-red-soft);
  border: 1px solid var(--rvn-red);
  border-radius: 7px 6px 8px 7px / 6px 8px 7px 6px;
}

.cluster-reasons__label {
  font-size: 14px;
  font-weight: 600;
  color: var(--rvn-red-deep);
}

.cluster-reason {
  padding: 2px 8px;
  font-size: 14px;
  color: var(--rvn-red-deep);
  background: var(--rvn-paper);
  border-radius: 999px;
}

.cluster-panel {
  margin-top: 28px;
  padding: 0;
  background: transparent;
  border: 0;
}

.cluster-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
}

.cluster-table th {
  padding: 8px 10px;
  font-size: 14px;
  font-weight: 500;
  color: var(--rvn-muted);
  text-align: left;
  border-bottom: 1px solid var(--rvn-line);
}

.cluster-table td {
  padding: 9px 10px;
  vertical-align: middle;
  border-bottom: 1px solid var(--rvn-line);
}

.cluster-table tbody tr {
  cursor: pointer;
  transition: background 0.18s ease;
}

.cluster-table tbody tr:hover td {
  background: var(--rvn-surface);
}

.cluster-table tbody tr.is-selected td {
  background: var(--rvn-brand-bg);
}

.cluster-table__question {
  font-weight: 500;
  color: var(--rvn-ink);

  &:hover {
    color: var(--rvn-brand-deep);
  }
}

.cluster-root-cause {
  padding: 14px 16px;
  margin-top: 24px;
  background: var(--rvn-yellow-soft);
  border: 1px solid var(--rvn-yellow-soft);
  border-radius: 7px 6px 8px 7px / 6px 8px 7px 6px;
}

.cluster-root-cause__title {
  display: flex;
  gap: 8px;
  align-items: center;

  h2 {
    margin: 0;
    font-size: 14.5px;
    font-weight: 600;
    color: var(--rvn-ink);
  }
}

.cluster-root-cause__source {
  padding: 1px 7px;
  font-size: 14px;
  color: var(--rvn-yellow-deep);
  background: var(--rvn-paper);
  border-radius: 999px;
}

.cluster-root-cause__summary {
  margin: 10px 0 0;
  font-size: 14px;
  line-height: 1.65;
  color: var(--rvn-ink);
}

.cluster-root-cause__chips span {
  color: var(--rvn-yellow-deep);
  background: var(--rvn-paper);
}

.cluster-actions {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  margin-top: 26px;
}

.cluster-action {
  display: flex;
  flex-direction: column;
  gap: 5px;
  align-items: flex-start;
  width: 100%;
  min-height: 132px;
  padding: 18px 20px;
  font-family: inherit;
  text-align: left;
  cursor: pointer;
  background: var(--rvn-paper);
  border: 1px solid var(--rvn-line);
  border-radius: 7px 6px 8px 7px / 6px 8px 7px 6px;
  transition: background 0.18s ease, border-color 0.18s ease, transform 0.12s ease;

  strong {
    margin-top: 4px;
    font-size: 14.5px;
    font-weight: 600;
    color: var(--rvn-ink);
  }

  span {
    font-size: 14px;
    line-height: 1.55;
    color: var(--rvn-ink-soft);
  }

  &:hover:not(:disabled) {
    filter: brightness(0.97);
    transform: translateY(-1px);
  }

  &:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }
}

.cluster-action--relearn {
  color: var(--rvn-red-deep);
  background: var(--rvn-red-soft);
  border-color: #dcc3bc;

  strong {
    color: var(--rvn-red-deep);
  }

  span {
    color: var(--rvn-red-deep);
    opacity: 0.82;
  }
}

.cluster-action--source {
  color: var(--rvn-accent-deep);
  background: var(--rvn-accent-soft);
  border-color: #c9d6de;

  strong {
    color: var(--rvn-accent-deep);
  }

  span {
    color: var(--rvn-accent-deep);
    opacity: 0.82;
  }
}

.cluster-action--split {
  color: var(--rvn-yellow-deep);
  background: var(--rvn-yellow-soft);
  border-color: #ddd2b8;

  strong {
    color: var(--rvn-yellow-deep);
  }

  span {
    color: var(--rvn-yellow-deep);
    opacity: 0.82;
  }
}

.cluster-action--angles {
  color: var(--rvn-purple-deep);
  background: var(--rvn-purple-soft);
  border-color: #d3ccd9;

  strong {
    color: var(--rvn-purple-deep);
  }

  span {
    color: var(--rvn-purple-deep);
    opacity: 0.82;
  }
}

@media (max-width: 1100px) {
  .cluster-page__inner {
    margin: 0;
    padding: 28px 24px 40px;
    border: 0;
    border-radius: 0;
  }

  .cluster-actions {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
