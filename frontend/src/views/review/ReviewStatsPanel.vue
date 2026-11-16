<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts/core'
import { BarChart, LineChart, PieChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { AlertTriangle } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { ReviewSession, ReviewStats } from '@/api/review'

echarts.use([BarChart, LineChart, PieChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

const props = defineProps<{
  stats: ReviewStats | null
  sessions: ReviewSession[]
  days: number
  loading?: boolean
}>()
const emit = defineEmits<{
  (e: 'update:days', days: number): void
  (e: 'export-csv'): void
}>()
const { t } = useI18n()

const distributionEl = ref<HTMLDivElement | null>(null)
const trendEl = ref<HTMLDivElement | null>(null)
const dueEl = ref<HTMLDivElement | null>(null)
const curveEl = ref<HTMLDivElement | null>(null)
const charts: echarts.ECharts[] = []
let resizeObserver: ResizeObserver | null = null
let themeObserver: MutationObserver | null = null

const hasDistribution = computed(() => (props.stats?.rating_distribution.total ?? 0) > 0)
const hasTrend = computed(() => props.stats?.rating_trend?.some((point) => point.total > 0) ?? false)
const hasDueForecast = computed(() => props.stats?.due_forecast?.some((point) => point.count > 0) ?? false)
const hasCurve = computed(() => (props.stats?.forgetting_curve?.length ?? 0) > 0)
const difficultCards = computed(() => (props.stats?.difficult_cards ?? []).filter((card) => card.forgot > 0))

const kpis = computed(() => [
  { label: t('review.totalCards'), value: String(props.stats?.total_cards ?? 0), hint: '' },
  { label: t('review.stats.reviewed'), value: String(props.stats?.reviewed ?? 0), hint: `${props.days} ${t('review.stats.daysUnit')}` },
  { label: t('review.stats.retention'), value: `${Math.round((props.stats?.retention ?? 0) * 100)}%`, hint: `${props.days} ${t('review.stats.daysUnit')}` },
  { label: t('review.stats.streak'), value: String(props.stats?.streak_days ?? 0), hint: `${t('review.stats.daysUnit')}` },
])

function cssVar(name: string, fallback: string) {
  const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return value || fallback
}

function palette() {
  return {
    brand: cssVar('--td-brand-color', '#00a870'),
    success: cssVar('--td-success-color', '#2ba471'),
    warning: cssVar('--td-warning-color', '#e37318'),
    error: cssVar('--td-error-color', '#d54941'),
    text: cssVar('--td-text-color-secondary', '#666666'),
    primary: cssVar('--td-text-color-primary', '#1a1a1a'),
    border: cssVar('--td-component-stroke', '#e7e7e7'),
    background: cssVar('--td-bg-color-container', '#ffffff'),
  }
}

function baseOption() {
  const c = palette()
  return {
    color: [c.brand, c.success, c.warning, c.error],
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
  charts.splice(0).forEach((chart) => chart.dispose())
}

function renderCharts() {
  const stats = props.stats
  resetCharts()
  if (!stats) return
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
          { name: t('review.remembered'), value: stats.rating_distribution.remembered, itemStyle: { color: c.success } },
          { name: t('review.fuzzy'), value: stats.rating_distribution.fuzzy, itemStyle: { color: c.warning } },
          { name: t('review.forgot'), value: stats.rating_distribution.forgot, itemStyle: { color: c.error } },
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
      xAxis: categoryAxis(c, stats.rating_trend.map((point) => point.date.slice(5))),
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
        { name: t('review.remembered'), type: 'bar', stack: 'rating', barMaxWidth: 18, data: stats.rating_trend.map((point) => point.remembered), itemStyle: { color: c.success } },
        { name: t('review.fuzzy'), type: 'bar', stack: 'rating', barMaxWidth: 18, data: stats.rating_trend.map((point) => point.fuzzy), itemStyle: { color: c.warning } },
        { name: t('review.forgot'), type: 'bar', stack: 'rating', barMaxWidth: 18, data: stats.rating_trend.map((point) => point.forgot), itemStyle: { color: c.error } },
        { name: t('review.stats.retention'), type: 'line', yAxisIndex: 1, smooth: true, symbolSize: 5, data: stats.rating_trend.map((point) => point.total ? point.retention : null), itemStyle: { color: c.primary }, lineStyle: { width: 2 } },
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
      xAxis: categoryAxis(c, stats.due_forecast.map((point) => point.date.slice(5))),
      yAxis: valueAxis(c, { minInterval: 1 }),
      series: [{
        type: 'bar',
        data: stats.due_forecast.map((point) => point.count),
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
      xAxis: categoryAxis(c, stats.forgetting_curve.map((point) => String(point.day))),
      yAxis: valueAxis(c, {
        min: 0,
        max: 1,
        axisLabel: { color: c.text, fontSize: 11, formatter: (value: number) => `${Math.round(value * 100)}%` },
      }),
      series: [
        { name: t('review.stats.actual'), type: 'line', smooth: true, symbolSize: 4, data: stats.forgetting_curve.map((point) => point.actual), itemStyle: { color: c.brand }, lineStyle: { width: 2 } },
        { name: t('review.stats.theoretical'), type: 'line', smooth: true, symbolSize: 4, lineStyle: { type: 'dashed', width: 2 }, data: stats.forgetting_curve.map((point) => point.theoretical), itemStyle: { color: c.warning } },
      ],
    }, true)
  }
}

function scheduleRender() {
  void nextTick(() => {
    renderCharts()
    charts.forEach((chart) => chart.resize())
  })
}

function formatTime(value?: string | null) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function statusVariant(status: ReviewSession['status']) {
  if (status === 'completed') return 'secondary'
  if (status === 'active') return 'default'
  if (status === 'abandoned') return 'outline'
  return 'destructive'
}

onMounted(() => {
  scheduleRender()
  const elements = [distributionEl.value, trendEl.value, dueEl.value, curveEl.value].filter(Boolean) as HTMLDivElement[]
  if (elements.length && 'ResizeObserver' in window) {
    resizeObserver = new ResizeObserver(() => charts.forEach((chart) => chart.resize()))
    elements.forEach((element) => resizeObserver?.observe(element))
  }
  themeObserver = new MutationObserver(() => scheduleRender())
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class', 'theme-mode'] })
})

watch(() => props.stats, scheduleRender, { deep: true })
watch(() => props.days, scheduleRender)

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  themeObserver?.disconnect()
  resetCharts()
})
</script>

<template>
  <div class="space-y-4" data-testid="review-stats">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p class="text-muted-foreground text-xs">{{ t('review.stats.subtitle') }}</p>
      <div class="border-border bg-card inline-flex overflow-hidden rounded-md border">
        <button
          v-for="option in [7, 30, 90]"
          :key="option"
          type="button"
          class="border-border h-8 min-w-[64px] border-l px-3 text-[13px] transition-colors first:border-l-0 focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none"
          :class="days === option ? 'bg-primary/10 text-primary font-medium' : 'text-muted-foreground hover:bg-muted hover:text-foreground'"
          @click="emit('update:days', option)"
        >
          {{ t(`review.stats.days${option}`) }}
        </button>
      </div>
    </div>

    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <Card v-for="kpi in kpis" :key="kpi.label" class="py-0">
        <CardContent class="flex h-full flex-col p-4">
          <div class="text-muted-foreground text-[13px] font-medium">{{ kpi.label }}</div>
          <div class="mt-1 text-2xl leading-8 font-semibold tabular-nums">{{ kpi.value }}</div>
          <div v-if="kpi.hint" class="text-muted-foreground mt-1 text-xs tabular-nums">{{ kpi.hint }}</div>
        </CardContent>
      </Card>
    </div>

    <div class="grid gap-3 xl:grid-cols-2">
      <Card class="py-0">
        <CardHeader class="px-4 pt-4 pb-0"><CardTitle class="text-sm font-medium">{{ t('review.stats.ratingDistribution') }}</CardTitle></CardHeader>
        <CardContent class="p-4 pt-3">
          <div class="relative">
            <div ref="distributionEl" class="h-60 w-full" />
            <div v-if="!hasDistribution" class="bg-card text-muted-foreground absolute inset-0 flex items-center justify-center text-sm">
              {{ t('review.stats.dataInsufficient') }}
            </div>
          </div>
        </CardContent>
      </Card>
      <Card class="py-0">
        <CardHeader class="px-4 pt-4 pb-0"><CardTitle class="text-sm font-medium">{{ t('review.stats.ratingTrend') }}</CardTitle></CardHeader>
        <CardContent class="p-4 pt-3">
          <div class="relative">
            <div ref="trendEl" class="h-60 w-full" />
            <div v-if="!hasTrend" class="bg-card text-muted-foreground absolute inset-0 flex items-center justify-center text-sm">
              {{ t('review.stats.dataInsufficient') }}
            </div>
          </div>
        </CardContent>
      </Card>
      <Card class="py-0">
        <CardHeader class="px-4 pt-4 pb-0"><CardTitle class="text-sm font-medium">{{ t('review.stats.dueForecast') }}</CardTitle></CardHeader>
        <CardContent class="p-4 pt-3">
          <div class="relative">
            <div ref="dueEl" class="h-60 w-full" />
            <div v-if="!hasDueForecast" class="bg-card text-muted-foreground absolute inset-0 flex items-center justify-center text-sm">
              {{ t('review.stats.dataInsufficient') }}
            </div>
          </div>
        </CardContent>
      </Card>
      <Card class="py-0">
        <CardHeader class="px-4 pt-4 pb-0"><CardTitle class="text-sm font-medium">{{ t('review.stats.forgettingCurve') }}</CardTitle></CardHeader>
        <CardContent class="p-4 pt-3">
          <div class="relative">
            <div ref="curveEl" class="h-60 w-full" />
            <div v-if="!hasCurve" class="bg-card text-muted-foreground absolute inset-0 flex items-center justify-center text-sm">
              {{ t('review.stats.dataInsufficient') }}
            </div>
          </div>
        </CardContent>
      </Card>
    </div>

    <div class="grid gap-3 xl:grid-cols-2">
      <Card class="py-0">
        <CardHeader class="px-4 pt-4 pb-0"><CardTitle class="text-sm font-medium">{{ t('review.stats.groupRanking') }}</CardTitle></CardHeader>
        <CardContent class="p-4 pt-3">
          <Table v-if="stats?.group_ranking?.length">
            <TableHeader>
              <TableRow>
                <TableHead>{{ t('review.groupName') }}</TableHead>
                <TableHead class="text-right">{{ t('review.stats.reviewed') }}</TableHead>
                <TableHead class="text-right">{{ t('review.stats.retention') }}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="group in stats.group_ranking" :key="group.group_id">
                <TableCell>{{ group.group_name || group.group_id }}</TableCell>
                <TableCell class="text-right tabular-nums">{{ group.reviewed }}</TableCell>
                <TableCell class="text-right tabular-nums">{{ Math.round(group.retention * 100) }}%</TableCell>
              </TableRow>
            </TableBody>
          </Table>
          <p v-else class="text-muted-foreground py-8 text-center text-sm">{{ t('review.stats.noGroupRanking') }}</p>
        </CardContent>
      </Card>
      <Card class="py-0">
        <CardHeader class="px-4 pt-4 pb-0"><CardTitle class="text-sm font-medium">{{ t('review.stats.difficultCards') }}</CardTitle></CardHeader>
        <CardContent class="p-4 pt-3">
          <div v-if="difficultCards.length" class="border-border divide-border divide-y overflow-hidden rounded-md border">
            <div v-for="card in difficultCards" :key="card.card_id" class="flex items-start gap-2 px-3 py-2">
              <AlertTriangle class="text-destructive mt-0.5 size-4 shrink-0" />
              <div class="min-w-0 flex-1">
                <div class="truncate text-sm">{{ card.question }}</div>
                <div class="text-muted-foreground mt-0.5 flex flex-wrap gap-x-3 gap-y-1 text-xs">
                  <span>{{ card.group_name }}</span>
                  <span>{{ t('review.forgot') }} {{ card.forgot }}/{{ card.total }}</span>
                  <span class="tabular-nums">{{ Math.round(card.forget_rate * 100) }}%</span>
                </div>
              </div>
            </div>
          </div>
          <p v-else class="text-muted-foreground py-8 text-center text-sm">{{ t('review.stats.noDifficultCards') }}</p>
        </CardContent>
      </Card>
    </div>

    <Card class="py-0">
      <CardHeader class="flex flex-row items-center justify-between px-4 pt-4 pb-0">
        <CardTitle class="text-sm font-medium">{{ t('review.stats.sessionHistory') }}</CardTitle>
        <Button variant="outline" size="sm" :disabled="!sessions.length" @click="emit('export-csv')">
          {{ t('review.stats.exportCsv') }}
        </Button>
      </CardHeader>
      <CardContent class="p-4 pt-3">
        <Table v-if="sessions.length">
          <TableHeader>
            <TableRow>
              <TableHead>{{ t('review.stats.startedAt') }}</TableHead>
              <TableHead>{{ t('review.mode') }}</TableHead>
              <TableHead>{{ t('review.stats.planned') }}</TableHead>
              <TableHead>{{ t('review.remembered') }}/{{ t('review.fuzzy') }}/{{ t('review.forgot') }}</TableHead>
              <TableHead>{{ t('review.stats.statusLabel') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="session in sessions" :key="session.id">
              <TableCell class="whitespace-nowrap tabular-nums">{{ formatTime(session.started_at) }}</TableCell>
              <TableCell>{{ session.mode === 'daily' ? t('review.stats.modeDaily') : t('review.stats.modeManual') }}</TableCell>
              <TableCell class="tabular-nums">{{ session.completed_count }}/{{ session.planned_count }}</TableCell>
              <TableCell class="tabular-nums">{{ session.remembered_count }}/{{ session.fuzzy_count }}/{{ session.forgot_count }}</TableCell>
              <TableCell>
                <Badge :variant="statusVariant(session.status)">
                  {{ t(`review.stats.status.${session.status}`) }}
                </Badge>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
        <p v-else class="text-muted-foreground py-8 text-center text-sm">{{ t('review.stats.noSessions') }}</p>
      </CardContent>
    </Card>
  </div>
</template>
