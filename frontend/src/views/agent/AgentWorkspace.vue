<template>
  <div class="agent-page" :class="{ 'is-chat-collapsed': chatCollapsed }">
    <PageTopbar :title="$t('agentWorkspace.title')" :meta="$t('agentWorkspace.meta')">
      <template #actions>
        <button type="button" class="agent-action" @click="notAvailable">
          {{ $t('agentWorkspace.newTask') }}
        </button>
        <button type="button" class="agent-action" @click="notAvailable">
          {{ $t('agentWorkspace.history') }}
        </button>
        <button type="button" class="agent-action" @click="notAvailable">
          {{ $t('agentWorkspace.versions') }}
        </button>
        <button
          type="button"
          class="agent-action"
          :aria-pressed="!chatCollapsed"
          @click="chatCollapsed = !chatCollapsed"
        >
          {{ chatCollapsed ? $t('agentWorkspace.expand') : $t('agentWorkspace.collapse') }}
        </button>
      </template>
    </PageTopbar>

    <div class="agent-workbench">
      <aside class="agent-pane agent-tree" :aria-label="$t('agentWorkspace.treeAria')">
        <div class="agent-tree__head">
          <span>{{ $t('agentWorkspace.treeRoot') }}</span>
          <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.newDocument')" @click="notAvailable">＋</button>
        </div>
        <div class="agent-tree__folder"><span>⌄</span>{{ $t('agentWorkspace.treeRequirements') }}<em>8</em></div>
        <button type="button" class="agent-tree__doc is-active">
          <span>▤</span><span>{{ $t('agentWorkspace.treeMain') }}</span><i />
        </button>
        <button type="button" class="agent-tree__doc" @click="notAvailable"><span>▤</span>{{ $t('agentWorkspace.treeCache') }}</button>
        <button type="button" class="agent-tree__doc" @click="notAvailable"><span>▤</span>{{ $t('agentWorkspace.treeChecklist') }}</button>
        <div class="agent-tree__folder is-muted"><span>›</span>{{ $t('agentWorkspace.treeCards') }}<em>12</em></div>
        <div class="agent-tree__folder is-muted"><span>›</span>{{ $t('agentWorkspace.treeAppendix') }}<em>3</em></div>
        <div class="agent-tree__divider" />
        <div class="agent-tree__folder"><span>⌄</span>{{ $t('agentWorkspace.treeTech') }}</div>
        <button type="button" class="agent-tree__doc" @click="notAvailable">
          <span>▤</span>{{ $t('agentWorkspace.treeVersionDoc') }}
        </button>
        <div class="agent-version-card">
          <strong>{{ $t('agentWorkspace.currentVersion') }}</strong>
          <span>main · 2026-11-26 20:42</span>
          <small>{{ $t('agentWorkspace.lastCommit') }}</small>
        </div>
      </aside>

      <section class="agent-pane agent-editor" :aria-label="$t('agentWorkspace.editorAria')">
        <div class="agent-tabs">
          <button type="button" class="agent-tab is-active">
            {{ $t('agentWorkspace.tabMain') }}<i />
          </button>
          <button type="button" class="agent-tab" @click="notAvailable">{{ $t('agentWorkspace.tabSummary') }}</button>
          <span class="agent-tabs__spacer" />
          <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.splitEditor')" @click="notAvailable">◫</button>
          <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.moreActions')" @click="notAvailable">⋯</button>
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
              {{ line.text }}
            </span>
          </div>
        </div>
      </section>

      <aside v-show="!chatCollapsed" class="agent-pane agent-chat" :aria-label="$t('agentWorkspace.chatAria')">
        <header class="agent-chat__head">
          <div class="agent-avatar">W</div>
          <div class="agent-chat__identity">
            <strong>{{ $t('agentWorkspace.chatTitle') }}</strong>
            <span>{{ $t('agentWorkspace.chatSub') }}</span>
          </div>
          <div class="agent-chat__actions">
            <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.newTask')" @click="notAvailable">＋</button>
            <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.history')" @click="notAvailable">◷</button>
            <button type="button" class="agent-icon-btn" :aria-label="$t('agentWorkspace.collapse')" @click="chatCollapsed = true">⟩</button>
          </div>
        </header>

        <div class="agent-chips">
          <span class="agent-chip is-accent">{{ $t('agentWorkspace.chipEdit') }}</span>
          <span class="agent-chip">{{ $t('agentWorkspace.chipRoute') }}</span>
          <span class="agent-chip">{{ $t('agentWorkspace.chipScope') }}</span>
        </div>

        <div class="agent-chat__body">
          <div class="agent-message is-user">
            <div class="agent-message__who">{{ $t('agentWorkspace.you') }}</div>
            <div class="agent-bubble">{{ $t('agentWorkspace.userMessage') }}</div>
          </div>
          <div class="agent-message">
            <div class="agent-message__who">{{ $t('agentWorkspace.chatTitle') }}<span class="agent-chip is-accent">{{ $t('agentWorkspace.chipEdit') }}</span></div>
            <div class="agent-bubble">{{ $t('agentWorkspace.agentReply') }}</div>
          </div>

          <article class="agent-tool">
            <header class="agent-tool__head">
              <span>▤</span><strong>{{ $t('agentWorkspace.toolRead') }}</strong>
              <code>{{ $t('agentWorkspace.toolReadFn') }}</code>
              <span class="agent-state is-done">{{ $t('agentWorkspace.done') }}</span>
            </header>
            <div class="agent-tool__body">
              <div><span>{{ $t('agentWorkspace.riskLevel') }}</span><p>{{ $t('agentWorkspace.toolReadRisk') }}</p></div>
              <div><span>{{ $t('agentWorkspace.returned') }}</span><p>{{ $t('agentWorkspace.toolReadReturn') }}</p></div>
            </div>
          </article>

          <article class="agent-tool">
            <header class="agent-tool__head">
              <span>✎</span><strong>{{ $t('agentWorkspace.toolEdit') }}</strong>
              <code>{{ $t('agentWorkspace.toolEditFn') }}</code>
              <span class="agent-state is-done">{{ $t('agentWorkspace.toolEditState') }}</span>
            </header>
            <div class="agent-tool__body">
              <div><span>{{ $t('agentWorkspace.location') }}</span><p>{{ $t('agentWorkspace.toolEditPos') }}</p></div>
              <div><span>{{ $t('agentWorkspace.source') }}</span><p>{{ $t('agentWorkspace.toolEditSource') }}</p></div>
            </div>
            <footer class="agent-tool__foot">
              <button type="button" class="agent-text-btn" @click="undoEdit">{{ $t('agentWorkspace.undo') }}</button>
            </footer>
          </article>

          <div class="agent-message">
            <div class="agent-message__who">{{ $t('agentWorkspace.chatTitle') }}</div>
            <div class="agent-bubble">{{ $t('agentWorkspace.agentReply2') }}</div>
          </div>

          <article class="agent-diff">
            <header class="agent-diff__head">{{ $t('agentWorkspace.diffTitle') }}<span>+14 −6</span></header>
            <div class="agent-diff__body">
              <div class="agent-diff__line"><span> </span>## 7. 非功能需求</div>
              <div class="agent-diff__line is-del"><span>−</span>## 附录 C：决策点裁决记录</div>
              <div class="agent-diff__line is-del"><span>−</span>| # | 决策点 | 裁决 |</div>
              <div class="agent-diff__line is-add"><span>+</span>&gt; 附录 C 已拆分至独立文档，此处保留回链。</div>
              <div class="agent-diff__line is-add"><span>+</span>详见 [附录C_决策点裁决记录.md]</div>
              <div class="agent-diff__line is-add"><span>+</span>【新文档】附录C_决策点裁决记录.md</div>
            </div>
          </article>

          <article class="agent-pending" :class="{ 'is-confirmed': pendingConfirmed }">
            <header>{{ pendingConfirmed ? $t('agentWorkspace.confirmed') : $t('agentWorkspace.pendingTitle') }}</header>
            <p><code>pending_task_id</code> pt_9f3c2a41-d08e-4b7f</p>
            <p><code>confirm_token</code> {{ pendingConfirmed ? $t('agentWorkspace.tokenIssued') : $t('agentWorkspace.tokenPending') }}</p>
            <p>{{ $t('agentWorkspace.pendingImpact') }}</p>
            <div v-if="!pendingConfirmed" class="agent-pending__actions">
              <button type="button" class="agent-btn is-primary" @click="confirmTask">{{ $t('agentWorkspace.confirm') }}</button>
              <button type="button" class="agent-btn" @click="notAvailable">{{ $t('agentWorkspace.reject') }}</button>
              <button type="button" class="agent-btn" @click="notAvailable">{{ $t('agentWorkspace.expire') }}</button>
            </div>
          </article>
        </div>

        <div class="agent-composer">
          <div class="agent-composer__mode">
            <span class="agent-chip is-accent">{{ $t('agentWorkspace.agentMode') }}</span>
            <span class="agent-muted">{{ $t('agentWorkspace.askMode') }}</span>
          </div>
          <textarea
            :value="draft"
            :placeholder="$t('agentWorkspace.composerPlaceholder')"
            @input="draft = ($event.target as HTMLTextAreaElement).value"
          />
          <div class="agent-composer__foot">
            <span>{{ $t('agentWorkspace.atScope') }}</span>
            <span>{{ $t('agentWorkspace.context') }}</span>
            <span class="agent-composer__spacer" />
            <span>{{ $t('agentWorkspace.model') }}</span>
            <button type="button" class="agent-send" @click="notAvailable">{{ $t('agentWorkspace.send') }}</button>
          </div>
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import PageTopbar from '@/components/layout/PageTopbar.vue'

type LineKind = 'normal' | 'is-add' | 'is-del' | 'is-h1' | 'is-h2' | 'is-quote'

type EditorLine = { key: string; no: number; text: string; kind?: LineKind; tag?: boolean }

const { t } = useI18n()
const chatCollapsed = ref(false)
const pendingConfirmed = ref(false)
const draft = ref('')

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
  { key: 'l15', no: 15, text: '统一使用 JSON Schema（兼容 MCP inputSchema）。L2/L3 工具 dry_run=true 只返回 Diff 与 pending_task_id，不返回可执行 token。' },
  { key: 'l16', no: 16, text: '附录 C 原文内容……（Agent 已准备拆分，等待确认）', kind: 'is-del' },
  { key: 'l17', no: 17, text: '> 附录 C 已拆分至独立文档，此处保留回链：', kind: 'is-add' },
  { key: 'l18', no: 18, text: '详见 [附录C_决策点裁决记录.md]（同目录）', kind: 'is-add' },
  { key: 'l19', no: 19, text: '' },
  { key: 'l20', no: 20, text: '## 8. 分期落地建议', kind: 'is-h2' },
  { key: 'l21', no: 21, text: 'P0 文档编辑 Agent · P1 chunk 联动 · P2 归档与协作能力。' },
]

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
</script>

<style lang="less" scoped>
.agent-page {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 0 24px 24px;
  overflow: hidden;
}

.agent-workbench {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: 250px minmax(0, 1fr) 392px;
  gap: 1px;
  overflow: hidden;
  border: 1px solid var(--w-hairline, var(--td-component-stroke));
  border-radius: var(--w-panel-radius, 16px);
  background: var(--w-hairline, var(--td-component-stroke));
}

.agent-page.is-chat-collapsed .agent-workbench {
  grid-template-columns: 250px minmax(0, 1fr);
}

.agent-pane {
  min-height: 0;
  overflow: hidden;
  background: var(--td-bg-color-container);
}

.agent-action {
  height: 30px;
  padding: 0 10px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--w-control-radius, 8px);
  background: transparent;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  cursor: pointer;
}

.agent-action:hover {
  border-color: var(--w-page-accent, var(--td-brand-color));
  color: var(--td-text-color-primary);
}

.agent-icon-btn {
  width: 26px;
  height: 26px;
  display: inline-grid;
  place-items: center;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--td-text-color-placeholder);
  font-size: 15px;
  cursor: pointer;
}

.agent-icon-btn:hover {
  background: var(--td-bg-color-container-hover);
  color: var(--td-text-color-primary);
}

/* tree */
.agent-tree {
  overflow-y: auto;
  padding: 14px 10px;
  background: var(--td-bg-color-secondarycontainer);
}

.agent-tree__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 6px 10px;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
  font-weight: 600;
}

.agent-tree__folder,
.agent-tree__doc {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  min-height: 32px;
  padding: 4px 9px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.agent-tree__folder em {
  margin-left: auto;
  color: var(--td-text-color-placeholder);
  font-size: 11px;
  font-style: normal;
}

.agent-tree__folder.is-muted,
.agent-tree__doc {
  color: var(--td-text-color-placeholder);
}

.agent-tree__doc:hover,
.agent-tree__folder:hover {
  background: var(--td-bg-color-container-hover);
  color: var(--td-text-color-primary);
}

.agent-tree__doc.is-active {
  background: var(--w-page-accent-soft, var(--td-brand-color-light));
  color: var(--w-page-accent, var(--td-brand-color));
  font-weight: 600;
}

.agent-tree__doc i {
  width: 6px;
  height: 6px;
  margin-left: auto;
  border-radius: 50%;
  background: var(--td-warning-color);
}

.agent-tree__divider {
  height: 1px;
  margin: 10px 6px;
  background: var(--td-component-stroke);
}

.agent-version-card {
  display: flex;
  flex-direction: column;
  gap: 3px;
  margin: 14px 6px 0;
  padding: 10px 12px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--w-control-radius, 8px);
  background: var(--td-bg-color-container);
  color: var(--td-text-color-placeholder);
  font-size: 11px;
}

.agent-version-card strong {
  color: var(--td-text-color-primary);
  font-size: 12px;
}

/* editor */
.agent-editor {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.agent-tabs {
  flex: none;
  height: 38px;
  display: flex;
  align-items: stretch;
  border-bottom: 1px solid var(--td-component-stroke);
  background: var(--td-bg-color-secondarycontainer);
}

.agent-tab {
  max-width: 280px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  overflow: hidden;
  padding: 0 12px;
  border: 0;
  border-right: 1px solid var(--td-component-stroke);
  background: transparent;
  color: var(--td-text-color-placeholder);
  font-size: 12.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
}

.agent-tab.is-active {
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
  box-shadow: inset 0 2px 0 var(--w-page-accent, var(--td-brand-color));
}

.agent-tab i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--td-warning-color);
}

.agent-tabs__spacer {
  flex: 1;
}

.agent-breadcrumb {
  flex: none;
  padding: 6px 16px;
  border-bottom: 1px solid var(--td-component-stroke);
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}

.agent-breadcrumb b {
  color: var(--td-text-color-primary);
  font-weight: 600;
}

.agent-breadcrumb span:last-child {
  color: var(--w-page-accent, var(--td-brand-color));
}

.agent-code {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 16px 0 40px;
  color: var(--td-text-color-primary);
  font-family: var(--app-font-family-mono, monospace);
  font-size: 12.8px;
  line-height: 1.85;
}

.agent-line {
  display: flex;
  min-height: 24px;
}

.agent-line__no {
  width: 48px;
  flex: none;
  padding-right: 12px;
  color: var(--td-text-color-placeholder);
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
  background: color-mix(in srgb, var(--w-page-accent, var(--td-brand-color)) 12%, var(--td-bg-color-container));
  box-shadow: inset 3px 0 0 var(--w-page-accent, var(--td-brand-color));
}

.agent-line.is-del {
  background: color-mix(in srgb, var(--td-error-color) 12%, var(--td-bg-color-container));
  box-shadow: inset 3px 0 0 var(--td-error-color);
}

.agent-line.is-del .agent-line__text {
  color: var(--td-error-color);
  text-decoration: line-through;
}

.agent-line.is-h1 .agent-line__text {
  font-size: 1.35em;
  font-weight: 700;
}

.agent-line.is-h2 .agent-line__text {
  font-size: 1.12em;
  font-weight: 700;
}

.agent-line.is-quote .agent-line__text {
  color: var(--td-text-color-secondary);
}

.agent-insert-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-right: 8px;
  padding: 1px 8px;
  border: 1px solid var(--w-page-accent-line, var(--td-brand-color-light));
  border-radius: 999px;
  background: var(--td-bg-color-container);
  color: var(--w-page-accent, var(--td-brand-color));
  font-size: 10.5px;
  font-weight: 700;
}

/* chat */
.agent-chat {
  display: flex;
  flex-direction: column;
  background: var(--td-bg-color-secondarycontainer);
}

.agent-chat__head {
  flex: none;
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--td-component-stroke);
}

.agent-avatar {
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  border-radius: 9px;
  background: var(--w-page-accent, var(--td-brand-color));
  color: var(--w-page-accent-ink, var(--td-text-color-anti));
  font-size: 13px;
  font-weight: 700;
}

.agent-chat__identity {
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.agent-chat__identity strong {
  color: var(--td-text-color-primary);
  font-size: 13px;
}

.agent-chat__identity span {
  overflow: hidden;
  color: var(--td-text-color-placeholder);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-chat__actions {
  margin-left: auto;
  display: flex;
  gap: 2px;
}

.agent-chips {
  flex: none;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 8px 14px;
  border-bottom: 1px solid var(--td-component-stroke);
}

.agent-chip {
  display: inline-flex;
  align-items: center;
  padding: 2px 9px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 999px;
  color: var(--td-text-color-placeholder);
  font-size: 11px;
  white-space: nowrap;
}

.agent-chip.is-accent {
  border-color: var(--w-page-accent-line, var(--td-brand-color-light));
  background: var(--w-page-accent-soft, var(--td-brand-color-light));
  color: var(--w-page-accent, var(--td-brand-color));
  font-weight: 600;
}

.agent-chat__body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
}

.agent-message {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.agent-message.is-user {
  align-items: flex-end;
}

.agent-message__who {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--td-text-color-placeholder);
  font-size: 11px;
}

.agent-bubble {
  max-width: 94%;
  padding: 9px 12px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 10px;
  background: var(--td-bg-color-container);
  color: var(--td-text-color-secondary);
  font-size: 12.5px;
  line-height: 1.65;
}

.agent-message.is-user .agent-bubble {
  border-color: var(--w-page-accent, var(--td-brand-color));
  background: var(--w-page-accent, var(--td-brand-color));
  color: var(--w-page-accent-ink, var(--td-text-color-anti));
  border-bottom-right-radius: 4px;
}

.agent-tool,
.agent-diff,
.agent-pending {
  overflow: hidden;
  border: 1px solid var(--td-component-stroke);
  border-radius: 9px;
  background: var(--td-bg-color-container);
}

.agent-tool__head {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 8px 11px;
  color: var(--td-text-color-primary);
  font-size: 12px;
}

.agent-tool__head code {
  color: var(--td-text-color-placeholder);
  font-size: 10.5px;
}

.agent-state {
  margin-left: auto;
  padding: 1px 7px;
  border-radius: 999px;
  font-size: 10.5px;
}

.agent-state.is-done {
  background: var(--w-page-accent-soft, var(--td-brand-color-light));
  color: var(--w-page-accent, var(--td-brand-color));
}

.agent-tool__body {
  padding: 0 11px 10px;
  color: var(--td-text-color-placeholder);
  font-size: 11.5px;
  line-height: 1.65;
}

.agent-tool__body > div {
  display: flex;
  gap: 8px;
}

.agent-tool__body span {
  width: 54px;
  flex: none;
}

.agent-tool__body p {
  margin: 0;
  color: var(--td-text-color-secondary);
}

.agent-tool__foot {
  padding: 7px 11px;
  border-top: 1px solid var(--td-component-stroke);
}

.agent-text-btn {
  padding: 0;
  border: 0;
  background: none;
  color: var(--w-page-accent, var(--td-brand-color));
  font-size: 11.5px;
  cursor: pointer;
}

.agent-diff__head {
  padding: 8px 11px;
  border-bottom: 1px solid var(--td-component-stroke);
  color: var(--td-text-color-primary);
  font-size: 12px;
}

.agent-diff__head span {
  float: right;
  color: var(--td-text-color-placeholder);
  font-family: var(--app-font-family-mono, monospace);
}

.agent-diff__body {
  padding: 6px 0;
  font-family: var(--app-font-family-mono, monospace);
  font-size: 11.5px;
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
  color: var(--td-text-color-placeholder);
}

.agent-diff__line.is-add {
  background: color-mix(in srgb, var(--w-page-accent, var(--td-brand-color)) 12%, var(--td-bg-color-container));
  color: var(--w-page-accent, var(--td-brand-color));
}

.agent-diff__line.is-del {
  background: color-mix(in srgb, var(--td-error-color) 12%, var(--td-bg-color-container));
  color: var(--td-error-color);
}

.agent-pending {
  padding: 11px 12px;
  border-color: var(--td-warning-color);
  background: var(--td-warning-color-light);
  color: var(--td-text-color-secondary);
  font-size: 11px;
  line-height: 1.7;
}

.agent-pending.is-confirmed {
  border-color: var(--w-page-accent-line, var(--td-brand-color-light));
  background: var(--w-page-accent-soft, var(--td-brand-color-light));
}

.agent-pending header {
  color: var(--td-warning-color);
  font-size: 12px;
  font-weight: 700;
}

.agent-pending.is-confirmed header {
  color: var(--w-page-accent, var(--td-brand-color));
}

.agent-pending p {
  margin: 5px 0 0;
}

.agent-pending code {
  color: var(--td-text-color-primary);
  font-family: var(--app-font-family-mono, monospace);
}

.agent-pending__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 9px;
}

.agent-btn {
  height: 28px;
  padding: 0 11px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 7px;
  background: var(--td-bg-color-container);
  color: var(--td-text-color-secondary);
  font-size: 12px;
  cursor: pointer;
}

.agent-btn.is-primary {
  border-color: var(--w-page-accent, var(--td-brand-color));
  background: var(--w-page-accent, var(--td-brand-color));
  color: var(--w-page-accent-ink, var(--td-text-color-anti));
  font-weight: 600;
}

.agent-composer {
  flex: none;
  padding: 10px 12px 12px;
  border-top: 1px solid var(--td-component-stroke);
  background: var(--td-bg-color-secondarycontainer);
}

.agent-composer__mode {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 7px;
}

.agent-muted {
  color: var(--td-text-color-placeholder);
  font-size: 11px;
}

.agent-composer textarea {
  width: 100%;
  height: 64px;
  padding: 9px 11px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--w-control-radius, 8px);
  outline: 0;
  resize: none;
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
  font-family: inherit;
  font-size: 12.5px;
  line-height: 1.6;
}

.agent-composer textarea:focus {
  border-color: var(--w-page-accent, var(--td-brand-color));
  box-shadow: 0 0 0 3px var(--w-page-accent-focus, transparent);
}

.agent-composer__foot {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 7px;
  color: var(--td-text-color-placeholder);
  font-size: 11px;
}

.agent-composer__spacer {
  flex: 1;
}

.agent-send {
  height: 28px;
  padding: 0 12px;
  border: 0;
  border-radius: 7px;
  background: var(--w-page-accent, var(--td-brand-color));
  color: var(--w-page-accent-ink, var(--td-text-color-anti));
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

@media (max-width: 1100px) {
  .agent-workbench {
    grid-template-columns: 220px minmax(0, 1fr);
  }

  .agent-chat {
    display: none;
  }
}

@media (max-width: 760px) {
  .agent-page {
    padding: 0 12px 12px;
  }

  .agent-workbench {
    grid-template-columns: minmax(0, 1fr);
  }

  .agent-tree {
    display: none;
  }
}
</style>
