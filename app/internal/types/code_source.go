package types

// CodeSourceKind identifies where a code project comes from.
type CodeSourceKind string

const (
	CodeSourceLocal CodeSourceKind = "local"
	CodeSourceGit   CodeSourceKind = "git"
)

// CodeSourceRequest asks the app to scan a local directory or clone a Git
// repository and index its text/code files into one knowledge base.
type CodeSourceRequest struct {
	Kind     CodeSourceKind `json:"kind" binding:"required"`
	Path     string         `json:"path"`
	URL      string         `json:"url"`
	Branch   string         `json:"branch"`
	Ignore   []string       `json:"ignore"`
	MaxFiles int            `json:"max_files"`
}

// CodeSourceSyncResult reports one sync pass.
type CodeSourceSyncResult struct {
	SourceRoot string `json:"source_root"`
	Commit     string `json:"commit,omitempty"`
	Files      int    `json:"files"`
	Chunks     int    `json:"chunks"`
	Skipped    int    `json:"skipped"`
	Deleted    int    `json:"deleted"`
}
