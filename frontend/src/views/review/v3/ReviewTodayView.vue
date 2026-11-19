<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ReviewHeatmap, ReviewHeatmapCell, ReviewQueue, ReviewSessionState } from '@/api/review'
import ReviewHeatmapPanel from './ReviewHeatmap.vue'
import ReviewPriorities from './ReviewPriorities.vue'
import goalIcon from '@/assets/icons/static/goal.png'
import checklistIcon from '@/assets/icons/static/checklist.png'
import bookIcon from '@/assets/icons/static/book.png'
import clockIcon from '@/assets/icons/static/clock.png'
import fireIcon from '@/assets/icons/static/fire.png'
import rocketIcon from '@/assets/icons/static/rocket.png'
import sparklesIcon from '@/assets/icons/static/sparkles.png'

const props = defineProps<{
  queue: ReviewQueue | null
  activeSession: ReviewSessionState | null
  selectedGroupName: string
  selectedGroupId: string
  groupOptions: Array<{ label: string; value: string }>
  heatmap: ReviewHeatmap | null
  heatmapLoading: boolean
  heatmapError: string
  loading: boolean
  error: string
}>()

const emit = defineEmits<{
  start: []
  retry: []
  retryHeatmap: []
  openCluster: [cell: ReviewHeatmapCell]
  drill: [groupID: string]
  expand: []
  'update:groupId': [groupId: string]
}>()

const { t } = useI18n()

const dueCount = computed(() => props.queue?.due.length ?? 0)
const newCount = computed(() => props.queue?.new.length ?? 0)
const completedToday = computed(() => props.queue?.completed_today ?? 0)
const streakDays = computed(() => props.queue?.streak_days ?? 0)
const hasQueue = computed(() => dueCount.value + newCount.value > 0)
const hasActiveSession = computed(() => Boolean(props.activeSession))

const estimatedMinutes = computed(() => {
  const remaining = dueCount.value + newCount.value
  if (remaining <= 0) return 0
  return Math.max(1, Math.ceil(remaining / 2))
})

const taskTitle = computed(() => {
  if (hasActiveSession.value && props.activeSession) {
    return t('review.v3.activeProgress', {
      completed: props.activeSession.session.completed_count,
      total: props.activeSession.session.planned_count,
    })
  }
  if (hasQueue.value) return t('review.today')
  if (completedToday.value > 0) return t('review.v3.completedToday')
  return t('review.v3.nothingToday')
})

const taskHint = computed(() => {
  if (hasActiveSession.value) return t('review.v3.continueSession')
  if (hasQueue.value) return t('review.startHint')
  if (completedToday.value > 0) return t('review.v3.completedHint', { count: completedToday.value })
  return t('review.v3.nothingHint')
})

const ctaLabel = computed(() => {
  if (hasActiveSession.value) return t('review.v3.continueSession')
  if (hasQueue.value) return t('review.v3.startToday')
  if (completedToday.value > 0) return t('review.v3.completedToday')
  return t('review.v3.nothingToday')
})

const ctaDisabled = computed(() => !hasActiveSession.value && !hasQueue.value)

function onGroupChange(value: string | number | Array<string | number>) {
  const next = Array.isArray(value) ? value[0] : value
  emit('update:groupId', next === undefined ? '' : String(next))
}

const canStart = computed(() => hasQueue.value || hasActiveSession.value)
</script>

<template>
  <div class="today-view">
    <div class="today-view__mobile-group">
      <span class="today-view__mobile-label">{{ t('review.v3.mobileGroup') }}</span>
      <t-select
        :model-value="selectedGroupId"
        :options="groupOptions"
        size="small"
        @change="onGroupChange"
      />
    </div>

    <t-loading :loading="loading" size="small" class="today-view__loading">
      <section class="task-card" data-testid="review-v3-task-bar">
        <div class="task-card__head">
          <div class="task-card__copy">
            <div class="task-card__eyebrow">{{ selectedGroupName || t('review.v3.allGroups') }}</div>
            <div class="task-card__title-row">
              <img class="rv-icon rv-icon--lg" :src="goalIcon" alt="" />
              <h2 class="task-card__title">{{ taskTitle }}</h2>
            </div>
            <p class="task-card__hint">{{ taskHint }}</p>
          </div>
          <t-button
            theme="primary"
            size="large"
            class="task-card__cta"
            :disabled="ctaDisabled || loading"
            data-testid="review-v3-start"
            @click="emit('start')"
          >
            <img v-if="!ctaDisabled" class="rv-icon rv-icon--sm task-card__cta-icon" :src="rocketIcon" alt="" />
            {{ ctaLabel }}
          </t-button>
        </div>

        <div class="task-card__metrics">
          <div class="metric">
            <div class="metric__label">
              <img class="rv-icon rv-icon--sm" :src="checklistIcon" alt="" />{{ t('review.v3.remainingDue') }}
            </div>
            <div class="metric__value">{{ dueCount }}</div>
          </div>
          <div class="metric">
            <div class="metric__label">
              <img class="rv-icon rv-icon--sm" :src="bookIcon" alt="" />{{ t('review.v3.remainingNew') }}
            </div>
            <div class="metric__value">{{ newCount }}</div>
          </div>
          <div class="metric" :title="t('review.v3.estimatedHint')">
            <div class="metric__label">
              <img class="rv-icon rv-icon--sm" :src="clockIcon" alt="" />{{ t('review.v3.estimatedLabel') }}
            </div>
            <div class="metric__value">
              {{ estimatedMinutes ? t('review.v3.estimatedValue', { count: estimatedMinutes }) : '—' }}
            </div>
          </div>
          <div class="metric">
            <div class="metric__label">
              <img class="rv-icon rv-icon--sm" :src="fireIcon" alt="" />{{ t('review.stats.streak') }}
            </div>
            <div class="metric__value">{{ t('review.v3.streakDays', { count: streakDays }) }}</div>
          </div>
        </div>

        <div v-if="error" class="task-card__error">
          <t-alert theme="error" :message="error" />
          <t-button size="small" variant="outline" @click="emit('retry')">{{ t('review.refresh') }}</t-button>
        </div>
      </section>

      <div class="today-view__panels">
        <ReviewHeatmapPanel
          :heatmap="heatmap"
          :loading="heatmapLoading"
          :error="heatmapError"
          :can-start="canStart"
          @select="cell => emit('openCluster', cell)"
          @drill="groupID => emit('drill', groupID)"
          @expand="emit('expand')"
          @start="emit('start')"
          @retry="emit('retryHeatmap')"
        />

        <ReviewPriorities :heatmap="heatmap" @select="cell => emit('openCluster', cell)" />
      </div>

      <div v-if="heatmap?.insight" class="today-view__insight" data-testid="review-v3-insight">
        <img class="rv-icon rv-icon--sm" :src="sparklesIcon" alt="" />
        <span class="today-view__insight-label">{{ t('review.v3.insightTitle') }}</span>
        <span>{{ heatmap.insight.message }}</span>
      </div>
    </t-loading>
  </div>
</template>

<style lang="less" scoped>
.today-view {
  min-width: 0;
}

.today-view__loading :deep(.t-loading__parent) {
  min-height: 160px;
}

.today-view__mobile-group {
  display: none;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;

  :deep(.t-select) {
    flex: 1;
    min-width: 0;
  }
}

.today-view__mobile-label {
  flex: 0 0 auto;
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.task-card {
  padding: 20px;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--td-brand-color) 4%, var(--td-bg-color-container)) 0%, var(--td-bg-color-container) 42%);
  border: 1px solid var(--td-component-stroke);
  border-radius: 0 0 var(--td-radius-medium) var(--td-radius-medium);
}

.task-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
}

.task-card__copy {
  min-width: 0;
}

.task-card__eyebrow {
  font-size: 12px;
  line-height: 18px;
  color: var(--td-text-color-placeholder);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-card__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 2px;
}

.task-card__title {
  margin: 0;
  font-size: 20px;
  line-height: 28px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.task-card__hint {
  margin: 4px 0 0;
  font-size: 13px;
  line-height: 20px;
  color: var(--td-text-color-secondary);
}

.task-card__cta {
  flex: 0 0 auto;

  :deep(.t-button__text),
  :deep(> span) {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
}

.task-card__cta-icon {
  margin-right: 2px;
}

.task-card__metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-top: 18px;
  padding-top: 16px;
  border-top: 1px solid var(--td-component-stroke);
}

.metric {
  min-width: 0;
}

.metric__label {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  line-height: 18px;
  color: var(--td-text-color-placeholder);
}

.metric__value {
  margin-top: 2px;
  font-size: 18px;
  line-height: 26px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--td-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-card__error {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 14px;

  :deep(.t-alert) {
    flex: 1;
    min-width: 0;
  }
}

.today-view__panels {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-top: 16px;
}

.today-view__insight {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 12px;
  padding: 9px 14px;
  border: 1px solid color-mix(in srgb, var(--td-warning-color) 28%, transparent);
  border-radius: var(--td-radius-default);
  background: color-mix(in srgb, var(--td-warning-color) 8%, var(--td-bg-color-container));
  font-size: 12px;
  line-height: 20px;
  color: var(--td-text-color-secondary);

  svg {
    color: var(--td-warning-color);
  }
}

.today-view__insight-label {
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin-right: 2px;
}

@media (max-width: 960px) {
  .today-view__panels {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 760px) {
  .today-view__mobile-group {
    display: flex;
  }

  .task-card__head {
    flex-direction: column;
    gap: 12px;
  }

  .task-card__cta {
    width: 100%;
  }

  .task-card__metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    row-gap: 14px;
  }
}
</style>
