package agent

import (
	"context"
	"os"
	"strings"
	"testing"

	"gaze/internal/agent/tools"
	"gaze/internal/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestGroundingGuidanceSurvivesTemplateSelection(t *testing.T) {
	data, err := os.ReadFile("../../config/prompt_templates/agent_system_prompt.yaml")
	require.NoError(t, err)
	var file struct {
		Templates []config.PromptTemplate `yaml:"templates"`
	}
	require.NoError(t, yaml.Unmarshal(data, &file))
	cfg := &config.Config{PromptTemplates: &config.PromptTemplatesConfig{AgentSystemPrompt: file.Templates}}
	for _, tc := range []struct {
		name   string
		kbs    []*KnowledgeBaseInfo
		custom string
	}{
		{name: "pure"},
		{name: "rag", kbs: []*KnowledgeBaseInfo{{ID: "kb", Capabilities: []string{"chunks"}}}},
		{name: "saved custom", custom: "Create the requested slides using the selected skill."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prompt := BuildSystemPromptWithOptions(tc.kbs, false, &BuildSystemPromptOptions{Config: cfg}, tc.custom)
			require.Contains(t, prompt, "撰写实质性事实内容前")
			require.Contains(t, prompt, "演示文稿、报告、教程与技术说明")
			require.Contains(t, prompt, "阅读生成器说明或成功运行其脚本，不等于验证了主题内容")
			require.Contains(t, prompt, "对所提供内容的翻译或格式转换，不需要检索")
			require.Contains(t, prompt, "相关来源不可用或检索存在缺口时")
			if tc.custom != "" {
				require.True(t, strings.HasPrefix(prompt, tc.custom))
			}
		})
	}
	// The legacy builder cannot infer tools, but still needs the evidence policy.
	require.Contains(t, BuildSystemPrompt(nil, false, "Custom"), "内容依据")
}

func TestGroundingUsesRegistryInsteadOfConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name       string
		registered []string
		webFlag    bool
		want       []string
		absent     []string
	}{
		{"skill only with stale flags", []string{tools.ToolReadFile, tools.ToolShellExec}, true,
			nil, []string{"可用知识工具：", "web_search 可用", "web_fetch 可用"}},
		{"rag", []string{tools.ToolKnowledgeSearch, tools.ToolListKnowledgeChunks}, false,
			[]string{"可用知识工具：knowledge_search、list_knowledge_chunks"}, []string{"gaze_search", "web_search 可用"}},
		{"gaze", []string{tools.ToolGazeReadPage, tools.ToolGazeSearch}, false,
			[]string{"可用知识工具：gaze_search、gaze_read_page"}, []string{"knowledge_search", "web_search 可用"}},
		{"registered web", []string{tools.ToolWebSearch, tools.ToolWebFetch}, false,
			[]string{"web_search 可用", "web_fetch 可用"}, []string{"可用知识工具："}},
		{"no tools", nil, false, nil,
			[]string{"可用知识工具：", "web_search 可用", "web_fetch 可用"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := newTestEngine(t, &mockChat{})
			engine.config.WebSearchEnabled = tc.webFlag
			engine.config.AllowedTools = []string{tools.ToolKnowledgeSearch, tools.ToolWebSearch}
			engine.toolRegistry = tools.NewToolRegistry()
			for _, name := range tc.registered {
				engine.toolRegistry.RegisterTool(newCountingTool(name))
			}
			prompt := engine.buildSystemPrompt(context.Background())
			require.Contains(t, prompt, "内容依据")
			for _, want := range tc.want {
				require.Contains(t, prompt, want)
			}
			for _, absent := range tc.absent {
				require.NotContains(t, prompt, absent)
			}
		})
	}
}

func TestPinnedGenerationSkillKeepsResearchAndKnowledgeScope(t *testing.T) {
	engine := newTestEngine(t, &mockChat{})
	engine.knowledgeBasesInfo = []*KnowledgeBaseInfo{{ID: "kb", Name: "Server operations", Capabilities: []string{"chunks"}}}
	engine.SetPinnedMentions(nil, []*PinnedSkillInfo{{Name: "pptx-generator"}})
	prompt := engine.RenderUserTurnContent("session", "如何在 Windows Server 2008 上连接 WiFi 网络？制作相关 PPT")
	require.Contains(t, prompt, "Server operations")
	require.Contains(t, prompt, `read_file(path="skill://pptx-generator/SKILL.md")`)
	require.Contains(t, prompt, "不替代对任务事实内容的检索")
	require.Contains(t, prompt, "除非用户明确限制")
}
