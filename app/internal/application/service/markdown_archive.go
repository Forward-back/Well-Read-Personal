package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	werrors "gaze/internal/errors"
	"gaze/internal/infrastructure/docparser"
	"gaze/internal/logger"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"github.com/google/uuid"
)

const (
	maxMarkdownArchiveEntries     = 10_000
	maxMarkdownArchiveEntryBytes  = 32 << 20
	maxMarkdownArchiveTotalBytes  = 512 << 20
	maxMarkdownArchiveMarkdownMax = 16 << 20
	maxMarkdownArchiveImages      = 1_000
	maxMarkdownArchiveImageBytes  = 20 << 20
	maxMarkdownArchiveReasons     = 10
)

var markdownArchiveImageExtensions = map[string]struct{}{
	"png": {}, "jpg": {}, "jpeg": {}, "gif": {}, "webp": {}, "bmp": {}, "tif": {}, "tiff": {},
}

type markdownArchiveEntry struct {
	file *zip.File
	name string
	size int64
}

// CreateKnowledgeFromMarkdownArchive imports a zip of Markdown files. Every
// Markdown entry becomes its own knowledge record; images referenced by
// relative paths are saved to the KB's storage backend and rewritten in the
// stored Markdown so the normal multimodal pipeline can OCR/caption them.
// Remote and absolute image references are skipped and reported.
func (s *knowledgeService) CreateKnowledgeFromMarkdownArchive(
	ctx context.Context,
	kbID string,
	file *multipart.FileHeader,
	metadata map[string]string,
	enableMultimodel *bool,
	tagIDs []string,
	channel string,
	processOverrides *types.KnowledgeProcessOverrides,
) (*types.MarkdownArchiveImportResult, error) {
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if kb == nil {
		return nil, werrors.NewNotFoundError("知识库不存在")
	}
	if kb.Type == types.KnowledgeBaseTypeFAQ {
		return nil, werrors.NewBadRequestError("FAQ 知识库不支持文件上传，请使用 FAQ 导入功能")
	}

	archive, err := readUploadedFile(file, maxMarkdownArchiveTotalBytes)
	if err != nil {
		return nil, err
	}
	entries, err := indexMarkdownArchive(archive)
	if err != nil {
		return nil, err
	}

	markdownEntries := make([]markdownArchiveEntry, 0)
	for _, entry := range entries {
		if isMarkdownArchiveName(entry.name) {
			markdownEntries = append(markdownEntries, entry)
		}
	}
	if len(markdownEntries) == 0 {
		return nil, werrors.NewBadRequestError("压缩包中未找到 Markdown 文件")
	}

	tenantID, _ := ctx.Value(types.TenantIDContextKey).(uint64)
	fileSvc := s.resolveFileService(ctx, kb)
	exact := make(map[string]*zip.File, len(entries))
	folded := make(map[string]*zip.File, len(entries))
	for _, entry := range entries {
		exact[entry.name] = entry.file
		key := strings.ToLower(entry.name)
		if _, exists := folded[key]; !exists {
			folded[key] = entry.file
		}
	}

	multimodal := enableMultimodel
	if multimodal == nil {
		enabled := true
		multimodal = &enabled
	}
	imageCache := make(map[string]string)

	result := &types.MarkdownArchiveImportResult{
		MarkdownCount: len(markdownEntries),
		Documents:     make([]types.MarkdownArchiveDocument, 0, len(markdownEntries)),
	}
	for _, entry := range markdownEntries {
		doc, importErr := s.importMarkdownArchiveEntry(
			ctx, kb, entry, exact, folded, imageCache, fileSvc, tenantID,
			metadata, multimodal, tagIDs, channel, processOverrides,
		)
		if importErr != nil {
			message := importErr.Error()
			if dup, ok := importErr.(*types.DuplicateKnowledgeError); ok && dup.Knowledge != nil {
				message = fmt.Sprintf("文档已存在：%s", dup.Knowledge.FileName)
			}
			logger.Warnf(ctx, "[MarkdownArchive] import %s failed: %v", entry.name, importErr)
			result.Errors = append(result.Errors, types.MarkdownArchiveError{
				Entry:   entry.name,
				Message: message,
			})
			continue
		}
		result.Documents = append(result.Documents, *doc)
		result.ImagesImported += doc.ImagesImported
		result.ImagesSkipped += doc.ImagesSkipped
	}
	return result, nil
}

func (s *knowledgeService) importMarkdownArchiveEntry(
	ctx context.Context,
	kb *types.KnowledgeBase,
	entry markdownArchiveEntry,
	exact map[string]*zip.File,
	folded map[string]*zip.File,
	imageCache map[string]string,
	fileSvc interfaces.FileService,
	tenantID uint64,
	metadata map[string]string,
	enableMultimodel *bool,
	tagIDs []string,
	channel string,
	processOverrides *types.KnowledgeProcessOverrides,
) (*types.MarkdownArchiveDocument, error) {
	source, err := readZipEntry(entry.file, maxMarkdownArchiveMarkdownMax)
	if err != nil {
		return nil, err
	}
	mdDir := path.Dir(entry.name)
	if mdDir == "." {
		mdDir = ""
	}

	imported := make(map[string]struct{})
	skipped := make([]string, 0)
	skippedSet := make(map[string]struct{})
	recordSkip := func(reason string) {
		if len(skipped) >= maxMarkdownArchiveReasons {
			return
		}
		if _, exists := skippedSet[reason]; exists {
			return
		}
		skippedSet[reason] = struct{}{}
		skipped = append(skipped, reason)
	}

	rewritten := docparser.RewriteMarkdownImageTargets(string(source), func(target string) (string, bool) {
		resolved, local, reason := resolveArchiveImagePath(mdDir, target)
		if !local {
			recordSkip(target + "：" + reason)
			return "", false
		}
		zipFile := lookupArchiveEntry(resolved, exact, folded)
		if zipFile == nil {
			recordSkip(target + "：压缩包内不存在")
			return "", false
		}
		ext := strings.ToLower(path.Ext(resolved))
		if _, ok := markdownArchiveImageExtensions[strings.TrimPrefix(ext, ".")]; !ok {
			recordSkip(target + "：不是支持的图片格式")
			return "", false
		}
		if cached, ok := imageCache[resolved]; ok {
			imported[cached] = struct{}{}
			return cached, true
		}
		if len(imageCache) >= maxMarkdownArchiveImages {
			recordSkip(target + "：超过单次导入图片上限")
			return "", false
		}
		data, readErr := readZipEntry(zipFile, maxMarkdownArchiveImageBytes)
		if readErr != nil {
			recordSkip(target + "：图片读取失败")
			return "", false
		}
		if docparser.IsIconImage(data) {
			recordSkip(target + "：图片过小，按图标跳过")
			return "", false
		}
		mimeType := mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = http.DetectContentType(data)
		}
		fileName := uuid.New().String() + ext
		servingURL, saveErr := fileSvc.SaveBytes(ctx, data, tenantID, fileName, false)
		if saveErr != nil {
			recordSkip(target + "：图片保存失败")
			return "", false
		}
		imageCache[resolved] = servingURL
		imported[servingURL] = struct{}{}
		return servingURL, true
	})

	fileName := path.Base(entry.name)
	header, err := multipartHeaderFromBytes(fileName, []byte(rewritten))
	if err != nil {
		return nil, err
	}
	originalHash := fmt.Sprintf("%x", md5.Sum(source))
	knowledge, err := s.createKnowledgeFromFile(
		ctx, kb.ID, header, metadata, enableMultimodel, entry.name, tagIDs, channel, processOverrides, originalHash,
	)
	if err != nil {
		return nil, err
	}

	return &types.MarkdownArchiveDocument{
		Knowledge:      knowledge,
		FileName:       fileName,
		FolderPath:     mdDir,
		ImagesImported: len(imported),
		ImagesSkipped:  len(skipped),
		SkippedReasons: skipped,
	}, nil
}

func indexMarkdownArchive(archive []byte) ([]markdownArchiveEntry, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, werrors.NewBadRequestError("文件不是有效的 zip 压缩包")
	}
	if len(reader.File) > maxMarkdownArchiveEntries {
		return nil, werrors.NewBadRequestError(
			fmt.Sprintf("压缩包条目超过上限（%d）", maxMarkdownArchiveEntries))
	}

	entries := make([]markdownArchiveEntry, 0, len(reader.File))
	var total int64
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || strings.HasPrefix(file.Name, "__MACOSX/") {
			continue
		}
		name, err := safeArchiveEntryName(file.Name)
		if err != nil {
			return nil, err
		}
		size := file.FileInfo().Size()
		if size > maxMarkdownArchiveEntryBytes {
			return nil, werrors.NewBadRequestError(
				fmt.Sprintf("压缩包条目过大：%s", file.Name))
		}
		total += size
		if total > maxMarkdownArchiveTotalBytes {
			return nil, werrors.NewBadRequestError("压缩包解压后超过大小上限")
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return nil, werrors.NewBadRequestError(
				fmt.Sprintf("压缩包包含符号链接：%s", file.Name))
		}
		entries = append(entries, markdownArchiveEntry{file: file, name: name, size: size})
	}
	return entries, nil
}

func safeArchiveEntryName(raw string) (string, error) {
	name := strings.ReplaceAll(raw, "\\", "/")
	name = path.Clean(name)
	if name == "." || name == "" || path.IsAbs(name) ||
		name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, "../") ||
		strings.ContainsRune(name, 0) || strings.Contains(name, ":") {
		return "", werrors.NewBadRequestError(fmt.Sprintf("压缩包条目路径非法：%s", raw))
	}
	return name, nil
}

func isMarkdownArchiveName(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".md", ".markdown":
		return !strings.HasPrefix(path.Base(name), ".")
	default:
		return false
	}
}

func resolveArchiveImagePath(mdDir, target string) (string, bool, string) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", false, "图片地址为空"
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return "", false, "图片地址无效"
	}
	if parsed.Scheme != "" || parsed.Host != "" {
		return "", false, "不是本地相对路径"
	}
	if strings.HasPrefix(target, "/") || strings.HasPrefix(target, "\\") {
		return "", false, "绝对路径不支持"
	}
	decoded, err := url.PathUnescape(parsed.Path)
	if err != nil {
		decoded = parsed.Path
	}
	decoded = strings.ReplaceAll(decoded, "\\", "/")
	if decoded == "" {
		return "", false, "图片地址为空"
	}
	resolved := path.Clean(path.Join(mdDir, decoded))
	if resolved == "." || resolved == ".." || strings.HasPrefix(resolved, "../") || path.IsAbs(resolved) {
		return "", false, "路径越出压缩包目录"
	}
	return resolved, true, ""
}

func lookupArchiveEntry(name string, exact, folded map[string]*zip.File) *zip.File {
	if file := exact[name]; file != nil {
		return file
	}
	return folded[strings.ToLower(name)]
}

func readZipEntry(entry *zip.File, max int64) ([]byte, error) {
	if entry == nil {
		return nil, fmt.Errorf("zip entry is nil")
	}
	rc, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("open zip entry %q: %w", entry.Name, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, max+1))
	if err != nil {
		return nil, fmt.Errorf("read zip entry %q: %w", entry.Name, err)
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("zip entry %q exceeds %d bytes", entry.Name, max)
	}
	return data, nil
}

func readUploadedFile(file *multipart.FileHeader, max int64) ([]byte, error) {
	if file == nil {
		return nil, werrors.NewBadRequestError("缺少上传文件")
	}
	rc, err := file.Open()
	if err != nil {
		return nil, werrors.NewBadRequestError("读取上传文件失败")
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, max+1))
	if err != nil {
		return nil, werrors.NewBadRequestError("读取上传文件失败")
	}
	if int64(len(data)) > max {
		return nil, werrors.NewBadRequestError("上传文件超过大小上限")
	}
	return data, nil
}

func multipartHeaderFromBytes(fileName string, content []byte) (*multipart.FileHeader, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(content); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	reader := multipart.NewReader(&body, writer.Boundary())
	form, err := reader.ReadForm(int64(len(content)) + 1<<20)
	if err != nil {
		return nil, err
	}
	files := form.File["file"]
	if len(files) == 0 {
		return nil, fmt.Errorf("multipart file header missing")
	}
	return files[0], nil
}
