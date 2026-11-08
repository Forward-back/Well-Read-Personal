package agent

import (
	"fmt"
	"strings"
	"time"

	"gaze/internal/agent/skills"
	"gaze/internal/config"
	"gaze/internal/types"
)

// formatFileSize formats file size in human-readable format
func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	if size < KB {
		return fmt.Sprintf("%d B", size)
	} else if size < MB {
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	} else if size < GB {
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	}
	return fmt.Sprintf("%.2f GB", float64(size)/GB)
}

// formatDocSummary cleans and truncates document summaries for table display
func formatDocSummary(summary string, maxLen int) string {
	cleaned := strings.TrimSpace(summary)
	if cleaned == "" {
		return "-"
	}
	cleaned = strings.ReplaceAll(cleaned, "\n", " ")
	cleaned = strings.ReplaceAll(cleaned, "\r", " ")
	cleaned = strings.Join(strings.Fields(cleaned), " ")

	runes := []rune(cleaned)
	if len(runes) <= maxLen {
		return cleaned
	}
	return strings.TrimSpace(string(runes[:maxLen])) + "..."
}

// RecentDocInfo contains brief information about a recently added document
type RecentDocInfo struct {
	ChunkID             string
	KnowledgeBaseID     string
	KnowledgeID         string
	Title               string
	Description         string
	FileName            string
	FileSize            int64
	Type                string
	CreatedAt           string // Formatted time string
	FAQStandardQuestion string
	FAQSimilarQuestions []string
	FAQAnswers          []string
}

// SelectedDocumentInfo contains summary information about a user-selected document (via @ mention).
// Injected into the user message runtime_context (pinned_documents); content is fetched via tools.
type SelectedDocumentInfo struct {
	KnowledgeID     string // Knowledge ID
	KnowledgeBaseID string // Knowledge base ID
	Title           string // Document title
	FileName        string // Original file name
	FileType        string // File type (pdf, docx, etc.)
}

// PinnedMCPServiceInfo describes an MCP service explicitly @mentioned for this turn.
type PinnedMCPServiceInfo struct {
	Discoverable bool // Available through the scoped MCP directory.
	ID           string
	Name         string
	Description  string
	ToolNames    []string // Registered tool.function names for this service (mcp_{service}_{tool})
}

// PinnedSkillInfo describes a skill explicitly @mentioned for this turn.
type PinnedSkillInfo struct {
	Name        string
	Description string
}

// KnowledgeBaseInfo contains essential information about a knowledge base for agent prompt
type KnowledgeBaseInfo struct {
	ID          string
	Name        string
	Type        string // Knowledge base type: "document" or "faq"
	Description string
	DocCount    int
	// Capabilities lists the retrieval surfaces this KB exposes. Any subset of
	// {"gaze", "chunks"}. "chunks" is present when the KB has vector and/or
	// keyword (BM25) indexing enabled. This is the *deterministic* source of
	// truth the agent should consult before picking a retrieval strategy —
	// significantly more reliable than running probing searches.
	Capabilities []string
	RecentDocs   []RecentDocInfo // Recently added documents (up to 10)
}

// PlaceholderDefinition defines a placeholder exposed to UI/configuration
// Deprecated: Use types.PromptPlaceholder instead
type PlaceholderDefinition struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// AvailablePlaceholders lists all supported prompt placeholders for UI hints
// This returns agent mode specific placeholders
func AvailablePlaceholders() []PlaceholderDefinition {
	// Use centralized placeholder definitions from types package
	placeholders := types.PlaceholdersByField(types.PromptFieldAgentSystemPrompt)
	result := make([]PlaceholderDefinition, len(placeholders))
	for i, p := range placeholders {
		result[i] = PlaceholderDefinition{
			Name:        p.Name,
			Label:       p.Label,
			Description: p.Description,
		}
	}
	return result
}

// formatKnowledgeBaseList formats knowledge base information as XML for the prompt
func formatKnowledgeBaseList(kbInfos []*KnowledgeBaseInfo) string {
	if len(kbInfos) == 0 {
		return "<knowledge_bases />"
	}

	var b strings.Builder
	b.WriteString("<knowledge_bases>\n")
	for _, kb := range kbInfos {
		kbType := kb.Type
		if kbType == "" {
			kbType = "document"
		}
		capsAttr := ""
		if len(kb.Capabilities) > 0 {
			capsAttr = fmt.Sprintf(" capabilities=\"%s\"", strings.Join(kb.Capabilities, ","))
		}
		b.WriteString(fmt.Sprintf("<knowledge_base id=\"%s\" name=\"%s\" type=\"%s\" doc_count=\"%d\"%s>\n",
			kb.ID, kb.Name, kbType, kb.DocCount, capsAttr))
		if kb.Description != "" {
			b.WriteString(fmt.Sprintf("<description>%s</description>\n", kb.Description))
		}

		if len(kb.RecentDocs) > 0 {
			if kbType == "faq" {
				b.WriteString("<faq_entries>\n")
				for j, doc := range kb.RecentDocs {
					if j >= 10 {
						break
					}
					question := doc.FAQStandardQuestion
					if question == "" {
						question = doc.FileName
					}
					b.WriteString(fmt.Sprintf("<faq chunk_id=\"%s\" knowledge_id=\"%s\" created_at=\"%s\">\n",
						doc.ChunkID, doc.KnowledgeID, doc.CreatedAt))
					b.WriteString(fmt.Sprintf("<question>%s</question>\n", question))
					if len(doc.FAQAnswers) > 0 {
						for _, ans := range doc.FAQAnswers {
							b.WriteString(fmt.Sprintf("<answer>%s</answer>\n", ans))
						}
					}
					b.WriteString("</faq>\n")
				}
				b.WriteString("</faq_entries>\n")
			} else {
				b.WriteString("<recent_documents>\n")
				for j, doc := range kb.RecentDocs {
					if j >= 2 {
						break
					}
					docName := doc.Title
					if docName == "" {
						docName = doc.FileName
					}
					fileSize := formatFileSize(doc.FileSize)
					b.WriteString(fmt.Sprintf("<document knowledge_id=\"%s\" type=\"%s\" file_size=\"%s\" created_at=\"%s\">\n",
						doc.KnowledgeID, doc.Type, fileSize, doc.CreatedAt))
					b.WriteString(fmt.Sprintf("<name>%s</name>\n", docName))
					if doc.Description != "" {
						summary := formatDocSummary(doc.Description, 120)
						b.WriteString(fmt.Sprintf("<summary>%s</summary>\n", summary))
					}
					b.WriteString("</document>\n")
				}
				b.WriteString("</recent_documents>\n")
			}
		}
		b.WriteString("</knowledge_base>\n")
	}
	b.WriteString("</knowledge_bases>")
	return b.String()
}

// renderPromptPlaceholders renders placeholders in the prompt template.
//
// Supported placeholders:
//   - {{knowledge_bases}} - Historically expanded to the full bound-KB XML
//     block. Since that block now lives in the user message's
//     `<runtime_context>` (see observe.buildRuntimeContextBlock), the
//     placeholder is expanded to a short pointer so legacy / custom
//     templates that still reference `{{knowledge_bases}}` degrade
//     gracefully instead of dumping the detail twice.
//   - `<must_use>` is NOT a placeholder — when the user @mentions MCP/Skill,
//     observe.buildMustUseBlock injects it as a sibling block in the user
//     message; system prompts document it by convention (see agent_system_prompt.yaml).
func renderPromptPlaceholders(template string, knowledgeBases []*KnowledgeBaseInfo) string {
	result := template

	if strings.Contains(result, "{{knowledge_bases}}") {
		var replacement string
		if len(knowledgeBases) == 0 {
			replacement = "（本会话未绑定知识库）"
		} else {
			replacement = "（当前绑定知识库列表与能力见用户消息 `<runtime_context>` 中的 `<bound_knowledge_bases>`）"
		}
		result = strings.ReplaceAll(result, "{{knowledge_bases}}", replacement)
	}

	return result
}

// formatSkillsMetadata formats skills metadata for the system prompt (Level 1 - Progressive Disclosure)
// This is a lightweight representation that only includes skill name and description
func formatSkillsMetadata(skillsMetadata []*skills.SkillMetadata, shellExecEnabled bool) string {
	if len(skillsMetadata) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n可用技能：应用相关技能前，先用 read_file 读取其列出的 SKILL.md 资源。仅在需要时加载其他文件；返回的文件列表已经标明随包脚本。\n")
	for _, skill := range skillsMetadata {
		if skill != nil {
			fmt.Fprintf(&b, "- %s: %s (read_file path=%q)\n", skill.Name, skill.Description, "skill://"+skill.Name+"/SKILL.md")
		}
	}
	return b.String()
}

// formatToolGuidance uses the actual registry, so disabled capabilities never
// leak into the runtime instructions. Mechanics and limits live in tool schemas.
func formatToolGuidance(names []string) string {
	if len(names) == 0 {
		return ""
	}
	has := func(name string) bool {
		for _, n := range names {
			if n == name {
				return true
			}
		}
		return false
	}
	var b strings.Builder
	b.WriteString("\n\n工具执行：只使用本轮提供的工具。在内部计划；只有规划工具确有帮助时才使用。已知路径直接读取。独立读取可批量进行，有依赖的操作按顺序执行。声称完成前先检查结果。\n")
	b.WriteString("长时间操作如提供文档化的异步模式，优先使用。用返回的任务 ID 按建议间隔等待或轮询，获取完成结果；超时后先检查已有任务再重新提交。\n")
	b.WriteString("失败时根据报告的原因修正输入或环境；只有相关条件发生变化后才重试。权限、策略或缺少配置类失败不能靠换工具解决；本轮无法修正时如实报告具体阻塞。\n")
	if has("read_file") {
		b.WriteString("工作区文件、已保存的 web:// 页面和列出的 skill:// 资源使用 read_file 读取。" +
			"旧说明中的 read_skill(skill_name, file_path) 对应 read_file(path=skill://<name>/<file_path 或 SKILL.md>)，read_sandbox_file 对应 read_file。\n")
	}
	if has("shell_exec") || has("write_sandbox_file") {
		b.WriteString("会话工作区为 /workspace。上传的原始文件保存在 /workspace/input。" +
			"/workspace/output 是唯一会被收集用于下载的目录，只放最终交付物；" +
			"草稿和中间文件放在 /workspace 下的其他目录。" +
			"每次命令都从指定的工作目录开始。" +
			"文件与已安装依赖在会话内保留。\n")
		b.WriteString(sandboxArtifactReferenceGuidance())
	}
	if has("shell_exec") && has("read_file") {
		b.WriteString("列出的技能，用 shell_exec(skill_name=..., command=...) 运行随技能脚本或自己的脚本。" +
			"它会选择已安装技能运行时或准备宿主技能资源，并应用限定凭据；" +
			"随包文件使用 $GAZE_SKILL_DIR。\n")
		b.WriteString("旧说明中的 execute_skill_script(skill_name, script_path, ...) 对应 shell_exec(skill_name=..., command=...)。\n")
	}

	return b.String()
}

// sandboxArtifactReferenceGuidance tells the model how to point at a file it
// generated in the sandbox from its final answer.
//
// Without this, models improvise a Markdown image with the bare file name
// (`![评分](市场画像评分.html)`), which the browser cannot resolve — the answer
// renders a broken image icon. The `sandbox:` prefix makes the intent explicit
// so the server can bind the name to the artifact index it hands the client.
func sandboxArtifactReferenceGuidance() string {
	var builder strings.Builder
	builder.WriteString("  - 最终答案需要引用关键生成交付物时，使用 ")
	builder.WriteString("`![描述](sandbox:<文件名>)`，文件名必须精确且不能带目录路径\n")
	builder.WriteString("    - 图片会内联渲染；图表、表格和文档会渲染为可点击预览的卡片\n")
	builder.WriteString("    - 不得直接引用 sandbox 路径（`/workspace/output/...`）或裸文件名，浏览器无法解析\n")
	builder.WriteString("    - 输出文件名尽量不含空格和括号，避免引用歧义\n")
	return builder.String()
}

// renderPromptPlaceholdersWithStatus renders placeholders including web search status
// Supported placeholders:
//   - {{knowledge_bases}}
//   - {{web_search_status}} -> "Enabled" or "Disabled"
//   - {{current_time}} -> current time string
//   - {{language}} -> user language name (e.g. "Chinese (Simplified)", "English")
//   - {{skills}} -> formatted skills metadata (if any)
func renderPromptPlaceholdersWithStatus(
	template string,
	knowledgeBases []*KnowledgeBaseInfo,
	webSearchEnabled bool,
	currentTime string,
	language string,
) string {
	// Knowledge bases need special formatting, so handle it first
	result := renderPromptPlaceholders(template, knowledgeBases)

	status := "未启用"
	if webSearchEnabled {
		status = "已启用"
	}

	result = types.RenderPromptPlaceholders(result, types.PlaceholderValues{
		"web_search_status": status,
		"current_time":      currentTime,
		"language":          language,
		"skills":            "", // Remove {{skills}} placeholder; skills are appended separately if present
	})
	return result
}

// BuildSystemPromptOptions contains optional parameters for BuildSystemPrompt
type BuildSystemPromptOptions struct {
	SelectedTools    []string
	SkillsMetadata   []*skills.SkillMetadata
	ShellExecEnabled bool
	Language         string         // User language name for {{language}} placeholder (e.g. "Chinese (Simplified)")
	Config           *config.Config // Config for reading prompt templates; nil falls back to hardcoded defaults
}

// BuildSystemPrompt builds the progressive RAG system prompt
// This is the main function to use - it uses a unified template with dynamic web search status
func BuildSystemPrompt(
	knowledgeBases []*KnowledgeBaseInfo,
	webSearchEnabled bool,
	systemPromptTemplate ...string,
) string {
	return BuildSystemPromptWithOptions(knowledgeBases, webSearchEnabled, nil, systemPromptTemplate...)
}

// BuildSystemPromptWithOptions builds the system prompt with additional options like skills
func BuildSystemPromptWithOptions(
	knowledgeBases []*KnowledgeBaseInfo,
	webSearchEnabled bool,
	options *BuildSystemPromptOptions,
	systemPromptTemplate ...string,
) string {
	var basePrompt string
	var template string

	// Determine template to use
	if len(systemPromptTemplate) > 0 && systemPromptTemplate[0] != "" {
		template = systemPromptTemplate[0]
	} else if len(knowledgeBases) == 0 {
		var cfg *config.Config
		if options != nil {
			cfg = options.Config
		}
		template = GetPureAgentSystemPrompt(cfg)
	} else {
		var cfg *config.Config
		if options != nil {
			cfg = options.Config
		}
		template = GetProgressiveRAGSystemPrompt(cfg)
	}

	currentTime := time.Now().Format("2006-01-02")
	language := ""
	if options != nil {
		language = options.Language
	}
	basePrompt = renderPromptPlaceholdersWithStatus(template, knowledgeBases, webSearchEnabled, currentTime, language)
	basePrompt += "\n\n" + steerGuidance

	if options != nil {
		basePrompt += formatGroundingGuidance(options.SelectedTools)
		basePrompt += formatToolGuidance(options.SelectedTools)
	} else {
		basePrompt += formatGroundingGuidance(nil)
	}

	// Append skills metadata if available (Level 1 - Progressive Disclosure)
	if options != nil && len(options.SkillsMetadata) > 0 {
		basePrompt += formatSkillsMetadata(options.SkillsMetadata, options.ShellExecEnabled)
	}

	return basePrompt
}

// Apply to custom prompts too: mid-run delivery is a harness capability.
const steerGuidance = "<steering_guidance>\n" +
	"<steer_message> 中的消息用于指导进行中的任务。结合上下文执行；合适时简短回应，然后继续未完成的工作。" +
	"除非明确变更，保留未完成目标、已接受的约束与有用的工具结果。" +
	"仅确认收到指导并不等于完成任务。遵循明确的取消或替换请求。" +
	"隐藏投递标签；未加标签的后续请求是普通用户消息。\n</steering_guidance>"

// GetPureAgentSystemPrompt returns the Pure Agent system prompt from config templates.
// The template must be defined in config/prompt_templates/agent_system_prompt.yaml
// with mode "pure". Returns empty string if config is nil or template not found.
func GetPureAgentSystemPrompt(cfg *config.Config) string {
	if prompt := cfg.AgentSystemPromptOverride(); prompt != "" {
		return prompt
	}
	if cfg != nil && cfg.PromptTemplates != nil {
		if t := config.DefaultTemplateByMode(cfg.PromptTemplates.AgentSystemPrompt, "pure"); t != nil && t.Content != "" {
			return t.Content
		}
	}
	return ""
}

// GetProgressiveRAGSystemPrompt returns the Progressive RAG Agent system prompt from config templates.
// The template must be defined in config/prompt_templates/agent_system_prompt.yaml
// with mode "rag". Returns empty string if config is nil or template not found.
func GetProgressiveRAGSystemPrompt(cfg *config.Config) string {
	if prompt := cfg.AgentSystemPromptOverride(); prompt != "" {
		return prompt
	}
	if cfg != nil && cfg.PromptTemplates != nil {
		if t := config.DefaultTemplateByMode(cfg.PromptTemplates.AgentSystemPrompt, "rag"); t != nil && t.Content != "" {
			return t.Content
		}
	}
	return ""
}
