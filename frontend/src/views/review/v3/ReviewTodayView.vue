<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ReviewGroup, ReviewHeatmap, ReviewHeatmapCell, ReviewQueue, ReviewSessionState } from '@/api/review'
import ReviewGroupRail from './ReviewGroupRail.vue'
import ReviewHeatmapPanel from './ReviewHeatmap.vue'
import ReviewPriorities from './ReviewPriorities.vue'
import ReviewIcon from './ReviewIcon.vue'
import type { ReviewFlatGroup } from './types'

const props = defineProps<{
  queue: ReviewQueue | null
  activeSession: ReviewSessionState | null
  selectedGroupName: string
  selectedGroupId: string
  groupOptions: Array<{ label: string; value: string }>
  groups: ReviewFlatGroup[]
  allCardCount: number
  allDueCount: number
  heatmap: ReviewHeatmap | null
  heatmapLoading: boolean
  heatmapError: string
  rowGroupIds: string[]
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
  selectGroup: [groupId: string]
  createGroup: [parentId: string]
  renameGroup: [group: ReviewGroup]
  deleteGroup: [group: ReviewGroup]
  'update:rowGroupIds': [ids: string[]]
  'update:groupId': [groupId: string]
}>()

const { t } = useI18n()

const dueCount = computed(() => props.queue?.due.length ?? 0)
const newCount = computed(() => props.queue?.new.length ?? 0)
const completedToday = computed(() => props.queue?.completed_today ?? 0)
const hasQueue = computed(() => dueCount.value + newCount.value > 0)
const hasActiveSession = computed(() => Boolean(props.activeSession))

const todayLabel = computed(() => {
  const now = new Date()
  return `${now.getFullYear()} 年 ${now.getMonth() + 1} 月 ${now.getDate()} 日`
})
const weekdayLabel = computed(() =>
  new Date().toLocaleDateString('zh-CN', { weekday: 'short' }),
)

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
  if (hasQueue.value) return t('review.v3.todayDueTitle', { count: dueCount.value + newCount.value })
  if (completedToday.value > 0) return t('review.v3.completedToday')
  return t('review.v3.nothingToday')
})

const taskHint = computed(() => {
  if (hasActiveSession.value) return t('review.v3.continueSession')
  if (hasQueue.value) return t('review.v3.todayDueHint', { count: dueCount.value })
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
const canStart = computed(() => hasQueue.value || hasActiveSession.value)
const reviewCardTone = computed(() => {
  if (hasActiveSession.value || hasQueue.value) return 'blue'
  if (completedToday.value > 0) return 'green'
  return 'gray'
})

const taskTotal = computed(() => dueCount.value + newCount.value + completedToday.value)
const composition = computed(() => {
  const total = taskTotal.value || 1
  return {
    due: Math.round((dueCount.value / total) * 100),
    new: Math.round((newCount.value / total) * 100),
    done: Math.round((completedToday.value / total) * 100),
  }
})
const completionRate = computed(() => {
  if (!taskTotal.value) return 0
  return Math.min(100, Math.round((completedToday.value / taskTotal.value) * 100))
})

function onGroupChange(value: string | number | Array<string | number>) {
  const next = Array.isArray(value) ? value[0] : value
  emit('update:groupId', next === undefined ? '' : String(next))
}
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
      <div class="rv-props">
        <div class="rv-prop">
          <span class="rv-prop__key">{{ t('review.v3.todayDate') }}</span>
          <b>{{ todayLabel }}</b>
          <span class="rv-note">{{ weekdayLabel }}</span>
        </div>
      </div>

      <ReviewPriorities :heatmap="heatmap" @select="cell => emit('openCluster', cell)" />

      <div class="today-review">
        <div class="rv-h2">{{ t('review.v3.todayReview') }}</div>
        <section class="today-review__card" :class="`today-review__card--${reviewCardTone}`" data-testid="review-v3-task-bar">
          <ReviewIcon name="clock" :size="20" :tilt="-1" class="today-review__icon" />
          <div class="today-review__copy">
            <strong>{{ taskTitle }}</strong>
            <p>{{ taskHint }}</p>
          </div>
          <button
            v-if="!ctaDisabled"
            type="button"
            class="rv-btn rv-btn--primary today-review__cta"
            :disabled="loading"
            data-testid="review-v3-start"
            @click="emit('start')"
          >
            {{ ctaLabel }}
            <ReviewIcon name="arrow-right" :size="13" />
          </button>
        </section>
      </div>

      <div v-if="error" class="today-view__error">
        <t-alert theme="error" :message="error" />
        <button type="button" class="rv-btn rv-btn--ghost rv-btn--sm" @click="emit('retry')">
          {{ t('review.refresh') }}
        </button>
      </div>

      <div class="today-view__grid">
        <div class="today-view__tree">
          <ReviewGroupRail
            :groups="groups"
            :selected-group-id="selectedGroupId"
            :all-card-count="allCardCount"
            :all-due-count="allDueCount"
            @select="groupId => emit('selectGroup', groupId)"
            @create="parentId => emit('createGroup', parentId)"
            @rename="group => emit('renameGroup', group)"
            @delete="group => emit('deleteGroup', group)"
          />
        </div>

        <div class="today-view__compose">
          <div class="rv-h2">{{ t('review.v3.taskComposition') }}</div>
          <div class="stackbar">
            <i class="is-due" :style="{ width: `${composition.due}%` }" />
            <i class="is-new" :style="{ width: `${composition.new}%` }" />
            <i class="is-done" :style="{ width: `${composition.done}%` }" />
          </div>
          <div class="legend">
            <span class="legend__item"><i class="legend__sq is-due" />{{ t('review.v3.remainingDue') }} {{ dueCount }}</span>
            <span class="legend__item"><i class="legend__sq is-new" />{{ t('review.v3.remainingNew') }} {{ newCount }}</span>
            <span class="legend__item"><i class="legend__sq is-done" />{{ t('review.v3.completedToday') }} {{ completedToday }}</span>
          </div>
          <div class="progressline">
            <div class="progressline__bar"><i :style="{ width: `${completionRate}%` }" /></div>
            <b>{{ completedToday }} / {{ taskTotal }}</b>
            <span class="rv-note">{{ t('review.v3.completedLabel') }}</span>
          </div>
          <p class="rv-note today-view__estimate">
            {{ estimatedMinutes ? t('review.v3.estimatedValue', { count: estimatedMinutes }) : '—' }}
            · {{ t('review.v3.estimatedHint') }}
          </p>

          <div class="rv-h3">{{ t('review.v3.masteryLegend') }}</div>
          <p class="rv-note">
            {{ t('review.v3.masteryLegendHint') }}
          </p>
        </div>
      </div>

      <ReviewHeatmapPanel
        :heatmap="heatmap"
        :loading="heatmapLoading"
        :error="heatmapError"
        :can-start="canStart"
        :groups="groups"
        :row-group-ids="rowGroupIds"
        @select="cell => emit('openCluster', cell)"
        @drill="groupID => emit('drill', groupID)"
        @expand="emit('expand')"
        @start="emit('start')"
        @retry="emit('retryHeatmap')"
        @update:row-group-ids="ids => emit('update:rowGroupIds', ids)"
      />

      <div v-if="heatmap?.insight" class="rv-callout rv-callout--blue" data-testid="review-v3-insight">
        <ReviewIcon name="target" :size="20" :tilt="-1" />
        <div>
          <strong>{{ t('review.v3.insightTitle') }}</strong>
          {{ heatmap.insight.message }}
        </div>
      </div>
    </t-loading>
  </div>
</template>

<style lang="less" scoped>
.today-view {
  min-width: 0;
  max-width: 1120px;
  margin: 0 auto;
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
  font-size: 14px;
  color: var(--rvn-muted);
}

.today-review {
  margin-top: 32px;

  .rv-h2 {
    margin-top: 0;
  }
}

.today-review__card {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding: 14px 16px;
  background: var(--rvn-paper);
  border: 1px solid var(--rvn-line);
  border-radius: 7px 6px 8px 7px / 6px 8px 7px 6px;
}

.today-review__card--blue {
  background: var(--rvn-accent-soft);
  border-color: #c9d6de;
}

.today-review__card--green {
  background: var(--rvn-green-soft);
  border-color: #cdd8c9;
}

.today-review__card--gray {
  background: var(--rvn-gray-soft);
  border-color: var(--rvn-line);
}

.today-review__icon {
  margin-top: 2px;
  color: var(--rvn-ink-soft);
}

.today-review__copy {
  min-width: 0;

  strong {
    font-size: 14px;
    color: var(--rvn-ink);
  }

  p {
    margin: 4px 0 0;
    font-size: 14px;
    line-height: 1.55;
    color: var(--rvn-ink-soft);
  }
}

.today-review__cta {
  flex: none;
  margin-left: auto;
}

.today-view__callout-copy {
  min-width: 0;

  strong {
    font-size: 14.5px;
  }

  p {
    margin: 3px 0 0;
    font-size: 14px;
    line-height: 1.55;
    color: var(--rvn-red-deep);
    opacity: 0.86;
  }
}

.today-view__error {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 14px 0;

  :deep(.t-alert) {
    flex: 1;
    min-width: 0;
  }
}

.today-view__grid {
  display: grid;
  grid-template-columns: minmax(0, 64%) minmax(260px, 1fr);
  gap: 48px;
  align-items: start;
  margin-top: 40px;
  padding-right: 16px;
}

.today-view__tree {
  min-width: 0;
}

.today-view__compose {
  min-width: 0;
  padding-top: 0;

  .rv-h2 {
    margin-top: 0;
    margin-bottom: 12px;
  }

  .rv-h3 {
    margin-top: 32px;
  }
}

.today-view__estimate {
  margin-top: 6px;
}

.stackbar {
  display: flex;
  height: 10px;
  overflow: hidden;
  background: var(--rvn-line);
  border-radius: 999px;

  i {
    display: block;
    height: 100%;
  }

  .is-due { background: var(--rvn-red); }
  .is-new { background: var(--rvn-accent); }
  .is-done { background: var(--rvn-green); }
}

.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin-top: 10px;
  font-size: 14px;
  color: var(--rvn-ink-soft);
}

.legend__item {
  display: inline-flex;
  gap: 6px;
  align-items: center;
}

.legend__sq {
  width: 9px;
  height: 9px;
  border-radius: 2px;

  &.is-due { background: var(--rvn-red); }
  &.is-new { background: var(--rvn-accent); }
  &.is-done { background: var(--rvn-green); }
}

.progressline {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 16px;
  font-size: 14px;
  color: var(--rvn-ink-soft);

  b {
    color: var(--rvn-ink);
  }
}

.progressline__bar {
  flex: 1;
  height: 6px;
  overflow: hidden;
  background: var(--rvn-line);
  border-radius: 999px;

  i {
    display: block;
    height: 100%;
    background: var(--rvn-accent);
    border-radius: 999px;
  }
}

@media (max-width: 960px) {
  .today-view__grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 32px;
    padding-right: 0;
  }
}

@media (max-width: 760px) {
  .today-view__mobile-group {
    display: flex;
  }

  .rv-callout {
    flex-wrap: wrap;
  }

  .rv-callout__act {
    width: 100%;
    margin-left: 32px;
  }
}
</style>
