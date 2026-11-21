package types

import "time"

// UserBackground is an uploaded shell background owned by one user.
// FilePath is relative to LOCAL_STORAGE_BASE_DIR and never exposed in JSON.
type UserBackground struct {
	ID          string    `json:"id"            gorm:"type:varchar(36);primaryKey"`
	UserID      string    `json:"-"             gorm:"type:varchar(36);index;not null"`
	DisplayName string    `json:"display_name"  gorm:"type:varchar(255);not null"`
	MimeType    string    `json:"mime_type"     gorm:"type:varchar(100);not null"`
	SizeBytes   int64     `json:"size_bytes"    gorm:"not null"`
	FilePath    string    `json:"-"             gorm:"type:text;not null"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName pins the table for GORM's pluralisation.
func (UserBackground) TableName() string { return "user_backgrounds" }

// BackgroundList is the read model returned to the settings page.
type BackgroundList struct {
	Choice         string            `json:"choice"`
	Uploads        []*UserBackground `json:"uploads"`
	MaxUploadBytes int64             `json:"max_upload_bytes"`
	MaxUploadCount int               `json:"max_upload_count"`
}

// Shell background choice values persisted on users.background_choice:
//   - "default"          grass background
//   - "builtin:<1..5>"   bundled wallpaper
//   - "upload:<uuid>"    row in user_backgrounds
const (
	BackgroundChoiceDefault = "default"
	BackgroundChoiceBuiltin = "builtin:"
	BackgroundChoiceUpload  = "upload:"
	BackgroundBuiltinCount  = 5
)
