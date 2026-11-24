/**
 * Cards tab for review V3: compact 56px rows, server-side filters, batch
 * actions, and inline full-width performance/create/edit detail pages. Reuses the existing
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
import ReviewIcon from './ReviewIcon.vue'
import ReviewCardAIDialog from './ReviewCardAIDialog.vue'
import type { ReviewFlatGroup } from './types'
import { holdDays, normalizeVulnerability } from './vulnerability'
import { useConfirmDelete } from '@/components/settings/useConfirmDelete'

const props = defineProps<{
  flatGroups: ReviewFlatGroup[]
  initialGroupId: string
}>()

const emit = defineEmits<{
  (e: 'review-card', card: ReviewCard): void
  (e: 'updated'): void
  (e: 'detail', open: boolean): void
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
const aiDialogVisible = ref(false)
const aiDialogMode = ref<'split' | 'angles'>('split')

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
const createTagDraft = ref('')
const editTagDraft = ref('')
const sourceEditing = ref(false)

const createSourceLabel = computed(() => {
  if (!createForm.knowledgeId) return t('review.v3.cardsPage.sourceRequired')
  return docOptions.value.find(item => item.value === createForm.knowledgeId)?.label || createForm.knowledgeId
})

function addTag(target: 'create' | 'edit') {
  const form = target === 'create' ? createForm : editForm
  const draft = target === 'create' ? createTagDraft : editTagDraft
  const value = draft.value.trim()
  if (!value) return
  if (!form.tags.includes(value)) form.tags.push(value)
  draft.value = ''
}

function removeTag(target: 'create' | 'edit', tag: string) {
  const form = target === 'create' ? createForm : editForm
  form.tags = form.tags.filter(item => item !== tag)
}

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

const groupFilterOpen = ref(false)
const expandedGroupIds = ref<Set<string>>(new Set())

const groupParentByID = computed(() => {
  const map = new Map<string, string>()
  props.flatGroups.forEach(item => map.set(item.group.id, item.group.parent_id || ''))
  return map
})

const groupParentIDs = computed(() => {
  const parents = new Set<string>()
  props.flatGroups.forEach(item => {
    if (item.group.parent_id) parents.add(item.group.parent_id)
  })
  return parents
})

const selectedGroupLabel = computed(() =>
  groupNameByID.value.get(filters.groupId) || t('review.v3.allGroups'),
)

const visibleGroupFilters = computed(() =>
  props.flatGroups.filter(item => {
    let cursor = item.group.parent_id || ''
    while (cursor) {
      if (!expandedGroupIds.value.has(cursor)) return false
      cursor = groupParentByID.value.get(cursor) || ''
    }
    return true
  }),
)

function toggleGroupFilterExpand(groupId: string) {
  const next = new Set(expandedGroupIds.value)
  if (next.has(groupId)) next.delete(groupId)
  else next.add(groupId)
  expandedGroupIds.value = next
}

function selectGroupFilter(groupId: string) {
  filters.groupId = groupId
  groupFilterOpen.value = false
}

function ensureGroupFilterVisible(groupId: string) {
  if (!groupId) return
  const next = new Set(expandedGroupIds.value)
  let cursor = groupParentByID.value.get(groupId) || ''
  while (cursor) {
    next.add(cursor)
    cursor = groupParentByID.value.get(cursor) || ''
  }
  expandedGroupIds.value = next
}

watch(() => filters.groupId, ensureGroupFilterVisible, { immediate: true })

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

function dueLabel(card: ReviewCard) {
  if (!card.due_at || card.status !== 'active') return '—'
  const due = new Date(card.due_at)
  if (Number.isNaN(due.getTime())) return '—'
  const today = new Date()
  const sameDay = due.toDateString() === today.toDateString()
  if (sameDay) return t('review.v3.cardsPage.dueToday')
  return due.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
}

function isDueToday(card: ReviewCard) {
  if (!card.due_at || card.status !== 'active') return false
  return new Date(card.due_at).toDateString() === new Date().toDateString()
}

function errorRate(card: ReviewCard) {
  if (!card.review_count) return 0
  return Math.min(100, Math.round((card.lapses / card.review_count) * 100))
}

function ratingTone(rating: string) {
  if (rating === 'forgot') return 'red'
  if (rating === 'fuzzy') return 'orange'
  if (rating === 'remembered') return 'green'
  return 'blue'
}

function distributionPercent(count: number, total: number) {
  if (!total) return 0
  return Math.round((count / total) * 100)
}

function openPerformanceSplit() {
  if (!performance.value?.card) return
  aiDialogMode.value = 'split'
  aiDialogVisible.value = true
}

function onPerformanceAIDialogApplied() {
  void reloadPerformance()
}

const performanceWeekAdded = computed(() => {
  const logs = performance.value?.logs ?? []
  const since = Date.now() - 7 * 24 * 3600 * 1000
  return logs.filter(log => new Date(log.reviewed_at).getTime() >= since).length
})

const performanceRecentRetention = computed(() => {
  const logs = (performance.value?.logs ?? []).slice(0, 5)
  if (!logs.length) return null
  const good = logs.filter(log => log.rating === 'remembered' || log.rating === 'easy').length
  return Math.round((good / logs.length) * 100)
})

const performanceTrend = computed(() => {
  const logs = [...(performance.value?.logs ?? [])].reverse().slice(-14)
  if (logs.length < 2) return ''
  let good = 0
  const points = logs.map((log, index) => {
    if (log.rating === 'remembered' || log.rating === 'easy') good += 1
    const x = (index / (logs.length - 1)) * 320
    const y = 104 - (good / (index + 1)) * 84
    return `${x.toFixed(1)},${y.toFixed(1)}`
  })
  return points.join(' ')
})

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
  performanceVisible.value = false
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
  createTagDraft.value = ''
  sourceEditing.value = false
  docOptions.value = []
  createVisible.value = true
  void loadKnowledgeBases()
}

async function saveCreate(closeAfter = true) {
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
    if (closeAfter) {
      createVisible.value = false
    } else {
      createForm.question = ''
      createForm.answer = ''
      createForm.explanation = ''
      createForm.tags = []
      createForm.excerpt = ''
      createTagDraft.value = ''
    }
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

watch(
  () => performanceVisible.value || createVisible.value || editVisible.value,
  (open) => emit('detail', open),
)
</script>

<template>
  <section v-if="performanceVisible" class="cards-subpage" data-testid="review-v3-card-performance">
    <header class="cards-subpage__head">
      <h1>{{ t('review.v3.cardsPage.performance') }}</h1>
      <button type="button" class="rv-btn rv-btn--ghost rv-btn--sm" @click="performanceVisible = false">
        <ReviewIcon name="arrow-left" :size="13" />
        {{ t('review.v3.cardsPage.back') }}
      </button>
    </header>
    <div class="cards-subpage__body performance-page">
      <t-loading :loading="performanceLoading" size="small">
        <template v-if="performance">
          <div class="rv-props">
            <div class="rv-prop">
              <span class="rv-prop__key">{{ t('review.v3.cardsPage.card') }}</span>
              <b>{{ performance.card.question }}</b>
              <span class="rv-tag rv-tag--gray">{{ t('review.v3.cardsPage.learning') }}</span>
            </div>
            <div class="rv-prop">
              <span class="rv-prop__key">{{ t('review.groupName') }}</span>
              <span>{{ groupName(performance.card) }}</span>
            </div>
            <div class="rv-prop">
              <span class="rv-prop__key">{{ t('review.v3.cardsPage.nextReview') }}</span>
              <span :class="{ 'is-due-today': isDueToday(performance.card) }">
                {{ dueLabel(performance.card) }}
              </span>
              <span class="rv-note">· {{ t('review.v3.cardsPage.holdDays', { count: holdDays(performance.card.stability) }) }}</span>
            </div>
          </div>

          <div class="perf-kpis">
            <div class="perf-kpi">
              <div class="perf-kpi__label">{{ t('review.v3.cardsPage.totalReviews') }}</div>
              <div class="perf-kpi__value">{{ performance.card.review_count }}</div>
              <div class="perf-kpi__hint">{{ t('review.v3.cardsPage.weekAdded', { count: performanceWeekAdded }) }}</div>
            </div>
            <div class="perf-kpi">
              <div class="perf-kpi__label">{{ t('review.v3.cardsPage.retention') }}</div>
              <div class="perf-kpi__value">{{ Math.round((1 - performance.recent_forget_rate) * 100) }}%</div>
              <div class="perf-kpi__hint" :class="{ 'is-down': (performanceRecentRetention ?? 100) < 80 }">
                {{ performanceRecentRetention === null ? '—' : t('review.v3.cardsPage.recentRetention', { count: performanceRecentRetention }) }}
              </div>
            </div>
            <div class="perf-kpi">
              <div class="perf-kpi__label">{{ t('review.v3.cardsPage.currentInterval') }}</div>
              <div class="perf-kpi__value">{{ t('review.v3.cardsPage.holdDays', { count: holdDays(performance.card.stability) }) }}</div>
              <div class="perf-kpi__hint is-down">{{ t('review.v3.cardsPage.intervalHint') }}</div>
            </div>
            <div class="perf-kpi">
              <div class="perf-kpi__label">{{ t('review.forgetStreak') }}</div>
              <div class="perf-kpi__value">{{ performance.forget_streak }}</div>
              <div class="perf-kpi__hint">{{ t('review.v3.cardsPage.streakHint') }}</div>
            </div>
          </div>

          <div class="perf-grid">
            <div class="rv-chart-card">
              <div class="rv-chart-card__title">{{ t('review.v3.cardsPage.retentionTrend') }}</div>
              <div class="rv-chart-card__sub">{{ t('review.v3.cardsPage.retentionTrendSub') }}</div>
              <svg v-if="performanceTrend" class="perf-trend" viewBox="0 0 320 110" preserveAspectRatio="none">
                <line x1="0" y1="20" x2="320" y2="20" stroke="#E0DBD1" stroke-dasharray="3 3" />
                <line x1="0" y1="62" x2="320" y2="62" stroke="#E0DBD1" stroke-dasharray="3 3" />
                <line x1="0" y1="104" x2="320" y2="104" stroke="#E0DBD1" />
                <text x="2" y="17" font-size="14" fill="#9C948A">100%</text>
                <text x="2" y="59" font-size="14" fill="#9C948A">50%</text>
                <polyline fill="none" stroke="#5C7E92" stroke-width="2" :points="performanceTrend" />
              </svg>
              <p v-else class="cards-state__hint">{{ t('review.noHistory') }}</p>
            </div>
            <div class="rv-chart-card">
              <div class="rv-chart-card__title">{{ t('review.v3.cardsPage.ratingHistory') }}</div>
              <div class="rv-chart-card__sub">{{ t('review.v3.cardsPage.recentRatings') }}</div>
              <div v-if="performance.logs.length" class="perf-history-list">
                <div v-for="log in performance.logs.slice(0, 8)" :key="log.id" class="perf-history-row">
                  <span class="rv-tag" :class="`rv-tag--${ratingTone(log.rating)}`">{{ t(`review.v3.rating.${log.rating}`) }}</span>
                  <span class="rv-note">{{ new Date(log.reviewed_at).toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) }}</span>
                  <span class="rv-note perf-history-row__interval">{{ t('review.v3.cardsPage.holdDays', { count: log.interval_days || 0 }) }}</span>
                </div>
              </div>
              <p v-else class="cards-state__hint">{{ t('review.noHistory') }}</p>
            </div>
          </div>

          <div class="rv-callout rv-callout--yellow perf-split">
            <ReviewIcon name="scissors" :size="20" :tilt="1" />
            <div>
              <strong>{{ t('review.v3.cardsPage.splitSuggestion') }}</strong>
              {{ t('review.v3.cardsPage.splitSuggestionHint') }}
            </div>
            <span class="rv-callout__act">
              <button type="button" class="rv-btn rv-btn--primary rv-btn--sm" @click="openPerformanceSplit">
                {{ t('review.v3.cardsPage.oneClickSplit') }}
              </button>
            </span>
          </div>

          <div class="perf-actions">
            <button
              type="button"
              class="rv-btn rv-btn--primary"
              :disabled="performance.card.status !== 'active'"
              data-testid="review-v3-card-review-now"
              @click="reviewCard(performance.card)"
            >
              {{ t('review.v3.cardsPage.reviewNow') }}
            </button>
            <button type="button" class="rv-btn rv-btn--ghost" data-testid="review-v3-card-edit" @click="openEdit(performance.card)">
              {{ t('review.v3.cardsPage.edit') }}
            </button>
            <button type="button" class="rv-btn rv-btn--ghost" data-testid="review-v3-card-reset" @click="resetCard(performance.card)">
              {{ t('review.v3.cardsPage.reset') }}
            </button>
          </div>
        </template>
      </t-loading>
    </div>

    <ReviewCardAIDialog
      v-model:visible="aiDialogVisible"
      :mode="aiDialogMode"
      :card="performance?.card || null"
      @applied="onPerformanceAIDialogApplied"
    />
  </section>

  <section v-else-if="editVisible" class="cards-subpage" data-testid="review-v3-card-edit-page">
    <header class="cards-subpage__head">
      <h1>{{ t('review.v3.cardsPage.edit') }}</h1>
      <button type="button" class="rv-btn rv-btn--ghost rv-btn--sm" @click="editVisible = false">
        <ReviewIcon name="arrow-left" :size="13" />
        {{ t('review.v3.cardsPage.back') }}
      </button>
    </header>
    <div class="cards-subpage__body card-form">
      <div class="form-stack">
        <label class="form-field">
          <span>{{ t('review.v3.cardsPage.createQuestion') }}<em>{{ t('review.v3.cardsPage.questionHint') }}</em></span>
          <t-textarea v-model="editForm.question" data-testid="review-v3-card-edit-question" :autosize="{ minRows: 3, maxRows: 5 }" />
        </label>
        <label class="form-field">
          <span>{{ t('review.v3.cardsPage.createAnswer') }}<em>{{ t('review.v3.cardsPage.answerHint') }}</em></span>
          <t-textarea v-model="editForm.answer" data-testid="review-v3-card-edit-answer" :autosize="{ minRows: 3, maxRows: 5 }" />
        </label>
        <label class="form-field">
          <span>{{ t('review.v3.cardsPage.createExplanation') }}<em>{{ t('review.v3.cardsPage.explanationHint') }}</em></span>
          <t-textarea v-model="editForm.explanation" data-testid="review-v3-card-edit-explanation" :autosize="{ minRows: 2, maxRows: 4 }" />
        </label>
        <div class="form-field">
          <span>{{ t('review.v3.cardsPage.tags') }}<em>{{ t('review.v3.cardsPage.tagsHint') }}</em></span>
          <div class="tag-editor" data-testid="review-v3-card-edit-tags">
            <span v-for="tag in editForm.tags" :key="tag" class="rv-tag rv-tag--blue">
              {{ tag }}<button type="button" @click="removeTag('edit', tag)">×</button>
            </span>
            <input
              v-model="editTagDraft"
              class="tag-editor__input"
              :placeholder="t('review.v3.cardsPage.tagsPlaceholder')"
              @keyup.enter="addTag('edit')"
            />
          </div>
        </div>
        <div class="form-field">
          <span>{{ t('review.v3.cardsPage.memoryParams') }}</span>
          <div class="memory-params">
            <span class="toggle-demo"><i />{{ t('review.v3.cardsPage.bidirectional') }}</span>
            <span class="radio-demo is-on"><i />FSRS</span>
            <span class="radio-demo"><i />SM-2</span>
            <em>{{ t('review.v3.cardsPage.memoryParamsHint') }}</em>
          </div>
        </div>
      </div>
    </div>
    <footer class="cards-subpage__foot">
      <button type="button" class="rv-btn rv-btn--primary" :disabled="editSaving" @click="saveEdit">
        {{ t('review.save') }}
      </button>
      <button type="button" class="rv-btn rv-btn--ghost" @click="editVisible = false">{{ t('review.cancel') }}</button>
    </footer>
  </section>

  <section v-else-if="createVisible" class="cards-subpage" data-testid="review-v3-card-create-page">
    <header class="cards-subpage__head">
      <h1>{{ t('review.v3.cardsPage.createTitle') }}</h1>
      <button type="button" class="rv-btn rv-btn--ghost rv-btn--sm" @click="createVisible = false">
        <ReviewIcon name="arrow-left" :size="13" />
        {{ t('review.v3.cardsPage.back') }}
      </button>
    </header>
    <div class="cards-subpage__body card-form">
      <div class="form-stack">
        <div class="form-field">
          <span>{{ t('review.v3.cardsPage.group') }}</span>
          <t-select
            v-model="createForm.groupId"
            data-testid="review-v3-card-create-group"
            :options="groupOptions.filter(item => item.value)"
            filterable
            class="card-form__select"
          />
        </div>
        <label class="form-field">
          <span>{{ t('review.v3.cardsPage.createQuestion') }}<em>{{ t('review.v3.cardsPage.questionHint') }}</em></span>
          <t-textarea v-model="createForm.question" data-testid="review-v3-card-create-question" :autosize="{ minRows: 3, maxRows: 5 }" />
        </label>
        <label class="form-field">
          <span>{{ t('review.v3.cardsPage.createAnswer') }}<em>{{ t('review.v3.cardsPage.answerHint') }}</em></span>
          <t-textarea v-model="createForm.answer" data-testid="review-v3-card-create-answer" :autosize="{ minRows: 3, maxRows: 5 }" />
        </label>
        <label class="form-field">
          <span>{{ t('review.v3.cardsPage.createExplanation') }}<em>{{ t('review.v3.cardsPage.explanationHint') }}</em></span>
          <t-textarea v-model="createForm.explanation" data-testid="review-v3-card-create-explanation" :autosize="{ minRows: 2, maxRows: 4 }" />
        </label>
        <div class="form-field">
          <span>{{ t('review.v3.cardsPage.tags') }}<em>{{ t('review.v3.cardsPage.tagsHint') }}</em></span>
          <div class="tag-editor" data-testid="review-v3-card-create-tags">
            <span v-for="tag in createForm.tags" :key="tag" class="rv-tag rv-tag--blue">
              {{ tag }}<button type="button" @click="removeTag('create', tag)">×</button>
            </span>
            <input
              v-model="createTagDraft"
              class="tag-editor__input"
              :placeholder="t('review.v3.cardsPage.tagsPlaceholder')"
              @keyup.enter="addTag('create')"
            />
          </div>
        </div>
        <div class="form-field">
          <span>{{ t('review.v3.cardsPage.source') }}<em>{{ t('review.v3.cardsPage.sourceHint') }}</em></span>
          <div class="source-card">
            <ReviewIcon name="file" :size="16" :tilt="1" />
            <b>{{ createSourceLabel }}</b>
            <button type="button" class="rv-btn rv-btn--ghost rv-btn--sm" @click="sourceEditing = !sourceEditing">
              {{ sourceEditing ? t('review.cancel') : t('review.v3.cardsPage.changeSource') }}
            </button>
          </div>
          <div v-if="sourceEditing || !createForm.knowledgeId" class="source-picker">
            <t-select
              v-model="createForm.knowledgeBaseId"
              data-testid="review-v3-card-create-kb"
              :options="kbOptions"
              :placeholder="t('review.selectKnowledgeBase')"
              filterable
              @change="(value: string) => loadDocuments(value)"
            />
            <t-select
              v-model="createForm.knowledgeId"
              data-testid="review-v3-card-create-doc"
              :options="docOptions"
              :loading="docsLoading"
              :placeholder="t('review.v3.cardsPage.selectKnowledge')"
              filterable
            />
          </div>
        </div>
        <div class="form-field">
          <span>{{ t('review.v3.cardsPage.memoryParams') }}</span>
          <div class="memory-params">
            <span class="toggle-demo"><i />{{ t('review.v3.cardsPage.bidirectional') }}</span>
            <span class="radio-demo is-on"><i />FSRS</span>
            <span class="radio-demo"><i />SM-2</span>
            <em>{{ t('review.v3.cardsPage.memoryParamsHint') }}</em>
          </div>
        </div>
      </div>
    </div>
    <footer class="cards-subpage__foot">
      <button type="button" class="rv-btn rv-btn--primary" :disabled="createSaving" @click="saveCreate(false)">
        {{ t('review.v3.cardsPage.saveAndNext') }}
      </button>
      <button type="button" class="rv-btn rv-btn--ghost" :disabled="createSaving" @click="saveCreate(true)">
        {{ t('review.v3.cardsPage.saveAndClose') }}
      </button>
      <span class="rv-note cards-subpage__shortcut">⌘ + Enter {{ t('review.v3.cardsPage.quickSave') }}</span>
    </footer>
  </section>

  <div v-else class="cards-page" data-testid="review-v3-cards">
    <div class="cards-panel">
      <div class="rv-props">
        <div class="rv-prop">
          <span class="cards-total">
            <b>{{ t('review.v3.cardsPage.total') }} {{ cards.length }} {{ t('review.v3.cardsPage.totalCards') }}</b>
            <i>·</i>
            <span>{{ props.flatGroups.length }} {{ t('review.v3.cardsPage.groupsUnit') }}</span>
          </span>
        </div>
      </div>

      <div class="cards-toolbar">
        <div class="rv-search cards-toolbar__search">
          <ReviewIcon name="search" :size="15" :tilt="-1" />
          <input
            v-model="filters.keyword"
            data-testid="review-v3-cards-search"
            :placeholder="t('review.searchPlaceholder')"
            @keyup.enter="loadCards()"
          />
        </div>
        <t-select v-model="filters.status" class="cards-toolbar__select" data-testid="review-v3-cards-status" :options="statusOptions" />
        <t-popup
          v-model:visible="groupFilterOpen"
          trigger="click"
          placement="bottom-left"
          :show-arrow="false"
          overlay-inner-class-name="cards-group-filter-popup"
          destroy-on-close
        >
          <template #content>
            <div class="group-filter">
              <button
                type="button"
                class="group-filter__item"
                :class="{ 'is-active': filters.groupId === '' }"
                @click="selectGroupFilter('')"
              >
                <span class="group-filter__spacer" />
                <span class="group-filter__name">{{ t('review.v3.allGroups') }}</span>
              </button>
              <div
                v-for="item in visibleGroupFilters"
                :key="item.group.id"
                class="group-filter__item"
                :class="{ 'is-active': filters.groupId === item.group.id }"
                :style="{ paddingLeft: `${8 + item.depth * 16}px` }"
              >
                <button
                  v-if="groupParentIDs.has(item.group.id)"
                  type="button"
                  class="group-filter__expand"
                  :aria-label="item.group.name"
                  @click.stop="toggleGroupFilterExpand(item.group.id)"
                >
                  <t-icon
                    :name="expandedGroupIds.has(item.group.id) ? 'chevron-down' : 'chevron-right'"
                    size="14px"
                  />
                </button>
                <span v-else class="group-filter__spacer" />
                <button
                  type="button"
                  class="group-filter__name"
                  @click="selectGroupFilter(item.group.id)"
                >
                  {{ item.group.name }}
                </button>
              </div>
            </div>
          </template>
          <button
            type="button"
            class="cards-toolbar__group-trigger"
            data-testid="review-v3-cards-group"
          >
            <span>{{ selectedGroupLabel }}</span>
            <t-icon name="chevron-down" size="14px" />
          </button>
        </t-popup>
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
        <span class="cards-toolbar__spacer" />
        <button
          type="button"
          class="rv-btn rv-btn--primary rv-btn--sm"
          data-testid="review-v3-card-create"
          @click="openCreate"
        >
          <ReviewIcon name="plus" :size="13" />
          {{ t('review.v3.cardsPage.newCard') }}
        </button>
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
      <table v-if="cards.length" class="rv-db card-table" data-testid="review-v3-card-list">
        <thead>
          <tr>
            <th class="card-table__check"><ReviewIcon name="box" :size="15" /></th>
            <th>{{ t('review.v3.cardsPage.question') }}</th>
            <th>{{ t('review.v3.cardsPage.group') }}</th>
            <th>{{ t('review.v3.cardsPage.status') }}</th>
            <th>{{ t('review.v3.cardsPage.interval') }}</th>
            <th>{{ t('review.v3.cardsPage.due') }}</th>
            <th>{{ t('review.v3.cardsPage.errorRate') }}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="card in cards"
            :key="card.id"
            :class="{ 'is-selected': selectedIds.includes(card.id), 'is-deleted': card.status === 'deleted' }"
            :data-card-id="card.id"
            data-testid="review-v3-card-row"
            @click="openPerformance(card)"
          >
            <td class="card-table__check" @click.stop>
              <t-checkbox
                :checked="selectedIds.includes(card.id)"
                :disabled="card.status === 'deleted'"
                @change="(value: boolean) => toggleSelection(card.id, Boolean(value))"
              />
            </td>
            <td class="card-table__main">
              <div class="card-table__question">{{ card.question }}</div>
              <div class="card-table__tags">
                <template v-if="cardTags(card).length">
                  <span v-for="tag in cardTags(card).slice(0, 2)" :key="tag" class="rv-tag rv-tag--gray">#{{ tag }}</span>
                  <span v-if="cardTags(card).length > 2" class="rv-note">+{{ cardTags(card).length - 2 }}</span>
                </template>
                <span v-else class="rv-note">{{ t('review.v3.cardsPage.noTags') }}</span>
              </div>
            </td>
            <td>{{ groupName(card) }}</td>
            <td>
              <ReviewLevelTag v-if="showsLevel(card)" :level="levelOf(card)" />
              <span v-if="card.status === 'suspended'" class="rv-tag rv-tag--gray">{{ t('review.status.suspended') }}</span>
              <span v-else-if="card.status === 'orphaned'" class="rv-tag rv-tag--red">{{ t('review.status.orphaned') }}</span>
              <span v-else-if="card.status === 'deleted'" class="rv-tag rv-tag--gray">{{ t('review.status.deleted') }}</span>
              <span v-else-if="!showsLevel(card)" class="rv-tag rv-tag--gray">{{ t('review.v3.level.healthy') }}</span>
            </td>
            <td>{{ t('review.v3.cardsPage.holdDays', { count: holdDays(card.stability) }) }}</td>
            <td :class="{ 'is-due-today': isDueToday(card) }">{{ dueLabel(card) }}</td>
            <td>
              <div class="card-table__err">
                <span class="card-table__err-track"><i :style="{ width: `${errorRate(card)}%` }" /></span>
                <b>{{ errorRate(card) }}%</b>
              </div>
            </td>
            <td class="card-table__actions card-row__actions" @click.stop>
              <t-dropdown :options="rowMenu(card)" placement="bottom-right" @click="(item: { value: string }) => onRowMenu(item, card)">
                <button type="button" class="rv-btn rv-btn--text rv-btn--sm" :aria-label="t('review.groupMoreActions')">
                  <ReviewIcon name="more" :size="15" />
                </button>
              </t-dropdown>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else-if="!loading" class="cards-state" data-testid="review-v3-cards-empty">
        <ReviewIcon name="book" :size="28" />
        <p>{{ t('review.v3.cardsPage.emptyTitle') }}</p>
        <p class="cards-state__hint">{{ t('review.v3.cardsPage.emptyHint') }}</p>
        <div class="cards-state__actions">
          <button v-if="filters.keyword || filters.tag || filters.status !== 'all' || filters.dueOnly" type="button" class="rv-btn rv-btn--ghost rv-btn--sm" @click="resetFilters">
            {{ t('review.v3.cardsPage.clearFilters') }}
          </button>
          <button type="button" class="rv-btn rv-btn--primary rv-btn--sm" @click="openCreate">{{ t('review.v3.cardsPage.newCard') }}</button>
        </div>
      </div>
      <div v-if="hasMore" class="cards-more">
        <button type="button" class="rv-btn rv-btn--ghost rv-btn--sm" :disabled="loadingMore" @click="loadCards({ append: true })">
          {{ t('review.v3.cardsPage.loadMore') }}
        </button>
      </div>
    </t-loading>







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
  background: transparent;
  border: 1px solid var(--rvn-border);
  border-radius: var(--rvn-radius);

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
      font-size: 14px;
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
  &__group-trigger {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
    width: 168px;
    height: 32px;
    padding: 0 10px;
    overflow: hidden;
    border: 1px solid var(--td-component-stroke);
    border-radius: 3px;
    background: var(--td-bg-color-container);
    color: var(--td-text-color-primary);
    font: inherit;
    font-size: 14px;
    text-align: left;
    cursor: pointer;

    span {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &:hover {
      border-color: var(--td-brand-color);
    }
  }
  &__tag { width: 160px; }
}

.group-filter {
  width: 168px;
  min-width: 0;
  max-height: 280px;
  padding: 4px;
  box-sizing: border-box;
  overflow-x: auto;
  overflow-y: auto;
  overscroll-behavior-x: contain;
  scrollbar-width: none;
  border: 1px solid var(--td-component-stroke);
  border-radius: 0;
  background: var(--td-bg-color-container);
  box-shadow: var(--td-shadow-1);

  &::-webkit-scrollbar {
    display: none;
  }
}

:global(.cards-group-filter-popup) {
  padding: 0;
  margin-top: 0 !important;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.group-filter__item {
  display: flex;
  align-items: center;
  gap: 2px;
  width: max-content;
  min-width: 100%;
  min-height: 32px;
  padding-right: 8px;
  box-sizing: border-box;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--td-text-color-primary);
  cursor: pointer;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }

  &.is-active {
    background: var(--td-brand-color-light);
    color: var(--td-brand-color);

    .group-filter__name {
      color: var(--td-brand-color);
    }
  }
}

.group-filter__expand,
.group-filter__spacer {
  flex: none;
  width: 20px;
  height: 20px;
}

.group-filter__expand {
  display: grid;
  place-items: center;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--td-text-color-secondary);
  cursor: pointer;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }
}

.group-filter__name {
  flex: none;
  min-width: 0;
  padding: 0;
  overflow: visible;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: 14px;
  text-align: left;
  white-space: nowrap;
  cursor: pointer;
}

.batch-bar {
  position: sticky;
  top: 0;
  z-index: 8;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 9px 14px;
  font-size: 14px;
  background: var(--td-brand-color-light);
  border: 1px solid var(--td-brand-color-focus);
  border-radius: 0;

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
  border-radius: 0 0 var(--td-radius-medium) var(--td-radius-medium);

  &__head {
    display: grid;
    grid-template-columns: 28px minmax(0, 1fr) 140px 110px 170px 40px;
    gap: 12px;
    align-items: center;
    min-height: 40px;
    padding: 0 14px;
    font-size: 14px;
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
    font-size: 14px;
    font-weight: 500;
    color: var(--td-text-color-primary);
  }

  &__meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-top: 3px;
    font-size: 14px;
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
    font-size: 14px;
    color: var(--td-text-color-secondary);
  }

  &__status {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
  }

  &__healthy {
    font-size: 14px;
    color: var(--td-success-color);
  }

  &__review {
    display: flex;
    flex-direction: column;
    gap: 1px;
    font-size: 14px;
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
    font-size: 14px;
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

.cards-subpage {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 100%;
  padding: 4px 0 24px;
}

.cards-subpage__head {
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
    color: var(--td-text-color-primary);
  }
}

.cards-subpage__body {
  width: 100%;
}

.cards-subpage__foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 16px;
  border-top: 1px solid var(--rvn-border);
}

.perf-page {
  width: 100%;
  padding: 0;
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
  font-size: 14px;
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
    font-size: 14px;
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
  font-size: 14px;
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
    font-size: 14px;
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
  font-size: 14px;
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
    font-size: 14px;
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

<style lang="less" scoped>
/* Paper-journal presentation for card management and its detail pages. */
.cards-page {
  gap: 0;
}

.cards-panel {
  padding: 0;
  background: transparent;
  border: 0;
}

.cards-toolbar {
  gap: 8px;
  margin-top: 16px;
  padding-top: 0;
  border-top: 0;

  :deep(.t-input),
  :deep(.t-select__wrap) {
    background: var(--rvn-surface);
    border: 1px solid var(--rvn-line);
    border-radius: 7px 6px 8px 7px / 6px 8px 7px 6px;
    box-shadow: none;
  }

  :deep(.t-input__inner) {
    background: transparent;
    border: 0 !important;
    outline: none;
    box-shadow: none !important;
  }

  :deep(.t-select__wrap .t-input) {
    background: transparent;
    border: 0 !important;
    box-shadow: none !important;
  }

  :deep(.t-input:focus-within),
  :deep(.t-select__wrap:focus-within),
  :deep(.t-is-focused) {
    border-color: var(--rvn-accent) !important;
    box-shadow: 0 0 0 2px var(--rvn-accent-soft) !important;
  }

  &__search {
    flex: 0 0 320px;
    width: 320px;
    min-width: 220px;
    height: 38px;
    padding: 0 12px;
    background: var(--rvn-surface);
    border: 1px solid var(--rvn-line);
    border-radius: 7px 6px 8px 7px / 6px 8px 7px 6px;

    input {
      flex: 1;
      min-width: 0;
      height: 36px;
      padding: 0;
      font: inherit;
      font-size: 14px;
      line-height: 36px;
      color: var(--rvn-ink);
      background: transparent;
      border: 0;
      outline: none;

      &::placeholder {
        color: var(--rvn-muted);
      }
    }
  }

  &__select {
    width: 148px;

    :deep(input),
    :deep(.t-input__inner) {
      cursor: pointer;
      caret-color: transparent;
    }
  }

  &__spacer {
    flex: 1;
  }

  &__group-trigger {
    height: 32px;
    background: var(--rvn-surface);
    border-color: var(--rvn-line);
    border-radius: 7px 6px 8px 7px / 6px 8px 7px 6px;
  }
}

.cards-total {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: baseline;
  font-size: 14px;
  color: var(--rvn-ink-soft);

  b {
    font-weight: 600;
    color: var(--rvn-ink);
  }

  i {
    font-style: normal;
    color: var(--rvn-muted);
  }
}

.card-table {
  width: 100%;
  margin-top: 10px;
  border-collapse: collapse;

  th,
  td {
    padding: 10px 10px;
    font-size: 14px;
    text-align: left;
    border-bottom: 1px solid var(--rvn-line);
  }

  th {
    font-size: 14px;
    font-weight: 500;
    color: var(--rvn-muted);
  }

  tbody tr {
    cursor: pointer;
    transition: background 0.18s ease;
  }

  tbody tr:hover td {
    background: var(--rvn-surface);
  }

  tbody tr.is-selected td {
    background: var(--rvn-brand-bg);
  }

  tbody tr.is-deleted {
    opacity: 0.6;
  }
}

.card-table__check {
  width: 34px;
  color: var(--rvn-muted);
}

.card-table__question {
  overflow: hidden;
  font-weight: 500;
  color: var(--rvn-ink);
  white-space: nowrap;
  text-overflow: ellipsis;

  .card-table tbody tr:hover & {
    color: var(--rvn-brand-deep);
  }
}

.card-table__tags {
  display: flex;
  gap: 5px;
  align-items: center;
  margin-top: 4px;
}

.card-table__err {
  display: flex;
  gap: 8px;
  align-items: center;

  b {
    min-width: 34px;
    font-size: 14px;
    color: var(--rvn-red-deep);
  }
}

.card-table__err-track {
  width: 64px;
  height: 5px;
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

.is-due-today {
  font-weight: 600;
  color: var(--rvn-red-deep) !important;
}

.batch-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  padding: 12px 14px;
  margin-top: 12px;
  font-size: 14px;
  color: var(--rvn-ink-soft);
  background: var(--rvn-gray-soft);
  border: 1px solid var(--rvn-line);
  border-radius: 7px 6px 8px 7px / 6px 8px 7px 6px;
}

.cards-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 40px 16px;
  color: var(--rvn-muted);
  text-align: center;
}

.cards-subpage {
  min-height: 100%;
}

.cards-subpage__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 22px;

  h1 {
    margin: 0;
    font-size: 34px;
    font-weight: 700;
    line-height: 1.2;
    letter-spacing: -0.02em;
    color: var(--rvn-ink);
  }
}

.cards-subpage__body {
  max-width: none;
}

.performance-page {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.cards-subpage__foot,
.form-foot {
  display: flex;
  gap: 10px;
  align-items: center;
  width: min(920px, 100%);
  margin: 26px auto 0;
  padding-top: 18px;
  border-top: 1px solid var(--rvn-line);
}

.form-stack {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.card-form {
  max-width: 920px;
  margin: 0 auto;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 8px;

  > span {
    font-size: 14px;
    font-weight: 600;
    color: var(--rvn-ink);

    em {
      margin-left: 8px;
      font-size: 14px;
      font-style: normal;
      font-weight: 400;
      color: var(--rvn-muted);
    }
  }

  :deep(.t-textarea__inner) {
    min-height: 92px !important;
    padding: 11px 12px;
    font-size: 14px;
    line-height: 1.6;
    background: var(--rvn-paper);
    border-color: var(--rvn-line-strong);
    border-radius: 4px;
    box-shadow: none;
  }

  :deep(.t-textarea__inner:focus) {
    border-color: var(--rvn-accent);
    box-shadow: 0 0 0 2px var(--rvn-accent-soft);
  }

  :deep(.t-select__wrap) {
    width: 100%;
  }

  :deep(.t-input) {
    height: 40px;
    background: var(--rvn-paper);
    border-color: var(--rvn-line-strong);
    border-radius: 4px;
    box-shadow: none;
  }
}

.card-form__select {
  width: 100%;
}

.tag-editor {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
  min-height: 40px;
  padding: 6px 10px;
  background: var(--rvn-paper);
  border: 1px solid var(--rvn-line-strong);
  border-radius: 4px;
}

.tag-editor .rv-tag button {
  padding: 0 0 0 2px;
  margin-left: 4px;
  font: inherit;
  color: inherit;
  cursor: pointer;
  background: transparent;
  border: 0;
  opacity: 0.7;

  &:hover {
    opacity: 1;
  }
}

.tag-editor__input {
  flex: 1;
  min-width: 140px;
  padding: 3px 2px;
  font: inherit;
  font-size: 14px;
  color: var(--rvn-ink);
  background: transparent;
  border: 0;
  outline: none;
}

.source-card {
  display: flex;
  gap: 9px;
  align-items: center;
  padding: 12px 14px;
  font-size: 14px;
  background: var(--rvn-surface);
  border: 1px solid var(--rvn-line);
  border-radius: 4px;

  b {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    font-weight: 500;
    color: var(--rvn-ink);
    white-space: nowrap;
    text-overflow: ellipsis;
  }
}

.source-picker {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-top: 10px;
}

.memory-params {
  display: flex;
  flex-wrap: wrap;
  gap: 14px 20px;
  align-items: center;
  font-size: 14px;
  color: var(--rvn-ink-soft);

  em {
    flex: 1 1 100%;
    font-size: 14px;
    font-style: normal;
    color: var(--rvn-muted);
  }
}

.toggle-demo {
  display: inline-flex;
  gap: 8px;
  align-items: center;
  color: var(--rvn-muted);

  i {
    position: relative;
    width: 32px;
    height: 18px;
    background: var(--rvn-line);
    border-radius: 999px;
    opacity: 0.8;

    &::after {
      position: absolute;
      top: 2px;
      left: 2px;
      width: 14px;
      height: 14px;
      content: '';
      background: var(--rvn-paper);
      border-radius: 50%;
    }
  }
}

.radio-demo {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  color: var(--rvn-muted);

  i {
    width: 14px;
    height: 14px;
    border: 1.5px solid var(--rvn-line-strong);
    border-radius: 50%;
  }

  &.is-on {
    color: var(--rvn-ink);

    i {
      border-color: var(--rvn-ink-soft);
      box-shadow: inset 0 0 0 3px var(--rvn-paper);
      background: var(--rvn-ink-soft);
    }
  }
}

.cards-subpage__shortcut {
  margin-left: auto;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;

  > span {
    font-size: 14px;
    font-weight: 600;
    color: var(--rvn-ink);
  }
}

.perf-kpis {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  margin-top: 28px;
}

.perf-kpi {
  padding: 18px 20px;
  background: var(--rvn-paper);
  border: 1px solid var(--rvn-line);
  border-radius: 8px;
}

.perf-kpi__label {
  font-size: 14px;
  color: var(--rvn-muted);
}

.perf-kpi__value {
  margin-top: 8px;
  font-size: 30px;
  font-weight: 700;
  line-height: 1.15;
  letter-spacing: -0.02em;
  color: var(--rvn-ink);
}

.perf-kpi__hint {
  margin-top: 6px;
  font-size: 14px;
  color: var(--rvn-green-deep);

  &.is-down {
    color: var(--rvn-ink-soft);
  }
}

.perf-trend {
  width: 100%;
  height: 220px;
  margin-top: 8px;
}

.perf-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
  margin-top: 24px;
}

.perf-bar {
  display: flex;
  gap: 10px;
  align-items: center;
  margin: 8px 0;
  font-size: 14px;
}

.perf-bar__label {
  display: flex;
  gap: 5px;
  align-items: center;
  width: 62px;
  color: var(--rvn-ink-soft);

  i {
    width: 9px;
    height: 9px;
    border-radius: 50%;

    &.is-red { background: var(--rvn-red); }
    &.is-orange { background: var(--rvn-orange); }
    &.is-green { background: var(--rvn-green); }
    &.is-blue { background: var(--rvn-accent); }
  }
}

.perf-bar__track {
  flex: 1;
  height: 14px;
  overflow: hidden;
  background: var(--rvn-gray-soft);

  i {
    display: block;
    height: 100%;

    &.is-red { background: var(--rvn-red); }
    &.is-orange { background: var(--rvn-orange); }
    &.is-green { background: var(--rvn-green); }
    &.is-blue { background: var(--rvn-accent); }
  }
}

.perf-bar__value {
  width: 30px;
  color: var(--rvn-muted);
  text-align: right;
}

.perf-history-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.perf-history-row {
  display: flex;
  gap: 10px;
  align-items: center;

  &__interval {
    margin-left: auto;
  }
}

.perf-split {
  margin-top: 28px;
}

.perf-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 26px;
}

@media (max-width: 960px) {
  .perf-kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .perf-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 760px) {
  .cards-subpage__head h1 {
    font-size: 28px;
  }

  .cards-toolbar__search {
    flex: 1 1 100%;
  }
}
</style>
