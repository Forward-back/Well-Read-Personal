package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"gaze/internal/types"
)

func TestParseReadingFeynmanResponse(t *testing.T) {
	parsed, err := parseReadingFeynmanResponse(`{"score":82,"coverage":80,"accuracy":90,"evidence_score":75,"summary":"覆盖主要流程，漏了异常分支。","omissions":[{"text":"异常分支","evidence":[{"chunk_id":"c1","quote":"异常时回滚"}]}],"errors":[]}`)
	require.NoError(t, err)
	require.Equal(t, 82, parsed.Score)
	require.Len(t, parsed.Omissions, 1)

	_, err = parseReadingFeynmanResponse(`{"score":80}`)
	require.Error(t, err, "an empty evaluation must be rejected")
}

func TestStrictReadingFeynmanPointsDropsInvalidEvidence(t *testing.T) {
	chunkInfo := map[string]*readingChunkInfo{
		"c1": {Chunk: &types.Chunk{ID: "c1", Content: "退款需在30天内发起。"}, KnowledgeID: "doc1"},
	}
	chunkSection := map[string]string{"c1": "s1"}
	points, evidence := strictReadingFeynmanPoints([]readingFeynmanPointResponse{
		{Text: "漏掉时限", Evidence: []readingEvidenceRef{{ChunkID: "c1", Quote: "退款需在30天内发起"}}},
		{Text: "引用不存在", Evidence: []readingEvidenceRef{{ChunkID: "c1", Quote: "退款可以随时发起"}}},
		{Text: " ", Evidence: []readingEvidenceRef{{ChunkID: "c1", Quote: "退款需在30天内发起"}}},
	}, chunkInfo, chunkSection)
	require.Len(t, points, 1)
	require.Equal(t, "漏掉时限", points[0].Text)
	require.Len(t, evidence, 1)
}

func TestBuildReadingArgumentChainMermaid(t *testing.T) {
	evidenceA := mustJSON(t, []types.ReadingEvidence{{
		KnowledgeID: "doc1", ChunkID: "c1", Quote: "退款需在30天内发起",
	}})
	evidenceB := mustJSON(t, []types.ReadingEvidence{{
		KnowledgeID: "doc1", ChunkID: "c2", Quote: "退款可随时发起",
	}})
	run := &types.ReadingRun{
		Final: mustJSON(t, map[string]any{
			"argument_chain": []map[string]any{{
				"claim": "退款有明确时限",
				"evidence": []map[string]any{{
					"chunk_id": "c1", "quote": "退款需在30天内发起",
				}},
			}},
		}),
		Contradictions: []*types.ReadingContradiction{{
			Description: "两处退款时限冲突",
			EvidenceA:   evidenceA,
			EvidenceB:   evidenceB,
			Status:      "open",
		}},
	}
	graph, nodes := buildReadingArgumentChainMermaid(run)
	require.Contains(t, graph, "flowchart LR")
	require.Contains(t, graph, "C1[\"退款有明确时限\"]:::claim")
	require.Contains(t, graph, "R1{\"两处退款时限冲突\"}:::rebuttal")
	require.Contains(t, graph, ":::evidence")
	require.NotEmpty(t, nodes)
}
