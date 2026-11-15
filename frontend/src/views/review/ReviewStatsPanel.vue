<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts/core'
import { BarChart, LineChart, PieChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { AlertTriangle, CalendarDays, Flame, Layers, Target } from '@lucide/vue'
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
    border: cssVar('--td-component-stroke', '#e7e7e7'),
    background: cssVar('--td-bg-color-container', '#ffffff'),
  }
}

function baseOption() {
  const c = palette()
  return {
    color: [c.brand, c.success, c.warning, c.error],
    textStyle: { color: c.text, fontSize: 12 },
    grid: { left: 42, right: 52, top: 28, bottom: 28 },
    tooltip: { trigger: 'axis' as const },
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

  if (distributionEl.value && stats.rating_distribution.total > 0) {
    const chart = mount(distributionEl.value)
    chart?.setOption({
      ...baseOption(),
      tooltip: { trigger: 'item' },
      legend: { bottom: 0, textStyle: { color: c.text } },
      series: [{
        type: 'pie',
        radius: ['50%', '72%'],
        center: ['50%', '44%'],
        label: { show: false },
        data: [
          { name: t('review.remembered'), value: stats.rating_distribution.remembered, itemStyle: { color: c.success } },
          { name: t('review.fuzzy'), value: stats.rating_distribution.fuzzy, itemStyle: { color: c.warning } },
          { name: t('review.forgot'), value: stats.rating_distribution.forgot, itemStyle: { color: c.error } },
        ],
      }],
    }, true)
  }

  if (trendEl.value && stats.rating_trend.some((point) => point.total > 0)) {
    const chart = mount(trendEl.value)
    chart?.setOption({
      ...baseOption(),
      legend: { top: 0, textStyle: { color: c.text } },
      xAxis: {
        type: 'category',
        data: stats.rating_trend.map((point) => point.date.slice(5)),
        axisLine: { lineStyle: { color: c.border } },
        axisLabel: { color: c.text },
      },
      yAxis: [
        {
          type: 'value',
          axisLabel: { color: c.text },
          splitLine: { lineStyle: { color: c.border } },
        },
        {
          type: 'value',
          min: 0,
          max: 1,
          axisLabel: { color: c.text, formatter: (value: number) => `${Math.round(value * 100)}%` },
          splitLine: { show: false },
        },
      ],
      series: [
        { name: t('review.remembered'), type: 'bar', stack: 'rating', data: stats.rating_trend.map((point) => point.remembered), itemStyle: { color: c.success } },
        { name: t('review.fuzzy'), type: 'bar', stack: 'rating', data: stats.rating_trend.map((point) => point.fuzzy), itemStyle: { color: c.warning } },
        { name: t('review.forgot'), type: 'bar', stack: 'rating', data: stats.rating_trend.map((point) => point.forgot), itemStyle: { color: c.error } },
        { name: t('review.stats.retention'), type: 'line', yAxisIndex: 1, smooth: true, symbolSize: 5, data: stats.rating_trend.map((point) => point.total ? point.retention : null), itemStyle: { color: c.brand } },
      ],
    }, true)
  }

  if (dueEl.value) {
    const chart = mount(dueEl.value)
    chart?.setOption({
      ...baseOption(),
      xAxis: {
        type: 'category',
        data: stats.due_forecast.map((point) => point.date.slice(5)),
        axisLine: { lineStyle: { color: c.border } },
        axisLabel: { color: c.text },
      },
      yAxis: {
        type: 'value',
        minInterval: 1,
        axisLabel: { color: c.text },
        splitLine: { lineStyle: { color: c.border } },
      },
      series: [{
        type: 'bar',
        data: stats.due_forecast.map((point) => point.count),
        barMaxWidth: 26,
        itemStyle: { color: c.brand, borderRadius: [4, 4, 0, 0] },
      }],
    }, true)
  }

  if (curveEl.value && stats.forgetting_curve.length > 0) {
    const chart = mount(curveEl.value)
    chart?.setOption({
      ...baseOption(),
      legend: { top: 0, textStyle: { color: c.text } },
      xAxis: {
        type: 'category',
        data: stats.forgetting_curve.map((point) => point.day),
        name: t('review.stats.daysUnit'),
        axisLine: { lineStyle: { color: c.border } },
        axisLabel: { color: c.text },
      },
      yAxis: {
        type: 'value',
        min: 0,
        max: 1,
        axisLabel: { color: c.text, formatter: (value: number) => `${Math.round(value * 100)}%` },
        splitLine: { lineStyle: { color: c.border } },
      },
      series: [
        { name: t('review.stats.actual'), type: 'line', smooth: true, data: stats.forgetting_curve.map((point) => point.actual), itemStyle: { color: c.brand } },
        { name: t('review.stats.theoretical'), type: 'line', smooth: true, lineStyle: { type: 'dashed' }, data: stats.forgetting_curve.map((point) => point.theoretical), itemStyle: { color: c.warning } },
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
})

watch(() => props.stats, scheduleRender, { deep: true })

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resetCharts()
})
</script>

<template>
  <div class="space-y-4" data-testid="review-stats">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-lg font-semibold">{{ t('review.stats.title') }}</h2>
        <p class="text-muted-foreground text-sm">{{ t('review.stats.subtitle') }}</p>
      </div>
      <div class="flex items-center gap-2">
        <Button
          v-for="option in [7, 30, 90]"
          :key="option"
          :variant="days === option ? 'default' : 'outline'"
          size="sm"
          @click="emit('update:days', option)"
        >
          {{ t(`review.stats.days${option}`) }}
        </Button>
      </div>
    </div>

    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <Card>
        <CardContent class="flex items-center gap-3 pt-4">
          <Layers class="text-primary size-5" />
          <div>
            <div class="text-2xl font-semibold">{{ stats?.total_cards ?? 0 }}</div>
            <div class="text-muted-foreground text-xs">{{ t('review.totalCards') }}</div>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="flex items-center gap-3 pt-4">
          <Target class="text-primary size-5" />
          <div>
            <div class="text-2xl font-semibold">{{ stats?.reviewed ?? 0 }}</div>
            <div class="text-muted-foreground text-xs">{{ t('review.stats.reviewed') }}</div>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="flex items-center gap-3 pt-4">
          <CalendarDays class="text-primary size-5" />
          <div>
            <div class="text-2xl font-semibold">{{ Math.round((stats?.retention ?? 0) * 100) }}%</div>
            <div class="text-muted-foreground text-xs">{{ t('review.stats.retention') }}</div>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="flex items-center gap-3 pt-4">
          <Flame class="text-primary size-5" />
          <div>
            <div class="text-2xl font-semibold">{{ stats?.streak_days ?? 0 }}</div>
            <div class="text-muted-foreground text-xs">{{ t('review.stats.streak') }}（{{ t('review.stats.daysUnit') }}）</div>
          </div>
        </CardContent>
      </Card>
    </div>

    <div class="grid gap-3 xl:grid-cols-2">
      <Card>
        <CardHeader><CardTitle class="text-sm">{{ t('review.stats.ratingDistribution') }}</CardTitle></CardHeader>
        <CardContent>
          <div ref="distributionEl" class="h-64 w-full" />
          <p v-if="!stats?.rating_distribution.total" class="text-muted-foreground py-16 text-center text-sm">
            {{ t('review.stats.dataInsufficient') }}
          </p>
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle class="text-sm">{{ t('review.stats.ratingTrend') }}</CardTitle></CardHeader>
        <CardContent>
          <div ref="trendEl" class="h-64 w-full" />
          <p v-if="!stats?.rating_trend?.some((point) => point.total > 0)" class="text-muted-foreground py-16 text-center text-sm">
            {{ t('review.stats.dataInsufficient') }}
          </p>
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle class="text-sm">{{ t('review.stats.dueForecast') }}</CardTitle></CardHeader>
        <CardContent><div ref="dueEl" class="h-64 w-full" /></CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle class="text-sm">{{ t('review.stats.forgettingCurve') }}</CardTitle></CardHeader>
        <CardContent>
          <div ref="curveEl" class="h-64 w-full" />
          <p v-if="!stats?.forgetting_curve?.length" class="text-muted-foreground py-16 text-center text-sm">
            {{ t('review.stats.dataInsufficient') }}
          </p>
        </CardContent>
      </Card>
    </div>

    <div class="grid gap-3 xl:grid-cols-2">
      <Card>
        <CardHeader><CardTitle class="text-sm">{{ t('review.stats.groupRanking') }}</CardTitle></CardHeader>
        <CardContent>
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
                <TableCell class="text-right">{{ group.reviewed }}</TableCell>
                <TableCell class="text-right">{{ Math.round(group.retention * 100) }}%</TableCell>
              </TableRow>
            </TableBody>
          </Table>
          <p v-else class="text-muted-foreground py-8 text-center text-sm">{{ t('review.stats.noGroupRanking') }}</p>
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle class="text-sm">{{ t('review.stats.difficultCards') }}</CardTitle></CardHeader>
        <CardContent>
          <div v-if="stats?.difficult_cards?.length" class="space-y-2">
            <div v-for="card in stats.difficult_cards" :key="card.card_id" class="border-border/60 flex items-start gap-2 rounded-md border p-2">
              <AlertTriangle class="text-destructive mt-0.5 size-4 shrink-0" />
              <div class="min-w-0 flex-1">
                <div class="truncate text-sm">{{ card.question }}</div>
                <div class="text-muted-foreground mt-1 flex flex-wrap gap-2 text-xs">
                  <span>{{ card.group_name }}</span>
                  <span>{{ t('review.forgot') }} {{ card.forgot }}/{{ card.total }}</span>
                  <span>{{ Math.round(card.forget_rate * 100) }}%</span>
                </div>
              </div>
            </div>
          </div>
          <p v-else class="text-muted-foreground py-8 text-center text-sm">{{ t('review.stats.noDifficultCards') }}</p>
        </CardContent>
      </Card>
    </div>

    <Card>
      <CardHeader class="flex flex-row items-center justify-between">
        <CardTitle class="text-sm">{{ t('review.stats.sessionHistory') }}</CardTitle>
        <Button variant="outline" size="sm" :disabled="!sessions.length" @click="emit('export-csv')">
          {{ t('review.stats.exportCsv') }}
        </Button>
      </CardHeader>
      <CardContent>
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
              <TableCell class="whitespace-nowrap">{{ formatTime(session.started_at) }}</TableCell>
              <TableCell>{{ session.mode === 'daily' ? t('review.stats.modeDaily') : t('review.stats.modeManual') }}</TableCell>
              <TableCell>{{ session.completed_count }}/{{ session.planned_count }}</TableCell>
              <TableCell>{{ session.remembered_count }}/{{ session.fuzzy_count }}/{{ session.forgot_count }}</TableCell>
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
