<template>
  <section class="skp-view">
    <div class="skp-editor-card">
      <header class="skp-editor-head">
        <h2>{{ item ? $t('skillCatalog.editTitle') : $t('skillCatalog.newTitle') }}</h2>
        <span v-if="item?.version" class="skp-tag ver">{{ item.version }}</span>
        <span v-if="item" class="skp-tag" :class="item.enabled === false ? 'off' : 'on'">
          <span v-if="item.enabled !== false" class="skp-dot" />{{ item.enabled === false ? $t('skillCatalog.disabled') : $t('skillCatalog.enabled') }}
        </span>
        <div class="skp-spacer" />
        <div class="skp-tabs" role="tablist">
          <button
            type="button"
            role="tab"
            :aria-selected="tab === 'file'"
            :disabled="Boolean(item)"
            @click="switchTab('file')"
          >
            {{ $t('skillCatalog.tabUpload') }}
          </button>
          <button type="button" role="tab" :aria-selected="tab === 'write'" @click="switchTab('write')">
            {{ $t('skillCatalog.tabWrite') }}
          </button>
        </div>
        <span v-if="autosavedAt" class="skp-autosave" aria-live="polite">
          <t-icon name="check-circle" />{{ $t('skillCatalog.autosaved', { time: autosavedAt }) }}
        </span>
        <button type="button" class="skp-btn sm" @click="cancelEditor">{{ $t('common.cancel') }}</button>
        <button type="button" class="skp-btn pri sm" :disabled="saving" :aria-busy="saving" @click="submit">
          {{ item ? $t('skillCatalog.saveEdit') : $t('skillCatalog.register') }}
        </button>
      </header>
      <p class="skp-faint">{{ tab === 'file' ? $t('skillCatalog.editorHint') : $t('skillCatalog.editorHintWrite') }}</p>
      <p v-if="draftRestored" class="skp-draft-note" aria-live="polite">{{ $t('skillCatalog.draftRestored') }}</p>

      <div v-if="tab === 'file'" class="skp-upload-panel">
        <p class="skp-muted">{{ $t('skillCatalog.fileHint') }}</p>
        <input ref="fileInput" class="skp-file-input" type="file" accept=".md" @change="onFileSelected" />
        <div class="skp-upload-row">
          <button type="button" class="skp-btn" @click="fileInput?.click()">{{ $t('skillCatalog.chooseFile') }}</button>
          <span v-if="pendingFile" class="skp-faint">{{ pendingFile.name }}</span>
        </div>
        <p v-if="errors.file" id="skill-file-error" class="skp-field-error">{{ errors.file }}</p>
      </div>

      <div v-else class="skp-editor-layout">
        <div class="skp-form">
          <div class="skp-form-card">
            <label class="skp-field" :class="{ 'has-error': errors.name }" for="skill-name">
              <span class="skp-field-label">{{ $t('skillCatalog.nameLabel') }}<span class="skp-required" aria-hidden="true">*</span></span>
              <p id="skill-name-help" class="skp-field-help">{{ $t('skillCatalog.nameHint') }}</p>
              <input
                id="skill-name"
                v-model="form.name"
                type="text"
                maxlength="100"
                required
                :aria-invalid="Boolean(errors.name)"
                aria-describedby="skill-name-help skill-name-count skill-name-error"
                :placeholder="$t('skillCatalog.namePlaceholder')"
              />
              <div class="skp-field-meta">
                <small id="skill-name-count" aria-live="polite">{{ form.name.length }} / 100 {{ $t('skillCatalog.charUnit') }}</small>
              </div>
              <p v-if="errors.name" id="skill-name-error" class="skp-field-error">{{ errors.name }}</p>
            </label>

            <label class="skp-field" :class="{ 'has-error': errors.version }" for="skill-version">
              <span class="skp-field-label">
                {{ $t('skillCatalog.versionLabel') }}<span class="skp-optional">({{ $t('skillCatalog.optional') }})</span>
              </span>
              <p id="skill-version-help" class="skp-field-help">{{ $t('skillCatalog.versionHint') }}</p>
              <input
                id="skill-version"
                v-model="form.version"
                type="text"
                maxlength="64"
                aria-describedby="skill-version-help skill-version-count"
                :placeholder="$t('skillCatalog.versionPlaceholder')"
              />
              <div class="skp-field-meta">
                <small id="skill-version-count" aria-live="polite">{{ form.version.length }} / 64 {{ $t('skillCatalog.charUnit') }}</small>
              </div>
            </label>

            <label class="skp-field" :class="{ 'has-error': errors.description }" for="skill-description">
              <span class="skp-field-label">{{ $t('skillCatalog.descriptionLabel') }}<span class="skp-required" aria-hidden="true">*</span></span>
              <p id="skill-description-help" class="skp-field-help">{{ $t('skillCatalog.descriptionHint') }}</p>
              <textarea
                id="skill-description"
                v-model="form.description"
                rows="4"
                maxlength="500"
                required
                :aria-invalid="Boolean(errors.description)"
                aria-describedby="skill-description-help skill-description-count skill-description-error"
                :placeholder="$t('skillCatalog.descriptionPlaceholder')"
              />
              <div class="skp-field-meta">
                <small id="skill-description-count" aria-live="polite">{{ form.description.length }} / 500 {{ $t('skillCatalog.charUnit') }}</small>
              </div>
              <p v-if="errors.description" id="skill-description-error" class="skp-field-error">{{ errors.description }}</p>
            </label>
          </div>

          <div class="skp-quality" :class="{ 'is-collapsed': !qualityOpen }">
            <button
              type="button"
              class="skp-quality-title"
              :aria-label="$t('skillCatalog.qualityToggle')"
              :aria-expanded="qualityOpen"
              @click="toggleQuality"
            >
              <span><t-icon name="info-circle" />{{ $t('skillCatalog.qualityTitle') }}</span>
              <t-icon :name="qualityOpen ? 'chevron-up' : 'chevron-down'" />
            </button>
            <div v-show="qualityOpen">
              <ul class="skp-qa-list">
                <li v-for="check in qualityChecks" :key="check.label">
                  <span :class="check.ok ? 'skp-ok' : 'skp-todo'">{{ check.ok ? '✓' : '•' }}</span>{{ check.label }}
                </li>
              </ul>
              <div class="skp-quality-tip">
                {{ $t('skillCatalog.qualityTip') }}
                <button type="button" class="skp-link-btn" @click="insertTemplate">
                  {{ $t('skillCatalog.insertTemplate') }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <div class="skp-markdown" :class="{ 'is-fullscreen': markdownFullscreen }">
          <div class="skp-markdown-head">
            <span class="skp-markdown-title">{{ $t('skillCatalog.mdTitle') }}<span class="skp-required" aria-hidden="true">*</span></span>
            <span class="skp-faint" aria-live="polite">{{ $t('skillCatalog.mdLimit', { count: form.instructions.length }) }}</span>
            <div class="skp-mode-toggle" role="tablist">
              <button type="button" role="tab" :aria-selected="mode === 'edit'" @click="mode = 'edit'">{{ $t('skillCatalog.modeEdit') }}</button>
              <button type="button" role="tab" :aria-selected="mode === 'split'" @click="mode = 'split'">{{ $t('skillCatalog.modeSplit') }}</button>
              <button type="button" role="tab" :aria-selected="mode === 'preview'" @click="mode = 'preview'">{{ $t('skillCatalog.modePreview') }}</button>
            </div>
            <button
              type="button"
              class="skp-fullscreen-btn"
              :aria-label="markdownFullscreen ? $t('shell.exitFullscreen') : $t('shell.fullscreen')"
              :aria-pressed="markdownFullscreen"
              :title="markdownFullscreen ? $t('shell.exitFullscreen') : $t('shell.fullscreen')"
              @click="toggleMarkdownFullscreen"
            >
              <t-icon :name="markdownFullscreen ? 'fullscreen-exit' : 'fullscreen'" />
            </button>
          </div>
          <div class="skp-markdown-body" :class="`is-${mode}`">
            <div class="skp-ed-pane">
              <textarea
                id="skill-instructions"
                v-model="form.instructions"
                spellcheck="false"
                maxlength="32000"
                required
                :aria-invalid="Boolean(errors.instructions)"
                aria-describedby="skill-instructions-error"
                :placeholder="$t('skillCatalog.instructionsPlaceholder')"
              />
              <p v-if="errors.instructions" id="skill-instructions-error" class="skp-field-error">{{ errors.instructions }}</p>
            </div>
            <div class="skp-pv-pane" v-html="previewHTML" />
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import {
  createSkillCatalogContent,
  getCatalogSkillFile,
  registerSkillCatalogFromFile,
  updateSkillCatalog,
  type SkillCatalogItem,
} from '@/api/skill'
import { renderDocumentPreviewMarkdown } from '@/utils/documentPreviewMarkdown'
import 'highlight.js/styles/github.css'
import 'katex/dist/katex.min.css'

const props = defineProps<{ item: SkillCatalogItem | null }>()
const emit = defineEmits<{
  (e: 'cancel'): void
  (e: 'saved', id: string): void
}>()

const { t } = useI18n()
const EMPTY_SNAPSHOT = JSON.stringify({ name: '', version: '', description: '', instructions: '', file: '' })
const tab = ref<'file' | 'write'>(props.item ? 'write' : 'file')
const mode = ref<'edit' | 'split' | 'preview'>('edit')
const saving = ref(false)
const markdownFullscreen = ref(false)
const pendingFile = ref<File | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const form = reactive({ name: '', version: '', description: '', instructions: '' })
const errors = reactive({ file: '', name: '', version: '', description: '', instructions: '' })
const qualityOpen = ref(true)
const qualityTouched = ref(false)
const autosavedAt = ref('')
const draftRestored = ref(false)
const snapshot = ref(EMPTY_SNAPSHOT)
const savedOrDiscarded = ref(false)
let draftTimer: ReturnType<typeof setTimeout> | null = null
let draftNoticeTimer: ReturnType<typeof setTimeout> | null = null

const previewHTML = computed(() => renderDocumentPreviewMarkdown(form.instructions))
const draftKey = computed(() => (props.item ? `skill-editor-draft:edit:${props.item.id}` : 'skill-editor-draft:new'))
const qualityChecks = computed(() => {
  const value = form.description.trim()
  return [
    { ok: /^(当|when|if|whenever)/i.test(value), label: t('skillCatalog.qualityStarts') },
    { ok: /(书|文档|笔记|引用|知识库|book|document|note|citation|knowledge)/i.test(value), label: t('skillCatalog.qualityKeywords') },
    { ok: value.length >= 16 && value.length <= 360, label: t('skillCatalog.qualityLength') },
    { ok: /(不|不要|不应|禁止|not|never|avoid)/i.test(value), label: t('skillCatalog.qualityNegative') },
  ]
})

const serialize = () =>
  JSON.stringify({
    name: form.name.trim(),
    version: form.version.trim(),
    description: form.description.trim(),
    instructions: form.instructions.trim(),
    file: pendingFile.value?.name || '',
  })
const dirty = computed(() => serialize() !== snapshot.value)
const hasDraftContent = () =>
  Boolean(form.name.trim() || form.version.trim() || form.description.trim() || form.instructions.trim())

const toggleMarkdownFullscreen = () => {
  markdownFullscreen.value = !markdownFullscreen.value
}

const focusFirstError = async () => {
  await nextTick()
  const id = errors.name ? 'skill-name' : errors.description ? 'skill-description' : errors.instructions ? 'skill-instructions' : 'skill-file-error'
  document.getElementById(id)?.scrollIntoView({ block: 'center', behavior: 'smooth' })
  if (id !== 'skill-file-error') document.getElementById(id)?.focus()
}

const clearDraft = () => {
  localStorage.removeItem(draftKey.value)
  autosavedAt.value = ''
  draftRestored.value = false
}

const saveDraft = () => {
  if (props.item || savedOrDiscarded.value || !hasDraftContent()) return
  const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  localStorage.setItem(
    draftKey.value,
    JSON.stringify({ form: { ...form }, tab: 'write', time }),
  )
  autosavedAt.value = time
}

const scheduleDraft = () => {
  if (props.item || savedOrDiscarded.value) return
  if (draftTimer) clearTimeout(draftTimer)
  draftTimer = setTimeout(saveDraft, 500)
}

const loadDraft = () => {
  if (props.item) return
  const raw = localStorage.getItem(draftKey.value)
  if (!raw) return
  try {
    const data = JSON.parse(raw)
    const savedForm = data?.form || {}
    if (!savedForm.name && !savedForm.version && !savedForm.description && !savedForm.instructions) return
    form.name = savedForm.name || ''
    form.version = savedForm.version || ''
    form.description = savedForm.description || ''
    form.instructions = savedForm.instructions || ''
    tab.value = 'write'
    autosavedAt.value = data.time || ''
    draftRestored.value = true
    draftNoticeTimer = setTimeout(() => {
      draftRestored.value = false
    }, 4000)
  } catch {
    localStorage.removeItem(draftKey.value)
  }
}

const stripFrontmatter = (content: string) => {
  const trimmed = content.replace(/^﻿/, '')
  if (!trimmed.trimStart().startsWith('---')) return trimmed.trim()
  const lines = trimmed.split(/\r?\n/)
  let start = -1
  let end = -1
  for (let i = 0; i < lines.length; i += 1) {
    if (lines[i].trim() === '---') {
      if (start === -1) start = i
      else {
        end = i
        break
      }
    }
  }
  if (end === -1) return trimmed.trim()
  return lines.slice(end + 1).join('\n').trim()
}

const onMarkdownKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape' && markdownFullscreen.value) {
    markdownFullscreen.value = false
  }
}

onMounted(async () => {
  window.addEventListener('keydown', onMarkdownKeydown)
  if (!props.item) {
    loadDraft()
    return
  }
  form.name = props.item.name
  form.version = props.item.version || ''
  form.description = props.item.description || ''
  try {
    const response = await getCatalogSkillFile(props.item.id, 'SKILL.md')
    form.instructions = stripFrontmatter(String(response.data?.content || ''))
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('skillCatalog.loadFailed'))
  }
  snapshot.value = serialize()
})

watch(
  form,
  () => {
    if (errors.file) errors.file = ''
    if (errors.name && form.name.trim()) errors.name = ''
    if (errors.description && form.description.trim()) errors.description = ''
    if (errors.instructions && form.instructions.trim()) errors.instructions = ''
    if (!qualityTouched.value) qualityOpen.value = form.description.trim().length < 80
    scheduleDraft()
  },
  { deep: true },
)

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onMarkdownKeydown)
  if (draftTimer) clearTimeout(draftTimer)
  if (draftNoticeTimer) clearTimeout(draftNoticeTimer)
  if (!props.item && !savedOrDiscarded.value && hasDraftContent()) saveDraft()
})

const toggleQuality = () => {
  qualityTouched.value = true
  qualityOpen.value = !qualityOpen.value
}

const onFileSelected = (event: Event) => {
  const input = event.target as HTMLInputElement
  pendingFile.value = input.files?.[0] || null
  errors.file = ''
}

const insertTemplate = () => {
  const apply = () => {
    form.description = t('skillCatalog.templateText')
    errors.description = ''
  }
  if (!form.description.trim()) {
    apply()
    return
  }
  const dialog = DialogPlugin.confirm({
    header: t('skillCatalog.templateOverwriteTitle'),
    body: t('skillCatalog.templateOverwriteBody'),
    theme: 'warning',
    confirmBtn: t('skillCatalog.templateOverwriteConfirm'),
    cancelBtn: t('common.cancel'),
    onConfirm: () => {
      apply()
      dialog.destroy()
    },
    onCancel: () => dialog.destroy(),
  })
}

const switchTab = (next: 'file' | 'write') => {
  if (next === tab.value || (props.item && next === 'file')) return
  const apply = () => {
    tab.value = next
  }
  if (next === 'file' && form.instructions.trim()) {
    const dialog = DialogPlugin.confirm({
      header: t('skillCatalog.modeSwitchTitle'),
      body: t('skillCatalog.modeSwitchBody'),
      theme: 'warning',
      confirmBtn: t('skillCatalog.modeSwitchConfirm'),
      cancelBtn: t('common.cancel'),
      onConfirm: () => {
        apply()
        dialog.destroy()
      },
      onCancel: () => dialog.destroy(),
    })
    return
  }
  apply()
}

const validateWrite = () => {
  errors.name = form.name.trim() ? '' : t('skillCatalog.nameRequired')
  errors.description = form.description.trim() ? '' : t('skillCatalog.descriptionRequired')
  errors.instructions = form.instructions.trim() ? '' : t('skillCatalog.instructionsRequired')
  return !errors.name && !errors.description && !errors.instructions
}

const finishCancel = () => {
  savedOrDiscarded.value = true
  clearDraft()
  emit('cancel')
}

const cancelEditor = () => {
  if (!dirty.value) {
    finishCancel()
    return
  }
  const dialog = DialogPlugin.confirm({
    header: t('skillCatalog.discardTitle'),
    body: t('skillCatalog.discardBody'),
    theme: 'warning',
    confirmBtn: t('skillCatalog.discardConfirm'),
    cancelBtn: t('common.cancel'),
    onConfirm: () => {
      finishCancel()
      dialog.destroy()
    },
    onCancel: () => dialog.destroy(),
  })
}

const submit = async () => {
  errors.file = ''
  errors.name = ''
  errors.description = ''
  errors.instructions = ''
  saving.value = true
  try {
    if (tab.value === 'file' && !props.item) {
      if (!pendingFile.value) {
        errors.file = t('skillCatalog.addRequired')
        MessagePlugin.warning(errors.file)
        await focusFirstError()
        return
      }
      const result = await registerSkillCatalogFromFile(pendingFile.value)
      MessagePlugin.success(t('skillCatalog.addSuccess'))
      savedOrDiscarded.value = true
      clearDraft()
      emit('saved', result.data?.id || '')
      return
    }
    if (!validateWrite()) {
      MessagePlugin.warning(errors.name || errors.description || errors.instructions)
      await focusFirstError()
      return
    }
    if (props.item) {
      await updateSkillCatalog(props.item.id, {
        name: form.name.trim(),
        version: form.version.trim(),
        description: form.description.trim(),
        instructions: form.instructions.trim(),
      })
      MessagePlugin.success(t('skillCatalog.updateSuccess'))
      savedOrDiscarded.value = true
      clearDraft()
      emit('saved', props.item.id)
    } else {
      const result = await createSkillCatalogContent({
        name: form.name.trim(),
        version: form.version.trim(),
        description: form.description.trim(),
        instructions: form.instructions.trim(),
      })
      MessagePlugin.success(t('skillCatalog.addSuccess'))
      savedOrDiscarded.value = true
      clearDraft()
      emit('saved', result.data?.id || '')
    }
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('skillCatalog.addFailed'))
  } finally {
    saving.value = false
  }
}
</script>
