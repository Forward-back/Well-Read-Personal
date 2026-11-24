# 精读 Agent 开发方案

> 状态：优化中
> 最后更新：2026-11-24
> 目标读者：开发、测试、运维
> 关联：[精读agent需求](精读agent需求.md)、[交互式精读](../交互式精读.md)、[ADR 0010](../../../06-架构决策/0010-精读agent采用四段流水线与双模型分层.md)、[当前进度](../../../03-进度与交接/当前进度.md)

## 1. 结论与前置

> 实施状态（2026-11-24）：P0 `0–7` 已落地并部署到后端镜像，迁移 7 已应用；评测基线/v3 指标、自测导入与 v2 运行回归通过。`8` 的完整浏览器全链路、双模型真实组合与 owner 签收未完成，详见[阶段记录](../../../03-进度与交接/阶段记录/2026-11-24-精读agent引擎P0源码与验收.md)。

- 不引入 agent 框架。继续使用 Go 侧状态机、结构化输出解析、既有检索与预算。
- P0 只做引擎闭环：结构预读、逐章策略与记忆、综合遍、自测出卡、双模型、严格 schema、评测指标。
- **前置条件**：先完成当前精读后端契约批次（跨知识库、草稿、候选答案与判定、理解确认、笔记状态、终态停止、跳章）。本方案默认这些契约已存在，逐条裁决与用户理解档案直接复用，不另建平行接口。2026-11-22 前置契约源码已落地，但迁移未应用、镜像未重建、真实链路未验收；agent P0 等前置验收通过后启动。
- 新运行使用 `state_version=3`。已有 `state_version=2` 运行继续走现有 `structuredStep`，不强制迁移。

## 2. 现状核对

以下机制已在代码中存在，P0 保持并复用：

| 机制 | 位置 | 处理 |
|---|---|---|
| 分节窗口 | `reading_run.go` `buildDocSections`、`readingWindowRunes` | 保留；结构遍只补充每节论点，不重建分节 |
| 单轮循环 | `reading_engine.go` `runReadingRound` | 保留主流程；增加 v3 分支与章节策略 |
| 状态摘要 | `buildReadingStateSummary` | 扩展为知识、用户理解、任务三类记忆 |
| 引用机械校验 | `readingQuoteContained`、`resolveReadingEvidence` | 保持，禁止降级为提示词约定 |
| 检索补证 | `searchReadingEvidenceMerged` 与强制检索 | 保持；检索范围仍为选中文档 |
| 结论相关性选择 | `selectRelevantConclusions` | 保留；读取 v3 记忆中的结论摘要 |
| 步数/token 硬顶 | `readingStructuredStepLimit`、`readingStructuredTokenBudget` | 保留；新增阶段不突破硬顶 |
| JSON 解析重试 | `callReadingRoundModel` | 保留重试；增加 v3 schema 校验与降级标记 |
| 终态摘要 | `finishStructured` | 拆成综合遍 + 自测遍 + 最终落盘 |
| 复习生成/导入 | `review_generation.go`、`/review/generations/import` | 自测导入复用复习的查重、吸收与来源逻辑 |

## 3. 目标架构

```text
会话层：ReadingRun + RunMemory + SelfTests + 持久化状态机
策略层：结构预读 / 章节节奏 / 提问预算 / 安静模式 / 预算控制器
执行层：结构遍 -> 逐章读取 -> 综合遍 -> 自测遍
模型层：extraction model（可非 thinking） / synthesis model（thinking）/ 兜底精读模型
```

### 3.1 运行阶段

新增运行 `phase`，与现有 `status` 并存：

| phase | 含义 | 退出条件 |
|---|---|---|
| `structure` | 结构预读与计划骨架 | 结构结果落盘，status=planning |
| `reading` | 逐章读取、陈述、至多一个问题、记忆蒸馏 | 所有 section 已读 |
| `synthesis` | 全局综合、矛盾裁决、术语定稿 | 综合结果落盘 |
| `self_test` | 生成自测题、准备复习预览 | 自测题落盘 |
| `done` | 结束 | status=finished 或 stopped/abandoned |

`Step` 的 v3 分支按 phase 推进，不改变现有 `waiting_user` 的反馈语义。

## 4. P0 实施切片与顺序

### 4.1 迁移与状态契约

1. 新增迁移 `reading_agent`（编号顺延到现有精读契约迁移之后），PostgreSQL 与 SQLite 各一份。
2. `tenants` 新增 `reading_extraction_model_id`、`reading_synthesis_model_id`，可空；现有 `reading_model_id` 保留为兜底。
3. `reading_runs` 新增 `extraction_model_id`、`synthesis_model_id`、`phase`；创建时快照模型，旧运行留空。
4. 新表 `reading_run_memories`：每个 run 一行，`knowledge`、`user_model`、`task` 三个 JSON，整行 replace。
5. 新表 `reading_self_tests`：题目、答案、解析、证据 JSON、section、状态 `candidate/imported/rejected`、创建时间。
6. `ReadingPlan` 增加 `structure` 与 `state_version=3`；结构缺失时用章节标题兜底。

### 4.2 结构预读

1. 在创建运行、生成分节后、进入 `planning` 前执行一次轻量调用。
2. 输入只含目录、标题、每节前若干字与图表标题；不带全文。
3. 输出固定 schema：`sections[{section_id, thesis, knowledge_type}]`，`knowledge_type` 限定 `concept/argument/process/comparison/example/reference/unknown`。
4. 用现有 section ID 做映射；未知 ID 丢弃，缺失 section 用标题与 `unknown` 兜底。
5. 失败时记录 degraded 状态并继续创建计划，不阻塞用户；前端显示兜底骨架与“结构预读降级”提示。
6. 计划确认卡展示每节论点；用户确认后才进入 `reading`。

### 4.3 逐章策略与记忆

1. 保留按标题边界切窗口；不改固定 token 硬切。
2. v3 的回合规则改为：
   - 必产理解陈述；
   - 每章最多一个问题；
   - 快速档不出题；标准/逐条档每章最多一题；
   - 连续跳过两次后，当前章起进入 quiet 模式：只产陈述与笔记，不等待用户。
3. 问题候选、判定与用户作答继续走现有后端契约；判定使用综合模型，返回引用原文的增量反馈。
4. 每轮结束由 `DistillReadingMemory` 合并：
   - `knowledge`：已确认结论、术语、矛盾、章节一句话要点；
   - `user_model`：章节结果 `confirmed/corrected/skipped`、错答点、连续跳过、被驳回结论；
   - `task`：目标、当前章节、剩余步数/token、深度、quiet 状态。
5. 记忆使用 replace，不追加完整对话；提示词只读蒸馏态。
6. 被驳回结论不进入后续摘要、综合与自测。

### 4.4 综合遍

1. 所有 section 读完后进入 `synthesis`，使用综合模型。
2. 输入：结构骨架、三类记忆、已确认结论与证据、未决问题、矛盾清单；不重新注入全文。
3. 输出固定 schema：综合摘要、论证链要点、跨章/跨文档矛盾、术语定稿建议。
4. 新结论仍必须带既有证据；没有新证据不得新增事实，只能合并、裁决或标注未决。
5. 综合结果写入 `final` 与一个 `action=synthesis` 的步骤；完成后进入 `self_test`。

### 4.5 自测遍与复习导入

1. 仅对已有确认结论或术语、且证据完整的 section 生成自测题；每节 2–4 题，全运行最多 20 题进入预览。
2. 使用综合模型，输入该节结论、术语、章节要点与证据，不注入全文。
3. 输出固定 schema：`question/answer/explanation/evidence[]`；问题必须可独立回答，答案必须能由证据支持。
4. 校验：空题、重复题（归一化后哈希）、无有效 quote、引用被驳回结论的题目丢弃；记录丢弃原因。
5. 新增 `GET /api/v1/reading/runs/:id/self-tests` 返回候选题与可导入卡片字段。
6. 新增 `POST /api/v1/reading/runs/:id/self-tests/import`，入参 `group_id` 与 `self_test_ids`；服务端校验 run owner 与题目状态，映射到 `CreateReviewCardRequest`，调用复习导入接口，成功后标记 `imported`。重复导入为幂等空操作。
7. 精读服务通过窄接口注入复习导入能力，容器统一装配；不直接依赖复习 handler，避免反向依赖。
8. 前端不再自行拼卡片来源；导入后刷新预览状态。

### 4.6 双模型与角色校验

1. `GET/PUT /api/v1/reading/config` 扩展两个字段；现有 `reading_model_id` 保留为兜底。
2. 角色解析：
   - 抽取模型：优先 `reading_extraction_model_id`，否则 `reading_model_id`；不要求 thinking。
   - 综合模型：优先 `reading_synthesis_model_id`，否则 `reading_model_id`；必须支持 thinking。
   - 两者都为空：拒绝创建运行。
3. 保存设置时校验模型存在；综合模型额外校验 thinking。运行创建时再次解析并快照到 run，避免设置变更影响进行中的运行。
4. 结构遍与逐章抽取使用抽取模型；综合遍、问题判定、矛盾裁决、自测判定使用综合模型。
5. 设置页文案从“精读模型必须支持思考”改为分别说明两个角色；旧单模型作为兜底显示。

### 4.7 严格 Schema 与可观测性

1. 新增手写校验器 `validateReadingRoundV3`、`validateReadingStructure`、`validateReadingSynthesis`、`validateReadingSelfTests`，不新增 schema 依赖。
2. 校验必填字段、枚举、数组上限、chunk 存在性与 quote 逐字匹配；单条非法证据丢弃并记录，不使整轮崩溃。
3. 解析或校验失败时保留现有一次重试；再次失败将该轮标记 `degraded`，错误写入步骤日志，前端思考链路可见。
4. 预算超限、模型缺失、schema 降级均使用现有步骤与错误通道，不新增静默失败路径。

### 4.8 评测与验收工具

1. 新增 `scripts/acceptance/reading_agent_metrics.py` 与 `scripts/acceptance/reading_agent_cases.json`。
2. 指标：
   - quote 命中率 = 通过逐字校验的模型提议证据 / 模型提议证据；
   - 关键结论召回率 = 命中人工标注关键论点的结论 / 标注关键论点；
   - 自测题有效率 = 通过答案与证据校验且不重复的题 / 生成题。
3. 先跑旧引擎记录基线，再跑 v3；P0 验收门槛见需求文档。
4. 输出每个案例的耗时、token、降级次数与三指标；不把测试数据当业务文档。

## 5. 数据与接口清单

### 5.1 迁移

| 对象 | 变更 |
|---|---|
| `tenants` | 新增 `reading_extraction_model_id`、`reading_synthesis_model_id`，可空 |
| `reading_runs` | 新增 `extraction_model_id`、`synthesis_model_id`、`phase` |
| `reading_run_memories` | 新表：`run_id` 主键、`tenant_id`、`knowledge`、`user_model`、`task`、`updated_at` |
| `reading_self_tests` | 新表：`id`、`run_id`、`tenant_id`、`section_id`、`question`、`answer`、`explanation`、`evidence`、`status`、`created_at` |

### 5.2 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET/PUT | `/api/v1/reading/config` | 扩展抽取模型与综合模型字段；保留兜底精读模型 |
| GET | `/api/v1/reading/runs/:id/self-tests` | 返回自测题与导入所需卡片字段 |
| POST | `/api/v1/reading/runs/:id/self-tests/import` | 按题目 ID 导入复习卡，标记 imported |

现有 `/reading/runs/*`、`/review/generations/import` 路径不变；自测导入由后端调用复习导入逻辑，不新增重复接口。

## 6. 前端改造

1. **模型设置**：抽取模型、综合模型两个选择；抽取模型列出全部可用问答模型，综合模型只列支持 thinking 的模型；保留兜底精读模型。
2. **计划确认**：显示结构骨架、每节一句话论点与知识类型；结构降级时显示轻提示。
3. **工作台**：每章最多一个问题；quiet 模式显示状态标签并自动推进，不出现空问题卡。
4. **阶段确认**：沿用现有理解陈述与确认/纠正/跳过；确认结果由后端写入用户理解档案。
5. **报告与自测**：完成后显示自测题入口、题目预览、卡组选择与导入按钮；导入成功显示数量与跳过数量。
6. **状态可见**：思考链路显示 `structure/synthesis/self_test` 步骤；降级显示可读原因，不显示原始错误堆栈。
7. **视觉**：沿用精读页 Notion/Morandi 页面级 token，不新增组件库或第二套视觉规则。

## 7. 测试与验收

### 7.1 后端

- 迁移测试：PostgreSQL 与 SQLite 均覆盖新增字段、表、索引与回滚。
- 单元：结构映射与兜底、记忆 replace、quiet 熔断、角色解析、schema 校验、自测去重与上限、导入幂等。
- Repository：memory 单行更新；self-tests 状态流转与按 run 查询。
- Handler：owner 校验、配置字段校验、导入 group 归属、空列表与重复导入。
- 旧运行兼容：`state_version=2` 运行可继续 Step/Feedback，不受 v3 分支影响。

### 7.2 前端

- `type-check`、现有测试、`build`。
- 设置双模型选择与回退文案；计划骨架；quiet 模式；自测预览与导入成功/失败态。
- 600px 无脚本错误；控制台 0 错误。

### 7.3 真实链路

- 单文档标准档：结构遍、逐章单问、跳过熔断、综合遍、自测预览、导入 2–5 张卡。
- 双文档跨库（前置契约完成后）：跨文档矛盾进入综合遍，不放开全库检索。
- 抽取模型为非 thinking、综合模型为 thinking：全流程完成。
- 旧运行恢复：`state_version=2` 运行可继续完成并导出。

## 8. 部署与迁移

1. 迁移前备份 PostgreSQL、SQLite 与 `data-files`；新表只增不删。
2. 后端重建 `gaze-app:personal`，前端按视觉签收批次重建 `gaze-ui:personal`。
3. 新运行使用 v3；旧运行不迁移、不强制重开。
4. 回滚策略：保留旧引擎分支；回滚应用版本即可停止创建 v3 运行，已产生的 memory/self-tests 不影响旧运行读取。
5. 不修改 MCP 工具面，不新增 RAG 权限参数，不把精读数据写入 RAG 索引。

## 9. 风险

- 结构遍增加一次模型调用与创建等待时间；必须支持失败兜底，不阻塞开始阅读。
- 每章单问可能降低交互覆盖；通过逐条档更细的窗口与综合遍补足，不在单章堆问题。
- 自测题默认上限 20，可能与每节 2–4 题的覆盖目标冲突；以运行总量和质量优先，未入预览的题留报告。
- 双模型配置增加设置复杂度；默认回退旧模型，旧用户不需要立即配置。
- 旧 `state_version=2` 与 v3 双分支增加维护成本；以常量化分支与测试隔离，后续大版本再评估删除旧分支。
