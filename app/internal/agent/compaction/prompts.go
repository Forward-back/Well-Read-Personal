package compaction

// The sections are not decoration: the summary is the agent's only memory of
// the dropped turns, and unstructured prose reliably loses the two things the
// next round actually needs — what is already finished, and what was about to
// happen next.

const summarizationSystemPrompt = "" +
	"你是上下文摘要助手。任务是阅读用户与 AI 助手之间的对话，" +
	"然后严格按照指定格式生成结构化摘要。\n\n" +
	"不要继续对话，不要回答对话中的任何问题，只输出结构化摘要。"

const summaryFormat = "" +
	"## 目标\n" +
	"[用户最终想完成什么，可能有多项]\n\n" +
	"## 约束与偏好\n" +
	"- [用户要求的格式、工具、风格，以及被否定的方案]\n" +
	"- [或“（无）”]\n\n" +
	"## 进度\n" +
	"### 已完成\n" +
	"- [x] [已完成工作，写明产出的具体文件和写入位置]\n\n" +
	"### 进行中\n" +
	"- [ ] [当前工作]\n\n" +
	"### 阻塞\n" +
	"- [阻碍进度的问题，如有]\n\n" +
	"## 关键决定\n" +
	"- **[决定]**：[简要理由，避免后续反复讨论]\n\n" +
	"## 下一步\n" +
	"1. [按顺序列出接下来要做的事]\n\n" +
	"## 关键上下文\n" +
	"- [后续步骤依赖的事实、数字、标识符、路径和错误信息]\n" +
	"- [或“（无）”]\n\n" +
	"每节保持简洁，使用与对话相同的语言。" +
	"路径、名称、数字和错误文本要保留原值，不要改写。" +
	"只输出摘要，不要前言。\n\n" +
	"长度很重要：摘要与保留的近期消息共享上下文窗口，必须足够紧凑才值得保留。" +
	"控制在 500 字以内。优先使用短条目，不要长篇叙述；后续步骤无法使用的细节应删除。"

const initialSummarizationInstructions = "" +
	"以上是需要总结的对话。请创建结构化上下文检查点，" +
	"供另一个 LLM 继续工作使用。\n\n" +
	"严格使用以下格式：\n\n" + summaryFormat

const updateSummarizationInstructions = "" +
	"以上是需要并入 <previous-summary> 中原有摘要的新对话消息。\n\n" +
	"规则：\n" +
	"- 保留后续步骤仍需要的原有摘要信息\n" +
	"- 加入新消息中的进度、决定和上下文\n" +
	"- 更新进度：完成项从“进行中”移到“已完成”\n" +
	"- 根据实际完成情况更新“下一步”\n" +
	"- 保留精确的文件路径、函数名和错误信息\n" +
	"- 持续压缩：除非确实出现新事实，结果不得长于原摘要。" +
	"已完成工作各压缩为一行；被取代的计划、已解决的错误和放弃的方案全部删除。" +
	"只增不减的更新会破坏压缩本身。\n\n" +
	"严格使用以下格式：\n\n" + summaryFormat

// turnPrefixInstructions summarizes the discarded head of a split turn. It is
// a different job from the history summary: the retained suffix is still in
// context, so this only has to supply what the suffix cannot explain about
// itself — above all, what the user originally asked for.
const turnPrefixInstructions = "" +
	"这是因过长而被拆分的一轮对话的前半部分。后半部分（近期工作）仍保留在对话中，" +
	"此处不显示。\n\n" +
	"总结前半部分，为保留的后半部分提供上下文，严格使用以下格式：\n\n" +
	"## 原始请求\n" +
	"[用户这一轮要求做什么？]\n\n" +
	"## 早期进度\n" +
	"- [前半部分的关键决定与已完成工作，写明产物和路径]\n\n" +
	"## 后半部分所需上下文\n" +
	"- [理解保留的近期工作所需的信息]\n\n" +
	"保持简洁，使用与对话相同的语言。保留精确的文件路径、名称和错误信息。只输出摘要。"

const splitTurnSeparator = "\n\n---\n\n**本轮上下文（拆分轮次）：**\n\n"
