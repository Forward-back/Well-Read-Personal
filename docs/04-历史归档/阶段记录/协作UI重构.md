# 协作 UI 重构

> 状态：历史
> 日期：2026-10-22 至 2026-10-23
> 关联：[协作与版本控制](../../02-需求与设计/功能与交互/协作与版本控制.md)、[协作工作台设计说明](../../02-需求与设计/界面与原型/协作工作台/设计说明.md)、[当前进度](../../03-进度与交接/当前进度.md)

> 2026-10-26 路径迁移：`app/frontend/` 现为 `frontend/`，`YLSW_RAG/` 现为 `rag/`；本文文件清单为历史路径。
>
> 阅读提示：§1–§7 是 2026-10-22 暂停时的快照；§8–§9 是 2026-10-23 续开发结果。本文为阶段记录，当前状态以[当前进度](../../03-进度与交接/当前进度.md)为准。
> 依据：协作工作台原型 `协作入口.html`、`变更请求工作台.html`、`变更请求详情.html`、`版本历史.html` 与设计说明。
> 原则：抽屉管“看”，整页管“干”；轻量 GitHub 模型，不引入文档分支。

## 1. 已对齐的设计决策

1. **范围**：全量落地，含后端数据模型/API 与前端四个原型；不是仅 UI 皮肤。
2. **CR 状态机**：`draft → pending_review → changes_requested → merged / closed`。审批通过不等于合并，必须显式执行「合并变更请求」。
3. **审阅者**：创建时从空间成员中指定 1–N 人；任意一位批准即可合并；owner/admin 可越权合并；空间创建者/admin 可随时调整审阅者；重提后所有批准重置。
4. **文档版本**：初始导入为 V1.0；合并 CR、手动分块编辑、回滚都会生成新版本（V1.x）。回滚不是指针回退，而是以目标版本内容生成新版本，保留审计链。
5. **选区锚点**：选区跨分块时记录主锚点（起点所在 chunk）与选区文本/偏移；「建议修改」预选所有相交分块并在工作台定位主块。行号按内容最佳努力计算。
6. **选区评论**：新增独立文档选区评论实体；文档内容 Tab 右侧评论栏展示与回复；「引用评论」走此实体，不走 issue。
7. **问题转 CR**：先创建 issue，再跳 02 工作台并自动关联；合并时可勾选关闭关联 issue。
8. **issue 类型**：数据错误 / 事实过期 / 表述不清 / 格式问题 / 其他。
9. **引用归属**：引用、锚点、引用统计必须使用 app chunk id。现状是 YLSW 自行分块并返回内部 id，必须修：ingest 时 app 把 app chunks 随 markdown 推给 YLSW，YLSW 保存 `source_chunk_id` 并在引用中返回 `app_chunk_id`。
10. **合并后同步**：沿用现有整篇重推+对账链路，合并后文档标记 `pending`；不实现分块级重嵌。UI 文案不要承诺“仅重嵌 N 个块”。
11. **版本迁移**：迁移时为现有文档生成 V1.0 当前内容快照；旧状态 `open/accepted/rejected` 映射到新五态；旧已接受 CR 标记 merged，不回溯造历史版本节点。
12. **视觉**：严格还原四个 HTML 的布局、字号、间距、徽章与 diff 配色；协作模块使用独立 `--cv-*` tokens（#10B981/紫/amber 等），不强行套现有 TDesign 视觉。
13. **检查 tab**：03 的「检查」是自动校验清单（锚点失效、分块冲突、建议为空/未变化、审阅者不足等）。
14. **直接编辑**：保留现有分块直接编辑，但每次内容变更也生成新版本。
15. **统计**：原型里的「被检索引用 N 次/月」目前没有真实数据源；待 app chunk id 映射和引用计数落库后再接，暂不造假。

## 2. 已完成的代码（未运行时验证）

### 2.1 后端数据模型与迁移

新增迁移：

- `app/migrations/versioned/000106_collaboration_v2.up.sql` / `.down.sql`
- `app/migrations/sqlite/000018_collaboration_v2.up.sql` / `.down.sql`

包含：

- `doc_issues`：`issue_type`、`anchor_text/start/end`、`anchor_line_start/end`。
- `doc_change_requests`：五态状态迁移、`base_version_id`、`merged_version_id`、`merged_at`、`closed_by/at`、`tags`、`draft_updated_at`。
- `doc_cr_items`：`original_revision`、`viewed_by`、`anchor_text`、`updated_at`。
- 新表：`doc_cr_reviewers`、`doc_cr_issues`、`doc_versions`、`doc_version_chunks`、`doc_anchor_comments`、`chunk_citation_daily`。

**重要：迁移尚未执行。** 停止开发时数据库仍是旧版本（PG 000105 / SQLite 000017），因为没有成功重启 app 容器。

### 2.2 后端服务与接口

已修改/新增：

- `internal/types/collaboration.go`：五态常量、审阅者/版本/选区评论/CR-issue 关联类型、新请求体。
- `internal/types/interfaces/collaboration.go`、`collaboration_service.go`：v2 repository/service 接口。
- `internal/application/repository/collaboration_v2.go`（新）：审阅者、显式关联、草稿更新、合并事务、已查看、版本快照/回滚、选区评论。
- `internal/application/service/collaboration.go`：五态创建/审批/合并/关闭/草稿/审阅者调整。
- `internal/application/service/collaboration_v2_service.go`（新）：选区评论、版本列表/详情/对比/回滚权限入口。
- `internal/application/service/document_version.go`（新）：V1.0 基线、版本创建、对比、回滚事务。
- `internal/application/service/chunk.go`：分块直接编辑前后接入版本服务（EnsureInitialVersion / RecordManualEdit）。
- `internal/application/service/engine_sync.go`、`knowledge_process.go`、`evaluation.go`：ingest 携带 app chunks。
- `internal/engine/engine.go`、`ylsw_engine.go`：`IngestChunk`、`app_chunk_id`、`source_doc_id` 映射；引用 ID 优先 app chunk。
- `internal/handler/collaboration_v2.go`（新）及 `internal/router/routes_collaboration.go`：合并/关闭/草稿/提交/审阅者/已查看/选区评论/版本接口。
- `internal/container/container.go`：注册 `DocumentVersionService`；`NewChunkService` 新增版本服务依赖。

验证情况：`go build ./internal/...` **通过**；未跑 Go 测试（本机 cgo/gojieba 限制），未做接口冒烟。

### 2.3 YLSW app chunk 归属映射

已修改：

- `YLSW_RAG/init.sql`：`chunks.source_chunk_id`。
- `YLSW_RAG/src/ingest/pipeline.py`：`insert_chunk` 支持 `source_chunk_id`。
- `YLSW_RAG/src/app.py`：
  - `/ingest` 接收 `chunks`（app 侧 chunk id/内容/偏移）；
  - 分块后按内容匹配写入 `source_chunk_id`；
  - 引用与 `/search` 返回 `app_chunk_id`、`source_doc_id`。
- `YLSW_RAG/src/retrieval/chunk_store.py`：加载并暴露 `source_chunk_id` 映射。

已验证：`py_compile` 通过；本机 MySQL 已手动加列；YLSW 容器已 `docker cp` 三个 py 文件并重启。

**未验证完**：app 容器二进制当时未完成热重启，重推文档时仍未发送 app chunks，`source_chunk_id` 填充数仍为 0。恢复后需重启 app、重推存量文档并再次确认。

### 2.4 前端已完成/初版

新增或重写：

- `api/collaboration/index.ts`：v2 类型与接口（草稿、审批、合并、关闭、审阅者、已查看、选区评论、版本）。
- `views/knowledge/collaboration/CollaborationWorkbench.vue`：原型 C 风格列表、状态计数、类型/我创建的筛选、草稿可见性（依赖后端迁移）。
- `views/knowledge/collaboration/IssueCreateDialog.vue`：原型 B 表单；类型 chips；锚点引用；「转为变更请求」先创建 issue 再跳 02。
- `views/knowledge/collaboration/CRWorkbench.vue`（新）：02 三栏工作台初版；全量 chunk 加载/搜索/多选；并排原文与建议编辑；diff 预览；标题/说明/标签/关联 issue/审阅者/自查/影响框；自动保存草稿；提交跳 03。
- `views/knowledge/collaboration/CRDetail.vue`（新）：03 详情页初版；对话/分块变更/版本/检查 tab；审阅时间线；评论；diff；已查看；审阅侧栏；批准/要求修改/合并/关闭/调整审阅者。
- `views/knowledge/collaboration/VersionHistory.vue`（新）：抽屉版本 Tab；时间线；任意版本对比；版本列表；下载；回滚。
- `views/knowledge/collaboration/DiffBlock.vue`、`diff.ts`、`status.ts`、`styles/collaboration.css`：diff/状态/视觉 tokens。
- `router/index.ts`：新增 `/platform/knowledge-bases/:kbId/doc/:docId/cr/new` 与 `/cr/:crId`。
- `components/doc-content.vue`：文档抽屉新增「版本」tab；分块「提 CR」改为跳整页工作台；移除旧新建 CR 弹窗使用。
- `views/knowledge/KnowledgeBase.vue`：`doc_tab=versions` 开抽屉版本页。
- `i18n/locales/zh-CN.ts`：新增一批协作 v2 文案键。

## 3. 未完成/明确缺口

### 3.1 阻塞项（必须先处理）

1. **Docker Desktop 异常**：停止时 Docker API 返回 500，`Gaze-app` unhealthy；无法迁移数据库、无法冒烟后端、无法浏览器联调。恢复后先 `docker ps`、重启 `Gaze-app`，确认 000106/000018 迁移成功。
2. **app 容器未完成热重启**：YLSW app chunk 映射未生效；需重启后重推文档并验证 `chunks.source_chunk_id` 与引用 `app_chunk_id`。
3. **迁移未执行**：所有 v2 接口和前端新页面在旧库上会失败（缺列/表/新状态）。
4. **前端未 type-check / build**：本次新增组件尚未跑 `vue-tsc` 和生产构建；已知旧项目 5 个 type-check 错误，但本次可能新增错误。
5. **后端接口未冒烟**：草稿、审批、合并、版本、评论、审阅者均未通过 HTTP 实测。

### 3.2 前端未完成

1. **01 A 区完全未做**：文档内容页选区浮动工具条（引用评论/提交问题/建议修改）尚未接入；选区到 chunk 的映射、工具条定位、行号计算未实现。
2. **右侧评论栏未做**：选区评论后端接口已就绪，但前端评论栏、回复线程、定位/解决/删除未实现。
3. **02 工作台缺口**：
   - `?cr=<id>` 修改中 CR 的“编辑并重新提交”未接入（CRDetail 会跳过去，但工作台还没 hydrate/resubmit）；
   - 分块冲突锁（CR#14 修改中）未接；缺后端 active-CR chunk lock 接口；
   - “被检索引用次数/月”未接；
   - 草稿自动保存/恢复逻辑未实测，且列表恢复草稿依赖详情接口；
   - 审阅者选择仅用全空间成员，未做权限过滤/推荐；
   - 前端草稿可能因首次输入即创建，需联调确认不会产生过多空草稿。
4. **03 详情页缺口**：
   - 批准/要求修改/合并/关闭、审阅者调整未实测；
   - 合并确认框、关联 issue 关闭未实测；
   - “版本” tab 只是版本列表 + 跳转，未做原型级版本对比/定位；
   - 评论删除/审阅者权限边界未实测；
   - 旧 `CollaborationDetailDrawer.vue` 仍保留 CR 分支和旧三态文案，需要清理为仅 issue 详情。
5. **04 版本历史缺口**：
   - `getDocVersion` 返回结构按 `{data, chunks}` 处理，需联调确认；
   - 回滚、下载、对比未实测；
   - 草稿/进行中 CR 的未来节点只用简化节点，未与 CR 状态联动；
   - 文档内容页版本 tab 与详情页版本跳转未实测。
6. **旧组件清理**：`ChangeRequestCreateDialog.vue` 已不再被 doc-content 引用，但文件仍在；`CollaborationDetailDrawer.vue` CR 部分未清理；`docInfo.vue`/`ChatReferencesDrawer.vue` 仍使用 IssueCreateDialog 新表单，未回归。
7. **引用统计**：`chunk_citation_daily` 表已建迁移，但未在引用落库时计数，也没有查询接口/前端展示；`CRWorkbench` 影响框暂未展示真实引用次数。
8. **i18n**：新增键未做审计；部分组件仍可能显示原始 key 或旧文案。
9. **冲突锁接口**：原型 02 的“该块正在被 CR#14 修改”没有后端查询接口，前端未显示。

### 3.3 后端未完成/风险

1. **版本回滚边界**：回滚时被软删的 chunk 无法用原 ID 恢复，会以新 chunk 创建；issue/CR 锚点可能失效。已预留 `NewChunks`，但未做旧 ID 恢复与锚点迁移测试。
2. **版本号并发**：`version_seq` 由“当前最新 +1”计算，未加并发锁；极端并发可能撞唯一索引。
3. **合并事务**：合并时版本快照、chunk 更新、issue 关闭、engine_sync pending 在一个事务；未实测回滚/冲突场景。
4. **直接编辑升版**：`UpdateDocumentChunk` 在保存后生成版本；若版本服务失败只记日志，不阻断编辑；需确认业务上可接受。
5. **草稿状态可见性**：聚合列表已为草稿加“仅创建者可见”条件，但 `ListDocChangeRequests` 单独列表接口仍可能泄露草稿，需补同样过滤。
6. **审阅者动态调整**：owner/admin/创建者可调整；保留同一审阅者历史决定；重提全量重置；未实测。
7. **旧数据状态映射**：迁移 SQL 已写，但未执行；前端旧组件/测试仍引用 `open/accepted/rejected`，需要清理。
8. **接口兼容**：旧 `review` 路由语义已改为 approve/request_changes；旧前端若未全部替换会不兼容。
9. **测试**：Go 测试未跑；前端已有 11 个既有失败（Wiki/MCP/导航类旧测试），本次新增代码未跑。

## 4. 恢复开发步骤（建议顺序）

1. **恢复 Docker 与 app**：检查 Docker Desktop；重启 `Gaze-app`；等待 air 构建完成；确认 `Gaze-app` healthy。
2. **执行迁移**：确认 PG `schema_migrations` 到 106、SQLite 到 018；检查列/表/状态映射；处理迁移失败。
3. **YLSW 映射复验**：重启 app 后对一篇文档调用 `engine-sync`；确认 YLSW `chunks.source_chunk_id` 非空、引用返回 `app_chunk_id`；再决定是否回填全部存量文档。
4. **后端冒烟**：按顺序测 draft 创建/更新/提交 → 指定审阅者 → 批准 → 合并 → 版本列表/对比/回滚 → issue 类型/选区评论 → 旧状态列表。
5. **前端编译**：`npm run type-check`、`npm run build`；修复本次新增错误（不要一次性修既有 5 个）。
6. **补 01**：文档内容页选区工具条 + 右侧评论栏；跨块主锚点；提交问题；建议修改跳 02；引用评论线程。
7. **补 02/03/04**：修改中 CR 回填重提；冲突锁；版本 tab 联动；审阅者/合并/关闭实测；清理旧组件。
8. **联调验收**：用 smoke 租户走全链路，截图与原型逐项比对；最后清测试数据、重建 `gaze-ui`/`gaze-ylsw`/`gaze-app` 镜像。

## 5. 关键环境与测试数据（暂停时）

- 开发入口：`http://localhost:5173`（Vite dev server 暂停前仍在运行，需确认 HMR 与端口占用）。
- app 容器：`Gaze-app` 源码 bind mount，air 热重载；暂停时 unhealthy，需重启。
- YLSW 容器：已热补 `app.py`/`pipeline.py`/`chunk_store.py`；MySQL `YLSW_Service_Table.chunks.source_chunk_id` 列已手动添加；镜像尚未重建。
- 本机 PG：`WellRead`，迁移暂停在 000105；本机 MySQL：`YLSW_Service_Table` 与 `ums_employee`。
- 测试账号：`e2etest@example.com / Admin123!`、`smoke_collab_20261022@example.com / Smoke123!`、临时 viewer `smoke_viewer_20261022@example.com / Smoke123!`（交付前删除）。
- 测试数据：P0 验收创建的 issue/CR（含 CR#4 已合并、issue #4/#5 等）、YLSW 文档重推记录；交付前清理。
- P0 既有修复（本文件之外）：需求五 `use-original` 传参、YLSW 空 child 守卫、QA 引用抽屉双入口、缺失 i18n 键等已完成并曾复验；但本次 CR 工作台已替换旧 CR 弹窗，旧 `ChangeRequestCreateDialog.vue` 不再使用。

## 6. 关键文件清单

后端：

- `app/internal/types/collaboration.go`
- `app/internal/types/interfaces/collaboration.go`
- `app/internal/types/interfaces/collaboration_service.go`
- `app/internal/application/repository/collaboration.go`
- `app/internal/application/repository/collaboration_v2.go`
- `app/internal/application/service/collaboration.go`
- `app/internal/application/service/collaboration_v2_service.go`
- `app/internal/application/service/document_version.go`
- `app/internal/application/service/chunk.go`
- `app/internal/application/service/engine_sync.go`
- `app/internal/application/service/knowledge_process.go`
- `app/internal/engine/engine.go`
- `app/internal/engine/ylsw_engine.go`
- `app/internal/handler/collaboration_v2.go`
- `app/internal/router/routes_collaboration.go`
- `app/internal/container/container.go`

迁移：

- `app/migrations/versioned/000106_collaboration_v2.up.sql`
- `app/migrations/versioned/000106_collaboration_v2.down.sql`
- `app/migrations/sqlite/000018_collaboration_v2.up.sql`
- `app/migrations/sqlite/000018_collaboration_v2.down.sql`

YLSW：

- `YLSW_RAG/src/app.py`
- `YLSW_RAG/src/ingest/pipeline.py`
- `YLSW_RAG/src/retrieval/chunk_store.py`
- `YLSW_RAG/init.sql`

前端：

- `app/frontend/src/api/collaboration/index.ts`
- `app/frontend/src/views/knowledge/collaboration/CollaborationWorkbench.vue`
- `app/frontend/src/views/knowledge/collaboration/IssueCreateDialog.vue`
- `app/frontend/src/views/knowledge/collaboration/CRWorkbench.vue`
- `app/frontend/src/views/knowledge/collaboration/CRDetail.vue`
- `app/frontend/src/views/knowledge/collaboration/VersionHistory.vue`
- `app/frontend/src/views/knowledge/collaboration/DiffBlock.vue`
- `app/frontend/src/views/knowledge/collaboration/diff.ts`
- `app/frontend/src/views/knowledge/collaboration/status.ts`
- `app/frontend/src/styles/collaboration.css`
- `app/frontend/src/router/index.ts`
- `app/frontend/src/components/doc-content.vue`
- `app/frontend/src/views/knowledge/KnowledgeBase.vue`
- `app/frontend/src/i18n/locales/zh-CN.ts`

## 7. 暂停时结论

- 设计已对齐，后端主体与前端四个页面的初版代码已落，但**未迁移、未冒烟、未编译验证、未联调、未验收**。
- 恢复开发时不要直接假设可运行；先完成第 4 节 1–5 步，再继续补 01 与联调。
- 中断原因是 Docker Desktop/app 容器异常；不是代码已收口。

---

## 8. 2026-10-23 续开发结果（已恢复并完成主体）

### 8.1 已完成并验证

- **迁移已执行**：PG `schema_migrations=106`，新列/新表全部生效；状态映射完成。
- **后端接口已冒烟**：
  - 草稿创建 → 自动保存 → 提交；
  - 指定审阅者 → 审阅者批准 / 要求修改 → 重提（审批重置）；
  - owner/admin 合并、审阅者批准后合并、关闭 CR；
  - 动态调整审阅者；
  - 版本快照（初始 V1.0、CR 合并、手动编辑、回滚）与回滚生成新版本；
  - issue 类型/锚点、CR↔issue 关联、合并时关闭关联 issue；
  - 选区评论、chunk 冲突锁、引用统计。
- **前端已完成并实测**：
  - 01：选区工具条（提交问题 / 建议修改 / 引用评论，跨块主锚点）、新建问题表单、issue 转 CR、右侧评论栏（提交/回复/解决/删除）；
  - 02：三栏工作台、全量分块搜索/多选、并排编辑/diff、草稿自动保存/恢复、冲突锁提示、审阅者、关联 issue、自查、引用统计、修改中重提；
  - 03：整页详情、对话时间线、分块 diff、已查看、审阅者侧栏、批准/要求修改/合并/关闭、检查 tab、版本 tab；
  - 04：文档抽屉版本历史、时间线、版本对比、下载、回滚；
  - 版本 tab 已接入文档抽屉（文档内容 / 协作 / 版本历史 / 评论）。
- **YLSW 引用归属已修**：ingest 时 app chunks 随 markdown 推送；YLSW 落 `source_chunk_id`；引用优先返回 app chunk id，YLSW 内部 id 保留在 `metadata.engine_chunk_id`。存量文档已批量重推：126/126 synced，4814/4852 YLSW chunk 已有 app 映射（剩余为旧/无 child 的 chunk）。
- **验证**：`go build ./internal/...` 通过；前端 `npm run type-check` 仅剩 5 个既有错误（ModelEditorDialog / citationMarkdown / RagPipelineProgress ×2 / ApiIntegrationSettings）；`npm run build` 通过（约 1m25s）。应用容器、YLSW、Vite 5173 均已恢复运行。

### 8.2 尚未完成/交付前事项

1. **用户最终目验**：四块 UI 尚未由用户逐项确认；截图在 `C:/tmp/gaze-cdp/shots/`（a6 列表、b8 工作台、b7 CR 详情、a5 版本 tab、c5 选区工具条、b4 评论栏等）。
2. **测试数据清理**：smoke 租户存在多条验收 CR/issue、版本（V1.0–V1.6）、临时 viewer 账号、YLSW 重推记录；交付前清理或重建干净演示数据。
3. **镜像重建**：YLSW 仅热补源码与 MySQL 列，未重建 `gaze-ylsw` 镜像；前端 `dist` 已构建但 `gaze-ui` 镜像未重建。交付前重建 app/ui/ylsw 镜像。
4. **旧组件清理**：`views/knowledge/collaboration/ChangeRequestCreateDialog.vue` 已无引用；`CollaborationDetailDrawer.vue` 仍保留旧 CR 分支（现在只用于 issue 详情），建议后续清理。
5. **UI 细节**：03 对话时间线未展示“某人更新了分块”的事件（只有评论/审阅事件）；版本 tab 的进行中 CR 节点为简化展示；选区工具条在预览/合并视图依赖内容匹配，跨页注释跳转只对当前已加载页做即时滚动。
6. **版本回滚边界**：被软删 chunk 回滚时目前会尝试用原 ID 新建，若库中存在同 ID 软删行，需再验证/补 unscoped 恢复；并发版本号仅靠唯一索引兜底。
7. **SQLite/PG 差异**：引用统计接口目前仅 PG 返回数据（SQLite 返回空）；数据库迁移 000018 尚未在 SQLite 环境实跑。
8. **测试**：Go 测试未运行（本机 cgo 限制）；前端既有 11 个 Wiki/MCP/导航类测试失败未处理，本次新增代码未引入 type-check 错误。
9. **docs 交接旧结论**：本文第 1–7 节是暂停时快照，仍可参考设计决策与文件清单；当前状态以本节为准。

### 8.3 快速验收入口

- 前端：`http://localhost:5173`
- owner：`smoke_collab_20261022@example.com / Smoke123!`（smoke 库）
- viewer：`smoke_viewer_20261022@example.com / Smoke123!`
- e2etest：`e2etest@example.com / Admin123!`（开发测试库，120 篇）
- 协作列表：`/platform/knowledge-bases/smoke-kb-1?tab=collab`
- 工作台：`/platform/knowledge-bases/smoke-kb-1/doc/smoke-doc-1/cr/new`
- 文档抽屉版本：`...?tab=documents&knowledge_id=smoke-doc-1&doc_tab=versions`
- 选区工具条：文档抽屉切「查看分块」→ 选中文字 → 浮动条；或「引用评论」打开右侧评论栏。

### 8.4 2026-10-23 UI 问题一修正

用户按原型目验后提出的第一组问题已修:

1. 工作台三栏独立滚动:页面固定视口高度,左分块列表/中编辑区/右表单各自 `overflow-y:auto`;内部滚动条隐藏但滚轮仍可滚。
2. 分块编辑器改为 VS Code 风格:双 pane(原内容只读/建议内容可编辑)、Cascadia Code 等宽字体、行号 gutter、280px 编辑器视口、底部行数/UTF-8/Markdown 状态条、品牌绿聚焦边框;建议 textarea 行号随滚动同步。
3. 对比预览改为对齐 split diff(`SplitDiff.vue`):删除行右侧保留空行、新增行左侧保留空行,上下文行不再自动靠拢;工作台预览与 CR 详情/版本对比的 Split 模式共用。
4. 编辑器/分块列表/中栏/右栏/diff 表格的滚动条均已隐藏。
5. 前端 `npm run build` 再次通过(约 1m4s)。

### 8.5 2026-10-23 编辑器滚动/草稿修复

- 分块编辑器与 split diff 的竖直滚动条隐藏,保留每栏独立水平滚动条(可拖动,触控板按所在栏滑动)。
- 原文 `<pre>` 不再被 flex 拉伸到固定高度,内容高度参与 `scrollHeight`,修复左侧末尾行被裁掉。
- 编辑态双栏纵向滚动同步(原内容 surface ↔ 建议 textarea ↔ 行号 gutter);横向独立。
- split diff 改为两个独立 scroll pane,修复长行文字跨栏重叠;上下文行左右沿用原文行号,新增行显示 `+新行号`。
- **草稿保存 400 根因**:`CreateDocCRRequest` 的 handler binding 仍要求 `title required`、`items min=1`,空白草稿在进入 service 前被拒。已移除这两个 binding;提交时仍由 service 强制校验标题与至少一个变更分块。
- 修复后创建空草稿/更新草稿 API 201/200;前端自动保存显示“草稿已自动保存”。注意:air 有时构建了新二进制但不重启旧进程,需 `docker restart Gaze-app` 兜底。
- 前端 `npm run build` 再次通过(1m22s);type-check 仍仅 5 个既有错误。

### 8.6 2026-10-23 滚动条与行号二次修正

- 分块编辑与预览统一滚动条口径:只保留右侧建议 pane 的竖向滚动条;左侧原内容 pane 的横/竖向滚动条隐藏,右侧横向滚动条隐藏。滚动仍可用滚轮/触控板。
- 修复右侧建议 gutter:其容器高度改为固定 100%,textarea 滚动时 gutter 同步 scrollTop,行号随内容行移动,不再固定。
- 编辑态原文/建议纵向滚动同步,横向独立;预览双 pane 纵向同步、横向独立。
- 行号按逻辑行计数(不自动换行,`white-space: pre`),上下文行左右沿用原文行号,新增行显示 `+新行号`。

### 8.7 2026-10-23 文档详情移除分块编辑

- 文档内容抽屉「查看分块」不再提供直接编辑:移除分块工具栏编辑铅笔、内联编辑器、分块编辑历史中的“回退版本”按钮(它会直接改分块内容)。
- 分块内容编辑唯一入口保留为协作 CR 工作台(分块工具栏「提 CR」仍可用);启用/禁用开关与只读编辑历史保留。
- 同步清理 `doc-content.vue` 中 `editingChunkId`/`chunkDraft`/`startChunkEdit`/`saveChunkEdit`/`revertChunk` 及相关样式、`revertDocumentChunk` 导入和对应 i18n 键。
- `npm run build` 通过(1m21s);CDP 实测分块视图 `.t-icon-edit` 0 个、`.chunk-editor` 0 个、历史弹窗回退按钮 0 个。

### 8.8 2026-10-23 CR 详情页双栏滚动

- 问题根因:平台路由出口 `.platform-route-outlet` 为 `overflow:hidden`,详情页用 `min-height:calc(100vh - 64px)` 撑高,内容超出视口时被直接裁掉,页面与栏目都没有滚动。
- 修复:`.cr-detail-page` 改为 `height:100%` 纵向 flex;顶部栏/标题/选项卡 `flex:none`;`.detail-body` 占满剩余高度。
- 左栏 `.detail-main` 与右栏 `.detail-side` 各自 `overflow-y:auto`,互不影响;两栏竖向滚动条通过 `scrollbar-width:none` + `::-webkit-scrollbar{display:none}` 隐藏。
- CDP 实测:1440×1000 合成超高内容两栏 scrollTop 分别 900/600;125% 缩放(1254×698)真实 CR#13 左栏 `scrollHeight 631 > clientHeight 487`,可滚至 144;两栏 `offsetWidth == clientWidth`,无可见滑块;页面本身无溢出。
- `npm run build` 通过(1m8s)。

---

## 9. 当前进度总结（2026-10-23 17:10）

### 9.1 总体状态

- 协作 UI v2 四块原型（01 入口/issue、02 CR 工作台、03 CR 详情、04 版本历史）**主体已全量落地并联调通过**；后端数据模型、API、YLSW 引用归属映射已完成并冒烟。
- 当前处于**用户逐项目验 + UI 细节返工阶段**：已按反馈完成 8.4–8.8 五轮修正，主体链路可用，尚未正式交付验收。
- 未做事项集中在交付收尾（测试数据清理、镜像重建、最终回归）与少数边界技术问题，不在主功能链路。

### 9.2 已完成

- **后端**：迁移 000106/000018 已落地；PG 已执行至 version 106（SQLite 环境尚未实跑）。CR 五态 `draft → pending_review → changes_requested → merged/closed`、草稿自动保存、动态审阅者、显式合并、关闭、重提重置审批、版本快照/对比/回滚、选区评论、CR↔issue 关联、chunk 冲突锁、引用统计均已接口冒烟。
- **数据归属**：ingest 时 app chunks 随 markdown 推送 YLSW；`chunks.source_chunk_id` 落库；引用优先返回 app chunk id（engine id 保留在 metadata）。存量文档 126/126 已重推，4814/4852 YLSW chunk 有 app 映射。
- **前端**：
  - 01 选区工具条（提交问题/建议修改/引用评论，跨块主锚点）、新建问题、issue 转 CR、右侧评论栏；
  - 02 三栏工作台（分块搜索/多选、VS Code 风格并排编辑、split diff、草稿自动保存/恢复、冲突锁、审阅者、关联 issue、自查、引用统计、修改中重提、左右栏折叠）；
  - 03 CR 详情（对话/分块变更/版本/检查、已查看、审阅侧栏、批准/要求修改/合并/关闭、双栏独立滚动）；
  - 04 文档抽屉版本历史（时间线、任意版本对比、下载、回滚生成新版本）；
  - 文档内容抽屉已接入版本 tab；「查看分块」已移除直接编辑，内容编辑唯一入口为协作 CR。

### 9.3 本轮验收后已修正的 UI 问题（8.4–8.8）

1. 工作台三栏独立滚动；编辑器 VS Code 化；split diff 对齐、不自动靠拢。
2. 编辑器/预览滚动条口径统一：只保留右侧竖向滚动条，其余隐藏；横向滚动独立。
3. 行号按逻辑行计数，左右行号一致；右侧 gutter 随 textarea 滚动。
4. 草稿保存 400 修复（handler binding 去掉 title/items 强制要求，提交时由 service 校验）。
5. 文档详情「查看分块」移除编辑铅笔、内联编辑器、历史回退；保留只读历史与启停开关。
6. CR 详情页左右栏独立纵向滚动、竖向滑块隐藏（根因：路由出口 `overflow:hidden` + 页面 `min-height` 撑高被裁切）。

### 9.4 验证基线

- `go build ./internal/...` 通过。
- 前端最新 `npm run build` 通过（1m8s）；`npm run type-check` 仍仅 5 个既有错误（ModelEditorDialog / citationMarkdown / RagPipelineProgress ×2 / ApiIntegrationSettings）。
- CDP 已验证：草稿创建/保存/提交、审阅者批准（含 viewer）、合并、关闭、重提、动态审阅者、版本对比/回滚、选区工具条三类入口、评论栏、冲突锁、引用统计、工作台折叠/滚动/行号、CR 详情双栏滚动、分块编辑入口移除。
- **镜像未重建**：`gaze-app`/`gaze-ui`/`gaze-ylsw` 仍是开发态（YLSW 仅热补源码与 MySQL 列）。

### 9.5 交付前剩余事项

1. 用户最终目验四个原型页面，按反馈继续微调。
2. 清理 smoke 租户测试 CR/issue/版本/草稿、临时 viewer 账号与 YLSW 重推测试记录；或重建干净演示数据。
3. 重建 `gaze-app`、`gaze-ui`、`gaze-ylsw` 镜像并做全栈回归。
4. 清理旧组件：`ChangeRequestCreateDialog.vue`（无引用）、`CollaborationDetailDrawer.vue` 的旧 CR 分支（现仅用于 issue）。
5. UI 遗留：03 对话时间线未展示“分块更新”事件；版本 tab 的进行中 CR 节点为简化展示；选区工具条在预览/合并视图依赖内容匹配，跨页注释仅对当前已加载页即时滚动。
6. 技术边界：回滚软删 chunk 的 ID 恢复与锚点迁移未验证；版本号并发仅靠唯一索引兜底；SQLite 迁移 000018 未实跑；引用统计仅 PG 返回数据。
7. 测试：Go 测试未运行（本机 cgo 限制）；前端既有 11 个 Wiki/MCP/导航类测试失败未处理。

### 9.6 环境与入口

- 前端 `http://localhost:5173`；app `http://localhost:8080`；容器 `Gaze-app`/`Gaze-ylsw`/`Gaze-postgres`/`Gaze-redis` 运行中。
- owner：`smoke_collab_20261022@example.com / Smoke123!`；viewer：`smoke_viewer_20261022@example.com / Smoke123!`；开发库：`e2etest@example.com / Admin123!`。
- 协作列表：`/platform/knowledge-bases/smoke-kb-1?tab=collab`；工作台：`/platform/knowledge-bases/smoke-kb-1/doc/smoke-doc-1/cr/new`；文档抽屉版本：`...?tab=documents&knowledge_id=smoke-doc-1&doc_tab=versions`。

### 9.7 建议下一步

1. 完成用户目验，收集剩余视觉/交互问题并逐项修正。
2. 验收通过后清理测试数据与临时账号。
3. 重建三套镜像，跑 compose 全栈冒烟。
4. 处理 9.5 中的旧组件清理与可选技术边界项（至少完成 SQLite 迁移实跑与回滚边界验证）。
5. 更新交付文档后收口。


