<template>
  <div class="reading-page">
    <div class="reading-header">
      <div>
        <h1 class="reading-title">{{ $t('reading.title') }}</h1>
        <p class="reading-desc">{{ $t('reading.description') }}</p>
      </div>
      <div v-if="run" class="reading-header-actions">
        <t-button v-if="canStartNewReading" theme="primary" @click="startNewReading">
          {{ $t('reading.newReading') }}
        </t-button>
        <t-button v-if="showReportEditor" variant="outline" theme="default" :loading="reportLoading" @click="openReportEditor">
          {{ $t('reading.editReport') }}
          <span v-if="reportEdited" class="reading-edited-badge">{{ $t('reading.editedTag') }}</span>
        </t-button>
        <t-button variant="outline" theme="default" :loading="exporting" @click="exportReport('markdown')">
          {{ $t('reading.exportMarkdown') }}
        </t-button>
        <t-button variant="outline" theme="default" :loading="exporting" @click="exportReport('json')">
          {{ $t('reading.exportJSON') }}
        </t-button>
        <t-button v-if="run.status !== 'finished' && run.status !== 'stopped'" variant="outline" theme="danger" @click="stopRun">
          {{ $t('reading.stop') }}
        </t-button>
      </div>
    </div>

    <div v-if="!run" class="reading-start">
      <div class="reading-form">
        <div class="reading-form-col">
          <label class="reading-label">{{ $t('reading.kbLabel') }}</label>
          <t-input v-model="kbSearch" clearable :placeholder="$t('reading.searchKb')" class="reading-search">
            <template #prefix-icon><t-icon name="search" /></template>
          </t-input>
          <t-select v-model="kbId" :options="filteredKbOptions" filterable @change="loadDocuments" />
          <label class="reading-label">{{ $t('reading.docsLabel') }}</label>
          <t-input v-model="docSearch" clearable :placeholder="$t('reading.searchDocs')" class="reading-search" :disabled="!kbId">
            <template #prefix-icon><t-icon name="search" /></template>
          </t-input>
          <t-select v-model="documentIds" :options="filteredDocumentOptions" multiple filterable :max="5" :disabled="!kbId" />
        </div>
        <div class="reading-form-col">
          <label class="reading-label">{{ $t('reading.depthLabel') }}</label>
          <t-radio-group v-model="depth">
            <t-radio-button value="quick">{{ $t('reading.depth.quick') }}</t-radio-button>
            <t-radio-button value="standard">{{ $t('reading.depth.standard') }}</t-radio-button>
            <t-radio-button value="deep">{{ $t('reading.depth.deep') }}</t-radio-button>
          </t-radio-group>
          <p class="reading-depth-hint">{{ $t(`reading.depthHint.${depth}`) }}</p>
          <label class="reading-label">{{ $t('reading.goalLabel') }}</label>
          <t-textarea v-model="goal" :autosize="{ minRows: 2, maxRows: 4 }"
            :placeholder="$t('reading.goalPlaceholder')" />
        </div>
        <div class="reading-form-footer">
          <t-button class="reading-start-button" theme="primary" :loading="creating" :disabled="documentIds.length === 0" @click="startRun">
            {{ $t('reading.start') }}
          </t-button>
        </div>
      </div>
      <p class="reading-hint">{{ $t('reading.hint') }}</p>
    </div>

    <div v-else class="reading-body" ref="readingBodyRef"
      :style="{ '--reading-left-width': leftWidth + 'px' }"
      :class="{ 'notes-collapsed': notesCollapsed, 'plan-collapsed': planCollapsed }">
      <section v-if="!planCollapsed" ref="documentPaneRef" class="reading-side reading-progress">
        <div class="reading-side-head">
          <span>{{ $t('reading.documentFullText') }}</span>
          <button type="button" class="reading-collapse" @click="planCollapsed = true">«</button>
        </div>
        <div class="reading-meta">
          <t-tag :theme="statusTheme" variant="light">{{ $t(`reading.status.${run.status}`) }}</t-tag>
          <t-tag variant="outline">{{ depthLabel }}</t-tag>
          <span>{{ $t('reading.stepCount', { count: run.step_count }) }}</span>
        </div>
        <p class="reading-meta-line">{{ $t('reading.policyTag') }}</p>
        <div class="reading-doc-toolbar">
          <t-select v-if="readingDocuments.length > 1" v-model="activeDocumentId"
            :options="readingDocumentOptions" size="small" class="reading-doc-select" />
          <span v-else-if="activeDocument" class="reading-doc-title">{{ activeDocument.title }}</span>
          <span class="reading-doc-progress">{{ readSectionCount }}/{{ run.plan.sections?.length || 0 }}</span>
        </div>
        <p v-if="currentSection" class="reading-doc-current">
          {{ $t('reading.currentLocation') }}：{{ currentSection.heading || currentSection.title }}
        </p>
        <div v-if="readingDocumentsLoading" class="reading-empty">{{ $t('reading.loadingDocument') }}</div>
        <div v-else-if="activeDocument" class="reading-document-content">
          <template v-if="documentRange">
            <span>{{ documentBefore }}</span>
            <mark ref="documentCurrentRef" class="reading-document-current">{{ documentHighlight }}</mark>
            <span>{{ documentAfter }}</span>
          </template>
          <template v-else>{{ activeDocument.content }}</template>
        </div>
        <div v-else class="reading-empty">{{ $t('reading.noDocumentContent') }}</div>
        <div v-if="canAddDocs" class="reading-add-docs">
          <t-button size="small" variant="outline" @click="openAddDocs">
            {{ $t('reading.addDocuments') }}
          </t-button>
        </div>
      </section>

      <button v-else type="button" class="reading-expand reading-expand-left" @click="planCollapsed = false">
        »
      </button>

      <div v-if="!planCollapsed" class="reading-resizer" role="separator" aria-orientation="vertical"
        tabindex="0" :aria-label="$t('reading.resizeLeftPane')"
        @pointerdown="startResize" @pointermove="onResize" @pointerup="endResize"
        @pointercancel="endResize" @dblclick="resetLeftWidth" @keydown="onResizeKeydown"></div>

      <section class="reading-main">
        <div class="reading-main-head">
          <div>
            <span class="reading-main-kicker">{{ isStructured ? $t('reading.currentRound') : $t('reading.currentReading') }}</span>
            <h2>{{ currentSection?.heading || currentSection?.title || $t('reading.plan') }}</h2>
            <p v-if="isStructured && currentRound > 0" class="reading-round-line">
              {{ $t('reading.roundLabel', { round: currentRound }) }}
            </p>
          </div>
          <span v-if="advancing" class="reading-reading-now">{{ $t('reading.readingNow') }}</span>
        </div>

        <template v-if="isStructured">
          <div v-if="run.status === 'planning'" class="reading-plan-confirm">
            <h3>{{ $t('reading.planConfirmTitle') }}</h3>
            <p>{{ $t('reading.planConfirmHint', { sections: run.plan.sections?.length || 0, docs: run.document_ids.length }) }}</p>
            <div class="reading-plan-summary">
              <span>{{ $t('reading.reportGoal') }}：{{ run.plan.goal || '—' }}</span>
              <span>{{ $t('reading.reportDepth') }}：{{ depthLabel }}</span>
              <span>{{ $t('reading.reportPolicy') }}：{{ run.plan.policy || 'grill-with-docs' }}</span>
            </div>
            <t-button theme="primary" :loading="acting" @click="confirmPlan">
              {{ $t('reading.startReading') }}
            </t-button>
          </div>

          <div v-else-if="run.status === 'finished'" class="reading-final">
            <ReadingThoughtChain
              :trace="currentRoundTrace"
              :questions="roundQuestions"
              :section-title="traceSection?.title"
              :section-heading="traceSection?.heading"
              :duration-label="selectedTraceDurationLabel"
              :round-options="traceRoundOptions"
              v-model:selected-round="selectedTraceRound"
            />
            <h3>{{ $t('reading.final') }}</h3>
            <p v-if="finalSummary" class="reading-final-summary">{{ finalSummary }}</p>
            <div class="reading-final-stats">
              <span>{{ $t('reading.confirmedCount', { count: confirmedConclusions.length }) }}</span>
              <span>{{ $t('reading.openCount', { count: openQuestions.length }) }}</span>
              <span>{{ $t('reading.contradictionCount', { count: structuredContradictions.length }) }}</span>
            </div>
            <p class="reading-hint">{{ $t('reading.finalHint') }}</p>
            <div class="reading-actions">
              <t-button theme="primary" @click="startNewReading">
                {{ $t('reading.newReading') }}
              </t-button>
            </div>
          </div>

          <div v-else-if="currentPendingQuestion" class="reading-question">
            <ReadingThoughtChain
              :trace="currentRoundTrace"
              :questions="roundQuestions"
              :section-title="traceSection?.title"
              :section-heading="traceSection?.heading"
              :duration-label="selectedTraceDurationLabel"
              :round-options="traceRoundOptions"
              v-model:selected-round="selectedTraceRound"
            />
            <div class="reading-question-meta">
              <t-tag size="small" theme="warning" variant="light">{{ $t('reading.pendingConfirm') }}</t-tag>
              <t-tag v-if="currentPendingQuestion.kind === 'fact'" size="small" theme="warning" variant="light">
                {{ $t('reading.questionKind.fact') }}
              </t-tag>
              <t-tag v-else-if="currentPendingQuestion.kind === 'decision'" size="small" theme="primary" variant="light">
                {{ $t('reading.questionKind.decision') }}
              </t-tag>
              <t-tag v-if="currentPendingQuestion.gap_type && currentPendingQuestion.gap_type !== 'unclassified'"
                size="small" variant="outline">
                {{ $t(`reading.gapTypes.${currentPendingQuestion.gap_type}`) }}
              </t-tag>
              <span>{{ $t('reading.roundLabel', { round: currentPendingQuestion.round }) }}</span>
            </div>
            <h3>{{ currentPendingQuestion.text }}</h3>
            <p v-if="currentPendingQuestion.why" class="reading-why">{{ currentPendingQuestion.why }}</p>
            <p v-if="currentPendingQuestion.recommended_answer" class="reading-recommend">
              {{ $t('reading.recommended') }}：{{ currentPendingQuestion.recommended_answer }}
            </p>
            <div v-if="currentPendingQuestion.evidence?.length" class="reading-evidence-list">
              <button v-for="evidence in currentPendingQuestion.evidence"
                :key="evidence.chunk_id + evidence.quote" type="button"
                class="reading-evidence-item" @click="openEvidence(evidence)">
                <span class="reading-evidence-source">{{ evidenceLabel(evidence) }}</span>
                <span class="reading-evidence-quote">{{ evidence.quote }}</span>
              </button>
            </div>
            <div class="reading-quick-actions">
              <t-button size="small" theme="primary" variant="outline" @click="acceptRecommendation">
                {{ $t('reading.acceptRecommendation') }}
              </t-button>
              <t-button size="small" variant="outline" @click="skipQuestion">{{ $t('reading.skipQuestion') }}</t-button>
            </div>
            <p v-if="pendingQueueCount > 0" class="reading-queue-hint">
              {{ $t('reading.pendingQueue', { count: pendingQueueCount }) }}
            </p>
            <t-textarea v-model="feedback" :autosize="{ minRows: 2, maxRows: 4 }"
              :placeholder="$t('reading.feedbackPlaceholder')" />
            <div class="reading-actions">
              <t-button theme="primary" :loading="acting" @click="submitFeedback">
                {{ $t('reading.submitFeedback') }}
              </t-button>
            </div>
          </div>

          <div v-else-if="run.status === 'stopped' || run.status === 'failed'" class="reading-idle">
            <p>{{ $t('reading.pausedHint') }}</p>
            <t-button theme="primary" :loading="acting" @click="resumeOrAdvance">
              {{ run.status === 'stopped' ? $t('reading.resume') : $t('reading.continueReading') }}
            </t-button>
            <p class="reading-hint">{{ $t('reading.stoppedHint') }}</p>
          </div>

          <template v-else>
            <ReadingThoughtChain
              :trace="currentRoundTrace"
              :questions="roundQuestions"
              :section-title="traceSection?.title"
              :section-heading="traceSection?.heading"
              :duration-label="selectedTraceDurationLabel"
              :round-options="traceRoundOptions"
              :advancing="advancing"
              v-model:selected-round="selectedTraceRound"
            />

            <div class="reading-round-card">
              <div class="reading-round-head">
                <span>{{ $t('reading.understanding') }}</span>
                <span class="reading-round-status">{{ advancing ? $t('reading.readingNow') : currentRoundDurationLabel }}</span>
              </div>
              <p v-if="currentUnderstanding" class="reading-round-understanding">{{ currentUnderstanding }}</p>
              <p v-else class="reading-empty">{{ $t('reading.emptyUnderstanding') }}</p>
            </div>

            <div v-if="roundAutoQuestions.length" class="reading-round-questions">
              <h4>{{ $t('reading.roundQuestions') }}</h4>
              <div v-for="question in roundAutoQuestions" :key="question.id" class="reading-qa-card">
                <div class="reading-qa-q">{{ question.text }}</div>
                <div class="reading-qa-a">
                  <t-tag size="small" :theme="question.status === 'contradicted' ? 'danger' : 'success'" variant="light">
                    {{ question.status === 'contradicted' ? $t('reading.contradicted') : $t('reading.autoInferred') }}
                  </t-tag>
                  <span>{{ question.answer }}</span>
                </div>
                <div v-if="question.evidence?.length" class="reading-evidence-list">
                  <button v-for="evidence in question.evidence"
                    :key="evidence.chunk_id + evidence.quote" type="button"
                    class="reading-evidence-item" @click="openEvidence(evidence)">
                    <span class="reading-evidence-source">{{ evidenceLabel(evidence) }}</span>
                    <span class="reading-evidence-quote">{{ evidence.quote }}</span>
                  </button>
                </div>
              </div>
            </div>

            <div class="reading-accumulated">
              <h4>{{ $t('reading.accumulated') }}</h4>
              <p v-if="!confirmedConclusions.length" class="reading-empty">—</p>
              <div v-for="conclusion in confirmedConclusions" :key="conclusion.id" class="reading-conclusion-item">
                <p>{{ conclusion.text }}</p>
                <div v-if="conclusion.evidence?.length" class="reading-evidence-list">
                  <button v-for="evidence in conclusion.evidence"
                    :key="evidence.chunk_id + evidence.quote" type="button"
                    class="reading-evidence-item" @click="openEvidence(evidence)">
                    <span class="reading-evidence-source">{{ evidenceLabel(evidence) }}</span>
                    <span class="reading-evidence-quote">{{ evidence.quote }}</span>
                  </button>
                </div>
              </div>
            </div>

            <div v-if="pauseEachRound && !advancing && run.status === 'reading'" class="reading-round-continue">
              <t-button theme="primary" :loading="acting" @click="resumeOrAdvance">
                {{ $t('reading.continueReading') }}
              </t-button>
              <p class="reading-hint">{{ $t('reading.roundReviewHint') }}</p>
            </div>
          </template>
        </template>

        <template v-else-if="currentQuestion">
          <div class="reading-question">
            <h3>{{ currentQuestion.text }}</h3>
            <p v-if="currentQuestion.recommended_answer" class="reading-recommend">
              {{ $t('reading.recommended') }}：{{ currentQuestion.recommended_answer }}
            </p>
            <p v-if="currentQuestion.why" class="reading-why">{{ currentQuestion.why }}</p>
            <div class="reading-quick-actions">
              <t-button size="small" theme="primary" variant="outline" @click="acceptRecommendation">
                {{ $t('reading.acceptRecommendation') }}
              </t-button>
              <t-button size="small" variant="outline" @click="skipQuestion">{{ $t('reading.skipQuestion') }}</t-button>
            </div>
            <t-textarea v-model="feedback" :autosize="{ minRows: 2, maxRows: 4 }"
              :placeholder="$t('reading.feedbackPlaceholder')" />
            <div class="reading-actions">
              <t-button theme="primary" :loading="acting" @click="submitFeedback">
                {{ $t('reading.submitFeedback') }}
              </t-button>
            </div>
          </div>
        </template>

        <template v-else-if="run.status === 'planning'">
          <div class="reading-plan-confirm">
            <h3>{{ $t('reading.planConfirmTitle') }}</h3>
            <p>{{ $t('reading.planConfirmHint', { sections: run.plan.sections?.length || 0, docs: run.document_ids.length }) }}</p>
            <div class="reading-plan-summary">
              <span>{{ $t('reading.reportGoal') }}：{{ run.plan.goal || '—' }}</span>
              <span>{{ $t('reading.reportDepth') }}：{{ depthLabel }}</span>
              <span>{{ $t('reading.reportPolicy') }}：{{ run.plan.policy || 'grill-with-docs' }}</span>
            </div>
            <t-button theme="primary" :loading="acting" @click="confirmPlan">
              {{ $t('reading.startReading') }}
            </t-button>
          </div>
        </template>

        <template v-else-if="run.status === 'finished'">
          <div class="reading-final">
            <h3>{{ $t('reading.final') }}</h3>
            <p v-if="finalSummary" class="reading-final-summary">{{ finalSummary }}</p>
            <div v-for="group in finalGroups" :key="group.key" class="reading-final-group">
              <h4>{{ group.label }}</h4>
              <p v-if="!group.items.length" class="reading-empty">—</p>
              <p v-for="(item, index) in group.items" :key="index" class="reading-note-item">{{ item }}</p>
            </div>
            <div class="reading-actions">
              <t-button theme="primary" @click="startNewReading">
                {{ $t('reading.newReading') }}
              </t-button>
            </div>
          </div>
        </template>

        <template v-else>
          <div class="reading-idle">
            <p>{{ advancing ? $t('reading.readingNow') : $t('reading.pausedHint') }}</p>
            <t-button v-if="!advancing && run.status !== 'finished'" theme="primary" :loading="acting" @click="resumeOrAdvance">
              {{ run.status === 'stopped' ? $t('reading.resume') : $t('reading.continueReading') }}
            </t-button>
            <p v-if="run.status === 'stopped'" class="reading-hint">{{ $t('reading.stoppedHint') }}</p>
          </div>
        </template>
      </section>

      <section v-if="!notesCollapsed" class="reading-side reading-notes">
        <div class="reading-side-head">
          <span>{{ $t('reading.notes') }}</span>
          <button type="button" class="reading-collapse" @click="notesCollapsed = true">»</button>
        </div>
        <template v-if="isStructured">
          <div class="reading-tabs">
            <button v-for="tab in structuredTabs" :key="tab.key" type="button" class="reading-tab"
              :class="{ active: activeTab === tab.key }" @click="activeTab = tab.key">
              {{ tab.label }}<span v-if="tab.count" class="reading-tab-count">{{ tab.count }}</span>
            </button>
          </div>
          <div class="reading-tab-body">
            <template v-if="activeTab === 'terms'">
              <p v-if="!structuredTerms.length" class="reading-empty">{{ $t('reading.emptyTerms') }}</p>
              <div v-for="term in structuredTerms" :key="term.id" class="reading-term-item">
                <div class="reading-term-name">
                  {{ term.term }}
                  <t-tag size="small" :theme="term.status === 'confirmed' ? 'success' : 'warning'" variant="light">
                    {{ term.status === 'confirmed' ? $t('reading.confirmedTag') : $t('reading.pendingConfirm') }}
                  </t-tag>
                </div>
                <p class="reading-term-definition">{{ term.definition }}</p>
                <div v-if="term.source?.length" class="reading-evidence-list">
                  <button v-for="evidence in term.source" :key="evidence.chunk_id + evidence.quote"
                    type="button" class="reading-evidence-item" v-evidence-hover="evidence" @click="openEvidence(evidence)">
                    <span class="reading-evidence-source">{{ evidenceLabel(evidence) }}</span>
                    <span class="reading-evidence-quote">{{ evidence.quote }}</span>
                  </button>
                </div>
              </div>
            </template>
            <template v-else-if="activeTab === 'conclusions'">
              <p v-if="!structuredConclusions.length" class="reading-empty">{{ $t('reading.emptyConclusions') }}</p>
              <div v-for="conclusion in structuredConclusions" :key="conclusion.id" class="reading-conclusion-item">
                <div class="reading-conclusion-head">
                  <t-tag size="small" :theme="conclusionTagTheme(conclusion)" variant="light">
                    {{ conclusionStatusLabel(conclusion) }}
                  </t-tag>
                  <span class="reading-round-tag">{{ $t('reading.roundLabel', { round: conclusion.round }) }}</span>
                </div>
                <p>{{ conclusion.text }}</p>
                <div v-if="conclusion.evidence?.length" class="reading-evidence-list">
                  <button v-for="evidence in conclusion.evidence" :key="evidence.chunk_id + evidence.quote"
                    type="button" class="reading-evidence-item" v-evidence-hover="evidence" @click="openEvidence(evidence)">
                    <span class="reading-evidence-source">{{ evidenceLabel(evidence) }}</span>
                    <span class="reading-evidence-quote">{{ evidence.quote }}</span>
                  </button>
                </div>
              </div>
            </template>
            <template v-else-if="activeTab === 'open'">
              <p v-if="!openQuestions.length" class="reading-empty">{{ $t('reading.emptyOpen') }}</p>
              <div v-for="question in openQuestions" :key="question.id" class="reading-open-item">
                <div class="reading-open-head">
                  <t-tag size="small" :theme="questionTagTheme(question)" variant="light">
                    {{ questionStatusLabel(question) }}
                  </t-tag>
                  <t-tag v-if="question.kind === 'fact'" size="small" theme="warning" variant="light">
                    {{ $t('reading.questionKind.fact') }}
                  </t-tag>
                  <t-tag v-else-if="question.kind === 'decision'" size="small" theme="primary" variant="light">
                    {{ $t('reading.questionKind.decision') }}
                  </t-tag>
                  <span class="reading-round-tag">{{ $t('reading.roundLabel', { round: question.round }) }}</span>
                </div>
                <p>{{ question.text }}</p>
                <p v-if="question.recommended_answer" class="reading-recommend">
                  {{ $t('reading.recommended') }}：{{ question.recommended_answer }}
                </p>
                <div v-if="question.evidence?.length" class="reading-evidence-list">
                  <button v-for="evidence in question.evidence" :key="evidence.chunk_id + evidence.quote"
                    type="button" class="reading-evidence-item" v-evidence-hover="evidence" @click="openEvidence(evidence)">
                    <span class="reading-evidence-source">{{ evidenceLabel(evidence) }}</span>
                    <span class="reading-evidence-quote">{{ evidence.quote }}</span>
                  </button>
                </div>
              </div>
            </template>
            <template v-else>
              <p v-if="!structuredContradictions.length" class="reading-empty">{{ $t('reading.emptyContradictions') }}</p>
              <div v-for="item in structuredContradictions" :key="item.id" class="reading-contradiction-item">
                <div class="reading-contradiction-head">
                  <t-tag size="small" :theme="item.status === 'open' ? 'warning' : 'success'" variant="light">
                    {{ item.status === 'open' ? $t('reading.contradictionOpen') : $t('reading.contradictionResolved') }}
                  </t-tag>
                </div>
                <p class="reading-contradiction-desc">{{ item.description }}</p>
                <div class="reading-contradiction-grid">
                  <div class="reading-contradiction-column">
                    <div class="reading-contradiction-column-head">A</div>
                    <div class="reading-evidence-list">
                      <button v-for="evidence in item.evidence_a || []" :key="evidence.chunk_id + evidence.quote"
                        type="button" class="reading-evidence-item" v-evidence-hover="evidence"
                        @click="openEvidence(evidence)">
                        <span class="reading-evidence-source">{{ evidenceLabel(evidence) }}</span>
                        <span class="reading-evidence-quote">{{ evidence.quote }}</span>
                      </button>
                    </div>
                  </div>
                  <div class="reading-contradiction-column">
                    <div class="reading-contradiction-column-head">B</div>
                    <div class="reading-evidence-list">
                      <button v-for="evidence in item.evidence_b || []" :key="evidence.chunk_id + evidence.quote"
                        type="button" class="reading-evidence-item" v-evidence-hover="evidence"
                        @click="openEvidence(evidence)">
                        <span class="reading-evidence-source">{{ evidenceLabel(evidence) }}</span>
                        <span class="reading-evidence-quote">{{ evidence.quote }}</span>
                      </button>
                    </div>
                  </div>
                </div>
                <p v-if="!(item.evidence_a?.length || item.evidence_b?.length)" class="reading-empty">
                  {{ $t('reading.legacyNoEvidence') }}
                </p>
                <p v-if="item.status !== 'open'" class="reading-resolution-text">
                  {{ $t('reading.resolution') }}：{{ item.resolution }}
                </p>
                <div v-else class="reading-contradiction-resolve">
                  <div class="reading-quick-actions">
                    <t-button size="small" variant="outline" :loading="resolvingId === item.id"
                      @click="resolveContradiction(item, 'a')">{{ $t('reading.resolveWithA') }}</t-button>
                    <t-button size="small" variant="outline" :loading="resolvingId === item.id"
                      @click="resolveContradiction(item, 'b')">{{ $t('reading.resolveWithB') }}</t-button>
                    <t-button size="small" variant="outline" @click="toggleCustomResolution(item)">
                      {{ $t('reading.resolveCustom') }}
                    </t-button>
                  </div>
                  <template v-if="customResolutionId === item.id">
                    <t-textarea v-model="customResolution" :autosize="{ minRows: 2, maxRows: 4 }"
                      :placeholder="$t('reading.resolutionPlaceholder')" />
                    <t-button size="small" theme="primary" :loading="resolvingId === item.id"
                      @click="resolveContradiction(item, 'custom')">{{ $t('reading.submitResolution') }}</t-button>
                  </template>
                </div>
              </div>
            </template>
          </div>
        </template>
        <template v-else>
          <div v-for="group in noteGroups" :key="group.key" class="reading-note-group">
            <h4>
              {{ group.label }}
              <span v-if="group.items.length" class="reading-note-count">{{ group.items.length }}</span>
            </h4>
            <p v-if="!group.items.length" class="reading-empty">—</p>
            <p v-for="(item, index) in group.items" :key="index" class="reading-note-item">{{ item }}</p>
          </div>
        </template>
      </section>
      <button v-else type="button" class="reading-expand reading-expand-right" @click="notesCollapsed = false">
        {{ $t('reading.notes') }} «
      </button>
    </div>

    <t-dialog v-model:visible="addDocDialogVisible" :header="$t('reading.addDocuments')" width="560px" :footer="false">
      <div class="reading-add-dialog">
        <p class="reading-add-dialog-hint">{{ $t('reading.addDocumentsHint') }}</p>
        <t-input v-model="addDocSearch" clearable :placeholder="$t('reading.searchAddDocs')">
          <template #prefix-icon><t-icon name="search" /></template>
        </t-input>
        <t-select v-model="addDocIds" :options="filteredAddDocOptions" multiple filterable
          :placeholder="$t('reading.addDocumentsPlaceholder')" />
        <p v-if="!addDocOptions.length" class="reading-hint">{{ $t('reading.noMoreDocuments') }}</p>
        <div class="reading-add-dialog-actions">
          <t-button variant="outline" @click="addDocDialogVisible = false">{{ $t('common.cancel') }}</t-button>
          <t-button theme="primary" :loading="addingDocs" :disabled="!addDocIds.length" @click="confirmAddDocs">
            {{ $t('common.confirm') }}
          </t-button>
        </div>
      </div>
    </t-dialog>

    <t-dialog v-model:visible="reportDialogVisible" :header="$t('reading.editReport')" width="900px" :footer="false">
      <div class="reading-report-editor">
        <p class="reading-add-dialog-hint">{{ $t('reading.editReportHint') }}</p>
        <t-textarea v-model="reportDraft" :autosize="{ minRows: 20, maxRows: 32 }" />
        <div class="reading-add-dialog-actions">
          <t-button variant="outline" :loading="reportSaving" @click="resetReportDraft">
            {{ $t('reading.resetReport') }}
          </t-button>
          <t-button variant="outline" @click="reportDialogVisible = false">{{ $t('common.cancel') }}</t-button>
          <t-button theme="primary" :loading="reportSaving" @click="saveReportDraft">
            {{ $t('reading.saveReport') }}
          </t-button>
        </div>
      </div>
    </t-dialog>

    <div v-if="evidenceFloat.visible" class="reading-evidence-float"
      :style="{ top: evidenceFloat.top + 'px', left: evidenceFloat.left + 'px' }"
      @mouseenter="cancelEvidenceClose" @mouseleave="hideEvidenceFloat">
      <div class="reading-evidence-float-title">{{ evidenceFloat.title }}</div>
      <div v-if="evidenceFloat.loading" class="reading-evidence-float-muted">{{ $t('reading.loadingOrigin') }}</div>
      <div v-else-if="evidenceFloat.error" class="reading-evidence-float-error">{{ evidenceFloat.error }}</div>
      <div v-else class="reading-evidence-float-body">
        <span v-for="(part, index) in evidenceFloatParts" :key="index"
          :class="{ 'reading-evidence-hit': part.hit }">{{ part.text }}</span>
      </div>
      <div class="reading-evidence-float-hint">{{ $t('reading.clickToOpen') }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch, type Directive } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { createSessions } from '@/api/chat'
import { getChunkByIdOnly, listKnowledgeBases, listKnowledgeFiles } from '@/api/knowledge-base'
import {
  createReadingRun,
  deleteReadingReport,
  exportReadingRun,
  feedbackReadingRun,
  getReadingReport,
  getReadingRun,
  getReadingRunDocuments,
  resolveReadingContradiction,
  saveReadingReport,
  stepReadingRun,
  stopReadingRun,
  resumeReadingRun,
  addReadingDocuments,
  type ReadingConclusion,
  type ReadingContradiction,
  type ReadingDepth,
  type ReadingDocument,
  type ReadingEvidence,
  type ReadingQuestionNode,
  type ReadingRun,
  type ReadingRunStep,
} from '@/api/reading'
import ReadingThoughtChain from '@/components/ReadingThoughtChain.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const kbId = ref('')
const kbOptions = ref<Array<{ value: string; label: string }>>([])
const documentIds = ref<string[]>([])
const documentOptions = ref<Array<{ value: string; label: string }>>([])
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
const exporting = ref(false)
const planCollapsed = ref(false)
const notesCollapsed = ref(false)
const addDocDialogVisible = ref(false)
const addDocOptions = ref<Array<{ value: string; label: string }>>([])
const addDocIds = ref<string[]>([])
const addingDocs = ref(false)
const readingBodyRef = ref<HTMLElement | null>(null)
const readingDocuments = ref<ReadingDocument[]>([])
const readingDocumentsLoading = ref(false)
const activeDocumentId = ref('')
const documentPaneRef = ref<HTMLElement | null>(null)
const documentCurrentRef = ref<HTMLElement | null>(null)
const selectedTraceRound = ref(0)
const leftWidth = ref(loadStoredLeftWidth())
const storageKey = 'gaze_active_reading_run_id'

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
const activeTab = ref<'terms' | 'conclusions' | 'open' | 'contradictions'>('conclusions')
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
  ['finished', 'stopped', 'failed'].includes(String(run.value?.status || '')),
)

const activeDocument = computed(() => {
  const documents = readingDocuments.value
  return documents.find((doc) => doc.id === activeDocumentId.value) || documents[0] || null
})
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
const activeDocumentSection = computed(() => {
  const section = currentPositionSection.value
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
  if (next === leftWidth.value) return
  leftWidth.value = next
  readingBodyRef.value?.style.setProperty('--reading-left-width', `${next}px`)
  localStorage.setItem('gaze_reading_left_width', String(next))
}

function evidenceLabel(evidence: ReadingEvidence) {
  const title = evidence.document_title || ''
  return evidence.heading ? `${title} § ${evidence.heading}` : title
}

function openEvidence(evidence: ReadingEvidence) {
  const kbId = run.value?.plan.knowledge_base_id
  if (!kbId || !evidence.knowledge_id) return
  router.push({
    path: `/platform/knowledge-bases/${kbId}`,
    query: { knowledge_id: evidence.knowledge_id },
  })
}

function conclusionStatusLabel(conclusion: ReadingConclusion) {
  if (conclusion.source === 'legacy') return t('reading.legacyNote')
  if (conclusion.status === 'confirmed') {
    return conclusion.source === 'user' ? t('reading.userDecision') : t('reading.confirmedTag')
  }
  if (conclusion.status === 'contradicted') return t('reading.contradicted')
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
    reportDialogVisible.value = true
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.reportLoadFailed'))
  } finally {
    reportLoading.value = false
  }
}

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
const finalGroups = computed(() => {
  const final = run.value?.final || {}
  return [
    { key: 'key_points', label: t('reading.finalGroups.keyPoints'), items: toStringList(final.key_points) },
    { key: 'open_questions', label: t('reading.finalGroups.openQuestions'), items: toStringList(final.open_questions) },
    { key: 'contradictions', label: t('reading.finalGroups.contradictions'), items: toStringList(final.contradictions) },
    { key: 'citations', label: t('reading.finalGroups.citations'), items: toStringList(final.citations) },
  ]
})

function toStringList(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.map((item) => typeof item === 'string' ? item : JSON.stringify(item))
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

const filteredKbOptions = computed(() => filterOptions(kbOptions.value, kbSearch.value))
const filteredDocumentOptions = computed(() =>
  withSelectedOptions(filterOptions(documentOptions.value, docSearch.value), documentIds.value),
)
const filteredAddDocOptions = computed(() =>
  withSelectedOptions(filterOptions(addDocOptions.value, addDocSearch.value), addDocIds.value),
)

const loadKnowledgeBases = async () => {
  try {
    const response = await listKnowledgeBases()
    const list = response.data?.data || response.data || []
    kbOptions.value = (Array.isArray(list) ? list : []).map((kb: any) => ({
      value: kb.id,
      label: kb.name || kb.id,
    }))
  } catch {
    kbOptions.value = []
  }
}

const loadDocuments = async () => {
  documentIds.value = []
  documentOptions.value = []
  docSearch.value = ''
  if (!kbId.value) return
  try {
    const response = await listKnowledgeFiles(kbId.value, { page: 1, page_size: 500 })
    const list = response.data || []
    documentOptions.value = (Array.isArray(list) ? list : [])
      .filter((doc: any) => doc.parse_status === 'completed')
      .map((doc: any) => ({ value: doc.id, label: doc.file_name || doc.title || doc.id }))
  } catch {
    documentOptions.value = []
  }
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
    const currentDocID = currentSection.value?.knowledge_id
    if (currentDocID && readingDocuments.value.some((doc) => doc.id === currentDocID)) {
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
  const kb = current?.plan.knowledge_base_id
  if (!current || !kb) return
  try {
    const response = await listKnowledgeFiles(kb, { page: 1, page_size: 500 })
    const list = response.data || []
    const existing = new Set(current.document_ids)
    addDocOptions.value = (Array.isArray(list) ? list : [])
      .filter((doc: any) => doc.parse_status === 'completed' && !existing.has(doc.id))
      .map((doc: any) => ({ value: doc.id, label: doc.file_name || doc.title || doc.id }))
    addDocIds.value = []
    addDocSearch.value = ''
    addDocDialogVisible.value = true
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.addDocumentsFailed'))
  }
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
  if (next.status === 'finished' || next.status === 'stopped') {
    localStorage.removeItem(storageKey)
  } else {
    localStorage.setItem(storageKey, next.id)
  }
}

const startRun = async () => {
  creating.value = true
  try {
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
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.startFailed'))
  } finally {
    creating.value = false
  }
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

const confirmPlan = async () => {
  if (!run.value || run.value.status !== 'planning') return
  acting.value = true
  try {
    await advanceUntilPause()
  } finally {
    acting.value = false
  }
}

const advanceUntilPause = async () => {
  if (!run.value || advancing.value) return
  advancing.value = true
  try {
    let next = run.value
    for (let i = 0; i < 12; i += 1) {
      if (!next || ['waiting_user', 'finished', 'stopped', 'failed'].includes(next.status)) break
      if (pendingQuestion(next)) break
      const response = await stepReadingRun(next.id)
      next = response.data
      await persistRun(next)
      if (pauseEachRound.value && next.status === 'reading') break
    }
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('reading.stepFailed'))
  } finally {
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

onMounted(async () => {
  window.addEventListener('scroll', hideEvidenceOnScroll, true)
  window.addEventListener('resize', onWindowResize)
  await loadKnowledgeBases()
  const runId = queryValue(route.query.run_id)
  if (runId) {
    try {
      const response = await getReadingRun(runId)
      run.value = response.data
      localStorage.setItem(storageKey, response.data.id)
      return
    } catch {
      localStorage.removeItem(storageKey)
    }
  }
  const kb = queryValue(route.query.kb)
  if (kb) {
    kbId.value = kb
    await loadDocuments()
    const docs = queryValue(route.query.docs).split(',').map((id) => id.trim()).filter(Boolean)
    const available = new Set(documentOptions.value.map((option) => option.value))
    documentIds.value = docs.filter((id) => available.has(id)).slice(0, 5)
    return
  }
  const activeId = localStorage.getItem(storageKey)
  if (!activeId) return
  try {
    const response = await getReadingRun(activeId)
    run.value = response.data
  } catch {
    localStorage.removeItem(storageKey)
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', hideEvidenceOnScroll, true)
  window.removeEventListener('resize', onWindowResize)
  if (evidenceCloseTimer) window.clearTimeout(evidenceCloseTimer)
  document.body.style.userSelect = ''
  document.body.style.cursor = ''
})
</script>

<style lang="less" scoped>
.reading-page {
  flex: 1 1 auto;
  width: 100%;
  max-width: 1480px;
  min-height: 0;
  margin: 0 auto;
  padding: 28px 24px 48px;
  overflow-y: auto;
  box-sizing: border-box;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.reading-page::-webkit-scrollbar {
  display: none;
}

.reading-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}

.reading-header-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.reading-title {
  margin: 0 0 6px;
  font-size: 22px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.reading-desc,
.reading-hint {
  margin: 0;
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.reading-start {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
  max-width: 1120px;
  margin: 4px auto 0;
}

.reading-form {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 16px 28px;
  width: 100%;
  padding: 24px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: 10px;
}

.reading-form-col {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}

.reading-form-footer {
  grid-column: 1 / -1;
  display: flex;
  justify-content: flex-end;
  padding-top: 4px;
}

.reading-start-button {
  min-width: 240px;
}

@media (max-width: 900px) {
  .reading-start {
    max-width: 680px;
    margin-left: 0;
    margin-right: 0;
  }

  .reading-form {
    grid-template-columns: minmax(0, 1fr);
    padding: 20px;
  }

  .reading-form-footer {
    justify-content: stretch;
  }

  .reading-start-button {
    width: 100%;
  }
}

.reading-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-primary);
}

.reading-search {
  margin-bottom: -4px;
}

.reading-depth-hint {
  margin: -4px 0 2px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.reading-body {
  position: relative;
  display: grid;
  grid-template-columns: var(--reading-left-width, 320px) minmax(0, 1fr) 300px;
  gap: 16px;
  align-items: start;
}

.reading-body.notes-collapsed {
  grid-template-columns: var(--reading-left-width, 320px) minmax(0, 1fr) 34px;
}

.reading-body.plan-collapsed {
  grid-template-columns: 34px minmax(0, 1fr) 300px;
}

.reading-body.plan-collapsed.notes-collapsed {
  grid-template-columns: 34px minmax(0, 1fr) 34px;
}

.reading-resizer {
  position: absolute;
  top: 0;
  bottom: 0;
  left: calc(var(--reading-left-width, 320px) + 8px);
  width: 8px;
  cursor: col-resize;
  touch-action: none;
  z-index: 5;
  outline: none;
}

.reading-resizer::after {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 3px;
  width: 2px;
  content: '';
  background: transparent;
  border-radius: 1px;
  transition: background 0.15s ease;
}

.reading-resizer:hover::after,
.reading-resizer:focus-visible::after {
  background: var(--td-brand-color);
}

.reading-doc-toolbar {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 6px;
}

.reading-doc-select {
  flex: 1;
  min-width: 0;
}

.reading-doc-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  font-size: 12px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.reading-doc-progress {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--td-text-color-secondary);
}

.reading-doc-current {
  margin: 0 0 8px;
  font-size: 11px;
  line-height: 1.5;
  color: var(--td-brand-color);
}

.reading-document-content {
  font-size: 12px;
  line-height: 1.8;
  color: var(--td-text-color-primary);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.reading-document-current {
  padding: 0 2px;
  color: inherit;
  background: var(--td-brand-color-light);
  border-radius: 3px;
}

.reading-side,
.reading-main {
  padding: 16px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: 10px;
}

.reading-progress,
.reading-notes {
  position: sticky;
  top: 16px;
  max-height: calc(100vh - 120px);
  overflow: auto;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.reading-progress::-webkit-scrollbar,
.reading-notes::-webkit-scrollbar {
  display: none;
}

.reading-side-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  font-size: 13px;
  font-weight: 600;
}

.reading-add-docs {
  display: flex;
  justify-content: center;
  margin-top: 12px;
}

.reading-add-dialog {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.reading-add-dialog-hint {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--td-text-color-secondary);
}

.reading-add-dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 6px;
}

.reading-collapse {
  padding: 0 4px;
  font-size: 14px;
  line-height: 1;
  color: var(--td-text-color-secondary);
  cursor: pointer;
  background: transparent;
  border: 0;
}

.reading-expand {
  align-self: start;
  padding: 10px 0;
  font-size: 12px;
  color: var(--td-text-color-secondary);
  cursor: pointer;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: 10px;
}

.reading-expand-left {
  writing-mode: vertical-rl;
}

.reading-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 8px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.reading-meta-line {
  margin: 0 0 12px;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.reading-section {
  display: flex;
  gap: 8px;
  padding: 8px;
  border-radius: 6px;
  color: var(--td-text-color-secondary);

  &.active {
    background: var(--td-brand-color-light);
    color: var(--td-brand-color);
  }

  &.read {
    opacity: 0.6;
  }
}

.reading-section-index {
  flex-shrink: 0;
  font-size: 12px;
}

.reading-section-title {
  font-size: 13px;
}

.reading-section-heading {
  margin-top: 2px;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.reading-main {
  min-height: 420px;
}

.reading-main-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 12px;
  margin-bottom: 16px;
  border-bottom: 1px solid var(--td-component-stroke);
}

.reading-main-head h2 {
  margin: 4px 0 0;
  font-size: 17px;
}

.reading-main-kicker {
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.reading-reading-now {
  flex-shrink: 0;
  padding: 4px 8px;
  font-size: 12px;
  color: var(--td-brand-color);
  background: var(--td-brand-color-light);
  border-radius: 999px;
}

.reading-question h3,
.reading-final h3,
.reading-plan-confirm h3 {
  margin: 0 0 12px;
  font-size: 16px;
}

.reading-plan-confirm p {
  margin: 0 0 12px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--td-text-color-secondary);
}

.reading-plan-summary {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  margin-bottom: 16px;
  font-size: 13px;
  color: var(--td-text-color-primary);
  background: var(--td-bg-color-secondarycontainer);
  border-radius: 8px;
}

.reading-note-group {
  margin-bottom: 14px;
}

.reading-note-group h4,
.reading-final-group h4 {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0 0 4px;
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.reading-note-count {
  padding: 0 6px;
  font-size: 11px;
  color: var(--td-text-color-secondary);
  background: var(--td-bg-color-secondarycontainer);
  border-radius: 999px;
}

.reading-note-item,
.reading-empty,
.reading-recommend,
.reading-why,
.reading-final-summary {
  margin: 0 0 4px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--td-text-color-primary);
}

.reading-recommend {
  color: var(--td-brand-color);
}

.reading-why {
  color: var(--td-text-color-secondary);
}

.reading-quick-actions,
.reading-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin: 12px 0;
}

.reading-idle {
  display: flex;
  flex-direction: column;
  gap: 12px;
  align-items: flex-start;
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.reading-final-group {
  margin-top: 12px;
}

.reading-round-line {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.reading-question-meta,
.reading-round-head,
.reading-conclusion-head,
.reading-open-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.reading-round-card,
.reading-round-questions,
.reading-accumulated {
  margin-bottom: 18px;
}

.reading-round-continue {
  display: flex;
  flex-direction: column;
  gap: 6px;
  align-items: flex-start;
  margin-top: 8px;
}

.reading-round-card {
  padding: 12px 14px;
  background: var(--td-bg-color-secondarycontainer);
  border-radius: 8px;
}

.reading-round-head {
  justify-content: space-between;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.reading-round-understanding {
  margin: 0;
  font-size: 14px;
  line-height: 1.7;
}

.reading-round-status {
  font-size: 12px;
  font-weight: 400;
  color: var(--td-brand-color);
}

.reading-round-questions h4,
.reading-accumulated h4 {
  margin: 0 0 8px;
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.reading-qa-card,
.reading-conclusion-item,
.reading-open-item,
.reading-term-item,
.reading-contradiction-item {
  padding: 10px 12px;
  margin-bottom: 8px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
}

.reading-qa-q {
  margin-bottom: 6px;
  font-size: 13px;
  color: var(--td-text-color-primary);
}

.reading-qa-a {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--td-text-color-primary);
}

.reading-evidence-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 6px;
}

.reading-evidence-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 6px 8px;
  font-family: inherit;
  font-size: 12px;
  text-align: left;
  cursor: pointer;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: 6px;
}

.reading-evidence-item:hover {
  border-color: var(--td-brand-color);
}

.reading-evidence-source {
  color: var(--td-brand-color);
}

.reading-evidence-quote {
  line-height: 1.5;
  color: var(--td-text-color-secondary);
}

.reading-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 10px;
}

.reading-tab {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  font-family: inherit;
  font-size: 12px;
  color: var(--td-text-color-secondary);
  cursor: pointer;
  background: var(--td-bg-color-secondarycontainer);
  border: 0;
  border-radius: 6px;
}

.reading-tab.active {
  color: #fff;
  background: var(--td-brand-color);
}

.reading-tab-count {
  padding: 0 4px;
  font-size: 10px;
  background: rgb(0 0 0 / 12%);
  border-radius: 999px;
}

.reading-tab-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.reading-term-name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.reading-term-definition {
  margin: 4px 0 0;
  font-size: 13px;
  line-height: 1.6;
}

.reading-round-tag {
  font-size: 11px;
  color: var(--td-text-color-placeholder);
}

.reading-contradiction-desc {
  margin: 0 0 6px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--td-text-color-primary);
}

.reading-contradiction-side {
  display: flex;
  gap: 6px;
  margin-top: 6px;
}

.reading-contradiction-label {
  flex-shrink: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--td-text-color-secondary);
}

.reading-final-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin: 10px 0;
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.reading-queue-hint {
  margin: 6px 0;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.reading-thought-card {
  padding: 10px 12px;
  margin-bottom: 14px;
  background: var(--td-bg-color-secondarycontainer);
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
}

.reading-thought-loading {
  display: flex;
  gap: 10px;
  align-items: center;
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.reading-thought-head {
  display: flex;
  gap: 8px;
  align-items: center;
  width: 100%;
  padding: 0;
  font-family: inherit;
  color: var(--td-text-color-primary);
  text-align: left;
  cursor: pointer;
  background: transparent;
  border: 0;
}

.reading-thought-title {
  font-size: 13px;
  font-weight: 600;
}

.reading-thought-meta {
  margin-left: auto;
  font-size: 11px;
  color: var(--td-text-color-secondary);
}

.reading-thought-caret {
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.reading-thought-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 10px;
}

.reading-thought-phase {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.reading-thought-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--td-brand-color);
}

.reading-thought-phase p,
.reading-thought-phase ul {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--td-text-color-primary);
}

.reading-thought-phase ul {
  padding-left: 16px;
}

.reading-thought-reasoning {
  max-height: 280px;
  padding: 8px;
  margin: 0;
  overflow: auto;
  font-family: inherit;
  font-size: 12px;
  line-height: 1.7;
  color: var(--td-text-color-primary);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
  background: var(--td-bg-color-container);
  border-radius: 6px;
  scrollbar-width: none;
}

.reading-thought-reasoning::-webkit-scrollbar {
  display: none;
}

.reading-thought-why {
  color: var(--td-text-color-secondary);
}

.reading-edited-badge {
  padding: 0 5px;
  margin-left: 4px;
  font-size: 10px;
  color: var(--td-brand-color);
  background: var(--td-brand-color-light);
  border-radius: 999px;
}

.reading-contradiction-head {
  margin-bottom: 6px;
}

.reading-contradiction-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 8px;
}

.reading-contradiction-column {
  min-width: 0;
}

.reading-contradiction-column-head {
  margin-bottom: 2px;
  font-size: 11px;
  font-weight: 600;
  color: var(--td-text-color-secondary);
}

.reading-resolution-text {
  padding: 6px 8px;
  margin: 8px 0 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--td-text-color-primary);
  background: var(--td-bg-color-secondarycontainer);
  border-radius: 6px;
}

.reading-contradiction-resolve {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 8px;
}

.reading-report-editor {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.reading-evidence-float {
  position: fixed;
  z-index: 3000;
  width: 420px;
  max-height: 320px;
  padding: 12px 14px;
  overflow: auto;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  box-shadow: var(--td-shadow-2);
}

.reading-evidence-float-title {
  margin-bottom: 6px;
  font-size: 12px;
  font-weight: 600;
  color: var(--td-brand-color);
}

.reading-evidence-float-muted,
.reading-evidence-float-error {
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.reading-evidence-float-error {
  color: var(--td-error-color);
}

.reading-evidence-float-body {
  font-size: 13px;
  line-height: 1.7;
  color: var(--td-text-color-primary);
  white-space: pre-wrap;
  word-break: break-word;
}

.reading-evidence-hit {
  padding: 0 2px;
  color: var(--td-text-color-primary);
  background: var(--td-brand-color-light);
  border-radius: 3px;
}

.reading-evidence-float-hint {
  margin-top: 8px;
  font-size: 11px;
  color: var(--td-text-color-placeholder);
}

@media (max-width: 1180px) {
  .reading-body,
  .reading-body.notes-collapsed,
  .reading-body.plan-collapsed,
  .reading-body.plan-collapsed.notes-collapsed {
    grid-template-columns: 1fr;
  }

  .reading-progress,
  .reading-notes {
    position: static;
    max-height: none;
  }

  .reading-expand {
    display: none;
  }

  .reading-contradiction-grid {
    grid-template-columns: 1fr;
  }

  .reading-resizer {
    display: none;
  }

  .reading-evidence-float {
    right: 12px;
    left: 12px !important;
    width: auto;
    max-width: none;
  }
}
</style>
