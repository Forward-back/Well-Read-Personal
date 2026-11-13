# 检索 ACL 落地

> 状态：历史
> 日期：2026-10-24
> 关联：[共享空间与ACL](../../02-需求与设计/角色与权限/共享空间与ACL.md)、[当前进度](../../03-进度与交接/当前进度.md)

> 本文是 2026-10-24 的阶段快照；当前 ACL 规则以共享空间与 ACL 文档为准。

## 1. 设计决策

### 1.1 scope 形态

- `mode`：`all | allow | deny`。
- `document_ids`：allow 模式为可读文档集合，deny 模式为排除集合；all 模式为空。
- `scope_version`：租户 ACL epoch。
- v1 App 采用最保守策略：KB 存在任何 ACL 规则时一律发送 `mode=allow`，携带 `Scope` 计算出的可读文档 ID；无规则时发送 `mode=all`。这样不依赖“允许集小于排除集”的启发式判断，先保证正确性。

### 1.2 fail-closed

- YLSW 在 `/chat/stream`、`/search` 收到缺失或非法 `acl_scope` 时：
  - `/search` 返回空结果；
  - `/chat/stream` 返回错误事件，不进入检索与生成。
- App 必须始终传入 scope；无法计算 scope 时不调用引擎。
- allow 模式下，无法映射 document_id 的候选（如 SQL 表格候选）一律丢弃。

### 1.3 过滤位置

- FAISS/BM25/meta/SQL 召回后先按 document_id 过滤，再进入 RRF 融合；避免被拒绝文档参与融合与 rerank。
- 规划器多步路径在 planner 候选合并后过滤。
- 邻域补全与引用组装前再过滤一次，防止邻域扩展重新带回被拒文档。
- App 收到 references 后按同一 scope 复核 knowledge_id，不一致项丢弃并记录降级日志。

### 1.4 审计

- Phase 4 先落 `retrieval_audit_logs` 基础表与 App 侧 best-effort 写入：
  tenant、user、principal_kind、query 摘要、KB、scope 摘要、命中文档、错误。
- 写入失败不阻断检索；异步批量与保留期策略后续补。

## 2. 开发进度

- [x] App ACLScope 类型与构建：`types.ACLScope`、`KBAclService.RetrievalScope`（无规则 all；有规则 allow；空 KB fail-closed）
- [x] App 引擎协议透传：`RAGEngine.StreamChat/Search` 增加 `aclScope`；YLSW client 发送 `acl_scope` 并在客户端复核 references
- [x] 调用方覆盖：会话问答、知识搜索、Agent knowledge_search、门户、评测、IM /search、检索测试
- [x] YLSW acl_scope 解析与过滤：`/chat`、`/chat/stream`、`/search` 缺失/非法 fail-closed；FAISS/BM25/meta 分支过滤 + 融合后过滤 + rerank/邻域后最终复核
- [x] 检索审计：`retrieval_audit_logs` 迁移 + `engine.NewYLSWEngineWithAudit` 装饰器 + best-effort 异步写入
- [x] 验证与清理

## 3. 验证记录

- App `go build ./internal/...` 通过；YLSW `python -m py_compile src/app.py` 通过。
- YLSW 镜像重建并重启（`gaze-ylsw:latest`）。
- `/search` 缺失 `acl_scope` → `{"results": []}`；`/chat/stream` 缺失 → HTTP 400 `acl_scope is required`。
- `/search` allow 空集合 → 0 条；allow 单文档 → 10 条且 `source_doc_id` 全部为该文档。
- `/chat/stream` allow 单文档 → 11 条引用且全部为该文档；allow 空集合 → 0 引用。
- App `hybrid-search`：给单个文档加 file deny 后，`scope_mode=allow`、`scope_doc_count=119`，该文档不再出现，其余文档仍返回 10 条；撤销后恢复。
- `retrieval_audit_logs` 写入 `all`/`allow`、scope version、doc count、decision（ok/empty）与命中文档 JSON。
- 测试产生的 ACL 规则已删除；无新增测试数据。

