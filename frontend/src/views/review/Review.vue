<script setup lang="ts">
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  AlertTriangle,
  BookOpen,
  Download,
  FilePlus2,
  Keyboard,
  Link2,
  MoreHorizontal,
  Pause,
  Play,
  Plus,
  RefreshCw,
  RotateCcw,
  Search,
  Sparkles,
  Trash2,
  X,
} from '@lucide/vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { listKnowledgeBases } from '@/api/knowledge-base'
import { cleanReviewExcerpt } from '@/utils/reviewExcerpt'
import {
  abandonReviewSession,
  createReviewGroup,
  deleteReviewCard,
  deleteReviewGroup,
  exportReviewData,
  getActiveReviewSession,
  getReviewCard,
  getReviewCardPerformance,
  getReviewQueue,
  getReviewStats,
  importReviewCards,
  listReviewCards,
  listReviewGroups,
  listReviewSessions,
  mergeReviewCards,
  previewReviewGeneration,
  rebindReviewSource,
  restoreReviewCard,
  setReviewCardStatus,
  startReviewSession,
  submitReview,
  updateReviewGroup,
  type GeneratedReviewCard,
  type ReviewCard,
  type ReviewCardDetail,
  type ReviewCardPerformance,
  type ReviewGenerationPreview,
  type ReviewGroup,
  type ReviewGroupNode,
  type ReviewQueue,
  type ReviewSession,
  type ReviewSessionState,
  type ReviewStats,
} from '@/api/review'
import ReviewGroupSidebar from './ReviewGroupSidebar.vue'

const ReviewStatsPanel = defineAsyncComponent(() => import('./ReviewStatsPanel.vue'))

const { t } = useI18n()

interface FlatGroup {
  group: ReviewGroup
  depth: number
  card_count: number
}

type ConfirmAction = () => void | Promise<void>

const groups = ref<ReviewGroupNode[]>([])
const flatGroups = ref<FlatGroup[]>([])
const selectedGroupId = ref('')
const activeTab = ref('review')
const cards = ref<ReviewCard[]>([])
const searchKeyword = ref('')
const statusFilter = ref('all')
const selectedCardIDs = ref<string[]>([])
const queue = ref<ReviewQueue | null>(null)
const stats = ref<ReviewStats | null>(null)
const statsDays = ref(30)
const sessions = ref<ReviewSession[]>([])
const loading = ref(false)
const ratingLoading = ref(false)
const groupDialogVisible = ref(false)
const groupDialogMode = ref<'create' | 'rename'>('create')
const groupSaving = ref(false)
const groupForm = reactive({ name: '' })
const groupParentID = ref('')
const groupDeleteVisible = ref(false)
const groupDeleteTarget = ref<ReviewGroup | null>(null)
const groupDeleteCards = ref(false)
const reviewing = ref(false)
const finished = ref(false)
const reviewCards = ref<ReviewCard[]>([])
const reviewIndex = ref(0)
const showingAnswer = ref(false)
const sessionDone = ref(0)
const sessionID = ref('')
const sessionTotal = ref(0)
const activeSession = ref<ReviewSessionState | null>(null)
const performanceVisible = ref(false)
const performanceLoading = ref(false)
const performance = ref<ReviewCardPerformance | null>(null)
const currentDetail = ref<ReviewCardDetail | null>(null)
const undoCardID = ref('')
let undoTimer: ReturnType<typeof setTimeout> | undefined
let toastTimer: ReturnType<typeof setTimeout> | undefined
const currentSourceExcerpt = computed(() => {
  const source = currentDetail.value?.sources?.[0]
  if (!source) return ''
  return cleanReviewExcerpt(source.excerpt || source.path || source.knowledge_id)
})
const rebindVisible = ref(false)
const rebindSaving = ref(false)
const rebindSourceID = ref('')
const rebindCardID = ref('')
const rebindForm = reactive({ knowledgeID: '', chunkID: '', excerpt: '' })
const generateVisible = ref(false)
const generating = ref(false)
const importing = ref(false)
const generationKB = ref('')
const generationCount = ref('5')
const generationGroup = ref('')
const generationPreview = ref<ReviewGenerationPreview | null>(null)
const selectedGenerated = ref<number[]>([])
const kbOptions = ref<Array<{ label: string; value: string }>>([])
const groupOptions = computed(() => flatGroups.value.map(item => ({
  label: `${'　'.repeat(item.depth)}${item.group.name}`,
  value: item.group.id,
})))
const countOptions = [5, 10, 20].map(value => ({ label: String(value), value: String(value) }))

const toast = ref<{ type: 'success' | 'error' | 'warning'; message: string } | null>(null)
const confirmVisible = ref(false)
const confirmTitle = ref('')
const confirmMessage = ref('')
const confirmText = ref('')
const confirmDanger = ref(false)
let confirmAction: ConfirmAction | null = null

const selectedGroup = computed(() => flatGroups.value.find(item => item.group.id === selectedGroupId.value)?.group ?? null)
const queueHasCards = computed(() => Boolean(queue.value && (queue.value.due.length + queue.value.new.length) > 0))
const currentCard = computed(() => reviewCards.value[reviewIndex.value] ?? null)
const statusOptions = computed(() => [
  { label: t('review.allStatuses'), value: 'all' },
  { label: t('review.status.active'), value: 'active' },
  { label: t('review.status.suspended'), value: 'suspended' },
  { label: t('review.status.orphaned'), value: 'orphaned' },
])
const dailyProgress = computed(() => {
  if (!queue.value || queue.value.limit <= 0) return 0
  return Math.min(100, Math.round((queue.value.completed_today / queue.value.limit) * 100))
})

function showToast(type: 'success' | 'error' | 'warning', message: string) {
  toast.value = { type, message }
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { toast.value = null }, 3200)
}

function askConfirm(options: {
  title: string
  message: string
  confirmText?: string
  danger?: boolean
  action: ConfirmAction
}) {
  confirmTitle.value = options.title
  confirmMessage.value = options.message
  confirmText.value = options.confirmText || t('review.confirm')
  confirmDanger.value = Boolean(options.danger)
  confirmAction = options.action
  confirmVisible.value = true
}

async function runConfirm() {
  const action = confirmAction
  confirmVisible.value = false
  confirmAction = null
  if (action) await action()
}

function flattenGroups(nodes: ReviewGroupNode[], depth = 0): FlatGroup[] {
  return nodes.flatMap(node => [
    { group: node.group, depth, card_count: node.card_count || 0 },
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
    status: statusFilter.value === 'all' ? undefined : statusFilter.value,
    keyword: searchKeyword.value.trim() || undefined,
    limit: 200,
  })
  cards.value = response.data || []
  const visible = new Set(cards.value.map(card => card.id))
  selectedCardIDs.value = selectedCardIDs.value.filter(id => visible.has(id))
}

async function loadQueue() {
  const response = await getReviewQueue({ group_id: selectedGroupId.value || undefined, limit: 30 })
  queue.value = response.data || { due: [], new: [], completed_today: 0, limit: 30, new_limit: 20, streak_days: 0 }
}

async function loadStats() {
  const response = await getReviewStats(statsDays.value)
  stats.value = response.data || null
}

async function loadSessions() {
  try {
    const response = await listReviewSessions(50)
    sessions.value = response.data || []
  } catch {
    sessions.value = []
  }
}

async function loadKnowledgeBasesForGeneration() {
  if (kbOptions.value.length) return
  try {
    const response: any = await listKnowledgeBases()
    const list = response.data?.data || response.data || []
    kbOptions.value = (Array.isArray(list) ? list : []).map((kb: any) => ({
      value: kb.id,
      label: kb.name || kb.id,
    }))
  } catch {
    kbOptions.value = []
  }
}

function defaultGroupID() {
  return selectedGroupId.value || flatGroups.value.find(item => item.group.is_default)?.group.id || ''
}

function openGeneration() {
  generationPreview.value = null
  selectedGenerated.value = []
  generationGroup.value = defaultGroupID()
  generationCount.value = '5'
  generateVisible.value = true
  void loadKnowledgeBasesForGeneration()
}

async function generatePreview() {
  if (!generationKB.value) {
    showToast('warning', t('review.selectKnowledgeBase'))
    return
  }
  generating.value = true
  try {
    const response = await previewReviewGeneration({
      knowledge_base_id: generationKB.value,
      group_id: generationGroup.value,
      count: Number(generationCount.value),
    })
    generationPreview.value = response.data || null
    selectedGenerated.value = (generationPreview.value?.cards || [])
      .map((card, index) => (card.duplicate ? -1 : index))
      .filter(index => index >= 0)
  } catch (error: any) {
    showToast('error', error?.message || t('review.generateFailed'))
  } finally {
    generating.value = false
  }
}

function toggleGenerated(index: number, checked: boolean) {
  if (checked) {
    if (!selectedGenerated.value.includes(index)) selectedGenerated.value.push(index)
  } else {
    selectedGenerated.value = selectedGenerated.value.filter(item => item !== index)
  }
}

async function importSelectedGenerated() {
  if (!generationPreview.value || !selectedGenerated.value.length) {
    showToast('warning', t('review.selectGeneratedCards'))
    return
  }
  importing.value = true
  try {
    const cards = selectedGenerated.value
      .map(index => generationPreview.value?.cards[index])
      .filter((card): card is GeneratedReviewCard => Boolean(card))
      .map(card => ({
        group_id: generationGroup.value,
        question: card.question,
        answer: card.answer,
        explanation: card.explanation,
        sources: card.sources,
      }))
    const response = await importReviewCards({ group_id: generationGroup.value, cards })
    showToast('success', t('review.importedCount', { count: response.data?.imported ?? cards.length }))
    generateVisible.value = false
    await Promise.all([loadCards(), loadQueue(), loadStats()])
  } catch (error: any) {
    showToast('error', error?.message || t('review.importFailed'))
  } finally {
    importing.value = false
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

async function refreshAll() {
  loading.value = true
  try {
    await loadGroups()
    await Promise.all([loadCards(), loadQueue(), loadStats(), loadActiveSession(), loadSessions()])
  } catch (error: any) {
    showToast('error', error?.message || t('review.loadFailed'))
  } finally {
    loading.value = false
  }
}

function selectGroup(groupId: string) {
  selectedGroupId.value = groupId
  selectedCardIDs.value = []
  void refreshAll()
}

function changeStatsDays(days: number) {
  statsDays.value = days
  void loadStats()
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
    showToast('warning', t('review.mergeNeedTwo'))
    return
  }
  const [targetID, ...sourceIDs] = selectedCardIDs.value
  askConfirm({
    title: t('review.mergeCards'),
    message: t('review.mergeConfirm', { count: sourceIDs.length }),
    action: async () => {
      try {
        await mergeReviewCards({ target_card_id: targetID, source_card_ids: sourceIDs })
        showToast('success', t('review.merged'))
        selectedCardIDs.value = []
        await Promise.all([loadCards(), loadQueue(), loadStats()])
      } catch (error: any) {
        showToast('error', error?.message || t('review.actionFailed'))
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
    showToast('error', error?.message || t('review.exportFailed'))
  }
}

function csvCell(value: unknown) {
  const text = String(value ?? '')
  return `"${text.replace(/"/g, '""')}"`
}

function exportSessionsCSV() {
  if (!sessions.value.length) return
  const headers = ['id', 'mode', 'status', 'planned_count', 'completed_count', 'remembered_count', 'fuzzy_count', 'forgot_count', 'review_day', 'started_at', 'completed_at']
  const rows = sessions.value.map(session => [
    session.id,
    session.mode,
    session.status,
    session.planned_count,
    session.completed_count,
    session.remembered_count,
    session.fuzzy_count,
    session.forgot_count,
    session.review_day,
    session.started_at,
    session.completed_at || '',
  ])
  const csv = [headers, ...rows].map(row => row.map(csvCell).join(',')).join('\n')
  const blob = new Blob([`﻿${csv}`], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `wellread-review-sessions-${new Date().toISOString().slice(0, 10)}.csv`
  link.click()
  URL.revokeObjectURL(url)
}

function openCreateGroup(parentID = '') {
  groupDialogMode.value = 'create'
  groupParentID.value = parentID
  groupForm.name = ''
  groupDialogVisible.value = true
}

function openRenameGroup(group: ReviewGroup | null = selectedGroup.value) {
  if (!group) return
  selectedGroupId.value = group.id
  groupDialogMode.value = 'rename'
  groupForm.name = group.name
  groupDialogVisible.value = true
}

function openDeleteGroup(group: ReviewGroup | null = selectedGroup.value) {
  if (!group) return
  groupDeleteTarget.value = group
  groupDeleteCards.value = false
  groupDeleteVisible.value = true
}

async function saveGroup() {
  const name = groupForm.name.trim()
  if (!name) {
    showToast('warning', t('review.groupNamePlaceholder'))
    return
  }
  groupSaving.value = true
  try {
    if (groupDialogMode.value === 'create') {
      await createReviewGroup({ name, parent_id: groupParentID.value || undefined })
      showToast('success', t('review.groupCreated'))
    } else if (selectedGroup.value) {
      await updateReviewGroup(selectedGroup.value.id, { name })
      showToast('success', t('review.groupUpdated'))
    }
    groupDialogVisible.value = false
    await refreshAll()
  } catch (error: any) {
    showToast('error', error?.message || t('review.groupFailed'))
  } finally {
    groupSaving.value = false
  }
}

async function doDeleteGroup() {
  const group = groupDeleteTarget.value
  if (!group) return
  try {
    await deleteReviewGroup(group.id, groupDeleteCards.value)
    showToast('success', t('review.groupDeleted'))
    groupDeleteVisible.value = false
    selectedGroupId.value = ''
    await refreshAll()
  } catch (error: any) {
    showToast('error', error?.message || t('review.groupFailed'))
  }
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

function applySession(state: ReviewSessionState | null | undefined) {
  if (!state) return
  activeSession.value = state
  sessionID.value = state.session.id
  sessionTotal.value = state.session.planned_count
  reviewCards.value = state.cards
  reviewIndex.value = 0
  sessionDone.value = state.session.completed_count
  showingAnswer.value = false
  finished.value = false
  reviewing.value = true
  void loadCurrentDetail()
}

function resumeSession() {
  applySession(activeSession.value)
}

async function startReview() {
  if (reviewing.value) return
  if (activeSession.value) {
    resumeSession()
    return
  }
  try {
    const response = await startReviewSession({
      mode: 'daily',
      group_id: selectedGroupId.value || undefined,
      limit: 30,
    })
    applySession(response.data)
  } catch (error: any) {
    showToast('error', error?.message || t('review.loadFailed'))
  }
}

async function startSelectedReview() {
  if (!selectedCardIDs.value.length) return
  if (activeSession.value) {
    askConfirm({
      title: t('review.abandonSession'),
      message: t('review.abandonSessionConfirm'),
      danger: true,
      action: () => beginSelectedReview(true),
    })
    return
  }
  await beginSelectedReview(false)
}

async function beginSelectedReview(forceNew: boolean) {
  if (!selectedCardIDs.value.length) return
  try {
    const response = await startReviewSession({
      mode: 'manual',
      group_id: selectedGroupId.value || undefined,
      card_ids: selectedCardIDs.value,
      force_new: forceNew || undefined,
    })
    selectedCardIDs.value = []
    applySession(response.data)
  } catch (error: any) {
    showToast('error', error?.message || t('review.loadFailed'))
  }
}

function abandonCurrentSession() {
  if (!sessionID.value) return
  askConfirm({
    title: t('review.abandonSession'),
    message: t('review.abandonSessionConfirm'),
    danger: true,
    action: async () => {
      try {
        await abandonReviewSession(sessionID.value)
        activeSession.value = null
        sessionID.value = ''
        showToast('success', t('review.sessionAbandoned'))
        await refreshAll()
      } catch (error: any) {
        showToast('error', error?.message || t('review.actionFailed'))
      }
    },
  })
}

async function openPerformance(card: ReviewCard) {
  performanceVisible.value = true
  performanceLoading.value = true
  performance.value = null
  try {
    const response = await getReviewCardPerformance(card.id)
    performance.value = response.data || null
  } catch (error: any) {
    showToast('error', error?.message || t('review.loadFailed'))
    performanceVisible.value = false
  } finally {
    performanceLoading.value = false
  }
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
      session_id: sessionID.value || undefined,
      rating,
      client_request_id: crypto.randomUUID(),
    })
    sessionDone.value += 1
    nextCard()
  } catch (error: any) {
    showToast('error', error?.message || t('review.actionFailed'))
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
      showToast('warning', t('review.noKnowledgeSource'))
      return
    }
    rebindCardID.value = card.id
    rebindSourceID.value = source.id
    rebindForm.knowledgeID = source.knowledge_id
    rebindForm.chunkID = source.chunk_id
    rebindForm.excerpt = source.excerpt
    rebindVisible.value = true
  } catch (error: any) {
    showToast('error', error?.message || t('review.actionFailed'))
  }
}

async function saveRebind() {
  if (!rebindCardID.value || !rebindSourceID.value || !rebindForm.knowledgeID.trim()) {
    showToast('warning', t('review.rebindRequired'))
    return
  }
  rebindSaving.value = true
  try {
    await rebindReviewSource(rebindCardID.value, rebindSourceID.value, {
      knowledge_id: rebindForm.knowledgeID.trim(),
      chunk_id: rebindForm.chunkID.trim(),
      excerpt: rebindForm.excerpt.trim(),
    })
    showToast('success', t('review.rebound'))
    rebindVisible.value = false
    await Promise.all([loadCards(), loadQueue(), loadStats()])
  } catch (error: any) {
    showToast('error', error?.message || t('review.actionFailed'))
  } finally {
    rebindSaving.value = false
  }
}

async function toggleSuspend(card: ReviewCard) {
  const nextStatus = card.status === 'suspended' ? 'active' : 'suspended'
  try {
    await setReviewCardStatus(card.id, nextStatus)
    showToast('success', t('review.cardUpdated'))
    await Promise.all([loadCards(), loadQueue(), loadStats()])
  } catch (error: any) {
    showToast('error', error?.message || t('review.actionFailed'))
  }
}

function confirmDeleteCard(card: ReviewCard) {
  askConfirm({
    title: t('review.deleteCard'),
    message: t('review.deleteCardConfirm'),
    danger: true,
    action: async () => {
      try {
        await deleteReviewCard(card.id)
        undoCardID.value = card.id
        if (undoTimer) clearTimeout(undoTimer)
        undoTimer = setTimeout(() => { undoCardID.value = '' }, 5000)
        showToast('success', t('review.cardDeleted'))
        await Promise.all([loadCards(), loadQueue(), loadStats()])
      } catch (error: any) {
        showToast('error', error?.message || t('review.actionFailed'))
      }
    },
  })
}

async function undoDelete() {
  const cardID = undoCardID.value
  if (!cardID) return
  if (undoTimer) clearTimeout(undoTimer)
  undoCardID.value = ''
  try {
    await restoreReviewCard(cardID)
    showToast('success', t('review.restored'))
    await Promise.all([loadCards(), loadQueue()])
  } catch (error: any) {
    showToast('error', error?.message || t('review.actionFailed'))
  }
}

function formatTime(value?: string | null) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatDue(value?: string | null) {
  if (!value) return '-'
  return new Date(value).toLocaleDateString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
  })
}

watch(activeTab, (tab) => {
  if (tab === 'stats') {
    void loadStats()
    void loadSessions()
  }
})

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  void refreshAll()
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  if (undoTimer) clearTimeout(undoTimer)
  if (toastTimer) clearTimeout(toastTimer)
})
</script>

<template>
  <div class="review-scope bg-background text-foreground h-full overflow-y-auto" data-testid="review-page">
    <div class="mx-auto w-full max-w-[1200px] px-4 py-5 sm:px-6 lg:px-8">
      <header v-if="!reviewing" class="mb-4 flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div class="min-w-0">
          <h1 class="text-[22px] leading-8 font-semibold tracking-tight">{{ t('review.title') }}</h1>
          <p class="text-muted-foreground mt-0.5 text-[13px] leading-5">{{ t('review.subtitle') }}</p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" :disabled="loading" @click="refreshAll">
            <RefreshCw :class="{ 'animate-spin': loading }" />{{ t('review.refresh') }}
          </Button>
          <Button variant="outline" size="sm" @click="handleExport">
            <Download />{{ t('review.export') }}
          </Button>
          <Button variant="outline" size="sm" @click="openGeneration">
            <FilePlus2 />{{ t('review.generate') }}
          </Button>
          <Button size="sm" @click="openCreateGroup()">
            <Plus />{{ t('review.newGroup') }}
          </Button>
        </div>
      </header>

      <section v-if="reviewing" class="mx-auto w-full max-w-2xl space-y-3" data-testid="review-run">
        <div class="text-muted-foreground flex items-center justify-between gap-2 text-sm">
          <span>{{ t('review.reviewing') }}</span>
          <span data-testid="review-progress">{{ t('review.progress', { current: sessionDone + 1, total: sessionTotal || reviewCards.length }) }}</span>
          <Button variant="ghost" size="sm" @click="exitReview">{{ t('review.exit') }}</Button>
        </div>
        <Card v-if="currentCard">
          <CardContent class="space-y-4 pt-4">
            <div class="flex flex-wrap items-center gap-2">
              <Badge variant="secondary">{{ currentCard.review_count ? `L${currentCard.review_level}` : t('review.newCards') }}</Badge>
              <Badge v-if="currentCard.lapses" variant="destructive">{{ currentCard.lapses }} × {{ t('review.forgot') }}</Badge>
            </div>
            <div class="text-lg font-medium leading-7" data-testid="review-question">{{ currentCard.question }}</div>
            <div v-if="showingAnswer" class="space-y-3" data-testid="review-answer">
              <div>
                <div class="text-muted-foreground mb-1 text-xs">{{ t('review.answer') }}</div>
                <div class="text-sm leading-6">{{ currentCard.answer }}</div>
              </div>
              <div v-if="currentCard.explanation">
                <div class="text-muted-foreground mb-1 text-xs">{{ t('review.explanation') }}</div>
                <div class="text-sm leading-6">{{ currentCard.explanation }}</div>
              </div>
              <div v-if="currentDetail?.sources?.length" class="border-border/60 rounded-md border p-3">
                <div class="text-muted-foreground mb-1 text-xs">{{ currentDetail.sources[0].knowledge_id ? t('review.sourceExcerpt') : t('review.sourceUnavailable') }}</div>
                <div class="text-muted-foreground text-sm leading-6">{{ currentSourceExcerpt }}</div>
              </div>
            </div>
            <div class="flex flex-wrap gap-2">
              <Button v-if="!showingAnswer" class="w-full sm:w-auto" data-testid="review-show-answer" @click="showingAnswer = true">
                {{ t('review.showAnswer') }}
              </Button>
              <template v-else>
                <Button variant="destructive" :disabled="ratingLoading" data-testid="review-rate-forgot" @click="rate('forgot')">
                  {{ t('review.forgot') }} <span class="text-xs opacity-70">1</span>
                </Button>
                <Button variant="outline" :disabled="ratingLoading" data-testid="review-rate-fuzzy" @click="rate('fuzzy')">
                  {{ t('review.fuzzy') }} <span class="text-xs opacity-70">2</span>
                </Button>
                <Button :disabled="ratingLoading" data-testid="review-rate-remembered" @click="rate('remembered')">
                  {{ t('review.remembered') }} <span class="text-xs opacity-70">3</span>
                </Button>
              </template>
            </div>
            <p class="text-muted-foreground flex items-center gap-1.5 text-xs">
              <Keyboard class="size-3.5" />{{ t('review.shortcutHint') }}
            </p>
          </CardContent>
        </Card>
      </section>

      <section v-else-if="finished" class="mx-auto max-w-xl py-16 text-center">
        <div class="bg-primary/10 text-primary mx-auto mb-3 flex size-12 items-center justify-center rounded-full">
          <Sparkles class="size-6" />
        </div>
        <h3 class="text-lg font-semibold">{{ t('review.finished') }}</h3>
        <p class="text-muted-foreground mt-1 text-sm">{{ t('review.finishedHint', { count: sessionDone }) }}</p>
        <Button class="mt-4" @click="exitReview">{{ t('review.backHome') }}</Button>
      </section>

      <Tabs v-else v-model="activeTab" default-value="review">
        <TabsList variant="line" class="w-fit">
          <TabsTrigger value="review" data-testid="review-tab-review">{{ t('review.tabs.review') }}</TabsTrigger>
          <TabsTrigger value="cards" data-testid="review-tab-cards">{{ t('review.tabs.cards') }}</TabsTrigger>
          <TabsTrigger value="stats" data-testid="review-tab-stats">{{ t('review.tabs.stats') }}</TabsTrigger>
        </TabsList>

        <TabsContent value="review" class="mt-4">
          <div class="grid gap-4 lg:grid-cols-[232px_minmax(0,1fr)]">
            <ReviewGroupSidebar
              :groups="flatGroups"
              :selected-group-id="selectedGroupId"
              :total-cards="stats?.total_cards ?? cards.length"
              @select="selectGroup"
              @create="openCreateGroup"
              @rename="openRenameGroup"
              @delete="openDeleteGroup"
            />
            <main class="min-w-0 space-y-4">
              <div v-if="activeSession" class="border-primary/25 bg-primary/5 flex flex-col gap-2 rounded-md border px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between" data-testid="review-active-session">
                <span class="text-sm">{{ t('review.activeSession', { completed: activeSession.session.completed_count, total: activeSession.session.planned_count }) }}</span>
                <div class="flex gap-2">
                  <Button size="sm" @click="resumeSession">{{ t('review.resumeSession') }}</Button>
                  <Button variant="ghost" size="sm" class="text-destructive" @click="abandonCurrentSession">{{ t('review.abandonSession') }}</Button>
                </div>
              </div>

              <div class="grid gap-3 md:grid-cols-3">
                <Card class="h-full py-0">
                  <CardContent class="flex h-full flex-col p-4">
                    <div class="text-muted-foreground text-[13px] font-medium">{{ t('review.today') }}</div>
                    <div class="mt-2 grid grid-cols-2 gap-3">
                      <div>
                        <div class="text-2xl leading-8 font-semibold tabular-nums">{{ queue?.due.length ?? 0 }}</div>
                        <div class="text-muted-foreground text-xs">{{ t('review.due') }}</div>
                      </div>
                      <div>
                        <div class="text-2xl leading-8 font-semibold tabular-nums">{{ queue?.new.length ?? 0 }}</div>
                        <div class="text-muted-foreground text-xs">{{ t('review.newCards') }}</div>
                      </div>
                    </div>
                    <div class="mt-auto pt-3">
                      <Button class="w-full" :disabled="!queueHasCards && !activeSession" data-testid="review-start" @click="startReview">
                        <Play />{{ activeSession ? t('review.resumeSession') : t('review.start') }}
                      </Button>
                      <p v-if="queueHasCards || activeSession" class="text-muted-foreground mt-1.5 text-xs leading-5">{{ t('review.startHint') }}</p>
                    </div>
                  </CardContent>
                </Card>
                <Card class="h-full py-0">
                  <CardContent class="flex h-full flex-col p-4">
                    <div class="flex items-center justify-between">
                      <div class="text-muted-foreground text-[13px] font-medium">{{ t('review.todayProgress') }}</div>
                      <span class="text-muted-foreground text-xs tabular-nums">{{ dailyProgress }}%</span>
                    </div>
                    <div class="mt-2 text-2xl leading-8 font-semibold tabular-nums">
                      {{ queue?.completed_today ?? 0 }}<span class="text-muted-foreground text-sm font-normal"> / {{ queue?.limit ?? 30 }}</span>
                    </div>
                    <Progress :model-value="dailyProgress" class="mt-3 h-1.5" />
                    <div class="text-muted-foreground mt-auto pt-3 text-xs leading-5">{{ t('review.completed') }} {{ queue?.completed_today ?? 0 }} · {{ t('review.limit') }} {{ queue?.limit ?? 30 }}</div>
                  </CardContent>
                </Card>
                <Card class="h-full py-0">
                  <CardContent class="flex h-full flex-col p-4">
                    <div class="text-muted-foreground text-[13px] font-medium">{{ t('review.longTerm') }}</div>
                    <div class="mt-2 grid grid-cols-2 gap-3">
                      <div>
                        <div class="text-2xl leading-8 font-semibold tabular-nums">{{ stats?.total_cards ?? cards.length }}</div>
                        <div class="text-muted-foreground text-xs">{{ t('review.totalCards') }}</div>
                      </div>
                      <div>
                        <div class="text-2xl leading-8 font-semibold tabular-nums">{{ Math.round((stats?.retention_30d ?? 0) * 100) }}%</div>
                        <div class="text-muted-foreground text-xs">{{ t('review.retention30d') }}</div>
                      </div>
                    </div>
                    <div class="text-muted-foreground mt-auto pt-3 text-xs leading-5">{{ t('review.currentGroup') }}：{{ selectedGroup?.name || t('review.allGroups') }}</div>
                  </CardContent>
                </Card>
              </div>

              <div v-if="stats?.weak_topics?.length" class="space-y-2">
                <div class="flex items-center justify-between gap-2">
                  <h2 class="text-sm font-medium">{{ t('review.weakTopics') }}</h2>
                  <span class="text-muted-foreground text-xs">{{ t('review.stats.retention') }}</span>
                </div>
                <div class="bg-card divide-border divide-y overflow-hidden rounded-md border">
                  <div v-for="topic in stats.weak_topics" :key="topic.group_id" class="flex items-center justify-between gap-3 px-3 py-2.5 text-sm">
                    <span class="truncate">{{ topic.group_name || topic.group_id }}</span>
                    <span class="text-muted-foreground flex shrink-0 items-center gap-2 text-xs tabular-nums">
                      <Badge :variant="topic.retention < 0.5 ? 'destructive' : 'secondary'">{{ Math.round(topic.retention * 100) }}%</Badge>
                      <span>{{ topic.reviewed }} {{ t('review.reviews') }}</span>
                    </span>
                  </div>
                </div>
              </div>

              <div v-if="!queueHasCards" class="bg-card rounded-md border border-dashed px-6 py-10 text-center">
                <BookOpen class="text-muted-foreground mx-auto mb-2 size-5" />
                <h3 class="text-sm font-medium">{{ t('review.noDue') }}</h3>
                <p class="text-muted-foreground mt-1 text-xs">{{ t('review.queueEmpty') }}</p>
              </div>
            </main>
          </div>
        </TabsContent>

        <TabsContent value="cards" class="mt-4">
          <div class="grid gap-4 lg:grid-cols-[232px_minmax(0,1fr)]">
            <ReviewGroupSidebar
              :groups="flatGroups"
              :selected-group-id="selectedGroupId"
              :total-cards="stats?.total_cards ?? cards.length"
              @select="selectGroup"
              @create="openCreateGroup"
              @rename="openRenameGroup"
              @delete="openDeleteGroup"
            />
            <main class="min-w-0 space-y-3">
              <div class="flex flex-wrap items-center gap-2">
                <div class="relative min-w-0 flex-1 sm:max-w-[320px]">
                  <Search class="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
                  <Input v-model="searchKeyword" class="pl-8" :placeholder="t('review.searchPlaceholder')" @keyup.enter="loadCards" />
                </div>
                <Select v-model="statusFilter" @update:model-value="loadCards">
                  <SelectTrigger class="w-[128px]"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="option in statusOptions" :key="option.value" :value="option.value">{{ option.label }}</SelectItem>
                  </SelectContent>
                </Select>
                <span class="text-muted-foreground ml-auto text-xs tabular-nums">{{ t('review.cardCount', { count: cards.length }) }}</span>
              </div>

              <div v-if="selectedCardIDs.length" class="border-primary/25 bg-primary/5 flex flex-wrap items-center justify-between gap-2 rounded-md border px-3 py-2">
                <span class="text-sm">{{ t('review.selectedCount', { count: selectedCardIDs.length }) }}</span>
                <div class="flex flex-wrap gap-2">
                  <Button variant="outline" size="sm" @click="startSelectedReview">
                    <Play />{{ t('review.startSelected') }} ({{ selectedCardIDs.length }})
                  </Button>
                  <Button v-if="selectedCardIDs.length > 1" variant="outline" size="sm" @click="confirmMergeCards">
                    {{ t('review.mergeCards') }} ({{ selectedCardIDs.length }})
                  </Button>
                </div>
              </div>

              <div v-if="undoCardID" class="bg-muted flex items-center justify-between rounded-md px-3 py-2 text-sm">
                <span>{{ t('review.deletedHint') }}</span>
                <Button variant="ghost" size="sm" @click="undoDelete">{{ t('review.undo') }}</Button>
              </div>

              <div v-if="loading" class="space-y-2">
                <Skeleton v-for="index in 3" :key="index" class="h-14 w-full" />
              </div>
              <div v-else-if="cards.length" class="bg-card divide-border divide-y overflow-hidden rounded-md border">
                <div
                  v-for="card in cards"
                  :key="card.id"
                  class="group flex items-start gap-3 px-3 py-2.5 transition-colors sm:items-center"
                  :class="selectedCardIDs.includes(card.id) ? 'bg-primary/5' : 'hover:bg-muted/60'"
                  data-testid="review-card-row"
                >
                  <Checkbox
                    class="mt-0.5 border-[var(--td-gray-color-5)] sm:mt-0"
                    :model-value="selectedCardIDs.includes(card.id)"
                    @update:model-value="(value) => toggleCardSelection(card.id, Boolean(value))"
                  />
                  <button
                    type="button"
                    class="min-w-0 flex-1 rounded-sm text-left outline-none focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:ring-offset-1"
                    data-testid="review-card-open"
                    @click="openPerformance(card)"
                  >
                    <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
                      <span class="truncate text-sm font-medium">{{ card.question }}</span>
                      <Badge v-if="card.status === 'suspended'" variant="outline">{{ t('review.status.suspended') }}</Badge>
                      <Badge v-else-if="card.status === 'orphaned'" variant="destructive">{{ t('review.status.orphaned') }}</Badge>
                      <Badge v-if="card.forget_streak >= 2 || card.lapses >= 3" variant="destructive">{{ t('review.warningBadge') }}</Badge>
                    </div>
                    <div class="text-muted-foreground mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
                      <span>L{{ card.review_level }}</span>
                      <span v-if="card.review_count">{{ t('review.stats.reviewCount', { count: card.review_count }) }}</span>
                      <span v-if="card.lapses">{{ card.lapses }} × {{ t('review.forgot') }}</span>
                      <span>{{ t('review.nextDue') }} {{ formatDue(card.due_at) }}</span>
                    </div>
                  </button>
                  <DropdownMenu>
                    <DropdownMenuTrigger as-child>
                      <Button variant="ghost" size="icon-sm" class="opacity-60 group-hover:opacity-100 focus-visible:opacity-100" :title="t('review.moreActions')"><MoreHorizontal /></Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem v-if="card.status === 'active'" @select="toggleSuspend(card)">
                        <Pause />{{ t('review.suspend') }}
                      </DropdownMenuItem>
                      <DropdownMenuItem v-if="card.status === 'suspended'" @select="toggleSuspend(card)">
                        <RotateCcw />{{ t('review.resume') }}
                      </DropdownMenuItem>
                      <DropdownMenuItem v-if="card.status === 'orphaned'" @select="openRebind(card)">
                        <Link2 />{{ t('review.rebindSource') }}
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem variant="destructive" @select="confirmDeleteCard(card)">
                        <Trash2 />{{ t('review.deleteCard') }}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </div>
              <div v-else class="bg-card text-muted-foreground rounded-md border border-dashed px-6 py-10 text-center text-sm">{{ t('review.noCards') }}</div>
            </main>
          </div>
        </TabsContent>

        <TabsContent value="stats" class="mt-4">
          <ReviewStatsPanel
            v-if="activeTab === 'stats'"
            :stats="stats"
            :sessions="sessions"
            :days="statsDays"
            :loading="loading"
            @update:days="changeStatsDays"
            @export-csv="exportSessionsCSV"
          />
        </TabsContent>
      </Tabs>

      <Dialog v-model:open="groupDialogVisible">
        <DialogContent class="review-scope sm:max-w-[420px]">
          <DialogHeader>
            <DialogTitle>{{ groupDialogMode === 'create' ? t('review.newGroup') : t('review.renameGroup') }}</DialogTitle>
          </DialogHeader>
          <div class="space-y-2">
            <label class="text-sm">{{ t('review.groupName') }}</label>
            <Input v-model="groupForm.name" :placeholder="t('review.groupNamePlaceholder')" maxlength="200" @keyup.enter="saveGroup" />
            <p v-if="groupDialogMode === 'create'" class="text-muted-foreground text-xs">
              {{ t('review.groupParent') }}：{{ flatGroups.find(item => item.group.id === groupParentID)?.group.name || t('review.rootGroup') }}
            </p>
          </div>
          <DialogFooter>
            <Button variant="outline" @click="groupDialogVisible = false">{{ t('review.cancel') }}</Button>
            <Button :disabled="groupSaving" @click="saveGroup">{{ t('review.confirm') }}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog v-model:open="groupDeleteVisible">
        <DialogContent class="review-scope sm:max-w-[420px]">
          <DialogHeader>
            <DialogTitle>{{ t('review.deleteGroup') }}</DialogTitle>
            <DialogDescription>{{ t('review.groupDeleteConfirm', { name: groupDeleteTarget?.name || '' }) }}</DialogDescription>
          </DialogHeader>
          <label class="flex items-center gap-2 text-sm">
            <Checkbox v-model="groupDeleteCards" />{{ t('review.groupDeleteWithCards') }}
          </label>
          <DialogFooter>
            <Button variant="outline" @click="groupDeleteVisible = false">{{ t('review.cancel') }}</Button>
            <Button variant="destructive" @click="doDeleteGroup">{{ t('review.confirm') }}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog v-model:open="rebindVisible">
        <DialogContent class="review-scope sm:max-w-[460px]">
          <DialogHeader>
            <DialogTitle>{{ t('review.rebindSource') }}</DialogTitle>
          </DialogHeader>
          <div class="space-y-3">
            <div class="space-y-1">
              <label class="text-sm">{{ t('review.knowledgeId') }}</label>
              <Input v-model="rebindForm.knowledgeID" :placeholder="t('review.knowledgeId')" />
            </div>
            <div class="space-y-1">
              <label class="text-sm">{{ t('review.chunkId') }}</label>
              <Input v-model="rebindForm.chunkID" :placeholder="t('review.chunkId')" />
            </div>
            <div class="space-y-1">
              <label class="text-sm">{{ t('review.excerpt') }}</label>
              <Textarea v-model="rebindForm.excerpt" :rows="4" />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" @click="rebindVisible = false">{{ t('review.cancel') }}</Button>
            <Button :disabled="rebindSaving" @click="saveRebind">{{ t('review.confirm') }}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog v-model:open="generateVisible">
        <DialogContent class="review-scope max-h-[86vh] overflow-y-auto sm:max-w-[760px]">
          <DialogHeader>
            <DialogTitle>{{ t('review.generateTitle') }}</DialogTitle>
          </DialogHeader>
          <div class="space-y-4">
            <div class="grid gap-3 sm:grid-cols-3">
              <div class="space-y-1">
                <label class="text-sm">{{ t('review.knowledgeBase') }}</label>
                <Select v-model="generationKB">
                  <SelectTrigger class="w-full"><SelectValue :placeholder="t('review.selectKnowledgeBase')" /></SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="option in kbOptions" :key="option.value" :value="option.value">{{ option.label }}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div class="space-y-1">
                <label class="text-sm">{{ t('review.cardCountLabel') }}</label>
                <Select v-model="generationCount">
                  <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="option in countOptions" :key="option.value" :value="option.value">{{ option.label }}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div class="space-y-1">
                <label class="text-sm">{{ t('review.groupName') }}</label>
                <Select v-model="generationGroup">
                  <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="option in groupOptions" :key="option.value" :value="option.value">{{ option.label }}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <Button :disabled="!generationKB || generating" @click="generatePreview">
              <Sparkles />{{ t('review.generatePreview') }}
            </Button>
            <div v-if="generationPreview?.cards?.length" class="space-y-2">
              <div v-for="(card, index) in generationPreview.cards" :key="index" class="flex items-start gap-2 rounded-md border p-3">
                <Checkbox
                  :model-value="selectedGenerated.includes(index)"
                  @update:model-value="(value) => toggleGenerated(index, Boolean(value))"
                />
                <div class="min-w-0 flex-1">
                  <div class="text-sm font-medium">{{ card.question }}</div>
                  <div class="text-muted-foreground mt-1 text-sm">{{ card.answer }}</div>
                  <div class="mt-1 flex flex-wrap items-center gap-2 text-xs">
                    <Badge v-if="card.duplicate" variant="destructive">{{ t('review.duplicate') }}</Badge>
                    <span class="text-muted-foreground">{{ card.source_title }}</span>
                  </div>
                </div>
              </div>
            </div>
            <p v-else-if="generationPreview" class="text-muted-foreground text-sm">{{ t('review.noGeneratedCards') }}</p>
          </div>
          <DialogFooter>
            <Button variant="outline" @click="generateVisible = false">{{ t('review.cancel') }}</Button>
            <Button :disabled="!selectedGenerated.length || importing" @click="importSelectedGenerated">
              {{ t('review.importSelected') }} ({{ selectedGenerated.length }})
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog v-model:open="confirmVisible">
        <DialogContent class="review-scope sm:max-w-[420px]">
          <DialogHeader>
            <DialogTitle>{{ confirmTitle }}</DialogTitle>
            <DialogDescription>{{ confirmMessage }}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" @click="confirmVisible = false">{{ t('review.cancel') }}</Button>
            <Button :variant="confirmDanger ? 'destructive' : 'default'" @click="runConfirm">{{ confirmText }}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Sheet v-model:open="performanceVisible">
        <SheetContent side="right" class="review-scope flex w-full flex-col gap-0 overflow-hidden p-0 sm:!max-w-[520px]">
          <SheetHeader class="border-border border-b px-5 py-4 pr-12">
            <SheetTitle class="text-base">{{ t('review.performanceTitle') }}</SheetTitle>
          </SheetHeader>
          <div v-if="performanceLoading" class="space-y-3 p-5">
            <Skeleton class="h-8 w-3/4" />
            <Skeleton class="h-24 w-full" />
          </div>
          <div v-else-if="performance" class="flex-1 overflow-y-auto">
            <div class="space-y-5 p-5">
              <div class="space-y-1.5">
                <p class="text-[15px] leading-6 font-medium">{{ performance.card.question }}</p>
                <div v-if="performance.warning !== 'none'" class="flex flex-wrap items-center gap-2 text-xs">
                  <Badge variant="destructive">{{ t('review.warningBadge') }}</Badge>
                  <span class="text-destructive">{{ performance.warning_reason }}</span>
                </div>
              </div>

              <div class="border-border bg-card grid grid-cols-4 divide-x divide-border overflow-hidden rounded-md border text-center">
                <div class="px-2 py-3">
                  <div class="text-lg leading-6 font-semibold tabular-nums">{{ performance.counts.remembered }}</div>
                  <div class="text-muted-foreground mt-0.5 text-xs">{{ t('review.remembered') }}</div>
                </div>
                <div class="px-2 py-3">
                  <div class="text-lg leading-6 font-semibold tabular-nums">{{ performance.counts.fuzzy }}</div>
                  <div class="text-muted-foreground mt-0.5 text-xs">{{ t('review.fuzzy') }}</div>
                </div>
                <div class="px-2 py-3">
                  <div class="text-lg leading-6 font-semibold tabular-nums">{{ performance.counts.forgot }}</div>
                  <div class="text-muted-foreground mt-0.5 text-xs">{{ t('review.forgot') }}</div>
                </div>
                <div class="px-2 py-3">
                  <div class="text-lg leading-6 font-semibold tabular-nums">{{ performance.counts.total }}</div>
                  <div class="text-muted-foreground mt-0.5 text-xs">{{ t('review.stats.reviewed') }}</div>
                </div>
              </div>

              <dl class="border-border bg-card grid grid-cols-3 gap-3 rounded-md border px-4 py-3">
                <div>
                  <dt class="text-muted-foreground text-xs">{{ t('review.stability') }}</dt>
                  <dd class="mt-0.5 text-sm font-medium tabular-nums">{{ Math.round(performance.card.stability) }} 天</dd>
                </div>
                <div>
                  <dt class="text-muted-foreground text-xs">{{ t('review.nextDue') }}</dt>
                  <dd class="mt-0.5 text-sm font-medium tabular-nums">{{ formatDue(performance.card.due_at) }}</dd>
                </div>
                <div>
                  <dt class="text-muted-foreground text-xs">{{ t('review.forgetStreak') }}</dt>
                  <dd class="mt-0.5 text-sm font-medium tabular-nums">{{ performance.forget_streak }}</dd>
                </div>
              </dl>

              <div class="space-y-2">
                <div class="flex items-center justify-between gap-2">
                  <h3 class="text-sm font-medium">{{ t('review.history') }}</h3>
                  <span class="text-muted-foreground text-xs tabular-nums">{{ performance.logs.length }}</span>
                </div>
                <div v-if="performance.logs.length" class="border-border divide-border divide-y overflow-hidden rounded-md border">
                  <div v-for="log in performance.logs" :key="log.id" class="grid grid-cols-[64px_minmax(0,1fr)_auto] items-center gap-2 px-3 py-2 text-xs">
                    <Badge :variant="log.rating === 'remembered' ? 'default' : log.rating === 'forgot' ? 'destructive' : 'outline'">
                      {{ t(`review.${log.rating}`) }}
                    </Badge>
                    <span class="text-muted-foreground truncate">{{ formatTime(log.reviewed_at) }}</span>
                    <span class="tabular-nums">{{ log.interval_days }} 天</span>
                  </div>
                </div>
                <p v-else class="text-muted-foreground py-4 text-center text-sm">{{ t('review.noHistory') }}</p>
              </div>
            </div>
          </div>
          <div class="border-border flex justify-end border-t p-4">
            <Button variant="outline" @click="performanceVisible = false">{{ t('review.close') }}</Button>
          </div>
        </SheetContent>
      </Sheet>

      <div
        v-if="toast"
        class="fixed right-4 bottom-4 z-[60] flex max-w-sm items-start gap-2 rounded-lg border p-3 text-sm shadow-lg"
        :class="toast.type === 'error' ? 'border-destructive/40 bg-destructive/10 text-destructive' : toast.type === 'warning' ? 'border-amber-400/50 bg-amber-50 text-amber-700' : 'border-primary/30 bg-primary/10 text-primary'"
      >
        <AlertTriangle v-if="toast.type === 'warning'" class="mt-0.5 size-4 shrink-0" />
        <X v-else-if="toast.type === 'error'" class="mt-0.5 size-4 shrink-0" />
        <Sparkles v-else class="mt-0.5 size-4 shrink-0" />
        <span>{{ toast.message }}</span>
      </div>
    </div>
  </div>
</template>

<style>
.review-scope,
.review-scope *,
.review-scope *::before,
.review-scope *::after {
  box-sizing: border-box;
}
</style>
