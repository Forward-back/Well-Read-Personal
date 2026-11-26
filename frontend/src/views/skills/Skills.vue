<template>
  <div class="skills-page">
    <div class="skills-page__inner">
      <SkillCatalogView
        v-if="view === 'catalog'"
        :catalog="catalog"
        :loading="loading"
        @create="openCreate"
        @detail="openDetail"
        @edit="openEdit"
        @toggle="toggleSkill"
        @delete="removeSkill"
      />
      <SkillEditorView
        v-else-if="view === 'editor'"
        :item="editing"
        @cancel="cancelEditor"
        @saved="onSaved"
      />
      <SkillDetailView
        v-else-if="active"
        :item="active"
        @back="view = 'catalog'"
        @edit="openEdit"
        @toggle="toggleSkill"
        @delete="removeSkill"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import {
  deleteSkillCatalog,
  listSkillCatalog,
  setSkillCatalogEnabled,
  type SkillCatalogItem,
} from '@/api/skill'
import SkillCatalogView from './SkillCatalogView.vue'
import SkillEditorView from './SkillEditorView.vue'
import SkillDetailView from './SkillDetailView.vue'

const { t } = useI18n()
const catalog = ref<SkillCatalogItem[]>([])
const loading = ref(false)
const view = ref<'catalog' | 'editor' | 'detail'>('catalog')
const active = ref<SkillCatalogItem | null>(null)
const editing = ref<SkillCatalogItem | null>(null)
const returnView = ref<'catalog' | 'detail'>('catalog')

const load = async () => {
  loading.value = true
  try {
    const response = await listSkillCatalog()
    catalog.value = Array.isArray(response.data) ? response.data : []
    if (active.value) {
      active.value = catalog.value.find((item) => item.id === active.value?.id) || active.value
    }
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('skillCatalog.loadFailed'))
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  editing.value = null
  returnView.value = 'catalog'
  view.value = 'editor'
}

const openDetail = (item: SkillCatalogItem) => {
  active.value = item
  view.value = 'detail'
}

const openEdit = (item: SkillCatalogItem) => {
  editing.value = item
  returnView.value = view.value === 'detail' ? 'detail' : 'catalog'
  active.value = item
  view.value = 'editor'
}

const cancelEditor = () => {
  view.value = returnView.value === 'detail' && active.value ? 'detail' : 'catalog'
  editing.value = null
}

const onSaved = async (id: string) => {
  await load()
  const saved = catalog.value.find((item) => item.id === id)
  editing.value = null
  if (saved) {
    active.value = saved
    view.value = 'detail'
  } else {
    view.value = 'catalog'
  }
}

const toggleSkill = async (item: SkillCatalogItem) => {
  try {
    await setSkillCatalogEnabled(item.id, item.enabled === false)
    await load()
    if (active.value?.id === item.id) {
      active.value = catalog.value.find((entry) => entry.id === item.id) || active.value
    }
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('skillCatalog.updateFailed'))
  }
}

const removeSkill = (item: SkillCatalogItem) => {
  const dialog = DialogPlugin.confirm({
    header: t('skillCatalog.deleteConfirmTitle'),
    body: t('skillCatalog.deleteConfirmBody', { name: item.name }),
    theme: 'danger',
    confirmBtn: t('common.delete'),
    cancelBtn: t('common.cancel'),
    onConfirm: async () => {
      try {
        await deleteSkillCatalog(item.id)
        MessagePlugin.success(t('skillCatalog.deleteSuccess'))
        if (active.value?.id === item.id) {
          active.value = null
          view.value = 'catalog'
        }
        await load()
      } catch (err: any) {
        MessagePlugin.error(err?.message || t('skillCatalog.deleteFailed'))
      } finally {
        dialog.destroy()
      }
    },
  })
}

onMounted(load)
</script>

<style lang="less">
.skills-page {
  --skp-line: var(--w-hairline, #e3dcce);
  --skp-line-soft: color-mix(in srgb, var(--w-hairline, #eee8dd) 55%, white);
  --skp-ink: #4b443b;
  --skp-ink-2: #7c7264;
  --skp-ink-3: #a79c8b;
  --skp-accent: var(--w-page-accent, #5c6b51);
  --skp-accent-deep: var(--w-page-accent-active, #46523c);
  --skp-accent-bg: var(--w-page-accent-soft, #eef1ea);
  --skp-amber: #96794b;
  --skp-amber-bg: #f6f0e5;
  --skp-clay: #a2685c;
  --skp-clay-bg: #f7eeeb;
  --skp-surface-tint: #f8f5ee;
  --skp-surface-sage: #f1f3ee;
  --skp-preview: #fcfaf5;
  --skp-surface: #fff;
  --skp-input: #fffdf9;
  --skp-hover: #fcfbf7;
  --skp-detail-head: linear-gradient(180deg, #f4f6f0 0%, #fff 64%);
  --skp-mono: "JetBrains Mono", "SFMono-Regular", Consolas, "Liberation Mono", "PingFang SC", monospace;
  --skp-serif: "Noto Serif SC", "Source Han Serif SC", Georgia, "Songti SC", serif;
  height: 100%;
  overflow-y: auto;
  background: transparent;
  color: var(--skp-ink);
  font-size: 14px;
  line-height: 1.6;
}

.skills-page__inner {
  min-height: 100%;
  width: 100%;
  background: transparent;
  padding: 0 0 72px;
}

.skp-view { max-width: none; margin: 0; }
.skp-view > .skp-editor-card,
.skp-view > .skp-detail-actions { margin-top: 24px; }

html[theme-mode="light"] .skills-page {
  --skp-ink: var(--w-detail-text-primary, #4b443b);
  --skp-ink-2: var(--w-detail-text-secondary, #7c7264);
  --skp-ink-3: var(--w-detail-text-placeholder, #a79c8b);
}

.skp-page-head { display: flex; align-items: flex-start; gap: 16px; margin-bottom: 20px; flex-wrap: wrap; }
.skp-page-head h1 { margin: 0; font-size: 22px; font-weight: 700; letter-spacing: 0.5px; }
.skp-sub { margin-top: 4px; color: var(--skp-ink-2); font-size: 13px; }
.skp-spacer { flex: 1; }
.skp-faint { color: var(--skp-ink-3); font-size: 12px; }
.skp-muted { color: var(--skp-ink-2); }

.skp-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: 1px solid var(--skp-line);
  background: var(--skp-surface);
  color: var(--skp-ink-2);
  padding: 6px 14px;
  font-size: 14px;
  border-radius: 11px 8px 12px 9px / 9px 12px 8px 11px;
  cursor: pointer;
  font-family: inherit;

  &:hover { background: var(--skp-surface-tint); color: var(--skp-ink); border-color: #d3c9b6; }
  &.sm { padding: 4px 10px; font-size: 12px; }
  &.pri { background: var(--skp-accent-deep); border-color: var(--skp-accent-deep); color: #fff; }
  &.pri:hover { background: var(--skp-accent); color: #fff; }
  &.danger { color: var(--skp-clay); }
  &.danger:hover { background: rgba(162, 104, 92, 0.1); border-color: var(--skp-clay); color: var(--skp-clay); }
  &:disabled { opacity: 0.5; cursor: not-allowed; }
}

.skp-toolbar { display: flex; gap: 12px; align-items: center; margin-bottom: 16px; flex-wrap: wrap; }
.skp-search {
  display: flex; align-items: center; gap: 8px;
  border: 1px solid var(--skp-line); border-radius: 11px 8px 12px 9px / 9px 12px 8px 11px;
  padding: 7px 14px; flex: 1; max-width: 340px; background: var(--skp-surface); color: var(--skp-ink-3);
  input { border: 0; outline: 0; background: none; color: var(--skp-ink); font-size: 13px; width: 100%; font-family: inherit; }
  input::placeholder { color: var(--skp-ink-3); }
}
.skp-filters {
  display: flex; overflow: hidden; border: 1px solid var(--skp-line);
  border-radius: 11px 8px 12px 9px / 9px 12px 8px 11px; background: var(--skp-surface);
  button { border: 0; border-right: 1px solid var(--skp-line-soft); background: none; color: var(--skp-ink-2); padding: 7px 14px; font-size: 13px; cursor: pointer; font-family: inherit; }
  button:last-child { border-right: 0; }
  button.active { background: var(--skp-accent-bg); color: var(--skp-accent-deep); font-weight: 600; }
}
.skp-viewtoggle { display: flex; gap: 6px; }
.skp-viewtoggle button {
  width: 34px; height: 32px; border: 1px solid var(--skp-line); background: var(--skp-surface);
  border-radius: 9px 7px 10px 8px / 8px 10px 7px 9px; color: var(--skp-ink-3); cursor: pointer;
  &.active { color: var(--skp-accent-deep); border-color: var(--skp-accent); background: var(--skp-accent-bg); }
}

.skp-state { padding: 28px 0; color: var(--skp-ink-3); font-size: 13px; }
.skp-skill-row {
  display: flex; align-items: flex-start; gap: 14px; padding: 16px 20px;
  background: var(--skp-surface); border: 1px solid var(--skp-line); border-left: 3px solid var(--skp-accent);
  border-radius: 13px 9px 15px 10px / 10px 14px 9px 12px; margin-bottom: 24px; transition: background .15s, border-color .15s;
  &:hover { background: var(--skp-hover); border-color: #cfc4af; }
  &.is-off { opacity: 0.66; border-left-color: #cfc7b9; }
}
.skp-monogram {
  width: 40px; height: 40px; flex-shrink: 0; border: 1px solid var(--skp-line);
  border-radius: 12px 8px 11px 9px / 9px 11px 8px 12px; background: var(--skp-surface-sage);
  display: flex; align-items: center; justify-content: center; font-family: var(--skp-mono);
  font-size: 13px; color: var(--skp-accent-deep); letter-spacing: -0.5px; border-color: #d7ddd0;
}
.skp-skill-main { flex: 1; min-width: 0; }
.skp-skill-name { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.skp-name { font-weight: 700; font-size: 15px; font-family: var(--skp-mono); }
.skp-skill-desc {
  margin: 4px 0 0; color: var(--skp-ink-2); font-size: 14px;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
}
.skp-skill-meta { margin-top: 8px; display: flex; gap: 14px; flex-wrap: wrap; color: var(--skp-ink-3); font-size: 12px; }
.skp-skill-actions { display: flex; gap: 12px; flex-shrink: 0; flex-wrap: wrap; justify-content: flex-end; }
.skp-tag {
  display: inline-flex; align-items: center; gap: 4px; font-size: 12px; padding: 2px 10px;
  border-radius: 10px 8px 9px 11px / 9px 11px 8px 10px; border: 1px solid transparent; line-height: 1.6; white-space: nowrap;
  &.on { background: var(--skp-accent-bg); color: var(--skp-accent-deep); border-color: color-mix(in srgb, var(--skp-accent) 35%, white); }
  &.off { background: var(--skp-surface-tint); color: var(--skp-ink-2); border-color: var(--skp-line); }
  &.ver { background: var(--skp-amber-bg); color: var(--skp-amber); border-color: #e2d3b8; }
}
.skp-dot { width: 7px; height: 7px; border-radius: 50%; display: inline-block; background: var(--skp-accent); }
.skp-dot.gray { background: var(--skp-ink-3); }
.skp-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 24px; }
.skp-skill-cell {
  background: var(--skp-surface); border: 1px solid var(--skp-line); border-top: 3px solid var(--skp-accent); border-radius: 13px 9px 15px 10px / 10px 14px 9px 12px;
  padding: 16px 24px; display: flex; flex-direction: column; gap: 12px; transition: background .15s, border-color .15s;
  &:hover { background: var(--skp-hover); border-color: #cfc4af; }
  &.is-off { opacity: 0.66; border-top-color: #cfc7b9; }
  .skp-skill-desc { -webkit-line-clamp: 3; flex: 1; }
  .skp-skill-actions { justify-content: flex-start; margin-top: 4px; }
}
.skp-cell-top { display: flex; justify-content: space-between; align-items: flex-start; gap: 8px; }
.skp-cell-badges { display: flex; gap: 6px; flex-wrap: wrap; justify-content: flex-end; }

.skp-editor-card {
  background: var(--skp-surface); border: 1px solid var(--skp-line);
  border-radius: 16px 11px 18px 12px / 12px 17px 11px 14px; padding: 24px 26px 26px;
  border-top: 3px solid var(--skp-accent);
}
.skp-editor-head { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; padding-bottom: 14px; border-bottom: 1px solid var(--skp-line-soft); flex-wrap: wrap; }
.skp-editor-head h2 { margin: 0; font-size: 17px; }
.skp-tabs { display: flex; overflow: hidden; border: 1px solid var(--skp-line); border-radius: 11px 8px 12px 9px / 9px 12px 8px 11px; }
.skp-tabs button { border: 0; border-right: 1px solid var(--skp-line-soft); background: var(--skp-surface); padding: 7px 18px; font-size: 13px; color: var(--skp-ink-2); cursor: pointer; font-family: inherit; }
.skp-tabs button:last-child { border-right: 0; }
.skp-tabs button.active { background: var(--skp-accent-bg); color: var(--skp-accent-deep); font-weight: 600; }
.skp-tabs button:disabled { opacity: 0.45; cursor: not-allowed; }
.skp-upload-panel { margin-top: 18px; padding: 24px; border: 1px dashed #c9bea8; border-radius: 13px 9px 14px 10px / 10px 14px 9px 13px; background: var(--skp-surface-tint); }
.skp-file-input { display: none; }
.skp-upload-row { display: flex; align-items: center; gap: 12px; margin-top: 12px; }
.skp-editor-layout { display: grid; grid-template-columns: minmax(320px, 380px) minmax(0, 1fr); gap: 22px; margin-top: 18px; }
.skp-form { display: flex; flex-direction: column; gap: 14px; }
.skp-form-card { background: var(--skp-surface); border: 1px solid var(--skp-line); border-radius: 12px 8px 13px 9px / 9px 13px 8px 12px; padding: 18px 20px; display: flex; flex-direction: column; gap: 18px; }
.skp-field-label {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  font-weight: 600;
  color: var(--skp-ink);
  letter-spacing: 0.3px;
}

.skp-field-help {
  margin: 0 0 6px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--skp-ink-3);
}

.skp-field-meta {
  display: flex;
  justify-content: flex-end;
  margin-top: 4px;
}

.skp-required {
  display: inline;
  color: var(--skp-clay);
  font-weight: 700;
}

.skp-optional {
  color: var(--skp-ink-3);
  font-size: 12px;
  font-weight: 400;
}

.skp-field-error {
  margin: 6px 0 0;
  color: var(--skp-clay);
  font-size: 12px;
  line-height: 1.6;
}

.skp-field.has-error input,
.skp-field.has-error textarea {
  border-color: var(--skp-clay);
  background: var(--skp-clay-bg);
}

.skp-field {
  display: block;
  input, textarea {
    width: 100%; border: 1px solid var(--skp-line); border-radius: 10px 7px 11px 8px / 8px 11px 7px 10px;
    background: var(--skp-input); padding: 10px 13px; font-size: 14px; line-height: 1.7; color: var(--skp-ink); outline: none; font-family: var(--skp-mono);
    &:focus { border-color: var(--skp-accent); background: var(--skp-surface); box-shadow: 0 0 0 3px rgba(92, 107, 81, 0.12); }
  }
  textarea { resize: vertical; min-height: 96px; line-height: 1.8; font-family: inherit; }
  small { display: block; margin-top: 3px; text-align: right; font-size: 11px; color: var(--skp-ink-3); font-family: var(--skp-mono); }
}
.skp-quality { background: var(--skp-surface-sage); border: 1px solid #dfe4d9; border-left: 3px solid var(--skp-accent); border-radius: 12px 8px 13px 9px / 9px 13px 8px 12px; padding: 16px 18px; }
.skp-quality-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  margin: 0 0 8px;
  padding: 0;
  border: 0;
  background: none;
  color: var(--skp-accent-deep);
  font-family: inherit;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.5px;
  text-align: left;
  cursor: pointer;

  span { display: inline-flex; align-items: center; gap: 6px; }
}
.skp-qa-list { list-style: none; margin: 0; padding: 0; font-size: 14px; color: var(--skp-ink-2); }
.skp-qa-list li { display: flex; gap: 8px; padding: 3px 0; align-items: flex-start; line-height: 1.7; }
.skp-qa-list .skp-dot { margin-top: 7px; flex-shrink: 0; }
.skp-ok { color: var(--skp-accent-deep); font-family: var(--skp-mono); font-weight: 700; }
.skp-todo { color: var(--skp-amber); font-family: var(--skp-mono); font-weight: 700; }
.skp-quality-tip { margin-top: 9px; padding-top: 8px; border-top: 1px dashed var(--skp-line); font-size: 12px; color: var(--skp-ink-2); line-height: 1.7; }
.skp-link-btn { border: 0; background: none; color: var(--skp-accent-deep); font-size: 12px; text-decoration: underline; text-underline-offset: 3px; padding: 0; cursor: pointer; font-family: inherit; }
.skp-markdown { border: 1px solid var(--skp-line); border-radius: 14px 9px 16px 10px / 10px 15px 9px 13px; background: var(--skp-surface); overflow: hidden; }
.skp-markdown-head { display: flex; align-items: center; gap: 10px; padding: 12px 16px; background: var(--skp-surface-tint); border-bottom: 1px solid var(--skp-line-soft); flex-wrap: wrap; }
.skp-markdown-title { font-size: 12px; font-weight: 700; color: var(--skp-ink-2); letter-spacing: 0.5px; }
.skp-mode-toggle { margin-left: auto; display: flex; overflow: hidden; border: 1px solid var(--skp-line); border-radius: 9px 7px 10px 8px / 8px 10px 7px 9px; background: var(--skp-surface); }
.skp-mode-toggle button { border: 0; border-right: 1px solid var(--skp-line-soft); background: none; padding: 4px 12px; font-size: 12px; color: var(--skp-ink-3); cursor: pointer; font-family: inherit; }
.skp-mode-toggle button:last-child { border-right: 0; }
.skp-mode-toggle button.active { background: var(--skp-accent-deep); color: #f7f4ec; }
.skp-fullscreen-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 24px;
  margin-left: 8px;
  padding: 0;
  border: 1px solid var(--skp-line);
  border-radius: 7px 6px 8px 7px / 7px 8px 6px 7px;
  background: var(--skp-surface);
  color: var(--skp-ink-2);
  cursor: pointer;

  &:hover { background: var(--skp-surface-tint); color: var(--skp-ink); }
}

.skp-markdown.is-fullscreen {
  position: absolute;
  inset: 0;
  z-index: 60;
  display: flex;
  flex-direction: column;
  margin: 0;
  border: 0;
  border-radius: 0;
  background: var(--skp-surface);
}

.skp-markdown.is-fullscreen .skp-markdown-head { flex: none; }

.skp-markdown.is-fullscreen .skp-markdown-body {
  flex: 1;
  height: auto;
  min-height: 0;
}

.skp-markdown-body { display: grid; grid-template-columns: 1fr 1fr; height: clamp(420px, 60vh, 640px); min-height: 0; overflow: hidden; }
.skp-markdown-body.is-edit { grid-template-columns: 1fr; .skp-pv-pane { display: none; } }
.skp-markdown-body.is-preview { grid-template-columns: 1fr; .skp-ed-pane { display: none; } }
.skp-ed-pane { display: flex; flex-direction: column; min-height: 0; overflow: hidden; }
.skp-ed-pane textarea {
  flex: 1; width: 100%; min-height: 0; height: auto; border: 0; outline: 0; resize: none; background: var(--skp-surface);
  color: var(--skp-ink); font-family: var(--skp-mono); font-size: 14px; line-height: 1.95; padding: 22px 24px;
  overflow-y: auto; scrollbar-width: none; -ms-overflow-style: none;
}
.skp-ed-pane textarea::-webkit-scrollbar { display: none; width: 0; height: 0; }
.skp-ed-pane .skp-field-error { flex: none; margin: 0; padding: 8px 24px 12px; background: var(--skp-surface); }
.skp-pv-pane {
  height: 100%; min-height: 0; box-sizing: border-box; border-left: 1px dashed #d8cebd; padding: 24px 28px;
  background: var(--skp-preview); overflow-y: auto; scrollbar-width: none; -ms-overflow-style: none;
  font-family: var(--skp-serif); font-size: 15px; line-height: 2.05; color: var(--skp-ink); word-break: break-word;
}
.skp-pv-pane::-webkit-scrollbar { display: none; width: 0; height: 0; }
.skp-pv-pane h1, .skp-pv-pane h2, .skp-pv-pane h3, .skp-pv-pane h4, .skp-pv-pane h5, .skp-pv-pane h6 {
  font-family: var(--skp-serif); margin: 18px 0 10px; color: var(--skp-ink); line-height: 1.5;
}
.skp-pv-pane h1 { font-size: 24px; border-bottom: 1px solid var(--skp-line); padding-bottom: 8px; }
.skp-pv-pane h2 { font-size: 20px; border-bottom: 1px solid var(--skp-line); padding-bottom: 6px; }
.skp-pv-pane h3 { font-size: 18px; }
.skp-pv-pane h4 { font-size: 16px; }
.skp-pv-pane p { margin: 8px 0; }
.skp-pv-pane ol, .skp-pv-pane ul { padding-left: 24px; margin: 8px 0; }
.skp-pv-pane li { margin: 4px 0; }
.skp-pv-pane blockquote { margin: 12px 0; padding: 8px 16px; border-left: 4px solid var(--skp-accent); background: var(--skp-surface-sage); color: var(--skp-ink-2); }
.skp-pv-pane table { width: 100%; border-collapse: collapse; margin: 12px 0; font-family: var(--skp-serif); font-size: 14px; }
.skp-pv-pane th, .skp-pv-pane td { border: 1px solid var(--skp-line); padding: 6px 12px; text-align: left; }
.skp-pv-pane th { background: var(--skp-surface-sage); font-weight: 600; }
.skp-pv-pane pre { margin: 12px 0; padding: 14px; background: var(--skp-surface-tint); border-radius: 8px; overflow: auto; scrollbar-width: none; }
.skp-pv-pane pre::-webkit-scrollbar { display: none; }
.skp-pv-pane pre code { background: transparent; padding: 0; font-family: var(--skp-mono); font-size: 13px; line-height: 1.6; }
.skp-pv-pane code { background: var(--skp-surface-sage); padding: 2px 6px; border-radius: 3px; font-family: var(--skp-mono); font-size: 0.9em; }
.skp-pv-pane img { max-width: 100%; border-radius: 6px; }
.skp-pv-pane hr { border: 0; border-top: 1px solid var(--skp-line); margin: 20px 0; }
.skp-pv-pane a { color: var(--skp-accent-deep); text-decoration: none; }
.skp-pv-pane a:hover { text-decoration: underline; }
.skp-pv-pane strong { font-weight: 600; }
.skp-pv-pane .katex-display { overflow-x: auto; scrollbar-width: none; }
.skp-pv-pane .katex-display::-webkit-scrollbar { display: none; }
.skp-editor-footer { margin-top: 10px; }
.skp-autosave { display: inline-flex; align-items: center; gap: 4px; color: var(--skp-accent-deep); font-size: 12px; white-space: nowrap; }
.skp-draft-note { margin: 4px 0 0; color: var(--skp-accent-deep); font-size: 12px; }

.skp-detail-actions { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; }
.skp-detail-head { background: var(--skp-detail-head); border: 1px solid var(--skp-line); border-top: 3px solid var(--skp-accent); border-radius: 15px 10px 17px 11px / 11px 16px 10px 13px; padding: 22px 26px; margin-bottom: 18px; }
.skp-detail-title-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.skp-detail-title-row h1 { margin: 0; font-family: var(--skp-mono); font-size: 19px; }
.skp-switch { display: inline-flex; align-items: center; gap: 8px; border: 0; background: none; cursor: pointer; font-family: inherit; padding: 0; }
.skp-switch-track { width: 36px; height: 20px; background: var(--td-bg-color-secondarycontainer, #f0ede6); border: 1px solid var(--skp-line); border-radius: 10px 8px 11px 9px; position: relative; transition: 0.2s; }
.skp-switch-track::after { content: ""; position: absolute; top: 2px; left: 2px; width: 13px; height: 13px; background: var(--skp-ink-3); border-radius: 50%; transition: 0.2s; }
.skp-switch.on .skp-switch-track { background: var(--skp-accent-bg); border-color: var(--skp-accent); }
.skp-switch.on .skp-switch-track::after { left: 17px; background: var(--skp-accent-deep); }
.skp-switch-label { color: var(--skp-ink-2); font-size: 12px; }
.skp-detail-desc { margin: 12px 0 0; max-width: 680px; color: var(--skp-ink-2); font-size: 13.5px; line-height: 1.8; }
.skp-chip { display: inline-flex; align-items: center; font-size: 12px; color: var(--skp-ink-2); border: 1px solid var(--skp-line); padding: 2px 10px; border-radius: 10px 7px 9px 11px / 9px 11px 7px 10px; background: var(--skp-surface); }
.skp-chip.on { color: var(--skp-accent-deep); border-color: var(--skp-accent); background: var(--skp-accent-bg); }
.skp-detail-grid { display: grid; grid-template-columns: 1.4fr 1fr; gap: 18px; }
.skp-detail-col { min-width: 0; }
.skp-panel { background: var(--skp-surface); border: 1px solid var(--skp-line); border-radius: 14px 9px 16px 10px / 10px 15px 9px 13px; padding: 18px 20px; margin-bottom: 18px; }
.skp-sec-title { font-size: 13px; font-weight: 700; color: var(--skp-ink-2); letter-spacing: 0.8px; margin-bottom: 12px; }
.skp-sec-title::before { content: ""; display: inline-block; width: 3px; height: 12px; margin-right: 8px; vertical-align: -1px; border-radius: 2px; background: var(--skp-accent); }
.skp-stat-row { display: flex; gap: 12px; margin-bottom: 14px; flex-wrap: wrap; }
.skp-stat-box { flex: 1; min-width: 110px; background: var(--skp-surface-sage); border: 1px solid #dfe4d9; border-radius: 11px 8px 12px 9px / 9px 12px 8px 11px; padding: 14px 16px; }
.skp-stat-n { font-size: 22px; font-weight: 700; font-family: var(--skp-mono); color: var(--skp-accent-deep); }
.skp-stat-l { margin-top: 2px; font-size: 11.5px; color: var(--skp-ink-3); }
.skp-bars { display: flex; align-items: flex-end; gap: 7px; height: 90px; padding: 4px 2px 0; }
.skp-bar { flex: 1; background: #dfe4d9; border-radius: 3px 3px 1px 1px; }
.skp-bar:last-child { background: var(--skp-accent); }
.skp-xlabels { display: flex; gap: 7px; margin-top: 6px; }
.skp-xlabels span { flex: 1; text-align: center; font-size: 10px; color: var(--skp-ink-3); font-family: var(--skp-mono); }
.skp-note { margin-top: 8px; }
.skp-try { background: var(--skp-surface-tint); border: 1px dashed #cfc4af; border-radius: 13px 9px 14px 10px / 10px 14px 9px 13px; padding: 18px 20px; }
.skp-try textarea { width: 100%; min-height: 72px; border: 1px solid var(--skp-line); border-radius: 10px 7px 11px 8px / 8px 11px 7px 10px; background: var(--skp-input); padding: 12px 14px; font-size: 14px; color: var(--skp-ink); outline: none; resize: vertical; font-family: inherit; }
.skp-try-actions { margin-top: 10px; }
.skp-try-result { margin-top: 12px; background: var(--td-bg-color-secondarycontainer, #faf9f6); border: 1px solid var(--skp-line-soft); border-radius: 10px 7px 11px 8px / 8px 11px 7px 10px; padding: 12px 16px; font-size: 12.5px; color: var(--skp-ink-2); line-height: 1.8; }
.skp-vlist { list-style: none; margin: 0; padding: 0; }
.skp-vlist li { position: relative; padding: 0 0 16px 22px; }
.skp-vlist li::before { content: ""; position: absolute; left: 5px; top: 7px; width: 8px; height: 8px; background: var(--skp-ink-3); border-radius: 50%; }
.skp-vlist li::after { content: ""; position: absolute; left: 8.5px; top: 18px; bottom: 0; width: 1px; background: var(--skp-line); }
.skp-vlist li:last-child::after { display: none; }
.skp-vlist li.cur::before { background: var(--skp-accent-deep); }
.skp-vhead { display: flex; gap: 10px; align-items: baseline; flex-wrap: wrap; }
.skp-v { font-family: var(--skp-mono); font-weight: 700; font-size: 13px; }
.skp-vdate { font-size: 11.5px; color: var(--skp-ink-3); }
.skp-vnote { margin-top: 4px; font-size: 13px; color: var(--skp-ink-2); line-height: 1.8; }
.skp-scope-row { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; margin-top: 12px; }

html[theme-mode="dark"] .skills-page {
  --skp-line: var(--w-hairline, #3a342c);
  --skp-line-soft: color-mix(in srgb, var(--w-hairline, #3a342c) 60%, var(--w-window, #191714));
  --skp-ink: var(--w-ink, #f3efe8);
  --skp-ink-2: var(--w-body, #c8c1b5);
  --skp-ink-3: var(--w-muted, #978e80);
  --skp-surface-tint: var(--w-surface-raised, #2a2621);
  --skp-surface-sage: color-mix(in srgb, var(--w-page-accent, #5edb95) 10%, var(--w-surface, #211e1a));
  --skp-preview: var(--w-surface, #211e1a);
  --skp-surface: var(--w-surface, #211e1a);
  --skp-input: var(--w-surface-raised, #2a2621);
  --skp-hover: var(--w-surface-hover, #2e2923);
  --skp-detail-head: linear-gradient(180deg, var(--w-surface-raised, #2a2621) 0%, var(--w-surface, #211e1a) 64%);
}

html[theme-mode="dark"] .skp-btn.pri,
html[theme-mode="dark"] .skp-btn.pri:hover {
  color: var(--w-page-accent-ink, #0e130f);
}

@media (max-width: 900px) {
  .skp-editor-layout, .skp-detail-grid { grid-template-columns: 1fr; }
  .skp-markdown-body { grid-template-columns: 1fr; height: min(70vh, 640px); }
  .skp-markdown-body.is-split { grid-template-rows: 1fr 1fr; }
  .skp-pv-pane { border-left: 0; border-top: 1px dashed var(--skp-line); }
}
</style>
