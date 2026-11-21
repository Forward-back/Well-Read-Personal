<template>
  <div class="background-settings">
    <div class="section-header">
      <h2>{{ $t('backgroundSettings.title') }}</h2>
      <p class="section-description">
        {{ $t('backgroundSettings.description') }}
      </p>
    </div>

    <div class="background-grid">
      <div
        v-for="card in cards"
        :key="card.key"
        class="background-card"
        :class="{ 'is-selected': store.choice === card.key }"
        role="button"
        tabindex="0"
        @click="handleSelect(card.key)"
        @keydown.enter.prevent="handleSelect(card.key)"
        @keydown.space.prevent="handleSelect(card.key)"
      >
        <span
          v-if="card.url"
          class="background-card__preview"
          :style="{ backgroundImage: `url(${card.url})` }"
        />
        <span v-else class="background-card__preview background-card__preview--empty">
          <t-icon name="image" size="20px" />
        </span>
        <span class="background-card__name" :title="card.label">{{ card.label }}</span>
        <t-icon
          v-if="store.choice === card.key"
          class="background-card__check"
          name="check-circle-filled"
          size="18px"
        />
        <button
          v-if="card.uploadId"
          type="button"
          class="background-card__delete"
          :aria-label="$t('backgroundSettings.deleteAria', { name: card.label })"
          @click.stop="handleDelete(card.uploadId, card.label)"
        >
          <t-icon name="delete" size="16px" />
        </button>
      </div>

      <button
        type="button"
        class="background-upload-tile"
        :disabled="uploading || store.loading || store.uploads.length >= store.maxUploadCount"
        @click="pickFile"
      >
        <t-icon :name="uploading ? 'loading' : 'add'" size="20px" :class="{ spinning: uploading }" />
        <span>{{ uploading ? $t('backgroundSettings.uploading') : $t('backgroundSettings.upload') }}</span>
        <small>{{ $t('backgroundSettings.count', { count: store.uploads.length, max: store.maxUploadCount }) }}</small>
      </button>
    </div>

    <div class="background-upload-hint">
      {{ $t('backgroundSettings.hint', { size: maxSizeLabel }) }}
    </div>
    <input
      ref="fileInput"
      type="file"
      accept="image/jpeg,image/png,image/webp"
      class="background-file-input"
      @change="handleFileChange"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import grassBackground from '@/assets/img/app-bg/grass.jpg'
import { BUILTIN_BACKGROUNDS } from '@/constants/backgrounds'
import { useBackgroundStore } from '@/stores/background'

type BackgroundCard = {
  key: string
  label: string
  url: string
  uploadId?: string
}

const { t } = useI18n()
const store = useBackgroundStore()
const fileInput = ref<HTMLInputElement | null>(null)
const uploading = ref(false)

const maxSizeLabel = computed(() => {
  const mb = store.maxUploadBytes / (1024 * 1024)
  return `${Number.isInteger(mb) ? mb : mb.toFixed(1)}MB`
})

const cards = computed<BackgroundCard[]>(() => [
  { key: 'default', label: t('backgroundSettings.defaultLabel'), url: grassBackground },
  ...BUILTIN_BACKGROUNDS.map((item) => ({
    key: item.id,
    label: t('backgroundSettings.wallpaperLabel', { index: item.index }),
    url: item.url,
  })),
  ...store.uploads.map((item) => ({
    key: `upload:${item.id}`,
    label: item.display_name,
    url: store.uploadUrls[item.id] || '',
    uploadId: item.id,
  })),
])

onMounted(async () => {
  await store.load()
  await store.hydrateUploadPreviews()
})

watch(
  () => store.uploads.map((item) => item.id).join(','),
  () => {
    void store.hydrateUploadPreviews()
  },
)

const errorMessage = (error: unknown, fallback: string) =>
  error instanceof Error && error.message ? error.message : fallback

const handleSelect = async (key: string) => {
  if (store.choice === key) return
  try {
    await store.select(key)
  } catch (error) {
    MessagePlugin.error(errorMessage(error, t('backgroundSettings.saveFailed')))
  }
}

const pickFile = () => {
  if (uploading.value) return
  fileInput.value?.click()
}

const handleFileChange = async (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  uploading.value = true
  try {
    await store.upload(file)
    MessagePlugin.success(t('backgroundSettings.uploadSuccess'))
  } catch (error) {
    MessagePlugin.error(errorMessage(error, t('backgroundSettings.uploadFailed')))
  } finally {
    uploading.value = false
  }
}

const handleDelete = (id: string, label: string) => {
  const dialog = DialogPlugin.confirm({
    header: t('backgroundSettings.deleteTitle'),
    body: t('backgroundSettings.deleteConfirm', { name: label }),
    theme: 'warning',
    confirmBtn: t('common.delete'),
    cancelBtn: t('common.cancel'),
    onConfirm: async () => {
      dialog.destroy()
      try {
        await store.remove(id)
        MessagePlugin.success(t('backgroundSettings.deleteSuccess'))
      } catch (error) {
        MessagePlugin.error(errorMessage(error, t('backgroundSettings.deleteFailed')))
      }
    },
  })
}
</script>

<style lang="less" scoped>
.background-settings {
  width: 100%;
}

.section-header {
  margin-bottom: 24px;

  h2 {
    font-size: 20px;
    font-weight: 600;
    color: var(--td-text-color-primary);
    margin: 0 0 8px;
  }

  .section-description {
    font-size: 14px;
    color: var(--td-text-color-secondary);
    margin: 0;
    line-height: 1.5;
  }
}

.background-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
}

.background-card {
  position: relative;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 2px solid transparent;
  border-radius: 10px;
  background: var(--td-bg-color-container);
  box-shadow: 0 0 0 1px var(--td-component-stroke);
  cursor: pointer;
  text-align: left;
  outline: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;

  &:hover,
  &:focus-visible {
    border-color: var(--td-brand-color-light-active);
  }

  &.is-selected {
    border-color: #07c05f;
    box-shadow: 0 0 0 3px rgba(7, 192, 95, 0.14);
  }
}

.background-card__preview {
  display: block;
  aspect-ratio: 16 / 10;
  background-position: center;
  background-size: cover;
  background-repeat: no-repeat;
}

.background-card__preview--empty {
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--td-text-color-placeholder);
  background: var(--td-bg-color-secondarycontainer);
}

.background-card__name {
  padding: 8px 10px;
  font-size: 13px;
  color: var(--td-text-color-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.background-card__check {
  position: absolute;
  top: 8px;
  right: 8px;
  color: #07c05f;
  background: #fff;
  border-radius: 50%;
}

.background-card__delete {
  position: absolute;
  top: 8px;
  left: 8px;
  display: none;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  padding: 0;
  border: none;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  cursor: pointer;
}

.background-card:hover .background-card__delete,
.background-card:focus-within .background-card__delete {
  display: flex;
}

.background-upload-tile {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 148px;
  border: 1px dashed var(--td-component-stroke);
  border-radius: 10px;
  background: transparent;
  color: var(--td-text-color-secondary);
  font-size: 14px;
  cursor: pointer;
  transition: border-color 0.2s ease, color 0.2s ease;

  small {
    color: var(--td-text-color-placeholder);
  }

  &:hover:not(:disabled) {
    border-color: #07c05f;
    color: #07c05f;
  }

  &:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }
}

.spinning {
  animation: background-spin 0.9s linear infinite;
}

@keyframes background-spin {
  to {
    transform: rotate(360deg);
  }
}

.background-upload-hint {
  margin-top: 12px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.background-file-input {
  display: none;
}
</style>
