# 浏览器验收脚本

依赖本机 Chrome for Testing 与 `frontend/node_modules` 里的 `puppeteer-core`。脚本只读 `deployments/.env` 的 owner 账号；DeepSeek Key 从 `data/test-docs/deepseek.key`（gitignore）读取，不写日志。

## 复习页验收

```bash
NODE_PATH=frontend/node_modules \
CHROME_PATH="D:/chrome-for-test/chrome/win64-157.0.8103.38/chrome-win64/chrome.exe" \
node scripts/acceptance/browser/review_v2_acceptance.js
```

`review_v2_acceptance.js` 覆盖：三标签、20 卡两轮、刷新恢复、易忘预警、第三次忘记自动暂停、来源失效转孤儿、跨日会话过期、7/30/90 统计、桌面/600px 布局与控制台检查。测试卡组与复习记录在结束时清理。

V2 脚本只验证当前 V2 行为；复习 V3 的四档评分、漏洞簇、批量与重置、TDesign 页面需要另写脚本，不能沿用此结论。

### V3 骨架预览

```bash
NODE_PATH=frontend/node_modules \
WELLREAD_WEB=http://localhost:5174 \
node scripts/acceptance/browser/review_v3_skeleton_acceptance.js
```

`review_v3_skeleton_acceptance.js` 验证临时预览路由 `/platform/review-v3`：三视图切换、卡组栏选择与悬停菜单、卡组新建/重命名/删除、任务条状态、今日占位模块、1440/600px 无横向溢出与控制台 0 错误。脚本会创建并删除一个 `V3骨架-*` 测试卡组；卡片管理、热力图与复习模式尚未覆盖。

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
