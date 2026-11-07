# WellRead MCP Server

把 WellRead 知识库 API 暴露为 Model Context Protocol 工具，供 Claude Desktop、Cursor、KiloCode、VS Code Copilot 等 MCP 客户端调用。服务本身不存业务数据，所有调用只读转发到 WellRead 后端。

## 快速开始

```bash
cd mcp
uv sync --extra test
export WELLREAD_BASE_URL="http://localhost:8080/api/v1"
export WELLREAD_API_KEY="<WellRead 单 Key>"
uv run run_server.py
```

WellRead 个人版只有一个只读机器密钥：在「设置 → 外部接入」查看、生成、重置或删除；明文只在生成/重置时显示一次。MCP 只暴露 17 个只读工具。详细步骤见 [安装与运行](docs/安装与运行.md)。

## 代码地图

| 文件 | 作用 |
|---|---|
| `gaze_mcp_server.py` | MCP 实例、鉴权中间件与 stdio/SSE/HTTP 传输 |
| `client.py` | 只读 `GazeClient`、名称解析、SSE 读取 |
| `tools.py` | 17 个只读工具的注册 |
| `main.py` | 统一 CLI 入口，支持 `--transport`、`--check-only` |
| `run_server.py` | stdio 启动包装 |
| `run.py` | 最小便捷启动脚本 |
| `tests/` | pytest/unittest 测试 |
| `scripts/check_imports.py` | 手工检查 MCP SDK 导入 |
| `pyproject.toml` / `setup.py` | 打包与 console entry points |
| `requirements.txt` / `uv.lock` | 运行依赖 |
| `Dockerfile` | HTTP 传输容器镜像 |

## 文档地图

| 文档 | 内容 |
|---|---|
| [安装与运行](docs/安装与运行.md) | 前置条件、API Key、环境变量、stdio/HTTP/SSE、Docker、故障排查 |
| [客户端接入](docs/客户端接入.md) | Claude Desktop、Cursor、KiloCode、远程 HTTP 的配置模板 |
| [工具参考](docs/工具参考.md) | 17 个只读工具的参数、用途与错误语义 |
| [调用示例](docs/调用示例.md) | 检索、问答、Agent、Gaze 的典型流程 |
| [架构与实现](docs/架构与实现.md) | 模块结构、请求链路、传输、鉴权、并发、安全、测试与限制 |

## 边界

- MCP 使用 WellRead 个人版单 Key；Key 只读，不继承用户角色，也不存在部门或共享空间上下文。
- 工具本身只做转发，最终授权由后端单 Key 能力与资源守卫决定。
- 工具面固定为 17 个只读工具；写入、删除、建库、模型管理、租户与共享空间能力已从 MCP 删除。
- 网络传输必须设置 `MCP_SERVER_AUTH_TOKEN`；本机单用户优先 stdio。

## 测试

```bash
cd mcp
uv run pytest -q
uv run python scripts/check_imports.py
```
