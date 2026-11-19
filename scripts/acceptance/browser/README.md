# 浏览器验收脚本

依赖本机 Chrome for Testing 与 `frontend/node_modules` 里的 `puppeteer-core`。脚本只读 `deployments/.env` 的 owner 账号；DeepSeek Key 从 `data/test-docs/deepseek.key`（gitignore）读取，不写日志。

## 登录、壳层与路由冒烟

```bash
NODE_PATH=frontend/node_modules \
WELLREAD_WEB=http://localhost:6151 \
node scripts/acceptance/browser/shell_auth_acceptance.js
```

`shell_auth_acceptance.js` 只读验证：未登录业务路由跳 `/login`；登录成功进 `/`；`/welcome` 与已登录 `/login` 回 `/`；左右栏 6+6 入口、知识库/复习/首页/设置跳转；顶栏头像入口；1440 与 600px 无横向溢出；控制台 0 错误。不写复习数据、不改设置。

## 复习页验收

复习主路由 `/platform/review` 已是 TDesign V3；V2 页面与 `review_v2_acceptance.js` 已随 shadcn 清理删除，历史结论只保留在文档归档。

### V3 骨架预览

```bash
NODE_PATH=frontend/node_modules \
WELLREAD_WEB=http://localhost:5174 \
node scripts/acceptance/browser/review_v3_skeleton_acceptance.js
```

`review_v3_skeleton_acceptance.js` 是早期骨架冒烟，验证三视图切换、卡组栏选择与悬停菜单、卡组新建/重命名/删除、任务条状态、1440/600px 无横向溢出与控制台 0 错误。页面后续已接入热力图与复习模式，新功能以本页下方脚本为准。

### V3 热力图 v2 与独立簇页

```bash
NODE_PATH=frontend/node_modules \
WELLREAD_WEB=http://localhost:5174 \
node scripts/acceptance/browser/review_v3_heatmap_acceptance.js
```

`review_v3_heatmap_acceptance.js` 验证行下钻、KB 回退列、top-N「显示全部」、独立簇子路由、四出口、根因 AI mock、返回保留 scope、1440/600px 与控制台。默认 mock heatmap/cluster/root-cause；脚本不连接真实后端聚合。

### V3 簇页 AI 出口

```bash
NODE_PATH=frontend/node_modules \
WELLREAD_WEB=http://localhost:5174 \
node scripts/acceptance/browser/review_v3_cluster_acceptance.js
```

`review_v3_cluster_acceptance.js` mock 簇详情与生成接口，验证四出口、重复候选禁用、拆分 apply 仅提交选中卡、新角度导入保留组/标签/来源、600px 与控制台。拆分/新角度不会改动真实卡片。

### V3 复习模式与重学

```bash
NODE_PATH=frontend/node_modules \
WELLREAD_WEB=http://localhost:5174 \
node scripts/acceptance/browser/review_v3_mode_acceptance.js
```

`review_v3_mode_acceptance.js` mock 会话与评分接口，验证：严重/警告卡横幅与参数条、空格翻面、Markdown 与代码块、来源新标签页、四档评分快捷键、完成页四档计数与连续天数、暂存退出、独立簇页重学冲突弹层（继续当前 / 放弃并开始补强）、600px 2×2 评分与控制台 0 错误。脚本不会改动真实复习调度。

### V3 真实后端与真实 LLM

```bash
NODE_PATH=frontend/node_modules \
WELLREAD_WEB=http://localhost:5174 \
node scripts/acceptance/browser/review_v3_real_acceptance.js
```

`review_v3_real_acceptance.js` 不拦截网络，直接连接运行中的后端与配置的问答模型。脚本在顶层新建 `V3真实验收-<run>` 卡组树（10 个子组、10 张卡，复用现有知识分块来源），对两张卡各提交两次「忘记」制造严重簇，然后验证：

- 热力图：默认 8 行/10 列截断、`其他卡组`/`其他主题` 桶、`显示全部` 展开到 10 行/12 列、知识库回退列、行下钻、严重格可点击。
- 簇详情：强度公式、原因、参数表、四出口启用；根因真实 LLM 成功（DNS/模型失败时脚本最多重试 3 次）。
- 重学：创建真实 `manual` 会话并暂存退出。
- 拆分：真实 LLM 预览并应用，原卡转 `suspended`。
- 新角度：真实 LLM 预览并导入，原卡保持 `active`。
- 回原文：新标签页打开真实知识库分块；600px 无页面横向溢出、控制台 0 错误。

前置：后端为当前 V3 镜像、Vite dev 在 `http://localhost:5174`、`deployments/.env` 有 owner 账号、`data/test-docs/deepseek.key` 对应的模型已配置。脚本结束会按卡组白名单删除本次测试的卡组、卡片、来源、日志与会话；不要把它指向真实卡组。截图、步骤日志与失败诊断写入 `ACCEPT_OUT`（默认系统临时目录 `wellread_accept`）。

### V3 卡片与统计

```bash
NODE_PATH=frontend/node_modules \
WELLREAD_WEB=http://localhost:5174 \
node scripts/acceptance/browser/review_v3_cards_stats_acceptance.js
```

`review_v3_cards_stats_acceptance.js` 连接真实后端，在隔离卡组中创建 4 张卡并评分，验证：搜索筛选、表现抽屉、编辑与单卡重置、批量追加标签、批量删除、「已删除」筛选与恢复、抽屉立即复习、真实新建卡片（知识库+文档来源）、统计四 KPI/图表/会话历史/CSV `easy_count`、600px 与 0 控制台错误。脚本结束清理测试卡组、卡、来源、日志与会话。

主路由为 `/platform/review`；V3 的 mock 套件也使用该路径，旧 `/platform/review-v3` 仅保留重定向。

### V3 对等能力与 daily 会话

```bash
NODE_PATH=frontend/node_modules \
WELLREAD_WEB=http://localhost:5174 \
node scripts/acceptance/browser/review_v3_parity_acceptance.js
```

`review_v3_parity_acceptance.js` 连接真实后端，验证：合并卡片、孤儿重绑、知识库生成预览与导入、JSON 导出，以及真实 `daily` 会话开始 → 评分 → 暂存 → 继续 → 完成。脚本在存在其他 active 会话时会跳过 daily 段，绝不触碰真实会话；其余段正常执行。测试数据按卡组白名单清理。

## 其它脚本

```bash
NODE_PATH=frontend/node_modules \
CHROME_PATH="D:/chrome-for-test/chrome/win64-157.0.8103.38/chrome-win64/chrome.exe" \
node scripts/acceptance/browser/provider_acceptance.js

NODE_PATH=frontend/node_modules \
CHROME_PATH="D:/chrome-for-test/chrome/win64-157.0.8103.38/chrome-win64/chrome.exe" \
node scripts/acceptance/browser/key_clear_acceptance.js
```

可选环境变量：

- `WELLREAD_WEB`：前端地址，默认 `http://localhost:6151`
- `WELLREAD_ROOT`：仓库根，默认脚本上三级
- `ACCEPT_OUT`：截图与日志目录，默认系统临时目录下 `wellread_accept`

`provider_acceptance.js` 覆盖：新建 DeepSeek（默认）、连接测试、编辑、新建 DuckDuckGo、切换默认、Agent 引用时删除 409、解除引用后删除两个配置。会临时向 `custom_agents` 插入并清理一条 `accept-agent-1` 记录。
