package handler

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"gaze/internal/application/service"
	"gaze/internal/application/service/file"
	"gaze/internal/config"
	"gaze/internal/database"
	"gaze/internal/infrastructure/docparser"
	"gaze/internal/logger"
	"gaze/internal/runtime"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	secutils "gaze/internal/utils"
	"github.com/gin-gonic/gin"
)

// SystemHandler handles system information and infrastructure diagnostics.
type SystemHandler struct {
	cfg            *config.Config
	documentReader interfaces.DocumentReader
	// storageBackendRepo lets GetStorageEngineStatus report multi-instance
	// storage backends as "available", not just the legacy singleton
	// tenant.StorageEngineConfig. Optional: a nil repo consults the legacy
	// config only.
	storageBackendRepo interfaces.StorageBackendRepository
	sandboxConfigSvc   sandboxConfigService
	// startup snapshot for GET /system/capabilities; bound in router.NewRouter.
	deploymentCapabilities DeploymentCapabilitiesData
}

func NewSystemHandler(
	cfg *config.Config,
	documentReader interfaces.DocumentReader,
	storageBackendRepo interfaces.StorageBackendRepository,
	sandboxConfigSvc *service.TenantSandboxConfigService,
) *SystemHandler {
	return &SystemHandler{
		cfg:                cfg,
		documentReader:     documentReader,
		storageBackendRepo: storageBackendRepo,
		sandboxConfigSvc:   sandboxConfigSvc,
	}
}

type GetSystemInfoResponse struct {
	Version             string `json:"version"`
	Edition             string `json:"edition"`
	CommitID            string `json:"commit_id,omitempty"`
	BuildTime           string `json:"build_time,omitempty"`
	GoVersion           string `json:"go_version,omitempty"`
	KeywordIndexEngine  string `json:"keyword_index_engine,omitempty"`
	VectorStoreEngine   string `json:"vector_store_engine,omitempty"`
	GraphDatabaseEngine string `json:"graph_database_engine,omitempty"`
	MinioEnabled        bool   `json:"minio_enabled,omitempty"`
	DBVersion           string `json:"db_version,omitempty"`
	DBMigrationError    string `json:"db_migration_error,omitempty"`
	StartedAt           string `json:"started_at,omitempty"`
	UptimeSeconds       int64  `json:"uptime_seconds,omitempty"`
}

var (
	Version   = "unknown"
	Edition   = "standard"
	CommitID  = "unknown"
	BuildTime = "unknown"
	GoVersion = "unknown"
)

func (h *SystemHandler) GetSystemInfo(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())

	// Get keyword index engine from RETRIEVE_DRIVER
	keywordIndexEngine := h.getKeywordIndexEngine()

	// Get vector store engine from config or RETRIEVE_DRIVER
	vectorStoreEngine := h.getVectorStoreEngine()

	// Get graph database engine from NEO4J_ENABLE
	graphDatabaseEngine := h.getGraphDatabaseEngine()

	// Get MinIO enabled status
	minioEnabled := h.isMinioConfigured(c)

	dbMigrationErr := database.CachedMigrationError()
	var dbVersion string
	if ver, dirty, ok := database.CachedMigrationVersion(); ok {
		dbVersion = fmt.Sprintf("%d", ver)
		if dirty {
			dbVersion += " (dirty)"
		}
		if dbMigrationErr != "" {
			dbVersion += " (failed)"
		}
	} else if dbMigrationErr != "" {
		// Failure happened before m.Version() could be read (e.g. could not
		// open the database). Still emit a placeholder so the frontend renders
		// the row and shows the troubleshooting banner.
		dbVersion = "unknown"
	}

	var startedAt string
	var uptimeSec int64
	if boot := runtime.ServerStartedAt(); !boot.IsZero() {
		startedAt = boot.UTC().Format(time.RFC3339)
		uptimeSec = int64(runtime.ServerUptime().Seconds())
	}

	response := GetSystemInfoResponse{
		Version:             Version,
		Edition:             Edition,
		CommitID:            CommitID,
		BuildTime:           BuildTime,
		GoVersion:           GoVersion,
		KeywordIndexEngine:  keywordIndexEngine,
		VectorStoreEngine:   vectorStoreEngine,
		GraphDatabaseEngine: graphDatabaseEngine,
		MinioEnabled:        minioEnabled,
		DBVersion:           dbVersion,
		DBMigrationError:    dbMigrationErr,
		StartedAt:           startedAt,
		UptimeSeconds:       uptimeSec,
	}

	logger.Info(ctx, "System info retrieved successfully")
	c.JSON(200, gin.H{
		"code": 0,
		"msg":  "success",
		"data": response,
	})
}

func (h *SystemHandler) getDocReaderConnInfo() (addr, transport string) {
	addr = strings.TrimSpace(os.Getenv("DOCREADER_ADDR"))
	transport = strings.TrimSpace(os.Getenv("DOCREADER_TRANSPORT"))
	if transport == "" {
		transport = "grpc"
	}
	transport = strings.ToLower(transport)
	return addr, transport
}

// ListParserEngines returns available document parser engines.
// Merges Go-native static engines with engines discovered from the remote
// docreader service, so newly added Python engines are auto-discovered.
// @Summary      列出可用的文档解析引擎
// @Tags         系统
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "解析引擎列表"
// @Router       /system/parser-engines [get]
func (h *SystemHandler) ListParserEngines(c *gin.Context) {
	var overrides map[string]string
	if v, exists := c.Get(types.TenantInfoContextKey.String()); exists {
		if tenant, ok := v.(*types.Tenant); ok && tenant != nil {
			if tenant.ParserEngineConfig != nil {
				overrides = tenant.ParserEngineConfig.ToOverridesMap()
			}
		}
	}

	reader, docreaderAddr, docreaderTransport := h.resolveDocReader(c.Request.Context(), overrides)
	connected := reader != nil && reader.IsConnected()
	remoteEngines := h.fetchRemoteEngines(c.Request.Context(), reader, overrides)
	engines := docparser.ListAllEngines(connected, overrides, remoteEngines)
	c.JSON(200, gin.H{"code": 0, "msg": "success", "data": engines, "docreader_addr": docreaderAddr, "docreader_transport": docreaderTransport, "connected": connected})
}

// ReconnectDocReader reconnects the document converter to a new (or same) DocReader address.
// @Summary      重连文档解析服务
// @Tags         系统
// @Accept       json
// @Produce      json
// @Param        request  body  object{addr string} true "DocReader 地址"
// @Success      200
// @Router       /system/docreader/reconnect [post]
func (h *SystemHandler) ReconnectDocReader(c *gin.Context) {
	var req struct {
		Addr string `json:"addr" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 1, "msg": "请提供 addr 参数"})
		return
	}
	addr := strings.TrimSpace(req.Addr)
	if addr == "" {
		c.JSON(400, gin.H{"code": 1, "msg": "addr 不能为空"})
		return
	}

	// SSRF validation for docreader address
	if err := secutils.ValidateURLForSSRF(addr); err != nil {
		logger.Warnf(c.Request.Context(), "SSRF validation failed for docreader addr: %v", err)
		c.JSON(400, gin.H{"code": 1, "msg": secutils.FormatSSRFError("DocReader 地址", addr, err)})
		return
	}

	if h.documentReader == nil {
		c.JSON(500, gin.H{"code": 1, "msg": "document converter not initialized"})
		return
	}

	if err := h.documentReader.Reconnect(addr); err != nil {
		logger.Errorf(c.Request.Context(), "Failed to reconnect docreader to %s: %v", addr, err)
		c.JSON(200, gin.H{"code": 1, "msg": fmt.Sprintf("连接失败: %v", err)})
		return
	}

	var overrides map[string]string
	if v, exists := c.Get(types.TenantInfoContextKey.String()); exists {
		if tenant, ok := v.(*types.Tenant); ok && tenant != nil {
			if tenant.ParserEngineConfig != nil {
				overrides = tenant.ParserEngineConfig.ToOverridesMap()
			}
		}
	}
	remoteEngines := h.fetchRemoteEngines(c.Request.Context(), h.documentReader, overrides)
	engines := docparser.ListAllEngines(true, overrides, remoteEngines)

	_, docreaderTransport := h.getDocReaderConnInfo()
	c.JSON(200, gin.H{"code": 0, "msg": "连接成功", "data": engines, "docreader_addr": addr, "docreader_transport": docreaderTransport, "connected": true})
}

func (h *SystemHandler) CheckParserEngines(c *gin.Context) {
	var body types.ParserEngineConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"code": 1, "msg": "请求体格式错误"})
		return
	}
	var existing *types.ParserEngineConfig
	if v, exists := c.Get(types.TenantInfoContextKey.String()); exists {
		if t, ok := v.(*types.Tenant); ok && t != nil {
			existing = t.ParserEngineConfig
		}
	}
	merged := types.MergeParserEngineConfigForUpdate(&body, existing)
	overrides := merged.ToOverridesMap()
	reader, docreaderAddr, docreaderTransport := h.resolveDocReader(c.Request.Context(), overrides)
	connected := reader != nil && reader.IsConnected()
	remoteEngines := h.fetchRemoteEngines(c.Request.Context(), reader, overrides)
	engines := docparser.ListAllEngines(connected, overrides, remoteEngines)
	c.JSON(200, gin.H{"code": 0, "msg": "success", "data": engines, "docreader_addr": docreaderAddr, "docreader_transport": docreaderTransport, "connected": connected})
}

func (h *SystemHandler) resolveDocReader(ctx context.Context, overrides map[string]string) (interfaces.DocumentReader, string, string) {
	addr, transport := h.getDocReaderConnInfo()
	return h.documentReader, addr, transport
}

func transportFromDocReaderAddr(addr string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(addr)), "https://") {
		return "https"
	}
	return "http"
}

// fetchRemoteEngines queries the remote docreader for its engine list.
// Returns nil on any error (e.g. not connected), letting the caller
// fall back to Go's static registry only.
func (h *SystemHandler) fetchRemoteEngines(ctx context.Context, reader interfaces.DocumentReader, overrides map[string]string) []types.ParserEngineInfo {
	if reader == nil || !reader.IsConnected() {
		return nil
	}
	engines, err := reader.ListEngines(ctx, overrides)
	if err != nil {
		logger.Warnf(ctx, "Failed to fetch remote engines from docreader: %v", err)
		return nil
	}
	return engines
}

// getKeywordIndexEngine returns the keyword index engine name
func (h *SystemHandler) getKeywordIndexEngine() string {
	retrieveDriver := os.Getenv("RETRIEVE_DRIVER")
	if retrieveDriver == "" {
		return "未配置"
	}

	drivers := strings.Split(retrieveDriver, ",")
	// Filter out engines that support keyword retrieval
	keywordEngines := []string{}
	for _, driver := range drivers {
		driver = strings.TrimSpace(driver)
		if h.supportsRetrieverType(driver, types.KeywordsRetrieverType) {
			keywordEngines = append(keywordEngines, driver)
		}
	}

	if len(keywordEngines) == 0 {
		return "未配置"
	}
	return strings.Join(keywordEngines, ", ")
}

// getVectorStoreEngine returns the vector store engine name
func (h *SystemHandler) getVectorStoreEngine() string {
	// First check config.yaml
	if h.cfg != nil && h.cfg.VectorDatabase != nil && h.cfg.VectorDatabase.Driver != "" {
		return h.cfg.VectorDatabase.Driver
	}

	// Fallback to RETRIEVE_DRIVER for vector support
	retrieveDriver := os.Getenv("RETRIEVE_DRIVER")
	if retrieveDriver == "" {
		return "未配置"
	}

	drivers := strings.Split(retrieveDriver, ",")
	// Filter out engines that support vector retrieval
	vectorEngines := []string{}
	for _, driver := range drivers {
		driver = strings.TrimSpace(driver)
		if h.supportsRetrieverType(driver, types.VectorRetrieverType) {
			vectorEngines = append(vectorEngines, driver)
		}
	}

	if len(vectorEngines) == 0 {
		return "未配置"
	}
	return strings.Join(vectorEngines, ", ")
}

// getGraphDatabaseEngine returns the graph database engine name
func (h *SystemHandler) getGraphDatabaseEngine() string {
	return "Not Enabled"
}

// supportsRetrieverType checks if a driver supports a specific retriever type
// by looking up the retrieverEngineMapping from types package
func (h *SystemHandler) supportsRetrieverType(driver string, retrieverType types.RetrieverType) bool {
	// Get the mapping of all supported drivers and their capabilities
	mapping := types.GetRetrieverEngineMapping()

	// Check if the driver exists in the mapping
	engines, exists := mapping[driver]
	if !exists {
		return false
	}

	// Check if any of the engine configurations support the requested retriever type
	for _, engine := range engines {
		if engine.RetrieverType == retrieverType {
			return true
		}
	}
	return false
}

// getMinioConfig resolves MinIO connection parameters from tenant config (if mode=remote) or env vars (mode=docker/default).
func (h *SystemHandler) getMinioConfig(c *gin.Context) (endpoint, accessKeyID, secretAccessKey string) {
	if v, exists := c.Get(types.TenantInfoContextKey.String()); exists {
		if tenant, ok := v.(*types.Tenant); ok && tenant != nil && tenant.StorageEngineConfig != nil && tenant.StorageEngineConfig.MinIO != nil {
			m := tenant.StorageEngineConfig.MinIO
			if m.Mode == "remote" {
				return m.Endpoint, m.AccessKeyID, m.SecretAccessKey
			}
		}
	}
	endpoint = os.Getenv("MINIO_ENDPOINT")
	accessKeyID = os.Getenv("MINIO_ACCESS_KEY_ID")
	secretAccessKey = os.Getenv("MINIO_SECRET_ACCESS_KEY")
	return
}

// activeBackendProviders returns the set of storage providers that have at
// least one active multi-instance backend registered for the caller's
// workspace. Used to keep GetStorageEngineStatus in sync with the new Storage
// settings UI, which writes to storage_backends rather than the legacy
// tenant.StorageEngineConfig singleton. Best-effort and nil-safe: a missing
// repo, missing tenant, or query error yields an empty set so callers fall
// back to the legacy config checks.
func (h *SystemHandler) activeBackendProviders(c *gin.Context) map[string]bool {
	result := map[string]bool{}
	if h.storageBackendRepo == nil {
		return result
	}
	v, exists := c.Get(types.TenantInfoContextKey.String())
	if !exists {
		return result
	}
	tenant, ok := v.(*types.Tenant)
	if !ok || tenant == nil {
		return result
	}
	backends, err := h.storageBackendRepo.List(c.Request.Context(), tenant.ID)
	if err != nil {
		logger.Warnf(c.Request.Context(), "[storage] list backends for status failed: tenant=%d err=%v", tenant.ID, err)
		return result
	}
	for _, backend := range backends {
		if backend == nil || backend.Status != types.StorageBackendStatusActive {
			continue
		}
		result[strings.ToLower(strings.TrimSpace(backend.Provider))] = true
	}
	return result
}

// isMinioConfigured checks whether MinIO connection info is available (from tenant config or env).
func (h *SystemHandler) isMinioConfigured(c *gin.Context) bool {
	endpoint, accessKeyID, secretAccessKey := h.getMinioConfig(c)
	return endpoint != "" && accessKeyID != "" && secretAccessKey != ""
}

// isMinioEnvAvailable checks whether MinIO env vars (MINIO_ENDPOINT etc.) are set.
func (h *SystemHandler) isMinioEnvAvailable() bool {
	return os.Getenv("MINIO_ENDPOINT") != "" &&
		os.Getenv("MINIO_ACCESS_KEY_ID") != "" &&
		os.Getenv("MINIO_SECRET_ACCESS_KEY") != ""
}

// isCOSConfigured checks whether COS connection info is available from tenant config.
func (h *SystemHandler) isCOSConfigured(c *gin.Context) bool {
	if v, exists := c.Get(types.TenantInfoContextKey.String()); exists {
		if tenant, ok := v.(*types.Tenant); ok && tenant != nil && tenant.StorageEngineConfig != nil && tenant.StorageEngineConfig.COS != nil {
			cosConf := tenant.StorageEngineConfig.COS
			return cosConf.SecretID != "" && cosConf.SecretKey != "" && cosConf.Region != "" && cosConf.BucketName != ""
		}
	}
	return false
}

// isTOSConfigured checks whether TOS connection info is available from tenant config or env.
func (h *SystemHandler) isTOSConfigured(c *gin.Context) bool {
	if v, exists := c.Get(types.TenantInfoContextKey.String()); exists {
		if tenant, ok := v.(*types.Tenant); ok && tenant != nil && tenant.StorageEngineConfig != nil && tenant.StorageEngineConfig.TOS != nil {
			tosConf := tenant.StorageEngineConfig.TOS
			return tosConf.Endpoint != "" && tosConf.Region != "" && tosConf.AccessKey != "" && tosConf.SecretKey != "" && tosConf.BucketName != ""
		}
	}
	return h.isTOSEnvAvailable()
}

// isOSSConfigured checks whether OSS connection info is available from tenant config.
func (h *SystemHandler) isOSSConfigured(c *gin.Context) bool {
	if v, exists := c.Get(types.TenantInfoContextKey.String()); exists {
		if tenant, ok := v.(*types.Tenant); ok && tenant != nil && tenant.StorageEngineConfig != nil && tenant.StorageEngineConfig.OSS != nil {
			ossConf := tenant.StorageEngineConfig.OSS
			return ossConf.Endpoint != "" && ossConf.Region != "" && ossConf.AccessKey != "" && ossConf.SecretKey != "" && ossConf.BucketName != ""
		}
	}
	return false
}

// isKS3Configured checks whether KS3 connection info is available from tenant config.
func (h *SystemHandler) isKS3Configured(c *gin.Context) bool {
	if v, exists := c.Get(types.TenantInfoContextKey.String()); exists {
		if tenant, ok := v.(*types.Tenant); ok && tenant != nil && tenant.StorageEngineConfig != nil && tenant.StorageEngineConfig.KS3 != nil {
			ks3Conf := tenant.StorageEngineConfig.KS3
			return ks3Conf.Endpoint != "" && ks3Conf.Region != "" && ks3Conf.AccessKey != "" && ks3Conf.SecretKey != "" && ks3Conf.BucketName != ""
		}
	}
	return false
}

// isOBSConfigured checks whether OBS connection info is available from tenant config or env.
func (h *SystemHandler) isOBSConfigured(c *gin.Context) bool {
	if v, exists := c.Get(types.TenantInfoContextKey.String()); exists {
		if tenant, ok := v.(*types.Tenant); ok && tenant != nil && tenant.StorageEngineConfig != nil && tenant.StorageEngineConfig.OBS != nil {
			obsConf := tenant.StorageEngineConfig.OBS
			return obsConf.Endpoint != "" && obsConf.Region != "" && obsConf.AccessKey != "" && obsConf.SecretKey != "" && obsConf.BucketName != ""
		}
	}
	return h.isOBSEnvAvailable()
}

// isTOSEnvAvailable checks whether TOS env vars are set.
func (h *SystemHandler) isTOSEnvAvailable() bool {
	return os.Getenv("TOS_ENDPOINT") != "" &&
		os.Getenv("TOS_REGION") != "" &&
		os.Getenv("TOS_ACCESS_KEY") != "" &&
		os.Getenv("TOS_SECRET_KEY") != "" &&
		os.Getenv("TOS_BUCKET_NAME") != ""
}

// isOBSEnvAvailable checks whether OBS env vars are set.
func (h *SystemHandler) isOBSEnvAvailable() bool {
	return os.Getenv("OBS_ENDPOINT") != "" &&
		os.Getenv("OBS_REGION") != "" &&
		os.Getenv("OBS_ACCESS_KEY") != "" &&
		os.Getenv("OBS_SECRET_KEY") != "" &&
		os.Getenv("OBS_BUCKET_NAME") != ""
}

// StorageEngineStatusItem describes one storage engine's availability and description.
type StorageEngineStatusItem struct {
	Name        string `json:"name"` // "local", "minio", "cos", "tos", "s3", "oss", "ks3", "obs"
	Allowed     bool   `json:"allowed"`
	Available   bool   `json:"available"`   // whether the engine can be used
	Description string `json:"description"` // short description for UI
}

// GetStorageEngineStatusResponse is the response for GET /system/storage-engine-status.
type GetStorageEngineStatusResponse struct {
	Engines           []StorageEngineStatusItem `json:"engines"`
	AllowedProviders  []string                  `json:"allowed_providers"`
	MinioEnvAvailable bool                      `json:"minio_env_available"`
}

// GetStorageEngineStatus godoc
// @Summary      获取存储引擎状态
// @Description  返回 Local、MinIO、COS 各存储引擎的可用状态及说明，供全局设置与知识库选择使用
// @Tags         系统
// @Produce      json
// @Success      200  {object}  GetStorageEngineStatusResponse
// @Router       /system/storage-engine-status [get]
func (h *SystemHandler) GetStorageEngineStatus(c *gin.Context) {
	// Providers that already have an active multi-instance backend registered
	// (Settings → Storage). These are authoritative and independent of the
	// legacy singleton tenant.StorageEngineConfig, so a workspace that only
	// configured storage through the new UI is still reported as available.
	activeBackend := h.activeBackendProviders(c)
	minioConfigured := h.isMinioConfigured(c) || activeBackend["minio"]
	minioEnvAvailable := h.isMinioEnvAvailable()
	cosConfigured := h.isCOSConfigured(c) || activeBackend["cos"]
	tosConfigured := h.isTOSConfigured(c) || activeBackend["tos"]
	s3Configured := h.isS3Configured(c) || activeBackend["s3"]
	ossConfigured := h.isOSSConfigured(c) || activeBackend["oss"]
	ks3Configured := h.isKS3Configured(c) || activeBackend["ks3"]
	obsConfigured := h.isOBSConfigured(c) || activeBackend["obs"]
	allowed := getAllowedStorageProviders()
	allowedProviders := make([]string, 0, len(getSupportedStorageProviders()))
	for _, provider := range getSupportedStorageProviders() {
		if allowed[provider] {
			allowedProviders = append(allowedProviders, provider)
		}
	}
	engines := []StorageEngineStatusItem{
		{Name: "local", Allowed: allowed["local"], Available: true, Description: "本地文件系统存储，仅适合单机部署"},
		{Name: "minio", Allowed: allowed["minio"], Available: minioConfigured || minioEnvAvailable, Description: "S3 兼容的自托管对象存储，适合内网和私有云部署"},
		{Name: "cos", Allowed: allowed["cos"], Available: cosConfigured, Description: "腾讯云对象存储服务，适合公有云部署，支持 CDN 加速"},
		{Name: "tos", Allowed: allowed["tos"], Available: tosConfigured, Description: "火山引擎对象存储服务，适合公有云部署"},
		{Name: "s3", Allowed: allowed["s3"], Available: s3Configured, Description: "AWS S3 与兼容对象存储服务，适合公有云与混合云部署"},
		{Name: "oss", Allowed: allowed["oss"], Available: ossConfigured, Description: "阿里云对象存储服务，适合公有云部署，支持 S3 兼容协议"},
		{Name: "ks3", Allowed: allowed["ks3"], Available: ks3Configured, Description: "金山云对象存储服务，适合公有云部署"},
		{Name: "obs", Allowed: allowed["obs"], Available: obsConfigured, Description: "华为云对象存储服务，适合公有云部署"},
	}
	c.JSON(200, gin.H{
		"code": 0,
		"msg":  "success",
		"data": GetStorageEngineStatusResponse{Engines: engines, AllowedProviders: allowedProviders, MinioEnvAvailable: minioEnvAvailable},
	})
}

// --- Storage engine helpers ---
// cosFieldPattern validates COS region and bucket name format to prevent URL injection.
var cosFieldPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,62}$`)

// ossFieldPattern validates OSS region and bucket name format to prevent URL injection.
var ossFieldPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,62}$`)

// sanitizeStorageCheckError converts a raw storage connectivity error into a safe
// user-facing message that does not leak internal network details (hostnames, IPs, ports).
// The concrete mapping lives in internal/utils so the service layer can share it.
func sanitizeStorageCheckError(err error) string {
	return secutils.SanitizeStorageConnectivityError(err)
}

// storageEndpointHost extracts the hostname from a storage endpoint string.
// Endpoints may be bare host:port, hostnames, or full URLs with a scheme
// (e.g. "http://127.0.0.1:9000" for S3-compatible stores).
func storageEndpointHost(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	if strings.Contains(endpoint, "://") {
		if u, err := url.Parse(endpoint); err == nil {
			if h := u.Hostname(); h != "" {
				return h
			}
		}
	}
	if host, _, err := net.SplitHostPort(endpoint); err == nil {
		return host
	}
	return endpoint
}

// isBlockedStorageEndpoint keeps the legacy handler contract while delegating
// to the same fail-closed SSRF policy used by the storage clients themselves.
// Private storage endpoints must be explicitly whitelisted by an operator.
func isBlockedStorageEndpoint(endpoint string) (bool, string) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return true, "无效的地址"
	}
	if err := secutils.ValidateURLForSSRF(endpoint); err != nil {
		return true, secutils.FormatSSRFError("存储 Endpoint", endpoint, err)
	}
	return false, ""
}

// --- Storage engine connectivity check ---

// StorageCheckRequest is the body for POST /system/storage-engine-check.
type StorageCheckRequest struct {
	Provider string                   `json:"provider"` // "minio", "cos", "tos", "s3", "oss", "ks3", "obs"
	MinIO    *types.MinIOEngineConfig `json:"minio,omitempty"`
	COS      *types.COSEngineConfig   `json:"cos,omitempty"`
	TOS      *types.TOSEngineConfig   `json:"tos,omitempty"`
	S3       *types.S3EngineConfig    `json:"s3,omitempty"`
	OSS      *types.OSSEngineConfig   `json:"oss,omitempty"`
	KS3      *types.KS3EngineConfig   `json:"ks3,omitempty"`
	OBS      *types.OBSEngineConfig   `json:"obs,omitempty"`
}

// StorageCheckResponse is the response for a single-engine connectivity check.
type StorageCheckResponse struct {
	OK            bool   `json:"ok"`
	Message       string `json:"message"`
	BucketCreated bool   `json:"bucket_created,omitempty"`
}

// CheckStorageEngine tests connectivity for a single storage engine using the provided config.
// @Summary      测试存储引擎连通性
// @Description  使用当前填写的参数测试 MinIO/COS 连通性，不保存配置
// @Tags         系统
// @Accept       json
// @Produce      json
// @Param        body  body  StorageCheckRequest  true  "存储引擎配置"
// @Success      200   {object}  StorageCheckResponse
// @Router       /system/storage-engine-check [post]
func (h *SystemHandler) CheckStorageEngine(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())

	var req StorageCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 1, "msg": "请求体格式错误"})
		return
	}
	if !isStorageProviderAllowed(req.Provider) {
		c.JSON(403, gin.H{"code": 1, "msg": "该存储引擎已被禁用"})
		return
	}

	switch req.Provider {
	case "minio":
		h.checkMinio(c, ctx, req.MinIO)
	default:
		c.JSON(200, gin.H{"code": 0, "data": StorageCheckResponse{OK: true, Message: "本地存储无需检测"}})
	}
}

func (h *SystemHandler) isS3Configured(c *gin.Context) bool {
	if v, exists := c.Get(types.TenantInfoContextKey.String()); exists {
		if tenant, ok := v.(*types.Tenant); ok && tenant != nil && tenant.StorageEngineConfig != nil && tenant.StorageEngineConfig.S3 != nil {
			s3Conf := tenant.StorageEngineConfig.S3
			return s3Conf.Region != "" && s3Conf.BucketName != "" && (s3Conf.AccessKey == "") == (s3Conf.SecretKey == "")
		}
	}
	return false
}

func (h *SystemHandler) checkMinio(c *gin.Context, ctx context.Context, cfg *types.MinIOEngineConfig) {
	if cfg == nil {
		c.JSON(200, gin.H{"code": 0, "data": StorageCheckResponse{OK: false, Message: "未提供 MinIO 配置"}})
		return
	}

	if cfg.BucketName != "" && !cosFieldPattern.MatchString(cfg.BucketName) {
		c.JSON(200, gin.H{"code": 0, "data": StorageCheckResponse{OK: false, Message: "Bucket 名称格式不正确，仅允许字母、数字、点、连字符"}})
		return
	}

	endpoint, accessKeyID, secretAccessKey := cfg.Endpoint, cfg.AccessKeyID, cfg.SecretAccessKey
	if cfg.Mode != "remote" {
		endpoint = os.Getenv("MINIO_ENDPOINT")
		accessKeyID = os.Getenv("MINIO_ACCESS_KEY_ID")
		secretAccessKey = os.Getenv("MINIO_SECRET_ACCESS_KEY")
	}
	if endpoint == "" || accessKeyID == "" || secretAccessKey == "" {
		c.JSON(200, gin.H{"code": 0, "data": StorageCheckResponse{OK: false, Message: "Endpoint、Access Key、Secret Key 不能为空"}})
		return
	}

	if cfg.Mode == "remote" {
		if blocked, reason := isBlockedStorageEndpoint(endpoint); blocked {
			logger.Warnf(ctx, "Storage check: MinIO endpoint blocked by SSRF protection, endpoint: %s", endpoint)
			c.JSON(200, gin.H{"code": 0, "data": StorageCheckResponse{OK: false, Message: reason}})
			return
		}
	}

	err := file.CheckMinioConnectivity(ctx, endpoint, accessKeyID, secretAccessKey, cfg.BucketName, cfg.UseSSL)
	if err != nil {
		errMsg := err.Error()
		// If bucket does not exist, auto-create it
		if strings.Contains(errMsg, "does not exist") && cfg.BucketName != "" {
			logger.Info(ctx, "Storage check: bucket does not exist, attempting auto-creation", "bucket", cfg.BucketName)
			if _, mkErr := file.NewMinioFileService(endpoint, accessKeyID, secretAccessKey, cfg.BucketName, cfg.UseSSL); mkErr != nil {
				logger.Error(ctx, "Storage check: failed to create bucket", "bucket", cfg.BucketName, "error", mkErr)
				c.JSON(200, gin.H{"code": 0, "data": StorageCheckResponse{OK: false, Message: fmt.Sprintf("Failed to auto-create Bucket '%s': %s", cfg.BucketName, sanitizeStorageCheckError(mkErr))}})
				return
			}
			logger.Info(ctx, "Storage check: bucket created", "bucket", cfg.BucketName)
			c.JSON(200, gin.H{"code": 0, "data": StorageCheckResponse{OK: true, BucketCreated: true, Message: fmt.Sprintf("Bucket '%s' does not exist, and has been automatically created", cfg.BucketName)}})
			return
		}
		logger.Error(ctx, "Storage check: MinIO connectivity failed", "error", err)
		c.JSON(200, gin.H{"code": 0, "data": StorageCheckResponse{OK: false, Message: sanitizeStorageCheckError(err)}})
		return
	}

	msg := "连接成功"
	if cfg.BucketName != "" {
		msg = fmt.Sprintf("连接成功，Bucket「%s」已确认存在", cfg.BucketName)
	}
	c.JSON(200, gin.H{"code": 0, "data": StorageCheckResponse{OK: true, Message: msg}})
}

func (h *SystemHandler) ResolveDocumentReader(ctx context.Context, addr string) interfaces.DocumentReader {
	if addr == "" {
		return h.documentReader
	}

	reader, err := docparser.NewHTTPDocumentReader(addr)
	if err != nil || reader == nil {
		return reader
	}
	return reader
}
