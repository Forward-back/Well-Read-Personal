package router

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	filesvc "gaze/internal/application/service/file"
	"gaze/internal/filetransport"
	"gaze/internal/logger"
	"gaze/internal/middleware"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	secutils "gaze/internal/utils"
	"github.com/gin-gonic/gin"
)

// files.go hosts the file-proxy surface the personal router exposes:
//
//   - /files                           authenticated raw storage proxy
//   - /api/v1/files/presigned          HMAC-signed anonymous access (IM)
//   - /api/v1/files/presigned-preview  presigned URL diagnostics
//
// Every surface shares the same storage plumbing — the helpers below are
// that shared plumbing.

// getRouteRegistrar is the minimal registration surface serveFiles* needs;
// both *gin.Engine and *gin.RouterGroup satisfy it, which keeps the file
// routes testable without building a full engine.
type getRouteRegistrar interface {
	GET(string, ...gin.HandlerFunc) gin.IRoutes
}

// localStorageBaseDir resolves LOCAL_STORAGE_BASE_DIR with the container
// default.
func localStorageBaseDir() string {
	baseDir := strings.TrimSpace(os.Getenv("LOCAL_STORAGE_BASE_DIR"))
	if baseDir == "" {
		baseDir = "/data/files"
	}
	return baseDir
}

// localStorageAbsDir is localStorageBaseDir made absolute — the form the
// storage resolvers expect.
func localStorageAbsDir() string {
	absDir, _ := filepath.Abs(localStorageBaseDir())
	return absDir
}

// parseStorageTarget splits a stored file path into its optional storage
// backend ID and provider scheme (e.g. "backend://3/local://7/x.png" ->
// ("3", "local"); "cos://7/x.png" -> ("", "cos")).
func parseStorageTarget(filePath string) (backendID, provider string) {
	backendID, providerPath, scoped := types.ParseStorageBackendPath(filePath)
	if !scoped {
		providerPath = filePath
	}
	return backendID, types.ParseProviderScheme(providerPath)
}

// requireFilePathQuery validates the file_path query parameter every proxy
// route accepts. On failure the 400 response has been written and ok=false.
func requireFilePathQuery(c *gin.Context) (string, bool) {
	filePath := strings.TrimSpace(c.Query("file_path"))
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required parameter: file_path"})
		return "", false
	}
	if strings.Contains(filePath, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file path"})
		return "", false
	}
	return filePath, true
}

// resolveCatalogResource maps a logical resource path onto its physical path
// when the catalog knows it, enforcing that the resource belongs to
// ownerTenantID. isResource reports whether the path resolved to a registered
// resource (whose tenant is then authoritative). On !ok the response has been
// written.
func resolveCatalogResource(
	c *gin.Context,
	catalog interfaces.ResourceCatalog,
	filePath string,
	ownerTenantID uint64,
) (resolved string, isResource, ok bool) {
	if catalog == nil {
		return filePath, false, true
	}
	resolvedPath, resource, err := catalog.ResolvePath(c.Request.Context(), filePath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return "", false, false
	}
	if resource == nil {
		return filePath, false, true
	}
	if resource.TenantID != ownerTenantID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden: resource not accessible"})
		return "", false, false
	}
	return resolvedPath, true, true
}

// resolveFileService picks the file service for (tenant, backendID, provider)
// — via the storage resolver when wired, else directly from the tenant's
// storage config. No fallback; used by the presigned surfaces where a
// missing tenant config must surface as an error.
func resolveFileService(
	ctx context.Context,
	tenant *types.Tenant,
	backendID, provider, absDir string,
	storageResolver interfaces.StorageBackendResolver,
) (interfaces.FileService, string, error) {
	if storageResolver != nil {
		return storageResolver.ResolveFileService(ctx, tenant, backendID, provider, absDir)
	}
	return filesvc.NewFileServiceFromStorageConfig(provider, tenant.StorageEngineConfig, absDir)
}

// streamStoredFile writes the shared success response of every file proxy:
// safe content type, nosniff, disposition for non-inline types, the route's
// cache policy, then the body (skipped for HEAD). Closes reader.
func streamStoredFile(c *gin.Context, reader io.ReadCloser, contentType string, inline bool, cacheControl, logTag string, fileNames ...string) {
	name := ""
	if len(fileNames) > 0 {
		name = fileNames[0]
	}
	if err := filetransport.Serve(c.Writer, c.Request, reader, filetransport.Options{
		Filename: name, Download: !inline, ContentType: contentType, CacheControl: cacheControl,
	}); err != nil {
		logger.Warnf(c.Request.Context(), "[Router] %s write response failed: %v", logTag, err)
	}
}

// newFileServeHandler builds the file-proxy handler. It reads the owner tenant
// from the request context set by the auth middleware. Tenant ownership of the
// requested path is enforced via ValidateStoragePathTenant.
func newFileServeHandler(
	globalFileService interfaces.FileService,
	storageResolver interfaces.StorageBackendResolver,
	resourceCatalogs ...interfaces.ResourceCatalog,
) gin.HandlerFunc {
	resourceCatalog := firstResourceCatalog(resourceCatalogs)
	absDir := localStorageAbsDir()
	if info, err := os.Stat(absDir); err != nil || !info.IsDir() {
		if err := os.MkdirAll(absDir, 0o755); err != nil {
			logger.Warnf(context.Background(), "[Router] Cannot create local storage dir %s: %v", absDir, err)
		}
	}

	return func(c *gin.Context) {
		filePath, ok := requireFilePathQuery(c)
		if !ok {
			return
		}

		tenant, _ := c.Request.Context().Value(types.TenantInfoContextKey).(*types.Tenant)
		if tenant == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: workspace context missing"})
			return
		}
		filePath, resourceResolved, ok := resolveCatalogResource(c, resourceCatalog, filePath, tenant.ID)
		if !ok {
			return
		}

		// A registered resource's tenant is authoritative. Physical provider
		// paths remain an internal locator and are not required to encode access
		// control metadata (some cloud layouts contain other numeric segments).
		if !resourceResolved {
			if err := secutils.ValidateStoragePathTenant(filePath, tenant.ID); err != nil {
				logger.Warnf(c.Request.Context(),
					"[Router] /files denied cross-tenant or invalid path: tenant_id=%d file_path=%q err=%v",
					tenant.ID, filePath, err)
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden: file path not accessible"})
				return
			}
		}

		backendID, provider := parseStorageTarget(filePath)
		fileSvc, resolvedProvider, ok := filesvc.ResolveTenantFileServiceWithFallback(
			c.Request.Context(), "/files", tenant, backendID, provider, absDir, storageResolver, globalFileService)
		if !ok {
			c.Status(http.StatusBadRequest)
			return
		}

		reader, err := fileSvc.GetFile(c.Request.Context(), filePath)
		if err != nil {
			logger.Warnf(c.Request.Context(), "[Router] /files get file failed: tenant_id=%d provider=%s path=%q err=%v",
				tenant.ID, resolvedProvider, filePath, err)
			c.Status(http.StatusNotFound)
			return
		}

		contentType, inline := secutils.SafeContentTypeByFilename(filePath)
		streamStoredFile(c, reader, contentType, inline, "public, max-age=86400", "/files", filePath)
	}
}

func serveFiles(r getRouteRegistrar, globalFileService interfaces.FileService, resolvers ...interfaces.StorageBackendResolver) {
	var storageResolver interfaces.StorageBackendResolver
	if len(resolvers) > 0 {
		storageResolver = resolvers[0]
	}
	serveFilesWithResources(r, globalFileService, storageResolver, nil)
}

// serveFilesWithResources registers the authenticated storage proxy.
//
// Route:
//   - GET /files?file_path=<provider://...>
func serveFilesWithResources(
	r getRouteRegistrar,
	globalFileService interfaces.FileService,
	storageResolver interfaces.StorageBackendResolver,
	resourceCatalog interfaces.ResourceCatalog,
) {
	logger.Infof(context.Background(), "[Router] Serving files from /files")
	// /files sits outside the /api/v1 APIKeyGate, so it carries its own
	// API-key guard: a raw storage path cannot be bounded to a KB allow-list,
	// so keys with a narrower scope are denied here.
	r.GET(
		"/files",
		middleware.AllowFileServeAPIKey(),
		newFileServeHandler(globalFileService, storageResolver, resourceCatalog),
	)
}

func firstResourceCatalog(catalogs []interfaces.ResourceCatalog) interfaces.ResourceCatalog {
	if len(catalogs) == 0 {
		return nil
	}
	return catalogs[0]
}

// servePresignedFiles serves files via HMAC-signed URLs without requiring authentication.
// This is used by IM channels to serve images that are embedded in bot replies.
//
// Routes:
//   - GET  /api/v1/files/presigned?file_path=<provider://...>&tenant_id=<id>&expires=<unix>&sig=<hmac>
//   - HEAD /api/v1/files/presigned?...  (IM platforms issue HEAD first to validate
//     Content-Type / Content-Length before rendering image previews; HEAD must
//     succeed or the inline image renders as broken)
//
// Failure paths log client IP + User-Agent + (truncated) file_path so operators
// can correlate an IM platform's fetch against the upstream signing log line.
// Without this it is otherwise impossible to tell whether a "broken image" is
// caused by an expired signature, a stale URL cached by the platform, the
// platform's IP being blocked, or the URL simply never reaching us.
func servePresignedFiles(r *gin.Engine, tenantService interfaces.TenantService, storageResolver interfaces.StorageBackendResolver) {
	handler := presignedFileHandler(tenantService, localStorageAbsDir(), storageResolver)
	r.GET("/api/v1/files/presigned", handler)
	r.HEAD("/api/v1/files/presigned", handler)
}

// presignedFileHandler returns the shared Gin handler used by both GET and HEAD.
// For HEAD requests it returns the same status + headers but does not stream
// the body — this is enough for IM platforms to validate the URL while saving
// us a full read of the backing object.
func presignedFileHandler(tenantService interfaces.TenantService, absDir string, resolvers ...interfaces.StorageBackendResolver) gin.HandlerFunc {
	var storageResolver interfaces.StorageBackendResolver
	if len(resolvers) > 0 {
		storageResolver = resolvers[0]
	}
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()

		filePath := strings.TrimSpace(c.Query("file_path"))
		tenantIDStr := strings.TrimSpace(c.Query("tenant_id"))
		expiresStr := strings.TrimSpace(c.Query("expires"))
		sig := strings.TrimSpace(c.Query("sig"))

		if filePath == "" || tenantIDStr == "" || expiresStr == "" || sig == "" {
			logger.Warnf(ctx, "[Router] /files/presigned missing params: client_ip=%s ua=%q file_path=%q tenant_id=%q expires=%q has_sig=%v",
				clientIP, userAgent, filePath, tenantIDStr, expiresStr, sig != "")
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing required parameters"})
			return
		}
		if strings.Contains(filePath, "..") {
			logger.Warnf(ctx, "[Router] /files/presigned rejected path traversal: client_ip=%s file_path=%q", clientIP, filePath)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file path"})
			return
		}

		tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64)
		if err != nil {
			logger.Warnf(ctx, "[Router] /files/presigned invalid tenant_id: client_ip=%s tenant_id=%q err=%v", clientIP, tenantIDStr, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
			return
		}

		// Verify HMAC signature and expiry. Logged at Warn because every 403
		// here is a signal worth investigating: either the URL was tampered
		// with, the IM platform cached an expired URL, or SYSTEM_AES_KEY was
		// rotated without invalidating in-flight links.
		if !secutils.VerifyFileURLSig(filePath, tenantID, expiresStr, sig) {
			logger.Warnf(ctx, "[Router] /files/presigned sig invalid or expired: client_ip=%s ua=%q tenant_id=%d file_path=%q expires=%s",
				clientIP, userAgent, tenantID, filePath, expiresStr)
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid or expired signature"})
			return
		}

		tenant, err := tenantService.GetTenantByID(ctx, tenantID)
		if err != nil {
			logger.Warnf(ctx, "[Router] /files/presigned tenant lookup failed: client_ip=%s tenant_id=%d err=%v", clientIP, tenantID, err)
			c.Status(http.StatusNotFound)
			return
		}

		backendID, provider := parseStorageTarget(filePath)
		fileSvc, resolvedProvider, err := resolveFileService(ctx, tenant, backendID, provider, absDir, storageResolver)
		if err != nil {
			logger.Warnf(ctx, "[Router] /files/presigned resolve file service failed: client_ip=%s tenant_id=%d provider=%s err=%v",
				clientIP, tenantID, provider, err)
			c.Status(http.StatusBadRequest)
			return
		}

		// HEAD short-circuits the body read inside streamStoredFile. We still
		// need to confirm the object exists via GetFile: skipping it entirely
		// for HEAD would risk reporting 200 for a signed URL that no longer
		// points at a real object, making subsequent GETs from the same client
		// mysteriously fail.
		reader, err := fileSvc.GetFile(ctx, filePath)
		if err != nil {
			logger.Warnf(ctx, "[Router] /files/presigned get file failed: client_ip=%s tenant_id=%d provider=%s path=%q err=%v",
				clientIP, tenantID, resolvedProvider, filePath, err)
			c.Status(http.StatusNotFound)
			return
		}

		contentType, inline := secutils.SafeContentTypeByFilename(filePath)
		streamStoredFile(c, reader, contentType, inline, "public, max-age=86400", "/files/presigned")
	}
}

// servePresignedPreview registers a diagnostic endpoint that returns the
// presigned HTTP URL that *would be* generated for a given storage path by the
// owner's current storage config — exactly the URL an IM channel would embed
// in a reply. It is registered after the auth middleware, so only the owner
// can query it.
//
// Route:
//   - GET /api/v1/files/presigned-preview?file_path=<provider://...>
func servePresignedPreview(r *gin.Engine, storageResolver interfaces.StorageBackendResolver) {
	absDir := localStorageAbsDir()

	r.GET("/api/v1/files/presigned-preview", func(c *gin.Context) {
		ctx := c.Request.Context()
		filePath, ok := requireFilePathQuery(c)
		if !ok {
			return
		}

		tenant, _ := ctx.Value(types.TenantInfoContextKey).(*types.Tenant)
		if tenant == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: workspace context missing"})
			return
		}

		backendID, provider := parseStorageTarget(filePath)
		fileSvc, resolvedProvider, err := resolveFileService(ctx, tenant, backendID, provider, absDir, storageResolver)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":    err.Error(),
				"provider": provider,
				"hint":     "workspace storage config is missing or incomplete for this provider",
			})
			return
		}

		httpURL, err := fileSvc.GetFileURL(ctx, filePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":    err.Error(),
				"provider": resolvedProvider,
				"hint":     "GetFileURL failed; for local storage this usually means APP_EXTERNAL_URL is unset",
			})
			return
		}

		// Detect the "no-op" case where local storage falls back to the
		// provider:// path because APP_EXTERNAL_URL is missing. Surfacing
		// this explicitly is the whole point of the endpoint.
		rewritten := httpURL != filePath
		hint := ""
		if !rewritten {
			hint = "URL unchanged; for local storage set APP_EXTERNAL_URL to enable presigned HTTP URLs"
		}

		c.JSON(http.StatusOK, gin.H{
			"file_path": filePath,
			"provider":  resolvedProvider,
			"url":       httpURL,
			"rewritten": rewritten,
			"hint":      hint,
		})
	})
}
