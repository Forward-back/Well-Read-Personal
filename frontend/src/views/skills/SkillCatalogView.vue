<template>
  <section class="skp-view">
    <PageTopbar
      :title="$t('skillCatalog.pageTitle')"
      :meta="$t('skillCatalog.pageSubtitle', { count: catalog.length, enabled: enabledCount })"
    >
      <template #actions>
        <button type="button" class="skp-btn pri" @click="$emit('create')">
          <t-icon name="add" />
          {{ $t('skillCatalog.add') }}
        </button>
      </template>
    </PageTopbar>

    <div class="skp-toolbar">
      <label class="skp-search">
        <t-icon name="search" />
        <input v-model="keyword" :placeholder="$t('skillCatalog.searchPlaceholder')" />
      </label>
      <div class="skp-filters">
        <button
          v-for="filter in filters"
          :key="filter.value"
          type="button"
          :class="{ active: status === filter.value }"
          @click="status = filter.value"
        >
          {{ filter.label }}
        </button>
      </div>
      <div class="skp-spacer" />
      <div class="skp-viewtoggle">
        <button type="button" :class="{ active: view === 'list' }" :title="$t('skillCatalog.viewList')" @click="view = 'list'">
          <t-icon name="view-list" />
        </button>
        <button type="button" :class="{ active: view === 'grid' }" :title="$t('skillCatalog.viewGrid')" @click="view = 'grid'">
          <t-icon name="view-module" />
        </button>
      </div>
    </div>

    <div v-if="loading" class="skp-state">
      <t-loading size="small" />
    </div>
    <div v-else-if="!catalog.length" class="skp-state">{{ $t('skillCatalog.empty') }}</div>
    <div v-else-if="!filtered.length" class="skp-state">{{ $t('skillCatalog.filteredEmpty') }}</div>

    <template v-else-if="view === 'list'">
      <article
        v-for="item in filtered"
        :key="item.id"
        class="skp-skill-row"
        :class="{ 'is-off': item.enabled === false }"
      >
        <div class="skp-monogram">{{ monogram(item.name) }}</div>
        <div class="skp-skill-main">
          <div class="skp-skill-name">
            <span class="skp-name">{{ item.name }}</span>
            <span v-if="item.version" class="skp-tag ver">{{ item.version }}</span>
            <span class="skp-tag" :class="item.enabled === false ? 'off' : 'on'">
              <span v-if="item.enabled !== false" class="skp-dot" />{{ item.enabled === false ? $t('skillCatalog.disabled') : $t('skillCatalog.enabled') }}
            </span>
          </div>
          <p class="skp-skill-desc">{{ item.description || '—' }}</p>
          <div class="skp-skill-meta">
            <span>{{ $t('skillCatalog.metaCalls', { count: '—' }) }}</span>
            <span>{{ $t('skillCatalog.scope', { scope: $t('skillCatalog.scopeQa') }) }}</span>
            <span>{{ $t('skillCatalog.updatedAt', { date: formatDate(item.updated_at) }) }}</span>
          </div>
        </div>
        <div class="skp-skill-actions">
          <button type="button" class="skp-btn sm" @click="$emit('detail', item)">{{ $t('skillCatalog.detail') }}</button>
          <button type="button" class="skp-btn sm" @click="$emit('edit', item)">{{ $t('common.edit') }}</button>
          <button type="button" class="skp-btn sm" @click="$emit('toggle', item)">
            {{ item.enabled === false ? $t('skillCatalog.enable') : $t('skillCatalog.disable') }}
          </button>
          <button type="button" class="skp-btn sm danger" @click="$emit('delete', item)">{{ $t('common.delete') }}</button>
        </div>
      </article>
    </template>

    <div v-else class="skp-grid">
      <article
        v-for="item in filtered"
        :key="item.id"
        class="skp-skill-cell"
        :class="{ 'is-off': item.enabled === false }"
      >
        <div class="skp-cell-top">
          <div class="skp-monogram">{{ monogram(item.name) }}</div>
          <div class="skp-cell-badges">
            <span v-if="item.version" class="skp-tag ver">{{ item.version }}</span>
            <span class="skp-tag" :class="item.enabled === false ? 'off' : 'on'">{{ item.enabled === false ? $t('skillCatalog.disabled') : $t('skillCatalog.enabled') }}</span>
          </div>
        </div>
        <div class="skp-name">{{ item.name }}</div>
        <p class="skp-skill-desc">{{ item.description || '—' }}</p>
        <div class="skp-skill-meta">
          <span>{{ $t('skillCatalog.scope', { scope: $t('skillCatalog.scopeQa') }) }}</span>
          <span>{{ $t('skillCatalog.updatedAt', { date: formatDate(item.updated_at) }) }}</span>
        </div>
        <div class="skp-skill-actions">
          <button type="button" class="skp-btn sm" @click="$emit('detail', item)">{{ $t('skillCatalog.detail') }}</button>
          <button type="button" class="skp-btn sm" @click="$emit('edit', item)">{{ $t('common.edit') }}</button>
          <button type="button" class="skp-btn sm" @click="$emit('toggle', item)">
            {{ item.enabled === false ? $t('skillCatalog.enable') : $t('skillCatalog.disable') }}
          </button>
          <button type="button" class="skp-btn sm danger" @click="$emit('delete', item)">{{ $t('common.delete') }}</button>
        </div>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SkillCatalogItem } from '@/api/skill'
import PageTopbar from '@/components/layout/PageTopbar.vue'

const props = defineProps<{
  catalog: SkillCatalogItem[]
  loading: boolean
}>()
defineEmits<{
  (e: 'create'): void
  (e: 'detail', item: SkillCatalogItem): void
  (e: 'edit', item: SkillCatalogItem): void
  (e: 'toggle', item: SkillCatalogItem): void
  (e: 'delete', item: SkillCatalogItem): void
}>()

const { t } = useI18n()
const keyword = ref('')
const status = ref<'all' | 'on' | 'off'>('all')
const view = ref<'list' | 'grid'>('list')

const enabledCount = computed(() => props.catalog.filter((item) => item.enabled !== false).length)
const disabledCount = computed(() => props.catalog.length - enabledCount.value)
const filters = computed(() => [
  { value: 'all' as const, label: t('skillCatalog.filterAll', { count: props.catalog.length }) },
  { value: 'on' as const, label: t('skillCatalog.filterOn', { count: enabledCount.value }) },
  { value: 'off' as const, label: t('skillCatalog.filterOff', { count: disabledCount.value }) },
])
const filtered = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  return props.catalog.filter((item) => {
    if (status.value === 'on' && item.enabled === false) return false
    if (status.value === 'off' && item.enabled !== false) return false
    if (!q) return true
    return `${item.name} ${item.description || ''}`.toLowerCase().includes(q)
  })
})

const monogram = (name: string) => {
  const parts = name.split(/[-_\s]+/).filter(Boolean)
  if (parts.length > 1) return (parts[0]!.slice(0, 1) + parts[1]!.slice(0, 1)).toLowerCase()
  return name.slice(0, 2).toLowerCase()
}

const formatDate = (value?: string) => {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleDateString()
}
</script>
