/**
 * Stats tab for review V3: global four-tier dashboard with empty-state gates,
 * ranking, priority suggestions and session history. ECharts is loaded only
 * through this async component.
 */
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts/core'
import { BarChart, LineChart, PieChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { MessagePlugin } from 'tdesign-vue-next'

import {
  getReviewStats,
  listReviewSessions,
  type ReviewSession,
  type ReviewStats,
} from '@/api/review'
import bookIcon from '@/assets/icons/static/book.png'
import checklistIcon from '@/assets/icons/static/checklist.png'
import goalIcon from '@/assets/icons/static/goal.png'
import fireIcon from '@/assets/icons/static/fire.png'
import chartIcon from '@/assets/icons/static/chart.png'
import calendarIcon from '@/assets/icons/static/calendar.png'
import brainIcon from '@/assets/icons/static/brain.png'
import trophyIcon from '@/assets/icons/static/trophy.png'

const emit = defineEmits<{
  (e: 'review-group', groupId: string): void
}>()

echarts.use([BarChart, LineChart, PieChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

const { t } = useI18n()

const stats = ref<ReviewStats | null>(null)
const sessions = ref<ReviewSession[]>([])
const days = ref(30)
const loading = ref(false)
const error = ref('')

const distributionEl = ref<HTMLDivElement | null>(null)
const trendEl = ref<HTMLDivElement | null>(null)
const dueEl = ref<HTMLDivElement | null>(null)
const curveEl = ref<HTMLDivElement | null>(null)
const charts: echarts.ECharts[] = []
let resizeObserver: ResizeObserver | null = null
let themeObserver: MutationObserver | null = null
let loadSeq = 0

const distributionTotal = computed(() => stats.value?.rating_distribution.total ?? 0)
const hasDistribution = computed(() => distributionTotal.value >= 5)
const hasTrend = computed(() => stats.value?.rating_trend?.some(point => point.total > 0) ?? false)
const hasDueForecast = computed(() => stats.value?.due_forecast?.some(point => point.count > 0) ?? false)
const curveSamples = computed(() =>
  (stats.value?.forgetting_curve ?? []).reduce((total, point) => total + (point.sample || 0), 0),
)
const hasCurve = computed(() => curveSamples.value >= 10)
const priorities = computed(() => (stats.value?.weak_topics ?? []).slice(0, 3))
const hasSessions = computed(() => sessions.value.length > 0)

const kpis = computed(() => [
  { label: t('review.totalCards'), value: String(stats.value?.total_cards ?? 0), hint: '', icon: bookIcon },
  { label: t('review.stats.reviewed'), value: String(stats.value?.reviewed ?? 0), hint: `${days.value} ${t('review.stats.daysUnit')}`, icon: checklistIcon },
  {
    label: t('review.stats.retention'),
    value: stats.value && stats.value.rating_distribution.total > 0
      ? `${Math.round((stats.value.retention ?? 0) * 100)}%`
      : '—',
    hint: t('review.v3.statsPage.retentionHint'),
    icon: goalIcon,
  },
  { label: t('review.stats.streak'), value: String(stats.value?.streak_days ?? 0), hint: t('review.stats.daysUnit'), icon: fireIcon },
])

function cssVar(name: string, fallback: string) {
  const root = document.querySelector('.review-v3') || document.documentElement
  const value = getComputedStyle(root).getPropertyValue(name).trim()
  return value || fallback
}

function palette() {
  const dark = document.documentElement.getAttribute('theme-mode') === 'dark'
  return {
    brand: '#8fa88b',
    success: '#8fa88b',
    warning: '#c9a96e',
    error: '#c9a4a0',
    info: '#9db0be',
    text: cssVar('--td-text-color-secondary', '#787774'),
    primary: cssVar('--td-text-color-primary', '#373530'),
    border: cssVar('--td-component-stroke', '#e9e7e2'),
    background: dark ? '#2a2926' : '#ffffff',
  }
}

function baseOption() {
  const c = palette()
  return {
    color: [c.info, c.success, c.warning, c.error],
    textStyle: { color: c.text, fontSize: 12 },
    grid: { left: 8, right: 12, top: 28, bottom: 34, containLabel: true },
    tooltip: {
      trigger: 'axis' as const,
      backgroundColor: c.background,
      borderColor: c.border,
      textStyle: { color: c.primary, fontSize: 12 },
    },
    legend: {
      bottom: 0,
      icon: 'roundRect',
      itemWidth: 10,
      itemHeight: 10,
      textStyle: { color: c.text, fontSize: 12 },
    },
  }
}

function categoryAxis(c: ReturnType<typeof palette>, data: string[]) {
  return {
    type: 'category' as const,
    data,
    axisLabel: { color: c.text, fontSize: 11, hideOverlap: true },
    axisLine: { lineStyle: { color: c.border } },
    axisTick: { show: false },
  }
}

function valueAxis(c: ReturnType<typeof palette>, extra: Record<string, unknown> = {}) {
  return {
    type: 'value' as const,
    axisLabel: { color: c.text, fontSize: 11 },
    axisLine: { show: false },
    axisTick: { show: false },
    splitLine: { lineStyle: { color: c.border } },
    ...extra,
  }
}

function mount(el: HTMLDivElement | null): echarts.ECharts | null {
  if (!el) return null
  const chart = echarts.init(el)
  charts.push(chart)
  return chart
}

function resetCharts() {
  charts.splice(0).forEach(chart => chart.dispose())
}

function renderCharts() {
  resetCharts()
  const data = stats.value
  if (!data) return
  const c = palette()

  if (distributionEl.value && hasDistribution.value) {
    const chart = mount(distributionEl.value)
    chart?.setOption({
      ...baseOption(),
      tooltip: {
        trigger: 'item',
        backgroundColor: c.background,
        borderColor: c.border,
        textStyle: { color: c.primary, fontSize: 12 },
      },
      series: [{
        type: 'pie',
        radius: ['48%', '68%'],
        center: ['50%', '42%'],
        label: { show: false },
        itemStyle: { borderColor: c.background, borderWidth: 2 },
        data: [
          { name: t('review.v3.rating.easy'), value: data.rating_distribution.easy, itemStyle: { color: c.info } },
          { name: t('review.v3.rating.remembered'), value: data.rating_distribution.remembered, itemStyle: { color: c.success } },
          { name: t('review.v3.rating.fuzzy'), value: data.rating_distribution.fuzzy, itemStyle: { color: c.warning } },
          { name: t('review.v3.rating.forgot'), value: data.rating_distribution.forgot, itemStyle: { color: c.error } },
        ],
      }],
    }, true)
  }

  if (trendEl.value && hasTrend.value) {
    const chart = mount(trendEl.value)
    const option = baseOption()
    option.grid = { left: 8, right: 8, top: 28, bottom: 44, containLabel: true }
    chart?.setOption({
      ...option,
      xAxis: categoryAxis(c, data.rating_trend.map(point => point.date.slice(5))),
      yAxis: [
        valueAxis(c),
        valueAxis(c, {
          min: 0,
          max: 1,
          axisLabel: { color: c.text, fontSize: 11, formatter: (value: number) => `${Math.round(value * 100)}%` },
          splitLine: { show: false },
        }),
      ],
      series: [
        { name: t('review.v3.rating.easy'), type: 'bar', stack: 'rating', barMaxWidth: 18, data: data.rating_trend.map(point => point.easy), itemStyle: { color: c.info } },
        { name: t('review.v3.rating.remembered'), type: 'bar', stack: 'rating', barMaxWidth: 18, data: data.rating_trend.map(point => point.remembered), itemStyle: { color: c.success } },
        { name: t('review.v3.rating.fuzzy'), type: 'bar', stack: 'rating', barMaxWidth: 18, data: data.rating_trend.map(point => point.fuzzy), itemStyle: { color: c.warning } },
        { name: t('review.v3.rating.forgot'), type: 'bar', stack: 'rating', barMaxWidth: 18, data: data.rating_trend.map(point => point.forgot), itemStyle: { color: c.error } },
        { name: t('review.stats.retention'), type: 'line', yAxisIndex: 1, smooth: true, symbolSize: 5, data: data.rating_trend.map(point => (point.total ? point.retention : null)), itemStyle: { color: c.primary }, lineStyle: { width: 2 } },
      ],
    }, true)
  }

  if (dueEl.value && hasDueForecast.value) {
    const chart = mount(dueEl.value)
    const option = baseOption()
    option.grid = { left: 8, right: 12, top: 24, bottom: 24, containLabel: true }
    chart?.setOption({
      ...option,
      legend: { show: false },
      xAxis: categoryAxis(c, data.due_forecast.map(point => point.date.slice(5))),
      yAxis: valueAxis(c, { minInterval: 1 }),
      series: [{
        type: 'bar',
        data: data.due_forecast.map(point => point.count),
        barMaxWidth: 24,
        label: { show: true, position: 'top', color: c.text, fontSize: 11, formatter: (params: { value: number }) => (params.value > 0 ? String(params.value) : '') },
        itemStyle: { color: c.brand, borderRadius: [3, 3, 0, 0] },
      }],
    }, true)
  }

  if (curveEl.value && hasCurve.value) {
    const chart = mount(curveEl.value)
    const option = baseOption()
    option.grid = { left: 8, right: 12, top: 28, bottom: 44, containLabel: true }
    chart?.setOption({
      ...option,
      xAxis: categoryAxis(c, data.forgetting_curve.map(point => String(point.day))),
      yAxis: valueAxis(c, {
        min: 0,
        max: 1,
        axisLabel: { color: c.text, fontSize: 11, formatter: (value: number) => `${Math.round(value * 100)}%` },
      }),
      series: [
        { name: t('review.stats.actual'), type: 'line', smooth: true, symbolSize: 4, data: data.forgetting_curve.map(point => point.actual), itemStyle: { color: c.brand }, lineStyle: { width: 2 } },
        { name: t('review.stats.theoretical'), type: 'line', smooth: true, symbolSize: 4, lineStyle: { type: 'dashed', width: 2 }, data: data.forgetting_curve.map(point => point.theoretical), itemStyle: { color: c.warning } },
      ],
    }, true)
  }
}

function scheduleRender() {
  void nextTick(() => {
    renderCharts()
    charts.forEach(chart => chart.resize())
  })
}

async function loadAll() {
  const seq = ++loadSeq
  loading.value = true
  error.value = ''
  try {
    const [statsRes, sessionsRes] = await Promise.all([
      getReviewStats(days.value),
      listReviewSessions(50),
    ])
    if (seq !== loadSeq) return
    stats.value = statsRes.data || null
    sessions.value = sessionsRes.data || []
  } catch (err: any) {
    if (seq !== loadSeq) return
    error.value = err?.message || t('review.loadFailed')
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

function changeDays(value: number) {
  if (days.value === value) return
  days.value = value
  void loadAll()
}

function formatTime(value?: string | null) {
  if (!value) return '—'
  return new Date(value).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function csvCell(value: unknown) {
  const text = String(value ?? '')
  return `"${text.replace(/"/g, '""')}"`
}

function exportSessionsCSV() {
  if (!sessions.value.length) return
  const headers = [
    'id', 'mode', 'status', 'planned_count', 'completed_count',
    'remembered_count', 'fuzzy_count', 'forgot_count', 'review_day',
    'started_at', 'completed_at', 'easy_count',
  ]
  const rows = sessions.value.map(session => [
    session.id,
    session.mode,
    session.status,
    session.planned_count,
    session.completed_count,
    session.remembered_count,
    session.fuzzy_count,
    session.forgot_count,
    session.review_day,
    session.started_at,
    session.completed_at || '',
    session.easy_count,
  ])
  const csv = [headers, ...rows].map(row => row.map(csvCell).join(',')).join('\n')
  const blob = new Blob([`﻿${csv}`], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `wellread-review-sessions-${new Date().toISOString().slice(0, 10)}.csv`
  link.click()
  URL.revokeObjectURL(url)
  MessagePlugin.success(t('review.v3.statsPage.exported'))
}

onMounted(() => {
  void loadAll()
  if ('ResizeObserver' in window) {
    resizeObserver = new ResizeObserver(() => charts.forEach(chart => chart.resize()))
  }
  themeObserver = new MutationObserver(() => scheduleRender())
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class', 'theme-mode'] })
})

watch(stats, () => {
  const elements = [distributionEl.value, trendEl.value, dueEl.value, curveEl.value].filter(Boolean) as HTMLDivElement[]
  if (resizeObserver) {
    resizeObserver.disconnect()
    elements.forEach(element => resizeObserver?.observe(element))
  }
  scheduleRender()
}, { deep: true })

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  themeObserver?.disconnect()
  resetCharts()
})
</script>

<template>
  <div class="stats-page" data-testid="review-v3-stats">
    <div class="stats-head">
      <p>{{ t('review.v3.statsPage.subtitle') }}</p>
      <div class="stats-days">
        <button
          v-for="option in [7, 30, 90]"
          :key="option"
          type="button"
          :class="{ 'is-active': days === option }"
          :data-testid="`review-v3-stats-days-${option}`"
          @click="changeDays(option)"
        >
          {{ t(`review.stats.days${option}`) }}
        </button>
      </div>
    </div>

    <t-alert v-if="error" theme="error" :message="error">
      <template #operation>
        <t-button size="small" variant="outline" @click="loadAll">{{ t('common.retry') }}</t-button>
      </template>
    </t-alert>

    <t-loading :loading="loading" size="small">
      <div class="stats-kpis">
        <t-card v-for="kpi in kpis" :key="kpi.label" :bordered="false" class="stats-kpi">
          <div class="stats-kpi__main">
            <span class="stats-kpi__icon"><img :src="kpi.icon" alt="" /></span>
            <div>
              <div class="stats-kpi__label">{{ kpi.label }}</div>
              <div class="stats-kpi__value">{{ kpi.value }}</div>
              <div v-if="kpi.hint" class="stats-kpi__hint">{{ kpi.hint }}</div>
            </div>
          </div>
        </t-card>
      </div>

      <div class="stats-grid">
        <t-card :bordered="false">
          <template #title>
            <div class="stats-card-title"><img :src="chartIcon" alt="" />{{ t('review.stats.ratingDistribution') }}</div>
          </template>
          <div class="chart-wrap">
            <div ref="distributionEl" class="chart" />
            <div v-if="!hasDistribution" class="chart-empty">
              <img :src="bookIcon" alt="" />
              <span>{{ t('review.v3.statsPage.emptyDistribution') }}</span>
            </div>
          </div>
        </t-card>
        <t-card :bordered="false">
          <template #title>
            <div class="stats-card-title"><img :src="chartIcon" alt="" />{{ t('review.stats.ratingTrend') }}</div>
          </template>
          <div class="chart-wrap">
            <div ref="trendEl" class="chart" />
            <div v-if="!hasTrend" class="chart-empty">
              <img :src="bookIcon" alt="" />
              <span>{{ t('review.v3.statsPage.emptyTrend') }}</span>
            </div>
          </div>
        </t-card>
        <t-card :bordered="false">
          <template #title>
            <div class="stats-card-title"><img :src="calendarIcon" alt="" />{{ t('review.stats.dueForecast') }}</div>
          </template>
          <div class="chart-wrap">
            <div ref="dueEl" class="chart" />
            <div v-if="!hasDueForecast" class="chart-empty">
              <img :src="calendarIcon" alt="" />
              <span>{{ t('review.v3.statsPage.emptyForecast') }}</span>
            </div>
          </div>
        </t-card>
        <t-card :bordered="false">
          <template #title>
            <div class="stats-card-title"><img :src="brainIcon" alt="" />{{ t('review.stats.forgettingCurve') }}</div>
          </template>
          <div class="chart-wrap">
            <div ref="curveEl" class="chart" />
            <div v-if="!hasCurve" class="chart-empty">
              <img :src="brainIcon" alt="" />
              <span>{{ t('review.v3.statsPage.emptyCurve') }}</span>
            </div>
          </div>
        </t-card>
      </div>

      <div class="stats-grid">
        <t-card :bordered="false">
          <template #title>
            <div class="stats-card-title"><img :src="trophyIcon" alt="" />{{ t('review.stats.groupRanking') }}</div>
          </template>
          <div v-if="stats?.group_ranking?.length" class="stats-table">
            <div class="stats-table__head">
              <span>{{ t('review.groupName') }}</span>
              <span>{{ t('review.stats.reviewed') }}</span>
              <span>{{ t('review.stats.retention') }}</span>
            </div>
            <div v-for="group in stats.group_ranking" :key="group.group_id" class="stats-table__row">
              <span class="stats-table__ellipsis">{{ group.group_name || group.group_id }}</span>
              <span class="stats-num">{{ group.reviewed }}</span>
              <span class="stats-num">{{ Math.round(group.retention * 100) }}%</span>
            </div>
            <p v-if="stats.group_ranking.length < 3" class="stats-hint">{{ t('review.v3.statsPage.rankingHint') }}</p>
          </div>
          <p v-else class="stats-empty">{{ t('review.v3.statsPage.emptyRanking') }}</p>
        </t-card>

        <t-card :bordered="false">
          <template #title>
            <div class="stats-card-title"><img :src="goalIcon" alt="" />{{ t('review.v3.statsPage.priorityTitle') }}</div>
          </template>
          <div v-if="priorities.length" class="priority-list">
            <div v-for="topic in priorities" :key="topic.group_id" class="priority-row">
              <div class="priority-row__main">
                <span class="priority-row__name">{{ topic.group_name || topic.group_id }}</span>
                <span class="priority-row__meta">
                  {{ t('review.v3.statsPage.priorityMeta', { reviewed: topic.reviewed, retention: Math.round(topic.retention * 100) }) }}
                </span>
              </div>
              <t-button size="small" variant="outline" @click="emit('review-group', topic.group_id)">
                {{ t('review.v3.statsPage.goReview') }}
              </t-button>
            </div>
          </div>
          <p v-else class="stats-empty">{{ t('review.v3.statsPage.emptyPriorities') }}</p>
        </t-card>
      </div>

      <t-card :bordered="false">
        <template #title>
          <div class="session-head">
            <div class="stats-card-title"><img :src="checklistIcon" alt="" />{{ t('review.stats.sessionHistory') }}</div>
            <t-button size="small" variant="outline" data-testid="review-v3-stats-export" :disabled="!hasSessions" @click="exportSessionsCSV">
              {{ t('review.stats.exportCsv') }}
            </t-button>
          </div>
        </template>
        <div v-if="hasSessions" class="session-table">
          <div class="session-table__head">
            <span>{{ t('review.stats.startedAt') }}</span>
            <span>{{ t('review.mode') }}</span>
            <span>{{ t('review.stats.planned') }}</span>
            <span>{{ t('review.v3.statsPage.fourCounts') }}</span>
            <span>{{ t('review.stats.statusLabel') }}</span>
          </div>
          <div v-for="session in sessions" :key="session.id" class="session-table__row">
            <span class="stats-num">{{ formatTime(session.started_at) }}</span>
            <span>{{ session.mode === 'daily' ? t('review.stats.modeDaily') : t('review.stats.modeManual') }}</span>
            <span class="stats-num">{{ session.completed_count }}/{{ session.planned_count }}</span>
            <span class="stats-num">{{ session.easy_count }}/{{ session.remembered_count }}/{{ session.fuzzy_count }}/{{ session.forgot_count }}</span>
            <span>{{ t(`review.stats.status.${session.status}`) }}</span>
          </div>
        </div>
        <p v-else class="stats-empty">{{ t('review.stats.noSessions') }}</p>
      </t-card>
    </t-loading>
  </div>
</template>

<style lang="less" scoped>
.stats-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.stats-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;

  p {
    margin: 0;
    font-size: 12px;
    color: var(--td-text-color-secondary);
  }
}

.stats-days {
  display: inline-flex;
  overflow: hidden;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-default);

  button {
    min-width: 64px;
    height: 30px;
    padding: 0 12px;
    font-size: 13px;
    color: var(--td-text-color-secondary);
    background: var(--td-bg-color-container);
    border: 0;
    border-left: 1px solid var(--td-component-stroke);
    cursor: pointer;

    &:first-child { border-left: 0; }
    &.is-active {
      color: var(--td-brand-color);
      background: var(--td-brand-color-light);
      font-weight: 500;
    }
  }
}

.stats-kpis {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.stats-kpi {
  :deep(.t-card__body) {
    padding: 16px 18px;
  }

  &__main {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  &__icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 46px;
    height: 46px;
    flex: 0 0 auto;

    img {
      width: 34px;
      height: 34px;
      object-fit: contain;
    }
  }

  &__label {
    font-size: 13px;
    color: var(--td-text-color-secondary);
  }

  &__value {
    margin-top: 2px;
    font-size: 26px;
    line-height: 34px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    color: var(--td-text-color-primary);
  }

  &__hint {
    margin-top: 1px;
    font-size: 12px;
    color: var(--td-text-color-placeholder);
  }
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.stats-card-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--td-text-color-primary);

  img {
    width: 20px;
    height: 20px;
    object-fit: contain;
  }
}

.chart-wrap {
  position: relative;
}

.chart {
  width: 100%;
  height: 240px;
}

.chart-empty {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 16px;
  font-size: 13px;
  color: var(--td-text-color-placeholder);
  text-align: center;
  background: var(--td-bg-color-container);

  img {
    width: 44px;
    height: 44px;
    object-fit: contain;
    opacity: 0.9;
  }
}

.stats-table {
  &__head,
  &__row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 90px 90px;
    gap: 8px;
    align-items: center;
    padding: 8px 4px;
    font-size: 13px;
    border-bottom: 1px solid var(--td-component-stroke);
  }

  &__head {
    font-size: 12px;
    color: var(--td-text-color-secondary);
    border-bottom: 1px solid var(--td-border-level-2-color);
  }

  &__row:last-child { border-bottom: 0; }

  &__ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.stats-num {
  font-variant-numeric: tabular-nums;
  text-align: right;
}

.stats-hint {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.stats-empty {
  margin: 0;
  padding: 36px 12px;
  font-size: 13px;
  color: var(--td-text-color-placeholder);
  text-align: center;
}

.priority-list {
  display: flex;
  flex-direction: column;
}

.priority-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 4px;
  border-bottom: 1px solid var(--td-component-stroke);

  &:last-child { border-bottom: 0; }

  &__main {
    min-width: 0;
  }

  &__name {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 13px;
    font-weight: 500;
    color: var(--td-text-color-primary);
  }

  &__meta {
    display: block;
    margin-top: 2px;
    font-size: 12px;
    color: var(--td-text-color-secondary);
  }
}

.session-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.session-table {
  &__head,
  &__row {
    display: grid;
    grid-template-columns: 130px 90px 90px minmax(150px, 1fr) 80px;
    gap: 8px;
    align-items: center;
    padding: 8px 4px;
    font-size: 13px;
    border-bottom: 1px solid var(--td-component-stroke);
  }

  &__head {
    font-size: 12px;
    color: var(--td-text-color-secondary);
    border-bottom: 1px solid var(--td-border-level-2-color);
  }

  &__row:last-child { border-bottom: 0; }
}

@media (max-width: 1000px) {
  .stats-kpis { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .stats-grid { grid-template-columns: 1fr; }
}

@media (max-width: 760px) {
  .session-table {
    overflow-x: auto;

    &__head, &__row { min-width: 620px; }
  }
}
</style>
