package service

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	apprepo "gaze/internal/application/repository"
	"gaze/internal/types"
)

type fakeBackgroundRepo struct {
	items  map[string]*types.UserBackground
	choice string
}

func newFakeBackgroundRepo() *fakeBackgroundRepo {
	return &fakeBackgroundRepo{items: map[string]*types.UserBackground{}, choice: types.BackgroundChoiceDefault}
}

func (r *fakeBackgroundRepo) CreateBackground(_ context.Context, bg *types.UserBackground) error {
	copy := *bg
	r.items[bg.ID] = &copy
	return nil
}

func (r *fakeBackgroundRepo) ListBackgrounds(_ context.Context, userID string) ([]*types.UserBackground, error) {
	out := make([]*types.UserBackground, 0, len(r.items))
	for _, item := range r.items {
		if item.UserID == userID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (r *fakeBackgroundRepo) GetBackground(_ context.Context, userID, id string) (*types.UserBackground, error) {
	item, ok := r.items[id]
	if !ok || item.UserID != userID {
		return nil, apprepo.ErrBackgroundNotFound
	}
	return item, nil
}

func (r *fakeBackgroundRepo) DeleteBackground(_ context.Context, userID, id string) error {
	item, ok := r.items[id]
	if !ok || item.UserID != userID {
		return apprepo.ErrBackgroundNotFound
	}
	delete(r.items, id)
	return nil
}

func (r *fakeBackgroundRepo) GetBackgroundChoice(context.Context, string) (string, error) {
	return r.choice, nil
}

func (r *fakeBackgroundRepo) UpdateBackgroundChoice(_ context.Context, _ string, choice string) error {
	r.choice = choice
	return nil
}

func buildBackgroundFileHeader(t *testing.T, name string, data []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}
	return req.MultipartForm.File["file"][0]
}

// 1x1 transparent PNG.
var backgroundTestPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

func newBackgroundServiceForTest(t *testing.T, repo *fakeBackgroundRepo) *backgroundService {
	t.Helper()
	t.Setenv("LOCAL_STORAGE_BASE_DIR", t.TempDir())
	svc := NewBackgroundService(repo).(*backgroundService)
	return svc
}

func TestUploadBackgroundStoresOriginalAndSelects(t *testing.T) {
	repo := newFakeBackgroundRepo()
	svc := newBackgroundServiceForTest(t, repo)
	file := buildBackgroundFileHeader(t, "my photo.png", backgroundTestPNG)

	bg, err := svc.UploadBackground(context.Background(), "user-1", file)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if bg.MimeType != "image/png" {
		t.Fatalf("mime = %q, want image/png", bg.MimeType)
	}
	if bg.SizeBytes != int64(len(backgroundTestPNG)) {
		t.Fatalf("size = %d, want %d", bg.SizeBytes, len(backgroundTestPNG))
	}
	if bg.DisplayName != "my photo.png" {
		t.Fatalf("display name = %q", bg.DisplayName)
	}
	if repo.choice != types.BackgroundChoiceUpload+bg.ID {
		t.Fatalf("choice = %q, want upload:%s", repo.choice, bg.ID)
	}
	reader, _, err := svc.OpenBackground(context.Background(), "user-1", bg.ID)
	if err != nil {
		t.Fatalf("open uploaded background: %v", err)
	}
	_ = reader.Close()
}

func TestUploadBackgroundRejectsNonImage(t *testing.T) {
	repo := newFakeBackgroundRepo()
	svc := newBackgroundServiceForTest(t, repo)
	file := buildBackgroundFileHeader(t, "notes.txt", []byte("plain text"))

	if _, err := svc.UploadBackground(context.Background(), "user-1", file); err != ErrBackgroundType {
		t.Fatalf("err = %v, want ErrBackgroundType", err)
	}
	if len(repo.items) != 0 {
		t.Fatalf("rejected upload must not create metadata")
	}
}

func TestUploadBackgroundEnforcesCount(t *testing.T) {
	repo := newFakeBackgroundRepo()
	for i := 0; i < MaxBackgroundUploadCount; i++ {
		repo.items[string(rune('a'+i))] = &types.UserBackground{ID: string(rune('a' + i)), UserID: "user-1"}
	}
	svc := newBackgroundServiceForTest(t, repo)
	file := buildBackgroundFileHeader(t, "photo.png", backgroundTestPNG)

	if _, err := svc.UploadBackground(context.Background(), "user-1", file); err != ErrBackgroundLimit {
		t.Fatalf("err = %v, want ErrBackgroundLimit", err)
	}
}

func TestDeleteBackgroundResetsSelectedChoice(t *testing.T) {
	repo := newFakeBackgroundRepo()
	svc := newBackgroundServiceForTest(t, repo)
	file := buildBackgroundFileHeader(t, "photo.png", backgroundTestPNG)
	bg, err := svc.UploadBackground(context.Background(), "user-1", file)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	abs, _, err := svc.OpenBackground(context.Background(), "user-1", bg.ID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	absPath := abs.(*os.File).Name()
	_ = abs.Close()

	if err := svc.DeleteBackground(context.Background(), "user-1", bg.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if repo.choice != types.BackgroundChoiceDefault {
		t.Fatalf("choice = %q, want default", repo.choice)
	}
	if _, err := os.Stat(absPath); !os.IsNotExist(err) {
		t.Fatalf("background file still exists: %v", err)
	}
}

func TestSetBackgroundChoiceValidatesBuiltinAndUpload(t *testing.T) {
	repo := newFakeBackgroundRepo()
	svc := newBackgroundServiceForTest(t, repo)
	ctx := context.Background()

	if err := svc.SetBackgroundChoice(ctx, "user-1", "builtin:3"); err != nil {
		t.Fatalf("builtin choice: %v", err)
	}
	if err := svc.SetBackgroundChoice(ctx, "user-1", "builtin:9"); err != ErrBackgroundChoice {
		t.Fatalf("err = %v, want ErrBackgroundChoice", err)
	}
	if err := svc.SetBackgroundChoice(ctx, "user-1", "upload:missing"); err != ErrBackgroundChoice {
		t.Fatalf("err = %v, want ErrBackgroundChoice", err)
	}
}
