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

    <t-dialog v-model:visible="addVisible" :header="$t('skillCatalog.addTitle')" :confirm-btn="null"
      width="560px">
      <div class="skill-add-form">
        <p class="skill-add-hint">{{ $t('skillCatalog.addHint') }}</p>
        <input ref="fileInputRef" type="file" accept=".zip,.md" class="skill-file-input"
          @change="onFileSelected" />
        <t-button variant="outline" :loading="registering" @click="chooseFile">
          {{ $t('skillCatalog.chooseFile') }}
        </t-button>
        <span v-if="pendingFile" class="skill-file-name">{{ pendingFile.name }}</span>
        <t-divider>{{ $t('skillCatalog.orSource') }}</t-divider>
        <t-textarea v-model="source" :autosize="{ minRows: 3, maxRows: 6 }"
          :placeholder="$t('skillCatalog.sourcePlaceholder')" />
        <div class="skill-dialog-actions">
          <t-button theme="primary" :loading="registering" @click="registerSkill">
            {{ $t('skillCatalog.register') }}
          </t-button>
        </div>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  deleteSkillCatalog,
  listSkillCatalog,
  registerSkillCatalogFromFile,
  registerSkillCatalogFromSource,
  setSkillCatalogEnabled,
  type SkillCatalogItem,
} from '@/api/skill'

const { t } = useI18n()
const loading = ref(false)
const catalog = ref<SkillCatalogItem[]>([])
const addVisible = ref(false)
const registering = ref(false)
const pendingFile = ref<File | null>(null)
const source = ref('')
const fileInputRef = ref<HTMLInputElement | null>(null)

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

const openAdd = () => {
  pendingFile.value = null
  source.value = ''
  addVisible.value = true
}

const chooseFile = () => fileInputRef.value?.click()

const onFileSelected = (event: Event) => {
  const input = event.target as HTMLInputElement
  pendingFile.value = input.files?.[0] || null
}

const registerSkill = async () => {
  if (!pendingFile.value && !source.value.trim()) {
    MessagePlugin.warning(t('skillCatalog.addRequired'))
    return
  }
  registering.value = true
  try {
    if (pendingFile.value) {
      await registerSkillCatalogFromFile(pendingFile.value)
    } else {
      await registerSkillCatalogFromSource(source.value.trim())
    }
    MessagePlugin.success(t('skillCatalog.addSuccess'))
    addVisible.value = false
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

.skill-add-form,
.skill-grant-form {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.skill-add-form {
  flex-direction: column;
  align-items: stretch;
}

.skill-add-hint {
  margin: 0;
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.skill-file-input {
  display: none;
}

.skill-file-name {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.skill-dialog-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 8px;
}

.skill-grant-field {
  min-width: 160px;
  flex: 1;
}

.skill-grant-list {
  margin-top: 16px;
  border-top: 1px solid var(--td-component-stroke);
}

.skill-grant-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 0;
  border-bottom: 1px solid var(--td-component-stroke);
  font-size: 13px;
}
</style>
