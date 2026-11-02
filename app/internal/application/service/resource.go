package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	secutils "gaze/internal/utils"
)

const defaultResourceGrantTTL = 2 * time.Hour

type resourceCatalog struct {
	repo interfaces.ResourceRepository
}

// NewResourceCatalog creates the stable resource-reference domain service.
func NewResourceCatalog(repo interfaces.ResourceRepository) interfaces.ResourceCatalog {
	return &resourceCatalog{repo: repo}
}

func randomResourceToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func resourceLocationHash(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:])
}

func (s *resourceCatalog) Register(
	ctx context.Context,
	tenantID uint64,
	physicalPath string,
	meta interfaces.ResourceRegistration,
) (string, error) {
	physicalPath = strings.TrimSpace(physicalPath)
	if tenantID == 0 || physicalPath == "" {
		return "", fmt.Errorf("resource registration requires tenant and physical path")
	}
	if _, ok := types.ParseResourcePath(physicalPath); ok {
		return physicalPath, nil
	}
	locationHash := resourceLocationHash(physicalPath)
	existing, err := s.repo.GetByTenantLocation(ctx, tenantID, locationHash)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return types.BuildResourcePath(existing.Handle), nil
	}

	backendID, inner, scoped := types.ParseStorageBackendPath(physicalPath)
	providerPath := physicalPath
	if scoped {
		providerPath = inner
	}
	provider := types.ParseProviderScheme(providerPath)
	if provider == "" {
		return "", fmt.Errorf("resource physical path has unsupported provider scheme")
	}
	lifecycle := types.ResourceLifecyclePersistent
	if meta.Temporary {
		lifecycle = types.ResourceLifecycleTemporary
	}
	for attempt := 0; attempt < 4; attempt++ {
		handle, tokenErr := randomResourceToken()
		if tokenErr != nil {
			return "", tokenErr
		}
		resource := &types.StoredResource{
			Handle:           handle,
			TenantID:         tenantID,
			StorageBackendID: backendID,
			Provider:         provider,
			PhysicalPath:     physicalPath,
			LocationHash:     locationHash,
			Kind:             meta.Kind,
			MimeType:         meta.MimeType,
			OriginalName:     meta.OriginalName,
			Size:             meta.Size,
			ContentHash:      meta.ContentHash,
			Lifecycle:        lifecycle,
		}
		if err := s.repo.Create(ctx, resource); err == nil {
			return types.BuildResourcePath(handle), nil
		} else if !strings.Contains(strings.ToLower(err.Error()), "unique") {
			return "", err
		}
		existing, lookupErr := s.repo.GetByTenantLocation(ctx, tenantID, locationHash)
		if lookupErr == nil && existing != nil {
			return types.BuildResourcePath(existing.Handle), nil
		}
	}
	return "", fmt.Errorf("failed to allocate unique resource handle")
}

func (s *resourceCatalog) Resolve(ctx context.Context, reference string) (*types.StoredResource, error) {
	handle, ok := types.ParseResourcePath(reference)
	if !ok {
		return nil, fmt.Errorf("invalid resource reference")
	}
	resource, err := s.repo.GetByHandle(ctx, handle)
	if err != nil {
		return nil, err
	}
	if resource == nil {
		return nil, fmt.Errorf("resource not found")
	}
	return resource, nil
}

func (s *resourceCatalog) ResolvePath(ctx context.Context, value string) (string, *types.StoredResource, error) {
	if _, ok := types.ParseResourcePath(value); !ok {
		return value, nil, nil
	}
	resource, err := s.Resolve(ctx, value)
	if err != nil {
		return "", nil, err
	}
	return resource.PhysicalPath, resource, nil
}

func (s *resourceCatalog) Bind(ctx context.Context, reference, ownerType, ownerID, relation string) error {
	resource, err := s.Resolve(ctx, reference)
	if err != nil {
		return err
	}
	if strings.TrimSpace(ownerType) == "" || strings.TrimSpace(ownerID) == "" {
		return fmt.Errorf("resource binding requires owner type and id")
	}
	if relation == "" {
		relation = "attachment"
	}
	return s.repo.CreateBinding(ctx, &types.ResourceBinding{
		ResourceID: resource.ID,
		TenantID:   resource.TenantID,
		OwnerType:  ownerType,
		OwnerID:    ownerID,
		Relation:   relation,
	})
}

func (s *resourceCatalog) IsReferencedByKnowledgeBase(
	ctx context.Context,
	tenantID uint64,
	kbID, reference string,
) (bool, error) {
	physical, resource, err := s.ResolvePath(ctx, reference)
	if err != nil {
		return false, err
	}
	if resource == nil {
		resource, err = s.repo.GetByTenantLocation(ctx, tenantID, resourceLocationHash(physical))
		if err != nil {
			return false, err
		}
	}
	if resource == nil || resource.TenantID != tenantID {
		return false, nil
	}
	return s.repo.IsReferencedByKnowledgeBase(ctx, tenantID, kbID, resource.ID)
}

// GetMessageFileBindings resolves registered aliases before reading authoritative origins.
func (s *resourceCatalog) GetMessageFileBindings(
	ctx context.Context, tenantID uint64, reference, messageID string,
) (*types.MessageFileBindings, error) {
	physical, resource, err := s.ResolvePath(ctx, reference)
	if err != nil {
		return nil, err
	}
	if resource == nil {
		resource, err = s.repo.GetByTenantLocation(ctx, tenantID, resourceLocationHash(physical))
		if err != nil {
			return nil, err
		}
	}
	if resource == nil || resource.TenantID != tenantID {
		return &types.MessageFileBindings{}, nil
	}
	return s.repo.GetMessageFileBindings(ctx, tenantID, resource.ID, messageID)
}

// Release implements interfaces.ResourceCatalog.
//
// Unbinding and counting are deliberately not a single transaction. A racing
// bind that lands between them makes the count too high, which keeps a live
// file — the safe direction. The opposite ordering could delete bytes another
// owner had just claimed.
func (s *resourceCatalog) Release(
	ctx context.Context, reference, ownerType, ownerID string,
) (int64, error) {
	if strings.TrimSpace(ownerType) == "" || strings.TrimSpace(ownerID) == "" {
		return -1, fmt.Errorf("resource release requires owner type and id")
	}
	if _, ok := types.ParseResourcePath(reference); !ok {
		// A raw provider path predates the catalog and has no bindings to
		// account for; the caller keeps its previous delete behaviour.
		return -1, nil
	}
	resource, err := s.Resolve(ctx, reference)
	if err != nil {
		return -1, err
	}
	if err := s.repo.DeleteBinding(ctx, resource.ID, ownerType, ownerID); err != nil {
		return -1, err
	}
	return s.repo.CountBindings(ctx, resource.ID)
}

func (s *resourceCatalog) MarkDeleted(ctx context.Context, reference string) error {
	resource, err := s.Resolve(ctx, reference)
	if err != nil {
		return err
	}
	return s.repo.MarkDeleted(ctx, resource.ID)
}

// accessGrantKey signs stateless file-access tokens. SYSTEM_AES_KEY is the
// deployment-wide key; the JWT secret keeps local single-process runs usable.
func accessGrantKey() []byte {
	if key := secutils.SystemHMACKey(); key != nil {
		return key
	}
	return []byte(getJwtSecret())
}

func signAccessGrant(payload string) string {
	mac := hmac.New(sha256.New, accessGrantKey())
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// CreateAccessGrant mints a stateless, expiring read token for a resource.
// The token is self-contained (resource id + expiry + HMAC), so file rendering
// needs no write path and no access-grant table.
func (s *resourceCatalog) CreateAccessGrant(ctx context.Context, reference string, ttl time.Duration) (string, error) {
	resource, err := s.Resolve(ctx, reference)
	if err != nil {
		return "", err
	}
	if ttl <= 0 {
		ttl = defaultResourceGrantTTL
	}
	payload := resource.ID + ":" + strconv.FormatInt(time.Now().UTC().Add(ttl).Unix(), 10)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + signAccessGrant(payload), nil
}

// ResolveAccessGrant verifies a token's signature and expiry, then loads the
// resource it names. Grants are stateless, so revocation is by short TTL;
// deleting the resource makes the token stop resolving.
func (s *resourceCatalog) ResolveAccessGrant(ctx context.Context, token string) (*types.StoredResource, error) {
	token = strings.TrimSpace(token)
	encodedPayload, signature, ok := strings.Cut(token, ".")
	if !ok || encodedPayload == "" || signature == "" {
		return nil, fmt.Errorf("resource access grant is invalid or expired")
	}
	rawPayload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("resource access grant is invalid or expired")
	}
	payload := string(rawPayload)
	expected := signAccessGrant(payload)
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return nil, fmt.Errorf("resource access grant is invalid or expired")
	}
	resourceID, expiryRaw, ok := strings.Cut(payload, ":")
	if !ok || resourceID == "" {
		return nil, fmt.Errorf("resource access grant is invalid or expired")
	}
	expiry, err := strconv.ParseInt(expiryRaw, 10, 64)
	if err != nil || time.Now().UTC().Unix() >= expiry {
		return nil, fmt.Errorf("resource access grant is invalid or expired")
	}
	resource, err := s.repo.GetByID(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	if resource == nil || resource.State == types.ResourceStateDeleted {
		return nil, fmt.Errorf("resource not found")
	}
	return resource, nil
}
