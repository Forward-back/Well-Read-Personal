package service

import (
	"strings"
	"testing"

	"gaze/internal/types"
)

func TestMergeReadingStructureFallsBackPerSection(t *testing.T) {
	plan := &types.ReadingPlan{Sections: []types.ReadingSection{
		{ID: "s1", Title: "文档", Heading: "第一章"},
		{ID: "s2", Title: "文档", Heading: "第二章"},
	}}
	entries := map[string]readingAgentStructureEntry{
		"s1": {SectionID: "s1", Thesis: "论点一", KnowledgeType: "argument"},
		"x9": {SectionID: "x9", Thesis: "未知章节", KnowledgeType: "concept"},
	}
	structure, matched := mergeReadingStructureEntries(plan, entries)
	if matched != 1 {
		t.Fatalf("matched = %d, want 1", matched)
	}
	if len(structure) != 2 {
		t.Fatalf("structure len = %d, want 2", len(structure))
	}
	if structure[0].Thesis != "论点一" || structure[0].KnowledgeType != types.ReadingKnowledgeTypeArgument {
		t.Fatalf("unexpected first entry: %+v", structure[0])
	}
	if structure[1].SectionID != "s2" || structure[1].Thesis != "第二章" ||
		structure[1].KnowledgeType != types.ReadingKnowledgeTypeUnknown {
		t.Fatalf("missing section must fall back to heading/unknown: %+v", structure[1])
	}
}

func TestReadingAgentTrailingSkips(t *testing.T) {
	cases := []struct {
		name    string
		results []readingAgentSectionResult
		want    int
	}{
		{"none", []readingAgentSectionResult{{Result: "confirmed"}, {Result: "answered"}}, 0},
		{"one", []readingAgentSectionResult{{Result: "confirmed"}, {Result: "skipped"}}, 1},
		{"two", []readingAgentSectionResult{{Result: "skipped"}, {Result: "skipped"}}, 2},
		{"reset", []readingAgentSectionResult{{Result: "skipped"}, {Result: "confirmed"}, {Result: "skipped"}}, 1},
	}
	for _, tc := range cases {
		if got := readingAgentTrailingSkips(tc.results); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestReadingStrictEvidenceDropsInvalidRefs(t *testing.T) {
	chunkInfo := map[string]*readingChunkInfo{
		"c1": {Chunk: &types.Chunk{ID: "c1", Content: "缓存键固定为 tenant_id 与 document_id。"}, KnowledgeID: "doc1"},
		"c2": {Chunk: &types.Chunk{ID: "c2", Content: "重试间隔未规定。"}, KnowledgeID: "doc1"},
	}
	chunkSection := map[string]string{"c1": "s1", "c2": "s1"}
	refs := []readingEvidenceRef{
		{ChunkID: "c1", Quote: "缓存键固定为 tenant_id 与 document_id"},
		{ChunkID: "c1", Quote: "模型改写的错误引用"},
		{ChunkID: "missing", Quote: "重试间隔未规定。"},
		{ChunkID: "c1", Quote: "缓存键固定为 tenant_id 与 document_id"},
	}
	evidence := readingStrictEvidence(refs, chunkInfo, chunkSection)
	if len(evidence) != 1 {
		t.Fatalf("evidence len = %d, want 1: %+v", len(evidence), evidence)
	}
	if evidence[0].ChunkID != "c1" || evidence[0].SectionID != "s1" {
		t.Fatalf("unexpected evidence: %+v", evidence[0])
	}
}

func TestParseReadingAgentStructureRejectsEmpty(t *testing.T) {
	if _, err := parseReadingAgentStructure("not json"); err == nil {
		t.Fatal("non-JSON must fail")
	}
	if _, err := parseReadingAgentStructure(`{"sections":[]}`); err == nil {
		t.Fatal("empty sections must fail")
	}
	parsed, err := parseReadingAgentStructure(
		"```json\n{\"sections\":[{\"section_id\":\"s1\",\"thesis\":\"论点\",\"knowledge_type\":\"concept\"}]}\n```",
	)
	if err != nil {
		t.Fatalf("valid structure failed: %v", err)
	}
	if len(parsed.Sections) != 1 || parsed.Sections[0].SectionID != "s1" {
		t.Fatalf("unexpected parse: %+v", parsed)
	}
}

func TestParseReadingAgentSynthesisRequiresContent(t *testing.T) {
	if _, err := parseReadingAgentSynthesis(`{}`); err == nil {
		t.Fatal("empty synthesis must fail")
	}
	parsed, err := parseReadingAgentSynthesis(
		`{"summary":"综合摘要","argument_chain":[],"contradictions":[],"term_finalizations":[],"open_questions":["未决"]}`,
	)
	if err != nil {
		t.Fatalf("valid synthesis failed: %v", err)
	}
	if parsed.Summary != "综合摘要" || len(parsed.OpenQuestions) != 1 {
		t.Fatalf("unexpected parse: %+v", parsed)
	}
}

func TestParseReadingAgentSelfTests(t *testing.T) {
	parsed, err := parseReadingAgentSelfTests(
		`{"tests":[{"section_id":"s1","question":"缓存键由哪些字段组成？","answer":"tenant_id、document_id、chunk_index","explanation":"","evidence":[{"chunk_id":"c1","quote":"缓存键固定为 tenant_id + document_id + chunk_index"}]}]}`,
	)
	if err != nil {
		t.Fatalf("valid self-test JSON failed: %v", err)
	}
	if len(parsed.Tests) != 1 || parsed.Tests[0].Question == "" {
		t.Fatalf("unexpected parse: %+v", parsed)
	}
	if _, err := parseReadingAgentSelfTests("not json"); err == nil {
		t.Fatal("non-JSON must fail")
	}
}

func TestNormalizeReadingKnowledgeType(t *testing.T) {
	if got := normalizeReadingKnowledgeType(" Process "); got != types.ReadingKnowledgeTypeProcess {
		t.Fatalf("got %q", got)
	}
	if got := normalizeReadingKnowledgeType("unknown-type"); got != types.ReadingKnowledgeTypeUnknown {
		t.Fatalf("got %q", got)
	}
}

func TestReadingAgentQuietPromptMarksNoQuestions(t *testing.T) {
	plan := &types.ReadingPlan{
		Depth:     types.ReadingDepthStandard,
		Structure: []types.ReadingSectionStructure{{SectionID: "s1", Thesis: "论点", KnowledgeType: "concept"}},
	}
	section := &types.ReadingSection{ID: "s1", Title: "文档", Heading: "第一章", Content: strings.Repeat("原文", 20)}
	run := &types.ReadingRun{Plan: types.JSON(`{"goal":"目标","depth":"standard"}`)}
	prompt := buildReadingAgentRoundPrompt(run, plan, section, &readingAgentMemoryState{}, true, 2, nil, nil)
	if !strings.Contains(prompt, "安静模式") {
		t.Fatalf("quiet prompt must mention quiet mode: %s", prompt[:120])
	}
}
