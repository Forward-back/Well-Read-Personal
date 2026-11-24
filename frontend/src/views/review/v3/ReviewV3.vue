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
import ReviewMode from './ReviewMode.vue'
import ReviewTodayView from './ReviewTodayView.vue'
import ReviewIcon from './ReviewIcon.vue'
import type { ReviewFlatGroup } from './types'
import './vulnerability.css'
import './review-ui.css'

const ReviewCardsView = defineAsyncComponent(() => import('./ReviewCardsView.vue'))
const ReviewStatsView = defineAsyncComponent(() => import('./ReviewStatsView.vue'))

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const activeView = ref<'today' | 'cards' | 'stats'>('today')
const viewMeta = computed(() => {
  if (activeView.value === 'cards') {
    return { label: t('review.v3.cardsPageTitle'), subtitle: t('review.v3.cardsPage.subtitle') }
  }
  if (activeView.value === 'stats') {
    return { label: t('review.v3.statsTab'), subtitle: t('review.v3.statsSubtitle') }
  }
  return { label: t('review.v3.todayTab'), subtitle: t('review.subtitle') }
})
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
const heatmapRowGroupIds = ref<string[]>([])
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
const cardsDetail = ref(false)
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
      row_group_ids: heatmapRowGroupIds.value.length ? heatmapRowGroupIds.value : undefined,
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
  heatmapRowGroupIds.value = []
  const query = { ...route.query }
  if (groupId) query.scope = groupId
  else delete query.scope
  void router.replace({ query })
}

function selectHeatmapRows(ids: string[]) {
  heatmapRowGroupIds.value = ids
  void loadHeatmap()
}

function selectView(view: string) {
  if (view === 'today' || view === 'cards' || view === 'stats') {
    activeView.value = view
  }
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
      <header v-if="!generateVisible && !reviewVisible && !cardsDetail" class="rv-page-head">
        <div class="rv-page-head__main">
          <h1>{{ viewMeta.label }}</h1>
        </div>
        <div class="rv-page-head__actions">
          <nav class="review-v3__nav">
            <button
              v-for="view in [
                { key: 'today', label: t('review.v3.todayTab') },
                { key: 'cards', label: t('review.v3.cardsTab') },
                { key: 'stats', label: t('review.v3.statsTab') },
              ]"
              :key="view.key"
              type="button"
              class="t-tabs__nav-item"
              :class="{ 't-is-active': activeView === view.key }"
              @click="selectView(view.key)"
            >
              {{ view.label }}
            </button>
          </nav>
          <t-dropdown :options="moreOptions" placement="bottom-right" @click="onMoreAction">
            <button type="button" class="rv-btn rv-btn--ghost rv-btn--sm" data-testid="review-v3-more">
              {{ t('review.v3.moreActions') }}
              <ReviewIcon name="chevron-down" :size="12" />
            </button>
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

      <ReviewTodayView
        v-if="!generateVisible && !reviewVisible && !cardsDetail && activeView === 'today'"
        :queue="queue"
        :active-session="activeSession"
        :selected-group-name="selectedGroupName"
        :selected-group-id="selectedGroupId"
        :group-options="groupOptions"
        :groups="flatGroups"
        :all-card-count="allCardCount"
        :all-due-count="allDueCount"
        :heatmap="heatmap"
        :heatmap-loading="heatmapLoading"
        :heatmap-error="heatmapError"
        :row-group-ids="heatmapRowGroupIds"
        :loading="loading || queueLoading"
        :error="error"
        @select-group="selectGroup"
        @create-group="openCreateGroup"
        @rename-group="openRenameGroup"
        @delete-group="openDeleteGroup"
        @start="handleStart"
        @retry="refresh"
        @retry-heatmap="loadHeatmap"
        @update:row-group-ids="selectHeatmapRows"
        @open-cluster="openCluster"
        @drill="drillGroup"
        @expand="expandHeatmap"
        @update:group-id="selectGroup"
      />

      <ReviewCardsView
        v-if="!generateVisible && !reviewVisible && activeView === 'cards'"
        :flat-groups="flatGroups"
        :initial-group-id="selectedGroupId"
        @review-card="handleCardReview"
        @updated="refreshAfterCardChange"
        @detail="cardsDetail = $event"
      />

      <ReviewStatsView
        v-if="!generateVisible && !reviewVisible && !cardsDetail && activeView === 'stats'"
        @review-group="handleStatsReview"
      />

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
  color: var(--rvn-ink, #3f3a34);
  font-size: 15px;
  line-height: 1.5;
  letter-spacing: -0.003em;
  --rvn-canvas: #f1ede6;
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
  --rvn-purple: #978ba8;
  --rvn-purple-deep: #6b5f7a;
  --rvn-purple-soft: #e6e1ea;
  --rvn-brand: #5a7688;
  --rvn-brand-deep: #4c6678;
  --rvn-brand-bg: #e7ecef;
  --rvn-brand-line: #c9d4db;
  --rvn-radius: 11px;
  --rvn-radius-sm: 7px;
  --rvn-border: var(--rvn-line);
  --rvn-panel: var(--rvn-surface);
  --rvn-panel-hover: var(--rvn-surface-deep);
  --rv-vuln-severe: #8a4e47;
  --rv-vuln-warning: #7e5e42;
  --rv-vuln-observe: #47677a;
  --rv-vuln-healthy: #5e7261;
  --td-brand-color: #5a7688;
  --td-brand-color-hover: #6e93a8;
  --td-brand-color-active: #4c6678;
  --td-brand-color-focus: #4c6678;
  --td-brand-color-light: #e7ecef;
  --td-text-color-primary: #3f3a34;
  --td-text-color-secondary: #6e675e;
  --td-text-color-placeholder: #857c70;
  --td-text-color-brand: #4c6678;
  --td-component-stroke: #e0dbd1;
  --td-component-border: #e0dbd1;
  --td-bg-color-container: #fbf9f5;
  --td-bg-color-container-hover: #f0ece5;
  --td-bg-color-container-active: #e9e4db;
  --td-success-color: #5e7261;
  --td-success-color-light: #e3e9df;
  --td-warning-color: #7e5e42;
  --td-warning-color-light: #ece3d8;
  --td-error-color: #8a4e47;
  --td-error-color-light: #ebdcd7;
  --td-info-color: #47677a;
  --td-info-color-light: #dfe7ea;
}

:global([theme-mode='dark']) .review-v3 {
  color: #f2f1ee;
  --rvn-canvas: #262420;
  --rvn-paper: #2b2925;
  --rvn-surface: #33302a;
  --rvn-surface-deep: #3b3731;
  --rvn-ink: #f2f1ee;
  --rvn-ink-soft: #c7c2b9;
  --rvn-muted: #9c948a;
  --rvn-line: #403c35;
  --rvn-line-strong: rgba(242, 241, 238, 0.18);
  --rvn-gray-soft: #35312b;
  --rvn-brand-bg: #28323a;
  --rvn-brand-line: #3b4a55;
  --rvn-accent-soft: #263039;
  --rvn-red-soft: #332827;
  --rvn-orange-soft: #332d25;
  --rvn-yellow-soft: #333026;
  --rvn-green-soft: #28302a;
  --rvn-purple-soft: #2e2936;
  --td-brand-color-light: #28323a;
  --td-text-color-primary: #f2f1ee;
  --td-text-color-secondary: #c7c2b9;
  --td-text-color-placeholder: #9c948a;
  --td-component-stroke: #403c35;
  --td-bg-color-container: #2b2925;
  --td-bg-color-container-hover: #33302a;
  --td-bg-color-container-active: #3b3731;
}

.review-v3__inner {
  width: 100%;
  max-width: none;
  margin: 0;
  padding: 24px 10% 48px;
  background: transparent;
  border: 0;
  border-radius: 0;
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
    font-size: 14px;
    line-height: 20px;
    color: var(--td-text-color-secondary);
  }
}

.review-v3__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.review-v3__nav {
  display: flex;
  gap: 20px;
  align-items: center;
  margin-right: 10px;
}

.review-v3__nav .t-tabs__nav-item {
  padding: 3px 1px;
  font-family: inherit;
  font-size: 14px;
  color: var(--rvn-muted);
  cursor: pointer;
  background: transparent;
  border: 0;
}

.review-v3__nav .t-tabs__nav-item:hover {
  color: var(--rvn-ink);
}

.review-v3__nav .t-tabs__nav-item.t-is-active {
  font-weight: 600;
  color: var(--rvn-brand-deep);
}

.review-v3__tabs {
  display: flex;
  gap: 0;
  margin: 6px 0 24px;
  border-bottom: 1px solid var(--rvn-line);
}

.review-v3__tabs .t-tabs__nav-item {
  position: relative;
  padding: 8px 2px 10px;
  margin-right: 28px;
  font-family: inherit;
  font-size: 14px;
  color: var(--rvn-muted);
  cursor: pointer;
  background: transparent;
  border: 0;
}

.review-v3__tabs .t-tabs__nav-item:hover {
  color: var(--rvn-ink);
}

.review-v3__tabs .t-tabs__nav-item.t-is-active {
  font-weight: 600;
  color: var(--rvn-brand-deep);
}

.review-v3__tabs .t-tabs__nav-item.t-is-active::after {
  position: absolute;
  right: 0;
  bottom: -1px;
  left: 0;
  height: 2px;
  content: '';
  background: var(--rvn-brand);
}

.review-v3 :deep(.t-card) {
  background: transparent;
  box-shadow: none;
  border: 1px solid var(--rvn-border);
  border-radius: var(--rvn-radius);
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
  font-size: 14px;
  color: var(--td-text-color-placeholder);
}

.review-v3__dialog-text {
  margin: 0 0 12px;
  font-size: 14px;
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
}
</style>
