package service

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"gaze/internal/types"
)

// buildChunkRowsFromParsed turns chunker output into DB chunk rows: parent rows
// first, then child text rows with parent and prev/next wiring plus document
// metadata. Shared by manual processing and agent edits.
func buildChunkRowsFromParsed(
	knowledge *types.Knowledge,
	parsed []types.ParsedChunk,
	parentChunks []types.ParsedParentChunk,
	metadata map[string]string,
	now time.Time,
) (rows []*types.Chunk, textChunks []*types.Chunk) {
	hasParentChild := len(parentChunks) > 0
	if hasParentChild {
		parentRows := make([]*types.Chunk, len(parentChunks))
		for i, pc := range parentChunks {
			parentRows[i] = &types.Chunk{
				ID:              uuid.NewString(),
				TenantID:        knowledge.TenantID,
				KnowledgeID:     knowledge.ID,
				KnowledgeBaseID: knowledge.KnowledgeBaseID,
				Content:         pc.Content,
				ChunkIndex:      pc.Seq,
				IsEnabled:       true,
				CreatedAt:       now,
				UpdatedAt:       now,
				StartAt:         pc.Start,
				EndAt:           pc.End,
				ChunkType:       types.ChunkTypeParentText,
			}
		}
		for i := range parentRows {
			if i > 0 {
				parentRows[i-1].NextChunkID = parentRows[i].ID
				parentRows[i].PreChunkID = parentRows[i-1].ID
			}
		}
		rows = append(rows, parentRows...)
	}

	for i := range parsed {
		if strings.TrimSpace(parsed[i].Content) == "" {
			continue
		}
		textChunk := &types.Chunk{
			ID:              uuid.NewString(),
			TenantID:        knowledge.TenantID,
			KnowledgeID:     knowledge.ID,
			KnowledgeBaseID: knowledge.KnowledgeBaseID,
			Content:         parsed[i].Content,
			ContextHeader:   parsed[i].ContextHeader,
			ChunkIndex:      int(parsed[i].Seq),
			IsEnabled:       true,
			CreatedAt:       now,
			UpdatedAt:       now,
			StartAt:         int(parsed[i].Start),
			EndAt:           int(parsed[i].End),
			ChunkType:       types.ChunkTypeText,
		}
		textChunk.DocType = knowledge.DocType
		textChunk.OrgName = knowledge.OrgName
		textChunk.ProductName = knowledge.ProductName
		if metadata != nil {
			if textChunk.DocType == "" {
				textChunk.DocType = metadata["doc_type"]
			}
			if textChunk.OrgName == "" {
				textChunk.OrgName = metadata["org_name"]
			}
			if textChunk.ProductName == "" {
				textChunk.ProductName = metadata["product_name"]
			}
		}
		if hasParentChild && parsed[i].ParentIndex >= 0 && parsed[i].ParentIndex < len(parentChunks) {
			// Parent rows were appended first and keep their input order.
			textChunk.ParentChunkID = rows[parsed[i].ParentIndex].ID
		}
		parsed[i].ChunkID = textChunk.ID
		rows = append(rows, textChunk)
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].ChunkIndex < rows[j].ChunkIndex
	})
	textChunks = make([]*types.Chunk, 0, len(parsed))
	for _, chunk := range rows {
		if chunk.ChunkType == types.ChunkTypeText {
			textChunks = append(textChunks, chunk)
		}
	}
	if !hasParentChild {
		for i, chunk := range textChunks {
			if i > 0 {
				textChunks[i-1].NextChunkID = chunk.ID
			}
			if i < len(textChunks)-1 {
				textChunks[i+1].PreChunkID = chunk.ID
			}
		}
	}
	return rows, textChunks
}
