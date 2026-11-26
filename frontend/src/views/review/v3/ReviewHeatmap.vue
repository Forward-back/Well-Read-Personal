<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ReviewHeatmap, ReviewHeatmapCell } from '@/api/review'
import { formatIntensity, normalizeVulnerability, vulnerabilityLabelKey } from './vulnerability'
import ReviewIcon from './ReviewIcon.vue'
import type { ReviewFlatGroup } from './types'

const props = defineProps<{
  heatmap: ReviewHeatmap | null
  loading: boolean
  error: string
  canStart: boolean
  groups: ReviewFlatGroup[]
  rowGroupIds: string[]
}>()

const emit = defineEmits<{
  select: [cell: ReviewHeatmapCell]
  drill: [groupID: string]
  expand: []
  start: []
  retry: []
  'update:rowGroupIds': [ids: string[]]
}>()

const { t } = useI18n()

const levels = ['severe', 'warning', 'observe', 'healthy', 'none'] as const

const rows = computed(() => props.heatmap?.rows ?? [])
const columns = computed(() => props.heatmap?.columns ?? [])

const cellMap = computed(() => {
  const map = new Map<string, ReviewHeatmapCell>()
  for (const cell of props.heatmap?.cells ?? []) {
    map.set(cellKey(cell.row_id, cell.column_key), cell)
  }
  return map
})

const emphasized = computed(() => {
  const set = new Set<string>()
  for (const cell of props.heatmap?.priorities ?? []) {
    set.add(cellKey(cell.row_id, cell.column_key))
  }
  return set
})

const hasRows = computed(() => rows.value.length > 0 && columns.value.length > 0)
const hasReviewed = computed(() => (props.heatmap?.cells ?? []).some(cell => cell.level !== 'none'))
const emptyMode = computed<'cards' | 'reviews' | ''>(() => {
  if (!hasRows.value) return 'cards'
  if (!hasReviewed.value) return 'reviews'
  return ''
})

const gridStyle = computed(() => ({
  gridTemplateColumns: `var(--heatmap-row-width, 168px) repeat(${Math.max(columns.value.length, 1)}, minmax(26px, 1fr))`,
}))

function cellKey(rowId: string, columnKey: string) {
  return `${rowId}|${columnKey}`
}

function getCell(rowId: string, columnKey: string) {
  return cellMap.value.get(cellKey(rowId, columnKey)) ?? null
}

function isEmphasized(rowId: string, columnKey: string) {
  return emphasized.value.has(cellKey(rowId, columnKey))
}

function onCellClick(cell: ReviewHeatmapCell | null) {
  if (!cell?.clickable) return
  emit('select', cell)
}

function levelClass(level: string | undefined) {
  return `heatmap__cell--${normalizeVulnerability(level)}`
}

function cellLabel(cell: ReviewHeatmapCell | null) {
  if (!cell) return ''
  return `${t(vulnerabilityLabelKey(cell.level))} · ${formatIntensity(cell.intensity)}`
}

function toggleRow(groupId: string, checked: boolean) {
  const next = new Set(props.rowGroupIds)
  if (checked) next.add(groupId)
  else next.delete(groupId)
  emit('update:rowGroupIds', [...next])
}

function resetRows() {
  emit('update:rowGroupIds', [])
}
</script>

<template>
  <section class="heatmap" data-testid="review-v3-heatmap-panel">
    <div class="heatmap__head">
      <div class="rv-h2">
        {{ t('review.v3.heatmapTitle') }}
        <span class="rv-note">{{ t('review.v3.heatmapSubtitle') }}</span>
      </div>
      <div class="heatmap__actions">
        <details class="heatmap__rowpick">
          <summary class="rv-btn rv-btn--ghost rv-btn--sm">
            {{ rowGroupIds.length
              ? t('review.v3.heatmapRowsSelected', { count: rowGroupIds.length })
              : t('review.v3.heatmapRowsAuto') }}
            <ReviewIcon name="chevron-down" :size="11" />
          </summary>
          <div class="heatmap__rowmenu">
            <div class="heatmap__rowmenu-head">
              <b>{{ t('review.v3.heatmapRowsTitle') }}</b>
              <button type="button" class="rv-btn rv-btn--text rv-btn--sm" @click="resetRows">
                {{ t('review.v3.heatmapRowsReset') }}
              </button>
            </div>
            <label v-for="item in groups" :key="item.group.id" class="heatmap__rowmenu-item">
              <input
                type="checkbox"
                :checked="rowGroupIds.includes(item.group.id)"
                @change="(event: Event) => toggleRow(item.group.id, (event.target as HTMLInputElement).checked)"
              />
              <span :style="{ paddingLeft: `${item.depth * 12}px` }">{{ item.group.name }}</span>
            </label>
            <p v-if="!groups.length" class="heatmap__rowmenu-empty">{{ t('review.v3.heatmapRowsEmpty') }}</p>
          </div>
        </details>
        <button
          v-if="heatmap?.row_truncated || heatmap?.column_truncated"
          type="button"
          class="rv-btn rv-btn--text rv-btn--sm"
          data-testid="review-v3-heatmap-expand"
          @click="emit('expand')"
        >
          {{ t('review.v3.expandAll') }}
        </button>
        <t-tooltip :content="t('review.v3.heatmapLegendHint')" placement="top-right">
          <span class="heatmap__help" tabindex="0">
            <ReviewIcon name="help" :size="16" />
          </span>
        </t-tooltip>
      </div>
    </div>

    <t-loading :loading="loading" size="small" class="heatmap__loading">
      <div v-if="error" class="heatmap__error">
        <t-alert theme="error" :message="error" />
        <button type="button" class="rv-btn rv-btn--ghost rv-btn--sm" @click="emit('retry')">
          {{ t('common.retry') }}
        </button>
      </div>

      <div v-else-if="emptyMode" class="heatmap__empty" data-testid="review-v3-heatmap-empty">
        <ReviewIcon name="book" :size="28" />
        <p class="heatmap__empty-title">
          {{ emptyMode === 'cards' ? t('review.v3.heatmapNoCardsTitle') : t('review.v3.heatmapNoReviewsTitle') }}
        </p>
        <p class="heatmap__empty-hint">
          {{ emptyMode === 'cards' ? t('review.v3.heatmapNoCardsHint') : t('review.v3.heatmapNoReviewsHint') }}
        </p>
        <button v-if="canStart" type="button" class="rv-btn rv-btn--ghost rv-btn--sm" @click="emit('start')">
          {{ t('review.v3.startToday') }}
        </button>
      </div>

      <template v-else>
        <div v-if="heatmap?.scope_path" class="heatmap__scope">{{ heatmap.scope_path }}</div>
        <div class="heatmap__scroll" data-testid="review-v3-heatmap">
          <div class="heatmap__grid" :style="gridStyle">
            <div class="heatmap__corner" />
            <div
              v-for="column in columns"
              :key="column.key"
              class="heatmap__colhead"
              :title="column.title"
            >
              {{ column.title }}
            </div>

            <template v-for="row in rows" :key="row.id">
              <button
                v-if="row.drillable"
                type="button"
                class="heatmap__rowhead is-drillable"
                :title="row.path"
                data-testid="review-v3-heatmap-row"
                @click="emit('drill', row.group_id)"
              >
                <span class="heatmap__rowtitle">{{ row.path }}</span>
                <ReviewIcon name="arrow-right" :size="12" />
              </button>
              <div v-else class="heatmap__rowhead" :title="row.path">
                {{ row.path }}
              </div>
              <div v-for="column in columns" :key="column.key" class="heatmap__cellwrap">
                <t-tooltip v-if="getCell(row.id, column.key)" placement="top" :show-arrow="true">
                  <button
                    type="button"
                    class="heatmap__cell"
                    :class="[
                      levelClass(getCell(row.id, column.key)?.level),
                      {
                        'is-clickable': getCell(row.id, column.key)?.clickable,
                        'is-emphasis': isEmphasized(row.id, column.key),
                      },
                    ]"
                    :tabindex="getCell(row.id, column.key)?.clickable ? 0 : -1"
                    :aria-disabled="!getCell(row.id, column.key)?.clickable"
                    :aria-label="cellLabel(getCell(row.id, column.key))"
                    @click="onCellClick(getCell(row.id, column.key))"
                  />
                  <template #content>
                    <div class="heatmap__tip">
                      <div class="heatmap__tip-title">{{ row.path }} · {{ column.title }}</div>
                      <div>
                        {{ t('review.v3.heatmapLevel') }}：{{ t(vulnerabilityLabelKey(getCell(row.id, column.key)?.level)) }}
                      </div>
                      <div>
                        {{ t('review.v3.heatmapIntensity') }}：{{ formatIntensity(getCell(row.id, column.key)?.intensity) }}
                      </div>
                      <div>{{ t('review.v3.heatmapTotal', { count: getCell(row.id, column.key)?.total ?? 0 }) }}</div>
                      <div>
                        {{
                          t('review.v3.heatmapCounts', {
                            severe: getCell(row.id, column.key)?.severe ?? 0,
                            warning: getCell(row.id, column.key)?.warning ?? 0,
                            observe: getCell(row.id, column.key)?.observe ?? 0,
                            healthy: getCell(row.id, column.key)?.healthy ?? 0,
                            none: getCell(row.id, column.key)?.no_data ?? 0,
                          })
                        }}
                      </div>
                      <div v-if="column.kind === 'tag'" class="heatmap__tip-note">
                        {{ t('review.v3.heatmapOverlapHint') }}
                      </div>
                      <div v-if="column.kind === 'knowledge_base'" class="heatmap__tip-note">
                        {{ t('review.v3.heatmapKnowledgeBaseHint') }}
                      </div>
                    </div>
                  </template>
                </t-tooltip>
                <div v-else class="heatmap__cell heatmap__cell--none heatmap__cell--missing" />
              </div>
            </template>
          </div>
        </div>

        <div class="heatmap__legend">
          <span v-for="level in levels" :key="level" class="heatmap__legend-item">
            <i class="heatmap__legend-swatch" :class="levelClass(level)" />
            {{ t(vulnerabilityLabelKey(level)) }}
          </span>
          <span class="heatmap__legend-note">{{ t('review.v3.heatmapLegendNote') }}</span>
        </div>
      </template>
    </t-loading>
  </section>
</template>

<style lang="less" scoped>
.heatmap {
  min-width: 0;
  margin-top: 8px;
}

.heatmap__head {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: baseline;
  justify-content: space-between;
}

.heatmap__actions {
  display: flex;
  gap: 6px;
  align-items: center;
}

.heatmap__rowpick {
  position: relative;
}

.heatmap__rowpick summary {
  display: inline-flex;
  gap: 5px;
  align-items: center;
  list-style: none;
  cursor: pointer;
  user-select: none;
}

.heatmap__rowpick summary::-webkit-details-marker {
  display: none;
}

.heatmap__rowmenu {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  z-index: 20;
  width: 270px;
  max-height: 320px;
  padding: 10px;
  overflow-y: auto;
  background: var(--rvn-paper);
  border: 1px solid var(--rvn-line);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(63, 58, 52, 0.12);
}

.heatmap__rowmenu-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 2px 4px 8px;
  font-size: 14px;
  color: var(--rvn-ink);
}

.heatmap__rowmenu-item {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 5px 6px;
  font-size: 14px;
  color: var(--rvn-ink-soft);
  cursor: pointer;
  border-radius: 5px;

  &:hover {
    background: var(--rvn-surface);
  }
}

.heatmap__rowmenu-empty {
  padding: 10px;
  margin: 0;
  font-size: 14px;
  color: var(--rvn-muted);
}

.heatmap__help {
  display: inline-flex;
  color: var(--rvn-muted);
  cursor: help;
}

.heatmap__loading :deep(.t-loading__parent) {
  min-height: 132px;
}

.heatmap__error {
  display: flex;
  align-items: center;
  gap: 12px;

  :deep(.t-alert) {
    flex: 1;
    min-width: 0;
  }
}

.heatmap__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 160px;
  padding: 16px;
  color: var(--rvn-muted);
  text-align: center;
  background: var(--rvn-surface);
  border: 1px dashed var(--rvn-line-strong);
  border-radius: 7px 6px 8px 7px / 6px 8px 7px 6px;
}

.heatmap__empty-title {
  margin: 2px 0 0;
  font-size: 14px;
  font-weight: 500;
  color: var(--rvn-ink-soft);
}

.heatmap__empty-hint {
  margin: 0;
  font-size: 14px;
}

.heatmap__scroll {
  overflow-x: auto;
  padding-bottom: 4px;
}

.heatmap__scope {
  margin-bottom: 6px;
  overflow: hidden;
  font-size: 14px;
  color: var(--rvn-muted);
  white-space: nowrap;
  text-overflow: ellipsis;
}

.heatmap__grid {
  display: grid;
  gap: 4px;
  min-width: max-content;
}

@media (max-width: 760px) {
  .heatmap__grid {
    --heatmap-row-width: 132px;
  }
}

.heatmap__rowhead,
.heatmap__colhead {
  background: transparent;
}

.heatmap__rowhead {
  position: sticky;
  left: 0;
  z-index: 2;
  display: flex;
  align-items: center;
  height: 22px;
  padding-right: 8px;
  overflow: hidden;
  font-size: 14px;
  color: var(--rvn-ink-soft);
  white-space: nowrap;
  background: var(--rvn-paper);
  text-overflow: ellipsis;
}

button.heatmap__rowhead {
  gap: 4px;
  font-family: inherit;
  text-align: left;
  cursor: pointer;
  border: 0;

  &:hover {
    color: var(--rvn-brand-deep);
  }

  &:focus-visible {
    outline: 2px solid var(--rvn-brand);
    outline-offset: -2px;
  }
}

.heatmap__rowtitle {
  overflow: hidden;
  text-overflow: ellipsis;
}

.heatmap__corner {
  position: sticky;
  left: 0;
  z-index: 3;
  height: 20px;
  background: var(--rvn-paper);
}

.heatmap__colhead {
  height: 20px;
  overflow: hidden;
  font-size: 14px;
  line-height: 20px;
  color: var(--rvn-muted);
  text-align: center;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.heatmap__cellwrap {
  min-width: 0;
}

.heatmap__cell {
  width: 100%;
  height: 22px;
  padding: 0;
  font-family: inherit;
  cursor: default;
  background: var(--rvn-gray-soft);
  border: 1px solid transparent;
  border-radius: 3px;

  &:focus-visible {
    outline: 2px solid var(--rvn-brand);
    outline-offset: 1px;
  }

  &.is-clickable {
    cursor: pointer;
  }

  &.is-clickable:hover {
    border-color: var(--rvn-ink-soft);
  }

  &.is-emphasis {
    outline: 2px solid var(--rvn-brand);
    outline-offset: -2px;
  }
}

.heatmap__cell--severe {
  background: #a96b64;
}

.heatmap__cell--warning {
  background: #d2aaa4;
}

.heatmap__cell--observe {
  background: #e9dcd8;
}

.heatmap__cell--healthy {
  background: #dde5d8;
}

.heatmap__cell--none {
  background: var(--rvn-gray-soft);
}

:global(html[theme-mode="dark"] .heatmap__cell--severe) {
  background: color-mix(in srgb, var(--rvn-red) 62%, var(--rvn-paper));
}

:global(html[theme-mode="dark"] .heatmap__cell--warning) {
  background: color-mix(in srgb, var(--rvn-orange) 52%, var(--rvn-paper));
}

:global(html[theme-mode="dark"] .heatmap__cell--observe) {
  background: color-mix(in srgb, var(--rvn-accent) 38%, var(--rvn-paper));
}

:global(html[theme-mode="dark"] .heatmap__cell--healthy) {
  background: color-mix(in srgb, var(--rvn-green) 36%, var(--rvn-paper));
}

.heatmap__cell--missing {
  background: transparent;
}

.heatmap__legend {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 14px;
  align-items: center;
  margin-top: 10px;
  font-size: 14px;
  color: var(--rvn-ink-soft);
}

.heatmap__legend-item {
  display: inline-flex;
  gap: 5px;
  align-items: center;
}

.heatmap__legend-swatch {
  display: inline-block;
  width: 11px;
  height: 11px;
  border-radius: 3px;
}

.heatmap__legend-note {
  color: var(--rvn-muted);
}

.heatmap__tip {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-width: 260px;
  font-size: 14px;
  line-height: 18px;
}

.heatmap__tip-title {
  margin-bottom: 2px;
  font-weight: 600;
}

.heatmap__tip-note {
  color: var(--rvn-muted);
}
</style>
