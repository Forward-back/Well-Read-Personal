<template>
  <div
    class="agent-page"
    :class="{
      'is-single-pane': editorCollapsed || chatCollapsed,
      'is-editor-collapsed': editorCollapsed,
      'is-chat-collapsed': chatCollapsed,
      'is-resizing': resizing,
    }"
  >
    <PageTopbar :title="$t('agentWorkspace.title')" :meta="$t('agentWorkspace.meta')">
      <template #actions>
        <t-tooltip :content="$t('agentWorkspace.newTask')" placement="bottom">
          <button type="button" class="agent-bar-btn is-primary" :aria-label="$t('agentWorkspace.newTask')" @click="openNewTaskDialog">
            <AddIcon />
          </button>
        </t-tooltip>
        <t-tooltip :content="$t('agentWorkspace.history')" placement="bottom">
          <button type="button" class="agent-bar-btn" :aria-label="$t('agentWorkspace.history')" @click="historyDialogVisible = true">
            <HistoryIcon />
          </button>
        </t-tooltip>
        <t-tooltip :content="$t('agentWorkspace.versions')" placement="bottom">
          <button type="button" class="agent-bar-btn" :aria-label="$t('agentWorkspace.versions')" @click="openVersionsDialog">
            <GitBranchIcon />
          </button>
        </t-tooltip>
        <span class="agent-bar-sep" aria-hidden="true" />
        <t-tooltip :content="editorCollapsed ? $t('agentWorkspace.expandEditor') : $t('agentWorkspace.collapseEditor')" placement="bottom">
          <button
            type="button"
            class="agent-bar-btn"
            :class="{ 'is-on': !editorCollapsed }"
            :aria-pressed="!editorCollapsed"
            :aria-label="$t('agentWorkspace.editorToggle')"
            @click="editorCollapsed = !editorCollapsed"
          >
            <LayoutIcon />
          </button>
        </t-tooltip>
        <t-tooltip :content="chatCollapsed ? $t('agentWorkspace.expand') : $t('agentWorkspace.collapse')" placement="bottom">
          <button
            type="button"
            class="agent-bar-btn"
            :class="{ 'is-on': !chatCollapsed }"
            :aria-pressed="!chatCollapsed"
            :aria-label="$t('agentWorkspace.chatToggle')"
            @click="chatCollapsed = !chatCollapsed"
          >
            <ChatIcon />
          </button>
        </t-tooltip>
      </template>
    </PageTopbar>

    <div ref="workbenchRef" class="agent-workbench" :style="workbenchStyle">
      <aside class="agent-pane agent-tree" :aria-label="$t('agentWorkspace.treeAria')">
        <div class="agent-tree__head">
          <span>{{ scopeTitle }}</span>
          <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.newTask')" @click="openNewTaskDialog">
            <AddIcon />
          </button>
        </div>

        <div v-if="!activeSession" class="agent-tree__empty">
          <p>{{ $t('agentWorkspace.sessionsEmpty') }}</p>
          <button type="button" class="agent-btn is-primary" @click="openNewTaskDialog">
            {{ $t('agentWorkspace.newTask') }}
          </button>
        </div>
        <template v-else>
          <div v-for="group in docGroups" :key="group.path" class="agent-tree__group">
            <button type="button" class="agent-tree__row is-folder" @click="toggleFolder(group.path)">
              <ChevronDownIcon v-if="expandedFolders.includes(group.path) || group.path === ''" class="agent-tree__caret" />
              <ChevronRightIcon v-else class="agent-tree__caret" />
              <FolderIcon class="agent-tree__icon" />
              <span>{{ group.label }}</span>
              <em>{{ group.docs.length }}</em>
            </button>
            <template v-if="expandedFolders.includes(group.path) || group.path === ''">
              <button
                v-for="doc in group.docs"
                :key="doc.id"
                type="button"
                class="agent-tree__row is-child"
                :class="{ 'is-active': doc.id === currentDocId }"
                @click="selectDoc(doc.id)"
              >
                <span class="agent-tree__caret" />
                <FileIcon class="agent-tree__icon" />
                <span>{{ doc.title }}</span>
                <i
                  v-if="doc.syncStatus === 'pending'"
                  class="agent-tree__dot"
                  :title="$t('agentWorkspace.syncPending')"
                />
              </button>
            </template>
          </div>
          <p v-if="activeSession && !documents.length" class="agent-hint">{{ $t('agentWorkspace.documentsEmpty') }}</p>

          <div class="agent-version-card">
            <strong>{{ $t('agentWorkspace.currentVersion') }} {{ currentVersionLabel }}</strong>
            <span>{{ currentDocTitle || '—' }}</span>
            <small>{{ syncLabel }}</small>
          </div>
        </template>
      </aside>

      <section v-show="!editorCollapsed" class="agent-pane agent-editor" :aria-label="$t('agentWorkspace.editorAria')">
        <div class="agent-tabs">
          <button type="button" class="agent-tab is-active">
            <FileIcon />
            <span>{{ currentDocTitle || $t('agentWorkspace.selectDoc') }}</span>
            <i v-if="syncStatus === 'pending'" class="agent-tab__dirty" />
          </button>
          <span class="agent-tabs__spacer" />
          <t-tooltip :content="$t('agentWorkspace.versions')" placement="bottom">
            <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.versions')" @click="openVersionsDialog">
              <GitBranchIcon />
            </button>
          </t-tooltip>
          <t-tooltip :content="$t('agentWorkspace.collapseEditor')" placement="bottom">
            <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.collapseEditor')" @click="editorCollapsed = true">
              <ChevronRightIcon />
            </button>
          </t-tooltip>
        </div>
        <div class="agent-breadcrumb">
          {{ scopeTitle }} › <b>{{ currentDocTitle || '—' }}</b> ›
          <span>{{ currentVersionLabel }} · {{ syncLabel }}</span>
        </div>
        <div v-if="!currentDocId" class="agent-editor__empty">
          <p>{{ $t('agentWorkspace.selectDoc') }}</p>
        </div>
        <div v-else class="agent-code" role="document">
          <div v-for="line in editorLines" :key="line.key" class="agent-line" :class="line.kind">
            <span class="agent-line__no">{{ line.no }}</span>
            <span class="agent-line__text">{{ line.text }}</span>
          </div>
        </div>
      </section>

      <div
        v-if="!editorCollapsed && !chatCollapsed"
        class="agent-divider"
        role="separator"
        aria-orientation="vertical"
        :aria-label="$t('agentWorkspace.resizeDivider')"
        :aria-valuemin="MIN_CHAT_WIDTH"
        :aria-valuemax="Math.round(maxChatWidth)"
        :aria-valuenow="Math.round(chatWidth)"
        tabindex="0"
        @pointerdown="startDrag"
        @keydown="onDividerKeydown"
        @dblclick="resetChatWidth"
      />

      <aside v-show="!chatCollapsed" class="agent-pane agent-chat" :aria-label="$t('agentWorkspace.chatAria')">
        <header class="agent-chat__head">
          <TraceSourceIcon class="agent-chat__icon" name="robot" :size="26" />
          <div class="agent-chat__identity">
            <strong>{{ $t('agentWorkspace.chatTitle') }}</strong>
            <span>{{ activeSession?.title || $t('agentWorkspace.sessionsEmpty') }}</span>
          </div>
          <div class="agent-chat__actions">
            <t-tooltip :content="$t('agentWorkspace.newTask')" placement="bottom">
              <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.newTask')" @click="openNewTaskDialog">
                <AddIcon />
              </button>
            </t-tooltip>
            <t-tooltip :content="$t('agentWorkspace.history')" placement="bottom">
              <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.history')" @click="historyDialogVisible = true">
                <HistoryIcon />
              </button>
            </t-tooltip>
            <t-tooltip :content="$t('agentWorkspace.collapse')" placement="bottom">
              <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.collapse')" @click="chatCollapsed = true">
                <ChevronRightIcon />
              </button>
            </t-tooltip>
          </div>
        </header>

        <div ref="chatBodyRef" class="agent-chat__body">
          <div v-if="!activeSession" class="agent-chat__empty">{{ $t('agentWorkspace.chatEmpty') }}</div>
          <div v-else class="agent-timeline">
            <template v-for="message in historyMessages" :key="message.id">
              <section v-if="message.role === 'user'" class="agent-turn is-user">
                <span class="agent-node is-user" aria-hidden="true" />
                <div class="agent-turn__content">
                  <div class="agent-turn__meta">{{ $t('agentWorkspace.you') }} · {{ formatTime(message.created_at) }}</div>
                  <div class="agent-user-box">{{ message.content }}</div>
                </div>
              </section>
              <template v-else>
                <section v-if="assistantEvents(message).length" class="agent-turn">
                  <span class="agent-node is-agent" aria-hidden="true" />
                  <div class="agent-turn__content">
                    <template v-for="(event, index) in assistantEvents(message)" :key="`${message.id}-ev-${index}`">
                      <details v-if="event.type === 'thinking'" class="agent-thinking">
                        <summary>{{ $t('agentWorkspace.thinking') }}</summary>
                        <pre>{{ (event as any).content }}</pre>
                      </details>
                      <div v-else-if="event.type === 'tool_call'" class="agent-tool-line">
                        <div class="agent-tool-head">
                          <strong>{{ toolLabel((event as any).tool_name) }}</strong>
                          <span
                            class="agent-state"
                            :class="{ 'is-done': !(event as any).pending && (event as any).success, 'is-warn': !(event as any).pending && !(event as any).success }"
                          >
                            {{ toolStateLabel(event) }}
                          </span>
                        </div>
                        <p v-if="(event as any).output" class="agent-tool-output">{{ String((event as any).output).slice(0, 600) }}</p>
                        <p v-if="(event as any).error" class="agent-tool-error">{{ (event as any).error }}</p>
                      </div>
                    </template>
                    <div v-if="message.content" class="agent-answer" v-html="assistantHtml(message)" />
                  </div>
                </section>
                <section v-else-if="message.content" class="agent-turn is-agent">
                  <span class="agent-node is-agent" aria-hidden="true" />
                  <div class="agent-turn__content">
                    <div class="agent-turn__meta">{{ $t('agentWorkspace.chatTitle') }}</div>
                    <div class="agent-answer" v-html="assistantHtml(message)" />
                  </div>
                </section>
                <section v-if="assistantEdits(message).length" class="agent-turn">
                  <span class="agent-node is-write" aria-hidden="true" />
                  <div class="agent-turn__content">
                    <div class="agent-edit-strip">
                      <div
                        v-for="(edit, index) in assistantEdits(message)"
                        :key="`${message.id}-edit-${index}`"
                        class="agent-edit-row"
                      >
                        <EditIcon />
                        <span class="agent-edit-row__label">
                          {{ edit.toolName === 'create_document' ? $t('agentWorkspace.createdTag') : $t('agentWorkspace.editedTag') }}
                          · {{ edit.title }} · {{ edit.versionLabel }}
                        </span>
                        <span class="agent-foot-sync">{{ syncStatusLabel(edit.engineSyncStatus) }}</span>
                        <button
                          v-if="edit.previousVersionId"
                          type="button"
                          class="agent-text-btn"
                          @click="undoEdit(edit.documentId, edit.previousVersionId)"
                        >
                          {{ $t('agentWorkspace.undoEdit') }}
                        </button>
                      </div>
                    </div>
                  </div>
                </section>
              </template>
            </template>

            <section v-if="liveTurn" class="agent-turn">
              <span class="agent-node is-agent" aria-hidden="true" />
              <div class="agent-turn__content">
                <div class="agent-turn__meta">{{ $t('agentWorkspace.streaming') }}</div>
                <details v-if="liveThinking" class="agent-thinking" open>
                  <summary>{{ $t('agentWorkspace.thinking') }}</summary>
                  <pre>{{ liveThinking }}</pre>
                </details>
                <div v-for="(event, index) in liveToolEvents" :key="`live-${index}`" class="agent-tool-line">
                  <div class="agent-tool-head">
                    <strong>{{ toolLabel(event.tool_name) }}</strong>
                    <span
                      class="agent-state"
                      :class="{ 'is-done': !event.pending && event.success, 'is-warn': !event.pending && !event.success }"
                    >
                      {{ toolStateLabel(event) }}
                    </span>
                  </div>
                  <p v-if="event.output" class="agent-tool-output">{{ event.output.slice(0, 600) }}</p>
                  <p v-if="event.error" class="agent-tool-error">{{ event.error }}</p>
                </div>
                <div v-if="liveTurn.content" class="agent-answer" v-html="liveAnswerHtml" />
              </div>
            </section>

            <section class="agent-turn">
              <span class="agent-node is-warning" aria-hidden="true" />
              <div class="agent-turn__content">
                <article class="agent-pending-box">
                  <header>{{ $t('agentWorkspace.pendingTitle') }}</header>
                  <div class="agent-pending__meta">
                    <p>{{ $t('agentWorkspace.l2Placeholder') }}</p>
                  </div>
                </article>
              </div>
            </section>
          </div>
        </div>

        <div class="agent-composer">
          <div class="agent-composer__mode">
            <span class="agent-mode-pill">{{ $t('agentWorkspace.agentMode') }}</span>
            <span class="agent-muted agent-foot-ask">{{ $t('agentWorkspace.askMode') }}</span>
          </div>
          <textarea
            v-model="draft"
            :placeholder="$t('agentWorkspace.composerPlaceholder')"
            :disabled="!activeSession"
            @keydown.enter.exact.prevent="send"
          />
          <div class="agent-composer__foot">
            <span class="agent-foot-scope">{{ $t('agentWorkspace.scopeLabel') }}：{{ scopeHint }}</span>
            <span class="agent-composer__spacer" />
            <div class="agent-foot-model">
              <ModelSelector
                model-type="KnowledgeQA"
                :selected-model-id="activeSession?.model_id || ''"
                :all-models="models"
                :allow-add="false"
                @update:selected-model-id="changeSessionModel"
              />
            </div>
            <button type="button" class="agent-send" :disabled="!canSend" @click="send">
              {{ streaming ? $t('agentWorkspace.streaming') : $t('agentWorkspace.send') }}
            </button>
          </div>
        </div>
      </aside>

      <div v-if="editorCollapsed && chatCollapsed" class="agent-restore">
        <strong>{{ $t('agentWorkspace.restoreTitle') }}</strong>
        <p>{{ $t('agentWorkspace.restoreHint') }}</p>
        <div class="agent-restore__actions">
          <button type="button" class="agent-btn is-primary" @click="editorCollapsed = false">
            <LayoutIcon />
            {{ $t('agentWorkspace.expandEditor') }}
          </button>
          <button type="button" class="agent-btn" @click="chatCollapsed = false">
            <ChatIcon />
            {{ $t('agentWorkspace.expand') }}
          </button>
        </div>
      </div>
    </div>

    <t-dialog
      v-model:visible="newSessionDialogVisible"
      :header="$t('agentWorkspace.newTask')"
      width="520px"
      :footer="false"
    >
      <div class="agent-dialog">
        <label class="agent-field">
          <span>{{ $t('agentWorkspace.sessionTitleLabel') }}</span>
          <input v-model="newSessionForm.title" type="text" :placeholder="$t('agentWorkspace.sessionTitlePlaceholder')" />
        </label>
        <label class="agent-field">
          <span>{{ $t('agentWorkspace.sessionKbLabel') }}</span>
          <select v-model="newSessionForm.kbId">
            <option v-for="kb in knowledgeBases" :key="kb.id" :value="kb.id">{{ kb.name }}</option>
          </select>
        </label>
        <label class="agent-field">
          <span>{{ $t('agentWorkspace.sessionModelLabel') }}</span>
          <ModelSelector
            v-model:selected-model-id="newSessionForm.modelId"
            model-type="KnowledgeQA"
            :all-models="models"
            :allow-add="false"
          />
        </label>
        <div class="agent-dialog__actions">
          <button type="button" class="agent-btn" @click="newSessionDialogVisible = false">{{ $t('common.cancel') }}</button>
          <button type="button" class="agent-btn is-primary" :disabled="newSessionSaving" @click="submitNewSession">
            {{ $t('agentWorkspace.create') }}
          </button>
        </div>
      </div>
    </t-dialog>

    <t-dialog
      v-model:visible="historyDialogVisible"
      :header="$t('agentWorkspace.history')"
      width="560px"
      :footer="false"
    >
      <div class="agent-dialog">
        <p v-if="sessionsLoading" class="agent-hint">{{ $t('agentWorkspace.loading') }}</p>
        <p v-else-if="!sessions.length" class="agent-hint">{{ $t('agentWorkspace.sessionsEmpty') }}</p>
        <div v-else class="agent-session-list">
          <div
            v-for="session in sessions"
            :key="session.id"
            class="agent-session-row"
            :class="{ 'is-active': session.id === activeSession?.id }"
          >
            <button type="button" class="agent-session-row__main" @click="openSessionFromHistory(session)">
              <strong>{{ session.title }}</strong>
              <span>{{ session.knowledge_base_ids.map((id) => knowledgeBaseName[id] || id).join('、') }} · {{ formatTime(session.updated_at) }}</span>
            </button>
            <button type="button" class="agent-text-btn is-danger" @click="removeSession(session)">
              {{ $t('common.delete') }}
            </button>
          </div>
        </div>
        <div class="agent-dialog__actions">
          <button type="button" class="agent-btn is-primary" @click="openNewTaskDialog">{{ $t('agentWorkspace.newTask') }}</button>
        </div>
      </div>
    </t-dialog>

    <t-dialog
      v-model:visible="versionsDialogVisible"
      :header="$t('agentWorkspace.versionHistory')"
      width="760px"
      :footer="false"
    >
      <div class="agent-dialog">
        <p v-if="versionsLoading" class="agent-hint">{{ $t('agentWorkspace.loading') }}</p>
        <p v-else-if="!currentVersions.length" class="agent-hint">{{ $t('agentWorkspace.noVersions') }}</p>
        <div v-else class="agent-version-list">
          <div v-for="version in currentVersions" :key="version.id" class="agent-version-row">
            <div class="agent-version-row__main">
              <strong>{{ version.version_label }}</strong>
              <span>{{ versionSourceLabel(version) }} · {{ formatTime(version.created_at) }}</span>
              <small>{{ version.title || version.description || '—' }}</small>
            </div>
            <span v-if="version.is_current" class="agent-state is-done">{{ $t('agentWorkspace.versionCurrent') }}</span>
            <button
              v-else
              type="button"
              class="agent-text-btn"
              @click="rollbackToVersion(version)"
            >
              {{ $t('agentWorkspace.versionRollback') }}
            </button>
          </div>
        </div>
        <div v-if="currentVersions.length > 1" class="agent-version-compare">
          <div class="agent-version-compare__row">
            <label class="agent-field is-inline">
              <span>{{ $t('agentWorkspace.compareFrom') }}</span>
              <select v-model="compareFrom">
                <option v-for="version in currentVersions" :key="version.id" :value="version.id">{{ version.version_label }}</option>
              </select>
            </label>
            <label class="agent-field is-inline">
              <span>{{ $t('agentWorkspace.compareTo') }}</span>
              <select v-model="compareTo">
                <option v-for="version in currentVersions" :key="version.id" :value="version.id">{{ version.version_label }}</option>
              </select>
            </label>
            <button type="button" class="agent-btn" :disabled="comparing" @click="runCompare">
              {{ $t('agentWorkspace.compare') }}
            </button>
          </div>
          <div v-if="compareData" class="agent-version-diff">
            <p class="agent-hint">{{ $t('agentWorkspace.compareSummary', { from: compareData.from.version_label, to: compareData.to.version_label }) }}</p>
            <pre>{{ (compareData.to_markdown || '').slice(0, 4000) }}</pre>
          </div>
        </div>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import {
  AddIcon,
  ChatIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  EditIcon,
  FileIcon,
  FolderIcon,
  GitBranchIcon,
  HistoryIcon,
  LayoutIcon,
} from 'tdesign-icons-vue-next'
import PageTopbar from '@/components/layout/PageTopbar.vue'
import TraceSourceIcon from '@/views/chat/components/TraceSourceIcon.vue'
import ModelSelector from '@/components/ModelSelector.vue'
import { loadPreference, savePreference } from '@/composables/preferenceStorage'
import { renderDocumentPreviewMarkdown } from '@/utils/documentPreviewMarkdown'
import {
  createAgentWorkspaceSession,
  deleteAgentWorkspaceSession,
  listAgentWorkspaceSessions,
  loadAgentWorkspaceMessages,
  streamAgentWorkspaceMessage,
  updateAgentWorkspaceSession,
  type AgentWorkspaceMessage,
  type AgentWorkspaceSession,
  type AgentWorkspaceStreamEvent,
} from '@/api/agent-workspace'
import { listModels, type ModelConfig } from '@/api/model'
import { listKnowledgeBases, listKnowledgeFiles } from '@/api/knowledge-base'
import {
  compareDocVersions,
  getKnowledgeContent,
  listDocVersions,
  rollbackDocVersion,
  type DocVersion,
  type DocVersionCompare,
  type DocumentContentView,
} from '@/api/knowledge-base/versions'
import {
  applyWorkspaceStreamEvent,
  createWorkspaceTurn,
  editResultsFromEvents,
  eventsFromAgentSteps,
  type WorkspaceStreamItem,
  type WorkspaceToolEvent,
  type WorkspaceTurnState,
} from '@/utils/agentWorkspaceStream'

type LineKind = 'normal' | 'is-h1' | 'is-h2' | 'is-quote'
type EditorLine = { key: string; no: number; text: string; kind: LineKind }
type DocRow = {
  id: string
  title: string
  fileName: string
  folderPath: string
  folderId: string
  knowledgeBaseId: string
  parseStatus: string
  syncStatus: string
}
type KnowledgeBaseOption = { id: string; name: string; type: string }

const { t } = useI18n()
const chatCollapsed = ref(false)
const editorCollapsed = ref(false)
const draft = ref('')

const workbenchRef = ref<HTMLElement | null>(null)
const chatBodyRef = ref<HTMLElement | null>(null)
const resizing = ref(false)
const MIN_CHAT_WIDTH = 280
const DEFAULT_CHAT_WIDTH = 392
const DIVIDER_WIDTH = 7

const workbenchWidth = ref(0)
const chatWidth = ref(loadChatWidth())
const maxChatWidth = computed(() => {
  return Math.max(MIN_CHAT_WIDTH, Math.round(workbenchWidth.value / 2) - DIVIDER_WIDTH)
})
const workbenchStyle = computed(() => ({ '--agent-chat-width': `${Math.round(chatWidth.value)}px` }))

// ---- agent workspace data -------------------------------------------------
const sessions = ref<AgentWorkspaceSession[]>([])
const activeSession = ref<AgentWorkspaceSession | null>(null)
const knowledgeBases = ref<KnowledgeBaseOption[]>([])
const documents = ref<DocRow[]>([])
const currentDocId = ref('')
const currentDoc = ref<DocumentContentView | null>(null)
const currentVersions = ref<DocVersion[]>([])
const historyMessages = ref<AgentWorkspaceMessage[]>([])
const messageEvents = ref<Map<string, WorkspaceStreamItem[]>>(new Map())
const liveTurn = ref<WorkspaceTurnState | null>(null)
const streaming = ref(false)
const models = ref<ModelConfig[]>([])
const expandedFolders = ref<string[]>([''])
let streamHandle: { close: () => void } | null = null
let documentChangedInTurn = false

const newSessionDialogVisible = ref(false)
const newSessionSaving = ref(false)
const newSessionForm = reactive({ title: '', kbId: '', modelId: '' })
const historyDialogVisible = ref(false)
const sessionsLoading = ref(false)
const versionsDialogVisible = ref(false)
const versionsLoading = ref(false)
const compareFrom = ref('')
const compareTo = ref('')
const compareData = ref<DocVersionCompare | null>(null)
const comparing = ref(false)

const knowledgeBaseName = computed(() => {
  const map: Record<string, string> = {}
  for (const kb of knowledgeBases.value) map[kb.id] = kb.name
  return map
})
const scopeTitle = computed(() => {
  const ids = activeSession.value?.knowledge_base_ids || []
  if (!ids.length) return t('agentWorkspace.treeRoot')
  return ids.map((id) => knowledgeBaseName.value[id] || id).join(' · ')
})
const currentDocTitle = computed(() => currentDoc.value?.title || '')
const currentVersionLabel = computed(() => currentDoc.value?.version?.version_label || '—')
const syncStatus = computed(() => currentDoc.value?.engine_sync_status || '')
const syncLabel = computed(() => {
  switch (syncStatus.value) {
    case 'pending':
      return t('agentWorkspace.syncPending')
    case 'failed':
      return t('agentWorkspace.syncFailed')
    case 'synced':
      return t('agentWorkspace.syncSynced')
    default:
      return '—'
  }
})
const scopeHint = computed(() => {
  const ids = activeSession.value?.knowledge_base_ids || []
  return ids.map((id) => knowledgeBaseName.value[id] || id).join('、') || '—'
})
const liveThinking = computed(() => {
  const events = liveTurn.value?.events || []
  for (let index = events.length - 1; index >= 0; index -= 1) {
    const event = events[index]
    if (event.type === 'thinking' && event.content) return event.content
  }
  return ''
})
const liveToolEvents = computed<WorkspaceToolEvent[]>(() =>
  (liveTurn.value?.events || []).filter((event): event is WorkspaceToolEvent => event.type === 'tool_call'),
)
const liveAnswerHtml = computed(() => renderDocumentPreviewMarkdown(liveTurn.value?.content || ''))
function syncStatusLabel(status: string): string {
  if (status === 'pending') return t('agentWorkspace.syncPending')
  if (status === 'failed') return t('agentWorkspace.syncFailed')
  if (status === 'synced') return t('agentWorkspace.syncSynced')
  return ''
}
const docGroups = computed(() => {
  const groups = new Map<string, DocRow[]>()
  for (const doc of documents.value) {
    const key = doc.folderPath || ''
    const list = groups.get(key) || []
    list.push(doc)
    groups.set(key, list)
  }
  return [...groups.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([path, docs]) => ({
      path,
      label: path || t('agentWorkspace.treeRoot'),
      docs: docs.slice().sort((a, b) => a.title.localeCompare(b.title)),
    }))
})
const editorLines = computed<EditorLine[]>(() => {
  const markdown = currentDoc.value?.markdown || ''
  if (!markdown) return []
  return markdown.replace(/\r\n/g, '\n').split('\n').map((text, index) => {
    let kind: LineKind = 'normal'
    if (/^#\s/.test(text)) kind = 'is-h1'
    else if (/^#{2,6}\s/.test(text)) kind = 'is-h2'
    else if (/^>\s?/.test(text)) kind = 'is-quote'
    return { key: `line-${index}`, no: index + 1, text, kind }
  })
})
const canSend = computed(() => Boolean(activeSession.value) && !streaming.value)

// ---- loading --------------------------------------------------------------
async function loadModels() {
  try {
    const all = await listModels('KnowledgeQA')
    models.value = Array.isArray(all) ? all : []
    if (!newSessionForm.modelId && models.value.length) {
      newSessionForm.modelId = String(models.value[0].id || '')
    }
  } catch {
    models.value = []
  }
}

async function loadKnowledgeBases() {
  try {
    const response = await listKnowledgeBases({ page: 1, page_size: 200 } as never)
    const body = response as unknown as { data?: KnowledgeBaseOption[] | { data?: KnowledgeBaseOption[] } }
    const raw = body?.data
    const list = (Array.isArray(raw) ? raw : raw?.data) || []
    knowledgeBases.value = list.filter((kb) => kb.type !== 'faq')
    if (!newSessionForm.kbId && knowledgeBases.value.length) {
      newSessionForm.kbId = knowledgeBases.value[0].id
    }
  } catch {
    knowledgeBases.value = []
  }
}

async function loadSessions() {
  sessionsLoading.value = true
  try {
    const response = await listAgentWorkspaceSessions({ page: 1, page_size: 50 })
    sessions.value = response.data || []
  } catch (error: unknown) {
    MessagePlugin.error(error instanceof Error ? error.message : t('agentWorkspace.sessionLoadFailed'))
  } finally {
    sessionsLoading.value = false
  }
}

async function loadDocuments() {
  const session = activeSession.value
  if (!session) {
    documents.value = []
    return
  }
  const rows: DocRow[] = []
  for (const kbId of session.knowledge_base_ids) {
    try {
      const response = await listKnowledgeFiles(kbId, { page: 1, page_size: 200 })
      const list = (response as unknown as { data?: Array<Record<string, unknown>> }).data || []
      for (const doc of list) {
        rows.push({
          id: String(doc.id || ''),
          title: String(doc.title || doc.file_name || doc.id || ''),
          fileName: String(doc.file_name || ''),
          folderPath: String(doc.folder_path || ''),
          folderId: String(doc.folder_id || ''),
          knowledgeBaseId: kbId,
          parseStatus: String(doc.parse_status || ''),
          syncStatus: String(doc.engine_sync_status || ''),
        })
      }
    } catch {
      // A KB failure should not hide documents from the other KBs.
    }
  }
  documents.value = rows.filter((row) => row.id)
  if (!currentDocId.value && documents.value.length) {
    await selectDoc(documents.value[0].id)
  }
}

async function loadDocContent() {
  if (!currentDocId.value) {
    currentDoc.value = null
    currentVersions.value = []
    return
  }
  try {
    const response = await getKnowledgeContent(currentDocId.value)
    currentDoc.value = response.data
    await loadVersions()
  } catch (error: unknown) {
    currentDoc.value = null
    MessagePlugin.error(error instanceof Error ? error.message : t('agentWorkspace.loadDocFailed'))
  }
}

async function loadVersions() {
  if (!currentDocId.value) {
    currentVersions.value = []
    return
  }
  try {
    const response = await listDocVersions(currentDocId.value)
    currentVersions.value = response.data?.versions || []
  } catch {
    currentVersions.value = []
  }
}

async function loadMessages() {
  const session = activeSession.value
  if (!session) {
    historyMessages.value = []
    messageEvents.value = new Map()
    return
  }
  try {
    const response = await loadAgentWorkspaceMessages(session.id, { page: 1, page_size: 200 })
    const messages = response.data?.messages || []
    historyMessages.value = messages
    const map = new Map<string, WorkspaceStreamItem[]>()
    for (const message of messages) {
      if (message.role === 'assistant') {
        map.set(message.id, eventsFromAgentSteps(message.agent_steps))
      }
    }
    messageEvents.value = map
    await nextTick()
    scrollChatToBottom()
  } catch (error: unknown) {
    MessagePlugin.error(error instanceof Error ? error.message : t('agentWorkspace.sessionLoadFailed'))
  }
}

async function selectSession(session: AgentWorkspaceSession) {
  cancelStream()
  activeSession.value = session
  currentDocId.value = ''
  currentDoc.value = null
  currentVersions.value = []
  liveTurn.value = null
  await Promise.all([loadDocuments(), loadMessages()])
}

async function openSessionFromHistory(session: AgentWorkspaceSession) {
  historyDialogVisible.value = false
  await selectSession(session)
}

async function selectDoc(docId: string) {
  if (!docId) return
  currentDocId.value = docId
  await loadDocContent()
}

// ---- session dialogs ------------------------------------------------------
function openNewTaskDialog() {
  if (!knowledgeBases.value.length) {
    void loadKnowledgeBases()
  }
  if (!models.value.length) {
    void loadModels()
  }
  newSessionForm.title = ''
  if (!newSessionForm.kbId && knowledgeBases.value.length) {
    newSessionForm.kbId = knowledgeBases.value[0].id
  }
  if (!newSessionForm.modelId && models.value.length) {
    newSessionForm.modelId = String(models.value[0].id || '')
  }
  newSessionDialogVisible.value = true
}

async function submitNewSession() {
  if (!newSessionForm.kbId) {
    MessagePlugin.warning(t('agentWorkspace.sessionKbRequired'))
    return
  }
  newSessionSaving.value = true
  try {
    const response = await createAgentWorkspaceSession({
      title: newSessionForm.title.trim() || undefined,
      model_id: newSessionForm.modelId || undefined,
      knowledge_base_ids: [newSessionForm.kbId],
    })
    newSessionDialogVisible.value = false
    await loadSessions()
    await selectSession(response.data)
  } catch (error: unknown) {
    MessagePlugin.error(error instanceof Error ? error.message : t('agentWorkspace.createFailed'))
  } finally {
    newSessionSaving.value = false
  }
}

function removeSession(session: AgentWorkspaceSession) {
  const dialog = DialogPlugin.confirm({
    header: t('agentWorkspace.deleteSession'),
    body: t('agentWorkspace.deleteSessionConfirm'),
    theme: 'warning',
    confirmBtn: t('common.delete'),
    cancelBtn: t('common.cancel'),
    onConfirm: async () => {
      dialog.hide()
      try {
        await deleteAgentWorkspaceSession(session.id)
        if (activeSession.value?.id === session.id) {
          activeSession.value = null
          historyMessages.value = []
          documents.value = []
          currentDoc.value = null
        }
        await loadSessions()
      } catch (error: unknown) {
        MessagePlugin.error(error instanceof Error ? error.message : t('agentWorkspace.deleteFailed'))
      }
    },
  })
}

async function changeSessionModel(value: string) {
  const session = activeSession.value
  if (!session || !value || value === session.model_id) return
  try {
    const response = await updateAgentWorkspaceSession(session.id, { model_id: value })
    activeSession.value = response.data
    const index = sessions.value.findIndex((item) => item.id === session.id)
    if (index >= 0) sessions.value[index] = response.data
  } catch (error: unknown) {
    MessagePlugin.error(error instanceof Error ? error.message : t('agentWorkspace.createFailed'))
  }
}

// ---- versions -------------------------------------------------------------
async function openVersionsDialog() {
  if (!currentDocId.value) {
    MessagePlugin.warning(t('agentWorkspace.selectDocFirst'))
    return
  }
  versionsDialogVisible.value = true
  versionsLoading.value = true
  compareData.value = null
  try {
    await loadVersions()
    const list = currentVersions.value
    compareTo.value = list[0]?.id || ''
    compareFrom.value = list[1]?.id || list[0]?.id || ''
  } finally {
    versionsLoading.value = false
  }
}

async function runCompare() {
  if (!currentDocId.value || !compareFrom.value || !compareTo.value) return
  comparing.value = true
  try {
    const response = await compareDocVersions(currentDocId.value, compareFrom.value, compareTo.value)
    compareData.value = response.data
  } catch (error: unknown) {
    MessagePlugin.error(error instanceof Error ? error.message : t('agentWorkspace.compareFailed'))
  } finally {
    comparing.value = false
  }
}

function rollbackToVersion(version: DocVersion) {
  const dialog = DialogPlugin.confirm({
    header: t('agentWorkspace.versionRollback'),
    body: t('agentWorkspace.rollbackConfirmBody', { label: version.version_label }),
    theme: 'warning',
    confirmBtn: t('agentWorkspace.versionRollback'),
    cancelBtn: t('common.cancel'),
    onConfirm: async () => {
      dialog.hide()
      await doRollback(version.id)
    },
  })
}

async function doRollback(versionId: string) {
  if (!currentDocId.value || !versionId) return
  try {
    await rollbackDocVersion(currentDocId.value, versionId)
    MessagePlugin.success(t('agentWorkspace.rollbackDone'))
    await Promise.all([loadDocContent(), loadDocuments()])
    if (versionsDialogVisible.value) await loadVersions()
  } catch (error: unknown) {
    MessagePlugin.error(error instanceof Error ? error.message : t('agentWorkspace.rollbackFailed'))
  }
}

function undoEdit(documentId: string, previousVersionId: string) {
  if (!previousVersionId) {
    MessagePlugin.warning(t('agentWorkspace.noPreviousVersion'))
    return
  }
  if (documentId !== currentDocId.value) {
    void selectDoc(documentId).then(() => doRollback(previousVersionId))
    return
  }
  void doRollback(previousVersionId)
}

// ---- chat turn ------------------------------------------------------------
function cancelStream() {
  streamHandle?.close()
  streamHandle = null
  streaming.value = false
  liveTurn.value = null
  documentChangedInTurn = false
}

async function send() {
  const session = activeSession.value
  if (!session) {
    openNewTaskDialog()
    return
  }
  if (streaming.value) return
  const query = draft.value.trim()
  if (!query) return
  draft.value = ''
  documentChangedInTurn = false
  historyMessages.value = [
    ...historyMessages.value,
    {
      id: `local-${Date.now()}`,
      session_id: session.id,
      role: 'user',
      content: query,
      created_at: new Date().toISOString(),
    },
  ]
  const state = reactive(createWorkspaceTurn()) as WorkspaceTurnState
  liveTurn.value = state
  streaming.value = true
  await nextTick()
  scrollChatToBottom()
  streamHandle = streamAgentWorkspaceMessage(session.id, query, {
    onEvent(event: AgentWorkspaceStreamEvent) {
      const effect = applyWorkspaceStreamEvent(state, event)
      if (effect.documentChange) {
        documentChangedInTurn = true
        if (effect.documentChange.data.document_id === currentDocId.value) {
          void loadDocContent()
        } else if (effect.documentChange.toolName === 'create_document') {
          void loadDocuments()
        }
      }
      scrollChatToBottom()
    },
    onError(error: Error) {
      MessagePlugin.error(error.message || t('agentWorkspace.sendFailed'))
      finishTurn()
    },
    onClose() {
      finishTurn()
    },
  })
}

let finishing = false
async function finishTurn() {
  if (finishing) return
  finishing = true
  streaming.value = false
  streamHandle = null
  await loadMessages()
  liveTurn.value = null
  if (documentChangedInTurn) {
    await loadDocuments()
    await loadDocContent()
  }
  finishing = false
}

function scrollChatToBottom() {
  const el = chatBodyRef.value
  if (!el) return
  void nextTick(() => {
    el.scrollTop = el.scrollHeight
  })
}

watch(
  () => historyMessages.value.length,
  () => {
    void nextTick(scrollChatToBottom)
  },
)

function assistantEvents(message: AgentWorkspaceMessage): WorkspaceStreamItem[] {
  return messageEvents.value.get(message.id) || []
}

function assistantEdits(message: AgentWorkspaceMessage) {
  return editResultsFromEvents(assistantEvents(message))
}

function assistantHtml(message: AgentWorkspaceMessage): string {
  return renderDocumentPreviewMarkdown(String(message.content || ''))
}

function toolLabel(name: string): string {
  const map: Record<string, string> = {
    list_documents: t('agentWorkspace.toolListDocuments'),
    get_document: t('agentWorkspace.toolGetDocument'),
    search_documents: t('agentWorkspace.toolSearchDocuments'),
    extract_knowledge_points: t('agentWorkspace.toolExtractPoints'),
    edit_document_section: t('agentWorkspace.toolEditSection'),
    create_document: t('agentWorkspace.toolCreateDocument'),
  }
  return map[name] || name
}

function toolStateLabel(event: WorkspaceStreamItem): string {
  if (event.type !== 'tool_call') return ''
  if (event.pending) return t('agentWorkspace.toolPending')
  return event.success ? t('agentWorkspace.toolDone') : t('agentWorkspace.toolFailed')
}

function toolDotClass(event: WorkspaceStreamItem): string {
  if (event.type !== 'tool_call') return ''
  if (event.pending) return 'is-warning'
  return event.success ? 'is-success' : 'is-warning'
}

function versionSourceLabel(version: DocVersion): string {
  const map: Record<string, string> = {
    upload: t('agentWorkspace.versionSourceUpload'),
    manual_edit: t('agentWorkspace.versionSourceManual'),
    rollback: t('agentWorkspace.versionSourceRollback'),
    reparse: t('agentWorkspace.versionSourceReparse'),
    agent_edit: t('agentWorkspace.versionSourceAgentEdit'),
    agent_create: t('agentWorkspace.versionSourceAgentCreate'),
  }
  return map[version.source_type] || version.source_type
}

function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function toggleFolder(path: string) {
  expandedFolders.value = expandedFolders.value.includes(path)
    ? expandedFolders.value.filter((item) => item !== path)
    : [...expandedFolders.value, path]
}

function loadChatWidth(): number {
  const raw = Number(loadPreference('agent_chat_width'))
  return Number.isFinite(raw) && raw >= MIN_CHAT_WIDTH ? raw : DEFAULT_CHAT_WIDTH
}

function clampChatWidth(value: number): number {
  return Math.round(Math.min(Math.max(value, MIN_CHAT_WIDTH), maxChatWidth.value))
}

function persistChatWidth() {
  savePreference('agent_chat_width', String(Math.round(chatWidth.value)))
}

let dragStartX = 0
let dragStartWidth = 0
let workbenchObserver: ResizeObserver | null = null

function startDrag(event: PointerEvent) {
  if (event.button !== 0) return
  event.preventDefault()
  resizing.value = true
  dragStartX = event.clientX
  dragStartWidth = chatWidth.value
  window.addEventListener('pointermove', onDragMove)
  window.addEventListener('pointerup', stopDrag, { once: true })
}

function onDragMove(event: PointerEvent) {
  chatWidth.value = clampChatWidth(dragStartWidth - (event.clientX - dragStartX))
}

function stopDrag() {
  resizing.value = false
  window.removeEventListener('pointermove', onDragMove)
  persistChatWidth()
}

function onDividerKeydown(event: KeyboardEvent) {
  let next = chatWidth.value
  if (event.key === 'ArrowLeft') next += 16
  else if (event.key === 'ArrowRight') next -= 16
  else if (event.key === 'Home') next = MIN_CHAT_WIDTH
  else if (event.key === 'End') next = maxChatWidth.value
  else return
  event.preventDefault()
  chatWidth.value = clampChatWidth(next)
  persistChatWidth()
}

function resetChatWidth() {
  chatWidth.value = clampChatWidth(DEFAULT_CHAT_WIDTH)
  persistChatWidth()
}

function syncWorkbenchWidth() {
  workbenchWidth.value = workbenchRef.value?.clientWidth ?? 0
  chatWidth.value = clampChatWidth(chatWidth.value)
}

onMounted(async () => {
  syncWorkbenchWidth()
  window.addEventListener('resize', syncWorkbenchWidth)
  if (workbenchRef.value && typeof ResizeObserver !== 'undefined') {
    workbenchObserver = new ResizeObserver(syncWorkbenchWidth)
    workbenchObserver.observe(workbenchRef.value)
  }
  await Promise.all([loadKnowledgeBases(), loadModels(), loadSessions()])
  if (sessions.value.length) {
    await selectSession(sessions.value[0])
  }
})

onUnmounted(() => {
  window.removeEventListener('pointermove', onDragMove)
  window.removeEventListener('resize', syncWorkbenchWidth)
  workbenchObserver?.disconnect()
  workbenchObserver = null
  streamHandle?.close()
})
</script>

<style lang="less" scoped>
.agent-page {
  --ag-paper: var(--w-agent-surface, #ffffff);
  --ag-raised: var(--w-agent-raised, #ffffff);
  --ag-border: var(--w-agent-border, #0f4a85);
  --ag-border-soft: color-mix(in srgb, var(--ag-border) 34%, var(--ag-paper));
  --ag-ink: var(--w-detail-text-primary, #292929);
  --ag-body: var(--w-detail-text-secondary, #515151);
  --ag-muted: var(--w-detail-text-placeholder, #7f7f7f);
  --ag-accent: var(--w-page-accent, #0f4a85);
  --ag-accent-emphasis: var(--ag-accent);
  --ag-accent-ink: var(--w-page-accent-ink, #ffffff);
  --ag-selection: color-mix(in srgb, var(--ag-accent) 10%, var(--ag-paper));
  --ag-hover: color-mix(in srgb, var(--ag-accent) 6%, var(--ag-paper));
  --ag-focus: #006bbd;
  --ag-warning: #895503;
  --ag-warning-soft: color-mix(in srgb, #895503 8%, var(--ag-paper));
  --ag-danger: #b5200d;
  --ag-danger-soft: color-mix(in srgb, #b5200d 8%, var(--ag-paper));
  --ag-add: #096d48;
  --ag-add-soft: color-mix(in srgb, #096d48 8%, var(--ag-paper));
  --ag-code-bg: #f2f2f2;
  --page-topbar-line: var(--ag-border);
  --page-topbar-title: var(--ag-ink);
  --page-topbar-meta: var(--ag-body);

  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--ag-paper);
  color: var(--ag-ink);
}

html[theme-mode="dark"] .agent-page {
  /* GitHub Primer dark-high-contrast: canvas #010409, muted #151b23,
     border #b7bdc8, fg #ffffff / #d1d7e0, accent #74b9ff / #194fb1. */
  --ag-paper: #010409;
  --ag-raised: #151b23;
  --ag-border: #b7bdc8;
  --ag-border-soft: color-mix(in srgb, #b7bdc8 55%, transparent);
  --ag-ink: #ffffff;
  --ag-body: #d1d7e0;
  --ag-muted: #b7bdc8;
  --ag-accent: #74b9ff;
  --ag-accent-emphasis: #194fb1;
  --ag-accent-ink: #ffffff;
  --ag-selection: #5cacff1a;
  --ag-hover: #212830;
  --ag-focus: #409eff;
  --ag-warning: #f0b72f;
  --ag-warning-soft: #edaa2726;
  --ag-danger: #ff9492;
  --ag-danger-soft: #ff80801a;
  --ag-add: #2bd853;
  --ag-add-soft: #0ac74026;
  --ag-code-bg: #151b23;
}

.agent-page.is-resizing {
  cursor: col-resize;
  user-select: none;
}

.agent-page :deep(.page-topbar) {
  margin-bottom: 0;
}

.agent-bar-btn {
  width: 30px;
  height: 30px;
  display: inline-grid;
  place-items: center;
  padding: 0;
  border: 1px solid var(--ag-border);
  border-radius: 3px;
  background: var(--ag-paper);
  color: var(--ag-ink);
  font-size: 16px;
  cursor: pointer;
}

.agent-bar-btn:hover {
  background: var(--ag-selection);
  color: var(--ag-accent);
}

.agent-bar-btn.is-on {
  background: var(--ag-selection);
  color: var(--ag-accent);
}

.agent-bar-btn.is-primary {
  border-color: var(--ag-accent-emphasis);
  background: var(--ag-accent-emphasis);
  color: var(--ag-accent-ink);
}

.agent-bar-btn.is-primary:hover {
  border-color: var(--ag-accent-emphasis);
  background: color-mix(in srgb, var(--ag-accent-emphasis) 88%, var(--ag-ink));
  color: var(--ag-accent-ink);
}

.agent-bar-sep {
  width: 1px;
  height: 20px;
  margin: 0 2px;
  background: var(--ag-border-soft);
}

.agent-workbench {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: 250px minmax(0, 1fr) 7px var(--agent-chat-width, 392px);
  overflow: hidden;
  background: var(--ag-paper);
}

.agent-page.is-single-pane .agent-workbench {
  grid-template-columns: 250px minmax(0, 1fr);
}

.agent-pane {
  min-height: 0;
  overflow: hidden;
}

.agent-icon-btn {
  width: 26px;
  height: 26px;
  display: inline-grid;
  place-items: center;
  flex: none;
  padding: 0;
  border: 0;
  border-radius: 3px;
  background: transparent;
  color: var(--ag-body);
  font-size: 15px;
  cursor: pointer;
}

.agent-icon-btn:hover {
  background: var(--ag-selection);
  color: var(--ag-accent);
}

/* document tree */
.agent-tree {
  overflow-y: auto;
  padding: 14px 10px;
  border-right: 1px solid var(--ag-border);
  background: var(--ag-raised);
}

.agent-tree__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 6px 10px;
  color: var(--ag-ink);
  font-size: 12.5px;
  font-weight: 600;
  letter-spacing: 0.03em;
}

.agent-tree__row {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  min-height: 32px;
  padding: 4px 8px;
  border: 0;
  border-radius: 0;
  background: transparent;
  color: var(--ag-body);
  font-size: 13.5px;
  text-align: left;
  cursor: pointer;
}

.agent-tree__row:hover {
  background: var(--ag-hover);
  color: var(--ag-ink);
}

.agent-tree__row.is-folder {
  color: var(--ag-ink);
  font-weight: 500;
}

.agent-tree__row.is-child {
  padding-left: 28px;
}

.agent-tree__row.is-active {
  background: var(--ag-selection);
  box-shadow: inset 3px 0 0 var(--ag-accent);
  color: var(--ag-accent);
  font-weight: 600;
}

.agent-tree__row.is-muted {
  color: var(--ag-body);
}

.agent-tree__caret {
  width: 14px;
  flex: none;
  color: var(--ag-body);
  font-size: 13px;
}

.agent-tree__icon {
  flex: none;
  color: var(--ag-body);
  font-size: 15px;
}

.agent-tree__row.is-active .agent-tree__icon,
.agent-tree__row.is-active .agent-tree__caret {
  color: var(--ag-accent);
}

.agent-tree__row > span:not(.agent-tree__caret) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-tree__row em {
  margin-left: auto;
  color: var(--ag-muted);
  font-size: 12px;
  font-style: normal;
}

.agent-tree__dot {
  width: 7px;
  height: 7px;
  margin-left: auto;
  border-radius: 50%;
  background: var(--ag-warning);
}

.agent-tree__divider {
  height: 1px;
  margin: 10px 6px;
  background: var(--ag-border-soft);
}

.agent-version-card {
  display: flex;
  flex-direction: column;
  gap: 3px;
  margin: 14px 6px 0;
  padding: 10px 12px;
  border: 1px solid var(--ag-border);
  border-radius: 3px;
  background: var(--ag-paper);
  color: var(--ag-body);
  font-size: 13px;
}

.agent-version-card strong {
  color: var(--ag-ink);
  font-size: 13px;
}

.agent-version-card small {
  font-size: 12px;
}

/* editor */
.agent-editor {
  display: flex;
  flex-direction: column;
  min-width: 0;
  background: var(--ag-paper);
}

.agent-tabs {
  flex: none;
  height: 38px;
  display: flex;
  align-items: stretch;
  border-bottom: 1px solid var(--ag-border);
  background: var(--ag-paper);
}

.agent-tab {
  max-width: 280px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  overflow: hidden;
  padding: 0 12px;
  border: 0;
  border-right: 1px solid var(--ag-border-soft);
  background: transparent;
  color: var(--ag-body);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
}

.agent-tab > span {
  overflow: hidden;
  text-overflow: ellipsis;
}

.agent-tab.is-active {
  background: var(--ag-paper);
  box-shadow: inset 0 2px 0 var(--ag-accent);
  color: var(--ag-ink);
}

.agent-tab__dirty {
  width: 7px;
  height: 7px;
  flex: none;
  border-radius: 50%;
  background: var(--ag-warning);
}

.agent-tabs__spacer {
  flex: 1;
}

.agent-tabs .agent-icon-btn {
  align-self: center;
}

.agent-breadcrumb {
  flex: none;
  padding: 6px 16px;
  border-bottom: 1px solid var(--ag-border-soft);
  color: var(--ag-body);
  font-size: 12.5px;
}

.agent-breadcrumb b {
  color: var(--ag-ink);
  font-weight: 600;
}

.agent-breadcrumb span:last-child {
  color: var(--ag-accent);
}

.agent-code {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 16px 0 40px;
  color: var(--ag-ink);
  font-family: var(--app-font-family-mono, monospace);
  font-size: 13.2px;
  line-height: 1.8;
}

.agent-line {
  display: flex;
  min-height: 24px;
}

.agent-line__no {
  width: 48px;
  flex: none;
  padding-right: 12px;
  color: var(--ag-muted);
  text-align: right;
  user-select: none;
}

.agent-line__text {
  flex: 1;
  min-width: 0;
  padding-right: 20px;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.agent-line.is-add {
  background: var(--ag-add-soft);
  box-shadow: inset 3px 0 0 var(--ag-add);
}

.agent-line.is-add .agent-line__text {
  color: var(--ag-add);
}

.agent-line.is-del {
  background: var(--ag-danger-soft);
  box-shadow: inset 3px 0 0 var(--ag-danger);
}

.agent-line.is-del .agent-line__text {
  color: var(--ag-danger);
  text-decoration: line-through;
}

.agent-line.is-h1 .agent-line__text {
  color: var(--ag-accent);
  font-size: 1.35em;
  font-weight: 700;
}

.agent-line.is-h2 .agent-line__text {
  color: var(--ag-accent);
  font-size: 1.12em;
  font-weight: 700;
}

.agent-line.is-quote {
  background: var(--ag-code-bg);
}

.agent-line.is-quote .agent-line__text {
  color: var(--ag-body);
}

.agent-inline-code {
  padding: 1px 4px;
  border: 1px solid var(--ag-border-soft);
  border-radius: 2px;
  background: var(--ag-code-bg);
  color: var(--ag-accent);
  font-size: 0.94em;
}

.agent-insert-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-right: 8px;
  padding: 1px 8px;
  border: 1px solid var(--ag-accent);
  border-radius: 2px;
  background: var(--ag-paper);
  color: var(--ag-accent);
  font-size: 11.5px;
  font-weight: 700;
}

.agent-divider {
  position: relative;
  width: 7px;
  min-height: 0;
  cursor: col-resize;
  background: transparent;
  outline: none;
  z-index: 4;
}

.agent-divider::after {
  content: "";
  position: absolute;
  top: 0;
  bottom: 0;
  left: 3px;
  width: 1px;
  background: var(--ag-border);
}

.agent-divider:hover::after,
.agent-divider:focus-visible::after {
  left: 2px;
  width: 3px;
  background: var(--ag-accent);
}

/* agent chat */
.agent-chat {
  display: flex;
  flex-direction: column;
  background: var(--ag-raised);
}

.agent-chat__head {
  flex: none;
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--ag-border);
}

.agent-chat__icon {
  flex: none;
}

.agent-chat__icon :deep(.trace-source-icon__dark) {
  fill: var(--ag-accent);
}

.agent-chat__icon :deep(.trace-source-icon__light) {
  fill: var(--ag-accent);
  opacity: 0.55;
}

.agent-chat__identity {
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.agent-chat__identity strong {
  color: var(--ag-ink);
  font-size: 14px;
}

.agent-chat__identity span {
  overflow: hidden;
  color: var(--ag-body);
  font-size: 12.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-chat__actions {
  margin-left: auto;
  display: flex;
  gap: 2px;
}

.agent-chat__identity strong {
  color: var(--ag-ink);
  font-size: 14px;
}

.agent-chat__identity span {
  overflow: hidden;
  color: var(--ag-body);
  font-size: 12.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-chat__actions {
  margin-left: auto;
  display: flex;
  gap: 2px;
}

.agent-chat__body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 0;
}

.agent-timeline {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 19px;
  padding: 18px 16px 24px 42px;
}

.agent-timeline::before {
  content: "";
  position: absolute;
  left: 18px;
  top: 22px;
  bottom: 24px;
  width: 1px;
  background: var(--ag-border-soft);
}

.agent-timeline > * {
  flex: none;
}


.agent-turn {
  position: relative;
}

.agent-node {
  position: absolute;
  left: -29px;
  top: 5px;
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--ag-body);
  box-shadow: 0 0 0 3px var(--ag-raised);
}

.agent-node.is-user,
.agent-node.is-write,
.agent-node.is-diff {
  background: var(--ag-accent);
}

.agent-node.is-success {
  background: var(--ag-add);
}

.agent-node.is-warning {
  background: var(--ag-warning);
}

.agent-node.is-agent {
  background: var(--ag-muted);
}

.agent-turn__content {
  min-width: 0;
}

.agent-turn.is-user .agent-turn__content {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  width: auto;
  max-width: 88%;
  margin-left: auto;
}

.agent-turn__meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 5px;
  color: var(--ag-body);
  font-size: 12.5px;
}

.agent-turn.is-user .agent-turn__meta {
  justify-content: flex-end;
}

.agent-turn__text {
  margin: 0;
  color: var(--ag-ink);
  font-size: 14px;
  line-height: 1.7;
}

.agent-user-box {
  width: fit-content;
  max-width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--ag-accent);
  border-radius: 3px;
  background: transparent;
  color: var(--ag-ink);
  font-size: 14px;
  line-height: 1.65;
}

.agent-node__head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  color: var(--ag-ink);
  font-size: 14px;
}

.agent-node__head > svg {
  flex: none;
  color: var(--ag-accent);
  font-size: 16px;
}

.agent-node__head strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-node__stat {
  margin-left: auto;
  color: var(--ag-body);
  font-family: var(--app-font-family-mono, monospace);
  font-size: 12.5px;
  white-space: nowrap;
}

.agent-state {
  margin-left: auto;
  padding: 1px 7px;
  border: 1px solid currentColor;
  border-radius: 2px;
  font-size: 12px;
  line-height: 1.5;
  white-space: nowrap;
}

.agent-state.is-done {
  background: var(--ag-add-soft);
  color: var(--ag-add);
}

.agent-node__panel {
  margin-top: 8px;
  padding: 9px 11px;
  border: 1px solid var(--ag-border-soft);
  border-radius: 3px;
  background: var(--ag-code-bg);
  color: var(--ag-body);
  font-size: 13.5px;
  line-height: 1.65;
}

.agent-node__panel p {
  margin: 0;
}

.agent-node__panel p + p {
  margin-top: 3px;
}

.agent-node__foot {
  margin-top: 6px;
}

.agent-text-btn {
  padding: 0;
  border: 0;
  background: none;
  color: var(--ag-accent);
  font-size: 12.5px;
  cursor: pointer;
}

.agent-diff-box {
  margin-top: 8px;
  padding: 7px 0;
  overflow: hidden;
  border: 1px solid var(--ag-border-soft);
  border-radius: 3px;
  background: var(--ag-code-bg);
  font-family: var(--app-font-family-mono, monospace);
  font-size: 13px;
  line-height: 1.7;
}

.agent-diff__line {
  display: flex;
  gap: 8px;
  padding: 1px 11px;
  white-space: pre-wrap;
}

.agent-diff__line span {
  width: 10px;
  color: var(--ag-muted);
}

.agent-diff__line.is-add {
  background: var(--ag-add-soft);
  color: var(--ag-add);
}

.agent-diff__line.is-del {
  background: var(--ag-danger-soft);
  color: var(--ag-danger);
}

.agent-pending-box {
  padding: 12px;
  border: 1px solid var(--ag-warning);
  border-radius: 3px;
  background: var(--ag-warning-soft);
  color: var(--ag-body);
  font-size: 13px;
  line-height: 1.7;
}

.agent-pending-box.is-confirmed {
  border-color: var(--ag-accent);
  background: var(--ag-selection);
}

.agent-pending-box header {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--ag-warning);
  font-size: 13px;
  font-weight: 700;
}

.agent-pending-box.is-confirmed header {
  color: var(--ag-accent);
}

.agent-pending__meta {
  margin-top: 6px;
}

.agent-pending__meta p {
  margin: 0;
}

.agent-pending__meta p + p {
  margin-top: 4px;
}

.agent-pending-box code {
  color: var(--ag-ink);
  font-family: var(--app-font-family-mono, monospace);
  font-size: 12px;
}

.agent-pending__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.agent-btn {
  min-height: 28px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 0 11px;
  border: 1px solid var(--ag-border);
  border-radius: 3px;
  background: var(--ag-paper);
  color: var(--ag-ink);
  font-size: 12px;
  cursor: pointer;
}

.agent-btn:hover {
  background: var(--ag-selection);
}

.agent-btn.is-primary {
  border-color: var(--ag-accent-emphasis);
  background: var(--ag-accent-emphasis);
  color: var(--ag-accent-ink);
  font-weight: 600;
}

.agent-btn.is-primary:hover {
  background: color-mix(in srgb, var(--ag-accent-emphasis) 88%, var(--ag-ink));
}

.agent-btn.is-danger {
  border-color: var(--ag-border-soft);
  color: var(--ag-danger);
}

.agent-btn.is-danger:hover {
  border-color: var(--ag-danger);
  background: var(--ag-danger-soft);
}

.agent-composer {
  flex: none;
  padding: 10px 12px 12px;
  border-top: 1px solid var(--ag-border);
  background: var(--ag-raised);
}

.agent-composer__mode {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 7px;
}

.agent-mode-pill {
  display: inline-flex;
  align-items: center;
  padding: 2px 9px;
  border-radius: 2px;
  background: var(--ag-selection);
  color: var(--ag-accent);
  font-size: 12px;
  font-weight: 600;
}

.agent-muted {
  color: var(--ag-muted);
  font-size: 12px;
}

.agent-composer textarea {
  width: 100%;
  height: 64px;
  padding: 9px 11px;
  border: 1px solid var(--ag-border);
  border-radius: 3px;
  outline: 0;
  resize: none;
  background: var(--ag-paper);
  color: var(--ag-ink);
  font-family: inherit;
  font-size: 13.5px;
  line-height: 1.6;
}

.agent-composer textarea:focus {
  border-color: var(--ag-focus);
  box-shadow: 0 0 0 1px var(--ag-focus);
}

.agent-composer__foot {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 7px;
  color: var(--ag-muted);
  font-size: 12px;
}

.agent-composer__spacer {
  flex: 1;
}

.agent-send {
  height: 28px;
  padding: 0 12px;
  border: 1px solid var(--ag-accent-emphasis);
  border-radius: 3px;
  background: var(--ag-accent-emphasis);
  color: var(--ag-accent-ink);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.agent-send:hover {
  background: color-mix(in srgb, var(--ag-accent-emphasis) 88%, var(--ag-ink));
}

.agent-restore {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 24px;
  background: var(--ag-paper);
  color: var(--ag-body);
  text-align: center;
}

.agent-restore strong {
  color: var(--ag-ink);
  font-size: 15px;
}

.agent-restore p {
  margin: 0;
  font-size: 13px;
}

.agent-restore__actions {
  display: flex;
  gap: 10px;
  margin-top: 8px;
}

/* live data additions */
.agent-hint {
  margin: 8px 6px;
  color: var(--ag-muted);
  font-size: 12.5px;
}

.agent-tree__empty {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 6px;
  color: var(--ag-body);
  font-size: 13px;
}

.agent-tree__empty p {
  margin: 0;
}

.agent-editor__empty {
  flex: 1;
  display: grid;
  place-items: center;
  color: var(--ag-muted);
  font-size: 13px;
}

.agent-chat__empty {
  padding: 24px 16px;
  color: var(--ag-muted);
  font-size: 13px;
}

.agent-thinking {
  margin: 0 0 10px;
  color: var(--ag-body);
  font-size: 12.5px;
}

.agent-thinking summary {
  color: var(--ag-muted);
  cursor: pointer;
}

.agent-thinking pre {
  max-height: 200px;
  padding: 8px 10px;
  margin: 6px 0 0;
  overflow: auto;
  font-family: inherit;
  font-size: 12.5px;
  line-height: 1.6;
  white-space: pre-wrap;
  background: var(--ag-code-bg);
  border: 1px solid var(--ag-border-soft);
  border-radius: 3px;
}

.agent-tool-line {
  padding: 8px 10px;
  margin: 0 0 10px;
  background: var(--ag-paper);
  border: 1px solid var(--ag-border-soft);
  border-radius: 3px;
}

.agent-tool-head {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13.5px;
}

.agent-tool-head strong {
  color: var(--ag-ink);
}

.agent-tool-output {
  max-height: 180px;
  margin: 6px 0 0;
  overflow: auto;
  color: var(--ag-body);
  font-size: 12.5px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.agent-tool-error {
  margin: 6px 0 0;
  color: var(--ag-danger);
  font-size: 12.5px;
  white-space: pre-wrap;
}

.agent-state.is-warn {
  background: var(--ag-warning-soft);
  color: var(--ag-warning);
}

.agent-answer {
  margin-top: 6px;
  color: var(--ag-ink);
  font-size: 14px;
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.agent-answer :deep(h1),
.agent-answer :deep(h2),
.agent-answer :deep(h3) {
  margin: 10px 0 6px;
  color: var(--ag-accent);
  font-size: 1.05em;
}

.agent-answer :deep(p) {
  margin: 6px 0;
}

.agent-answer :deep(ul),
.agent-answer :deep(ol) {
  padding-left: 20px;
  margin: 6px 0;
}

.agent-answer :deep(code) {
  padding: 1px 4px;
  font-family: var(--app-font-family-mono, monospace);
  font-size: 0.92em;
  background: var(--ag-code-bg);
  border: 1px solid var(--ag-border-soft);
  border-radius: 2px;
}

.agent-answer :deep(table) {
  margin: 8px 0;
  font-size: 13px;
  border-collapse: collapse;
}

.agent-answer :deep(th),
.agent-answer :deep(td) {
  padding: 4px 8px;
  border: 1px solid var(--ag-border-soft);
}

.agent-edit-strip {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 8px 10px;
  background: var(--ag-add-soft);
  border: 1px solid var(--ag-add);
  border-radius: 3px;
}

.agent-edit-row {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--ag-ink);
  font-size: 12.5px;
}

.agent-edit-row svg {
  flex: none;
  color: var(--ag-add);
}

.agent-edit-row__label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-foot-sync {
  color: var(--ag-muted);
  font-size: 12px;
  white-space: nowrap;
}

.agent-text-btn.is-danger {
  color: var(--ag-danger);
}

.agent-foot-model {
  width: 180px;
}

.agent-send:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.agent-dialog {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.agent-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: var(--ag-body);
  font-size: 12.5px;
}

.agent-field.is-inline {
  flex-direction: row;
  align-items: center;
  gap: 8px;
}

.agent-field input,
.agent-field select {
  height: 32px;
  padding: 0 10px;
  color: var(--ag-ink);
  font-size: 13px;
  background: var(--ag-paper);
  border: 1px solid var(--ag-border);
  border-radius: 3px;
  outline: 0;
}

.agent-field input:focus,
.agent-field select:focus {
  border-color: var(--ag-focus);
}

.agent-dialog__actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 4px;
}

.agent-session-list,
.agent-version-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 420px;
  overflow: auto;
}

.agent-session-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid var(--ag-border-soft);
  border-radius: 3px;
}

.agent-session-row.is-active {
  background: var(--ag-selection);
  border-color: var(--ag-accent);
}

.agent-session-row__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 0;
  color: var(--ag-ink);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  background: none;
  border: 0;
}

.agent-session-row__main span {
  overflow: hidden;
  color: var(--ag-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-version-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 10px;
  border: 1px solid var(--ag-border-soft);
  border-radius: 3px;
}

.agent-version-row__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  color: var(--ag-ink);
  font-size: 13px;
}

.agent-version-row__main span,
.agent-version-row__main small {
  color: var(--ag-muted);
  font-size: 12px;
}

.agent-version-compare {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 12px;
}

.agent-version-compare__row {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 12px;
}

.agent-version-diff pre {
  max-height: 260px;
  padding: 10px;
  overflow: auto;
  font-family: var(--app-font-family-mono, monospace);
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  background: var(--ag-code-bg);
  border: 1px solid var(--ag-border-soft);
  border-radius: 3px;
}

@media (max-width: 1100px) {
  .agent-workbench {
    grid-template-columns: 220px minmax(0, 1fr) 7px var(--agent-chat-width, 392px);
  }

  .agent-page.is-single-pane .agent-workbench {
    grid-template-columns: 220px minmax(0, 1fr);
  }

  .agent-foot-context,
  .agent-foot-model,
  .agent-foot-ask {
    display: none;
  }
}

@media (max-width: 900px) {
  .agent-tree {
    display: none;
  }

  .agent-workbench {
    grid-template-columns: minmax(0, 1fr) 7px var(--agent-chat-width, 392px);
  }

  .agent-page.is-single-pane .agent-workbench {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
