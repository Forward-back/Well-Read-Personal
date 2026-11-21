package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	apprepo "gaze/internal/application/repository"
	"gaze/internal/logger"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	secutils "gaze/internal/utils"
)

const (
	MaxBackgroundUploadBytes int64 = 10 << 20
	MaxBackgroundUploadCount       = 10
)

var (
	ErrBackgroundTooLarge = errors.New("background image exceeds 10 MB")
	ErrBackgroundType     = errors.New("background must be a JPEG, PNG or WebP image")
	ErrBackgroundLimit    = errors.New("at most 10 uploaded backgrounds are allowed")
	ErrBackgroundChoice   = errors.New("invalid background choice")
	ErrBackgroundNotFound = errors.New("background not found")
)

func mapBackgroundRepoErr(err error) error {
	if errors.Is(err, apprepo.ErrBackgroundNotFound) {
		return ErrBackgroundNotFound
	}
	return err
}

var backgroundMIMETypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

type backgroundService struct {
	repo    interfaces.BackgroundRepository
	baseDir string
}

func NewBackgroundService(repo interfaces.BackgroundRepository) interfaces.BackgroundService {
	baseDir := strings.TrimSpace(os.Getenv("LOCAL_STORAGE_BASE_DIR"))
	if baseDir == "" {
		baseDir = "/data/files"
	}
	return &backgroundService{repo: repo, baseDir: baseDir}
}

func (s *backgroundService) ListBackgrounds(ctx context.Context, userID string) (*types.BackgroundList, error) {
	choice, err := s.repo.GetBackgroundChoice(ctx, userID)
	if err != nil {
		return nil, err
	}
	uploads, err := s.repo.ListBackgrounds(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !s.choiceExists(ctx, userID, choice) {
		choice = types.BackgroundChoiceDefault
	}
	return &types.BackgroundList{
		Choice:         choice,
		Uploads:        uploads,
		MaxUploadBytes: MaxBackgroundUploadBytes,
		MaxUploadCount: MaxBackgroundUploadCount,
	}, nil
}

func (s *backgroundService) UploadBackground(
	ctx context.Context, userID string, file *multipart.FileHeader,
) (*types.UserBackground, error) {
	if file == nil {
		return nil, ErrBackgroundType
	}
	if file.Size <= 0 || file.Size > MaxBackgroundUploadBytes {
		return nil, ErrBackgroundTooLarge
	}
	uploads, err := s.repo.ListBackgrounds(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(uploads) >= MaxBackgroundUploadCount {
		return nil, ErrBackgroundLimit
	}

	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open upload: %w", err)
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, MaxBackgroundUploadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	if len(data) == 0 {
		return nil, ErrBackgroundType
	}
	if int64(len(data)) > MaxBackgroundUploadBytes {
		return nil, ErrBackgroundTooLarge
	}
	mime := http.DetectContentType(data)
	ext, ok := backgroundMIMETypes[mime]
	if !ok {
		return nil, ErrBackgroundType
	}

	id := uuid.NewString()
	relPath := path.Join("backgrounds", userID, id+ext)
	absPath, err := secutils.SafeJoinUnderBase(s.baseDir, relPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return nil, fmt.Errorf("create background directory: %w", err)
	}
	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("write background: %w", err)
	}

	bg := &types.UserBackground{
		ID:          id,
		UserID:      userID,
		DisplayName: sanitizeBackgroundName(file.Filename, ext),
		MimeType:    mime,
		SizeBytes:   int64(len(data)),
		FilePath:    filepath.ToSlash(relPath),
		CreatedAt:   time.Now(),
	}
	if err := s.repo.CreateBackground(ctx, bg); err != nil {
		_ = os.Remove(absPath)
		return nil, err
	}
	if err := s.repo.UpdateBackgroundChoice(ctx, userID, types.BackgroundChoiceUpload+id); err != nil {
		_ = s.repo.DeleteBackground(ctx, userID, id)
		_ = os.Remove(absPath)
		return nil, err
	}
	logger.Infof(ctx, "Background uploaded: user=%s id=%s size=%d", userID, id, bg.SizeBytes)
	return bg, nil
}

func (s *backgroundService) DeleteBackground(ctx context.Context, userID, id string) error {
	bg, err := s.repo.GetBackground(ctx, userID, id)
	if err != nil {
		return mapBackgroundRepoErr(err)
	}
	if err := s.repo.DeleteBackground(ctx, userID, id); err != nil {
		return mapBackgroundRepoErr(err)
	}
	if absPath, err := secutils.SafeJoinUnderBase(s.baseDir, bg.FilePath); err == nil {
		if err := os.Remove(absPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			logger.Warnf(ctx, "Failed to remove background file %s: %v", absPath, err)
		}
	}
	choice, err := s.repo.GetBackgroundChoice(ctx, userID)
	if err == nil && choice == types.BackgroundChoiceUpload+id {
		if err := s.repo.UpdateBackgroundChoice(ctx, userID, types.BackgroundChoiceDefault); err != nil {
			return err
		}
	}
	logger.Infof(ctx, "Background deleted: user=%s id=%s", userID, id)
	return nil
}

func (s *backgroundService) SetBackgroundChoice(ctx context.Context, userID, choice string) error {
	choice = strings.TrimSpace(choice)
	if !s.choiceExists(ctx, userID, choice) {
		return ErrBackgroundChoice
	}
	return s.repo.UpdateBackgroundChoice(ctx, userID, choice)
}

func (s *backgroundService) OpenBackground(
	ctx context.Context, userID, id string,
) (io.ReadCloser, *types.UserBackground, error) {
	bg, err := s.repo.GetBackground(ctx, userID, id)
	if err != nil {
		return nil, nil, mapBackgroundRepoErr(err)
	}
	absPath, err := secutils.SafeJoinUnderBase(s.baseDir, bg.FilePath)
	if err != nil {
		return nil, nil, err
	}
	file, err := os.Open(absPath)
	if err != nil {
		return nil, nil, err
	}
	return file, bg, nil
}

// choiceExists validates "default", "builtin:1".."builtin:5" and uploaded
// ids that belong to this user. Empty/invalid choices are treated as absent.
func (s *backgroundService) choiceExists(ctx context.Context, userID, choice string) bool {
	switch {
	case choice == types.BackgroundChoiceDefault:
		return true
	case strings.HasPrefix(choice, types.BackgroundChoiceBuiltin):
		n, err := strconv.Atoi(strings.TrimPrefix(choice, types.BackgroundChoiceBuiltin))
		return err == nil && n >= 1 && n <= types.BackgroundBuiltinCount
	case strings.HasPrefix(choice, types.BackgroundChoiceUpload):
		id := strings.TrimPrefix(choice, types.BackgroundChoiceUpload)
		if id == "" {
			return false
		}
		_, err := s.repo.GetBackground(ctx, userID, id)
		return err == nil
	default:
		return false
	}
}

func sanitizeBackgroundName(original, ext string) string {
	name := strings.ReplaceAll(original, "\\", "/")
	name = path.Base(name)
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." {
		name = "background" + ext
	}
	if utf8.RuneCountInString(name) > 120 {
		runes := []rune(name)
		name = string(runes[:120])
	}
	return name
}
