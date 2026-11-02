package service

import (
	"context"
	"errors"
	"strings"

	"gaze/internal/application/access"
	"gaze/internal/application/repository"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	secutils "gaze/internal/utils"
)

var (
	ErrKnowledgeFolderNotFound      = errors.New("knowledge folder not found")
	ErrKnowledgeFolderNotEmpty      = errors.New("knowledge folder is not empty")
	ErrKnowledgeFolderInvalidMove   = errors.New("invalid knowledge folder move")
	ErrKnowledgeFolderNameRequired  = errors.New("knowledge folder name is required")
	ErrKnowledgeFolderNameInvalid   = errors.New("knowledge folder name contains invalid characters")
	ErrKnowledgeFolderAlreadyExists = errors.New("knowledge folder already exists")
)

type knowledgeFolderService struct {
	repo      interfaces.KnowledgeFolderRepository
	kbService interfaces.KnowledgeBaseService
}

// NewKnowledgeFolderService wires real folder CRUD. Handler routes still own
// the coarse RBAC gate; this service re-checks KB write access for mutations.
func NewKnowledgeFolderService(
	repo interfaces.KnowledgeFolderRepository,
	kbService interfaces.KnowledgeBaseService,
) interfaces.KnowledgeFolderService {
	return &knowledgeFolderService{repo: repo, kbService: kbService}
}

func (s *knowledgeFolderService) authorizeWrite(ctx context.Context, kbID string) (*types.KnowledgeBase, error) {
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if kb == nil || kb.ID != kbID {
		return nil, ErrKnowledgeFolderNotFound
	}
	if err := access.RequireKBWrite(ctx, kb); err != nil {
		return nil, err
	}
	return kb, nil
}

func (s *knowledgeFolderService) root(ctx context.Context, kbID string) (*types.KnowledgeFolder, error) {
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if kb == nil || kb.ID != kbID {
		return nil, ErrKnowledgeFolderNotFound
	}
	createdBy, _ := types.UserIDFromContext(ctx)
	return s.repo.EnsureRoot(ctx, kb.TenantID, kbID, createdBy)
}

func (s *knowledgeFolderService) List(ctx context.Context, kbID string) (*types.KnowledgeFolderTree, error) {
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if kb == nil || kb.ID != kbID {
		return nil, ErrKnowledgeFolderNotFound
	}
	createdBy, _ := types.UserIDFromContext(ctx)
	if _, err := s.repo.EnsureRoot(ctx, kb.TenantID, kbID, createdBy); err != nil {
		return nil, err
	}
	folders, err := s.repo.ListByKB(ctx, kb.TenantID, kbID)
	if err != nil {
		return nil, err
	}
	counts, err := s.repo.CountDocumentsByFolder(ctx, kb.TenantID, kbID)
	if err != nil {
		return nil, err
	}
	return types.BuildKnowledgeFolderTreeFromEntities(folders, counts), nil
}

func cleanFolderName(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrKnowledgeFolderNameRequired
	}
	safe, ok := secutils.ValidateInput(raw)
	if !ok {
		return "", ErrKnowledgeFolderNameInvalid
	}
	safe = types.NormalizeKnowledgeFolderPath(safe)
	if safe == "" || strings.Contains(safe, "/") || len(safe) > types.MaxKnowledgeFolderSegmentLength {
		return "", ErrKnowledgeFolderNameInvalid
	}
	return safe, nil
}

func (s *knowledgeFolderService) Create(
	ctx context.Context, kbID, parentID, name string,
) (*types.KnowledgeFolder, error) {
	kb, err := s.authorizeWrite(ctx, kbID)
	if err != nil {
		return nil, err
	}
	name, err = cleanFolderName(name)
	if err != nil {
		return nil, err
	}
	parent, err := s.repo.GetByID(ctx, kb.TenantID, kbID, strings.TrimSpace(parentID))
	if err != nil {
		return nil, err
	}
	if parent == nil {
		return nil, ErrKnowledgeFolderNotFound
	}
	createdBy, _ := types.UserIDFromContext(ctx)
	folder := &types.KnowledgeFolder{
		TenantID:        kb.TenantID,
		KnowledgeBaseID: kbID,
		ParentID:        parent.ID,
		Name:            name,
		CreatedBy:       createdBy,
	}
	if err := s.repo.Create(ctx, folder); err != nil {
		return nil, err
	}
	return folder, nil
}

func (s *knowledgeFolderService) Rename(
	ctx context.Context, kbID, folderID, name string,
) (*types.KnowledgeFolder, error) {
	kb, err := s.authorizeWrite(ctx, kbID)
	if err != nil {
		return nil, err
	}
	name, err = cleanFolderName(name)
	if err != nil {
		return nil, err
	}
	folder, err := s.repo.RenameSubtree(ctx, kb.TenantID, kbID, folderID, name)
	if err != nil {
		return nil, err
	}
	return folder, nil
}

func (s *knowledgeFolderService) Move(
	ctx context.Context, kbID, folderID, newParentID string,
) (*types.KnowledgeFolder, error) {
	kb, err := s.authorizeWrite(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(newParentID) == "" {
		root, err := s.repo.EnsureRoot(ctx, kb.TenantID, kbID, "")
		if err != nil {
			return nil, err
		}
		newParentID = root.ID
	}
	folder, err := s.repo.MoveSubtree(ctx, kb.TenantID, kbID, folderID, newParentID)
	if err != nil {
		return nil, err
	}
	return folder, nil
}

func (s *knowledgeFolderService) Delete(ctx context.Context, kbID, folderID string) error {
	kb, err := s.authorizeWrite(ctx, kbID)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, kb.TenantID, kbID, folderID); err != nil {
		return err
	}
	return nil
}

func (s *knowledgeFolderService) EnsurePath(
	ctx context.Context, kbID, path, createdBy string,
) (*types.KnowledgeFolder, error) {
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if kb == nil || kb.ID != kbID {
		return nil, ErrKnowledgeFolderNotFound
	}
	return s.repo.EnsurePath(ctx, kb.TenantID, kbID, path, createdBy)
}

// Get returns one folder scoped to the KB.
func (s *knowledgeFolderService) Get(ctx context.Context, kbID, folderID string) (*types.KnowledgeFolder, error) {
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if kb == nil || kb.ID != kbID {
		return nil, ErrKnowledgeFolderNotFound
	}
	folder, err := s.repo.GetByID(ctx, kb.TenantID, kbID, strings.TrimSpace(folderID))
	if err != nil {
		return nil, err
	}
	if folder == nil {
		return nil, ErrKnowledgeFolderNotFound
	}
	return folder, nil
}

// SubtreeIDs resolves a folder selection into the concrete folder-id set used
// by document list filters. Empty folderID means the KB root.
func (s *knowledgeFolderService) SubtreeIDs(
	ctx context.Context, kbID, folderID string, recursive bool,
) ([]string, error) {
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if kb == nil || kb.ID != kbID {
		return nil, ErrKnowledgeFolderNotFound
	}
	createdBy, _ := types.UserIDFromContext(ctx)
	root, err := s.repo.EnsureRoot(ctx, kb.TenantID, kbID, createdBy)
	if err != nil {
		return nil, err
	}
	selected := root
	if strings.TrimSpace(folderID) != "" {
		selected, err = s.repo.GetByID(ctx, kb.TenantID, kbID, strings.TrimSpace(folderID))
		if err != nil {
			return nil, err
		}
		if selected == nil {
			return nil, ErrKnowledgeFolderNotFound
		}
	}
	if !recursive {
		return []string{selected.ID}, nil
	}
	folders, err := s.repo.ListByKB(ctx, kb.TenantID, kbID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(folders))
	prefix := selected.Path
	for _, folder := range folders {
		if folder == nil {
			continue
		}
		if selected.IsRoot || folder.Path == prefix || strings.HasPrefix(folder.Path, prefix+"/") {
			ids = append(ids, folder.ID)
		}
	}
	return ids, nil
}

func mapKnowledgeFolderError(err error) error {
	switch {
	case errors.Is(err, repository.ErrFolderNotFound):
		return ErrKnowledgeFolderNotFound
	case errors.Is(err, repository.ErrFolderNotEmpty):
		return ErrKnowledgeFolderNotEmpty
	case errors.Is(err, repository.ErrFolderInvalidMove):
		return ErrKnowledgeFolderInvalidMove
	case errors.Is(err, repository.ErrFolderAlreadyExists):
		return ErrKnowledgeFolderAlreadyExists
	default:
		return err
	}
}
