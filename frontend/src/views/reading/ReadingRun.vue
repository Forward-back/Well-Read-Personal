<template>
  <div
    ref="readingPageRef"
    class="reading-page"
    :class="{ 'reading-page--config': showConfigView, 'reading-page--workbench': !showConfigView, 'is-narrow': pageNarrow }"
  >
    <header class="rv-topbar">
      <div class="rv-topbar__doc">
        <span class="rv-topbar__title">{{ showConfigView ? $t('reading.createTitle') : workbenchTitle }}</span>
        <span v-if="run && !showConfigView" class="rv-topbar__meta">{{ workbenchMeta }}</span>
      </div>
      <nav v-if="run" class="rv-switch" :aria-label="$t('reading.viewSwitch')">
        <button type="button" :class="{ on: !showConfigView }" @click="switchView('workbench')">
          {{ $t('reading.viewWorkbench') }}
        </button>
        <button type="button" :class="{ on: showConfigView }" @click="switchView('config')">
          {{ $t('reading.viewConfig') }}
        </button>
      </nav>
      <button type="button" class="rv-action-btn rv-history-btn" @click="historyOpen = true">
        {{ $t('reading.history') }}
      </button>
      <div class="rv-topbar__spacer" />
      <div v-if="run && !showConfigView" class="rv-progress">
        <span class="rv-progress__bar"><i :style="{ width: readPercent + '%' }" /></span>
        <span class="rv-progress__text">
          {{ $t('reading.progressText', {
            read: readSectionCount,
            total: totalSectionCount,
            minutes: remainingMinutes,
          }) }}
        </span>
      </div>
      <div v-if="run && !showConfigView" class="rv-actions">
        <button type="button" class="rv-mobile-toggle" @click="mobileLeftOpen = true">
          {{ $t('reading.outline') }}
        </button>
        <button type="button" class="rv-mobile-toggle" @click="mobileRightOpen = true">
          {{ $t('reading.notes') }}
        </button>
        <t-dropdown :options="exportOptions" @click="handleExportMenu">
          <button type="button" class="rv-action-btn">{{ $t('reading.export') }} ▾</button>
        </t-dropdown>
        <button v-if="showReportEditor" type="button" class="rv-action-btn" @click="openReportEditor">
          {{ $t('reading.reportTitleShort') }}
        </button>
        <button v-if="canPauseRun" type="button" class="rv-action-btn" @click="pauseRun">
          {{ $t('reading.pause') }}
        </button>
        <button v-if="canPauseRun" type="button" class="rv-action-btn is-warn" @click="abandonRun">
          {{ $t('reading.stopShort') }}
        </button>
      </div>
    </header>

    <div v-if="showConfigView" class="rv-config">
      <div class="rv-config__inner">
        <h1>{{ $t('reading.createTitle') }}</h1>
        <p class="rv-config__sub">{{ $t('reading.createSub') }}</p>

        <section class="rv-section">
          <div class="rv-section__label">{{ $t('reading.documentPicker') }}</div>
          <input v-model="docSearch" class="rv-field" :placeholder="$t('reading.searchDocsAll')" />
          <div v-if="docSearchActive" class="rv-doc-list">
            <button
              v-for="doc in filteredAllDocuments"
              :key="doc.value"
              type="button"
              class="rv-doc-row"
              :class="{ on: documentIds.includes(doc.value) }"
              @click="toggleDocument(doc.value)"
            >
              <span class="rv-doc-row__name">{{ doc.label }}</span>
              <span class="rv-doc-row__meta">{{ doc.kbName }} · {{ doc.sizeLabel }}</span>
            </button>
            <p v-if="!filteredAllDocuments.length" class="rv-empty">{{ $t('reading.noDocuments') }}</p>
          </div>
          <div v-else class="rv-doc-tree">
            <button
              v-for="row in documentTreeRows"
              :key="row.key"
              type="button"
              class="rv-tree-row"
              :class="{
                'is-doc': row.kind === 'doc',
                'is-branch': row.kind !== 'doc',
                on: row.kind === 'doc' && row.doc && documentIds.includes(row.doc.value),
              }"
              :style="{ paddingLeft: 8 + row.depth * 16 + 'px' }"
              @click="handleTreeRow(row)"
            >
              <span class="rv-tree-row__caret">
                {{ row.kind === 'doc' ? '' : row.expanded ? '▾' : '▸' }}
              </span>
              <span class="rv-tree-row__label">{{ row.label }}</span>
              <span v-if="row.count !== undefined" class="rv-tree-row__count">{{ row.count }}</span>
              <span v-if="row.kind === 'doc' && row.doc && documentIds.includes(row.doc.value)" class="rv-tree-row__check">✓</span>
            </button>
            <p v-if="!documentTreeRows.length" class="rv-empty">{{ $t('reading.noDocuments') }}</p>
          </div>
          <p class="rv-hint">{{ $t('reading.documentPickerHint') }}</p>
          <div v-if="selectedDocuments.length" class="rv-selected-docs">
            <div class="rv-selected-docs__label">
              {{ $t('reading.selectedDocs', { count: selectedDocuments.length }) }}
            </div>
            <div class="rv-selected-docs__list">
              <button
                v-for="doc in selectedDocuments"
                :key="doc.value"
                type="button"
                class="rv-selected-doc"
                :title="$t('reading.removeDoc')"
                @click="toggleDocument(doc.value)"
              >
                <span class="rv-selected-doc__name">{{ doc.label }}</span>
                <span class="rv-selected-doc__kb">{{ doc.kbName }}</span>
                <span class="rv-selected-doc__remove">×</span>
              </button>
            </div>
          </div>
        </section>

        <section class="rv-section">
          <div class="rv-section__label">{{ $t('reading.depthLabel') }}</div>
          <button
            v-for="item in depthOptions"
            :key="item.value"
            type="button"
            class="rv-depth-row"
            :class="{ on: depth === item.value }"
            @click="depth = item.value"
          >
            <span class="rv-depth-row__name">{{ item.label }}</span>
            <span class="rv-depth-row__desc">{{ item.hint }}</span>
          </button>
        </section>

        <section class="rv-section">
          <div class="rv-section__label">{{ $t('reading.goalLabel') }}</div>
          <div class="rv-chips">
            <button
              v-for="chip in goalChips"
              :key="chip.key"
              type="button"
              class="rv-chip"
              :class="{ on: goal === chip.value }"
              @click="selectGoalChip(chip)"
            >
              {{ chip.label }}
            </button>
          </div>
          <textarea
            ref="goalTextareaRef"
            v-model="goal"
            class="rv-textarea"
            rows="2"
            :placeholder="$t('reading.goalPlaceholder')"
          />
        </section>

        <div class="rv-estimate">
          <span>{{ $t('reading.estimateTime', { minutes: estimate.minutes }) }}</span>
          <span>{{ $t('reading.estimateRounds', { rounds: estimate.rounds }) }}</span>
          <span>{{ $t('reading.estimateTokens', { tokens: estimate.tokens }) }}</span>
          <span>{{ $t('reading.estimateQuestions', { count: estimate.questions }) }}</span>
        </div>

        <div class="rv-config__actions">
          <button
            type="button"
            class="rv-primary"
            :disabled="creating || !documentIds.length"
            @click="startRun"
          >
            {{ $t('reading.start') }}
          </button>
          <button type="button" class="rv-text-btn" :disabled="!documentIds.length" @click="saveDraft">
            {{ $t('reading.saveDraft') }}
          </button>
          <span class="rv-config__note">{{ $t('reading.modelNote') }}</span>
        </div>

        <section v-if="drafts.length" class="rv-drafts">
          <button type="button" class="rv-drafts__toggle" @click="draftsExpanded = !draftsExpanded">
            {{ $t('reading.drafts') }} <span>{{ drafts.length }}</span>
            <span class="rv-drafts__caret">{{ draftsExpanded ? '▾' : '▸' }}</span>
          </button>
          <div v-if="draftsExpanded" class="rv-drafts__list">
            <div v-for="draft in drafts" :key="draft.id" class="rv-draft">
              <template v-if="renamingDraftId === draft.id">
                <input
                  v-model="renameDraftValue"
                  class="rv-draft__input"
                  @keydown.enter="confirmRenameDraft(draft.id)"
                  @keydown.esc="cancelRenameDraft"
                />
                <button type="button" class="rv-draft__action" @click="confirmRenameDraft(draft.id)">
                  {{ $t('common.confirm') }}
                </button>
                <button type="button" class="rv-draft__action" @click="cancelRenameDraft">
                  {{ $t('common.cancel') }}
                </button>
              </template>
              <template v-else>
                <button type="button" class="rv-draft__name" @click="loadDraft(draft)">
                  {{ draft.name }}
                </button>
                <span class="rv-draft__meta">
                  {{ $t('reading.draftMeta', {
                    docs: draft.documentIds.length,
                    depth: depthLabelOf(draft.depth),
                    date: formatDraftDate(draft.updatedAt),
                  }) }}
                </span>
                <button type="button" class="rv-draft__action" @click="startRenameDraft(draft)">
                  {{ $t('reading.renameDraft') }}
                </button>
                <button type="button" class="rv-draft__action is-warn" @click="deleteDraft(draft.id)">
                  {{ $t('reading.deleteDraft') }}
                </button>
              </template>
            </div>
          </div>
        </section>
      </div>
    </div>

    <div v-else-if="run" ref="readingBodyRef" class="rv-body">
      <aside class="rv-left" :class="{ 'is-open': mobileLeftOpen }">
        <div class="rv-left__mobile-head">
          <span>{{ $t('reading.outline') }}</span>
          <button type="button" @click="mobileLeftOpen = false">×</button>
        </div>
        <div class="rv-left__head">
          <p>{{ depthLabel }} · {{ $t('reading.policyTag') }}</p>
        </div>
        <div class="rv-left__label">{{ $t('reading.chapters') }}</div>
        <button
          v-for="row in outlineRows"
          :key="row.key"
          type="button"
          class="rv-toc-row"
          :class="{
            'is-root': row.kind === 'doc',
            current: row.current,
            preview: row.preview,
            read: row.read,
          }"
          :style="{ paddingLeft: 10 + row.depth * 14 + 'px' }"
          @click="handleOutlineRow(row)"
        >
          <span
            v-if="row.hasChildren"
            class="rv-toc-row__caret"
            @click.stop="toggleOutlineRow(row)"
          >{{ row.expanded ? '▾' : '▸' }}</span>
          <span v-else class="rv-toc-row__caret" />
          <span class="rv-toc-row__text">{{ row.label }}</span>
          <span v-if="row.current" class="rv-toc-row__mark">{{ $t('reading.currentTag') }}</span>
        </button>
        <div class="rv-left__label">{{ $t('reading.sessionStats') }}</div>
        <div class="rv-session-stats">
          <p>{{ $t('reading.sessionRead', { read: readSectionCount, total: totalSectionCount }) }}</p>
          <p>{{ $t('reading.sessionNotes', { count: noteTotalCount }) }}</p>
          <p>{{ $t('reading.sessionSkipped', { count: skippedQuestionCount }) }}</p>
        </div>
        <div v-if="canAddDocs" class="rv-left__add">
          <button type="button" class="rv-text-btn" @click="openAddDocs">{{ $t('reading.addDocuments') }}</button>
        </div>
      </aside>

      <main class="rv-main">
        <div class="rv-read">
          <div class="rv-chapter-head">
            <div>
              <h2>{{ viewingSectionTitle }}</h2>
              <p class="rv-chapter-meta">
                {{ viewingSectionMeta }}
                <span v-if="run.quiet" class="rv-quiet-tag">{{ $t('reading.quietMode') }}</span>
              </p>
            </div>
            <div class="rv-chapter-actions">
              <button type="button" class="rv-action-btn" @click="openOriginal">{{ $t('reading.originalText') }}</button>
              <button
                v-if="run.status !== 'planning' && viewingSection"
                type="button"
                class="rv-action-btn"
                @click="openFeynman"
              >
                {{ $t('reading.feynman') }}
              </button>
              <button
                v-if="previewSectionId"
                type="button"
                class="rv-action-btn"
                @click="clearPreview"
              >
                {{ $t('reading.backToCurrent') }}
              </button>
            </div>
          </div>

          <template v-if="run.status === 'planning'">
            <ReadingThoughtChain
              v-if="advancing"
              :trace="currentRoundTrace"
              :questions="roundQuestions"
              :section-title="traceSection?.title"
              :section-heading="traceSectionHeading"
              :round-options="traceRoundOptions"
              v-model:selected-round="selectedTraceRound"
              :advancing="advancing"
              :live="liveProgress"
              :elapsed-label="liveElapsedLabel"
            />
            <div class="rv-plan-card">
              <h3>{{ $t('reading.planConfirmTitle') }}</h3>
              <p>{{ $t('reading.planConfirmHint', { sections: run.plan.sections?.length || 0, docs: run.document_ids.length }) }}</p>
              <div class="rv-plan-card__meta">
                <span>{{ $t('reading.reportGoal') }}：{{ run.plan.goal || '—' }}</span>
                <span>{{ $t('reading.reportDepth') }}：{{ depthLabel }}</span>
                <span>{{ $t('reading.reportPolicy') }}：{{ run.plan.policy || 'grill-with-docs' }}</span>
              </div>
              <div v-if="planStructure.length" class="rv-plan-structure">
                <div class="rv-block-label">{{ $t('reading.structureTitle') }}</div>
                <div v-for="entry in planStructure" :key="entry.section_id" class="rv-plan-structure__row">
                  <div class="rv-plan-structure__head">
                    <span class="rv-plan-structure__section">{{ sectionTitleByID(entry.section_id) }}</span>
                    <span class="rv-plan-structure__type">{{ entry.knowledge_type || 'unknown' }}</span>
                  </div>
                  <p>{{ entry.thesis || '—' }}</p>
                </div>
              </div>
              <p v-if="run.plan.structure_degraded" class="rv-hint">{{ $t('reading.structureDegraded') }}</p>
              <button v-if="!advancing" type="button" class="rv-primary" :disabled="acting" @click="confirmPlan">
                {{ $t('reading.startReading') }}
              </button>
            </div>
          </template>

          <div v-else-if="run.status === 'finished'" class="rv-final">
            <ReadingThoughtChain
              :trace="currentRoundTrace"
              :questions="roundQuestions"
              :section-title="traceSection?.title"
              :section-heading="traceSectionHeading"
              :duration-label="selectedTraceDurationLabel"
              :round-options="traceRoundOptions"
              v-model:selected-round="selectedTraceRound"
            />
            <h3>{{ $t('reading.final') }}</h3>
            <div v-if="finalSummaryParagraphs.length" class="rv-final__summary">
              <p v-for="(paragraph, index) in finalSummaryParagraphs" :key="index">{{ paragraph }}</p>
            </div>
            <div class="rv-final__stats">
              <span>{{ $t('reading.confirmedCount', { count: confirmedConclusions.length }) }}</span>
              <span>{{ $t('reading.openCount', { count: openQuestions.length }) }}</span>
              <span>{{ $t('reading.contradictionCount', { count: structuredContradictions.length }) }}</span>
            </div>
            <section v-if="run.argument_chain_mermaid" class="rv-argument">
              <div class="rv-argument__head">
                <span class="rv-block-label">{{ $t('reading.argumentChain') }}</span>
                <button type="button" class="rv-text-btn" @click="openArgumentFullscreen">
                  {{ $t('reading.mermaidFullscreen') }}
                </button>
              </div>
              <div
                v-if="argumentChainSvg"
                class="rv-argument__canvas"
                @click="handleArgumentClick"
                v-html="argumentChainSvg"
              />
              <p v-else class="rv-hint">{{ $t('reading.argumentChainRendering') }}</p>
            </section>
            <div v-for="group in finalTextGroups" :key="group.key" class="rv-final__group">
              <h4>{{ group.label }}</h4>
              <p v-if="!group.items.length" class="rv-empty">—</p>
              <ol v-else class="rv-final__list">
                <li v-for="(item, index) in group.items" :key="index">
                  <strong v-if="item.lead">{{ item.lead }}</strong>{{ item.lead ? '：' : '' }}{{ item.rest }}
                </li>
              </ol>
            </div>
            <div v-if="finalCitations.length" class="rv-final__group">
              <h4>{{ $t('reading.finalGroups.citations') }}</h4>
              <ul class="rv-final__cites">
                <li v-for="cite in finalCitations" :key="cite.key">
                  <button type="button" class="rv-final__cite" @click="openFinalCitation(cite)">
                    {{ cite.title }}
                  </button>
                </li>
              </ul>
            </div>
            <div v-if="agentPhaseRows.length" class="rv-phase-trace">
              <span v-for="row in agentPhaseRows" :key="row.key" class="rv-phase-trace__item" :class="{ 'is-degraded': row.degraded }">
                {{ row.label }}<template v-if="row.degraded">（{{ $t('reading.phaseDegraded') }}）</template>
              </span>
            </div>
            <div v-if="(run.plan.state_version || 0) >= 3" class="rv-selftests">
              <div class="rv-block-label">{{ $t('reading.selfTests') }}</div>
              <p v-if="selfTestsLoading" class="rv-hint">{{ $t('reading.selfTestsLoading') }}</p>
              <p v-else-if="!selfTests.length" class="rv-hint">{{ $t('reading.selfTestsEmpty') }}</p>
              <template v-else>
                <div class="rv-selftests__toolbar">
                  <select v-model="selectedReviewGroupId" class="rv-selftests__group">
                    <option value="">{{ $t('reading.selfTestSelectGroup') }}</option>
                    <option v-for="group in reviewGroupOptions" :key="group.value" :value="group.value">
                      {{ group.label }}
                    </option>
                  </select>
                  <button
                    type="button"
                    class="rv-primary"
                    :disabled="!selectedReviewGroupId || !selectedSelfTestIds.length || importingSelfTests"
                    @click="importSelectedSelfTests"
                  >
                    {{ $t('reading.selfTestImport', { count: selectedSelfTestIds.length }) }}
                  </button>
                </div>
                <label
                  v-for="test in selfTests"
                  :key="test.id"
                  class="rv-selftest"
                  :class="{ 'is-imported': test.status === 'imported' }"
                >
                  <input
                    type="checkbox"
                    :checked="selectedSelfTestIds.includes(test.id)"
                    :disabled="test.status !== 'candidate'"
                    @change="toggleSelfTest(test)"
                  />
                  <span class="rv-selftest__body">
                    <span class="rv-selftest__question">{{ test.question }}</span>
                    <span class="rv-selftest__answer">{{ test.answer }}</span>
                    <span v-if="test.explanation" class="rv-selftest__explanation">{{ test.explanation }}</span>
                    <span v-if="test.evidence?.length" class="rv-selftest__evidence">
                      {{ $t('reading.selfTestSource') }}：{{ test.evidence[0].quote }}
                    </span>
                    <span v-if="test.status !== 'candidate'" class="rv-selftest__status">
                      {{ $t(`reading.selfTestStatus.${test.status}`) }}
                    </span>
                  </span>
                </label>
              </template>
            </div>
            <button type="button" class="rv-primary" @click="switchView('config')">
              {{ $t('reading.newReading') }}
            </button>
          </div>

          <div v-else-if="run.status === 'stopped' || run.status === 'failed' || run.status === 'abandoned'" class="rv-idle">
            <p>{{ run.status === 'abandoned' ? $t('reading.abandonedHint') : $t('reading.pausedHint') }}</p>
            <button
              v-if="run.status !== 'abandoned'"
              type="button"
              class="rv-primary"
              :disabled="acting"
              @click="resumeOrAdvance"
            >
              {{ run.status === 'stopped' ? $t('reading.resume') : $t('reading.continueReading') }}
            </button>
            <p v-if="run.status === 'stopped'" class="rv-hint">{{ $t('reading.stoppedHint') }}</p>
          </div>

          <div v-else-if="previewSectionId && previewSectionId !== run.current_section_id" class="rv-preview">
            <div class="rv-prose">{{ previewSectionData?.content }}</div>
            <div v-if="previewConclusions.length" class="rv-preview__notes">
              <div class="rv-block-label">{{ $t('reading.sectionResults') }}</div>
              <p v-for="item in previewConclusions" :key="item.id" class="rv-preview__note">{{ item.text }}</p>
            </div>
            <div class="rv-preview__actions">
              <button type="button" class="rv-primary" @click="continueFromPreview">
                {{ $t('reading.continueFromSection') }}
              </button>
              <p class="rv-hint">{{ $t('reading.previewHint') }}</p>
            </div>
          </div>

          <template v-else>
            <ReadingThoughtChain
              :trace="currentRoundTrace"
              :questions="roundQuestions"
              :section-title="traceSection?.title"
              :section-heading="traceSectionHeading"
              :duration-label="selectedTraceDurationLabel"
              :round-options="traceRoundOptions"
              :advancing="advancing"
              :live="liveProgress"
              :elapsed-label="liveElapsedLabel"
              v-model:selected-round="selectedTraceRound"
            />

            <section v-if="roundConclusions.length" class="rv-block">
              <div class="rv-block-label">{{ $t('reading.roundConclusions') }}</div>
              <div class="rv-prose">
                <div v-for="(conclusion, index) in roundConclusions" :key="conclusion.id" class="rv-conclusion">
                  <p :class="{ 'is-highlight': index === 0 }">
                    {{ conclusion.text }}
                    <button
                      v-for="(evidence, sourceIndex) in citationSources(conclusion)"
                      :key="evidence.chunk_id + evidence.quote"
                      type="button"
                      class="rv-cite"
                      v-evidence-hover="evidence"
                      @click="openEvidence(evidence)"
                    >[{{ sourceIndex + 1 }}]</button>
                  </p>
                  <div v-if="citationSources(conclusion).length" class="rv-conclusion-sources">
                    <span class="rv-conclusion-sources__label">{{ $t('reading.sourceLabel') }}</span>
                    <button
                      v-for="(evidence, sourceIndex) in citationSources(conclusion)"
                      :key="'source-' + evidence.chunk_id + evidence.quote"
                      type="button"
                      class="rv-conclusion-source"
                      v-evidence-hover="evidence"
                      @click="openEvidence(evidence)"
                    >
                      [{{ sourceIndex + 1 }}] {{ evidenceLabel(evidence) }}
                    </button>
                  </div>
                </div>
              </div>
            </section>

            <section v-if="questionCard" class="rv-qcard">
              <div class="rv-qcard__tag">
                {{ questionKindLabel(questionCard) }}
                <span v-if="pendingQueueCount"> · {{ $t('reading.questionIndex', { index: 1, total: pendingQueueCount + 1 }) }}</span>
              </div>
              <p class="rv-qcard__text">{{ questionCard.text }}</p>
              <p v-if="questionCard.why" class="rv-qcard__hint">{{ questionCard.why }}</p>

              <div v-if="!inlineAnswerSubmitted" class="rv-options">
                <button
                  v-for="(option, index) in answerOptions"
                  :key="option.id"
                  type="button"
                  class="rv-option"
                  :class="{ on: selectedAnswerId === option.id }"
                  @click="selectAnswer(option.id)"
                >
                  <span class="rv-option__label">{{ option.label }}</span>
                  <span class="rv-option__text">{{ option.text }}</span>
                  <span v-if="option.recommended" class="rv-option__badge">{{ $t('reading.recommended') }}</span>
                </button>
                <button
                  type="button"
                  class="rv-option"
                  :class="{ on: customAnswerMode }"
                  @click="enableCustomAnswer"
                >
                  <span class="rv-option__label">{{ $t('reading.customAnswer') }}</span>
                  <span class="rv-option__text">{{ $t('reading.customAnswerHint') }}</span>
                </button>
                <textarea
                  v-if="customAnswerMode"
                  ref="answerTextareaRef"
                  v-model="answerDraft"
                  class="rv-textarea rv-textarea--answer"
                  rows="2"
                  :placeholder="$t('reading.answerPlaceholder')"
                />
              </div>

              <div v-if="!inlineAnswerSubmitted" class="rv-qcard__actions">
                <button
                  type="button"
                  class="rv-primary rv-primary--small"
                  :disabled="!answerReady || answering"
                  @click="submitInlineAnswer"
                >
                  {{ $t('reading.submitAnswer') }}
                </button>
                <button type="button" class="rv-text-btn" :disabled="answering" @click="skipInlineQuestion">
                  {{ $t('reading.skipQuestion') }}
                </button>
              </div>

              <div v-else class="rv-verdict">
                <span class="rv-verdict__state">
                  {{ questionCard?.judgment ? questionJudgmentLabel : $t('reading.answerRecorded') }}
                </span>
                <span v-if="questionCard?.judgment_reason" class="rv-verdict__reason">
                  {{ questionCard.judgment_reason }}
                </span>
                <span v-else>{{ $t('reading.answerPendingEvaluation') }}</span>
                <button
                  v-if="!questionCard?.judgment"
                  type="button"
                  class="rv-text-btn"
                  :disabled="answering"
                  @click="retryQuestionJudgment"
                >
                  {{ $t('reading.retryJudgment') }}
                </button>
                <button type="button" class="rv-text-btn" @click="continueAfterAnswer">
                  {{ $t('reading.continueReading') }}
                </button>
              </div>
            </section>

            <section v-if="currentUnderstanding" class="rv-stage">
              <div class="rv-stage__tag">{{ $t('reading.stageConfirm') }}</div>
              <p class="rv-stage__text">
                {{ currentUnderstanding }}
                <span class="rv-stage__section">{{ stageSectionLabel }}</span>
              </p>
              <div v-if="!stageDecision" class="rv-stage__actions">
                <button type="button" class="rv-primary rv-primary--small" @click="decideStage('confirmed')">
                  ✓ {{ $t('reading.confirmUnderstanding') }}
                </button>
                <button type="button" class="rv-text-btn" @click="decideStage('corrected')">
                  {{ $t('reading.correctUnderstanding') }}
                </button>
                <button type="button" class="rv-text-btn" @click="decideStage('skipped')">
                  {{ $t('reading.skipQuestion') }}
                </button>
              </div>
              <div v-else class="rv-stage__result">
                <span v-if="stageDecision === 'confirmed'">{{ $t('reading.stageConfirmed') }}</span>
                <span v-else-if="stageDecision === 'skipped'">{{ $t('reading.stageSkipped') }}</span>
                <span v-else>{{ $t('reading.stageCorrected') }}</span>
                <button type="button" class="rv-text-btn" @click="resetStageDecision">
                  {{ $t('reading.undo') }}
                </button>
              </div>
              <textarea
                v-if="stageCorrecting"
                v-model="stageCorrection"
                class="rv-textarea rv-textarea--answer"
                rows="2"
                :placeholder="$t('reading.correctionPlaceholder')"
              />
              <div v-if="stageCorrecting" class="rv-stage__actions">
                <button type="button" class="rv-primary rv-primary--small" :disabled="!stageCorrection.trim()" @click="submitStageCorrection">
                  {{ $t('reading.submitCorrection') }}
                </button>
                <button type="button" class="rv-text-btn" @click="cancelStageCorrection">
                  {{ $t('common.cancel') }}
                </button>
              </div>
            </section>

            <section
              v-for="item in openContradictions"
              :key="item.id"
              class="rv-contra"
              :class="{ resolved: item.status !== 'open' }"
            >
              <div class="rv-contra__tag">
                {{ item.status === 'open' ? $t('reading.contradictionFound') : $t('reading.contradictionResolved') }}
              </div>
              <p class="rv-contra__title">{{ item.description }}</p>
              <div class="rv-quote">
                <div class="rv-quote__label">{{ $t('reading.sideA') }}</div>
                <p v-for="evidence in item.evidence_a || []" :key="evidence.chunk_id + evidence.quote" class="rv-quote__text is-a">
                  {{ evidence.quote }}
                </p>
              </div>
              <div class="rv-quote">
                <div class="rv-quote__label">{{ $t('reading.sideB') }}</div>
                <p v-for="evidence in item.evidence_b || []" :key="evidence.chunk_id + evidence.quote" class="rv-quote__text is-b">
                  {{ evidence.quote }}
                </p>
              </div>
              <p v-if="item.status !== 'open'" class="rv-contra__resolution">
                {{ $t('reading.resolution') }}：{{ item.resolution }}
              </p>
              <div v-else class="rv-contra__actions">
                <button type="button" class="rv-primary rv-primary--small" :disabled="resolvingId === item.id" @click="resolveContradiction(item, 'a')">
                  {{ $t('reading.resolveWithA') }}
                </button>
                <button type="button" class="rv-primary rv-primary--small" :disabled="resolvingId === item.id" @click="resolveContradiction(item, 'b')">
                  {{ $t('reading.resolveWithB') }}
                </button>
                <button type="button" class="rv-text-btn" @click="markContradictionUncertain(item)">
                  {{ $t('reading.markUncertain') }}
                </button>
                <button type="button" class="rv-text-btn" @click="toggleCustomResolution(item)">
                  {{ $t('reading.resolveCustom') }}
                </button>
              </div>
              <template v-if="customResolutionId === item.id">
                <textarea
                  v-model="customResolution"
                  class="rv-textarea rv-textarea--answer"
                  rows="2"
                  :placeholder="$t('reading.resolutionPlaceholder')"
                />
                <button
                  type="button"
                  class="rv-primary rv-primary--small"
                  :disabled="resolvingId === item.id"
                  @click="resolveContradiction(item, 'custom')"
                >
                  {{ $t('reading.submitResolution') }}
                </button>
              </template>
            </section>

            <div v-if="pauseEachRound && !advancing && run.status === 'reading' && !currentPendingQuestion" class="rv-round-continue">
              <button type="button" class="rv-primary" :disabled="acting" @click="resumeOrAdvance">
                {{ $t('reading.continueReading') }}
              </button>
              <p class="rv-hint">{{ $t('reading.roundReviewHint') }}</p>
            </div>
          </template>
        </div>
      </main>

      <aside
        class="rv-right"
        :class="{ 'is-open': mobileRightOpen }"
        :style="pageNarrow ? undefined : { width: rightWidth + 'px' }"
      >
        <div
          class="rv-right__resizer"
          role="separator"
          aria-orientation="vertical"
          tabindex="0"
          @pointerdown="startRightResize"
          @keydown="onRightResizeKeydown"
        />
        <div class="rv-right__scroll">
          <div class="rv-right__mobile-head">
            <span>{{ $t('reading.notes') }}</span>
            <button type="button" @click="mobileRightOpen = false">×</button>
          </div>
          <div class="rv-notes__head">
            <span>{{ $t('reading.notes') }}</span>
            <span class="rv-notes__count">{{ noteTotalCount }} {{ $t('reading.noteCountUnit') }}</span>
          </div>
          <div class="rv-annotations">
            <div class="rv-annotations__head">
              <span>{{ $t('reading.annotations') }}</span>
              <span class="rv-notes__count">{{ annotations.length }}</span>
            </div>
            <p v-if="!annotations.length" class="rv-hint">{{ $t('reading.annotationsEmpty') }}</p>
            <article v-for="annotation in annotations" :key="annotation.id" class="rv-annotation">
              <button type="button" class="rv-annotation__quote" @click="openAnnotationSource(annotation)">
                「{{ annotation.quote }}」
              </button>
              <p v-if="annotation.comment" class="rv-annotation__comment">{{ annotation.comment }}</p>
              <div class="rv-annotation__actions">
                <button type="button" class="rv-note__action" @click="openAnnotationSource(annotation)">
                  {{ $t('reading.annotationOpenSource') }}
                </button>
                <button type="button" class="rv-note__action" @click="openAnnotationCard(annotation)">
                  {{ $t('reading.annotationMakeCard') }}
                </button>
                <button type="button" class="rv-note__action is-danger" @click="removeAnnotation(annotation)">
                  {{ $t('common.delete') }}
                </button>
              </div>
            </article>
          </div>
          <div class="rv-notes__tabs">
            <button
              v-for="tab in noteTabs"
              :key="tab.key"
              type="button"
              class="rv-notes__tab"
              :class="[{ on: activeTab === tab.key }, 'is-' + tab.key]"
              @click="activeTab = tab.key"
            >
              {{ tab.label }}<span v-if="tab.count" class="rv-notes__tab-count">{{ tab.count }}</span>
            </button>
          </div>
          <div class="rv-notes__list">
            <p v-if="!visibleNotes.length" class="rv-empty">{{ $t('reading.notesEmpty') }}</p>
            <article
              v-for="note in visibleNotes"
              :key="note.key"
              class="rv-note"
              :class="[{ rejected: note.status === 'rejected', 'is-tinted': activeTab === 'all' }, 'is-' + note.type]"
            >
              <div class="rv-note__head">
                <span class="rv-note__type" :class="'is-' + note.type">{{ note.typeLabel }}</span>
                <span class="rv-note__state" :class="{ done: note.status === 'confirmed' }">{{ note.statusLabel }}</span>
                <span v-if="note.recommended" class="rv-note__recommended">{{ $t('reading.recommended') }}</span>
              </div>
              <template v-if="editingNoteKey === note.key">
                <textarea v-model="editingNoteText" class="rv-textarea rv-textarea--answer" rows="3" />
                <div class="rv-note__actions">
                  <button type="button" class="rv-note__action is-main" @click="saveNoteEdit(note)">{{ $t('reading.save') }}</button>
                  <button type="button" class="rv-note__action" @click="cancelNoteEdit">{{ $t('common.cancel') }}</button>
                </div>
              </template>
              <template v-else>
                <p class="rv-note__text">{{ note.text }}</p>
                <p v-if="note.cite" class="rv-note__cite">{{ note.cite }}</p>
                <div class="rv-note__actions">
                  <button
                    v-if="note.actions && note.status === 'candidate'"
                    type="button"
                    class="rv-note__action is-main"
                    @click="confirmNote(note)"
                  >✓ {{ $t('reading.confirmNote') }}</button>
                  <button v-if="note.actions" type="button" class="rv-note__action" @click="startNoteEdit(note)">
                    {{ $t('reading.editNote') }}
                  </button>
                  <button
                    v-if="note.actions && note.status === 'candidate'"
                    type="button"
                    class="rv-note__action is-danger"
                    @click="rejectNote(note)"
                  >{{ $t('reading.rejectNote') }}</button>
                  <button
                    v-if="note.actions && note.status === 'rejected'"
                    type="button"
                    class="rv-note__action"
                    @click="restoreNote(note)"
                  >{{ $t('reading.restoreNote') }}</button>
                </div>
              </template>
            </article>
          </div>
        </div>
      </aside>
    </div>

    <div v-if="originalDrawerOpen" class="rv-drawer-mask" @click.self="closeDrawerFromMask">
      <aside class="rv-drawer" :style="{ width: drawerWidth + 'px' }">
        <div
          class="rv-drawer__resizer"
          role="separator"
          aria-orientation="vertical"
          tabindex="0"
          @pointerdown="startDrawerResize"
          @keydown="onDrawerResizeKeydown"
        />
        <header class="rv-drawer__head">
          <span>{{ $t('reading.originalText') }}</span>
          <select v-if="readingDocuments.length > 1" v-model="activeDocumentId" class="rv-drawer__select">
            <option v-for="doc in readingDocumentOptions" :key="doc.value" :value="doc.value">{{ doc.label }}</option>
          </select>
          <div ref="drawerToolbarRef" class="rv-drawer__toolbar" />
          <button
            v-if="previewSelectionText"
            type="button"
            class="rv-action-btn"
            @click="openAnnotationDialog"
          >
            {{ $t('reading.annotationAdd') }}
          </button>
          <button type="button" class="rv-drawer__close" @click="originalDrawerOpen = false">×</button>
        </header>
        <div
          ref="documentPaneRef"
          class="rv-drawer__body rv-drawer__body--preview"
          @mouseup="capturePreviewSelection"
          @click="handleAnnotationClick"
        >
          <div v-if="readingDocumentsLoading" class="rv-empty">{{ $t('reading.loadingDocument') }}</div>
          <DocumentPreview
            v-else-if="activeDocument"
            :source-blob="originalFileBlob"
            file-type="md"
            :file-name="originalFileName"
            :active="originalDrawerOpen"
            :toolbar-target="drawerToolbarRef"
            fill-height
          />
          <div v-else class="rv-empty">{{ $t('reading.noDocumentContent') }}</div>
        </div>
        <div
          v-if="annotationPopoverTarget"
          class="rv-annotation-popover"
          :style="{ top: `${annotationPopover?.top ?? 0}px`, left: `${annotationPopover?.left ?? 0}px` }"
          @click.stop
        >
          <button type="button" class="rv-annotation-popover__close" :aria-label="$t('common.close')" @click="annotationPopover = null">×</button>
          <p v-if="annotationPopoverTarget.comment" class="rv-annotation-popover__comment">{{ annotationPopoverTarget.comment }}</p>
          <p v-else class="rv-annotation-popover__comment is-empty">{{ $t('reading.annotationNoComment') }}</p>
          <div class="rv-annotation-popover__actions">
            <button type="button" class="rv-note__action" @click="openAnnotationFromPopover(annotationPopoverTarget)">
              {{ $t('reading.annotationOpenSource') }}
            </button>
            <button type="button" class="rv-note__action" @click="openAnnotationCardFromPopover(annotationPopoverTarget)">
              {{ $t('reading.annotationMakeCard') }}
            </button>
            <button type="button" class="rv-note__action is-danger" @click="removeAnnotationFromPopover(annotationPopoverTarget)">
              {{ $t('common.delete') }}
            </button>
          </div>
        </div>
      </aside>
    </div>

    <ReadingHistoryDrawer v-model:visible="historyOpen" @open="openHistoryRun" />

    <t-dialog v-model:visible="addDocDialogVisible" :header="$t('reading.addDocuments')" width="560px" :footer="false">
      <div class="reading-add-dialog">
        <input v-model="addDocSearch" class="rv-field" :placeholder="$t('reading.searchAddDocs')" />
        <div class="rv-doc-list rv-doc-list--dialog">
          <button
            v-for="doc in filteredAddDocOptions"
            :key="doc.value"
            type="button"
            class="rv-doc-row"
            :class="{ on: addDocIds.includes(doc.value) }"
            @click="toggleAddDoc(doc.value)"
          >
            <span class="rv-doc-row__name">{{ doc.label }}</span>
            <span class="rv-doc-row__meta">{{ doc.kbName }} · {{ doc.sizeLabel }}</span>
          </button>
        </div>
        <p v-if="!filteredAddDocOptions.length" class="rv-hint">{{ $t('reading.noMoreDocuments') }}</p>
        <div class="rv-dialog-actions">
          <button type="button" class="rv-text-btn" @click="addDocDialogVisible = false">{{ $t('common.cancel') }}</button>
          <button type="button" class="rv-primary" :disabled="!addDocIds.length || addingDocs" @click="confirmAddDocs">
            {{ $t('common.confirm') }}
          </button>
        </div>
      </div>
    </t-dialog>

    <div v-if="reportDialogVisible" class="rv-report-page" role="dialog" aria-modal="true" :aria-label="$t('reading.editReport')">
      <header class="rv-report-page__head">
        <h2>{{ $t('reading.editReport') }}</h2>
        <div class="rv-report-page__actions">
          <button type="button" class="rv-text-btn" :disabled="reportSaving" @click="resetReportDraft">
            {{ $t('reading.resetReport') }}
          </button>
          <button type="button" class="rv-text-btn" @click="reportDialogVisible = false">{{ $t('common.cancel') }}</button>
          <button type="button" class="rv-primary" :disabled="reportSaving" @click="saveReportDraft">
            {{ $t('reading.saveReport') }}
          </button>
        </div>
      </header>
      <div class="rv-report-page__body">
        <div class="rv-report-page__inner">
          <p class="rv-hint rv-report-page__hint">{{ $t('reading.editReportHint') }}</p>
          <div class="rv-report-md" :class="{ 'is-fullscreen': reportFullscreen }">
            <div class="rv-report-md__head">
              <span class="rv-report-md__title">{{ $t('reading.reportMdTitle') }}</span>
              <span class="rv-report-md__count" aria-live="polite">
                {{ $t('reading.reportCharCount', { count: reportDraft.length }) }}
              </span>
              <div class="rv-report-md__modes" role="tablist">
                <button
                  type="button"
                  role="tab"
                  :aria-selected="reportMode === 'edit'"
                  :class="{ active: reportMode === 'edit' }"
                  @click="reportMode = 'edit'"
                >
                  {{ $t('reading.reportModeEdit') }}
                </button>
                <button
                  type="button"
                  role="tab"
                  :aria-selected="reportMode === 'split'"
                  :class="{ active: reportMode === 'split' }"
                  @click="reportMode = 'split'"
                >
                  {{ $t('reading.reportModeSplit') }}
                </button>
                <button
                  type="button"
                  role="tab"
                  :aria-selected="reportMode === 'preview'"
                  :class="{ active: reportMode === 'preview' }"
                  @click="reportMode = 'preview'"
                >
                  {{ $t('reading.reportModePreview') }}
                </button>
              </div>
              <button
                type="button"
                class="rv-report-md__fullscreen"
                :aria-label="reportFullscreen ? $t('shell.exitFullscreen') : $t('shell.fullscreen')"
                :aria-pressed="reportFullscreen"
                :title="reportFullscreen ? $t('shell.exitFullscreen') : $t('shell.fullscreen')"
                @click="reportFullscreen = !reportFullscreen"
              >
                <t-icon :name="reportFullscreen ? 'fullscreen-exit' : 'fullscreen'" />
              </button>
            </div>
            <div class="rv-report-md__body" :class="`is-${reportMode}`">
              <div class="rv-report-ed-pane">
                <textarea
                  v-model="reportDraft"
                  class="rv-report-page__input"
                  spellcheck="false"
                />
              </div>
              <div class="rv-report-pv-pane">
                <div class="reading-report-preview">
                  <template v-for="(part, index) in reportPreviewParts" :key="index">
                    <section v-if="part.kind === 'mermaid' && run?.argument_chain_mermaid" class="rv-argument">
                      <div class="rv-argument__head">
                        <span class="rv-block-label">{{ $t('reading.argumentChain') }}</span>
                        <button
                          v-if="argumentChainSvg"
                          type="button"
                          class="rv-text-btn"
                          @click="openArgumentFullscreen"
                        >
                          {{ $t('reading.mermaidFullscreen') }}
                        </button>
                      </div>
                      <div
                        v-if="argumentChainSvg"
                        class="rv-argument__canvas"
                        @click="handleArgumentClick"
                        v-html="argumentChainSvg"
                      />
                      <p v-else class="rv-hint">{{ $t('reading.argumentChainRendering') }}</p>
                    </section>
                    <div
                      v-else-if="part.kind === 'markdown'"
                      class="reading-report-preview__markdown"
                      v-html="part.html"
                    />
                  </template>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <t-dialog v-model:visible="feynmanVisible" :header="$t('reading.feynmanTitle')" width="720px" :footer="false">
      <div class="rv-feynman">
        <p class="rv-hint">{{ $t('reading.feynmanHint') }}</p>
        <div class="rv-feynman__angles">
          <label v-for="angle in feynmanAngleDefs" :key="angle.id" class="rv-feynman__angle">
            <span class="rv-feynman__angle-head">
              <b>{{ angle.label }}</b>
              <small>{{ angle.prompt }}</small>
            </span>
            <textarea
              v-model="feynmanAnswers[angle.id]"
              class="rv-textarea"
              rows="2"
              maxlength="2000"
              :placeholder="angle.prompt"
            />
          </label>
          <label class="rv-feynman__angle rv-feynman__freeform">
            <span class="rv-feynman__angle-head">
              <b>{{ $t('reading.feynmanFreeform') }}</b>
              <small>{{ $t('reading.feynmanFreeformHint') }}</small>
            </span>
            <textarea
              v-model="feynmanFreeform"
              class="rv-textarea"
              rows="3"
              maxlength="2000"
              :placeholder="$t('reading.feynmanFreeformHint')"
            />
          </label>
        </div>
        <p class="rv-hint">{{ $t('reading.feynmanMinHint') }}</p>
        <div class="rv-dialog-actions">
          <button type="button" class="rv-text-btn" @click="feynmanVisible = false">{{ $t('common.cancel') }}</button>
          <button type="button" class="rv-primary" :disabled="feynmanSubmitting || !feynmanFilled" @click="submitFeynman">
            {{ feynmanSubmitting ? $t('reading.feynmanSubmitting') : $t('reading.feynmanSubmit') }}
          </button>
        </div>
        <div class="rv-feynman__history">
          <p v-if="feynmanHistoryLoading" class="rv-hint">{{ $t('reading.loadingDocument') }}</p>
          <p v-else-if="!feynmanAttempts.length" class="rv-hint">{{ $t('reading.feynmanEmpty') }}</p>
          <article v-for="entry in feynmanHistory" :key="entry.attempt.id" class="rv-feynman__result">
            <div class="rv-feynman__score">
              <strong>{{ entry.attempt.score }}</strong><span>/ 100</span>
              <em>{{ entry.attempt.summary }}</em>
            </div>
            <div class="rv-feynman__dims">
              <span>{{ $t('reading.feynmanCoverage') }} {{ entry.attempt.coverage }}</span>
              <span>{{ $t('reading.feynmanAccuracy') }} {{ entry.attempt.accuracy }}</span>
              <span>{{ $t('reading.feynmanEvidenceScore') }} {{ entry.attempt.evidence_score }}</span>
            </div>
            <details v-for="answer in entry.answers" :key="answer.angle_id" class="rv-feynman__answer">
              <summary>{{ answer.label }}</summary>
              <p>{{ answer.text }}</p>
              <div v-if="answer.feedback" class="rv-feynman__angle-feedback" :class="`is-${answer.feedback.status}`">
                <span class="rv-feynman__angle-status">{{ feynmanStatusLabel(answer.feedback.status) }}</span>
                <span v-if="answer.feedback.summary">{{ answer.feedback.summary }}</span>
                <p v-for="(point, index) in answer.feedback.omissions" :key="`o-${index}`">
                  {{ $t('reading.feynmanOmissions') }}：{{ point.text }}
                </p>
                <p v-for="(point, index) in answer.feedback.errors" :key="`e-${index}`">
                  {{ $t('reading.feynmanErrors') }}：{{ point.text }}
                </p>
              </div>
            </details>
            <div v-if="entry.attempt.omissions?.length" class="rv-feynman__points">
              <b>{{ $t('reading.feynmanOmissions') }}</b>
              <p v-for="(point, index) in entry.attempt.omissions" :key="'o' + index">{{ point.text }}</p>
            </div>
            <div v-if="entry.attempt.errors?.length" class="rv-feynman__points is-error">
              <b>{{ $t('reading.feynmanErrors') }}</b>
              <p v-for="(point, index) in entry.attempt.errors" :key="'e' + index">{{ point.text }}</p>
            </div>
          </article>
        </div>
      </div>
    </t-dialog>

    <t-dialog v-model:visible="annotationDialogVisible" :header="$t('reading.annotationTitle')" width="560px" :footer="false">
      <div class="rv-annotation-dialog">
        <div class="rv-annotation-dialog__quote">「{{ annotationQuote }}」</div>
        <textarea
          v-model="annotationComment"
          class="rv-textarea"
          rows="4"
          maxlength="2000"
          :placeholder="$t('reading.annotationPlaceholder')"
        />
        <div class="rv-dialog-actions">
          <button type="button" class="rv-text-btn" @click="annotationDialogVisible = false">{{ $t('common.cancel') }}</button>
          <button type="button" class="rv-primary" :disabled="annotationSaving" @click="saveAnnotation">
            {{ annotationSaving ? $t('reading.saving') : $t('reading.annotationSave') }}
          </button>
        </div>
      </div>
    </t-dialog>

    <t-dialog v-model:visible="annotationCardVisible" :header="$t('reading.annotationCardTitle')" width="520px" :footer="false">
      <div class="rv-annotation-card-dialog">
        <label class="rv-field-label">{{ $t('reading.annotationCardGroup') }}</label>
        <select v-model="annotationCardGroupId" class="rv-field">
          <option value="">{{ $t('reading.selfTestSelectGroup') }}</option>
          <option v-for="group in reviewGroupOptions" :key="group.value" :value="group.value">{{ group.label }}</option>
        </select>
        <label class="rv-field-label">{{ $t('reading.annotationCardQuestion') }}</label>
        <textarea v-model="annotationCardQuestion" class="rv-textarea" rows="3" />
        <label class="rv-field-label">{{ $t('reading.annotationCardAnswer') }}</label>
        <textarea v-model="annotationCardAnswer" class="rv-textarea" rows="4" />
        <div class="rv-dialog-actions">
          <button type="button" class="rv-text-btn" @click="annotationCardVisible = false">{{ $t('common.cancel') }}</button>
          <button type="button" class="rv-primary" :disabled="annotationCardSaving" @click="submitAnnotationCard">
            {{ annotationCardSaving ? $t('reading.saving') : $t('reading.annotationCardSave') }}
          </button>
        </div>
      </div>
    </t-dialog>

    <div v-if="evidenceFloat.visible" class="rv-evidence-float"
      :style="{ top: evidenceFloat.top + 'px', left: evidenceFloat.left + 'px' }"
      @mouseenter="cancelEvidenceClose" @mouseleave="hideEvidenceFloat">
      <div class="rv-evidence-float__title">{{ evidenceFloat.title }}</div>
      <div v-if="evidenceFloat.loading" class="rv-evidence-float__muted">{{ $t('reading.loadingOrigin') }}</div>
      <div v-else-if="evidenceFloat.error" class="rv-evidence-float__error">{{ evidenceFloat.error }}</div>
      <div v-else class="rv-evidence-float__body">
        <span v-for="(part, index) in evidenceFloatParts" :key="index"
          :class="{ 'is-hit': part.hit }">{{ part.text }}</span>
      </div>
      <div class="rv-evidence-float__hint">{{ $t('reading.clickToOpen') }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch, type Directive } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import { createSessions } from '@/api/chat'
import { getChunkByIdOnly, listKnowledgeBases, listKnowledgeFiles, listKnowledgeFolders, type KnowledgeFolderNode } from '@/api/knowledge-base'
import {
  abandonReadingRun,
  addReadingDocuments,
  createCardFromReadingAnnotation,
  createReadingAnnotation,
  createReadingDraft,
  createReadingRun,
  deleteReadingAnnotation,
  deleteReadingDraft,
  deleteReadingReport,
  exportReadingRun,
  feedbackReadingRun,
  getReadingReport,
  getReadingRun,
  getReadingRunDocuments,
  importReadingSelfTests,
  judgeReadingQuestion,
  jumpReadingSection,
  listReadingDrafts,
  listReadingFeynman,
  listReadingSelfTests,
  resolveReadingContradiction,
  resumeReadingRun,
  saveReadingReport,
  stepReadingRun,
  streamReadingStep,
  stopReadingRun,
  submitReadingFeynman,
  updateReadingConclusion,
  updateReadingDraft,
  updateReadingTerm,
  updateReadingUnderstanding,
  type ReadingAnnotation,
  type ReadingConclusion,
  type ReadingContradiction,
  type ReadingDepth,
  type ReadingDocument,
  type ReadingDraft as ApiReadingDraft,
  type ReadingEvidence,
  type ReadingFeynmanAttempt,
  type ReadingHistoryItem,
  type ReadingProgressEvent,
  type ReadingQuestionNode,
  type ReadingRun,
  type ReadingRunStep,
  type ReadingSelfTest,
  type ReadingSectionStructure,
} from '@/api/reading'
import { listReviewGroups, type ReviewGroupNode } from '@/api/review'
import ReadingThoughtChain from '@/components/ReadingThoughtChain.vue'
import ReadingHistoryDrawer from './ReadingHistoryDrawer.vue'
import DocumentPreview from '@/components/document-preview.vue'
import { useShellUiStore } from '@/stores/shellUi'
import { openMermaidFullscreen } from '@/utils/mermaidViewer'
import { renderMermaidToSvg } from '@/utils/mermaidShared'
import { renderDocumentPreviewMarkdown } from '@/utils/documentPreviewMarkdown'
import { hydrateProtectedFileImages } from '@/utils/security'

type ReadingDocOption = {
  value: string
  label: string
  kbId: string
  kbName: string
  folderId: string
  fileSize: number
  sizeLabel: string
}

type DocTreeRow = {
  key: string
  kind: 'kb' | 'folder' | 'doc'
  id: string
  label: string
  depth: number
  count?: number
  expanded?: boolean
  doc?: ReadingDocOption
}

type OutlineNode = {
  key: string
  label: string
  level: number
  sectionIds: string[]
  children: OutlineNode[]
}

type OutlineRow = {
  key: string
  label: string
  depth: number
  kind: 'doc' | 'heading'
  hasChildren: boolean
  expanded: boolean
  sectionIds: string[]
  current: boolean
  preview: boolean
  read: boolean
}

type ReadingDraft = {
  id: string
  name: string
  documentIds: string[]
  depth: ReadingDepth
  goal: string
  updatedAt: string
}

type AnswerOption = {
  id: string
  label: string
  text: string
  recommended: boolean
}

type NoteItem = {
  key: string
  type: 'term' | 'conclusion' | 'open' | 'contradiction'
  typeLabel: string
  status: string
  statusLabel: string
  text: string
  cite: string
  recommended: boolean
  actions: boolean
  raw: Record<string, any>
}

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const shellUi = useShellUiStore()
const kbId = ref('')
const kbOptions = ref<Array<{ value: string; label: string }>>([])
const documentIds = ref<string[]>([])
const documentOptions = ref<ReadingDocOption[]>([])
const kbSearch = ref('')
const docSearch = ref('')
const addDocSearch = ref('')
const goal = ref('')
const depth = ref<ReadingDepth>('standard')
const run = ref<ReadingRun | null>(null)
const feedback = ref('')
const creating = ref(false)
const acting = ref(false)
const advancing = ref(false)

interface LiveReadingProgress {
  phase: string
  status: string
  sectionTitle: string
  sectionHeading: string
  round: number
  searches: Array<{ query: string; hitCount: number }>
  reasoning: string
}

const liveProgress = ref<LiveReadingProgress | null>(null)
const liveElapsedMs = ref(0)
let liveTimer: number | null = null
let activeReadingStream: { close: () => void } | null = null
const liveElapsedLabel = computed(() => {
  const seconds = Math.max(0, Math.floor(liveElapsedMs.value / 1000))
  const minutes = Math.floor(seconds / 60)
  return `${String(minutes).padStart(2, '0')}:${String(seconds % 60).padStart(2, '0')}`
})
const exporting = ref(false)
const planCollapsed = ref(false)
const notesCollapsed = ref(false)
const addDocDialogVisible = ref(false)
const addDocOptions = ref<ReadingDocOption[]>([])
const addDocIds = ref<string[]>([])
const addingDocs = ref(false)
const readingBodyRef = ref<HTMLElement | null>(null)
const readingDocuments = ref<ReadingDocument[]>([])
const readingDocumentsLoading = ref(false)
const activeDocumentId = ref('')
const documentPaneRef = ref<HTMLElement | null>(null)
const documentCurrentRef = ref<HTMLElement | null>(null)
const selectedTraceRound = ref(0)
const DEFAULT_RIGHT_WIDTH = 312
const MIN_RIGHT_WIDTH = 260
const leftWidth = ref(loadStoredLeftWidth())
const rightWidth = ref(loadStoredRightWidth())
const storageKey = 'gaze_active_reading_run_id'
const configView = ref(false)
const historyOpen = ref(false)
const mobileLeftOpen = ref(false)
const mobileRightOpen = ref(false)
const previewSectionId = ref('')
const originalDrawerOpen = ref(false)
const originalSectionId = ref('')
const drawerWidth = ref(loadStoredDrawerWidth())
const drafts = ref<ReadingDraft[]>([])
const draftsExpanded = ref(true)
const renamingDraftId = ref('')
const renameDraftValue = ref('')
const goalTextareaRef = ref<HTMLTextAreaElement | null>(null)
const folderTrees = ref<Record<string, KnowledgeFolderNode[]>>({})
const expandedTreeNodes = ref<Record<string, boolean>>({})
const outlineExpanded = ref<Record<string, boolean>>({})

const inlineAnswerSubmitted = ref(false)
const answeredQuestionSnapshot = ref<ReadingQuestionNode | null>(null)
const selectedAnswerId = ref('')
const customAnswerMode = ref(false)
const answerDraft = ref('')
const selfTests = ref<ReadingSelfTest[]>([])
const selfTestsLoading = ref(false)
const selectedSelfTestIds = ref<string[]>([])
const reviewGroupOptions = ref<Array<{ label: string; value: string }>>([])
const selectedReviewGroupId = ref('')
const importingSelfTests = ref(false)
const selfTestLoadedRunId = ref('')
const answering = ref(false)
const answerTextareaRef = ref<HTMLTextAreaElement | null>(null)

const stageDecision = ref<'' | 'confirmed' | 'corrected' | 'skipped'>('')
const stageCorrecting = ref(false)
const stageCorrection = ref('')

const editingNoteKey = ref('')
const editingNoteText = ref('')
const readingPageRef = ref<HTMLElement | null>(null)
const pageNarrow = ref(false)

let autoFullscreenApplied = false
let readingPageObserver: ResizeObserver | null = null

const currentSection = computed(() => {
  const sections = run.value?.plan.sections || []
  return sections.find((section) => section.id === run.value?.current_section_id)
})

const currentQuestion = computed(() => {
  const steps = run.value?.steps || []
  for (let i = steps.length - 1; i >= 0; i -= 1) {
    if (steps[i].action === 'ask_user' && !steps[i].user_feedback) {
      return steps[i].question || null
    }
  }
  return null
})

const isStructured = computed(() => Number(run.value?.plan?.state_version || 0) >= 2)
const activeTab = ref<'all' | 'terms' | 'conclusions' | 'open' | 'contradictions'>('all')
const annotations = computed(() => run.value?.annotations || [])
const feynmanVisible = ref(false)
const feynmanSectionId = ref('')
const feynmanAnswers = ref<Record<string, string>>({ macro: '', mechanism: '', detail: '', application: '' })
const feynmanFreeform = ref('')
const feynmanSubmitting = ref(false)
const feynmanHistoryLoading = ref(false)
const feynmanAttempts = ref<ReadingFeynmanAttempt[]>([])
const feynmanAngleDefs = computed(() => [
  { id: 'macro', label: t('reading.feynmanAngleMacro'), prompt: t('reading.feynmanAngleMacroHint') },
  { id: 'mechanism', label: t('reading.feynmanAngleMechanism'), prompt: t('reading.feynmanAngleMechanismHint') },
  { id: 'detail', label: t('reading.feynmanAngleDetail'), prompt: t('reading.feynmanAngleDetailHint') },
  { id: 'application', label: t('reading.feynmanAngleApplication'), prompt: t('reading.feynmanAngleApplicationHint') },
])
const feynmanFilled = computed(() =>
  Object.values(feynmanAnswers.value).some((value) => value.trim().length >= 20) || feynmanFreeform.value.trim().length >= 20,
)
const annotationDialogVisible = ref(false)
const annotationQuote = ref('')
const annotationComment = ref('')
const annotationSaving = ref(false)
const previewSelectionText = ref('')
const annotationCardVisible = ref(false)
const annotationCardTarget = ref<ReadingAnnotation | null>(null)
const annotationCardGroupId = ref('')
const annotationCardQuestion = ref('')
const annotationCardAnswer = ref('')
const annotationCardSaving = ref(false)

interface AnnotationPopoverState { id: string; top: number; left: number }
const annotationPopover = ref<AnnotationPopoverState | null>(null)
const annotationPopoverTarget = computed(() =>
  annotationPopover.value ? annotations.value.find((item) => item.id === annotationPopover.value?.id) || null : null,
)
let annotationHighlightTimers: number[] = []

function clearAnnotationHighlights() {
  const root = documentPaneRef.value
  if (!root) return
  root.querySelectorAll('.rv-annotation-pin').forEach((pin) => pin.remove())
  root.querySelectorAll('mark.rv-annotation-highlight').forEach((mark) => {
    const parent = mark.parentNode
    if (!parent) return
    mark.replaceWith(...Array.from(mark.childNodes))
    parent.normalize()
  })
}

function collectAnnotationTextNodes(root: HTMLElement): Text[] {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
    acceptNode(node) {
      const text = node.nodeValue || ''
      if (!text.trim()) return NodeFilter.FILTER_REJECT
      const parent = node.parentElement
      if (!parent || parent.closest('script, style, mark.rv-annotation-highlight, .rv-annotation-pin, .rv-annotation-popover')) {
        return NodeFilter.FILTER_REJECT
      }
      return NodeFilter.FILTER_ACCEPT
    },
  })
  const nodes: Text[] = []
  let current = walker.nextNode()
  while (current) {
    if (current instanceof Text) nodes.push(current)
    current = walker.nextNode()
  }
  return nodes
}

function applyAnnotationHighlights() {
  const root = documentPaneRef.value
  if (!root) return
  clearAnnotationHighlights()
  if (!annotations.value.length) return
  for (const annotation of annotations.value) {
    const quote = annotation.quote.replace(/\s+/g, ' ').trim()
    if (!quote) continue
    const textNodes = collectAnnotationTextNodes(root)
    if (!textNodes.length) continue
    let normalized = ''
    const starts: Array<{ node: Text; offset: number }> = []
    const ends: Array<{ node: Text; offset: number }> = []
    for (const node of textNodes) {
      const data = node.nodeValue || ''
      for (let i = 0; i < data.length; i += 1) {
        const ch = data[i]
        if (/\s/.test(ch)) {
          if (normalized.endsWith(' ')) continue
          normalized += ' '
        } else {
          normalized += ch
        }
        starts.push({ node, offset: i })
        ends.push({ node, offset: i + 1 })
      }
    }
    const index = normalized.indexOf(quote)
    if (index < 0) continue
    const start = starts[index]
    const end = ends[index + quote.length - 1]
    if (!start || !end) continue
    const range = document.createRange()
    range.setStart(start.node, start.offset)
    range.setEnd(end.node, end.offset)
    const intersecting = textNodes.filter((node) => range.intersectsNode(node))
    let lastMark: HTMLElement | null = null
    for (let i = intersecting.length - 1; i >= 0; i -= 1) {
      const node = intersecting[i]
      const localStart = node === start.node ? start.offset : 0
      const localEnd = node === end.node ? end.offset : (node.nodeValue || '').length
      if (localEnd <= localStart) continue
      let target: Text = node
      if (localStart > 0) target = node.splitText(localStart) as Text
      const remaining = localEnd - localStart
      if (remaining < (target.nodeValue || '').length) target.splitText(remaining)
      const mark = document.createElement('mark')
      mark.className = 'rv-annotation-highlight'
      mark.dataset.annotationId = annotation.id
      mark.title = annotation.comment || annotation.quote
      target.parentNode?.insertBefore(mark, target)
      mark.appendChild(target)
      if (node === end.node) lastMark = mark
    }
    if (lastMark) {
      const pin = document.createElement('button')
      pin.type = 'button'
      pin.className = 'rv-annotation-pin'
      pin.dataset.annotationId = annotation.id
      pin.title = annotation.comment || annotation.quote
      pin.setAttribute('aria-label', t('reading.annotations'))
      pin.innerHTML = '<svg viewBox="0 0 16 16" width="11" height="11" fill="none" aria-hidden="true"><path d="M3 2.5h10v7l-3 3H3z" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/><path d="M10 12.5v-3h3" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/></svg>'
      lastMark.parentNode?.insertBefore(pin, lastMark.nextSibling)
    }
  }
}

function scheduleAnnotationHighlights(delays: number[] = [0, 80, 250, 600]) {
  annotationHighlightTimers.forEach((timer) => window.clearTimeout(timer))
  annotationHighlightTimers = []
  if (!originalDrawerOpen.value) return
  for (const delay of delays) {
    annotationHighlightTimers.push(window.setTimeout(() => {
      void nextTick(applyAnnotationHighlights)
    }, delay))
  }
}

let popoverTrackHandler: (() => void) | null = null
let popoverTrackRaf = 0

function stopAnnotationPopoverTracking() {
  if (popoverTrackHandler) {
    document.removeEventListener('scroll', popoverTrackHandler, true)
    window.removeEventListener('resize', popoverTrackHandler)
    popoverTrackHandler = null
  }
  if (popoverTrackRaf) {
    window.cancelAnimationFrame(popoverTrackRaf)
    popoverTrackRaf = 0
  }
}

function positionAnnotationPopover(id: string) {
  const root = documentPaneRef.value
  if (!root) {
    annotationPopover.value = null
    return
  }
  const anchor = Array.from(root.querySelectorAll<HTMLElement>('[data-annotation-id]')).find(
    (el) => el.dataset.annotationId === id,
  )
  if (!anchor) {
    annotationPopover.value = null
    return
  }
  const rect = anchor.getBoundingClientRect()
  const bodyRect = root.getBoundingClientRect()
  if (rect.bottom < bodyRect.top + 12 || rect.top > bodyRect.bottom - 12) {
    annotationPopover.value = null
    return
  }
  const width = 300
  const height = 150
  annotationPopover.value = {
    id,
    top: Math.max(8, Math.min(rect.bottom + 8, window.innerHeight - height)),
    left: Math.max(8, Math.min(rect.left, window.innerWidth - width - 16)),
  }
}

function startAnnotationPopoverTracking(id: string) {
  stopAnnotationPopoverTracking()
  popoverTrackHandler = () => {
    if (popoverTrackRaf) return
    popoverTrackRaf = window.requestAnimationFrame(() => {
      popoverTrackRaf = 0
      positionAnnotationPopover(id)
    })
  }
  document.addEventListener('scroll', popoverTrackHandler, true)
  window.addEventListener('resize', popoverTrackHandler)
}

watch(annotationPopover, (value) => {
  if (!value) stopAnnotationPopoverTracking()
})

function handleAnnotationClick(event: MouseEvent) {
  const target = (event.target as HTMLElement | null)?.closest?.('[data-annotation-id]') as HTMLElement | null
  if (!target) {
    annotationPopover.value = null
    return
  }
  const id = target.dataset.annotationId || ''
  if (!annotations.value.some((item) => item.id === id)) return
  event.preventDefault()
  event.stopPropagation()
  positionAnnotationPopover(id)
  startAnnotationPopoverTracking(id)
}

function openAnnotationFromPopover(annotation: ReadingAnnotation) {
  annotationPopover.value = null
  openAnnotationSource(annotation)
}

function openAnnotationCardFromPopover(annotation: ReadingAnnotation) {
  annotationPopover.value = null
  void openAnnotationCard(annotation)
}

function removeAnnotationFromPopover(annotation: ReadingAnnotation) {
  annotationPopover.value = null
  removeAnnotation(annotation)
}

watch(originalDrawerOpen, async (open) => {
  annotationPopover.value = null
  if (open) {
    await nextTick()
    scheduleAnnotationHighlights()
  } else {
    annotationHighlightTimers.forEach((timer) => window.clearTimeout(timer))
    annotationHighlightTimers = []
  }
})

watch(activeDocumentId, () => {
  annotationPopover.value = null
  if (originalDrawerOpen.value) scheduleAnnotationHighlights()
})

watch(
  () => annotations.value.map((item) => item.id).join('|'),
  () => {
    if (originalDrawerOpen.value) scheduleAnnotationHighlights([0, 60])
  },
)
const structuredTerms = computed(() => run.value?.terms || [])
const structuredContradictions = computed(() => run.value?.contradictions || [])
const structuredConclusions = computed(() => (run.value?.conclusions || []).slice().reverse())
const confirmedConclusions = computed(() =>
  (run.value?.conclusions || []).filter((item) => item.status === 'confirmed').slice().reverse(),
)
const openQuestions = computed(() =>
  (run.value?.questions || []).filter((item) =>
    ['pending', 'contradicted', 'skipped'].includes(item.status),
  ),
)
const currentPendingQuestion = computed<ReadingQuestionNode | null>(() =>
  (run.value?.questions || []).find((item) => item.status === 'pending') || null,
)
const pendingQueueCount = computed(() => {
  const pending = (run.value?.questions || []).filter((item) => item.status === 'pending').length
  return Math.max(0, pending - 1)
})
const currentRound = computed(() => {
  const steps = run.value?.steps || []
  for (let i = steps.length - 1; i >= 0; i -= 1) {
    if (steps[i].action === 'round' || steps[i].action === 'ask_user') return steps[i].seq
  }
  const questions = run.value?.questions || []
  const questionRound = questions.reduce((max, item) => Math.max(max, item.round), 0)
  return Math.max(questionRound, run.value?.step_count || 0)
})
const roundAutoQuestions = computed(() =>
  (run.value?.questions || []).filter((item) =>
    item.round === currentRound.value && ['auto_answered', 'contradicted'].includes(item.status),
  ),
)
const currentUnderstanding = computed(() => {
  const steps = run.value?.steps || []
  for (let i = steps.length - 1; i >= 0; i -= 1) {
    const value = steps[i]?.notes_patch?.understanding
    if (typeof value === 'string' && value.trim()) return value
  }
  return ''
})
const currentUnderstandingRecord = computed(() => {
  const items = run.value?.understandings || []
  if (!items.length) return null
  for (let i = items.length - 1; i >= 0; i -= 1) {
    if (items[i].round === currentRound.value) return items[i]
  }
  return items[items.length - 1]
})
const structuredTabs = computed(() => [
  { key: 'terms' as const, label: t('reading.tabs.terms'), count: structuredTerms.value.length },
  { key: 'conclusions' as const, label: t('reading.tabs.conclusions'), count: structuredConclusions.value.length },
  { key: 'open' as const, label: t('reading.tabs.open'), count: openQuestions.value.length },
  { key: 'contradictions' as const, label: t('reading.tabs.contradictions'), count: structuredContradictions.value.length },
])
const pauseEachRound = computed(() =>
  isStructured.value && (run.value?.plan.depth || 'standard') !== 'quick',
)
const canStartNewReading = computed(() =>
  ['finished', 'stopped', 'failed', 'abandoned'].includes(String(run.value?.status || '')),
)

const showConfigView = computed(() => !run.value || configView.value)
const canPauseRun = computed(() =>
  ['planning', 'reading', 'waiting_user'].includes(String(run.value?.status || '')),
)
const totalSectionCount = computed(() => run.value?.plan.sections?.length || 0)
const readPercent = computed(() => {
  if (!totalSectionCount.value) return 0
  return Math.round((readSectionCount.value / totalSectionCount.value) * 100)
})
const remainingMinutes = computed(() => {
  const remaining = Math.max(0, totalSectionCount.value - readSectionCount.value)
  const factor = run.value?.plan.depth === 'deep' ? 3 : run.value?.plan.depth === 'quick' ? 1 : 2
  return Math.max(1, Math.round(remaining * factor))
})
const workbenchTitle = computed(() =>
  activeDocument.value?.title || currentSection.value?.title || t('reading.title'),
)
const workbenchMeta = computed(() => {
  if (!run.value) return ''
  return depthLabel.value
})
const exportOptions = computed(() => [
  { content: t('reading.exportMarkdown'), value: 'markdown' },
  { content: t('reading.exportJSON'), value: 'json' },
])

const previewSectionData = computed(() =>
  (run.value?.plan.sections || []).find((section) => section.id === previewSectionId.value) || null,
)
const viewingSection = computed(() => previewSectionData.value || currentSection.value)
const viewingSectionTitle = computed(() => {
  const heading = cleanHeading(viewingSection.value?.heading)
  return heading || viewingSection.value?.title || t('reading.plan')
})
const traceSectionHeading = computed(() => {
  const heading = cleanHeading(traceSection.value?.heading)
  if (!heading) return ''
  return documentTitle(traceSection.value?.title || '') === documentTitle(heading) ? '' : heading
})
const stageSectionLabel = computed(() =>
  cleanHeading(currentSection.value?.heading) || currentSection.value?.title || '',
)
const viewingSectionMeta = computed(() => {
  if (!run.value) return ''
  if (previewSectionId.value && previewSectionId.value !== run.value.current_section_id) {
    return t('reading.previewMeta')
  }
  return t('reading.roundLabel', { round: currentRound.value })
})
const previewConclusions = computed(() =>
  (run.value?.conclusions || []).filter((item) => item.section_id === previewSectionData.value?.id),
)

const outlineDocuments = computed<OutlineNode[]>(() => {
  const sections = run.value?.plan.sections || []
  const roots: OutlineNode[] = []
  const rootByTitle = new Map<string, OutlineNode>()
  const stacks = new Map<string, OutlineNode[]>()

  for (const section of sections) {
    const title = section.title || section.heading || section.id
    const docBase = documentTitle(title)
    let root = rootByTitle.get(docBase)
    if (!root) {
      root = { key: `doc:${docBase}`, label: docBase, level: 0, sectionIds: [], children: [] }
      rootByTitle.set(docBase, root)
      roots.push(root)
    }

    const segments = headingSegments(section.heading)
      .filter((segment, index) =>
        !(index === 0 && segment.level === 1 && documentTitle(segment.text) === docBase),
      )
    if (!segments.length) {
      if (!root.sectionIds.includes(section.id)) root.sectionIds.push(section.id)
      continue
    }

    const previousStack = stacks.get(docBase) || []
    const stack = previousStack.filter((node) => node.level < segments[0].level)
    for (const segment of segments) {
      while (stack.length && stack[stack.length - 1].level >= segment.level) stack.pop()
      const parent = stack[stack.length - 1] || root
      let node = parent.children.find((child) =>
        child.label === segment.text && child.level === segment.level,
      )
      if (!node) {
        node = {
          key: `${parent.key}>${segment.level}:${segment.text}`,
          label: segment.text,
          level: segment.level,
          sectionIds: [],
          children: [],
        }
        parent.children.push(node)
      }
      if (!node.sectionIds.includes(section.id)) node.sectionIds.push(section.id)
      stack.push(node)
    }
    stacks.set(docBase, stack)
  }
  return roots
})

const outlineRows = computed<OutlineRow[]>(() => {
  const sections = run.value?.plan.sections || []
  const readById = new Map(sections.map((section) => [section.id, Boolean(section.read)]))
  const rows: OutlineRow[] = []
  const walk = (node: OutlineNode, depth: number, root: boolean) => {
    const expanded = root
      ? outlineExpanded.value[node.key] !== false
      : Boolean(outlineExpanded.value[node.key])
    rows.push({
      key: node.key,
      label: node.label,
      depth,
      kind: root ? 'doc' : 'heading',
      hasChildren: node.children.length > 0,
      expanded,
      sectionIds: node.sectionIds,
      current: node.sectionIds.includes(String(run.value?.current_section_id || '')),
      preview: node.sectionIds.includes(previewSectionId.value),
      read: node.sectionIds.length > 0 && node.sectionIds.every((id) => readById.get(id)),
    })
    if (expanded) node.children.forEach((child) => walk(child, depth + 1, false))
  }
  outlineDocuments.value.forEach((root) => walk(root, 0, true))
  return rows
})

function toggleOutlineRow(row: OutlineRow) {
  const current = row.kind === 'doc'
    ? outlineExpanded.value[row.key] !== false
    : Boolean(outlineExpanded.value[row.key])
  outlineExpanded.value = { ...outlineExpanded.value, [row.key]: !current }
}

function handleOutlineRow(row: OutlineRow) {
  if (row.kind === 'doc') {
    if (row.hasChildren) toggleOutlineRow(row)
    return
  }
  const sectionId = row.sectionIds.includes(String(run.value?.current_section_id || ''))
    ? String(run.value?.current_section_id || '')
    : row.sectionIds[0]
  if (sectionId) previewSection({ id: sectionId })
}
const roundConclusions = computed(() => {
  const sectionId = viewingSection.value?.id
  const list = (run.value?.conclusions || []).filter((item) => {
    if (sectionId && item.section_id !== sectionId) return false
    return item.round === currentRound.value
  })
  return list.length ? list : (run.value?.conclusions || []).filter((item) => item.section_id === sectionId).slice(-3)
})
const openContradictions = computed(() => structuredContradictions.value)

const filteredAllDocuments = computed(() => {
  const q = docSearch.value.trim().toLowerCase()
  if (!q) return documentOptions.value
  return documentOptions.value.filter((doc) =>
    `${doc.label} ${doc.kbName}`.toLowerCase().includes(q),
  )
})
const docSearchActive = computed(() => docSearch.value.trim().length > 0)
const selectedDocuments = computed(() =>
  documentIds.value
    .map((id) => documentOptions.value.find((doc) => doc.value === id))
    .filter((doc): doc is ReadingDocOption => Boolean(doc)),
)

function docsInFolder(kbId: string, folderId: string) {
  return documentOptions.value.filter((doc) => doc.kbId === kbId && doc.folderId === folderId)
}

const documentTreeRows = computed<DocTreeRow[]>(() => {
  if (docSearchActive.value) return []
  const rows: DocTreeRow[] = []
  for (const base of kbOptions.value) {
    const docs = documentOptions.value.filter((doc) => doc.kbId === base.value)
    const kbKey = `kb:${base.value}`
    const kbExpanded = expandedTreeNodes.value[kbKey] !== false
    rows.push({
      key: kbKey,
      kind: 'kb',
      id: base.value,
      label: base.label,
      depth: 0,
      count: docs.length,
      expanded: kbExpanded,
    })
    if (!kbExpanded) continue

    const folderIds = new Set<string>()
    const collectFolderIds = (nodes: KnowledgeFolderNode[]) => {
      for (const node of nodes) {
        if (node.id) folderIds.add(String(node.id))
        collectFolderIds(node.children || [])
      }
    }
    collectFolderIds(folderTrees.value[base.value] || [])

    const pushDoc = (doc: ReadingDocOption, depth: number) => {
      rows.push({
        key: `doc:${base.value}:${doc.value}`,
        kind: 'doc',
        id: doc.value,
        label: doc.label,
        depth,
        doc,
      })
    }

    const walk = (nodes: KnowledgeFolderNode[], depth: number) => {
      for (const node of nodes) {
        const key = `folder:${base.value}:${node.id || node.path}`
        const expanded = Boolean(expandedTreeNodes.value[key])
        rows.push({
          key,
          kind: 'folder',
          id: String(node.id || node.path),
          label: node.name,
          depth,
          count: node.total_count,
          expanded,
        })
        if (!expanded) continue
        walk(node.children || [], depth + 1)
        docsInFolder(base.value, String(node.id || '')).forEach((doc) => pushDoc(doc, depth + 1))
      }
    }

    walk(folderTrees.value[base.value] || [], 1)
    docs
      .filter((doc) => !doc.folderId || !folderIds.has(doc.folderId))
      .forEach((doc) => pushDoc(doc, 1))
  }
  return rows
})

function handleTreeRow(row: DocTreeRow) {
  if (row.kind === 'doc' && row.doc) {
    toggleDocument(row.doc.value)
    return
  }
  const current = row.kind === 'kb'
    ? expandedTreeNodes.value[row.key] !== false
    : Boolean(expandedTreeNodes.value[row.key])
  expandedTreeNodes.value = { ...expandedTreeNodes.value, [row.key]: !current }
}

const filteredAddDocOptions = computed(() => {
  const q = addDocSearch.value.trim().toLowerCase()
  if (!q) return addDocOptions.value
  return addDocOptions.value.filter((doc) =>
    `${doc.label} ${doc.kbName}`.toLowerCase().includes(q),
  )
})

const depthOptions = computed(() => [
  { value: 'quick' as ReadingDepth, label: t('reading.depth.quick'), hint: t('reading.depthHint.quick') },
  { value: 'standard' as ReadingDepth, label: t('reading.depth.standard'), hint: t('reading.depthHint.standard') },
  { value: 'deep' as ReadingDepth, label: t('reading.depth.deep'), hint: t('reading.depthHint.deep') },
])
const goalChips = computed(() => [
  { key: 'summary', label: t('reading.goalChips.summary'), value: t('reading.goalChips.summary') },
  { key: 'contradiction', label: t('reading.goalChips.contradiction'), value: t('reading.goalChips.contradiction') },
  { key: 'terms', label: t('reading.goalChips.terms'), value: t('reading.goalChips.terms') },
  { key: 'pitfalls', label: t('reading.goalChips.pitfalls'), value: t('reading.goalChips.pitfalls') },
  { key: 'custom', label: t('reading.goalChips.custom'), value: '' },
])
const estimate = computed(() => {
  const docs = documentOptions.value.filter((doc) => documentIds.value.includes(doc.value))
  const chars = docs.reduce((sum, doc) => sum + Math.max(4000, Math.round((doc.fileSize || 0) / 3)), 0)
  const sections = Math.max(1, Math.ceil(chars / 3200))
  const roundsPerSection = depth.value === 'deep' ? 6 : depth.value === 'quick' ? 1 : 3
  const rounds = sections * roundsPerSection
  return {
    rounds,
    minutes: Math.max(5, Math.round(rounds * 0.25)),
    tokens: Math.max(2, Math.round((rounds * 300) / 1000)),
    questions: depth.value === 'deep' ? '2–4' : depth.value === 'quick' ? '0' : '1–2',
  }
})

const noteTotalCount = computed(() =>
  structuredTerms.value.length + structuredConclusions.value.length + openQuestions.value.length + structuredContradictions.value.length,
)
const skippedQuestionCount = computed(() =>
  (run.value?.questions || []).filter((item) => item.status === 'skipped').length,
)
const noteTabs = computed(() => [
  { key: 'all' as const, label: t('reading.tabs.all'), count: noteTotalCount.value },
  { key: 'terms' as const, label: t('reading.tabs.terms'), count: structuredTerms.value.length },
  { key: 'conclusions' as const, label: t('reading.tabs.conclusions'), count: structuredConclusions.value.length },
  { key: 'open' as const, label: t('reading.tabs.open'), count: openQuestions.value.length },
  { key: 'contradictions' as const, label: t('reading.tabs.contradictions'), count: structuredContradictions.value.length },
])
const visibleNotes = computed<NoteItem[]>(() => {
  const notes: NoteItem[] = []
  for (const term of structuredTerms.value) {
    notes.push({
      key: `term-${term.id}`,
      type: 'term',
      typeLabel: t('reading.tabs.terms'),
      status: term.status,
      statusLabel: term.status === 'confirmed'
        ? t('reading.confirmedTag')
        : term.status === 'rejected'
          ? t('reading.rejectedTag')
          : t('reading.pendingConfirm'),
      text: `${term.term}：${term.definition}`,
      cite: term.source?.[0] ? evidenceLabel(term.source[0]) : '',
      recommended: Boolean(term.canonical),
      actions: true,
      raw: term as unknown as Record<string, any>,
    })
  }
  for (const conclusion of structuredConclusions.value) {
    notes.push({
      key: `conclusion-${conclusion.id}`,
      type: 'conclusion',
      typeLabel: t('reading.tabs.conclusions'),
      status: conclusion.status,
      statusLabel: conclusionStatusLabel(conclusion),
      text: conclusion.text,
      cite: conclusion.evidence?.[0] ? evidenceLabel(conclusion.evidence[0]) : '',
      recommended: conclusion.source === 'user',
      actions: true,
      raw: conclusion as unknown as Record<string, any>,
    })
  }
  for (const question of openQuestions.value) {
    notes.push({
      key: `open-${question.id}`,
      type: 'open',
      typeLabel: t('reading.tabs.open'),
      status: question.status,
      statusLabel: questionStatusLabel(question),
      text: question.text,
      cite: question.evidence?.[0] ? evidenceLabel(question.evidence[0]) : '',
      recommended: Boolean(question.recommended_answer),
      actions: false,
      raw: question as unknown as Record<string, any>,
    })
  }
  for (const item of structuredContradictions.value) {
    notes.push({
      key: `contra-${item.id}`,
      type: 'contradiction',
      typeLabel: t('reading.tabs.contradictions'),
      status: item.status,
      statusLabel: item.status === 'open' ? t('reading.contradictionOpen') : t('reading.contradictionResolved'),
      text: item.description,
      cite: '',
      recommended: false,
      actions: false,
      raw: item as unknown as Record<string, any>,
    })
  }
  if (activeTab.value === 'all') return notes
  const typeMap = {
    terms: 'term',
    conclusions: 'conclusion',
    open: 'open',
    contradictions: 'contradiction',
  } as const
  return notes.filter((note) => note.type === typeMap[activeTab.value as keyof typeof typeMap])
})

const answerOptions = computed<AnswerOption[]>(() => {
  const question = currentPendingQuestion.value as (ReadingQuestionNode & {
    options?: Array<{ id?: string; label?: string; text?: string; recommended?: boolean }>
  }) | null
  const raw = question?.options
  if (Array.isArray(raw) && raw.length) {
    return raw.map((option, index) => ({
      id: option.id || String(index),
      label: option.label || t('reading.optionLabel', { index: index + 1 }),
      text: option.text || '',
      recommended: Boolean(option.recommended),
    }))
  }
  if (question?.recommended_answer) {
    return [{
      id: '0',
      label: t('reading.optionLabel', { index: 1 }),
      text: question.recommended_answer,
      recommended: true,
    }]
  }
  return []
})
const answerReady = computed(() =>
  customAnswerMode.value ? answerDraft.value.trim().length > 0 : Boolean(selectedAnswerId.value),
)
const questionCard = computed(() =>
  inlineAnswerSubmitted.value ? answeredQuestionSnapshot.value : currentPendingQuestion.value,
)
const questionJudgmentLabel = computed(() => {
  const judgment = questionCard.value?.judgment
  return judgment ? t(`reading.judgment.${judgment}`) : ''
})

const activeDocument = computed(() => {
  const documents = readingDocuments.value
  return documents.find((doc) => doc.id === activeDocumentId.value) || documents[0] || null
})
const activeDocumentKnowledgeBaseID = computed(() => {
  const documentID = activeDocument.value?.id
  if (!documentID) return run.value?.plan.knowledge_base_id || ''
  return (run.value?.plan.sections || [])
    .find((section) => section.knowledge_id === documentID)?.knowledge_base_id
    || run.value?.plan.knowledge_base_id
    || ''
})
const originalFileBlob = computed(() =>
  activeDocument.value
    ? new Blob([activeDocument.value.content || ''], { type: 'text/markdown' })
    : undefined,
)
const originalFileName = computed(() =>
  activeDocument.value?.file_name || activeDocument.value?.title || 'document.md',
)
const drawerToolbarRef = ref<HTMLElement | null>(null)
const readingDocumentOptions = computed(() =>
  readingDocuments.value.map((doc) => ({
    value: doc.id,
    label: doc.title || doc.file_name || doc.id,
  })),
)
const readSectionCount = computed(() =>
  (run.value?.plan.sections || []).filter((section) => section.read).length,
)
const latestRoundStep = computed(() => {
  const steps = run.value?.steps || []
  for (let i = steps.length - 1; i >= 0; i -= 1) {
    if (steps[i].action === 'round' || steps[i].action === 'ask_user') return steps[i]
  }
  return null
})
const traceRoundSteps = computed(() =>
  (run.value?.steps || []).filter((step) => step.action === 'round' || step.action === 'ask_user'),
)
const traceRoundOptions = computed(() =>
  traceRoundSteps.value.map((step) => ({
    value: step.seq,
    label: t('reading.roundLabel', { round: step.seq }),
  })),
)
const selectedTraceStep = computed(() =>
  traceRoundSteps.value.find((step) => step.seq === selectedTraceRound.value) || latestRoundStep.value,
)
const currentRoundTrace = computed(() => selectedTraceStep.value?.notes_patch || {})
const roundQuestions = computed(() =>
  (run.value?.questions || []).filter((question) => question.round === selectedTraceStep.value?.seq),
)
const selectedTraceDurationLabel = computed(() => {
  const ms = Number(currentRoundTrace.value.duration_ms || 0)
  if (ms <= 0) return ''
  return t('reading.roundDuration', { seconds: Math.max(1, Math.round(ms / 1000)) })
})
const traceSection = computed(() =>
  (run.value?.plan.sections || []).find((section) => section.id === selectedTraceStep.value?.section_id) ||
  currentSection.value,
)
const currentPositionSection = computed(() =>
  traceSection.value ||
  (run.value?.plan.sections || []).find((section) => !section.read) ||
  (run.value?.plan.sections || [])[0] ||
  null,
)
const originalTargetSection = computed(() =>
  (run.value?.plan.sections || []).find((section) => section.id === originalSectionId.value) ||
  previewSectionData.value ||
  currentSection.value ||
  currentPositionSection.value,
)
const activeDocumentSection = computed(() => {
  const section = originalTargetSection.value
  if (!section || !activeDocument.value) return null
  return (activeDocument.value.sections || []).find((item) => item.id === section.id) || null
})
const documentRange = computed(() => {
  const section = activeDocumentSection.value
  const content = activeDocument.value?.content || ''
  if (!section) return null
  const start = Math.max(0, Math.min(Number(section.start || 0), content.length))
  const end = Math.max(start, Math.min(Number(section.end || 0), content.length))
  return end > start ? { start, end } : null
})
const documentBefore = computed(() =>
  documentRange.value ? (activeDocument.value?.content || '').slice(0, documentRange.value.start) : '',
)
const documentHighlight = computed(() =>
  documentRange.value
    ? (activeDocument.value?.content || '').slice(documentRange.value.start, documentRange.value.end)
    : '',
)
const documentAfter = computed(() =>
  documentRange.value ? (activeDocument.value?.content || '').slice(documentRange.value.end) : '',
)

async function scrollToDocumentSection() {
  await nextTick()
  const pane = documentPaneRef.value
  const mark = documentCurrentRef.value
  if (!pane || !mark) return
  const paneRect = pane.getBoundingClientRect()
  const markRect = mark.getBoundingClientRect()
  if (markRect.top >= paneRect.top && markRect.bottom <= paneRect.bottom) return
  pane.scrollTop += markRect.top - paneRect.top - 60
}

watch(
  [() => activeDocumentId.value, () => currentPositionSection.value?.id],
  () => {
    void scrollToDocumentSection()
  },
)

watch(
  latestRoundStep,
  (step, previous) => {
    if (!step) return
    if (!selectedTraceRound.value || selectedTraceRound.value === previous?.seq) {
      selectedTraceRound.value = step.seq
    }
  },
  { immediate: true },
)

const DEFAULT_LEFT_WIDTH = 320
const MIN_LEFT_WIDTH = 220
const MAX_LEFT_WIDTH = 640

function clampLeftWidth(value: number) {
  const max = Math.min(MAX_LEFT_WIDTH, Math.round(window.innerWidth * 0.45))
  return Math.min(Math.max(MIN_LEFT_WIDTH, Math.round(value)), Math.max(MIN_LEFT_WIDTH, max))
}

function loadStoredLeftWidth() {
  const raw = Number(localStorage.getItem('gaze_reading_left_width') || '')
  return Number.isFinite(raw) && raw > 0 ? clampLeftWidth(raw) : DEFAULT_LEFT_WIDTH
}

let resizing = false
let resizeStartX = 0
let resizeStartWidth = 0
let resizeFrame = 0
let pendingLeftWidth = leftWidth.value

function setLeftWidth(value: number) {
  pendingLeftWidth = clampLeftWidth(value)
  if (resizeFrame) return
  resizeFrame = requestAnimationFrame(() => {
    resizeFrame = 0
    readingBodyRef.value?.style.setProperty('--reading-left-width', `${pendingLeftWidth}px`)
  })
}

function commitLeftWidth() {
  if (resizeFrame) {
    cancelAnimationFrame(resizeFrame)
    resizeFrame = 0
  }
  leftWidth.value = clampLeftWidth(pendingLeftWidth)
  readingBodyRef.value?.style.setProperty('--reading-left-width', `${leftWidth.value}px`)
  localStorage.setItem('gaze_reading_left_width', String(leftWidth.value))
}

function startResize(event: PointerEvent) {
  if (window.innerWidth <= 1180 || planCollapsed.value) return
  resizing = true
  resizeStartX = event.clientX
  resizeStartWidth = leftWidth.value
  pendingLeftWidth = leftWidth.value
  try {
    ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
  } catch {
    // Pointer capture is unavailable in some synthetic/headless events.
  }
  document.body.style.userSelect = 'none'
  document.body.style.cursor = 'col-resize'
  event.preventDefault()
}

function onResize(event: PointerEvent) {
  if (!resizing) return
  setLeftWidth(resizeStartWidth + event.clientX - resizeStartX)
}

function endResize(event: PointerEvent) {
  if (!resizing) return
  resizing = false
  commitLeftWidth()
  document.body.style.userSelect = ''
  document.body.style.cursor = ''
  try {
    ;(event.currentTarget as HTMLElement).releasePointerCapture(event.pointerId)
  } catch {
    // ignore
  }
}

function resetLeftWidth() {
  pendingLeftWidth = DEFAULT_LEFT_WIDTH
  commitLeftWidth()
}

function onResizeKeydown(event: KeyboardEvent) {
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
  event.preventDefault()
  pendingLeftWidth = leftWidth.value
  setLeftWidth(leftWidth.value + (event.key === 'ArrowLeft' ? -20 : 20))
  commitLeftWidth()
}

function onWindowResize() {
  const next = clampLeftWidth(leftWidth.value)
  if (next !== leftWidth.value) {
    leftWidth.value = next
    readingBodyRef.value?.style.setProperty('--reading-left-width', `${next}px`)
    localStorage.setItem('gaze_reading_left_width', String(next))
  }
  const nextRight = clampRightWidth(rightWidth.value)
  if (nextRight !== rightWidth.value) {
    rightWidth.value = nextRight
    localStorage.setItem('gaze_reading_right_width', String(nextRight))
  }
}

function clampRightWidth(value: number) {
  const bodyWidth = readingBodyRef.value?.clientWidth || window.innerWidth
  const max = Math.max(MIN_RIGHT_WIDTH, Math.floor(bodyWidth / 2))
  return Math.min(Math.max(MIN_RIGHT_WIDTH, Math.round(value)), max)
}

function loadStoredRightWidth() {
  const raw = Number(localStorage.getItem('gaze_reading_right_width') || '')
  if (Number.isFinite(raw) && raw > 0) return clampRightWidth(raw)
  return DEFAULT_RIGHT_WIDTH
}

let rightResizing = false
let rightResizeStartX = 0
let rightResizeStartWidth = 0

function startRightResize(event: PointerEvent) {
  if (pageNarrow.value) return
  rightResizing = true
  rightResizeStartX = event.clientX
  rightResizeStartWidth = rightWidth.value
  document.body.style.userSelect = 'none'
  document.body.style.cursor = 'col-resize'
  document.addEventListener('pointermove', onRightResize)
  document.addEventListener('pointerup', endRightResize, { once: true })
  event.preventDefault()
}

function onRightResize(event: PointerEvent) {
  if (!rightResizing) return
  rightWidth.value = clampRightWidth(rightResizeStartWidth + rightResizeStartX - event.clientX)
}

function endRightResize() {
  if (!rightResizing) return
  rightResizing = false
  rightWidth.value = clampRightWidth(rightWidth.value)
  localStorage.setItem('gaze_reading_right_width', String(rightWidth.value))
  document.body.style.userSelect = ''
  document.body.style.cursor = ''
  document.removeEventListener('pointermove', onRightResize)
}

function onRightResizeKeydown(event: KeyboardEvent) {
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
  event.preventDefault()
  const delta = event.key === 'ArrowLeft' ? 24 : -24
  rightWidth.value = clampRightWidth(rightWidth.value + delta)
  localStorage.setItem('gaze_reading_right_width', String(rightWidth.value))
}

type HeadingSegment = { level: number; text: string }

function headingSegments(value?: string): HeadingSegment[] {
  const raw = String(value || '').replace(/\r/g, '').trim()
  if (!raw) return []
  const segments: HeadingSegment[] = []
  const lines = raw.split('\n').map((line) => line.trim()).filter(Boolean)
  for (const line of lines) {
    if (!/^#{1,6}\s/.test(line)) {
      segments.push({ level: 2, text: line })
      continue
    }
    const pattern = /(#{1,6})\s+([\s\S]*?)(?=\s+#{1,6}\s+|$)/g
    let match: RegExpExecArray | null
    while ((match = pattern.exec(line)) !== null) {
      const text = match[2].trim()
      if (text) segments.push({ level: match[1].length, text })
    }
  }
  return segments
}

function cleanHeading(value?: string) {
  const segments = headingSegments(value)
  if (segments.length) return segments[segments.length - 1].text
  return String(value || '').replace(/^#+\s*/, '').trim()
}

function documentTitle(value?: string) {
  return String(value || '').replace(/\.(md|markdown|txt|pdf|docx?)$/i, '').trim()
}

function evidenceLabel(evidence: ReadingEvidence) {
  const title = documentTitle(evidence.document_title || '')
  const heading = cleanHeading(evidence.heading)
  if (heading && title !== documentTitle(heading)) return `${title} · ${heading}`
  return title || heading
}

function dedupeEvidence(evidence: ReadingEvidence[] | undefined) {
  const seen = new Set<string>()
  const out: ReadingEvidence[] = []
  for (const item of evidence || []) {
    const key = `${item.chunk_id}|${item.quote}|${item.heading || ''}`
    if (seen.has(key)) continue
    seen.add(key)
    out.push(item)
  }
  return out
}

function citationSources(conclusion: ReadingConclusion) {
  return dedupeEvidence(conclusion.evidence)
}

function openEvidence(evidence: ReadingEvidence) {
  if (!evidence.knowledge_id) return
  void ensureReadingDocuments()
  pendingEvidenceLocation = evidence
  originalSectionId.value = evidence.section_id || ''
  activeDocumentId.value = evidence.knowledge_id
  originalDrawerOpen.value = true
  void nextTick(scheduleEvidenceLocation)
}

let pendingEvidenceLocation: ReadingEvidence | null = null
let evidenceLocateTimer: number | null = null
let evidenceLocateAttempts = 0

function clearEvidenceMarks() {
  const root = documentPaneRef.value
  if (!root) return
  root.querySelectorAll('.rv-doc-locate-mark').forEach((mark) => {
    mark.replaceWith(...Array.from(mark.childNodes))
  })
  root.querySelectorAll('.rv-doc-locate-block').forEach((el) => {
    el.classList.remove('rv-doc-locate-block')
  })
}

function stopEvidenceLocation() {
  if (evidenceLocateTimer) {
    window.clearTimeout(evidenceLocateTimer)
    evidenceLocateTimer = null
  }
  evidenceLocateAttempts = 0
  pendingEvidenceLocation = null
  clearEvidenceMarks()
}

function scheduleEvidenceLocation() {
  if (evidenceLocateTimer) {
    window.clearTimeout(evidenceLocateTimer)
    evidenceLocateTimer = null
  }
  clearEvidenceMarks()
  evidenceLocateAttempts = 0
  const attempt = () => {
    if (!originalDrawerOpen.value || !pendingEvidenceLocation) return
    const root = documentPaneRef.value?.querySelector('.preview-markdown .markdown-body') as HTMLElement | null
    if (root && locateEvidenceInPreview(root, pendingEvidenceLocation)) return
    evidenceLocateAttempts += 1
    if (evidenceLocateAttempts >= 50) return
    evidenceLocateTimer = window.setTimeout(attempt, 80)
  }
  attempt()
}

function normalizeDocumentText(value: string) {
  let text = ''
  const map: number[] = []
  let previousSpace = false
  for (let i = 0; i < value.length; i += 1) {
    const char = value[i]
    if (/\s/.test(char)) {
      if (!previousSpace && text) {
        text += ' '
        map.push(i)
      }
      previousSpace = true
      continue
    }
    text += char
    map.push(i)
    previousSpace = false
  }
  return { text, map }
}

// Quotes come from raw chunks (Markdown, tables, emphasis); rendered text has
// none of those markers. Compare only letters, digits and CJK characters and
// map the match back to raw offsets.
function normalizeForLocate(value: string) {
  let text = ''
  const map: number[] = []
  for (let i = 0; i < value.length; i += 1) {
    const char = value[i]
    if (!/[\p{L}\p{N}]/u.test(char)) continue
    text += char.toLowerCase()
    map.push(i)
  }
  return { text, map }
}

function locateEvidenceInPreview(root: HTMLElement, evidence: ReadingEvidence) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
  const nodes: Array<{ node: Text; start: number; end: number }> = []
  let raw = ''
  let current = walker.nextNode() as Text | null
  while (current) {
    const value = current.nodeValue || ''
    nodes.push({ node: current, start: raw.length, end: raw.length + value.length })
    raw += value
    current = walker.nextNode() as Text | null
  }
  if (!nodes.length) return false
  const normalized = normalizeDocumentText(raw)
  const target = normalizeDocumentText(evidence.quote || '').text
  const headingText = normalizeDocumentText(cleanHeading(evidence.heading)).text
  let searchFrom = 0
  let headingElement: HTMLElement | null = null
  if (headingText) {
    headingElement = Array.from(root.querySelectorAll<HTMLElement>('h1, h2, h3, h4, h5, h6'))
      .find((heading) => normalizeDocumentText(heading.textContent || '').text.includes(headingText)) || null
    if (headingElement) {
      const firstText = nodes.find((entry) => headingElement?.contains(entry.node))
      if (firstText) searchFrom = firstText.start
    }
  }
  const nodeAt = (offset: number) =>
    nodes.find((entry) => offset >= entry.start && offset < entry.end) || nodes[nodes.length - 1]
  if (target) {
    let index = normalized.text.indexOf(target)
    while (index >= 0 && normalized.map[index] < searchFrom) {
      index = normalized.text.indexOf(target, index + 1)
    }
    if (index < 0) index = normalized.text.indexOf(target)
    if (index >= 0) {
      const rawStart = normalized.map[index]
      const rawEnd = normalized.map[index + target.length - 1] + 1
      const from = nodeAt(rawStart)
      const to = nodeAt(rawEnd - 1)
      try {
        const range = document.createRange()
        range.setStart(from.node, rawStart - from.start)
        range.setEnd(to.node, rawEnd - to.start)
        const block = (from.node.parentElement?.closest('p, li, h1, h2, h3, h4, h5, h6, blockquote, td, th, pre') as HTMLElement | null)
          || from.node.parentElement
        const mark = document.createElement('mark')
        mark.className = 'rv-doc-locate-mark'
        try {
          range.surroundContents(mark)
          scrollPreviewToElement(mark)
        } catch {
          block?.classList.add('rv-doc-locate-block')
          scrollPreviewToElement(block || root)
        }
        return true
      } catch {
        // Fall through to the compact match.
      }
    }
  }
  const compactTarget = normalizeForLocate(evidence.quote || '').text
  if (compactTarget) {
    const compactRaw = normalizeForLocate(raw)
    let index = compactRaw.text.indexOf(compactTarget)
    while (index >= 0 && compactRaw.map[index] < searchFrom) {
      index = compactRaw.text.indexOf(compactTarget, index + 1)
    }
    if (index < 0) index = compactRaw.text.indexOf(compactTarget)
    if (index >= 0) {
      const rawStart = compactRaw.map[index]
      const rawEnd = compactRaw.map[index + compactTarget.length - 1] + 1
      const from = nodeAt(rawStart)
      const to = nodeAt(rawEnd - 1)
      try {
        const range = document.createRange()
        range.setStart(from.node, rawStart - from.start)
        range.setEnd(to.node, rawEnd - to.start)
        const block = (from.node.parentElement?.closest('p, li, h1, h2, h3, h4, h5, h6, blockquote, td, th, pre') as HTMLElement | null)
          || from.node.parentElement
        const mark = document.createElement('mark')
        mark.className = 'rv-doc-locate-mark'
        try {
          range.surroundContents(mark)
          scrollPreviewToElement(mark)
        } catch {
          block?.classList.add('rv-doc-locate-block')
          scrollPreviewToElement(block || root)
        }
        return true
      } catch {
        // Fall through to the heading anchor.
      }
    }
  }
  if (headingElement) {
    headingElement.classList.add('rv-doc-locate-block')
    scrollPreviewToElement(headingElement)
    return true
  }
  return false
}

function scrollPreviewToElement(element: HTMLElement) {
  const scroller = element.closest('.preview-markdown') as HTMLElement | null
  const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  if (!scroller) {
    element.scrollIntoView({ block: 'center', behavior: reduced ? 'auto' : 'smooth' })
    return
  }
  const scrollerRect = scroller.getBoundingClientRect()
  const elementRect = element.getBoundingClientRect()
  const top = scroller.scrollTop + elementRect.top - scrollerRect.top
    - scroller.clientHeight / 2 + elementRect.height / 2
  scroller.scrollTo({ top: Math.max(0, top), behavior: reduced ? 'auto' : 'smooth' })
}

function conclusionStatusLabel(conclusion: ReadingConclusion) {
  if (conclusion.source === 'legacy') return t('reading.legacyNote')
  if (conclusion.status === 'confirmed') {
    return conclusion.source === 'user' ? t('reading.userDecision') : t('reading.confirmedTag')
  }
  if (conclusion.status === 'contradicted') return t('reading.contradicted')
  if (conclusion.status === 'rejected') return t('reading.rejectedTag')
  return t('reading.pendingConfirm')
}

function conclusionTagTheme(conclusion: ReadingConclusion) {
  if (conclusion.source === 'legacy') return 'default'
  if (conclusion.status === 'confirmed') return conclusion.source === 'user' ? 'primary' : 'success'
  if (conclusion.status === 'contradicted') return 'danger'
  return 'warning'
}

function questionStatusLabel(question: ReadingQuestionNode) {
  if (question.status === 'contradicted') return t('reading.contradicted')
  if (question.status === 'skipped') return t('reading.skippedTag')
  return t('reading.pendingConfirm')
}

function questionTagTheme(question: ReadingQuestionNode) {
  if (question.status === 'contradicted') return 'danger'
  if (question.status === 'skipped') return 'default'
  return 'warning'
}

const showReportEditor = computed(() =>
  isStructured.value && ['finished', 'stopped'].includes(String(run.value?.status || '')),
)
const reportDialogVisible = ref(false)
const reportLoading = ref(false)
const reportSaving = ref(false)
const reportDraft = ref('')
const reportEdited = ref(false)
const reportVersion = ref(0)
const reportMode = ref<'edit' | 'split' | 'preview'>('split')
const reportFullscreen = ref(false)
const argumentChainSvg = ref('')
let argumentChainRenderSeq = 0

const reportPreviewParts = computed<Array<{ kind: 'markdown'; html: string } | { kind: 'mermaid' }>>(() => {
  const draft = reportDraft.value
  const parts: Array<{ kind: 'markdown'; html: string } | { kind: 'mermaid' }> = []
  const fence = /```mermaid\s*\n?[\s\S]*?```/g
  let lastIndex = 0
  let match: RegExpExecArray | null
  while ((match = fence.exec(draft))) {
    const before = draft.slice(lastIndex, match.index).trim()
    if (before) parts.push({ kind: 'markdown', html: renderDocumentPreviewMarkdown(before) })
    parts.push({ kind: 'mermaid' })
    lastIndex = match.index + match[0].length
  }
  const rest = draft.slice(lastIndex).trim()
  if (rest) parts.push({ kind: 'markdown', html: renderDocumentPreviewMarkdown(rest) })
  if (!parts.length) parts.push({ kind: 'markdown', html: renderDocumentPreviewMarkdown(draft) })
  return parts
})

async function renderArgumentChain() {
  const source = run.value?.argument_chain_mermaid?.trim() || ''
  const seq = ++argumentChainRenderSeq
  if (!source) {
    argumentChainSvg.value = ''
    return
  }
  try {
    const svg = await renderMermaidToSvg(source, `reading-argument-${Date.now()}`)
    if (seq !== argumentChainRenderSeq) return
    argumentChainSvg.value = svg || ''
  } catch {
    if (seq === argumentChainRenderSeq) argumentChainSvg.value = ''
  }
}

function openArgumentFullscreen() {
  if (argumentChainSvg.value) openMermaidFullscreen(argumentChainSvg.value)
}

const argumentEvidenceNodes = computed(() => {
  const map: Record<string, ReadingEvidence> = {}
  for (const node of run.value?.argument_chain_evidence || []) {
    map[node.id] = node.evidence
  }
  return map
})

function handleArgumentClick(event: MouseEvent) {
  const target = event.target as Element | null
  const node = target?.closest('[id^="flowchart-E"]') as Element | null
  const match = node?.id?.match(/flowchart-(E\d+)-\d+/)
  const evidence = match ? argumentEvidenceNodes.value[match[1]] : null
  if (evidence) {
    void openEvidence(evidence)
    return
  }
  openArgumentFullscreen()
}

watch(
  () => run.value?.argument_chain_mermaid,
  () => { void renderArgumentChain() },
  { immediate: true },
)

const resolvingId = ref('')
const customResolutionId = ref('')
const customResolution = ref('')

const currentRoundDuration = computed(() => {
  const steps = run.value?.steps || []
  for (let i = steps.length - 1; i >= 0; i -= 1) {
    const value = steps[i]?.notes_patch?.duration_ms
    if (typeof value === 'number' && value > 0) return value
  }
  return 0
})
const currentRoundDurationLabel = computed(() => {
  if (!currentRoundDuration.value) return ''
  const seconds = Math.max(1, Math.round(currentRoundDuration.value / 1000))
  return t('reading.roundDuration', { seconds })
})

type EvidenceFloatState = {
  visible: boolean
  top: number
  left: number
  title: string
  quote: string
  content: string
  loading: boolean
  error: string
}
const evidenceFloat = ref<EvidenceFloatState>({
  visible: false, top: 0, left: 0, title: '', quote: '', content: '', loading: false, error: '',
})
const evidenceChunkCache = new Map<string, { content: string; error: string }>()
let evidenceCloseTimer: number | null = null
let evidenceRequestToken = 0

const evidenceFloatParts = computed(() => {
  const content = evidenceFloat.value.content
  const quote = evidenceFloat.value.quote.replace(/^…/, '').replace(/…$/, '').trim()
  if (!content) return []
  const index = quote ? content.indexOf(quote) : -1
  if (index < 0) return [{ text: content, hit: false }]
  return [
    { text: content.slice(0, index), hit: false },
    { text: content.slice(index, index + quote.length), hit: true },
    { text: content.slice(index + quote.length), hit: false },
  ].filter((part) => part.text)
})

function cancelEvidenceClose() {
  if (evidenceCloseTimer) {
    window.clearTimeout(evidenceCloseTimer)
    evidenceCloseTimer = null
  }
}

function hideEvidenceFloat() {
  if (evidenceCloseTimer) window.clearTimeout(evidenceCloseTimer)
  evidenceCloseTimer = window.setTimeout(() => {
    evidenceFloat.value.visible = false
  }, 120)
}

function hideEvidenceOnScroll() {
  if (evidenceFloat.value.visible) evidenceFloat.value.visible = false
}

async function showEvidenceFloat(event: MouseEvent, evidence: ReadingEvidence) {
  cancelEvidenceClose()
  const target = event.currentTarget as HTMLElement
  if (!target) return
  const rect = target.getBoundingClientRect()
  const width = 420
  const token = ++evidenceRequestToken
  evidenceFloat.value = {
    visible: true,
    top: Math.min(rect.bottom + 8, window.innerHeight - 240),
    left: Math.max(8, Math.min(rect.left, window.innerWidth - width - 12)),
    title: evidenceLabel(evidence),
    quote: evidence.quote,
    content: '',
    loading: true,
    error: '',
  }
  const chunkId = evidence.chunk_id
  if (!chunkId) {
    evidenceFloat.value.loading = false
    evidenceFloat.value.error = t('reading.originUnavailable')
    return
  }
  const cached = evidenceChunkCache.get(chunkId)
  if (cached) {
    evidenceFloat.value.content = cached.content
    evidenceFloat.value.error = cached.error
    evidenceFloat.value.loading = false
    return
  }
  try {
    const response = await getChunkByIdOnly(chunkId)
    if (token !== evidenceRequestToken) return
    const content = String((response as any)?.data?.content || '').trim()
    if (!content) throw new Error('empty')
    evidenceChunkCache.set(chunkId, { content, error: '' })
    if (evidenceFloat.value.visible) {
      evidenceFloat.value.content = content
      evidenceFloat.value.error = ''
      evidenceFloat.value.loading = false
    }
  } catch {
    if (token !== evidenceRequestToken) return
    const message = t('reading.originLoadFailed')
    evidenceChunkCache.set(chunkId, { content: '', error: message })
    if (evidenceFloat.value.visible) {
      evidenceFloat.value.error = message
      evidenceFloat.value.loading = false
    }
  }
}

const vEvidenceHover: Directive<HTMLElement, ReadingEvidence> = {
  mounted(el, binding) {
    const enter = (event: MouseEvent) => { void showEvidenceFloat(event, binding.value) }
    const leave = () => hideEvidenceFloat()
    ;(el as any).__evidenceHover = { enter, leave }
    el.addEventListener('mouseenter', enter)
    el.addEventListener('mouseleave', leave)
  },
  unmounted(el) {
    const handlers = (el as any).__evidenceHover
    if (!handlers) return
    el.removeEventListener('mouseenter', handlers.enter)
    el.removeEventListener('mouseleave', handlers.leave)
  },
}

const openReportEditor = async () => {
  if (!run.value) return
  reportLoading.value = true
  try {
    const response = await getReadingReport(run.value.id)
    reportDraft.value = response.data.content
    reportEdited.value = response.data.edited
    reportVersion.value = response.data.version || 0
    reportMode.value = 'split'
    reportFullscreen.value = false
    reportDialogVisible.value = true
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.reportLoadFailed'))
  } finally {
    reportLoading.value = false
  }
}

function onReportOverlayKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  if (reportFullscreen.value) {
    reportFullscreen.value = false
    return
  }
  reportDialogVisible.value = false
}

watch(reportDialogVisible, (open) => {
  if (open) {
    window.addEventListener('keydown', onReportOverlayKeydown)
  } else {
    window.removeEventListener('keydown', onReportOverlayKeydown)
    reportFullscreen.value = false
  }
})

async function reloadReportAfterConflict() {
  if (!run.value) return
  MessagePlugin.warning(t('reading.reportConflict'))
  try {
    const response = await getReadingReport(run.value.id)
    reportDraft.value = response.data.content
    reportEdited.value = response.data.edited
    reportVersion.value = response.data.version || 0
  } catch {
    // Keep the editor open; the next save will surface the same conflict if
    // the report cannot be reloaded.
  }
}

const saveReportDraft = async () => {
  if (!run.value || !reportDraft.value.trim()) return
  reportSaving.value = true
  try {
    const response = await saveReadingReport(
      run.value.id, reportDraft.value, reportVersion.value,
    )
    reportEdited.value = response.data.edited
    reportVersion.value = response.data.version || reportVersion.value
    reportDialogVisible.value = false
    MessagePlugin.success(t('reading.reportSaved'))
  } catch (err: any) {
    if (err?.status === 409) {
      await reloadReportAfterConflict()
    } else {
      MessagePlugin.error(err?.message || t('reading.reportSaveFailed'))
    }
  } finally {
    reportSaving.value = false
  }
}

const resetReportDraft = async () => {
  if (!run.value) return
  reportSaving.value = true
  try {
    const response = await deleteReadingReport(run.value.id, reportVersion.value)
    reportDraft.value = response.data.content
    reportEdited.value = false
    reportVersion.value = 0
    MessagePlugin.success(t('reading.reportReset'))
  } catch (err: any) {
    if (err?.status === 409) {
      await reloadReportAfterConflict()
    } else {
      MessagePlugin.error(err?.message || t('reading.reportSaveFailed'))
    }
  } finally {
    reportSaving.value = false
  }
}

const toggleCustomResolution = (item: ReadingContradiction) => {
  customResolutionId.value = customResolutionId.value === item.id ? '' : item.id
  customResolution.value = ''
}

const resolveContradiction = async (item: ReadingContradiction, choice: 'a' | 'b' | 'custom') => {
  if (!run.value || resolvingId.value) return
  resolvingId.value = item.id
  try {
    const response = await resolveReadingContradiction(run.value.id, item.id, {
      choice,
      resolution: choice === 'custom' ? customResolution.value : '',
    })
    await persistRun(response.data)
    customResolutionId.value = ''
    customResolution.value = ''
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.resolveFailed'))
  } finally {
    resolvingId.value = ''
  }
}

const statusTheme = computed(() => {
  if (run.value?.status === 'finished') return 'success'
  if (run.value?.status === 'failed') return 'danger'
  if (run.value?.status === 'stopped') return 'default'
  return 'primary'
})

const depthLabel = computed(() => t(`reading.depth.${run.value?.plan.depth || 'standard'}`))

const planStructure = computed<ReadingSectionStructure[]>(() => run.value?.plan.structure || [])

const sectionTitleByID = (sectionId: string) => {
  const section = (run.value?.plan.sections || []).find((item) => item.id === sectionId)
  if (!section) return sectionId
  return section.heading ? `${section.title} · ${section.heading}` : section.title
}

const agentPhaseRows = computed(() => {
  const labels: Record<string, string> = {
    structure: t('reading.phaseStructure'),
    synthesis: t('reading.phaseSynthesis'),
    self_test: t('reading.phaseSelfTest'),
  }
  const rows: Array<{ key: string; label: string; degraded: boolean }> = []
  const seen = new Set<string>()
  for (const step of run.value?.steps || []) {
    const action = String(step.action || '')
    const degraded = step.status === 'degraded'
      || Boolean((step.notes_patch as Record<string, unknown> | undefined)?.degraded)
    if (action === 'degraded') {
      rows.push({ key: `degraded-${step.seq}`, label: labels[step.section_id || ''] || t('reading.phaseReading'), degraded: true })
      continue
    }
    if (!labels[action] || seen.has(action)) continue
    seen.add(action)
    rows.push({ key: action, label: labels[action], degraded })
  }
  return rows
})

const canAddDocs = computed(() => {
  const current = run.value
  if (!current) return false
  return current.status !== 'finished'
    && current.status !== 'stopped'
    && current.status !== 'failed'
    && current.document_ids.length < 5
    && Boolean(current.plan.knowledge_base_id)
})

const noteGroups = computed(() => {
  const notes = run.value?.notes || {}
  return [
    { key: 'confirmed', label: t('reading.noteGroups.confirmed'), items: notes.confirmed || [] },
    { key: 'terms', label: t('reading.noteGroups.terms'), items: notes.terms || [] },
    { key: 'open_questions', label: t('reading.noteGroups.openQuestions'), items: notes.open_questions || [] },
    { key: 'contradictions', label: t('reading.noteGroups.contradictions'), items: notes.contradictions || [] },
    { key: 'decisions', label: t('reading.noteGroups.decisions'), items: notes.decisions || [] },
    { key: 'citations', label: t('reading.noteGroups.citations'), items: notes.citations || [] },
  ]
})

const finalSummary = computed(() => String(run.value?.final?.summary || ''))
const finalSummaryParagraphs = computed(() =>
  finalSummary.value
    .split(/\n+/)
    .map((line) => line.trim())
    .filter(Boolean),
)

interface FinalTextItem { lead: string; rest: string }
interface FinalCitationItem {
  key: string
  title: string
  knowledge_id: string
  chunk_id: string
  section_id: string
  document_title: string
  quote: string
}

function splitFinalText(text: string): FinalTextItem {
  const trimmed = text.trim()
  const match = trimmed.match(/^([^：:]{2,24})[：:]\s*(.+)$/s)
  if (match) return { lead: match[1].trim(), rest: match[2].trim() }
  return { lead: '', rest: trimmed }
}

function toStringItems(value: unknown): FinalTextItem[] {
  if (!Array.isArray(value)) return []
  const items: FinalTextItem[] = []
  for (const entry of value) {
    const text = typeof entry === 'string' ? entry : JSON.stringify(entry)
    for (const line of text.split('\n')) {
      const trimmed = line.trim()
      if (trimmed) items.push(splitFinalText(trimmed))
    }
  }
  return items
}

const finalTextGroups = computed(() => {
  const final = run.value?.final || {}
  return [
    { key: 'key_points', label: t('reading.finalGroups.keyPoints'), items: toStringItems(final.key_points) },
    { key: 'open_questions', label: t('reading.finalGroups.openQuestions'), items: toStringItems(final.open_questions) },
    { key: 'contradictions', label: t('reading.finalGroups.contradictions'), items: toStringItems(final.contradictions) },
  ]
})

function normalizeFinalCitation(value: unknown): Record<string, unknown> | null {
  if (!value) return null
  if (typeof value === 'string') {
    try {
      const parsed = JSON.parse(value)
      if (parsed && typeof parsed === 'object') return parsed as Record<string, unknown>
    } catch {
      // plain string reference; keep the text so it is not silently dropped
    }
    return { quote: value }
  }
  return typeof value === 'object' ? (value as Record<string, unknown>) : null
}

const finalCitations = computed<FinalCitationItem[]>(() => {
  const list = run.value?.final?.citations
  if (!Array.isArray(list)) return []
  const seen = new Map<string, FinalCitationItem>()
  for (const raw of list) {
    const item = normalizeFinalCitation(raw)
    if (!item) continue
    const knowledgeId = String(item.knowledge_id || '').trim()
    const documentTitle = String(item.document_title || item.title || '').trim()
    const title = documentTitle || (knowledgeId ? knowledgeId.slice(0, 8) : '')
    const key = knowledgeId || title
    if (!key || seen.has(key)) continue
    seen.set(key, {
      key,
      title: title || key,
      knowledge_id: knowledgeId,
      chunk_id: String(item.chunk_id || ''),
      section_id: String(item.section_id || ''),
      document_title: documentTitle,
      quote: String(item.quote || ''),
    })
  }
  return [...seen.values()]
})

function openFinalCitation(cite: FinalCitationItem) {
  if (!cite.knowledge_id) return
  openEvidence({
    knowledge_id: cite.knowledge_id,
    chunk_id: cite.chunk_id,
    section_id: cite.section_id,
    document_title: cite.document_title || cite.title,
    quote: cite.quote,
  })
}

function filterOptions(options: Array<{ value: string; label: string }>, search: string) {
  const q = search.trim().toLowerCase()
  if (!q) return options
  return options.filter((option) => option.label.toLowerCase().includes(q))
}

function withSelectedOptions(options: Array<{ value: string; label: string }>, selectedIds: string[]) {
  const selected = options.filter((option) => selectedIds.includes(option.value))
  if (!selected.length) return options
  const selectedValues = new Set(selected.map((option) => option.value))
  return [...selected, ...options.filter((option) => !selectedValues.has(option.value))]
}

const loadKnowledgeBases = async () => {
  try {
    const response = await listKnowledgeBases()
    const list = response.data?.data || response.data || []
    kbOptions.value = (Array.isArray(list) ? list : []).map((kb: any) => ({
      value: kb.id,
      label: kb.name || kb.id,
    }))
    return kbOptions.value
  } catch {
    kbOptions.value = []
    return []
  }
}

function formatFileSize(bytes: number) {
  if (!bytes) return t('reading.sizeUnknown')
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${Math.max(1, Math.round(bytes / 1024))} KB`
}

const loadDocuments = async () => {
  const bases = kbOptions.value.length ? kbOptions.value : await loadKnowledgeBases()
  const all: ReadingDocOption[] = []
  const trees: Record<string, KnowledgeFolderNode[]> = {}
  await Promise.all(bases.map(async (base) => {
    try {
      const [filesResponse, foldersResponse] = await Promise.all([
        listKnowledgeFiles(base.value, { page: 1, page_size: 500 }),
        listKnowledgeFolders(base.value),
      ])
      const tree = (foldersResponse as any)?.data
      trees[base.value] = Array.isArray(tree?.folders) ? tree.folders : []
      const list = filesResponse.data || []
      for (const doc of Array.isArray(list) ? list : []) {
        if (doc.parse_status !== 'completed') continue
        all.push({
          value: doc.id,
          label: doc.file_name || doc.title || doc.id,
          kbId: base.value,
          kbName: base.label,
          folderId: String(doc.folder_id || ''),
          fileSize: Number(doc.file_size || 0),
          sizeLabel: formatFileSize(Number(doc.file_size || 0)),
        })
      }
    } catch {
      // One unavailable knowledge base must not block the picker.
    }
  }))
  folderTrees.value = trees
  documentOptions.value = all.sort((a, b) => a.label.localeCompare(b.label, 'zh-CN'))
  documentIds.value = documentIds.value.filter((id) =>
    documentOptions.value.some((doc) => doc.value === id),
  )
}

const loadReadingDocuments = async () => {
  const current = run.value
  if (!current || !isStructured.value) {
    readingDocuments.value = []
    return
  }
  readingDocumentsLoading.value = true
  try {
    const response = await getReadingRunDocuments(current.id)
    readingDocuments.value = response.data.documents || []
    const evidenceDocID = pendingEvidenceLocation?.knowledge_id
    const currentDocID = currentSection.value?.knowledge_id
    if (evidenceDocID && readingDocuments.value.some((doc) => doc.id === evidenceDocID)) {
      activeDocumentId.value = evidenceDocID
    } else if (currentDocID && readingDocuments.value.some((doc) => doc.id === currentDocID)) {
      activeDocumentId.value = currentDocID
    } else if (!readingDocuments.value.some((doc) => doc.id === activeDocumentId.value)) {
      activeDocumentId.value = readingDocuments.value[0]?.id || ''
    }
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.documentLoadFailed'))
    readingDocuments.value = []
  } finally {
    readingDocumentsLoading.value = false
  }
}

const openAddDocs = async () => {
  const current = run.value
  if (!current) return
  if (!documentOptions.value.length) await loadDocuments()
  const existing = new Set(current.document_ids)
  addDocOptions.value = documentOptions.value.filter((doc) => !existing.has(doc.value))
  addDocIds.value = []
  addDocSearch.value = ''
  addDocDialogVisible.value = true
}

const confirmAddDocs = async () => {
  if (!run.value || !addDocIds.value.length) return
  addingDocs.value = true
  try {
    const response = await addReadingDocuments(run.value.id, addDocIds.value)
    await persistRun(response.data)
    addDocDialogVisible.value = false
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.addDocumentsFailed'))
  } finally {
    addingDocs.value = false
  }
}

const persistRun = async (next: ReadingRun) => {
  run.value = next
  if (next.status === 'finished' || next.status === 'stopped' || next.status === 'abandoned') {
    localStorage.removeItem(storageKey)
  } else {
    localStorage.setItem(storageKey, next.id)
  }
}

const flattenReviewGroups = (
  nodes: ReviewGroupNode[],
  depth = 0,
): Array<{ label: string; value: string }> =>
  nodes.flatMap((node) => [
    { label: `${'　'.repeat(depth)}${node.group.name}`, value: node.group.id },
    ...flattenReviewGroups(node.children || [], depth + 1),
  ])

const activeFeynmanSectionId = computed(() => viewingSection.value?.id || currentSection.value?.id || '')

const openFeynman = async () => {
  if (!run.value || !activeFeynmanSectionId.value) return
  feynmanSectionId.value = activeFeynmanSectionId.value
  feynmanAnswers.value = { macro: '', mechanism: '', detail: '', application: '' }
  feynmanFreeform.value = ''
  feynmanVisible.value = true
  feynmanHistoryLoading.value = true
  try {
    const response = await listReadingFeynman(run.value.id, feynmanSectionId.value)
    feynmanAttempts.value = response.data || []
  } catch {
    feynmanAttempts.value = []
  } finally {
    feynmanHistoryLoading.value = false
  }
}

const submitFeynman = async () => {
  if (!run.value || !feynmanSectionId.value || feynmanSubmitting.value) return
  if (!feynmanFilled.value) {
    MessagePlugin.warning(t('reading.feynmanTooShort'))
    return
  }
  const answers = feynmanAngleDefs.value
    .map((angle) => ({ angle_id: angle.id, label: angle.label, prompt: angle.prompt, text: feynmanAnswers.value[angle.id]?.trim() || '' }))
    .filter((answer) => answer.text)
  const freeform = feynmanFreeform.value.trim()
  feynmanSubmitting.value = true
  try {
    const response = await submitReadingFeynman(run.value.id, {
      section_id: feynmanSectionId.value,
      answers,
      freeform,
    })
    feynmanAttempts.value = [response.data, ...feynmanAttempts.value]
    if (run.value) {
      run.value.feynman_attempts = [response.data, ...(run.value.feynman_attempts || [])]
    }
    feynmanAnswers.value = { macro: '', mechanism: '', detail: '', application: '' }
    feynmanFreeform.value = ''
    MessagePlugin.success(t('reading.feynmanSubmitted'))
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.feynmanFailed'))
  } finally {
    feynmanSubmitting.value = false
  }
}

function feynmanStatusLabel(status?: string) {
  switch (status) {
    case 'strong': return t('reading.feynmanStatusStrong')
    case 'missing': return t('reading.feynmanStatusMissing')
    case 'skipped': return t('reading.feynmanStatusSkipped')
    default: return t('reading.feynmanStatusPartial')
  }
}

const feynmanHistory = computed(() =>
  feynmanAttempts.value.slice(0, 3).map((attempt) => ({
    attempt,
    answers: (attempt.answers || []).map((answer) => ({
      ...answer,
      feedback: attempt.angle_feedback?.find((item) => item.angle_id === answer.angle_id) || null,
    })),
  })),
)

const capturePreviewSelection = () => {
  const selection = window.getSelection()
  if (!selection || selection.isCollapsed || !documentPaneRef.value) {
    previewSelectionText.value = ''
    return
  }
  const range = selection.getRangeAt(0)
  if (!documentPaneRef.value.contains(range.commonAncestorContainer)) {
    previewSelectionText.value = ''
    return
  }
  const text = selection.toString().replace(/\s+/g, ' ').trim()
  previewSelectionText.value = text.length >= 2 && text.length <= 1200 ? text : ''
}

const openAnnotationDialog = () => {
  if (!previewSelectionText.value) return
  annotationQuote.value = previewSelectionText.value
  annotationComment.value = ''
  annotationDialogVisible.value = true
}

const saveAnnotation = async () => {
  if (!run.value || !annotationQuote.value || annotationSaving.value) return
  annotationSaving.value = true
  try {
    const response = await createReadingAnnotation(run.value.id, {
      section_id: previewSectionData.value?.id || run.value.current_section_id,
      quote: annotationQuote.value,
      comment: annotationComment.value.trim(),
    })
    run.value.annotations = [...(run.value.annotations || []), response.data]
    annotationDialogVisible.value = false
    annotationQuote.value = ''
    annotationComment.value = ''
    previewSelectionText.value = ''
    window.getSelection()?.removeAllRanges()
    MessagePlugin.success(t('reading.annotationSaved'))
    scheduleAnnotationHighlights([0, 60])
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.annotationFailed'))
  } finally {
    annotationSaving.value = false
  }
}

const removeAnnotation = (annotation: ReadingAnnotation) => {
  if (!run.value) return
  const dialog = DialogPlugin.confirm({
    header: t('reading.annotationDeleteTitle'),
    body: t('reading.annotationDeleteBody'),
    theme: 'danger',
    confirmBtn: t('common.delete'),
    cancelBtn: t('common.cancel'),
    onConfirm: async () => {
      try {
        await deleteReadingAnnotation(run.value!.id, annotation.id)
        run.value!.annotations = (run.value!.annotations || []).filter((item) => item.id !== annotation.id)
        MessagePlugin.success(t('reading.annotationDeleted'))
      } catch (err: any) {
        MessagePlugin.error(err?.message || t('reading.annotationDeleteFailed'))
      } finally {
        dialog.destroy()
      }
    },
  })
}

const openAnnotationSource = (annotation: ReadingAnnotation) => {
  void openEvidence({
    knowledge_id: annotation.knowledge_id,
    chunk_id: annotation.chunk_id,
    section_id: annotation.section_id,
    quote: annotation.quote,
  })
}

const ensureReviewGroups = async () => {
  if (reviewGroupOptions.value.length) return
  const response = await listReviewGroups()
  const groups = (response as { data?: ReviewGroupNode[] })?.data || []
  reviewGroupOptions.value = flattenReviewGroups(groups)
  if (reviewGroupOptions.value.length && !selectedReviewGroupId.value) {
    selectedReviewGroupId.value = reviewGroupOptions.value[0].value
  }
}

const openAnnotationCard = async (annotation: ReadingAnnotation) => {
  annotationCardTarget.value = annotation
  annotationCardQuestion.value = annotation.comment || `${t('reading.annotationDefaultQuestion')}${annotation.quote.slice(0, 120)}`
  annotationCardAnswer.value = annotation.quote
  annotationCardGroupId.value = selectedReviewGroupId.value
  annotationCardVisible.value = true
  try {
    await ensureReviewGroups()
    if (!annotationCardGroupId.value) annotationCardGroupId.value = selectedReviewGroupId.value
  } catch {
    // Leave the group selector empty; the submit action will explain.
  }
}

const submitAnnotationCard = async () => {
  if (!run.value || !annotationCardTarget.value || !annotationCardGroupId.value || annotationCardSaving.value) {
    if (!annotationCardGroupId.value) MessagePlugin.warning(t('reading.selfTestSelectGroup'))
    return
  }
  annotationCardSaving.value = true
  try {
    const response = await createCardFromReadingAnnotation(run.value.id, annotationCardTarget.value.id, {
      group_id: annotationCardGroupId.value,
      question: annotationCardQuestion.value.trim(),
      answer: annotationCardAnswer.value.trim(),
    })
    MessagePlugin.success(t('reading.annotationCardSaved', { count: response.data.imported }))
    annotationCardVisible.value = false
    annotationCardTarget.value = null
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.annotationCardFailed'))
  } finally {
    annotationCardSaving.value = false
  }
}

const loadSelfTests = async (runId: string) => {
  if (!runId) return
  selfTestsLoading.value = true
  try {
    const tasks: [Promise<unknown>, Promise<unknown> | null] = [
      listReadingSelfTests(runId),
      reviewGroupOptions.value.length ? null : listReviewGroups(),
    ]
    const [testsResponse, groupsResponse] = await Promise.all(tasks)
    selfTests.value = (testsResponse as { data?: ReadingSelfTest[] })?.data || []
    if (groupsResponse) {
      const groups = (groupsResponse as { data?: ReviewGroupNode[] })?.data || []
      reviewGroupOptions.value = flattenReviewGroups(groups)
      if (!selectedReviewGroupId.value && reviewGroupOptions.value.length) {
        selectedReviewGroupId.value = reviewGroupOptions.value[0].value
      }
    }
    selectedSelfTestIds.value = selfTests.value
      .filter((test) => test.status === 'candidate')
      .map((test) => test.id)
    selfTestLoadedRunId.value = runId
  } catch {
    selfTests.value = []
  } finally {
    selfTestsLoading.value = false
  }
}

const toggleSelfTest = (test: ReadingSelfTest) => {
  if (test.status !== 'candidate') return
  const index = selectedSelfTestIds.value.indexOf(test.id)
  if (index >= 0) {
    selectedSelfTestIds.value.splice(index, 1)
  } else {
    selectedSelfTestIds.value.push(test.id)
  }
}

const importSelectedSelfTests = async () => {
  if (!run.value || !selectedReviewGroupId.value || !selectedSelfTestIds.value.length) return
  importingSelfTests.value = true
  try {
    const response = await importReadingSelfTests(
      run.value.id,
      selectedReviewGroupId.value,
      selectedSelfTestIds.value,
    )
    const result = response.data
    MessagePlugin.success(
      t('reading.selfTestImported', {
        imported: result?.imported ?? 0,
        skipped: result?.skipped ?? 0,
      }),
    )
    await loadSelfTests(run.value.id)
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.selfTestImportFailed'))
  } finally {
    importingSelfTests.value = false
  }
}

watch(
  () => [run.value?.id, run.value?.status] as const,
  ([runId, status]) => {
    if (status === 'finished' && runId && selfTestLoadedRunId.value !== runId) {
      void loadSelfTests(runId)
    }
  },
  { immediate: true },
)

const createRun = async () => {
  creating.value = true
  try {
    if (run.value && canPauseRun.value) {
      const stopped = await stopReadingRun(run.value.id)
      await persistRun(stopped.data)
    }
    const session = await createSessions({ title: t('reading.sessionTitle') })
    const sessionId = session.data?.id || session.data?.data?.id
    if (!sessionId) throw new Error('session create failed')
    const response = await createReadingRun({
      session_id: sessionId,
      document_ids: documentIds.value,
      goal: goal.value,
      depth: depth.value,
    })
    await persistRun(response.data)
    configView.value = false
    syncViewQuery(false)
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.startFailed'))
  } finally {
    creating.value = false
  }
}

const startRun = async () => {
  if (!documentIds.value.length) return
  if (run.value && canPauseRun.value) {
    const dialog = DialogPlugin.confirm({
      header: t('reading.replaceRunTitle'),
      body: t('reading.replaceRunBody'),
      theme: 'warning',
      confirmBtn: t('common.confirm'),
      cancelBtn: t('common.cancel'),
      onConfirm: () => {
        dialog.hide()
        void createRun()
      },
      onClose: () => dialog.hide(),
    })
    return
  }
  await createRun()
}

const startNewReading = async () => {
  run.value = null
  feedback.value = ''
  goal.value = ''
  documentIds.value = []
  readingDocuments.value = []
  activeDocumentId.value = ''
  selectedTraceRound.value = 0
  addDocDialogVisible.value = false
  addDocIds.value = []
  localStorage.removeItem(storageKey)
  const query: Record<string, string> = {}
  if (kbId.value) query.kb = kbId.value
  await router.replace({ path: route.path, query })
}

function resetLiveProgress() {
  liveProgress.value = {
    phase: '',
    status: '',
    sectionTitle: '',
    sectionHeading: '',
    round: 0,
    searches: [],
    reasoning: '',
  }
  liveElapsedMs.value = 0
}

function startLiveTicker() {
  if (liveTimer !== null) window.clearInterval(liveTimer)
  liveTimer = window.setInterval(() => {
    liveElapsedMs.value += 1000
  }, 1000)
}

function stopLiveTicker() {
  if (liveTimer !== null) window.clearInterval(liveTimer)
  liveTimer = null
}

function applyLiveEvent(event: ReadingProgressEvent) {
  if (!liveProgress.value) resetLiveProgress()
  const live = liveProgress.value
  if (!live) return
  if (typeof event.elapsed_ms === 'number') liveElapsedMs.value = event.elapsed_ms
  if (event.type === 'phase') {
    live.phase = event.phase || live.phase
    live.status = event.status || ''
    live.sectionTitle = event.section_title || live.sectionTitle
    live.sectionHeading = event.section_heading || live.sectionHeading
    live.round = event.round || live.round
    return
  }
  if (event.type === 'search' && event.query) {
    live.searches = [...live.searches, { query: event.query, hitCount: Number(event.hit_count || 0) }].slice(-6)
    return
  }
  if (event.type === 'reasoning' && event.content) {
    live.reasoning = (live.reasoning + event.content).slice(-20000)
  }
}

function streamStepOnce(runId: string): Promise<ReadingRun> {
  return new Promise((resolve, reject) => {
    let settled = false
    let received = false
    const finish = (result: ReadingRun) => {
      if (settled) return
      settled = true
      resolve(result)
    }
    const fail = (error: Error) => {
      if (settled) return
      settled = true
      reject(error)
    }
    const fallbackIfUnsupported = (error: Error) => {
      if (received || !/HTTP (404|405)|Failed to fetch|NetworkError/i.test(error.message)) {
        fail(error)
        return
      }
      // Older backends do not expose the SSE route yet; keep the workbench
      // usable during rollout by falling back to the blocking step endpoint.
      stepReadingRun(runId).then((response) => finish(response.data)).catch(fail)
    }
    activeReadingStream = streamReadingStep(runId, {
      onEvent(event) {
        if (settled) return
        received = true
        if (event.type === 'complete') {
          if (event.run) finish(event.run)
          else fail(new Error(t('reading.stepFailed')))
          return
        }
        applyLiveEvent(event)
      },
      onError(error) {
        fallbackIfUnsupported(error)
      },
      onClose() {
        if (!settled) fail(new Error(t('reading.stepStreamClosed')))
      },
    })
  })
}

const confirmPlan = async () => {
  if (!run.value || run.value.status !== 'planning') return
  acting.value = true
  try {
    await advanceUntilPause()
  } finally {
    acting.value = false
  }
}

function scrollReadingToTop() {
  const main = readingBodyRef.value?.querySelector<HTMLElement>('.rv-main')
  if (main) main.scrollTop = 0
}

const advanceUntilPause = async () => {
  if (!run.value || advancing.value) return
  advancing.value = true
  scrollReadingToTop()
  startLiveTicker()
  try {
    let next = run.value
    let stepBaseline = Number(next?.step_count || 0)
    for (let i = 0; i < 40; i += 1) {
      if (!next || ['waiting_user', 'finished', 'stopped', 'failed'].includes(next.status)) break
      if (pendingQuestion(next)) break
      resetLiveProgress()
      next = await streamStepOnce(next.id)
      await persistRun(next)
      const stepped = Number(next.step_count || 0) > stepBaseline
      stepBaseline = Number(next.step_count || 0)
      // Phase-transition steps (e.g. structure -> reading) do not consume a
      // round; keep going so one click always advances a real reading round.
      const inReadingPhase = !next.phase || next.phase === 'reading'
      if (pauseEachRound.value && stepped && inReadingPhase && next.status === 'reading') break
    }
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.stepFailed'))
  } finally {
    activeReadingStream?.close()
    activeReadingStream = null
    stopLiveTicker()
    advancing.value = false
  }
}

function pendingQuestion(value: ReadingRun) {
  if (Number(value.plan?.state_version || 0) >= 2) {
    return (value.questions || []).some((question) => question.status === 'pending')
  }
  const steps: ReadingRunStep[] = value.steps || []
  for (let i = steps.length - 1; i >= 0; i -= 1) {
    if (steps[i].action === 'ask_user' && !steps[i].user_feedback) return true
  }
  return false
}

const acceptRecommendation = () => {
  const answer = currentPendingQuestion.value?.recommended_answer || currentQuestion.value?.recommended_answer
  if (answer) {
    feedback.value = answer
  }
}

const skipQuestion = async () => {
  if (isStructured.value) {
    if (!run.value || !currentPendingQuestion.value) return
    acting.value = true
    try {
      const response = await feedbackReadingRun(run.value.id, {
        skip: true,
        question_id: currentPendingQuestion.value.id,
      })
      feedback.value = ''
      await persistRun(response.data)
      await advanceUntilPause()
    } catch (err: any) {
      MessagePlugin.error(err?.message || t('reading.feedbackFailed'))
    } finally {
      acting.value = false
    }
    return
  }
  feedback.value = t('reading.skipFeedback')
  await submitFeedback()
}

const submitFeedback = async () => {
  const text = feedback.value.trim()
  if (!run.value || !text) return
  acting.value = true
  try {
    const payload = isStructured.value
      ? { feedback: text, question_id: currentPendingQuestion.value?.id }
      : { feedback: text }
    const response = await feedbackReadingRun(run.value.id, payload)
    feedback.value = ''
    await persistRun(response.data)
    await advanceUntilPause()
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.feedbackFailed'))
  } finally {
    acting.value = false
  }
}

const stopRun = async () => {
  if (!run.value) return
  try {
    const response = await stopReadingRun(run.value.id)
    await persistRun(response.data)
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.stopFailed'))
  }
}

const resumeOrAdvance = async () => {
  if (!run.value || advancing.value) return
  acting.value = true
  try {
    if (run.value.status === 'stopped' || run.value.status === 'failed') {
      const response = await resumeReadingRun(run.value.id)
      await persistRun(response.data)
    }
    await advanceUntilPause()
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.resumeFailed'))
  } finally {
    acting.value = false
  }
}

const exportReport = async (format: 'markdown' | 'json') => {
  if (!run.value) return
  exporting.value = true
  try {
    const response = await exportReadingRun(run.value.id, format)
    const report = response.data
    downloadFile(
      report.filename,
      report.content,
      format === 'json' ? 'application/json' : 'text/markdown',
    )
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.exportFailed'))
  } finally {
    exporting.value = false
  }
}

function downloadFile(filename: string, content: string, type: string) {
  const blob = new Blob([content], { type })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

function queryValue(value: unknown): string {
  if (Array.isArray(value)) return String(value[0] || '')
  return String(value || '')
}

watch(
  () => [run.value?.id, (run.value?.document_ids || []).join(',')],
  () => {
    void loadReadingDocuments()
  },
)
watch(
  () => currentSection.value?.knowledge_id,
  (docId) => {
    if (docId && readingDocuments.value.some((doc) => doc.id === docId)) {
      activeDocumentId.value = docId
    }
  },
)

function mapReadingDraft(draft: ApiReadingDraft): ReadingDraft {
  return {
    id: draft.id,
    name: draft.name,
    documentIds: Array.isArray(draft.document_ids) ? draft.document_ids : [],
    depth: draft.depth || 'standard',
    goal: draft.goal || '',
    updatedAt: draft.updated_at || new Date().toISOString(),
  }
}

async function loadDrafts() {
  try {
    const response = await listReadingDrafts()
    drafts.value = (response.data || []).map(mapReadingDraft).slice(0, 20)
  } catch {
    drafts.value = []
  }
}

function depthLabelOf(value: ReadingDepth) {
  return t(`reading.depth.${value || 'standard'}`)
}

function formatDraftDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit' }).format(date)
}

function syncViewQuery(config: boolean) {
  const query: Record<string, string> = {}
  for (const [key, value] of Object.entries(route.query)) {
    if (typeof value === 'string' && value) query[key] = value
  }
  if (config) query.view = 'config'
  else delete query.view
  void router.replace({ path: route.path, query })
}

function switchView(view: 'workbench' | 'config') {
  if (view === 'workbench' && !run.value) return
  configView.value = view === 'config'
  mobileLeftOpen.value = false
  mobileRightOpen.value = false
  if (view === 'workbench') previewSectionId.value = ''
  syncViewQuery(view === 'config')
}


const openHistoryRun = async (item: ReadingHistoryItem) => {
  try {
    const response = await getReadingRun(item.id)
    run.value = response.data
    configView.value = false
    previewSectionId.value = ''
    feedback.value = ''
    selectedTraceRound.value = 0
    localStorage.setItem(storageKey, response.data.id)
    const query: Record<string, string> = {}
    for (const [key, value] of Object.entries(route.query)) {
      if (typeof value === 'string' && value) query[key] = value
    }
    delete query.view
    query.run_id = response.data.id
    await router.replace({ path: route.path, query })
    void loadReadingDocuments()
    historyOpen.value = false
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.historyOpenFailed'))
  }
}

const workbenchVisible = computed(() => Boolean(run.value) && !showConfigView.value)

watch(
  workbenchVisible,
  (visible) => {
    if (visible && !autoFullscreenApplied) {
      shellUi.setFullscreen(true)
      autoFullscreenApplied = true
    } else if (!visible && autoFullscreenApplied) {
      autoFullscreenApplied = false
      shellUi.setFullscreen(false)
    }
  },
  { immediate: true },
)

watch(
  () => currentPendingQuestion.value?.id,
  () => {
    if (inlineAnswerSubmitted.value) return
    selectedAnswerId.value = ''
    customAnswerMode.value = false
    answerDraft.value = ''
    answerTextareaRef.value = null
  },
)

watch(
  () => `${currentRound.value}:${currentUnderstanding.value}:${currentUnderstandingRecord.value?.decision || ''}`,
  () => {
    const record = currentUnderstandingRecord.value
    stageDecision.value = record && record.decision !== 'pending' ? record.decision : ''
    stageCorrecting.value = false
    stageCorrection.value = record?.correction || ''
  },
)

function toggleDocument(id: string) {
  const index = documentIds.value.indexOf(id)
  if (index >= 0) {
    documentIds.value = documentIds.value.filter((item) => item !== id)
    return
  }
  if (documentIds.value.length >= 5) {
    MessagePlugin.warning(t('reading.maxRefsHint'))
    return
  }
  const doc = documentOptions.value.find((item) => item.value === id)
  if (doc) {
    const kbKey = `kb:${doc.kbId}`
    if (expandedTreeNodes.value[kbKey] === false) {
      expandedTreeNodes.value = { ...expandedTreeNodes.value, [kbKey]: true }
    }
  }
  documentIds.value = [...documentIds.value, id]
}

function toggleAddDoc(id: string) {
  const index = addDocIds.value.indexOf(id)
  addDocIds.value = index >= 0
    ? addDocIds.value.filter((item) => item !== id)
    : [...addDocIds.value, id]
}

function selectGoalChip(chip: { key: string; value: string }) {
  if (chip.key === 'custom') {
    goalTextareaRef.value?.focus()
    return
  }
  goal.value = goal.value === chip.value ? '' : chip.value
}

async function saveDraft() {
  if (!documentIds.value.length) return
  const first = documentOptions.value.find((doc) => doc.value === documentIds.value[0])
  const name = t('reading.draftAutoName', {
    name: first?.label || t('reading.title'),
    date: formatDraftDate(new Date().toISOString()),
  })
  try {
    const response = await createReadingDraft({
      name,
      document_ids: [...documentIds.value],
      depth: depth.value,
      goal: goal.value,
    })
    drafts.value = [mapReadingDraft(response.data), ...drafts.value].slice(0, 20)
    MessagePlugin.success(t('reading.draftSaved'))
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.draftSaveFailed'))
  }
}

function loadDraft(draft: ReadingDraft) {
  const available = new Set(documentOptions.value.map((doc) => doc.value))
  documentIds.value = draft.documentIds.filter((id) => available.has(id)).slice(0, 5)
  depth.value = draft.depth
  goal.value = draft.goal
  MessagePlugin.success(t('reading.draftLoaded'))
}

function startRenameDraft(draft: ReadingDraft) {
  renamingDraftId.value = draft.id
  renameDraftValue.value = draft.name
}

function cancelRenameDraft() {
  renamingDraftId.value = ''
  renameDraftValue.value = ''
}

async function confirmRenameDraft(id: string) {
  const name = renameDraftValue.value.trim()
  if (!name) return
  const existing = drafts.value.find((item) => item.id === id)
  try {
    const response = await updateReadingDraft(id, {
      name,
      document_ids: existing?.documentIds,
      depth: existing?.depth,
      goal: existing?.goal,
    })
    const index = drafts.value.findIndex((item) => item.id === id)
    if (index >= 0) drafts.value[index] = mapReadingDraft(response.data)
    MessagePlugin.success(t('reading.draftRenamed'))
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.draftRenameFailed'))
  }
  cancelRenameDraft()
}

function deleteDraft(id: string) {
  const dialog = DialogPlugin.confirm({
    header: t('reading.deleteDraft'),
    body: t('reading.deleteDraftConfirm'),
    theme: 'warning',
    confirmBtn: t('common.confirm'),
    cancelBtn: t('common.cancel'),
    onConfirm: async () => {
      dialog.hide()
      try {
        await deleteReadingDraft(id)
        drafts.value = drafts.value.filter((item) => item.id !== id)
        MessagePlugin.success(t('reading.draftDeleted'))
      } catch (err: any) {
        MessagePlugin.error(err?.message || t('reading.draftDeleteFailed'))
      }
    },
    onClose: () => dialog.hide(),
  })
}

function previewSection(section: { id: string }) {
  previewSectionId.value = section.id === run.value?.current_section_id ? '' : section.id
  mobileLeftOpen.value = false
}

function clearPreview() {
  previewSectionId.value = ''
}

async function continueFromPreview() {
  if (!run.value || !previewSectionId.value) return
  try {
    const response = await jumpReadingSection(run.value.id, previewSectionId.value)
    await persistRun(response.data)
    previewSectionId.value = ''
    MessagePlugin.success(t('reading.jumpApplied'))
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.jumpFailed'))
  }
}

function ensureReadingDocuments() {
  if (!readingDocuments.value.length && run.value && isStructured.value) {
    void loadReadingDocuments()
  }
}

function loadStoredDrawerWidth() {
  const raw = Number(localStorage.getItem('gaze_reading_drawer_width') || '')
  return Number.isFinite(raw) && raw >= 360 ? raw : 560
}

function clampDrawerWidth(value: number) {
  const half = Math.floor((readingPageRef.value?.clientWidth || window.innerWidth) / 2)
  const max = Math.max(280, Math.min(900, half))
  const min = Math.min(320, max)
  return Math.min(Math.max(min, Math.round(value)), max)
}

let drawerResizing = false
let drawerStartX = 0
let drawerStartWidth = 0
let suppressDrawerMaskClick = false

function startDrawerResize(event: PointerEvent) {
  drawerResizing = true
  drawerStartX = event.clientX
  drawerStartWidth = drawerWidth.value
  document.body.style.userSelect = 'none'
  document.body.style.cursor = 'col-resize'
  document.addEventListener('pointermove', onDrawerResize)
  document.addEventListener('pointerup', endDrawerResize, { once: true })
  event.preventDefault()
}

function onDrawerResize(event: PointerEvent) {
  if (!drawerResizing) return
  drawerWidth.value = clampDrawerWidth(drawerStartWidth + drawerStartX - event.clientX)
}

function endDrawerResize() {
  if (!drawerResizing) return
  drawerResizing = false
  drawerWidth.value = clampDrawerWidth(drawerWidth.value)
  localStorage.setItem('gaze_reading_drawer_width', String(drawerWidth.value))
  document.body.style.userSelect = ''
  document.body.style.cursor = ''
  document.removeEventListener('pointermove', onDrawerResize)
  // The pointerup that ends a drag can land on the mask and emit a click
  // there; keep that click from closing the drawer.
  suppressDrawerMaskClick = true
  window.setTimeout(() => {
    suppressDrawerMaskClick = false
  }, 0)
}

function closeDrawerFromMask() {
  if (suppressDrawerMaskClick) {
    suppressDrawerMaskClick = false
    return
  }
  originalDrawerOpen.value = false
}

function onDrawerResizeKeydown(event: KeyboardEvent) {
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
  event.preventDefault()
  const delta = event.key === 'ArrowLeft' ? 24 : -24
  drawerWidth.value = clampDrawerWidth(drawerWidth.value + delta)
  localStorage.setItem('gaze_reading_drawer_width', String(drawerWidth.value))
}

let drawerImageObserver: MutationObserver | null = null
let drawerHydrateTimer: number | null = null

function hydrateDrawerImages() {
  if (drawerHydrateTimer) window.clearTimeout(drawerHydrateTimer)
  drawerHydrateTimer = window.setTimeout(async () => {
    const root = documentPaneRef.value
    if (!root) return
    const kbId = activeDocumentKnowledgeBaseID.value
    await hydrateProtectedFileImages(
      root,
      kbId ? { mode: 'knowledgeBase', kbId } : undefined,
    )
    root.querySelectorAll<HTMLImageElement>('img').forEach((img) => {
      if (img.dataset.readingBound === '1') return
      img.dataset.readingBound = '1'
      img.addEventListener('error', () => img.classList.add('is-broken'))
      img.addEventListener('load', () => img.classList.remove('is-broken'))
    })
  }, 60)
}

function startDrawerImageWatch() {
  void nextTick(() => {
    const root = documentPaneRef.value
    if (!root) return
    drawerImageObserver?.disconnect()
    drawerImageObserver = new MutationObserver(() => hydrateDrawerImages())
    drawerImageObserver.observe(root, { childList: true, subtree: true })
    hydrateDrawerImages()
  })
}

watch(originalDrawerOpen, (open) => {
  if (open) {
    startDrawerImageWatch()
    return
  }
  stopEvidenceLocation()
  drawerImageObserver?.disconnect()
  drawerImageObserver = null
  if (drawerHydrateTimer) {
    window.clearTimeout(drawerHydrateTimer)
    drawerHydrateTimer = null
  }
})

watch(activeDocumentId, (documentId) => {
  if (pendingEvidenceLocation && pendingEvidenceLocation.knowledge_id !== documentId) {
    stopEvidenceLocation()
  }
  if (originalDrawerOpen.value) startDrawerImageWatch()
})

function openOriginal() {
  stopEvidenceLocation()
  originalSectionId.value = viewingSection.value?.id || currentSection.value?.id || ''
  drawerWidth.value = clampDrawerWidth(drawerWidth.value)
  ensureReadingDocuments()
  originalDrawerOpen.value = true
}

function questionKindLabel(question: ReadingQuestionNode) {
  if (question.kind === 'fact') return t('reading.questionKind.fact')
  if (question.kind === 'decision') return t('reading.questionKind.decision')
  if (question.gap_type && question.gap_type !== 'unclassified') {
    const key = `reading.gapTypes.${question.gap_type}`
    const translated = t(key)
    if (translated !== key) return translated
  }
  return t('reading.questionCardTag')
}

function selectAnswer(id: string) {
  selectedAnswerId.value = id
  customAnswerMode.value = false
  answerDraft.value = ''
}

function enableCustomAnswer() {
  customAnswerMode.value = true
  selectedAnswerId.value = ''
  void nextTick(() => answerTextareaRef.value?.focus())
}

function resetAnswerState() {
  inlineAnswerSubmitted.value = false
  answeredQuestionSnapshot.value = null
  selectedAnswerId.value = ''
  customAnswerMode.value = false
  answerDraft.value = ''
  answerTextareaRef.value = null
}

async function submitInlineAnswer() {
  const question = currentPendingQuestion.value
  if (!run.value || !question || !answerReady.value || answering.value) return
  const selected = answerOptions.value.find((option) => option.id === selectedAnswerId.value)
  const text = customAnswerMode.value ? answerDraft.value.trim() : selected?.text || ''
  if (!text) return
  answering.value = true
  try {
    answeredQuestionSnapshot.value = question
    const response = await feedbackReadingRun(run.value.id, {
      feedback: text,
      question_id: question.id,
      option_id: customAnswerMode.value ? '' : selectedAnswerId.value,
    })
    await persistRun(response.data)
    answeredQuestionSnapshot.value = (response.data.questions || [])
      .find((item) => item.id === question.id) || {
      ...question,
      answer: text,
      status: 'answered',
    }
    inlineAnswerSubmitted.value = true
  } catch (err: any) {
    answeredQuestionSnapshot.value = null
    MessagePlugin.error(err?.message || t('reading.feedbackFailed'))
  } finally {
    answering.value = false
  }
}

async function retryQuestionJudgment() {
  const question = answeredQuestionSnapshot.value
  if (!run.value || !question || answering.value) return
  answering.value = true
  try {
    const response = await judgeReadingQuestion(run.value.id, question.id)
    await persistRun(response.data)
    answeredQuestionSnapshot.value = (response.data.questions || [])
      .find((item) => item.id === question.id) || question
    MessagePlugin.success(t('reading.judgmentCompleted'))
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.judgmentFailed'))
  } finally {
    answering.value = false
  }
}

async function skipInlineQuestion() {
  const question = currentPendingQuestion.value
  if (!run.value || !question || answering.value) return
  answering.value = true
  try {
    const response = await feedbackReadingRun(run.value.id, {
      skip: true,
      question_id: question.id,
    })
    await persistRun(response.data)
    resetAnswerState()
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.feedbackFailed'))
  } finally {
    answering.value = false
  }
}

function continueAfterAnswer() {
  resetAnswerState()
  if (currentPendingQuestion.value) return
  void advanceUntilPause()
}

async function decideStage(decision: 'confirmed' | 'corrected' | 'skipped') {
  if (!run.value) return
  if (decision === 'corrected') {
    stageCorrecting.value = true
    stageDecision.value = ''
    return
  }
  try {
    const response = await updateReadingUnderstanding(run.value.id, { decision })
    await persistRun(response.data)
    stageDecision.value = decision
    stageCorrecting.value = false
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.stageUpdateFailed'))
  }
}

function resetStageDecision() {
  stageDecision.value = ''
  stageCorrecting.value = false
  stageCorrection.value = ''
}

async function submitStageCorrection() {
  if (!run.value || !stageCorrection.value.trim()) return
  try {
    const response = await updateReadingUnderstanding(run.value.id, {
      decision: 'corrected',
      correction: stageCorrection.value.trim(),
    })
    await persistRun(response.data)
    stageCorrecting.value = false
    stageDecision.value = 'corrected'
    MessagePlugin.success(t('reading.correctionRecorded'))
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.stageUpdateFailed'))
  }
}

function cancelStageCorrection() {
  stageCorrecting.value = false
  stageCorrection.value = ''
}

function markContradictionUncertain(item: ReadingContradiction) {
  ;(item as unknown as Record<string, any>).resolution = t('reading.uncertainResolution')
  MessagePlugin.info(t('reading.markedUncertain'))
}

async function updateNoteStatus(note: NoteItem, status: 'candidate' | 'confirmed' | 'rejected') {
  if (!run.value) return
  try {
    if (note.type === 'term') {
      const response = await updateReadingTerm(run.value.id, String(note.raw.id), { status })
      Object.assign(note.raw, response.data)
    } else if (note.type === 'conclusion') {
      const response = await updateReadingConclusion(run.value.id, String(note.raw.id), { status })
      Object.assign(note.raw, response.data)
    }
    return true
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.noteUpdateFailed'))
    return false
  }
}

async function confirmNote(note: NoteItem) {
  if (!(await updateNoteStatus(note, 'confirmed'))) return
  if (note.raw.gates && typeof note.raw.gates === 'object') {
    note.raw.gates.user_adjudicated = true
  }
  MessagePlugin.success(t('reading.noteConfirmed'))
}

async function rejectNote(note: NoteItem) {
  if (!(await updateNoteStatus(note, 'rejected'))) return
  MessagePlugin.success(t('reading.noteRejected'))
}

async function restoreNote(note: NoteItem) {
  if (!(await updateNoteStatus(note, 'candidate'))) return
  MessagePlugin.success(t('reading.noteRestored'))
}

function startNoteEdit(note: NoteItem) {
  editingNoteKey.value = note.key
  editingNoteText.value = note.type === 'term'
    ? String(note.raw.definition || '')
    : String(note.raw.text || '')
}

function cancelNoteEdit() {
  editingNoteKey.value = ''
  editingNoteText.value = ''
}

async function saveNoteEdit(note: NoteItem) {
  const text = editingNoteText.value.trim()
  if (!text || !run.value) return
  try {
    if (note.type === 'term') {
      const response = await updateReadingTerm(run.value.id, String(note.raw.id), { text })
      Object.assign(note.raw, response.data)
    } else if (note.type === 'conclusion') {
      const response = await updateReadingConclusion(run.value.id, String(note.raw.id), { text })
      Object.assign(note.raw, response.data)
    }
    cancelNoteEdit()
    MessagePlugin.success(t('reading.noteSaved'))
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.noteUpdateFailed'))
  }
}

function handleExportMenu(item: { value?: string }) {
  const format = item?.value === 'json' ? 'json' : 'markdown'
  void exportReport(format)
}

async function pauseRun() {
  if (!run.value) return
  try {
    const response = await stopReadingRun(run.value.id)
    await persistRun(response.data)
    MessagePlugin.success(t('reading.pausedToast'))
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.stopFailed'))
  }
}

function abandonRun() {
  if (!run.value) return
  const current = run.value
  const dialog = DialogPlugin.confirm({
    header: t('reading.abandonTitle'),
    body: t('reading.abandonBody'),
    theme: 'warning',
    confirmBtn: t('reading.stop'),
    cancelBtn: t('common.cancel'),
    onConfirm: () => {
      dialog.hide()
      void (async () => {
        try {
          const response = await abandonReadingRun(current.id)
          await persistRun(response.data)
          MessagePlugin.success(t('reading.abandonedToast'))
        } catch (err: any) {
          MessagePlugin.error(err?.message || t('reading.stopFailed'))
        }
      })()
    },
    onClose: () => dialog.hide(),
  })
}

onMounted(async () => {
  window.addEventListener('scroll', hideEvidenceOnScroll, true)
  window.addEventListener('resize', onWindowResize)
  if (readingPageRef.value) {
    readingPageObserver = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width || 0
      pageNarrow.value = width > 0 && width < 1100
      if (width > 0) {
        drawerWidth.value = clampDrawerWidth(drawerWidth.value)
        rightWidth.value = clampRightWidth(rightWidth.value)
      }
    })
    readingPageObserver.observe(readingPageRef.value)
  }
  configView.value = queryValue(route.query.view) === 'config'
  await loadKnowledgeBases()
  await loadDocuments()
  void loadDrafts()
  const runId = queryValue(route.query.run_id)
  if (runId) {
    try {
      const response = await getReadingRun(runId)
      run.value = response.data
      localStorage.setItem(storageKey, response.data.id)
      void loadReadingDocuments()
      return
    } catch {
      localStorage.removeItem(storageKey)
    }
  }
  const activeId = localStorage.getItem(storageKey)
  if (activeId && !configView.value) {
    try {
      const response = await getReadingRun(activeId)
      run.value = response.data
      void loadReadingDocuments()
      return
    } catch {
      localStorage.removeItem(storageKey)
    }
  }
  const kb = queryValue(route.query.kb)
  if (kb) {
    kbId.value = kb
    const docs = queryValue(route.query.docs).split(',').map((id) => id.trim()).filter(Boolean)
    const available = new Set(documentOptions.value.map((option) => option.value))
    documentIds.value = docs.filter((id) => available.has(id)).slice(0, 5)
  }
})

watch(
  () => run.value?.current_section_id,
  () => {
    if (run.value && isStructured.value) void loadReadingDocuments()
  },
)

onBeforeUnmount(() => {
  window.removeEventListener('scroll', hideEvidenceOnScroll, true)
  window.removeEventListener('resize', onWindowResize)
  window.removeEventListener('keydown', onReportOverlayKeydown)
  stopAnnotationPopoverTracking()
  readingPageObserver?.disconnect()
  readingPageObserver = null
  drawerImageObserver?.disconnect()
  drawerImageObserver = null
  if (drawerHydrateTimer) window.clearTimeout(drawerHydrateTimer)
  if (evidenceCloseTimer) window.clearTimeout(evidenceCloseTimer)
  document.body.style.userSelect = ''
  document.body.style.cursor = ''
  if (autoFullscreenApplied) {
    autoFullscreenApplied = false
    shellUi.setFullscreen(false)
  }
})
</script>

<style scoped lang="less">
.reading-page {
  --rv-text: #37352f;
  --rv-text-2: #787774;
  --rv-text-3: #9b9a97;
  --rv-bg: #ffffff;
  --rv-side: #faf9f7;
  --rv-hair: var(--w-hairline, #ebebea);
  --rv-hover: #f1efea;
  --rv-green: var(--w-page-accent, #07c05f);
  --rv-green-deep: var(--w-page-accent-active, #0b9e50);
  --rv-green-bg: var(--w-page-accent-soft, #e9f8ec);
  --rv-sage: #e9efe7;
  --rv-sage-d: #4e7a6c;
  --rv-haze: #e7edf1;
  --rv-haze-d: #5d7b8a;
  --rv-oat: #f2ede3;
  --rv-oat-d: #8a7a5c;
  --rv-clay: #f4eae8;
  --rv-clay-d: #a9766e;
  --rv-clay-deep: #f8e3df;
  --rv-mauve: #f1edf4;
  --rv-mauve-d: #8e7396;
  --rv-yellow: #c29343;
  --rv-r: 8px;
  --rv-r-pop: 12px;
  --rv-shadow-pop: 0 4px 16px rgba(15, 15, 15, 0.13);

  position: relative;
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  width: 100%;
  min-height: 0;
  height: 100%;
  overflow: hidden;
  font-family: inherit;
  font-size: 14px;
  line-height: 1.65;
  color: var(--rv-text);
  background: var(--rv-bg);
}

.reading-page *,
.reading-page *::before,
.reading-page *::after {
  box-sizing: border-box;
}

:where(.reading-page) button {
  padding: 0;
  font-family: inherit;
  font-size: 13px;
  color: inherit;
  cursor: pointer;
  background: none;
  border: 0;
  border-radius: var(--rv-r);
  transition: background 0.13s ease, color 0.13s ease;
}

.rv-topbar {
  display: flex;
  flex: none;
  gap: 14px;
  align-items: center;
  height: 48px;
  margin: 0 24px;
  padding: 0 16px;
  background: transparent;
  border-bottom: 1px solid var(--rv-hair);
}

.rv-topbar__doc {
  display: flex;
  gap: 10px;
  align-items: baseline;
  min-width: 0;
}

.rv-topbar__title {
  overflow: hidden;
  font-size: 16px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rv-topbar__meta {
  overflow: hidden;
  font-size: 12px;
  color: var(--rv-text-3);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rv-switch {
  display: flex;
  flex: none;
  gap: 2px;
  margin-left: 6px;
}

.rv-switch button {
  padding: 3px 10px;
  color: var(--rv-text-2);
}

.rv-switch button.on {
  font-weight: 500;
  color: var(--rv-text);
  background: var(--rv-side);
}

.rv-topbar__spacer {
  flex: 1;
}

.rv-progress {
  display: flex;
  flex: none;
  gap: 10px;
  align-items: center;
}

.rv-progress__bar {
  display: block;
  width: 140px;
  height: 3px;
  overflow: hidden;
  background: var(--rv-side);
  border-radius: 2px;
}

.rv-progress__bar i {
  display: block;
  height: 100%;
  background: var(--rv-green);
  transition: width 0.45s ease;
}

.rv-progress__text {
  font-size: 12px;
  color: var(--rv-text-3);
  white-space: nowrap;
}

.rv-actions {
  display: flex;
  flex: none;
  gap: 4px;
  align-items: center;
}

.rv-action-btn {
  padding: 4px 10px;
  color: var(--rv-text-2);
}

.rv-action-btn:hover {
  color: var(--rv-text);
  background: var(--rv-hover);
}

.rv-action-btn.is-warn {
  color: #b4544d;
}

.rv-mobile-toggle {
  display: none;
}

.rv-primary {
  padding: 8px 22px;
  font-size: 13.5px;
  font-weight: 500;
  color: #fff;
  background: var(--rv-green-deep);
}

.rv-primary:hover {
  background: #0a8c47;
}

.rv-primary:disabled {
  color: var(--rv-text-3);
  cursor: default;
  background: var(--rv-side);
}

.rv-primary--small {
  padding: 5px 14px;
  font-size: 12.5px;
}

.rv-text-btn {
  padding: 5px 10px;
  color: var(--rv-text-2);
}

.rv-text-btn:hover {
  color: var(--rv-text);
  background: var(--rv-hover);
}

.rv-hint {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--rv-text-3);
}

.rv-empty {
  margin: 4px 0;
  font-size: 12.5px;
  color: var(--rv-text-3);
}

.rv-field,
.rv-textarea {
  width: 100%;
  font-family: inherit;
  font-size: 13.5px;
  line-height: 1.6;
  color: var(--rv-text);
  background: var(--rv-side);
  border: 0;
  border-radius: var(--rv-r);
  outline: none;
  transition: background 0.13s ease;
}

.rv-field {
  height: 34px;
  padding: 0 12px;
}

.rv-textarea {
  padding: 9px 12px;
  resize: vertical;
}

.rv-field:focus,
.rv-textarea:focus {
  background: var(--rv-hover);
}

.rv-field::placeholder,
.rv-textarea::placeholder {
  color: var(--rv-text-3);
}

/* config */
.rv-config {
  flex: 1;
  overflow-y: auto;
  background: var(--rv-bg);
}

.rv-config__inner {
  width: min(600px, calc(100% - 48px));
  padding: 56px 0 80px;
  margin: 0 auto;
}

.rv-config__inner h1 {
  margin: 0 0 6px;
  font-size: 22px;
  font-weight: 700;
  line-height: 1.3;
}

.rv-config__sub {
  margin: 0 0 34px;
  font-size: 13px;
  color: var(--rv-text-3);
}

.rv-section {
  margin-bottom: 30px;
}

.rv-section__label {
  margin-bottom: 10px;
  font-size: 13px;
  font-weight: 600;
}

.rv-doc-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-height: 260px;
  margin-top: 8px;
  overflow-y: auto;
}

.rv-doc-list--dialog {
  max-height: 320px;
  margin-top: 12px;
}

.rv-doc-row {
  display: flex;
  flex: none;
  gap: 10px;
  align-items: baseline;
  width: 100%;
  padding: 8px 12px;
  text-align: left;
  border-radius: var(--rv-r);
}

.rv-doc-row:hover {
  background: var(--rv-hover);
}

.rv-doc-row.on {
  background: var(--rv-green-bg);
}

.rv-doc-row__name {
  overflow: hidden;
  font-size: 13.5px;
  font-weight: 500;
  color: var(--rv-text);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rv-doc-row.on .rv-doc-row__name {
  color: var(--rv-green-deep);
}

.rv-doc-row__meta {
  margin-left: auto;
  overflow: hidden;
  font-size: 12px;
  color: var(--rv-text-3);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rv-doc-tree {
  display: flex;
  flex-direction: column;
  gap: 1px;
  max-height: 300px;
  margin-top: 8px;
  overflow-y: auto;
  scrollbar-width: none;
}

.rv-doc-tree::-webkit-scrollbar {
  display: none;
}

.rv-tree-row {
  display: flex;
  flex: none;
  gap: 6px;
  align-items: baseline;
  width: 100%;
  padding-top: 6px;
  padding-right: 10px;
  padding-bottom: 6px;
  overflow: hidden;
  font-size: 13px;
  color: var(--rv-text-2);
  text-align: left;
  border-radius: var(--rv-r);
}

.rv-tree-row:hover {
  background: var(--rv-hover);
}

.rv-tree-row.is-doc {
  color: var(--rv-text);
}

.rv-tree-row.is-doc.on {
  color: var(--rv-green-deep);
  background: var(--rv-green-bg);
}

.rv-tree-row__caret {
  flex: none;
  width: 12px;
  font-size: 10px;
  color: var(--rv-text-3);
}

.rv-tree-row__label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rv-tree-row.is-doc .rv-tree-row__label {
  font-size: 13px;
}

.rv-tree-row__count {
  margin-left: auto;
  font-size: 11px;
  color: var(--rv-text-3);
}

.rv-tree-row__check {
  flex: none;
  margin-left: 8px;
  font-size: 11px;
  color: var(--rv-green-deep);
}

.rv-selected-docs {
  padding: 12px;
  margin-top: 12px;
  background: var(--rv-side);
  border-radius: var(--rv-r);
}

.rv-selected-docs__label {
  margin-bottom: 8px;
  font-size: 12px;
  font-weight: 500;
  color: var(--rv-text-2);
}

.rv-selected-docs__list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.rv-selected-doc {
  display: inline-flex;
  gap: 6px;
  align-items: baseline;
  max-width: 100%;
  padding: 4px 8px;
  background: var(--rv-bg);
  border-radius: var(--rv-r);
}

.rv-selected-doc:hover {
  background: var(--rv-hover);
}

.rv-selected-doc__name {
  overflow: hidden;
  font-size: 12px;
  color: var(--rv-text);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rv-selected-doc__kb {
  font-size: 10.5px;
  color: var(--rv-text-3);
  white-space: nowrap;
}

.rv-selected-doc__remove {
  margin-left: 2px;
  font-size: 13px;
  color: var(--rv-text-3);
}

.rv-depth-row {
  display: flex;
  gap: 10px;
  align-items: baseline;
  width: 100%;
  padding: 10px 12px;
  text-align: left;
  border-radius: var(--rv-r);
}

.rv-depth-row:hover {
  background: var(--rv-hover);
}

.rv-depth-row.on {
  background: var(--rv-green-bg);
}

.rv-depth-row__name {
  flex: none;
  font-size: 13.5px;
  font-weight: 500;
}

.rv-depth-row.on .rv-depth-row__name {
  color: var(--rv-green-deep);
}

.rv-depth-row__desc {
  font-size: 12px;
  color: var(--rv-text-3);
}

.rv-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 10px;
}

.rv-chip {
  padding: 5px 12px;
  font-size: 13px;
  color: var(--rv-text-2);
  background: var(--rv-side);
}

.rv-chip:hover {
  background: var(--rv-hover);
}

.rv-chip.on {
  font-weight: 500;
  color: var(--rv-green-deep);
  background: var(--rv-green-bg);
}

.rv-estimate {
  display: flex;
  flex-wrap: wrap;
  gap: 24px;
  padding: 12px 16px;
  margin-bottom: 28px;
  font-size: 12.5px;
  color: var(--rv-oat-d);
  background: var(--rv-oat);
  border-radius: var(--rv-r);
}

.rv-config__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  align-items: center;
}

.rv-config__note {
  margin-left: auto;
  font-size: 12px;
  color: var(--rv-text-3);
}

.rv-drafts {
  margin-top: 48px;
  border-top: 1px solid var(--rv-hair);
}

.rv-drafts__toggle {
  display: flex;
  gap: 6px;
  align-items: center;
  width: 100%;
  padding: 14px 2px;
  font-size: 12.5px;
  color: var(--rv-text-2);
  text-align: left;
}

.rv-drafts__toggle span:first-of-type {
  color: var(--rv-text-3);
}

.rv-drafts__caret {
  margin-left: auto;
}

.rv-drafts__list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding-bottom: 20px;
}

.rv-draft {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 8px 10px;
  border-radius: var(--rv-r);
}

.rv-draft:hover {
  background: var(--rv-hover);
}

.rv-draft__name {
  overflow: hidden;
  font-size: 13px;
  font-weight: 500;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rv-draft__meta {
  margin-left: auto;
  overflow: hidden;
  font-size: 11.5px;
  color: var(--rv-text-3);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rv-draft__action {
  flex: none;
  padding: 3px 8px;
  font-size: 12px;
  color: var(--rv-text-2);
}

.rv-draft__action:hover {
  color: var(--rv-text);
  background: var(--rv-hover);
}

.rv-draft__action.is-warn {
  color: #b4544d;
}

.rv-draft__input {
  flex: 1;
  height: 28px;
  padding: 0 8px;
  font-family: inherit;
  font-size: 13px;
  color: var(--rv-text);
  background: var(--rv-bg);
  border: 1px solid var(--rv-hair);
  border-radius: var(--rv-r);
  outline: none;
}

/* workbench */
.rv-body {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.rv-left {
  flex: none;
  width: 256px;
  padding: 20px 12px;
  overflow-y: auto;
  background: var(--rv-side);
  border-right: 1px solid var(--rv-hair);
}

.rv-left__head h2 {
  margin: 0 0 4px;
  overflow: hidden;
  font-size: 14.5px;
  font-weight: 600;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rv-left__head p {
  margin: 0 0 22px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--rv-text-3);
}

.rv-left__label {
  margin: 18px 0 8px;
  font-size: 11.5px;
  color: var(--rv-text-3);
}

.rv-toc-row {
  display: flex;
  gap: 6px;
  align-items: baseline;
  width: 100%;
  padding: 6px 10px;
  overflow: hidden;
  font-size: 13px;
  color: var(--rv-text-2);
  text-align: left;
  border-radius: var(--rv-r);
}

.rv-toc-row:hover {
  background: var(--rv-hover);
}

.rv-toc-row.current {
  font-weight: 500;
  color: var(--rv-green-deep);
  background: var(--rv-green-bg);
}

.rv-toc-row.preview {
  box-shadow: inset 0 0 0 1px var(--rv-haze-d);
}

.rv-toc-row.read .rv-toc-row__num {
  color: var(--rv-sage-d);
}

.rv-toc-row.is-root {
  font-weight: 600;
  color: var(--rv-text);
}

.rv-toc-row__caret {
  flex: none;
  width: 12px;
  font-size: 10px;
  color: var(--rv-text-3);
  text-align: center;
}

.rv-toc-row__num {
  flex: none;
  font-size: 12px;
  color: var(--rv-text-3);
  opacity: 0.85;
}

.rv-toc-row__text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rv-toc-row__mark {
  flex: none;
  margin-left: auto;
  font-size: 10.5px;
  opacity: 0.75;
}

.rv-session-stats {
  padding-left: 2px;
  font-size: 12.5px;
  line-height: 1.9;
  color: var(--rv-text-2);
}

.rv-session-stats p {
  margin: 0;
}

.rv-left__add {
  margin-top: 20px;
  padding-left: 2px;
}

.rv-main {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  background: var(--rv-bg);
}

.rv-read {
  width: 100%;
  max-width: 680px;
  padding: 36px 40px 120px;
  margin: 0 auto;
}

.rv-chapter-head {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 30px;
}

.rv-chapter-head h2 {
  margin: 0 0 6px;
  font-size: 19px;
  font-weight: 700;
  line-height: 1.4;
}

.rv-chapter-meta {
  margin: 0;
  font-size: 12.5px;
  color: var(--rv-text-3);
}

.rv-chapter-actions {
  display: flex;
  flex: none;
  gap: 2px;
}

.rv-block {
  margin-bottom: 34px;
}

.rv-block-label {
  margin-bottom: 12px;
  font-size: 13px;
  font-weight: 600;
  color: var(--rv-text-2);
}

.rv-prose {
  font-size: 15px;
  line-height: 1.75;
  color: var(--rv-text);
  white-space: pre-wrap;
}

.rv-prose p {
  margin: 0 0 12px;
  white-space: pre-wrap;
}

.rv-prose p.is-highlight {
  padding: 7px 12px;
  margin: -1px -12px 12px;
  background: var(--rv-sage);
  border-radius: var(--rv-r);
  box-decoration-break: clone;
  -webkit-box-decoration-break: clone;
}

.rv-cite {
  position: relative;
  padding: 0 2px;
  font-size: 10.5px;
  font-weight: 600;
  color: var(--rv-green-deep);
  vertical-align: super;
}

.rv-cite:hover {
  text-decoration: underline;
}

.rv-conclusion + .rv-conclusion {
  margin-top: 18px;
}

.rv-conclusion-sources {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-top: 6px;
  font-size: 11.5px;
  line-height: 1.6;
  color: var(--rv-text-3);
}

.rv-conclusion-sources__label {
  font-weight: 500;
  color: var(--rv-text-3);
}

.rv-conclusion-source {
  align-self: flex-start;
  max-width: 100%;
  overflow: hidden;
  font-size: 11.5px;
  color: var(--rv-text-3);
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rv-conclusion-source:hover {
  color: var(--rv-text-2);
  text-decoration: underline;
}

.rv-plan-card {
  padding: 20px 22px;
  margin: 26px 0;
  background: var(--rv-oat);
  border-radius: var(--rv-r-pop);
}

.rv-plan-card h3 {
  margin: 0 0 8px;
  font-size: 15px;
  font-weight: 600;
}

.rv-plan-card p {
  margin: 0 0 12px;
  font-size: 13.5px;
  color: var(--rv-text-2);
}

.rv-plan-card__meta {
  display: flex;
  flex-direction: column;
  gap: 3px;
  margin-bottom: 16px;
  font-size: 12.5px;
  color: var(--rv-text-2);
}

.rv-plan-structure {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 16px;
}

.rv-plan-structure__row {
  padding: 10px 12px;
  background: var(--rv-bg, #fff);
  border: 1px solid var(--rv-line, #e9e7e2);
  border-radius: var(--rv-r, 6px);
}

.rv-plan-structure__row p {
  margin: 4px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--rv-text-2);
}

.rv-plan-structure__head {
  display: flex;
  gap: 8px;
  align-items: baseline;
  justify-content: space-between;
}

.rv-plan-structure__section {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--rv-text);
}

.rv-plan-structure__type {
  flex: none;
  padding: 1px 7px;
  font-size: 11px;
  color: var(--rv-text-3);
  border: 1px solid var(--rv-line, #e9e7e2);
  border-radius: 999px;
}

.rv-quiet-tag {
  margin-left: 8px;
  padding: 1px 8px;
  font-size: 11px;
  color: var(--rv-green-deep);
  background: var(--rv-mint, #e9f8ec);
  border-radius: 999px;
}

.rv-phase-trace {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.rv-phase-trace__item {
  padding: 2px 9px;
  font-size: 11.5px;
  color: var(--rv-text-3);
  border: 1px solid var(--rv-line, #e9e7e2);
  border-radius: 999px;
}

.rv-phase-trace__item.is-degraded {
  color: var(--rv-haze-d);
  border-color: var(--rv-haze-d);
}

.rv-selftests {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
}

.rv-selftests__toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.rv-selftests__group {
  min-width: 220px;
  padding: 7px 10px;
  font-size: 13px;
  color: var(--rv-text);
  background: var(--rv-bg, #fff);
  border: 1px solid var(--rv-line, #e9e7e2);
  border-radius: var(--rv-r, 6px);
}

.rv-selftest {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 12px 14px;
  background: var(--rv-bg, #fff);
  border: 1px solid var(--rv-line, #e9e7e2);
  border-radius: var(--rv-r, 6px);
  cursor: pointer;
}

.rv-selftest.is-imported {
  opacity: 0.65;
}

.rv-selftest input {
  margin-top: 3px;
}

.rv-selftest__body {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.rv-selftest__question {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--rv-text);
}

.rv-selftest__answer {
  font-size: 13px;
  color: var(--rv-text-2);
}

.rv-selftest__explanation,
.rv-selftest__evidence {
  font-size: 11.5px;
  line-height: 1.6;
  color: var(--rv-text-3);
}

.rv-selftest__status {
  align-self: flex-start;
  padding: 1px 7px;
  font-size: 11px;
  color: var(--rv-text-3);
  border: 1px solid var(--rv-line, #e9e7e2);
  border-radius: 999px;
}

.rv-final {
  display: flex;
  flex-direction: column;
  gap: 18px;
  align-items: stretch;
}

.rv-final h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
}

.rv-final__summary {
  margin: 0;
  font-size: 14.5px;
  line-height: 1.75;
  color: var(--rv-text);
}

.rv-final__summary p {
  margin: 0 0 10px;
}

.rv-final__summary p:last-child {
  margin-bottom: 0;
}

.rv-final__stats {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  font-size: 12.5px;
  color: var(--rv-text-3);
}

.rv-final__group h4 {
  margin: 0 0 8px;
  font-size: 13px;
  font-weight: 600;
}

.rv-final__list {
  padding-left: 0;
  margin: 0;
  list-style: none;
  counter-reset: rv-final-item;
}

.rv-final__list li {
  position: relative;
  padding-left: 26px;
  margin: 0 0 10px;
  font-size: 13.5px;
  line-height: 1.7;
  color: var(--rv-text-2);
  white-space: pre-wrap;
}

.rv-final__list li::before {
  position: absolute;
  top: 0;
  left: 0;
  font-size: 12px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--rv-haze-d);
  counter-increment: rv-final-item;
  content: counter(rv-final-item, decimal-leading-zero);
}

.rv-final__list li strong {
  font-weight: 600;
  color: var(--rv-text);
}

.rv-final__cites {
  padding-left: 0;
  margin: 0;
  list-style: none;
}

.rv-final__cites li {
  margin: 0 0 6px;
}

.rv-final__cite {
  padding: 0;
  font-family: inherit;
  font-size: 13.5px;
  color: var(--rv-haze-d);
  text-align: left;
  text-decoration: underline;
  text-decoration-color: color-mix(in srgb, var(--rv-haze-d) 35%, transparent);
  text-underline-offset: 3px;
  cursor: pointer;
  background: transparent;
  border: 0;
}

.rv-final__cite:hover {
  color: var(--rv-text);
  text-decoration-color: currentColor;
}

.rv-idle {
  padding: 36px 0;
}

.rv-idle p {
  margin: 0 0 16px;
  color: var(--rv-text-2);
}

.rv-preview .rv-prose {
  font-size: 14.5px;
}

.rv-preview__notes {
  margin-top: 28px;
}

.rv-preview__note {
  margin: 0 0 8px;
  font-size: 13.5px;
  color: var(--rv-text-2);
}

.rv-preview__actions {
  margin-top: 28px;
}

/* question card */
.rv-qcard {
  padding: 20px 22px;
  margin: 26px 0;
  background: var(--rv-haze);
  border-radius: var(--rv-r-pop);
}

.rv-qcard__tag {
  margin-bottom: 8px;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--rv-haze-d);
}

.rv-qcard__text {
  margin: 0 0 6px;
  font-size: 14.5px;
  line-height: 1.7;
  color: var(--rv-text);
}

.rv-qcard__hint {
  margin: 0 0 14px;
  font-size: 12.5px;
  color: var(--rv-text-3);
}

.rv-options {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.rv-option {
  display: flex;
  gap: 10px;
  align-items: baseline;
  width: 100%;
  padding: 9px 12px;
  text-align: left;
  background: rgba(255, 255, 255, 0.66);
  border-radius: var(--rv-r);
}

.rv-option:hover {
  background: #fff;
}

.rv-option.on {
  box-shadow: inset 0 0 0 1px var(--rv-haze-d);
  background: #fff;
}

.rv-option__label {
  flex: none;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--rv-haze-d);
}

.rv-option__text {
  font-size: 13px;
  line-height: 1.65;
  color: var(--rv-text-2);
}

.rv-option__badge {
  flex: none;
  margin-left: auto;
  font-size: 10.5px;
  color: var(--rv-green-deep);
}

.rv-textarea--answer {
  margin-top: 6px;
  background: #fff;
}

.rv-qcard__actions,
.rv-stage__actions,
.rv-contra__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
  margin-top: 12px;
}

.rv-verdict {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  padding: 10px 14px;
  margin-top: 12px;
  font-size: 13px;
  color: var(--rv-text-2);
  background: var(--rv-sage);
  border-radius: var(--rv-r);
}

.rv-verdict__state {
  font-weight: 600;
  color: var(--rv-green-deep);
}

.rv-verdict__reason {
  flex: 1 1 100%;
  color: var(--rv-text-2);
  line-height: 1.6;
}

/* stage confirmation */
.rv-stage {
  padding: 20px 22px;
  margin: 26px 0;
  background: var(--rv-oat);
  border-radius: var(--rv-r-pop);
}

.rv-stage__tag {
  margin-bottom: 8px;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--rv-oat-d);
}

.rv-stage__text {
  margin: 0 0 6px;
  font-size: 14px;
  line-height: 1.7;
}

.rv-stage__section {
  margin-left: 4px;
  font-size: 12px;
  color: var(--rv-text-3);
}

.rv-stage__result {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-top: 10px;
  font-size: 13px;
  color: var(--rv-sage-d);
}

/* contradiction */
.rv-contra {
  padding: 20px 22px;
  margin: 26px 0;
  background: var(--rv-clay);
  border-radius: var(--rv-r-pop);
}

.rv-contra.resolved {
  background: var(--rv-bg);
  box-shadow: inset 0 0 0 1px var(--rv-hair);
}

.rv-contra__tag {
  display: inline-block;
  padding: 2px 8px;
  margin-bottom: 10px;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--rv-clay-d);
  background: var(--rv-clay-deep);
  border-radius: var(--rv-r);
}

.rv-contra__title {
  margin: 0 0 14px;
  font-size: 14.5px;
  font-weight: 600;
}

.rv-quote {
  margin-bottom: 10px;
}

.rv-quote__label {
  margin-bottom: 4px;
  font-size: 12px;
  font-weight: 600;
  color: var(--rv-text-2);
}

.rv-quote__text {
  padding: 8px 12px;
  margin: 0 0 4px;
  font-size: 13.5px;
  line-height: 1.7;
  color: var(--rv-text-2);
  border-radius: var(--rv-r);
}

.rv-quote__text.is-a {
  background: var(--rv-haze);
}

.rv-quote__text.is-b {
  background: var(--rv-mauve);
}

.rv-contra__resolution {
  margin: 12px 0 0;
  font-size: 12.5px;
  font-weight: 600;
  color: var(--rv-green-deep);
}

.rv-round-continue {
  padding: 24px 0 8px;
}

/* notes */
.rv-right {
  position: relative;
  flex: none;
  width: 312px;
  overflow: hidden;
  background: var(--rv-side);
  border-left: 1px solid var(--rv-hair);
}

.rv-right__scroll {
  height: 100%;
  padding: 20px 16px 60px;
  overflow-y: auto;
}

.rv-right__resizer {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  z-index: 6;
  width: 8px;
  cursor: col-resize;
  touch-action: none;
}

.rv-right__resizer::after {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 1px;
  content: '';
  background: var(--rv-hair);
  transition: width 0.13s ease, background 0.13s ease;
}

.rv-right__resizer:hover::after,
.rv-right__resizer:focus-visible::after {
  width: 2px;
  background: var(--rv-green);
}

.rv-right__resizer:focus-visible {
  outline: none;
}

.rv-notes__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 14px;
  font-size: 13px;
  font-weight: 600;
}

.rv-notes__count {
  font-size: 11.5px;
  font-weight: 400;
  color: var(--rv-text-3);
}

.rv-notes__tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
  margin-bottom: 20px;
}

.rv-notes__tab {
  padding: 3px 9px;
  font-size: 12px;
  color: var(--rv-text-3);
}

.rv-notes__tab:hover {
  color: var(--rv-text-2);
  background: var(--rv-hover);
}

.rv-notes__tab.on {
  font-weight: 500;
}

.rv-notes__tab.on.is-all {
  color: var(--rv-green-deep);
  background: var(--rv-green-bg);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--rv-green) 28%, transparent);
}

.rv-notes__tab.on.is-terms {
  color: var(--rv-mauve-d);
  background: var(--rv-mauve);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--rv-mauve-d) 22%, transparent);
}

.rv-notes__tab.on.is-conclusions {
  color: var(--rv-sage-d);
  background: var(--rv-sage);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--rv-sage-d) 22%, transparent);
}

.rv-notes__tab.on.is-open {
  color: var(--rv-oat-d);
  background: var(--rv-oat);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--rv-oat-d) 22%, transparent);
}

.rv-notes__tab.on.is-contradictions {
  color: var(--rv-clay-d);
  background: var(--rv-clay-deep);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--rv-clay-d) 22%, transparent);
}

.rv-notes__tab.on .rv-notes__tab-count {
  color: inherit;
  opacity: 0.72;
}

.rv-notes__tab-count {
  margin-left: 3px;
  font-size: 10.5px;
  color: var(--rv-text-3);
}

.rv-notes__list {
  display: flex;
  flex-direction: column;
}

.rv-note {
  padding: 12px;
  margin: 0 -12px;
  border-radius: var(--rv-r);
}

.rv-note + .rv-note {
  margin-top: 6px;
}

.rv-note:hover {
  background: var(--rv-hover);
}

.rv-note.rejected {
  opacity: 0.45;
}

.rv-note.is-term {
  --rv-note-bg: var(--rv-mauve);
  --rv-note-accent: var(--rv-mauve-d);
}

.rv-note.is-conclusion {
  --rv-note-bg: var(--rv-sage);
  --rv-note-accent: var(--rv-sage-d);
}

.rv-note.is-open {
  --rv-note-bg: var(--rv-oat);
  --rv-note-accent: var(--rv-oat-d);
}

.rv-note.is-contradiction {
  --rv-note-bg: var(--rv-clay-deep);
  --rv-note-accent: var(--rv-clay-d);
}

.rv-note.is-tinted {
  background: var(--rv-note-bg);
  box-shadow: inset 2px 0 0 var(--rv-note-accent);
}

.rv-note.is-tinted:hover {
  background: var(--rv-note-bg);
  box-shadow:
    inset 2px 0 0 var(--rv-note-accent),
    inset 0 0 0 1px var(--rv-note-accent);
}

.rv-note__head {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
  margin-bottom: 6px;
}

.rv-note__type {
  display: inline-block;
  padding: 1px 7px;
  font-size: 11px;
  font-weight: 600;
  border-radius: var(--rv-r);
}

.rv-note__type.is-term {
  color: var(--rv-mauve-d);
  background: var(--rv-mauve);
}

.rv-note__type.is-conclusion {
  color: var(--rv-sage-d);
  background: var(--rv-sage);
}

.rv-note__type.is-open {
  color: var(--rv-oat-d);
  background: var(--rv-oat);
}

.rv-note__type.is-contradiction {
  color: var(--rv-clay-d);
  background: var(--rv-clay-deep);
}

.rv-note__state {
  font-size: 11px;
  color: var(--rv-text-3);
}

.rv-note__state.done {
  color: var(--rv-green-deep);
}

.rv-note__recommended {
  margin-left: auto;
  font-size: 10.5px;
  color: var(--rv-green-deep);
}

.rv-note__text {
  margin: 0 0 6px;
  font-size: 13.5px;
  line-height: 1.65;
  white-space: pre-wrap;
}

.rv-note__cite {
  margin: 0;
  font-size: 11.5px;
  color: var(--rv-text-3);
}

.rv-note__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
  margin-top: 8px;
}

.rv-note__action {
  padding: 3px 8px;
  font-size: 12px;
  color: var(--rv-text-2);
}

.rv-note__action:hover {
  color: var(--rv-text);
  background: var(--rv-hover);
}

.rv-note__action.is-main {
  font-weight: 500;
  color: var(--rv-green-deep);
}

.rv-note__action.is-danger {
  color: #b4544d;
}

.rv-mobile-head,
.rv-left__mobile-head,
.rv-right__mobile-head {
  display: none;
}

/* drawer */
.rv-drawer-mask {
  position: absolute;
  inset: 0;
  z-index: 40;
  display: flex;
  justify-content: flex-end;
  background: rgba(28, 28, 24, 0.14);
}

.rv-drawer {
  position: relative;
  display: flex;
  flex-direction: column;
  width: 560px;
  max-width: calc(100% - 48px);
  height: 100%;
  background: var(--rv-bg);
  border-left: 1px solid var(--rv-hair);
  box-shadow: -12px 0 32px rgba(15, 15, 15, 0.08);
}

.rv-drawer__resizer {
  position: absolute;
  top: 0;
  bottom: 0;
  left: -4px;
  z-index: 2;
  width: 8px;
  cursor: col-resize;
}

.rv-drawer__resizer:hover,
.rv-drawer__resizer:focus-visible {
  background: rgba(7, 192, 95, 0.18);
  outline: none;
}

.rv-drawer__head {
  display: flex;
  gap: 10px;
  align-items: center;
  height: 48px;
  padding: 0 16px;
  font-size: 13px;
  font-weight: 600;
  border-bottom: 1px solid var(--rv-hair);
}

.rv-drawer__select {
  max-width: 260px;
  height: 28px;
  padding: 0 8px;
  font-family: inherit;
  font-size: 12.5px;
  color: var(--rv-text-2);
  background: var(--rv-side);
  border: 0;
  border-radius: var(--rv-r);
  outline: none;
}

.rv-drawer__close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  padding: 0;
  margin-left: 0;
  font-size: 18px;
  line-height: 1;
  color: var(--rv-text);
}

.rv-drawer__toolbar :deep(.t-button) {
  width: 30px;
  height: 30px;
  color: var(--rv-text);
}

.rv-drawer__toolbar :deep(.t-button .t-icon) {
  color: var(--rv-text);
}

.rv-drawer__toolbar :deep(.t-button:hover) {
  color: var(--rv-text);
}

.rv-drawer__body {
  flex: 1;
  padding: 24px 28px 80px;
  overflow-y: auto;
}

.rv-drawer__body--preview {
  display: flex;
  flex-direction: column;
  padding: 0;
  overflow: hidden;
}

.rv-drawer__body--preview :deep(.document-preview) {
  flex: 1;
  min-height: 0;
  height: 100%;
}

.rv-drawer__body--preview :deep(.rv-doc-locate-mark) {
  padding: 1px 0;
  color: inherit;
  background: color-mix(in srgb, var(--rv-green) 22%, transparent);
  border-radius: 2px;
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--rv-green) 26%, transparent);
}

.rv-drawer__body--preview :deep(.rv-doc-locate-block) {
  background: var(--rv-green-bg);
  box-shadow: inset 2px 0 0 var(--rv-green);
}

.rv-drawer__body--preview :deep(img.is-broken) {
  display: none;
}

.rv-drawer__toolbar {
  display: flex;
  align-items: center;
  margin-left: auto;
}

.rv-document-content {
  font-size: 14px;
  line-height: 1.8;
  color: var(--rv-text-2);
}

.rv-document-content :deep(h1),
.rv-document-content :deep(h2),
.rv-document-content :deep(h3),
.rv-document-content :deep(h4) {
  margin: 24px 0 10px;
  font-weight: 600;
  line-height: 1.45;
  color: var(--rv-text);
}

.rv-document-content :deep(h1) {
  font-size: 20px;
}

.rv-document-content :deep(h2) {
  font-size: 17px;
}

.rv-document-content :deep(h3) {
  font-size: 15px;
}

.rv-document-content :deep(p) {
  margin: 0 0 12px;
}

.rv-document-content :deep(table) {
  width: 100%;
  margin: 14px 0;
  font-size: 12.5px;
  border-collapse: collapse;
}

.rv-document-content :deep(th),
.rv-document-content :deep(td) {
  padding: 6px 10px;
  text-align: left;
  border: 1px solid var(--rv-hair);
}

.rv-document-content :deep(img) {
  max-width: 100%;
  border-radius: 3px;
}

.rv-document-content :deep(ul),
.rv-document-content :deep(ol) {
  padding-left: 22px;
  margin: 0 0 12px;
}

.rv-document-content :deep(li) {
  margin: 2px 0;
}

.rv-document-content :deep(blockquote) {
  padding: 4px 12px;
  margin: 12px 0;
  color: var(--rv-text-2);
  border-left: 2px solid var(--rv-hair);
}

.rv-document-content :deep(pre) {
  padding: 12px 14px;
  margin: 12px 0;
  overflow-x: auto;
  font-size: 12.5px;
  line-height: 1.6;
  background: var(--rv-side);
  border-radius: var(--rv-r);
}

.rv-document-content :deep(code) {
  font-family: var(--app-font-family-mono, Consolas, monospace);
  font-size: 0.92em;
}

.rv-document-content :deep(a) {
  color: var(--rv-green-deep);
}

.rv-document-content :deep(hr) {
  margin: 20px 0;
  border: 0;
  border-top: 1px solid var(--rv-hair);
}

/* dialogs */
.reading-add-dialog {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.rv-report-page {
  position: absolute;
  inset: 0;
  z-index: 240;
  display: flex;
  flex-direction: column;
  color: var(--rv-text);
  background: var(--rv-bg);
}

.rv-report-page__head {
  display: flex;
  flex: none;
  gap: 18px;
  align-items: center;
  height: 56px;
  padding: 0 24px;
  background: var(--rv-bg);
  border-bottom: 1px solid var(--rv-hair);
}

.rv-report-page__head h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.rv-report-page__actions {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-left: auto;
}

.rv-report-page__body {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.rv-report-page__inner {
  display: flex;
  flex-direction: column;
  width: min(1440px, calc(100% - 48px));
  height: 100%;
  padding: 18px 0 24px;
  margin: 0 auto;
}

.rv-report-page__hint {
  flex: none;
  margin: 0 0 10px;
}

.rv-report-md {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  background: var(--rv-bg);
  border: 1px solid var(--rv-hair);
  border-radius: var(--rv-r);
}

.rv-report-md.is-fullscreen {
  position: absolute;
  inset: 0;
  z-index: 60;
  border: 0;
  border-radius: 0;
}

.rv-report-md__head {
  display: flex;
  flex: none;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  padding: 10px 16px;
  background: var(--rv-side);
  border-bottom: 1px solid var(--rv-hair);
}

.rv-report-md__title {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.5px;
  color: var(--rv-text-2);
}

.rv-report-md__count {
  font-size: 11.5px;
  color: var(--rv-text-3);
}

.rv-report-md__modes {
  display: flex;
  margin-left: auto;
  overflow: hidden;
  background: var(--rv-bg);
  border: 1px solid var(--rv-hair);
  border-radius: var(--rv-r);
}

.rv-report-md__modes button {
  padding: 4px 12px;
  font-family: inherit;
  font-size: 12px;
  color: var(--rv-text-3);
  cursor: pointer;
  background: none;
  border: 0;
  border-right: 1px solid var(--rv-hair);
}

.rv-report-md__modes button:last-child {
  border-right: 0;
}

.rv-report-md__modes button.active {
  font-weight: 600;
  color: #f7f4ec;
  background: var(--rv-haze-d);
}

.rv-report-md__fullscreen {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 24px;
  padding: 0;
  color: var(--rv-text-2);
  cursor: pointer;
  background: var(--rv-bg);
  border: 1px solid var(--rv-hair);
  border-radius: var(--rv-r);
}

.rv-report-md__fullscreen:hover {
  color: var(--rv-text);
  background: var(--rv-hover);
}

.rv-report-md__body {
  display: grid;
  flex: 1;
  grid-template-columns: 1fr 1fr;
  min-height: 0;
  overflow: hidden;
}

.rv-report-md__body.is-edit {
  grid-template-columns: 1fr;
}

.rv-report-md__body.is-edit .rv-report-pv-pane {
  display: none;
}

.rv-report-md__body.is-preview {
  grid-template-columns: 1fr;
}

.rv-report-md__body.is-preview .rv-report-ed-pane {
  display: none;
}

.rv-report-ed-pane {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.rv-report-page__input {
  flex: 1;
  width: 100%;
  min-height: 0;
  padding: 20px 22px;
  font-family: var(--app-font-family-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 13.5px;
  line-height: 1.9;
  color: var(--rv-text);
  resize: none;
  background: var(--rv-bg);
  border: 0;
  outline: none;
}

.rv-report-pv-pane {
  height: 100%;
  min-height: 0;
  padding: 20px 24px;
  overflow-y: auto;
  background: var(--rv-side);
  border-left: 1px dashed var(--rv-hair);
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.rv-report-pv-pane::-webkit-scrollbar {
  display: none;
  width: 0;
  height: 0;
}

.rv-report-md__body.is-preview .rv-report-pv-pane {
  border-left: 0;
}

.rv-report-page .reading-report-preview {
  max-height: none;
  overflow: visible;
}

.reading-report-editor__tabs {
  display: flex;
  gap: 6px;
  align-self: flex-start;
  padding: 3px;
  border: 1px solid var(--rv-hair);
  border-radius: var(--rv-r);

  button {
    padding: 5px 14px;
    border: 0;
    border-radius: calc(var(--rv-r) - 2px);
    background: transparent;
    color: var(--rv-text-2);
    font-size: 13px;
    cursor: pointer;

    &.on {
      background: var(--rv-green-bg);
      color: var(--rv-green-deep);
      font-weight: 600;
    }
  }
}

.reading-report-preview {
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-height: min(62vh, 640px);
  overflow-y: auto;
  padding-right: 4px;
}

.reading-report-preview__markdown {
  color: var(--rv-text);
  font-size: 14px;
  line-height: 1.75;
  overflow-wrap: anywhere;

  h1,
  h2,
  h3,
  h4 {
    margin: 18px 0 8px;
    line-height: 1.4;
  }

  h1 { font-size: 22px; }
  h2 { font-size: 18px; }
  h3 { font-size: 16px; }

  p,
  ul,
  ol,
  blockquote,
  pre,
  table {
    margin: 8px 0;
  }

  ul,
  ol {
    padding-left: 22px;
  }

  code {
    padding: 1px 5px;
    border-radius: 4px;
    background: var(--rv-hover);
    font-family: var(--app-font-family-mono);
    font-size: 12.5px;
  }

  pre {
    overflow: auto;
    padding: 12px 14px;
    border: 1px solid var(--rv-hair);
    border-radius: var(--rv-r);
    background: var(--rv-side);

    code {
      padding: 0;
      background: none;
    }
  }

  table {
    border-collapse: collapse;

    th,
    td {
      padding: 6px 10px;
      border: 1px solid var(--rv-hair);
    }
  }

  blockquote {
    margin-left: 0;
    padding-left: 12px;
    border-left: 3px solid var(--rv-hair);
    color: var(--rv-text-2);
  }
}

.rv-argument {
  margin: 18px 0;
}

.rv-argument__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
}

.rv-argument__canvas {
  overflow-x: auto;
  padding: 12px;
  border: 1px solid var(--rv-hair);
  border-radius: var(--rv-r);
  background: var(--rv-side);
  cursor: zoom-in;

  :deep(svg) {
    display: block;
    width: 100%;
    max-width: 100%;
    height: auto;
  }

  :deep(g.node) {
    cursor: pointer;
  }

  :deep(g.node:hover) {
    filter: brightness(0.97);
  }
}

.rv-dialog-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 8px;
}

/* evidence float */
.rv-evidence-float {
  position: fixed;
  z-index: 60;
  width: 420px;
  max-width: calc(100vw - 24px);
  padding: 12px 14px;
  font-size: 12.5px;
  line-height: 1.65;
  color: var(--rv-text-2);
  background: #fff;
  border-radius: var(--rv-r-pop);
  box-shadow: var(--rv-shadow-pop);
}

.rv-evidence-float__title {
  margin-bottom: 6px;
  font-size: 12px;
  font-weight: 600;
  color: var(--rv-text);
}

.rv-evidence-float__muted {
  color: var(--rv-text-3);
}

.rv-evidence-float__error {
  color: #b4544d;
}

.rv-evidence-float__body {
  max-height: 240px;
  overflow-y: auto;
}

.rv-evidence-float__body .is-hit {
  color: var(--rv-text);
  background: var(--rv-sage);
}

.rv-evidence-float__hint {
  margin-top: 8px;
  font-size: 11.5px;
  color: var(--rv-text-3);
}

/* hide scrollbars, keep scrolling */
.rv-left,
.rv-right,
.rv-right__scroll,
.rv-main,
.rv-config,
.rv-drawer__body,
.rv-doc-list,
.rv-evidence-float__body {
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.rv-left::-webkit-scrollbar,
.rv-right::-webkit-scrollbar,
.rv-right__scroll::-webkit-scrollbar,
.rv-main::-webkit-scrollbar,
.rv-config::-webkit-scrollbar,
.rv-drawer__body::-webkit-scrollbar,
.rv-doc-list::-webkit-scrollbar,
.rv-evidence-float__body::-webkit-scrollbar {
  display: none;
}

/* Light mode follows the shared detail appearance: the configured canvas
   colour and transparency show through the workbench surfaces, and the three
   text levels use the settings overrides. Dark mode keeps its own tokens. */
:global(html[theme-mode="light"] .reading-page) {
  --rv-text: var(--w-detail-text-primary, #37352f);
  --rv-text-2: var(--w-detail-text-secondary, #787774);
  --rv-text-3: var(--w-detail-text-placeholder, #9b9a97);
}

:global(html[theme-mode="light"] .reading-page .rv-topbar),
:global(html[theme-mode="light"] .reading-page .rv-main),
:global(html[theme-mode="light"] .reading-page .rv-config) {
  background: transparent;
}

:global(html[theme-mode="light"] .reading-page .rv-left),
:global(html[theme-mode="light"] .reading-page .rv-right) {
  background: rgba(0, 0, 0, 0.02);
}

.rv-notes__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.rv-annotations {
  margin: 8px 0 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--rv-hair);
}

.rv-annotations__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--rv-text-2);
}

.rv-annotation-highlight {
  padding: 0 1px;
  color: inherit;
  cursor: pointer;
  background: color-mix(in srgb, var(--rv-yellow) 24%, transparent);
  border-bottom: 1.5px solid var(--rv-yellow);
}

.rv-annotation-pin {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  padding: 0;
  margin: 0 2px;
  color: var(--rv-oat-d);
  vertical-align: -3px;
  cursor: pointer;
  background: var(--rv-oat);
  border: 1px solid color-mix(in srgb, var(--rv-yellow) 45%, transparent);
  border-radius: 3px;
}

.rv-annotation-pin:hover {
  color: var(--rv-text);
  border-color: var(--rv-yellow);
}

.rv-annotation-popover {
  position: fixed;
  z-index: 120;
  width: 300px;
  padding: 12px 14px;
  color: var(--rv-text);
  background: var(--rv-bg);
  border: 1px solid var(--rv-hair);
  border-radius: var(--rv-r);
  box-shadow: var(--rv-shadow-pop);
}

.rv-annotation-popover__close {
  position: absolute;
  top: 6px;
  right: 8px;
  padding: 0;
  font-size: 16px;
  color: var(--rv-text-3);
  cursor: pointer;
  background: transparent;
  border: 0;
}

.rv-annotation-popover__comment {
  margin: 0 20px 0 0;
  font-size: 13.5px;
  line-height: 1.65;
  color: var(--rv-text);
  white-space: pre-wrap;
}

.rv-annotation-popover__comment.is-empty {
  color: var(--rv-text-3);
}

.rv-annotation-popover__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 10px;
}

.rv-annotation {
  margin-top: 8px;
}

.rv-annotation__quote {
  display: block;
  width: 100%;
  padding: 8px 10px;
  border: 1px solid var(--rv-hair);
  border-radius: var(--rv-r);
  background: var(--rv-side);
  color: var(--rv-text);
  font-size: 13px;
  line-height: 1.6;
  text-align: left;
  cursor: pointer;
}

.rv-annotation__comment {
  margin: 6px 2px 0;
  color: var(--rv-text-2);
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
}

.rv-annotation__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 6px;
}

.rv-feynman {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.rv-feynman__angles {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 52vh;
  padding-right: 4px;
  overflow-y: auto;
  scrollbar-width: none;
}

.rv-feynman__angles::-webkit-scrollbar {
  display: none;
}

.rv-feynman__angle {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.rv-feynman__angle-head {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: baseline;
}

.rv-feynman__angle-head b {
  font-size: 14px;
  color: var(--rv-text);
}

.rv-feynman__angle-head small {
  font-size: 12.5px;
  line-height: 1.5;
  color: var(--rv-text-3);
}

.rv-feynman__angle .rv-textarea {
  min-height: 60px;
}

.rv-feynman__freeform {
  padding-top: 10px;
  border-top: 1px dashed var(--rv-hair);
}

.rv-feynman__answer {
  margin-top: 8px;
  padding-top: 6px;
  border-top: 1px solid var(--rv-hair);
}

.rv-feynman__answer summary {
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  color: var(--rv-text);
}

.rv-feynman__answer > p {
  margin: 4px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--rv-text-2);
  white-space: pre-wrap;
}

.rv-feynman__angle-feedback {
  margin-top: 5px;
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--rv-text-2);
}

.rv-feynman__angle-feedback p {
  margin: 2px 0 0;
}

.rv-feynman__angle-status {
  display: inline-block;
  margin-right: 6px;
  padding: 0 6px;
  font-size: 11.5px;
  border-radius: 2px;
  color: var(--rv-green-d);
  background: var(--rv-sage);
}

.rv-feynman__angle-feedback.is-partial .rv-feynman__angle-status {
  color: var(--rv-yellow);
  background: var(--rv-oat);
}

.rv-feynman__angle-feedback.is-missing .rv-feynman__angle-status,
.rv-feynman__angle-feedback.is-skipped .rv-feynman__angle-status {
  color: var(--rv-clay-d);
  background: var(--rv-clay);
}

.rv-feynman__history {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 6px;
}

.rv-feynman__result {
  padding: 12px 14px;
  border: 1px solid var(--rv-hair);
  border-radius: var(--rv-r);
  background: var(--rv-side);
}

.rv-feynman__score {
  display: flex;
  align-items: baseline;
  gap: 6px;
  color: var(--rv-text-3);

  strong {
    font-size: 24px;
    color: var(--rv-green);
  }

  em {
    margin-left: auto;
    max-width: 65%;
    color: var(--rv-text-2);
    font-size: 13px;
    font-style: normal;
    line-height: 1.5;
  }
}

.rv-feynman__dims {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 4px;
  color: var(--rv-text-2);
  font-size: 12px;
}

.rv-feynman__points {
  margin-top: 8px;
  color: var(--rv-text-2);
  font-size: 13px;

  b {
    color: var(--rv-text);
  }

  p {
    margin: 2px 0 0;
    line-height: 1.6;
  }

  &.is-error b {
    color: var(--rv-clay-d);
  }
}

.rv-annotation-dialog,
.rv-annotation-card-dialog {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.rv-annotation-dialog__quote {
  padding: 10px 12px;
  border-left: 3px solid var(--rv-green);
  background: var(--rv-side);
  color: var(--rv-text-2);
  font-size: 13px;
  line-height: 1.7;
  white-space: pre-wrap;
}

.rv-field-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--rv-text-2);
}

/* dark */
:global(html[theme-mode="dark"] .reading-page) {
  --rv-text: var(--w-ink, #f3efe8);
  --rv-text-2: var(--w-body, #c8c1b5);
  --rv-text-3: var(--w-muted, #978e80);
  --rv-bg: var(--w-window, #191714);
  --rv-side: var(--w-surface, #211e1a);
  --rv-hair: var(--w-hairline, #3a342c);
  --rv-hover: var(--w-surface-hover, #2e2923);
  --rv-green: var(--w-page-accent, #5edb95);
  --rv-green-deep: var(--w-page-accent, #5edb95);
  --rv-green-bg: var(--w-page-accent-soft, #1e3a2b);
  --rv-sage: #25312b;
  --rv-sage-d: #9fc4b4;
  --rv-haze: #1e2a31;
  --rv-haze-d: #a3c2d3;
  --rv-oat: #2d2821;
  --rv-oat-d: #d3bd8f;
  --rv-clay: #332523;
  --rv-clay-d: #d9a9a1;
  --rv-clay-deep: #412d2a;
  --rv-mauve: #2a2431;
  --rv-mauve-d: #c1a6ca;
  --rv-yellow: #d8b36a;
}

:global(html[theme-mode="dark"] .reading-page .rv-primary) {
  color: var(--w-page-accent-ink, #0e130f);
}

:global(html[theme-mode="dark"] .rv-doc-row.on),
:global(html[theme-mode="dark"] .rv-depth-row.on) {
  background: var(--rv-green-bg);
}

:global(html[theme-mode="dark"] .rv-option) {
  background: rgba(255, 255, 255, 0.04);
}

:global(html[theme-mode="dark"] .rv-option:hover),
:global(html[theme-mode="dark"] .rv-option.on) {
  background: rgba(255, 255, 255, 0.08);
}

:global(html[theme-mode="dark"] .rv-textarea--answer) {
  background: var(--rv-bg);
}

:global(html[theme-mode="dark"] .rv-evidence-float) {
  background: var(--rv-side);
}

/* responsive: the workbench can be windowed below 1100px even on a wide screen */
.reading-page.is-narrow .rv-mobile-toggle {
  display: inline-block;
  padding: 4px 8px;
  color: var(--rv-text-2);
}

.reading-page.is-narrow .rv-right {
  position: absolute;
  top: 48px;
  right: 0;
  bottom: 0;
  z-index: 30;
  display: none;
  width: min(320px, 88%);
  background: var(--rv-side);
  box-shadow: -12px 0 32px rgba(15, 15, 15, 0.08);
}

.reading-page.is-narrow .rv-right.is-open {
  display: block;
}

.reading-page.is-narrow .rv-right__resizer {
  display: none;
}

.reading-page.is-narrow .rv-right__mobile-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
  font-size: 12px;
  font-weight: 600;
  color: var(--rv-text-2);
}

.reading-page.is-narrow .rv-right__mobile-head button {
  padding: 2px 8px;
  font-size: 18px;
  color: var(--rv-text-3);
}

@media (max-width: 1100px) {
  .rv-mobile-toggle {
    display: inline-block;
    padding: 4px 8px;
    color: var(--rv-text-2);
  }

  .rv-right {
    position: absolute;
    top: 48px;
    right: 0;
    bottom: 0;
    z-index: 30;
    display: none;
    width: min(320px, 88%);
    background: var(--rv-side);
    box-shadow: -12px 0 32px rgba(15, 15, 15, 0.08);
  }

  .rv-right.is-open {
    display: block;
  }

  .rv-right__resizer {
    display: none;
  }

  .rv-right__mobile-head,
  .rv-left__mobile-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 14px;
    font-size: 12px;
    font-weight: 600;
    color: var(--rv-text-2);
  }

  .rv-right__mobile-head button,
  .rv-left__mobile-head button {
    padding: 2px 8px;
    font-size: 18px;
    color: var(--rv-text-3);
  }
}

@media (max-width: 760px) {
  .rv-topbar {
    flex-wrap: wrap;
    height: auto;
    min-height: 48px;
    padding: 6px 12px;
  }

  .rv-topbar__doc {
    max-width: 60%;
  }

  .rv-progress {
    display: none;
  }

  .rv-left {
    position: absolute;
    top: 48px;
    bottom: 0;
    left: 0;
    z-index: 30;
    display: none;
    width: min(280px, 88%);
    background: var(--rv-side);
    box-shadow: 12px 0 32px rgba(15, 15, 15, 0.08);
  }

  .rv-left.is-open {
    display: block;
  }

  .rv-read {
    padding: 24px 20px 100px;
  }

  .rv-config__inner {
    width: calc(100% - 32px);
    padding-top: 32px;
  }

  .rv-chapter-head {
    flex-direction: column;
  }

  .rv-config__note {
    width: 100%;
    margin-left: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .reading-page *,
  .reading-page *::before,
  .reading-page *::after {
    transition: none !important;
  }
}
</style>
