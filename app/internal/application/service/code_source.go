package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	werrors "gaze/internal/errors"
	"gaze/internal/logger"
	"gaze/internal/types"
)

const (
	codeSourceMaxFileBytes = 1 << 20
	codeSourceMaxFiles     = 5000
	codeChunkMaxLines      = 120
	codeChunkMaxChars      = 8000
	defaultCodeSourceDir   = "/data/files/code-sources"
)

var codeSourceIgnoreDirs = map[string]struct{}{
	".git": {}, ".hg": {}, ".svn": {}, "node_modules": {}, "vendor": {},
	"dist": {}, "build": {}, "target": {}, "out": {}, "bin": {}, "obj": {},
	"__pycache__": {}, ".venv": {}, "venv": {}, ".idea": {}, ".vscode": {},
	"coverage": {}, ".next": {}, ".nuxt": {}, ".cache": {},
}

var codeExtensions = map[string]string{
	".go": "go", ".py": "python", ".java": "java", ".kt": "kotlin",
	".js": "javascript", ".jsx": "javascript", ".ts": "typescript", ".tsx": "typescript",
	".c": "c", ".h": "c", ".cc": "cpp", ".cpp": "cpp", ".hpp": "cpp", ".cs": "csharp",
	".rs": "rust", ".rb": "ruby", ".php": "php", ".swift": "swift", ".scala": "scala",
	".sh": "shell", ".bash": "shell", ".sql": "sql",
	".md": "markdown", ".markdown": "markdown", ".txt": "text", ".rst": "text",
	".yaml": "yaml", ".yml": "yaml", ".json": "json", ".toml": "toml",
	".ini": "ini", ".cfg": "ini", ".conf": "ini", ".xml": "xml",
	".html": "html", ".htm": "html", ".css": "css", ".scss": "css", ".less": "css",
}

var symbolPatterns = map[string][]*regexp.Regexp{
	"go": {
		regexp.MustCompile(`^\s*func\s+(?:\([^)]*\)\s*)?(\w+)`),
		regexp.MustCompile(`^\s*type\s+(\w+)`),
	},
	"python": {
		regexp.MustCompile(`^\s*(?:async\s+)?def\s+(\w+)`),
		regexp.MustCompile(`^\s*class\s+(\w+)`),
	},
	"javascript": {
		regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+(\w+)`),
		regexp.MustCompile(`^\s*(?:export\s+)?class\s+(\w+)`),
	},
	"typescript": {
		regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+(\w+)`),
		regexp.MustCompile(`^\s*(?:export\s+)?class\s+(\w+)`),
		regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s*)?\(`),
	},
	"java": {
		regexp.MustCompile(`^\s*(?:public|protected|private|static|final|abstract|synchronized|\s)*[\w<>\[\], ?]+\s+(\w+)\s*\([^;]*\)\s*(?:throws [\w, ]+)?\{`),
		regexp.MustCompile(`^\s*(?:public|protected|private)?\s*(?:static\s+)?(?:final\s+)?class\s+(\w+)`),
	},
	"rust": {
		regexp.MustCompile(`^\s*(?:pub\s+)?(?:async\s+)?fn\s+(\w+)`),
		regexp.MustCompile(`^\s*(?:pub\s+)?(?:struct|enum|trait)\s+(\w+)`),
	},
	"c": {
		regexp.MustCompile(`^\s*[\w\*]+\s+(\w+)\s*\([^;]*\)\s*\{`),
	},
	"cpp": {
		regexp.MustCompile(`^\s*[\w:\*&<>]+\s+(\w+)\s*\([^;]*\)\s*\{`),
		regexp.MustCompile(`^\s*(?:class|struct)\s+(\w+)`),
	},
}

var markdownHeading = regexp.MustCompile(`^\s{0,3}#{1,6}\s+`)

type codeChunk struct {
	Symbol     string
	SymbolType string
	StartLine  int
	EndLine    int
	Content    string
}

type codeFile struct {
	RelPath  string
	AbsPath  string
	Language string
	Content  string
	Hash     string
}

// SyncCodeSource scans a local directory or Git repository and indexes every
// supported text/code file as app-owned knowledge plus app-owned chunks.
func (s *knowledgeService) SyncCodeSource(
	ctx context.Context, kbID string, req *types.CodeSourceRequest,
) (*types.CodeSourceSyncResult, error) {
	if req == nil {
		return nil, werrors.NewValidationError("请求内容不能为空")
	}
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, err
	}
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		return nil, werrors.NewUnauthorizedError("Authentication required")
	}

	maxFiles := req.MaxFiles
	if maxFiles <= 0 || maxFiles > codeSourceMaxFiles {
		maxFiles = codeSourceMaxFiles
	}

	root, commit, sourceKey, err := prepareCodeSource(ctx, req)
	if err != nil {
		return nil, err
	}

	files, skipped, err := scanCodeFiles(root, req.Ignore, maxFiles)
	if err != nil {
		return nil, err
	}

	result := &types.CodeSourceSyncResult{
		SourceRoot: root,
		Commit:     commit,
		Skipped:    skipped,
	}
	existing, _ := s.repo.ListKnowledgeByKnowledgeBaseID(ctx, tenantID, kbID)
	sourcePrefix := "code://" + sourceKey + "/"
	now := time.Now()
	createdBy, _ := types.UserIDFromContext(ctx)
	currentIDs := make(map[string]struct{}, len(files))

	for _, file := range files {
		chunks := chunkCodeFile(file)
		if len(chunks) == 0 {
			result.Skipped++
			continue
		}
		knowledgeID := codeKnowledgeID(kbID, sourceKey, file.RelPath)
		currentIDs[knowledgeID] = struct{}{}

		knowledge := &types.Knowledge{
			ID:               knowledgeID,
			TenantID:         tenantID,
			KnowledgeBaseID:  kbID,
			Type:             types.KnowledgeTypeCode,
			Title:            file.RelPath,
			Description:      codeFileSummary(file, chunks),
			Source:           sourcePrefix + file.RelPath,
			Channel:          "code",
			CreatedBy:        createdBy,
			ParseStatus:      types.ParseStatusCompleted,
			EnableStatus:     "enabled",
			EmbeddingModelID: kb.EmbeddingModelID,
			FileName:         filepath.Base(file.RelPath),
			FolderPath:       filepath.Dir(file.RelPath),
			FileType:         strings.TrimPrefix(strings.ToLower(filepath.Ext(file.RelPath)), "."),
			FileSize:         int64(len(file.Content)),
			FileHash:         file.Hash,
			FilePath:         file.AbsPath,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if knowledge.FolderPath == "." {
			knowledge.FolderPath = ""
		}

		prev, prevErr := s.repo.GetKnowledgeByID(ctx, tenantID, knowledgeID)
		if prevErr == nil && prev != nil {
			// Unchanged files keep their existing chunks. A prior sync may
			// have created the knowledge row and then failed while writing
			// chunks, so an empty chunk set still forces a rebuild.
			existingChunks, _ := s.chunkRepo.ListChunksByKnowledgeID(ctx, tenantID, knowledgeID)
			if prev.FileHash == file.Hash && len(existingChunks) > 0 {
				continue
			}
			knowledge.CreatedAt = prev.CreatedAt
			if err := s.repo.UpdateKnowledge(ctx, knowledge); err != nil {
				return nil, fmt.Errorf("update code knowledge %s: %w", file.RelPath, err)
			}
			_ = s.chunkRepo.DeleteChunksByKnowledgeID(ctx, tenantID, knowledgeID)
		} else {
			// A prior sync may have removed this file, leaving a soft-deleted
			// tombstone that still owns the stable primary key.
			_ = s.repo.HardDeleteKnowledge(ctx, tenantID, knowledgeID)
			if err := s.repo.CreateKnowledge(ctx, knowledge); err != nil {
				return nil, fmt.Errorf("create code knowledge %s: %w", file.RelPath, err)
			}
		}

		appChunks := make([]*types.Chunk, 0, len(chunks))
		for index, chunk := range chunks {
			appChunks = append(appChunks, &types.Chunk{
				// Random IDs avoid primary-key collisions with soft-deleted
				// chunks from the previous revision of the same file.
				ID:              uuid.NewString(),
				TenantID:        tenantID,
				KnowledgeID:     knowledgeID,
				KnowledgeBaseID: kbID,
				Content:         chunk.Content,
				SourceContent:   chunk.Content,
				ChunkIndex:      index,
				IsEnabled:       true,
				Flags:           1,
				ChunkType:       types.ChunkTypeText,
				ContextHeader:   codeChunkHeader(file, chunk),
				IndexStatus:     "ready",
				Metadata:        codeChunkMetadata(sourceKey, commit, file, chunk),
				StartAt:         chunk.StartLine,
				EndAt:           chunk.EndLine,
			})
		}
		if err := s.chunkRepo.CreateChunks(ctx, appChunks); err != nil {
			return nil, fmt.Errorf("create code chunks %s: %w", file.RelPath, err)
		}
		result.Files++
		result.Chunks += len(appChunks)

		if s.ragEngine != nil {
			pushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Minute)
			markdown := codeFileMarkdown(file, chunks)
			pushErr := s.ragEngine.IngestDocument(
				pushCtx, kbID, knowledgeID, knowledge.FileName, knowledge.FileType,
				markdown, toEngineIngestChunks(appChunks), nil,
			)
			cancel()
			if pushErr != nil {
				logger.Warnf(ctx, "Push code knowledge %s to RAG failed: %v", knowledgeID, pushErr)
				_ = s.repo.UpdateEngineSyncStatus(ctx, tenantID, knowledgeID, "failed", pushErr.Error(), &now)
			}
		}
	}

	for _, item := range existing {
		if item == nil || !strings.HasPrefix(item.Source, sourcePrefix) {
			continue
		}
		if _, ok := currentIDs[item.ID]; ok {
			continue
		}
		if err := s.deleteCodeKnowledgeEntry(ctx, tenantID, item.ID); err == nil {
			result.Deleted++
		} else {
			logger.Warnf(ctx, "Delete stale code knowledge %s failed: %v", item.ID, err)
		}
	}

	logger.Infof(ctx, "[code-source] %s files=%d chunks=%d skipped=%d deleted=%d commit=%s",
		sourceKey, result.Files, result.Chunks, result.Skipped, result.Deleted, result.Commit)
	return result, nil
}

// codeKnowledgeID is the stable ID of one synced code file. The knowledge base
// is part of the key so the same source can be indexed into separate KBs.
func codeKnowledgeID(kbID, sourceKey, relPath string) string {
	return uuid.NewSHA1(
		uuid.NameSpaceURL, []byte("wellread-code:"+kbID+":"+sourceKey+":"+relPath),
	).String()
}

// deleteCodeKnowledgeEntry purges one synced code file: chunks, the engine
// document, and the knowledge row itself. Synced entries use stable IDs, so a
// soft-deleted tombstone would block the next sync of the same file; the row
// is hard-deleted instead of leaving a tombstone. The source file on disk is
// not touched (local mode may point at a user-owned directory).
func (s *knowledgeService) deleteCodeKnowledgeEntry(ctx context.Context, tenantID uint64, id string) error {
	if err := s.chunkRepo.DeleteChunksByKnowledgeID(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.repo.DeleteKnowledge(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.repo.HardDeleteKnowledge(ctx, tenantID, id); err != nil {
		return err
	}
	s.deleteFromEngine(ctx, id)
	return nil
}

func prepareCodeSource(ctx context.Context, req *types.CodeSourceRequest) (root, commit, sourceKey string, err error) {
	switch req.Kind {
	case types.CodeSourceLocal:
		root = strings.TrimSpace(req.Path)
		if root == "" {
			return "", "", "", werrors.NewValidationError("本地路径不能为空")
		}
		root, err = filepath.Abs(root)
		if err != nil {
			return "", "", "", err
		}
		info, statErr := os.Stat(root)
		if statErr != nil || !info.IsDir() {
			return "", "", "", werrors.NewValidationError("本地路径不存在或不是目录")
		}
		sum := sha256.Sum256([]byte("local:" + root))
		sourceKey = "local-" + hex.EncodeToString(sum[:8])
		return root, "", sourceKey, nil
	case types.CodeSourceGit:
		url := strings.TrimSpace(req.URL)
		if url == "" {
			return "", "", "", werrors.NewValidationError("Git 仓库地址不能为空")
		}
		branch := strings.TrimSpace(req.Branch)
		if branch == "" {
			branch = "main"
		}
		sum := sha256.Sum256([]byte("git:" + url + "#" + branch))
		sourceKey = "git-" + hex.EncodeToString(sum[:8])
		base := codeSourceRoot()
		root = filepath.Join(base, sourceKey)
		if err := os.MkdirAll(base, 0o755); err != nil {
			return "", "", "", err
		}
		_ = os.RemoveAll(root)
		args := []string{"clone", "--depth", "1", "--branch", branch, "--", url, root}
		if out, cmdErr := exec.CommandContext(ctx, "git", args...).CombinedOutput(); cmdErr != nil {
			return "", "", "", fmt.Errorf("git clone failed: %s: %w", strings.TrimSpace(string(out)), cmdErr)
		}
		return root, strings.TrimSpace(runGit(ctx, root, "rev-parse", "HEAD")), sourceKey, nil
	default:
		return "", "", "", werrors.NewValidationError("kind 仅支持 local 或 git")
	}
}

func codeSourceRoot() string {
	if configured := strings.TrimSpace(os.Getenv("CODE_SOURCE_DIR")); configured != "" {
		return configured
	}
	if err := os.MkdirAll(defaultCodeSourceDir, 0o755); err == nil {
		return defaultCodeSourceDir
	}
	return filepath.Join(os.TempDir(), "wellread-code-sources")
}

func runGit(ctx context.Context, dir string, args ...string) string {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func scanCodeFiles(root string, ignore []string, maxFiles int) ([]codeFile, int, error) {
	var files []codeFile
	skipped := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || path == root {
			return nil
		}
		name := entry.Name()
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if _, ignored := codeSourceIgnoreDirs[strings.ToLower(name)]; ignored || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			if matchesIgnore(rel, ignore) {
				return filepath.SkipDir
			}
			return nil
		}
		if len(files) >= maxFiles {
			skipped++
			return nil
		}
		if matchesIgnore(rel, ignore) || isSensitiveCodePath(rel) {
			skipped++
			return nil
		}
		lang, ok := codeExtensions[strings.ToLower(filepath.Ext(name))]
		if !ok {
			skipped++
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.Size() > codeSourceMaxFileBytes {
			skipped++
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 || !utf8.Valid(data) || bytesLookBinary(data) {
			skipped++
			return nil
		}
		sum := sha256.Sum256(data)
		files = append(files, codeFile{
			RelPath: rel, AbsPath: path, Language: lang,
			Content: string(data), Hash: hex.EncodeToString(sum[:]),
		})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	return files, skipped, err
}

func matchesIgnore(rel string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		pattern = filepath.ToSlash(pattern)
		if strings.HasPrefix(rel, strings.TrimSuffix(pattern, "/")+"/") {
			return true
		}
		if ok, _ := filepath.Match(pattern, rel); ok {
			return true
		}
	}
	return false
}

func isSensitiveCodePath(rel string) bool {
	base := strings.ToLower(filepath.Base(rel))
	if strings.HasPrefix(base, ".env") || strings.HasSuffix(base, ".pem") ||
		strings.HasSuffix(base, ".key") || strings.HasSuffix(base, ".p12") ||
		strings.HasSuffix(base, ".pfx") || base == "id_rsa" || base == "id_ed25519" {
		return true
	}
	return strings.Contains(base, "secret") || strings.Contains(base, "credential")
}

func bytesLookBinary(data []byte) bool {
	limit := len(data)
	if limit > 8192 {
		limit = 8192
	}
	for _, b := range data[:limit] {
		if b == 0 {
			return true
		}
	}
	return false
}

func chunkCodeFile(file codeFile) []codeChunk {
	lines := strings.Split(strings.ReplaceAll(file.Content, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(file.Content) == "" {
		return nil
	}
	if file.Language == "markdown" || file.Language == "text" || file.Language == "yaml" ||
		file.Language == "json" || file.Language == "toml" || file.Language == "ini" {
		return chunkSpanLines(lines, markdownHeading)
	}
	if patterns := symbolPatterns[file.Language]; len(patterns) > 0 {
		return chunkBySymbols(lines, patterns)
	}
	return chunkSpanLines(lines, nil)
}

func chunkBySymbols(lines []string, patterns []*regexp.Regexp) []codeChunk {
	var chunks []codeChunk
	start := 0
	symbol := ""
	for i := 0; i < len(lines); i++ {
		name := matchSymbol(lines[i], patterns)
		if name == "" {
			continue
		}
		if i > start && strings.TrimSpace(strings.Join(lines[start:i], "")) != "" {
			chunks = append(chunks, makeCodeChunk(lines, start, i-1, symbol))
		}
		start = i
		symbol = name
	}
	if start < len(lines) {
		chunks = append(chunks, makeCodeChunk(lines, start, len(lines)-1, symbol))
	}
	if len(chunks) == 0 {
		return splitOversizedCodeChunks([]codeChunk{makeCodeChunk(lines, 0, len(lines)-1, "")})
	}
	return splitOversizedCodeChunks(chunks)
}

func matchSymbol(line string, patterns []*regexp.Regexp) string {
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(line)
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			return match[1]
		}
	}
	return ""
}

func chunkSpanLines(lines []string, heading *regexp.Regexp) []codeChunk {
	var chunks []codeChunk
	start := 0
	symbol := ""
	for i := 0; i < len(lines); i++ {
		if heading != nil && heading.MatchString(lines[i]) {
			if i > start && strings.TrimSpace(strings.Join(lines[start:i], "")) != "" {
				chunks = append(chunks, makeCodeChunk(lines, start, i-1, symbol))
			}
			start = i
			symbol = strings.TrimSpace(strings.TrimLeft(lines[i], "# "))
		}
	}
	if start < len(lines) {
		chunks = append(chunks, makeCodeChunk(lines, start, len(lines)-1, symbol))
	}
	if len(chunks) == 0 {
		chunks = append(chunks, makeCodeChunk(lines, 0, len(lines)-1, ""))
	}
	return splitOversizedCodeChunks(chunks)
}

func makeCodeChunk(lines []string, start, end int, symbol string) codeChunk {
	return codeChunk{
		Symbol: symbol, SymbolType: "block", StartLine: start + 1, EndLine: end + 1,
		Content: strings.Join(lines[start:end+1], "\n"),
	}
}

func splitOversizedCodeChunks(chunks []codeChunk) []codeChunk {
	var out []codeChunk
	for _, chunk := range chunks {
		lines := strings.Split(chunk.Content, "\n")
		for len(lines) > codeChunkMaxLines || len(strings.Join(lines, "\n")) > codeChunkMaxChars {
			end := codeChunkMaxLines
			if len(lines) < end {
				end = len(lines)
			}
			out = append(out, codeChunk{
				Symbol: chunk.Symbol, SymbolType: chunk.SymbolType,
				StartLine: chunk.StartLine, EndLine: chunk.StartLine + end - 1,
				Content: strings.Join(lines[:end], "\n"),
			})
			chunk.StartLine += end
			lines = lines[end:]
		}
		if len(lines) > 0 {
			chunk.Content = strings.Join(lines, "\n")
			chunk.EndLine = chunk.StartLine + len(lines) - 1
			out = append(out, chunk)
		}
	}
	return out
}

func codeChunkHeader(file codeFile, chunk codeChunk) string {
	header := fmt.Sprintf("%s:%d-%d", file.RelPath, chunk.StartLine, chunk.EndLine)
	if chunk.Symbol != "" {
		header += " · " + chunk.Symbol
	}
	return header
}

func codeChunkMetadata(sourceKey, commit string, file codeFile, chunk codeChunk) types.JSON {
	payload := map[string]any{
		"code": map[string]any{
			"source_key": sourceKey,
			"commit":     commit,
			"path":       file.RelPath,
			"language":   file.Language,
			"symbol":     chunk.Symbol,
			"start_line": chunk.StartLine,
			"end_line":   chunk.EndLine,
		},
	}
	raw, _ := json.Marshal(payload)
	return types.JSON(raw)
}

func codeFileSummary(file codeFile, chunks []codeChunk) string {
	symbols := make([]string, 0, 8)
	seen := map[string]struct{}{}
	for _, chunk := range chunks {
		if chunk.Symbol == "" {
			continue
		}
		if _, ok := seen[chunk.Symbol]; ok {
			continue
		}
		seen[chunk.Symbol] = struct{}{}
		symbols = append(symbols, chunk.Symbol)
		if len(symbols) == 8 {
			break
		}
	}
	if len(symbols) == 0 {
		return file.Language
	}
	return file.Language + " · " + strings.Join(symbols, ", ")
}

func codeFileMarkdown(file codeFile, chunks []codeChunk) string {
	var builder strings.Builder
	builder.WriteString("# ")
	builder.WriteString(file.RelPath)
	builder.WriteString("\n\n")
	for _, chunk := range chunks {
		builder.WriteString(codeChunkHeader(file, chunk))
		builder.WriteString("\n```")
		builder.WriteString(file.Language)
		builder.WriteString("\n")
		builder.WriteString(chunk.Content)
		builder.WriteString("\n```\n\n")
	}
	return builder.String()
}
