<template>
  <div class="skill-catalog-settings">
    <div class="section-header">
      <div>
        <h2>{{ $t('skillCatalog.title') }}</h2>
        <p class="section-description">{{ $t('skillCatalog.description') }}</p>
      </div>
      <t-button theme="primary" @click="openAdd">{{ $t('skillCatalog.add') }}</t-button>
    </div>

    <div v-if="loading" class="skill-loading">
      <t-loading size="small" />
    </div>

    <template v-else>
      <div v-if="catalog.length === 0" class="skill-empty">{{ $t('skillCatalog.empty') }}</div>
      <div v-else class="skill-list">
        <article v-for="item in catalog" :key="item.id" class="skill-card"
          :class="{ 'skill-card--disabled': item.enabled === false }">
          <div class="skill-card-main">
            <div class="skill-card-heading">
              <t-icon name="book" class="skill-icon" />
              <span class="skill-name">{{ item.name }}</span>
              <t-tag v-if="item.version" size="small" variant="light">{{ item.version }}</t-tag>
              <t-tag size="small" :theme="item.enabled === false ? 'default' : 'success'" variant="light">
                {{ item.enabled === false ? $t('skillCatalog.disabled') : $t('skillCatalog.enabled') }}
              </t-tag>
            </div>
            <p v-if="item.description" class="skill-description">{{ item.description }}</p>
          </div>
          <div class="skill-card-actions">
            <t-button size="small" variant="text" @click="openEdit(item)">
              {{ $t('common.edit') }}
            </t-button>
            <t-button size="small" variant="text" @click="toggleEnabled(item)">
              {{ item.enabled === false ? $t('skillCatalog.enable') : $t('skillCatalog.disable') }}
            </t-button>
            <t-button size="small" variant="text" theme="danger" @click="removeSkill(item)">
              {{ $t('common.delete') }}
            </t-button>
          </div>
        </article>
      </div>
    </template>

    <t-dialog v-model:visible="dialogVisible" :header="dialogTitle" :confirm-btn="null" width="760px">
      <div class="skill-add-form">
        <t-tabs v-if="dialogMode === 'create'" v-model="activeTab" class="skill-tabs">
          <t-tab-panel value="file" :label="$t('skillCatalog.tabUpload')" />
          <t-tab-panel value="write" :label="$t('skillCatalog.tabWrite')" />
        </t-tabs>

        <template v-if="dialogMode === 'create' && activeTab === 'file'">
          <p class="skill-add-hint">{{ $t('skillCatalog.fileHint') }}</p>
          <input ref="fileInputRef" type="file" accept=".md" class="skill-file-input"
            @change="onFileSelected" />
          <div class="skill-file-row">
            <t-button variant="outline" :loading="registering" @click="chooseFile">
              {{ $t('skillCatalog.chooseFile') }}
            </t-button>
            <span v-if="pendingFile" class="skill-file-name">{{ pendingFile.name }}</span>
          </div>
        </template>

        <template v-else>
          <div class="skill-form-grid">
            <t-input v-model="form.name" :maxlength="100" :placeholder="$t('skillCatalog.namePlaceholder')"
              :label="$t('skillCatalog.nameLabel')" />
            <t-input v-model="form.version" :maxlength="64" :placeholder="$t('skillCatalog.versionPlaceholder')"
              :label="$t('skillCatalog.versionLabel')" />
          </div>
          <t-textarea v-model="form.description" :maxlength="500" :autosize="{ minRows: 2, maxRows: 3 }"
            :placeholder="$t('skillCatalog.descriptionPlaceholder')" :label="$t('skillCatalog.descriptionLabel')" />
          <div class="skill-instructions-header">
            <span class="skill-instructions-label">{{ $t('skillCatalog.instructionsLabel') }}</span>
            <t-switch v-model="showPreview" size="small" :label="$t('skillCatalog.previewToggle')" />
          </div>
          <div v-if="showPreview" class="skill-preview" v-html="previewHTML"></div>
          <t-textarea v-else v-model="form.instructions" :maxlength="32000"
            :autosize="{ minRows: 12, maxRows: 20 }"
            :placeholder="$t('skillCatalog.instructionsPlaceholder')" />
        </template>

        <div class="skill-dialog-actions">
          <t-button variant="outline" @click="dialogVisible = false">{{ $t('common.cancel') }}</t-button>
          <t-button theme="primary" :loading="registering" @click="submit">
            {{ dialogMode === 'edit' ? $t('skillCatalog.saveEdit') : $t('skillCatalog.register') }}
          </t-button>
        </div>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  createSkillCatalogContent,
  deleteSkillCatalog,
  getCatalogSkillFile,
  listSkillCatalog,
  registerSkillCatalogFromFile,
  setSkillCatalogEnabled,
  updateSkillCatalog,
  type SkillCatalogItem,
} from '@/api/skill'
import { safeMarkdownToHTML } from '@/utils/security'

const { t } = useI18n()
const loading = ref(false)
const catalog = ref<SkillCatalogItem[]>([])
const dialogVisible = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const activeTab = ref<'file' | 'write'>('file')
const registering = ref(false)
const pendingFile = ref<File | null>(null)
const editingId = ref('')
const fileInputRef = ref<HTMLInputElement | null>(null)
const showPreview = ref(false)
const form = reactive({ name: '', description: '', version: '', instructions: '' })

const dialogTitle = computed(() =>
  dialogMode.value === 'edit' ? t('skillCatalog.editTitle') : t('skillCatalog.addTitle'),
)
const previewHTML = computed(() => safeMarkdownToHTML(form.instructions))

const load = async () => {
  loading.value = true
  try {
    const response = await listSkillCatalog()
    catalog.value = Array.isArray(response.data) ? response.data : []
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('skillCatalog.loadFailed'))
  } finally {
    loading.value = false
  }
}

const resetForm = () => {
  form.name = ''
  form.description = ''
  form.version = ''
  form.instructions = ''
  pendingFile.value = null
  showPreview.value = false
}

const openAdd = () => {
  resetForm()
  dialogMode.value = 'create'
  activeTab.value = 'file'
  dialogVisible.value = true
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

const openEdit = async (item: SkillCatalogItem) => {
  resetForm()
  dialogMode.value = 'edit'
  activeTab.value = 'write'
  editingId.value = item.id
  form.name = item.name
  form.description = item.description || ''
  form.version = item.version || ''
  dialogVisible.value = true
  try {
    const response = await getCatalogSkillFile(item.id, 'SKILL.md')
    const content = response.data?.content || ''
    form.instructions = stripFrontmatter(String(content))
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('skillCatalog.loadFailed'))
  }
}

const chooseFile = () => fileInputRef.value?.click()

const onFileSelected = (event: Event) => {
  const input = event.target as HTMLInputElement
  pendingFile.value = input.files?.[0] || null
}

const validateForm = () => {
  if (!form.name.trim()) {
    MessagePlugin.warning(t('skillCatalog.nameRequired'))
    return false
  }
  if (!form.description.trim()) {
    MessagePlugin.warning(t('skillCatalog.descriptionRequired'))
    return false
  }
  if (!form.instructions.trim()) {
    MessagePlugin.warning(t('skillCatalog.instructionsRequired'))
    return false
  }
  return true
}

const submit = async () => {
  registering.value = true
  try {
    if (dialogMode.value === 'create' && activeTab.value === 'file') {
      if (!pendingFile.value) {
        MessagePlugin.warning(t('skillCatalog.addRequired'))
        return
      }
      await registerSkillCatalogFromFile(pendingFile.value)
      MessagePlugin.success(t('skillCatalog.addSuccess'))
    } else if (dialogMode.value === 'edit') {
      if (!validateForm()) return
      await updateSkillCatalog(editingId.value, {
        name: form.name.trim(),
        description: form.description.trim(),
        version: form.version.trim(),
        instructions: form.instructions.trim(),
      })
      MessagePlugin.success(t('skillCatalog.updateSuccess'))
    } else {
      if (!validateForm()) return
      await createSkillCatalogContent({
        name: form.name.trim(),
        description: form.description.trim(),
        version: form.version.trim(),
        instructions: form.instructions.trim(),
      })
      MessagePlugin.success(t('skillCatalog.addSuccess'))
    }
    dialogVisible.value = false
    await load()
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('skillCatalog.addFailed'))
  } finally {
    registering.value = false
  }
}

const toggleEnabled = async (item: SkillCatalogItem) => {
  try {
    await setSkillCatalogEnabled(item.id, item.enabled === false)
    await load()
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('skillCatalog.updateFailed'))
  }
}

const removeSkill = async (item: SkillCatalogItem) => {
  try {
    await deleteSkillCatalog(item.id)
    MessagePlugin.success(t('skillCatalog.deleteSuccess'))
    await load()
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('skillCatalog.deleteFailed'))
  }
}

onMounted(() => {
  void load()
})
</script>

<style lang="less" scoped>
.skill-catalog-settings {
  width: 100%;
}

.section-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;

  h2 {
    margin: 0 0 8px;
    font-size: 20px;
    font-weight: 600;
    color: var(--td-text-color-primary);
  }
}

.section-description {
  margin: 0;
  font-size: 14px;
  line-height: 1.5;
  color: var(--td-text-color-secondary);
}

.skill-loading {
  display: flex;
  justify-content: center;
  padding: 24px;
}

.skill-empty {
  padding: 20px 0;
  font-size: 13px;
  color: var(--td-text-color-placeholder);
}

.skill-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.skill-card {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;

  &--disabled {
    opacity: 0.65;
  }
}

.skill-card-main {
  min-width: 0;
}

.skill-card-heading {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.skill-icon {
  color: var(--td-brand-color);
}

.skill-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.skill-description {
  margin: 6px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--td-text-color-secondary);
}

.skill-card-actions {
  display: flex;
  flex-shrink: 0;
  gap: 2px;
}

.skill-tabs {
  margin-bottom: 4px;
}

.skill-add-form {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.skill-form-grid {
  display: flex;
  gap: 12px;
}

.skill-add-hint {
  margin: 0;
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.skill-file-input {
  display: none;
}

.skill-file-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.skill-file-name {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.skill-instructions-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.skill-instructions-label {
  font-size: 14px;
  color: var(--td-text-color-primary);
}

.skill-preview {
  min-height: 220px;
  max-height: 420px;
  overflow: auto;
  padding: 12px 14px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 6px;
  background: var(--td-bg-color-secondarycontainer);
  font-size: 13px;
  line-height: 1.7;

  :deep(pre) {
    overflow-x: auto;
  }
}

.skill-dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}
</style>
