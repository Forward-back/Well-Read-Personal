<template>
  <div class="review-page">
    <header class="review-header">
      <div>
        <h1 class="review-title">{{ t('review.title') }}</h1>
        <p class="review-subtitle">{{ t('review.subtitle') }}</p>
      </div>
      <div class="review-header-actions">
        <t-button variant="outline" :loading="loading" @click="refreshAll">
          <template #icon><t-icon name="refresh" /></template>
          {{ t('review.refresh') }}
        </t-button>
        <t-button variant="outline" @click="handleExport">
          <template #icon><t-icon name="download" /></template>
          {{ t('review.export') }}
        </t-button>
        <t-button theme="primary" @click="openCreateGroup">
          <template #icon><t-icon name="add" /></template>
          {{ t('review.newGroup') }}
        </t-button>
      </div>
    </header>

    <div class="review-layout">
      <aside class="review-sidebar">
        <div class="review-sidebar-head">
          <span>{{ t('review.groups') }}</span>
          <span v-if="selectedGroup" class="review-sidebar-current">{{ selectedGroup.name }}</span>
        </div>
        <button type="button" class="review-group-row" :class="{ active: selectedGroupId === '' }"
          @click="selectGroup('')">
          <span class="review-group-name">{{ t('review.allGroups') }}</span>
          <span class="review-group-count">{{ cards.length ? t('review.cardCount', { count: cards.length }) : '' }}</span>
        </button>
        <button v-for="item in flatGroups" :key="item.group.id" type="button" class="review-group-row"
          :class="{ active: selectedGroupId === item.group.id }" @click="selectGroup(item.group.id)">
          <span class="review-group-name" :style="{ paddingLeft: `${item.depth * 14}px` }">
            <t-icon :name="item.group.is_default ? 'home' : 'folder'" size="14px" />
            {{ item.group.name }}
          </span>
        </button>
        <div v-if="selectedGroup && !selectedGroup.is_default" class="review-group-actions">
          <t-button variant="text" size="small" @click="openRenameGroup">{{ t('review.renameGroup') }}</t-button>
          <t-button variant="text" size="small" theme="danger" @click="confirmDeleteGroup">
            {{ t('review.deleteGroup') }}
          </t-button>
        </div>
      </aside>

      <main class="review-main">
        <div class="review-stats">
          <div class="review-stat">
            <span class="review-stat-label">{{ t('review.today') }}</span>
            <strong>{{ queue?.due.length ?? 0 }} {{ t('review.due') }}</strong>
            <span>{{ queue?.new.length ?? 0 }} {{ t('review.newCards') }}</span>
          </div>
          <div class="review-stat">
            <span class="review-stat-label">{{ t('review.completed') }}</span>
            <strong>{{ queue?.completed_today ?? 0 }}</strong>
            <span>/ {{ queue?.limit ?? 30 }}</span>
          </div>
          <div class="review-stat">
            <span class="review-stat-label">{{ t('review.totalCards') }}</span>
            <strong>{{ stats?.total_cards ?? cards.length }}</strong>
            <span>{{ t('review.retention30d') }} {{ Math.round((stats?.retention_30d ?? 0) * 100) }}%</span>
          </div>
          <div class="review-stat review-stat--actions">
            <t-button theme="primary" :disabled="!queueHasCards" @click="startReview">
              {{ t('review.start') }}
            </t-button>
            <span class="review-stat-hint">{{ t('review.startHint') }}</span>
          </div>
        </div>

        <section v-if="reviewing" class="review-run">
          <div class="review-run-head">
            <span>{{ t('review.reviewing') }}</span>
            <span>{{ t('review.progress', { current: reviewIndex + 1, total: reviewCards.length }) }}</span>
            <t-button variant="text" size="small" @click="exitReview">{{ t('review.exit') }}</t-button>
          </div>
          <div v-if="currentCard" class="review-card">
            <div class="review-card-meta">
              <t-tag size="small" variant="light">{{ currentCard.review_count ? 'L' + currentCard.review_level : t('review.newCards') }}</t-tag>
              <span v-if="currentCard.lapses" class="review-card-lapses">{{ currentCard.lapses }} × {{ t('review.forgot') }}</span>
            </div>
            <div class="review-question">{{ currentCard.question }}</div>
            <div v-if="showingAnswer" class="review-answer">
              <div class="review-answer-label">{{ t('review.answer') }}</div>
              <div class="review-answer-text">{{ currentCard.answer }}</div>
              <div v-if="currentCard.explanation" class="review-explanation">
                <div class="review-answer-label">{{ t('review.explanation') }}</div>
                <div class="review-answer-text">{{ currentCard.explanation }}</div>
              </div>
              <div v-if="currentDetail?.sources?.length" class="review-source">
                <div class="review-answer-label">{{ currentDetail.sources[0].knowledge_id ? t('review.cards') : t('review.sourceUnavailable') }}</div>
                <div class="review-source-text">{{ currentDetail.sources[0].excerpt || currentDetail.sources[0].path || currentDetail.sources[0].knowledge_id }}</div>
              </div>
            </div>
            <div class="review-card-actions">
              <t-button v-if="!showingAnswer" theme="primary" size="large" @click="showingAnswer = true">
                {{ t('review.showAnswer') }}
              </t-button>
              <template v-else>
                <t-button theme="danger" variant="outline" :loading="ratingLoading" @click="rate('forgot')">
                  {{ t('review.forgot') }} <span class="review-key">1</span>
                </t-button>
                <t-button theme="warning" variant="outline" :loading="ratingLoading" @click="rate('fuzzy')">
                  {{ t('review.fuzzy') }} <span class="review-key">2</span>
                </t-button>
                <t-button theme="success" :loading="ratingLoading" @click="rate('remembered')">
                  {{ t('review.remembered') }} <span class="review-key">3</span>
                </t-button>
              </template>
            </div>
            <p class="review-shortcut">{{ t('review.shortcutHint') }}</p>
          </div>
        </section>

        <section v-else-if="finished" class="review-empty">
          <h3>{{ t('review.finished') }}</h3>
          <p>{{ t('review.finishedHint', { count: sessionDone }) }}</p>
          <t-button theme="primary" @click="exitReview">{{ t('review.backHome') }}</t-button>
        </section>

        <section v-else class="review-home">
          <div v-if="queueHasCards" class="review-callout">
            <div>
              <strong>{{ t('review.today') }}</strong>
              <p>{{ t('review.startHint') }}</p>
            </div>
            <t-button theme="primary" @click="startReview">{{ t('review.start') }}</t-button>
          </div>
          <div v-else class="review-empty review-empty--compact">
            <h3>{{ t('review.noDue') }}</h3>
            <p>{{ t('review.queueEmpty') }}</p>
          </div>

          <div class="review-list-tools">
            <t-input v-model="searchKeyword" class="review-list-search" clearable
              :placeholder="t('review.searchPlaceholder')" @enter="loadCards" @clear="loadCards" />
            <t-select v-model="statusFilter" class="review-list-status" clearable
              :placeholder="t('review.allStatuses')" :options="statusOptions" @change="loadCards" />
            <t-button v-if="selectedCardIDs.length > 1" variant="outline" @click="confirmMergeCards">
              {{ t('review.mergeCards') }} ({{ selectedCardIDs.length }})
            </t-button>
          </div>
          <div class="review-list-head">
            <span>{{ t('review.cards') }}</span>
            <span>{{ t('review.cardCount', { count: cards.length }) }}</span>
          </div>
          <div v-if="cards.length" class="review-card-list">
            <article v-for="card in cards" :key="card.id" class="review-card-row">
              <t-checkbox size="small" :checked="selectedCardIDs.includes(card.id)"
                @change="(checked: boolean) => toggleCardSelection(card.id, checked)" />
              <div class="review-card-row-main">
                <div class="review-card-row-question">{{ card.question }}</div>
                <div class="review-card-row-meta">
                  <t-tag size="small" :theme="statusTheme(card.status)" variant="light">
                    {{ t(`review.status.${card.status}`) }}
                  </t-tag>
                  <span>L{{ card.review_level }}</span>
                  <span v-if="card.review_count">×{{ card.review_count }}</span>
                  <span v-if="card.lapses" class="review-card-lapses">{{ card.lapses }} × {{ t('review.forgot') }}</span>
                </div>
              </div>
              <div class="review-card-row-actions">
                <t-button v-if="card.status === 'active'" variant="text" size="small" @click="toggleSuspend(card)">
                  {{ t('review.suspend') }}
                </t-button>
                <t-button v-if="card.status === 'suspended'" variant="text" size="small" @click="toggleSuspend(card)">
                  {{ t('review.resume') }}
                </t-button>
                <t-button v-if="card.status === 'orphaned'" variant="text" size="small" @click="openRebind(card)">
                  {{ t('review.rebindSource') }}
                </t-button>
                <t-button variant="text" size="small" theme="danger" @click="confirmDeleteCard(card)">
                  {{ t('review.deleteCard') }}
                </t-button>
              </div>
            </article>
          </div>
          <div v-else class="review-empty review-empty--compact">
            <p>{{ t('review.noCards') }}</p>
          </div>
        </section>
      </main>
    </div>

    <t-dialog v-model:visible="groupDialogVisible" :header="groupDialogMode === 'create' ? t('review.newGroup') : t('review.renameGroup')"
      :footer="false" width="420px" destroy-on-close>
      <div class="review-dialog-body">
        <label class="review-dialog-label">{{ t('review.groupName') }}</label>
        <t-input v-model="groupForm.name" :placeholder="t('review.groupNamePlaceholder')" :maxlength="200" @enter="saveGroup" />
        <p v-if="groupDialogMode === 'create' && selectedGroup && !selectedGroup.is_default" class="review-dialog-hint">
          {{ t('review.groupParent') }}：{{ selectedGroup.name }}
        </p>
        <p v-else-if="groupDialogMode === 'create'" class="review-dialog-hint">
          {{ t('review.groupParent') }}：{{ t('review.rootGroup') }}
        </p>
        <div class="review-dialog-footer">
          <t-button variant="outline" @click="groupDialogVisible = false">{{ t('review.cancel') }}</t-button>
          <t-button theme="primary" :loading="groupSaving" @click="saveGroup">{{ t('review.confirm') }}</t-button>
        </div>
      </div>
    </t-dialog>

    <t-dialog v-model:visible="rebindVisible" :header="t('review.rebindSource')" :footer="false" width="460px"
      destroy-on-close>
      <div class="review-dialog-body">
        <label class="review-dialog-label">{{ t('review.knowledgeId') }}</label>
        <t-input v-model="rebindForm.knowledgeID" :placeholder="t('review.knowledgeId')" />
        <label class="review-dialog-label">{{ t('review.chunkId') }}</label>
        <t-input v-model="rebindForm.chunkID" :placeholder="t('review.chunkId')" />
        <label class="review-dialog-label">{{ t('review.excerpt') }}</label>
        <t-textarea v-model="rebindForm.excerpt" :autosize="{ minRows: 3, maxRows: 6 }" />
        <div class="review-dialog-footer">
          <t-button variant="outline" @click="rebindVisible = false">{{ t('review.cancel') }}</t-button>
          <t-button theme="primary" :loading="rebindSaving" @click="saveRebind">{{ t('review.confirm') }}</t-button>
        </div>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import {
  createReviewGroup,
  deleteReviewCard,
  deleteReviewGroup,
  getReviewCard,
  exportReviewData,
  getReviewQueue,
  getReviewStats,
  type ReviewStats,
  listReviewCards,
  listReviewGroups,
  mergeReviewCards,
  rebindReviewSource,
  setReviewCardStatus,
  submitReview,
  updateReviewGroup,
  type ReviewCard,
  type ReviewCardDetail,
  type ReviewGroup,
  type ReviewGroupNode,
  type ReviewQueue,
} from '@/api/review'

const { t } = useI18n()

interface FlatGroup {
  group: ReviewGroup
  depth: number
}

const groups = ref<ReviewGroupNode[]>([])
const flatGroups = ref<FlatGroup[]>([])
const selectedGroupId = ref('')
const cards = ref<ReviewCard[]>([])
const searchKeyword = ref('')
const statusFilter = ref('')
const selectedCardIDs = ref<string[]>([])
const queue = ref<ReviewQueue | null>(null)
const stats = ref<ReviewStats | null>(null)
const loading = ref(false)
const ratingLoading = ref(false)
const groupDialogVisible = ref(false)
const groupDialogMode = ref<'create' | 'rename'>('create')
const groupSaving = ref(false)
const groupForm = reactive({ name: '' })
const reviewing = ref(false)
const finished = ref(false)
const reviewCards = ref<ReviewCard[]>([])
const reviewIndex = ref(0)
const showingAnswer = ref(false)
const sessionDone = ref(0)
const currentDetail = ref<ReviewCardDetail | null>(null)
const rebindVisible = ref(false)
const rebindSaving = ref(false)
const rebindSourceID = ref('')
const rebindCardID = ref('')
const rebindForm = reactive({ knowledgeID: '', chunkID: '', excerpt: '' })

const selectedGroup = computed(() => flatGroups.value.find(item => item.group.id === selectedGroupId.value)?.group ?? null)
const queueHasCards = computed(() => Boolean(queue.value && (queue.value.due.length + queue.value.new.length) > 0))
const currentCard = computed(() => reviewCards.value[reviewIndex.value] ?? null)
const statusOptions = computed(() => [
  { label: t('review.status.active'), value: 'active' },
  { label: t('review.status.suspended'), value: 'suspended' },
  { label: t('review.status.orphaned'), value: 'orphaned' },
])

function flattenGroups(nodes: ReviewGroupNode[], depth = 0): FlatGroup[] {
  return nodes.flatMap(node => [
    { group: node.group, depth },
    ...flattenGroups(node.children || [], depth + 1),
  ])
}

async function loadGroups() {
  const response = await listReviewGroups()
  groups.value = response.data || []
  flatGroups.value = flattenGroups(groups.value)
  if (!selectedGroupId.value && flatGroups.value.length) {
    const defaultGroup = flatGroups.value.find(item => item.group.is_default)
    selectedGroupId.value = defaultGroup?.group.id || ''
  }
}

async function loadCards() {
  const response = await listReviewCards({
    group_id: selectedGroupId.value || undefined,
    include_subtree: true,
    status: statusFilter.value || undefined,
    keyword: searchKeyword.value.trim() || undefined,
    limit: 200,
  })
  cards.value = response.data || []
  const visible = new Set(cards.value.map(card => card.id))
  selectedCardIDs.value = selectedCardIDs.value.filter(id => visible.has(id))
}

async function loadQueue() {
  const response = await getReviewQueue({ group_id: selectedGroupId.value || undefined, limit: 30 })
  queue.value = response.data || { due: [], new: [], completed_today: 0, limit: 30 }
}

async function loadStats() {
  const response = await getReviewStats()
  stats.value = response.data || null
}

async function refreshAll() {
  loading.value = true
  try {
    await loadGroups()
    await Promise.all([loadCards(), loadQueue(), loadStats()])
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('review.loadFailed'))
  } finally {
    loading.value = false
  }
}

function selectGroup(groupId: string) {
  selectedGroupId.value = groupId
  selectedCardIDs.value = []
  void refreshAll()
}

function toggleCardSelection(cardID: string, checked: boolean) {
  if (checked) {
    if (!selectedCardIDs.value.includes(cardID)) selectedCardIDs.value.push(cardID)
  } else {
    selectedCardIDs.value = selectedCardIDs.value.filter(id => id !== cardID)
  }
}

function confirmMergeCards() {
  if (selectedCardIDs.value.length < 2) {
    MessagePlugin.warning(t('review.mergeNeedTwo'))
    return
  }
  const [targetID, ...sourceIDs] = selectedCardIDs.value
  const dialog = DialogPlugin.confirm({
    header: t('review.mergeCards'),
    body: t('review.mergeConfirm', { count: sourceIDs.length }),
    cancelBtn: t('review.cancel'),
    onConfirm: async () => {
      dialog.hide()
      try {
        await mergeReviewCards({ target_card_id: targetID, source_card_ids: sourceIDs })
        MessagePlugin.success(t('review.merged'))
        selectedCardIDs.value = []
        await Promise.all([loadCards(), loadQueue(), loadStats()])
      } catch (error: any) {
        MessagePlugin.error(error?.message || t('review.actionFailed'))
      }
    },
  })
}

async function handleExport() {
  try {
    const response = await exportReviewData()
    const blob = new Blob([JSON.stringify(response.data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `wellread-review-${new Date().toISOString().slice(0, 10)}.json`
    link.click()
    URL.revokeObjectURL(url)
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('review.exportFailed'))
  }
}

function openCreateGroup() {
  groupDialogMode.value = 'create'
  groupForm.name = ''
  groupDialogVisible.value = true
}

function openRenameGroup() {
  if (!selectedGroup.value) return
  groupDialogMode.value = 'rename'
  groupForm.name = selectedGroup.value.name
  groupDialogVisible.value = true
}

async function saveGroup() {
  const name = groupForm.name.trim()
  if (!name) {
    MessagePlugin.warning(t('review.groupNamePlaceholder'))
    return
  }
  groupSaving.value = true
  try {
    if (groupDialogMode.value === 'create') {
      const parentID = selectedGroup.value && !selectedGroup.value.is_default ? selectedGroup.value.id : undefined
      await createReviewGroup({ name, parent_id: parentID })
      MessagePlugin.success(t('review.groupCreated'))
    } else if (selectedGroup.value) {
      await updateReviewGroup(selectedGroup.value.id, { name })
      MessagePlugin.success(t('review.groupUpdated'))
    }
    groupDialogVisible.value = false
    await refreshAll()
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('review.groupFailed'))
  } finally {
    groupSaving.value = false
  }
}

function confirmDeleteGroup() {
  if (!selectedGroup.value) return
  const group = selectedGroup.value
  const dialog = DialogPlugin.confirm({
    header: t('review.deleteGroup'),
    body: t('review.groupDeleteConfirm', { name: group.name }),
    cancelBtn: t('review.cancel'),
    onConfirm: async () => {
      dialog.hide()
      try {
        await deleteReviewGroup(group.id)
        MessagePlugin.success(t('review.groupDeleted'))
        selectedGroupId.value = ''
        await refreshAll()
      } catch (error: any) {
        MessagePlugin.error(error?.message || t('review.groupFailed'))
      }
    },
  })
}

async function loadCurrentDetail() {
  currentDetail.value = null
  if (!currentCard.value) return
  try {
    const response = await getReviewCard(currentCard.value.id)
    currentDetail.value = response.data || null
  } catch {
    currentDetail.value = null
  }
}

function startReview() {
  if (!queue.value) return
  const cardsToReview = [...queue.value.due, ...queue.value.new]
  if (!cardsToReview.length) {
    MessagePlugin.info(t('review.noDue'))
    return
  }
  reviewCards.value = cardsToReview
  reviewIndex.value = 0
  sessionDone.value = 0
  showingAnswer.value = false
  finished.value = false
  reviewing.value = true
  void loadCurrentDetail()
}

function nextCard() {
  showingAnswer.value = false
  currentDetail.value = null
  if (reviewIndex.value + 1 >= reviewCards.value.length) {
    finished.value = true
    reviewing.value = false
    void refreshAll()
    return
  }
  reviewIndex.value += 1
  void loadCurrentDetail()
}

async function rate(rating: 'forgot' | 'fuzzy' | 'remembered') {
  if (!currentCard.value || ratingLoading.value) return
  ratingLoading.value = true
  try {
    await submitReview({
      card_id: currentCard.value.id,
      rating,
      client_request_id: crypto.randomUUID(),
    })
    sessionDone.value += 1
    nextCard()
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('review.actionFailed'))
  } finally {
    ratingLoading.value = false
  }
}

function exitReview() {
  reviewing.value = false
  finished.value = false
  showingAnswer.value = false
  currentDetail.value = null
  void refreshAll()
}

function onKeydown(event: KeyboardEvent) {
  if (!reviewing.value) return
  const target = event.target as HTMLElement | null
  if (target && (['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName) || target.isContentEditable)) return
  if (event.code === 'Space' || event.code === 'Enter') {
    if (!showingAnswer.value && currentCard.value) {
      event.preventDefault()
      showingAnswer.value = true
    }
    return
  }
  if (event.key === 'Escape') {
    exitReview()
    return
  }
  if (!showingAnswer.value || ratingLoading.value) return
  if (event.key === '1') void rate('forgot')
  else if (event.key === '2') void rate('fuzzy')
  else if (event.key === '3') void rate('remembered')
}

async function openRebind(card: ReviewCard) {
  try {
    const response = await getReviewCard(card.id)
    const source = response.data?.sources?.find(item => item.source_type === 'knowledge_chunk')
    if (!source) {
      MessagePlugin.warning(t('review.noKnowledgeSource'))
      return
    }
    rebindCardID.value = card.id
    rebindSourceID.value = source.id
    rebindForm.knowledgeID = source.knowledge_id
    rebindForm.chunkID = source.chunk_id
    rebindForm.excerpt = source.excerpt
    rebindVisible.value = true
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('review.actionFailed'))
  }
}

async function saveRebind() {
  if (!rebindCardID.value || !rebindSourceID.value || !rebindForm.knowledgeID.trim()) {
    MessagePlugin.warning(t('review.rebindRequired'))
    return
  }
  rebindSaving.value = true
  try {
    await rebindReviewSource(rebindCardID.value, rebindSourceID.value, {
      knowledge_id: rebindForm.knowledgeID.trim(),
      chunk_id: rebindForm.chunkID.trim(),
      excerpt: rebindForm.excerpt.trim(),
    })
    MessagePlugin.success(t('review.rebound'))
    rebindVisible.value = false
    await Promise.all([loadCards(), loadQueue(), loadStats()])
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('review.actionFailed'))
  } finally {
    rebindSaving.value = false
  }
}

async function toggleSuspend(card: ReviewCard) {
  const nextStatus = card.status === 'suspended' ? 'active' : 'suspended'
  try {
    await setReviewCardStatus(card.id, nextStatus)
    MessagePlugin.success(t('review.cardUpdated'))
    await Promise.all([loadCards(), loadQueue(), loadStats()])
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('review.actionFailed'))
  }
}

function confirmDeleteCard(card: ReviewCard) {
  const dialog = DialogPlugin.confirm({
    header: t('review.deleteCard'),
    body: t('review.deleteCardConfirm'),
    cancelBtn: t('review.cancel'),
    onConfirm: async () => {
      dialog.hide()
      try {
        await deleteReviewCard(card.id)
        MessagePlugin.success(t('review.cardDeleted'))
        await Promise.all([loadCards(), loadQueue(), loadStats()])
      } catch (error: any) {
        MessagePlugin.error(error?.message || t('review.actionFailed'))
      }
    },
  })
}

function statusTheme(status: ReviewCard['status']) {
  if (status === 'active') return 'success'
  if (status === 'suspended') return 'warning'
  if (status === 'orphaned') return 'danger'
  return 'default'
}

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  void refreshAll()
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
})
</script>

<style scoped>
.review-page {
  height: 100%;
  overflow-y: auto;
  padding: 28px 32px 48px;
  background: var(--td-bg-color-page);
  color: var(--td-text-color-primary);
}
.review-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  max-width: 1200px;
  margin: 0 auto 24px;
}
.review-title {
  margin: 0 0 6px;
  font-size: 26px;
  font-weight: 700;
}
.review-subtitle {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 14px;
}
.review-header-actions {
  display: flex;
  gap: 10px;
}
.review-layout {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: 20px;
  max-width: 1200px;
  margin: 0 auto;
}
.review-sidebar,
.review-main {
  border: 1px solid var(--td-component-stroke);
  border-radius: 12px;
  background: var(--td-bg-color-container);
}
.review-sidebar {
  min-height: 520px;
  padding: 14px 10px;
}
.review-sidebar-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 8px 10px;
  color: var(--td-text-color-secondary);
  font-size: 13px;
}
.review-sidebar-current {
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--td-text-color-placeholder);
}
.review-group-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  min-height: 36px;
  padding: 6px 10px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--td-text-color-primary);
  cursor: pointer;
  text-align: left;
}
.review-group-row:hover,
.review-group-row.active {
  background: var(--td-bg-color-container-hover);
}
.review-group-row.active {
  color: var(--td-brand-color);
}
.review-group-name {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.review-group-count {
  flex: 0 0 auto;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}
.review-group-actions {
  display: flex;
  gap: 2px;
  padding: 8px;
  border-top: 1px solid var(--td-component-stroke);
  margin-top: 8px;
}
.review-main {
  min-width: 0;
  padding: 20px;
}
.review-stats {
  display: flex;
  align-items: center;
  gap: 28px;
  padding-bottom: 18px;
  border-bottom: 1px solid var(--td-component-stroke);
  margin-bottom: 18px;
}
.review-stat {
  display: flex;
  align-items: baseline;
  gap: 6px;
  flex-wrap: wrap;
  font-size: 13px;
  color: var(--td-text-color-secondary);
}
.review-stat strong {
  color: var(--td-text-color-primary);
  font-size: 18px;
}
.review-stat--actions {
  margin-left: auto;
  flex-wrap: nowrap;
}
.review-stat-hint {
  max-width: 220px;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}
.review-callout {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 18px 20px;
  border-radius: 10px;
  background: var(--td-bg-color-container-hover);
  margin-bottom: 18px;
}
.review-callout p {
  margin: 6px 0 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
}
.review-run-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  margin-bottom: 16px;
}
.review-card {
  max-width: 760px;
  margin: 0 auto;
  padding: 28px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 14px;
  background: var(--td-bg-color-container);
}
.review-card-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 18px;
}
.review-card-lapses {
  color: var(--td-error-color);
  font-size: 12px;
}
.review-question {
  font-size: 22px;
  font-weight: 650;
  line-height: 1.55;
  white-space: pre-wrap;
}
.review-answer {
  margin-top: 22px;
  padding-top: 18px;
  border-top: 1px dashed var(--td-component-stroke);
}
.review-answer-label {
  margin-bottom: 6px;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}
.review-answer-text {
  font-size: 16px;
  line-height: 1.7;
  white-space: pre-wrap;
}
.review-explanation,
.review-source {
  margin-top: 16px;
}
.review-source-text {
  max-height: 120px;
  overflow-y: auto;
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--td-bg-color-container-hover);
  color: var(--td-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
}
.review-card-actions {
  display: flex;
  gap: 12px;
  margin-top: 24px;
}
.review-key {
  margin-left: 4px;
  opacity: 0.6;
  font-size: 12px;
}
.review-shortcut {
  margin: 14px 0 0;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}
.review-empty {
  padding: 48px 20px;
  text-align: center;
}
.review-empty--compact {
  padding: 28px 20px;
}
.review-empty h3 {
  margin: 0 0 8px;
}
.review-empty p {
  margin: 0 0 16px;
  color: var(--td-text-color-secondary);
}
.review-list-tools {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}
.review-list-search {
  max-width: 300px;
}
.review-list-status {
  width: 130px;
}
.review-list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  margin-bottom: 10px;
}
.review-card-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.review-card-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 10px;
}
.review-card-row-main {
  min-width: 0;
}
.review-card-row-question {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 550;
}
.review-card-row-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 6px;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}
.review-card-row-actions {
  display: flex;
  flex: 0 0 auto;
  gap: 2px;
}
.review-dialog-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.review-dialog-label {
  color: var(--td-text-color-secondary);
  font-size: 13px;
}
.review-dialog-hint {
  margin: 0;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}
.review-dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}
@media (max-width: 860px) {
  .review-page {
    padding: 20px 16px 40px;
  }
  .review-layout {
    grid-template-columns: 1fr;
  }
  .review-sidebar {
    min-height: 0;
  }
  .review-stats {
    flex-wrap: wrap;
    gap: 14px;
  }
  .review-stat--actions {
    margin-left: 0;
  }
  .review-card-actions {
    flex-wrap: wrap;
  }
}
</style>
