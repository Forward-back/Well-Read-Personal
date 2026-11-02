package types

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Document version sources.
const (
	DocVersionSourceUpload     = "upload"
	DocVersionSourceManualEdit = "manual_edit"
	DocVersionSourceRollback   = "rollback"
	DocVersionSourceReparse    = "reparse"
)

// Engine sync states stored on knowledges.
const (
	EngineSyncStatusSynced  = "synced"
	EngineSyncStatusPending = "pending"
	EngineSyncStatusFailed  = "failed"
)

// DocVersion is one immutable document snapshot. Version labels are V1.{seq}.
type DocVersion struct {
	ID              string    `json:"id"               gorm:"type:varchar(36);primaryKey"`
	TenantID        uint64    `json:"tenant_id"`
	KnowledgeBaseID string    `json:"knowledge_base_id" gorm:"type:varchar(36);index"`
	KnowledgeID     string    `json:"knowledge_id"     gorm:"type:varchar(36);index"`
	VersionSeq      int       `json:"version_seq"`
	VersionLabel    string    `json:"version_label"    gorm:"type:varchar(16)"`
	SourceType      string    `json:"source_type"      gorm:"type:varchar(24);not null"`
	SourceID        string    `json:"source_id"        gorm:"type:varchar(36)"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	ChunkCount      int       `json:"chunk_count"`
	CreatedBy       string    `json:"created_by"       gorm:"type:varchar(36)"`
	CreatedAt       time.Time `json:"created_at"`
	IsCurrent       bool      `json:"is_current"       gorm:"-"`

	CreatedByName string `json:"created_by_name,omitempty" gorm:"-"`
}

// DocVersionChunk is one chunk snapshot inside a DocVersion.
type DocVersionChunk struct {
	ID         string `json:"id"          gorm:"type:varchar(36);primaryKey"`
	TenantID   uint64 `json:"tenant_id"`
	VersionID  string `json:"version_id"  gorm:"type:varchar(36);index"`
	ChunkID    string `json:"chunk_id"    gorm:"type:varchar(36);index"`
	ChunkIndex int    `json:"chunk_index"`
	Content    string `json:"content"`
	IsEnabled  bool   `json:"is_enabled"`
}

// EngineSyncConfig is the tenant-level daily engine resync switch.
type EngineSyncConfig struct {
	Enabled bool `json:"enabled"`
}

// Value implements driver.Valuer for JSONB persistence.
func (c EngineSyncConfig) Value() (driver.Value, error) { return json.Marshal(c) }

// Scan implements sql.Scanner for JSONB persistence.
func (c *EngineSyncConfig) Scan(value interface{}) error {
	if value == nil {
		*c = EngineSyncConfig{}
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	}
	return json.Unmarshal(raw, c)
}

// DocVersionCompareResponse pairs two snapshots for a side-by-side diff.
type DocVersionCompareResponse struct {
	From       *DocVersion        `json:"from"`
	To         *DocVersion        `json:"to"`
	FromChunks []*DocVersionChunk `json:"from_chunks"`
	ToChunks   []*DocVersionChunk `json:"to_chunks"`
}

// RollbackDocVersionRequest optionally records why a rollback happened.
type RollbackDocVersionRequest struct {
	Comment string `json:"comment"`
}

// DocVersionRollbackTx atomically restores a snapshot and records the new
// rollback version.
type DocVersionRollbackTx struct {
	Version               *DocVersion
	KnowledgeID           string
	Chunks                []*Chunk
	NewChunks             []*Chunk
	Revisions             []*ChunkRevision
	DisableChunkIDs       []string
	MarkEngineSyncPending bool
	Now                   time.Time
}
