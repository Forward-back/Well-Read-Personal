<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ReviewHeatmap, ReviewHeatmapCell } from '@/api/review'
import ReviewLevelTag from './ReviewLevelTag.vue'
import ReviewIcon from './ReviewIcon.vue'
import { formatIntensity, normalizeVulnerability } from './vulnerability'

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
  <section class="priorities" data-testid="review-v3-priorities">
    <div class="rv-h2">
      {{ t('review.v3.prioritiesTitle') }}
      <span class="rv-note">{{ t('review.v3.prioritiesSubtitle') }}</span>
    </div>
    <div v-if="priorities.length" class="priorities__list">
      <button
        v-for="cell in priorities"
        :key="`${cell.row_id}-${cell.column_key}`"
        type="button"
        class="priority-item"
        :class="`priority-item--${normalizeVulnerability(cell.level)}`"
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
            <ReviewIcon name="arrow-right" :size="12" />
          </span>
        </div>
      </button>
    </div>

    <div v-else class="priorities__empty">
      <ReviewIcon name="book" :size="24" />
      <p>{{ t('review.v3.prioritiesEmpty') }}</p>
    </div>

    <p v-if="observeCount > 0" class="priorities__observe">
      {{ t('review.v3.observeSummary', { count: observeCount }) }}
    </p>
  </section>
</template>

<style lang="less" scoped>
.priorities {
  min-width: 0;
  margin-top: 4px;
}

.priorities .rv-h2 {
  margin-top: 0;
}

.priorities__list {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 10px;
}

.priority-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
  padding: 12px 14px;
  font-family: inherit;
  text-align: left;
  cursor: pointer;
  background: var(--rvn-paper);
  border: 1px solid var(--rvn-line);
  border-radius: 7px 6px 8px 7px / 6px 8px 7px 6px;
  transition: background 0.18s ease, border-color 0.18s ease;

  &:hover {
    border-color: var(--rvn-line-strong);
    filter: brightness(0.98);
  }

  &:focus-visible {
    outline: 2px solid var(--rvn-brand);
    outline-offset: 1px;
  }
}

.priority-item--severe {
  background: var(--rvn-red-soft);
  border-color: #dcc3bc;
}

.priority-item--warning {
  background: var(--rvn-orange-soft);
  border-color: #ddcdb9;
}

.priority-item--observe {
  background: var(--rvn-accent-soft);
  border-color: #c9d6de;
}

.priority-item--healthy {
  background: var(--rvn-green-soft);
  border-color: #cdd8c9;
}

.priority-item--none {
  background: var(--rvn-gray-soft);
  border-color: var(--rvn-line);
}

.priority-item__row {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
  min-width: 0;
}

.priority-item__intensity {
  font-size: 14px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--rvn-ink);
}

.priority-item__name {
  overflow: hidden;
  font-size: 14px;
  line-height: 19px;
  color: var(--rvn-ink);
  white-space: nowrap;
  text-overflow: ellipsis;
}

.priority-item__meta {
  font-size: 14px;
  line-height: 17px;
  color: var(--rvn-muted);
}

.priority-item__action {
  display: inline-flex;
  gap: 2px;
  align-items: center;
  font-size: 14px;
  color: var(--rvn-brand-deep);
}

.priorities__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 120px;
  font-size: 14px;
  color: var(--rvn-muted);
  background: var(--rvn-surface);
  border: 1px dashed var(--rvn-line-strong);
  border-radius: 7px 6px 8px 7px / 6px 8px 7px 6px;

  p {
    margin: 0;
  }
}

.priorities__observe {
  margin: 10px 0 0;
  font-size: 14px;
  color: var(--rvn-muted);
}

@media (max-width: 760px) {
  .priorities__list {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
