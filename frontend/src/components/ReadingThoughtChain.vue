<template>
  <div v-if="advancing" class="rtc rtc--live">
    <div class="rtc__live">
      <div class="rtc__head rtc__head--live">
        <UseAnimationIcon name="loading" :size="20" color="var(--rv-haze-d, #5d7b8a)" />
        <span class="rtc__title">{{ t('reading.rawReasoning') }}</span>
        <span class="rtc__meta">
          {{ livePhaseLabel }}<template v-if="elapsedLabel"> · {{ elapsedLabel }}</template>
        </span>
      </div>
      <p v-if="live?.sectionTitle" class="rtc__live-context">
        <template v-if="live.round">{{ t('reading.roundLabel', { round: live.round }) }} · </template>
        {{ live.sectionTitle }}<template v-if="live.sectionHeading"> · {{ live.sectionHeading }}</template>
      </p>
      <pre v-if="live?.reasoning" ref="liveReasoningRef" class="rtc__live-reasoning">{{ live.reasoning.trim() }}</pre>
      <p v-else class="rtc__live-wait">{{ t('reading.thinkingNow') }}</p>

      <div class="rtc__chain-head">
        <span class="rtc__title">{{ t('reading.thoughtChain') }}</span>
        <span class="rtc__meta">{{ livePhaseLabel }}</span>
      </div>
      <p
        v-for="(search, index) in live?.searches || []"
        :key="`${search.query}-${index}`"
        class="rtc__live-search"
      >
        {{ t('reading.phaseSearch') }}「{{ search.query }}」 · {{ t('reading.searchHits', { count: search.hitCount }) }}
      </p>
    </div>
  </div>
  <div v-else class="rtc">
    <div class="rtc__head">
      <button type="button" class="rtc__toggle" @click="rawOpen = !rawOpen">
        <span class="rtc__title">{{ t('reading.rawReasoning') }}</span>
        <span class="rtc__meta">{{ durationLabel || '—' }}</span>
        <span class="rtc__caret">{{ rawOpen ? '▾' : '▸' }}</span>
      </button>
    </div>
    <pre v-if="rawOpen" class="rtc__raw">{{ reasoning || t('reading.noReasoning') }}</pre>

    <div class="rtc__chain-head rtc__chain-head--done">
      <button type="button" class="rtc__toggle" @click="expanded = !expanded">
        <span class="rtc__title">{{ t('reading.thoughtChain') }}</span>
        <span class="rtc__meta">
          {{ t('reading.thoughtMeta', {
            searches: searches.length,
            questions: questions.length,
            duration: durationLabel || '—',
          }) }}
        </span>
        <span class="rtc__caret">{{ expanded ? '▾' : '▸' }}</span>
      </button>
      <select
        v-if="roundOptions.length > 1"
        class="rtc__round"
        :value="selectedRound"
        @change="onRoundChange"
      >
        <option v-for="option in roundOptions" :key="option.value" :value="option.value">
          {{ option.label }}
        </option>
      </select>
    </div>

    <div v-if="expanded" class="rtc__body">
      <div class="rtc__step">
        <span class="rtc__num">01</span>
        <p>
          <b>{{ t('reading.phaseWindow') }}</b>
          {{ sectionTitle || '—' }}<template v-if="sectionHeading"> · {{ sectionHeading }}</template>
        </p>
      </div>
      <div v-if="searches.length" class="rtc__step">
        <span class="rtc__num">02</span>
        <p>
          <b>{{ t('reading.phaseSearch') }}</b>
          <span v-for="(item, index) in searches" :key="index" class="rtc__search">
            {{ item.query }} · {{ t('reading.searchHits', { count: item.hit_count }) }}
          </span>
        </p>
      </div>
      <div v-if="understanding" class="rtc__step">
        <span class="rtc__num">03</span>
        <p><b>{{ t('reading.phaseUnderstanding') }}</b>{{ understanding }}</p>
      </div>
      <div v-if="questions.length" class="rtc__step">
        <span class="rtc__num">04</span>
        <p>
          <b>{{ t('reading.phaseQuestions') }}</b>
          <span v-for="question in questions" :key="question.id" class="rtc__question">
            {{ question.text }}<template v-if="question.why">（{{ question.why }}）</template>
          </span>
        </p>
      </div>
      <div class="rtc__step">
        <span class="rtc__num">05</span>
        <p><b>{{ t('reading.phaseDeposit') }}</b>{{ t('reading.traceCounts', counts) }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ReadingNotes, ReadingQuestionNode } from '@/api/reading'
import UseAnimationIcon from '@/components/animations/UseAnimationIcon.vue'

const props = defineProps<{
  trace: ReadingNotes
  questions: ReadingQuestionNode[]
  sectionTitle?: string
  sectionHeading?: string
  durationLabel?: string
  roundOptions: Array<{ value: number; label: string }>
  selectedRound: number
  advancing?: boolean
  elapsedLabel?: string
  live?: {
    phase: string
    status: string
    sectionTitle: string
    sectionHeading: string
    round: number
    searches: Array<{ query: string; hitCount: number }>
    reasoning: string
  } | null
}>()

const emit = defineEmits<{
  (event: 'update:selectedRound', value: number): void
}>()

const { t } = useI18n()
const expanded = ref(true)
const liveReasoningRef = ref<HTMLElement | null>(null)
const livePhaseLabel = computed(() => {
  const phase = props.live?.phase || ''
  const labels: Record<string, string> = {
    structure: t('reading.phaseStructure'),
    reading: t('reading.phaseReading'),
    synthesis: t('reading.phaseSynthesis'),
    self_test: t('reading.phaseSelfTest'),
  }
  return labels[phase] || t('reading.thinkingNow')
})
watch(
  () => props.live?.reasoning,
  async () => {
    await nextTick()
    const el = liveReasoningRef.value
    if (el) el.scrollTop = el.scrollHeight
  },
)
const rawOpen = ref(true)
const reasoning = computed(() =>
  typeof props.trace.reasoning === 'string' ? props.trace.reasoning : '',
)
const understanding = computed(() =>
  typeof props.trace.understanding === 'string' ? props.trace.understanding : '',
)
const searches = computed(() => props.trace.search_trace || [])
const counts = computed(() => ({
  conclusions: Number(props.trace.conclusion_count || 0),
  terms: Number(props.trace.term_count || 0),
  contradictions: Number(props.trace.contradiction_count || 0),
}))

function onRoundChange(event: Event) {
  emit('update:selectedRound', Number((event.target as HTMLSelectElement).value))
}
</script>

<style scoped lang="less">
.rtc {
  margin: 0 0 30px;
  font-size: 13px;
  color: var(--rv-text, #37352f);
}

.rtc--live {
  padding: 10px 0;
}

.rtc__live {
  min-width: 0;
}

.rtc__head {
  display: flex;
  gap: 10px;
  align-items: baseline;
}

.rtc__head--live {
  align-items: center;
}

.rtc__head--live .rtc__meta {
  margin-left: auto;
}

.rtc__live-context,
.rtc__live-search,
.rtc__live-wait {
  margin: 6px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--rv-text-3, #9b9a97);
}

.rtc__live-wait {
  margin-top: 10px;
}

.rtc__live-search {
  color: var(--rv-haze-d, #5d7b8a);
}

.rtc__live-reasoning {
  max-height: 420px;
  padding: 10px 12px;
  margin: 10px 0 0;
  overflow-y: auto;
  font-family: inherit;
  font-size: 13px;
  line-height: 1.7;
  color: var(--rv-text-2, #787774);
  white-space: pre-wrap;
  background: var(--rv-side, #faf9f7);
  border-radius: 3px;
  scrollbar-width: none;
}

.rtc__live-reasoning::-webkit-scrollbar {
  display: none;
}

.rtc__chain-head {
  display: flex;
  gap: 10px;
  align-items: baseline;
  padding-top: 12px;
  margin-top: 16px;
  border-top: 1px solid var(--rv-hair, rgba(55, 53, 47, 0.1));
}

.rtc__chain-head--done {
  align-items: center;
}

.rtc__toggle {
  display: flex;
  flex: 1;
  gap: 10px;
  align-items: baseline;
  min-width: 0;
  padding: 0;
  font-family: inherit;
  color: inherit;
  text-align: left;
  cursor: pointer;
  background: transparent;
  border: 0;
}

.rtc__title {
  flex: none;
  font-size: 13px;
  font-weight: 600;
  color: var(--rv-text-2, #787774);
}

.rtc__meta {
  margin-left: auto;
  overflow: hidden;
  font-size: 11.5px;
  color: var(--rv-text-3, #9b9a97);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rtc__caret {
  flex: none;
  font-size: 11px;
  color: var(--rv-text-3, #9b9a97);
}

.rtc__round {
  flex: none;
  max-width: 110px;
  height: 24px;
  padding: 0 6px;
  font-family: inherit;
  font-size: 11.5px;
  color: var(--rv-text-3, #9b9a97);
  background: transparent;
  border: 0;
  outline: none;
}

.rtc__body {
  padding-top: 14px;
}

.rtc__step {
  display: flex;
  gap: 12px;
  margin-bottom: 10px;
}

.rtc__num {
  flex: none;
  width: 20px;
  padding-top: 1px;
  font-size: 12px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--rv-haze-d, #5d7b8a);
}

.rtc__step p {
  flex: 1;
  margin: 0;
  font-size: 13.5px;
  line-height: 1.65;
  color: var(--rv-text-2, #787774);
}

.rtc__step b {
  margin-right: 4px;
  font-weight: 500;
  color: var(--rv-text, #37352f);
}

.rtc__search,
.rtc__question {
  display: block;
}

.rtc__raw {
  max-height: 360px;
  padding: 12px 14px;
  margin: 10px 0 0;
  overflow-y: auto;
  font-family: inherit;
  font-size: 12.5px;
  line-height: 1.7;
  color: var(--rv-text-2, #787774);
  white-space: pre-wrap;
  background: var(--rv-side, #faf9f7);
  border-radius: 3px;
  scrollbar-width: none;
}

.rtc__raw::-webkit-scrollbar {
  display: none;
}
</style>
