package types

// MarkdownArchiveImportResult summarizes a zip import that turns every
// Markdown entry into its own knowledge record. Per-document failures do not
// abort the rest of the archive; callers render both lists.
type MarkdownArchiveImportResult struct {
	MarkdownCount  int                       `json:"markdown_count"`
	ImagesImported int                       `json:"images_imported"`
	ImagesSkipped  int                       `json:"images_skipped"`
	Documents      []MarkdownArchiveDocument `json:"documents"`
	Errors         []MarkdownArchiveError    `json:"errors,omitempty"`
}

// MarkdownArchiveDocument reports one Markdown entry that produced a
// knowledge record, including how many local images were imported or skipped.
type MarkdownArchiveDocument struct {
	Knowledge      *Knowledge `json:"knowledge"`
	FileName       string     `json:"file_name"`
	FolderPath     string     `json:"folder_path,omitempty"`
	ImagesImported int        `json:"images_imported"`
	ImagesSkipped  int        `json:"images_skipped"`
	SkippedReasons []string   `json:"skipped_reasons,omitempty"`
}

// MarkdownArchiveError reports one Markdown entry that failed to import.
type MarkdownArchiveError struct {
	Entry   string `json:"entry"`
	Message string `json:"message"`
}
