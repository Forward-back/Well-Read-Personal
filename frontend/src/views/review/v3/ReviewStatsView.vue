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
import ReviewIcon from './ReviewIcon.vue'

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

const todayCompleted = computed(() => {
  const today = new Date().toDateString()
  return sessions.value
    .filter(session => new Date(session.started_at).toDateString() === today)
    .reduce((total, session) => total + (session.completed_count || 0), 0)
})

const distributionRows = computed(() => {
  const data = stats.value?.rating_distribution
  const total = data?.total ?? 0
  if (!data || !total) return []
  return [
    { key: 'forgot', label: t('review.v3.rating.forgot'), count: data.forgot, tone: 'red' },
    { key: 'fuzzy', label: t('review.v3.rating.fuzzy'), count: data.fuzzy, tone: 'orange' },
    { key: 'remembered', label: t('review.v3.rating.remembered'), count: data.remembered, tone: 'green' },
    { key: 'easy', label: t('review.v3.rating.easy'), count: data.easy, tone: 'blue' },
  ].map(row => ({ ...row, percent: Math.round((row.count / total) * 100) }))
})

const distributionSummary = computed(() => {
  if (!distributionRows.value.length) return ''
  const good = distributionRows.value
    .filter(row => row.key === 'remembered' || row.key === 'easy')
    .reduce((total, row) => total + row.percent, 0)
  return t('review.v3.statsPage.distributionSummary', { count: good })
})

const dueForecast = computed(() =>
  (stats.value?.due_forecast ?? []).map((point, index) => ({
    ...point,
    weekday: new Date(point.date).toLocaleDateString('zh-CN', { weekday: 'short' }),
    isToday: index === 0,
  })),
)
const dueMax = computed(() => Math.max(1, ...dueForecast.value.map(point => point.count)))
const duePeak = computed(() => {
  const sorted = [...dueForecast.value].sort((a, b) => b.count - a.count)
  return sorted[0] || null
})

const calendarCells = computed(() =>
  (stats.value?.rating_trend ?? []).slice(-84).map(point => {
    const total = point.total || 0
    const level = total === 0 ? 0 : total <= 2 ? 1 : total <= 5 ? 2 : 3
    return { date: point.date, total, level }
  }),
)

const activeCalendarCell = ref<{ date: string; total: number; level: number } | null>(null)

function formatCalendarDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'short' })
}

function cssVar(name: string, fallback: string) {
  const root = document.querySelector('.review-v3') || document.documentElement
  const value = getComputedStyle(root).getPropertyValue(name).trim()
  return value || fallback
}

function palette() {
  const dark = document.documentElement.getAttribute('theme-mode') === 'dark'
  return {
    brand: cssVar('--w-page-accent', '#07c05f'),
    success: cssVar('--w-page-accent', '#07c05f'),
    warning: '#c9a96e',
    error: '#c9a4a0',
    info: '#9db0be',
    text: cssVar('--td-text-color-secondary', '#787774'),
    primary: cssVar('--td-text-color-primary', '#373530'),
    border: cssVar('--td-component-stroke', '#e9e7e2'),
    background: dark ? '#211e1a' : '#ffffff',
  }
}

function baseOption() {
  const c = palette()
  return {
    color: [c.info, c.success, c.warning, c.error],
    textStyle: { color: c.text, fontSize: 14 },
    grid: { left: 8, right: 12, top: 28, bottom: 34, containLabel: true },
    tooltip: {
      trigger: 'axis' as const,
      backgroundColor: c.background,
      borderColor: c.border,
      textStyle: { color: c.primary, fontSize: 14 },
    },
    legend: {
      bottom: 0,
      icon: 'roundRect',
      itemWidth: 10,
      itemHeight: 10,
      textStyle: { color: c.text, fontSize: 14 },
    },
  }
}

function categoryAxis(c: ReturnType<typeof palette>, data: string[]) {
  return {
    type: 'category' as const,
    data,
    axisLabel: { color: c.text, fontSize: 14, hideOverlap: true },
    axisLine: { lineStyle: { color: c.border } },
    axisTick: { show: false },
  }
}

function valueAxis(c: ReturnType<typeof palette>, extra: Record<string, unknown> = {}) {
  return {
    type: 'value' as const,
    axisLabel: { color: c.text, fontSize: 14 },
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
        textStyle: { color: c.primary, fontSize: 14 },
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
          axisLabel: { color: c.text, fontSize: 14, formatter: (value: number) => `${Math.round(value * 100)}%` },
          splitLine: { show: false },
        }),
      ],
      series: [
        { name: t('review.v3.rating.easy'), type: 'bar', stack: 'rating', barMaxWidth: 18, data: data.rating_trend.map(point => point.easy), itemStyle: { color: c.info } },
        { name: t('review.v3.rating.remembered'), type: 'bar', stack: 'rating', barMaxWidth: 18, data: data.rating_trend.map(point => point.remembered), itemStyle: { color: c.success } },
        { name: t('review.v3.rating.fuzzy'), type: 'bar', stack: 'rating', barMaxWidth: 18, data: data.rating_trend.map(point => point.fuzzy), itemStyle: { color: c.warning } },
        { name: t('review.v3.rating.forgot'), type: 'bar', stack: 'rating', barMaxWidth: 18, data: data.rating_trend.map(point => point.forgot), itemStyle: { color: c.error } },
        { name: t('review.stats.retention'), type: 'line', yAxisIndex: 1, smooth: true, symbolSize: 5, data: data.rating_trend.map(point => (point.total ? point.retention : null)), itemStyle: { color: c.info }, lineStyle: { width: 2 } },
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
        label: { show: true, position: 'top', color: c.text, fontSize: 14, formatter: (params: { value: number }) => (params.value > 0 ? String(params.value) : '') },
        itemStyle: { color: c.error, borderRadius: [3, 3, 0, 0] },
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
        axisLabel: { color: c.text, fontSize: 14, formatter: (value: number) => `${Math.round(value * 100)}%` },
      }),
      series: [
        { name: t('review.stats.actual'), type: 'line', smooth: true, symbolSize: 4, data: data.forgetting_curve.map(point => point.actual), itemStyle: { color: c.info }, lineStyle: { width: 2 } },
        { name: t('review.stats.theoretical'), type: 'line', smooth: true, symbolSize: 4, lineStyle: { type: 'dashed', width: 2 }, data: data.forgetting_curve.map(point => point.theoretical), itemStyle: { color: c.text } },
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
    <div class="stats-range">
      <span class="rv-prop__key">{{ t('review.v3.statsPage.timeRange') }}</span>
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
        <div class="stat-kpi">
          <div class="stat-kpi__label">{{ t('review.v3.statsPage.todayReviewed') }}</div>
          <div class="stat-kpi__value">{{ todayCompleted }}</div>
          <div class="stat-kpi__hint">{{ t('review.v3.statsPage.todayReviewedHint') }}</div>
        </div>
        <div class="stat-kpi">
          <div class="stat-kpi__label">{{ t('review.v3.statsPage.rangeReviewed', { days }) }}</div>
          <div class="stat-kpi__value">{{ stats?.reviewed ?? 0 }}</div>
          <div class="stat-kpi__hint">{{ t('review.v3.statsPage.rangeReviewedHint') }}</div>
        </div>
        <div class="stat-kpi">
          <div class="stat-kpi__label">{{ t('review.stats.retention') }}</div>
          <div class="stat-kpi__value">{{ stats && stats.rating_distribution.total > 0 ? `${Math.round((stats.retention ?? 0) * 100)}%` : '—' }}</div>
          <div class="stat-kpi__hint">{{ t('review.v3.statsPage.retentionHint') }}</div>
        </div>
        <div class="stat-kpi">
          <div class="stat-kpi__label">{{ t('review.stats.streak') }}</div>
          <div class="stat-kpi__value">{{ stats?.streak_days ?? 0 }}</div>
          <div class="stat-kpi__hint">{{ t('review.v3.statsPage.sessionTotal', { count: sessions.length }) }}</div>
        </div>
      </div>

      <div class="stats-row">
        <div class="stat-card">
          <div class="stat-card__title">
            {{ t('review.stats.ratingDistribution') }}
            <span class="rv-note">· {{ t('review.v3.statsPage.rangeLabel', { days }) }} · {{ distributionTotal }} {{ t('review.v3.statsPage.timesUnit') }}</span>
          </div>
          <div v-if="hasDistribution" class="dist-list">
            <div v-for="row in distributionRows" :key="row.key" class="dist-row">
              <span class="dist-row__label">
                <i :class="`is-${row.tone}`" />{{ row.label }}
              </span>
              <span class="dist-row__track"><i :class="`is-${row.tone}`" :style="{ width: `${row.percent}%` }" /></span>
              <span class="dist-row__value">{{ row.percent }}%</span>
            </div>
            <p class="stat-note">{{ distributionSummary }}</p>
          </div>
          <p v-else class="stat-empty">{{ t('review.v3.statsPage.emptyDistribution') }}</p>
        </div>

        <div class="stat-card">
          <div class="stat-card__title">
            {{ t('review.stats.dueForecast') }}
            <span class="rv-note">· {{ t('review.v3.statsPage.cardsUnit') }}</span>
          </div>
          <div v-if="hasDueForecast" class="due-chart">
            <div v-for="point in dueForecast" :key="point.date" class="due-bar" :class="{ 'is-hot': point.isToday }">
              <b>{{ point.count }}</b>
              <i :style="{ height: `${Math.max(6, Math.round((point.count / dueMax) * 100))}%` }" />
              <span>{{ point.weekday }}</span>
            </div>
          </div>
          <p v-else class="stat-empty">{{ t('review.v3.statsPage.emptyForecast') }}</p>
          <p class="stat-note">{{ t('review.v3.statsPage.forecastHint') }}</p>
        </div>
      </div>

      <div class="stats-row stats-row--curve">
        <div class="stat-card stat-card--wide">
          <div class="stat-card__title">
            {{ t('review.stats.forgettingCurve') }}
            <span class="rv-note">· {{ t('review.v3.statsPage.curveSub') }}</span>
          </div>
          <div class="chart-wrap">
            <div ref="curveEl" class="chart" />
            <div v-if="!hasCurve" class="chart-empty">
              <img :src="bookIcon" alt="" />
              <span>{{ t('review.v3.statsPage.emptyCurve') }}</span>
            </div>
          </div>
        </div>
        <div class="stat-card stat-card--calendar">
          <div class="stat-card__title">
            {{ t('review.v3.statsPage.reviewCalendar') }}
            <span class="rv-note">· {{ t('review.v3.statsPage.rangeLabel', { days }) }}</span>
          </div>
          <div class="cal-grid">
            <button
              v-for="cell in calendarCells"
              :key="cell.date"
              type="button"
              class="cal-cell"
              :class="`cal-cell--${cell.level}`"
              :aria-label="`${formatCalendarDate(cell.date)} · ${cell.total}`"
              @mouseenter="activeCalendarCell = cell"
              @mouseleave="activeCalendarCell = null"
              @focus="activeCalendarCell = cell"
              @blur="activeCalendarCell = null"
              @click="activeCalendarCell = cell"
            />
          </div>
          <div class="cal-readout" :class="{ 'is-empty': !activeCalendarCell }">
            <template v-if="activeCalendarCell">
              <b>{{ formatCalendarDate(activeCalendarCell.date) }}</b>
              <span>{{ t('review.v3.statsPage.calendarReviews', { count: activeCalendarCell.total }) }}</span>
            </template>
            <template v-else>{{ t('review.v3.statsPage.calendarHint') }}</template>
          </div>
          <div class="cal-legend">
            <span>{{ t('review.v3.statsPage.less') }}</span>
            <i class="cal-cell--0" /><i class="cal-cell--1" /><i class="cal-cell--2" /><i class="cal-cell--3" />
            <span>{{ t('review.v3.statsPage.more') }}</span>
          </div>
          <div class="rv-callout rv-callout--blue stat-insight">
            <ReviewIcon name="target" :size="20" :tilt="-1" />
            <div v-if="duePeak">
              {{ t('review.v3.statsPage.forecastInsight', { day: duePeak.weekday, count: duePeak.count }) }}
            </div>
            <div v-else>{{ t('review.v3.statsPage.forecastInsightEmpty') }}</div>
          </div>
        </div>
      </div>

      <div class="stats-grid">
        <t-card :bordered="false">
          <template #title>
            <div class="stats-card-title">{{ t('review.stats.groupRanking') }}</div>
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
            <div class="stats-card-title">{{ t('review.v3.statsPage.priorityTitle') }}</div>
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
            <div class="stats-card-title">{{ t('review.stats.sessionHistory') }}</div>
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
    font-size: 14px;
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
    font-size: 14px;
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
  gap: 16px;
  margin-bottom: 20px;
}

.stats-kpi {
  :deep(.t-card__body) {
    padding: 18px 20px;
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
    font-size: 14px;
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
    font-size: 14px;
    color: var(--td-text-color-placeholder);
  }
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 20px;
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
  font-size: 14px;
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
    font-size: 14px;
    border-bottom: 1px solid var(--td-component-stroke);
  }

  &__head {
    font-size: 14px;
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
  font-size: 14px;
  color: var(--td-text-color-placeholder);
}

.stats-empty {
  margin: 0;
  padding: 36px 12px;
  font-size: 14px;
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
    font-size: 14px;
    font-weight: 500;
    color: var(--td-text-color-primary);
  }

  &__meta {
    display: block;
    margin-top: 2px;
    font-size: 14px;
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
    font-size: 14px;
    border-bottom: 1px solid var(--td-component-stroke);
  }

  &__head {
    font-size: 14px;
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

<style lang="less" scoped>
/* Paper-journal presentation for the stats tab. */
.stats-page {
  gap: 18px;
  color: var(--rvn-ink);
}

.stats-head {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 0;

  p {
    margin: 0;
    font-size: 14px;
    color: var(--rvn-ink-soft);
  }
}

.stats-days {
  display: flex;
  gap: 6px;

  button {
    padding: 4px 10px;
    font-family: inherit;
    font-size: 14px;
    color: var(--rvn-ink-soft);
    cursor: pointer;
    background: transparent;
    border: 1px solid var(--rvn-line);
    border-radius: 6px;

    &:hover {
      background: var(--rvn-surface);
    }

    &.is-active {
      font-weight: 600;
      color: var(--rvn-brand-deep);
      background: var(--rvn-brand-bg);
      border-color: var(--rvn-brand-line);
    }
  }
}

.stats-kpis {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.stats-kpi {
  padding: 0;
  background: transparent;
  border: 0;

  :deep(.t-card__body) {
    padding: 16px 18px;
  }

  :deep(.t-card) {
    background: var(--rvn-paper);
    border: 1px solid var(--rvn-line);
    border-radius: 7px 6px 8px 7px / 6px 8px 7px 6px;
  }
}

.stats-kpi__main {
  display: block;
}

.stats-kpi__label {
  font-size: 14px;
  color: var(--rvn-muted);
}

.stats-kpi__value {
  margin-top: 6px;
  font-size: 26px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--rvn-ink);
}

.stats-kpi__hint {
  margin-top: 4px;
  font-size: 14px;
  color: var(--rvn-green-deep);
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;

  :deep(.t-card) {
    background: var(--rvn-paper);
    border: 1px solid var(--rvn-line);
    border-radius: 7px 6px 8px 7px / 6px 8px 7px 6px;
    box-shadow: none;
  }

  :deep(.t-card__header) {
    min-height: 42px;
    padding: 0 18px;
    border-bottom: 0;
  }

  :deep(.t-card__body) {
    padding: 0 18px 16px;
  }
}

.stats-card-title {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  font-size: 14px;
  font-weight: 600;
  color: var(--rvn-ink);
}

.chart-wrap {
  position: relative;
  min-height: 210px;
}

.chart-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 190px;
  font-size: 14px;
  color: var(--rvn-muted);

  img {
    width: 26px;
    height: 26px;
    object-fit: contain;
    opacity: 0.8;
  }
}

.stats-table,
.session-table {
  min-width: 0;
}

.stats-table__head,
.session-table__head {
  font-size: 14px;
  color: var(--rvn-muted);
  border-bottom: 1px solid var(--rvn-line);
}

.stats-table__row,
.session-table__row {
  border-bottom: 1px solid var(--rvn-line);

  &:hover {
    background: var(--rvn-surface);
  }
}

.session-table__head,
.session-table__row {
  grid-template-columns: 140px 110px 110px minmax(220px, 1fr) 100px;
  column-gap: 16px;
  padding: 10px 8px;
}

.session-table .stats-num {
  text-align: left;
}

.priority-row {
  border-bottom: 1px solid var(--rvn-line);
}

.priority-row__name {
  font-weight: 500;
  color: var(--rvn-ink);
}

.stats-hint,
.stats-empty {
  color: var(--rvn-muted);
}

.session-head :deep(.t-button) {
  border-radius: 6px;
}

.stats-range {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  align-items: center;
  margin-bottom: 22px;

  .rv-prop__key {
    width: auto;
  }
}

.stat-kpi {
  padding: 18px 20px;
  background: var(--rvn-paper);
  border: 1px solid var(--rvn-line);
  border-radius: 8px;
}

.stat-kpi__label {
  font-size: 14px;
  color: var(--rvn-muted);
}

.stat-kpi__value {
  margin-top: 8px;
  font-size: 30px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--rvn-ink);
}

.stat-kpi__hint {
  margin-top: 6px;
  font-size: 14px;
  color: var(--rvn-green-deep);
}

.stats-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
  margin-top: 20px;
}

.stats-page > :deep(.t-card) {
  margin-top: 20px;
}

.stats-grid {
  margin-top: 20px;
}

.stats-row--curve {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-items: stretch;
}

.stat-card--wide {
  display: flex;
  flex-direction: column;
}

.stat-card--wide .chart-wrap {
  flex: 1;
  min-height: 300px;
}

.stat-card--wide .chart {
  height: 100%;
  min-height: 300px;
}

.stat-card--calendar {
  display: flex;
  flex-direction: column;
}

.stat-card--calendar .cal-grid {
  align-content: start;
}

.stat-card--calendar .stat-insight {
  margin-top: 16px;
}

.stats-side {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-width: 0;
}

.stat-card,
.stat-insight {
  min-width: 0;
}

.stat-insight div {
  min-width: 0;
  overflow-wrap: anywhere;
}

.stat-card {
  padding: 18px 20px;
  background: var(--rvn-paper);
  border: 1px solid var(--rvn-line);
  border-radius: 8px;
}

.stat-card__title {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: baseline;
  margin-bottom: 14px;
  font-size: 14px;
  font-weight: 600;
  color: var(--rvn-ink);
}

.dist-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.dist-row {
  display: flex;
  gap: 10px;
  align-items: center;
  font-size: 14px;
}

.dist-row__label {
  display: flex;
  gap: 6px;
  align-items: center;
  width: 62px;
  color: var(--rvn-ink-soft);

  i {
    width: 9px;
    height: 9px;
    border-radius: 50%;

    &.is-red { background: var(--rvn-red); }
    &.is-orange { background: var(--rvn-orange); }
    &.is-green { background: var(--rvn-green); }
    &.is-blue { background: var(--rvn-accent); }
  }
}

.dist-row__track {
  flex: 1;
  height: 14px;
  overflow: hidden;
  background: var(--rvn-gray-soft);

  i {
    display: block;
    height: 100%;

    &.is-red { background: var(--rvn-red); }
    &.is-orange { background: var(--rvn-orange); }
    &.is-green { background: var(--rvn-green); }
    &.is-blue { background: var(--rvn-accent); }
  }
}

.dist-row__value {
  width: 34px;
  color: var(--rvn-muted);
  text-align: right;
}

.stat-note {
  margin: 12px 0 0;
  font-size: 14px;
  line-height: 1.6;
  color: var(--rvn-ink-soft);
}

.due-chart {
  display: flex;
  gap: 12px;
  align-items: flex-end;
  height: 190px;
  padding-top: 8px;
}

.due-bar {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 6px;
  align-items: center;
  justify-content: flex-end;
  height: 100%;

  i {
    display: block;
    width: 100%;
    max-width: 34px;
    background: var(--rvn-accent-soft);
    border: 1px solid var(--rvn-accent);
  }

  b {
    font-size: 14px;
    font-weight: 600;
    color: var(--rvn-ink);
  }

  span {
    font-size: 14px;
    color: var(--rvn-muted);
  }

  &.is-hot i {
    background: var(--rvn-red);
    border-color: var(--rvn-red-deep);
  }
}

.cal-grid {
  display: grid;
  grid-template-columns: repeat(12, 1fr);
  gap: 4px;

  i {
    aspect-ratio: 1;
    border-radius: 3px;
  }
}

.cal-cell {
  display: block;
  width: 100%;
  aspect-ratio: 1;
  padding: 0;
  cursor: pointer;
  border: 1px solid transparent;
  border-radius: 3px;
  transition: transform 0.12s ease, outline-color 0.12s ease;
}

.cal-cell:hover,
.cal-cell:focus-visible {
  outline: 2px solid var(--rvn-brand);
  outline-offset: 1px;
  transform: scale(1.08);
}

.cal-readout {
  display: flex;
  gap: 8px;
  align-items: baseline;
  min-height: 20px;
  margin-top: 10px;
  font-size: 14px;
  color: var(--rvn-ink-soft);

  b {
    font-weight: 600;
    color: var(--rvn-ink);
  }

  &.is-empty {
    color: var(--rvn-muted);
  }
}

.cal-cell--0 { background: var(--rvn-gray-soft); }
.cal-cell--1 { background: var(--rvn-accent-soft); }
.cal-cell--2 { background: #aec3cf; }

:global(html[theme-mode="dark"] .cal-cell--2) {
  background: color-mix(in srgb, var(--rvn-accent) 52%, var(--rvn-paper));
}
.cal-cell--3 { background: var(--rvn-accent); }

.cal-legend {
  display: flex;
  gap: 5px;
  align-items: center;
  margin-top: 10px;
  font-size: 14px;
  color: var(--rvn-muted);

  i {
    width: 10px;
    height: 10px;
    border-radius: 2px;
  }
}

.stat-insight {
  margin: 0;
  align-items: flex-start;
}

.stat-empty {
  margin: 0;
  padding: 28px 0;
  font-size: 14px;
  color: var(--rvn-muted);
  text-align: center;
}

@media (max-width: 960px) {
  .stats-kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .stats-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .stats-row,
  .stats-row--curve {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
