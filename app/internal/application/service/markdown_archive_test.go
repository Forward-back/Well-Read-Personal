package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"strings"
	"testing"

	"gaze/internal/types"
	"github.com/stretchr/testify/require"
)

type archiveFileServiceStub struct {
	savedDocs map[string]string
	savedURLs []string
}

func (s *archiveFileServiceStub) CheckConnectivity(context.Context) error { return nil }

func (s *archiveFileServiceStub) SaveFile(
	_ context.Context, file *multipart.FileHeader, _ uint64, knowledgeID string,
) (string, error) {
	f, err := file.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()
	content, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	if s.savedDocs == nil {
		s.savedDocs = make(map[string]string)
	}
	s.savedDocs[knowledgeID] = string(content)
	return "stored/" + knowledgeID, nil
}

func (s *archiveFileServiceStub) SaveBytes(
	_ context.Context, _ []byte, _ uint64, fileName string, _ bool,
) (string, error) {
	url := "local://images/" + fileName
	s.savedURLs = append(s.savedURLs, url)
	return url, nil
}

func (s *archiveFileServiceStub) GetFile(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func (s *archiveFileServiceStub) GetFileURL(context.Context, string) (string, error) {
	return "", errors.New("not implemented")
}

func (s *archiveFileServiceStub) DeleteFile(context.Context, string) error { return nil }

func (s *archiveFileServiceStub) CopyFile(context.Context, string, uint64, string) (string, error) {
	return "", errors.New("not implemented")
}

func newMarkdownArchiveTestService(t *testing.T) (*knowledgeService, *archiveFileServiceStub, *createKnowledgeTaskEnqueuerStub) {
	t.Helper()
	fileSvc := &archiveFileServiceStub{}
	task := &createKnowledgeTaskEnqueuerStub{}
	return &knowledgeService{
		repo:      &createKnowledgeFileRepoStub{},
		kbService: &createKnowledgeFileKBServiceStub{kb: &types.KnowledgeBase{ID: "kb-1"}},
		fileSvc:   fileSvc,
		task:      task,
	}, fileSvc, task
}

func buildZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, content := range files {
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = entry.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buf.Bytes()
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 120, 120))
	for x := 0; x < 120; x++ {
		for y := 0; y < 120; y++ {
			img.Set(x, y, color.RGBA{R: 10, G: 120, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestCreateKnowledgeFromMarkdownArchiveImportsEachMarkdown(t *testing.T) {
	archive := buildZip(t, map[string][]byte{
		"guide/intro.md":     []byte("# Intro\n\n![arch](images/a.png)\n\n![remote](https://example.com/x.png)"),
		"guide/images/a.png": testPNG(t),
		"other/b.md":         []byte("![shared](../guide/images/a.png)"),
		"guide/skip.txt":     []byte("ignored"),
	})
	header, err := multipartHeaderFromBytes("docs.zip", archive)
	require.NoError(t, err)

	svc, fileSvc, task := newMarkdownArchiveTestService(t)
	result, err := svc.CreateKnowledgeFromMarkdownArchive(
		newCreateKnowledgeFileContext(), "kb-1", header, nil, nil, nil, "", nil,
	)
	require.NoError(t, err)
	require.Equal(t, 2, result.MarkdownCount)
	require.Len(t, result.Documents, 2)
	require.Empty(t, result.Errors)
	require.Equal(t, 2, result.ImagesImported)
	require.Equal(t, 1, result.ImagesSkipped)
	require.Len(t, fileSvc.savedURLs, 1, "same zip image should be stored once")
	require.Equal(t, 2, task.calls)

	for _, doc := range result.Documents {
		saved := fileSvc.savedDocs[doc.Knowledge.ID]
		require.Contains(t, saved, "local://images/")
		require.NotContains(t, saved, "images/a.png")
		require.NotContains(t, saved, "https://example.com/x.png")
	}
	first := ""
	for _, doc := range result.Documents {
		if doc.FileName == "intro.md" {
			first = fileSvc.savedDocs[doc.Knowledge.ID]
		}
	}
	require.Contains(t, first, "remote", "skipped image should keep alt text")
}

func TestCreateKnowledgeFromMarkdownArchiveRejectsTraversal(t *testing.T) {
	archive := buildZip(t, map[string][]byte{
		"../evil.md": []byte("# evil"),
	})
	header, err := multipartHeaderFromBytes("docs.zip", archive)
	require.NoError(t, err)

	svc, _, _ := newMarkdownArchiveTestService(t)
	_, err = svc.CreateKnowledgeFromMarkdownArchive(
		newCreateKnowledgeFileContext(), "kb-1", header, nil, nil, nil, "", nil,
	)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "路径非法"))
}

func TestCreateKnowledgeFromMarkdownArchiveRequiresMarkdown(t *testing.T) {
	archive := buildZip(t, map[string][]byte{"readme.txt": []byte("no md")})
	header, err := multipartHeaderFromBytes("docs.zip", archive)
	require.NoError(t, err)

	svc, _, _ := newMarkdownArchiveTestService(t)
	_, err = svc.CreateKnowledgeFromMarkdownArchive(
		newCreateKnowledgeFileContext(), "kb-1", header, nil, nil, nil, "", nil,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "未找到 Markdown")
}
