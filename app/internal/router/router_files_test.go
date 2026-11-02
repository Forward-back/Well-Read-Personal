package router

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

var _ interfaces.FileService = (*stubFileService)(nil)

type stubFileService struct {
	getFile func(ctx context.Context, filePath string) (io.ReadCloser, error)
}

type stubResourceCatalog struct {
	resource     *types.StoredResource
	fileBindings *types.MessageFileBindings
	bound        func(context.Context, uint64, string, string) (bool, error)
}

func (s *stubResourceCatalog) IsReferencedByKnowledgeBase(
	ctx context.Context,
	tenant uint64,
	kb, ref string,
) (bool, error) {
	if s.bound == nil {
		return false, nil
	}
	return s.bound(ctx, tenant, kb, ref)
}

func (s *stubResourceCatalog) Register(
	context.Context,
	uint64,
	string,
	interfaces.ResourceRegistration,
) (string, error) {
	panic("unexpected Register")
}

func (s *stubResourceCatalog) Resolve(context.Context, string) (*types.StoredResource, error) {
	return s.resource, nil
}

func (s *stubResourceCatalog) ResolvePath(_ context.Context, value string) (string, *types.StoredResource, error) {
	if _, ok := types.ParseResourcePath(value); ok && s.resource != nil {
		return s.resource.PhysicalPath, s.resource, nil
	}
	return value, nil, nil
}

func (s *stubResourceCatalog) Bind(context.Context, string, string, string, string) error {
	panic("unexpected Bind")
}

func (s *stubResourceCatalog) Release(context.Context, string, string, string) (int64, error) {
	panic("unexpected Release")
}

func (s *stubResourceCatalog) MarkDeleted(context.Context, string) error {
	panic("unexpected MarkDeleted")
}

func (s *stubResourceCatalog) CreateAccessGrant(context.Context, string, time.Duration) (string, error) {
	panic("unexpected CreateAccessGrant")
}

func (s *stubResourceCatalog) ResolveAccessGrant(context.Context, string) (*types.StoredResource, error) {
	return s.resource, nil
}

func (s *stubFileService) CheckConnectivity(ctx context.Context) error {
	return nil
}

func (s *stubFileService) SaveFile(ctx context.Context, file *multipart.FileHeader, tenantID uint64, knowledgeID string) (string, error) {
	panic("unexpected call to SaveFile")
}

func (s *stubFileService) SaveBytes(ctx context.Context, data []byte, tenantID uint64, fileName string, temp bool) (string, error) {
	panic("unexpected call to SaveBytes")
}

func (s *stubFileService) GetFile(ctx context.Context, filePath string) (io.ReadCloser, error) {
	if s.getFile == nil {
		panic("unexpected call to GetFile")
	}
	return s.getFile(ctx, filePath)
}

func (s *stubFileService) GetFileURL(ctx context.Context, filePath string) (string, error) {
	panic("unexpected call to GetFileURL")
}

func (s *stubFileService) DeleteFile(ctx context.Context, filePath string) error {
	panic("unexpected call to DeleteFile")
}

func (s *stubFileService) CopyFile(ctx context.Context, srcPath string, tenantID uint64, knowledgeID string) (string, error) {
	panic("unexpected call to CopyFile")
}

func TestServeFilesFallsBackToGlobalFileService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("STORAGE_TYPE", "local")

	engine := gin.New()
	var requestedPath string
	serveFiles(engine, &stubFileService{
		getFile: func(ctx context.Context, filePath string) (io.ReadCloser, error) {
			requestedPath = filePath
			return io.NopCloser(strings.NewReader("fallback-body")), nil
		},
	})

	filePath := "local://42/docs/example.txt"
	req := httptest.NewRequest(http.MethodGet, "/files?file_path="+url.QueryEscape(filePath), nil)
	req = req.WithContext(context.WithValue(req.Context(), types.TenantInfoContextKey, &types.Tenant{ID: 42}))

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if requestedPath != filePath {
		t.Fatalf("requested path = %q, want %q", requestedPath, filePath)
	}
	if body := recorder.Body.String(); body != "fallback-body" {
		t.Fatalf("body = %q, want %q", body, "fallback-body")
	}
}

func TestServeFilesResolvesShortResourceReference(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("STORAGE_TYPE", "local")
	const ref = "resource://AbCdEfGhIjKlMnOpQrStUv"
	const physical = "local://42/exports/a.png"

	engine := gin.New()
	var requestedPath string
	serveFilesWithResources(engine, &stubFileService{getFile: func(_ context.Context, path string) (io.ReadCloser, error) {
		requestedPath = path
		return io.NopCloser(strings.NewReader("image")), nil
	}}, nil, &stubResourceCatalog{resource: &types.StoredResource{TenantID: 42, PhysicalPath: physical}})

	req := httptest.NewRequest(http.MethodGet, "/files?file_path="+url.QueryEscape(ref), nil)
	req = req.WithContext(context.WithValue(req.Context(), types.TenantInfoContextKey, &types.Tenant{ID: 42}))
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if requestedPath != physical {
		t.Fatalf("requested path = %q, want %q", requestedPath, physical)
	}
}

func TestServeFilesRejectsCrossTenantResourceReference(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const ref = "resource://AbCdEfGhIjKlMnOpQrStUv"
	engine := gin.New()
	serveFilesWithResources(engine, &stubFileService{getFile: func(context.Context, string) (io.ReadCloser, error) {
		t.Fatal("GetFile should not be called")
		return nil, nil
	}}, nil, &stubResourceCatalog{resource: &types.StoredResource{TenantID: 7, PhysicalPath: "local://7/exports/a.png"}})

	req := httptest.NewRequest(http.MethodGet, "/files?file_path="+url.QueryEscape(ref), nil)
	req = req.WithContext(context.WithValue(req.Context(), types.TenantInfoContextKey, &types.Tenant{ID: 42}))
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestServeFilesDoesNotFallbackWhenProviderDoesNotMatchGlobalStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("STORAGE_TYPE", "minio")

	engine := gin.New()
	serveFiles(engine, &stubFileService{
		getFile: func(ctx context.Context, filePath string) (io.ReadCloser, error) {
			t.Fatalf("GetFile should not be called for mismatched provider, got %q", filePath)
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/files?file_path="+url.QueryEscape("local://42/docs/example.txt"), nil)
	req = req.WithContext(context.WithValue(req.Context(), types.TenantInfoContextKey, &types.Tenant{ID: 42}))

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusBadRequest; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestServeFilesRejectsCrossTenantPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("STORAGE_TYPE", "local")

	engine := gin.New()
	serveFiles(engine, &stubFileService{
		getFile: func(ctx context.Context, filePath string) (io.ReadCloser, error) {
			t.Fatalf("GetFile should not be called for cross-tenant path, got %q", filePath)
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/files?file_path="+url.QueryEscape("local://7/knowledge/secret.pdf"), nil)
	req = req.WithContext(context.WithValue(req.Context(), types.TenantInfoContextKey, &types.Tenant{ID: 42}))

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusForbidden; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestServeFilesRejectsPathWithoutTenantSegment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("STORAGE_TYPE", "local")

	engine := gin.New()
	serveFiles(engine, &stubFileService{
		getFile: func(ctx context.Context, filePath string) (io.ReadCloser, error) {
			t.Fatalf("GetFile should not be called without tenant segment, got %q", filePath)
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/files?file_path="+url.QueryEscape("local://docs/example.txt"), nil)
	req = req.WithContext(context.WithValue(req.Context(), types.TenantInfoContextKey, &types.Tenant{ID: 42}))

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusForbidden; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

// /files carries its own API-key guard (middleware.AllowFileServeAPIKey):
// full-access and tenant-wide retrieve keys may serve tenant-bounded paths,
// but KB-restricted keys (and keys lacking retrieve) are denied because a raw
// storage path cannot be bounded to a KB allow-list.
func TestServeFilesAPIKeyScopeMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("STORAGE_TYPE", "local")

	const filePath = "local://42/docs/example.txt"

	cases := []struct {
		name     string
		scope    types.TenantAPIKeyScope
		wantCode int
	}{
		{
			name:     "full access allowed",
			scope:    types.TenantAPIKeyScope{FullAccess: true},
			wantCode: http.StatusOK,
		},
		{
			name: "tenant-wide retrieve allowed",
			scope: types.TenantAPIKeyScope{
				Capabilities: types.StringArray{string(types.APIKeyCapabilityRetrieve)},
			},
			wantCode: http.StatusOK,
		},
		{
			name: "kb-restricted retrieve denied",
			scope: types.TenantAPIKeyScope{
				KnowledgeBaseIDs: types.StringArray{"kb-1"},
				Capabilities:     types.StringArray{string(types.APIKeyCapabilityRetrieve)},
			},
			wantCode: http.StatusForbidden,
		},
		{
			name: "non-retrieve capability denied",
			scope: types.TenantAPIKeyScope{
				Capabilities: types.StringArray{string(types.APIKeyCapabilityChat)},
			},
			wantCode: http.StatusForbidden,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := gin.New()
			serveFiles(engine, &stubFileService{
				getFile: func(_ context.Context, _ string) (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader("body")), nil
				},
			})

			req := httptest.NewRequest(http.MethodGet, "/files?file_path="+url.QueryEscape(filePath), nil)
			ctx := context.WithValue(req.Context(), types.TenantInfoContextKey, &types.Tenant{ID: 42})
			ctx = types.WithTenantAPIKeyScope(ctx, tc.scope)
			req = req.WithContext(ctx)

			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)

			if got := recorder.Code; got != tc.wantCode {
				t.Fatalf("status = %d, want %d body=%s", got, tc.wantCode, recorder.Body.String())
			}
		})
	}
}

func TestServeFilesForcesActiveContentDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("STORAGE_TYPE", "local")

	engine := gin.New()
	serveFiles(engine, &stubFileService{
		getFile: func(_ context.Context, _ string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`<svg onload="alert(1)"></svg>`)), nil
		},
	})

	filePath := "local://42/docs/payload.svg"
	req := httptest.NewRequest(http.MethodGet, "/files?file_path="+url.QueryEscape(filePath), nil)
	req = req.WithContext(context.WithValue(req.Context(), types.TenantInfoContextKey, &types.Tenant{ID: 42}))

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("Content-Type = %q, want application/octet-stream", got)
	}
	if got := recorder.Header().Get("Content-Disposition"); got != "attachment; filename=payload.svg" {
		t.Fatalf("Content-Disposition = %q, want attachment with filename", got)
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func (s *stubResourceCatalog) GetMessageFileBindings(
	context.Context,
	uint64,
	string,
	string,
) (*types.MessageFileBindings, error) {
	return s.fileBindings, nil
}
