package service

import (
	"strings"
	"testing"

	"gaze/internal/types"
)

func TestBuildDocSectionsWindowsAndSegments(t *testing.T) {
	knowledge := &types.Knowledge{ID: "doc1", Title: "测试制度"}
	chunks := []*types.Chunk{
		{ID: "c1", ContextHeader: "第一章", Content: strings.Repeat("甲", 100)},
		{ID: "c2", ContextHeader: "第一章", Content: strings.Repeat("乙", 100)},
		{ID: "c3", ContextHeader: "第二章", Content: strings.Repeat("丙", 30)},
	}
	sections := buildDocSections(knowledge, chunks, 60)
	if len(sections) < 4 {
		t.Fatalf("expected multiple windows, got %d", len(sections))
	}
	if sections[0].ID != "doc1-0" {
		t.Fatalf("unexpected first section id: %s", sections[0].ID)
	}
	validChunks := map[string]bool{"c1": true, "c2": true, "c3": true}
	for _, section := range sections {
		if len(section.Segments) == 0 {
			t.Fatalf("section %s has no segments", section.ID)
		}
		if strings.TrimSpace(section.Content) == "" {
			t.Fatalf("section %s has empty content", section.ID)
		}
		if len([]rune(section.Content)) > 130 {
			t.Fatalf("section %s unexpectedly large: %d runes", section.ID, len([]rune(section.Content)))
		}
		for _, segment := range section.Segments {
			if !validChunks[segment.ID] {
				t.Fatalf("unexpected segment chunk id %s", segment.ID)
			}
		}
	}
}

func TestReadingQueryTokensChineseBigrams(t *testing.T) {
	tokens := readingQueryTokens("南沙金融30条")
	found := map[string]bool{}
	for _, token := range tokens {
		found[token] = true
	}
	for _, expected := range []string{"南沙", "沙金", "金融"} {
		if !found[expected] {
			t.Fatalf("expected token %q in %v", expected, tokens)
		}
	}
}

func TestReadingQuoteContained(t *testing.T) {
	content := "本制度适用于所有收单机构。退款需在30天内发起。"
	if !readingQuoteContained("退款需在30天内发起", content) {
		t.Fatal("exact quote should be contained")
	}
	if !readingQuoteContained("退款需在 30 天内发起。", content) {
		t.Fatal("quote with whitespace/punctuation differences should be contained")
	}
	if readingQuoteContained("本制度不适用于收单机构", content) {
		t.Fatal("paraphrase must not be accepted as a verbatim quote")
	}
	if readingQuoteContained("退款", content) {
		t.Fatal("too-short quote must not be accepted")
	}
}

func TestReadingCosineSimilarity(t *testing.T) {
	if score := readingCosineSimilarity([]float32{1, 0}, []float32{1, 0}); score < 0.999 {
		t.Fatalf("identical vectors score = %f, want 1", score)
	}
	if score := readingCosineSimilarity([]float32{1, 0}, []float32{0, 1}); score != 0 {
		t.Fatalf("orthogonal vectors score = %f, want 0", score)
	}
	if score := readingCosineSimilarity([]float32{0, 0}, []float32{1, 0}); score != 0 {
		t.Fatalf("zero vector score = %f, want 0", score)
	}
	if score := readingCosineSimilarity([]float32{1}, []float32{1, 0}); score != 0 {
		t.Fatalf("dimension mismatch score = %f, want 0", score)
	}
}

func TestReadingTopIndexes(t *testing.T) {
	indexes := readingTopIndexes([]float32{0.1, 0.9, 0.5}, 2)
	if len(indexes) != 2 || indexes[0] != 1 || indexes[1] != 2 {
		t.Fatalf("unexpected top indexes: %v", indexes)
	}
	if indexes := readingTopIndexes([]float32{0.3}, 5); len(indexes) != 1 || indexes[0] != 0 {
		t.Fatalf("limit larger than input failed: %v", indexes)
	}
}

func TestBuildReadingRoundOutcomeGates(t *testing.T) {
	service := &readingRunService{}
	run := &types.ReadingRun{ID: "run1", TenantID: 1}
	section := &types.ReadingSection{ID: "doc1-0", KnowledgeID: "doc1"}
	chunkInfo := map[string]*readingChunkInfo{
		"c1": {
			Chunk:       &types.Chunk{ID: "c1", Content: "本制度适用于所有收单机构。退款需在30天内发起。"},
			KnowledgeID: "doc1", DocumentTitle: "测试制度", Heading: "第一章",
		},
		"c2": {
			Chunk:       &types.Chunk{ID: "c2", Content: "另一制度要求退款需在30天内发起。"},
			KnowledgeID: "doc1", DocumentTitle: "测试制度", Heading: "第二章",
		},
		"c3": {
			Chunk:       &types.Chunk{ID: "c3", Content: "二维码退款仅需订单号即可发起。"},
			KnowledgeID: "doc2", DocumentTitle: "另一制度", Heading: "退款",
		},
		"c4": {
			Chunk:       &types.Chunk{ID: "c4", Content: "二维码退款必须提供原始交易信息。"},
			KnowledgeID: "doc2", DocumentTitle: "另一制度", Heading: "退款",
		},
	}
	merged := &readingRoundResponse{
		Understanding: "本轮理解：退款规则存在跨文档差异。",
		Questions: []readingLLMQuestion{
			{Text: "退款期限是多久？", SearchQuery: "退款 30天", RecommendedAnswer: "30 天"},
			// Every question must carry the model's recommended reading; the
			// engine only keeps questions that offer one.
			{Text: "需要额外证书吗？", SearchQuery: "证书", RecommendedAnswer: "需要额外证书"},
		},
		Answers: []readingLLMAnswer{
			{
				QuestionIndex: 0, QuestionText: "退款期限是多久？", Answer: "30 天", Supported: true,
				Evidence: []readingEvidenceRef{{ChunkID: "c1", Quote: "退款需在30天内发起"}},
			},
		},
		Conclusions: []readingLLMConclusion{
			{Text: "收单机构适用本制度。", Evidence: []readingEvidenceRef{{ChunkID: "c1", Quote: "本制度适用于所有收单机构"}}},
			{Text: "二维码退款仅需订单号。", Evidence: []readingEvidenceRef{{ChunkID: "c3", Quote: "二维码退款仅需订单号即可发起"}}},
			{Text: "这是一条没有证据的幻觉结论。", Evidence: []readingEvidenceRef{{ChunkID: "missing", Quote: "不存在"}}},
		},
		Terms: []readingLLMTerm{
			{Term: "收单机构", Definition: "受理商户交易的机构。", Source: []readingEvidenceRef{{ChunkID: "c1", Quote: "本制度适用于所有收单机构"}}},
		},
		Contradictions: []readingLLMContradiction{
			{
				Description: "二维码退款所需材料在两个制度中不一致。",
				EvidenceA:   []readingEvidenceRef{{ChunkID: "c3", Quote: "二维码退款仅需订单号即可发起"}},
				EvidenceB:   []readingEvidenceRef{{ChunkID: "c4", Quote: "二维码退款必须提供原始交易信息"}},
			},
		},
	}
	outcome := service.buildReadingRoundOutcome(
		run, section, 1, merged,
		merged.Questions, merged.Answers, map[string][]types.ReadingEvidence{},
		&readingRoundState{}, chunkInfo, map[string]string{"c1": "doc1-0"},
	)

	if outcome.Understanding == "" {
		t.Fatal("expected understanding")
	}
	if len(outcome.Questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(outcome.Questions))
	}
	if outcome.Questions[0].Status != types.ReadingQuestionAutoAnswered {
		t.Fatalf("expected auto answered, got %s", outcome.Questions[0].Status)
	}
	if outcome.Questions[1].Status != types.ReadingQuestionPending {
		t.Fatalf("expected pending, got %s", outcome.Questions[1].Status)
	}
	if outcome.pendingCount() != 1 {
		t.Fatalf("expected 1 pending question, got %d", outcome.pendingCount())
	}
	if len(outcome.Conclusions) != 3 {
		t.Fatalf("expected 2 evidence-backed conclusions plus auto answer, got %d", len(outcome.Conclusions))
	}
	confirmed := 0
	contradicted := 0
	for _, conclusion := range outcome.Conclusions {
		switch conclusion.Status {
		case types.ReadingConclusionConfirmed:
			confirmed++
		case types.ReadingConclusionContradicted:
			contradicted++
		}
		if strings.Contains(conclusion.Text, "幻觉") {
			t.Fatal("conclusion with missing evidence must be dropped")
		}
	}
	if confirmed != 2 {
		t.Fatalf("expected 2 confirmed conclusions, got %d", confirmed)
	}
	if contradicted != 1 {
		t.Fatalf("expected 1 contradicted conclusion, got %d", contradicted)
	}
	if len(outcome.Terms) != 1 || outcome.Terms[0].Status != types.ReadingTermConfirmed {
		t.Fatalf("expected 1 confirmed term, got %+v", outcome.Terms)
	}
	if len(outcome.Contradictions) != 1 {
		t.Fatalf("expected 1 contradiction, got %d", len(outcome.Contradictions))
	}
}
