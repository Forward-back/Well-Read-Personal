<template>
  <Transition name="session-drawer">
    <section
      v-if="visible"
      class="session-drawer"
      :class="{ 'session-drawer--rail': mode === 'rail' }"
      role="dialog"
      aria-label="问答记录"
      @click.stop
    >
      <div class="session-drawer__head">
        <strong>{{ t('menu.myChats') }}</strong>
        <div class="session-drawer__head-actions">
          <button
            type="button"
            class="session-drawer__close"
            aria-label="搜索历史对话"
            title="搜索历史对话"
            @click="openSearch"
          >
            <t-icon name="search" />
          </button>
          <button
            v-if="mode !== 'rail'"
            type="button"
            class="session-drawer__close"
            aria-label="关闭"
            title="关闭"
            @click="close"
          >
            <t-icon name="close" />
          </button>
        </div>
      </div>

      <button type="button" class="session-drawer__new" @click="newChat">
        <t-icon name="add" />
        <span>{{ t('menu.newChat') }}</span>
      </button>

      <div class="session-drawer__list">
        <div v-if="loading" class="session-drawer__state">
          <t-loading size="small" />
          <span>加载中…</span>
        </div>
        <button v-else-if="!sessions.length" type="button" class="session-drawer__empty" @click="newChat">
          {{ t('menu.noSessions') }}
          <span>新建对话</span>
        </button>

        <template v-else>
          <div v-for="row in sessions" :key="row.id" class="session-row" :class="{ 'is-active': isActiveSession(row) }">
            <button type="button" class="session-row__main" @click="openSession(row)">
              <t-icon v-if="row.is_pinned" name="pin" class="session-row__pin" />
              <span class="session-row__title">{{ row.title }}</span>
            </button>
            <button
              type="button"
              class="session-row__more"
              aria-label="更多操作"
              @click.stop="toggleMenu(row.id)"
            >
              <t-icon name="ellipsis" />
            </button>

            <form
              v-if="editingId === row.id"
              class="session-row__edit"
              @submit.prevent="submitRename(row)"
              @click.stop
            >
              <input
                ref="renameInput"
                v-model="editingTitle"
                maxlength="80"
                @keydown.esc.prevent="cancelRename"
                @blur="submitRename(row)"
              />
            </form>

            <div v-if="activeMenuId === row.id" class="session-row__menu" @click.stop>
              <button type="button" @click="startRename(row)">
                <t-icon name="edit" /> 重命名
              </button>
              <button type="button" @click="togglePin(row)">
                <t-icon :name="row.is_pinned ? 'pin-filled' : 'pin'" />
                {{ row.is_pinned ? t('menu.unpin') : t('menu.pin') }}
              </button>
              <button type="button" @click="confirmClear(row)">
                <t-icon name="clear" /> {{ t('menu.clearMessages') }}
              </button>
              <button type="button" class="is-danger" @click="confirmDelete(row)">
                <t-icon name="delete" /> {{ t('upload.deleteRecord') }}
              </button>
            </div>
          </div>

          <button v-if="sessions.length < total" type="button" class="session-drawer__more" @click="load(false)">
            加载更多
          </button>
        </template>
      </div>
    </section>
  </Transition>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import { getSessionsList } from '@/api/chat'
import { useChatLayoutStore } from '@/stores/chatLayout'
import {
  SESSION_MUTATION_EVENT,
  clearSession,
  removeSession,
  renameSession,
  setSessionPinned,
  type SessionMutationDetail,
} from '@/components/sessionMutations'

const props = defineProps<{ visible: boolean; mode?: 'drawer' | 'rail' }>()
const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
}>()

type SessionRow = {
  id: string
  title: string
  is_pinned: boolean
  updated_at?: string
  path: string
}

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const chatLayout = useChatLayoutStore()
const visible = computed({
  get: () => props.visible,
  set: value => emit('update:visible', value),
})

const sessions = ref<SessionRow[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)
const activeMenuId = ref('')
const editingId = ref('')
const editingTitle = ref('')
const renameInput = ref<HTMLInputElement | null>(null)
const PAGE_SIZE = 30

function mapRow(item: any): SessionRow {
  return {
    id: String(item.id),
    title: item.title || t('menu.newSession'),
    is_pinned: !!item.is_pinned,
    updated_at: item.updated_at || item.created_at || '',
    path: item.reading_run_id
      ? `/platform/reading?run_id=${encodeURIComponent(item.reading_run_id)}`
      : `/platform/chat/${item.id}`,
  }
}

function sortRows(rows: SessionRow[]) {
  return [...rows].sort((a, b) => {
    if (a.is_pinned !== b.is_pinned) return a.is_pinned ? -1 : 1
    return String(b.updated_at || '').localeCompare(String(a.updated_at || ''))
  })
}

async function load(reset = true) {
  if (!reset && loading.value) return
  loading.value = true
  activeMenuId.value = ''
  try {
    if (reset) {
      page.value = 1
      sessions.value = []
    }
    const nextPage = reset ? 1 : page.value + 1
    const res: any = await getSessionsList(nextPage, PAGE_SIZE)
    const rows = (res?.data || []).map(mapRow)
    sessions.value = sortRows(reset ? rows : [...sessions.value, ...rows])
    page.value = nextPage
    total.value = Number(res?.total ?? sessions.value.length)
  } catch {
    if (reset) {
      sessions.value = []
      total.value = 0
    }
    MessagePlugin.error('加载会话失败')
  } finally {
    loading.value = false
  }
}

watch(() => props.visible, open => {
  if (open) void load(true)
}, { immediate: true })

function close() {
  visible.value = false
  activeMenuId.value = ''
  editingId.value = ''
}

function openSearch() {
  if (props.mode !== 'rail') close()
  chatLayout.openSearch()
}

function newChat() {
  if (props.mode !== 'rail') close()
  router.push('/platform/creatChat')
}

function openSession(row: SessionRow) {
  if (props.mode !== 'rail') close()
  router.push(row.path)
}

function toggleMenu(id: string) {
  activeMenuId.value = activeMenuId.value === id ? '' : id
}

function isActiveSession(row: SessionRow) {
  return route.name === 'chat' && String(route.params.chatid || '') === row.id
}

function startRename(row: SessionRow) {
  activeMenuId.value = ''
  editingId.value = row.id
  editingTitle.value = row.title
  nextTick(() => {
    renameInput.value?.focus()
    renameInput.value?.select()
  })
}

function cancelRename() {
  editingId.value = ''
  editingTitle.value = ''
}

async function submitRename(row: SessionRow) {
  const title = editingTitle.value.trim()
  if (!title || title === row.title) {
    cancelRename()
    return
  }
  try {
    await renameSession(row.id, title)
    row.title = title
    MessagePlugin.success(t('menu.renameSessionSuccess'))
  } catch {
    MessagePlugin.error(t('menu.renameSessionFailed'))
  } finally {
    cancelRename()
  }
}

async function togglePin(row: SessionRow) {
  activeMenuId.value = ''
  const next = !row.is_pinned
  try {
    await setSessionPinned(row.id, next)
    row.is_pinned = next
    sessions.value = sortRows(sessions.value)
  } catch {
    MessagePlugin.error(next ? t('menu.pinFailed') : t('menu.unpinFailed'))
  }
}

function confirmClear(row: SessionRow) {
  activeMenuId.value = ''
  const dialog = DialogPlugin.confirm({
    header: t('chatHeader.clearConfirmTitle'),
    body: t('chatHeader.clearConfirmBody'),
    theme: 'warning',
    confirmBtn: '清空',
    cancelBtn: t('common.cancel'),
    onConfirm: async () => {
      try {
        await clearSession(row.id)
        MessagePlugin.success(t('menu.clearMessagesSuccess'))
      } catch {
        MessagePlugin.error(t('menu.clearMessagesFailed'))
      } finally {
        dialog.destroy()
      }
    },
  })
}

function confirmDelete(row: SessionRow) {
  activeMenuId.value = ''
  const dialog = DialogPlugin.confirm({
    header: t('chatHeader.deleteConfirmTitle'),
    body: t('chatHeader.deleteConfirmBody'),
    theme: 'danger',
    confirmBtn: t('common.delete'),
    cancelBtn: t('common.cancel'),
    onConfirm: async () => {
      try {
        await removeSession(row.id)
        sessions.value = sessions.value.filter(item => item.id !== row.id)
        total.value = Math.max(0, total.value - 1)
      } catch {
        MessagePlugin.error(t('chat.deleteSessionFailed'))
      } finally {
        dialog.destroy()
      }
    },
  })
}

function onSessionMutation(event: Event) {
  const detail = (event as CustomEvent<SessionMutationDetail>).detail
  if (!detail) return
  if (detail.removed) {
    sessions.value = sessions.value.filter(item => item.id !== detail.sessionId)
    total.value = Math.max(0, total.value - 1)
    return
  }
  if (detail.patch) {
    sessions.value = sessions.value.map(item =>
      item.id === detail.sessionId
        ? {
            ...item,
            title: detail.patch?.title ?? item.title,
            is_pinned: detail.patch?.is_pinned ?? item.is_pinned,
          }
        : item,
    )
  }
}

function handleDocumentClick() {
  activeMenuId.value = ''
}

onMounted(() => {
  window.addEventListener(SESSION_MUTATION_EVENT, onSessionMutation)
  document.addEventListener('click', handleDocumentClick)
})

onUnmounted(() => {
  window.removeEventListener(SESSION_MUTATION_EVENT, onSessionMutation)
  document.removeEventListener('click', handleDocumentClick)
})
</script>

<style lang="less">
.session-drawer {
  position: fixed;
  top: 66px;
  bottom: 8px;
  left: 116px;
  z-index: 60;
  display: flex;
  flex-direction: column;
  width: 300px;
  padding: 14px;
  gap: 10px;
  overflow: hidden;
  border: 1px solid var(--w-line);
  border-radius: 12px;
  background: var(--w-cream);
  box-shadow: var(--w-shadow);
  color: var(--w-body);
}

.session-drawer--rail {
  position: relative;
  top: auto;
  bottom: auto;
  left: auto;
  z-index: auto;
  width: 100%;
  height: 100%;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.session-drawer__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--w-ink);
  font-size: 14px;
  font-weight: 700;
}

.session-drawer__head-actions {
  display: flex;
  align-items: center;
  gap: 2px;
}

.session-drawer__close {
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--w-muted);
  cursor: pointer;

  &:hover {
    background: var(--w-accent);
    color: var(--w-ink);
  }
}

.session-drawer__new {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 34px;
  border: 0;
  border-radius: 6px;
  background: var(--w-green-soft);
  color: var(--w-green-dark);
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: filter 0.15s ease;

  &:hover {
    filter: brightness(0.97);
  }
}

.session-drawer__list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-right: 2px;
}

.session-drawer__state,
.session-drawer__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 120px;
  color: var(--w-muted);
  font: inherit;
  font-size: 12px;
}

.session-drawer__empty {
  border: 1px dashed var(--w-line);
  border-radius: 8px;
  background: transparent;
  cursor: pointer;

  span {
    color: var(--w-green-dark);
    font-weight: 600;
  }
}

.session-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: 2px;
  border-radius: 6px;

  &:hover {
    background: var(--w-accent);
  }

  &.is-active {
    background: transparent;
  }

  &.is-active .session-row__title {
    color: var(--w-green-dark);
    font-weight: 600;
  }
}

.session-row__main {
  display: flex;
  align-items: center;
  gap: 5px;
  flex: 1;
  min-width: 0;
  padding: 8px 6px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--w-body);
  font: inherit;
  font-size: 12px;
  font-weight: 500;
  line-height: 18px;
  text-align: left;
  cursor: pointer;
}

.session-row__pin {
  flex: none;
  color: var(--w-green-dark);
  font-size: 13px;
}

.session-row__title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-row__more {
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: var(--w-muted);
  opacity: 0;
  cursor: pointer;

  .session-row:hover &,
  &:focus-visible {
    opacity: 1;
  }

  &:hover {
    background: var(--w-cream);
    color: var(--w-ink);
  }
}

.session-row__edit {
  position: absolute;
  inset: 2px 28px 2px 2px;
  display: flex;
  align-items: center;
  padding: 0 6px;
  border: 1px solid var(--w-green-dark);
  border-radius: 5px;
  background: var(--w-cream);

  input {
    width: 100%;
    border: 0;
    outline: none;
    background: transparent;
    color: var(--w-ink);
    font: inherit;
    font-size: 12px;
  }
}

.session-row__menu {
  position: absolute;
  top: calc(100% - 2px);
  right: 4px;
  z-index: 5;
  display: flex;
  flex-direction: column;
  min-width: 132px;
  padding: 4px;
  border: 1px solid var(--w-line);
  border-radius: 8px;
  background: var(--w-cream);
  box-shadow: 0 10px 24px -10px rgba(31, 41, 26, 0.45);

  button {
    display: flex;
    align-items: center;
    gap: 7px;
    padding: 7px 8px;
    border: 0;
    border-radius: 5px;
    background: transparent;
    color: var(--w-body);
    font: inherit;
    font-size: 12px;
    text-align: left;
    cursor: pointer;

    &:hover {
      background: var(--w-accent);
      color: var(--w-ink);
    }

    &.is-danger {
      color: #c9353f;
    }
  }
}

.session-drawer__more {
  margin-top: 4px;
  padding: 7px;
  border: 1px solid var(--w-line);
  border-radius: 6px;
  background: transparent;
  color: var(--w-muted);
  font: inherit;
  font-size: 12px;
  cursor: pointer;

  &:hover {
    border-color: var(--w-green-dark);
    color: var(--w-green-dark);
  }
}

.session-drawer-enter-active,
.session-drawer-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.session-drawer-enter-from,
.session-drawer-leave-to {
  opacity: 0;
  transform: translateX(-8px);
}

@media (max-width: 900px) {
  .session-drawer {
    top: 56px;
    right: 8px;
    bottom: 8px;
    left: 8px;
    width: auto;
  }
}
</style>
