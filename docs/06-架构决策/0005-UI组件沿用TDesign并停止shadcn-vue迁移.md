---
status: accepted
---

# UI 组件沿用 TDesign 并停止 shadcn-vue 迁移

## 背景

复习页曾作为 shadcn-vue + Tailwind 的试点，功能与布局验收通过，但 owner 实机查看后判定视觉效果不佳。原计划以复习页为参考实现，把全前端逐步迁移到 shadcn-vue；试点未通过意味着继续迁移会把审美返工扩散到所有页面。前端目标风格始终是 TDesign 视觉语言 + 品牌绿，而不是 shadcn 默认灰黑色板。

同时，双组件栈会带来主题、圆角、间距、深色模式与可访问性规则的分叉；复习页一套、其他页面一套，维护成本高于收益。

## 决策

1. 全前端组件统一使用 TDesign Vue Next；Tailwind 只用于布局、间距与自定义区块，不承载第二套组件主题。
2. 停止 shadcn-vue 全前端迁移；不新增 `shadcn-vue`、`reka-ui`、`class-variance-authority` 等 shadcn 相关依赖，也不新增其他第二套组件库。
3. 现有复习页的 shadcn-vue 实现视为 V2 试点残留，保留到复习 V3 同批替换；替换完成时删除 `src/components/ui`、`components.json`、仅服务于 shadcn 的工具与依赖，并清理 `tailwind.css` 中的 shadcn 导入与语义 token。
4. 禁止同一页面混用 TDesign 与 shadcn 组件；后续新页面直接按 TDesign 规范新建。
5. 视觉硬规则写入[界面规范](../02-需求与设计/界面规范.md) §3 与 §6：品牌绿只用于主 CTA、选中、进度与关键数据；状态色只用于小面积语义；卡片 6px、控件 3px；图标使用 lucide；不使用 emoji。
6. 统计图使用 ECharts，颜色读取 TDesign CSS 变量；主题切换后重绘。
7. 每个页面动工前单独确认信息密度，不跨页面默认继承；复习 V3 的密度与结构见[复习页 V3 设计与实施说明](../02-需求与设计/复习功能页面优化/优化事项.md)。

## 考虑过的替代方案

- **继续迁移 shadcn-vue，只重做复习页视觉**：可以保留既有组件代码，但双栈长期存在，且 owner 已否定试点方向；继续投入会放大返工。
- **TDesign 与 shadcn-vue 长期并存，按页面选择**：短期灵活，长期导致主题、交互与可访问性规则分叉，新页面没有唯一答案。
- **完全移除 Tailwind，只用 TDesign + Less**：风格最统一，但布局表达力下降，自定义区块与响应式需要更多手写 CSS；保留 Tailwind 做布局是成本更低的折中。

## 后果

- 复习 V3 需要重写现有 shadcn 复习页，工作量高于在旧页面上小修。
- 删除 shadcn 目录与依赖前，必须先完成后端契约与 TDesign 页面替换，避免页面不可用。
- 旧 TDesign 页面不受影响；不再存在「历史页 8px / 新页 6px」的圆角分叉。
- 2026-11-17 已完成：复习 V3 替换主路由，生成/合并/重绑/JSON 导出已移植；`src/components/ui`、`components.json`、shadcn/reka 依赖与 `tailwind.css` 语义 token 已删除，Tailwind 只保留布局 utility。
- 视觉验收仍由 owner 实机或截图确认；自动化测试只证明功能与布局，不代表审美通过。
- [ADR 0003](0003-UI组件渐进迁移到shadcn-vue.md) 标记为 superseded；本文成为 UI 组件的长期决策。
- 文档入口：[界面规范](../02-需求与设计/界面规范.md) §3/§6、`frontend/README.md`、`CLAUDE.md` §3。
- 2026-11-18：视觉目标条款由 [ADR 0007](0007-PostHog布局语言落地页与壳层改造.md) 部分修订：落地页与共享壳层已改用 PostHog 布局语言，TDesign 组件栈与品牌绿强调保留；详情页在各自批次迁移前仍按本文与界面规范执行。
