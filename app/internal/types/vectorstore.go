package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gaze/internal/errors"
	"gaze/internal/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EnvStoreIDPrefix is the prefix for virtual env store IDs.
const EnvStoreIDPrefix = "__env_"

const (
	envTencentVectorDBReplicaNumber     = "TENCENT_VECTORDB_REPLICA_NUMBER"
	defaultTencentVectorDBReplicaNumber = 1
)

// IsEnvStoreID checks if the given ID is an env store virtual ID.
func IsEnvStoreID(id string) bool {
	return strings.HasPrefix(id, EnvStoreIDPrefix)
}

// EnvLookupFunc is a function type for looking up environment variables.
// In production: os.Getenv, in tests: custom lookup function.
type EnvLookupFunc func(string) string

// VectorStore represents a configured vector database instance for a workspace.
// Each workspace can register multiple VectorStore entries (even of the same engine type)
// to support multi-store scenarios (e.g., ES-hot + ES-warm clusters).
type VectorStore struct {
	// Unique identifier (UUID, auto-generated)
	ID string `yaml:"id" json:"id" gorm:"type:varchar(36);primaryKey"`
	// Workspace ID for scoping
	TenantID uint64 `yaml:"tenant_id" json:"tenant_id"`
	// User-friendly name, e.g., "elasticsearch-hot"
	Name string `yaml:"name" json:"name" gorm:"type:varchar(255);not null"`
	// Engine type: postgres, elasticsearch, qdrant, milvus, weaviate, sqlite
	EngineType RetrieverEngineType `yaml:"engine_type" json:"engine_type" gorm:"type:varchar(50);not null"`
	// Driver-specific connection parameters (sensitive fields encrypted with AES-GCM)
	ConnectionConfig ConnectionConfig `yaml:"connection_config" json:"connection_config" gorm:"type:json"`
	// Optional index/collection configuration (engine-specific defaults if empty)
	IndexConfig IndexConfig `yaml:"index_config" json:"index_config" gorm:"type:json"`
	// Timestamps
	CreatedAt time.Time      `yaml:"created_at" json:"created_at"`
	UpdatedAt time.Time      `yaml:"updated_at" json:"updated_at"`
	DeletedAt gorm.DeletedAt `yaml:"deleted_at" json:"deleted_at" gorm:"index"`
}

// TableName returns the table name for VectorStore
func (VectorStore) TableName() string {
	return "vector_stores"
}

// BeforeCreate is a GORM hook that runs before creating a new record.
// Automatically generates a UUID for new vector stores.
func (v *VectorStore) BeforeCreate(tx *gorm.DB) error {
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	return nil
}

// validEngineTypes defines the engine types that can be registered as a
// DB-managed VectorStore (i.e., persisted to the vector_stores table and
// listed in GetVectorStoreTypes for the UI dropdown). The personal edition
// supports only the application's own PostgreSQL database and local SQLite;
// both can also be reached as env stores via RETRIEVE_DRIVER.
var validEngineTypes = map[RetrieverEngineType]bool{
	PostgresRetrieverEngineType: true,
	SQLiteRetrieverEngineType:   true,
}

// IsValidEngineType checks whether the given engine type is valid for VectorStore.
func IsValidEngineType(t RetrieverEngineType) bool {
	return validEngineTypes[t]
}

// Validate checks required fields and engine type validity.
func (v *VectorStore) Validate() error {
	if v.Name == "" {
		return errors.NewValidationError("name is required")
	}
	if !validEngineTypes[v.EngineType] {
		return errors.NewValidationError(fmt.Sprintf("unsupported engine type: %s", v.EngineType))
	}
	if v.TenantID == 0 {
		return errors.NewValidationError("tenant_id is required")
	}
	return nil
}

// ---------------------------------------------------------------------------
// ConnectionConfig
// ---------------------------------------------------------------------------

// ConnectionConfig holds driver-specific connection parameters.
// Sensitive fields (Password, APIKey) are encrypted with AES-GCM at rest.
type ConnectionConfig struct {
	// Common
	Addr     string `yaml:"addr" json:"addr,omitempty"`
	Username string `yaml:"username" json:"username,omitempty"`
	Password string `yaml:"password" json:"password,omitempty"` // AES-GCM encrypted
	APIKey   string `yaml:"api_key" json:"api_key,omitempty"`   // AES-GCM encrypted
	// InsecureSkipVerify disables TLS certificate verification when
	// talking to the backing store over HTTPS. Defaults to false
	// (secure). Set to true ONLY for self-signed development clusters;
	// production deployments should provide trusted certificates via
	// the system CA pool. Cross-driver applicable but currently only
	// the OpenSearch driver (Phase 3) reads this field. Note: this
	// differs from the Qdrant-specific UseTLS below, which *enables*
	// TLS on gRPC connections — InsecureSkipVerify only controls
	// *verification* of an already-TLS connection.
	InsecureSkipVerify bool `yaml:"insecure_skip_verify" json:"insecure_skip_verify,omitempty"`
	// Qdrant
	Host   string `yaml:"host" json:"host,omitempty"`
	Port   int    `yaml:"port" json:"port,omitempty"`
	UseTLS bool   `yaml:"use_tls" json:"use_tls,omitempty"`
	// Weaviate
	GrpcAddress string `yaml:"grpc_address" json:"grpc_address,omitempty"`
	Scheme      string `yaml:"scheme" json:"scheme,omitempty"`
	// Database name used by engines that support database-level namespaces
	// (currently Milvus, Tencent VectorDB, and Doris).
	Database string `yaml:"database" json:"database,omitempty"`
	// Postgres
	UseDefaultConnection bool `yaml:"use_default_connection" json:"use_default_connection,omitempty"`
	// Doris: HTTP port for Stream Load API (FE default 8030).
	// Addr is reused for the MySQL protocol "host:9030"; HTTPPort + the host of Addr
	// together form the FE HTTP endpoint used by Stream Load.
	HTTPPort int `yaml:"http_port" json:"http_port,omitempty"`
	// Version is the detected server version (e.g., "7.10.1", "16.2", "1.12.6").
	// Auto-populated by TestConnection on successful connectivity check.
	Version string `yaml:"version" json:"version,omitempty"`
}

// Value implements the driver.Valuer interface.
// Encrypts Password and APIKey before persisting to database.
func (c ConnectionConfig) Value() (driver.Value, error) {
	if key := utils.GetAESKey(); key != nil {
		if c.Password != "" {
			if encrypted, err := utils.EncryptAESGCM(c.Password, key); err == nil {
				c.Password = encrypted
			}
		}
		if c.APIKey != "" {
			if encrypted, err := utils.EncryptAESGCM(c.APIKey, key); err == nil {
				c.APIKey = encrypted
			}
		}
	}
	return json.Marshal(c)
}

// Scan implements the sql.Scanner interface.
// Decrypts Password and APIKey after loading from database.
func (c *ConnectionConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	if err := json.Unmarshal(b, c); err != nil {
		return err
	}
	password, err := utils.DecryptStoredSecret(c.Password)
	if err != nil {
		return fmt.Errorf("decrypt vector store connection password: %w", err)
	}
	c.Password = password
	apiKey, err := utils.DecryptStoredSecret(c.APIKey)
	if err != nil {
		return fmt.Errorf("decrypt vector store connection api_key: %w", err)
	}
	c.APIKey = apiKey
	return nil
}

// GetEndpoint returns a normalized endpoint string for duplicate detection.
func (c ConnectionConfig) GetEndpoint() string {
	if c.Addr != "" {
		if c.Database != "" {
			return c.Addr + "/" + c.Database
		}
		return c.Addr
	}
	if c.Host != "" {
		port := c.Port
		if port == 0 {
			port = 6334 // Qdrant default port
		}
		return fmt.Sprintf("%s:%d", c.Host, port)
	}
	if c.UseDefaultConnection {
		return "__default_postgres__"
	}
	return ""
}

// MaskSensitiveFields returns a copy with Password and APIKey replaced by the
// shared RedactedSecretPlaceholder. Empty values stay empty so the frontend
// can distinguish "set (hidden)" from "not set" without an extra flag.
func (c ConnectionConfig) MaskSensitiveFields() ConnectionConfig {
	masked := c
	if masked.Password != "" {
		masked.Password = RedactedSecretPlaceholder
	}
	if masked.APIKey != "" {
		masked.APIKey = RedactedSecretPlaceholder
	}
	return masked
}

// ---------------------------------------------------------------------------
// IndexConfig
// ---------------------------------------------------------------------------

// IndexConfig holds optional index/collection configuration for the vector store.
// If empty, engine-specific defaults are used.
type IndexConfig struct {
	// --- Existing fields ---
	IndexName        string `yaml:"index_name" json:"index_name,omitempty"`                 // ES, OpenSearch
	NumberOfShards   int    `yaml:"number_of_shards" json:"number_of_shards,omitempty"`     // ES, OpenSearch
	NumberOfReplicas int    `yaml:"number_of_replicas" json:"number_of_replicas,omitempty"` // ES, OpenSearch
	CollectionPrefix string `yaml:"collection_prefix" json:"collection_prefix,omitempty"`   // Qdrant, Weaviate
	CollectionName   string `yaml:"collection_name" json:"collection_name,omitempty"`       // Milvus

	// --- Scalability fields ---
	ShardNumber       int `yaml:"shard_number" json:"shard_number,omitempty"`               // Qdrant: number of shards per collection
	ReplicationFactor int `yaml:"replication_factor" json:"replication_factor,omitempty"`   // Qdrant, Weaviate: number of replicas
	ShardsNum         int `yaml:"shards_num" json:"shards_num,omitempty"`                   // Milvus: number of shards per collection (CreateCollection)
	ReplicaNumber     int `yaml:"replica_number" json:"replica_number,omitempty"`           // Milvus LoadCollection / Tencent VectorDB CreateCollection replicas
	DesiredShardCount int `yaml:"desired_shard_count" json:"desired_shard_count,omitempty"` // Weaviate: number of shards per collection
	BucketsNum        int `yaml:"buckets_num" json:"buckets_num,omitempty"`                 // Doris: number of buckets per table (DISTRIBUTED BY HASH ... BUCKETS N)
	ReplicationNum    int `yaml:"replication_num" json:"replication_num,omitempty"`         // Doris: replication_num PROPERTIES

	// --- OpenSearch k-NN HNSW fields ---
	// All omitempty so other engines' serialized IndexConfig is unchanged.
	// Zero / empty values fall back to the driver defaults in buildInternalCfg.
	HNSWM              int    `yaml:"hnsw_m" json:"hnsw_m,omitempty"`                             // OpenSearch: HNSW graph degree (M)
	HNSWEFConstruction int    `yaml:"hnsw_ef_construction" json:"hnsw_ef_construction,omitempty"` // OpenSearch: HNSW index-build candidate list size
	HNSWEFSearch       int    `yaml:"hnsw_ef_search" json:"hnsw_ef_search,omitempty"`             // OpenSearch: HNSW search candidate list size (faiss; lucene reads at query time)
	KNNEngine          string `yaml:"knn_engine" json:"knn_engine,omitempty"`                     // OpenSearch: k-NN backend ("lucene" | "faiss")
}

// Value implements the driver.Valuer interface.
func (c IndexConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements the sql.Scanner interface.
func (c *IndexConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, c)
}

// GetIndexNameOrDefault returns the effective index/collection name,
// falling back to engine-specific defaults when the user has not specified one.
func (c IndexConfig) GetIndexNameOrDefault(engineType RetrieverEngineType) string {
	switch engineType {
	case PostgresRetrieverEngineType:
		if c.CollectionPrefix != "" {
			return c.CollectionPrefix
		}
		if c.CollectionName != "" {
			return c.CollectionName
		}
		return "gaze_embeddings"
	case SQLiteRetrieverEngineType:
		if c.CollectionPrefix != "" {
			return c.CollectionPrefix
		}
		if c.CollectionName != "" {
			return c.CollectionName
		}
		return "gaze_embeddings"
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// IndexConfig — getter helpers (pointer receiver for nil safety)
// ---------------------------------------------------------------------------

// GetNumberOfShards returns the configured number_of_shards, or def if unset/zero.
func (c *IndexConfig) GetNumberOfShards(def int) int {
	if c != nil && c.NumberOfShards > 0 {
		return c.NumberOfShards
	}
	return def
}

// GetNumberOfReplicas returns the configured number_of_replicas, or def if unset/zero.
// Note: 0 replicas cannot be distinguished from "not set" because the int field with
// json:"omitempty" omits zero values. If zero-replica support is needed in the future,
// change the field type to *int. Currently 0 is treated as "use server default".
func (c *IndexConfig) GetNumberOfReplicas(def int) int {
	if c != nil && c.NumberOfReplicas > 0 {
		return c.NumberOfReplicas
	}
	return def
}

// GetShardNumber returns the configured shard_number (Qdrant), or def if unset/zero.
func (c *IndexConfig) GetShardNumber(def int) int {
	if c != nil && c.ShardNumber > 0 {
		return c.ShardNumber
	}
	return def
}

// GetReplicationFactor returns the configured replication_factor (Qdrant, Weaviate), or def if unset/zero.
func (c *IndexConfig) GetReplicationFactor(def int) int {
	if c != nil && c.ReplicationFactor > 0 {
		return c.ReplicationFactor
	}
	return def
}

// GetShardsNum returns the configured shards_num (Milvus), or def if unset/zero.
func (c *IndexConfig) GetShardsNum(def int) int {
	if c != nil && c.ShardsNum > 0 {
		return c.ShardsNum
	}
	return def
}

// GetReplicaNumber returns the configured replica_number, or def if unset/zero.
// Milvus applies it at LoadCollection time; Tencent VectorDB applies it at
// CreateCollection time. It controls read HA/throughput replicas.
func (c *IndexConfig) GetReplicaNumber(def int) int {
	if c != nil && c.ReplicaNumber > 0 {
		return c.ReplicaNumber
	}
	return def
}

// GetDesiredShardCount returns the configured desired_shard_count (Weaviate), or def if unset/zero.
func (c *IndexConfig) GetDesiredShardCount(def int) int {
	if c != nil && c.DesiredShardCount > 0 {
		return c.DesiredShardCount
	}
	return def
}

// GetBucketsNum returns the configured buckets_num (Doris), or def if unset/zero.
func (c *IndexConfig) GetBucketsNum(def int) int {
	if c != nil && c.BucketsNum > 0 {
		return c.BucketsNum
	}
	return def
}

// GetReplicationNum returns the configured replication_num (Doris), or def if unset/zero.
func (c *IndexConfig) GetReplicationNum(def int) int {
	if c != nil && c.ReplicationNum > 0 {
		return c.ReplicationNum
	}
	return def
}

// ---------------------------------------------------------------------------
// IndexConfig — resolve helpers (for Repository layer, with env var fallback)
// ---------------------------------------------------------------------------

// ResolveIndexName returns the index name from IndexConfig, falling back to env var and then default.
// Used by Repository constructors. For service-layer duplicate checking, use GetIndexNameOrDefault instead.
func ResolveIndexName(ic *IndexConfig, envKey, defaultVal string) string {
	if ic != nil && ic.IndexName != "" {
		return ic.IndexName
	}
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return defaultVal
}

// ResolveCollectionName returns the collection name from IndexConfig, falling back to env var and then default.
// Priority: CollectionPrefix > CollectionName > env var > defaultVal.
// CollectionPrefix is checked first because Qdrant/Weaviate use it as the base name.
// CollectionName (Milvus) is checked second. If both are set, CollectionPrefix wins —
// this is safe because each VectorStore has a single engine type, so only one field is relevant.
func ResolveCollectionName(ic *IndexConfig, envKey, defaultVal string) string {
	if ic != nil {
		if ic.CollectionPrefix != "" {
			return ic.CollectionPrefix
		}
		if ic.CollectionName != "" {
			return ic.CollectionName
		}
	}
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return defaultVal
}

// OptionalUint32 converts int to *uint32 for Qdrant SDK.
// Returns nil for values <= 0, which tells Qdrant to use its server default.
func OptionalUint32(v int) *uint32 {
	if v <= 0 {
		return nil
	}
	u := uint32(v)
	return &u
}

// ---------------------------------------------------------------------------
// IndexConfig — validation
// ---------------------------------------------------------------------------

const (
	// maxShards is the upper bound for shard-related configuration values.
	maxShards = 64
	// maxReplicas is the upper bound for replication-related configuration values.
	maxReplicas = 10
)

// validIndexNamePattern restricts index/collection names to safe characters.
// Must start with a letter, followed by alphanumeric, underscore, or hyphen. Max 128 chars.
var validIndexNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,127}$`)

// ValidateIndexConfig checks IndexConfig fields for safe values.
// Call this from the service layer before persisting a VectorStore.
func ValidateIndexConfig(ic IndexConfig) error {
	// Validate string fields (index/collection names)
	if ic.IndexName != "" && !validIndexNamePattern.MatchString(ic.IndexName) {
		return errors.NewValidationError(
			"index_name must start with a letter and contain only alphanumeric, underscore, or hyphen characters (max 128)")
	}
	if ic.CollectionPrefix != "" && !validIndexNamePattern.MatchString(ic.CollectionPrefix) {
		return errors.NewValidationError(
			"collection_prefix must start with a letter and contain only alphanumeric, underscore, or hyphen characters (max 128)")
	}
	if ic.CollectionName != "" && !validIndexNamePattern.MatchString(ic.CollectionName) {
		return errors.NewValidationError(
			"collection_name must start with a letter and contain only alphanumeric, underscore, or hyphen characters (max 128)")
	}

	// Validate numeric fields (shards/replicas) — must be within safe bounds
	if ic.NumberOfShards < 0 || ic.NumberOfShards > maxShards {
		return errors.NewValidationError(fmt.Sprintf("number_of_shards must be between 0 and %d", maxShards))
	}
	if ic.NumberOfReplicas < 0 || ic.NumberOfReplicas > maxReplicas {
		return errors.NewValidationError(fmt.Sprintf("number_of_replicas must be between 0 and %d", maxReplicas))
	}
	if ic.ShardNumber < 0 || ic.ShardNumber > maxShards {
		return errors.NewValidationError(fmt.Sprintf("shard_number must be between 0 and %d", maxShards))
	}
	if ic.ReplicationFactor < 0 || ic.ReplicationFactor > maxReplicas {
		return errors.NewValidationError(fmt.Sprintf("replication_factor must be between 0 and %d", maxReplicas))
	}
	if ic.ShardsNum < 0 || ic.ShardsNum > maxShards {
		return errors.NewValidationError(fmt.Sprintf("shards_num must be between 0 and %d", maxShards))
	}
	if ic.ReplicaNumber < 0 || ic.ReplicaNumber > maxReplicas {
		return errors.NewValidationError(fmt.Sprintf("replica_number must be between 0 and %d", maxReplicas))
	}
	if ic.DesiredShardCount < 0 || ic.DesiredShardCount > maxShards {
		return errors.NewValidationError(fmt.Sprintf("desired_shard_count must be between 0 and %d", maxShards))
	}
	if ic.BucketsNum < 0 || ic.BucketsNum > maxShards {
		return errors.NewValidationError(fmt.Sprintf("buckets_num must be between 0 and %d", maxShards))
	}
	if ic.ReplicationNum < 0 || ic.ReplicationNum > maxReplicas {
		return errors.NewValidationError(fmt.Sprintf("replication_num must be between 0 and %d", maxReplicas))
	}

	return nil
}

// ---------------------------------------------------------------------------
// StoreDisplay — API-safe projection embedded in other resource responses
// ---------------------------------------------------------------------------

// Vector store source classifiers used by API responses.
// Kept as package-level constants so handlers and services share a single
// vocabulary instead of repeating magic strings.
const (
	StoreSourceEnv         = "env"         // env-driven (RETRIEVE_DRIVER)
	StoreSourceUser        = "user"        // DB-managed VectorStore row
	StoreSourceShared      = "shared"      // cross-tenant access — metadata suppressed
	StoreSourceUnavailable = "unavailable" // bound store row missing / registry miss
)

// StoreDisplay is the API-safe projection of a VectorStore for embedding in
// other resource responses (notably KnowledgeBase). It carries only the
// display-safe identifiers — never connection credentials.
//
// Source is one of the StoreSource* constants. EngineType is the underlying
// engine name (e.g. "elasticsearch"). Status mirrors Source by default but
// is split out to give the UI a stable boolean-like signal independent of
// future Source value additions.
type StoreDisplay struct {
	Name       string `json:"vector_store_name,omitempty"`
	Source     string `json:"vector_store_source,omitempty"`
	EngineType string `json:"vector_store_engine_type,omitempty"`
	Status     string `json:"vector_store_status,omitempty"` // "available" / "unavailable"
}

// DefaultStoreDisplay is the display payload for KBs that fall back to the
// tenant's env stores (VectorStoreID == nil).
func DefaultStoreDisplay() StoreDisplay {
	return StoreDisplay{
		Name:   "System default",
		Source: StoreSourceEnv,
		Status: "available",
	}
}

// UnavailableStoreDisplay is used when the bound store cannot be resolved
// (deleted row, registry miss, transient infra error). The UI can branch on
// Status to guide recovery (admin tool, rebind, etc.).
func UnavailableStoreDisplay() StoreDisplay {
	return StoreDisplay{
		Source: StoreSourceUnavailable,
		Status: "unavailable",
	}
}

// SharedStoreDisplay is returned for cross-tenant shared KB views so that
// the underlying owner-tenant store's name and engine remain hidden — only
// the fact that "this KB is shared" leaks, which is already implied by the
// share grant itself.
func SharedStoreDisplay() StoreDisplay {
	return StoreDisplay{
		Source: StoreSourceShared,
		Status: "available",
	}
}

// ---------------------------------------------------------------------------
// VectorStoreResponse — API response DTO
// ---------------------------------------------------------------------------

// VectorStoreResponse is the API response DTO for vector store.
// Wraps VectorStore with additional metadata (source, readonly).
type VectorStoreResponse struct {
	VectorStore
	Source   string `json:"source"`   // "env" or "user"
	ReadOnly bool   `json:"readonly"` // env stores are read-only
}

// NewVectorStoreResponse creates a response DTO from a VectorStore
// with sensitive fields masked.
func NewVectorStoreResponse(store *VectorStore, source string, readonly bool) VectorStoreResponse {
	masked := *store
	masked.ConnectionConfig = store.ConnectionConfig.MaskSensitiveFields()
	return VectorStoreResponse{
		VectorStore: masked,
		Source:      source,
		ReadOnly:    readonly,
	}
}

// ---------------------------------------------------------------------------
// VectorStore type metadata — for /types endpoint
// ---------------------------------------------------------------------------

// VectorStoreTypeInfo describes a supported engine type and its configuration schema.
type VectorStoreTypeInfo struct {
	Type             string                 `json:"type"`
	DisplayName      string                 `json:"display_name"`
	ConnectionFields []VectorStoreFieldInfo `json:"connection_fields"`
	IndexFields      []VectorStoreFieldInfo `json:"index_fields,omitempty"`
}

func resolveTencentVectorDBReplicaNumber(lookup EnvLookupFunc) int {
	if lookup == nil {
		lookup = os.Getenv
	}
	raw := strings.TrimSpace(lookup(envTencentVectorDBReplicaNumber))
	if raw == "" {
		return defaultTencentVectorDBReplicaNumber
	}
	replicas, err := strconv.Atoi(raw)
	if err != nil || replicas < 0 {
		return defaultTencentVectorDBReplicaNumber
	}
	return replicas
}

// VectorStoreFieldInfo describes a single configuration field exposed
// by /api/v1/vector-stores/types for the registration UI. The optional
// validation hints (`Immutable`, `Min`, `Max`, `Enum`) are used by both
// the frontend (to disable / constrain inputs) and the backend
// (defense-in-depth validation in the service layer).
type VectorStoreFieldInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "string", "number", "boolean"
	Required    bool   `json:"required"`
	Sensitive   bool   `json:"sensitive,omitempty"`
	Default     any    `json:"default,omitempty"`
	Description string `json:"description,omitempty"`

	// Immutable marks a field whose value cannot be changed after the
	// VectorStore is first created. The UI shows the input as read-only
	// in edit mode; the backend rejects modification attempts. Used by
	// engines whose underlying index structure is fixed at create time
	// (e.g. OpenSearch's HNSW engine / M / ef_construction).
	Immutable bool `json:"immutable,omitempty"`

	// Min / Max set inclusive bounds for "number"-typed fields. nil
	// means no bound on that side. Frontend uses these to constrain
	// inputs; backend re-validates as defense-in-depth.
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`

	// Enum constrains the allowed string values. Empty means no
	// constraint. Used by fields whose value space is closed (e.g.
	// OpenSearch's knn_engine ∈ {"lucene", "faiss"}).
	Enum []string `json:"enum,omitempty"`
}

// GetVectorStoreTypes returns metadata for the engine types the personal
// edition supports: the application's own PostgreSQL database or a local
// SQLite file. The external vector databases of the enterprise edition were
// removed, so they are neither listed nor registerable.
func GetVectorStoreTypes() []VectorStoreTypeInfo {
	return []VectorStoreTypeInfo{
		{
			Type:        string(PostgresRetrieverEngineType),
			DisplayName: "PostgreSQL",
			ConnectionFields: []VectorStoreFieldInfo{
				{Name: "use_default_connection", Type: "boolean", Required: false, Description: "Use the application's default PostgreSQL connection", Default: false},
				{Name: "addr", Type: "string", Required: false, Description: "PostgreSQL connection string (required unless use_default_connection is set)"},
				{Name: "username", Type: "string", Required: false, Description: "Username", Default: "postgres"},
				{Name: "password", Type: "string", Required: false, Sensitive: true, Description: "Password"},
			},
			IndexFields: []VectorStoreFieldInfo{
				{Name: "collection_prefix", Type: "string", Required: false, Description: "Collection Prefix", Default: "gaze_embeddings"},
				{Name: "collection_name", Type: "string", Required: false, Description: "Collection Name", Default: "gaze_embeddings"},
			},
		},
		{
			Type:             string(SQLiteRetrieverEngineType),
			DisplayName:      "SQLite",
			ConnectionFields: nil,
			IndexFields: []VectorStoreFieldInfo{
				{Name: "collection_prefix", Type: "string", Required: false, Description: "Collection Prefix", Default: "gaze_embeddings"},
				{Name: "collection_name", Type: "string", Required: false, Description: "Collection Name", Default: "gaze_embeddings"},
			},
		},
	}
}

// floatPtr returns a pointer to v, for setting VectorStoreFieldInfo Min/Max.
func floatPtr(v float64) *float64 { return &v }

// ---------------------------------------------------------------------------
// BuildEnvVectorStores — virtual stores from RETRIEVE_DRIVER env var
// ---------------------------------------------------------------------------

// BuildEnvVectorStores builds virtual VectorStore entries from RETRIEVE_DRIVER.
// Returns []VectorStore (not VectorStoreResponse) so that business logic (e.g.,
// duplicate checking) can use them directly. API responses should wrap them
// via NewVectorStoreResponse.
//
// Pure function — does not call os.Getenv directly.
//
// Usage:
//
//	types.BuildEnvVectorStores(os.Getenv("RETRIEVE_DRIVER"), os.Getenv)
func BuildEnvVectorStores(retrieveDriver string, envLookup EnvLookupFunc) []VectorStore {
	if retrieveDriver == "" {
		return nil
	}

	drivers := strings.Split(retrieveDriver, ",")
	var stores []VectorStore

	for _, driver := range drivers {
		driver = strings.TrimSpace(driver)
		if driver == "" {
			continue
		}

		store := buildEnvStoreForDriver(driver, envLookup)
		if store != nil {
			stores = append(stores, *store)
		}
	}
	return stores
}

// FindEnvVectorStore finds a specific env store by its virtual ID.
func FindEnvVectorStore(retrieveDriver string, envLookup EnvLookupFunc, id string) *VectorStore {
	for _, s := range BuildEnvVectorStores(retrieveDriver, envLookup) {
		if s.ID == id {
			return &s
		}
	}
	return nil
}

func buildEnvStoreForDriver(driver string, envLookup EnvLookupFunc) *VectorStore {
	switch driver {
	case "postgres":
		return &VectorStore{
			ID:         "__env_postgres__",
			Name:       "PostgreSQL",
			EngineType: PostgresRetrieverEngineType,
			ConnectionConfig: ConnectionConfig{
				UseDefaultConnection: true,
			},
		}
	case "sqlite":
		return &VectorStore{
			ID:         "__env_sqlite__",
			Name:       "SQLite",
			EngineType: SQLiteRetrieverEngineType,
		}
	default:
		return nil
	}
}
