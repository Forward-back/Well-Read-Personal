<template>
  <section class="skp-view">
    <div class="skp-detail-actions">
      <button type="button" class="skp-btn sm" @click="$emit('back')">{{ $t('skillCatalog.back') }}</button>
      <div class="skp-spacer" />
      <button type="button" class="skp-btn sm" @click="$emit('edit', item)">{{ $t('common.edit') }}</button>
      <button type="button" class="skp-btn sm" @click="$emit('toggle', item)">
        {{ item.enabled === false ? $t('skillCatalog.enable') : $t('skillCatalog.disable') }}
      </button>
      <button type="button" class="skp-btn sm danger" @click="$emit('delete', item)">{{ $t('common.delete') }}</button>
    </div>

    <header class="skp-detail-head">
      <div class="skp-detail-title-row">
        <h1>{{ item.name }}</h1>
        <span v-if="item.version" class="skp-tag ver">{{ item.version }}</span>
        <button type="button" class="skp-switch" :class="{ on: item.enabled !== false }" @click="$emit('toggle', item)">
          <span class="skp-switch-track" />
          <span class="skp-switch-label">{{ item.enabled === false ? $t('skillCatalog.disabled') : $t('skillCatalog.enabled') }}</span>
        </button>
        <div class="skp-spacer" />
        <span class="skp-faint">{{ $t('skillCatalog.scope', { scope: '' }) }}</span>
        <span class="skp-chip on">{{ $t('skillCatalog.scopeQa') }}</span>
      </div>
      <p class="skp-detail-desc">{{ item.description || '—' }}</p>
    </header>

    <div class="skp-detail-grid">
      <div class="skp-detail-col">
        <section class="skp-panel">
          <div class="skp-sec-title">{{ $t('skillCatalog.usageTitle') }}</div>
          <div class="skp-stat-row">
            <div class="skp-stat-box"><div class="skp-stat-n">—</div><div class="skp-stat-l">{{ $t('skillCatalog.calls30') }}</div></div>
            <div class="skp-stat-box"><div class="skp-stat-n">—</div><div class="skp-stat-l">{{ $t('skillCatalog.lastUsed') }}</div></div>
          </div>
          <div class="skp-faint">{{ $t('skillCatalog.trendHint') }}</div>
          <div class="skp-bars">
            <div v-for="index in 12" :key="index" class="skp-bar" :style="{ height: '14%' }" />
          </div>
          <div class="skp-xlabels">
            <span v-for="index in 12" :key="index">W{{ 37 + index }}</span>
          </div>
          <div class="skp-faint skp-note">{{ $t('skillCatalog.statsPending') }}</div>
          <div class="skp-faint skp-note">{{ $t('skillCatalog.usageNote') }}</div>
        </section>

        <section class="skp-try">
          <div class="skp-sec-title">{{ $t('skillCatalog.tryTitle') }}</div>
          <textarea disabled :placeholder="$t('skillCatalog.tryPlaceholder')" />
          <div class="skp-try-actions">
            <button type="button" class="skp-btn pri sm" disabled>{{ $t('skillCatalog.tryRun') }}</button>
          </div>
          <div class="skp-try-result">{{ $t('skillCatalog.tryPending') }}</div>
        </section>
      </div>

      <div class="skp-detail-col">
        <section class="skp-panel">
          <div class="skp-sec-title">{{ $t('skillCatalog.versionsTitle') }}</div>
          <ul class="skp-vlist">
            <li class="cur">
              <div class="skp-vhead">
                <span class="skp-v">{{ item.version || '—' }}</span>
                <span class="skp-tag on">{{ $t('skillCatalog.current') }}</span>
                <span class="skp-vdate">{{ formatDate(item.updated_at) }}</span>
              </div>
              <div class="skp-vnote">{{ $t('skillCatalog.versionsPending') }}</div>
            </li>
          </ul>
        </section>

        <section class="skp-panel">
          <div class="skp-sec-title">{{ $t('skillCatalog.enableTitle') }}</div>
          <ul class="skp-qa-list">
            <li><span class="skp-dot" />{{ $t('skillCatalog.enabledLine') }}</li>
            <li><span class="skp-dot gray" />{{ $t('skillCatalog.disabledLine') }}</li>
          </ul>
          <div class="skp-scope-row">
            <span class="skp-chip on">{{ $t('skillCatalog.scopeQa') }}</span>
            <span class="skp-faint">{{ $t('skillCatalog.scopeHint') }}</span>
          </div>
        </section>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import type { SkillCatalogItem } from '@/api/skill'

defineProps<{ item: SkillCatalogItem }>()
defineEmits<{
  (e: 'back'): void
  (e: 'edit', item: SkillCatalogItem): void
  (e: 'toggle', item: SkillCatalogItem): void
  (e: 'delete', item: SkillCatalogItem): void
}>()

const formatDate = (value?: string) => {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleDateString()
}
</script>
