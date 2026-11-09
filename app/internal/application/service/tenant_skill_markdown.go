package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"gaze/internal/agent/skills"
	"gaze/internal/types"
)

const (
	// maxSkillMarkdownBytes bounds a single uploaded SKILL.md file.
	maxSkillMarkdownBytes    = 1 << 20 // 1 MiB
	maxSkillNameRunes        = 100
	maxSkillDescriptionRunes = 500
	maxSkillVersionRunes     = 64
)

// ValidateSkillDefinition applies the shared field limits for uploaded and
// hand-written skills. Errors wrap ErrSkillBundleInvalid so the handler maps
// them to 400 with a readable message.
func ValidateSkillDefinition(name, description, version, instructions string) error {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	version = strings.TrimSpace(version)
	instructions = strings.TrimSpace(instructions)

	if name == "" {
		return fmt.Errorf("%w: name is required", ErrSkillBundleInvalid)
	}
	if utf8.RuneCountInString(name) > maxSkillNameRunes {
		return fmt.Errorf("%w: name exceeds %d characters", ErrSkillBundleInvalid, maxSkillNameRunes)
	}
	if description == "" {
		return fmt.Errorf("%w: description is required", ErrSkillBundleInvalid)
	}
	if utf8.RuneCountInString(description) > maxSkillDescriptionRunes {
		return fmt.Errorf("%w: description exceeds %d characters", ErrSkillBundleInvalid, maxSkillDescriptionRunes)
	}
	if utf8.RuneCountInString(version) > maxSkillVersionRunes {
		return fmt.Errorf("%w: version exceeds %d characters", ErrSkillBundleInvalid, maxSkillVersionRunes)
	}
	if instructions == "" {
		return fmt.Errorf("%w: instructions are required", ErrSkillBundleInvalid)
	}
	if utf8.RuneCountInString(instructions) > maxSkillInstructionsRunes {
		return fmt.Errorf("%w: instructions exceed %d characters", ErrSkillBundleInvalid, maxSkillInstructionsRunes)
	}
	return nil
}

// RegisterCatalogFromMarkdown registers one uploaded SKILL.md: frontmatter is
// mandatory, instructions are the body. The archive stored for the catalog is
// a deterministic single-file zip so the existing bundle storage keeps working.
func (s *TenantSkillService) RegisterCatalogFromMarkdown(
	ctx context.Context, tenantID uint64, content []byte,
) (*types.TenantSkillCatalogEntity, error) {
	if len(content) == 0 {
		return nil, fmt.Errorf("%w: SKILL.md is empty", ErrSkillBundleInvalid)
	}
	if len(content) > maxSkillMarkdownBytes {
		return nil, fmt.Errorf("%w: SKILL.md exceeds 1 MiB", ErrSkillBundleInvalid)
	}
	parsed, err := skills.ParseSkillFile(string(content))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSkillBundleInvalid, err)
	}
	if err := ValidateSkillDefinition(parsed.Name, parsed.Description, "", parsed.Instructions); err != nil {
		return nil, err
	}
	files := map[string][]byte{"SKILL.md": content}
	archive, err := zipSkillFiles(files)
	if err != nil {
		return nil, err
	}
	bundle := &SkillBundle{
		Name:         parsed.Name,
		Description:  parsed.Description,
		Instructions: parsed.Instructions,
		SHA256:       skillArchiveSHA256(archive),
		Files:        files,
	}
	return s.upsertCatalogFromBundle(ctx, tenantID, bundle, archive, true)
}

// RegisterCatalogFromContent creates a skill from the online authoring form.
func (s *TenantSkillService) RegisterCatalogFromContent(
	ctx context.Context, tenantID uint64, name, description, version, instructions string,
) (*types.TenantSkillCatalogEntity, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	version = strings.TrimSpace(version)
	instructions = strings.TrimSpace(instructions)
	if err := ValidateSkillDefinition(name, description, version, instructions); err != nil {
		return nil, err
	}
	frontmatter, err := yaml.Marshal(struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Version     string `yaml:"version,omitempty"`
	}{Name: name, Description: description, Version: version})
	if err != nil {
		return nil, fmt.Errorf("%w: build frontmatter: %v", ErrSkillBundleInvalid, err)
	}
	markdown := "---\n" + string(frontmatter) + "---\n\n" + instructions + "\n"
	return s.RegisterCatalogFromMarkdown(ctx, tenantID, []byte(markdown))
}
