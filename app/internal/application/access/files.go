package access

import (
	"context"
	"encoding/json"
	"strings"

	"gaze/internal/types"
	secutils "gaze/internal/utils"
)

// FileAccess is a request-local result, produced before storage is opened.
// Physical locators are not a substitute for message authorization.
type FileAccess struct {
	OwnerTenantID    uint64
	Path             string
	Filename         string
	StorageBackendID string
}

// FileCatalog resolves logical references to their stored owner and locator.
type FileCatalog interface {
	ResolvePath(context.Context, string) (string, *types.StoredResource, error)
}

func resolveFile(
	ctx context.Context,
	catalog FileCatalog,
	reference string,
) (FileAccess, *types.StoredResource, error) {
	file := FileAccess{Path: reference, Filename: reference}
	if catalog == nil {
		if _, resource := types.ParseResourcePath(reference); resource {
			return file, nil, ErrNotFound
		}
		return file, nil, nil
	}
	path, resource, err := catalog.ResolvePath(ctx, reference)
	if err != nil {
		return file, nil, ErrNotFound
	}
	file.Path = path
	file.Filename = path
	if resource != nil {
		file.OwnerTenantID = resource.TenantID
		file.StorageBackendID = resource.StorageBackendID
		if strings.TrimSpace(resource.OriginalName) != "" {
			file.Filename = resource.OriginalName
		}
	}
	return file, resource, nil
}

// MessageReferencesFile examines only persisted rendering/output fields. Tool
// arguments and request metadata are not evidence of a returned file.
func MessageReferencesFile(message *types.Message, reference string) bool {
	if message == nil {
		return false
	}
	if types.ContainsStorageReference(message.Content, reference) {
		return true
	}
	for _, artifact := range message.Artifacts {
		if artifact.URL == reference {
			return true
		}
	}
	for _, value := range []interface{}{message.KnowledgeReferences, message.Images} {
		data, _ := json.Marshal(value)
		if types.ContainsStorageReference(string(data), reference) {
			return true
		}
	}
	for _, step := range message.AgentSteps {
		for _, call := range step.ToolCalls {
			if call.Result == nil {
				continue
			}
			data, _ := json.Marshal(call.Result)
			if types.ContainsStorageReference(string(data), reference) {
				return true
			}
		}
	}
	return false
}

// AuthorizeMessageFile applies the personal edition's single-owner rule: the
// message's agent tenant and the stored resource must both belong to the
// caller's tenant, and the reference must appear in persisted message output.
func AuthorizeMessageFile(
	ctx context.Context,
	message *types.Message,
	reference string,
	catalog FileCatalog,
) (FileAccess, error) {
	caller := types.CallerFromContext(ctx)
	if caller.TenantID == 0 {
		return FileAccess{}, ErrUnauthorized
	}
	if !MessageReferencesFile(message, reference) {
		return FileAccess{}, ErrForbidden
	}
	file, resource, err := resolveFile(ctx, catalog, reference)
	if err != nil {
		return FileAccess{}, err
	}
	owner := message.AgentTenantID
	if resource != nil {
		owner = resource.TenantID
	}
	if owner == 0 || owner != caller.TenantID {
		return FileAccess{}, ErrForbidden
	}
	if resource == nil {
		if err := secutils.ValidateStoragePathTenant(file.Path, owner); err != nil {
			return FileAccess{}, ErrForbidden
		}
	}
	file.OwnerTenantID = owner
	return file, nil
}

// ResolveMessageArtifact selects a persisted artifact after session ownership
// was checked.
func ResolveMessageArtifact(
	ctx context.Context,
	message *types.Message,
	index int,
	catalog FileCatalog,
) (FileAccess, error) {
	caller := types.CallerFromContext(ctx)
	if caller.TenantID == 0 {
		return FileAccess{}, ErrUnauthorized
	}
	if message == nil || index < 0 || index >= len(message.Artifacts) {
		return FileAccess{}, ErrNotFound
	}
	artifact := message.Artifacts[index]
	file, err := AuthorizeMessageFile(ctx, message, artifact.URL, catalog)
	file.Filename = artifact.FileName
	return file, err
}
