<script setup lang="ts">
import { computed, defineAsyncComponent, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'

import {
  createReviewGroup,
  deleteReviewGroup,
  exportReviewData,
  getActiveReviewSession,
  getReviewHeatmap,
  getReviewQueue,
  listReviewGroups,
  startReviewSession,
  updateReviewGroup,
  type ReviewCard,
  type ReviewGroup,
  type ReviewGroupNode,
  type ReviewHeatmap,
  type ReviewHeatmapCell,
  type ReviewQueue,
  type ReviewSessionState,
} from '@/api/review'
import ReviewGenerateDialog from './ReviewGenerateDialog.vue'
import ReviewGroupRail from './ReviewGroupRail.vue'
import ReviewMode from './ReviewMode.vue'
import ReviewTodayView from './ReviewTodayView.vue'
import type { ReviewFlatGroup } from './types'
import './vulnerability.css'
import './review-ui.css'
import calendarIcon from '@/assets/icons/static/calendar.png'

const ReviewCardsView = defineAsyncComponent(() => import('./ReviewCardsView.vue'))
const ReviewStatsView = defineAsyncComponent(() => import('./ReviewStatsView.vue'))

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const activeView = ref<'today' | 'cards' | 'stats'>('today')
const pageRoot = ref<HTMLElement | null>(null)
const groups = ref<ReviewGroupNode[]>([])
const flatGroups = ref<ReviewFlatGroup[]>([])
const selectedGroupId = ref(typeof route.query.scope === 'string' ? route.query.scope : '')
const queue = ref<ReviewQueue | null>(null)
const heatmap = ref<ReviewHeatmap | null>(null)
const activeSession = ref<ReviewSessionState | null>(null)
const reviewVisible = ref(false)
const reviewState = ref<ReviewSessionState | null>(null)
const reviewLabel = ref('')
const reviewStarting = ref(false)
const heatmapExpanded = ref(false)
const loading = ref(true)
const queueLoading = ref(false)
const heatmapLoading = ref(false)
const error = ref('')
const heatmapError = ref('')
let queueRequestSeq = 0
let heatmapRequestSeq = 0

const groupDialogVisible = ref(false)
const groupDialogMode = ref<'create' | 'rename'>('create')
const groupSaving = ref(false)
const groupForm = reactive({ name: '' })
const groupParentId = ref('')
const groupTarget = ref<ReviewGroup | null>(null)
const groupDeleteVisible = ref(false)
const groupDeleting = ref(false)
const groupDeleteTarget = ref<ReviewGroup | null>(null)
const groupDeleteCards = ref(false)

const pendingCardReview = ref<ReviewCard | null>(null)
const cardConflictVisible = ref(false)
const cardConflictBusy = ref(false)

const generateVisible = ref(false)
const moreOptions = computed(() => [
  { content: t('review.v3.generateCards'), value: 'generate' },
  { content: t('review.v3.exportJson'), value: 'export' },
])

function onMoreAction(item: { value: string }) {
  if (item.value === 'generate') {
    generateVisible.value = true
    return
  }
  void exportJson()
}

async function exportJson() {
  try {
    const response = await exportReviewData()
    const blob = new Blob([JSON.stringify(response.data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `wellread-review-${new Date().toISOString().slice(0, 10)}.json`
    link.click()
    URL.revokeObjectURL(url)
    MessagePlugin.success(t('review.v3.exported'))
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.v3.exportFailed'))
  }
}

function flattenGroups(nodes: ReviewGroupNode[], depth = 0): ReviewFlatGroup[] {
  return nodes.flatMap(node => [
    {
      group: node.group,
      depth,
      card_count: node.card_count || 0,
      due_count: node.due_count || 0,
    },
    ...flattenGroups(node.children || [], depth + 1),
  ])
}

const selectedGroupName = computed(() => {
  if (!selectedGroupId.value) return ''
  return flatGroups.value.find(item => item.group.id === selectedGroupId.value)?.group.name || ''
})

const allCardCount = computed(() =>
  groups.value.reduce((total, node) => total + (node.card_count || 0), 0),
)
const allDueCount = computed(() =>
  groups.value.reduce((total, node) => total + (node.due_count || 0), 0),
)

const groupOptions = computed(() => [
  { label: t('review.v3.allGroups'), value: '' },
  ...flatGroups.value.map(item => ({
    label: `${'　'.repeat(item.depth)}${item.group.name}`,
    value: item.group.id,
  })),
])

async function loadGroups() {
  const response = await listReviewGroups()
  groups.value = response.data || []
  flatGroups.value = flattenGroups(groups.value)
}

async function loadQueue() {
  const seq = ++queueRequestSeq
  queueLoading.value = true
  try {
    const response = await getReviewQueue({
      group_id: selectedGroupId.value || undefined,
    })
    if (seq !== queueRequestSeq) return
    queue.value = response.data || null
    error.value = ''
  } catch (err: any) {
    if (seq !== queueRequestSeq) return
    error.value = err?.message || t('review.loadFailed')
  } finally {
    if (seq === queueRequestSeq) queueLoading.value = false
  }
}

async function loadHeatmap() {
  const seq = ++heatmapRequestSeq
  heatmapLoading.value = true
  try {
    const response = await getReviewHeatmap({
      group_id: selectedGroupId.value || undefined,
      expand: heatmapExpanded.value || undefined,
    })
    if (seq !== heatmapRequestSeq) return
    heatmap.value = response.data || null
    heatmapError.value = ''
  } catch (err: any) {
    if (seq !== heatmapRequestSeq) return
    heatmapError.value = err?.message || t('review.loadFailed')
  } finally {
    if (seq === heatmapRequestSeq) heatmapLoading.value = false
  }
}

async function loadActiveSession() {
  try {
    const response = await getActiveReviewSession()
    activeSession.value = response.data || null
  } catch {
    activeSession.value = null
  }
}

async function refresh() {
  loading.value = true
  error.value = ''
  try {
    await Promise.all([loadGroups(), loadQueue(), loadHeatmap(), loadActiveSession()])
  } catch (err: any) {
    error.value = err?.message || t('review.loadFailed')
  } finally {
    loading.value = false
  }
}

function selectGroup(groupId: string) {
  selectedGroupId.value = groupId
  const query = { ...route.query }
  if (groupId) query.scope = groupId
  else delete query.scope
  void router.replace({ query })
}

function sessionLabel(state: ReviewSessionState) {
  return state.session.mode === 'daily' ? t('review.v3.dailyQueue') : t('review.v3.relearnQueue')
}

function openReview(state: ReviewSessionState, label: string) {
  reviewState.value = state
  reviewLabel.value = label
  reviewVisible.value = true
}

async function refreshAfterReview() {
  await Promise.all([loadQueue(), loadHeatmap(), loadActiveSession()])
}

function closeReview() {
  reviewVisible.value = false
  reviewState.value = null
  void refreshAfterReview()
}

function onReviewCompleted() {
  void refreshAfterReview()
}

async function beginSession(
  payload: Parameters<typeof startReviewSession>[0],
  label: string,
) {
  if (reviewStarting.value) return
  reviewStarting.value = true
  try {
    const response = await startReviewSession(payload)
    if (!response.data) throw new Error(t('review.loadFailed'))
    activeSession.value = response.data
    openReview(response.data, label)
    void loadQueue()
    void loadHeatmap()
  } catch (err: any) {
    await loadActiveSession()
    if (activeSession.value) {
      openReview(activeSession.value, sessionLabel(activeSession.value))
    } else {
      MessagePlugin.error(err?.message || t('review.loadFailed'))
    }
  } finally {
    reviewStarting.value = false
  }
}

function handleStart() {
  if (reviewStarting.value || reviewVisible.value) return
  if (activeSession.value) {
    openReview(activeSession.value, sessionLabel(activeSession.value))
    return
  }
  void beginSession(
    { mode: 'daily', group_id: selectedGroupId.value || undefined },
    t('review.v3.dailyQueue'),
  )
}

async function refreshAfterCardChange() {
  await Promise.all([loadGroups(), loadQueue(), loadHeatmap()])
}

async function beginCardReview(card: ReviewCard, forceNew: boolean) {
  const response = await startReviewSession({
    mode: 'manual',
    group_id: card.group_id,
    card_ids: [card.id],
    force_new: forceNew || undefined,
  })
  if (!response.data) throw new Error(t('review.loadFailed'))
  activeSession.value = response.data
  cardConflictVisible.value = false
  pendingCardReview.value = null
  openReview(response.data, t('review.v3.singleQueue'))
  void refreshAfterReview()
}

async function handleCardReview(card: ReviewCard) {
  if (reviewStarting.value || reviewVisible.value) return
  reviewStarting.value = true
  try {
    const active = await getActiveReviewSession()
    if (active.data) {
      pendingCardReview.value = card
      cardConflictVisible.value = true
      return
    }
    await beginCardReview(card, false)
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.loadFailed'))
  } finally {
    reviewStarting.value = false
  }
}

async function abandonAndReviewCard() {
  const card = pendingCardReview.value
  if (!card || cardConflictBusy.value) return
  cardConflictBusy.value = true
  try {
    await beginCardReview(card, true)
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.loadFailed'))
  } finally {
    cardConflictBusy.value = false
  }
}

function continueCurrentCardReview() {
  cardConflictVisible.value = false
  pendingCardReview.value = null
  if (activeSession.value) {
    openReview(activeSession.value, sessionLabel(activeSession.value))
  } else {
    void loadActiveSession()
  }
}

async function handleStatsReview(groupId: string) {
  if (reviewStarting.value || reviewVisible.value) return
  reviewStarting.value = true
  try {
    await beginSession({ mode: 'daily', group_id: groupId }, t('review.v3.dailyQueue'))
  } finally {
    reviewStarting.value = false
  }
}

function openCluster(cell: ReviewHeatmapCell) {
  if (cell.row_id === 'other_row' || cell.column_key === 'other_column') return
  const row = heatmap.value?.rows.find(item => item.id === cell.row_id)
  if (!row) return
  const query: Record<string, string> = { col: cell.column_key }
  if (selectedGroupId.value) query.scope = selectedGroupId.value
  if (row.kind === 'direct') query.direct = '1'
  void router.push({
    name: 'reviewV3Cluster',
    params: {
      rowGroupId: row.group_id,
    },
    query,
  })
}

function drillGroup(groupID: string) {
  heatmapExpanded.value = false
  selectGroup(groupID)
}

function expandHeatmap() {
  heatmapExpanded.value = true
  void loadHeatmap()
}

function maybeResumeFromQuery() {
  if (route.query.resume !== '1' || !activeSession.value) return
  openReview(activeSession.value, sessionLabel(activeSession.value))
  const query = { ...route.query }
  delete query.resume
  void router.replace({ query })
}

function openCreateGroup(parentId: string) {
  groupDialogMode.value = 'create'
  groupParentId.value = parentId
  groupTarget.value = null
  groupForm.name = ''
  groupDialogVisible.value = true
}

function openRenameGroup(group: ReviewGroup) {
  groupDialogMode.value = 'rename'
  groupTarget.value = group
  groupParentId.value = group.parent_id
  groupForm.name = group.name
  groupDialogVisible.value = true
}

function openDeleteGroup(group: ReviewGroup) {
  groupDeleteTarget.value = group
  groupDeleteCards.value = false
  groupDeleteVisible.value = true
}

async function saveGroup() {
  const name = groupForm.name.trim()
  if (!name) {
    MessagePlugin.warning(t('review.groupNameRequired'))
    return
  }
  groupSaving.value = true
  try {
    if (groupDialogMode.value === 'create') {
      const response = await createReviewGroup({ parent_id: groupParentId.value || undefined, name })
      MessagePlugin.success(t('review.groupCreated'))
      groupDialogVisible.value = false
      await loadGroups()
      if (response.data?.id) selectedGroupId.value = response.data.id
    } else if (groupTarget.value) {
      await updateReviewGroup(groupTarget.value.id, { name })
      MessagePlugin.success(t('review.groupUpdated'))
      groupDialogVisible.value = false
      await loadGroups()
      await loadHeatmap()
    }
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.groupFailed'))
  } finally {
    groupSaving.value = false
  }
}

async function confirmDeleteGroup() {
  const target = groupDeleteTarget.value
  if (!target) return
  groupDeleting.value = true
  try {
    await deleteReviewGroup(target.id, groupDeleteCards.value)
    MessagePlugin.success(t('review.groupDeleted'))
    groupDeleteVisible.value = false
    if (selectedGroupId.value === target.id) selectedGroupId.value = ''
    await loadGroups()
    await loadQueue()
    await loadHeatmap()
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.groupFailed'))
  } finally {
    groupDeleting.value = false
  }
}

watch(selectedGroupId, () => {
  void loadQueue()
  void loadHeatmap()
})

watch(activeView, () => {
  void nextTick(() => pageRoot.value?.scrollTo({ top: 0 }))
})

onMounted(() => {
  void refresh().then(() => {
    maybeResumeFromQuery()
  })
})
</script>

<template>
  <div ref="pageRoot" class="review-v3 rv-vuln-scope">
    <div class="review-v3__inner">
      <header class="rv-page-head">
        <div class="rv-page-head__main">
          <img class="rv-page-head__icon" :src="calendarIcon" alt="" />
          <div>
            <h1>{{ t('review.title') }}</h1>
            <p>{{ t('review.subtitle') }}</p>
          </div>
        </div>
        <div class="rv-page-head__actions">
          <t-dropdown :options="moreOptions" placement="bottom-right" @click="onMoreAction">
            <t-button variant="outline" data-testid="review-v3-more">
              {{ t('review.v3.moreActions') }}
            </t-button>
          </t-dropdown>
        </div>
      </header>

      <ReviewGenerateDialog
        v-if="generateVisible"
        v-model:visible="generateVisible"
        :flat-groups="flatGroups"
        :default-group-id="selectedGroupId"
        @imported="refreshAfterCardChange"
      />

      <t-tabs v-if="!generateVisible && !reviewVisible" v-model="activeView" class="review-v3__tabs">
        <t-tab-panel value="today" :label="t('review.v3.todayTab')">
          <div class="review-v3__today">
            <ReviewGroupRail
              :groups="flatGroups"
              :selected-group-id="selectedGroupId"
              :all-card-count="allCardCount"
              :all-due-count="allDueCount"
              @select="selectGroup"
              @create="openCreateGroup"
              @rename="openRenameGroup"
              @delete="openDeleteGroup"
            />
            <main class="review-v3__main">
              <ReviewTodayView
                :queue="queue"
                :active-session="activeSession"
                :selected-group-name="selectedGroupName"
                :selected-group-id="selectedGroupId"
                :group-options="groupOptions"
                :heatmap="heatmap"
                :heatmap-loading="heatmapLoading"
                :heatmap-error="heatmapError"
                :loading="loading || queueLoading"
                :error="error"
                @start="handleStart"
                @retry="refresh"
                @retry-heatmap="loadHeatmap"
                @open-cluster="openCluster"
                @drill="drillGroup"
                @expand="expandHeatmap"
                @update:group-id="selectGroup"
              />
            </main>
          </div>
        </t-tab-panel>

        <t-tab-panel value="cards" :label="t('review.v3.cardsTab')">
          <ReviewCardsView
            :flat-groups="flatGroups"
            :initial-group-id="selectedGroupId"
            @review-card="handleCardReview"
            @updated="refreshAfterCardChange"
          />
        </t-tab-panel>

        <t-tab-panel value="stats" :label="t('review.v3.statsTab')">
          <ReviewStatsView @review-group="handleStatsReview" />
        </t-tab-panel>
      </t-tabs>

      <footer class="rv-attribution">
        <a href="https://icons8.com" target="_blank" rel="noopener noreferrer">Icons by Icons8</a>
      </footer>
    </div>

    <t-dialog
      v-model:visible="groupDialogVisible"
      :header="groupDialogMode === 'create' ? t('review.newGroup') : t('review.renameGroup')"
      :confirm-btn="{ content: t('review.save'), loading: groupSaving }"
      :cancel-btn="{ content: t('review.cancel') }"
      width="420px"
      @confirm="saveGroup"
    >
      <t-input
        v-model="groupForm.name"
        :placeholder="t('review.groupNamePlaceholder')"
        :maxlength="200"
        @enter="saveGroup"
      />
    </t-dialog>

    <t-dialog
      v-model:visible="groupDeleteVisible"
      :header="t('review.deleteGroup')"
      :confirm-btn="{ content: t('review.confirm'), theme: 'danger', loading: groupDeleting }"
      :cancel-btn="{ content: t('review.cancel') }"
      width="440px"
      @confirm="confirmDeleteGroup"
    >
      <p class="review-v3__dialog-text">
        {{ t('review.groupDeleteConfirm', { name: groupDeleteTarget?.name || '' }) }}
      </p>
      <t-checkbox v-model="groupDeleteCards">{{ t('review.groupDeleteWithCards') }}</t-checkbox>
    </t-dialog>

    <t-dialog
      v-model:visible="cardConflictVisible"
      :header="t('review.v3.sessionConflictTitle')"
      :footer="false"
      width="460px"
    >
      <p class="review-v3__dialog-text">{{ t('review.v3.cardConflictHint') }}</p>
      <div class="review-v3__dialog-actions">
        <t-button variant="outline" :disabled="cardConflictBusy" @click="continueCurrentCardReview">
          {{ t('review.v3.continueCurrent') }}
        </t-button>
        <t-button theme="danger" :loading="cardConflictBusy" @click="abandonAndReviewCard">
          {{ t('review.v3.abandonAndReviewCard') }}
        </t-button>
      </div>
    </t-dialog>

    <ReviewMode
      v-if="reviewVisible && reviewState"
      :state="reviewState"
      :queue-label="reviewLabel"
      :streak-days="queue?.streak_days ?? 0"
      @close="closeReview"
      @completed="onReviewCompleted"
    />
  </div>
</template>

<style lang="less" scoped>
.review-v3,
.review-v3 *,
.review-v3 *::before,
.review-v3 *::after {
  box-sizing: border-box;
}

.review-v3 {
  position: relative;
  height: 100%;
  overflow-y: auto;
  background: transparent;
  color: #373530;
  font-size: 14px;
  line-height: 1.65;
  --rvn-border: #e9e7e2;
  --rvn-panel: #f1f1ef;
  --rvn-panel-hover: #ececea;
  --rvn-brand: #07c05f;
  --rvn-brand-deep: #06b04d;
  --rvn-brand-bg: #e9f8ec;
  --rvn-brand-line: #bcebdc;
  --rvn-milk: #f2efe4;
  --rvn-milk-deep: #b5a89c;
  --rvn-blue: #9db0be;
  --rvn-blue-bg: #edf2f5;
  --rvn-rose: #c9a4a0;
  --rvn-rose-bg: #f7eeed;
  --rvn-rose-text: #9e5a54;
  --rvn-gold: #c9a96e;
  --rvn-gray: #a9a9a2;
  --rvn-radius: 10px;
  --rvn-radius-sm: 6px;
  --rv-vuln-severe: #c9a4a0;
  --rv-vuln-warning: #c9a96e;
  --rv-vuln-observe: #9db0be;
  --rv-vuln-healthy: #07c05f;
  --td-brand-color: #07c05f;
  --td-brand-color-hover: #08dd6e;
  --td-brand-color-active: #06b04d;
  --td-brand-color-focus: #06b04d;
  --td-brand-color-light: #e9f8ec;
  --td-text-color-primary: #373530;
  --td-text-color-secondary: #787774;
  --td-text-color-placeholder: #9b9a95;
  --td-component-stroke: #e9e7e2;
  --td-component-border: #d5dfd2;
  --td-success-color: #07c05f;
  --td-success-color-light: #e9f8ec;
  --td-warning-color: #c9a96e;
  --td-warning-color-light: #f2efe4;
  --td-error-color: #9e5a54;
  --td-error-color-light: #f7eeed;
  --td-info-color: #5b7891;
  --td-info-color-light: #edf2f5;
}

:global([theme-mode='dark']) .review-v3 {
  color: #f2f1ee;
  --rvn-border: #3d3b37;
  --rvn-panel: #2a2926;
  --rvn-panel-hover: #33322e;
  --rvn-brand-bg: #06b04d33;
  --rvn-brand-line: #06b04d4d;
  --td-brand-color-light: #06b04d33;
  --rvn-milk: #302d26;
  --rvn-blue-bg: #263039;
  --rvn-rose-bg: #332827;
  --rvn-rose-text: #d8a8a3;
  --td-text-color-primary: #f2f1ee;
  --td-text-color-secondary: #b8b6b0;
  --td-text-color-placeholder: #8f8d87;
  --td-component-stroke: #3d3b37;
}

.review-v3__inner {
  width: 100%;
  padding: 24px 32px 48px;
}

.review-v3__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;

  h1 {
    margin: 0;
    font-size: 22px;
    line-height: 30px;
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  p {
    margin: 4px 0 0;
    font-size: 13px;
    line-height: 20px;
    color: var(--td-text-color-secondary);
  }
}

.review-v3__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.review-v3__tabs {
  margin-top: 14px;
  overflow: visible;
  background: transparent;

  :deep(.t-tabs) {
    background: transparent;
  }

  :deep(.t-tabs__content) {
    padding: 0;
    overflow: visible;
  }
}

.review-v3 :deep(.t-card) {
  background: transparent;
  box-shadow: none;
  border: 1px solid var(--rvn-border);
  border-radius: var(--rvn-radius);
}

.review-v3 :deep(.t-tabs__nav-item) {
  font-size: 13.5px;
  color: var(--td-text-color-secondary);
}

.review-v3 :deep(.t-tabs__nav-item.t-is-active) {
  color: var(--rvn-brand);
  font-weight: 600;
}

.review-v3__today {
  display: flex;
  align-items: stretch;
  gap: 20px;
  min-height: calc(100vh - 236px);
}

.review-v3__main {
  flex: 1;
  min-width: 0;
}

.review-v3__placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 360px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-medium);
}

.review-v3__note {
  margin: 0;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.review-v3__dialog-text {
  margin: 0 0 12px;
  font-size: 13px;
  line-height: 21px;
  color: var(--td-text-color-secondary);
}

.review-v3__dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

@media (max-width: 760px) {
  .review-v3__inner {
    padding: 16px 20px 32px;
  }

  .review-v3__today {
    display: block;
  }
}
</style>
