# Markdown 文档「类 Git」版本控制技术方案

> 版本：V1.1（对齐稿，设计未实现）
> 日期：2026-11-26
> 适用对象：WellRead Personal 后端 / Agent / RAG 开发者
> 关联文档：[Agent 功能说明](agent功能说明.md)（本文的 Agent 工作台与风险分级以此为准）
> 目标：为平台内唯一文字性维护格式——Markdown（md）——提供版本控制、Diff 预览、回滚能力，并与 chunk 切分管线、Agent 写操作确认机制（apply_change / confirm_token）衔接。

## 0.1 已对齐决策（2026-11-26）

| 决策点 | 结论 |
|---|---|
| Agent 入口 | 左栏独立「智能体 / Agent」工作台；本文只描述其版本能力 |
| Agent 范围 | 首版为文档编辑 Agent；不覆盖完整 KB 写操作矩阵 |
| 写入确认 | 按风险分级：L1 自动 commit + 撤销；L2/L3 先 dry_run + Diff，confirm_token 后 apply/commit |
| 版本架构 | 自建线性版本链，不引入 git 引擎；版本与业务同事务 |
| 首版范围 | P0：版本表/CAS/commit/读/diff/revert + Agent commit；chunk 指纹复用与增量重切放 P1 |
| 运行环境 | 个人版单 owner；内部 tenant 只作归属，不引入 RBAC、协作者、共享空间或分支合并 |
| 当前状态 | 仅设计文档；本轮不修改业务代码 |

文中保留的「V3 §N.N」是旧需求稿的追溯引用，具体落地以[Agent 功能说明](agent功能说明.md)与本节为准。

---

## 0. 设计总览（TL;DR）

| 决策点 | 结论 |
|---|---|
| 存储模型 | **自建版本链（document + version + commit 链）落业务库**，内容采用「内容寻址 blob（CAS）+ 快照/补丁混合存储」，**不引入 git 引擎** |
| 版本模型 | 简化**线性版本链**（单文档单分支），版本号用单调递增 `seq`；不开放 branch，跨文档整合用「文档血缘（lineage）」表达 |
| Diff | 行级 Myers + patience 优化；**存储采用快照 + patch 混合**：首版及每 N 版存全量快照，中间版本存 unified patch |
| 回滚 | **前向 revert**（回滚 = 生成一个内容等于历史版本的新版本），配合 chunk 指纹复用实现索引近零成本恢复 |
| 切分联动 | chunk 记录 `content_hash + doc_version`，md 更新后按**结构 diff 增量重切**，未变更 chunk 复用向量；回滚时按 chunk 指纹「复活」历史 chunk，不重嵌入 |
| Agent 衔接 | L1 操作执行成功后自动 commit；L2/L3 在 `apply_change` 消费 confirm_token 成功后自动 commit；dry_run 的 Diff 预览 = 版本服务计算的 `diff(current_version, proposed_content)` |
| 并发 | 文档级写锁（沿用 V3 §4.3 续租锁）+ 乐观版本校验（`base_seq` 不符返回 409，Agent/编辑器 rebase 重试） |
| 清理 | 软删除文档 → 冷归档版本 → 快照重压实（re-snapshot compaction），版本数按「时间 + 数量」双阈值控制 |

---

## 1. 存储模型选型对比

### 1.1 候选方案

| 维度 | 方案 A：直接用 git（libgit2 / isomorphic-git） | 方案 B：自建版本表（document + version + commit 链） | 方案 C：事件溯源（Event Sourcing） |
|---|---|---|---|
| 核心思路 | 每个知识库/文档对应一个 git 仓库（或裸仓库 + worktree），md 文件即工作区文件 | 业务库建 `document` / `doc_version` 表，内容按 hash 寻址存储，版本链用 parent 指针表达 | 把「文档的每次变更」建模为不可变事件流，当前内容 = 事件折叠（fold/replay）结果 |
| 事务一致性 | ❌ git 对象库与业务库（权限、审计、pending_task）是两套存储，需双写，难以保证原子性；崩溃恢复复杂 | ✅ 版本写入与业务变更同库同事务 | ✅ 事件表即事实来源，天然一致 |
| 与 chunk 管线联动 | ⚠️ 需自行解析 git commit/diff 得到 chunk 变更；git 的 tree/blob 模型面向「文件树」，单文档多版本映射别扭 | ✅ chunk 表可直接外键到 version，diff 结果结构化可查 | ⚠️ chunk 变更需从事件流推导，需维护投影（projection） |
| Diff/回滚能力 | ✅ git diff/checkout 原生强大 | ✅ 用成熟 diff 库（Myers/patience）自实现，够用且可控 | ⚠️ 回滚 = 追加逆事件，历史重放成本高；diff 需另算 |
| 部署与运维 | ⚠️ libgit2 需原生绑定（CGO/native dep），isomorphic-git 纯 JS 但性能弱、生态小；文档量大时裸仓库数量与 GC 管理复杂 | ✅ 无新组件，复用现有 DB + 对象存储 | ✅ 无新组件，但需自建投影与快照机制 |
| 单 owner 多任务 / 多 Agent 并发 | ✅ git 合并模型成熟，但平台场景是「单文档段落编辑」，merge commit 模型过重 | ✅ 线性链 + 乐观锁 + 写锁即可覆盖（V3 §4.3 已有文档级写锁） | ✅ 天然无冲突写入，但语义冲突仍需业务层解决 |
| 审计与权限 | ⚠️ git 无法承载平台单 owner / confirm_token 语义，需旁路审计表，双份事实 | ✅ 版本表天然带 author/source/pending_task_id，审计单份事实 | ✅ 事件即审计日志 |
| 学习/维护成本 | 中高（团队需懂 git 内部对象模型） | 低（表结构直观） | 高（事件建模、投影、幂等消费） |
| 规模适配 | 知识库文档量级大（数十万级文档 × 高频 Agent 编辑），git 仓库膨胀与 GC 是长期负担 | 版本行数可控（配合 §5 清理策略） | 事件量 = 编辑次数 × 事件类型，折叠开销需持续运维 |

### 1.2 推荐：方案 B（自建版本表），吸收方案 A 的内容寻址思想

**理由：**

1. **一致性压倒一切。** 版本记录必须与 Agent 审计（pending_task_id、confirm_token 消费记录）、chunk 映射、缓存失效事件（V3 §3.5）在**同一个事务**里落库。git 引擎无法参与业务库事务，必然引入双写不一致窗口——而平台的安全模型（L2/L3 确认、审计留痕）恰恰不能容忍「版本已 commit 但审计缺失」这类中间态。
2. **RAG 场景不需要 branch。** git 的价值在并行分支与三方合并；本平台编辑模型是「单文档、段落级、顺序修订」（V3 §4.3 已约束同一文档同一时刻只有一个写者），线性链 + 乐观锁完全覆盖。为不用的能力引入 git 的全部复杂度（仓库管理、GC、对象模型映射）不划算。
3. **事件溯源在本场景是「重锤钉小钉」。** 我们真正需要的不可变事实只有一条：`doc_version`（谁、在哪个版本之上、把内容改成了什么）。这用一张 append-only 的版本表就能表达，不需要通用事件流 + 投影。
4. **吸收 git 最有价值的两点**：① **内容寻址（CAS）**——内容按 `sha256` 存 blob，相同内容天然去重（Agent 反复生成相同段落时零成本）；② **快照 + 补丁的混合存储**——兼顾读取效率与存储成本（见 §2.4）。
5. **保留演进空间**：若未来确需跨文档/跨空间的合并语义（如未来协作者分支编辑），可在版本链之上加「工作副本 + 三方合并」层，无需推翻本模型。

**明确不做 branch 的说明：** 平台需求中的「整合/拆分/移动」等结构操作（V3 §4.1 merge_documents / split_document）通过「文档血缘表」表达（见 §2.6），而不是文档内分支。单人/单 Agent 串行编辑下，分支带来的冲突价值远小于其复杂度。

---

## 2. 数据模型与核心操作

### 2.1 表结构（DDL 示例，PostgreSQL 方言）

```sql
-- 文档主表（在现有 document 表上扩展字段；示意列出与版本控制相关字段）
CREATE TABLE document (
  doc_id          VARCHAR(32) PRIMARY KEY,          -- 如 'doc_x7Fk2a'
  kb_id           VARCHAR(32) NOT NULL,             -- 所属知识库
  title           VARCHAR(256) NOT NULL,
  current_seq     BIGINT NOT NULL DEFAULT 0,        -- 当前版本号（乐观锁依据）
  deleted_at      TIMESTAMPTZ,                      -- 软删除标记（§5.4）
  deleted_by      VARCHAR(64),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_doc_kb ON document(kb_id) WHERE deleted_at IS NULL;

-- 版本表（append-only，禁止 UPDATE/DELETE 业务字段；清理只改 archive 状态，§5.3）
CREATE TABLE doc_version (
  version_id      VARCHAR(40) PRIMARY KEY,          -- 'ver_' || seq || '_' || 短hash，便于人工辨识
  doc_id          VARCHAR(32) NOT NULL REFERENCES document(doc_id),
  seq             BIGINT NOT NULL,                  -- 单文档内单调递增，1 起始
  parent_seq      BIGINT,                           -- 基于的版本；seq=1 时为 NULL
  content_hash    CHAR(64) NOT NULL,                -- sha256(规范化后的 md 全文)
  blob_key        VARCHAR(80) NOT NULL,             -- 内容寻址 blob 键（§2.3）
  storage_kind    VARCHAR(8) NOT NULL,              -- 'snapshot' | 'patch'
  op_type         VARCHAR(16) NOT NULL,             -- create|edit|revert|merge_in|split_out|archive|restore
  author_type     VARCHAR(8) NOT NULL,              -- 'user' | 'agent' | 'system'
  author_id       VARCHAR(64) NOT NULL,             -- 用户ID 或 Agent caller_id
  session_id      VARCHAR(64),                      -- Agent 会话（溯源）
  pending_task_id VARCHAR(64),                      -- L2/L3 关联的待确认任务（审计链，V3 §4.2）
  message         VARCHAR(256),                     -- commit message（Agent 生成的变更摘要 / 用户备注）
  add_lines       INT NOT NULL DEFAULT 0,           -- diff 统计（预览列表展示用）
  del_lines       INT NOT NULL DEFAULT 0,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (doc_id, seq)
);
CREATE INDEX idx_ver_doc ON doc_version(doc_id, seq DESC);

-- 内容 blob（内容寻址，同 hash 只存一份；大文本走对象存储，小文本可内联）
CREATE TABLE content_blob (
  content_hash    CHAR(64) PRIMARY KEY,
  size_bytes      INT NOT NULL,
  inline_text     TEXT,                             -- size ≤ 64KB 时内联，否则为 NULL
  oss_key         VARCHAR(128),                     -- size > 64KB 时对象存储键 'md-blob/{yy}/{hash}'
  ref_count       INT NOT NULL DEFAULT 1,           -- 引用计数，清理时递减（§5.3）
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- patch 存储（storage_kind='patch' 的版本在此存补丁；也可以合并进 blob，此处独立建表便于运维）
CREATE TABLE version_patch (
  version_id      VARCHAR(40) PRIMARY KEY REFERENCES doc_version(version_id),
  base_hash       CHAR(64) NOT NULL,                -- 补丁基底版本内容 hash（防错配）
  unified_diff    TEXT NOT NULL                     -- unified diff 格式，context=3
);

-- 文档血缘（merge/split 等结构操作的表达，替代 branch）
CREATE TABLE doc_lineage (
  id              BIGSERIAL PRIMARY KEY,
  src_doc_id      VARCHAR(32) NOT NULL,
  src_seq         BIGINT NOT NULL,                  -- 操作发生时源文档版本
  dst_doc_id      VARCHAR(32) NOT NULL,
  dst_seq         BIGINT NOT NULL,                  -- 产出文档的版本
  relation        VARCHAR(16) NOT NULL,             -- merged_from | split_from | copied_from
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

> **归属与隔离说明**：版本表按 `doc_id` 归属知识库；个人版单 owner，内部 tenant 只作归属，版本读取接口不做独立权限模型（版本接口沿用现有文档读取守卫）。

### 2.2 版本号设计

- **`seq`**：`document` 内单调递增整数，`current_seq` 即文档当前版本。展示层格式化为 `v{seq}`（如 v42）。
- **`version_id`**：`ver_{seq}_{content_hash前8位}`，便于日志与审计中人工比对。
- **无全局 commit id、无 branch 名**：跨文档操作的溯源靠 `doc_lineage` + `pending_task_id`（同一任务动多份文档时，各文档版本行携带同一 `pending_task_id`，可按任务聚出一组关联 commit——等价于「一次跨文档 commit 集」）。
- **乐观锁**：所有写请求必须携带 `base_seq`（调用方读到的版本）。服务端校验 `base_seq == current_seq`，不符返回 `409 VERSION_CONFLICT`（见 §4.4）。

### 2.3 内容寻址与规范化

为保证 hash 稳定可比，commit 前对 md 做一次**规范化**（normalize，仅用于 hash 与 diff，不影响展示原文）：

1. 行尾统一 `\n`（去除 `\r`）；
2. 去除文末多余空行（保留一个）；
3. 不做语法级转换（不重排标题层级、不改列表符号），保持 diff 语义贴近用户所见。

`content_hash = sha256(normalize(md_text))`。规范化函数与 parser_version 一样纳入配置管理，**升级即触发全量 re-hash 校验任务**（低频，离线）。

### 2.4 diff 算法与存储方式：快照 vs patch 混合

**Diff 算法：**

- 行级 **Myers O(ND)** 为基线，叠加 **patience diff** 启发式（以「唯一行」为锚点，优先对齐标题行 `^#{1,6} ` 等唯一锚），显著降低 md 结构性编辑（改标题/移动段落）产生的噪声 diff；
- 大文件（>2MB）降级为**分段 diff**：先按 md 一级/二级标题切块，只在变化的块内跑行级 diff；
- 统一输出 **unified diff**（context=3），前端渲染两侧对照。

**存储策略（快照 vs patch 切换规则）：**

| 条件 | 存储方式 | 理由 |
|---|---|---|
| seq = 1（首版） | snapshot | 链头必须有完整内容 |
| `seq % 20 == 0` | snapshot | 周期性锚点，限制 patch 回溯深度 |
| 与 parent 的 patch 体积 > 全文体积 × 0.6 | snapshot | 大改时 patch 不划算 |
| patch 应用链长 ≥ 20（即距最近 snapshot 已有 20 个 patch） | snapshot | 控制重建时的补丁重放成本 |
| 其余 | patch | 日常小编辑占绝大多数，省存储 |

**读取重建算法：**

```
content_at(seq):
  v = version(doc_id, seq)
  while v.storage_kind == 'patch':
      base = version(doc_id, v.parent_seq)        # patch 的 parent 必为 snapshot 或更早 patch
      v = base
  text = blob.read(v.blob_key)                    # 从最近 snapshot 出发
  for p in patches(snapshot_seq+1 .. seq):        # 顺序应用 patch（校验 base_hash 链）
      text = apply_unified_patch(text, p)
  return text
```

- 常见读路径（读当前版本、回滚到目标版本、切分管线取内容）优先命中 snapshot 或短链 patch；
- 热文档（近 7 天被读 > 10 次）的内容重建结果写入 Redis 缓存（key = `docver:{doc_id}:{seq}`，随版本变更天然失配，TTL 1h）。

### 2.5 核心接口（内部服务 API）

```
POST   /v1/docs/{doc_id}/versions              # 提交新版本（所有写通道的统一落版本入口）
  body: { base_seq, content, op_type, message, author_type, author_id,
          session_id?, pending_task_id? }
  resp: { ok, data: { version_id, seq, content_hash, diff_stat: {add, del} } }

GET    /v1/docs/{doc_id}/versions              # 版本列表（分页）
  resp: data.items = [{ version_id, seq, op_type, author, message,
                        add_lines, del_lines, created_at }]   # 不含正文，防撑爆

GET    /v1/docs/{doc_id}/versions/{seq}/content     # 取某版本全文（重建算法）
GET    /v1/docs/{doc_id}/diff?from_seq=41&to_seq=42 # 两版本 diff（行级 unified + 结构化 hunks）
POST   /v1/docs/{doc_id}/preview                    # 不落版本的 Diff 预览（dry_run 专用，§4.2）
  body: { base_seq, proposed_content }
  resp: { hunks: [...], diff_stat, target_content_hash }

POST   /v1/docs/{doc_id}/revert                # 回滚（前向 revert，§2.7）
  body: { target_seq, message, author..., pending_task_id? }   # L2 走确认流
```

`POST /versions` 的服务端事务序列（伪代码）：

```python
def commit_version(doc_id, base_seq, content, meta):
    with db.transaction():
        doc = lock_row("SELECT * FROM document WHERE doc_id=%s FOR UPDATE", doc_id)  # 行锁
        assert doc.current_seq == base_seq, VersionConflict(doc.current_seq)          # 乐观校验
        normalized = normalize(content)
        h = sha256(normalized)
        parent = last_version(doc_id)               # parent 内容 hash 用于 diff/patch
        patch = make_unified_patch(parent.content, normalized)
        kind = decide_snapshot_or_patch(seq=parent.seq+1, patch, normalized)          # §2.4 规则
        upsert_blob(h, normalized)                  # content_blob ref_count+1
        v = insert_doc_version(doc_id, seq=parent.seq+1, parent_seq=base_seq,
                               content_hash=h, storage_kind=kind, blob_key=blob_key_of(h), **meta)
        if kind == 'patch': insert_version_patch(v.id, parent.content_hash, patch)
        update document set current_seq = v.seq, updated_at = now()
        outbox.insert(event('doc.updated', {doc_id, seq: v.seq, changed: h != parent.content_hash}))
    # 事务提交后：outbox 由投递器发布 doc.updated，驱动切分管线与缓存失效（§3）
    return v
```

> 关键点：**写锁 + `FOR UPDATE` 行锁 + 乐观校验**三重保险；事件用 outbox 模式保证「版本落库」与「下游事件」同生共死，契合 V3 §3.5 的「写后失效」一致性原则。

### 2.6 op_type 与结构操作的落版本规则

| op_type | 触发场景 | 落版本行为 |
|---|---|---|
| create | create_document | seq=1，snapshot |
| edit | edit_document_section / 编辑器保存 / 用户手动编辑 | 常规 commit |
| revert | 回滚（§2.7） | 内容 = 目标历史版本，parent 仍是当前最新版本（**不改写历史**） |
| merge_in | merge_documents 确认后 | 目标文档新版本（内容=整合稿）；**各源文档**生成 `archive` 版本或直接软删除，血缘表记 `merged_from` |
| split_out | split_document 确认后 | 每个子文档 seq=1（内容=对应章节），血缘表记 `split_from`；源文档可选生成 archive 版本（删除尾部章节的 diff）或软删除 |
| archive / restore | 归档与恢复 | archive 不改内容，仅标记；恢复时如内容缺失可从血缘源重建 |

### 2.7 回滚实现：前向 revert（推荐），指针回退不采用

**为什么前向 revert：**

1. **历史不可变**是审计与 L1「执行后可撤销」（V3 §1.3/§5.2）的地基；指针回退会丢弃中间版本，破坏「删除、归档必须留痕」的要求；
2. 与 chunk 管线天然兼容——回滚产生一个「新版本」，可以完全复用「更新 → 增量重切」链路（§3），不需要为回退写一套独立的索引恢复逻辑；
3. 前端展示统一：「回滚到 v38」在版本列表里就是 v43（op_type=revert，message="回滚至 v38：xxx"），用户心智一致。

**实现：**

```python
def revert(doc_id, target_seq, actor, pending_task_id=None):
    target_text = content_at(doc_id, target_seq)          # 重建历史内容
    return commit_version(doc_id, base_seq=current_seq(doc_id), content=target_text,
                          op_type='revert', message=f'回滚至 v{target_seq}',
                          pending_task_id=pending_task_id)
```

**风险等级映射**：回滚本身定为 **L2**（结构性影响：chunk 大范围变化、检索结果变化），走 dry_run → Diff 预览（diff(current, target_text)）→ confirm_token → `apply_change` 执行。L1 编辑的「撤销按钮」（V3 §5.2 第 4 条）在实现上是「一键 revert 到上一版本」的快捷封装，因目标内容确定且范围单文档，确认交互可简化为通知卡片内直接确认（确认事件仍由用户真实点击产生，token 仍由确认服务签发）。

---

## 3. 与切分（chunk）管线的联动

### 3.1 chunk 数据模型扩展

```sql
CREATE TABLE chunk (
  chunk_id      VARCHAR(40) PRIMARY KEY,
  doc_id        VARCHAR(32) NOT NULL,
  born_version  BIGINT NOT NULL,        -- 首次出现的文档版本 seq
  last_version  BIGINT NOT NULL,        -- 最近一次内容不变的版本 seq（随重切滚动更新）
  chunk_index   INT NOT NULL,           -- 在当版文档中的序号
  section_path  VARCHAR(512),           -- 结构树路径，如 'H1[2]/H2[1]'（V3 §2.2 元数据）
  content_hash  CHAR(64) NOT NULL,      -- sha256(chunk 文本)，指纹复用的关键
  char_count    INT NOT NULL,
  embedding_key VARCHAR(80),            -- 向量库中的向量记录键（按 content_hash 寻址）
  status        VARCHAR(12) NOT NULL,   -- active | superseded | orphan
  UNIQUE (doc_id, born_version, chunk_index)
);
CREATE INDEX idx_chunk_active ON chunk(doc_id) WHERE status = 'active';
CREATE INDEX idx_chunk_hash ON chunk(doc_id, content_hash);
```

向量库侧：**向量记录以 `content_hash` 为逻辑键**（`emb:{content_hash}`），同一段文本无论在哪个版本、哪个 chunk_index 出现，向量只有一份——这是「回滚近零成本恢复」的基础。

### 3.2 md 更新后的增量重切流程

由 §2.5 的 `doc.updated` 事件驱动（消费端幂等，按 `(doc_id, seq)` 去重）：

```
输入: doc_id, new_seq
1. old_chunks = active chunks of doc_id（取自上一版本）
2. new_chunks = chunker.split(content_at(doc_id, new_seq))   # 结构感知切分（按标题层级+语义边界）
3. 对齐: 对 new_chunks 逐个计算 content_hash，与 old_chunks 按 hash 匹配
   ├─ hash 命中 → chunk 复用：更新 chunk_index/section_path/last_version，向量零操作
   └─ 未命中    → 进入候选变更集
4. 对候选变更集做局部校准：
   - 边界漂移检测：若新旧 chunk 文本编辑距离 < 阈值（如 ≤15% 字符变化），
     判定为「同 chunk 微改」，保留 chunk_id 重嵌入该块（保证 chunk_id 稳定，利于引用回链）；
   - 纯结构变化（章节合并/拆分/移动）→ 按新结构生成新 chunk_id，旧 chunk 置 superseded
5. 事务落库：更新/插入/状态翻转 chunk + 写 doc_chunk_map(doc_id, seq, chunk_id) 快照映射
6. 嵌入：仅对「内容 hash 首次出现」的 chunk 调 embedding（emb:{hash} 已存在则跳过写入，只更新引用）
7. 发布 doc.chunks.updated → 失效 C2 检索缓存（该 doc 关联）、C4 知识产物缓存（V3 §3.5）
```

**版本 ↔ chunk 映射**：`doc_chunk_map` 记录「某版本下的 chunk 集合」，使平台可回答：

- 「这段引用（V3 §2.2 要求携带 version）当时对应哪个 chunk」→ 按 `doc_id + version + section_path/chunk_index` 查；
- 「v38 的检索快照」→ 直接用该版本的 chunk 集 + 各 chunk 的 embedding_key（向量仍在）。

### 3.3 回滚后的索引恢复策略（核心：指纹复活，不重嵌入）

回滚到 v38 时，`revert` 产生的新版本内容 `content_hash` 与 v38 相同，因此：

```
增量重切流程照常执行（§3.2），但步骤 3 的 hash 匹配会发生：
- 新版本各 chunk 的 content_hash 几乎全部能在 chunk 历史表（含 superseded 记录）中命中
- 命中 → 直接把历史 chunk 复活（status: superseded → active，chunk_id 保持不变，
  born_version/last_version 更新），embedding_key 不变 → 向量库零写入、零删除
```

**效果**：回滚的索引恢复成本 ≈ 一次 DB 批量 UPDATE，P95 目标 < 1s；只有「回滚目标版本从未被切分过」的极端情况（理论上不会发生，因为成为版本即触发过切分）才需要全量重嵌入。

**对照策略（不采用纯重建）**：直接对回滚后全文全量重切 + 重嵌入的实现更简单，但一次回滚 = 全文档 embedding 重算，成本高且产生大量向量库写放大。指纹复活方案只在增量重切流程上加一条「历史 hash 命中即复活」的规则，改动小、收益大，推荐采用。

### 3.4 失败与一致性

- 事件消费失败：重试 5 次指数退避后进入死信队列，文档标记 `chunk_lag`，检索层对该文档降权并告警（宁缺毋旧——对齐 V3 §3.5「宁可不命中，不可返回旧知识」）；
- chunk 状态与版本的一致性以 `doc_chunk_map` 为准：检索层只信任 active 且 `last_version ≤ current_seq` 的 chunk；
- **parser_version 升级**：沿用 V3 §3.4 双版本并行 + 惰性迁移；注意升级会改变 chunk 边界导致指纹大面积失配，此时按知识库为单位灰度重切，不复用跨 parser 版本的指纹。

---

## 4. 与平台现有机制的衔接

### 4.1 Agent 写操作如何落版本

原则：**所有写通道（用户编辑器、Agent 工具、第三方 API）都收敛到 §2.5 的 `POST /versions`，版本服务是唯一 commit 入口**；差异只在上游的确认流程。

| 通道 / 风险级（V3 §1.3） | 落版本时机 | Diff 预览来源 | 审计字段 |
|---|---|---|---|
| 用户编辑器手动保存（L1） | 保存即 commit（`base_seq` 冲突时走 §4.4 rebase 流） | 编辑器本地 diff | author_type=user |
| Agent `edit_document_section`（L1） | 工具执行成功后**自动 commit**，前端通知卡片带撤销入口（= revert 快捷流） | 执行前计算好，随通知展示摘要 | author_type=agent, session_id |
| Agent `create_document`（L1） | 执行成功后 commit seq=1 | —（新建无对照） | 同上 |
| L2/L3（merge/split/revert/删除覆盖等） | `apply_change` 校验 pending_task_id + confirm_token 通过、执行成功后**才 commit** | **dry_run 的 Diff 预览直接复用版本服务 `POST /preview`**，保证「预览所见 = commit 所得」 | pending_task_id 必填，写 doc_version 行 |

**diff 即预览的实现要点：**

1. L2/L3 工具 dry_run 时，Agent 产出 `proposed_content`（如整合稿全文），版本服务 `POST /preview` 计算 `diff(current_version, proposed_content)` 返回结构化 hunks——与最终 commit 用的 diff 是**同一函数同一输入**，杜绝「预览与实际不符」；
2. preview 结果摘要（diff_stat + 目标 content_hash）写入 `pending_task.payload`；`apply_change` 执行时**校验最终内容 hash 与 preview 的 target_content_hash 一致**（Agent 在等待确认期间不得静默改稿；改稿需重新 dry_run，pending_task 作废重建），保证确认对象与执行对象严格一致；
3. confirm 链路完全沿用 V3 §4.2：用户确认事件 → 确认服务签发 token（HMAC，绑定 pending_task_id + 用户 + 10 分钟）→ `apply_change` 一次性消费 → 执行 → commit（携带 pending_task_id）→ 审计留痕。版本系统**不引入新的确认语义，只是确认链路的最终落点**。

### 4.2 版本信息与 RAG 元数据的贯通

V3 §2.2 要求召回片段携带 `{doc_id, doc_title, section_path, version, score, updated_at}`。落地映射：

- `version` → chunk 所在 `doc_chunk_map.seq`（或 chunk.last_version）；
- `updated_at` → 该版本 `doc_version.created_at`；
- 引用回链（前端跳转「文档对应章节」）→ `doc_id + seq + section_path` 锚点，历史版本的引用也可回链到版本快照页。

### 4.3 单 owner 多任务 / 多 Agent 并发编辑冲突处理

分两层：

**第一层：串行化（防物理覆盖）**——沿用 V3 §4.3 文档级写锁（30s 续租 / 60s 无续租回收），保证同一时刻只有一个写事务在途。

**第二层：乐观版本校验（防逻辑丢失更新）**——写请求必须携带 `base_seq`：

- `base_seq == current_seq` → 放行；
- `base_seq < current_seq` → `409 VERSION_CONFLICT`，响应携带 `current_seq + current 内容（或 diff）`，调用方 rebase：
  - **Agent**：把冲突信息作为工具错误返回（结构化信封，V3 §4.2），Agent 自行决定「基于最新版重算编辑 / 提示用户人工裁决」；Agent 的重试天然带 step_id 幂等键，不会产生重复 commit；
  - **编辑器**：弹三方对照（用户稿 vs 最新版），提供「以最新版为基础重放我的修改」；
- **段落级微并发优化（P1 可选）**：`edit_document_section` 若编辑区间与当前版本的实际变更区间（由 hunks 计算）**不重叠**，可自动 rebase 合并（把 section 内容替换到最新版文本上再 commit），冲突率大幅下降；重叠则必须人工裁决。该优化依赖「编辑不重叠 → 语义可合并」的假设，md 结构编辑下基本成立，作为渐进增强。

**多 Agent 同时操作同一文档**：被写锁互斥排队；若同一任务（同一 pending_task_id 组）内多步编辑，建议 Agent 在一步内合并为单次 commit，减少版本碎片。

### 4.4 L1 撤销的完整闭环

```
Agent L1 编辑 commit 成功 → 通知卡片（含 diff 摘要 + 撤销按钮）
  → 用户点击撤销（真实确认事件）→ 确认服务签发一次性 token（短有效期 5min，绑定撤销意图）
  → 撤销执行 = revert(doc_id, target_seq=current_seq-1) → 新版本 op_type=revert
  → chunk 指纹复活（§3.3）→ 撤销动作写审计
```

---

## 5. 并发、性能与清理策略

### 5.1 性能预算（对齐 V3 §7.1 的量级感）

| 操作 | 目标 | 依赖优化 |
|---|---|---|
| 读当前版本 | P95 ≤ 50ms | current_seq 行内即可取 blob 直读；热文档 Redis |
| 读历史版本（重建） | P95 ≤ 150ms | snapshot 锚点限深 ≤20 patch；热版本缓存 |
| 两版本 diff | P95 ≤ 100ms（≤1MB 文档） | Myers+patience；大文件分段 |
| commit（含乐观校验） | P95 ≤ 120ms | 单事务、blob 现算现存 |
| 增量重切（小编辑） | 异步，P95 ≤ 5s 端到端失效 | 指纹复用，绝大多数 chunk 零嵌入 |
| 回滚索引恢复 | P95 ≤ 1s | 指纹复活（§3.3） |

### 5.2 并发控制汇总

- 写：文档行锁（事务级）+ 写锁（跨事务，续租）+ `base_seq` 乐观校验，三层缺一不可；
- 读：无锁快照读（append-only 版本表天然适合 MVCC）；
- 切分管线：同一 `(doc_id, seq)` 事件幂等消费；同一 doc 的重切事件按 seq 顺序串行消费（单 doc 并发重切无意义），跨 doc 并行。

### 5.3 版本数量控制与打包归档

**双阈值保留策略（默认值可按知识库配置）：**

| 规则 | 默认值 | 说明 |
|---|---|---|
| 热保留期 | 近 90 天全部版本保留 | 全部可 diff/可回滚/可查 |
| 数量上限 | 单文档 > 500 版触发压实 | Agent 高频编辑场景（V3 §4.4 限流下单用户 20 次/分钟，可触达） |
| 冷归档 | 热保留期外版本 → 冷存储 | 见下 |

**压实（compaction）：**

```
对超出保留阈值的连续版本段 [a..b]：
1. 计算 content_at(a)、content_at(b)（在热存储中完成重建）
2. 用「新 snapshot(a) + 单条合并 patch(a→b)」替换原段落的存储形态，
   version 元数据行保留（历史列表不缺行），但 storage_kind 重标为 'archived'
   ——版本元数据永久保留，重内容合并为段首快照 + 段尾补丁
3. patch 链中间 blob 引用计数递减，ref_count=0 的 blob 转对象存储冷层（低频介质）或按策略删除
4. archived 版本仍支持「取内容」（重建路径不变，只是 patch 链合并）与「回滚」（内容重建后前向 revert）；
   仅「逐版本 diff」在段内受限（返回段合并 diff），产品上对超期版本提示此限制
```

**打包归档（批量场景）：** 知识库整体归档时，把全部版本重建成最终快照 + 版本清单（JSONL：seq/hash/author/op_type/时间/任务ID）打成单个归档包（如 zstd 压缩）存对象存储，热库只留 `doc_version` 元数据行 + 归档包指针；恢复时按包回放。适用于下线项目空间、法务留存等场景。

### 5.4 软删除

- 删除文档（L3，走 `apply_change`）→ `document.deleted_at` 置位，**版本数据一律保留**（对齐 V3 §1.2「不可覆盖历史版本」）：
  - 索引侧：该 doc 全部 chunk 置 `orphan`（不删除，保留向量），检索层硬过滤 orphan（V3 §6.5 第 3 条同样采用「召回时排除」而非物理删除）；
  - C2/C3/C4 按 V3 §3.5 失效；
- 恢复（restore）：`deleted_at` 清空，orphan chunk 批量复活为 active（指纹机制同 §3.3，零重嵌入）；
- **物理清除仅两个入口**：① 用户在回收站对已软删除文档执行「彻底删除」（L3 二次确认 + 审计），删除 `doc_version` 行与 blob（blob ref_count 减至 0 才物理删）；② 合规删除请求（保留 30 天缓冲期）。均不可恢复，操作前建议自动生成归档包（§5.3）留存 180 天以对齐审计保留期。

### 5.5 监控指标（并入 V3 §7.2 面板）

| 指标 | 告警阈值（初始） |
|---|---|
| commit 失败率 / VERSION_CONFLICT 率 | 失败率 > 1%；冲突率 > 10%（说明 Agent rebase 策略需调优） |
| chunk 重切积压（lag 事件数） | > 100 持续 10min |
| 指纹复用率（重切时复用 chunk 占比） | < 60% 日级（异常偏低说明 diff 边界漂移检测需调参） |
| 版本重建 P95 | > 300ms |
| blob 存储增长速率 | 超预算 80% 预警（触发压实参数复核） |

---

## 6. 分期落地建议

| 阶段 | 范围 |
|---|---|
| **P0（随平台 MVP，4~6 周内）** | 版本表/blob/patch 存储 + commit/读/diff/revert 接口；L1 自动 commit + 撤销快捷 revert；L2/L3 经 apply_change 落版本（含 pending_task_id 审计链）；乐观锁 409 + Agent rebase 错误信封 |
| **P1（+4 周）** | chunk 指纹复用 + 增量重切 + 回滚索引复活全链路；doc_chunk_map 与 RAG 元数据 version 贯通；段落级自动 rebase；压实与保留策略 |
| **P2（+6 周）** | 冷归档打包、物理清除合规流；版本重建与 diff 的性能调优收官；第三方通道（kb.document.edit）复用同一 commit 入口与冲突语义 |

> 首版（P0）不实现 chunk 指纹复用、增量重切、归档压实与多人协作文档模型；这些能力在 P1/P2 接入，不阻塞 Agent 工作台的版本列表、Diff 与回滚。当前仅出设计，不落代码。

---

## 附录 A：一次完整链路示例（Agent 整合两篇文档）

```
1. Agent 调 merge_documents(dry_run=true) → 产出整合稿 proposed_content
2. 版本服务 POST /preview → diff(current v12, proposed) → hunks + target_content_hash
3. pending_task 建立（awaiting_confirm），payload 存 diff 摘要 + target_content_hash
4. 用户在 E2 向导确认 → 确认服务签发 confirm_token（10min 有效）
5. Agent 调 apply_change(pending_task_id, confirm_token)
   → token 校验通过 → 执行整合 → POST /versions commit（op_type=merge_in, pending_task_id 落库）
6. 事务内更新 document.current_seq → v13；outbox 发布 doc.updated
7. 切分管线消费事件 → 增量重切 → 变更 chunk 重嵌入、其余复用 → 失效 C2/C4
8. 源文档软删除，doc_lineage 记 merged_from；检索层过滤
9. 用户在 E6/通知卡片发现整合稿不理想 → 「回滚本次整合」
   → revert 预览（L2 确认）→ v14 = v12 内容（op_type=revert）
   → chunk 指纹复活，索引秒级恢复
```

## 附录 B：关键边界情况清单

| # | 边界情况 | 处理 |
|---|---|---|
| 1 | 提交内容与当前版本内容完全相同（hash 相等） | 不产生新版本，返回 ok + `{seq: current_seq, no_change: true}`，防 Agent 空转刷版本 |
| 2 | pending_task 等待期间文档被他人编辑 | apply_change 执行前重校验：文档 current_seq 与 dry_run 时不一致 → 任务失效，要求重新 dry_run（保证确认对象即执行对象） |
| 3 | 回滚目标版本处于 archived 段 | 内容重建路径不变（§2.4），可正常 revert；diff 预览若跨压实段返回合并 diff 并提示 |
| 4 | parser_version 升级后回滚 | chunk 指纹仅在相同 parser_version 内复用；跨版本回滚触发该文档全量重切（灰度队列） |
| 5 | blob 引用计数并发竞态 | ref_count 变更与版本写入同事务（对 blob 行 FOR UPDATE）；压实/清理为单实例定时任务，天然串行 |
| 6 | 版本列表被 Agent 拉取撑上下文 | 列表接口强制分页（limit ≤ 20，V3 §4.2 规范），默认只返回统计不返回正文 |
| 7 | 第三方 L1 编辑并发 | 与平台内同语义：409 + rebase；第三方无 L2（V3 附录 C-4），不涉及确认链 |
