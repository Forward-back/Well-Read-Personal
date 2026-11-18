/**
 * Cards tab for review V3: compact 56px rows, server-side filters, batch
 * actions, performance drawer, create/edit dialogs. Reuses the existing
 * review API; generation/merge/rebind stay on the legacy page until parity.
 */
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'

import {
  batchReviewCards,
  createReviewCard,
  deleteReviewCard,
  getReviewCard,
  getReviewCardPerformance,
  listReviewCards,
  mergeReviewCards,
  rebindReviewSource,
  resetReviewCardSchedule,
  restoreReviewCard,
  setReviewCardStatus,
  updateReviewCard,
  type ReviewBatchAction,
  type ReviewCard,
  type ReviewCardPerformance,
  type ReviewCardSourceInput,
} from '@/api/review'
import { listKnowledgeBases, listKnowledgeFiles } from '@/api/knowledge-base'
import ReviewLevelTag from './ReviewLevelTag.vue'
import type { ReviewFlatGroup } from './types'
import { holdDays, normalizeVulnerability } from './vulnerability'
import { useConfirmDelete } from '@/components/settings/useConfirmDelete'
import checklistIcon from '@/assets/icons/animated/checklist.gif'
import bookIcon from '@/assets/icons/animated/book.gif'
import searchIcon from '@/assets/icons/hand/search.png'
import plusIcon from '@/assets/icons/hand/plus.png'
import moreIcon from '@/assets/icons/hand/more.png'
import closeIcon from '@/assets/icons/hand/close.png'

const props = defineProps<{
  flatGroups: ReviewFlatGroup[]
  initialGroupId: string
}>()

const emit = defineEmits<{
  (e: 'review-card', card: ReviewCard): void
  (e: 'updated'): void
}>()

const { t } = useI18n()
const confirmDelete = useConfirmDelete()

const PAGE_SIZE = 100
const MAX_SELECTION = 100
const cards = ref<ReviewCard[]>([])
const loading = ref(false)
const loadingMore = ref(false)
const error = ref('')
const selectedIds = ref<string[]>([])
const filters = reactive({
  keyword: '',
  status: 'all',
  groupId: props.initialGroupId,
  tag: '',
  dueOnly: false,
})
let loadSeq = 0

const performanceVisible = ref(false)
const performanceLoading = ref(false)
const performance = ref<ReviewCardPerformance | null>(null)
const performanceCardId = ref('')

const editVisible = ref(false)
const editSaving = ref(false)
const editForm = reactive({ id: '', question: '', answer: '', explanation: '', tags: [] as string[] })

const createVisible = ref(false)
const createSaving = ref(false)
const createForm = reactive({
  groupId: '',
  question: '',
  answer: '',
  explanation: '',
  tags: [] as string[],
  knowledgeBaseId: '',
  knowledgeId: '',
  excerpt: '',
})
const kbOptions = ref<Array<{ label: string; value: string }>>([])
const docOptions = ref<Array<{ label: string; value: string }>>([])
const docsLoading = ref(false)

const moveVisible = ref(false)
const moveSaving = ref(false)
const moveGroupId = ref('')
const tagsVisible = ref(false)
const tagsSaving = ref(false)
const batchTags = ref<string[]>([])

const rebindVisible = ref(false)
const rebindSaving = ref(false)
const rebindCardId = ref('')
const rebindSourceId = ref('')
const rebindForm = reactive({ knowledgeId: '', chunkId: '', excerpt: '' })

const groupNameByID = computed(() => {
  const map = new Map<string, string>()
  for (const item of props.flatGroups) map.set(item.group.id, item.group.name)
  return map
})

const groupOptions = computed(() => [
  { label: t('review.v3.allGroups'), value: '' },
  ...props.flatGroups.map(item => ({
    label: `${'　'.repeat(item.depth)}${item.group.name}`,
    value: item.group.id,
  })),
])

const statusOptions = computed(() => [
  { label: t('review.allStatuses'), value: 'all' },
  { label: t('review.status.active'), value: 'active' },
  { label: t('review.status.suspended'), value: 'suspended' },
  { label: t('review.status.orphaned'), value: 'orphaned' },
  { label: t('review.status.deleted'), value: 'deleted' },
])

const hasMore = computed(() => cards.value.length > 0 && cards.value.length % PAGE_SIZE === 0)

function cardTags(card: ReviewCard): string[] {
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

function groupName(card: ReviewCard) {
  return groupNameByID.value.get(card.group_id) || '—'
}

function levelOf(card: ReviewCard) {
  return normalizeVulnerability(card.vulnerability)
}

function showsLevel(card: ReviewCard) {
  const level = levelOf(card)
  return card.status === 'active' && (level === 'severe' || level === 'warning' || level === 'observe')
}

async function loadCards({ append = false } = {}) {
  const seq = ++loadSeq
  if (append) loadingMore.value = true
  else loading.value = true
  error.value = ''
  try {
    const response = await listReviewCards({
      group_id: filters.groupId || undefined,
      include_subtree: true,
      status: filters.status === 'all' ? undefined : filters.status,
      keyword: filters.keyword.trim() || undefined,
      tag: filters.tag.trim() || undefined,
      due_only: filters.dueOnly || undefined,
      limit: PAGE_SIZE,
      offset: append ? cards.value.length : 0,
    })
    if (seq !== loadSeq) return
    const next = response.data || []
    cards.value = append ? cards.value.concat(next) : next
    if (!append) selectedIds.value = []
  } catch (err: any) {
    if (seq !== loadSeq) return
    error.value = err?.message || t('review.loadFailed')
    if (!append) cards.value = []
  } finally {
    if (seq === loadSeq) {
      loading.value = false
      loadingMore.value = false
    }
  }
}

function resetFilters() {
  filters.keyword = ''
  filters.status = 'all'
  filters.tag = ''
  filters.dueOnly = false
  void loadCards()
}

watch(() => props.initialGroupId, (value) => {
  if (filters.groupId !== value) {
    filters.groupId = value
    void loadCards()
  }
})

watch(() => [filters.status, filters.groupId, filters.dueOnly], () => {
  void loadCards()
})

function toggleSelection(cardId: string, checked: boolean) {
  const next = new Set(selectedIds.value)
  if (checked) {
    if (next.size >= MAX_SELECTION) {
      MessagePlugin.warning(t('review.v3.cardsPage.selectionLimit', { count: MAX_SELECTION }))
      return
    }
    next.add(cardId)
  } else {
    next.delete(cardId)
  }
  selectedIds.value = [...next]
}

function clearSelection() {
  selectedIds.value = []
}

async function runBatch(action: ReviewBatchAction, extra: { group_id?: string; tags?: string[] } = {}) {
  const ids = [...selectedIds.value]
  if (!ids.length) return 0
  const response = await batchReviewCards({ card_ids: ids, action, ...extra })
  const affected = response.data?.affected ?? ids.length
  clearSelection()
  await loadCards()
  emit('updated')
  return affected
}

function confirmBatchDelete() {
  const count = selectedIds.value.length
  if (!count) return
  confirmDelete({
    title: t('review.v3.cardsPage.batchDelete'),
    body: t('review.v3.cardsPage.batchDeleteConfirm', { count }),
    onConfirm: async () => {
      const affected = await runBatch('delete')
      MessagePlugin.success(t('review.v3.cardsPage.batchDone', { count: affected }))
    },
  })
}

function mergeSelected() {
  const ids = [...selectedIds.value]
  if (ids.length < 2) {
    MessagePlugin.warning(t('review.mergeNeedTwo'))
    return
  }
  confirmDelete({
    title: t('review.mergeCards'),
    body: t('review.mergeConfirm', { count: ids.length - 1 }),
    confirmText: t('review.mergeCards'),
    onConfirm: async () => {
      await mergeReviewCards({ target_card_id: ids[0], source_card_ids: ids.slice(1) })
      MessagePlugin.success(t('review.merged'))
      clearSelection()
      await loadCards()
      emit('updated')
    },
  })
}

function confirmBatchReset() {
  const count = selectedIds.value.length
  if (!count) return
  confirmDelete({
    title: t('review.v3.cardsPage.batchReset'),
    body: t('review.v3.cardsPage.batchResetConfirm', { count }),
    confirmText: t('review.v3.cardsPage.batchReset'),
    onConfirm: async () => {
      const affected = await runBatch('reset')
      MessagePlugin.success(t('review.v3.cardsPage.batchDone', { count: affected }))
    },
  })
}

function openMove() {
  moveGroupId.value = filters.groupId || ''
  moveVisible.value = true
}

async function saveMove() {
  if (!moveGroupId.value) return
  moveSaving.value = true
  try {
    const affected = await runBatch('move', { group_id: moveGroupId.value })
    moveVisible.value = false
    MessagePlugin.success(t('review.v3.cardsPage.batchDone', { count: affected }))
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.actionFailed'))
  } finally {
    moveSaving.value = false
  }
}

function openTags() {
  batchTags.value = []
  tagsVisible.value = true
}

async function saveTags() {
  const tags = batchTags.value.map(tag => tag.trim()).filter(Boolean)
  if (!tags.length) return
  tagsSaving.value = true
  try {
    const affected = await runBatch('add_tags', { tags })
    tagsVisible.value = false
    MessagePlugin.success(t('review.v3.cardsPage.batchDone', { count: affected }))
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.actionFailed'))
  } finally {
    tagsSaving.value = false
  }
}

async function openPerformance(card: ReviewCard) {
  performanceCardId.value = card.id
  performanceVisible.value = true
  performanceLoading.value = true
  performance.value = null
  try {
    const response = await getReviewCardPerformance(card.id)
    performance.value = response.data || null
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.loadFailed'))
    performanceVisible.value = false
  } finally {
    performanceLoading.value = false
  }
}

async function reloadPerformance() {
  if (!performanceCardId.value) return
  try {
    const response = await getReviewCardPerformance(performanceCardId.value)
    performance.value = response.data || null
  } catch {
    performance.value = null
  }
}

function openEdit(card: ReviewCard) {
  editForm.id = card.id
  editForm.question = card.question
  editForm.answer = card.answer
  editForm.explanation = card.explanation || ''
  editForm.tags = cardTags(card)
  editVisible.value = true
}

async function saveEdit() {
  if (!editForm.question.trim() || !editForm.answer.trim()) {
    MessagePlugin.warning(t('review.v3.cardsPage.questionRequired'))
    return
  }
  editSaving.value = true
  try {
    await updateReviewCard(editForm.id, {
      question: editForm.question.trim(),
      answer: editForm.answer.trim(),
      explanation: editForm.explanation.trim(),
      tags: editForm.tags,
    })
    editVisible.value = false
    MessagePlugin.success(t('review.v3.cardsPage.updated'))
    await loadCards()
    await reloadPerformance()
    emit('updated')
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.saveFailed'))
  } finally {
    editSaving.value = false
  }
}

async function toggleStatus(card: ReviewCard) {
  const next = card.status === 'suspended' ? 'active' : 'suspended'
  try {
    await setReviewCardStatus(card.id, next)
    MessagePlugin.success(next === 'active' ? t('review.restored') : t('review.cardUpdated'))
    await loadCards()
    await reloadPerformance()
    emit('updated')
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.actionFailed'))
  }
}

function removeCard(card: ReviewCard) {
  confirmDelete({
    title: t('review.deleteCard'),
    body: t('review.deleteCardConfirm'),
    onConfirm: async () => {
      await deleteReviewCard(card.id)
      MessagePlugin.success(t('review.cardDeleted'))
      performanceVisible.value = false
      await loadCards()
      emit('updated')
    },
  })
}

async function restoreCard(card: ReviewCard) {
  try {
    await restoreReviewCard(card.id)
    MessagePlugin.success(t('review.restored'))
    await loadCards()
    emit('updated')
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.actionFailed'))
  }
}

async function openRebind(card: ReviewCard) {
  try {
    const response = await getReviewCard(card.id)
    const source = response.data?.sources?.find(item => item.source_type === 'knowledge_chunk')
    if (!source) {
      MessagePlugin.warning(t('review.noKnowledgeSource'))
      return
    }
    rebindCardId.value = card.id
    rebindSourceId.value = source.id
    rebindForm.knowledgeId = source.knowledge_id
    rebindForm.chunkId = source.chunk_id
    rebindForm.excerpt = source.excerpt
    rebindVisible.value = true
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.actionFailed'))
  }
}

async function saveRebind() {
  if (!rebindCardId.value || !rebindSourceId.value || !rebindForm.knowledgeId.trim()) {
    MessagePlugin.warning(t('review.rebindRequired'))
    return
  }
  rebindSaving.value = true
  try {
    await rebindReviewSource(rebindCardId.value, rebindSourceId.value, {
      knowledge_id: rebindForm.knowledgeId.trim(),
      chunk_id: rebindForm.chunkId.trim(),
      excerpt: rebindForm.excerpt.trim(),
    })
    MessagePlugin.success(t('review.rebound'))
    rebindVisible.value = false
    await loadCards()
    await reloadPerformance()
    emit('updated')
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.actionFailed'))
  } finally {
    rebindSaving.value = false
  }
}

function resetCard(card: ReviewCard) {
  confirmDelete({
    title: t('review.v3.cardsPage.reset'),
    body: t('review.v3.cardsPage.resetConfirm'),
    confirmText: t('review.v3.cardsPage.reset'),
    onConfirm: async () => {
      await resetReviewCardSchedule(card.id)
      MessagePlugin.success(t('review.v3.cardsPage.resetDone'))
      await loadCards()
      await reloadPerformance()
      emit('updated')
    },
  })
}

function reviewCard(card: ReviewCard) {
  if (card.status !== 'active') return
  performanceVisible.value = false
  emit('review-card', card)
}

function rowMenu(card: ReviewCard) {
  const options: Array<{ content: string; value: string }> = [
    { content: t('review.v3.cardsPage.open'), value: 'open' },
    { content: t('review.v3.cardsPage.edit'), value: 'edit' },
  ]
  if (card.status === 'active') options.push({ content: t('review.suspend'), value: 'suspend' })
  if (card.status === 'suspended') options.push({ content: t('review.resume'), value: 'resume' })
  if (card.status === 'orphaned') options.push({ content: t('review.rebindSource'), value: 'rebind' })
  if (card.status === 'deleted') options.push({ content: t('review.restoreCard'), value: 'restore' })
  options.push({ content: t('review.deleteCard'), value: 'delete' })
  return options
}

function onRowMenu(item: { value: string }, card: ReviewCard) {
  switch (item.value) {
    case 'open':
      void openPerformance(card)
      break
    case 'edit':
      openEdit(card)
      break
    case 'suspend':
    case 'resume':
      void toggleStatus(card)
      break
    case 'restore':
      void restoreCard(card)
      break
    case 'rebind':
      void openRebind(card)
      break
    case 'delete':
      removeCard(card)
      break
  }
}

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

async function loadDocuments(kbId: string) {
  docOptions.value = []
  createForm.knowledgeId = ''
  if (!kbId) return
  docsLoading.value = true
  try {
    const response: any = await listKnowledgeFiles(kbId, { page: 1, page_size: 100, folder_recursive: true })
    const list = response.data || response?.data?.data || []
    docOptions.value = (Array.isArray(list) ? list : []).map((doc: any) => ({
      label: doc.name || doc.title || doc.file_name || doc.id,
      value: doc.id,
    }))
  } catch {
    docOptions.value = []
  } finally {
    docsLoading.value = false
  }
}

function openCreate() {
  createForm.groupId = filters.groupId || ''
  createForm.question = ''
  createForm.answer = ''
  createForm.explanation = ''
  createForm.tags = []
  createForm.knowledgeBaseId = ''
  createForm.knowledgeId = ''
  createForm.excerpt = ''
  docOptions.value = []
  createVisible.value = true
  void loadKnowledgeBases()
}

async function saveCreate() {
  if (!createForm.question.trim() || !createForm.answer.trim()) {
    MessagePlugin.warning(t('review.v3.cardsPage.questionRequired'))
    return
  }
  if (!createForm.knowledgeId) {
    MessagePlugin.warning(t('review.v3.cardsPage.sourceRequired'))
    return
  }
  const sources: ReviewCardSourceInput[] = [{
    source_type: 'knowledge_chunk',
    knowledge_id: createForm.knowledgeId,
    excerpt: createForm.excerpt.trim(),
    is_primary: true,
  }]
  createSaving.value = true
  try {
    await createReviewCard({
      group_id: createForm.groupId || undefined,
      question: createForm.question.trim(),
      answer: createForm.answer.trim(),
      explanation: createForm.explanation.trim(),
      tags: createForm.tags,
      sources,
    })
    createVisible.value = false
    MessagePlugin.success(t('review.v3.cardsPage.created'))
    await loadCards()
    emit('updated')
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('review.saveFailed'))
  } finally {
    createSaving.value = false
  }
}

onMounted(() => {
  void loadCards()
})
</script>

<template>
  <div class="cards-page" data-testid="review-v3-cards">
    <div class="cards-panel">
      <div class="cards-panel__head">
        <div class="cards-panel__title">
          <img class="rv-icon rv-icon--lg" :src="checklistIcon" alt="" />
          <div>
            <h3>{{ t('review.cards') }}</h3>
            <p>{{ t('review.v3.cardsPage.subtitle') }}</p>
          </div>
        </div>
        <div class="cards-panel__head-actions">
          <span class="cards-toolbar__count">{{ t('review.cardCount', { count: cards.length }) }}</span>
          <t-button theme="primary" data-testid="review-v3-card-create" @click="openCreate">
            <template #icon><img class="inline-hand-icon" :src="plusIcon" alt="" /></template>
            {{ t('review.v3.cardsPage.newCard') }}
          </t-button>
        </div>
      </div>

      <div class="cards-toolbar">
        <t-input
          v-model="filters.keyword"
          class="cards-toolbar__search"
          data-testid="review-v3-cards-search"
          :placeholder="t('review.searchPlaceholder')"
          clearable
          @enter="loadCards()"
          @clear="loadCards()"
        >
          <template #prefix-icon><img class="inline-hand-icon" :src="searchIcon" alt="" /></template>
        </t-input>
        <t-select v-model="filters.status" class="cards-toolbar__select" data-testid="review-v3-cards-status" :options="statusOptions" />
        <t-select v-model="filters.groupId" class="cards-toolbar__select" data-testid="review-v3-cards-group" :options="groupOptions" />
        <t-input
          v-model="filters.tag"
          class="cards-toolbar__tag"
          data-testid="review-v3-cards-tag"
          :placeholder="t('review.v3.cardsPage.tagFilter')"
          clearable
          @enter="loadCards()"
          @clear="loadCards()"
        />
        <t-checkbox v-model="filters.dueOnly" data-testid="review-v3-cards-due">{{ t('review.v3.cardsPage.dueOnly') }}</t-checkbox>
      </div>
    </div>

    <div v-if="selectedIds.length" class="batch-bar" data-testid="review-v3-card-batch">
      <span>{{ t('review.v3.cardsPage.selected', { count: selectedIds.length }) }}</span>
      <div class="batch-bar__actions">
        <t-button size="small" variant="outline" data-testid="review-v3-cards-batch-merge" :disabled="selectedIds.length < 2" @click="mergeSelected">{{ t('review.mergeCards') }}</t-button>
        <t-button size="small" variant="outline" data-testid="review-v3-cards-batch-move" @click="openMove">{{ t('review.v3.cardsPage.batchMove') }}</t-button>
        <t-button size="small" variant="outline" data-testid="review-v3-cards-batch-tags" @click="openTags">{{ t('review.v3.cardsPage.batchTags') }}</t-button>
        <t-button size="small" variant="outline" data-testid="review-v3-cards-batch-reset" @click="confirmBatchReset">{{ t('review.v3.cardsPage.batchReset') }}</t-button>
        <t-button size="small" theme="danger" variant="outline" data-testid="review-v3-cards-batch-delete" @click="confirmBatchDelete">{{ t('review.v3.cardsPage.batchDelete') }}</t-button>
        <t-button size="small" variant="text" @click="clearSelection">{{ t('review.v3.cardsPage.clearSelection') }}</t-button>
      </div>
    </div>

    <div v-if="error" class="cards-state">
      <t-alert theme="error" :message="error" />
      <t-button size="small" variant="outline" @click="loadCards()">{{ t('common.retry') }}</t-button>
    </div>

    <t-loading :loading="loading" size="small" class="cards-loading">
      <div v-if="cards.length" class="card-list" data-testid="review-v3-card-list">
        <div class="card-list__head">
          <span />
          <span>{{ t('review.v3.cardsPage.question') }}</span>
          <span>{{ t('review.v3.cardsPage.group') }}</span>
          <span>{{ t('review.v3.cardsPage.status') }}</span>
          <span>{{ t('review.v3.cardsPage.reviewInfo') }}</span>
          <span />
        </div>
        <div
          v-for="card in cards"
          :key="card.id"
          class="card-row"
          :class="{ 'is-selected': selectedIds.includes(card.id), 'is-deleted': card.status === 'deleted' }"
          :data-card-id="card.id"
          data-testid="review-v3-card-row"
          @click="openPerformance(card)"
        >
          <span class="card-row__check" @click.stop>
            <t-checkbox
              :checked="selectedIds.includes(card.id)"
              :disabled="card.status === 'deleted'"
              @change="(value: boolean) => toggleSelection(card.id, Boolean(value))"
            />
          </span>
          <div class="card-row__main">
            <div class="card-row__title">
              <span class="card-row__question">{{ card.question }}</span>
            </div>
            <div class="card-row__meta">
              <span v-if="cardTags(card).length" class="card-row__tags">
                <t-tag v-for="tag in cardTags(card).slice(0, 2)" :key="tag" size="small" variant="light">{{ tag }}</t-tag>
                <span v-if="cardTags(card).length > 2">+{{ cardTags(card).length - 2 }}</span>
              </span>
              <span v-else class="card-row__muted">{{ t('review.v3.cardsPage.noTags') }}</span>
            </div>
          </div>
          <span class="card-row__group" :title="groupName(card)">{{ groupName(card) }}</span>
          <span class="card-row__status">
            <ReviewLevelTag v-if="showsLevel(card)" :level="levelOf(card)" />
            <t-tag v-if="card.status === 'suspended'" size="small" variant="outline">{{ t('review.status.suspended') }}</t-tag>
            <t-tag v-else-if="card.status === 'orphaned'" size="small" theme="danger" variant="outline">{{ t('review.status.orphaned') }}</t-tag>
            <t-tag v-else-if="card.status === 'deleted'" size="small" variant="outline">{{ t('review.status.deleted') }}</t-tag>
            <span v-if="!showsLevel(card) && card.status === 'active'" class="card-row__healthy">
              {{ t('review.v3.level.healthy') }}
            </span>
          </span>
          <span class="card-row__review">
            <span v-if="card.review_count">{{ t('review.stats.reviewCount', { count: card.review_count }) }}</span>
            <span v-else class="card-row__muted">{{ t('review.v3.cardsPage.neverReviewed') }}</span>
            <span class="card-row__hold">{{ t('review.v3.cardsPage.holdDays', { count: holdDays(card.stability) }) }}</span>
          </span>
          <div class="card-row__actions" @click.stop>
            <t-dropdown :options="rowMenu(card)" placement="bottom-right" @click="(item: { value: string }) => onRowMenu(item, card)">
              <t-button variant="text" shape="square" size="small">
                <img class="inline-hand-icon" :src="moreIcon" alt="" />
              </t-button>
            </t-dropdown>
          </div>
        </div>
      </div>
      <div v-else-if="!loading" class="cards-state" data-testid="review-v3-cards-empty">
        <img class="rv-icon rv-icon--lg" :src="bookIcon" alt="" />
        <p>{{ t('review.v3.cardsPage.emptyTitle') }}</p>
        <p class="cards-state__hint">{{ t('review.v3.cardsPage.emptyHint') }}</p>
        <div class="cards-state__actions">
          <t-button v-if="filters.keyword || filters.tag || filters.status !== 'all' || filters.dueOnly" variant="outline" size="small" @click="resetFilters">
            {{ t('review.v3.cardsPage.clearFilters') }}
          </t-button>
          <t-button theme="primary" size="small" @click="openCreate">{{ t('review.v3.cardsPage.newCard') }}</t-button>
        </div>
      </div>
      <div v-if="hasMore" class="cards-more">
        <t-button size="small" variant="outline" :loading="loadingMore" @click="loadCards({ append: true })">
          {{ t('review.v3.cardsPage.loadMore') }}
        </t-button>
      </div>
    </t-loading>

    <t-drawer
      v-model:visible="performanceVisible"
      size="560px"
      :footer="false"
      attach="body"
      data-testid="review-v3-card-drawer"
    >
      <template #header>
        <div class="drawer-head">
          <span>{{ t('review.v3.cardsPage.performance') }}</span>
          <t-button
            variant="text"
            shape="square"
            size="small"
            data-testid="review-v3-card-drawer-close"
            @click="performanceVisible = false"
          >
            <img class="inline-hand-icon" :src="closeIcon" alt="" />
          </t-button>
        </div>
      </template>
      <t-loading :loading="performanceLoading" size="small">
        <template v-if="performance">
          <div class="perf-card">
            <h3>{{ performance.card.question }}</h3>
            <p v-if="performance.warning !== 'none'" class="perf-warning">
              {{ performance.warning_reason || t('review.warningBadge') }}
            </p>
            <div class="perf-counts">
              <div><span>{{ t('review.v3.rating.easy') }}</span><strong>{{ performance.counts.easy }}</strong></div>
              <div><span>{{ t('review.v3.rating.remembered') }}</span><strong>{{ performance.counts.remembered }}</strong></div>
              <div><span>{{ t('review.v3.rating.fuzzy') }}</span><strong>{{ performance.counts.fuzzy }}</strong></div>
              <div><span>{{ t('review.v3.rating.forgot') }}</span><strong>{{ performance.counts.forgot }}</strong></div>
            </div>
            <div class="perf-meta">
              <span>{{ t('review.v3.cardsPage.recentCounts') }}：{{ performance.recent_counts.easy }}/{{ performance.recent_counts.remembered }}/{{ performance.recent_counts.fuzzy }}/{{ performance.recent_counts.forgot }}</span>
              <span>{{ t('review.v3.cardsPage.forgetRate') }}：{{ Math.round(performance.recent_forget_rate * 100) }}%</span>
              <span>{{ t('review.forgetStreak') }}：{{ performance.forget_streak }}</span>
              <span>{{ t('review.stability') }}：{{ t('review.v3.cardsPage.holdDays', { count: holdDays(performance.card.stability) }) }}</span>
            </div>
            <div class="perf-tags">
              <t-tag v-for="tag in cardTags(performance.card)" :key="tag" size="small" variant="light">{{ tag }}</t-tag>
            </div>
          </div>

          <div class="perf-history">
            <h4>{{ t('review.history') }}</h4>
            <div v-if="performance.logs.length" class="perf-log-list">
              <div v-for="log in performance.logs.slice(0, 12)" :key="log.id" class="perf-log">
                <span :class="`perf-log__rating perf-log__rating--${log.rating}`">{{ t(`review.v3.rating.${log.rating}`) }}</span>
                <span class="perf-log__time">{{ new Date(log.reviewed_at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) }}</span>
              </div>
            </div>
            <p v-else class="cards-state__hint">{{ t('review.noHistory') }}</p>
          </div>

          <div class="perf-actions">
            <t-button
              theme="primary"
              :disabled="performance.card.status !== 'active'"
              data-testid="review-v3-card-review-now"
              @click="reviewCard(performance.card)"
            >
              {{ t('review.v3.cardsPage.reviewNow') }}
            </t-button>
            <t-button variant="outline" data-testid="review-v3-card-edit" @click="openEdit(performance.card)">
              {{ t('review.v3.cardsPage.edit') }}
            </t-button>
            <t-button variant="outline" data-testid="review-v3-card-reset" @click="resetCard(performance.card)">
              {{ t('review.v3.cardsPage.reset') }}
            </t-button>
          </div>
        </template>
      </t-loading>
    </t-drawer>

    <t-dialog
      v-model:visible="editVisible"
      :header="t('review.v3.cardsPage.edit')"
      :confirm-btn="{ content: t('review.save'), loading: editSaving }"
      :cancel-btn="{ content: t('review.cancel') }"
      width="640px"
      @confirm="saveEdit"
    >
      <div class="form-stack">
        <label class="form-field">
          <span>{{ t('review.v3.cardsPage.createQuestion') }}</span>
          <t-textarea v-model="editForm.question" data-testid="review-v3-card-edit-question" :autosize="{ minRows: 2, maxRows: 4 }" />
        </label>
        <label class="form-field">
          <span>{{ t('review.v3.cardsPage.createAnswer') }}</span>
          <t-textarea v-model="editForm.answer" data-testid="review-v3-card-edit-answer" :autosize="{ minRows: 2, maxRows: 4 }" />
        </label>
        <label class="form-field">
          <span>{{ t('review.v3.cardsPage.createExplanation') }}</span>
          <t-textarea v-model="editForm.explanation" data-testid="review-v3-card-edit-explanation" :autosize="{ minRows: 2, maxRows: 4 }" />
        </label>
        <label class="form-field">
          <span>{{ t('review.v3.cardsPage.tags') }}</span>
          <t-tag-input v-model="editForm.tags" data-testid="review-v3-card-edit-tags" :placeholder="t('review.v3.cardsPage.tagsPlaceholder')" />
        </label>
      </div>
    </t-dialog>

    <t-dialog
      v-model:visible="createVisible"
      :header="t('review.v3.cardsPage.createTitle')"
      :confirm-btn="{ content: t('review.create'), loading: createSaving }"
      :cancel-btn="{ content: t('review.cancel') }"
      width="680px"
      @confirm="saveCreate"
    >
      <div class="form-stack">
        <t-select v-model="createForm.groupId" data-testid="review-v3-card-create-group" :label="t('review.groupName')" :options="groupOptions.filter(item => item.value)" filterable />
        <label class="form-field">
          <span>{{ t('review.v3.cardsPage.createQuestion') }}</span>
          <t-textarea v-model="createForm.question" data-testid="review-v3-card-create-question" :autosize="{ minRows: 2, maxRows: 4 }" />
        </label>
        <label class="form-field">
          <span>{{ t('review.v3.cardsPage.createAnswer') }}</span>
          <t-textarea v-model="createForm.answer" data-testid="review-v3-card-create-answer" :autosize="{ minRows: 2, maxRows: 4 }" />
        </label>
        <label class="form-field">
          <span>{{ t('review.v3.cardsPage.createExplanation') }}</span>
          <t-textarea v-model="createForm.explanation" data-testid="review-v3-card-create-explanation" :autosize="{ minRows: 2, maxRows: 4 }" />
        </label>
        <label class="form-field">
          <span>{{ t('review.v3.cardsPage.tags') }}</span>
          <t-tag-input v-model="createForm.tags" data-testid="review-v3-card-create-tags" :placeholder="t('review.v3.cardsPage.tagsPlaceholder')" />
        </label>
        <div class="form-grid">
          <t-select
            v-model="createForm.knowledgeBaseId"
            data-testid="review-v3-card-create-kb"
            :label="t('review.knowledgeBase')"
            :options="kbOptions"
            :placeholder="t('review.selectKnowledgeBase')"
            filterable
            @change="(value: string) => loadDocuments(value)"
          />
          <t-select
            v-model="createForm.knowledgeId"
            data-testid="review-v3-card-create-doc"
            :label="t('review.v3.cardsPage.selectKnowledge')"
            :options="docOptions"
            :loading="docsLoading"
            :placeholder="t('review.v3.cardsPage.selectKnowledge')"
            filterable
          />
        </div>
        <label class="form-field">
          <span>{{ t('review.v3.cardsPage.sourceExcerpt') }}</span>
          <t-textarea v-model="createForm.excerpt" data-testid="review-v3-card-create-excerpt" :autosize="{ minRows: 2, maxRows: 4 }" />
        </label>
      </div>
    </t-dialog>

    <t-dialog
      v-model:visible="moveVisible"
      :header="t('review.v3.cardsPage.moveTitle')"
      :confirm-btn="{ content: t('review.confirm'), loading: moveSaving }"
      :cancel-btn="{ content: t('review.cancel') }"
      width="420px"
      data-testid="review-v3-cards-move-dialog"
      @confirm="saveMove"
    >
      <t-select v-model="moveGroupId" :options="groupOptions.filter(item => item.value)" :placeholder="t('review.v3.cardsPage.movePlaceholder')" filterable />
    </t-dialog>

    <t-dialog
      v-model:visible="tagsVisible"
      :header="t('review.v3.cardsPage.tagsTitle')"
      :confirm-btn="{ content: t('review.save'), loading: tagsSaving }"
      :cancel-btn="{ content: t('review.cancel') }"
      width="460px"
      data-testid="review-v3-cards-tags-dialog"
      @confirm="saveTags"
    >
      <t-tag-input v-model="batchTags" data-testid="review-v3-cards-tags-input" :placeholder="t('review.v3.cardsPage.tagsPlaceholder')" />
    </t-dialog>

    <t-dialog
      v-model:visible="rebindVisible"
      :header="t('review.rebindSource')"
      :confirm-btn="{ content: t('review.save'), loading: rebindSaving }"
      :cancel-btn="{ content: t('review.cancel') }"
      width="520px"
      data-testid="review-v3-cards-rebind-dialog"
      @confirm="saveRebind"
    >
      <div class="form-stack">
        <t-input v-model="rebindForm.knowledgeId" data-testid="review-v3-cards-rebind-knowledge" :label="t('review.knowledgeId')" />
        <t-input v-model="rebindForm.chunkId" data-testid="review-v3-cards-rebind-chunk" :label="t('review.chunkId')" />
        <t-textarea v-model="rebindForm.excerpt" data-testid="review-v3-cards-rebind-excerpt" :label="t('review.excerpt')" :autosize="{ minRows: 3, maxRows: 6 }" />
      </div>
    </t-dialog>
  </div>
</template>

<style lang="less" scoped>
.inline-hand-icon {
  width: 16px;
  height: 16px;
  object-fit: contain;
}

.cards-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.cards-panel {
  padding: 16px 20px 18px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-medium);

  &__head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  &__title {
    display: flex;
    align-items: center;
    gap: 10px;

    h3 {
      margin: 0;
      font-size: 16px;
      line-height: 24px;
      font-weight: 600;
      color: var(--td-text-color-primary);
    }

    p {
      margin: 1px 0 0;
      font-size: 12px;
      line-height: 18px;
      color: var(--td-text-color-secondary);
    }
  }

  &__head-actions {
    display: flex;
    align-items: center;
    gap: 10px;
  }
}

.cards-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--td-component-stroke);

  &__search { width: 260px; }
  &__select { width: 148px; }
  &__tag { width: 160px; }
  &__count {
    font-size: 12px;
    color: var(--td-text-color-secondary);
    font-variant-numeric: tabular-nums;
  }
}

.batch-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 9px 14px;
  font-size: 13px;
  background: var(--td-brand-color-light);
  border: 1px solid var(--td-brand-color-focus);
  border-radius: var(--td-radius-medium);

  &__actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
}

.cards-loading {
  display: block;
}

.card-list {
  overflow: hidden;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-medium);

  &__head {
    display: grid;
    grid-template-columns: 28px minmax(0, 1fr) 140px 110px 170px 40px;
    gap: 12px;
    align-items: center;
    min-height: 40px;
    padding: 0 14px;
    font-size: 12px;
    color: var(--td-text-color-secondary);
    background: var(--td-bg-color-secondarycontainer);
    border-bottom: 1px solid var(--td-component-stroke);
  }
}

.card-row {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr) 140px 110px 170px 40px;
  gap: 12px;
  align-items: center;
  min-height: 56px;
  padding: 8px 14px;
  border-bottom: 1px solid var(--td-component-stroke);
  cursor: pointer;
  transition: background-color 0.15s ease;

  &:last-child { border-bottom: 0; }
  &:hover, &:focus-within { background: var(--td-bg-color-container-hover); }
  &.is-selected { background: var(--td-brand-color-light); }
  &.is-deleted { opacity: 0.7; }

  &__check {
    opacity: 0;
    transition: opacity 0.15s ease;
  }
  &:hover &__check,
  &:focus-within &__check,
  &.is-selected &__check {
    opacity: 1;
  }

  &__main {
    min-width: 0;
  }

  &__title {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }

  &__question {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 13px;
    font-weight: 500;
    color: var(--td-text-color-primary);
  }

  &__meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-top: 3px;
    font-size: 12px;
    color: var(--td-text-color-secondary);
  }

  &__tags {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }

  &__group {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 13px;
    color: var(--td-text-color-secondary);
  }

  &__status {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
  }

  &__healthy {
    font-size: 12px;
    color: var(--td-success-color);
  }

  &__review {
    display: flex;
    flex-direction: column;
    gap: 1px;
    font-size: 12px;
    color: var(--td-text-color-secondary);
    font-variant-numeric: tabular-nums;
  }

  &__hold { color: var(--td-text-color-placeholder); }

  &__muted { color: var(--td-text-color-placeholder); }

  &__actions { justify-self: end; }
}

.cards-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 220px;
  padding: 24px;
  font-size: 14px;
  color: var(--td-text-color-secondary);
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-medium);

  &__hint {
    margin: 0;
    font-size: 12px;
    color: var(--td-text-color-placeholder);
  }

  &__actions {
    display: flex;
    gap: 8px;
    margin-top: 8px;
  }
}

.cards-more {
  display: flex;
  justify-content: center;
  padding: 12px 0;
}

@media (max-width: 1100px) {
  .card-list__head,
  .card-row {
    grid-template-columns: 28px minmax(0, 1fr) 110px 150px 40px;
  }

  .card-list__head span:nth-child(3),
  .card-row__group {
    display: none;
  }
}

@media (max-width: 760px) {
  .cards-panel { padding: 14px 14px 16px; }

  .cards-toolbar {
    &__search { width: 100%; }
    &__select, &__tag { flex: 1 1 140px; width: auto; }
  }

  .card-list__head { display: none; }

  .card-row {
    grid-template-columns: 24px minmax(0, 1fr) 32px;
    gap: 8px;
    padding: 10px 12px;
  }

  .card-row__group,
  .card-row__status,
  .card-row__review {
    display: none;
  }
}

.perf-card {
  padding-bottom: 14px;
  border-bottom: 1px solid var(--td-component-stroke);

  h3 {
    margin: 0 0 8px;
    font-size: 15px;
    line-height: 23px;
    color: var(--td-text-color-primary);
  }
}

.drawer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
}

.perf-warning {
  margin: 0 0 10px;
  padding: 8px 10px;
  font-size: 12px;
  color: var(--td-error-color);
  background: var(--td-error-color-light);
  border-radius: var(--td-radius-default);
}

.perf-counts {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;

  div {
    padding: 8px 10px;
    background: var(--td-bg-color-secondarycontainer);
    border-radius: var(--td-radius-default);
  }

  span {
    display: block;
    font-size: 12px;
    color: var(--td-text-color-secondary);
  }

  strong {
    display: block;
    margin-top: 2px;
    font-size: 18px;
    font-variant-numeric: tabular-nums;
    color: var(--td-text-color-primary);
  }
}

.perf-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  margin-top: 10px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.perf-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 10px;
}

.perf-history {
  padding-top: 14px;

  h4 {
    margin: 0 0 8px;
    font-size: 13px;
    color: var(--td-text-color-primary);
  }
}

.perf-log-list {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-default);
}

.perf-log {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 7px 10px;
  font-size: 12px;
  border-bottom: 1px solid var(--td-component-stroke);

  &:last-child { border-bottom: 0; }

  &__rating--easy { color: var(--td-brand-color); }
  &__rating--remembered { color: var(--td-success-color); }
  &__rating--fuzzy { color: var(--td-warning-color); }
  &__rating--forgot { color: var(--td-error-color); }

  &__time { color: var(--td-text-color-placeholder); }
}

.perf-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 16px;
}

.form-stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;

  > span {
    font-size: 13px;
    color: var(--td-text-color-primary);
  }
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

@media (max-width: 760px) {
  .cards-toolbar {
    &__search { width: 100%; }
    &__select, &__tag { flex: 1 1 140px; width: auto; }
  }

  .form-grid { grid-template-columns: 1fr; }
}
</style>
