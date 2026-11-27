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
          <button type="button" class="agent-bar-btn is-primary" :aria-label="$t('agentWorkspace.newTask')" @click="notAvailable">
            <AddIcon />
          </button>
        </t-tooltip>
        <t-tooltip :content="$t('agentWorkspace.history')" placement="bottom">
          <button type="button" class="agent-bar-btn" :aria-label="$t('agentWorkspace.history')" @click="notAvailable">
            <HistoryIcon />
          </button>
        </t-tooltip>
        <t-tooltip :content="$t('agentWorkspace.versions')" placement="bottom">
          <button type="button" class="agent-bar-btn" :aria-label="$t('agentWorkspace.versions')" @click="notAvailable">
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
          <span>{{ $t('agentWorkspace.treeRoot') }}</span>
          <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.newDocument')" @click="notAvailable">
            <AddIcon />
          </button>
        </div>

        <button type="button" class="agent-tree__row is-folder" @click="notAvailable">
          <ChevronDownIcon class="agent-tree__caret" />
          <FolderIcon class="agent-tree__icon" />
          <span>{{ $t('agentWorkspace.treeRequirements') }}</span>
          <em>8</em>
        </button>
        <button type="button" class="agent-tree__row is-child is-active" @click="notAvailable">
          <span class="agent-tree__caret" />
          <FileIcon class="agent-tree__icon" />
          <span>{{ $t('agentWorkspace.treeMain') }}</span>
          <i class="agent-tree__dot" :title="$t('agentWorkspace.treeModified')" />
        </button>
        <button type="button" class="agent-tree__row is-child" @click="notAvailable">
          <span class="agent-tree__caret" />
          <FileIcon class="agent-tree__icon" />
          <span>{{ $t('agentWorkspace.treeCache') }}</span>
        </button>
        <button type="button" class="agent-tree__row is-child" @click="notAvailable">
          <span class="agent-tree__caret" />
          <FileIcon class="agent-tree__icon" />
          <span>{{ $t('agentWorkspace.treeChecklist') }}</span>
        </button>
        <button type="button" class="agent-tree__row is-folder is-muted" @click="notAvailable">
          <ChevronRightIcon class="agent-tree__caret" />
          <FolderIcon class="agent-tree__icon" />
          <span>{{ $t('agentWorkspace.treeCards') }}</span>
          <em>12</em>
        </button>
        <button type="button" class="agent-tree__row is-folder is-muted" @click="notAvailable">
          <ChevronRightIcon class="agent-tree__caret" />
          <FolderIcon class="agent-tree__icon" />
          <span>{{ $t('agentWorkspace.treeAppendix') }}</span>
          <em>3</em>
        </button>

        <div class="agent-tree__divider" />

        <button type="button" class="agent-tree__row is-folder" @click="notAvailable">
          <ChevronDownIcon class="agent-tree__caret" />
          <FolderIcon class="agent-tree__icon" />
          <span>{{ $t('agentWorkspace.treeTech') }}</span>
        </button>
        <button type="button" class="agent-tree__row is-child" @click="notAvailable">
          <span class="agent-tree__caret" />
          <FileIcon class="agent-tree__icon" />
          <span>{{ $t('agentWorkspace.treeVersionDoc') }}</span>
        </button>

        <div class="agent-version-card">
          <strong>{{ $t('agentWorkspace.currentVersion') }}</strong>
          <span>main · 2026-11-26 20:42</span>
          <small>{{ $t('agentWorkspace.lastCommit') }}</small>
        </div>
      </aside>

      <section v-show="!editorCollapsed" class="agent-pane agent-editor" :aria-label="$t('agentWorkspace.editorAria')">
        <div class="agent-tabs">
          <button type="button" class="agent-tab is-active">
            <FileIcon />
            <span>{{ $t('agentWorkspace.tabMain') }}</span>
            <i class="agent-tab__dirty" />
          </button>
          <button type="button" class="agent-tab" @click="notAvailable">
            <span>{{ $t('agentWorkspace.tabSummary') }}</span>
          </button>
          <span class="agent-tabs__spacer" />
          <t-tooltip :content="$t('agentWorkspace.splitEditor')" placement="bottom">
            <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.splitEditor')" @click="notAvailable">
              <ColumnLayoutIcon />
            </button>
          </t-tooltip>
          <t-tooltip :content="$t('agentWorkspace.moreActions')" placement="bottom">
            <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.moreActions')" @click="notAvailable">
              <EllipsisIcon />
            </button>
          </t-tooltip>
          <t-tooltip :content="$t('agentWorkspace.collapseEditor')" placement="bottom">
            <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.collapseEditor')" @click="editorCollapsed = true">
              <ChevronRightIcon />
            </button>
          </t-tooltip>
        </div>
        <div class="agent-breadcrumb">
          {{ $t('agentWorkspace.breadcrumbRoot') }} › <b>{{ $t('agentWorkspace.tabMain') }}</b> ›
          <span>{{ $t('agentWorkspace.breadcrumbInsert') }}</span>
        </div>
        <div class="agent-code" role="document">
          <div v-for="line in editorLines" :key="line.key" class="agent-line" :class="line.kind">
            <span class="agent-line__no">{{ line.no }}</span>
            <span class="agent-line__text">
              <span v-if="line.tag" class="agent-insert-tag">{{ $t('agentWorkspace.insertTag') }}</span>
              <template v-if="line.parts">
                <template v-for="(part, partIndex) in line.parts" :key="partIndex">
                  <code v-if="part.code" class="agent-inline-code">{{ part.text }}</code>
                  <template v-else>{{ part.text }}</template>
                </template>
              </template>
              <template v-else>{{ line.text }}</template>
            </span>
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
          <div class="agent-avatar">W</div>
          <div class="agent-chat__identity">
            <strong>{{ $t('agentWorkspace.chatTitle') }}</strong>
            <span>{{ $t('agentWorkspace.chatSub') }}</span>
          </div>
          <div class="agent-chat__actions">
            <t-tooltip :content="$t('agentWorkspace.newTask')" placement="bottom">
              <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.newTask')" @click="notAvailable">
                <AddIcon />
              </button>
            </t-tooltip>
            <t-tooltip :content="$t('agentWorkspace.history')" placement="bottom">
              <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.history')" @click="notAvailable">
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

        <div class="agent-chat__body">
          <div class="agent-timeline">
            <section class="agent-turn is-user">
            <span class="agent-node is-user" aria-hidden="true" />
            <div class="agent-turn__content">
              <div class="agent-turn__meta">{{ $t('agentWorkspace.you') }}</div>
              <div class="agent-user-box">{{ $t('agentWorkspace.userMessage') }}</div>
            </div>
          </section>

          <section class="agent-turn is-agent">
            <span class="agent-node is-agent" aria-hidden="true" />
            <div class="agent-turn__content">
              <div class="agent-turn__meta">{{ $t('agentWorkspace.chatTitle') }}</div>
              <p class="agent-turn__text">{{ $t('agentWorkspace.agentReply') }}</p>
            </div>
          </section>

          <section class="agent-turn">
            <span class="agent-node is-success" aria-hidden="true" />
            <div class="agent-turn__content">
              <div class="agent-node__head">
                <FileIcon />
                <strong>{{ $t('agentWorkspace.toolRead') }}</strong>
                <span class="agent-state is-done">{{ $t('agentWorkspace.done') }}</span>
              </div>
              <div class="agent-node__panel">
                <p>{{ $t('agentWorkspace.toolReadRisk') }}</p>
                <p>{{ $t('agentWorkspace.toolReadReturn') }}</p>
              </div>
            </div>
          </section>

          <section class="agent-turn">
            <span class="agent-node is-write" aria-hidden="true" />
            <div class="agent-turn__content">
              <div class="agent-node__head">
                <EditIcon />
                <strong>{{ $t('agentWorkspace.toolEdit') }}</strong>
                <span class="agent-state is-done">{{ $t('agentWorkspace.toolEditState') }}</span>
              </div>
              <div class="agent-node__panel">
                <p>{{ $t('agentWorkspace.toolEditPos') }}</p>
                <p>{{ $t('agentWorkspace.toolEditSource') }}</p>
              </div>
              <footer class="agent-node__foot">
                <button type="button" class="agent-text-btn" @click="undoEdit">{{ $t('agentWorkspace.undo') }}</button>
              </footer>
            </div>
          </section>

          <section class="agent-turn is-agent">
            <span class="agent-node is-agent" aria-hidden="true" />
            <div class="agent-turn__content">
              <div class="agent-turn__meta">{{ $t('agentWorkspace.chatTitle') }}</div>
              <p class="agent-turn__text">{{ $t('agentWorkspace.agentReply2') }}</p>
            </div>
          </section>

          <section class="agent-turn">
            <span class="agent-node is-diff" aria-hidden="true" />
            <div class="agent-turn__content">
              <div class="agent-node__head">
                <GitBranchIcon />
                <strong>{{ $t('agentWorkspace.diffTitle') }}</strong>
                <span class="agent-node__stat">+14 −6</span>
              </div>
              <div class="agent-diff-box">
                <div class="agent-diff__line"><span> </span>## 7. 非功能需求</div>
                <div class="agent-diff__line is-del"><span>−</span>## 附录 C：决策点裁决记录</div>
                <div class="agent-diff__line is-del"><span>−</span>| # | 决策点 | 裁决 |</div>
                <div class="agent-diff__line is-add"><span>+</span>&gt; 附录 C 已拆分至独立文档，此处保留回链。</div>
                <div class="agent-diff__line is-add"><span>+</span>详见 [附录C_决策点裁决记录.md]</div>
                <div class="agent-diff__line is-add"><span>+</span>【新文档】附录C_决策点裁决记录.md</div>
              </div>
            </div>
          </section>

          <section class="agent-turn">
            <span class="agent-node" :class="pendingConfirmed ? 'is-success' : 'is-warning'" aria-hidden="true" />
            <div class="agent-turn__content">
              <article class="agent-pending-box" :class="{ 'is-confirmed': pendingConfirmed }">
                <header>
                  <CheckCircleIcon v-if="pendingConfirmed" />
                  <ErrorCircleIcon v-else />
                  {{ pendingConfirmed ? $t('agentWorkspace.confirmed') : $t('agentWorkspace.pendingTitle') }}
                </header>
                <div class="agent-pending__meta">
                  <p><code>pending_task_id</code> pt_9f3c2a41-d08e-4b7f</p>
                  <p><code>confirm_token</code> {{ pendingConfirmed ? $t('agentWorkspace.tokenIssued') : $t('agentWorkspace.tokenPending') }}</p>
                  <p>{{ $t('agentWorkspace.pendingImpact') }}</p>
                </div>
                <div v-if="!pendingConfirmed" class="agent-pending__actions">
                  <button type="button" class="agent-btn is-primary" @click="confirmTask">{{ $t('agentWorkspace.confirm') }}</button>
                  <button type="button" class="agent-btn is-danger" @click="notAvailable">{{ $t('agentWorkspace.reject') }}</button>
                  <button type="button" class="agent-btn" @click="notAvailable">{{ $t('agentWorkspace.expire') }}</button>
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
            :value="draft"
            :placeholder="$t('agentWorkspace.composerPlaceholder')"
            @input="draft = ($event.target as HTMLTextAreaElement).value"
          />
          <div class="agent-composer__foot">
            <span class="agent-foot-scope">{{ $t('agentWorkspace.atScope') }}</span>
            <span class="agent-foot-context">{{ $t('agentWorkspace.context') }}</span>
            <span class="agent-composer__spacer" />
            <span class="agent-foot-model">{{ $t('agentWorkspace.model') }}</span>
            <button type="button" class="agent-send" @click="notAvailable">{{ $t('agentWorkspace.send') }}</button>
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
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  AddIcon,
  ChatIcon,
  CheckCircleIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  ColumnLayoutIcon,
  EditIcon,
  EllipsisIcon,
  ErrorCircleIcon,
  FileIcon,
  FolderIcon,
  GitBranchIcon,
  HistoryIcon,
  LayoutIcon,
} from 'tdesign-icons-vue-next'
import PageTopbar from '@/components/layout/PageTopbar.vue'
import { loadPreference, savePreference } from '@/composables/preferenceStorage'

type LineKind = 'normal' | 'is-add' | 'is-del' | 'is-h1' | 'is-h2' | 'is-quote'
type LinePart = { text: string; code?: boolean }
type EditorLine = { key: string; no: number; text?: string; parts?: LinePart[]; kind?: LineKind; tag?: boolean }

const { t } = useI18n()
const chatCollapsed = ref(false)
const editorCollapsed = ref(false)
const pendingConfirmed = ref(false)
const draft = ref('')

const workbenchRef = ref<HTMLElement | null>(null)
const resizing = ref(false)
const MIN_CHAT_WIDTH = 320
const DEFAULT_CHAT_WIDTH = 392
const MIN_EDITOR_WIDTH = 420
const TREE_WIDTH = 250
const DIVIDER_WIDTH = 7

const chatWidth = ref(loadChatWidth())
const maxChatWidth = computed(() => {
  const total = workbenchRef.value?.clientWidth ?? 0
  return Math.max(MIN_CHAT_WIDTH, total - TREE_WIDTH - DIVIDER_WIDTH - MIN_EDITOR_WIDTH)
})
const workbenchStyle = computed(() => ({ '--agent-chat-width': `${Math.round(chatWidth.value)}px` }))

const editorLines: EditorLine[] = [
  { key: 'l1', no: 1, text: '知识库平台 Agent 功能需求文档（V3 定稿版）', kind: 'is-h1' },
  { key: 'l2', no: 2, text: '' },
  { key: 'l3', no: 3, text: '> 版本：V3.0（定稿版） · 日期：2026-11-26 · 状态：已定稿', kind: 'is-quote' },
  { key: 'l4', no: 4, text: '' },
  { key: 'l5', no: 5, text: '缓存设计 · 核心知识', kind: 'is-add', tag: true },
  { key: 'l6', no: 6, text: '- 三类缓存：Prompt 前缀缓存 C1、检索结果缓存 C2（15–30 min）、文档解析缓存 C3（随 content_hash 失效）', kind: 'is-add' },
  { key: 'l7', no: 7, text: '- 写后失效：文档变更采用事件驱动失效；缓存条目记录 doc_version 防脏读', kind: 'is-add' },
  { key: 'l8', no: 8, text: '- 缓存只省检索计算，不省权限：命中后仍按当前文档范围二次过滤', kind: 'is-add' },
  { key: 'l9', no: 9, text: '- 失效风暴保护：批量导入合并失效事件，重建进入低优先级队列限速执行', kind: 'is-add' },
  { key: 'l10', no: 10, text: '' },
  { key: 'l11', no: 11, text: '## 2. 上下文设计', kind: 'is-h2' },
  { key: 'l12', no: 12, text: '上下文由五层组成，按优先级注入：系统层 → 会话层 → 文档层 → 知识层 → 历史层。' },
  { key: 'l13', no: 13, text: '' },
  { key: 'l14', no: 14, text: '## 4. 工具封装', kind: 'is-h2' },
  {
    key: 'l15',
    no: 15,
    parts: [
      { text: '统一使用 JSON Schema（兼容 MCP inputSchema）。L2/L3 工具 ' },
      { text: 'dry_run=true', code: true },
      { text: ' 只返回 Diff 与 ' },
      { text: 'pending_task_id', code: true },
      { text: '，不返回可执行 token。' },
    ],
  },
  { key: 'l16', no: 16, text: '附录 C 原文内容……（Agent 已准备拆分，等待确认）', kind: 'is-del' },
  { key: 'l17', no: 17, text: '> 附录 C 已拆分至独立文档，此处保留回链：', kind: 'is-add' },
  { key: 'l18', no: 18, text: '详见 [附录C_决策点裁决记录.md]（同目录）', kind: 'is-add' },
  { key: 'l19', no: 19, text: '' },
  { key: 'l20', no: 20, text: '## 8. 分期落地建议', kind: 'is-h2' },
  { key: 'l21', no: 21, text: 'P0 文档编辑 Agent · P1 chunk 联动 · P2 归档与协作能力。' },
]

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

function handleWindowResize() {
  chatWidth.value = clampChatWidth(chatWidth.value)
}

function notAvailable() {
  MessagePlugin.info(t('agentWorkspace.backendPending'))
}

function confirmTask() {
  pendingConfirmed.value = true
  MessagePlugin.success(t('agentWorkspace.confirmPlaceholder'))
}

function undoEdit() {
  MessagePlugin.info(t('agentWorkspace.undoPlaceholder'))
}

onMounted(() => {
  chatWidth.value = clampChatWidth(chatWidth.value)
  window.addEventListener('resize', handleWindowResize)
})

onUnmounted(() => {
  window.removeEventListener('pointermove', onDragMove)
  window.removeEventListener('resize', handleWindowResize)
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
  --ag-paper: #191714;
  --ag-raised: #211e1a;
  --ag-border: #3a342c;
  --ag-ink: #f3efe8;
  --ag-body: #c8c1b5;
  --ag-muted: #978e80;
  --ag-focus: var(--ag-accent);
  --ag-warning: #dcc08c;
  --ag-warning-soft: #332b1f;
  --ag-danger: #e2a39a;
  --ag-danger-soft: #342421;
  --ag-add: #9ed3b2;
  --ag-add-soft: #1e3a2b;
  --ag-code-bg: #2a2621;
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
  border-color: var(--ag-accent);
  background: var(--ag-accent);
  color: var(--ag-accent-ink);
}

.agent-bar-btn.is-primary:hover {
  border-color: var(--ag-accent);
  background: color-mix(in srgb, var(--ag-accent) 88%, var(--ag-ink));
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
  grid-template-columns: 250px minmax(0, 1fr) 7px clamp(280px, 30vw, var(--agent-chat-width, 392px));
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

.agent-avatar {
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  border-radius: 3px;
  background: var(--ag-accent);
  color: var(--ag-accent-ink);
  font-size: 14px;
  font-weight: 700;
}

.agent-chat__identity {
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.agent-chat__identity strong {
  color: var(--ag-ink);
  font-size: 13px;
}

.agent-chat__identity span {
  overflow: hidden;
  color: var(--ag-body);
  font-size: 11px;
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
  border-color: var(--ag-accent);
  background: var(--ag-accent);
  color: var(--ag-accent-ink);
  font-weight: 600;
}

.agent-btn.is-primary:hover {
  background: color-mix(in srgb, var(--ag-accent) 88%, var(--ag-ink));
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
  border: 1px solid var(--ag-accent);
  border-radius: 3px;
  background: var(--ag-accent);
  color: var(--ag-accent-ink);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.agent-send:hover {
  background: color-mix(in srgb, var(--ag-accent) 88%, var(--ag-ink));
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

@media (max-width: 1100px) {
  .agent-workbench {
    grid-template-columns: 220px minmax(0, 1fr) 7px clamp(280px, 30vw, var(--agent-chat-width, 392px));
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
    grid-template-columns: minmax(0, 1fr) 7px clamp(280px, 30vw, var(--agent-chat-width, 392px));
  }

  .agent-page.is-single-pane .agent-workbench {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
