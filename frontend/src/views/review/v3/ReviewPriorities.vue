<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ReviewHeatmap, ReviewHeatmapCell } from '@/api/review'
import ReviewLevelTag from './ReviewLevelTag.vue'
import { formatIntensity } from './vulnerability'
import goalIcon from '@/assets/icons/static/goal.png'
import bookIcon from '@/assets/icons/static/book.png'
import arrowIcon from '@/assets/icons/static/arrow-right.png'

const props = defineProps<{
  heatmap: ReviewHeatmap | null
}>()

const emit = defineEmits<{
  select: [cell: ReviewHeatmapCell]
}>()

const { t } = useI18n()

const rowPaths = computed(() => {
  const map = new Map<string, string>()
  for (const row of props.heatmap?.rows ?? []) {
    map.set(row.id, row.path)
  }
  return map
})

const columnTitles = computed(() => {
  const map = new Map<string, string>()
  for (const column of props.heatmap?.columns ?? []) {
    map.set(column.key, column.title)
  }
  return map
})

const priorities = computed(() => props.heatmap?.priorities ?? [])
const observeCount = computed(() => props.heatmap?.observe_count ?? 0)

function nameOf(cell: ReviewHeatmapCell) {
  const path = rowPaths.value.get(cell.row_id) || cell.row_id
  const title = columnTitles.value.get(cell.column_key) || cell.column_key
  return `${path} · ${title}`
}
</script>

<template>
  <t-card class="priorities" :bordered="false" data-testid="review-v3-priorities">
    <template #title>
      <div class="priorities__title">
        <img :src="goalIcon" alt="" />{{ t('review.v3.prioritiesTitle') }}
      </div>
    </template>
    <div v-if="priorities.length" class="priorities__list">
      <button
        v-for="cell in priorities"
        :key="`${cell.row_id}-${cell.column_key}`"
        type="button"
        class="priority-item"
        @click="emit('select', cell)"
      >
        <div class="priority-item__row">
          <ReviewLevelTag :level="cell.level" />
          <span class="priority-item__intensity">{{ formatIntensity(cell.intensity) }}</span>
        </div>
        <div class="priority-item__name" :title="nameOf(cell)">{{ nameOf(cell) }}</div>
        <div class="priority-item__row">
          <span class="priority-item__meta">
            {{ t('review.v3.prioritySummary', { severe: cell.severe, warning: cell.warning }) }}
          </span>
          <span class="priority-item__action">
            {{ t('review.v3.goReinforce') }}
            <img class="inline-hand-icon" :src="arrowIcon" alt="" />
          </span>
        </div>
      </button>
    </div>

    <div v-else class="priorities__empty">
      <img class="rv-icon rv-icon--lg" :src="bookIcon" alt="" />
      <p>{{ t('review.v3.prioritiesEmpty') }}</p>
    </div>

    <p v-if="observeCount > 0" class="priorities__observe">
      {{ t('review.v3.observeSummary', { count: observeCount }) }}
    </p>
  </t-card>
</template>

<style lang="less" scoped>
.inline-hand-icon {
  width: 12px;
  height: 12px;
  object-fit: contain;
}

.priorities {
  min-width: 0;

  :deep(.t-card__header) {
    min-height: 44px;
    padding: 0 16px;
  }

  :deep(.t-card__body) {
    padding: 0 16px 16px;
  }
}

.priorities__list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.priorities__title {
  display: inline-flex;
  align-items: center;
  gap: 8px;

  img {
    width: 20px;
    height: 20px;
    object-fit: contain;
  }
}

.priority-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-default);
  background: var(--td-bg-color-container);
  font-family: inherit;
  text-align: left;
  cursor: pointer;

  &:hover {
    background: var(--td-bg-color-container-hover);
    border-color: var(--td-component-border);
  }

  &:focus-visible {
    outline: 2px solid var(--td-brand-color);
    outline-offset: 1px;
  }
}

.priority-item__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
}

.priority-item__intensity {
  font-size: 13px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--td-text-color-primary);
}

.priority-item__name {
  font-size: 13px;
  line-height: 19px;
  color: var(--td-text-color-primary);
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.priority-item__meta {
  font-size: 11px;
  line-height: 17px;
  color: var(--td-text-color-secondary);
}

.priority-item__action {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  font-size: 11px;
  color: var(--td-text-color-brand);
}

.priorities__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 132px;
  color: var(--td-text-color-placeholder);
  font-size: 13px;

  p {
    margin: 0;
  }
}

.priorities__observe {
  margin: 10px 0 0;
  font-size: 11px;
  color: var(--td-text-color-placeholder);
}
</style>
