# CLAUDE.md — AI Agent 操作指南

本文件面向在本仓库工作的 AI Agent。术语、身份与访问规则见[术语与约定](docs/01-项目总览/术语与约定.md)与[个人版账号与访问](docs/02-需求与设计/角色与权限/个人版账号与访问.md)。

## 1. 仓库形态

WellRead Personal 是单 owner、本地部署的个人知识库系统，根目录为 monorepo：

| 目录 | 职责 |
|---|---|
| `app/` | Go 后端、docreader、CLI、Go SDK、Swagger、迁移 |
| `frontend/` | Vue 3 SPA；五项目导航与五分区设置 |
| `rag/` | Flask 检索/生成侧车；语料/模型/索引在 `data/rag/`（gitignore） |
| `mcp/` | MCP 外部接入层；17 个只读工具 |
| `deployments/` | 六服务 compose、hotreload overlay、systemd、env example |
| `scripts/` | 跨服务构建/运维脚本 |
| `docs/` | 01 总览、02 需求与设计、03 进度、04 历史归档、05 演示 |

本地运行数据在仓库内 `data/rag/`（gitignore），不进入 git、不进入镜像。

## 2. 开工前阅读顺序

1. [当前进度](docs/03-进度与交接/当前进度.md)：当前阶段、待办、验证基线、风险。
2. [系统总览](docs/01-项目总览/系统总览.md)：服务生态、身份、问答/入库/精读链路。
3. 与任务对应的 `docs/02-需求与设计/` 文档。
4. 涉及认证/外部接入：[个人版账号与访问](docs/02-需求与设计/角色与权限/个人版账号与访问.md)。
5. 历史归档只用于追溯，不作为当前实现依据。

## 3. 硬性规则

- **后端是安全边界**：前端隐藏不是授权；登录态、owner 身份与单 Key 路由白名单最终由后端判定。
- **单 owner**：没有角色、部门、能力契约、共享空间、ACL 或协作权限；不要重新引入 `requiredCaps`、角色 if-else 或多账号概念。
- **内部 tenant 只作归属**：用户不可见、不可切换；不要新增多租户/工作区功能。
- **RAG 只负责索引与检索**：`/ingest` 必须携带 App 生成的 chunks；不要在 RAG 重新加文件解析、分块或元数据抽取。
- **检索不带权限过滤**：RAG 请求不接收 `acl_scope`、部门、密级；不要恢复 fail-closed scope 逻辑。
- **精读与技能隔离**：精读固定 `grill-with-docs`；技能只用于普通问答；两者不共享可执行工具。
- **MCP 只读**：工具面固定 17 个只读工具；不要重新加写入、删除、建库、模型、租户或共享空间工具。
- **密钥不落库/不落文档**：`.env`、模型 Key、数据库口令、owner 初始密码不提交；单 Key 只显示掩码，明文只在生成/重置时出现一次。
- **RAG 数据不进入 git/镜像**：不把 `Data/`、`models/`、`faiss_index.bin` 加进 git；本地数据放 `data/rag/`，不要删除。
- **文档来源**：产品/需求文档只从 app 业务侧与当前产品文档获取；`rag/test/`、`data/rag/Data` 是独立测试数据，不能当作业务需求或演示数据。
- **只推个人仓库**：origin = `git@github.com:Forward-back/Well-Reard.git`；不要添加或推送其他上游；提交前确认不含 `.env`、密钥、`data/rag/`、`backups/`。
- **高风险动作先确认**：删除/移动大量文件、重置数据库、重建数据卷、轮换凭据、修改部署与 CI，先向用户说明影响。

## 4. 常用命令

```bash
make up / down / restart / ps / logs
make dev / dev-logs / dev-stop
make test-app / test-frontend / test-rag / test-mcp / test
make images / package-lite
make migrate-up / migrate-down
make health
```

- 前端开发：`cd frontend && npm run dev`，`/api` 代理到 6152。
- RAG 语法：`cd rag && python -m compileall -q src`。
- MCP 测试：`cd mcp && uv sync --extra test && uv run pytest -q`。
- Swagger：`cd app && GOSUMDB=off go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go -o ./docs --parseDependency --parseInternal`，然后 `go test ./docs/`。
- 后端测试：Linux 容器内 `go test ./internal/... ./docs/... -count=1`；Windows 本机可能因 duckdb/gojieba CGO 失败，记录为环境限制，不要为此改业务代码。

## 5. 路径约定

- compose 项目名固定 `app`，沿用原命名卷（`app_postgres-data` 等）。
- 构建上下文：app/docreader `../app`；frontend `../frontend`；rag `../rag`。
- 前端镜像不再使用 contracts named context。
- RAG 挂载：`../data/rag:/app/external`（相对于 `deployments/`）；`RAG_DATA_DIR=/app/external/Data`、`LOCAL_MODELS_DIR=/app/external/models`；FAISS 索引写 `dirname(DATA_DIR)/faiss_index.bin`。
- 根 `scripts/` 只放跨服务脚本；Go 构建脚本在 `app/scripts/`；前端脚本在 `frontend/scripts/`。

## 6. 文档规范

- 文档状态只用：`当前`、`优化中`、`重构中`、`规划`、`历史`；日期只写在文档头。
- 当前文档名不带日期、Phase、批次；阶段记录放 `docs/03-进度与交接/阶段记录/`。
- `docs/` 下不放 README；根 `README.md` 是唯一文档总入口。
- 每份文档一个主题；产品/功能文档写“为什么用、怎么用、业务规则、界面意图、差距、验收”，不复制 API 字段/数据库表结构。
- 账号与访问规则只维护一份，其他文档引用[个人版账号与访问](docs/02-需求与设计/角色与权限/个人版账号与访问.md)。
- 历史归档保留原貌；需要时加“历史快照/旧路径对照”，不按当前口径重写。
- 文档与代码冲突时：以当前代码/运行结果为准，把差异写入对应文档的“当前实现与差距”，不要按旧文档直接改代码。

## 7. 改动流程

1. 读当前进度与对应 02 文档，确认任务属于当前阶段。
2. 涉及认证/外部接入：先改后端路由白名单与资源守卫，再改页面；补 401/403 测试。
3. 涉及检索：确认请求不带权限参数，App 负责解析/分块，RAG 只做索引与检索。
4. 涉及精读/技能：保持 `grill-with-docs` 隔离；技能只进普通问答。
5. 涉及文档：更新对应功能文档的“当前实现与差距”，需要时更新阶段记录，不新建重复文档。
6. 验证后记录命令、结果与限制；未验证的不要写成已验证。

## 8. 相关入口

- [系统总览](docs/01-项目总览/系统总览.md)
- [术语与约定](docs/01-项目总览/术语与约定.md)
- [个人版账号与访问](docs/02-需求与设计/角色与权限/个人版账号与访问.md)
- [当前进度](docs/03-进度与交接/当前进度.md)
- [部署手册](deployments/README.md)
- [RAG 文档](rag/README.md)、[MCP 文档](mcp/README.md)
- [app/README](app/README.md)、[frontend/README](frontend/README.md)
