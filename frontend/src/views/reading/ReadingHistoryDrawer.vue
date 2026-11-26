<template>
  <Transition name="rv-history">
    <div v-if="visible" class="rv-history__overlay" @click.self="close">
      <aside class="rv-history" role="dialog" aria-modal="true" :aria-label="$t('reading.historyTitle')" :style="{ width: historyWidth + 'px' }">
        <div
          class="rv-history__resizer"
          role="separator"
          aria-orientation="vertical"
          tabindex="0"
          @pointerdown="startHistoryResize"
          @keydown="onHistoryResizeKeydown"
        />
        <header class="rv-history__head">
          <div class="rv-history__heading">
            <span class="rv-history__title">{{ $t('reading.historyTitle') }}</span>
            <span v-if="!loading && !error" class="rv-history__count">{{ $t('reading.historyCount', { count: items.length }) }}</span>
          </div>
          <button type="button" class="rv-history__close" :aria-label="$t('shell.close')" @click="close">×</button>
        </header>

        <div class="rv-history__body">
          <div v-if="loading" class="rv-history__state"><t-loading size="small" /></div>
          <div v-else-if="error" class="rv-history__state is-error">
            <span>{{ $t('reading.historyLoadFailed') }}</span>
            <button type="button" @click="load">{{ $t('reading.resume') }}</button>
          </div>
          <div v-else-if="!items.length" class="rv-history__state">{{ $t('reading.historyEmpty') }}</div>

          <template v-else>
            <article
              v-for="item in items"
              :key="item.id"
              class="rv-history-card"
              :class="{ 'is-expanded': expandedId === item.id }"
            >
              <button type="button" class="rv-history-card__head" @click="toggle(item.id)">
                <div class="rv-history-card__main">
                  <div class="rv-history-card__title-row">
                    <span class="rv-history-card__name" :title="displayTitle(item)">{{ displayTitle(item) }}</span>
                    <span class="rv-history-card__status" :class="`is-${item.status}`">{{ statusLabel(item.status) }}</span>
                  </div>
                  <div class="rv-history-card__meta">
                    <span>{{ formatDate(item.updated_at || item.created_at) }}</span>
                    <span>{{ $t('reading.historyProgress', { read: item.read_sections, total: item.total_sections }) }}</span>
                    <span>{{ item.documents.length }} · {{ $t('reading.historyDocuments') }}</span>
                  </div>
                </div>
                <span class="rv-history-card__chevron">{{ expandedId === item.id ? '⌃' : '⌄' }}</span>
              </button>

              <div class="rv-history-card__actions">
                <button type="button" @click.stop="startRename(item)">{{ $t('reading.historyRename') }}</button>
              </div>

              <form v-if="editingId === item.id" class="rv-history-card__rename" @submit.prevent="saveRename(item)" @click.stop>
                <input
                  ref="renameInput"
                  v-model="editingTitle"
                  maxlength="120"
                  :placeholder="$t('reading.historyRenamePlaceholder')"
                  @keydown.esc.prevent="cancelRename"
                />
                <button type="submit" :disabled="renaming">{{ $t('reading.historyRenameSave') }}</button>
                <button type="button" @click="cancelRename">{{ $t('reading.historyRenameCancel') }}</button>
              </form>

              <div v-if="expandedId === item.id" class="rv-history-card__details">
                <div class="rv-history-card__detail">
                  <span class="rv-history-card__label">{{ $t('reading.historyDocuments') }}</span>
                  <div class="rv-history-card__docs">
                    <span v-for="doc in item.documents" :key="doc.id" class="rv-history-card__doc" :title="doc.title">
                      {{ doc.title || doc.file_name || $t('reading.historyDeletedDoc') }}
                    </span>
                    <span v-if="!item.documents.length" class="rv-history-card__faint">{{ $t('reading.historyDeletedDoc') }}</span>
                  </div>
                </div>
                <div class="rv-history-card__detail">
                  <span class="rv-history-card__label">{{ $t('reading.historyGoal') }}</span>
                  <span class="rv-history-card__value">{{ item.goal || '—' }}</span>
                </div>
                <div class="rv-history-card__detail">
                  <span class="rv-history-card__label">{{ $t('reading.historyDepth') }}</span>
                  <span class="rv-history-card__value">{{ depthLabel(item.depth) }}</span>
                </div>
                <button type="button" class="rv-history-card__open" @click.stop="emit('open', item)">
                  {{ $t('reading.historyOpen') }}
                </button>
              </div>
            </article>
          </template>
        </div>
      </aside>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import { listReadingHistory, type ReadingHistoryItem } from '@/api/reading'
import { renameSession } from '@/components/sessionMutations'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
  (e: 'open', item: ReadingHistoryItem): void
}>()

const { t } = useI18n()
const loading = ref(false)
const error = ref(false)
const items = ref<ReadingHistoryItem[]>([])
const expandedId = ref('')
const editingId = ref('')
const editingTitle = ref('')
const renaming = ref(false)
const renameInput = ref<HTMLInputElement | null>(null)
const historyWidth = ref(loadStoredHistoryWidth())
let historyResizing = false
let historyStartX = 0
let historyStartWidth = 0
let suppressOverlayClose = false

function loadStoredHistoryWidth() {
  const raw = Number(localStorage.getItem('gaze_reading_history_width') || '')
  return Number.isFinite(raw) && raw >= 320 ? raw : 440
}

function clampHistoryWidth(value: number) {
  const max = Math.max(360, Math.min(760, Math.round(window.innerWidth * 0.8)))
  const min = Math.min(320, max)
  return Math.min(Math.max(min, Math.round(value)), max)
}

const startHistoryResize = (event: PointerEvent) => {
  historyResizing = true
  historyStartX = event.clientX
  historyStartWidth = historyWidth.value
  document.body.style.userSelect = 'none'
  document.body.style.cursor = 'col-resize'
  document.addEventListener('pointermove', onHistoryResize)
  document.addEventListener('pointerup', endHistoryResize, { once: true })
  event.preventDefault()
}

function onHistoryResize(event: PointerEvent) {
  if (!historyResizing) return
  historyWidth.value = clampHistoryWidth(historyStartWidth + historyStartX - event.clientX)
}

function endHistoryResize() {
  if (!historyResizing) return
  historyResizing = false
  historyWidth.value = clampHistoryWidth(historyWidth.value)
  localStorage.setItem('gaze_reading_history_width', String(historyWidth.value))
  document.body.style.userSelect = ''
  document.body.style.cursor = ''
  document.removeEventListener('pointermove', onHistoryResize)
  suppressOverlayClose = true
  window.setTimeout(() => {
    suppressOverlayClose = false
  }, 0)
}

function onHistoryResizeKeydown(event: KeyboardEvent) {
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
  event.preventDefault()
  const delta = event.key === 'ArrowLeft' ? 24 : -24
  historyWidth.value = clampHistoryWidth(historyWidth.value + delta)
  localStorage.setItem('gaze_reading_history_width', String(historyWidth.value))
}

const close = () => {
  if (suppressOverlayClose) {
    suppressOverlayClose = false
    return
  }
  emit('update:visible', false)
}

const displayTitle = (item: ReadingHistoryItem) => {
  if (item.title?.trim()) return item.title.trim()
  const first = item.documents.find((doc) => doc.title || doc.file_name)
  return first?.title || first?.file_name || t('reading.historyUntitled')
}

const statusLabel = (status: string) => {
  const key = `reading.historyStatus.${status}`
  const value = t(key)
  return value === key ? status : value
}

const depthLabel = (depth?: string) => {
  if (!depth) return '—'
  const key = `reading.depth.${depth}`
  const value = t(key)
  return value === key ? depth : value
}

const formatDate = (value?: string) => {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleDateString()
}

const load = async () => {
  loading.value = true
  error.value = false
  try {
    const response = await listReadingHistory()
    items.value = Array.isArray(response.data) ? response.data : []
  } catch {
    error.value = true
    items.value = []
  } finally {
    loading.value = false
  }
}

const toggle = (id: string) => {
  if (editingId.value) return
  expandedId.value = expandedId.value === id ? '' : id
}

const startRename = (item: ReadingHistoryItem) => {
  editingId.value = item.id
  editingTitle.value = displayTitle(item)
  nextTick(() => {
    renameInput.value?.focus()
    renameInput.value?.select()
  })
}

const cancelRename = () => {
  editingId.value = ''
  editingTitle.value = ''
}

const saveRename = async (item: ReadingHistoryItem) => {
  const title = editingTitle.value.trim()
  if (!title || !item.session_id) {
    MessagePlugin.warning(t('reading.historyRenameFailed'))
    return
  }
  renaming.value = true
  try {
    const result = await renameSession(item.session_id, title)
    item.title = result?.title || title
    cancelRename()
  } catch {
    MessagePlugin.error(t('reading.historyRenameFailed'))
  } finally {
    renaming.value = false
  }
}

onBeforeUnmount(() => {
  document.removeEventListener('pointermove', onHistoryResize)
  document.body.style.userSelect = ''
  document.body.style.cursor = ''
})

watch(
  () => props.visible,
  (visible) => {
    if (!visible) {
      cancelRename()
      return
    }
    expandedId.value = ''
    void load()
  },
)
</script>

<style scoped lang="less">
.rv-history__overlay {
  position: absolute;
  inset: 0;
  z-index: 80;
  display: flex;
  justify-content: flex-end;
  background: rgba(15, 15, 15, 0.16);
}

.rv-history {
  position: relative;
  display: flex;
  flex-direction: column;
  max-width: 92%;
  height: 100%;
  background: var(--rv-bg);
  border-left: 1px solid var(--rv-hair);
  box-shadow: -8px 0 24px rgba(15, 15, 15, 0.08);
}

.rv-history__resizer {
  position: absolute;
  top: 0;
  bottom: 0;
  left: -4px;
  z-index: 2;
  width: 8px;
  cursor: col-resize;
}

.rv-history__resizer:hover,
.rv-history__resizer:focus-visible {
  background: rgba(7, 192, 95, 0.18);
  outline: none;
}

.rv-history__head {
  display: flex;
  flex: none;
  align-items: center;
  gap: 12px;
  height: 52px;
  padding: 0 16px;
  border-bottom: 1px solid var(--rv-hair);
}

.rv-history__heading {
  display: flex;
  flex: 1;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}

.rv-history__title {
  font-size: 14px;
  font-weight: 600;
  white-space: nowrap;
}

.rv-history__count {
  overflow: hidden;
  font-size: 12px;
  color: var(--rv-text-3);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rv-history__close {
  flex: none;
  width: 28px;
  height: 28px;
  font-size: 20px;
  line-height: 1;
  color: var(--rv-text-3);
}

.rv-history__close:hover {
  color: var(--rv-text);
  background: var(--rv-hover);
}

.rv-history__body {
  flex: 1;
  min-height: 0;
  padding: 12px;
  overflow-y: auto;
  scrollbar-width: none;
}

.rv-history__body::-webkit-scrollbar {
  display: none;
}

.rv-history__state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 160px;
  font-size: 13px;
  color: var(--rv-text-3);
  text-align: center;
}

.rv-history__state.is-error button {
  color: var(--rv-green-deep);
  text-decoration: underline;
}

.rv-history-card {
  margin-bottom: 10px;
  border: 1px solid var(--rv-hair);
  border-radius: var(--rv-r-pop);
  background: var(--rv-bg);
}

.rv-history-card.is-expanded {
  border-color: #c9dfd1;
  background: #fcfdfb;
}

.rv-history-card__head {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  width: 100%;
  padding: 12px 14px 8px;
  text-align: left;
}

.rv-history-card__main {
  flex: 1;
  min-width: 0;
}

.rv-history-card__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.rv-history-card__name {
  overflow: hidden;
  font-size: 14px;
  font-weight: 600;
  color: var(--rv-text);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rv-history-card__status {
  flex: none;
  padding: 1px 7px;
  font-size: 11px;
  line-height: 18px;
  border-radius: 9px;
  white-space: nowrap;
}

.rv-history-card__status.is-finished {
  color: #3e6b4f;
  background: #e9f3ec;
}

.rv-history-card__status.is-stopped {
  color: #8a6a3b;
  background: #f5efe2;
}

.rv-history-card__status.is-abandoned {
  color: #8a5f58;
  background: #f5e9e6;
}

.rv-history-card__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 12px;
  margin-top: 5px;
  font-size: 12px;
  color: var(--rv-text-3);
}

.rv-history-card__chevron {
  flex: none;
  margin-top: 2px;
  color: var(--rv-text-3);
}

.rv-history-card__actions {
  display: flex;
  justify-content: flex-end;
  padding: 0 14px 10px;
}

.rv-history-card__actions button {
  font-size: 12px;
  color: var(--rv-text-2);
}

.rv-history-card__actions button:hover {
  color: var(--rv-green-deep);
}

.rv-history-card__rename {
  display: flex;
  gap: 6px;
  padding: 0 14px 10px;
}

.rv-history-card__rename input {
  flex: 1;
  min-width: 0;
  height: 28px;
  padding: 0 8px;
  border: 1px solid var(--rv-hair);
  border-radius: var(--rv-r);
  outline: none;
  background: #fff;
  color: var(--rv-text);
  font: inherit;
  font-size: 13px;
}

.rv-history-card__rename button {
  flex: none;
  padding: 0 8px;
  font-size: 12px;
  color: var(--rv-green-deep);
}

.rv-history-card__details {
  padding: 10px 14px 14px;
  border-top: 1px dashed var(--rv-hair);
}

.rv-history-card__detail {
  display: flex;
  gap: 10px;
  margin-bottom: 8px;
  font-size: 12.5px;
  line-height: 1.6;
}

.rv-history-card__label {
  flex: none;
  width: 58px;
  color: var(--rv-text-3);
}

.rv-history-card__value,
.rv-history-card__docs {
  min-width: 0;
  color: var(--rv-text-2);
}

.rv-history-card__docs {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.rv-history-card__doc {
  max-width: 100%;
  overflow: hidden;
  padding: 1px 7px;
  border: 1px solid var(--rv-hair);
  border-radius: 9px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rv-history-card__faint {
  color: var(--rv-text-3);
}

.rv-history-card__open {
  margin-top: 4px;
  padding: 5px 10px;
  border: 1px solid #c9dfd1;
  border-radius: var(--rv-r);
  color: var(--rv-green-deep);
  background: var(--rv-green-bg);
  font-size: 12px;
}

.rv-history-enter-active,
.rv-history-leave-active {
  transition: opacity 0.18s ease;
}

.rv-history-enter-active .rv-history,
.rv-history-leave-active .rv-history {
  transition: transform 0.18s ease;
}

.rv-history-enter-from,
.rv-history-leave-to {
  opacity: 0;
}

.rv-history-enter-from .rv-history,
.rv-history-leave-to .rv-history {
  transform: translateX(16px);
}
</style>
