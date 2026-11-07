# WellRead Frontend

WellRead 个人版的 Vue 3 单页应用：问答、知识库、精读、技能与设置。前端只负责展示与交互，认证与数据范围最终由后端判定。

## 技术栈

- Vue 3 + TypeScript + Vite 7
- TDesign Vue Next
- Pinia（状态）、Vue Router（路由）
- vue-i18n（多语言）
- Markdown/Mermaid/KaTeX、Office 预览、xlsx 导入等展示能力

## 目录地图

| 路径 | 作用 |
|---|---|
| `src/api/` | 按业务域拆分的后端 API 封装 |
| `src/components/` | 通用组件（菜单、聊天、设置、模型选择等） |
| `src/composables/` | 可复用组合式逻辑 |
| `src/config/` | 部署能力、上下文引导等前端配置 |
| `src/directives/` | 全局指令 |
| `src/hooks/` | 页面级 hooks |
| `src/i18n/` | 语言包与文案扫描脚本 |
| `src/router/` | 路由表与登录守卫 |
| `src/stores/` | Pinia 状态（auth、ui、menu、chat 资源等） |
| `src/styles/` | 全局样式与主题扩展 |
| `src/types/` | 前后端共享类型/视图类型 |
| `src/utils/` | 请求、格式化、Markdown、引用等工具 |
| `src/views/` | 页面：auth/chat/creatChat/knowledge/reading/skills/settings/platform |
| `public/` | 静态资源；登录页架构图在 `public/diagrams/` |
| `design/` | 设计源（图标、字体），不进入生产构建 |
| `scripts/` | 前端专项脚本（构建、图标矢量化、PR 校验） |

## 本地开发

```bash
cd frontend
npm ci
npm run dev
```

- 开发地址：`http://localhost:5173`
- `/api`、`/files` 默认代理到 `http://localhost:6152`
- 可通过 `VITE_DEV_PROXY_TARGET` 或 `FRONTEND_BACKEND_URL` 改代理目标

## 构建与检查

```bash
npm run type-check       # vue-tsc
npm test                 # tsx --test
npm run scan-i18n-gaps   # 扫描缺失文案
npm run build            # 生产构建到 dist/
npm run preview          # 本地预览生产构建，4173
```

`npm run build-with-types` 并行跑 type-check 和 build，适合交付前使用。

## 路由与菜单

- 个人版只有登录/未登录两态，没有角色、能力契约或 403 页面。
- 侧栏五项目导航在 `src/stores/menu.ts`：问答、知识库、精读、技能、设置。
- 路由守卫只校验登录态，未登录跳 `/login`。
- `/platform/*` 是唯一业务命名空间；`/` 默认重定向到 `/platform/creatChat`。

## 数据与请求

- `src/api/` 按域封装；统一请求工具在 `src/utils/request`。
- 登录态在 `src/stores/auth`；个人版单账号，没有空间切换与角色计算。
- 聊天/SSE、知识库、精读、技能、设置分别有独立 API 模块，不在页面里直接拼 URL。
- 写操作权限由后端单 Key / JWT 判定；前端隐藏按钮不是授权。

## i18n

- 主语言包：`src/i18n/locales/zh-CN.ts`。
- 新增文案用 `npm run scan-i18n-gaps` 定位缺失项；不要把中文文案硬编码在新增的通用组件里，业务页已有历史文案按现有模式处理。

## 资产与设计源

- 运行时资源：`src/assets/`（字体、图片、主题）。
- 设计源：`design/ui-source/`、`design/fonts/`、`design/brand/`；不进入镜像，映射见 `design/README.md`。
- 手绘图标矢量化：`scripts/vectorize_icon.py`；生成结果在聊天检索图标组件中。
- 新增 UI 组件应模仿同页同型组件，不另起一套视觉语言。

## 登录页与架构图

- 页面：`src/views/auth/Login.vue`。
- 左侧展示区为 Archify 生成的交互式架构图：`public/diagrams/wellread-architecture.html`。
- iframe 使用 `?embed=1&theme=light`；`Login.vue` 加载后移除 `data-embed`，注入登录页样式隐藏 Viewer 外壳，并调用 `Archify.motionGovernor.resume()` 开启流动与节点详情。
- 架构图展示个人版服务链路与一次问答/入库数据流，不再包含部门、共享空间、协作或治理节点。
- 图源与校验产物在仓库根 `.archify/architecture-wellread-login-20261026/`；重新生成后需验证节点详情、流动动画和徽标隐藏。

## Docker 与 nginx

```bash
docker build -f Dockerfile -t gaze-ui:latest .
```

- 构建阶段不需要 contracts named context；产物不包含 `design/`。
- 运行阶段是 nginx，`nginx-api-proxy.conf` 关闭代理缓冲，适配 SSE。
- 容器端口 80；`MAX_FILE_SIZE_MB` 控制上传限制。

## 约定

- 页面可见性由后端数据决定；不要在前端自行扩大后端返回的数据范围。
- 新增路由只需登录守卫，不再声明 `requiredCaps` / `requiredAnyCaps`。
- API 返回的敏感字段不在前端日志/控制台打印。
- 大列表/长文档优先复用现有虚拟滚动、分页和 Markdown 渲染管线。

## 相关文档

- 文档总入口：`../README.md`
- 当前进度：`../docs/03-进度与交接/当前进度.md`
- 个人版需求：`../docs/个人版改造/需求与范围.md`
- 页面与提示词：`../docs/个人版改造/页面与提示词.md`
- 设计源说明：`design/README.md`
