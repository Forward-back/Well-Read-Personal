# WellRead App（Go 后端）

WellRead 个人版的 Go 后端：HTTP API、知识库与文档、检索编排、问答/精读/技能、Agent 运行时、MCP 单 Key、设置、数据源同步、CLI、Go SDK、数据库迁移与 Swagger。

个人版是单 owner、单内部工作区：没有部门、角色能力契约、共享空间、ACL、协作 CR 或治理台。前端只负责展示与交互，认证与授权最终由后端判定。

## 1. 模块边界

| 归属 | 内容 |
|---|---|
| `app/` | API、会话、知识库/文档、检索、精读、技能、设置、CLI、SDK、docreader 客户端 |
| `frontend/` | Vue 3 SPA；只消费后端 API，不加载能力契约 |
| `rag/` | 检索与生成侧车；App 通过 `/ingest`、`/search`、`/chat/stream` 调用 |
| `mcp/` | 外部 MCP 接入层；使用单只读 Key |
| `deployments/` | 六服务 compose、环境变量、systemd |

`app/` 不负责前端渲染、RAG 索引构建或外部部署编排。

## 2. 目录地图

```text
app/
├── cmd/server/              # HTTP 服务入口与 owner 初始化
├── internal/
│   ├── agent/               # Agent 运行时、技能、工具策略
│   ├── application/         # 应用服务与仓储
│   ├── datasource/          # 外部数据源同步框架
│   ├── engine/              # RAG 引擎客户端
│   ├── event/               # 进程内事件总线
│   ├── handler/             # HTTP handlers
│   ├── infrastructure/      # 文档解析、分块等基础设施
│   ├── mcp/                 # MCP 客户端与工具管理
│   ├── middleware/          # JWT / API Key / 错误处理
│   ├── modelcontext/        # 模型上下文句柄与引用编解码
│   ├── models/              # 模型 provider 与请求适配
│   ├── router/              # 路由与 API Key 白名单
│   ├── sandbox/             # Docker sandbox 运行时
│   └── types/               # 领域类型与接口
├── cli/                     # WellRead CLI（独立 Go module）
├── client/                  # Go SDK
├── migrations/              # PostgreSQL / SQLite 单基线迁移
├── docreader/               # 文档解析 gRPC 服务
├── config/                  # 业务配置与提示词
├── docker/                  # app/docreader 镜像
├── scripts/                 # Go/构建/迁移/anydoc 脚本
├── docs/                    # Swagger 生成物（不是人工文档）
├── licenses/                # 分发许可
└── third_party/             # anydoc Go/Rust 绑定
```

## 3. 子系统文档

| 子系统 | 文档 | 说明 |
|---|---|---|
| CLI | [cli/使用说明.md](cli/使用说明.md) | 安装、profile、KB/文档、搜索、问答、JSON/退出码 |
| Go SDK | [client/使用说明.md](client/使用说明.md) | 客户端初始化、KB/文档、会话、流式问答、模型与分块 |
| DocReader | [docreader/服务说明.md](docreader/服务说明.md) | 解析服务、gRPC、环境变量、PDF/表格/OCR |
| 数据源同步 | [internal/datasource/数据源同步框架.md](internal/datasource/数据源同步框架.md) | Connector 架构、任务流、数据模型、API |
| 连接器开发 | [internal/datasource/连接器开发指南.md](internal/datasource/连接器开发指南.md) | 新平台接入步骤、OAuth、分页、增量、删除同步 |
| 事件总线 | [internal/event/事件系统.md](internal/event/事件系统.md) | 进程内事件、处理器、中间件、集成方式 |
| 模型上下文 | [internal/modelcontext/模型上下文边界.md](internal/modelcontext/模型上下文边界.md) | 模型句柄、引用、MCP 路由、解码边界 |
| 云镜像脚本 | [scripts/cloud-image/云镜像部署.md](scripts/cloud-image/云镜像部署.md) | 旧云镜像脚本说明；当前主部署见 `deployments/` |

## 4. 常用命令

```bash
cd app
make build             # 构建后端
make test              # Go 测试
make docs              # 重新生成 Swagger
make anydoc-lib        # 构建 anydoc 静态库
make build-anydoc      # 带 anydoc 引擎构建
```

Windows 本机可能因 duckdb/gjieba CGO 链接或初始化失败；完整测试在 Linux 容器内执行：

```bash
go test ./internal/... ./docs/... -count=1
```

## 5. 配置

- 全栈环境变量：`../deployments/.env`，示例 `../deployments/.env.example`。
- 首次启动必须配置 `OWNER_EMAIL` / `OWNER_PASSWORD`；缺失或密码不合规会启动失败。
- app 业务配置：`config/config.yaml`（compose 绑定挂载到容器）。
- 密钥、模型凭据、DB 密码不入库；由 env 注入。

## 6. 认证与权限

- 登录只支持邮箱 + 密码；注册、邀请、OIDC 与工作区切换已删除。
- JWT 会话用于前端；单只读 API Key 用于 MCP/外部接入，在「设置 → 外部接入」管理。
- API Key 路由使用显式白名单，未声明的路由默认拒绝。
- 没有部门、角色能力码、共享空间、ACL 或协作权限模型。

## 7. 数据库与迁移

- PostgreSQL 迁移：`migrations/versioned/` 的单个 `000000_init` 基线。
- SQLite 迁移：`migrations/sqlite/000000_init`，业务表/列与 PostgreSQL 对齐（`embeddings` 为 PostgreSQL 专用）。
- 个人版要求全新数据库，不提供企业库原地升级。
- 命令：`scripts/migrate.sh up|down|version|force`；`AUTO_MIGRATE=true` 时启动自动迁移。

## 8. API 与 Swagger

- API 前缀：`/api/v1`；Swagger UI：启动后 `/swagger/index.html`。
- `docs/docs.go`、`docs/swagger.json`、`docs/swagger.yaml` 由 `swag init` 生成，**不要手改**。
- `docs/swagger_contract_test.go` 校验生成物：包含 `/knowledge-search`、不残留 `/sessions/search`、`DELETE /models/{id}` 记录 400。
- 改 handler 注释后：`make docs && go test ./docs/`。

## 9. 测试与调试

```bash
go build ./internal/... ./cmd/server
go test ./internal/... ./docs/...
go vet ./internal/...
```

RAG 侧 e2e 见 `../rag/docs/测试与评测.md`。

## 10. 相关文档

- 文档索引：`../docs/03-进度与交接/当前进度.md`
- 个人版需求：`../docs/个人版改造/需求与范围.md`
- 部署：`../deployments/README.md`
- 前端：`../frontend/README.md`
- RAG：`../rag/README.md`
- MCP：`../mcp/README.md`

## License

见 `LICENSE` 与 `THIRD_PARTY_NOTICES.md`。
