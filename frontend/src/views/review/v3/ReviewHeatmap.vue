<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ReviewHeatmap, ReviewHeatmapCell } from '@/api/review'
import { formatIntensity, normalizeVulnerability, vulnerabilityLabelKey } from './vulnerability'
import chartIcon from '@/assets/icons/static/chart.png'
import bookIcon from '@/assets/icons/static/book.png'
import helpIcon from '@/assets/icons/static/help.png'
import arrowIcon from '@/assets/icons/static/arrow-right.png'

const props = defineProps<{
  heatmap: ReviewHeatmap | null
  loading: boolean
  error: string
  canStart: boolean
}>()

const emit = defineEmits<{
  select: [cell: ReviewHeatmapCell]
  drill: [groupID: string]
  expand: []
  start: []
  retry: []
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
  gridTemplateColumns: `var(--heatmap-row-width, 200px) repeat(${Math.max(columns.value.length, 1)}, var(--heatmap-cell-width, 88px))`,
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
</script>

<template>
  <t-card class="heatmap" :bordered="false" data-testid="review-v3-heatmap-panel">
    <template #title>
      <div class="heatmap__title"><img :src="chartIcon" alt="" />{{ t('review.v3.heatmapTitle') }}</div>
    </template>
    <template #actions>
      <t-button
        v-if="heatmap?.row_truncated || heatmap?.column_truncated"
        size="small"
        variant="text"
        theme="primary"
        data-testid="review-v3-heatmap-expand"
        @click="emit('expand')"
      >
        {{ t('review.v3.expandAll') }}
      </t-button>
      <t-tooltip :content="t('review.v3.heatmapLegendHint')" placement="top-right">
        <span class="heatmap__help" tabindex="0">
          <img class="inline-hand-icon" :src="helpIcon" alt="" />
        </span>
      </t-tooltip>
    </template>

    <t-loading :loading="loading" size="small" class="heatmap__loading">
      <div v-if="error" class="heatmap__error">
        <t-alert theme="error" :message="error" />
        <t-button size="small" variant="outline" @click="emit('retry')">{{ t('common.retry') }}</t-button>
      </div>

      <div v-else-if="emptyMode" class="heatmap__empty" data-testid="review-v3-heatmap-empty">
        <img class="rv-icon rv-icon--lg" :src="bookIcon" alt="" />
        <p class="heatmap__empty-title">
          {{ emptyMode === 'cards' ? t('review.v3.heatmapNoCardsTitle') : t('review.v3.heatmapNoReviewsTitle') }}
        </p>
        <p class="heatmap__empty-hint">
          {{ emptyMode === 'cards' ? t('review.v3.heatmapNoCardsHint') : t('review.v3.heatmapNoReviewsHint') }}
        </p>
        <t-button v-if="canStart" size="small" variant="outline" @click="emit('start')">
          {{ t('review.v3.startToday') }}
        </t-button>
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
                <img class="inline-hand-icon inline-hand-icon--12" :src="arrowIcon" alt="" />
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
                    @click="onCellClick(getCell(row.id, column.key))"
                  >
                    <span class="heatmap__cell-value">
                      {{
                        getCell(row.id, column.key)?.level === 'none'
                          ? '—'
                          : formatIntensity(getCell(row.id, column.key)?.intensity)
                      }}
                    </span>
                    <span
                      v-if="(getCell(row.id, column.key)?.severe ?? 0) > 0"
                      class="heatmap__cell-severe"
                    >
                      {{ t('review.v3.heatmapSevereShort', { count: getCell(row.id, column.key)?.severe }) }}
                    </span>
                  </button>
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
                <div v-else class="heatmap__cell heatmap__cell--none heatmap__cell--missing">—</div>
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
  </t-card>
</template>

<style lang="less" scoped>
.inline-hand-icon {
  width: 16px;
  height: 16px;
  object-fit: contain;
}

.inline-hand-icon--12 {
  width: 12px;
  height: 12px;
}

.heatmap {
  min-width: 0;

  :deep(.t-card__header) {
    min-height: 44px;
    padding: 0 16px;
  }

  :deep(.t-card__body) {
    padding: 0 16px 16px;
  }
}

.heatmap__title {
  display: inline-flex;
  align-items: center;
  gap: 8px;

  img {
    width: 20px;
    height: 20px;
    object-fit: contain;
  }
}

.heatmap__help {
  display: inline-flex;
  color: var(--td-text-color-placeholder);
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
  border: 1px dashed var(--td-component-stroke);
  border-radius: var(--td-radius-default);
  background: var(--td-bg-color-container-hover);
  color: var(--td-text-color-placeholder);
  text-align: center;

  :deep(.t-button) {
    margin-top: 6px;
  }
}

.heatmap__empty-title {
  margin: 2px 0 0;
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-secondary);
}

.heatmap__empty-hint {
  margin: 0;
  font-size: 12px;
}

.heatmap__scroll {
  overflow-x: auto;
  padding-bottom: 4px;
}

.heatmap__scope {
  margin-bottom: 6px;
  font-size: 11px;
  color: var(--td-text-color-placeholder);
  overflow: hidden;
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
    --heatmap-row-width: 140px;
    --heatmap-cell-width: 80px;
  }
}

.heatmap__corner,
.heatmap__rowhead,
.heatmap__colhead {
  background: var(--rvn-panel);
}

.heatmap__rowhead {
  position: sticky;
  left: 0;
  z-index: 2;
  display: flex;
  align-items: center;
  height: 44px;
  padding: 0 8px;
  font-size: 12px;
  line-height: 18px;
  color: var(--td-text-color-secondary);
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  border-right: 1px solid var(--td-component-stroke);
  background: var(--td-bg-color-container);
}

button.heatmap__rowhead {
  gap: 4px;
  border-top: 0;
  border-bottom: 0;
  border-left: 0;
  font-family: inherit;
  text-align: left;
  cursor: pointer;

  &:hover {
    color: var(--td-text-color-primary);
    background: var(--td-bg-color-container-hover);
  }

  &:focus-visible {
    outline: 2px solid var(--td-brand-color);
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
  height: 36px;
  border-right: 1px solid var(--td-component-stroke);
}

.heatmap__colhead {
  height: 36px;
  padding: 0 4px;
  font-size: 11px;
  line-height: 36px;
  color: var(--td-text-color-placeholder);
  text-align: center;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.heatmap__cellwrap {
  min-width: 0;
}

.heatmap__cell {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 44px;
  padding: 2px 4px;
  border: 1px solid transparent;
  border-radius: var(--td-radius-default);
  background: transparent;
  font-family: inherit;
  cursor: default;

  &:focus-visible {
    outline: 2px solid var(--td-brand-color);
    outline-offset: 1px;
  }

  &.is-clickable {
    cursor: pointer;
  }

  &.is-clickable:hover {
    border-color: currentColor;
  }

  &.is-emphasis {
    outline: 2px solid var(--td-brand-color);
    outline-offset: -2px;
  }
}

.heatmap__cell-value {
  font-size: 12px;
  line-height: 16px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.heatmap__cell-severe {
  font-size: 10px;
  line-height: 14px;
}

.heatmap__cell--severe {
  background: color-mix(in srgb, var(--rv-vuln-severe) 14%, var(--td-bg-color-container));
  border-color: color-mix(in srgb, var(--rv-vuln-severe) 30%, transparent);
  color: var(--rv-vuln-severe);
}

.heatmap__cell--warning {
  background: color-mix(in srgb, var(--rv-vuln-warning) 14%, var(--td-bg-color-container));
  border-color: color-mix(in srgb, var(--rv-vuln-warning) 30%, transparent);
  color: var(--rv-vuln-warning);
}

.heatmap__cell--observe {
  background: color-mix(in srgb, var(--rv-vuln-observe) 10%, var(--td-bg-color-container));
  border-color: color-mix(in srgb, var(--rv-vuln-observe) 26%, transparent);
  color: var(--rv-vuln-observe);
}

.heatmap__cell--healthy {
  background: color-mix(in srgb, var(--rv-vuln-healthy) 10%, var(--td-bg-color-container));
  border-color: color-mix(in srgb, var(--rv-vuln-healthy) 26%, transparent);
  color: var(--rv-vuln-healthy);
}

.heatmap__cell--none {
  background: var(--td-bg-color-container-hover);
  color: var(--td-text-color-placeholder);
}

.heatmap__cell--missing {
  cursor: default;
}

.heatmap__legend {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 14px;
  margin-top: 10px;
  font-size: 11px;
  color: var(--td-text-color-secondary);
}

.heatmap__legend-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.heatmap__legend-swatch {
  display: inline-block;
  width: 12px;
  height: 12px;
  border: 1px solid transparent;
  border-radius: 3px;
}

.heatmap__legend-swatch.heatmap__cell--severe {
  background: var(--rv-vuln-severe);
}

.heatmap__legend-swatch.heatmap__cell--warning {
  background: var(--rv-vuln-warning);
}

.heatmap__legend-swatch.heatmap__cell--observe {
  background: var(--rv-vuln-observe);
}

.heatmap__legend-swatch.heatmap__cell--healthy {
  background: var(--rv-vuln-healthy);
}

.heatmap__legend-swatch.heatmap__cell--none {
  background: var(--td-bg-color-container-active);
}

.heatmap__legend-note {
  color: var(--td-text-color-placeholder);
}

.heatmap__tip {
  max-width: 260px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
  line-height: 18px;
}

.heatmap__tip-title {
  font-weight: 600;
  margin-bottom: 2px;
}

.heatmap__tip-note {
  color: var(--td-text-color-placeholder);
}
</style>
