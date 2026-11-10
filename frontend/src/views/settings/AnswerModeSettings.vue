<template>
  <section class="block">
    <header class="block__header">
      <div>
        <h3>默认回答模板</h3>
        <p>新会话默认使用的 RAG 回答结构；当前会话可在输入栏临时切换，不影响这里的全局默认。</p>
      </div>
    </header>

    <div class="mode-list">
      <button type="button" class="mode-card" :class="{ active: current === 'study' }" @click="select('study')">
        <span class="mode-title">学习卡</span>
        <span class="mode-desc">结论、背景、原理拆解、例题应用、易错点，适合打基础。仅 RAG 模式生效。</span>
      </button>
      <button type="button" class="mode-card" :class="{ active: current === 'review' }" @click="select('review')">
        <span class="mode-title">复习卡</span>
        <span class="mode-desc">考点清单、易错辨析、2 道自测题，适合考前快速回顾。仅 RAG 模式生效。</span>
      </button>
    </div>

    <header class="block__header">
      <div>
        <h3>默认检索方式</h3>
        <p>新会话默认的检索方式；当前会话可在输入栏临时切换，不影响这里的全局默认。直连模式需先在「模型与引擎 → 直连模型」中配置。</p>
      </div>
    </header>

    <div class="mode-list">
      <button type="button" class="mode-card" :class="{ active: retrieval === 'rag' }" @click="selectRetrieval('rag')">
        <span class="mode-title">RAG 检索</span>
        <span class="mode-desc">先检索知识库再回答，带引用；可切换学习/复习模板。</span>
      </button>
      <button type="button" class="mode-card" :class="{ active: retrieval === 'direct' }" @click="selectRetrieval('direct')">
        <span class="mode-title">直连模型</span>
        <span class="mode-desc">不检索知识库，直接用思考模型回答；可显式开启联网搜索，网页来源附在答案末尾。</span>
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useSettingsStore } from '@/stores/settings'

const settingsStore = useSettingsStore()
const current = computed(() => settingsStore.answerMode)
const retrieval = computed(() => settingsStore.retrievalMode)

function select(mode: 'study' | 'review') {
  settingsStore.setDefaultAnswerMode(mode)
}

function selectRetrieval(mode: 'rag' | 'direct') {
  settingsStore.setDefaultRetrievalMode(mode)
}
</script>

<style scoped>
.block {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.block__header h3 {
  margin: 0 0 6px;
  font-size: 16px;
  font-weight: 600;
}

.block__header p {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.mode-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.mode-card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  padding: 14px 16px;
  border: 1px solid var(--td-component-border);
  border-radius: 8px;
  background: var(--td-bg-color-container);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.2s ease, background 0.2s ease;
}

.mode-card:hover {
  border-color: var(--td-brand-color);
}

.mode-card.active {
  border-color: var(--td-brand-color);
  background: var(--td-brand-color-light);
}

.mode-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.mode-desc {
  font-size: 12px;
  line-height: 1.6;
  color: var(--td-text-color-secondary);
}
</style>
